package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"wordgo/internal/keyboard"
	"wordgo/internal/lang"
	"wordgo/internal/store"
	"wordgo/internal/wordlist"
)

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

type guessResp struct {
	Attempt int      `json:"attempt"`
	Word    string   `json:"word"`
	Chars   string   `json:"chars,omitempty"`
	Tiles   []string `json:"tiles"`
	States  []string `json:"states"`
}

func parseGuesses(records []store.GuessRecord, hanzi map[string]string, toneLang string) []guessResp {
	out := make([]guessResp, 0, len(records))
	for _, r := range records {
		var states []string
		if err := json.Unmarshal([]byte(r.States), &states); err != nil {
			slog.Error("corrupt guess states", "game_id", r.GameID, "attempt", r.Attempt, "error", err)
		}
		out = append(out, guessResp{Attempt: r.Attempt, Word: r.Word, Chars: hanzi[r.Word], Tiles: lang.WordChars(r.Word, toneLang), States: states})
	}
	return out
}

// stateRank orders letter states so a keyboard key shows its best-known
// state once a letter has appeared in more than one guess.
var stateRank = map[string]int{"correct": 3, "present": 2, "absent": 1}

// aggregateKeyStates merges every guessed letter's best-known state across
// all of a game's guesses, keyed by lang.NormalizeChar (accent/kana/case
// insensitive) — computed once here so every client (web, TUI, ...) can
// color an on-screen keyboard without re-deriving this merge itself.
func aggregateKeyStates(records []guessResp, toneLang string) map[string]string {
	out := make(map[string]string)
	for _, r := range records {
		chars := lang.WordChars(r.Word, toneLang)
		for i, ch := range chars {
			if i >= len(r.States) {
				break
			}
			key := lang.NormalizeChar(ch)
			if stateRank[r.States[i]] > stateRank[out[key]] {
				out[key] = r.States[i]
			}
		}
	}
	return out
}

// winMessages are the exclamations shown on a won game, indexed by attempt
// number (1st guess -> index 0). Kept server-side so every client shares the
// same copy instead of each hardcoding it.
var winMessages = []string{"Genius!", "Magnificent!", "Impressive!", "Splendid!", "Great!", "Phew!"}

func winMessage(attempt int) string {
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(winMessages) {
		idx = len(winMessages) - 1
	}
	return winMessages[idx]
}

// addAnswerReveal fills in answer/definition/chars/etymology once a game is won.
func addAnswerReveal(resp map[string]any, game *store.Game, hanzi map[string]string) {
	resp["answer"] = game.Answer
	if words, err := wordlist.GetCachedWordList(game.Lang, game.WordLength); err == nil {
		if def := words[game.Answer]; def != "" {
			resp["definitions"] = wordlist.SplitDefinitions(def)
		}
	}
	if chars := hanzi[game.Answer]; chars != "" {
		resp["answer_chars"] = chars
	}
	if ety := wordlist.GetCachedEtymology(game.Lang, game.WordLength)[game.Answer]; ety != "" {
		resp["etymology"] = ety
	}
}

// POST /api/cache/clear. Scoped to a single lang/length so one client
// force-refreshing a stale word list can't wipe every other player's cache.
func HandleClearCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GameID uint   `json:"game_id"`
		Lang   string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	var key wordlist.Key
	if req.GameID != 0 {
		game, err := store.GetGame(req.GameID)
		if err != nil {
			jsonErr(w, "game not found", http.StatusNotFound)
			return
		}
		key = wordlist.Key{Lang: game.Lang, Len: game.WordLength}
	} else if lng := strings.TrimSpace(req.Lang); lng != "" {
		key = wordlist.Key{Lang: lng, Len: keyboard.DefaultLengthForLang(lng)}
	} else {
		jsonErr(w, "game_id or lang is required", http.StatusBadRequest)
		return
	}

	if err := wordlist.ClearWordListCache(key); err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true})
}

// POST /api/game.
func HandleNewGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Lang = strings.TrimSpace(req.Lang)

	if !slices.Contains(wordlist.GetCachedLanguages(), req.Lang) {
		jsonErr(w, fmt.Sprintf("unknown language %q - check /api/languages for valid names", req.Lang), http.StatusBadRequest)
		return
	}

	length := keyboard.DefaultLengthForLang(req.Lang)

	words, err := wordlist.GetCachedWordList(req.Lang, length)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	answer := wordlist.DailyAnswer(req.Lang, length, words)

	game := store.Game{
		Lang:       req.Lang,
		WordLength: length,
		Answer:     answer,
		Status:     "playing",
	}
	if err := store.CreateGame(&game); err != nil {
		slog.Error("create game failed", "err", err)
		jsonErr(w, "failed to create game", http.StatusInternalServerError)
		return
	}
	slog.Info("game created", "id", game.ID, "lang", game.Lang, "length", game.WordLength)

	alphabet := lang.BuildAlphabet(words, lang.ToneSplitKind(req.Lang))
	keyboardRows, overflowBases, equivalences, rtl, matraMap, layoutName := keyboard.BuildGameExtras(alphabet, req.Lang, words)
	jsonOK(w, map[string]any{
		"id":              game.ID,
		"lang":            game.Lang,
		"word_length":     game.WordLength,
		"status":          game.Status,
		"guesses":         []guessResp{},
		"alphabet":        alphabet,
		"keyboard_rows":   keyboardRows,
		"keyboard_layout": layoutName,
		"overflow_bases":  overflowBases,
		"equivalences":    equivalences,
		"rtl":             rtl,
		"matra_map":       matraMap,
		"key_states":      map[string]string{},
	})
}

// GET /api/game/{id}.
func HandleGetGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, "invalid game id", http.StatusBadRequest)
		return
	}

	game, err := store.GetGame(uint(id))
	if err != nil {
		jsonErr(w, "game not found", http.StatusNotFound)
		return
	}

	var alphabet []string
	var keyboardRows [][]string
	var overflowBases []string
	var equivalences [][]string
	var rtl bool
	var matraMap map[string]string
	var layoutName string
	toneLang := lang.ToneSplitKind(game.Lang)
	if words := wordlist.GetWordListIfCached(game.Lang, game.WordLength); words != nil {
		alphabet = lang.BuildAlphabet(words, toneLang)
		keyboardRows, overflowBases, equivalences, rtl, matraMap, layoutName = keyboard.BuildGameExtras(alphabet, game.Lang, words)
	}

	hanzi := wordlist.GetCachedHanzi(game.Lang, game.WordLength)
	guesses := parseGuesses(game.Guesses, hanzi, toneLang)
	resp := map[string]any{
		"id":              game.ID,
		"lang":            game.Lang,
		"word_length":     game.WordLength,
		"status":          game.Status,
		"guesses":         guesses,
		"alphabet":        alphabet,
		"keyboard_rows":   keyboardRows,
		"keyboard_layout": layoutName,
		"overflow_bases":  overflowBases,
		"equivalences":    equivalences,
		"rtl":             rtl,
		"matra_map":       matraMap,
		"key_states":      aggregateKeyStates(guesses, toneLang),
	}
	if game.Status != "playing" {
		addAnswerReveal(resp, game, hanzi)
	}

	jsonOK(w, resp)
}

// POST /api/game/{id}/guess.
func HandleGuess(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, "invalid game id", http.StatusBadRequest)
		return
	}

	var req struct {
		Word string `json:"word"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	game, err := store.GetGame(uint(id))
	if err != nil {
		jsonErr(w, "game not found", http.StatusNotFound)
		return
	}
	if game.Status != "playing" {
		jsonErr(w, "game is already over", http.StatusBadRequest)
		return
	}

	toneLang := lang.ToneSplitKind(game.Lang)

	guess := strings.ToLower(strings.TrimSpace(req.Word))
	if lang.IsJapaneseLang(game.Lang) {
		guess = lang.KatakanaToHiragana(guess)
	}
	guessChars := lang.WordChars(guess, toneLang)

	if len(guessChars) != game.WordLength {
		jsonErr(w, fmt.Sprintf("word must be %d characters", game.WordLength), http.StatusBadRequest)
		return
	}
	for _, ch := range guess {
		if ch != '*' && !lang.IsWordChar(ch) {
			jsonErr(w, "word contains invalid characters", http.StatusBadRequest)
			return
		}
	}

	words, err := wordlist.GetCachedWordList(game.Lang, game.WordLength)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, ok := words[guess]; !ok {
		normSet := wordlist.GetCachedNormalized(game.Lang, game.WordLength)
		var canonical string
		if strings.Contains(guess, "*") {
			canonical = lang.MatchWildcard(guessChars, normSet, wordlist.GetCachedOverflow(game.Lang, game.WordLength), toneLang)
		} else {
			canonical = normSet[lang.NormalizeWord(guess, toneLang)]
		}
		if canonical != "" {
			guess = canonical
			guessChars = lang.WordChars(guess, toneLang)
		} else {
			jsonOK(w, map[string]any{"error": "Not in word list"})
			return
		}
	}

	answerChars := lang.WordChars(game.Answer, toneLang)
	states := lang.Evaluate(guessChars, answerChars)
	statesJSON, _ := json.Marshal(states)
	attempt := len(game.Guesses) + 1

	rec := store.GuessRecord{
		GameID:  game.ID,
		Attempt: attempt,
		Word:    guess,
		States:  string(statesJSON),
	}
	if err := store.CreateGuess(&rec); err != nil {
		jsonErr(w, "failed to save guess", http.StatusInternalServerError)
		return
	}

	const maxGuesses = 6

	won := true
	for _, st := range states {
		if st != "correct" {
			won = false
			break
		}
	}
	lost := !won && attempt >= maxGuesses
	newStatus := game.Status
	if won {
		newStatus = "won"
	} else if lost {
		newStatus = "lost"
	}
	if won || lost {
		if err := store.UpdateGameStatus(game.ID, newStatus); err != nil {
			slog.Error("failed to update game status", "id", game.ID, "error", err)
		}
		slog.Info("game over", "id", game.ID, "status", newStatus, "attempts", attempt)
	}
	slog.Debug("guess", "id", game.ID, "attempt", attempt, "word", guess, "won", won)

	hanzi := wordlist.GetCachedHanzi(game.Lang, game.WordLength)
	allGuesses := append(parseGuesses(game.Guesses, nil, toneLang), guessResp{Attempt: attempt, Word: guess, Tiles: guessChars, States: states})
	resp := map[string]any{
		"attempt":      attempt,
		"word":         guess,
		"tiles":        guessChars,
		"states":       states,
		"status":       newStatus,
		"in_word_list": true,
		"key_states":   aggregateKeyStates(allGuesses, toneLang),
	}
	if chars := hanzi[guess]; chars != "" {
		resp["chars"] = chars
	}
	if won {
		resp["message"] = winMessage(attempt)
	}
	if won || lost {
		addAnswerReveal(resp, game, hanzi)
	}

	jsonOK(w, resp)
}

// GET /api/stats?lang=X&length=Y.
func HandleGetStats(w http.ResponseWriter, r *http.Request) {
	lng := r.URL.Query().Get("lang")
	length, _ := strconv.Atoi(r.URL.Query().Get("length"))

	games, err := store.GetCompletedGames(lng, length)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	total, wonCount := len(games), 0
	for _, g := range games {
		if g.Status == "won" {
			wonCount++
		}
	}

	winPct := 0
	if total > 0 {
		winPct = wonCount * 100 / total
	}

	maxStreak, streak := 0, 0
	for _, g := range games {
		if g.Status == "won" {
			streak++
			if streak > maxStreak {
				maxStreak = streak
			}
		} else {
			streak = 0
		}
	}

	currentStreak := 0
	for i := len(games) - 1; i >= 0; i-- {
		if games[i].Status == "won" {
			currentStreak++
		} else {
			break
		}
	}

	wonIDs := make([]uint, 0, wonCount)
	for _, g := range games {
		if g.Status == "won" {
			wonIDs = append(wonIDs, g.ID)
		}
	}
	distribution, _ := store.GetGuessDistribution(wonIDs)

	jsonOK(w, map[string]any{
		"games_played":   total,
		"games_won":      wonCount,
		"win_pct":        winPct,
		"current_streak": currentStreak,
		"max_streak":     maxStreak,
		"distribution":   distribution,
	})
}

// GET /api/languages.
func HandleGetLanguages(w http.ResponseWriter, r *http.Request) {
	langs := wordlist.GetCachedLanguages()
	defaultLengths := make(map[string]int, len(langs))
	for _, l := range langs {
		defaultLengths[l] = keyboard.DefaultLengthForLang(l)
	}
	jsonOK(w, map[string]any{"languages": langs, "default_lengths": defaultLengths})
}

// GET /api/progress?lang=X&length=Y.
func HandleGetProgress(w http.ResponseWriter, r *http.Request) {
	lng := r.URL.Query().Get("lang")
	length, _ := strconv.Atoi(r.URL.Query().Get("length"))
	key := fmt.Sprintf("%s:%d", lng, length)
	count := 0
	if v, ok := wordlist.DownloadProgress.Load(key); ok {
		count = v.(int)
	}
	jsonOK(w, map[string]any{"count": count})
}

// Package api implements the HTTP handlers for the Wordgo game API.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	States  []string `json:"states"`
}

func parseGuesses(records []store.GuessRecord, hanzi map[string]string) []guessResp {
	out := make([]guessResp, 0, len(records))
	for _, r := range records {
		var states []string
		if err := json.Unmarshal([]byte(r.States), &states); err != nil {
			slog.Error("corrupt guess states", "game_id", r.GameID, "attempt", r.Attempt, "error", err)
		}
		out = append(out, guessResp{Attempt: r.Attempt, Word: r.Word, Chars: hanzi[r.Word], States: states})
	}
	return out
}

// addAnswerReveal fills in answer/definition/chars/etymology once a game is won or lost.
func addAnswerReveal(resp map[string]any, game *store.Game, hanzi map[string]string) {
	resp["answer"] = game.Answer
	if words, err := wordlist.GetCachedWordList(game.Lang, game.WordLength); err == nil {
		resp["definition"] = words[game.Answer]
	}
	if chars := hanzi[game.Answer]; chars != "" {
		resp["answer_chars"] = chars
	}
	if ety := wordlist.GetCachedEtymology(game.Lang, game.WordLength)[game.Answer]; ety != "" {
		resp["etymology"] = ety
	}
}

// HandleClearCache handles POST /api/cache/clear.
// If game_id refers to a game still in progress, that game's word list is
// kept cached so the current game isn't broken mid-play.
func HandleClearCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GameID uint `json:"game_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var keep wordlist.Key
	if req.GameID != 0 {
		if game, err := store.GetGame(req.GameID); err == nil && game.Status == "playing" {
			keep = wordlist.Key{Lang: game.Lang, Len: game.WordLength}
		}
	}

	if err := wordlist.ClearWordListCache(keep); err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true})
}

// HandleNewGame handles POST /api/game.
func HandleNewGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lang       string `json:"lang"`
		Length     int    `json:"length"`
		MaxGuesses int    `json:"max_guesses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Lang == "" {
		req.Lang = wordlist.DefaultLang
	}
	if req.Length == 0 {
		req.Length = keyboard.DefaultLengthForLang(req.Lang)
	}
	if req.MaxGuesses == 0 {
		req.MaxGuesses = wordlist.DefaultGuesses
	}
	if req.Length < 2 || req.Length > 20 {
		jsonErr(w, "length must be between 2 and 20", http.StatusBadRequest)
		return
	}
	if req.MaxGuesses < 1 || req.MaxGuesses > 30 {
		jsonErr(w, "max_guesses must be between 1 and 30", http.StatusBadRequest)
		return
	}

	words, err := wordlist.GetCachedWordList(req.Lang, req.Length)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	wordSlice := make([]string, 0, len(words))
	for word := range words {
		wordSlice = append(wordSlice, word)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	answer := wordSlice[rng.Intn(len(wordSlice))]

	game := store.Game{
		Lang:       req.Lang,
		WordLength: req.Length,
		MaxGuesses: req.MaxGuesses,
		Answer:     answer,
		Status:     "playing",
	}
	if err := store.CreateGame(&game); err != nil {
		slog.Error("create game failed", "err", err)
		jsonErr(w, "failed to create game", http.StatusInternalServerError)
		return
	}
	slog.Info("game created", "id", game.ID, "lang", game.Lang, "length", game.WordLength, "max_guesses", game.MaxGuesses)

	alphabet := lang.BuildAlphabet(words, lang.ToneSplitKind(req.Lang))
	keyboardRows, overflowBases, equivalences, rtl, matraMap := keyboard.BuildGameExtras(alphabet, req.Lang, words)
	jsonOK(w, map[string]any{
		"id":             game.ID,
		"lang":           game.Lang,
		"word_length":    game.WordLength,
		"max_guesses":    game.MaxGuesses,
		"status":         game.Status,
		"guesses":        []guessResp{},
		"alphabet":       alphabet,
		"keyboard_rows":  keyboardRows,
		"overflow_bases": overflowBases,
		"equivalences":   equivalences,
		"rtl":            rtl,
		"matra_map":      matraMap,
	})
}

// HandleGetGame handles GET /api/game/{id}.
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
	if words := wordlist.GetWordListIfCached(game.Lang, game.WordLength); words != nil {
		alphabet = lang.BuildAlphabet(words, lang.ToneSplitKind(game.Lang))
		keyboardRows, overflowBases, equivalences, rtl, matraMap = keyboard.BuildGameExtras(alphabet, game.Lang, words)
	}

	hanzi := wordlist.GetCachedHanzi(game.Lang, game.WordLength)
	resp := map[string]any{
		"id":             game.ID,
		"lang":           game.Lang,
		"word_length":    game.WordLength,
		"max_guesses":    game.MaxGuesses,
		"status":         game.Status,
		"guesses":        parseGuesses(game.Guesses, hanzi),
		"alphabet":       alphabet,
		"keyboard_rows":  keyboardRows,
		"overflow_bases": overflowBases,
		"equivalences":   equivalences,
		"rtl":            rtl,
		"matra_map":      matraMap,
	}
	if game.Status != "playing" {
		addAnswerReveal(resp, game, hanzi)
	}

	jsonOK(w, resp)
}

// HandleGuess handles POST /api/game/{id}/guess.
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

	won := true
	for _, st := range states {
		if st != "correct" {
			won = false
			break
		}
	}
	lost := !won && attempt >= game.MaxGuesses
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
	resp := map[string]any{
		"attempt":      attempt,
		"word":         guess,
		"states":       states,
		"status":       newStatus,
		"in_word_list": true,
	}
	if chars := hanzi[guess]; chars != "" {
		resp["chars"] = chars
	}
	if won || lost {
		addAnswerReveal(resp, game, hanzi)
	}

	jsonOK(w, resp)
}

// HandleGetStats handles GET /api/stats?lang=English&length=5.
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

// HandleGetLanguages handles GET /api/languages.
func HandleGetLanguages(w http.ResponseWriter, r *http.Request) {
	langs := wordlist.GetCachedLanguages()
	defaultLengths := make(map[string]int, len(langs))
	for _, l := range langs {
		defaultLengths[l] = keyboard.DefaultLengthForLang(l)
	}
	jsonOK(w, map[string]any{"languages": langs, "default_lengths": defaultLengths})
}

// HandleGetProgress handles GET /api/progress?lang=X&length=Y.
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

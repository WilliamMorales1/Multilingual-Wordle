package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ── JSON helpers ──────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// ── Response helpers ──────────────────────────────────────────────────────────

type guessResp struct {
	Attempt int      `json:"attempt"`
	Word    string   `json:"word"`
	Chars   string   `json:"chars,omitempty"`
	States  []string `json:"states"`
}

func parseGuesses(records []GuessRecord, hanzi map[string]string) []guessResp {
	out := make([]guessResp, 0, len(records))
	for _, r := range records {
		var states []string
		_ = json.Unmarshal([]byte(r.States), &states)
		out = append(out, guessResp{Attempt: r.Attempt, Word: r.Word, Chars: hanzi[r.Word], States: states})
	}
	return out
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// POST /api/cache/clear
func handleClearCache(w http.ResponseWriter, r *http.Request) {
	if err := clearWordListCache(); err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true})
}

// POST /api/game
func handleNewGame(w http.ResponseWriter, r *http.Request) {
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
		req.Lang = DefaultLang
	}
	if req.Length == 0 {
		req.Length = DefaultLength
	}
	if req.MaxGuesses == 0 {
		req.MaxGuesses = DefaultGuesses
	}
	if req.Length < 2 || req.Length > 20 {
		jsonErr(w, "length must be between 2 and 20", http.StatusBadRequest)
		return
	}
	if req.MaxGuesses < 1 || req.MaxGuesses > 30 {
		jsonErr(w, "max_guesses must be between 1 and 30", http.StatusBadRequest)
		return
	}

	words, err := getCachedWordList(req.Lang, req.Length)
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

	game := Game{
		Lang:       req.Lang,
		WordLength: req.Length,
		MaxGuesses: req.MaxGuesses,
		Answer:     answer,
		Status:     "playing",
	}
	if err := dbCreateGame(&game); err != nil {
		logger.Error("create game failed", "err", err)
		jsonErr(w, "failed to create game", http.StatusInternalServerError)
		return
	}
	logger.Info("game created", "id", game.ID, "lang", game.Lang, "length", game.WordLength, "max_guesses", game.MaxGuesses)

	alphabet := buildAlphabet(words)
	keyboardRows, overflowBases, equivalences, rtl := buildGameExtras(alphabet, req.Lang, words)
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
	})
}

// GET /api/game/{id}
func handleGetGame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, "invalid game id", http.StatusBadRequest)
		return
	}

	game, err := dbGetGame(uint(id))
	if err != nil {
		jsonErr(w, "game not found", http.StatusNotFound)
		return
	}

	var alphabet []string
	var keyboardRows [][]string
	var overflowBases []string
	var equivalences [][]string
	var rtl bool
	if words := getWordListIfCached(game.Lang, game.WordLength); words != nil {
		alphabet = buildAlphabet(words)
		keyboardRows, overflowBases, equivalences, rtl = buildGameExtras(alphabet, game.Lang, words)
	}

	hanzi := getCachedHanzi(game.Lang, game.WordLength)
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
	}
	if game.Status != "playing" {
		resp["answer"] = game.Answer
		if words, err := getCachedWordList(game.Lang, game.WordLength); err == nil {
			resp["definition"] = words[game.Answer]
		}
		if hanzi == nil {
			hanzi = getCachedHanzi(game.Lang, game.WordLength)
		}
		if chars := hanzi[game.Answer]; chars != "" {
			resp["answer_chars"] = chars
		}
		if ety := getCachedEtymology(game.Lang, game.WordLength)[game.Answer]; ety != "" {
			resp["etymology"] = ety
		}
	}

	jsonOK(w, resp)
}

// POST /api/game/{id}/guess
func handleGuess(w http.ResponseWriter, r *http.Request) {
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

	game, err := dbGetGame(uint(id))
	if err != nil {
		jsonErr(w, "game not found", http.StatusNotFound)
		return
	}
	if game.Status != "playing" {
		jsonErr(w, "game is already over", http.StatusBadRequest)
		return
	}

	guess := strings.ToLower(strings.TrimSpace(req.Word))
	if isJapaneseLang(game.Lang) {
		guess = katakanaToHiragana(guess)
	}
	guessChars := wordChars(guess)

	if len(guessChars) != game.WordLength {
		jsonErr(w, fmt.Sprintf("word must be %d characters", game.WordLength), http.StatusBadRequest)
		return
	}
	for _, ch := range guess {
		if ch != '*' && !isWordChar(ch) {
			jsonErr(w, "word contains invalid characters", http.StatusBadRequest)
			return
		}
	}

	words, err := getCachedWordList(game.Lang, game.WordLength)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, ok := words[guess]; !ok {
		normSet := getCachedNormalized(game.Lang, game.WordLength)
		var canonical string
		if strings.Contains(guess, "*") {
			canonical = matchWildcard(guessChars, normSet, getCachedOverflow(game.Lang, game.WordLength))
		} else {
			canonical = normSet[normalizeWord(guess)]
		}
		if canonical != "" {
			guess = canonical
			guessChars = wordChars(guess)
		} else {
			jsonOK(w, map[string]any{"error": "Not in word list"})
			return
		}
	}

	answerChars := wordChars(game.Answer)
	states := evaluate(guessChars, answerChars)
	statesJSON, _ := json.Marshal(states)
	attempt := len(game.Guesses) + 1

	rec := GuessRecord{
		GameID:  game.ID,
		Attempt: attempt,
		Word:    guess,
		States:  string(statesJSON),
	}
	if err := dbCreateGuess(&rec); err != nil {
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
		_ = dbUpdateGameStatus(game.ID, newStatus)
		logger.Info("game over", "id", game.ID, "status", newStatus, "attempts", attempt)
	}
	logger.Debug("guess", "id", game.ID, "attempt", attempt, "word", guess, "won", won)

	hanzi := getCachedHanzi(game.Lang, game.WordLength)
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
		resp["answer"] = game.Answer
		if words, err := getCachedWordList(game.Lang, game.WordLength); err == nil {
			resp["definition"] = words[game.Answer]
		}
		if chars := hanzi[game.Answer]; chars != "" {
			resp["answer_chars"] = chars
		}
		if ety := getCachedEtymology(game.Lang, game.WordLength)[game.Answer]; ety != "" {
			resp["etymology"] = ety
		}
	}

	jsonOK(w, resp)
}

// GET /api/stats?lang=English&length=5
func handleGetStats(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	length, _ := strconv.Atoi(r.URL.Query().Get("length"))

	games, err := dbGetCompletedGames(lang, length)
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
	distribution, _ := dbGetGuessDistribution(wonIDs)

	jsonOK(w, map[string]any{
		"games_played":   total,
		"games_won":      wonCount,
		"win_pct":        winPct,
		"current_streak": currentStreak,
		"max_streak":     maxStreak,
		"distribution":   distribution,
	})
}

// GET /api/languages
func handleGetLanguages(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{"languages": getCachedLanguages()})
}

// GET /api/avglength?lang=X
func handleGetAvgLength(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = DefaultLang
	}
	jsonOK(w, map[string]any{"avg_length": avgWordLengths[lang]})
}

// GET /api/progress?lang=X&length=Y
func handleGetProgress(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	length, _ := strconv.Atoi(r.URL.Query().Get("length"))
	key := fmt.Sprintf("%s:%d", lang, length)
	count := 0
	if v, ok := downloadProgress.Load(key); ok {
		count = v.(int)
	}
	jsonOK(w, map[string]any{"count": count})
}

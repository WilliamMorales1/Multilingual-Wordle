package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dataPath returns a path inside DATA_DIR (defaults to "." for local dev).
func dataPath(name string) string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, name)
}

// ----------------------------------------------------------------------------
// Global state
// ----------------------------------------------------------------------------

var (
	db            *gorm.DB
	wordListCache sync.Map // key: "lang:length" → map[string]string
	loadMu        sync.Mutex

	langCacheMu sync.RWMutex
	langCache   []string // sorted language names, populated on first request

	downloadProgress sync.Map // key: "lang:length" → int (words collected so far)
)

// ----------------------------------------------------------------------------
// Database
// ----------------------------------------------------------------------------

func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open(dataPath("wordle.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	if err := db.AutoMigrate(&Game{}, &GuessRecord{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	log.Println("Database ready (wordle.db)")
}

// ----------------------------------------------------------------------------
// Cached helpers
// ----------------------------------------------------------------------------

// getCachedWordList returns the word list from memory, or loads + caches it.
func getCachedWordList(lang string, length int) (map[string]string, error) {
	key := fmt.Sprintf("%s:%d", lang, length)
	if v, ok := wordListCache.Load(key); ok {
		return v.(map[string]string), nil
	}

	loadMu.Lock()
	defer loadMu.Unlock()

	// Re-check under lock (double-checked locking)
	if v, ok := wordListCache.Load(key); ok {
		return v.(map[string]string), nil
	}

	words, err := loadWordList(lang, length)
	if err != nil {
		return nil, err
	}
	wordListCache.Store(key, words)
	return words, nil
}

// getCachedLanguages returns a sorted slice of language names, fetching once.
func getCachedLanguages() []string {
	langCacheMu.RLock()
	if langCache != nil {
		defer langCacheMu.RUnlock()
		return langCache
	}
	langCacheMu.RUnlock()

	langCacheMu.Lock()
	defer langCacheMu.Unlock()

	if langCache != nil {
		return langCache
	}

	langMap := getLanguages()
	names := make([]string, 0, len(langMap))
	for name := range langMap {
		names = append(names, name)
	}
	sort.Strings(names)
	langCache = names
	return names
}

// ----------------------------------------------------------------------------
// Response types
// ----------------------------------------------------------------------------

type guessResp struct {
	Attempt int      `json:"attempt"`
	Word    string   `json:"word"`
	States  []string `json:"states"`
}

func parseGuesses(records []GuessRecord) []guessResp {
	out := make([]guessResp, 0, len(records))
	for _, r := range records {
		var states []string
		_ = json.Unmarshal([]byte(r.States), &states)
		out = append(out, guessResp{Attempt: r.Attempt, Word: r.Word, States: states})
	}
	return out
}

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

// POST /api/game
func handleNewGame(c *gin.Context) {
	var req struct {
		Lang       string `json:"lang"`
		Length     int    `json:"length"`
		MaxGuesses int    `json:"max_guesses"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "length must be between 2 and 20"})
		return
	}
	if req.MaxGuesses < 1 || req.MaxGuesses > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_guesses must be between 1 and 30"})
		return
	}

	words, err := getCachedWordList(req.Lang, req.Length)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Pick a random answer
	wordSlice := make([]string, 0, len(words))
	for w := range words {
		wordSlice = append(wordSlice, w)
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
	if err := db.Create(&game).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create game"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          game.ID,
		"lang":        game.Lang,
		"word_length": game.WordLength,
		"max_guesses": game.MaxGuesses,
		"status":      game.Status,
		"guesses":     []guessResp{},
		"alphabet":    buildAlphabet(words),
	})
}

// GET /api/game/:id
func handleGetGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
		return
	}

	var game Game
	if err := db.Preload("Guesses", func(db *gorm.DB) *gorm.DB {
		return db.Order("attempt asc")
	}).First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}

	// Alphabet from in-memory cache (best-effort)
	var alphabet []string
	if v, ok := wordListCache.Load(fmt.Sprintf("%s:%d", game.Lang, game.WordLength)); ok {
		alphabet = buildAlphabet(v.(map[string]string))
	}

	resp := gin.H{
		"id":          game.ID,
		"lang":        game.Lang,
		"word_length": game.WordLength,
		"max_guesses": game.MaxGuesses,
		"status":      game.Status,
		"guesses":     parseGuesses(game.Guesses),
		"alphabet":    alphabet,
	}
	if game.Status != "playing" {
		resp["answer"] = game.Answer
		if words, err := getCachedWordList(game.Lang, game.WordLength); err == nil {
			resp["definition"] = words[game.Answer]
		}
	}

	c.JSON(http.StatusOK, resp)
}

// POST /api/game/:id/guess
func handleGuess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
		return
	}

	var req struct {
		Word string `json:"word"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var game Game
	if err := db.Preload("Guesses", func(db *gorm.DB) *gorm.DB {
		return db.Order("attempt asc")
	}).First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}
	if game.Status != "playing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "game is already over"})
		return
	}

	guess := strings.ToLower(strings.TrimSpace(req.Word))
	guessChars := wordChars(guess)

	if len(guessChars) != game.WordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("word must be %d characters", game.WordLength)})
		return
	}
	for _, r := range guess {
		if !isWordChar(r) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "word contains invalid characters"})
			return
		}
	}

	// Reject guesses not in the word list
	words, err := getCachedWordList(game.Lang, game.WordLength)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, ok := words[guess]; !ok {
		c.JSON(http.StatusOK, gin.H{"error": "Not in word list"})
		return
	}

	// Evaluate
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
	if err := db.Create(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save guess"})
		return
	}

	const inWordList = true

	// Update game status
	// Win if every tile is correct (accent-insensitive, derived from evaluate).
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
		db.Model(&game).Update("status", newStatus)
	}

	resp := gin.H{
		"attempt":      attempt,
		"word":         guess,
		"states":       states,
		"status":       newStatus,
		"in_word_list": inWordList,
	}
	if won || lost {
		resp["answer"] = game.Answer
		if words, err := getCachedWordList(game.Lang, game.WordLength); err == nil {
			resp["definition"] = words[game.Answer]
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GET /api/stats?lang=English&length=5
func handleGetStats(c *gin.Context) {
	lang := c.Query("lang")
	lengthStr := c.Query("length")

	// Build base query with optional filters
	filter := func(q *gorm.DB) *gorm.DB {
		if lang != "" {
			q = q.Where("lang = ?", lang)
		}
		if lengthStr != "" {
			if n, err := strconv.Atoi(lengthStr); err == nil {
				q = q.Where("word_length = ?", n)
			}
		}
		return q
	}

	// Load all completed games for streak + distribution calculation
	var games []Game
	filter(db.Model(&Game{}).Where("status IN ?", []string{"won", "lost"})).
		Order("created_at asc").Find(&games)

	total := len(games)
	wonCount := 0
	for _, g := range games {
		if g.Status == "won" {
			wonCount++
		}
	}

	winPct := 0
	if total > 0 {
		winPct = wonCount * 100 / total
	}

	// Max streak
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

	// Current streak (count consecutive wins from the end)
	currentStreak := 0
	for i := len(games) - 1; i >= 0; i-- {
		if games[i].Status == "won" {
			currentStreak++
		} else {
			break
		}
	}

	// Guess distribution for won games
	wonIDs := make([]uint, 0, wonCount)
	for _, g := range games {
		if g.Status == "won" {
			wonIDs = append(wonIDs, g.ID)
		}
	}

	distribution := make(map[int]int)
	if len(wonIDs) > 0 {
		type row struct {
			GameID     uint `gorm:"column:game_id"`
			MaxAttempt int  `gorm:"column:max_attempt"`
		}
		var rows []row
		db.Table("guess_records").
			Select("game_id, MAX(attempt) as max_attempt").
			Where("game_id IN ?", wonIDs).
			Group("game_id").
			Scan(&rows)
		for _, r := range rows {
			distribution[r.MaxAttempt]++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"games_played":   total,
		"games_won":      wonCount,
		"win_pct":        winPct,
		"current_streak": currentStreak,
		"max_streak":     maxStreak,
		"distribution":   distribution,
	})
}

// GET /api/languages
func handleGetLanguages(c *gin.Context) {
	langs := getCachedLanguages()
	c.JSON(http.StatusOK, gin.H{"languages": langs})
}

// GET /api/progress?lang=X&length=Y
func handleGetProgress(c *gin.Context) {
	lang := c.Query("lang")
	length, _ := strconv.Atoi(c.Query("length"))
	key := fmt.Sprintf("%s:%d", lang, length)
	count := 0
	if v, ok := downloadProgress.Load(key); ok {
		count = v.(int)
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// ----------------------------------------------------------------------------
// main
// ----------------------------------------------------------------------------

func main() {
	initDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Serve the frontend — no-cache so browsers always fetch fresh files after a deploy
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") || c.Request.URL.Path == "/" {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	})
	r.Static("/static", "./static")
	r.StaticFile("/", "./static/index.html")

	// API
	api := r.Group("/api")
	{
		api.GET("/languages", handleGetLanguages)
		api.GET("/progress", handleGetProgress)
		api.POST("/game", handleNewGame)
		api.GET("/game/:id", handleGetGame)
		api.POST("/game/:id/guess", handleGuess)
		api.GET("/stats", handleGetStats)
	}

	log.Println("Wordle server running → http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

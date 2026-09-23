// Package store persists games and guesses to a SQLite database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init opens & migrates the SQLite database. It is fatal on failure, since
// the server can't run without persistence. Calling it again (tests point it
// at a fresh temp directory per case) closes the previous handle first, so
// the old connection pool isn't leaked.
func Init() {
	if db != nil {
		db.Close()
		db = nil
	}
	dbPath := "wordgo.db"
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		dbPath = filepath.Join(dir, "wordgo.db")
	}

	var err error
	db, err = sql.Open("sqlite", dsn(dbPath))
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	// SQLite allows one writer at a time. WAL keeps readers from blocking
	// that writer, and busy_timeout makes a second writer wait its turn
	// instead of failing immediately — without it every concurrent request
	// that writes (creating a game, saving a guess) returns SQLITE_BUSY
	// "database is locked" the moment two players act at once.
	db.SetMaxOpenConns(maxOpenConns)
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	if err := createTables(); err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	log.Printf("Database ready: %s", dbPath)
}

// maxOpenConns caps the pool. SQLite serializes writes anyway, and a small
// pool keeps the busy_timeout queue short rather than letting dozens of
// connections pile up on the same write lock.
const maxOpenConns = 8

// busyTimeout is how long a blocked writer waits for the write lock before
// giving up with SQLITE_BUSY.
const busyTimeout = 5 * time.Second

// dsn builds the connection string: the file path plus the pragmas that make
// concurrent access work (see Init).
func dsn(path string) string {
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	return "file:" + path + "?" + q.Encode()
}

func createTables() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			lang        TEXT NOT NULL,
			word_length INTEGER NOT NULL,
			answer      TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'playing',
			max_guesses INTEGER NOT NULL DEFAULT 6
		);
		CREATE TABLE IF NOT EXISTS guess_records (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			game_id INTEGER NOT NULL,
			attempt INTEGER NOT NULL,
			word    TEXT NOT NULL,
			states  TEXT NOT NULL,
			FOREIGN KEY (game_id) REFERENCES games(id)
		);
	`)
	return err
}

func CreateGame(g *Game) error {
	res, err := db.Exec(
		`INSERT INTO games (lang, word_length, answer, status, max_guesses) VALUES (?, ?, ?, ?, 6)`,
		g.Lang, g.WordLength, g.Answer, g.Status,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	g.ID = uint(id)
	return nil
}

// ErrNotFound reports that no game has the requested id. Callers have to be
// able to tell it apart from a database that is down or locked: answering
// every GetGame failure with "game not found" turns a transient 500 into a
// 404, and a client that trusts the 404 throws the player's game away.
var ErrNotFound = errors.New("game not found")

func GetGame(id uint) (*Game, error) {
	g := &Game{}
	err := db.QueryRow(
		`SELECT id, lang, word_length, answer, status FROM games WHERE id = ?`, id,
	).Scan(&g.ID, &g.Lang, &g.WordLength, &g.Answer, &g.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("game %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT id, game_id, attempt, word, states FROM guess_records WHERE game_id = ? ORDER BY attempt ASC`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r GuessRecord
		if err := rows.Scan(&r.ID, &r.GameID, &r.Attempt, &r.Word, &r.States); err != nil {
			return nil, err
		}
		g.Guesses = append(g.Guesses, r)
	}
	return g, rows.Err()
}

func CreateGuess(r *GuessRecord) error {
	res, err := db.Exec(
		`INSERT INTO guess_records (game_id, attempt, word, states) VALUES (?, ?, ?, ?)`,
		r.GameID, r.Attempt, r.Word, r.States,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	r.ID = uint(id)
	return nil
}

func UpdateGameStatus(id uint, status string) error {
	_, err := db.Exec(`UPDATE games SET status = ? WHERE id = ?`, status, id)
	return err
}

// GetCompletedGames returns every finished game (won *and* lost) for a
// lang/length, oldest first. Losses have to come back too: the stats handler
// derives win percentage and streaks from this list, and a won-only query
// would silently report a 100% win rate and an unbroken streak.
//
// id breaks ties in the ordering: SQLite's CURRENT_TIMESTAMP only has
// second resolution, so games finished in the same second would otherwise
// come back in an arbitrary order and make the streak numbers unstable.
func GetCompletedGames(lang string, length int) ([]Game, error) {
	query := `SELECT id, status FROM games WHERE status IN ('won', 'lost')`
	args := []any{}
	if lang != "" {
		query += ` AND lang = ?`
		args = append(args, lang)
	}
	if length > 0 {
		query += ` AND word_length = ?`
		args = append(args, length)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.Status); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func GetGuessDistribution(wonIDs []uint) (map[int]int, error) {
	if len(wonIDs) == 0 {
		return make(map[int]int), nil
	}

	placeholders := make([]string, len(wonIDs))
	args := make([]any, len(wonIDs))
	for i, id := range wonIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := db.Query(
		fmt.Sprintf(
			`SELECT game_id, MAX(attempt) FROM guess_records WHERE game_id IN (%s) GROUP BY game_id`,
			strings.Join(placeholders, ","),
		),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dist := make(map[int]int)
	for rows.Next() {
		var gameID uint
		var maxAttempt int
		if err := rows.Scan(&gameID, &maxAttempt); err != nil {
			return nil, err
		}
		dist[maxAttempt]++
	}
	return dist, rows.Err()
}

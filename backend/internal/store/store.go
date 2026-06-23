// Package store persists games and guesses to a SQLite database.
package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// dataPath resolves name relative to DATA_DIR (or the working directory if unset).
func dataPath(name string) string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, name)
}

// Init opens (and migrates) the SQLite database. It is fatal on failure since
// the server can't run without persistence.
func Init() {
	var err error
	db, err = sql.Open("sqlite", dataPath("wordgo.db"))
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	if err := createTables(); err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	log.Println("Database ready (wordgo.db)")
}

func createTables() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			lang        TEXT NOT NULL,
			word_length INTEGER NOT NULL,
			answer      TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'playing'
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
		`INSERT INTO games (lang, word_length, answer, status) VALUES (?, ?, ?, ?)`,
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

func GetGame(id uint) (*Game, error) {
	g := &Game{}
	err := db.QueryRow(
		`SELECT id, lang, word_length, answer, status FROM games WHERE id = ?`, id,
	).Scan(&g.ID, &g.Lang, &g.WordLength, &g.Answer, &g.Status)
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

func GetCompletedGames(lang string, length int) ([]Game, error) {
	query := `SELECT id, status FROM games WHERE status = 'won'`
	args := []any{}
	if lang != "" {
		query += ` AND lang = ?`
		args = append(args, lang)
	}
	if length > 0 {
		query += ` AND word_length = ?`
		args = append(args, length)
	}
	query += ` ORDER BY created_at ASC`

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

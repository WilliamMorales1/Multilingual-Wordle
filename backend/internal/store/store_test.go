package store

import (
	"testing"
)

// initTestDB points the package at a throwaway database for this test binary.
func initTestDB(t *testing.T) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	Init()
}

func seed(t *testing.T, lang string, length int, status string) uint {
	t.Helper()
	g := Game{Lang: lang, WordLength: length, Answer: "words", Status: status}
	if err := CreateGame(&g); err != nil {
		t.Fatalf("CreateGame(%s): %v", status, err)
	}
	return g.ID
}

// GetCompletedGames must return losses as well as wins: the stats handler
// derives win percentage and streaks from this list, so a won-only query
// reports a permanent 100% win rate and an unbreakable streak.
func TestGetCompletedGamesIncludesLosses(t *testing.T) {
	initTestDB(t)

	won := seed(t, "English", 5, "won")
	lost := seed(t, "English", 5, "lost")
	playing := seed(t, "English", 5, "playing")

	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("got %d completed games, want 2 (one won, one lost)", len(games))
	}

	byID := map[uint]string{}
	for _, g := range games {
		byID[g.ID] = g.Status
	}
	if byID[won] != "won" {
		t.Errorf("won game %d missing from completed games: %v", won, byID)
	}
	if byID[lost] != "lost" {
		t.Errorf("lost game %d missing from completed games: %v", lost, byID)
	}
	if _, ok := byID[playing]; ok {
		t.Errorf("unfinished game %d should not be in completed games", playing)
	}
}

// Oldest-first ordering is what makes the "current streak" (a walk back from
// the newest game) mean what it says.
func TestGetCompletedGamesIsOldestFirst(t *testing.T) {
	initTestDB(t)

	first := seed(t, "English", 5, "won")
	second := seed(t, "English", 5, "lost")
	third := seed(t, "English", 5, "won")

	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	want := []uint{first, second, third}
	if len(games) != len(want) {
		t.Fatalf("got %d games, want %d", len(games), len(want))
	}
	for i, id := range want {
		if games[i].ID != id {
			t.Errorf("games[%d].ID = %d, want %d", i, games[i].ID, id)
		}
	}
}

func TestGetCompletedGamesFiltersLangAndLength(t *testing.T) {
	initTestDB(t)

	seed(t, "English", 5, "won")
	seed(t, "English", 6, "lost")
	seed(t, "Spanish", 5, "won")

	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("got %d games for English/5, want 1", len(games))
	}

	// No length filter: both English games, either status.
	games, err = GetCompletedGames("English", 0)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("got %d games for English at any length, want 2", len(games))
	}
}

// The distribution is keyed by the attempt a game was won on, so it has to
// come from the games' own guess records.
func TestGetGuessDistribution(t *testing.T) {
	initTestDB(t)

	id := seed(t, "English", 5, "won")
	for attempt := 1; attempt <= 3; attempt++ {
		rec := GuessRecord{GameID: id, Attempt: attempt, Word: "words", States: "[]"}
		if err := CreateGuess(&rec); err != nil {
			t.Fatalf("CreateGuess: %v", err)
		}
	}

	dist, err := GetGuessDistribution([]uint{id})
	if err != nil {
		t.Fatalf("GetGuessDistribution: %v", err)
	}
	if dist[3] != 1 {
		t.Errorf("distribution = %v, want one game at 3 guesses", dist)
	}

	empty, err := GetGuessDistribution(nil)
	if err != nil {
		t.Fatalf("GetGuessDistribution(nil): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("GetGuessDistribution(nil) = %v, want empty", empty)
	}
}

// seedAt inserts a finished game with an explicit created_at, so a whole
// batch can share one timestamp.
func seedAt(t *testing.T, createdAt, status string) uint {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO games (created_at, lang, word_length, answer, status, max_guesses) VALUES (?, 'English', 5, 'words', ?, 6)`,
		createdAt, status,
	)
	if err != nil {
		t.Fatalf("insert game at %s: %v", createdAt, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return uint(id)
}

// SQLite's CURRENT_TIMESTAMP only has second resolution, so a run of games
// finished in the same second all carry the identical created_at. Ordering on
// that column alone leaves their order up to the query planner, and the
// current-streak walk reads whichever game happens to land last as the newest
// one. id has to break the tie.
func TestGetCompletedGamesOrdersTiedTimestampsByID(t *testing.T) {
	initTestDB(t)

	const sameSecond = "2024-05-01 12:00:00"
	var want []uint
	for range 6 {
		want = append(want, seedAt(t, sameSecond, "won"))
	}
	// One game a second later must still sort after all of them.
	want = append(want, seedAt(t, "2024-05-01 12:00:01", "lost"))

	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != len(want) {
		t.Fatalf("got %d games, want %d", len(games), len(want))
	}
	for i, id := range want {
		if games[i].ID != id {
			t.Fatalf("games[%d].ID = %d, want %d (order %v)", i, games[i].ID, id, gameIDs(games))
		}
	}
}

func gameIDs(games []Game) []uint {
	out := make([]uint, 0, len(games))
	for _, g := range games {
		out = append(out, g.ID)
	}
	return out
}

// Init is called once per process in the server, but once per case in tests.
// A second call has to adopt the new database rather than keep serving the
// first one (and leak its connection pool).
func TestInitRepointsAtANewDatabase(t *testing.T) {
	initTestDB(t)
	seed(t, "English", 5, "won")

	initTestDB(t)
	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != 0 {
		t.Errorf("second Init still sees %d games from the first database", len(games))
	}
}

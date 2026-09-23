package store

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// SQLite lets one connection write at a time. Without journal_mode=WAL and a
// busy_timeout, the second writer fails immediately with SQLITE_BUSY
// ("database is locked") instead of waiting its turn — so the moment two
// players act at once, creating a game or saving a guess returns HTTP 500.
//
// Every write here is one a request handler makes, so a single failure is a
// request a real player would have lost.
func TestConcurrentWritesDoNotFailBusy(t *testing.T) {
	initTestDB(t)

	const players, guessesEach = 24, 6

	var wg sync.WaitGroup
	errCh := make(chan error, players*(guessesEach+1))
	for range players {
		wg.Go(func() {
			g := Game{Lang: "English", WordLength: 5, Answer: "words", Status: "playing"}
			if err := CreateGame(&g); err != nil {
				errCh <- err
				return
			}
			for attempt := 1; attempt <= guessesEach; attempt++ {
				rec := GuessRecord{GameID: g.ID, Attempt: attempt, Word: "arose", States: `["absent"]`}
				if err := CreateGuess(&rec); err != nil {
					errCh <- err
					return
				}
				// Readers race the writers too: a read that can't get in is
				// a "game not found" the player sees mid-game.
				if _, err := GetGame(g.ID); err != nil {
					errCh <- err
					return
				}
			}
			if err := UpdateGameStatus(g.ID, "won"); err != nil {
				errCh <- err
			}
		})
	}
	wg.Wait()
	close(errCh)

	var failures []error
	for err := range errCh {
		failures = append(failures, err)
	}
	if len(failures) > 0 {
		t.Fatalf("%d of %d concurrent players hit a database error; first: %v",
			len(failures), players, failures[0])
	}

	games, err := GetCompletedGames("English", 5)
	if err != nil {
		t.Fatalf("GetCompletedGames: %v", err)
	}
	if len(games) != players {
		t.Errorf("%d games finished, want %d — writes were silently lost", len(games), players)
	}
}

// The pragmas only take effect if they are on the connection string, so check
// the DSN actually carries them rather than trusting the timing test above
// not to go green on a fast enough machine.
func TestDSNCarriesConcurrencyPragmas(t *testing.T) {
	got := dsn("/tmp/wordgo.db")
	for _, want := range []string{"busy_timeout", "journal_mode%28WAL%29"} {
		if !strings.Contains(got, want) {
			t.Errorf("dsn() = %q, missing %q", got, want)
		}
	}
}

// A missing game and a broken database are different answers: the handlers
// turn the first into a 404 and the second into a 500, and they can only tell
// them apart if GetGame says which it was.
func TestGetGameReportsNotFound(t *testing.T) {
	initTestDB(t)

	if _, err := GetGame(4242); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetGame(missing) = %v, want an error wrapping ErrNotFound", err)
	}

	id := seed(t, "English", 5, "playing")
	g, err := GetGame(id)
	if err != nil {
		t.Fatalf("GetGame(%d): %v", id, err)
	}
	if errors.Is(err, ErrNotFound) || g == nil {
		t.Fatalf("GetGame(%d) reported an existing game as missing", id)
	}
}

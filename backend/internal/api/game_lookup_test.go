package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"wordgo/internal/store"
)

// Every handler that loads a game used to answer "game not found" with a 404
// for *any* store failure, so a database that was merely locked told the
// client its game was gone — and a client that believes a 404 throws the
// player's board away. Only a genuinely missing game is a 404.
func TestGameLookupStatusSeparatesMissingFromBroken(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"missing game", fmt.Errorf("game 7: %w", store.ErrNotFound), http.StatusNotFound},
		{"bare not-found", store.ErrNotFound, http.StatusNotFound},
		{"database locked", errors.New("database is locked (5) (SQLITE_BUSY)"), http.StatusInternalServerError},
		{"scan failure", fmt.Errorf("query games: %w", errors.New("disk I/O error")), http.StatusInternalServerError},
	}
	for _, c := range cases {
		if got := gameLookupStatus(c.err); got != c.want {
			t.Errorf("%s: gameLookupStatus(%v) = %d, want %d", c.name, c.err, got, c.want)
		}
	}
}

// End to end: a game id nobody ever created is still a 404 on every route
// that loads one, and the message says so.
func TestMissingGameIsStillNotFound(t *testing.T) {
	seedWordListCache(t, "Vietnamese", 3, vietnameseWords())

	const missing = 999999
	mux := testMux()
	mux.HandleFunc("POST /api/cache/clear", HandleClearCache)

	cases := []struct {
		method, path string
		body         any
	}{
		{"GET", fmt.Sprintf("/api/game/%d", missing), nil},
		{"POST", fmt.Sprintf("/api/game/%d/guess", missing), map[string]string{"word": "bà"}},
		{"POST", "/api/cache/clear", map[string]any{"game_id": missing}},
	}
	for _, c := range cases {
		code, resp := do(t, mux, c.method, c.path, c.body)
		if code != http.StatusNotFound {
			t.Errorf("%s %s: HTTP %d, want 404 (%v)", c.method, c.path, code, resp)
			continue
		}
		msg, _ := resp["error"].(string)
		if !strings.Contains(msg, "not found") {
			t.Errorf("%s %s: error = %q, want it to say the game was not found", c.method, c.path, msg)
		}
	}
}

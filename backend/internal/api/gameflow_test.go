package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"wordgo/internal/store"
	"wordgo/internal/wordlist"
)

// This is an end-to-end pass over the real handlers: for each language, play a
// full game — two wrong words, then the answer on the third try — and check
// the responses the way a client would read them.
//
// Word lists are fetched the same way the server fetches them: a language not
// already cached is downloaded from kaikki.org on the spot, which is slow and
// large, so give the run a generous -timeout. Downloads land in backend/cache
// and are reused by later runs; the SQLite database is created there too.
//
// Every run writes backend/test-logs/gameflow-<timestamp>.log: the per-step
// trace below plus the handlers' own slog/log output (download progress, game
// created, game over), which `go test` otherwise only shows interleaved on
// stderr with no clue which language produced it.
//
// TestPlayDefaultLangs plays the handful of languages in defaultLangs;
// TestPlayEachLanguage plays every language in the kaikki.org index (thousands
// of dumps — expect hours and hundreds of GB), so it is normally run alone
// with -run and a long -timeout.

// defaultLangs is the short language set: one per script family the game has
// special handling for.
var defaultLangs = []string{
	"English",
	"Spanish",
	"Japanese",
	"Korean",
	"Russian",
	"Greek",
	"Hindi",
	"Arabic",
	"Chinese (Mandarin)",
}

// testLog receives everything the run logs: the file, and stderr so a plain
// `go test` still shows progress on a run that can last hours.
var testLog io.Writer = os.Stderr

func TestMain(m *testing.M) {
	// Word lists and the database live in the backend directory, so
	// downloaded dumps persist in backend/cache and a rerun doesn't fetch
	// them all over again.
	dir, err := filepath.Abs("../..")
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve data dir:", err)
		os.Exit(1)
	}
	os.Setenv("DATA_DIR", dir)

	logFile, err := openLogFile(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open log file:", err)
		os.Exit(1)
	}
	defer logFile.Close()
	testLog = io.MultiWriter(os.Stderr, logFile)

	// Capture the handlers' own logging too, so a failure's trace sits next
	// to the request that caused it.
	log.SetOutput(testLog)
	slog.SetDefault(slog.New(slog.NewTextHandler(testLog, &slog.HandlerOptions{Level: slog.LevelDebug})))
	fmt.Fprintf(testLog, "=== gameflow test log %s\n", time.Now().Format(time.RFC3339))

	store.Init()
	code := m.Run()
	logFile.Close()
	os.Exit(code)
}

// openLogFile creates this run's log under backend/test-logs.
func openLogFile(dir string) (*os.File, error) {
	logDir := filepath.Join(dir, "test-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(logDir, fmt.Sprintf("gameflow-%s.log", time.Now().Format("20060102-150405")))
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "gameflow test log:", path)
	return f, nil
}

// step records one step of a game: to the test's own output (visible under
// -v, and printed for a failing subtest either way) and to the run log,
// tagged with the subtest name so languages stay apart in the file.
func step(t *testing.T, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	t.Log(msg)
	fmt.Fprintf(testLog, "%s [%s] %s\n", time.Now().Format("15:04:05.000"), t.Name(), msg)
}

func testMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/languages", HandleGetLanguages)
	mux.HandleFunc("POST /api/game", HandleNewGame)
	mux.HandleFunc("GET /api/game/{id}", HandleGetGame)
	mux.HandleFunc("POST /api/game/{id}/guess", HandleGuess)
	return mux
}

// do runs one request against the handlers and decodes the JSON body.
func do(t *testing.T, mux *http.ServeMux, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: bad JSON (%v): %s", method, path, err, rec.Body.String())
	}
	return rec.Code, out
}

// TestPlayDefaultLangs plays the built-in language set — one language per
// script family the game handles specially.
func TestPlayDefaultLangs(t *testing.T) {
	runFlow(t, defaultLangs)
}

// TestPlayEachLanguage plays every language kaikki.org publishes. Each one
// not already cached is a full dump download, so this is a long run.
func TestPlayEachLanguage(t *testing.T) {
	runFlow(t, languageIndex(t))
}

// runFlow plays a whole game per language: two wrong guesses, then the answer.
func runFlow(t *testing.T, langs []string) {
	mux := testMux()
	known := languageIndex(t)
	step(t, "playing %d languages: %s", len(langs), strings.Join(langs, ", "))

	for _, lng := range langs {
		t.Run(lng, func(t *testing.T) {
			start := time.Now()
			playGame(t, mux, known, lng)
			step(t, "done in %s", time.Since(start).Round(time.Millisecond))
		})
	}
}

// languageIndex is the kaikki.org language list HandleNewGame validates
// against; it is fetched once over the network, and without it no game can be
// created at all.
func languageIndex(t *testing.T) []string {
	t.Helper()
	known := wordlist.GetCachedLanguages()
	step(t, "kaikki.org index lists %d languages", len(known))
	if len(known) == 0 {
		t.Skip("kaikki.org language index unavailable (offline?)")
	}
	return known
}

// playGame runs one language's full three-guess game against the handlers.
func playGame(t *testing.T, mux *http.ServeMux, known []string, lng string) {

	if !slices.Contains(known, lng) {
		t.Fatalf("language %q is not in the kaikki.org index", lng)
	}

	code, game := do(t, mux, "POST", "/api/game", map[string]string{"lang": lng})
	if code != http.StatusOK {
		t.Fatalf("new game: HTTP %d: %v", code, game["error"])
	}
	id := uint(game["id"].(float64))
	length := int(game["word_length"].(float64))
	step(t, "new game %d: %d-char words, keyboard %v, %d alphabet keys, rtl=%v",
		id, length, game["keyboard_layout"], len(game["alphabet"].([]any)), game["rtl"])
	if got := game["status"]; got != "playing" {
		t.Fatalf("new game status = %v, want playing", got)
	}
	if len(game["guesses"].([]any)) != 0 {
		t.Fatalf("new game already has guesses")
	}

	stored, err := store.GetGame(id)
	if err != nil {
		t.Fatalf("load game %d: %v", id, err)
	}
	answer := stored.Answer

	words, err := wordlist.GetCachedWordList(lng, length)
	if err != nil {
		t.Fatalf("word list: %v", err)
	}
	step(t, "word list has %d words; answer %q", len(words), answer)
	wrong := pickWrong(t, words, answer, 2)

	guessPath := fmt.Sprintf("/api/game/%d/guess", id)
	for i, w := range wrong {
		code, resp := do(t, mux, "POST", guessPath, map[string]string{"word": w})
		if code != http.StatusOK {
			t.Fatalf("guess %q: HTTP %d: %v", w, code, resp["error"])
		}
		if err, ok := resp["error"]; ok {
			t.Fatalf("guess %q rejected: %v", w, err)
		}
		if got, want := int(resp["attempt"].(float64)), i+1; got != want {
			t.Errorf("guess %q: attempt = %d, want %d", w, got, want)
		}
		if got := resp["status"]; got != "playing" {
			t.Fatalf("guess %q: status = %v, want playing (answer %q)", w, got, answer)
		}
		if allCorrect(t, resp["states"]) {
			t.Fatalf("wrong guess %q scored as the answer %q", w, answer)
		}
		if _, revealed := resp["answer"]; revealed {
			t.Errorf("guess %q: answer revealed while still playing", w)
		}
		step(t, "guess %d %q -> %v %v", i+1, w, resp["states"], resp["status"])
	}

	code, win := do(t, mux, "POST", guessPath, map[string]string{"word": answer})
	if code != http.StatusOK {
		t.Fatalf("winning guess: HTTP %d: %v", code, win["error"])
	}
	if got := int(win["attempt"].(float64)); got != 3 {
		t.Errorf("winning attempt = %d, want 3", got)
	}
	if got := win["status"]; got != "won" {
		t.Fatalf("status after answer %q = %v, want won", answer, got)
	}
	if !allCorrect(t, win["states"]) {
		t.Errorf("answer %q did not score all correct: %v", answer, win["states"])
	}
	if got := win["answer"]; got != answer {
		t.Errorf("revealed answer = %v, want %q", got, answer)
	}
	if win["message"] == nil {
		t.Errorf("no win message on attempt 3")
	}
	step(t, "guess 3 %q -> %v %v (%v)", answer, win["states"], win["status"], win["message"])

	code, final := do(t, mux, "GET", fmt.Sprintf("/api/game/%d", id), nil)
	if code != http.StatusOK {
		t.Fatalf("get game: HTTP %d: %v", code, final["error"])
	}
	if got := final["status"]; got != "won" {
		t.Errorf("reloaded status = %v, want won", got)
	}
	if got := len(final["guesses"].([]any)); got != 3 {
		t.Errorf("reloaded game has %d guesses, want 3", got)
	}
	step(t, "reloaded game %d: status %v, %d guesses", id, final["status"], len(final["guesses"].([]any)))

	if code, over := do(t, mux, "POST", guessPath, map[string]string{"word": answer}); code != http.StatusBadRequest {
		t.Errorf("guess after win: HTTP %d, want 400 (%v)", code, over)
	}
}

// pickWrong returns n word-list entries that are not the answer, chosen
// deterministically so a failure can be replayed.
func pickWrong(t *testing.T, words map[string]string, answer string, n int) []string {
	t.Helper()
	keys := make([]string, 0, len(words))
	for w := range words {
		if w != answer {
			keys = append(keys, w)
		}
	}
	sort.Strings(keys)
	if len(keys) < n {
		t.Fatalf("word list has %d words besides the answer, need %d", len(keys), n)
	}
	return keys[:n]
}

func allCorrect(t *testing.T, states any) bool {
	t.Helper()
	list, ok := states.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("missing tile states: %v", states)
	}
	for _, s := range list {
		if s != "correct" {
			return false
		}
	}
	return true
}

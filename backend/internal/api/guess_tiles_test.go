package api

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"wordgo/internal/lang"
	"wordgo/internal/store"
	"wordgo/internal/wordlist"
)

// vietnameseWords is a stand-in word list of three-tile Vietnamese words: two
// letters plus a tone mark, which lang.WordChars splits into its own tile.
// loadCachedWordList wants at least 20 entries before it trusts a cache file.
func vietnameseWords() map[string]string {
	words := map[string]string{}
	for _, base := range []string{"b", "c", "đ", "g", "h", "l", "m", "n", "r", "t", "v", "x"} {
		for _, vowel := range []string{"à", "á"} {
			words[base+vowel] = "a " + base + vowel
		}
	}
	return words
}

// seedWordListCache writes a word list to the on-disk cache and points the
// process at a throwaway cache directory, so the handlers can be driven
// without downloading anything. The store is repointed too, and put back
// afterwards, so the test's games don't land in the shared database.
func seedWordListCache(t *testing.T, lng string, length int, words map[string]string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	store.Init()
	t.Cleanup(store.Init) // reopen the database TestMain set up

	data, err := json.Marshal(words)
	if err != nil {
		t.Fatalf("marshal word list: %v", err)
	}
	safe := strings.ToLower(strings.ReplaceAll(lng, " ", "_"))
	path := fmt.Sprintf("%s/cache/%s_%dl.json", dir, safe, length)
	if err := os.MkdirAll(dir+"/cache", 0755); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write word list cache: %v", err)
	}
	t.Cleanup(func() { wordlist.ClearWordListCache(wordlist.Key{Lang: lng, Len: length}) })
}

// A Vietnamese guess arrives as the tiles the keyboard produced — "cá" is
// typed as c, a, and the acute-tone key, and submitted as "ca´". The handler
// used to validate those runes with lang.IsWordChar, which files "´" (and
// "`", "~", ".") as a symbol rather than a letter, so every toned guess came
// back as HTTP 400 "word contains invalid characters".
func TestGuessAcceptsToneTilesAsTyped(t *testing.T) {
	const lng, length = "Vietnamese", 3
	words := vietnameseWords()
	seedWordListCache(t, lng, length, words)

	game := store.Game{Lang: lng, WordLength: length, Answer: "bà", Status: "playing"}
	if err := store.CreateGame(&game); err != nil {
		t.Fatalf("create game: %v", err)
	}

	const canonical = "cá"
	typed := strings.Join(lang.WordChars(canonical, lang.ToneSplitKind(lng)), "")
	if typed == canonical {
		t.Fatalf("%q is not tone-split; the test is not exercising the tile path", canonical)
	}

	mux := testMux()
	code, resp := do(t, mux, "POST", fmt.Sprintf("/api/game/%d/guess", game.ID), map[string]string{"word": typed})
	if code != http.StatusOK {
		t.Fatalf("guess %q: HTTP %d: %v", typed, code, resp["error"])
	}
	if err, ok := resp["error"]; ok {
		t.Fatalf("guess %q rejected: %v", typed, err)
	}
	if got := resp["word"]; got != canonical {
		t.Errorf("guess %q resolved to %v, want the canonical %q", typed, got, canonical)
	}
	tiles, _ := resp["tiles"].([]any)
	if len(tiles) != length {
		t.Errorf("guess %q came back as %v, want %d tiles", typed, resp["tiles"], length)
	}
}

// The wildcard and the tone tiles are the only non-letters a guess may carry;
// anything else still has to be turned away.
func TestGuessStillRejectsJunkCharacters(t *testing.T) {
	const lng, length = "Vietnamese", 3
	seedWordListCache(t, lng, length, vietnameseWords())

	game := store.Game{Lang: lng, WordLength: length, Answer: "bà", Status: "playing"}
	if err := store.CreateGame(&game); err != nil {
		t.Fatalf("create game: %v", err)
	}

	code, resp := do(t, testMux(), "POST", fmt.Sprintf("/api/game/%d/guess", game.ID), map[string]string{"word": "a1;"})
	if code != http.StatusBadRequest {
		t.Fatalf("guess \"a1;\": HTTP %d, want 400 (%v)", code, resp)
	}
}

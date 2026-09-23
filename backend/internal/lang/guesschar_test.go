package lang

import (
	"slices"
	"strings"
	"testing"
)

// A Vietnamese guess reaches the server as the tiles the on-screen keyboard
// produced, not as the composed word: "ắn" is typed as ă, the acute tone key,
// then n, and submitted as "ă´n". Validating those runes with IsWordChar
// rejected four of the five tones outright ("`", "´", "~", "." are symbols or
// punctuation to Unicode), so no toned Vietnamese guess could be played.
func TestIsGuessCharAcceptsEveryToneTile(t *testing.T) {
	for kind, table := range toneTranslationsByKind {
		for mark, tile := range table {
			for _, r := range tile {
				if !IsGuessChar(r) {
					t.Errorf("%s: tone tile %q (for U+%04X) rejected by IsGuessChar", kind, tile, mark)
				}
			}
		}
	}
}

// IsGuessChar only widens IsWordChar; it must not start accepting the digits
// and separators IsWord exists to keep out of a word list.
func TestIsGuessCharRejectsNonWordPunctuation(t *testing.T) {
	for _, r := range []rune{'1', ' ', ',', ';', '/', '(', '\n'} {
		if IsGuessChar(r) {
			t.Errorf("IsGuessChar(%q) = true, want false", r)
		}
	}
	for _, r := range []rune{'a', 'ă', 'ㄱ', 'あ', 'ˊ'} {
		if !IsGuessChar(r) {
			t.Errorf("IsGuessChar(%q) = false, want true", r)
		}
	}
}

// Widening validation is only half of it: the tiles joined back into a string
// have to normalize to the same key BuildNormalizedSet filed the canonical
// word under, or the guess resolves to nothing and comes back "Not in word
// list" instead.
func TestToneTilesResolveToTheCanonicalWord(t *testing.T) {
	const toneLang = "vietnamese"
	words := map[string]string{"ắn": "", "ăn": "", "bàn": ""}
	normSet := BuildNormalizedSet(words, toneLang)

	for canonical := range words {
		tiles := WordChars(canonical, toneLang)
		typed := strings.Join(tiles, "")

		for _, r := range typed {
			if !IsGuessChar(r) {
				t.Fatalf("%q: typed form %q has rune %q the guess handler would reject", canonical, typed, r)
			}
		}
		if got := WordChars(typed, toneLang); !slices.Equal(got, tiles) {
			t.Errorf("%q: retiling the typed form gave %q, want %q", canonical, got, tiles)
		}
		if got := normSet[NormalizeWord(typed, toneLang)]; got != canonical {
			t.Errorf("%q typed as %q resolved to %q", canonical, typed, got)
		}
	}
}

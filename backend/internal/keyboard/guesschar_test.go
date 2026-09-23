package keyboard

import (
	"testing"

	"wordgo/internal/lang"
)

// Every key a layout offers has to be something a guess may contain. The
// Vietnamese tone keys ("`", "´", "~", ".") were not: the guess handler
// validated with lang.IsWordChar, which files them as punctuation, so every
// guess carrying a tone was rejected with "word contains invalid characters".
func TestEveryLayoutKeyIsAGuessChar(t *testing.T) {
	for name, rows := range keyboardLayouts {
		for _, row := range rows {
			for _, key := range row {
				for _, r := range key {
					if !lang.IsGuessChar(r) {
						t.Errorf("layout %q: key %q has rune %q that a guess may not contain", name, key, r)
					}
				}
			}
		}
	}
}

// A syllabary layout only changes the pre-download length estimate if some
// language actually resolves to it — there is no word list to detect from at
// that point, so it has to come from an override.
func TestEverySyllabaryLayoutIsReachableByOverride(t *testing.T) {
	reachable := map[string]bool{}
	for _, lng := range []string{
		"English", "French", "German", "Amharic", "Tigrinya", "Japanese",
		"Cherokee", "Inuktitut", "Vietnamese",
		"Chinese", "Chinese (Mandarin)", "Chinese (Cangjie)", "Chinese (Zhuyin)",
	} {
		reachable[resolveLayoutOverride(lng)] = true
	}
	for _, name := range syllabaryLayouts {
		if !reachable[name] {
			t.Errorf("no language resolves to syllabary layout %q, so its 4-tile estimate is dead code", name)
		}
	}
}

func TestDefaultLengthForSyllabaryLangs(t *testing.T) {
	for _, lng := range []string{"Japanese", "Amharic", "Cherokee", "Inuktitut", "Chinese (Cangjie)"} {
		if got := DefaultLengthForLang(lng); got != 4 {
			t.Errorf("DefaultLengthForLang(%q) = %d, want 4", lng, got)
		}
	}
}

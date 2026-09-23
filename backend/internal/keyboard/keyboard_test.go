package keyboard

import (
	"strings"
	"testing"

	"wordgo/internal/lang"
)

func TestDefaultLengthForLang(t *testing.T) {
	cases := []struct {
		lang string
		want int
	}{
		{"English", 6},
		{"French", 6},
		{"Japanese", 4},           // syllabary (hiragana/moras)
		{"Chinese (Cangjie)", 4},  // root-glyph code length
		{"Chinese (Mandarin)", 6}, // romanization letters only, no tone tiles
		{"Vietnamese", 8},         // tone marks take their own tile
		// Korean isn't in langLayoutMap (only detectLayout, run later once the
		// word list is available, would recognize it as "korean") so at this
		// pre-fetch stage it falls through to the 6-tile default like a
		// script-detection miss would.
		{"Korean", 6},
	}
	for _, c := range cases {
		if got := DefaultLengthForLang(c.lang); got != c.want {
			t.Errorf("DefaultLengthForLang(%q) = %d, want %d", c.lang, got, c.want)
		}
	}
}

func TestResolveLayoutOverride(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"English", "qwerty"},
		{"French", "azerty"},
		{"German", "qwertz"},
		{"Japanese", "hiragana"},
		{"Vietnamese", "vietnamese"},
		{"Chinese (Cangjie)", "cangjie"},
		{"Chinese (Zhuyin)", "zhuyin"},
		{"Chinese (Mandarin)", "chinese"},
		{"Spanish", ""}, // no override; falls back to detectLayout
	}
	for _, c := range cases {
		if got := resolveLayoutOverride(c.lang); got != c.want {
			t.Errorf("resolveLayoutOverride(%q) = %q, want %q", c.lang, got, c.want)
		}
	}
}

// Every layout key is a distinct keystroke: a key listed twice renders two
// identical buttons and makes the layout's coverage look larger than it is.
func TestLayoutsHaveNoDuplicateKeys(t *testing.T) {
	for name, rows := range keyboardLayouts {
		seen := map[string]bool{}
		for _, row := range rows {
			for _, key := range row {
				if seen[key] {
					t.Errorf("layout %q lists key %q more than once", name, key)
				}
				seen[key] = true
			}
		}
	}
}

// Word lists are lowercased before the alphabet is built (wordlist.streamURL),
// so a layout written in capitals matches nothing: every letter falls through
// to the overflow "*" key and the language is left with no keyboard at all.
// Cherokee is the deliberate exception — its words are kept upper-case
// because the lower-case block isn't real orthography.
func TestLayoutKeysAreLowercase(t *testing.T) {
	for name, rows := range keyboardLayouts {
		if name == "cherokee" {
			continue
		}
		for _, row := range rows {
			for _, key := range row {
				if lower := strings.ToLower(key); lower != key {
					t.Errorf("layout %q key %q is upper-case (want %q); word lists are lowercased", name, key, lower)
				}
			}
		}
	}
}

// BuildKeyboardData has to place a script's own letters on its own layout.
// Armenian regressed here once: the preset was written in capitals, so
// nothing matched and the whole alphabet became overflow.
func TestBuildKeyboardDataPlacesNativeLetters(t *testing.T) {
	cases := []struct {
		name  string
		words map[string]string
	}{
		{"armenian", map[string]string{"գիրք": "", "սիրտ": "", "դպրոց": "", "մարդ": ""}},
		{"greek", map[string]string{"λογος": "", "κοσμος": "", "φωνη": ""}},
		{"jcuken", map[string]string{"слово": "", "книга": "", "мир": ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alphabet := lang.BuildAlphabet(c.words, "")
			rows, overflow, placed := BuildKeyboardData(alphabet, "", c.words)
			if len(rows) == 0 {
				t.Fatalf("no keyboard rows built; every letter went to overflow %v", overflow)
			}
			if len(overflow) != 0 {
				t.Errorf("overflow = %v, want none: every letter should sit on a key", overflow)
			}
			for _, ch := range alphabet {
				if !placed[ch] {
					t.Errorf("letter %q was not placed on a key", ch)
				}
			}
		})
	}
}

// detectLayout samples the word list, and Go randomizes map iteration — so
// sampling the map directly could pick a different layout for the same words
// on every process start. The sample is sorted to keep the choice stable.
func TestDetectLayoutIsDeterministic(t *testing.T) {
	// More words than the sample size, so the sampling step actually bites,
	// and mixed enough that an unstable sample could see different scripts.
	words := map[string]string{}
	for _, w := range []string{
		"ασπρο", "βιβλιο", "γαλα", "δρομος", "ελπιδα", "ζωη", "ηλιος", "θαλασσα",
		"ιδεα", "καρδια", "λογος", "μητερα", "νερο", "ξενος", "ουρανος", "πατερας",
		"ρευμα", "σπιτι", "τραπεζι", "υγεια", "φιλος", "χρονος", "ψωμι", "ωρα",
		"ανθρωπος", "γραμμα", "δεντρο", "εικονα", "ζαχαρη", "θεατρο", "κοσμος",
		"μουσικη", "νησι", "παιδι", "ρολοι", "φωτια",
	} {
		words[w] = ""
	}

	first := detectLayout(words)
	if first != "greek" {
		t.Fatalf("detectLayout = %q, want %q", first, "greek")
	}
	for range 50 {
		if got := detectLayout(words); got != first {
			t.Fatalf("detectLayout is unstable: got %q, then %q", first, got)
		}
	}
}

// An empty/unknown word list has no distinguishing characters, so detection
// stays on the qwerty default rather than picking an arbitrary layout.
func TestDetectLayoutDefaultsToQwerty(t *testing.T) {
	if got := detectLayout(map[string]string{"hello": "", "world": ""}); got != "qwerty" {
		t.Errorf("detectLayout(ascii) = %q, want qwerty", got)
	}
	if got := detectLayout(nil); got != "qwerty" {
		t.Errorf("detectLayout(nil) = %q, want qwerty", got)
	}
}

// BuildGameExtras must report the same layout it built the rows from.
func TestBuildGameExtrasLayoutMatchesRows(t *testing.T) {
	words := map[string]string{"λογος": "", "κοσμος": "", "φωνη": ""}
	alphabet := lang.BuildAlphabet(words, "")
	rows, _, _, rtl, _, layoutName := BuildGameExtras(alphabet, "", words)
	if layoutName != "greek" {
		t.Errorf("layoutName = %q, want greek", layoutName)
	}
	if len(rows) == 0 {
		t.Error("no keyboard rows built")
	}
	if rtl {
		t.Error("rtl = true for a Greek word list")
	}
}

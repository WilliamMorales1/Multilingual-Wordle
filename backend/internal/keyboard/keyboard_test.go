package keyboard

import "testing"

func TestDefaultLengthForLang(t *testing.T) {
	cases := []struct {
		lang string
		want int
	}{
		{"English", 6},
		{"French", 6},
		{"Japanese", 4},          // syllabary (hiragana/moras)
		{"Chinese (Cangjie)", 4}, // root-glyph code length
		{"Chinese (Mandarin)", 8},
		{"Vietnamese", 8},
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

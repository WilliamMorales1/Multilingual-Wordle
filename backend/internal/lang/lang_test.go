package lang

import (
	"reflect"
	"testing"
)

func TestWordChars(t *testing.T) {
	cases := []struct {
		name     string
		word     string
		toneLang string
		want     []string
	}{
		{"ascii", "hello", "", []string{"h", "e", "l", "l", "o"}},
		{"accented base merges diacritic", "café", "", []string{"c", "a", "f", "é"}},
		{"devanagari matra splits into its own tile", "का", "", []string{"क", "ा"}},
		{"vietnamese tone splits into its own tile", "tiếng", "vietnamese", []string{"t", "i", "ê", "´", "n", "g"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WordChars(c.word, c.toneLang)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("WordChars(%q, %q) = %v, want %v", c.word, c.toneLang, got, c.want)
			}
		})
	}
}

func TestVietnameseToneSplitTranslatesMark(t *testing.T) {
	// "tiếng" carries an acute (sắc) tone mark over ế — WordChars should
	// pull it off as its own tile using the ASCII-safe translation, not the
	// bare combining mark.
	chars := WordChars("tiếng", "vietnamese")
	found := false
	for _, c := range chars {
		if c == "´" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tone tile %q in %v", "´", chars)
	}
}

func TestNormalizeChar(t *testing.T) {
	cases := []struct{ in, want string }{
		{"é", "e"},
		{"É", "e"},
		{"ñ", "n"},
		{"が", "か"}, // dakuten strips
		{"ぱ", "は"}, // handakuten strips
		{"ゃ", "や"}, // small kana collapses to large
		{"ा", "आ"}, // lone devanagari matra normalizes to its independent vowel
	}
	for _, c := range cases {
		if got := NormalizeChar(c.in); got != c.want {
			t.Errorf("NormalizeChar(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name          string
		guess, answer string
		want          []string
	}{
		{"all correct", "abc", "abc", []string{"correct", "correct", "correct"}},
		{"all absent", "xyz", "abc", []string{"absent", "absent", "absent"}},
		{"present letters", "bca", "abc", []string{"present", "present", "present"}},
		{
			"duplicate guess letter, single answer letter",
			"aab", "abc",
			[]string{"correct", "absent", "present"},
		},
		{
			"duplicate answer letter, single correct match",
			"axxb", "aabb",
			[]string{"correct", "absent", "absent", "correct"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := WordChars(c.guess, "")
			a := WordChars(c.answer, "")
			got := Evaluate(g, a)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Evaluate(%q, %q) = %v, want %v", c.guess, c.answer, got, c.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	cases := []struct {
		word     string
		length   int
		toneLang string
		want     bool
	}{
		{"hello", 5, "", true},
		{"hello", 4, "", false},
		{"café", 4, "", true},
		{"h3llo", 5, "", false}, // digit isn't a word char
	}
	for _, c := range cases {
		if got := IsValid(c.word, c.length, c.toneLang); got != c.want {
			t.Errorf("IsValid(%q, %d) = %v, want %v", c.word, c.length, got, c.want)
		}
	}
}

func TestChineseToneifyMandarin(t *testing.T) {
	// NFD-composed pinyin with combining tone marks over "a" (macron=1) and
	// "e" (acute=2) folds each into a trailing numeral tile.
	got := ChineseToneify("Mandarin", "āé") // ā é (precomposed) folds via NFD
	want := "a1e2"
	if got != want {
		t.Errorf("ChineseToneify(Mandarin) = %q, want %q", got, want)
	}
}

func TestChineseToneifyDialect(t *testing.T) {
	cases := []struct {
		name, dialect, rom, want string
	}{
		{"single syllable with tone", "Cantonese", "aa1", "aa1"},
		{"fused syllables recover both tones", "Cantonese", "aa1het6", "aa1het6"},
		{"space-separated syllables", "Hokkien", "chiah4 png7", "chiah4png7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ChineseToneify(c.dialect, c.rom); got != c.want {
				t.Errorf("ChineseToneify(%q, %q) = %q, want %q", c.dialect, c.rom, got, c.want)
			}
		})
	}
}

func TestMatchWildcardIsDeterministic(t *testing.T) {
	normSet := map[string]string{
		"cot": "cot",
		"cat": "cat",
		"cut": "cut",
	}
	overflow := map[string]bool{"a": true, "o": true, "u": true}
	guessChars := []string{"c", "*", "t"}

	first := MatchWildcard(guessChars, normSet, overflow, "")
	for i := 0; i < 20; i++ {
		if got := MatchWildcard(guessChars, normSet, overflow, ""); got != first {
			t.Fatalf("MatchWildcard is nondeterministic: got %q then %q", first, got)
		}
	}
	if first != "cat" {
		t.Errorf("MatchWildcard picked %q, want lexicographically smallest match %q", first, "cat")
	}
}

package lang

import (
	"slices"
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
			if !slices.Equal(got, c.want) {
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

func TestZhuyinToneMarksAreWordChars(t *testing.T) {
	// A zhuyin reading keeps its tone marks as tiles, so IsValid must accept
	// them — ˙ (neutral tone) is a Unicode symbol, not a letter.
	for _, r := range []rune{'ˊ', 'ˇ', 'ˋ', '˙'} {
		if !IsWordChar(r) {
			t.Errorf("IsWordChar(%q) = false, want true", r)
		}
	}
	if !IsValid("ㄇㄚˊ", 3, "") {
		t.Errorf("IsValid(%q, 3) = false, want true", "ㄇㄚˊ")
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
			if !slices.Equal(got, c.want) {
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

func TestChineseRomanizeMandarin(t *testing.T) {
	cases := []struct{ rom, want string }{
		{"āé", "ae"},        // pinyin tone diacritics drop
		{"lǜ", "lü"},        // ü keeps its diaeresis, loses the tone
		{"nǐ hǎo", "nihao"}, // separators drop
	}
	for _, c := range cases {
		if got := ChineseRomanize("Mandarin", c.rom); got != c.want {
			t.Errorf("ChineseRomanize(Mandarin, %q) = %q, want %q", c.rom, got, c.want)
		}
	}
}

func TestChineseRomanizeDialect(t *testing.T) {
	cases := []struct {
		name, dialect, rom, want string
	}{
		{"single syllable", "Cantonese", "aa1", "aa"},
		{"fused syllables", "Cantonese", "aa1het6", "aahet"},
		{"space-separated syllables", "Hokkien", "chiah4 png7", "chiahpng"},
		{"superscript tone numerals", "Wu", "gu¹teq⁴", "guteq"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ChineseRomanize(c.dialect, c.rom); got != c.want {
				t.Errorf("ChineseRomanize(%q, %q) = %q, want %q", c.dialect, c.rom, got, c.want)
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
	for range 20 {
		if got := MatchWildcard(guessChars, normSet, overflow, ""); got != first {
			t.Fatalf("MatchWildcard is nondeterministic: got %q then %q", first, got)
		}
	}
	if first != "cat" {
		t.Errorf("MatchWildcard picked %q, want lexicographically smallest match %q", first, "cat")
	}
}

// Several words can normalize to the same accent-stripped key. Whichever one
// wins must not depend on Go's randomized map iteration, or the same word
// list resolves an accent-free guess to a different word in every process.
func TestBuildNormalizedSetIsDeterministic(t *testing.T) {
	words := map[string]string{
		"café": "", "cafe": "", "çafe": "",
		"élan": "", "elan": "",
		"naive": "", "naïve": "",
	}

	first := BuildNormalizedSet(words, "")
	if got := first["cafe"]; got != "cafe" {
		t.Errorf("normalized %q resolved to %q, want the lexicographically smallest %q", "cafe", got, "cafe")
	}
	for range 50 {
		got := BuildNormalizedSet(words, "")
		if len(got) != len(first) {
			t.Fatalf("set size changed: %d then %d", len(first), len(got))
		}
		for k, v := range first {
			if got[k] != v {
				t.Fatalf("normalized %q resolved to %q, then to %q", k, v, got[k])
			}
		}
	}
}

// Evaluate indexes the guess by the answer's tile count. Callers validate the
// count first, but a mismatch should not take the server down with it.
func TestEvaluateShortGuessDoesNotPanic(t *testing.T) {
	states := Evaluate([]string{"c", "a"}, []string{"c", "a", "f", "e"})
	if len(states) != 4 {
		t.Fatalf("got %d states, want 4 (one per answer tile)", len(states))
	}
	if states[0] != "correct" || states[1] != "correct" {
		t.Errorf("states[:2] = %v, want both correct", states[:2])
	}
	if states[2] != "absent" || states[3] != "absent" {
		t.Errorf("missing tiles = %v, want absent", states[2:])
	}
}

func TestIsPureHiragana(t *testing.T) {
	cases := []struct {
		word string
		want bool
	}{
		{"ねこ", true},
		{"がっこう", true}, // composed dakuten is a hiragana letter of its own
		{"ゝ", true},    // iteration mark
		{"", false},
		{"ネコ", false},   // katakana
		{"日本", false},   // kanji
		{"ラーメン", false}, // katakana with a prolonged-sound mark
		// The prolonged-sound mark is shared by both kana scripts, so the
		// hiragana form of a long-vowel loanword is still pure hiragana.
		{"らーめん", true},
		{"こーひー", true},
		// A bare combining/standalone dakuten is no tile anyone can type, and
		// U+309F ゟ is a vertical digraph, not a kana key.
		{"゙", false},
		{"か゛", false},
		{"ゟ", false},
	}
	for _, c := range cases {
		if got := IsPureHiragana(c.word); got != c.want {
			t.Errorf("IsPureHiragana(%q) = %v, want %v", c.word, got, c.want)
		}
	}
}

// Each tone mark is its own keystroke in a Vietnamese IME, so a cluster that
// somehow carries two of them owes two tiles, not one.
func TestWordCharsSplitsEveryToneMark(t *testing.T) {
	// a + hook above (hỏi) + dot below (nặng), decomposed. NFD reorders
	// the marks by canonical combining class — the dot below (220) sorts
	// ahead of the hook above (230) — so that is the order the tiles take.
	got := WordChars("a\u0309\u0323", "vietnamese")
	want := []string{"a", ".", "ˀ"}
	if !slices.Equal(got, want) {
		t.Errorf("WordChars = %v, want %v", got, want)
	}
}

// WordLen counts tiles, which is what the board and the length check use.
func TestWordLenCountsTiles(t *testing.T) {
	cases := []struct {
		word, toneLang string
		want           int
	}{
		{"hello", "", 5},
		{"café", "", 4},            // the accent rides along on its base letter
		{"का", "", 2},              // consonant + matra are separate tiles
		{"tiếng", "vietnamese", 6}, // t i ê ´ n g
	}
	for _, c := range cases {
		if got := WordLen(c.word, c.toneLang); got != c.want {
			t.Errorf("WordLen(%q, %q) = %d, want %d", c.word, c.toneLang, got, c.want)
		}
	}
}

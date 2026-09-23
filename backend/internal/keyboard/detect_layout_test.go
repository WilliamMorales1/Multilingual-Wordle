package keyboard

import (
	"fmt"
	"slices"
	"testing"

	"wordgo/internal/lang"
)

// A dump is not written in one script: kaikki.org's Arabic list carries 15
// Hebrew-script entries among 12,933 Arabic ones (Judeo-Arabic spellings).
// Detection used to score only the 30 lexicographically smallest words, and
// Hebrew (U+05xx) sorts before Arabic (U+06xx) — so all 30 were Hebrew, every
// Arabic game came back with a Hebrew keyboard, and the entire Arabic
// alphabet fell into the "*" overflow key.
func TestDetectLayoutIgnoresAForeignScriptMinority(t *testing.T) {
	words := map[string]string{}
	// The minority, spelled so it sorts first.
	for _, w := range []string{"אלמא", "בצלה", "חכמה", "טלסם", "כלמה", "לילה", "מחבה", "נהאר", "שאהד"} {
		words[w] = ""
	}
	// The language itself: far more words, but none of them sort early.
	for _, w := range []string{"كتاب", "مدرسة", "شمس", "قمر", "بيت", "ماء", "نار", "باب", "قلم", "ولد"} {
		for i := range 40 {
			words[w+fmt.Sprint(i)] = "" // distinct keys, same letters
		}
	}

	got := detectLayout(words)
	if got != "arabic" {
		t.Fatalf("detectLayout = %q, want arabic (%d Hebrew words outvoted the rest)", got, 9)
	}
	for range 20 {
		if again := detectLayout(words); again != got {
			t.Fatalf("detectLayout is unstable: %q then %q", got, again)
		}
	}
}

// The mirror case: a mostly-Latin language with a minority in another script
// (Yoruba's word list is 77% Latin and 10% Arabic-script Ajami) has to stay
// on qwerty. Scoring only the characters qwerty *cannot* type hands the list
// to the minority script, because the majority scores nothing at all.
func TestDetectLayoutKeepsQwertyForALatinMajority(t *testing.T) {
	words := map[string]string{}
	for _, w := range []string{"aago", "aajo", "aake", "bata", "dide", "kule", "omode", "owuro"} {
		for i := range 40 {
			words[w+fmt.Sprint(i)] = ""
		}
	}
	for _, w := range []string{"كتاب", "مدرسة", "شمس", "قمر"} {
		for i := range 8 {
			words[w+fmt.Sprint(i)] = ""
		}
	}
	if got := detectLayout(words); got != "qwerty" {
		t.Errorf("detectLayout = %q, want qwerty (the Arabic-script minority won)", got)
	}
}

// A layout that only exists for one language's override must never be picked
// by detection: its extra keys mean nothing elsewhere. Romanian's ă/â used to
// pull in the Vietnamese layout, dead tone keys and all.
func TestDetectLayoutSkipsOverrideOnlyLayouts(t *testing.T) {
	words := map[string]string{}
	for _, w := range []string{"mânca", "brânză", "câine", "păine", "sănătate", "tânăr"} {
		for i := range 40 {
			words[w+fmt.Sprint(i)] = ""
		}
	}
	got := detectLayout(words)
	if overrideOnlyLayouts[got] {
		t.Errorf("detectLayout = %q, an override-only layout", got)
	}
	if got != "qwerty" {
		t.Errorf("detectLayout = %q, want qwerty", got)
	}
}

// A genuine diacritic of the language's own — Danish å/ø/æ — is still worth
// switching for, or the threshold would have turned detection off entirely.
func TestDetectLayoutStillSwitchesForRealDiacritics(t *testing.T) {
	words := map[string]string{}
	for _, w := range []string{"håbet", "søster", "ærlig", "blåt", "grøn", "væske"} {
		for i := range 40 {
			words[w+fmt.Sprint(i)] = ""
		}
	}
	if got := detectLayout(words); got != "nordic" {
		t.Errorf("detectLayout = %q, want nordic", got)
	}
}

// The consequence the player sees: with the wrong layout, none of the
// language's letters find a key and the whole alphabet collapses into the
// single "*" overflow bucket.
func TestDetectedLayoutActuallyCarriesTheAlphabet(t *testing.T) {
	words := map[string]string{}
	for _, w := range []string{"אלמא", "בצלה", "חכמה"} {
		words[w] = ""
	}
	for _, w := range []string{"كتاب", "مدرسة", "شمس", "قمر", "بيت", "قلم"} {
		for i := range 40 {
			words[w+fmt.Sprint(i)] = ""
		}
	}

	alphabet := lang.BuildAlphabet(words, "")
	rows, overflow, placed := BuildKeyboardData(alphabet, "Arabic", words)

	placedCount := 0
	for _, row := range rows {
		placedCount += len(row)
	}
	if placedCount == 0 {
		t.Fatalf("no alphabet char reached a key; %d overflow bases", len(overflow))
	}
	for _, ch := range []string{"ك", "ت", "ا", "ب", "م", "ق"} {
		if !placed[ch] {
			t.Errorf("Arabic letter %q has no key (overflow bases: %v)", ch, overflow)
		}
		if slices.Contains(overflow, ch) {
			t.Errorf("Arabic letter %q was pushed into the \"*\" overflow bucket", ch)
		}
	}
}

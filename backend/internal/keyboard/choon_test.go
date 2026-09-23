package keyboard

import (
	"slices"
	"testing"
)

// A Japanese word list carries ー in every long-vowel loanword. It has to be a
// key of the hiragana layout: without it the mark is an unplaced alphabet
// char, so it lands in the overflow "*" bucket even though the flick keyboard
// already draws a dedicated ー key for it.
func TestHiraganaLayoutHasTheChoonKey(t *testing.T) {
	keys := layoutKeySets()["hiragana"]
	if !keys["ー"] {
		t.Fatal(`hiragana layout has no "ー" key`)
	}

	words := map[string]string{"らーめん": "", "こーひー": "", "ねこ": "", "すーぱー": ""}
	alphabet := []string{"ー", "こ", "す", "ね", "は", "ひ", "め", "ら", "ぱ", "ん"}

	rows, overflow, placed := BuildKeyboardData(alphabet, "Japanese", words)
	if !placed["ー"] {
		t.Errorf(`"ー" was not placed on a key; overflow bases = %v`, overflow)
	}
	if slices.Contains(overflow, "ー") {
		t.Errorf(`"ー" landed in the overflow bucket: %v`, overflow)
	}
	if !slices.ContainsFunc(rows, func(r []string) bool { return slices.Contains(r, "ー") }) {
		t.Errorf(`no keyboard row carries "ー": %v`, rows)
	}
}

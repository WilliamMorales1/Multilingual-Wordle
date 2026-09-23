package lang

import "testing"

// KatakanaToHiragana leaves ー (U+30FC) alone because the mark belongs to both
// kana scripts. IsPureHiragana therefore has to accept it: rejecting it threw
// away every long-vowel loanword the converter had just produced, so no word
// with a long vowel could ever be the answer or a valid guess — while the kana
// keyboard still drew a ー key that could not spell anything.
func TestKatakanaLoanwordsSurviveTheHiraganaFilter(t *testing.T) {
	cases := []struct{ katakana, hiragana string }{
		{"ラーメン", "らーめん"},
		{"コーヒー", "こーひー"},
		{"ケーキ", "けーき"},
		{"スーパー", "すーぱー"},
	}
	for _, c := range cases {
		got := KatakanaToHiragana(c.katakana)
		if got != c.hiragana {
			t.Errorf("KatakanaToHiragana(%q) = %q, want %q", c.katakana, got, c.hiragana)
		}
		if !IsPureHiragana(got) {
			t.Errorf("IsPureHiragana(%q) = false: %q is dropped from the word list", got, c.katakana)
		}
	}
}

// ー is one tile, and one that survives normalization unchanged — the
// keyboard key and the aggregated key state have to agree on its spelling.
func TestChoonMarkIsItsOwnTile(t *testing.T) {
	if got := WordChars("らーめん", ""); len(got) != 4 || got[1] != "ー" {
		t.Errorf(`WordChars("らーめん") = %q, want 4 tiles with "ー" second`, got)
	}
	if got := NormalizeChar("ー"); got != "ー" {
		t.Errorf(`NormalizeChar("ー") = %q, want "ー"`, got)
	}
	if !IsWordChar(ChoonMark) {
		t.Error("IsWordChar(ー) = false, so IsWord rejects every word carrying one")
	}
}

// Widening the filter for ー must not let the rest of the katakana block in:
// a katakana leftover means the conversion missed something.
func TestChoonMarkDoesNotAdmitKatakana(t *testing.T) {
	for _, w := range []string{"ラー", "ーメン", "ーン"} {
		if IsPureHiragana(w) {
			t.Errorf("IsPureHiragana(%q) = true, want false (katakana leftover)", w)
		}
	}
}

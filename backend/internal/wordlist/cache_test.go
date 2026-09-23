package wordlist

import (
	"testing"
)

func sampleWords() map[string]string {
	words := map[string]string{}
	for _, w := range []string{
		"apple", "bread", "chair", "dream", "eagle", "flame", "grape", "house",
		"ivory", "jelly", "knife", "lemon", "money", "night", "ocean", "piano",
		"queen", "river", "stone", "table", "uncle", "vivid", "water", "xenon",
	} {
		words[w] = ""
	}
	return words
}

// The answer is drawn from a sorted key list, so it must not wander between
// calls within the same UTC day.
func TestDailyAnswerIsStable(t *testing.T) {
	words := sampleWords()
	first := DailyAnswer("English", 5, words)
	if first == "" {
		t.Fatal("DailyAnswer returned no word")
	}
	if _, ok := words[first]; !ok {
		t.Fatalf("DailyAnswer returned %q, which is not in the word list", first)
	}
	for range 50 {
		if got := DailyAnswer("English", 5, words); got != first {
			t.Fatalf("DailyAnswer is unstable: %q then %q", first, got)
		}
	}
}

// The length is part of the hash: two lengths of the same language are two
// different word lists and shouldn't be indexed identically. It used to be
// accepted and ignored, contradicting the function's own doc comment.
func TestDailyAnswerVariesWithLength(t *testing.T) {
	words := sampleWords()

	seen := map[string]bool{}
	for length := 3; length <= 12; length++ {
		seen[DailyAnswer("English", length, words)] = true
	}
	if len(seen) < 2 {
		t.Errorf("every length picked the same word (%v); length is not in the hash", seen)
	}

	// Still stable for a fixed length.
	if DailyAnswer("English", 5, words) != DailyAnswer("English", 5, words) {
		t.Error("DailyAnswer(5) differs between calls")
	}
}

func TestDailyAnswerVariesWithLanguage(t *testing.T) {
	words := sampleWords()
	seen := map[string]bool{}
	for _, lng := range []string{"English", "Spanish", "German", "Greek", "Korean"} {
		seen[DailyAnswer(lng, 5, words)] = true
	}
	if len(seen) < 2 {
		t.Errorf("every language picked the same word (%v)", seen)
	}
}

// An empty word list has no answer to give; it must not divide by zero.
func TestDailyAnswerEmptyWordList(t *testing.T) {
	if got := DailyAnswer("English", 5, nil); got != "" {
		t.Errorf("DailyAnswer(nil) = %q, want \"\"", got)
	}
	if got := DailyAnswer("English", 5, map[string]string{}); got != "" {
		t.Errorf("DailyAnswer(empty) = %q, want \"\"", got)
	}
}

// The measured length round-trips through the cache directory, so a restart
// doesn't re-download a language just to re-measure it.
func TestRecordAndForgetMedianLength(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	// Reset the process-wide memo so this test reads its own temp directory.
	medianLengths.Lock()
	medianLengths.byLang = map[string]int{}
	medianLengths.Unlock()

	if _, ok := RecordedLength("Klingon"); ok {
		t.Fatal("a never-measured language reported a recorded length")
	}

	recordMedianLength("Klingon", 7)
	length, ok := RecordedLength("Klingon")
	if !ok || length != 7 {
		t.Fatalf("RecordedLength = (%d, %v), want (7, true)", length, ok)
	}

	// DefaultLength prefers the measurement over the script estimate.
	if got := DefaultLength("Klingon"); got != 7 {
		t.Errorf("DefaultLength = %d, want the recorded 7", got)
	}

	forgetMedianLength("Klingon")
	if _, ok := RecordedLength("Klingon"); ok {
		t.Error("length survived forgetMedianLength")
	}
	// Falls back to the script-based estimate once forgotten.
	if got := DefaultLength("English"); got != 6 {
		t.Errorf("DefaultLength(English) = %d, want the 6-letter estimate", got)
	}
}

// The median is measured only during an auto-length download. A load at an
// explicitly requested length has measured nothing, so recording it would
// overwrite the language's median — and DefaultLength, which decides the
// length every later game is created at, would follow that one caller.
func TestRecordMeasuredLengthIgnoresExplicitLengths(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	medianLengths.Lock()
	medianLengths.byLang = map[string]int{}
	medianLengths.Unlock()

	recordMeasuredLength("Klingon", AutoLength, 5)
	if length, ok := RecordedLength("Klingon"); !ok || length != 5 {
		t.Fatalf("after an auto-length load: RecordedLength = (%d, %v), want (5, true)", length, ok)
	}

	recordMeasuredLength("Klingon", 9, 9)
	if length, _ := RecordedLength("Klingon"); length != 5 {
		t.Errorf("a load at an explicitly requested length overwrote the median: %d, want the measured 5", length)
	}

	recordMeasuredLength("Vulcan", 7, 7)
	if length, ok := RecordedLength("Vulcan"); ok {
		t.Errorf("an explicit-length load recorded a median for a never-measured language: %d", length)
	}
}

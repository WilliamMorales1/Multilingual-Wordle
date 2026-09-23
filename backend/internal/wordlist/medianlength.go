package wordlist

import (
	json "encoding/json/v2"
	"log"
	"os"
	"path/filepath"
	"sync"

	"wordgo/internal/keyboard"
)

// medianLengthFile remembers, per language, the length its word list was
// built at — the median word length measured while streaming the dump.
// Kept on disk so a restart doesn't have to re-download a language just to
// learn its median again.
//
// The filename is the historical one: older installs already have this file
// in their cache directory, and renaming it would silently discard every
// measurement they have made.
const medianLengthFile = "avg_lengths.json"

var medianLengths = struct {
	sync.RWMutex
	byLang map[string]int
}{}

func medianLengthPath() string { return filepath.Join(cacheDir(), medianLengthFile) }

// loadMedianLengths reads the measurements off disk, once per process.
var loadMedianLengths = sync.OnceFunc(func() {
	medianLengths.Lock()
	defer medianLengths.Unlock()
	medianLengths.byLang = make(map[string]int)
	data, err := os.ReadFile(medianLengthPath())
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &medianLengths.byLang); err != nil {
		log.Printf("Warning: failed to parse %s: %v", medianLengthPath(), err)
		medianLengths.byLang = make(map[string]int)
	}
})

// RecordedLength returns the length a language's word list was last built at.
func RecordedLength(lng string) (int, bool) {
	loadMedianLengths()
	medianLengths.RLock()
	defer medianLengths.RUnlock()
	length, ok := medianLengths.byLang[lng]
	return length, ok
}

func recordMedianLength(lng string, length int) {
	loadMedianLengths()
	medianLengths.Lock()
	if medianLengths.byLang[lng] == length {
		medianLengths.Unlock()
		return
	}
	medianLengths.byLang[lng] = length
	data, err := json.Marshal(medianLengths.byLang)
	medianLengths.Unlock()
	if err != nil {
		return
	}
	if err := os.WriteFile(medianLengthPath(), data, 0644); err != nil {
		log.Printf("Warning: failed to write %s: %v", medianLengthPath(), err)
	}
}

// forgetMedianLength drops a language's measured length so the next load
// re-measures it.
func forgetMedianLength(lng string) {
	loadMedianLengths()
	medianLengths.Lock()
	if _, ok := medianLengths.byLang[lng]; !ok {
		medianLengths.Unlock()
		return
	}
	delete(medianLengths.byLang, lng)
	data, err := json.Marshal(medianLengths.byLang)
	medianLengths.Unlock()
	if err != nil {
		return
	}
	if err := os.WriteFile(medianLengthPath(), data, 0644); err != nil {
		log.Printf("Warning: failed to write %s: %v", medianLengthPath(), err)
	}
}

// DefaultLength is the word length a new game in this language gets: the
// language's own median word length once it's been measured, and the
// script-based estimate (keyboard.DefaultLengthForLang) until then.
func DefaultLength(lng string) int {
	if length, ok := RecordedLength(lng); ok {
		return length
	}
	return keyboard.DefaultLengthForLang(lng)
}

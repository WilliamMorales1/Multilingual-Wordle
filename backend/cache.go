package main

import (
	"os"
	"sort"
	"sync"
)

type wlKey struct {
	lang string
	len  int
}

type wlEntry struct {
	words      map[string]string // word→def
	hanzi      map[string]string // romanized word→hanzi (Chinese dialects only)
	etymology  map[string]string // word→etymology text
	normalized map[string]string // normalizedWord→canonical
	overflow   map[string]bool   // base→bool
}

type wordListStore struct {
	mu      sync.RWMutex
	entries map[wlKey]*wlEntry
	loadMu  sync.Mutex
}

var wlCache = &wordListStore{
	entries: make(map[wlKey]*wlEntry),
}

// downloadProgress tracks in-flight word downloads: "lang:len" → count.
var downloadProgress sync.Map

func getCachedWordList(lang string, length int) (map[string]string, error) {
	key := wlKey{lang, length}

	wlCache.mu.RLock()
	if e, ok := wlCache.entries[key]; ok {
		wlCache.mu.RUnlock()
		return e.words, nil
	}
	wlCache.mu.RUnlock()

	// Double-checked locking: only one goroutine loads per key.
	wlCache.loadMu.Lock()
	defer wlCache.loadMu.Unlock()

	wlCache.mu.RLock()
	if e, ok := wlCache.entries[key]; ok {
		wlCache.mu.RUnlock()
		return e.words, nil
	}
	wlCache.mu.RUnlock()

	words, hanzi, etymology, err := loadWordList(lang, length)
	if err != nil {
		return nil, err
	}

	normalized := buildNormalizedSet(words)
	alphabet := buildAlphabet(words)
	_, overflowBases, _ := buildKeyboardData(alphabet, lang, words)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}

	wlCache.mu.Lock()
	wlCache.entries[key] = &wlEntry{
		words:      words,
		hanzi:      hanzi,
		etymology:  etymology,
		normalized: normalized,
		overflow:   overflowSet,
	}
	wlCache.mu.Unlock()

	return words, nil
}

// getCachedHanzi returns the romanized-word→hanzi map for a Chinese-dialect
// word list, or nil if the language isn't a Chinese dialect or isn't cached yet.
func getCachedHanzi(lang string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[wlKey{lang, length}]; ok {
		return e.hanzi
	}
	return nil
}

// getCachedEtymology returns the word→etymology map for a word list, or nil if not cached yet.
func getCachedEtymology(lang string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[wlKey{lang, length}]; ok {
		return e.etymology
	}
	return nil
}

// getWordListIfCached returns the cached word list without triggering a load.
func getWordListIfCached(lang string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[wlKey{lang, length}]; ok {
		return e.words
	}
	return nil
}

func getCachedNormalized(lang string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[wlKey{lang, length}]; ok {
		return e.normalized
	}
	return nil
}

func getCachedOverflow(lang string, length int) map[string]bool {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[wlKey{lang, length}]; ok {
		return e.overflow
	}
	return nil
}

var (
	langCacheMu sync.RWMutex
	langCache   []string
)

// dir. If keep is non-zero, that entry's in-memory data is preserved so an
// in-progress game using it keeps working; its on-disk cache file is still
// removed since it isn't needed again until the process restarts.
func clearWordListCache(keep wlKey) error {
	wlCache.mu.Lock()
	newEntries := make(map[wlKey]*wlEntry)
	if keep != (wlKey{}) {
		if e, ok := wlCache.entries[keep]; ok {
			newEntries[keep] = e
		}
	}
	wlCache.entries = newEntries
	wlCache.mu.Unlock()

	langCacheMu.Lock()
	langCache = nil
	langCacheMu.Unlock()

	return os.RemoveAll(dataPath("cache"))
}

func getCachedLanguages() []string {
	langCacheMu.RLock()
	if langCache != nil {
		defer langCacheMu.RUnlock()
		return langCache
	}
	langCacheMu.RUnlock()

	langCacheMu.Lock()
	defer langCacheMu.Unlock()

	if langCache != nil {
		return langCache
	}

	langMap := getLanguages()
	names := make([]string, 0, len(langMap)+len(chineseDialects))

	sort.Strings(names)
	langCache = names
	return names
}

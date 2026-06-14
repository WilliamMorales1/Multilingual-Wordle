package main

import (
	"fmt"
	"sort"
	"sync"
)

type wordListStore struct {
	mu         sync.RWMutex
	words      map[string]map[string]string // "lang:len" → word→def
	normalized map[string]map[string]string // "lang:len" → normalizedWord→canonical
	overflow   map[string]map[string]bool   // "lang:len" → base→bool
	loadMu     sync.Mutex
}

var wlCache = &wordListStore{
	words:      make(map[string]map[string]string),
	normalized: make(map[string]map[string]string),
	overflow:   make(map[string]map[string]bool),
}

// downloadProgress tracks in-flight word downloads: "lang:len" → count.
var downloadProgress sync.Map

func getCachedWordList(lang string, length int) (map[string]string, error) {
	key := fmt.Sprintf("%s:%d", lang, length)

	wlCache.mu.RLock()
	if m, ok := wlCache.words[key]; ok {
		wlCache.mu.RUnlock()
		return m, nil
	}
	wlCache.mu.RUnlock()

	// Double-checked locking: only one goroutine loads per key.
	wlCache.loadMu.Lock()
	defer wlCache.loadMu.Unlock()

	wlCache.mu.RLock()
	if m, ok := wlCache.words[key]; ok {
		wlCache.mu.RUnlock()
		return m, nil
	}
	wlCache.mu.RUnlock()

	words, err := loadWordList(lang, length)
	if err != nil {
		return nil, err
	}

	normalized := buildNormalizedSet(words)
	alphabet := buildAlphabet(words)
	_, overflowBases := buildKeyboardData(alphabet, lang)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}

	wlCache.mu.Lock()
	wlCache.words[key] = words
	wlCache.normalized[key] = normalized
	wlCache.overflow[key] = overflowSet
	wlCache.mu.Unlock()

	return words, nil
}

// getWordListIfCached returns the cached word list without triggering a load.
func getWordListIfCached(lang string, length int) map[string]string {
	key := fmt.Sprintf("%s:%d", lang, length)
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	return wlCache.words[key]
}

func getCachedNormalized(lang string, length int) map[string]string {
	key := fmt.Sprintf("%s:%d", lang, length)
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	return wlCache.normalized[key]
}

func getCachedOverflow(lang string, length int) map[string]bool {
	key := fmt.Sprintf("%s:%d", lang, length)
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	return wlCache.overflow[key]
}

var (
	langCacheMu sync.RWMutex
	langCache   []string
)

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
	names := make([]string, 0, len(langMap))
	for name := range langMap {
		names = append(names, name)
	}
	sort.Strings(names)
	langCache = names
	return names
}

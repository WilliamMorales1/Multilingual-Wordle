package main

import (
	"fmt"
	"os"
	"sort"
	"sync"
)

type wordListStore struct {
	mu         sync.RWMutex
	words      map[string]map[string]string // "lang:len" → word→def
	hanzi      map[string]map[string]string // "lang:len" → romanized word→hanzi (Chinese dialects only)
	etymology  map[string]map[string]string // "lang:len" → word→etymology text
	normalized map[string]map[string]string // "lang:len" → normalizedWord→canonical
	overflow   map[string]map[string]bool   // "lang:len" → base→bool
	loadMu     sync.Mutex
}

var wlCache = &wordListStore{
	words:      make(map[string]map[string]string),
	hanzi:      make(map[string]map[string]string),
	etymology:  make(map[string]map[string]string),
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
	wlCache.words[key] = words
	wlCache.hanzi[key] = hanzi
	wlCache.etymology[key] = etymology
	wlCache.normalized[key] = normalized
	wlCache.overflow[key] = overflowSet
	wlCache.mu.Unlock()

	return words, nil
}

// getCachedHanzi returns the romanized-word→hanzi map for a Chinese-dialect
// word list, or nil if the language isn't a Chinese dialect or isn't cached yet.
func getCachedHanzi(lang string, length int) map[string]string {
	key := fmt.Sprintf("%s:%d", lang, length)
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	return wlCache.hanzi[key]
}

// getCachedEtymology returns the word→etymology map for a word list, or nil if not cached yet.
func getCachedEtymology(lang string, length int) map[string]string {
	key := fmt.Sprintf("%s:%d", lang, length)
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	return wlCache.etymology[key]
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

// clearWordListCache wipes the in-memory word list cache and on-disk cache
// dir. If keep is non-empty (a "lang:len" key), that entry's in-memory data
// is preserved so an in-progress game using it keeps working; its on-disk
// cache file is still removed since it isn't needed again until the process
// restarts.
func clearWordListCache(keep string) error {
	wlCache.mu.Lock()
	newWords := make(map[string]map[string]string)
	newHanzi := make(map[string]map[string]string)
	newEtymology := make(map[string]map[string]string)
	newNormalized := make(map[string]map[string]string)
	newOverflow := make(map[string]map[string]bool)
	if keep != "" {
		if v, ok := wlCache.words[keep]; ok {
			newWords[keep] = v
		}
		if v, ok := wlCache.hanzi[keep]; ok {
			newHanzi[keep] = v
		}
		if v, ok := wlCache.etymology[keep]; ok {
			newEtymology[keep] = v
		}
		if v, ok := wlCache.normalized[keep]; ok {
			newNormalized[keep] = v
		}
		if v, ok := wlCache.overflow[keep]; ok {
			newOverflow[keep] = v
		}
	}
	wlCache.words = newWords
	wlCache.hanzi = newHanzi
	wlCache.etymology = newEtymology
	wlCache.normalized = newNormalized
	wlCache.overflow = newOverflow
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
	for name := range langMap {
		if _, ok := avgWordLengths[name]; ok {
			names = append(names, name)
		}
	}
	for _, d := range chineseDialects {
		name := fmt.Sprintf("Chinese (%s)", d)
		if _, ok := avgWordLengths[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	langCache = names
	return names
}

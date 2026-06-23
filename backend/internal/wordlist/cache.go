package wordlist

import (
	"hash/fnv"
	"os"
	"sort"
	"sync"
	"time"

	"wordgo/internal/keyboard"
	"wordgo/internal/lang"
)

// Key identifies a cached word list by language and word length.
type Key struct {
	Lang string
	Len  int
}

type entry struct {
	words      map[string]string // word→def
	hanzi      map[string]string // romanized word→hanzi (Chinese dialects only)
	etymology  map[string]string // word→etymology text
	normalized map[string]string // normalizedWord→canonical
	overflow   map[string]bool   // base→bool
}

type wordListStore struct {
	mu      sync.RWMutex
	entries map[Key]*entry
	loadMu  sync.Mutex
}

var wlCache = &wordListStore{
	entries: make(map[Key]*entry),
}

// DownloadProgress tracks in-flight word downloads: "lang:len" → count.
var DownloadProgress sync.Map

// DailyAnswer picks one word per UTC calendar day for a given language/length,
// deterministically from a hash of the date so every player sees the same
// answer that day, like regular Wordle.
func DailyAnswer(lng string, length int, words map[string]string) string {
	keys := make([]string, 0, len(words))
	for w := range words {
		keys = append(keys, w)
	}
	sort.Strings(keys)

	h := fnv.New64a()
	h.Write([]byte(time.Now().UTC().Format("2006-01-02")))
	h.Write([]byte(lng))
	idx := int(h.Sum64() % uint64(len(keys)))
	return keys[idx]
}

func GetCachedWordList(lng string, length int) (map[string]string, error) {
	key := Key{lng, length}

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

	words, hanzi, etymology, err := loadWordList(lng, length)
	if err != nil {
		return nil, err
	}

	toneLang := lang.ToneSplitKind(lng)
	normalized := lang.BuildNormalizedSet(words, toneLang)
	alphabet := lang.BuildAlphabet(words, toneLang)
	_, overflowBases, _ := keyboard.BuildKeyboardData(alphabet, lng, words)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}

	wlCache.mu.Lock()
	wlCache.entries[key] = &entry{
		words:      words,
		hanzi:      hanzi,
		etymology:  etymology,
		normalized: normalized,
		overflow:   overflowSet,
	}
	wlCache.mu.Unlock()

	return words, nil
}

// GetCachedHanzi returns the romanized-word→hanzi map for a Chinese-dialect
// word list, or nil if the language isn't a Chinese dialect or isn't cached yet.
func GetCachedHanzi(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.hanzi
	}
	return nil
}

// GetCachedEtymology returns the word→etymology map for a word list, or nil if not cached yet.
func GetCachedEtymology(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.etymology
	}
	return nil
}

// GetWordListIfCached returns the cached word list without triggering a load.
func GetWordListIfCached(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.words
	}
	return nil
}

func GetCachedNormalized(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.normalized
	}
	return nil
}

func GetCachedOverflow(lng string, length int) map[string]bool {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.overflow
	}
	return nil
}

var (
	langCacheMu sync.RWMutex
	langCache   []string
)

// ClearWordListCache wipes the in-memory and on-disk word list cache. If keep
// is non-zero, that entry's in-memory data is preserved so an in-progress
// game using it keeps working; its on-disk cache file is still removed since
// it isn't needed again until the process restarts.
func ClearWordListCache(keep Key) error {
	wlCache.mu.Lock()
	newEntries := make(map[Key]*entry)
	if keep != (Key{}) {
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

func GetCachedLanguages() []string {
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
		names = append(names, name)
	}
	for _, d := range chineseDialects {
		names = append(names, "Chinese ("+d+")")
	}

	sort.Strings(names)
	langCache = names
	return names
}

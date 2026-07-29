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

// Identifies a cached word list
type Key struct {
	Lang string
	Len  int
}

type entry struct {
	words      map[string]string // word→def
	hanzi      map[string]string // romanized word→hanzi (for Chinese dialects only)
	etymology  map[string]string // word→etymology text
	normalized map[string]string // normalizedWord→canonical
	overflow   map[string]bool   // base→bool
}

type wordListStore struct {
	mu      sync.RWMutex
	entries map[Key]*entry
	order   []Key // insertion order, oldest first — FIFO eviction once over cap
	loadMu  sync.Mutex
}

var wlCache = &wordListStore{
	entries: make(map[Key]*entry),
}

// maxCachedWordLists bounds in-memory growth: each entry holds a full word
// list (plus definitions/hanzi/etymology) for one lang/length pair, kept
// forever otherwise. A simple FIFO cap is good enough here — this isn't a
// hot enough path to justify true LRU bookkeeping.
const maxCachedWordLists = 50

// Tracks in-flight word downloads: "lang:len" → count.
var DownloadProgress sync.Map

// Picks one word per UTC calendar day by hashing language/length,
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
	wlCache.order = append(wlCache.order, key)
	if len(wlCache.order) > maxCachedWordLists {
		oldest := wlCache.order[0]
		wlCache.order = wlCache.order[1:]
		delete(wlCache.entries, oldest)
	}
	wlCache.mu.Unlock()

	return words, nil
}

// nil if the language isn't a Chinese dialect or isn't cached yet.
func GetCachedHanzi(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.hanzi
	}
	return nil
}

func GetCachedEtymology(lng string, length int) map[string]string {
	wlCache.mu.RLock()
	defer wlCache.mu.RUnlock()
	if e, ok := wlCache.entries[Key{lng, length}]; ok {
		return e.etymology
	}
	return nil
}

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

// ClearWordListCache evicts a single lang/length's cached word list, both in
// memory and its on-disk JSON files, so a stale or corrupted entry can be
// force-refreshed without disturbing every other player's cache — this used
// to wipe the entire shared cache, which meant one client force-clearing
// their own stale list nuked everyone else's mid-game.
func ClearWordListCache(key Key) error {
	if key == (Key{}) {
		return nil
	}

	wlCache.mu.Lock()
	delete(wlCache.entries, key)
	for i, k := range wlCache.order {
		if k == key {
			wlCache.order = append(wlCache.order[:i], wlCache.order[i+1:]...)
			break
		}
	}
	wlCache.mu.Unlock()

	for _, suffix := range []string{"", "_hanzi", "_etymology"} {
		if err := os.Remove(cacheFilePath(key.Lang, key.Len, suffix)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
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

package wordlist

import (
	"encoding/binary"
	"hash/fnv"
	"log"
	"maps"
	"os"
	"slices"
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

// DailyAnswer picks one word per UTC calendar day by hashing date, language
// and length together. The length is part of the hash on purpose: the same
// language played at two lengths should not land on the same index of two
// different word lists.
//
// Returns "" for an empty word list rather than dividing by zero.
func DailyAnswer(lng string, length int, words map[string]string) string {
	if len(words) == 0 {
		return ""
	}
	keys := slices.Sorted(maps.Keys(words))

	h := fnv.New64a()
	h.Write([]byte(time.Now().UTC().Format("2006-01-02")))
	h.Write([]byte(lng))
	h.Write(binary.BigEndian.AppendUint64(nil, uint64(length)))
	idx := int(h.Sum64() % uint64(len(keys)))
	return keys[idx]
}

func GetCachedWordList(lng string, length int) (map[string]string, error) {
	words, _, err := getOrLoad(lng, length)
	return words, err
}

// GetCachedWordListAuto loads a language's word list at its own median word
// length, returning the length used. The median is measured the first time
// the language is downloaded and remembered from then on.
func GetCachedWordListAuto(lng string) (map[string]string, int, error) {
	if length, ok := RecordedLength(lng); ok {
		words, err := GetCachedWordList(lng, length)
		return words, length, err
	}
	return getOrLoad(lng, AutoLength)
}

// getOrLoad returns a lang/length's word list, loading and caching it if
// needed. With AutoLength the length is picked from the language's median
// word length; either way the length actually used is returned.
func getOrLoad(lng string, length int) (map[string]string, int, error) {
	if length != AutoLength {
		wlCache.mu.RLock()
		e, ok := wlCache.entries[Key{lng, length}]
		wlCache.mu.RUnlock()
		if ok {
			return e.words, length, nil
		}
	}

	// Double-checked locking: only one goroutine loads per key.
	wlCache.loadMu.Lock()
	defer wlCache.loadMu.Unlock()

	if length != AutoLength {
		wlCache.mu.RLock()
		e, ok := wlCache.entries[Key{lng, length}]
		wlCache.mu.RUnlock()
		if ok {
			return e.words, length, nil
		}
	}

	words, hanzi, etymology, length, err := loadWordList(lng, length)
	if err != nil {
		return nil, length, err
	}
	key := Key{lng, length}

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
	if !slices.Contains(wlCache.order, key) {
		wlCache.order = append(wlCache.order, key)
	}
	if len(wlCache.order) > maxCachedWordLists {
		oldest := wlCache.order[0]
		wlCache.order = wlCache.order[1:]
		delete(wlCache.entries, oldest)
	}
	wlCache.mu.Unlock()

	return words, length, nil
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

// languageIndex memoizes the kaikki.org language index (plus the Chinese
// pseudo-languages). Only a *successful* fetch is memoized: getLanguages
// returns nil on a network or parse error, and caching that would leave the
// process permanently believing no language exists — every /api/game call
// rejected as an unknown language until a restart. A failed fetch is simply
// retried on the next call.
var languageIndex struct {
	mu    sync.Mutex
	names []string
}

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
	if i := slices.Index(wlCache.order, key); i >= 0 {
		wlCache.order = slices.Delete(wlCache.order, i, i+1)
	}
	wlCache.mu.Unlock()

	for _, suffix := range []string{"", "_hanzi", "_etymology"} {
		if err := os.Remove(cacheFilePath(key.Lang, key.Len, suffix)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// Forget the measured length too, so the refresh re-measures the
	// language rather than rebuilding at a length that may be why the list
	// was cleared in the first place.
	forgetMedianLength(key.Lang)
	return nil
}

func GetCachedLanguages() []string {
	languageIndex.mu.Lock()
	defer languageIndex.mu.Unlock()
	if languageIndex.names != nil {
		return languageIndex.names
	}

	langMap := getLanguages()
	if len(langMap) == 0 {
		log.Printf("Warning: kaikki.org language index unavailable - will retry on the next request")
		return nil
	}

	names := make([]string, 0, len(langMap)+len(chineseDialects))
	names = append(names, slices.Collect(maps.Keys(langMap))...)
	for _, d := range chineseDialects {
		names = append(names, "Chinese ("+d+")")
	}
	slices.Sort(names)
	languageIndex.names = names
	return names
}

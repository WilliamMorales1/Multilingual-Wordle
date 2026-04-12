package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/unicode/norm"
)

// Defaults
const (
	DefaultLang    = "English"
	DefaultLength  = 5
	DefaultGuesses = 6
)

const (
	rawDumpURL           = "https://kaikki.org/dictionary/raw-wiktextract-data.jsonl.gz"
	logographicThreshold = 200
)

// KaikkiEntry represents a JSON entry from the kaikki.org dump
type KaikkiEntry struct {
	Word   string           `json:"word"`
	Lang   string           `json:"lang"`
	Senses []map[string]any `json:"senses"`
}

// wordChars splits a word into grapheme clusters (logical characters)
func wordChars(word string) []string {
	var chars []string
	normalized := norm.NFC.String(word)

	runes := []rune(normalized)
	i := 0
	for i < len(runes) {
		var char strings.Builder
		char.WriteString(string(runes[i]))
		i++

		// Collect combining marks
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			char.WriteString(string(runes[i]))
			i++
		}
		chars = append(chars, char.String())
	}

	return chars
}

// wordLen returns the length in grapheme clusters
func wordLen(word string) int {
	return len(wordChars(word))
}

// isWordChar checks if a character is a letter or combining mark
func isWordChar(r rune) bool {
	return unicode.In(r,
		unicode.Ll, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Lm,
		unicode.Mn, unicode.Mc, unicode.Me)
}

// isValid checks if a word is valid for the given length
func isValid(word string, length int) bool {
	if wordLen(word) != length {
		return false
	}
	for _, r := range word {
		if !isWordChar(r) {
			return false
		}
	}
	return true
}

// kaikkiURL constructs the URL for a language's word dump.
// Directory uses %20 for spaces; filename has spaces stripped entirely.
// e.g. "Old English" → /dictionary/Old%20English/kaikki.org-dictionary-OldEnglish.jsonl.gz
func kaikkiURL(lang string) string {
	slug := strings.ReplaceAll(lang, " ", "")
	u := &url.URL{
		Scheme: "https",
		Host:   "kaikki.org",
		Path:   fmt.Sprintf("/dictionary/%s/kaikki.org-dictionary-%s.jsonl.gz", lang, slug),
	}
	return u.String()
}

// cachePath returns the path to the per-language, per-length cache file.
func cachePath(lang string, length int) string {
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl.json", safe, length))
}

// firstGloss extracts the first definition from a kaikki entry
func firstGloss(entry KaikkiEntry) string {
	for _, sense := range entry.Senses {
		if glosses, ok := sense["glosses"].([]interface{}); ok && len(glosses) > 0 {
			if gloss, ok := glosses[0].(string); ok {
				return gloss
			}
		}
	}
	return ""
}

// streamURL downloads words of the given length for a language from a gzipped JSONL file.
// Parsing is parallelised across workers while the scanner streams the download.
// onProgress is called with the running word count every 500 words (may be nil).
func streamURL(rawURL, lang string, length int, onProgress func(int)) (map[string]string, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	type result struct{ word, def string }

	numWorkers := runtime.NumCPU()
	lines   := make(chan []byte, numWorkers*8)
	results := make(chan result,  numWorkers*8)

	// Workers: parse JSON and filter in parallel.
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for line := range lines {
				var entry KaikkiEntry
				if json.Unmarshal(line, &entry) != nil {
					continue
				}
				if entry.Lang != lang || len(entry.Senses) == 0 {
					continue
				}
				if !isValid(entry.Word, length) {
					continue
				}
				results <- result{strings.ToLower(entry.Word), firstGloss(entry)}
			}
		})
	}

	// Close results once all workers finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Scanner feeds raw line bytes to workers.
	var scanErr error
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(gz)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			b := make([]byte, len(scanner.Bytes()))
			copy(b, scanner.Bytes())
			lines <- b
		}
		scanErr = scanner.Err()
	}()

	// Collect results on the main goroutine.
	words := make(map[string]string)
	for r := range results {
		if _, exists := words[r.word]; !exists {
			words[r.word] = r.def
		}
		if len(words)%500 == 0 {
			log.Printf("  %d words collected...", len(words))
			if onProgress != nil {
				onProgress(len(words))
			}
		}
	}

	if scanErr != nil {
		if len(words) >= 20 {
			log.Printf("Warning: scanner error after %d words (%v) — using partial results", len(words), scanErr)
			return words, nil
		}
		return nil, scanErr
	}

	return words, nil
}

// loadWordList loads words of the given length from disk cache, or downloads
// and caches them from kaikki.org if not present.
func loadWordList(lang string, length int) (map[string]string, error) {
	cf := cachePath(lang, length)

	if data, err := os.ReadFile(cf); err == nil {
		var cached map[string]string
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) >= 20 {
			log.Printf("Loaded %d %s %d-letter words from cache (%s)", len(cached), lang, length, filepath.Base(cf))
			return cached, nil
		}
	}

	u := kaikkiURL(lang)
	log.Printf("Downloading %s wiktextract dump from %s", lang, u)

	key := fmt.Sprintf("%s:%d", lang, length)
	words, err := streamURL(u, lang, length, func(n int) {
		downloadProgress.Store(key, n)
	})
	downloadProgress.Delete(key)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, fmt.Errorf("language %q not found on kaikki.org — check /api/languages for valid names", lang)
		}
		return nil, err
	}

	if len(words) == 0 {
		return nil, fmt.Errorf("no %d-character %s words found", length, lang)
	}

	log.Printf("%d %s %d-letter words collected", len(words), lang, length)

	if data, err := json.Marshal(words); err == nil {
		if err := os.WriteFile(cf, data, 0644); err == nil {
			log.Printf("Cached at %s", cf)
		}
	}

	return words, nil
}

// normalizeWord returns the accent-stripped, lowercased form of a whole word,
// joining the normalizeChar result for each grapheme cluster.
func normalizeWord(word string) string {
	chars := wordChars(word)
	var b strings.Builder
	for _, ch := range chars {
		b.WriteString(normalizeChar(ch))
	}
	return b.String()
}

// buildNormalizedSet returns a set of normalized (accent-stripped) word forms
// mapped back to one canonical original word. Used for variant-aware lookup.
func buildNormalizedSet(words map[string]string) map[string]string {
	set := make(map[string]string, len(words))
	for w := range words {
		set[normalizeWord(w)] = w
	}
	return set
}

// buildAlphabet collects all unique grapheme clusters from the word list.
// Returns nil for logographic scripts with more than logographicThreshold chars.
func buildAlphabet(wordList map[string]string) []string {
	charSet := make(map[string]bool)
	for word := range wordList {
		for _, ch := range wordChars(word) {
			charSet[ch] = true
		}
	}

	if len(charSet) > logographicThreshold {
		return nil
	}

	chars := make([]string, 0, len(charSet))
	for ch := range charSet {
		chars = append(chars, ch)
	}
	sort.Strings(chars)
	return chars
}

// normalizeChar strips diacritical marks from a grapheme cluster and lowercases it,
// so that accented variants (é, ñ, ü…) compare equal to their base letters (e, n, u…).
func normalizeChar(ch string) string {
	// NFD decomposes precomposed chars: é → e + U+0301 (combining acute)
	nfd := norm.NFD.String(ch)
	var buf strings.Builder
	for _, r := range nfd {
		if !unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
			buf.WriteRune(r)
		}
	}
	return strings.ToLower(buf.String())
}

// evaluate checks a guess against the answer and returns per-character states.
// States are "correct", "present", or "absent".
// Comparison is accent-insensitive: é matches e, ñ matches n, etc.
func evaluate(guessChars, answerChars []string) []string {
	length := len(answerChars)
	states := make([]string, length)

	// Normalize both sides for comparison
	normGuess := make([]string, length)
	normAnswer := make([]string, length)
	for i := range answerChars {
		normGuess[i] = normalizeChar(guessChars[i])
		normAnswer[i] = normalizeChar(answerChars[i])
	}

	pool := make([]string, length)
	copy(pool, normAnswer)

	// First pass: mark correct positions
	for i := range normGuess {
		if normGuess[i] == normAnswer[i] {
			states[i] = "correct"
			pool[i] = ""
		} else {
			states[i] = "absent"
		}
	}

	// Second pass: mark present characters
	for i, g := range normGuess {
		if states[i] == "correct" {
			continue
		}
		for j, p := range pool {
			if p != "" && g == p {
				states[i] = "present"
				pool[j] = ""
				break
			}
		}
	}

	return states
}

// getLanguages fetches available language names from kaikki.org.
// Returns a map of language name → relative href.
func getLanguages() map[string]string {
	resp, err := http.Get("https://kaikki.org/dictionary/index.html")
	if err != nil {
		log.Printf("Error fetching language list: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Error fetching language list: HTTP %d", resp.StatusCode)
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Error parsing language list: %v", err)
		return nil
	}

	languages := make(map[string]string)
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.HasSuffix(href, "/index.html") {
			trimmed := strings.TrimSuffix(href, "/index.html")
			decoded, err := url.QueryUnescape(trimmed)
			if err != nil {
				return
			}
			if !strings.Contains(decoded, ".") {
				languages[decoded] = href
			}
		}
	})

	return languages
}

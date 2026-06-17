package main

// Word list loading from kaikki.org and local cache management.

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
	"strings"
	"sync"

	"golang.org/x/net/html"
)

const (
	DefaultLang    = "English"
	DefaultLength  = 5
	DefaultGuesses = 6
)

type KaikkiEntry struct {
	Word      string           `json:"word"`
	Lang      string           `json:"lang"`
	Pos       string           `json:"pos"`
	Senses    []map[string]any `json:"senses"`
	Sounds    []KaikkiSound    `json:"sounds"`
	Etymology string           `json:"etymology_text"`
}

// excludedSenseTags marks senses that identify a word as a proper noun,
// given name, or surname rather than an ordinary dictionary word.
var excludedSenseTags = map[string]bool{
	"proper nouns":    true,
	"given names":     true,
	"surnames":        true,
	"non-lemma forms": true,
}

// isExcludedEntry reports whether an entry should be skipped because it (or
// all of its senses) is tagged as a proper noun, given name, or surname.
func isExcludedEntry(entry KaikkiEntry) bool {
	if strings.EqualFold(entry.Pos, "name") {
		return true
	}
	for _, sense := range entry.Senses {
		tags, ok := sense["tags"].([]any)
		if !ok {
			continue
		}
		for _, t := range tags {
			tag, ok := t.(string)
			if ok && excludedSenseTags[strings.ToLower(tag)] {
				return true
			}
		}
	}
	return false
}

type KaikkiSound struct {
	ZhPron string   `json:"zh_pron"`
	Tags   []string `json:"tags"`
}

// chineseDialects lists topolects exposed as "Chinese (X)" pseudo-languages.
// Each maps to the single kaikki.org "Chinese" dump, picking romanization by
// matching this name against a sound entry's tags (e.g. "Cantonese", "Hokkien").
var chineseDialects = []string{
	"Mandarin", "Cantonese", "Hokkien", "Teochew", "Hakka", "Wu",
	"Min Bei", "Min Dong", "Gan", "Xiang", "Jin",
}

// parseChineseDialect extracts the dialect name from a "Chinese (X)" pseudo-language.
func parseChineseDialect(lang string) (string, bool) {
	if !strings.HasPrefix(lang, "Chinese (") || !strings.HasSuffix(lang, ")") {
		return "", false
	}
	d := strings.TrimSuffix(strings.TrimPrefix(lang, "Chinese ("), ")")
	if d == "" {
		return "", false
	}
	return d, true
}

// romanizeEntry returns the romanization for the given dialect from an entry's
// sounds list (e.g. Pinyin for Mandarin, Jyutping for Cantonese), or "" if absent.
func romanizeEntry(entry KaikkiEntry, dialect string) string {
	for _, s := range entry.Sounds {
		if s.ZhPron == "" {
			continue
		}
		for _, t := range s.Tags {
			if strings.EqualFold(t, dialect) {
				return s.ZhPron
			}
		}
	}
	return ""
}

// filterLetters strips everything but letters and combining marks (digits,
// spaces, hyphens, tone numbers) so a romanization can be used as a word.
func filterLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isWordChar(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// kaikkiURL builds the per-language dump URL on kaikki.org.
// Directory uses %20 for spaces; filename has spaces stripped entirely.
func kaikkiURL(lang string) string {
	slug := strings.ReplaceAll(lang, " ", "")
	u := &url.URL{
		Scheme: "https",
		Host:   "kaikki.org",
		Path:   fmt.Sprintf("/dictionary/%s/kaikki.org-dictionary-%s.jsonl.gz", lang, slug),
	}
	return u.String()
}

func cachePath(lang string, length int) string {
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl.json", safe, length))
}

func firstGloss(entry KaikkiEntry) string {
	for _, sense := range entry.Senses {
		if glosses, ok := sense["glosses"].([]any); ok && len(glosses) > 0 {
			if gloss, ok := glosses[0].(string); ok {
				return gloss
			}
		}
	}
	return ""
}

// streamURL downloads and parses a gzipped JSONL word dump from kaikki.org.
// Parsing is parallelised across CPU workers while the scanner streams the download.
func streamURL(rawURL, lang, dialect string, length int, onProgress func(int)) (map[string]string, map[string]string, map[string]string, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}
	defer gz.Close()

	type result struct{ word, def, hanzi, etymology string }

	numWorkers := runtime.NumCPU()
	lines := make(chan []byte, numWorkers*8)
	results := make(chan result, numWorkers*8)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for line := range lines {
				var entry KaikkiEntry
				if json.Unmarshal(line, &entry) != nil {
					continue
				}
				if entry.Lang != lang || len(entry.Senses) == 0 || isExcludedEntry(entry) {
					continue
				}
				var word, hanzi string
				if dialect != "" {
					rom := romanizeEntry(entry, dialect)
					if rom == "" {
						continue
					}
					word = filterLetters(strings.ToLower(rom))
					hanzi = entry.Word
				} else {
					word = strings.ToLower(entry.Word)
					if isHangulLang(lang) {
						word = expandJamo(decomposeHangul(word))
						if !isPureJamo(word) {
							continue
						}
					} else if isJapaneseLang(lang) {
						word = katakanaToHiragana(word)
						if !isPureHiragana(word) {
							continue
						}
					}
				}
				if !isValid(word, length) {
					continue
				}
				results <- result{word, firstGloss(entry), hanzi, entry.Etymology}
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

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

	words := make(map[string]string)
	etymology := make(map[string]string)
	var hanziMap map[string]string
	if dialect != "" {
		hanziMap = make(map[string]string)
	}
	for r := range results {
		if _, exists := words[r.word]; !exists {
			words[r.word] = r.def
			if r.etymology != "" {
				etymology[r.word] = r.etymology
			}
			if hanziMap != nil {
				hanziMap[r.word] = r.hanzi
			}
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
			return words, hanziMap, etymology, nil
		}
		return nil, nil, nil, scanErr
	}
	return words, hanziMap, etymology, nil
}

// hanziCachePath returns the sidecar cache path holding hanzi for a Chinese-dialect word list.
func hanziCachePath(lang string, length int) string {
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl_hanzi.json", safe, length))
}

// etymologyCachePath returns the sidecar cache path holding etymology text for a word list.
func etymologyCachePath(lang string, length int) string {
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl_etymology.json", safe, length))
}

func loadWordList(lang string, length int) (map[string]string, map[string]string, map[string]string, error) {
	cf := cachePath(lang, length)
	hcf := hanziCachePath(lang, length)
	ecf := etymologyCachePath(lang, length)

	if data, err := os.ReadFile(cf); err == nil {
		var cached map[string]string
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) >= 20 {
			log.Printf("Loaded %d %s %d-letter words from cache (%s)", len(cached), lang, length, filepath.Base(cf))
			var hanzi map[string]string
			if hdata, err := os.ReadFile(hcf); err == nil {
				json.Unmarshal(hdata, &hanzi)
			}
			var etymology map[string]string
			if edata, err := os.ReadFile(ecf); err == nil {
				json.Unmarshal(edata, &etymology)
			}
			return cached, hanzi, etymology, nil
		}
	}

	matchLang := lang
	dialect := ""
	if d, ok := parseChineseDialect(lang); ok {
		matchLang = "Chinese"
		dialect = d
	}

	u := kaikkiURL(matchLang)
	log.Printf("Downloading %s wiktextract dump from %s", matchLang, u)

	key := fmt.Sprintf("%s:%d", lang, length)
	words, hanzi, etymology, err := streamURL(u, matchLang, dialect, length, func(n int) {
		downloadProgress.Store(key, n)
	})
	downloadProgress.Delete(key)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, nil, nil, fmt.Errorf("language %q not found on kaikki.org — check /api/languages for valid names", lang)
		}
		return nil, nil, nil, err
	}
	if len(words) == 0 {
		return nil, nil, nil, fmt.Errorf("no %d-character %s words found", length, lang)
	}

	log.Printf("%d %s %d-letter words collected", len(words), lang, length)

	if data, err := json.Marshal(words); err == nil {
		if err := os.WriteFile(cf, data, 0644); err == nil {
			log.Printf("Cached at %s", cf)
		}
	}
	if hanzi != nil {
		if data, err := json.Marshal(hanzi); err == nil {
			os.WriteFile(hcf, data, 0644)
		}
	}
	if len(etymology) > 0 {
		if data, err := json.Marshal(etymology); err == nil {
			os.WriteFile(ecf, data, 0644)
		}
	}
	return words, hanzi, etymology, nil
}

// getLanguages scrapes available language names from kaikki.org.
func getLanguages() map[string]string {
	resp, err := http.Get("https://kaikki.org/dictionary/index.html")
	if err != nil {
		log.Printf("Error fetching language list: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error fetching language list: HTTP %d", resp.StatusCode)
		return nil
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Printf("Error parsing language list: %v", err)
		return nil
	}

	languages := make(map[string]string)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.HasSuffix(attr.Val, "/index.html") {
					trimmed := strings.TrimSuffix(attr.Val, "/index.html")
					decoded, err := url.QueryUnescape(trimmed)
					if err != nil {
						break
					}
					if !strings.Contains(decoded, ".") {
						languages[decoded] = attr.Val
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return languages
}

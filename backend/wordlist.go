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
	Word   string           `json:"word"`
	Lang   string           `json:"lang"`
	Senses []map[string]any `json:"senses"`
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

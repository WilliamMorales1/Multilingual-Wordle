// Package wordlist fetches, caches, and serves per-language word lists
// sourced from kaikki.org wiktextract dumps.
package wordlist

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
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"wordgo/internal/lang"
)

const (
	DefaultLang   = "English"
	DefaultLength = 6
)

// dataPath resolves name relative to DATA_DIR (or the working directory if unset).
func dataPath(name string) string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, name)
}

type KaikkiEntry struct {
	Word          string               `json:"word"`
	Lang          string               `json:"lang"`
	Pos           string               `json:"pos"`
	Senses        []map[string]any     `json:"senses"`
	Sounds        []KaikkiSound        `json:"sounds"`
	Etymology     string               `json:"etymology_text"`
	HeadTemplates []KaikkiHeadTemplate `json:"head_templates"`
}

// KaikkiHeadTemplate carries the "Han char" infobox template on Translingual
// character entries — Expansion's prose embeds the Cangjie root-glyph code
// (e.g. "...Cangjie input 女弓木 (VND)..."), Args["canj"] its ASCII-letter form.
type KaikkiHeadTemplate struct {
	Args      map[string]string `json:"args"`
	Expansion string            `json:"expansion"`
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
// For Japanese and Ainu, "non-lemma forms" is not excluded — kana spellings
// of kanji words (e.g. わたし, あした) are tagged non-lemma but are the only
// form that passes IsPureHiragana, so excluding them would drop the word.
func isExcludedEntry(entry KaikkiEntry) bool {
	if strings.EqualFold(entry.Pos, "name") {
		return true
	}
	allowNonLemma := lang.IsJapaneseLang(entry.Lang) || lang.IsHangulLang(entry.Lang)
	for _, sense := range entry.Senses {
		tags, ok := sense["tags"].([]any)
		if !ok {
			continue
		}
		for _, t := range tags {
			tag, ok := t.(string)
			if !ok {
				continue
			}
			tag = strings.ToLower(tag)
			if tag == "non-lemma forms" && allowNonLemma {
				continue
			}
			if excludedSenseTags[tag] {
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
	"Min Bei", "Min Dong", "Gan", "Xiang", "Jin", "Cangjie", "Zhuyin",
}

// cangjieDialect names the pseudo-dialect whose "romanization" is each hanzi
// character's Cangjie input code (looked up via cangjieTable) rather than a
// spoken pronunciation pulled from a sound entry's zh_pron tags.
const cangjieDialect = "Cangjie"

// zhuyinDialect names the pseudo-dialect whose "romanization" is Mandarin's
// Zhuyin (Bopomofo) transcription, pulled from a sound entry tagged
// "Bopomofo" rather than matched by dialect name like romanizeEntry does.
const zhuyinDialect = "Zhuyin"

// zhuyinRomanize returns an entry's Mandarin Bopomofo reading (e.g. "ㄡ ㄎㄟˋ"),
// or "" if absent. Tone marks (ˊˇˋ˙) are spacing modifier-letter runes, not
// combining marks, so WordChars already splits them into their own tile
// without any special tone-handling — only the syllable separators (spaces)
// need stripping, done by zhuyinify.
func zhuyinRomanize(entry KaikkiEntry) string {
	for _, s := range entry.Sounds {
		if s.ZhPron == "" {
			continue
		}
		hasMandarin, hasBopomofo := false, false
		for _, t := range s.Tags {
			switch {
			case strings.EqualFold(t, "Mandarin"):
				hasMandarin = true
			case strings.EqualFold(t, "Bopomofo"):
				hasBopomofo = true
			}
		}
		if hasMandarin && hasBopomofo {
			return s.ZhPron
		}
	}
	return ""
}

// zhuyinify strips the space/hyphen syllable separators from a Bopomofo
// reading, concatenating its syllables (each already ending in its own tone
// mark, or no mark for first tone) into one guessable string.
func zhuyinify(rom string) string {
	return strings.Join(strings.FieldsFunc(rom, func(r rune) bool { return r == ' ' || r == '-' }), "")
}

// parseChineseDialect extracts the dialect name from a "Chinese (X)" pseudo-language.
func parseChineseDialect(lng string) (string, bool) {
	if !strings.HasPrefix(lng, "Chinese (") || !strings.HasSuffix(lng, ")") {
		return "", false
	}
	d := strings.TrimSuffix(strings.TrimPrefix(lng, "Chinese ("), ")")
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

// kaikkiURL builds the per-language dump URL on kaikki.org.
// Directory uses %20 for spaces; filename has spaces stripped entirely.
func kaikkiURL(lng string) string {
	slug := strings.ReplaceAll(lng, " ", "")
	u := &url.URL{
		Scheme: "https",
		Host:   "kaikki.org",
		Path:   fmt.Sprintf("/dictionary/%s/kaikki.org-dictionary-%s.jsonl.gz", lng, slug),
	}
	return u.String()
}

// cacheFilePath returns a cache file path for lng/length, creating the cache dir if needed.
func cacheFilePath(lng string, length int, suffix string) string {
	safe := strings.ToLower(strings.ReplaceAll(lng, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl%s.json", safe, length, suffix))
}

func cachePath(lng string, length int) string {
	return cacheFilePath(lng, length, "")
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

// defSeparator joins multiple distinct definitions for the same word within
// a single cached string entry — chosen because it can't appear in gloss
// prose, unlike "; " or newline.
const defSeparator = "\x1f"

// maxDefs caps how many distinct definitions are kept per word; callers
// display at most this many at game end.
const maxDefs = 3

// SplitDefinitions splits a cached definition string back into its
// individual glosses (as accumulated by addDef).
func SplitDefinitions(def string) []string {
	return strings.Split(def, defSeparator)
}

// addDef appends gloss to the word's stored definition string if it's
// non-empty, not a duplicate, and under maxDefs — used to accumulate
// definitions from multiple dictionary entries/senses of the same word.
func addDef(words map[string]string, word, gloss string) {
	if gloss == "" {
		return
	}
	existing := words[word]
	if existing == "" {
		words[word] = gloss
		return
	}
	parts := strings.Split(existing, defSeparator)
	if len(parts) >= maxDefs || slices.Contains(parts, gloss) {
		return
	}
	words[word] = existing + defSeparator + gloss
}

// cangjieTableCachePath caches the hanzi->Cangjie-code table, which is
// independent of word length so it's only ever fetched once.
func cangjieTableCachePath() string {
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "cangjie_table.json")
}

// cangjieGlossPattern catches "Cangjie input <glyphs> (<CODE>)" embedded in a
// character entry's head-template expansion or gloss prose — <glyphs> is the
// sequence of root characters (one per <CODE> letter) that the on-screen
// Cangjie keyboard actually displays and that guesses are checked against.
var cangjieGlossPattern = regexp.MustCompile(`Cangjie input (\S+) \(([A-Z]+)\)`)

// cangjieLetterGlyphs is the fixed 24-key Cangjie root-letter assignment
// (A-Y, no X — see https://en.wikipedia.org/wiki/Cangjie_input_method),
// used only as a fallback when an entry's "canj" arg has no parseable
// expansion/gloss text to pull the root glyphs from directly.
var cangjieLetterGlyphs = map[byte]string{
	'A': "日", 'B': "月", 'C': "金", 'D': "木", 'E': "水", 'F': "火", 'G': "土",
	'H': "竹", 'I': "戈", 'J': "十", 'K': "大", 'L': "中", 'M': "一", 'N': "弓",
	'O': "人", 'P': "心", 'Q': "手", 'R': "口", 'S': "尸", 'T': "廿", 'U': "山",
	'V': "女", 'W': "田", 'X': "難", 'Y': "卜",
}

// cangjieGlyphsFromCode converts an ASCII-letter Cangjie code (e.g. "VND")
// into its root-glyph form (e.g. "女弓木") via cangjieLetterGlyphs.
func cangjieGlyphsFromCode(code string) string {
	var b strings.Builder
	for i := 0; i < len(code); i++ {
		glyph, ok := cangjieLetterGlyphs[code[i]]
		if !ok {
			return ""
		}
		b.WriteString(glyph)
	}
	return b.String()
}

// cangjieCodeFromEntry extracts a character entry's Cangjie code as root
// glyphs, preferring the head-template/gloss prose (authoritative — kaikki's
// glyph rendering occasionally diverges from the textbook letter assignment)
// and falling back to converting the structured "canj" arg via the static
// letter table.
func cangjieCodeFromEntry(entry KaikkiEntry) string {
	for _, ht := range entry.HeadTemplates {
		if m := cangjieGlossPattern.FindStringSubmatch(ht.Expansion); m != nil {
			return m[1]
		}
	}
	for _, sense := range entry.Senses {
		glosses, ok := sense["glosses"].([]any)
		if !ok {
			continue
		}
		for _, g := range glosses {
			gloss, ok := g.(string)
			if !ok {
				continue
			}
			if m := cangjieGlossPattern.FindStringSubmatch(gloss); m != nil {
				return m[1]
			}
		}
	}
	for _, ht := range entry.HeadTemplates {
		if code := strings.ToUpper(ht.Args["canj"]); code != "" {
			if glyphs := cangjieGlyphsFromCode(code); glyphs != "" {
				return glyphs
			}
		}
	}
	return ""
}

// loadCangjieTable returns a per-character hanzi -> root-glyph Cangjie code
// map (e.g. "好" -> "女弓木"), sourced from kaikki.org's Translingual dump.
// Covers every length; callers filter down to the requested word length.
func loadCangjieTable() (map[string]string, error) {
	cf := cangjieTableCachePath()
	if data, err := os.ReadFile(cf); err == nil {
		var cached map[string]string
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	u := kaikkiURL("Translingual")
	log.Printf("Downloading Translingual wiktextract dump from %s for Cangjie codes", u)
	resp, err := http.Get(u)
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

	table := make(map[string]string)
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var entry KaikkiEntry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.Lang != "Translingual" || entry.Pos != "character" || len([]rune(entry.Word)) != 1 {
			continue
		}
		if _, exists := table[entry.Word]; exists {
			continue
		}
		if code := cangjieCodeFromEntry(entry); code != "" {
			table[entry.Word] = code
		}
	}
	if err := scanner.Err(); err != nil && len(table) < 100 {
		return nil, err
	}

	if data, err := json.Marshal(table); err == nil {
		if err := os.WriteFile(cf, data, 0644); err != nil {
			log.Printf("Warning: failed to write cangjie table cache %s: %v", cf, err)
		}
	}
	log.Printf("%d Cangjie character codes collected", len(table))
	return table, nil
}

// cangjieWord looks up a single hanzi character's root-glyph Cangjie code.
// Cangjie listings are single-character only — codes for different
// characters span 1-5 tiles, so "word" here is always one hanzi, never a
// multi-character dictionary word.
func cangjieWord(hanzi string, table map[string]string) string {
	if len([]rune(hanzi)) != 1 {
		return ""
	}
	code, ok := table[hanzi]
	if !ok {
		return ""
	}
	return code
}

// streamURL downloads and parses a gzipped JSONL word dump from kaikki.org.
// Parsing is parallelised across CPU workers while the scanner streams the download.
func streamURL(rawURL, lng, dialect string, length int, toneLang string, cangjieTable map[string]string, onProgress func(int)) (map[string]string, map[string]string, map[string]string, error) {
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
				if entry.Lang != lng || len(entry.Senses) == 0 || isExcludedEntry(entry) {
					continue
				}
				var word, hanzi string
				if dialect == cangjieDialect {
					word = cangjieWord(entry.Word, cangjieTable)
					if word == "" {
						continue
					}
					hanzi = entry.Word
				} else if dialect == zhuyinDialect {
					rom := zhuyinRomanize(entry)
					if rom == "" {
						continue
					}
					word = zhuyinify(rom)
					if word == "" {
						continue
					}
					hanzi = entry.Word
				} else if dialect != "" {
					rom := romanizeEntry(entry, dialect)
					if rom == "" {
						continue
					}
					word = lang.ChineseToneify(dialect, strings.ToLower(rom))
					if word == "" {
						continue
					}
					hanzi = entry.Word
				} else {
					word = strings.ToLower(entry.Word)
					if lang.IsHangulLang(lng) {
						word = lang.ExpandJamo(lang.DecomposeHangul(word))
						if !lang.IsPureJamo(word) {
							continue
						}
					} else if lang.IsJapaneseLang(lng) {
						word = lang.KatakanaToHiragana(word)
						if !lang.IsPureHiragana(word) {
							continue
						}
					} else if strings.EqualFold(lng, "Cherokee") {
						// Cherokee's lower-case block (Unicode 8 Cherokee
						// Supplement) isn't real orthography — everyday text
						// and the keyboard layout both use upper-case only.
						word = entry.Word
					}
				}
				if !lang.IsValid(word, length, toneLang) {
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
		_, existed := words[r.word]
		if !existed {
			words[r.word] = ""
		}
		addDef(words, r.word, r.def)
		if !existed {
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
func hanziCachePath(lng string, length int) string {
	return cacheFilePath(lng, length, "_hanzi")
}

// etymologyCachePath returns the sidecar cache path holding etymology text for a word list.
func etymologyCachePath(lng string, length int) string {
	return cacheFilePath(lng, length, "_etymology")
}

func loadWordList(lng string, length int) (map[string]string, map[string]string, map[string]string, error) {
	cf := cachePath(lng, length)
	hcf := hanziCachePath(lng, length)
	ecf := etymologyCachePath(lng, length)

	if data, err := os.ReadFile(cf); err == nil {
		var cached map[string]string
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) >= 20 {
			log.Printf("Loaded %d %s %d-letter words from cache (%s)", len(cached), lng, length, filepath.Base(cf))
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

	matchLang := lng
	dialect := ""
	if d, ok := parseChineseDialect(lng); ok {
		matchLang = "Chinese"
		dialect = d
	}

	var cangjieTable map[string]string
	if dialect == cangjieDialect {
		var err error
		cangjieTable, err = loadCangjieTable()
		if err != nil {
			return nil, nil, nil, err
		}
	}

	u := kaikkiURL(matchLang)
	log.Printf("Downloading %s wiktextract dump from %s", matchLang, u)

	toneLang := lang.ToneSplitKind(lng)

	key := fmt.Sprintf("%s:%d", lng, length)
	words, hanzi, etymology, err := streamURL(u, matchLang, dialect, length, toneLang, cangjieTable, func(n int) {
		DownloadProgress.Store(key, n)
	})
	DownloadProgress.Delete(key)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, nil, nil, fmt.Errorf("language %q not found on kaikki.org — check /api/languages for valid names", lng)
		}
		return nil, nil, nil, err
	}
	if len(words) == 0 {
		return nil, nil, nil, fmt.Errorf("no %d-character %s words found", length, lng)
	}

	log.Printf("%d %s %d-letter words collected", len(words), lng, length)

	if data, err := json.Marshal(words); err == nil {
		if err := os.WriteFile(cf, data, 0644); err == nil {
			log.Printf("Cached at %s", cf)
		}
	}
	if hanzi != nil {
		if data, err := json.Marshal(hanzi); err == nil {
			if err := os.WriteFile(hcf, data, 0644); err != nil {
				log.Printf("Warning: failed to write hanzi cache %s: %v", hcf, err)
			}
		}
	}
	if len(etymology) > 0 {
		if data, err := json.Marshal(etymology); err == nil {
			if err := os.WriteFile(ecf, data, 0644); err != nil {
				log.Printf("Warning: failed to write etymology cache %s: %v", ecf, err)
			}
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

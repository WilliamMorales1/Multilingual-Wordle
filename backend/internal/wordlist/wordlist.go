package wordlist

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"golang.org/x/net/html"

	"wordgo/internal/lang"
)

// permissiveJSON relaxes encoding/json/v2's two strictest defaults for data
// we don't produce ourselves: kaikki.org's wiktextract dumps (and cache files
// written by older builds) occasionally carry invalid UTF-8 or a repeated
// object member, which v2 rejects outright. Since every decode site treats an
// error as "skip this entry", the strict defaults would silently drop words
// that the previous encoding/json decoder accepted.
var permissiveJSON = json.JoinOptions(
	jsontext.AllowInvalidUTF8(true),
	jsontext.AllowDuplicateNames(true),
)

type KaikkiEntry struct {
	Word string `json:"word"`
	// These lists are decoded only to be counted: a word Wiktionary bothers
	// to translate, spin derived terms off, or cross-reference is a word
	// people actually use (see medianWeight).
	Translations    []struct{} `json:"translations"`
	Derived         []struct{} `json:"derived"`
	Synonyms        []struct{} `json:"synonyms"`
	Related         []struct{} `json:"related"`
	Descendants     []struct{} `json:"descendants"`
	Antonyms        []struct{} `json:"antonyms"`
	Hyponyms        []struct{} `json:"hyponyms"`
	Hypernyms       []struct{} `json:"hypernyms"`
	CoordinateTerms []struct{} `json:"coordinate_terms"`

	Lang          string               `json:"lang"`
	Pos           string               `json:"pos"`
	Senses        []map[string]any     `json:"senses"`
	Sounds        []KaikkiSound        `json:"sounds"` // for Chinese romanizations
	Etymology     string               `json:"etymology_text"`
	HeadTemplates []KaikkiHeadTemplate `json:"head_templates"`
	Redirects     []string             `json:"redirects"`
}

// KaikkiHeadTemplate carries the "Han char" infobox template on Translingual
// character entries - Expansion's prose embeds the Cangjie root-glyph code
// (e.g. "...Cangjie input 女弓木 (VND)..."), Args["canj"] its ASCII-letter form.
type KaikkiHeadTemplate struct {
	Args      map[string]string `json:"args"`
	Expansion string            `json:"expansion"`
}

var excludedSenseTags = map[string]bool{
	"proper nouns": true,
	"given names":  true,
	"surnames":     true,
}

// senseTags returns a sense's tags, lowercased.
func senseTags(sense map[string]any) []string {
	raw, ok := sense["tags"].([]any)
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(raw))
	for _, t := range raw {
		if tag, ok := t.(string); ok {
			tags = append(tags, strings.ToLower(tag))
		}
	}
	return tags
}

// isExcludedEntry reports whether an entry should be skipped because it (or
// all of its senses) is tagged as a proper noun, given name, or surname.
func isExcludedEntry(entry KaikkiEntry) bool {
	if strings.EqualFold(entry.Pos, "name") {
		return true
	}
	for _, sense := range entry.Senses {
		for _, tag := range senseTags(sense) {
			if excludedSenseTags[tag] {
				return true
			}
		}
	}
	return false
}

// rareSenseTags mark a sense nobody would think to guess. A word whose every
// sense is one of these doesn't count toward the language's median word
// length (English's dictionary is full of long obsolete words).
var rareSenseTags = map[string]bool{
	"obsolete":    true,
	"archaic":     true,
	"rare":        true,
	"uncommon":    true,
	"dated":       true,
	"historical":  true,
	"nonstandard": true,
	"misspelling": true,
	"poetic":      true,
	"literary":    true,
}

// commonSenseTags mark a sense as everyday vocabulary. Wiktionary only tags
// these occasionally, so they're a bonus on top of medianWeight's count,
// never a requirement.
var commonSenseTags = map[string]bool{
	"common":     true,
	"frequent":   true,
	"frequently": true,
}

// commonWordWeight multiplies the weight of a word Wiktionary tags as common.
const commonWordWeight = 3

// inflectionTags mark an entry as a form of another word (conjugations,
// plurals, abbreviations, alternative spellings) rather than a word in its
// own right. Those aren't words to guess — the Spanish dump alone carries
// every conjugation of every verb — so entries like these are skipped
// outright, in every language.
var inflectionTags = map[string]bool{
	"form-of":      true,
	"alt-of":       true,
	"abbreviation": true,
	"initialism":   true,
	"acronym":      true,
	"participle":   true,
	"plural":       true,
	"past":         true,
	"comparative":  true,
	"superlative":  true,
}

// isInflectedForm reports whether every one of an entry's senses just points
// at another word, by tag or by an explicit form_of/alt_of reference. An
// entry with one such sense among real ones (English "found", a verb of its
// own as well as the past of "find") is kept.
func isInflectedForm(entry KaikkiEntry) bool {
	if len(entry.Senses) == 0 {
		return false
	}
	for _, sense := range entry.Senses {
		if _, ok := sense["form_of"]; ok {
			continue
		}
		if _, ok := sense["alt_of"]; ok {
			continue
		}
		inflection := false
		for _, tag := range senseTags(sense) {
			if inflectionTags[tag] {
				inflection = true
				break
			}
		}
		if !inflection {
			return false
		}
	}
	return true
}

// crossReferences counts the entry lists that mark a word as widely used.
func crossReferences(entry KaikkiEntry) int {
	return len(entry.Translations) + len(entry.Derived) + len(entry.Synonyms) +
		len(entry.Related) + len(entry.Descendants) + len(entry.Antonyms) +
		len(entry.Hyponyms) + len(entry.Hypernyms) + len(entry.CoordinateTerms)
}

// medianWeight reports how much an entry counts toward its language's median
// word length. Counting dictionary entries equally gives a median far longer
// than the words anyone actually plays with — a dictionary is mostly long
// technical terms and inflected forms — so each entry is weighted by how
// much of a word it is in practice. The dumps carry no frequency data, so
// the weight is built from what every language's entries do carry:
//
//   - one point for existing, plus one for each cross-reference (translation,
//     derived term, synonym, antonym, descendant, hypernym…). Editors lavish
//     these on everyday words and give a long technical term none.
//     Deliberately uncapped: that long tail is the signal.
//   - times the number of senses. A word people use all day accumulates
//     senses; a single-sense entry is usually a technical one. This is what
//     carries languages other than English, whose entries in the English
//     Wiktionary are too sparse to cross-reference much (Spanish lists
//     translations on ~0% of entries, against English's ~4%).
//   - times commonWordWeight if a sense is tagged common.
//   - zero if every sense is tagged rare/obsolete/etc. Those words still
//     join the word list and stay guessable, they just don't sway the
//     median. (Inflected forms never get this far — isInflectedForm drops
//     them from the word list itself.)
func medianWeight(entry KaikkiEntry) int {
	weight := (1 + crossReferences(entry)) * max(1, len(entry.Senses))
	allRare := len(entry.Senses) > 0
	for _, sense := range entry.Senses {
		rare := false
		for _, tag := range senseTags(sense) {
			if commonSenseTags[tag] {
				return weight * commonWordWeight
			}
			if rareSenseTags[tag] {
				rare = true
			}
		}
		if !rare {
			allRare = false
		}
	}
	if allRare {
		return 0
	}
	return weight
}

type KaikkiSound struct {
	ZhPron string   `json:"zh_pron"`
	Tags   []string `json:"tags"`
}

// unplayableLanguages are kaikki.org index entries the game hides. Each one
// downloads fine but yields no word list anyone could play, so offering it
// only buys the player a long download and an error.
var unplayableLanguages = map[string]bool{
	// Not a language: the concatenation of every dump kaikki publishes,
	// tens of GB, and its file isn't even named after the entry.
	"All languages combined": true,
	// A jyutping pronunciation dump — every headword carries a tone digit
	// ("cyun3"), which no keyboard layout here types. The playable Cantonese
	// is "Chinese (Cantonese)", romanized off the Chinese dump.
	"Cantonese": true,
	// Tibetan-script, and almost entirely one- and two-syllable words: what
	// is left above minAutoLength is written with the tsheg separator (་),
	// which isn't a letter, so nothing survives.
	"Kurtöp": true,
	// Tangut script, a few hundred entries, one of them long enough to play.
	"Tangut": true,
	// A stub entry: 65 playable words in the whole dump, 15 at its fullest
	// length. Norwegian's actual vocabulary is under "Norwegian Bokmål" and
	// "Norwegian Nynorsk", both of which stay in the list.
	"Norwegian": true,
}

// chineseDialects lists topolects exposed as "Chinese (X)" languages.
// Each maps to the single kaikki.org "Chinese" dump, picking romanization by
// matching this name against a sound entry's tags (e.g. "Cantonese", "Hokkien").
var chineseDialects = []string{
	"Mandarin", "Cantonese", "Hokkien", "Teochew", "Hakka", "Wu",
	"Min Bei", "Min Dong", "Gan", "Xiang", "Jin", "Cangjie", "Zhuyin",
}

// "romanization" = cangjie input code
const cangjieDialect = "Cangjie"

// "romanization" = zhuyin transcription
const zhuyinDialect = "Zhuyin"

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

// Strips the space/hyphen syllable separators, keeping the bopomofo tone
// marks (ˊ ˇ ˋ ˙) — a zhuyin IME types those as their own key, so they stay
// as tiles. First tone is unmarked, as in the IME (its key is the spacebar).
func zhuyinify(rom string) string {
	return strings.Join(strings.FieldsFunc(rom, func(r rune) bool { return r == ' ' || r == '-' }), "")
}

// Extracts the dialect name from a "Chinese (X)" pseudo-language.
func parseChineseDialect(lng string) (string, bool) {
	inner, ok := strings.CutPrefix(lng, "Chinese (")
	if !ok {
		return "", false
	}
	d, ok := strings.CutSuffix(inner, ")")
	if !ok || d == "" {
		return "", false
	}
	return d, true
}

// The dumps hyphenate multi-word tags ("Min-Bei", "Min-Dong") while the
// language names spell them with a space, so both are folded to one form
// before comparing.
func sameDialectTag(tag, dialect string) bool {
	return strings.EqualFold(strings.ReplaceAll(tag, "-", " "), strings.ReplaceAll(dialect, "-", " "))
}

func romanizeEntry(entry KaikkiEntry, dialect string) string {
	for _, s := range entry.Sounds {
		if s.ZhPron == "" {
			continue
		}
		for _, t := range s.Tags {
			if sameDialectTag(t, dialect) {
				return s.ZhPron
			}
		}
	}
	return ""
}

// Directory uses %20 for spaces; the filename drops every character that
// isn't a letter or a digit — not just spaces, but the hyphens, apostrophes
// and parentheses in names like "Proto-Slavic", "Ge'ez" and "Yao (Africa)".
// Accented letters stay ("Franco-Provençal" -> "FrancoProvençal").
func kaikkiURL(lng string) string {
	slug := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, lng)
	u := &url.URL{
		Scheme: "https",
		Host:   "kaikki.org",
		Path:   fmt.Sprintf("/dictionary/%s/kaikki.org-dictionary-%s.jsonl.gz", lng, slug),
	}
	return u.String()
}

// cacheDir returns the directory used for on-disk caches, under DATA_DIR if set.
func cacheDir() string {
	dir := "cache"
	if d := os.Getenv("DATA_DIR"); d != "" {
		dir = filepath.Join(d, "cache")
	}
	os.MkdirAll(dir, 0755)
	return dir
}

// cacheFilePath returns a cache file path for lng/length, creating dir if needed.
func cacheFilePath(lng string, length int, suffix string) string {
	safe := strings.ToLower(strings.ReplaceAll(lng, " ", "_"))
	return filepath.Join(cacheDir(), fmt.Sprintf("%s_%dl%s.json", safe, length, suffix))
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

// For Japanese hiragana redirects without direct glosses
func formOfWord(entry KaikkiEntry) string {
	if len(entry.Redirects) > 0 {
		return entry.Redirects[0]
	}
	for _, sense := range entry.Senses {
		formOf, ok := sense["form_of"].([]any)
		if !ok {
			continue
		}
		for _, f := range formOf {
			fm, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if w, ok := fm["word"].(string); ok && w != "" {
				return w
			}
		}
	}
	return ""
}

// Joins definitions, chosen over \n or "; " because it doesn't appear in glosses
const defSeparator = "\x1f"

// Caps how many distinct definitions are shown
const maxDefs = 3

// Split cached definition back to its glosses
func SplitDefinitions(def string) []string {
	return strings.Split(def, defSeparator)
}

// Appends gloss to the word's stored definition
// if non-empty && non-duplicate && under maxDefs
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

// cangjieTableCachePath caches the hanzi->Cangjie-code table
func cangjieTableCachePath() string {
	return filepath.Join(cacheDir(), "cangjie_table.json")
}

// Catches "Cangjie input <glyphs> (<CODE>)" embedded in a
// character entry's head-template expansion or gloss prose - <glyphs> is the
// sequence of root characters (one per <CODE> letter) that the on-screen
// Cangjie keyboard actually displays and that guesses are checked against.
var cangjieGlossPattern = regexp.MustCompile(`Cangjie input (\S+) \(([A-Z]+)\)`)

// see https://en.wikipedia.org/wiki/Cangjie_input_method)
var cangjieLetterGlyphs = map[byte]string{
	'A': "日", 'B': "月", 'C': "金", 'D': "木", 'E': "水", 'F': "火", 'G': "土",
	'H': "竹", 'I': "戈", 'J': "十", 'K': "大", 'L': "中", 'M': "一", 'N': "弓",
	'O': "人", 'P': "心", 'Q': "手", 'R': "口", 'S': "尸", 'T': "廿", 'U': "山",
	'V': "女", 'W': "田", 'X': "難", 'Y': "卜",
}

// ASCII codes to Cangjie root-glyph codes
func cangjieGlyphsFromCode(code string) string {
	var b strings.Builder
	for i := range len(code) {
		glyph, ok := cangjieLetterGlyphs[code[i]]
		if !ok {
			return ""
		}
		b.WriteString(glyph)
	}
	return b.String()
}

// cangjieCodeFromEntry extracts a character entry's Cangjie code as root
// glyphs, preferring the head-template/gloss prose (authoritative - kaikki's
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
		if err := json.Unmarshal(data, &cached, permissiveJSON); err == nil && len(cached) > 0 {
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
		if json.Unmarshal(scanner.Bytes(), &entry, permissiveJSON) != nil {
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

// cangjieCodeForChar returns a single hanzi's Cangjie root-glyph code
// (e.g. "好" -> "女弓木"), or "" if the table doesn't cover it.
func cangjieCodeForChar(hanzi string, table map[string]string) string {
	if len([]rune(hanzi)) != 1 {
		return ""
	}
	code, ok := table[hanzi]
	if !ok {
		return ""
	}
	return code
}

// AutoLength asks streamURL to pick the word length from the language's own
// median word length instead of filtering to a length chosen up front.
const AutoLength = 0

// Bounds on an auto-picked length: below 3 tiles a word carries too little
// information to guess, above 12 the board stops fitting on a phone. Words
// outside this range are dropped as they're parsed - they can never be the
// answer, so they're never buffered, measured or cached.
const (
	minAutoLength = 3
	maxAutoLength = 12
)

// minWordListSize is the smallest word list a language can be played with.
const minWordListSize = 20

// medianLengthSample is how many words are buffered to take the median of
// before committing to a length. Large enough that the median is stable,
// small enough that the buffer stays a few tens of MB on a big language; a
// language with fewer words than this takes the median of all of them.
const medianLengthSample = 100000

// pickAutoLength returns a language's median word length — weights maps a
// tile count to the total weight of the words that long, so a common word
// counts for several ordinary ones. The median ignores outliers by
// construction: however long a language's longest words get, they only ever
// move it one word at a time.
func pickAutoLength(weights map[int]int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return minAutoLength
	}

	// The median sits at weight position (total-1)/2, counting from 0.
	half, seen := (total-1)/2, 0
	for _, length := range slices.Sorted(maps.Keys(weights)) {
		seen += weights[length]
		if seen > half {
			return min(max(length, minAutoLength), maxAutoLength)
		}
	}
	return maxAutoLength
}

// fullestLength returns the tile count with the most words behind it,
// preferring the shorter one when two are tied.
func fullestLength(counts map[int]int) int {
	best := 0
	for _, length := range slices.Sorted(maps.Keys(counts)) {
		if counts[length] > counts[best] {
			best = length
		}
	}
	return best
}

// Parsing is parallelised across CPU workers while the scanner streams the download.
// length is the tile count to keep, or AutoLength to take the median of the
// language's words and keep that length; the length actually used is returned.
func streamURL(rawURL, lng, dialect string, length int, toneLang string, cangjieTable map[string]string, onProgress func(int)) (map[string]string, map[string]string, map[string]string, int, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, nil, nil, length, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, length, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, nil, nil, length, err
	}
	defer gz.Close()

	// kanjiDef marks side-table entries: kanji lemma defs keyed by kana reading.
	type result struct {
		word, def, hanzi, etymology string
		tiles                       int
		weight                      int // how much it counts toward the average length
		kanjiDef                    bool
	}

	// chosenLen is the length workers filter on. In auto mode it stays 0
	// until the collector has averaged enough words to commit to one.
	var chosenLen atomic.Int64
	chosenLen.Store(int64(length))

	isJP := lang.IsJapaneseLang(lng)

	numWorkers := runtime.NumCPU()
	lines := make(chan []byte, numWorkers*8)
	results := make(chan result, numWorkers*8)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for line := range lines {
				var entry KaikkiEntry
				if json.Unmarshal(line, &entry, permissiveJSON) != nil {
					continue
				}
				if entry.Lang != lng || len(entry.Senses) == 0 || isExcludedEntry(entry) || isInflectedForm(entry) {
					continue
				}
				var word, hanzi string
				if dialect == cangjieDialect {
					word = cangjieCodeForChar(entry.Word, cangjieTable)
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
					word = lang.ChineseRomanize(dialect, strings.ToLower(rom))
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
					} else if isJP {
						word = lang.KatakanaToHiragana(word)
						if !lang.IsPureHiragana(word) {
							// Not a playable kana word, but may be a kanji lemma whose
							// kana reading matches a word in the list. Key the side-table
							// by the kana reading (from head_templates[0].args["1"])
							// This is so, for example, 辛い(からい)="spicy" and 辛い(つらい)="painful"
							// are kept separately and looked up by the exact hiragana form.
							if gloss := firstGloss(entry); gloss != "" {
								reading := ""
								if len(entry.HeadTemplates) > 0 {
									r := lang.KatakanaToHiragana(strings.ToLower(entry.HeadTemplates[0].Args["1"]))
									if lang.IsPureHiragana(r) {
										reading = r
									}
								}
								if reading != "" {
									results <- result{
										word:     reading,
										def:      gloss,
										kanjiDef: true,
									}
								}
							}
							continue
						}
					} else if strings.EqualFold(lng, "Cherokee") {
						// Cherokee's lower-case block (Unicode 8 Cherokee
						// Supplement) isn't real orthography - everyday text
						// and the keyboard layout both use upper-case only.
						word = entry.Word
					}
				}
				if !lang.IsWord(word) {
					continue
				}
				tiles := lang.WordLen(word, toneLang)
				if want := int(chosenLen.Load()); want != AutoLength {
					if tiles != want {
						continue
					}
				} else if tiles < minAutoLength || tiles > maxAutoLength {
					continue
				}
				results <- result{
					word:      word,
					def:       firstGloss(entry),
					hanzi:     hanzi,
					etymology: entry.Etymology,
					tiles:     tiles,
					weight:    medianWeight(entry),
				}
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
			lines <- bytes.Clone(scanner.Bytes())
		}
		scanErr = scanner.Err()
	}()

	words := make(map[string]string)
	etymology := make(map[string]string)
	var hanziMap map[string]string
	if dialect != "" {
		hanziMap = make(map[string]string)
	}
	// kanjiDefs maps kana reading -> definition for kanji entries, keyed by reading
	// so that homograph kanji (e.g. 辛い read as からい vs つらい) stay separate.
	kanjiDefs := make(map[string]string)

	add := func(r result) {
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
			// Only a new word moves the counter. Logging on every add would
			// repeat the same line for each extra definition landing on an
			// already-known word while the count sits on a multiple of 500.
			if len(words)%500 == 0 {
				log.Printf("  %d words collected...", len(words))
				if onProgress != nil {
					onProgress(len(words))
				}
			}
		}
	}

	// In auto mode words of every playable length are buffered until there
	// are enough to measure; their median then fixes the length, the buffer
	// is replayed keeping only that length, and the workers filter the rest
	// of the stream themselves.
	var buffered []result
	sampled, totalWeight := 0, 0
	weights := make(map[int]int)    // tile count -> weight of the words that long
	unweighted := make(map[int]int) // same, counting every word once
	commit := func() {
		if totalWeight > 0 {
			length = pickAutoLength(weights)
		} else {
			// Every sampled word was tagged rare (or the language tags no
			// senses at all) - fall back to counting them all equally.
			length = pickAutoLength(unweighted)
		}
		if sampled > 0 {
			log.Printf("%s: median of %d words (weighted by how widely used each is) is %d tiles",
				lng, sampled, length)
		}
		// On a thin language the median can land on a length with barely a
		// word in it (Norwegian's median is 7 tiles, and it has eight such
		// words - most Norwegian entries live under Bokmål and Nynorsk).
		// Play its biggest bucket instead. Only worth doing when the whole
		// dump fit in the sample: past that the counts are a prefix of the
		// stream, and the median's own bucket keeps growing anyway.
		if sampled < medianLengthSample && unweighted[length] < minWordListSize {
			if best := fullestLength(unweighted); unweighted[best] > unweighted[length] {
				log.Printf("%s: only %d words that long - playing %d tiles instead (%d words)",
					lng, unweighted[length], best, unweighted[best])
				length = best
			}
		}
		chosenLen.Store(int64(length))
		for _, b := range buffered {
			if b.tiles == length {
				add(b)
			}
		}
		buffered = nil
	}

	for r := range results {
		if r.kanjiDef {
			if _, exists := kanjiDefs[r.word]; !exists {
				kanjiDefs[r.word] = r.def
			}
			continue
		}
		if length == AutoLength {
			buffered = append(buffered, r)
			weights[r.tiles] += r.weight
			totalWeight += r.weight
			unweighted[r.tiles]++
			sampled++
			if sampled >= medianLengthSample {
				commit()
			}
			continue
		}
		// A worker may still be holding a word from before the length was
		// committed, so re-check here.
		if r.tiles != length {
			continue
		}
		add(r)
	}
	if length == AutoLength {
		commit()
	}
	// Fill in definitions for Japanese kana words that had no gloss of their own
	// by looking up the kanji lemma's definition keyed by this exact kana reading.
	for w := range words {
		if words[w] == "" {
			if def, ok := kanjiDefs[w]; ok {
				addDef(words, w, def)
			}
		}
	}

	if scanErr != nil {
		if len(words) >= 20 {
			log.Printf("Warning: scanner error after %d words (%v) - using partial results", len(words), scanErr)
			return words, hanziMap, etymology, length, nil
		}
		return nil, nil, nil, length, scanErr
	}
	return words, hanziMap, etymology, length, nil
}

// loadWordList returns the word list for lng at the given length, or — with
// AutoLength — at the language's own median word length, which it returns.
func loadWordList(lng string, length int) (map[string]string, map[string]string, map[string]string, int, error) {
	// streamURL overwrites length in auto mode, so remember what was asked
	// for: only a measured length is worth recording.
	requested := length
	if length != AutoLength {
		if words, hanzi, etymology, ok := loadCachedWordList(lng, length); ok {
			return words, hanzi, etymology, length, nil
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
			return nil, nil, nil, length, err
		}
	}

	u := kaikkiURL(matchLang)
	log.Printf("Downloading %s wiktextract dump from %s", matchLang, u)

	toneLang := lang.ToneSplitKind(lng)

	// Keyed by language alone: with AutoLength the length isn't known until
	// partway through the download, and a progress poll can't name it.
	words, hanzi, etymology, length, err := streamURL(u, matchLang, dialect, length, toneLang, cangjieTable, func(n int) {
		DownloadProgress.Store(lng, n)
	})
	DownloadProgress.Delete(lng)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, nil, nil, length, fmt.Errorf("language %q not found on kaikki.org - check /api/languages for valid names", lng)
		}
		return nil, nil, nil, length, err
	}
	if len(words) < minWordListSize {
		// Not just "none found": a handful of words is a list nobody can
		// play — the answer is most of it, so there is nothing to guess
		// with. Same threshold loadCachedWordList uses to decide a cache
		// file is too thin to bother with.
		if requested != AutoLength {
			// A remembered median that no longer holds enough words (or a
			// length the caller picked itself). Drop it and re-measure,
			// which lands on the language's fullest length instead.
			log.Printf("%s has only %d playable %d-character words - re-measuring its word length", lng, len(words), length)
			forgetMedianLength(lng)
			return loadWordList(lng, AutoLength)
		}
		return nil, nil, nil, length, fmt.Errorf("%s has only %d playable %d-character words - not enough for a game",
			lng, len(words), length)
	}
	recordMeasuredLength(lng, requested, length)

	log.Printf("%d %s %d-letter words collected", len(words), lng, length)

	cf := cacheFilePath(lng, length, "")
	hcf := cacheFilePath(lng, length, "_hanzi")
	ecf := cacheFilePath(lng, length, "_etymology")

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
	return words, hanzi, etymology, length, nil
}

// recordMeasuredLength remembers a language's word length, but only when the
// load actually measured one. A caller that asked for a specific length has
// measured nothing, and recording its choice would overwrite the language's
// median — and with it the length every later game is created at.
func recordMeasuredLength(lng string, requested, measured int) {
	if requested != AutoLength {
		return
	}
	recordMedianLength(lng, measured)
}

// loadCachedWordList reads a lang/length's word list off disk, reporting
// whether a usable one was there.
func loadCachedWordList(lng string, length int) (map[string]string, map[string]string, map[string]string, bool) {
	cf := cacheFilePath(lng, length, "")
	data, err := os.ReadFile(cf)
	if err != nil {
		return nil, nil, nil, false
	}
	var cached map[string]string
	if err := json.Unmarshal(data, &cached, permissiveJSON); err != nil || len(cached) < minWordListSize {
		return nil, nil, nil, false
	}
	log.Printf("Loaded %d %s %d-letter words from cache (%s)", len(cached), lng, length, filepath.Base(cf))

	var hanzi map[string]string
	if hdata, err := os.ReadFile(cacheFilePath(lng, length, "_hanzi")); err == nil {
		json.Unmarshal(hdata, &hanzi, permissiveJSON)
	}
	var etymology map[string]string
	if edata, err := os.ReadFile(cacheFilePath(lng, length, "_etymology")); err == nil {
		json.Unmarshal(edata, &etymology, permissiveJSON)
	}
	return cached, hanzi, etymology, true
}

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
					if !strings.Contains(decoded, ".") && !unplayableLanguages[decoded] {
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

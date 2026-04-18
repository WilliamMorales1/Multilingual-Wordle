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

type KaikkiEntry struct {
	Word   string           `json:"word"`
	Lang   string           `json:"lang"`
	Senses []map[string]any `json:"senses"`
}

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

func wordLen(word string) int {
	return len(wordChars(word))
}

func isWordChar(r rune) bool {
	return unicode.In(r,
		unicode.Ll, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Lm,
		unicode.Mn, unicode.Mc, unicode.Me)
}

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

func cachePath(lang string, length int) string {
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	dir := dataPath("cache")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%s_%dl.json", safe, length))
}

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
	lines := make(chan []byte, numWorkers*8)
	results := make(chan result, numWorkers*8)

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

// matchWildcard finds a canonical word from normSet whose grapheme clusters match
// guessChars, where a guessChar of "*" matches only chars whose normalizeChar is
// in overflowBaseSet (the keyboard-overflow equivalence group).
// Non-wildcard positions are compared accent-insensitively via normalizeChar.
// Returns the canonical word on first match, or "" if none found.
func matchWildcard(guessChars []string, normSet map[string]string, overflowBaseSet map[string]bool) string {
	n := len(guessChars)
	for _, canonical := range normSet {
		cChars := wordChars(canonical)
		if len(cChars) != n {
			continue
		}
		match := true
		for i, gc := range guessChars {
			if gc == "*" {
				if !overflowBaseSet[normalizeChar(cChars[i])] {
					match = false
					break
				}
				continue
			}
			if normalizeChar(gc) != normalizeChar(cChars[i]) {
				match = false
				break
			}
		}
		if match {
			return canonical
		}
	}
	return ""
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

// ── Keyboard layout data ──────────────────────────────────────────────────────

var keyboardLayouts = map[string][][]string{
	"qwerty":     {{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l"}, {"z", "x", "c", "v", "b", "n", "m"}},
	"azerty":     {{"a", "z", "e", "r", "t", "y", "u", "i", "o", "p"}, {"q", "s", "d", "f", "g", "h", "j", "k", "l", "m"}, {"w", "x", "c", "v", "b", "n"}},
	"qwertz":     {{"q", "w", "e", "r", "t", "z", "u", "i", "o", "p"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l"}, {"y", "x", "c", "v", "b", "n", "m"}},
	"nordic":     {{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "å"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l", "ø", "æ"}, {"z", "x", "c", "v", "b", "n", "m"}},
	"turkish":    {{"q", "w", "e", "r", "t", "y", "u", "ı", "o", "p", "ğ", "ü"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l", "ş", "i"}, {"z", "x", "c", "v", "b", "n", "m", "ö", "ç"}},
	"jcuken":     {{"й", "ц", "у", "к", "е", "н", "г", "ш", "щ", "з", "х"}, {"ф", "ы", "в", "а", "п", "р", "о", "л", "д", "ж", "э"}, {"я", "ч", "с", "м", "и", "т", "ь", "б", "ю"}},
	"greek":      {{"ε", "ρ", "τ", "υ", "θ", "ι", "ο", "π"}, {"α", "σ", "δ", "φ", "γ", "η", "ξ", "κ", "λ"}, {"ζ", "χ", "ψ", "ω", "β", "ν", "μ"}},
	"arabic":     {{"ض", "ص", "ث", "ق", "ف", "غ", "ع", "ه", "خ", "ح", "ج", "د"}, {"ش", "س", "ي", "ب", "ل", "ا", "ت", "ن", "م", "ك", "ط", "ذ"}, {"ئ", "ء", "ؤ", "ر", "ى", "ة", "و", "ز", "ظ"}},
	"hebrew":     {{"ק", "ר", "א", "ט", "ו", "ן", "ם", "פ"}, {"ש", "ד", "ג", "כ", "ע", "י", "ח", "ל", "ך", "ף"}, {"ז", "ס", "ב", "ה", "נ", "צ", "ת", "ץ"}},
	"devanagari": {{"औ", "ऐ", "आ", "ई", "ऊ", "भ", "ङ", "घ", "ध", "झ", "ढ", "ञ"}, {"ओ", "ए", "अ", "इ", "उ", "ब", "ह", "ग", "द", "ज", "ड", "श"}, {"ऑ", "ृ", "र", "क", "त", "च", "ट", "प", "य", "स", "म", "व", "ल", "ष", "न"}},
	"bengali":    {{"ঔ", "ঐ", "আ", "ঈ", "ঊ", "ভ", "ঙ", "ঘ", "ধ", "ঝ", "ঢ", "ঞ"}, {"ও", "এ", "অ", "ই", "উ", "ব", "হ", "গ", "দ", "জ", "ড", "শ"}, {"ঋ", "র", "ক", "ত", "চ", "ট", "প", "য", "স", "ম", "ব", "ল", "ষ", "ন"}},
	"tamil":      {{"ஔ", "ஐ", "ஆ", "ஈ", "ஊ", "ங", "ஞ", "ண", "ந", "ன"}, {"ஓ", "ஏ", "அ", "இ", "உ", "க", "ச", "ட", "த", "ப", "ற"}, {"எ", "ஒ", "ய", "ர", "ல", "வ", "ழ", "ள", "ம", "ஷ", "ஸ", "ஹ"}},
	"telugu":     {{"ఔ", "ఐ", "ఆ", "ఈ", "ఊ", "భ", "ఙ", "ఘ", "ధ", "ఝ", "ఢ", "ఞ"}, {"ఓ", "ఏ", "అ", "ఇ", "ఉ", "బ", "హ", "గ", "ద", "జ", "డ", "శ"}, {"ఎ", "ఒ", "ర", "క", "త", "చ", "ట", "ప", "య", "స", "మ", "వ", "ల", "ష", "న"}},
	"thai":       {{"โ", "ฌ", "ฆ", "ฏ", "โ", "ซ", "ศ", "ฮ", "?", "ฒ", "ฬ", "ฦ"}, {"ฟ", "ห", "ก", "ด", "เ", "า", "ส", "ว", "ง", "ผ", "ป", "แ", "อ"}, {"พ", "ะ", "ั", "ร", "น", "ย", "บ", "ล", "ข", "ช", "ต", "ค", "ม"}},
	"hiragana":   {{"わ", "ら", "や", "ま", "は", "な", "た", "さ", "か", "あ"}, {"ゐ", "り", "み", "ひ", "に", "ち", "し", "き", "い"}, {"ん", "る", "ゆ", "む", "ふ", "ぬ", "つ", "す", "く", "う"}, {"ゑ", "れ", "め", "へ", "ね", "て", "せ", "け", "え"}, {"を", "ろ", "よ", "も", "ほ", "の", "と", "そ", "こ", "お"}},
	"katakana":   {{"ワ", "ラ", "ヤ", "マ", "ハ", "ナ", "タ", "サ", "カ", "ア"}, {"ヰ", "リ", "ミ", "ヒ", "ニ", "チ", "シ", "キ", "イ"}, {"ン", "ル", "ユ", "ム", "フ", "ヌ", "ツ", "ス", "ク", "ウ"}, {"ヱ", "レ", "メ", "ヘ", "ネ", "テ", "セ", "ケ", "エ"}, {"ヲ", "ロ", "ヨ", "モ", "ホ", "ノ", "ト", "ソ", "コ", "オ"}},
}

var langLayoutMap = map[string]string{
	"French": "azerty", "German": "qwertz", "Norwegian": "nordic",
	"Danish": "nordic", "Swedish": "nordic", "Turkish": "turkish",
}

// detectLayout infers a keyboard layout name from the alphabet's script.
func detectLayout(alphabet []string) string {
	joined := strings.Join(alphabet, "")
	// Latin check first
	for _, r := range joined {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return "qwerty"
		}
	}
	for _, r := range joined {
		switch {
		case r >= 0x0400 && r <= 0x04FF:
			return "jcuken"
		case (r >= 0x0370 && r <= 0x03FF) || (r >= 0x1F00 && r <= 0x1FFF):
			return "greek"
		case r >= 0x0600 && r <= 0x06FF:
			return "arabic"
		case r >= 0x05D0 && r <= 0x05EA:
			return "hebrew"
		case r >= 0x0900 && r <= 0x097F:
			return "devanagari"
		case r >= 0x0980 && r <= 0x09FF:
			return "bengali"
		case r >= 0x0B80 && r <= 0x0BFF:
			return "tamil"
		case r >= 0x0C00 && r <= 0x0C7F:
			return "telugu"
		case r >= 0x0E00 && r <= 0x0E7F:
			return "thai"
		case r >= 0x3040 && r <= 0x309F:
			return "hiragana"
		case r >= 0x30A0 && r <= 0x30FF:
			return "katakana"
		}
	}
	return "qwerty"
}

// buildKeyboardData returns the keyboard rows (base chars only) and overflow
// bases (alphabet bases not placed on any layout key) for a given alphabet/lang.
// Mirrors the JS buildKeyboardRows logic exactly.
func buildKeyboardData(alphabet []string, lang string) (rows [][]string, overflowBases []string) {
	if len(alphabet) == 0 {
		return nil, nil
	}

	// Map each alphabet grapheme to its diacritic-stripped base.
	basesInAlphabet := make(map[string]bool, len(alphabet))
	for _, ch := range alphabet {
		basesInAlphabet[normalizeChar(ch)] = true
	}

	layoutName := langLayoutMap[lang]
	if layoutName == "" {
		layoutName = detectLayout(alphabet)
	}
	layout, ok := keyboardLayouts[layoutName]
	if !ok {
		layout = keyboardLayouts["qwerty"]
	}

	placedBases := make(map[string]bool)
	for _, layoutRow := range layout {
		var row []string
		for _, base := range layoutRow {
			if basesInAlphabet[base] {
				row = append(row, base)
				placedBases[base] = true
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	for base := range basesInAlphabet {
		if !placedBases[base] {
			overflowBases = append(overflowBases, base)
		}
	}
	sort.Strings(overflowBases)

	if len(rows) == 0 {
		return nil, overflowBases
	}
	return rows, overflowBases
}

// computeEquivalences groups alphabet chars by their base (diacritic-stripped)
// form, returning only groups with >1 member or the wildcard '*' group (overflow).
// Each group is [base/label, variant1, variant2, ...] sorted.
func computeEquivalences(alphabet []string, overflowBaseSet map[string]bool) [][]string {
	if len(alphabet) == 0 {
		return nil
	}

	type set = map[string]bool
	groups := make(map[string]set)
	for _, ch := range alphabet {
		base := normalizeChar(ch)
		if overflowBaseSet[base] {
			base = "*"
		}
		if groups[base] == nil {
			groups[base] = make(set)
		}
		groups[base][ch] = true
	}

	var result [][]string
	for base, chars := range groups {
		if len(chars) <= 1 && base != "*" {
			continue
		}
		members := make([]string, 0, len(chars)+1)
		if base == "*" {
			members = append(members, "*")
			for ch := range chars {
				members = append(members, ch)
			}
			sort.Strings(members[1:])
		} else {
			members = append(members, base)
			for ch := range chars {
				if ch != base {
					members = append(members, ch)
				}
			}
			sort.Strings(members[1:])
		}
		result = append(result, members)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i][0] == "*" {
			return true
		}
		if result[j][0] == "*" {
			return false
		}
		return result[i][0] < result[j][0]
	})
	return result
}

func isRTL(alphabet []string) bool {
	for _, ch := range alphabet {
		for _, r := range ch {
			if (r >= 0x0600 && r <= 0x06FF) || (r >= 0x05D0 && r <= 0x05EA) {
				return true
			}
		}
	}
	return false
}

// buildGameExtras computes all derived UI data from the alphabet in one call.
func buildGameExtras(alphabet []string, lang string) (keyboardRows [][]string, overflowBases []string, equivalences [][]string, rtl bool) {
	keyboardRows, overflowBases = buildKeyboardData(alphabet, lang)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}
	equivalences = computeEquivalences(alphabet, overflowSet)
	rtl = isRTL(alphabet)
	return
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

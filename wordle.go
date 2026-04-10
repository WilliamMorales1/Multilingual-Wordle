package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Green  = "\033[42m\033[97m"
	Yellow = "\033[43m\033[30m"
	Gray   = "\033[100m\033[97m"
	White  = "\033[47m\033[30m"
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

// Guess represents a single guess with its evaluation states
type Guess struct {
	Word   string
	States []string
}

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

// kaikkiURL constructs the URL for a language's word dump
func kaikkiURL(lang string) string {
	slug := url.QueryEscape(lang)
	return fmt.Sprintf("https://kaikki.org/dictionary/%s/kaikki.org-dictionary-%s.jsonl.gz", slug, slug)
}

// cachePath returns the path to the cache file
func cachePath(lang string, length int) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	safe := strings.ToLower(strings.ReplaceAll(lang, " ", "_"))
	return filepath.Join(home, fmt.Sprintf(".wordle_%s_%dl_cache.json", safe, length))
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

// streamURL downloads and processes a gzipped JSONL file
func streamURL(url, lang string, length int) (map[string]string, error) {
	words := make(map[string]string)

	resp, err := http.Get(url)
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

	scanner := bufio.NewScanner(gz)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var entry KaikkiEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if entry.Lang != lang {
			continue
		}
		if !isValid(entry.Word, length) {
			continue
		}
		if len(entry.Senses) == 0 {
			continue
		}

		key := strings.ToLower(entry.Word)
		if _, exists := words[key]; !exists {
			words[key] = firstGloss(entry)
		}

		if len(words)%500 == 0 {
			fmt.Printf("\r  %d words collected…", len(words))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return words, nil
}

// streamKaikki downloads word list from kaikki.org
func streamKaikki(lang string, length int) (map[string]string, error) {
	url := kaikkiURL(lang)

	fmt.Printf("  Downloading %s wiktextract dump from kaikki.org...\n", lang)
	fmt.Printf("  URL: %s\n", url)
	fmt.Println("  (This only happens once per language/length; results are cached.)")

	words, err := streamURL(url, lang, length)

	if err != nil && strings.Contains(err.Error(), "HTTP 404") {
		fmt.Printf("\n  No dedicated dump found for '%s' (HTTP 404).\n", lang)
		fmt.Println("  Falling back to the full raw wiktextract dump (~2.3 GB compressed).")
		fmt.Println("  This will take several minutes but only happens once.")

		fmt.Print("  Proceed with large download? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(strings.ToLower(yn))

		if yn != "y" {
			fmt.Println("\n  Aborted. Check language name at https://kaikki.org/dictionary/")
			os.Exit(0)
		}

		fmt.Printf("\n  Streaming %s …\n\n", rawDumpURL)
		words, err = streamURL(rawDumpURL, lang, length)

		if err != nil {
			return nil, err
		}

		if len(words) == 0 {
			fmt.Printf("\n  No %d-character words found for '%s' in the raw dump.\n", length, lang)
			fmt.Println("  The language name may be wrong. Check: https://kaikki.org/dictionary/")
			os.Exit(1)
		}
	} else if err != nil {
		return nil, err
	}

	fmt.Printf("\r  %d %s %d-character words collected.   \n", len(words), lang, length)
	return words, nil
}

// loadWordList loads or downloads the word list
func loadWordList(lang string, length int) (map[string]string, error) {
	cf := cachePath(lang, length)

	if data, err := os.ReadFile(cf); err == nil {
		var cached map[string]string
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) >= 20 {
			fmt.Printf("  Loaded %d words from cache (%s).\n", len(cached), filepath.Base(cf))
			return cached, nil
		}
	}

	words, err := streamKaikki(lang, length)
	if err != nil {
		return nil, err
	}

	if len(words) == 0 {
		return nil, fmt.Errorf("no %d-character %s words found", length, lang)
	}

	// Cache the results
	if data, err := json.Marshal(words); err == nil {
		if err := os.WriteFile(cf, data, 0644); err == nil {
			fmt.Printf("  Cached at %s\n", cf)
		}
	}

	return words, nil
}

// isWide checks if a character is double-width (CJK, etc.)
func isWide(char string) bool {
	if len(char) == 0 {
		return false
	}
	r := []rune(char)[0]
	// East Asian Width property check (simplified)
	return r >= 0x1100 && (r <= 0x115F || // Hangul Jamo
		(r >= 0x2E80 && r <= 0x9FFF) || // CJK
		(r >= 0xAC00 && r <= 0xD7AF) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
		(r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Forms
		(r >= 0x20000 && r <= 0x2FFFD) || // CJK Extension
		(r >= 0x30000 && r <= 0x3FFFD))
}

// tile creates a colored tile for a character
func tile(char, state string) string {
	pad := " "
	if isWide(char) {
		pad = ""
	}
	c := strings.ToUpper(char)

	switch state {
	case "correct":
		return fmt.Sprintf("%s%s%s%s%s", Green, pad, c, pad, Reset)
	case "present":
		return fmt.Sprintf("%s%s%s%s%s", Yellow, pad, c, pad, Reset)
	case "absent":
		return fmt.Sprintf("%s%s%s%s%s", Gray, pad, c, pad, Reset)
	default:
		return fmt.Sprintf("%s%s%s%s%s", White, pad, c, pad, Reset)
	}
}

// printBoard displays the game board
func printBoard(guesses []Guess, maxGuesses, wlen int) {
	fmt.Println()
	for _, guess := range guesses {
		chars := wordChars(guess.Word)
		fmt.Print("  ")
		for i, ch := range chars {
			fmt.Print(tile(ch, guess.States[i]))
		}
		fmt.Println()
	}

	blank := fmt.Sprintf("%s   %s", Gray, Reset)
	for i := 0; i < maxGuesses-len(guesses); i++ {
		fmt.Print("  ")
		for j := 0; j < wlen; j++ {
			fmt.Print(blank)
		}
		fmt.Println()
	}
	fmt.Println()
}

// buildAlphabet collects all unique grapheme clusters from the word list
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

// printKeyboard displays the character set with color-coded states
func printKeyboard(guesses []Guess, alphabet []string) {
	if alphabet == nil {
		return
	}

	priority := map[string]int{
		"correct": 3,
		"present": 2,
		"absent":  1,
		"":        0,
	}

	state := make(map[string]string)
	for _, guess := range guesses {
		chars := wordChars(guess.Word)
		for i, ch := range chars {
			if priority[guess.States[i]] > priority[state[ch]] {
				state[ch] = guess.States[i]
			}
		}
	}

	// Determine row width based on character byte length
	rowSize := 10
	allSingleByte := true
	for _, ch := range alphabet {
		if len(ch) > 1 {
			allSingleByte = false
			break
		}
	}
	if !allSingleByte {
		rowSize = 8
	}

	line := "  "
	for i, ch := range alphabet {
		st := state[ch]
		if st != "" {
			line += tile(ch, st)
		} else {
			pad := " "
			if isWide(ch) {
				pad = ""
			}
			line += fmt.Sprintf("%s%s%s%s%s", White, pad, strings.ToUpper(ch), pad, Reset)
		}

		if (i+1)%rowSize == 0 {
			fmt.Println(line)
			line = "  "
		}
	}
	if strings.TrimSpace(line) != "" {
		fmt.Println(line)
	}
	fmt.Println()
}

// evaluate checks a guess against the answer
func evaluate(guessChars, answerChars []string) []string {
	length := len(answerChars)
	states := make([]string, length)
	pool := make([]string, length)
	copy(pool, answerChars)

	// Mark correct positions
	for i := range guessChars {
		if guessChars[i] == answerChars[i] {
			states[i] = "correct"
			pool[i] = ""
		} else {
			states[i] = "absent"
		}
	}

	// Mark present characters
	for i, g := range guessChars {
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

// clear clears the terminal screen
func clear() {
	if runtime.GOOS == "windows" {
		fmt.Print("\033[H\033[2J")
	} else {
		fmt.Print("\033[H\033[2J")
	}
}

// banner displays the game header
func banner(lang string, wlen, maxGuesses int) {
	fmt.Printf("\n%s  W O R D L E  —  %s Edition  (%d chars, %d guesses)%s\n", Bold, lang, wlen, maxGuesses, Reset)
	fmt.Printf("  %s   %s Correct character & position\n", Green, Reset)
	fmt.Printf("  %s   %s Correct character, wrong position\n", Yellow, Reset)
	fmt.Printf("  %s   %s Character not in word\n\n", Gray, Reset)
}

// printDefinition displays the word's definition
func printDefinition(word, definition string) {
	if definition != "" {
		fmt.Printf("  %s%s%s: %s\n\n", Bold, strings.ToUpper(word), Reset, definition)
	} else {
		fmt.Printf("  %s%s%s: (no definition available)\n\n", Bold, strings.ToUpper(word), Reset)
	}
}

// play runs a single game
func play(wordList map[string]string, lang string, wlen, maxGuesses int) {
	// Convert map to slice for random selection
	words := make([]string, 0, len(wordList))
	for word := range wordList {
		words = append(words, word)
	}

	answer := words[rand.Intn(len(words))]
	answerChars := wordChars(answer)
	var guesses []Guess
	alphabet := buildAlphabet(wordList)

	clear()
	banner(lang, wlen, maxGuesses)
	printBoard(guesses, maxGuesses, wlen)
	printKeyboard(guesses, alphabet)

	reader := bufio.NewReader(os.Stdin)

	for attempt := 1; attempt <= maxGuesses; attempt++ {
		var raw string
		var rawChars []string

		for {
			fmt.Printf("  Guess %d/%d: ", attempt, maxGuesses)
			input, _ := reader.ReadString('\n')
			raw = strings.TrimSpace(strings.ToLower(input))
			rawChars = wordChars(raw)

			if len(rawChars) != wlen {
				fmt.Printf("  ✗ Enter a %d-character word.\n", wlen)
				continue
			}

			valid := true
			for _, ch := range raw {
				if !isWordChar(ch) {
					valid = false
					break
				}
			}
			if !valid {
				fmt.Printf("  ✗ Enter a %d-character word.\n", wlen)
				continue
			}

			if _, exists := wordList[raw]; !exists {
				fmt.Print("  Not in word list — play anyway? [y/N] ")
				yn, _ := reader.ReadString('\n')
				yn = strings.TrimSpace(strings.ToLower(yn))
				if yn != "y" {
					continue
				}
			}
			break
		}

		states := evaluate(rawChars, answerChars)
		guesses = append(guesses, Guess{Word: raw, States: states})

		clear()
		printBoard(guesses, maxGuesses, wlen)
		printKeyboard(guesses, alphabet)

		if raw == answer {
			msgs := []string{"Genius!", "Magnificent!", "Impressive!", "Splendid!", "Great!", "Phew!", "Lucky!", "Barely made it!"}
			idx := attempt - 1
			if idx >= len(msgs) {
				idx = len(msgs) - 1
			}
			msg := msgs[idx]
			fmt.Printf("  %s%s%s  (%d/%d)\n\n", Bold, msg, Reset, attempt, maxGuesses)
			printDefinition(answer, wordList[answer])
			return
		}
	}

	fmt.Printf("  The word was %s%s%s\n\n", Bold, strings.ToUpper(answer), Reset)
	printDefinition(answer, wordList[answer])
}

// getLanguages gets (and displays) all known languages
func getLanguages(printing bool) map[string]string {
	if printing {
		fmt.Println("\n  List of language names (pass as-is to --lang):")
	}

	// Get page
	resp, err := http.Get("https://kaikki.org/dictionary/index.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching URL: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("Status code error: %d %s", resp.StatusCode, resp.Status)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Map to store (langugage_name, language_url)
	languages := make(map[string]string)

	// Find all links (language names appear as links)
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")

		// Filter out non-language links (like navigation, etc.)
		if exists && strings.HasSuffix(href, "/index.html") {
			trimmed := strings.TrimSuffix(href, "/index.html")

			// Decode URL-encoded string
			decoded, err := url.QueryUnescape(trimmed)
			if err != nil {
				log.Fatal(err)
			}

			if !strings.Contains(decoded, ".") {
				// Key: language name (text), Value: URL path (decoded)
				languages[decoded] = href
			}
		}
	})

	if printing {
		// Get sorted keys
		keys := make([]string, 0, len(languages))
		for name := range languages {
			keys = append(keys, name)
		}
		sort.Strings(keys)

		// Print in columns using sorted keys
		for i, name := range keys {
			fmt.Printf("%-20s", name)
			if (i+1)%3 == 0 {
				fmt.Println()
			}
		}
		if len(keys)%3 != 0 {
			fmt.Println()
		}
	}

	return languages
}

func main() {
	var (
		lang       string
		length     int
		guesses    int
		listLangs  bool
		clearCache bool
	)

	flag.StringVar(&lang, "lang", DefaultLang, "Language to use (e.g., 'French', 'Russian', 'Arabic')")
	flag.StringVar(&lang, "l", DefaultLang, "Language to use (shorthand)")
	flag.IntVar(&length, "length", DefaultLength, "Characters per word")
	flag.IntVar(&length, "n", DefaultLength, "Characters per word (shorthand)")
	flag.IntVar(&guesses, "guesses", DefaultGuesses, "Maximum guesses allowed")
	flag.IntVar(&guesses, "g", DefaultGuesses, "Maximum guesses allowed (shorthand)")
	flag.BoolVar(&listLangs, "list-langs", false, "Print known language names and exit")
	flag.BoolVar(&clearCache, "clear-cache", false, "Delete cached word list and re-download")

	flag.Parse()

	if listLangs {
		getLanguages(true)
		return
	}

	// Resolve language name
	languages := getLanguages(false)
	langKey := strings.ToLower(lang)
	if known, ok := languages[langKey]; ok {
		lang = known
	} else {
		caser := cases.Title(language.English)
		lang = caser.String(lang)
	}

	if length < 2 || length > 20 {
		fmt.Println("  --length must be between 2 and 20.")
		os.Exit(1)
	}
	if guesses < 1 || guesses > 30 {
		fmt.Println("  --guesses must be between 1 and 30.")
		os.Exit(1)
	}

	if clearCache {
		cf := cachePath(lang, length)
		if _, err := os.Stat(cf); err == nil {
			os.Remove(cf)
			fmt.Printf("  Cache cleared: %s\n", cf)
		} else {
			fmt.Printf("  No cache found for %s / %d-character words.\n", lang, length)
		}
	}

	fmt.Printf("\n  Loading %s %d-character word list...\n", lang, length)
	words, err := loadWordList(lang, length)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  %d words ready.\n\n", len(words))

	reader := bufio.NewReader(os.Stdin)
	for {
		play(words, lang, length, guesses)
		fmt.Print("  Play again? [Y/n] ")
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(strings.ToLower(yn))
		if yn == "n" {
			fmt.Println("\n  Thanks for playing!")
			break
		}
	}
}

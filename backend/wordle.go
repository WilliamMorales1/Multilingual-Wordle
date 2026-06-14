package main

// Word game evaluation and normalization logic.

import (
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// wordChars splits a word into grapheme clusters (base letter + combining marks).
func wordChars(word string) []string {
	var chars []string
	normalized := norm.NFC.String(word)
	runes := []rune(normalized)
	i := 0
	for i < len(runes) {
		var char strings.Builder
		char.WriteString(string(runes[i]))
		i++
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			char.WriteString(string(runes[i]))
			i++
		}
		chars = append(chars, char.String())
	}
	return chars
}

func wordLen(word string) int { return len(wordChars(word)) }

func isWordChar(r rune) bool {
	return unicode.In(r, unicode.Ll, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Lm,
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

// normalizeChar strips diacritical marks and lowercases a grapheme cluster.
// Enables accent-insensitive comparison: é→e, ñ→n, ü→u.
func normalizeChar(ch string) string {
	nfd := norm.NFD.String(ch)
	var buf strings.Builder
	for _, r := range nfd {
		if !unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
			buf.WriteRune(r)
		}
	}
	return strings.ToLower(buf.String())
}

func normalizeWord(word string) string {
	var b strings.Builder
	for _, ch := range wordChars(word) {
		b.WriteString(normalizeChar(ch))
	}
	return b.String()
}

// buildNormalizedSet maps accent-stripped forms back to one canonical original word.
func buildNormalizedSet(words map[string]string) map[string]string {
	set := make(map[string]string, len(words))
	for w := range words {
		set[normalizeWord(w)] = w
	}
	return set
}

// buildAlphabet collects unique grapheme clusters from the word list.
// Returns nil for logographic scripts exceeding logographicThreshold.
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

// matchWildcard finds the first canonical word whose grapheme clusters match
// guessChars, treating "*" as matching any char whose base is in overflowBaseSet.
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

// evaluate returns per-character states ("correct"/"present"/"absent") for a guess.
// Comparison is accent-insensitive.
func evaluate(guessChars, answerChars []string) []string {
	length := len(answerChars)
	states := make([]string, length)
	normGuess := make([]string, length)
	normAnswer := make([]string, length)

	var wg sync.WaitGroup
	for i := range length {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			normGuess[i] = normalizeChar(guessChars[i])
			normAnswer[i] = normalizeChar(answerChars[i])
			if normGuess[i] == normAnswer[i] {
				states[i] = "correct"
			} else {
				states[i] = "absent"
			}
		}(i)
	}
	wg.Wait()

	pool := make([]string, length)
	for i := range length {
		if states[i] != "correct" {
			pool[i] = normAnswer[i]
		}
	}
	for i, g := range normGuess {
		if states[i] == "correct" {
			continue
		}
		for j, p := range pool {
			if p == g {
				states[i] = "present"
				pool[j] = ""
				break
			}
		}
	}
	return states
}

package main

// Word game evaluation and normalization logic.

import (
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Hangul syllable decomposition tables (compatibility jamo codepoints)
var hangulChoseong = []rune{'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}
var hangulJungseong = []rune{'ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'}
var hangulJongseong = []rune{0, 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}

// decomposeHangul converts precomposed syllable blocks (AC00–D7A3) into
// compatibility jamo sequences. Other runes pass through unchanged.
func decomposeHangul(word string) string {
	var buf strings.Builder
	for _, r := range word {
		if r >= 0xAC00 && r <= 0xD7A3 {
			idx := int(r - 0xAC00)
			jong := idx % 28
			jung := (idx / 28) % 21
			cho := idx / 28 / 21
			buf.WriteRune(hangulChoseong[cho])
			buf.WriteRune(hangulJungseong[jung])
			if jong != 0 {
				buf.WriteRune(hangulJongseong[jong])
			}
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func isHangulLang(lang string) bool {
	return slices.Contains([]string{"korean", "middle korean", "jeju"}, strings.ToLower(lang))
}
func isJapaneseLang(lang string) bool {
	return slices.Contains([]string{"japanese", "ainu"}, strings.ToLower(lang))
}

// katakanaToHiragana converts fullwidth katakana (U+30A1–U+30F6) to hiragana.
func katakanaToHiragana(word string) string {
	var buf strings.Builder
	for _, r := range word {
		if r >= 0x30A1 && r <= 0x30F6 {
			buf.WriteRune(r - 0x60)
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// isPureHiragana returns true when every rune is in the hiragana block U+3041–U+309F.
func isPureHiragana(word string) bool {
	for _, r := range word {
		if r < 0x3041 || r > 0x309F {
			return false
		}
	}
	return len(word) > 0
}

// jamoExpansion maps compound compatibility jamo to their two base components.
// Covers doubled consonants, consonant clusters (jongseong), and compound vowels.
var jamoExpansion = map[rune]string{
	// Doubled consonants
	'ㄲ': "ㄱㄱ", 'ㄸ': "ㄷㄷ", 'ㅃ': "ㅂㅂ", 'ㅆ': "ㅅㅅ", 'ㅉ': "ㅈㅈ",
	// Consonant clusters
	'ㄳ': "ㄱㅅ", 'ㄵ': "ㄴㅈ", 'ㄶ': "ㄴㅎ",
	'ㄺ': "ㄹㄱ", 'ㄻ': "ㄹㅁ", 'ㄼ': "ㄹㅂ", 'ㄽ': "ㄹㅅ", 'ㄾ': "ㄹㅌ", 'ㄿ': "ㄹㅍ", 'ㅀ': "ㄹㅎ",
	'ㅄ': "ㅂㅅ",
	// Compound vowels (diphthongs)
	'ㅘ': "ㅗㅏ", 'ㅙ': "ㅗㅐ", 'ㅚ': "ㅗㅣ",
	'ㅝ': "ㅜㅓ", 'ㅞ': "ㅜㅔ", 'ㅟ': "ㅜㅣ",
	'ㅢ': "ㅡㅣ",
}

// expandJamo splits compound jamo into base components (second decomposition pass).
func expandJamo(word string) string {
	var buf strings.Builder
	for _, r := range word {
		if exp, ok := jamoExpansion[r]; ok {
			buf.WriteString(exp)
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// isPureJamo returns true when every rune is a Korean compatibility jamo (U+3131–U+3163).
func isPureJamo(word string) bool {
	for _, r := range word {
		if r < 0x3131 || r > 0x3163 {
			return false
		}
	}
	return len(word) > 0
}

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

// normalizeKanaRune maps small kana to their full-size equivalents.
// Dakuten/handakuten are handled by NFD stripping in normalizeChar.
func normalizeKanaRune(r rune) rune {
	switch r {
	case 'ぁ':
		return 'あ'
	case 'ぃ':
		return 'い'
	case 'ぅ':
		return 'う'
	case 'ぇ':
		return 'え'
	case 'ぉ':
		return 'お'
	case 'っ':
		return 'つ'
	case 'ゃ':
		return 'や'
	case 'ゅ':
		return 'ゆ'
	case 'ょ':
		return 'よ'
	case 'ゎ':
		return 'わ'
	case 'ゕ':
		return 'か'
	case 'ゖ':
		return 'け'
	}
	return r
}

// normalizeChar strips diacritical marks and lowercases a grapheme cluster.
// Enables accent-insensitive comparison: é→e, ñ→n, ü→u.
// For kana: small variants collapse to large (っ→つ), and dakuten/handakuten
// are stripped via NFD so voiced forms match their base (が→か, ぱ→は).
func normalizeChar(ch string) string {
	var pre strings.Builder
	for _, r := range ch {
		pre.WriteRune(normalizeKanaRune(r))
	}
	nfd := norm.NFD.String(pre.String())
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

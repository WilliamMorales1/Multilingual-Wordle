package lang

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var hangulChoseong = []rune{'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}
var hangulJungseong = []rune{'ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'}
var hangulJongseong = []rune{0, 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}

func DecomposeHangul(word string) string {
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

func IsHangulLang(lang string) bool {
	return slices.Contains([]string{"korean", "middle korean", "jeju"}, strings.ToLower(lang))
}
func IsJapaneseLang(lang string) bool {
	return slices.Contains([]string{"japanese", "ainu"}, strings.ToLower(lang))
}

func KatakanaToHiragana(word string) string {
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

func IsPureHiragana(word string) bool {
	for _, r := range word {
		if r < 0x3041 || r > 0x309F {
			return false
		}
	}
	return len(word) > 0
}

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

func ExpandJamo(word string) string {
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

func IsPureJamo(word string) bool {
	for _, r := range word {
		if r < 0x3131 || r > 0x3163 {
			return false
		}
	}
	return len(word) > 0
}

var toneTranslationsByKind = map[string]map[rune]string{
	"vietnamese": {
		0x0300: "`",  // huyền (grave)        -> GRAVE ACCENT
		0x0301: "´",  // sắc (acute)          -> ACUTE ACCENT
		0x0303: "~",  // ngã (tilde)          -> TILDE
		0x0309: "ˀ",  // hỏi (hook above)     -> MODIFIER LETTER GLOTTAL STOP
		0x0323: ".",  // nặng (dot below)     -> FULL STOP
	},
}

// ToneSplitKind reports which combining-mark tone-splitting scheme a language
// uses ("vietnamese" or "" if none), based on the requested language name.
func ToneSplitKind(lng string) string {
	if strings.EqualFold(lng, "Vietnamese") {
		return "vietnamese"
	}
	return ""
}

// wordCharsToneSplit splits a word like WordChars, but pulls each tone mark
// into its own tile (translated via toneTranslationsByKind) instead of
// merging it into the preceding base letter.
func wordCharsToneSplit(word, kind string) []string {
	toneMarks := toneTranslationsByKind[kind]
	var chars []string
	normalized := norm.NFD.String(word)
	runes := []rune(normalized)
	i := 0
	for i < len(runes) {
		var char strings.Builder
		char.WriteRune(runes[i])
		i++
		var tone string
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			r := runes[i]
			if tone == "" {
				if t, ok := toneMarks[r]; ok {
					tone = t
					i++
					continue
				}
			}
			char.WriteRune(r)
			i++
		}
		chars = append(chars, norm.NFC.String(char.String()))
		if tone != "" {
			chars = append(chars, tone)
		}
	}
	return chars
}

// WordChars splits a word into grapheme clusters (base letter + combining marks).
// For tone-split languages (toneLang from ToneSplitKind), tone marks are split
// into their own tile instead of merging into the base letter.
func WordChars(word, toneLang string) []string {
	if toneLang != "" {
		return wordCharsToneSplit(word, toneLang)
	}
	var chars []string
	normalized := norm.NFC.String(word)
	runes := []rune(normalized)
	i := 0
	for i < len(runes) {
		var char strings.Builder
		char.WriteString(string(runes[i]))
		i++
		// Abugida vowel-sign (matra) marks split off into their own tile
		// instead of merging into the consonant's tile, since they're a
		// keyboard-visible vowel choice, not a mere accent.
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			if _, isMatra := matraToVowel[runes[i]]; isMatra {
				break
			}
			char.WriteString(string(runes[i]))
			i++
		}
		chars = append(chars, char.String())
		for i < len(runes) {
			if _, isMatra := matraToVowel[runes[i]]; !isMatra {
				break
			}
			chars = append(chars, string(runes[i]))
			i++
		}
	}
	return chars
}

func wordLen(word, toneLang string) int { return len(WordChars(word, toneLang)) }

func IsWordChar(r rune) bool {
	return unicode.In(r, unicode.Ll, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Lm,
		unicode.Mn, unicode.Mc, unicode.Me)
}

func IsValid(word string, length int, toneLang string) bool {
	if wordLen(word, toneLang) != length {
		return false
	}
	for _, r := range word {
		if !IsWordChar(r) {
			return false
		}
	}
	return true
}

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

// abugidaVowelMatras maps each Brahmic script's independent vowels to their
// dependent matra form, used to split consonant+matra into two tiles
// (WordChars) and map a lone matra back to its vowel (NormalizeChar).
var abugidaVowelMatras = map[string]map[rune]rune{
	"devanagari": {
		'आ': 0x093E, 'इ': 0x093F, 'ई': 0x0940, 'उ': 0x0941, 'ऊ': 0x0942,
		'ऋ': 0x0943, 'ए': 0x0947, 'ऐ': 0x0948, 'ऑ': 0x0949, 'ओ': 0x094B, 'औ': 0x094C,
	},
	"gujarati": {
		'આ': 0x0ABE, 'ઇ': 0x0ABF, 'ઈ': 0x0AC0, 'ઉ': 0x0AC1, 'ઊ': 0x0AC2,
		'ઋ': 0x0AC3, 'એ': 0x0AC7, 'ઐ': 0x0AC8, 'ઑ': 0x0AC9, 'ઓ': 0x0ACB, 'ઔ': 0x0ACC,
	},
	"bengali": {
		'আ': 0x09BE, 'ই': 0x09BF, 'ঈ': 0x09C0, 'উ': 0x09C1, 'ঊ': 0x09C2,
		'ঋ': 0x09C3, 'এ': 0x09C7, 'ঐ': 0x09C8, 'ও': 0x09CB, 'ঔ': 0x09CC,
	},
	"gurmukhi": {
		'ਆ': 0x0A3E, 'ਇ': 0x0A3F, 'ਈ': 0x0A40, 'ਉ': 0x0A41, 'ਊ': 0x0A42,
		'ਏ': 0x0A47, 'ਐ': 0x0A48, 'ਓ': 0x0A4B, 'ਔ': 0x0A4C,
	},
	"tamil": {
		'ஆ': 0x0BBE, 'இ': 0x0BBF, 'ஈ': 0x0BC0, 'உ': 0x0BC1, 'ஊ': 0x0BC2,
		'எ': 0x0BC6, 'ஏ': 0x0BC7, 'ஐ': 0x0BC8, 'ஒ': 0x0BCA, 'ஓ': 0x0BCB, 'ஔ': 0x0BCC,
	},
	"telugu": {
		'ఆ': 0x0C3E, 'ఇ': 0x0C3F, 'ఈ': 0x0C40, 'ఉ': 0x0C41, 'ఊ': 0x0C42,
		'ఋ': 0x0C43, 'ఎ': 0x0C46, 'ఏ': 0x0C47, 'ఐ': 0x0C48, 'ఒ': 0x0C4A, 'ఓ': 0x0C4B, 'ఔ': 0x0C4C,
	},
	"kannada": {
		'ಆ': 0x0CBE, 'ಇ': 0x0CBF, 'ಈ': 0x0CC0, 'ಉ': 0x0CC1, 'ಊ': 0x0CC2,
		'ಋ': 0x0CC3, 'ಎ': 0x0CC6, 'ಏ': 0x0CC7, 'ಐ': 0x0CC8, 'ಒ': 0x0CCA, 'ಓ': 0x0CCB, 'ಔ': 0x0CCC,
	},
}

// matraToVowel is the flattened reverse index (matra rune -> independent
// vowel rune) used by WordChars (to spot a matra worth splitting off) and
// NormalizeChar (to map a lone matra tile back to its base vowel key).
var matraToVowel = func() map[rune]rune {
	m := make(map[rune]rune)
	for _, table := range abugidaVowelMatras {
		for vowel, matra := range table {
			m[matra] = vowel
		}
	}
	return m
}()

// MatraTable returns the independent-vowel -> matra map for an abugida
// keyboard layout name (e.g. "devanagari"), or nil if not one. Exposed so
// the frontend can swap a vowel key to its combining form after a consonant.
func MatraTable(layoutName string) map[string]string {
	table, ok := abugidaVowelMatras[layoutName]
	if !ok {
		return nil
	}
	out := make(map[string]string, len(table))
	for vowel, matra := range table {
		out[string(vowel)] = string(matra)
	}
	return out
}

// NormalizeChar strips diacritical marks and lowercases a grapheme cluster
// for accent-insensitive comparison (é→e, ñ→n). For kana, small variants
// collapse to large and dakuten/handakuten strip via NFD (が→か, ぱ→は).
func NormalizeChar(ch string) string {
	// A lone matra tile normalizes to its independent vowel letter, so it
	// groups with — and is covered by — that vowel's keyboard key.
	if runes := []rune(ch); len(runes) == 1 {
		if vowel, ok := matraToVowel[runes[0]]; ok {
			return string(vowel)
		}
	}
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

func NormalizeWord(word, toneLang string) string {
	var b strings.Builder
	for _, ch := range WordChars(word, toneLang) {
		b.WriteString(NormalizeChar(ch))
	}
	return b.String()
}

func BuildNormalizedSet(words map[string]string, toneLang string) map[string]string {
	set := make(map[string]string, len(words))
	for w := range words {
		set[NormalizeWord(w, toneLang)] = w
	}
	return set
}

func BuildAlphabet(wordList map[string]string, toneLang string) []string {
	charSet := make(map[string]bool)
	for word := range wordList {
		for _, ch := range WordChars(word, toneLang) {
			charSet[ch] = true
		}
	}
	chars := make([]string, 0, len(charSet))
	for ch := range charSet {
		chars = append(chars, ch)
	}
	sort.Strings(chars)
	return chars
}

// MatchWildcard finds the lexicographically smallest canonical word whose
// grapheme clusters match guessChars, treating "*" as matching any char
// whose base is in overflowBaseSet. Picking deterministically (rather than
// the first hit from Go's randomized map iteration) means the same wildcard
// guess always resolves to the same word.
func MatchWildcard(guessChars []string, normSet map[string]string, overflowBaseSet map[string]bool, toneLang string) string {
	n := len(guessChars)
	best := ""
	for _, canonical := range normSet {
		cChars := WordChars(canonical, toneLang)
		if len(cChars) != n {
			continue
		}
		match := true
		for i, gc := range guessChars {
			if gc == "*" {
				if !overflowBaseSet[NormalizeChar(cChars[i])] {
					match = false
					break
				}
				continue
			}
			if NormalizeChar(gc) != NormalizeChar(cChars[i]) {
				match = false
				break
			}
		}
		if match && (best == "" || canonical < best) {
			best = canonical
		}
	}
	return best
}

// Evaluate returns per-character states ("correct"/"present"/"absent") for a guess.
// Comparison is accent-insensitive.
func Evaluate(guessChars, answerChars []string) []string {
	length := len(answerChars)
	states := make([]string, length)
	normGuess := make([]string, length)
	normAnswer := make([]string, length)

	for i := range length {
		normGuess[i] = NormalizeChar(guessChars[i])
		normAnswer[i] = NormalizeChar(answerChars[i])
		if normGuess[i] == normAnswer[i] {
			states[i] = "correct"
		} else {
			states[i] = "absent"
		}
	}

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

// mandarinToneMarks maps Wiktionary pinyin's NFD combining marks (macron/
// acute/caron/grave) to their conventional pinyin tone number (1-4), per
// https://en.wikipedia.org/wiki/Pinyin#Tone_marks.
var mandarinToneMarks = map[rune]string{
	0x0304: "1", // macron  (ā) - tone 1, flat
	0x0301: "2", // acute   (á) - tone 2, rising
	0x030C: "3", // caron   (ǎ) - tone 3, dipping
	0x0300: "4", // grave   (à) - tone 4, falling
}

var superscriptDigits = map[rune]int{
	'⁰': 0, '¹': 1, '²': 2, '³': 3, '⁴': 4,
	'⁵': 5, '⁶': 6, '⁷': 7, '⁸': 8, '⁹': 9,
}

func digitValue(r rune) (int, bool) {
	if r >= '0' && r <= '9' {
		return int(r - '0'), true
	}
	if n, ok := superscriptDigits[r]; ok {
		return n, true
	}
	return 0, false
}

// mandarinToneify folds pinyin's diacritic tone marks into trailing numeral
// tiles (1-4, conventional pinyin tone-number notation) char by char —
// Wiktionary's readings are often concatenated without syllable separators,
// so unlike other dialects this can't split on hyphens.
func mandarinToneify(rom string) string {
	normalized := norm.NFD.String(rom)
	runes := []rune(normalized)
	var out strings.Builder
	i := 0
	for i < len(runes) {
		var base strings.Builder
		base.WriteRune(runes[i])
		i++
		var tone string
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			r := runes[i]
			if tone == "" {
				if t, ok := mandarinToneMarks[r]; ok {
					tone = t
					i++
					continue
				}
			}
			base.WriteRune(r)
			i++
		}
		out.WriteString(norm.NFC.String(base.String()))
		if tone != "" {
			out.WriteString(tone)
		}
	}
	return out.String()
}

// ChineseToneify converts a dialect's raw romanization into a guessable word
// where each syllable's tone number is kept as its own trailing numeral
// tile, using that dialect's own conventional tone-number notation (e.g.
// Jyutping 1-6, POJ/Tâi-lô 1-8) rather than collapsing distinct tones into a
// shared category.
//
// Syllables are split on tone-number digit runs (which terminate a syllable
// and become its tile) and on any other non-word rune (space/hyphen/comma/
// parenthesis/etc., which separates syllables without itself carrying a
// tone) — this also recovers tones for entries where Wiktionary fuses
// adjacent syllables with no separator at all, e.g. "aa1het6" (two
// syllables, no space between them).
func ChineseToneify(dialect, rom string) string {
	if dialect == "Mandarin" {
		return mandarinToneify(rom)
	}

	var out, letters strings.Builder
	flush := func(digit int) {
		clean := letters.String()
		letters.Reset()
		if clean == "" {
			return
		}
		out.WriteString(clean)
		if digit > 0 {
			out.WriteString(strconv.Itoa(digit))
		}
	}

	runes := []rune(rom)
	for i := 0; i < len(runes); {
		if r := runes[i]; IsWordChar(r) {
			letters.WriteRune(r)
			i++
			continue
		}
		if _, ok := digitValue(runes[i]); ok {
			n := 0
			j := i
			for j < len(runes) {
				d, ok := digitValue(runes[j])
				if !ok {
					break
				}
				n = n*10 + d
				j++
			}
			flush(n)
			i = j
			continue
		}
		flush(0)
		i++
	}
	flush(0)
	return out.String()
}

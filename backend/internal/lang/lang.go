// Package lang implements script-aware word normalization: grapheme splitting,
// accent-insensitive comparison, Hangul/Japanese handling, and Wordle-style guess
// evaluation.
package lang

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// logographicThreshold caps alphabet size before BuildAlphabet gives up (CJK etc).
const logographicThreshold = 200

// Hangul syllable decomposition tables (compatibility jamo codepoints)
var hangulChoseong = []rune{'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}
var hangulJungseong = []rune{'ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'}
var hangulJongseong = []rune{0, 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}

// DecomposeHangul converts precomposed syllable blocks (AC00–D7A3) into
// compatibility jamo sequences. Other runes pass through unchanged.
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

// KatakanaToHiragana converts fullwidth katakana (U+30A1–U+30F6) to hiragana.
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

// IsPureHiragana returns true when every rune is in the hiragana block U+3041–U+309F.
func IsPureHiragana(word string) bool {
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

// ExpandJamo splits compound jamo into base components (second decomposition pass).
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

// IsPureJamo returns true when every rune is a Korean compatibility jamo (U+3131–U+3163).
func IsPureJamo(word string) bool {
	for _, r := range word {
		if r < 0x3131 || r > 0x3163 {
			return false
		}
	}
	return len(word) > 0
}

// toneTranslationsByKind maps the combining tone marks of a tone-split
// language to a non-combining (spacing) glyph used as that tone's tile —
// in source text, on the keyboard, and in the stored answer alike, so a
// lone tone tile always renders as a real visible glyph instead of a
// combining mark with nothing to attach to. Vietnamese tone marks ride
// alongside vowel-quality diacritics (â, ơ, ư, ...) which must stay attached
// to their base letter, so only the five actual tone marks split off.
//
// Chinese isn't handled here: its tone is folded into a literal trailing
// hanzi tile (see ChineseToneify) at word-list ingestion time, since the
// four traditional tone categories need digit/diacritic *and* checked-coda
// inspection per dialect that doesn't fit the generic combining-mark model.
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

// wordCharsToneSplit splits a word into grapheme clusters like WordChars, but
// pulls each tone mark out into its own tile (translated to its non-combining
// glyph via toneTranslationsByKind) instead of merging it into the preceding
// base letter.
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
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			char.WriteString(string(runes[i]))
			i++
		}
		chars = append(chars, char.String())
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

// normalizeKanaRune maps small kana to their full-size equivalents.
// Dakuten/handakuten are handled by NFD stripping in NormalizeChar.
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

// normalizeSyllabicsRune maps Canadian Aboriginal Syllabics consonant+vowel
// glyphs to the canonical "a"-vowel form of the same consonant, so the
// keyboard shows one key per consonant regardless of vowel.
func normalizeSyllabicsRune(r rune) rune {
	switch r {
	case 'ᐱ', 'ᐯ', 'ᐳ':
		return 'ᐸ'
	case 'ᑎ', 'ᑐ':
		return 'ᑕ'
	case 'ᑭ', 'ᑯ':
		return 'ᑲ'
	case 'ᒋ', 'ᒍ':
		return 'ᒐ'
	case 'ᒥ', 'ᒧ':
		return 'ᒪ'
	case 'ᓂ', 'ᓄ':
		return 'ᓇ'
	case 'ᓭ', 'ᓯ', 'ᓱ':
		return 'ᓴ'
	case 'ᔨ', 'ᔪ':
		return 'ᔭ'
	case 'ᕆ', 'ᕈ':
		return 'ᕒ'
	case 'ᓕ', 'ᓗ':
		return 'ᓚ'
	}
	return r
}

// NormalizeChar strips diacritical marks and lowercases a grapheme cluster.
// Enables accent-insensitive comparison: é→e, ñ→n, ü→u.
// For kana: small variants collapse to large (っ→つ), and dakuten/handakuten
// are stripped via NFD so voiced forms match their base (が→か, ぱ→は).
// For syllabics: vowel variants of the same consonant collapse to the "a" form.
func NormalizeChar(ch string) string {
	var pre strings.Builder
	for _, r := range ch {
		pre.WriteRune(normalizeSyllabicsRune(normalizeKanaRune(r)))
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

// BuildNormalizedSet maps accent-stripped forms back to one canonical original word.
func BuildNormalizedSet(words map[string]string, toneLang string) map[string]string {
	set := make(map[string]string, len(words))
	for w := range words {
		set[NormalizeWord(w, toneLang)] = w
	}
	return set
}

// BuildAlphabet collects unique grapheme clusters from the word list.
// Returns nil for logographic scripts exceeding logographicThreshold.
func BuildAlphabet(wordList map[string]string, toneLang string) []string {
	charSet := make(map[string]bool)
	for word := range wordList {
		for _, ch := range WordChars(word, toneLang) {
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

// MatchWildcard finds the first canonical word whose grapheme clusters match
// guessChars, treating "*" as matching any char whose base is in overflowBaseSet.
func MatchWildcard(guessChars []string, normSet map[string]string, overflowBaseSet map[string]bool, toneLang string) string {
	n := len(guessChars)
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
		if match {
			return canonical
		}
	}
	return ""
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

// The four traditional Middle Chinese tone categories — see
// https://en.wikipedia.org/wiki/Four_tones_(Middle_Chinese). Every Chinese
// dialect's romanized words get one of these folded into the word as its own
// trailing tile per syllable, instead of the dialect's native tone notation.
const (
	TonePing  = "平" // level
	ToneShang = "上" // rising
	ToneQu    = "去" // departing
	ToneRu    = "入" // entering (checked, historically ended in -p/-t/-k)
)

// mandarinToneMarks maps the NFD combining marks Wiktionary's Mandarin pinyin
// uses (macron/acute/caron/grave) to a tone category. Pinyin tones 1 (yīn
// píng) and 2 (yáng píng) are both reflexes of Middle Chinese 平; tone 3 is
// 上; tone 4 covers both the old 去 syllables and the (lost in Mandarin)
// 入 syllables that merged into it, so it's labeled 去.
var mandarinToneMarks = map[rune]string{
	0x0304: TonePing,
	0x0301: TonePing,
	0x030C: ToneShang,
	0x0300: ToneQu,
}

// chineseDialectToneDigits maps each dialect's modern tone-number scheme to a
// tone category for *non-checked* syllables. This is an approximation: true
// Middle Chinese tone class depends on the syllable's original initial
// voicing (e.g. 浊上归去 — voiced-initial 上 syllables became 去ed in
// Mandarin), which modern romanizations don't carry, and entering-tone
// syllables are detected structurally instead (see isCheckedCoda) since most
// of these dialects reuse non-entering tone numbers for entering syllables.
// Numbering follows each dialect's conventional yin/yang-ping/shang/qu(/ru)
// ordering.
var chineseDialectToneDigits = map[string]map[int]string{
	"Mandarin":  {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu},
	"Cantonese": {1: TonePing, 2: ToneShang, 3: ToneQu, 4: TonePing, 5: ToneShang, 6: ToneQu},
	"Hokkien":   {1: TonePing, 2: ToneShang, 3: ToneQu, 4: ToneRu, 5: TonePing, 6: ToneShang, 7: ToneQu, 8: ToneRu},
	"Teochew":   {1: TonePing, 2: ToneShang, 3: ToneQu, 4: ToneRu, 5: TonePing, 6: ToneShang, 7: ToneQu, 8: ToneRu},
	"Hakka":     {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu, 5: ToneRu, 6: ToneQu, 7: ToneRu},
	"Wu":        {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu, 5: ToneRu},
	"Min Bei":   {1: TonePing, 2: ToneShang, 3: ToneQu, 4: ToneRu, 5: TonePing, 6: ToneShang, 7: ToneQu, 8: ToneRu},
	"Min Dong":  {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu, 5: ToneQu, 6: ToneQu, 7: ToneRu},
	"Gan":       {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu, 5: ToneQu, 6: ToneRu, 7: ToneRu},
	"Xiang":     {1: TonePing, 2: ToneShang, 3: ToneQu, 4: TonePing, 5: ToneShang, 6: ToneQu},
	"Jin":       {1: TonePing, 2: TonePing, 3: ToneShang, 4: ToneQu, 5: ToneRu},
}

// checkedCodaSuffixes are stop-consonant/glottal codas that mark a syllable
// as historically checked (入), overriding the dialect's tone-digit lookup —
// these dialects (unlike Mandarin) reuse some tone numbers across both
// checked and unchecked syllables, distinguished structurally by the coda.
var checkedCodaSuffixes = []string{"p", "t", "k", "h"}

func isCheckedCoda(letters string) bool {
	for _, suf := range checkedCodaSuffixes {
		if strings.HasSuffix(letters, suf) {
			return true
		}
	}
	return false
}

func filterLetterMarks(s string) string {
	var b strings.Builder
	for _, r := range s {
		if IsWordChar(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitTrailingDigits peels a trailing run of ASCII digits (a romanization's
// tone number) off a syllable token.
func splitTrailingDigits(s string) (string, int) {
	end := len(s)
	for end > 0 && s[end-1] >= '0' && s[end-1] <= '9' {
		end--
	}
	if end == len(s) {
		return s, 0
	}
	n, err := strconv.Atoi(s[end:])
	if err != nil {
		return s, 0
	}
	return s[:end], n
}

// mandarinToneify folds Mandarin pinyin's diacritic tone marks into trailing
// 平/上/去 tiles, character by character — Wiktionary's Mandarin pinyin
// readings are often concatenated without syllable separators, so unlike
// the other dialects this can't split on whitespace/hyphens first.
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

// ChineseToneify converts a Chinese dialect's raw romanization into a
// guessable word where each syllable's tone is folded down into one of the
// four traditional tone-category hanzi (TonePing/ToneShang/ToneQu/ToneRu),
// appended as its own tile — see the four-tones doc comment above.
func ChineseToneify(dialect, rom string) string {
	if dialect == "Mandarin" {
		return mandarinToneify(rom)
	}

	digitMap := chineseDialectToneDigits[dialect]
	var out strings.Builder
	for _, token := range strings.FieldsFunc(rom, func(r rune) bool { return r == ' ' || r == '-' }) {
		letters, digit := splitTrailingDigits(token)
		clean := filterLetterMarks(letters)
		if clean == "" {
			continue
		}
		var tone string
		if isCheckedCoda(clean) {
			tone = ToneRu
		} else if digit > 0 {
			tone = digitMap[digit]
		}
		out.WriteString(clean)
		out.WriteString(tone)
	}
	return out.String()
}

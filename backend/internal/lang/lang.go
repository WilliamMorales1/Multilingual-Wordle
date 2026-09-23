package lang

import (
	"maps"
	"slices"
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

// ChoonMark is the prolonged-sound mark ー (U+30FC). It is shared by both
// kana scripts, so KatakanaToHiragana leaves it alone — ラーメン converts to
// らーめん, not to a katakana leftover — and it is a key of its own on the
// kana keyboard.
const ChoonMark = 0x30FC

// IsPureHiragana reports whether a word is written entirely in hiragana
// proper: U+3041ぁ through U+3096ゖ, plus the iteration marks ゝ/ゞ and the
// prolonged-sound mark ー. The rest of the block (U+3099-U+309C combining/
// standalone dakuten, U+309D-U+309F's katakana-only and vertical variants)
// is deliberately excluded — a bare combining mark isn't a tile anyone can
// type on the kana keyboard.
//
// ー has to be accepted: it is not in the hiragana block, but every long
// vowel written in kana uses it, so excluding it threw away every loanword
// (らーめん, こーひー, けーき) after KatakanaToHiragana had just produced it,
// while the keyboard still offered a ー key that could never spell anything.
func IsPureHiragana(word string) bool {
	for _, r := range word {
		switch {
		case r >= 0x3041 && r <= 0x3096: // ぁ..ゖ
		case r == 0x309D || r == 0x309E: // ゝ ゞ
		case r == ChoonMark: // ー
		default:
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

// toneTranslationsByKind maps a language's combining tone marks to the
// standalone tile (and keyboard key) that stands for them. Tones are typed as
// their own keystroke in these languages' IMEs, so they're guessed as their
// own tile rather than folded into the base letter.
var toneTranslationsByKind = map[string]map[rune]string{
	"vietnamese": {
		0x0300: "`", // huyền (grave)        -> GRAVE ACCENT
		0x0301: "´", // sắc (acute)          -> ACUTE ACCENT
		0x0303: "~", // ngã (tilde)          -> TILDE
		0x0309: "ˀ", // hỏi (hook above)     -> MODIFIER LETTER GLOTTAL STOP
		0x0323: ".", // nặng (dot below)     -> FULL STOP
	},
}

// ToneSplitKind reports which combining-mark tone-splitting scheme a language
// uses ("vietnamese" or "" if none), based on the requested language name.
// Chinese romanizations are toneless (see ChineseRomanize) and zhuyin already
// carries its tones as standalone marks, so neither needs splitting.
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
		// Every tone mark on this cluster splits off, not just the first:
		// a syllable carrying two of them is two extra keystrokes in the
		// IME, so it has to be two extra tiles.
		var tones []string
		for i < len(runes) && unicode.In(runes[i], unicode.Mn, unicode.Mc, unicode.Me) {
			r := runes[i]
			if t, ok := toneMarks[r]; ok {
				tones = append(tones, t)
				i++
				continue
			}
			char.WriteRune(r)
			i++
		}
		chars = append(chars, norm.NFC.String(char.String()))
		chars = append(chars, tones...)
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

// WordLen counts a word's guessable tiles (not its runes or bytes).
func WordLen(word, toneLang string) int { return len(WordChars(word, toneLang)) }

// zhuyinToneMarks are the standalone bopomofo tone marks typed as their own
// key in a zhuyin IME. They're kept as tiles, so IsWordChar accepts them even
// though Unicode files ˙ (U+02D9) as a symbol rather than a letter.
var zhuyinToneMarks = map[rune]bool{
	'ˊ': true, // 2nd tone
	'ˇ': true, // 3rd tone
	'ˋ': true, // 4th tone
	'˙': true, // neutral tone
}

func IsWordChar(r rune) bool {
	return zhuyinToneMarks[r] || unicode.In(r, unicode.Ll, unicode.Lu, unicode.Lt, unicode.Lo, unicode.Lm,
		unicode.Mn, unicode.Mc, unicode.Me)
}

// toneTileRunes is every rune a tone-split language's standalone tone tile is
// made of (see toneTranslationsByKind). Several of them — "`", "~", ".", "´" —
// are punctuation or symbols to Unicode, so IsWordChar rejects them, but a
// client types them as their own key and submits them as part of the guess.
var toneTileRunes = func() map[rune]bool {
	m := make(map[rune]bool)
	for _, table := range toneTranslationsByKind {
		for _, tile := range table {
			for _, r := range tile {
				m[r] = true
			}
		}
	}
	return m
}()

// IsGuessChar reports whether a rune may appear in a submitted guess. It is
// IsWordChar plus the tone tiles: a Vietnamese guess arrives as the tiles the
// on-screen keyboard produced ("ă´n"), not as the composed word ("ắn"), so
// validating it with IsWordChar alone rejects every toned guess.
func IsGuessChar(r rune) bool { return IsWordChar(r) || toneTileRunes[r] }

// IsWord reports whether a word is made only of word chars — no digits,
// spaces or punctuation, so it's typeable on the game's keyboard.
func IsWord(word string) bool {
	for _, r := range word {
		if !IsWordChar(r) {
			return false
		}
	}
	return true
}

func IsValid(word string, length int, toneLang string) bool {
	return WordLen(word, toneLang) == length && IsWord(word)
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

// BuildNormalizedSet maps each word's accent-insensitive form back to a
// canonical spelling. Several words can share one normalized form ("cafe" and
// "café"); the lexicographically smallest wins, so the same word list always
// resolves an accent-stripped guess to the same word — picking whichever word
// Go's randomized map iteration reached last would differ per process.
func BuildNormalizedSet(words map[string]string, toneLang string) map[string]string {
	set := make(map[string]string, len(words))
	for w := range words {
		key := NormalizeWord(w, toneLang)
		if prev, ok := set[key]; ok && prev <= w {
			continue
		}
		set[key] = w
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
	return slices.Sorted(maps.Keys(charSet))
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
//
// A guess shorter than the answer is padded with "absent" rather than
// indexed past its end: callers validate the tile count first, but a panic
// deep in evaluation is a poor way to find out one of them forgot to.
func Evaluate(guessChars, answerChars []string) []string {
	length := len(answerChars)
	if len(guessChars) < length {
		guessChars = append(slices.Clone(guessChars), make([]string, length-len(guessChars))...)
	}
	states := make([]string, length)
	normGuess := make([]string, length)
	normAnswer := make([]string, length)

	for i := range length {
		normGuess[i] = NormalizeChar(guessChars[i])
		normAnswer[i] = NormalizeChar(answerChars[i])
		if normGuess[i] != "" && normGuess[i] == normAnswer[i] {
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
		// "" is a tile the guess never supplied (a short guess, padded
		// above); it must not claim one of the answer's letters.
		if states[i] == "correct" || g == "" {
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

// mandarinToneMarks are pinyin's NFD combining tone marks (macron/acute/
// caron/grave). Tones aren't part of the guess, so these are stripped from a
// reading, leaving only the letters typed on the keyboard. The diaeresis of
// "ü" is deliberately absent — it's a letter choice, not a tone.
var mandarinToneMarks = map[rune]bool{
	0x0304: true, // macron (ā)
	0x0301: true, // acute  (á)
	0x030C: true, // caron  (ǎ)
	0x0300: true, // grave  (à)
}

func stripMandarinToneMarks(rom string) string {
	var out strings.Builder
	for _, r := range norm.NFD.String(rom) {
		if !mandarinToneMarks[r] {
			out.WriteRune(r)
		}
	}
	return norm.NFC.String(out.String())
}

// ChineseRomanize converts a dialect's raw romanization into a guessable word
// made only of the letters the keyboard types: tone notation is dropped
// (Mandarin's pinyin diacritics, the other dialects' trailing tone numerals),
// as are the space/hyphen/punctuation syllable separators Wiktionary uses.
func ChineseRomanize(dialect, rom string) string {
	if dialect == "Mandarin" {
		rom = stripMandarinToneMarks(rom)
	}
	var out strings.Builder
	for _, r := range rom {
		if IsWordChar(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

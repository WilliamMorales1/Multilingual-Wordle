package main

// Keyboard layout detection and per-language key arrangement.

import "sort"

const logographicThreshold = 200

var keyboardLayouts = map[string][][]string{
	"qwerty": {{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l"}, {"z", "x", "c", "v", "b", "n", "m"}},
	"azerty": {{"a", "z", "e", "r", "t", "y", "u", "i", "o", "p"}, {"q", "s", "d", "f", "g", "h", "j", "k", "l", "m"}, {"w", "x", "c", "v", "b", "n"}},
	"qwertz": {{"q", "w", "e", "r", "t", "z", "u", "i", "o", "p"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l"}, {"y", "x", "c", "v", "b", "n", "m"}},
	"nordic": {
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "å"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l", "ø", "æ"},
		{"z", "x", "c", "v", "b", "n", "m"},
	},
	"turkish": {
		{"q", "w", "e", "r", "t", "y", "u", "ı", "o", "p", "ğ", "ü"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l", "ş", "i"},
		{"z", "x", "c", "v", "b", "n", "m", "ö", "ç"},
	},
	"jcuken": {
		{"й", "ц", "у", "к", "е", "н", "г", "ш", "щ", "з", "х"},
		{"ф", "ы", "в", "а", "п", "р", "о", "л", "д", "ж", "э"},
		{"я", "ч", "с", "м", "и", "т", "ь", "б", "ю"},
	},
	"greek": {
		{"ε", "ρ", "τ", "υ", "θ", "ι", "ο", "π"},
		{"α", "σ", "δ", "φ", "γ", "η", "ξ", "κ", "λ"},
		{"ζ", "χ", "ψ", "ω", "β", "ν", "μ"},
	},
	"arabic": {
		{"ض", "ص", "ث", "ق", "ف", "غ", "ع", "ه", "خ", "ح", "ج", "د"},
		{"ش", "س", "ي", "ب", "ل", "ا", "ت", "ن", "م", "ك", "ط", "ذ"},
		{"ئ", "ء", "ؤ", "ر", "ى", "ة", "و", "ز", "ظ"},
	},
	"hebrew": {
		{"ק", "ר", "א", "ט", "ו", "ן", "ם", "פ"},
		{"ש", "ד", "ג", "כ", "ע", "י", "ח", "ל", "ך", "ף"},
		{"ז", "ס", "ב", "ה", "נ", "צ", "ת", "ץ"},
	},
	"devanagari": {
		{"औ", "ऐ", "आ", "ई", "ऊ", "भ", "ङ", "घ", "ध", "झ", "ढ", "ञ"},
		{"ओ", "ए", "अ", "इ", "उ", "ब", "ह", "ग", "द", "ज", "ड", "श"},
		{"ऑ", "र", "क", "त", "च", "ट", "प", "य", "स", "म", "व", "ल", "ष", "न"},
	},
	"bengali": {
		{"ঔ", "ঐ", "আ", "ঈ", "ঊ", "ভ", "ঙ", "ঘ", "ধ", "ঝ", "ঢ", "ঞ"},
		{"ও", "এ", "অ", "ই", "উ", "ব", "হ", "গ", "দ", "জ", "ড", "শ"},
		{"ঋ", "র", "ক", "ত", "চ", "ট", "প", "য", "স", "ম", "ব", "ল", "ষ", "ন"},
	},
	"tamil": {
		{"ஔ", "ஐ", "ஆ", "ஈ", "ஊ", "ங", "ஞ", "ண", "ந", "ன"},
		{"ஓ", "ஏ", "அ", "இ", "உ", "க", "ச", "ட", "த", "ப", "ற"},
		{"எ", "ஒ", "ய", "ர", "ல", "வ", "ழ", "ள", "ம", "ஷ", "ஸ", "ஹ"},
	},
	"telugu": {
		{"ఔ", "ఐ", "ఆ", "ఈ", "ఊ", "భ", "ఙ", "ఘ", "ధ", "ఝ", "ఢ", "ఞ"},
		{"ఓ", "ఏ", "అ", "ఇ", "ఉ", "బ", "హ", "గ", "ద", "జ", "డ", "శ"},
		{"ఎ", "ఒ", "ర", "క", "త", "చ", "ట", "ప", "య", "స", "మ", "వ", "ల", "ష", "న"},
	},
	"thai": {
		{"โ", "ฌ", "ฆ", "ฏ", "โ", "ซ", "ศ", "ฮ", "?", "ฒ", "ฬ", "ฦ"},
		{"ฟ", "ห", "ก", "ด", "เ", "า", "ส", "ว", "ง", "ผ", "ป", "แ", "อ"},
		{"พ", "ะ", "ั", "ร", "น", "ย", "บ", "ล", "ข", "ช", "ต", "ค", "ม"},
	},
	"hiragana": {
		{"わ", "ら", "や", "ま", "は", "な", "た", "さ", "か", "あ"},
		{"ゐ", "り", "み", "ひ", "に", "ち", "し", "き", "い"},
		{"ん", "る", "ゆ", "む", "ふ", "ぬ", "つ", "す", "く", "う"},
		{"ゑ", "れ", "め", "へ", "ね", "て", "せ", "け", "え"},
		{"を", "ろ", "よ", "も", "ほ", "の", "と", "そ", "こ", "お"},
	},
	"korean": {
		{"ㅂ", "ㅈ", "ㄷ", "ㄱ", "ㅅ", "ㅛ", "ㅕ", "ㅑ", "ㅐ", "ㅒ", "ㅔ", "ㅖ"},
		{"ㅁ", "ㄴ", "ㅇ", "ㄹ", "ㅎ", "ㅗ", "ㅓ", "ㅏ", "ㅣ"},
		{"ㅋ", "ㅌ", "ㅊ", "ㅍ", "ㅠ", "ㅜ", "ㅡ"},
	},
}

// langLayoutMap overrides script-detection for languages that use a non-default layout.
// Only should add very common langauges here for effeciency
// French/German use azerty/qwertz which are pure rearrangements of the same
// ASCII letters as qwerty — detectLayout has no unique chars to key off, so
// these must stay manual. Everything else (nordic, turkish, devanagari,
// korean, hiragana, ...) has distinguishing chars and auto-detects fine.
var langLayoutMap = map[string]string{
	"French": "azerty", "German": "qwertz",
}

// detectLayout picks the preset layout that covers the most characters found in
// a sample of up to 30 words from the word list.
func detectLayout(words map[string]string) string {
	const sampleSize = 30
	sample := make([]string, 0, sampleSize)
	for w := range words {
		sample = append(sample, w)
		if len(sample) == sampleSize {
			break
		}
	}

	// Build set of normalised chars appearing in the sample.
	chars := make(map[string]bool)
	for _, w := range sample {
		for _, r := range w {
			chars[string(r)] = true
		}
	}

	// Flatten each layout into a set of its keys for fast lookup.
	layoutKeys := make(map[string]map[string]bool, len(keyboardLayouts))
	for name, rows := range keyboardLayouts {
		s := make(map[string]bool)
		for _, row := range rows {
			for _, key := range row {
				s[key] = true
			}
		}
		layoutKeys[name] = s
	}

	best, bestCount := "qwerty", -1
	for name, keys := range layoutKeys {
		count := 0
		for ch := range chars {
			if keys[ch] {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = name, count
		}
	}
	return best
}

// buildKeyboardData returns keyboard rows (base chars) and overflow bases
// (alphabet bases not present in any layout key).
func buildKeyboardData(alphabet []string, lang string, words map[string]string) (rows [][]string, overflowBases []string, placedExact map[string]bool) {
	// If no alphabet (logographic threshold exceeded) but a preset exists, return it as-is.
	if len(alphabet) == 0 {
		name := langLayoutMap[lang]
		if name == "" {
			name = detectLayout(words)
		}
		if layout, ok := keyboardLayouts[name]; ok {
			return layout, nil, nil
		}
		return keyboardLayouts["qwerty"], nil, nil
	}

	alphabetSet := make(map[string]bool, len(alphabet))
	for _, ch := range alphabet {
		alphabetSet[ch] = true
	}

	layoutName := langLayoutMap[lang]
	if layoutName == "" {
		layoutName = detectLayout(words)
	}
	layout, ok := keyboardLayouts[layoutName]
	if !ok {
		layout = keyboardLayouts["qwerty"]
	}

	// Place exact literal matches first so preset keys with diacritics (e.g. "й")
	// aren't dropped just because normalizeChar would strip them to a different letter.
	placedExact = make(map[string]bool)
	for _, layoutRow := range layout {
		var row []string
		for _, key := range layoutRow {
			if alphabetSet[key] {
				row = append(row, key)
				placedExact[key] = true
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	// Bases already covered by a placed key (e.g. "e" covers é if "e" but not "é" is placed).
	coveredBases := make(map[string]bool, len(placedExact))
	for key := range placedExact {
		coveredBases[normalizeChar(key)] = true
	}

	overflowSet := make(map[string]bool)
	for _, ch := range alphabet {
		if placedExact[ch] {
			continue
		}
		base := normalizeChar(ch)
		if coveredBases[base] {
			continue
		}
		overflowSet[base] = true
	}
	for base := range overflowSet {
		overflowBases = append(overflowBases, base)
	}
	sort.Strings(overflowBases)

	if len(rows) == 0 {
		return nil, overflowBases, placedExact
	}
	return rows, overflowBases, placedExact
}

// computeEquivalences groups alphabet chars by their base form.
// Returns only groups with >1 member or the "*" overflow group.
// Each group is [base/label, variant1, variant2, ...].
func computeEquivalences(alphabet []string, overflowBaseSet map[string]bool, placedExact map[string]bool) [][]string {
	if len(alphabet) == 0 {
		return nil
	}

	type set = map[string]bool
	groups := make(map[string]set)
	for _, ch := range alphabet {
		base := ch
		if !placedExact[ch] {
			base = normalizeChar(ch)
			if overflowBaseSet[base] {
				base = "*"
			}
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
func buildGameExtras(alphabet []string, lang string, words map[string]string) (keyboardRows [][]string, overflowBases []string, equivalences [][]string, rtl bool) {
	var placedExact map[string]bool
	keyboardRows, overflowBases, placedExact = buildKeyboardData(alphabet, lang, words)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}
	equivalences = computeEquivalences(alphabet, overflowSet, placedExact)
	rtl = isRTL(alphabet)
	return
}

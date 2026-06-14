package main

// Keyboard layout detection and per-language key arrangement.

import "sort"

const logographicThreshold = 200

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

// langLayoutMap overrides script-detection for languages that use a non-default layout.
var langLayoutMap = map[string]string{
	"French": "azerty", "German": "qwertz", "Norwegian": "nordic",
	"Danish": "nordic", "Swedish": "nordic", "Turkish": "turkish",
}

// detectLayout infers a keyboard layout from the alphabet's Unicode script.
func detectLayout(alphabet []string) string {
	joined := ""
	for _, ch := range alphabet {
		joined += ch
	}
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

// buildKeyboardData returns keyboard rows (base chars) and overflow bases
// (alphabet bases not present in any layout key).
func buildKeyboardData(alphabet []string, lang string) (rows [][]string, overflowBases []string) {
	if len(alphabet) == 0 {
		return nil, nil
	}

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

// computeEquivalences groups alphabet chars by their base form.
// Returns only groups with >1 member or the "*" overflow group.
// Each group is [base/label, variant1, variant2, ...].
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

// Package keyboard derives on-screen keyboard layout, overflow, and
// equivalence-grouping data from a language's alphabet.
package keyboard

import (
	"slices"
	"sort"
	"strings"

	"wordgo/internal/lang"
)

var keyboardLayouts = map[string][][]string{
	"qwerty": {
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"z", "x", "c", "v", "b", "n", "m"},
	},
	"azerty": {
		{"a", "z", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"q", "s", "d", "f", "g", "h", "j", "k", "l", "m"},
		{"w", "x", "c", "v", "b", "n"},
	},
	"qwertz": {
		{"q", "w", "e", "r", "t", "z", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"y", "x", "c", "v", "b", "n", "m"},
	},
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
		{"ς", "ε", "ρ", "τ", "υ", "θ", "ι", "ο", "π"},
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
		{"ฌ", "ฆ", "ฏ", "ซ", "ศ", "ฮ", "ฒ", "ฬ", "ฦ"},
		{"ฟ", "ห", "ก", "ด", "า", "ส", "ว", "ง", "ผ", "ป", "แ", "อ"},
		{"พ", "ร", "น", "ย", "บ", "ล", "ข", "ช", "ต", "ค", "ม"},
	},
	"hiragana": {
		{"わ", "ら", "や", "ま", "は", "な", "た", "さ", "か", "あ"},
		{"ゐ", "り", "み", "ひ", "に", "ち", "し", "き", "い"},
		{"ん", "る", "ゆ", "む", "ふ", "ぬ", "つ", "す", "く", "う"},
		{"ゑ", "れ", "め", "へ", "ね", "て", "せ", "け", "え"},
		{"を", "ろ", "よ", "も", "ほ", "の", "と", "そ", "こ", "お"},
	},
	"korean": {
		{"ㅂ", "ㅈ", "ㄷ", "ㄱ", "ㅅ", "ㅛ", "ㅕ", "ㅑ", "ㅐ", "ㅔ"},
		{"ㅁ", "ㄴ", "ㅇ", "ㄹ", "ㅎ", "ㅗ", "ㅓ", "ㅏ", "ㅣ"},
		{"ㅋ", "ㅌ", "ㅊ", "ㅍ", "ㅠ", "ㅜ", "ㅡ"},
	},
	"gujarati": {
		{"ઔ", "ઐ", "આ", "ઈ", "ઊ", "ભ", "ઙ", "ઘ", "ધ", "ઝ", "ઢ", "ઞ"},
		{"ઓ", "એ", "અ", "ઇ", "ઉ", "બ", "હ", "ગ", "દ", "જ", "ડ", "શ"},
		{"ઑ", "ર", "ક", "ત", "ચ", "ટ", "પ", "ય", "સ", "મ", "વ", "લ", "ષ", "ન"},
	},
	"kannada": {
		{"ಔ", "ಐ", "ಆ", "ಈ", "ಊ", "ಭ", "ಙ", "ಘ", "ಧ", "ಝ", "ಢ", "ಞ"},
		{"ಓ", "ಏ", "ಅ", "ಇ", "ಉ", "ಬ", "ಹ", "ಗ", "ದ", "ಜ", "ಡ", "ಶ"},
		{"ಎ", "ಒ", "ರ", "ಕ", "ತ", "ಚ", "ಟ", "ಪ", "ಯ", "ಸ", "ಮ", "ವ", "ಲ", "ಷ", "ನ"},
	},
	"gurmukhi": {
		{"ਔ", "ਐ", "ਆ", "ਈ", "ਊ", "ਭ", "ਙ", "ਘ", "ਧ", "ਝ", "ਢ", "ਞ"},
		{"ਓ", "ਏ", "ਅ", "ਇ", "ਉ", "ਬ", "ਹ", "ਗ", "ਦ", "ਜ", "ਡ", "ਸ਼"},
		{"ਰ", "ਕ", "ਤ", "ਚ", "ਟ", "ਪ", "ਯ", "ਸ", "ਮ", "ਵ", "ਲ", "ਣ", "ਨ"},
	},
	"geez": {
		{"ሀ", "ለ", "ሐ", "መ", "ሠ", "ረ", "ሰ", "ሸ", "ቀ", "በ", "ተ"},
		{"ቸ", "ኀ", "ነ", "ኘ", "አ", "ከ", "ኸ", "ወ", "ዐ", "ዘ", "ዠ"},
		{"የ", "ደ", "ጀ", "ገ", "ጠ", "ጨ", "ጰ", "ጸ", "ፀ", "ፈ", "ፐ"},
	},
	"georgian": {
		{"ქ", "წ", "ე", "რ", "ტ", "ყ", "უ", "ი", "ო", "პ"},
		{"ა", "ს", "დ", "ფ", "გ", "ჰ", "ჯ", "კ", "ლ"},
		{"ძ", "ხ", "ც", "ვ", "ბ", "ნ", "მ"},
	},
	"armenian": {
		{"Խ", "Ւ", "Է", "Ր", "Տ", "Ե", "Ը", "Ի", "Ո", "Պ", "Չ", "Ջ"},
		{"Ա", "Ս", "Դ", "Ֆ", "Ք", "Հ", "Ճ", "Կ", "Լ", "Թ", "Փ"},
		{"Զ", "Ց", "Գ", "Վ", "Բ", "Ն", "Մ", "Շ", "Ղ", "Ծ"},
	},
	"cherokee": {
		{"Ꭰ", "Ꭱ", "Ꭲ", "Ꭳ", "Ꭴ", "Ꭵ", "Ꭶ", "Ꭷ", "Ꭸ", "Ꭹ"},
		{"Ꭺ", "Ꭻ", "Ꭼ", "Ꭽ", "Ꭾ", "Ꭿ", "Ꮀ", "Ꮁ", "Ꮂ", "Ꮃ"},
		{"Ꮄ", "Ꮅ", "Ꮆ", "Ꮇ", "Ꮈ", "Ꮉ", "Ꮊ", "Ꮋ", "Ꮌ", "Ꮍ"},
		{"Ꮎ", "Ꮏ", "Ꮐ", "Ꮑ", "Ꮒ", "Ꮓ", "Ꮔ", "Ꮕ", "Ꮖ", "Ꮗ"},
		{"Ꮘ", "Ꮙ", "Ꮚ", "Ꮛ", "Ꮜ", "Ꮝ", "Ꮞ", "Ꮟ", "Ꮠ", "Ꮡ"},
		{"Ꮢ", "Ꮣ", "Ꮤ", "Ꮥ", "Ꮦ", "Ꮧ", "Ꮨ", "Ꮩ", "Ꮪ", "Ꮫ"},
		{"Ꮬ", "Ꮭ", "Ꮮ", "Ꮯ", "Ꮰ", "Ꮱ", "Ꮲ", "Ꮳ", "Ꮴ", "Ꮵ"},
		{"Ꮶ", "Ꮷ", "Ꮸ", "Ꮹ", "Ꮺ", "Ꮻ", "Ꮼ", "Ꮽ", "Ꮾ", "Ꮿ"},
		{"Ᏸ", "Ᏹ", "Ᏺ", "Ᏻ", "Ᏼ"},
	},
	"syllabics": {
		{"ᐁ", "ᐃ", "ᐅ", "ᐊ"},
		{"ᐍ", "ᐏ", "ᐓ", "ᐘ"},
		{"ᐯ", "ᐱ", "ᐳ", "ᐸ"},
		{"ᑌ", "ᑎ", "ᑐ", "ᑕ"},
		{"ᑫ", "ᑭ", "ᑯ", "ᑲ"},
		{"ᒉ", "ᒋ", "ᒍ", "ᒐ"},
		{"ᒣ", "ᒥ", "ᒧ", "ᒪ"},
		{"ᓀ", "ᓂ", "ᓄ", "ᓇ"},
		{"ᓭ", "ᓯ", "ᓱ", "ᓴ"},
		{"ᔦ", "ᔨ", "ᔪ", "ᔭ"},
		{"ᓓ", "ᓕ", "ᓗ", "ᓚ"},
		{"ᕃ", "ᕆ", "ᕈ", "ᕒ"},
		{"ᐤ", "ᐦ", "ᐨ", "ᐠ", "ᒼ", "ᐣ", "ᐢ", "ᐩ"},
	},
	"thaana": {
		{"ޤ", "ވ", "އ", "ރ", "ތ", "ޔ", "ޕ"},
		{"ސ", "ދ", "ފ", "ގ", "ހ", "ޖ", "ކ", "ލ"},
		{"ޒ", "ޚ", "ޛ", "ވ", "ބ", "ނ", "މ", "ށ", "ޏ"},
	},
	"osage": {
		{"𐒰", "𐒱", "𐒲", "𐒳", "𐒴", "𐒵", "𐒶", "𐒷", "𐒸", "𐒹", "𐒺", "𐒻"},
		{"𐒼", "𐒽", "𐒾", "𐒿", "𐓀", "𐓁", "𐓂", "𐓃", "𐓄", "𐓅", "𐓆", "𐓇"},
		{"𐓈", "𐓉", "𐓊", "𐓋", "𐓌", "𐓍", "𐓎", "𐓏", "𐓐", "𐓑", "𐓒", "𐓓"},
	},
	"vietnamese": {
		{"q", "ư", "e", "ê", "r", "ˀ", "t", "y", "u", "i", "o", "ô", "ơ", "p"},
		{"a", "ă", "â", "s", "´", "d", "đ", "`", "g", "h", ".", "k", "l"},
		{"x", "~", "c", "v", "b", "n", "m"},
	},
	// Every dialect's tone is folded by lang.ChineseToneify into one of the four traditional Middle Chinese
	// tone categories (see https://en.wikipedia.org/wiki/Four_tones_(Middle_Chinese)),
	// so the same four tone keys cover every dialect's romanization.
	"chinese": {
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"z", "x", "c", "ü", "b", "n", "m"},
		{lang.TonePing, lang.ToneShang, lang.ToneQu, lang.ToneRu}, // 平 上 去 入
	},
}

func isSyllabary(keyboardLayout string) bool {
	marked := []string{"hiragana", "geez", "syllabics", "cherokee", "vietnamese", "chinese"}
	if slices.Contains(marked, keyboardLayout) {
		return true
	}
	return false
}

// resolveLayoutOverride returns the explicit preset layout for a language,
// bypassing char-sampling detectLayout. Covers langLayoutMap plus Vietnamese
// and "Chinese (Dialect)" pseudo-languages, which need the tone-mark keys above.
func resolveLayoutOverride(lng string) string {
	if name, ok := langLayoutMap[lng]; ok {
		return name
	}
	if strings.EqualFold(lng, "Vietnamese") {
		return "vietnamese"
	}
	if strings.EqualFold(lng, "Chinese") || strings.HasPrefix(lng, "Chinese (") {
		return "chinese"
	}
	return ""
}

// DefaultLengthForLang picks the default word length before the word list is
// fetched: 3 for syllabary-style layouts (each "letter" carries more info,
// e.g. tone marks occupy their own tile), 6 otherwise.
func DefaultLengthForLang(lng string) int {
	name := resolveLayoutOverride(lng)
	if name != "" && isSyllabary(name) {
		return 3
	}
	return 6
}

// langLayoutMap overrides script-detection for languages whose alphabet is a
// pure rearrangement of qwerty's 26 ASCII letters (azerty, qwertz, geez), since
// detectLayout has no unique chars to key off. Keep small — others auto-detect fine.
var langLayoutMap = map[string]string{
	"English": "qwerty", "French": "azerty", "German": "qwertz",
	"Amharic": "geez", "Tigrinya": "geez",
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

	// qwerty is the default: only switch if another layout covers chars qwerty
	// doesn't (e.g. æ, ğ, й). nordic/turkish are supersets of qwerty's 26
	// letters, so plain-overlap scoring would tie and pick an arbitrary one.
	qwertyKeys := layoutKeys["qwerty"]
	best, bestDistinct := "qwerty", 0
	for name, keys := range layoutKeys {
		if name == "qwerty" {
			continue
		}
		distinct := 0
		for ch := range chars {
			if keys[ch] && !qwertyKeys[ch] {
				distinct++
			}
		}
		if distinct > bestDistinct {
			best, bestDistinct = name, distinct
		}
	}
	return best
}

// BuildKeyboardData returns keyboard rows (base chars) and overflow bases
// (alphabet bases not present in any layout key).
func BuildKeyboardData(alphabet []string, lng string, words map[string]string) (rows [][]string, overflowBases []string, placedExact map[string]bool) {
	// If no alphabet (logographic threshold exceeded) but a preset exists, return it as-is.
	if len(alphabet) == 0 {
		name := resolveLayoutOverride(lng)
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

	layoutName := resolveLayoutOverride(lng)
	if layoutName == "" {
		layoutName = detectLayout(words)
	}
	layout, ok := keyboardLayouts[layoutName]
	if !ok {
		layout = keyboardLayouts["qwerty"]
	}

	// Place exact literal matches first so preset keys with diacritics (e.g. "й")
	// aren't dropped just because NormalizeChar would strip them to a different letter.
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
		coveredBases[lang.NormalizeChar(key)] = true
	}

	overflowSet := make(map[string]bool)
	for _, ch := range alphabet {
		if placedExact[ch] {
			continue
		}
		base := lang.NormalizeChar(ch)
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

// ComputeEquivalences groups alphabet chars by their base form.
// Returns only groups with >1 member or the "*" overflow group.
// Each group is [base/label, variant1, variant2, ...].
func ComputeEquivalences(alphabet []string, overflowBaseSet map[string]bool, placedExact map[string]bool) [][]string {
	if len(alphabet) == 0 {
		return nil
	}

	type set = map[string]bool
	groups := make(map[string]set)
	for _, ch := range alphabet {
		base := ch
		if !placedExact[ch] {
			base = lang.NormalizeChar(ch)
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

// BuildGameExtras computes all derived UI data from the alphabet in one call.
func BuildGameExtras(alphabet []string, lng string, words map[string]string) (keyboardRows [][]string, overflowBases []string, equivalences [][]string, rtl bool, matraMap map[string]string) {
	var placedExact map[string]bool
	keyboardRows, overflowBases, placedExact = BuildKeyboardData(alphabet, lng, words)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}
	equivalences = ComputeEquivalences(alphabet, overflowSet, placedExact)

	layoutName := resolveLayoutOverride(lng)
	if layoutName == "" {
		layoutName = detectLayout(words)
	}

	// this is so it is displayed ltr even if there are some in a rtl script in * chars
	if layoutName == "arabic" || layoutName == "hebrew" {
		rtl = true
	} else {
		rtl = false
	}

	matraMap = lang.MatraTable(layoutName)

	return
}

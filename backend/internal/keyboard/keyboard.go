package keyboard

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"sync"

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
		{"ঋ", "র", "ক", "ত", "চ", "ট", "প", "য", "স", "ম", "ল", "ষ", "ন"},
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
	// ー (the prolonged-sound mark) is a key of its own: it is not in the
	// hiragana block, so without it every word carrying one (らーめん) would
	// land in the overflow "*" bucket instead of on the ー key the flick
	// keyboard already draws.
	"hiragana": {
		{"や", "わ", "ら", "ま", "は", "な", "た", "さ", "か", "あ", "ー"},
		{"ゐ", "り", "み", "ひ", "に", "ち", "し", "き", "い"},
		{"ゆ", "ん", "る", "む", "ふ", "ぬ", "つ", "す", "く", "う"},
		{"ゑ", "れ", "め", "へ", "ね", "て", "せ", "け", "え"},
		{"よ", "を", "ろ", "も", "ほ", "の", "と", "そ", "こ", "お"},
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
		{"խ", "ւ", "է", "ր", "տ", "ե", "ը", "ի", "ո", "պ", "չ", "ջ"},
		{"ա", "ս", "դ", "ֆ", "ք", "հ", "ճ", "կ", "լ", "թ", "փ"},
		{"զ", "ց", "գ", "վ", "բ", "ն", "մ", "շ", "ղ", "ծ"},
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
		{"ޤ", "އ", "ރ", "ތ", "ޔ", "ޕ"},
		{"ސ", "ދ", "ފ", "ގ", "ހ", "ޖ", "ކ", "ލ"},
		{"ޒ", "ޚ", "ޛ", "ވ", "ބ", "ނ", "މ", "ށ", "ޏ"},
	},
	"osage": {
		{"𐓘", "𐓙", "𐓚", "𐓛", "𐓜", "𐓝", "𐓞", "𐓟", "𐓠", "𐓡", "𐓢", "𐓣"},
		{"𐓤", "𐓥", "𐓦", "𐓧", "𐓨", "𐓩", "𐓪", "𐓫", "𐓬", "𐓭", "𐓮", "𐓯"},
		{"𐓰", "𐓱", "𐓲", "𐓳", "𐓴", "𐓵", "𐓶", "𐓷", "𐓸", "𐓹", "𐓺", "𐓻"},
	},
	// Tone marks are typed as their own keystroke in a Vietnamese IME
	// (Telex/VNI), so they get their own keys — lang.WordChars splits them
	// into their own tile to match.
	"vietnamese": {
		{"q", "ư", "e", "ê", "r", "ˀ", "t", "y", "u", "i", "o", "ô", "ơ", "p"},
		{"a", "ă", "â", "s", "´", "d", "đ", "`", "g", "h", ".", "k", "l"},
		{"x", "~", "c", "v", "b", "n", "m"},
	},
	"chinese": {
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"z", "x", "c", "ü", "b", "n", "m"},
	},
	"cangjie": {
		{"手", "田", "水", "口", "廿", "卜", "山", "戈", "人", "心"},
		{"日", "尸", "木", "火", "土", "竹", "十", "大", "中"},
		{"金", "女", "月", "弓", "一"},
	},
	"zhuyin": {
		{"ㄅ", "ㄉ", "ㄓ", "ㄚ", "ㄞ", "ㄢ", "ㄦ"},
		{"ㄆ", "ㄊ", "ㄍ", "ㄐ", "ㄔ", "ㄗ", "ㄧ", "ㄛ", "ㄟ", "ㄣ"},
		{"ㄇ", "ㄋ", "ㄎ", "ㄑ", "ㄕ", "ㄙ", "ㄨ", "ㄜ", "ㄠ", "ㄤ"},
		{"ㄈ", "ㄌ", "ㄏ", "ㄒ", "ㄖ", "ㄘ", "ㄩ", "ㄝ", "ㄡ", "ㄥ"},
		{"ˊ", "ˇ", "ˋ", "˙"}, // 2nd, 3rd, 4th, neutral tone (1st is unmarked)
	},
}

// syllabaryLayouts are the layouts whose keys are whole syllables (or, for
// Cangjie, a whole root-glyph code), so a word needs fewer tiles.
var syllabaryLayouts = []string{"hiragana", "geez", "syllabics", "cherokee", "cangjie"}

func isSyllabary(keyboardLayout string) bool {
	return slices.Contains(syllabaryLayouts, keyboardLayout)
}

// resolveLayoutOverride returns the explicit preset layout for a language,
// bypassing char-sampling detectLayout. Covers langLayoutMap plus Vietnamese
// (which needs the tone-mark keys above) and the "Chinese (Dialect)"
// pseudo-languages, whose words are romanizations rather than native script.
func resolveLayoutOverride(lng string) string {
	if name, ok := langLayoutMap[lng]; ok {
		return name
	}
	if strings.EqualFold(lng, "Vietnamese") {
		return "vietnamese"
	}
	if strings.EqualFold(lng, "Chinese (Cangjie)") {
		return "cangjie"
	}
	if strings.EqualFold(lng, "Chinese (Zhuyin)") {
		return "zhuyin"
	}
	if strings.EqualFold(lng, "Chinese") || strings.HasPrefix(lng, "Chinese (") {
		return "chinese"
	}
	return ""
}

// isTonal reports whether a layout spends extra tiles on tone keystrokes
// (Vietnamese tone marks, zhuyin tone marks), so a normal-length word needs
// more tiles to fit.
func isTonal(keyboardLayout string) bool {
	return keyboardLayout == "vietnamese" || keyboardLayout == "zhuyin"
}

// DefaultLengthForLang estimates a word length from the script alone. It's
// only the stand-in shown before a language has ever been downloaded — once
// it has, wordlist.DefaultLength serves that language's measured median
// word length instead. The estimate is 4 for syllabary-style layouts, Cangjie
// included (each "letter" carries more info — for Cangjie, a root-glyph code
// per character), 8 for tonal layouts (tones take their own tile), 6 otherwise.
//
// Only a language with an explicit layout override can be recognised here:
// there is no word list yet to run detection over, so anything else gets the
// generic 6. That is why the syllabary scripts are listed in langLayoutMap.
func DefaultLengthForLang(lng string) int {
	name := resolveLayoutOverride(lng)
	if isSyllabary(name) {
		return 4
	}
	if isTonal(name) {
		return 8
	}
	return 6
}

// langLayoutMap overrides script-detection for languages detectLayout can't
// get right on its own: the ones whose letters are a rearrangement of the
// same ASCII set qwerty already covers (azerty, qwertz — detection scores
// them identically), and the syllabary scripts, whose layout also decides
// DefaultLengthForLang's pre-download estimate.
var langLayoutMap = map[string]string{
	"English": "qwerty", "French": "azerty", "German": "qwertz",
	"Amharic": "geez", "Tigrinya": "geez", "Japanese": "hiragana",
	"Cherokee": "cherokee", "Inuktitut": "syllabics",
}

// layoutKeySets flattens every preset into a set of its keys for fast lookup,
// built once rather than on every detectLayout call.
var layoutKeySets = sync.OnceValue(func() map[string]map[string]bool {
	sets := make(map[string]map[string]bool, len(keyboardLayouts))
	for name, rows := range keyboardLayouts {
		s := make(map[string]bool)
		for _, row := range rows {
			for _, key := range row {
				s[key] = true
			}
		}
		sets[name] = s
	}
	return sets
})

// overrideOnlyLayouts are never candidates for script detection: each one
// exists for the language resolveLayoutOverride hands it to, and its extra
// keys are meaningless anywhere else. Without this, any Latin-script language
// using ü picked up the pinyin "chinese" layout, and Romanian's ă/â pulled in
// the Vietnamese one along with four dead tone keys.
var overrideOnlyLayouts = map[string]bool{
	"vietnamese": true,
	"chinese":    true,
	"cangjie":    true,
	"zhuyin":     true,
}

// minDetectShare is how much more of a word list's text a layout has to cover
// than qwerty does before detection switches to it. A language's own script
// accounts for nearly all of its characters (Arabic 96%, Russian 99%, Hindi
// 60%), and a real Latin diacritic for a few percent (Danish's å/ø/æ 4.4%,
// Turkish's ı/ğ/ş 9.6%); a few stray loanwords account for a fraction of one
// (English 0.02%, Spanish 0.03%). The threshold is what separates a
// language's own letters from another script's leftovers.
const minDetectShare = 0.02

// layoutCoverage is the share of a word list's characters a layout has a key
// for — freq is rune -> occurrences and total their sum.
func layoutCoverage(keys map[string]bool, freq map[string]int, total int) float64 {
	covered := 0
	for ch, n := range freq {
		if keys[ch] {
			covered += n
		}
	}
	return float64(covered) / float64(total)
}

// detectLayout picks the preset layout that types the most of a word list's
// text, measured over every character occurrence in the list.
//
// It used to score the 30 lexicographically smallest words on how many
// distinct characters a layout covered. That sample is deterministic but not
// representative: it is whichever words start with the lowest code points, so
// a handful of foreign-script entries decided the layout for a whole
// language. The Arabic dump carries 15 Hebrew-script words among 12,933;
// Hebrew (U+05xx) sorts before Arabic (U+06xx), so all 30 sampled words were
// Hebrew and an Arabic game came back with a Hebrew keyboard and its whole
// alphabet dumped into the "*" overflow key. Punjabi picked up Arabic the
// same way, and Latin languages picked up whatever layout their first few
// accented words happened to touch.
//
// Counting the whole list is both representative and order-independent, so
// the same word list always detects the same layout however Go happens to
// range over it. Ties between layouts break on the layout name, for the same
// reason. qwerty is scored alongside the rest rather than treated as a
// fallback with no score of its own: a mostly-Latin list with an Ajami
// minority (Yoruba: 77% Latin, 10% Arabic) has to stay on qwerty, and only a
// layout that beats qwerty by minDetectShare is worth the switch.
func detectLayout(words map[string]string) string {
	freq := make(map[string]int)
	total := 0
	for w := range words {
		for _, r := range w {
			freq[string(r)]++
			total++
		}
	}
	if total == 0 {
		return "qwerty"
	}

	layoutKeys := layoutKeySets()
	qwertyShare := layoutCoverage(layoutKeys["qwerty"], freq, total)

	best, bestShare := "qwerty", qwertyShare
	for _, name := range slices.Sorted(maps.Keys(layoutKeys)) {
		if name == "qwerty" || overrideOnlyLayouts[name] {
			continue
		}
		// Sorted names and a strict >, so an exact tie keeps the layout that
		// sorts first rather than whichever one came up last.
		if share := layoutCoverage(layoutKeys[name], freq, total); share > bestShare {
			best, bestShare = name, share
		}
	}
	// nordic and turkish are supersets of qwerty's 26 letters, so they edge
	// it out on any English text; the margin is what says whether the extra
	// keys are the language's own letters or somebody's loanword.
	if bestShare-qwertyShare < minDetectShare {
		return "qwerty"
	}
	return best
}

// layoutFor resolves a language's layout name once: its explicit override if
// it has one, else script detection over the word list.
func layoutFor(lng string, words map[string]string) string {
	if name := resolveLayoutOverride(lng); name != "" {
		return name
	}
	return detectLayout(words)
}

// BuildKeyboardData returns keyboard rows (base chars) and overflow bases
// (alphabet bases not present in any layout key).
func BuildKeyboardData(alphabet []string, lng string, words map[string]string) (rows [][]string, overflowBases []string, placedExact map[string]bool) {
	return buildKeyboardDataForLayout(alphabet, layoutFor(lng, words))
}

// buildKeyboardDataForLayout is BuildKeyboardData with the layout already
// resolved, so a caller that needs the layout name too (BuildGameExtras)
// doesn't run detection twice over the same word list.
func buildKeyboardDataForLayout(alphabet []string, layoutName string) (rows [][]string, overflowBases []string, placedExact map[string]bool) {
	// If no alphabet (logographic threshold exceeded) but a preset exists, return it as-is.
	if len(alphabet) == 0 {
		if layout, ok := keyboardLayouts[layoutName]; ok {
			return layout, nil, nil
		}
		return keyboardLayouts["qwerty"], nil, nil
	}

	alphabetSet := make(map[string]bool, len(alphabet))
	for _, ch := range alphabet {
		alphabetSet[ch] = true
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
	overflowBases = slices.Sorted(maps.Keys(overflowSet))

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
		members = append(members, base)
		for ch := range chars {
			if base == "*" || ch != base {
				members = append(members, ch)
			}
		}
		slices.Sort(members[1:])
		result = append(result, members)
	}

	// The "*" overflow group always sorts first; the rest sort by base char.
	slices.SortFunc(result, func(a, b []string) int {
		if (a[0] == "*") != (b[0] == "*") {
			if a[0] == "*" {
				return -1
			}
			return 1
		}
		return cmp.Compare(a[0], b[0])
	})
	return result
}

// BuildGameExtras computes all derived UI data from the alphabet in one call.
func BuildGameExtras(alphabet []string, lng string, words map[string]string) (keyboardRows [][]string, overflowBases []string, equivalences [][]string, rtl bool, matraMap map[string]string, layoutName string) {
	layoutName = layoutFor(lng, words)

	var placedExact map[string]bool
	keyboardRows, overflowBases, placedExact = buildKeyboardDataForLayout(alphabet, layoutName)
	overflowSet := make(map[string]bool, len(overflowBases))
	for _, b := range overflowBases {
		overflowSet[b] = true
	}
	equivalences = ComputeEquivalences(alphabet, overflowSet, placedExact)

	// this is so it is displayed ltr even if there are some in a rtl script in * chars
	rtl = layoutName == "arabic" || layoutName == "hebrew"

	matraMap = lang.MatraTable(layoutName)

	return
}

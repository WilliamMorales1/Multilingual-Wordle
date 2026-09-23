// Recombines decomposed Hangul compatibility jamo (e.g. "ㄱㅓㄷㅡㄹㄷㅏ") into syllable blocks.
const LEADS = ['ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];
const VOWELS = ['ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'];
const FINALS = ['', 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];

// JAMO_MERGE inverts the backend's ExpandJamo (backend/internal/lang/lang.go),
// which splits every doubled consonant and compound vowel into two plain jamo
// before a Korean word is stored. Without merging them back, "까" arrives as
// "ㄱㄱㅏ" and composes to "ㄱ가", and "과" as "ㄱㅗㅏ" composing to "고ㅏ".
const JAMO_MERGE: Record<string, string> = {
  'ㄱㄱ': 'ㄲ', 'ㄷㄷ': 'ㄸ', 'ㅂㅂ': 'ㅃ', 'ㅅㅅ': 'ㅆ', 'ㅈㅈ': 'ㅉ',
  'ㄱㅅ': 'ㄳ', 'ㄴㅈ': 'ㄵ', 'ㄴㅎ': 'ㄶ',
  'ㄹㄱ': 'ㄺ', 'ㄹㅁ': 'ㄻ', 'ㄹㅂ': 'ㄼ', 'ㄹㅅ': 'ㄽ', 'ㄹㅌ': 'ㄾ', 'ㄹㅍ': 'ㄿ', 'ㄹㅎ': 'ㅀ',
  'ㅂㅅ': 'ㅄ',
  'ㅗㅏ': 'ㅘ', 'ㅗㅐ': 'ㅙ', 'ㅗㅣ': 'ㅚ',
  'ㅜㅓ': 'ㅝ', 'ㅜㅔ': 'ㅞ', 'ㅜㅣ': 'ㅟ',
  'ㅡㅣ': 'ㅢ',
};

function toIndexMap(list: string[]): Map<string, number> {
  const m = new Map<string, number>();
  list.forEach((c, i) => { if (c !== '') m.set(c, i); });
  return m;
}

const leadIndex = toIndexMap(LEADS);
const vowelIndex = toIndexMap(VOWELS);
const finalIndex = toIndexMap(FINALS);

// A jamo slot can be filled by one character or by a merged pair, so each
// position offers up to two readings: [slot index, characters consumed].
// The merged pair is offered first — it is the one ExpandJamo took apart.
type Slot = [index: number, consumed: number];

function slots(map: Map<string, number>, chars: string[], i: number): Slot[] {
  const out: Slot[] = [];
  const pair = JAMO_MERGE[(chars[i] ?? '') + (chars[i + 1] ?? '')];
  const merged = pair !== undefined ? map.get(pair) : undefined;
  if (merged !== undefined) out.push([merged, 2]);
  const single = map.get(chars[i] ?? '');
  if (single !== undefined) out.push([single, 1]);
  return out;
}

export function composeHangul(input: string): string {
  const chars = Array.from(input);
  let out = '';
  let i = 0;
  while (i < chars.length) {
    const lead = slots(leadIndex, chars, i).find(l => slots(vowelIndex, chars, i + l[1]).length > 0);
    const vowel = lead && slots(vowelIndex, chars, i + lead[1])[0];
    if (!lead || !vowel) {
      out += chars[i];
      i += 1;
      continue;
    }
    let j = i + lead[1] + vowel[1];

    // A trailing consonant belongs to this syllable only if a vowel does not
    // follow it: in "각가" (ㄱㅏㄱㄱㅏ) the ㄱㄱ is a final ㄱ plus the next
    // syllable's lead, not the merged ㄲ. Each reading is tried in turn, so
    // rejecting the pair still leaves the single jamo available.
    let final = 0;
    const fin = slots(finalIndex, chars, j).find(f => slots(vowelIndex, chars, j + f[1]).length === 0);
    if (fin) {
      final = fin[0];
      j += fin[1];
    }

    out += String.fromCodePoint(0xac00 + (lead[0] * 21 + vowel[0]) * 28 + final);
    i = j;
  }
  return out;
}

// HANGUL_LANGS are the languages whose word lists the backend stores as
// expanded compatibility jamo (lang.IsHangulLang in backend/internal/lang/
// lang.go). Their answers only read as Korean once composeHangul puts the
// syllable blocks back together, so anything showing the answer as text — the
// Wiktionary link above all — has to cover all three. Testing the language
// name with startsWith('Korean') covered only one of them, leaving Middle
// Korean and Jeju linking to a string of loose jamo no Wiktionary page has.
const HANGUL_LANGS = ['korean', 'middle korean', 'jeju'];

export function isHangulLang(lang: string): boolean {
  return HANGUL_LANGS.includes(lang.trim().toLowerCase());
}

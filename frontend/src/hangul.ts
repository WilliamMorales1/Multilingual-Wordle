// Recombines decomposed Hangul compatibility jamo (e.g. "ㄱㅓㄷㅡㄹㄷㅏ") into syllable blocks.
const LEADS = ['ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];
const VOWELS = ['ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'];
const FINALS = ['', 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];

function toIndexMap(list: string[]): Map<string, number> {
  const m = new Map<string, number>();
  list.forEach((c, i) => { if (c !== '') m.set(c, i); });
  return m;
}

const leadIndex = toIndexMap(LEADS);
const vowelIndex = toIndexMap(VOWELS);
const finalIndex = toIndexMap(FINALS);

export function composeHangul(input: string): string {
  const chars = Array.from(input);
  let out = '';
  let i = 0;
  while (i < chars.length) {
    const c = chars[i];
    const lead = leadIndex.get(c);
    const nextVowel = i + 1 < chars.length ? vowelIndex.get(chars[i + 1]) : undefined;
    if (lead !== undefined && nextVowel !== undefined) {
      const vowel = nextVowel;
      i += 2;
      let final = 0;
      if (i < chars.length) {
        const finalCandidate = finalIndex.get(chars[i]);
        const followingVowel = i + 1 < chars.length ? vowelIndex.get(chars[i + 1]) : undefined;
        if (finalCandidate !== undefined && followingVowel === undefined) {
          final = finalCandidate;
          i += 1;
        }
      }
      out += String.fromCodePoint(0xac00 + (lead * 21 + vowel) * 28 + final);
    } else {
      out += c;
      i += 1;
    }
  }
  return out;
}

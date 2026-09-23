// Regression tests for composeHangul, which turns the jamo string the backend
// stores back into Hangul syllables for the Wiktionary link on the stats
// modal. Bundles the TypeScript source with esbuild, so the test runs the same
// code the browser gets.
//
// What is pinned here: the backend's ExpandJamo splits every doubled consonant
// (ㄲ -> ㄱㄱ) and compound vowel (ㅘ -> ㅗㅏ) before storing a Korean word, so
// composing has to merge them back — while still reading a run like ㄱㄱ in
// "각가" as a final plus the next syllable's lead.
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

let failures = 0;
function check(name, got, want) {
  if (got === want) {
    console.log(`  ok   ${name}`);
  } else {
    failures++;
    console.error(`  FAIL ${name}\n       got  ${got}\n       want ${want}`);
  }
}

const dir = await mkdtemp(path.join(tmpdir(), 'wordgo-hangul-'));
const outfile = path.join(dir, 'hangul.mjs');
await build({
  entryPoints: [path.join(here, '..', 'src', 'hangul.ts')],
  bundle: true,
  format: 'esm',
  outfile,
  logLevel: 'silent',
});
const { composeHangul, isHangulLang } = await import(pathToFileURL(outfile).href);

console.log('composeHangul');

// Plain syllables: lead + vowel, with and without a final.
check('lead + vowel', composeHangul('ㅋㅐ'), '캐');
check('lead + vowel + final', composeHangul('ㅎㅏㄴㄱㅜㄱ'), '한국');

// Merged back from ExpandJamo.
check('doubled lead (ㄱㄱ -> ㄲ)', composeHangul('ㄱㄱㅏ'), '까');
check('compound vowel (ㅗㅏ -> ㅘ)', composeHangul('ㄱㅗㅏ'), '과');
check('compound vowel ㅡㅣ -> ㅢ', composeHangul('ㅇㅡㅣㅅㅏ'), '의사');
check('cluster final (ㄹㄱ -> ㄺ)', composeHangul('ㄷㅏㄹㄱ'), '닭');
check('doubled final (ㄱㄱ -> ㄲ)', composeHangul('ㅂㅏㄱㄱ'), '밖');

// The same two jamo have to stay a final plus the next lead when a vowel
// follows, or 각가 would come out as 갂 plus a stray ㅏ.
check('final then lead, not a merge', composeHangul('ㄱㅏㄱㄱㅏ'), '각가');
check('cluster split across syllables', composeHangul('ㄷㅏㄹㄱㅏ'), '달가');
check('final before a new syllable', composeHangul('ㅈㅗㅎㄷㅏ'), '좋다');

// Anything that is not jamo is passed through untouched.
check('non-jamo passthrough', composeHangul('abc'), 'abc');
check('empty input', composeHangul(''), '');
check('lone lead', composeHangul('ㄱ'), 'ㄱ');

console.log('isHangulLang');

// The backend stores Middle Korean and Jeju as expanded jamo too
// (lang.IsHangulLang), so the stats modal has to compose their answers as
// well. Matching the language name with startsWith('Korean') recognised only
// one of the three and linked the other two to a string of loose jamo.
for (const lang of ['Korean', 'Middle Korean', 'Jeju', 'korean', ' jeju ']) {
  check(`${lang} is a hangul language`, isHangulLang(lang), true);
}
for (const lang of ['Japanese', 'English', 'Chinese (Mandarin)', 'Korean Sign Language']) {
  check(`${lang} is not a hangul language`, isHangulLang(lang), false);
}

await rm(dir, { recursive: true, force: true });
process.exit(failures === 0 ? 0 : 1);

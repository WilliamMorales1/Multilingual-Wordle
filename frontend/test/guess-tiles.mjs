// Regression tests for the guess-submission path, run in plain node against
// a stub DOM (see dom-stub.mjs). Bundles the TypeScript sources with esbuild
// first, so the test exercises the same code the browser gets.
//
// What is pinned here:
//   * a submitted row renders the server's tiles, not the typed string split
//     by code point — the two differ whenever the server resolves the guess
//     to a different word (a "*" wildcard, an accent-insensitive match) or
//     whenever a tile spans more than one code point (an Indic cluster).
//   * bounceRow animates the whole row instead of bailing out on the first
//     tile it can't find.

import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { installDOM, stubFetch, sleep } from './dom-stub.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));

let failures = 0;
function check(name, got, want) {
  const g = JSON.stringify(got);
  const w = JSON.stringify(want);
  if (g === w) {
    console.log(`  ok   ${name}`);
  } else {
    failures++;
    console.error(`  FAIL ${name}\n       got  ${g}\n       want ${w}`);
  }
}

async function bundle() {
  const dir = await mkdtemp(path.join(tmpdir(), 'wordgo-test-'));
  const outfile = path.join(dir, 'entry.mjs');
  await build({
    entryPoints: [path.join(here, 'entry.ts')],
    bundle: true,
    format: 'esm',
    outfile,
    logLevel: 'silent',
  });
  return { outfile, cleanup: () => rm(dir, { recursive: true, force: true }) };
}

// tileTexts reads back what the board shows for a row.
const tileTexts = (dom, row, n) =>
  Array.from({ length: n }, (_, c) => dom.el(`tile-${row}-${c}`).textContent);

// prepare wires up a board wide enough for `wordLength` tiles and returns the
// freshly imported module (one import per case, so state starts clean).
async function prepare(outfile, wordLength, response) {
  const dom = installDOM();
  dom.el('toast');
  dom.el('board');
  dom.el('keyboard');
  dom.el('row-0');
  dom.el('caption-0');
  for (let c = 0; c < wordLength; c++) dom.el(`tile-0-${c}`);

  stubFetch(response);
  // Cache-bust so each case gets its own module instance and its own S.
  const mod = await import(`${pathToFileURL(outfile).href}?case=${Math.random()}`);
  return { dom, mod };
}

async function testWildcardGuessShowsResolvedWord(outfile) {
  // The player types "ca*e"; the server resolves the wildcard and answers
  // with the real word's tiles.
  const { dom, mod } = await prepare(outfile, 4, {
    attempt: 1,
    word: 'cane',
    tiles: ['c', 'a', 'n', 'e'],
    states: ['correct', 'correct', 'absent', 'correct'],
    status: 'playing',
    in_word_list: true,
    key_states: {},
  });

  Object.assign(mod.S, {
    gameId: 1, wordLength: 4, status: 'playing', currentRow: 0,
    input: ['c', 'a', '*', 'e'], history: [], charStates: {}, matraMap: {},
  });

  await mod.onEnter();
  await sleep(4 * 400 + 500); // let the flip animation finish

  check('wildcard row shows the resolved word',
    tileTexts(dom, 0, 4), ['C', 'A', 'N', 'E']);
}

async function testMultiCodePointTilesStayAligned(outfile) {
  // "क्या" is 4 code points but 3 tiles: क + virama is one cluster, then य,
  // then the matra ा. Splitting the string by code point would put the
  // virama in its own tile and slide every state one position over.
  const { dom, mod } = await prepare(outfile, 3, {
    attempt: 1,
    word: 'क्या',
    tiles: ['क्', 'य', 'ा'],
    states: ['correct', 'present', 'absent'],
    status: 'playing',
    in_word_list: true,
    key_states: {},
  });

  Object.assign(mod.S, {
    gameId: 1, wordLength: 3, status: 'playing', currentRow: 0,
    input: ['क्', 'य', 'ा'], history: [], charStates: {}, matraMap: {},
  });

  await mod.onEnter();
  await sleep(3 * 400 + 500);

  check('indic row keeps one tile per cluster',
    tileTexts(dom, 0, 3), ['क्', 'य', '◌ा']);
  check('tile count matches the state count',
    tileTexts(dom, 0, 3).filter(Boolean).length, 3);
}

async function testBounceRowCoversWholeRow(outfile) {
  const dom = installDOM();
  dom.el('board');
  // Tile 1 is deliberately missing: bounceRow used to `return` on the first
  // gap and silently skip every tile after it.
  dom.el('tile-0-0');
  dom.el('tile-0-2');

  const mod = await import(`${pathToFileURL(outfile).href}?case=${Math.random()}`);
  mod.S.wordLength = 3;
  mod.bounceRow(0);
  await sleep(3 * 80 + 100);

  check('bounce reaches the tile after a gap',
    dom.el('tile-0-2').classList.contains('bounce'), true);
}

const { outfile, cleanup } = await bundle();
try {
  console.log('guess-tiles:');
  await testWildcardGuessShowsResolvedWord(outfile);
  await testMultiCodePointTilesStayAligned(outfile);
  await testBounceRowCoversWholeRow(outfile);
} finally {
  await cleanup();
}

if (failures > 0) {
  console.error(`\n${failures} test(s) failed`);
  process.exit(1);
}
console.log('\nall guess-tile tests passed');

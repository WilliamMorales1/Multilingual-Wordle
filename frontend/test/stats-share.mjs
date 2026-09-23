// Regression test for the stats modal's Share button. A finished game is
// shareable either way: buildShareText already scores a loss as X/6, but the
// button was hidden unless the game was won, so that branch could never be
// reached by a player.
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { installDOM, stubFetch } from './dom-stub.mjs';

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

const dir = await mkdtemp(path.join(tmpdir(), 'wordgo-stats-'));
const outfile = path.join(dir, 'entry.mjs');
await build({
  entryPoints: [path.join(here, 'entry.ts')],
  bundle: true,
  format: 'esm',
  outfile,
  logLevel: 'silent',
});

// The ids showStats writes into; the stub only hands back elements it knows.
const IDS = [
  'toast', 'stat-played', 'stat-win-pct', 'stat-streak', 'stat-max-streak',
  'distContainer', 'definition', 'defWord', 'defWiktionary', 'defText',
  'defEtymology', 'shareBtn', 'statsModal',
];

// shareButtonHidden runs showStats over a finished game and reports whether
// the Share button ended up hidden. Each case re-imports the bundle so the
// module's game state starts clean.
async function shareButtonHidden(status, history) {
  const dom = installDOM();
  IDS.forEach(dom.el);
  stubFetch({ games_played: 1, win_pct: 0, current_streak: 0, max_streak: 0, distribution: {} });

  const mod = await import(`${pathToFileURL(outfile).href}?${status}${history.length}`);
  Object.assign(mod.S, { status, history, lang: 'English', wordLength: 5, lastAttempt: history.length });
  await mod.showStats(null);
  return dom.el('shareBtn').hidden;
}

console.log('stats share button');
check('shown after a win',  await shareButtonHidden('won',  [['correct']]), false);
check('shown after a loss', await shareButtonHidden('lost', [['absent']]),  false);
check('hidden with no guesses to share', await shareButtonHidden('lost', []), true);
check('hidden mid-game', await shareButtonHidden('playing', [['absent']]), true);

await rm(dir, { recursive: true, force: true });
process.exit(failures === 0 ? 0 : 1);

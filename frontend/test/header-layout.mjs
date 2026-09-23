// Layout test for the header title block: adding the current-language line
// under "WORDGO" must not change the header's height, and no language name
// may be clipped.
//
// Needs a Chromium binary (chromium / chromium-browser / google-chrome).
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));
const page = path.join(here, 'header-layout.html');

const browser = [
  process.env.CHROME_BIN,
  '/usr/bin/chromium',
  '/usr/bin/chromium-browser',
  '/usr/bin/google-chrome-stable',
].find(p => p && existsSync(p));

if (!browser) {
  console.error('No Chromium binary found; set CHROME_BIN.');
  process.exit(1);
}

const decode = s =>
  s.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&amp;/g, '&');

function measure(width) {
  const dom = execFileSync(
    browser,
    [
      '--headless',
      '--disable-gpu',
      '--no-sandbox',
      '--virtual-time-budget=2000',
      '--allow-file-access-from-files',
      '--dump-dom',
      `file://${page}?width=${width}`,
    ],
    { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }
  );
  const match = dom.match(/<pre id="results">([\s\S]*?)<\/pre>/);
  if (!match) {
    console.error(`Test page produced no results at ${width}px.`);
    process.exit(1);
  }
  return JSON.parse(decode(match[1]));
}

// Desktop width and a narrow phone width.
const runs = [800, 380, 320].map(width => {
  const r = measure(width);
  if (Math.abs(r.viewport - width) > 2) {
    console.error(`Browser viewport was ${r.viewport}px, expected ${width}px.`);
    process.exit(1);
  }
  return { width, ...r };
});

const failures = [];
for (const { width, baselineHeight, results } of runs) {
for (const r of results) {
  const label = `${width}px / ${r.lang === '' ? '(no language)' : r.lang}`;
  if (Math.abs(r.headerHeight - baselineHeight) > 0.5) {
    failures.push(
      `${label}: header height ${r.headerHeight}px != baseline ${baselineHeight}px`
    );
  }
  if (r.clippedX) failures.push(`${label}: language text clipped horizontally`);
  if (r.clippedY) failures.push(`${label}: language text clipped vertically`);
  if (r.overflowsHeader) failures.push(`${label}: language text overflows the header`);
  if (!r.visible) {
    failures.push(
      r.lang === ''
        ? `${label}: empty language line still takes up space`
        : `${label}: language text not rendered`
    );
  }
}
}

for (const { width, baselineHeight, results } of runs) {
  console.log(`viewport ${width}px (baseline h1-only header: ${baselineHeight.toFixed(2)}px)`);
  for (const r of results) {
    console.log(
      `  ${(r.lang || '(no language)').padEnd(42)} header ${r.headerHeight.toFixed(2)}px`
    );
  }
}

if (failures.length) {
  console.error('\nFAIL');
  for (const f of failures) console.error('  - ' + f);
  process.exit(1);
}
console.log('\nPASS: header height unchanged, no clipped language names.');

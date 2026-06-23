import { S } from './state.js';

export function stripDiacritics(s: string): string {
  return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

// markDisplay prefixes a lone combining mark (e.g. a split-off matra tile)
// with a dotted circle so it renders visibly instead of vanishing.
export function markDisplay(ch: string): string {
  return /^\p{M}$/u.test(ch) ? '◌' + ch : ch.toUpperCase();
}

// gojuonTable lists each hiragana consonant cluster's vowel forms in
// a/i/u/e/o slot order ('' = no such combination, e.g. yi/ye). わ is built
// separately below — its flick directions hold ん/を/choon, not い/う/え.
const gojuonTable: string[][] = [
  ['あ', 'い', 'う', 'え', 'お'],
  ['か', 'き', 'く', 'け', 'こ'],
  ['さ', 'し', 'す', 'せ', 'そ'],
  ['た', 'ち', 'つ', 'て', 'と'],
  ['な', 'に', 'ぬ', 'ね', 'の'],
  ['は', 'ひ', 'ふ', 'へ', 'ほ'],
  ['ま', 'み', 'む', 'め', 'も'],
  ['や', '', 'ゆ', '', 'よ'],
  ['ら', 'り', 'る', 'れ', 'ろ'],
];

// わ's flick directions don't follow the regular vowel slots: up is ん
// (no い slot exists), left is を, right is the choon (long-vowel) mark,
// down is * (wildcard) when overflow kana exist.
const WA_CLUSTER_BASE = ['わ', 'を', 'ん', 'ー', ''];

// Flick direction -> vowel-slot index (center/no-flick = 'a', the 0th slot).
// up=u, right=e, down=o, left=i, per standard Japanese flick-input layout.
const FLICK_DIRS: { dx: number; dy: number; idx: number }[] = [
  { dx: 0, dy: -1, idx: 2 }, // up    -> u
  { dx: 1, dy: 0,  idx: 3 }, // right -> e
  { dx: 0, dy: 1,  idx: 4 }, // down  -> o
  { dx: -1, dy: 0, idx: 1 }, // left  -> i
];
const FLICK_THRESHOLD = 20; // px before a drag counts as a flick

function pickFlickChar(cluster: string[], dx: number, dy: number): string {
  if (Math.hypot(dx, dy) < FLICK_THRESHOLD) return cluster[0];
  let best = FLICK_DIRS[0];
  let bestDot = -Infinity;
  for (const dir of FLICK_DIRS) {
    const len = Math.hypot(dir.dx, dir.dy);
    const dot = (dx * dir.dx + dy * dir.dy) / len;
    if (dot > bestDot) { bestDot = dot; best = dir; }
  }
  return cluster[best.idx] || cluster[0];
}

// buildFlickKeyboard renders a phone-style flick keyboard: one key per
// gojuon consonant (tap = a-row kana, flick up/right/down/left = i/u/e/o),
// built only from the kana actually present in `rows` (the backend's
// per-vowel hiragana grid, flattened and reclassified into clusters here).
export function buildFlickKeyboard(rows: string[][] | null, overflowBases: Set<string>, onKey: (ch: string) => void, onEnter: () => void, onBack: () => void): void {
  const kb = document.getElementById('keyboard')!;
  kb.innerHTML = '';

  const present = new Set<string>();
  (rows ?? []).forEach(r => r.forEach(ch => present.add(ch)));

  const clusters = gojuonTable
    .map(cluster => cluster.map(ch => (present.has(ch) ? ch : '')))
    .filter(cluster => cluster.some(ch => ch !== ''));
  const hasWa = present.has('わ');

  if (clusters.length === 0 && !hasWa) {
    kb.innerHTML = '<div id="no-keyboard">Type your guess and press <strong>Enter</strong>.</div>';
    return;
  }

  kb.classList.add('flick');

  // Standard phone-flick layout: 3 columns x 3 rows, gojuon reading order
  // (あかさ / たなは / まやら), then a final row with Enter/わ/Backspace
  // flanking the わ key, like a classic Japanese feature-phone keypad.
  const allKeys = [...clusters];
  const waCluster = overflowBases.size > 0
    ? ['わ', 'を', 'ん', 'ー', '*']
    : WA_CLUSTER_BASE;

  // Only 3 keys per row (vs. ~10 for a normal keyboard), so each key can be
  // much wider — sized to comfortably fit the center glyph plus all four
  // edge labels without any overlap.
  const GAP = 6;
  const perRow = 3;
  const available = Math.min(300, window.innerWidth - 16) - 16;
  const keyW = Math.max(56, Math.min(72, Math.floor((available - (perRow - 1) * GAP) / perRow)));
  document.documentElement.style.setProperty('--flick-key-size', keyW + 'px');

  for (let i = 0; i < allKeys.length; i += perRow) {
    const rowEl = document.createElement('div');
    rowEl.className = 'key-row';
    for (const cluster of allKeys.slice(i, i + perRow)) {
      rowEl.appendChild(makeFlickKey(cluster, onKey));
    }
    kb.appendChild(rowEl);
  }

  // Enter and Backspace sit at the same size and level as the other flick
  // keys, directly flanking わ on the left and right.
  const lastRow = document.createElement('div');
  lastRow.className = 'key-row';
  const enterBtn = makeFlickActionKey('Enter', onEnter);
  enterBtn.id = 'enter-key';
  lastRow.appendChild(enterBtn);
  if (hasWa) lastRow.appendChild(makeFlickKey(waCluster, onKey));
  const backBtn = makeFlickActionKey('⌫', onBack);
  lastRow.appendChild(backBtn);
  kb.appendChild(lastRow);
}

function makeFlickActionKey(label: string, handler: () => void): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.className = 'key flick-key flick-action';
  btn.textContent = label;
  btn.addEventListener('pointerdown', e => { e.preventDefault(); handler(); });
  return btn;
}

// Edge position for each vowel-slot's static label, matching FLICK_DIRS
// (index 0 unused — center/a has no edge label).
const FLICK_EDGE_CLASS = ['', 'flick-left', 'flick-up', 'flick-right', 'flick-down'];

function makeFlickKey(cluster: string[], onKey: (ch: string) => void): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.className = 'key flick-key';
  btn.dataset['char'] = cluster[0];
  btn.dataset['cluster'] = cluster.join(',');

  const label = document.createElement('span');
  label.className = 'flick-main';
  label.textContent = cluster[0];
  btn.appendChild(label);

  // Always-visible edge labels showing which kana each flick direction yields.
  for (const dir of FLICK_DIRS) {
    const ch = cluster[dir.idx];
    if (!ch) continue;
    const edge = document.createElement('span');
    edge.className = `flick-edge ${FLICK_EDGE_CLASS[dir.idx]}`;
    edge.textContent = ch;
    btn.appendChild(edge);
  }

  let startX = 0, startY = 0, dragging = false;

  btn.addEventListener('pointerdown', e => {
    e.preventDefault();
    // Capture the pointer so move/up events keep reaching this key even
    // once the drag leaves its small bounds — without this, a flick that
    // crosses into a neighboring key never fires pointerup on this button,
    // leaving it stuck in its translated/highlighted "active" state.
    btn.setPointerCapture(e.pointerId);
    startX = e.clientX;
    startY = e.clientY;
    dragging = true;
    btn.classList.remove('flick-active-up', 'flick-active-right', 'flick-active-down', 'flick-active-left');
  });
  btn.addEventListener('pointermove', e => {
    if (!dragging) return;
    const dx = e.clientX - startX, dy = e.clientY - startY;
    btn.classList.remove('flick-active-up', 'flick-active-right', 'flick-active-down', 'flick-active-left');
    if (Math.hypot(dx, dy) >= FLICK_THRESHOLD) {
      const ch = pickFlickChar(cluster, dx, dy);
      const idx = cluster.indexOf(ch);
      const cls = FLICK_EDGE_CLASS[idx];
      if (cls) btn.classList.add('flick-active-' + cls.replace('flick-', ''));
    }
  });
  const release = (e: PointerEvent) => {
    if (!dragging) return;
    dragging = false;
    btn.classList.remove('flick-active-up', 'flick-active-right', 'flick-active-down', 'flick-active-left');
    const dx = e.clientX - startX, dy = e.clientY - startY;
    const ch = pickFlickChar(cluster, dx, dy);
    if (ch) onKey(ch);
  };
  btn.addEventListener('pointerup', release);
  btn.addEventListener('pointercancel', () => {
    dragging = false;
    btn.classList.remove('flick-active-up', 'flick-active-right', 'flick-active-down', 'flick-active-left');
  });

  return btn;
}

export function buildKeyboard(rows: string[][] | null, overflowBases: Set<string>, onKey: (ch: string) => void, onEnter: () => void, onBack: () => void): void {
  const kb = document.getElementById('keyboard')!;
  kb.innerHTML = '';
  kb.classList.remove('flick');

  if (!rows || rows.length === 0) {
    kb.innerHTML = '<div id="no-keyboard">Type your guess and press <strong>Enter</strong>.</div>';
    return;
  }

  const GAP = 5;
  const available = Math.min(500, window.innerWidth - 16) - 16;
  const hasOverflow = overflowBases.size > 0;

  const keyWPerRow = rows.map((rowChars, idx) => {
    const regular = rowChars.length + (idx === rows.length - 1 && hasOverflow ? 1 : 0);
    const wide = idx === rows.length - 1 ? 2 : 0;
    return Math.floor((available - (regular + wide - 1) * GAP) / (regular + 1.5 * wide));
  });
  const keyW     = Math.max(24, Math.min(52, Math.min(...keyWPerRow)));
  const wideKeyW = Math.max(52, Math.round(keyW * 1.5));
  document.documentElement.style.setProperty('--key-width',      keyW     + 'px');
  document.documentElement.style.setProperty('--key-wide-width', wideKeyW + 'px');

  rows.forEach((rowChars, idx) => {
    const rowEl = document.createElement('div');
    rowEl.className = 'key-row';

    for (const char of rowChars) {
      const btn = makeKey(markDisplay(char), '', () => onKey(char));
      btn.dataset['char'] = char;
      rowEl.appendChild(btn);
    }

    if (idx === rows.length - 1) {
      const enterBtn = makeKey('Enter', 'wide', onEnter);
      enterBtn.id = 'enter-key';
      rowEl.insertBefore(enterBtn, rowEl.firstChild);
      if (hasOverflow) {
        const starBtn = makeKey('*', '', () => onKey('*'));
        starBtn.id = 'star-key';
        rowEl.appendChild(starBtn);
      }
      rowEl.appendChild(makeKey('⌫', 'wide', onBack));
    }

    kb.appendChild(rowEl);
  });
}

function makeKey(label: string, extra: string, handler: () => void): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.className = `key ${extra}`.trim();
  btn.textContent = label;
  btn.addEventListener('pointerdown', e => { e.preventDefault(); handler(); });
  return btn;
}

// refreshVowelKeys swaps abugida vowel keys between their independent form
// (default, or after another vowel/matra) and combining (matra) form (right
// after a consonant), per the matra map returned by the server.
export function refreshVowelKeys(matraMap: Record<string, string>): void {
  const vowelSet = new Set(Object.keys(matraMap));
  if (vowelSet.size === 0) return;
  const matraSet = new Set(Object.values(matraMap));
  const prev = S.input[S.input.length - 1];
  const useMatra = prev !== undefined && !vowelSet.has(prev) && !matraSet.has(prev) && !/^\p{M}+$/u.test(prev);

  document.querySelectorAll<HTMLButtonElement>('.key[data-char]').forEach(btn => {
    const baseChar = btn.dataset['char']!;
    if (!vowelSet.has(baseChar)) return;
    btn.textContent = markDisplay(useMatra ? matraMap[baseChar] : baseChar);
  });
}

const STATE_PRI: Record<string, number> = { correct: 3, present: 2, absent: 1 };

export function refreshKeyboard(): void {
  document.querySelectorAll<HTMLButtonElement>('.key[data-char]').forEach(btn => {
    const cluster = btn.dataset['cluster'] ? btn.dataset['cluster']!.split(',').filter(Boolean) : [btn.dataset['char']!];
    let best = '';
    for (const ch of cluster) {
      const st = S.charStates[stripDiacritics(ch)];
      if (st && (STATE_PRI[st] ?? 0) > (STATE_PRI[best] ?? 0)) best = st;
    }
    btn.className = 'key' + (btn.classList.contains('flick-key') ? ' flick-key' : '') + (best ? ` ${best}` : '');
  });
}

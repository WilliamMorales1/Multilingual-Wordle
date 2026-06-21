import { S } from './state.js';

export function stripDiacritics(s: string): string {
  return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

// markDisplay prefixes a lone combining mark (e.g. a split-off matra tile)
// with a dotted circle so it renders visibly instead of vanishing.
export function markDisplay(ch: string): string {
  return /^\p{M}$/u.test(ch) ? '◌' + ch : ch.toUpperCase();
}

export function buildKeyboard(rows: string[][] | null, overflowBases: Set<string>, onKey: (ch: string) => void, onEnter: () => void, onBack: () => void): void {
  const kb = document.getElementById('keyboard')!;
  kb.innerHTML = '';

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

export function refreshKeyboard(): void {
  document.querySelectorAll<HTMLButtonElement>('.key[data-char]').forEach(btn => {
    const baseKey = stripDiacritics(btn.dataset['char']!);
    const st = S.charStates[baseKey];
    btn.className = 'key' + (st ? ` ${st}` : '');
  });
}

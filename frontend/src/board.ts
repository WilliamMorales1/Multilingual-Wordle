import { S } from './state.js';
import { markDisplay } from './keyboard.js';

export function tileSize(wordLen: number): number {
  return Math.min(62, Math.max(28, Math.floor(310 / wordLen)));
}

// INITIAL_ROWS is how many guess rows are shown up front. Guessing is
// unlimited, so ensureRow appends more on demand once these run out.
const INITIAL_ROWS = 6;

function appendRow(board: HTMLElement, r: number): void {
  const wrap = document.createElement('div');
  wrap.className = 'board-row-wrap';

  const row = document.createElement('div');
  row.className = 'board-row';
  row.id = `row-${r}`;
  row.style.gridTemplateColumns = `repeat(${S.wordLength}, 1fr)`;
  if (S.rtl) row.dir = 'rtl';
  for (let c = 0; c < S.wordLength; c++) {
    const tile = document.createElement('div');
    tile.className = 'tile';
    tile.id = `tile-${r}-${c}`;
    row.appendChild(tile);
  }
  wrap.appendChild(row);

  const caption = document.createElement('div');
  caption.className = 'row-caption';
  caption.id = `caption-${r}`;
  wrap.appendChild(caption);

  board.appendChild(wrap);
}

export function buildBoard(): void {
  const board = document.getElementById('board')!;
  const sz = tileSize(S.wordLength);
  board.innerHTML = '';
  document.documentElement.style.setProperty('--tile-size', sz + 'px');

  for (let r = 0; r < INITIAL_ROWS; r++) appendRow(board, r);
}

// ensureRow appends a new board row if rowIdx isn't built yet — guessing has
// no cap, so the board grows past its initial rows as needed.
export function ensureRow(rowIdx: number): void {
  if (document.getElementById(`row-${rowIdx}`)) return;
  appendRow(document.getElementById('board')!, rowIdx);
}

// setRowCaption shows romanized words' original characters (e.g. Chinese
// hanzi) under a guess row, since the tiles themselves hold the romanization.
export function setRowCaption(rowIdx: number, chars: string | undefined): void {
  const el = document.getElementById(`caption-${rowIdx}`);
  if (!el) return;
  el.textContent = chars ?? '';
}

export function setTileText(row: number, col: number, ch: string): void {
  const t = document.getElementById(`tile-${row}-${col}`);
  if (!t) return;
  t.textContent = ch ? markDisplay(ch) : '';
  if (ch) {
    t.classList.add('filled');
    t.classList.remove('pop');
    void t.offsetWidth;
    t.classList.add('pop');
  } else {
    t.classList.remove('filled', 'pop');
  }
}

export function updateCurrentRow(): void {
  ensureRow(S.currentRow);
  for (let c = 0; c < S.wordLength; c++) {
    setTileText(S.currentRow, c, S.input[c] ?? '');
  }
}

export function revealRow(rowIdx: number, chars: string[], states: string[], onDone: () => void): void {
  const FLIP = 400;
  chars.forEach((ch, i) => {
    const t = document.getElementById(`tile-${rowIdx}-${i}`);
    if (!t) return;
    setTimeout(() => {
      t.classList.add('flipping');
      setTimeout(() => {
        t.textContent = markDisplay(ch);
        t.className = `tile ${states[i]} filled`;
      }, FLIP / 2);
    }, i * FLIP);
  });
  setTimeout(onDone, chars.length * FLIP + FLIP);
}

export function bounceRow(rowIdx: number): void {
  for (let c = 0; c < S.wordLength; c++) {
    const t = document.getElementById(`tile-${rowIdx}-${c}`);
    if (!t) return;
    setTimeout(() => {
      t.classList.add('bounce');
      t.addEventListener('animationend', () => t.classList.remove('bounce'), { once: true });
    }, c * 80);
  }
}

export function shakeRow(rowIdx: number): void {
  const row = document.getElementById(`row-${rowIdx}`);
  if (!row) return;
  row.classList.add('shake');
  row.addEventListener('animationend', () => row.classList.remove('shake'), { once: true });
}

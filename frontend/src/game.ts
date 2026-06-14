import { S } from './state.js';
import { api } from './api.js';
import { buildBoard, updateCurrentRow, revealRow, bounceRow, shakeRow } from './board.js';
import { buildKeyboard, refreshKeyboard, stripDiacritics } from './keyboard.js';
import { toast, openModal, closeModal, showEquivNotice, showStats } from './ui.js';

let _progressTimer: ReturnType<typeof setInterval> | null = null;

function startProgressPolling(lang: string, length: number): void {
  const el = document.getElementById('loading-count');
  if (el) el.textContent = '';
  _progressTimer = setInterval(async () => {
    try {
      const { count } = await api.progress(lang, length);
      if (el && count > 0) el.textContent = `${count.toLocaleString()} words found so far…`;
    } catch (_) {}
  }, 60000);
}

function stopProgressPolling(): void {
  if (_progressTimer !== null) clearInterval(_progressTimer);
  _progressTimer = null;
  const el = document.getElementById('loading-count');
  if (el) el.textContent = '';
}

export function onKeyPress(ch: string): void {
  if (S.status !== 'playing') return;
  if (S.input.length >= S.wordLength) return;
  S.input.push(ch);
  updateCurrentRow();
}

export function onBackspace(): void {
  if (S.status !== 'playing') return;
  S.input.pop();
  updateCurrentRow();
}

export async function onEnter(): Promise<void> {
  if (S.status !== 'playing') return;
  if (S.input.length !== S.wordLength) {
    shakeRow(S.currentRow);
    toast(`Enter a ${S.wordLength}-character word`);
    return;
  }

  const word = S.input.join('');
  S.status = 'submitting';

  let result;
  try {
    result = await api.guess(S.gameId!, word);
  } catch (_) {
    toast('Network error — please try again');
    S.status = 'playing';
    return;
  }

  if (result.error) {
    toast(result.error);
    S.status = 'playing';
    shakeRow(S.currentRow);
    return;
  }

  const rowIdx = S.currentRow;
  const chars  = [...word];
  const states = result.states;

  const PRI: Record<string, number> = { correct: 3, present: 2, absent: 1 };
  chars.forEach((ch, i) => {
    const baseKey = stripDiacritics(ch);
    const oldSt = S.charStates[baseKey];
    const newSt = states[i];
    if (!oldSt || (PRI[newSt] ?? 0) > (PRI[oldSt] ?? 0)) {
      S.charStates[baseKey] = newSt;
    }
  });

  for (let c = 0; c < S.wordLength; c++) {
    const t = document.getElementById(`tile-${rowIdx}-${c}`);
    if (t) t.textContent = (chars[c] ?? '').toUpperCase();
  }

  revealRow(rowIdx, chars, states, () => {
    refreshKeyboard();
    S.currentRow++;
    S.input = [];
    S.status = 'playing';

    if (result.status === 'won') {
      S.status = 'won';
      S.lastAttempt = result.attempt;
      bounceRow(rowIdx);
      const msgs = ['Genius!', 'Magnificent!', 'Impressive!', 'Splendid!', 'Great!', 'Phew!'];
      toast(msgs[Math.min(result.attempt - 1, msgs.length - 1)], 0);
      setTimeout(() => showStats(result), 2000);
    } else if (result.status === 'lost') {
      S.status = 'lost';
      toast(result.answer!.toUpperCase(), 0);
      setTimeout(() => showStats(result), 2500);
    }
  });
}

export async function startGame(): Promise<void> {
  const lang       = (document.getElementById('langInput') as HTMLInputElement).value.trim() || 'English';
  const length     = parseInt((document.getElementById('lengthInput') as HTMLInputElement).value) || 5;
  const maxGuesses = parseInt((document.getElementById('guessesInput') as HTMLInputElement).value) || 6;

  closeModal('settingsModal');

  Object.assign(S, { lang, wordLength: length, maxGuesses, status: 'loading', currentRow: 0, input: [], charStates: {}, gameId: null });

  document.getElementById('loading')!.style.display  = 'flex';
  document.getElementById('board')!.style.display    = 'none';
  document.getElementById('keyboard')!.style.display = 'none';

  startProgressPolling(lang, length);

  let result;
  try {
    result = await api.newGame({ lang, length, max_guesses: maxGuesses });
  } catch (_) {
    stopProgressPolling();
    toast('Network error — could not start game');
    S.status = 'idle';
    document.getElementById('loading')!.style.display = 'none';
    openModal('settingsModal');
    return;
  }

  stopProgressPolling();
  document.getElementById('loading')!.style.display = 'none';

  if (result.error) {
    toast(result.error, 5000);
    S.status = 'idle';
    openModal('settingsModal');
    return;
  }

  S.gameId = result.id;
  S.status = 'playing';
  S.rtl    = result.rtl ?? false;

  document.getElementById('board')!.style.display    = '';
  document.getElementById('keyboard')!.style.display = '';

  buildBoard();
  buildKeyboard(result.keyboard_rows ?? null, new Set(result.overflow_bases ?? []), onKeyPress, onEnter, onBackspace);
  showEquivNotice(result.equivalences ?? []);
}

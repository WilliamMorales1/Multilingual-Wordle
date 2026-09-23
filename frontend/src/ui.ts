import { S } from './state.js';
import { api } from './api.js';
import type { GuessResult, StatsResult } from './types.js';
import { composeHangul, isHangulLang } from './hangul.js';

let toastTimer: ReturnType<typeof setTimeout> | null = null;

export function toast(msg: string, duration = 1800): void {
  const el = document.getElementById('toast')!;
  el.textContent = msg;
  el.classList.add('show');
  if (toastTimer !== null) clearTimeout(toastTimer);
  if (duration > 0) toastTimer = setTimeout(() => el.classList.remove('show'), duration);
}

export function clearToast(): void {
  if (toastTimer !== null) clearTimeout(toastTimer);
  document.getElementById('toast')!.classList.remove('show');
}

export function openModal(id: string):  void { document.getElementById(id)!.classList.add('open'); }
export function closeModal(id: string): void { document.getElementById(id)!.classList.remove('open'); }

export function showEquivNotice(equivalences: string[][]): void {
  const notice = document.getElementById('equiv-notice')!;
  const list   = document.getElementById('equiv-list')!;

  if (!equivalences || equivalences.length === 0) { notice.hidden = true; return; }

  list.innerHTML = equivalences.map(g => {
    const base     = g[0].toUpperCase();
    const variants = g.slice(1).map(c => c.toUpperCase()).join(', ');
    return `<span class="equiv-group"><strong>${base}</strong> = ${variants}</span>`;
  }).join(' &middot; ');

  notice.hidden = false;
}

const STATE_EMOJI: Record<string, string> = { correct: '🟩', present: '🟨', absent: '⬛' };

// Mirrors the backend's maxGuesses (api/handlers.go).
const MAX_GUESSES = 6;

function buildShareText(): string {
  // Wordle-style score: attempts out of the six a game allows, or X when the
  // game was lost.
  const n     = S.status === 'won' ? `${S.history.length}/${MAX_GUESSES}` : `X/${MAX_GUESSES}`;
  const grid  = S.history.map(row => row.map(st => STATE_EMOJI[st] ?? '⬛').join('')).join('\n');
  return `Wordgo — ${S.lang} (${S.wordLength}) ${n}\n\n${grid}`;
}

export async function shareResult(): Promise<void> {
  const text = buildShareText();
  try {
    if (navigator.share) {
      await navigator.share({ text });
      return;
    }
  } catch (_) { return; }
  try {
    await navigator.clipboard.writeText(text);
    toast('Copied results to clipboard');
  } catch (_) {
    toast('Could not copy results');
  }
}

export async function showStats(lastResult: Partial<GuessResult> | null): Promise<void> {
  clearToast();

  let data: Partial<StatsResult> = {};
  try { data = await api.stats(S.lang, S.wordLength); } catch (_) {}

  document.getElementById('stat-played')!.textContent     = String(data.games_played   ?? 0);
  document.getElementById('stat-win-pct')!.textContent    = String(data.win_pct        ?? 0);
  document.getElementById('stat-streak')!.textContent     = String(data.current_streak ?? 0);
  document.getElementById('stat-max-streak')!.textContent = String(data.max_streak     ?? 0);

  const dist = data.distribution ?? {};
  const container = document.getElementById('distContainer')!;
  container.innerHTML = '';
  const maxCount = Math.max(...Object.values(dist).map(Number), 1);
  const maxRow = Math.max(...Object.keys(dist).map(Number), S.lastAttempt, 6);

  for (let i = 1; i <= maxRow; i++) {
    const count = dist[i] ?? 0;
    const pct   = Math.max(7, Math.round(count / maxCount * 100));
    const highlight = (S.status === 'won' && i === S.lastAttempt) ? ' highlight' : '';
    const bar = document.createElement('div');
    bar.className = 'dist-bar';
    bar.innerHTML = `<span class="dist-label">${i}</span><div class="dist-fill${highlight}" style="width:${pct}%">${count}</div>`;
    container.appendChild(bar);
  }

  const defEl = document.getElementById('definition')!;
  if (lastResult?.answer) {
    const word = lastResult.answer.toUpperCase();
    document.getElementById('defWord')!.textContent = lastResult.answer_chars
      ? `${word} (${lastResult.answer_chars})`
      : word;

    const wiktTerm = lastResult.answer_chars ||
      (isHangulLang(S.lang) ? composeHangul(lastResult.answer) : lastResult.answer);
    const wiktLangSection = S.lang.replace(/\s*\(.*\)\s*$/, '');
    const wiktLink = document.getElementById('defWiktionary') as HTMLAnchorElement;
    wiktLink.href = `https://en.wiktionary.org/wiki/${encodeURIComponent(wiktTerm)}#${encodeURIComponent(wiktLangSection)}`;
    wiktLink.style.display = 'inline';

    const defs = lastResult.definitions?.length ? lastResult.definitions : ['(no definition available)'];
    const defText = document.getElementById('defText')!;
    defText.textContent = '';
    defs.forEach((d, i) => {
      const line = document.createElement('div');
      line.textContent = defs.length > 1 ? `${i + 1}. ${d}` : d;
      defText.appendChild(line);
    });
    const etyEl = document.getElementById('defEtymology')!;
    if (lastResult.etymology) {
      etyEl.textContent = `Etymology: ${lastResult.etymology}`;
      etyEl.style.display = 'block';
    } else {
      etyEl.style.display = 'none';
    }
    defEl.style.display = 'block';
  } else {
    defEl.style.display = 'none';
  }

  // A finished game is shareable either way — buildShareText already scores a
  // loss as X/6, and hiding the button on a loss made that branch unreachable.
  const shareBtn = document.getElementById('shareBtn')!;
  shareBtn.hidden = S.history.length === 0 || (S.status !== 'won' && S.status !== 'lost');

  openModal('statsModal');
}

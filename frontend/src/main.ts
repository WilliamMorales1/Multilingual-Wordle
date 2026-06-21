import { S } from './state.js';
import { api } from './api.js';

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(err => console.error('SW registration failed:', err));
}
import { startGame, onEnter, onBackspace, onKeyPress } from './game.js';
import { openModal, closeModal, showStats, toast, shareResult } from './ui.js';

document.addEventListener('keydown', (e: KeyboardEvent) => {
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  const tag = (document.activeElement as HTMLElement | null)?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA') return;

  if (e.key === 'Enter')          onEnter();
  else if (e.key === 'Backspace') onBackspace();
  else if (e.key.length === 1)    onKeyPress(e.key.toLowerCase());
});

document.getElementById('startBtn')!.addEventListener('click', startGame);
document.getElementById('settingsBtn')!.addEventListener('click', () => openModal('settingsModal'));
document.getElementById('statsBtn')!.addEventListener('click', () => showStats(null));
document.getElementById('closeStats')!.addEventListener('click', () => closeModal('statsModal'));
document.getElementById('shareBtn')!.addEventListener('click', shareResult);
document.getElementById('newGameFromStats')!.addEventListener('click', () => {
  closeModal('statsModal');
  openModal('settingsModal');
});
document.getElementById('equiv-close')!.addEventListener('click', () => {
  document.getElementById('equiv-notice')!.hidden = true;
});
document.getElementById('clearCacheBtn')!.addEventListener('click', async () => {
  try {
    await api.clearCache(S.status === 'playing' ? S.gameId : null);
    toast('Cache cleared');
  } catch (_) {
    toast('Failed to clear cache');
  }
});

document.querySelectorAll<HTMLElement>('.modal').forEach(m => {
  m.addEventListener('click', e => {
    if (e.target === m) {
      if (m.id === 'settingsModal' && S.status === 'idle') return;
      closeModal(m.id);
    }
  });
});

(async function initLangDropdown() {
  const input   = document.getElementById('langInput') as HTMLInputElement | null;
  const options = document.getElementById('langOptions') as HTMLElement | null;
  if (!input || !options) return;

  let allLangs: string[] = [];
  let activeIdx = -1;

  try {
    const data = await api.languages();
    allLangs = (data.languages ?? []).filter(l => l !== 'Chinese');
  } catch (_) {}

  function render(filter: string): void {
    const q = filter.trim().toLowerCase();
    const matches = q ? allLangs.filter(l => l.toLowerCase().includes(q)) : allLangs;
    options!.innerHTML = '';
    activeIdx = -1;
    matches.forEach(lang => {
      const div = document.createElement('div');
      div.className = 'lang-option';
      div.textContent = lang;
      div.addEventListener('mousedown', e => { e.preventDefault(); select(lang); });
      options!.appendChild(div);
    });
    options!.hidden = matches.length === 0;
  }

  function select(lang: string): void {
    input!.value = lang;
    options!.hidden = true;
    activeIdx = -1;
    const lengthInput = document.getElementById('lengthInput') as HTMLInputElement | null;
    if (lengthInput) lengthInput.value = '6';
  }

  function setActive(idx: number): void {
    const opts = options!.querySelectorAll<HTMLElement>('.lang-option');
    opts.forEach(o => o.classList.remove('active'));
    activeIdx = Math.max(0, Math.min(idx, opts.length - 1));
    opts[activeIdx]?.classList.add('active');
    opts[activeIdx]?.scrollIntoView({ block: 'nearest' });
  }

  input.addEventListener('focus', () => render(input.value));
  input.addEventListener('input', () => render(input.value));
  input.addEventListener('blur',  () => setTimeout(() => { options.hidden = true; }, 150));
  input.addEventListener('keydown', e => {
    const opts = options.querySelectorAll<HTMLElement>('.lang-option');
    if (options.hidden || opts.length === 0) return;
    if      (e.key === 'ArrowDown')               { e.preventDefault(); setActive(activeIdx + 1); }
    else if (e.key === 'ArrowUp')                 { e.preventDefault(); setActive(activeIdx - 1); }
    else if (e.key === 'Enter' && activeIdx >= 0) { e.preventDefault(); select(opts[activeIdx].textContent!); }
    else if (e.key === 'Escape')                  { options.hidden = true; }
  });
})();

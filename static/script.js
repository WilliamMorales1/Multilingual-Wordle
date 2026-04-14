function stripDiacritics(s) {
  return s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
}

// equivalences is the precomputed [][]string from the server:
// each element is [base/label, variant1, variant2, ...]
function showEquivNotice(equivalences) {
  const notice = document.getElementById('equiv-notice');
  const list   = document.getElementById('equiv-list');

  if (!equivalences || equivalences.length === 0) { notice.hidden = true; return; }

  list.innerHTML = equivalences.map(g => {
    const base     = g[0].toUpperCase();
    const variants = g.slice(1).map(c => c.toUpperCase()).join(', ');
    return `<span class="equiv-group"><strong>${base}</strong> = ${variants}</span>`;
  }).join(' &middot; ');

  notice.hidden = false;
}

// ── State ────────────────────────────────────────────────────────────────────
const S = {
  gameId:       null,
  wordLength:   5,
  maxGuesses:   6,
  lang:         'English',
  status:       'idle',   // idle | loading | playing | won | lost
  currentRow:   0,
  input:        [],       // []string  — current typing buffer (grapheme units)
  charStates:   {},       // char → 'correct' | 'present' | 'absent'
  lastAttempt:  0,        // winning attempt number (for dist highlight)
  rtl:          false,    // true for Arabic, Hebrew, etc.
};

// ── API ──────────────────────────────────────────────────────────────────────
async function apiFetch(path, opts = {}) {
  const method = opts.method || 'GET';
  console.log(`[api] ${method} ${path}`, opts.body ? JSON.parse(opts.body) : '');
  const t0 = performance.now();
  let r;
  try {
    r = await fetch(path, {
      headers: { 'Content-Type': 'application/json' },
      ...opts,
    });
  } catch (e) {
    console.error(`[api] ${method} ${path} — network error after ${((performance.now()-t0)/1000).toFixed(1)}s:`, e);
    throw e;
  }
  const ms = ((performance.now() - t0) / 1000).toFixed(1);
  console.log(`[api] ${method} ${path} → HTTP ${r.status} (${ms}s)`);
  if (!r.ok) {
    console.error(`[api] HTTP ${r.status} body:`, await r.clone().text());
  }
  return r.json();
}

const api = {
  languages: ()            => apiFetch('/api/languages'),
  newGame:   (b)           => apiFetch('/api/game', { method: 'POST', body: JSON.stringify(b) }),
  guess:     (id, word)    => apiFetch(`/api/game/${id}/guess`, { method: 'POST', body: JSON.stringify({ word }) }),
  stats:     (lang, len)   => apiFetch(`/api/stats?lang=${encodeURIComponent(lang)}&length=${len}`),
  progress:  (lang, len)   => apiFetch(`/api/progress?lang=${encodeURIComponent(lang)}&length=${len}`),
};

// ── Download progress polling ─────────────────────────────────────────────────
let _progressTimer = null;
function startProgressPolling(lang, length) {
  const el = document.getElementById('loading-count');
  if (el) el.textContent = '';
  _progressTimer = setInterval(async () => {
    try {
      const { count } = await api.progress(lang, length);
      if (el && count > 0) el.textContent = `${count.toLocaleString()} words found so far…`;
    } catch (_) {}
  }, 60000);
}
function stopProgressPolling() {
  clearInterval(_progressTimer);
  _progressTimer = null;
  const el = document.getElementById('loading-count');
  if (el) el.textContent = '';
}

// ── Toast ────────────────────────────────────────────────────────────────────
let toastTimer = null;
function toast(msg, duration = 1800) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.classList.add('show');
  clearTimeout(toastTimer);
  if (duration > 0) toastTimer = setTimeout(() => el.classList.remove('show'), duration);
}

// ── Modal helpers ─────────────────────────────────────────────────────────────
function openModal(id)  { document.getElementById(id).classList.add('open'); }
function closeModal(id) { document.getElementById(id).classList.remove('open'); }

// ── Board ─────────────────────────────────────────────────────────────────────
function tileSize(wordLen) {
  // Scale tile size down for longer words so they fit on screen
  return Math.min(62, Math.max(28, Math.floor(310 / wordLen)));
}

function buildBoard() {
  const board = document.getElementById('board');
  const sz = tileSize(S.wordLength);
  board.innerHTML = '';
  board.style.gridTemplateRows = `repeat(${S.maxGuesses}, 1fr)`;

  document.documentElement.style.setProperty('--tile-size', sz + 'px');

  for (let r = 0; r < S.maxGuesses; r++) {
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
    board.appendChild(row);
  }
}

function setTileText(row, col, ch) {
  const t = document.getElementById(`tile-${row}-${col}`);
  if (!t) return;
  t.textContent = ch ? ch.toUpperCase() : '';
  if (ch) {
    t.classList.add('filled');
    t.classList.remove('pop');
    void t.offsetWidth; // reflow
    t.classList.add('pop');
  } else {
    t.classList.remove('filled', 'pop');
  }
}

function updateCurrentRow() {
  for (let c = 0; c < S.wordLength; c++) {
    setTileText(S.currentRow, c, S.input[c] || '');
  }
}

// Reveal a row with a staggered flip animation then apply state classes.
function revealRow(rowIdx, chars, states, onDone) {
  const FLIP = 400; // ms per tile

  chars.forEach((ch, i) => {
    const t = document.getElementById(`tile-${rowIdx}-${i}`);
    if (!t) return;

    setTimeout(() => {
      t.classList.add('flipping');
      // At the midpoint of the flip, swap the colour
      setTimeout(() => {
        t.textContent = ch.toUpperCase();
        t.className = `tile ${states[i]} filled`;
      }, FLIP / 2);
    }, i * FLIP);
  });

  const total = chars.length * FLIP + FLIP;
  setTimeout(onDone, total);
}

function bounceRow(rowIdx) {
  for (let c = 0; c < S.wordLength; c++) {
    const t = document.getElementById(`tile-${rowIdx}-${c}`);
    if (!t) return;
    setTimeout(() => {
      t.classList.add('bounce');
      t.addEventListener('animationend', () => t.classList.remove('bounce'), { once: true });
    }, c * 80);
  }
}

function shakeRow(rowIdx) {
  const row = document.getElementById(`row-${rowIdx}`);
  if (!row) return;
  row.classList.add('shake');
  row.addEventListener('animationend', () => row.classList.remove('shake'), { once: true });
}

// ── Keyboard ──────────────────────────────────────────────────────────────────
// rows: [][]string from server (pre-computed base chars per row)
// overflowBases: Set of base chars not on any layout key (for the '*' key)
function buildKeyboard(rows, overflowBases) {
  const kb = document.getElementById('keyboard');
  kb.innerHTML = '';

  if (!rows || rows.length === 0) {
    kb.innerHTML = '<div id="no-keyboard">Type your guess and press <strong>Enter</strong>.</div>';
    return;
  }

  const GAP = 5;
  const available = Math.min(500, window.innerWidth - 16) - 16;
  const hasOverflow = overflowBases.size > 0;

  // For each row, solve for the keyW that exactly fills available width.
  // Wide keys (⌫, Enter) count as 1.5 regular-key units.
  // Last row gets Enter (wide, left) + letters + optionally * (regular) + ⌫ (wide, right).
  const keyWPerRow = rows.map((rowChars, idx) => {
    let regular = rowChars.length;
    let wide = 0;
    if (idx === rows.length - 1) { wide += 2; if (hasOverflow) regular += 1; }
    const totalKeys = regular + wide;
    const units     = regular + 1.5 * wide;
    return Math.floor((available - (totalKeys - 1) * GAP) / units);
  });
  const keyW     = Math.max(24, Math.min(52, Math.min(...keyWPerRow)));
  const wideKeyW = Math.max(52, Math.round(keyW * 1.5));
  document.documentElement.style.setProperty('--key-width',      keyW     + 'px');
  document.documentElement.style.setProperty('--key-wide-width', wideKeyW + 'px');

  rows.forEach((rowChars, idx) => {
    const rowEl = newKeyRow();

    for (const char of rowChars) {
      const btn = document.createElement('button');
      btn.className = 'key';
      btn.textContent = char.toUpperCase();
      btn.dataset.char = char;
      btn.addEventListener('pointerdown', e => { e.preventDefault(); onKeyPress(char); });
      rowEl.appendChild(btn);
    }

    // Enter left of bottom row, ⌫ right of bottom row (like a physical keyboard's Z/M flanks)
    if (idx === rows.length - 1) {
      const enterBtn = makeKey('Enter', 'wide', onEnter);
      enterBtn.id = 'enter-key';
      rowEl.insertBefore(enterBtn, rowEl.firstChild);
      if (overflowBases.size > 0) {
        const starBtn = makeKey('*', '', () => onKeyPress('*'));
        starBtn.id = 'star-key';
        rowEl.appendChild(starBtn);
      }
      rowEl.appendChild(makeKey('⌫', 'wide', onBackspace));
    }

    kb.appendChild(rowEl);
  });
}

function newKeyRow() {
  const r = document.createElement('div');
  r.className = 'key-row';
  return r;
}

function makeKey(label, extra, handler) {
  const btn = document.createElement('button');
  btn.className = `key ${extra}`;
  btn.textContent = label;
  btn.addEventListener('pointerdown', (e) => { e.preventDefault(); handler(); });
  return btn;
}

function refreshKeyboard() {
  document.querySelectorAll('.key[data-char]').forEach(btn => {
    // Normalize so é/ê/e all share the same state slot
    const baseKey = stripDiacritics(btn.dataset.char);
    const st = S.charStates[baseKey];
    btn.className = 'key' + (st ? ` ${st}` : '');
  });
}

// ── Input handlers ────────────────────────────────────────────────────────────
function onKeyPress(ch) {
  if (S.status !== 'playing') return;
  if (S.input.length >= S.wordLength) return;
  S.input.push(ch);
  updateCurrentRow();
}

function onBackspace() {
  if (S.status !== 'playing') return;
  if (S.input.length === 0) return;
  S.input.pop();
  updateCurrentRow();
}

async function onEnter() {
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
    result = await api.guess(S.gameId, word);
  } catch (e) {
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
  const chars  = [...word];        // spread by code point (good enough for display)
  const states = result.states;

  // Update char priority map — key on the base (diacritic-stripped) character
  // so é, e, ê etc. all update the same keyboard slot.
  const PRI = { correct: 3, present: 2, absent: 1 };
  chars.forEach((ch, i) => {
    const newSt  = states[i];
    const baseKey = stripDiacritics(ch);
    const oldSt  = S.charStates[baseKey];
    if (!oldSt || (PRI[newSt] || 0) > (PRI[oldSt] || 0)) {
      S.charStates[baseKey] = newSt;
    }
  });

  // Clear the input row before flipping (show blank during animation)
  for (let c = 0; c < S.wordLength; c++) {
    const t = document.getElementById(`tile-${rowIdx}-${c}`);
    if (t) { t.textContent = chars[c] ? chars[c].toUpperCase() : ''; }
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
      const msg  = msgs[Math.min(result.attempt - 1, msgs.length - 1)];
      toast(msg, 0);
      setTimeout(() => showStats(result), 2000);

    } else if (result.status === 'lost') {
      S.status = 'lost';
      toast(result.answer.toUpperCase(), 0);
      setTimeout(() => showStats(result), 2500);
    }
  });
}

// Physical keyboard
document.addEventListener('keydown', e => {
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  // Don't intercept when a modal or input is focused
  const tag = document.activeElement?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA') return;

  if (e.key === 'Enter')     { onEnter(); }
  else if (e.key === 'Backspace') { onBackspace(); }
  else if (e.key.length === 1) { onKeyPress(e.key.toLowerCase()); }
});

// ── Game flow ─────────────────────────────────────────────────────────────────
async function startGame() {
  const lang       = document.getElementById('langInput').value.trim()  || 'English';
  const length     = parseInt(document.getElementById('lengthInput').value)  || 5;
  const maxGuesses = parseInt(document.getElementById('guessesInput').value) || 6;

  closeModal('settingsModal');

  S.lang       = lang;
  S.wordLength = length;
  S.maxGuesses = maxGuesses;
  S.status     = 'loading';
  S.currentRow = 0;
  S.input      = [];
  S.charStates = {};
  S.gameId     = null;

  document.getElementById('loading').style.display = 'flex';
  document.getElementById('board').style.display   = 'none';
  document.getElementById('keyboard').style.display = 'none';

  startProgressPolling(lang, length);

  let result;
  try {
    result = await api.newGame({ lang, length, max_guesses: maxGuesses });
  } catch (e) {
    stopProgressPolling();
    toast('Network error — could not start game');
    S.status = 'idle';
    document.getElementById('loading').style.display = 'none';
    openModal('settingsModal');
    return;
  }

  stopProgressPolling();
  document.getElementById('loading').style.display = 'none';

  if (result.error) {
    toast(result.error, 5000);
    S.status = 'idle';
    openModal('settingsModal');
    return;
  }

  S.gameId  = result.id;
  S.status  = 'playing';
  S.rtl     = result.rtl || false;

  document.getElementById('board').style.display   = '';
  document.getElementById('keyboard').style.display = '';

  buildBoard();
  buildKeyboard(result.keyboard_rows || null, new Set(result.overflow_bases || []));
  showEquivNotice(result.equivalences || []);
}

// ── Stats ─────────────────────────────────────────────────────────────────────
async function showStats(lastResult) {
  clearTimeout(toastTimer);
  document.getElementById('toast').classList.remove('show');
  let data;
  try {
    data = await api.stats(S.lang, S.wordLength);
  } catch(e) {
    data = {};
  }

  document.getElementById('stat-played').textContent    = data.games_played    ?? 0;
  document.getElementById('stat-win-pct').textContent   = data.win_pct         ?? 0;
  document.getElementById('stat-streak').textContent    = data.current_streak  ?? 0;
  document.getElementById('stat-max-streak').textContent= data.max_streak      ?? 0;

  const dist = data.distribution || {};
  const container = document.getElementById('distContainer');
  container.innerHTML = '';
  const maxCount = Math.max(...Object.values(dist), 1);

  for (let i = 1; i <= S.maxGuesses; i++) {
    const count = dist[i] || 0;
    const pct   = Math.max(7, Math.round(count / maxCount * 100));
    const bar   = document.createElement('div');
    bar.className = 'dist-bar';
    const highlight = (S.status === 'won' && i === S.lastAttempt) ? ' highlight' : '';
    bar.innerHTML = `
      <span class="dist-label">${i}</span>
      <div class="dist-fill${highlight}" style="width:${pct}%">${count}</div>`;
    container.appendChild(bar);
  }

  const defEl = document.getElementById('definition');
  if (lastResult?.answer) {
    document.getElementById('defWord').textContent = lastResult.answer.toUpperCase();
    document.getElementById('defText').textContent = lastResult.definition || '(no definition available)';
    defEl.style.display = 'block';
  } else {
    defEl.style.display = 'none';
  }

  openModal('statsModal');
}

// ── Wiring ────────────────────────────────────────────────────────────────────
document.getElementById('startBtn').addEventListener('click', startGame);

document.getElementById('settingsBtn').addEventListener('click', () => openModal('settingsModal'));

document.getElementById('statsBtn').addEventListener('click', () => showStats(null));

document.getElementById('closeStats').addEventListener('click', () => closeModal('statsModal'));

document.getElementById('newGameFromStats').addEventListener('click', () => {
  closeModal('statsModal');
  openModal('settingsModal');
});

// Close modal on backdrop click (except settings if no game is running)
document.querySelectorAll('.modal').forEach(m => {
  m.addEventListener('click', e => {
    if (e.target === m) {
      if (m.id === 'settingsModal' && S.status === 'idle') return; // force choice
      closeModal(m.id);
    }
  });
});

document.getElementById('equiv-close').addEventListener('click', () => {
  document.getElementById('equiv-notice').hidden = true;
});

// ── Language dropdown ─────────────────────────────────────────────────────────
(async function initLangDropdown() {
  const input   = document.getElementById('langInput');
  const options = document.getElementById('langOptions');
  if (!input || !options) return; // stale cached HTML — bail out gracefully
  let allLangs  = [];
  let activeIdx = -1;

  try {
    const data = await api.languages();
    allLangs = data.languages || [];
  } catch (e) {}

  function render(filter) {
    const q = filter.trim().toLowerCase();
    const matches = q ? allLangs.filter(l => l.toLowerCase().includes(q)) : allLangs;
    options.innerHTML = '';
    activeIdx = -1;
    matches.forEach(lang => {
      const div = document.createElement('div');
      div.className = 'lang-option';
      div.textContent = lang;
      div.addEventListener('mousedown', e => { e.preventDefault(); select(lang); });
      options.appendChild(div);
    });
    options.hidden = matches.length === 0;
  }

  function select(lang) {
    input.value = lang;
    options.hidden = true;
    activeIdx = -1;
  }

  function setActive(idx) {
    const opts = options.querySelectorAll('.lang-option');
    opts.forEach(o => o.classList.remove('active'));
    activeIdx = Math.max(0, Math.min(idx, opts.length - 1));
    if (opts[activeIdx]) {
      opts[activeIdx].classList.add('active');
      opts[activeIdx].scrollIntoView({ block: 'nearest' });
    }
  }

  input.addEventListener('focus', () => render(input.value));
  input.addEventListener('input', () => render(input.value));
  input.addEventListener('blur',  () => setTimeout(() => { options.hidden = true; }, 150));

  input.addEventListener('keydown', e => {
    const opts = options.querySelectorAll('.lang-option');
    if (options.hidden || opts.length === 0) return;
    if (e.key === 'ArrowDown')  { e.preventDefault(); setActive(activeIdx + 1); }
    else if (e.key === 'ArrowUp')   { e.preventDefault(); setActive(activeIdx - 1); }
    else if (e.key === 'Enter' && activeIdx >= 0) { e.preventDefault(); select(opts[activeIdx].textContent); }
    else if (e.key === 'Escape') { options.hidden = true; }
  });
})();
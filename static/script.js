// ── Accent normalization ──────────────────────────────────────────────────────
// Mirrors normalizeChar() in wordle.go.  Maps é→e, ñ→n, ü→u, etc.
function stripDiacritics(s) {
  return s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
}

// Add this global variable at the top of your file to store the current overflow bases
let overflowBases = new Set();

/**
 * Normalizes characters: é -> e, and redirects all unplaced characters to '*'
 */
function getEquivalentBase(char) {
  if (char === '*') return '*';
  const base = stripDiacritics(char);
  // If the character's base wasn't placed on a standard key, it belongs to '*'
  if (overflowBases.has(base)) return '*';
  return base;
}

// ── Keyboard layout tables ────────────────────────────────────────────────────
// Each row is an array of *base* (diacritic-free) characters in keyboard order.
const LAYOUTS = {
  qwerty:  [['q','w','e','r','t','y','u','i','o','p'],
            ['a','s','d','f','g','h','j','k','l'],
            ['z','x','c','v','b','n','m']],
  azerty:  [['a','z','e','r','t','y','u','i','o','p'],
            ['q','s','d','f','g','h','j','k','l','m'],
            ['w','x','c','v','b','n']],
  qwertz:  [['q','w','e','r','t','z','u','i','o','p'],
            ['a','s','d','f','g','h','j','k','l'],
            ['y','x','c','v','b','n','m']],
  nordic:  [['q','w','e','r','t','y','u','i','o','p','å'],
            ['a','s','d','f','g','h','j','k','l','ø','æ'],
            ['z','x','c','v','b','n','m']],
  turkish: [['q','w','e','r','t','y','u','ı','o','p','ğ','ü'],
            ['a','s','d','f','g','h','j','k','l','ş','i'],
            ['z','x','c','v','b','n','m','ö','ç']],
  jcuken:  [['й','ц','у','к','е','н','г','ш','щ','з','х'],
            ['ф','ы','в','а','п','р','о','л','д','ж','э'],
            ['я','ч','с','м','и','т','ь','б','ю']],
  greek:   [['ε','ρ','τ','υ','θ','ι','ο','π'],
            ['α','σ','δ','φ','γ','η','ξ','κ','λ'],
            ['ζ','χ','ψ','ω','β','ν','μ']],
  arabic:  [['ض','ص','ث','ق','ف','غ','ع','ه','خ','ح','ج','د'],
            ['ش','س','ي','ب','ل','ا','ت','ن','م','ك','ط','ذ'],
            ['ئ','ء','ؤ','ر','ى','ة','و','ز','ظ']],
  hebrew:  [['ק','ר','א','ט','ו','ן','ם','פ'],
            ['ש','ד','ג','כ','ע','י','ח','ל','ך','ף'],
            ['ז','ס','ב','ה','נ','צ','ת','ץ']],
  // InScript standard mapped roughly to standard rows
  devanagari:[['औ','ऐ','आ','ई','ऊ','भ','ङ','घ','ध','झ','ढ','ञ'],
              ['ओ','ए','अ','इ','उ','ब','ह','ग','द','ज','ड','श'],
              ['ऑ','ृ','र','क','त','च','ट','प','य','स','म','व','ल','ष','न']],
  bengali: [['ঔ','ঐ','আ','ঈ','ঊ','ভ','ঙ','ঘ','ধ','ঝ','ঢ','ঞ'],
            ['ও','এ','অ','ই','উ','ব','হ','গ','দ','জ','ড','শ'],
            ['ঋ','র','ক','ত','চ','ট','প','য','স','ম','ব','ল','ষ','ন']],
  tamil:   [['ஔ','ஐ','ஆ','ஈ','ஊ','ங','ஞ','ண','ந','ன'],
            ['ஓ','ஏ','அ','இ','உ','க','ச','ட','த','ப','ற'],
            ['எ','ஒ','ய','ர','ல','வ','ழ','ள','ம','ஷ','ஸ','ஹ']],
  telugu:  [['ఔ','ఐ','ఆ','ఈ','ఊ','భ','ఙ','ఘ','ధ','ఝ','ఢ','ఞ'],
            ['ఓ','ఏ','అ','ఇ','ఉ','బ','హ','గ','ద','జ','డ','శ'],
            ['ఎ','ఒ','ర','క','త','చ','ట','ప','య','స','మ','వ','ల','ష','న']],
  thai:    [['โ','ฌ','ฆ','ฏ','โ','ซ','ศ','ฮ','?','ฒ','ฬ','ฦ'],
            ['ฟ','ห','ก','ด','เ','า','ส','ว','ง','ผ','ป','แ','อ'],
            ['พ','ะ','ั','ร','น','ย','บ','ล','ข','ช','ต','ค','ม']],
  hiragana:[['わ','ら','や','ま','は','な','た','さ','か','あ'],
            ['ゐ','り','み','ひ','に','ち','し','き','い'],
            ['ん','る','ゆ','む','ふ','ぬ','つ','す','く','う'],
            ['ゑ','れ','め','へ','ね','て','せ','け','え'],
            ['を','ろ','よ','も','ほ','の','と','そ','こ','お']],
  katakana:[['ワ','ラ','ヤ','マ','ハ','ナ','タ','サ','カ','ア'],
            ['ヰ','リ','ミ','ヒ','ニ','チ','シ','キ','イ'],
            ['ン','ル','ユ','ム','フ','ヌ','ツ','ス','ク','ウ'],
            ['ヱ','レ','メ','ヘ','ネ','テ','セ','ケ','エ'],
            ['ヲ','ロ','ヨ','モ','ホ','ノ','ト','ソ','コ','オ']]
};

const LANG_LAYOUT_MAP = {
  'French': 'azerty', 'German': 'qwertz', 'Norwegian': 'nordic', 
  'Danish': 'nordic', 'Swedish': 'nordic', 'Turkish': 'turkish', 
};

function detectLayout(alphabet) {
  const s = (alphabet || []).join('');
  if (/[a-z]/.test(s)) {
    // If it's Latin, we default to qwerty (other logic like LANG_LAYOUT_MAP 
    // handles specific Latin variations like French/German)
    return 'qwerty';
  }

  if (/[\u0400-\u04FF]/.test(s)) return 'jcuken';
  if (/[\u0370-\u03FF\u1F00-\u1FFF]/.test(s)) return 'greek';
  if (/[\u0600-\u06FF]/.test(s)) return 'arabic';
  if (/[\u05D0-\u05EA]/.test(s)) return 'hebrew';
  if (/[\u0900-\u097F]/.test(s)) return 'devanagari';
  if (/[\u0980-\u09FF]/.test(s)) return 'bengali';
  if (/[\u0B80-\u0BFF]/.test(s)) return 'tamil';
  if (/[\u0C00-\u0C7F]/.test(s)) return 'telugu';
  if (/[\u0E00-\u0E7F]/.test(s)) return 'thai';
  if (/[\u3040-\u309F]/.test(s)) return 'hiragana';
  if (/[\u30A0-\u30FF]/.test(s)) return 'katakana';
  return 'qwerty';
}

// Build an ordered list of keyboard rows from the alphabet + layout.
// Only BASE characters are shown (no accented variants) — the equivalence notice
// already tells the user that E covers É, È, Ê, etc.
function buildKeyboardRows(alphabet, lang) {
  if (!alphabet || alphabet.length === 0) return { rows: null, overflowBases: new Set() };

  const basesInAlphabet = new Set(alphabet.map(ch => stripDiacritics(ch)));
  const layoutName = LANG_LAYOUT_MAP[lang] || detectLayout(alphabet);
  const layout     = LAYOUTS[layoutName] || LAYOUTS.qwerty;

  const placedBases = new Set();
  const rows = [];

  for (const layoutRow of layout) {
    const row = [];
    for (const base of layoutRow) {
      if (basesInAlphabet.has(base)) {
        row.push(base);
        placedBases.add(base);
      }
    }
    if (row.length > 0) rows.push(row);
  }

  // Any base chars not in the layout are classified as "overflow"
  const overflowBases = new Set([...basesInAlphabet].filter(b => !placedBases.has(b)));

  return { rows: rows.length > 0 ? rows : null, overflowBases };
}

// Return groups of equivalent chars: only groups with > 1 member (i.e. has variants).
// Return groups of equivalent chars: groups with > 1 member OR the wildcard '*' group.
function computeEquivalences(alphabet) {
  if (!alphabet || alphabet.length === 0) return [];
  
  const groups = {};
  
  alphabet.forEach(ch => {
    // Use the redirected base (e.g., ॐ -> *)
    const base = getEquivalentBase(ch);
    if (!groups[base]) groups[base] = new Set();
    groups[base].add(ch);
  });

  return Object.entries(groups)
    .filter(([base, chars]) => chars.size > 1 || base === '*')
    .map(([base, chars]) => {
      const arr = [...chars].sort((a, b) => {
        // The base (or '*') always comes first in the array
        if (a === base) return -1;
        if (b === base) return 1;
        return a.localeCompare(b);
      });
      return arr; // [base, variant1, variant2, ...]
    })
    .sort((a, b) => {
      // Keep '*' at the top of the list for visibility, otherwise sort alphabetically
      if (a[0] === '*') return -1;
      if (b[0] === '*') return 1;
      return a[0].localeCompare(b[0]);
    });
}

function showEquivNotice(alphabet) {
  const notice = document.getElementById('equiv-notice');
  const list   = document.getElementById('equiv-list');
  const groups = computeEquivalences(alphabet);

  if (groups.length === 0) { notice.hidden = true; return; }

  list.innerHTML = groups.map((g, i) => {
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
};

// ── API ──────────────────────────────────────────────────────────────────────
async function apiFetch(path, opts = {}) {
  const r = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  return r.json();
}

const api = {
  languages: ()            => apiFetch('/api/languages'),
  newGame:   (b)           => apiFetch('/api/game', { method: 'POST', body: JSON.stringify(b) }),
  guess:     (id, word)    => apiFetch(`/api/game/${id}/guess`, { method: 'POST', body: JSON.stringify({ word }) }),
  stats:     (lang, len)   => apiFetch(`/api/stats?lang=${encodeURIComponent(lang)}&length=${len}`),
};

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
    // pop animation
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
function buildKeyboard(alphabet, lang) {
  const kb = document.getElementById('keyboard');
  kb.innerHTML = '';

  const { rows, overflowBases  } = buildKeyboardRows(alphabet, lang);
  
  // Update global state so computeEquivalences() knows what to group under '*'
  overflowEquivalents = overflowBases;
  currentOverflowBases = overflowBases;

  if (!rows) {
    kb.innerHTML = '<div id="no-keyboard">Type your guess and press <strong>Enter</strong>.</div>';
    return;
  }

  const maxLen    = Math.max(...rows.map(r => r.length));
  const available = Math.min(500, window.innerWidth - 16) - 16;
  const keyW      = Math.max(24, Math.min(52, Math.floor((available - (maxLen - 1) * 5) / maxLen)));
  const wideKeyW  = Math.max(52, Math.round(keyW * 1.5));
  document.documentElement.style.setProperty('--key-width',      keyW     + 'px');
  document.documentElement.style.setProperty('--key-wide-width', wideKeyW + 'px');

  for (const rowChars of rows) {
    const rowEl = newKeyRow();
    for (const char of rowChars) {
      const btn = document.createElement('button');
      btn.className = 'key';
      btn.textContent = char.toUpperCase();
      btn.dataset.char = char;
      btn.addEventListener('pointerdown', e => { e.preventDefault(); onKeyPress(char); });
      rowEl.appendChild(btn);
    }
    kb.appendChild(rowEl);
  }

  // Action row
  const actionRow = newKeyRow();
  actionRow.appendChild(makeKey('⌫', 'wide', onBackspace));
  
  // If there are overflow characters, insert the wildcard * key
  if (currentOverflowBases.size > 0) {
    const starBtn = makeKey('*', '', () => onKeyPress('*'));
    starBtn.id = 'star-key';
    actionRow.appendChild(starBtn);
  }

  actionRow.appendChild(makeKey('Enter', 'wide', onEnter));
  kb.appendChild(actionRow);
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

  if (!result.in_word_list) {
    toast('Not in word list', 1200);
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

  // Show spinner, hide board
  document.getElementById('loading').style.display = 'flex';
  document.getElementById('board').style.display   = 'none';
  document.getElementById('keyboard').style.display = 'none';

  let result;
  try {
    result = await api.newGame({ lang, length, max_guesses: maxGuesses });
  } catch (e) {
    toast('Network error — could not start game');
    S.status = 'idle';
    document.getElementById('loading').style.display = 'none';
    openModal('settingsModal');
    return;
  }

  document.getElementById('loading').style.display = 'none';

  if (result.error) {
    toast(result.error, 5000);
    S.status = 'idle';
    openModal('settingsModal');
    return;
  }

  S.gameId  = result.id;
  S.status  = 'playing';

  document.getElementById('board').style.display   = '';
  document.getElementById('keyboard').style.display = '';

  buildBoard();
  buildKeyboard(result.alphabet, S.lang);
  showEquivNotice(result.alphabet);
}

// ── Stats ─────────────────────────────────────────────────────────────────────
async function showStats(lastResult) {
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

  // Distribution
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

  // Definition
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

// ── Init ──────────────────────────────────────────────────────────────────────
(async function init() {
  // Populate language datalist in the background
  api.languages().then(data => {
    const dl = document.getElementById('langList');
    (data.languages || []).forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      dl.appendChild(opt);
    });
  }).catch(() => {});
})();
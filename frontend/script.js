"use strict";
(() => {
  // src/state.ts
  var S = {
    gameId: null,
    wordLength: 5,
    maxGuesses: 6,
    lang: "English",
    status: "idle",
    currentRow: 0,
    input: [],
    charStates: {},
    lastAttempt: 0,
    rtl: false
  };

  // src/api.ts
  async function apiFetch(path, opts = {}) {
    const method = opts.method ?? "GET";
    console.log(`[api] ${method} ${path}`, opts.body ? JSON.parse(opts.body) : "");
    const t0 = performance.now();
    let r;
    try {
      r = await fetch(path, { headers: { "Content-Type": "application/json" }, ...opts });
    } catch (e) {
      console.error(`[api] ${method} ${path} \u2014 network error after ${((performance.now() - t0) / 1e3).toFixed(1)}s:`, e);
      throw e;
    }
    const ms = ((performance.now() - t0) / 1e3).toFixed(1);
    console.log(`[api] ${method} ${path} \u2192 HTTP ${r.status} (${ms}s)`);
    if (!r.ok) console.error(`[api] HTTP ${r.status} body:`, await r.clone().text());
    return r.json();
  }
  var api = {
    languages: () => apiFetch("/api/languages"),
    avgLength: (lang) => apiFetch(`/api/avglength?lang=${encodeURIComponent(lang)}`),
    newGame: (b) => apiFetch("/api/game", { method: "POST", body: JSON.stringify(b) }),
    guess: (id, word) => apiFetch(`/api/game/${id}/guess`, { method: "POST", body: JSON.stringify({ word }) }),
    stats: (lang, len) => apiFetch(`/api/stats?lang=${encodeURIComponent(lang)}&length=${len}`),
    progress: (lang, len) => apiFetch(`/api/progress?lang=${encodeURIComponent(lang)}&length=${len}`),
    clearCache: () => apiFetch("/api/cache/clear", { method: "POST" })
  };

  // src/board.ts
  function tileSize(wordLen) {
    return Math.min(62, Math.max(28, Math.floor(310 / wordLen)));
  }
  function buildBoard() {
    const board = document.getElementById("board");
    const sz = tileSize(S.wordLength);
    board.innerHTML = "";
    board.style.gridTemplateRows = `repeat(${S.maxGuesses}, 1fr)`;
    document.documentElement.style.setProperty("--tile-size", sz + "px");
    for (let r = 0; r < S.maxGuesses; r++) {
      const wrap = document.createElement("div");
      wrap.className = "board-row-wrap";
      const row = document.createElement("div");
      row.className = "board-row";
      row.id = `row-${r}`;
      row.style.gridTemplateColumns = `repeat(${S.wordLength}, 1fr)`;
      if (S.rtl) row.dir = "rtl";
      for (let c = 0; c < S.wordLength; c++) {
        const tile = document.createElement("div");
        tile.className = "tile";
        tile.id = `tile-${r}-${c}`;
        row.appendChild(tile);
      }
      wrap.appendChild(row);
      const caption = document.createElement("div");
      caption.className = "row-caption";
      caption.id = `caption-${r}`;
      wrap.appendChild(caption);
      board.appendChild(wrap);
    }
  }
  function setRowCaption(rowIdx, chars) {
    const el = document.getElementById(`caption-${rowIdx}`);
    if (!el) return;
    el.textContent = chars ?? "";
  }
  function setTileText(row, col, ch) {
    const t = document.getElementById(`tile-${row}-${col}`);
    if (!t) return;
    t.textContent = ch ? ch.toUpperCase() : "";
    if (ch) {
      t.classList.add("filled");
      t.classList.remove("pop");
      void t.offsetWidth;
      t.classList.add("pop");
    } else {
      t.classList.remove("filled", "pop");
    }
  }
  function updateCurrentRow() {
    for (let c = 0; c < S.wordLength; c++) {
      setTileText(S.currentRow, c, S.input[c] ?? "");
    }
  }
  function revealRow(rowIdx, chars, states, onDone) {
    const FLIP = 400;
    chars.forEach((ch, i) => {
      const t = document.getElementById(`tile-${rowIdx}-${i}`);
      if (!t) return;
      setTimeout(() => {
        t.classList.add("flipping");
        setTimeout(() => {
          t.textContent = ch.toUpperCase();
          t.className = `tile ${states[i]} filled`;
        }, FLIP / 2);
      }, i * FLIP);
    });
    setTimeout(onDone, chars.length * FLIP + FLIP);
  }
  function bounceRow(rowIdx) {
    for (let c = 0; c < S.wordLength; c++) {
      const t = document.getElementById(`tile-${rowIdx}-${c}`);
      if (!t) return;
      setTimeout(() => {
        t.classList.add("bounce");
        t.addEventListener("animationend", () => t.classList.remove("bounce"), { once: true });
      }, c * 80);
    }
  }
  function shakeRow(rowIdx) {
    const row = document.getElementById(`row-${rowIdx}`);
    if (!row) return;
    row.classList.add("shake");
    row.addEventListener("animationend", () => row.classList.remove("shake"), { once: true });
  }

  // src/keyboard.ts
  function stripDiacritics(s) {
    return s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
  }
  function buildKeyboard(rows, overflowBases, onKey, onEnter2, onBack) {
    const kb = document.getElementById("keyboard");
    kb.innerHTML = "";
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
    const keyW = Math.max(24, Math.min(52, Math.min(...keyWPerRow)));
    const wideKeyW = Math.max(52, Math.round(keyW * 1.5));
    document.documentElement.style.setProperty("--key-width", keyW + "px");
    document.documentElement.style.setProperty("--key-wide-width", wideKeyW + "px");
    rows.forEach((rowChars, idx) => {
      const rowEl = document.createElement("div");
      rowEl.className = "key-row";
      for (const char of rowChars) {
        const btn = makeKey(char.toUpperCase(), "", () => onKey(char));
        btn.dataset["char"] = char;
        rowEl.appendChild(btn);
      }
      if (idx === rows.length - 1) {
        const enterBtn = makeKey("Enter", "wide", onEnter2);
        enterBtn.id = "enter-key";
        rowEl.insertBefore(enterBtn, rowEl.firstChild);
        if (hasOverflow) {
          const starBtn = makeKey("*", "", () => onKey("*"));
          starBtn.id = "star-key";
          rowEl.appendChild(starBtn);
        }
        rowEl.appendChild(makeKey("\u232B", "wide", onBack));
      }
      kb.appendChild(rowEl);
    });
  }
  function makeKey(label, extra, handler) {
    const btn = document.createElement("button");
    btn.className = `key ${extra}`.trim();
    btn.textContent = label;
    btn.addEventListener("pointerdown", (e) => {
      e.preventDefault();
      handler();
    });
    return btn;
  }
  function refreshKeyboard() {
    document.querySelectorAll(".key[data-char]").forEach((btn) => {
      const baseKey = stripDiacritics(btn.dataset["char"]);
      const st = S.charStates[baseKey];
      btn.className = "key" + (st ? ` ${st}` : "");
    });
  }

  // src/hangul.ts
  var LEADS = ["\u3131", "\u3132", "\u3134", "\u3137", "\u3138", "\u3139", "\u3141", "\u3142", "\u3143", "\u3145", "\u3146", "\u3147", "\u3148", "\u3149", "\u314A", "\u314B", "\u314C", "\u314D", "\u314E"];
  var VOWELS = ["\u314F", "\u3150", "\u3151", "\u3152", "\u3153", "\u3154", "\u3155", "\u3156", "\u3157", "\u3158", "\u3159", "\u315A", "\u315B", "\u315C", "\u315D", "\u315E", "\u315F", "\u3160", "\u3161", "\u3162", "\u3163"];
  var FINALS = ["", "\u3131", "\u3132", "\u3133", "\u3134", "\u3135", "\u3136", "\u3137", "\u3139", "\u313A", "\u313B", "\u313C", "\u313D", "\u313E", "\u313F", "\u3140", "\u3141", "\u3142", "\u3144", "\u3145", "\u3146", "\u3147", "\u3148", "\u314A", "\u314B", "\u314C", "\u314D", "\u314E"];
  function toIndexMap(list) {
    const m = /* @__PURE__ */ new Map();
    list.forEach((c, i) => {
      if (c !== "") m.set(c, i);
    });
    return m;
  }
  var leadIndex = toIndexMap(LEADS);
  var vowelIndex = toIndexMap(VOWELS);
  var finalIndex = toIndexMap(FINALS);
  function composeHangul(input) {
    const chars = Array.from(input);
    let out = "";
    let i = 0;
    while (i < chars.length) {
      const c = chars[i];
      const lead = leadIndex.get(c);
      const nextVowel = i + 1 < chars.length ? vowelIndex.get(chars[i + 1]) : void 0;
      if (lead !== void 0 && nextVowel !== void 0) {
        const vowel = nextVowel;
        i += 2;
        let final = 0;
        if (i < chars.length) {
          const finalCandidate = finalIndex.get(chars[i]);
          const followingVowel = i + 1 < chars.length ? vowelIndex.get(chars[i + 1]) : void 0;
          if (finalCandidate !== void 0 && followingVowel === void 0) {
            final = finalCandidate;
            i += 1;
          }
        }
        out += String.fromCodePoint(44032 + (lead * 21 + vowel) * 28 + final);
      } else {
        out += c;
        i += 1;
      }
    }
    return out;
  }

  // src/ui.ts
  var toastTimer = null;
  function toast(msg, duration = 1800) {
    const el = document.getElementById("toast");
    el.textContent = msg;
    el.classList.add("show");
    if (toastTimer !== null) clearTimeout(toastTimer);
    if (duration > 0) toastTimer = setTimeout(() => el.classList.remove("show"), duration);
  }
  function clearToast() {
    if (toastTimer !== null) clearTimeout(toastTimer);
    document.getElementById("toast").classList.remove("show");
  }
  function openModal(id) {
    document.getElementById(id).classList.add("open");
  }
  function closeModal(id) {
    document.getElementById(id).classList.remove("open");
  }
  function showEquivNotice(equivalences) {
    const notice = document.getElementById("equiv-notice");
    const list = document.getElementById("equiv-list");
    if (!equivalences || equivalences.length === 0) {
      notice.hidden = true;
      return;
    }
    list.innerHTML = equivalences.map((g) => {
      const base = g[0].toUpperCase();
      const variants = g.slice(1).map((c) => c.toUpperCase()).join(", ");
      return `<span class="equiv-group"><strong>${base}</strong> = ${variants}</span>`;
    }).join(" &middot; ");
    notice.hidden = false;
  }
  async function showStats(lastResult) {
    clearToast();
    let data = {};
    try {
      data = await api.stats(S.lang, S.wordLength);
    } catch (_) {
    }
    document.getElementById("stat-played").textContent = String(data.games_played ?? 0);
    document.getElementById("stat-win-pct").textContent = String(data.win_pct ?? 0);
    document.getElementById("stat-streak").textContent = String(data.current_streak ?? 0);
    document.getElementById("stat-max-streak").textContent = String(data.max_streak ?? 0);
    const dist = data.distribution ?? {};
    const container = document.getElementById("distContainer");
    container.innerHTML = "";
    const maxCount = Math.max(...Object.values(dist).map(Number), 1);
    for (let i = 1; i <= S.maxGuesses; i++) {
      const count = dist[i] ?? 0;
      const pct = Math.max(7, Math.round(count / maxCount * 100));
      const highlight = S.status === "won" && i === S.lastAttempt ? " highlight" : "";
      const bar = document.createElement("div");
      bar.className = "dist-bar";
      bar.innerHTML = `<span class="dist-label">${i}</span><div class="dist-fill${highlight}" style="width:${pct}%">${count}</div>`;
      container.appendChild(bar);
    }
    const defEl = document.getElementById("definition");
    if (lastResult?.answer) {
      const word = lastResult.answer.toUpperCase();
      document.getElementById("defWord").textContent = lastResult.answer_chars ? `${word} (${lastResult.answer_chars})` : word;
      const isKorean = S.lang.startsWith("Korean");
      const wiktTerm = lastResult.answer_chars || (isKorean ? composeHangul(lastResult.answer) : lastResult.answer);
      const wiktLangSection = S.lang.replace(/\s*\(.*\)\s*$/, "");
      const wiktLink = document.getElementById("defWiktionary");
      wiktLink.href = `https://en.wiktionary.org/wiki/${encodeURIComponent(wiktTerm)}#${encodeURIComponent(wiktLangSection)}`;
      wiktLink.style.display = "inline";
      document.getElementById("defText").textContent = lastResult.definition ?? "(no definition available)";
      const etyEl = document.getElementById("defEtymology");
      if (lastResult.etymology) {
        etyEl.textContent = `Etymology: ${lastResult.etymology}`;
        etyEl.style.display = "block";
      } else {
        etyEl.style.display = "none";
      }
      defEl.style.display = "block";
    } else {
      defEl.style.display = "none";
    }
    openModal("statsModal");
  }

  // src/game.ts
  var _progressTimer = null;
  function startProgressPolling(lang, length) {
    const el = document.getElementById("loading-count");
    if (el) el.textContent = "";
    _progressTimer = setInterval(async () => {
      try {
        const { count } = await api.progress(lang, length);
        if (el && count > 0) el.textContent = `${count.toLocaleString()} words found so far\u2026`;
      } catch (_) {
      }
    }, 6e4);
  }
  function stopProgressPolling() {
    if (_progressTimer !== null) clearInterval(_progressTimer);
    _progressTimer = null;
    const el = document.getElementById("loading-count");
    if (el) el.textContent = "";
  }
  function onKeyPress(ch) {
    if (S.status !== "playing") return;
    if (S.input.length >= S.wordLength) return;
    if (/^\p{M}+$/u.test(ch)) return;
    S.input.push(ch);
    updateCurrentRow();
  }
  function onBackspace() {
    if (S.status !== "playing") return;
    S.input.pop();
    updateCurrentRow();
  }
  async function onEnter() {
    if (S.status !== "playing") return;
    if (S.input.length !== S.wordLength) {
      shakeRow(S.currentRow);
      toast(`Enter a ${S.wordLength}-character word`);
      return;
    }
    const word = S.input.join("");
    S.status = "submitting";
    let result;
    try {
      result = await api.guess(S.gameId, word);
    } catch (_) {
      toast("Network error \u2014 please try again");
      S.status = "playing";
      return;
    }
    if (result.error) {
      toast(result.error);
      S.status = "playing";
      shakeRow(S.currentRow);
      return;
    }
    const rowIdx = S.currentRow;
    const chars = [...word];
    const states = result.states;
    const PRI = { correct: 3, present: 2, absent: 1 };
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
      if (t) t.textContent = (chars[c] ?? "").toUpperCase();
    }
    setRowCaption(rowIdx, result.chars);
    revealRow(rowIdx, chars, states, () => {
      refreshKeyboard();
      S.currentRow++;
      S.input = [];
      S.status = "playing";
      if (result.status === "won") {
        S.status = "won";
        S.lastAttempt = result.attempt;
        bounceRow(rowIdx);
        const msgs = ["Genius!", "Magnificent!", "Impressive!", "Splendid!", "Great!", "Phew!"];
        toast(msgs[Math.min(result.attempt - 1, msgs.length - 1)], 0);
        setTimeout(() => showStats(result), 2e3);
      } else if (result.status === "lost") {
        S.status = "lost";
        toast(result.answer.toUpperCase(), 0);
        setTimeout(() => showStats(result), 2500);
      }
    });
  }
  async function startGame() {
    const lang = document.getElementById("langInput").value.trim() || "English";
    const length = parseInt(document.getElementById("lengthInput").value) || 5;
    const maxGuesses = parseInt(document.getElementById("guessesInput").value) || 6;
    closeModal("settingsModal");
    Object.assign(S, { lang, wordLength: length, maxGuesses, status: "loading", currentRow: 0, input: [], charStates: {}, gameId: null });
    document.getElementById("loading").style.display = "flex";
    document.getElementById("board").style.display = "none";
    document.getElementById("keyboard").style.display = "none";
    startProgressPolling(lang, length);
    let result;
    try {
      result = await api.newGame({ lang, length, max_guesses: maxGuesses });
    } catch (_) {
      stopProgressPolling();
      toast("Network error \u2014 could not start game");
      S.status = "idle";
      document.getElementById("loading").style.display = "none";
      openModal("settingsModal");
      return;
    }
    stopProgressPolling();
    document.getElementById("loading").style.display = "none";
    if (result.error) {
      toast(result.error, 5e3);
      S.status = "idle";
      openModal("settingsModal");
      return;
    }
    S.gameId = result.id;
    S.status = "playing";
    S.rtl = result.rtl ?? false;
    document.getElementById("board").style.display = "";
    document.getElementById("keyboard").style.display = "";
    buildBoard();
    buildKeyboard(result.keyboard_rows ?? null, new Set(result.overflow_bases ?? []), onKeyPress, onEnter, onBackspace);
    showEquivNotice(result.equivalences ?? []);
  }

  // src/main.ts
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js");
  }
  document.addEventListener("keydown", (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const tag = document.activeElement?.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA") return;
    if (e.key === "Enter") onEnter();
    else if (e.key === "Backspace") onBackspace();
    else if (e.key.length === 1) onKeyPress(e.key.toLowerCase());
  });
  document.getElementById("startBtn").addEventListener("click", startGame);
  document.getElementById("settingsBtn").addEventListener("click", () => openModal("settingsModal"));
  document.getElementById("statsBtn").addEventListener("click", () => showStats(null));
  document.getElementById("closeStats").addEventListener("click", () => closeModal("statsModal"));
  document.getElementById("newGameFromStats").addEventListener("click", () => {
    closeModal("statsModal");
    openModal("settingsModal");
  });
  document.getElementById("equiv-close").addEventListener("click", () => {
    document.getElementById("equiv-notice").hidden = true;
  });
  document.getElementById("clearCacheBtn").addEventListener("click", async () => {
    try {
      await api.clearCache();
      toast("Cache cleared");
    } catch (_) {
      toast("Failed to clear cache");
    }
  });
  document.querySelectorAll(".modal").forEach((m) => {
    m.addEventListener("click", (e) => {
      if (e.target === m) {
        if (m.id === "settingsModal" && S.status === "idle") return;
        closeModal(m.id);
      }
    });
  });
  (async function initLangDropdown() {
    const input = document.getElementById("langInput");
    const options = document.getElementById("langOptions");
    if (!input || !options) return;
    let allLangs = [];
    let activeIdx = -1;
    try {
      const data = await api.languages();
      allLangs = data.languages ?? [];
    } catch (_) {
    }
    function render(filter) {
      const q = filter.trim().toLowerCase();
      const matches = q ? allLangs.filter((l) => l.toLowerCase().includes(q)) : allLangs;
      options.innerHTML = "";
      activeIdx = -1;
      matches.forEach((lang) => {
        const div = document.createElement("div");
        div.className = "lang-option";
        div.textContent = lang;
        div.addEventListener("mousedown", (e) => {
          e.preventDefault();
          select(lang);
        });
        options.appendChild(div);
      });
      options.hidden = matches.length === 0;
    }
    function select(lang) {
      input.value = lang;
      options.hidden = true;
      activeIdx = -1;
      applyAvgLength(lang);
    }
    function applyAvgLength(lang) {
      const lengthInput = document.getElementById("lengthInput");
      if (!lengthInput) return;
      api.avgLength(lang).then((data) => {
        if (data.avg_length > 0) lengthInput.value = String(Math.round(data.avg_length));
      }).catch(() => {
      });
    }
    function setActive(idx) {
      const opts = options.querySelectorAll(".lang-option");
      opts.forEach((o) => o.classList.remove("active"));
      activeIdx = Math.max(0, Math.min(idx, opts.length - 1));
      opts[activeIdx]?.classList.add("active");
      opts[activeIdx]?.scrollIntoView({ block: "nearest" });
    }
    input.addEventListener("focus", () => render(input.value));
    input.addEventListener("input", () => render(input.value));
    input.addEventListener("blur", () => setTimeout(() => {
      options.hidden = true;
    }, 150));
    input.addEventListener("keydown", (e) => {
      const opts = options.querySelectorAll(".lang-option");
      if (options.hidden || opts.length === 0) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setActive(activeIdx + 1);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setActive(activeIdx - 1);
      } else if (e.key === "Enter" && activeIdx >= 0) {
        e.preventDefault();
        select(opts[activeIdx].textContent);
      } else if (e.key === "Escape") {
        options.hidden = true;
      }
    });
  })();
})();

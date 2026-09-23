// A deliberately tiny stand-in for the browser globals the game modules
// touch. It is not a DOM implementation — just enough of one to drive the
// guess-submission path (tiles, rows, toast) in plain node and read back what
// the code put on screen.

class StubClassList {
  constructor(el) { this.el = el; this.set = new Set(); }
  add(...names)      { names.forEach(n => n && this.set.add(n)); }
  remove(...names)   { names.forEach(n => this.set.delete(n)); }
  contains(name)     { return this.set.has(name); }
  toString()         { return [...this.set].join(' '); }
}

class StubElement {
  constructor(tag = 'div', id = '') {
    this.tagName = tag.toUpperCase();
    this.id = id;
    this.textContent = '';
    this.innerHTML = '';
    this.hidden = false;
    this.dataset = {};
    this.style = {};
    this.children = [];
    this.classList = new StubClassList(this);
    this.offsetWidth = 0;
  }
  get className() { return this.classList.toString(); }
  set className(v) {
    this.classList.set = new Set(String(v).split(/\s+/).filter(Boolean));
  }
  appendChild(child) { this.children.push(child); return child; }
  insertBefore(child) { this.children.unshift(child); return child; }
  addEventListener() {}
  removeEventListener() {}
  querySelectorAll() { return []; }
  scrollIntoView() {}
  setPointerCapture() {}
}

// installDOM puts the stub globals in place and returns a handle for the
// test: `el(id)` fetches (creating on demand) the element with that id.
export function installDOM() {
  const byId = new Map();
  const get = id => {
    if (!byId.has(id)) byId.set(id, new StubElement('div', id));
    return byId.get(id);
  };

  globalThis.document = {
    documentElement: Object.assign(new StubElement('html'), {
      style: { setProperty() {} },
    }),
    activeElement: null,
    getElementById: id => (byId.has(id) ? byId.get(id) : null),
    createElement: tag => new StubElement(tag),
    querySelectorAll: () => [],
    addEventListener() {},
  };
  globalThis.window = { innerWidth: 400, addEventListener() {} };
  globalThis.localStorage = {
    getItem: () => null,
    setItem() {},
    removeItem() {},
  };
  globalThis.performance ??= { now: () => 0 };

  return { el: get, has: id => byId.has(id) };
}

// stubFetch installs a fetch that answers every request with `body`, and
// records the requests it saw.
export function stubFetch(body) {
  const calls = [];
  globalThis.fetch = async (path, opts = {}) => {
    calls.push({ path, body: opts.body ? JSON.parse(opts.body) : null });
    return {
      ok: true,
      status: 200,
      json: async () => body,
      text: async () => JSON.stringify(body),
      clone() { return this; },
    };
  };
  return calls;
}

export const sleep = ms => new Promise(r => setTimeout(r, ms));

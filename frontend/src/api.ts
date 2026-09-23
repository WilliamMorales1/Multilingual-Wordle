import type {
  GameResult, GuessResult, StatsResult,
  ProgressResult, LanguagesResult, NewGameRequest,
} from './types.js';

// Verbose per-request logging is opt-in (localStorage.debug = '1') — it was
// previously unconditional and flooded the console on every request.
const DEBUG = typeof localStorage !== 'undefined' && localStorage.getItem('debug') === '1';

async function apiFetch<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const method = opts.method ?? 'GET';
  if (DEBUG) console.log(`[api] ${method} ${path}`, opts.body ? JSON.parse(opts.body as string) : '');
  const t0 = performance.now();
  let r: Response;
  try {
    r = await fetch(path, { headers: { 'Content-Type': 'application/json' }, ...opts });
  } catch (e) {
    console.error(`[api] ${method} ${path} — network error after ${((performance.now()-t0)/1000).toFixed(1)}s:`, e);
    throw e;
  }
  const ms = ((performance.now() - t0) / 1000).toFixed(1);
  if (DEBUG) console.log(`[api] ${method} ${path} → HTTP ${r.status} (${ms}s)`);
  if (!r.ok) console.error(`[api] HTTP ${r.status} body:`, await r.clone().text());
  return r.json() as Promise<T>;
}

export const api = {
  languages: ():                          Promise<LanguagesResult> => apiFetch('/api/languages'),
  newGame:   (b: NewGameRequest):         Promise<GameResult>      => apiFetch('/api/game', { method: 'POST', body: JSON.stringify(b) }),
  guess:     (id: number, word: string):  Promise<GuessResult>     => apiFetch(`/api/game/${id}/guess`, { method: 'POST', body: JSON.stringify({ word }) }),
  stats:     (lang: string, len: number): Promise<StatsResult>     => apiFetch(`/api/stats?lang=${encodeURIComponent(lang)}&length=${len}`),
  // No length: an auto-length download only picks its length partway
  // through, so the server tracks progress per language.
  progress:  (lang: string):              Promise<ProgressResult>  => apiFetch(`/api/progress?lang=${encodeURIComponent(lang)}`),
  clearCache: (gameId: number | null, lang?: string): Promise<{ok: boolean}> => apiFetch('/api/cache/clear', { method: 'POST', body: JSON.stringify({ game_id: gameId ?? 0, lang: lang ?? '' }) }),
};

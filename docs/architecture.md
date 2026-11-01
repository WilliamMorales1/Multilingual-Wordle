# Architecture

```
backend/
  cmd/server        HTTP server binary
  cmd/tui           terminal client (can embed the API)
  internal/server   route table; APIMux() (JSON only) + Handler() (API + static + /metrics)
  internal/api      HTTP handlers, guess evaluation glue, stats arithmetic
  internal/lang     grapheme splitting, normalization, tone handling, Evaluate, wildcards
  internal/keyboard keyboard layouts, layout detection, equivalence groups, default lengths
  internal/wordlist kaikki.org download/parse, word selection, disk + memory cache, daily answer
  internal/store    SQLite persistence (games, guess_records)
  internal/log      slog setup + request logging middleware
frontend/
  src/              TypeScript (esbuild -> public/script.js)
  public/           index.html, style.css, manifest.json, sw.js, bundled script.js
  test/             node-based tests with a DOM stub
```

## Running locally

The server serves the web frontend from `frontend/public`, so it must be run from the `backend` directory (the Makefile does this). The SQLite database lands at `backend/wordgo.db`, or at `$DATA_DIR/wordgo.db` when `DATA_DIR` is set.

The `api` package's `TestPlay*` tests download real dictionaries and are skipped by `make test`. Run them on their own:

```
cd backend && go test ./internal/api -run TestPlayDefaultLangs -timeout 2h
```

## Terminal client

`backend/cmd/tui` talks to the same backend HTTP API as the web frontend, so all game logic (validation, evaluation, win/loss, letter-state tracking, stats) is shared and identical between the two.

If nothing is answering at the backend URL, the TUI starts a private in-process copy of the API on a loopback port, so no separate server is needed. Point it at an existing backend with `WORDGO_URL=http://host:port make tui`.

Type a guess and press Enter; colored tiles and a running letter-state list print after each guess, same rules as the web version. Press Up/Down to recall previous guesses.

## HTTP API

| Method | Path                         | Purpose |
| ------ | ---------------------------- | ------- |
| GET    | `/api/languages`             | language names + default lengths |
| GET    | `/api/progress?lang=X`       | words parsed so far for an in-flight download |
| POST   | `/api/game`                  | `{"lang": "..."}` → new game + alphabet, keyboard rows, equivalences, RTL flag, matra map |
| GET    | `/api/game/{id}`             | game state, guesses, aggregated key states; answer only once the game is over |
| POST   | `/api/game/{id}/guess`       | `{"word": "..."}` → tiles, states, status, key states |
| GET    | `/api/stats?lang=X&length=Y` | played, won, win %, streaks, guess distribution |
| POST   | `/api/cache/clear`           | drop one lang/length word list (by `game_id` or `lang`) |
| GET    | `/metrics`                   | Prometheus metrics |

Letter/key states are aggregated server-side, so every client colors its keyboard from the same data instead of re-deriving it.

## Deployment

`Dockerfile` builds the Go binary and the frontend bundle in separate stages and copies both into a `scratch` image. `fly.toml` deploys it to fly.io with a persistent volume mounted at `/data` (`DATA_DIR=/data`) and metrics scraped from `/metrics`.

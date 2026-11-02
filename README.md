# Multilingual Wordle

Live at [multilingual-wordle.fly.dev](https://multilingual-wordle.fly.dev/)

A Wordle game in Go and TypeScript that works with any language on Wiktionary: English, Spanish, French, Russian, Arabic, Greek, Hebrew, Thai, and more. New word per language daily.

- Word length set per language from its own dictionary (3–12 tiles).
- Keyboard layout detected from the word list.
- Stats, share text, installable as a PWA.
- Terminal client against the same backend.

## Dependencies

- Backend: Go 1.27+. Modules: `modernc.org/sqlite` (pure-Go SQLite), `prometheus/client_golang` (metrics), `golang.org/x/{net,text,term}`.
- Frontend: Node.js and npm (Docker build uses Node 20). Dev only: TypeScript, esbuild.
- Optional: [air](https://github.com/air-verse/air) for `make watch-backend`; Docker for the container build.
- Word lists download from kaikki.org on first use.

## Running it

```
make build        # frontend bundle + backend binary
make run          # build, then serve http://localhost:8080
make dev          # build frontend, then `go run ./cmd/server`
make tui          # terminal client (starts its own API if none is running)
make test         # go vet + backend tests + frontend typecheck/tests
make watch-frontend
make clean        # drop db, caches, built bundle
```

## Docs

- [Architecture](docs/architecture.md): layout, local setup, TUI, HTTP API, deployment.
- [How it works](docs/how-it-works.md): word selection, keyboards, scripts and tones, guessing.

## Credits

Word data from [kaikki.org](https://kaikki.org/) (Wiktionary wiktextract dumps, CC BY-SA). Inspired by Josh Wardle's Wordle. Personal educational project.

## License

Code: MIT ([LICENSE](LICENSE)). Word data: CC BY-SA.

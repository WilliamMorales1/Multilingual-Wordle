# Multilingual Wordle

Live at [multilingual-wordle.fly.dev/](https://multilingual-wordle.fly.dev/)

A multilingual Wordle game written in Go & TypeScript that works with any language on Wiktionary — English, Spanish, French, Russian, Arabic, Greek, Hebrew, Thai, and more. Word per language resets daily.

## Features

- **Automatic word length** - each language is played at the median word length of its own dictionary (clamped to 3–12 tiles)
- **Per-language keyboard** - the layout is detected from the word list itself (qwerty/azerty/qwertz, Cyrillic, Arabic, Devanagari, hiragana, Ge'ez, Cherokee, syllabics, …), with an overflow `*` key for letters that don't fit
- **Statistics** - played, win %, current/max streak, guess distribution, share text
- **Installable (PWA)** - manifest + service worker; in Chrome, three dots → add to home screen → install
- **Terminal client (TUI)** - play from the command line against the same backend

## Running it

```
make build        # build frontend bundle + backend binary
make run          # build, then run the server (http://localhost:8080)
make dev          # build frontend, then `go run ./cmd/server`
make tui          # terminal client
make test         # go vet + backend unit tests + frontend typecheck/tests
make watch-frontend   # esbuild --watch
make clean        # drop db, caches, built bundle
```

The server serves the web frontend from `frontend/public`, so it must be run from the `backend` directory (the Makefile does this). The SQLite database lands at `backend/wordgo.db`, or at `$DATA_DIR/wordgo.db` when `DATA_DIR` is set.

The `api` package's `TestPlay*` tests download real dictionaries and are skipped by `make test`. Run them on their own:

```
cd backend && go test ./internal/api -run TestPlayDefaultLangs -timeout 2h
```

## Terminal client (TUI)

A terminal frontend lives at `backend/cmd/tui`. It talks to the same backend HTTP API as the web frontend, so all game logic (validation, evaluation, win/loss, letter-state tracking, stats) is shared and identical between the two.

```
make tui
```

If nothing is answering at the backend URL, the TUI starts a private in-process copy of the API on a loopback port — no separate server needed. Point it at an existing backend with `WORDGO_URL=http://host:port make tui`.

Type a guess and press Enter; colored tiles and a running letter-state list print after each guess, same rules as the web version. Press Up/Down to recall previous guesses.

## Architecture

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

### HTTP API

| Method | Path                         | Purpose                                                                                   |
| ------ | ---------------------------- | ----------------------------------------------------------------------------------------- |
| GET    | `/api/languages`             | language names + default lengths                                                          |
| GET    | `/api/progress?lang=X`       | words parsed so far for an in-flight download                                             |
| POST   | `/api/game`                  | `{"lang": "..."}` → new game + alphabet, keyboard rows, equivalences, RTL flag, matra map |
| GET    | `/api/game/{id}`             | game state, guesses, aggregated key states; answer only once the game is over             |
| POST   | `/api/game/{id}/guess`       | `{"word": "..."}` → tiles, states, status, key states                                     |
| GET    | `/api/stats?lang=X&length=Y` | played, won, win %, streaks, guess distribution                                           |
| POST   | `/api/cache/clear`           | drop one lang/length word list (by `game_id` or `lang`)                                   |
| GET    | `/metrics`                   | Prometheus metrics                                                                        |

Letter/key states are aggregated server-side, so every client colors its keyboard from the same data instead of re-deriving it.

### Deployment

`Dockerfile` builds the Go binary and the frontend bundle in separate stages and copies both into a `scratch` image. `fly.toml` deploys it to fly.io with a persistent volume mounted at `/data` (`DATA_DIR=/data`) and metrics scraped from `/metrics`.

## Credits

- Word data from [kaikki.org](https://kaikki.org/) (Wiktionary wiktextract dumps)
- Inspired by Josh Wardle's Wordle

## License

This is a personal educational project. Word data is from Wiktionary (CC BY-SA).

## Implementation Details

### Word selection

- Word lists are streamed and parsed straight from kaikki.org's gzipped JSONL dumps, in parallel across CPU workers, and cached to `backend/cache/<language>_<length>l.json` (plus `_etymology` and, for Chinese, `_hanzi` sidecars).
- Inflected forms, "form of" entries, and entries whose senses are tagged obsolete/rare/archaic and similar are dropped, so the answer is a word someone might actually know.
- Words are weighted when the length is picked: entries tagged common, or heavily cross-referenced from other entries, count for several ordinary ones.
- The playable length is the median length of the language's own words, measured over a sample of up to 100k entries while the dump streams, then clamped to 3–12 tiles. It is remembered per language in `cache/avg_lengths.json` so a restart doesn't re-download just to re-measure. Before anything is downloaded, a rough estimate is used instead: 6 tiles by default, 4 for syllabaries (e.g. Japanese, Cherokee, Inuktitut) and Cangjie, 8 for tone-splitting layouts (Vietnamese, Zhuyin), where tones take their own tile.
- The daily answer is `hash(UTC date, language, length)` modulo the sorted word list — same word for everyone, new word at UTC midnight.

### Scripts and characters

- In general, letters with diacritics are normalized into their analogous non-diacritic version, i.e. 'á' and 'a' are treated as if they are equivalent.
- Chinese is handled by using the romanization (+ Zhuyin for Mandarin in addition to Pinyin) for character counts, considering each dialect as if it's its own language. There is also a Cangjie-based version for Chinese, if you want to play based on characters rather than pronunciation.
- Tones are handled per script, following what an IME actually types:
  - Chinese romanizations are toneless. Mandarin's pinyin tone diacritics are stripped, and the other dialects' trailing tone numerals (Jyutping, POJ/Tâi-lô, etc.) are dropped, so a guess is just the letters. Syllable separators (spaces, hyphens, punctuation) are dropped too.
  - Zhuyin keeps its tones as their own tile. The standalone bopomofo marks ˊ ˇ ˋ ˙ are typed as their own key in a zhuyin IME, so they each count as a space and get their own keyboard key.
  - Vietnamese tones are split into their own tile. The combining marks are pulled off the base letter and shown as standalone tiles: huyền `` ` ``, sắc `´`, ngã `~`, hỏi `ˀ`, nặng `.`. Other diacritics (ă, â, ê, ô, ơ, ư, đ) stay part of the letter.
- Japanese is handled by converting all words to hiragana. Each mora == 1 space.
- Brahmic scripts (Devanagari, Gujarati, Tamil, etc.) and other abugidas are handled by counting each unicode point as a separate space (including combining diacritics for vowels).
- RTL scripts are handled by simply showing the spaces fill in RTL instead. Letters are each shown in their independent forms.
- Hangul characters are not shown in Jamo blocks, and instead are shown together horizontally, each counting as 1 space each. Consonant clusters, duplicated consonants, and diphthongs are all counted as 2 spaces, not 1.

### Guessing

- Guesses are matched against the word list after normalization, so an unaccented or otherwise loosely-typed guess still resolves to the real word.
- `*` is a wildcard: the overflow key stands for every letter too rare to get its own key, and a guess containing `*` is resolved against the word list by matching everything else. The game shows which letters share a key ("equivalences") when a language needs them.
- Six guesses, as usual. Statistics are kept per language and word length in SQLite, over both wins and losses.

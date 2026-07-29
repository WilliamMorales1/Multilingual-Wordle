# Multilingual Wordle

Live at [multilingual-wordle.fly.dev/](multilingual-wordle.fly.dev/)

A multilingual Wordle game written in Go & Typescript that works with **any language** on Wiktionary — English, Spanish, French, Russian, Arabic, Greek, Hebrew, Thai, and more. Word per language resets daily.

## Features

- **70+ languages** supported (any language from kaikki.org)
- **Unicode-aware** handles grapheme clusters correctly for all scripts
- **Automatic caching** downloading word lists once, then caches locally
- **Color-coded feedback** with green (correct), yellow (present), gray (absent)
- **Visual keyboard** to see which letters you've used
- **Dowloadable** as an app via Chome (three dots -> add to home screen -> install)
- **Terminal client (TUI)** — play from the command line against the same backend

## Terminal client (TUI)

A simple terminal frontend lives at `backend/cmd/tui`, alongside the web frontend — it talks to the same backend HTTP API, so all game logic (validation, evaluation, win/loss, letter-state tracking, stats) is shared and identical between the two.

```
cd backend && go run ./cmd/server   # start the backend first, in one terminal
make tui                            # then, in another
```

Type a guess and press Enter; colored tiles and a running letter-state list print after each guess, same rules as the web version. Press Up/Down to recall previous guesses. Point at a different backend with `WORDGO_URL=http://host:port make tui`.

## Credits

- Word data from [kaikki.org](https://kaikki.org/) (Wiktionary wiktextract dumps)
- Inspired by Josh Wardle's Wordle

## License

This is a personal educational project. Word data is from Wiktionary (CC BY-SA).

## Implementation Details

- In general, letters with diacritics are normalized into their analogous non-diacritic version, i.e. 'á' and 'a' are treated as if they are equivalent.
- Most languages average around 6 phonemes in non-technical vocabulary, so the default # of spaces for most languages is set to 6 (including English, in contrast to regular wordle). For languages with syllabaries (such as Japanese), the default is set to 4, since each character is typically two phonemes. Canjie is set to 4 and tonal languages are set to 8 by default.
- Chinese is handled by using the romanization (+ Zhuyin for Mandarin in addition to Pinyin) for character counts, considering each dialect as if it's its own language. There is also a Cangjie-based version for Chinese, if you want to play based on characters rather than pronunciation.
- Chinese and Vietnamese tones are considered their own space. Chinese tones use each dialect's own conventional numbered tone notation (e.g. Pinyin 1-4, Jyutping 1-6, POJ/Tâi-lô 1-8), keeping every dialect's tone distinctions instead of folding them into a shared category.
- Japanese is handled by converting all words to hiragana. Each mora == 1 space.
- Brahmic scripts (Devanagari, Gujarati, Tamil, etc.) and other abugidas are handled by counting each unicode point as a seperate space (including combining diacritics for vowels).
- RTL scripts are handled by simply showing the spaces fill in RTL instead. Letters are each shown in their independent forms.
- Hangul characters are not shown in Jamo blocks, and instead are shown together horizontally, each counting as 1 space each. Consonant clusters, duplicated consonants, and diphthongs are all counted as 2 spaces, not 1.

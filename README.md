# Multilingual Wordle

Live at [multilingual-wordle.fly.dev/](multilingual-wordle.fly.dev/)

A multilingual Wordle game written in Go & Typescript that works with **any language** on Wiktionary — English, Spanish, French, Russian, Arabic, Greek, Hebrew, Thai, and more.

## Features

- **70+ languages** supported (any language from kaikki.org)
- **Unicode-aware** handles grapheme clusters correctly for all scripts
- **Automatic caching** downloading word lists once, then caches locally
- **Color-coded feedback** with green (correct), yellow (present), gray (absent)
- **Visual keyboard** to see which letters you've used
- **Dowloadable** as an app via Chome (three dots -> add to home screen -> install)

## Credits

- Word data from [kaikki.org](https://kaikki.org/) (Wiktionary wiktextract dumps)
- Inspired by Josh Wardle's Wordle

## License

This is a personal educational project. Word data is from Wiktionary (CC BY-SA).

## Implementation Details

- In general, letters with diacritics are normalized into their analogous non-diacritic version, i.e. 'á' and 'a' are treated as if they are equivalent.
- Most languages average around 6 phonemes in non-technical vocabulary, so the default # of spaces for most languages is set to 6 (including English, in contrast to regular wordle). For languages with syllabaries (such as Japanese), the default is set to 3, since each character is typically two phonemes. Canjie is set to 4 and tonal languages are set to 8 by default.
- Chinese is handled by using the romanization (+ Zhuyin for Mandarin in addition to Pinyin) for character counts, considering each dialect as if it's its own language. There is also a Canjie-based version for Chinese, if you want to play based on characters rather than pronunciation.
- Chinese and Vietnamese tones are considered their own space. Chinese tones are categorized based on their Middle Chinese names (as this was the easiest middle-ground between dialects to implement).
- Japanese is handled by converting all words to hiragana. Each mora == 1 space.
- Brahmic scripts (Devanagari, Gujarati, Tamil, etc.) and other abugidas are handled by counting each unicode point as a seperate space (including combining diacritics for vowels).
- RTL scripts are handled by simply showing the spaces fill in RTL instead. Letters are each shown in their independent forms.
- Hangul characters are not shown in Jamo blocks, and instead are shown together horizontally, each counting as 1 space each. Consonant clusters, duplicated consonants, and diphthongs are all counted as 2 spaces, not 1.

# How it works

## Word selection

- Word lists are streamed and parsed straight from kaikki.org's gzipped JSONL dumps, in parallel across CPU workers, and cached to `backend/cache/<language>_<length>l.json` (plus `_etymology` and, for Chinese, `_hanzi` sidecars).
- Inflected forms, "form of" entries, and entries whose senses are tagged obsolete/rare/archaic and similar are dropped, so the answer is a word someone might actually know. Words tagged obsolete, archaic or historical in every sense are never the day's answer but stay valid guesses.
- Words are weighted when the length is picked: entries tagged common, or heavily cross-referenced from other entries, count for several ordinary ones.
- The playable length is the median length of the language's own words, measured over a sample of up to 100k entries while the dump streams, then clamped to 3–12 tiles. It is remembered per language in `cache/avg_lengths.json` so a restart doesn't re-download just to re-measure. Before anything is downloaded, a rough estimate is used instead: 6 tiles by default, 4 for syllabaries (e.g. Japanese, Cherokee, Inuktitut) and Cangjie, 8 for tone-splitting layouts (Vietnamese, Zhuyin), where tones take their own tile.
- The daily answer is `hash(UTC date, language, length)` modulo the sorted word list: same word for everyone, new word at UTC midnight.

## Keyboards

The layout is detected from the word list itself (qwerty/azerty/qwertz, Cyrillic, Arabic, Devanagari, hiragana, Ge'ez, Cherokee, syllabics, …), with an overflow `*` key for letters that don't fit.

## Scripts and characters

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

## Guessing

- Guesses are matched against the word list after normalization, so an unaccented or otherwise loosely-typed guess still resolves to the real word.
- `*` is a wildcard: the overflow key stands for every letter too rare to get its own key, and a guess containing `*` is resolved against the word list by matching everything else. The game shows which letters share a key ("equivalences") when a language needs them.
- Six guesses, as usual. Statistics are kept per language and word length in SQLite, over both wins and losses.

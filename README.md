# Wordle CLI with Go

A multilingual Wordle game written in Go that works with **any language** on kaikki.org — Latin, Cyrillic, Arabic, Japanese, Korean, Greek, Hebrew, Thai, and more.

## Features

- **70+ languages** supported (any language from kaikki.org)
- **Unicode-aware** — handles grapheme clusters correctly for all scripts
- **Automatic caching** — downloads word lists once, then caches locally
- **Color-coded feedback** — green (correct), yellow (present), gray (absent)
- **Visual keyboard** — see which letters you've used

## Installation

```bash
# Clone or download the files
# wordle.go and go.mod

# Initialize the module and download dependencies
go mod tidy

# Build the executable
go build -o wordle wordle.go

# Or run directly
go run wordle.go
```

**Important**: If you get a "use of vendored package not allowed" error, run:
```bash
go mod tidy
# This creates go.sum and ensures dependencies are properly downloaded
```

## Usage

```bash
# English, 5 letters, 6 guesses (default)
./wordle

# French Wordle
./wordle --lang French

# Russian Wordle (Cyrillic)
./wordle --lang Russian

# Arabic Wordle
./wordle --lang Arabic

# Japanese Wordle
./wordle --lang Japanese

# Korean Wordle (2 characters)
./wordle --lang Korean --length 2

# Spanish with 6 letters and 8 guesses
./wordle --lang Spanish --length 6 --guesses 8

# List all known languages
./wordle --list-langs

# Clear cache and re-download
./wordle --clear-cache --lang French
```

## Command-line Options

- `--lang` / `-l` — Language to use (default: English)
- `--length` / `-n` — Characters per word (default: 5)
- `--guesses` / `-g` — Maximum guesses allowed (default: 6)
- `--list-langs` — Print known language names and exit
- `--clear-cache` — Delete cached word list for the chosen language/length

## How It Works

1. **First run**: Downloads word list from kaikki.org (Wiktionary dumps)
2. **Subsequent runs**: Uses cached word list from `~/.wordle_LANG_Nl_cache.json`
3. **Unicode handling**: Counts grapheme clusters, not bytes — so "é" = 1 character, not 2
4. **Definitions**: Shows the first definition from Wiktionary after each game

## Examples

### English (default)
```bash
./wordle
```

### French 5-letter words
```bash
./wordle --lang French
```

### Russian 4-letter words
```bash
./wordle --lang Russian --length 4
```

### Japanese 3-character words
```bash
./wordle --lang Japanese --length 3
```

### Arabic 5-letter words
```bash
./wordle --lang Arabic
```

## Supported Languages

Run `./wordle --list-langs` to see all 70+ known languages, including:

- **European**: English, French, Spanish, German, Italian, Russian, Polish, Greek, etc.
- **Asian**: Chinese, Japanese, Korean, Thai, Vietnamese, Hindi, etc.
- **Middle Eastern**: Arabic, Hebrew, Persian, Turkish, Urdu, etc.
- **Other**: Latin, Sanskrit, Esperanto, Irish, Welsh, Swahili, etc.

Any language available on [kaikki.org](https://kaikki.org/dictionary/) will work!

## Word Length

Word length is counted in **grapheme clusters** (visual characters), not bytes:
- `café` = 4 characters (not 5)
- `日本語` = 3 characters
- Arabic with diacritics: combining marks don't inflate the count

## Cache Location

Cached word lists are stored in your home directory:
- Format: `~/.wordle_LANGUAGE_Nl_cache.json`
- Example: `~/.wordle_french_5l_cache.json`

Clear cache with `--clear-cache` if you want to re-download.

## Dependencies

- Go 1.21 or later
- `golang.org/x/text` — for Unicode normalization

## Notes

- **First download**: May take a minute for each new language/length combination
- **Large languages**: Languages without dedicated dumps fall back to the full 2.3GB raw dump
- **Definitions**: Pulled from the first sense in the Wiktionary entry

## Credits

- Word data from [kaikki.org](https://kaikki.org/) (Wiktionary wiktextract dumps)
- Inspired by Josh Wardle's Wordle
- Go port of the Python multilingual Wordle

## License

This is a personal educational project. Word data is from Wiktionary (CC BY-SA).

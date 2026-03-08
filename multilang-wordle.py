"""
Wordle - Terminal Edition
Word list sourced from kaikki.org's wiktextract-processed Wiktionary dump.

Works with ANY language on kaikki.org — Latin, Cyrillic, Arabic, Japanese,
Korean, Greek, Hebrew, Thai, etc.

Usage:
  python wordle.py                         # English, 5 letters, 6 guesses
  python wordle.py --lang French           # French, 5 letters
  python wordle.py --lang Russian          # Russian, 5 letters (Cyrillic)
  python wordle.py --lang Arabic           # Arabic, 5 letters
  python wordle.py --lang Japanese         # Japanese, 5 letters
  python wordle.py --lang Spanish --length 6
  python wordle.py --length 4 --guesses 8
  python wordle.py --list-langs            # show known language names
  python wordle.py --clear-cache --lang French

Word length is counted in Unicode grapheme clusters (not bytes), so CJK /
Arabic / Cyrillic / Indic lengths all work naturally.

No extra dependencies — stdlib only (argparse + gzip + json + unicodedata + urllib).
"""

from __future__ import annotations

import argparse
import gzip
import json
import os
import random
import sys
import unicodedata
import urllib.parse
import urllib.request
import urllib.error
from pathlib import Path

# ── ANSI colours ──────────────────────────────────────────────────────────────
RESET  = "\033[0m"
BOLD   = "\033[1m"
GREEN  = "\033[42m\033[97m"
YELLOW = "\033[43m\033[30m"
GRAY   = "\033[100m\033[97m"
WHITE  = "\033[47m\033[30m"

# ── Known kaikki.org language names ───────────────────────────────────────────
KNOWN_LANGUAGES: dict[str, str] = {
    "afrikaans": "Afrikaans", "arabic": "Arabic", "armenian": "Armenian",
    "basque": "Basque", "bengali": "Bengali", "bulgarian": "Bulgarian",
    "catalan": "Catalan", "chinese": "Chinese", "czech": "Czech",
    "danish": "Danish", "dutch": "Dutch", "english": "English",
    "esperanto": "Esperanto", "estonian": "Estonian", "faroese": "Faroese",
    "finnish": "Finnish", "french": "French", "galician": "Galician",
    "georgian": "Georgian", "german": "German", "greek": "Greek",
    "gujarati": "Gujarati", "hebrew": "Hebrew", "hindi": "Hindi",
    "hungarian": "Hungarian", "icelandic": "Icelandic", "indonesian": "Indonesian",
    "irish": "Irish", "italian": "Italian", "japanese": "Japanese",
    "kannada": "Kannada", "kazakh": "Kazakh", "korean": "Korean",
    "latin": "Latin", "latvian": "Latvian", "lithuanian": "Lithuanian",
    "macedonian": "Macedonian", "malay": "Malay", "maltese": "Maltese",
    "marathi": "Marathi", "mongolian": "Mongolian", "norwegian": "Norwegian",
    "occitan": "Occitan", "persian": "Persian", "polish": "Polish",
    "portuguese": "Portuguese", "punjabi": "Punjabi", "romanian": "Romanian",
    "russian": "Russian", "sanskrit": "Sanskrit", "scots": "Scots",
    "serbian": "Serbian", "slovak": "Slovak", "slovenian": "Slovenian",
    "spanish": "Spanish", "swahili": "Swahili", "swedish": "Swedish",
    "tagalog": "Tagalog", "tamil": "Tamil", "telugu": "Telugu",
    "thai": "Thai", "tibetan": "Tibetan", "turkish": "Turkish",
    "ukrainian": "Ukrainian", "urdu": "Urdu", "vietnamese": "Vietnamese",
    "welsh": "Welsh", "yoruba": "Yoruba",
}

DEFAULT_LANG    = "English"
DEFAULT_LENGTH  = 5
DEFAULT_GUESSES = 6


# ── Helpers ───────────────────────────────────────────────────────────────────

def word_chars(word: str) -> list[str]:
    """
    Split a word into logical characters (grapheme clusters).
    Combining marks (diacritics, vowel points, etc.) are attached to their
    base character so that e.g. Arabic tashkeel or Hebrew niqqud don't inflate
    the length count.
    """
    chars: list[str] = []
    for ch in word:
        if chars and unicodedata.combining(ch):
            chars[-1] += ch
        else:
            chars.append(ch)
    return chars


def word_len(word: str) -> int:
    return len(word_chars(word))


def kaikki_url(lang: str) -> str:
    slug = urllib.parse.quote(lang, safe="")
    return (
        f"https://kaikki.org/dictionary/{slug}/"
        f"kaikki.org-dictionary-{slug}.jsonl.gz"
    )


def cache_path(lang: str, length: int) -> Path:
    safe = lang.lower().replace(" ", "_")
    return Path.home() / f".wordle_{safe}_{length}l_cache.json"


# ── Word validation ───────────────────────────────────────────────────────────

_LETTER_CATS = {"Ll", "Lu", "Lt", "Lo", "Lm", "Mn", "Mc", "Me"}


def _is_word_char(ch: str) -> bool:
    """Accept letters and combining marks from any Unicode script."""
    return unicodedata.category(ch) in _LETTER_CATS


def _valid(word: str, length: int) -> bool:
    if word_len(word) != length:
        return False
    return all(_is_word_char(ch) for ch in word)


# ── Word-list sourcing ────────────────────────────────────────────────────────

RAW_DUMP_URL = "https://kaikki.org/dictionary/raw-wiktextract-data.jsonl.gz"


def _first_gloss(entry: dict) -> str:
    """Extract the first definition gloss from a kaikki entry, or empty string."""
    for sense in entry.get("senses", []):
        glosses = sense.get("glosses", [])
        if glosses and isinstance(glosses[0], str):
            return glosses[0]
    return ""


def _stream_url(url: str, lang: str, length: int) -> dict[str, str]:
    """Stream a gzip JSONL URL and collect matching words with their first definition."""
    words: dict[str, str] = {}
    req = urllib.request.Request(url, headers={"Accept-Encoding": "gzip"})
    with urllib.request.urlopen(req, timeout=300) as resp:
        with gzip.GzipFile(fileobj=resp) as gz:
            for raw_line in gz:
                try:
                    entry: dict = json.loads(raw_line)
                except json.JSONDecodeError:
                    continue
                if entry.get("lang") != lang:
                    continue
                word: str = entry.get("word", "")
                if not _valid(word, length):
                    continue
                if not entry.get("senses"):
                    continue
                key = word.lower()
                if key not in words:
                    words[key] = _first_gloss(entry)
                if len(words) % 500 == 0:
                    print(f"\r  {len(words):,} words collected…", end="", flush=True)
    return words


def stream_kaikki(lang: str, length: int) -> dict[str, str]:
    url = kaikki_url(lang)

    print(f"  Downloading {lang} wiktextract dump from kaikki.org …")
    print(f"  URL: {url}")
    print("  (This only happens once per language/length; results are cached.)\n")

    try:
        words = _stream_url(url, lang, length)

    except urllib.error.HTTPError as e:
        if e.code != 404:
            print(f"\n  HTTP {e.code} error. Please try again later.")
            sys.exit(1)

        # No dedicated per-language dump — fall back to the full raw dump.
        print(f"\n  No dedicated dump found for '{lang}' (HTTP 404).")
        print(f"  Falling back to the full raw wiktextract dump (~2.3 GB compressed).")
        print(f"  This will take several minutes but only happens once.\n")
        yn = input("  Proceed with large download? [y/N] ").strip().lower()
        if yn != "y":
            print("\n  Aborted. Check language name at https://kaikki.org/dictionary/")
            sys.exit(0)

        print(f"\n  Streaming {RAW_DUMP_URL} …\n")
        try:
            words = _stream_url(RAW_DUMP_URL, lang, length)
        except urllib.error.HTTPError as e2:
            print(f"\n  HTTP {e2.code}: could not download raw dump.")
            sys.exit(1)

        if not words:
            print(f"\n  No {length}-character words found for '{lang}' in the raw dump.")
            print(f"  The language name may be wrong. Check: https://kaikki.org/dictionary/")
            sys.exit(1)

    print(f"\r  {len(words):,} {lang} {length}-character words collected.   ")
    return words


def load_word_list(lang: str, length: int) -> dict[str, str]:
    cf = cache_path(lang, length)
    if cf.exists():
        try:
            cached = json.loads(cf.read_text(encoding="utf-8"))
            if isinstance(cached, dict) and len(cached) >= 20:
                print(f"  Loaded {len(cached):,} words from cache ({cf.name}).")
                return cached
            # Legacy list cache — discard and re-download
        except Exception:
            pass

    words = stream_kaikki(lang, length)

    if not words:
        print(f"  No {length}-character {lang} words found. Try a different length.")
        sys.exit(1)

    try:
        cf.write_text(json.dumps(words, ensure_ascii=False), encoding="utf-8")
        print(f"  Cached at {cf}")
    except Exception:
        pass
    return words


# ── Rendering ─────────────────────────────────────────────────────────────────

def _is_wide(ch: str) -> bool:
    """True for CJK and other double-width terminal characters."""
    return unicodedata.east_asian_width(ch[0]) in ("W", "F")


def tile(char: str, state: str) -> str:
    pad = "" if _is_wide(char) else " "
    c = char.upper()
    if state == "correct": return f"{GREEN}{pad}{c}{pad}{RESET}"
    if state == "present": return f"{YELLOW}{pad}{c}{pad}{RESET}"
    if state == "absent":  return f"{GRAY}{pad}{c}{pad}{RESET}"
    return f"{WHITE}{pad}{c}{pad}{RESET}"


def print_board(guesses: list[tuple[str, list[str]]], max_guesses: int, wlen: int) -> None:
    print()
    for word, states in guesses:
        chars = word_chars(word)
        print("  " + "".join(tile(c, s) for c, s in zip(chars, states)))
    blank = f"{GRAY}   {RESET}"
    for _ in range(max_guesses - len(guesses)):
        print("  " + blank * wlen)
    print()


def print_keyboard(guesses: list[tuple[str, list[str]]]) -> None:
    """
    For Latin scripts: show QWERTY rows coloured by state.
    For all other scripts: show seen characters grouped by state (correct / present / absent).
    """
    priority = {"correct": 3, "present": 2, "absent": 1, "": 0}
    state: dict[str, str] = {}
    for word, states in guesses:
        for ch, st in zip(word_chars(word), states):
            if priority[st] > priority.get(state.get(ch, ""), 0):
                state[ch] = st

    if not state:
        return

    all_ascii = all(ch.isascii() for ch in state)

    if all_ascii:
        for row in ["qwertyuiop", "asdfghjkl", "zxcvbnm"]:
            line = "  "
            for ch in row:
                st = state.get(ch, "")
                line += tile(ch, st) if st else f"{WHITE} {ch.upper()} {RESET}"
            print(line)
    else:
        groups: dict[str, list[str]] = {"correct": [], "present": [], "absent": []}
        for ch, st in sorted(state.items()):
            if st in groups:
                groups[st].append(ch)
        labels = {"correct": "✓ right place", "present": "~ wrong place", "absent": "✗ not in word"}
        for st, chars in groups.items():
            if chars:
                print(f"  {labels[st]}: " + " ".join(tile(c, st) for c in chars))

    print()


# ── Game logic ────────────────────────────────────────────────────────────────

def evaluate(guess_chars: list[str], answer_chars: list[str]) -> list[str]:
    length = len(answer_chars)
    states = ["absent"] * length
    pool: list[str | None] = list(answer_chars)
    for i, (g, a) in enumerate(zip(guess_chars, answer_chars)):
        if g == a:
            states[i] = "correct"
            pool[i]   = None
    for i, g in enumerate(guess_chars):
        if states[i] == "correct":
            continue
        if g in pool:
            states[i] = "present"
            pool[pool.index(g)] = None
    return states


def clear() -> None:
    os.system("cls" if sys.platform == "win32" else "clear")


def banner(lang: str, wlen: int, max_guesses: int) -> None:
    print(f"\n{BOLD}  W O R D L E  —  {lang} Edition  ({wlen} chars, {max_guesses} guesses){RESET}")
    print(f"  {GREEN}   {RESET} Correct character & position")
    print(f"  {YELLOW}   {RESET} Correct character, wrong position")
    print(f"  {GRAY}   {RESET} Character not in word\n")


def play(word_list: dict[str, str], lang: str, wlen: int, max_guesses: int) -> None:
    answer       = random.choice(list(word_list.keys()))
    answer_chars = word_chars(answer)
    guesses: list[tuple[str, list[str]]] = []
    word_set = set(word_list.keys())

    clear()
    banner(lang, wlen, max_guesses)

    for attempt in range(1, max_guesses + 1):
        while True:
            raw = input(f"  Guess {attempt}/{max_guesses}: ").strip().lower()
            raw_chars = word_chars(raw)
            if len(raw_chars) != wlen or not all(_is_word_char(c) for c in raw_chars):
                print(f"  ✗ Enter a {wlen}-character word.")
                continue
            if raw not in word_set:
                yn = input("  Not in word list — play anyway? [y/N] ").strip().lower()
                if yn != "y":
                    continue
            break

        states = evaluate(raw_chars, answer_chars)
        guesses.append((raw, states))
        clear()
        banner(lang, wlen, max_guesses)
        print_board(guesses, max_guesses, wlen)
        print_keyboard(guesses)

        if raw == answer:
            msgs = ["Genius!", "Magnificent!", "Impressive!", "Splendid!", "Great!", "Phew!",
                    "Lucky!", "Barely made it!"]
            msg = msgs[min(attempt - 1, len(msgs) - 1)]
            print(f"  🎉 {BOLD}{msg}{RESET}  ({attempt}/{max_guesses})\n")
            _print_definition(answer, word_list.get(answer, ""))
            return

    print(f"  💀 The word was {BOLD}{answer.upper()}{RESET}\n")
    _print_definition(answer, word_list.get(answer, ""))


def _print_definition(word: str, definition: str) -> None:
    """Print the word's first definition, if available."""
    if definition:
        print(f"  📖 {BOLD}{word.upper()}{RESET}: {definition}\n")
    else:
        print(f"  📖 {BOLD}{word.upper()}{RESET}: (no definition available)\n")


# ── CLI ───────────────────────────────────────────────────────────────────────

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Wordle — Wiktionary Edition (any language / any script)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python wordle.py
  python wordle.py --lang French
  python wordle.py --lang Russian
  python wordle.py --lang Arabic --length 4
  python wordle.py --lang Japanese --length 3
  python wordle.py --lang Korean --length 2
  python wordle.py --lang Spanish --length 6 --guesses 8
  python wordle.py --list-langs
  python wordle.py --clear-cache --lang French
        """,
    )
    parser.add_argument("--lang", "-l", default=DEFAULT_LANG, metavar="LANGUAGE",
        help=f"Language to use (default: {DEFAULT_LANG}). Pass the English "
             "title-case name used by kaikki.org, e.g. 'French', 'Russian', "
             "'Arabic', 'Japanese'. Use --list-langs for known options.")
    parser.add_argument("--length", "-n", type=int, default=DEFAULT_LENGTH, metavar="N",
        help=f"Characters per word (default: {DEFAULT_LENGTH}). Counts Unicode "
             "grapheme clusters, so works correctly for every script.")
    parser.add_argument("--guesses", "-g", type=int, default=DEFAULT_GUESSES, metavar="N",
        help=f"Maximum guesses allowed (default: {DEFAULT_GUESSES}).")
    parser.add_argument("--list-langs", action="store_true",
        help="Print known language names and exit.")
    parser.add_argument("--clear-cache", action="store_true",
        help="Delete cached word list for the chosen language/length, then re-download.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    if args.list_langs:
        print("\n  Known language names (pass as-is to --lang):\n")
        items = sorted(KNOWN_LANGUAGES.values())
        for i, name in enumerate(items):
            print(f"    {name:<20}", end="")
            if (i + 1) % 3 == 0:
                print()
        if len(items) % 3:
            print()
        print("\n  Any other kaikki.org language name works too.")
        print("  Browse: https://kaikki.org/dictionary/\n")
        return

    lang_key = args.lang.lower()
    lang: str | None = KNOWN_LANGUAGES.get(lang_key, args.lang.title())
    if lang == None:
        print("  No languages found")
        exit(0)

    wlen        = args.length
    max_guesses = args.guesses

    if wlen < 2 or wlen > 20:
        print("  --length must be between 2 and 20.")
        sys.exit(1)
    if max_guesses < 1 or max_guesses > 30:
        print("  --guesses must be between 1 and 30.")
        sys.exit(1)

    if args.clear_cache:
        cf = cache_path(lang, wlen)
        if cf.exists():
            cf.unlink()
            print(f"  Cache cleared: {cf}")
        else:
            print(f"  No cache found for {lang} / {wlen}-character words.")

    print(f"\n  Loading {lang} {wlen}-character word list…")
    words = load_word_list(lang, wlen)
    print(f"  {len(words):,} words ready.\n")

    while True:
        play(words, lang, wlen, max_guesses)
        if input("  Play again? [Y/n] ").strip().lower() == "n":
            print("\n  Thanks for playing!\n")
            break


if __name__ == "__main__":
    main()
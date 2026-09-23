// Command tui is a simple terminal frontend for wordgo. It talks to the same
// HTTP API the web frontend uses, so all game logic (validation, evaluation,
// win/loss, letter-state aggregation, stats) lives in the backend and this
// client only has to render what the server sends back.
package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"wordgo/internal/lang"
	"wordgo/internal/server"
	"wordgo/internal/store"
)

const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	fgRed  = "\x1b[31m"
	fgCyan = "\x1b[36m"
	// Tiles use xterm-256 palette indices rather than the basic 8/16 colors:
	// indices 16-255 are fixed by the spec, while the low 16 are remapped by
	// the terminal theme, which made "present" tiles show up green.
	bgGreen  = "\x1b[48;5;71;38;5;235m"  // correct
	bgYellow = "\x1b[48;5;179;38;5;235m" // present / wrong place
	bgGray   = "\x1b[48;5;244;38;5;231m" // absent
)

// legend shows a sample of each tile color so the meaning of the colors is
// visible without having to guess at them.
func legend() string {
	return fmt.Sprintf("%s A %s correct  %s B %s wrong place  %s C %s not in word",
		bgGreen, reset, bgYellow, reset, bgGray, reset)
}

func tileColor(state string) string {
	switch state {
	case "correct":
		return bgGreen
	case "present":
		return bgYellow
	default:
		return bgGray
	}
}

type apiClient struct {
	baseURL string
	http    *http.Client
}

func (c *apiClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *apiClient) post(path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.http.Post(c.baseURL+path, "application/json", strings.NewReader(string(buf)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

type languagesResp struct {
	Languages []string `json:"languages"`
}

type gameResp struct {
	ID           int               `json:"id"`
	Lang         string            `json:"lang"`
	WordLength   int               `json:"word_length"`
	Status       string            `json:"status"`
	RTL          bool              `json:"rtl"`
	KeyboardRows [][]string        `json:"keyboard_rows"`
	OverflowBase []string          `json:"overflow_bases"`
	Equivalences [][]string        `json:"equivalences"`
	KeyStates    map[string]string `json:"key_states"`
	Error        string            `json:"error"`
}

type guessResp struct {
	Attempt     int               `json:"attempt"`
	Word        string            `json:"word"`
	Tiles       []string          `json:"tiles"`
	States      []string          `json:"states"`
	Status      string            `json:"status"`
	Message     string            `json:"message"`
	KeyStates   map[string]string `json:"key_states"`
	Answer      string            `json:"answer"`
	AnswerChars string            `json:"answer_chars"`
	Definitions []string          `json:"definitions"`
	Etymology   string            `json:"etymology"`
	Error       string            `json:"error"`
}

type statsResp struct {
	GamesPlayed   int            `json:"games_played"`
	GamesWon      int            `json:"games_won"`
	WinPct        int            `json:"win_pct"`
	CurrentStreak int            `json:"current_streak"`
	MaxStreak     int            `json:"max_streak"`
	Distribution  map[string]int `json:"distribution"`
}

func main() {
	baseURL := cmp.Or(os.Getenv("WORDGO_URL"), "http://localhost:8080")
	client := &apiClient{baseURL: baseURL, http: &http.Client{Timeout: 65 * time.Second}}
	reader := bufio.NewReader(os.Stdin)
	var history []string

	fmt.Printf("%s%swordgo%s — terminal Wordle\n", bold, fgCyan, reset)

	// Playing from the terminal should not require starting the web server
	// first. If nothing answers at baseURL, serve the same API in-process on
	// a loopback port and talk to that instead.
	if !serverUp(client) {
		embedded, err := startEmbeddedServer()
		if err != nil {
			fmt.Printf("%sNo server at %s and could not start one: %v%s\n", fgRed, baseURL, err, reset)
			os.Exit(1)
		}
		client.baseURL = embedded
		fmt.Printf("%sNo server at %s, using a built-in one.%s\n", dim, baseURL, reset)
		baseURL = embedded
	}
	fmt.Printf("Server: %s\n\n", baseURL)

	for {
		language := promptLanguage(reader, client)
		game, err := startGame(client, language)
		if err != nil {
			fmt.Printf("%sCould not start game: %v%s\n", fgRed, err, reset)
			continue
		}
		playGame(reader, client, game, &history)

		fmt.Print("\nPlay in a different language? [Y/n] ")
		again, _ := reader.ReadString('\n')
		if strings.EqualFold(strings.TrimSpace(again), "n") {
			break
		}
		fmt.Println()
	}
}

// serverUp reports whether an API is already answering at the client's base URL.
func serverUp(client *apiClient) bool {
	probe := &http.Client{Timeout: 2 * time.Second}
	resp, err := probe.Get(client.baseURL + "/api/languages")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// startEmbeddedServer runs the API in this process on a free loopback port and
// returns its base URL. Server logging is discarded so it cannot scribble over
// the board.
func startEmbeddedServer() (string, error) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	log.SetOutput(io.Discard)
	store.Init()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: server.APIMux()}
	go srv.Serve(ln)
	return "http://" + ln.Addr().String(), nil
}

func promptLanguage(reader *bufio.Reader, client *apiClient) string {
	for {
		fmt.Print("Language (default: English, or 'list' to browse, 'quit' to exit): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return "English"
		}
		if strings.EqualFold(line, "quit") || strings.EqualFold(line, "exit") {
			fmt.Println()
			os.Exit(0)
		}
		if strings.EqualFold(line, "list") {
			var langs languagesResp
			if err := client.get("/api/languages", &langs); err != nil {
				fmt.Printf("%sCould not fetch language list: %v%s\n", fgRed, err, reset)
				continue
			}
			slices.Sort(langs.Languages)
			for i, l := range langs.Languages {
				fmt.Printf("%-28s", l)
				if (i+1)%3 == 0 {
					fmt.Println()
				}
			}
			fmt.Println()
			continue
		}
		return line
	}
}

func startGame(client *apiClient, language string) (*gameResp, error) {
	var g gameResp
	if err := client.post("/api/game", map[string]string{"lang": language}, &g); err != nil {
		return nil, err
	}
	if g.Error != "" {
		return nil, errors.New(g.Error)
	}
	return &g, nil
}

func playGame(reader *bufio.Reader, client *apiClient, game *gameResp, history *[]string) {
	fmt.Printf("\n%sNew game: %s, %d characters.%s Type a guess and press Enter (Up/Down for previous guesses, 'chars' to reprint the character list, or 'quit').\n",
		bold, game.Lang, game.WordLength, reset)
	fmt.Printf("%s\n", legend())
	printPasteChars(game)
	fmt.Println()

	for game.Status == "playing" {
		word, ok := readLine(reader, history, "Guess: ")
		if !ok {
			return
		}
		if word == "" {
			continue
		}
		if strings.EqualFold(word, "quit") || strings.EqualFold(word, "exit") {
			return
		}
		if strings.EqualFold(word, "chars") || strings.EqualFold(word, "keys") {
			printPasteChars(game)
			fmt.Println()
			continue
		}

		var result guessResp
		if err := client.post(fmt.Sprintf("/api/game/%d/guess", game.ID), map[string]string{"word": word}, &result); err != nil {
			fmt.Printf("%sNetwork error: %v%s\n", fgRed, err, reset)
			continue
		}
		if result.Error != "" {
			fmt.Printf("%s%s%s\n", fgRed, result.Error, reset)
			continue
		}

		printRow(result.Tiles, result.States, game.RTL)
		printKeyStates(result.KeyStates)
		fmt.Println()

		game.Status = result.Status
		switch result.Status {
		case "won":
			fmt.Printf("%s%s%s\n", bold, result.Message, reset)
			printReveal(&result)
			printStats(client, game)
		case "lost":
			fmt.Printf("%s%s%s\n", bold, strings.ToUpper(result.Answer), reset)
			printReveal(&result)
			printStats(client, game)
		}
	}
}

// needsPaste reports whether a character has to be pasted rather than typed
// on a US qwerty keyboard: it is not ASCII itself, and the game does not
// accept its plain-ASCII form either. Guess matching is accent-insensitive
// (lang.NormalizeChar), so é, ü and ư are all typeable as e, u and u and are
// left out; ı, ø, æ, か and я have no ASCII form the game accepts, so they
// are the ones worth listing.
func needsPaste(ch string) bool {
	return !isASCII(ch) && !isASCII(lang.NormalizeChar(ch))
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// printPasteChars writes out every character of this game's keyboard that
// cannot be typed on a US qwerty keyboard, row by row, so a player without
// the language's keyboard layout installed can copy and paste them. Overflow
// characters (the ones no layout key covers) get their own line: those are
// typed as "*", but the literal character is accepted too.
func printPasteChars(game *gameResp) {
	var rows [][]string
	for _, row := range game.KeyboardRows {
		var out []string
		for _, key := range row {
			if needsPaste(key) {
				out = append(out, key)
			}
		}
		if len(out) > 0 {
			rows = append(rows, out)
		}
	}

	// The overflow bases stand for every alphabet character no keyboard key
	// covers, so they are the only ones left to list.
	var overflow []string
	for _, base := range game.OverflowBase {
		if needsPaste(base) {
			overflow = append(overflow, base)
		}
	}

	if len(rows) == 0 && len(overflow) == 0 {
		return
	}

	fmt.Printf("%sNon-qwerty characters used:%s\n", dim, reset)
	for _, row := range rows {
		fmt.Printf("  %s\n", strings.Join(row, " "))
	}
	if len(overflow) > 0 {
		fmt.Printf("  %s\"*\" for any of:%s %s\n", dim, reset, strings.Join(overflow, " "))
	}
}

func printRow(tiles, states []string, rtl bool) {
	if rtl {
		tiles = reversed(tiles)
		states = reversed(states)
	}
	var b strings.Builder
	for i, t := range tiles {
		st := ""
		if i < len(states) {
			st = states[i]
		}
		fmt.Fprintf(&b, "%s %s %s ", tileColor(st), strings.ToUpper(t), reset)
	}
	fmt.Println(b.String())
}

func reversed(in []string) []string {
	out := slices.Clone(in)
	slices.Reverse(out)
	return out
}

func printKeyStates(keyStates map[string]string) {
	if len(keyStates) == 0 {
		return
	}
	keys := slices.Sorted(maps.Keys(keyStates))
	fmt.Printf("%sLetters:%s ", dim, reset)
	for _, k := range keys {
		fmt.Printf("%s%s%s ", tileColor(keyStates[k]), strings.ToUpper(k), reset)
	}
	fmt.Println()
}

func printReveal(result *guessResp) {
	if word := result.AnswerChars; word != "" {
		fmt.Printf("(%s)\n", word)
	}
	if len(result.Definitions) > 0 {
		for i, d := range result.Definitions {
			if len(result.Definitions) > 1 {
				fmt.Printf("%d. %s\n", i+1, d)
			} else {
				fmt.Println(d)
			}
		}
	}
	if result.Etymology != "" {
		fmt.Printf("%sEtymology: %s%s\n", dim, result.Etymology, reset)
	}
}

func printStats(client *apiClient, game *gameResp) {
	var s statsResp
	// The language goes through QueryEscape: names like "Chinese (Mandarin)"
	// carry spaces and parentheses, which would otherwise make an invalid
	// URL and leave stats silently unprinted.
	path := fmt.Sprintf("/api/stats?lang=%s&length=%d", url.QueryEscape(game.Lang), game.WordLength)
	if err := client.get(path, &s); err != nil {
		return
	}
	fmt.Printf("\n%sStats for %s (%d): %d played, %d%% won, streak %d (max %d)%s\n",
		dim, game.Lang, game.WordLength, s.GamesPlayed, s.WinPct, s.CurrentStreak, s.MaxStreak, reset)
}

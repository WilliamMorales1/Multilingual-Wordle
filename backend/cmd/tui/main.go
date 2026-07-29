// Command tui is a simple terminal frontend for wordgo. It talks to the same
// HTTP API the web frontend uses, so all game logic (validation, evaluation,
// win/loss, letter-state aggregation, stats) lives in the backend and this
// client only has to render what the server sends back.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	reset    = "\x1b[0m"
	bold     = "\x1b[1m"
	dim      = "\x1b[2m"
	fgRed    = "\x1b[31m"
	fgCyan   = "\x1b[36m"
	bgGreen  = "\x1b[42;30m"
	bgYellow = "\x1b[43;30m"
	bgGray   = "\x1b[100;97m"
)

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
	baseURL := os.Getenv("WORDGO_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	client := &apiClient{baseURL: baseURL, http: &http.Client{Timeout: 65 * time.Second}}
	reader := bufio.NewReader(os.Stdin)
	var history []string

	fmt.Printf("%s%swordgo%s — terminal Wordle\n", bold, fgCyan, reset)
	fmt.Printf("Server: %s\n\n", baseURL)

	for {
		lang := promptLanguage(reader, client)
		game, err := startGame(client, lang)
		if err != nil {
			fmt.Printf("%sCould not start game: %v%s\n", fgRed, err, reset)
			continue
		}
		playGame(reader, client, game, &history)

		fmt.Print("\nPlay in a differnet language? [Y/n] ")
		again, _ := reader.ReadString('\n')
		if strings.EqualFold(strings.TrimSpace(again), "n") {
			break
		}
		fmt.Println()
	}
}

func promptLanguage(reader *bufio.Reader, client *apiClient) string {
	for {
		fmt.Print("Language (default: English, or 'list' to browse): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return "English"
		}
		if strings.EqualFold(line, "list") {
			var langs languagesResp
			if err := client.get("/api/languages", &langs); err != nil {
				fmt.Printf("%sCould not fetch language list: %v%s\n", fgRed, err, reset)
				continue
			}
			sort.Strings(langs.Languages)
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

func startGame(client *apiClient, lang string) (*gameResp, error) {
	var g gameResp
	if err := client.post("/api/game", map[string]string{"lang": lang}, &g); err != nil {
		return nil, err
	}
	if g.Error != "" {
		return nil, errors.New(g.Error)
	}
	return &g, nil
}

func playGame(reader *bufio.Reader, client *apiClient, game *gameResp, history *[]string) {
	fmt.Printf("\n%sNew game: %s, %d characters.%s Type a guess and press Enter (Up/Down for previous guesses, or 'quit').\n\n",
		bold, game.Lang, game.WordLength, reset)

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
		if result.Status == "won" {
			fmt.Printf("%s%s%s\n", bold, result.Message, reset)
			printReveal(&result)
			printStats(client, game)
		} else if result.Status == "lost" {
			fmt.Printf("%s%s%s\n", bold, strings.ToUpper(result.Answer), reset)
			printReveal(&result)
			printStats(client, game)
		}
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
	out := make([]string, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

func printKeyStates(keyStates map[string]string) {
	if len(keyStates) == 0 {
		return
	}
	keys := make([]string, 0, len(keyStates))
	for k := range keyStates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
	if err := client.get(fmt.Sprintf("/api/stats?lang=%s&length=%d", game.Lang, game.WordLength), &s); err != nil {
		return
	}
	fmt.Printf("\n%sStats for %s (%d): %d played, %d%% won, streak %d (max %d)%s\n",
		dim, game.Lang, game.WordLength, s.GamesPlayed, s.WinPct, s.CurrentStreak, s.MaxStreak, reset)
}

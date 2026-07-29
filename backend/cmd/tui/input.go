package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// readLine reads one line of input, echoing as the user types. When stdin is
// a real terminal it puts the terminal in raw mode so it can also handle
// Up/Down arrows itself: Up/Down walk back and forth through history,
// mirroring shell-style command recall. ok is false on EOF or Ctrl+C/Ctrl+D.
func readLine(reader *bufio.Reader, history *[]string, prompt string) (line string, ok bool) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Print(prompt)
		raw, err := reader.ReadString('\n')
		if err != nil {
			return "", false
		}
		line = strings.TrimSpace(raw)
		if line != "" {
			*history = append(*history, line)
		}
		return line, true
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Print(prompt)
		raw, err := reader.ReadString('\n')
		if err != nil {
			return "", false
		}
		line = strings.TrimSpace(raw)
		if line != "" {
			*history = append(*history, line)
		}
		return line, true
	}
	defer term.Restore(fd, oldState)

	fmt.Print(prompt)
	var buf []rune
	histIdx := len(*history)

	redraw := func() {
		fmt.Print("\r\x1b[K")
		fmt.Print(prompt)
		fmt.Print(string(buf))
	}

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			fmt.Print("\r\n")
			return "", false
		}

		switch r {
		case '\r', '\n':
			fmt.Print("\r\n")
			line = strings.TrimSpace(string(buf))
			if line != "" {
				*history = append(*history, line)
			}
			return line, true

		case 3: // Ctrl+C
			fmt.Print("\r\n")
			return "", false

		case 4: // Ctrl+D
			if len(buf) == 0 {
				fmt.Print("\r\n")
				return "", false
			}

		case 127, 8: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				redraw()
			}

		case 27: // ESC — check for an arrow-key sequence (ESC [ A/B/C/D)
			r2, _, err := reader.ReadRune()
			if err != nil || r2 != '[' {
				continue
			}
			r3, _, err := reader.ReadRune()
			if err != nil {
				continue
			}
			switch r3 {
			case 'A': // Up: step back to the previous history entry
				if histIdx > 0 {
					histIdx--
					buf = []rune((*history)[histIdx])
					redraw()
				}
			case 'B': // Down: step forward, back to a blank line past the newest entry
				if histIdx < len(*history)-1 {
					histIdx++
					buf = []rune((*history)[histIdx])
					redraw()
				} else if histIdx < len(*history) {
					histIdx++
					buf = nil
					redraw()
				}
			}

		default:
			if r >= 32 {
				buf = append(buf, r)
				redraw()
			}
		}
	}
}

//go:build !windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const maxVisibleEntries = 10

type unixHistorySelector struct {
	tty    *os.File
	writer io.Writer
	reader *bufio.Reader
	close  bool
}

func newHistorySelector(stderr io.Writer) historySelector {
	if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) != 0 {
		return &unixHistorySelector{
			tty:    os.Stdin,
			writer: stderr,
			reader: bufio.NewReader(os.Stdin),
		}
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		return &unixHistorySelector{
			tty:    tty,
			writer: tty,
			reader: bufio.NewReader(tty),
			close:  true,
		}
	}

	return &lineHistorySelector{reader: bufio.NewReader(os.Stdin), writer: stderr}
}

func (s *unixHistorySelector) Select(entries []string) (string, error) {
	if s.close {
		defer s.tty.Close()
	}

	state, err := s.stty("-g")
	if err != nil {
		return "", err
	}
	if _, err := s.stty("raw", "-echo", "min", "1", "time", "0"); err != nil {
		return "", err
	}
	defer func() {
		_, _ = s.stty(state)
		fmt.Fprint(s.writer, "\r\x1b[J\x1b[?25h\n")
	}()

	fmt.Fprint(s.writer, "\x1b[?25l")

	view := selectorState{}
	lastLines := 0

	for {
		filtered := view.filtered(entries)
		if len(filtered) > 0 && view.cursor >= len(filtered) {
			view.cursor = len(filtered) - 1
		}
		if view.cursor < 0 {
			view.cursor = 0
		}

		lastLines = renderSelector(s.writer, lastLines, view, filtered)

		key, err := s.readKey()
		if err != nil {
			return "", err
		}

		switch key.kind {
		case keyUp:
			if len(filtered) > 0 {
				view.cursor--
				if view.cursor < 0 {
					view.cursor = len(filtered) - 1
				}
			}
		case keyDown:
			if len(filtered) > 0 {
				view.cursor++
				if view.cursor >= len(filtered) {
					view.cursor = 0
				}
			}
		case keyEnter:
			if len(filtered) == 0 {
				continue
			}
			return filtered[view.cursor], nil
		case keyBackspace:
			if len(view.query) > 0 {
				view.query = view.query[:len(view.query)-1]
				view.cursor = 0
			}
		case keyAbort:
			return "", errAborted
		default:
			if key.printable != "" {
				view.query += key.printable
				view.cursor = 0
			}
		}
	}
}

func (s *unixHistorySelector) stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = s.tty
	cmd.Stderr = io.Discard

	var out strings.Builder
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("stty %v: %w", args, err)
	}

	return strings.TrimSpace(out.String()), nil
}

type inputKey struct {
	printable string
	kind      keyKind
}

type keyKind int

const (
	keyPrintable keyKind = iota
	keyUp
	keyDown
	keyEnter
	keyBackspace
	keyAbort
)

func (s *unixHistorySelector) readKey() (inputKey, error) {
	b, err := s.reader.ReadByte()
	if err != nil {
		return inputKey{}, err
	}

	switch b {
	case 14:
		return inputKey{kind: keyDown}, nil
	case 16:
		return inputKey{kind: keyUp}, nil
	case 3, 27:
		if b == 27 {
			next, err := s.reader.Peek(2)
			if err == nil && len(next) == 2 && next[0] == '[' {
				_, _ = s.reader.Discard(2)
				switch next[1] {
				case 'A':
					return inputKey{kind: keyUp}, nil
				case 'B':
					return inputKey{kind: keyDown}, nil
				}
			}
		}
		return inputKey{kind: keyAbort}, nil
	case 13, 10:
		return inputKey{kind: keyEnter}, nil
	case 127, 8:
		return inputKey{kind: keyBackspace}, nil
	default:
		if b >= 32 {
			return inputKey{kind: keyPrintable, printable: string([]byte{b})}, nil
		}
		return inputKey{}, nil
	}
}

func renderSelector(w io.Writer, lastLines int, state selectorState, filtered []string) int {
	if lastLines > 0 {
		fmt.Fprintf(w, "\x1b[%dA\r", lastLines)
	}
	fmt.Fprint(w, "\x1b[J")

	lines := 0
	lines += linef(w, "Select a directory")
	lines += linef(w, "Search: %s", state.query)
	lines += linef(w, "Keys: Up/Down or Ctrl-P/Ctrl-N to move, Enter to select, Backspace to filter, Esc/Ctrl-C to cancel")

	if len(filtered) == 0 {
		lines += linef(w, "  no matches")
		return lines
	}

	start := 0
	if state.cursor >= maxVisibleEntries {
		start = state.cursor - maxVisibleEntries + 1
	}
	end := start + maxVisibleEntries
	if end > len(filtered) {
		end = len(filtered)
	}

	for i := start; i < end; i++ {
		prefix := "  "
		if i == state.cursor {
			prefix = "> "
		}
		lines += linef(w, "%s%s", prefix, filtered[i])
	}

	if end < len(filtered) {
		lines += linef(w, "  ... %d more", len(filtered)-end)
	}

	return lines
}

func linef(w io.Writer, format string, args ...any) int {
	fmt.Fprintf(w, "\r"+format+"\r\n", args...)
	return 1
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type historySelector interface {
	Select(entries []string) (string, error)
}

func chooseFromHistory(historyFile string, stderr io.Writer) (string, error) {
	entries, err := readHistory(historyFile)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("cd history is empty")
	}

	return newHistorySelector(stderr).Select(entries)
}

type lineHistorySelector struct {
	reader *bufio.Reader
	writer io.Writer
}

func (s *lineHistorySelector) Select(entries []string) (string, error) {
	fmt.Fprintln(s.writer, "Select a directory:")
	for i, entry := range entries {
		fmt.Fprintf(s.writer, "  %d) %s\n", i+1, entry)
	}
	fmt.Fprint(s.writer, "> ")

	line, err := s.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read selection: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errAborted
	}

	index, err := strconv.Atoi(line)
	if err != nil {
		return "", fmt.Errorf("invalid selection %q", line)
	}
	if index < 1 || index > len(entries) {
		return "", fmt.Errorf("selection out of range: %d", index)
	}

	return entries[index-1], nil
}

type selectorState struct {
	query  string
	cursor int
}

func (s selectorState) filtered(entries []string) []string {
	if s.query == "" {
		return entries
	}

	query := strings.ToLower(s.query)
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry), query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

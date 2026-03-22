//go:build windows

package main

import (
	"bufio"
	"io"
	"os"
)

func newHistorySelector(stderr io.Writer) historySelector {
	return &lineHistorySelector{reader: bufio.NewReader(os.Stdin), writer: stderr}
}

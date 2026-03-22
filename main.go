package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultHistoryLimit = 200
	envHistoryFile      = "FOOTCD_HISTORY_FILE"
	envHistoryLimit     = "FOOTCD_HISTORY_LIMIT"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	args = normalizeArgs(args)

	fs := flag.NewFlagSet("footcd", flag.ContinueOnError)
	fs.SetOutput(stderr)

	historyFileFlag := fs.String("history-file", "", "history file path")
	limitFlag := fs.Int("history-limit", historyLimit(), "maximum history entries to keep")
	versionFlag := fs.Bool("version", false, "show version")
	versionShortFlag := fs.Bool("v", false, "show version")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *versionFlag || *versionShortFlag {
		fmt.Fprintln(stdout, version)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "usage: footcd [--history-file PATH] [--history-limit N] <init SHELL|select|record DIR>")
		return 2
	}

	historyFile, err := resolveHistoryFile(*historyFileFlag)
	if err != nil {
		fmt.Fprintf(stderr, "resolve history file: %v\n", err)
		return 1
	}

	switch rest[0] {
	case "init":
		if len(rest) != 2 {
			fmt.Fprintln(stderr, "usage: footcd init <bash|zsh|sh>")
			return 2
		}
		script, err := initScript(rest[1], filepath.Base(os.Args[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	case "select":
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "usage: footcd [--history-file PATH] select")
			return 2
		}
		target, err := chooseFromHistory(historyFile, stderr)
		if err != nil {
			if errors.Is(err, errAborted) {
				return 1
			}
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, target)
		return 0
	case "record":
		if len(rest) != 2 {
			fmt.Fprintln(stderr, "usage: footcd [--history-file PATH] [--history-limit N] record <dir>")
			return 2
		}
		target, err := resolveExistingDir(rest[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := appendHistory(historyFile, target, *limitFlag); err != nil {
			fmt.Fprintf(stderr, "update history: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(stderr, "usage: footcd [--history-file PATH] <init SHELL|select|record DIR>")
		return 2
	}
}

var errAborted = errors.New("selection aborted")

func resolveExistingDir(arg string) (string, error) {
	target, err := filepath.Abs(arg)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", target, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", target)
	}

	return target, nil
}

func resolveHistoryFile(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if envValue := os.Getenv(envHistoryFile); envValue != "" {
		return filepath.Abs(envValue)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "footcd", "history"), nil
}

func normalizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--version" {
			out = append(out, "-version")
			continue
		}
		out = append(out, arg)
	}
	return out
}

func initScript(shell, cmdName string) (string, error) {
	switch shell {
	case "bash", "zsh", "sh":
		return fmt.Sprintf(`cd() {
  local rc target;

  if [ "$#" -eq 1 ] && [ "$1" = "-" ]; then
    target="$(command %s select)" || return $?;
    builtin cd "$target" || return $?;
    command %s record "$PWD" >/dev/null 2>&1 || true;
    return 0;
  fi;

  builtin cd "$@";
  rc=$?;
  if [ "$rc" -eq 0 ]; then
    command %s record "$PWD" >/dev/null 2>&1 || true;
  fi;
  return "$rc";
}
`, cmdName, cmdName, cmdName), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func historyLimit() int {
	if raw := os.Getenv(envHistoryLimit); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultHistoryLimit
}

func readHistory(historyFile string) ([]string, error) {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read history file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	seen := make(map[string]struct{}, len(lines))
	entries := make([]string, 0, len(lines))

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if _, err := os.Stat(line); err != nil {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		entries = append(entries, line)
	}

	return entries, nil
}

func appendHistory(historyFile, dir string, limit int) error {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	entries, err := readHistory(historyFile)
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(entries)+1)
	filtered = append(filtered, dir)
	for _, entry := range entries {
		if entry != dir {
			filtered = append(filtered, entry)
		}
		if len(filtered) >= limit {
			break
		}
	}

	if err := os.MkdirAll(filepath.Dir(historyFile), 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}

	content := strings.Join(reverse(filtered), "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(historyFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write history file: %w", err)
	}
	return nil
}

func reverse(values []string) []string {
	out := make([]string, len(values))
	for i := range values {
		out[len(values)-1-i] = values[i]
	}
	return out
}

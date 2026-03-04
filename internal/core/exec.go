package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fatal prints an error message and exits.
func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

// MustAbs returns the absolute path, or fatals.
func MustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		Fatal("cannot resolve path: %s", path)
	}
	return abs
}

// RunCmd runs a command and returns trimmed stdout.
func RunCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// Git runs a git command in the given directory and returns output.
func Git(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	return RunCmd("git", fullArgs...)
}

// GitMust runs git and returns output, ignoring errors.
func GitMust(dir string, args ...string) string {
	out, _ := Git(dir, args...)
	return out
}

// TmuxRun runs a tmux command.
func TmuxRun(args ...string) error {
	_, err := RunCmd("tmux", args...)
	return err
}

// TmuxQuery runs a tmux command and returns its trimmed output.
func TmuxQuery(args ...string) (string, error) {
	return RunCmd("tmux", args...)
}

// ReadFileContent reads a file and returns its trimmed contents, or empty string on error.
func ReadFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteFileContent writes content to a file.
func WriteFileContent(path, content string) error {
	return os.WriteFile(path, []byte(content+"\n"), 0644)
}

// IsDir checks if a path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}



package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fatal prints an error message and exits.
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

// mustAbs returns the absolute path, or fatals.
func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		fatal("cannot resolve path: %s", path)
	}
	return abs
}

// runCmd runs a command and returns trimmed stdout.
func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// runCmdPassthrough runs a command with terminal I/O attached.
func runCmdPassthrough(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// git runs a git command in the given directory and returns output.
func git(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	return runCmd("git", fullArgs...)
}

// gitMust runs git and returns output, ignoring errors.
func gitMust(dir string, args ...string) string {
	out, _ := git(dir, args...)
	return out
}

// tmuxRun runs a tmux command.
func tmuxRun(args ...string) error {
	_, err := runCmd("tmux", args...)
	return err
}

// promptUser shows a prompt and reads a line from stdin.
func promptUser(msg string) string {
	fmt.Print(msg)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// readFile reads a file and returns its trimmed contents, or empty string on error.
func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeFileContent writes content to a file.
func writeFileContent(path, content string) error {
	return os.WriteFile(path, []byte(content+"\n"), 0644)
}

// isDir checks if a path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AgentDef describes how to invoke a particular agent CLI tool.
// To add a new agent, add an entry to the knownAgents slice below.
type AgentDef struct {
	ID             string // unique key, e.g. "auggie"
	DisplayName    string // shown in picker, e.g. "Auggie (Augment Code)"
	Command        string // binary name, e.g. "auggie"
	PromptFileFlag string // flag to pass an instruction file, e.g. "--instruction-file"
	InlineFlag     string // flag to pass inline prompt text, e.g. "--print"
}

// knownAgents is the registry of supported agent CLIs.
// To support a new agent, add an entry here — no other file needs to change.
var knownAgents = []AgentDef{
	{
		ID:             "auggie",
		DisplayName:    "Auggie (Augment Code)",
		Command:        "auggie",
		PromptFileFlag: "--instruction-file",
		InlineFlag:     "--print",
	},
	{
		ID:             "cursor",
		DisplayName:    "Cursor (Cursor AI)",
		Command:        "agent",
		PromptFileFlag: "", // Cursor takes prompt as positional arg; handled via $(cat file)
		InlineFlag:     "-p",
	},
}

// agentConfig is the persisted user config.
type agentConfig struct {
	Agent string `json:"agent"`
}

// configPath returns ~/.config/augmux/config.json.
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".config", "augmux", "config.json")
}

// loadConfig reads the config file. Returns nil if it doesn't exist.
func loadConfig() *agentConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil
	}
	var cfg agentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// saveConfig writes the config file.
func saveConfig(cfg *agentConfig) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// findAgent looks up an AgentDef by ID.
func findAgent(id string) *AgentDef {
	for i := range knownAgents {
		if knownAgents[i].ID == id {
			return &knownAgents[i]
		}
	}
	return nil
}

// promptAgentSetup asks the user to pick an agent CLI and saves the config.
func promptAgentSetup() *AgentDef {
	var options []string
	for _, a := range knownAgents {
		options = append(options, a.DisplayName)
	}

	title := fmt.Sprintf("No agent CLI configured — select one:\n(config will be saved to %s)", configPath())
	choice := runMenu(title, options)
	if choice < 0 || choice >= len(knownAgents) {
		fatal("No agent selected.")
	}

	agent := &knownAgents[choice]
	if err := saveConfig(&agentConfig{Agent: agent.ID}); err != nil {
		fatal("Failed to save config: %v", err)
	}
	fmt.Printf("\n  ✓ Configured to use %s (%s)\n", agent.DisplayName, agent.Command)
	fmt.Printf("    Config saved to %s\n\n", configPath())
	return agent
}

// activeAgent returns the configured agent, prompting for setup if needed.
func activeAgent() *AgentDef {
	cfg := loadConfig()
	if cfg != nil {
		if a := findAgent(cfg.Agent); a != nil {
			return a
		}
		fmt.Fprintf(os.Stderr, "Warning: configured agent %q not found, reconfiguring...\n\n", cfg.Agent)
	}
	return promptAgentSetup()
}

// --- Command builders: all agent CLI coupling lives here ---

// AgentSpawnCmd returns the shell command string to type into a tmux window
// to start the agent interactively (no initial prompt).
func (a *AgentDef) SpawnCmd() string {
	return a.Command
}

// SpawnWithPromptCmd returns the shell command string to start the agent
// with an instruction file.
func (a *AgentDef) SpawnWithPromptCmd(promptFile string) string {
	if a.PromptFileFlag != "" {
		return fmt.Sprintf("%s %s '%s'", a.Command, a.PromptFileFlag, promptFile)
	}
	// Agent takes prompt as positional arg — read file content via shell substitution
	return fmt.Sprintf("%s \"$(cat '%s')\"", a.Command, promptFile)
}

// RunInline runs the agent synchronously with an inline prompt, attaching
// stdout/stderr to the terminal. Returns any error.
func (a *AgentDef) RunInline(dir, promptText string) error {
	args := []string{a.InlineFlag, promptText}
	if a.ID == "cursor" {
		// Cursor print mode: agent -p --force "prompt"
		args = []string{a.InlineFlag, "--force", promptText}
	}
	cmd := exec.Command(a.Command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AgentDisplayName returns a user-friendly name for the configured agent.
func (a *AgentDef) Label() string {
	return strings.Split(a.DisplayName, " (")[0] // e.g. "Auggie"
}


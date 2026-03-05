package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xemotrix/augmux/internal/components"
	"github.com/xemotrix/augmux/internal/core"
)

// AgentDef describes how to invoke a particular agent CLI tool.
// To add a new agent, add an entry to the knownAgents slice below.
type AgentDef struct {
	ID          string // unique key, e.g. "auggie"
	DisplayName string // shown in picker, e.g. "Auggie (Augment Code)"
	Command     string // binary name, e.g. "auggie"
	InlineFlag  string // flag to pass inline prompt text, e.g. "--print"
}

// knownAgents is the registry of supported agent CLIs.
// To support a new agent, add an entry here — no other file needs to change.
var knownAgents = []AgentDef{
	{
		ID:          "auggie",
		DisplayName: "Auggie (Augment Code)",
		Command:     "auggie",
		InlineFlag:  "--print",
	},
	{
		ID:          "cursor",
		DisplayName: "Cursor (Cursor AI)",
		Command:     "agent",
		InlineFlag:  "-p",
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
		core.Fatal("cannot determine home directory: %v", err)
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
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
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
	choice := components.RunMenu(title, options)
	if choice < 0 || choice >= len(knownAgents) {
		core.Fatal("No agent selected.")
	}

	agent := &knownAgents[choice]
	if err := saveConfig(&agentConfig{Agent: agent.ID}); err != nil {
		core.Fatal("Failed to save config: %v", err)
	}
	fmt.Printf("\n  ✓ Configured to use %s (%s)\n", agent.DisplayName, agent.Command)
	fmt.Printf("    Config saved to %s\n\n", configPath())
	return agent
}

// ActiveAgent returns the configured agent, prompting for setup if needed.
func ActiveAgent() *AgentDef {
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

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xemotrix/augmux/internal/components"
	"github.com/xemotrix/augmux/internal/core"
)

// AgentDef describes how to invoke a particular agent CLI tool.
// To add a new agent, add an entry to the knownAgents slice below.
type AgentDef struct {
	ID          string // unique key, e.g. "auggie"
	DisplayName string // shown in picker, e.g. "Auggie (Augment Code)"
	Command     string // binary name, e.g. "auggie"
}

// knownAgents is the registry of supported agent CLIs.
// To support a new agent, add an entry here — no other file needs to change.
var knownAgents = []AgentDef{
	{
		ID:          "auggie",
		DisplayName: "Auggie (Augment Code)",
		Command:     "auggie",
	},
	{
		ID:          "cursor",
		DisplayName: "Cursor (Cursor AI)",
		Command:     "agent",
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
	choice := components.RunSelectMenu(title, options)
	if choice < 0 || choice >= len(knownAgents) {
		core.Fatal("No agent selected.")
	}

	agent := &knownAgents[choice]
	if err := saveConfig(&agentConfig{Agent: agent.ID}); err != nil {
		core.Fatal("Failed to save config: %v", err)
	}
	return agent
}

// IsConfigured returns true if a valid agent config exists.
func IsConfigured() bool {
	cfg := loadConfig()
	return cfg != nil && findAgent(cfg.Agent) != nil
}

// KnownAgentDefs returns the list of supported agent CLIs.
func KnownAgentDefs() []AgentDef {
	return knownAgents
}

// SaveAgentChoice persists the given agent ID to the config file.
func SaveAgentChoice(id string) error {
	return saveConfig(&agentConfig{Agent: id})
}

// ActiveAgent returns the configured agent, prompting for setup if needed.
func ActiveAgent() *AgentDef {
	cfg := loadConfig()
	if cfg != nil {
		if a := findAgent(cfg.Agent); a != nil {
			return a
		}
	}
	return promptAgentSetup()
}


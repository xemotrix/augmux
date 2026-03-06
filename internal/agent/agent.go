package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "augmux", "config.json"), nil
}

// loadConfig reads the config file. Returns nil if it doesn't exist.
func loadConfig() *agentConfig {
	p, err := configPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
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
	p, err := configPath()
	if err != nil {
		return err
	}
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

// ConfiguredAgent returns the configured agent without prompting, or nil if not configured.
func ConfiguredAgent() *AgentDef {
	cfg := loadConfig()
	if cfg != nil {
		return findAgent(cfg.Agent)
	}
	return nil
}

// ActiveAgent returns the configured agent, or an error if none is configured.
func ActiveAgent() (*AgentDef, error) {
	cfg := loadConfig()
	if cfg != nil {
		if a := findAgent(cfg.Agent); a != nil {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no agent CLI configured; select one from the TUI first")
}


// Package agentcli handles the external cli used with augmux
// (Cursor's 'agent', Augment's 'auggie', etc.)
package agentcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentCliDef describes how to invoke a particular agent CLI tool.
// To add a new agent, add an entry to the knownAgents slice below.
type AgentCliDef struct {
	ID          string // unique key, e.g. "auggie"
	DisplayName string // shown in picker, e.g. "Auggie (Augment Code)"
	Command     string // binary name, e.g. "auggie"
}

// knownAgents is the registry of supported agent CLIs.
// To support a new agent, add an entry here — no other file needs to change.
var knownAgents = []AgentCliDef{
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

// agentCliConfig is the persisted user config.
type agentCliConfig struct {
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
func loadConfig() *agentCliConfig {
	p, err := configPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cfg agentCliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// saveConfig writes the config file.
func saveConfig(cfg *agentCliConfig) error {
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

// findAgentCli looks up an AgentCliDef by ID.
func findAgentCli(id string) *AgentCliDef {
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
	return cfg != nil && findAgentCli(cfg.Agent) != nil
}

// KnownAgentCliDefs returns the list of supported agent CLIs.
func KnownAgentCliDefs() []AgentCliDef {
	return knownAgents
}

// SaveAgentCliChoice persists the given agent ID to the config file.
func SaveAgentCliChoice(id string) error {
	return saveConfig(&agentCliConfig{Agent: id})
}

// ConfiguredAgentCli returns the configured agent without prompting, or nil if not configured.
func ConfiguredAgentCli() *AgentCliDef {
	cfg := loadConfig()
	if cfg != nil {
		return findAgentCli(cfg.Agent)
	}
	return nil
}

// ActiveAgentCli returns the configured agent, or an error if none is configured.
func ActiveAgentCli() (*AgentCliDef, error) {
	cfg := loadConfig()
	if cfg != nil {
		if a := findAgentCli(cfg.Agent); a != nil {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no agent CLI configured; select one from the TUI first")
}

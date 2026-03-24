package faction

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FactionsDir is the content directory containing faction YAML files.
type FactionsDir string

// FactionConfigPath is the path to faction_config.yaml.
type FactionConfigPath string

// ProvideRegistry loads FactionRegistry from dir.
//
// Precondition: dir must be a readable directory containing faction YAML files.
// Postcondition: Returns a non-nil *FactionRegistry or error.
func ProvideRegistry(dir FactionsDir) (*FactionRegistry, error) {
	reg, err := LoadFactions(string(dir))
	if err != nil {
		return nil, fmt.Errorf("faction.ProvideRegistry: %w", err)
	}
	return &reg, nil
}

// ProvideConfig loads FactionConfig from configPath.
//
// Precondition: configPath must point to a readable YAML file.
// Postcondition: Returns a validated FactionConfig or error.
func ProvideConfig(configPath FactionConfigPath) (FactionConfig, error) {
	data, err := os.ReadFile(string(configPath))
	if err != nil {
		return FactionConfig{}, fmt.Errorf("faction.ProvideConfig: reading %q: %w", configPath, err)
	}
	var cfg FactionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FactionConfig{}, fmt.Errorf("faction.ProvideConfig: parsing: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return FactionConfig{}, fmt.Errorf("faction.ProvideConfig: validation: %w", err)
	}
	return cfg, nil
}

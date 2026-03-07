// Package xp provides XP award logic and level-up calculations.
package xp

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Awards holds the configurable XP values for each award source.
type Awards struct {
	// KillXPPerNPCLevel is the XP multiplier per NPC level for combat kills.
	KillXPPerNPCLevel int `yaml:"kill_xp_per_npc_level"`
	// NewRoomXP is the flat XP award for discovering a previously unseen room.
	NewRoomXP int `yaml:"new_room_xp"`
	// SkillCheckSuccessXP is the base XP for a success outcome on a skill check.
	SkillCheckSuccessXP int `yaml:"skill_check_success_xp"`
	// SkillCheckCritSuccessXP is the base XP for a crit_success outcome on a skill check.
	SkillCheckCritSuccessXP int `yaml:"skill_check_crit_success_xp"`
	// SkillCheckDCMultiplier is multiplied by DC and added to skill check XP awards.
	SkillCheckDCMultiplier int `yaml:"skill_check_dc_multiplier"`
}

// XPConfig holds all configurable parameters for the levelling system.
type XPConfig struct {
	// BaseXP is the coefficient in the formula: xp_to_reach_level(n) = n² × BaseXP.
	BaseXP int `yaml:"base_xp"`
	// HPPerLevel is the max HP increase granted on each level-up.
	HPPerLevel int `yaml:"hp_per_level"`
	// BoostInterval is the level interval at which an ability boost is granted.
	BoostInterval int `yaml:"boost_interval"`
	// LevelCap is the maximum character level.
	LevelCap int `yaml:"level_cap"`
	// JobLevelCap is the maximum level for any single job (reserved for future use).
	JobLevelCap int `yaml:"job_level_cap"`
	// Awards holds per-source XP values.
	Awards Awards `yaml:"awards"`
}

// LoadXPConfig reads and parses the XP configuration from the given YAML file.
//
// Precondition: path must refer to a readable YAML file matching XPConfig.
// Postcondition: Returns a non-nil *XPConfig on success, or a non-nil error.
func LoadXPConfig(path string) (*XPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading xp config %q: %w", path, err)
	}
	var cfg XPConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing xp config %q: %w", path, err)
	}
	return &cfg, nil
}

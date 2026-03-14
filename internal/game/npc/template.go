// Package npc provides NPC template definitions and live instance management.
package npc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// EquipmentEntry represents one option in a weighted random equipment table.
type EquipmentEntry struct {
	ID     string `yaml:"id"`
	Weight int    `yaml:"weight"`
}

// Abilities holds the six core ability scores for an NPC template.
type Abilities struct {
	Brutality int `yaml:"brutality"`
	Grit      int `yaml:"grit"`
	Quickness int `yaml:"quickness"`
	Reasoning int `yaml:"reasoning"`
	Savvy     int `yaml:"savvy"`
	Flair     int `yaml:"flair"`
}

// CombatStrategy defines per-NPC-type tactical behaviors in combat.
type CombatStrategy struct {
	// UseCover indicates whether this NPC will automatically take cover
	// at the start of their turn if cover is available in the room.
	UseCover bool `yaml:"use_cover"`
}

// Template defines a reusable NPC archetype loaded from YAML.
type Template struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	// Type is the NPC category used for predators_eye passive matching (e.g. "human", "robot", "mutant").
	// Empty string means no category is defined.
	Type        string    `yaml:"type"`
	Level       int       `yaml:"level"`
	MaxHP       int       `yaml:"max_hp"`
	AC          int       `yaml:"ac"`
	Perception  int       `yaml:"perception"`
	// Deception is the NPC's deception skill modifier, used as the DC for the motive command.
	// Zero means untrained. Loaded from YAML field "deception".
	Deception   int       `yaml:"deception"`
	Abilities   Abilities `yaml:"abilities"`
	AIDomain    string    `yaml:"ai_domain"` // HTN domain ID; empty = simple attack fallback
	// RespawnDelay is the duration string (e.g. "5m", "30s") before a dead NPC
	// of this template respawns. Empty means the NPC does not respawn.
	RespawnDelay  string     `yaml:"respawn_delay"`
	Loot          *LootTable `yaml:"loot"`
	Taunts        []string   `yaml:"taunts"`
	TauntChance   float64    `yaml:"taunt_chance"`
	TauntCooldown string     `yaml:"taunt_cooldown"`
	// SkillChecks defines skill check triggers fired when a player greets this NPC.
	SkillChecks []skillcheck.TriggerDef `yaml:"skill_checks"`
	// Resistances maps damage type → flat damage reduction (minimum 0 after reduction).
	Resistances map[string]int `yaml:"resistances"`
	// Weaknesses maps damage type → flat damage addition applied on any hit.
	Weaknesses map[string]int `yaml:"weaknesses"`
	// Weapon is a weighted random table of weapon IDs. Empty = unarmed.
	Weapon []EquipmentEntry `yaml:"weapon"`
	// Armor is a weighted random table of armor IDs. Empty = no armor.
	Armor []EquipmentEntry `yaml:"armor"`
	// Combat defines the tactical strategy this NPC uses in combat.
	Combat CombatStrategy `yaml:"combat"`
	// ToughnessRank is the NPC's Toughness save proficiency rank
	// ("trained", "expert", "master", "legendary", or "" for untrained).
	// Used to compute Toughness DC for Grapple and Shove.
	ToughnessRank string `yaml:"toughness_rank"`
	// HustleRank is the NPC's Hustle save proficiency rank.
	// Used to compute Hustle DC for Trip, Disarm, and Tumble.
	HustleRank string `yaml:"hustle_rank"`
	// CoolRank is the NPC's Cool save proficiency rank.
	// Used to compute Cool DC for Demoralize.
	CoolRank string `yaml:"cool_rank"`
	// RobMultiplier controls whether and how aggressively this NPC robs defeated
	// players. 0.0 = never robs (default). 1.0 = baseline human aggression.
	// Values > 1.0 represent especially predatory NPCs.
	// Used at spawn to compute Instance.RobPercent.
	RobMultiplier float64 `yaml:"rob_multiplier"`
}

// Validate checks that the template satisfies basic invariants.
//
// Precondition: t must not be nil.
// Postcondition: Returns nil iff ID is non-empty, Name is non-empty, Level >= 1,
// MaxHP >= 1, and AC >= 10; returns an error on the first violation otherwise.
func (t *Template) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("npc template: id must not be empty")
	}
	if t.Name == "" {
		return fmt.Errorf("npc template %q: name must not be empty", t.ID)
	}
	if t.Level < 1 {
		return fmt.Errorf("npc template %q: level must be >= 1", t.ID)
	}
	if t.MaxHP < 1 {
		return fmt.Errorf("npc template %q: max_hp must be >= 1", t.ID)
	}
	if t.AC < 10 {
		return fmt.Errorf("npc template %q: ac must be >= 10", t.ID)
	}
	if t.RespawnDelay != "" {
		if _, err := time.ParseDuration(t.RespawnDelay); err != nil {
			return fmt.Errorf("npc template %q: respawn_delay %q is not a valid duration: %w", t.ID, t.RespawnDelay, err)
		}
	}
	if t.TauntChance < 0 || t.TauntChance > 1 {
		return fmt.Errorf("npc template %q: taunt_chance must be between 0 and 1", t.ID)
	}
	if t.TauntCooldown != "" {
		if _, err := time.ParseDuration(t.TauntCooldown); err != nil {
			return fmt.Errorf("npc template %q: taunt_cooldown %q is not a valid duration: %w", t.ID, t.TauntCooldown, err)
		}
	}
	if t.RobMultiplier < 0 {
		return fmt.Errorf("npc template %q: rob_multiplier must be >= 0", t.ID)
	}
	if t.Loot != nil {
		if err := t.Loot.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	}
	return nil
}

// LoadTemplateFromBytes parses a single NPC template from raw YAML bytes.
//
// Precondition: data must be valid YAML for a single Template.
// Postcondition: Returns a validated *Template, or an error. RespawnDelay, if
// non-empty, is guaranteed to be a valid Go duration string.
func LoadTemplateFromBytes(data []byte) (*Template, error) {
	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing template YAML: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// LoadTemplates reads all *.yaml files in dir and returns the parsed templates.
//
// Precondition: dir must be a readable directory.
// Postcondition: Returns all templates or an error on the first parse or validate
// failure; on error, the partial result is discarded.
func LoadTemplates(dir string) ([]*Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading npc dir %q: %w", dir, err)
	}

	var templates []*Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}

		tmpl, err := LoadTemplateFromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", path, err)
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// Package npc provides NPC template definitions and live instance management.
package npc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
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
	Awareness   int       `yaml:"awareness"`
	// Hustle is the NPC's hustle skill modifier, used as the DC for the motive command.
	// Zero means untrained. Loaded from YAML field "hustle".
	Hustle   int       `yaml:"hustle"`
	Abilities   Abilities `yaml:"abilities"`
	AIDomain    string    `yaml:"ai_domain"` // HTN domain ID; empty = simple attack fallback
	// RespawnDelay is the duration string (e.g. "5m", "30s") before a dead NPC
	// of this template respawns. Empty means the NPC does not respawn.
	RespawnDelay  string     `yaml:"respawn_delay"`
	Loot          *LootTable `yaml:"loot"`
	Taunts        []string   `yaml:"taunts"`
	TauntChance   float64    `yaml:"taunt_chance"`
	TauntCooldown string     `yaml:"taunt_cooldown"`
	// CourageThreshold is the threat score above which the NPC will not engage.
	// Default 999 preserves always-engage behavior for all existing templates.
	CourageThreshold int `yaml:"courage_threshold"`
	// FleeHPPct is the HP percentage below which the NPC attempts to flee combat. 0 = never flee.
	FleeHPPct int `yaml:"flee_hp_pct"`
	// HomeRoom is the room ID the NPC returns to when idle. Defaults to spawn room if not set.
	HomeRoom string `yaml:"home_room"`
	// WanderRadius is the maximum BFS hop distance from HomeRoom during patrol. 0 = no movement.
	WanderRadius int `yaml:"wander_radius"`
	// Schedule is an optional time-of-day behavior window list.
	// Templates without it behave using default template settings.
	Schedule []behavior.ScheduleEntry `yaml:"schedule,omitempty"`
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
	// SenseAbilities lists named special abilities for sense motive reveal.
	SenseAbilities []string `yaml:"sense_abilities"`
	// BossAbilities defines the set of special abilities for boss-tier NPCs.
	// Validated by Template.Validate().
	BossAbilities []BossAbility `yaml:"boss_abilities"`
	// Tier sets the difficulty tier for this NPC. Valid values: "minion", "standard",
	// "elite", "champion", "boss". Empty means "standard" is assumed at runtime.
	Tier string `yaml:"tier"`
	// Tags is a list of free-form content labels. Not code-enforced.
	Tags []string `yaml:"tags"`
	// Feats is a list of feat IDs assigned to this NPC. Each must be an NPC-valid feat
	// (AllowNPC == true). Validated by ValidateWithRegistry at startup.
	Feats []string `yaml:"feats"`
	// Disposition sets the initial NPC disposition: "hostile","wary","neutral","friendly".
	// Empty string defaults to "hostile" at spawn.
	Disposition string `yaml:"disposition"`
	// FactionID is the faction this NPC belongs to.
	// Empty string means no faction affiliation.
	// NPCs with a FactionID hostile to the player's faction are treated as enemies.
	FactionID string `yaml:"faction_id"`

	// NpcRole classifies the NPC for map POI display.
	// It is orthogonal to NPCType: NPCType governs behavior categories (combat, merchant, etc.)
	// while NpcRole governs the map POI symbol shown to players.
	// Valid non-empty values: "merchant", "healer", "job_trainer", "guard",
	// or any other non-empty string (maps to generic "npc" POI).
	// Empty (default) means this NPC contributes no POI to the map.
	NpcRole string `yaml:"npc_role,omitempty"`

	// AttackVerb is the verb used in combat narratives (e.g. "bites", "shoots").
	// Empty string defaults to "attacks" in combat.
	AttackVerb string `yaml:"attack_verb"`

	// Immobile prevents this NPC from patrolling or wandering.
	Immobile bool `yaml:"immobile"`

	// NPCType classifies the NPC's role.
	// Valid values: "combat", "merchant", "guard", "healer", "quest_giver",
	// "hireling", "banker", "job_trainer", "crafter".
	// Defaults to "combat" at load time if absent (REQ-NPC-1).
	NPCType string `yaml:"npc_type"`

	// Personality names the HTN preset governing non-combat flee/cower behavior.
	// Valid values: "cowardly" (always flee), "brave" (always cower),
	// "neutral" (use type default), "opportunistic" (use type default).
	// Empty string also falls through to the type default.
	Personality string `yaml:"personality"`

	// Type-specific config — at most one is non-nil for a given NPC.
	Merchant   *MerchantConfig   `yaml:"merchant,omitempty"`
	Guard      *GuardConfig      `yaml:"guard,omitempty"`
	Healer     *HealerConfig     `yaml:"healer,omitempty"`
	QuestGiver *QuestGiverConfig `yaml:"quest_giver,omitempty"`
	Hireling   *HirelingConfig   `yaml:"hireling,omitempty"`
	Banker     *BankerConfig     `yaml:"banker,omitempty"`
	JobTrainer *JobTrainerConfig `yaml:"job_trainer,omitempty"`
	Crafter    *CrafterConfig    `yaml:"crafter,omitempty"`
	Fixer      *FixerConfig      `yaml:"fixer,omitempty"`
}

// Validate checks that the template satisfies basic invariants.
//
// Precondition: t must not be nil.
// Postcondition: Returns nil iff ID is non-empty, Name is non-empty, Level >= 1,
// MaxHP >= 1, and AC >= 10; returns an error on the first violation otherwise.
func (t *Template) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("npc template (no id): id must not be empty")
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
	validTiers := map[string]bool{
		"": true, "minion": true, "standard": true,
		"elite": true, "champion": true, "boss": true,
	}
	if !validTiers[t.Tier] {
		return fmt.Errorf("npc template %q: unknown tier %q", t.ID, t.Tier)
	}
	for _, ability := range t.BossAbilities {
		if err := ability.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	}
	if t.Loot != nil {
		if err := t.Loot.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
		// Animals must not have currency, items, equipment, or salvage_drop.
		if t.IsAnimal() {
			if t.Loot.Currency != nil {
				return fmt.Errorf("npc template %q: animal type must not have currency loot", t.ID)
			}
			if len(t.Loot.Items) > 0 {
				return fmt.Errorf("npc template %q: animal type must not have items loot", t.ID)
			}
			if t.Loot.SalvageDrop != nil {
				return fmt.Errorf("npc template %q: animal type must not have salvage_drop", t.ID)
			}
		}
	}

	// REQ-NPC-1: default NPCType to "combat".
	if t.NPCType == "" {
		t.NPCType = "combat"
	}

	// Default CourageThreshold to 999 (always engage). REQ-NB-10.
	if t.CourageThreshold == 0 {
		t.CourageThreshold = 999
	}

	// Validate NPCType value and corresponding config struct (REQ-NPC-2).
	validTypes := map[string]bool{
		"combat": true, "merchant": true, "guard": true, "healer": true,
		"quest_giver": true, "hireling": true, "banker": true,
		"job_trainer": true, "crafter": true, "fixer": true,
	}
	if !validTypes[t.NPCType] {
		return fmt.Errorf("npc template %q: unknown npc_type %q", t.ID, t.NPCType)
	}

	switch t.NPCType {
	case "combat":
		// no config struct required
	case "merchant":
		if t.Merchant == nil {
			return fmt.Errorf("npc template %q: npc_type 'merchant' requires a merchant: config block", t.ID)
		}
		if err := t.Merchant.ReplenishRate.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	case "guard":
		if t.Guard == nil {
			return fmt.Errorf("npc template %q: npc_type 'guard' requires a guard: config block", t.ID)
		}
		if err := t.Guard.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	case "healer":
		if t.Healer == nil {
			return fmt.Errorf("npc template %q: npc_type 'healer' requires a healer: config block", t.ID)
		}
	case "quest_giver":
		if t.QuestGiver == nil {
			return fmt.Errorf("npc template %q: npc_type 'quest_giver' requires a quest_giver: config block", t.ID)
		}
		if err := t.QuestGiver.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	case "hireling":
		if t.Hireling == nil {
			return fmt.Errorf("npc template %q: npc_type 'hireling' requires a hireling: config block", t.ID)
		}
	case "banker":
		if t.Banker == nil {
			return fmt.Errorf("npc template %q: npc_type 'banker' requires a banker: config block", t.ID)
		}
	case "job_trainer":
		if t.JobTrainer == nil {
			return fmt.Errorf("npc template %q: npc_type 'job_trainer' requires a job_trainer: config block", t.ID)
		}
	case "crafter":
		if t.Crafter == nil {
			return fmt.Errorf("npc template %q: npc_type 'crafter' requires an explicit crafter: {} config block", t.ID)
		}
	case "fixer":
		if t.Fixer == nil {
			return fmt.Errorf("npc template %q: npc_type 'fixer' requires a fixer: config block", t.ID)
		}
		if err := t.Fixer.Validate(); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
		// REQ-WC-3: fixers must not enter initiative order; enforce cowardly (flee) personality.
		if t.Personality != "" && t.Personality != "cowardly" {
			return fmt.Errorf("npc template %q: fixer npc_type requires personality 'cowardly' or empty (got %q)", t.ID, t.Personality)
		}
		t.Personality = "cowardly" // normalise empty → cowardly
	}

	return nil
}

// IsAnimal returns true when the template's Type is "animal".
func (t *Template) IsAnimal() bool { return t.Type == "animal" }

// IsRobot returns true when the template's Type is "robot".
func (t *Template) IsRobot() bool { return t.Type == "robot" }

// IsMachine returns true when the template's Type is "machine".
func (t *Template) IsMachine() bool { return t.Type == "machine" }

// ValidateWithSkills runs Validate and then checks all skill IDs referenced
// in any JobTrainerConfig against the provided skill registry.
//
// REQ-NPC-2a: unknown skill IDs MUST be a fatal load error.
//
// Precondition: t must not be nil; knownSkills may be nil (treated as empty).
// Postcondition: Returns nil iff Validate passes and all skill IDs are known.
func (t *Template) ValidateWithSkills(knownSkills map[string]bool) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if t.JobTrainer != nil {
		if err := t.JobTrainer.Validate(knownSkills); err != nil {
			return fmt.Errorf("npc template %q: %w", t.ID, err)
		}
	}
	return nil
}

// ValidateWithRegistry verifies that all feat IDs in Feats exist in registry
// and have AllowNPC == true.
//
// Precondition: t must not be nil; registry must not be nil.
// Postcondition: Returns nil iff all feats are valid for NPC use; error otherwise.
func (t *Template) ValidateWithRegistry(registry *ruleset.FeatRegistry) error {
	for _, featID := range t.Feats {
		f, ok := registry.Feat(featID)
		if !ok {
			return fmt.Errorf("npc template %q: feat %q not found in registry", t.ID, featID)
		}
		if !f.AllowNPC {
			return fmt.Errorf("npc template %q: feat %q does not have allow_npc: true", t.ID, featID)
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
// loadTemplatesFromBytes parses YAML that may be a single template (map) or a
// list of templates (sequence). Both formats are supported.
func loadTemplatesFromBytes(path string, data []byte) ([]*Template, error) {
	// Detect list vs single by attempting list unmarshal first.
	var list []Template
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		out := make([]*Template, 0, len(list))
		for i := range list {
			if err := list[i].Validate(); err != nil {
				return nil, fmt.Errorf("validating template %d in %q: %w", i, path, err)
			}
			out = append(out, &list[i])
		}
		return out, nil
	}
	// Fall back to single-template format.
	tmpl, err := LoadTemplateFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("loading %q: %w", path, err)
	}
	return []*Template{tmpl}, nil
}

func LoadTemplates(dir string) ([]*Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading npc dir %q: %w", dir, err)
	}

	var templates []*Template
	for _, entry := range entries {
		if entry.IsDir() {
			sub, err := LoadTemplates(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			templates = append(templates, sub...)
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}

		loaded, err := loadTemplatesFromBytes(path, data)
		if err != nil {
			return nil, err
		}
		templates = append(templates, loaded...)
	}
	return templates, nil
}

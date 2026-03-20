package technology

import "fmt"

type Tradition string
type UsageType string
type Range string
type Targets string
type EffectType string

const (
	TraditionTechnical       Tradition = "technical"
	TraditionFanaticDoctrine Tradition = "fanatic_doctrine"
	TraditionNeural          Tradition = "neural"
	TraditionBioSynthetic    Tradition = "bio_synthetic"
)

var validTraditions = map[Tradition]bool{
	TraditionTechnical: true, TraditionFanaticDoctrine: true,
	TraditionNeural: true, TraditionBioSynthetic: true,
}

const (
	UsageHardwired   UsageType = "hardwired"
	UsagePrepared    UsageType = "prepared"
	UsageSpontaneous UsageType = "spontaneous"
	UsageInnate      UsageType = "innate"
)

var validUsageTypes = map[UsageType]bool{
	UsageHardwired: true, UsagePrepared: true,
	UsageSpontaneous: true, UsageInnate: true,
}

const (
	RangeSelf   Range = "self"
	RangeMelee  Range = "melee"
	RangeRanged Range = "ranged"
	RangeZone   Range = "zone"
)

var validRanges = map[Range]bool{
	RangeSelf: true, RangeMelee: true, RangeRanged: true, RangeZone: true,
}

const (
	TargetsSingle     Targets = "single"
	TargetsAllEnemies Targets = "all_enemies"
	TargetsAllAllies  Targets = "all_allies"
	TargetsZone       Targets = "zone"
)

var validTargets = map[Targets]bool{
	TargetsSingle: true, TargetsAllEnemies: true,
	TargetsAllAllies: true, TargetsZone: true,
}

const (
	EffectDamage     EffectType = "damage"
	EffectHeal       EffectType = "heal"
	EffectCondition  EffectType = "condition"
	EffectSkillCheck EffectType = "skill_check"
	EffectMovement   EffectType = "movement"
	EffectZone       EffectType = "zone"
	EffectSummon     EffectType = "summon"
	EffectUtility    EffectType = "utility"
	EffectDrain      EffectType = "drain"
	EffectTremorsense EffectType = "tremorsense"
)

var validEffectTypes = map[EffectType]bool{
	EffectDamage: true, EffectHeal: true, EffectCondition: true,
	EffectSkillCheck: true, EffectMovement: true, EffectZone: true,
	EffectSummon: true, EffectUtility: true, EffectDrain: true,
	EffectTremorsense: true,
}

// TechEffect is one effect within a technology, discriminated by Type.
// Only fields relevant to the given Type need be set; others are zero-valued.
// The mappings below describe which fields are meaningful for each type —
// Validate() enforces required fields only for skill_check; other type-specific
// field constraints are advisory and enforced at resolution time.
//
//	damage      — Dice or Amount; DamageType
//	heal        — Dice or Amount
//	drain       — Dice or Amount; Resource ("hp" | "ap")
//	condition   — ConditionID; Duration (optional, overrides parent if set)
//	skill_check — Skill (non-empty); DC (> 0) — enforced by Validate()
//	movement    — Distance; Direction ("toward" | "away" | "teleport")
//	zone        — Radius
//	summon      — NPCID; SummonRounds
//	utility     — UtilityType ("unlock" | "reveal" | "hack")
type TechEffect struct {
	Type EffectType `yaml:"type"`

	// damage / heal / drain
	Dice       string `yaml:"dice,omitempty"`
	DamageType string `yaml:"damage_type,omitempty"`
	Amount     int    `yaml:"amount,omitempty"`
	Resource   string `yaml:"resource,omitempty"` // drain: "hp" | "ap"

	// condition
	ConditionID string `yaml:"condition_id,omitempty"`
	Value       int    `yaml:"value,omitempty"`    // condition severity (e.g. frightened 1, 2)
	Duration    string `yaml:"duration,omitempty"` // overrides parent duration if set

	// skill_check — DC must be > 0; yaml tag does NOT use omitempty so zero is preserved
	Skill string `yaml:"skill,omitempty"`
	DC    int    `yaml:"dc"`

	// movement
	Distance  int    `yaml:"distance,omitempty"`  // feet; > 0
	Direction string `yaml:"direction,omitempty"` // toward | away | teleport

	// zone
	Radius int `yaml:"radius,omitempty"` // feet; > 0

	// summon
	NPCID        string `yaml:"npc_id,omitempty"`
	SummonRounds int    `yaml:"summon_rounds,omitempty"` // > 0

	// utility
	UtilityType string `yaml:"utility_type,omitempty"` // unlock | reveal | hack
	Description string `yaml:"description,omitempty"` // human-readable text for utility effects
}

// TieredEffects holds per-outcome effect lists for a technology.
// Only the tiers relevant to the tech's Resolution type need to be populated.
//
//	Save-based (resolution:"save"):   OnCritSuccess/OnSuccess/OnFailure/OnCritFailure
//	Attack-based (resolution:"attack"): OnMiss/OnHit/OnCritHit
//	No-roll (resolution:"none" or ""):  OnApply
type TieredEffects struct {
	// Save-based tiers
	OnCritSuccess []TechEffect `yaml:"on_crit_success,omitempty"`
	OnSuccess     []TechEffect `yaml:"on_success,omitempty"`
	OnFailure     []TechEffect `yaml:"on_failure,omitempty"`
	OnCritFailure []TechEffect `yaml:"on_crit_failure,omitempty"`
	// Attack-based tiers
	OnMiss    []TechEffect `yaml:"on_miss,omitempty"`
	OnHit     []TechEffect `yaml:"on_hit,omitempty"`
	OnCritHit []TechEffect `yaml:"on_crit_hit,omitempty"`
	// No-roll
	OnApply []TechEffect `yaml:"on_apply,omitempty"`
}

// AllEffects returns a flat slice of all TechEffect entries across all tiers.
// Used for validation to check all contained effects are structurally valid.
func (te TieredEffects) AllEffects() []TechEffect {
	var all []TechEffect
	all = append(all, te.OnCritSuccess...)
	all = append(all, te.OnSuccess...)
	all = append(all, te.OnFailure...)
	all = append(all, te.OnCritFailure...)
	all = append(all, te.OnMiss...)
	all = append(all, te.OnHit...)
	all = append(all, te.OnCritHit...)
	all = append(all, te.OnApply...)
	return all
}

// TechnologyDef defines a single technology — the game's analog of a PF2E spell.
//
// Precondition: ID, Name, Tradition, Level (1–10), UsageType, Range, Targets,
// Duration, and at least one Effect must all be set.
// Postcondition: Validate() returns nil iff all required fields are present and valid.
type TechnologyDef struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description,omitempty"`
	Tradition   Tradition `yaml:"tradition"`
	Level       int       `yaml:"level"`
	UsageType   UsageType `yaml:"usage_type"`
	ActionCost  int       `yaml:"action_cost"`
	Range       Range     `yaml:"range"`
	Targets     Targets   `yaml:"targets"`
	Duration    string    `yaml:"duration"`
	SaveType    string    `yaml:"save_type,omitempty"`
	SaveDC      int       `yaml:"save_dc,omitempty"`
	Resolution   string        `yaml:"resolution,omitempty"`   // "save" | "attack" | "none"
	Effects      TieredEffects `yaml:"effects,omitempty"`
	AmpedLevel   int           `yaml:"amped_level,omitempty"`
	AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
	// Passive indicates this technology fires automatically on room state changes
	// and requires no player action. When true, ActionCost must be 0.
	Passive bool `yaml:"passive,omitempty"`
}

// Validate returns an error if any required field is missing or invalid.
// Precondition: t is not nil.
// Postcondition: returns nil iff all required fields are present and valid.
func (t *TechnologyDef) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if t.Name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if !validTraditions[t.Tradition] {
		return fmt.Errorf("unknown tradition %q", t.Tradition)
	}
	if t.Level < 1 || t.Level > 10 {
		return fmt.Errorf("level %d out of range [1,10]", t.Level)
	}
	if !validUsageTypes[t.UsageType] {
		return fmt.Errorf("unknown usage_type %q", t.UsageType)
	}
	if t.Passive && t.ActionCost != 0 {
		return fmt.Errorf("passive technology %q must have action_cost 0, got %d", t.ID, t.ActionCost)
	}
	if !validRanges[t.Range] {
		return fmt.Errorf("unknown range %q", t.Range)
	}
	if !validTargets[t.Targets] {
		return fmt.Errorf("unknown targets %q", t.Targets)
	}
	if t.Duration == "" {
		return fmt.Errorf("duration must not be empty")
	}
	// Validate Resolution/SaveType/SaveDC consistency.
	switch t.Resolution {
	case "", "none":
		if t.SaveType != "" {
			return fmt.Errorf("save_type must be empty when resolution is %q", t.Resolution)
		}
		if t.SaveDC != 0 {
			return fmt.Errorf("save_dc must be 0 when resolution is %q", t.Resolution)
		}
	case "save":
		if t.SaveType == "" {
			return fmt.Errorf("save_type must be set when resolution is \"save\"")
		}
		if t.SaveDC == 0 {
			return fmt.Errorf("save_dc must be > 0 when resolution is \"save\"")
		}
	case "attack":
		if t.SaveType != "" {
			return fmt.Errorf("save_type must be empty when resolution is \"attack\"")
		}
	default:
		return fmt.Errorf("unknown resolution %q", t.Resolution)
	}
	// Validate all effects in all tiers.
	if len(t.Effects.AllEffects()) == 0 {
		return fmt.Errorf("effects must have at least one entry")
	}
	for i, e := range t.Effects.AllEffects() {
		if err := validateEffect(e, i); err != nil {
			return err
		}
	}
	if len(t.AmpedEffects.AllEffects()) > 0 && t.AmpedLevel == 0 {
		return fmt.Errorf("amped_level must be > 0 when amped_effects is non-empty")
	}
	if t.AmpedLevel > 0 && len(t.AmpedEffects.AllEffects()) == 0 {
		return fmt.Errorf("amped_effects must have at least one effect when amped_level > 0")
	}
	for i, e := range t.AmpedEffects.AllEffects() {
		if err := validateEffect(e, i); err != nil {
			return fmt.Errorf("amped_effects[%d]: %w", i, err)
		}
	}
	return nil
}

func validateEffect(e TechEffect, idx int) error {
	if !validEffectTypes[e.Type] {
		return fmt.Errorf("effects[%d]: unknown type %q", idx, e.Type)
	}
	// Only skill_check requires runtime-enforced field validation per the spec.
	// Other per-type field constraints are enforced at resolution time.
	if e.Type == EffectSkillCheck {
		if e.Skill == "" {
			return fmt.Errorf("effects[%d]: skill_check effect requires skill", idx)
		}
		if e.DC == 0 {
			return fmt.Errorf("effects[%d]: skill_check effect requires dc > 0", idx)
		}
	}
	return nil
}

// TechAtSlotLevel returns the tech definition to use when activating at the given slot level.
// When slotLevel >= tech.AmpedLevel and AmpedLevel > 0, returns a shallow copy of tech
// with Effects replaced by AmpedEffects.
// Otherwise returns tech unchanged.
//
// Precondition: tech is non-nil; slotLevel >= 0.
// Postcondition: the original tech is never mutated.
func TechAtSlotLevel(tech *TechnologyDef, slotLevel int) *TechnologyDef {
	if tech.AmpedLevel > 0 && slotLevel >= tech.AmpedLevel {
		copy := *tech
		copy.Effects = tech.AmpedEffects
		return &copy
	}
	return tech
}

package technology

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/cory-johannsen/mud/internal/game/reaction"
)
var shortNameRE = regexp.MustCompile(`^[a-z0-9_]+$`)

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
	Resource   string `yaml:"resource,omitempty"`    // drain: "hp" | "ap"
	Persistent bool    `yaml:"persistent,omitempty"`  // damage persists each round (PF2E persistent damage)
	Multiplier float64 `yaml:"multiplier,omitempty"` // damage multiplier (e.g. 0.5 for basic save half-damage)
	// Projectiles, when > 0, resolves the damage effect as that many independent
	// rolls (Magic Missile-style). Heighten delta is added to this count at
	// activation time. Zero or unset resolves as a single roll (existing behavior).
	Projectiles int `yaml:"projectiles,omitempty"`

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

// ValidateMultiplier checks that the Multiplier field is a legal value.
// Legal values: 0 (unset/no-op), 0.5 (halver), 1.0 (no-op), or > 1.0 (multiplier).
// Values in (0, 1) other than 0.5 are a load-time error (MULT-10).
// Postcondition: returns non-nil error iff Multiplier is illegal.
func (e TechEffect) ValidateMultiplier() error {
	m := e.Multiplier
	if m == 0 || m == 1.0 || m > 1.0 {
		return nil
	}
	if math.Abs(m-0.5) < 1e-9 {
		return nil
	}
	if m < 0 {
		return fmt.Errorf("tech effect: negative multiplier %v is not permitted", m)
	}
	return fmt.Errorf("tech effect: illegal fractional multiplier %v (only 0.5 permitted)", m)
}

// IsHalver returns true iff Multiplier == 0.5 (converts to a halver stage). (MULT-9)
func (e TechEffect) IsHalver() bool {
	return math.Abs(e.Multiplier-0.5) < 1e-9
}

// IsMultiplier returns true iff Multiplier > 1.0 (feeds the multiplier bucket). (MULT-8)
func (e TechEffect) IsMultiplier() bool {
	return e.Multiplier > 1.0
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
	ID        string `yaml:"id"`
	ShortName string `yaml:"short_name,omitempty"`
	Name      string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	// RechargeCondition is a human-readable string describing when limited uses restore.
	// Examples: "Recharges on rest", "Daily". Empty for unlimited techs.
	RechargeCondition string `yaml:"recharge_condition,omitempty"`
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
	// FocusCost indicates this technology costs 1 Focus Point to use.
	// Cannot be true when Passive is true.
	FocusCost bool `yaml:"focus_cost,omitempty"`
	// Reaction declares this tech as a player reaction with the given trigger and effect.
	// Only applicable to innate techs. Nil means this tech is not a reaction.
	Reaction *reaction.ReactionDef `yaml:"reaction,omitempty"`
	// AoeRadius is the radius in feet of an area-of-effect burst centered on the target grid square.
	// 0 means single-target (default). When > 0 and UseRequest.target_x/target_y are >= 0 (not the -1 sentinel),
	// effects are applied to every combatant within Chebyshev distance AoeRadius of the target square.
	AoeRadius int `yaml:"aoe_radius,omitempty"`
	// PassiveBonuses are always-on typed bonuses granted while this technology is active (Passive == true only).
	PassiveBonuses []effect.Bonus `yaml:"passive_bonuses,omitempty"`
}

// Validate returns an error if any required field is missing or invalid.
// Precondition: t is not nil.
// Postcondition: returns nil iff all required fields are present and valid.
func (t *TechnologyDef) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if t.ShortName != "" {
		if len(t.ShortName) < 2 || len(t.ShortName) > 32 {
			return fmt.Errorf("short_name %q must be between 2 and 32 characters", t.ShortName)
		}
		if !shortNameRE.MatchString(t.ShortName) {
			return fmt.Errorf("short_name %q must contain only lowercase letters, digits, and underscores", t.ShortName)
		}
		if strings.HasPrefix(t.ShortName, "_") || strings.HasSuffix(t.ShortName, "_") {
			return fmt.Errorf("short_name %q must not begin or end with an underscore", t.ShortName)
		}
		if t.ShortName == t.ID {
			return fmt.Errorf("short_name %q must not be identical to the technology id", t.ShortName)
		}
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
	if t.Passive && t.FocusCost {
		return fmt.Errorf("technology %q: focus_cost and passive cannot both be true", t.ID)
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
	// REQ-CRX6: reaction-only techs (Reaction != nil) do not require effects.
	if len(t.Effects.AllEffects()) == 0 && t.Reaction == nil {
		return fmt.Errorf("effects must have at least one entry")
	}
	for i, e := range t.Effects.AllEffects() {
		if err := validateEffect(e, i); err != nil {
			return err
		}
	}
	if t.AoeRadius < 0 {
		return fmt.Errorf("technology %q: aoe_radius must be >= 0, got %d", t.ID, t.AoeRadius)
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
	if err := e.ValidateMultiplier(); err != nil {
		return fmt.Errorf("effects[%d]: %w", idx, err)
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

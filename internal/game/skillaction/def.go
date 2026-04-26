// Package skillaction provides a unified, data-driven framework for resolving
// non-combat (skill-based) actions taken against combat NPCs.
//
// An ActionDef is the YAML-loaded static description of a single skill action
// (Demoralize, Feint, Trip, Recall Knowledge, ...). The ActionDef enumerates
// the AP cost, the skill exercised, the DC source, the range / target kind
// requirements, and a per-degree-of-success outcome map. The Resolver walks
// that outcome map after a DoS computation to drive a small effect-application
// callback (ApplyCondition / Damage / Move / Narrative / Reveal).
//
// The package is the functional core; all mutation lives behind the
// ResolveContext.Apply callback so that Resolve itself can stay deterministic
// and pure (NCA-11). See docs/architecture/combat.md for the full pipeline.
package skillaction

import (
	"fmt"
)

// DegreeOfSuccess is the four-tier PF2E result band.
//
// The integer values intentionally match the ordering used by skillcheck.CheckOutcome
// (CritSuccess=0, Success=1, Failure=2, CritFailure=3).
type DegreeOfSuccess int

const (
	CritSuccess DegreeOfSuccess = iota
	Success
	Failure
	CritFailure
)

// String returns the YAML-friendly snake_case name for the degree.
func (d DegreeOfSuccess) String() string {
	switch d {
	case CritSuccess:
		return "crit_success"
	case Success:
		return "success"
	case Failure:
		return "failure"
	case CritFailure:
		return "crit_failure"
	default:
		return "unknown"
	}
}

// DCKind enumerates the supported DC source types.
type DCKind string

const (
	DCKindFixed            DCKind = "fixed"             // literal value from def.DC.Value
	DCKindTargetWill       DCKind = "target_will"       // 10 + level + Savvy mod + Cool rank bonus
	DCKindTargetPerception DCKind = "target_perception" // 10 + level + Savvy mod + Awareness rank bonus
	DCKindTargetAC         DCKind = "target_ac"         // raw target AC
	DCKindFormula          DCKind = "formula"           // free-form expression (deferred)
)

// RangeKind enumerates the supported targeting ranges.
type RangeKind string

const (
	RangeMelee  RangeKind = "melee_reach" // adjacent (≤5 ft)
	RangeRanged RangeKind = "ranged"      // up to def.Range.Feet
	RangeSelf   RangeKind = "self"        // actor only
)

// TargetKind enumerates the kinds of combatant the action may target.
type TargetKind string

const (
	TargetKindNPC    TargetKind = "npc"
	TargetKindPlayer TargetKind = "player"
	TargetKindAny    TargetKind = "any"
	TargetKindSelf   TargetKind = "self"
)

// DC describes how the difficulty class is computed at resolve time.
type DC struct {
	Kind  DCKind `yaml:"kind"`
	Value int    `yaml:"value,omitempty"`
	Expr  string `yaml:"expr,omitempty"`
}

// Range describes the legal actor↔target distance.
type Range struct {
	Kind RangeKind `yaml:"kind"`
	Feet int       `yaml:"feet,omitempty"`
}

// Effect is the discriminated union of side-effects produced by a successful
// (or failed) outcome. The wire format uses one effect kind per YAML map entry
// — see UnmarshalYAML on EffectEntry.
type Effect interface {
	effect()
}

// ApplyCondition adds a condition (by canonical ID) to the target, with a
// stack count and a duration in rounds. Duration -1 means "until removed".
type ApplyCondition struct {
	ID             string `yaml:"id"`
	Stacks         int    `yaml:"stacks,omitempty"`
	DurationRounds int    `yaml:"duration_rounds,omitempty"`
}

func (ApplyCondition) effect() {}

// Narrative emits a player-facing line. May contain {actor}/{target} placeholders.
type Narrative struct {
	Text string `yaml:"text"`
}

func (Narrative) effect() {}

// Damage rolls and applies an HP damage formula to the target.
type Damage struct {
	Expr string `yaml:"expr"`
}

func (Damage) effect() {}

// Move shifts the target by a number of feet (positive = away from actor).
type Move struct {
	Feet int `yaml:"feet"`
}

func (Move) effect() {}

// Reveal exposes a number of NPC-metadata facts (Recall Knowledge).
type Reveal struct {
	Count int `yaml:"count"`
}

func (Reveal) effect() {}

// OutcomeDef enumerates the side-effects of a single degree band, plus
// optional flags such as ap_refund (NCA-33).
type OutcomeDef struct {
	APRefund  bool      `yaml:"ap_refund,omitempty"`
	Effects   []Effect  `yaml:"-"`
	Narrative string    `yaml:"-"` // resolved narrative shortcut
	rawEffects []EffectEntry
}

// ActionDef is the full static description of one skill action.
type ActionDef struct {
	ID          string                              `yaml:"id"`
	DisplayName string                              `yaml:"display_name"`
	Description string                              `yaml:"description"`
	APCost      int                                 `yaml:"ap_cost"`
	Skill       string                              `yaml:"skill"`
	DC          DC                                  `yaml:"dc"`
	Range       Range                               `yaml:"range"`
	TargetKinds []TargetKind                        `yaml:"target_kinds"`
	Outcomes    map[DegreeOfSuccess]*OutcomeDef     `yaml:"-"`
	rawOutcomes map[string]rawOutcome
}

type rawOutcome struct {
	APRefund bool          `yaml:"ap_refund,omitempty"`
	Effects  []EffectEntry `yaml:"effects,omitempty"`
}

// EffectEntry is the wire form of one Effect. Each YAML mapping has exactly
// one of the known keys (apply_condition / narrative / damage / move / reveal).
type EffectEntry struct {
	ApplyCondition *ApplyCondition `yaml:"apply_condition,omitempty"`
	Narrative      *Narrative      `yaml:"narrative,omitempty"`
	Damage         *Damage         `yaml:"damage,omitempty"`
	Move           *Move           `yaml:"move,omitempty"`
	Reveal         *Reveal         `yaml:"reveal,omitempty"`
}

// ToEffect collapses an EffectEntry to its single non-nil Effect, or returns
// an error if zero or multiple effect kinds are populated.
func (e EffectEntry) ToEffect() (Effect, error) {
	count := 0
	var out Effect
	if e.ApplyCondition != nil {
		count++
		out = *e.ApplyCondition
	}
	if e.Narrative != nil {
		count++
		out = *e.Narrative
	}
	if e.Damage != nil {
		count++
		out = *e.Damage
	}
	if e.Move != nil {
		count++
		out = *e.Move
	}
	if e.Reveal != nil {
		count++
		out = *e.Reveal
	}
	if count == 0 {
		return nil, fmt.Errorf("effect entry has no recognised kind")
	}
	if count > 1 {
		return nil, fmt.Errorf("effect entry has %d kinds; exactly one is required", count)
	}
	return out, nil
}

// parseDegree maps the YAML key onto a DegreeOfSuccess.
func parseDegree(s string) (DegreeOfSuccess, error) {
	switch s {
	case "crit_success":
		return CritSuccess, nil
	case "success":
		return Success, nil
	case "failure":
		return Failure, nil
	case "crit_failure":
		return CritFailure, nil
	default:
		return 0, fmt.Errorf("unknown degree of success %q", s)
	}
}

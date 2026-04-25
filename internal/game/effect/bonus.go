// Package effect implements typed-bonus stacking and resolution for combat
// stat contributions. Per DEDUP requirements, bonuses of the same type stack
// by taking the maximum; bonuses of different types stack additively; penalties
// stack separately and always apply.
package effect

import (
	"fmt"
	"strings"
)

// BonusType classifies how bonuses of the same type stack.
type BonusType string

const (
	BonusTypeStatus       BonusType = "status"
	BonusTypeCircumstance BonusType = "circumstance"
	BonusTypeItem         BonusType = "item"
	BonusTypeUntyped      BonusType = "untyped"
)

// Stat identifies what a bonus applies to.
type Stat string

const (
	StatAttack    Stat = "attack"
	StatAC        Stat = "ac"
	StatDamage    Stat = "damage"
	StatSpeed     Stat = "speed"
	StatBrutality Stat = "brutality"
	StatGrit      Stat = "grit"
	StatQuickness Stat = "quickness"
	StatReasoning Stat = "reasoning"
	StatSavvy     Stat = "savvy"
	StatFlair     Stat = "flair"
	StatSkill     Stat = "skill"
)

// DurationKind indicates how an effect's duration is tracked.
type DurationKind string

const (
	DurationRounds      DurationKind = "rounds"
	DurationUntilRemove DurationKind = "until_remove"
	DurationPermanent   DurationKind = "permanent"
	DurationEncounter   DurationKind = "encounter"
	DurationCalendar    DurationKind = "calendar"
)

// Bonus is a single typed numeric contribution to one stat.
// Precondition: Value must not be zero (enforced by Validate).
type Bonus struct {
	Stat  Stat      `yaml:"stat"`
	Value int       `yaml:"value"` // positive = bonus, negative = penalty; 0 is invalid
	Type  BonusType `yaml:"type"`  // defaults to BonusTypeUntyped via Normalise
}

// Validate returns an error if the Bonus is malformed.
// Postcondition: returns non-nil error iff Value == 0.
func (b Bonus) Validate() error {
	if b.Value == 0 {
		return fmt.Errorf("effect.Bonus: value 0 is not permitted (stat %q, type %q)", b.Stat, b.Type)
	}
	return nil
}

// Normalise sets Type to BonusTypeUntyped if it is empty.
// Call this after YAML unmarshal to apply the default-type rule (DEDUP-2).
func (b *Bonus) Normalise() {
	if b.Type == "" {
		b.Type = BonusTypeUntyped
	}
}

// StatMatches reports whether a bonus on bonusStat contributes to a query for queryStat.
// Per DEDUP-16: a bonus to "skill" contributes to any "skill:<id>" query;
// a bonus to "skill:stealth" does NOT contribute to "skill:savvy".
func StatMatches(bonusStat, queryStat Stat) bool {
	if bonusStat == queryStat {
		return true
	}
	prefix := string(bonusStat) + ":"
	return strings.HasPrefix(string(queryStat), prefix)
}

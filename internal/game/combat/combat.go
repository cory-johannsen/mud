// Package combat implements the PvE combat engine for Gunchete.
package combat

// Kind distinguishes player combatants from NPC combatants.
type Kind int

const (
	KindPlayer Kind = iota
	KindNPC
)

// Outcome is the PF2E 4-tier attack result.
type Outcome int

const (
	CritSuccess Outcome = iota
	Success
	Failure
	CritFailure
)

// String returns a human-readable outcome label.
func (o Outcome) String() string {
	switch o {
	case CritSuccess:
		return "critical success"
	case Success:
		return "success"
	case Failure:
		return "failure"
	case CritFailure:
		return "critical failure"
	default:
		return "unknown"
	}
}

// Combatant represents one participant in a combat — either a player or an NPC instance.
type Combatant struct {
	ID         string
	Kind       Kind
	Name       string
	MaxHP      int
	CurrentHP  int
	AC         int
	Level      int
	StrMod     int
	DexMod     int
	Initiative int
}

func (c *Combatant) IsPlayer() bool { return c.Kind == KindPlayer }
func (c *Combatant) IsDead() bool   { return c.CurrentHP <= 0 }

// ApplyDamage reduces CurrentHP by amount, flooring at zero.
func (c *Combatant) ApplyDamage(amount int) {
	c.CurrentHP -= amount
	if c.CurrentHP < 0 {
		c.CurrentHP = 0
	}
}

// OutcomeFor determines the PF2E 4-tier attack outcome for a given roll vs AC.
func OutcomeFor(roll, ac int) Outcome {
	switch {
	case roll >= ac+10:
		return CritSuccess
	case roll >= ac:
		return Success
	case roll >= ac-10:
		return Failure
	default:
		return CritFailure
	}
}

// ProficiencyBonus returns the PF2E simplified proficiency bonus for the given level.
// Formula: 2 + (level-1)/4
func ProficiencyBonus(level int) int {
	return 2 + (level-1)/4
}

// AbilityMod computes the standard ability modifier: (score - 10) / 2.
func AbilityMod(score int) int {
	return (score - 10) / 2
}

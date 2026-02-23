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
	// Dead is true when this combatant has been permanently killed.
	// For NPCs, reaching 0 HP sets Dead=true immediately.
	// For players, Dead=true only when the dying condition advances to stack 4.
	Dead bool
}

// IsPlayer reports whether this combatant is a player character.
// Postcondition: Returns true iff Kind == KindPlayer.
func (c *Combatant) IsPlayer() bool { return c.Kind == KindPlayer }

// IsDead reports whether this combatant is permanently dead.
// For NPCs: true when CurrentHP <= 0.
// For players: true when Dead flag is set (dying chain resolved to death).
//
// Postcondition: Returns true iff the combatant is permanently dead.
func (c *Combatant) IsDead() bool {
	if c.Kind == KindPlayer {
		return c.Dead
	}
	return c.CurrentHP <= 0
}

// ApplyDamage reduces CurrentHP by amount, flooring at zero.
// Precondition: amount must be >= 0.
// Postcondition: CurrentHP >= 0.
func (c *Combatant) ApplyDamage(amount int) {
	c.CurrentHP -= amount
	if c.CurrentHP < 0 {
		c.CurrentHP = 0
	}
}

// OutcomeFor determines the PF2E 4-tier attack outcome for a given roll vs AC.
// Precondition: roll >= 1; ac >= 10.
// Postcondition: Returns one of CritSuccess, Success, Failure, CritFailure.
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
// Formula: 2 + (level-1)/4, minimum 2.
// Precondition: level >= 1.
// Postcondition: Returns >= 2.
func ProficiencyBonus(level int) int {
	return 2 + (level-1)/4
}

// AbilityMod computes the standard ability modifier using floor division: floor((score - 10) / 2).
// Postcondition: Returns floor((score - 10) / 2).
func AbilityMod(score int) int {
	diff := score - 10
	if diff < 0 {
		return (diff - 1) / 2
	}
	return diff / 2
}

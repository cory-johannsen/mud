// Package combat implements the PvE combat engine for Gunchete.
package combat

import "github.com/cory-johannsen/mud/internal/game/inventory"

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
	// Loadout is the active weapon preset for this combatant; may be nil.
	Loadout *inventory.WeaponPreset
	// NPCType is the category of this combatant used for predators_eye passive matching.
	// Empty string for player combatants.
	NPCType string
	// WeaponProficiencyRank is the character's proficiency rank for their equipped weapon category.
	// Empty string or "untrained" means no proficiency bonus on attack rolls.
	WeaponProficiencyRank string
	// ArmorProficiencyRank is the character's proficiency rank for their equipped armor category.
	// Empty string or "untrained" means no proficiency bonus to AC.
	ArmorProficiencyRank string
	// Save ability modifiers (derived from character ability scores at combat start).
	GritMod      int // used for Toughness saves
	QuicknessMod int // used for Hustle saves
	SavvyMod     int // used for Cool saves
	// Save proficiency ranks from character_proficiencies.
	ToughnessRank string // "untrained", "trained", "expert", "master", "legendary"
	HustleRank    string
	CoolRank      string
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

// CombatProficiencyBonus returns the PF2E proficiency bonus for an attack or AC calculation.
//
// Precondition: level >= 1; rank is one of "untrained", "trained", "expert", "master", "legendary", or "".
// Postcondition: Returns 0 for untrained/empty; level+2 for trained; level+4 for expert; level+6 for master; level+8 for legendary.
func CombatProficiencyBonus(level int, rank string) int {
	switch rank {
	case "trained":
		return level + 2
	case "expert":
		return level + 4
	case "master":
		return level + 6
	case "legendary":
		return level + 8
	default: // "untrained" or unknown/empty
		return 0
	}
}

// ProficiencyBonus returns the combat proficiency bonus assuming trained rank.
//
// Deprecated: use CombatProficiencyBonus with an explicit rank.
// Precondition: level >= 1.
// Postcondition: Returns level+2 (equivalent to CombatProficiencyBonus(level, "trained")).
func ProficiencyBonus(level int) int {
	return CombatProficiencyBonus(level, "trained")
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

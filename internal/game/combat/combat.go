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

// Cover tier constants define the valid values for Combatant.CoverTier.
const (
	CoverTierNone     = ""
	CoverTierLesser   = "lesser"
	CoverTierStandard = "standard"
	CoverTierGreater  = "greater"
)

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
	// InitiativeBonus is the persistent attack+AC bonus for a player who wins initiative.
	// Scaled 1–5→+1, 6–10→+2, 11+→+3. Always 0 for NPCs.
	InitiativeBonus int
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
	// WeaponDamageType is the damage type of the currently equipped main-hand weapon.
	// Empty string means unarmed (no special type).
	WeaponDamageType string
	// Resistances maps damage type → flat damage reduction (minimum 0). Always nil for players.
	Resistances map[string]int
	// Weaknesses maps damage type → flat damage addition. Always nil for players.
	Weaknesses map[string]int
	// ArmorProficiencyRank is the character's proficiency rank for their equipped armor category.
	// Empty string or "untrained" means no proficiency bonus to AC.
	ArmorProficiencyRank string
	// GritMod is the character's Grit ability modifier, used for Toughness saving rolls.
	// Computed via AbilityMod(sess.Abilities.Grit) at combat start; zero for NPCs.
	GritMod int
	// QuicknessMod is the character's Quickness ability modifier, used for Hustle saving rolls.
	// Computed via AbilityMod(sess.Abilities.Quickness) at combat start; zero for NPCs.
	QuicknessMod int
	// SavvyMod is the character's Savvy ability modifier, used for Cool saving rolls.
	// Computed via AbilityMod(sess.Abilities.Savvy) at combat start; zero for NPCs.
	SavvyMod int
	// ToughnessRank is the character's proficiency rank for Toughness saving rolls.
	// Valid values: "untrained", "trained", "expert", "master", "legendary". Defaults to "untrained".
	ToughnessRank string
	// HustleRank is the character's proficiency rank for Hustle saving rolls.
	// Valid values: "untrained", "trained", "expert", "master", "legendary". Defaults to "untrained".
	HustleRank string
	// CoolRank is the character's proficiency rank for Cool saving rolls.
	// Valid values: "untrained", "trained", "expert", "master", "legendary". Defaults to "untrained".
	CoolRank string
	// ACMod is a temporary mid-round AC modifier applied by conditions (e.g. flat_footed, shield_raised).
	// Negative values reduce effective AC; positive values increase it.
	ACMod int
	// AttackMod is a temporary mid-round attack roll modifier applied by conditions (e.g. frightened).
	// Negative values reduce the attacker's roll total.
	AttackMod int
	// WeaponName is the display name of the NPC's equipped weapon; empty = unarmed.
	WeaponName string
	// Hidden is true when this combatant is concealed. Attackers must pass a DC 11 flat check.
	// For player combatants: set by hide/divert actions; cleared when the player attacks or is targeted.
	// For NPC combatants: unused (always false).
	Hidden bool
	// RevealedUntilRound suppresses the DC 11 flat check for attackers through this round number.
	// Set by a successful Seek action to cbt.Round+1.
	RevealedUntilRound int
	// Position is the distance in feet along the combat axis from the player's starting point (0).
	// Player combatants are initialized to 0; NPC combatants are initialized to 25.
	Position int
	// CoverEquipmentID is the ItemID of the room equipment object this combatant is
	// using for cover. Empty string means the combatant is not in cover.
	CoverEquipmentID string
	// CoverTier is the tier of cover this combatant has: "lesser", "standard",
	// "greater", or "" (none).
	CoverTier string
	// AttackVerb is the verb used in combat attack narratives (e.g. "bites", "shoots").
	// Empty string means the default verb ("attacks") will be used.
	AttackVerb string
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

// DefaultSaveRank returns rank if non-empty, otherwise "untrained".
// Precondition: none.
// Postcondition: Returns a non-empty proficiency rank string.
func DefaultSaveRank(rank string) string {
	if rank == "" {
		return "untrained"
	}
	return rank
}

// combatantDist returns the distance in feet between two combatants.
//
// Precondition: a and b must be non-nil.
// Postcondition: Returns abs(a.Position - b.Position).
func combatantDist(a, b *Combatant) int {
	return posDist(a.Position, b.Position)
}

// PosDist returns the absolute distance between two raw position values.
//
// Precondition: none.
// Postcondition: Returns abs(a - b).
func PosDist(a, b int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d
}

// posDist is the unexported alias for PosDist, kept for internal use.
func posDist(a, b int) int { return PosDist(a, b) }

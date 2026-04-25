// Package combat implements the PvE combat engine for Gunchete.
package combat

import (
	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// MaxCombatRange is the maximum engagement distance in feet.
// Legacy value retained for ranged weapon checks. On the 10×10 grid the actual
// maximum Chebyshev distance is 9 squares × 5 ft = 45 ft; the 100 ft cap is never
// reached via grid movement but acts as a hard upper bound for ranged weapon
// range-increment calculations inherited from the 1D system.
const MaxCombatRange = 100

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
	// WeaponBonus is the item bonus from the equipped weapon's "+" designation.
	// Applied to both attack rolls and damage rolls. Zero for NPCs and unarmed combatants.
	WeaponBonus int
	// SpeedFt is this combatant's movement speed in feet per stride action.
	// 0 means 25 ft (PF2e default). Populated from NPC template at combat start;
	// always 0 (= 25 ft default) for players.
	SpeedFt int
	// WeaponName is the display name of the NPC's equipped weapon; empty = unarmed.
	WeaponName string
	// WeaponDefID is the registry ID of the equipped main-hand weapon (the inventory key).
	// Empty means unarmed or unknown. Used as the stable dedup key for the weapon's
	// item-typed SourceID ("item:<WeaponDefID>") in the effect.EffectSet pipeline so
	// the same key is produced at combat start and when a combatant joins a pending combat.
	WeaponDefID string
	// Hidden is true when this combatant is concealed. Attackers must pass a DC 11 flat check.
	// For player combatants: set by hide/divert actions; cleared when the player attacks or is targeted.
	// For NPC combatants: unused (always false).
	Hidden bool
	// RevealedUntilRound suppresses the DC 11 flat check for attackers through this round number.
	// Set by a successful Seek action to cbt.Round+1.
	RevealedUntilRound int
	// GridX is the column position on the combat grid (0 = leftmost, GridWidth-1 = rightmost).
	// Player combatants spawn at the first available column in row 0.
	GridX int
	// GridY is the row position on the combat grid (0 = player side, GridHeight-1 = NPC side).
	GridY int
	// CoverEquipmentID is the ItemID of the room equipment object this combatant is
	// using for cover. Empty string means the combatant is not in cover.
	CoverEquipmentID string
	// CoverTier is the tier of cover this combatant has: "lesser", "standard",
	// "greater", or "" (none).
	CoverTier string
	// AttackVerb is the verb used in combat attack narratives (e.g. "bites", "shoots").
	// Empty string means the default verb ("attacks") will be used.
	AttackVerb string
	// AttacksMadeThisRound counts attack rolls resolved for this combatant in the
	// current round, used to compute the cross-action Multiple Attack Penalty
	// (MAP). Reset to 0 in StartRoundWithSrc. Each ResolveAttack-producing action
	// (Attack, Strike, FireBurst, FireAutomatic) increments this counter once per
	// attack roll. The MAP penalty for the next attack is -5 * min(AttacksMadeThisRound, 2).
	AttacksMadeThisRound int
	// FactionID is the faction this combatant belongs to; empty for players and faction-less NPCs.
	// Used by the Lua scripting layer for faction-aware targeting (e.g. get_faction_enemies).
	FactionID string
	// ReactionBudget tracks this combatant's per-round reaction spending.
	// Nil before the first StartRound call. Reset by StartRoundWithSrc each round.
	ReactionBudget *reaction.Budget
	// Effects is the unified typed-bonus set for this combatant.
	// Populated at combatant creation from conditions + feat/tech passive bonuses + equipment bonuses.
	// Kept in sync with ActiveSet condition state via SyncConditionApply/SyncConditionRemove/SyncConditionsTick.
	Effects *effect.EffectSet
}

// SpeedSquares returns the number of grid squares this combatant may move per stride action.
// SpeedFt == 0 is treated as the PF2e default of 25 ft = 5 squares.
// Minimum 1 square.
//
// Postcondition: returns >= 1.
func (c *Combatant) SpeedSquares() int {
	ft := c.SpeedFt
	if ft <= 0 {
		ft = 25
	}
	sq := ft / 5
	if sq < 1 {
		sq = 1
	}
	return sq
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

// CombatRange returns the Chebyshev (chessboard) distance in feet between two combatants.
// Chebyshev distance = max(|dx|, |dy|) squares × 5 ft/square.
//
// Precondition: none.
// Postcondition: Returns non-negative distance in feet.
func CombatRange(a, b Combatant) int {
	dx := a.GridX - b.GridX
	if dx < 0 {
		dx = -dx
	}
	dy := a.GridY - b.GridY
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx * 5
	}
	return dy * 5
}

// CellOccupied reports whether any living combatant other than actorID occupies
// grid position (x, y).
//
// Precondition: cbt must not be nil.
// Postcondition: Returns true iff a living combatant other than actorID is at (x, y).
func CellOccupied(cbt *Combat, actorID string, x, y int) bool {
	for _, c := range cbt.Combatants {
		if c.ID != actorID && !c.IsDead() && c.GridX == x && c.GridY == y {
			return true
		}
	}
	return false
}

// CellBlockedByCover reports whether a cover object occupies grid position
// (x, y). Destroyed cover objects are removed from cbt.CoverObjects by the
// combat handler and therefore no longer block.
//
// Precondition: cbt must not be nil.
// Postcondition: Returns true iff any entry in cbt.CoverObjects has
// GridX == x && GridY == y.
func CellBlockedByCover(cbt *Combat, x, y int) bool {
	for _, co := range cbt.CoverObjects {
		if co.GridX == x && co.GridY == y {
			return true
		}
	}
	return false
}

// CellBlocked reports whether grid position (x, y) is unavailable for the
// given actor — either because another living combatant is there or because
// a cover object occupies the tile. Cover blocks movement for both players
// and NPCs until it is destroyed (GH #227).
//
// Precondition: cbt must not be nil.
// Postcondition: Returns true iff CellOccupied(cbt, actorID, x, y) or
// CellBlockedByCover(cbt, x, y).
func CellBlocked(cbt *Combat, actorID string, x, y int) bool {
	return CellOccupied(cbt, actorID, x, y) || CellBlockedByCover(cbt, x, y)
}

// IsFlanked reports whether target is flanked by the given attackers.
// A target is flanked when at least two attackers are in opposite quadrants:
// both row and column differ by ≥1 in opposite directions relative to the target.
//
// Precondition: none.
// Postcondition: Returns true iff the flanking condition is met.
func IsFlanked(target Combatant, attackers []Combatant) bool {
	for i := 0; i < len(attackers); i++ {
		for j := i + 1; j < len(attackers); j++ {
			a, b := attackers[i], attackers[j]
			adx := a.GridX - target.GridX
			ady := a.GridY - target.GridY
			bdx := b.GridX - target.GridX
			bdy := b.GridY - target.GridY
			if adx != 0 && ady != 0 && bdx != 0 && bdy != 0 {
				if sign(adx) != sign(bdx) && sign(ady) != sign(bdy) {
					return true
				}
			}
		}
	}
	return false
}

func sign(n int) int {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}


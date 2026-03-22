package gameserver

import (
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// DurabilityNotifier is called when an item is destroyed during combat.
// actorID is the combatant whose item was destroyed.
// message is the human-readable notification.
type DurabilityNotifier func(actorID, message string)

// DurabilityRoller satisfies inventory.Roller using a simple random source.
// It is used for destruction-chance rolls during combat durability deduction.
type DurabilityRoller struct {
	rnd *rand.Rand
}

// NewDurabilityRoller creates a DurabilityRoller seeded with the given source.
//
// Precondition: rndSrc must not be nil.
// Postcondition: Returns a non-nil DurabilityRoller.
func NewDurabilityRoller(rndSrc rand.Source) *DurabilityRoller {
	return &DurabilityRoller{rnd: rand.New(rndSrc)} //nolint:gosec // not security-sensitive
}

// Roll evaluates a dice expression of the form "[count]d<sides>[+modifier]"
// (e.g. "1d6", "2d6+4") and returns the total.
//
// Postcondition: result >= 1 for any valid expression with positive count and sides.
func (r *DurabilityRoller) Roll(expr string) int {
	count, sides, modifier := parseDiceExpr(expr)
	total := 0
	for i := 0; i < count; i++ {
		total += r.rnd.Intn(sides) + 1 //nolint:gosec
	}
	return total + modifier
}

// parseDiceExpr parses a dice expression of the form "[count]d<sides>[+/-modifier]".
// Defaults: count=1, sides=6, modifier=0.
func parseDiceExpr(expr string) (count, sides, modifier int) {
	count = 1
	sides = 6
	modifier = 0

	// Find the 'd' separator.
	dIdx := -1
	for i, ch := range expr {
		if ch == 'd' {
			dIdx = i
			break
		}
	}
	if dIdx < 0 {
		return
	}

	// Parse count (digits before 'd').
	if dIdx > 0 {
		n := 0
		for _, c := range expr[:dIdx] {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > 0 {
			count = n
		}
	}

	// Parse sides and optional +/- modifier after 'd'.
	rest := expr[dIdx+1:]
	plusIdx := -1
	for i, c := range rest {
		if (c == '+' || c == '-') && i > 0 {
			plusIdx = i
			break
		}
	}
	sidesStr := rest
	if plusIdx >= 0 {
		sidesStr = rest[:plusIdx]
	}
	s := 0
	for _, c := range sidesStr {
		if c >= '0' && c <= '9' {
			s = s*10 + int(c-'0')
		}
	}
	if s > 0 {
		sides = s
	}
	if plusIdx >= 0 {
		sign := 1
		if rest[plusIdx] == '-' {
			sign = -1
		}
		m := 0
		for _, c := range rest[plusIdx+1:] {
			if c >= '0' && c <= '9' {
				m = m*10 + int(c-'0')
			}
		}
		modifier = sign * m
	}
	return
}

// RollD20 rolls a single d20.
//
// Postcondition: result in [1, 20].
func (r *DurabilityRoller) RollD20() int {
	return r.rnd.Intn(20) + 1 //nolint:gosec
}

// RollFloat returns a random float64 in [0.0, 1.0) for probability checks.
//
// Postcondition: result in [0.0, 1.0).
func (r *DurabilityRoller) RollFloat() float64 {
	return r.rnd.Float64() //nolint:gosec
}

// EquippedWeaponGetter returns the active EquippedWeapon for actorID, or nil.
type EquippedWeaponGetter func(actorID string) *inventory.EquippedWeapon

// ArmorSlotRemover removes the armor from the given slot of targetID's equipment
// and returns the name of the removed item.
// It must clear the SlottedItem pointer for that slot.
type ArmorSlotRemover func(targetID string, slot inventory.ArmorSlot) string

// RandomOccupiedArmorSlot returns one occupied armor slot from eq at random using rnd,
// or the zero value if no slots are occupied.
//
// Precondition: eq must not be nil.
// Postcondition: returns a slot key that has a non-nil SlottedItem, or "" if none occupied.
func RandomOccupiedArmorSlot(eq *inventory.Equipment, rnd *rand.Rand) inventory.ArmorSlot {
	var occupied []inventory.ArmorSlot
	for slot, si := range eq.Armor {
		if si != nil {
			occupied = append(occupied, slot)
		}
	}
	if len(occupied) == 0 {
		return ""
	}
	return occupied[rnd.Intn(len(occupied))] //nolint:gosec
}

// ApplyRoundDurability processes combat round events and deducts durability for:
//   - The attacker's equipped weapon on each attack action (REQ-EM-5).
//   - A random occupied armor slot of the target when a hit lands (REQ-EM-6).
//
// When an item reaches 0 durability and the destruction roll succeeds (REQ-EM-10):
//   - For weapons: the EquippedWeapon.Durability is already 0; the caller must
//     check the returned DestroyedWeapons slice and clear the slot.
//   - For armor: the ArmorSlotRemover callback clears the slot immediately.
//   - notify is called with the combatant ID and a human-readable message.
//
// Precondition: events, getWeapon, removeArmor, notify, rng, and rnd must not be nil.
// Postcondition: EquippedWeapon.Durability values are mutated in place; destroyed
// armor slots are cleared via removeArmor; notify is called for each destruction.
func ApplyRoundDurability(
	events []combat.RoundEvent,
	getWeapon func(actorID string) *inventory.EquippedWeapon,
	getEquipment func(targetID string) *inventory.Equipment,
	removeArmor func(targetID string, slot inventory.ArmorSlot),
	notify DurabilityNotifier,
	rng inventory.Roller,
	rnd *rand.Rand,
) {
	for _, ev := range events {
		if ev.ActionType != combat.ActionAttack && ev.ActionType != combat.ActionStrike {
			continue
		}
		if ev.AttackResult == nil {
			continue
		}
		// REQ-EM-5: deduct weapon durability for the attacker on every attack roll.
		if ew := getWeapon(ev.ActorID); ew != nil && ew.Durability > 0 {
			// Build a temporary ItemInstance for DeductDurability from EquippedWeapon cached fields.
			rarity := ""
			if ew.Def != nil {
				rarity = ew.Def.Rarity
			}
			inst := &inventory.ItemInstance{
				InstanceID:    ew.InstanceID,
				Durability:    ew.Durability,
				MaxDurability: weaponMaxDurability(ew),
				Rarity:        rarity,
			}
			result := inventory.DeductDurability(inst, rng)
			ew.Durability = result.NewDurability
			if result.Destroyed {
				notify(ev.ActorID, "Your "+ew.Def.Name+" has been destroyed!")
			} else if result.BecameBroken {
				notify(ev.ActorID, "Your "+ew.Def.Name+" is broken (durability 0).")
			}
		}

		// REQ-EM-6/REQ-EM-10: on a hit, deduct from one random armor slot of target.
		r := ev.AttackResult
		if r.Outcome != combat.Success && r.Outcome != combat.CritSuccess {
			continue
		}
		if ev.TargetID == "" {
			continue
		}
		eq := getEquipment(ev.TargetID)
		if eq == nil {
			continue
		}
		slot := RandomOccupiedArmorSlot(eq, rnd)
		if slot == "" {
			continue
		}
		si := eq.Armor[slot]
		if si == nil {
			continue
		}
		if si.Durability <= 0 {
			continue
		}
		inst := &inventory.ItemInstance{
			InstanceID:    si.InstanceID,
			Durability:    si.Durability,
			MaxDurability: armorMaxDurability(si),
			Rarity:        si.Rarity,
		}
		result := inventory.DeductDurability(inst, rng)
		si.Durability = result.NewDurability
		if result.Destroyed {
			name := si.Name
			removeArmor(ev.TargetID, slot)
			notify(ev.TargetID, "Your "+name+" has been destroyed!")
		} else if result.BecameBroken {
			notify(ev.TargetID, "Your "+si.Name+" is broken (durability 0).")
		}
	}
}

// weaponMaxDurability infers MaxDurability from the weapon's rarity via the registry.
// Returns 0 if no rarity is known (destruction never occurs).
func weaponMaxDurability(ew *inventory.EquippedWeapon) int {
	if ew.Def == nil {
		return 0
	}
	if def, ok := inventory.LookupRarity(ew.Def.Rarity); ok {
		return def.MaxDurability
	}
	return 0
}

// armorMaxDurability infers MaxDurability from the SlottedItem's rarity.
// Returns 0 if no rarity is known.
func armorMaxDurability(si *inventory.SlottedItem) int {
	if def, ok := inventory.LookupRarity(si.Rarity); ok {
		return def.MaxDurability
	}
	return 0
}

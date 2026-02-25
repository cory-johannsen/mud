package inventory

import "errors"

// Slot identifies a carried-weapon position on a combatant.
type Slot string

const (
	// SlotPrimary is the primary weapon slot (e.g. long arm or main hand).
	SlotPrimary Slot = "primary"
	// SlotSecondary is the secondary weapon slot (e.g. off-hand).
	SlotSecondary Slot = "secondary"
	// SlotHolster is the holster slot for a sidearm.
	SlotHolster Slot = "holster"
)

// EquippedItem pairs a WeaponDef with its Magazine.
// Magazine is nil for melee weapons.
type EquippedItem struct {
	// Def is the static weapon definition.
	Def *WeaponDef
	// Magazine holds the loaded round state; nil when the weapon is melee.
	Magazine *Magazine
}

// Loadout tracks a combatant's equipped items.
// Invariant: each Slot holds at most one EquippedItem.
type Loadout struct {
	slots map[Slot]*EquippedItem
}

// NewLoadout returns an empty Loadout with no weapons equipped.
//
// Postcondition: all slots are empty.
func NewLoadout() *Loadout {
	return &Loadout{slots: make(map[Slot]*EquippedItem)}
}

// Equip places the given weapon into the specified slot.
// For firearms (IsFirearm() == true) a fully loaded Magazine is initialised.
// For melee weapons Magazine is nil.
//
// Precondition:  def must not be nil and must satisfy def.Validate().
// Postcondition: on success Equipped(slot) returns the new EquippedItem.
func (l *Loadout) Equip(slot Slot, def *WeaponDef) error {
	if def == nil {
		return errors.New("inventory: Loadout.Equip: def must not be nil")
	}
	if err := def.Validate(); err != nil {
		return err
	}

	item := &EquippedItem{Def: def}
	if def.IsFirearm() {
		item.Magazine = NewMagazine(def.ID, def.MagazineCapacity)
	}
	l.slots[slot] = item
	return nil
}

// Unequip removes the weapon from the specified slot.
//
// Postcondition: Equipped(slot) == nil.
func (l *Loadout) Unequip(slot Slot) {
	delete(l.slots, slot)
}

// Equipped returns the EquippedItem in the given slot, or nil if empty.
func (l *Loadout) Equipped(slot Slot) *EquippedItem {
	return l.slots[slot]
}

// Primary returns the EquippedItem in SlotPrimary, or nil if empty.
func (l *Loadout) Primary() *EquippedItem {
	return l.Equipped(SlotPrimary)
}

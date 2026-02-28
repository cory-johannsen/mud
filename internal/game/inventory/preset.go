package inventory

import "fmt"

// EquippedWeapon pairs a WeaponDef with its Magazine.
// Magazine is nil for melee weapons and shields (non-firearms).
type EquippedWeapon struct {
	// Def is the static weapon definition.
	Def *WeaponDef
	// Magazine holds the loaded round state; nil when the weapon is not a firearm.
	Magazine *Magazine
}

// WeaponPreset holds the main-hand and off-hand weapon slots for one loadout preset.
//
// Invariants:
//   - A two-handed main-hand weapon forces OffHand to nil.
//   - OffHand only accepts one-handed weapons or shields.
//   - A shield or one-handed weapon in OffHand requires MainHand to be one-handed or nil (not two-handed).
type WeaponPreset struct {
	// MainHand is the main-hand equipped weapon; nil when empty.
	MainHand *EquippedWeapon
	// OffHand is the off-hand equipped weapon; nil when empty or locked by a two-handed main.
	OffHand *EquippedWeapon
}

// NewWeaponPreset returns an empty WeaponPreset.
//
// Postcondition: MainHand == nil and OffHand == nil.
func NewWeaponPreset() *WeaponPreset {
	return &WeaponPreset{}
}

// newEquippedWeapon constructs an EquippedWeapon for def.
// For firearms (IsFirearm()==true) a full Magazine is initialised.
// Precondition: def must not be nil and must already have passed def.Validate()
// to ensure MagazineCapacity > 0 for any firearm.
func newEquippedWeapon(def *WeaponDef) *EquippedWeapon {
	ew := &EquippedWeapon{Def: def}
	if def.IsFirearm() {
		ew.Magazine = NewMagazine(def.ID, def.MagazineCapacity)
	}
	return ew
}

// EquipMainHand equips def in the main-hand slot.
// If def is two-handed, OffHand is cleared automatically.
//
// Precondition: def must not be nil and must satisfy def.Validate().
// Postcondition: MainHand is set; OffHand is nil when def is two-handed.
func (p *WeaponPreset) EquipMainHand(def *WeaponDef) error {
	if def == nil {
		return fmt.Errorf("inventory: WeaponPreset.EquipMainHand: def must not be nil")
	}
	if err := def.Validate(); err != nil {
		return err
	}
	p.MainHand = newEquippedWeapon(def)
	if def.IsTwoHanded() {
		p.OffHand = nil
	}
	return nil
}

// EquipOffHand equips def in the off-hand slot.
// Only one-handed weapons and shields are accepted; the main-hand must not be two-handed.
//
// Precondition: def must not be nil, must satisfy def.Validate(), must be one-handed or a shield,
// and MainHand must not hold a two-handed weapon.
// Postcondition: OffHand is set on success.
func (p *WeaponPreset) EquipOffHand(def *WeaponDef) error {
	if def == nil {
		return fmt.Errorf("inventory: WeaponPreset.EquipOffHand: def must not be nil")
	}
	if err := def.Validate(); err != nil {
		return err
	}
	if !def.IsOneHanded() && !def.IsShield() {
		return fmt.Errorf("inventory: WeaponPreset.EquipOffHand: off-hand slot only accepts one-handed weapons or shields, got kind=%q", def.Kind)
	}
	if p.MainHand != nil && p.MainHand.Def.IsTwoHanded() {
		return fmt.Errorf("inventory: WeaponPreset.EquipOffHand: cannot equip off-hand while a two-handed weapon is in the main hand")
	}
	p.OffHand = newEquippedWeapon(def)
	return nil
}

// UnequipMainHand removes the weapon from the main-hand slot.
//
// Postcondition: MainHand == nil.
func (p *WeaponPreset) UnequipMainHand() { p.MainHand = nil }

// UnequipOffHand removes the weapon from the off-hand slot.
//
// Postcondition: OffHand == nil.
func (p *WeaponPreset) UnequipOffHand() { p.OffHand = nil }

// LoadoutSet holds all weapon presets and tracks the active one.
// It enforces a once-per-round swap limit via SwappedThisRound.
type LoadoutSet struct {
	// Presets is the ordered list of available weapon presets.
	Presets []*WeaponPreset
	// Active is the zero-based index of the currently active preset.
	Active int
	// SwappedThisRound is true when the player has already swapped presets in the current combat round.
	// Reset to false by ResetRound at the start of each new round.
	SwappedThisRound bool
}

// NewLoadoutSet returns a LoadoutSet with two empty presets and Active=0.
//
// Postcondition: len(Presets)==2, Active==0, SwappedThisRound==false.
func NewLoadoutSet() *LoadoutSet {
	return &LoadoutSet{
		Presets: []*WeaponPreset{NewWeaponPreset(), NewWeaponPreset()},
	}
}

// ActivePreset returns the currently active WeaponPreset.
//
// Precondition: Active must be a valid index into Presets.
// Postcondition: Returns the preset at index Active, or nil if Active is out of range.
func (ls *LoadoutSet) ActivePreset() *WeaponPreset {
	if ls.Active < 0 || ls.Active >= len(ls.Presets) {
		return nil
	}
	return ls.Presets[ls.Active]
}

// Swap activates the preset at idx (0-based).
// This is a standard action limited to once per combat round.
//
// Precondition: SwappedThisRound must be false; idx must be in [0, len(Presets)).
// Postcondition: Active==idx, SwappedThisRound==true.
func (ls *LoadoutSet) Swap(idx int) error {
	if ls.SwappedThisRound {
		return fmt.Errorf("inventory: loadout already swapped this round")
	}
	if idx < 0 || idx >= len(ls.Presets) {
		return fmt.Errorf("inventory: preset index %d out of range [0,%d)", idx, len(ls.Presets))
	}
	if idx == ls.Active {
		// Swapping to the already-active preset is a no-op; does not consume the round action.
		return nil
	}
	ls.Active = idx
	ls.SwappedThisRound = true
	return nil
}

// ResetRound clears SwappedThisRound, called at the start of each combat round.
//
// Postcondition: SwappedThisRound==false.
func (ls *LoadoutSet) ResetRound() { ls.SwappedThisRound = false }

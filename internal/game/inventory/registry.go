package inventory

import "fmt"

// Registry holds all loaded weapon, explosive, and item definitions indexed by ID.
type Registry struct {
	weapons    map[string]*WeaponDef
	explosives map[string]*ExplosiveDef
	items      map[string]*ItemDef
}

// NewRegistry returns an empty Registry.
//
// Postcondition: all internal maps are initialised.
func NewRegistry() *Registry {
	return &Registry{
		weapons:    make(map[string]*WeaponDef),
		explosives: make(map[string]*ExplosiveDef),
		items:      make(map[string]*ItemDef),
	}
}

// RegisterWeapon adds w to the registry.
//
// Precondition:  w must not be nil.
// Postcondition: Weapon(w.ID) returns w; returns error if w.ID already registered.
func (r *Registry) RegisterWeapon(w *WeaponDef) error {
	if _, exists := r.weapons[w.ID]; exists {
		return fmt.Errorf("inventory: Registry.RegisterWeapon: weapon ID %q already registered", w.ID)
	}
	r.weapons[w.ID] = w
	return nil
}

// RegisterExplosive adds e to the registry.
//
// Precondition:  e must not be nil.
// Postcondition: Explosive(e.ID) returns e; returns error if e.ID already registered.
func (r *Registry) RegisterExplosive(e *ExplosiveDef) error {
	if _, exists := r.explosives[e.ID]; exists {
		return fmt.Errorf("inventory: Registry.RegisterExplosive: explosive ID %q already registered", e.ID)
	}
	r.explosives[e.ID] = e
	return nil
}

// Weapon returns the WeaponDef for the given id, or nil if not found.
func (r *Registry) Weapon(id string) *WeaponDef {
	return r.weapons[id]
}

// Explosive returns the ExplosiveDef for the given id, or nil if not found.
func (r *Registry) Explosive(id string) *ExplosiveDef {
	return r.explosives[id]
}

// RegisterItem adds d to the registry.
//
// Precondition:  d must not be nil.
// Postcondition: Item(d.ID) returns (d, true); returns error if d.ID already registered.
func (r *Registry) RegisterItem(d *ItemDef) error {
	if _, exists := r.items[d.ID]; exists {
		return fmt.Errorf("inventory: Registry.RegisterItem: item ID %q already registered", d.ID)
	}
	r.items[d.ID] = d
	return nil
}

// Item returns the ItemDef for the given id and whether it was found.
//
// Postcondition: ok is true iff the id is registered.
func (r *Registry) Item(id string) (*ItemDef, bool) {
	d, ok := r.items[id]
	return d, ok
}

// AllWeapons returns all registered WeaponDefs in unspecified order.
//
// Postcondition: len(result) == number of registered weapons.
func (r *Registry) AllWeapons() []*WeaponDef {
	out := make([]*WeaponDef, 0, len(r.weapons))
	for _, w := range r.weapons {
		out = append(out, w)
	}
	return out
}

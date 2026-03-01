package inventory

import "fmt"

// Registry holds all loaded weapon, explosive, item, and armor definitions indexed by ID.
type Registry struct {
	weapons    map[string]*WeaponDef
	explosives map[string]*ExplosiveDef
	items      map[string]*ItemDef
	armors     map[string]*ArmorDef
}

// NewRegistry returns an empty Registry.
//
// Postcondition: all internal maps are initialised.
func NewRegistry() *Registry {
	return &Registry{
		weapons:    make(map[string]*WeaponDef),
		explosives: make(map[string]*ExplosiveDef),
		items:      make(map[string]*ItemDef),
		armors:     make(map[string]*ArmorDef),
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

// ItemByArmorRef returns the first ItemDef whose ArmorRef matches armorDefID, or false if none found.
//
// Precondition: armorDefID must be non-empty.
// Postcondition: Returns (def, true) if a matching item is found; (nil, false) otherwise.
func (r *Registry) ItemByArmorRef(armorDefID string) (*ItemDef, bool) {
	for _, d := range r.items {
		if d.ArmorRef == armorDefID {
			return d, true
		}
	}
	return nil, false
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

// RegisterArmor adds an ArmorDef to the registry.
//
// Precondition: def must be non-nil with a non-empty ID.
// Postcondition: Returns error if an armor with the same ID is already registered.
func (r *Registry) RegisterArmor(def *ArmorDef) error {
	if _, exists := r.armors[def.ID]; exists {
		return fmt.Errorf("inventory: Registry.RegisterArmor: armor ID %q already registered", def.ID)
	}
	r.armors[def.ID] = def
	return nil
}

// Armor returns the ArmorDef with the given ID, or false if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns (def, true) if found; (nil, false) otherwise.
func (r *Registry) Armor(id string) (*ArmorDef, bool) {
	def, ok := r.armors[id]
	return def, ok
}

// AllArmors returns all registered ArmorDef instances in unspecified order.
//
// Postcondition: Returns a non-nil slice; may be empty if no armors registered.
func (r *Registry) AllArmors() []*ArmorDef {
	out := make([]*ArmorDef, 0, len(r.armors))
	for _, def := range r.armors {
		out = append(out, def)
	}
	return out
}

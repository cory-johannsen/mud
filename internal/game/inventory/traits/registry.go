// Package traits provides the canonical registry of weapon traits used by the
// MUD game engine. Traits are content-tagged on weapons (via the `traits:` YAML
// list on WeaponDef) and the registry maps each trait id to a Behavior struct
// that downstream systems consult at runtime.
//
// The registry is a single source of truth (REQ-WMOVE-3): all weapon-trait
// behaviour is keyed off a Behavior in this registry. Unknown traits warn but
// do not error (REQ-WMOVE-4) so content can land before behaviour ships.
//
// To avoid an import cycle between this package and internal/game/combat, the
// Behavior.GrantsFreeAction field is a string action key rather than a typed
// combat.ActionType. The combat package owns the canonical key constants
// (e.g. ActionMoveTraitStrideKey) and consumers translate the key locally.
package traits

import (
	"log/slog"
)

// Trait identifiers. Add new entries here when adding new traits.
const (
	// Mobile is the canonical id for the Move-trait weapon. Grants a free Stride
	// before or after a Strike with this weapon. Aliased from the legacy `move`
	// id at registry lookup time (WMOVE-Q1).
	Mobile = "mobile"

	// Reach is reserved here for future use; behaviour ships in a separate ticket.
	Reach = "reach"
)

// Action key constants — used by Behavior.GrantsFreeAction. Combat consumers
// translate these keys to their typed ActionType locally to avoid an import
// cycle (combat depends on inventory).
const (
	// ActionMoveTraitStrideKey identifies the free Stride granted by the Mobile
	// trait. Translated to combat.ActionMoveTraitStride at the consumer.
	ActionMoveTraitStrideKey = "move_trait_stride"
)

// Behavior is the runtime contract a trait id resolves to.
type Behavior struct {
	// ID is the canonical trait identifier (e.g. "mobile").
	ID string
	// DisplayName is the human-readable trait name.
	DisplayName string
	// Description is the runtime-behaviour description (player-facing).
	Description string
	// GrantsFreeAction, when non-empty, indicates the action key the trait
	// queues as a free action when its triggering condition is met. Empty
	// means the trait does not grant a free action.
	GrantsFreeAction string
	// SuppressesReactions, when true, marks any movement caused by this trait
	// as one that does not provoke reactive strikes / opportunity attacks.
	SuppressesReactions bool
}

// Registry maps trait ids to Behaviors and known aliases to canonical ids.
type Registry struct {
	behaviors map[string]*Behavior
	aliases   map[string]string
}

// DefaultRegistry returns the canonical registry shipped with the binary.
//
// Postcondition: returns a non-nil Registry containing at minimum the Mobile
// trait and the `move` -> Mobile alias.
func DefaultRegistry() *Registry {
	return &Registry{
		behaviors: map[string]*Behavior{
			Mobile: {
				ID:                  Mobile,
				DisplayName:         "Mobile",
				Description:         "Grants a free Stride before or after a Strike with this weapon. The free Stride does not provoke reactive strikes and does not consume action points.",
				GrantsFreeAction:    ActionMoveTraitStrideKey,
				SuppressesReactions: true,
			},
		},
		aliases: map[string]string{
			"move": Mobile,
		},
	}
}

// CanonicalID returns the canonical trait id for the given input. Aliases are
// resolved to their target id; unknown ids are returned unchanged so the caller
// can decide how to handle them.
//
// Postcondition: returns a non-empty string for every non-empty input.
func (r *Registry) CanonicalID(id string) string {
	if c, ok := r.aliases[id]; ok {
		return c
	}
	return id
}

// Behavior returns the registered Behavior for id (after alias resolution), or
// nil if no behaviour is registered.
//
// Postcondition: returns nil iff CanonicalID(id) is not present in the registry.
func (r *Registry) Behavior(id string) *Behavior {
	return r.behaviors[r.CanonicalID(id)]
}

// HasBehavior reports whether id (after alias resolution) is registered.
func (r *Registry) HasBehavior(id string) bool {
	return r.Behavior(id) != nil
}

// Validate inspects the given trait ids and emits a warning log entry for each
// id that is not present in the registry. It never returns an error: unknown
// traits are explicitly tolerated so content can land before behaviour ships
// (WMOVE-4).
//
// Postcondition: returns nil. For each unknown id, a single warning log entry
// is emitted at zerolog.WarnLevel including the trait id.
func (r *Registry) Validate(ids []string) error {
	for _, id := range ids {
		if r.Behavior(id) == nil {
			slog.Warn("unknown weapon trait — registry has no behavior", "trait", id)
		}
	}
	return nil
}

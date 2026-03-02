// Package inventory_test contains content completeness tests for archetype starting loadouts.
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

var archetypes = []string{"aggressor", "criminal", "drifter", "influencer", "nerd", "normie"}

// buildTestRegistry loads all weapons, armors, and items from the content directories
// and registers them in a Registry suitable for ID-resolution checks.
func buildTestRegistry(t *testing.T) *inventory.Registry {
	t.Helper()
	reg := inventory.NewRegistry()

	weapons, err := inventory.LoadWeapons("../../../content/weapons")
	require.NoError(t, err, "LoadWeapons must not error")
	for _, w := range weapons {
		require.NoError(t, reg.RegisterWeapon(w), "RegisterWeapon must not error for weapon %q", w.ID)
	}

	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err, "LoadArmors must not error")
	for _, a := range armors {
		require.NoError(t, reg.RegisterArmor(a), "RegisterArmor must not error for armor %q", a.ID)
	}

	items, err := inventory.LoadItems("../../../content/items")
	require.NoError(t, err, "LoadItems must not error")
	for _, item := range items {
		require.NoError(t, reg.RegisterItem(item), "RegisterItem must not error for item %q", item.ID)
	}

	return reg
}

// TestContent_AllArchetypeLoadoutsLoad verifies that every archetype YAML loads without
// error and produces a loadout with a non-empty weapon and positive currency.
func TestContent_AllArchetypeLoadoutsLoad(t *testing.T) {
	for _, arch := range archetypes {
		arch := arch
		t.Run(arch, func(t *testing.T) {
			sl, err := inventory.LoadStartingLoadout("../../../content/loadouts", arch, "", "")
			require.NoError(t, err, "archetype %q must load without error", arch)
			assert.NotEmpty(t, sl.Weapon, "archetype %q base loadout must have a weapon", arch)
			assert.Greater(t, sl.Currency, 0, "archetype %q base loadout must have positive currency", arch)
		})
	}
}

// TestContent_AllLoadoutItemRefsResolve verifies that every weapon, armor slot, and
// consumable item ID referenced in any archetype×team loadout resolves in the registry.
func TestContent_AllLoadoutItemRefsResolve(t *testing.T) {
	reg := buildTestRegistry(t)

	teams := []string{"", "gun", "machete"}

	for _, arch := range archetypes {
		for _, team := range teams {
			arch, team := arch, team
			label := arch + "/" + team
			if team == "" {
				label = arch + "/base"
			}
			t.Run(label, func(t *testing.T) {
				sl, err := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")
				require.NoError(t, err, "loadout %q must load without error", label)

				// Weapon ID must resolve in the weapon registry.
				if sl.Weapon != "" {
					w := reg.Weapon(sl.Weapon)
					assert.NotNil(t, w, "loadout %q: weapon ID %q not found in registry", label, sl.Weapon)
				}

				// Each armor slot ID must resolve as an armor def.
				for slot, armorID := range sl.Armor {
					if armorID == "" {
						continue
					}
					_, ok := reg.Armor(armorID)
					assert.True(t, ok, "loadout %q: armor slot %q references unknown armor ID %q", label, slot, armorID)
				}

				// Each consumable item ID must resolve in the item registry.
				for _, grant := range sl.Consumables {
					if grant.ItemID == "" {
						continue
					}
					_, ok := reg.Item(grant.ItemID)
					assert.True(t, ok, "loadout %q: consumable item ID %q not found in registry", label, grant.ItemID)
				}
			})
		}
	}
}

// TestProperty_StartingLoadout_TeamOverrideIsDeterministic verifies that loading the
// same archetype+team combination twice always produces identical results.
func TestProperty_StartingLoadout_TeamOverrideIsDeterministic(t *testing.T) {
	teams := []string{"", "gun", "machete"}

	rapid.Check(t, func(rt *rapid.T) {
		arch := rapid.SampledFrom(archetypes).Draw(rt, "arch")
		team := rapid.SampledFrom(teams).Draw(rt, "team")

		sl1, err1 := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")
		sl2, err2 := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")

		assert.Equal(rt, err1 == nil, err2 == nil, "error consistency for arch=%q team=%q", arch, team)
		if err1 == nil {
			assert.Equal(rt, sl1.Weapon, sl2.Weapon, "weapon must be deterministic for arch=%q team=%q", arch, team)
			assert.Equal(rt, sl1.Currency, sl2.Currency, "currency must be deterministic for arch=%q team=%q", arch, team)
		}
	})
}

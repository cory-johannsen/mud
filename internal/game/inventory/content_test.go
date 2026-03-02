// Package inventory_test contains content completeness property tests for the
// weapon and armor library.
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestContent_AllArmorSlotsValid verifies every armor YAML has a valid slot value.
func TestContent_AllArmorSlotsValid(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err, "content/armor should load without error")
	require.NotEmpty(t, armors, "at least one armor should exist")

	validSlots := inventory.ValidArmorSlots()
	for _, a := range armors {
		_, ok := validSlots[a.Slot]
		assert.True(t, ok, "armor %q has invalid slot %q", a.ID, a.Slot)
	}
}

// TestContent_AllWeaponArmorRefsResolve verifies every item YAML with armor_ref or
// weapon_ref references a def that can be loaded.
func TestContent_AllWeaponArmorRefsResolve(t *testing.T) {
	reg := inventory.NewRegistry()

	weapons, err := inventory.LoadWeapons("../../../content/weapons")
	require.NoError(t, err)
	for _, w := range weapons {
		require.NoError(t, reg.RegisterWeapon(w))
	}

	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)
	for _, a := range armors {
		require.NoError(t, reg.RegisterArmor(a))
	}

	items, err := inventory.LoadItems("../../../content/items")
	require.NoError(t, err)
	for _, item := range items {
		switch item.Kind {
		case "weapon":
			if item.WeaponRef != "" {
				w := reg.Weapon(item.WeaponRef)
				assert.NotNil(t, w, "item %q references unknown weapon_ref %q", item.ID, item.WeaponRef)
			}
		case "armor":
			if item.ArmorRef != "" {
				_, ok := reg.Armor(item.ArmorRef)
				assert.True(t, ok, "item %q references unknown armor_ref %q", item.ID, item.ArmorRef)
			}
		}
	}
}

// TestContent_AllArmorCrossTeamEffectsHaveValidKind verifies cross_team_effect.kind is
// always "condition" or "penalty".
func TestContent_AllArmorCrossTeamEffectsHaveValidKind(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)
	for _, a := range armors {
		if a.CrossTeamEffect != nil {
			assert.Contains(t, []string{"condition", "penalty"}, a.CrossTeamEffect.Kind,
				"armor %q cross_team_effect.kind must be condition or penalty", a.ID)
			assert.NotEmpty(t, a.CrossTeamEffect.Value,
				"armor %q cross_team_effect.value must not be empty", a.ID)
		}
	}
}

// TestContent_AllWeaponCrossTeamEffectsHaveValidKind mirrors the armor test for weapons.
func TestContent_AllWeaponCrossTeamEffectsHaveValidKind(t *testing.T) {
	weapons, err := inventory.LoadWeapons("../../../content/weapons")
	require.NoError(t, err)
	for _, w := range weapons {
		if w.CrossTeamEffect != nil {
			assert.Contains(t, []string{"condition", "penalty"}, w.CrossTeamEffect.Kind,
				"weapon %q cross_team_effect.kind must be condition or penalty", w.ID)
		}
	}
}

// TestProperty_ArmorACBonus_NonNegative verifies ac_bonus is always >= 0 in loaded content.
func TestProperty_ArmorACBonus_NonNegative(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)
	if len(armors) == 0 {
		t.Skip("no armors loaded")
	}

	rapid.Check(t, func(rt *rapid.T) {
		a := rapid.SampledFrom(armors).Draw(rt, "armor")
		assert.GreaterOrEqual(rt, a.ACBonus, 0, "armor %q has negative ac_bonus", a.ID)
	})
}

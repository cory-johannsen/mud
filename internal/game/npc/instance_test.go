package npc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func baseTauntTemplate() *npc.Template {
	return &npc.Template{
		ID:            "taunter",
		Name:          "Taunter",
		Description:   "A taunting NPC.",
		Level:         1,
		MaxHP:         10,
		AC:            10,
		Taunts:        []string{"Taunt A", "Taunt B", "Taunt C"},
		TauntChance:   1.0,
		TauntCooldown: "5s",
	}
}

func TestTryTaunt_ReturnsTauntWhenReady(t *testing.T) {
	tmpl := baseTauntTemplate()
	inst := npc.NewInstance("i1", tmpl, "room-1")
	now := time.Now()

	taunt, ok := inst.TryTaunt(now)
	require.True(t, ok)
	assert.NotEmpty(t, taunt)
	assert.Contains(t, tmpl.Taunts, taunt)
}

func TestTryTaunt_RespectsChanceZero(t *testing.T) {
	tmpl := baseTauntTemplate()
	tmpl.TauntChance = 0
	inst := npc.NewInstance("i1", tmpl, "room-1")

	taunt, ok := inst.TryTaunt(time.Now())
	assert.False(t, ok)
	assert.Empty(t, taunt)
}

func TestTryTaunt_RespectsCooldown(t *testing.T) {
	tmpl := baseTauntTemplate()
	inst := npc.NewInstance("i1", tmpl, "room-1")
	now := time.Now()

	// First taunt should succeed (chance=1.0)
	_, ok := inst.TryTaunt(now)
	require.True(t, ok)

	// Immediately after should fail (within 5s cooldown)
	_, ok = inst.TryTaunt(now.Add(1 * time.Second))
	assert.False(t, ok)

	// After cooldown should succeed
	_, ok = inst.TryTaunt(now.Add(6 * time.Second))
	assert.True(t, ok)
}

func TestTryTaunt_NoTauntsReturnsFalse(t *testing.T) {
	tmpl := baseTauntTemplate()
	tmpl.Taunts = nil
	inst := npc.NewInstance("i1", tmpl, "room-1")

	_, ok := inst.TryTaunt(time.Now())
	assert.False(t, ok)
}

func TestNewInstance_CopiesTauntFields(t *testing.T) {
	tmpl := baseTauntTemplate()
	inst := npc.NewInstance("i1", tmpl, "room-1")

	assert.Equal(t, tmpl.Taunts, inst.Taunts)
	assert.Equal(t, tmpl.TauntChance, inst.TauntChance)
	assert.Equal(t, 5*time.Second, inst.TauntCooldown)
}

func TestProperty_TryTaunt_NeverTauntsWhenChanceZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nTaunts := rapid.IntRange(1, 10).Draw(rt, "nTaunts")
		taunts := make([]string, nTaunts)
		for i := range taunts {
			taunts[i] = "taunt"
		}
		tmpl := &npc.Template{
			ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
			Taunts:      taunts,
			TauntChance: 0,
		}
		inst := npc.NewInstance("i", tmpl, "r")
		_, ok := inst.TryTaunt(time.Now())
		assert.False(rt, ok, "TryTaunt must never succeed when TauntChance == 0")
	})
}

func TestProperty_TryTaunt_NeverTauntsWithinCooldown(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cooldownSec := rapid.IntRange(5, 60).Draw(rt, "cooldownSec")
		tmpl := &npc.Template{
			ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
			Taunts:        []string{"taunt"},
			TauntChance:   1.0,
			TauntCooldown: fmt.Sprintf("%ds", cooldownSec),
		}
		inst := npc.NewInstance("i", tmpl, "r")

		now := time.Now()
		// Force first taunt
		_, ok := inst.TryTaunt(now)
		require.True(rt, ok)

		// Check within cooldown window
		withinSec := rapid.IntRange(0, cooldownSec-1).Draw(rt, "withinSec")
		_, ok = inst.TryTaunt(now.Add(time.Duration(withinSec) * time.Second))
		assert.False(rt, ok, "TryTaunt must not fire within cooldown window")
	})
}

func TestNewInstance_PicksWeaponFromTable(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
		Weapon: []npc.EquipmentEntry{
			{ID: "cheap_blade", Weight: 1},
		},
	}
	inst := npc.NewInstance("id1", tmpl, "room1")
	assert.Equal(t, "cheap_blade", inst.WeaponID)
}

func TestNewInstance_NoWeapon_EmptyWeaponID(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
	}
	inst := npc.NewInstance("id1", tmpl, "room1")
	assert.Empty(t, inst.WeaponID)
}

func TestNewInstanceWithResolver_ArmorACBonusAddedToBase(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
		Armor: []npc.EquipmentEntry{{ID: "test_armor", Weight: 1}},
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", func(armorID string) int {
		if armorID == "test_armor" {
			return 3
		}
		return 0
	})
	assert.Equal(t, "test_armor", inst.ArmorID)
	assert.Equal(t, 15, inst.AC) // 12 + 3
}

func TestNewInstanceWithResolver_NoArmor_ACUnchanged(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil)
	assert.Empty(t, inst.ArmorID)
	assert.Equal(t, 12, inst.AC)
}

func TestNewInstanceWithResolver_NilResolver_NoACBonus(t *testing.T) {
	// Even with an armor entry, nil resolver means no AC adjustment.
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
		Armor: []npc.EquipmentEntry{{ID: "leather_jacket", Weight: 1}},
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil)
	assert.Equal(t, "leather_jacket", inst.ArmorID)
	assert.Equal(t, 12, inst.AC) // no bonus — resolver is nil
}

func TestNewInstance_HustleCopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test-npc", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
		Hustle: 7,
	}
	inst := npc.NewInstance("inst-1", tmpl, "room_a")
	if inst.Hustle != 7 {
		t.Errorf("expected Hustle=7, got %d", inst.Hustle)
	}
}

func TestNewInstance_HustleDefaultsToZero(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test-npc2", Name: "Test2", Level: 1, MaxHP: 10, AC: 10,
	}
	inst := npc.NewInstance("inst-2", tmpl, "room_a")
	if inst.Hustle != 0 {
		t.Errorf("expected Hustle=0, got %d", inst.Hustle)
	}
}

func TestInstance_AbilityCooldowns_LazyInit(t *testing.T) {
	inst := &npc.Instance{}
	if inst.AbilityCooldowns != nil {
		t.Error("AbilityCooldowns should be nil at zero value")
	}
	count := 0
	for range inst.AbilityCooldowns {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 iterations over nil map, got %d", count)
	}
}

func TestManager_Spawn_AppliesArmorACBonus(t *testing.T) {
	mgr := npc.NewManager()
	mgr.SetArmorACResolver(func(armorID string) int {
		if armorID == "leather_jacket" {
			return 2
		}
		return 0
	})
	tmpl := &npc.Template{
		ID: "guard", Name: "Guard", Level: 1, MaxHP: 10, AC: 12, Perception: 4,
		Armor: []npc.EquipmentEntry{{ID: "leather_jacket", Weight: 1}},
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	require.NoError(t, err)
	assert.Equal(t, 14, inst.AC) // 12 + 2
	assert.Equal(t, "leather_jacket", inst.ArmorID)
}

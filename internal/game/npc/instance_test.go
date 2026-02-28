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

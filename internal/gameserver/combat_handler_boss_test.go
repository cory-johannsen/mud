package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestNPCEffectiveStats_BrutalStrike_AddsDamage(t *testing.T) {
	feats := []*ruleset.Feat{
		{ID: "brutal_strike", Name: "Brutal Strike", AllowNPC: true},
	}
	registry := ruleset.NewFeatRegistry(feats)
	inst := &npc.Instance{Feats: []string{"brutal_strike"}}

	stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
	assert.Equal(t, 2, stats.DamageBonus)
}

func TestNPCEffectiveStats_Evasive_AddsAC(t *testing.T) {
	feats := []*ruleset.Feat{
		{ID: "evasive", Name: "Evasive", AllowNPC: true},
	}
	registry := ruleset.NewFeatRegistry(feats)
	inst := &npc.Instance{AC: 14, Feats: []string{"evasive"}}

	stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
	assert.Equal(t, 2, stats.ACBonus)
}

func TestNPCEffectiveStats_PackTactics_AllyPresent(t *testing.T) {
	feats := []*ruleset.Feat{
		{ID: "pack_tactics", Name: "Pack Tactics", AllowNPC: true},
	}
	registry := ruleset.NewFeatRegistry(feats)
	ally := &npc.Instance{ID: "ally1", Feats: nil}
	attacker := &npc.Instance{ID: "attacker", Feats: []string{"pack_tactics"}}
	roomNPCs := []*npc.Instance{attacker, ally}

	stats := gameserver.ComputeNPCEffectiveStats(attacker, registry, roomNPCs)
	assert.Equal(t, 2, stats.AttackBonus)
}

func TestNPCEffectiveStats_PackTactics_NoAlly(t *testing.T) {
	feats := []*ruleset.Feat{
		{ID: "pack_tactics", Name: "Pack Tactics", AllowNPC: true},
	}
	registry := ruleset.NewFeatRegistry(feats)
	attacker := &npc.Instance{ID: "solo", Feats: []string{"pack_tactics"}}
	roomNPCs := []*npc.Instance{attacker} // alone

	stats := gameserver.ComputeNPCEffectiveStats(attacker, registry, roomNPCs)
	assert.Equal(t, 0, stats.AttackBonus)
}

func TestProperty_NPCEffectiveStats_NeverNegativeWithValidFeats(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		featIDs := rapid.SliceOfDistinct(
			rapid.SampledFrom([]string{"brutal_strike", "evasive"}),
			func(s string) string { return s },
		).Draw(t, "feats")

		feats := []*ruleset.Feat{
			{ID: "brutal_strike", AllowNPC: true},
			{ID: "evasive", AllowNPC: true},
		}
		registry := ruleset.NewFeatRegistry(feats)
		inst := &npc.Instance{Feats: featIDs, AC: 12}

		stats := gameserver.ComputeNPCEffectiveStats(inst, registry, nil)
		assert.GreaterOrEqual(t, stats.DamageBonus, 0)
		assert.GreaterOrEqual(t, stats.ACBonus, 0)
		assert.GreaterOrEqual(t, stats.AttackBonus, 0)
	})
}

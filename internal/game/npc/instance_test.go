package npc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/xp"
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
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
		Weapon: []npc.EquipmentEntry{
			{ID: "cheap_blade", Weight: 1},
		},
	}
	inst := npc.NewInstance("id1", tmpl, "room1")
	assert.Equal(t, "cheap_blade", inst.WeaponID)
}

func TestNewInstance_NoWeapon_EmptyWeaponID(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
	}
	inst := npc.NewInstance("id1", tmpl, "room1")
	assert.Empty(t, inst.WeaponID)
}

func TestNewInstanceWithResolver_ArmorACBonusAddedToBase(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
		Armor: []npc.EquipmentEntry{{ID: "test_armor", Weight: 1}},
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", func(armorID string) int {
		if armorID == "test_armor" {
			return 3
		}
		return 0
	}, nil, nil)
	assert.Equal(t, "test_armor", inst.ArmorID)
	assert.Equal(t, 15, inst.AC) // 12 + 3
}

func TestNewInstanceWithResolver_NoArmor_ACUnchanged(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, nil, nil)
	assert.Empty(t, inst.ArmorID)
	assert.Equal(t, 12, inst.AC)
}

func TestNewInstanceWithResolver_NilResolver_NoACBonus(t *testing.T) {
	// Even with an armor entry, nil resolver means no AC adjustment.
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
		Armor: []npc.EquipmentEntry{{ID: "leather_jacket", Weight: 1}},
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, nil, nil)
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

func TestInstance_NPCTypeFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test_npc", Name: "Test NPC", Level: 1, MaxHP: 10, AC: 12,
		NPCType:  "merchant",
		Merchant: &npc.MerchantConfig{ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4}},
	}
	inst := npc.NewInstance("inst-1", tmpl, "room-1")
	assert.Equal(t, "merchant", inst.NPCType, "NPCType must be copied from template")
}

func TestInstance_PersonalityFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test_guard", Name: "Guard", Level: 2, MaxHP: 20, AC: 14,
		NPCType: "guard", Personality: "brave",
		Guard: &npc.GuardConfig{WantedThreshold: 2},
	}
	inst := npc.NewInstance("inst-2", tmpl, "room-1")
	assert.Equal(t, "brave", inst.Personality, "Personality must be copied from template")
}

func TestInstance_CombatNPCType(t *testing.T) {
	tmpl := &npc.Template{ID: "bandit", Name: "Bandit", Level: 1, MaxHP: 20, AC: 12, NPCType: "combat"}
	inst := npc.NewInstance("inst-3", tmpl, "room-1")
	assert.Equal(t, "combat", inst.NPCType, "combat NPCType must propagate")
}

func TestInstance_CoweringDefaultsFalse(t *testing.T) {
	tmpl := &npc.Template{ID: "test_npc", Name: "NPC", Level: 1, MaxHP: 10, AC: 12, NPCType: "combat"}
	inst := npc.NewInstance("inst-4", tmpl, "room-1")
	assert.False(t, inst.Cowering, "Cowering must default to false at spawn")
}

func TestManager_SpawnPropagatesNPCType(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID: "healer_npc", Name: "Healer", Level: 1, MaxHP: 10, AC: 10,
		NPCType: "healer",
		Healer:  &npc.HealerConfig{PricePerHP: 5, DailyCapacity: 200},
	}
	inst, err := mgr.Spawn(tmpl, "room-heal")
	require.NoError(t, err)
	assert.Equal(t, "healer", inst.NPCType, "Manager.Spawn must propagate NPCType")
}

// TestProperty_Instance_NPCTypeAlwaysPropagates checks that spawning any NPC
// template always produces an instance with the same NPCType as the template.
func TestProperty_Instance_NPCTypeAlwaysPropagates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcType := rapid.SampledFrom([]string{
			"combat", "merchant", "guard", "healer",
			"quest_giver", "hireling", "banker", "job_trainer", "crafter",
		}).Draw(rt, "npc_type")
		tmpl := &npc.Template{
			ID:      "prop_npc",
			Name:    "Prop NPC",
			Level:   1,
			MaxHP:   10,
			AC:      12,
			NPCType: npcType,
		}
		inst := npc.NewInstance("prop-inst", tmpl, "prop-room")
		if inst.NPCType != npcType {
			rt.Fatalf("expected NPCType %q, got %q", npcType, inst.NPCType)
		}
	})
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
		ID: "guard", Name: "Guard", Level: 1, MaxHP: 10, AC: 12, Awareness: 4,
		Armor: []npc.EquipmentEntry{{ID: "leather_jacket", Weight: 1}},
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	require.NoError(t, err)
	assert.Equal(t, 14, inst.AC) // 12 + 2
	assert.Equal(t, "leather_jacket", inst.ArmorID)
}

func TestNewInstanceWithResolver_NpcRolePropagated(t *testing.T) {
	tmpl := &npc.Template{
		ID:      "test-merchant",
		Name:    "Merchant Bob",
		Level:   1,
		MaxHP:   10,
		AC:      10,
		NpcRole: "merchant",
		NPCType: "merchant",
		Merchant: &npc.MerchantConfig{
			Budget: 50,
			ReplenishRate: npc.ReplenishConfig{
				MinHours: 1,
				MaxHours: 2,
			},
		},
	}
	inst := npc.NewInstanceWithResolver("inst-1", tmpl, "room-1", nil, nil, nil)
	if inst.NpcRole != "merchant" {
		t.Errorf("NpcRole = %q, want %q", inst.NpcRole, "merchant")
	}
}

func TestNewInstanceWithResolver_NpcRoleEmptyByDefault(t *testing.T) {
	tmpl := &npc.Template{
		ID:      "test-combat",
		Name:    "Bandit",
		Level:   1,
		MaxHP:   10,
		AC:      10,
		NPCType: "combat",
	}
	inst := npc.NewInstanceWithResolver("inst-2", tmpl, "room-1", nil, nil, nil)
	if inst.NpcRole != "" {
		t.Errorf("NpcRole = %q, want empty string", inst.NpcRole)
	}
}

// ---- NHN: spawn propagation and resistance defaults ----

func baseNHNTemplate(id, name, typ string) *npc.Template {
	return &npc.Template{
		ID:    id,
		Name:  name,
		Type:  typ,
		Level: 3,
		MaxHP: 30,
		AC:    12,
	}
}

func TestSpawnPropagatesAttackVerb(t *testing.T) {
	tmpl := baseNHNTemplate("dog", "Dog", "animal")
	tmpl.AttackVerb = "bites"
	inst := npc.NewInstanceWithResolver("inst1", tmpl, "room1", nil, nil, nil)
	if inst.AttackVerb != "bites" {
		t.Errorf("AttackVerb: got %q, want %q", inst.AttackVerb, "bites")
	}
}

func TestSpawnPropagatesImmobile(t *testing.T) {
	tmpl := baseNHNTemplate("turret", "Turret", "machine")
	tmpl.Immobile = true
	inst := npc.NewInstanceWithResolver("inst2", tmpl, "room1", nil, nil, nil)
	if !inst.Immobile {
		t.Error("expected Immobile == true")
	}
}

func TestRobotSpawnResistanceDefaults(t *testing.T) {
	tmpl := baseNHNTemplate("robot", "Robot", "robot")
	inst := npc.NewInstanceWithResolver("inst3", tmpl, "room1", nil, nil, nil)
	if inst.Resistances["bleed"] != 999 {
		t.Errorf("robot bleed resistance: got %d, want 999", inst.Resistances["bleed"])
	}
	if inst.Resistances["poison"] != 999 {
		t.Errorf("robot poison resistance: got %d, want 999", inst.Resistances["poison"])
	}
}

func TestRobotSpawnResistanceTemplateOverrides(t *testing.T) {
	tmpl := baseNHNTemplate("robot2", "Robot2", "robot")
	tmpl.Resistances = map[string]int{"bleed": 5, "fire": 10}
	inst := npc.NewInstanceWithResolver("inst4", tmpl, "room1", nil, nil, nil)
	if inst.Resistances["bleed"] != 5 {
		t.Errorf("robot bleed override: got %d, want 5", inst.Resistances["bleed"])
	}
	if inst.Resistances["poison"] != 999 {
		t.Errorf("robot poison resistance: got %d, want 999", inst.Resistances["poison"])
	}
	if inst.Resistances["fire"] != 10 {
		t.Errorf("robot fire resistance: got %d, want 10", inst.Resistances["fire"])
	}
}

func TestMachineSpawnResistanceDefaults(t *testing.T) {
	tmpl := baseNHNTemplate("machine", "Machine", "machine")
	inst := npc.NewInstanceWithResolver("inst5", tmpl, "room1", nil, nil, nil)
	if inst.Resistances["bleed"] != 999 {
		t.Errorf("machine bleed resistance: got %d, want 999", inst.Resistances["bleed"])
	}
	if inst.Resistances["poison"] != 999 {
		t.Errorf("machine poison resistance: got %d, want 999", inst.Resistances["poison"])
	}
}

func TestHumanSpawnNoResistanceDefaults(t *testing.T) {
	tmpl := baseNHNTemplate("human", "Human", "human")
	inst := npc.NewInstanceWithResolver("inst6", tmpl, "room1", nil, nil, nil)
	if inst.Resistances != nil && inst.Resistances["bleed"] > 0 {
		t.Errorf("human should not have bleed resistance, got %d", inst.Resistances["bleed"])
	}
}

func TestInstance_HasTag_True(t *testing.T) {
	inst := &npc.Instance{Tags: []string{"undead", "flying"}}
	assert.True(t, inst.HasTag("undead"))
	assert.True(t, inst.HasTag("flying"))
}

func TestInstance_HasTag_False(t *testing.T) {
	inst := &npc.Instance{Tags: []string{"undead"}}
	assert.False(t, inst.HasTag("robot"))
}

func TestInstance_HasTag_Empty(t *testing.T) {
	inst := &npc.Instance{}
	assert.False(t, inst.HasTag("anything"))
}

func TestProperty_Instance_HasTag_Reflexive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tags := rapid.SliceOf(rapid.StringN(1, 10, -1)).Draw(t, "tags")
		inst := &npc.Instance{Tags: tags}
		for _, tag := range tags {
			assert.True(t, inst.HasTag(tag), "HasTag must return true for any tag in Tags")
		}
	})
}

func TestNewInstanceWithResolver_TierHPScaling(t *testing.T) {
	cfg := &xp.XPConfig{
		TierMultipliers: map[string]xp.TierMultiplier{
			"minion":   {HP: 0.75}, "standard": {HP: 1.0},
			"elite":    {HP: 1.5},  "champion": {HP: 2.0},
			"boss":     {HP: 3.0},
		},
	}
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 20, AC: 10, Tier: "elite",
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, nil)
	// ceil(20 * 1.5) = 30
	assert.Equal(t, 30, inst.MaxHP)
	assert.Equal(t, 30, inst.CurrentHP)
}

func TestNewInstanceWithResolver_ToughFeatAddsFiveHP(t *testing.T) {
	cfg := &xp.XPConfig{
		TierMultipliers: map[string]xp.TierMultiplier{
			"standard": {HP: 1.0},
		},
	}
	toughFeat := &ruleset.Feat{ID: "tough", Name: "Tough", AllowNPC: true}
	registry := ruleset.NewFeatRegistry([]*ruleset.Feat{toughFeat})
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 20, AC: 10,
		Tier: "standard", Feats: []string{"tough"},
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, registry)
	// ceil(20 * 1.0) = 20, then + 5 from tough = 25
	assert.Equal(t, 25, inst.MaxHP)
	assert.Equal(t, 25, inst.CurrentHP)
}

func TestNewInstanceWithResolver_EmptyTierDefaultsToStandard(t *testing.T) {
	cfg := &xp.XPConfig{
		TierMultipliers: map[string]xp.TierMultiplier{
			"standard": {HP: 1.0},
		},
	}
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
		// Tier is empty — should default to "standard"
	}
	inst := npc.NewInstanceWithResolver("id1", tmpl, "room1", nil, cfg, nil)
	assert.Equal(t, 10, inst.MaxHP)
}

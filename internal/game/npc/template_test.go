package npc_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestTemplate_RespawnDelay_ParsesCorrectly(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "5m"
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "5m", tmpl.RespawnDelay)
}

func TestTemplate_RespawnDelay_EmptyByDefault(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "", tmpl.RespawnDelay)
}

func TestProperty_Template_ValidRespawnDelay_ParsesWithoutError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate valid duration strings
		value := rapid.IntRange(1, 3600).Draw(rt, "value")
		unit := rapid.SampledFrom([]string{"s", "m", "h"}).Draw(rt, "unit")
		delay := fmt.Sprintf("%d%s", value, unit)

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "%s"
`, delay))
		tmpl, err := npc.LoadTemplateFromBytes(data)
		require.NoError(rt, err)
		assert.Equal(rt, delay, tmpl.RespawnDelay)
	})
}

func TestTemplate_LootTable_ParsesFromYAML(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
loot:
  currency:
    min: 5
    max: 20
  items:
    - item: sword
      chance: 0.5
      min_qty: 1
      max_qty: 1
    - item: potion
      chance: 1.0
      min_qty: 1
      max_qty: 3
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, tmpl.Loot)
	require.NotNil(t, tmpl.Loot.Currency)
	assert.Equal(t, 5, tmpl.Loot.Currency.Min)
	assert.Equal(t, 20, tmpl.Loot.Currency.Max)
	require.Len(t, tmpl.Loot.Items, 2)
	assert.Equal(t, "sword", tmpl.Loot.Items[0].ItemID)
	assert.Equal(t, 0.5, tmpl.Loot.Items[0].Chance)
	assert.Equal(t, "potion", tmpl.Loot.Items[1].ItemID)
	assert.Equal(t, 1.0, tmpl.Loot.Items[1].Chance)
	assert.Equal(t, 3, tmpl.Loot.Items[1].MaxQty)
}

func TestTemplate_Taunts_ParsesCorrectly(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
taunt_chance: 0.3
taunt_cooldown: "30s"
taunts:
  - "You don't belong here."
  - "Keep walking."
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, 0.3, tmpl.TauntChance)
	assert.Equal(t, "30s", tmpl.TauntCooldown)
	assert.Equal(t, []string{"You don't belong here.", "Keep walking."}, tmpl.Taunts)
}

func TestTemplate_TauntChance_InvalidRange(t *testing.T) {
	for _, chance := range []string{"-0.1", "1.1", "2.0"} {
		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
taunt_chance: %s
`, chance))
		_, err := npc.LoadTemplateFromBytes(data)
		assert.Error(t, err, "expected error for taunt_chance=%s", chance)
	}
}

func TestTemplate_TauntCooldown_InvalidDuration(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
taunt_cooldown: "forever"
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err)
}

func TestProperty_Template_ValidTaunts_ParseWithoutError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		chance := rapid.Float64Range(0, 1).Draw(rt, "chance")
		value := rapid.IntRange(1, 3600).Draw(rt, "value")
		unit := rapid.SampledFrom([]string{"s", "m", "h"}).Draw(rt, "unit")
		cooldown := fmt.Sprintf("%d%s", value, unit)

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
taunt_chance: %f
taunt_cooldown: "%s"
taunts:
  - "Test taunt."
`, chance, cooldown))
		_, err := npc.LoadTemplateFromBytes(data)
		require.NoError(rt, err)
	})
}

func TestProperty_Template_InvalidTauntChance_ReturnsError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate chance outside [0, 1]
		negative := rapid.Bool().Draw(rt, "negative")
		var chance float64
		if negative {
			chance = -rapid.Float64Range(0.01, 100).Draw(rt, "chance")
		} else {
			chance = 1 + rapid.Float64Range(0.01, 100).Draw(rt, "chance")
		}

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
taunt_chance: %f
`, chance))
		_, err := npc.LoadTemplateFromBytes(data)
		assert.Error(rt, err, "expected error for taunt_chance=%f", chance)
	})
}

func TestProperty_Template_InvalidRespawnDelay_ReturnsError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate invalid duration strings (words that are not valid Go durations)
		invalid := rapid.SampledFrom([]string{"forever", "never", "5 minutes", "1day", "abc"}).Draw(rt, "invalid")

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "%s"
`, invalid))
		_, err := npc.LoadTemplateFromBytes(data)
		assert.Error(rt, err, "expected error for invalid respawn_delay %q", invalid)
	})
}

func TestTemplate_ResistancesWeaknesses_LoadedFromYAML(t *testing.T) {
	input := "id: test_npc\nname: Test NPC\ndescription: desc\nlevel: 1\nmax_hp: 10\nac: 10\nperception: 0\nresistances:\n  fire: 5\n  piercing: 2\nweaknesses:\n  electricity: 3\n"
	var tmpl npc.Template
	require.NoError(t, yaml.Unmarshal([]byte(input), &tmpl))
	assert.Equal(t, 5, tmpl.Resistances["fire"])
	assert.Equal(t, 2, tmpl.Resistances["piercing"])
	assert.Equal(t, 3, tmpl.Weaknesses["electricity"])
}

func TestNewInstance_CopiesResistancesWeaknesses(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
		Resistances: map[string]int{"fire": 5},
		Weaknesses:  map[string]int{"electricity": 3},
	}
	inst := npc.NewInstance("i1", tmpl, "room1")
	assert.Equal(t, 5, inst.Resistances["fire"])
	assert.Equal(t, 3, inst.Weaknesses["electricity"])
}

func TestLoadTemplateFromBytes_WeaponAndArmor(t *testing.T) {
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 12
perception: 4
weapon:
  - id: cheap_blade
    weight: 3
  - id: combat_knife
    weight: 1
armor:
  - id: leather_jacket
    weight: 1
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	require.Len(t, tmpl.Weapon, 2)
	assert.Equal(t, "cheap_blade", tmpl.Weapon[0].ID)
	assert.Equal(t, 3, tmpl.Weapon[0].Weight)
	assert.Equal(t, "combat_knife", tmpl.Weapon[1].ID)
	assert.Equal(t, 1, tmpl.Weapon[1].Weight)
	require.Len(t, tmpl.Armor, 1)
	assert.Equal(t, "leather_jacket", tmpl.Armor[0].ID)
	assert.Equal(t, 1, tmpl.Armor[0].Weight)
}

func TestLoadTemplateFromBytes_NoEquipment(t *testing.T) {
	data := []byte(`
id: bare_npc
name: Bare NPC
level: 1
max_hp: 10
ac: 12
perception: 4
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Empty(t, tmpl.Weapon)
	assert.Empty(t, tmpl.Armor)
}

func TestCombatStrategyUseCover(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		useCover := rapid.Bool().Draw(t, "useCover")
		tmpl := &npc.Template{
			ID:    "test",
			Name:  "Test",
			Level: 1,
			MaxHP: 10,
			AC:    10,
			Combat: npc.CombatStrategy{UseCover: useCover},
		}
		if tmpl.Combat.UseCover != useCover {
			t.Errorf("UseCover: got %v", tmpl.Combat.UseCover)
		}
	})
}

// TestTemplate_SaveRankFields_DefaultToEmpty verifies that toughness_rank,
// hustle_rank, cool_rank all default to "" when not set in YAML.
//
// Precondition: YAML has no rank fields.
// Postcondition: all three rank fields are "".
func TestTemplate_SaveRankFields_DefaultToEmpty(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, "", tmpl.ToughnessRank)
	assert.Equal(t, "", tmpl.HustleRank)
	assert.Equal(t, "", tmpl.CoolRank)
}

// TestTemplate_SaveRankFields_ParseFromYAML verifies that rank fields round-trip
// through YAML parsing.
//
// Precondition: YAML specifies toughness_rank=trained, hustle_rank=expert, cool_rank=master.
// Postcondition: parsed fields equal the specified values.
func TestTemplate_SaveRankFields_ParseFromYAML(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
toughness_rank: trained
hustle_rank: expert
cool_rank: master
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, "trained", tmpl.ToughnessRank)
	assert.Equal(t, "expert", tmpl.HustleRank)
	assert.Equal(t, "master", tmpl.CoolRank)
}

// TestTemplate_RobMultiplier_DefaultsToZero verifies that rob_multiplier defaults
// to 0.0 when not present in YAML.
//
// Precondition: YAML has no rob_multiplier field.
// Postcondition: tmpl.RobMultiplier == 0.0.
func TestTemplate_RobMultiplier_DefaultsToZero(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, 0.0, tmpl.RobMultiplier)
}

// TestTemplate_RobMultiplier_ParsesFromYAML verifies that rob_multiplier round-trips
// through YAML parsing.
//
// Precondition: YAML specifies rob_multiplier: 1.5.
// Postcondition: tmpl.RobMultiplier == 1.5.
func TestTemplate_RobMultiplier_ParsesFromYAML(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
rob_multiplier: 1.5
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, 1.5, tmpl.RobMultiplier)
}

// TestInstance_RobPercent_ZeroWhenMultiplierZero verifies that Instance.RobPercent
// is 0 when the template RobMultiplier is 0.
//
// Precondition: tmpl.RobMultiplier == 0.
// Postcondition: inst.RobPercent == 0.
func TestInstance_RobPercent_ZeroWhenMultiplierZero(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t1", Name: "T", Level: 5, MaxHP: 10, AC: 10, Perception: 0,
		RobMultiplier: 0.0,
	}
	inst := npc.NewInstance("i1", tmpl, "room1")
	assert.Equal(t, 0.0, inst.RobPercent)
	assert.Equal(t, 0, inst.Currency)
}

// TestProperty_Instance_RobPercent_InRange verifies that for any RobMultiplier > 0
// and level in [1,20], inst.RobPercent is in [5.0, 30.0].
//
// Uses rapid property-based testing (SWENG-5a).
func TestProperty_Instance_RobPercent_InRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		multiplier := rapid.Float64Range(0.1, 3.0).Draw(rt, "multiplier")

		tmpl := &npc.Template{
			ID: "prop-rob", Name: "T", Level: level, MaxHP: 10, AC: 10, Perception: 0,
			RobMultiplier: multiplier,
		}
		inst := npc.NewInstance(fmt.Sprintf("i-%d", level), tmpl, "room1")
		assert.GreaterOrEqual(rt, inst.RobPercent, 5.0,
			"RobPercent must be >= 5.0 when multiplier > 0")
		assert.LessOrEqual(rt, inst.RobPercent, 30.0,
			"RobPercent must be <= 30.0")
	})
}

// TestInstance_SaveFields_CopiedFromTemplate verifies that Instance fields
// Brutality, Quickness, Savvy, ToughnessRank, HustleRank, CoolRank are copied
// from the template at spawn.
//
// Precondition: template has non-zero ability scores and rank fields.
// Postcondition: instance fields equal template values.
func TestInstance_SaveFields_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t1", Name: "T", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
		Abilities:     npc.Abilities{Brutality: 14, Quickness: 12, Savvy: 8},
		ToughnessRank: "trained",
		HustleRank:    "expert",
		CoolRank:      "master",
	}
	inst := npc.NewInstance("i1", tmpl, "room1")
	assert.Equal(t, 14, inst.Brutality)
	assert.Equal(t, 12, inst.Quickness)
	assert.Equal(t, 8, inst.Savvy)
	assert.Equal(t, "trained", inst.ToughnessRank)
	assert.Equal(t, "expert", inst.HustleRank)
	assert.Equal(t, "master", inst.CoolRank)
}

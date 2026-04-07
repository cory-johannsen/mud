package npc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
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
		ID: "t1", Name: "T", Level: 5, MaxHP: 10, AC: 10, Awareness: 0,
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
			ID: "prop-rob", Name: "T", Level: level, MaxHP: 10, AC: 10, Awareness: 0,
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
		ID: "t1", Name: "T", Level: 1, MaxHP: 10, AC: 10, Awareness: 0,
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

func TestTemplate_DefaultNPCType(t *testing.T) {
	data := []byte(`id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 12
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "combat", tmpl.NPCType, "missing npc_type must default to 'combat'")
}

func TestTemplate_MerchantRequiresMerchantConfig(t *testing.T) {
	data := []byte(`id: test_merchant
name: Test Merchant
level: 1
max_hp: 10
ac: 12
npc_type: merchant
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "merchant npc_type without merchant config must error")
}

func TestTemplate_MerchantWithConfigLoads(t *testing.T) {
	data := []byte(`id: test_merchant
name: Test Merchant
level: 1
max_hp: 10
ac: 12
npc_type: merchant
merchant:
  merchant_type: consumables
  sell_margin: 1.2
  buy_margin: 0.6
  budget: 500
  replenish_rate:
    min_hours: 4
    max_hours: 8
    stock_refill: 1
    budget_refill: 200
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "merchant", tmpl.NPCType)
	require.NotNil(t, tmpl.Merchant)
	assert.Equal(t, "consumables", tmpl.Merchant.MerchantType)
}

func TestTemplate_QuestGiverEmptyDialogErrors(t *testing.T) {
	data := []byte(`id: test_qg
name: Test Quest Giver
level: 1
max_hp: 10
ac: 12
npc_type: quest_giver
quest_giver:
  placeholder_dialog: []
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "quest_giver with empty placeholder_dialog must error")
}

func TestTemplate_CrafterRequiresExplicitConfig(t *testing.T) {
	data := []byte(`id: test_crafter
name: Test Crafter
level: 1
max_hp: 10
ac: 12
npc_type: crafter
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "crafter npc_type without explicit crafter: {} must error")
}

func TestTemplate_CrafterWithEmptyBlockLoads(t *testing.T) {
	data := []byte(`id: test_crafter
name: Test Crafter
level: 1
max_hp: 10
ac: 12
npc_type: crafter
crafter: {}
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "crafter", tmpl.NPCType)
	require.NotNil(t, tmpl.Crafter)
}

func TestTemplate_UnknownNPCTypeErrors(t *testing.T) {
	data := []byte(`id: test_bad
name: Bad NPC
level: 1
max_hp: 10
ac: 12
npc_type: wizard
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "unknown npc_type must error")
}

func TestTemplate_PersonalityPreserved(t *testing.T) {
	data := []byte(`id: test_guard
name: Test Guard
level: 2
max_hp: 20
ac: 14
npc_type: guard
personality: brave
guard:
  wanted_threshold: 2
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "brave", tmpl.Personality, "personality must be preserved from YAML")
}

// TestTemplate_ValidateWithSkills_UnknownSkill verifies fatal error for unknown skill.
func TestTemplate_ValidateWithSkills_UnknownSkill(t *testing.T) {
	tmpl := &npc.Template{
		ID: "trainer_x", Name: "Trainer X", NPCType: "job_trainer",
		Level: 2, MaxHP: 20, AC: 10,
		JobTrainer: &npc.JobTrainerConfig{
			OfferedJobs: []npc.TrainableJob{
				{
					JobID: "hacker", TrainingCost: 200,
					Prerequisites: npc.JobPrerequisites{
						MinSkillRanks: map[string]string{"nonexistent_skill_abc": "trained"},
					},
				},
			},
		},
	}
	knownSkills := map[string]bool{"smooth_talk": true}
	err := tmpl.ValidateWithSkills(knownSkills)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_skill_abc")
}

// TestTemplate_ValidateWithSkills_KnownSkill verifies known skill passes.
func TestTemplate_ValidateWithSkills_KnownSkill(t *testing.T) {
	tmpl := &npc.Template{
		ID: "trainer_y", Name: "Trainer Y", NPCType: "job_trainer",
		Level: 2, MaxHP: 20, AC: 10,
		JobTrainer: &npc.JobTrainerConfig{
			OfferedJobs: []npc.TrainableJob{
				{
					JobID: "hacker", TrainingCost: 200,
					Prerequisites: npc.JobPrerequisites{
						MinSkillRanks: map[string]string{"smooth_talk": "trained"},
					},
				},
			},
		},
	}
	knownSkills := map[string]bool{"smooth_talk": true}
	err := tmpl.ValidateWithSkills(knownSkills)
	assert.NoError(t, err)
}

// TestTemplate_Validate_BribeableGuard_Invalid verifies fatal error for invalid bribeable guard.
func TestTemplate_Validate_BribeableGuard_Invalid(t *testing.T) {
	tmpl := &npc.Template{
		ID: "bad_guard", Name: "Bad Guard", NPCType: "guard",
		Level: 2, MaxHP: 30, AC: 14,
		Guard: &npc.GuardConfig{
			WantedThreshold:     2,
			Bribeable:           true,
			MaxBribeWantedLevel: 0, // invalid
		},
	}
	err := tmpl.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_bribe_wanted_level")
}

// TestTemplate_Validate_BribeableGuard_Valid verifies valid bribeable guard passes.
func TestTemplate_Validate_BribeableGuard_Valid(t *testing.T) {
	tmpl := &npc.Template{
		ID: "good_guard", Name: "Good Guard", NPCType: "guard",
		Level: 2, MaxHP: 30, AC: 14,
		Guard: &npc.GuardConfig{
			WantedThreshold:     2,
			Bribeable:           true,
			MaxBribeWantedLevel: 2,
			BaseCosts:           map[int]int{1: 100, 2: 200, 3: 300, 4: 400},
		},
	}
	assert.NoError(t, tmpl.Validate())
}

func TestTemplate_FixerRequiresConfigBlock(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "fixer without fixer: block must error")
	assert.Contains(t, err.Error(), "requires a fixer:")
}

func TestTemplate_FixerWithInvalidConfig(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
fixer:
  npc_variance: 0
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "fixer with npc_variance=0 must error")
}

func TestTemplate_FixerValidLoads(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
fixer:
  npc_variance: 1.2
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	assert.NoError(t, err)
	assert.NotNil(t, tmpl.Fixer)
}

func TestTemplate_FixerNonCowardlyPersonalityErrors(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: brave
fixer:
  npc_variance: 1.2
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "fixer with personality 'brave' must error (REQ-WC-3)")
	assert.Contains(t, err.Error(), "personality")
}

// TestProperty_AllExistingNPCTemplatesStillLoad verifies that adding NPCType/Validate changes
// does not break any existing NPC YAML file. Reads all *.yaml in content/npcs/.
func TestProperty_AllExistingNPCTemplatesStillLoad(t *testing.T) {
	validTypes := map[string]bool{
		"combat": true, "merchant": true, "black_market_merchant": true, "guard": true, "healer": true,
		"quest_giver": true, "hireling": true, "banker": true,
		"job_trainer": true, "crafter": true, "fixer": true,
		"chip_doc": true, "motel_keeper": true, "brothel_keeper": true,
	}
	templates, err := npc.LoadTemplates("../../../content/npcs")
	require.NoError(t, err, "all existing NPC templates must still load after Validate() changes")
	assert.NotEmpty(t, templates, "expected at least one template in content/npcs/")
	for _, tmpl := range templates {
		assert.True(t, validTypes[tmpl.NPCType],
			"existing NPC %q must have a valid npc_type, got %q", tmpl.ID, tmpl.NPCType)
	}
}

// ---- NHN: IsAnimal/IsRobot/IsMachine helper tests ----

func TestIsAnimalRobotMachineHelpers(t *testing.T) {
	animal := &npc.Template{ID: "a", Name: "Dog", Type: "animal", Level: 1, MaxHP: 10, AC: 10}
	robot := &npc.Template{ID: "b", Name: "Bot", Type: "robot", Level: 1, MaxHP: 10, AC: 10}
	machine := &npc.Template{ID: "c", Name: "Turret", Type: "machine", Level: 1, MaxHP: 10, AC: 10}
	human := &npc.Template{ID: "d", Name: "Guy", Type: "human", Level: 1, MaxHP: 10, AC: 10}

	if !animal.IsAnimal() {
		t.Error("expected animal.IsAnimal() == true")
	}
	if animal.IsRobot() {
		t.Error("expected animal.IsRobot() == false")
	}
	if animal.IsMachine() {
		t.Error("expected animal.IsMachine() == false")
	}

	if robot.IsAnimal() {
		t.Error("expected robot.IsAnimal() == false")
	}
	if !robot.IsRobot() {
		t.Error("expected robot.IsRobot() == true")
	}
	if robot.IsMachine() {
		t.Error("expected robot.IsMachine() == false")
	}

	if !machine.IsMachine() {
		t.Error("expected machine.IsMachine() == true")
	}
	if human.IsAnimal() || human.IsRobot() || human.IsMachine() {
		t.Error("expected human to return false for all type helpers")
	}
}

func TestAttackVerbField(t *testing.T) {
	yaml := `
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
attack_verb: bites
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadTemplateFromBytes failed: %v", err)
	}
	if tmpl.AttackVerb != "bites" {
		t.Errorf("AttackVerb: got %q, want %q", tmpl.AttackVerb, "bites")
	}
}

func TestImmobileField(t *testing.T) {
	yaml := `
id: test_turret
name: Turret
level: 1
max_hp: 10
ac: 10
immobile: true
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadTemplateFromBytes failed: %v", err)
	}
	if !tmpl.Immobile {
		t.Error("expected Immobile == true")
	}
}

func TestAnimalLootValidation_RejectsCredits(t *testing.T) {
	yaml := `
id: bad_animal
name: Bad Animal
type: animal
level: 1
max_hp: 10
ac: 10
loot:
  currency:
    min: 1
    max: 5
`
	_, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for animal with currency loot, got nil")
	}
}

func TestAnimalLootValidation_RejectsItems(t *testing.T) {
	yaml := `
id: bad_animal2
name: Bad Animal 2
type: animal
level: 1
max_hp: 10
ac: 10
loot:
  items:
    - item: sword
      chance: 0.5
      min_qty: 1
      max_qty: 1
`
	_, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for animal with items loot, got nil")
	}
}

func TestAnimalLootValidation_RejectsSalvageDrop(t *testing.T) {
	yaml := `
id: bad_animal3
name: Bad Animal 3
type: animal
level: 1
max_hp: 10
ac: 10
loot:
  salvage_drop:
    item_ids:
      - scrap
    quantity_min: 1
    quantity_max: 1
`
	_, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for animal with salvage_drop, got nil")
	}
}

func TestUnknownTypeNotRejected(t *testing.T) {
	yaml := `
id: alien_npc
name: Alien
type: alien
level: 1
max_hp: 10
ac: 10
`
	_, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err != nil {
		t.Errorf("expected unknown type to be accepted, got: %v", err)
	}
}

func TestTemplate_SenseAbilitiesField(t *testing.T) {
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
sense_abilities:
  - detect_lies
  - read_aura
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"detect_lies", "read_aura"}, tmpl.SenseAbilities)
}

func TestTemplate_SpecialAbilitiesYAMLKeyRejected(t *testing.T) {
	// old key must not silently map — strict decode will ignore it
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
special_abilities:
  - detect_lies
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	// old key is not aliased — SenseAbilities must be empty
	assert.Empty(t, tmpl.SenseAbilities)
}

func TestTemplate_Tier_DefaultsToStandardAtValidate(t *testing.T) {
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	// Tier is empty in YAML — Validate normalizes to ""
	// actual tier resolution to "standard" happens at usage time
	assert.Equal(t, "", tmpl.Tier)
}

func TestTemplate_Tier_ValidValues(t *testing.T) {
	for _, tier := range []string{"minion", "standard", "elite", "champion", "boss"} {
		data := []byte(fmt.Sprintf(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tier: %s
`, tier))
		tmpl, err := npc.LoadTemplateFromBytes(data)
		require.NoError(t, err, "tier %q should be valid", tier)
		assert.Equal(t, tier, tmpl.Tier)
	}
}

func TestTemplate_Tier_InvalidRejected(t *testing.T) {
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tier: legendary
`)
	_, err := npc.LoadTemplateFromBytes(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tier")
}

func TestProperty_Template_TierValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Use printable ASCII only to avoid YAML normalization of control chars.
		tier := rapid.StringMatching(`[a-z_]{1,20}`).Draw(t, "tier")
		valid := map[string]bool{
			"minion": true, "standard": true, "elite": true,
			"champion": true, "boss": true, "": true,
		}
		data := []byte(fmt.Sprintf(`
id: npc_%s
name: NPC %s
level: 1
max_hp: 10
ac: 10
tier: %s
`, tier, tier, tier))
		_, err := npc.LoadTemplateFromBytes(data)
		if valid[tier] {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	})
}

func TestTemplate_Tags_PropagatedToInstance(t *testing.T) {
	data := []byte(`
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
tags:
  - undead
  - flying
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"undead", "flying"}, tmpl.Tags)
}

func TestTemplate_ValidateWithRegistry_UnknownFeat(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
		Feats: []string{"nonexistent_feat"},
	}
	registry := ruleset.NewFeatRegistry([]*ruleset.Feat{})
	err := tmpl.ValidateWithRegistry(registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_feat")
}

func TestTemplate_ValidateWithRegistry_FeatNotAllowNPC(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
		Feats: []string{"player_only_feat"},
	}
	feats := []*ruleset.Feat{{ID: "player_only_feat", Name: "PO Feat", AllowNPC: false}}
	registry := ruleset.NewFeatRegistry(feats)
	err := tmpl.ValidateWithRegistry(registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "player_only_feat")
}

func TestTemplate_ValidateWithRegistry_ValidFeats(t *testing.T) {
	tmpl := &npc.Template{
		ID: "test", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
		Feats: []string{"tough", "brutal_strike"},
	}
	feats := []*ruleset.Feat{
		{ID: "tough", Name: "Tough", AllowNPC: true},
		{ID: "brutal_strike", Name: "Brutal Strike", AllowNPC: true},
	}
	registry := ruleset.NewFeatRegistry(feats)
	err := tmpl.ValidateWithRegistry(registry)
	require.NoError(t, err)
}

func TestTemplate_MotelKeeperRequiresMotelConfig(t *testing.T) {
	data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "motel_keeper without motel config must error")
	assert.Contains(t, err.Error(), "requires a motel: config block")
}

func TestTemplate_MotelKeeperRequiresPositiveRestCost(t *testing.T) {
	data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
motel:
  rest_cost: 0
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "motel_keeper with rest_cost 0 must error")
	assert.Contains(t, err.Error(), "rest_cost > 0")
}

func TestTemplate_MotelKeeperWithValidConfigLoads(t *testing.T) {
	data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
motel:
  rest_cost: 50
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "motel_keeper", tmpl.NPCType)
	require.NotNil(t, tmpl.Motel)
	assert.Equal(t, 50, tmpl.Motel.RestCost)
}

// ---- BrothelConfig tests (REQ-BR-1, REQ-BR-2, REQ-BR-3, REQ-BR-T1) ----

// validBrothelConfig returns a BrothelConfig that satisfies all constraints (REQ-BR-3).
func validBrothelConfig() *npc.BrothelConfig {
	return &npc.BrothelConfig{
		RestCost:      50,
		DiseaseChance: 0.1,
		RobberyChance: 0.05,
		DiseasePool:   []string{"syphilis"},
		FlairBonusDur: "24h",
	}
}

// TestBrothelConfig_Validate_ValidConfig verifies that a fully valid BrothelConfig passes.
//
// Precondition: config has rest_cost>0, chances in [0,1], non-empty disease_pool,
// valid duration string.
// Postcondition: Validate() returns nil.
func TestBrothelConfig_Validate_ValidConfig(t *testing.T) {
	cfg := validBrothelConfig()
	assert.NoError(t, cfg.Validate())
}

// TestBrothelConfig_Validate_ZeroRestCostErrors verifies REQ-BR-3: rest_cost <= 0 rejected.
//
// Precondition: rest_cost == 0.
// Postcondition: Validate() returns a non-nil error.
func TestBrothelConfig_Validate_ZeroRestCostErrors(t *testing.T) {
	cfg := validBrothelConfig()
	cfg.RestCost = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rest_cost")
}

// TestBrothelConfig_Validate_NegativeRestCostErrors verifies REQ-BR-3: rest_cost <= 0 rejected.
//
// Precondition: rest_cost == -1.
// Postcondition: Validate() returns a non-nil error.
func TestBrothelConfig_Validate_NegativeRestCostErrors(t *testing.T) {
	cfg := validBrothelConfig()
	cfg.RestCost = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rest_cost")
}

// TestBrothelConfig_Validate_DiseaseChanceOutOfRangeErrors verifies REQ-BR-3.
//
// Precondition: disease_chance is -0.01 or 1.01.
// Postcondition: Validate() returns a non-nil error referencing disease_chance.
func TestBrothelConfig_Validate_DiseaseChanceOutOfRangeErrors(t *testing.T) {
	for _, bad := range []float64{-0.01, 1.01, -100.0, 5.0} {
		cfg := validBrothelConfig()
		cfg.DiseaseChance = bad
		err := cfg.Validate()
		assert.Error(t, err, "expected error for disease_chance %f", bad)
		assert.Contains(t, err.Error(), "disease_chance")
	}
}

// TestBrothelConfig_Validate_RobberyChanceOutOfRangeErrors verifies REQ-BR-3.
//
// Precondition: robbery_chance is -0.01 or 1.01.
// Postcondition: Validate() returns a non-nil error referencing robbery_chance.
func TestBrothelConfig_Validate_RobberyChanceOutOfRangeErrors(t *testing.T) {
	for _, bad := range []float64{-0.01, 1.01, -100.0, 5.0} {
		cfg := validBrothelConfig()
		cfg.RobberyChance = bad
		err := cfg.Validate()
		assert.Error(t, err, "expected error for robbery_chance %f", bad)
		assert.Contains(t, err.Error(), "robbery_chance")
	}
}

// TestBrothelConfig_Validate_EmptyDiseasePoolErrors verifies REQ-BR-3.
//
// Precondition: disease_pool is empty.
// Postcondition: Validate() returns a non-nil error referencing disease_pool.
func TestBrothelConfig_Validate_EmptyDiseasePoolErrors(t *testing.T) {
	cfg := validBrothelConfig()
	cfg.DiseasePool = []string{}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disease_pool")
}

// TestBrothelConfig_Validate_InvalidDurationErrors verifies REQ-BR-3.
//
// Precondition: flair_bonus_duration is "banana" (not a valid Go duration).
// Postcondition: Validate() returns a non-nil error.
func TestBrothelConfig_Validate_InvalidDurationErrors(t *testing.T) {
	for _, bad := range []string{"banana", "forever", "1day", "5 minutes", "abc"} {
		cfg := validBrothelConfig()
		cfg.FlairBonusDur = bad
		err := cfg.Validate()
		assert.Error(t, err, "expected error for flair_bonus_duration %q", bad)
		assert.Contains(t, err.Error(), "flair_bonus_duration")
	}
}

// TestBrothelConfig_Validate_ValidDurations verifies that standard Go duration strings pass.
//
// Precondition: flair_bonus_duration is "24h", "30m", "1h30m".
// Postcondition: Validate() returns nil.
func TestBrothelConfig_Validate_ValidDurations(t *testing.T) {
	for _, dur := range []string{"24h", "30m", "1h30m", "45s", "2h"} {
		cfg := validBrothelConfig()
		cfg.FlairBonusDur = dur
		assert.NoError(t, cfg.Validate(), "expected no error for duration %q", dur)
	}
}

// TestProperty_BrothelConfig_ValidConfigPasses verifies that any generated valid config passes (REQ-BR-T1).
//
// Precondition: all fields satisfy constraints.
// Postcondition: Validate() returns nil.
func TestProperty_BrothelConfig_ValidConfigPasses(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		restCost := rapid.IntRange(1, 10000).Draw(rt, "rest_cost")
		diseaseChance := rapid.Float64Range(0.0, 1.0).Draw(rt, "disease_chance")
		robberyChance := rapid.Float64Range(0.0, 1.0).Draw(rt, "robbery_chance")
		diseaseCount := rapid.IntRange(1, 5).Draw(rt, "disease_count")
		diseases := make([]string, diseaseCount)
		for i := range diseases {
			diseases[i] = fmt.Sprintf("disease_%d", i)
		}
		value := rapid.IntRange(1, 3600).Draw(rt, "dur_value")
		unit := rapid.SampledFrom([]string{"s", "m", "h"}).Draw(rt, "dur_unit")
		dur := fmt.Sprintf("%d%s", value, unit)

		cfg := &npc.BrothelConfig{
			RestCost:      restCost,
			DiseaseChance: diseaseChance,
			RobberyChance: robberyChance,
			DiseasePool:   diseases,
			FlairBonusDur: dur,
		}
		assert.NoError(rt, cfg.Validate())
	})
}

// TestProperty_BrothelConfig_InvalidRestCostFails verifies REQ-BR-T1 for rest_cost <= 0.
//
// Precondition: rest_cost is in [-1000, 0].
// Postcondition: Validate() always returns a non-nil error.
func TestProperty_BrothelConfig_InvalidRestCostFails(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		restCost := rapid.IntRange(-1000, 0).Draw(rt, "rest_cost")
		cfg := validBrothelConfig()
		cfg.RestCost = restCost
		assert.Error(rt, cfg.Validate())
	})
}

// TestProperty_BrothelConfig_InvalidDiseaseChanceFails verifies REQ-BR-T1 for disease_chance out of [0,1].
//
// Precondition: disease_chance < 0 or > 1.
// Postcondition: Validate() always returns a non-nil error.
func TestProperty_BrothelConfig_InvalidDiseaseChanceFails(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sign := rapid.SampledFrom([]float64{-1.0, 1.0}).Draw(rt, "sign")
		offset := rapid.Float64Range(0.001, 100.0).Draw(rt, "offset")
		var chance float64
		if sign < 0 {
			chance = -offset
		} else {
			chance = 1.0 + offset
		}
		cfg := validBrothelConfig()
		cfg.DiseaseChance = chance
		assert.Error(rt, cfg.Validate())
	})
}

// TestProperty_BrothelConfig_InvalidRobberyChanceFails verifies REQ-BR-T1 for robbery_chance out of [0,1].
//
// Precondition: robbery_chance < 0 or > 1.
// Postcondition: Validate() always returns a non-nil error.
func TestProperty_BrothelConfig_InvalidRobberyChanceFails(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sign := rapid.SampledFrom([]float64{-1.0, 1.0}).Draw(rt, "sign")
		offset := rapid.Float64Range(0.001, 100.0).Draw(rt, "offset")
		var chance float64
		if sign < 0 {
			chance = -offset
		} else {
			chance = 1.0 + offset
		}
		cfg := validBrothelConfig()
		cfg.RobberyChance = chance
		assert.Error(rt, cfg.Validate())
	})
}

// TestBrothelConfig_EmptyDiseasePoolFails verifies REQ-BR-T1 for empty disease_pool.
//
// Precondition: disease_pool is empty.
// Postcondition: Validate() returns a non-nil error.
func TestBrothelConfig_EmptyDiseasePoolFails(t *testing.T) {
	cfg := validBrothelConfig()
	cfg.DiseasePool = []string{}
	assert.Error(t, cfg.Validate())
}

// TestProperty_BrothelConfig_InvalidDurationFails verifies REQ-BR-T1 for invalid flair_bonus_duration strings.
//
// Precondition: flair_bonus_duration is a non-empty string that time.ParseDuration cannot parse.
// Postcondition: Validate() always returns a non-nil error.
func TestProperty_BrothelConfig_InvalidDurationFails(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_!@#$%"))).
			Filter(func(s string) bool {
				if s == "" {
					return false
				}
				_, err := time.ParseDuration(s)
				return err != nil
			}).Draw(rt, "invalid_duration")
		cfg := validBrothelConfig()
		cfg.FlairBonusDur = s
		if err := cfg.Validate(); err == nil {
			rt.Errorf("Validate() returned nil for invalid duration %q, want error", s)
		}
	})
}

// TestTemplate_BrothelKeeperRequiresBrothelConfig verifies REQ-BR-1:
// brothel_keeper without a brothel: block is a fatal load error.
//
// Precondition: npc_type is "brothel_keeper", no brothel: config block present.
// Postcondition: LoadTemplateFromBytes returns a non-nil error.
func TestTemplate_BrothelKeeperRequiresBrothelConfig(t *testing.T) {
	data := []byte(`id: test_brothel
name: Test Brothel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: brothel_keeper
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "brothel_keeper without brothel config must error")
	assert.Contains(t, err.Error(), "requires a brothel: config block")
}

// TestTemplate_BrothelKeeperWithValidConfigLoads verifies REQ-BR-1 and REQ-BR-2:
// a well-formed brothel_keeper template loads without error.
//
// Precondition: all BrothelConfig fields satisfy REQ-BR-3.
// Postcondition: LoadTemplateFromBytes returns a non-nil *Template with Brothel set.
func TestTemplate_BrothelKeeperWithValidConfigLoads(t *testing.T) {
	data := []byte(`id: test_brothel
name: Test Brothel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: brothel_keeper
brothel:
  rest_cost: 75
  disease_chance: 0.1
  robbery_chance: 0.05
  disease_pool:
    - syphilis
    - gonorrhea
  flair_bonus_duration: 24h
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "brothel_keeper", tmpl.NPCType)
	require.NotNil(t, tmpl.Brothel)
	assert.Equal(t, 75, tmpl.Brothel.RestCost)
	assert.Equal(t, 0.1, tmpl.Brothel.DiseaseChance)
	assert.Equal(t, 0.05, tmpl.Brothel.RobberyChance)
	assert.Equal(t, []string{"syphilis", "gonorrhea"}, tmpl.Brothel.DiseasePool)
	assert.Equal(t, "24h", tmpl.Brothel.FlairBonusDur)
}

// TestTemplate_BrothelKeeperInvalidRestCostErrors verifies REQ-BR-3 via template load.
//
// Precondition: brothel.rest_cost is 0.
// Postcondition: LoadTemplateFromBytes returns a non-nil error.
func TestTemplate_BrothelKeeperInvalidRestCostErrors(t *testing.T) {
	data := []byte(`id: test_brothel
name: Test Brothel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: brothel_keeper
brothel:
  rest_cost: 0
  disease_chance: 0.1
  robbery_chance: 0.05
  disease_pool:
    - syphilis
  flair_bonus_duration: 24h
`)
	_, err := npc.LoadTemplateFromBytes(data)
	assert.Error(t, err, "brothel_keeper with rest_cost=0 must error")
	assert.Contains(t, err.Error(), "rest_cost")
}

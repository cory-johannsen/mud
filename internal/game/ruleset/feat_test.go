package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadFeats_ParsesAllFeats(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	if len(feats) == 0 {
		t.Fatal("expected non-empty feats list")
	}
	var found bool
	for _, f := range feats {
		if f.ID == "toughness" && f.Category == "general" && !f.Active {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'toughness' general feat")
	}
}

func TestLoadFeats_SkillFeatHasSkillField(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Category == "skill" && f.Skill == "" {
			t.Errorf("skill feat %q has empty Skill field", f.ID)
		}
	}
}

func TestLoadFeats_ActiveFeatHasActivateText(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Active && f.ActivateText == "" {
			t.Errorf("active feat %q has empty ActivateText", f.ID)
		}
	}
}

func TestFeatRegistry_LookupByID(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	f, ok := reg.Feat("toughness")
	if !ok {
		t.Fatal("expected to find toughness in registry")
	}
	if f.Name != "Toughness" {
		t.Errorf("expected Name=Toughness got %q", f.Name)
	}
}

func TestFeatRegistry_ByCategory(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	generals := reg.ByCategory("general")
	if len(generals) == 0 {
		t.Error("expected non-empty general feats")
	}
	for _, f := range generals {
		if f.Category != "general" {
			t.Errorf("ByCategory(general) returned feat with Category=%q", f.Category)
		}
	}
}

func TestFeatRegistry_BySkill(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	parkour := reg.BySkill("parkour")
	if len(parkour) == 0 {
		t.Error("expected parkour skill feats")
	}
}

func TestFeat_YAML_WithReactionBlock(t *testing.T) {
	yml := `
feats:
- id: reactive_strike
  name: Reactive Strike
  category: combat
  reaction:
    triggers:
      - on_enemy_move_adjacent
    requirement: wielding_melee_weapon
    effect:
      type: strike
      target: trigger_source
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yml))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	f := feats[0]
	assert.Equal(t, "reactive_strike", f.ID)
	require.NotNil(t, f.Reaction)
	require.Len(t, f.Reaction.Triggers, 1)
	assert.Equal(t, reaction.TriggerOnEnemyMoveAdjacent, f.Reaction.Triggers[0])
	assert.Equal(t, "wielding_melee_weapon", f.Reaction.Requirement)
	assert.Equal(t, reaction.ReactionEffectStrike, f.Reaction.Effect.Type)
	assert.Equal(t, "trigger_source", f.Reaction.Effect.Target)
}

func TestFeat_YAML_WithoutReaction_NilReactionField(t *testing.T) {
	yml := `
feats:
- id: sucker_punch
  name: Sucker Punch
  category: combat
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yml))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	assert.Nil(t, feats[0].Reaction)
}

func TestFeat_AllowNPC_DefaultFalse(t *testing.T) {
	data := []byte(`
feats:
  - id: test_feat
    name: Test Feat
    category: general
    active: false
    description: "A test feat."
`)
	feats, err := ruleset.LoadFeatsFromBytes(data)
	require.NoError(t, err)
	require.Len(t, feats, 1)
	assert.False(t, feats[0].AllowNPC)
}

func TestFeat_AllowNPC_TrueWhenSet(t *testing.T) {
	data := []byte(`
feats:
  - id: brutal_strike
    name: Brutal Strike
    category: general
    active: false
    allow_npc: true
    description: "+2 damage."
`)
	feats, err := ruleset.LoadFeatsFromBytes(data)
	require.NoError(t, err)
	assert.True(t, feats[0].AllowNPC)
}

func TestLoadFeats_GeneralGapFeatsPresent(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	require.NoError(t, err)
	byID := make(map[string]bool)
	for _, f := range feats {
		byID[f.ID] = true
	}
	required := []string{
		"adopted_culture", "loyal_companion", "vehicle_operator", "parallel_lives",
		"speedrun_strats", "cornered_beast", "cultural_roots", "contingency_stash",
		"street_improvisation", "field_repair", "crew_boss", "hardened_constitution",
		"steel_your_resolve", "push_the_pace", "methodical_sweep", "sharp_follower",
		"skitter", "street_eye", "neural_crossover", "district_adept", "ghost_step",
		"efficient_sweep", "contingency_consumable", "vital_sense", "death_proof",
		"chemical_palate", "zone_sense", "combat_vigor", "tech_overload_capacity",
		"scout_mastery", "local_everywhere", "blood_will", "crew_leader", "true_perception",
	}
	for _, id := range required {
		assert.True(t, byID[id], "missing general feat: %s", id)
	}
}

func TestLoadFeats_SkillGapFeats_ParkourMusclGhostingGrift(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	require.NoError(t, err)
	byID := make(map[string]bool)
	for _, f := range feats {
		byID[f.ID] = true
	}
	required := []string{
		"acrobatic_performer", "fast_crawl", "power_jump", "quick_vault", "roll_landing",
		"kip_up", "aerial_mastery", "wall_jump", "water_run", "impossible_leap",
		"anchor_climber", "speed_climb", "speed_swim",
		"armored_ghost", "silent_crew", "sense_block", "speed_ghost", "terrain_vanish", "ghost_legend",
		"concealed_activation", "careful_disarm", "shadow_mark", "rolling_lift", "speed_unlock", "master_thief",
	}
	for _, id := range required {
		assert.True(t, byID[id], "missing skill feat: %s", id)
	}
}

func TestFeat_TargetTags_Loaded(t *testing.T) {
	data := []byte(`
feats:
  - id: hunter_feat
    name: Hunter Feat
    category: general
    active: false
    target_tags:
      - undead
      - mutant
    description: "Bonus vs tagged targets."
`)
	feats, err := ruleset.LoadFeatsFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"undead", "mutant"}, feats[0].TargetTags)
}

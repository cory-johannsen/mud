package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
    trigger: on_enemy_move_adjacent
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
	assert.Equal(t, reaction.TriggerOnEnemyMoveAdjacent, f.Reaction.Trigger)
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

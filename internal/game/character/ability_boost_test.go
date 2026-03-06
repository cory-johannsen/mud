package character_test

import (
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestApplyAbilityBoosts_EachBoostAddsTwo(t *testing.T) {
	base := character.AbilityScores{
		Brutality: 10, Grit: 10, Quickness: 10,
		Reasoning: 10, Savvy: 10, Flair: 10,
	}
	archetypeBoosts := &ruleset.AbilityBoostGrant{Fixed: []string{"brutality", "grit"}, Free: 2}
	archetypeChosen := []string{"quickness", "reasoning"}
	regionBoosts := &ruleset.AbilityBoostGrant{Fixed: []string{"savvy", "flair"}, Free: 1}
	regionChosen := []string{"brutality"}

	got := character.ApplyAbilityBoosts(base, archetypeBoosts, archetypeChosen, regionBoosts, regionChosen)

	assert.Equal(t, 14, got.Brutality)  // +2 archetype fixed +2 region chosen
	assert.Equal(t, 12, got.Grit)       // +2 archetype fixed
	assert.Equal(t, 12, got.Quickness)  // +2 archetype free
	assert.Equal(t, 12, got.Reasoning)  // +2 archetype free
	assert.Equal(t, 12, got.Savvy)      // +2 region fixed
	assert.Equal(t, 12, got.Flair)      // +2 region fixed
}

func TestApplyAbilityBoosts_NilGrantsAreNoOp(t *testing.T) {
	base := character.AbilityScores{Brutality: 14, Grit: 12}
	got := character.ApplyAbilityBoosts(base, nil, nil, nil, nil)
	assert.Equal(t, base, got)
}

func TestApplyAbilityBoosts_EmptyChosen(t *testing.T) {
	base := character.AbilityScores{
		Brutality: 10, Grit: 10, Quickness: 10,
		Reasoning: 10, Savvy: 10, Flair: 10,
	}
	grant := &ruleset.AbilityBoostGrant{Fixed: []string{"brutality"}, Free: 1}
	got := character.ApplyAbilityBoosts(base, grant, nil, nil, nil)
	assert.Equal(t, 12, got.Brutality)
	assert.Equal(t, 10, got.Grit)
}

func TestProperty_ApplyAbilityBoosts_EachBoostExactlyTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		abilities := []string{"brutality", "grit", "quickness", "reasoning", "savvy", "flair"}
		n := rapid.IntRange(0, 4).Draw(rt, "n")
		chosen := abilities[:n]

		base := character.AbilityScores{
			Brutality: 10, Grit: 10, Quickness: 10,
			Reasoning: 10, Savvy: 10, Flair: 10,
		}
		grant := &ruleset.AbilityBoostGrant{Fixed: chosen, Free: 0}
		got := character.ApplyAbilityBoosts(base, grant, nil, nil, nil)

		for _, ab := range chosen {
			switch ab {
			case "brutality":
				assert.Equal(rt, 12, got.Brutality)
			case "grit":
				assert.Equal(rt, 12, got.Grit)
			case "quickness":
				assert.Equal(rt, 12, got.Quickness)
			case "reasoning":
				assert.Equal(rt, 12, got.Reasoning)
			case "savvy":
				assert.Equal(rt, 12, got.Savvy)
			case "flair":
				assert.Equal(rt, 12, got.Flair)
			}
		}
	})
}

func TestAbilityBoostPool_ExcludesFixed(t *testing.T) {
	pool := character.AbilityBoostPool([]string{"brutality", "grit"}, nil)
	assert.NotContains(t, pool, "brutality")
	assert.NotContains(t, pool, "grit")
	assert.Len(t, pool, 4)
}

func TestAbilityBoostPool_ExcludesAlreadyChosen(t *testing.T) {
	pool := character.AbilityBoostPool([]string{"brutality"}, []string{"quickness"})
	assert.NotContains(t, pool, "brutality")
	assert.NotContains(t, pool, "quickness")
	assert.Len(t, pool, 4)
}

func TestAbilityBoostPool_EmptyInputsReturnsAll(t *testing.T) {
	pool := character.AbilityBoostPool(nil, nil)
	assert.Len(t, pool, 6)
}

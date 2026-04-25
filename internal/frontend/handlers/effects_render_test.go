package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestRenderEffectsBlock_Empty verifies the empty-effect-set rendering.
// Precondition: a fresh EffectSet with no applied effects.
// Postcondition: the output contains the "No active effects" sentinel.
func TestRenderEffectsBlock_Empty(t *testing.T) {
	es := effect.NewEffectSet()
	out := RenderEffectsBlock(es, nil, 80)
	assert.Contains(t, out, "No active effects")
}

// TestRenderEffectsBlock_NilSet verifies rendering of a nil EffectSet.
// Postcondition: safe, returns the empty-set message.
func TestRenderEffectsBlock_NilSet(t *testing.T) {
	out := RenderEffectsBlock(nil, nil, 80)
	assert.Contains(t, out, "No active effects")
}

// TestRenderEffectsBlock_SingleActive verifies rendering of a single
// active effect with a typed bonus and a caster display name.
func TestRenderEffectsBlock_SingleActive(t *testing.T) {
	es := effect.NewEffectSet()
	es.Apply(effect.Effect{
		EffectID:   "heroism",
		SourceID:   "condition:heroism",
		CasterUID:  "kira",
		Annotation: "Heroism",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
		},
		DurKind: effect.DurationUntilRemove,
	})
	casterNames := map[string]string{"kira": "Kira"}
	out := RenderEffectsBlock(es, casterNames, 80)
	assert.Contains(t, out, "Heroism")
	assert.Contains(t, out, "active")
	assert.Contains(t, out, "attack +1 status")
	assert.Contains(t, out, "from Kira")
}

// TestRenderEffectsBlock_Suppressed verifies that a suppressed (overridden)
// contribution is marked as overridden in the output.
func TestRenderEffectsBlock_Suppressed(t *testing.T) {
	es := effect.NewEffectSet()
	es.Apply(effect.Effect{
		EffectID:   "heroism",
		SourceID:   "condition:heroism",
		CasterUID:  "",
		Annotation: "Heroism",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus},
		},
		DurKind: effect.DurationUntilRemove,
	})
	es.Apply(effect.Effect{
		EffectID:   "inspire",
		SourceID:   "condition:inspire_courage",
		CasterUID:  "",
		Annotation: "Inspire Courage",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
		},
		DurKind: effect.DurationUntilRemove,
	})
	out := RenderEffectsBlock(es, nil, 80)
	assert.Contains(t, out, "overridden")
	assert.Contains(t, out, "condition:heroism")
}

// TestRenderEffectsBlock_SourceLabelItem verifies that an item-prefixed
// SourceID renders as "item" when no caster name is provided.
func TestRenderEffectsBlock_SourceLabelItem(t *testing.T) {
	es := effect.NewEffectSet()
	es.Apply(effect.Effect{
		EffectID:   "sword_magic",
		SourceID:   "item:magic_sword",
		Annotation: "Magic Sword",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeItem},
		},
		DurKind: effect.DurationPermanent,
	})
	out := RenderEffectsBlock(es, nil, 80)
	assert.Contains(t, out, "item")
	assert.Contains(t, out, "Magic Sword")
}

// TestRenderEffectsBlock_NoStatBonuses verifies rendering of an effect
// that declares no stat bonuses yields the [no stat bonuses] annotation.
func TestRenderEffectsBlock_NoStatBonuses(t *testing.T) {
	es := effect.NewEffectSet()
	es.Apply(effect.Effect{
		EffectID:   "flavor_only",
		SourceID:   "condition:inspired",
		Annotation: "Inspired",
		Bonuses:    nil,
		DurKind:    effect.DurationUntilRemove,
	})
	out := RenderEffectsBlock(es, nil, 80)
	assert.Contains(t, out, "Inspired")
	assert.Contains(t, out, "no stat bonuses")
}

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

// buildDef creates a ConditionDef with only flat fields (legacy path).
func buildDef(id string, opts ...func(*condition.ConditionDef)) *condition.ConditionDef {
	d := &condition.ConditionDef{ID: id, Name: id, MaxStacks: 0, DurationType: "permanent"}
	for _, o := range opts {
		o(d)
	}
	return d
}

func TestModifiersCompat_AttackBonus_Positive(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("inspired", func(d *condition.ConditionDef) { d.AttackBonus = 2 })
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, 2, condition.AttackBonus(s))
}

func TestModifiersCompat_AttackBonus_Negative(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("frightened", func(d *condition.ConditionDef) { d.AttackPenalty = 1 })
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, -1, condition.AttackBonus(s))
}

func TestModifiersCompat_ACBonus_Mixed(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("prone", func(d *condition.ConditionDef) { d.ACPenalty = 2 })
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestModifiersCompat_DamageBonus(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("rage", func(d *condition.ConditionDef) { d.DamageBonus = 3 })
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, 3, condition.DamageBonus(s))
}

func TestModifiersCompat_SkillPenalty_FlatAndPerSkill(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("sickened", func(d *condition.ConditionDef) {
		d.SkillPenalty = 1
		d.SkillPenalties = map[string]int{"flair": 2}
	})
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, 1, condition.SkillPenalty(s))
}

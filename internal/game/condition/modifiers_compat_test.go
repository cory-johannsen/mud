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

// TestModifiersCompat_MultiCondition_AttackAggregate verifies that bonuses from two
// distinct flat-field conditions are correctly aggregated through AttackBonus.
func TestModifiersCompat_MultiCondition_AttackAggregate(t *testing.T) {
	s := condition.NewActiveSet()
	frightened := buildDef("frightened", func(d *condition.ConditionDef) { d.AttackPenalty = 1 })
	prone := buildDef("prone", func(d *condition.ConditionDef) { d.AttackPenalty = 2 })
	_ = s.Apply("uid", frightened, 1, -1)
	_ = s.Apply("uid", prone, 1, -1)
	assert.Equal(t, -3, condition.AttackBonus(s))
}

// TestModifiersCompat_MultiBonus_AttackBonusAndPenaltyNet verifies that an attack bonus
// from one condition and an attack penalty from another net correctly through AttackBonus.
func TestModifiersCompat_MultiBonus_AttackBonusAndPenaltyNet(t *testing.T) {
	s := condition.NewActiveSet()
	inspired := buildDef("inspired", func(d *condition.ConditionDef) { d.AttackBonus = 2 })
	frightened := buildDef("frightened", func(d *condition.ConditionDef) { d.AttackPenalty = 1 })
	_ = s.Apply("uid", inspired, 1, -1)
	_ = s.Apply("uid", frightened, 1, -1)
	assert.Equal(t, 1, condition.AttackBonus(s))
}

// TestModifiersCompat_Stacks_ScaleFlatFieldPenalty verifies that applying a stackable
// condition twice scales its synthesised bonuses to stacks=2.
func TestModifiersCompat_Stacks_ScaleFlatFieldPenalty(t *testing.T) {
	s := condition.NewActiveSet()
	def := buildDef("frightened", func(d *condition.ConditionDef) {
		d.AttackPenalty = 1
		d.MaxStacks = 4 // must be > 0 for stacking to be enabled
	})
	// First apply: stacks=1, penalty=-1
	_ = s.Apply("uid", def, 1, -1)
	// Second apply: stacks 1->2, scaled penalty=-2
	_ = s.Apply("uid", def, 1, -1)
	assert.Equal(t, -2, condition.AttackBonus(s))
}

package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// stubRoller is a test double for Roller that returns pre-set values.
type stubRoller struct {
	rollResult      int
	rollD20Result   int
	rollFloatResult float64
}

func (s *stubRoller) Roll(_ string) int       { return s.rollResult }
func (s *stubRoller) RollD20() int            { return s.rollD20Result }
func (s *stubRoller) RollFloat() float64      { return s.rollFloatResult }

func makeInst(durability, maxDurability int, rarity string) *inventory.ItemInstance {
	return &inventory.ItemInstance{
		InstanceID:    "test-inst",
		ItemDefID:     "test-item",
		Quantity:      1,
		Durability:    durability,
		MaxDurability: maxDurability,
		Rarity:        rarity,
		Modifier:      "",
	}
}

// ── DeductDurability ──────────────────────────────────────────────────────────

func TestDeductDurability_ReducesByOne(t *testing.T) {
	inst := makeInst(10, 40, "street")
	rng := &stubRoller{rollFloatResult: 0.99} // 0.99 > 0.30 → no destruction
	result := inventory.DeductDurability(inst, rng)
	assert.Equal(t, 9, result.NewDurability)
	assert.Equal(t, 9, inst.Durability)
	assert.False(t, result.BecameBroken)
	assert.False(t, result.Destroyed)
}

func TestDeductDurability_NoOpWhenAlreadyZero(t *testing.T) {
	inst := makeInst(0, 40, "street")
	rng := &stubRoller{rollFloatResult: 0.01}
	result := inventory.DeductDurability(inst, rng)
	assert.Equal(t, 0, result.NewDurability)
	assert.Equal(t, 0, inst.Durability)
	assert.False(t, result.BecameBroken)
	assert.False(t, result.Destroyed)
}

func TestDeductDurability_ReachesZero_DestructionRollFails(t *testing.T) {
	// street: DestructionChance = 0.30; roll 0.99 > 0.30 → survives
	inst := makeInst(1, 40, "street")
	rng := &stubRoller{rollFloatResult: 0.99}
	result := inventory.DeductDurability(inst, rng)
	assert.Equal(t, 0, result.NewDurability)
	assert.True(t, result.BecameBroken)
	assert.False(t, result.Destroyed)
}

func TestDeductDurability_ReachesZero_DestructionRollSucceeds(t *testing.T) {
	// street: DestructionChance = 0.30; roll 0.10 < 0.30 → destroyed
	inst := makeInst(1, 40, "street")
	rng := &stubRoller{rollFloatResult: 0.10}
	result := inventory.DeductDurability(inst, rng)
	assert.Equal(t, 0, result.NewDurability)
	assert.True(t, result.BecameBroken)
	assert.True(t, result.Destroyed)
}

func TestDeductDurability_GhostLowDestructionChance(t *testing.T) {
	// ghost: DestructionChance = 0.01; roll 0.005 < 0.01 → destroyed
	inst := makeInst(1, 100, "ghost")
	rng := &stubRoller{rollFloatResult: 0.005}
	result := inventory.DeductDurability(inst, rng)
	assert.True(t, result.Destroyed)
}

func TestDeductDurability_GhostSurvivesHighRoll(t *testing.T) {
	// ghost: DestructionChance = 0.01; roll 0.99 > 0.01 → not destroyed
	inst := makeInst(1, 100, "ghost")
	rng := &stubRoller{rollFloatResult: 0.99}
	result := inventory.DeductDurability(inst, rng)
	assert.False(t, result.Destroyed)
	assert.True(t, result.BecameBroken)
}

// ── RepairField ───────────────────────────────────────────────────────────────

func TestRepairField_Restores1d6Durability(t *testing.T) {
	inst := makeInst(5, 40, "street")
	rng := &stubRoller{rollResult: 4} // "1d6" → 4
	restored := inventory.RepairField(inst, rng)
	assert.Equal(t, 4, restored)
	assert.Equal(t, 9, inst.Durability)
}

func TestRepairField_CapsAtMaxDurability(t *testing.T) {
	inst := makeInst(38, 40, "street")
	rng := &stubRoller{rollResult: 6}
	restored := inventory.RepairField(inst, rng)
	assert.Equal(t, 2, restored) // only 2 space left
	assert.Equal(t, 40, inst.Durability)
}

func TestRepairField_BrokenItem_CanBeRepaired(t *testing.T) {
	inst := makeInst(0, 40, "street")
	rng := &stubRoller{rollResult: 3}
	restored := inventory.RepairField(inst, rng)
	assert.Equal(t, 3, restored)
	assert.Equal(t, 3, inst.Durability)
}

// ── RepairFull ────────────────────────────────────────────────────────────────

func TestRepairFull_RestoresToMax(t *testing.T) {
	inst := makeInst(5, 60, "mil_spec")
	inventory.RepairFull(inst)
	assert.Equal(t, 60, inst.Durability)
}

func TestRepairFull_AlreadyFull_NoChange(t *testing.T) {
	inst := makeInst(60, 60, "mil_spec")
	inventory.RepairFull(inst)
	assert.Equal(t, 60, inst.Durability)
}

// ── InitDurability ────────────────────────────────────────────────────────────

func TestInitDurability_SentinelMinus1_InitializesToMax(t *testing.T) {
	inst := makeInst(-1, -1, "")
	inventory.InitDurability(inst, "street")
	assert.Equal(t, 40, inst.MaxDurability)
	assert.Equal(t, 40, inst.Durability)
}

func TestInitDurability_NonSentinel_NoChange(t *testing.T) {
	inst := makeInst(15, 40, "street")
	inventory.InitDurability(inst, "ghost")
	// Should not change since Durability != -1
	assert.Equal(t, 15, inst.Durability)
	assert.Equal(t, 40, inst.MaxDurability)
}

func TestInitDurability_UnknownRarity_UsesZero(t *testing.T) {
	inst := makeInst(-1, -1, "")
	inventory.InitDurability(inst, "nonexistent")
	// Unknown rarity → MaxDurability stays 0, Durability set to 0
	assert.Equal(t, 0, inst.MaxDurability)
	assert.Equal(t, 0, inst.Durability)
}

// ── Property tests ────────────────────────────────────────────────────────────

func TestProperty_DeductDurability_NeverBelowZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dur := rapid.IntRange(0, 100).Draw(rt, "dur")
		inst := makeInst(dur, 100, "street")
		rng := &stubRoller{rollFloatResult: 0.99}
		result := inventory.DeductDurability(inst, rng)
		assert.GreaterOrEqual(rt, result.NewDurability, 0)
		assert.GreaterOrEqual(rt, inst.Durability, 0)
	})
}

func TestProperty_RepairField_NeverExceedsMax(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		max := rapid.IntRange(1, 100).Draw(rt, "max")
		cur := rapid.IntRange(0, max).Draw(rt, "cur")
		roll := rapid.IntRange(1, 6).Draw(rt, "roll")
		inst := makeInst(cur, max, "street")
		rng := &stubRoller{rollResult: roll}
		inventory.RepairField(inst, rng)
		assert.LessOrEqual(rt, inst.Durability, inst.MaxDurability)
	})
}

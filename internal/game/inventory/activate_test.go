package inventory_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// fakeSession implements ActivateSession for testing.
type fakeSession struct {
	team      string
	instances []*inventory.ItemInstance
}

func (f *fakeSession) GetTeam() string                                          { return f.team }
func (f *fakeSession) GetStatModifier(stat string) int                          { return 0 }
func (f *fakeSession) ApplyHeal(amount int)                                     {}
func (f *fakeSession) ApplyCondition(id string, dur time.Duration)              {}
func (f *fakeSession) RemoveCondition(id string)                                {}
func (f *fakeSession) ApplyDisease(id string, sev int)                          {}
func (f *fakeSession) ApplyToxin(id string, sev int)                            {}
func (f *fakeSession) EquippedInstances() []*inventory.ItemInstance             { return f.instances }

func mustRegisterItem(t *testing.T, reg *inventory.Registry, def *inventory.ItemDef) {
	t.Helper()
	require.NoError(t, reg.RegisterItem(def))
}

func makeActivatableItem(t *testing.T) (*inventory.Registry, *inventory.ItemDef, *inventory.ItemInstance) {
	t.Helper()
	def := &inventory.ItemDef{
		ID: "stim_rod", Name: "Stim Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 3,
		ActivationEffect: &inventory.ConsumableEffect{Heal: "2d6"},
	}
	reg := inventory.NewRegistry()
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{
		InstanceID: "inst-1", ItemDefID: "stim_rod",
		ChargesRemaining: -1,
	}
	return reg, def, inst
}

func TestHandleActivate_SentinelInitialization(t *testing.T) {
	reg, _, inst := makeActivatableItem(t)
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
	assert.Empty(t, errMsg)
	assert.Equal(t, 2, inst.ChargesRemaining) // initialized to 3, decremented to 2
	assert.Equal(t, 1, result.AP)
	assert.NotNil(t, result.ActivationEffect)
}

func TestHandleActivate_DecrementCharge(t *testing.T) {
	reg, _, inst := makeActivatableItem(t)
	inst.ChargesRemaining = 2
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	_, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
	assert.Empty(t, errMsg)
	assert.Equal(t, 1, inst.ChargesRemaining)
	assert.False(t, inst.Expended)
}

func TestHandleActivate_ExpendOnDeplete(t *testing.T) {
	reg, def, inst := makeActivatableItem(t)
	def.OnDeplete = "expend"
	inst.ChargesRemaining = 1
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
	assert.Empty(t, errMsg)
	assert.Equal(t, 0, inst.ChargesRemaining)
	assert.True(t, inst.Expended)
	assert.False(t, result.Destroyed)
}

func TestHandleActivate_DestroyOnDeplete(t *testing.T) {
	reg, def, inst := makeActivatableItem(t)
	def.OnDeplete = "destroy"
	inst.ChargesRemaining = 1
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
	assert.Empty(t, errMsg)
	assert.True(t, result.Destroyed)
}

func TestHandleActivate_ExpendedItemBlocked(t *testing.T) {
	reg, _, inst := makeActivatableItem(t)
	inst.ChargesRemaining = 0
	inst.Expended = true
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	_, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
	assert.NotEmpty(t, errMsg)
}

func TestHandleActivate_InCombatInsufficientAP(t *testing.T) {
	reg, def, inst := makeActivatableItem(t)
	def.ActivationCost = 2
	inst.ChargesRemaining = 3
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	_, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", true, 1) // only 1 AP, needs 2
	assert.NotEmpty(t, errMsg)
	assert.Equal(t, 3, inst.ChargesRemaining) // unchanged
}

// REQ-ACT-5: out-of-combat, AP cost is informational only — activation MUST succeed regardless of AP value.
func TestHandleActivate_OutOfCombat_APNotRequired(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "heavy_ring", Name: "Heavy Ring", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 3, Charges: 2,
		ActivationEffect: &inventory.ConsumableEffect{Heal: "1d4"},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i5", ItemDefID: "heavy_ring", ChargesRemaining: 2}
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	// currentAP=0, inCombat=false: must succeed even though cost=3 > ap=0.
	_, errMsg := inventory.HandleActivate(sess, reg, "heavy_ring", false, 0)
	assert.Empty(t, errMsg)
	assert.Equal(t, 1, inst.ChargesRemaining)
}

func TestHandleActivate_NotActivatable(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{ID: "junk", Name: "Junk", Kind: inventory.KindJunk, MaxStack: 1}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i1", ItemDefID: "junk"}
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	_, errMsg := inventory.HandleActivate(sess, reg, "junk", false, 0)
	assert.NotEmpty(t, errMsg)
}

func TestHandleActivate_ScriptRouting(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "arcane_rod", Name: "Arcane Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 2,
		ActivationScript: "arcane_rod_activate",
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i2", ItemDefID: "arcane_rod", ChargesRemaining: 2}
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	result, errMsg := inventory.HandleActivate(sess, reg, "arcane_rod", false, 0)
	assert.Empty(t, errMsg)
	assert.Equal(t, "arcane_rod_activate", result.Script)
	assert.Nil(t, result.ActivationEffect)
}

func TestHandleActivate_RechargeItemExpendNotDestroy(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "solar_ring", Name: "Solar Ring", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 1, OnDeplete: "destroy", // OnDeplete ignored when Recharge set
		Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i3", ItemDefID: "solar_ring", ChargesRemaining: 1}
	sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
	result, errMsg := inventory.HandleActivate(sess, reg, "solar_ring", false, 0)
	assert.Empty(t, errMsg)
	assert.False(t, result.Destroyed) // expend semantics, not destroy
	assert.True(t, inst.Expended)
}

func TestHandleActivate_Property_ChargesNeverGoNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		initial := rapid.IntRange(1, 10).Draw(rt, "initial")
		reg := inventory.NewRegistry()
		def := &inventory.ItemDef{
			ID: "item", Name: "Item", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 1, Charges: initial,
		}
		require.NoError(t, reg.RegisterItem(def))
		inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "item", ChargesRemaining: initial}
		sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
		for i := 0; i < initial+2; i++ {
			inventory.HandleActivate(sess, reg, "item", false, 0)
		}
		assert.GreaterOrEqual(rt, inst.ChargesRemaining, 0)
	})
}

func TestTickRecharge_RestoresCharges(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 3,
		Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 1}
	modified := inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "dawn")
	assert.Len(t, modified, 1)
	assert.Equal(t, 2, inst.ChargesRemaining)
}

func TestTickRecharge_CapsAtMaxCharges(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 3,
		Recharge: []inventory.RechargeEntry{{Trigger: "rest", Amount: 5}},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 2}
	inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "rest")
	assert.Equal(t, 3, inst.ChargesRemaining) // capped at max
}

func TestTickRecharge_ClearsExpended(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 3,
		Recharge: []inventory.RechargeEntry{{Trigger: "midnight", Amount: 1}},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 0, Expended: true}
	inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "midnight")
	assert.Equal(t, 1, inst.ChargesRemaining)
	assert.False(t, inst.Expended)
}

func TestTickRecharge_WrongTriggerNoOp(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
		ActivationCost: 1, Charges: 3,
		Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
	}
	mustRegisterItem(t, reg, def)
	inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 1}
	modified := inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "midnight")
	assert.Empty(t, modified)
	assert.Equal(t, 1, inst.ChargesRemaining)
}

func TestTickRecharge_Property_NeverExceedsMaxCharges(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxCharges := rapid.IntRange(1, 10).Draw(rt, "max")
		current := rapid.IntRange(0, maxCharges).Draw(rt, "current")
		amount := rapid.IntRange(1, 5).Draw(rt, "amount")
		reg := inventory.NewRegistry()
		def := &inventory.ItemDef{
			ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 1, Charges: maxCharges,
			Recharge: []inventory.RechargeEntry{{Trigger: "daily", Amount: amount}},
		}
		require.NoError(t, reg.RegisterItem(def))
		inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: current}
		inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "daily")
		assert.LessOrEqual(rt, inst.ChargesRemaining, maxCharges)
	})
}

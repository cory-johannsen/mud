package trap_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestTrapTemplate_NewFields_ZeroValue(t *testing.T) {
	tmpl := &trap.TrapTemplate{}
	assert.Equal(t, 0, tmpl.TriggerRangeFt)
	assert.Equal(t, 0, tmpl.BlastRadiusFt)
}

func TestEffectiveTriggerRange_ZeroIsDefault(t *testing.T) {
	tmpl := &trap.TrapTemplate{TriggerRangeFt: 0}
	assert.Equal(t, 5, trap.EffectiveTriggerRange(tmpl))
}

func TestEffectiveTriggerRange_NonZeroPassthrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 100).Draw(t, "range")
		tmpl := &trap.TrapTemplate{TriggerRangeFt: n}
		assert.Equal(t, n, trap.EffectiveTriggerRange(tmpl))
	})
}

func TestTrapKindConsumable_Constant(t *testing.T) {
	assert.Equal(t, "consumable", trap.TrapKindConsumable)
}

func TestAddConsumableTrap_ArmedWithPosition(t *testing.T) {
	mgr := trap.NewTrapManager()
	tmpl := &trap.TrapTemplate{ID: "mine", Name: "Mine"}
	err := mgr.AddConsumableTrap("zone/room/consumable/1", tmpl, 15)
	require.NoError(t, err)
	inst, ok := mgr.GetTrap("zone/room/consumable/1")
	require.True(t, ok)
	assert.True(t, inst.Armed)
	assert.True(t, inst.IsConsumable)
	assert.Equal(t, 15, inst.DeployPosition)
}

func TestAddConsumableTrap_DuplicateReturnsError(t *testing.T) {
	mgr := trap.NewTrapManager()
	tmpl := &trap.TrapTemplate{ID: "mine"}
	require.NoError(t, mgr.AddConsumableTrap("id1", tmpl, 0))
	err := mgr.AddConsumableTrap("id1", tmpl, 0)
	assert.Error(t, err)
}

func TestProperty_AddConsumableTrap_AlwaysArmedIsConsumable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pos := rapid.Int().Draw(t, "pos")
		mgr := trap.NewTrapManager()
		tmpl := &trap.TrapTemplate{ID: "t"}
		id := "zone/room/consumable/" + rapid.StringN(1, 20, -1).Draw(t, "id")
		err := mgr.AddConsumableTrap(id, tmpl, pos)
		require.NoError(t, err)
		inst, ok := mgr.GetTrap(id)
		require.True(t, ok)
		assert.True(t, inst.Armed)
		assert.True(t, inst.IsConsumable)
		assert.Equal(t, pos, inst.DeployPosition)
	})
}

// REQ-CTR-5: A deployed trap must be position-anchored — DeployPosition must not change after creation.
func TestAddConsumableTrap_PositionAnchored(t *testing.T) {
	mgr := trap.NewTrapManager()
	tmpl := &trap.TrapTemplate{ID: "mine"}
	require.NoError(t, mgr.AddConsumableTrap("zone/room/consumable/anc-1", tmpl, 20))
	inst, ok := mgr.GetTrap("zone/room/consumable/anc-1")
	require.True(t, ok)
	assert.Equal(t, 20, inst.DeployPosition, "DeployPosition must be the value set at creation")
	// Callers must not mutate DeployPosition — the struct is returned by value, so mutating
	// the returned copy must not affect the stored state.
	inst.DeployPosition = 999
	inst2, ok2 := mgr.GetTrap("zone/room/consumable/anc-1")
	require.True(t, ok2)
	assert.Equal(t, 20, inst2.DeployPosition, "stored DeployPosition must be immutable after creation")
}

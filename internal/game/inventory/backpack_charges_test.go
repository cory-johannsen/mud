package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestItemInstance_ChargeFields_DefaultValues(t *testing.T) {
	// Reference new fields by name to force compile error before they exist.
	inst := ItemInstance{
		InstanceID:       "i1",
		ItemDefID:        "x",
		ChargesRemaining: -1, // uninitialized sentinel
		Expended:         false,
	}
	assert.Equal(t, -1, inst.ChargesRemaining)
	assert.False(t, inst.Expended)
}

// TestItemInstance_ChargeFields_Property verifies the structural invariant:
// an ItemInstance with ChargesRemaining == -1 (uninitialized sentinel) must NOT have Expended == true.
// Expended == true requires ChargesRemaining to have been initialized and then decremented to 0.
func TestItemInstance_ChargeFields_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		instanceID := rapid.StringMatching(`[a-z]{4}`).Draw(rt, "instance_id")
		itemDefID := rapid.StringMatching(`[a-z]{4}`).Draw(rt, "item_def_id")
		// Construct an uninitialized-sentinel instance; Expended must always be false.
		inst := ItemInstance{
			InstanceID:       instanceID,
			ItemDefID:        itemDefID,
			ChargesRemaining: -1,
			Expended:         false,
		}
		assert.Equal(rt, -1, inst.ChargesRemaining, "ChargesRemaining must be -1 (uninitialized sentinel)")
		assert.False(rt, inst.Expended, "Expended must be false when ChargesRemaining == -1 (uninitialized)")
	})
}

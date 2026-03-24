package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

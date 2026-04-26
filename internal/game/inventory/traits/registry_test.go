package traits_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory/traits"
)

func TestDefaultRegistry_NotNil(t *testing.T) {
	r := traits.DefaultRegistry()
	require.NotNil(t, r)
}

func TestDefaultRegistry_MobileBehavior(t *testing.T) {
	r := traits.DefaultRegistry()
	b := r.Behavior(traits.Mobile)
	require.NotNil(t, b, "Mobile must be registered")
	assert.Equal(t, traits.Mobile, b.ID)
	assert.Equal(t, "Mobile", b.DisplayName)
	assert.Equal(t, traits.ActionMoveTraitStrideKey, b.GrantsFreeAction)
	assert.True(t, b.SuppressesReactions, "Mobile must suppress reactions on its granted move")
	assert.NotEmpty(t, b.Description)
}

func TestDefaultRegistry_MoveAliasesToMobile(t *testing.T) {
	r := traits.DefaultRegistry()
	assert.Equal(t, traits.Mobile, r.CanonicalID("move"))
	b := r.Behavior("move")
	require.NotNil(t, b, "alias `move` must resolve to the Mobile behaviour")
	assert.Equal(t, traits.Mobile, b.ID)
}

func TestDefaultRegistry_UnknownIDReturnsNil(t *testing.T) {
	r := traits.DefaultRegistry()
	assert.Nil(t, r.Behavior("definitely_not_a_trait"))
	assert.False(t, r.HasBehavior("definitely_not_a_trait"))
}

func TestDefaultRegistry_CanonicalIDPassthroughForUnknown(t *testing.T) {
	r := traits.DefaultRegistry()
	assert.Equal(t, "definitely_not_a_trait", r.CanonicalID("definitely_not_a_trait"))
}

func TestDefaultRegistry_ValidateUnknownTraitDoesNotError(t *testing.T) {
	r := traits.DefaultRegistry()
	err := r.Validate([]string{"reach", "definitely_not_a_trait", traits.Mobile, "move"})
	assert.NoError(t, err, "unknown traits warn but do not error (WMOVE-4)")
}

func TestDefaultRegistry_HasBehaviorForKnown(t *testing.T) {
	r := traits.DefaultRegistry()
	assert.True(t, r.HasBehavior(traits.Mobile))
	assert.True(t, r.HasBehavior("move"))
}

// Property: every alias must resolve to a registered behaviour, and CanonicalID
// must be idempotent (canonical(canonical(x)) == canonical(x)).
func TestProperty_CanonicalID_Idempotent(t *testing.T) {
	r := traits.DefaultRegistry()
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.SampledFrom([]string{traits.Mobile, "move", "reach", "unknown_xyz", ""}).Draw(rt, "id")
		c1 := r.CanonicalID(id)
		c2 := r.CanonicalID(c1)
		if c1 != c2 {
			rt.Fatalf("CanonicalID not idempotent: %q -> %q -> %q", id, c1, c2)
		}
	})
}

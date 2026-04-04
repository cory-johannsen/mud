package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// TestHotbarJSON_MigratesOldStringFormat verifies that the old plain-string
// format is detected and migrated to typed slots on load.
func TestHotbarJSON_MigratesOldStringFormat(t *testing.T) {
	t.Parallel()
	old := `["attack goblin","","use heal_tech","","","","","","",""]`
	slots, err := postgres.UnmarshalHotbarSlots([]byte(old))
	require.NoError(t, err)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "attack goblin"}, slots[0])
	assert.Equal(t, session.HotbarSlot{}, slots[1]) // empty string → empty slot
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "use heal_tech"}, slots[2])
	// slots 3-9 should all be empty
	for i := 3; i < 10; i++ {
		assert.True(t, slots[i].IsEmpty(), "slot %d should be empty", i)
	}
}

// TestHotbarJSON_RoundTripsTypedSlots verifies that typed slots survive
// marshal → unmarshal unchanged.
func TestHotbarJSON_RoundTripsTypedSlots(t *testing.T) {
	t.Parallel()
	in := [10]session.HotbarSlot{
		{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"},
		{Kind: session.HotbarSlotKindTechnology, Ref: "healing_salve"},
		{Kind: session.HotbarSlotKindThrowable, Ref: "frag_grenade"},
		{Kind: session.HotbarSlotKindConsumable, Ref: "stim_pack"},
		{Kind: session.HotbarSlotKindCommand, Ref: "flee"},
	}
	b, err := postgres.MarshalHotbarSlots(in)
	require.NoError(t, err)
	require.NotNil(t, b)
	out, err := postgres.UnmarshalHotbarSlots(b)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

// TestHotbarJSON_AllEmptyMarshalsNil verifies an all-empty hotbar produces nil bytes.
func TestHotbarJSON_AllEmptyMarshalsNil(t *testing.T) {
	t.Parallel()
	b, err := postgres.MarshalHotbarSlots([10]session.HotbarSlot{})
	require.NoError(t, err)
	assert.Nil(t, b)
}

// TestHotbarJSON_EmptyStringInOldFormatBecomesEmptySlot verifies that an empty
// string in the old format becomes a zero-value HotbarSlot (not a command slot with empty ref).
func TestHotbarJSON_EmptyStringInOldFormatBecomesEmptySlot(t *testing.T) {
	t.Parallel()
	old := `["","","","","","","","","",""]`
	slots, err := postgres.UnmarshalHotbarSlots([]byte(old))
	require.NoError(t, err)
	for i, s := range slots {
		assert.True(t, s.IsEmpty(), "slot %d should be empty, got %+v", i, s)
	}
}

// TestProperty_HotbarJSON_RoundTrip verifies marshal/unmarshal is lossless
// for randomly-generated typed slot arrays.
func TestProperty_HotbarJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	kinds := []string{
		session.HotbarSlotKindCommand,
		session.HotbarSlotKindFeat,
		session.HotbarSlotKindTechnology,
		session.HotbarSlotKindThrowable,
		session.HotbarSlotKindConsumable,
	}
	rapid.Check(t, func(rt *rapid.T) {
		var slots [10]session.HotbarSlot
		for i := range slots {
			if rapid.Bool().Draw(rt, "assigned") {
				kind := kinds[rapid.IntRange(0, len(kinds)-1).Draw(rt, "kind")]
				ref := rapid.StringMatching(`[a-z_]+`).Draw(rt, "ref")
				if ref != "" {
					slots[i] = session.HotbarSlot{Kind: kind, Ref: ref}
				}
			}
		}
		b, err := postgres.MarshalHotbarSlots(slots)
		require.NoError(rt, err)
		if b == nil {
			// all-empty: verify all slots are empty
			for i, s := range slots {
				assert.True(rt, s.IsEmpty(), "slot %d should be empty", i)
			}
			return
		}
		out, err := postgres.UnmarshalHotbarSlots(b)
		require.NoError(rt, err)
		assert.Equal(rt, slots, out)
	})
}

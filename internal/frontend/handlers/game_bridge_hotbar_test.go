package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ── hotbarSlotCommand ────────────────────────────────────────────────────────

// REQ-HB-TB-1: nil slot returns empty command.
func TestHotbarSlotCommand_NilReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", hotbarSlotCommand(nil))
}

// REQ-HB-TB-2: slot with empty ref returns empty command regardless of kind.
func TestHotbarSlotCommand_EmptyRefReturnsEmpty(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"feat", "technology", "consumable", "throwable", "command", ""} {
		assert.Equal(t, "", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: kind, Ref: ""}), "kind=%s", kind)
	}
}

// REQ-HB-TB-3: feat kind returns "use <ref>".
func TestHotbarSlotCommand_FeatKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "use power_strike", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "feat", Ref: "power_strike"}))
}

// REQ-HB-TB-4: technology kind returns "use <ref>".
func TestHotbarSlotCommand_TechnologyKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "use neural_hack", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "technology", Ref: "neural_hack"}))
}

// REQ-HB-TB-5: consumable kind returns "use <ref>".
func TestHotbarSlotCommand_ConsumableKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "use stimpak", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "consumable", Ref: "stimpak"}))
}

// REQ-HB-TB-6: throwable kind returns "throw <ref>".
func TestHotbarSlotCommand_ThrowableKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "throw frag_grenade", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "throwable", Ref: "frag_grenade"}))
}

// REQ-HB-TB-7: command kind returns ref directly.
func TestHotbarSlotCommand_CommandKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "look north", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "command", Ref: "look north"}))
}

// REQ-HB-TB-8: empty/unrecognized kind returns ref directly.
func TestHotbarSlotCommand_DefaultKind(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "status", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "", Ref: "status"}))
	assert.Equal(t, "some_cmd", hotbarSlotCommand(&gamev1.HotbarSlot{Kind: "unknown_future_kind", Ref: "some_cmd"}))
}

// Property: hotbarSlotCommand never panics for any non-nil slot.
func TestProperty_HotbarSlotCommand_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.String().Draw(rt, "kind")
		ref := rapid.String().Draw(rt, "ref")
		assert.NotPanics(t, func() {
			hotbarSlotCommand(&gamev1.HotbarSlot{Kind: kind, Ref: ref})
		})
	})
}

// Property: if ref is non-empty, hotbarSlotCommand returns a non-empty string.
func TestProperty_HotbarSlotCommand_NonEmptyRefNonEmptyResult(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.String().Draw(rt, "kind")
		ref := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_"))).Filter(func(s string) bool { return s != "" }).Draw(rt, "ref")
		result := hotbarSlotCommand(&gamev1.HotbarSlot{Kind: kind, Ref: ref})
		assert.NotEmpty(t, result, "non-empty ref must produce non-empty command")
	})
}

// ── hotbarLabels ─────────────────────────────────────────────────────────────

// REQ-HB-TB-9: all-nil slots return zero-value [10]string.
func TestHotbarLabels_AllNilReturnsZero(t *testing.T) {
	t.Parallel()
	var slots [10]*gamev1.HotbarSlot
	labels := hotbarLabels(slots)
	assert.Equal(t, [10]string{}, labels)
}

// REQ-HB-TB-10: display_name is used when present.
func TestHotbarLabels_UsesDisplayName(t *testing.T) {
	t.Parallel()
	var slots [10]*gamev1.HotbarSlot
	slots[0] = &gamev1.HotbarSlot{Kind: "feat", Ref: "power_strike", DisplayName: "Power Strike"}
	labels := hotbarLabels(slots)
	assert.Equal(t, "Power Strike", labels[0])
}

// REQ-HB-TB-11: ref is used as fallback when display_name is empty.
func TestHotbarLabels_FallsBackToRef(t *testing.T) {
	t.Parallel()
	var slots [10]*gamev1.HotbarSlot
	slots[3] = &gamev1.HotbarSlot{Kind: "command", Ref: "look"}
	labels := hotbarLabels(slots)
	assert.Equal(t, "look", labels[3])
	assert.Equal(t, "", labels[0]) // other slots remain empty
}

// REQ-HB-TB-12: mixed nil and non-nil slots work correctly.
func TestHotbarLabels_MixedSlots(t *testing.T) {
	t.Parallel()
	var slots [10]*gamev1.HotbarSlot
	slots[0] = &gamev1.HotbarSlot{Ref: "look", DisplayName: "Look"}
	slots[5] = &gamev1.HotbarSlot{Ref: "status"} // no display name
	slots[9] = &gamev1.HotbarSlot{Ref: "attack", DisplayName: "Attack"}
	labels := hotbarLabels(slots)
	assert.Equal(t, "Look", labels[0])
	assert.Equal(t, "", labels[1])
	assert.Equal(t, "status", labels[5])
	assert.Equal(t, "Attack", labels[9])
}

// Property: hotbarLabels never panics for any combination of nil/non-nil slots.
func TestProperty_HotbarLabels_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		var slots [10]*gamev1.HotbarSlot
		for i := range slots {
			if rapid.Bool().Draw(rt, "hasSlot") {
				kind := rapid.String().Draw(rt, "kind")
				ref := rapid.String().Draw(rt, "ref")
				dn := rapid.String().Draw(rt, "displayName")
				slots[i] = &gamev1.HotbarSlot{Kind: kind, Ref: ref, DisplayName: dn}
			}
		}
		assert.NotPanics(t, func() {
			hotbarLabels(slots)
		})
	})
}

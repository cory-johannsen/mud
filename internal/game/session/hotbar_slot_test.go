package session_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
)

func TestHotbarSlot_ActivationCommand_Command(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "strike goblin"}
	assert.Equal(t, "strike goblin", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_EmptyKindFallsBackToRef(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: "", Ref: "strike goblin"}
	assert.Equal(t, "strike goblin", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Feat(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}
	assert.Equal(t, "use power_strike", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Technology(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: "healing_salve"}
	assert.Equal(t, "use healing_salve", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Throwable(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindThrowable, Ref: "frag_grenade"}
	assert.Equal(t, "throw frag_grenade", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Consumable(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindConsumable, Ref: "stim_pack"}
	assert.Equal(t, "use stim_pack", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_EmptyRefReturnsEmpty(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		session.HotbarSlotKindCommand,
		session.HotbarSlotKindFeat,
		session.HotbarSlotKindTechnology,
		session.HotbarSlotKindThrowable,
		session.HotbarSlotKindConsumable,
	} {
		s := session.HotbarSlot{Kind: kind, Ref: ""}
		assert.Equal(t, "", s.ActivationCommand(), "kind=%s", kind)
	}
}

func TestHotbarSlot_IsEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, session.HotbarSlot{}.IsEmpty())
	assert.False(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "x"}.IsEmpty())
}

func TestHotbarSlot_CommandSlot(t *testing.T) {
	t.Parallel()
	s := session.CommandSlot("attack")
	assert.Equal(t, session.HotbarSlotKindCommand, s.Kind)
	assert.Equal(t, "attack", s.Ref)
}

func TestProperty_HotbarSlot_ActivationCommand_NeverPanic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_"))).Draw(rt, "kind")
		ref := rapid.String().Draw(rt, "ref")
		s := session.HotbarSlot{Kind: kind, Ref: ref}
		_ = s.ActivationCommand() // must not panic
	})
}

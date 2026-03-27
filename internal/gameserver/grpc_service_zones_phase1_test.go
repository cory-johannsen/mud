package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyRoomEffectsOnEntry_KnownCondition verifies that when the room has an effect whose
// Track matches a registered condition ID and the player fails the save, the condition is
// applied to the session via condRegistry + ActiveSet.Apply.
//
// Precondition: condRegistry contains "fear" with BaseDC=20 and player has Grit=0 so roll+mod < 20.
// Postcondition: sess.Conditions.Has("fear") is true after applyRoomEffectsOnEntry.
func TestApplyRoomEffectsOnEntry_KnownCondition(t *testing.T) {
	// diceVal=0 → roll=1, GritMod=0 → total=1 < BaseDC=20 → fail.
	svc, sessMgr := newMentalStateSvc(t, 0)

	// Override condRegistry with one containing "fear".
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID:           "fear",
		Name:         "Fear",
		DurationType: "encounter",
		MaxStacks:    0,
	})
	svc.condRegistry = reg

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{{
			Track:           "fear",
			BaseDC:          20,
			CooldownMinutes: 5,
		}},
	}

	svc.applyRoomEffectsOnEntry(sess, "p1", room, 0)

	require.NotNil(t, sess.Conditions, "sess.Conditions must be initialized")
	assert.True(t, sess.Conditions.Has("fear"), "condition 'fear' must be active after failed save")
}

// TestApplyRoomEffectsOnEntry_UnknownCondition_Skipped verifies that an effect whose Track
// is not in the condition registry is silently skipped without panicking.
//
// Precondition: condRegistry is empty; room has effect with Track="nonexistent".
// Postcondition: no panic; sess.Conditions remains empty (no conditions applied).
func TestApplyRoomEffectsOnEntry_UnknownCondition_Skipped(t *testing.T) {
	svc, sessMgr := newMentalStateSvc(t, 0)

	// Use an empty condition registry.
	svc.condRegistry = condition.NewRegistry()

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{{
			Track:  "nonexistent",
			BaseDC: 5,
		}},
	}

	assert.NotPanics(t, func() {
		svc.applyRoomEffectsOnEntry(sess, "p1", room, 0)
	})

	if sess.Conditions != nil {
		assert.Empty(t, sess.Conditions.All(), "no conditions must be applied for unknown track")
	}
}

// TestApplyRoomEffectsOnEntry_CooldownRespected verifies that when a zone effect cooldown
// has not yet expired, the condition is not applied.
//
// Precondition: ZoneEffectCooldowns["room_b:fear"] > now; condRegistry contains "fear".
// Postcondition: sess.Conditions does not gain "fear" condition.
func TestApplyRoomEffectsOnEntry_CooldownRespected(t *testing.T) {
	// diceVal=0 → always fails, but cooldown should prevent application.
	svc, sessMgr := newMentalStateSvc(t, 0)

	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID:           "fear",
		Name:         "Fear",
		DurationType: "encounter",
	})
	svc.condRegistry = reg

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	const now = int64(1000)
	// Set cooldown to a future time (> now).
	sess.ZoneEffectCooldowns = map[string]int64{"room_b:fear": now + 300}

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{{
			Track:  "fear",
			BaseDC: 5,
		}},
	}

	svc.applyRoomEffectsOnEntry(sess, "p1", room, now)

	if sess.Conditions != nil {
		assert.False(t, sess.Conditions.Has("fear"), "fear must not be applied while cooldown is active")
	}
}

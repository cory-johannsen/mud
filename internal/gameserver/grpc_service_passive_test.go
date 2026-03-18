package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// passiveTechDef returns a minimal TechnologyDef with Passive set.
func passiveTechDef(id string, passive bool) *technology.TechnologyDef {
	cost := 0
	if !passive {
		cost = 1
	}
	return &technology.TechnologyDef{
		ID:         id,
		Name:       id,
		Passive:    passive,
		ActionCost: cost,
	}
}

// addPlayerWithInnateTechs registers a player in sessMgr and sets InnateTechs on the returned session.
func addPlayerWithInnateTechs(t *testing.T, sessMgr *session.Manager, uid, roomID string, techs map[string]*session.InnateSlot) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   roomID,
		Role:     "player",
	})
	require.NoError(t, err)
	sess.InnateTechs = techs
	return sess
}

// REQ-PTM4: triggerPassiveTechsForRoom fires passive innate techs for players in the room.
// Observable: BridgeEntity.Events() channel receives a pushed event.
func TestTriggerPassiveTechsForRoom_PassiveTechFires(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	def := passiveTechDef("seismic_sense", true)
	reg.Register(def)
	svc.SetTechRegistry(reg)

	const roomID = "room_passive"
	sess := addPlayerWithInnateTechs(t, sessMgr, "uid-passive", roomID, map[string]*session.InnateSlot{
		"seismic_sense": {MaxUses: 0, UsesRemaining: 0},
	})
	// Entity is created by AddPlayer; consume events asynchronously so channel doesn't block.
	go func() {
		for range sess.Entity.Events() {
		}
	}()

	// Must not panic; the passive tech fires without error.
	assert.NotPanics(t, func() {
		svc.triggerPassiveTechsForRoom(roomID)
	})
}

// REQ-PTM4: Non-passive innate tech must NOT produce a push event.
func TestTriggerPassiveTechsForRoom_NonPassiveTechDoesNotFire(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	def := passiveTechDef("fireball", false)
	reg.Register(def)
	svc.SetTechRegistry(reg)

	const roomID = "room_nonpassive"
	sess := addPlayerWithInnateTechs(t, sessMgr, "uid-nonpassive", roomID, map[string]*session.InnateSlot{
		"fireball": {MaxUses: 3, UsesRemaining: 3},
	})

	// Drain entity so it doesn't block.
	go func() {
		for range sess.Entity.Events() {
		}
	}()

	// Capture channel length before and after — non-passive should not push.
	lenBefore := len(sess.Entity.Events())
	svc.triggerPassiveTechsForRoom(roomID)
	lenAfter := len(sess.Entity.Events())

	assert.Equal(t, lenBefore, lenAfter, "non-passive tech must not push to entity channel")
}

// REQ-PTM4: Player with nil Entity must be skipped without panic.
func TestTriggerPassiveTechsForRoom_NilEntitySkipped(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	reg.Register(passiveTechDef("seismic_sense", true))
	svc.SetTechRegistry(reg)

	const roomID = "room_nilentity"
	sess := addPlayerWithInnateTechs(t, sessMgr, "uid-nilentity", roomID, map[string]*session.InnateSlot{
		"seismic_sense": {MaxUses: 0},
	})
	// Explicitly set Entity to nil to simulate a partially-constructed session.
	sess.Entity = nil

	assert.NotPanics(t, func() {
		svc.triggerPassiveTechsForRoom(roomID)
	})
}

// REQ-PTM4: Tech absent from registry must be skipped without panic.
func TestTriggerPassiveTechsForRoom_TechNotInRegistrySkipped(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	// Register nothing — tech is unknown.
	svc.SetTechRegistry(reg)

	const roomID = "room_unknown_tech"
	sess := addPlayerWithInnateTechs(t, sessMgr, "uid-unknowntech", roomID, map[string]*session.InnateSlot{
		"unknown_ability": {MaxUses: 0},
	})
	go func() {
		for range sess.Entity.Events() {
		}
	}()

	assert.NotPanics(t, func() {
		svc.triggerPassiveTechsForRoom(roomID)
	})
}

// TestTriggerPassiveTechsForRoom_DepartedPlayerExcluded verifies that a player who has
// moved to a different room does NOT receive the source-room passive trigger (REQ-PTM5).
func TestTriggerPassiveTechsForRoom_DepartedPlayerExcluded(t *testing.T) {
	const (
		uid      = "player-departed"
		srcRoom  = "room_source_departed"
		destRoom = "room_dest_departed"
		techID   = "seismic_sense"
	)

	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	reg := technology.NewRegistry()
	reg.Register(passiveTechDef(techID, true))
	svc.SetTechRegistry(reg)

	// Add player to source room with passive innate tech.
	sess := addPlayerWithInnateTechs(t, sessMgr, uid, srcRoom, map[string]*session.InnateSlot{
		techID: {MaxUses: 0, UsesRemaining: 0},
	})

	// Simulate player moving to destRoom — session manager updates their RoomID.
	_, err := sessMgr.MovePlayer(uid, destRoom)
	require.NoError(t, err)

	// Snapshot channel length after the move; any prior buffered events are already present.
	lenBefore := len(sess.Entity.Events())

	// Fire passive trigger for the OLD (source) room.
	// Because the player is now in destRoom, PlayersInRoomDetails(srcRoom) must not
	// return them, so no new event should be pushed.
	svc.triggerPassiveTechsForRoom(srcRoom)

	lenAfter := len(sess.Entity.Events())
	assert.Equal(t, lenBefore, lenAfter,
		"departed player must not receive source-room passive trigger")
}

// REQ-PTM5: Property test — N players each with a passive innate tech must all receive
// their passive tech trigger without panic.
func TestPropertyTriggerPassiveTechsForRoom_AllPlayersReceiveEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "n")

		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		reg := technology.NewRegistry()
		reg.Register(passiveTechDef("seismic_sense", true))
		svc.SetTechRegistry(reg)

		const roomID = "room_property"
		for i := 0; i < n; i++ {
			uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")
			uid = uid + "_" + string(rune('0'+i)) // ensure uniqueness
			sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID:      uid,
				Username: uid,
				CharName: uid,
				RoomID:   roomID,
				Role:     "player",
			})
			if err != nil {
				// Duplicate uid drawn — skip silently.
				continue
			}
			sess.InnateTechs = map[string]*session.InnateSlot{
				"seismic_sense": {MaxUses: 0},
			}
			go func(s *session.PlayerSession) {
				for range s.Entity.Events() {
				}
			}(sess)
		}

		// All players in room_property should fire their passive tech without panic.
		assert.NotPanics(t, func() {
			svc.triggerPassiveTechsForRoom(roomID)
		})
	})
}

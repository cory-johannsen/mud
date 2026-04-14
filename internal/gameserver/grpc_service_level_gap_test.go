package gameserver

// REQ-79-1: NPCs MUST NOT initiate combat when average player level exceeds NPC level by > 4.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// newSingleDangerousRoomWorld creates a minimal world with one dangerous room.
func newSingleDangerousRoomWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_lg",
		Name:        "Level Gap Test Zone",
		Description: "Zone for level gap tests",
		DangerLevel: "dangerous",
		StartRoom:   "room_lg",
		Rooms: map[string]*world.Room{
			"room_lg": {
				ID:          "room_lg",
				ZoneID:      "test_lg",
				Title:       "Danger Room",
				Description: "NPCs may initiate combat here.",
				DangerLevel: "dangerous",
				Properties:  map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// addPlayerWithLevel registers a player session with the given level.
func addPlayerWithLevel(t *testing.T, sessMgr *session.Manager, uid, roomID string, level int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   20,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Level = level
	return sess
}

// TestEvaluateThreatEngagement_LargeGap_DoesNotInitiate verifies that an NPC
// at level 1 does NOT initiate combat against a level 6+ player (gap > 4).
// REQ-79-1.
func TestEvaluateThreatEngagement_LargeGap_DoesNotInitiate(t *testing.T) {
	worldMgr, sessMgr := newSingleDangerousRoomWorld(t)
	svc := newSafeRoomService(t, worldMgr, sessMgr)
	svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

	addPlayerWithLevel(t, sessMgr, "u_lg_player", "room_lg", 6) // NPC level 1, gap = 5

	inst := &npc.Instance{
		ID:               "lg_npc",
		TemplateID:       "lg_npc",
		RoomID:           "room_lg",
		NPCType:          "combat",
		Disposition:      "hostile",
		CourageThreshold: 999, // Would otherwise always engage.
		Level:            1,
		MaxHP:            20,
		CurrentHP:        20,
	}

	svc.evaluateThreatEngagement(inst, "room_lg")

	assert.False(t, svc.combatH.IsInCombat(inst.ID),
		"level-1 NPC must NOT initiate combat against a level-6 player (gap > 4)")
}

// TestEvaluateThreatEngagement_BoundaryGap_Initiates verifies that an NPC
// at level 1 DOES initiate combat against a level 5 player (gap == 4, at threshold).
// REQ-79-1: the check is strictly > 4, so gap of 4 still allows engagement.
func TestEvaluateThreatEngagement_BoundaryGap_Initiates(t *testing.T) {
	worldMgr, sessMgr := newSingleDangerousRoomWorld(t)
	svc := newSafeRoomService(t, worldMgr, sessMgr)
	svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

	addPlayerWithLevel(t, sessMgr, "u_lg_bound", "room_lg", 5) // gap = 4, at threshold

	inst := &npc.Instance{
		ID:               "bound_npc",
		TemplateID:       "bound_npc",
		RoomID:           "room_lg",
		NPCType:          "combat",
		Disposition:      "hostile",
		CourageThreshold: 999,
		Level:            1,
		MaxHP:            20,
		CurrentHP:        20,
	}

	svc.evaluateThreatEngagement(inst, "room_lg")

	assert.True(t, svc.combatH.IsInCombat(inst.ID),
		"level-1 NPC MUST initiate combat against a level-5 player (gap == 4, within threshold)")
}

// TestEvaluateThreatEngagement_EqualLevel_Initiates verifies engagement when
// NPC and player are the same level.
func TestEvaluateThreatEngagement_EqualLevel_Initiates(t *testing.T) {
	worldMgr, sessMgr := newSingleDangerousRoomWorld(t)
	svc := newSafeRoomService(t, worldMgr, sessMgr)
	svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

	addPlayerWithLevel(t, sessMgr, "u_eq_level", "room_lg", 3) // gap = 0

	inst := &npc.Instance{
		ID:               "eq_npc",
		TemplateID:       "eq_npc",
		RoomID:           "room_lg",
		NPCType:          "combat",
		Disposition:      "hostile",
		CourageThreshold: 999,
		Level:            3,
		MaxHP:            20,
		CurrentHP:        20,
	}

	svc.evaluateThreatEngagement(inst, "room_lg")

	assert.True(t, svc.combatH.IsInCombat(inst.ID),
		"equal-level NPC MUST initiate combat against a same-level player")
}

// TestProperty_EvaluateThreatEngagement_LevelGapNeverEngages is a property-based test
// verifying that an NPC never engages when the player level gap exceeds the threshold.
// REQ-79-1.
func TestProperty_EvaluateThreatEngagement_LevelGapNeverEngages(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcLevel := rapid.IntRange(1, 10).Draw(rt, "npcLevel")
		playerLevel := rapid.IntRange(npcLevel+5, npcLevel+15).Draw(rt, "playerLevel")

		worldMgr, sessMgr := newSingleDangerousRoomWorld(t)
		svc := newSafeRoomService(t, worldMgr, sessMgr)
		svc.combatH = makeSafeRoomCombatHandler(t, sessMgr)

		addPlayerWithLevel(t, sessMgr, "u_prop_gap", "room_lg", playerLevel)

		inst := &npc.Instance{
			ID:               "prop_npc",
			TemplateID:       "prop_npc",
			RoomID:           "room_lg",
			NPCType:          "combat",
			Disposition:      "hostile",
			CourageThreshold: 999,
			Level:            npcLevel,
			MaxHP:            20,
			CurrentHP:        20,
		}

		svc.evaluateThreatEngagement(inst, "room_lg")

		if svc.combatH.IsInCombat(inst.ID) {
			rt.Fatalf("NPC level %d initiated combat against player level %d (gap %d > 4)",
				npcLevel, playerLevel, playerLevel-npcLevel)
		}
	})
}

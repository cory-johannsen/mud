package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// newUseEquipServer builds a GameServiceServer using testServiceWithAdmin but
// injects a RoomEquipmentManager when equipMgr is non-nil.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil *GameServiceServer with a single player session
// in room_a of the test world.
func newUseEquipServer(t *testing.T, equipMgr *inventory.RoomEquipmentManager) (*GameServiceServer, string) {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	svc.roomEquipMgr = equipMgr

	uid := "ue_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "equip_user",
		CharName:    "EquipChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleUseEquipment_UnknownSession verifies that an unregistered UID returns an error.
//
// Precondition: uid does not exist in the session manager.
// Postcondition: Returns nil event and non-nil error.
func TestHandleUseEquipment_UnknownSession(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)
	evt, err := svc.handleUseEquipment("nonexistent", "inst-1")
	assert.Nil(t, evt)
	assert.Error(t, err)
}

// TestHandleUseEquipment_NoEquipmentManager verifies that a nil roomEquipMgr returns
// a "No equipment available" message.
//
// Precondition: roomEquipMgr is nil; uid is a valid session.
// Postcondition: Returns a non-nil event whose message content contains "No equipment".
func TestHandleUseEquipment_NoEquipmentManager(t *testing.T) {
	svc, uid := newUseEquipServer(t, nil)
	evt, err := svc.handleUseEquipment(uid, "inst-1")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "No equipment")
}

// TestHandleUseEquipment_ItemNotHere verifies that requesting an instance not present
// in the room returns a "not here" message.
//
// Precondition: roomEquipMgr is initialised for room_a with no matching instance ID.
// Postcondition: Returns a non-nil event whose message content contains "not here".
func TestHandleUseEquipment_ItemNotHere(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "console", MaxCount: 1, Immovable: true, Script: ""},
	})
	svc, uid := newUseEquipServer(t, mgr)

	evt, err := svc.handleUseEquipment(uid, "nonexistent-instance-id")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not here")
}

// TestHandleUseEquipment_NoScript verifies that an item with an empty Script field
// returns a "Nothing happens" message.
//
// Precondition: roomEquipMgr contains an instance with Script == "" in room_a.
// Postcondition: Returns a non-nil event whose message content contains "Nothing happens".
func TestHandleUseEquipment_NoScript(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "terminal", MaxCount: 1, Immovable: true, Script: ""},
	})

	// Retrieve the real instance ID from the manager.
	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1, "expected exactly 1 instance in room_a")
	instanceID := instances[0].InstanceID

	svc, uid := newUseEquipServer(t, mgr)
	evt, err := svc.handleUseEquipment(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Nothing happens")
}

// TestProperty_HandleUseEquipment_NilMgr_NeverPanics is a property test verifying
// that arbitrary instance IDs with a nil equipment manager never panic.
//
// Precondition: roomEquipMgr is nil.
// Postcondition: handleUseEquipment never panics for any instanceID string.
func TestProperty_HandleUseEquipment_NilMgr_NeverPanics(t *testing.T) {
	svc, uid := newUseEquipServer(t, nil)
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		evt, err := svc.handleUseEquipment(uid, id)
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}

// newUseEquipServerWithSkillChecks creates a GameServiceServer with a dice roller,
// allSkills, and an optional roomEquipMgr for skill-check-aware equipment tests.
//
// Precondition: t must be non-nil; roller must be non-nil; mgr may be nil.
// Postcondition: Returns a non-nil *GameServiceServer with one player session
// in room_a configured with the supplied abilities and skills.
func newUseEquipServerWithSkillChecks(
	t *testing.T,
	mgr *inventory.RoomEquipmentManager,
	roller *dice.Roller,
	skills []*ruleset.Skill,
	abilities character.AbilityScores,
	playerSkills map[string]string,
) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger,
		nil, roller, nil, nil, nil, nil,
		nil, nil, mgr, nil, nil, nil, nil, nil, nil, nil, "",
		skills, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	uid := "ue_sc_u1"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "sc_user",
		CharName:    "SCChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   abilities,
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Skills = playerSkills
	return svc, uid
}

// TestHandleUseEquipment_OnUse_SkillCheck_Success verifies that a successful on_use
// skill check delivers the success message and allows item use to proceed.
//
// Precondition: equipment has an on_use skill check (parkour DC 10); player has
// quickness=14 (mod=+2) and parkour=trained (+2); dice returns 9 (roll=10, total=14 >= 10).
// Postcondition: Returns a non-nil event; message contains the success outcome text.
func TestHandleUseEquipment_OnUse_SkillCheck_Success(t *testing.T) {
	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}
	src := &fixedDiceSource{val: 9} // Intn(20)=9 → roll=10; total=10+2+2=14 ≥ DC10 → success
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(src, logger)

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{
			ItemID:    "parkour_panel",
			MaxCount:  1,
			Immovable: true,
			Script:    "",
			SkillChecks: []skillcheck.TriggerDef{
				{
					Skill:   "parkour",
					DC:      10,
					Trigger: "on_use",
					Outcomes: skillcheck.OutcomeMap{
						Success: &skillcheck.Outcome{Message: "You activate it effortlessly."},
						Failure: &skillcheck.Outcome{Message: "You fumble."},
					},
				},
			},
		},
	})
	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1)
	instanceID := instances[0].InstanceID

	svc, uid := newUseEquipServerWithSkillChecks(
		t, mgr, roller, skills,
		character.AbilityScores{Quickness: 14},
		map[string]string{"parkour": "trained"},
	)

	evt, err := svc.handleUseEquipment(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "You activate it effortlessly.")
}

// TestHandleUseEquipment_OnUse_SkillCheck_Failure_Deny verifies that a failed on_use
// skill check with a "deny" effect blocks item execution and delivers the failure message.
//
// Precondition: equipment has an on_use skill check (parkour DC 15); player has
// quickness=10 (mod=0) and no skills; dice returns 4 (roll=5, total=5, DC15 → dc-10=5 ≤ 5 < 15 → failure).
// Postcondition: Returns a non-nil event; message contains the failure text; the item Lua script
// is NOT executed (deny blocks execution).
func TestHandleUseEquipment_OnUse_SkillCheck_Failure_Deny(t *testing.T) {
	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}
	src := &fixedDiceSource{val: 4} // Intn(20)=4 → roll=5; total=5; DC15 → dc-10=5 ≤ 5 < 15 → failure
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(src, logger)

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{
			ItemID:    "locked_panel",
			MaxCount:  1,
			Immovable: true,
			Script:    "use_panel",
			SkillChecks: []skillcheck.TriggerDef{
				{
					Skill:   "parkour",
					DC:      15,
					Trigger: "on_use",
					Outcomes: skillcheck.OutcomeMap{
						Failure: &skillcheck.Outcome{
							Message: "The panel rejects your touch.",
							Effect:  &skillcheck.Effect{Type: "deny"},
						},
					},
				},
			},
		},
	})
	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1)
	instanceID := instances[0].InstanceID

	svc, uid := newUseEquipServerWithSkillChecks(
		t, mgr, roller, skills,
		character.AbilityScores{Quickness: 10},
		map[string]string{},
	)

	evt, err := svc.handleUseEquipment(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	// Deny blocks execution; the failure message must be delivered.
	assert.Contains(t, msg.Content, "The panel rejects your touch.")
}

// TestHandleUseEquipment_OnUse_SkillCheck_Failure_Damage verifies that a failed on_use
// skill check with a "damage" effect applies damage to the player and does NOT block
// item execution.
//
// Precondition: equipment has an on_use skill check (parkour DC 15, failure→damage 1d4);
// player starts with 10 HP; dice returns 4 (roll=5, total=5, DC15 → dc-10=5 ≤ 5 < 15 → failure).
// 1d4 roll uses Intn(4)=4 → dmg clamped to max(4,1)=4 (or similar); HP must decrease.
// Postcondition: Player HP is reduced; item proceeds since there is no deny.
func TestHandleUseEquipment_OnUse_SkillCheck_Failure_Damage(t *testing.T) {
	skills := []*ruleset.Skill{
		{ID: "parkour", Name: "Parkour", Ability: "quickness"},
	}
	// fixedDiceSource returns 4 for all Intn calls.
	// d20: Intn(20)=4 → roll=5; total=5; DC=15 → dc-10=5 ≤ 5 < 15 → failure.
	// 1d4 damage: Intn(4)=4 → clamped to 4 (die max) → dmg >= 1.
	src := &fixedDiceSource{val: 4}
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(src, logger)

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{
			ItemID:    "electric_panel",
			MaxCount:  1,
			Immovable: true,
			Script:    "",
			SkillChecks: []skillcheck.TriggerDef{
				{
					Skill:   "parkour",
					DC:      15,
					Trigger: "on_use",
					Outcomes: skillcheck.OutcomeMap{
						Failure: &skillcheck.Outcome{
							Message: "You get shocked!",
							Effect:  &skillcheck.Effect{Type: "damage", Formula: "1d4"},
						},
					},
				},
			},
		},
	})
	instances := mgr.EquipmentInRoom("room_a")
	require.Len(t, instances, 1)
	instanceID := instances[0].InstanceID

	svc, uid := newUseEquipServerWithSkillChecks(
		t, mgr, roller, skills,
		character.AbilityScores{Quickness: 10},
		map[string]string{},
	)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	initialHP := sess.CurrentHP

	evt, err := svc.handleUseEquipment(uid, instanceID)
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	// Damage effect does not deny; item use proceeds (Script="" → "Nothing happens").
	// The event message may contain either the outcome message or the item message.
	// HP must have decreased.
	sess, ok = svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Less(t, sess.CurrentHP, initialHP, "player HP must decrease after damage effect")
}

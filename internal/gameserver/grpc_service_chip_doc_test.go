package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildChipDocServer creates a minimal GameServiceServer for chip_doc (uncurse) handler tests.
//
// Precondition: t must be non-nil; diceRoller may be nil.
// Postcondition: Returns a configured server, the session manager, the npc manager, and the player UID.
func buildChipDocServer(t *testing.T, diceRoller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, string) {
	t.Helper()

	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone",
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties:  map[string]string{},
			},
		},
	}

	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	condReg := condition.NewRegistry()

	// Register fatigue condition so crit-failure tests can check for it.
	fatigueDef := &condition.ConditionDef{ID: "fatigue", Name: "Fatigue", MaxStacks: 5}
	condReg.Register(fatigueDef)

	logger := zaptest.NewLogger(t)
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	npcHandler := NewNPCHandler(npcMgr, sessMgr)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		worldHandler,
		chatHandler,
		logger,
		nil, diceRoller, npcHandler, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	uid := "chip_doc_u1"
	_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
		Abilities: character.AbilityScores{},
	})
	require.NoError(t, addErr)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.InitDone = make(chan struct{})
	close(sess.InitDone)
	sess.Conditions = condition.NewActiveSet()
	sess.Entity = session.NewBridgeEntity(uid, 8)
	sess.Currency = 500

	return svc, sessMgr, npcMgr, uid
}

// chipDocTemplate returns a minimal chip_doc NPC template with the given ID and name.
//
// Precondition: id and name must be non-empty.
// Postcondition: Returns a valid *npc.Template of type chip_doc.
func chipDocTemplate(id, name string, removalCost, checkDC int) *npc.Template {
	return &npc.Template{
		ID:      id,
		Name:    name,
		Level:   1,
		MaxHP:   10,
		AC:      10,
		NPCType: "chip_doc",
		ChipDoc: &npc.ChipDocConfig{
			RemovalCost: removalCost,
			CheckDC:     checkDC,
		},
	}
}

// TestHandleUncurse_NoNPCInRoom verifies that when the requested NPC name does not match
// any NPC in the player's room, the response contains a "don't see" message.
//
// Precondition: Room contains no NPCs; req.NpcName = "wrong_name".
// Postcondition: Response message contains "don't see" (case-insensitive).
func TestHandleUncurse_NoNPCInRoom(t *testing.T) {
	svc, _, _, uid := buildChipDocServer(t, nil)

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "wrong_name"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event")
	assert.True(t, strings.Contains(strings.ToLower(msg.Content), "don't see") ||
		strings.Contains(strings.ToLower(msg.Content), "dont see") ||
		strings.Contains(strings.ToLower(msg.Content), "not here") ||
		strings.Contains(strings.ToLower(msg.Content), "no one") ||
		strings.Contains(strings.ToLower(msg.Content), "can't find"),
		"expected message indicating NPC not found, got: %q", msg.Content)
}

// TestHandleUncurse_ItemNotEquipped verifies that when the chip_doc NPC is present but
// the player has no cursed item equipped, the response contains "no cursed item".
//
// Precondition: Room contains chip_doc NPC; sess.Equipment has no cursed items.
// Postcondition: Response message contains "no cursed item" (case-insensitive).
func TestHandleUncurse_ItemNotEquipped(t *testing.T) {
	svc, sessMgr, npcMgr, uid := buildChipDocServer(t, nil)

	tmpl := chipDocTemplate("chip_doc_1", "Doc", 100, 14)
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Equipment = inventory.NewEquipment()

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, strings.ToLower(msg.Content), "no cursed item",
		"expected 'no cursed item' message, got: %q", msg.Content)
}

// TestHandleUncurse_InsufficientCredits verifies that when the player cannot afford
// the removal cost, the response mentions "credit".
//
// Precondition: Player has 50 currency; chip_doc removal_cost=200; cursed item equipped in torso slot.
// Postcondition: Response message contains "credit" (case-insensitive).
func TestHandleUncurse_InsufficientCredits(t *testing.T) {
	svc, sessMgr, npcMgr, uid := buildChipDocServer(t, nil)

	tmpl := chipDocTemplate("chip_doc_2", "RichDoc", 200, 14)
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 50
	sess.Equipment = inventory.NewEquipment()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "cursed_vest",
		Name:      "Cursed Vest",
		Modifier:  "cursed",
	}

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "RichDoc"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, strings.ToLower(msg.Content), "credit",
		"expected credit-related message, got: %q", msg.Content)
}

// TestHandleUncurse_CritSuccess_ItemBecomesDefective verifies that on a critical success
// (roll=20 >= dc+10 when dc=14 → need 24, but crit success is roll=20 natural 20),
// the torso slot item is removed (or becomes defective), currency is deducted by cost,
// and the message mentions "removed" or "curse".
//
// Precondition: dice returns 19 (d20=20, natural 20 = crit success); player has 500 currency;
// removal_cost=100; dc=14; cursed item in torso slot.
// Postcondition: torso slot is nil OR modifier is "defective"; currency==400;
// message contains "removed" or "curse".
func TestHandleUncurse_CritSuccess_ItemBecomesDefective(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // Intn(20)=19 → d20 roll=20 → natural 20 = crit success
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, uid := buildChipDocServer(t, roller)

	tmpl := chipDocTemplate("chip_doc_crit", "CritDoc", 100, 14)
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500
	sess.Equipment = inventory.NewEquipment()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "cursed_vest",
		Name:      "Cursed Vest",
		Modifier:  "cursed",
	}

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "CritDoc"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)

	// Currency must have been deducted.
	assert.Equal(t, 400, sess.Currency, "expected 100 credits deducted")

	// Item must be removed or become defective.
	torso := sess.Equipment.Armor[inventory.SlotTorso]
	assert.True(t, torso == nil || torso.Modifier == "defective",
		"expected torso slot to be nil or defective after crit success, got: %v", torso)

	assert.True(t, strings.Contains(strings.ToLower(msg.Content), "removed") ||
		strings.Contains(strings.ToLower(msg.Content), "curse"),
		"expected 'removed' or 'curse' in message, got: %q", msg.Content)
}

// TestHandleUncurse_Failure_CreditsLostItemStays verifies that on a regular failure
// (roll=1, dc=20 — roll is well below dc and not a natural 1 crit fail at dc=20),
// the cursed item remains equipped, currency is still deducted, and the message
// mentions "fail" or "unable".
//
// Precondition: dice returns 0 (d20=1); removal_cost=100; dc=20.
// Postcondition: torso slot still has cursed item; currency==400;
// message contains "fail" or "unable".
func TestHandleUncurse_Failure_CreditsLostItemStays(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // Intn(20)=0 → d20 roll=1
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, uid := buildChipDocServer(t, roller)

	// dc=20: roll=1 → 1 < 20-10=10 → crit failure. Use dc=15 to make it a plain failure.
	// With roll=1, total=1 (no modifiers) → well below dc=15 → failure (not crit since 1 < 15-10=5? check).
	// Per PF2e: crit failure = total <= dc-10. dc=15 → crit fail threshold = 5. roll=1 <= 5 → crit fail.
	// Use dc=11 so crit-fail threshold is 1, but roll=1 exactly equals it → crit fail. Try dc=10.
	// dc=10 → crit fail threshold = 0. roll=1 > 0 → plain failure. Use dc=10 for plain failure.
	tmpl := chipDocTemplate("chip_doc_fail", "FailDoc", 100, 10)
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500
	sess.Equipment = inventory.NewEquipment()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "cursed_vest",
		Name:      "Cursed Vest",
		Modifier:  "cursed",
	}

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "FailDoc"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)

	// Credits must be deducted even on failure.
	assert.Equal(t, 400, sess.Currency, "expected 100 credits deducted even on failure")

	// Cursed item must still be equipped.
	torso := sess.Equipment.Armor[inventory.SlotTorso]
	require.NotNil(t, torso, "expected cursed item to remain in torso slot after failure")
	assert.Equal(t, "cursed", torso.Modifier, "expected item to remain cursed after failure")

	assert.True(t, strings.Contains(strings.ToLower(msg.Content), "fail") ||
		strings.Contains(strings.ToLower(msg.Content), "unable"),
		"expected 'fail' or 'unable' in message, got: %q", msg.Content)
}

// TestHandleUncurse_CritFailure_FatigueApplied verifies that on a critical failure
// the fatigue condition is applied, the item stays cursed, and currency is deducted.
//
// Precondition: dice returns 0 (d20=1); removal_cost=100; dc=30 so crit fail threshold=20,
// total=1 <= 20 → crit failure guaranteed.
// Postcondition: sess.Conditions.Stacks("fatigue")==1; torso slot still cursed; currency==400.
func TestHandleUncurse_CritFailure_FatigueApplied(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // Intn(20)=0 → d20 roll=1
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, uid := buildChipDocServer(t, roller)

	tmpl := chipDocTemplate("chip_doc_critfail", "CritFailDoc", 100, 30)
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500
	sess.Equipment = inventory.NewEquipment()
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "cursed_vest",
		Name:      "Cursed Vest",
		Modifier:  "cursed",
	}

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "CritFailDoc"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Credits deducted.
	assert.Equal(t, 400, sess.Currency, "expected 100 credits deducted on crit failure")

	// Cursed item remains.
	torso := sess.Equipment.Armor[inventory.SlotTorso]
	require.NotNil(t, torso, "expected cursed item to remain after crit failure")
	assert.Equal(t, "cursed", torso.Modifier, "expected item to remain cursed after crit failure")

	// Fatigue condition applied.
	assert.Equal(t, 1, sess.Conditions.Stacks("fatigue"),
		"expected fatigue condition stack count == 1 after crit failure")
}

// TestProperty_HandleUncurse_CreditsAlwaysDeductedWhenFound verifies with random dice rolls
// that whenever a chip_doc NPC is found and the player can afford the cost, the currency is
// always decremented by exactly the removal cost.
//
// Precondition: Player has 500 credits; removal_cost=100; dc=14; cursed item in torso.
// Postcondition: currency == 400 regardless of roll outcome.
func TestProperty_HandleUncurse_CreditsAlwaysDeductedWhenFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rollVal := rapid.IntRange(0, 19).Draw(rt, "roll")

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: rollVal}
		roller := dice.NewLoggedRoller(src, logger)

		svc, sessMgr, npcMgr, uid := buildChipDocServer(t, roller)

		tmpl := chipDocTemplate("chip_doc_prop", "PropDoc", 100, 14)
		_, err := npcMgr.Spawn(tmpl, "room_a")
		if err != nil {
			rt.Fatal(err)
		}

		sess, ok := sessMgr.GetPlayer(uid)
		if !ok {
			rt.Fatal("session not found")
		}
		sess.Currency = 500
		sess.Equipment = inventory.NewEquipment()
		sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
			ItemDefID: "cursed_vest",
			Name:      "Cursed Vest",
			Modifier:  "cursed",
		}

		_, err = svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "PropDoc"})
		if err != nil {
			rt.Fatal(err)
		}

		if sess.Currency != 400 {
			rt.Fatalf("expected currency==400 after uncurse with roll=%d, got %d", rollVal+1, sess.Currency)
		}
	})
}

package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// newAdminSvc creates a minimal GameServiceServer for admin RPC tests.
func newAdminSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr
}

// addAdminPlayer registers a player with the given options.
func addAdminPlayer(t *testing.T, sessMgr *session.Manager, opts session.AddPlayerOptions) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(opts)
	require.NoError(t, err)
	require.NotNil(t, sess)
	return sess
}

// ---------------------------------------------------------------------------
// AdminListSessions tests
// ---------------------------------------------------------------------------

// TestAdminListSessions_OmitsZeroCharID verifies that sessions with CharacterID == 0 are excluded.
func TestAdminListSessions_OmitsZeroCharID(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	// Add a player with CharacterID == 0 (should be omitted).
	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_anon",
		Username:    "anon",
		CharName:    "Anon",
		CharacterID: 0,
		RoomID:      "room_a",
		Role:        "player",
	})
	// Add a player with CharacterID > 0 (should appear).
	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_known",
		Username:    "known",
		CharName:    "Known",
		CharacterID: 42,
		RoomID:      "room_a",
		Role:        "player",
	})

	resp, err := svc.AdminListSessions(context.Background(), &gamev1.AdminListSessionsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	for _, info := range resp.Sessions {
		assert.NotZero(t, info.CharId, "sessions with CharacterID == 0 must be omitted")
	}
	require.Len(t, resp.Sessions, 1)
	assert.Equal(t, int64(42), resp.Sessions[0].CharId)
}

// TestAdminListSessions_MapsSessionFields verifies that all session fields are correctly mapped.
func TestAdminListSessions_MapsSessionFields(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_map1",
		Username:    "mapper",
		CharName:    "Mapper",
		CharacterID: 99,
		RoomID:      "room_a",
		CurrentHP:   25,
		Level:       5,
		Role:        "player",
	})

	resp, err := svc.AdminListSessions(context.Background(), &gamev1.AdminListSessionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Sessions, 1)

	info := resp.Sessions[0]
	assert.Equal(t, int64(99), info.CharId)
	assert.Equal(t, "Mapper", info.PlayerName)
	assert.Equal(t, int32(5), info.Level)
	assert.Equal(t, "room_a", info.RoomId)
	assert.Equal(t, "test", info.Zone, "zone should come from room's ZoneID")
	assert.Equal(t, int32(25), info.CurrentHp)
	assert.Equal(t, int64(0), info.AccountId, "AccountId must be 0 — not available in PlayerSession")
}

// TestAdminListSessions_PropertyZone uses rapid to verify zone is always empty string
// when a player is in a room that does not exist in the world.
func TestAdminListSessions_PropertyZone(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newAdminSvc(t)

		charID := rapid.Int64Range(1, 10000).Draw(rt, "charID")
		// Use a room ID that does not exist in the test world so zone falls through to "".
		addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
			UID:         "u_prop",
			Username:    "prop",
			CharName:    "PropPlayer",
			CharacterID: charID,
			RoomID:      "room_a", // valid room so AddPlayer doesn't fail
			Role:        "player",
		})
		// Move the player to a nonexistent room via the session Manager to avoid
		// bypassing the Manager's mutex with a direct field mutation.
		sessMgr.MovePlayer("u_prop", "nonexistent_room")

		resp, err := svc.AdminListSessions(context.Background(), &gamev1.AdminListSessionsRequest{})
		require.NoError(rt, err)
		require.Len(rt, resp.Sessions, 1)
		assert.Equal(rt, "", resp.Sessions[0].Zone, "zone must be empty string when room is not found")
	})
}

// ---------------------------------------------------------------------------
// AdminKickPlayer tests
// ---------------------------------------------------------------------------

// TestAdminKickPlayer_NotFound verifies codes.NotFound when the player is not online.
func TestAdminKickPlayer_NotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.AdminKickPlayer(context.Background(), &gamev1.AdminKickRequest{CharId: 9999})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestAdminKickPlayer_PushesDisconnectedEvent verifies a parseable ServerEvent with
// Disconnected payload is pushed to the target player's entity.
func TestAdminKickPlayer_PushesDisconnectedEvent(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	target := addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_kick",
		Username:    "kicker",
		CharName:    "Victim",
		CharacterID: 7,
		RoomID:      "room_a",
		Role:        "player",
	})

	resp, err := svc.AdminKickPlayer(context.Background(), &gamev1.AdminKickRequest{CharId: 7})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Drain one event from entity.
	events := target.Entity.Events()
	require.NotEmpty(t, events)
	data := <-events

	var evt gamev1.ServerEvent
	require.NoError(t, proto.Unmarshal(data, &evt))

	disc := evt.GetDisconnected()
	require.NotNil(t, disc, "event must carry a Disconnected payload")
	assert.Contains(t, disc.Reason, "Victim", "disconnect reason must mention the player's name")
}

// ---------------------------------------------------------------------------
// AdminMessagePlayer tests
// ---------------------------------------------------------------------------

// TestAdminMessagePlayer_NotFound verifies codes.NotFound when player is not online.
func TestAdminMessagePlayer_NotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.AdminMessagePlayer(context.Background(), &gamev1.AdminMessageRequest{
		CharId: 8888,
		Text:   "hello",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestAdminMessagePlayer_PushesMessageEvent verifies the text is delivered as a Message payload.
func TestAdminMessagePlayer_PushesMessageEvent(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	target := addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_msg",
		Username:    "msger",
		CharName:    "Receiver",
		CharacterID: 55,
		RoomID:      "room_a",
		Role:        "player",
	})

	const wantText = "Admin says hello"
	resp, err := svc.AdminMessagePlayer(context.Background(), &gamev1.AdminMessageRequest{
		CharId: 55,
		Text:   wantText,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	events := target.Entity.Events()
	require.NotEmpty(t, events)
	data := <-events

	var evt gamev1.ServerEvent
	require.NoError(t, proto.Unmarshal(data, &evt))

	msg := evt.GetMessage()
	require.NotNil(t, msg, "event must carry a Message payload")
	assert.Equal(t, wantText, msg.Content)
	assert.Equal(t, gamev1.MessageType_MESSAGE_TYPE_SAY, msg.Type)
}

// TestAdminMessagePlayer_PropertyText uses rapid to verify arbitrary text is forwarded exactly.
func TestAdminMessagePlayer_PropertyText(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newAdminSvc(t)

		text := rapid.StringMatching(`[a-zA-Z0-9 ]+`).Draw(rt, "text")
		addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
			UID:         "u_proptext",
			Username:    "proptext",
			CharName:    "TextReceiver",
			CharacterID: 101,
			RoomID:      "room_a",
			Role:        "player",
		})
		target := sessMgr.GetPlayerByCharID(101)
		require.NotNil(rt, target)

		_, err := svc.AdminMessagePlayer(context.Background(), &gamev1.AdminMessageRequest{
			CharId: 101,
			Text:   text,
		})
		require.NoError(rt, err)

		data := <-target.Entity.Events()
		var evt gamev1.ServerEvent
		require.NoError(rt, proto.Unmarshal(data, &evt))
		assert.Equal(rt, text, evt.GetMessage().Content)
	})
}

// ---------------------------------------------------------------------------
// AdminTeleportPlayer tests
// ---------------------------------------------------------------------------

// TestAdminTeleportPlayer_CharNotFound verifies codes.NotFound when the char is not online.
func TestAdminTeleportPlayer_CharNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.AdminTeleportPlayer(context.Background(), &gamev1.AdminTeleportRequest{
		CharId: 7777,
		RoomId: "room_a",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestAdminTeleportPlayer_RoomNotFound verifies codes.NotFound when the room does not exist.
func TestAdminTeleportPlayer_RoomNotFound(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_tele_roomnf",
		Username:    "teleroomnf",
		CharName:    "TeleRoomNF",
		CharacterID: 200,
		RoomID:      "room_a",
		Role:        "player",
	})

	_, err := svc.AdminTeleportPlayer(context.Background(), &gamev1.AdminTeleportRequest{
		CharId: 200,
		RoomId: "nonexistent_room",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestAdminTeleportPlayer_MovesAndBroadcasts verifies MovePlayer is called and the target
// entity receives at least one push (the teleport message).
func TestAdminTeleportPlayer_MovesAndBroadcasts(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	target := addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_tele",
		Username:    "tele",
		CharName:    "Traveller",
		CharacterID: 300,
		RoomID:      "room_a",
		Role:        "player",
	})

	resp, err := svc.AdminTeleportPlayer(context.Background(), &gamev1.AdminTeleportRequest{
		CharId: 300,
		RoomId: "room_b",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify MovePlayer was called: the session's room should now be room_b.
	assert.Equal(t, "room_b", target.RoomID, "player's RoomID must be updated after teleport")

	// Verify at least one event was pushed to the target.
	events := target.Entity.Events()
	require.NotEmpty(t, events, "target entity must receive at least one push")

	// Decode and verify first push is the teleport message.
	data := <-events
	var evt gamev1.ServerEvent
	require.NoError(t, proto.Unmarshal(data, &evt))
	msg := evt.GetMessage()
	require.NotNil(t, msg, "first push must be a message event")
	assert.Contains(t, msg.Content, "teleported")
}

// ---------------------------------------------------------------------------
// AdminGiveItem tests
// ---------------------------------------------------------------------------

// newAdminSvcWithRegistry creates a minimal GameServiceServer with an invRegistry for item-give tests.
func newAdminSvcWithRegistry(t *testing.T, reg *inventory.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, reg, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr
}

// REQ-ADM-1: AdminGiveItem must return NotFound when player is not online.
func TestAdminGiveItem_PlayerNotFound(t *testing.T) {
	reg := inventory.NewRegistry()
	svc, _ := newAdminSvcWithRegistry(t, reg)

	_, err := svc.AdminGiveItem(context.Background(), &gamev1.AdminGiveItemRequest{
		CharId:   999,
		ItemId:   "potion",
		Quantity: 1,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// REQ-ADM-1: AdminGiveItem must return NotFound when item_id is not in the registry.
func TestAdminGiveItem_UnknownItem(t *testing.T) {
	reg := inventory.NewRegistry()
	svc, sessMgr := newAdminSvcWithRegistry(t, reg)

	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_give",
		Username:    "give_user",
		CharName:    "Giver",
		CharacterID: 500,
		RoomID:      "room_a",
		Role:        "player",
	})

	_, err := svc.AdminGiveItem(context.Background(), &gamev1.AdminGiveItemRequest{
		CharId:   500,
		ItemId:   "nonexistent_item",
		Quantity: 1,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// REQ-ADM-1: AdminGiveItem must add the item to the target player's backpack.
func TestAdminGiveItem_AddsToBackpack(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID:       "test_stim",
		Name:     "Test Stim",
		Kind:     inventory.KindConsumable,
		MaxStack: 10,
		Weight:   0.1,
	}))
	svc, sessMgr := newAdminSvcWithRegistry(t, reg)

	target := addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_give2",
		Username:    "give_user2",
		CharName:    "Receiver",
		CharacterID: 501,
		RoomID:      "room_a",
		Role:        "player",
	})

	resp, err := svc.AdminGiveItem(context.Background(), &gamev1.AdminGiveItemRequest{
		CharId:   501,
		ItemId:   "test_stim",
		Quantity: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify item is in backpack.
	items := target.Backpack.FindByItemDefID("test_stim")
	totalQty := 0
	for _, it := range items {
		totalQty += it.Quantity
	}
	assert.Equal(t, 3, totalQty, "backpack must contain 3 test_stim after AdminGiveItem")
}

// ---------------------------------------------------------------------------
// AdminGiveCurrency tests
// ---------------------------------------------------------------------------

// REQ-ADM-2: AdminGiveCurrency must return NotFound when player is not online.
func TestAdminGiveCurrency_PlayerNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.AdminGiveCurrency(context.Background(), &gamev1.AdminGiveCurrencyRequest{
		CharId: 999,
		Amount: 100,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// REQ-ADM-2: AdminGiveCurrency must return InvalidArgument when amount < 1.
func TestAdminGiveCurrency_InvalidAmount(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_curr",
		Username:    "curr_user",
		CharName:    "Holder",
		CharacterID: 600,
		RoomID:      "room_a",
		Role:        "player",
	})

	_, err := svc.AdminGiveCurrency(context.Background(), &gamev1.AdminGiveCurrencyRequest{
		CharId: 600,
		Amount: 0,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// REQ-ADM-2: AdminGiveCurrency must add the amount to the target player's Currency.
func TestAdminGiveCurrency_AddsToCurrency(t *testing.T) {
	svc, sessMgr := newAdminSvc(t)

	target := addAdminPlayer(t, sessMgr, session.AddPlayerOptions{
		UID:         "u_curr2",
		Username:    "curr_user2",
		CharName:    "Banker",
		CharacterID: 601,
		RoomID:      "room_a",
		Role:        "player",
	})
	initialCurrency := target.Currency

	resp, err := svc.AdminGiveCurrency(context.Background(), &gamev1.AdminGiveCurrencyRequest{
		CharId: 601,
		Amount: 250,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, initialCurrency+250, target.Currency, "player's Currency must increase by 250 after AdminGiveCurrency")
}

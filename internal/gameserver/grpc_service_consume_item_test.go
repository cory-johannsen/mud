package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// buildConsumeItemService builds a minimal GameServiceServer with an inventory
// registry containing a plain consumable item (no substance_id).
func buildConsumeItemService(t *testing.T, sessMgr *session.Manager, item *inventory.ItemDef) *GameServiceServer {
	t.Helper()
	worldMgr, _ := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	reg := inventory.NewRegistry()
	if err := reg.RegisterItem(item); err != nil {
		t.Fatalf("RegisterItem: %v", err)
	}
	return newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
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
}

// addPlayerWithBackpack adds a player with an initialised backpack and adds qty
// units of item to it.
func addPlayerWithBackpack(t *testing.T, sessMgr *session.Manager, uid string, item *inventory.ItemDef, qty int, reg *inventory.Registry) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Backpack = inventory.NewBackpack(20, 50.0)
	for i := 0; i < qty; i++ {
		_, err := sess.Backpack.Add(item.ID, 1, reg)
		require.NoError(t, err, "Add item to backpack")
	}
	return sess
}

// TestHandleUse_PlainConsumable_ConsumesItemAndReturnsMessage verifies that using a
// plain consumable (Kind==consumable, no substance_id) removes it from the backpack
// and returns a success message, not "No prepared uses of … remaining."
func TestHandleUse_PlainConsumable_ConsumesItemAndReturnsMessage(t *testing.T) {
	item := &inventory.ItemDef{
		ID:        "canadian_bacon",
		Name:      "Canadian Bacon",
		Kind:      inventory.KindConsumable,
		Stackable: true,
		MaxStack:  5,
	}
	sessMgr := session.NewManager()
	svc := buildConsumeItemService(t, sessMgr, item)
	reg := svc.invRegistry

	sess := addPlayerWithBackpack(t, sessMgr, "u_consume_plain", item, 2, reg)
	initialInstances := sess.Backpack.FindByItemDefID("canadian_bacon")
	require.Len(t, initialInstances, 1, "setup: one stacked instance in backpack")
	require.Equal(t, 2, initialInstances[0].Quantity, "setup: 2 items in backpack")

	event, err := svc.handleUse("u_consume_plain", "canadian_bacon", "")
	require.NoError(t, err)
	require.NotNil(t, event)

	// Must be a Message event with success text, not an error about prepared tech.
	msg := event.GetMessage()
	require.NotNil(t, msg, "expected Message event")
	assert.Contains(t, msg.Content, "Canadian Bacon", "message must reference the item name")
	assert.NotContains(t, msg.Content, "No prepared uses", "must not return prepared-tech error")

	// Backpack must have one fewer item (still 1 stacked instance, quantity reduced).
	remainingInst := sess.Backpack.FindByItemDefID("canadian_bacon")
	require.Len(t, remainingInst, 1, "stacked instance still present")
	assert.Equal(t, 1, remainingInst[0].Quantity, "one item must be removed from backpack")
}

// TestHandleUse_PlainConsumable_LastItem_EmptiesSlot verifies all items consumed.
func TestHandleUse_PlainConsumable_LastItem_EmptiesSlot(t *testing.T) {
	item := &inventory.ItemDef{
		ID:        "fresh_produce",
		Name:      "Fresh Produce",
		Kind:      inventory.KindConsumable,
		Stackable: true,
		MaxStack:  10,
	}
	sessMgr := session.NewManager()
	svc := buildConsumeItemService(t, sessMgr, item)
	reg := svc.invRegistry

	sess := addPlayerWithBackpack(t, sessMgr, "u_consume_last", item, 1, reg)

	event, err := svc.handleUse("u_consume_last", "fresh_produce", "")
	require.NoError(t, err)
	require.NotNil(t, event)
	require.NotNil(t, event.GetMessage())

	remaining := len(sess.Backpack.FindByItemDefID("fresh_produce"))
	assert.Equal(t, 0, remaining, "backpack must be empty after last item consumed")
}

// TestProperty_PlainConsumable_NeverReturnsPreparedTechError: for any quantity >= 1,
// using a plain consumable must never produce "No prepared uses" error text.
func TestProperty_PlainConsumable_NeverReturnsPreparedTechError(t *testing.T) {
	item := &inventory.ItemDef{
		ID:        "prop_food",
		Name:      "Prop Food",
		Kind:      inventory.KindConsumable,
		Stackable: true,
		MaxStack:  20,
	}
	rapid.Check(t, func(rt *rapid.T) {
		qty := rapid.IntRange(1, 20).Draw(rt, "qty")
		sessMgr := session.NewManager()
		svc := buildConsumeItemService(t, sessMgr, item)
		reg := svc.invRegistry
		uid := "u_prop_consume"
		addPlayerWithBackpack(t, sessMgr, uid, item, qty, reg)

		event, err := svc.handleUse(uid, "prop_food", "")
		if err != nil {
			rt.Errorf("handleUse returned error: %v", err)
			return
		}
		if event == nil {
			rt.Errorf("handleUse returned nil event")
			return
		}
		msg := event.GetMessage()
		if msg == nil {
			rt.Errorf("expected Message event, got %T", event.Payload)
			return
		}
		if msg.Content == "" {
			rt.Errorf("message content must not be empty")
		}
		if contains(msg.Content, "No prepared uses") {
			rt.Errorf("must not return prepared-tech error: %q", msg.Content)
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

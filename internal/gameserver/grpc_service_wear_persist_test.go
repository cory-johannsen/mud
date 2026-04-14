package gameserver

// REQ-66-1: handleWear MUST persist inventory and equipment to durable storage
// immediately after a successful wear operation.
// REQ-66-2: handleRemoveArmor MUST persist inventory and equipment to durable
// storage immediately after a successful removal.

import (
	"context"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// wearTrackingCharSaver wraps mockCharSaver, counting SaveInventory/SaveEquipment calls.
type wearTrackingCharSaver struct {
	mockCharSaver
	saveInvCalls int64
	saveEqCalls  int64
}

func (s *wearTrackingCharSaver) SaveInventory(_ context.Context, characterID int64, items []inventory.InventoryItem) error {
	atomic.AddInt64(&s.saveInvCalls, 1)
	return nil
}

func (s *wearTrackingCharSaver) SaveEquipment(_ context.Context, characterID int64, eq *inventory.Equipment) error {
	atomic.AddInt64(&s.saveEqCalls, 1)
	return nil
}

// newWearTestRegistry returns a Registry with a chest armor piece registered.
func newWearTestRegistry() *inventory.Registry {
	reg := inventory.NewRegistry()
	_ = reg.RegisterItem(&inventory.ItemDef{
		ID:       "test-chest",
		Name:     "Test Chest Plate",
		Kind:     inventory.KindArmor,
		ArmorRef: "test-chest-def",
		MaxStack: 1,
	})
	_ = reg.RegisterArmor(&inventory.ArmorDef{
		ID:   "test-chest-def",
		Name: "Test Chest Plate",
		Slot: inventory.SlotTorso,
	})
	return reg
}

// newWearTestService creates a GameServiceServer wired with a tracking charSaver and
// the given inventory registry. Returns the service, tracker, and the player session.
func newWearTestService(t *testing.T, uid string, charID int64, addToBackpack bool) (*GameServiceServer, *wearTrackingCharSaver, *session.PlayerSession) {
	t.Helper()
	reg := newWearTestRegistry()
	tracker := &wearTrackingCharSaver{mockCharSaver: mockCharSaver{saved: make(map[int64]string)}}
	wm, sessMgr := newSingleDangerousRoomWorld(t)

	svc := newTestGameServiceServer(
		wm, sessMgr,
		nil, nil, nil,
		zap.NewNop(),
		tracker,                                                                           // charSaver
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		reg,                                                                               // invRegistry
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "testuser",
		CharName:    "Warrior",
		CharacterID: charID,
		RoomID:      "room_lg",
		CurrentHP:   20,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}

	if addToBackpack {
		if _, err := sess.Backpack.Add("test-chest", 1, reg); err != nil {
			t.Fatalf("Backpack.Add: %v", err)
		}
	}

	return svc, tracker, sess
}

// TestHandleWear_PersistsInventoryAndEquipment verifies that a successful wear
// call immediately saves both inventory and equipment to durable storage. REQ-66-1.
func TestHandleWear_PersistsInventoryAndEquipment(t *testing.T) {
	svc, tracker, _ := newWearTestService(t, "uid-wear-persist", 42, true)

	_, err := svc.handleWear("uid-wear-persist", &gamev1.WearRequest{
		ItemId: "test-chest",
		Slot:   "torso",
	})
	if err != nil {
		t.Fatalf("handleWear: %v", err)
	}

	if atomic.LoadInt64(&tracker.saveInvCalls) == 0 {
		t.Error("REQ-66-1: SaveInventory must be called after successful wear; was not called")
	}
	if atomic.LoadInt64(&tracker.saveEqCalls) == 0 {
		t.Error("REQ-66-1: SaveEquipment must be called after successful wear; was not called")
	}
}

// TestHandleWear_FailedWear_DoesNotPersist verifies that a failed wear (item not in
// backpack) does NOT call SaveInventory or SaveEquipment. REQ-66-1.
func TestHandleWear_FailedWear_DoesNotPersist(t *testing.T) {
	// No item added to backpack — wear will fail.
	svc, tracker, _ := newWearTestService(t, "uid-wear-fail", 43, false)

	_, err := svc.handleWear("uid-wear-fail", &gamev1.WearRequest{
		ItemId: "test-chest",
		Slot:   "torso",
	})
	if err != nil {
		t.Fatalf("handleWear: %v", err)
	}

	if atomic.LoadInt64(&tracker.saveInvCalls) != 0 {
		t.Error("SaveInventory must NOT be called after a failed wear")
	}
	if atomic.LoadInt64(&tracker.saveEqCalls) != 0 {
		t.Error("SaveEquipment must NOT be called after a failed wear")
	}
}

// TestHandleRemoveArmor_PersistsInventoryAndEquipment verifies that a successful
// remove-armor call immediately saves both inventory and equipment. REQ-66-2.
func TestHandleRemoveArmor_PersistsInventoryAndEquipment(t *testing.T) {
	svc, tracker, sess := newWearTestService(t, "uid-remove-persist", 44, false)

	// Equip the item directly so we can test removal.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "test-chest-def",
	}

	_, err := svc.handleRemoveArmor("uid-remove-persist", &gamev1.RemoveArmorRequest{
		Slot: "torso",
	})
	if err != nil {
		t.Fatalf("handleRemoveArmor: %v", err)
	}

	if atomic.LoadInt64(&tracker.saveInvCalls) == 0 {
		t.Error("REQ-66-2: SaveInventory must be called after successful remove-armor; was not called")
	}
	if atomic.LoadInt64(&tracker.saveEqCalls) == 0 {
		t.Error("REQ-66-2: SaveEquipment must be called after successful remove-armor; was not called")
	}
}

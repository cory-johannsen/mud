package gameserver

// REQ-88-1: handleWear MUST push a CharacterSheetView event to the player's entity
// after a successful wear so the web UI equipment drawer refreshes immediately.
// REQ-88-2: handleEquip MUST push a CharacterSheetView event to the player's entity
// after a successful equip so the web UI equipment drawer reflects the new weapon.

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/protobuf/proto"
)

// drainCharacterSheetPushes reads all pending events from the given channel (with
// a short timeout) and returns the count of CharacterSheetView events received.
func drainCharacterSheetPushes(ch <-chan []byte, timeout time.Duration) int {
	count := 0
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case data := <-ch:
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			if evt.GetCharacterSheet() != nil {
				count++
			}
		case <-timer.C:
			return count
		}
	}
}

// TestHandleWear_PushesCharacterSheet verifies that a successful wear sends a
// CharacterSheetView event to the player's entity so the equipment drawer
// refreshes immediately (REQ-88-1).
func TestHandleWear_PushesCharacterSheet(t *testing.T) {
	svc, _, sess := newWearTestService(t, "uid-wear-charsheet", 50, true)

	_, err := svc.handleWear("uid-wear-charsheet", &gamev1.WearRequest{
		ItemId: "test-chest",
		Slot:   "torso",
	})
	if err != nil {
		t.Fatalf("handleWear: %v", err)
	}

	count := drainCharacterSheetPushes(sess.Entity.Events(), 200*time.Millisecond)
	if count == 0 {
		t.Error("REQ-88-1: CharacterSheetView must be pushed after successful wear; was not pushed")
	}
}

// TestHandleWear_FailedWear_DoesNotPushCharacterSheet verifies that a failed wear
// (item not in backpack) does NOT push a CharacterSheetView event. REQ-88-1.
func TestHandleWear_FailedWear_DoesNotPushCharacterSheet(t *testing.T) {
	svc, _, sess := newWearTestService(t, "uid-wear-charsheet-fail", 51, false)

	_, err := svc.handleWear("uid-wear-charsheet-fail", &gamev1.WearRequest{
		ItemId: "test-chest",
		Slot:   "torso",
	})
	if err != nil {
		t.Fatalf("handleWear: %v", err)
	}

	count := drainCharacterSheetPushes(sess.Entity.Events(), 100*time.Millisecond)
	if count != 0 {
		t.Error("CharacterSheetView must NOT be pushed after a failed wear")
	}
}

// TestHandleEquip_PushesCharacterSheet verifies that a successful weapon equip sends
// a CharacterSheetView event to the player's entity so the equipment drawer reflects
// the new weapon immediately (REQ-88-2).
func TestHandleEquip_PushesCharacterSheet(t *testing.T) {
	reg := inventory.NewRegistry()
	if err := reg.RegisterWeapon(&inventory.WeaponDef{
		ID:                  "push_test_cleaver",
		Name:                "Push Test Cleaver",
		Kind:                inventory.WeaponKindOneHanded,
		DamageDice:          "1d6",
		DamageType:          "slashing",
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}); err != nil {
		t.Fatalf("RegisterWeapon: %v", err)
	}
	if err := reg.RegisterItem(&inventory.ItemDef{
		ID:        "push_test_cleaver",
		Name:      "Push Test Cleaver",
		Kind:      "weapon",
		WeaponRef: "push_test_cleaver",
		MaxStack:  1,
	}); err != nil {
		t.Fatalf("RegisterItem: %v", err)
	}

	wm, sessMgr := newSingleDangerousRoomWorld(t)
	svc := newTestGameServiceServer(
		wm, sessMgr,
		nil, nil, nil, zap.NewNop(),
		nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		reg,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
	)

	uid := "uid-equip-charsheet"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "testuser",
		CharName:    "Warrior",
		CharacterID: 60,
		RoomID:      "room_lg",
		CurrentHP:   20,
		MaxHP:       20,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}

	sess, ok := sessMgr.GetPlayer(uid)
	if !ok {
		t.Fatal("session not found after AddPlayer")
	}
	if _, err := sess.Backpack.Add("push_test_cleaver", 1, reg); err != nil {
		t.Fatalf("Backpack.Add: %v", err)
	}

	_, err = svc.handleEquip(uid, &gamev1.EquipRequest{
		WeaponId: "push_test_cleaver",
		Slot:     "main",
		Preset:   1,
	})
	if err != nil {
		t.Fatalf("handleEquip: %v", err)
	}

	count := drainCharacterSheetPushes(sess.Entity.Events(), 200*time.Millisecond)
	if count == 0 {
		t.Error("REQ-88-2: CharacterSheetView must be pushed after successful equip; was not pushed")
	}
}

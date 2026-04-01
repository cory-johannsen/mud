package gameserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const loadoutSentinel = "\x00loadout\x00"

// extractLoadoutView decodes a sentinel-encoded LoadoutView from a ServerEvent MessageEvent.
// Returns nil if the event does not carry a loadout sentinel.
func extractLoadoutView(evt *gamev1.ServerEvent) *gamev1.LoadoutView {
	msg := evt.GetMessage()
	if msg == nil || !strings.HasPrefix(msg.Content, loadoutSentinel) {
		return nil
	}
	var lv gamev1.LoadoutView
	if err := json.Unmarshal([]byte(msg.Content[len(loadoutSentinel):]), &lv); err != nil {
		return nil
	}
	return &lv
}

// TestHandleLoadout_NoArg_ReturnsLoadoutView verifies that handleLoadout with an empty arg
// returns a structured LoadoutView event (not a message), which the web client renders
// as a loadout management UI and the telnet bridge renders as text.
//
// Precondition: Player session has a non-nil LoadoutSet and Class set.
// Postcondition: Returns a ServerEvent carrying a LoadoutView with the correct preset count.
func TestHandleLoadout_NoArg_ReturnsLoadoutView(t *testing.T) {
	svc := newLoadoutServer(t, "u_combined")
	sess, ok := svc.sessions.GetPlayer("u_combined")
	require.True(t, ok)
	sess.Class = "nerd"
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "arc_flash", Expended: false}},
	}

	evt, err := svc.handleLoadout("u_combined", &gamev1.LoadoutRequest{Arg: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	lv := extractLoadoutView(evt)
	require.NotNil(t, lv, "no-arg handleLoadout must return a sentinel-encoded LoadoutView")
	assert.Len(t, lv.Presets, 2, "default LoadoutSet has 2 presets")
	assert.Equal(t, int32(0), lv.ActiveIndex, "active index must be 0 by default")
}

// newLoadoutServer creates a GameServiceServer with a default loadout-aware session.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a server with one player session that has LoadoutSet and Equipment initialized.
func newLoadoutServer(t *testing.T, uid string) *GameServiceServer {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "test_user",
		CharName:    "TestChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       0,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	sess.Equipment = inventory.NewEquipment()
	return svc
}

// TestHandleLoadout_NoArg_HasPresetsInView verifies that handleLoadout with no arg returns
// a LoadoutView event whose Presets slice is non-empty.
//
// Precondition: Player session has a non-nil LoadoutSet with 2 default presets.
// Postcondition: Returns a ServerEvent carrying a LoadoutView with len(Presets) > 0.
func TestHandleLoadout_NoArg_HasPresetsInView(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleLoadout("u1", &gamev1.LoadoutRequest{Arg: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	lv := extractLoadoutView(evt)
	require.NotNil(t, lv, "no-arg handleLoadout must return a sentinel-encoded LoadoutView")
	assert.NotEmpty(t, lv.Presets, "LoadoutView must contain at least one preset")
}

// TestHandleUnequip_UnknownSlot verifies that handleUnequip with an invalid slot name
// returns a message containing "unknown slot".
//
// Precondition: Player session has a non-nil LoadoutSet and Equipment.
// Postcondition: Returns a ServerEvent whose MessageEvent.Content contains "unknown slot".
func TestHandleUnequip_UnknownSlot(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleUnequip("u1", &gamev1.UnequipRequest{Slot: "bad_slot"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Unknown slot")
}

// TestHandleEquipment_ReturnsDisplay verifies that handleEquipment returns a message
// containing "Weapons", indicating the full equipment display was rendered.
//
// Precondition: Player session has non-nil LoadoutSet and Equipment.
// Postcondition: Returns a ServerEvent whose MessageEvent.Content contains "Weapons".
func TestHandleEquipment_ReturnsDisplay(t *testing.T) {
	svc := newLoadoutServer(t, "u1")

	evt, err := svc.handleEquipment("u1", &gamev1.EquipmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Weapons")
}

// TestPropertyHandleEquipment_ValidSessionAlwaysReturnsEvent is a property test
// verifying that any uid mapped to a valid session always produces a non-nil ServerEvent.
//
// Precondition: uid must be registered in the session manager with valid LoadoutSet and Equipment.
// Postcondition: handleEquipment always returns a non-nil event and nil error for valid sessions.
func TestPropertyHandleEquipment_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_u_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "u",
			CharName:    "Char",
			CharacterID: 1,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       0,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		evt, err := svc.handleEquipment(uid, &gamev1.EquipmentRequest{})
		if err != nil {
			rt.Fatalf("handleEquipment: %v", err)
		}
		if evt == nil {
			rt.Fatal("expected non-nil ServerEvent for valid session")
		}
	})
}

// TestHandleLoadout_PlayerNotFound verifies that handleLoadout returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleLoadout_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleLoadout("missing", &gamev1.LoadoutRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestHandleUnequip_PlayerNotFound verifies that handleUnequip returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleUnequip_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleUnequip("missing", &gamev1.UnequipRequest{Slot: "main"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestHandleEquipment_PlayerNotFound verifies that handleEquipment returns an error event
// when the uid does not match any session.
//
// Precondition: uid is not registered.
// Postcondition: Returns an ErrorEvent with message "player not found".
func TestHandleEquipment_PlayerNotFound(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	evt, err := svc.handleEquipment("missing", &gamev1.EquipmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "player not found")
}

// TestHandleEquip_UpdatesSessionLoadout is a regression test verifying that calling
// handleEquip actually updates sess.LoadoutSet (the source of truth for the equipment display).
//
// Precondition: A weapon item is in the player's backpack; sess.LoadoutSet is initialized.
// Postcondition: After handleEquip, sess.LoadoutSet.ActivePreset().MainHand is non-nil.
func TestHandleEquip_UpdatesSessionLoadout(t *testing.T) {
	// Build a registry with a real weapon item AND its weapon definition.
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterWeapon(&inventory.WeaponDef{
		ID:                  "test_cleaver",
		Name:                "Butcher's Cleaver",
		Kind:                inventory.WeaponKindOneHanded,
		DamageDice:          "1d6",
		DamageType:          "slashing",
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}))
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID:        "test_cleaver",
		Name:      "Butcher's Cleaver",
		Kind:      "weapon",
		WeaponRef: "test_cleaver",
		MaxStack:  1,
	}))

	// Build the server with the registry.
	svc := newSummonItemService(t, nil, reg)

	// Add a session with LoadoutSet and a backpack containing the cleaver.
	uid := "equip_test_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: "TestEquipper", CharacterID: 1,
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Abilities: character.AbilityScores{}, Role: "player",
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.LoadoutSet = inventory.NewLoadoutSet()
	_, addErr := sess.Backpack.Add("test_cleaver", 1, reg)
	require.NoError(t, addErr)

	// Equip the cleaver.
	resp, err := svc.handleEquip(uid, &gamev1.EquipRequest{WeaponId: "test_cleaver", Slot: "main"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The equipment display must now show the cleaver — not empty.
	equipResp, err := svc.handleEquipment(uid, &gamev1.EquipmentRequest{})
	require.NoError(t, err)
	msg := equipResp.GetMessage()
	require.NotNil(t, msg, "expected a message event from handleEquipment")
	assert.Contains(t, msg.Content, "Butcher's Cleaver",
		"equipment display should show the equipped cleaver, not empty hands")
}

// TestPropertyHandleLoadout_ValidSessionAlwaysReturnsEvent asserts that any valid
// session uid always receives a non-nil ServerEvent from handleLoadout, regardless of arg.
// Precondition: uid maps to a valid player session.
// Postcondition: event is non-nil; empty arg yields LoadoutView, non-empty arg yields MessageEvent.
func TestPropertyHandleLoadout_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_loadout_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "u",
			CharName:    "Char",
			CharacterID: 1,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       0,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		arg := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter)).Draw(rt, "arg")
		evt, err := svc.handleLoadout(uid, &gamev1.LoadoutRequest{Arg: arg})
		require.NoError(rt, err)
		require.NotNil(rt, evt)
		if arg == "" {
			require.NotNil(rt, extractLoadoutView(evt), "empty arg must return sentinel-encoded LoadoutView")
		} else {
			require.NotNil(rt, evt.GetMessage(), "non-empty arg must return MessageEvent")
		}
	})
}

// TestPropertyHandleUnequip_ValidSessionAlwaysReturnsEvent asserts that any valid
// session uid always receives a non-nil ServerEvent from handleUnequip, regardless of slot.
// Precondition: uid maps to a valid player session.
// Postcondition: event is non-nil and has a message payload.
func TestPropertyHandleUnequip_ValidSessionAlwaysReturnsEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := fmt.Sprintf("prop_unequip_%d", rapid.IntRange(0, 99999).Draw(rt, "uid"))
		svc := testServiceWithAdmin(t, nil)
		_, addErr := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "u",
			CharName:    "Char",
			CharacterID: 1,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       0,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session must exist after AddPlayer")
		}
		sess.LoadoutSet = inventory.NewLoadoutSet()
		sess.Equipment = inventory.NewEquipment()

		slot := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter)).Draw(rt, "slot")
		evt, err := svc.handleUnequip(uid, &gamev1.UnequipRequest{Slot: slot})
		require.NoError(t, err)
		require.NotNil(t, evt)
		require.NotNil(t, evt.GetMessage())
	})
}

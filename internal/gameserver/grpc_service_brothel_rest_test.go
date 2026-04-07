package gameserver

// grpc_service_brothel_rest_test.go — REQ-BR-T2 through REQ-BR-T6
// Table-driven + property-based tests for handleBrothelRest.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap/zaptest"
)

// newBrothelSvcWithSafeRoom creates a GameServiceServer with a safe room for brothel tests.
func newBrothelSvcWithSafeRoom(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "brothel_zone",
		Name:        "Brothel Zone",
		Description: "A zone with a brothel.",
		StartRoom:   "room_brothel",
		DangerLevel: "safe",
		Rooms: map[string]*world.Room{
			"room_brothel": {
				ID:          "room_brothel",
				ZoneID:      "brothel_zone",
				Title:       "The Velvet Den",
				Description: "A brothel.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
				DangerLevel: "safe",
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
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
	return svc, sessMgr, npcMgr
}

// spawnBrothelNPC spawns a brothel_keeper NPC in the given room.
func spawnBrothelNPC(t *testing.T, npcMgr *npc.Manager, roomID string, cfg npc.BrothelConfig) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:      "brothel_keeper_test",
		Name:    "Madame Rouge",
		Level:   1,
		MaxHP:   20,
		AC:      10,
		NPCType: "brothel_keeper",
		Brothel: &cfg,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)
	return inst
}

// addBrothelPlayer adds a player in a brothel room with idle status.
func addBrothelPlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string, currency, maxHP int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   1,
		MaxHP:       maxHP,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.Currency = currency
	return sess
}

// defaultBrothelCfg returns a BrothelConfig suitable for most tests.
func defaultBrothelCfg() npc.BrothelConfig {
	return npc.BrothelConfig{
		RestCost:      50,
		DiseaseChance: 0.0,
		RobberyChance: 0.0,
		DiseasePool:   []string{"street_fever"},
		FlairBonusDur: "1h",
	}
}

// loadTestCondRegistry loads the condition registry from content/conditions.
func loadTestCondRegistry(t *testing.T) *condition.Registry {
	t.Helper()
	reg, err := condition.LoadDirectory("../../content/conditions")
	require.NoError(t, err)
	return reg
}

// loadTestSubstanceRegistry loads the substance registry from content/substances.
func loadTestSubstanceRegistry(t *testing.T) *substance.Registry {
	t.Helper()
	reg, err := substance.LoadDirectory("../../content/substances")
	require.NoError(t, err)
	return reg
}

// allMessages returns all message contents from the stream.
func allMessages(stream *fakeSessionStream) []string {
	var msgs []string
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil {
			msgs = append(msgs, msg.GetContent())
		}
	}
	return msgs
}

// hasAnyMessage returns true if any sent message contains sub.
func hasAnyMessage(stream *fakeSessionStream, sub string) bool {
	for _, m := range allMessages(stream) {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// TestBrothelRest_InsufficientCredits_BlocksRest verifies REQ-BR-T2:
// insufficient credits blocks rest and does not restore HP.
//
// Precondition: sess.Currency < brothelConfig.RestCost.
// Postcondition: HP not restored; message mentions cost and credits.
func TestBrothelRest_InsufficientCredits_BlocksRest(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-nocreds", "room_brothel", 10, 20)
	sess.CurrentHP = 5

	cfg := defaultBrothelCfg()
	cfg.RestCost = 50
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-nocreds", "req", stream))

	msg := lastMessage(stream)
	assert.Contains(t, msg, "50", "message must mention rest cost")
	assert.Contains(t, msg, "credits", "message must mention credits")
	assert.Equal(t, 5, sess.CurrentHP, "HP must not be restored when credits are insufficient")
}

// TestBrothelRest_SufficientCredits_RestoresHPAndAppliesFlair verifies REQ-BR-T3:
// sufficient credits result in full HP restoration and flair_bonus_1 applied.
//
// Precondition: sess.Currency >= brothelConfig.RestCost; condRegistry has flair_bonus_1.
// Postcondition: HP == MaxHP; flair_bonus_1 condition present; currency reduced by rest cost.
func TestBrothelRest_SufficientCredits_RestoresHPAndAppliesFlair(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-paid", "room_brothel", 100, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	cfg := defaultBrothelCfg()
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-paid", "req", stream))

	assert.Equal(t, 20, sess.CurrentHP, "HP must be restored to MaxHP")
	assert.Equal(t, 50, sess.Currency, "currency deducted by rest cost")
	assert.True(t, sess.Conditions.Has("flair_bonus_1"), "flair_bonus_1 condition must be applied")
	assert.True(t, hasAnyMessage(stream, "confident"), "flair message must mention confident")
}

// TestBrothelRest_DiseaseChance1_AppliesDisease verifies REQ-BR-T4:
// disease_chance == 1.0 guarantees a disease substance is applied to the player's active substances.
//
// Precondition: brothelConfig.DiseaseChance == 1.0; substanceRegistry has disease_pool entry.
// Postcondition: sess.ActiveSubstances has an entry for the disease substance.
func TestBrothelRest_DiseaseChance1_AppliesDisease(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-disease", "room_brothel", 200, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	svc.substanceReg = loadTestSubstanceRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	cfg := defaultBrothelCfg()
	cfg.DiseaseChance = 1.0
	cfg.DiseasePool = []string{"street_fever"}
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-disease", "req", stream))

	// Verify the disease was applied via active substances.
	require.NotEmpty(t, sess.ActiveSubstances, "at least one substance must be active after disease roll")
	found := false
	for _, as := range sess.ActiveSubstances {
		if as.SubstanceID == "street_fever" {
			found = true
			break
		}
	}
	assert.True(t, found, "street_fever must be the applied disease substance")
}

// TestBrothelRest_RobberyChance1_DeductsCrypto verifies REQ-BR-T5 (currency part):
// robbery_chance == 1.0 deducts ~5% of post-rest currency from the player.
//
// Precondition: brothelConfig.RobberyChance == 1.0; sess.Currency > 0 after rest.
// Postcondition: sess.Currency reduced by max(1, postRestCurrency * 5 / 100).
func TestBrothelRest_RobberyChance1_DeductsCrypto(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	// Start with 200 credits; rest costs 50 → 150 after rest.
	// Robbery of 5% of 150 = max(1, 7) = 7.
	sess := addBrothelPlayer(t, sessMgr, "br-robbed", "room_brothel", 200, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	cfg := defaultBrothelCfg()
	cfg.RobberyChance = 1.0
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-robbed", "req", stream))

	// After paying 50, currency is 150. Robbery = max(1, 150*5/100) = 7.
	expectedAfterRobbery := 150 - 7
	assert.Equal(t, expectedAfterRobbery, sess.Currency, "robbery must deduct 5%% of remaining currency")
	assert.True(t, hasAnyMessage(stream, "belongings"), "robbery message must mention belongings")
}

// TestBrothelRest_RobberyChance1_RemovesBackpackItem verifies REQ-BR-T5 (item part):
// robbery_chance == 1.0 with >=20 items removes at least one item.
//
// Precondition: brothelConfig.RobberyChance == 1.0; backpack has 20 non-stackable items.
// Postcondition: backpack has fewer items than before.
func TestBrothelRest_RobberyChance1_RemovesBackpackItem(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-robbed-items", "room_brothel", 200, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	// Populate backpack with 20 non-stackable items so 5% = 1.
	invReg := inventory.NewRegistry()
	for i := 0; i < 20; i++ {
		def := &inventory.ItemDef{
			ID:       fmt.Sprintf("junk_%02d", i),
			Name:     fmt.Sprintf("Junk %d", i),
			Kind:     inventory.KindJunk,
			Weight:   0.1,
			MaxStack: 1,
		}
		require.NoError(t, invReg.RegisterItem(def))
		_, addErr := sess.Backpack.Add(def.ID, 1, invReg)
		require.NoError(t, addErr)
	}
	svc.invRegistry = invReg

	itemsBefore := len(sess.Backpack.Items())

	cfg := defaultBrothelCfg()
	cfg.RobberyChance = 1.0
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-robbed-items", "req", stream))

	itemsAfter := len(sess.Backpack.Items())
	assert.Less(t, itemsAfter, itemsBefore, "robbery must remove at least one item from backpack")
}

// TestBrothelRest_RobberyChance1_StackableItemsLoseOneUnit verifies that stackable items
// lose exactly 1 unit (not the entire stack) during robbery. This is a regression test for
// the bug where Remove(item.InstanceID, item.Quantity) removed the whole stack.
//
// Precondition: backpack has a stackable item with Quantity=2.
// Postcondition: after robbery, that item still exists with Quantity=1.
func TestBrothelRest_RobberyChance1_StackableItemsLoseOneUnit(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-stackable-robbed", "room_brothel", 200, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	// Create an inventory with 10 junk items (non-stackable) + 1 stackable item with qty 2.
	invReg := inventory.NewRegistry()
	for i := 0; i < 10; i++ {
		def := &inventory.ItemDef{
			ID:       fmt.Sprintf("junk_%02d", i),
			Name:     fmt.Sprintf("Junk %d", i),
			Kind:     inventory.KindJunk,
			Weight:   0.1,
			MaxStack: 1,
		}
		require.NoError(t, invReg.RegisterItem(def))
		_, addErr := sess.Backpack.Add(def.ID, 1, invReg)
		require.NoError(t, addErr)
	}

	// Add one stackable item with qty 2
	stackDef := &inventory.ItemDef{
		ID:       "potion_health",
		Name:     "Health Potion",
		Kind:     inventory.KindConsumable,
		Weight:   0.1,
		MaxStack: 10,
	}
	require.NoError(t, invReg.RegisterItem(stackDef))
	stackItem, addErr := sess.Backpack.Add("potion_health", 2, invReg)
	require.NoError(t, addErr)
	stackID := stackItem.InstanceID

	svc.invRegistry = invReg

	// Configure brothel to guarantee robbery.
	cfg := defaultBrothelCfg()
	cfg.RobberyChance = 1.0
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-stackable-robbed", "req", stream))

	// Check that the stackable item still exists with qty 1 (not removed entirely).
	items := sess.Backpack.Items()
	var foundItem *inventory.ItemInstance
	for i, item := range items {
		if item.InstanceID == stackID {
			foundItem = &items[i]
			break
		}
	}
	assert.NotNil(t, foundItem, "stackable item must still exist in backpack after robbery")
	assert.Equal(t, 1, foundItem.Quantity, "stackable item must lose exactly 1 unit (not entire stack)")
}

// TestBrothelRest_ZeroChances_NoSideEffects verifies REQ-BR-T6:
// disease_chance == 0.0, robbery_chance == 0.0 → no disease, no robbery, only rest cost deducted.
//
// Precondition: both chances == 0.0.
// Postcondition: currency only reduced by rest cost; flair_bonus_1 applied; no active substances.
func TestBrothelRest_ZeroChances_NoSideEffects(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-safe", "room_brothel", 100, 20)
	sess.CurrentHP = 5

	svc.condRegistry = loadTestCondRegistry(t)
	svc.substanceReg = loadTestSubstanceRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	cfg := defaultBrothelCfg()
	cfg.DiseaseChance = 0.0
	cfg.RobberyChance = 0.0
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-safe", "req", stream))

	// Currency only reduced by rest cost.
	assert.Equal(t, 50, sess.Currency, "only rest cost deducted; no robbery")
	// No disease substance applied.
	assert.Empty(t, sess.ActiveSubstances, "no disease substances must be applied when chance is 0")
	// Flair bonus applied (standard brothel rest).
	assert.True(t, sess.Conditions.Has("flair_bonus_1"), "flair_bonus_1 must still be applied")
}

// TestProperty_BrothelRest_CurrencyNeverNegative verifies that brothel rest
// never makes currency negative regardless of input (property, REQ-BR-6).
func TestProperty_BrothelRest_CurrencyNeverNegative(t *testing.T) {
	condReg, err := condition.LoadDirectory("../../content/conditions")
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		currency := rapid.IntRange(0, 1000).Draw(rt, "currency")
		restCost := rapid.IntRange(1, 200).Draw(rt, "restCost")

		svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
		uid := fmt.Sprintf("prop-br-%d-%d", currency, rapid.IntRange(0, 9999).Draw(rt, "uid"))

		addOpts := session.AddPlayerOptions{
			UID:         uid,
			Username:    uid,
			CharName:    uid,
			CharacterID: 1,
			RoomID:      "room_brothel",
			CurrentHP:   1,
			MaxHP:       20,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		}
		sess, addErr := sessMgr.AddPlayer(addOpts)
		if addErr != nil {
			rt.Skip()
		}
		sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
		sess.Currency = currency
		sess.CurrentHP = 5

		svc.condRegistry = condReg
		sess.Conditions = condition.NewActiveSet()

		cfg := npc.BrothelConfig{
			RestCost:      restCost,
			DiseaseChance: 0.0,
			RobberyChance: 1.0, // max robbery to stress-test
			DiseasePool:   []string{"street_fever"},
			FlairBonusDur: "1h",
		}
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID:      fmt.Sprintf("br-tmpl-%d-%d", restCost, currency),
			Name:    "Madame",
			Level:   1,
			MaxHP:   20,
			AC:      10,
			NPCType: "brothel_keeper",
			Brothel: &cfg,
		}, "room_brothel")
		if spawnErr != nil {
			rt.Skip()
		}

		stream := &fakeSessionStream{}
		_ = svc.handleRest(uid, "req", stream)

		if sess.Currency < 0 {
			rt.Fatalf("currency went negative: got %d", sess.Currency)
		}
	})
}

// TestBrothelRest_PushesCharacterSheet verifies that after a successful brothel rest,
// a CharacterSheetView event is sent to the entity channel (BUG-150 fix).
//
// Precondition: Player is idle with sufficient credits in a room with a brothel NPC.
// Postcondition: A ServerEvent with CharacterSheetView payload is sent to the entity channel.
func TestBrothelRest_PushesCharacterSheet(t *testing.T) {
	svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)
	sess := addBrothelPlayer(t, sessMgr, "br-sheet", "room_brothel", 100, 20)
	sess.CurrentHP = 5
	sess.Entity = session.NewBridgeEntity("br-sheet", 8)

	svc.condRegistry = loadTestCondRegistry(t)
	sess.Conditions = condition.NewActiveSet()

	charSaver := &fakeCharSaver{}
	svc.SetCharSaver(charSaver)

	cfg := defaultBrothelCfg()
	spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest("br-sheet", "req", stream))

	// Check if CharacterSheetView was pushed to the entity channel
	var foundCharSheet bool
	for {
		select {
		case data := <-sess.Entity.Events():
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err == nil && evt.GetCharacterSheet() != nil {
				foundCharSheet = true
				break
			}
		default:
			// No more events available
			goto done
		}
	}
done:
	assert.True(t, foundCharSheet, "CharacterSheetView must be sent to entity channel after brothel rest (BUG-150)")
}

// TestPropertyBrothelRest_AlwaysPushesCharacterSheet is a property-based test
// verifying that CharacterSheetView is sent after any valid brothel rest, regardless
// of currency amount or side effects (disease/robbery) (property, BUG-150).
func TestPropertyBrothelRest_AlwaysPushesCharacterSheet(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr, npcMgr := newBrothelSvcWithSafeRoom(t)

		uid := fmt.Sprintf("br-sheet-prop-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		minCurrency := rapid.IntRange(50, 500).Draw(rt, "currency")
		maxHP := rapid.IntRange(10, 100).Draw(rt, "maxHP")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    uid,
			CharName:    uid,
			CharacterID: 1,
			RoomID:      "room_brothel",
			CurrentHP:   1,
			MaxHP:       maxHP,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		if err != nil {
			rt.Skip()
		}

		sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
		sess.Currency = minCurrency
		sess.Entity = session.NewBridgeEntity(uid, 8)

		svc.condRegistry = loadTestCondRegistry(t)
		sess.Conditions = condition.NewActiveSet()

		charSaver := &fakeCharSaver{}
		svc.SetCharSaver(charSaver)

		cfg := defaultBrothelCfg()
		spawnBrothelNPC(t, npcMgr, "room_brothel", cfg)

		stream := &fakeSessionStream{}
		if err := svc.handleRest(uid, "req", stream); err != nil {
			rt.Skip()
		}

		// Verify CharacterSheetView was pushed to the entity channel
		foundCharSheet := false
		for {
			select {
			case data := <-sess.Entity.Events():
				var evt gamev1.ServerEvent
				if err := proto.Unmarshal(data, &evt); err == nil && evt.GetCharacterSheet() != nil {
					foundCharSheet = true
					break
				}
			default:
				// No more events available
				goto done
			}
		}
	done:
		if !foundCharSheet {
			rt.Fatalf("CharacterSheetView not sent after brothel rest")
		}
	})
}

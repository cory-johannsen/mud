package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newNonCombatCombatHandler builds a CombatHandler and session.Manager for non-combat NPC tests.
//
// Precondition: t is non-nil; npcMgr is non-nil.
// Postcondition: Returns non-nil CombatHandler and session.Manager.
func newNonCombatCombatHandler(t *testing.T, npcMgr *npc.Manager) (*CombatHandler, *session.Manager) {
	t.Helper()
	sessMgr := session.NewManager()
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, zaptest.NewLogger(t))
	condReg := makeTestConditionRegistry()
	h := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return h, sessMgr
}

// spawnNonCombatNPCs spawns a combat NPC ("Bandit") and a merchant NPC ("Shopkeeper")
// into the given roomID.
//
// Precondition: npcMgr is non-nil; roomID is non-empty.
// Postcondition: Both NPCs are present in roomID.
func spawnNonCombatNPCs(t *testing.T, npcMgr *npc.Manager, roomID string) {
	t.Helper()
	combatTmpl := &npc.Template{
		ID:      "bandit-nc",
		Name:    "Bandit",
		Level:   1,
		MaxHP:   20,
		AC:      12,
		NPCType: "combat",
	}
	_, err := npcMgr.Spawn(combatTmpl, roomID)
	require.NoError(t, err)

	merchantTmpl := &npc.Template{
		ID:      "shopkeeper-nc",
		Name:    "Shopkeeper",
		Level:   1,
		MaxHP:   10,
		AC:      10,
		NPCType: "merchant",
		Merchant: &npc.MerchantConfig{
			MerchantType: "consumables",
			ReplenishRate: npc.ReplenishConfig{
				MinHours: 1,
				MaxHours: 4,
			},
		},
	}
	_, err = npcMgr.Spawn(merchantTmpl, roomID)
	require.NoError(t, err)
}

// TestAttack_BlocksNonCombatNPC verifies that attacking a merchant NPC returns
// a "not a valid combat target" error (REQ-NPC-4).
//
// Precondition: Player is in "nc_room1"; "Shopkeeper" (merchant) is in "nc_room1".
// Postcondition: Attack returns an error containing "not a valid combat target".
func TestAttack_BlocksNonCombatNPC(t *testing.T) {
	npcMgr := npc.NewManager()
	spawnNonCombatNPCs(t, npcMgr, "nc_room1")
	h, sessMgr := newNonCombatCombatHandler(t, npcMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_nc_block",
		Username:  "Player",
		CharName:  "Player",
		Role:      "player",
		RoomID:    "nc_room1",
		CurrentHP: 20,
		MaxHP:     20,
	})
	require.NoError(t, err)

	_, attackErr := h.Attack("u_nc_block", "Shopkeeper")
	require.Error(t, attackErr)
	assert.Contains(t, attackErr.Error(), "not a valid combat target")
}

// TestAttack_AllowsCombatNPC verifies that attacking a combat NPC succeeds (REQ-NPC-4).
//
// Precondition: Player is in "nc_room1"; "Bandit" (combat NPC) is in "nc_room1".
// Postcondition: Attack returns no error and at least one CombatEvent.
func TestAttack_AllowsCombatNPC(t *testing.T) {
	npcMgr := npc.NewManager()
	spawnNonCombatNPCs(t, npcMgr, "nc_room1")
	h, sessMgr := newNonCombatCombatHandler(t, npcMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_nc_allow",
		Username:  "Fighter",
		CharName:  "Fighter",
		Role:      "player",
		RoomID:    "nc_room1",
		CurrentHP: 20,
		MaxHP:     20,
	})
	require.NoError(t, err)

	events, attackErr := h.Attack("u_nc_allow", "Bandit")
	require.NoError(t, attackErr)
	assert.NotEmpty(t, events)

	h.cancelTimer("nc_room1")
}

// TestCombatStart_MerchantCowers verifies that a merchant NPC sets Cowering=true
// when combat starts in its room (REQ-NPC-5).
//
// Precondition: Merchant and Bandit are both in "nc_room1"; player attacks Bandit.
// Postcondition: Merchant is still in "nc_room1" with Cowering==true.
func TestCombatStart_MerchantCowers(t *testing.T) {
	npcMgr := npc.NewManager()
	spawnNonCombatNPCs(t, npcMgr, "nc_room1")
	h, sessMgr := newNonCombatCombatHandler(t, npcMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_cower",
		Username:  "Alice",
		CharName:  "Alice",
		Role:      "player",
		RoomID:    "nc_room1",
		CurrentHP: 20,
		MaxHP:     20,
	})
	require.NoError(t, err)

	_, err = h.Attack("u_cower", "Bandit")
	require.NoError(t, err, "attack must succeed to start combat")

	var merchant *npc.Instance
	for _, n := range npcMgr.InstancesInRoom("nc_room1") {
		if n.NPCType == "merchant" {
			merchant = n
		}
	}
	require.NotNil(t, merchant, "merchant must still be in nc_room1 (cower behavior)")
	assert.True(t, merchant.Cowering, "merchant must be cowering after combat starts")

	h.cancelTimer("nc_room1")
}

// TestDefaultCombatResponse_TypeDefaults verifies per-type default responses
// when personality is empty.
func TestDefaultCombatResponse_TypeDefaults(t *testing.T) {
	cases := []struct {
		npcType  string
		expected string
	}{
		{"merchant", "cower"},
		{"quest_giver", "cower"},
		{"job_trainer", "cower"},
		{"healer", "flee"},
		{"banker", "flee"},
		{"crafter", "flee"},
		{"guard", "engage"},
		{"hireling", "engage"},
	}
	for _, tc := range cases {
		t.Run(tc.npcType, func(t *testing.T) {
			got := defaultCombatResponse(tc.npcType, "")
			assert.Equal(t, tc.expected, got,
				"npc_type %q with empty personality must default to %q", tc.npcType, tc.expected)
		})
	}
}

// TestDefaultCombatResponse_PersonalityOverrides verifies that personality
// "cowardly" and "brave" override the type default.
func TestDefaultCombatResponse_PersonalityOverrides(t *testing.T) {
	assert.Equal(t, "flee", defaultCombatResponse("merchant", "cowardly"),
		"cowardly personality always flees regardless of type default")
	assert.Equal(t, "cower", defaultCombatResponse("healer", "brave"),
		"brave personality always cowers regardless of type default")
	assert.Equal(t, "flee", defaultCombatResponse("healer", "neutral"),
		"neutral personality falls through to healer type default (flee)")
	assert.Equal(t, "cower", defaultCombatResponse("merchant", "opportunistic"),
		"opportunistic personality falls through to merchant type default (cower)")
}

// TestProperty_DefaultCombatResponse_NeverPanics verifies that
// defaultCombatResponse never panics and always returns a valid response for
// any arbitrary npcType/personality combination.
func TestProperty_DefaultCombatResponse_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		npcType := rapid.String().Draw(rt, "npc_type")
		personality := rapid.String().Draw(rt, "personality")
		result := defaultCombatResponse(npcType, personality)
		validResponses := map[string]bool{"flee": true, "cower": true, "engage": true}
		if !validResponses[result] {
			rt.Fatalf("defaultCombatResponse(%q, %q) returned unexpected %q", npcType, personality, result)
		}
	})
}

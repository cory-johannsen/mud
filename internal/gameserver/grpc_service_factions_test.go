package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newFactionTestServer builds a GameServiceServer with a faction registry and config,
// and a player in room_a.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a configured server and the player UID.
func newFactionTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	// Inject faction registry and config directly.
	reg := faction.FactionRegistry{
		"red_sky": &faction.FactionDef{
			ID:     "red_sky",
			Name:   "Red Sky Collective",
			ZoneID: "zone_a",
			Tiers: []faction.FactionTier{
				{Label: "Associate", MinRep: 0, PriceDiscount: 0},
				{Label: "Trusted", MinRep: 100, PriceDiscount: 0.05},
				{Label: "Inner Circle", MinRep: 300, PriceDiscount: 0.10},
				{Label: "Vanguard", MinRep: 700, PriceDiscount: 0.20},
			},
		},
	}
	svc.factionRegistry = &reg
	svc.factionSvc = faction.NewServiceWithRepo(reg, nil)
	svc.factionConfig = &faction.FactionConfig{
		RepPerFixerService: 50,
		RepChangeCosts:     map[int]int{1: 100, 2: 250, 3: 500, 4: 1000},
	}

	uid := "faction_u1"
	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "faction_user",
		CharName:  "FactionChar",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.FactionID = "red_sky"
	sess.FactionRep = map[string]int{"red_sky": 50}
	sess.Currency = 2000

	return svc, uid
}

// TestHandleFaction_NoFactionID verifies that a player with no faction affiliation
// receives an appropriate message.
//
// REQ-FA-37
func TestHandleFaction_NoFactionID(t *testing.T) {
	svc, uid := newFactionTestServer(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FactionID = ""
	sess.FactionRep = map[string]int{}

	ev, err := svc.handleFaction(uid, &gamev1.FactionRequest{})
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Contains(t, ev.GetMessage().Content, "no faction")
}

// TestHandleFaction_WithFaction verifies that a player with a faction affiliation
// receives faction details including tier and rep.
//
// REQ-FA-37
func TestHandleFaction_WithFaction(t *testing.T) {
	svc, uid := newFactionTestServer(t)

	ev, err := svc.handleFaction(uid, &gamev1.FactionRequest{})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msg := ev.GetMessage().Content
	assert.Contains(t, msg, "Red Sky Collective")
	assert.Contains(t, msg, "Associate")
	assert.Contains(t, msg, "50")
}

// TestHandleFactionInfo_UnknownFaction verifies that querying an unknown faction ID
// returns an appropriate error message.
//
// REQ-FA-38
func TestHandleFactionInfo_UnknownFaction(t *testing.T) {
	svc, uid := newFactionTestServer(t)

	ev, err := svc.handleFactionInfo(uid, &gamev1.FactionInfoRequest{FactionId: "unknown_faction"})
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Contains(t, ev.GetMessage().Content, "Unknown faction")
}

// TestHandleFactionInfo_KnownFaction verifies that querying a known faction returns
// tier information.
//
// REQ-FA-38
func TestHandleFactionInfo_KnownFaction(t *testing.T) {
	svc, uid := newFactionTestServer(t)

	ev, err := svc.handleFactionInfo(uid, &gamev1.FactionInfoRequest{FactionId: "red_sky"})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msg := ev.GetMessage().Content
	assert.Contains(t, msg, "Red Sky Collective")
	assert.Contains(t, msg, "Associate")
	assert.Contains(t, msg, "Trusted")
}

// TestHandleFactionStanding_EmptyRep verifies that a player with no faction standing
// receives a "no standing" message.
//
// REQ-FA-39
func TestHandleFactionStanding_EmptyRep(t *testing.T) {
	svc, uid := newFactionTestServer(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FactionRep = map[string]int{}

	ev, err := svc.handleFactionStanding(uid, &gamev1.FactionStandingRequest{})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msg := ev.GetMessage().Content
	assert.True(t,
		strings.Contains(msg, "no standing") || strings.Contains(msg, "Standing"),
		"expected standing header or no-standing message, got %q", msg)
}

// TestHandleFactionStanding_WithRep verifies that a player with faction rep sees
// their standing listed.
//
// REQ-FA-39
func TestHandleFactionStanding_WithRep(t *testing.T) {
	svc, uid := newFactionTestServer(t)

	ev, err := svc.handleFactionStanding(uid, &gamev1.FactionStandingRequest{})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msg := ev.GetMessage().Content
	assert.Contains(t, msg, "Red Sky Collective")
	assert.Contains(t, msg, "50")
}

// TestHandleChangeRep_NoFixer verifies that change_rep fails gracefully when no
// Fixer NPC is present in the room.
//
// REQ-FA-34
func TestHandleChangeRep_NoFixer(t *testing.T) {
	svc, uid := newFactionTestServer(t)

	ev, err := svc.handleChangeRep(uid, &gamev1.ChangeRepRequest{FactionId: "red_sky"})
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Contains(t, ev.GetMessage().Content, "no Fixer")
}

// TestHandleChangeRep_InsufficientFunds verifies that change_rep fails with a cost
// message when the player cannot afford it.
//
// REQ-FA-35
func TestHandleChangeRep_InsufficientFunds(t *testing.T) {
	svc, uid := newFactionTestServer(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 0

	// Spawn a fixer NPC.
	tmpl := &npc.Template{
		ID:      "test_fixer_fa",
		Name:    "Zara",
		NPCType: "fixer",
		Level:   3,
		MaxHP:   20,
		AC:      12,
		Fixer: &npc.FixerConfig{
			BaseCosts:      map[int]int{1: 100, 2: 250, 3: 500, 4: 1000},
			NPCVariance:    1.0,
			MaxWantedLevel: 4,
		},
	}
	_, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	ev, err := svc.handleChangeRep(uid, &gamev1.ChangeRepRequest{FactionId: "red_sky"})
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Contains(t, ev.GetMessage().Content, "credits")
}

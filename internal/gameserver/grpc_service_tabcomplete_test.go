package gameserver

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// buildTabCompleteServer creates a GameServiceServer with optional feat and room equipment
// configuration for handleTabComplete tests.
//
// Precondition: t must be non-nil; feats and equipItems may be nil.
// Postcondition: Returns a non-nil *GameServiceServer with a single player session in room_a.
func buildTabCompleteServer(
	t *testing.T,
	feats []*ruleset.Feat,
	equipItems []world.RoomEquipmentConfig,
) (*GameServiceServer, string) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)

	var featRegistry *ruleset.FeatRegistry
	var featsRepo CharacterFeatsGetter
	if len(feats) > 0 {
		featRegistry = ruleset.NewFeatRegistry(feats)
		ids := make([]string, len(feats))
		for i, f := range feats {
			ids[i] = f.ID
		}
		featsRepo = &stubFeatsRepo{data: map[int64][]string{1: ids}}
	}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		worldHandler,
		chatHandler,
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		feats, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	if len(equipItems) > 0 {
		mgr := inventory.NewRoomEquipmentManager()
		mgr.InitRoom("room_a", equipItems)
		svc.roomEquipMgr = mgr
	}

	uid := "tc_u1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "tabcomplete_user",
		CharName:    "TCChar",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleTabComplete_EmptyPrefix_ReturnsAllCommands verifies that an empty prefix
// returns completions for all non-hidden command names and aliases.
//
// Precondition: prefix is "".
// Postcondition: Returns sorted, non-empty completions list; hidden commands excluded.
func TestHandleTabComplete_EmptyPrefix_ReturnsAllCommands(t *testing.T) {
	svc, uid := buildTabCompleteServer(t, nil, nil)

	evt, err := svc.handleTabComplete(uid, "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetTabComplete()
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Completions)

	// Completions must be sorted.
	assert.True(t, sort.StringsAreSorted(resp.Completions), "completions must be sorted")

	// Known visible commands must appear.
	assert.Contains(t, resp.Completions, "look")
	assert.Contains(t, resp.Completions, "north")
	assert.Contains(t, resp.Completions, "use")

	// Hidden commands must not appear.
	for _, c := range resp.Completions {
		assert.NotEqual(t, command.HandlerTabComplete, c)
		assert.NotEqual(t, "archetype_selection", c)
	}
}

// TestHandleTabComplete_SingleWordPrefix_FiltersCommands verifies that a single-word
// prefix filters command names and aliases alphabetically.
//
// Precondition: prefix is "lo".
// Postcondition: Completions contain only names/aliases starting with "lo".
func TestHandleTabComplete_SingleWordPrefix_FiltersCommands(t *testing.T) {
	svc, uid := buildTabCompleteServer(t, nil, nil)

	evt, err := svc.handleTabComplete(uid, "lo")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetTabComplete()
	require.NotNil(t, resp)

	for _, c := range resp.Completions {
		assert.True(t, strings.HasPrefix(c, "lo"), "expected prefix 'lo', got %q", c)
	}

	assert.Contains(t, resp.Completions, "look")
	assert.Contains(t, resp.Completions, "loadout")
	assert.True(t, sort.StringsAreSorted(resp.Completions))
}

// TestHandleTabComplete_UsePrefix_ReturnsFeatNames verifies that "use med" returns
// feat names starting with "med" (case-insensitive).
//
// Precondition: Player has an active feat named "Medpatch" (ID "medpatch").
// Postcondition: Completions contain "medpatch"; sorted.
func TestHandleTabComplete_UsePrefix_ReturnsFeatNames(t *testing.T) {
	feat := &ruleset.Feat{
		ID:     "medpatch",
		Name:   "Medpatch",
		Active: true,
	}
	svc, uid := buildTabCompleteServer(t, []*ruleset.Feat{feat}, nil)

	evt, err := svc.handleTabComplete(uid, "use med")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetTabComplete()
	require.NotNil(t, resp)
	assert.Contains(t, resp.Completions, "medpatch")
	assert.True(t, sort.StringsAreSorted(resp.Completions))
}

// TestHandleTabComplete_UsePrefix_ReturnsEquipmentDescriptions verifies that
// "use cont" returns room equipment whose description starts with "cont".
//
// Precondition: Room contains equipment with Description "Control Panel".
// Postcondition: Completions contain "control panel" (lowercased); sorted.
func TestHandleTabComplete_UsePrefix_ReturnsEquipmentDescriptions(t *testing.T) {
	equip := []world.RoomEquipmentConfig{
		{ItemID: "ctrl_panel", Description: "Control Panel", MaxCount: 1, Immovable: true},
	}
	svc, uid := buildTabCompleteServer(t, nil, equip)

	evt, err := svc.handleTabComplete(uid, "use cont")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetTabComplete()
	require.NotNil(t, resp)
	assert.Contains(t, resp.Completions, "control panel")
	assert.True(t, sort.StringsAreSorted(resp.Completions))
}

// TestHandleTabComplete_SortedAndDeduped verifies that a completion appearing in both
// feat names and equipment descriptions appears exactly once and results are sorted.
//
// Precondition: Player has feat ID "stimpack"; room contains equipment Description "Stimpack".
// Postcondition: "stimpack" appears exactly once in completions; sorted.
func TestHandleTabComplete_SortedAndDeduped(t *testing.T) {
	feat := &ruleset.Feat{ID: "stimpack", Name: "Stimpack", Active: true}
	equip := []world.RoomEquipmentConfig{
		{ItemID: "stim_item", Description: "Stimpack", MaxCount: 1, Immovable: true},
	}
	svc, uid := buildTabCompleteServer(t, []*ruleset.Feat{feat}, equip)

	evt, err := svc.handleTabComplete(uid, "use s")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetTabComplete()
	require.NotNil(t, resp)

	count := 0
	for _, c := range resp.Completions {
		if c == "stimpack" {
			count++
		}
	}
	assert.Equal(t, 1, count, "stimpack must appear exactly once")
	assert.True(t, sort.StringsAreSorted(resp.Completions))
}

// TestHandleTabComplete_Property_CompletionsAlwaysSorted verifies via property testing
// that completions returned by handleTabComplete are always lexicographically sorted.
//
// Precondition: any string prefix.
// Postcondition: resp.Completions is always sorted.
func TestHandleTabComplete_Property_CompletionsAlwaysSorted(t *testing.T) {
	feat := &ruleset.Feat{ID: "agility_boost", Name: "Agility Boost", Active: true}
	equip := []world.RoomEquipmentConfig{
		{ItemID: "locker", Description: "Metal Locker", MaxCount: 1, Immovable: true},
	}
	svc, uid := buildTabCompleteServer(t, []*ruleset.Feat{feat}, equip)

	rapid.Check(t, func(rt *rapid.T) {
		prefix := rapid.StringMatching(`[a-z ]{0,20}`).Draw(rt, "prefix")
		evt, err := svc.handleTabComplete(uid, prefix)
		assert.NoError(t, err)
		if evt == nil {
			return
		}
		resp := evt.GetTabComplete()
		if resp == nil {
			return
		}
		assert.True(t, sort.StringsAreSorted(resp.Completions),
			"completions not sorted for prefix %q: %v", prefix, resp.Completions)
	})
}

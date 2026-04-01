package gameserver

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
)

// TestCombatRound_RangeShownAtRoundStart verifies that the round-start broadcast
// includes a range status message showing the player's distance to their NPC target.
//
// Precondition: Active combat exists with player at Position=0 and NPC at Position=25.
// Postcondition: At least one broadcast event narrative from round-start contains
// distance information in feet (e.g. "25 ft") referencing the NPC, case-insensitive.
func TestCombatRound_RangeShownAtRoundStart(t *testing.T) {
	const roomID = "range-test-room"
	const playerUID = "range-player-1"
	const npcName = "RangeGoblin"

	var mu sync.Mutex
	var allBroadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(_ string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		allBroadcasts = append(allBroadcasts, events)
	}

	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()

	_, err := npcMgr.Spawn(&npc.Template{
		ID:        npcName + "-tmpl",
		Name:      npcName,
		Level:     1,
		MaxHP:     100,
		AC:        1, // very low AC so attacks hit; test only checks messaging
		Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       playerUID,
		Username:  playerUID,
		CharName:  "Ranger",
		Role:      "player",
		RoomID:    roomID,
		CurrentHP: 30,
		MaxHP:     30,
		Abilities: character.AbilityScores{},
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, nil)
	h := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		broadcastFn,
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)

	// Start combat — Attack initialises the combat instance with player at
	// Position=0 and NPC at Position=50 (engine defaults).
	_, err = h.Attack(playerUID, npcName)
	require.NoError(t, err)

	// Wait for the round timer to fire and broadcast round results + next round start.
	// testRoundDuration = 200ms; wait 2x to ensure at least one full cycle.
	time.Sleep(2 * testRoundDuration)

	h.cancelTimer(roomID)

	// Collect all narrative strings from every broadcast.
	mu.Lock()
	var narratives []string
	for _, batch := range allBroadcasts {
		for _, ev := range batch {
			if ev.Narrative != "" {
				narratives = append(narratives, ev.Narrative)
			}
		}
	}
	mu.Unlock()

	// At least one narrative must mention the NPC name and distance in feet.
	found := false
	for _, n := range narratives {
		lower := strings.ToLower(n)
		if strings.Contains(lower, "ft") && strings.Contains(lower, strings.ToLower(npcName)) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected a round-start broadcast containing range info (e.g. '25 ft') for %q; got narratives:\n%v",
			npcName, narratives)
	}
}

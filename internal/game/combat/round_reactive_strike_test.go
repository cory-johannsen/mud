package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// fixedSrcRS is a deterministic Source for reactive-strike tests; always returns val.
type fixedSrcRS struct{ val int }

func (f fixedSrcRS) Intn(_ int) int { return f.val }

// makeReactiveStrikeCombat builds a minimal combat for CheckReactiveStrikes tests.
// The player starts at (playerX, playerY); each NPC starts at its provided grid position.
func makeReactiveStrikeCombat(t *testing.T, playerX, playerY int, npcs []struct {
	id   string
	gx   int
	gy   int
	hp   int
}) (*combat.Combat, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	player := &combat.Combatant{
		ID: "player1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1,
		GridX: playerX, GridY: playerY,
	}
	combatants := []*combat.Combatant{player}
	for _, n := range npcs {
		hp := n.hp
		if hp == 0 {
			hp = 20
		}
		combatants = append(combatants, &combat.Combatant{
			ID: n.id, Kind: combat.KindNPC, Name: n.id,
			CurrentHP: hp, MaxHP: 20, AC: 10, Level: 1,
			GridX: n.gx, GridY: n.gy,
		})
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_rs", combatants, reg, nil, "")
	require.NoError(t, err)
	return cbt, player
}

// TestCheckReactiveStrikes covers the core triggering rules.
// Grid distances: 1 square = 5 ft; adjacent means CombatRange <= 5 ft (Chebyshev ≤ 1 square).
func TestCheckReactiveStrikes(t *testing.T) {
	// val=19 → Intn(20)=19 → d20=20; guaranteed hit against any reasonable AC.
	hitSrc := fixedSrcRS{val: 19}
	// val=0 → Intn(20)=0 → d20=1; guaranteed miss against any reasonable AC.
	missSrc := fixedSrcRS{val: 0}

	tests := []struct {
		name       string
		playerNewX int // player GridX AFTER stride
		playerNewY int // player GridY AFTER stride
		playerOldX int // player GridX BEFORE stride
		playerOldY int // player GridY BEFORE stride
		npcGX      int // NPC GridX
		npcGY      int // NPC GridY
		npcHP      int // 0 = alive default
		src        fixedSrcRS
		wantEvents int
		desc       string
	}{
		{
			name:       "adjacent NPC triggers RS when player moves away",
			playerOldX: 1, playerOldY: 0, // player was at (1,0), dist to NPC at (1,0)=0 → adjacent
			playerNewX: 6, playerNewY: 0, // moved to (6,0), dist = 5 squares = 25 ft > 5 ft
			npcGX: 1, npcGY: 0,
			src:        hitSrc,
			wantEvents: 1,
			desc:       "NPC adjacent before stride fires reactive strike",
		},
		{
			name:       "non-adjacent NPC does NOT trigger RS",
			playerOldX: 0, playerOldY: 0, // player was at (0,0), dist to NPC at (5,0)=25ft > 5ft
			playerNewX: 3, playerNewY: 0, // moved to (3,0)
			npcGX: 5, npcGY: 0,
			src:        hitSrc,
			wantEvents: 0,
			desc:       "NPC beyond 5ft before stride: no reactive strike",
		},
		{
			name:       "dead NPC does NOT trigger RS",
			playerOldX: 1, playerOldY: 0,
			playerNewX: 6, playerNewY: 0,
			npcGX: 1, npcGY: 0,
			npcHP:      -1, // IsDead for NPC: CurrentHP <= 0
			src:        hitSrc,
			wantEvents: 0,
			desc:       "Dead NPC never fires reactive strike",
		},
		{
			name:       "adjacent NPC does NOT trigger RS when player moves toward (not away)",
			playerOldX: 2, playerOldY: 0, // player was at (2,0), dist to NPC at (1,0)=5ft → adjacent
			playerNewX: 1, playerNewY: 0, // moved to (1,0) — same cell as NPC → dist=0 ≤ 5
			npcGX: 1, npcGY: 0,
			src:        hitSrc,
			wantEvents: 0,
			desc:       "Player moving toward NPC: no reactive strike even if adjacent",
		},
		{
			name:       "reactive strike rolls miss on low roll",
			playerOldX: 1, playerOldY: 0,
			playerNewX: 6, playerNewY: 0,
			npcGX: 1, npcGY: 0,
			src:        missSrc,
			wantEvents: 1, // event is still generated (miss narrative)
			desc:       "Miss still generates an RS event",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			npcHP := tc.npcHP
			if npcHP == 0 {
				npcHP = 20 // alive
			}
			cbt, player := makeReactiveStrikeCombat(t, tc.playerNewX, tc.playerNewY, []struct {
				id   string
				gx   int
				gy   int
				hp   int
			}{{"npc1", tc.npcGX, tc.npcGY, npcHP}})

			// Simulate: oldX/oldY is where the player WAS before the stride.
			events := combat.CheckReactiveStrikes(cbt, player.ID, tc.playerOldX, tc.playerOldY, tc.src, nil)
			assert.Len(t, events, tc.wantEvents, tc.desc)
			for _, ev := range events {
				assert.Equal(t, combat.ActionAttack, ev.ActionType)
				assert.Equal(t, "npc1", ev.ActorID)
				assert.Equal(t, player.ID, ev.TargetID)
				assert.NotEmpty(t, ev.Narrative)
				assert.Contains(t, ev.Narrative, "reactive strike")
			}
		})
	}
}

// TestCheckReactiveStrikes_SelfNotTriggered ensures the mover's own combatant record
// (if it happens to be in Combatants with the moverID) is never counted as an attacker.
func TestCheckReactiveStrikes_SelfNotTriggered(t *testing.T) {
	hitSrc := fixedSrcRS{val: 19}
	// Only the player is in combat (no NPC) — zero events expected.
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	player := &combat.Combatant{
		ID: "solo", Kind: combat.KindPlayer, Name: "Solo",
		CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1, GridX: 5, GridY: 5,
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_solo", []*combat.Combatant{player}, reg, nil, "")
	require.NoError(t, err)

	events := combat.CheckReactiveStrikes(cbt, player.ID, 0, 0, hitSrc, nil)
	assert.Empty(t, events, "mover cannot trigger a reactive strike against themselves")
}

// TestPropertyReactiveStrike_NeverTriggersFromNonAdjacent verifies that when every NPC
// is beyond 5ft of the player's old grid position, zero reactive strikes fire.
func TestPropertyReactiveStrike_NeverTriggersFromNonAdjacent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate player old position on the grid.
		playerOldX := rapid.IntRange(0, 5).Draw(rt, "playerOldX")
		playerOldY := rapid.IntRange(0, 5).Draw(rt, "playerOldY")
		// Generate NPC position at least 2 squares away (>5ft) from old player position.
		npcOffX := rapid.IntRange(2, 4).Draw(rt, "npcOffX")
		npcOffY := rapid.IntRange(2, 4).Draw(rt, "npcOffY")
		npcGX := playerOldX + npcOffX
		if npcGX > 9 {
			npcGX = playerOldX - npcOffX
		}
		npcGY := playerOldY + npcOffY
		if npcGY > 9 {
			npcGY = playerOldY - npcOffY
		}

		// Player new position: moved 5 squares in X direction.
		playerNewX := playerOldX + 5
		if playerNewX > 9 {
			playerNewX = 9
		}
		playerNewY := playerOldY

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		player := &combat.Combatant{
			ID: "player", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1,
			GridX: playerNewX, GridY: playerNewY,
		}
		npc := &combat.Combatant{
			ID: "npc", Kind: combat.KindNPC, Name: "npc",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
			GridX: npcGX, GridY: npcGY,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_prop", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatal(err)
		}

		hitSrc := fixedSrcRS{val: 19}
		// Verify that NPC is more than 5ft from old position before testing.
		oldMover := combat.Combatant{GridX: playerOldX, GridY: playerOldY}
		distFromOld := combat.CombatRange(*npc, oldMover)
		if distFromOld <= 5 {
			rt.Skip("NPC ended up adjacent to old position; skipping this draw")
		}

		events := combat.CheckReactiveStrikes(cbt, player.ID, playerOldX, playerOldY, hitSrc, nil)
		if len(events) != 0 {
			rt.Fatalf("expected 0 reactive strikes when NPC dist=%d > 5 from oldPos=(%d,%d), got %d",
				distFromOld, playerOldX, playerOldY, len(events))
		}
	})
}

// TestPropertyReactiveStrike_AdjacentAlwaysTriggers verifies that when an NPC is adjacent
// (≤5 ft) before the stride and the player moves away (new dist > 5), at least one
// reactive strike event is always returned.
func TestPropertyReactiveStrike_AdjacentAlwaysTriggers(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate NPC position in grid [0,8]x[0,8] to allow player to be adjacent.
		npcGX := rapid.IntRange(0, 7).Draw(rt, "npcGX")
		npcGY := rapid.IntRange(0, 7).Draw(rt, "npcGY")
		// oldPos: player at same cell as NPC (adjacent, dist=0).
		oldX := npcGX
		oldY := npcGY
		// newPos: player moved 2 squares right and 2 squares down (Chebyshev = 2, dist=10ft > 5ft).
		newX := npcGX + 2
		newY := npcGY + 2
		if newX > 9 {
			newX = 9
		}
		if newY > 9 {
			newY = 9
		}

		// Verify that new position is actually far enough (>5ft from NPC).
		newMover := combat.Combatant{GridX: newX, GridY: newY}
		npcC := combat.Combatant{GridX: npcGX, GridY: npcGY}
		if combat.CombatRange(newMover, npcC) <= 5 {
			rt.Skip("unable to construct valid newPos far enough from NPC; skipping")
		}

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		player := &combat.Combatant{
			ID: "player", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1,
			GridX: newX, GridY: newY,
		}
		npc := &combat.Combatant{
			ID: "npc", Kind: combat.KindNPC, Name: "npc",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
			GridX: npcGX, GridY: npcGY,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_adj", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatal(err)
		}

		// val=19 → d20=20; guaranteed hit against any reasonable AC.
		hitSrc := fixedSrcRS{val: 19}
		events := combat.CheckReactiveStrikes(cbt, player.ID, oldX, oldY, hitSrc, nil)
		if len(events) < 1 {
			rt.Fatalf("expected at least 1 reactive strike when NPC at (%d,%d) is adjacent to oldPos=(%d,%d) and player moved to (%d,%d), got 0",
				npcGX, npcGY, oldX, oldY, newX, newY)
		}
	})
}

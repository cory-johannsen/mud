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
// The player starts at playerPos; each NPC starts at its provided position.
func makeReactiveStrikeCombat(t *testing.T, playerPos int, npcs []struct {
	id  string
	pos int
	hp  int
}) (*combat.Combat, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	player := &combat.Combatant{
		ID: "player1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1,
		Position: playerPos,
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
			Position: n.pos,
		})
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_rs", combatants, reg, nil, "")
	require.NoError(t, err)
	return cbt, player
}

// TestCheckReactiveStrikes covers the core triggering rules.
func TestCheckReactiveStrikes(t *testing.T) {
	// val=19 → Intn(20)=19 → d20=20; guaranteed hit against any reasonable AC.
	hitSrc := fixedSrcRS{val: 19}
	// val=0 → Intn(20)=0 → d20=1; guaranteed miss against any reasonable AC.
	missSrc := fixedSrcRS{val: 0}

	tests := []struct {
		name       string
		playerOld  int // player position BEFORE stride (oldPos)
		playerNew  int // player position AFTER stride (new Position on combatant)
		npcPos     int
		npcHP      int // 0 = alive default
		src        fixedSrcRS
		wantEvents int
		desc       string
	}{
		{
			name:       "adjacent NPC triggers RS when player moves away",
			playerOld:  0,
			playerNew:  25, // moved away from NPC at 5
			npcPos:     5,  // was adjacent (dist=5 ≤ 5)
			src:        hitSrc,
			wantEvents: 1,
			desc:       "NPC adjacent before stride fires reactive strike",
		},
		{
			name:       "non-adjacent NPC does NOT trigger RS",
			playerOld:  0,
			playerNew:  25,
			npcPos:     20, // dist before = 20 > 5 → not adjacent
			src:        hitSrc,
			wantEvents: 0,
			desc:       "NPC beyond 5ft before stride: no reactive strike",
		},
		{
			name:       "dead NPC does NOT trigger RS",
			playerOld:  0,
			playerNew:  25,
			npcPos:     5,
			npcHP:      -1, // IsDead for NPC: CurrentHP <= 0
			src:        hitSrc,
			wantEvents: 0,
			desc:       "Dead NPC never fires reactive strike",
		},
		{
			name:       "adjacent NPC does NOT trigger RS when player moves toward (not away)",
			playerOld:  10,
			playerNew:  5,  // moved toward NPC at 5 (now distance=0, but moved closer)
			npcPos:     5,  // was adjacent (dist=5 ≤ 5), new dist=0 — not moving away
			src:        hitSrc,
			wantEvents: 0,
			desc:       "Player moving toward NPC: no reactive strike even if adjacent",
		},
		{
			name:       "reactive strike rolls miss on low roll",
			playerOld:  0,
			playerNew:  25,
			npcPos:     5,
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
			cbt, player := makeReactiveStrikeCombat(t, tc.playerNew, []struct {
				id  string
				pos int
				hp  int
			}{{"npc1", tc.npcPos, npcHP}})

			// Simulate: oldPos is where the player WAS before the stride.
			events := combat.CheckReactiveStrikes(cbt, player.ID, tc.playerOld, tc.src, nil)
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
		CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1, Position: 25,
	}
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_solo", []*combat.Combatant{player}, reg, nil, "")
	require.NoError(t, err)

	events := combat.CheckReactiveStrikes(cbt, player.ID, 0, hitSrc, nil)
	assert.Empty(t, events, "mover cannot trigger a reactive strike against themselves")
}

// TestPropertyReactiveStrike_NeverTriggersFromNonAdjacent verifies that when every NPC
// is beyond 5ft of the player's old position, zero reactive strikes fire.
func TestPropertyReactiveStrike_NeverTriggersFromNonAdjacent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a player old position and an NPC position such that |oldPos - npcPos| >= 6.
		playerOldPos := rapid.IntRange(0, 100).Draw(rt, "playerOldPos")
		// Generate NPC offset >= 6 in either direction.
		npcOffset := rapid.IntRange(6, 50).Draw(rt, "npcOffset")
		// Randomly choose direction.
		direction := rapid.IntRange(0, 1).Draw(rt, "direction")
		npcPos := playerOldPos + npcOffset
		if direction == 1 {
			npcPos = playerOldPos - npcOffset
			if npcPos < 0 {
				npcPos = playerOldPos + npcOffset
			}
		}

		// Player new position: moved 25ft away.
		playerNewPos := playerOldPos + 25

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		player := &combat.Combatant{
			ID: "player", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1, Position: playerNewPos,
		}
		npc := &combat.Combatant{
			ID: "npc", Kind: combat.KindNPC, Name: "npc",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, Position: npcPos,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_prop", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatal(err)
		}

		hitSrc := fixedSrcRS{val: 19}
		events := combat.CheckReactiveStrikes(cbt, player.ID, playerOldPos, hitSrc, nil)
		if len(events) != 0 {
			rt.Fatalf("expected 0 reactive strikes when NPC dist=%d > 5 from oldPos=%d, got %d",
				abs(npcPos-playerOldPos), playerOldPos, len(events))
		}
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestPropertyReactiveStrike_AdjacentAlwaysTriggers verifies that when an NPC is adjacent
// (≤5 ft) before the stride and the player moves away (new dist > 5), at least one
// reactive strike event is always returned.
func TestPropertyReactiveStrike_AdjacentAlwaysTriggers(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate NPC position in [0, 100].
		npcPos := rapid.IntRange(0, 100).Draw(rt, "npcPos")
		// Generate oldPos such that |npcPos - oldPos| <= 5 (adjacent).
		offset := rapid.IntRange(0, 5).Draw(rt, "offset")
		sign := rapid.IntRange(0, 1).Draw(rt, "sign")
		oldPos := npcPos + offset
		if sign == 1 {
			oldPos = npcPos - offset
			if oldPos < 0 {
				oldPos = npcPos + offset
			}
		}
		// Compute newPos such that |npcPos - newPos| > 5 (moved away).
		// Move 25 ft in the direction away from the NPC.
		newPos := oldPos + 25
		if abs(npcPos-newPos) <= 5 {
			// Fallback: move in the other direction.
			newPos = oldPos - 25
			if newPos < 0 {
				newPos = oldPos + 25
			}
		}
		// If still not far enough (pathological npcPos placement), skip this draw.
		if abs(npcPos-newPos) <= 5 {
			rt.Skip("unable to construct valid newPos for this draw; skipping")
		}

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		player := &combat.Combatant{
			ID: "player", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 30, MaxHP: 30, AC: 15, Level: 1, Position: newPos,
		}
		npc := &combat.Combatant{
			ID: "npc", Kind: combat.KindNPC, Name: "npc",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, Position: npcPos,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_adj", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatal(err)
		}

		// val=19 → d20=20; guaranteed hit against any reasonable AC.
		hitSrc := fixedSrcRS{val: 19}
		events := combat.CheckReactiveStrikes(cbt, player.ID, oldPos, hitSrc, nil)
		if len(events) < 1 {
			rt.Fatalf("expected at least 1 reactive strike when NPC at %d is adjacent to oldPos=%d and player moved to newPos=%d, got 0",
				npcPos, oldPos, newPos)
		}
	})
}

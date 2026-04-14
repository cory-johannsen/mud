package gameserver

import (
	"sync"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestStartCombat_InitialPositions_PopulateHP verifies that when combat starts
// the RoundStartEvent initial_positions include hp_current and hp_max for each combatant.
// REQ-78-1: CombatantPosition MUST include hp_current and hp_max at round start.
func TestStartCombat_InitialPositions_PopulateHP(t *testing.T) {
	var mu sync.Mutex
	var captured *gamev1.RoundStartEvent

	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	h.SetRoundStartBroadcastFn(func(_ string, evt *gamev1.RoundStartEvent) {
		mu.Lock()
		defer mu.Unlock()
		if captured == nil {
			captured = evt
		}
	})

	const roomID = "room-hp-broadcast"
	const npcMaxHP = 30

	tmpl := &npc.Template{
		ID:        "hp-npc",
		Name:      "Guard",
		Level:     1,
		MaxHP:     npcMaxHP,
		AC:        13,
		Awareness: 2,
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	inst.CurrentHP = npcMaxHP

	playerSess := addTestPlayer(t, h.sessions, "uid-hp-test", roomID)
	playerSess.MaxHP = 25
	playerSess.CurrentHP = 25

	_, err = h.Attack("uid-hp-test", "Guard")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	mu.Lock()
	rs := captured
	mu.Unlock()

	if rs == nil {
		t.Fatal("expected RoundStartEvent to be broadcast; got none")
	}

	if len(rs.InitialPositions) == 0 {
		t.Fatal("expected InitialPositions to be non-empty")
	}

	for _, pos := range rs.InitialPositions {
		if pos.HpMax == 0 {
			t.Errorf("combatant %q has HpMax=0 in InitialPositions; expected > 0", pos.Name)
		}
		if pos.HpCurrent < 0 {
			t.Errorf("combatant %q has HpCurrent=%d; must be >= 0", pos.Name, pos.HpCurrent)
		}
	}
}

// TestStartCombat_InitialPositions_NPCHPMatchesTemplate verifies that the NPC
// combatant position HP fields match the spawned template HP values.
// REQ-78-2: NPC hp_max in initial_positions MUST equal the NPC template MaxHP.
func TestStartCombat_InitialPositions_NPCHPMatchesTemplate(t *testing.T) {
	var mu sync.Mutex
	var captured *gamev1.RoundStartEvent

	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	h.SetRoundStartBroadcastFn(func(_ string, evt *gamev1.RoundStartEvent) {
		mu.Lock()
		defer mu.Unlock()
		if captured == nil {
			captured = evt
		}
	})

	const roomID = "room-hp-npc-match"
	const npcMaxHP = 45

	tmpl := &npc.Template{
		ID:        "match-npc",
		Name:      "Bandit",
		Level:     2,
		MaxHP:     npcMaxHP,
		AC:        14,
		Awareness: 2,
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	inst.CurrentHP = npcMaxHP

	playerSess := addTestPlayer(t, h.sessions, "uid-hp-match", roomID)
	playerSess.MaxHP = 30
	playerSess.CurrentHP = 30

	_, err = h.Attack("uid-hp-match", "Bandit")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	mu.Lock()
	rs := captured
	mu.Unlock()

	if rs == nil {
		t.Fatal("expected RoundStartEvent to be broadcast; got none")
	}

	for _, pos := range rs.InitialPositions {
		if pos.Name == "Bandit" {
			if pos.HpMax != int32(npcMaxHP) {
				t.Errorf("NPC HpMax=%d; want %d", pos.HpMax, npcMaxHP)
			}
			if pos.HpCurrent != int32(npcMaxHP) {
				t.Errorf("NPC HpCurrent=%d; want %d (full health at start)", pos.HpCurrent, npcMaxHP)
			}
			return
		}
	}
	t.Error("did not find 'Bandit' in InitialPositions")
}

package ai_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"pgregory.net/rapid"
)

func TestWorldState_EnemiesOf_ReturnsOnlyOppositeKind(t *testing.T) {
	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", HP: 20, Dead: false},
			{UID: "n1", Kind: "npc", HP: 15, Dead: false},
			{UID: "n2", Kind: "npc", HP: 10, Dead: false},
		},
	}
	enemies := ws.EnemiesOf("n1")
	if len(enemies) != 1 || enemies[0].UID != "p1" {
		t.Fatalf("expected 1 player enemy, got %v", enemies)
	}
}

func TestWorldState_EnemiesOf_ExcludesDead(t *testing.T) {
	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", HP: 0, Dead: true},
			{UID: "p2", Kind: "player", HP: 20, Dead: false},
		},
	}
	enemies := ws.EnemiesOf("n1")
	if len(enemies) != 1 || enemies[0].UID != "p2" {
		t.Fatalf("expected only living p2, got %v", enemies)
	}
}

func TestWorldState_NearestEnemy_ReturnsFirst(t *testing.T) {
	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", HP: 30, Dead: false},
			{UID: "p2", Kind: "player", HP: 20, Dead: false},
		},
	}
	nearest := ws.NearestEnemy("n1")
	if nearest == nil || nearest.UID != "p1" {
		t.Fatalf("expected p1 as nearest, got %v", nearest)
	}
}

func TestWorldState_WeakestEnemy_ReturnsLowestHP(t *testing.T) {
	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", HP: 30, MaxHP: 30, Dead: false},
			{UID: "p2", Kind: "player", HP: 5, MaxHP: 30, Dead: false},
		},
	}
	weakest := ws.WeakestEnemy("n1")
	if weakest == nil || weakest.UID != "p2" {
		t.Fatalf("expected p2 as weakest, got %v", weakest)
	}
}

func TestWorldState_HasLivingEnemies_ReturnsTrueWhenPresent(t *testing.T) {
	ws := &ai.WorldState{
		NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
		Combatants: []*ai.CombatantState{
			{UID: "p1", Kind: "player", HP: 20, Dead: false},
		},
	}
	if !ws.HasLivingEnemies("n1") {
		t.Fatal("expected HasLivingEnemies=true")
	}
}

func TestProperty_WorldState_NearestEnemy_NilWhenNoEnemies(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ws := &ai.WorldState{
			NPC: &ai.NPCState{UID: "n1", Kind: "npc"},
			Combatants: []*ai.CombatantState{
				// only same-kind combatants
				{UID: "n2", Kind: "npc", HP: 10, Dead: false},
			},
		}
		nearest := ws.NearestEnemy("n1")
		if nearest != nil {
			rt.Fatal("expected nil nearest enemy when no opponents")
		}
	})
}

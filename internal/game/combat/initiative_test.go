package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

type fixedInitSrc struct {
	vals []int
	idx  int
}

func (f *fixedInitSrc) Intn(n int) int {
	v := f.vals[f.idx%len(f.vals)]
	f.idx++
	return v % n
}

func TestRollInitiative_PlayerWins_BonusApplied(t *testing.T) {
	src := &fixedInitSrc{vals: []int{14, 4}} // player=15, npc=5, margin=10 → +2
	player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer}
	npc := &combat.Combatant{ID: "n", Kind: combat.KindNPC}
	combat.RollInitiative([]*combat.Combatant{player, npc}, src)
	if player.InitiativeBonus != 2 {
		t.Fatalf("want 2, got %d", player.InitiativeBonus)
	}
	if npc.InitiativeBonus != 0 {
		t.Fatalf("npc want 0, got %d", npc.InitiativeBonus)
	}
}

func TestRollInitiative_NPCWins_NoBonusToPlayer(t *testing.T) {
	src := &fixedInitSrc{vals: []int{2, 17}}
	player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer}
	npc := &combat.Combatant{ID: "n", Kind: combat.KindNPC}
	combat.RollInitiative([]*combat.Combatant{player, npc}, src)
	if player.InitiativeBonus != 0 {
		t.Fatalf("want 0, got %d", player.InitiativeBonus)
	}
}

func TestRollInitiative_Tie_NoBonusToPlayer(t *testing.T) {
	src := &fixedInitSrc{vals: []int{9, 9}}
	player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer}
	npc := &combat.Combatant{ID: "n", Kind: combat.KindNPC}
	combat.RollInitiative([]*combat.Combatant{player, npc}, src)
	if player.InitiativeBonus != 0 {
		t.Fatalf("want 0 on tie, got %d", player.InitiativeBonus)
	}
}

func TestRollInitiative_MarginBands(t *testing.T) {
	cases := []struct{ margin, want int }{{1, 1}, {5, 1}, {6, 2}, {10, 2}, {11, 3}, {20, 3}}
	for _, tc := range cases {
		got := combat.InitiativeBonusForMargin(tc.margin)
		if got != tc.want {
			t.Errorf("margin %d: want %d got %d", tc.margin, tc.want, got)
		}
	}
}

func TestProperty_RollInitiative_BonusInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pRoll := rapid.IntRange(1, 20).Draw(t, "pRoll")
		nRoll := rapid.IntRange(1, 20).Draw(t, "nRoll")
		src := &fixedInitSrc{vals: []int{pRoll - 1, nRoll - 1}}
		player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer}
		npc := &combat.Combatant{ID: "n", Kind: combat.KindNPC}
		combat.RollInitiative([]*combat.Combatant{player, npc}, src)
		if player.InitiativeBonus < 0 || player.InitiativeBonus > 3 {
			t.Fatalf("InitiativeBonus %d out of [0,3]", player.InitiativeBonus)
		}
		if npc.InitiativeBonus != 0 {
			t.Fatalf("NPC got %d, want 0", npc.InitiativeBonus)
		}
	})
}

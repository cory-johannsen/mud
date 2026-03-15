package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestCombat_DamageDealt_InitializedNonNil(t *testing.T) {
	e := combat.NewEngine()
	c1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
	c2 := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 20, CurrentHP: 20, Initiative: 5}
	cbt, err := e.StartCombat("room1", []*combat.Combatant{c1, c2}, nil, nil, "zone1")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	if cbt.DamageDealt == nil {
		t.Fatal("DamageDealt is nil; want initialized map")
	}
}

func TestCombat_RecordDamage_Accumulates(t *testing.T) {
	e := combat.NewEngine()
	c1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, MaxHP: 30, CurrentHP: 30, Initiative: 10}
	c2 := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, MaxHP: 100, CurrentHP: 100, Initiative: 5}
	cbt, err := e.StartCombat("room1", []*combat.Combatant{c1, c2}, nil, nil, "zone1")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.RecordDamage("p1", 10)
	cbt.RecordDamage("p1", 7)
	if got := cbt.DamageDealt["p1"]; got != 17 {
		t.Errorf("DamageDealt[p1]: want 17, got %d", got)
	}
}

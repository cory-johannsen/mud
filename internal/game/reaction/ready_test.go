// internal/game/reaction/ready_test.go
package reaction_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestProperty_ReadyRegistry_ConsumeAtomic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := reaction.NewReadyRegistry()
		uid := "player1"
		trigger := reaction.TriggerOnEnemyEntersRoom
		r.Add(reaction.ReadyEntry{UID: uid, Trigger: trigger, RoundSet: 1})

		e1 := r.Consume(uid, trigger, "")
		if e1 == nil {
			t.Fatal("first Consume: expected entry, got nil")
		}
		e2 := r.Consume(uid, trigger, "")
		if e2 != nil {
			t.Fatal("second Consume: expected nil after first consume, got entry")
		}
	})
}

func TestProperty_ReadyRegistry_ExpireRoundClearsOnlyThatRound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := reaction.NewReadyRegistry()
		r.Add(reaction.ReadyEntry{UID: "a", Trigger: reaction.TriggerOnEnemyEntersRoom, RoundSet: 1})
		r.Add(reaction.ReadyEntry{UID: "b", Trigger: reaction.TriggerOnEnemyMoveAdjacent, RoundSet: 2})
		r.ExpireRound(1)
		if e := r.Consume("a", reaction.TriggerOnEnemyEntersRoom, ""); e != nil {
			t.Fatal("round 1 entry should have been expired")
		}
		if e := r.Consume("b", reaction.TriggerOnEnemyMoveAdjacent, ""); e == nil {
			t.Fatal("round 2 entry should still be present")
		}
	})
}

func TestProperty_ReadyRegistry_CancelRemovesAllForUID(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := reaction.NewReadyRegistry()
		r.Add(reaction.ReadyEntry{UID: "p1", Trigger: reaction.TriggerOnEnemyEntersRoom, RoundSet: 1})
		r.Add(reaction.ReadyEntry{UID: "p1", Trigger: reaction.TriggerOnEnemyMoveAdjacent, RoundSet: 1})
		r.Add(reaction.ReadyEntry{UID: "p2", Trigger: reaction.TriggerOnAllyDamaged, RoundSet: 1})
		r.Cancel("p1")
		if e := r.Consume("p1", reaction.TriggerOnEnemyEntersRoom, ""); e != nil {
			t.Fatal("p1's TriggerOnEnemyEntersRoom entry should be cancelled")
		}
		if e := r.Consume("p1", reaction.TriggerOnEnemyMoveAdjacent, ""); e != nil {
			t.Fatal("p1's TriggerOnEnemyMoveAdjacent entry should be cancelled")
		}
		if e := r.Consume("p2", reaction.TriggerOnAllyDamaged, ""); e == nil {
			t.Fatal("p2 entry should be unaffected by p1 Cancel")
		}
	})
}

func TestProperty_ReadyRegistry_TriggerTgtFiltering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := reaction.NewReadyRegistry()
		r.Add(reaction.ReadyEntry{
			UID: "p1", Trigger: reaction.TriggerOnEnemyEntersRoom,
			TriggerTgt: "npc-goblin", RoundSet: 1,
		})
		// Wrong source UID — should not match
		if e := r.Consume("p1", reaction.TriggerOnEnemyEntersRoom, "npc-orc"); e != nil {
			t.Fatal("Consume with wrong source should return nil")
		}
		// Correct source UID — should match
		if e := r.Consume("p1", reaction.TriggerOnEnemyEntersRoom, "npc-goblin"); e == nil {
			t.Fatal("Consume with correct source should return entry")
		}
	})
}

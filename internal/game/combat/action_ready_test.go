// internal/game/combat/action_ready_test.go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestActionReady_CostIsTwo(t *testing.T) {
	if combat.ActionReady.Cost() != 2 {
		t.Fatalf("ActionReady.Cost() = %d, want 2", combat.ActionReady.Cost())
	}
}

func TestActionReady_StringIsReady(t *testing.T) {
	if combat.ActionReady.String() != "ready" {
		t.Fatalf("ActionReady.String() = %q, want %q", combat.ActionReady.String(), "ready")
	}
}

func TestActionReady_EnqueueDeductsTwoAP(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnEnemyEntersRoom,
		ReadyAction:  &combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.RemainingPoints() != 1 {
		t.Fatalf("RemainingPoints() = %d, want 1 (started 3, cost 2)", q.RemainingPoints())
	}
}

func TestActionReady_RejectsForbiddenTrigger(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnSaveFail, // not in AllowedReadyTriggers
		ReadyAction:  &combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"},
	})
	if err == nil {
		t.Fatal("expected error for forbidden trigger, got nil")
	}
}

func TestActionReady_RejectsForbiddenAction(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnEnemyEntersRoom,
		ReadyAction:  &combat.QueuedAction{Type: combat.ActionStrike}, // not in whitelist
	})
	if err == nil {
		t.Fatal("expected error for forbidden prepared action, got nil")
	}
}

func TestActionReady_RejectsNilReadyAction(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnEnemyEntersRoom,
		ReadyAction:  nil,
	})
	if err == nil {
		t.Fatal("expected error for nil ReadyAction, got nil")
	}
}

func TestActionReady_RejectsAbilityCostNotOne(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnEnemyEntersRoom,
		ReadyAction: &combat.QueuedAction{
			Type:        combat.ActionUseTech,
			AbilityID:   "some_tech",
			AbilityCost: 2, // must be 1 per REACTION-16
		},
	})
	if err == nil {
		t.Fatal("expected error for AbilityCost != 1 on use_tech, got nil")
	}
}

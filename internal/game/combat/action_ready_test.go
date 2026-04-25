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

// makeSinglePlayerCombat builds a *combat.Combat with exactly one living player
// combatant and calls StartRound(3) so ActionQueues are populated.
func makeSinglePlayerCombat(uid string) *combat.Combat {
	p := &combat.Combatant{
		ID: uid, Kind: combat.KindPlayer, Level: 1,
		MaxHP: 30, CurrentHP: 30, AC: 15,
	}
	engine := combat.NewEngine()
	cbt, err := engine.StartCombat("test-room", []*combat.Combatant{p}, makeTestRegistry(), nil, "")
	if err != nil {
		panic(err)
	}
	cbt.StartRound(3)
	return cbt
}

func TestQueueAction_ActionReady_RegistersReadyEntry(t *testing.T) {
	cbt := makeSinglePlayerCombat("p1")
	err := cbt.QueueAction("p1", combat.QueuedAction{
		Type:         combat.ActionReady,
		ReadyTrigger: reaction.TriggerOnEnemyEntersRoom,
		ReadyAction:  &combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"},
	})
	if err != nil {
		t.Fatalf("QueueAction: %v", err)
	}
	entry := cbt.ReadyRegistry.Consume("p1", reaction.TriggerOnEnemyEntersRoom, "")
	if entry == nil {
		t.Fatal("expected ReadyEntry in registry after QueueAction(ActionReady)")
	}
	if entry.Action.Type != "attack" {
		t.Fatalf("ReadyEntry.Action.Type = %q, want %q", entry.Action.Type, "attack")
	}
	if entry.Action.Target != "goblin" {
		t.Fatalf("ReadyEntry.Action.Target = %q, want %q", entry.Action.Target, "goblin")
	}
	if entry.RoundSet != cbt.Round {
		t.Fatalf("ReadyEntry.RoundSet = %d, want %d", entry.RoundSet, cbt.Round)
	}
}

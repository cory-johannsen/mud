package combat

// GH #244 Task 9: white-box tests for the fireTrigger helper.
//
// These tests exercise the three branches of REACTION-8 priority:
//   1. Ready-first: a queued Ready entry wins over any feat reaction path.
//   2. Ready fizzled: the Ready entry matched but revalidation fails; the
//      ReactionBudget is refunded and a EventTypeReadyFizzled event is emitted.
//   3. Budget exhausted for feat reactions: the feat-reaction callback is NOT
//      invoked once the combatant's reaction budget is at zero.
//
// We work at the package-internal level because fireTrigger is unexported.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// makeFireTriggerCombat builds a minimal Combat with a single player combatant
// and an initialised ReactionBudget (Max=1). A Ready registry is attached so
// tests can seed entries directly.
func makeFireTriggerCombat(t *testing.T) *Combat {
	t.Helper()
	cbt := &Combat{
		RoomID: "room1",
		Combatants: []*Combatant{
			{ID: "p1", Kind: KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1,
				ReactionBudget: &reaction.Budget{Max: 1, Spent: 0}},
			{ID: "n1", Kind: KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1,
				ReactionBudget: &reaction.Budget{Max: 1, Spent: 0}},
		},
		ActionQueues:  map[string]*ActionQueue{},
		ReadyRegistry: reaction.NewReadyRegistry(),
	}
	return cbt
}

// neverInvokedCallback fails the test if it is ever called.
func neverInvokedCallback(t *testing.T) reaction.ReactionCallback {
	t.Helper()
	return func(
		_ context.Context,
		_ string,
		_ reaction.ReactionTriggerType,
		_ reaction.ReactionContext,
		_ []reaction.PlayerReaction,
	) (bool, *reaction.PlayerReaction, error) {
		t.Fatalf("reaction callback was invoked when it should not have been")
		return false, nil, nil
	}
}

// TestFireTrigger_ReadyWinsOverFeatReaction verifies REACTION-8 priority: when a
// player has a queued Ready entry bound to the trigger, fireTrigger consumes
// the Ready entry and does NOT invoke the feat-reaction callback, even when
// both are otherwise eligible. Budget is spent; a EventTypeReactionFired event
// is emitted carrying the ReadyEntry pointer.
func TestFireTrigger_ReadyWinsOverFeatReaction(t *testing.T) {
	cbt := makeFireTriggerCombat(t)
	cbt.ReadyRegistry.Add(reaction.ReadyEntry{
		UID:      "p1",
		Trigger:  reaction.TriggerOnEnemyMoveAdjacent,
		Action:   reaction.ReadyActionDesc{Type: "attack", Target: "Ganger"},
		RoundSet: 1,
	})
	p1 := cbt.Combatants[0]

	events := fireTrigger(cbt, "p1", reaction.TriggerOnEnemyMoveAdjacent,
		reaction.ReactionContext{TriggerUID: "p1", SourceUID: "n1"},
		"n1", neverInvokedCallback(t), time.Second, nil)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ReadyEntry == nil {
		t.Fatalf("expected ReadyEntry to be non-nil on fired event")
	}
	if events[0].ReadyEntry.UID != "p1" || events[0].ReadyEntry.Action.Type != "attack" {
		t.Fatalf("unexpected ReadyEntry contents: %+v", events[0].ReadyEntry)
	}
	if !strings.Contains(events[0].Narrative, EventTypeReactionFired) {
		t.Fatalf("event narrative missing %q tag: %q", EventTypeReactionFired, events[0].Narrative)
	}
	if p1.ReactionBudget.Spent != 1 {
		t.Fatalf("ReactionBudget.Spent = %d, want 1", p1.ReactionBudget.Spent)
	}
}

// TestFireTrigger_ReadyFizzledWhenTargetDead verifies that when a Ready entry
// matches its trigger but the queued target is dead at fire time,
// revalidateReadyEntry returns false: the budget is refunded and an
// EventTypeReadyFizzled event is emitted with no ReadyEntry pointer.
func TestFireTrigger_ReadyFizzledWhenTargetDead(t *testing.T) {
	cbt := makeFireTriggerCombat(t)
	// Kill the Ready target before the trigger fires.
	cbt.Combatants[1].CurrentHP = 0
	cbt.ReadyRegistry.Add(reaction.ReadyEntry{
		UID:      "p1",
		Trigger:  reaction.TriggerOnEnemyMoveAdjacent,
		Action:   reaction.ReadyActionDesc{Type: "attack", Target: "Ganger"},
		RoundSet: 1,
	})
	p1 := cbt.Combatants[0]

	events := fireTrigger(cbt, "p1", reaction.TriggerOnEnemyMoveAdjacent,
		reaction.ReactionContext{TriggerUID: "p1", SourceUID: "n1"},
		"n1", neverInvokedCallback(t), time.Second, nil)

	if len(events) != 1 {
		t.Fatalf("expected 1 fizzled event, got %d", len(events))
	}
	if events[0].ReadyEntry != nil {
		t.Fatalf("fizzled event should not carry ReadyEntry")
	}
	if !strings.Contains(events[0].Narrative, EventTypeReadyFizzled) {
		t.Fatalf("event narrative missing %q tag: %q", EventTypeReadyFizzled, events[0].Narrative)
	}
	if p1.ReactionBudget.Spent != 0 {
		t.Fatalf("budget should be refunded on fizzle; Spent = %d, want 0", p1.ReactionBudget.Spent)
	}
}

// TestFireTrigger_BudgetExhaustedSkipsCallback verifies that when the
// combatant's ReactionBudget has no remaining slots, the feat-reaction callback
// is NOT invoked and no event is emitted.
func TestFireTrigger_BudgetExhaustedSkipsCallback(t *testing.T) {
	cbt := makeFireTriggerCombat(t)
	// Pre-exhaust the budget.
	cbt.Combatants[0].ReactionBudget.Spent = cbt.Combatants[0].ReactionBudget.Max

	events := fireTrigger(cbt, "p1", reaction.TriggerOnDamageTaken,
		reaction.ReactionContext{TriggerUID: "p1", SourceUID: "n1"},
		"n1", neverInvokedCallback(t), time.Second, nil)

	if len(events) != 0 {
		t.Fatalf("expected no events when budget exhausted, got %d", len(events))
	}
	if cbt.Combatants[0].ReactionBudget.Spent != cbt.Combatants[0].ReactionBudget.Max {
		t.Fatalf("budget should be unchanged when callback skipped")
	}
}

// TestFireTrigger_FeatReactionFiresWhenNoReady verifies the fallback: with no
// Ready entry queued but a callback that accepts, the feat reaction fires,
// budget is spent, and an EventTypeReactionFired narrative event is emitted
// without a ReadyEntry pointer.
func TestFireTrigger_FeatReactionFiresWhenNoReady(t *testing.T) {
	cbt := makeFireTriggerCombat(t)
	p1 := cbt.Combatants[0]

	chosen := &reaction.PlayerReaction{
		UID:      "p1",
		Feat:     "chrome_reflex",
		FeatName: "Chrome Reflex",
	}
	callback := reaction.ReactionCallback(func(
		_ context.Context,
		_ string,
		_ reaction.ReactionTriggerType,
		_ reaction.ReactionContext,
		_ []reaction.PlayerReaction,
	) (bool, *reaction.PlayerReaction, error) {
		return true, chosen, nil
	})

	events := fireTrigger(cbt, "p1", reaction.TriggerOnDamageTaken,
		reaction.ReactionContext{TriggerUID: "p1", SourceUID: "n1"},
		"n1", callback, time.Second, nil)

	if len(events) != 1 {
		t.Fatalf("expected 1 fired event, got %d", len(events))
	}
	if events[0].ReadyEntry != nil {
		t.Fatalf("feat-reaction event should not carry ReadyEntry")
	}
	if !strings.Contains(events[0].Narrative, EventTypeReactionFired) {
		t.Fatalf("event narrative missing %q tag: %q", EventTypeReactionFired, events[0].Narrative)
	}
	if !strings.Contains(events[0].Narrative, "Chrome Reflex") {
		t.Fatalf("event narrative missing feat name: %q", events[0].Narrative)
	}
	if p1.ReactionBudget.Spent != 1 {
		t.Fatalf("ReactionBudget.Spent = %d, want 1", p1.ReactionBudget.Spent)
	}
}

// TestFireTrigger_FeatReactionDeclinedRefundsBudget verifies that when the
// callback returns (false, nil, nil) — player declined or the reaction didn't
// satisfy requirements — the budget is refunded.
func TestFireTrigger_FeatReactionDeclinedRefundsBudget(t *testing.T) {
	cbt := makeFireTriggerCombat(t)
	p1 := cbt.Combatants[0]

	declined := reaction.ReactionCallback(func(
		_ context.Context,
		_ string,
		_ reaction.ReactionTriggerType,
		_ reaction.ReactionContext,
		_ []reaction.PlayerReaction,
	) (bool, *reaction.PlayerReaction, error) {
		return false, nil, nil
	})

	events := fireTrigger(cbt, "p1", reaction.TriggerOnDamageTaken,
		reaction.ReactionContext{TriggerUID: "p1", SourceUID: "n1"},
		"n1", declined, time.Second, nil)

	if len(events) != 0 {
		t.Fatalf("expected no events on decline, got %d", len(events))
	}
	if p1.ReactionBudget.Spent != 0 {
		t.Fatalf("budget should be refunded on decline; Spent = %d, want 0", p1.ReactionBudget.Spent)
	}
}

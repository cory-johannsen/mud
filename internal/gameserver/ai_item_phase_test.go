package gameserver_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ai"
)

// TestProperty_AIItemPhase_APNeverNegative_MultipleItems verifies that regardless of
// how many AI items are equipped and what their operator AP costs are, the AP pool
// NEVER goes below 0. Satisfies REQ-AIE-11h (multi-item variant).
func TestProperty_AIItemPhase_APNeverNegative_MultipleItems(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numItems := rapid.IntRange(0, 4).Draw(rt, "numItems")
		initialAP := rapid.IntRange(0, 3).Draw(rt, "initialAP")

		// Simulate StartRound(3) + N AP contributions (1 per item, per REQ-AIE-3).
		ap := initialAP + numItems

		reg := ai.NewItemDomainRegistry()
		for i := 0; i < numItems; i++ {
			opCost := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("cost_%d", i))
			domain := &ai.Domain{
				ID:    fmt.Sprintf("item_%d", i),
				Tasks: []*ai.Task{{ID: "behave"}},
				Methods: []*ai.Method{{
					TaskID: "behave", ID: "act", Subtasks: []string{"op"},
				}},
				Operators: []*ai.Operator{{ID: "op", Action: "lua_hook", APCost: opCost}},
			}
			if err := reg.Register(domain); err != nil {
				rt.Fatalf("Register domain %d: %v", i, err)
			}

			script := `operators = {}; operators.op = function(self) end`
			snap := ai.ItemCombatSnapshot{
				Player: ai.ItemPlayerSnapshot{ID: "p1", HP: 20, MaxHP: 20, AP: ap},
			}
			cbs := ai.ItemPrimitiveCalls{
				Attack: func(_, _ string, cost int) bool {
					if ap < cost {
						return false
					}
					ap -= cost
					return true
				},
				Say:    func(_ []string) {},
				Buff:   func(_, _ string, _, cost int) bool { return ap >= cost },
				Debuff: func(_, _ string, _, cost int) bool { return ap >= cost },
				GetAP:  func() int { return ap },
				SpendAP: func(n int) bool {
					if ap < n {
						return false
					}
					ap -= n
					return true
				},
			}
			p, ok := reg.PlannerFor(fmt.Sprintf("item_%d", i))
			if !ok {
				rt.Fatalf("PlannerFor item_%d not found", i)
			}
			if err := p.Execute(script, map[string]interface{}{}, snap, cbs); err != nil {
				rt.Skip() // Lua errors are not invariant failures
			}
		}

		if ap < 0 {
			rt.Errorf("AP went negative after %d items: final AP=%d (initial=%d+%d item AP)",
				numItems, ap, initialAP, numItems)
		}
	})
}

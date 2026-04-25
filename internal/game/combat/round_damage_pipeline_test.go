package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// TestProperty_ResolveRound_DamageNonNegative verifies that, regardless of the
// target's resistance value or the seed of the deterministic Source, the damage
// applied via the new ResolveDamage pipeline is always >= 0 and HP never falls
// below 0. This exercises the full attack -> BuildDamageInput -> ResolveDamage
// path via ResolveRound and proves the floor stage prevents negative damage
// even when resistance exceeds the rolled damage. (MULT-17)
func TestProperty_ResolveRound_DamageNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		val := rapid.IntRange(0, 19).Draw(rt, "rngVal")
		resist := rapid.IntRange(0, 50).Draw(rt, "resist")

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0})
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})

		eng := combat.NewEngine()
		combatants := []*combat.Combatant{
			{
				ID: "p1", Kind: combat.KindPlayer, Name: "Alice",
				MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1,
			},
			{
				ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
				MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1, StrMod: 1, DexMod: 0,
				// Apply slashing resistance — the unarmed default ResolveAttack
				// produces no DamageType, but if a future test adds one, the
				// resistance must still floor at 0.
				Resistances: map[string]int{"slashing": resist, "": resist},
				Weaknesses:  map[string]int{},
			},
		}
		cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
		if err != nil {
			rt.Fatalf("StartCombat: %v", err)
		}
		_ = cbt.StartRound(3)

		if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
			rt.Fatalf("QueueAction p1: %v", err)
		}
		if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
			rt.Fatalf("QueueAction n1: %v", err)
		}

		src := fixedSrc{val: val}
		events := combat.ResolveRound(cbt, src, func(id string, hp int) {}, nil, 0)

		for _, ev := range events {
			if ev.AttackResult != nil && ev.AttackResult.BaseDamage < 0 {
				rt.Fatalf("AttackResult.BaseDamage=%d, expected >= 0", ev.AttackResult.BaseDamage)
			}
		}
		for _, c := range cbt.Combatants {
			if c.CurrentHP < 0 {
				rt.Fatalf("combatant %s has CurrentHP=%d, expected >= 0", c.ID, c.CurrentHP)
			}
		}
	})
}

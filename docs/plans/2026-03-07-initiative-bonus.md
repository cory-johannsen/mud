# Initiative Bonus Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace PF2E turn-order initiative with a Gunchete combat bonus — when the player wins the initiative roll, they gain +N to attack rolls and AC for the entire combat, scaled by margin of victory.

**Architecture:** Add `InitiativeBonus int` field to `Combatant`; compute it in `RollInitiative` after comparing player vs NPC rolls; include it in attack resolution and AC comparison in `round.go`.

**Tech Stack:** Go, `internal/game/combat/`, `internal/gameserver/combat_handler.go`

**Margin → Bonus table:**
- Margin 1–5 → +1
- Margin 6–10 → +2
- Margin 11+ → +3

---

### Task 1: Add InitiativeBonus to Combatant and compute it in RollInitiative

**Files:**
- Modify: `internal/game/combat/combat.go` — add `InitiativeBonus int` field
- Modify: `internal/game/combat/initiative.go` — export `InitiativeBonusForMargin`, update `RollInitiative`
- Test: `internal/game/combat/initiative_test.go`

**Step 1: Write the failing tests**

Create `internal/game/combat/initiative_test.go`:

```go
package combat_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/combat"
)

type fixedSrc struct{ vals []int; idx int }
func (f *fixedSrc) Intn(n int) int { v := f.vals[f.idx%len(f.vals)]; f.idx++; return v % n }

func TestRollInitiative_PlayerWins_BonusApplied(t *testing.T) {
    // Player rolls 15 (14+1), NPC rolls 5 (4+1) → margin 10 → +2
    src := &fixedSrc{vals: []int{14, 4}}
    player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer, DexMod: 0}
    npc    := &combat.Combatant{ID: "n", Kind: combat.KindNPC,    DexMod: 0}
    combat.RollInitiative([]*combat.Combatant{player, npc}, src)
    if player.InitiativeBonus != 2 {
        t.Fatalf("expected InitiativeBonus=2, got %d", player.InitiativeBonus)
    }
    if npc.InitiativeBonus != 0 {
        t.Fatalf("expected NPC InitiativeBonus=0, got %d", npc.InitiativeBonus)
    }
}

func TestRollInitiative_NPCWins_NoBonusToPlayer(t *testing.T) {
    src := &fixedSrc{vals: []int{2, 17}}
    player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer, DexMod: 0}
    npc    := &combat.Combatant{ID: "n", Kind: combat.KindNPC,    DexMod: 0}
    combat.RollInitiative([]*combat.Combatant{player, npc}, src)
    if player.InitiativeBonus != 0 {
        t.Fatalf("expected InitiativeBonus=0, got %d", player.InitiativeBonus)
    }
}

func TestRollInitiative_Tie_NoBonusToPlayer(t *testing.T) {
    src := &fixedSrc{vals: []int{9, 9}}
    player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer, DexMod: 0}
    npc    := &combat.Combatant{ID: "n", Kind: combat.KindNPC,    DexMod: 0}
    combat.RollInitiative([]*combat.Combatant{player, npc}, src)
    if player.InitiativeBonus != 0 {
        t.Fatalf("expected InitiativeBonus=0 on tie, got %d", player.InitiativeBonus)
    }
}

func TestRollInitiative_MarginBands(t *testing.T) {
    cases := []struct{ margin, want int }{
        {1, 1}, {5, 1}, {6, 2}, {10, 2}, {11, 3}, {20, 3},
    }
    for _, tc := range cases {
        got := combat.InitiativeBonusForMargin(tc.margin)
        if got != tc.want {
            t.Errorf("margin %d: want %d, got %d", tc.margin, tc.want, got)
        }
    }
}

func TestProperty_RollInitiative_BonusInRange(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        pRoll := rapid.IntRange(1, 20).Draw(t, "pRoll")
        nRoll := rapid.IntRange(1, 20).Draw(t, "nRoll")
        src := &fixedSrc{vals: []int{pRoll - 1, nRoll - 1}}
        player := &combat.Combatant{ID: "p", Kind: combat.KindPlayer}
        npc    := &combat.Combatant{ID: "n", Kind: combat.KindNPC}
        combat.RollInitiative([]*combat.Combatant{player, npc}, src)
        if player.InitiativeBonus < 0 || player.InitiativeBonus > 3 {
            t.Fatalf("InitiativeBonus %d out of [0,3]", player.InitiativeBonus)
        }
        if npc.InitiativeBonus != 0 {
            t.Fatalf("NPC must never get InitiativeBonus, got %d", npc.InitiativeBonus)
        }
    })
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/game/combat/... -run TestRollInitiative -v 2>&1 | head -20
```
Expected: compile error (InitiativeBonus undefined).

**Step 3: Add InitiativeBonus field to combat.go**

After `Initiative int` in the Combatant struct:
```go
// InitiativeBonus is the persistent attack and AC bonus for a player who wins initiative.
// Range [0,3]; always 0 for NPCs.
InitiativeBonus int
```

**Step 4: Export InitiativeBonusForMargin and update RollInitiative in initiative.go**

Replace the entire file with:
```go
package combat

// InitiativeBonusForMargin maps a positive initiative margin to a combat bonus.
// Margin 1–5 → +1, 6–10 → +2, 11+ → +3.
//
// Precondition: margin > 0
func InitiativeBonusForMargin(margin int) int {
    switch {
    case margin <= 5:
        return 1
    case margin <= 10:
        return 2
    default:
        return 3
    }
}

// RollInitiative rolls initiative for all combatants and sets their Initiative field.
// Formula: d20 + DexMod.
// If a player combatant beats all NPC combatants, that player receives an
// InitiativeBonus scaled by their margin of victory over the highest NPC roll.
//
// Precondition: combatants non-nil; src non-nil.
// Postcondition: Initiative set for all; player InitiativeBonus set if they win.
func RollInitiative(combatants []*Combatant, src Source) {
    for _, c := range combatants {
        roll := src.Intn(20) + 1
        c.Initiative = roll + c.DexMod
    }

    var player *Combatant
    highestNPC := -1 << 30
    for _, c := range combatants {
        if c.Kind == KindPlayer {
            player = c
        } else if c.Initiative > highestNPC {
            highestNPC = c.Initiative
        }
    }

    if player == nil {
        return
    }
    margin := player.Initiative - highestNPC
    if margin > 0 {
        player.InitiativeBonus = InitiativeBonusForMargin(margin)
    }
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/game/combat/... -v 2>&1 | tail -20
```

**Step 6: Commit**

```bash
git add internal/game/combat/combat.go internal/game/combat/initiative.go internal/game/combat/initiative_test.go
git commit -m "feat: add InitiativeBonus to Combatant; compute from margin in RollInitiative"
```

---

### Task 2: Apply InitiativeBonus in attack resolution

**Files:**
- Modify: `internal/game/combat/round.go`
- Test: `internal/game/combat/round_test.go`

**Context:** Three attack action cases exist: ActionAttack, ActionStrike, ActionBash. Each reads:
```go
atkBonus := condition.AttackBonus(cbt.Conditions[actor.ID])
acBonus  := condition.ACBonus(cbt.Conditions[target.ID])
r := ResolveAttack(actor, target, src)
r.AttackTotal += atkBonus
r.AttackTotal += acBonus
r.AttackTotal = hookAttackRoll(...)
r.Outcome = OutcomeFor(r.AttackTotal, target.AC)
```

Must add `actor.InitiativeBonus` to attack total and `target.InitiativeBonus` to effective AC in all three cases.

**Step 1: Write failing test in round_test.go**

```go
func TestResolveRound_InitiativeBonus_AffectsAttackAndAC(t *testing.T) {
    player := &Combatant{
        ID: "p", Kind: KindPlayer, Name: "Player",
        MaxHP: 20, CurrentHP: 20, AC: 10, InitiativeBonus: 2,
        DexMod: 0, StrMod: 2,
    }
    npc := &Combatant{
        ID: "n", Kind: KindNPC, Name: "Goon",
        MaxHP: 10, CurrentHP: 10, AC: 8,
    }
    // Fixed dice: always roll 1 so we can reason about totals
    src := &testSrc{val: 0}
    cbt := &Combat{
        RoomID: "r1", Round: 1,
        Combatants: []*Combatant{player, npc},
        Actions: map[string]Action{
            "p": {Type: ActionAttack, Target: "Goon"},
        },
    }
    events := ResolveRound(cbt, src, nil, nil, nil)
    found := false
    for _, ev := range events {
        if ev.ActorID == "p" && ev.AttackResult != nil {
            // roll(1) + StrMod(2) + InitiativeBonus(2) = 5 minimum
            if ev.AttackResult.AttackTotal < 5 {
                t.Fatalf("AttackTotal %d does not include InitiativeBonus=2", ev.AttackResult.AttackTotal)
            }
            found = true
        }
    }
    if !found {
        t.Fatal("no player attack event found")
    }
}
```

Note: Use whatever `testSrc` type already exists in `round_test.go`; check its interface before writing.

**Step 2: Run test to verify it fails**

```bash
go test ./internal/game/combat/... -run TestResolveRound_InitiativeBonus -v
```

**Step 3: Modify round.go — apply InitiativeBonus in all 3 attack cases**

For each of ActionAttack, ActionStrike, ActionBash, change:
```go
r.Outcome = OutcomeFor(r.AttackTotal, target.AC)
```
to:
```go
r.AttackTotal += actor.InitiativeBonus
effectiveAC := target.AC + target.InitiativeBonus
r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
```

Place `r.AttackTotal += actor.InitiativeBonus` BEFORE the `hookAttackRoll` call so hooks see the full bonus.

**Step 4: Run full combat test suite**

```bash
go test ./internal/game/combat/... -v 2>&1 | tail -20
```

**Step 5: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/round_test.go
git commit -m "feat: apply InitiativeBonus to attack rolls and AC in ResolveRound"
```

---

### Task 3: Display initiative bonus in combat start narrative

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

**Context:** After the initiative event loop (around line 742–750 in combat_handler.go), add a bonus announcement event when the player wins.

**Step 1: Add bonus announcement after the initiative events loop**

In `startCombat` (or equivalent function), after:
```go
for _, c := range cbt.Combatants {
    events = append(events, &gamev1.CombatEvent{
        Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
        Attacker:  c.Name,
        Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
    })
}
```

Add:
```go
for _, c := range cbt.Combatants {
    if c.Kind == combat.KindPlayer && c.InitiativeBonus > 0 {
        events = append(events, &gamev1.CombatEvent{
            Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
            Attacker:  c.Name,
            Narrative: fmt.Sprintf("You win initiative! +%d to attack and AC this combat.", c.InitiativeBonus),
        })
    }
}
```

**Step 2: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```
Expected: all pass.

**Step 3: Commit**

```bash
git add internal/gameserver/combat_handler.go
git commit -m "feat: announce initiative bonus in combat start events"
```

---

### Task 4: Update FEATURES.md and deploy

**Step 1: Mark feature complete in docs/requirements/FEATURES.md**

Change:
```
- [ ] Initiative.  
  - PF2E uses Initiative for combat order but Gunchete uses a timed round with simultaneous action.  
  - Initiative grants players an advantage in P2FE that Gunchete has lost
    - I want to replace it with a combat bonus that reflects the original advantage initiative provided.
```
To:
```
- [x] Initiative — replaced with persistent combat bonus when player wins initiative roll.
  - Margin 1–5 → +1, 6–10 → +2, 11+ → +3 to attack rolls and AC for the entire combat.
```

**Step 2: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

**Step 3: Deploy and push**

```bash
make k8s-redeploy
git add docs/requirements/FEATURES.md
git commit -m "docs: mark Initiative bonus feature complete"
git push
```

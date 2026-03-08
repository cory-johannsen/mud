# Damage Types — Resistance & Weakness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Wire weapon damage types through the combat pipeline so NPC resistances and weaknesses affect final damage, with console announcements.

**Architecture:** `WeaponDef.DamageType` already exists on all weapons. We add `Resistances`/`Weaknesses map[string]int` to NPC templates, instances, and combatants. `AttackResult` gains a `DamageType` field. `ResolveAttack` receives the weapon def so it can record the damage type. After damage is computed in `round.go`, resistance/weakness is applied and annotated in the narrative.

**Tech Stack:** Go, existing `internal/game/combat`, `internal/game/npc`, `internal/game/inventory` packages. No proto changes needed.

---

### Task 1: Add Resistances/Weaknesses to NPC Template and Instance

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`

**Step 1: Add fields to Template**

In `template.go`, add after `SkillChecks`:

```go
// Resistances maps damage type → flat damage reduction (minimum 0 after reduction).
Resistances map[string]int `yaml:"resistances"`
// Weaknesses maps damage type → flat damage addition applied on any hit (including 0 damage).
Weaknesses  map[string]int `yaml:"weaknesses"`
```

**Step 2: Add fields to Instance**

In `instance.go`, add after `SkillChecks []skillcheck.TriggerDef`:

```go
// Resistances maps damage type → flat reduction. Copied from template at spawn.
Resistances map[string]int
// Weaknesses maps damage type → flat bonus. Copied from template at spawn.
Weaknesses  map[string]int
```

**Step 3: Wire in NewInstance**

In `NewInstance`, add:
```go
Resistances:   tmpl.Resistances,
Weaknesses:    tmpl.Weaknesses,
```

**Step 4: Write failing test**

In `internal/game/npc/template_test.go` (create if absent, else add to existing), add:

```go
func TestTemplate_ResistancesWeaknesses_LoadedFromYAML(t *testing.T) {
    yaml := `
id: test_npc
name: Test NPC
description: desc
level: 1
max_hp: 10
ac: 10
perception: 0
resistances:
  fire: 5
  piercing: 2
weaknesses:
  electricity: 3
`
    var tmpl npc.Template
    require.NoError(t, yaml.Unmarshal([]byte(yaml), &tmpl))
    assert.Equal(t, 5, tmpl.Resistances["fire"])
    assert.Equal(t, 2, tmpl.Resistances["piercing"])
    assert.Equal(t, 3, tmpl.Weaknesses["electricity"])
}

func TestNewInstance_CopiesResistancesWeaknesses(t *testing.T) {
    tmpl := &npc.Template{
        ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10,
        Resistances: map[string]int{"fire": 5},
        Weaknesses:  map[string]int{"electricity": 3},
    }
    inst := npc.NewInstance("i1", tmpl, "room1")
    assert.Equal(t, 5, inst.Resistances["fire"])
    assert.Equal(t, 3, inst.Weaknesses["electricity"])
}
```

**Step 5: Run test to verify it fails**

```bash
go test ./internal/game/npc/... -run TestTemplate_ResistancesWeaknesses -v
go test ./internal/game/npc/... -run TestNewInstance_CopiesResistances -v
```

Expected: compile error or FAIL (fields don't exist yet).

**Step 6: Implement (add fields as described above)**

**Step 7: Run tests to verify they pass**

```bash
go test ./internal/game/npc/... -v
```

**Step 8: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/template_test.go
git commit -m "feat: add Resistances/Weaknesses to NPC template and instance"
```

---

### Task 2: Add DamageType to Combatant and AttackResult

**Files:**
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/game/combat/resolver.go`

**Step 1: Write failing test**

In `internal/game/combat/resolver_test.go` (or add to existing resolver test file), add:

```go
func TestResolveAttack_PropagatesDamageType(t *testing.T) {
    attacker := &combat.Combatant{
        ID: "p", Kind: combat.KindPlayer, Name: "Player",
        MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, StrMod: 2,
        WeaponDamageType: "fire",
    }
    target := &combat.Combatant{
        ID: "n", Kind: combat.KindNPC, Name: "NPC",
        MaxHP: 10, CurrentHP: 10, AC: 10,
    }
    result := combat.ResolveAttack(attacker, target, dice.NewSource(0))
    assert.Equal(t, "fire", result.DamageType)
}
```

**Step 2: Run test — expect compile error (field doesn't exist)**

```bash
go test ./internal/game/combat/... -run TestResolveAttack_PropagatesDamageType -v
```

**Step 3: Add `WeaponDamageType string` to Combatant**

In `combat.go`, after `WeaponProficiencyRank`:

```go
// WeaponDamageType is the damage type of the currently equipped main-hand weapon.
// Empty string means unarmed (bludgeoning baseline for narrative, no special type).
WeaponDamageType string
```

**Step 4: Add `DamageType string` to AttackResult**

In `resolver.go`, after `DamageRoll []int`:

```go
// DamageType is the damage type of the attack (e.g. "fire", "piercing").
// Empty string means unarmed/untyped.
DamageType string
```

**Step 5: Propagate in ResolveAttack**

In `ResolveAttack`, add to the returned struct:
```go
DamageType: attacker.WeaponDamageType,
```

**Step 6: Add `Resistances`/`Weaknesses` to Combatant**

In `combat.go`:

```go
// Resistances maps damage type → flat damage reduction (applied after all other modifiers, min 0).
// Always nil for player combatants.
Resistances map[string]int
// Weaknesses maps damage type → flat damage addition on any hit.
// Always nil for player combatants.
Weaknesses  map[string]int
```

**Step 7: Run tests**

```bash
go test ./internal/game/combat/... -v
```

**Step 8: Commit**

```bash
git add internal/game/combat/combat.go internal/game/combat/resolver.go
git commit -m "feat: add WeaponDamageType to Combatant, DamageType+Resistances/Weaknesses to AttackResult/Combatant"
```

---

### Task 3: Apply Resistance/Weakness in round.go

**Files:**
- Modify: `internal/game/combat/round.go`

**Step 1: Write failing test**

In `internal/game/combat/round_resistance_test.go` (new file):

```go
package combat_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestResolveRound_Resistance_ReducesDamage verifies that a target's fire resistance
// reduces fire damage, minimum 0.
func TestResolveRound_Resistance_ReducesDamage(t *testing.T) {
    src := dice.NewSource(15) // high roll → hit guaranteed
    player := &combat.Combatant{
        ID: "p", Kind: combat.KindPlayer, Name: "Player",
        MaxHP: 30, CurrentHP: 30, AC: 10, Level: 1, StrMod: 3,
        WeaponDamageType: "fire",
    }
    npc := &combat.Combatant{
        ID: "n", Kind: combat.KindNPC, Name: "NPC",
        MaxHP: 20, CurrentHP: 20, AC: 10,
        Resistances: map[string]int{"fire": 5},
    }
    cbt := &combat.Combat{
        Combatants: []*combat.Combatant{player, npc},
        Conditions: map[string]*combat.ConditionSet{},
    }
    player.QueueAction(combat.Action{Type: combat.ActionAttack, Target: "NPC"})
    events := combat.ResolveRound(cbt, src, nil, nil)
    // Damage must have been reduced by 5 (resistance applied)
    require.NotEmpty(t, events)
    // NPC HP should reflect reduced damage
    assert.Less(t, npc.CurrentHP, 20)
    // Narrative should mention resistance
    found := false
    for _, e := range events {
        if strings.Contains(e.Narrative, "resist") {
            found = true
        }
    }
    assert.True(t, found, "expected 'resist' in narrative")
}

// TestResolveRound_Weakness_AddsDamage verifies that a target's electricity weakness
// adds bonus damage.
func TestResolveRound_Weakness_AddsDamage(t *testing.T) {
    src := dice.NewSource(15)
    player := &combat.Combatant{
        ID: "p", Kind: combat.KindPlayer, Name: "Player",
        MaxHP: 30, CurrentHP: 30, AC: 10, Level: 1, StrMod: 3,
        WeaponDamageType: "electricity",
    }
    npc := &combat.Combatant{
        ID: "n", Kind: combat.KindNPC, Name: "NPC",
        MaxHP: 20, CurrentHP: 20, AC: 10,
        Weaknesses: map[string]int{"electricity": 4},
    }
    cbt := &combat.Combat{
        Combatants: []*combat.Combatant{player, npc},
        Conditions: map[string]*combat.ConditionSet{},
    }
    player.QueueAction(combat.Action{Type: combat.ActionAttack, Target: "NPC"})
    events := combat.ResolveRound(cbt, src, nil, nil)
    require.NotEmpty(t, events)
    // NPC took extra damage from weakness
    assert.Less(t, npc.CurrentHP, 20)
    found := false
    for _, e := range events {
        if strings.Contains(e.Narrative, "weak") {
            found = true
        }
    }
    assert.True(t, found, "expected 'weak' in narrative")
}

// TestResolveRound_Resistance_MinZero verifies resistance can't make damage negative.
func TestResolveRound_Resistance_MinZero(t *testing.T) {
    src := dice.NewSource(10)
    player := &combat.Combatant{
        ID: "p", Kind: combat.KindPlayer, Name: "Player",
        MaxHP: 30, CurrentHP: 30, AC: 10, Level: 1, StrMod: 0,
        WeaponDamageType: "piercing",
    }
    npc := &combat.Combatant{
        ID: "n", Kind: combat.KindNPC, Name: "NPC",
        MaxHP: 20, CurrentHP: 20, AC: 10,
        Resistances: map[string]int{"piercing": 100}, // huge resistance
    }
    cbt := &combat.Combat{
        Combatants: []*combat.Combatant{player, npc},
        Conditions: map[string]*combat.ConditionSet{},
    }
    player.QueueAction(combat.Action{Type: combat.ActionAttack, Target: "NPC"})
    combat.ResolveRound(cbt, src, nil, nil)
    // HP must not go below 0 due to negative damage
    assert.GreaterOrEqual(t, npc.CurrentHP, 0)
}
```

**Step 2: Run test — expect compile errors**

```bash
go test ./internal/game/combat/... -run TestResolveRound_Resistance -v
go test ./internal/game/combat/... -run TestResolveRound_Weakness -v
```

**Step 3: Add `applyResistanceWeakness` helper in round.go**

Add near top of `round.go` (after `attackNarrative`):

```go
// applyResistanceWeakness applies target resistance/weakness for the given damage type
// to baseDmg. Returns adjusted damage (minimum 0) and annotation strings for the narrative.
//
// Precondition: baseDmg >= 0; damageType may be empty (no adjustment applied).
// Postcondition: returned damage >= 0; annotations is empty when no adjustment applies.
func applyResistanceWeakness(target *Combatant, damageType string, baseDmg int) (finalDmg int, annotations []string) {
    if damageType == "" || baseDmg == 0 {
        return baseDmg, nil
    }
    result := baseDmg
    if r, ok := target.Resistances[damageType]; ok && r > 0 {
        result -= r
        if result < 0 {
            result = 0
        }
        annotations = append(annotations, fmt.Sprintf("resisted %d %s damage", r, damageType))
    }
    if w, ok := target.Weaknesses[damageType]; ok && w > 0 {
        result += w
        annotations = append(annotations, fmt.Sprintf("%s weakness +%d", damageType, w))
    }
    return result, annotations
}
```

**Step 4: Wire applyResistanceWeakness in ActionAttack, ActionStrike (×2) in round.go**

For each attack block that currently does:
```go
dmg := r.EffectiveDamage()
dmg += condition.DamageBonus(...)
dmg += applyPassiveFeats(...)
dmg = hookDamageRoll(...)
if dmg > 0 {
    target.ApplyDamage(dmg)
    ...
}
// narrative: attackNarrative(...)
```

Change to insert resistance/weakness after `hookDamageRoll`:
```go
dmg, rwAnnotations := applyResistanceWeakness(target, r.DamageType, dmg)
if dmg > 0 {
    target.ApplyDamage(dmg)
    ...
}
narrative := attackNarrative(actor.Name, "attacks", target.Name, r.Outcome, r.AttackTotal, dmg)
if len(rwAnnotations) > 0 {
    narrative += " (" + strings.Join(rwAnnotations, "; ") + ")"
}
```

Apply the same pattern to the explosive/cast paths that deal typed damage.

**Step 5: Run tests**

```bash
go test ./internal/game/combat/... -v
```

**Step 6: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/round_resistance_test.go
git commit -m "feat: apply resistance/weakness in combat round; annotate narrative"
```

---

### Task 4: Wire WeaponDamageType into combat_handler.go

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

**Step 1: Write failing test (verify via existing combat handler test structure)**

Locate `TestCombatHandler_*` tests or the integration test. Add a test that starts combat with a fire weapon and confirms the event narrative contains "fire" type info. If no suitable test exists, note the gap and add a minimal smoke test.

**Step 2: Wire in combat_handler.go**

After wiring `WeaponProficiencyRank`, add:
```go
// Wire weapon damage type from equipped main-hand weapon.
if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
    playerCbt.WeaponDamageType = playerCbt.Loadout.MainHand.Def.DamageType
}
```

**Step 3: Wire NPC resistances/weaknesses**

In `npcToCombatant` (or wherever NPC instances are converted to `*Combatant`):

```go
npcCbt.Resistances = inst.Resistances
npcCbt.Weaknesses  = inst.Weaknesses
```

**Step 4: Run tests**

```bash
go test ./internal/gameserver/... -v
```

**Step 5: Commit**

```bash
git add internal/gameserver/combat_handler.go
git commit -m "feat: wire WeaponDamageType and NPC resistances/weaknesses into combat handler"
```

---

### Task 5: Add Resistance/Weakness to NPC YAML content

**Files:**
- Modify: Several NPC YAML files in `content/npcs/`

Add resistance/weakness to at least 3 NPCs, covering multiple damage types, to make the system testable in-game:

- `content/npcs/82nd_enforcer.yaml`: `resistances: {bludgeoning: 2}` (tough enforcer)
- `content/npcs/ganger.yaml` (or equivalent): `weaknesses: {fire: 3}`
- One NPC with electricity resistance (e.g. a robot NPC if one exists; otherwise skip)

Format:
```yaml
resistances:
  bludgeoning: 2
weaknesses:
  fire: 3
```

Run `go test ./...` to ensure content loads cleanly (YAML parse tests).

**Commit:**
```bash
git add content/npcs/
git commit -m "content: add damage type resistances/weaknesses to select NPCs"
```

---

### Task 6: End-to-end build and deploy

**Step 1:** Run full test suite
```bash
go test ./... 2>&1
```
All must pass.

**Step 2:** Deploy
```bash
make k8s-redeploy
```

**Step 3:** Mark feature complete in `docs/requirements/FEATURES.md`:
```
- [x] Damage types
  - [x] Resistance
  - [x] Weakness
```

**Step 4:** Commit docs
```bash
git add docs/requirements/FEATURES.md docs/plans/2026-03-07-damage-types.md
git commit -m "docs: mark damage types feature complete"
git push
```

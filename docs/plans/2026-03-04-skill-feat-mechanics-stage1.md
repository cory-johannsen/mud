# Skill & Feat Mechanical Effects — Stage 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement automatic skill check triggers for room entry, NPC greeting, and item use, with proficiency-based roll resolution and Lua hook support.

**Architecture:** A new `internal/game/skillcheck` package provides the resolver (d20 + ability_mod + proficiency_bonus vs DC, 4-tier outcome). Trigger definitions live in existing room/NPC/item YAML files as optional `skill_checks:` blocks. The gRPC service fires triggers at the appropriate interaction points and delivers outcome messages to the player.

**Tech Stack:** Go (pgregory.net/rapid for property tests), YAML (gopkg.in/yaml.v3), gopher-lua (existing scripting system)

---

## Background & Key Facts

- Skills are stored as `map[string]string` (skill_id → proficiency rank) on `character.Character`
- `PlayerSession` in `internal/game/session/manager.go` does NOT currently carry skills — Task 3 adds this
- Proficiency ranks: untrained=+0, trained=+2, expert=+4, master=+6, legendary=+8
- 4-tier outcomes match combat system: CritSuccess (≥DC+10), Success (≥DC), Failure (<DC), CritFailure (<DC−10)
- `scripting.Manager.CallHook(zoneID, hook, args...)` — fires Lua hook, never panics on missing hook
- Ability scores are on `session.Abilities` (type `character.AbilityScores`): Brutality, Grit, Quickness, Reasoning, Savvy, Flair
- Ability modifier: `(score - 10) / 2` (use `combat.AbilityMod` in `internal/game/combat/resolver.go`)
- Room YAML is parsed in the world package — find where `Room` struct is defined before editing
- Conditions are applied via `ActiveSet.Apply(uid, def, stacks, duration)` in `internal/game/condition/active.go`
- `handleMove` in `internal/gameserver/grpc_service.go` already calls `scriptMgr.CallHook` for `on_enter`/`on_exit`

---

## Task 1: `skillcheck` package — types

**Files:**
- Create: `internal/game/skillcheck/types.go`
- Create: `internal/game/skillcheck/types_test.go`

**Step 1: Write the failing test**

```go
// internal/game/skillcheck/types_test.go
package skillcheck_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "gopkg.in/yaml.v3"
    "github.com/cory-johannsen/mud/internal/game/skillcheck"
)

func TestTriggerDef_ParsesFromYAML(t *testing.T) {
    raw := `
skill: parkour
dc: 14
trigger: on_enter
outcomes:
  crit_success:
    message: "You vault effortlessly."
  success:
    message: "You pick your way through."
  failure:
    message: "You stumble."
    effect:
      type: damage
      formula: "1d4"
  crit_failure:
    message: "You fall hard."
    effect:
      type: damage
      formula: "2d4"
`
    var td skillcheck.TriggerDef
    err := yaml.Unmarshal([]byte(raw), &td)
    assert.NoError(t, err)
    assert.Equal(t, "parkour", td.Skill)
    assert.Equal(t, 14, td.DC)
    assert.Equal(t, "on_enter", td.Trigger)
    assert.NotNil(t, td.Outcomes.CritSuccess)
    assert.Equal(t, "You vault effortlessly.", td.Outcomes.CritSuccess.Message)
    assert.NotNil(t, td.Outcomes.Failure.Effect)
    assert.Equal(t, "damage", td.Outcomes.Failure.Effect.Type)
    assert.Equal(t, "1d4", td.Outcomes.Failure.Effect.Formula)
    assert.Nil(t, td.Outcomes.Success.Effect)
}

func TestCheckOutcome_Constants(t *testing.T) {
    assert.True(t, skillcheck.CritSuccess < skillcheck.Success)
    assert.True(t, skillcheck.Success < skillcheck.Failure)
    assert.True(t, skillcheck.Failure < skillcheck.CritFailure)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run "TestTriggerDef_ParsesFromYAML|TestCheckOutcome" -v ./internal/game/skillcheck/...
```
Expected: FAIL with "no such package"

**Step 3: Implement types**

```go
// internal/game/skillcheck/types.go
package skillcheck

// CheckOutcome represents the 4-tier result of a skill check (matches combat system).
type CheckOutcome int

const (
    // Precondition: roll total and DC must be valid integers.
    // Postcondition: exactly one constant represents any given roll result.
    CritSuccess CheckOutcome = iota
    Success
    Failure
    CritFailure
)

func (o CheckOutcome) String() string {
    switch o {
    case CritSuccess:
        return "crit_success"
    case Success:
        return "success"
    case Failure:
        return "failure"
    case CritFailure:
        return "crit_failure"
    default:
        return "unknown"
    }
}

// Effect describes a mechanical consequence applied after a skill check outcome.
type Effect struct {
    Type    string `yaml:"type"`    // "damage" | "condition" | "deny" | "reveal"
    Formula string `yaml:"formula"` // dice formula for "damage" (e.g., "1d4")
    ID      string `yaml:"id"`      // condition ID for "condition"
    Target  string `yaml:"target"`  // target ID for "reveal"
}

// Outcome pairs a player-facing message with an optional mechanical effect.
type Outcome struct {
    Message string  `yaml:"message"`
    Effect  *Effect `yaml:"effect,omitempty"`
}

// OutcomeMap holds one Outcome per tier.
type OutcomeMap struct {
    CritSuccess *Outcome `yaml:"crit_success"`
    Success     *Outcome `yaml:"success"`
    Failure     *Outcome `yaml:"failure"`
    CritFailure *Outcome `yaml:"crit_failure"`
}

// ForOutcome returns the Outcome for a given CheckOutcome tier, or nil if not defined.
// Precondition: none (nil is a valid return).
func (m OutcomeMap) ForOutcome(o CheckOutcome) *Outcome {
    switch o {
    case CritSuccess:
        return m.CritSuccess
    case Success:
        return m.Success
    case Failure:
        return m.Failure
    case CritFailure:
        return m.CritFailure
    default:
        return nil
    }
}

// TriggerDef defines a single skill check trigger declared in YAML.
type TriggerDef struct {
    Skill    string     `yaml:"skill"`   // skill ID (e.g., "parkour")
    DC       int        `yaml:"dc"`      // difficulty class
    Trigger  string     `yaml:"trigger"` // "on_enter" | "on_greet" | "on_use"
    Outcomes OutcomeMap `yaml:"outcomes"`
}

// CheckResult holds the full result of a resolved skill check.
type CheckResult struct {
    TriggerDef  TriggerDef
    Roll        int          // raw d20 result (1-20)
    AbilityMod  int          // ability modifier applied
    ProfBonus   int          // proficiency bonus from rank
    Total       int          // Roll + AbilityMod + ProfBonus
    Outcome     CheckOutcome
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run "TestTriggerDef_ParsesFromYAML|TestCheckOutcome" -v ./internal/game/skillcheck/...
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/skillcheck/types.go internal/game/skillcheck/types_test.go
git commit -m "feat: skillcheck package types (TriggerDef, CheckResult, CheckOutcome)"
```

---

## Task 2: `skillcheck` package — resolver

**Files:**
- Create: `internal/game/skillcheck/resolver.go`
- Modify: `internal/game/skillcheck/types_test.go` (add resolver tests)
- Create: `internal/game/skillcheck/resolver_test.go`

**Context:** The resolver rolls d20 + ability modifier + proficiency bonus vs DC. Proficiency bonus by rank: untrained=0, trained=2, expert=4, master=6, legendary=8. 4-tier outcome: CritSuccess if total ≥ DC+10, Success if ≥ DC, Failure if ≥ DC−10, CritFailure otherwise.

The `dice.Source` interface in `internal/game/dice/` has `Intn(n int) int`. Use it for the d20.

**Step 1: Write the failing tests**

```go
// internal/game/skillcheck/resolver_test.go
package skillcheck_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// fixedSource always returns the same value (for deterministic tests).
type fixedSource struct{ val int }
func (f fixedSource) Intn(n int) int { return f.val % n }

func TestProficiencyBonus(t *testing.T) {
    cases := []struct{ rank string; want int }{
        {"untrained", 0},
        {"trained", 2},
        {"expert", 4},
        {"master", 6},
        {"legendary", 8},
        {"", 0},         // unknown → untrained
        {"godlike", 0},  // unknown → untrained
    }
    for _, c := range cases {
        assert.Equal(t, c.want, skillcheck.ProficiencyBonus(c.rank), "rank=%q", c.rank)
    }
}

func TestOutcomeFor(t *testing.T) {
    cases := []struct {
        total, dc int
        want      skillcheck.CheckOutcome
    }{
        {30, 15, skillcheck.CritSuccess},  // 30 ≥ 15+10
        {25, 15, skillcheck.CritSuccess},  // 25 = 15+10
        {20, 15, skillcheck.Success},      // 20 ≥ 15
        {15, 15, skillcheck.Success},      // 15 = 15
        {14, 15, skillcheck.Failure},      // 14 < 15 but ≥ 15-10=5
        {5, 15,  skillcheck.Failure},      // 5 = 15-10
        {4, 15,  skillcheck.CritFailure},  // 4 < 15-10
    }
    for _, c := range cases {
        got := skillcheck.OutcomeFor(c.total, c.dc)
        assert.Equal(t, c.want, got, "total=%d dc=%d", c.total, c.dc)
    }
}

func TestResolver_Resolve(t *testing.T) {
    // d20=10 (fixedSource returns 9, +1), abilityScore=14 (mod=+2), rank=trained (+2), DC=14
    // total = 10+2+2 = 14 → Success (≥ DC)
    src := fixedSource{val: 9} // Intn(20) returns 9, so d20 = 9+1 = 10
    result := skillcheck.Resolve(src, "parkour", 14, "trained", 14)
    assert.Equal(t, 10, result.Roll)
    assert.Equal(t, 2, result.AbilityMod)
    assert.Equal(t, 2, result.ProfBonus)
    assert.Equal(t, 14, result.Total)
    assert.Equal(t, skillcheck.Success, result.Outcome)
}

// Property: Total always equals Roll + AbilityMod + ProfBonus.
func TestProperty_Resolver_TotalIsSum(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        score  := rapid.IntRange(1, 20).Draw(t, "score")
        dc     := rapid.IntRange(1, 30).Draw(t, "dc")
        rankIdx := rapid.IntRange(0, 4).Draw(t, "rankIdx")
        ranks := []string{"untrained", "trained", "expert", "master", "legendary"}
        rank  := ranks[rankIdx]
        d20val := rapid.IntRange(0, 19).Draw(t, "d20val")
        src := fixedSource{val: d20val}

        result := skillcheck.Resolve(src, "any_skill", score, rank, dc)
        assert.Equal(t, result.Roll+result.AbilityMod+result.ProfBonus, result.Total)
    })
}

// Property: Outcome is consistent with Total and DC.
func TestProperty_Resolver_OutcomeConsistentWithTotal(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        score  := rapid.IntRange(1, 20).Draw(t, "score")
        dc     := rapid.IntRange(1, 30).Draw(t, "dc")
        d20val := rapid.IntRange(0, 19).Draw(t, "d20val")
        src := fixedSource{val: d20val}

        result := skillcheck.Resolve(src, "any_skill", score, "untrained", dc)
        switch result.Outcome {
        case skillcheck.CritSuccess:
            assert.GreaterOrEqual(t, result.Total, dc+10)
        case skillcheck.Success:
            assert.GreaterOrEqual(t, result.Total, dc)
            assert.Less(t, result.Total, dc+10)
        case skillcheck.Failure:
            assert.Less(t, result.Total, dc)
            assert.GreaterOrEqual(t, result.Total, dc-10)
        case skillcheck.CritFailure:
            assert.Less(t, result.Total, dc-10)
        }
    })
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run "TestProficiencyBonus|TestOutcomeFor|TestResolver" -v ./internal/game/skillcheck/...
```
Expected: FAIL — functions undefined

**Step 3: Implement the resolver**

```go
// internal/game/skillcheck/resolver.go
package skillcheck

import "github.com/cory-johannsen/mud/internal/game/dice"

// ProficiencyBonus returns the flat bonus for a proficiency rank.
// Precondition: rank is one of untrained/trained/expert/master/legendary (or empty/unknown).
// Postcondition: returns 0 for unknown ranks.
func ProficiencyBonus(rank string) int {
    switch rank {
    case "trained":
        return 2
    case "expert":
        return 4
    case "master":
        return 6
    case "legendary":
        return 8
    default:
        return 0 // untrained or unknown
    }
}

// OutcomeFor returns the 4-tier outcome for a roll total vs a DC.
// Precondition: dc > 0.
func OutcomeFor(total, dc int) CheckOutcome {
    switch {
    case total >= dc+10:
        return CritSuccess
    case total >= dc:
        return Success
    case total >= dc-10:
        return Failure
    default:
        return CritFailure
    }
}

// abilityMod returns the ability modifier for a raw ability score.
func abilityMod(score int) int {
    diff := score - 10
    if diff < 0 {
        return (diff - 1) / 2
    }
    return diff / 2
}

// Resolve performs a skill check roll and returns the full result.
// Preconditions: src must not be nil; skillID is informational only.
// Postcondition: result.Total == result.Roll + result.AbilityMod + result.ProfBonus.
func Resolve(src dice.Source, skillID string, abilityScore int, rank string, dc int) CheckResult {
    roll      := src.Intn(20) + 1
    amod      := abilityMod(abilityScore)
    prof      := ProficiencyBonus(rank)
    total     := roll + amod + prof
    outcome   := OutcomeFor(total, dc)
    return CheckResult{
        Roll:       roll,
        AbilityMod: amod,
        ProfBonus:  prof,
        Total:      total,
        Outcome:    outcome,
    }
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run "TestProficiencyBonus|TestOutcomeFor|TestResolver|TestProperty_Resolver" -v ./internal/game/skillcheck/...
```
Expected: PASS (including 100 rapid iterations)

**Step 5: Commit**

```bash
git add internal/game/skillcheck/resolver.go internal/game/skillcheck/resolver_test.go
git commit -m "feat: skillcheck resolver (proficiency bonus, 4-tier outcome, Resolve function)"
```

---

## Task 3: Add `Skills` to `PlayerSession`

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/frontend/handlers/character_flow.go`
- Test: `internal/game/session/manager_test.go` (or existing test file)

**Context:** `PlayerSession` needs `Skills map[string]string` so the skill check resolver can look up proficiency without a DB call. Skills are already loaded in `ensureSkills` in `character_flow.go`. After `ensureSkills`, they need to be written back onto the session. Find how `gameBridge` creates or hydrates the session to wire this in.

Read `internal/frontend/handlers/character_flow.go` and `internal/game/session/manager.go` before editing.

**Step 1: Write the failing test**

In `internal/game/session/manager_test.go` (or create it), add:

```go
func TestPlayerSession_HasSkillsField(t *testing.T) {
    sess := &session.PlayerSession{}
    assert.NotNil(t, sess) // compiles only if Skills field exists
    sess.Skills = map[string]string{"parkour": "trained"}
    assert.Equal(t, "trained", sess.Skills["parkour"])
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestPlayerSession_HasSkillsField -v ./internal/game/session/...
```
Expected: FAIL — `PlayerSession` has no `Skills` field

**Step 3: Add `Skills` field to `PlayerSession`**

In `internal/game/session/manager.go`, find `PlayerSession` struct and add after `Level int`:

```go
// Skills maps skill_id to proficiency rank for the active character.
// Populated after ensureSkills completes; empty map means all untrained.
Skills map[string]string
```

**Step 4: Populate `Skills` from character after `ensureSkills`**

In `internal/frontend/handlers/character_flow.go`, find where `ensureSkills` is called (3 call sites — all 3 `gameBridge` invocation paths). After each `ensureSkills(ctx, conn, char)` call, add:

```go
// Hydrate session skills from character for skill check resolution.
if sess, ok := h.sessionStore.GetPlayer(char.Name); ok {
    if sess.Skills == nil {
        sess.Skills = make(map[string]string)
    }
    for id, rank := range char.Skills {
        sess.Skills[id] = rank
    }
}
```

Look at how `h.sessionStore` is accessed in the existing code to use the correct field name.

**Step 5: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestPlayerSession_HasSkillsField -v ./internal/game/session/...
```
Expected: PASS

**Step 6: Run full suite**

```bash
cd /home/cjohannsen/src/mud && mise run test -- ./internal/game/... ./internal/frontend/...
```
Expected: PASS

**Step 7: Commit**

```bash
git add internal/game/session/manager.go internal/frontend/handlers/character_flow.go internal/game/session/manager_test.go
git commit -m "feat: add Skills map to PlayerSession, populate from character after ensureSkills"
```

---

## Task 4: Extend Room struct with `SkillChecks`

**Files:**
- Modify: the file where `Room` struct is defined (find it: `grep -rn "type Room struct" internal/`)
- Modify: the YAML loader for rooms (likely in `internal/game/world/`)
- Test: existing room/world test file

**Context:** Rooms are defined in zone YAML files. The zone loader parses rooms into a `Room` struct. We need to add an optional `SkillChecks []skillcheck.TriggerDef` field to `Room` and parse `skill_checks:` from YAML.

**Step 1: Find the Room struct and loader**

```bash
grep -rn "type Room struct" /home/cjohannsen/src/mud/internal/
grep -rn "type Zone struct" /home/cjohannsen/src/mud/internal/
```

Read whichever file contains `Room` struct.

**Step 2: Write the failing test**

In the world/zone test file (or create one), add:

```go
func TestRoom_ParsesSkillChecks(t *testing.T) {
    raw := `
zone:
  id: test_zone
  name: Test Zone
  start_room: room1
  rooms:
  - id: room1
    title: Test Room
    description: A test room.
    skill_checks:
    - skill: parkour
      dc: 14
      trigger: on_enter
      outcomes:
        success:
          message: "You pass."
        failure:
          message: "You fail."
          effect:
            type: damage
            formula: "1d4"
`
    // Use the existing zone loader function to parse this YAML
    // (find the function name by reading the zone loader file)
    zone, err := world.ParseZoneYAML([]byte(raw)) // adjust function name as needed
    assert.NoError(t, err)
    assert.Len(t, zone.Rooms["room1"].SkillChecks, 1)
    assert.Equal(t, "parkour", zone.Rooms["room1"].SkillChecks[0].Skill)
    assert.Equal(t, 14, zone.Rooms["room1"].SkillChecks[0].DC)
}
```

**Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestRoom_ParsesSkillChecks -v ./internal/game/world/...
```
Expected: FAIL

**Step 4: Add `SkillChecks` field to Room struct**

```go
SkillChecks []skillcheck.TriggerDef `yaml:"skill_checks"`
```

Import `github.com/cory-johannsen/mud/internal/game/skillcheck` in the world package.

**Step 5: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestRoom_ParsesSkillChecks -v ./internal/game/world/...
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/game/world/ internal/game/skillcheck/
git commit -m "feat: add SkillChecks field to Room struct, parsed from YAML skill_checks block"
```

---

## Task 5: Room `on_enter` trigger in `handleMove`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_test.go`

**Context:** `handleMove` in `grpc_service.go` already calls `scriptMgr.CallHook(zoneID, "on_enter", ...)` after movement. After this hook call, add skill check trigger logic:

1. Get the new room from `s.world.GetRoom(result.View.RoomId)`
2. For each trigger with `Trigger == "on_enter"` in `room.SkillChecks`:
   a. Get the session to find ability score and skill rank
   b. Look up which ability the skill uses (need `allSkills` registry — it's on `GameServiceServer`)
   c. Resolve the check with `skillcheck.Resolve(s.dice, skillID, abilityScore, rank, dc)`
   d. Get the outcome and its message
   e. Send a `messageEvent` to the player
   f. Apply the effect (damage or condition)
   g. Call Lua hook: `s.scriptMgr.CallHook(zoneID, "on_skill_check", lua.LString(uid), lua.LString(skillID), lua.LNumber(result.Total), lua.LNumber(dc), lua.LString(result.Outcome.String()))`

**Note:** `s.dice` may not exist — find how the gRPC service accesses a dice source. Look for `rand` or `dice` usage in `grpc_service.go`. If none exists, add `dice dice.Source` to `GameServiceServer` and initialize it in `NewGameServiceServer` with `dice.NewRandSource()` (or equivalent — check `internal/game/dice/`).

**Step 1: Write the failing test**

In `grpc_service_test.go`, add a test that:
- Creates a server with a world containing a room with a `parkour` skill check (DC 10)
- Gives the player a session with `Skills["parkour"] = "trained"` and Quickness=14
- Sends a move request to that room
- Asserts the server response includes a message event with the outcome text

Use the existing `testGRPCServer` helper pattern from the file.

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestHandleMove_SkillCheck -v ./internal/gameserver/...
```
Expected: FAIL

**Step 3: Implement the trigger in `handleMove`**

Add a helper method `applyRoomSkillChecks(uid string, room *world.Room) []*gamev1.ServerEvent` to `GameServiceServer`.

```go
func (s *GameServiceServer) applyRoomSkillChecks(uid string, room *world.Room) []*gamev1.ServerEvent {
    if len(room.SkillChecks) == 0 {
        return nil
    }
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil
    }

    var events []*gamev1.ServerEvent
    for _, trigger := range room.SkillChecks {
        if trigger.Trigger != "on_enter" {
            continue
        }
        // Find the skill's key ability
        abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
        rank := sess.Skills[trigger.Skill] // "" = untrained
        result := skillcheck.Resolve(s.dice, trigger.Skill, abilityScore, rank, trigger.DC)

        // Get outcome message
        outcome := trigger.Outcomes.ForOutcome(result.Outcome)
        if outcome != nil && outcome.Message != "" {
            events = append(events, messageEvent(outcome.Message))
        }

        // Apply effect
        if outcome != nil && outcome.Effect != nil {
            s.applySkillCheckEffect(uid, sess, outcome.Effect, room.ZoneID)
        }

        // Lua hook
        if s.scriptMgr != nil {
            s.scriptMgr.CallHook(room.ZoneID, "on_skill_check",
                lua.LString(uid),
                lua.LString(trigger.Skill),
                lua.LNumber(result.Total),
                lua.LNumber(trigger.DC),
                lua.LString(result.Outcome.String()),
            )
        }
    }
    return events
}

// abilityScoreForSkill returns the raw ability score for the skill's key ability.
// Precondition: sess must not be nil.
func (s *GameServiceServer) abilityScoreForSkill(sess *session.PlayerSession, skillID string) int {
    // Look up the skill's key ability from allSkills
    for _, sk := range s.allSkills {
        if sk.ID == skillID {
            switch sk.Ability {
            case "brutality":
                return sess.Abilities.Brutality
            case "grit":
                return sess.Abilities.Grit
            case "quickness":
                return sess.Abilities.Quickness
            case "reasoning":
                return sess.Abilities.Reasoning
            case "savvy":
                return sess.Abilities.Savvy
            case "flair":
                return sess.Abilities.Flair
            }
        }
    }
    return 10 // fallback: no modifier
}

// applySkillCheckEffect applies the mechanical effect of a skill check outcome.
func (s *GameServiceServer) applySkillCheckEffect(uid string, sess *session.PlayerSession, effect *skillcheck.Effect, zoneID string) {
    switch effect.Type {
    case "damage":
        dmg, err := dice.RollExpr(effect.Formula, s.dice)
        if err != nil {
            s.logger.Warn("skill check damage formula error", zap.String("formula", effect.Formula), zap.Error(err))
            return
        }
        sess.CurrentHP -= dmg.Total()
        if sess.CurrentHP < 0 {
            sess.CurrentHP = 0
        }
    case "condition":
        // Condition application via scriptMgr callback if available
        if s.scriptMgr != nil && s.scriptMgr.ApplyCondition != nil {
            s.scriptMgr.ApplyCondition(uid, effect.ID, 1, -1)
        }
    }
    // "deny" and "reveal" are handled by callers (not applicable for on_enter)
}
```

In `handleMove`, after the existing `on_enter` hook call, add:
```go
if skillEvents := s.applyRoomSkillChecks(uid, newRoom); len(skillEvents) > 0 {
    // Send each event to the player
    for _, ev := range skillEvents {
        s.sendToPlayer(uid, ev) // use whatever send function exists
    }
}
```

**Note:** Check how `handleMove` returns events to the player — it may return a single `ServerEvent` or use a streaming send. Match the existing pattern.

**Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise run test -- -run TestHandleMove_SkillCheck -v ./internal/gameserver/...
```
Expected: PASS

**Step 5: Run full suite (excluding slow postgres tests)**

```bash
cd /home/cjohannsen/src/mud && mise run test -- ./internal/game/... ./internal/gameserver/... ./internal/frontend/...
```
Expected: all pass

**Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat: fire skill check on room on_enter trigger in handleMove"
```

---

## Task 6: NPC template `SkillChecks` and `on_greet` trigger

**Files:**
- Modify: NPC template struct (find with `grep -rn "type.*Template struct\|type NPC struct" internal/game/npc/`)
- Modify: NPC YAML loader
- Modify: `internal/gameserver/grpc_service.go` — handleMove's NPC greeting path
- Test: NPC test file + grpc_service_test.go

**Context:** `handleMove` already calls `inst.TryTaunt(time.Now())` for NPCs in the new room. Alongside this, check each NPC's skill_checks for `on_greet` triggers.

**Step 1: Find the NPC template struct**

```bash
grep -rn "type.*Template\|SkillCheck\|skill_check" /home/cjohannsen/src/mud/internal/game/npc/ | head -20
```

Read the NPC template file to understand the struct.

**Step 2: Write the failing test**

Add a test that creates an NPC template with a `smooth_talk` skill check (DC 16, on_greet), enters a room with that NPC, and verifies the skill check outcome message is delivered.

**Step 3: Add `SkillChecks []skillcheck.TriggerDef` to the NPC template struct**

Add the field with `yaml:"skill_checks"` tag.

**Step 4: Wire `on_greet` trigger in `handleMove`**

After the existing NPC `TryTaunt` logic in `handleMove`, add:

```go
for _, inst := range s.npcH.InstancesInRoom(result.View.RoomId) {
    template := s.npcH.Template(inst.TemplateID) // find correct method name
    if template == nil {
        continue
    }
    for _, trigger := range template.SkillChecks {
        if trigger.Trigger != "on_greet" {
            continue
        }
        abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
        rank := sess.Skills[trigger.Skill]
        result := skillcheck.Resolve(s.dice, trigger.Skill, abilityScore, rank, trigger.DC)
        outcome := trigger.Outcomes.ForOutcome(result.Outcome)
        if outcome != nil && outcome.Message != "" {
            s.sendToPlayer(uid, messageEvent(outcome.Message))
        }
        if outcome != nil && outcome.Effect != nil {
            s.applySkillCheckEffect(uid, sess, outcome.Effect, room.ZoneID)
        }
    }
}
```

**Step 5: Run tests, fix, commit**

```bash
git commit -m "feat: NPC on_greet skill check trigger"
```

---

## Task 7: Item `SkillChecks` and `on_use` trigger with `deny` support

**Files:**
- Modify: item definition struct (find with `grep -rn "type.*Item\b" internal/game/`)
- Modify: item YAML loader
- Modify: `internal/gameserver/grpc_service.go` — `handleUseEquipment`
- Test: grpc_service_test.go

**Context:** `handleUseEquipment` currently calls the item's Lua script. Before invoking the Lua script, check if the item has a `skill_checks` block with an `on_use` trigger. If the skill check outcome has `effect.type == "deny"`, return the failure message and do NOT execute the Lua script.

**Step 1: Find item struct**

```bash
grep -rn "type RoomEquipDef\|type ItemDef\|type Equipment struct" /home/cjohannsen/src/mud/internal/ | head -10
```

Read the relevant file.

**Step 2: Add `SkillChecks []skillcheck.TriggerDef` to item/equipment definition struct**

**Step 3: Write test — deny blocks item use**

Test that an item with a Grift check (DC 20) and deny on failure blocks use when the player has untrained Grift and a bad roll.

**Step 4: Implement in `handleUseEquipment`**

At the start of `handleUseEquipment`, before the Lua script call:

```go
// Check skill_checks for on_use triggers
for _, trigger := range inst.SkillChecks {
    if trigger.Trigger != "on_use" {
        continue
    }
    abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
    rank := sess.Skills[trigger.Skill]
    checkResult := skillcheck.Resolve(s.dice, trigger.Skill, abilityScore, rank, trigger.DC)
    outcome := trigger.Outcomes.ForOutcome(checkResult.Outcome)
    if outcome != nil && outcome.Message != "" {
        // always send the check result message
        // if deny effect, return early (block the use)
        if outcome.Effect != nil && outcome.Effect.Type == "deny" {
            return messageEvent(outcome.Message), nil
        }
        // otherwise send message and continue
        s.sendToPlayer(uid, messageEvent(outcome.Message))
    }
    if outcome != nil && outcome.Effect != nil && outcome.Effect.Type != "deny" {
        s.applySkillCheckEffect(uid, sess, outcome.Effect, zoneID)
    }
}
// ... existing Lua script call
```

**Step 5: Run tests, fix, commit**

```bash
git commit -m "feat: item on_use skill check trigger with deny support"
```

---

## Task 8: Add sample skill_checks to content + update FEATURES.md + deploy

**Files:**
- Modify: 2-3 room YAML files in `content/zones/`
- Modify: 1-2 NPC YAML files in `content/npcs/`
- Modify: 1 item YAML file in `content/items/`
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Add a Parkour check to a room with difficult terrain**

Find a room that describes rubble, wreckage, or obstacles. Add:

```yaml
skill_checks:
  - skill: parkour
    dc: 12
    trigger: on_enter
    outcomes:
      crit_success:
        message: "You vault the debris with ease."
      success:
        message: "You carefully pick your way through the rubble."
      failure:
        message: "You stumble through the debris, scraping yourself up."
        effect:
          type: damage
          formula: "1d4"
      crit_failure:
        message: "You fall hard into the wreckage."
        effect:
          type: damage
          formula: "2d4"
```

**Step 2: Add a Smooth Talk check to a hostile NPC**

Find an NPC with taunts. Add:

```yaml
skill_checks:
  - skill: smooth_talk
    dc: 16
    trigger: on_greet
    outcomes:
      crit_success:
        message: "They regard you with grudging respect."
      success:
        message: "They seem less hostile than usual."
      failure:
        message: "They sneer at you dismissively."
      crit_failure:
        message: "They take your approach as an insult."
```

**Step 3: Add a Grift check to a locked container item (if one exists)**

```yaml
skill_checks:
  - skill: grift
    dc: 14
    trigger: on_use
    outcomes:
      success:
        message: "The lock clicks. You're in."
      failure:
        message: "The lock holds fast."
        effect:
          type: deny
      crit_failure:
        message: "You snap your pick in the lock. It's jammed now."
        effect:
          type: deny
```

**Step 4: Mark FEATURES.md items complete**

In `docs/requirements/FEATURES.md`, change:
```
- [ ] All the P2FE skills need to be implemented.
```
to:
```
- [x] All the P2FE skills need to be implemented. (framework + automatic triggers complete)
```

Add note: "Skill mechanical effects: automatic proficiency-based checks on room entry, NPC greeting, and item use. Active/passive feat effects deferred to Stage 2."

**Step 5: Commit docs**

```bash
git add content/ docs/requirements/FEATURES.md
git commit -m "content: add sample skill_checks to rooms/NPCs/items; mark skills implemented"
```

**Step 6: Deploy**

```bash
make k8s-redeploy
```

Expected: Helm upgrade completes, pods Running.

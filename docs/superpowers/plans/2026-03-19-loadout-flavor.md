# Loadout Flavor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace generic "technologies prepared" copy with per-tradition lore-flavored language, extend the `loadout` command to show prepared techs, and rework the `rest` flow with tradition-aware prompts.

**Architecture:** A new `TraditionFlavor` struct in `internal/game/technology/flavor.go` maps each tradition to display labels. `RearrangePreparedTechs` gains two new trailing parameters (`sendFn`, `flavor`) to emit tradition-flavored progress messages. `handleLoadout` is extended to combine weapon preset output with prepared tech output when called with no argument.

**Tech Stack:** Go, gRPC, property-based tests via `pgregory.net/rapid`

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/flavor.go` | New: `TraditionFlavor`, `FlavorFor`, `DominantTradition`, `FormatPreparedTechs` |
| `internal/game/technology/flavor_test.go` | New: table-driven tests for all flavor values, job mappings, and `FormatPreparedTechs` |
| `internal/game/command/commands.go` | Add `"prep"` and `"kit"` to loadout command `Aliases` |
| `internal/gameserver/technology_assignment.go` | Add `sendFn func(string)` and `flavor technology.TraditionFlavor` params to `RearrangePreparedTechs`; emit progress messages |
| `internal/gameserver/technology_assignment_test.go` | Update three test call sites to pass `nil, technology.TraditionFlavor{}` |
| `internal/gameserver/grpc_service.go` | Extend `handleLoadout` (no-arg combined view); update `handleRest` to compute flavor + pass sendFn; update rest completion message |

---

## Task 1: `flavor.go` — TraditionFlavor data type and helpers

**Files:**
- Create: `internal/game/technology/flavor.go`
- Create: `internal/game/technology/flavor_test.go`

- [ ] **Step 1: Write failing tests in `flavor_test.go`**

```go
package technology_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/technology"
)

// REQ-LF2: FlavorFor returns correct flavor per tradition.
func TestFlavorFor(t *testing.T) {
    cases := []struct {
        tradition string
        want      technology.TraditionFlavor
    }{
        {"technical", technology.TraditionFlavor{LoadoutTitle: "Field Loadout", PrepVerb: "Configure", SlotNoun: "slot", RestMessage: "Field loadout configured."}},
        {"bio_synthetic", technology.TraditionFlavor{LoadoutTitle: "Chem Kit", PrepVerb: "Mix", SlotNoun: "dose", RestMessage: "Chem kit mixed."}},
        {"neural", technology.TraditionFlavor{LoadoutTitle: "Neural Profile", PrepVerb: "Queue", SlotNoun: "routine", RestMessage: "Neural profile written."}},
        {"fanatic_doctrine", technology.TraditionFlavor{LoadoutTitle: "Doctrine", PrepVerb: "Prepare", SlotNoun: "rite", RestMessage: "Doctrine prepared."}},
        {"", technology.TraditionFlavor{LoadoutTitle: "Loadout", PrepVerb: "Prepare", SlotNoun: "slot", RestMessage: "Technologies prepared."}},
        {"unknown", technology.TraditionFlavor{LoadoutTitle: "Loadout", PrepVerb: "Prepare", SlotNoun: "slot", RestMessage: "Technologies prepared."}},
    }
    for _, tc := range cases {
        t.Run(tc.tradition, func(t *testing.T) {
            assert.Equal(t, tc.want, technology.FlavorFor(tc.tradition))
        })
    }
}

// REQ-LF3: DominantTradition maps job IDs correctly.
func TestDominantTradition(t *testing.T) {
    cases := []struct {
        jobID string
        want  string
    }{
        {"nerd", "technical"},
        {"naturalist", "bio_synthetic"},
        {"drifter", "bio_synthetic"},
        {"schemer", "neural"},
        {"influencer", "neural"},
        {"zealot", "fanatic_doctrine"},
        {"", ""},
        {"unknown", ""},
    }
    for _, tc := range cases {
        t.Run(tc.jobID, func(t *testing.T) {
            assert.Equal(t, tc.want, technology.DominantTradition(tc.jobID))
        })
    }
}

// REQ-LF4: FormatPreparedTechs formats slots correctly.
func TestFormatPreparedTechs(t *testing.T) {
    flavor := technology.FlavorFor("technical") // LoadoutTitle="Field Loadout", SlotNoun="slot"

    t.Run("empty map", func(t *testing.T) {
        got := technology.FormatPreparedTechs(nil, flavor)
        assert.Equal(t, "No Field Loadout configured.", got)
    })

    t.Run("empty slots map", func(t *testing.T) {
        got := technology.FormatPreparedTechs(map[int][]*session.PreparedSlot{}, flavor)
        assert.Equal(t, "No Field Loadout configured.", got)
    })

    t.Run("single level one ready slot", func(t *testing.T) {
        slots := map[int][]*session.PreparedSlot{
            1: {{TechID: "scorching_blast", Expended: false}},
        }
        got := technology.FormatPreparedTechs(slots, flavor)
        want := "[Field Loadout]\n  Level 1 — 1 slot\n    scorching_blast    ready"
        assert.Equal(t, want, got)
    })

    t.Run("single level mixed ready and expended", func(t *testing.T) {
        slots := map[int][]*session.PreparedSlot{
            2: {
                {TechID: "heal_bio_synthetic", Expended: false},
                {TechID: "fear_bio_synthetic", Expended: true},
            },
        }
        got := technology.FormatPreparedTechs(slots, flavor)
        want := "[Field Loadout]\n  Level 2 — 2 slots\n    heal_bio_synthetic    ready\n    fear_bio_synthetic    expended"
        assert.Equal(t, want, got)
    })

    t.Run("multiple levels ascending order", func(t *testing.T) {
        slots := map[int][]*session.PreparedSlot{
            3: {{TechID: "tech_c", Expended: false}},
            1: {{TechID: "tech_a", Expended: false}},
        }
        got := technology.FormatPreparedTechs(slots, flavor)
        require.Contains(t, got, "Level 1")
        require.Contains(t, got, "Level 3")
        // Level 1 must appear before Level 3
        assert.Less(t, indexOf(got, "Level 1"), indexOf(got, "Level 3"))
    })

    t.Run("bio_synthetic flavor uses dose noun", func(t *testing.T) {
        bioFlavor := technology.FlavorFor("bio_synthetic") // SlotNoun="dose"
        slots := map[int][]*session.PreparedSlot{
            1: {
                {TechID: "heal_bio_synthetic", Expended: false},
                {TechID: "fear_bio_synthetic", Expended: false},
            },
        }
        got := technology.FormatPreparedTechs(slots, bioFlavor)
        assert.Contains(t, got, "2 doses")
        assert.Contains(t, got, "[Chem Kit]")
    })
}

func indexOf(s, sub string) int {
    for i := range s {
        if len(s[i:]) >= len(sub) && s[i:i+len(sub)] == sub {
            return i
        }
    }
    return -1
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -run TestFlavorFor -v 2>&1 | head -20`

Expected: compile error — `flavor.go` does not exist yet.

- [ ] **Step 3: Create `flavor.go`**

```go
package technology

import (
    "fmt"
    "sort"
    "strings"

    "github.com/cory-johannsen/mud/internal/game/session"
)

// TraditionFlavor holds the player-facing copy for a technology tradition.
type TraditionFlavor struct {
    LoadoutTitle string
    PrepVerb     string
    SlotNoun     string
    RestMessage  string
}

var fallbackFlavor = TraditionFlavor{
    LoadoutTitle: "Loadout",
    PrepVerb:     "Prepare",
    SlotNoun:     "slot",
    RestMessage:  "Technologies prepared.",
}

var traditionFlavors = map[string]TraditionFlavor{
    "technical": {
        LoadoutTitle: "Field Loadout",
        PrepVerb:     "Configure",
        SlotNoun:     "slot",
        RestMessage:  "Field loadout configured.",
    },
    "bio_synthetic": {
        LoadoutTitle: "Chem Kit",
        PrepVerb:     "Mix",
        SlotNoun:     "dose",
        RestMessage:  "Chem kit mixed.",
    },
    "neural": {
        LoadoutTitle: "Neural Profile",
        PrepVerb:     "Queue",
        SlotNoun:     "routine",
        RestMessage:  "Neural profile written.",
    },
    "fanatic_doctrine": {
        LoadoutTitle: "Doctrine",
        PrepVerb:     "Prepare",
        SlotNoun:     "rite",
        RestMessage:  "Doctrine prepared.",
    },
}

var jobTradition = map[string]string{
    "nerd":        "technical",
    "naturalist":  "bio_synthetic",
    "drifter":     "bio_synthetic",
    "schemer":     "neural",
    "influencer":  "neural",
    "zealot":      "fanatic_doctrine",
}

// FlavorFor returns the TraditionFlavor for the given tradition string.
// For any unknown or empty tradition, the fallback flavor is returned.
func FlavorFor(tradition string) TraditionFlavor {
    if f, ok := traditionFlavors[tradition]; ok {
        return f
    }
    return fallbackFlavor
}

// DominantTradition returns the primary technology tradition for a job ID.
// Returns "" for unknown job IDs.
func DominantTradition(jobID string) string {
    return jobTradition[jobID]
}

// FormatPreparedTechs formats the prepared tech slots grouped by level in
// ascending level order.
//
// Precondition: flavor is a valid TraditionFlavor (zero value produces fallback labels).
// Postcondition: Returns "No <LoadoutTitle> configured." when slots is empty or all levels
// have zero entries; otherwise returns the full formatted display string.
func FormatPreparedTechs(slots map[int][]*session.PreparedSlot, flavor TraditionFlavor) string {
    if len(slots) == 0 {
        return fmt.Sprintf("No %s configured.", flavor.LoadoutTitle)
    }

    levels := make([]int, 0, len(slots))
    for lvl, s := range slots {
        if len(s) > 0 {
            levels = append(levels, lvl)
        }
    }
    if len(levels) == 0 {
        return fmt.Sprintf("No %s configured.", flavor.LoadoutTitle)
    }
    sort.Ints(levels)

    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("[%s]", flavor.LoadoutTitle))

    for _, lvl := range levels {
        s := slots[lvl]
        noun := flavor.SlotNoun
        count := len(s)
        plural := "s"
        if count == 1 {
            plural = ""
        }
        sb.WriteString(fmt.Sprintf("\n  Level %d — %d %s%s", lvl, count, noun, plural))
        for _, slot := range s {
            state := "ready"
            if slot.Expended {
                state = "expended"
            }
            sb.WriteString(fmt.Sprintf("\n    %s    %s", slot.TechID, state))
        }
    }

    return sb.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -v 2>&1 | tail -30`

Expected: All tests PASS including new flavor tests.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/technology/flavor.go internal/game/technology/flavor_test.go
git commit -m "feat: add TraditionFlavor, FlavorFor, DominantTradition, FormatPreparedTechs"
```

---

## Task 2: Add `prep` and `kit` aliases to loadout command

**Files:**
- Modify: `internal/game/command/commands.go:147`

- [ ] **Step 1: Write failing test**

In `internal/game/command/commands_test.go` (or an existing command test file), verify that `"prep"` and `"kit"` resolve to the loadout handler. First check whether a command lookup test already exists:

Run: `cd /home/cjohannsen/src/mud && grep -n 'TestCommands\|TestFind\|TestLookup\|TestResolve' internal/game/command/*_test.go 2>&1 | head -20`

If a command resolution test exists, add cases for `"prep"` and `"kit"`. If none exists, add this to `internal/game/command/commands_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestLoadoutAliases(t *testing.T) {
    reg := command.DefaultRegistry()
    for _, alias := range []string{"lo", "prep", "kit"} {
        t.Run(alias, func(t *testing.T) {
            cmd, ok := reg.Resolve(alias)
            require.True(t, ok, "alias %q not found", alias)
            assert.Equal(t, command.HandlerLoadout, cmd.Handler)
        })
    }
}
```

The correct lookup API is `command.DefaultRegistry().Resolve(alias)` — use it directly.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -run TestLoadoutAliases -v 2>&1`

Expected: FAIL — `"prep"` and `"kit"` not found.

- [ ] **Step 3: Add aliases to `commands.go`**

In `internal/game/command/commands.go` at line 147, change:

```go
{Name: "loadout", Aliases: []string{"lo"}, Help: "Display or swap weapon presets (loadout [1|2])", Category: CategoryCombat, Handler: HandlerLoadout},
```

to:

```go
{Name: "loadout", Aliases: []string{"lo", "prep", "kit"}, Help: "Display or swap weapon presets (loadout [1|2])", Category: CategoryCombat, Handler: HandlerLoadout},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -v 2>&1 | tail -20`

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/command/commands.go
git commit -m "feat: add prep and kit as aliases for loadout command"
```

---

## Task 3: Thread `sendFn` + `flavor` through `RearrangePreparedTechs`

**Files:**
- Modify: `internal/gameserver/technology_assignment.go` (signature + body)
- Modify: `internal/gameserver/technology_assignment_test.go` (three call sites)

- [ ] **Step 1: Update test call sites first (compile-time guard)**

In `internal/gameserver/technology_assignment_test.go`, find ALL call sites (there are four — approximately lines 481, 522, 560, 579). First grep to confirm:

Run: `grep -n 'RearrangePreparedTechs' /home/cjohannsen/src/mud/internal/gameserver/technology_assignment_test.go`

Update every call from:

```go
err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, noPrompt, prep)
// or
err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep)
```

to:

```go
err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, noPrompt, prep, nil, technology.TraditionFlavor{})
// or
err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep, nil, technology.TraditionFlavor{})
```

Add the import for `"github.com/cory-johannsen/mud/internal/game/technology"` in the test file if not already present.

- [ ] **Step 2: Run tests to verify they fail (compile error expected)**

Run: `cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1 | head -20`

Expected: compile error — `RearrangePreparedTechs` signature mismatch.

- [ ] **Step 3: Update `RearrangePreparedTechs` signature and body**

In `internal/gameserver/technology_assignment.go`, change the function signature from:

```go
func RearrangePreparedTechs(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    prepRepo PreparedTechRepo,
) error {
```

to:

```go
func RearrangePreparedTechs(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    prepRepo PreparedTechRepo,
    sendFn func(string),
    flavor technology.TraditionFlavor,
) error {
```

Then add a nil-safe `send` helper immediately after the no-op guard (after `if len(slotsByLevel) == 0 { return nil }`):

```go
send := func(msg string) {
    if sendFn != nil {
        sendFn(msg)
    }
}
```

After the no-op guard and before the `allFixed`/`allPool` aggregation, add the opening message per REQ-LF7:

```go
send(fmt.Sprintf("%sing %s...", flavor.PrepVerb, flavor.LoadoutTitle))
```

Then, in the slot-filling loop, for each fixed slot auto-assignment per REQ-LF8 add (find where fixed slots are assigned — look for the assignment line and add the send call before it):

```go
send(fmt.Sprintf("Level %d, %s %d (fixed): %s", lvl, flavor.SlotNoun, idx, techID))
```

And for each open pool slot prompt per REQ-LF8:

```go
send(fmt.Sprintf("Level %d, %s %d: choose from pool", lvl, flavor.SlotNoun, idx))
```

**Important:** Read the full body of `RearrangePreparedTechs` first to find the exact location of the slot filling loops before making changes. The idx values are 1-based within each level.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestRearrange -v 2>&1 | tail -30`

Expected: All `TestRearrange*` tests PASS.

Run full suite: `cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... ./internal/game/command/... ./internal/gameserver/... 2>&1 | tail -20`

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat: add sendFn and flavor params to RearrangePreparedTechs with progress messages"
```

---

## Task 4: Update `handleRest` and extend `handleLoadout`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing tests for REQ-LF9 and REQ-LF12**

Check whether `internal/gameserver/grpc_service_rest_test.go` exists and covers the rest completion message:

Run: `grep -n 'technologies prepared\|HP restored to maximum' /home/cjohannsen/src/mud/internal/gameserver/grpc_service_rest_test.go 2>/dev/null | head -10`

If the rest completion message test exists, note the test name — it will need updating after Step 2.

For `handleLoadout` combined no-arg display, add a test in `internal/gameserver/grpc_service_test.go` or the most appropriate existing test file (grep for `TestHandleLoadout`). The test should call `handleLoadout` with an empty `arg` on a session with prepared tech slots and assert the response contains both weapon preset output and prepared tech output separated by a blank line. Sketch:

```go
func TestHandleLoadout_NoArg_CombinesWeaponAndPreparedTechs(t *testing.T) {
    // Set up a minimal GameServiceServer with a session that has:
    //   - a non-nil LoadoutSet (so weapon section is non-empty)
    //   - PreparedTechs with at least one slot
    // Call handleLoadout(uid, &gamev1.LoadoutRequest{}) (empty Arg)
    // Assert response contains "\n\n" separating two sections
    // Assert response contains "[" (start of loadout title from FormatPreparedTechs)
}
```

Write the minimal version you can that actually fails before the production code is changed, then proceed.

- [ ] **Step 2: Update the production call site in `handleRest`**

In `grpc_service.go` around line 2509, change:

```go
if err := RearrangePreparedTechs(ctx, sess, sess.CharacterID,
    job, s.techRegistry, promptFn, s.preparedTechRepo,
); err != nil {
```

to:

```go
restFlavor := technology.FlavorFor(technology.DominantTradition(sess.Class))
sendFn := func(text string) {
    _ = sendMsg(text)
}
if err := RearrangePreparedTechs(ctx, sess, sess.CharacterID,
    job, s.techRegistry, promptFn, s.preparedTechRepo,
    sendFn, restFlavor,
); err != nil {
```

Add import `"github.com/cory-johannsen/mud/internal/game/technology"` to the import block if not already present.

- [ ] **Step 3: Update rest completion message per REQ-LF9**

Change:

```go
return sendMsg("You finish your rest. HP restored to maximum and technologies prepared.")
```

to:

```go
return sendMsg(fmt.Sprintf("You finish your rest. HP restored to maximum. %s", restFlavor.RestMessage))
```

Note: `restFlavor` must be declared before the `RearrangePreparedTechs` call (done in step 2). Ensure the variable is in scope at the `sendMsg` line.

- [ ] **Step 4: Extend `handleLoadout` per REQ-LF12**

Change `handleLoadout` from:

```go
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("player not found"), nil
    }
    return messageEvent(command.HandleLoadout(sess, req.GetArg())), nil
}
```

to:

```go
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("player not found"), nil
    }
    arg := req.GetArg()
    if arg != "" {
        return messageEvent(command.HandleLoadout(sess, arg)), nil
    }
    flavor := technology.FlavorFor(technology.DominantTradition(sess.Class))
    weaponSection := command.HandleLoadout(sess, "")
    prepSection := technology.FormatPreparedTechs(sess.PreparedTechs, flavor)
    return messageEvent(weaponSection + "\n\n" + prepSection), nil
}
```

- [ ] **Step 5: Run full test suite**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... ./internal/game/command/... ./internal/gameserver/... 2>&1 | tail -30`

Expected: All tests PASS.

- [ ] **Step 6: Mark feature checkbox in docs**

In `docs/features/technology.md`, find the unchecked item:

```
      - [ ] Spellbook/memorization needs to be mapped to a lore-friendly analog that preserves the underlying mechanic
```

Change it to:

```
      - [x] Spellbook/memorization needs to be mapped to a lore-friendly analog that preserves the underlying mechanic
```

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go docs/features/technology.md
git commit -m "feat: tradition-flavored rest flow and combined loadout display"
```

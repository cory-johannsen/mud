# Skill Check Stage 2 — condition & reveal Effects

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Wire the `condition` and `reveal` skill check effect types so that skill check outcomes can apply named conditions to the player and permanently expose hidden room exits.

**Architecture:** `condition` effects require a `*condition.ActiveSet` on `PlayerSession` (initialized at login); `applySkillCheckEffect` looks up the `ConditionDef` by `effect.ID` from `s.condRegistry` and calls `ActiveSet.Apply`. `reveal` effects toggle `Exit.Hidden = false` on the live `*world.Room` returned by `world.Manager` — this is a permanent in-memory mutation visible to all players in the zone (correct game semantics; persisted YAML is not modified).

**Tech Stack:** Go, `pgregory.net/rapid` for property tests, existing `condition.ActiveSet` / `condition.Registry`, `world.Manager.GetRoom`

---

## Background & Key Facts

- **`condition.ActiveSet`** — `internal/game/condition/active.go`. Method: `Apply(uid string, def *ConditionDef, stacks, duration int) error`. Stacks cap at `def.MaxStacks`; duration `-1` = permanent.
- **`condition.Registry`** — `Get(id string) (*ConditionDef, bool)`. Already on `GameServiceServer` as `s.condRegistry`.
- **`PlayerSession`** — `internal/game/session/manager.go`. Currently has **no** `Conditions` field. Add `Conditions *condition.ActiveSet`.
- **`condition.NewActiveSet()`** — check `internal/game/condition/active.go` for the constructor name.
- **`Exit.Hidden bool`** — field on `world.Exit` struct in `internal/game/world/model.go`. `Room.VisibleExits()` already filters hidden exits.
- **`world.Manager.GetRoom(id string)`** — returns `*world.Room` (pointer to live room). Mutating `Exit.Hidden = false` is immediately visible to all callers.
- **`applySkillCheckEffect(sess *session.PlayerSession, effect *skillcheck.Effect)`** — `internal/gameserver/grpc_service.go`. Currently only handles `"damage"`. Called from `applyRoomSkillChecks` (line ~906), `applyNPCSkillChecks` (line ~973), and `handleUseEquipment` (line ~2331). **All three callers also pass `uid` implicitly through `sess.UID`** — note that `sess.UID` holds the character UID string.
- **Effect struct** — `internal/game/skillcheck/types.go`: `Type string`, `Formula string`, `ID string` (condition id), `Target string` (exit direction for reveal).
- **Content YAML for conditions** — `content/conditions/*.yaml`. Format: see `content/conditions/prone.yaml`. `duration_type: "permanent"` / `"rounds"` / `"until_save"`.
- **Condition effect in YAML trigger**: `effect: { type: condition, id: distrusted }`. The `distrusted` condition must exist in `content/conditions/distrusted.yaml`.
- **Reveal effect in YAML trigger**: `effect: { type: reveal, target: "north" }`. `target` is the exit direction to un-hide (matches `Exit.Direction` string e.g. `"north"`, `"east"`, etc.).
- **`world.Direction` type** — find how directions are represented; exits may store direction as a `Direction` type with a `String()` method or as a plain string.

---

## Task 1: Add `Conditions *condition.ActiveSet` to `PlayerSession`

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go` (initialize in `Session()`)
- Test: `internal/game/session/manager_test.go`

**Context:** `PlayerSession` currently carries no `ActiveSet`. Conditions fired by skill checks (e.g., `distrusted` applied by a smooth-talk failure) must persist on the session for the duration of the game session. Initialize the `ActiveSet` in `Session()` immediately after the session is created.

**Step 1: Write the failing test**

In `internal/game/session/manager_test.go`, add:

```go
func TestPlayerSession_HasConditionsField(t *testing.T) {
    sess := &session.PlayerSession{}
    assert.NotNil(t, sess) // compiles only if Conditions field exists
    // Conditions may be nil until initialized — just confirm the field exists
    _ = sess.Conditions
}
```

Run:
```bash
go test -run TestPlayerSession_HasConditionsField -v ./internal/game/session/...
```
Expected: FAIL — compile error `sess.Conditions undefined`

**Step 2: Add field to `PlayerSession`**

In `internal/game/session/manager.go`, find the `PlayerSession` struct. Import `"github.com/cory-johannsen/mud/internal/game/condition"`. Add after the `Skills` field:

```go
// Conditions tracks active conditions applied outside of combat (e.g., from skill check effects).
// Initialized at login; nil before Session() runs.
Conditions *condition.ActiveSet
```

**Step 3: Run test to verify it passes**

```bash
go test -run TestPlayerSession_HasConditionsField -v ./internal/game/session/...
```
Expected: PASS

**Step 4: Initialize `Conditions` in `Session()`**

In `internal/gameserver/grpc_service.go`, find the `Session()` function. Find where `sess` is created (via `s.sessions.AddPlayer(...)` or similar). After the session is created and before `sess.Skills` is populated, add:

```go
// Initialize out-of-combat condition tracking.
sess.Conditions = condition.NewActiveSet()
```

Check `internal/game/condition/active.go` for the exact constructor name — it may be `NewActiveSet()`, `New()`, or a bare struct literal `&condition.ActiveSet{}`. Use whichever is idiomatic.

**Step 5: Add property test (SWENG-5a)**

In `internal/game/session/manager_test.go`, add:

```go
func TestProperty_PlayerSession_ConditionsField_NotNilAfterInit(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        sess := &session.PlayerSession{
            Conditions: condition.NewActiveSet(), // or &condition.ActiveSet{}
        }
        assert.NotNil(t, sess.Conditions)
    })
}
```

**Step 6: Run full fast suite**

```bash
go test ./internal/game/... ./internal/gameserver/... ./internal/frontend/...
```
Expected: all pass

**Step 7: Commit**

```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go internal/game/session/manager_test.go
git commit -m "feat: add Conditions ActiveSet to PlayerSession; initialize at login"
```

---

## Task 2: Wire `condition` effect in `applySkillCheckEffect`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_test.go`
- Create: `content/conditions/distrusted.yaml`

**Context:** `applySkillCheckEffect` currently handles only `"damage"`. Extend it to handle `"condition"`: look up `effect.ID` in `s.condRegistry`, and if found, call `sess.Conditions.Apply(sess.UID, def, 1, -1)`.

**Step 1: Create the `distrusted` condition**

Create `content/conditions/distrusted.yaml`:

```yaml
id: distrusted
name: Distrusted
description: |
  This character regards you with open suspicion. Social interactions with
  them are harder. You take a -2 penalty to Smooth Talk checks against this
  NPC for the remainder of the session.
duration_type: permanent
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Write the failing test**

In `internal/gameserver/grpc_service_test.go`, add a test that:
- Creates a server with a `condRegistry` containing the `distrusted` condition
- Builds an `Effect{Type: "condition", ID: "distrusted"}`
- Calls `applySkillCheckEffect(sess, effect)` where `sess.Conditions` is initialized
- Asserts `sess.Conditions.Has("distrusted")` returns true

Look at the existing test helpers (e.g., `newTestServer(...)`) to understand how to build a minimal server with a condRegistry. You may need to build a `condition.Registry` from a YAML byte slice:

```go
// Minimal registry for tests — build from YAML string, not file
reg, err := condition.NewRegistry([]*condition.ConditionDef{
    {ID: "distrusted", Name: "Distrusted", DurationType: "permanent", MaxStacks: 0},
})
```

Check `internal/game/condition/definition.go` and registry for the correct constructor. If `condition.NewRegistry` doesn't exist, use `condition.LoadDirectory` with a temp dir or build the struct directly.

Run:
```bash
go test -run TestApplySkillCheckEffect_Condition -v ./internal/gameserver/...
```
Expected: FAIL — condition effect not handled

**Step 3: Implement `condition` handling in `applySkillCheckEffect`**

In `internal/gameserver/grpc_service.go`, find `applySkillCheckEffect`. Extend it:

```go
func (s *GameServiceServer) applySkillCheckEffect(sess *session.PlayerSession, effect *skillcheck.Effect) {
    if effect == nil {
        return
    }
    switch effect.Type {
    case "damage":
        if effect.Formula == "" || s.dice == nil {
            return
        }
        dmg, err := s.dice.RollExpr(effect.Formula)
        if err != nil {
            s.logger.Warn("skill check damage formula error",
                zap.String("formula", effect.Formula),
                zap.Error(err),
            )
            return
        }
        sess.CurrentHP -= dmg.Total()
        if sess.CurrentHP < 0 {
            sess.CurrentHP = 0
        }
    case "condition":
        if effect.ID == "" || sess.Conditions == nil || s.condRegistry == nil {
            return
        }
        def, ok := s.condRegistry.Get(effect.ID)
        if !ok {
            s.logger.Warn("skill check condition not found", zap.String("condition_id", effect.ID))
            return
        }
        if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
            s.logger.Warn("skill check condition apply failed",
                zap.String("condition_id", effect.ID),
                zap.Error(err),
            )
        }
    }
}
```

**Step 4: Run test to verify it passes**

```bash
go test -run TestApplySkillCheckEffect_Condition -v ./internal/gameserver/...
```
Expected: PASS

**Step 5: Add property test (SWENG-5a)**

Add a property test verifying: for any known condition ID, `applySkillCheckEffect` with `{Type: "condition", ID: id}` results in `sess.Conditions.Has(id) == true`.

```go
func TestProperty_ApplySkillCheckEffect_ConditionAlwaysApplied(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        id := rapid.SampledFrom([]string{"distrusted", "prone", "flat_footed"}).Draw(rt, "condition_id")
        // build registry with that condition
        // call applySkillCheckEffect
        // assert sess.Conditions.Has(id)
    })
}
```

**Step 6: Run full fast suite**

```bash
go test ./internal/game/... ./internal/gameserver/...
```
Expected: all pass

**Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go content/conditions/distrusted.yaml
git commit -m "feat: wire condition effect in applySkillCheckEffect; add distrusted condition"
```

---

## Task 3: Wire `reveal` effect — expose hidden room exits

**Files:**
- Modify: `internal/game/world/manager.go` (add `RevealExit` method)
- Modify: `internal/gameserver/grpc_service.go` (`applySkillCheckEffect` + signature change)
- Test: `internal/game/world/manager_test.go` or `loader_test.go`
- Test: `internal/gameserver/grpc_service_test.go`

**Context:** `reveal` effect un-hides a room exit permanently (in-memory world state mutation). The `Effect.Target` field holds the exit direction string (e.g., `"north"`). `applySkillCheckEffect` needs access to the `roomID` to find the room — currently it only receives `sess` and `effect`. The method signature must be extended.

**Step 1: Find how `Direction` is stored on `Exit`**

Read `internal/game/world/model.go`. Check if `Exit.Direction` is type `Direction` (a named type) or `string`. Check if `Direction` has a `String()` method. The YAML `target` field will be a plain lowercase string like `"north"`, `"east"`, etc.

**Step 2: Add `RevealExit(roomID, direction string) bool` to world.Manager**

Find `internal/game/world/manager.go`. Add:

```go
// RevealExit un-hides the exit in the given direction from the specified room.
// Precondition: roomID must be a valid room ID; direction is lowercase (e.g., "north").
// Postcondition: if the exit exists and was hidden, it is now visible to all players.
// Returns true if an exit was found and revealed; false if not found or already visible.
func (m *Manager) RevealExit(roomID, direction string) bool {
    room, ok := m.GetRoom(roomID)
    if !ok {
        return false
    }
    for i := range room.Exits {
        if strings.EqualFold(room.Exits[i].Direction.String(), direction) {
            if room.Exits[i].Hidden {
                room.Exits[i].Hidden = false
                return true
            }
            return false // already visible
        }
    }
    return false
}
```

Check whether `Exit.Direction` is a `Direction` type or `string` — adjust the comparison accordingly. If `Direction` is a plain `string`, use `strings.EqualFold(room.Exits[i].Direction, direction)`.

**Step 3: Write failing tests for `RevealExit`**

In the world test file (find it with `grep -rn "func Test" internal/game/world/`), add:

```go
func TestRevealExit_HidesExitBecomesVisible(t *testing.T) {
    // Build a zone with one room that has a hidden north exit
    // Call RevealExit(roomID, "north")
    // Assert the exit is now visible (Hidden == false)
    // Assert VisibleExits() includes it
}

func TestRevealExit_NonExistent_ReturnsFalse(t *testing.T) {
    // Call RevealExit on a non-existent room or direction
    // Assert returns false
}

func TestRevealExit_AlreadyVisible_ReturnsFalse(t *testing.T) {
    // Call RevealExit on a visible exit (Hidden == false)
    // Assert returns false
}

func TestProperty_RevealExit_VisibleExitsCountNeverDecreases(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Build a room with some hidden and some visible exits
        // Call RevealExit
        // Assert len(room.VisibleExits()) >= before
    })
}
```

Run:
```bash
go test -run TestRevealExit -v ./internal/game/world/...
```
Expected: FAIL — `RevealExit` undefined

**Step 4: Verify `RevealExit` implementation passes tests**

```bash
go test -run TestRevealExit -v ./internal/game/world/...
```
Expected: PASS

**Step 5: Extend `applySkillCheckEffect` signature to accept `roomID`**

Change the signature to:
```go
func (s *GameServiceServer) applySkillCheckEffect(sess *session.PlayerSession, effect *skillcheck.Effect, roomID string)
```

Update all 3 call sites to pass `roomID`:
- In `applyRoomSkillChecks(uid, room)`: pass `room.ID`
- In `applyNPCSkillChecks(uid, roomID)`: pass the `roomID` parameter (already available — the room where the NPC is)
- In `handleUseEquipment`: pass the current room ID (find it from the session or from the room lookup already done in that function)

Add `reveal` handling to the switch:

```go
case "reveal":
    if effect.Target == "" || roomID == "" {
        return
    }
    revealed := s.world.RevealExit(roomID, effect.Target)
    if !revealed {
        s.logger.Debug("skill check reveal: no hidden exit found",
            zap.String("room_id", roomID),
            zap.String("direction", effect.Target),
        )
    }
```

**Step 6: Write test for `reveal` effect in `applySkillCheckEffect`**

In `grpc_service_test.go`, add:

```go
func TestApplySkillCheckEffect_Reveal_UnhidesExit(t *testing.T) {
    // Set up a world with a room that has a hidden north exit
    // Call applySkillCheckEffect(sess, &skillcheck.Effect{Type:"reveal", Target:"north"}, roomID)
    // Assert room.VisibleExits() now includes the north exit
}
```

**Step 7: Run full fast suite**

```bash
go test ./internal/game/... ./internal/gameserver/...
```
Expected: all pass

**Step 8: Commit**

```bash
git add internal/game/world/manager.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat: wire reveal effect in applySkillCheckEffect; add RevealExit to world.Manager"
```

---

## Task 4: Sample content + FEATURES.md + deploy

**Files:**
- Modify: an NPC YAML (`content/npcs/`) — add `condition` effect to on_greet failure
- Modify: a zone YAML (`content/zones/`) — add hidden exit + `reveal` effect on success
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Add `condition` effect to ganger NPC**

Open `content/npcs/ganger.yaml`. Find the existing `skill_checks:` block (added in Stage 1). Update the failure outcome to apply `distrusted`:

```yaml
skill_checks:
  - skill: smooth_talk
    dc: 16
    trigger: on_greet
    outcomes:
      crit_success:
        message: "The ganger regards you with grudging respect."
      success:
        message: "The ganger seems less hostile than usual."
      failure:
        message: "The ganger sneers at you dismissively."
        effect:
          type: condition
          id: distrusted
      crit_failure:
        message: "The ganger takes your approach as a personal insult."
        effect:
          type: condition
          id: distrusted
```

**Step 2: Add a hidden exit with `reveal` trigger to a room**

Find a room in `content/zones/felony_flats.yaml` (or another zone) that could plausibly have a hidden passage. Add a hidden exit to that room:

```yaml
exits:
  - direction: north
    target_room: <existing_connected_room>
  - direction: east
    target_room: <some_room_id>
    hidden: true   # concealed alley entrance
```

Then add a skill check trigger that reveals it on success:

```yaml
skill_checks:
  - skill: savvy          # or "spot" if a spot skill exists
    dc: 14
    trigger: on_enter
    outcomes:
      crit_success:
        message: "You notice a narrow gap between the dumpsters — a hidden alley leads east."
        effect:
          type: reveal
          target: east
      success:
        message: "Something catches your eye — there's a concealed passage to the east."
        effect:
          type: reveal
          target: east
      failure:
        message: "Nothing unusual catches your attention."
      crit_failure:
        message: "You're completely oblivious to your surroundings."
```

Use a skill that exists in `content/skills.yaml` (read the file to check available skill IDs).

**Step 3: Mark Stage 2 complete in FEATURES.md**

In `docs/requirements/FEATURES.md`, find the Stage 2 section and mark subitems complete:

```markdown
- [x] **Skill check triggers — Stage 2: extended effects**
  - [x] `condition` effect type wired to the condition system (`ActiveSet.Apply`)
  - [x] `reveal` effect type exposes hidden room exits
```

**Step 4: Verify build**

```bash
cd /home/cjohannsen/src/mud && make build-gameserver 2>&1 | tail -5
```
Must succeed.

**Step 5: Run full fast test suite**

```bash
go test ./internal/game/... ./internal/gameserver/... ./internal/frontend/...
```
All must pass.

**Step 6: Commit**

```bash
git add content/ docs/requirements/FEATURES.md
git commit -m "feat: Stage 2 sample content (condition+reveal effects); mark FEATURES.md complete"
```

**Step 7: Deploy**

```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy 2>&1 | tail -10
```

Report Helm revision number.

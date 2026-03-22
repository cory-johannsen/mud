# Passive Innate Tech Mechanics (Seismic Sense) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `seismic_sense` reveal all creatures in the current room (including hidden) to the owning player automatically on room entry/exit, without requiring the `use` command.

**Architecture:** Add a `tremorsense` EffectType and a `RoomQuerier` interface to the effect resolver; thread a `querier RoomQuerier` parameter through `activateTechWithEffects` → `ResolveTechEffects` → `applyEffect`; implement `CreaturesInRoom` on `GameServiceServer`; pass `s` from `triggerPassiveTechsForRoom` so the passive trigger populates the room creature list.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based tests), YAML, `go test ./internal/game/technology/... ./internal/gameserver/...`

**Spec:** `docs/superpowers/specs/2026-03-19-passive-innate-tech-design.md`

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `tremorsense` to `EffectType` constants and `validEffectTypes` map |
| `content/technologies/innate/seismic_sense.yaml` | Change `type: utility` → `type: tremorsense` |
| `internal/gameserver/tech_effect_resolver.go` | Add `RoomQuerier`, `CreatureInfo`, `FormatTremorsenseOutput`; thread `querier` through `ResolveTechEffects` + `applyEffect`; remove `"No effect."` guard; filter empty strings; update godoc |
| `internal/gameserver/grpc_service.go` | Add `querier` param to `activateTechWithEffects`; implement `CreaturesInRoom`; wire `triggerPassiveTechsForRoom` |
| `internal/gameserver/tech_effect_resolver_test.go` | PBT invariant + table-driven tests for `FormatTremorsenseOutput` |
| `internal/gameserver/grpc_service_passive_test.go` | Integration tests using `mockRoomQuerier` |

---

## Task 1: Add `tremorsense` EffectType

**Files:**
- Modify: `internal/game/technology/model.go`

- [ ] **Step 1: Open model.go and find the EffectType constants and validEffectTypes map**

  The `EffectType` constants are around line 57, `validEffectTypes` around line 70. Confirm with:
  ```bash
  grep -n "tremorsense\|EffectType\|validEffectTypes\|utility" internal/game/technology/model.go | head -20
  ```

- [ ] **Step 2: Add `tremorsense` constant**

  In the `EffectType` const block (after `EffectTypeUtility`), add:
  ```go
  EffectTypeTremorsense EffectType = "tremorsense"
  ```

- [ ] **Step 3: Add `tremorsense` to `validEffectTypes`**

  In the `validEffectTypes` map, add:
  ```go
  EffectTypeTremorsense: true,
  ```

- [ ] **Step 4: Build to verify no compile errors**

  ```bash
  go build ./internal/game/technology/...
  ```
  Expected: no output (success)

- [ ] **Step 5: Run technology tests**

  ```bash
  go test ./internal/game/technology/... -v 2>&1 | tail -20
  ```
  Expected: all PASS

- [ ] **Step 6: Commit**

  ```bash
  git add internal/game/technology/model.go
  git commit -m "feat: add tremorsense EffectType constant and validation entry"
  ```

---

## Task 2: Update seismic_sense.yaml

**Files:**
- Modify: `content/technologies/innate/seismic_sense.yaml`

- [ ] **Step 1: Update the effect type**

  Change the `on_apply` effect from `type: utility` to `type: tremorsense`:
  ```yaml
  id: seismic_sense
  name: Seismic Sense
  description: Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.
  tradition: technical
  level: 1
  usage_type: innate
  action_cost: 0
  passive: true
  range: zone
  targets: single
  duration: rounds:1
  resolution: none
  effects:
    on_apply:
      - type: tremorsense
        description: "Your bone-conduction implants detect ground vibrations. You sense the movement of all creatures in the room through the floor."
  ```

- [ ] **Step 2: Verify YAML loads without error**

  The technology package tests load all YAML files and validate effect types. Build + test to confirm `tremorsense` is accepted:
  ```bash
  go build ./internal/game/technology/... && go test ./internal/game/technology/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | tail -10
  ```
  Expected: all `ok`, no `FAIL`

- [ ] **Step 3: Commit**

  ```bash
  git add content/technologies/innate/seismic_sense.yaml
  git commit -m "feat: update seismic_sense to use tremorsense effect type"
  ```

---

## Task 3: Add FormatTremorsenseOutput (TDD)

**Files:**
- Modify: `internal/gameserver/tech_effect_resolver.go`
- Modify: `internal/gameserver/tech_effect_resolver_test.go`

- [ ] **Step 1: Write failing table-driven tests for `FormatTremorsenseOutput`**

  Add to `internal/gameserver/tech_effect_resolver_test.go`:
  ```go
  func TestFormatTremorsenseOutput_TableDriven(t *testing.T) {
      cases := []struct {
          name     string
          input    []CreatureInfo
          expected string
      }{
          {
              name:     "empty slice returns no-creatures message",
              input:    []CreatureInfo{},
              expected: "[Seismic Sense] No creatures detected.",
          },
          {
              name:     "single visible creature",
              input:    []CreatureInfo{{Name: "Guard", Hidden: false}},
              expected: "[Seismic Sense] Creatures detected in this room: Guard",
          },
          {
              name:     "single hidden creature",
              input:    []CreatureInfo{{Name: "Assassin", Hidden: true}},
              expected: "[Seismic Sense] Creatures detected in this room: Assassin (concealed)",
          },
          {
              name: "mixed visible and hidden",
              input: []CreatureInfo{
                  {Name: "Guard", Hidden: false},
                  {Name: "Assassin", Hidden: true},
                  {Name: "you", Hidden: false},
              },
              expected: "[Seismic Sense] Creatures detected in this room: Guard, Assassin (concealed), you",
          },
      }
      for _, tc := range cases {
          t.Run(tc.name, func(t *testing.T) {
              got := FormatTremorsenseOutput(tc.input)
              assert.Equal(t, tc.expected, got)
          })
      }
  }
  ```

- [ ] **Step 2: Write failing PBT for `FormatTremorsenseOutput`**

  Add to `internal/gameserver/tech_effect_resolver_test.go`:
  ```go
  func genCreatureInfo(t *rapid.T) CreatureInfo {
      return CreatureInfo{
          Name:   rapid.StringN(1, 20, -1).Draw(t, "name"),
          Hidden: rapid.Bool().Draw(t, "hidden"),
      }
  }

  func TestProperty_FormatTremorsenseOutput_HiddenSuffix(t *testing.T) {
      rapid.Check(t, func(t *rapid.T) {
          creatures := rapid.SliceOf(rapid.Custom(genCreatureInfo), rapid.MinLen(1)).Draw(t, "creatures")
          output := FormatTremorsenseOutput(creatures)
          for _, c := range creatures {
              if c.Hidden {
                  assert.Contains(t, output, c.Name+" (concealed)",
                      "hidden creature %q must appear with (concealed) suffix", c.Name)
              } else {
                  // visible entry should appear without (concealed) suffix
                  assert.Contains(t, output, c.Name)
                  assert.NotContains(t, output, c.Name+" (concealed)")
              }
          }
      })
  }
  ```

- [ ] **Step 3: Run tests to verify they fail**

  ```bash
  go test ./internal/gameserver/... -run "TestFormatTremorsenseOutput|TestProperty_FormatTremorsenseOutput" -v 2>&1 | tail -10
  ```
  Expected: FAIL with "undefined: FormatTremorsenseOutput" or "undefined: CreatureInfo"

- [ ] **Step 4: Add `RoomQuerier`, `CreatureInfo`, and `FormatTremorsenseOutput` to tech_effect_resolver.go**

  The `"strings"` package is already imported in `tech_effect_resolver.go` — no import change needed.

  Add near the top of `internal/gameserver/tech_effect_resolver.go` (after imports, before `ResolveTechEffects`):
  ```go
  // RoomQuerier provides creature presence information for a room.
  // The sensingUID identifies the player activating the tremorsense effect,
  // whose entry is returned as CreatureInfo{Name: "you"}.
  type RoomQuerier interface {
      CreaturesInRoom(roomID, sensingUID string) []CreatureInfo
  }

  // CreatureInfo describes a creature present in a room for tremorsense output.
  type CreatureInfo struct {
      Name   string
      Hidden bool
  }

  // FormatTremorsenseOutput formats a []CreatureInfo into a [Seismic Sense] message.
  // Hidden creatures are suffixed with " (concealed)".
  // Returns a no-creatures message if the slice is empty.
  func FormatTremorsenseOutput(creatures []CreatureInfo) string {
      if len(creatures) == 0 {
          return "[Seismic Sense] No creatures detected."
      }
      parts := make([]string, len(creatures))
      for i, c := range creatures {
          if c.Hidden {
              parts[i] = c.Name + " (concealed)"
          } else {
              parts[i] = c.Name
          }
      }
      return "[Seismic Sense] Creatures detected in this room: " + strings.Join(parts, ", ")
  }
  ```

- [ ] **Step 5: Run tests to verify they pass**

  ```bash
  go test ./internal/gameserver/... -run "TestFormatTremorsenseOutput|TestProperty_FormatTremorsenseOutput" -v 2>&1 | tail -15
  ```
  Expected: all PASS

- [ ] **Step 6: Commit**

  ```bash
  git add internal/gameserver/tech_effect_resolver.go internal/gameserver/tech_effect_resolver_test.go
  git commit -m "feat: add RoomQuerier, CreatureInfo, FormatTremorsenseOutput with tests"
  ```

---

## Task 4: Thread querier through ResolveTechEffects and applyEffect

**Files:**
- Modify: `internal/gameserver/tech_effect_resolver.go`
- Modify: `internal/gameserver/tech_effect_resolver_test.go`

- [ ] **Step 1: Write failing test for nil-querier tremorsense in applyEffect**

  Add to `internal/gameserver/tech_effect_resolver_test.go`:
  ```go
  func TestResolveTechEffects_TremorsenseNilQuerier_ReturnsEmpty(t *testing.T) {
      sess := &session.PlayerSession{UID: "u1", RoomID: "room1"}
      tech := &technology.TechnologyDef{
          ID:         "seismic_sense",
          Passive:    true,
          ActionCost: 0,
          Resolution: "",
          Effects: technology.TieredEffects{
              OnApply: []technology.TechEffect{
                  {Type: technology.EffectTypeTremorsense},
              },
          },
      }
      msgs := ResolveTechEffects(sess, tech, nil, nil, nil, deterministicSrc{val: 1}, nil)
      assert.Empty(t, msgs, "nil querier tremorsense should produce no messages")
  }

  func TestResolveTechEffects_TremorsenseWithQuerier_ReturnsCreatureList(t *testing.T) {
      sess := &session.PlayerSession{UID: "u1", RoomID: "room1"}
      tech := &technology.TechnologyDef{
          ID:         "seismic_sense",
          Passive:    true,
          ActionCost: 0,
          Resolution: "",
          Effects: technology.TieredEffects{
              OnApply: []technology.TechEffect{
                  {Type: technology.EffectTypeTremorsense},
              },
          },
      }
      q := &mockRoomQuerier{creatures: []CreatureInfo{
          {Name: "Guard", Hidden: false},
          {Name: "you", Hidden: false},
      }}
      msgs := ResolveTechEffects(sess, tech, nil, nil, nil, deterministicSrc{val: 1}, q)
      require.Len(t, msgs, 1)
      assert.Equal(t, "[Seismic Sense] Creatures detected in this room: Guard, you", msgs[0])
  }
  ```

  **Note on `mockRoomQuerier`:** Both `tech_effect_resolver_test.go` and `grpc_service_passive_test.go` are in `package gameserver`. A type can only be defined once per package. Define `mockRoomQuerier` here (in `tech_effect_resolver_test.go`) so it is available when the resolver tests in Task 4 compile. Task 6's integration tests will use it from here — do NOT redefine it in `grpc_service_passive_test.go`.

  Add to `tech_effect_resolver_test.go`:
  ```go
  type mockRoomQuerier struct{ creatures []CreatureInfo }
  func (m *mockRoomQuerier) CreaturesInRoom(_, _ string) []CreatureInfo { return m.creatures }
  ```

- [ ] **Step 2: Run tests to verify they fail**

  ```bash
  go test ./internal/gameserver/... -run "TestResolveTechEffects_Tremorsense" -v 2>&1 | tail -10
  ```
  Expected: FAIL (compile error — querier param not yet added)

- [ ] **Step 3: Update `ResolveTechEffects` signature, remove fallback guards, update godoc**

  The current signature (line 38):
  ```go
  func ResolveTechEffects(sess *session.PlayerSession, tech *technology.TechnologyDef, targets []*combat.Combatant, cbt *combat.Combat, condRegistry *condition.Registry, src combat.Source) []string {
  ```
  Change to:
  ```go
  func ResolveTechEffects(sess *session.PlayerSession, tech *technology.TechnologyDef, targets []*combat.Combatant, cbt *combat.Combat, condRegistry *condition.Registry, src combat.Source, querier RoomQuerier) []string {
  ```

  Update the godoc postcondition (line 35) from `//   - Returns at least one message.` to `//   - Returns zero or more non-empty messages.`

  Remove ALL THREE fallback guards from `ResolveTechEffects`:

  **Guard 1** (lines 48–50): remove:
  ```go
  if len(msgs) == 0 {
      msgs = append(msgs, "No effect.")
  }
  ```

  **Guard 2** (lines 72–77): remove:
  ```go
  if len(effectMsgs) == 0 {
      if label != "" {
          msgs = append(msgs, label+"No effect.")
      } else {
          msgs = append(msgs, "No effect.")
      }
  }
  ```
  The outer `else` block (lines 78–86) that appends labelled effectMsgs is kept.

  **Guard 3** (lines 88–90): remove:
  ```go
  if len(msgs) == 0 {
      msgs = append(msgs, "Nothing happens.")
  }
  ```

  Both `return msgs` statements (line 51 and line 91) stay as-is. `applyEffects` already filters empty strings internally (line 185–187 in the current file), so `msgs` is already clean — no additional filter pass is needed.

- [ ] **Step 4: Pass `querier` through to `applyEffect`**

  Find `applyEffects` (lines 174–190). It currently filters empty strings internally (the `if msg != "" { msgs = append(msgs, msg) }` guard). Keep that filter. Just add `querier RoomQuerier` as the last parameter and pass it to `applyEffect`:
  ```go
  func applyEffects(
      sess *session.PlayerSession,
      effects []technology.TechEffect,
      target *combat.Combatant,
      cbt *combat.Combat,
      condRegistry *condition.Registry,
      src combat.Source,
      querier RoomQuerier,
  ) []string {
      var msgs []string
      for _, e := range effects {
          msg := applyEffect(sess, e, target, cbt, condRegistry, src, querier)
          if msg != "" {
              msgs = append(msgs, msg)
          }
      }
      return msgs
  }
  ```

  Update `applyEffect` signature:
  ```go
  func applyEffect(sess *session.PlayerSession, e technology.TechEffect, target *combat.Combatant, cbt *combat.Combat, condRegistry *condition.Registry, src combat.Source, querier RoomQuerier) string {
  ```

  Add the tremorsense case to the `switch e.Type` in `applyEffect`:
  ```go
  case technology.EffectTypeTremorsense:
      if querier == nil {
          return ""
      }
      creatures := querier.CreaturesInRoom(sess.RoomID, sess.UID)
      return FormatTremorsenseOutput(creatures)
  ```

  Find and update all `applyEffects(...)` call sites to pass `querier`. There are exactly two call sites in `ResolveTechEffects` (lines 47 and 71 in the current file). Confirm no other callers exist outside this file:
  ```bash
  grep -rn "applyEffects(" internal/gameserver/ --include="*.go"
  ```
  Update both call sites to append `, querier` as the final argument.

- [ ] **Step 5: Fix all existing `ResolveTechEffects` call sites in grpc_service.go to pass `nil`**

  Find all call sites:
  ```bash
  grep -n "ResolveTechEffects" internal/gameserver/grpc_service.go
  ```
  There is exactly one call site (inside `activateTechWithEffects`):
  ```go
  msgs := ResolveTechEffects(sess, techDef, techTargets, cbt, s.condRegistry, globalRandSrc{})
  ```
  Change to:
  ```go
  msgs := ResolveTechEffects(sess, techDef, techTargets, cbt, s.condRegistry, globalRandSrc{}, nil)
  ```

- [ ] **Step 6: Build to verify**

  ```bash
  go build ./internal/gameserver/...
  ```
  Expected: no output (success)

- [ ] **Step 7: Run tests**

  ```bash
  go test ./internal/gameserver/... -run "TestResolveTechEffects_Tremorsense" -v 2>&1 | tail -15
  ```
  Expected: all PASS

- [ ] **Step 8: Run full test suite to check no regressions**

  ```bash
  go test ./internal/gameserver/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | tail -30
  ```
  Expected: all PASS

- [ ] **Step 9: Commit**

  ```bash
  git add internal/gameserver/tech_effect_resolver.go internal/gameserver/tech_effect_resolver_test.go internal/gameserver/grpc_service.go
  git commit -m "feat: thread RoomQuerier through ResolveTechEffects and applyEffect; add tremorsense handler"
  ```

---

## Task 5: Thread querier through activateTechWithEffects

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Update `activateTechWithEffects` signature**

  Current (line ~4978):
  ```go
  func (s *GameServiceServer) activateTechWithEffects(sess *session.PlayerSession, uid, abilityID, targetID, fallbackMsg string) (*gamev1.ServerEvent, error) {
  ```
  Change to:
  ```go
  func (s *GameServiceServer) activateTechWithEffects(sess *session.PlayerSession, uid, abilityID, targetID, fallbackMsg string, querier RoomQuerier) (*gamev1.ServerEvent, error) {
  ```

  Inside `activateTechWithEffects`, find the `ResolveTechEffects` call and pass `querier`:
  ```go
  msgs := ResolveTechEffects(sess, techDef, techTargets, cbt, s.condRegistry, globalRandSrc{}, querier)
  ```

- [ ] **Step 2: Update all existing call sites to pass `nil`**

  There are 4 existing call sites (lines ~4851, 4893, 4906, 4908). Change each:
  ```go
  // Line ~4851 (prepared tech):
  return s.activateTechWithEffects(sess, uid, abilityID, targetID, fmt.Sprintf("You activate %s.", abilityID), nil)

  // Line ~4893 (spontaneous tech):
  return s.activateTechWithEffects(sess, uid, abilityID, targetID, fmt.Sprintf("You activate %s. (%d uses remaining at level %d.)", abilityID, pool.Remaining, foundLevel), nil)

  // Line ~4906 (innate with uses):
  return s.activateTechWithEffects(sess, uid, abilityID, targetID, fmt.Sprintf("You activate %s. (%d uses remaining.)", abilityID, slot.UsesRemaining), nil)

  // Line ~4908 (innate unlimited):
  return s.activateTechWithEffects(sess, uid, abilityID, targetID, fmt.Sprintf("You activate %s.", abilityID), nil)
  ```

- [ ] **Step 3: Update the passive call site in `triggerPassiveTechsForRoom` to pass `s`**

  Current (line ~7635):
  ```go
  evt, err := s.activateTechWithEffects(sess, sess.UID, techID, "", "")
  ```
  Change to:
  ```go
  evt, err := s.activateTechWithEffects(sess, sess.UID, techID, "", "", s)
  ```

- [ ] **Step 4: Build to verify**

  ```bash
  go build ./internal/gameserver/...
  ```
  Expected: no output (success)

- [ ] **Step 5: Run full test suite**

  ```bash
  go test ./internal/gameserver/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | tail -20
  ```
  Expected: all PASS

- [ ] **Step 6: Commit**

  ```bash
  git add internal/gameserver/grpc_service.go
  git commit -m "feat: thread RoomQuerier through activateTechWithEffects; wire triggerPassiveTechsForRoom"
  ```

---

## Task 6: Implement CreaturesInRoom on GameServiceServer

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_passive_test.go`

- [ ] **Step 1: Read the passive test file to understand existing helpers**

  Before writing any test code, read the existing passive test file to find exact helper names and patterns:
  ```bash
  cat internal/gameserver/grpc_service_passive_test.go
  ```
  Note: the exact constructor, session helper, and event-channel access patterns. Do NOT assume names like `newTestServer` — use what is actually in the file.

  **Important:** `mockRoomQuerier` is defined in `tech_effect_resolver_test.go` (same package). Do NOT redefine it here.

  Write a failing integration test for tremorsense passive activation in `grpc_service_passive_test.go`. The test verifies:
  - Player 1 (has `seismic_sense`) receives `"[Seismic Sense]"` after `triggerPassiveTechsForRoom`
  - Player 2 (no `seismic_sense`, same room) receives nothing

  Use the exact helper names from the file you just read. Adapt the test below:
  ```go
  func TestSeismicSense_PassiveActivation_SendsCreatureList(t *testing.T) {
      // ADAPT: use actual constructor and helper names from this file
      srv := /* actual constructor from this file */
      roomID := "room-test"

      // Player 1: has seismic_sense innate tech
      p1 := /* addPlayerWithInnateTechs or equivalent, with seismic_sense */
      // Register seismic_sense as passive tech in the server's tech registry
      /* register technology.TechnologyDef{
          ID: "seismic_sense", Passive: true, ActionCost: 0, Resolution: "",
          Effects: technology.TieredEffects{
              OnApply: []technology.TechEffect{{Type: technology.EffectTypeTremorsense}},
          },
      } */

      // Player 2: same room, no seismic_sense
      p2 := /* addPlayer or equivalent */

      srv.triggerPassiveTechsForRoom(roomID)

      // Player 1 must receive [Seismic Sense] message
      /* select on p1's event channel with 100ms timeout, unmarshal, assert Contains "[Seismic Sense]" */

      // Player 2 must receive nothing
      /* select on p2's event channel with 20ms timeout, assert no event */
  }
  ```

  Run to confirm compile error (test infrastructure not yet written):
  ```bash
  go test ./internal/gameserver/... -run TestSeismicSense_PassiveActivation -v 2>&1 | tail -5
  ```
  Expected: compile error or FAIL (not a skip — this is the failing test first)

- [ ] **Step 2: Implement `CreaturesInRoom` on `GameServiceServer`**

  Add to `internal/gameserver/grpc_service.go` (near `triggerPassiveTechsForRoom`):
  ```go
  // CreaturesInRoom implements RoomQuerier for GameServiceServer.
  // It returns all players and NPCs currently in roomID.
  // The sensing player (sensingUID) is returned as CreatureInfo{Name: "you"}.
  //
  // Precondition: roomID and sensingUID are non-empty strings.
  // Postcondition: Returns one entry per creature; sensing player entry has Name="you".
  func (s *GameServiceServer) CreaturesInRoom(roomID, sensingUID string) []CreatureInfo {
      var result []CreatureInfo

      // Add NPC instances
      if s.npcH != nil {
          for _, inst := range s.npcH.InstancesInRoom(roomID) {
              result = append(result, CreatureInfo{Name: inst.Name, Hidden: false})
          }
      }

      // Add players
      for _, sess := range s.sessions.PlayersInRoomDetails(roomID) {
          if sess.UID == sensingUID {
              result = append(result, CreatureInfo{Name: "you", Hidden: false})
          } else {
              result = append(result, CreatureInfo{Name: sess.CharName, Hidden: false})
          }
      }

      return result
  }
  ```

- [ ] **Step 3: Build to verify**

  ```bash
  go build ./internal/gameserver/...
  ```
  Expected: no output (success)

- [ ] **Step 4: Complete the integration test using actual helper names**

  You already read `grpc_service_passive_test.go` in Step 1. Now fill in the real helper names you found and complete the integration test. The test structure is:

  1. Create server with actual constructor
  2. Add player 1 with `seismic_sense` innate slot + register the `TechnologyDef` as passive tremorsense
  3. Add player 2 with no innate techs, same room
  4. Call `srv.triggerPassiveTechsForRoom(roomID)`
  5. Assert player 1 receives an event with `"[Seismic Sense]"` in the message
  6. Assert player 2 receives no event within 20ms

  Use only existing helpers — do not invent new ones. If a required helper doesn't exist, surface the gap explicitly rather than guessing.

- [ ] **Step 5: Run integration tests**

  ```bash
  go test ./internal/gameserver/... -run "TestSeismicSense_PassiveActivation" -v 2>&1 | tail -15
  ```
  Expected: PASS

- [ ] **Step 6: Run full test suite**

  ```bash
  go test ./internal/game/technology/... ./internal/gameserver/... 2>&1 | grep -E "^(ok|FAIL)" | tail -10
  ```
  Expected: all `ok`

- [ ] **Step 7: Commit**

  ```bash
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_passive_test.go
  git commit -m "feat: implement CreaturesInRoom on GameServiceServer; integration test for seismic_sense"
  ```

---

## Task 7: Final verification and docs

**Files:**
- Modify: `docs/features/technology.md` (mark passive tech checkbox)

- [ ] **Step 1: Run full test suite**

  ```bash
  go test ./... 2>&1 | grep -E "^(ok|FAIL)" | tail -30
  ```
  Expected: all `ok`, no `FAIL`

- [ ] **Step 2: Mark passive innate tech checkbox complete in technology.md**

  In `docs/features/technology.md`, find:
  ```
        - [ ] Passive innate tech mechanics — `seismic_sense` (always-on tremorsense) and `moisture_reclaim` (always-on water extraction) should apply passively without `use` command (Sub-project: Passive Tech Mechanics)
  ```
  Change to:
  ```
        - [x] Passive innate tech mechanics — `seismic_sense` (always-on tremorsense) applies passively without `use` command; `moisture_reclaim` cantrip refactor is a separate sub-project
  ```

- [ ] **Step 3: Final commit**

  ```bash
  git add docs/features/technology.md
  git commit -m "docs: mark seismic_sense passive tech mechanic complete"
  ```

# Detection States — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the symmetric global `Combatant.Hidden bool` with the full PF2E **detection ladder** as a per-pair asymmetric `(observer, target) → State` map. Six states — `Observed`, `Concealed`, `Hidden`, `Undetected`, `Unnoticed`, `Invisible` — gate attack resolution (flat checks, miss chances, square-guessing), filter outbound room views, and drive transition actions (Hide, Sneak, Seek, Avoid Notice, Create-a-Diversion). Existing pinned tests in `round_hidden_test.go` survive via a back-compat shim. LoS occlusion is explicitly out of scope (#267); the plan ships a no-op `OcclusionProvider` interface for #267 to fill.

**Spec:** [docs/superpowers/specs/2026-04-24-detection-states.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-24-detection-states.md)

**Architecture:** A new `internal/game/detection/` package owns the entire surface — `State` enum + helpers, `Map` (per-pair table), `GateAttack` (single uniform gate replacing the ad-hoc DC-11 block in `round.go:812-842`), `FilterRoomView` (per-recipient outbound filter), and an `OcclusionProvider` interface that v1 implements as a no-op. `Combat.DetectionStates *detection.Map` lives on the existing `Combat` struct, initialised empty at combat start; absent pairs default to `Observed`. The back-compat shim runs at combat start: any `Combatant.Hidden == true` populates the map symmetrically. Attack resolution consults the map, gates per state, applies `off_guard` via the existing condition pipeline (no parallel "transient -2 AC" plumbing), and on damage advances the (target, attacker) pair toward `Observed`. Square-guess targeting flows through a new `oneof { string target | Cell target_square }` on existing strike protos; the resolver branches on whichever is set. Initiative honours a new `InitiativeRollMode` (`Perception` default; `Stealth` set by `Avoid Notice`). Six new condition YAML files express each state through the typed-bonus model (per #245), so durations / expiries / badges all reuse the existing condition pipeline.

**Tech Stack:** Go (`internal/game/detection/`, `internal/game/combat/`, `internal/gameserver/`, `internal/game/condition/`), `pgregory.net/rapid` for property tests, protobuf (`api/proto/game/v1/game.proto`), telnet handlers, web client.

**Prerequisite:** None hard. #245 (typed bonuses) is a soft dep — condition `effects` use the typed-bonus model with `kind: detection_state`. #252 (Off-Guard condition + Skill actions) is a soft dep — `off_guard` is reused, and DETECT-26 reconciles with NCA-32's `create_a_diversion`. #267 (Visibility / LoS) is the seam this plan reserves; v1 ships a no-op `OcclusionProvider`.

**User confirmation checkpoints:**

- **DETECT-Q3 — reactions interaction**: spec recommends reactions fire only when the reactor's pair-state to the trigger source is at least `Hidden` (i.e., the reactor knows the source's location). Task 6 requires user confirmation before locking the rule.
- **DETECT-Q5 — Create-a-Diversion conflict with #252**: NCA-32 says "all enemies are off-guard to actor's next attack this turn"; DETECT-26 says "actor becomes Hidden to all enemies for the actor's next attack". Whichever ticket lands second MUST adjust. Task 7 makes this explicit.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/detection/state.go` |
| Create | `internal/game/detection/state_test.go` |
| Create | `internal/game/detection/map.go` |
| Create | `internal/game/detection/map_test.go` |
| Create | `internal/game/detection/gate.go` |
| Create | `internal/game/detection/gate_test.go` |
| Create | `internal/game/detection/filter.go` |
| Create | `internal/game/detection/filter_test.go` |
| Create | `internal/game/detection/occlusion.go` (no-op `OcclusionProvider`) |
| Create | `internal/game/detection/testdata/rapid/TestDetection_Property/` |
| Modify | `internal/game/combat/combat.go` (`Combat.DetectionStates`; `Combatant.MadeSoundThisRound`; `InitiativeRollMode`) |
| Modify | `internal/game/combat/round.go` (replace ad-hoc DC-11 block with `detection.GateAttack`) |
| Modify | `internal/game/combat/initiative.go` (honour `InitiativeRollMode`) |
| Create | `internal/game/combat/round_detection_test.go` |
| Modify | `internal/gameserver/combat_handler.go` (Hide / Seek / Sneak migrate; new `handleSneak`) |
| Modify | `internal/gameserver/grpc_service.go` (outbound `RoomView` pipes through filter) |
| Modify | `api/proto/game/v1/game.proto` (`oneof { target | target_square }`; `RoomView.combatants[].redacted_as`) |
| Modify | `content/conditions/hidden.yaml`, `undetected.yaml` (rewrite to typed-bonus schema) |
| Create | `content/conditions/observed.yaml`, `concealed.yaml`, `unnoticed.yaml`, `invisible.yaml` |
| Modify | `internal/frontend/handlers/strike_command.go` (`attack-at`, `strike-at` square-guess commands) |
| Modify | `cmd/webclient/ui/src/combat/CombatActionBar.tsx` + `MapPanel.tsx` (square-picker on Undetected target) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: `State` enum + `Map` type

**Files:**
- Create: `internal/game/detection/state.go`
- Create: `internal/game/detection/state_test.go`
- Create: `internal/game/detection/map.go`
- Create: `internal/game/detection/map_test.go`

- [ ] **Step 1: Failing tests** for the enum and helpers (DETECT-1):

```go
func TestState_Stringers(t *testing.T) {
    cases := map[detection.State]string{
        detection.Observed:   "observed",
        detection.Concealed:  "concealed",
        detection.Hidden:     "hidden",
        detection.Undetected: "undetected",
        detection.Unnoticed:  "unnoticed",
        detection.Invisible:  "invisible",
    }
    for s, want := range cases {
        require.Equal(t, want, s.String())
    }
}

func TestState_MissChancePercent(t *testing.T) {
    require.Equal(t, 0,  detection.Observed.MissChancePercent())
    require.Equal(t, 20, detection.Concealed.MissChancePercent())
    require.Equal(t, 50, detection.Hidden.MissChancePercent())   // DC 11 ≈ 50%
    require.Equal(t, 50, detection.Undetected.MissChancePercent())
    // Unnoticed and Invisible vary; documented as 100/50 placeholder.
}
```

- [ ] **Step 2: Failing tests** for the `Map` (DETECT-2 / DETECT-4):

```go
func TestMap_AbsentPairDefaultsObserved(t *testing.T) {
    m := detection.NewMap()
    require.Equal(t, detection.Observed, m.Get("a", "b"))
}

func TestMap_AsymmetricGetSet(t *testing.T) {
    m := detection.NewMap()
    m.Set("a", "b", detection.Hidden)
    require.Equal(t, detection.Hidden, m.Get("a", "b"))
    require.Equal(t, detection.Observed, m.Get("b", "a"), "asymmetric — Get(B,A) is independent (DETECT-4)")
}

func TestMap_ClearRevertsToObserved(t *testing.T) {
    m := detection.NewMap()
    m.Set("a", "b", detection.Hidden)
    m.Clear("a", "b")
    require.Equal(t, detection.Observed, m.Get("a", "b"))
}

func TestMap_ForObserverIteratesAllTargets(t *testing.T) {
    m := detection.NewMap()
    m.Set("a", "b", detection.Hidden)
    m.Set("a", "c", detection.Concealed)
    seen := map[string]detection.State{}
    for tgt, st := range m.ForObserver("a") {
        seen[tgt] = st
    }
    require.Equal(t, map[string]detection.State{"b": detection.Hidden, "c": detection.Concealed}, seen)
}
```

- [ ] **Step 3: Implement** the enum, `MissChancePercent`, and `Map`. The map is a `map[string]map[string]State` (observer → target → state). `ForObserver` is a Go 1.23 `iter.Seq2` range-able iterator.

- [ ] **Step 4:** All tests pass.

---

### Task 2: `GateAttack` — uniform gate per state

**Files:**
- Create: `internal/game/detection/gate.go`
- Create: `internal/game/detection/gate_test.go`

- [ ] **Step 1: Failing tests** covering the full state matrix (DETECT-7, DETECT-10). One scenario per branch:

```go
func TestGate_Observed_Proceeds(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Observed, dice.Fixed(15))
    require.Equal(t, detection.GateProceed, res.Outcome)
    require.False(t, res.OffGuard)
}

func TestGate_Concealed_FlatCheckFail_AutoMiss(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Concealed, dice.Fixed(4)) // < 5
    require.Equal(t, detection.GateAutoMiss, res.Outcome)
}

func TestGate_Concealed_FlatCheckPass_Proceeds(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Concealed, dice.Fixed(5))
    require.Equal(t, detection.GateProceed, res.Outcome)
    require.False(t, res.OffGuard)
}

func TestGate_Hidden_PassAppliesOffGuard(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Hidden, dice.Fixed(15))
    require.Equal(t, detection.GateProceed, res.Outcome)
    require.True(t, res.OffGuard)
}

func TestGate_Undetected_WrongSquareAutoMiss(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Undetected, dice.Any(),
        detection.WithSquareGuess(Cell{X: 99, Y: 99})) // wrong cell
    require.Equal(t, detection.GateAutoMiss, res.Outcome)
    require.True(t, res.OffGuard, "Undetected always confers off-guard regardless of guess")
}

func TestGate_Undetected_RightSquareThenFlatCheckFail(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Undetected, dice.Fixed(5),
        detection.WithSquareGuess(target.Cell()))
    require.Equal(t, detection.GateAutoMiss, res.Outcome)
    require.True(t, res.OffGuard)
}

func TestGate_Undetected_RightSquareFlatCheckPass(t *testing.T) {
    res := detection.GateAttack(attacker, target, detection.Undetected, dice.Fixed(15),
        detection.WithSquareGuess(target.Cell()))
    require.Equal(t, detection.GateProceed, res.Outcome)
    require.True(t, res.OffGuard)
}

func TestGate_Invisible_WithSoundCue_BehavesLikeHidden(t *testing.T) {
    target.MadeSoundThisRound = true
    res := detection.GateAttack(attacker, target, detection.Invisible, dice.Fixed(15))
    require.Equal(t, detection.GateProceed, res.Outcome)
    require.True(t, res.OffGuard)
}

func TestGate_Invisible_WithoutSoundCue_BehavesLikeUndetected(t *testing.T) {
    target.MadeSoundThisRound = false
    res := detection.GateAttack(attacker, target, detection.Invisible, dice.Fixed(15),
        detection.WithSquareGuess(Cell{X: 99, Y: 99}))
    require.Equal(t, detection.GateAutoMiss, res.Outcome)
}
```

- [ ] **Step 2: Implement** `GateAttack`:

```go
type GateOutcome int

const (
    GateProceed GateOutcome = iota
    GateAutoMiss
)

type GateResult struct {
    Outcome  GateOutcome
    OffGuard bool
    DCRolled int      // diagnostic
    DCNeeded int      // diagnostic
}

type Option func(*opts)

func WithSquareGuess(c Cell) Option { ... }

func GateAttack(attacker, target *combat.Combatant, st State, dice dice.Roller, opts ...Option) GateResult {
    o := buildOpts(opts)
    switch st {
    case Observed:
        return GateResult{Outcome: GateProceed}
    case Concealed:
        roll := dice.Roll("1d20")
        if roll < 5 { return GateResult{Outcome: GateAutoMiss, DCRolled: roll, DCNeeded: 5} }
        return GateResult{Outcome: GateProceed, DCRolled: roll, DCNeeded: 5}
    case Hidden:
        roll := dice.Roll("1d20")
        if roll < 11 { return GateResult{Outcome: GateAutoMiss, DCRolled: roll, DCNeeded: 11} }
        return GateResult{Outcome: GateProceed, OffGuard: true, DCRolled: roll, DCNeeded: 11}
    case Undetected, Unnoticed:
        if !o.guessed || o.guess != target.Cell() {
            return GateResult{Outcome: GateAutoMiss, OffGuard: true}
        }
        roll := dice.Roll("1d20")
        if roll < 11 { return GateResult{Outcome: GateAutoMiss, OffGuard: true, DCRolled: roll, DCNeeded: 11} }
        return GateResult{Outcome: GateProceed, OffGuard: true, DCRolled: roll, DCNeeded: 11}
    case Invisible:
        if target.MadeSoundThisRound {
            return GateAttack(attacker, target, Hidden, dice, opts...)
        }
        return GateAttack(attacker, target, Undetected, dice, opts...)
    }
    return GateResult{Outcome: GateProceed}
}
```

- [ ] **Step 3: All gate-matrix tests pass.**

---

### Task 3: `Combat.DetectionStates` + back-compat shim + sound tracking

**Files:**
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/round_hidden_test.go` (no test changes; just verify they pass)
- Create: `internal/game/combat/round_detection_test.go`

- [ ] **Step 1: Add fields**:

```go
type Combat struct {
    // ... existing ...
    DetectionStates *detection.Map  // DETECT-3
}

type Combatant struct {
    // ... existing ...
    Hidden              bool                // DEPRECATED — DETECT-Q4 + DETECT-5
    MadeSoundThisRound  bool                // DETECT-19
    InitiativeRollMode  InitiativeRollMode  // DETECT-28
}

type InitiativeRollMode int

const (
    InitiativeUsePerception InitiativeRollMode = iota
    InitiativeUseStealth
)
```

- [ ] **Step 2: Failing test** for the back-compat shim (DETECT-5):

```go
func TestCombatStart_LegacyHiddenFlagPopulatesMapSymmetrically(t *testing.T) {
    c := newCombat()
    npc := addCombatant(c, "thug", combat.WithHidden(true))
    p1 := addCombatant(c, "hero1")
    p2 := addCombatant(c, "hero2")
    c.Start()
    require.Equal(t, detection.Hidden, c.DetectionStates.Get(p1.UID, npc.UID))
    require.Equal(t, detection.Hidden, c.DetectionStates.Get(p2.UID, npc.UID))
    // The reverse — npc viewing players — stays Observed (asymmetry).
    require.Equal(t, detection.Observed, c.DetectionStates.Get(npc.UID, p1.UID))
}
```

- [ ] **Step 3: Implement** the shim in `Combat.Start()`:

```go
func (c *Combat) Start() {
    c.DetectionStates = detection.NewMap()
    for _, target := range c.Combatants {
        if !target.Hidden {
            continue
        }
        for _, observer := range c.Combatants {
            if observer == target { continue }
            c.DetectionStates.Set(observer.UID, target.UID, detection.Hidden)
        }
    }
    // existing Start body
}
```

- [ ] **Step 4: Replace the ad-hoc DC-11 block** at `round.go:812-842` with a single `detection.GateAttack` call:

```go
pairState := cbt.DetectionStates.Get(attacker.UID, target.UID)
gate := detection.GateAttack(attacker, target, pairState, dice)
if gate.OffGuard {
    cbt.AttachTransientCondition(target.UID, "off_guard")
}
defer func() {
    if gate.OffGuard {
        cbt.DetachTransientCondition(target.UID, "off_guard")
    }
}()
if gate.Outcome == detection.GateAutoMiss {
    emitMiss(narrative)
    return
}
// existing resolveAttack(...) path
```

- [ ] **Step 5: Add the round-start reset** for `MadeSoundThisRound`:

```go
func (c *Combat) StartRound() {
    for _, cb := range c.Combatants { cb.MadeSoundThisRound = false }
    // existing body
}
```

- [ ] **Step 6: Sound-cue setting** — every action that PF2E classifies `auditory` (Strike, Reload, Stride, MoveTo, future trait-tagged actions) sets the actor's `MadeSoundThisRound = true` at action-resolution start. Tests:

```go
func TestStrike_SetsMadeSoundThisRound(t *testing.T) { ... }
func TestSneak_DoesNotSetMadeSoundThisRound(t *testing.T) { ... }
```

- [ ] **Step 7: All three pinning tests** in `round_hidden_test.go` pass without modification (DETECT-9).

---

### Task 4: Damage advances state + scenario tests

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/round_detection_test.go`

- [ ] **Step 1: Failing tests** (DETECT-10 plus DETECT-27):

```go
func TestDamage_AdvancesAttackerStateTowardObserved(t *testing.T) {
    cbt := newCombat()
    a := addCombatant(cbt, "a")
    b := addCombatant(cbt, "b")
    cbt.Start()
    cbt.DetectionStates.Set(b.UID, a.UID, detection.Hidden)
    resolveAttack(cbt, a, b, /*hits*/ true, /*damage*/ 1)
    require.Equal(t, detection.Concealed, cbt.DetectionStates.Get(b.UID, a.UID),
        "Hidden → Concealed advance after damage (DETECT-27)")
}

func TestDamage_NoAdvanceWhenAttackerAlreadyObserved(t *testing.T) {
    cbt := newCombat()
    a := addCombatant(cbt, "a")
    b := addCombatant(cbt, "b")
    cbt.Start()
    // already Observed
    resolveAttack(cbt, a, b, true, 1)
    require.Equal(t, detection.Observed, cbt.DetectionStates.Get(b.UID, a.UID))
}
```

- [ ] **Step 2: Implement** `detection.AdvanceTowardObserved(map, observerUID, targetUID)`:

```go
func AdvanceTowardObserved(m *Map, observerUID, targetUID string) {
    cur := m.Get(observerUID, targetUID)
    next := cur
    switch cur {
    case Concealed:  next = Observed
    case Hidden:     next = Concealed
    case Undetected: next = Hidden
    case Unnoticed:  next = Undetected
    case Invisible:  // Invisible doesn't decay through damage in v1.
    case Observed:   // already top.
    }
    if next != cur {
        m.Set(observerUID, targetUID, next)
    }
}
```

Called from `resolveAttack` after damage applies, on the (target, attacker) pair.

- [ ] **Step 3: Implement scenario tests** for the full state matrix in `round_detection_test.go` per DETECT-10. One scenario per state.

---

### Task 5: Square-guess targeting

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/strike_command.go`

- [ ] **Step 1: Add proto oneof**:

```proto
message StrikeRequest {
  // ... existing fields ...
  oneof target_kind {
    string target_uid    = 30;
    Cell   target_square = 31;
  }
}
```

Existing `target string` field stays for one release as a deprecated alias for `target_uid` (avoids a breaking change to existing clients).

- [ ] **Step 2: Failing handler tests** (DETECT-11..15):

```go
func TestHandleStrike_TargetSquareEmptyCell_AutoMissNarrative(t *testing.T) {
    res, _ := s.HandleStrike(ctx, &gamev1.StrikeRequest{TargetSquare: &cell{X: 99, Y: 99}})
    require.Contains(t, res.Narrative, "hits empty")
}

func TestHandleStrike_TargetSquareWithObservedTarget_Rejected(t *testing.T) {
    setObserved(s, attackerUID, observableUID)
    _, err := s.HandleStrike(ctx, &gamev1.StrikeRequest{TargetSquare: &cell{X: observable.X, Y: observable.Y}})
    require.Error(t, err)
    require.Contains(t, err.Error(), "target by name instead")
}

func TestHandleStrike_TargetSquare_DoesNotRevealCorrectness(t *testing.T) {
    setUndetected(s, attackerUID, hiddenUID)
    // Wrong square miss
    a, _ := s.HandleStrike(ctx, &gamev1.StrikeRequest{TargetSquare: &cell{X: 99, Y: 99}})
    // Right square but flat-check fail
    setDice(s, 5)
    b, _ := s.HandleStrike(ctx, &gamev1.StrikeRequest{TargetSquare: hidden.Cell()})
    require.Equal(t, a.Narrative, b.Narrative,
        "wrong-square miss must be indistinguishable from flat-check fail (DETECT-14)")
}
```

- [ ] **Step 3: Implement** the resolver branching: if `target_square` is set, look up the cell in the combat grid; if empty, emit the empty-terrain narrative; if occupied with an Observed/Concealed/Hidden target, reject; if Undetected, route through `GateAttack(WithSquareGuess(...))`.

- [ ] **Step 4: Telnet `attack-at` / `strike-at` commands** with explicit cell args. Web square-picker reuses the AoE single-cell template from #250's overlay; tests under Task 8.

---

### Task 6: `FilterRoomView` outbound filter

**Files:**
- Create: `internal/game/detection/filter.go`
- Create: `internal/game/detection/filter_test.go`
- Modify: `internal/gameserver/grpc_service.go` (every `RoomView` send-site pipes through `FilterRoomView`)
- Modify: `api/proto/game/v1/game.proto` (`RoomView.combatants[].redacted_as`)

- [ ] **Step 1: Checkpoint (DETECT-Q3 — reactions vs detection).** Confirm with user the rule for reaction triggers. Default = reactor's pair-state to source must be at least `Hidden` (i.e., reactor knows source's location); `Concealed` and `Observed` allow reactions; `Undetected` / `Unnoticed` / `Invisible-without-sound` block them. This rule lives in #244's `Trigger.Fire` path; this task only documents and stages it. The actual check goes in Task 9.

- [ ] **Step 2: Failing tests** for `FilterRoomView` (DETECT-16 / DETECT-17 / DETECT-18):

```go
func TestFilter_StripsUnnoticedCombatants(t *testing.T) {
    rv := buildRoomViewWith("hero", "ghost")
    m := detection.NewMap()
    m.Set("hero", "ghost", detection.Unnoticed)
    out := detection.FilterRoomView(rv, "hero", m)
    require.NotContains(t, names(out), "ghost")
}

func TestFilter_RedactsUndetectedAsTripleQuestion(t *testing.T) {
    rv := buildRoomViewWith("hero", "stalker")
    m := detection.NewMap()
    m.Set("hero", "stalker", detection.Undetected)
    out := detection.FilterRoomView(rv, "hero", m)
    e := byUID(out, "stalker")
    require.Equal(t, "???", e.Name)
    require.Zero(t, e.X) // grid cell hidden (DETECT-16)
}

func TestFilter_HiddenShowsSilhouetteAtCell(t *testing.T) {
    rv := buildRoomViewWith("hero", "thug")
    m := detection.NewMap()
    m.Set("hero", "thug", detection.Hidden)
    out := detection.FilterRoomView(rv, "hero", m)
    e := byUID(out, "thug")
    require.Equal(t, "<silhouette>", e.RedactedAs)
    require.NotZero(t, e.X) // cell present
}

func TestFilter_Idempotent(t *testing.T) {
    rv := buildRoomViewWith("hero", "thug")
    m := detection.NewMap()
    m.Set("hero", "thug", detection.Hidden)
    once := detection.FilterRoomView(rv, "hero", m)
    twice := detection.FilterRoomView(once, "hero", m)
    require.Equal(t, once, twice, "filter is idempotent (DETECT-17)")
}

func TestFilter_PerRecipient_DifferentResults(t *testing.T) {
    rv := buildRoomViewWith("h1", "h2", "thug")
    m := detection.NewMap()
    m.Set("h1", "thug", detection.Undetected)
    out1 := detection.FilterRoomView(rv, "h1", m)
    out2 := detection.FilterRoomView(rv, "h2", m)
    require.Equal(t, "???", byUID(out1, "thug").Name)
    require.Equal(t, "thug", byUID(out2, "thug").Name)
}
```

- [ ] **Step 3: Implement** the filter. Returns a fresh `RoomView` (does not mutate input). Pure-function; tests prove idempotence by feeding the output back in.

- [ ] **Step 4: Wire** the filter at every server→client emit point that ships `RoomView` or `CombatStateView`. The send-helper takes a per-recipient UID.

- [ ] **Step 5: Property test** at `internal/game/detection/testdata/rapid/TestDetection_Property/`:

```go
func TestProperty_FilterIdempotence(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        rv, m, observerUID := arbitraryRoomViewAndMap(t)
        a := detection.FilterRoomView(rv, observerUID, m)
        b := detection.FilterRoomView(a, observerUID, m)
        require.Equal(t, a, b)
    })
}
```

---

### Task 7: Transition actions — Hide / Seek / Sneak / Avoid Notice / Create-a-Diversion

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/game/combat/initiative.go`

- [ ] **Step 1: Failing tests** for each transition (DETECT-22..27):

```go
func TestHandleHide_PerEnemyStealthVsPerception(t *testing.T) {
    a := addPlayer(t, "hero", withStealth(20))
    npc1 := addNPC(t, "thug", withPerception(10))
    npc2 := addNPC(t, "captain", withPerception(25))
    s.HandleHide(ctx, &gamev1.HideRequest{ActorUid: a.UID})
    require.Equal(t, detection.Hidden, c.DetectionStates.Get(npc1.UID, a.UID), "low-Perception NPC loses sight")
    require.Equal(t, detection.Observed, c.DetectionStates.Get(npc2.UID, a.UID), "high-Perception NPC keeps sight")
}

func TestHandleSeek_AdvancesOneRung(t *testing.T) {
    a := addPlayer(t, "seeker", withPerception(20))
    target := addNPC(t, "stalker", withStealth(10))
    c.DetectionStates.Set(a.UID, target.UID, detection.Undetected)
    s.HandleSeek(ctx, &gamev1.SeekRequest{ActorUid: a.UID})
    require.Equal(t, detection.Hidden, c.DetectionStates.Get(a.UID, target.UID))
}

func TestHandleSneak_ChainsStealthChecksAndDoesNotSetSound(t *testing.T) {
    a := addPlayer(t, "hero", withStealth(20))
    addNPC(t, "thug", withPerception(10))
    res := s.HandleSneak(ctx, &gamev1.SneakRequest{ActorUid: a.UID, DestinationX: 4, DestinationY: 4})
    require.NoError(t, res.Err)
    require.False(t, a.MadeSoundThisRound, "Sneak does not set MadeSoundThisRound (DETECT-21)")
}

func TestSneak_RejectsWhenAlreadyObserved(t *testing.T) {
    a := addPlayer(t, "hero")
    addNPC(t, "thug")
    // both pair-states default to Observed
    _, err := s.HandleSneak(ctx, &gamev1.SneakRequest{ActorUid: a.UID})
    require.Error(t, err)
    require.Contains(t, err.Error(), "must be Hidden or Undetected")
}

func TestAvoidNotice_InitiativeRolledWithStealth(t *testing.T) {
    a := addPlayer(t, "hero", withStealth(20), withPerception(10))
    a.InitiativeRollMode = combat.InitiativeUseStealth
    rolls := combat.RollInitiative(a, dice.Fixed(10))
    // 10 + 20 (Stealth) = 30, not 10 + 10 (Perception) = 20.
    require.Equal(t, 30, rolls.Total)
}

func TestAvoidNotice_PerEnemyStartingState(t *testing.T) {
    a := addPlayer(t, "hero")
    a.InitiativeRollMode = combat.InitiativeUseStealth
    a.StealthRoll = 25
    npc1 := addNPC(t, "thug", withPerceptionDC(15))    // margin +10 → Unnoticed
    npc2 := addNPC(t, "captain", withPerceptionDC(30)) // margin -5 → Hidden
    npc3 := addNPC(t, "boss", withPerceptionDC(40))    // margin -15 → Observed
    c.Start()
    require.Equal(t, detection.Unnoticed, c.DetectionStates.Get(npc1.UID, a.UID))
    require.Equal(t, detection.Hidden,    c.DetectionStates.Get(npc2.UID, a.UID))
    require.Equal(t, detection.Observed,  c.DetectionStates.Get(npc3.UID, a.UID))
}

func TestCreateADiversion_AppliesHiddenForOneAttack(t *testing.T) {
    // Per DETECT-26 (and pending Q5 reconciliation with NCA-32).
    ...
}
```

- [ ] **Step 2: Implement** each handler. `handleSneak` is genuinely new; the others are migrations of existing handlers from the global `Hidden` flag onto the per-pair map.

- [ ] **Step 3: Initiative integration** in `initiative.go`:

```go
func RollInitiative(c *Combatant, dice dice.Roller) Init {
    base := dice.Roll("1d20")
    switch c.InitiativeRollMode {
    case InitiativeUseStealth:
        return Init{Total: base + c.StealthBonus, Roll: base}
    default:
        return Init{Total: base + c.PerceptionMod, Roll: base}
    }
}
```

Per-enemy starting-state evaluation runs at `Combat.Start()` immediately after initiative is rolled (DETECT-29).

- [ ] **Step 4:** "Avoid Notice" is an out-of-combat action that sets `c.InitiativeRollMode` and stages the Stealth roll. Implementation lives in the exploration handler; the combat layer only consumes the field.

- [ ] **Step 5: Q5 reconciliation note** — when both #252 and this ticket have landed, whichever lands second updates `create_a_diversion.yaml` accordingly. Plan default: align with DETECT-26 semantics (actor becomes Hidden to all enemies for one attack), since per-pair state is the more general primitive.

---

### Task 8: Telnet + Web UX for Undetected targets

**Files:**
- Modify: `internal/frontend/handlers/strike_command.go`
- Modify: `cmd/webclient/ui/src/combat/CombatActionBar.tsx`
- Modify: `cmd/webclient/ui/src/combat/MapPanel.tsx`

- [ ] **Step 1: Failing telnet tests** (DETECT-15):

```go
func TestStrikeCommand_AttackAtAcceptsCell(t *testing.T) {
    h := newHandler(t)
    h.Run("attack-at 5 7")
    require.Equal(t, &gamev1.StrikeRequest{TargetSquare: &cell{X: 5, Y: 7}}, h.LastStrikeReq())
}

func TestStrikeCommand_StrikeAtAcceptsCell(t *testing.T) {
    h := newHandler(t)
    h.Run("strike-at 3 9")
    require.Equal(t, &cell{X: 3, Y: 9}, h.LastStrikeReq().GetTargetSquare())
}
```

- [ ] **Step 2: Implement** the two new commands (`attack-at <x> <y>` / `strike-at <x> <y>`). Existing `attack <name>` / `strike <name>` continue to work for visible targets.

- [ ] **Step 3: Failing web component tests**:

```ts
test("Undetected target shows ??? in action bar", () => {
  render(<CombatActionBar combatants={[{ uid: "stalker", redactedAs: "???" }]} />);
  expect(screen.getByText("???")).toBeInTheDocument();
});

test("Clicking Undetected target opens square-picker", () => {
  render(<MapPanel ... />);
  fireEvent.click(screen.getByText("???"));
  expect(screen.getByText("Choose a square")).toBeVisible();
});
```

- [ ] **Step 4: Implement** the `redactedAs` rendering and the square-picker (reuse #250's single-cell template UI). The picker dispatches `StrikeRequest{target_square: cell}`.

---

### Task 9: Conditions bridge

**Files:**
- Modify: `content/conditions/hidden.yaml`, `undetected.yaml`
- Create: `content/conditions/observed.yaml`, `concealed.yaml`, `unnoticed.yaml`, `invisible.yaml`
- Modify: `internal/game/condition/loader.go` (recognise `kind: detection_state` typed-bonus)

- [ ] **Step 1: Failing tests** (DETECT-31..33):

```go
func TestConditionCatalog_LoadsAllSixDetectionStates(t *testing.T) {
    cat := condition.LoadCatalog(t, "content/conditions")
    for _, id := range []string{"observed", "concealed", "hidden", "undetected", "unnoticed", "invisible"} {
        require.NotNil(t, cat.ByID(id))
    }
}

func TestApplyHiddenCondition_SetsPairStateToHidden(t *testing.T) {
    c := newCombat()
    a := addCombatant(c, "a")
    b := addCombatant(c, "b")
    c.Start()
    cond := condition.ByID("hidden")
    c.ApplyCombatCondition(b /*target=hidden*/, cond, /*duration*/ 1)
    require.Equal(t, detection.Hidden, c.DetectionStates.Get(a.UID, b.UID))
}
```

- [ ] **Step 2: Author** the four new YAML files plus rewrite the existing two:

```yaml
# content/conditions/hidden.yaml
id: hidden
display_name: Hidden
description: |
  An attacker can pinpoint your square but can't see you precisely. They
  must succeed at a DC 11 flat check to target you. You are off-guard
  against any attacker who can't see you.
stacking: replace_if_higher
effects:
  - kind: detection_state
    state: hidden
```

```yaml
# content/conditions/observed.yaml
id: observed
display_name: Observed
description: An attacker can see you clearly. No special detection effects apply.
stacking: replace_if_higher
effects:
  - kind: detection_state
    state: observed
```

(Plus `concealed.yaml`, `undetected.yaml`, `unnoticed.yaml`, `invisible.yaml` along the same shape.)

- [ ] **Step 3: Loader extension** — recognise `kind: detection_state` typed-bonus and route the apply step to `Combat.DetectionStates.Set(...)` for each observer in scope. The condition is applied to the *target* combatant; the loader expands "all observers" by iterating `c.Combatants`.

- [ ] **Step 4: Reaction-vs-detection rule** (per Task 6 checkpoint outcome). Add the gate to the `Trigger.Fire` path used by #244's reactions:

```go
func (t *Trigger) ShouldFire(reactor *Combatant, source *Combatant, cbt *Combat) bool {
    pair := cbt.DetectionStates.Get(reactor.UID, source.UID)
    switch pair {
    case Undetected, Unnoticed:
        return false
    case Invisible:
        return source.MadeSoundThisRound
    }
    return true
}
```

If the user picks a different rule at the Task 6 checkpoint, swap accordingly.

---

### Task 10: Telemetry + narrative

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/game/detection/map.go` (transition log emit)

- [ ] **Step 1: Failing tests** (DETECT-34 / DETECT-35):

```go
func TestStateTransition_EmitsStructuredLog(t *testing.T) {
    sink := captureLog(t)
    c.DetectionStates.Set("a", "b", detection.Hidden) // transition Observed → Hidden
    e := sink.Last()
    require.Equal(t, "a", e["observer_uid"])
    require.Equal(t, "b", e["target_uid"])
    require.Equal(t, "observed", e["from_state"])
    require.Equal(t, "hidden", e["to_state"])
}

func TestNarrative_PerspectiveSafe(t *testing.T) {
    // Player loses sight of a Hidden NPC; narrative on player's combat log shows this,
    // but does NOT reveal that the player is Hidden to a specific other NPC.
    out := h.Run("hide")
    require.NotContains(t, out, "thug can no longer see you", "narrative must be perspective-safe")
}
```

- [ ] **Step 2: Implement** the structured emit on `Map.Set` (and `Clear`). `Map.Set` takes an optional `cause` argument used in the log entry; default = `"unspecified"`.

- [ ] **Step 3: Per-perspective combat log narrative** at the gameserver layer. The combat log builder consults the recipient's pair-state map and renders only the transitions visible to that recipient.

---

### Task 11: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Detection States" section** documenting:
  - All 35 `DETECT-N` requirements with one-line summaries.
  - The six-state ladder and asymmetric per-pair semantics.
  - The `GateAttack` matrix (state → flat check, miss chance, off-guard).
  - The square-guess targeting mode and the `oneof { target | target_square }` proto shape.
  - The `FilterRoomView` per-recipient outbound filter.
  - Sound-cue tracking and the `MadeSoundThisRound` flag.
  - The transition handlers (Hide / Seek / Sneak / Avoid Notice / Create-a-Diversion).
  - Initiative integration with `InitiativeRollMode`.
  - The conditions bridge (`kind: detection_state`) and back-compat with `Combatant.Hidden`.
  - The `OcclusionProvider` no-op interface and the contract for #267 to fill.
  - Open question resolutions (DETECT-Q1..Q5).

- [ ] **Step 2: Cross-link** to the spec, `internal/game/detection/`, and the rewritten condition YAML files.

- [ ] **Step 3:** Verify the doc renders correctly in GitHub markdown preview.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- All three pinned `round_hidden_test.go` tests pass without modification (DETECT-9).
- Telnet smoke test: hero hides; verify `attack thug` from a different NPC's perspective fails the DC-11 flat check sometimes; verify `attack-at <x> <y>` from an Undetected pair-state works as a square-guess.
- Web smoke test: two players in the same combat see different `RoomView` payloads; verify the Undetected / Unnoticed / Hidden redactions render.
- Initiative smoke test: an actor with `InitiativeRollMode = Stealth` rolls Stealth for initiative; per-enemy starting state matches the margin rules in DETECT-29.

---

## Rollout / Open Questions Resolved at Plan Time

- **DETECT-Q1**: `Unnoticed` is a per-pair state; only the actor's own auditory action converts their own `Unnoticed → Undetected`. Allies stay Unnoticed independently.
- **DETECT-Q2**: AoE templates apply effects positionally — pair-state does not gate AoE inclusion. Wrong-square gating applies only to single-target Undetected attacks.
- **DETECT-Q3**: Reactions fire only when the reactor's pair-state to the source is at least `Hidden` (knows location). User confirmation taken at Task 6 checkpoint.
- **DETECT-Q4**: `Combatant.Hidden bool` retained one release with `// DEPRECATED`. A tracker issue records the future deletion.
- **DETECT-Q5**: Reconciliation between this ticket's DETECT-26 and #252's NCA-32 happens at the second-to-land; default = align with DETECT-26 (actor becomes Hidden to all enemies for one attack).

## Non-Goals Reaffirmed

Per spec §2.2:

- No LoS / occlusion model (`OcclusionProvider` no-op v1; #267 fills it).
- No sound-based detection beyond the binary "made sound this round".
- No senses other than vision (scent, tremorsense, etc.).
- No multi-room / cross-room detection.
- No authoring UI for editing pair-state.
- No new Hide / Seek wire shapes — handlers migrate internally.

# Move Trait for Weapons — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a weapon-level `move` (recommended rename: `mobile`) trait that grants the wielder a single free Stride tied to a Strike with that weapon. The free Stride bypasses AP accounting (no 3-AP turn cost, no 2-AP movement cap) and suppresses reaction triggers. Stride may occur immediately before or after the Strike. NPCs with a Move-trait weapon take the free Stride through the existing HTN planner and the (pending) `chooseMoveDestination` step from #251.

**Spec:** [docs/superpowers/specs/2026-04-24-move-trait-for-weapons.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-24-move-trait-for-weapons.md)

**Architecture:** Three new pieces of plumbing, all single-source-of-truth. (1) A `traits` package with a registry keyed by trait id holding `Behavior{grants_free_action, suppresses_reactions, ...}`; `WeaponDef.HasTrait(id)` is the only public read path. (2) An `ActionMoveTraitStride` action type with cost 0, queued via a new `ActionQueue.QueueFreeAction` that bypasses AP-deduction logic and is gated by an explicit allowlist. (3) A `MoveContext{actor, from, to, cause MoveCause}` parameter threaded through `CheckReactiveStrikes` (and any future move-keyed trigger) — when `cause == MoveTrait`, reaction checks early-return. The Strike pipeline marks the queued action `MoveTraitWielder=true`; `ResolveRound` executes the free Stride before or after the Strike per the queued ordering. Telnet adds `move-strike` / `strike-move` commands plus a 30s post-Strike prompt; web adds a two-stage placement UI on the action bar with a `MOVE` badge. NPC integration rides on #251's HTN action shape `move_trait_stride{x,y}` so HTN domains can queue the free Stride directly.

**Tech Stack:** Go (`internal/game/inventory/`, `internal/game/combat/`, `internal/gameserver/`, `internal/game/ai/`), `pgregory.net/rapid` for property tests, protobuf (`api/proto/game/v1/game.proto`), telnet handlers, React/TypeScript (`cmd/webclient/ui/src/`).

**Prerequisite:** None hard. #244 (Reactions and Ready actions) is the trigger system that `CheckReactiveStrikes` reads from — already present today; this plan only adds a `MoveContext` parameter to it. #251 (Smarter NPC movement) and #262 (Agile / MAP) are sister tickets that benefit from the same trait registry but are independent.

**User confirmation checkpoints:**

- **WMOVE-Q1 — trait id**: spec recommends renaming the trait from `move` to `mobile` to avoid collision with PF2E's existing `move` trait semantics (which marks *actions*, not weapons). Task 1 below requires user confirmation before locking the id. The plan assumes the recommendation (`mobile` with a `move` alias in the registry) but is reversible.
- **WMOVE-NG3 / Q3 — multi-Strike**: spec locks one free Stride per Strike *action* (umbrella), not per attack within a multi-Strike. Task 4 requires user confirmation if the alternative reading is preferred.
- **WMOVE-30 — exemplar weapon**: at least one weapon must be tagged with the trait. Task 8 requires user confirmation of the chosen content; defaults are reach polearms or whip-class weapons.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/inventory/traits/registry.go` |
| Create | `internal/game/inventory/traits/registry_test.go` |
| Modify | `internal/game/inventory/weapon.go` (`HasTrait` helper; startup registry validation) |
| Modify | `internal/game/inventory/weapon_test.go` |
| Modify | `internal/game/combat/action.go` (`ActionMoveTraitStride` + `QueueFreeAction`) |
| Modify | `internal/game/combat/action_test.go` |
| Create | `internal/game/combat/move_context.go` |
| Modify | `internal/game/combat/round.go` (`CheckReactiveStrikes` takes `MoveContext`) |
| Create | `internal/game/combat/move_trait_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestMoveTrait_Property/` |
| Modify | `internal/gameserver/combat_handler.go` (Strike marks `MoveTraitWielder`; queues free Stride) |
| Modify | `internal/gameserver/grpc_service.go` (`handleStrike` extension; new `handleMoveStrike` / `handleStrikeMove`) |
| Modify | `internal/game/ai/domain.go` (`move_trait_stride{x,y}` action shape) |
| Modify | `internal/game/ai/build_state.go` (surface `MoveTraitWielder` in `WorldState`) |
| Modify | `api/proto/game/v1/game.proto` (`StrikeRequest.move_trait_stride`) |
| Modify | `internal/frontend/handlers/strike_command.go` (move-strike / strike-move + post-Strike prompt) |
| Modify | `cmd/webclient/ui/src/combat/CombatActionBar.tsx` (`MOVE` badge) |
| Create | `cmd/webclient/ui/src/combat/MoveTraitPlacement.tsx` |
| Create | `cmd/webclient/ui/src/combat/MoveTraitPlacement.test.tsx` |
| Modify | `content/weapons/<chosen-weapon>.yaml` (tag with trait + description) |
| Modify | `internal/game/account/settings.go` (`MoveTraitAutoPrompt` setting) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: Trait registry — `traits.Mobile`, `Registry`, `Behavior`, `WeaponDef.HasTrait`

**Files:**
- Create: `internal/game/inventory/traits/registry.go`
- Create: `internal/game/inventory/traits/registry_test.go`
- Modify: `internal/game/inventory/weapon.go`

- [ ] **Step 1: Checkpoint (WMOVE-Q1).** STOP and confirm with the user the trait id:
  - Option A (spec recommendation): `mobile`, with `move` aliased in the registry for any content already authored.
  - Option B: `move`, accepting the PF2E semantic collision.

  Plan default below assumes Option A. If user confirms Option B, swap `traits.Mobile` for `traits.Move` and drop the alias.

- [ ] **Step 2: Failing tests** for the registry:

```go
func TestRegistry_KnownTraitsHaveBehavior(t *testing.T) {
    r := traits.DefaultRegistry()
    b := r.Behavior(traits.Mobile)
    require.NotNil(t, b)
    require.Equal(t, "Mobile", b.DisplayName)
    require.Equal(t, combat.ActionMoveTraitStride, b.GrantsFreeAction)
    require.True(t, b.SuppressesReactions)
}

func TestRegistry_MoveAliasesToMobile(t *testing.T) {
    r := traits.DefaultRegistry()
    require.Equal(t, traits.Mobile, r.CanonicalID("move"))
}

func TestRegistry_UnknownTraitWarnsNotErrors(t *testing.T) {
    sink := captureLog(t)
    r := traits.DefaultRegistry()
    err := r.Validate([]string{"reach", "definitely_not_a_trait"})
    require.NoError(t, err, "unknown traits warn but do not error (WMOVE-4)")
    require.Contains(t, sink.String(), "definitely_not_a_trait")
}

func TestWeaponDef_HasTrait(t *testing.T) {
    w := &inventory.WeaponDef{Traits: []string{"mobile"}}
    require.True(t, w.HasTrait(traits.Mobile))
    require.False(t, w.HasTrait(traits.Reach))
}
```

- [ ] **Step 3: Implement** the registry:

```go
package traits

const (
    Mobile = "mobile"
)

type Behavior struct {
    ID                  string
    DisplayName         string
    Description         string
    GrantsFreeAction    combat.ActionType
    SuppressesReactions bool
}

type Registry struct {
    behaviors map[string]*Behavior
    aliases   map[string]string
}

func DefaultRegistry() *Registry {
    return &Registry{
        behaviors: map[string]*Behavior{
            Mobile: {
                ID:                  Mobile,
                DisplayName:         "Mobile",
                Description:         "Grants a free Stride before or after a Strike with this weapon.",
                GrantsFreeAction:    combat.ActionMoveTraitStride,
                SuppressesReactions: true,
            },
        },
        aliases: map[string]string{"move": Mobile},
    }
}

func (r *Registry) CanonicalID(id string) string {
    if c, ok := r.aliases[id]; ok {
        return c
    }
    return id
}

func (r *Registry) Behavior(id string) *Behavior {
    return r.behaviors[r.CanonicalID(id)]
}

func (r *Registry) Validate(ids []string) error {
    for _, id := range ids {
        if r.Behavior(id) == nil {
            log.Warn().Str("trait", id).Msg("unknown weapon trait — registry has no behavior")
        }
    }
    return nil
}
```

- [ ] **Step 4: `WeaponDef.HasTrait`**:

```go
func (w *WeaponDef) HasTrait(id string) bool {
    canonical := traits.DefaultRegistry().CanonicalID(id)
    for _, t := range w.Traits {
        if traits.DefaultRegistry().CanonicalID(t) == canonical {
            return true
        }
    }
    return false
}
```

- [ ] **Step 5: Startup validation** — at server boot, call `Registry.Validate(weapon.Traits)` for every loaded weapon (WMOVE-4). Warns; never errors.

---

### Task 2: `ActionMoveTraitStride` + `ActionQueue.QueueFreeAction`

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify: `internal/game/combat/action_test.go`

- [ ] **Step 1: Failing tests** for the new action type and the free-action queue:

```go
func TestActionMoveTraitStride_CostIsZero(t *testing.T) {
    require.Equal(t, 0, combat.ActionMoveTraitStride.Cost())
}

func TestQueueFreeAction_DoesNotConsumeAP(t *testing.T) {
    q := combat.NewActionQueue(3 /*AP*/)
    err := q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionMoveTraitStride})
    require.NoError(t, err)
    require.Equal(t, 3, q.Remaining(), "free action must not deduct AP (WMOVE-7)")
    require.Equal(t, 0, q.MovementAPSpent(), "free Stride must not increment movementAPSpent (WMOVE-7)")
}

func TestQueueFreeAction_RejectsNonAllowlistedTypes(t *testing.T) {
    q := combat.NewActionQueue(3)
    err := q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionStride})
    require.Error(t, err, "QueueFreeAction must reject types outside the allowlist (WMOVE-8)")
}

func TestQueueFreeAction_OncePerStrikeEnforcement(t *testing.T) {
    q := combat.NewActionQueue(3)
    q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionMoveTraitStride, StrikeAction: 1})
    err := q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionMoveTraitStride, StrikeAction: 1})
    require.Error(t, err, "second free Stride for the same Strike must be rejected (WMOVE-10)")
}
```

`StrikeAction` is an integer id used to scope the once-per-Strike enforcement (Task 4 sets it).

- [ ] **Step 2: Implement** the action type and `QueueFreeAction`:

```go
const ActionMoveTraitStride ActionType = ActionLast + 1

func (a ActionType) Cost() int {
    switch a {
    // ... existing cases ...
    case ActionMoveTraitStride:
        return 0
    }
}

var freeActionAllowlist = map[ActionType]bool{
    ActionMoveTraitStride: true,
}

func (q *ActionQueue) QueueFreeAction(qa QueuedAction) error {
    if !freeActionAllowlist[qa.Type] {
        return fmt.Errorf("QueueFreeAction: %v not allowlisted", qa.Type)
    }
    if qa.Type == ActionMoveTraitStride {
        for _, existing := range q.actions {
            if existing.Type == ActionMoveTraitStride && existing.StrikeAction == qa.StrikeAction && qa.StrikeAction != 0 {
                return fmt.Errorf("once per Strike: free Stride already queued for strike action %d", qa.StrikeAction)
            }
        }
    }
    q.actions = append(q.actions, qa)
    // Note: do NOT touch remaining or movementAPSpent.
    return nil
}
```

- [ ] **Step 3:** All four tests pass.

---

### Task 3: `MoveContext` + `CheckReactiveStrikes` refactor

**Files:**
- Create: `internal/game/combat/move_context.go`
- Modify: `internal/game/combat/round.go`
- Modify: existing tests that already exercise `CheckReactiveStrikes` (signature change)

- [ ] **Step 1: Define `MoveContext`**:

```go
type MoveCause int

const (
    MoveCauseStride MoveCause = iota
    MoveCauseMoveTo
    MoveCauseMoveTrait
    MoveCauseForced
)

type MoveContext struct {
    Actor *Combatant
    From  Cell
    To    Cell
    Cause MoveCause
}
```

- [ ] **Step 2: Failing tests**:

```go
func TestCheckReactiveStrikes_NoOpWhenCauseIsMoveTrait(t *testing.T) {
    cbt := scenarioWithThreatenedCell(t)
    log := captureCombatLog(t, cbt)
    combat.CheckReactiveStrikes(cbt, combat.MoveContext{
        Actor: cbt.ByName("hero"),
        From:  Cell{X: 5, Y: 5},
        To:    Cell{X: 6, Y: 5},
        Cause: combat.MoveCauseMoveTrait,
    })
    require.Empty(t, log.ReactiveStrikes(), "MoveTrait must suppress all reactions (WMOVE-12)")
}

func TestCheckReactiveStrikes_RegressionForStrideUnchanged(t *testing.T) {
    // Existing test asserting reactive strike fires on Stride must still pass.
    ...
}
```

- [ ] **Step 3: Refactor `CheckReactiveStrikes`** to accept `MoveContext`:

```go
func CheckReactiveStrikes(cbt *Combat, ctx MoveContext) {
    if ctx.Cause == MoveCauseMoveTrait {
        return // WMOVE-12
    }
    // existing body
}
```

- [ ] **Step 4: Migrate** all in-package callers to pass an explicit `MoveContext`. The default cause for `ActionStride` is `MoveCauseStride`; for `MoveToRequest` is `MoveCauseMoveTo`; forced movement remains `MoveCauseForced`.

- [ ] **Step 5:** Existing combat tests pass without behaviour change for non-trait paths (WMOVE-13).

---

### Task 4: Strike pipeline integration

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Checkpoint (WMOVE-Q3).** Confirm with user: one free Stride per Strike *action* (umbrella), even when the action contains multiple attacks. Plan default = once-per-action.

- [ ] **Step 2: Add proto field** to `StrikeRequest`:

```proto
message StrikeRequest {
  // ... existing fields ...
  optional MoveTraitStride move_trait_stride = 20;
}

message MoveTraitStride {
  int32  destination_x = 1;
  int32  destination_y = 2;
  enum Ordering { BEFORE = 0; AFTER = 1; }
  Ordering ordering = 3;
  bool   skip = 4;
}
```

- [ ] **Step 3: Failing handler tests**:

```go
func TestHandleStrike_MoveTraitWielderQueuesFreeStrideBeforeStrike(t *testing.T) {
    setupWielder(t, "mobile_weapon")
    res := s.HandleStrike(ctx, &gamev1.StrikeRequest{
        TargetUid:        "thug",
        MoveTraitStride:  &gamev1.MoveTraitStride{DestinationX: 6, DestinationY: 5, Ordering: gamev1.MoveTraitStride_BEFORE},
    })
    require.NoError(t, err)
    queue := getCombat(s).WielderQueue("hero")
    require.Equal(t, combat.ActionMoveTraitStride, queue[0].Type)
    require.Equal(t, combat.ActionStrike, queue[1].Type)
}

func TestHandleStrike_NonMoveWeaponRejectsMoveTraitStride(t *testing.T) {
    setupWielder(t, "non_mobile_weapon")
    _, err := s.HandleStrike(ctx, &gamev1.StrikeRequest{
        TargetUid: "thug",
        MoveTraitStride: &gamev1.MoveTraitStride{DestinationX: 6, DestinationY: 5},
    })
    require.Error(t, err, "non-Move-trait weapon must not honour move_trait_stride")
}

func TestHandleStrike_OverDistanceRejected(t *testing.T) {
    setupWielderWithSpeed(t, "mobile_weapon", 3 /*cells*/)
    _, err := s.HandleStrike(ctx, &gamev1.StrikeRequest{
        TargetUid: "thug",
        MoveTraitStride: &gamev1.MoveTraitStride{DestinationX: 9, DestinationY: 5}, // 4 cells away
    })
    require.Error(t, err, "free Stride must respect SpeedSquares() (WMOVE-G1)")
}

func TestHandleStrike_MoveTraitWielder_RemainingAPMatchesNonTraitStrike(t *testing.T) {
    apMobile := remainingAPAfterStrike(t, "mobile_weapon", withFreeStride())
    apPlain := remainingAPAfterStrike(t, "non_mobile_weapon")
    require.Equal(t, apMobile, apPlain, "free Stride must not consume AP")
}
```

- [ ] **Step 4: Implement** in `combat_handler.go`:

```go
func (h *CombatHandler) Strike(uid string, target string, moveStride *MoveTraitStride) error {
    actor := h.combat.ByUID(uid)
    weapon := actor.EquippedWeapon()

    moveTraitWielder := weapon.HasTrait(traits.Mobile)
    strikeID := h.nextStrikeID() // monotonic
    if moveStride != nil && !moveStride.Skip {
        if !moveTraitWielder {
            return fmt.Errorf("weapon does not have move trait")
        }
        if cells := chebyshev(actor.Cell(), Cell{X: int(moveStride.DestinationX), Y: int(moveStride.DestinationY)}); cells > actor.SpeedSquares() {
            return fmt.Errorf("free Stride distance %d exceeds Speed %d", cells, actor.SpeedSquares())
        }
        if err := h.combat.WielderQueue(uid).QueueFreeAction(QueuedAction{
            Type:         ActionMoveTraitStride,
            TargetX:      moveStride.DestinationX,
            TargetY:      moveStride.DestinationY,
            Ordering:     moveStride.Ordering,
            StrikeAction: strikeID,
        }); err != nil {
            return err
        }
    }
    return h.combat.WielderQueue(uid).Enqueue(QueuedAction{
        Type:             ActionStrike,
        Target:           target,
        StrikeAction:     strikeID,
        MoveTraitWielder: moveTraitWielder,
    })
}
```

- [ ] **Step 5: ResolveRound integration** in `round.go`: when consuming the queued action list, the `ActionMoveTraitStride` action is processed before or after its sibling `ActionStrike` based on `Ordering`. Both run under `MoveContext{Cause: MoveCauseMoveTrait}` for the Stride. Tests:

```go
func TestResolveRound_StrideBeforeStrike(t *testing.T) { ... }
func TestResolveRound_StrideAfterStrike(t *testing.T) { ... }
func TestResolveRound_StrideDiscardedWhenWielderDowned(t *testing.T) {
    // Strike resolves first, downs the wielder via reaction; queued AFTER stride must silently drop (WMOVE-19).
    ...
}
func TestResolveRound_FreeStrideStillGrantedOnStrikeMiss(t *testing.T) { ... }
```

---

### Task 5: Property tests

**Files:**
- Create: `internal/game/combat/move_trait_test.go`
- Create: `internal/game/combat/testdata/rapid/TestMoveTrait_Property/`

- [ ] **Step 1: Implement property tests** per WMOVE-35:

```go
func TestProperty_MoveTrait_Determinism(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt, hero, target, dest := buildArbitraryMoveTraitScenario(t)
        a := simulateStrikeWithFreeStride(cbt, hero, target, dest)
        b := simulateStrikeWithFreeStride(cbt, hero, target, dest)
        require.Equal(t, a, b)
    })
}

func TestProperty_MoveTrait_StrideBoundedBySpeed(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt, hero, target, dest := buildArbitraryMoveTraitScenario(t)
        ok := simulateStrikeRespectsSpeed(cbt, hero, target, dest)
        require.True(t, ok)
    })
}

func TestProperty_MoveTrait_TotalAPNotExceeded(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt, hero, target, dest := buildArbitraryMoveTraitScenario(t)
        spent := simulateAPSpentInTurn(cbt, hero, target, dest)
        require.LessOrEqual(t, spent, 3)
    })
}

func TestProperty_MoveTrait_NoReactiveStrikesForMoveCauseMoveTrait(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cbt, hero, target, dest := buildArbitraryMoveTraitScenario(t)
        events := simulateAndCaptureReactions(cbt, hero, target, dest)
        for _, e := range events {
            if e.Cause == combat.MoveCauseMoveTrait {
                require.NotEqual(t, "ReactiveStrike", e.Reaction, "no reactive strike on MoveTrait move")
            }
        }
    })
}
```

- [ ] **Step 2:** Rapid recordings checked into `testdata/rapid/`.

---

### Task 6: Telnet UX — `move-strike`, `strike-move`, post-Strike prompt

**Files:**
- Modify: `internal/frontend/handlers/strike_command.go`
- Modify: `internal/game/account/settings.go` (`MoveTraitAutoPrompt` setting)

- [ ] **Step 1: Failing tests** per WMOVE-20 / WMOVE-21 / WMOVE-22:

```go
func TestStrikeCommand_MoveStrikeBeforeOrdering(t *testing.T) {
    h := newHandlerWithMobileWeapon(t)
    h.Run("move-strike thug 6,5")
    queue := h.WielderQueue()
    require.Equal(t, combat.ActionMoveTraitStride, queue[0].Type)
    require.Equal(t, combat.OrderingBefore, queue[0].Ordering)
    require.Equal(t, combat.ActionStrike, queue[1].Type)
}

func TestStrikeCommand_StrikeMoveAfterOrdering(t *testing.T) {
    h := newHandlerWithMobileWeapon(t)
    h.Run("strike-move thug 6,5")
    queue := h.WielderQueue()
    require.Equal(t, combat.OrderingAfter, queue[0].Ordering)
}

func TestStrikeCommand_PlainStrikePromptsWhenAutoPromptOn(t *testing.T) {
    h := newHandlerWithMobileWeapon(t, withAccountSetting("move_trait_auto_prompt", true))
    out := h.Run("strike thug")
    require.Contains(t, out, "Move-trait Stride available")
}

func TestStrikeCommand_PlainStrikeNoPromptWhenAutoPromptOff(t *testing.T) {
    h := newHandlerWithMobileWeapon(t, withAccountSetting("move_trait_auto_prompt", false))
    out := h.Run("strike thug")
    require.NotContains(t, out, "Move-trait Stride available")
}

func TestStrikeCommand_PromptTimeoutAutoSkips(t *testing.T) {
    h := newHandlerWithMobileWeapon(t, withAccountSetting("move_trait_auto_prompt", true))
    h.Run("strike thug")
    h.AdvanceTime(31 * time.Second)
    require.False(t, h.WielderQueue().HasFreeAction(combat.ActionMoveTraitStride),
        "30s timeout must auto-skip the Stride (WMOVE-22)")
}
```

- [ ] **Step 2: Implement** the three command shapes and the post-Strike prompt:

```go
func (h *Handler) handleStrike(args []string) string {
    target := args[0]
    weapon := h.session.EquippedWeapon()
    autoPrompt := h.session.Account.MoveTraitAutoPrompt
    if !weapon.HasTrait(traits.Mobile) || !autoPrompt {
        return h.dispatchStrike(target, nil)
    }
    h.dispatchStrike(target, nil)
    h.startStrideSubprompt(30 * time.Second)
    return "Move-trait Stride available. Specify destination cell or \"skip\"."
}
```

The subprompt is a small per-session state: `awaitingStrideUntil time.Time`. Subsequent input parses as a destination, `skip`, or times out. Per WMOVE-Q5, the Strike does NOT block the round — it has been queued already; the Stride is added if confirmed before the round resolves, or forfeited otherwise.

- [ ] **Step 3:** Add `MoveTraitAutoPrompt bool` to account settings, default `true`, and a settings command to toggle it.

---

### Task 7: Web UX — `MOVE` badge + two-stage placement

**Files:**
- Modify: `cmd/webclient/ui/src/combat/CombatActionBar.tsx`
- Create: `cmd/webclient/ui/src/combat/MoveTraitPlacement.tsx`
- Create: `cmd/webclient/ui/src/combat/MoveTraitPlacement.test.tsx`

- [ ] **Step 1: Failing component tests** per WMOVE-23 / WMOVE-24 / WMOVE-25 / WMOVE-26:

```ts
test("strike button shows MOVE badge for mobile weapon", () => {
  render(<CombatActionBar weaponTraits={["mobile"]} />);
  expect(screen.getByLabelText("Strike")).toContainElement(screen.getByText("MOVE"));
});

test("clicking strike opens two-stage placement", () => {
  render(<CombatActionBar weaponTraits={["mobile"]} />);
  fireEvent.click(screen.getByLabelText("Strike"));
  expect(screen.getByText("Stride placement")).toBeVisible();
  expect(screen.getByText("Skip Stride")).toBeVisible();
});

test("Esc on Stage 1 returns to Stage 2 with no Stride", async () => {
  render(<MoveTraitPlacement />);
  fireEvent.keyDown(document, { key: "Escape" });
  expect(screen.getByText("Choose target")).toBeVisible();
  // No move_trait_stride was sent yet.
});

test("Esc on Stage 2 cancels entire action", async () => {
  // confirm Stride first, then cancel target picker → no AP spent
  ...
});

test("legal Stride cells match server SpeedSquares Chebyshev radius", () => {
  // Inject server-known speed; verify only cells within radius are rendered.
  ...
});

test("Stride after Strike toggle swaps stages", async () => {
  ...
});
```

- [ ] **Step 2: Implement** `MoveTraitPlacement` as a two-stage modal/overlay:

```ts
type Stage = "stride" | "target" | "done";

const MoveTraitPlacement = ({ ordering, onConfirm, onCancel }: Props) => {
  const [stage, setStage] = useState<Stage>(ordering === "before" ? "stride" : "target");
  const [stride, setStride] = useState<Cell | "skip" | null>(null);
  const [target, setTarget] = useState<string | null>(null);

  // Esc handling
  useEffect(() => { ... }, []);

  // legal cells = SpeedSquares Chebyshev disc minus blocked/occupied
  const legalCells = useMemo(() => computeLegalStrideCells(actor), [actor]);

  return (
    <Overlay>
      {stage === "stride" && (
        <StridePlacement legalCells={legalCells} onPick={(cell) => { setStride(cell); setStage(ordering === "before" ? "target" : "done"); }} onSkip={() => { setStride("skip"); setStage(...); }} />
      )}
      {stage === "target" && (
        <TargetPicker onPick={(uid) => { setTarget(uid); setStage(...); }} />
      )}
      {stage === "done" && submit({ stride, target })}
    </Overlay>
  );
};
```

- [ ] **Step 3:** Add the badge to the action bar:

```tsx
{weaponTraits.includes("mobile") && <span className="trait-badge mobile">MOVE</span>}
```

- [ ] **Step 4:** Mirror server's `SpeedSquares()` cell computation client-side for preview (WMOVE-26). Server stays authoritative — the client preview is a hint.

---

### Task 8: Content — tag exemplar weapon

**Files:**
- Modify: `content/weapons/<chosen>.yaml`

- [ ] **Step 1: Checkpoint (WMOVE-30).** STOP and confirm with user which existing weapon receives the trait. Suggested defaults from the spec: reach polearms or whip-class weapons. Implementer MUST not pick unilaterally.

- [ ] **Step 2: Tag the chosen weapon** with `mobile` (or `move` if user picked Option B in Task 1) in its `traits:` list, and append a sentence to its `description` documenting the runtime behaviour (WMOVE-31).

- [ ] **Step 3: Smoke test** end-to-end — equip the tagged weapon, run `move-strike thug 6,5`, verify the free Stride and Strike both resolve correctly with no reactive strikes.

---

### Task 9: NPC integration via HTN

**Files:**
- Modify: `internal/game/ai/domain.go`
- Modify: `internal/game/ai/build_state.go`
- Modify: `internal/gameserver/combat_handler.go` (legacy fallback recognition)

- [ ] **Step 1: Failing tests**:

```go
func TestHTN_MoveTraitStrideActionShape(t *testing.T) {
    plan, _ := planner.Plan("dom_with_move_trait_stride", state)
    require.Contains(t, plan.Actions, ai.ActionMoveTraitStride{X: 6, Y: 5, Ordering: ai.OrderingBefore})
}

func TestLegacyAutoQueue_MoveTraitWielderPrefersFreeStride(t *testing.T) {
    cbt := combatWithMobileWieldingNPC()
    s.LegacyAutoQueueLocked(cbt, npc)
    queue := cbt.WielderQueue(npc.UID)
    require.Equal(t, combat.ActionMoveTraitStride, queue[0].Type, "NPC with mobile weapon must prefer free Stride")
}
```

- [ ] **Step 2: Add the HTN action shape** `move_trait_stride{x, y, ordering}`. Surface `MoveTraitWielder` in `WorldState`. HTN domains can read it and emit the action when appropriate.

- [ ] **Step 3: Legacy fallback (`legacyAutoQueueLocked`)**: when the NPC's weapon has `mobile` and `chooseMoveDestination` (#251) returns a destination cell, queue it as a free Stride before the next Strike instead of as a normal `ActionStride`. Without #251, the legacy fallback uses the existing `npcMovementStrideLocked` direction string and queues whatever cell is one step in that direction.

- [ ] **Step 4: Per-cell guard** (WMOVE-29): when both the free Stride and a normal Stride want the same destination, the free Stride wins; the normal Stride is dropped. When they want different cells, the free Stride executes first; the normal Stride re-evaluates from the new position.

---

### Task 10: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Weapon Traits" section** documenting:
  - The trait registry (`internal/game/inventory/traits/`) and its `Behavior` struct.
  - The `Mobile` trait (with `move` alias) and what it grants.
  - All 35 `WMOVE-N` requirements with one-line summaries.
  - The free-action plumbing (`ActionMoveTraitStride`, `QueueFreeAction`, allowlist).
  - The `MoveContext` parameter and the reaction-suppression rule.
  - The Strike pipeline ordering (BEFORE / AFTER) and the once-per-Strike enforcement.
  - The telnet `move-strike` / `strike-move` commands and the `move_trait_auto_prompt` account setting.
  - The web two-stage placement UI and the `MOVE` badge.
  - The NPC integration via HTN `move_trait_stride{x,y}` and the legacy-fallback preference.
  - Open question resolutions (Q1..Q5).

- [ ] **Step 2: Cross-link** to the spec, the trait registry, the `MoveContext` definition, and the chosen exemplar weapon's content file.

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
- Telnet smoke test: equip the exemplar weapon, run `strike thug` with `move_trait_auto_prompt = true`, confirm prompt appears; choose a destination; verify the free Stride and Strike resolve in the queued ordering with no reactive strikes; verify remaining AP equals what a non-Move Strike would have left.
- Web smoke test: action bar shows `MOVE` badge; clicking opens the two-stage placement; Esc on Stage 1 vs Stage 2 behaves per WMOVE-25; `Stride after Strike` toggle swaps stages.
- NPC smoke test: an NPC tagged with the exemplar weapon executes the free Stride end-to-end via HTN or the legacy fallback.

---

## Rollout / Open Questions Resolved at Plan Time

- **WMOVE-Q1**: Trait id default = `mobile`, with `move` aliased in the registry. Subject to user confirmation at Task 1.
- **WMOVE-Q2**: AoE persistent effects evaluate independently of `MoveContext`. Reaction-trigger consumers are the only consumers suppressed; non-reaction zone effects (e.g., a *Web* zone) still fire for the free Stride.
- **WMOVE-Q3**: One free Stride per Strike *action* (umbrella), even on multi-Strike actions. Confirmed default; subject to user override at Task 4.
- **WMOVE-Q4**: Off-hand attack with a non-Move weapon does NOT confer a free Stride; only the *attacking* weapon's traits matter.
- **WMOVE-Q5**: Telnet Strike queues immediately; the Stride sub-prompt is non-blocking. The Stride opportunity is forfeited if the prompt times out, the round resolves on schedule.

## Non-Goals Reaffirmed

Per spec §2.2:

- No PF2E weapon trait beyond `mobile` (registry shape supports the rest, but content + behaviour ship per ticket).
- No Step-only variant (Step and Stride share reaction suppression on this trait; v1 ships Stride only).
- No multi-Strike per-attack free Stride.
- No diagonal-cost geometry change (Chebyshev preserved).
- No general free-action support beyond the `ActionMoveTraitStride` allowlist.
- No authoring UI for trait tagging; YAML stays the source of truth.

# Reactions and Ready Actions

**Issue:** [#244 Feature: Reactions and Ready actions](https://github.com/cory-johannsen/mud/issues/244)
**Status:** Spec
**Date:** 2026-04-21

## 1. Summary

This spec adds three coordinated capabilities to the combat system:

1. A formal **reaction economy** — every combatant has a per-round reaction budget (default 1), trackable and forward-compatible with feats that grant extras.
2. A **synchronous timed prompt** so players choose whether to spend a reaction when a trigger fires, with a short deadline after which the reaction is skipped.
3. A new **Ready action** that prepares a delayed 1-AP action bound to a trigger; when the trigger fires, the prepared action auto-executes, consuming the player's reaction budget.

Reactions fire automatically for the trigger-source-detection side but are gated by the interactive prompt on the player side. Ready actions fire without prompting (the player committed when they Readied).

All three changes share the existing `internal/game/reaction/` registry plumbing and extend `internal/game/combat/` resolver flow.

## 2. Requirements

- REACTION-1: Every combatant MUST have a per-round reaction budget of `1 + sum(BonusReactions)` across their active feats.
- REACTION-2: The reaction budget MUST reset at round start and MUST NOT persist across rounds.
- REACTION-3: A reaction prompt MUST be dismissed after `ReactionPromptTimeout` elapses, and the pending reaction MUST be skipped.
- REACTION-4: Reaction budget MUST be refunded on explicit skip, timeout, or callback error. Budget is consumed only on successful effect application.
- REACTION-5: `ActionReady` MUST cost 2 AP and MUST prepare exactly one 1-AP action.
- REACTION-6: A Readied action MUST auto-fire without prompting when its trigger fires.
- REACTION-7: A Readied action MUST expire at end of round if its trigger has not fired.
- REACTION-8: Ready entries MUST be evaluated before feat reactions at each trigger point.
- REACTION-9: A Ready that fails re-validation at fire time MUST refund the reaction budget and emit an `EventReadyFizzled` event.
- REACTION-10: Feat reactions and Ready entries MUST share the same reaction budget.
- REACTION-11: The reaction prompt MUST be synchronous with respect to `ResolveRound` (the resolver blocks until the prompt returns or the deadline elapses).
- REACTION-12: `ReactionPromptTimeout` MUST be bounded to `[500ms, 30s]` at config load; out-of-range values MUST fall back to the default.
- REACTION-13: The default `ReactionPromptTimeout` MUST be 3 seconds.
- REACTION-14: NPC combatants MUST receive a reaction budget of exactly 1 per round; NPCs do not gain `BonusReactions`.
- REACTION-15: The allowed trigger menu for Ready MUST be `on_enemy_enters_room`, `on_enemy_move_adjacent`, and `on_ally_damaged`. Any other trigger MUST be rejected at enqueue time.
- REACTION-16: The allowed prepared-action whitelist for Ready MUST be `ActionAttack`, `ActionStride`, `ActionThrow`, `ActionReload`, and `ActionUseAbility`/`ActionUseTech` with `AbilityCost == 1`. Any other action MUST be rejected at enqueue time.
- REACTION-17: A reaction prompt MUST NOT open for a player who already has an outstanding prompt; concurrent triggers for the same player MUST be dropped silently.
- REACTION-18: On reaction fire (feat or Ready), the system MUST emit an `EventReactionFired` RoundEvent with `{uid, featName, effectSummary}` visible in the combat narrative.
- REACTION-19: Game commands typed by the player during the reaction window MUST be buffered and applied after the prompt resolves; chat/say input MUST NOT be buffered.

## 3. Architecture

### 3.1 Package boundaries

- `internal/game/reaction/` gains:
  - `Budget` type (per-combatant per-round reaction accounting).
  - `ReadyEntry` type (one prepared delayed action).
  - `ReadyRegistry` type (maps trigger → pending entries).
  - Updated `ReactionCallback` signature carrying a context deadline and candidate list.
  - `BonusReactions int` field on `ReactionDef`.
- `internal/game/combat/` gains:
  - `ActionReady` in the `ActionType` enum with cost 2.
  - `QueuedAction` fields `ReadyTrigger`, `ReadyAction *QueuedAction`, `ReadyTriggerTgt`.
  - `Combatant.ReactionBudget *reaction.Budget`, populated on add and reset at `AdvanceRound`.
  - Resolver hook at every trigger point: `ReadyRegistry.Consume` → budget check → feat-reaction callback.
- `internal/frontend/` (telnet): reaction prompt rendering and input buffering; Ready command parsing.
- `internal/webclient/` (web): `reaction_prompt` / `reaction_response` WS messages; modal UI; two-step Ready action selector; "R" budget badge.
- `internal/config/`: `ReactionPromptTimeout time.Duration` with the bounds check and 3s default.

### 3.2 Round lifecycle

1. **Round start (`AdvanceRound`):** For each combatant, recompute `Budget.Max = 1 + sum(ReactionDef.BonusReactions)` across active feats; set `Spent = 0`. NPCs: `Max = 1`. `ReadyRegistry.ExpireRound(prevRound)` prunes any still-pending entries from the prior round.
2. **Action submission:** Players enqueue actions, including `ActionReady` entries. On `ActionReady` enqueue: validate whitelist + trigger menu, deduct 2 AP, append the action to the queue for display, and register a `ReadyEntry` in the round's `ReadyRegistry`.
3. **`ResolveRound` execution:** At every trigger fire point, the resolver:
   1. Calls `ReadyRegistry.Consume(uid, trigger, sourceUID)`.
   2. If an entry is returned: `TrySpend` on budget; if fails, drop entry silently (budget already spent). Re-validate the prepared action; on failure refund budget and emit `EventReadyFizzled`. On success execute the prepared action inline via the same resolver subroutines a queued action uses, then emit `EventReactionFired`.
   3. If no Ready entry: look up candidate feat reactions in `ReactionRegistry` filtered by uid + trigger + requirement. If none, continue. Otherwise `TrySpend`; if fails, skip. Invoke the callback with `context.WithTimeout(ReactionPromptTimeout)` and the candidate list. Handle the four outcomes per §3.4.
4. **Round end:** `ReadyRegistry.ExpireRound(currentRound)` drops unfired entries. No persistence.

### 3.3 Reaction budget (`reaction.Budget`)

```go
type Budget struct {
    Max   int
    Spent int
}

func (b *Budget) Remaining() int         // Max - Spent
func (b *Budget) TrySpend() bool         // returns false if Spent >= Max
func (b *Budget) Refund()                // decrements Spent; floors at 0
func (b *Budget) Reset(max int)          // Max = max, Spent = 0
```

Invariants: `Max >= 0`; `0 <= Spent <= Max` at all observation points. `TrySpend` and `Refund` are the only public mutators post-construction.

### 3.4 Callback signature

```go
type ReactionCallback func(
    ctx context.Context,
    uid string,
    trigger ReactionTriggerType,
    rctx ReactionContext,
    candidates []PlayerReaction,
) (spent bool, chosen *PlayerReaction, err error)
```

Resolver outcomes:

- `(true, chosen, nil)` within deadline → apply `chosen.Def.Effect`, emit `EventReactionFired`, budget stays consumed.
- `(false, _, nil)` → player explicitly declined; `Budget.Refund()`; no event.
- `ctx.Err() == context.DeadlineExceeded` on return → timeout; `Budget.Refund()`; no event (silent skip).
- Non-nil `err` (not deadline) → log, `Budget.Refund()`, continue resolver; no event. Frontend errors MUST NOT propagate into round resolution.

### 3.5 Ready action

`QueuedAction` extension (only meaningful when `Type == ActionReady`):

```go
ReadyTrigger    reaction.ReactionTriggerType
ReadyAction     *QueuedAction
ReadyTriggerTgt string // optional narrowing to a specific source UID/name
```

`ReadyEntry`:

```go
type ReadyEntry struct {
    UID        string
    Trigger    ReactionTriggerType
    TriggerTgt string
    Action     ReadyActionDesc // minimal serializable shape to avoid combat->reaction import cycle
    RoundSet   int
}

type ReadyActionDesc struct {
    Type        string // "attack" | "stride" | "throw" | "reload" | "use_ability" | "use_tech"
    Target      string
    Direction   string
    WeaponID    string
    ExplosiveID string
    AbilityID   string
    AbilityCost int
}
```

`ReadyRegistry`:

```go
type ReadyRegistry struct { /* byTrigger map */ }

func (r *ReadyRegistry) Add(e ReadyEntry)
func (r *ReadyRegistry) Consume(uid string, trigger ReactionTriggerType, sourceUID string) *ReadyEntry
func (r *ReadyRegistry) ExpireRound(round int)
func (r *ReadyRegistry) Cancel(uid string) // called on pre-resolve action-queue edit
```

`Consume` matches by `uid + trigger`, additionally filtering by `TriggerTgt` when set. It atomically returns and removes the entry. `Cancel` removes all of a player's entries (used when a player clears their action queue pre-resolve).

### 3.6 Fire-point ordering

At each trigger point in `round.go`:

```
triggerFires(uid, trigger, rctx, sourceUID)
├─ ready := ReadyRegistry.Consume(uid, trigger, sourceUID)
├─ if ready != nil:
│    ├─ if !Budget.TrySpend(): drop silently
│    ├─ if !revalidate(ready): Budget.Refund(); emit EventReadyFizzled
│    └─ else: execute ready.Action; emit EventReactionFired
└─ else:
     ├─ candidates := ReactionRegistry.Filter(uid, trigger, requirement)
     ├─ if len(candidates) == 0: return
     ├─ if !Budget.TrySpend(): return
     ├─ ctx, cancel := context.WithTimeout(parent, ReactionPromptTimeout)
     ├─ spent, chosen, err := callback(ctx, uid, trigger, rctx, candidates)
     ├─ on success: apply chosen.Def.Effect; emit EventReactionFired
     └─ on decline/timeout/error: Budget.Refund()
```

### 3.7 Config

```go
// In config package
const DefaultReactionPromptTimeout = 3 * time.Second

type GameServerConfig struct {
    // ... existing fields ...
    ReactionPromptTimeout time.Duration `yaml:"reaction_prompt_timeout"`
}
```

On load: if zero, use default. If outside `[500ms, 30s]`, log a warning and clamp to the default. Hot-reload semantics match existing config fields (change takes effect on next round start).

## 4. UI / UX

### 4.1 Telnet — reaction prompt

Rendered on the console region via `WriteConsole`:

```
[REACTION] Shield Block: spend your reaction? [y/n] (3s)
```

Multi-candidate:

```
[REACTION] Choose (3s):  1) Shield Block  2) Parry  [1-2 or enter]
```

The countdown updates once per second by carriage-return-rewriting the same line. Input handling uses a per-session `reactionInputCh` consumed in a `select` against `ctx.Done()`. Non-reaction input typed during the window is buffered by the frontend and replayed after resolution. Chat/say input bypasses the buffer and is delivered immediately. A bell character (`\a`) is emitted on prompt open only if the player has `notify_bell` enabled.

### 4.2 Telnet — reaction fired notification

Rendered as a `RoundEvent` in the normal narrative stream:

```
[REACTION] Shield Block: absorbed 4 damage.
[REACTION] Ready: you attack the thug for 7 damage as it enters the room.
```

Third-person for reactions by other players:

```
Kira's Shield Block absorbs 4 damage.
```

### 4.3 Telnet — Ready command

```
ready <action> [args] when <trigger> [targeting <uid>]
  examples:
    ready attack goblin when enemy moves adjacent
    ready stride toward when enemy enters room
    ready throw grenade when enemy enters room
```

`help ready` lists the trigger menu in plain words (`enemy enters room`, `enemy moves adjacent`, `ally damaged`).

### 4.4 Telnet — budget indicator

Prompt row gains an `R:N` suffix after `AP:N`:

```
AP:3 R:1
```

Transitions to `R:0` when spent, back to `R:1` at round start.

### 4.5 Web — reaction prompt

WS message from server:

```json
{
  "type": "reaction_prompt",
  "prompt_id": "rx-abc123",
  "deadline_ms": 1713724800000,
  "options": [
    { "id": "shield_block", "label": "Shield Block" }
  ]
}
```

Client renders a modal overlay: title, option buttons, a linear progress bar tied to `deadline_ms`, and a Skip button. On client-side timeout the modal auto-closes.

WS response from client:

```json
{
  "type": "reaction_response",
  "prompt_id": "rx-abc123",
  "chosen": "shield_block"
}
```

`chosen: null` indicates skip. The server treats missing or late responses the same as timeout. `prompt_id` scopes responses so a stale late reply for a prior prompt cannot race the next one.

### 4.6 Web — reaction fired notification

Emitted on the existing combat-log WS stream. A small icon keyed to `effect.type` (shield / sword / dice) is shown next to the line. Icon assets are implementation details deferred to the plan.

### 4.7 Web — Ready action UI

Two-step action-bar selector:

1. Click **Ready** → primary action bar dims; a secondary bar shows eligible 1-AP actions.
2. Pick the prepared action → a trigger picker appears with the fixed menu.
3. Confirm → 2 AP deducted; action queued; `ReadyEntry` registered.

The queued Ready renders in the action queue as a pill:

```
Ready: Attack (when enemy enters room) ⨯
```

The ⨯ cancels the Ready, refunds 2 AP, and removes the `ReadyEntry`. Cancel is allowed only before `ResolveRound` starts for the current round.

### 4.8 Web — reaction budget indicator

A small "R" badge adjacent to the AP tracker:

- Filled when `Remaining() > 0`.
- Struck through when `Spent == Max`.
- Tooltip: `"1 reaction per round"` or `"{Max} reactions per round ({feat name})"` when `Max > 1`.

## 5. Data Model Changes

### 5.1 `ReactionDef` YAML

```yaml
reaction:
  triggers: [on_damage_taken]
  requirement: wielding_shield
  effect:
    type: reduce_damage
  bonus_reactions: 0   # NEW: default 0
```

`BonusReactions` is summed across all of a player's active feats at round start. Per-trigger `BonusReactions` is *not* supported — the value is a flat additive to `Max`.

### 5.2 Persistence

No persistence layer changes. `Budget` and `ReadyRegistry` are round-scoped and reconstructed each round from feat state.

### 5.3 Protobuf / gRPC

New `reaction_prompt` / `reaction_response` message types on the web WS path. The gRPC gameserver does not need new RPCs for reactions — the prompt is routed via the existing per-session streaming channel from frontend / webclient to gameserver.

## 6. Testing

Per SWENG-5 / SWENG-5a, TDD with property-based tests where applicable.

- `internal/game/reaction/budget_test.go` — property tests on random `(Max, spend, refund)` sequences; invariants: `0 <= Spent <= Max`, `TrySpend` idempotent when `Spent == Max`, `Refund` is a no-op when `Spent == 0`.
- `internal/game/reaction/ready_registry_test.go` — property tests: add/consume/expire sequences; no entry outlives its round; `Consume` atomically removes; `Cancel` removes all of a UID's entries.
- `internal/game/combat/action_ready_test.go` — table tests for enqueue validation (whitelist, trigger menu, recursion rejection, AP cost, ability-cost==1 invariant).
- `internal/game/combat/round_ready_test.go` — end-to-end resolver tests: Ready-Attack fires on `on_enemy_enters_room`; Ready-Stride fires and consumes budget; Ready fizzles when target is dead at fire time; Ready vs feat-reaction contention (Ready fires first, feat lookup skipped).
- `internal/game/combat/round_reaction_budget_test.go` — budget cap enforcement: second trigger in same round is silently dropped; feat with `BonusReactions: 1` gets two reactions; NPC always gets 1.
- `internal/game/combat/round_reaction_prompt_test.go` — resolver test with a stub callback covering: (a) respond within deadline → effect applied; (b) decline → budget refunded, no event; (c) deadline elapses → budget refunded, no event; (d) callback returns error → budget refunded, round continues.
- Frontend (telnet) integration tests: prompt rendering, countdown update, input buffering, bell opt-in.
- Webclient integration tests: `reaction_prompt` WS ingestion, modal render, `reaction_response` emission, timeout handling, stale `prompt_id` rejection.

## 7. Documentation

- `docs/architecture/combat.md`: add a "Reaction economy" section and a "Ready action" section; update the round-resolution diagram to show the Ready → budget → prompt → effect path; host the `REACTION-N` requirements defined in §2.
- Existing feat / tech authoring docs: update the `reaction:` YAML example to include `bonus_reactions`.

## 8. Non-Goals (v1)

- No per-player timeout override.
- No reaction budget persistence across combats or sessions (round-scoped, stateless).
- No new canonical reactions authored (Shield Block / Strike of Opportunity / Parry content are separate tickets).
- No per-NPC reaction budget customization — bosses get the default 1 plus any NPC-granted `BonusReactions` (NPCs do not read `BonusReactions` in v1, per REACTION-14).
- No free-form trigger predicate language for Ready — fixed menu only (REACTION-15).
- No client-side reaction UI customization (hotkey rebinding, modal position, etc.).
- No "held" reactions across multiple rounds — everything is round-scoped.
- No reaction economy for AoE / saving-throw reactions beyond what the existing `ReactionEffect` types already support.

## 9. Open Questions

None at spec time — all design questions raised during brainstorming were resolved. Open items below are for the planner to decide during plan authoring, not for the spec to dictate:

- Exact file paths in `internal/config/` and whether `GameServerConfig` or a nested sub-struct hosts `ReactionPromptTimeout`.
- Whether `ReadyActionDesc → QueuedAction` translation lives in `combat` or a small bridge package.
- Telnet prompt row redraw strategy (carriage-return vs re-render the whole status line).
- Web modal component location and styling alignment with existing combat UI.

---
title: Detection States — Observed, Concealed, Hidden, Undetected, Unnoticed, Invisible
issue: https://github.com/cory-johannsen/mud/issues/254
date: 2026-04-24
status: spec
prefix: DETECT
depends_on: []
related:
  - "#247 Cover bonuses (sister positional condition)"
  - "#249 Targeting system (validation pipeline shares the per-pair check)"
  - "#252 Non-combat actions (Create-a-Diversion etc. drive detection transitions)"
  - "#267 Visibility / LOS (occlusion sets the upper bound on detectability)"
---

# Detection States

## 1. Summary

Replace the current single boolean `Combatant.Hidden` with the full PF2E **detection ladder** as a *per-pair asymmetric* relationship: every (observer, target) pair carries one of six states — `Observed`, `Concealed`, `Hidden`, `Undetected`, `Unnoticed`, `Invisible`. Each state has distinct attack-resolution gating (flat checks, miss chances, square-guessing) and distinct effects on what the observer perceives in the world view they receive from the server.

Today's plumbing is sparse but pointed:
- `Combatant.Hidden bool` (`internal/game/combat/combat.go:134`) is a single global flag — symmetrically true for all observers.
- A DC-11 flat-check gate is already implemented for the NPC-attacks-Hidden-player case in `internal/game/combat/round.go:812-842`, with three pinning tests in `round_hidden_test.go`.
- `handleHide` (`grpc_service.go:11432`) uses the `stealth` skill vs the maximum NPC Perception in the room and sets the global `Hidden` flag.
- `handleSeek` (`:11481`) flips an NPC out of `Hidden` for one round via `RevealedUntilRound`.
- `content/conditions/hidden.yaml` and `undetected.yaml` exist as documents only — no runtime behavior beyond what `Hidden` already provides.
- No per-pair state, no concealed/invisible/unnoticed/observed enumeration, no Undetected square-guessing, no visibility filter on outbound `RoomView`, no Stealth-vs-Perception initiative, no LoS blocker plumbing for "Concealed by darkness".

This spec adds the asymmetric state, the resolver gating for every state, the square-guessing target shape, the visibility filter on outbound views, and the transition handlers (Hide, Sneak, Seek, Avoid Notice, Create-a-Diversion). It explicitly **does not** ship a line-of-sight occlusion model; that is #267's job. Detection state and LoS interact (LoS-blocked → state cannot be `Observed`) but compose cleanly: the detection layer treats LoS as an upstream input, and #267 lands a `Cell.Opaque` model that this spec's transition rules will read once available.

## 2. Goals & Non-Goals

### 2.1 Goals

- DETECT-G1: Every ordered pair `(observer, target)` of combatants carries exactly one detection state from the six-element enum.
- DETECT-G2: Attack resolution honors the attacker→target detection state via a single uniform gate, replacing the ad-hoc `Hidden` flat-check at `round.go:812-842`.
- DETECT-G3: When the attacker→target state is `Undetected` or `Invisible`-without-sound-cues, the attacker MUST submit a target *square* (not a name); a wrong guess automatically misses without revealing the actual position.
- DETECT-G4: Outbound `RoomView` and combat-state messages are filtered per-recipient so an observer only learns what their pair-state permits.
- DETECT-G5: The existing `Hidden` flag and DC-11 flat check continue to work — three pinning tests in `round_hidden_test.go` MUST pass unchanged via a back-compat shim (`Hidden == true` is migrated to "every player observer sees this NPC as Hidden, every NPC observer sees this player as Hidden").
- DETECT-G6: Transition actions (Hide, Sneak, Seek, Avoid Notice, Create-a-Diversion) drive the per-pair state via Stealth checks vs Perception DCs.
- DETECT-G7: Initiative-time integration: an Unnoticed creature rolls Stealth (not Perception) for initiative, mirroring PF2E.
- DETECT-G8: Detection state changes are reflected as conditions on the target so the existing condition pipeline (durations, expiries, badges) applies without parallel plumbing.

### 2.2 Non-Goals

- DETECT-NG1: Line-of-sight occlusion. `Cell.Opaque`, walls, smoke, fog, and darkness models belong to #267. This spec uses an `OcclusionProvider` interface that #267 implements; v1 ships a no-op implementation that always returns "clear LoS".
- DETECT-NG2: Sound-based detection mechanics beyond the binary "this Invisible target makes noise / does not". A future ticket can layer audible Stealth checks.
- DETECT-NG3: Senses other than sight: scent, tremorsense, blindsight, lifesense. Add per-Combatant senses in a follow-on ticket; v1 assumes ordinary vision.
- DETECT-NG4: Multi-room / cross-room detection. Detection is intra-combat (single room) only.
- DETECT-NG5: Authoring UI for editing per-pair state. State is computed from actions and conditions, not hand-edited.
- DETECT-NG6: Replacing existing `handleHide` / `handleSeek` proto messages. The handlers are migrated internally; their wire shapes stay the same.

## 3. Glossary

- **Detection state**: one of `Observed`, `Concealed`, `Hidden`, `Undetected`, `Unnoticed`, `Invisible`. Strictly ordered by *information loss* from the observer's perspective.
- **Observer**: the combatant whose perspective we are computing.
- **Target**: the combatant being perceived.
- **Pair-state**: the detection state of `(observer, target)`. Asymmetric.
- **Square-guess**: a target shape that names a grid cell rather than a combatant; used when targeting an `Undetected` or `Invisible-no-sound` target.
- **Flat check**: a d20 roll vs a fixed DC, independent of skills. PF2E's `Concealed` uses DC 5; `Hidden` uses DC 11; no flat check for `Undetected` (the cost is the wrong-square guess).
- **Off-guard**: the existing condition (per spec #252's catalog) — `-2 circumstance AC` against the affected attacker. Hidden, Undetected, and Unnoticed targets render the observer off-guard at attack time.

## 4. Requirements

### 4.1 Per-Pair State Model

- DETECT-1: A new package `internal/game/detection/` MUST define:
  - `type State int` with constants `Observed`, `Concealed`, `Hidden`, `Undetected`, `Unnoticed`, `Invisible` (ordered as listed).
  - `func (State) String() string` returning the lowercase name.
  - `func (State) MissChancePercent() int` returning `0` for `Observed`, `20` for `Concealed`, etc., per §4.2.
- DETECT-2: A new struct `Map` in the same package MUST hold the per-pair table. Its API:
  - `Get(observerUID, targetUID string) State` — defaults to `Observed` when the pair is absent.
  - `Set(observerUID, targetUID string, s State)` — stores the state.
  - `Clear(observerUID, targetUID string)` — removes the entry, reverting to `Observed`.
  - `ForObserver(observerUID string) iter.Seq2[string, State]` — iterates all targets seen by the observer.
- DETECT-3: A new field `DetectionStates *detection.Map` MUST be added to `Combat` in `internal/game/combat/combat.go`. It MUST be initialized to an empty map at combat start.
- DETECT-4: The asymmetry rule MUST hold: `Get(A, B)` and `Get(B, A)` are independent. No code path may assume mutual visibility.
- DETECT-5: A back-compat shim MUST migrate the existing `Combatant.Hidden bool` flag at combat start: when `c.Hidden == true`, for every other combatant `o`, `Map.Set(o.UID, c.UID, Hidden)` is called. The `Hidden` field MUST remain on `Combatant` for one release cycle and remain readable, but new writes go through `Map.Set` only.

### 4.2 Attack Resolution Gating

- DETECT-6: The current ad-hoc DC-11 flat-check block in `round.go:812-842` MUST be removed and replaced by a single function `detection.GateAttack(attacker, target *Combatant, state State, dice dice.Roller) GateResult` returning one of `GateProceed`, `GateAutoMiss`, `GateOffGuard`, or a combination flag.
- DETECT-7: The gate MUST implement the following per state, in this order:
  - `Observed`: `GateProceed`. No flat check, no miss chance, no off-guard.
  - `Concealed`: roll a DC-5 flat check; on failure, `GateAutoMiss`. On success, `GateProceed`.
  - `Hidden`: roll a DC-11 flat check; on failure, `GateAutoMiss`. On success, `GateProceed | GateOffGuard` (target is off-guard against this attacker).
  - `Undetected`: only valid when the attack uses a square-guess (DETECT-15); when the guess matches `target.GridX/GridY`, roll the DC-11 flat check (same as `Hidden`); otherwise `GateAutoMiss`. Always `GateOffGuard` regardless of guess.
  - `Unnoticed`: equivalent to `Undetected` for combat purposes (the combat layer never sees Unnoticed mid-combat — it converts to `Undetected` at initiative; included here for completeness).
  - `Invisible`: when the attacker has a sound cue (target made a sound this round — see DETECT-19), treat as `Hidden`; otherwise treat as `Undetected`.
- DETECT-8: `GateOffGuard` MUST be applied via the existing condition pipeline by adding the `off_guard` condition (per spec #252) to the target for the duration of this attack only. The condition is removed in the same resolver call. This avoids parallel "transient -2 AC" plumbing.
- DETECT-9: The three existing tests in `round_hidden_test.go` MUST pass unchanged after migration.
- DETECT-10: A new test file `internal/game/combat/round_detection_test.go` MUST cover one scenario per state (Concealed flat-check fail, Concealed flat-check pass, Hidden flat-check fail, Hidden flat-check pass, Undetected wrong-square, Undetected right-square + flat-check fail, Undetected right-square + flat-check pass + off-guard applied, Invisible with sound cue → Hidden behavior, Invisible without sound cue → Undetected behavior).

### 4.3 Square-Guess Targeting

- DETECT-11: A new oneof field MUST be added to the existing strike/attack proto messages: either `target string` (current behavior — combatant name) or `target_square Cell` (square-guess). The combat resolver MUST accept either shape.
- DETECT-12: When the resolver receives `target_square` and the cell is empty (no combatant), `GateAutoMiss` MUST fire with narrative `"Your attack hits empty <terrain>."`.
- DETECT-13: When the resolver receives `target_square` and the cell is occupied by a combatant whose attacker→target detection state is `Observed`, `Concealed`, or `Hidden`, the attack MUST be rejected with `"You can see <name>; target by name instead."` to prevent square-guess from becoming a generic LOS workaround.
- DETECT-14: Server response on `Undetected` square-guess MUST NOT reveal whether the cell was correct or empty. Both produce the same narrative `"Your attack hits empty <terrain>."` — the wrong-square miss is indistinguishable from a flat-check fail. The combat log entry MAY include the truth for GM debugging behind a server flag.
- DETECT-15: When the attacker has no observable target visible at attack time (the action bar shows `???` for `Undetected` targets), the client MUST present a square-picker UI. Telnet uses `attack-at <x> <y>` and `strike-at <x> <y>` commands; web uses the existing AoE template-placement cell-picker (single-cell variant).

### 4.4 Outbound View Filter

- DETECT-16: A new function `detection.FilterRoomView(rv *gamev1.RoomView, observerUID string, dm *detection.Map) *gamev1.RoomView` MUST be applied at every server→client send site that emits `RoomView` or `CombatStateView` to a given player. The filter MUST:
  - Strip combatants whose state is `Unnoticed`.
  - Replace name and stat block of `Undetected` combatants with `???`; keep their grid cell hidden.
  - Show `Hidden` combatants as `<silhouette>` at their grid cell.
  - Show `Concealed` combatants normally but with a `(concealed)` annotation.
  - Show `Observed` combatants normally.
  - Show `Invisible` combatants the same as `Undetected` unless the observer has a sense that pierces invisibility (out of scope, NG3); the framework hook MUST be present.
- DETECT-17: The filter MUST be deterministic and idempotent.
- DETECT-18: `FilterRoomView` MUST be called once per recipient per send. Server-internal state remains unfiltered — only the wire payload is filtered.

### 4.5 Sound Cue Tracking

- DETECT-19: A new `Combatant.MadeSoundThisRound bool` field MUST be added. It is reset to `false` at the start of each round and set to `true` whenever the combatant takes any action that PF2E classifies as `auditory`: Strike (any), Reload, Stride, MoveTo, any action with the `auditory` trait per the future trait registry (per spec #253's plumbing).
- DETECT-20: The `Invisible` state's gate (DETECT-7) consults `target.MadeSoundThisRound` to decide between `Hidden` (sound cue present) and `Undetected` (silent) treatment.
- DETECT-21: A new `Sneak` action type (already referenced in proto at `:2518` per research) MUST be implemented to perform a Stride that does NOT set `MadeSoundThisRound`. AP cost matches PF2E: 1 AP. Distance: half Speed (rounded down to cells). Requires the actor to be `Hidden` or `Undetected` to at least one observer; otherwise rejected.

### 4.6 Transition Actions

- DETECT-22: `handleHide` MUST be migrated from setting the global `Hidden` flag to setting per-pair state. The action computes the actor's Stealth check, then for each enemy in the room with LoS (via `OcclusionProvider`, no-op v1), compares against that enemy's Perception DC; on success the per-pair state for `(enemy, actor)` becomes `Hidden`, on failure it remains `Observed`.
- DETECT-23: `handleSeek` MUST iterate every combatant whose state from the seeker's perspective is worse than `Observed`, roll the seeker's Perception vs the target's Stealth DC, and on success advance the state by one rung toward `Observed` (Concealed → Observed; Hidden → Concealed; Undetected → Hidden; Unnoticed → Undetected).
- DETECT-24: `Sneak` (per DETECT-21) MUST chain Stealth checks vs each enemy's Perception DC after the Stride, allowing the actor to remain Hidden while moving.
- DETECT-25: `Avoid Notice` (a new exploration action — out-of-combat path) MUST set the actor's initiative to be rolled with Stealth instead of Perception; on success against each enemy's Perception, the actor enters combat at `Unnoticed` to that enemy.
- DETECT-26: `Create a Diversion` (per spec #252's NCA-32 catalog) MUST, on success, set the per-pair state for `(target, actor)` to `Hidden` for the actor's next attack. The state reverts to whatever it was before after the next attack resolves, success or fail.
- DETECT-27: Damage MUST advance state toward `Observed`: when an attacker damages a target while the attacker is at state worse than `Observed` from the target's perspective, the (target, attacker) pair state MUST advance one rung toward `Observed` immediately after damage applies.

### 4.7 Initiative Integration

- DETECT-28: `RollInitiative` (`internal/game/combat/initiative.go:18`) MUST consult the actor's `InitiativeRollMode` enum (`Perception` default, `Stealth` set by `Avoid Notice`); when `Stealth`, the actor rolls `1d20 + StealthBonus` instead of `1d20 + PerceptionMod`.
- DETECT-29: When at least one combatant entered combat at `Unnoticed` to one or more others, the initiative phase MUST evaluate per-pair starting states based on the Stealth-vs-Perception margin: a margin of ≥0 yields `Unnoticed`; margin of -1 to -10 yields `Hidden`; worse than -10 yields `Observed`.
- DETECT-30: There is no separate "surprise round". Per PF2E, the unnoticed actor simply acts during their initiative position with the per-pair state advantages already baked in. This is the explicit rule, not an oversight.

### 4.8 Conditions Bridge

- DETECT-31: Each detection state MUST also be expressed as a condition in `content/conditions/`:
  - `observed.yaml` (no effects — informational).
  - `concealed.yaml` (effects: attacker DC-5 flat check before damage).
  - `hidden.yaml` (effects: attacker DC-11 flat check before damage; off-guard against attacker).
  - `undetected.yaml` (effects: attacker must square-guess; off-guard against attacker).
  - `unnoticed.yaml` (effects: target absent from observer's view).
  - `invisible.yaml` (effects: hidden-or-undetected branch on sound cue).
- DETECT-32: The condition's `effects` MUST reference the detection state via a typed-bonus (per spec #245's bonuses model) tagged `kind: detection_state`. The condition pipeline applies the effect by calling `detection.Map.Set` rather than by adding a numeric modifier.
- DETECT-33: The existing `content/conditions/hidden.yaml` and `undetected.yaml` MUST be rewritten to match this schema; the documented-only versions are replaced by enforceable versions.

### 4.9 Observability & Telemetry

- DETECT-34: Every state transition MUST emit a structured log entry: `combat_id`, `observer_uid`, `target_uid`, `from_state`, `to_state`, `cause` (action id or `damage` / `sound` / `damage_advance`).
- DETECT-35: The combat log shown to players MUST narrate state transitions in their own perspective only (e.g., `"You lose sight of the gunner."` when an enemy goes Hidden to the player; never reveal that the player is Hidden to a specific NPC).

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/detection/
  state.go                # State enum + helpers
  map.go                  # Map type with Get/Set/Clear/ForObserver
  gate.go                 # GateAttack(attacker, target, state, dice) GateResult
  filter.go               # FilterRoomView(rv, observerUID, dm)
  occlusion.go            # OcclusionProvider interface (no-op v1; #267 fills it)
  state_test.go, map_test.go, gate_test.go, filter_test.go

internal/game/combat/
  combat.go               # existing; Combat.DetectionStates *detection.Map field;
                          #   Combatant.Hidden retained one release as back-compat
  round.go                # existing; ad-hoc DC-11 block at :812-842 removed,
                          #   replaced by detection.GateAttack call
  initiative.go           # existing; honor InitiativeRollMode (Perception | Stealth)
  round_detection_test.go # NEW: full state matrix coverage

internal/gameserver/
  combat_handler.go       # existing; Hide/Seek/Sneak migrate to per-pair updates
  grpc_service.go         # existing; handleHide / handleSeek migrated;
                          # handleSneak added; outbound RoomView pipes through filter

api/proto/game/v1/game.proto
  StrikeRequest / AttackRequest gain oneof { string target | Cell target_square }
  RoomView combatants gain optional `redacted_as` string ("???" / "<silhouette>" / "")

content/conditions/
  observed.yaml, concealed.yaml, hidden.yaml, undetected.yaml,
  unnoticed.yaml, invisible.yaml — all rewritten to reference detection_state
```

### 5.2 Attack flow with detection gate

```
client: StrikeRequest{ target: "name" } or { target_square: {x,y} }
   │
   ▼
handleStrike (grpc_service.go) → combatH.Strike → ActionStrike queued
   │
   ▼
ResolveRound (round.go)
   │
   ▼
For each Strike action:
   pairState = combat.DetectionStates.Get(attackerUID, targetUID)
   gate = detection.GateAttack(attacker, target, pairState, dice)
   if gate has GateOffGuard:
       attach off_guard condition to target for this attack only
   if gate == GateAutoMiss:
       emit "miss" narrative, no damage, possibly don't reveal
       skip attack roll
   else:
       resolveAttack(attacker, target, src) — existing path unchanged
       on damage: detection.AdvanceTowardObserved(combat.DetectionStates, target, attacker)
   reset off_guard if attached
```

### 5.3 Outbound view flow

```
server about to send RoomView to playerUID
   │
   ▼
filtered := detection.FilterRoomView(rv, playerUID, combat.DetectionStates)
   │
   ▼
send filtered to client
```

### 5.4 Single sources of truth

- Per-pair state: `Combat.DetectionStates`. No duplicate storage on Combatant.
- State semantics: `internal/game/detection/state.go` constants + `gate.go` rules.
- Condition catalog: `content/conditions/*.yaml` — six detection conditions.
- LoS occlusion: `OcclusionProvider` interface, no-op v1, filled by #267.

## 6. Open Questions

- DETECT-Q1: PF2E's `Unnoticed` is technically only a pre-encounter state — once combat starts, it is supposed to convert to `Undetected` on the first action. The spec preserves `Unnoticed` as an in-combat state for the initiative phase only (DETECT-29). Should it be removable mid-combat by any action other than the actor's own (e.g., does an Investigator's first attack also reveal nearby Unnoticed allies)? Recommendation: only the actor's own auditory action converts their own `Unnoticed → Undetected`. Allies of the unnoticed actor stay Unnoticed independently.
- DETECT-Q2: When player A is Hidden to NPC X but Observed to NPC Y, does an AoE template (#250) placed by X "see" A? Recommendation: yes — AoE effect application is positional, not perceptional. The attacker placed the template against X's worldview, but the resolver applies effects to every combatant in the actual cell set regardless of pair-state. The "wrong square" gating applies only to single-target Undetected attacks.
- DETECT-Q3: How do reactions (#244) interact with detection? Recommendation: a reaction's trigger fires only when the reactor's pair-state to the trigger source is at least `Hidden` (i.e., reactor knows the source's location). `Concealed` and `Observed` allow reactions; `Undetected` and `Unnoticed` and `Invisible-without-sound` block them. Implementer should confirm with the user — the rule is a judgment call.
- DETECT-Q4: The spec retains `Combatant.Hidden bool` for one release as back-compat. When does it actually get deleted? Recommendation: delete in the next minor release after this lands; add a `// DEPRECATED` marker now and a tracker issue.
- DETECT-Q5: NCA-32's `create_a_diversion` outcome (per spec #252) currently says "all enemies are off-guard to actor's next attack this turn"; with detection in play it should become "actor becomes Hidden to all enemies for the actor's next attack this turn" per DETECT-26. This conflicts with #252's literal text. Recommendation: defer to whichever of #252 / #254 lands second; the implementer of the second one MUST adjust accordingly.

## 7. Acceptance

- [ ] `Combat.DetectionStates` is initialized empty; every absent pair returns `Observed`.
- [ ] All three pinned `round_hidden_test.go` tests pass after migration via the back-compat shim (DETECT-5).
- [ ] All ten new `round_detection_test.go` scenario tests pass (one per state branch per DETECT-10).
- [ ] Property tests under `internal/game/detection/testdata/rapid/` cover map round-trip, gate determinism, and filter idempotence.
- [ ] An end-to-end manual test on telnet and web demonstrates: a player Hides → NPCs no longer see them; player Strikes from concealment → attack resolves with off-guard applied; player misses Hide check → state remains Observed; an Invisible NPC remains hidden until it Strikes (sound cue) at which point it becomes Hidden to the player; player attempts to attack an Undetected NPC by name and is forced to use square-guess.
- [ ] Outbound `RoomView` for two players in the same combat differs based on their per-pair states (verified by capturing the wire payloads and diffing).
- [ ] Initiative test with one combatant Avoid-Noticing demonstrates Stealth-rolled initiative and per-pair `Unnoticed` start state.

## 8. Out-of-Scope Follow-Ons

- DETECT-F1: Senses other than sight (per NG3): blindsight, scent, tremorsense, lifesense.
- DETECT-F2: Audible-only Stealth (sound flat-check independent of vision).
- DETECT-F3: Conditional invisibility — items that grant Invisible only against specific creature types.
- DETECT-F4: Group Stealth checks (party-wide Avoid Notice with a single roll).
- DETECT-F5: Detection states across rooms (peeking through doorways, leaning out of cover).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/254
- Existing global Hidden flag: `internal/game/combat/combat.go:134`
- Existing DC-11 flat check (to be replaced): `internal/game/combat/round.go:812-842`
- Pinning tests: `internal/game/combat/round_hidden_test.go:47,103,149`
- Hide handler: `internal/gameserver/grpc_service.go:11432`
- Seek handler: `internal/gameserver/grpc_service.go:11481`
- Initiative: `internal/game/combat/initiative.go:18` (`RollInitiative`)
- Existing condition catalog: `content/conditions/hidden.yaml`, `undetected.yaml`
- PF2E reference: `vendor/pf2e-data/packs/pf2e/conditions/{observed,concealed,hidden,undetected,unnoticed,invisible}.json`
- Cover spec (sister positional condition): `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
- Skill actions catalog (Create-a-Diversion, Sneak references): `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md`
- AoE template placement (square-picker UI shared pattern): `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- Off-guard condition (used by Hidden/Undetected): `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md` NCA-4
- Conditions back-compat to typed bonuses: `docs/superpowers/specs/2026-04-21-duplicate-effects-handling.md`

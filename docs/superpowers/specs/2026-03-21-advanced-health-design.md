# Advanced Health Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `advanced-health` (priority 360)
**Dependencies:** `persistent-calendar` (calendar tick for time-based transitions)

---

## Overview

Models drugs, alcohol, medicine, poisons, and toxins as data-driven consumable substances with onset delays, duration-limited effects, overdose thresholds, and an addiction/withdrawal/recovery state machine. Substance effects are implemented as temporary `condition.ConditionDef` applications, reusing the existing condition system. No new database tables are required — substance state is session-only (lost on disconnect, same as combat state).

---

## 1. Substance Definition Model

Substances are defined in YAML files loaded from `content/substances/`. Each file defines one `SubstanceDef`.

### 1.1 SubstanceDef Structure

```yaml
id: jet
name: Jet
category: drug            # drug | alcohol | medicine | poison | toxin
onset_delay: "0s"         # Go duration; 0 = immediate effect
duration: "30m"           # Go duration; how long effects last after onset
effects:
  - apply_condition: speed_boost
    stacks: 1
  - apply_condition: jittery
    stacks: 1
remove_on_expire:
  - speed_boost
  - jittery
addictive: true
addiction_chance: 0.15    # probability per dose while at_risk
overdose_threshold: 3     # doses within duration window that triggers overdose
overdose_condition: overdosed
withdrawal_conditions:
  - withdrawal_craving
  - nausea
recovery_duration: "2h"   # wall-clock duration from last dose to clean
```

### 1.2 SubstanceEffect

A `SubstanceEffect` MUST have exactly one non-zero action field. The four valid action fields are:

- REQ-AH-0A: `SubstanceEffect.apply_condition string` MUST specify a condition ID to apply to the player on onset. `stacks int` (default 1) specifies the stack count.
- REQ-AH-0B: `SubstanceEffect.remove_condition string` MUST specify a condition ID to remove from the player on onset.
- REQ-AH-0C: `SubstanceEffect.hp_regen int` MUST specify HP added per `tickSubstances` call while the substance is active (medicine only; see Section 7).
- REQ-AH-0D: `SubstanceEffect.cure_conditions []string` MUST specify condition IDs removed immediately on onset, not deferred per tick (medicine only; see Section 7).

### 1.3 Substance Condition Reference Validation

- REQ-AH-1: `SubstanceDef` MUST be loaded from `content/substances/*.yaml`. Each file MUST define exactly one substance.
- REQ-AH-2: `SubstanceDef` MUST have fields: `id`, `name`, `category` (`"drug"|"alcohol"|"medicine"|"poison"|"toxin"`), `onset_delay` (Go duration string), `duration` (Go duration string), `effects []SubstanceEffect`, `remove_on_expire []string`, `addictive bool`, `addiction_chance float64`, `overdose_threshold int`, `overdose_condition string`, `withdrawal_conditions []string`, `recovery_duration` (Go duration string).
- REQ-AH-3: A `SubstanceRegistry` MUST be loaded at server startup (alongside the condition registry) and injected into `GameServiceServer`.
- REQ-AH-4: `SubstanceDef.Validate()` MUST return an error if: `id` is empty, `name` is empty, `category` is not one of the five valid values, `onset_delay` or `duration` or `recovery_duration` is not a valid Go duration string, `addiction_chance` is not in `[0.0, 1.0]`, or `overdose_threshold < 1`.
- REQ-AH-4A: At server startup, after both the condition registry and substance registry are loaded, the server MUST cross-validate all condition ID references in every `SubstanceDef` (in `effects[*].apply_condition`, `effects[*].remove_condition`, `effects[*].cure_conditions`, `remove_on_expire`, `overdose_condition`, and `withdrawal_conditions`) against the condition registry. Any unknown condition ID MUST cause a fatal startup error.

---

## 2. Active Substance Tracking

Player sessions track consumed substances in a new `ActiveSubstances []ActiveSubstance` slice on `PlayerSession`. This is session-only state; no persistence is required.

**On disconnect:** All active substance effects and addiction state are cleared. On reconnect, the player starts with a clean substance state. This is intentional — the implementation cost of persisting transient session state does not justify the benefit at this stage of development. The player receives no reconnect message about lost substance state.

### 2.1 ActiveSubstance

```go
type ActiveSubstance struct {
    SubstanceID    string
    DoseCount      int
    OnsetAt        time.Time // wall-clock time when effects apply
    ExpiresAt      time.Time // wall-clock time when effects expire
    EffectsApplied bool      // true after onset has fired
}
```

### 2.2 Addiction State

Each player session gains an `AddictionState map[string]SubstanceAddiction` field on `PlayerSession`, keyed by substance ID.

```go
type SubstanceAddiction struct {
    Status          string    // "at_risk" | "addicted" | "withdrawal"
    WithdrawalUntil time.Time // zero if not in withdrawal
}
```

### 2.3 Condition Reference Counting

When multiple active substances apply the same condition ID, the condition must not be removed by the first substance to expire. A new `SubstanceConditionRefs map[string]int` field on `PlayerSession` tracks how many active substances have applied each condition ID. A condition is removed from `sess.Conditions` only when its ref count drops to zero.

- REQ-AH-5: `PlayerSession` MUST gain `ActiveSubstances []ActiveSubstance` (zero value: nil slice).
- REQ-AH-6: `PlayerSession` MUST gain `AddictionState map[string]SubstanceAddiction` (zero value: nil map, lazily initialized on first write).
- REQ-AH-6A: `PlayerSession` MUST gain `SubstanceConditionRefs map[string]int` (zero value: nil map, lazily initialized on first write). This map counts how many active substances have applied each condition ID.
- REQ-AH-7: Substance state (ActiveSubstances, AddictionState, SubstanceConditionRefs) MUST be session-only. No persistence to the database is required. All three fields are cleared implicitly when the session ends.

---

## 3. Consumption & Dose Resolution

When a player uses a substance item (via the existing `use` command on an inventory item), the item's `effect` field references a substance ID. The `use` command handler looks up the substance and, after verifying the category, calls `applySubstanceDose(uid, substanceDef)`.

- REQ-AH-8: Inventory items that are substances MUST have an `effect` field containing a valid substance ID. The `use` command handler MUST look up the substance in `SubstanceRegistry`.
- REQ-AH-8A: The `use` command handler MUST return an error to the player (`"You can't use that directly."`) and MUST NOT call `applySubstanceDose` if the substance `category` is `"poison"` or `"toxin"`. The guard lives in the `use` handler, not inside `applySubstanceDose`.
- REQ-AH-9: `applySubstanceDose(uid string, def *SubstanceDef)` MUST find or create an `ActiveSubstance` entry for `def.ID` in `sess.ActiveSubstances`. If the substance is already active, `DoseCount` MUST be incremented and `ExpiresAt` MUST be extended to `time.Now().Add(def.OnsetDelay + def.Duration)`. If not active, a new entry MUST be created with `DoseCount = 1`, `OnsetAt = time.Now().Add(def.OnsetDelay)`, `ExpiresAt = time.Now().Add(def.OnsetDelay + def.Duration)`.
- REQ-AH-10: If `DoseCount > def.OverdoseThreshold`, `applySubstanceDose` MUST immediately apply the condition named by `def.OverdoseCondition` to the player's `Conditions`. The player MUST receive the message `"You've taken too much. Your body reacts violently."`.
- REQ-AH-11: If `def.Addictive == true`, `applySubstanceDose` MUST update addiction state: if current status is `""` (clean/no entry), set to `"at_risk"`. If `"at_risk"`, roll `rand.Float64() < def.AddictionChance`; on success, set status to `"addicted"` and send `"You feel a gnawing need for more."`. If already `"addicted"`, see REQ-AH-18 for re-roll behavior.

---

## 4. Substance Tick Processing

Substance onset, expiry, and withdrawal transitions are driven by a per-session wall-clock ticker. The existing `roomRefreshTicker` pattern (5-second `time.Ticker` in the session goroutine) is extended to also call `tickSubstances(uid)`.

- REQ-AH-12: The session goroutine's 5-second wall-clock ticker MUST call `tickSubstances(uid)` on every tick in addition to its existing room refresh logic.
- REQ-AH-13: `tickSubstances(uid string)` MUST iterate `sess.ActiveSubstances` and:
  - For each entry where `!EffectsApplied && time.Now().After(OnsetAt)`: apply all `effects` from the `SubstanceDef` (apply/remove conditions, start hp_regen, cure conditions), increment `SubstanceConditionRefs[condID]` for each applied condition, set `EffectsApplied = true`, and send the player a message `"The <name> kicks in."`.
  - For each entry where `EffectsApplied && time.Now().After(ExpiresAt)`: for each condition ID in `def.RemoveOnExpire`, decrement `SubstanceConditionRefs[condID]`; remove the condition from `sess.Conditions` only if the ref count drops to 0. Remove the `ActiveSubstance` entry from the slice. Call `onSubstanceExpired(uid, def)`.
- REQ-AH-14: HP regen from medicine substances MUST apply on every `tickSubstances` call while `EffectsApplied == true && time.Now().Before(ExpiresAt)`. The regen MUST be clamped to `sess.MaxHP`. The player MUST NOT receive a message for each regen tick (silent healing).
- REQ-AH-15: `onSubstanceExpired(uid string, def *SubstanceDef)` MUST check addiction state. If `AddictionState[def.ID].Status == "addicted"`, it MUST set status to `"withdrawal"`, set `WithdrawalUntil = time.Now().Add(def.RecoveryDuration)`, apply all conditions in `def.WithdrawalConditions` (incrementing refs), and send the player message `"You feel sick without your fix."`.

---

## 5. Addiction & Withdrawal State Machine

```
clean ──dose──> at_risk ──roll──> addicted ──expire──> withdrawal ──time──> clean
                                      ^                    │
                                      └────────────dose────┘  (resets WithdrawalUntil, suppresses symptoms)
```

- REQ-AH-16: `tickSubstances` MUST check `AddictionState` for each substance with `status == "withdrawal"`. When `time.Now().After(WithdrawalUntil)`, MUST decrement refs and remove withdrawal conditions from `sess.Conditions`, set status to `""` (clean), and send `"You feel like yourself again."`.
- REQ-AH-17: If a player consumes an addictive substance while `status == "withdrawal"`, `applySubstanceDose` MUST: reset `WithdrawalUntil = time.Now().Add(def.RecoveryDuration)`, decrement refs and remove `def.WithdrawalConditions` from `sess.Conditions` (dose suppresses symptoms), and set status back to `"addicted"`.
- REQ-AH-18: If a player consumes an addictive substance while `status == "addicted"`, `applySubstanceDose` MUST roll `rand.Float64() < def.AddictionChance`. On success, the player MUST receive the message `"Your dependency deepens."`. Status remains `"addicted"` regardless.
- REQ-AH-19: Addiction state per substance MUST be independent. `AddictionState` is keyed by substance ID; one substance's state does not affect another's.

---

## 6. Poison & Toxin Application

Poisons and toxins reach players via weapons, traps, and NPC abilities — not by voluntary consumption. A new `ApplySubstanceByID(uid, substanceID string)` server method allows any system to apply a substance without going through inventory.

- REQ-AH-20: `ApplySubstanceByID(uid, substanceID string) error` MUST look up `substanceID` in `SubstanceRegistry` and call `applySubstanceDose` directly (bypassing the `use` handler category guard). Returns an error if the substance is not found.
- REQ-AH-21: Poisoned weapon items MUST have a `poison_substance_id string` field (added to the item data model). The attack pipeline MUST call `ApplySubstanceByID` after dealing damage on a hit when `poison_substance_id` is non-empty.
- REQ-AH-22: Trap definitions MUST support a `substance_id string` field. When a trap trigger fires, it MUST call `ApplySubstanceByID` in addition to dealing damage if `substance_id` is non-empty.

---

## 7. Medicine

Medicine is consumed voluntarily and has only beneficial effects. The `category == "medicine"` substances use the `cure_conditions` and `hp_regen` effect fields.

- REQ-AH-24: A `SubstanceEffect` with `cure_conditions []string` MUST remove each listed condition from `sess.Conditions` immediately on onset (not deferred per tick), decrementing `SubstanceConditionRefs` accordingly.
- REQ-AH-25: A `SubstanceEffect` with `hp_regen int` MUST add `hp_regen` HP per `tickSubstances` call while the substance is active and `EffectsApplied == true`. HP MUST be clamped to `sess.MaxHP`.
- REQ-AH-26: `SubstanceDef.Validate()` MUST return an error if `category == "medicine"` and `addictive == true`.
- REQ-AH-27: Medicine substances MAY set `overdose_threshold` to model medicine overdose. No code validation is required for this case.

---

## 8. Example Substances

### 8.1 Drug: Jet

```yaml
id: jet
name: Jet
category: drug
onset_delay: "0s"
duration: "20m"
effects:
  - apply_condition: speed_boost
    stacks: 1
  - apply_condition: tunnel_vision
    stacks: 1
remove_on_expire:
  - speed_boost
  - tunnel_vision
addictive: true
addiction_chance: 0.20
overdose_threshold: 3
overdose_condition: stimulant_overdose
withdrawal_conditions:
  - fatigue
  - nausea
recovery_duration: "4h"
```

### 8.2 Alcohol: Cheap Whiskey

```yaml
id: cheap_whiskey
name: Cheap Whiskey
category: alcohol
onset_delay: "30s"
duration: "15m"
effects:
  - apply_condition: drunk
    stacks: 1
remove_on_expire:
  - drunk
addictive: false
overdose_threshold: 5
overdose_condition: alcohol_poisoning
withdrawal_conditions: []
recovery_duration: "0s"
```

### 8.3 Medicine: Stimpak

```yaml
id: stimpak
name: Stimpak
category: medicine
onset_delay: "0s"
duration: "5m"
effects:
  - hp_regen: 2
remove_on_expire: []
addictive: false
overdose_threshold: 10
overdose_condition: stimulant_overdose
withdrawal_conditions: []
recovery_duration: "0s"
```

### 8.4 Poison: Viper Venom

```yaml
id: viper_venom
name: Viper Venom
category: poison
onset_delay: "10s"
duration: "3m"
effects:
  - apply_condition: poisoned
    stacks: 1
remove_on_expire:
  - poisoned
addictive: false
overdose_threshold: 1
overdose_condition: severe_poisoning
withdrawal_conditions: []
recovery_duration: "0s"
```

---

## 9. Requirements Summary

- REQ-AH-0A: `SubstanceEffect.apply_condition` MUST specify a condition ID to apply on onset; `stacks int` (default 1) specifies the stack count.
- REQ-AH-0B: `SubstanceEffect.remove_condition` MUST specify a condition ID to remove on onset.
- REQ-AH-0C: `SubstanceEffect.hp_regen int` MUST specify HP per tick while active (medicine only).
- REQ-AH-0D: `SubstanceEffect.cure_conditions []string` MUST specify condition IDs removed immediately on onset (medicine only).
- REQ-AH-1: `SubstanceDef` MUST be loaded from `content/substances/*.yaml`.
- REQ-AH-2: `SubstanceDef` MUST have the fields listed in Section 1.1.
- REQ-AH-3: `SubstanceRegistry` MUST be loaded at startup and injected into `GameServiceServer`.
- REQ-AH-4: `SubstanceDef.Validate()` MUST reject empty `id`/`name`, invalid `category`, invalid duration strings, `addiction_chance` outside `[0,1]`, or `overdose_threshold < 1`.
- REQ-AH-4A: At startup, all condition ID references in every `SubstanceDef` MUST be cross-validated against the condition registry; any unknown ID MUST cause a fatal startup error.
- REQ-AH-5: `PlayerSession` MUST gain `ActiveSubstances []ActiveSubstance`.
- REQ-AH-6: `PlayerSession` MUST gain `AddictionState map[string]SubstanceAddiction`.
- REQ-AH-6A: `PlayerSession` MUST gain `SubstanceConditionRefs map[string]int` for reference counting conditions applied by active substances.
- REQ-AH-7: Substance state MUST be session-only; no DB persistence; all fields cleared on disconnect.
- REQ-AH-8: Substance items MUST have an `effect` field with a substance ID; `use` handler MUST look up the substance.
- REQ-AH-8A: The `use` handler MUST block `category == "poison"` or `"toxin"` substances with `"You can't use that directly."` before calling `applySubstanceDose`.
- REQ-AH-9: `applySubstanceDose` MUST create or increment `ActiveSubstance` entries, extending `ExpiresAt` on re-dose.
- REQ-AH-10: `DoseCount > overdose_threshold` MUST immediately apply `overdose_condition` and send overdose message.
- REQ-AH-11: Addictive dose MUST advance: clean→at_risk, at_risk→addicted (probabilistic with message), addicted→see REQ-AH-18.
- REQ-AH-12: Session 5-second ticker MUST call `tickSubstances(uid)`.
- REQ-AH-13: `tickSubstances` MUST fire onset (apply effects, increment refs, send "kicks in") and expiry (decrement refs, remove zero-ref conditions, call `onSubstanceExpired`).
- REQ-AH-14: Medicine hp_regen MUST apply per tick while active; clamped to `MaxHP`; no per-tick message.
- REQ-AH-15: `onSubstanceExpired` MUST trigger withdrawal if `status == "addicted"`: set withdrawal, apply withdrawal conditions (incrementing refs), send withdrawal message.
- REQ-AH-16: `tickSubstances` MUST check `WithdrawalUntil` expiry; on expiry, remove withdrawal conditions (decrement refs), set status to clean, send recovery message.
- REQ-AH-17: Dose while in withdrawal MUST reset `WithdrawalUntil`, remove withdrawal conditions (decrement refs), set status back to `"addicted"`.
- REQ-AH-18: Dose while `status == "addicted"` MUST re-roll addiction chance; on success send `"Your dependency deepens."`; status stays `"addicted"`.
- REQ-AH-19: Addiction state MUST be independent per substance ID.
- REQ-AH-20: `ApplySubstanceByID(uid, substanceID) error` MUST call `applySubstanceDose` directly (bypassing the `use` handler category guard).
- REQ-AH-21: Poisoned weapon items MUST have `poison_substance_id string`; attack pipeline MUST call `ApplySubstanceByID` on hit.
- REQ-AH-22: Trap definitions MUST support `substance_id string`; trap triggers MUST call `ApplySubstanceByID` if non-empty.
- REQ-AH-24: `cure_conditions` effects MUST remove listed conditions immediately on onset, decrementing refs.
- REQ-AH-25: `hp_regen` effects MUST add HP per tick while active, clamped to `MaxHP`.
- REQ-AH-26: `SubstanceDef.Validate()` MUST reject `category == "medicine"` with `addictive == true`.
- REQ-AH-27: Medicine `overdose_threshold` enforcement is a content decision; no code validation required.

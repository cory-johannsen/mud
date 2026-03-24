# Activate Item Command — Design Spec

## Overview

Adds an `activate <item>` command that triggers effects on equipped charged-gear items (weapons, armor, accessories). Items have a configurable charge count, AP cost, effect (consumable-style or Lua script), and optional multi-trigger recharge configuration. Charge state is persisted per item instance.

---

## 1. Scope

- REQ-ACT-1: `activate <item>` MUST only resolve items currently equipped in a weapon slot (main/off-hand), armor slot, or accessory slot (neck, rings).
- REQ-ACT-2: Items with `ActivationCost == 0` MUST NOT be activatable; attempting to activate them MUST return an error.
- REQ-ACT-3: Items with `ChargesRemaining == 0` and `Expended == true` MUST NOT be activatable.
- REQ-ACT-4: In combat, `ActivationCost` AP MUST be deducted before applying the effect; insufficient AP MUST return an error without consuming charges.
- REQ-ACT-5: Out of combat, AP cost is informational only (no AP pool to deduct from).

---

## 2. Data Model

### 2.1 ItemDef additions (`internal/game/inventory/item.go`)

New fields on the existing `ItemDef` struct (in `item.go`):

```go
// ActivationCost is the AP cost to activate this item (1–3). 0 = not activatable (default).
ActivationCost int `yaml:"activation_cost,omitempty"`
// Charges is the initial and maximum charge count. Must be > 0 when ActivationCost > 0.
Charges int `yaml:"charges,omitempty"`
// OnDeplete controls item fate when ChargesRemaining reaches 0.
// "destroy" removes the item permanently; "expend" leaves it in slot as inactive.
// Ignored when Recharge is non-empty (expend semantics always apply).
OnDeplete string `yaml:"on_deplete,omitempty"`
// ActivationScript is the Lua hook name to invoke on activation.
// Mutually exclusive with ActivationEffect.
ActivationScript string `yaml:"activation_script,omitempty"`
// ActivationEffect holds consumable-style effects for non-scripted activation.
// Mutually exclusive with ActivationScript.
ActivationEffect *ConsumableEffect `yaml:"activation_effect,omitempty"`
// Recharge lists triggers that restore charges to this item.
Recharge []RechargeEntry `yaml:"recharge,omitempty"`
```

New types in `item.go`:

```go
// RechargeEntry defines one recharge trigger for an item.
type RechargeEntry struct {
    // Trigger is when this recharge fires.
    // Valid values: "daily", "midnight", "dawn", "rest".
    Trigger string `yaml:"trigger"`
    // Amount is the number of charges restored. Must be > 0.
    Amount int `yaml:"amount"`
}
```

Validation requirements (enforced in `ItemDef.Validate()`):

- REQ-ACT-6: `ActivationCost` MUST be in [0, 3]; values outside this range MUST be a fatal load error.
- REQ-ACT-7: `Charges` MUST be > 0 when `ActivationCost > 0`; mismatch MUST be a fatal load error.
- REQ-ACT-8: `OnDeplete` MUST be `""`, `"destroy"`, or `"expend"`; any other value MUST be a fatal load error. Empty defaults to `"expend"`.
- REQ-ACT-9: `ActivationScript` and `ActivationEffect` MUST NOT both be non-nil/non-empty on the same item; mismatch MUST be a fatal load error.
- REQ-ACT-10: Each `Recharge[].Trigger` MUST be one of `"daily"`, `"midnight"`, `"dawn"`, `"rest"`; unknown values MUST be a fatal load error.
- REQ-ACT-11: Each `Recharge[].Amount` MUST be > 0; zero or negative values MUST be a fatal load error.
- REQ-ACT-12: When `Recharge` is non-empty, `OnDeplete` is ignored; depleted items MUST use `"expend"` semantics (stay in slot, `Expended = true`).

### 2.2 ItemInstance additions (`internal/game/inventory/backpack.go`)

New fields on the existing `ItemInstance` struct:

```go
// ChargesRemaining is the number of activations remaining.
// -1 = uninitialized sentinel; initialized to ItemDef.Charges on first activation.
ChargesRemaining int
// Expended is true when ChargesRemaining == 0 and the item uses expend semantics.
Expended bool
```

- REQ-ACT-13: `ChargesRemaining == -1` sentinel MUST be initialized to `ItemDef.Charges` on first activation attempt, following the same pattern as `Durability == -1`.

### 2.3 ActivateSession interface (`internal/game/inventory/activate.go`)

`HandleActivate` operates on this interface to avoid importing `internal/game/session`. It embeds `ConsumableTarget` so the handler can pass it directly to `ApplyConsumable`.

```go
// ActivateSession provides the equipped item view needed by HandleActivate
// and satisfies ConsumableTarget so effects can be applied by the caller.
//
// Precondition: all methods return valid, non-nil values.
type ActivateSession interface {
    ConsumableTarget // GetTeam, ApplyHeal, ApplyCondition, RemoveCondition, ApplyDisease, ApplyToxin, GetStatModifier

    // EquippedInstances returns all ItemInstances currently equipped.
    // The implementation MUST resolve instances from:
    //   1. WeaponPreset.MainHand.InstanceID → backpack lookup
    //   2. WeaponPreset.OffHand.InstanceID → backpack lookup
    //   3. Equipment.Armor[slot].InstanceID → backpack lookup (all non-nil slots)
    //   4. Equipment.Accessories[slot].InstanceID → backpack lookup (all non-nil slots)
    // Items whose InstanceID is not found in the backpack are silently skipped.
    EquippedInstances() []*ItemInstance
}
```

### 2.4 Database

New migration `migrations/037_item_instance_charges.up.sql`:
```sql
ALTER TABLE character_inventory_instances
  ADD COLUMN IF NOT EXISTS charges_remaining INTEGER NOT NULL DEFAULT -1,
  ADD COLUMN IF NOT EXISTS expended BOOLEAN NOT NULL DEFAULT FALSE;
```

New migration `migrations/037_item_instance_charges.down.sql`:
```sql
ALTER TABLE character_inventory_instances
  DROP COLUMN IF EXISTS charges_remaining,
  DROP COLUMN IF EXISTS expended;
```

- REQ-ACT-14: `charges_remaining` and `expended` MUST be persisted and restored on restart.

---

## 3. Pure Functions (`internal/game/inventory/activate.go`)

```go
// ActivateResult is the outcome of a successful HandleActivate call.
type ActivateResult struct {
    AP              int              // AP cost consumed (equals ItemDef.ActivationCost)
    ItemDefID       string           // ID of the activated item's def
    ActivationEffect *ConsumableEffect // non-nil if consumable-style effect should be applied
    Script          string           // non-empty if Lua hook should be invoked
    Destroyed       bool             // true if item was destroyed on depletion
}

// HandleActivate resolves query across all equipped item instances, validates charge
// and AP state, decrements ChargesRemaining, and marks the item Expended or Destroyed.
// It does NOT apply effects or persist state — the caller does both.
//
// Precondition: sess, reg non-nil; query non-empty; currentAP >= 0.
// Postcondition: On success, ChargesRemaining is decremented in the matched
// ItemInstance and ActivateResult is returned with errMsg == "".
// On failure, ItemInstance is unchanged and errMsg is non-empty.
func HandleActivate(sess ActivateSession, reg *Registry, query string, inCombat bool, currentAP int) (result ActivateResult, errMsg string)

// TickRecharge restores charges to all item instances whose ItemDef has a recharge
// entry matching trigger. Charges are capped at ItemDef.Charges. Expended is
// cleared when charges become > 0 after recharge.
// Returns the subset of instances that were modified.
//
// Precondition: instances and reg non-nil; trigger is a known RechargeEntry.Trigger value.
// Postcondition: ChargesRemaining and Expended are updated in-place on modified instances.
func TickRecharge(instances []*ItemInstance, reg *Registry, trigger string) []*ItemInstance
```

- REQ-ACT-15: `HandleActivate` and `TickRecharge` MUST be pure functions; all DB persistence in the caller.
- REQ-ACT-16: `HandleActivate` MUST NOT import `internal/game/session`; it operates only on the `ActivateSession` interface.

---

## 4. Command Handler (`internal/gameserver/grpc_service_activate.go`)

New proto message at field `113` in `ClientMessage` oneof (`api/proto/game/v1/game.proto`):

```proto
message ActivateItemRequest {
  string item_query = 1;
}

// In ClientMessage oneof payload:
ActivateItemRequest activate_item = 113;
```

Handler `handleActivate` in `grpc_service_activate.go`:

1. Calls `HandleActivate` — on error, emits error event and returns.
2. Deducts AP in combat (handler responsibility, not pure function's).
3. If `result.Script != ""`: calls Lua scripting manager with the hook name.
4. Else if `result.ActivationEffect != nil`: calls `ApplyConsumable(sess, syntheticDef, rng)` where `syntheticDef` is an `*ItemDef` with `Effect` set to `result.ActivationEffect` and `Team` set from the original `ItemDef` via registry lookup. (`ApplyConsumable` reads `def.Effect`; the synthetic def MUST populate `Effect`, not `ActivationEffect`.)
5. If `result.Destroyed`: removes the item instance from the equipped slot, persists the equipment change to DB.
6. Persists updated `ChargesRemaining` and `Expended` to DB (even if not destroyed).
7. Emits narrative event to player.

Weapon slot destruction (addressing Issue 5): when a weapon (main/off-hand `EquippedWeapon`) is destroyed, its preset slot is set to `nil`. No combat-state side effects are triggered beyond the slot becoming empty — the player is left unarmed for that slot.

- REQ-ACT-17: The handler MUST persist charge state to DB after every successful activation.
- REQ-ACT-18: If the item is destroyed, the handler MUST remove it from the equipped slot and persist the equipment change before emitting the event.
- REQ-ACT-19: Destroyed weapon slots MUST be set to nil in the active preset; no other combat-state changes are required.

---

## 5. Recharge Wiring

### 5.1 Period-transition state

`GameServiceServer` gains a `lastTimePeriod TimePeriod` field protected by a dedicated `itemTickMu sync.Mutex`. This mutex is owned exclusively by the item-tick goroutine and is separate from the session lock. On each calendar tick, the item-tick goroutine locks `itemTickMu`, compares `dt.Hour.Period()` to `lastTimePeriod`, fires any period transitions, and updates `lastTimePeriod` before unlocking.

- REQ-ACT-21: `lastTimePeriod` MUST be initialized to the current game period at server start to prevent spurious triggers on the first tick.

### 5.2 Time-based triggers (`daily`, `midnight`, `dawn`)

Implemented in a new `grpc_service_item_ticks.go` file, wired into the existing `GameCalendar` subscriber pattern:

```go
// On each calendar tick (dt GameDateTime, lastPeriod *TimePeriod):
currentPeriod := dt.Hour.Period()

// "daily": fires once per game day at hour 0, which is also PeriodMidnight.
// Items that want both daily AND midnight recharge must use two separate entries.
// "daily" fires unconditionally at Hour==0; "midnight" fires on period transition.
if dt.Hour == 0 {
    s.tickItemRecharge("daily")
}
if currentPeriod != *lastPeriod {
    if currentPeriod == PeriodMidnight {
        s.tickItemRecharge("midnight")
    }
    if currentPeriod == PeriodDawn {
        s.tickItemRecharge("dawn")
    }
    *lastPeriod = currentPeriod
}
```

Note: `"daily"` and `"midnight"` both fire at `Hour == 0` if an item has both entries — this is intentional. Items that want only one of the two must use only that trigger.

`tickItemRecharge(trigger string)` iterates all online sessions, calls `TickRecharge` on each player's equipped item instances, persists updated charge state for modified instances, and emits a notification to the player for each recharged item.

Note: when `dt.Hour == 0`, the `GameCalendar` has already advanced `day`/`month` before broadcasting, so the `"daily"` trigger fires on the new day's date. `TickRecharge` does not use date context, so this has no behavioral impact.

- REQ-ACT-20: Period-based triggers (`midnight`, `dawn`) MUST fire only on period transition, not on every hour tick within the period.

### 5.3 Rest trigger

`GameServiceServer` exposes `RechargeOnRest(uid string)` in `grpc_service_activate.go` — calls `TickRecharge` with `"rest"` for that player's equipped instances and persists changes. Implemented now with a real body (not a stub); it simply has no callers until the Resting feature is implemented.

- REQ-ACT-22: `RechargeOnRest` MUST be a real, tested implementation callable at the Resting feature boundary without code changes.

---

## 6. Architecture Summary

| Layer | File | Responsibility |
|-------|------|---------------|
| Pure logic | `internal/game/inventory/activate.go` | `HandleActivate`, `TickRecharge`, `ActivateSession`, `ActivateResult` |
| Pure tests | `internal/game/inventory/activate_test.go` | Property-based + unit tests |
| ItemDef/Instance | `internal/game/inventory/item.go`, `backpack.go` | New fields + validation; `RechargeEntry` type in `item.go` |
| Proto | `api/proto/game/v1/game.proto` | `ActivateItemRequest` at field `113` |
| Handler | `internal/gameserver/grpc_service_activate.go` | `handleActivate`, `RechargeOnRest` |
| Tick wiring | `internal/gameserver/grpc_service_item_ticks.go` | `tickItemRecharge`, calendar subscriber, `lastTimePeriod` |
| Handler tests | `internal/gameserver/grpc_service_activate_test.go` | TDD handler tests |
| Migration up | `migrations/037_item_instance_charges.up.sql` | Add `charges_remaining`, `expended` columns |
| Migration down | `migrations/037_item_instance_charges.down.sql` | Drop columns |
| Feature doc | `docs/features/actions.md` | Mark Activate Item complete |

---

## 7. Testing

- Property-based tests (rapid) on `HandleActivate`: charge decrement invariants, AP deduction, depletion behavior (destroy vs. expend), script vs. effect routing, sentinel initialization.
- Property-based tests on `TickRecharge`: charges never exceed max, `Expended` cleared correctly on recharge, trigger matching, multiple entries for same item.
- Handler tests: full round-trip with mock `ActivateSession`, proto dispatch, AP deduction in combat, DB persistence after activation, destroyed weapon slot becomes nil.
- Tick tests: `tickItemRecharge` fires period triggers only on transition, `daily` fires at `Hour == 0`.

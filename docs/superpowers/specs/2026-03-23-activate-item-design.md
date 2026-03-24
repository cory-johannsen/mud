# Activate Item Command — Design Spec

## Overview

Adds an `activate <item>` command that triggers effects on equipped charged-gear items (weapons, armor, accessories). Items have a configurable charge count, AP cost, effect (consumable-style or Lua script), and optional multi-trigger recharge configuration. Charge state is persisted per item instance.

---

## 1. Scope

- REQ-ACT-1: `activate <item>` MUST only resolve items currently equipped in a weapon slot (main/off-hand), armor slot, or accessory slot (neck, rings).
- REQ-ACT-2: Items with `activation_cost == 0` MUST NOT be activatable; attempting to activate them MUST return an error.
- REQ-ACT-3: Items with `ChargesRemaining == 0` and `Expended == true` MUST NOT be activatable.
- REQ-ACT-4: In combat, `activation_cost` AP MUST be deducted before applying the effect; insufficient AP MUST return an error without consuming charges.
- REQ-ACT-5: Out of combat, AP cost is informational only (no AP pool to deduct from).

---

## 2. Data Model

### 2.1 ItemDef additions (YAML-loaded)

```yaml
activation_cost: 2           # AP cost (1–3). 0 = not activatable (default).
charges: 3                   # initial and maximum charge count.
on_deplete: "expend"         # "destroy" | "expend". Ignored if recharge is non-empty.
activation_script: ""        # Lua hook name. Mutually exclusive with effect fields below.
# Effect fields (reuse existing consumable effect fields):
heal_hp: 0
apply_conditions: []
remove_conditions: []
apply_disease: null
apply_toxin: null
# Recharge config:
recharge:
  - trigger: "daily"   # fires at Hour == 0 each game day
    amount: 1
  - trigger: "midnight"  # fires when period transitions to Midnight
    amount: 1
  - trigger: "dawn"      # fires when period transitions to Dawn
    amount: 1
  - trigger: "rest"      # fires when player completes a rest (stubbed until Resting feature)
    amount: 2
```

- REQ-ACT-6: `activation_cost` MUST be in [0, 3]; values outside this range MUST be a fatal load error.
- REQ-ACT-7: `charges` MUST be > 0 when `activation_cost > 0`; mismatch MUST be a fatal load error.
- REQ-ACT-8: `on_deplete` MUST be `"destroy"` or `"expend"`; any other value MUST be a fatal load error.
- REQ-ACT-9: `activation_script` and effect fields MUST NOT both be non-zero on the same item; mismatch MUST be a fatal load error.
- REQ-ACT-10: Each `recharge[].trigger` MUST be one of `"daily"`, `"midnight"`, `"dawn"`, `"rest"`; unknown values MUST be a fatal load error.
- REQ-ACT-11: Each `recharge[].amount` MUST be > 0; zero or negative values MUST be a fatal load error.
- REQ-ACT-12: When `recharge` is non-empty, `on_deplete` is ignored; depleted items MUST use `"expend"` semantics (stay in slot).

### 2.2 ItemInstance additions

```go
// ChargesRemaining is the number of activations remaining. -1 = not yet initialized (use ItemDef.Charges).
ChargesRemaining int
// Expended is true when ChargesRemaining == 0 and on_deplete == "expend".
Expended bool
```

- REQ-ACT-13: `ChargesRemaining == -1` sentinel MUST be initialized to `ItemDef.Charges` on first use (same pattern as `Durability == -1`).

### 2.3 Database

New migration `037_item_instance_charges.up.sql`:
```sql
ALTER TABLE character_inventory_instances
  ADD COLUMN IF NOT EXISTS charges_remaining INTEGER NOT NULL DEFAULT -1,
  ADD COLUMN IF NOT EXISTS expended BOOLEAN NOT NULL DEFAULT FALSE;
```

- REQ-ACT-14: `charges_remaining` and `expended` MUST be persisted and restored on restart.

---

## 3. Pure Functions (`internal/game/inventory/activate.go`)

```go
// ActivateResult is the outcome of HandleActivate.
type ActivateResult struct {
    AP          int    // AP cost consumed
    Message     string // success/failure message
    Script      string // non-empty if effect is script-based (caller must execute)
    Destroyed   bool   // true if item was destroyed on depletion
}

// HandleActivate resolves, validates, and charges an activatable item.
// It does NOT apply effects — the caller applies consumable effects or executes
// the Lua script returned in ActivateResult.Script.
//
// Precondition: sess, reg non-nil; query non-empty.
// Postcondition: On success, ChargesRemaining is decremented (and item marked
// Expended or destroyed per config). Returns error string on failure.
func HandleActivate(sess ActivateSession, reg *Registry, query string, inCombat bool, currentAP int) (ActivateResult, string)

// TickRecharge restores charges to all item instances in the slice
// whose ItemDef has a recharge entry matching trigger.
// Charges are capped at ItemDef.Charges. Expended is cleared when charges > 0.
//
// Precondition: instances non-nil; trigger is a valid RechargeT rigger constant.
// Postcondition: ChargesRemaining and Expended updated in-place; returns modified instances.
func TickRecharge(instances []*ItemInstance, defs *Registry, trigger string) []*ItemInstance
```

- REQ-ACT-15: `HandleActivate` and `TickRecharge` MUST be pure functions; all DB persistence in caller.
- REQ-ACT-16: `HandleActivate` MUST NOT import `internal/game/session`; it operates on an `ActivateSession` interface.

---

## 4. Command Handler (`internal/gameserver/grpc_service_activate.go`)

New proto message added to `ClientMessage` oneof:
```proto
message ActivateItemRequest {
  string item_query = 1;
}
```

Handler `handleActivate` in `grpc_service_activate.go`:
1. Calls `HandleActivate` — returns result or error event.
2. If `result.Script != ""`: calls Lua scripting manager with the hook name.
3. Else: calls `ApplyConsumable` with the item's effect fields and the player session as target.
4. If `result.Destroyed`: removes item instance from equipped slot and persists.
5. Persists updated `ChargesRemaining` and `Expended` to DB.
6. Emits narrative event to player.

- REQ-ACT-17: The handler MUST persist charge state after every activation.
- REQ-ACT-18: If the item is destroyed, the handler MUST remove it from the equipped slot and persist the equipment change.

---

## 5. Recharge Wiring

### Time-based triggers (`daily`, `midnight`, `dawn`)

Wired into the existing `GameCalendar` subscriber pattern in `grpc_service_npc_ticks.go` (or a new `grpc_service_item_ticks.go`):

```go
// On each calendar tick:
if dt.Hour == 0 {
    s.tickItemRecharge("daily")
}
if dt.Hour.Period() == PeriodMidnight && previousPeriod != PeriodMidnight {
    s.tickItemRecharge("midnight")
}
if dt.Hour.Period() == PeriodDawn && previousPeriod != PeriodDawn {
    s.tickItemRecharge("dawn")
}
```

`tickItemRecharge` iterates all online sessions, calls `TickRecharge` on each player's equipped item instances, persists updated charge state, and notifies the player if any item recharged.

- REQ-ACT-19: Period-based triggers (`midnight`, `dawn`) MUST fire only once per period transition, not once per hour within the period.

### Rest trigger

`GameServiceServer` exposes `RechargeOnRest(uid string)` — calls `TickRecharge` with `"rest"` for that player's instances. Stubbed now (no-op body with comment); called by the Resting feature when implemented.

- REQ-ACT-20: The rest recharge stub MUST exist and be callable at the Resting feature boundary without changes to this feature's code.

---

## 6. Architecture Summary

| Layer | File | Responsibility |
|-------|------|---------------|
| Pure logic | `internal/game/inventory/activate.go` | `HandleActivate`, `TickRecharge`, `ActivateSession` interface |
| Pure tests | `internal/game/inventory/activate_test.go` | Property-based + unit tests |
| Proto | `api/proto/game/v1/game.proto` | `ActivateItemRequest` message |
| Handler | `internal/gameserver/grpc_service_activate.go` | `handleActivate`, `tickItemRecharge`, `RechargeOnRest` |
| Handler tests | `internal/gameserver/grpc_service_activate_test.go` | TDD handler tests |
| Migration | `migrations/037_item_instance_charges.up.sql` | `charges_remaining`, `expended` columns |
| Feature doc | `docs/features/actions.md` | Mark Activate Item complete |

---

## 7. Testing

- Property-based tests (rapid) on `HandleActivate`: charge decrement invariants, AP deduction, depletion behavior, script vs. effect routing.
- Property-based tests on `TickRecharge`: charge never exceeds max, expended cleared correctly, trigger matching.
- Handler tests: full round-trip with mock session, proto dispatch, DB persistence.

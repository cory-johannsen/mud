# Activate Item Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `activate <item>` command that triggers effects (consumable-style or Lua script) on equipped charged-gear items with configurable recharge triggers.

**Architecture:** Pure logic lives in `internal/game/inventory/activate.go` (no DB I/O). The `handleActivate` handler in `grpc_service_activate.go` applies effects, persists charge state, and handles item destruction. Recharge wiring uses the existing `GameCalendar` subscriber pattern in a new `grpc_service_item_ticks.go`. Charge state is stored on `ItemInstance` in `character_inventory_instances` (following the same pattern as durability, modifier, etc.).

**Tech Stack:** Go 1.26, `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify`, `google.golang.org/protobuf/proto` for event push, protobuf/protoc (`make proto`), existing `GameCalendar` subscriber pattern.

**Spec:** `docs/superpowers/specs/2026-03-23-activate-item-design.md`

**Key codebase facts (read before implementing):**
- `Registry.Item(id string) (*ItemDef, bool)` — not `Get()`
- `Registry.RegisterItem(d *ItemDef) error` — not `Register()`
- `s.invRegistry` — not `s.itemReg`
- `s.charSaver` (type `CharacterSaver`) — not `s.charRepo`
- `s.dice` (type `*dice.Roller`) — not `s.rng`
- `s.calendar.CurrentDateTime()` — not `.Now()`
- Player-in-combat check: `sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)`
- Get player AP: `s.combatH.RemainingAP(uid)`
- Spend AP: `s.combatH.SpendAP(uid, cost) error`
- Push event to player: marshal proto then `sess.Entity.Push(data)`
- `s.sessions.AllPlayers() []*session.PlayerSession` exists
- `ItemDef.Validate()` requires `MaxStack >= 1` — always set `MaxStack: 1` in test defs

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/game/inventory/item.go` | Modify | Add `ActivationCost`, `Charges`, `OnDeplete`, `ActivationScript`, `ActivationEffect`, `Recharge` fields to `ItemDef`; add `RechargeEntry` type; extend `Validate()` |
| `internal/game/inventory/item_test.go` | Modify | Tests for new `Validate()` rules |
| `internal/game/inventory/backpack.go` | Modify | Add `ChargesRemaining`, `Expended` fields to `ItemInstance` |
| `internal/game/inventory/activate.go` | Create | `ActivateSession` interface, `ActivateResult`, `HandleActivate`, `TickRecharge` |
| `internal/game/inventory/activate_test.go` | Create | Property-based + unit tests |
| `internal/game/session/manager.go` | Modify | Add `EquippedInstances() []*inventory.ItemInstance` method to `PlayerSession` |
| `api/proto/game/v1/game.proto` | Modify | Add `ActivateItemRequest` message; add field `113` to `ClientMessage` oneof |
| `internal/gameserver/gamev1/` | Regenerate | Run `make proto` |
| `internal/gameserver/grpc_service.go` | Modify | Add `CharacterSaver` charge methods to interface; add `lastTimePeriod` field to struct; dispatch case; call `StartItemTickHook` |
| `internal/gameserver/grpc_service_activate.go` | Create | `handleActivate`, `RechargeOnRest`, `persistChargeState`, `removeEquippedItem` |
| `internal/gameserver/grpc_service_activate_test.go` | Create | TDD handler tests |
| `internal/gameserver/grpc_service_item_ticks.go` | Create | `StartItemTickHook`, `tickItemRecharge` |
| `internal/storage/postgres/character.go` | Modify | Add `SaveInstanceCharges` method |
| `migrations/037_item_instance_charges.up.sql` | Create | Add `charges_remaining`, `expended` to `character_inventory_instances` |
| `migrations/037_item_instance_charges.down.sql` | Create | Drop those columns |
| `docs/features/actions.md` | Modify | Mark Activate Item complete |

---

## Task 1: ItemDef activation fields + validation

**Files:**
- Modify: `internal/game/inventory/item.go`
- Modify: `internal/game/inventory/item_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/inventory/item_test.go`:

```go
func TestItemDef_Validate_ActivationFields(t *testing.T) {
    t.Run("activation_cost out of range", func(t *testing.T) {
        d := &ItemDef{ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1, ActivationCost: 4, Charges: 1}
        assert.Error(t, d.Validate())
    })
    t.Run("activation_cost valid max", func(t *testing.T) {
        d := &ItemDef{ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1, ActivationCost: 3, Charges: 1}
        assert.NoError(t, d.Validate())
    })
    t.Run("charges zero when cost nonzero", func(t *testing.T) {
        d := &ItemDef{ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 0}
        assert.Error(t, d.Validate())
    })
    t.Run("on_deplete invalid value", func(t *testing.T) {
        d := &ItemDef{ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 1, OnDeplete: "vanish"}
        assert.Error(t, d.Validate())
    })
    t.Run("on_deplete empty is valid (defaults to expend)", func(t *testing.T) {
        d := &ItemDef{ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 1}
        assert.NoError(t, d.Validate())
    })
    t.Run("script and effect both set", func(t *testing.T) {
        d := &ItemDef{
            ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1,
            ActivationCost: 1, Charges: 1,
            ActivationScript: "my_hook",
            ActivationEffect: &ConsumableEffect{},
        }
        assert.Error(t, d.Validate())
    })
    t.Run("recharge unknown trigger", func(t *testing.T) {
        d := &ItemDef{
            ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1,
            ActivationCost: 1, Charges: 1,
            Recharge: []RechargeEntry{{Trigger: "sunrise", Amount: 1}},
        }
        assert.Error(t, d.Validate())
    })
    t.Run("recharge amount zero", func(t *testing.T) {
        d := &ItemDef{
            ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1,
            ActivationCost: 1, Charges: 1,
            Recharge: []RechargeEntry{{Trigger: "dawn", Amount: 0}},
        }
        assert.Error(t, d.Validate())
    })
    t.Run("recharge valid multi-trigger", func(t *testing.T) {
        d := &ItemDef{
            ID: "x", Name: "x", Kind: KindConsumable, MaxStack: 1,
            ActivationCost: 2, Charges: 3,
            Recharge: []RechargeEntry{
                {Trigger: "dawn", Amount: 1},
                {Trigger: "rest", Amount: 2},
            },
        }
        assert.NoError(t, d.Validate())
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run TestItemDef_Validate_ActivationFields -v
```

Expected: compile error (types don't exist yet).

- [ ] **Step 3: Add `RechargeEntry` type and new fields to `ItemDef` in `item.go`**

After the closing `}` of `ItemDef` struct, add:

```go
// RechargeEntry defines one recharge trigger for an activatable item.
type RechargeEntry struct {
    // Trigger is when this recharge fires.
    // Valid values: "daily", "midnight", "dawn", "rest".
    Trigger string `yaml:"trigger"`
    // Amount is the number of charges restored. Must be > 0.
    Amount int `yaml:"amount"`
}
```

Add these fields to `ItemDef` after the `SubstanceID` field:

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

- [ ] **Step 4: Extend `ItemDef.Validate()` in `item.go`**

Inside the existing `Validate()` method (after existing validations, before the final `errors.Join`), add:

```go
// REQ-ACT-6: ActivationCost must be in [0, 3].
if d.ActivationCost < 0 || d.ActivationCost > 3 {
    errs = append(errs, fmt.Errorf("activation_cost %d is out of range [0, 3]", d.ActivationCost))
}
// REQ-ACT-7: Charges must be > 0 when ActivationCost > 0.
if d.ActivationCost > 0 && d.Charges <= 0 {
    errs = append(errs, fmt.Errorf("charges must be > 0 when activation_cost > 0 (got %d)", d.Charges))
}
// REQ-ACT-8: OnDeplete must be "", "destroy", or "expend".
if d.OnDeplete != "" && d.OnDeplete != "destroy" && d.OnDeplete != "expend" {
    errs = append(errs, fmt.Errorf("on_deplete %q is invalid; must be \"destroy\" or \"expend\"", d.OnDeplete))
}
// REQ-ACT-9: ActivationScript and ActivationEffect are mutually exclusive.
if d.ActivationScript != "" && d.ActivationEffect != nil {
    errs = append(errs, fmt.Errorf("activation_script and activation_effect are mutually exclusive"))
}
// REQ-ACT-10/11: Validate each RechargeEntry.
validTriggers := map[string]bool{"daily": true, "midnight": true, "dawn": true, "rest": true}
for i, re := range d.Recharge {
    if !validTriggers[re.Trigger] {
        errs = append(errs, fmt.Errorf("recharge[%d].trigger %q is invalid; must be daily|midnight|dawn|rest", i, re.Trigger))
    }
    if re.Amount <= 0 {
        errs = append(errs, fmt.Errorf("recharge[%d].amount must be > 0 (got %d)", i, re.Amount))
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run TestItemDef_Validate_ActivationFields -v
```

Expected: all pass.

- [ ] **Step 6: Run full inventory test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/game/inventory/item.go internal/game/inventory/item_test.go
git commit -m "feat: add ItemDef activation fields and validation (REQ-ACT-6 through REQ-ACT-12)"
```

---

## Task 2: ItemInstance charge fields

**Files:**
- Modify: `internal/game/inventory/backpack.go`
- Modify: `internal/game/inventory/backpack_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/game/inventory/backpack_test.go`:

```go
func TestItemInstance_ChargeFields_DefaultValues(t *testing.T) {
    // Reference new fields by name to force compile error before they exist.
    inst := ItemInstance{
        InstanceID:       "i1",
        ItemDefID:        "x",
        ChargesRemaining: -1,  // uninitialized sentinel
        Expended:         false,
    }
    assert.Equal(t, -1, inst.ChargesRemaining)
    assert.False(t, inst.Expended)
}
```

- [ ] **Step 2: Run test to verify it fails (compile error)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run TestItemInstance_ChargeFields -v 2>&1 | tail -10
```

Expected: compile error — `ChargesRemaining` and `Expended` are unknown fields.

- [ ] **Step 3: Add `ChargesRemaining` and `Expended` to `ItemInstance` in `backpack.go`**

After `CurseRevealed bool`:

```go
// ChargesRemaining is the number of activations remaining.
// -1 = uninitialized sentinel; initialized to ItemDef.Charges on first activation (REQ-ACT-13).
ChargesRemaining int
// Expended is true when ChargesRemaining == 0 and the item uses expend semantics.
Expended bool
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run TestItemInstance_ChargeFields -v
```

Expected: pass.

- [ ] **Step 5: Run full inventory test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/inventory/backpack.go internal/game/inventory/backpack_test.go
git commit -m "feat: add ChargesRemaining and Expended fields to ItemInstance"
```

---

## Task 3: `HandleActivate` and `TickRecharge` pure functions

**Files:**
- Create: `internal/game/inventory/activate.go`
- Create: `internal/game/inventory/activate_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/game/inventory/activate_test.go`:

```go
package inventory_test

import (
    "testing"
    "time"

    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)

// fakeSession implements ActivateSession for testing.
type fakeSession struct {
    team      string
    instances []*inventory.ItemInstance
}

func (f *fakeSession) GetTeam() string                                          { return f.team }
func (f *fakeSession) GetStatModifier(stat string) int                          { return 0 }
func (f *fakeSession) ApplyHeal(amount int)                                     {}
func (f *fakeSession) ApplyCondition(id string, dur time.Duration)              {}
func (f *fakeSession) RemoveCondition(id string)                                {}
func (f *fakeSession) ApplyDisease(id string, sev int)                          {}
func (f *fakeSession) ApplyToxin(id string, sev int)                            {}
func (f *fakeSession) EquippedInstances() []*inventory.ItemInstance             { return f.instances }

func mustRegisterItem(t *testing.T, reg *inventory.Registry, def *inventory.ItemDef) {
    t.Helper()
    require.NoError(t, reg.RegisterItem(def))
}

func makeActivatableItem(t *testing.T) (*inventory.Registry, *inventory.ItemDef, *inventory.ItemInstance) {
    t.Helper()
    def := &inventory.ItemDef{
        ID: "stim_rod", Name: "Stim Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 3,
        ActivationEffect: &inventory.ConsumableEffect{Heal: "2d6"},
    }
    reg := inventory.NewRegistry()
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{
        InstanceID: "inst-1", ItemDefID: "stim_rod",
        ChargesRemaining: -1,
    }
    return reg, def, inst
}

func TestHandleActivate_SentinelInitialization(t *testing.T) {
    reg, _, inst := makeActivatableItem(t)
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
    assert.Empty(t, errMsg)
    assert.Equal(t, 2, inst.ChargesRemaining) // initialized to 3, decremented to 2
    assert.Equal(t, 1, result.AP)
    assert.NotNil(t, result.ActivationEffect)
}

func TestHandleActivate_DecrementCharge(t *testing.T) {
    reg, _, inst := makeActivatableItem(t)
    inst.ChargesRemaining = 2
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    _, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
    assert.Empty(t, errMsg)
    assert.Equal(t, 1, inst.ChargesRemaining)
    assert.False(t, inst.Expended)
}

func TestHandleActivate_ExpendOnDeplete(t *testing.T) {
    reg, def, inst := makeActivatableItem(t)
    def.OnDeplete = "expend"
    inst.ChargesRemaining = 1
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
    assert.Empty(t, errMsg)
    assert.Equal(t, 0, inst.ChargesRemaining)
    assert.True(t, inst.Expended)
    assert.False(t, result.Destroyed)
}

func TestHandleActivate_DestroyOnDeplete(t *testing.T) {
    reg, def, inst := makeActivatableItem(t)
    def.OnDeplete = "destroy"
    inst.ChargesRemaining = 1
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    result, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
    assert.Empty(t, errMsg)
    assert.True(t, result.Destroyed)
}

func TestHandleActivate_ExpendedItemBlocked(t *testing.T) {
    reg, _, inst := makeActivatableItem(t)
    inst.ChargesRemaining = 0
    inst.Expended = true
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    _, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", false, 0)
    assert.NotEmpty(t, errMsg)
}

func TestHandleActivate_InCombatInsufficientAP(t *testing.T) {
    reg, def, inst := makeActivatableItem(t)
    def.ActivationCost = 2
    inst.ChargesRemaining = 3
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    _, errMsg := inventory.HandleActivate(sess, reg, "stim_rod", true, 1) // only 1 AP, needs 2
    assert.NotEmpty(t, errMsg)
    assert.Equal(t, 3, inst.ChargesRemaining) // unchanged
}

// REQ-ACT-5: out-of-combat, AP cost is informational only — activation MUST succeed regardless of AP value.
func TestHandleActivate_OutOfCombat_APNotRequired(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "heavy_ring", Name: "Heavy Ring", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 3, Charges: 2,
        ActivationEffect: &inventory.ConsumableEffect{Heal: "1d4"},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i5", ItemDefID: "heavy_ring", ChargesRemaining: 2}
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    // currentAP=0, inCombat=false: must succeed even though cost=3 > ap=0.
    _, errMsg := inventory.HandleActivate(sess, reg, "heavy_ring", false, 0)
    assert.Empty(t, errMsg)
    assert.Equal(t, 1, inst.ChargesRemaining)
}

func TestHandleActivate_NotActivatable(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{ID: "junk", Name: "Junk", Kind: inventory.KindJunk, MaxStack: 1}
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i1", ItemDefID: "junk"}
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    _, errMsg := inventory.HandleActivate(sess, reg, "junk", false, 0)
    assert.NotEmpty(t, errMsg)
}

func TestHandleActivate_ScriptRouting(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "arcane_rod", Name: "Arcane Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 2,
        ActivationScript: "arcane_rod_activate",
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i2", ItemDefID: "arcane_rod", ChargesRemaining: 2}
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    result, errMsg := inventory.HandleActivate(sess, reg, "arcane_rod", false, 0)
    assert.Empty(t, errMsg)
    assert.Equal(t, "arcane_rod_activate", result.Script)
    assert.Nil(t, result.ActivationEffect)
}

func TestHandleActivate_RechargeItemExpendNotDestroy(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "solar_ring", Name: "Solar Ring", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 1, OnDeplete: "destroy", // OnDeplete ignored when Recharge set
        Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i3", ItemDefID: "solar_ring", ChargesRemaining: 1}
    sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
    result, errMsg := inventory.HandleActivate(sess, reg, "solar_ring", false, 0)
    assert.Empty(t, errMsg)
    assert.False(t, result.Destroyed) // expend semantics, not destroy
    assert.True(t, inst.Expended)
}

func TestHandleActivate_Property_ChargesNeverGoNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        initial := rapid.IntRange(1, 10).Draw(rt, "initial")
        reg := inventory.NewRegistry()
        def := &inventory.ItemDef{
            ID: "item", Name: "Item", Kind: inventory.KindConsumable, MaxStack: 1,
            ActivationCost: 1, Charges: initial,
        }
        require.NoError(t, reg.RegisterItem(def))
        inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "item", ChargesRemaining: initial}
        sess := &fakeSession{instances: []*inventory.ItemInstance{inst}}
        for i := 0; i < initial+2; i++ {
            inventory.HandleActivate(sess, reg, "item", false, 0)
        }
        assert.GreaterOrEqual(rt, inst.ChargesRemaining, 0)
    })
}

func TestTickRecharge_RestoresCharges(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 3,
        Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 1}
    modified := inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "dawn")
    assert.Len(t, modified, 1)
    assert.Equal(t, 2, inst.ChargesRemaining)
}

func TestTickRecharge_CapsAtMaxCharges(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 3,
        Recharge: []inventory.RechargeEntry{{Trigger: "rest", Amount: 5}},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 2}
    inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "rest")
    assert.Equal(t, 3, inst.ChargesRemaining) // capped at max
}

func TestTickRecharge_ClearsExpended(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 3,
        Recharge: []inventory.RechargeEntry{{Trigger: "midnight", Amount: 1}},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 0, Expended: true}
    inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "midnight")
    assert.Equal(t, 1, inst.ChargesRemaining)
    assert.False(t, inst.Expended)
}

func TestTickRecharge_WrongTriggerNoOp(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{
        ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
        ActivationCost: 1, Charges: 3,
        Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 1}},
    }
    mustRegisterItem(t, reg, def)
    inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: 1}
    modified := inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "midnight")
    assert.Empty(t, modified)
    assert.Equal(t, 1, inst.ChargesRemaining)
}

func TestTickRecharge_Property_NeverExceedsMaxCharges(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        maxCharges := rapid.IntRange(1, 10).Draw(rt, "max")
        current := rapid.IntRange(0, maxCharges).Draw(rt, "current")
        amount := rapid.IntRange(1, 5).Draw(rt, "amount")
        reg := inventory.NewRegistry()
        def := &inventory.ItemDef{
            ID: "rod", Name: "Rod", Kind: inventory.KindConsumable, MaxStack: 1,
            ActivationCost: 1, Charges: maxCharges,
            Recharge: []inventory.RechargeEntry{{Trigger: "daily", Amount: amount}},
        }
        require.NoError(t, reg.RegisterItem(def))
        inst := &inventory.ItemInstance{InstanceID: "i", ItemDefID: "rod", ChargesRemaining: current}
        inventory.TickRecharge([]*inventory.ItemInstance{inst}, reg, "daily")
        assert.LessOrEqual(rt, inst.ChargesRemaining, maxCharges)
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run "TestHandleActivate|TestTickRecharge" -v 2>&1 | tail -20
```

Expected: compile error (activate.go doesn't exist).

- [ ] **Step 3: Create `internal/game/inventory/activate.go`**

```go
package inventory

import "strings"

// ActivateSession provides the equipped item view needed by HandleActivate
// and satisfies ConsumableTarget so the caller can pass it to ApplyConsumable.
//
// Precondition: all methods return valid, non-nil values.
type ActivateSession interface {
    ConsumableTarget // GetTeam, GetStatModifier, ApplyHeal, ApplyCondition, RemoveCondition, ApplyDisease, ApplyToxin

    // EquippedInstances returns all ItemInstances currently equipped.
    // The implementation MUST resolve instances from:
    //   1. WeaponPreset.MainHand.InstanceID → backpack lookup
    //   2. WeaponPreset.OffHand.InstanceID → backpack lookup
    //   3. Equipment.Armor[slot].InstanceID → backpack lookup (all non-nil slots)
    //   4. Equipment.Accessories[slot].InstanceID → backpack lookup (all non-nil slots)
    // Items whose InstanceID is not found in the backpack are silently skipped.
    EquippedInstances() []*ItemInstance
}

// ActivateResult is the outcome of a successful HandleActivate call.
type ActivateResult struct {
    AP               int               // AP cost consumed (equals ItemDef.ActivationCost)
    ItemDefID        string            // ID of the activated item's def
    ActivationEffect *ConsumableEffect // non-nil if consumable-style effect should be applied by caller
    Script           string            // non-empty if Lua hook should be invoked by caller
    Destroyed        bool              // true if item was destroyed on depletion
}

// HandleActivate resolves query across all equipped item instances, validates charge
// and AP state, decrements ChargesRemaining, and marks the item Expended or Destroyed.
// Does NOT apply effects or persist state — the caller is responsible for both.
//
// Precondition: sess, reg non-nil; query non-empty; currentAP >= 0.
// Postcondition: On success, ChargesRemaining is decremented in the matched
// ItemInstance and ActivateResult is returned with errMsg == "".
// On failure, ItemInstance is unchanged and errMsg is non-empty.
func HandleActivate(sess ActivateSession, reg *Registry, query string, inCombat bool, currentAP int) (result ActivateResult, errMsg string) {
    // Resolve the equipped instance matching query.
    var matched *ItemInstance
    for _, inst := range sess.EquippedInstances() {
        def, ok := reg.Item(inst.ItemDefID)
        if !ok {
            continue
        }
        if strings.EqualFold(def.ID, query) || strings.EqualFold(def.Name, query) {
            matched = inst
            break
        }
    }
    if matched == nil {
        return result, "No activatable item matching \"" + query + "\" found in your equipped gear."
    }

    def, ok := reg.Item(matched.ItemDefID)
    if !ok || def.ActivationCost == 0 {
        return result, "That item cannot be activated."
    }

    // REQ-ACT-13: initialize sentinel.
    if matched.ChargesRemaining == -1 {
        matched.ChargesRemaining = def.Charges
    }

    // REQ-ACT-3: block expended items.
    if matched.Expended {
        return result, "That item is expended and has no charges remaining."
    }
    if matched.ChargesRemaining == 0 {
        return result, "That item has no charges remaining."
    }

    // REQ-ACT-4: in combat, check AP.
    if inCombat && currentAP < def.ActivationCost {
        return result, "Not enough AP to activate that item."
    }

    // Decrement charge.
    matched.ChargesRemaining--

    // Determine depletion behavior.
    var destroyed bool
    if matched.ChargesRemaining == 0 {
        // REQ-ACT-12: when recharge entries exist, always use expend semantics.
        useExpend := len(def.Recharge) > 0 || def.OnDeplete != "destroy"
        if useExpend {
            matched.Expended = true
        } else {
            destroyed = true
        }
    }

    return ActivateResult{
        AP:               def.ActivationCost,
        ItemDefID:        def.ID,
        ActivationEffect: def.ActivationEffect,
        Script:           def.ActivationScript,
        Destroyed:        destroyed,
    }, ""
}

// TickRecharge restores charges to all item instances whose ItemDef has a recharge
// entry matching trigger. Charges are capped at ItemDef.Charges. Expended is
// cleared when charges become > 0 after recharge.
// Returns the subset of instances that were modified.
//
// Precondition: instances and reg non-nil; trigger is a known RechargeEntry.Trigger value.
// Postcondition: ChargesRemaining and Expended are updated in-place on modified instances.
func TickRecharge(instances []*ItemInstance, reg *Registry, trigger string) []*ItemInstance {
    var modified []*ItemInstance
    for _, inst := range instances {
        def, ok := reg.Item(inst.ItemDefID)
        if !ok {
            continue
        }
        for _, re := range def.Recharge {
            if re.Trigger != trigger {
                continue
            }
            before := inst.ChargesRemaining
            wasExpended := inst.Expended
            inst.ChargesRemaining += re.Amount
            if inst.ChargesRemaining > def.Charges {
                inst.ChargesRemaining = def.Charges
            }
            if inst.ChargesRemaining > 0 {
                inst.Expended = false
            }
            if inst.ChargesRemaining != before || inst.Expended != wasExpended {
                modified = append(modified, inst)
            }
            break // first matching trigger wins per item
        }
    }
    return modified
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... -run "TestHandleActivate|TestTickRecharge" -v
```

Expected: all pass.

- [ ] **Step 5: Run full inventory test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/inventory/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/inventory/activate.go internal/game/inventory/activate_test.go
git commit -m "feat: add HandleActivate and TickRecharge (REQ-ACT-1 through REQ-ACT-16)"
```

---

## Task 4: `PlayerSession.EquippedInstances()`

**Files:**
- Modify: `internal/game/session/manager.go`

This method makes `PlayerSession` satisfy `ActivateSession.EquippedInstances()`. It resolves instance IDs from the **active weapon preset** and all armor/accessory slots back to `ItemInstance` pointers in the backpack. Only the active preset is used (REQ-ACT-1: "items currently equipped").

- [ ] **Step 1: Write failing tests**

Locate the session test file:
```bash
ls /home/cjohannsen/src/mud/internal/game/session/*_test.go
```

Add tests to the appropriate session test file (or create `internal/game/session/equipped_instances_test.go`):

```go
func TestEquippedInstances_Empty(t *testing.T) {
    sess := &PlayerSession{
        Backpack:   inventory.NewBackpack(10, 100),
        LoadoutSet: inventory.NewLoadoutSet(),
        Equipment:  inventory.NewEquipment(),
    }
    insts := sess.EquippedInstances()
    assert.NotNil(t, insts)
    assert.Empty(t, insts)
}

func TestEquippedInstances_ActivePresetOnly(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.ItemDef{ID: "sword", Name: "Sword", Kind: inventory.KindWeapon, MaxStack: 1, Weight: 1.0, MinLevel: 1}
    require.NoError(t, reg.RegisterItem(def))

    bp := inventory.NewBackpack(10, 100)
    inst, err := bp.Add("sword", 1, reg)
    require.NoError(t, err)
    inst.InstanceID = "sword-inst-1"

    ls := inventory.NewLoadoutSet()
    // Active preset 0 has main hand; preset 1 has nothing.
    ls.Presets[0].MainHand = &inventory.EquippedWeapon{Def: def, InstanceID: "sword-inst-1"}

    sess := &PlayerSession{
        Backpack:   bp,
        LoadoutSet: ls,
        Equipment:  inventory.NewEquipment(),
    }
    insts := sess.EquippedInstances()
    require.Len(t, insts, 1)
    assert.Equal(t, "sword-inst-1", insts[0].InstanceID)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -run "TestEquippedInstances" -v 2>&1 | tail -20
```

Expected: compile error (method doesn't exist yet).

- [ ] **Step 3: Add `EquippedInstances()` method**

Add to `internal/game/session/manager.go`:

```go
// EquippedInstances returns all ItemInstances currently equipped across the active weapon
// preset and all armor/accessory slots. Resolves InstanceIDs to backpack pointers.
// Unresolvable InstanceIDs are silently skipped.
//
// Precondition: sess.Backpack, sess.LoadoutSet, sess.Equipment may be nil (handled gracefully).
// Postcondition: Returns a non-nil slice (may be empty).
func (sess *PlayerSession) EquippedInstances() []*inventory.ItemInstance {
    var result []*inventory.ItemInstance

    // Active weapon preset slots only (REQ-ACT-1).
    if sess.LoadoutSet != nil && int(sess.LoadoutSet.Active) < len(sess.LoadoutSet.Presets) {
        active := sess.LoadoutSet.Presets[sess.LoadoutSet.Active]
        if active != nil {
            for _, ew := range []*inventory.EquippedWeapon{active.MainHand, active.OffHand} {
                if ew == nil || ew.InstanceID == "" {
                    continue
                }
                if inst := sess.Backpack.GetByInstanceID(ew.InstanceID); inst != nil {
                    result = append(result, inst)
                }
            }
        }
    }

    // Armor and accessory slots.
    if sess.Equipment != nil {
        for _, si := range sess.Equipment.Armor {
            if si == nil || si.InstanceID == "" {
                continue
            }
            if inst := sess.Backpack.GetByInstanceID(si.InstanceID); inst != nil {
                result = append(result, inst)
            }
        }
        for _, si := range sess.Equipment.Accessories {
            if si == nil || si.InstanceID == "" {
                continue
            }
            if inst := sess.Backpack.GetByInstanceID(si.InstanceID); inst != nil {
                result = append(result, inst)
            }
        }
    }

    return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -run "TestEquippedInstances" -v
```

Expected: all pass.

- [ ] **Step 5: Run full session test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/session/manager.go internal/game/session/equipped_instances_test.go
git commit -m "feat: add PlayerSession.EquippedInstances() for ActivateSession interface"
```

Note: if the tests were added to an existing test file rather than a new one, replace `equipped_instances_test.go` with the actual filename (e.g., `manager_test.go`).

---

## Task 5: DB migration + persistence methods

**Files:**
- Create: `migrations/037_item_instance_charges.up.sql`
- Create: `migrations/037_item_instance_charges.down.sql`
- Modify: `internal/storage/postgres/character.go`
- Modify: `internal/gameserver/grpc_service.go` (CharacterSaver interface)

- [ ] **Step 1: Create migrations**

Create `migrations/037_item_instance_charges.up.sql`:

```sql
-- Migration 037: Add charge state to item instances (REQ-ACT-14).
-- Charges belong to the item instance, not the equipped slot.

ALTER TABLE character_inventory_instances
    ADD COLUMN IF NOT EXISTS charges_remaining INTEGER NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS expended          BOOLEAN NOT NULL DEFAULT FALSE;
```

Create `migrations/037_item_instance_charges.down.sql`:

```sql
ALTER TABLE character_inventory_instances
    DROP COLUMN IF EXISTS charges_remaining,
    DROP COLUMN IF EXISTS expended;
```

- [ ] **Step 2: Add `SaveInstanceCharges` and `LoadInstanceCharges` to `CharacterRepository`**

`character_inventory_instances` uses `instance_id TEXT PRIMARY KEY` (from migration 035). `ItemInstance.InstanceID` is a UUID string matching that column.

Add to `internal/storage/postgres/character.go`:

```go
// InstanceChargeState holds persisted charge state for one item instance.
type InstanceChargeState struct {
    ChargesRemaining int
    Expended         bool
}

// SaveInstanceCharges upserts charges_remaining and expended for a backpack item instance.
// Requires itemDefID to satisfy the NOT NULL constraint on first insert.
//
// Precondition: characterID > 0; instanceID and itemDefID non-empty.
// Postcondition: DB row upserted; returns nil on success.
func (r *CharacterRepository) SaveInstanceCharges(ctx context.Context, characterID int64, instanceID, itemDefID string, charges int, expended bool) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO character_inventory_instances (instance_id, character_id, item_def_id, charges_remaining, expended)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (instance_id)
        DO UPDATE SET charges_remaining = EXCLUDED.charges_remaining, expended = EXCLUDED.expended`,
        instanceID, characterID, itemDefID, charges, expended,
    )
    return err
}

// LoadInstanceCharges returns a map of instanceID → InstanceChargeState for all instances
// belonging to characterID that have non-sentinel charge state (charges_remaining != -1).
//
// Precondition: characterID > 0.
// Postcondition: Returns nil map on error; empty map if no rows exist.
func (r *CharacterRepository) LoadInstanceCharges(ctx context.Context, characterID int64) (map[string]InstanceChargeState, error) {
    rows, err := r.db.Query(ctx, `
        SELECT instance_id, charges_remaining, expended
        FROM character_inventory_instances
        WHERE character_id = $1 AND charges_remaining != -1`,
        characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("loading instance charges for character %d: %w", characterID, err)
    }
    defer rows.Close()
    result := make(map[string]InstanceChargeState)
    for rows.Next() {
        var id string
        var s InstanceChargeState
        if err := rows.Scan(&id, &s.ChargesRemaining, &s.Expended); err != nil {
            return nil, fmt.Errorf("scanning instance charges: %w", err)
        }
        result[id] = s
    }
    return result, rows.Err()
}
```

After loading the player's backpack, call `LoadInstanceCharges` and apply to matching instances:

```go
// In the character load path — find where Backpack is populated:
//   grep -n "Backpack\|LoadCharacter\|loadCharacter" /home/cjohannsen/src/mud/internal/storage/postgres/character.go | head -20
// Wire LoadInstanceCharges immediately after backpack items are loaded:
if chargeMap, err := r.LoadInstanceCharges(ctx, characterID); err == nil {
    for i := range backpack.Items() {
        inst := &backpack.Items()[i]
        if s, ok := chargeMap[inst.InstanceID]; ok {
            inst.ChargesRemaining = s.ChargesRemaining
            inst.Expended = s.Expended
        }
    }
}
```

Note: `backpack.Items()` returns `[]ItemInstance`. If the method doesn't exist yet, use `Backpack.GetAll()` or the equivalent accessor. Verify by reading the `Backpack` type in `backpack.go` before implementing.

- [ ] **Step 3: Add `SaveInstanceCharges` to the `CharacterSaver` interface in `grpc_service.go`**

In `internal/gameserver/grpc_service.go`, find the `CharacterSaver interface` block and add:

```go
SaveInstanceCharges(ctx context.Context, characterID int64, instanceID, itemDefID string, charges int, expended bool) error
```

- [ ] **Step 4: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Expected: no errors.

- [ ] **Step 5: Run postgres tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/storage/postgres/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add migrations/037_item_instance_charges.up.sql migrations/037_item_instance_charges.down.sql \
    internal/storage/postgres/character.go internal/gameserver/grpc_service.go
git commit -m "feat: migration 037 and SaveInstanceCharges (REQ-ACT-14)"
```

---

## Task 6: Proto message

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/`

- [ ] **Step 1: Add `ActivateItemRequest` message to proto file**

In `api/proto/game/v1/game.proto`, add the message definition near the end (after `TravelRequest`):

```proto
message ActivateItemRequest {
  string item_query = 1;
}
```

- [ ] **Step 2: Add field `113` to `ClientMessage` oneof**

In the `ClientMessage` oneof payload block, after `TravelRequest travel = 112;`:

```proto
ActivateItemRequest activate_item = 113;
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: `internal/gameserver/gamev1/game.pb.go` updated, no errors.

- [ ] **Step 4: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add ActivateItemRequest proto message at field 113"
```

---

## Task 7: Handler

**Files:**
- Create: `internal/gameserver/grpc_service_activate.go`
- Create: `internal/gameserver/grpc_service_activate_test.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/gameserver/grpc_service_activate_test.go`:

```go
package gameserver_test

import (
    "context"
    "testing"

    "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestHandleActivate_NotFound(t *testing.T) {
    s, cleanup := newTestServer(t)
    defer cleanup()
    uid := createTestCharacter(t, s)

    evt, err := s.HandleClientMessage(context.Background(), uid, &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_ActivateItem{
            ActivateItem: &gamev1.ActivateItemRequest{ItemQuery: "nonexistent_item"},
        },
    })
    require.NoError(t, err)
    assert.NotNil(t, evt)
    // Error event — check narrative contains "not found" or similar
    narrative := evt.GetNarrative()
    assert.NotEmpty(t, narrative)
}
```

Note: `newTestServer` and `createTestCharacter` are test helpers already used in other `_test.go` files in the `gameserver_test` package. Check `grpc_service_healer_test.go` for the exact pattern.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandleActivate_NotFound -v 2>&1 | tail -20
```

Expected: compile error (handler doesn't exist).

- [ ] **Step 3: Create `grpc_service_activate.go`**

```go
package gameserver

import (
    "context"
    "fmt"

    "google.golang.org/protobuf/proto"

    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleActivate processes the "activate <item>" command.
//
// Precondition: uid non-empty; req non-nil.
// Postcondition: on success, charge state is persisted and a narrative event is returned.
func (s *GameServiceServer) handleActivate(uid string, req *gamev1.ActivateItemRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }

    inCombat := sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)
    ap := 0
    if inCombat {
        ap = s.combatH.RemainingAP(uid)
    }

    result, errMsg := inventory.HandleActivate(sess, s.invRegistry, req.ItemQuery, inCombat, ap)
    if errMsg != "" {
        return errorEvent(errMsg), nil
    }

    // REQ-ACT-4: deduct AP in combat.
    if inCombat {
        if err := s.combatH.SpendAP(uid, result.AP); err != nil {
            return errorEvent(err.Error()), nil
        }
    }

    // Apply effects.
    if result.Script != "" {
        // Resolve zone for Lua hook (matches pattern used in skill-check handler).
        var zoneID string
        if s.worldMgr != nil {
            if room, ok := s.worldMgr.GetRoom(sess.RoomID); ok {
                zoneID = room.ZoneID
            }
        }
        if zoneID != "" {
            s.scriptMgr.CallHook(zoneID, result.Script) //nolint:errcheck
        }
    } else if result.ActivationEffect != nil {
        def, _ := s.invRegistry.Item(result.ItemDefID)
        team := ""
        if def != nil {
            team = def.Team
        }
        syntheticDef := &inventory.ItemDef{
            Effect: result.ActivationEffect,
            Team:   team,
        }
        inventory.ApplyConsumable(sess, syntheticDef, s.dice)
    }

    // REQ-ACT-17: persist charge state BEFORE removing the item from the slot,
    // because persistChargeState locates the instance via EquippedInstances().
    if err := s.persistChargeState(context.Background(), sess, result.ItemDefID); err != nil {
        s.logger.Warn("handleActivate: failed to persist charge state", "error", err)
    }

    // REQ-ACT-18: handle destruction (after persist).
    if result.Destroyed {
        s.removeEquippedItem(sess, result.ItemDefID)
    }

    def, _ := s.invRegistry.Item(result.ItemDefID)
    name := result.ItemDefID
    if def != nil {
        name = def.Name
    }
    if result.Destroyed {
        return messageEvent(fmt.Sprintf("You activate %s — it crumbles to dust.", name)), nil
    }
    return messageEvent(fmt.Sprintf("You activate %s.", name)), nil
}

// RechargeOnRest fires the "rest" recharge trigger for the given player's equipped instances.
// Called by the Resting feature when a player completes a rest (REQ-ACT-22).
//
// Precondition: uid non-empty.
// Postcondition: ChargesRemaining updated for matching items and persisted to DB.
func (s *GameServiceServer) RechargeOnRest(uid string) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return
    }
    s.runRechargeForSession(context.Background(), sess, "rest")
}

// persistChargeState writes ChargesRemaining and Expended for the ItemInstance matching itemDefID.
// Iterates EquippedInstances to find the matching instance, then saves by InstanceID.
//
// Precondition: sess non-nil; itemDefID non-empty.
// Postcondition: DB updated for the first matching instance found; returns nil on success.
func (s *GameServiceServer) persistChargeState(ctx context.Context, sess *session.PlayerSession, itemDefID string) error {
    for _, inst := range sess.EquippedInstances() {
        if inst.ItemDefID == itemDefID {
            return s.charSaver.SaveInstanceCharges(ctx, sess.CharacterID, inst.InstanceID, inst.ItemDefID, inst.ChargesRemaining, inst.Expended)
        }
    }
    return nil
}

// removeEquippedItem removes a destroyed item from equipped slots and persists.
// REQ-ACT-18/19: only the ACTIVE weapon preset slot is cleared; other presets are untouched.
func (s *GameServiceServer) removeEquippedItem(sess *session.PlayerSession, itemDefID string) {
    ctx := context.Background()
    // REQ-ACT-19: only clear the active preset slot.
    if sess.LoadoutSet != nil && int(sess.LoadoutSet.Active) < len(sess.LoadoutSet.Presets) {
        active := sess.LoadoutSet.Presets[sess.LoadoutSet.Active]
        if active != nil {
            if active.MainHand != nil && active.MainHand.Def != nil && active.MainHand.Def.ID == itemDefID {
                active.MainHand = nil
            }
            if active.OffHand != nil && active.OffHand.Def != nil && active.OffHand.Def.ID == itemDefID {
                active.OffHand = nil
            }
        }
    }
    if sess.Equipment != nil {
        for slot, si := range sess.Equipment.Armor {
            if si != nil && si.ItemDefID == itemDefID {
                sess.Equipment.Armor[slot] = nil
            }
        }
        for slot, si := range sess.Equipment.Accessories {
            if si != nil && si.ItemDefID == itemDefID {
                sess.Equipment.Accessories[slot] = nil
            }
        }
    }
    _ = s.charSaver.SaveWeaponPresets(ctx, sess.CharacterID, sess.LoadoutSet)
    _ = s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment)
}

// runRechargeForSession calls TickRecharge for a player and persists/notifies on changes.
func (s *GameServiceServer) runRechargeForSession(ctx context.Context, sess *session.PlayerSession, trigger string) {
    instances := sess.EquippedInstances()
    modified := inventory.TickRecharge(instances, s.invRegistry, trigger)
    for _, inst := range modified {
        if err := s.persistChargeState(ctx, sess, inst.ItemDefID); err != nil {
            s.logger.Warn("runRechargeForSession: failed to persist", "uid", sess.UID, "item", inst.ItemDefID, "error", err)
        }
        def, _ := s.invRegistry.Item(inst.ItemDefID)
        name := inst.ItemDefID
        if def != nil {
            name = def.Name
        }
        evt := messageEvent(name + " has recharged.")
        if data, err := proto.Marshal(evt); err == nil {
            _ = sess.Entity.Push(data)
        }
    }
}
```

- [ ] **Step 4: Add dispatch case to `grpc_service.go`**

In `internal/gameserver/grpc_service.go`, in the `switch p := msg.Payload.(type)` block, after the `*gamev1.ClientMessage_Travel` case:

```go
case *gamev1.ClientMessage_ActivateItem:
    return s.handleActivate(uid, p.ActivateItem)
```

- [ ] **Step 5: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Fix any compile errors before continuing.

- [ ] **Step 6: Run handler tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandleActivate -v
```

Expected: all pass.

- [ ] **Step 7: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service_activate.go internal/gameserver/grpc_service_activate_test.go internal/gameserver/grpc_service.go
git commit -m "feat: add handleActivate handler, RechargeOnRest, persistChargeState (REQ-ACT-17 through REQ-ACT-19)"
```

---

## Task 8: Recharge tick wiring

**Files:**
- Create: `internal/gameserver/grpc_service_item_ticks.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Add `lastTimePeriod` and `itemTickMu` fields to `GameServiceServer` in `grpc_service.go`**

Find the `GameServiceServer` struct definition. Add the fields after existing fields:

```go
// lastTimePeriod tracks the last observed time period for transition-based recharge triggers.
// Protected by itemTickMu; owned exclusively by the item-tick goroutine (REQ-ACT-21).
lastTimePeriod TimePeriod
// itemTickMu protects lastTimePeriod; owned exclusively by the item-tick goroutine.
itemTickMu sync.Mutex
```

Also add `"sync"` to the import block of `grpc_service.go` if not already present.

- [ ] **Step 2: Create `grpc_service_item_ticks.go`**

```go
package gameserver

import "context"

// StartItemTickHook subscribes to the calendar and drives item charge recharge triggers.
// Fires "daily" at Hour==0, "midnight" and "dawn" on period transitions.
//
// Precondition: MUST be called after GameServiceServer is fully initialized.
// Precondition: s.calendar MUST NOT be nil.
// Postcondition: returns a stop function; call it to unsubscribe and stop the goroutine.
func (s *GameServiceServer) StartItemTickHook() func() {
    if s.calendar == nil {
        return func() {}
    }
    ch := make(chan GameDateTime, 4)
    s.calendar.Subscribe(ch)
    stop := make(chan struct{})

    // REQ-ACT-21: initialize lastTimePeriod to avoid spurious triggers on first tick.
    s.itemTickMu.Lock()
    s.lastTimePeriod = s.calendar.CurrentDateTime().Hour.Period()
    s.itemTickMu.Unlock()

    go func() {
        for {
            select {
            case dt := <-ch:
                currentPeriod := dt.Hour.Period()

                // Determine which triggers fire before releasing the lock.
                s.itemTickMu.Lock()
                fireMidnight := currentPeriod == PeriodMidnight && currentPeriod != s.lastTimePeriod
                fireDawn := currentPeriod == PeriodDawn && currentPeriod != s.lastTimePeriod
                if currentPeriod != s.lastTimePeriod {
                    s.lastTimePeriod = currentPeriod
                }
                s.itemTickMu.Unlock()
                // tickItemRecharge performs DB I/O and proto pushes — must NOT be called under lock.
                if dt.Hour == 0 {
                    s.tickItemRecharge("daily")
                }
                if fireMidnight {
                    s.tickItemRecharge("midnight")
                }
                if fireDawn {
                    s.tickItemRecharge("dawn")
                }

            case <-stop:
                s.calendar.Unsubscribe(ch)
                return
            }
        }
    }()
    return func() { close(stop) }
}

// tickItemRecharge fires a recharge trigger for all online players' equipped items.
func (s *GameServiceServer) tickItemRecharge(trigger string) {
    ctx := context.Background()
    for _, sess := range s.sessions.AllPlayers() {
        s.runRechargeForSession(ctx, sess, trigger)
    }
}
```

- [ ] **Step 3: Call `StartItemTickHook` at server initialization**

Find where `StartNPCTickHook` is called:

```bash
grep -rn "StartNPCTickHook" /home/cjohannsen/src/mud/cmd/ /home/cjohannsen/src/mud/internal/ --include="*.go" | grep -v "_test"
```

Add `s.StartItemTickHook()` immediately after `s.StartNPCTickHook()` at that call site.

- [ ] **Step 4: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_item_ticks.go internal/gameserver/grpc_service.go
git commit -m "feat: add StartItemTickHook for daily/midnight/dawn recharge triggers (REQ-ACT-20 through REQ-ACT-22)"
```

---

## Task 9: Feature doc + final verification

**Files:**
- Modify: `docs/features/actions.md`

- [ ] **Step 1: Mark Activate Item complete in `actions.md`**

Change:
```markdown
- [ ] Activate Item command — implement `activate <item>` ...
```
To:
```markdown
- [x] Activate Item command — implement `activate <item>` ...
```

- [ ] **Step 2: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: all pass, zero failures.

- [ ] **Step 3: Commit and push**

```bash
git add docs/features/actions.md
git commit -m "docs: mark activate item command complete"
git push
```

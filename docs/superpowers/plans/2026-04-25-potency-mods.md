# Potency Mods (Runes) on Weapons and Armor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement PF2E "potency rune" mechanics under the cyberpunk-friendly name **mod** (chip / firmware patch / aftermarket bolt-on). Mods are item-typed bonuses on a weapon (attack + damage) or armor (AC) attached to a per-instance slot loadout. Type-stacking rule from #259 ensures only the highest item-typed bonus applies, so a `+2` mod on a `+1` base weapon yields effective `+2` (not `+3`). Player UX: telnet `mod install` / `mod remove` and a web `ModsPanel.tsx`. Three weapon-tier mods + three armor-tier mods + one non-potency exemplar (`extended_magazine`).

**Spec:** [docs/superpowers/specs/2026-04-25-potency-mods.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-potency-mods.md) (PR [#288](https://github.com/cory-johannsen/mud/pull/288))

**Architecture:** Three pieces. (1) Content + schema — `ItemKindMod` constant, `Mod` loader with `slot`/`bonus`/`bonuses`/`consumes_slots` fields. (2) Per-instance persistence — `ItemInstance.AffixedMods []string` mirroring the `AffixedMaterials` precedent. (3) Equip-time wiring — `BuildCombatantEffects` sums potency-mod `bonus` values from `AffixedMods`, takes `max(WeaponDef.Bonus, sumMods)` (per type-stacking rule), applies as a single item-typed `Bonus` with `SourceID = "weapon:<id>"`. Non-potency mod `bonuses` ride as separate `Bonus` entries with `SourceID = "mod:<modID>"`. Unequip removes via `EffectSet.RemoveBySource`. Telnet `mod install` / `mod remove` and web `ModsPanel.tsx`; merchants stock mods through the existing inventory pipeline.

**Tech Stack:** Go (`internal/game/inventory/`, `internal/game/combat/`, `internal/gameserver/`), protobuf, telnet, React/TypeScript (`cmd/webclient/ui/src/game/inventory/`).

**Prerequisite:** #245 typed-bonus pipeline merged. #259 (bonus types — armor AC migration + breakdown tooltip) is a soft dep — the breakdown surface is what makes the mod row visible.

**Note on spec PR**: Spec is on PR #288, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/inventory/item.go` (`ItemKindMod` constant; `ApplicableTo`, `Slot` fields on `ItemDef`) |
| Modify | `internal/game/inventory/item_test.go` |
| Create | `internal/game/inventory/mod.go` |
| Create | `internal/game/inventory/mod_test.go` |
| Modify | `internal/game/inventory/backpack.go` (`ItemInstance.AffixedMods`) |
| Modify | `internal/game/inventory/backpack_test.go` |
| Modify | `internal/game/combat/combatant_effects.go` (mod sum + max-rule + non-potency dispatch) |
| Modify | `internal/game/combat/combatant_effects_test.go` |
| Modify | `internal/gameserver/grpc_service.go` (`ModInstall`, `ModRemove` RPCs) |
| Create | `internal/gameserver/grpc_service_mods_test.go` |
| Modify | `api/proto/game/v1/game.proto` (`ModInstallRequest`, `ModRemoveRequest`, `ItemModView`) |
| Create | `internal/frontend/telnet/mod_handler.go` |
| Create | `internal/frontend/telnet/mod_handler_test.go` |
| Create | `cmd/webclient/ui/src/game/inventory/ModsPanel.tsx` |
| Create | `cmd/webclient/ui/src/game/inventory/ModsPanel.test.tsx` |
| Modify | `cmd/webclient/ui/src/game/inventory/InventoryItem.tsx` (slot badge per MOD-20) |
| Create | `migrations/NNN_item_instance_affixed_mods.up.sql`, `.down.sql` (only if items table needs the column — verify) |
| Create | `content/items/mods/potency_mod_1.yaml`, `potency_mod_2.yaml`, `potency_mod_3.yaml`, `armor_potency_mod_1.yaml`, `armor_potency_mod_2.yaml`, `armor_potency_mod_3.yaml`, `extended_magazine.yaml` |
| Modify | `docs/architecture/items.md` |

---

### Task 1: `ItemKindMod` + Mod schema + loader

**Files:**
- Modify: `internal/game/inventory/item.go`
- Modify: `internal/game/inventory/item_test.go`
- Create: `internal/game/inventory/mod.go`
- Create: `internal/game/inventory/mod_test.go`

- [ ] **Step 1: Failing tests** (MOD-1, MOD-2, MOD-3):

```go
func TestLoadMod_PotencyOne(t *testing.T) {
    item, err := inventory.LoadItem([]byte(`
id: potency_mod_1
kind: mod
display_name: Potency Mod I
slot: weapon
bonus: 1
consumes_slots: 1
`))
    require.NoError(t, err)
    require.Equal(t, inventory.ItemKindMod, item.Kind)
    require.Equal(t, "weapon", item.Mod.Slot)
    require.Equal(t, 1, item.Mod.Bonus)
}

func TestLoadMod_RejectsUnknownSlot(t *testing.T) {
    _, err := inventory.LoadItem([]byte(`
id: bogus_mod
kind: mod
slot: pancake
bonus: 1
`))
    require.Error(t, err)
}

func TestLoadMod_AcceptsBonusAndBonusesTogether(t *testing.T) {
    item, err := inventory.LoadItem([]byte(`
id: combo_mod
kind: mod
slot: weapon
bonus: 1
bonuses:
  - { stat: damage, value: 1, type: status }
`))
    require.NoError(t, err)
    require.Equal(t, 1, item.Mod.Bonus)
    require.Len(t, item.Mod.Bonuses, 1)
}

func TestLoadMod_RejectsZeroConsumesSlots(t *testing.T) {
    _, err := inventory.LoadItem(yamlWithConsumesSlots(0))
    require.Error(t, err)
}
```

- [ ] **Step 2: Implement** the schema:

```go
const ItemKindMod ItemKind = "mod"

type ItemDef struct {
    // ... existing ...
    Mod *ModDef // populated when Kind == ItemKindMod
}

type ModDef struct {
    Slot          string  // "weapon" | "armor"
    Bonus         int
    Bonuses       []effect.Bonus
    ConsumesSlots int
    InstallerDC   int // reserved
}

func (m *ModDef) Validate() error {
    if m.Slot != "weapon" && m.Slot != "armor" {
        return fmt.Errorf("mod.slot must be weapon or armor")
    }
    if m.ConsumesSlots < 1 {
        return fmt.Errorf("mod.consumes_slots must be >= 1")
    }
    return nil
}
```

- [ ] **Step 3:** Loader recognises `kind: mod` and unmarshals into `ItemDef.Mod`. Pure-tests for the schema.

---

### Task 2: Per-instance `AffixedMods` persistence

**Files:**
- Modify: `internal/game/inventory/backpack.go`
- Modify: `internal/game/inventory/backpack_test.go`
- Optional: `migrations/NNN_item_instance_affixed_mods.up.sql`, `.down.sql`

- [ ] **Step 1: Verify** whether the items table currently uses a JSON column for instance metadata (which already round-trips arbitrary fields) or a typed column. If JSON, no migration needed. If typed, author the migration to add `affixed_mods JSONB DEFAULT '[]'::jsonb`.

- [ ] **Step 2: Failing tests** (MOD-5, MOD-6, MOD-7):

```go
func TestItemInstance_AffixedModsRoundTripsThroughDB(t *testing.T) {
    s := newPGStore(t)
    inst := &inventory.ItemInstance{ID: "weapon-1", DefID: "pistol", AffixedMods: []string{"potency_mod_2", "extended_magazine"}}
    require.NoError(t, s.SaveInstance(inst))
    out, _ := s.LoadInstance("weapon-1")
    require.Equal(t, []string{"potency_mod_2", "extended_magazine"}, out.AffixedMods)
}

func TestItemInstance_AffixedModsSurviveTradeAndPickup(t *testing.T) {
    a, b := newCharacter(t, "a"), newCharacter(t, "b")
    weapon := giveWeapon(a, "pistol", withAffixedMods("potency_mod_2"))
    a.Trade(b, weapon)
    require.Equal(t, []string{"potency_mod_2"}, b.Inventory.GetByID("weapon-1").AffixedMods)
}

func TestItemInstance_DestroyedItemConsumesUnrecoverableMods(t *testing.T) {
    weapon := newWeapon(t, withMods("potency_mod_1"), modsRecoverable: false)
    weapon.Destroy()
    require.Empty(t, droppedMods(weapon))
}

func TestItemInstance_DestroyedItemPreservesRecoverableMods(t *testing.T) {
    weapon := newWeapon(t, withMods("potency_mod_1"), modsRecoverable: true)
    weapon.Destroy()
    require.Contains(t, droppedMods(weapon), "potency_mod_1")
}
```

- [ ] **Step 3: Implement**:

```go
type ItemInstance struct {
    // ... existing ...
    AffixedMods []string
}
```

Plus the host-item field:

```go
type WeaponDef struct {
    // ... existing ...
    ModsRecoverable bool // when destroyed, drop affixed mods
}
type ArmorDef struct {
    // ... existing ...
    ModsRecoverable bool
}
```

- [ ] **Step 4:** Trade / drop / pickup / save paths preserve the slice. Destroy path consults `ModsRecoverable`.

---

### Task 3: Equip-time bonus wiring — sum + max-rule

**Files:**
- Modify: `internal/game/combat/combatant_effects.go`
- Modify: `internal/game/combat/combatant_effects_test.go`

- [ ] **Step 1: Failing tests** (MOD-8, MOD-9, MOD-10, MOD-11):

```go
func TestEquipWeapon_TakesMaxOfBaseBonusAndPotencyMods(t *testing.T) {
    w := equip(t, "pistol", baseBonus: 1, withMods("potency_mod_2"))
    c := newCombatant(t, withWeapon(w))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatAttack)
    // base 1, mod 2 → effective 2 (not 3).
    require.Equal(t, 2, contributionFromSource(res, "weapon:pistol").Bonus.Value)
}

func TestEquipArmor_TakesMaxOfBaseACAndPotencyMods(t *testing.T) {
    a := equip(t, "leather", baseAC: 1, withMods("armor_potency_mod_2"))
    c := newCombatant(t, withArmor(a))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatAC)
    require.Equal(t, 2, contributionFromSource(res, "item:leather").Bonus.Value)
}

func TestEquipWeapon_NonPotencyModBonusesFlowSeparately(t *testing.T) {
    w := equip(t, "pistol", baseBonus: 1, withMods("extended_magazine_status_dmg")) // adds +1 status to damage
    c := newCombatant(t, withWeapon(w))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatDamage)
    require.NotNil(t, contributionFromSource(res, "mod:extended_magazine_status_dmg"))
}

func TestUnequipWeapon_RemovesAllModBonuses(t *testing.T) {
    w := equip(t, "pistol", withMods("potency_mod_2", "extended_magazine_status_dmg"))
    c := newCombatant(t, withWeapon(w))
    combat.BuildCombatantEffects(c)
    require.NotEmpty(t, c.Effects.BySourcePrefix("mod:"))
    c.UnequipWeapon()
    combat.BuildCombatantEffects(c)
    require.Empty(t, c.Effects.BySourcePrefix("mod:"))
}
```

- [ ] **Step 2: Implement** in `BuildCombatantEffects`:

```go
if w := c.EquippedWeapon(); w != nil {
    sumPotency := 0
    for _, modID := range w.AffixedMods {
        m := inv.ByID(modID)
        if m.Mod != nil && m.Mod.Slot == "weapon" {
            sumPotency += m.Mod.Bonus
        }
    }
    effective := max(w.Def().Bonus, sumPotency)
    c.Effects.Apply(effect.Bonus{Type: effect.BonusItem, Stat: effect.StatAttack, Value: effective, SourceID: "weapon:" + w.Def().ID, CasterUID: c.UID})
    c.Effects.Apply(effect.Bonus{Type: effect.BonusItem, Stat: effect.StatDamage, Value: effective, SourceID: "weapon:" + w.Def().ID, CasterUID: c.UID})

    // Non-potency mod bonuses ride separately by source.
    for _, modID := range w.AffixedMods {
        m := inv.ByID(modID)
        for _, b := range m.Mod.Bonuses {
            b.SourceID = "mod:" + modID
            b.CasterUID = c.UID
            c.Effects.Apply(b)
        }
    }
}
// Same for armor with effect.StatAC.
```

- [ ] **Step 3: Unequip path** — `EffectSet.RemoveBySource("weapon:" + id)` and `RemoveBySourcePrefix("mod:")` for the unequipped item's mods. (`EffectSet` already supports source-keyed removal per #245.)

---

### Task 4: `mod install` / `mod remove` actions — telnet + gRPC + web

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_mods_test.go`
- Create: `internal/frontend/telnet/mod_handler.go`
- Create: `internal/frontend/telnet/mod_handler_test.go`
- Create: `cmd/webclient/ui/src/game/inventory/ModsPanel.tsx`
- Create: `cmd/webclient/ui/src/game/inventory/ModsPanel.test.tsx`

- [ ] **Step 1: Add proto messages** (MOD-12, MOD-13, MOD-14, MOD-15):

```proto
message ModInstallRequest { string character_id = 1; string host_item_id = 2; string mod_item_id = 3; }
message ModInstallResponse { string error = 1; }
message ModRemoveRequest  { string character_id = 1; string host_item_id = 2; int32 slot_index = 3; }
message ModRemoveResponse  { string error = 1; }

message ItemModView {
  int32 slot_index = 1;
  string mod_id = 2;
  string mod_display_name = 3;
}
```

- [ ] **Step 2: Failing handler tests** (MOD-12..15):

```go
func TestModInstall_FillsFirstAvailableSlot(t *testing.T) {
    s := newServer(t, withChar("c1"), withWeapon("pistol", upgradeSlots: 2), withMod("potency_mod_1"))
    _, err := s.ModInstall(ctx, &gamev1.ModInstallRequest{CharacterId: "c1", HostItemId: "pistol-instance", ModItemId: "potency_mod_1-instance"})
    require.NoError(t, err)
    weapon := s.LoadWeapon("pistol-instance")
    require.Equal(t, []string{"potency_mod_1"}, weapon.AffixedMods)
}

func TestModInstall_RejectsSlotMismatch(t *testing.T) {
    s := newServer(t, withChar("c1"), withWeapon("pistol"), withArmorMod("armor_potency_mod_1"))
    _, err := s.ModInstall(ctx, &gamev1.ModInstallRequest{CharacterId: "c1", HostItemId: "pistol-instance", ModItemId: "armor_potency_mod_1-instance"})
    require.Error(t, err)
    require.Contains(t, err.Error(), "slot mismatch")
}

func TestModInstall_RejectsNoSlotsAvailable(t *testing.T) { ... }
func TestModInstall_RejectsNotInInventory(t *testing.T) { ... }

func TestModInstall_RejectsDuringCombat(t *testing.T) {
    s := newServer(t, withChar("c1"), withCombatActive("c1"))
    _, err := s.ModInstall(ctx, &gamev1.ModInstallRequest{CharacterId: "c1", ...})
    require.Error(t, err)
    require.Contains(t, err.Error(), "tinker with gear during combat")
}

func TestModRemove_ReturnsModWhenRecoverable(t *testing.T) { ... }
func TestModRemove_DestroysModWhenNotRecoverable(t *testing.T) { ... }

func TestModInstall_WarnsOnRedundantPotency(t *testing.T) {
    // MOD-Q2: warn but allow when adding a second potency mod that won't stack.
    res, _ := s.ModInstall(ctx, ...)
    require.Contains(t, res.Warning, "this mod's potency will not stack")
}
```

- [ ] **Step 3: Implement** the telnet commands:

```go
func handleModInstall(args []string, sess *Session) string {
    hostID, modID := args[0], args[1]
    if sess.InCombat() {
        return "you can't tinker with gear during combat"
    }
    res, err := sess.GameClient.ModInstall(ctx, &gamev1.ModInstallRequest{...})
    if err != nil { return err.Error() }
    out := "installed."
    if res.Warning != "" { out += " warning: " + res.Warning }
    return out
}
```

- [ ] **Step 4: Web `ModsPanel.tsx`**:

```ts
test("ModsPanel renders slot grid and offers install", () => {
  render(<ModsPanel host={{ upgradeSlots: 3, affixedMods: ["potency_mod_1"] }} />);
  expect(screen.getAllByLabelText(/slot/i)).toHaveLength(3);
  fireEvent.click(screen.getByLabelText("slot 2"));
  expect(screen.getByText(/Choose a mod/)).toBeVisible();
});

test("ModsPanel disables in combat", () => {
  render(<ModsPanel host={...} inCombat />);
  expect(screen.getAllByLabelText(/slot/i)[0]).toBeDisabled();
});

test("ModsPanel right-click on filled slot offers remove", () => { ... });
```

---

### Task 5: Inventory UI — slot badge + `inspect` listing

**Files:**
- Modify: `cmd/webclient/ui/src/game/inventory/InventoryItem.tsx`
- Modify: `internal/frontend/telnet/inspect_handler.go` (or wherever `inspect` renders)

- [ ] **Step 1: Failing tests** (MOD-20, MOD-21):

```ts
test("InventoryItem shows [N/M mods] badge", () => {
  render(<InventoryItem instance={{ upgradeSlots: 3, affixedMods: ["potency_mod_1"] }} />);
  expect(screen.getByText("1/3 mods")).toBeVisible();
});
```

```go
func TestInspect_ListsAffixedMods(t *testing.T) {
    h := newHandler(t, withInstance("pistol", withMods("potency_mod_2", "extended_magazine")))
    out := h.Run("inspect pistol")
    require.Contains(t, out, "Potency Mod II")
    require.Contains(t, out, "Extended Magazine")
}
```

- [ ] **Step 2: Implement** both surfaces.

---

### Task 6: Exemplar mod content + merchant stocking

**Files:**
- Create: `content/items/mods/potency_mod_1.yaml`, `potency_mod_2.yaml`, `potency_mod_3.yaml`, `armor_potency_mod_1.yaml`, `armor_potency_mod_2.yaml`, `armor_potency_mod_3.yaml`, `extended_magazine.yaml`
- Modify: chosen merchant template in `content/npcs/` to stock at least one of each potency tier (per MOD-18)

- [ ] **Step 1: Author** the seven exemplars per MOD-4:

```yaml
# content/items/mods/potency_mod_1.yaml
id: potency_mod_1
kind: mod
display_name: Potency Mod I
description: A modest firmware patch and a trigger-group polish. Sharpens accuracy.
weight: 0.1
base_price: 200
mod:
  slot: weapon
  bonus: 1
  consumes_slots: 1
```

```yaml
# content/items/mods/extended_magazine.yaml
id: extended_magazine
kind: mod
display_name: Extended Magazine
description: Aftermarket high-capacity feed. Faster reloads.
weight: 0.2
base_price: 350
mod:
  slot: weapon
  consumes_slots: 1
  bonuses:
    - { stat: damage, value: 1, type: status }   # MOD-Q5 placeholder — substitute the agreed-on stat
```

(Plus the four other potency mods.)

- [ ] **Step 2: Checkpoint (MOD-18).** Confirm with user which merchant template stocks the mods. Suggested defaults: a weapons merchant in a Warlord Territory zone for the basic tier; a black-market dealer for higher tiers.

- [ ] **Step 3: Smoke test** end-to-end: enter the chosen merchant's room, run `buy potency_mod_1`, run `mod install pistol potency_mod_1`, verify equip-time bonus rises.

---

### Task 7: Architecture documentation

**Files:**
- Modify: `docs/architecture/items.md` (or new `docs/architecture/mods.md`)

- [ ] **Step 1: Author** a "Mods" section documenting:
  - The `Mod` schema and the `slot`/`bonus`/`bonuses`/`consumes_slots` fields.
  - The max-rule for stacking with base item bonus.
  - The non-potency `bonuses` flow (each ride as a separate `Bonus` with `SourceID = "mod:..."`).
  - The `mods_recoverable` host-item field.
  - The install / remove UX contracts (telnet + web).
  - The cyberpunk-friendly naming convention (in-fiction = "mod"; spec/schema = "mod").

- [ ] **Step 2: Cross-link** spec, plan, content directory, and the typed-bonus pipeline doc from #259.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
make migrate-up && make migrate-down  # if migration was authored
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly.
- Telnet smoke test: see Task 6 step 3.
- Web smoke test: open the inventory panel, click a weapon, see the slot grid; click a slot; install a mod; verify the slot badge updates and the StatTooltip on attack shows the mod's contribution.

---

## Rollout / Open Questions Resolved at Plan Time

- **MOD-Q1**: Breakdown tooltip's mod row makes override visible. No combat-log spam.
- **MOD-Q2**: Multiple potency mods install with a warning. Player agency preserved.
- **MOD-Q3**: Items with `UpgradeSlots == 0` reject install with explicit "no slots" error.
- **MOD-Q4**: Mod removal is free in v1.
- **MOD-Q5**: Non-potency exemplar uses an existing stat (`damage` with a status bonus, placeholder until reload mechanic spec). Final stat to confirm with user during exemplar authoring.

## Non-Goals Reaffirmed

Per spec §2.2:

- No general weapon-attachment framework (sights / grips / suppressors).
- No mod crafting.
- No mod tier-gating beyond standard merchant tiers.
- No mod durability / charges.
- No magical / arcane in-fiction vocabulary.
- No multi-stack potency bypass.

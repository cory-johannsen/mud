---
title: Potency Mods (Runes) on Weapons and Armor
issue: https://github.com/cory-johannsen/mud/issues/261
date: 2026-04-25
status: spec
prefix: MOD
depends_on:
  - "#245 Duplicate effects handling (item-typed bonus pipeline)"
  - "#259 Bonus types (UI breakdown shows item bonuses with source labels)"
related:
  - "#262 Agile weapons / MAP (sister item-trait surfacing)"
  - "Material affixing system (precedent for slot-based weapon mods)"
---

# Potency Mods (Runes) on Weapons and Armor

## 1. Summary

The PF2E "potency rune" concept is a +N item-typed bonus on a weapon (attack + damage) or armor (AC). Gunchete is a cyberpunk setting with **no magic**; the lore-prompt (`importer/localizer.go`) explicitly forbids magical concepts. The issue uses the PF2E vocabulary "rune" — this spec implements the mechanic faithfully but renames the in-fiction concept to **potency mod** (chip / firmware patch / aftermarket bolt-on). Throughout the spec, *mod* and *rune* refer to the same mechanism; the YAML / UI strings use *mod*.

The substrate already exists:

- `WeaponDef.Bonus int` (`internal/game/inventory/weapon.go:66`) is a flat int loaded from YAML and is already flowing as a `BonusTypeItem` bonus on `attack` and `damage` via `combatant_effects.go`.
- `ArmorDef.ACBonus int` (`internal/game/inventory/armor.go:26`) loads from YAML and is multiplied by `RarityStatMultiplier` at load. After spec #259 (BTYPE-1) lands it will flow through the typed-bonus pipeline as `BonusTypeItem`.
- `ItemInstance.AffixedMaterials []string` (`internal/game/inventory/backpack.go:12`) is the precedent for per-instance attachable parts; we mirror its shape for mods.
- `WeaponDef.UpgradeSlots` and `ArmorDef.UpgradeSlots` already declare per-item slot counts derived from rarity.

Missing:

1. A `Mod` `ItemKind` and a content directory of mod items (the +1 / +2 / +3 potency mods, plus future non-potency mods).
2. A per-instance `AffixedMods []string` slice on `ItemInstance` mirroring `AffixedMaterials`.
3. An equip-time hook that, on equip of a weapon or armor, sums each affixed mod's `Bonus` into the typed-bonus pipeline.
4. Player UX for installing / removing mods: telnet `mod install <item> <mod>` / `mod remove <item> <slot>`, and a web "Mods" panel on the equipment view.
5. Merchant integration: mods are sold by the existing `weapons`, `armor`, and `technology` merchants per the existing inventory pipeline; no new merchant type required.

## 2. Goals & Non-Goals

### 2.1 Goals

- MOD-G1: A mod is an `ItemDef` with `Kind == ItemKindMod` whose `ApplicableTo` field declares whether it slots into weapons, armor, or both.
- MOD-G2: A potency mod (`+1`, `+2`, `+3`) declares a `Bonus int` and a `Slot string` (`weapon`, `armor`); equipping the host item adds the mod's bonus to the typed-bonus pipeline as item-typed.
- MOD-G3: Per-instance mod loadout (`AffixedMods []string`) persists with the item, mirrors `AffixedMaterials`, and survives drop / pick-up / trade.
- MOD-G4: A weapon's effective `WeaponBonus` is `max(WeaponDef.Bonus, sum(potency mods))` — a base-+2 weapon does NOT stack with an installed +1 mod (item-typed bonuses do not stack within type per #259). The higher of the two applies.
- MOD-G5: Player UX (telnet + web) for installing / removing mods within the host item's `UpgradeSlots` capacity.
- MOD-G6: At least three potency mods (`potency_mod_1`, `potency_mod_2`, `potency_mod_3`) and one non-potency exemplar mod (e.g., `extended_magazine`) author and validate at landing.
- MOD-G7: Existing tests pass; `WeaponDef.Bonus` continues to work for legacy `+N` weapons.

### 2.2 Non-Goals

- MOD-NG1: A general "weapon attachment" framework with sights, grips, suppressors, etc. v1 ships only mods that emit typed bonuses.
- MOD-NG2: Crafting / recipe system. Mods are sold only.
- MOD-NG3: Mod tier-gating / faction restrictions. Standard merchant tier gates suffice.
- MOD-NG4: Removable durability / charges on mods (a mod is permanent once installed unless explicitly removed via the remove action).
- MOD-NG5: Magical / arcane vocabulary in any user-facing string. The word "rune" appears only in this spec's title (mirroring the issue text) and in the schema field names that users do not see.
- MOD-NG6: Stacking multiple identical potency mods on one item to bypass the type-stacking rule (handled by MOD-G4).

## 3. Glossary

- **Mod**: an `ItemDef` of kind `mod` that, when affixed to a host weapon or armor, contributes typed bonuses while the host is equipped.
- **Potency mod**: a mod whose primary `Bonus` declares an integer item bonus to attack+damage (weapon) or AC (armor).
- **Host item**: the weapon or armor instance carrying a mod.
- **Slot**: an integer index into the host item's `UpgradeSlots`. Each slot holds at most one mod.
- **Affixed mods**: the per-instance list `ItemInstance.AffixedMods` mirroring `AffixedMaterials`.

## 4. Requirements

### 4.1 Mod Item Schema

- MOD-1: A new `ItemKind` constant `ItemKindMod` MUST be added; the loader MUST recognize `kind: mod` in YAML.
- MOD-2: Mod `ItemDef` MUST carry these fields beyond the base shape:
  - `slot` (string): one of `weapon`, `armor`. Determines which host item types accept the mod.
  - `bonus` (int, optional): when set, contributes to the host's effective bonus (attack+damage for `weapon` slot, AC for `armor` slot).
  - `bonuses` (list, optional): additional typed bonuses to apply while the host is equipped, using the bonuses schema from the typed-bonus pipeline (#245). Allows non-potency mods to add non-stat-bonus effects (e.g., a weapon mod that adds `+1 status to damage` — note the type is the author's choice, NOT auto-item-typed).
  - `consumes_slots` (int, default `1`): how many of the host item's `UpgradeSlots` this mod consumes.
  - `installer_dc` (int, optional): a Crafting / Engineering DC for self-install (out of scope to fully implement v1; reserve the field).
- MOD-3: The loader MUST validate that:
  - `slot` is one of the two recognized values.
  - `bonus` and `bonuses` may be set together; both apply.
  - `consumes_slots >= 1`.
- MOD-4: At least four exemplar mods MUST be authored under `content/items/mods/`:
  - `potency_mod_1` — `slot: weapon`, `bonus: 1`.
  - `potency_mod_2` — `slot: weapon`, `bonus: 2`.
  - `potency_mod_3` — `slot: weapon`, `bonus: 3`.
  - `armor_potency_mod_1` — `slot: armor`, `bonus: 1`.
  - `armor_potency_mod_2` — `slot: armor`, `bonus: 2`.
  - `armor_potency_mod_3` — `slot: armor`, `bonus: 3`.
  - `extended_magazine` — non-potency exemplar, `slot: weapon`, `bonuses: [{ stat: "reload_speed", value: 1, type: "item" }]` (placeholder; wire to actual reload mechanic later).

### 4.2 Per-Instance Affix Persistence

- MOD-5: `ItemInstance` MUST gain a field `AffixedMods []string` mirroring `AffixedMaterials` semantics: each entry is the `ItemDef.ID` of an affixed mod; order corresponds to slot index.
- MOD-6: The serialization paths (DB save/load, trade, drop, pickup) MUST round-trip `AffixedMods` losslessly. A migration MAY be required for the items table or its JSON column; the implementer MUST verify and write the migration if needed.
- MOD-7: When an item is destroyed (durability zero), affixed mods MUST also be destroyed unless the host weapon/armor declares `mods_recoverable: true` (default `false`). This is a property on the host item, not the mod.

### 4.3 Equip-Time Bonus Wiring

- MOD-8: When a weapon is equipped, `BuildCombatantEffects` MUST sum potency-mod `bonus` values from `AffixedMods`, then take `max(WeaponDef.Bonus, sumPotencyMods)` and apply that as the single item-typed bonus to attack+damage. This honors the type-stacking rule from #259 (item bonuses do not stack within type).
- MOD-9: When armor is equipped, the same max-rule MUST apply with `ArmorDef.ACBonus * rarityMultiplier` vs the sum of armor potency mods.
- MOD-10: Non-potency mod `bonuses` (per-mod typed-bonus list from MOD-2) MUST be appended to the EffectSet as separate `Bonus` entries, each with `SourceID = "mod:<modID>"` so the resolver dedups correctly and the breakdown UI shows them by source.
- MOD-11: When the host item is unequipped, all mod-derived bonuses MUST be removed from the EffectSet via the existing `RemoveBySource` API on `EffectSet`.

### 4.4 Install / Remove Actions

- MOD-12: A new telnet command `mod install <host_item_id> <mod_item_id>` MUST install the named mod into the host's first available slot. Failures (slot mismatch, no slots available, mod not in inventory) emit clear errors and consume nothing.
- MOD-13: A new telnet command `mod remove <host_item_id> <slot_index>` MUST remove the mod at the named slot index, returning the mod to the player's inventory (assuming `mods_recoverable: true` on the host, otherwise destroying it).
- MOD-14: A new web component `ModsPanel.tsx` MUST live under `cmd/webclient/ui/src/game/inventory/` showing the host item's slot grid; clicking a slot opens an installer for compatible mods from the player's inventory; right-clicking a filled slot offers remove.
- MOD-15: Install / remove MUST require the player to be out of combat. Combat-state validation rejects with "you can't tinker with gear during combat".

### 4.5 Merchant Integration

- MOD-16: Mods MUST be salable through the existing `weapons`, `armor`, and `technology` merchant types. No new `MerchantType` is added.
- MOD-17: A merchant's `Inventory` MAY include any mod items by id; the existing pricing pipeline (`ComputeBuyPrice`) applies unchanged.
- MOD-18: At least one merchant template (existing or new) MUST stock at least one of each potency mod tier as an exemplar. Choice of merchant to be confirmed with the user.

### 4.6 UI Surfacing

- MOD-19: Per spec #259's BTYPE-9 breakdown tooltip on attack and AC totals, the mod-derived bonus MUST appear as a contribution row labeled with the mod's display name (e.g., `+1 (item) from Potency Mod I`).
- MOD-20: The web inventory panel MUST show a small `[N/M mods]` badge on each weapon and armor instance indicating used / total slots.
- MOD-21: Telnet `inspect <item>` MUST list the host item's affixed mods one-per-line below the existing inspect output.

### 4.7 Tests

- MOD-22: Existing weapon / armor / equipment tests MUST pass unchanged.
- MOD-23: New tests in `internal/game/inventory/mods_test.go` MUST cover:
  - Mod load + validation (MOD-3 invariants).
  - Per-instance round-trip of `AffixedMods` (MOD-6).
  - Equip-time bonus computation: `max(Bonus, sumMods)` (MOD-8) and the same for armor (MOD-9).
  - Non-potency mod `bonuses` flow through EffectSet by source (MOD-10).
  - Install / remove handler validation (MOD-12, MOD-13, MOD-15).
  - Mods recoverable vs destroyed semantics (MOD-7).

## 5. Architecture

### 5.1 Where the new code lives

```
content/items/mods/
  potency_mod_1.yaml, potency_mod_2.yaml, potency_mod_3.yaml,
  armor_potency_mod_1.yaml, armor_potency_mod_2.yaml, armor_potency_mod_3.yaml,
  extended_magazine.yaml

internal/game/inventory/
  item.go                            # ItemKindMod constant; ApplicableTo and Slot fields
  mod.go                             # NEW: Mod loader + validation
  backpack.go                        # ItemInstance.AffixedMods []string
  mods_test.go                       # NEW

internal/game/combat/
  combatant_effects.go               # BuildCombatantEffects: sum mods, take max,
                                     # append non-potency mod bonuses by source

internal/gameserver/
  grpc_service.go                    # ModInstall / ModRemove RPCs

api/proto/game/v1/game.proto
  ModInstallRequest / ModRemoveRequest / ItemModView messages

cmd/webclient/ui/src/game/inventory/
  ModsPanel.tsx                      # NEW
  InventoryItem.tsx                  # existing; gains slot badge per MOD-20

internal/frontend/telnet/
  mod_handler.go                     # `mod install`, `mod remove` commands

migrations/
  NNN_item_instance_affixed_mods.up.sql / .down.sql  # only if items table needs the column
```

### 5.2 Equip flow

```
player equips weapon W
   │
   ▼
BuildCombatantEffects (existing)
   │
   ├── sumMods = sum(W.AffixedMods → ItemKindMod.Bonus where Slot == "weapon")
   ├── effectiveBonus = max(W.WeaponDef.Bonus, sumMods)
   ├── EffectSet.Apply(Bonus{ Stat: attack, Value: effectiveBonus, Type: item, SourceID: "weapon:" + W.ID, ... })
   ├── EffectSet.Apply(Bonus{ Stat: damage, Value: effectiveBonus, Type: item, SourceID: "weapon:" + W.ID, ... })
   │
   ├── for each mod in W.AffixedMods:
   │       for each Bonus b in mod.Bonuses (non-potency):
   │             EffectSet.Apply(Bonus{ ...b, SourceID: "mod:" + mod.ID, CasterUID: combatant.UID })
   │
   ▼
character is in steady state; resolver reads EffectSet on every roll
```

### 5.3 Single sources of truth

- Mod schema: `internal/game/inventory/mod.go`.
- Mod-derived bonus dispatch: `BuildCombatantEffects` only.
- Per-instance affix persistence: `ItemInstance.AffixedMods` only.

## 6. Open Questions

- MOD-Q1: When a host weapon's `WeaponDef.Bonus` is `+1` and the player installs a `+2` potency mod, does the in-fiction narrative mention the mod overriding the base bonus? Recommendation: yes — the breakdown tooltip's `(item) from Potency Mod II` row makes it visible without spam in the combat log.
- MOD-Q2: Can the player install *multiple* potency mods on one weapon (e.g., `+1` + `+2`)? Mechanically the type-stacking rule (MOD-G4 / MOD-8) ensures only the highest applies, so the second mod is wasted. Should the install action *warn* before allowing the second install? Recommendation: yes, warn but allow — preserves player agency.
- MOD-Q3: Mods on items with `UpgradeSlots == 0` (not all gear has slots). Confirm the install handler rejects with "this item has no upgrade slots". Recommendation: yes, explicit error.
- MOD-Q4: Mods recovered via remove action — do they consume an action / time? Recommendation: free in v1; revisit if abused.
- MOD-Q5: The `extended_magazine` non-potency exemplar references a `reload_speed` stat that may not yet exist. Should this exemplar wait for the reload-mechanic spec? Recommendation: implement the field as a future-proof `Bonus` with the existing `Stat` enum (extending the enum is fine), or substitute a different non-potency exemplar that uses an existing stat (e.g., a `damage_die_step` is too complex — pick `+1 status to damage` for the exemplar).

## 7. Acceptance

- [ ] All existing weapon / armor / equipment tests pass.
- [ ] Three weapon and three armor potency mods author and validate at startup.
- [ ] One non-potency exemplar mod author and validate.
- [ ] Equipping a `+1` weapon with a `+2` mod produces an effective `+2` attack/damage bonus (max-rule).
- [ ] Equipping armor with a potency mod adds the mod's bonus to AC under the same max-rule.
- [ ] Telnet `mod install` and `mod remove` work; web `ModsPanel.tsx` mirrors.
- [ ] Per-instance `AffixedMods` round-trips through trade / drop / pickup.
- [ ] BTYPE-9 breakdown tooltip shows the mod row with its display name.
- [ ] Combat-time install attempt is rejected with a clear error.

## 8. Out-of-Scope Follow-Ons

- MOD-F1: Generalized weapon attachments (sights, grips, suppressors).
- MOD-F2: Mod crafting / synthesis recipes.
- MOD-F3: Mod tier-gating / faction restrictions.
- MOD-F4: Mod durability / charges (single-use mods).
- MOD-F5: Self-install Crafting DC (`installer_dc` field reserved but not enforced v1).
- MOD-F6: A "mod set bonus" mechanic (matching three of a kind grants a synergy effect).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/261
- Weapon bonus field: `internal/game/inventory/weapon.go:66`
- Armor AC bonus field: `internal/game/inventory/armor.go:26`
- Affixed materials precedent: `internal/game/inventory/backpack.go:12`
- Upgrade slots precedent: `internal/game/inventory/weapon.go:61`, `armor.go:50`
- Item-typed weapon bonus already wired: `internal/game/combat/combatant_effects.go:18-22`
- Typed bonus model: `internal/game/effect/bonus.go`
- Stacking rule (item bonuses): `docs/superpowers/specs/2026-04-21-duplicate-effects-handling.md` DEDUP rules
- Breakdown UI tooltip: `docs/superpowers/specs/2026-04-25-bonus-types.md` BTYPE-9
- Cyberpunk lore prompt (no magic): `internal/importer/localizer.go`

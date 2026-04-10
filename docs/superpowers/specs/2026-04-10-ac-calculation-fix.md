# Spec: AC Calculation Fix — Proficiency Bonus and Multi-Slot Armor

**GitHub Issue:** cory-johannsen/mud#36
**Date:** 2026-04-10

## Root Cause

In `internal/game/inventory/equipment.go`, `ComputedDefensesWithProficiencies()` adds `armorProfBonus(level, rank)` once **per equipped slot** that the player is proficient in. With 8 slots and a proficiency bonus of +10, this produces +80 from proficiency alone, yielding the reported armor total of +89 instead of ~+19 (9 item + 10 proficiency).

---

## Design Decision: Multi-Slot Proficiency Model

The game allows equipping multiple armor pieces across 8 slots with mixed types (light/medium/heavy). Four options were considered and Option B was selected by the user:

| Option | Approach | Verdict |
|--------|----------|---------|
| A | Sum all item bonuses; apply proficiency once using highest equipped category | Rejected — too permissive |
| B | Only count slots where player is proficient; apply proficiency once by highest proficient+equipped category | **Selected** |
| C | Single torso-slot armor only (PF2e RAW) | Rejected — breaks multi-slot system |
| D | Per-slot proficiency check + one bonus (effectively identical to B) | Rejected — same as B |

**Selected approach: Option B**

Only equipped armor pieces whose `proficiency_category` the player is proficient in contribute their `ac_bonus`. Proficiency bonus is applied **once** based on the player's highest-ranked proficiency among the categories they are both proficient in and have equipped. Unequipped or unproficient slots contribute no AC bonus.

---

## REQ-1: Per-slot proficiency filtering

- REQ-1a: In the `ComputedDefenses()` item loop, each slot's `ac_bonus` MUST only be added to `stats.ACBonus` if the player is proficient in that item's `proficiency_category`
- REQ-1b: Slots where the player is NOT proficient in the item's category MUST contribute 0 to `ACBonus`
- REQ-1c: Broken armor (durability = 0) continues to be skipped regardless of proficiency (unchanged)

## REQ-2: Effective proficiency category

- REQ-2a: After the per-slot loop, the effective proficiency category MUST be determined as the heaviest `proficiency_category` among all slots that passed the REQ-1a check (i.e. proficient AND equipped)
- REQ-2b: Category precedence MUST be: `heavy_armor` > `medium_armor` > `light_armor` > `unarmored`
- REQ-2c: If no proficient slots exist, effective category MUST be `unarmored`

## REQ-3: Proficiency bonus applied once

- REQ-3a: `ComputedDefensesWithProficiencies()` MUST apply `armorProfBonus(level, rank)` exactly once to `stats.ACBonus`
- REQ-3b: The rank used MUST be the player's proficiency rank in the effective category (REQ-2a)
- REQ-3c: The per-slot loop MUST NOT add any proficiency bonus — item bonuses only

## REQ-4: Check penalty

- REQ-4a: Check penalty is summed from all equipped, non-broken pieces regardless of proficiency (unchanged)

## REQ-5: DexCap

- REQ-5a: DexCap is the strictest (minimum) across all equipped, non-broken pieces (unchanged)

## REQ-6: CharacterSheetView breakdown

- REQ-6a: `view.AcBonus` MUST reflect the corrected item bonus total (proficient slots only, no proficiency bonus)
- REQ-6b: A new field `proficiency_ac_bonus int32` MUST be added to `CharacterSheetView` for the single proficiency contribution
- REQ-6c: A new field `effective_armor_category string` MUST be added to `CharacterSheetView` so the client can display the AC breakdown (supports issue #32)

## Files to Modify

- `internal/game/inventory/equipment.go` — fix `ComputedDefensesWithProficiencies()` per REQ-1 through REQ-3
- `api/proto/game/v1/game.proto` — add `proficiency_ac_bonus` and `effective_armor_category` to `CharacterSheetView`
- `api/proto/game/v1/game.pb.go` — regenerate after proto change
- `internal/gameserver/grpc_service.go` — populate new `CharacterSheetView` fields (lines ~5861-5866)

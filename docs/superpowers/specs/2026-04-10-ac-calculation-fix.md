# Spec: AC Calculation Fix — Proficiency Bonus and Multi-Slot Armor

**GitHub Issue:** cory-johannsen/mud#36
**Date:** 2026-04-10

## Root Cause

In `internal/game/inventory/equipment.go`, `ComputedDefensesWithProficiencies()` adds `armorProfBonus(level, rank)` once **per equipped slot** that the player is proficient in. With 8 slots and a proficiency bonus of +10, this produces +80 from proficiency alone, yielding the reported armor total of +89 instead of ~+19 (9 item + 10 proficiency).

---

## Design Decision: Multi-Slot Proficiency Model

The game allows equipping multiple armor pieces across 8 slots with mixed types (light/medium/heavy). Four options were considered:

| Option | Approach | Verdict |
|--------|----------|---------|
| A | Sum all item bonuses; apply proficiency once using highest equipped category | **Selected** |
| B | Only count slots where player is proficient; apply proficiency once | Overly punishing for mixed kits |
| C | Single torso-slot armor only (PF2e RAW) | Breaks the multi-slot equipment system |
| D | Per-slot proficiency check + one bonus | Too complex; harsh for mixed kits |

**Selected approach: Option A**

The player's effective armor category is the **heaviest category among all equipped armor pieces**. Proficiency bonus is applied **once** based on the player's proficiency rank in that effective category. All equipped item AC bonuses are always summed regardless of proficiency. If the player is not proficient in the effective category, the proficiency contribution is untrained (0) and a check penalty applies.

This matches the spirit of PF2e (one proficiency bonus, determined by armor category) while supporting the multi-slot equipment model.

---

## REQ-1: Effective armor category

- REQ-1a: `ComputedDefenses()` MUST determine the effective armor category as the heaviest category present among all equipped, non-broken armor pieces
- REQ-1b: Category precedence MUST be: `heavy_armor` > `medium_armor` > `light_armor` > `unarmored`
- REQ-1c: If no armor is equipped, effective category MUST be `unarmored`

## REQ-2: Proficiency bonus applied once

- REQ-2a: `ComputedDefensesWithProficiencies()` MUST apply `armorProfBonus(level, rank)` exactly once to `stats.ACBonus`
- REQ-2b: The rank used MUST be the player's proficiency rank in the effective armor category (REQ-1a)
- REQ-2c: If the player has no entry in `Proficiencies` for the effective category, rank MUST default to `"untrained"` (contributes 0)
- REQ-2d: The per-slot loop MUST NOT add any proficiency bonus — it MUST accumulate item AC bonuses only

## REQ-3: Check penalty

- REQ-3a: Check penalty MUST be applied if the player is untrained in the effective armor category
- REQ-3b: Check penalty value MUST be the sum of `CheckPenalty` fields from all equipped, non-broken pieces (unchanged from current logic)

## REQ-4: DexCap

- REQ-4a: DexCap calculation is unchanged — the strictest (minimum) DexCap across all equipped pieces applies

## REQ-5: CharacterSheetView breakdown

- REQ-5a: The proto `CharacterSheetView` MUST expose the effective armor category and proficiency rank so the client can display the AC breakdown (resolves issue #32)
- REQ-5b: `view.AcBonus` MUST reflect only the corrected item bonus total (no proficiency)
- REQ-5c: A new field `proficiency_ac_bonus int32` MUST be added to `CharacterSheetView` for the proficiency contribution
- REQ-5d: A new field `effective_armor_category string` MUST be added to `CharacterSheetView`

## Files to Modify

- `internal/game/inventory/equipment.go` — fix `ComputedDefensesWithProficiencies()` per REQ-1 and REQ-2; add effective category computation
- `api/proto/game/v1/game.proto` — add `proficiency_ac_bonus` and `effective_armor_category` to `CharacterSheetView` (REQ-5)
- `api/proto/game/v1/game.pb.go` — regenerate after proto change
- `internal/gameserver/grpc_service.go` — populate new `CharacterSheetView` fields (lines ~5861-5866)

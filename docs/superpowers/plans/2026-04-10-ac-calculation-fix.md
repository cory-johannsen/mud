# Plan: AC Calculation Fix — Proficiency Bonus and Multi-Slot Armor

**GitHub Issue:** cory-johannsen/mud#36
**Spec:** `docs/superpowers/specs/2026-04-10-ac-calculation-fix.md`
**Date:** 2026-04-10

---

## Step 1 — Fix ComputedDefensesWithProficiencies() (REQ-1, REQ-2, REQ-3)

**File:** `internal/game/inventory/equipment.go` (lines 227-290)

Rewrite the per-slot loop and post-loop logic:

**Current (buggy) per-slot behavior:**
- Always adds `slotAC` to `stats.ACBonus`
- If proficient: also adds `armorProfBonus(level, rank)` — BUG: runs once per slot

**New behavior (Option B):**
1. Track `proficientSlots []string` — collect `def.ProficiencyCategory` for each slot where `rank != ""`
2. Only add `slotAC` to `stats.ACBonus` when `rank != ""` (proficient); skip unproficient slot item bonuses
3. Unproficient slots still contribute `CheckPenalty`, `SpeedPenalty`, `DexCap`, `StrengthReq` (unchanged)
4. After the loop, determine `effectiveCategory` = heaviest category in `proficientSlots` using precedence `heavy_armor > medium_armor > light_armor > unarmored`
5. Apply `armorProfBonus(level, profs[effectiveCategory])` exactly once to `stats.ACBonus`

Also add two new fields to `DefenseStats` struct (lines 123-134):
```go
ProficiencyACBonus      int    // the single proficiency contribution
EffectiveArmorCategory  string // heaviest proficient category equipped
```
Populate these in step 4/5 above so callers can report them to clients.

**TDD (write tests first in `equipment_proficiency_test.go`):**
- Mixed light+medium equipped, proficient in both: item bonuses from both slots count, proficiency bonus applied once using medium
- Unproficient slot: item bonus excluded, penalties still applied
- No proficient slots: ACBonus = 0 proficiency contribution, EffectiveArmorCategory = "unarmored"
- Single proficient slot: proficiency bonus applied once
- Property-based: for any equipment configuration, `ProficiencyACBonus` appears exactly once in total ACBonus

---

## Step 2 — Extend DefenseStats and update ComputedDefenses() (REQ-1c)

**File:** `internal/game/inventory/equipment.go`

`ComputedDefenses()` (lines 141-193) does not take proficiencies — it always adds all item AC bonuses. No behavioral change needed there. Only `ComputedDefensesWithProficiencies()` needs the fix from Step 1.

`ComputedDefensesWithProficienciesAndSetBonuses()` (lines 297-301) wraps `ComputedDefensesWithProficiencies()` and adds `sb.ACBonus` — no change needed; it inherits the fix automatically.

---

## Step 3 — Proto: add breakdown fields to CharacterSheetView (REQ-6b, REQ-6c)

**File:** `api/proto/game/v1/game.proto`

Add to `CharacterSheetView`:
```proto
int32  proficiency_ac_bonus     = 53;
string effective_armor_category = 54;
```

Regenerate `api/proto/game/v1/game.pb.go`:
```bash
cd api/proto/game/v1 && protoc --go_out=. --go_opt=paths=source_relative game.proto
```

---

## Step 4 — Populate new CharacterSheetView fields (REQ-6)

**File:** `internal/gameserver/grpc_service.go` (lines ~5860-5866)

Update the AC block to populate the new fields:
```go
dexMod := (sess.Abilities.Quickness - 10) / 2
def := sess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(
    s.invRegistry, dexMod, sess.Proficiencies, sess.Level, sess.SetBonusSummary)
view.AcBonus = int32(def.ACBonus - def.ProficiencyACBonus)  // item bonuses only
view.ProficiencyAcBonus = int32(def.ProficiencyACBonus)
view.EffectiveArmorCategory = def.EffectiveArmorCategory
view.CheckPenalty = int32(def.CheckPenalty)
view.SpeedPenalty = int32(def.SpeedPenalty)
view.TotalAc = int32(10 + def.EffectiveDex + def.ACBonus)
```

Note: `TotalAc` remains `10 + EffectiveDex + ACBonus` since `ACBonus` now correctly includes item bonuses + one proficiency bonus.

**TDD:** Add a test in `grpc_service_char_test.go` (or equivalent) asserting that a level-8 character with 7 proficient armor slots has `TotalAc = 31` (10 + 2 Dex + 9 items + 10 proficiency) and `ProficiencyAcBonus = 10`.

---

## Step 5 — Update TypeScript types (supports issue #32 hover breakdown)

**File:** `cmd/webclient/ui/src/proto/index.ts`

Add to the `CharacterSheetView` interface:
```typescript
proficiencyAcBonus?: number
proficiency_ac_bonus?: number
effectiveArmorCategory?: string
effective_armor_category?: string
```

---

## Step 6 — Run full test suite and verify

```bash
mise exec -- go test ./internal/game/inventory/... -count=1 -v
mise exec -- go test ./internal/gameserver/... -count=1
cd cmd/webclient/ui && npm run build
```

All tests must pass. Build must succeed with no TypeScript errors.

---

## Dependency Order

```
Step 1 (fix equipment.go) ──▶ Step 2 (no-op, verify)
Step 1 ──▶ Step 4 (populate view fields)
Step 3 (proto) ──▶ Step 4
Step 3 ──▶ Step 5 (TS types)
Step 4 + Step 5 ──▶ Step 6 (test suite)
```

Steps 1 and 3 are independent and can run in parallel.
Step 5 is independent of Steps 1 and 4 and can run in parallel with them.

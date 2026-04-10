# Plan: Job Panel Technology Grants and Selections Display

**GitHub Issue:** cory-johannsen/mud#35
**Spec:** `docs/superpowers/specs/2026-04-10-job-panel-technology-grants.md`
**Date:** 2026-04-10

---

## Root Cause Summary

`addTechGrants` (grpc_service.go lines 7165-7199) handles `Hardwired`, `Prepared.Fixed`, and `Spontaneous.Fixed` entries but silently skips:

- `Prepared.SlotsByLevel` — prepared slot count grants (e.g. "+2 prepared slots for level 1 tech")
- `Spontaneous.UsesByLevel` — spontaneous use pool grants (e.g. "+3 uses of level 1 tech")

These are emitted as synthetic `JobTechGrant` entries using `tech_type = "prepared_slot"` or `"spontaneous_use"` per REQ-6b. `JobDrawer.tsx` already iterates `techGrants` but has no rendering path for these two new types.

---

## Step 1 — TDD: Add failing tests for slot/use grants (REQ-3, REQ-4, REQ-5)

**File:** `internal/gameserver/grpc_service_job_grants_test.go`

Add three new test functions at the end of the file:

### Test A: `TestHandleJobGrants_PreparedSlotsByLevel_EmittedAsSyntheticGrants`

```go
// Precondition: job has TechnologyGrants.Prepared.SlotsByLevel = {1: 2, 2: 1}.
// Postcondition: response contains synthetic JobTechGrant entries for each (techLevel, count) pair,
//   tech_type = "prepared_slot", tech_name = "+N Prepared Slot (Level X tech)".
```

Assert:
- Two `prepared_slot` entries in TechGrants
- Entry for techLevel=1: `TechName = "+2 Prepared Slot (Level 1 tech)"`, `TechLevel = 1`, `TechId = ""`
- Entry for techLevel=2: `TechName = "+1 Prepared Slot (Level 2 tech)"`, `TechLevel = 2`, `TechId = ""`
- `GrantLevel = 1` on both (creation grant)

### Test B: `TestHandleJobGrants_SpontaneousUsesByLevel_EmittedAsSyntheticGrants`

```go
// Precondition: job has TechnologyGrants.Spontaneous.UsesByLevel = {1: 3}.
// Postcondition: response contains one synthetic JobTechGrant,
//   tech_type = "spontaneous_use", tech_name = "+3 Use (Level 1 tech)".
```

Assert:
- One `spontaneous_use` entry with `TechName = "+3 Use (Level 1 tech)"`, `TechLevel = 1`, `TechId = ""`, `GrantLevel = 1`

### Test C: `TestHandleJobGrants_LevelUp_SlotAndUseGrantsAtCorrectLevel`

```go
// Precondition: job has LevelUpGrants[3].Prepared.SlotsByLevel = {2: 1}
//   and LevelUpGrants[5].Spontaneous.UsesByLevel = {1: 2}. Player level = 5.
// Postcondition: slot grant has GrantLevel = 3, use grant has GrantLevel = 5.
```

Assert:
- `prepared_slot` entry has `GrantLevel = 3`
- `spontaneous_use` entry has `GrantLevel = 5`

Run tests — they MUST fail before any implementation change:
```bash
mise exec -- go test ./internal/gameserver/... -run "TestHandleJobGrants_Prepared\|TestHandleJobGrants_Spontaneous\|TestHandleJobGrants_LevelUp_Slot" -count=1 -v
```

---

## Step 2 — Extend `addTechGrants` to emit slot/use grants (REQ-3, REQ-4, REQ-5)

**File:** `internal/gameserver/grpc_service.go` (lines 7177-7198, inside `addTechGrants`)

After the existing `tg.Prepared.Fixed` loop, add a `SlotsByLevel` loop. After the existing `tg.Spontaneous.Fixed` loop, add a `UsesByLevel` loop. Sort keys for deterministic output.

Replace the `addTechGrants` closure body (lines 7165-7199) with:

```go
addTechGrants := func(tg *ruleset.TechnologyGrants, level int) {
    if tg == nil {
        return
    }
    for _, id := range tg.Hardwired {
        techGrants = append(techGrants, &gamev1.JobTechGrant{
            GrantLevel: int32(level),
            TechId:     id,
            TechName:   techName(id),
            TechType:   "hardwired",
        })
    }
    if tg.Prepared != nil {
        for _, e := range tg.Prepared.Fixed {
            techGrants = append(techGrants, &gamev1.JobTechGrant{
                GrantLevel: int32(level),
                TechId:     e.ID,
                TechName:   techName(e.ID),
                TechLevel:  int32(e.Level),
                TechType:   "prepared",
            })
        }
        // Emit slot count grants sorted by tech level for deterministic ordering.
        slotLevels := make([]int, 0, len(tg.Prepared.SlotsByLevel))
        for tl := range tg.Prepared.SlotsByLevel {
            slotLevels = append(slotLevels, tl)
        }
        sort.Ints(slotLevels)
        for _, tl := range slotLevels {
            count := tg.Prepared.SlotsByLevel[tl]
            techGrants = append(techGrants, &gamev1.JobTechGrant{
                GrantLevel: int32(level),
                TechLevel:  int32(tl),
                TechName:   fmt.Sprintf("+%d Prepared Slot (Level %d tech)", count, tl),
                TechType:   "prepared_slot",
            })
        }
    }
    if tg.Spontaneous != nil {
        for _, e := range tg.Spontaneous.Fixed {
            techGrants = append(techGrants, &gamev1.JobTechGrant{
                GrantLevel: int32(level),
                TechId:     e.ID,
                TechName:   techName(e.ID),
                TechLevel:  int32(e.Level),
                TechType:   "spontaneous",
            })
        }
        // Emit use pool grants sorted by tech level for deterministic ordering.
        useLevels := make([]int, 0, len(tg.Spontaneous.UsesByLevel))
        for tl := range tg.Spontaneous.UsesByLevel {
            useLevels = append(useLevels, tl)
        }
        sort.Ints(useLevels)
        for _, tl := range useLevels {
            count := tg.Spontaneous.UsesByLevel[tl]
            techGrants = append(techGrants, &gamev1.JobTechGrant{
                GrantLevel: int32(level),
                TechLevel:  int32(tl),
                TechName:   fmt.Sprintf("+%d Use (Level %d tech)", count, tl),
                TechType:   "spontaneous_use",
            })
        }
    }
}
```

Verify `sort` and `fmt` are already imported (both are standard library; `fmt` is used elsewhere in the file).

---

## Step 3 — Run failing tests to confirm they now pass

```bash
mise exec -- go test ./internal/gameserver/... -run "TestHandleJobGrants_Prepared\|TestHandleJobGrants_Spontaneous\|TestHandleJobGrants_LevelUp_Slot" -count=1 -v
```

All three tests from Step 1 MUST pass.

---

## Step 4 — Update `JobDrawer.tsx` to render new tech types (REQ-1b, REQ-1c, REQ-3b, REQ-4b)

**File:** `cmd/webclient/ui/src/game/drawers/JobDrawer.tsx` (lines 111-121)

The existing color logic (line 113-115) maps `hardwired`, `prepared`, `spontaneous` only. Add `prepared_slot` and `spontaneous_use` cases. Per REQ-1c, slot grants use prepared color (#ffcc88); use grants use spontaneous color (#cc88ff).

Replace lines 111-121 with:

```tsx
{techs.map((g, i) => {
  const type = g.techType ?? g.tech_type ?? ''
  const techLvl = g.techLevel ?? g.tech_level ?? 0
  const isSlot = type === 'prepared_slot'
  const isUse = type === 'spontaneous_use'
  const baseType = isSlot ? 'prepared' : isUse ? 'spontaneous' : type
  const typeColor = baseType === 'hardwired' ? '#88ccff' : baseType === 'prepared' ? '#ffcc88' : '#cc88ff'
  const typeBg = baseType === 'hardwired' ? 'rgba(100,180,255,0.12)' : baseType === 'prepared' ? 'rgba(200,140,40,0.12)' : 'rgba(180,80,255,0.12)'
  const typeBorder = baseType === 'hardwired' ? 'rgba(100,180,255,0.3)' : baseType === 'prepared' ? 'rgba(200,140,40,0.3)' : 'rgba(180,80,255,0.3)'
  const label = isSlot ? 'slot' : isUse ? 'use' : (type || 'tech')
  return (
    <div key={`tech-${i}`} style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '3px', paddingLeft: '4px' }}>
      <span style={{ fontSize: '0.7rem', color: typeColor, background: typeBg, border: `1px solid ${typeBorder}`, borderRadius: '4px', padding: '0 4px' }}>{label}</span>
      <span style={{ color: '#ddd', fontSize: '0.85rem' }}>{g.techName ?? g.tech_name ?? g.techId ?? g.tech_id}</span>
      {techLvl > 0 && !isSlot && !isUse && <span style={{ fontSize: '0.75rem', color: '#888' }}>lv{techLvl}</span>}
    </div>
  )
})}
```

Note: `lv{techLvl}` is suppressed for slot/use grants because the level is already encoded in `TechName`. Hardwired color also corrected to spec value `#88ccff` (was `#c0e0a0`).

---

## Step 5 — Run full test suite and TypeScript build

```bash
mise exec -- go test ./internal/gameserver/... -count=1
cd cmd/webclient/ui && npm run build
```

All Go tests MUST pass. TypeScript build MUST succeed with no errors.

---

## Dependency Order

```
Step 1 (write failing tests) ──▶ Step 2 (implement addTechGrants) ──▶ Step 3 (verify tests pass)
Step 3 ──▶ Step 5 (full suite)
Step 4 (JobDrawer.tsx) ──▶ Step 5 (TS build)
```

Steps 2 and 4 are independent once Step 1 tests exist — they can be implemented in parallel.

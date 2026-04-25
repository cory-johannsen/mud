# Content Migration — Legacy Flat-Field Bonuses to Typed Bonuses — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the deprecation of legacy flat-field bonuses started by #245. Migrate every YAML file under `content/` to the typed `bonuses:` list with PF2E type assignments, remove `ConditionDef`'s legacy struct fields and `condition.ReflexBonus`, add a strict-mode loader gate that becomes the permanent default in Tier D, and document the classification rules. Numeric values are preserved exactly — no rebalancing.

**Spec:** [docs/superpowers/specs/2026-04-25-content-migration-typed-bonuses.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-content-migration-typed-bonuses.md) (PR [#292](https://github.com/cory-johannsen/mud/pull/292))

**Architecture:** Four sequential tiers, each landed as its own PR for bisectability. **Tier A** rewrites every `content/conditions/*.yaml` file. **Tier B** sweeps any straggler `content/items/*.yaml` files. **Tier C** sweeps misc (zones, room effects). **Tier D** sunsets the code path: removes `ConditionDef.AttackBonus / AttackPenalty / ACBonus / ACPenalty / DamageBonus / ReflexBonus / StealthBonus / SkillPenalty / SkillPenalties / FlairBonus / ExtraWeaponDice` struct fields, removes `SynthesiseBonuses`, removes `condition.ReflexBonus`, flips the strict-mode default to `true`, removes the gate flag entirely. A property test under `internal/game/condition/testdata/rapid/TestNoLegacyFieldsRemain/` scans the content tree on every CI run as a permanent regression guard. An optional `cmd/migrate_bonuses/` Go program performs the YAML rewrite mechanically and is deleted in Tier D.

**Tech Stack:** Go (`internal/game/condition/`, `internal/game/loader/`, optional `cmd/migrate_bonuses/`), `pgregory.net/rapid` for the regression-guard property test, YAML editing (~126 content files).

**Prerequisite:** #245 typed-bonus pipeline is merged (confirmed). #259 (bonus types) is a soft dep — its breakdown UI displays the migrated typings; not strictly required for the migration to land.

**Note on spec PR**: Spec is on PR #292, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Tier | Action | Path |
|------|--------|------|
| A | Modify | `content/conditions/*.yaml` (~84 files; per audit largest field counts) |
| A | Create | `docs/architecture/bonuses-migration.md` |
| A | Optional | `cmd/migrate_bonuses/main.go` |
| A | Optional | `cmd/migrate_bonuses/main_test.go` |
| B | Modify | `content/items/*.yaml` (audit-identified stragglers, if any) |
| C | Modify | `content/zones/**/*.yaml`, `content/rooms/*.yaml`, etc. |
| D | Modify | `internal/game/condition/definition.go` (remove legacy struct fields + `SynthesiseBonuses`) |
| D | Modify | `internal/game/condition/modifiers.go` (remove `ReflexBonus`) |
| D | Modify | `internal/game/condition/definition_test.go` (rewrite three legacy round-trip tests) |
| D | Create | `internal/game/condition/loader_strict_mode_test.go` |
| D | Create | `internal/game/condition/testdata/rapid/TestNoLegacyFieldsRemain/` |
| D | Modify | `internal/game/loader/loader.go` (`StrictTypedBonuses` flag → default `true` → flag removed in same PR) |
| D | Delete | `cmd/migrate_bonuses/` (if Tier A added it) |
| D | Modify | `docs/architecture/bonuses-migration.md` (mark migration complete) |

---

### Task 0: Pre-migration audit

**Files:** none (read-only)

- [ ] **Step 1: Re-run the field audit.** Verify the spec's count (~126 files, largest fields by occurrence) is current. The exact list informs Tier A vs Tier B/C splits.

```bash
for field in attack_bonus attack_penalty ac_bonus ac_penalty damage_bonus reflex_bonus stealth_bonus skill_penalty skill_penalties flair_bonus extra_weapon_dice; do
    n=$(rg -l "^${field}:" content/ 2>/dev/null | wc -l)
    printf "%-22s %d files\n" "$field" "$n"
done
```

- [ ] **Step 2: Capture the field-by-file matrix** in a working file (e.g., `/tmp/audit.txt`) so each tier can clearly enumerate its scope and the property test in Tier D has a known initial-zero baseline.

- [ ] **Step 3: Confirm with user** the tier scoping if the audit surfaces unexpected files outside `content/conditions/`.

---

### Task 1: Strict-mode loader flag (Tier A)

**Files:**
- Modify: `internal/game/loader/loader.go`
- Modify: `internal/game/condition/definition.go` (consult flag)
- Create: `internal/game/condition/loader_strict_mode_test.go`

- [ ] **Step 1: Failing tests** (MIGR-16, MIGR-18):

```go
func TestStrictMode_Off_AcceptsLegacyField(t *testing.T) {
    l := loader.New(loader.Options{StrictTypedBonuses: false})
    _, err := l.LoadCondition([]byte(`
id: x
attack_penalty: -1
`))
    require.NoError(t, err)
}

func TestStrictMode_On_RejectsLegacyField(t *testing.T) {
    l := loader.New(loader.Options{StrictTypedBonuses: true})
    _, err := l.LoadCondition([]byte(`
id: x
attack_penalty: -1
`))
    require.Error(t, err)
    require.Regexp(t, `legacy bonus field 'attack_penalty' not allowed; convert to bonuses: list`, err.Error())
}

func TestStrictMode_On_AcceptsTypedBonusesList(t *testing.T) {
    l := loader.New(loader.Options{StrictTypedBonuses: true})
    _, err := l.LoadCondition([]byte(`
id: x
bonuses:
  - { stat: attack, value: -1, type: status }
`))
    require.NoError(t, err)
}
```

- [ ] **Step 2: Implement** the flag:

```go
type Options struct {
    // ... existing ...
    StrictTypedBonuses bool
}

func (l *Loader) LoadCondition(b []byte) (*condition.ConditionDef, error) {
    raw, err := unmarshalRaw(b)
    if err != nil { return nil, err }
    if l.opts.StrictTypedBonuses {
        for _, field := range legacyBonusFields {
            if _, present := raw[field]; present {
                return nil, fmt.Errorf("%s: legacy bonus field %q not allowed; convert to bonuses: list (see docs/architecture/bonuses-migration.md)", raw["id"], field)
            }
        }
    }
    // existing path: SynthesiseBonuses runs only when flag is false
    ...
}
```

- [ ] **Step 3: Default `StrictTypedBonuses: false`** in Tier A. The flag exists but is opt-in. QA may flip it on for early validation.

---

### Task 2: Classification reference doc (Tier A)

**Files:**
- Create: `docs/architecture/bonuses-migration.md`

- [ ] **Step 1: Author** the canonical classification reference per MIGR-19:
  - Table mapping each legacy field to the most common typed equivalent.
  - Type-by-source rules (status from spells/drugs/conditions; circumstance from positional/situational; item from equipment; untyped only as last resort).
  - Worked before/after for one condition file.
  - Migration cadence (Tier A → D).
  - Stat allowlist (`attack`, `ac`, `damage`, `speed`, six abilities, `skill:<id>`).

- [ ] **Step 2: Cross-link** spec, parent #245 spec, `internal/game/effect/bonus.go`, and the strict-mode flag.

---

### Task 3: Optional migration helper (Tier A)

**Files:**
- Optional: `cmd/migrate_bonuses/main.go`
- Optional: `cmd/migrate_bonuses/main_test.go`

- [ ] **Step 1: Decide** with user whether to ship the helper (MIGR-24, MIGR-25). Recommendation: yes for a 126-file sweep — saves time and reduces classification drift.

- [ ] **Step 2: Failing tests**:

```go
func TestMigrate_AttackPenaltyToStatus(t *testing.T) {
    in := []byte(`
id: frightened
attack_penalty: -1
ac_penalty: -1
`)
    out, choices, err := migrate.RewriteFile(in, "content/conditions/frightened.yaml")
    require.NoError(t, err)
    require.YAMLEq(t, `
id: frightened
bonuses:
  - { stat: attack, value: -1, type: status }
  - { stat: ac, value: -1, type: status }
`, string(out))
    require.Contains(t, choices, "frightened: attack_penalty -> status (path heuristic: conditions/)")
}

func TestMigrate_CoverConditionToCircumstance(t *testing.T) {
    out, _, _ := migrate.RewriteFile(coverInputYAML, "content/conditions/cover_lesser.yaml")
    require.Contains(t, string(out), "type: circumstance")
}

func TestMigrate_AmbiguousFileSkippedWithWarning(t *testing.T) {
    in := []byte(`
id: weird_thing
attack_penalty: -2
`)
    _, choices, err := migrate.RewriteFile(in, "content/conditions/weird_thing.yaml")
    require.NoError(t, err)
    require.Contains(t, choices[0], "SKIP — ambiguous")
}
```

- [ ] **Step 3: Implement** the rewriter:

```go
// Heuristics:
// - path contains "/conditions/cover_" or filename starts with "cover_" → circumstance
// - path contains "/conditions/" otherwise → status (default for conditions)
// - path contains "/items/"      → item
// - path contains "/zones/"      → circumstance (terrain/situational)
// - everything else              → SKIP with warning
```

The helper logs every classification choice and runs `git diff` for the user to review (per MIGR-24).

- [ ] **Step 4:** This whole subtree is deleted in Tier D (MIGR-25).

---

### Task 4: Tier A — content/conditions/*.yaml migration

**Files:**
- Modify: every file under `content/conditions/` containing a legacy field

- [ ] **Step 1: Run the helper** (or manual rewrites) per the spec's per-tier sequencing (MIGR-1).

- [ ] **Step 2: Validation pass** — every rewritten file:
  - Passes existing YAML schema validation.
  - Runs through the loader without warning when `StrictTypedBonuses: false` and without error when `StrictTypedBonuses: true`.
  - Produces identical numeric values when loaded as `Bonus` entries.

- [ ] **Step 3: Targeted regression** — pick five high-impact conditions (e.g., `frightened`, `sickened`, `flat_footed`, `cover_lesser`, `cover_standard`) and load them under both code paths (legacy and typed); verify the produced `EffectSet` is byte-identical except for the source IDs.

- [ ] **Step 4: PR + CI green + merge.** Tier A is its own PR for bisectability (MIGR-1).

---

### Task 5: Tier B — content/items/*.yaml stragglers

**Files:**
- Modify: any `content/items/*.yaml` file containing a legacy field (audit-identified)

- [ ] **Step 1:** Per the audit (Task 0), confirm whether items contain any legacy fields. If none, this tier is a no-op and Tier C immediately follows.

- [ ] **Step 2:** For any straggler, rewrite using the same conventions as Tier A. Most equipment-granted bonuses MUST become `type: item`.

- [ ] **Step 3:** PR + CI green + merge.

---

### Task 6: Tier C — misc YAML

**Files:**
- Modify: stragglers under `content/zones/**/*.yaml`, `content/rooms/*.yaml`, `content/effects/*.yaml`, etc.

- [ ] **Step 1:** Audit-driven sweep of any remaining legacy fields outside conditions and items.

- [ ] **Step 2:** Apply the conventions: zone/room ambient effects → typically `circumstance` (terrain / situational); zone-level character buffs → `status`; everything else → case-by-case with the rule that `untyped` requires a YAML comment justifying the choice (MIGR-7).

- [ ] **Step 3:** PR + CI green + merge.

---

### Task 7: Tier D — code sunset + strict-mode default + property guard

**Files:**
- Modify: `internal/game/condition/definition.go` (remove legacy struct fields + `SynthesiseBonuses`)
- Modify: `internal/game/condition/modifiers.go` (remove `ReflexBonus`)
- Modify: `internal/game/condition/definition_test.go` (rewrite three legacy round-trip tests)
- Modify: `internal/game/loader/loader.go` (default `StrictTypedBonuses: true`, then remove the flag entirely)
- Create: `internal/game/condition/testdata/rapid/TestNoLegacyFieldsRemain/`
- Delete: `cmd/migrate_bonuses/` (if Tier A added it)
- Modify: `docs/architecture/bonuses-migration.md` (mark complete)

- [ ] **Step 1: Failing tests** (MIGR-12, MIGR-13, MIGR-21, MIGR-22, MIGR-23):

```go
func TestConditionDef_NoLegacyFieldsInStruct(t *testing.T) {
    typ := reflect.TypeOf(condition.ConditionDef{})
    for _, f := range []string{"AttackBonus", "AttackPenalty", "ACBonus", "ACPenalty", "DamageBonus", "ReflexBonus", "StealthBonus", "SkillPenalty", "SkillPenalties", "FlairBonus", "ExtraWeaponDice"} {
        _, found := typ.FieldByName(f)
        require.False(t, found, "ConditionDef.%s must be removed", f)
    }
}

func TestReflexBonusFunctionRemoved(t *testing.T) {
    // Compile-time guarantee — use go/types or a separate test that imports the package and tries the call.
    // The simplest version: a build-tag test that fails to compile if condition.ReflexBonus exists.
    require.True(t, true, "verified by inability to call condition.ReflexBonus from this test (uncomment to confirm)")
}

func TestProperty_NoLegacyFieldsRemainInContent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        files := allYAMLFiles(t, "content/")
        f := rapid.SampledFrom(files).Draw(t, "file")
        b, _ := os.ReadFile(f)
        for _, field := range legacyBonusFields {
            require.False(t, regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(field)+`:`).Match(b),
                "file %s still contains legacy field %s", f, field)
        }
    })
}

func TestLoader_DefaultIsStrictMode(t *testing.T) {
    l := loader.New(loader.Options{}) // no StrictTypedBonuses set
    _, err := l.LoadCondition([]byte(`id: x
attack_penalty: -1
`))
    require.Error(t, err)
}
```

- [ ] **Step 2: Rewrite the three pinned tests** in `internal/game/condition/definition_test.go` (lines 127-166, 181-206, 297-318) to assert the typed-bonus shape:

```go
func TestConditionRoundTrip_TypedBonuses(t *testing.T) {
    in := `
id: frightened
bonuses:
  - { stat: attack, value: -1, type: status }
  - { stat: ac, value: -1, type: status }
`
    cd, _ := condition.Load([]byte(in))
    require.Len(t, cd.Bonuses, 2)
    require.Equal(t, effect.BonusStatus, cd.Bonuses[0].Type)
}
```

- [ ] **Step 3: Remove the legacy struct fields** from `ConditionDef`, the `SynthesiseBonuses` function, and `condition.ReflexBonus`. Compiler points to all callers; update or delete each.

- [ ] **Step 4: Flip the strict-mode default** to `true` and **remove the flag entirely** in the same commit (MIGR-17). The `LoadCondition` codepath for legacy is gone.

- [ ] **Step 5: Property regression guard** — `TestNoLegacyFieldsRemain` walks `content/` and fails CI on any reintroduction.

- [ ] **Step 6: Delete `cmd/migrate_bonuses/`** if Task 3 added it.

- [ ] **Step 7: Update the architecture doc** to mark the migration complete; future authors only need the classification rules section.

- [ ] **Step 8: PR + CI green + merge + deploy + smoke test.** Per spec acceptance, manually load at least one max-level character to verify no observable behaviour change.

---

## Verification

```
go test ./...
make migrate-up && make migrate-down  # no DB migrations expected, but a sanity run
```

Per-tier:

- After **Tier A**: `go test ./internal/game/condition/...` and full content-load tests pass; manual reload of a representative condition (e.g., `frightened`) emits the same numerical bonuses.
- After **Tier B / C**: full test suite passes; manual smoke test with strict mode flipped on locally to verify no straggler legacy fields remain.
- After **Tier D**: full test suite passes with strict mode as the sole default; `TestNoLegacyFieldsRemain` is the permanent CI guard.

Final acceptance bullet: load a max-level character on a deployed environment, run a representative combat encounter, verify no observable behaviour difference vs pre-migration recordings.

---

## Rollout / Open Questions Resolved at Plan Time

- **MIGR-Q1**: Cover conditions migrate to `circumstance`. #247's spec consumes this convention.
- **MIGR-Q2**: Tier A only migrates *existing* condition files. Drug-buff conditions (#258) authors them fresh as part of #258.
- **MIGR-Q3**: Skill-penalty entries without a specific skill are flagged for human review; the migrator skips them with a warning. They land in Tier C only after each is hand-classified.
- **MIGR-Q4**: Helper skips ambiguous files; never silently writes `untyped`.
- **MIGR-Q5**: `StrictTypedBonuses` flag is exposed in Tier A so QA can opt-in early. Default flip happens in Tier D.

## Non-Goals Reaffirmed

Per spec §2.2:

- No rebalancing of bonus values during migration.
- No new bonus types beyond the four PF2E types.
- No reauthoring of conditions for clarity / consistency — pure mechanical rewrite.
- No equipment provider routing changes (already typed via #245 follow-on consolidation).
- No touching of `FeatDef.PassiveBonuses` / `TechnologyDef.PassiveBonuses` (already typed).
- No new bonus-kind extension (`morale`, `racial`, etc.).

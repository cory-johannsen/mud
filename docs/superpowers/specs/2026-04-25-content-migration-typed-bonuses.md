---
title: Content Migration — Legacy Flat-Field Bonuses to Typed Bonuses
issue: https://github.com/cory-johannsen/mud/issues/265
date: 2026-04-25
status: spec
prefix: MIGR
depends_on:
  - "#245 Duplicate effects handling (typed-bonus pipeline + DEDUP-11 mixed-mode reject)"
  - "#259 Bonus types (UI breakdown displays migrated typings)"
related: []
---

# Content Migration — Legacy Flat-Field Bonuses to Typed Bonuses

## 1. Summary

Spec #245 shipped the typed-bonus pipeline. The loader keeps a back-compat shim that synthesizes legacy flat fields into untyped `Bonus` entries at load time (`internal/game/condition/definition.go:80-123` `SynthesiseBonuses`). DEDUP-11 forbids mixing legacy and typed forms in the same file. This ticket finishes the deprecation: migrate every legacy-field consumer to the typed `bonuses:` list, drop the synthesis path, and remove vestigial PF2E save fields that Gunchete (six-ability scheme) never consumes.

The audit found:

- **126 distinct YAML files** under `content/` use one or more legacy fields. Largest fields by count: `attack_penalty:` (84), `ac_penalty:` (84), `damage_bonus:` (79), `skill_penalty:` (29), `ac_bonus:` (41), `attack_bonus:` (20).
- **Only `ConditionDef`** carries legacy struct fields (`internal/game/condition/definition.go:23-63`). `FeatDef.PassiveBonuses` and `TechnologyDef.PassiveBonuses` are already typed.
- **`condition.ReflexBonus`** function exists at `modifiers.go:97-109` but is consumed only inside the `condition` package and its tests. `ReflexBonus` is NOT synthesized into the typed `Bonuses` slice today; it stays vestigial.
- **`character.AbilityScores`** confirmed as the six Gunchete stats (Brutality, Grit, Quickness, Reasoning, Savvy, Flair). PF2E saves (Reflex/Fortitude/Will) are not part of the model.
- **No strict-mode toggle** exists — the loader silently synthesizes whatever it sees.
- **Three tests** (`condition/definition_test.go`) directly assert legacy field round-trip behavior and will need migration in lock-step.
- **Equipment bonuses** are already routed through `BuildCombatantEffects` (`combatant_effects.go:36-130`) and the `effect.EffectSet` pipeline. No inline bonus computation lurks in `round.go`. The audit ticket text predates that consolidation; the equipment-provider-audit work is already done.

This spec sequences the migration into safe stages, surfaces the type-classification rules, locks per-file rewrite conventions, and adds the strict-mode loader gate to prevent regression.

## 2. Goals & Non-Goals

### 2.1 Goals

- MIGR-G1: Every YAML file under `content/` that previously used a legacy bonus field uses the `bonuses:` typed list instead.
- MIGR-G2: `condition.ConditionDef`'s legacy fields and `condition.ReflexBonus` function are removed from code; tests updated.
- MIGR-G3: The loader rejects legacy fields with a clear error pointing to the offending file and field name; this enforces the migration as one-way.
- MIGR-G4: Every typed bonus written during the migration carries an explicit PF2E `type` (`status`, `circumstance`, `item`, or `untyped`) per the classification rules in §4.2.
- MIGR-G5: All combat-regression tests pass; no observable behavior change for any character build.
- MIGR-G6: A migration helper script (single-shot Go program) MAY be added under `cmd/migrate_bonuses/` to perform the YAML rewrite mechanically, then is deleted with the ticket. Optional, not required.

### 2.2 Non-Goals

- MIGR-NG1: Rebalancing any bonus value during the migration. Numeric values are preserved exactly.
- MIGR-NG2: Adding new bonus types beyond the four PF2E types.
- MIGR-NG3: Migrating equipment provider routing (already done — see §1).
- MIGR-NG4: Touching `FeatDef.PassiveBonuses` or `TechnologyDef.PassiveBonuses` (already typed).
- MIGR-NG5: Reauthoring conditions for clarity / consistency. Pure mechanical rewrite.
- MIGR-NG6: Adding any new `kind` for bonuses (e.g., `morale`, `racial`). Strict to the PF2E four-type scheme.

## 3. Glossary

- **Legacy field**: any of `attack_bonus`, `attack_penalty`, `ac_bonus`, `ac_penalty`, `damage_bonus`, `reflex_bonus`, `stealth_bonus`, `skill_penalty`, `skill_penalties`, `flair_bonus`, `extra_weapon_dice`.
- **Typed bonus**: a `Bonus` entry in a `bonuses:` list with explicit `stat`, `value`, and `type`.
- **Strict mode**: the loader rejects legacy fields entirely.
- **Migration tier**: a logical batch of files migrated together (conditions → cover → drugs → misc, etc.).

## 4. Requirements

### 4.1 Migration Sequencing

- MIGR-1: Migration MUST proceed in tiers, each landed as its own PR so regressions are bisectable:
  - **Tier A — Conditions.** All `content/conditions/*.yaml` files. Includes the largest body of legacy usage (the 126-file count is dominated by conditions).
  - **Tier B — Equipment.** Any `content/items/*` files using legacy bonus fields (the audit suggests these are absent or rare; verify and either skip the tier or include the stragglers).
  - **Tier C — Misc.** Anything else (zone effects, ambient substances, room effects).
  - **Tier D — Code/loader sunset.** Removes `ConditionDef` legacy struct fields, `SynthesiseBonuses`, and `condition.ReflexBonus`. Adds strict-mode loader gate.
- MIGR-2: After Tier A lands, the strict-mode loader gate MUST NOT be enabled. It is enabled only after Tier C.
- MIGR-3: Each tier MUST run the full test suite (combat regression, condition tests, content load tests) before merging.

### 4.2 Type Classification Rules

When rewriting a flat field as a typed bonus, the rewriter MUST assign one of the four types using these rules:

- MIGR-4: **`status`** — bonuses from spells, drugs, conditions imposed by enemies (frightened, sickened, etc.), boons or buffs that originate from a creature or magical/quasi-magical source. Includes drug buffs (#258). Includes Demoralize / Bon Mot effects (#252).
- MIGR-5: **`circumstance`** — bonuses from positional or situational sources: cover (#247), flanking, terrain, off-guard, hidden, take-cover, aim. Reflects "the situation gives you the bonus" rather than "an effect on you".
- MIGR-6: **`item`** — bonuses granted by carried/worn equipment: weapons, armor, runes/mods (#261), implants, accessories. Always tied to a specific physical item.
- MIGR-7: **`untyped`** — only when no PF2E-defined type fits and the bonus is a free-floating ad-hoc value. Should be RARE; the migrator MUST justify each instance with a YAML comment `# untyped: <reason>`.

### 4.3 YAML Rewrite Convention

- MIGR-8: A flat field rewrites into a single `bonuses:` entry per affected stat. Example:
  ```yaml
  # before
  attack_penalty: -2
  ac_penalty: -2
  # after
  bonuses:
    - { stat: attack, value: -2, type: status }
    - { stat: ac, value: -2, type: status }
  ```
- MIGR-9: When a single legacy field maps to multiple stats (`skill_penalties: { stealth: -2, athletics: -2 }`), each stat becomes its own `bonuses:` entry with `stat: skill:<id>` per the existing `Bonus.Stat` schema.
- MIGR-10: The migrator MUST preserve YAML comments above legacy fields onto the migrated entries (best-effort; comments inside multi-line maps are lost — acceptable).
- MIGR-11: After migration, the file MUST NOT contain any legacy field. DEDUP-11 already enforces non-mixing; this rule formalizes the "remove the old field entirely" half.

### 4.4 Vestigial Fields & Functions

- MIGR-12: `ConditionDef.ReflexBonus` (struct field) MUST be removed in Tier D.
- MIGR-13: `condition.ReflexBonus(set)` (the function in `modifiers.go:97-109`) MUST be removed in Tier D. Tests calling it must be updated or deleted.
- MIGR-14: Any other vestigial save fields surfaced during the audit (Fortitude, Will if discovered) MUST be removed in Tier D.
- MIGR-15: Per-bonus `stat` values MUST resolve to a Gunchete-native stat: `attack`, `ac`, `damage`, `speed`, one of `brutality / grit / quickness / reasoning / savvy / flair`, or `skill:<id>`. The loader MUST reject any `stat:` that does not match this allowlist.

### 4.5 Strict-Mode Loader Gate

- MIGR-16: A new boolean on the loader (`StrictTypedBonuses bool`, default `false`) MUST short-circuit content load with a structured error when any legacy field is encountered.
- MIGR-17: Tier D MUST flip the default to `true` and remove the `false` branch (`SynthesiseBonuses` deletion). The flag is then redundant and MUST be removed entirely; no lingering toggle.
- MIGR-18: Error format: `"<file>: legacy bonus field '<field>' not allowed; convert to bonuses: list (see docs/architecture/bonuses-migration.md)"`.

### 4.6 Documentation

- MIGR-19: A new doc `docs/architecture/bonuses-migration.md` MUST be authored describing the type classification rules (§4.2), the YAML rewrite convention (§4.3), and a worked-example before/after for one file. This is the canonical reference for any future content author wondering "what type goes here?".

### 4.7 Tests

- MIGR-20: Existing combat-regression tests MUST pass after every tier.
- MIGR-21: The three legacy-field round-trip tests at `internal/game/condition/definition_test.go` (lines 127-166, 181-206, 297-318) MUST be rewritten in Tier D to assert the typed-bonus shape instead.
- MIGR-22: A new test `loader_strict_mode_test.go` MUST verify that strict mode rejects a file with a legacy field and accepts one without.
- MIGR-23: A property test under `internal/game/condition/testdata/rapid/TestNoLegacyFieldsRemain/` MUST scan every YAML file under `content/` and assert no legacy field survives. Becomes a permanent regression guard.

### 4.8 Migration Helper (Optional)

- MIGR-24: A one-shot Go program at `cmd/migrate_bonuses/main.go` MAY be added to perform the YAML rewrite mechanically. Contract:
  - Walks `content/` for files containing any legacy field.
  - For each file, applies the type classification per §4.2 using a heuristic table (file-path prefix → suggested type — e.g., `content/conditions/cover_*.yaml` → circumstance).
  - Writes the rewritten file in place, then runs `git diff` for the user to review.
  - Logs every classification choice with the reasoning.
- MIGR-25: When the migration is complete, the helper MUST be deleted in the same PR that ships Tier D.

## 5. Architecture

### 5.1 Where the changes land

```
content/conditions/*.yaml             # Tier A: rewrite to typed bonuses
content/items/*.yaml                  # Tier B: any remaining stragglers
content/{zones,etc}/*.yaml            # Tier C: misc

internal/game/condition/
  definition.go                       # Tier D: remove legacy struct fields, SynthesiseBonuses
  modifiers.go                        # Tier D: remove ReflexBonus function
  definition_test.go                  # Tier D: update round-trip tests
  loader_strict_mode_test.go          # Tier D: strict-mode tests
  testdata/rapid/TestNoLegacyFieldsRemain/   # Tier D

internal/game/{loader}/
  loader.go                           # Tier D: StrictTypedBonuses flag → permanent

cmd/migrate_bonuses/                  # optional helper (added Tier A, removed Tier D)

docs/architecture/bonuses-migration.md  # Tier A: classification reference
```

### 5.2 Migration cadence

```
Tier A: conditions  →   PR   → CI green → merge → deploy + smoke test
Tier B: equipment   →   PR   → CI green → merge
Tier C: misc        →   PR   → CI green → merge
Tier D: sunset      →   PR   → CI green → merge → docs updated
```

### 5.3 Single sources of truth

- Classification rules: `docs/architecture/bonuses-migration.md` only.
- Bonus typed schema: `internal/game/effect/bonus.go` only.
- Strict-mode enforcement: the loader's typed validation (no fallback paths after Tier D).

## 6. Open Questions

- MIGR-Q1: Cover spec #247 has not yet implemented its conditions; cover-related condition rewrites in Tier A may anticipate the cover spec's authoring choices. Recommendation: rewrite cover conditions to `circumstance` per the spec contract (#247 will consume them); the cover spec acknowledges the convention.
- MIGR-Q2: Drug buffs (#258) ship after Tier A. Should Tier A pre-author the seven canonical condition files used by drugs, or does #258 add them? Recommendation: #258 adds them; Tier A only migrates *existing* files.
- MIGR-Q3: Some legacy `skill_penalty` entries don't specify which skill (`skill_penalty: -2`). What stat does that map to? Recommendation: investigate each — most are likely "all skills" which Gunchete does not natively support; rewrite as a comment-flagged `untyped` on a generic `skill:any` stat (extending the schema if needed) and capture the actual intent in the audit notes.
- MIGR-Q4: When the migration helper (MIGR-24) cannot confidently classify a file, should it skip with a warning or use `untyped`? Recommendation: skip with a warning so a human reviews; do not silently write `untyped` on ambiguous content.
- MIGR-Q5: Should the per-tier PRs include the strict-mode flag flip immediately (with the flag default remaining `false`) so QA can opt-in early? Recommendation: yes — flag is harmless until the default flips in Tier D.

## 7. Acceptance

- [ ] Every YAML file under `content/` containing a legacy bonus field has been rewritten to use `bonuses:` typed entries.
- [ ] `ConditionDef` no longer carries the legacy struct fields; `condition.ReflexBonus` function deleted.
- [ ] Loader rejects legacy fields with the prescribed error format.
- [ ] All combat regression tests, condition tests, and content load tests pass.
- [ ] `docs/architecture/bonuses-migration.md` exists and documents the classification rules.
- [ ] The "no legacy fields remain" property test passes against the entire content tree.
- [ ] No observable in-game behavior change for any pre-existing character build (manual smoke test on at least one max-level character).

## 8. Out-of-Scope Follow-Ons

- MIGR-F1: Rebalancing bonus values now that types are explicit.
- MIGR-F2: Adding non-PF2E-canonical bonus types if a future system warrants them (e.g., a `morale` type).
- MIGR-F3: A linting command in CI that scans `content/` for legacy fields on every PR (the property test in MIGR-23 covers this; F3 is a CI ergonomics improvement only).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/265
- Predecessor spec: `docs/superpowers/specs/2026-04-21-duplicate-effects-handling.md`
- Sibling spec: `docs/superpowers/specs/2026-04-25-bonus-types.md`
- Synthesis function (to be removed): `internal/game/condition/definition.go:80-123`
- Vestigial Reflex function (to be removed): `internal/game/condition/modifiers.go:97-109`
- ConditionDef legacy fields (to be removed): `internal/game/condition/definition.go:23-63`
- Equipment bonus pipeline (already typed): `internal/game/combat/combatant_effects.go:36-130`
- Bonus model: `internal/game/effect/bonus.go`
- Gunchete ability scores: `internal/game/character/model.go` (`AbilityScores`)
- Tests requiring update in Tier D: `internal/game/condition/definition_test.go` (lines 127-166, 181-206, 297-318)

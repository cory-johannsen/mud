# PF2E Complete Spell Import — Ranks 2–10 Design

**Date:** 2026-03-19
**Sub-project:** PF2E Import — Ranks 2–10 + Archetype Slot Progression

---

## Goal

Import all PF2E spell ranks 2–10 across all four Gunchete technology traditions, localize them into the Gunchete cyberpunk aesthetic, extend archetype `level_up_grants` to job level 20 with correct PF2E slot deltas, populate each archetype's per-rank pool exhaustively from imported content, and bake all localizations into `static_localizer.go` for idempotent re-import.

---

## Context

- Rank-1 spells are already imported (~487 files across 4 traditions).
- Archetype `level_up_grants` currently only extend to job level 5, and levels 3–5 contain incorrect slot deltas (`{1:1}` at every level instead of new-rank unlocks).
- Job level cap is 20. Character level cap is 100. Max job level per job is 20.
- Cantrips map to hardwired technologies (unlimited-use, fixed per job) — out of scope.
- Job-level pool additions are out of scope for this pass; archetype pools only.
- `vendor/pf2e-data` is the canonical PF2E data source (SYSREQ-5/6).
- Localization is performed in-session (no external API key needed) and baked into `internal/importer/static_localizer.go`.

### Spell Rank → Job Level Unlock

| Tech Rank | Job Level (Full Casters) | Job Level (Drifter) |
|-----------|--------------------------|---------------------|
| 1         | 1                        | 1                   |
| 2         | 3                        | 3                   |
| 3         | 5                        | 5                   |
| 4         | 7                        | 9                   |
| 5         | 9                        | 11                  |
| 6         | 11                       | 13                  |
| 7         | 13                       | 17                  |
| 8         | 15                       | 19                  |
| 9         | 17                       | —                   |
| 10        | 19                       | —                   |

---

## Requirements

### REQ-IMP1
The importer MUST be run once per rank (ranks 2–10) against `vendor/pf2e-data/packs/pf2e/spells/spells/rank-{N}` with no `-localize` flag, producing raw PF2E YAML files in `content/technologies/<tradition>/`.

### REQ-IMP2
All imported files MUST be localized in-session into the Gunchete cyberpunk aesthetic (hardware/EM-device/chemistry flavor), preserving all mechanical text (dice expressions, ranges, durations, save types, action costs) exactly.

### REQ-IMP3
After localization, `internal/importer/static_localizer.go` MUST be regenerated from all YAML files in `content/technologies/` to include all new entries, ensuring idempotent re-import.

### REQ-SLT1
The `level_up_grants` in all 6 tech-enabled archetype YAMLs MUST be corrected at levels 3–5 and extended through level 20, following the PF2E full-caster or half-caster slot delta tables exactly.

### REQ-SLT2
Full prepared casters (schemer, naturalist, zealot, nerd) MUST use `slots_by_level` deltas per rank per job level as follows:
- Level 3: `{2:2}` | Level 4: `{2:1}` | Level 5: `{3:2}` | Level 6: `{3:1}`
- Level 7: `{4:2}` | Level 8: `{4:1}` | Level 9: `{5:2}` | Level 10: `{5:1}`
- Level 11: `{6:2}` | Level 12: `{6:1}` | Level 13: `{7:2}` | Level 14: `{7:1}`
- Level 15: `{8:2}` | Level 16: `{8:1}` | Level 17: `{9:2}` | Level 18: `{9:1}`
- Level 19: `{10:2}` | Level 20: `{10:1}`

### REQ-SLT3
The spontaneous full caster (influencer) MUST use `known_by_level` / `uses_by_level` deltas:
- Level 3: `known:{2:2} uses:{2:2}` | Level 4: `uses:{2:1}` | Level 5: `known:{3:2} uses:{3:2}` | Level 6: `uses:{3:1}`
- Level 7: `known:{4:2} uses:{4:2}` | Level 8: `uses:{4:1}` | Level 9: `known:{5:2} uses:{5:2}` | Level 10: `uses:{5:1}`
- Level 11: `known:{6:2} uses:{6:2}` | Level 12: `uses:{6:1}` | Level 13: `known:{7:2} uses:{7:2}` | Level 14: `uses:{7:1}`
- Level 15: `known:{8:2} uses:{8:2}` | Level 16: `uses:{8:1}` | Level 17: `known:{9:2} uses:{9:2}` | Level 18: `uses:{9:1}`
- Level 19: `known:{10:2} uses:{10:2}` | Level 20: `uses:{10:1}`

### REQ-SLT4
The half-caster (drifter, bio_synthetic, prepared) MUST use 1 slot per new rank with a 4-job-level gap between tiers:
- Level 3: `{2:1}` | Level 5: `{3:1}` | Level 9: `{4:1}` | Level 11: `{5:1}` | Level 13: `{6:1}` | Level 17: `{7:1}` | Level 19: `{8:1}`
- Ranks 9 and 10 are NOT granted to drifter.

### REQ-POOL1
At each job level where a new rank first unlocks, the archetype YAML MUST include an exhaustive `pool:` list containing every imported technology of the matching tradition at that rank. Pool entries use the format `{ id: <tech_id>, level: <rank> }`.

### REQ-POOL2
Pool entries MUST NOT duplicate IDs already present in `technology_grants.prepared.pool` or `technology_grants.spontaneous.pool` (the base level-1 pools defined in the archetype).

### REQ-POOL3
Pool entries for even job levels (which add slots but do not unlock a new rank) MUST NOT include a `pool:` block — the player selects from the pool defined at the previous odd level.

### REQ-TEST1
`go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v` MUST pass after all archetype YAML changes are applied.

### REQ-DOC1
`docs/requirements/pf2e-import-reference.md` MUST be updated with a "Batch Import 2026-03-19" section containing per-tradition file counts for each rank imported.

### REQ-DOC2
`docs/features/technology.md` MUST have the "Spell import from PF2E with translation into Gunchete" checkbox marked complete after all work is done.

---

## Architecture

### Import + Localize Pipeline (per rank)

```
vendor/pf2e-data/spells/rank-N/  →  import-content (noop)  →  content/technologies/<tradition>/
                                                                        ↓
                                                              in-session Python localization
                                                                        ↓
                                                              localized YAML committed
```

### Archetype YAML Structure (per rank unlock level)

**Prepared full caster example (schemer, job level 3, rank-2 unlock):**
```yaml
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 2
      pool:
        - { id: fear_neural_2, level: 2 }   # all rank-2 neural techs
        ...
  4:
    prepared:
      slots_by_level:
        2: 1
  # no pool at level 4 — player picks from the level-3 pool
```

**Spontaneous caster example (influencer, job level 3, rank-2 unlock):**
```yaml
level_up_grants:
  3:
    spontaneous:
      known_by_level:
        2: 2
      uses_by_level:
        2: 2
      pool:
        - { id: fear_neural_2, level: 2 }
        ...
  4:
    spontaneous:
      uses_by_level:
        2: 1
```

**Half-caster example (drifter, job level 3, rank-2 unlock):**
```yaml
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool:
        - { id: X_bio_synthetic, level: 2 }
        ...
```

### Pool Generation Script

A Python script reads all `content/technologies/<tradition>/*.yaml` files, groups them by `level`, and for each archetype generates the `pool:` block to be inserted into `level_up_grants`. The script outputs a YAML fragment that can be applied directly.

### StaticLocalizer Regeneration

After all localization passes, re-run `/tmp/gen_static_localizer.py` to regenerate `internal/importer/static_localizer.go` with all entries across all traditions and ranks.

---

## Out of Scope

- Cantrip import (hardwired techs, handled per-job separately)
- Job-level pool additions for ranks 2–10 (future pass)
- Amped/heightened spell mechanics
- Ranks beyond 10

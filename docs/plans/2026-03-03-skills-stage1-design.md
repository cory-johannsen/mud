# Skills System — Stage 1 Design

**Date:** 2026-03-03
**Scope:** Infrastructure + skills only. Feats and abilities follow in later stages.

---

## Skill List

17 Gunchete-native skills mapped 1:1 from P2FE. Grit (Con) has no associated skills, matching P2FE.

| ID | Name | Ability | P2FE Equivalent |
|----|------|---------|-----------------|
| parkour | Parkour | quickness | Acrobatics |
| ghosting | Ghosting | quickness | Stealth |
| grift | Grift | quickness | Thievery |
| muscle | Muscle | brutality | Athletics |
| tech_lore | Tech Lore | reasoning | Arcana |
| rigging | Rigging | reasoning | Crafting |
| conspiracy | Conspiracy | reasoning | Occultism |
| factions | Factions | reasoning | Society |
| intel | Intel | reasoning | Lore |
| patch_job | Patch Job | savvy | Medicine |
| wasteland | Wasteland | savvy | Nature |
| gang_codes | Gang Codes | savvy | Religion |
| scavenging | Scavenging | savvy | Survival |
| hustle | Hustle | flair | Deception |
| smooth_talk | Smooth Talk | flair | Diplomacy |
| hard_look | Hard Look | flair | Intimidation |
| rep | Rep | flair | Performance |

---

## Data Model

### `content/skills.yaml`

Single master file defining all 17 skills:

```yaml
skills:
  - id: parkour
    name: Parkour
    ability: quickness
    pf2e: acrobatics
    description: "Movement through ruins, vaults, chases, and tight spaces."
  # ... all 17
```

### Job YAML Extension

Each of the 76 job YAMLs gains a `skills` block:

```yaml
skills:
  fixed:
    - muscle
    - patch_job
  choices:
    pool: [hustle, grift, ghosting, smooth_talk]
    count: 2
```

`fixed` skills are always trained. `choices` defines a pool and how many the player picks. Either field may be omitted if the job has no fixed or choice skills.

### DB Schema

```sql
CREATE TABLE character_skills (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id     TEXT NOT NULL,
    proficiency  TEXT NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, skill_id)
);
```

Proficiency ranks: `untrained`, `trained`, `expert`, `master`, `legendary` (P2FE standard). Stage 1 stores the rank only; bonus calculation (`level + rank_bonus + ability_mod`) is deferred to the skill checks feature.

### Character Model Extension

```go
// in internal/game/character/model.go
Skills map[string]string // skill_id → proficiency rank
```

---

## Character Creation & Backfill

### Creation Flow

After job selection, if the job has `skills.choices`, the builder inserts a sequential skill selection step:

```
You know the following skills:
  Muscle (trained), Patch Job (trained)

Choose 2 additional skills (2 remaining):
  1. Hustle
  2. Grift
  3. Ghosting
  4. Smooth Talk

> 1

Choose 2 additional skills (1 remaining):
  2. Grift
  3. Ghosting
  4. Smooth Talk

> 3

Selected: Hustle, Ghosting
```

- One selection per prompt
- Selected options are removed from the list on subsequent prompts
- Counter decrements each round
- All 17 skills are written to `character_skills` at finalization (selected/fixed as `trained`, all others as `untrained`)

### Backfill for Existing Characters

On first login after deploy, if a character has zero rows in `character_skills`:

1. Fixed skills are auto-assigned as `trained`
2. If the job has `choices`, the player is prompted using the same sequential UI before entering the world
3. All remaining skills written as `untrained`

Backfill runs in the session handler (not as a DB migration), so existing character rows are never touched.

---

## `skills` Command

Displays all 17 skills grouped by ability score. Trained skills are highlighted in cyan; untrained are dim. No bonus shown in Stage 1.

```
=== Skills ===

Quickness:
  Parkour        untrained
  Ghosting       trained
  Grift          untrained

Brutality:
  Muscle         trained

Reasoning:
  Tech Lore      untrained
  Rigging        untrained
  Conspiracy     untrained
  Factions       untrained
  Intel          untrained

Savvy:
  Patch Job      trained
  Wasteland      untrained
  Gang Codes     untrained
  Scavenging     untrained

Flair:
  Hustle         trained
  Smooth Talk    untrained
  Hard Look      untrained
  Rep            untrained
```

Follows CMD-1 through CMD-7: proto message, bridge handler, gRPC dispatch, full TDD coverage.

---

## Files Changed

| File | Change |
|------|--------|
| `content/skills.yaml` | New — 17 skill definitions |
| `content/jobs/*.yaml` | All 76 — add `skills` block |
| `internal/game/ruleset/skill.go` | New — `Skill` type, `LoadSkills()` |
| `internal/game/ruleset/job.go` | Extend `Job` struct with `SkillGrants` |
| `internal/game/character/model.go` | Add `Skills map[string]string` |
| `internal/game/character/builder.go` | Add sequential skill selection step |
| `internal/storage/postgres/` | `character_skills` table + CRUD |
| `internal/game/command/skills.go` | New — `HandleSkills` |
| `internal/game/command/commands.go` | Register `HandlerSkills` |
| `api/proto/game/v1/game.proto` | Add `SkillsRequest` / `SkillsResponse` |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeSkills` |
| `internal/gameserver/grpc_service.go` | Add `handleSkills` dispatch |

---

## Out of Scope for Stage 1

- Feats (Stage 2)
- Abilities / class features (Stage 3)
- Skill check mechanics (future)
- Skill increases on level-up (future)
- `feats` and `abilities` commands (future stages)

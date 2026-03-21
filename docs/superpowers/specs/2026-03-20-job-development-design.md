# Job Development — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `job-development` (priority 270)
**Dependencies:** `non-combat-npcs`

---

## Overview

Extends the job system with three additions:

1. **Advancement hierarchy** — Basic (tier 1) → Specialist (tier 2) → Expert (tier 3). Advancement requires meeting level, feat, skill, and credit prerequisites, performed at a `job_trainer` NPC.
2. **Drawbacks** — each job definition carries 1–3 drawbacks (passive or situational penalties active while the job is held).
3. **Multi-job** — players may hold any number of jobs simultaneously provided they meet each job's prerequisites. All benefits (attribute boosts, feats, proficiencies, skills) are cumulative across all held jobs.

---

## 1. Job YAML Schema Extensions

### 1.1 New Fields

Every job YAML definition gains:

```yaml
tier: 1                          # 1=Basic, 2=Specialist, 3=Expert (required; fatal load error if absent)

advancement_requirements:        # only relevant when this job is a Specialist or Expert target
  min_level: 10                  # default 10 for Basic→Specialist; 15 for Specialist→Expert; overridable per job
  required_feats:                # feats the player must hold (by feat ID)
    - feat_id_1
    - feat_id_2
  required_skill_ranks:          # minimum skill rank per skill ID ("untrained"|"trained"|"expert"|"master"|"legendary")
    rigging: trained
    scavenging: expert
  prerequisite_jobs:             # jobs the player must already hold (by job ID); equivalent to RequiredJobs in non-combat-npcs spec
    - basic_job_id

drawbacks:
  - id: drawback_unique_id
    type: passive                # "passive" | "situational"
    description: "..."           # player-facing description
    # passive fields:
    condition_id: fatigued       # condition applied at login and reapplied on job-change; omit for non-condition passives
    stat_modifier:               # optional stat penalty (alternative to condition_id)
      stat: strength
      amount: -1
    # situational fields (only for type: situational):
    trigger: on_leave_combat_without_kill  # trigger ID (see Section 3.2)
    effect_condition_id: demoralized       # condition applied when trigger fires
    duration: "1h"                         # Go time.Duration string; defaults to "1h"
```

Default advancement thresholds (used when `advancement_requirements.min_level` is absent):
- Basic → Specialist: level 10
- Specialist → Expert: level 15

- REQ-JD-1: All job YAML definitions MUST include `tier int`. Absence MUST be a fatal load error at startup.
- REQ-JD-2: `advancement_requirements.min_level` defaults to 10 for tier 2 jobs and 15 for tier 3 jobs if not specified.

### 1.2 Existing Non-Combat-NPC Integration

The `non-combat-npcs` spec already defines `JobTrainerConfig.TrainableJob` with `Prerequisites` (including `MinLevel`, `MinJobLevel`, `MinSkillRanks`, `RequiredJobs`, `MinAttributes`). The `train <job>` command (from non-combat-npcs spec) validates these prerequisites and deducts the training cost.

This spec extends the prerequisite validation in `train <job>` to also check `advancement_requirements.required_feats` from the target job's YAML definition. The `JobPrerequisites` struct gains:

```go
RequiredFeats []string `yaml:"required_feats,omitempty"` // feat IDs the player must hold
```

- REQ-JD-3: `train <specialist/expert job>` MUST validate `advancement_requirements.required_feats` from the target job's YAML in addition to the `JobTrainerConfig.TrainableJob.Prerequisites` already checked.

---

## 2. Multi-Job Data Model

### 2.1 Player Job List

Players hold a list of active jobs. The existing `characters` table stores the active job. This spec extends it:

```sql
character_jobs (
    character_id  bigint NOT NULL REFERENCES characters(id),
    job_id        text   NOT NULL,
    PRIMARY KEY (character_id, job_id)
)
```

`PlayerSession` gains:

```go
HeldJobs    []string  // all job IDs the player currently holds; loaded at login
ActiveJobID string    // the job displayed on character sheet and used for tier/level-based calculations
```

The existing `characters.job` column remains as the active job ID for backward compatibility. `character_jobs` stores the full list.

- REQ-JD-4: Players MUST be able to hold any number of jobs simultaneously, provided they meet each job's prerequisites at train time.
- REQ-JD-5: `setjob <job>` (existing command from non-combat-npcs spec) MUST update `characters.job` (active job) to the specified held job.

### 2.2 Cumulative Benefits

All attribute boosts, feats, proficiencies, and skills from all held jobs are cumulative. At login (and on any job-change event), the character's derived stats are recomputed by aggregating across all held jobs:

1. Sum all attribute boosts from all held jobs (Basic + all Specialist + all Expert jobs held).
2. Union all feats from all held jobs.
3. Union all proficiencies from all held jobs (highest rank wins for overlapping proficiencies).
4. Union all skills from all held jobs.
5. Apply all drawback passive conditions/modifiers from all held jobs.

- REQ-JD-6: Character derived stats MUST be recomputed from all held jobs whenever `HeldJobs` changes (login, `train`, `setjob`).
- REQ-JD-7: For overlapping proficiencies across jobs, the highest rank MUST be used. For overlapping skills (same skill ID in multiple jobs), take the union: if both jobs grant a skill, the player receives the rank from whichever job grants the higher rank. Numeric skill bonuses from different jobs sum together.

---

## 3. Drawbacks

### 3.1 Passive Drawbacks

Passive drawbacks are always active while the job is held. They apply as either:
- A persistent condition (e.g., `condition_id: light_sensitive`) — tagged with source `drawback:<job_id>` and applied at login. Passive drawback conditions are **permanent** while the job is held; they cannot be cleared by normal condition-clearing means (rest, medicine, etc.). They are removed only when the job is dropped via `removejob` (future admin command) or replaced. This avoids reapplication complexity.
- A stat modifier (e.g., `stat: strength, amount: -1`) — included in `ComputeDerivedStats` as a persistent negative modifier.

Passive drawbacks from multiple held jobs stack. If two jobs share the same `condition_id`, it is applied once (deduplicated by condition ID at application time).

- REQ-JD-8: Passive drawback conditions MUST be applied at login with source tag `drawback:<job_id>`. They MUST NOT be clearable by normal condition-clearing mechanisms. They are removed only when the job is removed from the player's held jobs.
- REQ-JD-9: Passive drawback stat modifiers MUST be included in `ComputeDerivedStats` for as long as the job is held.

### 3.2 Situational Drawbacks

Situational drawbacks register a trigger and apply a condition when the trigger fires. Supported trigger IDs:

| Trigger ID | Fires when |
|---|---|
| `on_leave_combat_without_kill` | Combat ends and this player dealt 0 killing blows |
| `on_take_damage_in_one_hit_above_threshold` | A single hit deals ≥ 50% of max HP |
| `on_fail_skill_check` | Any skill check results in failure or critical failure |
| `on_enter_room_danger_level` | Player enters a room with danger level ≥ 3 |

When a situational trigger fires, the `effect_condition_id` condition is applied to the player for a defined duration (specified in the drawback definition as `duration: "1 hour"` in in-game time, i.e., 1 real minute).

New optional field on drawback definition:
```yaml
duration: "2h"  # Go time.Duration string (e.g. "30m", "1h", "2h"); in-game hours; defaults to "1h" if absent
```

Duration is parsed using Go's `time.ParseDuration`. Invalid duration strings MUST be a fatal load error at startup. Situational drawback conditions are applied via the existing condition system's TTL mechanism — they expire automatically when the duration elapses (same as any other timed condition). The condition is tagged with source `drawback:<job_id>` to allow identification.

- REQ-JD-10: Situational drawback triggers MUST be evaluated at the specified fire points in the game loop.
- REQ-JD-11: Situational drawback conditions MUST be applied with the duration parsed by `time.ParseDuration`. Invalid duration strings MUST be fatal at startup. Condition expiry MUST use the existing condition TTL mechanism.

---

## 4. Advancement Flow

The full advancement flow via `train <specialist/expert job>` at a `job_trainer` NPC:

1. Verify player is in the same room as a `job_trainer` NPC that offers the target job.
2. Check all prerequisites:
   a. `min_level` — player's overall level (not per-job level).
   b. `required_feats` — player holds all listed feat IDs.
   c. `required_skill_ranks` — player meets all listed minimum skill ranks.
   d. `prerequisite_jobs` — player currently holds all listed job IDs.
   e. `min_job_level` (from `JobTrainerConfig`) — player meets minimum level in prerequisite jobs.
   f. `min_attributes` (from `JobTrainerConfig`) — player meets minimum attribute scores.
3. Check player has sufficient credits for `TrainingCost`.
4. If any prerequisite unmet: fail with a message naming the specific unmet requirement.
5. Deduct credits. Add job to `character_jobs`. Recompute derived stats.
6. Notify player: "Training complete. You are now a <job name>."

- REQ-JD-12: `train` MUST fail with a specific unmet-requirement message for each prerequisite check.
- REQ-JD-13: Job addition MUST be atomic: credits deducted and `character_jobs` row inserted in a single DB transaction.

---

## 5. Architecture

### 5.1 Job Registry Extension

`JobRegistry` (or equivalent) loads all job YAML files at startup. After this spec:
- Validates `tier` field present (fatal if absent).
- Loads `advancement_requirements` and `drawbacks` into `JobDef`.

`JobDef` struct gains:

```go
type JobDef struct {
    // existing fields...
    Tier                    int                     `yaml:"tier"`
    AdvancementRequirements AdvancementRequirements `yaml:"advancement_requirements,omitempty"`
    Drawbacks               []DrawbackDef           `yaml:"drawbacks,omitempty"`
}

type AdvancementRequirements struct {
    MinLevel          int               `yaml:"min_level,omitempty"`
    RequiredFeats     []string          `yaml:"required_feats,omitempty"`
    RequiredSkillRanks map[string]string `yaml:"required_skill_ranks,omitempty"`
    PrerequisiteJobs  []string          `yaml:"prerequisite_jobs,omitempty"`
}

type DrawbackDef struct {
    ID              string `yaml:"id"`
    Type            string `yaml:"type"` // "passive" | "situational"
    Description     string `yaml:"description"`
    ConditionID     string `yaml:"condition_id,omitempty"`
    StatModifier    *StatModifier `yaml:"stat_modifier,omitempty"`
    Trigger         string `yaml:"trigger,omitempty"`
    EffectConditionID string `yaml:"effect_condition_id,omitempty"`
    Duration        string `yaml:"duration,omitempty"` // in-game hours; default "1 hour"
}

type StatModifier struct {
    Stat   string `yaml:"stat"`
    Amount int    `yaml:"amount"`
}
```

### 5.2 Character Jobs Repository

New `CharacterJobsRepository` in `internal/storage/postgres/`:

```go
AddJob(ctx context.Context, characterID int64, jobID string) error
RemoveJob(ctx context.Context, characterID int64, jobID string) error
ListJobs(ctx context.Context, characterID int64) ([]string, error)
```

`AddJob` and the credit deduction are wrapped in a single DB transaction by the `train` handler.

### 5.3 Derived Stats Recomputation

A `ComputeDerivedStats(character, heldJobs []JobDef) DerivedStats` function aggregates all attribute boosts, feats, proficiencies, skills, and drawback passive modifiers from all held jobs. Called at login and on any job-change event.

- REQ-JD-14: `ComputeDerivedStats` MUST be a pure function with no side effects; all mutation happens in the caller.

### 5.4 Drawback Engine

A `DrawbackEngine` registers situational drawback triggers at login for all held jobs. Trigger fire points:

- `on_leave_combat_without_kill`: end-of-combat handler
- `on_take_damage_in_one_hit_above_threshold`: damage application handler (after damage is applied)
- `on_fail_skill_check`: skill check resolution handler
- `on_enter_room_danger_level`: room-entry handler

On trigger: apply `effect_condition_id` with the specified duration via the existing condition system.

### 5.5 Non-Combat-NPCs Spec Update

`JobPrerequisites` struct in the non-combat-npcs spec gains `RequiredFeats []string`. The `train` handler adds one new validation step (feat check) after all existing prerequisite checks.

---

## 6. Requirements Summary

- REQ-JD-1: All job YAML definitions MUST include `tier int`. Absence MUST be a fatal load error.
- REQ-JD-2: `advancement_requirements.min_level` defaults to 10 (Basic→Specialist) or 15 (Specialist→Expert) if absent.
- REQ-JD-3: `train` MUST validate `advancement_requirements.required_feats` in addition to existing prerequisites.
- REQ-JD-4: Players MUST be able to hold any number of jobs simultaneously.
- REQ-JD-5: `setjob` MUST update the active job ID on `characters.job`.
- REQ-JD-6: Derived stats MUST be recomputed from all held jobs on login and on any job-change.
- REQ-JD-7: For overlapping proficiencies, the highest rank MUST be used. For overlapping skills, take highest rank; numeric bonuses sum.
- REQ-JD-8: Passive drawback conditions MUST be applied at login with source tag `drawback:<job_id>`. They MUST NOT be clearable by normal condition-clearing mechanisms.
- REQ-JD-9: Passive drawback stat modifiers MUST be included in `ComputeDerivedStats` while the job is held.
- REQ-JD-10: Situational drawback triggers MUST be evaluated at the specified fire points in the game loop.
- REQ-JD-11: Situational drawback conditions MUST be applied via the existing condition TTL mechanism. Duration MUST be parsed by `time.ParseDuration`; invalid strings MUST be a fatal load error.
- REQ-JD-12: `train` MUST fail with a specific unmet-requirement message for each unmet prerequisite.
- REQ-JD-13: Credit deduction and job addition MUST be atomic in a single DB transaction.
- REQ-JD-14: `ComputeDerivedStats` MUST be a pure function.

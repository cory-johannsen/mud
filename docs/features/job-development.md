# Job Development

Advancement hierarchy (Basic→Specialist→Expert), job drawbacks (passive and situational), and multi-job support. Extends the `non-combat-npcs` `train` command. See `docs/superpowers/specs/2026-03-20-job-development-design.md` for full design spec.

## Requirements

### Job YAML Schema

- [ ] REQ-JD-1: All job YAML definitions MUST include `tier int` (1=Basic, 2=Specialist, 3=Expert); absence is a fatal load error
- [ ] REQ-JD-2: `advancement_requirements.min_level` defaults to 10 (Basic→Specialist) or 15 (Specialist→Expert) if absent
- [ ] `advancement_requirements`: `min_level`, `required_feats []string`, `required_skill_ranks map[string]string`, `prerequisite_jobs []string`
- [ ] `drawbacks []DrawbackDef`: each with `id`, `type` (passive|situational), `description`, `condition_id`/`stat_modifier` (passive), `trigger`/`effect_condition_id`/`duration` (situational)
- [ ] REQ-JD-11: `duration` on drawbacks uses Go `time.ParseDuration` format (e.g. `"1h"`); invalid strings are fatal at startup

### Multi-Job Data Model

- [ ] `character_jobs` table: `character_id bigint, job_id text, PRIMARY KEY (character_id, job_id)`
- [ ] `PlayerSession.HeldJobs []string` — all held job IDs, loaded at login
- [ ] `PlayerSession.ActiveJobID string` — active job for display; updated by `setjob`
- [ ] REQ-JD-4: Players may hold any number of jobs simultaneously
- [ ] REQ-JD-5: `setjob` updates `characters.job` (active job ID)

### Cumulative Benefits

- [ ] REQ-JD-6: Derived stats recomputed from ALL held jobs on login and on any job-change
- [ ] REQ-JD-7: Overlapping proficiencies → highest rank wins; overlapping skills → union of highest ranks + sum numeric bonuses
- [ ] REQ-JD-14: `ComputeDerivedStats(character, heldJobs []JobDef) DerivedStats` is a pure function

### Drawbacks

- [ ] REQ-JD-8: Passive drawback conditions applied at login with source tag `drawback:<job_id>`; NOT clearable by normal condition-clearing; removed only when job is dropped
- [ ] REQ-JD-9: Passive drawback stat modifiers included in `ComputeDerivedStats` while job is held
- [ ] REQ-JD-10: Situational drawback triggers evaluated at fire points: end-of-combat, damage-application, skill-check-resolution, room-entry
- [ ] REQ-JD-11: Situational conditions applied via existing condition TTL mechanism; expire automatically

### Advancement Flow

- [ ] REQ-JD-3: `train <specialist/expert job>` validates `advancement_requirements.required_feats` in addition to existing `JobPrerequisites` checks
- [ ] `JobPrerequisites` struct gains `RequiredFeats []string`
- [ ] REQ-JD-12: `train` fails with specific unmet-requirement message per failed check
- [ ] REQ-JD-13: Credit deduction + `character_jobs` insert are atomic in a single DB transaction

### Architecture

- [ ] `CharacterJobsRepository`: `AddJob`, `RemoveJob`, `ListJobs`
- [ ] `JobDef` gains `Tier`, `AdvancementRequirements`, `Drawbacks` fields
- [ ] `DrawbackEngine`: registers situational triggers at login for all held jobs
- [ ] `DrawbackDef.Duration` parsed at registry load time; invalid strings fatal

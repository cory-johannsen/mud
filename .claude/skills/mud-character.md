---
name: mud-character
description: Character creation, ability scores, progression, and level-up system
type: reference
---

## Trigger

Reference this skill when working on:
- Character creation flow (archetype, team, job selection)
- Ability score calculation and boost application
- Skill, feat, and class feature assignment
- Level-up and experience progression
- Innate / hardwired / prepared / spontaneous technology grants at creation
- `handleChar`, `handleArchetypeSelection`, `handleLevelUp` in `grpc_service.go`
- Any file under `internal/game/character/` or `internal/frontend/handlers/character_flow.go`

## Responsibility Boundary

The character system owns:
- Pure domain model: `internal/game/character/` — `Character` struct, `AbilityScores`, builder functions, gender constants
- Interactive creation UI: `internal/frontend/handlers/character_flow.go` — telnet prompts, confirmation, skill/feat/class-feature ensure passes
- gRPC orchestration: `internal/gameserver/grpc_service.go` — `handleChar`, `handleArchetypeSelection`, `handleLevelUp`
- Technology assignment at session join: `internal/gameserver/technology_assignment.go` — `AssignTechnologies`

Out of scope: combat resolution, inventory, room navigation, persistence schema (those live in their own subsystems).

## Key Files

| File | Purpose |
|------|---------|
| `internal/game/character/model.go` | `Character` struct and `AbilityScores` type |
| `internal/game/character/builder.go` | `BuildWithJob`, `ApplyAbilityBoosts`, `BuildSkillsFromJob`, `BuildFeatsFromJob`, `BuildClassFeaturesFromJob` |
| `internal/game/character/gender.go` | `StandardGenders`, `RandomStandardGender` |
| `internal/game/ruleset/archetype.go` | `Archetype` struct, `InnateGrant`, `TechnologyGrants` |
| `internal/game/ruleset/job.go` | `Job` struct, `SkillGrants`, `FeatGrants`, `ClassFeatureGrants` |
| `internal/frontend/handlers/character_flow.go` | `characterFlow`, `characterCreationFlow`, `ensureSkills`, `ensureFeats`, `ensureClassFeatures`, `buildAndConfirm` |
| `internal/gameserver/grpc_service.go` | `handleChar`, `handleArchetypeSelection`, `handleLevelUp` |
| `internal/gameserver/technology_assignment.go` | `AssignTechnologies` |
| `content/archetypes/*.yaml` | Archetype YAML content |
| `content/jobs/*.yaml` | Job YAML content |
| `api/proto/game/v1/game.proto` | `CharacterInfo` and `CharacterSheetView` proto messages |

## Core Data Structures

### `character.Character` (internal/game/character/model.go)

| Field | Type | Notes |
|-------|------|-------|
| `ID`, `AccountID` | `int64` | Zero until persisted |
| `Name` | `string` | 2–32 characters |
| `Region` | `string` | Home region ID |
| `Class` | `string` | Job ID (not archetype ID) |
| `Team` | `string` | `"gun"` or `"machete"` |
| `Level`, `Experience` | `int` | Start at 1, 0 |
| `Location` | `string` | Current room ID; defaults to `"grinders_row"` |
| `Abilities` | `AbilityScores` | Six scores: Brutality, Grit, Quickness, Reasoning, Savvy, Flair |
| `MaxHP`, `CurrentHP` | `int` | Computed at creation: `HitPointsPerLevel + GRT modifier`, min 1 |
| `DefaultCombatAction` | `string` | Persisted; `"pass"` when unset |
| `Gender` | `string` | One of standard values or `"custom:<text>"` |
| `Skills` | `map[string]string` | skill_id → proficiency rank (`"trained"` / `"untrained"`) |
| `Feats` | `[]string` | Feat IDs |
| `ClassFeatures` | `[]string` | Class feature IDs |

### `CharacterInfo` proto (api/proto/game/v1/game.proto)

Sent to the client at session join. Key fields: `character_id`, `name`, `region`, `class`, `level`, `experience`, `max_hp`, `current_hp`, plus all six ability scores as `int32`.

### Archetype YAML structure (content/archetypes/*.yaml)

```yaml
id: aggressor
name: Aggressor
description: "..."
key_ability: brutality         # ability that receives +2 at job application
hit_points_per_level: 10
ability_boosts:
  fixed: [brutality, grit]    # always applied
  free: 2                     # number of player-chosen boosts
# Optional fields:
technology_grants:             # hardwired/prepared/spontaneous tech grants
  hardwired: [tech_id]
  prepared:
    slots_by_level: {1: 2}
innate_technologies:           # innate tech grants [{id, max_uses}]
  - id: tech_id
    max_uses: 3
level_up_grants:               # map[level → TechnologyGrants]
  2:
    prepared:
      slots_by_level: {1: 1}
```

### Job YAML structure (content/jobs/*.yaml)

```yaml
id: goon
name: Goon
archetype: aggressor           # parent archetype ID
description: "..."
key_ability: flair             # overrides archetype key ability for HP calc
hit_points_per_level: 8
proficiencies:
  simple_weapons: trained
  light_armor: trained
skills:
  fixed: [muscle, hard_look]   # always trained
  choices:
    count: 2
    pool: [rep, grift, parkour, gang_codes]
feats:
  general_count: 1
  fixed: [raw_intensity]
  choices:
    pool: [reach_influence, crowd_performer]
    count: 1
class_features: [command_attention, fast_talk, muscle_up]
technology_grants:             # optional; merged with archetype grants
  hardwired: [tech_id]
level_up_grants:               # optional map[level → TechnologyGrants]
```

## Primary Data Flow

### Step 1 — Archetype selection

`characterCreationFlow` (character_flow.go) presents archetypes filtered by selected team via `jobRegistry.ArchetypesForTeam`. Player selects; `selectedArchetype` captures `key_ability`, `hit_points_per_level`, `ability_boosts.fixed`, `ability_boosts.free`, and optional `technology_grants`. `handleArchetypeSelection` in grpc_service.go is a stub that validates the session exists; the bulk of creation state lives in the frontend flow.

### Step 2 — Job selection and ability boosts

Player selects a job via `jobRegistry.JobsForTeamAndArchetype`. The job carries its own `key_ability` and `hit_points_per_level` (which override the archetype values). `BuildWithJob` in builder.go applies region modifiers (start all six at 10, add region deltas), then applies the job `key_ability` +2 boost. `MaxHP = HitPointsPerLevel + GRT_modifier`, minimum 1. Hardwired technology IDs listed in `job.TechnologyGrants.Hardwired` (merged with archetype grants) are assigned when `AssignTechnologies` runs at session join.

### Step 3 — Character completion (`handleChar`)

`handleChar` (grpc_service.go) assembles a `CharacterSheetView` proto from the active `PlayerSession`. It reads job name and archetype from `jobRegistry`, computes defense stats from equipment, populates weapon attack bonuses from the active loadout, and returns the full sheet as a `ServerEvent`. `CharacterInfo` is assembled separately at session-join time (stream open) and sent as the initial `ServerEvent`.

### Step 4 — Level-up (`handleLevelUp`)

`handleLevelUp` (grpc_service.go) checks `sess.PendingBoosts > 0`, applies +2 to the named ability in a copy of `AbilityScores`, persists via `charSaver.SaveAbilities` and `progressRepo.ConsumePendingBoost`, then mutates session state. `LevelUpGrants` from the archetype YAML (map[int → TechnologyGrants]) provide prepared/spontaneous slot increases; these are applied by `AssignTechnologies` when the session reconnects or when explicit level-up tech logic runs.

### Step 5 — Innate region grants (`AssignTechnologies`)

`AssignTechnologies` (technology_assignment.go) is called at session join after character load. It merges `archetype.TechnologyGrants` with `job.TechnologyGrants` via `ruleset.MergeGrants`, then:
1. Assigns hardwired IDs to `sess.HardwiredTechs` and persists via `hwRepo.SetAll`.
2. Walks `archetype.InnateTechnologies` and `region.InnateTechnologies` (each an `[]InnateGrant` with `id` and `max_uses`), initializes `sess.InnateTechs`, and persists via `innateRepo.Set`.
3. Assigns prepared slots from `grants.Prepared.SlotsByLevel` and spontaneous known-list from `grants.Spontaneous`.

## Invariants & Contracts

- `BuildWithJob` MUST receive non-nil region, job, and team; returns error otherwise.
- All six ability scores start at 10; region modifiers are additive; key ability receives exactly +2.
- `MaxHP` is always >= 1 (enforced by `max(1, hpPerLevel+grtMod)` in builder.go).
- `BuildSkillsFromJob` returns a map with exactly `len(allSkillIDs)` entries.
- `BuildFeatsFromJob` deduplicates feat IDs across fixed + chosen + generalChosen + skillChosen.
- `ensureSkills`, `ensureFeats`, `ensureClassFeatures` are idempotent — they check for existing rows before prompting.
- `handleLevelUp` MUST NOT mutate session state until both persistence calls succeed.
- `AssignTechnologies` validates merged grants before any persistence write.

## Extension Points

### How to add a new archetype

1. Create `content/archetypes/<id>.yaml` with required fields: `id`, `name`, `description`, `key_ability`, `hit_points_per_level`, `ability_boosts.fixed` (list), `ability_boosts.free` (int).
2. Optional: add `technology_grants`, `innate_technologies`, and `level_up_grants` sections.
3. The archetype loader (in `internal/game/ruleset/`) reads all YAML files in `content/archetypes/` at startup — no code registration needed.
4. Jobs reference the archetype by its `id` field in their `archetype:` YAML key.
5. Run the test suite to confirm the new archetype parses correctly and all property-based tests pass.

### How to add a new job

1. Create `content/jobs/<id>.yaml` with required fields: `id`, `name`, `archetype` (must match an existing archetype `id`), `description`, `key_ability`, `hit_points_per_level`.
2. Add `skills`, `feats`, `class_features`, and optionally `technology_grants` / `level_up_grants` as needed.
3. Set `team:` to `"gun"` or `"machete"` if the job is team-exclusive; omit for cross-team jobs.
4. The job loader reads all YAML in `content/jobs/` at startup — no code registration needed.
5. Run the test suite; the job registry builds automatically and the new job will appear in `JobsForTeamAndArchetype` lookups.

## Common Pitfalls

- `Character.Class` stores the **job ID**, not the archetype ID. Archetype is looked up on the fly via `jobRegistry.Job(class).Archetype`.
- `handleArchetypeSelection` is currently a stub (returns empty `ServerEvent`). All real archetype-selection state lives in the frontend `characterCreationFlow`.
- `BuildWithJob` applies the **job** `key_ability` boost, not the archetype one. The archetype key ability is used only for display in `RenderArchetypeMenu`.
- Ability boost order in `ApplyAbilityBoosts`: archetype fixed → archetype free chosen → region fixed → region free chosen. Wrong ordering changes scores.
- `ensureFeats` uses `FeatPoolDeficit` per pool — it MUST NOT re-prompt pools that are already satisfied. Passing the wrong pool or stored-set will cause double grants.
- `AssignTechnologies` short-circuits if `job == nil && region == nil && len(archetype.InnateTechnologies) == 0`. A nil archetype passed for a character that has innate tech will silently skip those grants.
- HP uses the **job** `HitPointsPerLevel`, not the archetype's, even though the archetype also carries that field.

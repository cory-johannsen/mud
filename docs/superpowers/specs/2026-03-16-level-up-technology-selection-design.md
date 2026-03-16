# Level-Up Technology Selection — Design Spec

**Date:** 2026-03-16

---

## Goal

When a character levels up, apply technology grants defined on their job for the new character level. New prepared and spontaneous slots trigger the same interactive player prompt used at character creation. This sub-project covers the YAML data model extension, validation, the `LevelUpTechnologies` function, and gameserver wiring. Use count decrement, slot rearrangement at rest, and `UsesByLevel` changes at level-up are out of scope.

---

## Context

The Technology Grants sprint (2026-03-15) built `AssignTechnologies` and the four technology repos. `TechnologyGrants` on `Job` defines slots at character creation via `SlotsByLevel` / `KnownByLevel`. This spec adds a parallel `LevelUpGrants` map that expresses per-character-level technology deltas, reusing all existing infrastructure.

---

## Feature 1: Job YAML extension

### New `level_up_grants` field on `Job`

```go
// LevelUpGrants maps character level to the technology grants gained at that level.
// Each entry is a delta — only new slots/techs added at that character level, not the
// full cumulative table.
LevelUpGrants map[int]*TechnologyGrants `yaml:"level_up_grants,omitempty"`
```

Added to the existing `Job` struct alongside `TechnologyGrants`.

### Validation

At YAML load time, `LoadJobs` iterates `LevelUpGrants` and calls `Validate()` on each entry. The loop runs after the existing `TechnologyGrants` validation:

```go
for charLevel, grants := range job.LevelUpGrants {
    if err := grants.Validate(); err != nil {
        return nil, fmt.Errorf("job %q level_up_grants[%d]: %w", job.ID, charLevel, err)
    }
}
```

A validation error on any entry fails the entire job load (fail-fast). Level keys must be positive integers (≥ 1); a key of 0 or negative is a validation error.

`UsesByLevel` within a `level_up_grants` spontaneous entry is parsed but silently ignored at level-up time (use tracking is out of scope). It is not validated beyond the existing `TechnologyGrants.Validate()` rules.

### Example Job YAML

```yaml
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool:
        - id: arc_thought
          level: 2
        - id: mind_spike
          level: 2
  5:
    prepared:
      slots_by_level:
        3: 1
      pool:
        - id: neural_shock
          level: 3
    spontaneous:
      known_by_level:
        2: 1
      pool:
        - id: acid_spray
          level: 2
```

---

## Feature 2: `LevelUpTechnologies` function

New function in `internal/gameserver/technology_assignment.go` alongside `AssignTechnologies`:

```go
// LevelUpTechnologies applies a technology grants delta to an existing character's session
// and persists new slot assignments. It is called once per character level gained.
//
// Precondition: grants must be non-nil and valid (validated at YAML load time).
// Postcondition: sess and repos reflect all new slots from grants; existing slots are unchanged.
func LevelUpTechnologies(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    grants *TechnologyGrants,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    hwRepo HardwiredTechRepo,
    prepRepo PreparedTechRepo,
    spontRepo SpontaneousTechRepo,
    innateRepo InnateTechRepo,
) error
```

### Behavior

1. **Nil guard** — if `grants` is nil, return nil immediately (no-op).

2. **Hardwired** — append any new IDs in `grants.Hardwired` to `sess.HardwiredTechs`, skipping any ID already present in `sess.HardwiredTechs` (map-based deduplication, O(n)). Persist the updated full list via `hwRepo.SetAll`. The resulting slice order is: existing IDs first, then new IDs in the order they appear in `grants.Hardwired`.

3. **Prepared** — for each level in `grants.Prepared.SlotsByLevel`:
   - Determine the next available slot index for this level by calling `prepRepo.GetAll` and finding `len(existing[level])`. Existing slot slices are always dense (no nil gaps for prior levels), so `len` gives the correct next index.
   - Pre-fill from `grants.Prepared.Fixed` at this level (no prompt); persist via `prepRepo.Set` at the next indices.
   - For remaining open slots: auto-assign if `len(pool at level) == open`, otherwise prompt.
   - Persist each new slot via `prepRepo.Set`.
   - Update `sess.PreparedTechs`.

4. **Spontaneous** — for each level in `grants.Spontaneous.KnownByLevel`:
   - Pre-fill from `grants.Spontaneous.Fixed` at this level (no prompt); add via `spontRepo.Add`.
   - For remaining open slots: auto-assign if `len(pool at level) == open`, otherwise prompt.
   - Add each new known tech via `spontRepo.Add`.
   - Update `sess.SpontaneousTechs`.

5. **Innate** — not supported in level-up grants (innate technologies are archetype-granted and do not change on level-up). Any `InnateGrant` entries in the delta are silently ignored.

---

## Feature 3: Gameserver wiring

In `internal/gameserver/grpc_service.go`, modify the `handleGrant` function (the XP award handler). Before calling `xp.Award`, capture the current level:

```go
oldLevel := target.Level
result := s.xpService.Award(target, amount)
```

After level-up is detected, apply grants for every level gained. Errors are logged at Warn and do not abort the level-up (non-fatal, to avoid blocking game flow):

```go
if result.LeveledUp && s.hardwiredTechRepo != nil && s.jobRegistry != nil {
    if job, ok := s.jobRegistry.Job(sess.Class); ok {
        for lvl := oldLevel + 1; lvl <= result.NewLevel; lvl++ {
            grants, hasGrants := job.LevelUpGrants[lvl]
            if !hasGrants {
                continue
            }
            promptFn := func(options []string) (string, error) {
                choices := &ruleset.FeatureChoices{
                    Prompt:  fmt.Sprintf("Choose a level-%d technology:", lvl),
                    Options: options,
                    Key:     "tech_choice",
                }
                return s.promptFeatureChoice(stream, "tech_choice", choices)
            }
            if err := LevelUpTechnologies(ctx, sess, characterID,
                grants, s.techRegistry, promptFn,
                s.hardwiredTechRepo, s.preparedTechRepo,
                s.spontaneousTechRepo, s.innateTechRepo,
            ); err != nil {
                s.logger.Warn("LevelUpTechnologies failed",
                    zap.Int64("character_id", characterID),
                    zap.Int("level", lvl),
                    zap.Error(err))
            }
        }
    }
}
```

No new proto messages, no new repos, no new DB tables.

---

## Testing

All tests using TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-LUT1**: `Job` with `level_up_grants` YAML round-trips without data loss.
- **REQ-LUT2**: `TechnologyGrants.Validate()` on a `level_up_grants` entry rejects `pool + fixed < slots_by_level` for any tech level.
- **REQ-LUT3**: `LevelUpTechnologies` with a delta containing hardwired IDs appends new IDs to existing session hardwired techs; IDs already present are skipped (no duplicates introduced).
- **REQ-LUT4**: `LevelUpTechnologies` with a prepared delta fills new slots starting after existing slot indices; for any combination of N existing slots and M new slots at a given level, all resulting slot indices in the repo are unique.
- **REQ-LUT5**: `LevelUpTechnologies` with a spontaneous delta adds new known techs without removing existing ones.
- **REQ-LUT6**: `LevelUpTechnologies` with a nil `grants` argument returns nil and makes no changes (no-op).
- **REQ-LUT7**: The gameserver applies `level_up_grants` for every level gained in ascending level order when a player skips levels (e.g., 2→4 applies grants for level 3 then level 4).
- **REQ-LUT8** (property): For any valid `level_up_grants` map, YAML marshal/unmarshal round-trip preserves all fields.
- **REQ-LUT9**: `LoadJobs` rejects a YAML file with an invalid `level_up_grants` entry (e.g., pool + fixed < slots_by_level for any tech level); the error message includes the job ID and the failing character level.

---

## Constraints

- Use count decrement is out of scope.
- Slot rearrangement at rest is out of scope.
- `UsesByLevel` changes at level-up are out of scope.
- Innate technology level-up grants are not supported (innate technologies are archetype-granted only).
- No new DB tables or repos — all existing technology repos are reused.
- No new proto messages.

# Archetype Selection — Design

**Date:** 2026-03-02
**Feature:** Insert archetype selection step into character creation flow (team → archetype → job).

---

## Summary

A new archetype selection step is inserted between team selection and job selection in the character creation flow. The player sees all archetypes available for their chosen team, each displayed with name, description, key ability (Gunchete name), and HP/level. The job list is then filtered to only show jobs matching both the selected team and archetype. Archetype is derived at runtime from the job ID — no DB migration required.

---

## Architecture

**Flow change** (`internal/frontend/handlers/character_flow.go`):
- New state `StateArchetypeSelection` inserted between `StateTeamSelection` and `StateJobSelection`.
- `AuthHandler` gains an `archetypes []*ruleset.Archetype` field loaded at startup.
- `handleTeamSelected` transitions to `StateArchetypeSelection` instead of `StateJobSelection`.
- New `handleArchetypeSelected` stores `selectedArchetype` and transitions to `StateJobSelection`.
- `renderArchetypeMenu` displays each archetype with name, description, key ability (Gunchete name), and HP/level.
- Job menu filtered by `job.Archetype == selectedArchetype && job.Team == selectedTeam`.

**Content fixes** (`content/archetypes/*.yaml`):
- `key_ability` values updated from D&D names to Gunchete names:
  - strength → brutality
  - dexterity → quickness
  - constitution → grit
  - intelligence → reasoning
  - wisdom → savvy
  - charisma → flair

**Archetype storage**: Derived at runtime from job ID — no DB migration needed.

**Proto** (`api/proto/game/v1/game.proto`):
- `ArchetypeSelectionRequest { string archetype_id = 1; }` added to `ClientMessage` oneof.

---

## Data Schema

### New proto message

```proto
message ArchetypeSelectionRequest {
    string archetype_id = 1;
}
```

Added to `ClientMessage` oneof at next available field number. `make proto` regenerates bindings.

### JobRegistry additions

```go
// ArchetypesForTeam returns all archetypes that have at least one job for the given team.
func (r *JobRegistry) ArchetypesForTeam(team string) []*Archetype

// JobsForTeamAndArchetype returns jobs matching both team and archetype.
func (r *JobRegistry) JobsForTeamAndArchetype(team, archetype string) []*Job
```

### Session state

`selectedArchetype string` added to character flow state struct. Passed into `JoinWorldRequest.Archetype` (already consumed by server for starting inventory).

---

## Key Files

| File | Change |
|---|---|
| `content/archetypes/*.yaml` | UPDATE `key_ability` to Gunchete names (6 files) |
| `internal/game/ruleset/job_registry.go` | ADD `ArchetypesForTeam`, `JobsForTeamAndArchetype` |
| `internal/game/ruleset/job_registry_test.go` | ADD unit + property tests for new methods |
| `api/proto/game/v1/game.proto` | ADD `ArchetypeSelectionRequest`; wire into `ClientMessage` oneof |
| `internal/frontend/handlers/character_flow.go` | ADD `StateArchetypeSelection`, `handleArchetypeSelected`, `renderArchetypeMenu`; update `handleTeamSelected` |
| `internal/frontend/handlers/bridge_handlers.go` | ADD `bridgeArchetypeSelection` + register in `bridgeHandlerMap` |
| `internal/game/command/commands.go` | ADD `HandlerArchetypeSelection` constant + `Command{...}` entry |
| `internal/gameserver/grpc_service.go` | ADD `handleArchetypeSelection` wired into dispatch type switch |

---

## Testing

### Registry tests (`internal/game/ruleset/job_registry_test.go`)
- `TestArchetypesForTeam_ReturnsOnlyMatchingTeam` — gun team returns only archetypes with gun jobs.
- `TestJobsForTeamAndArchetype_FiltersCorrectly` — combined filter returns correct subset.
- `TestProperty_ArchetypesForTeam_NeverPanics` — random team strings never panic.

### Flow tests (`internal/frontend/handlers/character_flow_test.go`)
- `TestArchetypeSelectionState_TransitionsToJobSelection`
- `TestArchetypeMenuRender_ContainsKeyAbility` — rendered menu shows Gunchete ability name.
- `TestProperty_HandleArchetypeSelected_NeverPanics`

### Content tests
- `TestArchetypeYAML_KeyAbilitiesUseGuncheteNames` — all 6 YAMLs use valid Gunchete ability names.
- `TestAllArchetypesHaveJobsForBothTeams` — each archetype has jobs on both teams (documents exceptions).

### Wiring test
- `TestAllCommandHandlersAreWired` must pass.

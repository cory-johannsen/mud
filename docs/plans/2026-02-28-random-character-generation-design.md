# Random Character Generation Design

**Date:** 2026-02-28

## Goal

Allow players to choose Random at any character creation step, randomizing that step and all remaining steps, then jumping straight to the preview. Random is the default selection at each list step.

## Flow

Character creation has four steps: name → region → team → job → preview → confirm.

### Name step
- Prompt unchanged except for an added hint: `(type 'random' for a random name)`
- If the player types `random`, a name is picked from a static name list
- This step never auto-cascades — the player must explicitly request a random name
- Default behavior: player types a name manually

### Region step
- `R. Random` appended as the last option in the list
- Prompt: `[1-N/R, default=R]:`
- Blank input or `r`/`R` → pick random region, then cascade: randomize team and job, skip to preview
- Numeric input → explicit choice, continue to team step normally

### Team step
- Same pattern as region
- `R` → pick random team, then cascade: randomize job, skip to preview

### Job step
- Same pattern
- `R` → pick random job, skip to preview

### Preview
- Always shows the full resolved selections (including any randomly chosen values) so the player can see what was picked before confirming

## Architecture

All changes are confined to `internal/frontend/handlers/character_flow.go`.

No new files. No schema changes. No migrations.

### Random name list
A package-level `var randomNames = []string{...}` slice with ~20 post-apocalyptic themed names (e.g. Raze, Vex, Cinder, Sable, Grit, Ash, Flint). Selected via `rand.Intn(len(randomNames))`.

### randomizeRemaining helper
```go
func randomizeRemaining(regions []*ruleset.Region, teams []*ruleset.Team, jobs []*ruleset.Job, fixedTeam *ruleset.Team) (*ruleset.Region, *ruleset.Team, *ruleset.Job)
```
Encapsulates all cascade randomization. Called whenever the player picks R at any step. Keeps step handlers thin and the logic independently testable.

### Prompt parsing
Each list prompt changes input parsing:
- blank input or `r`/`R` → random
- valid number → explicit choice
- anything else → invalid, re-prompt (consistent with current behavior)

## Testing

Extend `internal/frontend/handlers/character_flow_test.go`:
- Random name: typing `random` at name step picks from `randomNames`
- R at region: cascades through team and job, lands at preview
- R at team: cascades through job, lands at preview
- R at job: randomizes job only, lands at preview
- Explicit choices still work at all steps
- Blank input at list steps triggers random (default behavior)

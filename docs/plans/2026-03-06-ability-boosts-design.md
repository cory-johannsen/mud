# Character Ability Boosts Design

**Date:** 2026-03-06
**Status:** Approved

## Problem

Characters are created with ability scores derived only from region `modifiers` (fixed deltas) and the job `key_ability` (+2). P2FE grants players free ability-boost choices at character creation from three sources: Archetype (2 fixed + 2 free), Region (2 fixed + 1 free), and Job (1 fixed — already implemented). These free choices are missing.

## Approach: P2FE-faithful with additive region modifiers

- Keep existing `region.modifiers` as a separate "background flavor" layer (can be negative).
- Add a new `ability_boosts` block to both Archetype and Region YAML.
- Archetype: 2 fixed boosts + 2 free player-chosen boosts.
- Region: 2 fixed boosts + 1 free player-chosen boost.
- Job: key ability (+2) — no YAML change.
- Within-source uniqueness enforced: player cannot pick an ability already listed as a fixed boost for that source, nor repeat their own earlier free-slot pick within the same source.
- No cap at 18 (deferred to levelling feature).
- Existing characters missing choices are prompted at login (same pattern as feat/class feature choices).

## Section 1: YAML Content Changes

### Ability boost assignments

**Archetypes** (`content/archetypes/<id>.yaml`):

| Archetype  | Fixed                   | Free |
|------------|-------------------------|------|
| aggressor  | brutality, grit         | 2    |
| criminal   | quickness, savvy        | 2    |
| drifter    | grit, brutality         | 2    |
| influencer | flair, savvy            | 2    |
| nerd       | reasoning, savvy        | 2    |
| normie     | savvy, flair            | 2    |

**Regions** (`content/regions/<id>.yaml`):

| Region              | Fixed                   | Free |
|---------------------|-------------------------|------|
| gresham_outskirts   | savvy, grit             | 1    |
| midwest             | brutality, reasoning    | 1    |
| mountain            | brutality, grit         | 1    |
| northeast           | quickness, reasoning    | 1    |
| north_portland      | brutality, grit         | 1    |
| old_town            | flair, quickness        | 1    |
| pacific_northwest   | flair, quickness        | 1    |
| pearl_district      | reasoning, savvy        | 1    |
| southeast_portland  | grit, quickness         | 1    |
| southern_california | quickness, reasoning    | 1    |
| south               | grit, brutality         | 1    |

YAML block added to each file:
```yaml
ability_boosts:
  fixed: [brutality, grit]
  free: 2
```

## Section 2: Ruleset Struct Changes

New shared struct in `internal/game/ruleset/`:

```go
// AbilityBoostGrant describes the ability boosts a source provides.
type AbilityBoostGrant struct {
    Fixed []string `yaml:"fixed"` // ability IDs always boosted by this source
    Free  int      `yaml:"free"`  // number of player-chosen free boost slots
}
```

`Archetype` gains:
```go
AbilityBoosts *AbilityBoostGrant `yaml:"ability_boosts"`
```

`Region` gains:
```go
AbilityBoosts *AbilityBoostGrant `yaml:"ability_boosts"`
```

## Section 3: Persistence

New DB table:

```sql
CREATE TABLE character_ability_boosts (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    source        TEXT    NOT NULL,  -- "archetype" or "region"
    ability       TEXT    NOT NULL,
    PRIMARY KEY (character_id, source, ability)
);
```

Repository interface (`internal/gameserver/grpc_service.go`):

```go
type CharacterAbilityBoostsRepository interface {
    GetAll(ctx context.Context, characterID int64) (map[string][]string, error) // source → []ability
    Add(ctx context.Context, characterID int64, source, ability string) error
}
```

Implementation: Postgres, same pattern as `CharacterFeatureChoicesRepository`.

## Section 4: Score Computation

New pure function in `internal/game/character/builder.go`:

```go
// ApplyAbilityBoosts returns the ability scores after stacking all boost sources.
// Order: archetype fixed → archetype free chosen → region fixed → region free chosen → job key ability.
// Each boost adds +2. The job key ability boost is already in base (from BuildWithJob);
// this function is used to recompute scores at login when boost choices are loaded.
//
// Precondition: archetypeChosen and regionChosen must satisfy within-source uniqueness.
// Postcondition: Each boosted ability increases by exactly +2 per boost application.
func ApplyAbilityBoosts(
    base AbilityScores,
    archetypeBoosts *AbilityBoostGrant, archetypeChosen []string,
    regionBoosts *AbilityBoostGrant, regionChosen []string,
) AbilityScores
```

Helper for prompt UI:

```go
// AbilityBoostPool returns the valid free-boost ability pool for a source.
// Excludes: abilities in fixed, abilities already chosen in earlier free slots.
//
// Postcondition: returned slice is sorted and contains no duplicates.
func AbilityBoostPool(fixed []string, alreadyChosen []string) []string
```

At session load:
1. Fetch stored boost choices from DB.
2. Call `ApplyAbilityBoosts` with base scores (base 10 + region modifiers + job key ability already in `Character.Abilities`).

Wait — the scores in `Character.Abilities` already include region modifiers and key ability from `BuildWithJob`. To avoid double-applying, we need to compute fresh from scratch at load time:

```
base 10 + region.Modifiers + archetype fixed boosts + archetype free chosen
        + region fixed boosts + region free chosen + job key ability
```

This means `ApplyAbilityBoosts` takes the raw modifiers-only base (before key ability), applies all boosts, and the result replaces `Character.Abilities` in the session. The DB record for `Character.Abilities` is also updated after boost selection so the character sheet stays current.

## Section 5: Interactive Prompt Flow

After the initial room view is sent, missing boost choices are prompted in order:

1. **Archetype free boost slot 1** — pool = all 6 abilities minus archetype fixed
2. **Archetype free boost slot 2** — pool = slot 1 pool minus slot 1 pick
3. **Region free boost** — pool = all 6 abilities minus region fixed

Each step uses `promptFeatureChoice`-style blocking `stream.Recv()` with a numbered list. After all selections, `ApplyAbilityBoosts` is computed, `Character.Abilities` updated in DB.

For creation flow: characters are saved to DB before first session starts, so the login path handles both new and existing characters uniformly.

## Section 6: Testing

- **Unit — `ApplyAbilityBoosts`** (property-based via `pgregory.net/rapid`): each boost adds exactly +2; nil boost grant is a no-op; stacking multiple sources is additive.
- **Unit — `AbilityBoostPool`**: excludes fixed and already-chosen; returns sorted slice; handles empty inputs.
- **Unit — DB repo**: `Add` stores row; `GetAll` returns correct map; duplicate `Add` is idempotent (ON CONFLICT DO NOTHING).
- **Integration — prompt flow**: mock stream test: character missing archetype boosts is prompted in correct order; choices persisted; `Character.Abilities` reflects all stacked boosts.
- **Regression**: `BuildWithJob` tests unaffected; `ApplyAbilityBoosts` with nil chosen slices produces scores identical to current behavior.

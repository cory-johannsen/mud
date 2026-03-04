# Class Features Stage 1 — Design Document

**Date:** 2026-03-03
**Status:** Approved

---

## Overview

Implement P2FE level-1 class features for all 76 Gunchete jobs. Each character receives a fixed
set of class features at creation: archetype-shared features (2 per archetype, 12 total) plus one
job-specific feature (76 total). All features are fixed grants — no player selection. Active
features are activated via the existing `use` command alongside feats.

---

## Content: `content/class_features.yaml`

Each entry has the same shape as a feat entry, with `archetype` and `job` fields for categorization:

```yaml
- id: brutal_surge
  name: Brutal Surge
  archetype: aggressor     # non-empty for archetype-shared features
  job: ""                  # non-empty for job-specific features
  pf2e: rage
  active: true
  activate_text: "The red haze drops and you move on pure instinct."
  description: "Enter a combat frenzy: +2 melee damage, -2 AC until end of encounter."
```

### Archetype-Shared Features (12 total — 2 per archetype)

| Archetype  | ID                   | Name               | P2FE Equivalent     | Active |
|------------|----------------------|--------------------|---------------------|--------|
| aggressor  | street_brawler       | Street Brawler     | attack_of_opportunity | false |
| aggressor  | brutal_surge         | Brutal Surge       | rage                | true   |
| criminal   | sucker_punch         | Sucker Punch       | sneak_attack        | false  |
| criminal   | slippery             | Slippery           | evasion             | false  |
| drifter    | predators_eye        | Predator's Eye     | hunters_edge        | false  |
| drifter    | zone_awareness       | Zone Awareness     | wild_stride         | false  |
| influencer | command_attention    | Command Attention  | inspire_courage     | true   |
| influencer | fast_talk            | Fast Talk          | inspire_defense     | true   |
| nerd       | daily_prep           | Daily Prep         | daily_infusions     | false  |
| nerd       | formulaic_mind       | Formulaic Mind     | formula_book        | false  |
| normie     | street_tough         | Street Tough       | flurry_of_blows     | true   |
| normie     | resilience           | Resilience         | incredible_movement | false  |

### Job-Specific Features (76 total — 1 per job)

Each job gets one signature feature. Full list authored during implementation (see Task 2 in the
implementation plan). Examples:

| Job              | ID                    | Name                  | Description |
|------------------|-----------------------|-----------------------|-------------|
| soldier          | guerilla_warfare      | Guerilla Warfare      | +cover attack bonus in urban terrain |
| thief            | five_finger_discount  | Five-Finger Discount  | Free palm attempt after successful steal |
| scout            | dead_reckoning        | Dead Reckoning        | Always knows direction; no navigation penalty |
| beat_down_artist | bone_breaker          | Bone Breaker          | Successful grapple reduces target movement |
| contract_killer  | clean_shot            | Clean Shot            | First attack from concealment is always a surprise |

---

## Job YAML Changes

- **Remove** all legacy `features` blocks from all 76 job YAMLs (leftover from data import)
- **Add** `class_features` block — flat list of IDs, all fixed grants:

```yaml
class_features:
  - brutal_surge
  - street_brawler
  - guerilla_warfare
```

---

## Go Types

### `internal/game/ruleset/class_feature.go`

```go
type ClassFeature struct {
    ID           string `yaml:"id"`
    Name         string `yaml:"name"`
    Archetype    string `yaml:"archetype"`
    Job          string `yaml:"job"`
    PF2E         string `yaml:"pf2e"`
    Active       bool   `yaml:"active"`
    ActivateText string `yaml:"activate_text"`
    Description  string `yaml:"description"`
}

func LoadClassFeatures(path string) ([]*ClassFeature, error)

type ClassFeatureRegistry struct { /* byID, byArchetype, byJob */ }
func NewClassFeatureRegistry(features []*ClassFeature) *ClassFeatureRegistry
func (r *ClassFeatureRegistry) ClassFeature(id string) (*ClassFeature, bool)
func (r *ClassFeatureRegistry) ByArchetype(archetype string) []*ClassFeature
func (r *ClassFeatureRegistry) ByJob(job string) []*ClassFeature
```

### Job struct extension (`internal/game/ruleset/job.go`)

```go
ClassFeatureGrants []string `yaml:"class_features"`
```

---

## Database

**Migration `013_character_class_features`:**
```sql
CREATE TABLE character_class_features (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id   TEXT NOT NULL,
    PRIMARY KEY (character_id, feature_id)
);
```

---

## Repository

`internal/storage/postgres/character_class_features.go`:
- `HasClassFeatures(ctx, characterID) (bool, error)`
- `GetAll(ctx, characterID) ([]string, error)`
- `SetAll(ctx, characterID, featureIDs []string) error`

Same delete-then-insert transaction pattern as `CharacterFeatsRepository`.

---

## Character Model

`internal/game/character/model.go` — add:
```go
ClassFeatures []string
```

`internal/game/character/builder.go` — add:
```go
// BuildClassFeaturesFromJob returns all class feature IDs granted by the job.
// All grants are fixed; no player selection required.
func BuildClassFeaturesFromJob(job *ruleset.Job) []string
```

---

## Frontend

### `internal/frontend/handlers/auth.go`

Add interface:
```go
type CharacterClassFeaturesSetter interface {
    HasClassFeatures(ctx context.Context, characterID int64) (bool, error)
    SetAll(ctx context.Context, characterID int64, featureIDs []string) error
}
```

Add fields to `AuthHandler`: `characterClassFeatures`, `allClassFeatures`, `classFeatureRegistry`
Update `NewAuthHandler` signature with two new params.

### `internal/frontend/handlers/character_flow.go`

Add `ensureClassFeatures` — checks `HasClassFeatures`, calls `BuildClassFeaturesFromJob`, calls
`SetAll`. No interactive selection (all fixed). Called after `ensureFeats` at all 3 `gameBridge`
call sites.

---

## Commands

### `class_features` command (CMD-1 through CMD-7)

- Constant: `HandlerClassFeatures = "class_features"`
- Alias: `cf`
- Proto messages: `ClassFeaturesRequest` (ClientMessage field 42), `ClassFeatureEntry`,
  `ClassFeaturesResponse` (ServerEvent field 23)
- Renderer: groups by archetype features vs. job features; `[active]` tag on active features
- Server handler: `handleClassFeatures` — fetches from `characterClassFeaturesRepo`, resolves
  via `classFeatureRegistry`, returns `ClassFeaturesResponse`

### `use` command extension

`handleUse` in `grpc_service.go` is extended to search active class features in addition to
active feats. The combined list of active abilities (feats + class features) is returned in the
`UseResponse.Choices` or used for direct activation by name.

---

## Wiring

### `cmd/frontend/main.go` and `cmd/gameserver/main.go`
- `-class-features` flag (default: `content/class_features.yaml`)
- `ruleset.LoadClassFeatures`, `ruleset.NewClassFeatureRegistry`
- `postgres.NewCharacterClassFeaturesRepository`
- Pass to `NewAuthHandler` and `NewGameServiceServer`

### Dockerfiles
- Append `-class-features /content/class_features.yaml` to CMD arrays

---

## Implementation Tasks (summary)

1. `content/class_features.yaml` — all 88 features (12 archetype + 76 job)
2. `internal/game/ruleset/class_feature.go` — ClassFeature type, LoadClassFeatures, ClassFeatureRegistry
3. Extend Job struct with `ClassFeatureGrants`
4. Update all 76 job YAMLs — remove legacy `features` blocks, add `class_features` list
5. DB migration `013_character_class_features`
6. `CharacterClassFeaturesRepository`
7. Character model `ClassFeatures []string` + `BuildClassFeaturesFromJob`
8. `ensureClassFeatures` + auth.go wiring
9. `class_features` command end-to-end (CMD-1 through CMD-7)
10. Extend `use` command to include active class features
11. Wire into main.go flags and Dockerfiles
12. Deploy

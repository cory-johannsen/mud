# Feature Choice System — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Generalize per-feature sub-choices into a YAML-declared, generically-stored system, and fix the backfill gap where characters with partial feats skip the optional-choice prompt.

**Architecture:** Add a `FeatureChoices` struct to `ruleset.ClassFeature` and `ruleset.Feat`; store selections in a new `character_feature_choices` DB table; replace the hand-coded `predators_eye` prompt in `grpc_service.go` with a generic loop; fix `ensureFeats` in `character_flow.go` to use per-pool deficit checking instead of `HasFeats()`.

**Tech Stack:** Go, PostgreSQL, protobuf, pgx/v5, pgregory.net/rapid (property tests), YAML

---

## Task 1 — Add `FeatureChoices` struct to ruleset

**Files:**
- Modify: `internal/game/ruleset/class_feature.go`
- Modify: `internal/game/ruleset/feat.go`
- Create: `internal/game/ruleset/feature_choices_test.go`

**Step 1: Write failing tests**

Create `internal/game/ruleset/feature_choices_test.go`:

```go
package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestClassFeature_WithChoicesBlock(t *testing.T) {
	src := `
class_features:
  - id: predators_eye
    name: "Predator's Eye"
    archetype: drifter
    job: ""
    pf2e: hunters_edge
    active: false
    activate_text: ""
    description: "Choose a favored target."
    choices:
      key: favored_target
      prompt: "Choose your favored target type"
      options: [human, robot, animal, mutant]
`
	features, err := ruleset.LoadClassFeaturesFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, features, 1)
	cf := features[0]
	require.NotNil(t, cf.Choices)
	assert.Equal(t, "favored_target", cf.Choices.Key)
	assert.Equal(t, "Choose your favored target type", cf.Choices.Prompt)
	assert.Equal(t, []string{"human", "robot", "animal", "mutant"}, cf.Choices.Options)
}

func TestClassFeature_WithoutChoicesBlock(t *testing.T) {
	src := `
class_features:
  - id: street_brawler
    name: Street Brawler
    archetype: aggressor
    job: ""
    pf2e: attack_of_opportunity
    active: false
    activate_text: ""
    description: "No choices needed."
`
	features, err := ruleset.LoadClassFeaturesFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Nil(t, features[0].Choices)
}

func TestFeat_WithChoicesBlock(t *testing.T) {
	src := `
feats:
  - id: weapon_focus
    name: Weapon Focus
    category: general
    choices:
      key: weapon_group
      prompt: "Choose a weapon group"
      options: [pistol, rifle, melee, explosive]
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	f := feats[0]
	require.NotNil(t, f.Choices)
	assert.Equal(t, "weapon_group", f.Choices.Key)
	assert.Equal(t, []string{"pistol", "rifle", "melee", "explosive"}, f.Choices.Options)
}

func TestFeat_WithoutChoicesBlock(t *testing.T) {
	src := `
feats:
  - id: toughness
    name: Toughness
    category: general
    description: "+8 max HP."
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	assert.Nil(t, feats[0].Choices)
}

func TestPropertyFeatureChoices_OptionsRoundtrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		key := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "key")
		prompt := rapid.StringMatching(`[a-zA-Z ]{1,64}`).Draw(rt, "prompt")
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		options := make([]string, n)
		for i := range options {
			options[i] = rapid.StringMatching(`[a-z]{1,16}`).Draw(rt, "opt")
		}
		orig := ruleset.FeatureChoices{Key: key, Prompt: prompt, Options: options}
		data, err := yaml.Marshal(orig)
		if err != nil {
			rt.Fatalf("marshal: %v", err)
		}
		var got ruleset.FeatureChoices
		if err := yaml.Unmarshal(data, &got); err != nil {
			rt.Fatalf("unmarshal: %v", err)
		}
		if got.Key != orig.Key {
			rt.Fatalf("Key mismatch: got %q want %q", got.Key, orig.Key)
		}
		if len(got.Options) != len(orig.Options) {
			rt.Fatalf("Options len mismatch: got %d want %d", len(got.Options), len(orig.Options))
		}
	})
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/game/ruleset/... -run TestClassFeature_WithChoicesBlock
```

Expected: FAIL — `LoadClassFeaturesFromBytes undefined`

**Step 3: Add `FeatureChoices` struct to `internal/game/ruleset/class_feature.go`**

Add `FeatureChoices` struct (before the `ClassFeature` struct) and `Choices *FeatureChoices` field to `ClassFeature`:

```go
// FeatureChoices declares an interactive player choice attached to a feature or feat.
// Precondition: Options must be non-empty; Key and Prompt must be non-empty strings.
type FeatureChoices struct {
	Key     string   `yaml:"key"`
	Prompt  string   `yaml:"prompt"`
	Options []string `yaml:"options"`
}
```

Updated `ClassFeature` struct — add `Choices *FeatureChoices \`yaml:"choices"\`` after `Description`.

Add `LoadClassFeaturesFromBytes` function:

```go
// LoadClassFeaturesFromBytes parses class features from raw YAML bytes.
//
// Precondition: data must be valid YAML matching the classFeaturesFile schema.
// Postcondition: Returns all class features or a non-nil error.
func LoadClassFeaturesFromBytes(data []byte) ([]*ClassFeature, error) {
	var f classFeaturesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing class features: %w", err)
	}
	return f.ClassFeatures, nil
}
```

**Step 4: Add `Choices` field and `LoadFeatsFromBytes` to `internal/game/ruleset/feat.go`**

Add `Choices *FeatureChoices \`yaml:"choices"\`` to the `Feat` struct (after `Description`). Note: `FeatureChoices` is defined in `class_feature.go` — both files are in the same `ruleset` package, so no import is needed.

Add `LoadFeatsFromBytes`:

```go
// LoadFeatsFromBytes parses feats from raw YAML bytes.
//
// Precondition: data must be valid YAML matching the featsFile schema.
// Postcondition: Returns all feats or a non-nil error.
func LoadFeatsFromBytes(data []byte) ([]*Feat, error) {
	var f featsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing feats: %w", err)
	}
	return f.Feats, nil
}
```

**Step 5: Run tests to verify they pass**

```
go test ./internal/game/ruleset/...
```

Expected: PASS

**Step 6: Commit**

```
git add internal/game/ruleset/class_feature.go internal/game/ruleset/feat.go internal/game/ruleset/feature_choices_test.go
git commit -m "feat(ruleset): add FeatureChoices struct and Choices field to ClassFeature and Feat"
```

---

## Task 2 — DB migration 015

**Files:**
- Create: `migrations/015_character_feature_choices.up.sql`
- Create: `migrations/015_character_feature_choices.down.sql`

**Step 1: Create up migration**

`migrations/015_character_feature_choices.up.sql`:

```sql
CREATE TABLE character_feature_choices (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id    TEXT    NOT NULL,
    choice_key    TEXT    NOT NULL,
    value         TEXT    NOT NULL,
    PRIMARY KEY (character_id, feature_id, choice_key)
);

INSERT INTO character_feature_choices (character_id, feature_id, choice_key, value)
SELECT character_id, 'predators_eye', 'favored_target', target_type
FROM character_favored_target;

DROP TABLE character_favored_target;
```

**Step 2: Create down migration**

`migrations/015_character_feature_choices.down.sql`:

```sql
CREATE TABLE character_favored_target (
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    target_type   TEXT   NOT NULL,
    PRIMARY KEY (character_id)
);

INSERT INTO character_favored_target (character_id, target_type)
SELECT character_id, value
FROM character_feature_choices
WHERE feature_id = 'predators_eye' AND choice_key = 'favored_target';

DROP TABLE character_feature_choices;
```

**Step 3: Commit**

```
git add migrations/015_character_feature_choices.up.sql migrations/015_character_feature_choices.down.sql
git commit -m "feat(migration): add character_feature_choices (015), migrate favored_target data, drop old table"
```

---

## Task 3 — `CharacterFeatureChoicesRepo`

**Files:**
- Create: `internal/storage/postgres/character_feature_choices.go`
- Create: `internal/storage/postgres/character_feature_choices_test.go`
- Modify: `internal/testutil/postgres.go` — add `ApplyFeatureChoicesMigration`

**Step 1: Add `ApplyFeatureChoicesMigration` to testutil**

In `internal/testutil/postgres.go`, add after the existing `Apply*Migration` methods:

```go
// ApplyFeatureChoicesMigration adds the character_feature_choices table for tests.
//
// Precondition: Pool connected; characters table exists.
// Postcondition: character_feature_choices table exists.
func (pc *PostgresContainer) ApplyFeatureChoicesMigration(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	start := time.Now()
	_, err := pc.RawPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS character_feature_choices (
			character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			feature_id    TEXT    NOT NULL,
			choice_key    TEXT    NOT NULL,
			value         TEXT    NOT NULL,
			PRIMARY KEY (character_id, feature_id, choice_key)
		);
	`)
	if err != nil {
		t.Fatalf("applying feature choices migration: %v", err)
	}
	t.Logf("feature choices migration applied [%s]", time.Since(start))
}
```

**Step 2: Write failing tests**

Create `internal/storage/postgres/character_feature_choices_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
)

func testDBWithFeatureChoices(t *testing.T) *testutil.PostgresContainer {
	t.Helper()
	pc := testutil.NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	pc.ApplyFeatureChoicesMigration(t)
	return pc
}

func TestCharacterFeatureChoicesRepo_GetAll_EmptyForNew(t *testing.T) {
	pc := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pc.RawPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCharacterFeatureChoicesRepo_Set_And_GetAll(t *testing.T) {
	pc := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pc.RawPool)

	err := repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "human")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Contains(t, got, "predators_eye")
	assert.Equal(t, "human", got["predators_eye"]["favored_target"])
}

func TestCharacterFeatureChoicesRepo_Set_IsIdempotent(t *testing.T) {
	pc := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pc.RawPool)

	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "robot"))
	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "mutant"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "mutant", got["predators_eye"]["favored_target"])
}

func TestCharacterFeatureChoicesRepo_MultipleFeatures(t *testing.T) {
	pc := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pc.RawPool)

	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "animal"))
	require.NoError(t, repo.Set(ctx, ch.ID, "weapon_focus", "weapon_group", "rifle"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "animal", got["predators_eye"]["favored_target"])
	assert.Equal(t, "rifle", got["weapon_focus"]["weapon_group"])
}

func TestPropertyCharacterFeatureChoicesRepo_RoundTrip(t *testing.T) {
	pc := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pc.RawPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		featureID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "featureID")
		choiceKey := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "choiceKey")
		value := rapid.StringMatching(`[a-z]{1,32}`).Draw(rt, "value")

		if err := repo.Set(ctx, ch.ID, featureID, choiceKey, value); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		if got[featureID][choiceKey] != value {
			rt.Fatalf("value mismatch: got %q want %q", got[featureID][choiceKey], value)
		}
	})
}
```

**Step 3: Run tests to verify they fail**

```
go test ./internal/storage/postgres/... -run TestCharacterFeatureChoicesRepo
```

Expected: FAIL — `pgstore.NewCharacterFeatureChoicesRepo undefined`

**Step 4: Implement the repo**

Create `internal/storage/postgres/character_feature_choices.go`:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFeatureChoicesRepo persists and retrieves per-character feature choices.
type CharacterFeatureChoicesRepo struct {
	db *pgxpool.Pool
}

// NewCharacterFeatureChoicesRepo constructs a CharacterFeatureChoicesRepo.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repo.
func NewCharacterFeatureChoicesRepo(db *pgxpool.Pool) *CharacterFeatureChoicesRepo {
	return &CharacterFeatureChoicesRepo{db: db}
}

// GetAll returns all stored choices for characterID as a nested map:
// feature_id → choice_key → value.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterFeatureChoicesRepo) GetAll(ctx context.Context, characterID int64) (map[string]map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT feature_id, choice_key, value
		 FROM character_feature_choices
		 WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll: %w", err)
	}
	defer rows.Close()

	out := make(map[string]map[string]string)
	for rows.Next() {
		var featureID, choiceKey, value string
		if err := rows.Scan(&featureID, &choiceKey, &value); err != nil {
			return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll scan: %w", err)
		}
		if out[featureID] == nil {
			out[featureID] = make(map[string]string)
		}
		out[featureID][choiceKey] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll rows: %w", err)
	}
	return out, nil
}

// Set upserts a single choice for characterID.
//
// Precondition: characterID > 0; featureID, choiceKey, and value must be non-empty.
// Postcondition: Exactly one row exists for (character_id, feature_id, choice_key) with the given value.
func (r *CharacterFeatureChoicesRepo) Set(ctx context.Context, characterID int64, featureID, choiceKey, value string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_feature_choices (character_id, feature_id, choice_key, value)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (character_id, feature_id, choice_key) DO UPDATE SET value = EXCLUDED.value`,
		characterID, featureID, choiceKey, value,
	)
	if err != nil {
		return fmt.Errorf("CharacterFeatureChoicesRepo.Set: %w", err)
	}
	return nil
}
```

**Step 5: Run tests to verify they pass**

```
go test ./internal/storage/postgres/...
```

Expected: PASS

**Step 6: Commit**

```
git add internal/storage/postgres/character_feature_choices.go internal/storage/postgres/character_feature_choices_test.go internal/testutil/postgres.go
git commit -m "feat(storage): add CharacterFeatureChoicesRepo with GetAll and Set; TDD with property tests"
```

---

## Task 4 — `PlayerSession.FeatureChoices`

**Files:**
- Modify: `internal/game/session/manager.go`

**Step 1: Add `FeatureChoices` field to `PlayerSession`**

In the `PlayerSession` struct, after the `FavoredTarget` field, add:

```go
// FeatureChoices maps feature_id → choice_key → selected value.
// Populated at login from character_feature_choices table.
FeatureChoices map[string]map[string]string
```

Update the `FavoredTarget` comment:

```go
// FavoredTarget is the NPC type favored by the predators_eye class feature.
// Populated after the generic feature-choice loop from
// FeatureChoices["predators_eye"]["favored_target"].
FavoredTarget string
```

**Step 2: Build to verify compilation**

```
go build ./internal/game/session/...
```

**Step 3: Commit**

```
git add internal/game/session/manager.go
git commit -m "feat(session): add FeatureChoices map to PlayerSession"
```

---

## Task 5 — Generic choice-resolution loop in `grpc_service.go`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Delete: `internal/storage/postgres/character_favored_target.go`
- Delete: `internal/storage/postgres/character_favored_target_test.go`

**Step 1: Replace `FavoredTargetRepository` interface with `CharacterFeatureChoicesRepository`**

In `grpc_service.go`, find the `FavoredTargetRepository` interface (approximately line 105–112). Replace it with:

```go
// CharacterFeatureChoicesRepository persists and retrieves per-character feature choices.
//
// Precondition: characterID must be > 0.
// Postcondition: GetAll returns a non-nil map; Set durably persists the choice.
type CharacterFeatureChoicesRepository interface {
	GetAll(ctx context.Context, characterID int64) (map[string]map[string]string, error)
	Set(ctx context.Context, characterID int64, featureID, choiceKey, value string) error
}
```

In the `GameServiceServer` struct, replace `favoredTargetRepo FavoredTargetRepository` with:

```go
featureChoicesRepo CharacterFeatureChoicesRepository
```

In `NewGameServiceServer`, replace the `favoredTargetRepo FavoredTargetRepository` parameter with:

```go
featureChoicesRepo CharacterFeatureChoicesRepository,
```

In the struct initializer, replace `favoredTargetRepo: favoredTargetRepo,` with:

```go
featureChoicesRepo: featureChoicesRepo,
```

**Step 2: Delete the hand-coded predators_eye block**

Find and delete the block starting with `// Load favored target for predators_eye` through the end of the `predators_eye` prompt handling (approximately lines 516–588).

**Step 3: Insert the generic choice-resolution loop**

Insert the following immediately after the `sess.PassiveFeats` population block:

```go
// Load stored feature choices and resolve any missing ones interactively.
sess.FeatureChoices = make(map[string]map[string]string)
if characterID > 0 && s.featureChoicesRepo != nil {
	stored, fcErr := s.featureChoicesRepo.GetAll(stream.Context(), characterID)
	if fcErr != nil {
		s.logger.Warn("loading feature choices", zap.Int64("character_id", characterID), zap.Error(fcErr))
	} else {
		sess.FeatureChoices = stored
	}

	if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
		cfIDs, cfErr2 := s.characterClassFeaturesRepo.GetAll(stream.Context(), characterID)
		if cfErr2 != nil {
			s.logger.Warn("loading class features for choice resolution", zap.Error(cfErr2))
		} else {
			for _, id := range cfIDs {
				cf, ok := s.classFeatureRegistry.ClassFeature(id)
				if !ok || cf.Choices == nil {
					continue
				}
				if sess.FeatureChoices[id] != nil && sess.FeatureChoices[id][cf.Choices.Key] != "" {
					continue
				}
				chosen, promptErr := s.promptFeatureChoice(stream, id, cf.Choices)
				if promptErr != nil {
					s.logger.Warn("prompting feature choice", zap.String("feature", id), zap.Error(promptErr))
					continue
				}
				if chosen == "" {
					continue
				}
				if setErr := s.featureChoicesRepo.Set(stream.Context(), characterID, id, cf.Choices.Key, chosen); setErr != nil {
					s.logger.Warn("persisting feature choice", zap.String("feature", id), zap.Error(setErr))
					continue
				}
				if sess.FeatureChoices[id] == nil {
					sess.FeatureChoices[id] = make(map[string]string)
				}
				sess.FeatureChoices[id][cf.Choices.Key] = chosen
			}
		}
	}

	if s.characterFeatsRepo != nil && s.featRegistry != nil {
		featIDs, featErr := s.characterFeatsRepo.GetAll(stream.Context(), characterID)
		if featErr != nil {
			s.logger.Warn("loading feats for choice resolution", zap.Error(featErr))
		} else {
			for _, id := range featIDs {
				f, ok := s.featRegistry.Feat(id)
				if !ok || f.Choices == nil {
					continue
				}
				if sess.FeatureChoices[id] != nil && sess.FeatureChoices[id][f.Choices.Key] != "" {
					continue
				}
				chosen, promptErr := s.promptFeatureChoice(stream, id, f.Choices)
				if promptErr != nil {
					s.logger.Warn("prompting feat choice", zap.String("feat", id), zap.Error(promptErr))
					continue
				}
				if chosen == "" {
					continue
				}
				if setErr := s.featureChoicesRepo.Set(stream.Context(), characterID, id, f.Choices.Key, chosen); setErr != nil {
					s.logger.Warn("persisting feat choice", zap.String("feat", id), zap.Error(setErr))
					continue
				}
				if sess.FeatureChoices[id] == nil {
					sess.FeatureChoices[id] = make(map[string]string)
				}
				sess.FeatureChoices[id][f.Choices.Key] = chosen
			}
		}
	}

	// Populate derived fields from FeatureChoices.
	sess.FavoredTarget = sess.FeatureChoices["predators_eye"]["favored_target"]
}
```

**Step 4: Add `promptFeatureChoice` helper**

Add the following method to `grpc_service.go` (near other helper methods):

```go
// promptFeatureChoice sends a numbered prompt for the given FeatureChoices block
// over stream and reads a single numeric response.
//
// Precondition: stream must be writable; choices must be non-nil with non-empty Options.
// Postcondition: Returns one of choices.Options, or "" on invalid input or recv failure.
func (s *GameServiceServer) promptFeatureChoice(
	stream gamev1.GameService_SessionServer,
	featureID string,
	choices *ruleset.FeatureChoices,
) (string, error) {
	var sb strings.Builder
	sb.WriteString(choices.Prompt)
	sb.WriteString("\n")
	for i, opt := range choices.Options {
		fmt.Fprintf(&sb, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(&sb, "Enter 1-%d:", len(choices.Options))

	if err := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: sb.String()},
		},
	}); err != nil {
		return "", fmt.Errorf("sending choice prompt for %s: %w", featureID, err)
	}

	msg, err := stream.Recv()
	if err != nil {
		return "", fmt.Errorf("receiving choice for %s: %w", featureID, err)
	}

	selText := ""
	if say := msg.GetSay(); say != nil {
		selText = strings.TrimSpace(say.GetMessage())
	}

	n := 0
	idx := -1
	if _, scanErr := fmt.Sscanf(selText, "%d", &n); scanErr == nil && n >= 1 && n <= len(choices.Options) {
		idx = n - 1
	}

	if idx < 0 {
		s.logger.Warn("invalid feature choice selection",
			zap.String("feature", featureID),
			zap.String("input", selText),
		)
		_ = stream.Send(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: "Invalid selection. You will be prompted again on next login."},
			},
		})
		return "", nil
	}

	chosen := choices.Options[idx]
	_ = stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: choices.Key + " set to: " + chosen},
		},
	})
	return chosen, nil
}
```

**Step 5: Delete old files**

```
git rm internal/storage/postgres/character_favored_target.go
git rm internal/storage/postgres/character_favored_target_test.go
```

**Step 6: Build and test**

```
go build ./...
go test ./internal/gameserver/...
```

Expected: PASS

**Step 7: Commit**

```
git add -A
git commit -m "feat(gameserver): replace hand-coded predators_eye block with generic feature-choice loop"
```

---

## Task 6 — Backfill gap fix in `character_flow.go`

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Create: `internal/frontend/handlers/character_flow_feats_test.go`

**Step 1: Write failing tests**

Create `internal/frontend/handlers/character_flow_feats_test.go`:

```go
package handlers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

func TestFeatPoolDeficit_NoExistingFeats(t *testing.T) {
	pool := []string{"fleet", "toughness", "quick_dodge"}
	stored := map[string]bool{}
	assert.Equal(t, 2, handlers.FeatPoolDeficit(pool, stored, 2))
}

func TestFeatPoolDeficit_AllFeatsPresent(t *testing.T) {
	pool := []string{"fleet", "toughness"}
	stored := map[string]bool{"fleet": true, "toughness": true}
	assert.Equal(t, 0, handlers.FeatPoolDeficit(pool, stored, 2))
}

func TestFeatPoolDeficit_PartialFeats(t *testing.T) {
	pool := []string{"fleet", "toughness", "quick_dodge"}
	stored := map[string]bool{"fleet": true}
	assert.Equal(t, 1, handlers.FeatPoolDeficit(pool, stored, 2))
}

func TestFeatPoolDeficit_DeficitNeverNegative(t *testing.T) {
	pool := []string{"fleet", "toughness"}
	stored := map[string]bool{"fleet": true, "toughness": true, "extra": true}
	assert.Equal(t, 0, handlers.FeatPoolDeficit(pool, stored, 1))
}

func TestPropertyFeatPoolDeficit(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		poolSize := rapid.IntRange(1, 10).Draw(rt, "poolSize")
		count := rapid.IntRange(1, poolSize).Draw(rt, "count")
		storedCount := rapid.IntRange(0, poolSize).Draw(rt, "storedCount")

		pool := make([]string, poolSize)
		seen := map[string]bool{}
		for i := range pool {
			id := rapid.StringMatching(`[a-z]{3,16}`).Draw(rt, "feat")
			pool[i] = id
			if seen[id] {
				return // skip — duplicates break clean counting
			}
			seen[id] = true
		}

		storedMap := map[string]bool{}
		for i := 0; i < storedCount && i < poolSize; i++ {
			storedMap[pool[i]] = true
		}

		got := handlers.FeatPoolDeficit(pool, storedMap, count)
		want := count - storedCount
		if want < 0 {
			want = 0
		}
		if got != want {
			rt.Fatalf("FeatPoolDeficit = %d, want %d", got, want)
		}
	})
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/frontend/handlers/... -run TestFeatPoolDeficit
```

Expected: FAIL — `handlers.FeatPoolDeficit undefined`

**Step 3: Add `FeatPoolDeficit` function to `character_flow.go`**

At the top of `internal/frontend/handlers/character_flow.go`, after the import block, add:

```go
// FeatPoolDeficit returns how many feats from pool the character still needs to choose.
// It counts how many pool members are present in storedFeats and subtracts from count.
//
// Precondition: count >= 0; pool and storedFeats must not be nil.
// Postcondition: Returns max(0, count - storedFromPool).
func FeatPoolDeficit(pool []string, storedFeats map[string]bool, count int) int {
	have := 0
	for _, id := range pool {
		if storedFeats[id] {
			have++
		}
	}
	deficit := count - have
	if deficit < 0 {
		return 0
	}
	return deficit
}
```

**Step 4: Replace `HasFeats()` gate in `ensureFeats()`**

In `ensureFeats()` (lines 593–683), replace the early-return check:

```go
// OLD — skip if any feats exist:
has, err := h.characterFeats.HasFeats(ctx, char.ID)
if err != nil { ... }
if has { return nil }
```

with a per-pool deficit approach:

```go
// Load all currently stored feat IDs for this character.
storedIDs, err := h.characterFeats.GetAll(ctx, char.ID)
if err != nil {
	h.logger.Warn("loading stored feats for backfill", zap.Int64("id", char.ID), zap.Error(err))
	return nil
}
storedSet := make(map[string]bool, len(storedIDs))
for _, id := range storedIDs {
	storedSet[id] = true
}
```

Then replace each choice-loop invocation to use `FeatPoolDeficit`:

For the job feat choices pool:
```go
if job.FeatGrants != nil && job.FeatGrants.Choices != nil && job.FeatGrants.Choices.Count > 0 {
	deficit := FeatPoolDeficit(job.FeatGrants.Choices.Pool, storedSet, job.FeatGrants.Choices.Count)
	if deficit > 0 {
		chosen, choiceErr := h.featChoiceLoop(ctx, conn, "Choose a job feat", job.FeatGrants.Choices.Pool, deficit)
		// ... persist and update storedSet
	}
}
```

Apply the same pattern to the general feats pool and skill feats pool, adjusting `deficit` computation for each.

**Step 5: Verify the `CharacterFeatsInterface` in `AuthHandler` has `GetAll`**

Run:

```bash
grep -n "characterFeats\b" /home/cjohannsen/src/mud/internal/frontend/handlers/character_flow.go | head -5
grep -n "HasFeats\|GetAll\|SetAll" /home/cjohannsen/src/mud/internal/frontend/handlers/character_flow.go | head -10
```

If the field type only has `HasFeats` and `SetAll`, add `GetAll(ctx context.Context, characterID int64) ([]string, error)` to the interface definition.

**Step 6: Run tests**

```
go test ./internal/frontend/handlers/...
```

Expected: PASS

**Step 7: Commit**

```
git add internal/frontend/handlers/character_flow.go internal/frontend/handlers/character_flow_feats_test.go
git commit -m "fix(character-flow): replace HasFeats gate with per-pool deficit check; add FeatPoolDeficit"
```

---

## Task 7 — Update `predators_eye` YAML

**Files:**
- Modify: `content/class_features.yaml`

**Step 1: Find and update the `predators_eye` entry**

```bash
grep -n "predators_eye" /home/cjohannsen/src/mud/content/class_features.yaml
```

Replace the `predators_eye` entry with:

```yaml
  - id: predators_eye
    name: Predator's Eye
    archetype: drifter
    job: ""
    pf2e: hunters_edge
    active: false
    activate_text: ""
    description: "Choose a favored target type at character creation; you deal +1d8 precision damage against targets of that type."
    choices:
      key: favored_target
      prompt: "You have the Predator's Eye ability. Choose your favored target type"
      options: [human, robot, animal, mutant]
```

**Step 2: Verify**

```
go build ./...
go test ./internal/game/ruleset/...
```

**Step 3: Commit**

```
git add content/class_features.yaml
git commit -m "feat(content): add choices block to predators_eye class feature YAML"
```

---

## Task 8 — Wire `CharacterFeatureChoicesRepo` into `GameServiceServer`

**Files:**
- Modify: `cmd/gameserver/main.go`

**Step 1: Find where `favoredTargetRepo` was wired**

```bash
grep -n "favoredTargetRepo\|FavoredTargetRepo" /home/cjohannsen/src/mud/cmd/gameserver/main.go
```

**Step 2: Replace with `featureChoicesRepo`**

Replace:
```go
favoredTargetRepo := postgres.NewCharacterFavoredTargetRepo(pool.DB())
```
with:
```go
featureChoicesRepo := postgres.NewCharacterFeatureChoicesRepo(pool.DB())
```

In `NewGameServiceServer(...)`, replace `favoredTargetRepo` with `featureChoicesRepo`.

**Step 3: Update any test doubles referencing `FavoredTargetRepository`**

```bash
grep -rn "FavoredTargetRepository\|favoredTargetRepo\|NewGameServiceServer" /home/cjohannsen/src/mud --include="*_test.go"
```

For each test double, implement the new `CharacterFeatureChoicesRepository` interface:

```go
type noopFeatureChoicesRepo struct{}

func (n *noopFeatureChoicesRepo) GetAll(_ context.Context, _ int64) (map[string]map[string]string, error) {
	return make(map[string]map[string]string), nil
}
func (n *noopFeatureChoicesRepo) Set(_ context.Context, _ int64, _, _, _ string) error {
	return nil
}
```

**Step 4: Build and test**

```
go build ./...
go test ./...
```

Expected: all packages PASS

**Step 5: Commit**

```
git add cmd/gameserver/main.go
git add -u  # catch any test double updates
git commit -m "feat(wire): replace favoredTargetRepo with featureChoicesRepo in GameServiceServer wiring"
```

---

## Task 9 — Full test and deploy

**Step 1: Full build and test**

```
go build ./...
go test ./...
```

Expected: all packages PASS

**Step 2: Deploy**

```
make k8s-redeploy DB_PASSWORD=mud
```

**Step 3: Update FEATURES.md**

Mark the item complete in `docs/requirements/FEATURES.md`:

```markdown
- [x] During character creation, if the player must choose between multiple feat/feature options they should be prompted to select from a list; existing characters checked at login for missing choices
```

**Step 4: Final commit**

```
git add docs/requirements/FEATURES.md
git commit -m "docs: mark feat/feature choice prompt complete in FEATURES.md"
```

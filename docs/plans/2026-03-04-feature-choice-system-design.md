# Feature Choice System Design

**Date:** 2026-03-04

## Goal

Generalize per-feature sub-choices (currently hand-coded for `predators_eye`) into a YAML-declared, generically-stored system. Fix the existing backfill gap where characters with partial feats skip the optional-choice prompt.

---

## Section 1: Data Model

### YAML `choices` block (class features and feats)

```yaml
id: predators_eye
choices:
  key: favored_target
  prompt: "Choose your favored target type"
  options: [human, robot, animal, mutant]
```

- `key`: storage key — unique per feature/feat entry
- `prompt`: player-facing prompt text
- `options`: fixed list of valid string values

### Go struct added to `ruleset.ClassFeature` and `ruleset.Feat`

```go
type FeatureChoices struct {
    Key     string   `yaml:"key"`
    Prompt  string   `yaml:"prompt"`
    Options []string `yaml:"options"`
}
```

### New DB table (migration 015)

```sql
CREATE TABLE character_feature_choices (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id    TEXT    NOT NULL,
    choice_key    TEXT    NOT NULL,
    value         TEXT    NOT NULL,
    PRIMARY KEY (character_id, feature_id, choice_key)
);
```

### `PlayerSession` additions

```go
// FeatureChoices maps feature_id → choice_key → selected value.
FeatureChoices map[string]map[string]string
```

`FavoredTarget` remains on `PlayerSession` for backward-compat with combat code, populated from `FeatureChoices["predators_eye"]["favored_target"]` after the generic loop.

---

## Section 2: Runtime Flow

### Login / character selection (`grpc_service.go`)

After passive feats are loaded into `sess.PassiveFeats`:

1. Load all rows from `character_feature_choices` for `characterID` into `sess.FeatureChoices`
2. Iterate all class features held by the character; for each with a `choices` block: if `sess.FeatureChoices[featureID][choiceKey]` is empty, prompt the player, persist to DB, update `sess.FeatureChoices`
3. Repeat step 2 for all feats held by the character
4. After all choices resolved, populate derived fields: `sess.FavoredTarget = sess.FeatureChoices["predators_eye"]["favored_target"]`

The prompt uses the same `stream.Send` / `stream.Recv` pattern as the existing `predators_eye` prompt.

### Backfill gap fix (`character_flow.go` — `ensureFeats()`)

Replace the `HasFeats()` gate with a per-pool count check:
- For each `feats.choices` pool in the job: count how many feats from that pool are already stored
- If stored count < `pool.count`, run `featChoiceLoop()` for the deficit

This correctly handles characters who have fixed feats but are missing their optional pool selections.

---

## Section 3: Migration of `predators_eye`

1. **Migration 015** creates `character_feature_choices`, copies existing `character_favored_target` rows:
   ```sql
   INSERT INTO character_feature_choices (character_id, feature_id, choice_key, value)
   SELECT character_id, 'predators_eye', 'favored_target', target_type
   FROM character_favored_target;
   DROP TABLE character_favored_target;
   ```

2. **`content/class_features.yaml`** — `predators_eye` gains the `choices` block.

3. **`grpc_service.go`** — hand-coded `predators_eye` prompt block deleted; replaced by generic choice-resolution loop.

4. **`FavoredTargetRepository`** interface and `favoredTargetRepo` field on `GameServiceServer` removed.

5. **`internal/storage/postgres/character_favored_target.go`** deleted.

---

## Out of Scope

- Choice types other than "pick one from a fixed string list"
- Multiple selections (`count > 1`) within a single feature's choices block
- Feat-pool choices (job-level `feats.choices`) — only the backfill gap fix applies there

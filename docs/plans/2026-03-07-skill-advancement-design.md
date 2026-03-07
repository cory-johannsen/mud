# Skill Advancement Design

## Goal

Players advance skill proficiency ranks by spending skill increases granted at every even character level.

## Decisions

- **Grant schedule**: every even level (L2, L4, L6 ... L100) — one increase per even level
- **Rank progression**: untrained → trained → expert → master → legendary
- **Level gates** (proportionally scaled from PF2E L3/7/15 out of 20 to max L100):
  - expert: character level 15+
  - master: character level 35+
  - legendary: character level 75+
- **Assignment**: deferred — pending increases stored, assigned via `trainskill <skill>` command
- **Gate enforcement**: strictly enforced at assignment time

---

## Data Model

### Migration 021

```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS pending_skill_increases INTEGER NOT NULL DEFAULT 0;
```

No changes to `character_skills` — the existing `(character_id, skill_id, proficiency)` schema is sufficient.

### New Repository Methods

**`CharacterRepository`**:
- `IncrementPendingSkillIncreases(ctx, characterID)` — called at level-up on even levels
- `ConsumePendingSkillIncrease(ctx, characterID)` — called by `trainskill`; mirrors `ConsumePendingBoost`

**`CharacterSkillsRepository`**:
- `UpgradeSkill(ctx, characterID, skillID, newRank string)` — upserts a single skill row

### Proto

Add `pending_skill_increases int32` to `CharacterInfo` message.

### Session

`PlayerSession.SkillRanks map[string]string` is already loaded at login. `trainskill` updates it in-memory after DB success.

---

## Level-up Integration

In `grpc_service.go`, at level-up:
1. If new level is even → call `IncrementPendingSkillIncreases`
2. Include the updated `pending_skill_increases` in the `CharacterInfo` push event
3. Level-up message includes: "You gained a skill increase! Type `trainskill <skill>` to advance a skill."

### Backfill at Login

If `pending_skill_increases == 0 && floor(level/2) > 0`, set `pending_skill_increases = floor(level/2)`. This gives existing characters the increases they have earned. Job-granted starting skills already exist in `character_skills` and do not consume pending increases.

---

## `trainskill` Command (CMD-1–7)

### Pure handler (`internal/game/command/trainskill.go`)

```go
func HandleTrainSkill(args []string, currentRank string, level int) (string, error)
```

- Validates skill ID exists in the 17-skill catalog
- Computes next rank from current rank
- Enforces level gate
- Returns new rank string on success

### Proto

```proto
message TrainSkillRequest {
  string skill_id = 1;
}
```

Added to `ClientMessage` oneof.

### Bridge (`bridge_handlers.go`)

`bridgeTrainSkill` calls `HandleTrainSkill` with the player's current rank (from session) and level (from session), sends `TrainSkillRequest` proto on success.

### gRPC Handler (`grpc_service.go`)

`handleTrainSkill`:
1. Validate pending count > 0
2. Call `UpgradeSkill` (persist rank change)
3. Call `ConsumePendingSkillIncrease`
4. Mutate `sess.SkillRanks[skillID] = newRank`
5. Send confirmation text event: "You advanced Parkour from trained to expert."
6. Push updated `CharacterInfo` event

Persistence before session mutation — same ordering as `handleLevelUp`.

---

## Display

### Character Sheet

Progress section (already shows XP and pending ability boosts):

```
Pending Skill Increases: N   (type 'trainskill <skill>' to assign)
```

Hint shown only when N > 0.

### `skills` Command

No change — rank and numeric bonus already display. The pending count hint lives on the character sheet only.

---

## Testing

- TDD with property-based tests (`pgregory.net/rapid`) for all new repo methods
- `HandleTrainSkill` unit tests: valid advancement, invalid skill, rank-gated rejection, max-rank rejection
- `handleTrainSkill` server tests: pending=0 blocks, persistence failure rolls back session, happy path
- Backfill logic tested in isolation

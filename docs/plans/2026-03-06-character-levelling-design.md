# Character Levelling Design

## Goal

Implement XP-based character levelling with three XP sources (combat, exploration, skill checks), geometric XP curve, automatic level-up with deferred ability boost selection, and configurable XP/HP values.

## XP Sources

| Source | Award |
|--------|-------|
| Kill NPC | `npc_level × kill_xp_per_npc_level` |
| Discover new room | `new_room_xp` (flat) |
| Skill check success | `skill_check_success_xp + DC × skill_check_dc_multiplier` |
| Skill check crit_success | `skill_check_crit_success_xp + DC × skill_check_dc_multiplier` |

## XP Curve

```
xp_to_reach_level(n) = n² × base_xp
```

With `base_xp = 100`: level 2 requires 400 XP, level 3 requires 900 XP, level 10 requires 10,000 XP, level 100 requires 1,000,000 XP.

`LevelForXP(xp, baseXP)` walks levels until the next threshold would be exceeded, capped at `level_cap`.

## Level-Up Effects

On each level-up:
- `max_hp += hp_per_level` (configurable, default 5)
- `CombatProficiencyBonus(level, rank)` improves automatically — already uses `level` field
- Every `boost_interval` levels (default 5): one pending ability boost is recorded

Ability boost: player picks one of the 6 abilities via the `levelup` command; `+2` applied to that ability score.

## Level-Up Flow

1. XP awarded → `Award()` detects threshold crossed → level incremented, max_hp increased
2. `SaveProgress` persists `level`, `experience`, `max_hp` immediately
3. Player receives: `"*** You reached level N! ***"`
4. If boost pending: `"Type 'levelup' to assign your ability boost."`
5. Player types `levelup` → prompted to choose ability → boost stored in DB

## Configuration (`content/xp_config.yaml`)

```yaml
base_xp: 100
hp_per_level: 5
boost_interval: 5
level_cap: 100
job_level_cap: 20

awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
```

## Data Model

### Existing (no change needed)
- `characters.level INT` — already exists
- `characters.experience INT` — already exists

### New migration
```sql
CREATE TABLE IF NOT EXISTS character_pending_boosts (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    count        INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id)
);
```

### Extended `SaveState` → new `SaveProgress`
New repo method persists `level`, `experience`, `max_hp`, and upserts `character_pending_boosts`.

## Components

### `internal/game/xp/` (new package)
- `xp.go` — pure functions: `XPToLevel`, `LevelForXP`, `Award`
- `config.go` — `XPConfig` struct loaded from `content/xp_config.yaml`
- `xp_test.go` — property-based + unit tests

### `internal/storage/postgres/character_progress.go` (new)
- `SaveProgress(ctx, id, level, experience, maxHP, pendingBoosts int) error`
- `GetPendingBoosts(ctx, id int64) (int, error)`
- `ConsumePendingBoost(ctx, id int64) error`

### `internal/gameserver/` (modified)
- `combat_handler.go` — award kill XP after NPC death
- `grpc_service.go` — award room discovery XP on join; award skill check XP after resolve
- new `levelup_handler.go` — `handleLevelUp` gRPC handler + `Handle<LevelUp>` command (CMD-1 through CMD-7)

### `content/xp_config.yaml` (new)

### `api/proto/game/v1/game.proto` (modified)
- Add `LevelUpRequest` message with `ability` field (one of: brutality/quickness/grit/reasoning/savvy/flair)
- Add to `ClientMessage` oneof

## Integration Points

1. **Combat kill**: `combat_handler.go` after NPC defeat → `xpSvc.AwardKill(sess, npc.Level)`
2. **Room discovery**: `grpc_service.go` room join handler → check automap, if new room → `xpSvc.AwardRoomDiscovery(sess)`
3. **Skill check**: `grpc_service.go` after `ResolveSkillCheck` → if success/crit_success → `xpSvc.AwardSkillCheck(sess, dc, outcome)`

## Testing Strategy

- **Property-based**: `LevelForXP(XPToLevel(n)) == n` for all valid n; `LevelForXP` never exceeds `level_cap`; `Award` never skips multiple levels
- **Unit**: level-up at threshold boundaries; boost triggers at exact `boost_interval` multiples; `levelup` command rejects with no pending boosts
- **Integration**: `SaveProgress` round-trip via shared postgres container
- **Wiring**: `TestAllCommandHandlersAreWired` passes with `HandlerLevelUp`

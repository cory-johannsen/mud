# Spontaneous Technology Use Counts — Design Spec

**Date:** 2026-03-17

---

## Goal

Add daily use-count tracking for spontaneous technologies. Each tech level has a shared use pool (PF2E spell-slot model). Using a spontaneous tech consumes one use from its level's pool. Pools are restored on rest. State is persisted so server restarts do not reset uses.

---

## Context

`character_spontaneous_technologies` tracks which techs are known at which level. `SpontaneousGrants.UsesByLevel` defines the daily pool per level. Neither session nor DB currently tracks uses remaining. `handleUse` handles prepared techs but ignores spontaneous. `handleRest` does not restore spontaneous pools.

**Out of scope:** Spontaneous tech level-up selection (Sub-project B); effect resolution; Heightened/Amped tech.

---

## Feature 1: DB Migration

New migration `migrations/028_spontaneous_use_pools.up.sql`:

```sql
CREATE TABLE character_spontaneous_use_pools (
    character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    tech_level     INT    NOT NULL,
    uses_remaining INT    NOT NULL DEFAULT 0,
    max_uses       INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_level)
);
```

Down migration `migrations/028_spontaneous_use_pools.down.sql`:

```sql
DROP TABLE character_spontaneous_use_pools;
```

`max_uses` is stored alongside `uses_remaining` so rest can restore without recomputing grants. This mirrors the `innate_technologies.max_uses` pattern.

---

## Feature 2: Session type and field

Add to `internal/game/session/technology.go`:

```go
// UsePool tracks remaining and maximum daily uses for a spontaneous tech level.
type UsePool struct {
    Remaining int
    Max       int
}
```

Add field to `PlayerSession`:

```go
// SpontaneousUsePools tracks daily use pools per tech level.
// Key: tech level (1-based). Value: UsePool with remaining and max uses.
SpontaneousUsePools map[int]UsePool
```

`UsePool` is defined in the session package so no cross-package import is needed. All features below use `map[int]UsePool` consistently.

---

## Feature 3: `SpontaneousUsePoolRepo`

New interface in `internal/gameserver/technology_assignment.go`:

```go
// SpontaneousUsePoolRepo manages the daily use pool for spontaneous technologies.
//
// Precondition: characterID > 0; techLevel >= 1; uses >= 0.
type SpontaneousUsePoolRepo interface {
    // GetAll returns all use pools for the character.
    // Postcondition: returned map contains one UsePool per initialized tech level.
    GetAll(ctx context.Context, characterID int64) (map[int]session.UsePool, error)

    // Set initializes or overwrites a pool entry.
    // Postcondition: row (characterID, techLevel) has uses_remaining=usesRemaining, max_uses=maxUses.
    Set(ctx context.Context, characterID int64, techLevel, usesRemaining, maxUses int) error

    // Decrement atomically decrements uses_remaining by 1 if > 0.
    // Precondition: caller has verified uses_remaining > 0 in session before calling.
    // Postcondition: uses_remaining = max(0, uses_remaining - 1).
    Decrement(ctx context.Context, characterID int64, techLevel int) error

    // RestoreAll sets uses_remaining = max_uses for all rows of this character.
    // Postcondition: all pools are at maximum.
    RestoreAll(ctx context.Context, characterID int64) error

    // DeleteAll removes all pool entries for the character.
    DeleteAll(ctx context.Context, characterID int64) error
}
```

New implementation `CharacterSpontaneousUsePoolRepository` in `internal/storage/postgres/character_spontaneous_use_pool.go`:

```go
func (r *CharacterSpontaneousUsePoolRepository) GetAll(ctx context.Context, characterID int64) (map[int]session.UsePool, error) {
    rows, err := r.db.Query(ctx,
        `SELECT tech_level, uses_remaining, max_uses
           FROM character_spontaneous_use_pools
          WHERE character_id = $1`,
        characterID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    pools := make(map[int]session.UsePool)
    for rows.Next() {
        var level, remaining, max int
        if err := rows.Scan(&level, &remaining, &max); err != nil {
            return nil, err
        }
        pools[level] = session.UsePool{Remaining: remaining, Max: max}
    }
    return pools, rows.Err()
}

func (r *CharacterSpontaneousUsePoolRepository) Set(ctx context.Context, characterID int64, techLevel, usesRemaining, maxUses int) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_spontaneous_use_pools (character_id, tech_level, uses_remaining, max_uses)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (character_id, tech_level)
         DO UPDATE SET uses_remaining = EXCLUDED.uses_remaining, max_uses = EXCLUDED.max_uses`,
        characterID, techLevel, usesRemaining, maxUses)
    return err
}

func (r *CharacterSpontaneousUsePoolRepository) Decrement(ctx context.Context, characterID int64, techLevel int) error {
    _, err := r.db.Exec(ctx,
        `UPDATE character_spontaneous_use_pools
            SET uses_remaining = GREATEST(0, uses_remaining - 1)
          WHERE character_id = $1 AND tech_level = $2`,
        characterID, techLevel)
    return err
}

func (r *CharacterSpontaneousUsePoolRepository) RestoreAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `UPDATE character_spontaneous_use_pools
            SET uses_remaining = max_uses
          WHERE character_id = $1`,
        characterID)
    return err
}

func (r *CharacterSpontaneousUsePoolRepository) DeleteAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `DELETE FROM character_spontaneous_use_pools WHERE character_id = $1`,
        characterID)
    return err
}
```

---

## Feature 4: `AssignTechnologies` update

`AssignTechnologies` gains one parameter: `usePoolRepo SpontaneousUsePoolRepo`.

After filling spontaneous known techs, initialize use pools from merged `grants.Spontaneous.UsesByLevel`:

```go
if grants != nil && grants.Spontaneous != nil {
    if sess.SpontaneousUsePools == nil {
        sess.SpontaneousUsePools = make(map[int]session.UsePool)
    }
    for level, uses := range grants.Spontaneous.UsesByLevel {
        sess.SpontaneousUsePools[level] = session.UsePool{Remaining: uses, Max: uses}
        if err := usePoolRepo.Set(ctx, characterID, level, uses, uses); err != nil {
            return fmt.Errorf("AssignTechnologies: set spontaneous use pool level %d: %w", level, err)
        }
    }
}
```

`GameServiceServer` wires the new repo at initialization.

---

## Feature 4b: `LevelUpTechnologies` update

`LevelUpTechnologies` in `internal/gameserver/technology_assignment.go` also grants spontaneous techs at level-up and must initialize new use pool levels. It gains the same `usePoolRepo SpontaneousUsePoolRepo` parameter.

`ResolvePendingTechGrants` calls `LevelUpTechnologies` and must also gain the `usePoolRepo` parameter to pass it through. The call sites in `grpc_service.go` must be updated to supply the new argument.

Level-up grants are **additive deltas** — they add to the existing pool rather than replacing it. This is the correct model because `AssignTechnologies` (Feature 4) already set the baseline at character creation; each level-up grant adds to that baseline. By contrast, `AssignTechnologies` uses a full-replacement model because there is no prior state.

After processing any spontaneous `UsesByLevel` grants at the levelled-up level:

```go
if levelGrants.Spontaneous != nil {
    if sess.SpontaneousUsePools == nil {
        sess.SpontaneousUsePools = make(map[int]session.UsePool)
    }
    for level, uses := range levelGrants.Spontaneous.UsesByLevel {
        existing := sess.SpontaneousUsePools[level]
        newMax := existing.Max + uses
        newRemaining := existing.Remaining + uses
        sess.SpontaneousUsePools[level] = session.UsePool{Remaining: newRemaining, Max: newMax}
        if err := usePoolRepo.Set(ctx, characterID, level, newRemaining, newMax); err != nil {
            return fmt.Errorf("LevelUpTechnologies: set spontaneous use pool level %d: %w", level, err)
        }
    }
}
```

---

## Feature 5: `LoadTechnologies` update

`LoadTechnologies` gains one parameter: `usePoolRepo SpontaneousUsePoolRepo`.

After loading spontaneous tech assignments, load use pools:

```go
pools, err := usePoolRepo.GetAll(ctx, characterID)
if err != nil {
    return fmt.Errorf("LoadTechnologies: load spontaneous use pools: %w", err)
}
sess.SpontaneousUsePools = pools
```

---

## Feature 6: `handleUse` extension

In `internal/gameserver/grpc_service.go`, extend `handleUse` after the existing prepared-tech path.

### No-arg (list) mode

When `abilityID == ""`, append spontaneous tech entries to the choices list. For each tech level L where `sess.SpontaneousUsePools[L].Remaining > 0`, include each known tech at that level:

```
Mind Spike (2 uses remaining at level 1)
```

Only include levels with at least one use remaining.

### Activation path

When `abilityID != ""` and no feat/class-feature/prepared-tech matched:

1. Collect the keys of `sess.SpontaneousTechs`, sort them ascending, then iterate in order. For each level, check if `techID` is present in the tech list at that level. If the same tech ID appears at multiple levels (currently impossible given Phase 1 content, but future-proof), the first match at the **lowest level with remaining uses** is selected.
2. If not found in any level: return `"You don't know <techID>."`
3. Get the tech's level L from the match.
4. If `sess.SpontaneousUsePools[L].Remaining <= 0`: return `"No level <L> uses remaining."`
5. Otherwise:
   - Call `s.spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, L)`
   - Update session: `pool := sess.SpontaneousUsePools[L]; pool.Remaining--; sess.SpontaneousUsePools[L] = pool`
   - Return `"You activate <techID>. (<N> uses remaining at level <L>.)"`

---

## Feature 7: `handleRest` extension

In `internal/gameserver/grpc_service.go`, extend `handleRest` after `RearrangePreparedTechs`:

```go
if err := s.spontaneousUsePoolRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
    return fmt.Errorf("handleRest: restore spontaneous use pools: %w", err)
}
pools, err := s.spontaneousUsePoolRepo.GetAll(ctx, sess.CharacterID)
if err != nil {
    return fmt.Errorf("handleRest: reload spontaneous use pools: %w", err)
}
sess.SpontaneousUsePools = pools
```

The rest message is unchanged.

---

## Feature 8: Character sheet

Add to `api/proto/game/v1/game.proto`:

```protobuf
message SpontaneousUsePoolView {
    int32 tech_level     = 1;
    int32 uses_remaining = 2;
    int32 max_uses       = 3;
}
```

Add to `CharacterSheetView` (implementer must verify the next available field number after 44 — use the next unused number):

```protobuf
repeated SpontaneousUsePoolView spontaneous_use_pools = <next_available>;
```

Run `make proto` to regenerate.

In `handleChar`, populate from session:

```go
for level, pool := range sess.SpontaneousUsePools {
    view.SpontaneousUsePools = append(view.SpontaneousUsePools, &gamev1.SpontaneousUsePoolView{
        TechLevel:     int32(level),
        UsesRemaining: int32(pool.Remaining),
        MaxUses:       int32(pool.Max),
    })
}
```

Also add schema migration for `main_test.go` in `internal/storage/postgres/main_test.go` — add the `character_spontaneous_use_pools` CREATE TABLE to `applyAllMigrations`.

---

## Testing

All tests use TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-SUC1**: `use <tech>` with uses remaining → activation message with count; DB decremented
- **REQ-SUC2**: `use <tech>` with 0 uses → `"No level N uses remaining."`
- **REQ-SUC3**: `use <tech>` not in spontaneous techs → `"You don't know <tech>."`
- **REQ-SUC4**: `use` (no-arg) includes spontaneous techs with remaining use counts
- **REQ-SUC5**: After `rest`, pools restored to max (session + DB)
- **REQ-SUC6**: Use pool round-trips through DB (`GetAll` returns correct values after `Set` and `Decrement`)
- **REQ-SUC7** (property): For N uses at level L, exactly min(calls, N) uses consumed after that many `use` calls; (N+1)th call returns "no remaining"

---

## Constraints

- One new DB migration (028) — new table `character_spontaneous_use_pools`
- One new repo interface + implementation (`SpontaneousUsePoolRepo`)
- `AssignTechnologies`, `LevelUpTechnologies`, `ResolvePendingTechGrants`, and `LoadTechnologies` signatures each gain one `usePoolRepo` parameter; all call sites in `grpc_service.go` updated
- `handleUse` extended in-place
- `handleRest` extended in-place
- One new proto message (`SpontaneousUsePoolView`) and one new field on `CharacterSheetView`
- `UsePool` struct defined in `internal/game/session/technology.go`
- Effect resolution is out of scope
- Spontaneous tech level-up selection is out of scope (Sub-project B)

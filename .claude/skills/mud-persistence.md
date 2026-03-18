---
name: mud-persistence
description: PostgreSQL persistence — repo interfaces, implementations, migrations
type: reference
---

## Trigger

Invoke when working on: database queries, migration files, repo interface definitions,
connection pool setup, or any task that reads from or writes to PostgreSQL.

## Responsibility Boundary

This layer owns: SQL queries, parameterized statements, row scanning, connection pool
lifecycle, and schema migrations. It does NOT own game rules, business logic, or
domain model validation — those belong in `internal/game/`.

## Key Files

| File | Purpose |
|------|---------|
| `internal/storage/postgres/postgres.go` | `Pool` struct — pgxpool lifecycle, health check |
| `internal/storage/postgres/account.go` | `AccountRepository` — create, authenticate, role management |
| `internal/storage/postgres/character.go` | `CharacterRepository` — core character CRUD plus equipment, inventory, weapon presets |
| `internal/storage/postgres/character_progress.go` | `CharacterProgressRepository` — level/XP/pending boosts |
| `internal/storage/postgres/character_feats.go` | `CharacterFeatsRepository` — feat persistence |
| `internal/storage/postgres/character_skills.go` | `CharacterSkillsRepository` — skill ranks |
| `internal/storage/postgres/character_proficiencies.go` | `CharacterProficienciesRepository` — proficiency persistence |
| `internal/storage/postgres/character_class_features.go` | `CharacterClassFeaturesRepository` — class feature assignments |
| `internal/storage/postgres/character_feature_choices.go` | `CharacterFeatureChoicesRepo` — feature option selections |
| `internal/storage/postgres/character_ability_boosts.go` | `CharacterAbilityBoostsRepository` (interface + impl) — pending boost tracking |
| `internal/storage/postgres/character_hardwired_tech.go` | `CharacterHardwiredTechRepository` — hardwired technology |
| `internal/storage/postgres/character_prepared_tech.go` | `CharacterPreparedTechRepository` — prepared tech slots |
| `internal/storage/postgres/character_spontaneous_tech.go` | `CharacterSpontaneousTechRepository` — spontaneous tech known list |
| `internal/storage/postgres/character_spontaneous_use_pool.go` | `CharacterSpontaneousUsePoolRepository` — per-level use pools |
| `internal/storage/postgres/character_innate_tech.go` | `CharacterInnateTechRepository` — innate tech with use tracking |
| `internal/storage/postgres/automap.go` | `AutomapRepository` — explored room bitmask persistence |
| `internal/game/xp/service.go` | `ProgressSaver`, `SkillIncreaseSaver` interfaces (defined in domain) |
| `cmd/migrate/main.go` | Migration runner binary (`make migrate`) |
| `migrations/` | 58 numbered SQL migration files (`NNN_name.up.sql` / `.down.sql`) |
| `configs/dev.yaml`, `configs/prod.yaml` | Database DSN and pool configuration |

## Core Data Structures

### Domain-Level Repo Interfaces (defined in `internal/game/`)

```go
// internal/game/xp/service.go
type ProgressSaver interface {
    SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error
}

type SkillIncreaseSaver interface {
    IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error
}
```

### Storage-Level Repository Structs (defined in `internal/storage/postgres/`)

| Struct | Key Methods |
|--------|-------------|
| `AccountRepository` | `Create`, `Authenticate`, `GetByUsername`, `SetRole` |
| `CharacterRepository` | `Create`, `GetByID`, `ListByAccount`, `SaveState`, `SaveAbilities`, `SaveProgress`, `LoadWeaponPresets`, `SaveWeaponPresets`, `LoadEquipment`, `SaveEquipment`, `LoadInventory`, `SaveInventory`, `SaveCurrency`, `LoadCurrency`, `SaveHeroPoints`, `LoadHeroPoints`, `HasReceivedStartingInventory`, `MarkStartingInventoryGranted` |
| `CharacterProgressRepository` | `SaveProgress` |
| `CharacterFeatsRepository` | `GetAll`, `Add`, `Remove` |
| `CharacterSkillsRepository` | `GetAll`, `Save` |
| `CharacterProficienciesRepository` | `GetAll`, `Save` |
| `CharacterClassFeaturesRepository` | `GetAll`, `Save` |
| `CharacterFeatureChoicesRepo` | `GetAll`, `Save` |
| `CharacterAbilityBoostsRepository` | `GetPending`, `SaveBoost`, `DecrementPending` |
| `CharacterHardwiredTechRepository` | `GetAll`, `Save` |
| `CharacterPreparedTechRepository` | `GetAll`, `Save` |
| `CharacterSpontaneousTechRepository` | `GetAll`, `Save` |
| `CharacterSpontaneousUsePoolRepository` | `GetAll`, `Save` |
| `CharacterInnateTechRepository` | `GetAll`, `Save` |
| `AutomapRepository` | `Load`, `Save` |

### Pool

```go
type Pool struct { pool *pgxpool.Pool }
// NewPool(ctx, config.DatabaseConfig) (*Pool, error)
// Pool.DB() *pgxpool.Pool  — returned to each repository constructor
```

## Primary Data Flow

1. Game logic calls a repo interface method — e.g., `ProgressSaver.SaveProgress(ctx, id, ...)`.
2. The interface is defined in a domain package under `internal/game/` (e.g., `internal/game/xp/service.go`), keeping game logic free of storage imports.
3. A concrete `CharacterProgressRepository` in `internal/storage/postgres/` satisfies that interface.
4. The implementation calls `pgxpool.Pool.Exec` or `QueryRow` with parameterized SQL (`$1`, `$2`, …) — no string interpolation.
5. Row results are scanned directly into domain structs (e.g., `character.Character`, `session.InnateSlot`).
6. Schema is managed by numbered migration files in `migrations/` — currently 58 files (029 migrations × up + down, plus any singles).
7. Migrations are applied via `cmd/migrate/main.go` (using `golang-migrate/migrate`) invoked as `make migrate`.

## Invariants & Contracts

- PERS-INV-1: Repo interfaces MUST be defined in game domain packages (`internal/game/`) — never in `internal/storage/`.
- PERS-INV-2: Raw SQL MUST appear only in `internal/storage/postgres/` implementations — never in game logic.
- PERS-INV-3: All SQL queries MUST use parameterized placeholders (`$1`, `$2`, …) — string interpolation in SQL is forbidden.
- PERS-INV-4: Migration filenames MUST be sequential integers (`NNN_description.up.sql` / `.down.sql`); a committed migration file MUST NOT be edited — new behaviour requires a new migration.
- PERS-INV-5: Current migration count is 58 files (029 numbered migrations, each with an `.up.sql` and `.down.sql`).
- PERS-INV-6: Every repository constructor MUST accept a `*pgxpool.Pool` obtained from `Pool.DB()`.
- PERS-INV-7: `ErrCharacterNotFound`, `ErrAccountNotFound`, and similar sentinel errors MUST be checked with `errors.Is`, not string comparison.

## Extension Points

To add a new table and repository:

1. **Add migration file** — create `migrations/NNN_description.up.sql` and `NNN_description.down.sql` using the next sequential integer after 029.
2. **Add interface in game domain** — define a minimal interface (e.g., `FooSaver`, `FooLoader`) in the relevant `internal/game/<domain>/` package; use only domain types in method signatures.
3. **Implement in postgres package** — create `internal/storage/postgres/foo.go` with a `FooRepository` struct holding `*pgxpool.Pool`, a `NewFooRepository` constructor, and method implementations using parameterized SQL.
4. **Wire in service constructor** — instantiate `NewFooRepository(pool.DB())` in the application wiring (typically `internal/gameserver/grpc_service.go` or equivalent service constructor) and pass it to the domain service that requires the interface.

## Common Pitfalls

- Do not place SQL in `internal/game/` — even a single query string violates PERS-INV-2.
- Do not skip the `.down.sql` companion for every `.up.sql` — the migration runner requires both for rollback.
- Do not reuse or renumber an existing migration file — always append with the next integer.
- `pgx.ErrNoRows` must be mapped to domain sentinel errors (e.g., `ErrCharacterNotFound`) at the repo boundary; callers should never see `pgx.ErrNoRows` directly.
- `isDuplicateKeyError` checks SQLSTATE `23505` — use it for unique constraint violations instead of string matching on the error message.
- The `CharacterRepository` delegates some operations to sub-repositories (e.g., `SaveProgress` delegates to `CharacterProgressRepository`) — ensure the sub-repository is also wired when constructing `CharacterRepository` independently.

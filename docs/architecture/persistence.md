# Persistence Architecture

**As of:** 2026-03-18 (commit: 525d1823035a157f2de7f516f2fa087983e9833b)

## Overview

The MUD persistence layer uses the **Repository Pattern** with a strict separation of
concerns: interfaces are defined in game domain packages (`internal/game/`),
implementations live in `internal/storage/postgres/`, and schema changes are managed
through numbered SQL migration files in `migrations/`.

The database driver is **pgx v5** via a connection pool (`pgxpool.Pool`). All SQL uses
parameterized placeholders — no string interpolation.

Cross-reference: `docs/requirements/PERSISTENCE.md`

---

## Component Diagram

```mermaid
graph TD
    subgraph "Game Domain (internal/game/)"
        XP["xp.ProgressSaver<br/>xp.SkillIncreaseSaver<br/>(interfaces)"]
    end

    subgraph "Postgres Implementations (internal/storage/postgres/)"
        POOL["Pool<br/>(pgxpool.Pool)"]
        ACCT["AccountRepository"]
        CHAR["CharacterRepository"]
        PROG["CharacterProgressRepository"]
        FEAT["CharacterFeatsRepository"]
        SKILL["CharacterSkillsRepository"]
        PROF["CharacterProficienciesRepository"]
        CF["CharacterClassFeaturesRepository"]
        FC["CharacterFeatureChoicesRepo"]
        AB["CharacterAbilityBoostsRepository"]
        HW["CharacterHardwiredTechRepository"]
        PREP["CharacterPreparedTechRepository"]
        SPON["CharacterSpontaneousTechRepository"]
        SUP["CharacterSpontaneousUsePoolRepository"]
        INN["CharacterInnateTechRepository"]
        AMAP["AutomapRepository"]
        HOTBAR["CharacterHotbarRepository"]
    end

    subgraph "Database"
        PG[(PostgreSQL)]
        MIG["migrations/<br/>058 files<br/>(029 numbered)"]
    end

    subgraph "Migration Runner"
        MIGRATE["cmd/migrate/main.go<br/>(golang-migrate)"]
    end

    XP -->|"satisfied by"| PROG

    POOL --> ACCT
    POOL --> CHAR
    POOL --> PROG
    POOL --> FEAT
    POOL --> SKILL
    POOL --> PROF
    POOL --> CF
    POOL --> FC
    POOL --> AB
    POOL --> HW
    POOL --> PREP
    POOL --> SPON
    POOL --> SUP
    POOL --> INN
    POOL --> AMAP

    ACCT --> PG
    CHAR --> PG
    PROG --> PG
    FEAT --> PG
    SKILL --> PG
    PROF --> PG
    CF --> PG
    FC --> PG
    AB --> PG
    HW --> PG
    PREP --> PG
    SPON --> PG
    SUP --> PG
    INN --> PG
    AMAP --> PG
    HOTBAR --> PG

    MIG -->|"applied by"| MIGRATE
    MIGRATE --> PG
```

---

## Request Sequence: Saving Character Progress

```mermaid
sequenceDiagram
    participant XPSvc as xp.Service
    participant Iface as xp.ProgressSaver (interface)
    participant Repo as CharacterProgressRepository
    participant Pool as pgxpool.Pool
    participant DB as PostgreSQL

    XPSvc->>Iface: SaveProgress(ctx, id, level, xp, maxHP, boosts)
    Note over Iface: Defined in internal/game/xp/service.go
    Iface->>Repo: SaveProgress(ctx, id, level, xp, maxHP, boosts)
    Note over Repo: Concrete impl in internal/storage/postgres/
    Repo->>Pool: Exec(ctx, "UPDATE characters SET ...", $1...$6)
    Pool->>DB: parameterized UPDATE
    DB-->>Pool: CommandTag (rows affected)
    Pool-->>Repo: tag, err
    Repo-->>Iface: nil | ErrCharacterNotFound
    Iface-->>XPSvc: nil | error
```

---

## Migration Strategy

- Migration files are in `migrations/` as sequential integer pairs:
  `NNN_description.up.sql` / `NNN_description.down.sql`.
- Current count: **58 files** (029 numbered migrations).
- The runner is `cmd/migrate/main.go`, invoked via `make migrate`.
- **Never edit a committed migration.** Always append a new file with the next integer.
- The runner uses `golang-migrate/migrate` with the `file://` source driver and the
  `postgres` database driver.

---

## Adding a New Table / Repo

1. **Add migration file** — create `migrations/NNN_description.up.sql` and
   `migrations/NNN_description.down.sql` where NNN is the next sequential integer after 029.
2. **Add interface in game domain** — declare a minimal interface in the relevant
   `internal/game/<domain>/` package; method signatures MUST use only domain types.
3. **Implement in postgres package** — create `internal/storage/postgres/<name>.go`
   with a struct holding `*pgxpool.Pool`, a `New<Name>Repository(db *pgxpool.Pool)`
   constructor, and methods using parameterized SQL.
4. **Wire in service constructor** — instantiate `New<Name>Repository(pool.DB())` in
   the gameserver wiring and pass it to the domain service that declares the interface.

---

## Hotbar Persistence

`CharacterHotbarRepository` (`internal/storage/postgres/character_hotbar.go`) stores and loads typed hotbar slots.

### JSON Format

Slots are stored as a `TEXT` column `hotbar` on `characters` (NULL = all empty). Current format is a JSON array of 10 objects:

```json
[{"kind":"command","ref":"look"},{"kind":"feat","ref":"power_strike"},...]
```

Empty slots are represented as `{}` (zero-value object). A `NULL` column value means all slots are empty.

### Legacy Migration

`LoadHotbar` auto-migrates the legacy format (JSON array of plain strings) on read:

1. Attempt to unmarshal as `[]struct{Kind,Ref string}`.
2. If that fails (e.g. the array contains strings not objects), attempt to unmarshal as `[]string`.
3. Each non-empty string becomes a `CommandSlot(text)`.
4. Slots beyond index 9 are silently discarded.

No explicit DB migration is required for the format change — the migration is transparent at read time.

### CharacterSaver Interface

The `gameserver` package declares:

```go
type CharacterSaver interface {
    SaveHotbar(ctx context.Context, characterID int64, slots [10]session.HotbarSlot) error
    LoadHotbar(ctx context.Context, characterID int64) ([10]session.HotbarSlot, error)
}
```

`GameServiceServer` holds a `charSaver CharacterSaver` field wired to `CharacterHotbarRepository` at startup.

---

## Invariants

| ID | Rule |
|----|------|
| PERS-INV-1 | Repo interfaces MUST be defined in `internal/game/` domain packages — never in `internal/storage/`. |
| PERS-INV-2 | Raw SQL MUST appear only in `internal/storage/postgres/` — never in game logic. |
| PERS-INV-3 | All SQL MUST use parameterized placeholders (`$1`, `$2`, …). |
| PERS-INV-4 | Committed migration files MUST NOT be edited; new behaviour requires a new file. |
| PERS-INV-5 | Current migration count is 58 files (029 numbered migrations × 2 files each). |
| PERS-INV-6 | Every repository constructor MUST accept a `*pgxpool.Pool` from `Pool.DB()`. |
| PERS-INV-7 | `pgx.ErrNoRows` MUST be mapped to a domain sentinel error at the repo boundary. |

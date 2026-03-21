# Wire Dependency Injection Refactor — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Feature:** `wire-refactor` (priority 210)
**Dependencies:** none

---

## Overview

Replace manual dependency wiring in `cmd/gameserver/main.go` (~670 lines, 45-parameter `NewGameServiceServer`), `cmd/devserver/main.go`, and `cmd/frontend/main.go` with Google Wire code-generated injectors. Each binary's `main.go` is reduced to flag parsing, a call to the wire-generated `Initialize()` function, lifecycle management, and any post-construction setter injection that cannot be expressed as constructor dependencies (see Section 4). No behavior changes — this is a pure structural refactor.

---

## 1. Provider Organization

Wire works by defining *providers* (functions that construct a value) and *injectors* (wire-generated functions that call providers in dependency order). Providers are grouped into named sets and live in the packages that own the types.

### 1.1 Provider Sets

| Provider Set | Package | Contents |
|---|---|---|
| `StorageProviders` | `internal/storage/postgres/providers.go` | `postgres.Pool`, all repository constructors, `wire.Bind` mappings for interface/concrete pairs (see Section 1.3) |
| `ContentProviders` | per-domain `providers.go` files | World loader + manager, NPC template loader + manager + respawn manager, condition registry, inventory registry (weapons/armor/items/explosives) + floor manager + room equipment manager, ruleset loaders (jobs/skills/feats/archetypes/regions/class features + registry), tech registry, AI registry, scripting manager, dice roller, mental state manager, combat engine |
| `HandlerProviders` | `internal/gameserver/providers.go` | `CombatHandler`, `WorldHandler`, `ChatHandler`, `NPCHandler`, `ActionHandler`, `RegenManager`, `GameClock`, `GameCalendar`, `ZoneTickManager` |
| `ServerProviders` | `internal/gameserver/providers.go` | `GameServiceServer`, `command.Registry`, `session.Manager`, `NewAccountRepoAdapter` |
| `FrontendProviders` | `internal/frontend/providers.go` | `AuthHandler`, `GameBridge`, `Acceptor`, `TextRenderer` |

### 1.2 Dependency Struct Refactor

`NewGameServiceServer` is refactored from 45 individual parameters to grouped dependency structs. Types match exactly what the current constructor uses — interfaces where interfaces are used, concrete pointers where concrete pointers are used.

```go
type StorageDeps struct {
    CharRepo                   CharacterSaver                             // *postgres.CharacterRepository
    AccountRepo                AccountAdmin                               // via NewAccountRepoAdapter
    SkillsRepo                 CharacterSkillsRepository                  // *postgres.CharacterSkillsRepository
    ProficienciesRepo          CharacterProficienciesRepository           // *postgres.CharacterProficienciesRepository
    FeatsRepo                  CharacterFeatsGetter                       // *postgres.CharacterFeatsRepository
    ClassFeaturesRepo          CharacterClassFeaturesGetter               // *postgres.CharacterClassFeaturesRepository
    FeatureChoicesRepo         CharacterFeatureChoicesRepository          // *postgres.CharacterFeatureChoicesRepo
    AbilityBoostsRepo          postgres.CharacterAbilityBoostsRepository  // *postgres.PostgresCharacterAbilityBoostsRepository
    HardwiredTechRepo          HardwiredTechRepo                          // *postgres.CharacterHardwiredTechRepository
    PreparedTechRepo           PreparedTechRepo                           // *postgres.CharacterPreparedTechRepository
    SpontaneousTechRepo        SpontaneousTechRepo                        // *postgres.CharacterSpontaneousTechRepository
    InnateTechRepo             InnateTechRepo                             // *postgres.CharacterInnateTechRepository
    SpontaneousUsePoolRepo     SpontaneousUsePoolRepo                     // *postgres.CharacterSpontaneousUsePoolRepository
    WantedRepo                 *postgres.WantedRepository                 // concrete; no interface exists
    AutomapRepo                *postgres.AutomapRepository                // concrete; no interface exists
}

type ContentDeps struct {
    WorldMgr              *world.Manager
    NpcMgr                *npc.Manager
    RespawnMgr            *npc.RespawnManager
    InvRegistry           *inventory.Registry
    FloorMgr              *inventory.FloorManager
    RoomEquipMgr          *inventory.RoomEquipmentManager
    TechRegistry          *technology.Registry
    CondRegistry          *condition.Registry
    AIRegistry            *ai.Registry
    NpcTemplates          []*npc.Template
    AllSkills             []*ruleset.Skill
    AllFeats              []*ruleset.Feat
    ClassFeatures         []*ruleset.ClassFeature
    FeatRegistry          *ruleset.FeatRegistry
    ClassFeatureRegistry  *ruleset.ClassFeatureRegistry
    JobRegistry           *ruleset.JobRegistry
    ArchetypeMap          map[string]*ruleset.Archetype
    RegionMap             map[string]*ruleset.Region
    ScriptMgr             *scripting.Manager
    DiceRoller            *dice.Roller                  // struct is Roller; constructor is NewLoggedRoller
    CombatEngine          *combat.Engine
    MentalStateMgr        *mentalstate.Manager
    LoadoutsDir           string
}

type HandlerDeps struct {
    WorldHandler   *WorldHandler
    ChatHandler    *ChatHandler
    NPCHandler     *NPCHandler
    CombatHandler  *CombatHandler
    ActionHandler  *ActionHandler
}
```

`NewGameServiceServer` signature becomes:

```go
func NewGameServiceServer(
    storage StorageDeps,
    content ContentDeps,
    handlers HandlerDeps,
    sessMgr *session.Manager,
    cmdRegistry *command.Registry,
    gameCalendar *GameCalendar,
    logger *zap.Logger,
) *GameServiceServer
```

Note: `trapMgr *trap.TrapManager` and `trapTemplates map[string]*trap.TrapTemplate` are currently passed as `nil` in `main.go` (traps not yet loaded). They are removed from the `NewGameServiceServer` constructor signature and their corresponding struct fields are initialized to `nil` directly in the constructor body (not via wire). This is consistent with the fields being stubs awaiting a future trap-loading feature.

### 1.3 Interface Bindings (wire.Bind)

`StorageProviders` must include `wire.Bind` calls for every interface/concrete pair. Wire resolves by exact type; without these bindings the generated injector will not compile. Concrete types with no corresponding interface (`*postgres.WantedRepository`, `*postgres.AutomapRepository`) do not need `wire.Bind`.

`NewPool` requires `context.Context` and `config.DatabaseConfig`. Both enter the wire graph as injector inputs (see Section 2.1). All repository constructors take `*pgxpool.Pool` (not `*postgres.Pool`); the `StorageProviders` set must include a `PoolDB` unwrapper provider:

```go
// PoolDB extracts the raw pgx pool from the wrapped Pool for use by repository constructors.
func PoolDB(p *postgres.Pool) *pgxpool.Pool { return p.DB() }
```

`GameClock` takes two primitive constructor parameters (`startHour int32`, `tickInterval time.Duration`) sourced from `AppConfig`. To avoid primitive type collision in the wire graph, `GameClock` is constructed in `main.go` before `Initialize()` and passed as an injector input value. It is NOT constructed by a wire provider.

```go
var StorageProviders = wire.NewSet(
    postgres.NewPool,
    postgres.PoolDB,              // *postgres.Pool → *pgxpool.Pool for repo constructors
    postgres.NewCharacterRepository,
    wire.Bind(new(gameserver.CharacterSaver), new(*postgres.CharacterRepository)),
    postgres.NewAccountRepository,
    gameserver.NewAccountRepoAdapter,
    wire.Bind(new(gameserver.AccountAdmin), new(*gameserver.AccountRepoAdapter)),
    postgres.NewCharacterSkillsRepository,
    wire.Bind(new(gameserver.CharacterSkillsRepository), new(*postgres.CharacterSkillsRepository)),
    postgres.NewCharacterProficienciesRepository,
    wire.Bind(new(gameserver.CharacterProficienciesRepository), new(*postgres.CharacterProficienciesRepository)),
    postgres.NewCharacterFeatsRepository,
    wire.Bind(new(gameserver.CharacterFeatsGetter), new(*postgres.CharacterFeatsRepository)),
    postgres.NewCharacterClassFeaturesRepository,
    wire.Bind(new(gameserver.CharacterClassFeaturesGetter), new(*postgres.CharacterClassFeaturesRepository)),
    postgres.NewCharacterFeatureChoicesRepo,
    wire.Bind(new(gameserver.CharacterFeatureChoicesRepository), new(*postgres.CharacterFeatureChoicesRepo)),
    postgres.NewCharacterAbilityBoostsRepository,
    wire.Bind(new(postgres.CharacterAbilityBoostsRepository), new(*postgres.PostgresCharacterAbilityBoostsRepository)),
    postgres.NewCharacterHardwiredTechRepository,
    wire.Bind(new(gameserver.HardwiredTechRepo), new(*postgres.CharacterHardwiredTechRepository)),
    postgres.NewCharacterPreparedTechRepository,
    wire.Bind(new(gameserver.PreparedTechRepo), new(*postgres.CharacterPreparedTechRepository)),
    postgres.NewCharacterSpontaneousTechRepository,
    wire.Bind(new(gameserver.SpontaneousTechRepo), new(*postgres.CharacterSpontaneousTechRepository)),
    postgres.NewCharacterInnateTechRepository,
    wire.Bind(new(gameserver.InnateTechRepo), new(*postgres.CharacterInnateTechRepository)),
    postgres.NewCharacterSpontaneousUsePoolRepository,
    wire.Bind(new(gameserver.SpontaneousUsePoolRepo), new(*postgres.CharacterSpontaneousUsePoolRepository)),
    postgres.NewWantedRepository,     // concrete *postgres.WantedRepository; no wire.Bind needed
    postgres.NewAutomapRepository,    // concrete *postgres.AutomapRepository; no wire.Bind needed
    postgres.NewCalendarRepo,
    postgres.NewCharacterProgressRepository,  // required for App.ProgressRepo post-init setter
)
```

---

## 2. File Layout

```
cmd/gameserver/
  main.go          — flag parsing + AppConfig + wire-generated Initialize() + lifecycle + post-init setters
  wire.go          — injector declaration (build tag: wireinject)
  wire_gen.go      — generated by `wire` (committed to repo)

cmd/devserver/
  main.go
  wire.go
  wire_gen.go

cmd/frontend/
  main.go
  wire.go
  wire_gen.go

internal/storage/postgres/
  providers.go     — StorageProviders wire.ProviderSet (constructors + wire.Bind mappings)

internal/game/world/
  providers.go     — world loader + manager provider

internal/game/npc/
  providers.go     — NPC template loader, manager, respawn manager providers

internal/game/condition/
  providers.go     — condition registry provider

internal/game/inventory/
  providers.go     — inventory registry, floor manager, room equipment manager providers

internal/game/ruleset/
  providers.go     — jobs, skills, feats, archetypes, regions, class features + registry providers

internal/game/technology/
  providers.go     — tech registry provider

internal/game/ai/
  providers.go     — AI registry provider

internal/game/dice/
  providers.go     — dice roller provider (wraps NewLoggedRoller)

internal/game/combat/
  providers.go     — combat engine provider

internal/game/mentalstate/
  providers.go     — mental state manager provider

internal/scripting/
  providers.go     — scripting manager provider

internal/gameserver/
  providers.go     — HandlerProviders + ServerProviders wire.ProviderSets

internal/frontend/
  providers.go     — FrontendProviders wire.ProviderSet

tools/
  tools.go         — wire tool pin (build tag: tools)
```

### 2.1 wire.go Pattern

Each binary's `wire.go` declares the injector function with the `wireinject` build tag. `AppConfig` aggregates all CLI flag values and is passed as a wire input value (not constructed by wire). Each binary defines its own `AppConfig` type matching its flags.

`context.Context` and `config.DatabaseConfig` enter the wire graph as injector inputs. A `AppConfigToDatabase` provider extracts `config.DatabaseConfig` from `*AppConfig`. `GameClock` is constructed before `Initialize()` in `main.go` (see Section 2.1 notes) and passed as an injector input.

```go
//go:build wireinject

package main

import "github.com/google/wire"

func Initialize(ctx context.Context, cfg *AppConfig, clock *gameserver.GameClock, logger *zap.Logger) (*App, error) {
    wire.Build(
        AppConfigToDatabase,      // *AppConfig → config.DatabaseConfig
        postgres.StorageProviders,
        gameserver.HandlerProviders,
        gameserver.ServerProviders,
        // ... other provider sets
    )
    return nil, nil
}

// AppConfigToDatabase extracts the database config from AppConfig for wire.
func AppConfigToDatabase(cfg *AppConfig) config.DatabaseConfig {
    return cfg.Database
}
```

`main.go` constructs `GameClock` before calling `Initialize()`:

```go
gameClock := gameserver.NewGameClock(cfg.GameServer.GameClockStart, cfg.GameServer.GameTickDuration)
app, err := Initialize(ctx, &cfg, gameClock, logger)
```

### 2.2 App Struct and Lifecycle

The `App` struct returned by `Initialize()` holds all top-level components that require lifecycle management or post-construction access. Fields required only for post-`Initialize()` setter injection are included explicitly:

```go
type App struct {
    // Lifecycle-managed components
    GRPCService    *gameserver.GameServiceServer
    CombatHandler  *gameserver.CombatHandler
    RegenMgr       *gameserver.RegenManager
    ZoneTickMgr    *gameserver.ZoneTickManager
    AIRegistry     *ai.Registry
    GameClock      *gameserver.GameClock
    GameCalendar   *gameserver.GameCalendar
    Pool           *postgres.Pool
    GRPCServer     *grpc.Server

    // Concrete repo references required for post-Initialize setter injection
    CharRepo       *postgres.CharacterRepository
    ProgressRepo   *postgres.CharacterProgressRepository
}
```

`main.go` after `Initialize()`:
1. Calls post-construction setters (XP service, progress repo — see Section 4)
2. Calls `app.RegenMgr.Start(ctx)`
3. Calls `app.GRPCService.StartZoneTicks(ctx, app.ZoneTickMgr, app.AIRegistry)`
4. Runs `lifecycle.Run(ctx)`

### 2.3 GameCalendar Provider

`GameCalendar` construction requires a fallible DB load (`calendarRepo.Load()` returns `calDay, calMonth int, err error`). Wire supports providers that return `error`. The provider for `GameCalendar` in `HandlerProviders` is:

```go
func NewGameCalendarFromRepo(repo *postgres.CalendarRepo, clock *GameClock) (*GameCalendar, error) {
    calDay, calMonth, err := repo.Load()
    if err != nil {
        return nil, fmt.Errorf("loading calendar: %w", err)
    }
    return NewGameCalendar(clock, calDay, calMonth, repo), nil
}
```

This replaces the manual `calendarRepo.Load()` block in `main.go`.

### 2.4 wire_gen.go

`wire_gen.go` carries the `//go:build !wireinject` build tag and is committed to the repository so the binary builds without requiring `wire` installed. This mirrors the pattern used for protobuf-generated files.

---

## 3. Build Integration

### 3.1 Makefile Target

```makefile
.PHONY: wire
wire:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...
```

### 3.2 Tool Pin

`wire` is pinned via `tools/tools.go`:

```go
//go:build tools

package tools

import _ "github.com/google/wire/cmd/wire"
```

`wire` is installed via `go install` from the pinned module version in `go.mod`. The `.mise.toml` file manages the Go toolchain only; `wire` is not a mise plugin. The canonical installation command (without `@latest`, so it uses the version pinned in `go.mod`) is:

```bash
go install github.com/google/wire/cmd/wire
```

This is added to the `make deps` target so `wire` is available in the `mise`-managed Go environment before `make wire` can run. The version is pinned by the `tools/tools.go` blank import, which causes `go mod tidy` to record the version in `go.mod`/`go.sum`.

### 3.3 Staleness Check

A `make wire-check` target diffs regenerated output against committed `wire_gen.go` files to catch stale codegen in CI:

```makefile
.PHONY: wire-check
wire-check:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...
	git diff --exit-code cmd/gameserver/wire_gen.go cmd/devserver/wire_gen.go cmd/frontend/wire_gen.go
```

---

## 4. Post-Construction Setter Injection

Wire models constructor injection only. The XP service and progress repo are wired via setter injection after construction because their initialization is conditional (the XP config file load can fail non-fatally):

```go
// In main.go, after Initialize():
if xpCfg, err := xp.LoadXPConfig(*xpConfigFile); err != nil {
    logger.Warn("loading xp config; XP awards disabled", zap.Error(err))
} else {
    xpSvc := xp.NewService(xpCfg, app.ProgressRepo)
    xpSvc.SetSkillIncreaseSaver(app.ProgressRepo)
    app.GRPCService.SetProgressRepo(app.ProgressRepo)
    app.GRPCService.SetXPService(xpSvc)
    app.CombatHandler.SetXPService(xpSvc)
    app.CombatHandler.SetCurrencySaver(app.CharRepo)
}
```

`main.go` is therefore not reduced to a single `Initialize()` call — it retains this post-construction block. `App.CombatHandler`, `App.GRPCService`, `App.CharRepo`, and `App.ProgressRepo` are all exposed as fields for this purpose (see Section 2.2). This is a documented exception to the wire pattern, not an oversight.

---

## 5. Devserver Scope

`cmd/devserver` is a frontend + storage binary — it requires `StorageProviders` (the auth handler needs DB access) and a subset of `ContentProviders` (ruleset loaders: skills, feats, jobs, archetypes, regions, class features, **teams** — needed by `AuthHandler` for character creation). It does NOT use `HandlerProviders` or `ServerProviders` (no gRPC game handlers). It also does NOT need the game engine content providers (combat engine, AI registry, scripting manager, mental state manager). A `RulesetContentProviders` sub-set is extracted from `ContentProviders` containing exactly these loaders:

- `ruleset.LoadSkills`
- `ruleset.LoadFeats`
- `ruleset.LoadJobs`
- `ruleset.LoadArchetypes`
- `ruleset.LoadRegions`
- `ruleset.LoadClassFeatures`
- `ruleset.LoadTeams`
- `ruleset.NewFeatRegistry`
- `ruleset.NewClassFeatureRegistry`

The devserver injector uses `StorageProviders` and `RulesetContentProviders` only, avoiding the full game engine dependency graph.

---

## 6. Requirements

- REQ-WIRE-1: All tests passing before the refactor MUST pass after. No new skips permitted.
- REQ-WIRE-2: `wire_gen.go` MUST be committed to the repository in each binary's directory with the `!wireinject` build tag.
- REQ-WIRE-3: `make wire` MUST regenerate all three `wire_gen.go` files cleanly with no errors.
- REQ-WIRE-4: `NewGameServiceServer` MUST be refactored to accept `StorageDeps`, `ContentDeps`, and `HandlerDeps` structs in place of individual parameters. `trapMgr` and `trapTemplates` MUST be removed from the constructor signature and set to nil via `wire.Value` in the injector.
- REQ-WIRE-5: Provider functions MUST live in `providers.go` files within the packages that own the types, not in `cmd/` directories.
- REQ-WIRE-6: Flag parsing MUST remain in each binary's `main.go`; wire MUST NOT be responsible for CLI flag binding.
- REQ-WIRE-7: `wire` MUST be pinned via `tools/tools.go` and installed via `go install github.com/google/wire/cmd/wire` (no `@latest`; version resolved from `go.mod`) as part of `make deps`. The `.mise.toml` file MUST NOT be modified.
- REQ-WIRE-8: This refactor MUST introduce no behavior changes. No new features, no new flags, no changes to game logic.
- REQ-WIRE-9: The XP service setter injection block MUST remain in `main.go` after the `Initialize()` call. It MUST NOT be forced into a wire provider.
- REQ-WIRE-10: `StorageProviders` MUST include `wire.Bind` calls for every interface/concrete pair consumed by `NewGameServiceServer`. Concrete types with no interface (`*postgres.WantedRepository`, `*postgres.AutomapRepository`) MUST NOT have spurious `wire.Bind` calls.
- REQ-WIRE-11: `make wire-check` MUST diff regenerated `wire_gen.go` files against committed versions and fail if they differ, for use in CI.
- REQ-WIRE-12: `StorageDeps.AbilityBoostsRepo` MUST be typed `postgres.CharacterAbilityBoostsRepository` (the interface defined in the postgres package). The `wire.Bind` for this field MUST map `postgres.CharacterAbilityBoostsRepository` to `*postgres.PostgresCharacterAbilityBoostsRepository`.
- REQ-WIRE-13: `GameCalendar` MUST be constructed via a `NewGameCalendarFromRepo(*postgres.CalendarRepo, *GameClock) (*GameCalendar, error)` provider that calls `repo.Load()` internally. The manual calendar load in `main.go` MUST be replaced by this provider.
- REQ-WIRE-14: `postgres.NewCharacterProgressRepository` MUST be included in `StorageProviders` so that `App.ProgressRepo` is available for post-construction setter injection.
- REQ-WIRE-15: A `RulesetContentProviders` wire set MUST be extracted from `ContentProviders` containing only ruleset loaders (skills, feats, jobs, archetypes, regions, class features, teams). The devserver injector MUST use `RulesetContentProviders` instead of the full `ContentProviders` to avoid pulling the game engine into the devserver binary.
- REQ-WIRE-16: `StorageProviders` MUST include a `postgres.PoolDB(p *postgres.Pool) *pgxpool.Pool` provider that calls `p.DB()`. This provider satisfies all repository constructor dependencies on `*pgxpool.Pool`.
- REQ-WIRE-17: The `Initialize()` injector MUST accept `context.Context`, `*AppConfig`, `*gameserver.GameClock`, and `*zap.Logger` as inputs. `GameClock` MUST be constructed in `main.go` before `Initialize()` is called and passed as an input value, not as a wire provider. An `AppConfigToDatabase(*AppConfig) config.DatabaseConfig` function in `main.go` MUST be included in `wire.Build` to satisfy the `config.DatabaseConfig` dependency of `postgres.NewPool`.
- REQ-WIRE-18: `trapMgr` and `trapTemplates` MUST be removed from the `NewGameServiceServer` constructor signature. Their corresponding struct fields MUST be set to `nil` directly in the constructor body. No wire involvement.
- REQ-WIRE-19: `NewGameCalendarFromRepo` MUST be placed in `internal/gameserver/providers.go` and MUST have signature `func NewGameCalendarFromRepo(repo *postgres.CalendarRepo, clock *GameClock) (*GameCalendar, error)`. It MUST call `repo.Load()` and return the error if `Load()` fails.

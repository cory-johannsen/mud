## Trigger
When to load: working on YAML content files, importer, registry, content formats, adding new content types, modifying startup loading logic, debugging missing content errors.

## Responsibility Boundary
This system owns: parsing and validating YAML content files, populating in-memory registries, and making content available to game handlers at runtime. It does not persist content to the database (DB is used only for character/session state). Named packages: `internal/game/world/`, `internal/game/inventory/`, `internal/game/ruleset/`, `internal/game/condition/`, `internal/game/technology/`, `internal/game/npc/`, `internal/game/ai/`.

## Key Files
- `/home/cjohannsen/src/mud/cmd/gameserver/main.go` — orchestrates all content loading at startup; each content type is loaded in sequence before gRPC server starts.
- `/home/cjohannsen/src/mud/internal/game/world/loader.go` — `LoadZonesFromDir`, `LoadZoneFromFile`, `LoadZoneFromBytes`; validates cross-zone exits via `worldMgr.ValidateExits()`.
- `/home/cjohannsen/src/mud/internal/game/world/manager.go` — `Manager` holds all zones and rooms; provides `GetRoom`, `Navigate`, `AllZones`.
- `/home/cjohannsen/src/mud/internal/game/inventory/registry.go` — unified `Registry` for weapons, explosives, items, and armor; `RegisterWeapon`, `RegisterArmor`, etc.
- `/home/cjohannsen/src/mud/internal/game/ruleset/feat.go` — `LoadFeats`, `FeatRegistry`; also hosts `LoadSkills`, `LoadJobs`, `LoadArchetypes`, `LoadRegions`, `LoadClassFeatures` in sibling files.
- `/home/cjohannsen/src/mud/internal/game/condition/definition.go` — `LoadDirectory` returns `*Registry` of `ConditionDef` keyed by ID.
- `/home/cjohannsen/src/mud/internal/game/technology/registry.go` — `Load(dir)` returns `*Registry`; `Get`, `ByTradition`, `ByUsageType` lookups.
- `/home/cjohannsen/src/mud/internal/game/npc/template.go` — `LoadTemplates(dir)` returns `[]*Template`; NPC instances spawned into `npc.Manager` at startup.
- `/home/cjohannsen/src/mud/content/` — root of all YAML content files; sub-directories: zones, npcs, conditions, weapons, items, explosives, armor, jobs, archetypes, regions, technologies, ai, loadouts, scripts, teams (teams: exists in content/ and ruleset package but not wired into startup sequence).

## Core Data Structures

`world.Zone` (the most structurally rich content type):
```go
type Zone struct {
    ID          string
    Name        string
    Description string
    StartRoom   string
    Rooms       map[string]*Room   // keyed by room ID
    ScriptDir   string             // path to Lua scripts; empty = no scripts
    ScriptInstructionLimit int
}

type Room struct {
    ID          string
    ZoneID      string
    Title       string
    Description string
    Exits       []Exit
    Properties  map[string]string
    Spawns      []RoomSpawnConfig
    Equipment   []RoomEquipmentConfig
    MapX, MapY  int
    SkillChecks []skillcheck.TriggerDef
    Effects     []RoomEffect
    Terrain     string
}
```

`inventory.WeaponDef` (flat definition with validation):
```go
type WeaponDef struct {
    ID                  string
    Name                string
    DamageDice          string
    DamageType          string
    Kind                WeaponKind
    Group               string
    FiringModes         []FiringMode
    Traits              []string
    MagazineCapacity    int
    ReloadActions       int
    RangeIncrement      int
    ProficiencyCategory string
    TeamAffinity        string
    CrossTeamEffect     *CrossTeamEffect // nil = no side effect
}
```

## Primary Data Flow

1. `cmd/gameserver/main.go` is invoked with CLI flags pointing to content directories (e.g., `--zones content/zones`, `--weapons-dir content/weapons`).
2. Each loader function (e.g., `world.LoadZonesFromDir`, `inventory.LoadWeapons`, `ruleset.LoadJobs`) reads all `.yaml` files from the specified path, unmarshals them with `gopkg.in/yaml.v3`, and validates required fields.
3. Loaded definitions are inserted into their respective in-memory registry (`inventory.Registry.RegisterWeapon`, `condition.Registry.Register`, `technology.Registry.Register`, etc.) or stored as a `map[string]*T` (archetypes, regions) or plain slice (skills, class features).
4. Cross-reference validation runs after all types are loaded (e.g., `worldMgr.ValidateExits()` checks that exit targets exist; NPC spawn configs reference templates that must exist).
5. Registries and maps are injected into `gameserver.GameServiceServer` and handler structs via constructor arguments — no globals.
6. The gRPC server begins accepting connections only after all content is loaded. Any load failure calls `logger.Fatal`, terminating the process. DB is not used for content (content is read-only at startup).

## Invariants & Contracts

- PIPE-INV-1: Content MUST be immutable at runtime; registries expose only read methods after startup; no content is written back to disk or DB.
- PIPE-INV-2: All content MUST be fully loaded before the gRPC listener starts; any load error calls `logger.Fatal` and prevents startup.
- PIPE-INV-3: Every YAML file that is referenced (e.g., NPC template ID in a zone spawn) MUST exist; a missing reference is a startup failure.
- PIPE-INV-4: Cross-zone exit targets MUST resolve to known room IDs; `worldMgr.ValidateExits()` enforces this after all zones are loaded.
- PIPE-INV-5: Content IDs MUST be unique within their type; duplicate registration returns an error that causes `logger.Fatal`.
- PIPE-INV-6: The DB is used exclusively for mutable character/session state; content registries are never queried from or written to Postgres.

## Extension Points

To add a new content type:
1. Create a Go model file in the appropriate `internal/game/<domain>/` package with the definition struct and a `Validate()` method.
2. Add a `Load<Type>(path string) ([]*<Type>, error)` or `Load<Type>FromDir(dir string) (*Registry, error)` function in the same package.
3. Add a `Registry` type (or reuse `inventory.Registry`) with `Register` and `Get` methods.
4. Add a CLI flag in `cmd/gameserver/main.go` pointing to the YAML directory.
5. Call the loader in `main.go` and wire `logger.Fatal` on error; inject the registry into `GameServiceServer` and relevant handlers.
6. Create YAML files under `content/<type>/`.
7. Write property-based tests covering load, validate, and lookup (SWENG-5, SWENG-5a).

## Common Pitfalls

- Forgetting to call `worldMgr.ValidateExits()` after adding cross-zone exit references — silently broken navigation at runtime.
- Adding a new content type to a handler but forgetting to inject the registry through `GameServiceServer`'s constructor — results in nil pointer panics at runtime, not compile time.
- Using a bare `map[string]*T` instead of a dedicated `Registry` type — skips duplicate-ID detection and makes content inaccessible to handlers that only accept the typed registry.
- Mutating a content definition struct at runtime (e.g., setting a field on a `*WeaponDef`) — violates PIPE-INV-1 and causes data races under concurrent access.
- Placing content YAML under a sub-directory that does not match the CLI flag default — the file will not be found in development but will silently succeed if the directory is empty (the loader returns zero items, not an error, for an empty directory in some types).
- Skipping `Validate()` in the loader — invalid YAML (e.g., unknown firing mode) reaches the registry and fails only at the first runtime use.
- `content/teams/` is not wired into the startup sequence — `LoadTeams` exists in `internal/game/ruleset/team.go` but is not called from `main.go`; team definitions (`gun.yaml`, `machete.yaml`) are present on disk but are not loaded at runtime.

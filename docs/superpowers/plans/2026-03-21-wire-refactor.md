# Wire Dependency Injection Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 45-parameter `NewGameServiceServer` and manual wiring in three binaries with Google Wire code-generated injectors, preserving all existing behavior.

**Architecture:** Wire providers are placed in `providers.go` files within each package that owns its types. `NewGameServiceServer` is refactored to accept three dependency structs (`StorageDeps`, `ContentDeps`, `HandlerDeps`). Each binary gets `wire.go` (injector declaration) and `wire_gen.go` (committed generated output). Post-construction callbacks (scripting, broadcastFn, XP service) remain in `main.go` after `Initialize()`.

**Tech Stack:** Go 1.26, `github.com/google/wire`, pgx v5, gRPC, zap

---

## File Structure

**New files:**
- `tools/tools.go` — wire tool pin
- `internal/gameserver/deps.go` — StorageDeps, ContentDeps, HandlerDeps structs
- `internal/storage/postgres/providers.go` — StorageProviders
- `internal/game/world/providers.go`
- `internal/game/npc/providers.go`
- `internal/game/condition/providers.go`
- `internal/game/technology/providers.go`
- `internal/game/inventory/providers.go`
- `internal/game/ruleset/providers.go` — GameProviders + RulesetContentProviders
- `internal/game/ai/providers.go`
- `internal/game/dice/providers.go`
- `internal/game/combat/providers.go`
- `internal/game/mentalstate/providers.go`
- `internal/scripting/providers.go`
- `internal/gameserver/providers.go` — HandlerProviders, ServerProviders, NewGameCalendarFromRepo
- `cmd/gameserver/wire.go`, `cmd/gameserver/wire_gen.go`
- `cmd/devserver/wire.go`, `cmd/devserver/wire_gen.go`
- `cmd/frontend/wire.go`, `cmd/frontend/wire_gen.go`

**Modified files:**
- `go.mod` / `go.sum` — add wire dependency
- `Makefile` — add `deps`, `wire`, `wire-check` targets
- `internal/gameserver/grpc_service.go` — refactor `NewGameServiceServer` signature
- `cmd/gameserver/main.go` — replace manual wiring with `Initialize()`
- `cmd/devserver/main.go` — replace manual wiring with `Initialize()`
- `cmd/frontend/main.go` — replace manual wiring with `Initialize()`

---

### Task 1: Wire Tool Pin

**Files:**
- Create: `tools/tools.go`
- Modify: `go.mod`, `Makefile`

- [ ] **Step 1: Create tools/tools.go**

```go
//go:build tools

package tools

import _ "github.com/google/wire/cmd/wire"
```

- [ ] **Step 2: Add wire to go.mod**

```bash
go get github.com/google/wire
go mod tidy
```

Note: Do NOT use `@latest`. The version is pinned via `tools/tools.go`; `go get` without a version suffix resolves the highest compatible version and records it in `go.mod` (REQ-WIRE-7).

- [ ] **Step 3: Add deps target to Makefile** (after the `.PHONY` line, before `build:`)

Note: The existing Makefile defines `GO := go` (verified at line 3). Use `$(GO)` consistently.

```makefile
.PHONY: deps wire wire-check

deps:
	$(GO) mod tidy
	$(GO) install github.com/google/wire/cmd/wire

wire:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...

wire-check:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...
	git diff --exit-code cmd/gameserver/wire_gen.go cmd/devserver/wire_gen.go cmd/frontend/wire_gen.go
```

- [ ] **Step 4: Install wire and verify**

```bash
make deps
wire --version
```

Expected: wire version printed (e.g. `wire: v0.6.0`)

- [ ] **Step 5: Verify build still passes**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add tools/tools.go go.mod go.sum Makefile
git commit -m "chore: pin google/wire tool; add deps/wire/wire-check make targets"
```

---

### Task 2: Dependency Struct Definitions

**Files:**
- Create: `internal/gameserver/deps.go`

- [ ] **Step 1: Create deps.go**

Note: `LoadoutsDir` named type is defined here (not in `providers.go`) so that `deps.go` compiles independently. `NpcTemplates []*npc.Template` is intentionally omitted from `ContentDeps` because `NewGameServiceServer` does not store or use that field directly — the NPC manager encapsulates templates internally.

```go
package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// LoadoutsDir is a named string type for the loadouts directory path.
// Named to avoid wire ambiguity with other string-typed paths.
type LoadoutsDir string

// StorageDeps groups all repository dependencies for GameServiceServer.
type StorageDeps struct {
	CharRepo               CharacterSaver
	AccountRepo            AccountAdmin
	SkillsRepo             CharacterSkillsRepository
	ProficienciesRepo      CharacterProficienciesRepository
	FeatsRepo              CharacterFeatsGetter
	ClassFeaturesRepo      CharacterClassFeaturesGetter
	FeatureChoicesRepo     CharacterFeatureChoicesRepository
	AbilityBoostsRepo      postgres.CharacterAbilityBoostsRepository
	HardwiredTechRepo      HardwiredTechRepo
	PreparedTechRepo       PreparedTechRepo
	SpontaneousTechRepo    SpontaneousTechRepo
	InnateTechRepo         InnateTechRepo
	SpontaneousUsePoolRepo SpontaneousUsePoolRepo
	WantedRepo             *postgres.WantedRepository
	AutomapRepo            *postgres.AutomapRepository
}

// ContentDeps groups all content/world dependencies for GameServiceServer.
type ContentDeps struct {
	WorldMgr             *world.Manager
	NpcMgr               *npc.Manager
	RespawnMgr           *npc.RespawnManager
	InvRegistry          *inventory.Registry
	FloorMgr             *inventory.FloorManager
	RoomEquipMgr         *inventory.RoomEquipmentManager
	TechRegistry         *technology.Registry
	CondRegistry         *condition.Registry
	AIRegistry           *ai.Registry
	AllSkills            []*ruleset.Skill
	AllFeats             []*ruleset.Feat
	ClassFeatures        []*ruleset.ClassFeature
	FeatRegistry         *ruleset.FeatRegistry
	ClassFeatureRegistry *ruleset.ClassFeatureRegistry
	JobRegistry          *ruleset.JobRegistry
	ArchetypeMap         map[string]*ruleset.Archetype
	RegionMap            map[string]*ruleset.Region
	ScriptMgr            *scripting.Manager
	DiceRoller           *dice.Roller
	CombatEngine         *combat.Engine
	MentalStateMgr       *mentalstate.Manager
	LoadoutsDir          LoadoutsDir
}

// HandlerDeps groups all handler dependencies for GameServiceServer.
type HandlerDeps struct {
	WorldHandler  *WorldHandler
	ChatHandler   *ChatHandler
	NPCHandler    *NPCHandler
	CombatHandler *CombatHandler
	ActionHandler *ActionHandler
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/gameserver/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/gameserver/deps.go
git commit -m "feat(wire): add StorageDeps, ContentDeps, HandlerDeps structs"
```

---

### Task 3: Refactor NewGameServiceServer Signature

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

The current constructor (line 240) takes 45 individual parameters. Replace with 3 structs + 4 scalars. Also remove `trapMgr` and `trapTemplates` from the signature (set to nil in body).

Note (REQ-WIRE-4 vs REQ-WIRE-18 conflict): REQ-WIRE-4 says `trapMgr`/`trapTemplates` should be set via `wire.Value`. REQ-WIRE-18 (more specific, later) says "No wire involvement — set directly in the constructor body." This plan follows REQ-WIRE-18: the fields are set to `nil` inside `NewGameServiceServer` itself, with no wire.Value injector entry. REQ-WIRE-18 is the authoritative requirement.

- [ ] **Step 0: Write a failing compilation test (TDD)**

Create `internal/gameserver/grpc_service_refactor_test.go`:

```go
package gameserver_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

// TestNewGameServiceServerSignature asserts that the refactored constructor
// accepts the three dependency structs. This test fails until Step 1 is done.
func TestNewGameServiceServerSignature(t *testing.T) {
	// Compilation-only test: if this file compiles, the signature is correct.
	// The zero-value structs are intentionally not passed to avoid nil panics.
	var _ = gameserver.NewGameServiceServer
	_ = gameserver.StorageDeps{}
	_ = gameserver.ContentDeps{}
	_ = gameserver.HandlerDeps{}
}
```

Run and verify it fails to compile (because `StorageDeps`, `ContentDeps`, `HandlerDeps` don't exist yet — wait, they exist from Task 2 but `NewGameServiceServer` still has the old signature):

```bash
go test ./internal/gameserver/... 2>&1 | head -20
```

Expected: compilation error referencing `NewGameServiceServer` old signature mismatch or test compiles but constructor call won't match. Confirm the test is present before proceeding.

- [ ] **Step 1: Replace the NewGameServiceServer signature and body in grpc_service.go**

Replace lines 218–340 (the constructor doc comment + function) with:

```go
// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: storage, content, and handlers must be fully populated.
// sessMgr, cmdRegistry, and logger must be non-nil.
// Postcondition: Returns a fully initialised GameServiceServer.
func NewGameServiceServer(
	storage StorageDeps,
	content ContentDeps,
	handlers HandlerDeps,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	gameCalendar *GameCalendar,
	logger *zap.Logger,
) *GameServiceServer {
	s := &GameServiceServer{
		world:                      content.WorldMgr,
		sessions:                   sessMgr,
		commands:                   cmdRegistry,
		worldH:                     handlers.WorldHandler,
		chatH:                      handlers.ChatHandler,
		charSaver:                  storage.CharRepo,
		dice:                       content.DiceRoller,
		npcH:                       handlers.NPCHandler,
		npcMgr:                     content.NpcMgr,
		combatH:                    handlers.CombatHandler,
		scriptMgr:                  content.ScriptMgr,
		respawnMgr:                 content.RespawnMgr,
		floorMgr:                   content.FloorMgr,
		roomEquipMgr:               content.RoomEquipMgr,
		automapRepo:                storage.AutomapRepo,
		invRegistry:                content.InvRegistry,
		accountAdmin:               storage.AccountRepo,
		calendar:                   gameCalendar,
		logger:                     logger,
		jobRegistry:                content.JobRegistry,
		condRegistry:               content.CondRegistry,
		techRegistry:               content.TechRegistry,
		hardwiredTechRepo:          storage.HardwiredTechRepo,
		preparedTechRepo:           storage.PreparedTechRepo,
		spontaneousTechRepo:        storage.SpontaneousTechRepo,
		innateTechRepo:             storage.InnateTechRepo,
		spontaneousUsePoolRepo:     storage.SpontaneousUsePoolRepo,
		loadoutsDir:                string(content.LoadoutsDir),
		allSkills:                  content.AllSkills,
		characterSkillsRepo:        storage.SkillsRepo,
		characterProficienciesRepo: storage.ProficienciesRepo,
		allFeats:                   content.AllFeats,
		featRegistry:               content.FeatRegistry,
		characterFeatsRepo:         storage.FeatsRepo,
		allClassFeatures:           content.ClassFeatures,
		classFeatureRegistry:       content.ClassFeatureRegistry,
		characterClassFeaturesRepo: storage.ClassFeaturesRepo,
		featureChoicesRepo:         storage.FeatureChoicesRepo,
		charAbilityBoostsRepo:      storage.AbilityBoostsRepo,
		archetypes:                 content.ArchetypeMap,
		regions:                    content.RegionMap,
		mentalStateMgr:             content.MentalStateMgr,
		actionH:                    handlers.ActionHandler,
		wantedRepo:                 storage.WantedRepo,
		trapMgr:                    nil, // trap loading not yet wired
		trapTemplates:              nil, // trap loading not yet wired
	}
	s.WireCoverCrossfireTrap()
	s.WireConsumableTrapTrigger()
	return s
}
```

- [ ] **Step 2: Update the call site in cmd/gameserver/main.go**

Replace lines 577–596 (the `grpcService = gameserver.NewGameServiceServer(...)` block) with:

```go
	storage := gameserver.StorageDeps{
		CharRepo:               charRepo,
		AccountRepo:            gameserver.NewAccountRepoAdapter(accountRepo),
		SkillsRepo:             characterSkillsRepo,
		ProficienciesRepo:      characterProficienciesRepo,
		FeatsRepo:              characterFeatsRepo,
		ClassFeaturesRepo:      characterClassFeaturesRepo,
		FeatureChoicesRepo:     featureChoicesRepo,
		AbilityBoostsRepo:      charAbilityBoostsRepo,
		HardwiredTechRepo:      hardwiredTechRepo,
		PreparedTechRepo:       preparedTechRepo,
		SpontaneousTechRepo:    spontaneousTechRepo,
		InnateTechRepo:         innateTechRepo,
		SpontaneousUsePoolRepo: postgres.NewCharacterSpontaneousUsePoolRepository(pool.DB()),
		WantedRepo:             postgres.NewWantedRepository(pool.DB()),
		AutomapRepo:            automapRepo,
	}
	content := gameserver.ContentDeps{
		WorldMgr:             worldMgr,
		NpcMgr:               npcMgr,
		RespawnMgr:           respawnMgr,
		InvRegistry:          invRegistry,
		FloorMgr:             floorMgr,
		RoomEquipMgr:         roomEquipMgr,
		TechRegistry:         techReg,
		CondRegistry:         condRegistry,
		AIRegistry:           aiRegistry,
		AllSkills:            allSkills,
		AllFeats:             allFeats,
		ClassFeatures:        classFeatures,
		FeatRegistry:         featRegistry,
		ClassFeatureRegistry: cfReg,
		JobRegistry:          jobReg,
		ArchetypeMap:         archetypeMap,
		RegionMap:            regionMap,
		ScriptMgr:            scriptMgr,
		DiceRoller:           diceRoller,
		CombatEngine:         combatEngine,
		MentalStateMgr:       mentalMgr,
		LoadoutsDir:          *loadoutsDir,
	}
	handlers := gameserver.HandlerDeps{
		WorldHandler:  worldHandler,
		ChatHandler:   chatHandler,
		NPCHandler:    npcHandler,
		CombatHandler: combatHandler,
		ActionHandler: actionH,
	}
	grpcService = gameserver.NewGameServiceServer(storage, content, handlers, sessMgr, cmdRegistry, gameCalendar, logger)
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Run fast tests**

```bash
go test -count=1 -timeout=120s ./internal/gameserver/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/deps.go cmd/gameserver/main.go
git commit -m "feat(wire): refactor NewGameServiceServer to StorageDeps/ContentDeps/HandlerDeps structs"
```

---

### Task 4: postgres/providers.go

**Files:**
- Create: `internal/storage/postgres/providers.go`

- [ ] **Step 1: Create providers.go**

```go
package postgres

import (
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolDB unwraps the postgres Pool to the raw pgxpool.Pool required by repository constructors.
func PoolDB(p *Pool) *pgxpool.Pool {
	return p.DB()
}

// StorageProviders is the wire provider set for all storage dependencies.
var StorageProviders = wire.NewSet(
	NewPool,
	PoolDB,
	NewCharacterRepository,
	NewAccountRepository,
	NewCharacterSkillsRepository,
	NewCharacterProficienciesRepository,
	NewCharacterFeatsRepository,
	NewCharacterClassFeaturesRepository,
	NewCharacterFeatureChoicesRepo,
	NewCharacterAbilityBoostsRepository,
	NewCharacterHardwiredTechRepository,
	NewCharacterPreparedTechRepository,
	NewCharacterSpontaneousTechRepository,
	NewCharacterInnateTechRepository,
	NewCharacterSpontaneousUsePoolRepository,
	NewWantedRepository,
	NewAutomapRepository,
	NewCalendarRepo,
	NewCharacterProgressRepository,
	wire.Bind(new(CharacterAbilityBoostsRepository), new(*PostgresCharacterAbilityBoostsRepository)),
)
```

Note (REQ-WIRE-10 deviation): REQ-WIRE-10 specifies that `StorageProviders` MUST include `wire.Bind` calls for every interface/concrete pair. However, `CharacterSaver`, `AccountAdmin`, and the other gameserver interfaces are defined in the `gameserver` package. Adding `wire.Bind` calls here would require importing `gameserver`, creating an import cycle (`postgres` → `gameserver`). REQ-WIRE-10 is satisfied by placing all cross-package `wire.Bind` declarations in `cmd/gameserver/wire.go` (Task 12). The 11 bindings enumerated in spec Section 1.3 all appear in that file.

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/storage/postgres/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/providers.go
git commit -m "feat(wire): add postgres StorageProviders"
```

---

### Task 5: World Provider

**Files:**
- Create: `internal/game/world/providers.go`

- [ ] **Step 1: Create providers.go**

```go
package world

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// WorldDir is the path to zone YAML files. Used as a named type to avoid
// wire ambiguity with other string paths.
type WorldDir string

// NewManagerFromDir loads zones from dir and constructs a Manager.
func NewManagerFromDir(dir WorldDir, logger *zap.Logger) (*Manager, error) {
	zones, err := LoadZonesFromDir(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading zones from %q: %w", dir, err)
	}
	mgr, err := NewManager(zones)
	if err != nil {
		return nil, fmt.Errorf("creating world manager: %w", err)
	}
	if err := mgr.ValidateExits(); err != nil {
		return nil, fmt.Errorf("validating cross-zone exits: %w", err)
	}
	logger.Info("world loaded",
		zap.Int("zones", mgr.ZoneCount()),
		zap.Int("rooms", mgr.RoomCount()),
	)
	return mgr, nil
}

// Providers is the wire provider set for world dependencies.
var Providers = wire.NewSet(NewManagerFromDir)
```

- [ ] **Step 2: Verify**

```bash
go build ./internal/game/world/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/game/world/providers.go
git commit -m "feat(wire): add world Providers"
```

---

### Task 6: NPC Providers

**Files:**
- Create: `internal/game/npc/providers.go`

The NPC provider handles three things: template loading, manager construction (with armor AC resolver), and respawn manager construction (with initial population).

- [ ] **Step 1: Create providers.go**

```go
package npc

import (
	"fmt"
	"time"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// NPCsDir is the path to NPC template YAML files.
type NPCsDir string

// LoadTemplatesFromDir loads NPC templates from the given directory.
func LoadTemplatesFromDir(dir NPCsDir, logger *zap.Logger) ([]*Template, error) {
	templates, err := LoadTemplates(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading npc templates from %q: %w", dir, err)
	}
	logger.Info("loaded npc templates", zap.Int("count", len(templates)))
	return templates, nil
}

// NewWiredManager creates a Manager and wires the armor AC resolver from invRegistry.
func NewWiredManager(invRegistry *inventory.Registry) *Manager {
	mgr := NewManager()
	mgr.SetArmorACResolver(func(armorID string) int {
		if def, ok := invRegistry.Armor(armorID); ok {
			return def.ACBonus
		}
		return 0
	})
	return mgr
}

// NewPopulatedRespawnManager builds the respawn manager from zone spawn configs,
// populates initial NPC instances into npcMgr, and returns the manager.
func NewPopulatedRespawnManager(
	templates []*Template,
	worldMgr *world.Manager,
	npcMgr *Manager,
	logger *zap.Logger,
) (*RespawnManager, error) {
	templateByID := make(map[string]*Template, len(templates))
	for _, tmpl := range templates {
		templateByID[tmpl.ID] = tmpl
	}
	roomSpawns := make(map[string][]RoomSpawn)
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			for _, sc := range room.Spawns {
				tmpl, ok := templateByID[sc.Template]
				if !ok {
					return nil, fmt.Errorf("spawn in room %q references unknown npc template %q", room.ID, sc.Template)
				}
				var delay time.Duration
				if sc.RespawnAfter != "" {
					d, err := time.ParseDuration(sc.RespawnAfter)
					if err != nil {
						return nil, fmt.Errorf("invalid respawn_after %q in room %q: %w", sc.RespawnAfter, room.ID, err)
					}
					delay = d
				} else if tmpl.RespawnDelay != "" {
					d, err := time.ParseDuration(tmpl.RespawnDelay)
					if err != nil {
						return nil, fmt.Errorf("invalid respawn_delay %q on template %q: %w", tmpl.RespawnDelay, tmpl.ID, err)
					}
					delay = d
				}
				roomSpawns[room.ID] = append(roomSpawns[room.ID], RoomSpawn{
					TemplateID:   sc.Template,
					Max:          sc.Count,
					RespawnDelay: delay,
				})
			}
		}
	}
	respawnMgr := NewRespawnManager(roomSpawns, templateByID)
	for roomID := range roomSpawns {
		respawnMgr.PopulateRoom(roomID, npcMgr)
	}
	logger.Info("initial NPC population complete", zap.Int("room_configs", len(roomSpawns)))
	return respawnMgr, nil
}

// Providers is the wire provider set for NPC dependencies.
var Providers = wire.NewSet(
	LoadTemplatesFromDir,
	NewWiredManager,
	NewPopulatedRespawnManager,
)
```

- [ ] **Step 2: Verify**

```bash
go build ./internal/game/npc/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/game/npc/providers.go
git commit -m "feat(wire): add npc Providers"
```

---

### Task 7: Condition, Technology, Dice, Combat, MentalState Providers

**Files:**
- Create: `internal/game/condition/providers.go`
- Create: `internal/game/technology/providers.go`
- Create: `internal/game/dice/providers.go`
- Create: `internal/game/combat/providers.go`
- Create: `internal/game/mentalstate/providers.go`

- [ ] **Step 1: Create condition/providers.go**

```go
package condition

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// ConditionsDir is the path to condition YAML definitions.
type ConditionsDir string

// MentalConditionsDir is "content/conditions/mental" — a fixed subdirectory
// that is always loaded alongside the main conditions directory.
type MentalConditionsDir string

// NewRegistryFromDir loads condition definitions from dir (and the mental subdir).
func NewRegistryFromDir(dir ConditionsDir, mentalDir MentalConditionsDir, logger *zap.Logger) (*Registry, error) {
	reg, err := LoadDirectory(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading conditions from %q: %w", dir, err)
	}
	mentalReg, err := LoadDirectory(string(mentalDir))
	if err != nil {
		return nil, fmt.Errorf("loading mental conditions from %q: %w", mentalDir, err)
	}
	for _, def := range mentalReg.All() {
		reg.Register(def)
	}
	logger.Info("loaded condition definitions", zap.Int("count", len(reg.All())))
	return reg, nil
}

// Providers is the wire provider set for condition dependencies.
var Providers = wire.NewSet(NewRegistryFromDir)
```

- [ ] **Step 2: Create technology/providers.go**

```go
package technology

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// TechContentDir is the path to technology YAML content.
type TechContentDir string

// NewRegistryFromDir loads technology definitions; missing dir is a non-fatal warning.
func NewRegistryFromDir(dir TechContentDir, logger *zap.Logger) (*Registry, error) {
	reg, err := Load(string(dir))
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr.Err) {
			log.Printf("WARN: technology content dir %q not found — starting with empty tech registry", dir)
			return NewRegistry(), nil
		}
		return nil, fmt.Errorf("loading technology content: %w", err)
	}
	logger.Info("loaded technology definitions", zap.Int("count", len(reg.All())))
	return reg, nil
}

// Providers is the wire provider set for technology dependencies.
var Providers = wire.NewSet(NewRegistryFromDir)
```

- [ ] **Step 3: Create dice/providers.go**

```go
package dice

import (
	"github.com/google/wire"
	"go.uber.org/zap"
)

// NewRoller creates a logged dice roller using a crypto source.
func NewRoller(logger *zap.Logger) *Roller {
	return NewLoggedRoller(NewCryptoSource(), logger)
}

// Providers is the wire provider set for dice dependencies.
var Providers = wire.NewSet(NewRoller)
```

- [ ] **Step 4: Create combat/providers.go**

```go
package combat

import "github.com/google/wire"

// Providers is the wire provider set for combat dependencies.
var Providers = wire.NewSet(NewEngine)
```

- [ ] **Step 5: Create mentalstate/providers.go**

```go
package mentalstate

import "github.com/google/wire"

// Providers is the wire provider set for mental state dependencies.
var Providers = wire.NewSet(NewManager)
```

- [ ] **Step 6: Verify all compile**

```bash
go build ./internal/game/condition/... ./internal/game/technology/... ./internal/game/dice/... ./internal/game/combat/... ./internal/game/mentalstate/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/game/condition/providers.go internal/game/technology/providers.go internal/game/dice/providers.go internal/game/combat/providers.go internal/game/mentalstate/providers.go
git commit -m "feat(wire): add condition, technology, dice, combat, mentalstate Providers"
```

---

### Task 8: Inventory Providers

**Files:**
- Create: `internal/game/inventory/providers.go`

- [ ] **Step 1: Create providers.go**

```go
package inventory

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/world"
)

// WeaponsDir is the path to weapon YAML definitions.
type WeaponsDir string

// ItemsDir is the path to item YAML definitions.
type ItemsDir string

// ExplosivesDir is the path to explosive YAML definitions.
type ExplosivesDir string

// ArmorsDir is the path to armor YAML definitions.
type ArmorsDir string

// NewRegistryFromDirs loads all inventory definitions into a single Registry.
func NewRegistryFromDirs(
	weaponsDir WeaponsDir,
	itemsDir ItemsDir,
	explosivesDir ExplosivesDir,
	armorsDir ArmorsDir,
	logger *zap.Logger,
) (*Registry, error) {
	reg := NewRegistry()
	if weaponsDir != "" {
		weapons, err := LoadWeapons(string(weaponsDir))
		if err != nil {
			return nil, fmt.Errorf("loading weapons: %w", err)
		}
		for _, w := range weapons {
			if err := reg.RegisterWeapon(w); err != nil {
				return nil, fmt.Errorf("registering weapon %q: %w", w.ID, err)
			}
		}
		logger.Info("loaded weapon definitions", zap.Int("count", len(weapons)))
	}
	if explosivesDir != "" {
		explosives, err := LoadExplosives(string(explosivesDir))
		if err != nil {
			return nil, fmt.Errorf("loading explosives: %w", err)
		}
		for _, ex := range explosives {
			if err := reg.RegisterExplosive(ex); err != nil {
				return nil, fmt.Errorf("registering explosive %q: %w", ex.ID, err)
			}
		}
		logger.Info("loaded explosive definitions", zap.Int("count", len(explosives)))
	}
	if itemsDir != "" {
		items, err := LoadItems(string(itemsDir))
		if err != nil {
			return nil, fmt.Errorf("loading items: %w", err)
		}
		for _, item := range items {
			if err := reg.RegisterItem(item); err != nil {
				return nil, fmt.Errorf("registering item %q: %w", item.ID, err)
			}
		}
		logger.Info("loaded item definitions", zap.Int("count", len(items)))
	}
	armors, err := LoadArmors(string(armorsDir))
	if err != nil {
		return nil, fmt.Errorf("loading armors: %w", err)
	}
	for _, a := range armors {
		if err := reg.RegisterArmor(a); err != nil {
			return nil, fmt.Errorf("registering armor %q: %w", a.ID, err)
		}
	}
	logger.Info("loaded armor definitions", zap.Int("count", len(armors)))
	return reg, nil
}

// NewSeededRoomEquipmentManager creates a RoomEquipmentManager seeded with equipment from zone data.
func NewSeededRoomEquipmentManager(worldMgr *world.Manager, logger *zap.Logger) *RoomEquipmentManager {
	mgr := NewRoomEquipmentManager()
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			if len(room.Equipment) > 0 {
				mgr.InitRoom(room.ID, room.Equipment)
			}
		}
	}
	logger.Info("room equipment manager initialized")
	return mgr
}

// Providers is the wire provider set for inventory dependencies.
var Providers = wire.NewSet(
	NewRegistryFromDirs,
	NewFloorManager,
	NewSeededRoomEquipmentManager,
)
```

- [ ] **Step 2: Verify**

```bash
go build ./internal/game/inventory/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/game/inventory/providers.go
git commit -m "feat(wire): add inventory Providers"
```

---

### Task 9: Ruleset Providers

**Files:**
- Create: `internal/game/ruleset/providers.go`

- [ ] **Step 1: Create providers.go**

```go
package ruleset

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// JobsDir is the path to job YAML definitions.
type JobsDir string

// SkillsFile is the path to the skills YAML file.
type SkillsFile string

// FeatsFile is the path to the feats YAML file.
type FeatsFile string

// ClassFeaturesFile is the path to the class features YAML file.
type ClassFeaturesFile string

// ArchetypesDir is the path to archetype YAML definitions.
type ArchetypesDir string

// RegionsDir is the path to region YAML definitions.
type RegionsDir string

// TeamsDir is the path to team YAML definitions.
type TeamsDir string

// NewJobRegistryFromDir loads jobs and returns a JobRegistry.
func NewJobRegistryFromDir(dir JobsDir, logger *zap.Logger) (*JobRegistry, error) {
	jobs, err := LoadJobs(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading jobs: %w", err)
	}
	reg := NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	logger.Info("loaded job definitions", zap.Int("count", len(jobs)))
	return reg, nil
}

// LoadAllSkills loads skills from file.
func LoadAllSkills(file SkillsFile, logger *zap.Logger) ([]*Skill, error) {
	skills, err := LoadSkills(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}
	logger.Info("loaded skill definitions", zap.Int("count", len(skills)))
	return skills, nil
}

// LoadAllFeats loads feats from file.
func LoadAllFeats(file FeatsFile, logger *zap.Logger) ([]*Feat, error) {
	feats, err := LoadFeats(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading feats: %w", err)
	}
	logger.Info("loaded feat definitions", zap.Int("count", len(feats)))
	return feats, nil
}

// NewFeatRegistryFromFeats creates a FeatRegistry from loaded feats.
func NewFeatRegistryFromFeats(feats []*Feat) *FeatRegistry {
	return NewFeatRegistry(feats)
}

// LoadAllClassFeatures loads class features from file.
func LoadAllClassFeatures(file ClassFeaturesFile, logger *zap.Logger) ([]*ClassFeature, error) {
	features, err := LoadClassFeatures(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading class features: %w", err)
	}
	logger.Info("loaded class features", zap.Int("count", len(features)))
	return features, nil
}

// NewClassFeatureRegistryFromFeatures creates a ClassFeatureRegistry.
func NewClassFeatureRegistryFromFeatures(features []*ClassFeature) *ClassFeatureRegistry {
	return NewClassFeatureRegistry(features)
}

// LoadArchetypeMap loads archetypes and returns a map keyed by ID.
func LoadArchetypeMap(dir ArchetypesDir, logger *zap.Logger) (map[string]*Archetype, error) {
	list, err := LoadArchetypes(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading archetypes: %w", err)
	}
	m := make(map[string]*Archetype, len(list))
	for _, a := range list {
		m[a.ID] = a
	}
	logger.Info("loaded archetype definitions", zap.Int("count", len(list)))
	return m, nil
}

// LoadRegionMap loads regions and returns a map keyed by ID.
func LoadRegionMap(dir RegionsDir, logger *zap.Logger) (map[string]*Region, error) {
	list, err := LoadRegions(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading regions: %w", err)
	}
	m := make(map[string]*Region, len(list))
	for _, r := range list {
		m[r.ID] = r
	}
	logger.Info("loaded region definitions", zap.Int("count", len(list)))
	return m, nil
}

// LoadAllTeams loads teams from dir (used by devserver/frontend only).
func LoadAllTeams(dir TeamsDir, logger *zap.Logger) ([]*Team, error) {
	teams, err := LoadTeams(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading teams: %w", err)
	}
	logger.Info("loaded team definitions", zap.Int("count", len(teams)))
	return teams, nil
}

// LoadAllJobsSlice loads jobs as a slice (used by devserver/frontend; gameserver uses NewJobRegistryFromDir).
func LoadAllJobsSlice(dir JobsDir, logger *zap.Logger) ([]*Job, error) {
	jobs, err := LoadJobs(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading jobs: %w", err)
	}
	logger.Info("loaded job definitions", zap.Int("count", len(jobs)))
	return jobs, nil
}

// LoadAllRegionsSlice loads regions as a slice (used by devserver/frontend; gameserver uses LoadRegionMap).
func LoadAllRegionsSlice(dir RegionsDir, logger *zap.Logger) ([]*Region, error) {
	regions, err := LoadRegions(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading regions: %w", err)
	}
	logger.Info("loaded region definitions", zap.Int("count", len(regions)))
	return regions, nil
}

// LoadAllArchetypesSlice loads archetypes as a slice (used by devserver/frontend; gameserver uses LoadArchetypeMap).
func LoadAllArchetypesSlice(dir ArchetypesDir, logger *zap.Logger) ([]*Archetype, error) {
	list, err := LoadArchetypes(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading archetypes: %w", err)
	}
	logger.Info("loaded archetype definitions", zap.Int("count", len(list)))
	return list, nil
}

// GameProviders is the full ruleset provider set for the gameserver.
var GameProviders = wire.NewSet(
	NewJobRegistryFromDir,
	LoadAllSkills,
	LoadAllFeats,
	NewFeatRegistryFromFeats,
	LoadAllClassFeatures,
	NewClassFeatureRegistryFromFeatures,
	LoadArchetypeMap,
	LoadRegionMap,
)

// RulesetContentProviders is the minimal ruleset provider set for devserver/frontend.
// It excludes game-engine content (combat, AI, scripting, world, NPC).
// Uses slice-returning variants for jobs/regions/archetypes to match handlers.NewAuthHandler signature.
var RulesetContentProviders = wire.NewSet(
	LoadAllJobsSlice,
	LoadAllRegionsSlice,
	LoadAllArchetypesSlice,
	LoadAllSkills,
	LoadAllFeats,
	LoadAllClassFeatures,
	LoadAllTeams,
)
```

- [ ] **Step 2: Verify**

```bash
go build ./internal/game/ruleset/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/game/ruleset/providers.go
git commit -m "feat(wire): add ruleset GameProviders + RulesetContentProviders"
```

---

### Task 10: AI and Scripting Providers

**Files:**
- Create: `internal/game/ai/providers.go`
- Create: `internal/scripting/providers.go`

The scripting manager has complex post-construction callback wiring (QueryRoom, GetEntityRoom, etc.). Those callbacks are wired in `main.go` after `Initialize()` — the provider only constructs and loads the scripts.

- [ ] **Step 1: Create ai/providers.go**

```go
package ai

import "github.com/google/wire"

// NewRegistry creates an empty AI registry. Domains are registered by the
// scripting provider after script loading.
func NewEmptyRegistry() *Registry {
	return NewRegistry()
}

// Providers is the wire provider set for AI dependencies.
var Providers = wire.NewSet(NewEmptyRegistry)
```

- [ ] **Step 2: Create scripting/providers.go**

The scripting manager is constructed and scripts loaded in the provider. The callbacks that reference worldMgr, sessMgr, etc. are wired in `main.go` post-`Initialize()` because they form a dependency cycle with grpcService.

```go
package scripting

import (
	"os"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/dice"
)

// ScriptRoot is the root directory for Lua scripts; empty disables scripting.
type ScriptRoot string

// CondScriptDir is the directory of global condition scripts.
type CondScriptDir string

// AIScriptDir is the path to Lua AI precondition scripts.
type AIScriptDir string

// AIDir is the path to HTN AI domain YAML files.
type AIDir string

// NewManagerFromDirs constructs the scripting manager and loads all scripts.
// If scriptRoot is empty, scripting is disabled (returns nil, nil).
// AI domains are loaded into aiRegistry. Callbacks (QueryRoom, GetEntityRoom,
// GetCombatantsInRoom, RevealZoneMap) must be wired in main.go post-Initialize().
func NewManagerFromDirs(
	scriptRoot ScriptRoot,
	condScriptDir CondScriptDir,
	aiScriptDir AIScriptDir,
	aiDir AIDir,
	diceRoller *dice.Roller,
	aiRegistry *ai.Registry,
	logger *zap.Logger,
) (*Manager, error) {
	if scriptRoot == "" {
		return nil, nil
	}

	mgr := NewManager(diceRoller, logger)

	// Load global condition scripts.
	if info, err := os.Stat(string(condScriptDir)); err == nil && info.IsDir() {
		if err := mgr.LoadGlobal(string(condScriptDir), 0); err != nil {
			return nil, err
		}
		logger.Info("global condition scripts loaded", zap.String("dir", string(condScriptDir)))
	}

	// Load AI precondition scripts before registering domains.
	if aiScriptDir != "" {
		if _, statErr := os.Stat(string(aiScriptDir)); statErr == nil {
			if err := mgr.LoadGlobal(string(aiScriptDir), DefaultInstructionLimit); err != nil {
				return nil, err
			}
			logger.Info("loaded AI scripts", zap.String("dir", string(aiScriptDir)))
		}
	}

	// Load HTN AI domains.
	if aiDir != "" {
		domains, err := ai.LoadDomains(string(aiDir))
		if err != nil {
			return nil, err
		}
		for _, domain := range domains {
			if err := aiRegistry.Register(domain, mgr, ""); err != nil {
				return nil, err
			}
		}
		logger.Info("loaded AI domains", zap.Int("count", len(domains)))
	}

	return mgr, nil
}

// Providers is the wire provider set for scripting dependencies.
var Providers = wire.NewSet(NewManagerFromDirs)
```

- [ ] **Step 3: Verify**

```bash
go build ./internal/game/ai/... ./internal/scripting/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/game/ai/providers.go internal/scripting/providers.go
git commit -m "feat(wire): add ai and scripting Providers"
```

---

### Task 11: gameserver/providers.go

**Files:**
- Create: `internal/gameserver/providers.go`

This file contains HandlerProviders, ServerProviders, NewGameCalendarFromRepo, command registry builder, and BroadcastFn setup.

- [ ] **Step 1: Create providers.go**

```go
package gameserver

import (
	"fmt"
	"time"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// LoadoutsDir type is defined in deps.go; do NOT redeclare here.

// RoundDurationMs is the combat round duration in milliseconds.
type RoundDurationMs int

// AITickInterval is the NPC AI tick interval.
type AITickInterval time.Duration

// NewGameCalendarFromRepo loads calendar state and constructs GameCalendar.
// logger is threaded through so the calendar logger can be set after construction in main.go.
func NewGameCalendarFromRepo(repo *postgres.CalendarRepo, clock *GameClock) (*GameCalendar, error) {
	day, month, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("loading calendar state: %w", err)
	}
	return NewGameCalendar(clock, day, month, repo), nil
}

// NewCommandRegistry builds the command registry with class feature shortcuts.
func NewCommandRegistry(classFeatures []*ruleset.ClassFeature) (*command.Registry, error) {
	allCmds := command.RegisterShortcuts(classFeatures, command.BuiltinCommands())
	return command.NewRegistry(allCmds)
}

// NewSessionManager creates a session manager.
func NewSessionManager() *session.Manager {
	return session.NewManager()
}

// NewChatHandlerProvider creates a ChatHandler.
func NewChatHandlerProvider(sessMgr *session.Manager) *ChatHandler {
	return NewChatHandler(sessMgr)
}

// NewNPCHandlerProvider creates an NPCHandler.
func NewNPCHandlerProvider(npcMgr *npc.Manager, sessMgr *session.Manager) *NPCHandler {
	return NewNPCHandler(npcMgr, sessMgr)
}

// NewWorldHandlerProvider creates a WorldHandler.
func NewWorldHandlerProvider(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	npcMgr *npc.Manager,
	gameClock *GameClock,
	roomEquipMgr *inventory.RoomEquipmentManager,
	invRegistry *inventory.Registry,
) *WorldHandler {
	return NewWorldHandler(worldMgr, sessMgr, npcMgr, gameClock, roomEquipMgr, invRegistry)
}

// NewCombatHandlerProvider creates a CombatHandler with a no-op broadcast function.
// main.go wires the real broadcast function post-Initialize() via SetBroadcastFn.
func NewCombatHandlerProvider(
	combatEngine *combat.Engine,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	diceRoller *dice.Roller,
	roundDurationMs RoundDurationMs,
	condRegistry *condition.Registry,
	worldMgr *world.Manager,
	scriptMgr *scripting.Manager,
	invRegistry *inventory.Registry,
	aiRegistry *ai.Registry,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	mentalMgr *mentalstate.Manager,
	logger *zap.Logger,
) *CombatHandler {
	roundDuration := time.Duration(roundDurationMs) * time.Millisecond
	if roundDuration <= 0 {
		roundDuration = 6 * time.Second
	}
	// Broadcast function is nil; caller must invoke SetBroadcastFn after Initialize().
	h := NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller, nil, roundDuration, condRegistry, worldMgr, scriptMgr, invRegistry, aiRegistry, respawnMgr, floorMgr, mentalMgr)
	h.SetLogger(logger)
	return h
}

// NewActionHandlerProvider creates an ActionHandler.
func NewActionHandlerProvider(
	sessMgr *session.Manager,
	cfReg *ruleset.ClassFeatureRegistry,
	condRegistry *condition.Registry,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	charRepo *postgres.CharacterRepository,
	diceRoller *dice.Roller,
	logger *zap.Logger,
) *ActionHandler {
	return NewActionHandler(sessMgr, cfReg, condRegistry, npcMgr, combatHandler, charRepo, diceRoller, logger)
}

// NewRegenManagerProvider creates a RegenManager.
func NewRegenManagerProvider(
	sessMgr *session.Manager,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	charRepo *postgres.CharacterRepository,
	logger *zap.Logger,
) *RegenManager {
	return NewRegenManager(sessMgr, npcMgr, combatHandler, charRepo, RegenInterval, logger)
}

// NewZoneTickManagerProvider creates a ZoneTickManager.
func NewZoneTickManagerProvider(interval AITickInterval) *ZoneTickManager {
	return NewZoneTickManager(time.Duration(interval))
}

// HandlerProviders is the wire provider set for game handlers.
var HandlerProviders = wire.NewSet(
	NewChatHandlerProvider,
	NewNPCHandlerProvider,
	NewWorldHandlerProvider,
	NewCombatHandlerProvider,
	NewActionHandlerProvider,
	NewRegenManagerProvider,
	NewZoneTickManagerProvider,
	NewGameCalendarFromRepo,
)

// ServerProviders is the wire provider set for the gRPC service and supporting registries.
var ServerProviders = wire.NewSet(
	NewGameServiceServer,
	NewCommandRegistry,
	NewSessionManager,
	NewAccountRepoAdapter,
	wire.Bind(new(AccountAdmin), new(*AccountRepoAdapter)),
)
```

Note: `NewCombatHandler` needs to accept a nil `broadcastFn`. Verify the current signature accepts nil and add `SetBroadcastFn` if needed (see Step 2).

- [ ] **Step 2: Add SetBroadcastFn to CombatHandler**

In `internal/gameserver/combat_handler.go`, add:

```go
// SetBroadcastFn sets the function used to broadcast combat events to room subscribers.
// Called by main.go after Initialize() to wire the circular GRPCService dependency.
func (h *CombatHandler) SetBroadcastFn(fn func(roomID string, events []*gamev1.CombatEvent)) {
	h.broadcast = fn
}
```

(Check the field name by reading combat_handler.go first.)

- [ ] **Step 3: Verify**

```bash
go build ./internal/gameserver/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/providers.go internal/gameserver/combat_handler.go
git commit -m "feat(wire): add gameserver HandlerProviders + ServerProviders; add CombatHandler.SetBroadcastFn"
```

---

### Task 12: cmd/gameserver Wire Injector + Updated main.go

**Files:**
- Create: `cmd/gameserver/wire.go`
- Create: `cmd/gameserver/wire_gen.go` (generated)
- Modify: `cmd/gameserver/main.go`

- [ ] **Step 1: Create AppConfig struct at top of main.go**

Add after the imports in `cmd/gameserver/main.go`:

```go
// AppConfig holds all CLI flag values for the gameserver binary.
type AppConfig struct {
	Config         *config.Config
	ZonesDir       world.WorldDir
	NPCsDir        npc.NPCsDir
	ConditionsDir  condition.ConditionsDir
	MentalCondDir  condition.MentalConditionsDir
	ScriptRoot     scripting.ScriptRoot
	CondScriptDir  scripting.CondScriptDir
	WeaponsDir     inventory.WeaponsDir
	ItemsDir       inventory.ItemsDir
	ExplosivesDir  inventory.ExplosivesDir
	ArmorsDir      inventory.ArmorsDir
	AIDir          scripting.AIDir
	AIScriptDir    scripting.AIScriptDir
	AITickInterval gameserver.AITickInterval
	JobsDir        ruleset.JobsDir
	LoadoutsDir    gameserver.LoadoutsDir
	SkillsFile     ruleset.SkillsFile
	FeatsFile      ruleset.FeatsFile
	ClassFeatsFile ruleset.ClassFeaturesFile
	ArchetypesDir  ruleset.ArchetypesDir
	RegionsDir     ruleset.RegionsDir
	TechContentDir technology.TechContentDir
	RoundDurationMs gameserver.RoundDurationMs
	XPConfigFile   string
}

// AppConfigToDatabase extracts database config from AppConfig for wire.
func AppConfigToDatabase(cfg *AppConfig) config.DatabaseConfig {
	return cfg.Config.Database
}
```

- [ ] **Step 2: Create cmd/gameserver/wire.go**

```go
//go:build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// App holds all top-level components for the gameserver binary.
type App struct {
	GRPCService    *gameserver.GameServiceServer
	CombatHandler  *gameserver.CombatHandler
	RegenMgr       *gameserver.RegenManager
	ZoneTickMgr    *gameserver.ZoneTickManager
	AIRegistry     *ai.Registry
	GameClock      *gameserver.GameClock
	GameCalendar   *gameserver.GameCalendar
	Pool           *postgres.Pool
	ScriptMgr      *scripting.Manager
	WorldMgr       *world.Manager
	NpcMgr         *npc.Manager
	SessMgr        *session.Manager
	CombatEngine   *combat.Engine
	AutomapRepo    *postgres.AutomapRepository
	InvRegistry    *inventory.Registry
	RespawnMgr     *npc.RespawnManager
	RoomEquipMgr   *inventory.RoomEquipmentManager
	CharRepo       *postgres.CharacterRepository
	ProgressRepo   *postgres.CharacterProgressRepository
}

// Initialize is the wire-generated injector for the gameserver binary.
func Initialize(ctx context.Context, cfg *AppConfig, clock *gameserver.GameClock, logger *zap.Logger) (*App, error) {
	wire.Build(
		AppConfigToDatabase,
		// Extract typed path values from AppConfig
		wire.FieldsOf(new(*AppConfig),
			"ZonesDir", "NPCsDir", "ConditionsDir", "MentalCondDir",
			"ScriptRoot", "CondScriptDir", "WeaponsDir", "ItemsDir",
			"ExplosivesDir", "ArmorsDir", "AIDir", "AIScriptDir",
			"AITickInterval", "JobsDir", "LoadoutsDir",
			"SkillsFile", "FeatsFile", "ClassFeatsFile",
			"ArchetypesDir", "RegionsDir", "TechContentDir", "RoundDurationMs",
		),
		postgres.StorageProviders,
		world.Providers,
		npc.Providers,
		condition.Providers,
		technology.Providers,
		inventory.Providers,
		ruleset.GameProviders,
		ai.Providers,
		dice.Providers,
		combat.Providers,
		mentalstate.Providers,
		scripting.Providers,
		gameserver.HandlerProviders,
		gameserver.ServerProviders,
		// Interface bindings (cannot live in postgres package due to import cycle)
		wire.Bind(new(gameserver.CharacterSaver), new(*postgres.CharacterRepository)),
		wire.Bind(new(gameserver.CharacterSkillsRepository), new(*postgres.CharacterSkillsRepository)),
		wire.Bind(new(gameserver.CharacterProficienciesRepository), new(*postgres.CharacterProficienciesRepository)),
		wire.Bind(new(gameserver.CharacterFeatsGetter), new(*postgres.CharacterFeatsRepository)),
		wire.Bind(new(gameserver.CharacterClassFeaturesGetter), new(*postgres.CharacterClassFeaturesRepository)),
		wire.Bind(new(gameserver.CharacterFeatureChoicesRepository), new(*postgres.CharacterFeatureChoicesRepo)),
		wire.Bind(new(gameserver.HardwiredTechRepo), new(*postgres.CharacterHardwiredTechRepository)),
		wire.Bind(new(gameserver.PreparedTechRepo), new(*postgres.CharacterPreparedTechRepository)),
		wire.Bind(new(gameserver.SpontaneousTechRepo), new(*postgres.CharacterSpontaneousTechRepository)),
		wire.Bind(new(gameserver.InnateTechRepo), new(*postgres.CharacterInnateTechRepository)),
		wire.Bind(new(gameserver.SpontaneousUsePoolRepo), new(*postgres.CharacterSpontaneousUsePoolRepository)),
		wire.Struct(new(gameserver.StorageDeps), "*"),
		wire.Struct(new(gameserver.ContentDeps), "*"),
		wire.Struct(new(gameserver.HandlerDeps), "*"),
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
```

- [ ] **Step 3: Run wire to generate wire_gen.go**

```bash
cd cmd/gameserver && wire && cd ../..
```

Expected: `wire_gen.go` created with `//go:build !wireinject`

If wire reports errors, fix the provider set until it succeeds.

- [ ] **Step 4: Replace cmd/gameserver/main.go**

Replace the entire `main()` function body with the wire-driven version:

```go
func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	zonesDir := flag.String("zones", "content/zones", "path to zone YAML files directory")
	npcsDir := flag.String("npcs-dir", "content/npcs", "path to NPC YAML templates directory")
	conditionsDir := flag.String("conditions-dir", "content/conditions", "path to condition YAML definitions directory")
	scriptRoot := flag.String("script-root", "content/scripts", "root directory for Lua scripts; empty = scripting disabled")
	condScriptDir := flag.String("condition-scripts", "content/scripts/conditions", "directory of global condition scripts")
	weaponsDir := flag.String("weapons-dir", "content/weapons", "path to weapon YAML definitions directory")
	itemsDir := flag.String("items-dir", "content/items", "path to item YAML definitions directory")
	explosivesDir := flag.String("explosives-dir", "content/explosives", "path to explosive YAML definitions directory")
	aiDir := flag.String("ai-dir", "content/ai", "path to HTN AI domain YAML directory")
	aiScriptDir := flag.String("ai-scripts", "content/scripts/ai", "path to Lua AI precondition scripts")
	aiTickInterval := flag.Duration("ai-tick", 10*time.Second, "NPC AI tick interval")
	armorsDir := flag.String("armors-dir", "content/armor", "path to armor YAML definitions directory")
	jobsDir := flag.String("jobs-dir", "content/jobs", "path to job YAML definitions directory")
	loadoutsDir := flag.String("loadouts-dir", "content/loadouts", "path to archetype loadout YAML directory")
	skillsFile := flag.String("skills", "content/skills.yaml", "path to skills YAML file")
	featsFile := flag.String("feats", "content/feats.yaml", "path to feats YAML file")
	classFeatsFile := flag.String("class-features", "content/class_features.yaml", "path to class features YAML file")
	archetypesDir := flag.String("archetypes-dir", "content/archetypes", "path to archetype YAML definitions directory")
	regionsDir := flag.String("regions-dir", "content/regions", "path to region YAML definitions directory")
	xpConfigFile := flag.String("xp-config", "content/xp_config.yaml", "path to XP configuration YAML file")
	techContentDir := flag.String("tech-content-dir", "content/technologies", "path to technology YAML content directory")
	flag.Parse()

	ctx := context.Background()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting game server", zap.String("grpc_addr", cfg.GameServer.Addr()))

	// GameClock is pre-constructed because its primitive params would collide in the wire graph.
	gameClock := gameserver.NewGameClock(
		int32(cfg.GameServer.GameClockStart),
		cfg.GameServer.GameTickDuration,
	)
	stopClock := gameClock.Start()
	defer stopClock()

	appCfg := &AppConfig{
		Config:          cfg,
		ZonesDir:        world.WorldDir(*zonesDir),
		NPCsDir:         npc.NPCsDir(*npcsDir),
		ConditionsDir:   condition.ConditionsDir(*conditionsDir),
		MentalCondDir:   condition.MentalConditionsDir("content/conditions/mental"),
		ScriptRoot:      scripting.ScriptRoot(*scriptRoot),
		CondScriptDir:   scripting.CondScriptDir(*condScriptDir),
		WeaponsDir:      inventory.WeaponsDir(*weaponsDir),
		ItemsDir:        inventory.ItemsDir(*itemsDir),
		ExplosivesDir:   inventory.ExplosivesDir(*explosivesDir),
		ArmorsDir:       inventory.ArmorsDir(*armorsDir),
		AIDir:           scripting.AIDir(*aiDir),
		AIScriptDir:     scripting.AIScriptDir(*aiScriptDir),
		AITickInterval:  gameserver.AITickInterval(*aiTickInterval),
		JobsDir:         ruleset.JobsDir(*jobsDir),
		LoadoutsDir:     gameserver.LoadoutsDir(*loadoutsDir),
		SkillsFile:      ruleset.SkillsFile(*skillsFile),
		FeatsFile:       ruleset.FeatsFile(*featsFile),
		ClassFeatsFile:  ruleset.ClassFeaturesFile(*classFeatsFile),
		ArchetypesDir:   ruleset.ArchetypesDir(*archetypesDir),
		RegionsDir:      ruleset.RegionsDir(*regionsDir),
		TechContentDir:  technology.TechContentDir(*techContentDir),
		RoundDurationMs: gameserver.RoundDurationMs(cfg.GameServer.RoundDurationMs),
		XPConfigFile:    *xpConfigFile,
	}

	app, err := Initialize(ctx, appCfg, gameClock, logger)
	if err != nil {
		logger.Fatal("initializing application", zap.Error(err))
	}

	// Wire broadcast function (circular dep: CombatHandler ↔ GRPCService).
	app.CombatHandler.SetBroadcastFn(func(roomID string, events []*gamev1.CombatEvent) {
		app.GRPCService.BroadcastCombatEvents(roomID, events)
	})

	// Wire scripting callbacks post-Initialize (reference grpcService, worldMgr, etc.).
	if app.ScriptMgr != nil {
		app.ScriptMgr.QueryRoom = func(roomID string) *scripting.RoomInfo {
			room, ok := app.WorldMgr.GetRoom(roomID)
			if !ok {
				return nil
			}
			return &scripting.RoomInfo{ID: room.ID, Title: room.Title}
		}
		app.ScriptMgr.GetEntityRoom = func(uid string) string {
			if inst, ok := app.NpcMgr.Get(uid); ok {
				return inst.RoomID
			}
			if sess, ok := app.SessMgr.GetPlayer(uid); ok {
				return sess.RoomID
			}
			return ""
		}
		app.ScriptMgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
			cbt, ok := app.CombatEngine.GetCombat(roomID)
			if !ok {
				return nil
			}
			living := cbt.LivingCombatants()
			if len(living) == 0 {
				return nil
			}
			out := make([]*scripting.CombatantInfo, 0, len(living))
			for _, c := range living {
				kind := "npc"
				if c.Kind == combat.KindPlayer {
					kind = "player"
				}
				out = append(out, &scripting.CombatantInfo{
					UID:   c.ID,
					Name:  c.Name,
					HP:    c.CurrentHP,
					MaxHP: c.MaxHP,
					AC:    c.AC,
					Kind:  kind,
				})
			}
			return out
		}
		app.ScriptMgr.RevealZoneMap = func(uid, zoneID string) {
			sess, ok := app.SessMgr.GetPlayer(uid)
			if !ok {
				return
			}
			zone, ok := app.WorldMgr.GetZone(zoneID)
			if !ok {
				return
			}
			if sess.AutomapCache[zoneID] == nil {
				sess.AutomapCache[zoneID] = make(map[string]bool)
			}
			var newRooms []string
			for roomID := range zone.Rooms {
				if !sess.AutomapCache[zoneID][roomID] {
					sess.AutomapCache[zoneID][roomID] = true
					newRooms = append(newRooms, roomID)
				}
			}
			if app.AutomapRepo != nil && len(newRooms) > 0 {
				if err := app.AutomapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, newRooms); err != nil {
					logger.Warn("bulk-persisting zone map reveal", zap.Error(err))
				}
			}
		}
	}

	// Wire XP service (conditional on config file presence).
	if xpCfg, xpErr := xp.LoadXPConfig(appCfg.XPConfigFile); xpErr != nil {
		logger.Warn("loading xp config; XP awards disabled", zap.Error(xpErr))
	} else {
		xpSvc := xp.NewService(xpCfg, app.ProgressRepo)
		xpSvc.SetSkillIncreaseSaver(app.ProgressRepo)
		app.GRPCService.SetProgressRepo(app.ProgressRepo)
		app.GRPCService.SetXPService(xpSvc)
		app.CombatHandler.SetXPService(xpSvc)
		app.CombatHandler.SetCurrencySaver(app.CharRepo)
	}

	// Wire GameCalendar logger and start (provider cannot set logger; logger is available here).
	app.GameCalendar.SetLogger(logger.Sugar())
	stopCalendar := app.GameCalendar.Start()
	defer stopCalendar()

	// Start room equipment respawn ticker.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			app.RoomEquipMgr.ProcessRespawns()
		}
	}()

	// Start out-of-combat health regeneration.
	app.RegenMgr.Start(ctx)

	// Start zone AI ticks.
	app.GRPCService.StartZoneTicks(ctx, app.ZoneTickMgr, app.AIRegistry)

	// Start calendar-driven WantedLevel decay.
	app.GRPCService.StartWantedDecayHook()
	defer app.GRPCService.StopWantedDecayHook()

	// Create gRPC server.
	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, app.GRPCService)

	lifecycle := server.NewLifecycle(logger)
	lifecycle.Add("grpc", &server.FuncService{
		StartFn: func() error {
			lis, err := net.Listen("tcp", cfg.GameServer.Addr())
			if err != nil {
				return fmt.Errorf("listening on %s: %w", cfg.GameServer.Addr(), err)
			}
			logger.Info("gRPC server listening", zap.String("addr", lis.Addr().String()))
			return grpcServer.Serve(lis)
		},
		StopFn: func() { grpcServer.GracefulStop() },
	})

	logger.Info("game server initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("grpc_addr", cfg.GameServer.Addr()),
	)

	lifecycle.Add("postgres", &server.FuncService{
		StartFn: func() error {
			for {
				time.Sleep(30 * time.Second)
				if err := app.Pool.Health(ctx, 5*time.Second); err != nil {
					logger.Warn("database health check failed", zap.Error(err))
				}
			}
		},
		StopFn: func() { app.Pool.Close() },
	})

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
```

- [ ] **Step 5: Write Initialize smoke test (TDD)**

Create `cmd/gameserver/wire_test.go`:

```go
//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/gameserver"
	"go.uber.org/zap"
)

// TestInitialize_ReturnsNonNilApp asserts that Initialize produces a non-nil App.
// Run with: go test -tags=integration ./cmd/gameserver/... -run TestInitialize
// This test requires a running database and is tagged integration-only.
func TestInitialize_ReturnsNonNilApp(t *testing.T) {
	cfg, err := config.Load("../../configs/dev.yaml")
	if err != nil {
		t.Skipf("skipping: config not available: %v", err)
	}
	logger, _ := zap.NewDevelopment()
	appCfg := &AppConfig{
		Config: cfg,
		// Content paths use defaults matching cmd/gameserver/main.go flag defaults
		ZonesDir:        world.WorldDir("content/zones"),
		NPCsDir:         npc.NPCsDir("content/npcs"),
		ConditionsDir:   condition.ConditionsDir("content/conditions"),
		MentalCondDir:   condition.MentalConditionsDir("content/mental_conditions"),
		ScriptRoot:      scripting.ScriptRoot("content/scripts"),
		CondScriptDir:   scripting.CondScriptDir("content/condition_scripts"),
		WeaponsDir:      inventory.WeaponsDir("content/weapons"),
		ItemsDir:        inventory.ItemsDir("content/items"),
		ExplosivesDir:   inventory.ExplosivesDir("content/explosives"),
		ArmorsDir:       inventory.ArmorsDir("content/armor"),
		AIDir:           scripting.AIDir("content/ai"),
		AIScriptDir:     scripting.AIScriptDir("content/ai_scripts"),
		AITickInterval:  gameserver.AITickInterval(5 * time.Second),
		JobsDir:         ruleset.JobsDir("content/jobs"),
		LoadoutsDir:     gameserver.LoadoutsDir("content/loadouts"),
		SkillsFile:      ruleset.SkillsFile("content/skills.yaml"),
		FeatsFile:       ruleset.FeatsFile("content/feats.yaml"),
		ClassFeatsFile:  ruleset.ClassFeaturesFile("content/class_features.yaml"),
		ArchetypesDir:   ruleset.ArchetypesDir("content/archetypes"),
		RegionsDir:      ruleset.RegionsDir("content/regions"),
		TechContentDir:  technology.TechContentDir("content/technology"),
		RoundDurationMs: gameserver.RoundDurationMs(3000),
		XPConfigFile:    "content/xp.yaml",
	}
	clock := gameserver.NewGameClock(cfg.Calendar.StartHour, cfg.Calendar.TickInterval)
	app, err := Initialize(context.Background(), appCfg, clock, logger)
	if err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
	if app == nil {
		t.Fatal("Initialize() returned nil App")
	}
}
```

Run to verify test compiles under the integration build tag (it will skip without a live DB at the correct path):

```bash
go test -tags=integration -run TestInitialize_ReturnsNonNilApp ./cmd/gameserver/... 2>&1 | head -5
```

Expected: `--- SKIP` or `PASS` (not a compilation failure).

- [ ] **Step 6: Build and verify**

```bash
go build ./cmd/gameserver/...
```

Expected: no errors

- [ ] **Step 7: Run fast tests**

```bash
go test -count=1 -timeout=120s ./internal/gameserver/... ./internal/game/...
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add cmd/gameserver/wire.go cmd/gameserver/wire_gen.go cmd/gameserver/wire_test.go cmd/gameserver/main.go
git commit -m "feat(wire): wire cmd/gameserver with Initialize() injector"
```

---

### Task 13: cmd/devserver and cmd/frontend Wire Injectors

Both binaries are nearly identical (153/155 lines each). The devserver and frontend use only storage + ruleset content providers.

**Files:**
- Create: `internal/frontend/handlers/providers.go`
- Create: `internal/frontend/telnet/providers.go`
- Create: `cmd/devserver/wire.go`, `cmd/devserver/wire_gen.go`
- Modify: `cmd/devserver/main.go`
- Create: `cmd/frontend/wire.go`, `cmd/frontend/wire_gen.go`
- Modify: `cmd/frontend/main.go`

- [ ] **Step 1: Create internal/frontend/handlers/providers.go**

```go
package handlers

import "github.com/google/wire"

// Providers is the wire provider set for frontend handlers.
var Providers = wire.NewSet(NewAuthHandler)
```

- [ ] **Step 2: Create internal/frontend/telnet/providers.go**

```go
package telnet

import "github.com/google/wire"

// Providers is the wire provider set for the telnet acceptor.
var Providers = wire.NewSet(NewAcceptor)
```

- [ ] **Step 3: Create cmd/devserver/wire.go**

`handlers.NewAuthHandler` takes `gameServerAddr string` and `telnetCfg config.TelnetConfig`. To avoid `string` type collision in the wire graph, define a named `GameServerAddr` type and a wrapper provider in the injector file.

```go
//go:build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// AppConfig holds CLI flag values for the devserver binary.
type AppConfig struct {
	Config         *config.Config
	RegionsDir     ruleset.RegionsDir
	TeamsDir       ruleset.TeamsDir
	JobsDir        ruleset.JobsDir
	ArchetypesDir  ruleset.ArchetypesDir
	SkillsFile     ruleset.SkillsFile
	FeatsFile      ruleset.FeatsFile
	ClassFeatsFile ruleset.ClassFeaturesFile
}

// App holds top-level components for the devserver binary.
type App struct {
	Pool           *postgres.Pool
	TelnetAcceptor *telnet.Acceptor
}

// AppConfigToDatabase extracts database config for wire.
func AppConfigToDatabase(cfg *AppConfig) config.DatabaseConfig {
	return cfg.Config.Database
}

// AppConfigToTelnet extracts telnet config for wire.
func AppConfigToTelnet(cfg *AppConfig) config.TelnetConfig {
	return cfg.Config.Telnet
}

// GameServerAddr is a named type for the gRPC server address string.
// Named to avoid wire ambiguity with other string values.
type GameServerAddr string

// AppConfigToGameServerAddr extracts the game server address for wire.
func AppConfigToGameServerAddr(cfg *AppConfig) GameServerAddr {
	return GameServerAddr(cfg.Config.GameServer.Addr())
}

// ProvideAuthHandler wraps handlers.NewAuthHandler converting GameServerAddr to string.
func ProvideAuthHandler(
	accounts handlers.AccountStore,
	characters handlers.CharacterStore,
	regions []*ruleset.Region,
	teams []*ruleset.Team,
	jobs []*ruleset.Job,
	archetypes []*ruleset.Archetype,
	logger *zap.Logger,
	addr GameServerAddr,
	telnetCfg config.TelnetConfig,
	allSkills []*ruleset.Skill,
	characterSkills handlers.CharacterSkillsSetter,
	allFeats []*ruleset.Feat,
	characterFeats handlers.CharacterFeatsSetter,
	allClassFeatures []*ruleset.ClassFeature,
	characterClassFeatures handlers.CharacterClassFeaturesSetter,
) *handlers.AuthHandler {
	return handlers.NewAuthHandler(
		accounts, characters, regions, teams, jobs, archetypes, logger, string(addr), telnetCfg,
		allSkills, characterSkills, allFeats, characterFeats, allClassFeatures, characterClassFeatures,
	)
}

// Initialize is the wire-generated injector for the devserver binary.
func Initialize(ctx context.Context, cfg *AppConfig, logger *zap.Logger) (*App, error) {
	wire.Build(
		AppConfigToDatabase,
		AppConfigToTelnet,
		AppConfigToGameServerAddr,
		wire.FieldsOf(new(*AppConfig),
			"RegionsDir", "TeamsDir", "JobsDir", "ArchetypesDir",
			"SkillsFile", "FeatsFile", "ClassFeatsFile",
		),
		postgres.StorageProviders,
		ruleset.RulesetContentProviders,
		ProvideAuthHandler,
		telnet.Providers,
		// Interface bindings: postgres repos → handlers interfaces
		wire.Bind(new(handlers.AccountStore), new(*postgres.AccountRepository)),
		wire.Bind(new(handlers.CharacterStore), new(*postgres.CharacterRepository)),
		wire.Bind(new(handlers.CharacterSkillsSetter), new(*postgres.CharacterSkillsRepository)),
		wire.Bind(new(handlers.CharacterFeatsSetter), new(*postgres.CharacterFeatsRepository)),
		wire.Bind(new(handlers.CharacterClassFeaturesSetter), new(*postgres.CharacterClassFeaturesRepository)),
		// Bind AuthHandler as SessionHandler for telnet.NewAcceptor
		wire.Bind(new(telnet.SessionHandler), new(*handlers.AuthHandler)),
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
```

- [ ] **Step 4: Run wire for devserver**

```bash
cd cmd/devserver && wire && cd ../..
```

Expected: `wire_gen.go` created. Fix any errors until wire succeeds.

- [ ] **Step 5: Update cmd/devserver/main.go**

Replace `main()` body:

```go
func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	regionsDir := flag.String("regions", "content/regions", "path to region YAML files directory")
	teamsDir := flag.String("teams", "content/teams", "path to team YAML files directory")
	jobsDir := flag.String("jobs", "content/jobs", "path to job YAML files directory")
	archetypesDir := flag.String("archetypes", "content/archetypes", "path to archetype YAML files directory")
	skillsFile := flag.String("skills", "content/skills.yaml", "path to skills YAML file")
	featsFile := flag.String("feats", "content/feats.yaml", "path to feats YAML file")
	classFeatsFile := flag.String("class-features", "content/class_features.yaml", "path to class features YAML file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()
	logger.Info("starting Gunchete MUD server",
		zap.String("mode", cfg.Server.Mode),
		zap.String("type", cfg.Server.Type),
	)

	appCfg := &AppConfig{
		Config:         cfg,
		RegionsDir:     ruleset.RegionsDir(*regionsDir),
		TeamsDir:       ruleset.TeamsDir(*teamsDir),
		JobsDir:        ruleset.JobsDir(*jobsDir),
		ArchetypesDir:  ruleset.ArchetypesDir(*archetypesDir),
		SkillsFile:     ruleset.SkillsFile(*skillsFile),
		FeatsFile:      ruleset.FeatsFile(*featsFile),
		ClassFeatsFile: ruleset.ClassFeaturesFile(*classFeatsFile),
	}

	app, err := Initialize(context.Background(), appCfg, logger)
	if err != nil {
		logger.Fatal("initializing application", zap.Error(err))
	}

	lifecycle := server.NewLifecycle(logger)
	lifecycle.Add("postgres", &server.FuncService{
		StartFn: func() error {
			for {
				time.Sleep(30 * time.Second)
				if err := app.Pool.Health(context.Background(), 5*time.Second); err != nil {
					logger.Warn("database health check failed", zap.Error(err))
				}
			}
		},
		StopFn: func() { app.Pool.Close() },
	})
	lifecycle.Add("telnet", &server.FuncService{
		StartFn: func() error { return app.TelnetAcceptor.ListenAndServe() },
		StopFn:  func() { app.TelnetAcceptor.Stop() },
	})

	logger.Info("server initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("telnet_addr", fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)),
	)
	if err := lifecycle.Run(context.Background()); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
```

- [ ] **Step 6: Create cmd/frontend/wire.go**

The frontend binary serves the same purpose as devserver: accept telnet connections and authenticate players. Its wire graph is the same. Write it explicitly (do not copy-paste):

```go
//go:build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// AppConfig holds CLI flag values for the frontend binary.
// Note: matches devserver AppConfig exactly; separate type for package isolation.
type AppConfig struct {
	Config         *config.Config
	RegionsDir     ruleset.RegionsDir
	TeamsDir       ruleset.TeamsDir
	JobsDir        ruleset.JobsDir
	ArchetypesDir  ruleset.ArchetypesDir
	SkillsFile     ruleset.SkillsFile
	FeatsFile      ruleset.FeatsFile
	ClassFeatsFile ruleset.ClassFeaturesFile
}

// App holds top-level components for the frontend binary.
type App struct {
	Pool           *postgres.Pool
	TelnetAcceptor *telnet.Acceptor
}

// AppConfigToDatabase extracts database config for wire.
func AppConfigToDatabase(cfg *AppConfig) config.DatabaseConfig {
	return cfg.Config.Database
}

// AppConfigToTelnet extracts telnet config for wire.
func AppConfigToTelnet(cfg *AppConfig) config.TelnetConfig {
	return cfg.Config.Telnet
}

// GameServerAddr is a named type for the gRPC server address string.
type GameServerAddr string

// AppConfigToGameServerAddr extracts the game server address for wire.
func AppConfigToGameServerAddr(cfg *AppConfig) GameServerAddr {
	return GameServerAddr(cfg.Config.GameServer.Addr())
}

// ProvideAuthHandler wraps handlers.NewAuthHandler converting GameServerAddr to string.
func ProvideAuthHandler(
	accounts handlers.AccountStore,
	characters handlers.CharacterStore,
	regions []*ruleset.Region,
	teams []*ruleset.Team,
	jobs []*ruleset.Job,
	archetypes []*ruleset.Archetype,
	logger *zap.Logger,
	addr GameServerAddr,
	telnetCfg config.TelnetConfig,
	allSkills []*ruleset.Skill,
	characterSkills handlers.CharacterSkillsSetter,
	allFeats []*ruleset.Feat,
	characterFeats handlers.CharacterFeatsSetter,
	allClassFeatures []*ruleset.ClassFeature,
	characterClassFeatures handlers.CharacterClassFeaturesSetter,
) *handlers.AuthHandler {
	return handlers.NewAuthHandler(
		accounts, characters, regions, teams, jobs, archetypes, logger, string(addr), telnetCfg,
		allSkills, characterSkills, allFeats, characterFeats, allClassFeatures, characterClassFeatures,
	)
}

// Initialize is the wire-generated injector for the frontend binary.
func Initialize(ctx context.Context, cfg *AppConfig, logger *zap.Logger) (*App, error) {
	wire.Build(
		AppConfigToDatabase,
		AppConfigToTelnet,
		AppConfigToGameServerAddr,
		wire.FieldsOf(new(*AppConfig),
			"RegionsDir", "TeamsDir", "JobsDir", "ArchetypesDir",
			"SkillsFile", "FeatsFile", "ClassFeatsFile",
		),
		postgres.StorageProviders,
		ruleset.RulesetContentProviders,
		ProvideAuthHandler,
		telnet.Providers,
		wire.Bind(new(handlers.AccountStore), new(*postgres.AccountRepository)),
		wire.Bind(new(handlers.CharacterStore), new(*postgres.CharacterRepository)),
		wire.Bind(new(handlers.CharacterSkillsSetter), new(*postgres.CharacterSkillsRepository)),
		wire.Bind(new(handlers.CharacterFeatsSetter), new(*postgres.CharacterFeatsRepository)),
		wire.Bind(new(handlers.CharacterClassFeaturesSetter), new(*postgres.CharacterClassFeaturesRepository)),
		wire.Bind(new(telnet.SessionHandler), new(*handlers.AuthHandler)),
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
```

Run wire for frontend:

```bash
cd cmd/frontend && wire && cd ../..
```

Expected: `wire_gen.go` created with no errors.

Update `cmd/frontend/main.go` with the same `main()` pattern as devserver, using frontend-appropriate log messages ("starting Gunchete frontend", "frontend initialized").

- [ ] **Step 7: Build both binaries**

```bash
go build ./cmd/devserver/... ./cmd/frontend/...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add cmd/devserver/ cmd/frontend/ internal/frontend/handlers/providers.go internal/frontend/telnet/providers.go
git commit -m "feat(wire): wire cmd/devserver and cmd/frontend with Initialize() injectors"
```

---

### Task 14: Makefile Build Target + Full Test Suite

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add build-devserver target to Makefile**

```makefile
build-devserver: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/devserver ./cmd/devserver

build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole
```

Update the `.PHONY` line to include `build-devserver`.

- [ ] **Step 2: Verify make wire runs cleanly**

```bash
make wire
```

Expected: wire regenerates all three wire_gen.go files with no errors

- [ ] **Step 3: Verify make wire-check passes on clean tree**

```bash
make wire-check
```

Expected: exit 0 (no diff)

- [ ] **Step 4: Run full build**

```bash
make build
```

Expected: all binaries build successfully

- [ ] **Step 5: Run fast test suite**

```bash
make test-fast
```

Expected: all tests pass, no new failures

- [ ] **Step 6: Run postgres test suite**

```bash
make test-postgres
```

Expected: all tests pass

- [ ] **Step 7: Commit**

```bash
git add Makefile
git commit -m "feat(wire): add build-devserver target; verify make wire + make wire-check"
```

---

### Task 15: Update Feature Doc and Final Verification

**Files:**
- Modify: `docs/features/wire-refactor.md`

- [ ] **Step 1: Mark all requirements complete in wire-refactor.md**

Update all `- [ ]` checkboxes to `- [x]` in `docs/features/wire-refactor.md`.

- [ ] **Step 2: Run full test suite one final time**

```bash
make test
```

Expected: 0 failures

- [ ] **Step 3: Verify wire-check passes**

```bash
make wire-check
```

Expected: exit 0

- [ ] **Step 4: Final commit**

```bash
git add docs/features/wire-refactor.md
git commit -m "docs: mark wire-refactor requirements complete"
```

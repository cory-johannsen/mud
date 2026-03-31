//go:build wireinject

// Package main provides the game server binary that runs the game backend
// with a gRPC service for frontend connections.
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
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// App holds all top-level components for the gameserver binary.
type App struct {
	GRPCService   *gameserver.GameServiceServer
	CombatHandler *gameserver.CombatHandler
	RegenMgr      *gameserver.RegenManager
	ZoneTickMgr   *gameserver.ZoneTickManager
	AIRegistry    *ai.Registry
	GameCalendar  *gameserver.GameCalendar
	Pool          *postgres.Pool
	ScriptMgr     *scripting.Manager
	WorldMgr      *world.Manager
	NpcMgr        *npc.Manager
	CombatEngine  *combat.Engine
	SessMgr       *session.Manager
	AutomapRepo   *postgres.AutomapRepository
	InvRegistry   *inventory.Registry
	RespawnMgr    *npc.RespawnManager
	RoomEquipMgr  *inventory.RoomEquipmentManager
	CharRepo      *postgres.CharacterRepository
	ProgressRepo  *postgres.CharacterProgressRepository
	WeatherMgr    *gameserver.WeatherManager
}

// Initialize is the wire-generated injector for the gameserver binary.
func Initialize(ctx context.Context, cfg *AppConfig, clock *gameserver.GameClock, logger *zap.Logger) (*App, error) {
	wire.Build(
		AppConfigToDatabase,
		// Extract typed path values from AppConfig.
		wire.FieldsOf(new(*AppConfig),
			"ZonesDir", "NPCsDir", "ConditionsDir", "MentalCondDir",
			"ScriptRoot", "CondScriptDir", "WeaponsDir", "ItemsDir",
			"ExplosivesDir", "ArmorsDir", "PreciousMaterialsDir", "AIScriptDir",
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
		// Interface bindings (cannot live in postgres package due to import cycles).
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
		wire.Bind(new(gameserver.CharacterDowntimeRepository), new(*postgres.CharacterDowntimeRepository)),
		wire.Struct(new(gameserver.StorageDeps), "*"),
		wire.Struct(new(gameserver.ContentDeps), "*"),
		wire.Struct(new(gameserver.HandlerDeps), "*"),
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}

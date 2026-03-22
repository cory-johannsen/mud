//go:build wireinject

// Package main provides the all-in-one development server for the MUD.
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
	Config         config.Config
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

// AppConfigToGameServerAddr extracts the game server address string for wire.
func AppConfigToGameServerAddr(cfg *AppConfig) string {
	return cfg.Config.GameServer.Addr()
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
		handlers.Providers,
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

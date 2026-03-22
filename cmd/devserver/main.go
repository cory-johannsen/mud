// Package main provides the all-in-one development server for the MUD.
// It wires together configuration, database, Telnet acceptor, and handlers.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/server"
)

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

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	// Initialize logger.
	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting Gunchete MUD server",
		zap.String("mode", cfg.Server.Mode),
		zap.String("type", cfg.Server.Type),
	)

	ctx := context.Background()

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

	app, err := Initialize(ctx, appCfg, logger)
	if err != nil {
		logger.Fatal("initializing application", zap.Error(err))
	}

	// Wire lifecycle.
	lifecycle := server.NewLifecycle(logger)

	lifecycle.Add("postgres", &server.FuncService{
		StartFn: func() error {
			// Pool is already connected; just keep it alive.
			for {
				time.Sleep(30 * time.Second)
				if err := app.Pool.Health(ctx, 5*time.Second); err != nil {
					logger.Warn("database health check failed", zap.Error(err))
				}
			}
		},
		StopFn: func() {
			app.Pool.Close()
		},
	})

	lifecycle.Add("telnet", &server.FuncService{
		StartFn: func() error {
			return app.TelnetAcceptor.ListenAndServe()
		},
		StopFn: func() {
			app.TelnetAcceptor.Stop()
		},
	})

	logger.Info("server initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("telnet_addr", fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)),
	)

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

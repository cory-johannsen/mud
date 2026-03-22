// Package main provides the Telnet frontend server for the MUD.
// It handles client connections, authentication, and bridges to the game server via gRPC.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
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

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting Gunchete frontend",
		zap.String("telnet_addr", cfg.Telnet.Addr()),
		zap.String("gameserver_addr", cfg.GameServer.Addr()),
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

	if cfg.Telnet.HeadlessPort != 0 {
		headlessCfg := cfg.Telnet
		headlessCfg.Port = cfg.Telnet.HeadlessPort
		headlessAcceptor := telnet.NewHeadlessAcceptor(headlessCfg, app.TelnetAcceptor.Handler(), logger)
		lifecycle.Add("telnet-headless", &server.FuncService{
			StartFn: func() error {
				return headlessAcceptor.ListenAndServe()
			},
			StopFn: func() {
				headlessAcceptor.Stop()
			},
		})
		logger.Info("headless telnet acceptor configured",
			zap.Int("port", cfg.Telnet.HeadlessPort),
		)
	}

	logger.Info("frontend initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("telnet_addr", fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)),
		zap.String("gameserver_addr", cfg.GameServer.Addr()),
	)

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

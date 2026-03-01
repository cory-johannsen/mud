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
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/server"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	regionsDir := flag.String("regions", "content/regions", "path to region YAML files directory")
	teamsDir := flag.String("teams", "content/teams", "path to team YAML files directory")
	jobsDir := flag.String("jobs", "content/jobs", "path to job YAML files directory")
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

	// Connect to PostgreSQL
	ctx := context.Background()
	dbStart := time.Now()
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("connecting to database", zap.Error(err))
	}
	logger.Info("database connected",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
		zap.String("database", cfg.Database.Name),
		zap.Duration("elapsed", time.Since(dbStart)),
	)

	// Load ruleset data
	regions, err := ruleset.LoadRegions(*regionsDir)
	if err != nil {
		logger.Fatal("loading regions", zap.Error(err))
	}
	teams, err := ruleset.LoadTeams(*teamsDir)
	if err != nil {
		logger.Fatal("loading teams", zap.Error(err))
	}
	jobs, err := ruleset.LoadJobs(*jobsDir)
	if err != nil {
		logger.Fatal("loading jobs", zap.Error(err))
	}
	logger.Info("ruleset loaded",
		zap.Int("regions", len(regions)),
		zap.Int("teams", len(teams)),
		zap.Int("jobs", len(jobs)),
	)

	// Build services
	accounts := postgres.NewAccountRepository(pool.DB())
	characters := postgres.NewCharacterRepository(pool.DB())
	authHandler := handlers.NewAuthHandler(accounts, characters, regions, teams, jobs, logger, cfg.GameServer.Addr(), cfg.Telnet)
	telnetAcceptor := telnet.NewAcceptor(cfg.Telnet, authHandler, logger)

	// Wire lifecycle
	lifecycle := server.NewLifecycle(logger)

	lifecycle.Add("postgres", &server.FuncService{
		StartFn: func() error {
			for {
				time.Sleep(30 * time.Second)
				if err := pool.Health(ctx, 5*time.Second); err != nil {
					logger.Warn("database health check failed", zap.Error(err))
				}
			}
		},
		StopFn: func() {
			pool.Close()
		},
	})

	lifecycle.Add("telnet", &server.FuncService{
		StartFn: func() error {
			return telnetAcceptor.ListenAndServe()
		},
		StopFn: func() {
			telnetAcceptor.Stop()
		},
	})

	logger.Info("frontend initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("telnet_addr", fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)),
		zap.String("gameserver_addr", cfg.GameServer.Addr()),
	)

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

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
	classesDir := flag.String("classes", "content/classes", "path to class YAML files directory")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	// Initialize logger
	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting Gunchete MUD server",
		zap.String("mode", cfg.Server.Mode),
		zap.String("type", cfg.Server.Type),
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
	classes, err := ruleset.LoadClasses(*classesDir)
	if err != nil {
		logger.Fatal("loading classes", zap.Error(err))
	}
	logger.Info("ruleset loaded",
		zap.Int("regions", len(regions)),
		zap.Int("classes", len(classes)),
	)

	// Build services
	accounts := postgres.NewAccountRepository(pool.DB())
	characters := postgres.NewCharacterRepository(pool.DB())
	authHandler := handlers.NewAuthHandler(accounts, characters, regions, classes, logger, cfg.GameServer.Addr())
	telnetAcceptor := telnet.NewAcceptor(cfg.Telnet, authHandler, logger)

	// Wire lifecycle
	lifecycle := server.NewLifecycle(logger)

	lifecycle.Add("postgres", &server.FuncService{
		StartFn: func() error {
			// Pool is already connected; just keep it alive
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

	logger.Info("server initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("telnet_addr", fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)),
	)

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

// Package main provides the game server binary that runs the game backend
// with a gRPC service for frontend connections.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/server"
)

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	zonesDir := flag.String("zones", "content/zones", "path to zone YAML files directory")
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

	logger.Info("starting game server",
		zap.String("grpc_addr", cfg.GameServer.Addr()),
	)

	// Load world
	zoneStart := time.Now()
	zones, err := world.LoadZonesFromDir(*zonesDir)
	if err != nil {
		logger.Fatal("loading zones", zap.Error(err))
	}
	worldMgr, err := world.NewManager(zones)
	if err != nil {
		logger.Fatal("creating world manager", zap.Error(err))
	}
	logger.Info("world loaded",
		zap.Int("zones", worldMgr.ZoneCount()),
		zap.Int("rooms", worldMgr.RoomCount()),
		zap.Duration("elapsed", time.Since(zoneStart)),
	)

	// Create managers
	sessMgr := session.NewManager()
	cmdRegistry := command.DefaultRegistry()

	// Create handlers
	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr)
	chatHandler := gameserver.NewChatHandler(sessMgr)

	// Create gRPC service
	grpcService := gameserver.NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
	)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, grpcService)

	// Wire lifecycle
	lifecycle := server.NewLifecycle(logger)

	lifecycle.Add("grpc", &server.FuncService{
		StartFn: func() error {
			lis, err := net.Listen("tcp", cfg.GameServer.Addr())
			if err != nil {
				return fmt.Errorf("listening on %s: %w", cfg.GameServer.Addr(), err)
			}
			logger.Info("gRPC server listening",
				zap.String("addr", lis.Addr().String()),
			)
			return grpcServer.Serve(lis)
		},
		StopFn: func() {
			grpcServer.GracefulStop()
		},
	})

	logger.Info("game server initialized",
		zap.Duration("startup", time.Since(start)),
		zap.String("grpc_addr", cfg.GameServer.Addr()),
	)

	if err := lifecycle.Run(context.Background()); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

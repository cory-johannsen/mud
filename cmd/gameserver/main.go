// Package main provides the game server binary that runs the game backend
// with a gRPC service for frontend connections.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/server"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func main() {
	start := time.Now()

	configPath    := flag.String("config", "configs/dev.yaml", "path to configuration file")
	zonesDir      := flag.String("zones", "content/zones", "path to zone YAML files directory")
	npcsDir       := flag.String("npcs-dir", "content/npcs", "path to NPC YAML templates directory")
	conditionsDir := flag.String("conditions-dir", "content/conditions", "path to condition YAML definitions directory")
	scriptRoot    := flag.String("script-root", "content/scripts", "root directory for Lua scripts; empty = scripting disabled")
	condScriptDir := flag.String("condition-scripts", "content/scripts/conditions", "directory of global condition scripts loaded into __global__ VM")
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

	cryptoSrc := dice.NewCryptoSource()
	diceRoller := dice.NewLoggedRoller(cryptoSrc, logger)

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

	// Connect to PostgreSQL for character persistence
	dbStart := time.Now()
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("connecting to database", zap.Error(err))
	}
	logger.Info("database connected",
		zap.String("host", cfg.Database.Host),
		zap.Duration("elapsed", time.Since(dbStart)),
	)
	charRepo := postgres.NewCharacterRepository(pool.DB())

	// Create managers
	sessMgr := session.NewManager()
	cmdRegistry := command.DefaultRegistry()

	// Load NPC templates and spawn initial instances
	npcTemplates, err := npc.LoadTemplates(*npcsDir)
	if err != nil {
		logger.Fatal("loading npc templates", zap.Error(err))
	}
	logger.Info("loaded npc templates", zap.Int("count", len(npcTemplates)))

	npcMgr := npc.NewManager()

	startRoom := worldMgr.StartRoom()
	if startRoom != nil {
		for _, tmpl := range npcTemplates {
			if _, err := npcMgr.Spawn(tmpl, startRoom.ID); err != nil {
				logger.Fatal("spawning npc", zap.String("template", tmpl.ID), zap.Error(err))
			}
			logger.Info("spawned npc", zap.String("template", tmpl.ID), zap.String("room", startRoom.ID))
		}
	}

	// Load condition definitions
	condStart := time.Now()
	condRegistry, err := condition.LoadDirectory(*conditionsDir)
	if err != nil {
		logger.Fatal("loading condition definitions", zap.Error(err))
	}
	logger.Info("loaded condition definitions",
		zap.Int("count", len(condRegistry.All())),
		zap.Duration("elapsed", time.Since(condStart)),
	)

	// Initialise scripting engine
	var scriptMgr *scripting.Manager
	if *scriptRoot != "" {
		scriptStart := time.Now()
		scriptMgr = scripting.NewManager(diceRoller, logger)

		// Load global condition scripts.
		if info, err := os.Stat(*condScriptDir); err == nil && info.IsDir() {
			if err := scriptMgr.LoadGlobal(*condScriptDir, 0); err != nil {
				logger.Fatal("loading global condition scripts",
					zap.String("dir", *condScriptDir), zap.Error(err))
			}
			logger.Info("global condition scripts loaded",
				zap.String("dir", *condScriptDir),
				zap.Duration("elapsed", time.Since(scriptStart)))
		}

		// Load per-zone scripts.
		for _, zone := range worldMgr.AllZones() {
			if zone.ScriptDir == "" {
				continue
			}
			info, err := os.Stat(zone.ScriptDir)
			if err != nil || !info.IsDir() {
				logger.Warn("zone script_dir not found, skipping",
					zap.String("zone", zone.ID), zap.String("dir", zone.ScriptDir))
				continue
			}
			if err := scriptMgr.LoadZone(zone.ID, zone.ScriptDir, zone.ScriptInstructionLimit); err != nil {
				logger.Fatal("loading zone scripts",
					zap.String("zone", zone.ID), zap.Error(err))
			}
			logger.Info("zone scripts loaded",
				zap.String("zone", zone.ID), zap.String("dir", zone.ScriptDir))
		}
		logger.Info("scripting engine initialized",
			zap.Duration("elapsed", time.Since(scriptStart)))

		// Wire QueryRoom callback.
		scriptMgr.QueryRoom = func(roomID string) *scripting.RoomInfo {
			room, ok := worldMgr.GetRoom(roomID)
			if !ok {
				return nil
			}
			return &scripting.RoomInfo{ID: room.ID, Title: room.Title}
		}
	}

	// Create handlers
	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr, npcMgr)
	chatHandler := gameserver.NewChatHandler(sessMgr)
	npcHandler := gameserver.NewNPCHandler(npcMgr, sessMgr)
	combatEngine := combat.NewEngine()

	roundDuration := time.Duration(cfg.GameServer.RoundDurationMs) * time.Millisecond
	if roundDuration <= 0 {
		roundDuration = 6 * time.Second
	}

	var grpcService *gameserver.GameServiceServer
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		if grpcService != nil {
			grpcService.BroadcastCombatEvents(roomID, events)
		}
	}
	combatHandler := gameserver.NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller, broadcastFn, roundDuration, condRegistry)

	// Create gRPC service
	grpcService = gameserver.NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger, charRepo, diceRoller, npcHandler, combatHandler, scriptMgr,
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

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

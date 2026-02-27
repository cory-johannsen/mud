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
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
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

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	zonesDir := flag.String("zones", "content/zones", "path to zone YAML files directory")
	npcsDir := flag.String("npcs-dir", "content/npcs", "path to NPC YAML templates directory")
	conditionsDir := flag.String("conditions-dir", "content/conditions", "path to condition YAML definitions directory")
	scriptRoot := flag.String("script-root", "content/scripts", "root directory for Lua scripts; empty = scripting disabled")
	condScriptDir := flag.String("condition-scripts", "content/scripts/conditions", "directory of global condition scripts loaded into __global__ VM")
	weaponsDir := flag.String("weapons-dir", "content/weapons", "path to weapon YAML definitions directory")
	explosivesDir := flag.String("explosives-dir", "content/explosives", "path to explosive YAML definitions directory")
	aiDir := flag.String("ai-dir", "content/ai", "path to HTN AI domain YAML directory")
	aiScriptDir := flag.String("ai-scripts", "content/scripts/ai", "path to Lua AI precondition scripts")
	aiTickInterval := flag.Duration("ai-tick", 10*time.Second, "NPC AI tick interval")
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

	// Build per-room spawn configs from zone data.
	roomSpawns := make(map[string][]npc.RoomSpawn)
	templateByID := make(map[string]*npc.Template, len(npcTemplates))
	for _, tmpl := range npcTemplates {
		templateByID[tmpl.ID] = tmpl
	}
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			for _, sc := range room.Spawns {
				tmpl, ok := templateByID[sc.Template]
				if !ok {
					logger.Fatal("spawn references unknown npc template",
						zap.String("zone", zone.ID),
						zap.String("room", room.ID),
						zap.String("template", sc.Template),
					)
				}
				var delay time.Duration
				switch {
				case sc.RespawnAfter != "":
					d, err := time.ParseDuration(sc.RespawnAfter)
					if err != nil {
						logger.Fatal("invalid respawn_after duration",
							zap.String("room", room.ID),
							zap.String("template", sc.Template),
							zap.String("value", sc.RespawnAfter),
							zap.Error(err),
						)
					}
					delay = d
				case tmpl.RespawnDelay != "":
					d, err := time.ParseDuration(tmpl.RespawnDelay)
					if err != nil {
						logger.Fatal("invalid respawn_delay on template",
							zap.String("template", tmpl.ID),
							zap.String("value", tmpl.RespawnDelay),
							zap.Error(err),
						)
					}
					delay = d
				}
				roomSpawns[room.ID] = append(roomSpawns[room.ID], npc.RoomSpawn{
					TemplateID:   sc.Template,
					Max:          sc.Count,
					RespawnDelay: delay,
				})
			}
		}
	}
	respawnMgr := npc.NewRespawnManager(roomSpawns, templateByID)
	logger.Info("built respawn manager", zap.Int("room_configs", len(roomSpawns)))

	// Populate all rooms with configured NPC spawns.
	for roomID := range roomSpawns {
		respawnMgr.PopulateRoom(roomID, npcMgr)
	}
	logger.Info("initial NPC population complete")

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

	// Load inventory definitions.
	invRegistry := inventory.NewRegistry()
	if *weaponsDir != "" {
		weapons, err := inventory.LoadWeapons(*weaponsDir)
		if err != nil {
			logger.Fatal("loading weapon definitions", zap.Error(err))
		}
		for _, w := range weapons {
			if err := invRegistry.RegisterWeapon(w); err != nil {
				logger.Fatal("registering weapon", zap.String("id", w.ID), zap.Error(err))
			}
		}
		logger.Info("loaded weapon definitions", zap.Int("count", len(weapons)))
	}
	if *explosivesDir != "" {
		explosives, err := inventory.LoadExplosives(*explosivesDir)
		if err != nil {
			logger.Fatal("loading explosive definitions", zap.Error(err))
		}
		for _, ex := range explosives {
			if err := invRegistry.RegisterExplosive(ex); err != nil {
				logger.Fatal("registering explosive", zap.String("id", ex.ID), zap.Error(err))
			}
		}
		logger.Info("loaded explosive definitions", zap.Int("count", len(explosives)))
	}

	// Load HTN AI domains.
	aiRegistry := ai.NewRegistry()

	combatEngine := combat.NewEngine()

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

		// Load weapon scripts.
		weaponScriptDir := filepath.Join(*scriptRoot, "weapons")
		if _, statErr := os.Stat(weaponScriptDir); statErr == nil {
			if err := scriptMgr.LoadGlobal(weaponScriptDir, scripting.DefaultInstructionLimit); err != nil {
				logger.Fatal("loading weapon scripts", zap.Error(err))
			}
			logger.Info("loaded weapon scripts", zap.String("dir", weaponScriptDir))
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

		// Wire GetEntityRoom callback.
		// Checks NPC instances first, then player sessions.
		//
		// Precondition: npcMgr and sessMgr must not be nil.
		// Postcondition: Returns room ID string or empty string when entity not found.
		scriptMgr.GetEntityRoom = func(uid string) string {
			if inst, ok := npcMgr.Get(uid); ok {
				return inst.RoomID
			}
			if sess, ok := sessMgr.GetPlayer(uid); ok {
				return sess.RoomID
			}
			return ""
		}

		// Wire GetCombatantsInRoom callback.
		// Returns CombatantInfo for all living combatants in the active combat for roomID.
		//
		// Precondition: combatEngine must not be nil.
		// Postcondition: Returns nil when no active combat exists for roomID.
		scriptMgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
			cbt, ok := combatEngine.GetCombat(roomID)
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
	}

	// Load AI precondition scripts before registering domains so that Lua
	// functions are available when the planner evaluates preconditions.
	if scriptMgr != nil && *aiScriptDir != "" {
		if _, statErr := os.Stat(*aiScriptDir); statErr == nil {
			if err := scriptMgr.LoadGlobal(*aiScriptDir, scripting.DefaultInstructionLimit); err != nil {
				logger.Fatal("loading AI scripts", zap.Error(err))
			}
			logger.Info("loaded AI scripts", zap.String("dir", *aiScriptDir))
		}
	}

	// Load HTN AI domain YAML files. Lua precondition scripts must be loaded first.
	if scriptMgr != nil && *aiDir != "" {
		domains, err := ai.LoadDomains(*aiDir)
		if err != nil {
			logger.Fatal("loading AI domains", zap.Error(err))
		}
		for _, domain := range domains {
			if err := aiRegistry.Register(domain, scriptMgr, ""); err != nil {
				logger.Fatal("registering AI domain", zap.String("id", domain.ID), zap.Error(err))
			}
		}
		logger.Info("loaded AI domains", zap.Int("count", len(domains)))
	}

	// Create handlers
	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr, npcMgr)
	chatHandler := gameserver.NewChatHandler(sessMgr)
	npcHandler := gameserver.NewNPCHandler(npcMgr, sessMgr)
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
	// Create floor manager for room item tracking.
	floorMgr := inventory.NewFloorManager()

	combatHandler := gameserver.NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller, broadcastFn, roundDuration, condRegistry, worldMgr, scriptMgr, invRegistry, aiRegistry, respawnMgr, floorMgr)

	// Create gRPC service
	grpcService = gameserver.NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger, charRepo, diceRoller, npcHandler, npcMgr, combatHandler, scriptMgr, respawnMgr, floorMgr, invRegistry,
	)

	// Start zone AI ticks.
	zm := gameserver.NewZoneTickManager(*aiTickInterval)
	grpcService.StartZoneTicks(ctx, zm, aiRegistry)

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

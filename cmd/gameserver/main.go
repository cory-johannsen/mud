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

	"errors"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
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
	if err := worldMgr.ValidateExits(); err != nil {
		logger.Fatal("validating cross-zone exits", zap.Error(err))
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
	accountRepo := postgres.NewAccountRepository(pool.DB())
	automapRepo := postgres.NewAutomapRepository(pool.DB())
	progressRepo := postgres.NewCharacterProgressRepository(pool.DB())

	// Create managers
	sessMgr := session.NewManager()
	// cmdRegistry is built after class features are loaded so shortcuts can be registered.

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
	mentalCondRegistry, err := condition.LoadDirectory("content/conditions/mental")
	if err != nil {
		logger.Fatal("loading mental condition definitions", zap.Error(err))
	}
	for _, def := range mentalCondRegistry.All() {
		condRegistry.Register(def)
	}
	logger.Info("loaded mental condition definitions", zap.Int("count", len(mentalCondRegistry.All())))

	// Load technology definitions.
	techReg, err := technology.Load(*techContentDir)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr.Err) {
			log.Printf("WARN: technology content dir %q not found — starting with empty tech registry", *techContentDir)
			techReg = technology.NewRegistry()
		} else {
			log.Fatalf("loading technology content: %v", err)
		}
	}
	logger.Info("loaded technology definitions", zap.Int("count", len(techReg.All())))

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

	if *itemsDir != "" {
		items, err := inventory.LoadItems(*itemsDir)
		if err != nil {
			logger.Fatal("loading item definitions", zap.Error(err))
		}
		for _, item := range items {
			if err := invRegistry.RegisterItem(item); err != nil {
				logger.Fatal("registering item", zap.String("id", item.ID), zap.Error(err))
			}
		}
		logger.Info("loaded item definitions", zap.Int("count", len(items)))
	}

	// Load armor definitions.
	armors, err := inventory.LoadArmors(*armorsDir)
	if err != nil {
		logger.Fatal("failed to load armors", zap.Error(err))
	}
	for _, a := range armors {
		if err := invRegistry.RegisterArmor(a); err != nil {
			logger.Fatal("failed to register armor", zap.String("id", a.ID), zap.Error(err))
		}
	}
	logger.Info("loaded armor definitions", zap.Int("count", len(armors)))

	// Wire armor AC resolver so NPC spawn applies equipped armor AC bonus.
	npcMgr.SetArmorACResolver(func(armorID string) int {
		if def, ok := invRegistry.Armor(armorID); ok {
			return def.ACBonus
		}
		return 0
	})

	// Load job definitions.
	jobList, err := ruleset.LoadJobs(*jobsDir)
	if err != nil {
		logger.Fatal("failed to load jobs", zap.Error(err))
	}
	jobReg := ruleset.NewJobRegistry()
	for _, j := range jobList {
		jobReg.Register(j)
	}
	logger.Info("loaded job definitions", zap.Int("count", len(jobList)))

	allSkills, err := ruleset.LoadSkills(*skillsFile)
	if err != nil {
		logger.Fatal("failed to load skills", zap.Error(err))
	}
	logger.Info("loaded skill definitions", zap.Int("count", len(allSkills)))
	characterSkillsRepo := postgres.NewCharacterSkillsRepository(pool.DB())
	characterProficienciesRepo := postgres.NewCharacterProficienciesRepository(pool.DB())

	allFeats, err := ruleset.LoadFeats(*featsFile)
	if err != nil {
		logger.Fatal("failed to load feats", zap.Error(err))
	}
	logger.Info("loaded feat definitions", zap.Int("count", len(allFeats)))
	featRegistry := ruleset.NewFeatRegistry(allFeats)
	characterFeatsRepo := postgres.NewCharacterFeatsRepository(pool.DB())

	classFeatures, err := ruleset.LoadClassFeatures(*classFeatsFile)
	if err != nil {
		logger.Fatal("loading class features", zap.Error(err))
	}
	cfReg := ruleset.NewClassFeatureRegistry(classFeatures)
	logger.Info("class features loaded", zap.Int("class_features", len(classFeatures)))
	allCmds := command.RegisterShortcuts(classFeatures, command.BuiltinCommands())
	cmdRegistry, err := command.NewRegistry(allCmds)
	if err != nil {
		logger.Fatal("building command registry with shortcuts", zap.Error(err))
	}
	logger.Info("command registry built", zap.Int("commands", len(allCmds)))
	characterClassFeaturesRepo := postgres.NewCharacterClassFeaturesRepository(pool.DB())
	featureChoicesRepo := postgres.NewCharacterFeatureChoicesRepo(pool.DB())
	charAbilityBoostsRepo := postgres.NewCharacterAbilityBoostsRepository(pool.DB())

	archetypeList, err := ruleset.LoadArchetypes(*archetypesDir)
	if err != nil {
		logger.Fatal("failed to load archetypes", zap.Error(err))
	}
	archetypeMap := make(map[string]*ruleset.Archetype, len(archetypeList))
	for _, a := range archetypeList {
		archetypeMap[a.ID] = a
	}
	logger.Info("loaded archetype definitions", zap.Int("count", len(archetypeList)))

	regionList, err := ruleset.LoadRegions(*regionsDir)
	if err != nil {
		logger.Fatal("failed to load regions", zap.Error(err))
	}
	regionMap := make(map[string]*ruleset.Region, len(regionList))
	for _, r := range regionList {
		regionMap[r.ID] = r
	}
	logger.Info("loaded region definitions", zap.Int("count", len(regionList)))

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

		// Wire RevealZoneMap callback.
		// Bulk-reveals all rooms in zoneID for the player's automap cache and persists to DB.
		//
		// Precondition: sessMgr, worldMgr, and automapRepo must not be nil.
		// Postcondition: All rooms in zoneID are marked discovered in the session cache and persisted.
		scriptMgr.RevealZoneMap = func(uid, zoneID string) {
			sess, ok := sessMgr.GetPlayer(uid)
			if !ok {
				return
			}
			zone, ok := worldMgr.GetZone(zoneID)
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
			if automapRepo != nil && len(newRooms) > 0 {
				if err := automapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, newRooms); err != nil {
					logger.Warn("bulk-persisting zone map reveal", zap.Error(err))
				}
			}
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

	// Create and start game clock.
	gameClock := gameserver.NewGameClock(
		int32(cfg.GameServer.GameClockStart),
		cfg.GameServer.GameTickDuration,
	)
	stopClock := gameClock.Start()
	defer stopClock()

	// Create handlers
	// worldHandler is initialized after roomEquipMgr below.
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

	// Create room equipment manager and seed it from zone data.
	roomEquipMgr := inventory.NewRoomEquipmentManager()
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			if len(room.Equipment) > 0 {
				roomEquipMgr.InitRoom(room.ID, room.Equipment)
			}
		}
	}
	logger.Info("room equipment manager initialized")

	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr, npcMgr, gameClock, roomEquipMgr, invRegistry)

	mentalMgr := mentalstate.NewManager()
	combatHandler := gameserver.NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller, broadcastFn, roundDuration, condRegistry, worldMgr, scriptMgr, invRegistry, aiRegistry, respawnMgr, floorMgr, mentalMgr)
	combatHandler.SetLogger(logger)

	// Create action handler for player-activated class feature actions.
	actionH := gameserver.NewActionHandler(sessMgr, cfReg, condRegistry, npcMgr, combatHandler, charRepo, diceRoller, logger)

	// Create gRPC service
	grpcService = gameserver.NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger, charRepo, diceRoller, npcHandler, npcMgr, combatHandler, scriptMgr, respawnMgr, floorMgr, roomEquipMgr, automapRepo, invRegistry, gameserver.NewAccountRepoAdapter(accountRepo), gameClock,
		jobReg, condRegistry, techReg, *loadoutsDir,
		allSkills, characterSkillsRepo, characterProficienciesRepo,
		allFeats, featRegistry, characterFeatsRepo,
		classFeatures, cfReg, characterClassFeaturesRepo,
		featureChoicesRepo,
		charAbilityBoostsRepo,
		archetypeMap,
		regionMap,
		mentalMgr,
		actionH,
	)

	// Wire XP service with progress and skill-increase persistence.
	if xpCfg, xpErr := xp.LoadXPConfig(*xpConfigFile); xpErr != nil {
		logger.Warn("loading xp config; XP awards disabled", zap.Error(xpErr))
	} else {
		xpSvc := xp.NewService(xpCfg, progressRepo)
		xpSvc.SetSkillIncreaseSaver(progressRepo)
		grpcService.SetProgressRepo(progressRepo)
		grpcService.SetXPService(xpSvc)
		combatHandler.SetXPService(xpSvc)
		combatHandler.SetCurrencySaver(charRepo)
	}

	// Start respawn goroutine for room equipment.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			roomEquipMgr.ProcessRespawns()
		}
	}()

	// Start out-of-combat health regeneration.
	regenMgr := gameserver.NewRegenManager(sessMgr, npcMgr, combatHandler, charRepo, gameserver.RegenInterval, logger)
	regenMgr.Start(ctx)

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

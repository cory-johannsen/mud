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
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/server"
)

// AppConfig holds all CLI flag values for the gameserver binary.
type AppConfig struct {
	Config          config.Config
	ZonesDir        world.WorldDir
	NPCsDir         npc.NPCsDir
	ConditionsDir   condition.ConditionsDir
	MentalCondDir   condition.MentalConditionsDir
	ScriptRoot      scripting.ScriptRoot
	CondScriptDir   scripting.CondScriptDir
	WeaponsDir      inventory.WeaponsDir
	ItemsDir        inventory.ItemsDir
	ExplosivesDir   inventory.ExplosivesDir
	ArmorsDir              inventory.ArmorsDir
	PreciousMaterialsDir   inventory.PreciousMaterialsDir
	AIDir           string
	AIScriptDir     scripting.AIScriptDir
	AITickInterval  gameserver.AITickInterval
	JobsDir         ruleset.JobsDir
	LoadoutsDir     gameserver.LoadoutsDir
	SkillsFile      ruleset.SkillsFile
	FeatsFile       ruleset.FeatsFile
	ClassFeatsFile  ruleset.ClassFeaturesFile
	ArchetypesDir   ruleset.ArchetypesDir
	RegionsDir      ruleset.RegionsDir
	TechContentDir  technology.TechContentDir
	RoundDurationMs gameserver.RoundDurationMs
	XPConfigFile      string
	SetsDir           string
	SubstancesDir     string
	FactionsDir       string
	FactionConfigPath string
	MaterialsFile           string
	RecipesDir              string
	DowntimeQueueLimitsFile string
	WeatherChancePerTick    float64
	WeatherFile             string
	QuestsDir               string
}

// AppConfigToDatabase extracts database config from AppConfig for wire.
func AppConfigToDatabase(cfg *AppConfig) config.DatabaseConfig {
	return cfg.Config.Database
}

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
	preciousMaterialsDir := flag.String("precious-materials-dir", "content/items/precious_materials", "path to precious material YAML definitions directory")
	jobsDir := flag.String("jobs-dir", "content/jobs", "path to job YAML definitions directory")
	loadoutsDir := flag.String("loadouts-dir", "content/loadouts", "path to archetype loadout YAML directory")
	skillsFile := flag.String("skills", "content/skills.yaml", "path to skills YAML file")
	featsFile := flag.String("feats", "content/feats.yaml", "path to feats YAML file")
	classFeatsFile := flag.String("class-features", "content/class_features.yaml", "path to class features YAML file")
	archetypesDir := flag.String("archetypes-dir", "content/archetypes", "path to archetype YAML definitions directory")
	regionsDir := flag.String("regions-dir", "content/regions", "path to region YAML definitions directory")
	xpConfigFile := flag.String("xp-config", "content/xp_config.yaml", "path to XP configuration YAML file")
	techContentDir := flag.String("tech-content-dir", "content/technologies", "path to technology YAML content directory")
	contentDir := flag.String("content-dir", "content", "path to content directory for world editing")
	setsDir := flag.String("sets-dir", "content/sets", "path to equipment set YAML definitions directory")
	substancesDir := flag.String("substances-dir", "content/substances", "path to substance YAML definitions directory")
	factionsDir := flag.String("factions-dir", "content/factions", "path to faction YAML definitions directory")
	factionConfigPath := flag.String("faction-config", "content/faction_config.yaml", "path to faction configuration YAML file")
	materialsFile := flag.String("materials-file", "content/materials.yaml", "path to crafting materials YAML file")
	recipesDir := flag.String("recipes-dir", "content/recipes", "path to crafting recipe YAML definitions directory")
	downtimeQueueLimitsFile := flag.String("downtime-queue-limits", "content/downtime_queue_limits.yaml", "path to downtime queue limits YAML file")
	questsDir := flag.String("quests-dir", "content/quests", "path to quest YAML files directory")
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

	logger.Info("starting game server",
		zap.String("grpc_addr", cfg.GameServer.Addr()),
	)

	// Pre-construct GameClock; its primitive params would collide with wire's string provider.
	gameClock := gameserver.NewGameClock(
		int32(cfg.GameServer.GameClockStart),
		cfg.GameServer.GameTickDuration,
	)

	appCfg := &AppConfig{
		Config:          cfg,
		ZonesDir:        world.WorldDir(*zonesDir),
		NPCsDir:         npc.NPCsDir(*npcsDir),
		ConditionsDir:   condition.ConditionsDir(*conditionsDir),
		MentalCondDir:   condition.MentalConditionsDir(*conditionsDir + "/mental"),
		ScriptRoot:      scripting.ScriptRoot(*scriptRoot),
		CondScriptDir:   scripting.CondScriptDir(*condScriptDir),
		WeaponsDir:      inventory.WeaponsDir(*weaponsDir),
		ItemsDir:        inventory.ItemsDir(*itemsDir),
		ExplosivesDir:   inventory.ExplosivesDir(*explosivesDir),
		ArmorsDir:            inventory.ArmorsDir(*armorsDir),
		PreciousMaterialsDir: inventory.PreciousMaterialsDir(*preciousMaterialsDir),
		AIDir:           *aiDir,
		AIScriptDir:     scripting.AIScriptDir(*aiScriptDir),
		AITickInterval:  gameserver.AITickInterval(*aiTickInterval),
		JobsDir:         ruleset.JobsDir(*jobsDir),
		LoadoutsDir:     gameserver.LoadoutsDir(*loadoutsDir),
		SkillsFile:      ruleset.SkillsFile(*skillsFile),
		FeatsFile:       ruleset.FeatsFile(*featsFile),
		ClassFeatsFile:  ruleset.ClassFeaturesFile(*classFeatsFile),
		ArchetypesDir:   ruleset.ArchetypesDir(*archetypesDir),
		RegionsDir:      ruleset.RegionsDir(*regionsDir),
		TechContentDir:  technology.TechContentDir(*techContentDir),
		RoundDurationMs: gameserver.RoundDurationMs(cfg.GameServer.RoundDurationMs),
		XPConfigFile:    *xpConfigFile,
		SetsDir:         *setsDir,
		SubstancesDir:     *substancesDir,
		FactionsDir:       *factionsDir,
		FactionConfigPath: *factionConfigPath,
		MaterialsFile:           *materialsFile,
		RecipesDir:              *recipesDir,
		DowntimeQueueLimitsFile: *downtimeQueueLimitsFile,
		WeatherChancePerTick:    cfg.Weather.ChancePerTick,
		WeatherFile:             cfg.Weather.ContentFile,
		QuestsDir:               *questsDir,
	}

	app, err := Initialize(ctx, appCfg, gameClock, logger)
	if err != nil {
		logger.Fatal("initializing application", zap.Error(err))
	}

	// Attempt to initialize WorldEditor for in-game world-editing commands.
	worldEditor, weErr := world.NewWorldEditor(*contentDir, app.GRPCService.World())
	if weErr != nil {
		logger.Warn("WARNING: content/ is not writable — world-editing commands disabled.", zap.Error(weErr))
	} else {
		app.GRPCService.SetWorldEditor(worldEditor)
	}

	// Start game clock.
	stopClock := gameClock.Start()
	defer stopClock()

	// Start game calendar.
	app.GameCalendar.SetLogger(logger.Sugar())
	stopCalendar := app.GameCalendar.Start()
	defer stopCalendar()

	// Validate NPC templates against the feat registry at startup (REQ-AE-14).
	if fr := app.GRPCService.FeatRegistry(); fr != nil {
		for _, tmpl := range app.NpcMgr.AllTemplates() {
			if err := tmpl.ValidateWithRegistry(fr); err != nil {
				logger.Fatal("NPC template feat validation failed", zap.Error(err))
			}
		}
	}

	// Wire feat registry into combat handler (REQ-AE-16).
	if fr := app.GRPCService.FeatRegistry(); fr != nil {
		app.CombatHandler.SetFeatRegistry(fr)
	}

	// Validate quest registry cross-references at startup.
	if err := app.GRPCService.ValidateQuestRegistry(); err != nil {
		log.Fatalf("quest registry validation failed: %v", err)
	}

	// Wire broadcast function: CombatHandler needs GRPCService after both are constructed.
	app.CombatHandler.SetBroadcastFn(func(roomID string, events []*gamev1.CombatEvent) {
		if app.GRPCService != nil {
			app.GRPCService.BroadcastCombatEvents(roomID, events)
		}
	})

	// Wire substance service: CombatHandler needs GRPCService to apply poison on hit (REQ-AH-21).
	app.CombatHandler.SetSubstanceSvc(app.GRPCService)

	// Wire scripting callbacks (post-Initialize to avoid circular deps).
	if app.ScriptMgr != nil {
		// Wire QueryRoom callback.
		app.ScriptMgr.QueryRoom = func(roomID string) *scripting.RoomInfo {
			room, ok := app.WorldMgr.GetRoom(roomID)
			if !ok {
				return nil
			}
			return &scripting.RoomInfo{ID: room.ID, Title: room.Title}
		}

		// Wire GetEntityRoom callback.
		// Checks NPC instances first, then player sessions.
		//
		// Precondition: app.NpcMgr and app.SessMgr must not be nil.
		// Postcondition: Returns room ID string or empty string when entity not found.
		app.ScriptMgr.GetEntityRoom = func(uid string) string {
			if inst, ok := app.NpcMgr.Get(uid); ok {
				return inst.RoomID
			}
			if sess, ok := app.SessMgr.GetPlayer(uid); ok {
				return sess.RoomID
			}
			return ""
		}

		// Wire GetCombatantsInRoom callback.
		// Returns CombatantInfo for all living combatants in the active combat for roomID.
		//
		// Precondition: app.CombatEngine must not be nil.
		// Postcondition: Returns nil when no active combat exists for roomID.
		app.ScriptMgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
			cbt, ok := app.CombatEngine.GetCombat(roomID)
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
		// Precondition: app.GRPCService, app.WorldMgr, and app.AutomapRepo must not be nil.
		// Postcondition: All rooms in zoneID are marked discovered in the session cache and persisted.
		app.ScriptMgr.RevealZoneMap = func(uid, zoneID string) {
			sess, ok := app.SessMgr.GetPlayer(uid)
			if !ok {
				return
			}
			zone, ok := app.WorldMgr.GetZone(zoneID)
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
			if app.AutomapRepo != nil && len(newRooms) > 0 {
				if err := app.AutomapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, newRooms, false); err != nil {
					logger.Warn("bulk-persisting zone map reveal", zap.Error(err))
				}
			}
		}

		// Load AI domains post-Initialize (scripting cannot import ai due to import cycle).
		if appCfg.AIDir != "" {
			domains, err := ai.LoadDomains(appCfg.AIDir)
			if err != nil {
				logger.Fatal("loading AI domains", zap.Error(err))
			}
			for _, domain := range domains {
				if err := app.AIRegistry.Register(domain, app.ScriptMgr, ""); err != nil {
					logger.Fatal("registering AI domain", zap.String("id", domain.ID), zap.Error(err))
				}
			}
			logger.Info("loaded AI domains", zap.Int("count", len(domains)))
		}
	}

	// Wire REQ-NPC-8: prevent a player from attacking their own bound hireling.
	app.CombatHandler.SetHirelingOwnerOf(app.GRPCService.HirelingOwnerOf)

	// Wire feat registry into NPC manager for tough feat HP bonus at spawn (REQ-AE-18).
	app.NpcMgr.SetFeatRegistry(app.GRPCService.FeatRegistry())

	// Wire XP service with progress and skill-increase persistence.
	if xpCfg, xpErr := xp.LoadXPConfig(appCfg.XPConfigFile); xpErr != nil {
		logger.Warn("loading xp config; XP awards disabled", zap.Error(xpErr))
	} else {
		// Validate tier multipliers at startup (REQ-AE-4).
		if tierErr := xpCfg.ValidateTiers(); tierErr != nil {
			logger.Warn("XP tier multipliers incomplete; defaulting to standard tier only", zap.Error(tierErr))
		}
		// Wire XP config into NPC manager for tier HP scaling at spawn (REQ-AE-5).
		app.NpcMgr.SetXPConfig(xpCfg)
		xpSvc := xp.NewService(xpCfg, app.ProgressRepo)
		xpSvc.SetSkillIncreaseSaver(app.ProgressRepo)
		app.GRPCService.SetProgressRepo(app.ProgressRepo)
		app.GRPCService.SetXPService(xpSvc)
		app.CombatHandler.SetXPService(xpSvc)
		app.CombatHandler.SetCurrencySaver(app.CharRepo)
	}

	// Start respawn goroutine for room equipment.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			app.RoomEquipMgr.ProcessRespawns()
		}
	}()

	// Start out-of-combat health regeneration.
	app.RegenMgr.Start(ctx)

	// Start zone AI ticks.
	app.GRPCService.StartZoneTicks(ctx, app.ZoneTickMgr, app.AIRegistry)

	// Start calendar-driven WantedLevel decay.
	app.GRPCService.StartWantedDecayHook()
	defer app.GRPCService.StopWantedDecayHook()

	// Start calendar-driven item charge recharge ticks.
	stopItemTicks := app.GRPCService.StartItemTickHook()
	defer stopItemTicks()

	// Create gRPC server.
	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, app.GRPCService)

	// Wire lifecycle.
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
				if err := app.Pool.Health(ctx, 5*time.Second); err != nil {
					logger.Warn("database health check failed", zap.Error(err))
				}
			}
		},
		StopFn: func() {
			app.Pool.Close()
		},
	})

	if err := lifecycle.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}

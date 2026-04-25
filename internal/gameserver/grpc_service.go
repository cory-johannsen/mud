package gameserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"

	"sort"
	"strconv"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/crafting"
	effectrender "github.com/cory-johannsen/mud/internal/game/effect/render"
	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/cory-johannsen/mud/internal/game/drawback"
	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/focuspoints"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/maputil"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"

	lua "github.com/yuin/gopher-lua"
)

// errQuit is returned by handleQuit to signal the command loop to stop cleanly.
var errQuit = fmt.Errorf("quit")

// requireEditor returns an error ServerEvent if the session lacks editor or admin role.
// Precondition: sess must be non-nil.
// Postcondition: Returns nil if role is editor or admin; returns error ServerEvent otherwise.
func requireEditor(sess *session.PlayerSession) *gamev1.ServerEvent {
	if sess.Role != postgres.RoleEditor && sess.Role != postgres.RoleAdmin {
		return errorEvent("permission denied: editor role required")
	}
	return nil
}

// requireAdmin returns an error ServerEvent if the session lacks admin role.
// Precondition: sess must be non-nil.
// Postcondition: Returns nil if role is admin; returns error ServerEvent otherwise.
func requireAdmin(sess *session.PlayerSession) *gamev1.ServerEvent {
	if sess.Role != postgres.RoleAdmin {
		return errorEvent("permission denied: admin role required")
	}
	return nil
}

// AccountAdmin provides account lookup and role mutation for in-game admin commands.
//
// Precondition: Implementations must be safe for concurrent use.
type AccountAdmin interface {
	GetAccountByUsername(ctx context.Context, username string) (AccountInfo, error)
	SetAccountRole(ctx context.Context, accountID int64, role string) error
}

// AccountInfo is a minimal view of an account used by AccountAdmin.
type AccountInfo struct {
	ID       int64
	Username string
	Role     string
}

// CharacterSaver persists and loads character state at session boundaries.
//
// Precondition: All id/characterID arguments must be > 0.
// Postcondition: Returns nil on success or a non-nil error on failure.
type CharacterSaver interface {
	// GetByID retrieves the character with the given id.
	// Postcondition: if err == nil, the returned *Character is non-nil.
	GetByID(ctx context.Context, id int64) (*character.Character, error)
	SaveState(ctx context.Context, id int64, location string, currentHP int) error
	LoadWeaponPresets(ctx context.Context, characterID int64, reg *inventory.Registry) (*inventory.LoadoutSet, error)
	SaveWeaponPresets(ctx context.Context, characterID int64, ls *inventory.LoadoutSet) error
	LoadEquipment(ctx context.Context, characterID int64) (*inventory.Equipment, error)
	SaveEquipment(ctx context.Context, characterID int64, eq *inventory.Equipment) error
	LoadInventory(ctx context.Context, characterID int64) ([]inventory.InventoryItem, error)
	SaveInventory(ctx context.Context, characterID int64, items []inventory.InventoryItem) error
	HasReceivedStartingInventory(ctx context.Context, characterID int64) (bool, error)
	MarkStartingInventoryGranted(ctx context.Context, characterID int64) error
	SaveAbilities(ctx context.Context, characterID int64, abilities character.AbilityScores) error
	SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error
	SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error
	SaveCurrency(ctx context.Context, characterID int64, currency int) error
	LoadCurrency(ctx context.Context, characterID int64) (int, error)
	SaveGender(ctx context.Context, characterID int64, gender string) error
	SaveHeroPoints(ctx context.Context, characterID int64, heroPoints int) error
	LoadHeroPoints(ctx context.Context, characterID int64) (int, error)
	SaveJobs(ctx context.Context, characterID int64, jobs map[string]int, activeJobID string) error
	LoadJobs(ctx context.Context, characterID int64) (jobs map[string]int, activeJobID string, err error)
	SaveInstanceCharges(ctx context.Context, characterID int64, instanceID, itemDefID string, charges int, expended bool) error
	LoadFocusPoints(ctx context.Context, characterID int64) (int, error)
	SaveFocusPoints(ctx context.Context, characterID int64, focusPoints int) error
	SaveHotbars(ctx context.Context, characterID int64, bars [][10]session.HotbarSlot, activeIdx int) error
	LoadHotbars(ctx context.Context, characterID int64) ([][10]session.HotbarSlot, int, error)
}

// CharacterSkillsGetter retrieves per-character skill proficiency data.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns a map of skill_id → proficiency (may be empty).
type CharacterSkillsGetter interface {
	GetAll(ctx context.Context, characterID int64) (map[string]string, error)
}

// CharacterSkillsRepository retrieves and persists per-character skill proficiency data.
//
// Precondition: characterID must be > 0.
// Postcondition: Mutations are durably persisted before returning.
type CharacterSkillsRepository interface {
	CharacterSkillsGetter
	HasSkills(ctx context.Context, characterID int64) (bool, error)
	SetAll(ctx context.Context, characterID int64, skills map[string]string) error
	UpgradeSkill(ctx context.Context, characterID int64, skillID, newRank string) error
}

// ProgressRepository persists and retrieves character level, XP, max HP, and pending counts.
//
// Precondition: characterID must be > 0.
// Postcondition: Mutations are durably persisted before returning.
type ProgressRepository interface {
	GetProgress(ctx context.Context, id int64) (level, experience, maxHP, pendingBoosts int, err error)
	GetPendingSkillIncreases(ctx context.Context, id int64) (int, error)
	IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error
	ConsumePendingBoost(ctx context.Context, id int64) error
	ConsumePendingSkillIncrease(ctx context.Context, id int64) error
	IsSkillIncreasesInitialized(ctx context.Context, id int64) (bool, error)
	MarkSkillIncreasesInitialized(ctx context.Context, id int64) error
	GetPendingTechLevels(ctx context.Context, id int64) ([]int, error)
	SetPendingTechLevels(ctx context.Context, id int64, levels []int) error
}

// CharacterProficienciesRepository persists per-character armor/weapon proficiency data.
//
// Precondition: characterID must be > 0.
// Postcondition: Mutations are durably persisted before returning.
type CharacterProficienciesRepository interface {
	GetAll(ctx context.Context, characterID int64) (map[string]string, error)
	Upsert(ctx context.Context, characterID int64, category, rank string) error
}

// CharacterFeatsGetter retrieves the feat IDs assigned to a character.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns a slice of feat IDs (may be empty).
type CharacterFeatsGetter interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
}

// CharacterFeatsRepo extends CharacterFeatsGetter with mutation needed for
// level-up feat grants.
//
// Precondition: characterID must be > 0; featID must be non-empty.
// Postcondition: Add inserts a feat row; duplicate adds are no-ops.
type CharacterFeatsRepo interface {
	CharacterFeatsGetter
	Add(ctx context.Context, characterID int64, featID string) error
}

// CharacterFeatLevelGrantsRepo tracks which character levels have had their
// feat level-up grants applied, preventing re-processing when pools overlap.
//
// Precondition: characterID > 0; level >= 2.
type CharacterFeatLevelGrantsRepo interface {
	IsLevelGranted(ctx context.Context, characterID int64, level int) (bool, error)
	MarkLevelGranted(ctx context.Context, characterID int64, level int) error
}

// CharacterClassFeaturesGetter retrieves the class feature IDs assigned to a character.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns a slice of feature IDs (may be empty).
type CharacterClassFeaturesGetter interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
}

// CharacterFeatureChoicesRepository persists and retrieves per-character feature choices.
//
// Precondition: characterID must be > 0.
// Postcondition: GetAll returns a non-nil map; Set durably persists the choice.
type CharacterFeatureChoicesRepository interface {
	GetAll(ctx context.Context, characterID int64) (map[string]map[string]string, error)
	Set(ctx context.Context, characterID int64, featureID, choiceKey, value string) error
}

// GameServiceServer implements the gRPC GameService with bidirectional streaming.
type GameServiceServer struct {
	gamev1.UnimplementedGameServiceServer
	world                      *world.Manager
	sessions                   *session.Manager
	commands                   *command.Registry
	worldH                     *WorldHandler
	chatH                      *ChatHandler
	charSaver                  CharacterSaver
	dice                       *dice.Roller
	npcH                       *NPCHandler
	npcMgr                     *npc.Manager
	combatH                    *CombatHandler
	scriptMgr                  *scripting.Manager
	respawnMgr                 *npc.RespawnManager
	floorMgr                   *inventory.FloorManager
	roomEquipMgr               *inventory.RoomEquipmentManager
	automapRepo                *postgres.AutomapRepository
	invRegistry                *inventory.Registry
	accountAdmin               AccountAdmin
	calendar                   *GameCalendar
	logger                     *zap.Logger
	jobRegistry                *ruleset.JobRegistry
	condRegistry               *condition.Registry
	techRegistry               *technology.Registry
	hardwiredTechRepo          HardwiredTechRepo
	preparedTechRepo           PreparedTechRepo
	knownTechRepo        KnownTechRepo
	innateTechRepo             InnateTechRepo
	spontaneousUsePoolRepo     SpontaneousUsePoolRepo
	loadoutsDir                string
	allSkills                  []*ruleset.Skill
	characterSkillsRepo        CharacterSkillsRepository
	characterProficienciesRepo CharacterProficienciesRepository
	allFeats                   []*ruleset.Feat
	featRegistry               *ruleset.FeatRegistry
	characterFeatsRepo         CharacterFeatsRepo
	featLevelGrantsRepo        CharacterFeatLevelGrantsRepo
	allClassFeatures           []*ruleset.ClassFeature
	classFeatureRegistry       *ruleset.ClassFeatureRegistry
	characterClassFeaturesRepo CharacterClassFeaturesGetter
	featureChoicesRepo         CharacterFeatureChoicesRepository
	charAbilityBoostsRepo      postgres.CharacterAbilityBoostsRepository
	archetypes                 map[string]*ruleset.Archetype
	regions                    map[string]*ruleset.Region
	xpSvc                      *xp.Service
	progressRepo               ProgressRepository
	mentalStateMgr             *mentalstate.Manager
	actionH                    *ActionHandler
	wantedRepo                 *postgres.WantedRepository
	stopWantedDecay            func()
	stopCarrierRad             func()
	rovingMgr                  *npc.RovingManager
	stopRovingNPCs             func()
	trapMgr                    *trap.TrapManager
	trapTemplates              map[string]*trap.TrapTemplate
	// merchantRuntimeStates maps NPC instance ID to active merchant runtime state.
	merchantRuntimeStates map[string]*npc.MerchantRuntimeState
	// bankerRuntimeStates maps NPC instance ID to active banker runtime state.
	bankerRuntimeStates map[string]*npc.BankerRuntimeState
	// healerRuntimeStates maps NPC instance ID to active healer runtime state.
	healerRuntimeStates map[string]*npc.HealerRuntimeState
	// hirelingRuntimeStates maps NPC instance ID to active hireling runtime state.
	hirelingRuntimeStates map[string]*npc.HirelingRuntimeState
	// healerCapacityRepo persists per-template healer daily capacity usage across restarts.
	// May be nil (capacity resets on restart if not set).
	healerCapacityRepo HealerCapacityRepo
	// detainedUntilRepo persists detention expiry timestamps per character.
	// May be nil (detention expiry is not persisted if not set).
	detainedUntilRepo DetainedUntilUpdater
	worldEditor       *world.WorldEditor
	// setRegistry holds equipment set definitions for computing set bonuses (REQ-EM-29/35).
	// May be nil (treated as empty — no set bonuses).
	setRegistry *inventory.SetRegistry
	// substanceReg holds all substance definitions.
	// REQ-AH-3: loaded at startup from content/substances/.
	substanceReg *substance.Registry
	// factionRegistry holds all faction definitions loaded at startup.
	// May be nil when the faction feature is not configured.
	factionRegistry *faction.FactionRegistry
	// factionSvc provides faction logic operations.
	// May be nil when factionRegistry is nil.
	factionSvc *faction.Service
	// factionRepRepo persists per-character faction reputation scores.
	// May be nil when faction feature is not configured.
	factionRepRepo FactionRepRepository
	// materialReg holds all crafting material definitions loaded at startup.
	// May be nil when the crafting feature is not yet configured.
	materialReg *crafting.MaterialRegistry
	// materialRepo persists per-character material inventories.
	// May be nil when the crafting feature is not yet configured.
	materialRepo CharacterMaterialsRepository
	// recipeReg holds all crafting recipe definitions loaded at startup.
	// May be nil when the crafting feature is not yet configured.
	recipeReg *crafting.RecipeRegistry
	// craftEngine executes crafting logic.
	// May be nil when the crafting feature is not yet configured.
	craftEngine *crafting.CraftingEngine
	// factionConfig holds global faction economy parameters (rep costs, rep per service).
	// May be nil when faction feature is not configured.
	factionConfig *faction.FactionConfig
	// questSvc handles quest lifecycle (offer, accept, progress, complete, abandon).
	// May be nil when no quests are configured.
	questSvc *quest.Service
	// gameHourFn returns the current game hour (0–23). Used by NPC schedule evaluation. REQ-NB-16.
	gameHourFn func() int
	// npcIdleTickInterval is the ZoneTickManager tick interval used to convert
	// say cooldown durations to tick counts. REQ-NB-2.
	npcIdleTickInterval time.Duration
	// lastTimePeriod tracks the last observed time period for transition-based recharge triggers.
	// Protected by itemTickMu; owned exclusively by the item-tick goroutine (REQ-ACT-21).
	lastTimePeriod TimePeriod
	// itemTickMu protects lastTimePeriod; owned exclusively by the item-tick goroutine.
	itemTickMu sync.Mutex
	// downtimeRepo persists active downtime state for characters.
	// May be nil (downtime state is not persisted across restarts if not set).
	downtimeRepo CharacterDowntimeRepository
	// downtimeQueueRepo manages the per-character downtime activity queue.
	// May be nil (queue features are disabled if not set).
	downtimeQueueRepo DowntimeQueueRepo
	// downtimeQueueLimitReg holds per-tier/per-level queue slot limits.
	// May be nil (queue limit lookups are skipped if not set).
	downtimeQueueLimitReg *downtime.DowntimeQueueLimitRegistry
	// drawbackEngine evaluates situational drawback triggers and applies conditions (REQ-JD-10).
	drawbackEngine *drawback.Engine
	// characterJobsRepo persists the set of all jobs held by each character (REQ-JD-4).
	// May be nil (job persistence to character_jobs table is skipped if not set).
	characterJobsRepo CharacterJobsRepository
	// seduceConditions maps NPC instance ID → their runtime condition set for charmed tracking.
	// Used by executeSeduce (REQ-ZN-7/8) and the charmed saving throw at round end (REQ-ZN-9).
	// Initialized lazily; always access via getOrCreateSeduceConditions.
	seduceConditions map[string]*condition.ActiveSet
	// weatherMgr manages active weather events and provides per-tick effect application.
	// May be nil when weather feature is not configured.
	weatherMgr *WeatherManager
	// pendingTechSlotsRepo persists L2+ pending tech slots awaiting trainer resolution (REQ-TTA-12).
	// May be nil (pending slot persistence is skipped if not set).
	pendingTechSlotsRepo PendingTechSlotsRepo
	// maxHotbars is the maximum number of hotbar pages a player may configure.
	// Defaults to 4 when not set via SetMaxHotbars. REQ-HB-2.
	maxHotbars int
	// autoNavStepMs is the delay in milliseconds between auto-navigation steps sent to the web client.
	// Defaults to 1000 when not set via SetAutoNavStepMs. (REQ-CNT-2)
	autoNavStepMs int
	// reactionPromptHub routes ReactionResponse ClientMessages back to the
	// goroutine blocked inside buildReactionCallback. See reaction_prompt_hub.go.
	reactionPromptHub *reactionPromptHub
	// reactionPromptTimeout bounds the interactive reaction prompt wait
	// independently from the CombatHandler's ResolveRound timeout. Treated as
	// the deadline_unix_ms sent to clients when non-zero; default
	// config.DefaultReactionPromptTimeout applies at Set time.
	reactionPromptTimeout time.Duration
}

// applyArmorTrainingProficiency applies the armor_training feat choice as a real proficiency
// on the session and persists it to the repository.
//
// Precondition: featureChoices["armor_training"]["armor_category"] must contain a valid armor
// category string if the armor_training feat has been chosen.
// Postcondition: profs[category] == "trained" when a choice exists. If profRepo is non-nil and
// the category is not already in profs, an Upsert is attempted; failure is logged at Error level
// but the in-memory proficiency is always set regardless.
func applyArmorTrainingProficiency(
	ctx context.Context,
	characterID int64,
	featureChoices map[string]map[string]string,
	profRepo CharacterProficienciesRepository,
	profs map[string]string,
	logger *zap.Logger,
) {
	armorCategory := featureChoices["armor_training"]["armor_category"]
	if armorCategory == "" {
		return
	}
	if profRepo != nil {
		if _, alreadySet := profs[armorCategory]; !alreadySet {
			if upsertErr := profRepo.Upsert(ctx, characterID, armorCategory, "trained"); upsertErr != nil {
				logger.Error("persisting armor_training proficiency",
					zap.String("category", armorCategory),
					zap.Error(upsertErr),
				)
			}
		}
	}
	profs[armorCategory] = "trained"
}

// CharacterJobsRepository persists per-character job lists in the character_jobs table.
//
// Precondition: characterID must be > 0.
type CharacterJobsRepository interface {
	AddJob(ctx context.Context, characterID int64, jobID string) error
	RemoveJob(ctx context.Context, characterID int64, jobID string) error
	ListJobs(ctx context.Context, characterID int64) ([]string, error)
}

// CharacterDowntimeRepository persists active downtime state for characters.
//
// Precondition: characterID must be > 0.
type CharacterDowntimeRepository interface {
	Save(ctx context.Context, characterID int64, state postgres.DowntimeState) error
	Load(ctx context.Context, characterID int64) (*postgres.DowntimeState, error)
	Clear(ctx context.Context, characterID int64) error
}

// DowntimeQueueRepo abstracts CharacterDowntimeQueueRepository for testing.
//
// Precondition: non-nil implementation.
type DowntimeQueueRepo interface {
	PopHead(ctx context.Context, characterID int64) (*postgres.QueueEntry, error)
	Enqueue(ctx context.Context, characterID int64, activityID string, activityArgs string) error
	ListQueue(ctx context.Context, characterID int64) ([]postgres.QueueEntry, error)
	RemoveAt(ctx context.Context, characterID int64, position int) error
	Clear(ctx context.Context, characterID int64) error
}

// HealerCapacityRepo persists and loads healer NPC daily capacity usage keyed by template ID.
//
// Precondition: templateID must be non-empty.
type HealerCapacityRepo interface {
	Save(ctx context.Context, templateID string, capacityUsed int) error
	LoadAll(ctx context.Context) (map[string]int, error)
}

// DetainedUntilUpdater persists the detained_until timestamp for a character.
//
// Precondition: characterID must be > 0.
// Postcondition: detained_until is updated; nil clears the detention.
type DetainedUntilUpdater interface {
	UpdateDetainedUntil(ctx context.Context, characterID int64, detainedUntil *time.Time) error
}

// CharacterMaterialsRepository persists and loads per-character material inventories.
//
// Precondition: characterID must be > 0; materialID must be non-empty.
// Postcondition: Mutations are durably persisted before returning.
type CharacterMaterialsRepository interface {
	Load(ctx context.Context, characterID int64) (map[string]int, error)
	Add(ctx context.Context, characterID int64, materialID string, amount int) error
	DeductMany(ctx context.Context, characterID int64, deductions map[string]int) error
}

// FactionRepRepository persists and loads per-character faction reputation.
//
// Precondition: characterID > 0; factionID non-empty.
// Postcondition: Mutations are durably persisted before returning.
type FactionRepRepository interface {
	SaveRep(ctx context.Context, characterID int64, factionID string, rep int) error
	LoadRep(ctx context.Context, characterID int64) (map[string]int, error)
}

// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: storage, content, and handlers must be fully populated.
// sessMgr, cmdRegistry, and logger must be non-nil.
// Postcondition: Returns a fully initialised GameServiceServer.
func NewGameServiceServer(
	storage StorageDeps,
	content ContentDeps,
	handlers HandlerDeps,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	gameCalendar *GameCalendar,
	logger *zap.Logger,
) *GameServiceServer {
	s := &GameServiceServer{
		world:                      content.WorldMgr,
		sessions:                   sessMgr,
		commands:                   cmdRegistry,
		worldH:                     handlers.WorldHandler,
		chatH:                      handlers.ChatHandler,
		charSaver:                  storage.CharRepo,
		dice:                       content.DiceRoller,
		npcH:                       handlers.NPCHandler,
		npcMgr:                     content.NpcMgr,
		combatH:                    handlers.CombatHandler,
		scriptMgr:                  content.ScriptMgr,
		respawnMgr:                 content.RespawnMgr,
		floorMgr:                   content.FloorMgr,
		roomEquipMgr:               content.RoomEquipMgr,
		automapRepo:                storage.AutomapRepo,
		invRegistry:                content.InvRegistry,
		accountAdmin:               storage.AccountRepo,
		calendar:                   gameCalendar,
		logger:                     logger,
		jobRegistry:                content.JobRegistry,
		condRegistry:               content.CondRegistry,
		techRegistry:               content.TechRegistry,
		hardwiredTechRepo:          storage.HardwiredTechRepo,
		preparedTechRepo:           storage.PreparedTechRepo,
		knownTechRepo:        storage.KnownTechRepo,
		innateTechRepo:             storage.InnateTechRepo,
		spontaneousUsePoolRepo:     storage.SpontaneousUsePoolRepo,
		loadoutsDir:                string(content.LoadoutsDir),
		allSkills:                  content.AllSkills,
		characterSkillsRepo:        storage.SkillsRepo,
		characterProficienciesRepo: storage.ProficienciesRepo,
		allFeats:                   content.AllFeats,
		featRegistry:               content.FeatRegistry,
		characterFeatsRepo:         storage.FeatsRepo,
		featLevelGrantsRepo:        storage.FeatLevelGrantsRepo,
		allClassFeatures:           content.ClassFeatures,
		classFeatureRegistry:       content.ClassFeatureRegistry,
		characterClassFeaturesRepo: storage.ClassFeaturesRepo,
		featureChoicesRepo:         storage.FeatureChoicesRepo,
		charAbilityBoostsRepo:      storage.AbilityBoostsRepo,
		archetypes:                 content.ArchetypeMap,
		regions:                    content.RegionMap,
		mentalStateMgr:             content.MentalStateMgr,
		actionH:                    handlers.ActionHandler,
		wantedRepo:                 storage.WantedRepo,
		detainedUntilRepo:          storage.DetainedUntilRepo,
		trapMgr:                    nil, // trap loading not yet wired
		trapTemplates:              nil, // trap loading not yet wired
		setRegistry:                content.SetRegistry,
		substanceReg:               content.SubstanceRegistry,
		factionRepRepo:             storage.FactionRepRepo,
		materialReg:                content.MaterialRegistry,
		materialRepo:               storage.MaterialRepo,
		recipeReg:                  content.RecipeRegistry,
		craftEngine:                crafting.NewEngine(),
		characterJobsRepo:          storage.CharacterJobsRepo,
		seduceConditions:           make(map[string]*condition.ActiveSet),
		reactionPromptHub:          newReactionPromptHub(),
		reactionPromptTimeout:      config.DefaultReactionPromptTimeout,
	}
	if content.FactionRegistry != nil {
		s.factionRegistry = content.FactionRegistry
		s.factionSvc = faction.NewServiceWithRepo(*content.FactionRegistry, storage.FactionRepRepo)
	}
	if content.FactionConfig != nil {
		s.factionConfig = content.FactionConfig
	}
	// REQ-CCF-3: wire faction combat initiation on NPC respawn placement.
	if s.respawnMgr != nil && s.factionRegistry != nil {
		reg := *s.factionRegistry
		s.respawnMgr.AfterPlace = func(inst *npc.Instance, roomID string) {
			checkFactionInitiation(
				inst, roomID,
				func(rID string) []*npc.Instance { return s.npcMgr.InstancesInRoom(rID) },
				func(rID string) *world.Room {
					r, _ := s.world.GetRoom(rID)
					return r
				},
				func(factionID string) []string {
					def := reg.ByID(factionID)
					if def == nil {
						return nil
					}
					return def.HostileFactions
				},
				func(attacker, target *npc.Instance, room *world.Room) {
					s.initiateNPCFactionCombat(attacker, target, roomID)
				},
			)
		}
	}
	if storage.QuestRepo != nil {
		// xpSvc is nil at construction time; wired later via SetXPService → SetQuestXPAwarder.
		s.questSvc = quest.NewService(content.QuestRegistry, storage.QuestRepo, nil, s.invRegistry, s.charSaver)
	}
	if storage.DowntimeRepo != nil {
		s.downtimeRepo = storage.DowntimeRepo
	}
	if storage.DowntimeQueueRepo != nil {
		s.downtimeQueueRepo = storage.DowntimeQueueRepo
	}
	if content.DowntimeQueueLimitRegistry != nil {
		s.downtimeQueueLimitReg = content.DowntimeQueueLimitRegistry
	}
	// gameHourFn defaults to reading from calendar if available. REQ-NB-16.
	s.gameHourFn = func() int {
		if s.calendar != nil {
			return int(s.calendar.CurrentDateTime().Hour)
		}
		return 0
	}
	if s.combatH != nil {
		s.combatH.SetOnCombatEnd(func(roomID string) {
			sessions := s.sessions.PlayersInRoomDetails(roomID)
			for _, sess := range sessions {
				sess.Status = int32(1) // gamev1.CombatStatus_COMBAT_STATUS_IDLE
				if sess.Conditions != nil {
					// REQ-80-1: apply 1-minute Wrath cooldown before clearing encounter conditions.
					if sess.Conditions.Has("wrath_active") {
						sess.WrathCooldownUntil = time.Now().Add(time.Minute)
					}
					sess.Conditions.ClearEncounter()
				}
				sess.MaterialState.CombatUsed = make(map[string]bool)
				sess.HasHitThisCombat = false
			}
			// Clear pending join invitations for this room now that combat has ended.
			s.clearPendingJoinForRoom(roomID)
			// Push updated room view so "fighting X" labels clear immediately.
			s.pushRoomViewToAllInRoom(roomID)
		})
		s.combatH.SetRoundStartBroadcastFn(func(roomID string, evt *gamev1.RoundStartEvent) {
			s.broadcastToRoom(roomID, "", &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_RoundStart{RoundStart: evt},
			})
		})
		s.combatH.SetAPUpdateBroadcastFn(func(roomID string, evt *gamev1.APUpdateEvent) {
			s.broadcastToRoom(roomID, "", &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_ApUpdate{ApUpdate: evt},
			})
		})
		s.worldH.SetCombatHandler(s.combatH)
		// REQ-AH-21: wire substance service for poison-on-hit.
		s.combatH.SetSubstanceSvc(s)
		// REQ-FA-20: wire faction service for rep-on-kill.
		if s.factionSvc != nil && s.factionConfig != nil {
			s.combatH.SetFactionService(s.factionSvc, s.factionConfig)
		}
		// REQ-QU-19: wire quest service for kill-progress recording.
		if s.questSvc != nil {
			s.combatH.SetQuestService(s.questSvc)
		}
		// REQ-BUG61-1: wire pushCharacterSheet so the web UI Stats tab updates after XP award.
		s.combatH.SetPushCharacterSheetFn(s.pushCharacterSheet)
		// Wire pushQuestLogView so the quest drawer auto-refreshes when a kill quest completes.
		s.combatH.SetPushQuestLogFn(s.pushQuestLogView)
		// REQ-BUG99-1: wire applyLevelUpTechGrants so organic XP level-ups issue trainer quests.
		s.combatH.SetOnLevelUpFn(s.applyLevelUpTechGrants)
		// REQ-BUG92-1: wire pushInventory so the web UI Inventory tab updates after currency award.
		s.combatH.SetPushInventoryFn(s.pushInventory)
		// REQ-BUG96-1: wire saveInventory so looted items are persisted immediately.
		s.combatH.SetSaveInventoryFn(s.saveInventory)
		// Wire tech use resolver so ActionUseTech round events apply effects post-round.
		// cbt is passed directly from resolveAndAdvanceLocked to avoid re-acquiring combatMu
		// inside the callback (which would deadlock since combatMu is already held at call time).
		s.combatH.SetTechUseResolverFn(func(uid, techID, targetID string, targetX, targetY int32, cbt *combat.Combat) {
			sess, ok := s.sessions.GetPlayer(uid)
			if !ok {
				return
			}
			evt, err := s.activateTechWithEffectsWithCombat(sess, uid, techID, targetID,
				fmt.Sprintf("You use %s.", techID), nil, targetX, targetY, cbt)
			if err != nil || evt == nil {
				return
			}
			s.pushEventToUID(uid, evt)
		})
	}
	s.merchantRuntimeStates = make(map[string]*npc.MerchantRuntimeState)
	s.bankerRuntimeStates = make(map[string]*npc.BankerRuntimeState)
	s.healerRuntimeStates = make(map[string]*npc.HealerRuntimeState)
	s.hirelingRuntimeStates = make(map[string]*npc.HirelingRuntimeState)
	s.WireCoverCrossfireTrap()
	s.WireConsumableTrapTrigger()
	s.wireRevealZone()
	s.wireScriptMgrCombatCallbacks()
	// Initialize drawback engine for situational trigger evaluation (REQ-JD-10).
	s.drawbackEngine = drawback.NewEngine(s.condRegistry)
	// REQ-JD-10: Wire on_take_damage_in_one_hit_above_threshold drawback trigger into CombatHandler.
	if s.combatH != nil {
		s.combatH.SetOnPlayerDeath(func(uid string) {
			s.respawnPlayer(uid)
		})
		s.combatH.SetOnMassiveDamage(func(uid string) {
			playerSess, ok := s.sessions.GetPlayer(uid)
			if !ok || s.jobRegistry == nil || playerSess.Conditions == nil {
				return
			}
			heldJobs := s.resolveHeldJobs(playerSess)
			s.drawbackEngine.FireTrigger(uid, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, heldJobs, playerSess.Conditions, time.Now())
		})
		s.combatH.SetOnNPCDamageTaken(s.pauseRovingOnCombat)
		s.combatH.SetOnNPCDeath(func(instID string) {
			if s.rovingMgr != nil {
				s.rovingMgr.Unregister(instID)
			}
		})
		// REQ-ZN-9: wire seduceConditions so CombatHandler can process charmed saves at round end.
		s.combatH.SetSeduceConditions(s.seduceConditions)
	}
	if s.maxHotbars <= 0 {
		s.maxHotbars = 4
	}
	if s.autoNavStepMs < 100 {
		s.autoNavStepMs = 1000
	}
	return s
}

// ValidateQuestRegistry cross-checks all quest definitions against live registries.
// Must be called after all registries are loaded and before Serve().
//
// Precondition: npcMgr, invRegistry, and world must be fully initialized.
// Postcondition: Returns non-nil error if any quest reference is unresolvable.
func (s *GameServiceServer) ValidateQuestRegistry() error {
	if s.questSvc == nil || len(s.questSvc.Registry()) == 0 {
		return nil
	}
	npcIDs := s.npcMgr.AllTemplateIDs()
	itemIDs := s.invRegistry.AllItemIDs()
	roomIDs := s.world.AllRoomIDs()
	return s.questSvc.Registry().CrossValidate(npcIDs, itemIDs, roomIDs)
}

// ValidateZoneNPCLevels cross-checks all zone NPC spawns against zone level ranges.
// Zones without min_level/max_level (both 0) are skipped.
//
// Precondition: world and npcMgr must be fully initialized.
// Postcondition: returns nil if all NPC levels are in-range, or a non-nil error describing the violation.
func (s *GameServiceServer) ValidateZoneNPCLevels() error {
	zones := s.world.AllZones()
	for _, zone := range zones {
		if err := zone.ValidateNPCLevels(s.npcMgr); err != nil {
			return err
		}
	}
	return nil
}

// SetNPCIdleTickInterval sets the zone-tick interval used to convert say cooldown durations
// to tick counts. REQ-NB-2. Must be called before StartZoneTicks.
//
// Precondition: interval must be > 0.
func (s *GameServiceServer) SetNPCIdleTickInterval(interval time.Duration) {
	if interval > 0 {
		s.npcIdleTickInterval = interval
	}
}

// SetProgressRepo registers the CharacterProgressRepository used to load pending boosts at login.
//
// Precondition: repo must be non-nil.
// Postcondition: PendingBoosts are loaded from the DB on each player login.
func (s *GameServiceServer) SetProgressRepo(repo ProgressRepository) {
	s.progressRepo = repo
}

// SetPendingTechSlotsRepo wires the pending tech slots repository for L2+ trainer slot persistence (REQ-TTA-12).
//
// Precondition: repo may be nil (persistence is skipped when nil).
// Postcondition: s.pendingTechSlotsRepo is set.
func (s *GameServiceServer) SetPendingTechSlotsRepo(repo PendingTechSlotsRepo) {
	s.pendingTechSlotsRepo = repo
}

// World returns the world Manager. Used by startup initialization.
func (s *GameServiceServer) World() *world.Manager {
	return s.world
}

// SetWorldEditor sets the WorldEditor after startup writability check.
// Passing nil disables world-editing commands.
func (s *GameServiceServer) SetWorldEditor(we *world.WorldEditor) {
	s.worldEditor = we
}

// SetWeatherManager wires the WeatherManager into the GameServiceServer so weather
// effects are applied on room entry and weather events are sent on session join.
//
// Precondition: wm may be nil (weather features are skipped when nil).
// Postcondition: s.weatherMgr is set; weather effect and event processing are enabled.
func (s *GameServiceServer) SetWeatherManager(wm *WeatherManager) {
	s.weatherMgr = wm
}

// SetXPService registers the XP service used to award experience.
//
// Precondition: svc must be non-nil.
// Postcondition: XP is awarded at kill, room discovery, and skill check sites.
func (s *GameServiceServer) SetXPService(svc *xp.Service) {
	s.xpSvc = svc
	if s.questSvc != nil {
		s.questSvc.SetXPAwarder(&xpServiceQuestAdapter{svc: svc})
	}
}

// xpServiceQuestAdapter bridges *xp.Service to quest.XPAwarder.
// It type-asserts the quest.SessionState to *session.PlayerSession; if the assertion
// fails the award is skipped rather than panicking.
//
// Precondition: svc must be non-nil.
type xpServiceQuestAdapter struct {
	svc *xp.Service
}

// AwardXPAmount implements quest.XPAwarder by delegating to the underlying xp.Service.
//
// Precondition: sess must be a *session.PlayerSession; characterID > 0; xpAmount >= 0.
// Postcondition: XP is awarded when the type assertion succeeds; returns nil otherwise.
func (a *xpServiceQuestAdapter) AwardXPAmount(ctx context.Context, sess quest.SessionState, characterID int64, xpAmount int) ([]string, error) {
	ps, ok := sess.(*session.PlayerSession)
	if !ok {
		return nil, nil
	}
	return a.svc.AwardXPAmount(ctx, ps, characterID, xpAmount)
}

// SetJobRegistry injects a job registry for testing.
func (s *GameServiceServer) SetJobRegistry(r *ruleset.JobRegistry) { s.jobRegistry = r }

// SetHardwiredTechRepo injects a hardwired tech repo for testing.
func (s *GameServiceServer) SetHardwiredTechRepo(r HardwiredTechRepo) { s.hardwiredTechRepo = r }

// SetPreparedTechRepo injects a prepared tech repo for testing.
func (s *GameServiceServer) SetPreparedTechRepo(r PreparedTechRepo) { s.preparedTechRepo = r }

// SetKnownTechRepo injects a spontaneous tech repo for testing.
func (s *GameServiceServer) SetKnownTechRepo(r KnownTechRepo) { s.knownTechRepo = r }

// SetInnateTechRepo injects an innate tech repo for testing.
func (s *GameServiceServer) SetInnateTechRepo(r InnateTechRepo) { s.innateTechRepo = r }

// SetTechRegistry replaces the server's technology registry. Used in tests.
func (s *GameServiceServer) SetTechRegistry(r *technology.Registry) { s.techRegistry = r }

// SetHealerCapacityRepo injects the healer capacity repository.
func (s *GameServiceServer) SetHealerCapacityRepo(r HealerCapacityRepo) {
	s.healerCapacityRepo = r
}

// SetCharSaver sets the character saver (used in tests).
func (s *GameServiceServer) SetCharSaver(cs CharacterSaver) {
	s.charSaver = cs
}

// SetMaxHotbars sets the maximum number of hotbar pages a player may configure.
// A value <= 0 is ignored; the existing value (default 4) is retained.
func (s *GameServiceServer) SetMaxHotbars(n int) {
	if n > 0 {
		s.maxHotbars = n
	}
}

// SetAutoNavStepMs sets the auto-navigation step delay in milliseconds.
// Precondition: ms must be >= 100. If ms < 100, it is clamped to 1000.
func (s *GameServiceServer) SetAutoNavStepMs(ms int) {
	if ms >= 100 {
		s.autoNavStepMs = ms
	}
}

// SetReactionPromptTimeout sets the interactive reaction prompt timeout
// delivered to clients via the deadline_unix_ms field.
// Values <= 0 are ignored; the existing default is retained.
func (s *GameServiceServer) SetReactionPromptTimeout(d time.Duration) {
	if d > 0 {
		s.reactionPromptTimeout = d
	}
}

// FeatRegistry returns the feat registry used by this service.
//
// Postcondition: Returns nil when no feat registry has been set.
func (s *GameServiceServer) FeatRegistry() *ruleset.FeatRegistry {
	return s.featRegistry
}

// Session implements the bidirectional streaming RPC.
// Flow:
//  1. Wait for JoinWorldRequest
//  2. Create player session, place in start room
//  3. Spawn goroutine to forward entity events to gRPC stream
//  4. Main loop: read ClientMessage, dispatch, send response
//  5. On disconnect: clean up session
//
// Postcondition: sess.LoadoutSet and sess.Equipment are loaded from DB (or default-initialized on error).
func (s *GameServiceServer) Session(stream gamev1.GameService_SessionServer) error {
	// Step 1: Wait for JoinWorldRequest
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving join request: %w", err)
	}

	joinReq := firstMsg.GetJoinWorld()
	if joinReq == nil {
		return fmt.Errorf("first message must be JoinWorldRequest")
	}

	uid := joinReq.Uid
	username := joinReq.Username
	charName := joinReq.CharacterName
	if charName == "" {
		charName = username // fallback for backward compat
	}
	characterID := joinReq.CharacterId
	currentHP := int(joinReq.CurrentHp)

	s.logger.Info("player joining world",
		zap.String("uid", uid),
		zap.String("username", username),
		zap.String("char_name", charName),
		zap.Int64("character_id", characterID),
	)

	// Step 2: Create player session in saved location (or global start room)
	var spawnRoom *world.Room
	if loc := joinReq.Location; loc != "" {
		if r, ok := s.world.GetRoom(loc); ok {
			spawnRoom = r
		}
	}
	if spawnRoom == nil {
		spawnRoom = s.world.StartRoom()
	}
	if spawnRoom == nil {
		return fmt.Errorf("no start room configured")
	}

	role := joinReq.Role
	if role == "" {
		role = "player"
	}

	// Load MaxHP and Abilities from the DB (server-authoritative).
	// currentHP is taken from the proto (the persisted current_hp sent by the frontend).
	var maxHP int
	var abilities character.AbilityScores
	var dbChar *character.Character
	if characterID > 0 && s.charSaver != nil {
		var dbErr error
		dbChar, dbErr = s.charSaver.GetByID(stream.Context(), characterID)
		if dbErr != nil {
			s.logger.Warn("failed to load character from DB at login; using zero values",
				zap.Int64("character_id", characterID),
				zap.Error(dbErr),
			)
			dbChar = nil
		} else if dbChar != nil {
			maxHP = dbChar.MaxHP
			abilities = dbChar.Abilities
		}
	}
	defaultCombatAction := "attack"
	if dbChar != nil && dbChar.DefaultCombatAction != "" {
		defaultCombatAction = dbChar.DefaultCombatAction
	}
	genderVal := ""
	teamVal := ""
	if dbChar != nil {
		genderVal = dbChar.Gender
		teamVal = dbChar.Team
	}

	// Team territory enforcement: redirect spawn to home room if saved location is in enemy territory (REQ-TEAM-3).
	if teamVal != "" {
		if spawnRoom != nil {
			if isEnemyZone(teamVal, spawnRoom.ZoneID) {
				if homeRoomID, homeOK := teamHomeRooms[teamVal]; homeOK {
					if homeRoom, worldOK := s.world.GetRoom(homeRoomID); worldOK {
						s.logger.Warn("player spawn in enemy zone; redirecting to home room",
							zap.String("uid", uid),
							zap.String("team", teamVal),
							zap.String("bad_room", spawnRoom.ID),
							zap.String("home_room", homeRoomID),
						)
						spawnRoom = homeRoom
					}
				}
			}
		}
	}

	sess, err := s.sessions.AddPlayer(session.AddPlayerOptions{
		UID:                 uid,
		Username:            username,
		CharName:            charName,
		CharacterID:         characterID,
		RoomID:              spawnRoom.ID,
		CurrentHP:           currentHP,
		MaxHP:               maxHP,
		Abilities:           abilities,
		Role:                role,
		RegionDisplayName:   joinReq.RegionDisplay,
		Class:               joinReq.Class,
		Level:               int(joinReq.Level),
		DefaultCombatAction: defaultCombatAction,
		Gender:              genderVal,
		Team:                teamVal,
	})
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	// Capture the entity assigned to THIS session so cleanupPlayer can guard
	// against stale cleanup after a rapid reconnect evicts this session.
	myEntity := sess.Entity
	defer func() { s.cleanupPlayer(uid, username, myEntity) }()

	// Restore combat status if the player reconnected mid-combat.
	// AddPlayer always initialises Status=1 (idle); if an active combat in this room
	// already has the player as a combatant, reset Status to statusInCombat so that
	// combat commands (stride, pass, etc.) work immediately after reconnect.
	// REQ-BUG-143: Also push a RoundStartEvent directly to the reconnecting player so
	// the web client immediately restores its combat UI without a visible interruption.
	if s.combatH != nil {
		if cbt, inCombat := s.combatH.GetCombatForRoom(sess.RoomID); inCombat {
			if cbt.GetCombatant(uid) != nil {
				sess.Status = statusInCombat
				// Build and push a RoundStartEvent to restore the client's combat state.
				turnOrder := make([]string, 0, len(cbt.Combatants))
				for _, c := range cbt.Combatants {
					turnOrder = append(turnOrder, c.Name)
				}
				positions := make([]*gamev1.CombatantPosition, 0, len(cbt.Combatants))
				for _, c := range cbt.Combatants {
					positions = append(positions, &gamev1.CombatantPosition{
						Name: c.Name, X: int32(c.GridX), Y: int32(c.GridY),
					})
				}
				_ = stream.Send(&gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_RoundStart{
						RoundStart: &gamev1.RoundStartEvent{
							Round:            int32(cbt.Round),
							ActionsPerTurn:   3,
							DurationMs:       int32(s.combatH.roundDuration.Milliseconds()),
							TurnOrder:        turnOrder,
							InitialPositions: positions,
						},
					},
				})
			}
		}
	}

	// Propagate headless flag from the join request; used to skip interactive prompts.
	sess.Headless = joinReq.Headless

	// Set home region ID for Honkeypot targeting.
	if dbChar != nil {
		sess.Region = dbChar.Region
	}

	// Backfill gender: if the loaded character has no gender, assign a random standard gender and persist it.
	if sess.Gender == "" && characterID > 0 && s.charSaver != nil {
		sess.Gender = character.RandomStandardGender()
		if gErr := s.charSaver.SaveGender(stream.Context(), characterID, sess.Gender); gErr != nil {
			s.logger.Warn("backfilling gender", zap.Int64("character_id", characterID), zap.Error(gErr))
		}
	}

	// Load XP, level, maxHP, and pending boosts into session from persisted state.
	if characterID > 0 && s.progressRepo != nil {
		dbLevel, dbExperience, dbMaxHP, boosts, progressErr := s.progressRepo.GetProgress(stream.Context(), characterID)
		if progressErr == nil {
			sess.Level = dbLevel
			sess.Experience = dbExperience
			sess.MaxHP = dbMaxHP
			sess.PendingBoosts = boosts
			if skillIncreases, siErr := s.progressRepo.GetPendingSkillIncreases(stream.Context(), characterID); siErr == nil {
				sess.PendingSkillIncreases = skillIncreases
			} else {
				s.logger.Warn("GetPendingSkillIncreases failed", zap.Error(siErr))
			}
			// Backfill: existing characters that have never been granted skill increases
			// get floor(level/2) pending. The initialized flag prevents re-granting on
			// subsequent logins after the player has spent their increases.
			earnedIncreases := sess.Level / 2
			if earnedIncreases > 0 {
				initialized, initErr := s.progressRepo.IsSkillIncreasesInitialized(stream.Context(), characterID)
				if initErr != nil {
					s.logger.Warn("IsSkillIncreasesInitialized failed", zap.Error(initErr))
				} else if !initialized {
					if bfErr := s.progressRepo.IncrementPendingSkillIncreases(stream.Context(), characterID, earnedIncreases); bfErr == nil {
						sess.PendingSkillIncreases = earnedIncreases
						if markErr := s.progressRepo.MarkSkillIncreasesInitialized(stream.Context(), characterID); markErr != nil {
							s.logger.Warn("MarkSkillIncreasesInitialized failed", zap.Error(markErr))
						}
					} else {
						s.logger.Warn("backfill skill increases failed", zap.Error(bfErr))
					}
				}
			}
		} else {
			s.logger.Warn("loading progress at login",
				zap.Int64("character_id", characterID),
				zap.Error(progressErr),
			)
			// Fall back to client-supplied level and DB character experience when available.
			if dbChar != nil {
				sess.Experience = dbChar.Experience
			}
		}
	} else if dbChar != nil {
		sess.Experience = dbChar.Experience
	}

	// Load automap cache from DB and record spawn room discovery.
	if s.automapRepo != nil {
		automapResult, loadErr := s.automapRepo.LoadAll(stream.Context(), characterID)
		if loadErr != nil {
			s.logger.Warn("loading automap", zap.Error(loadErr))
		} else {
			sess.AutomapCache = automapResult.AllKnown
			sess.ExploredCache = automapResult.ExploredOnly
		}
	}
	// Record spawn room discovery.
	if spawnRoom != nil {
		zID := spawnRoom.ZoneID
		if sess.AutomapCache[zID] == nil {
			sess.AutomapCache[zID] = make(map[string]bool)
		}
		if sess.ExploredCache[zID] == nil {
			sess.ExploredCache[zID] = make(map[string]bool)
		}
		if !sess.AutomapCache[zID][spawnRoom.ID] {
			sess.AutomapCache[zID][spawnRoom.ID] = true
			sess.ExploredCache[zID][spawnRoom.ID] = true
			if s.automapRepo != nil {
				if err := s.automapRepo.Insert(stream.Context(), characterID, zID, spawnRoom.ID, true); err != nil {
					s.logger.Warn("persisting spawn room discovery", zap.Error(err))
				}
			}
			if s.questSvc != nil {
				_, _ = s.questSvc.RecordExplore(stream.Context(), sess, sess.CharacterID, spawnRoom.ID)
			}
		}
	}

	// Load persisted wanted levels.
	if s.wantedRepo != nil {
		wantedLevels, wantedErr := s.wantedRepo.Load(stream.Context(), sess.CharacterID)
		if wantedErr != nil {
			s.logger.Warn("failed to load wanted levels", zap.Int64("characterID", sess.CharacterID), zap.Error(wantedErr))
		} else {
			sess.WantedLevel = wantedLevels
		}
	}

	// Load faction_id from character record (REQ-FA-26).
	if dbChar != nil {
		sess.FactionID = dbChar.FactionID
	}

	// Load faction rep from DB (REQ-FA-24).
	if s.factionRepRepo != nil {
		repMap, repErr := s.factionRepRepo.LoadRep(stream.Context(), sess.CharacterID)
		if repErr != nil {
			s.logger.Warn("failed to load faction rep", zap.Error(repErr))
		} else {
			sess.FactionRep = repMap
		}
	}

	// Restore detained_until from DB so offline detention expiry is honoured on reconnect.
	if dbChar != nil && dbChar.DetainedUntil != nil {
		sess.DetainedUntil = dbChar.DetainedUntil
		// Apply detained condition so enforcement is active immediately.
		if s.condRegistry != nil {
			if def, ok := s.condRegistry.Get("detained"); ok {
				sess.Conditions.Apply(sess.UID, def, 1, -1) //nolint:errcheck
			}
		}
	}
	// Check whether detention has already expired (handles offline reconnect).
	s.checkDetentionCompletion(sess)

	// Load persisted equipment state if charSaver supports it.
	if characterID > 0 && s.charSaver != nil {
		loadCtx, loadCancel := context.WithTimeout(stream.Context(), 5*time.Second)
		ls, lsErr := s.charSaver.LoadWeaponPresets(loadCtx, characterID, s.invRegistry)
		loadCancel()
		if lsErr != nil {
			s.logger.Warn("failed to load weapon presets on login",
				zap.String("uid", uid),
				zap.Int64("character_id", characterID),
				zap.Error(lsErr),
			)
			sess.LoadoutSet = inventory.NewLoadoutSet()
		} else {
			sess.LoadoutSet = ls
		}

		loadCtx2, loadCancel2 := context.WithTimeout(stream.Context(), 5*time.Second)
		eq, eqErr := s.charSaver.LoadEquipment(loadCtx2, characterID)
		loadCancel2()
		if eqErr != nil {
			s.logger.Warn("failed to load equipment on login",
				zap.String("uid", uid),
				zap.Int64("character_id", characterID),
				zap.Error(eqErr),
			)
			sess.Equipment = inventory.NewEquipment()
		} else {
			sess.Equipment = eq
			hydrateEquipmentNames(sess.Equipment, s.invRegistry)
		}

		// Compute defense stats from equipped armor (AC, resistances, weaknesses).
		// REQ-EM-29/35: compute set bonuses at login.
		{
			loginDexMod := (sess.Abilities.Quickness - 10) / 2
			session.RecomputeSetBonuses(sess, s.setRegistry)
			loginDef := sess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(s.invRegistry, loginDexMod, sess.Proficiencies, sess.Level, sess.SetBonusSummary)
			sess.Resistances = loginDef.Resistances
			sess.Weaknesses = loginDef.Weaknesses
		}

		// Compute passive material bonuses from equipped items at login.
		if sess.LoadoutSet != nil && sess.Equipment != nil {
			if active := sess.LoadoutSet.ActivePreset(); active != nil {
				equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
				sess.PassiveMaterials = inventory.ComputePassiveMaterials(equipped, sess.Equipment.Armor, s.invRegistry)
			}
		}

		// Load persisted inventory.
		{
			invItems, invErr := s.charSaver.LoadInventory(stream.Context(), characterID)
			if invErr != nil {
				s.logger.Warn("failed to load inventory on login",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(invErr),
				)
			} else {
				for _, it := range invItems {
					if _, addErr := sess.Backpack.Add(it.ItemDefID, it.Quantity, s.invRegistry); addErr != nil {
						s.logger.Warn("failed to restore inventory item",
							zap.String("item", it.ItemDefID),
							zap.Error(addErr),
						)
					}
				}
				// Apply persisted charge state to backpack instances (REQ-ACT-14).
				if charRepo, ok := s.charSaver.(*postgres.CharacterRepository); ok {
					if chargeMap, chargeErr := charRepo.LoadInstanceCharges(stream.Context(), characterID); chargeErr != nil {
						s.logger.Warn("failed to load instance charges on login",
							zap.String("uid", uid),
							zap.Int64("character_id", characterID),
							zap.Error(chargeErr),
						)
					} else {
						for instanceID, cs := range chargeMap {
							if inst := sess.Backpack.GetByInstanceID(instanceID); inst != nil {
								inst.ChargesRemaining = cs.ChargesRemaining
								inst.Expended = cs.Expended
							}
						}
					}
				}
			}

			// Load persisted currency.
			if savedCurrency, currErr := s.charSaver.LoadCurrency(stream.Context(), characterID); currErr != nil {
				s.logger.Warn("failed to load currency on login",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(currErr),
				)
			} else {
				sess.Currency = savedCurrency
			}
			// Hydrate quest state from DB (REQ-QU-14).
			if s.questSvc != nil {
				qRecords, qErr := s.questSvc.LoadQuests(stream.Context(), characterID)
				if qErr != nil {
					s.logger.Warn("failed to load quests", zap.Int64("characterID", characterID), zap.Error(qErr))
					sess.ActiveQuests = make(map[string]*quest.ActiveQuest)
					sess.CompletedQuests = make(map[string]*time.Time)
				} else {
					sess.ActiveQuests = make(map[string]*quest.ActiveQuest)
					sess.CompletedQuests = make(map[string]*time.Time)
					s.questSvc.HydrateSession(sess, qRecords)
				}
			} else {
				sess.ActiveQuests = make(map[string]*quest.ActiveQuest)
				sess.CompletedQuests = make(map[string]*time.Time)
			}
			// HeroPoints is loaded separately from AddPlayerOptions because it requires
			// a DB read that is only available at login, not at session construction time.
			if savedHP, hpErr := s.charSaver.LoadHeroPoints(stream.Context(), characterID); hpErr != nil {
				s.logger.Warn("failed to load hero points on login",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(hpErr),
				)
			} else {
				sess.HeroPoints = savedHP
			}
			// Load persisted jobs.
			if loadedJobs, loadedActiveJobID, jobsErr := s.charSaver.LoadJobs(stream.Context(), characterID); jobsErr != nil {
				s.logger.Warn("failed to load jobs on login",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(jobsErr),
				)
			} else {
				sess.Jobs = loadedJobs
				sess.ActiveJobID = loadedActiveJobID
			}
			// Compute JobTier and DowntimeQueueLimit from the active job (REQ-DTQ-13/14).
			if s.jobRegistry != nil && sess.ActiveJobID != "" {
				if activeJob, ok := s.jobRegistry.Job(sess.ActiveJobID); ok {
					sess.JobTier = activeJob.Tier
				}
			}
			if s.downtimeQueueLimitReg != nil {
				sess.DowntimeQueueLimit = s.downtimeQueueLimitReg.Lookup(sess.JobTier, sess.Level)
			}
			// Apply passive drawback conditions for all held jobs at login (REQ-JD-8).
			if s.jobRegistry != nil && sess.Conditions != nil {
				for _, loginJob := range s.resolveHeldJobs(sess) {
					for _, db := range loginJob.Drawbacks {
						if db.Type != "passive" || db.ConditionID == "" {
							continue
						}
						if !sess.Conditions.Has(db.ConditionID) {
							source := "drawback:" + loginJob.ID
							if def, ok := s.condRegistry.Get(db.ConditionID); ok {
								_ = sess.Conditions.ApplyTagged(uid, def, 1, -1, source)
							}
						}
					}
				}
			}
			// Grant starting kit on first login.
			if s.loadoutsDir != "" {
				received, flagErr := s.charSaver.HasReceivedStartingInventory(stream.Context(), characterID)
				if flagErr != nil {
					s.logger.Warn("failed to check starting inventory flag",
						zap.String("uid", uid),
						zap.Int64("character_id", characterID),
						zap.Error(flagErr),
					)
				} else if !received {
					// Resolve archetype from job registry — authoritative source regardless of
					// what the client sent. The web client sends char.Team ("gun"/"machete")
					// instead of the job's archetype ID ("aggressor"/"drifter"/etc.).
					archetype := joinReq.Archetype
					if s.jobRegistry != nil {
						if job, ok := s.jobRegistry.Job(sess.Class); ok && job.Archetype != "" {
							archetype = job.Archetype
						}
					}
					team := ""
					if s.jobRegistry != nil {
						team = s.jobRegistry.TeamFor(sess.Class)
					}
					var jobOverride *inventory.StartingLoadoutOverride
					if s.jobRegistry != nil {
						if job, ok := s.jobRegistry.Job(sess.Class); ok && job.StartingInventory != nil {
							jobOverride = job.StartingInventory
						}
					}
					if grantErr := s.grantStartingInventory(stream.Context(), sess, characterID, archetype, team, jobOverride); grantErr != nil {
						s.logger.Error("failed to grant starting inventory",
							zap.String("uid", uid),
							zap.Int64("character_id", characterID),
							zap.Error(grantErr),
						)
					} else {
						// Issue onboarding quest for new characters.
						s.issueOnboardingQuest(stream.Context(), uid, sess)
					}
				}
			}
		}
	}

	// Skill backfill: auto-assign skills for characters that have none recorded.
	// This covers characters created before the skills system existed.
	// No interactive choice is presented; fixed grants are applied and choice slots are left untrained.
	if characterID > 0 && s.characterSkillsRepo != nil && len(s.allSkills) > 0 && s.jobRegistry != nil {
		has, skillCheckErr := s.characterSkillsRepo.HasSkills(stream.Context(), characterID)
		if skillCheckErr != nil {
			s.logger.Warn("checking character skills for backfill",
				zap.Int64("character_id", characterID),
				zap.Error(skillCheckErr),
			)
		} else if !has {
			if job, ok := s.jobRegistry.Job(sess.Class); ok {
				allSkillIDs := make([]string, len(s.allSkills))
				for i, sk := range s.allSkills {
					allSkillIDs[i] = sk.ID
				}
				skillMap := character.BuildSkillsFromJob(job, allSkillIDs, nil)
				if setErr := s.characterSkillsRepo.SetAll(stream.Context(), characterID, skillMap); setErr != nil {
					s.logger.Error("backfilling character skills",
						zap.Int64("character_id", characterID),
						zap.Error(setErr),
					)
				} else {
					s.logger.Info("backfilled skills for character",
						zap.Int64("character_id", characterID),
						zap.String("class", sess.Class),
					)
				}
			}
		}
	}

	// Load skills into the session for runtime access (e.g. skill checks).
	// This runs after the backfill block above so the data is always present.
	if characterID > 0 && s.characterSkillsRepo != nil {
		skillMap, loadSkillsErr := s.characterSkillsRepo.GetAll(stream.Context(), characterID)
		if loadSkillsErr != nil {
			s.logger.Warn("loading character skills into session",
				zap.Int64("character_id", characterID),
				zap.Error(loadSkillsErr),
			)
		} else {
			if sess.Skills == nil {
				sess.Skills = make(map[string]string)
			}
			for id, rank := range skillMap {
				sess.Skills[id] = rank
			}
		}
	}

	// Proficiency backfill: assign job proficiencies for characters that have none recorded.
	// Always adds `unarmored: trained` (PF2E baseline for all characters).
	// Idempotent: safe to run on every login.
	if characterID > 0 && s.characterProficienciesRepo != nil && s.jobRegistry != nil {
		existing, profCheckErr := s.characterProficienciesRepo.GetAll(stream.Context(), characterID)
		if profCheckErr != nil {
			s.logger.Warn("checking character proficiencies for backfill",
				zap.Int64("character_id", characterID),
				zap.Error(profCheckErr),
			)
		} else {
			// Always ensure unarmored is trained; override with job proficiencies.
			profMap := map[string]string{"unarmored": "trained"}
			if job, ok := s.jobRegistry.Job(sess.Class); ok {
				for cat, rank := range job.Proficiencies {
					profMap[cat] = rank
				}
			}
			for cat, rank := range profMap {
				if _, alreadySet := existing[cat]; !alreadySet {
					if upsertErr := s.characterProficienciesRepo.Upsert(
						stream.Context(), characterID, cat, rank,
					); upsertErr != nil {
						s.logger.Error("upserting proficiency",
							zap.String("category", cat),
							zap.Error(upsertErr),
						)
					}
				}
			}
		}
		// Load proficiencies into session.
		profMap, loadProfErr := s.characterProficienciesRepo.GetAll(stream.Context(), characterID)
		if loadProfErr != nil {
			s.logger.Warn("loading character proficiencies into session",
				zap.Int64("character_id", characterID),
				zap.Error(loadProfErr),
			)
		} else {
			if sess.Proficiencies == nil {
				sess.Proficiencies = make(map[string]string)
			}
			for cat, rank := range profMap {
				sess.Proficiencies[cat] = rank
			}
		}
	}

	// Initialize out-of-combat conditions set for this session.
	sess.Conditions = condition.NewActiveSet()

	// Fetch class feature IDs once; reused for both passive-feat caching and
	// choice-resolution below to avoid a duplicate repository call.
	var cfIDs []string
	if characterID > 0 && s.characterClassFeaturesRepo != nil {
		var cfErr error
		cfIDs, cfErr = s.characterClassFeaturesRepo.GetAll(stream.Context(), characterID)
		if cfErr != nil {
			s.logger.Warn("loading class features", zap.Error(cfErr))
			cfIDs = nil
		}
	}

	// Populate passive feat cache from class features.
	sess.PassiveFeats = make(map[string]bool)
	if s.classFeatureRegistry != nil {
		for _, id := range cfIDs {
			cf, ok := s.classFeatureRegistry.ClassFeature(id)
			if ok && !cf.Active {
				sess.PassiveFeats[id] = true
			}
		}
	}
	// Populate passive feat cache from player feats (e.g. snap_shot).
	if characterID > 0 && s.characterFeatsRepo != nil && s.featRegistry != nil {
		pfIDs, pfErr := s.characterFeatsRepo.GetAll(stream.Context(), characterID)
		if pfErr != nil {
			s.logger.Warn("loading feats for passive feat cache", zap.Error(pfErr))
		} else {
			for _, id := range pfIDs {
				if f, ok := s.featRegistry.Feat(id); ok && !f.Active {
					sess.PassiveFeats[id] = true
				}
			}
		}
	}

	// Load and compute Focus Points (REQ-FP-1, REQ-FP-11).
	if characterID > 0 && s.charSaver != nil {
		fpFromDB, fpErr := s.charSaver.LoadFocusPoints(stream.Context(), characterID)
		if fpErr != nil {
			s.logger.Warn("failed to load focus points at login",
				zap.Int64("character_id", characterID),
				zap.Error(fpErr),
			)
		} else {
			grantCount := 0
			if s.classFeatureRegistry != nil {
				for _, id := range cfIDs {
					if cf, ok := s.classFeatureRegistry.ClassFeature(id); ok && cf.GrantsFocusPoint {
						grantCount++
					}
				}
			}
			if s.characterFeatsRepo != nil && s.featRegistry != nil {
				fpFeatIDs, fpFeatErr := s.characterFeatsRepo.GetAll(stream.Context(), characterID)
				if fpFeatErr != nil {
					s.logger.Warn("loading feats for focus point computation", zap.Error(fpFeatErr))
				} else {
					for _, id := range fpFeatIDs {
						if f, ok := s.featRegistry.Feat(id); ok && f.GrantsFocusPoint {
							grantCount++
						}
					}
				}
			}
			sess.MaxFocusPoints = focuspoints.ComputeMax(grantCount)
			sess.FocusPoints = focuspoints.Clamp(fpFromDB, sess.MaxFocusPoints)
		}
	}

	// Load material inventory (REQ-CRAFT-7)
	if s.materialRepo != nil && characterID > 0 {
		mats, err := s.materialRepo.Load(context.Background(), characterID)
		if err != nil {
			s.logger.Warn("loading materials", zap.Error(err))
			mats = make(map[string]int)
		}
		sess.Materials = mats
	} else {
		sess.Materials = make(map[string]int)
	}

	// Load stored feature choices (interactive prompting deferred until after initial room view).
	sess.FeatureChoices = make(map[string]map[string]string)
	if characterID > 0 && s.featureChoicesRepo != nil {
		stored, fcErr := s.featureChoicesRepo.GetAll(stream.Context(), characterID)
		if fcErr != nil {
			s.logger.Warn("loading feature choices", zap.Int64("character_id", characterID), zap.Error(fcErr))
		} else {
			sess.FeatureChoices = stored
		}
	}

	// REQ-DT-6: restore downtime state on reconnect
	s.restoreDowntimeState(stream.Context(), uid, sess, characterID)

	// Load hotbars from DB (REQ-HB-9, REQ-HB-11).
	if s.charSaver != nil && characterID > 0 {
		hotbarBars, hotbarActiveIdx, hotbarErr := s.charSaver.LoadHotbars(stream.Context(), characterID)
		if hotbarErr != nil {
			s.logger.Warn("loading hotbars",
				zap.Int64("character_id", characterID),
				zap.Error(hotbarErr),
			)
		}
		sess.Hotbars = hotbarBars
		sess.ActiveHotbarIndex = hotbarActiveIdx
	}

	// Broadcast arrival to other players in the room
	s.broadcastRoomEvent(spawnRoom.ID, uid, &gamev1.RoomEvent{
		Player: charName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send initial room view — this initializes the client's split-screen layout.
	// Feature choice prompts are sent after this so they appear in the console region.
	roomView := s.worldH.buildRoomView(uid, spawnRoom)
	if err := stream.Send(&gamev1.ServerEvent{
		RequestId: firstMsg.RequestId,
		Payload:   &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}); err != nil {
		return fmt.Errorf("sending initial room view: %w", err)
	}

	// Notify joining player of any active weather event.
	if s.weatherMgr != nil {
		if name, desc := s.weatherMgr.ActiveWeather(); name != "" {
			_ = stream.Send(&gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Weather{
					Weather: &gamev1.WeatherEvent{
						WeatherName: name,
						Active:      true,
						Description: desc,
					},
				},
			})
		}
	}

	// Resolve any missing feature/feat choices interactively now that split-screen is active.
	// promptFeatureChoice blocks on stream.Recv(); no entity/clock goroutines are running yet
	// so there are no concurrent stream.Send calls during this phase.
	if characterID > 0 && s.featureChoicesRepo != nil {
		if s.classFeatureRegistry != nil {
			for _, id := range cfIDs {
				cf, ok := s.classFeatureRegistry.ClassFeature(id)
				if !ok || cf.Choices == nil {
					continue
				}
				if sess.FeatureChoices[id] != nil && sess.FeatureChoices[id][cf.Choices.Key] != "" {
					continue
				}
				chosen, promptErr := s.promptFeatureChoice(stream, id, cf.Choices, sess.Headless)
				if promptErr != nil {
					s.logger.Warn("prompting feature choice", zap.String("feature", id), zap.Error(promptErr))
					continue
				}
				if chosen == "" {
					continue
				}
				if setErr := s.featureChoicesRepo.Set(stream.Context(), characterID, id, cf.Choices.Key, chosen); setErr != nil {
					s.logger.Warn("persisting feature choice", zap.String("feature", id), zap.Error(setErr))
					continue
				}
				if sess.FeatureChoices[id] == nil {
					sess.FeatureChoices[id] = make(map[string]string)
				}
				sess.FeatureChoices[id][cf.Choices.Key] = chosen
			}
		}

		if s.characterFeatsRepo != nil && s.featRegistry != nil {
			featIDs, featErr := s.characterFeatsRepo.GetAll(stream.Context(), characterID)
			if featErr != nil {
				s.logger.Warn("loading feats for choice resolution", zap.Error(featErr))
			} else {
				for _, id := range featIDs {
					f, ok := s.featRegistry.Feat(id)
					if !ok || f.Choices == nil {
						continue
					}
					if sess.FeatureChoices[id] != nil && sess.FeatureChoices[id][f.Choices.Key] != "" {
						continue
					}
					chosen, promptErr := s.promptFeatureChoice(stream, id, f.Choices, sess.Headless)
					if promptErr != nil {
						s.logger.Warn("prompting feat choice", zap.String("feat", id), zap.Error(promptErr))
						continue
					}
					if chosen == "" {
						continue
					}
					if setErr := s.featureChoicesRepo.Set(stream.Context(), characterID, id, f.Choices.Key, chosen); setErr != nil {
						s.logger.Warn("persisting feat choice", zap.String("feat", id), zap.Error(setErr))
						continue
					}
					if sess.FeatureChoices[id] == nil {
						sess.FeatureChoices[id] = make(map[string]string)
					}
					sess.FeatureChoices[id][f.Choices.Key] = chosen
				}
			}
		}

		// Populate derived fields from FeatureChoices.
		// Backward-compat: FavoredTarget is still read by combat code; derive from generic choices store.
		sess.FavoredTargetMu.Lock()
		sess.FavoredTarget = sess.FeatureChoices["predators_eye"]["favored_target"]
		sess.FavoredTargetMu.Unlock()

		// Apply armor_training feat choice as a real proficiency.
		// If the player has the armor_training feat and has made a category choice,
		// ensure that category is persisted and loaded into the session.
		if sess.Proficiencies == nil {
			sess.Proficiencies = make(map[string]string)
		}
		applyArmorTrainingProficiency(
			stream.Context(), characterID, sess.FeatureChoices,
			s.characterProficienciesRepo, sess.Proficiencies, s.logger,
		)
	}

	// Resolve missing ability boost choices.
	if s.charAbilityBoostsRepo != nil && s.archetypes != nil && s.regions != nil {
		storedBoosts, boostErr := s.charAbilityBoostsRepo.GetAll(stream.Context(), characterID)
		skipBoostPrompts := false
		if boostErr != nil {
			s.logger.Warn("loading ability boosts, skipping prompt", zap.Int64("character_id", characterID), zap.Error(boostErr))
			storedBoosts = map[string][]string{}
			skipBoostPrompts = true
		}

		// Determine archetype from job.
		archetypeID := ""
		if s.jobRegistry != nil {
			if job, ok := s.jobRegistry.Job(sess.Class); ok {
				archetypeID = job.Archetype
			}
		}

		// Prompt for archetype free boosts.
		if !skipBoostPrompts && archetypeID != "" {
			if archetype, ok := s.archetypes[archetypeID]; ok && archetype.AbilityBoosts != nil {
				chosenForArchetype := storedBoosts["archetype"]
				needed := archetype.AbilityBoosts.Free - len(chosenForArchetype)
				for i := 0; i < needed; i++ {
					pool := character.AbilityBoostPool(archetype.AbilityBoosts.Fixed, chosenForArchetype)
					if len(pool) == 0 {
						break
					}
					choices := &ruleset.FeatureChoices{
						Prompt:  fmt.Sprintf("Choose archetype free ability boost %d of %d:", len(chosenForArchetype)+1, archetype.AbilityBoosts.Free),
						Options: pool,
						Key:     fmt.Sprintf("archetype_boost_%d", i),
					}
					chosen, promptErr := s.promptFeatureChoice(stream, "archetype_boost", choices, sess.Headless)
					if promptErr != nil || chosen == "" {
						break
					}
					if addErr := s.charAbilityBoostsRepo.Add(stream.Context(), characterID, "archetype", chosen); addErr != nil {
						s.logger.Warn("persisting archetype boost", zap.Error(addErr))
					}
					chosenForArchetype = append(chosenForArchetype, chosen)
				}
				storedBoosts["archetype"] = chosenForArchetype
			}
		}

		// Prompt for region free boost.
		if !skipBoostPrompts && dbChar != nil {
			if region, ok := s.regions[dbChar.Region]; ok && region.AbilityBoosts != nil {
				chosenForRegion := storedBoosts["region"]
				needed := region.AbilityBoosts.Free - len(chosenForRegion)
				for i := 0; i < needed; i++ {
					pool := character.AbilityBoostPool(region.AbilityBoosts.Fixed, chosenForRegion)
					if len(pool) == 0 {
						break
					}
					choices := &ruleset.FeatureChoices{
						Prompt:  fmt.Sprintf("Choose region free ability boost %d of %d:", len(chosenForRegion)+1, region.AbilityBoosts.Free),
						Options: pool,
						Key:     fmt.Sprintf("region_boost_%d", i),
					}
					chosen, promptErr := s.promptFeatureChoice(stream, "region_boost", choices, sess.Headless)
					if promptErr != nil || chosen == "" {
						break
					}
					if addErr := s.charAbilityBoostsRepo.Add(stream.Context(), characterID, "region", chosen); addErr != nil {
						s.logger.Warn("persisting region boost", zap.Error(addErr))
					}
					chosenForRegion = append(chosenForRegion, chosen)
				}
				storedBoosts["region"] = chosenForRegion
			}
		}

		// Recompute scores from scratch, apply all boosts, persist.
		if dbChar != nil {
			baseScores := recomputeBaseScores(dbChar, s.regions, s.jobRegistry)
			archetypeBoosts := (*ruleset.AbilityBoostGrant)(nil)
			if archetypeID != "" {
				if archetype, ok := s.archetypes[archetypeID]; ok {
					archetypeBoosts = archetype.AbilityBoosts
				}
			}
			regionBoosts := (*ruleset.AbilityBoostGrant)(nil)
			if region, ok := s.regions[dbChar.Region]; ok {
				regionBoosts = region.AbilityBoosts
			}
			newAbilities := character.ApplyAbilityBoosts(baseScores, archetypeBoosts, storedBoosts["archetype"], regionBoosts, storedBoosts["region"])
			sess.Abilities = newAbilities
			if saveErr := s.charSaver.SaveAbilities(stream.Context(), characterID, newAbilities); saveErr != nil {
				s.logger.Warn("saving ability scores to DB", zap.Error(saveErr))
			}
		}
	}

	// Assign technologies at character creation (only if no slots have ever been persisted).
	// REQ-TG-BACKFILL1: Check all four tech repo types so that jobs with no hardwired techs
	// (prepared/spontaneous/innate only) are not re-assigned on every subsequent login.
	if s.hardwiredTechRepo != nil && s.jobRegistry != nil && characterID > 0 {
		existingHW, hwCheckErr := s.hardwiredTechRepo.GetAll(stream.Context(), characterID)
		if hwCheckErr != nil {
			s.logger.Warn("checking existing hardwired technologies", zap.Int64("character_id", characterID), zap.Error(hwCheckErr))
		} else {
			alreadyAssigned := len(existingHW) > 0
			if !alreadyAssigned && s.preparedTechRepo != nil {
				existingPrep, prepCheckErr := s.preparedTechRepo.GetAll(stream.Context(), characterID)
				if prepCheckErr != nil {
					s.logger.Warn("checking existing prepared technologies", zap.Int64("character_id", characterID), zap.Error(prepCheckErr))
				} else if len(existingPrep) > 0 {
					alreadyAssigned = true
				}
			}
			if !alreadyAssigned && s.knownTechRepo != nil {
				existingSpont, spontCheckErr := s.knownTechRepo.GetAll(stream.Context(), characterID)
				if spontCheckErr != nil {
					s.logger.Warn("checking existing spontaneous technologies", zap.Int64("character_id", characterID), zap.Error(spontCheckErr))
				} else if len(existingSpont) > 0 {
					alreadyAssigned = true
				}
			}
			if !alreadyAssigned && s.innateTechRepo != nil {
				existingInnate, innateCheckErr := s.innateTechRepo.GetAll(stream.Context(), characterID)
				if innateCheckErr != nil {
					s.logger.Warn("checking existing innate technologies", zap.Int64("character_id", characterID), zap.Error(innateCheckErr))
				} else if len(existingInnate) > 0 {
					alreadyAssigned = true
				}
			}
			if !alreadyAssigned {
				if job, ok := s.jobRegistry.Job(sess.Class); ok {
					var archetype *ruleset.Archetype
					if archetypeID := job.Archetype; archetypeID != "" {
						archetype = s.archetypes[archetypeID]
					}
					headless := sess.Headless
					promptFn := func(prompt string, options []string, slotCtx *TechSlotContext) (string, error) {
						choices := &ruleset.FeatureChoices{
							Prompt:  prompt,
							Options: options,
							Key:     "tech_choice",
						}
						return s.promptFeatureChoice(stream, "tech_choice", choices, headless)
					}
					var region *ruleset.Region
					if dbChar != nil {
						region = s.regions[dbChar.Region]
					}
					if assignErr := AssignTechnologies(stream.Context(), sess, characterID,
						job, archetype, s.techRegistry, promptFn,
						s.hardwiredTechRepo, s.preparedTechRepo, s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
						region,
					); assignErr != nil {
						s.logger.Warn("assigning technologies", zap.Int64("character_id", characterID), zap.Error(assignErr))
					}
				}
			}
		}
	}

	// Load pending tech levels from DB and reconstruct PendingTechGrants.
	// Uses merged archetype+job grants (not job-only) so archetype level_up_grants are included.
	if sess.CharacterID > 0 && s.progressRepo != nil && s.jobRegistry != nil {
		if pendingLevels, err := s.progressRepo.GetPendingTechLevels(stream.Context(), sess.CharacterID); err == nil {
			if len(pendingLevels) > 0 {
				if job, ok := s.jobRegistry.Job(sess.Class); ok {
					var archetypeLUG map[int]*ruleset.TechnologyGrants
					if job.Archetype != "" {
						if arch, aOK := s.archetypes[job.Archetype]; aOK {
							archetypeLUG = arch.LevelUpGrants
						}
					}
					mergedLUG := ruleset.MergeLevelUpGrants(archetypeLUG, job.LevelUpGrants)
					if sess.PendingTechGrants == nil {
						sess.PendingTechGrants = make(map[int]*ruleset.TechnologyGrants)
					}
					for _, lvl := range pendingLevels {
						if grants, gOK := mergedLUG[lvl]; gOK && grants != nil {
							_, deferred := PartitionTechGrants(grants)
							if deferred != nil {
								sess.PendingTechGrants[lvl] = deferred
							}
						}
					}
					// REQ-TTA-7: Re-issue find-trainer quests for any pending grants loaded
					// from DB, in case the quest was never issued (e.g. first login after
					// NPC was placed in zone) or the player abandoned it.
					for _, deferred := range sess.PendingTechGrants {
						s.issueTechTrainerQuests(stream.Context(), sess, 0, deferred)
					}
				}
			}
		} else {
			s.logger.Warn("Session: GetPendingTechLevels failed", zap.Error(err))
		}
	}

	// Load persisted technology assignments for this session.
	if s.hardwiredTechRepo != nil && characterID > 0 {
		if techErr := LoadTechnologies(stream.Context(), sess, characterID,
			s.hardwiredTechRepo, s.preparedTechRepo, s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
		); techErr != nil {
			s.logger.Warn("loading technologies", zap.Int64("character_id", characterID), zap.Error(techErr))
		}
	}

	// Resolve and cache the casting model for this character's job+archetype combination.
	// Used throughout the session to select the correct tech pool and catalog population logic.
	if s.jobRegistry != nil && sess.Class != "" {
		if loginJob, loginJobOK := s.jobRegistry.Job(sess.Class); loginJobOK {
			var loginArchetype *ruleset.Archetype
			if loginJob.Archetype != "" {
				loginArchetype = s.archetypes[loginJob.Archetype]
			}
			sess.CastingModel = ruleset.ResolveCastingModel(loginJob, loginArchetype)
		}
	}

	// Backfill KnownTechs from PreparedTechs for wizard/ranger characters created before the
	// KnownTechs catalog feature was introduced. Characters that have PreparedTechs but an empty
	// KnownTechs table would otherwise see no options during rest rearrangement.
	// This is idempotent: the INSERT uses ON CONFLICT DO NOTHING.
	if s.knownTechRepo != nil && characterID > 0 &&
		(sess.CastingModel == ruleset.CastingModelWizard || sess.CastingModel == ruleset.CastingModelRanger) &&
		len(sess.KnownTechs) == 0 && len(sess.PreparedTechs) > 0 {
		if sess.KnownTechs == nil {
			sess.KnownTechs = make(map[int][]string)
		}
		for lvl, slots := range sess.PreparedTechs {
			for _, slot := range slots {
				if slot == nil {
					continue
				}
				if !containsString(sess.KnownTechs[lvl], slot.TechID) {
					sess.KnownTechs[lvl] = append(sess.KnownTechs[lvl], slot.TechID)
				}
				if addErr := s.knownTechRepo.Add(stream.Context(), characterID, slot.TechID, lvl); addErr != nil {
					s.logger.Warn("KnownTechs backfill: Add failed",
						zap.Int64("character_id", characterID),
						zap.String("tech_id", slot.TechID),
						zap.Int("level", lvl),
						zap.Error(addErr),
					)
				}
			}
		}
		s.logger.Info("KnownTechs backfilled from PreparedTechs",
			zap.Int64("character_id", characterID),
			zap.String("casting_model", string(sess.CastingModel)),
		)
	}

	// Retroactively backfill any technology level-up grants that were never applied.
	// This covers characters created before the archetype level_up_grants bug was fixed.
	// BackfillLevelUpTechnologies is idempotent: only missing grants (delta > 0) are applied.
	if s.hardwiredTechRepo != nil && s.preparedTechRepo != nil && s.jobRegistry != nil && characterID > 0 && sess.Level >= 2 {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			var archetype *ruleset.Archetype
			if job.Archetype != "" {
				archetype = s.archetypes[job.Archetype]
			}
			var archetypeLUG map[int]*ruleset.TechnologyGrants
			if archetype != nil {
				archetypeLUG = archetype.LevelUpGrants
			}
			mergedLUG := ruleset.MergeLevelUpGrants(archetypeLUG, job.LevelUpGrants)
			pendingFromBackfill, backfillErr := BackfillLevelUpTechnologies(stream.Context(), sess, characterID,
				job, archetype, mergedLUG, s.techRegistry,
				s.hardwiredTechRepo, s.preparedTechRepo, s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
			)
			if backfillErr != nil {
				s.logger.Warn("BackfillLevelUpTechnologies failed",
					zap.Int64("character_id", characterID),
					zap.Error(backfillErr),
				)
			}
			// REQ-TTA-2: Register any cleared/missing L2+ prepared slots as pending trainer grants
			// and issue the corresponding find-trainer quests.
			if pendingFromBackfill != nil && characterID > 0 {
				ctx := stream.Context()
				if sess.PendingTechGrants == nil {
					sess.PendingTechGrants = make(map[int]*ruleset.TechnologyGrants)
				}
				// Use sess.Level as the key because we don't know the exact character level
				// at which each tech level was granted. The trainer resolution uses the
				// grants' SlotsByLevel to determine what to assign, so the key only needs
				// to be unique and representative.
				const backfillCharLevel = 0 // sentinel: backfill-recovered pending grants
				sess.PendingTechGrants[backfillCharLevel] = pendingFromBackfill
				if s.progressRepo != nil {
					levels := make([]int, 0, len(sess.PendingTechGrants))
					for lvl := range sess.PendingTechGrants {
						levels = append(levels, lvl)
					}
					sort.Ints(levels)
					if err := s.progressRepo.SetPendingTechLevels(ctx, characterID, levels); err != nil {
						s.logger.Warn("BackfillLevelUpTechnologies: SetPendingTechLevels failed", zap.Error(err))
					}
				}
				if s.pendingTechSlotsRepo != nil && pendingFromBackfill.Prepared != nil {
					for techLvl, slots := range pendingFromBackfill.Prepared.SlotsByLevel {
						tradition := ""
						for _, e := range pendingFromBackfill.Prepared.Pool {
							if e.Level == techLvl && s.techRegistry != nil {
								if def, defOK := s.techRegistry.Get(e.ID); defOK {
									tradition = string(def.Tradition)
									break
								}
							}
						}
						for i := 0; i < slots; i++ {
							if err := s.pendingTechSlotsRepo.AddPendingTechSlot(
								ctx, characterID, backfillCharLevel, techLvl, tradition, "prepared",
							); err != nil {
								s.logger.Warn("BackfillLevelUpTechnologies: AddPendingTechSlot failed", zap.Error(err))
							}
						}
					}
				}
				s.issueTechTrainerQuests(ctx, sess, backfillCharLevel, pendingFromBackfill)
			}
		}
	}

	// Resolve any pending technology selections interactively before the command loop.
	// Like ability-boost prompts, this blocks on stream.Recv() so it must run before
	// entity/clock goroutines are spawned.
	if len(sess.PendingTechGrants) > 0 && s.jobRegistry != nil {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			headless := sess.Headless
			promptFn := func(prompt string, options []string, slotCtx *TechSlotContext) (string, error) {
				choices := &ruleset.FeatureChoices{
					Prompt:  prompt,
					Options: options,
					Key:     "tech_choice",
				}
				return s.promptFeatureChoice(stream, "tech_choice", choices, headless)
			}
			if err := ResolvePendingTechGrants(stream.Context(), sess, characterID,
				job, s.techRegistry, promptFn,
				s.hardwiredTechRepo, s.preparedTechRepo,
				s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
				s.progressRepo,
			); err != nil {
				s.logger.Warn("Session: ResolvePendingTechGrants failed", zap.Error(err))
			}
		}
	}

	// Retroactively backfill any feat level-up grants never applied.
	// Runs before feat loading so newly-granted feats are included in session state.
	// BackfillLevelUpFeats is idempotent: only missing grants are applied.
	if s.characterFeatsRepo != nil && s.jobRegistry != nil && characterID > 0 && sess.Level >= 2 {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			var archFeatGrants map[int]*ruleset.FeatGrants
			if job.Archetype != "" {
				if arch, archOK := s.archetypes[job.Archetype]; archOK {
					archFeatGrants = arch.LevelUpFeatGrants
				}
			}
			mergedFeatGrants := ruleset.MergeFeatLevelUpGrants(archFeatGrants, job.LevelUpFeatGrants)
			if err := BackfillLevelUpFeats(stream.Context(), sess, characterID,
				mergedFeatGrants, s.featRegistry, s.characterFeatsRepo, s.featLevelGrantsRepo,
			); err != nil {
				s.logger.Warn("BackfillLevelUpFeats failed",
					zap.Int64("character_id", characterID),
					zap.Error(err),
				)
			}
		}
	}

	// REQ-RXN15: register reactions from feats; also populate ActiveFeatUses for limited active feats.
	if s.characterFeatsRepo != nil && s.featRegistry != nil && characterID > 0 {
		if featIDs, featErr := s.characterFeatsRepo.GetAll(stream.Context(), characterID); featErr != nil {
			s.logger.Warn("Session: failed to load character feats for reaction registration", zap.Error(featErr))
		} else {
			for _, id := range featIDs {
				f, ok := s.featRegistry.Feat(id)
				if !ok {
					continue
				}
				if f.Reaction != nil {
					sess.Reactions.Register(uid, id, f.Name, *f.Reaction)
				}
				if f.Active && f.PreparedUses > 0 {
					if sess.ActiveFeatUses == nil {
						sess.ActiveFeatUses = make(map[string]int)
					}
					sess.ActiveFeatUses[id] = f.PreparedUses
				}
			}
		}
	}

	// REQ-RXN15: register reactions from innate techs.
	if s.techRegistry != nil {
		for techID := range sess.InnateTechs {
			if techDef, ok := s.techRegistry.Get(techID); ok && techDef.Reaction != nil {
				sess.Reactions.Register(uid, techID, techDef.Name, *techDef.Reaction)
			}
		}
	}

	// Send initial HotbarUpdateEvent after LoadTechnologies and ActiveFeatUses initialization so
	// use counts (InnateTechs, PreparedTechs, SpontaneousUsePools, ActiveFeatUses) are populated.
	// REQ-HB-12: web client renders the hotbar row on receipt of this event.
	if err := stream.Send(s.hotbarUpdateEvent(sess)); err != nil {
		s.logger.Warn("failed to send initial hotbar update", zap.Error(err))
	}

	// Send game config so the web client receives server-side configuration. (REQ-CNT-2)
	if err := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_GameConfig{
			GameConfig: &gamev1.GameConfig{
				AutoNavStepMs: int32(s.autoNavStepMs),
			},
		},
	}); err != nil {
		s.logger.Warn("failed to send game config", zap.Error(err))
	}

	// Wrap the stream in a mutex-protected sender so that forwardEvents,
	// the calendar goroutine, and commandLoop can all call Send concurrently
	// without data races.  Recv and other stream methods are not affected.
	ss := &syncStream{GameService_SessionServer: stream}

	// REQ-RXN20: build and store the interactive reaction callback.
	// Must use ss so that reaction prompts go through the mutex.
	sess.ReactionFn = s.buildReactionCallback(uid, sess)

	// Subscribe to calendar ticks for this session (nil-safe: calendar may be nil).
	var calCh chan GameDateTime
	if s.calendar != nil {
		calCh = make(chan GameDateTime, 2)
		s.calendar.Subscribe(calCh)
		defer s.calendar.Unsubscribe(calCh)
	}

	// Step 3: Spawn goroutine to forward entity events to stream
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.forwardEvents(ctx, sess.Entity, ss)
	}()

	// Spawn goroutine to forward calendar ticks to stream as TimeOfDayEvents.
	// When the period changes (e.g., Dawn→Morning), also push an updated RoomView
	// through the entity channel so the room display reflects the new flavor text.
	// Only launched when a calendar is configured.
	if calCh != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastPeriod := TimePeriod(roomView.GetPeriod())
			for {
				select {
				case dt, ok := <-calCh:
					if !ok {
						return
					}
					period := dt.Hour.Period()
					evt := &gamev1.ServerEvent{
						Payload: &gamev1.ServerEvent_TimeOfDay{
							TimeOfDay: &gamev1.TimeOfDayEvent{
								Hour:   int32(dt.Hour),
								Period: string(period),
								Day:    int32(dt.Day),
								Month:  int32(dt.Month),
							},
						},
					}
					if err := ss.Send(evt); err != nil {
						return
					}
					// Expire time-limited conditions for this player on each calendar tick (REQ-JD-11).
					if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil {
						sess.Conditions.TickCalendar(uid, time.Now())
					}
					// On period change, push an updated RoomView so the room
					// display reflects the new time-of-day flavor text.
					if period != lastPeriod {
						lastPeriod = period
						if sess, ok := s.sessions.GetPlayer(uid); ok {
							if room, ok := s.world.GetRoom(sess.RoomID); ok {
								rv := s.worldH.buildRoomView(uid, room)
								rvEvt := &gamev1.ServerEvent{
									Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
								}
								if data, err := proto.Marshal(rvEvt); err == nil {
									_ = sess.Entity.Push(data)
								}
							}
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Periodic room-view refresh: push a fresh RoomView to the player every 5s
	// so NPC movement and combat status stay current without requiring look.
	wg.Add(1)
	go func() {
		defer wg.Done()
		roomRefreshTicker := time.NewTicker(5 * time.Second)
		defer roomRefreshTicker.Stop()
		for {
			select {
			case <-roomRefreshTicker.C:
				if sess, ok := s.sessions.GetPlayer(uid); ok {
					s.pushRoomViewToAllInRoom(sess.RoomID)
				}
				s.tickSubstances(uid)     // REQ-AH-12
				s.tickAmbientSubstances() // REQ-OCF-8
				// REQ-DT-7: check all sessions with active downtime
				for _, activeUID := range s.sessions.AllUIDs() {
					s.checkDowntimeCompletion(activeUID)
					s.checkCampingStatus(activeUID) // REQ-REST-15/18
					s.checkRefocusStatus(activeUID) // REQ-EXP-REFOCUS-1
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Signal that all session-initialization writes are complete.
	// Tests and other consumers that need a race-free snapshot of any
	// PlayerSession field MUST wait on sess.InitDone before reading.
	close(sess.InitDone)

	// Step 4: Main command loop
	err = s.commandLoop(ctx, uid, ss)

	// Step 5: Cleanup happens via defer
	cancel()
	wg.Wait()

	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

// commandLoop processes incoming ClientMessages until the stream ends.
func (s *GameServiceServer) commandLoop(ctx context.Context, uid string, stream gamev1.GameService_SessionServer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			return io.EOF
		}
		if err != nil {
			return fmt.Errorf("receiving message: %w", err)
		}

		// status is a personal query: all ConditionEvents are sent directly on the stream.
		if _, ok := msg.Payload.(*gamev1.ClientMessage_Status); ok {
			if err := s.handleStatus(uid, msg.RequestId, stream); err != nil {
				errEvt := &gamev1.ServerEvent{
					RequestId: msg.RequestId,
					Payload: &gamev1.ServerEvent_Error{
						Error: &gamev1.ErrorEvent{Message: err.Error()},
					},
				}
				if sendErr := stream.Send(errEvt); sendErr != nil {
					return fmt.Errorf("sending error: %w", sendErr)
				}
			}
			continue
		}

		// rest requires direct stream access for interactive tech prompts.
		if _, ok := msg.Payload.(*gamev1.ClientMessage_Rest); ok {
			if err := s.handleRest(uid, msg.RequestId, stream); err != nil {
				errEvt := &gamev1.ServerEvent{
					RequestId: msg.RequestId,
					Payload: &gamev1.ServerEvent_Error{
						Error: &gamev1.ErrorEvent{Message: err.Error()},
					},
				}
				if sendErr := stream.Send(errEvt); sendErr != nil {
					return fmt.Errorf("sending error: %w", sendErr)
				}
			}
			continue
		}

		// Pre-dispatch: intercept UseRequest for spontaneous techs with AmpedEffects.
		if p, ok := msg.Payload.(*gamev1.ClientMessage_UseRequest); ok {
			techID := p.UseRequest.GetFeatId()
			if s.techRegistry != nil {
				if techDef, found := s.techRegistry.Get(techID); found &&
					techDef.UsageType == technology.UsageSpontaneous &&
					techDef.AmpedLevel > 0 {
					resp, err := s.handleAmpedUse(uid, p.UseRequest, stream)
					if err != nil {
						errEvt := &gamev1.ServerEvent{
							RequestId: msg.RequestId,
							Payload: &gamev1.ServerEvent_Error{
								Error: &gamev1.ErrorEvent{Message: err.Error()},
							},
						}
						if sendErr := stream.Send(errEvt); sendErr != nil {
							return fmt.Errorf("sending amped-use error: %w", sendErr)
						}
						continue
					}
					if resp != nil {
						resp.RequestId = msg.RequestId
						if sendErr := stream.Send(resp); sendErr != nil {
							return sendErr
						}
					}
					continue
				}
			}
		}

		if _, ok := msg.Payload.(*gamev1.ClientMessage_SelectTech); ok {
			if err := s.handleSelectTech(uid, msg.RequestId, stream); err != nil {
				s.logger.Warn("handleSelectTech error", zap.String("uid", uid), zap.Error(err))
			}
			continue
		}

		resp, err := s.dispatch(uid, msg)
		if err == errQuit {
			// Send Disconnected event then exit cleanly.
			if resp != nil {
				resp.RequestId = msg.RequestId
				_ = stream.Send(resp)
			}
			return nil
		}
		if err != nil {
			// Send error event
			errEvt := &gamev1.ServerEvent{
				RequestId: msg.RequestId,
				Payload: &gamev1.ServerEvent_Error{
					Error: &gamev1.ErrorEvent{Message: err.Error()},
				},
			}
			if sendErr := stream.Send(errEvt); sendErr != nil {
				return fmt.Errorf("sending error: %w", sendErr)
			}
			continue
		}

		if resp != nil {
			resp.RequestId = msg.RequestId
			if err := stream.Send(resp); err != nil {
				return fmt.Errorf("sending response: %w", err)
			}
		}
	}
}

// dispatch routes a ClientMessage to the appropriate handler.
func (s *GameServiceServer) dispatch(uid string, msg *gamev1.ClientMessage) (*gamev1.ServerEvent, error) {
	// Check whether any in-flight detention has expired before processing the command.
	if sess, ok := s.sessions.GetPlayer(uid); ok {
		s.checkDetentionCompletion(sess)
	}

	// Apply out-of-combat drowning damage on every dispatch while submerged.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && sess.Conditions.Has("submerged") && sess.Status != statusInCombat {
		dmgResult, _ := s.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		s.checkNonCombatDeath(uid, sess) // REQ-BUG100-1
	}

	// DETAINED: block action commands but permit informational queries.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && condition.IsCommandsPrevented(sess.Conditions) {
		switch msg.Payload.(type) {
		case *gamev1.ClientMessage_Look,
			*gamev1.ClientMessage_Exits,
			*gamev1.ClientMessage_Who,
			*gamev1.ClientMessage_Quit,
			*gamev1.ClientMessage_SwitchCharacter,
			*gamev1.ClientMessage_CharSheet:
			// Informational commands are permitted while detained.
		default:
			return messageEvent("You are detained and cannot act."), nil
		}
	}

	switch p := msg.Payload.(type) {
	case *gamev1.ClientMessage_Move:
		return s.handleMove(uid, p.Move)
	case *gamev1.ClientMessage_Look:
		return s.handleLook(uid)
	case *gamev1.ClientMessage_Exits:
		return s.handleExits(uid)
	case *gamev1.ClientMessage_Say:
		return s.handleSay(uid, p.Say)
	case *gamev1.ClientMessage_Emote:
		return s.handleEmote(uid, p.Emote)
	case *gamev1.ClientMessage_Who:
		return s.handleWho(uid)
	case *gamev1.ClientMessage_Quit:
		return s.handleQuit(uid)
	case *gamev1.ClientMessage_SwitchCharacter:
		return s.handleSwitch(uid)
	case *gamev1.ClientMessage_Examine:
		return s.handleExamine(uid, p.Examine)
	case *gamev1.ClientMessage_Attack:
		return s.handleAttack(uid, p.Attack)
	case *gamev1.ClientMessage_Flee:
		return s.handleFlee(uid)
	case *gamev1.ClientMessage_Pass:
		return s.handlePass(uid)
	case *gamev1.ClientMessage_Strike:
		return s.handleStrike(uid, p.Strike)
	case *gamev1.ClientMessage_Equip:
		return s.handleEquip(uid, p.Equip)
	case *gamev1.ClientMessage_Reload:
		return s.handleReload(uid, p.Reload)
	case *gamev1.ClientMessage_FireBurst:
		return s.handleFireBurst(uid, p.FireBurst)
	case *gamev1.ClientMessage_FireAutomatic:
		return s.handleFireAutomatic(uid, p.FireAutomatic)
	case *gamev1.ClientMessage_Throw:
		return s.handleThrow(uid, p.Throw)
	case *gamev1.ClientMessage_InventoryReq:
		return s.handleInventory(uid)
	case *gamev1.ClientMessage_GetItem:
		return s.handleGetItem(uid, p.GetItem.Target)
	case *gamev1.ClientMessage_DropItem:
		return s.handleDropItem(uid, p.DropItem.Target)
	case *gamev1.ClientMessage_Balance:
		return s.handleBalance(uid)
	case *gamev1.ClientMessage_SetRole:
		return s.handleSetRole(uid, p.SetRole)
	case *gamev1.ClientMessage_Teleport:
		return s.handleTeleport(uid, p.Teleport)
	case *gamev1.ClientMessage_Loadout:
		return s.handleLoadout(uid, p.Loadout)
	case *gamev1.ClientMessage_Unequip:
		return s.handleUnequip(uid, p.Unequip)
	case *gamev1.ClientMessage_Equipment:
		return s.handleEquipment(uid, p.Equipment)
	case *gamev1.ClientMessage_Wear:
		return s.handleWear(uid, p.Wear)
	case *gamev1.ClientMessage_RemoveArmor:
		return s.handleRemoveArmor(uid, p.RemoveArmor)
	case *gamev1.ClientMessage_CharSheet:
		return s.handleChar(uid)
	case *gamev1.ClientMessage_ArchetypeSelection:
		return s.handleArchetypeSelection(uid, p.ArchetypeSelection)
	case *gamev1.ClientMessage_UseEquipment:
		return s.handleUseEquipment(uid, p.UseEquipment.InstanceId)
	case *gamev1.ClientMessage_RoomEquip:
		return s.handleRoomEquip(uid, p.RoomEquip)
	case *gamev1.ClientMessage_Map:
		return s.handleMap(uid, p.Map)
	case *gamev1.ClientMessage_SkillsRequest:
		return s.handleSkills(uid)
	case *gamev1.ClientMessage_FeatsRequest:
		return s.handleFeats(uid)
	case *gamev1.ClientMessage_ClassFeaturesRequest:
		return s.handleClassFeatures(uid)
	case *gamev1.ClientMessage_InteractRequest:
		return s.handleInteract(uid, p.InteractRequest.InstanceId)
	case *gamev1.ClientMessage_UseRequest:
		return s.handleUse(uid, p.UseRequest.FeatId, p.UseRequest.GetTarget(), p.UseRequest.GetTargetX(), p.UseRequest.GetTargetY())
	case *gamev1.ClientMessage_Action:
		return s.handleAction(uid, p.Action)
	case *gamev1.ClientMessage_RaiseShield:
		return s.handleRaiseShield(uid)
	case *gamev1.ClientMessage_TakeCover:
		return s.handleTakeCover(uid)
	case *gamev1.ClientMessage_FirstAid:
		return s.handleFirstAid(uid)
	case *gamev1.ClientMessage_Feint:
		return s.handleFeint(uid, p.Feint)
	case *gamev1.ClientMessage_Demoralize:
		return s.handleDemoralize(uid, p.Demoralize)
	case *gamev1.ClientMessage_Grapple:
		return s.handleGrapple(uid, p.Grapple)
	case *gamev1.ClientMessage_Trip:
		return s.handleTrip(uid, p.Trip)
	case *gamev1.ClientMessage_Delay:
		return s.handleDelay(uid, p.Delay)
	case *gamev1.ClientMessage_Aid:
		return s.handleAid(uid, p.Aid)
	case *gamev1.ClientMessage_DisarmTrap:
		return s.handleDisarmTrap(uid, p.DisarmTrap)
	case *gamev1.ClientMessage_DeployTrap:
		return s.handleDeployTrap(uid, p.DeployTrap)
	case *gamev1.ClientMessage_Ready:
		return s.handleReady(uid, p.Ready)
	case *gamev1.ClientMessage_Disarm:
		return s.handleDisarm(uid, p.Disarm)
	case *gamev1.ClientMessage_Stride:
		return s.handleStride(uid, p.Stride)
	case *gamev1.ClientMessage_MoveTo:
		return s.handleMoveTo(uid, p.MoveTo)
	case *gamev1.ClientMessage_Step:
		return s.handleStep(uid, p.Step)
	case *gamev1.ClientMessage_Hide:
		return s.handleHide(uid)
	case *gamev1.ClientMessage_Sneak:
		return s.handleSneak(uid)
	case *gamev1.ClientMessage_Divert:
		return s.handleDivert(uid)
	case *gamev1.ClientMessage_Escape:
		return s.handleEscape(uid)
	case *gamev1.ClientMessage_SummonItem:
		return s.handleSummonItem(uid, p.SummonItem)
	case *gamev1.ClientMessage_ProficienciesRequest:
		return s.handleProficiencies(uid)
	case *gamev1.ClientMessage_LevelUp:
		return s.handleLevelUp(uid, p.LevelUp.Ability)
	case *gamev1.ClientMessage_CombatDefault:
		return s.handleCombatDefault(uid, p.CombatDefault.Action)
	case *gamev1.ClientMessage_TrainSkill:
		return s.handleTrainSkill(uid, p.TrainSkill.SkillId)
	case *gamev1.ClientMessage_Grant:
		return s.handleGrant(uid, p.Grant)
	case *gamev1.ClientMessage_Shove:
		return s.handleShove(uid, p.Shove)
	case *gamev1.ClientMessage_Tumble:
		return s.handleTumble(uid, p.Tumble)
	case *gamev1.ClientMessage_Seek:
		return s.handleSeek(uid)
	case *gamev1.ClientMessage_Climb:
		return s.handleClimb(uid, p.Climb)
	case *gamev1.ClientMessage_Swim:
		return s.handleSwim(uid, p.Swim)
	case *gamev1.ClientMessage_Motive:
		return s.handleMotive(uid, p.Motive)
	case *gamev1.ClientMessage_Calm:
		return s.handleCalm(uid, p.Calm)
	case *gamev1.ClientMessage_HeroPoint:
		return s.handleHeroPoint(uid, p.HeroPoint)
	case *gamev1.ClientMessage_Join:
		return s.handleJoin(uid, p.Join)
	case *gamev1.ClientMessage_Decline:
		return s.handleDecline(uid, p.Decline)
	case *gamev1.ClientMessage_Group:
		return s.handleGroup(uid, p.Group)
	case *gamev1.ClientMessage_Invite:
		return s.handleInvite(uid, p.Invite)
	case *gamev1.ClientMessage_AcceptGroup:
		return s.handleAcceptGroup(uid, p.AcceptGroup)
	case *gamev1.ClientMessage_DeclineGroup:
		return s.handleDeclineGroup(uid, p.DeclineGroup)
	case *gamev1.ClientMessage_Ungroup:
		return s.handleUngroup(uid, p.Ungroup)
	case *gamev1.ClientMessage_Kick:
		return s.handleKick(uid, p.Kick)
	case *gamev1.ClientMessage_Browse:
		return s.handleBrowse(uid, p.Browse)
	case *gamev1.ClientMessage_Buy:
		return s.handleBuy(uid, p.Buy)
	case *gamev1.ClientMessage_Sell:
		return s.handleSell(uid, p.Sell)
	case *gamev1.ClientMessage_Negotiate:
		return s.handleNegotiate(uid, p.Negotiate)
	case *gamev1.ClientMessage_StashDeposit:
		return s.handleStashDeposit(uid, p.StashDeposit)
	case *gamev1.ClientMessage_StashWithdraw:
		return s.handleStashWithdraw(uid, p.StashWithdraw)
	case *gamev1.ClientMessage_StashBalance:
		return s.handleStashBalance(uid, p.StashBalance)
	case *gamev1.ClientMessage_Heal:
		return s.handleHeal(uid, p.Heal)
	case *gamev1.ClientMessage_HealAmount:
		return s.handleHealAmount(uid, p.HealAmount)
	case *gamev1.ClientMessage_TrainJob:
		return s.handleTrainJob(uid, p.TrainJob)
	case *gamev1.ClientMessage_TrainTech:
		return s.handleTrainTech(uid, p.TrainTech.GetNpcName(), p.TrainTech.GetTechId())
	case *gamev1.ClientMessage_ChooseFeat:
		return s.handleChooseFeat(uid, int(p.ChooseFeat.GetGrantLevel()), p.ChooseFeat.GetFeatId())
	case *gamev1.ClientMessage_ReactionResponse:
		// Route the reaction response to the goroutine blocked inside
		// buildReactionCallback. No server event is produced here; the
		// blocked goroutine emits follow-up MessageEvents on wake.
		if s.reactionPromptHub != nil {
			s.reactionPromptHub.Deliver(p.ReactionResponse)
		}
		return nil, nil
	case *gamev1.ClientMessage_ListJobs:
		return s.handleListJobs(uid, p.ListJobs)
	case *gamev1.ClientMessage_SetJob:
		return s.handleSetJob(uid, p.SetJob)
	case *gamev1.ClientMessage_Hire:
		return s.handleHire(uid, p.Hire)
	case *gamev1.ClientMessage_Dismiss:
		return s.handleDismiss(uid, p.Dismiss)
	case *gamev1.ClientMessage_Talk:
		return s.handleTalk(uid, p.Talk)
	case *gamev1.ClientMessage_BribeRequest:
		return s.handleBribe(uid, p.BribeRequest)
	case *gamev1.ClientMessage_BribeConfirmRequest:
		return s.handleBribeConfirm(uid, p.BribeConfirmRequest)
	case *gamev1.ClientMessage_SurrenderRequest:
		return s.handleSurrender(uid, p.SurrenderRequest)
	case *gamev1.ClientMessage_ReleaseRequest:
		return s.handleRelease(uid, p.ReleaseRequest)
	case *gamev1.ClientMessage_SpawnNpc:
		return s.handleSpawnNPC(uid, p.SpawnNpc)
	case *gamev1.ClientMessage_AddRoom:
		return s.handleAddRoom(uid, p.AddRoom)
	case *gamev1.ClientMessage_AddLink:
		return s.handleAddLink(uid, p.AddLink)
	case *gamev1.ClientMessage_RemoveLink:
		return s.handleRemoveLink(uid, p.RemoveLink)
	case *gamev1.ClientMessage_SetRoom:
		return s.handleSetRoom(uid, p.SetRoom)
	case *gamev1.ClientMessage_EditorCmds:
		return s.handleEditorCmds(uid)
	case *gamev1.ClientMessage_Travel:
		return s.handleTravel(uid, p.Travel)
	case *gamev1.ClientMessage_ActivateItem:
		return s.handleActivate(uid, p.ActivateItem)
	case *gamev1.ClientMessage_FactionRequest:
		return s.handleFaction(uid, p.FactionRequest)
	case *gamev1.ClientMessage_FactionInfoRequest:
		return s.handleFactionInfo(uid, p.FactionInfoRequest)
	case *gamev1.ClientMessage_FactionStandingRequest:
		return s.handleFactionStanding(uid, p.FactionStandingRequest)
	case *gamev1.ClientMessage_ChangeRepRequest:
		return s.handleChangeRep(uid, p.ChangeRepRequest)
	case *gamev1.ClientMessage_TabComplete:
		return s.handleTabComplete(uid, p.TabComplete.GetPrefix())
	case *gamev1.ClientMessage_MaterialsRequest:
		return s.handleMaterials(uid, p.MaterialsRequest)
	case *gamev1.ClientMessage_ScavengeRequest:
		return s.handleScavenge(uid)
	case *gamev1.ClientMessage_CraftListRequest:
		return s.handleCraftList(uid, p.CraftListRequest)
	case *gamev1.ClientMessage_CraftRequest:
		return s.handleCraft(uid, p.CraftRequest)
	case *gamev1.ClientMessage_CraftConfirmRequest:
		return s.handleCraftConfirm(uid)
	case *gamev1.ClientMessage_AffixRequest:
		return s.handleAffix(uid, p.AffixRequest)
	case *gamev1.ClientMessage_ExploreRequest:
		return s.handleExplore(uid, p.ExploreRequest)
	case *gamev1.ClientMessage_QuestRequest:
		return s.handleQuestCommand(uid, p.QuestRequest.GetArgs())
	case *gamev1.ClientMessage_UncurseRequest:
		return s.handleUncurse(uid, p.UncurseRequest)
	case *gamev1.ClientMessage_DowntimeRequest:
		return s.handleDowntime(uid, p.DowntimeRequest)
	case *gamev1.ClientMessage_RefocusRequest:
		if err := s.handleRefocus(uid); err != nil {
			return nil, err
		}
		return nil, nil
	case *gamev1.ClientMessage_SeduceRequest:
		return s.handleSeduce(uid, p.SeduceRequest)
	case *gamev1.ClientMessage_SpawnCharRequest:
		return s.handleSpawnChar(uid, p.SpawnCharRequest)
	case *gamev1.ClientMessage_DeleteCharRequest:
		return s.handleDeleteChar(uid, p.DeleteCharRequest)
	case *gamev1.ClientMessage_KillNpcRequest:
		return s.handleKillNPC(uid, p.KillNpcRequest)
	case *gamev1.ClientMessage_UncoverRequest:
		return s.handleUncover(uid)
	case *gamev1.ClientMessage_JobGrantsRequest:
		return s.handleJobGrants(uid)
	case *gamev1.ClientMessage_QuestLogRequest:
		return s.handleQuestLog(uid)
	case *gamev1.ClientMessage_HotbarRequest:
		evt, hbErr := s.handleHotbar(uid, p.HotbarRequest)
		if hbErr != nil {
			return errorEvent(hbErr.Error()), nil
		}
		// handleHotbar "show" returns nil; events already pushed per-slot.
		// For set/clear: handleHotbar already returns a HotbarUpdateEvent directly.
		return evt, nil
	default:
		return nil, fmt.Errorf("unknown message type")
	}
}

func (s *GameServiceServer) handleMove(uid string, req *gamev1.MoveRequest) (*gamev1.ServerEvent, error) {
	dir := world.Direction(req.Direction)

	// REQ-MV-1: movement is blocked while the player is in combat.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Status == statusInCombat {
		return messageEvent("You cannot move while in combat."), nil
	}

	// REQ-DT-5: movement is blocked while a downtime activity is active.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.DowntimeBusy {
		return messageEvent("You are busy with a downtime activity. Use 'downtime cancel' to stop."), nil
	}

	// REQ-REST-16: cancel camping if player moves voluntarily.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.CampingActive {
		s.cancelCamping(uid, sess, "you broke camp")
	}

	// Cancel refocus if player moves.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.RefocusingActive {
		sess.RefocusingActive = false
		sess.RefocusingStartTime = time.Time{}
		s.pushMessageToUID(uid, "You stop refocusing.")
	}

	// IMMOBILIZED: grabbed condition prevents leaving the room.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil {
		if condition.IsActionRestricted(sess.Conditions, "move") {
			return errorEvent("You are grabbed and cannot move!"), nil
		}
		if condition.IsMovementPrevented(sess.Conditions) {
			return errorEvent("You are detained and cannot move."), nil
		}
	}

	// Faction room gating check (REQ-FA-18, REQ-FA-29).
	if s.factionSvc != nil {
		if moveSess, moveSessOK := s.sessions.GetPlayer(uid); moveSessOK {
			if destRoom, destErr := s.world.Navigate(moveSess.RoomID, dir); destErr == nil {
				if destRoom.MinFactionTierID != "" {
					if zone, zoneOK := s.world.GetZone(destRoom.ZoneID); zoneOK {
						if !s.factionSvc.CanEnterRoom(moveSess, destRoom, zone) {
							tierLabel, factionName := s.factionSvc.MinTierLabelForRoom(destRoom, zone)
							return messageEvent(fmt.Sprintf("Only %s members of %s may enter here.", tierLabel, factionName)), nil
						}
					}
				}
			}
		}
	}

	// Team territory enforcement: block movement into enemy zones (REQ-TEAM-1).
	if teamSess, teamSessOK := s.sessions.GetPlayer(uid); teamSessOK && teamSess.Team != "" {
		if destRoom, navErr := s.world.Navigate(teamSess.RoomID, dir); navErr == nil {
			if isEnemyZone(teamSess.Team, destRoom.ZoneID) {
				return messageEvent("Enemy territory — that zone is closed to you."), nil
			}
		}
	}

	result, err := s.worldH.MoveWithContext(uid, dir)
	if err != nil {
		return nil, err
	}

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found after move", uid)
	}

	// Broadcast departure from old room
	s.broadcastRoomEvent(result.OldRoomID, uid, &gamev1.RoomEvent{
		Player:    sess.CharName,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
		Direction: string(dir),
	})
	// Notify remaining players in source room via passive techs.
	s.triggerPassiveTechsForRoom(result.OldRoomID)

	// Broadcast arrival in new room
	s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
		Player:    sess.CharName,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
		Direction: string(dir.Opposite()),
	})
	// Notify all players in destination room via passive techs. This fires after
	// the arrival broadcast (room occupancy is settled) but before Lua on_enter
	// hooks, which is intentional — passive sensing observes physical presence,
	// not script-driven events.
	s.triggerPassiveTechsForRoom(result.View.RoomId)

	if s.scriptMgr != nil {
		if oldRoom, ok := s.world.GetRoom(result.OldRoomID); ok {
			s.scriptMgr.CallHook(oldRoom.ZoneID, "on_exit", //nolint:errcheck
				lua.LString(uid),
				lua.LString(result.OldRoomID),
				lua.LString(result.View.RoomId),
			)
		}
		if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
			s.scriptMgr.CallHook(newRoom.ZoneID, "on_enter", //nolint:errcheck
				lua.LString(uid),
				lua.LString(result.View.RoomId),
				lua.LString(result.OldRoomID),
			)
		}
	}

	// Move bound hireling with player on room transition.
	if s.npcMgr != nil {
		var oldZoneID, newZoneID string
		if oldRoom, ok := s.world.GetRoom(result.OldRoomID); ok {
			oldZoneID = oldRoom.ZoneID
		}
		if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
			newZoneID = newRoom.ZoneID
		}
		s.moveHirelingWithPlayer(uid, result.View.RoomId, oldZoneID, newZoneID)
	}

	// Fire on_enter skill check triggers for the new room.
	// Messages are pushed to the player's entity channel so they are delivered
	// alongside the room-view response via the forwardEvents goroutine.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		if msgs := s.applyRoomSkillChecks(uid, newRoom); len(msgs) > 0 {
			for _, msg := range msgs {
				msgEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{
							Content: msg,
							Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
						},
					},
				}
				data, marshalErr := proto.Marshal(msgEvt)
				if marshalErr == nil {
					if pushErr := sess.Entity.Push(data); pushErr != nil {
						s.logger.Warn("pushing skill check message to player entity",
							zap.String("uid", uid),
							zap.Error(pushErr),
						)
					}
				}
			}
		}
	}

	// REQ-ZN-13: accumulate terrain_ condition AP costs from room.Effects.
	// Unless the player has zone_awareness, send a flavor message for each terrain condition.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok && s.condRegistry != nil {
		var terrainConds []*condition.ConditionDef
		for _, eff := range newRoom.Effects {
			if !strings.HasPrefix(eff.Track, "terrain_") {
				continue
			}
			def, defOK := s.condRegistry.Get(eff.Track)
			if !defOK || def.MoveAPCost <= 0 {
				continue
			}
			terrainConds = append(terrainConds, def)
		}
		sort.Slice(terrainConds, func(i, j int) bool {
			return terrainConds[i].ID < terrainConds[j].ID
		})
		if !sess.PassiveFeats["zone_awareness"] {
			for _, def := range terrainConds {
				terrainEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{
							Content: fmt.Sprintf("The ground here is %s — movement costs extra AP.", strings.ToLower(def.Name)),
							Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
						},
					},
				}
				if data, marshalErr := proto.Marshal(terrainEvt); marshalErr == nil {
					if pushErr := sess.Entity.Push(data); pushErr != nil {
						s.logger.Warn("pushing terrain condition message to player entity",
							zap.String("uid", uid),
							zap.String("condition", def.ID),
							zap.Error(pushErr),
						)
					}
				}
			}
		}
	}

	// Fire on_enter hazards for the new room (REQ-AE-28).
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		if len(newRoom.Hazards) > 0 {
			ApplyHazards(newRoom, sess, "on_enter", s.dice, s.condRegistry,
				func(msg string) { s.pushMessageToUID(uid, msg) },
				s.logger,
			)
		}
	}

	// Apply out-of-combat zone effects for the new room.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		s.applyRoomEffectsOnEntry(sess, uid, newRoom, time.Now().Unix())
	}

	// Record automap discovery for the new room.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		zID := newRoom.ZoneID
		if sess.AutomapCache[zID] == nil {
			sess.AutomapCache[zID] = make(map[string]bool)
		}
		if sess.ExploredCache[zID] == nil {
			sess.ExploredCache[zID] = make(map[string]bool)
		}
		// Mark the room as physically explored (first physical entry only).
		// XP and quest progress are awarded here so that rooms pre-loaded into
		// AutomapCache via zone reveal still grant XP when first walked into.
		if !sess.ExploredCache[zID][newRoom.ID] {
			sess.ExploredCache[zID][newRoom.ID] = true
			if s.automapRepo != nil {
				if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, zID, newRoom.ID, true); err != nil {
					s.logger.Warn("persisting exploration flag", zap.Error(err))
				}
			}
			if s.questSvc != nil {
				if exploreMsgs, exploreErr := s.questSvc.RecordExplore(context.Background(), sess, sess.CharacterID, newRoom.ID); exploreErr == nil {
					for _, em := range exploreMsgs {
						s.pushMessageToUID(uid, em)
					}
					if len(exploreMsgs) > 0 {
						s.pushCharacterSheet(sess)
					}
				} else {
					s.logger.Warn("RecordExplore failed", zap.String("uid", uid), zap.Error(exploreErr))
				}
			}
			// Award room discovery XP on first physical entry.
			if s.xpSvc != nil {
				oldLevel := sess.Level
				if xpMsgs, xpErr := s.xpSvc.AwardRoomDiscovery(context.Background(), sess, sess.CharacterID); xpErr != nil {
					s.logger.Warn("awarding room discovery XP", zap.String("uid", uid), zap.Error(xpErr))
				} else {
					for _, msg := range xpMsgs {
						xpEvt := &gamev1.ServerEvent{
							Payload: &gamev1.ServerEvent_Message{
								Message: &gamev1.MessageEvent{
									Content: msg,
									Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
								},
							},
						}
						if data, marshalErr := proto.Marshal(xpEvt); marshalErr == nil {
							if pushErr := sess.Entity.Push(data); pushErr != nil {
								s.logger.Warn("pushing room discovery XP message", zap.String("uid", uid), zap.Error(pushErr))
							}
						}
					}
					if len(xpMsgs) > 0 {
						s.pushHPUpdate(uid, sess)
						s.pushCharacterSheet(sess)
						// REQ-BUG99-3: apply tech grants for organic level-ups from room discovery XP.
						// Guard on actual level change: AwardRoomDiscovery always returns >= 1 message.
						if sess.Level > oldLevel {
							s.applyLevelUpTechGrants(context.Background(), sess, oldLevel, sess.Level)
						}
					}
				}
			}
		}
		// Mark the room as known on the automap (if not already via zone reveal or prior visit).
		if !sess.AutomapCache[zID][newRoom.ID] {
			sess.AutomapCache[zID][newRoom.ID] = true
			if s.automapRepo != nil {
				if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, zID, newRoom.ID, false); err != nil {
					s.logger.Warn("persisting map discovery", zap.Error(err))
				}
			}
		}
	}

	// Fire armed entry-trigger traps and perform Search-mode detection.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		s.checkEntryTraps(uid, sess, newRoom)
	}

	// Clear LayLow crit-fail block from previous room (REQ-EXP-8a).
	sess.LayLowBlockedRoom = ""

	// Fire exploration mode room-entry hook (REQ-EXP-38).
	if sess.ExploreMode != "" {
		if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
			msgs := s.applyExploreModeOnEntry(uid, sess, newRoom)
			for _, msg := range msgs {
				s.pushMessageToUID(uid, msg)
			}
		}
	}

	// Room trap roll and wanted-level guard check on room entry.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		if newZone, zoneOK := s.world.GetZone(newRoom.ZoneID); zoneOK {
			effectiveLevel := danger.EffectiveDangerLevel(newZone.DangerLevel, newRoom.DangerLevel)
			if danger.RollRoomTrap(effectiveLevel, newRoom.RoomTrapChance, danger.RandRoller{}) {
				s.logger.Info("room trap triggered", zap.String("uid", uid), zap.String("room", newRoom.ID))
				trapEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{
							Content: "You trigger a hidden trap!",
							Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
						},
					},
				}
				if data, marshalErr := proto.Marshal(trapEvt); marshalErr == nil {
					if pushErr := sess.Entity.Push(data); pushErr != nil {
						s.logger.Warn("pushing trap message to player entity",
							zap.String("uid", uid),
							zap.Error(pushErr),
						)
					}
				}
			}
		}
		// Wanted-level guard check: initiate guard combat if player is wanted in this zone.
		// Skip re-evaluation during the 5-second grace window after detention completes (REQ-WC-14c).
		if !time.Now().Before(sess.DetentionGraceUntil) {
			wantedLevel := sess.WantedLevel[newRoom.ZoneID]
			if wantedLevel >= 2 && s.combatH != nil {
				s.combatH.InitiateGuardCombat(uid, newRoom.ZoneID, wantedLevel)
			} else if wantedLevel == 1 && s.npcMgr != nil {
				// WantedLevel 1: guards watch and warn without engaging.
				for _, inst := range s.npcMgr.InstancesInRoom(newRoom.ID) {
					if inst.NPCType == "guard" {
						s.pushMessageToUID(uid, fmt.Sprintf("%s eyes you suspiciously. \"We're watching you.\"", inst.Name()))
						break
					}
				}
			}
		}
	}

	if s.npcH != nil {
		for _, inst := range s.npcH.InstancesInRoom(result.View.RoomId) {
			// Set PlayerEnteredRoom flag for one-shot idle tick evaluation. REQ-NB-4.
			inst.PlayerEnteredRoom = true
		}
	}

	if msgs := s.applyNPCSkillChecks(uid, result.View.RoomId); len(msgs) > 0 {
		if npcSess, ok := s.sessions.GetPlayer(uid); ok {
			for _, msg := range msgs {
				msgEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{
							Content: msg,
							Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
						},
					},
				}
				data, marshalErr := proto.Marshal(msgEvt)
				if marshalErr == nil {
					if pushErr := npcSess.Entity.Push(data); pushErr != nil {
						s.logger.Warn("pushing NPC skill check message to player entity",
							zap.String("uid", uid),
							zap.Error(pushErr),
						)
					}
				}
			}
		}
	}

	// Combat join trigger — prompt the player if entering a room with active combat.
	s.notifyCombatJoinIfEligible(sess, result.View.RoomId)

	// Clear any active negotiate state: modifiers must not persist across rooms. REQ-NPC-5a.
	s.clearNegotiateState(sess)

	// Clear crafting session state on room exit (REQ-CRAFT-11).
	sess.ScavengeExhaustedRoomID = ""
	sess.PendingCraftRecipeID = ""

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: result.View},
	}, nil
}

// signedInt formats n with an explicit sign, e.g. "+3" or "-1" or "+0".
//
// Postcondition: always has a leading '+' or '-'.
func signedInt(n int) string {
	if n >= 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}

// weaponDamageString formats weapon damage for display, e.g. "1d8+3" or "1d8".
// For melee weapons the ability modifier is added to damage; ranged weapons show only the dice.
//
// Precondition: damageDice is a valid dice expression (e.g. "1d8").
// Postcondition: returns a non-empty string.
func weaponDamageString(damageDice string, abilityMod int, isMelee bool) string {
	if !isMelee || abilityMod == 0 {
		return damageDice
	}
	if abilityMod > 0 {
		return fmt.Sprintf("%s+%d", damageDice, abilityMod)
	}
	return fmt.Sprintf("%s%d", damageDice, abilityMod)
}

// skillDisplayName converts a snake_case skill ID to a title-cased display name.
//
// Precondition: id must be non-empty.
// Postcondition: Returns a non-empty title-cased string.
func skillDisplayName(id string) string {
	parts := strings.Split(id, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// outcomeDisplayName converts a CheckOutcome to a human-readable string.
//
// Postcondition: Returns one of "critical success", "success", "failure", "critical failure".
func outcomeDisplayName(o skillcheck.CheckOutcome) string {
	switch o {
	case skillcheck.CritSuccess:
		return "critical success"
	case skillcheck.Success:
		return "success"
	case skillcheck.Failure:
		return "failure"
	case skillcheck.CritFailure:
		return "critical failure"
	default:
		return "unknown"
	}
}

// abilityModFrom returns the PF2E-style ability modifier for a given score.
//
// Precondition: score is any integer.
// Postcondition: returns (score-10)/2 for scores >= 10, and (score-11)/2 for scores < 10,
// matching the standard mathematical floor division for negative values.
func abilityModFrom(score int) int {
	if score >= 10 {
		return (score - 10) / 2
	}
	return (score - 11) / 2
}

// abilityScoreForSkill returns the raw ability score from the session corresponding to
// the ability linked to the given skill ID.
//
// Precondition: sess must be non-nil; skillID must be non-empty.
// Postcondition: returns the ability score, or 10 (neutral) if the skill is not found in allSkills.
func (s *GameServiceServer) abilityScoreForSkill(sess *session.PlayerSession, skillID string) int {
	for _, sk := range s.allSkills {
		if sk.ID == skillID {
			switch sk.Ability {
			case "brutality":
				return sess.Abilities.Brutality
			case "grit":
				return sess.Abilities.Grit
			case "quickness":
				return sess.Abilities.Quickness
			case "reasoning":
				return sess.Abilities.Reasoning
			case "savvy":
				return sess.Abilities.Savvy
			case "flair":
				return sess.Abilities.Flair
			}
		}
	}
	return 10
}

// applyRoomSkillChecks fires all on_enter skill check triggers for the given room.
// Returns the outcome message strings to send to the player; callers are responsible for delivery.
//
// Precondition: uid must be non-empty; room must be non-nil.
// Postcondition: each on_enter TriggerDef in room.SkillChecks is resolved; damage effects reduce sess.CurrentHP.
func (s *GameServiceServer) applyRoomSkillChecks(uid string, room *world.Room) []string {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil
	}

	var msgs []string
	shadowUsed := false
	for _, trigger := range room.SkillChecks {
		if trigger.Trigger != "on_enter" {
			continue
		}

		abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
		amod := abilityModFrom(abilityScore)
		rank := ""
		if sess.Skills != nil {
			rank = sess.Skills[trigger.Skill]
		}
		// Shadow mode: use ally's rank for the FIRST check where ally's rank exceeds player's (REQ-EXP-29, REQ-EXP-30).
		if !shadowUsed && sess.ExploreMode == session.ExploreModeShadow && sess.ExploreShadowTarget != 0 {
			if ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget); ally != nil && ally.RoomID == sess.RoomID {
				if allyRank := ally.Skills[trigger.Skill]; allyRank != "" {
					if skillRankBonus(allyRank) > skillRankBonus(rank) {
						rank = allyRank
						shadowUsed = true
					}
				}
			}
		}

		var roll int
		if s.dice != nil {
			roll = s.dice.Src().Intn(20) + 1
		} else {
			roll = 10 // neutral fallback when no dice configured
		}

		result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

		detail := fmt.Sprintf("%s check (DC %d): rolled %d+%d=%d — %s.",
			skillDisplayName(trigger.Skill), trigger.DC, roll, amod, result.Total, outcomeDisplayName(result.Outcome))
		msgs = append(msgs, detail)

		outcome := trigger.Outcomes.ForOutcome(result.Outcome)
		if outcome != nil {
			if outcome.Message != "" {
				msgs = append(msgs, outcome.Message)
			}
			s.applySkillCheckEffect(sess, outcome.Effect, room.ID)
		}

		// Award XP for successful skill checks.
		if s.xpSvc != nil && (result.Outcome == skillcheck.CritSuccess || result.Outcome == skillcheck.Success) {
			isCrit := result.Outcome == skillcheck.CritSuccess
			oldLevel := sess.Level
			if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
				s.logger.Warn("awarding skill check XP", zap.String("uid", uid), zap.Error(xpErr))
			} else {
				msgs = append(msgs, xpMsgs...)
				if len(xpMsgs) > 0 {
					s.pushHPUpdate(uid, sess)
					s.pushCharacterSheet(sess)
					// REQ-BUG99-4: apply tech grants for organic level-ups from skill check XP.
					// Guard on actual level change: AwardSkillCheck always returns >= 1 message.
					if sess.Level > oldLevel {
						s.applyLevelUpTechGrants(context.Background(), sess, oldLevel, sess.Level)
					}
				}
			}
		}

		if s.scriptMgr != nil {
			s.scriptMgr.CallHook(room.ZoneID, "on_skill_check", //nolint:errcheck
				lua.LString(uid),
				lua.LString(trigger.Skill),
				lua.LNumber(result.Total),
				lua.LNumber(trigger.DC),
				lua.LString(result.Outcome.String()),
			)
		}
	}
	return msgs
}

// applyRoomEffectsOnEntry applies zone effects for the given room to sess on room entry.
//
// Precondition: sess and room must not be nil; uid is the player UID; now is Unix timestamp seconds.
// Postcondition: For each effect in room.Effects not currently on cooldown, resolves a binary
// Will save (d20 + GritMod vs BaseDC).  On failure: the condition identified by effect.Track is
// looked up in condRegistry and applied to sess.Conditions; no cooldown is recorded.  On success:
// ZoneEffectCooldowns[roomID:track] is set to now + CooldownMinutes*60.
// If condRegistry is nil or does not contain effect.Track, that effect is silently skipped.
func (s *GameServiceServer) applyRoomEffectsOnEntry(
	sess *session.PlayerSession, uid string, room *world.Room, now int64,
) {
	effects := room.Effects
	if s.weatherMgr != nil {
		effects = append(effects, s.weatherMgr.ActiveEffects(room.Indoor)...)
	}
	if len(effects) == 0 {
		return
	}
	for _, effect := range effects {
		if s.condRegistry == nil {
			continue
		}
		condDef, ok := s.condRegistry.Get(effect.Track)
		if !ok {
			continue
		}
		key := room.ID + ":" + effect.Track
		if sess.ZoneEffectCooldowns != nil && sess.ZoneEffectCooldowns[key] > now {
			continue // immune: cooldown has not expired
		}
		gritMod := combat.AbilityMod(sess.Abilities.Grit)
		var roll int
		if s.dice != nil {
			roll = s.dice.Src().Intn(20) + 1
		} else {
			roll = 10 // neutral fallback when no dice configured
		}
		total := roll + gritMod
		if total < effect.BaseDC {
			// Failed save: apply condition via registry.
			if sess.Conditions != nil {
				if err := sess.Conditions.Apply(uid, condDef, 1, -1); err != nil {
					s.logger.Warn("applyRoomEffectsOnEntry: Apply failed",
						zap.String("uid", uid),
						zap.String("track", effect.Track),
						zap.Error(err),
					)
				}
			}
		} else {
			// Successful save: record immunity cooldown.
			if sess.ZoneEffectCooldowns == nil {
				sess.ZoneEffectCooldowns = make(map[string]int64)
			}
			sess.ZoneEffectCooldowns[key] = now + int64(effect.CooldownMinutes)*60
		}
	}
}

// applyNPCSkillChecks fires on_greet skill check triggers for all NPCs in the room.
// Returns message strings to send to the player; callers are responsible for delivery.
//
// Precondition: uid must be non-empty; roomID must be non-empty.
// Postcondition: each on_greet TriggerDef on every NPC instance in the room is resolved;
// deny effects are not applicable for greet and are silently skipped.
func (s *GameServiceServer) applyNPCSkillChecks(uid string, roomID string) []string {
	if s.npcH == nil {
		return nil
	}

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil
	}

	var zoneID string
	if room, roomOk := s.world.GetRoom(roomID); roomOk {
		zoneID = room.ZoneID
	}

	var msgs []string
	for _, inst := range s.npcH.InstancesInRoom(roomID) {
		for _, trigger := range inst.SkillChecks {
			if trigger.Trigger != "on_greet" {
				continue
			}

			abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
			amod := abilityModFrom(abilityScore)
			rank := ""
			if sess.Skills != nil {
				rank = sess.Skills[trigger.Skill]
			}

			var roll int
			if s.dice != nil {
				roll = s.dice.Src().Intn(20) + 1
			} else {
				roll = 10 // neutral fallback when no dice configured
			}

			result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

			detail := fmt.Sprintf("%s check (DC %d): rolled %d+%d=%d — %s.",
				skillDisplayName(trigger.Skill), trigger.DC, roll, amod, result.Total, outcomeDisplayName(result.Outcome))
			msgs = append(msgs, detail)

			outcome := trigger.Outcomes.ForOutcome(result.Outcome)
			if outcome != nil {
				if outcome.Message != "" {
					msgs = append(msgs, fmt.Sprintf("%s: %s", inst.Name(), outcome.Message))
				}
				// Apply non-deny effects (deny is not applicable for on_greet).
				if outcome.Effect == nil || outcome.Effect.Type != "deny" {
					s.applySkillCheckEffect(sess, outcome.Effect, roomID)
				}
			}

			// Award XP for successful NPC skill checks.
			if s.xpSvc != nil && (result.Outcome == skillcheck.CritSuccess || result.Outcome == skillcheck.Success) {
				isCrit := result.Outcome == skillcheck.CritSuccess
				oldLevel := sess.Level
				if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
					s.logger.Warn("awarding NPC skill check XP", zap.String("uid", uid), zap.Error(xpErr))
				} else {
					msgs = append(msgs, xpMsgs...)
					if len(xpMsgs) > 0 {
						s.pushHPUpdate(uid, sess)
						s.pushCharacterSheet(sess)
						// REQ-BUG99-5: apply tech grants for organic level-ups from NPC skill check XP.
						// Guard on actual level change: AwardSkillCheck always returns >= 1 message.
						if sess.Level > oldLevel {
							s.applyLevelUpTechGrants(context.Background(), sess, oldLevel, sess.Level)
						}
					}
				}
			}

			if s.scriptMgr != nil {
				s.scriptMgr.CallHook(zoneID, "on_skill_check", //nolint:errcheck
					lua.LString(uid),
					lua.LString(trigger.Skill),
					lua.LNumber(result.Total),
					lua.LNumber(trigger.DC),
					lua.LString(result.Outcome.String()),
				)
			}
		}
	}
	return msgs
}

func (s *GameServiceServer) handleLook(uid string) (*gamev1.ServerEvent, error) {
	view, err := s.worldH.Look(uid)
	if err != nil {
		return nil, err
	}
	if s.scriptMgr != nil {
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			if room, ok := s.world.GetRoom(sess.RoomID); ok {
				s.scriptMgr.CallHook(room.ZoneID, "on_look", //nolint:errcheck
					lua.LString(uid),
					lua.LString(room.ID),
				)
			}
		}
	}

	// Append floor items to the room view.
	//
	// Precondition: view is a fully constructed RoomView from worldH.Look.
	// Postcondition: view.FloorItems contains all items on the floor of the
	// player's current room; item names are resolved from the registry when
	// available, falling back to the raw ItemDefID.
	if sess, ok := s.sessions.GetPlayer(uid); ok && s.floorMgr != nil {
		floorItems := s.floorMgr.ItemsInRoom(sess.RoomID)
		for _, fi := range floorItems {
			name := fi.ItemDefID
			if s.invRegistry != nil {
				if def, ok := s.invRegistry.Item(fi.ItemDefID); ok {
					name = def.Name
				}
			}
			view.FloorItems = append(view.FloorItems, &gamev1.FloorItem{
				InstanceId: fi.InstanceID,
				Name:       name,
				Quantity:   int32(fi.Quantity),
			})
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: view},
	}, nil
}

func (s *GameServiceServer) handleExits(uid string) (*gamev1.ServerEvent, error) {
	exitList, err := s.worldH.Exits(uid)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ExitList{ExitList: exitList},
	}, nil
}

func (s *GameServiceServer) handleSay(uid string, req *gamev1.SayRequest) (*gamev1.ServerEvent, error) {
	msgEvt, err := s.chatH.Say(uid, req.Message)
	if err != nil {
		return nil, err
	}

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found", uid)
	}
	s.broadcastMessage(sess.RoomID, uid, msgEvt)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: msgEvt},
	}, nil
}

func (s *GameServiceServer) handleEmote(uid string, req *gamev1.EmoteRequest) (*gamev1.ServerEvent, error) {
	msgEvt, err := s.chatH.Emote(uid, req.Action)
	if err != nil {
		return nil, err
	}

	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q session not found", uid)
	}
	s.broadcastMessage(sess.RoomID, uid, msgEvt)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: msgEvt},
	}, nil
}

func (s *GameServiceServer) handleWho(uid string) (*gamev1.ServerEvent, error) {
	playerList, err := s.chatH.Who(uid)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_PlayerList{PlayerList: playerList},
	}, nil
}

func (s *GameServiceServer) handleQuit(uid string) (*gamev1.ServerEvent, error) {
	sess, _ := s.sessions.GetPlayer(uid)
	reason := "Goodbye"
	if sess != nil {
		reason = fmt.Sprintf("%s has quit", sess.CharName)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Disconnected{
			Disconnected: &gamev1.Disconnected{Reason: reason},
		},
	}, errQuit
}

// handleSwitch saves the current player's state and signals a clean character switch.
// The behaviour is identical to handleQuit: errQuit causes the Session goroutine to
// call cleanupPlayer via defer, persisting all state before the stream closes.
//
// Precondition: uid must be non-empty.
// Postcondition: Returns a Disconnected event with errQuit so the session terminates cleanly.
func (s *GameServiceServer) handleSwitch(uid string) (*gamev1.ServerEvent, error) {
	sess, _ := s.sessions.GetPlayer(uid)
	reason := "Switching characters"
	if sess != nil {
		reason = fmt.Sprintf("%s is switching characters", sess.CharName)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Disconnected{
			Disconnected: &gamev1.Disconnected{Reason: reason},
		},
	}, errQuit
}

// handleExamine returns detailed information about a named target.
// It first checks for a player with the given name in the same room;
// if found, returns CharacterInfo. Otherwise falls back to NPC examine.
//
// Precondition: uid must be a valid connected player; req.Target must be non-empty.
// Postcondition: Returns CharacterInfo for player targets in the same room, NpcView for NPC targets, or error if not found.
func (s *GameServiceServer) handleExamine(uid string, req *gamev1.ExamineRequest) (*gamev1.ServerEvent, error) {
	// Check if target is a player in the same room.
	examiner, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	target, ok := s.sessions.GetPlayerByCharName(req.Target)
	if ok && target.RoomID == examiner.RoomID {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_CharacterInfo{
				CharacterInfo: &gamev1.CharacterInfo{
					CharacterId: target.CharacterID,
					Name:        target.CharName,
					Region:      target.RegionDisplayName,
					Class:       target.Class,
					Level:       int32(target.Level),
					CurrentHp:   int32(target.CurrentHP),
					MaxHp:       int32(target.MaxHP),
				},
			},
		}, nil
	}

	// Fall back to NPC examine. For typed non-combat NPCs, return a structured view.
	if s.npcH != nil {
		inst := s.npcMgr.FindInRoom(examiner.RoomID, req.Target)
		if inst != nil {
			switch inst.NPCType {
			case "healer":
				return s.buildHealerView(uid, inst)
			case "job_trainer":
				return s.buildTrainerView(uid, inst)
			case "tech_trainer":
				return s.buildTechTrainerView(uid, inst)
			case "fixer":
				return s.buildFixerView(uid, inst)
			case "motel_keeper", "brothel_keeper":
				return s.buildRestView(uid, inst)
			case "quest_giver":
				return s.buildQuestGiverView(uid, inst)
			}
		}
		view, err := s.npcH.Examine(uid, req.Target)
		if err == nil {
			if inst != nil {
				view.NpcType = inst.NPCType
			}
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_NpcView{NpcView: view},
			}, nil
		}
	}

	// Fall back to room equipment interaction (e.g. "examine Zone Map" → interact).
	if s.roomEquipMgr != nil {
		evt, err := s.handleInteract(uid, req.Target)
		if err == nil {
			return evt, nil
		}
	}

	return nil, fmt.Errorf("target %q not found", req.Target)
}

// broadcastRoomEvent sends a RoomEvent to all players in a room except the excluded UID.
func (s *GameServiceServer) broadcastRoomEvent(roomID, excludeUID string, evt *gamev1.RoomEvent) {
	serverEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomEvent{RoomEvent: evt},
	}
	s.broadcastToRoom(roomID, excludeUID, serverEvt)
}

// broadcastMessage sends a MessageEvent to all players in a room except the sender.
func (s *GameServiceServer) broadcastMessage(roomID, excludeUID string, evt *gamev1.MessageEvent) {
	serverEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{Message: evt},
	}
	s.broadcastToRoom(roomID, excludeUID, serverEvt)
}

// broadcastToRoom serializes a ServerEvent and pushes it to all BridgeEntities
// in the room, excluding the specified UID.
func (s *GameServiceServer) broadcastToRoom(roomID, excludeUID string, evt *gamev1.ServerEvent) {
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Error("marshaling broadcast event", zap.Error(err))
		return
	}

	uids := s.sessions.PlayerUIDsInRoom(roomID)
	for _, uid := range uids {
		if uid == excludeUID {
			continue
		}
		sess, ok := s.sessions.GetPlayer(uid)
		if !ok {
			continue
		}
		if err := sess.Entity.Push(data); err != nil {
			s.logger.Warn("push to entity failed",
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}
}

// pushRoomViewToAllInRoom builds a fresh RoomView for each player currently in
// roomID and delivers it via their entity channel.
//
// Precondition: roomID must be a valid room identifier.
// Postcondition: Each player in roomID receives a RoomView event; players
// elsewhere are unaffected.
func (s *GameServiceServer) pushRoomViewToAllInRoom(roomID string) {
	room, ok := s.world.GetRoom(roomID)
	if !ok {
		return
	}
	uids := s.sessions.PlayerUIDsInRoom(roomID)
	for _, uid := range uids {
		rv := s.worldH.buildRoomView(uid, room)
		evt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
		}
		data, err := proto.Marshal(evt)
		if err != nil {
			s.logger.Error("pushRoomViewToAllInRoom: marshal failed", zap.Error(err))
			continue
		}
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			// Use PushBlocking so the room view is never silently dropped
			// when the event buffer is full (e.g., after a busy combat round).
			if err := sess.Entity.PushBlocking(data, 2*time.Second); err != nil {
				s.logger.Warn("pushRoomViewToAllInRoom: failed to deliver room view", zap.String("uid", uid), zap.Error(err))
			}
		}
	}
}

// syncStream wraps a GameService_SessionServer to make Send goroutine-safe.
// gRPC-Go ServerStream.SendMsg is NOT documented as goroutine-safe; multiple
// goroutines (commandLoop, forwardEvents, calendar ticker) must share one stream.
//
// Precondition: GameService_SessionServer must be non-nil.
// Postcondition: concurrent Send calls are serialized; Recv and other methods
// delegate to the underlying stream unchanged.
type syncStream struct {
	mu sync.Mutex
	gamev1.GameService_SessionServer
}

func (ss *syncStream) Send(evt *gamev1.ServerEvent) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.GameService_SessionServer.Send(evt)
}

// forwardEvents reads from the BridgeEntity events channel and sends
// deserialized ServerEvents to the gRPC stream.
func (s *GameServiceServer) forwardEvents(ctx context.Context, entity *session.BridgeEntity, stream gamev1.GameService_SessionServer) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-entity.Events():
			if !ok {
				return
			}
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				s.logger.Error("unmarshaling event from entity", zap.Error(err))
				continue
			}
			if err := stream.Send(&evt); err != nil {
				s.logger.Debug("forward event send failed", zap.Error(err))
				return
			}
		}
	}
}

func (s *GameServiceServer) handleAttack(uid string, req *gamev1.AttackRequest) (*gamev1.ServerEvent, error) {
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && sess.Conditions.Has("submerged") {
		return messageEvent("You are submerged underwater and cannot attack. Swim or Escape to surface."), nil
	}
	// GH #231: after Overpower, further attack actions are blocked until the
	// player's next turn (overpower_committed condition, rounds=1).
	if s.overpowerCommitted(uid) {
		return messageEvent("You've committed your attacks this round to the Overpower strike."), nil
	}

	// Safe-room and danger-level enforcement.
	if sess, ok := s.sessions.GetPlayer(uid); ok {
		if room, roomOK := s.world.GetRoom(sess.RoomID); roomOK {
			if zone, zoneOK := s.world.GetZone(room.ZoneID); zoneOK {
				currentDay := 0
				if s.calendar != nil {
					currentDay = s.calendar.CurrentDateTime().Day
				}
				safeEvents, safeErr := CheckSafeViolation(sess, zone.ID, zone.DangerLevel, room.DangerLevel, currentDay, s.combatH, s.wantedRepo, nil)
				if safeErr != nil {
					return nil, safeErr
				}
				if len(safeEvents) > 0 {
					return &gamev1.ServerEvent{
						Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: safeEvents[0]},
					}, nil
				}
				effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, room.DangerLevel)
				if !danger.CanInitiateCombat(effectiveLevel, "player") {
					return messageEvent("Combat is not permitted in this area."), nil
				}
			}
		}
	}

	// DETAINED TARGET: prevent targeting a detained player.
	if targetSess, ok := s.sessions.GetPlayerByCharName(req.Target); ok {
		if targetSess.Conditions != nil && condition.IsTargetingPrevented(targetSess.Conditions) {
			return messageEvent("You cannot target a detained player."), nil
		}
	}

	// Cancel refocus if player initiates combat.
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.RefocusingActive {
		sess.RefocusingActive = false
		sess.RefocusingStartTime = time.Time{}
		s.pushMessageToUID(uid, "Your focus is broken — combat has started!")
	}

	events, err := s.combatH.Attack(uid, req.Target)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	// Track last explicit combat target and broadcast all events except the first.
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		sess.LastCombatTarget = req.Target
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
		// Push updated room view so all players see the combat status immediately.
		s.pushRoomViewToAllInRoom(sess.RoomID)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleFlee(uid string) (*gamev1.ServerEvent, error) {
	// Capture origRoomID before Flee mutates sess.RoomID on success (FLEE-12).
	origRoomID := ""
	if sess, ok := s.sessions.GetPlayer(uid); ok {
		origRoomID = sess.RoomID
	}

	events, fled, err := s.combatH.Flee(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	// Broadcast all events except the first to the original room (first is returned directly to caller).
	for _, evt := range events[1:] {
		s.broadcastCombatEvent(origRoomID, uid, evt)
	}

	if fled {
		// Push room view to players remaining in original room (FLEE-10).
		s.pushRoomViewToAllInRoom(origRoomID)

		// Push room view to the fleeing player in their new room (FLEE-9).
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			if newRoom, ok := s.world.GetRoom(sess.RoomID); ok {
				rv := s.worldH.buildRoomView(uid, newRoom)
				evt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
				}
				data, err := proto.Marshal(evt)
				if err != nil {
					s.logger.Error("handleFlee: marshal room view failed", zap.Error(err))
				} else {
					_ = sess.Entity.PushBlocking(data, 2*time.Second)
				}
			}
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) broadcastCombatEvent(roomID, excludeUID string, evt *gamev1.CombatEvent) {
	s.broadcastToRoom(roomID, excludeUID, &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: evt},
	})
}

// BroadcastCombatEvents sends combat events to all players in the room.
// Called by CombatHandler's round timer callback.
//
// Postcondition: Each event is delivered to all sessions in roomID.
func (s *GameServiceServer) BroadcastCombatEvents(roomID string, events []*gamev1.CombatEvent) {
	for _, evt := range events {
		s.broadcastCombatEvent(roomID, "", evt)
	}
}

func (s *GameServiceServer) handlePass(uid string) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Pass(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

func (s *GameServiceServer) handleStrike(uid string, req *gamev1.StrikeRequest) (*gamev1.ServerEvent, error) {
	// GH #231: after Overpower, further attack actions are blocked until the
	// player's next turn.
	if s.overpowerCommitted(uid) {
		return messageEvent("You've committed your attacks this round to the Overpower strike."), nil
	}
	events, err := s.combatH.Strike(uid, req.GetTarget())
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	// Track last explicit combat target and broadcast all events except the first.
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		sess.LastCombatTarget = req.GetTarget()
		for _, evt := range events[1:] {
			s.broadcastCombatEvent(sess.RoomID, uid, evt)
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

// handleStatus sends a CharacterInfo event followed by active condition events for uid on stream.
// One ConditionEvent is sent per active condition. If no conditions are active,
// a single empty sentinel ConditionEvent is sent to signal "no conditions".
// All events are sent only to the requesting player — no room broadcast occurs.
//
// Precondition: uid must be a valid connected player; stream must be non-nil.
// Postcondition: A CharacterInfo event is sent first, followed by one or more ConditionEvents; returns nil on success.
func (s *GameServiceServer) handleStatus(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	// Send character info first.
	if err := stream.Send(&gamev1.ServerEvent{
		RequestId: requestID,
		Payload: &gamev1.ServerEvent_CharacterInfo{
			CharacterInfo: &gamev1.CharacterInfo{
				CharacterId: sess.CharacterID,
				Name:        sess.CharName,
				Region:      sess.RegionDisplayName,
				Class:       sess.Class,
				Level:       int32(sess.Level),
				CurrentHp:   int32(sess.CurrentHP),
				MaxHp:       int32(sess.MaxHP),
			},
		},
	}); err != nil {
		return fmt.Errorf("sending character info: %w", err)
	}

	// Then send conditions.
	conds, err := s.combatH.Status(uid)
	if err != nil {
		return err
	}
	if len(conds) == 0 {
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_ConditionEvent{
				ConditionEvent: &gamev1.ConditionEvent{},
			},
		})
	}
	for _, ac := range conds {
		if err := stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_ConditionEvent{
				ConditionEvent: &gamev1.ConditionEvent{
					TargetUid:     uid,
					TargetName:    sess.CharName,
					ConditionId:   ac.Def.ID,
					ConditionName: ac.Def.Name,
					Stacks:        int32(ac.Stacks),
					Applied:       true,
				},
			},
		}); err != nil {
			return fmt.Errorf("sending condition event: %w", err)
		}
	}
	return nil
}

// handleRest processes the rest command for a player.
// Called pre-dispatch because it requires direct stream access to prompt the player.
//
// Precondition: uid identifies a valid player session.
// Postcondition: Routes to motel rest, camping rest, or legacy full rest depending on room danger level
// and available NPC/exploration state; sends a message to the player's stream.
func (s *GameServiceServer) handleRest(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("handleRest: player %q not found", uid)
	}

	sendMsg := func(text string) error {
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: text},
			},
		})
	}

	// Combat guard (REQ-REST-1).
	if sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT) {
		return sendMsg("You can't rest while in combat.")
	}

	// Already camping — inform the player.
	if sess.CampingActive {
		elapsed := time.Since(sess.CampingStartTime)
		remaining := sess.CampingDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		return sendMsg(fmt.Sprintf("You are already camping. Time remaining: %s.", remaining.Round(time.Second)))
	}

	// Determine room danger level for routing.
	var zoneDangerStr, roomDangerStr string
	if room, roomOK := s.world.GetRoom(sess.RoomID); roomOK {
		roomDangerStr = room.DangerLevel
		if zone, zoneOK := s.world.GetZone(room.ZoneID); zoneOK {
			zoneDangerStr = zone.DangerLevel
		}
	}
	effectiveDanger := danger.EffectiveDangerLevel(zoneDangerStr, roomDangerStr)

	// Rooms with explicit "safe" danger level require a motel NPC to rest.
	// Rooms with no danger level (empty) fall through to legacy full rest.
	// Rooms with non-safe danger level require exploration mode for camping.
	switch effectiveDanger {
	case danger.Safe:
		// REQ-REST-2: safe room + motel NPC → motel rest.
		if s.npcMgr != nil {
			for _, npcInst := range s.npcMgr.InstancesInRoom(sess.RoomID) {
				if npcInst.RestCost > 0 {
					return s.handleMotelRest(uid, sess, npcInst, sendMsg, requestID, stream)
				}
			}
		}
		// REQ-BR-4: safe room + brothel NPC → brothel rest.
		if s.npcMgr != nil {
			for _, npcInst := range s.npcMgr.InstancesInRoom(sess.RoomID) {
				if npcInst.NPCType == "brothel_keeper" {
					return s.handleBrothelRest(uid, sess, npcInst, sendMsg, requestID, stream)
				}
			}
		}
		// REQ-REST-4: safe room but no motel NPC.
		return sendMsg("There is no motel here to rest at.")
	case danger.Sketchy, danger.Dangerous, danger.AllOutWar:
		// REQ-REST-3: non-safe zone + active exploration mode → camping rest.
		if sess.ExploreMode != "" {
			return s.startCamping(uid, sess, sendMsg)
		}
		// REQ-REST-4: non-safe, no exploration mode.
		return sendMsg("You cannot rest here. You must be in exploration mode to camp in a dangerous area.")
	default:
		// DangerLevel is unset ("") — use legacy full rest for backward compatibility.
	}

	return s.applyFullLongRest(uid, sess, requestID, stream)
}

// handleMotelRest performs an instant full rest at a motel NPC (REQ-REST-2/5/6/7/9).
//
// Precondition: motelNPC.RestCost > 0 and room is safe; stream is non-nil.
// Postcondition: player HP + tech pools fully restored; motelNPC.RestCost deducted from Currency;
// if tech preparation requires player choice, a sentinel-encoded FeatureChoicePrompt is sent.
func (s *GameServiceServer) handleMotelRest(uid string, sess *session.PlayerSession, motelNPC *npc.Instance, sendMsg func(string) error, requestID string, stream gamev1.GameService_SessionServer) error {
	cost := motelNPC.RestCost
	if sess.Currency < cost {
		return sendMsg(fmt.Sprintf("A night here costs %d credits. You only have %d.", cost, sess.Currency))
	}
	sess.Currency -= cost
	ctx := stream.Context()
	if s.charSaver != nil {
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("handleMotelRest: failed to save currency", zap.Error(err))
		}
	}
	if err := sendMsg(fmt.Sprintf("You pay %d credits and settle in for the night.", cost)); err != nil {
		return err
	}
	if err := s.applyLongRestEffects(uid, sess, stream.Context(), sendMsg, stream); err != nil {
		return err
	}
	s.pushCharacterSheet(sess)
	return nil
}

// applyLongRestEffects applies the full long-rest restoration (HP, tech pools, durability)
// via applyFullLongRestCtx, building the tech-choice prompt from the given stream.
//
// Precondition: uid must be a valid player UID; sess, stream must not be nil.
// Postcondition: HP + tech pools fully restored; tech-choice prompt sent if needed.
func (s *GameServiceServer) applyLongRestEffects(uid string, sess *session.PlayerSession, ctx context.Context, sendMsg func(string) error, stream gamev1.GameService_SessionServer) error {
	headless := sess.Headless
	promptFn := func(prompt string, options []string, slotCtx *TechSlotContext) (string, error) {
		choices := &ruleset.FeatureChoices{
			Prompt:  prompt,
			Options: options,
			Key:     "tech_choice",
		}
		return s.promptTechSlotChoice(stream, "tech_choice", choices, slotCtx, headless)
	}
	return s.applyFullLongRestCtx(uid, sess, ctx, sendMsg, promptFn)
}

// handleBrothelRest performs an instant full rest at a brothel NPC (REQ-BR-6 through REQ-BR-13).
//
// Precondition: brothelNPC.Brothel != nil and room is safe; stream is non-nil.
// Postcondition: player HP + tech pools fully restored; RestCost deducted from Currency;
// flair_bonus_1 condition applied; disease and robbery rolled independently after rest.
func (s *GameServiceServer) handleBrothelRest(uid string, sess *session.PlayerSession, brothelNPC *npc.Instance, sendMsg func(string) error, requestID string, stream gamev1.GameService_SessionServer) error {
	brothelConfig := brothelNPC.Brothel
	ctx := stream.Context()

	// REQ-BR-6: insufficient credits → block rest.
	if sess.Currency < brothelConfig.RestCost {
		return sendMsg(fmt.Sprintf("A night here costs %d credits. You only have %d.", brothelConfig.RestCost, sess.Currency))
	}

	// REQ-BR-7: deduct credits, save, confirm, apply full rest.
	sess.Currency -= brothelConfig.RestCost
	if s.charSaver != nil {
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("handleBrothelRest: failed to save currency", zap.Error(err))
		}
	}
	if err := sendMsg(fmt.Sprintf("You pay %d credits and settle in for the night.", brothelConfig.RestCost)); err != nil {
		return err
	}
	if err := s.applyLongRestEffects(uid, sess, ctx, sendMsg, stream); err != nil {
		return err
	}

	// REQ-BR-8: apply flair_bonus_1 timed condition.
	if s.condRegistry != nil && sess.Conditions != nil {
		if def, ok := s.condRegistry.Get("flair_bonus_1"); ok {
			dur, parseErr := time.ParseDuration(brothelConfig.FlairBonusDur)
			if parseErr == nil {
				expiresAt := time.Now().Add(dur)
				if applyErr := sess.Conditions.ApplyTaggedWithExpiry(uid, def, 1, "brothel_rest", expiresAt); applyErr != nil {
					s.logger.Warn("handleBrothelRest: failed to apply flair_bonus_1", zap.Error(applyErr))
				} else {
					_ = sendMsg("You feel unusually confident. (+1 Flair)")
				}
			}
		}
	}

	// REQ-BR-9: independent disease roll. Rest completes before this (REQ-BR-12).
	if rand.Float64() < brothelConfig.DiseaseChance && len(brothelConfig.DiseasePool) > 0 {
		diseaseID := brothelConfig.DiseasePool[rand.Intn(len(brothelConfig.DiseasePool))]
		diseaseName := diseaseID
		if err := s.ApplySubstanceByID(uid, diseaseID); err != nil {
			s.logger.Warn("handleBrothelRest: ApplySubstanceByID failed", zap.String("substance", diseaseID), zap.Error(err))
		} else {
			_ = sendMsg("You've contracted " + diseaseName + ".")
		}
	}

	// REQ-BR-10: robbery gated by Awareness check vs zone-danger DC (REQ-BR-11, REQ-BR-14).
	// RobberyChance == 0 disables robbery entirely; otherwise the player's Awareness skill
	// is checked against the room's danger DC — failure or critical failure triggers robbery.
	var robbed bool
	if brothelConfig.RobberyChance > 0 {
		if room, roomOk := s.world.GetRoom(sess.RoomID); roomOk {
			dc := s.exploreDangerDC(room)
			outcome := s.exploreRoll(sess, "awareness", dc)
			robbed = outcome == skillcheck.Failure || outcome == skillcheck.CritFailure
		} else {
			// Room not found — default to robbery to preserve safe fallback.
			robbed = true
		}
	}
	if robbed {
		var robDetails []string
		// Steal crypto.
		if sess.Currency > 0 {
			stolenCrypto := sess.Currency * 5 / 100
			if stolenCrypto < 1 {
				stolenCrypto = 1
			}
			sess.Currency -= stolenCrypto
			if s.charSaver != nil {
				if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
					s.logger.Warn("handleBrothelRest: failed to save currency after robbery", zap.Error(err))
				}
			}
			robDetails = append(robDetails, fmt.Sprintf("%d crypto", stolenCrypto))
		}
		// Steal items.
		if sess.Backpack != nil {
			items := sess.Backpack.Items()
			numToSteal := len(items) * 5 / 100
			if numToSteal > 0 {
				// Pick random indices without replacement using Fisher-Yates partial shuffle.
				indices := make([]int, len(items))
				for i := range indices {
					indices[i] = i
				}
				for i := 0; i < numToSteal; i++ {
					j := i + rand.Intn(len(indices)-i)
					indices[i], indices[j] = indices[j], indices[i]
				}
				for i := 0; i < numToSteal; i++ {
					item := items[indices[i]]
					_ = sess.Backpack.Remove(item.InstanceID, 1)
					name := item.ItemDefID
					if s.invRegistry != nil {
						if def, ok := s.invRegistry.Item(item.ItemDefID); ok {
							name = def.Name
						}
					}
					robDetails = append(robDetails, name)
				}
				if s.charSaver != nil {
					invItems := backpackToInventoryItems(sess.Backpack)
					if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, invItems); err != nil {
						s.logger.Warn("handleBrothelRest: failed to save inventory after robbery", zap.Error(err))
					}
				}
			}
		}
		if len(robDetails) > 0 {
			stolen := strings.Join(robDetails, ", ")
			_ = sendMsg(fmt.Sprintf("You wake to find someone has gone through your belongings, taking: %s.", stolen))
		}
	}

	s.pushCharacterSheet(sess)
	return nil
}

// startCamping validates gear requirements and starts a camping rest session (REQ-REST-3/10-14/17).
//
// Precondition: sess.ExploreMode != "" and room is non-safe.
// Postcondition: sess.CampingActive = true; sess.CampingDuration set per gear count.
func (s *GameServiceServer) startCamping(uid string, sess *session.PlayerSession, sendMsg func(string) error) error {
	// REQ-REST-10: require sleeping_bag item.
	hasSleepingBag := len(sess.Backpack.FindByItemDefID("sleeping_bag")) > 0

	// REQ-REST-11: require fire_material-tagged item; count camping_gear items.
	hasFireMaterial := false
	campingGearCount := 0
	if sess.Backpack != nil {
		for _, item := range sess.Backpack.Items() {
			def, ok := s.invRegistry.Item(item.ItemDefID)
			if !ok {
				continue
			}
			if def.HasTag("fire_material") {
				hasFireMaterial = true
			}
			if def.HasTag("camping_gear") {
				campingGearCount += item.Quantity
			}
		}
	}

	// REQ-REST-12: collect all missing-gear errors.
	var missing []string
	if !hasSleepingBag {
		missing = append(missing, "a sleeping bag (sleeping_bag)")
	}
	if !hasFireMaterial {
		missing = append(missing, "fire-starting materials (fire_material)")
	}
	if len(missing) > 0 {
		return sendMsg(fmt.Sprintf("You need the following to camp: %s.", strings.Join(missing, "; ")))
	}

	// REQ-REST-13/14: compute duration (base 5min, -30s per camping_gear, min 2min).
	base := 5 * time.Minute
	reduction := time.Duration(campingGearCount) * 30 * time.Second
	campDuration := base - reduction
	if campDuration < 2*time.Minute {
		campDuration = 2 * time.Minute
	}

	// REQ-REST-17: gear NOT consumed.
	sess.CampingActive = true
	sess.CampingStartTime = time.Now()
	sess.CampingDuration = campDuration

	return sendMsg(fmt.Sprintf("You set up camp. You will rest for %s. Move to cancel early.", campDuration.Round(time.Second)))
}

// cancelCamping applies partial restoration and clears camping state (REQ-REST-15/16).
//
// Precondition: sess.CampingActive == true.
// Postcondition: sess.CampingActive = false; partial HP + tech restore applied.
func (s *GameServiceServer) cancelCamping(uid string, sess *session.PlayerSession, reason string) {
	elapsed := time.Since(sess.CampingStartTime)
	duration := sess.CampingDuration
	if duration == 0 {
		duration = time.Second
	}

	sess.CampingActive = false
	sess.CampingStartTime = time.Time{}
	sess.CampingDuration = 0

	frac := float64(elapsed) / float64(duration)
	if frac > 1.0 {
		frac = 1.0
	}

	// Partial HP restore.
	hpGain := int(math.Floor(frac * float64(sess.MaxHP-sess.CurrentHP)))
	if hpGain > 0 {
		sess.CurrentHP = sess.CurrentHP + hpGain
		if sess.CurrentHP > sess.MaxHP {
			sess.CurrentHP = sess.MaxHP
		}
		ctx := context.Background()
		if s.charSaver != nil {
			if err := s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, sess.CurrentHP); err != nil {
				s.logger.Warn("cancelCamping: failed to save HP", zap.Error(err))
			}
		}
	}

	// Partial tech pool restore.
	if s.spontaneousUsePoolRepo != nil {
		if err := s.spontaneousUsePoolRepo.RestorePartial(context.Background(), sess.CharacterID, frac); err != nil {
			s.logger.Warn("cancelCamping: failed to restore tech pools", zap.Error(err))
		}
	}

	s.pushMessageToUID(uid, fmt.Sprintf("Camping interrupted: %s. Partial restoration applied (%.0f%% complete).", reason, frac*100))
}

// checkCampingStatus is called on each game clock tick.
// Cancels camping if hostile NPCs are present (REQ-REST-15); completes if elapsed (REQ-REST-18).
//
// Precondition: uid refers to a connected player session.
// Postcondition: if camping complete, full long rest applied and CampingActive set false;
// if hostile NPC present, partial rest applied and CampingActive set false.
func (s *GameServiceServer) checkCampingStatus(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || !sess.CampingActive {
		return
	}

	// REQ-REST-15: cancel camping if hostile NPC is in the room.
	if s.npcMgr != nil {
		for _, npcInst := range s.npcMgr.InstancesInRoom(sess.RoomID) {
			if npcInst.Disposition == "hostile" {
				s.cancelCamping(uid, sess, "enemies spotted")
				return
			}
		}
	}

	// REQ-REST-18: apply full long rest when camping duration has elapsed.
	if time.Since(sess.CampingStartTime) >= sess.CampingDuration {
		sess.CampingActive = false
		sess.CampingStartTime = time.Time{}
		sess.CampingDuration = 0

		ctx := context.Background()
		sendMsg := func(text string) error {
			s.pushMessageToUID(uid, text)
			return nil
		}
		promptFn := func(_ string, options []string, _ *TechSlotContext) (string, error) {
			if len(options) == 0 {
				return "", nil
			}
			return options[0], nil
		}
		if err := s.applyFullLongRestCtx(uid, sess, ctx, sendMsg, promptFn); err != nil {
			if s.logger != nil {
				s.logger.Warn("checkCampingStatus: applyFullLongRestCtx failed", zap.String("uid", uid), zap.Error(err))
			}
		}
	}
}

// RefocusDuration is the real-time duration of a Refocus action.
// REQ-EXP-REFOCUS-1: 1 minute real time (proxy for 10 in-game minutes).
const RefocusDuration = 1 * time.Minute

// handleRefocus starts a Refocus action — a 1-minute out-of-combat rest that restores 1 Focus Point.
//
// Precondition: uid refers to a connected player session.
// Postcondition: sess.RefocusingActive = true and sess.RefocusingStartTime set, or error message sent.
func (s *GameServiceServer) handleRefocus(uid string) error {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("handleRefocus: player %q not found", uid)
	}

	// Block in combat.
	if sess.Status == statusInCombat {
		s.pushMessageToUID(uid, "You cannot refocus during combat.")
		return nil
	}

	// Already refocusing — show time remaining.
	if sess.RefocusingActive {
		elapsed := time.Since(sess.RefocusingStartTime)
		remaining := RefocusDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		s.pushMessageToUID(uid, fmt.Sprintf("You are already refocusing. Time remaining: %s.", remaining.Round(time.Second)))
		return nil
	}

	// Already at max focus points.
	if sess.MaxFocusPoints > 0 && sess.FocusPoints >= sess.MaxFocusPoints {
		s.pushMessageToUID(uid, "Your focus is already at its maximum.")
		return nil
	}

	// No focus pool to refill.
	if sess.MaxFocusPoints == 0 {
		s.pushMessageToUID(uid, "You have no focus pool to refill.")
		return nil
	}

	sess.RefocusingActive = true
	sess.RefocusingStartTime = time.Now()
	s.pushMessageToUID(uid, fmt.Sprintf("You take a moment to refocus. You will restore 1 Focus Point in %s.", RefocusDuration.Round(time.Second)))
	return nil
}

// checkRefocusStatus is called on each game clock tick.
// Completes refocus if 1 minute has elapsed, restoring 1 Focus Point.
//
// Precondition: uid refers to a connected player session.
// Postcondition: if elapsed, FocusPoints increased by 1 (capped at MaxFocusPoints) and persisted.
func (s *GameServiceServer) checkRefocusStatus(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || !sess.RefocusingActive {
		return
	}

	// Cancel refocus if a hostile NPC enters the room (mirrors camping cancel behavior).
	if s.npcMgr != nil {
		for _, npcInst := range s.npcMgr.InstancesInRoom(sess.RoomID) {
			if npcInst.Disposition == "hostile" {
				sess.RefocusingActive = false
				sess.RefocusingStartTime = time.Time{}
				s.pushMessageToUID(uid, "Your focus is broken — enemies are nearby!")
				return
			}
		}
	}

	if time.Since(sess.RefocusingStartTime) < RefocusDuration {
		return
	}

	sess.RefocusingActive = false
	sess.RefocusingStartTime = time.Time{}

	if sess.FocusPoints < sess.MaxFocusPoints {
		sess.FocusPoints++
		if s.charSaver != nil {
			if err := s.charSaver.SaveFocusPoints(context.Background(), sess.CharacterID, sess.FocusPoints); err != nil {
				if s.logger != nil {
					s.logger.Warn("checkRefocusStatus: failed to save focus points", zap.Error(err))
				}
			}
		}
		s.pushMessageToUID(uid, fmt.Sprintf("You feel your focus restore. (%d/%d Focus Points)", sess.FocusPoints, sess.MaxFocusPoints))
	} else {
		s.pushMessageToUID(uid, "You feel centered but your focus was already full.")
	}
}

// applyFullLongRest applies full long-rest restoration via the player's stream (REQ-REST-9/18).
//
// Precondition: sess is a valid player session; stream is non-nil.
// Postcondition: HP at max; tech pools fully restored; prepared techs rearranged.
func (s *GameServiceServer) applyFullLongRest(uid string, sess *session.PlayerSession, requestID string, stream gamev1.GameService_SessionServer) error {
	sendMsg := func(text string) error {
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: text},
			},
		})
	}
	headless := sess.Headless
	promptFn := func(prompt string, options []string, slotCtx *TechSlotContext) (string, error) {
		choices := &ruleset.FeatureChoices{
			Prompt:  prompt,
			Options: options,
			Key:     "tech_choice",
		}
		return s.promptTechSlotChoice(stream, "tech_choice", choices, slotCtx, headless)
	}
	return s.applyFullLongRestCtx(uid, sess, stream.Context(), sendMsg, promptFn)
}

// applyFullLongRestCtx performs full long-rest restoration with a caller-supplied context, send function,
// and prompt function for interactive tech selection.
//
// Precondition: sess is a valid player session; ctx, sendMsg, and promptFn are non-nil.
// Postcondition: HP at max; tech pools fully restored; prepared techs rearranged.
func (s *GameServiceServer) applyFullLongRestCtx(uid string, sess *session.PlayerSession, ctx context.Context, sendMsg func(string) error, promptFn TechPromptFn) error {
	// REQ-LR1: Restore HP to maximum.
	sess.CurrentHP = sess.MaxHP

	// REQ-EM-16: Restore all equipped items to full MaxDurability during rest.
	if sess.LoadoutSet != nil {
		for _, preset := range sess.LoadoutSet.Presets {
			if preset.MainHand != nil {
				inst := &inventory.ItemInstance{Durability: preset.MainHand.Durability, MaxDurability: preset.MainHand.Durability}
				if rd, ok := inventory.LookupRarity(preset.MainHand.Def.Rarity); ok {
					inst.MaxDurability = rd.MaxDurability
				}
				inventory.RepairFull(inst)
				preset.MainHand.Durability = inst.Durability
			}
			if preset.OffHand != nil {
				inst := &inventory.ItemInstance{Durability: preset.OffHand.Durability, MaxDurability: preset.OffHand.Durability}
				if rd, ok := inventory.LookupRarity(preset.OffHand.Def.Rarity); ok {
					inst.MaxDurability = rd.MaxDurability
				}
				inventory.RepairFull(inst)
				preset.OffHand.Durability = inst.Durability
			}
		}
	}
	if sess.Equipment != nil {
		for _, si := range sess.Equipment.Armor {
			if si == nil {
				continue
			}
			inst := &inventory.ItemInstance{Durability: si.Durability, MaxDurability: si.Durability}
			if rd, ok := inventory.LookupRarity(si.Rarity); ok {
				inst.MaxDurability = rd.MaxDurability
			}
			inventory.RepairFull(inst)
			si.Durability = inst.Durability
		}
	}

	// REQ-LR2: Persist HP to database.
	if s.charSaver != nil {
		if err := s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, sess.CurrentHP); err != nil {
			return fmt.Errorf("applyFullLongRest: save HP: %w", err)
		}
	}

	// Restore spontaneous use pools unconditionally (influencer characters).
	if s.spontaneousUsePoolRepo != nil {
		if err := s.spontaneousUsePoolRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
			return fmt.Errorf("applyFullLongRest: restore spontaneous use pools: %w", err)
		}
		pools, err := s.spontaneousUsePoolRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return fmt.Errorf("applyFullLongRest: reload spontaneous use pools: %w", err)
		}
		sess.SpontaneousUsePools = pools
	}

	// Restore innate tech use slots.
	if s.innateTechRepo != nil {
		if err := s.innateTechRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
			return fmt.Errorf("applyFullLongRest: restore innate slots: %w", err)
		}
		innates, err := s.innateTechRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return fmt.Errorf("applyFullLongRest: reload innate slots: %w", err)
		}
		sess.InnateTechs = innates
	}

	// Restore active feat uses to their PreparedUses maximum.
	if s.characterFeatsRepo != nil && s.featRegistry != nil && sess.ActiveFeatUses != nil {
		if featIDs, featErr := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID); featErr != nil {
			s.logger.Warn("applyFullLongRest: failed to load feats for use restoration", zap.Error(featErr))
		} else {
			for _, id := range featIDs {
				f, ok := s.featRegistry.Feat(id)
				if !ok || !f.Active || f.PreparedUses <= 0 {
					continue
				}
				sess.ActiveFeatUses[id] = f.PreparedUses
			}
		}
	}

	// Job lookup.
	if s.jobRegistry == nil {
		return sendMsg("You rest and recover to full HP.")
	}
	job, ok := s.jobRegistry.Job(sess.Class)
	if !ok {
		return sendMsg("You rest and recover to full HP.")
	}

	restFlavor := technology.FlavorFor(technology.DominantTradition(sess.Class))
	sendFn := func(text string) {
		_ = sendMsg(text)
	}
	var rearrangeArchetype *ruleset.Archetype
	if s.archetypes != nil && job.Archetype != "" {
		if arch, ok := s.archetypes[job.Archetype]; ok {
			rearrangeArchetype = arch
		}
	}
	if err := RearrangePreparedTechs(ctx, sess, sess.CharacterID,
		job, rearrangeArchetype, s.techRegistry, promptFn, s.preparedTechRepo,
		sendFn, restFlavor,
	); err != nil {
		s.logger.Warn("applyFullLongRest: RearrangePreparedTechs failed",
			zap.String("uid", uid),
			zap.Error(err))
		return sendMsg("Something went wrong preparing your technologies.")
	}

	s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
	s.pushCharacterSheet(sess)
	return sendMsg(fmt.Sprintf("You finish your rest. HP restored to maximum. %s", restFlavor.RestMessage))
}

// handleSelectTech processes the selecttech command for a player.
// Called pre-dispatch because it requires direct stream access to prompt the player.
//
// Precondition: uid identifies a valid player session.
// Postcondition: If pending tech grants exist, they are resolved interactively;
// a confirmation or error message is sent to the player's stream.
func (s *GameServiceServer) handleSelectTech(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("handleSelectTech: player %q not found", uid)
	}

	sendMsg := func(text string) error {
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: text},
			},
		})
	}

	if len(sess.PendingTechGrants) == 0 {
		return sendMsg("You have no pending technology selections.")
	}

	if s.jobRegistry == nil {
		return sendMsg("You have no pending technology selections.")
	}
	job, ok := s.jobRegistry.Job(sess.Class)
	if !ok {
		return sendMsg("You have no pending technology selections.")
	}

	headlessFlag := sess.Headless
	promptFn := func(prompt string, options []string, slotCtx *TechSlotContext) (string, error) {
		choices := &ruleset.FeatureChoices{
			Prompt:  prompt,
			Options: options,
			Key:     "tech_choice",
		}
		return s.promptTechSlotChoice(stream, "tech_choice", choices, slotCtx, headlessFlag)
	}

	if err := ResolvePendingTechGrants(stream.Context(), sess, sess.CharacterID,
		job, s.techRegistry, promptFn,
		s.hardwiredTechRepo, s.preparedTechRepo,
		s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
		s.progressRepo,
	); err != nil {
		s.logger.Warn("handleSelectTech failed", zap.String("uid", uid), zap.Error(err))
		return sendMsg("Something went wrong selecting your technologies.")
	}

	return sendMsg("Your technology selections are complete.")
}

// pushMessageToUID sends a plain-text MessageEvent to a single player session identified by uid.
//
// Precondition: uid must be non-empty and the session must exist.
// Postcondition: If the session exists and has a non-nil Entity, the message is pushed; errors are logged.
func (s *GameServiceServer) pushMessageToUID(uid, text string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: text},
		},
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Error("pushMessageToUID: marshal failed", zap.String("uid", uid), zap.Error(err))
		return
	}
	if pushErr := sess.Entity.Push(data); pushErr != nil {
		s.logger.Warn("pushMessageToUID: push failed", zap.String("uid", uid), zap.Error(pushErr))
	}
}

// pushEventToUID sends a raw ServerEvent to a single player session identified by uid.
//
// Precondition: uid must be non-empty; evt must be non-nil.
// Postcondition: If the session exists and has a non-nil Entity, the event is pushed; errors are logged.
func (s *GameServiceServer) pushEventToUID(uid string, evt *gamev1.ServerEvent) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Error("pushEventToUID: marshal failed", zap.String("uid", uid), zap.Error(err))
		return
	}
	if pushErr := sess.Entity.Push(data); pushErr != nil {
		s.logger.Warn("pushEventToUID: push failed", zap.String("uid", uid), zap.Error(pushErr))
	}
}

// handleGroup handles the group command.
//
// Precondition: uid must identify an existing player session.
// Postcondition: Without args, displays group membership or an error. With args, creates a group and
//
//	sends an invitation to the named player, or returns an appropriate error message.
func (s *GameServiceServer) handleGroup(uid string, req *gamev1.GroupRequest) (*gamev1.ServerEvent, error) {
	caller, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if req.Args == "" {
		// Display current group membership.
		if caller.GroupID == "" {
			return messageEvent("You are not in a group."), nil
		}
		g, exists := s.sessions.GroupByID(caller.GroupID)
		if !exists {
			return messageEvent("You are not in a group."), nil
		}
		// Find leader name.
		leaderName := g.LeaderUID
		if leaderSess, ok := s.sessions.GetPlayer(g.LeaderUID); ok {
			leaderName = leaderSess.CharName
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Group (leader: %s):", leaderName)
		for _, memberUID := range g.MemberUIDs {
			if ms, ok := s.sessions.GetPlayer(memberUID); ok {
				fmt.Fprintf(&sb, "\n  %s", ms.CharName)
			}
		}
		return messageEvent(sb.String()), nil
	}

	// Create group and invite the named player.
	if caller.GroupID != "" {
		return messageEvent("You are already in a group. Use 'ungroup' to leave first."), nil
	}
	targetName := req.Args
	if strings.EqualFold(targetName, caller.CharName) {
		return messageEvent("You cannot invite yourself."), nil
	}
	target := s.sessions.GetPlayerByCharNameCI(targetName)
	if target == nil {
		return messageEvent("Player not found."), nil
	}
	if target.GroupID != "" {
		return messageEvent(fmt.Sprintf("%s is already in a group.", target.CharName)), nil
	}
	if target.PendingGroupInvite != "" {
		return messageEvent(fmt.Sprintf("%s already has a pending group invitation.", target.CharName)), nil
	}

	g := s.sessions.CreateGroup(uid)
	s.sessions.SetPendingGroupInvite(target.UID, g.ID)
	s.pushMessageToUID(target.UID, fmt.Sprintf(
		"%s has invited you to join their group. (accept / decline)", caller.CharName,
	))
	return messageEvent(fmt.Sprintf("You created a group and invited %s.", target.CharName)), nil
}

// handleInvite handles the invite command.
//
// Precondition: uid must identify an existing player session.
// Postcondition: If all preconditions pass, the target player's PendingGroupInvite is set and they
//
//	receive a notification; the caller receives a confirmation message.
func (s *GameServiceServer) handleInvite(uid string, req *gamev1.InviteRequest) (*gamev1.ServerEvent, error) {
	caller, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if caller.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}
	g, exists := s.sessions.GroupByID(caller.GroupID)
	if !exists {
		return messageEvent("You are not in a group."), nil
	}
	if g.LeaderUID != uid {
		return messageEvent("Only the group leader can invite players."), nil
	}
	targetName := req.Player
	if strings.EqualFold(targetName, caller.CharName) {
		return messageEvent("You cannot invite yourself."), nil
	}
	target := s.sessions.GetPlayerByCharNameCI(targetName)
	if target == nil {
		return messageEvent("Player not found."), nil
	}
	if target.GroupID != "" {
		return messageEvent(fmt.Sprintf("%s is already in a group.", target.CharName)), nil
	}
	if target.PendingGroupInvite != "" {
		return messageEvent(fmt.Sprintf("%s already has a pending group invitation.", target.CharName)), nil
	}
	if len(g.MemberUIDs) >= 8 {
		return messageEvent("Group is full (max 8 members)."), nil
	}

	s.sessions.SetPendingGroupInvite(target.UID, g.ID)
	s.pushMessageToUID(target.UID, fmt.Sprintf(
		"%s has invited you to join their group. (accept / decline)", caller.CharName,
	))
	return messageEvent(fmt.Sprintf("You invited %s to the group.", target.CharName)), nil
}

// handleAcceptGroup handles the accept command for group invitations.
//
// Precondition: uid must identify an existing player session.
// Postcondition: If a valid pending invite exists, the player is added to the group, PendingGroupInvite
//
//	is cleared, existing members are notified, and a confirmation is returned.
func (s *GameServiceServer) handleAcceptGroup(uid string, _ *gamev1.AcceptGroupRequest) (*gamev1.ServerEvent, error) {
	caller, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if caller.PendingGroupInvite == "" {
		return messageEvent("You have no pending group invitation."), nil
	}
	groupID := caller.PendingGroupInvite
	g, exists := s.sessions.GroupByID(groupID)
	if !exists {
		s.sessions.SetPendingGroupInvite(uid, "")
		return messageEvent("That group no longer exists."), nil
	}

	if err := s.sessions.AddGroupMember(groupID, uid); err != nil {
		s.sessions.SetPendingGroupInvite(uid, "")
		return messageEvent("The group is full."), nil
	}
	s.sessions.SetPendingGroupInvite(uid, "")

	// Notify existing members (all except the new joiner).
	for _, memberUID := range g.MemberUIDs {
		if memberUID != uid {
			s.pushMessageToUID(memberUID, fmt.Sprintf("%s joined the group.", caller.CharName))
		}
	}

	// Find leader name for confirmation message.
	leaderName := g.LeaderUID
	if leaderSess, ok := s.sessions.GetPlayer(g.LeaderUID); ok {
		leaderName = leaderSess.CharName
	}
	return messageEvent(fmt.Sprintf("You joined %s's group.", leaderName)), nil
}

// handleDeclineGroup handles the decline command for group invitations.
//
// Precondition: uid must identify an existing player session.
// Postcondition: PendingGroupInvite is cleared; the group leader (if online) is notified; a
//
//	confirmation message is returned to the caller.
func (s *GameServiceServer) handleDeclineGroup(uid string, _ *gamev1.DeclineGroupRequest) (*gamev1.ServerEvent, error) {
	caller, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if caller.PendingGroupInvite == "" {
		return messageEvent("You have no pending group invitation."), nil
	}
	groupID := caller.PendingGroupInvite
	s.sessions.SetPendingGroupInvite(uid, "")

	if g, exists := s.sessions.GroupByID(groupID); exists {
		s.pushMessageToUID(g.LeaderUID, fmt.Sprintf(
			"%s declined your group invitation.", caller.CharName,
		))
	}
	return messageEvent("You declined the group invitation."), nil
}

// handleUngroup handles the 'ungroup' command.
// Non-leaders leave the group; the leader disbands it entirely.
//
// Precondition: uid must identify an existing player session.
// Postcondition: Leader disbands group (all members' GroupID cleared, group removed);
// non-leader's GroupID is cleared; remaining members and/or disbanded members are notified.
func (s *GameServiceServer) handleUngroup(uid string, _ *gamev1.UngroupRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}
	group, exists := s.sessions.GroupByID(sess.GroupID)
	if !exists {
		sess.GroupID = ""
		return messageEvent("You are not in a group."), nil
	}

	if group.LeaderUID == uid {
		// Leader path: disband — copy MemberUIDs BEFORE DisbandGroup modifies the slice.
		disbandMsg := fmt.Sprintf("The group has been disbanded by %s.", sess.CharName)
		memberUIDs := make([]string, len(group.MemberUIDs))
		copy(memberUIDs, group.MemberUIDs)
		s.sessions.DisbandGroup(group.ID)
		for _, memberUID := range memberUIDs {
			if memberUID == uid {
				continue
			}
			s.pushMessageToUID(memberUID, disbandMsg)
		}
		return messageEvent("You disbanded the group."), nil
	}

	// Non-leader path: leave — copy MemberUIDs BEFORE RemoveGroupMember modifies the slice.
	memberUIDs := make([]string, len(group.MemberUIDs))
	copy(memberUIDs, group.MemberUIDs)
	s.sessions.RemoveGroupMember(group.ID, uid)
	leftMsg := fmt.Sprintf("%s left the group.", sess.CharName)
	for _, memberUID := range memberUIDs {
		if memberUID == uid {
			continue
		}
		s.pushMessageToUID(memberUID, leftMsg)
	}
	return messageEvent("You left the group."), nil
}

// handleKick handles the 'kick <player>' command.
// Leader-only: removes a named member from the group.
//
// Precondition: uid must identify an existing player session; req.Player must be the target's CharName.
// Postcondition: Target's GroupID is cleared and removed from MemberUIDs; target and remaining members are notified.
func (s *GameServiceServer) handleKick(uid string, req *gamev1.KickRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}
	group, exists := s.sessions.GroupByID(sess.GroupID)
	if !exists {
		sess.GroupID = ""
		return messageEvent("You are not in a group."), nil
	}
	if group.LeaderUID != uid {
		return messageEvent("Only the group leader can kick members."), nil
	}

	arg := strings.TrimSpace(req.GetPlayer())

	// Find target among group members.
	var targetSess *session.PlayerSession
	for _, memberUID := range group.MemberUIDs {
		if mSess, ok2 := s.sessions.GetPlayer(memberUID); ok2 {
			if strings.EqualFold(mSess.CharName, arg) {
				targetSess = mSess
				break
			}
		}
	}
	if targetSess == nil {
		return messageEvent(fmt.Sprintf("%s is not in your group.", arg)), nil
	}

	if targetSess.UID == uid {
		return messageEvent("Use 'ungroup' to disband the group."), nil
	}

	// Capture remaining UIDs BEFORE removal so we can notify them.
	remainingUIDs := make([]string, 0, len(group.MemberUIDs))
	for _, memberUID := range group.MemberUIDs {
		if memberUID != targetSess.UID {
			remainingUIDs = append(remainingUIDs, memberUID)
		}
	}

	s.sessions.RemoveGroupMember(group.ID, targetSess.UID)
	s.pushMessageToUID(targetSess.UID, "You were kicked from the group.")

	kickedMsg := fmt.Sprintf("%s was kicked from the group.", targetSess.CharName)
	for _, memberUID := range remainingUIDs {
		s.pushMessageToUID(memberUID, kickedMsg)
	}

	return messageEvent(fmt.Sprintf("You kicked %s from the group.", targetSess.CharName)), nil
}

// cleanupPlayer removes a player from the session manager, persists character state, and broadcasts departure.
//
// Precondition: uid must be non-empty; myEntity must be the BridgeEntity allocated for this session.
// Postcondition: If the session in the registry is still the one identified by myEntity, it is removed
// and character state is saved. If the session was evicted by a rapid reconnect (different entity),
// this is a no-op to avoid disrupting the new session.
func (s *GameServiceServer) cleanupPlayer(uid, username string, myEntity *session.BridgeEntity) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	// Guard against stale cleanup: if the session in the registry has a different entity,
	// a rapid reconnect already evicted our session and created a new one. Skip cleanup
	// so we don't remove the replacement session.
	if sess.Entity != myEntity {
		s.logger.Debug("cleanupPlayer: session was replaced by reconnect, skipping stale cleanup",
			zap.String("uid", uid))
		return
	}
	roomID := sess.RoomID
	characterID := sess.CharacterID
	currentHP := sess.CurrentHP
	charName := sess.CharName

	// Clear pending group invite on disconnect.
	if sess.PendingGroupInvite != "" {
		if grp, ok := s.sessions.GroupByID(sess.PendingGroupInvite); ok {
			s.pushMessageToUID(grp.LeaderUID, fmt.Sprintf(
				"%s disconnected before responding to your invitation.", sess.CharName,
			))
		}
		s.sessions.SetPendingGroupInvite(sess.UID, "")
	}

	// Handle group membership on disconnect.
	if sess.GroupID != "" {
		if grp, ok := s.sessions.GroupByID(sess.GroupID); ok {
			if grp.LeaderUID == sess.UID {
				// Leader disconnecting — copy members, disband, notify.
				memberUIDs := make([]string, len(grp.MemberUIDs))
				copy(memberUIDs, grp.MemberUIDs)
				s.sessions.DisbandGroup(grp.ID)
				for _, memberUID := range memberUIDs {
					if memberUID == sess.UID {
						continue
					}
					s.pushMessageToUID(memberUID, fmt.Sprintf(
						"%s disconnected. The group has been disbanded.", sess.CharName,
					))
				}
			} else {
				// Non-leader disconnecting — remove from group, notify remaining.
				remainingUIDs := make([]string, 0, len(grp.MemberUIDs))
				for _, mUID := range grp.MemberUIDs {
					if mUID != sess.UID {
						remainingUIDs = append(remainingUIDs, mUID)
					}
				}
				s.sessions.RemoveGroupMember(grp.ID, sess.UID)
				for _, mUID := range remainingUIDs {
					s.pushMessageToUID(mUID, fmt.Sprintf(
						"%s disconnected and left the group.", sess.CharName,
					))
				}
			}
		}
	}

	if err := s.sessions.RemovePlayer(uid); err != nil {
		s.logger.Warn("removing player on cleanup", zap.String("uid", uid), zap.Error(err))
	}

	// Persist character state on disconnect.
	if characterID > 0 && s.charSaver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(ctx, characterID, roomID, currentHP); err != nil {
			s.logger.Warn("saving character state on disconnect",
				zap.String("uid", uid),
				zap.Int64("character_id", characterID),
				zap.Error(err),
			)
		} else {
			s.logger.Info("character state saved",
				zap.Int64("character_id", characterID),
				zap.String("room", roomID),
			)
		}

		if sess.LoadoutSet != nil {
			if err := s.charSaver.SaveWeaponPresets(ctx, characterID, sess.LoadoutSet); err != nil {
				s.logger.Warn("saving weapon presets on disconnect",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(err),
				)
			}
		}

		if sess.Equipment != nil {
			if err := s.charSaver.SaveEquipment(ctx, characterID, sess.Equipment); err != nil {
				s.logger.Warn("saving equipment on disconnect",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(err),
				)
			}
		}

		if sess.Backpack != nil {
			invItems := backpackToInventoryItems(sess.Backpack)
			if err := s.charSaver.SaveInventory(ctx, characterID, invItems); err != nil {
				s.logger.Error("failed to save inventory on disconnect",
					zap.String("uid", uid),
					zap.Int64("character_id", characterID),
					zap.Error(err),
				)
			}
		}
	}

	s.broadcastRoomEvent(roomID, uid, &gamev1.RoomEvent{
		Player: charName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	s.logger.Info("player disconnected",
		zap.String("uid", uid),
		zap.String("username", username),
		zap.Int64("character_id", characterID),
	)
}

// grantStartingInventory grants the starting kit for a new character and auto-equips it.
//
// Precondition: sess must be non-nil; characterID must be > 0; loadoutsDir must be non-empty.
// Postcondition: Backpack populated, weapon equipped, armor equipped in slots,
// currency set, inventory saved, flag marked. Returns nil on success.
func (s *GameServiceServer) grantStartingInventory(ctx context.Context, sess *session.PlayerSession, characterID int64, archetype, team string, jobOverride *inventory.StartingLoadoutOverride) error {
	sl, err := inventory.LoadStartingLoadoutWithOverride(s.loadoutsDir, archetype, team, jobOverride)
	if err != nil {
		return fmt.Errorf("resolving starting loadout: %w", err)
	}

	// Add weapon to backpack then equip in main hand.
	if sl.Weapon != "" {
		if _, addErr := sess.Backpack.Add(sl.Weapon, 1, s.invRegistry); addErr != nil {
			s.logger.Warn("failed to add starting weapon", zap.String("item", sl.Weapon), zap.Error(addErr))
		} else {
			command.HandleEquip(sess, s.invRegistry, sl.Weapon+" main", 0)
		}
	}

	// Add and wear armor.
	for slot, itemID := range sl.Armor {
		if _, addErr := sess.Backpack.Add(itemID, 1, s.invRegistry); addErr != nil {
			s.logger.Warn("failed to add starting armor", zap.String("item", itemID), zap.Error(addErr))
			continue
		}
		command.HandleWear(sess, s.invRegistry, itemID+" "+string(slot))
	}

	// Add consumables.
	for _, cg := range sl.Consumables {
		if _, addErr := sess.Backpack.Add(cg.ItemID, cg.Quantity, s.invRegistry); addErr != nil {
			s.logger.Warn("failed to add starting consumable", zap.String("item", cg.ItemID), zap.Error(addErr))
		}
	}

	// Set currency.
	sess.Currency = sl.Currency

	// Persist inventory.
	items := backpackToInventoryItems(sess.Backpack)
	if err := s.charSaver.SaveInventory(ctx, characterID, items); err != nil {
		return fmt.Errorf("saving starting inventory: %w", err)
	}

	// Persist equipment (armor slots worn during starting grant).
	if err := s.charSaver.SaveEquipment(ctx, characterID, sess.Equipment); err != nil {
		return fmt.Errorf("saving starting equipment: %w", err)
	}

	// Persist weapon presets (weapon equipped during starting grant).
	if err := s.charSaver.SaveWeaponPresets(ctx, characterID, sess.LoadoutSet); err != nil {
		return fmt.Errorf("saving starting weapon presets: %w", err)
	}

	// Mark flag.
	if err := s.charSaver.MarkStartingInventoryGranted(ctx, characterID); err != nil {
		return fmt.Errorf("marking starting inventory granted: %w", err)
	}

	return nil
}

// backpackToInventoryItems converts backpack contents to a deduplicated slice of InventoryItem.
//
// Precondition: bp must be non-nil.
// Postcondition: Returns a slice with one entry per unique item def ID, summing quantities.
func backpackToInventoryItems(bp *inventory.Backpack) []inventory.InventoryItem {
	instances := bp.Items()
	counts := make(map[string]int, len(instances))
	for _, inst := range instances {
		counts[inst.ItemDefID] += inst.Quantity
	}
	out := make([]inventory.InventoryItem, 0, len(counts))
	for id, qty := range counts {
		out = append(out, inventory.InventoryItem{ItemDefID: id, Quantity: qty})
	}
	return out
}

// errorEvent builds a ServerEvent carrying an ErrorEvent with the given message.
//
// Precondition: msg must be non-empty.
// Postcondition: Returns a non-nil ServerEvent.
// armorProfCategoryLabel converts an ArmorDef.ProficiencyCategory value like
// "light_armor" into a short display label ("light", "medium", "heavy").
// buildConsumableEffectsSummary returns a human-readable one-liner describing
// the effects of a consumable ItemDef, used to populate tooltip text in the
// shop and inventory UIs. Falls back to the item description if no structured
// effects are configured.
func buildConsumableEffectsSummary(def *inventory.ItemDef) string {
	if def == nil {
		return ""
	}
	var parts []string
	if def.Effect != nil {
		if def.Effect.Heal != "" {
			parts = append(parts, "Heal: "+def.Effect.Heal)
		}
		for _, c := range def.Effect.Conditions {
			parts = append(parts, "Applies "+c.ConditionID+" ("+c.Duration+")")
		}
		for _, r := range def.Effect.RemoveConditions {
			parts = append(parts, "Removes: "+r)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	return def.Description
}

func armorProfCategoryLabel(category string) string {
	switch category {
	case "light_armor":
		return "light"
	case "medium_armor":
		return "medium"
	case "heavy_armor":
		return "heavy"
	default:
		return ""
	}
}

func errorEvent(msg string) *gamev1.ServerEvent {
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Error{
			Error: &gamev1.ErrorEvent{Message: msg},
		},
	}
}

// messageEvent builds a ServerEvent carrying a MessageEvent with the given text.
//
// Precondition: text must be non-empty.
// Postcondition: Returns a non-nil ServerEvent.
func messageEvent(text string) *gamev1.ServerEvent {
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: text},
		},
	}
}

// buildConsumableResultMsg constructs a player-facing message describing the effects
// that were applied from using a consumable item.
//
// Precondition: itemName must be non-empty.
// Postcondition: Returns a non-empty string describing the outcome.
func buildConsumableResultMsg(itemName string, r inventory.ConsumableResult) string {
	var parts []string
	if r.HealApplied > 0 {
		parts = append(parts, fmt.Sprintf("You recover %d HP.", r.HealApplied))
	}
	for _, cid := range r.ConditionsRemoved {
		parts = append(parts, fmt.Sprintf("Removed condition: %s.", cid))
	}
	for _, cid := range r.ConditionsApplied {
		parts = append(parts, fmt.Sprintf("Applied condition: %s.", cid))
	}
	if r.DiseaseApplied != "" {
		parts = append(parts, fmt.Sprintf("Contracted disease: %s.", r.DiseaseApplied))
	}
	if r.ToxinApplied != "" {
		parts = append(parts, fmt.Sprintf("Exposed to toxin: %s.", r.ToxinApplied))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("You use the %s.", itemName)
	}
	return fmt.Sprintf("You use the %s. %s", itemName, strings.Join(parts, " "))
}

// handleEquip equips a weapon from the player's backpack into the named slot.
//
// Precondition: uid identifies an active session; req is non-nil.
// Postcondition: on success, sess.LoadoutSet is updated and persisted; on failure an error event is returned.
func (s *GameServiceServer) handleEquip(uid string, req *gamev1.EquipRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	arg := req.GetWeaponId()
	if slot := req.GetSlot(); slot != "" {
		arg += " " + slot
	}
	result := command.HandleEquip(sess, s.invRegistry, arg, int(req.GetPreset()))
	if result == "" || result == "Usage: equip <item_id> <main|off>" || result == "specify main or off" {
		return errorEvent(result), nil
	}
	// Persist the updated loadout if we have a character saver.
	if s.charSaver != nil && sess.LoadoutSet != nil {
		if err := s.charSaver.SaveWeaponPresets(context.Background(), sess.CharacterID, sess.LoadoutSet); err != nil {
			s.logger.Warn("failed to persist weapon presets after equip", zap.Error(err))
		}
	}
	// Also update the combat handler's loadout cache if in combat.
	if s.combatH != nil {
		_, _ = s.combatH.Equip(uid, req.GetWeaponId(), req.GetSlot())
	}
	// Push updated inventory (item removed from backpack), loadout (weapon now in slot),
	// and character sheet (main/off hand displayed in equipment drawer) immediately
	// so the web UI refreshes all panels without requiring re-login (REQ-UI-EQUIP-1, REQ-88-1).
	s.pushInventory(sess)
	s.pushLoadout(sess)
	s.pushCharacterSheet(sess)
	return messageEvent(result), nil
}

// handleWear equips an armor item from the player's backpack into the specified body slot.
//
// Precondition: uid must be a valid connected player; req must be non-nil.
// Postcondition: On success returns a message event; on error returns an error event.
func (s *GameServiceServer) handleWear(uid string, req *gamev1.WearRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	arg := req.GetItemId() + " " + req.GetSlot()
	result := command.HandleWear(sess, s.invRegistry, arg)

	// REQ-EM-29: recompute set bonuses whenever armor changes.
	if strings.HasPrefix(result, "Wore ") {
		session.RecomputeSetBonuses(sess, s.setRegistry)
		if sess.LoadoutSet != nil && sess.Equipment != nil {
			if active := sess.LoadoutSet.ActivePreset(); active != nil {
				equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
				sess.PassiveMaterials = inventory.ComputePassiveMaterials(equipped, sess.Equipment.Armor, s.invRegistry)
			}
		}
	}

	// Only apply team affinity effect if the wear succeeded.
	if strings.HasPrefix(result, "Wore ") && s.jobRegistry != nil && s.invRegistry != nil {
		itemDef, ok := s.invRegistry.Item(req.GetItemId())
		if ok && itemDef.ArmorRef != "" {
			if armorDef, ok := s.invRegistry.Armor(itemDef.ArmorRef); ok && armorDef.TeamAffinity != "" {
				playerTeam := s.jobRegistry.TeamFor(sess.Class)
				if playerTeam != "" && playerTeam != armorDef.TeamAffinity {
					if armorDef.CrossTeamEffect != nil {
						s.applyEquipEffect(uid, armorDef.CrossTeamEffect)
					}
				}
			}
		}
	}

	// Persist and refresh UI after a successful wear.
	if strings.HasPrefix(result, "Wore ") {
		ctx := context.Background()
		invItems := backpackToInventoryItems(sess.Backpack)
		if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, invItems); err != nil {
			s.logger.Error("handleWear: SaveInventory failed", zap.String("uid", uid), zap.Error(err))
		}
		if err := s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment); err != nil {
			s.logger.Error("handleWear: SaveEquipment failed", zap.String("uid", uid), zap.Error(err))
		}
		// REQ-BUG101-1: push inventory refresh after wear so the web UI removes the item.
		// REQ-88-1: push character sheet after wear so the equipment drawer reflects the new armor.
		if sess2, ok2 := s.sessions.GetPlayer(uid); ok2 {
			s.pushInventory(sess2)
			s.pushCharacterSheet(sess2)
		}
	}
	return messageEvent(result), nil
}

// handleRemoveArmor removes armor from a player's body slot, returning it to the backpack.
//
// Precondition: uid must be a valid connected player; req must be non-nil.
// Postcondition: On success, the slot is cleared and a message event is returned; on failure an error event is returned.
func (s *GameServiceServer) handleRemoveArmor(uid string, req *gamev1.RemoveArmorRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	result := command.HandleRemoveArmor(sess, s.invRegistry, req.GetSlot())
	// REQ-EM-29: recompute set bonuses whenever armor changes.
	if strings.HasPrefix(result, "Removed ") {
		session.RecomputeSetBonuses(sess, s.setRegistry)
		if sess.LoadoutSet != nil && sess.Equipment != nil {
			if active := sess.LoadoutSet.ActivePreset(); active != nil {
				equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
				sess.PassiveMaterials = inventory.ComputePassiveMaterials(equipped, sess.Equipment.Armor, s.invRegistry)
			}
		}
		// Persist immediately so removal survives a disconnect or crash.
		ctx := context.Background()
		invItems := backpackToInventoryItems(sess.Backpack)
		if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, invItems); err != nil {
			s.logger.Error("handleRemoveArmor: SaveInventory failed", zap.String("uid", uid), zap.Error(err))
		}
		if err := s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment); err != nil {
			s.logger.Error("handleRemoveArmor: SaveEquipment failed", zap.String("uid", uid), zap.Error(err))
		}
		s.pushInventory(sess)
	}
	return messageEvent(result), nil
}

// applyEquipEffect applies a CrossTeamEffect when a player equips rival-team gear.
//
// Precondition: uid must be a valid player; effect must be non-nil.
// Postcondition: If effect.Kind == "condition" and condition exists in condRegistry, it is applied.
func (s *GameServiceServer) applyEquipEffect(uid string, effect *inventory.CrossTeamEffect) {
	if effect.Kind != "condition" {
		return
	}
	if s.condRegistry == nil {
		return
	}
	cond, ok := s.condRegistry.Get(effect.Value)
	if !ok {
		if s.logger != nil {
			s.logger.Warn("unknown cross-team condition", zap.String("condition", effect.Value))
		}
		return
	}
	// Apply the condition via the combat handler's active combat, if one exists.
	// If no active combat, log that the condition cannot be applied outside combat.
	cbt := s.combatH.ActiveCombatForPlayer(uid)
	if cbt == nil {
		if s.logger != nil {
			s.logger.Debug("cross-team condition skipped: player not in combat",
				zap.String("uid", uid),
				zap.String("condition", cond.ID),
			)
		}
		return
	}
	if err := cbt.ApplyCondition(uid, cond.ID, 1, -1); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to apply cross-team condition",
				zap.String("uid", uid),
				zap.String("condition", cond.ID),
				zap.Error(err),
			)
		}
	}
}

func (s *GameServiceServer) handleReload(uid string, req *gamev1.ReloadRequest) (*gamev1.ServerEvent, error) {
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && sess.Conditions.Has("submerged") {
		return messageEvent("You are submerged underwater and cannot perform that action. Swim or Escape to surface."), nil
	}
	events, err := s.combatH.Reload(uid)
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleFireBurst(uid string, req *gamev1.FireBurstRequest) (*gamev1.ServerEvent, error) {
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && sess.Conditions.Has("submerged") {
		return messageEvent("You are submerged underwater and cannot attack. Swim or Escape to surface."), nil
	}
	events, err := s.combatH.FireBurst(uid, req.GetTarget())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleFireAutomatic(uid string, req *gamev1.FireAutomaticRequest) (*gamev1.ServerEvent, error) {
	if sess, ok := s.sessions.GetPlayer(uid); ok && sess.Conditions != nil && sess.Conditions.Has("submerged") {
		return messageEvent("You are submerged underwater and cannot attack. Swim or Escape to surface."), nil
	}
	events, err := s.combatH.FireAutomatic(uid, req.GetTarget())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

func (s *GameServiceServer) handleThrow(uid string, req *gamev1.ThrowRequest) (*gamev1.ServerEvent, error) {
	events, err := s.combatH.Throw(uid, req.GetExplosiveId())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	sess, sErr := s.sessions.GetPlayer(uid)
	if sErr && len(events) > 0 {
		s.BroadcastCombatEvents(sess.RoomID, events)
	}
	return nil, nil
}

// StartZoneTicks starts per-zone periodic tick goroutines.
//
// Precondition: ctx must not be nil; zm and aiReg must not be nil.
// Postcondition: zone tick goroutines are running until ctx is cancelled.
func (s *GameServiceServer) StartZoneTicks(ctx context.Context, zm *ZoneTickManager, aiReg *ai.Registry) {
	for _, zone := range s.world.AllZones() {
		zoneID := zone.ID
		zm.RegisterTick(zoneID, func() {
			s.tickZone(zoneID, aiReg)
		})
	}
	zm.Start(ctx)
}

// StartWantedDecayHook subscribes to the calendar and begins once-per-day
// WantedLevel decay for all online players.
//
// Precondition: MUST be called after NewGameServiceServer.
// Postcondition: decay goroutine runs until StopWantedDecayHook is called.
func (s *GameServiceServer) StartWantedDecayHook() {
	if s.calendar != nil && s.wantedRepo != nil {
		s.stopWantedDecay = StartWantedDecay(s.calendar, s.sessions, s.wantedRepo, s.logger)
	}
}

// StopWantedDecayHook unsubscribes from the calendar and stops the decay goroutine.
//
// Postcondition: safe to call when StartWantedDecayHook was never called or calendar/wantedRepo was nil.
func (s *GameServiceServer) StopWantedDecayHook() {
	if s.stopWantedDecay != nil {
		s.stopWantedDecay()
	}
}

// StartCarrierRadHook subscribes to the calendar and applies CarrierRadDmgPerHour
// radiation damage to players with Rad-Core passive material on each game-hour tick.
//
// Precondition: MUST be called after NewGameServiceServer.
// Postcondition: goroutine runs until StopCarrierRadHook is called.
func (s *GameServiceServer) StartCarrierRadHook() {
	if s.calendar == nil {
		return
	}
	ch := make(chan GameDateTime, 4)
	s.calendar.Subscribe(ch)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				for _, sess := range s.sessions.AllPlayers() {
					if sess.PassiveMaterials.CarrierRadDmgPerHour <= 0 {
						continue
					}
					dmg := sess.PassiveMaterials.CarrierRadDmgPerHour
					sess.CurrentHP -= dmg
					if sess.CurrentHP < 0 {
						sess.CurrentHP = 0
					}
					s.checkNonCombatDeath(sess.UID, sess) // REQ-BUG100-1
					s.pushMessageToUID(sess.UID, fmt.Sprintf(
						"Your Rad-Core implant irradiates you for %d radiation damage.", dmg))
				}
			case <-stop:
				s.calendar.Unsubscribe(ch)
				return
			}
		}
	}()
	s.stopCarrierRad = func() { close(stop) }
}

// StopCarrierRadHook unsubscribes from the calendar and stops the carrier rad goroutine.
//
// Postcondition: safe to call when StartCarrierRadHook was never called or calendar was nil.
func (s *GameServiceServer) StopCarrierRadHook() {
	if s.stopCarrierRad != nil {
		s.stopCarrierRad()
	}
}

// tickZone executes one game tick for the given zone, advancing NPC AI and
// draining the respawn queue.
//
// Precondition: zoneID must be a valid zone identifier loaded in worldMgr.
func (s *GameServiceServer) tickZone(zoneID string, aiReg *ai.Registry) {
	for _, zone := range s.world.AllZones() {
		if zone.ID != zoneID {
			continue
		}
		if s.npcH == nil {
			break
		}
		for _, room := range zone.Rooms {
			for _, inst := range s.npcH.InstancesInRoom(room.ID) {
				if s.combatH != nil && s.combatH.IsInCombat(inst.ID) {
					continue
				}
				s.tickNPCIdle(inst, zoneID, aiReg)
			}
		}
	}
	if s.respawnMgr != nil {
		s.respawnMgr.Tick(time.Now(), s.npcMgr)
	}
}

// tickNPCIdle evaluates idle/patrol behavior for a non-combat NPC.
func (s *GameServiceServer) tickNPCIdle(inst *npc.Instance, zoneID string, aiReg *ai.Registry) {
	if inst.Immobile {
		return
	}

	// REQ-NB-34: NPC was recruited via call_for_help on the previous tick; join the active combat.
	if inst.PendingJoinCombatRoomID != "" {
		pendingRoom := inst.PendingJoinCombatRoomID
		inst.PendingJoinCombatRoomID = ""
		inst.PlayerEnteredRoom = false
		inst.OnDamageTaken = false
		if s.combatH != nil {
			s.combatH.JoinPendingNPCCombat(inst, pendingRoom)
		}
		return
	}

	// Decrement ability cooldowns. REQ-NB-2.
	for k := range inst.AbilityCooldowns {
		if inst.AbilityCooldowns[k] > 0 {
			inst.AbilityCooldowns[k]--
		}
	}

	// Home-room return movement. REQ-NB-42–44.
	if inst.ReturningHome && s.combatH != nil && !s.combatH.IsInCombat(inst.ID) {
		s.npcBFSStep(inst, inst.HomeRoomID)
		if inst.RoomID == inst.HomeRoomID {
			inst.ReturningHome = false // REQ-NB-43
		}
		inst.PlayerEnteredRoom = false
		inst.OnDamageTaken = false
		return // movement consumes the tick
	}

	// Threat assessment on idle tick for hostile NPCs. REQ-NB-7.
	// REQ-FA-27: enemy faction NPCs are treated as hostile regardless of disposition.
	// REQ-FA-28: allied faction NPCs MUST NOT initiate combat against same-faction players.
	// Non-combat NPC types (merchant, healer, banker, job_trainer, etc.) never initiate combat.
	isCombatCapable := inst.NPCType == "" || inst.NPCType == "combat" || inst.NPCType == "guard" || inst.NPCType == "hireling"
	isHostileToPlayers := inst.Disposition == "hostile"
	// Allied-faction exclusion: suppress hostility if any room player is an ally of this NPC.
	if isHostileToPlayers && s.factionSvc != nil && inst.FactionID != "" {
		for _, p := range s.sessions.PlayersInRoomDetails(inst.RoomID) {
			if s.factionSvc.IsAllyOf(p, inst.FactionID) {
				isHostileToPlayers = false
				break
			}
		}
	}
	// Enemy-faction promotion: non-hostile NPC becomes hostile if any room player is a faction enemy.
	if !isHostileToPlayers && s.factionSvc != nil && inst.FactionID != "" {
		for _, p := range s.sessions.PlayersInRoomDetails(inst.RoomID) {
			if s.factionSvc.IsEnemyOf(p, inst.FactionID) {
				isHostileToPlayers = true
				break
			}
		}
	}
	if isCombatCapable && isHostileToPlayers && s.combatH != nil && !s.combatH.IsInCombat(inst.ID) {
		// REQ-57-1: NPCs MUST NOT initiate combat in rooms with effective danger_level "safe".
		canEngage := false
		if room, roomOK := s.world.GetRoom(inst.RoomID); roomOK {
			if zone, zoneOK := s.world.GetZone(room.ZoneID); zoneOK {
				effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, room.DangerLevel)
				canEngage = danger.CanInitiateCombat(effectiveLevel, "npc")
			}
		}
		if canEngage {
			s.evaluateThreatEngagement(inst, inst.RoomID)
		}
	}

	// Schedule evaluation. REQ-NB-19–22, 24.
	if s.gameHourFn != nil && s.npcMgr != nil {
		hour := s.gameHourFn()
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl != nil && len(tmpl.Schedule) > 0 {
			entry := behavior.ActiveEntry(tmpl.Schedule, hour)
			if entry != nil {
				s.applyScheduleEntry(inst, entry, zoneID, aiReg)
				inst.PlayerEnteredRoom = false
				inst.OnDamageTaken = false
				return
			}
		}
	}

	if inst.AIDomain == "" || aiReg == nil {
		inst.PlayerEnteredRoom = false
		inst.OnDamageTaken = false
		return
	}
	planner, ok := aiReg.PlannerFor(inst.AIDomain)
	if !ok {
		inst.PlayerEnteredRoom = false
		inst.OnDamageTaken = false
		return
	}

	hpPct := 0
	if inst.MaxHP > 0 {
		hpPct = inst.CurrentHP * 100 / inst.MaxHP
	}
	ws := &ai.WorldState{
		NPC: &ai.NPCState{
			UID:       inst.ID,
			Name:      inst.Name(),
			Kind:      "npc",
			HP:        inst.CurrentHP,
			MaxHP:     inst.MaxHP,
			Awareness: inst.Awareness,
			ZoneID:    zoneID,
			RoomID:    inst.RoomID,
		},
		Room:              &ai.RoomState{ID: inst.RoomID, ZoneID: zoneID},
		Combatants:        nil,
		InCombat:          false,
		PlayerEnteredRoom: inst.PlayerEnteredRoom,
		HPPctBelow:        hpPct,
		OnDamageTaken:     inst.OnDamageTaken,
		HasGrudgeTarget:   inst.GrudgePlayerID != "",
		GrudgePlayerID:    inst.GrudgePlayerID,
	}
	actions, err := planner.Plan(ws)
	if err != nil || len(actions) == 0 {
		inst.PlayerEnteredRoom = false
		inst.OnDamageTaken = false
		return
	}
	actions = FilterAnimalPlanActions(actions, inst.IsAnimal())
	for _, a := range actions {
		switch a.Action {
		case "move_random":
			s.npcMoveRandomFenced(inst, inst.HomeRoomID, inst.WanderRadius)
		case "say":
			s.npcSay(inst, a)
		default:
			// idle/pass: no-op
		}
	}
	// Clear one-shot flags after HTN evaluation. REQ-NB-4.
	inst.PlayerEnteredRoom = false
	inst.OnDamageTaken = false
}

// npcPatrolRandom moves the NPC to a random visible exit.
func (s *GameServiceServer) npcPatrolRandom(inst *npc.Instance) {
	if inst.Immobile {
		return
	}
	room, ok := s.world.GetRoom(inst.RoomID)
	if !ok || len(room.Exits) == 0 {
		return
	}
	exits := room.VisibleExits()
	if len(exits) == 0 {
		return
	}
	oldRoomID := inst.RoomID
	idx := rand.Intn(len(exits))
	newRoomID := exits[idx].TargetRoom
	_ = s.npcH.MoveNPC(inst.ID, newRoomID)
	s.pushRoomViewToAllInRoom(oldRoomID)
	s.pushRoomViewToAllInRoom(newRoomID)
}

// npcSay executes a "say" HTN operator: picks a random string and broadcasts it. REQ-NB-1, REQ-NB-2.
//
// Precondition: inst must not be nil; a.Strings must not be empty.
// Postcondition: broadcasts to room; updates AbilityCooldowns.
func (s *GameServiceServer) npcSay(inst *npc.Instance, a ai.PlannedAction) {
	if len(a.Strings) == 0 {
		return
	}
	// Enforce cooldown via AbilityCooldowns. REQ-NB-2.
	if inst.AbilityCooldowns == nil {
		inst.AbilityCooldowns = make(map[string]int)
	}
	if inst.AbilityCooldowns[a.OperatorID] > 0 {
		return
	}
	if a.Cooldown != "" {
		if d, err := time.ParseDuration(a.Cooldown); err == nil {
			// Compute tick count from cooldown duration and idle tick interval. REQ-NB-2.
			ticks := 1
			if s.npcIdleTickInterval > 0 && d > 0 {
				ticks = int(d / s.npcIdleTickInterval)
				if ticks < 1 {
					ticks = 1
				}
			}
			inst.AbilityCooldowns[a.OperatorID] = ticks
		}
	}
	line := a.Strings[rand.Intn(len(a.Strings))]
	s.broadcastMessage(inst.RoomID, "", &gamev1.MessageEvent{
		Content: fmt.Sprintf("%s says \"%s\"", inst.Name(), line),
	})
}

// evaluateThreatEngagement runs threat assessment for a hostile NPC not in combat.
// Initiates combat when threat score <= courage_threshold. REQ-NB-7.
//
// Precondition: inst must not be nil; inst must not be in combat; inst must be a combat-capable type.
func (s *GameServiceServer) evaluateThreatEngagement(inst *npc.Instance, roomID string) {
	// Guard: non-combat NPC types must never initiate combat.
	if inst.NPCType != "" && inst.NPCType != "combat" && inst.NPCType != "guard" && inst.NPCType != "hireling" {
		return
	}
	players := s.sessions.PlayersInRoomDetails(roomID)
	if len(players) == 0 {
		return
	}
	snapshots := make([]behavior.PlayerSnapshot, len(players))
	sumLevel := 0
	for i, p := range players {
		snapshots[i] = behavior.PlayerSnapshot{
			Level:     p.Level,
			CurrentHP: p.CurrentHP,
			MaxHP:     p.MaxHP,
		}
		sumLevel += p.Level
	}
	avgPlayerLevel := sumLevel / len(players)
	// REQ-79-1: NPCs MUST NOT initiate combat when the average player level exceeds
	// the NPC level by more than 4. At this gap the NPC cannot realistically land a hit.
	const levelGapThreshold = 4
	if avgPlayerLevel-inst.Level > levelGapThreshold {
		return
	}
	score := behavior.ThreatScore(snapshots, inst.Level)
	if score <= inst.CourageThreshold {
		// Engage: initiate combat with first player in room. REQ-NB-7.
		if s.combatH != nil && len(players) > 0 {
			s.combatH.InitiateNPCCombat(inst, players[0].UID)
		}
	}
	// else: NPC remains passive (REQ-NB-7A).
}

// npcBFSStep moves the NPC one step toward targetRoomID using its HomeRoomBFS map.
//
// Precondition: inst.HomeRoomBFS must be populated.
// Postcondition: NPC moves to adjacent room with minimum BFS distance to targetRoomID.
func (s *GameServiceServer) npcBFSStep(inst *npc.Instance, targetRoomID string) {
	if len(inst.HomeRoomBFS) == 0 {
		return
	}
	if inst.RoomID == targetRoomID {
		return
	}
	room, ok := s.world.GetRoom(inst.RoomID)
	if !ok {
		return
	}
	// The BFS map is keyed from homeRoom. To move toward targetRoomID, pick the
	// neighbor with the smallest distance value in HomeRoomBFS (closer to origin).
	bestDist := inst.HomeRoomBFS[inst.RoomID]
	bestRoom := ""
	for _, exit := range room.VisibleExits() {
		d, ok := inst.HomeRoomBFS[exit.TargetRoom]
		if !ok {
			continue
		}
		if d < bestDist {
			bestDist = d
			bestRoom = exit.TargetRoom
		}
	}
	if bestRoom == "" {
		return
	}
	oldRoomID := inst.RoomID
	_ = s.npcH.MoveNPC(inst.ID, bestRoom)
	s.pushRoomViewToAllInRoom(oldRoomID)
	s.pushRoomViewToAllInRoom(bestRoom)
}

// npcMoveRandomFenced moves the NPC to a random exit within wanderRadius BFS hops of anchorRoomID.
// REQ-NB-39, REQ-NB-40.
//
// Precondition: inst must not be nil; anchorRoomID must be non-empty.
func (s *GameServiceServer) npcMoveRandomFenced(inst *npc.Instance, anchorRoomID string, wanderRadius int) {
	if inst.Immobile {
		return
	}
	room, ok := s.world.GetRoom(inst.RoomID)
	if !ok {
		return
	}
	exits := room.VisibleExits()
	if len(exits) == 0 {
		return
	}

	// Filter exits by wander radius from anchorRoomID using HomeRoomBFS. REQ-NB-39.
	if wanderRadius > 0 && len(inst.HomeRoomBFS) > 0 {
		var filtered []world.Exit
		for _, exit := range exits {
			d, ok := inst.HomeRoomBFS[exit.TargetRoom]
			if !ok {
				continue
			}
			if d <= wanderRadius {
				filtered = append(filtered, exit)
			}
		}
		exits = filtered
	}

	if len(exits) == 0 {
		// REQ-NB-40: fail and allow HTN fallback.
		return
	}

	oldRoomID := inst.RoomID
	newRoomID := exits[rand.Intn(len(exits))].TargetRoom
	_ = s.npcH.MoveNPC(inst.ID, newRoomID)
	s.pushRoomViewToAllInRoom(oldRoomID)
	s.pushRoomViewToAllInRoom(newRoomID)
}

// applyScheduleEntry applies a schedule entry's behavior mode for this tick. REQ-NB-21.
func (s *GameServiceServer) applyScheduleEntry(inst *npc.Instance, entry *behavior.ScheduleEntry, zoneID string, aiReg *ai.Registry) {
	anchorRoomID := entry.PreferredRoom

	switch entry.BehaviorMode {
	case "idle":
		// REQ-NB-21A: remain in preferred_room; fire say operators via HTN. REQ-NB-21A.
		if inst.AIDomain != "" && aiReg != nil {
			if planner, ok := aiReg.PlannerFor(inst.AIDomain); ok {
				hpPct := 0
				if inst.MaxHP > 0 {
					hpPct = inst.CurrentHP * 100 / inst.MaxHP
				}
				ws := &ai.WorldState{
					NPC: &ai.NPCState{
						UID:       inst.ID,
						Name:      inst.Name(),
						Kind:      "npc",
						HP:        inst.CurrentHP,
						MaxHP:     inst.MaxHP,
						Awareness: inst.Awareness,
						ZoneID:    zoneID,
						RoomID:    inst.RoomID,
					},
					Room:              &ai.RoomState{ID: inst.RoomID, ZoneID: zoneID},
					InCombat:          false,
					PlayerEnteredRoom: inst.PlayerEnteredRoom,
					HPPctBelow:        hpPct,
					OnDamageTaken:     inst.OnDamageTaken,
					HasGrudgeTarget:   inst.GrudgePlayerID != "",
					GrudgePlayerID:    inst.GrudgePlayerID,
				}
				if actions, err := planner.Plan(ws); err == nil {
					for _, a := range actions {
						if a.Action == "say" {
							s.npcSay(inst, a)
						}
					}
				}
			}
		}
	case "patrol":
		// REQ-NB-21B: wander within wander_radius.
		s.npcMoveRandomFenced(inst, anchorRoomID, inst.WanderRadius)
	case "aggressive":
		// REQ-NB-21C: effective courage_threshold = 0 for this tick.
		// Non-combat NPC types never initiate combat even in aggressive schedule mode.
		if inst.NPCType == "" || inst.NPCType == "combat" || inst.NPCType == "guard" || inst.NPCType == "hireling" {
			origThreshold := inst.CourageThreshold
			inst.CourageThreshold = 0
			s.evaluateThreatEngagement(inst, inst.RoomID)
			inst.CourageThreshold = origThreshold
		}
	}

	// Move toward preferred_room if not already there and not patrolling. REQ-NB-20.
	if entry.BehaviorMode != "patrol" && inst.RoomID != anchorRoomID {
		s.npcBFSStep(inst, anchorRoomID)
	}
}

// handleInventory sends the player's backpack contents and currency.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent containing InventoryView, or an error event.
func (s *GameServiceServer) handleInventory(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	var items []*gamev1.InventoryItem
	for _, inst := range sess.Backpack.Items() {
		name := inst.ItemDefID
		kind := ""
		weight := 0.0
		armorSlot := ""
		armorCategory := ""
		effectsSummary := ""
		throwable := false
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
				name = def.Name
				kind = def.Kind
				weight = def.Weight
				throwable = def.HasTag("throwable")
				if def.Kind == inventory.KindArmor && def.ArmorRef != "" {
					if armorDef, ok := s.invRegistry.Armor(def.ArmorRef); ok {
						armorSlot = string(armorDef.Slot)
						armorCategory = armorProfCategoryLabel(string(armorDef.ProficiencyCategory))
					}
				}
				if def.Kind == inventory.KindConsumable {
					effectsSummary = buildConsumableEffectsSummary(def)
				}
			}
		}
		items = append(items, &gamev1.InventoryItem{
			InstanceId:     inst.InstanceID,
			Name:           name,
			Kind:           kind,
			Quantity:       int32(inst.Quantity),
			Weight:         weight * float64(inst.Quantity),
			ItemDefId:      inst.ItemDefID,
			ArmorSlot:      armorSlot,
			ArmorCategory:  armorCategory,
			EffectsSummary: effectsSummary,
			Throwable:      throwable,
		})
	}
	var totalWeight float64
	if s.invRegistry != nil {
		totalWeight = sess.Backpack.TotalWeight(s.invRegistry)
	}
	view := &gamev1.InventoryView{
		Items:       items,
		UsedSlots:   int32(sess.Backpack.UsedSlots()),
		MaxSlots:    int32(sess.Backpack.MaxSlots),
		TotalWeight: totalWeight,
		MaxWeight:   sess.Backpack.MaxWeight,
		Currency:    inventory.FormatCrypto(sess.Currency),
		TotalCrypto: int32(sess.Currency),
	}
	return &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_InventoryView{InventoryView: view}}, nil
}

// pushLoadout pushes a fresh LoadoutView event to the given player session.
//
// Precondition: sess must be non-nil and have a non-nil Entity.
// Postcondition: A LoadoutView event is marshaled and pushed; errors are
// logged and silently dropped so the calling handler is unaffected.
func (s *GameServiceServer) pushLoadout(sess *session.PlayerSession) {
	if sess == nil || sess.Entity == nil {
		return
	}
	evt := s.buildLoadoutView(sess)
	if evt == nil {
		return
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Warn("pushLoadout: marshal failed", zap.Error(err))
		return
	}
	if err := sess.Entity.Push(data); err != nil {
		s.logger.Warn("pushLoadout: push failed", zap.String("uid", sess.UID), zap.Error(err))
	}
}

// handleBalance sends the player's currency breakdown.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent containing a message with the currency string.
func (s *GameServiceServer) handleBalance(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatCrypto(sess.Currency))), nil
}

// handleLoadout displays or swaps weapon presets for the player.
//
// Precondition: uid must be a valid connected player with a non-nil LoadoutSet.
// Precondition: s.combatH must be non-nil whenever any player may be statusInCombat;
// if s.combatH is nil and a player is in combat, the AP gate is silently skipped.
// Postcondition: Returns a ServerEvent with the loadout display or swap result.
// Postcondition: If req.Arg is non-empty and sess.Status == statusInCombat and
// s.combatH is non-nil, exactly 1 AP is deducted before the swap; if AP is
// insufficient the swap is aborted and an error message is returned.
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	arg := req.GetArg()
	if arg != "" {
		if sess.Status == statusInCombat {
			if s.combatH != nil {
				if err := s.combatH.SpendAP(uid, 1); err != nil {
					return messageEvent("Not enough AP to swap loadouts."), nil
				}
			}
		} else {
			// Outside combat there are no rounds, so the once-per-round swap
			// limit does not apply. Clear the flag so repeated swaps work.
			if sess.LoadoutSet != nil {
				sess.LoadoutSet.ResetRound()
			}
		}
		return messageEvent(command.HandleLoadout(sess, arg, s.invRegistry)), nil
	}
	// No arg: return a structured LoadoutView for web clients; telnet bridge renders it as text.
	return s.buildLoadoutView(sess), nil
}

// buildLoadoutView constructs a LoadoutView ServerEvent from the session's LoadoutSet.
//
// Precondition: sess must not be nil.
// Postcondition: Returns a ServerEvent wrapping a sentinel-encoded MessageEvent carrying
// the LoadoutView as JSON. The sentinel prefix "\x00loadout\x00" allows the websocket and
// telnet handlers to detect and re-render the payload correctly.
func (s *GameServiceServer) buildLoadoutView(sess *session.PlayerSession) *gamev1.ServerEvent {
	lv := &gamev1.LoadoutView{}
	if sess.LoadoutSet != nil {
		lv.ActiveIndex = int32(sess.LoadoutSet.Active)
		brutalityMod := combat.AbilityMod(sess.Abilities.Brutality)
		for _, preset := range sess.LoadoutSet.Presets {
			wp := &gamev1.LoadoutWeaponPreset{}
			if preset.MainHand != nil {
				def := preset.MainHand.Def
				wp.MainHand = def.Name
				wp.MainHandDamage = weaponDamageString(def.DamageDice, brutalityMod, def.IsMelee())
			}
			if preset.OffHand != nil {
				def := preset.OffHand.Def
				wp.OffHand = def.Name
				wp.OffHandDamage = weaponDamageString(def.DamageDice, brutalityMod, def.IsMelee())
			}
			lv.Presets = append(lv.Presets, wp)
		}
	}
	data, err := protojson.Marshal(lv)
	if err != nil {
		return errorEvent("failed to build loadout view")
	}
	// Embed JSON as sentinel-prefixed MessageEvent content.
	// The websocket handler detects "\x00loadout\x00" and re-wraps as "LoadoutView".
	// The telnet bridge renders it as human-readable text.
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: "\x00loadout\x00" + string(data),
			},
		},
	}
}

// handleUnequip removes the item in the given slot and returns it to the backpack.
//
// Precondition: uid must be a valid connected player with non-nil LoadoutSet and Equipment; req.Slot must be non-empty.
// Postcondition: Returns a ServerEvent with the unequip result string.
func (s *GameServiceServer) handleUnequip(uid string, req *gamev1.UnequipRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	result := command.HandleUnequip(sess, req.GetSlot())
	// Push updated inventory (item returned to backpack) and loadout (slot now empty)
	// so the web UI refreshes both panels immediately (REQ-UI-EQUIP-1).
	s.pushInventory(sess)
	s.pushLoadout(sess)
	return messageEvent(result), nil
}

// handleEquipment displays all equipped armor, accessories, and weapon presets.
//
// Precondition: uid must be a valid connected player with non-nil LoadoutSet and Equipment.
// Postcondition: Returns a ServerEvent with the full equipment display string.
func (s *GameServiceServer) handleEquipment(uid string, _ *gamev1.EquipmentRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	// Hydrate display names immediately before rendering so the output always
	// reflects the current registry, regardless of when login hydration ran.
	hydrateEquipmentNames(sess.Equipment, s.invRegistry)
	return messageEvent(command.HandleEquipment(sess, 80, s.invRegistry)), nil
}

// pushCharacterSheet sends an updated CharacterSheetView to the player's entity
// channel. It is used after in-place session mutations (e.g. hero point spends)
// so the client display reflects the new state without requiring the player to
// type "char" manually.
//
// Precondition: sess must be non-nil and have a non-nil Entity.
// Postcondition: A CharacterSheetView event is marshaled and pushed; errors are
// logged and silently dropped so the calling handler is unaffected.
func (s *GameServiceServer) pushCharacterSheet(sess *session.PlayerSession) {
	if sess == nil || sess.Entity == nil {
		return
	}
	evt, err := s.handleChar(sess.UID)
	if err != nil || evt == nil {
		return
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Warn("pushCharacterSheet: marshal failed", zap.Error(err))
		return
	}
	if err := sess.Entity.Push(data); err != nil {
		s.logger.Warn("pushCharacterSheet: push failed", zap.String("uid", sess.UID), zap.Error(err))
	}
}

// saveInventory persists the player's current backpack to durable storage.
// It is wired into CombatHandler via SetSaveInventoryFn so that post-combat
// item loot is immediately durable after being granted.
//
// Precondition: sess must be non-nil and have a non-nil Backpack.
// Postcondition: Backpack contents are persisted; errors are returned to the caller.
func (s *GameServiceServer) saveInventory(sess *session.PlayerSession) error {
	if sess == nil || sess.Backpack == nil {
		return nil
	}
	items := backpackToInventoryItems(sess.Backpack)
	return s.charSaver.SaveInventory(context.Background(), sess.CharacterID, items)
}

// pushInventory pushes a fresh InventoryView event to the given player session.
// It is wired into CombatHandler via SetPushInventoryFn so the web UI Inventory
// tab updates immediately when currency is distributed after combat.
//
// Precondition: sess must be non-nil and have a non-nil Entity.
// Postcondition: An InventoryView event is marshaled and pushed; errors are
// logged and silently dropped so the calling handler is unaffected.
func (s *GameServiceServer) pushInventory(sess *session.PlayerSession) {
	if sess == nil || sess.Entity == nil {
		return
	}
	evt, err := s.handleInventory(sess.UID)
	if err != nil || evt == nil {
		return
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		s.logger.Warn("pushInventory: marshal failed", zap.Error(err))
		return
	}
	if err := sess.Entity.Push(data); err != nil {
		s.logger.Warn("pushInventory: push failed", zap.String("uid", sess.UID), zap.Error(err))
	}
}

// handleChar builds and returns a CharacterSheetView for the requesting player.
//
// Precondition: uid must identify an active session.
// Postcondition: Returns CharacterSheetView on success; errorEvent if session not found.
func (s *GameServiceServer) handleChar(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	view := &gamev1.CharacterSheetView{
		Name:           sess.CharName,
		Level:          int32(sess.Level),
		CurrentHp:      int32(sess.CurrentHP),
		MaxHp:          int32(sess.MaxHP),
		Brutality:      int32(sess.Abilities.Brutality),
		Grit:           int32(sess.Abilities.Grit),
		Quickness:      int32(sess.Abilities.Quickness),
		Reasoning:      int32(sess.Abilities.Reasoning),
		Savvy:          int32(sess.Abilities.Savvy),
		Flair:          int32(sess.Abilities.Flair),
		Currency:       inventory.FormatCrypto(sess.Currency),
		Gender:         sess.Gender,
		Team:           sess.Team,
		HeroPoints:     int32(sess.HeroPoints),
		FocusPoints:    int32(sess.FocusPoints),
		MaxFocusPoints: int32(sess.MaxFocusPoints),
	}

	// Job info from registry.
	archetypeID := ""
	if s.jobRegistry != nil {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			view.Job = job.Name
			archetypeID = job.Archetype
			// Resolve archetype display name from archetype registry.
			view.Archetype = archetypeID
			if s.archetypes != nil {
				if arch, ok := s.archetypes[archetypeID]; ok {
					view.Archetype = arch.Name
				}
			}
		} else {
			view.Job = sess.Class
		}
		if t := s.jobRegistry.TeamFor(sess.Class); t != "" {
			view.Team = t
		}
	} else {
		view.Job = sess.Class
	}

	// Defense stats (dex mod from Quickness). REQ-EM-35: apply set bonuses.
	dexMod := (sess.Abilities.Quickness - 10) / 2
	def := sess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(s.invRegistry, dexMod, sess.Proficiencies, sess.Level, sess.SetBonusSummary)
	itemAC := def.ACBonus - def.ProficiencyACBonus
	if itemAC < 0 {
		itemAC = 0
	}
	view.AcBonus = int32(itemAC) // item bonuses only (for UI breakdown)
	view.ProficiencyAcBonus = int32(def.ProficiencyACBonus)
	view.EffectiveArmorCategory = def.EffectiveArmorCategory
	view.CheckPenalty = int32(def.CheckPenalty)
	view.SpeedPenalty = int32(def.SpeedPenalty)
	view.TotalAc = int32(10 + def.EffectiveDex + def.ACBonus)
	sess.Resistances = def.Resistances
	sess.Weaknesses = def.Weaknesses

	// Player resistances and weaknesses from equipped armor.
	for dmgType, val := range sess.Resistances {
		view.PlayerResistances = append(view.PlayerResistances, &gamev1.ResistanceEntry{
			DamageType: dmgType,
			Value:      int32(val),
		})
	}
	sort.Slice(view.PlayerResistances, func(i, j int) bool {
		return view.PlayerResistances[i].DamageType < view.PlayerResistances[j].DamageType
	})
	for dmgType, val := range sess.Weaknesses {
		view.PlayerWeaknesses = append(view.PlayerWeaknesses, &gamev1.ResistanceEntry{
			DamageType: dmgType,
			Value:      int32(val),
		})
	}
	sort.Slice(view.PlayerWeaknesses, func(i, j int) bool {
		return view.PlayerWeaknesses[i].DamageType < view.PlayerWeaknesses[j].DamageType
	})

	// Armor slots.
	view.Armor = make(map[string]string)
	view.ArmorCategories = make(map[string]string)
	for slot, item := range sess.Equipment.Armor {
		if item == nil {
			continue
		}
		name := item.Name
		if s.invRegistry != nil {
			if armorDef, ok := s.invRegistry.Armor(item.ItemDefID); ok {
				name = armorDef.Name
				if armorDef.ACBonus > 0 {
					name = fmt.Sprintf("%s (+%d AC)", armorDef.Name, armorDef.ACBonus)
				}
				if cat := armorProfCategoryLabel(string(armorDef.ProficiencyCategory)); cat != "" {
					view.ArmorCategories[string(slot)] = cat
				}
			}
		}
		view.Armor[string(slot)] = name
	}

	// Accessory slots.
	view.Accessories = make(map[string]string)
	for slot, item := range sess.Equipment.Accessories {
		if item == nil {
			continue
		}
		name := item.Name
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(item.ItemDefID); ok {
				name = def.Name
			}
		}
		view.Accessories[string(slot)] = name
	}

	// Weapons from active loadout.
	if sess.LoadoutSet != nil {
		if preset := sess.LoadoutSet.ActivePreset(); preset != nil {
			brutalityMod := combat.AbilityMod(sess.Abilities.Brutality)
			weaponLevel := sess.Level
			if weaponLevel < 1 {
				weaponLevel = 1
			}
			if preset.MainHand != nil {
				def := preset.MainHand.Def
				view.MainHand = def.Name
				// GH #241: resolve the effective proficiency rank via the
				// weapon-category fallback chain (jobs grant e.g. simple_weapons;
				// weapons declare simple_melee/martial_melee).
				profRank, _ := resolveWeaponProficiency(sess.Proficiencies, def.ProficiencyCategory)
				profBonus := combat.CombatProficiencyBonus(weaponLevel, profRank)
				atkBonus := brutalityMod + profBonus + def.Bonus
				view.MainHandAttackBonus = signedInt(atkBonus)
				view.MainHandDamage = weaponDamageString(def.DamageDice, brutalityMod+def.Bonus, def.IsMelee())
				view.MainHandAbilityBonus = int32(brutalityMod)
				view.MainHandProfBonus = int32(profBonus)
				view.MainHandProfRank = profRank
				view.MainHandProfCategory = def.ProficiencyCategory // GH #242
			}
			if preset.OffHand != nil {
				def := preset.OffHand.Def
				view.OffHand = def.Name
				profRank, _ := resolveWeaponProficiency(sess.Proficiencies, def.ProficiencyCategory)
				profBonus := combat.CombatProficiencyBonus(weaponLevel, profRank)
				atkBonus := brutalityMod + profBonus + def.Bonus
				view.OffHandAttackBonus = signedInt(atkBonus)
				view.OffHandDamage = weaponDamageString(def.DamageDice, brutalityMod+def.Bonus, def.IsMelee())
				view.OffHandAbilityBonus = int32(brutalityMod)
				view.OffHandProfBonus = int32(profBonus)
				view.OffHandProfRank = profRank
				view.OffHandProfCategory = def.ProficiencyCategory // GH #242
			}
		}
	}

	// Skills block.
	if s.characterSkillsRepo != nil && len(s.allSkills) > 0 {
		skillMap, err := s.characterSkillsRepo.GetAll(context.Background(), sess.CharacterID)
		if err == nil {
			for _, sk := range s.allSkills {
				rank := skillMap[sk.ID]
				if rank == "" {
					rank = "untrained"
				}
				view.Skills = append(view.Skills, &gamev1.SkillEntry{
					SkillId:     sk.ID,
					Name:        sk.Name,
					Ability:     sk.Ability,
					Proficiency: rank,
				})
			}
		}
	}

	// Feats block.
	if s.characterFeatsRepo != nil && s.featRegistry != nil {
		featIDs, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
		if err == nil {
			for _, fid := range featIDs {
				f, ok := s.featRegistry.Feat(fid)
				if !ok {
					continue
				}
				armorCat := ""
				if f.ID == "armor_training" && sess.FeatureChoices != nil {
					armorCat = armorProfCategoryLabel(sess.FeatureChoices["armor_training"]["armor_category"])
				}
				view.Feats = append(view.Feats, &gamev1.FeatEntry{
					FeatId:        f.ID,
					Name:          f.Name,
					Active:        f.Active,
					Description:   f.Description,
					ActivateText:  f.ActivateText,
					IsReaction:    f.Reaction != nil,
					ArmorCategory: armorCat,
					ActionCost:    int32(f.ActionCost),
				})
			}
		}
	}

	// Class features block.
	if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
		cfIDs, err := s.characterClassFeaturesRepo.GetAll(context.Background(), sess.CharacterID)
		if err == nil {
			for _, cfid := range cfIDs {
				cf, ok := s.classFeatureRegistry.ClassFeature(cfid)
				if !ok {
					continue
				}
				view.ClassFeatures = append(view.ClassFeatures, &gamev1.ClassFeatureEntry{
					FeatureId:    cf.ID,
					Name:         cf.Name,
					Archetype:    cf.Archetype,
					Job:          cf.Job,
					Active:       cf.Active,
					Description:  cf.Description,
					ActivateText: cf.ActivateText,
				})
			}
		}
	}

	// Proficiencies block.
	level := sess.Level
	if level < 1 {
		level = 1
	}
	view.Proficiencies = buildProficiencyEntries(sess.Proficiencies, level)

	// Saves: static bonus = ability_mod + CombatProficiencyBonus(level, rank).
	view.ToughnessSave = int32(combat.AbilityMod(sess.Abilities.Grit) +
		combat.CombatProficiencyBonus(level, sess.Proficiencies["toughness"]))
	view.HustleSave = int32(combat.AbilityMod(sess.Abilities.Quickness) +
		combat.CombatProficiencyBonus(level, sess.Proficiencies["hustle"]))
	view.CoolSave = int32(combat.AbilityMod(sess.Abilities.Savvy) +
		combat.CombatProficiencyBonus(level, sess.Proficiencies["cool"]))

	// Awareness defaults to trained if no rank is recorded.
	if _, hasAwareness := sess.Proficiencies["awareness"]; !hasAwareness {
		if sess.Proficiencies == nil {
			sess.Proficiencies = make(map[string]string)
		}
		sess.Proficiencies["awareness"] = "trained"
	}

	// Awareness: 10 + savvy_mod + awareness proficiency bonus.
	view.Awareness = int32(10 + combat.AbilityMod(sess.Abilities.Savvy) +
		combat.CombatProficiencyBonus(level, sess.Proficiencies["awareness"]))

	// XP progress.
	view.Experience = int32(sess.Experience)
	view.PendingBoosts = int32(sess.PendingBoosts)
	view.PendingSkillIncreases = int32(sess.PendingSkillIncreases)
	view.PendingTechSelections = int32(len(sess.PendingTechGrants))
	// Active exploration mode (empty string when no mode is active).
	view.ExploreMode = sess.ExploreMode
	// Tech tradition derived from the job's archetype ID.
	view.TechTradition = technology.DominantTradition(archetypeID)
	// Prepared technology slots with expended state.
	if len(sess.PreparedTechs) > 0 {
		levels := make([]int, 0, len(sess.PreparedTechs))
		for lvl := range sess.PreparedTechs {
			levels = append(levels, lvl)
		}
		sort.Ints(levels)
		for _, lvl := range levels {
			for _, slot := range sess.PreparedTechs[lvl] {
				if slot != nil {
					techName := slot.TechID
					techDesc := ""
					techFX := ""
					techShortName := ""
					if s.techRegistry != nil {
						if def, ok := s.techRegistry.Get(slot.TechID); ok {
							techName = def.Name
							techDesc = def.Description
							techFX = technology.FormatEffectsSummary(def)
							techShortName = def.ShortName
						}
					}
					view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
						TechId:         slot.TechID,
						Expended:       slot.Expended,
						TechName:       techName,
						Description:    techDesc,
						EffectsSummary: techFX,
						ShortName:      techShortName,
						TechLevel:      int32(lvl), // lvl is the slot level (map key), not def.Level
					})
				}
			}
		}
	}
	if s.xpSvc != nil {
		cfg := s.xpSvc.Config()
		if sess.Level < cfg.LevelCap {
			view.XpToNext = int32(xp.XPToLevel(sess.Level+1, cfg.BaseXP))
		}
		// If at level cap, XpToNext remains 0 (zero-value).
	}

	for level, pool := range sess.SpontaneousUsePools {
		view.SpontaneousUsePools = append(view.SpontaneousUsePools, &gamev1.SpontaneousUsePoolView{
			TechLevel:     int32(level),
			UsesRemaining: int32(pool.Remaining),
			MaxUses:       int32(pool.Max),
		})
	}

	// Spontaneous known techs with names.
	if len(sess.KnownTechs) > 0 {
		knownLevels := make([]int, 0, len(sess.KnownTechs))
		for lvl := range sess.KnownTechs {
			knownLevels = append(knownLevels, lvl)
		}
		sort.Ints(knownLevels)
		for _, lvl := range knownLevels {
			for _, tid := range sess.KnownTechs[lvl] {
				techName := tid
				techDesc := ""
				techFX := ""
				techShortName := ""
				if s.techRegistry != nil {
					if def, ok := s.techRegistry.Get(tid); ok {
						techName = def.Name
						techDesc = def.Description
						techFX = technology.FormatEffectsSummary(def)
						techShortName = def.ShortName
					}
				}
				view.SpontaneousKnown = append(view.SpontaneousKnown, &gamev1.SpontaneousKnownEntry{
					TechId:         tid,
					TechName:       techName,
					TechLevel:      int32(lvl),
					Description:    techDesc,
					EffectsSummary: techFX,
					ShortName:      techShortName,
				})
			}
		}
	}

	innateIDs := make([]string, 0, len(sess.InnateTechs))
	for id := range sess.InnateTechs {
		innateIDs = append(innateIDs, id)
	}
	sort.Strings(innateIDs)
	for _, id := range innateIDs {
		slot := sess.InnateTechs[id]
		techName := id
		techDesc := ""
		techFX := ""
		techShortName := ""
		var isReaction bool
		var isPassive bool
		var techLevel int32
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(id); ok {
				techName = def.Name
				techDesc = def.Description
				techFX = technology.FormatEffectsSummary(def)
				isReaction = def.Reaction != nil
				techShortName = def.ShortName
				isPassive = def.Passive
				techLevel = int32(def.Level)
			}
		}
		view.InnateSlots = append(view.InnateSlots, &gamev1.InnateSlotView{
			TechId:         id,
			UsesRemaining:  int32(slot.UsesRemaining),
			MaxUses:        int32(slot.MaxUses),
			TechName:       techName,
			Description:    techDesc,
			EffectsSummary: techFX,
			IsReaction:     isReaction,
			ShortName:      techShortName,
			Passive:        isPassive,
			TechLevel:      techLevel,
		})
	}

	// Hardwired technologies (always available, unlimited use).
	if len(sess.HardwiredTechs) > 0 {
		hwIDs := make([]string, len(sess.HardwiredTechs))
		copy(hwIDs, sess.HardwiredTechs)
		sort.Strings(hwIDs)
		for _, id := range hwIDs {
			techName := id
			techDesc := ""
			techFX := ""
			techShortName := ""
			var techLevel int32
			if s.techRegistry != nil {
				if def, ok := s.techRegistry.Get(id); ok {
					techName = def.Name
					techDesc = def.Description
					techFX = technology.FormatEffectsSummary(def)
					techShortName = def.ShortName
					techLevel = int32(def.Level)
				}
			}
			view.HardwiredSlots = append(view.HardwiredSlots, &gamev1.HardwiredSlotView{
				TechId:         id,
				TechName:       techName,
				Description:    techDesc,
				EffectsSummary: techFX,
				ShortName:      techShortName,
				TechLevel:      techLevel,
			})
		}
	}

	// REQ-EM-31: active set bonuses on character sheet.
	if s.setRegistry != nil {
		session.RecomputeSetBonuses(sess, s.setRegistry)
		for _, bonus := range s.setRegistry.ActiveBonuses(func() []string {
			ids := make([]string, 0, 8)
			for _, slotted := range sess.Equipment.Armor {
				if slotted != nil && slotted.ItemDefID != "" {
					ids = append(ids, slotted.ItemDefID)
				}
			}
			return ids
		}()) {
			view.ActiveSetBonuses = append(view.ActiveSetBonuses, bonus.Description)
		}
	}

	// Duplicate-effects plan Task 10: populate EffectsSummary so telnet and web
	// clients render identical "Active Effects" blocks. We feed the player's
	// condition-derived EffectSet into effect/render.EffectsBlock. casterNames
	// is nil out-of-combat — render falls back to source-prefix labels
	// ("item", "feat", "tech", "self").
	if sess.Conditions != nil {
		view.EffectsSummary = effectrender.EffectsBlock(sess.Conditions.Effects(), nil, 80)
	} else {
		view.EffectsSummary = effectrender.EffectsBlock(nil, nil, 80)
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CharacterSheet{CharacterSheet: view},
	}, nil
}

// handleSetRole changes a target account's privilege level.
//
// Precondition: uid must be a valid connected player with admin role.
// Postcondition: Returns a success message or an error event.
func (s *GameServiceServer) handleSetRole(uid string, req *gamev1.SetRoleRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if evt := requireAdmin(sess); evt != nil {
		return evt, nil
	}
	if s.accountAdmin == nil {
		return errorEvent("account administration not available"), nil
	}
	if req.TargetUsername == "" || req.Role == "" {
		return errorEvent("usage: setrole <username> <role>"), nil
	}

	target, err := s.accountAdmin.GetAccountByUsername(context.Background(), req.TargetUsername)
	if err != nil {
		return errorEvent(fmt.Sprintf("account %q not found", req.TargetUsername)), nil
	}
	if err := s.accountAdmin.SetAccountRole(context.Background(), target.ID, req.Role); err != nil {
		return errorEvent(fmt.Sprintf("failed to set role: %v", err)), nil
	}

	return messageEvent(fmt.Sprintf("Set role for %s (#%d): %s -> %s",
		target.Username, target.ID, target.Role, req.Role)), nil
}

// handleSummonItem places an item instance on the floor of the caller's current room.
//
// Precondition: uid identifies an active session; req is non-nil.
// Postcondition: on success, one ItemInstance is added to the room floor and a success
// message is returned; on failure, an error event is returned with no side effects.
func (s *GameServiceServer) handleSummonItem(uid string, req *gamev1.SummonItemRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.invRegistry == nil {
		return errorEvent("item registry unavailable"), nil
	}
	def, ok := s.invRegistry.Item(req.ItemId)
	if !ok {
		return errorEvent(fmt.Sprintf("unknown item: %q", req.ItemId)), nil
	}
	qty := int(req.Quantity)
	if qty < 1 {
		qty = 1
	}
	inst := inventory.ItemInstance{
		InstanceID: uuid.New().String(),
		ItemDefID:  req.ItemId,
		Quantity:   qty,
	}
	if s.floorMgr == nil {
		return errorEvent("floor system not available"), nil
	}
	s.floorMgr.Drop(sess.RoomID, inst)
	return messageEvent(fmt.Sprintf("Summoned %dx %s to the room.", qty, def.Name)), nil
}

// handleTeleport moves a target player to a specific room by ID.
//
// Precondition: uid must be a valid connected player with admin role.
// Postcondition: Target player is moved, location is persisted, target receives a message and room view.
func (s *GameServiceServer) handleTeleport(uid string, req *gamev1.TeleportRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if evt := requireAdmin(sess); evt != nil {
		return evt, nil
	}
	if req.TargetCharacter == "" || req.RoomId == "" {
		return errorEvent("usage: teleport <character> <room_id>"), nil
	}

	targetRoom, ok := s.world.GetRoom(req.RoomId)
	if !ok {
		return errorEvent(fmt.Sprintf("room %q not found", req.RoomId)), nil
	}

	target, ok := s.sessions.GetPlayerByCharName(req.TargetCharacter)
	if !ok {
		return errorEvent(fmt.Sprintf("player %q not online", req.TargetCharacter)), nil
	}

	oldRoomID, err := s.sessions.MovePlayer(target.UID, targetRoom.ID)
	if err != nil {
		return errorEvent(fmt.Sprintf("failed to move player: %v", err)), nil
	}

	// Persist location immediately.
	if target.CharacterID > 0 && s.charSaver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.charSaver.SaveState(ctx, target.CharacterID, targetRoom.ID, target.CurrentHP); err != nil {
			s.logger.Warn("persisting teleport location",
				zap.String("target", target.CharName),
				zap.Error(err),
			)
		}
	}

	// Broadcast departure from old room.
	s.broadcastRoomEvent(oldRoomID, target.UID, &gamev1.RoomEvent{
		Player: target.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	// Broadcast arrival in new room.
	s.broadcastRoomEvent(targetRoom.ID, target.UID, &gamev1.RoomEvent{
		Player: target.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send message and room view to the target player.
	roomView := s.worldH.buildRoomView(target.UID, targetRoom)
	teleportMsg := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You have been teleported to %s.", targetRoom.Title),
				Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
			},
		},
	}
	teleportView := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}
	if data, err := proto.Marshal(teleportMsg); err == nil {
		_ = target.Entity.Push(data)
	}
	if data, err := proto.Marshal(teleportView); err == nil {
		_ = target.Entity.Push(data)
	}

	return messageEvent(fmt.Sprintf("Teleported %s to %s (%s)",
		target.CharName, targetRoom.Title, targetRoom.ID)), nil
}

// handleGetItem picks up an item from the room floor into the player's backpack.
//
// Precondition: uid must be a valid connected player; target is the item name or "all".
// Postcondition: On success the item is moved from floor to backpack; returns a message event.
func (s *GameServiceServer) handleGetItem(uid, target string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.floorMgr == nil {
		return errorEvent("floor system not available"), nil
	}
	if target == "all" {
		items := s.floorMgr.PickupAll(sess.RoomID)
		if len(items) == 0 {
			return messageEvent("There is nothing here to pick up."), nil
		}
		picked := 0
		for _, item := range items {
			_, err := sess.Backpack.Add(item.ItemDefID, item.Quantity, s.invRegistry)
			if err != nil {
				s.floorMgr.Drop(sess.RoomID, item)
				continue
			}
			if s.questSvc != nil {
				if questMsgs, questErr := s.questSvc.RecordFetch(context.Background(), sess, sess.CharacterID, item.ItemDefID, item.Quantity); questErr == nil {
					for _, qm := range questMsgs {
						s.pushMessageToUID(uid, qm)
					}
					if len(questMsgs) > 0 {
						s.pushCharacterSheet(sess)
					}
				}
			}
			picked++
		}
		return messageEvent(fmt.Sprintf("Picked up %d item(s).", picked)), nil
	}

	floorItems := s.floorMgr.ItemsInRoom(sess.RoomID)
	for _, item := range floorItems {
		name := item.ItemDefID
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(item.ItemDefID); ok {
				name = def.Name
			}
		}
		if strings.EqualFold(name, target) || strings.EqualFold(item.ItemDefID, target) {
			picked, ok := s.floorMgr.Pickup(sess.RoomID, item.InstanceID)
			if !ok {
				return errorEvent("Item is no longer there."), nil
			}
			_, err := sess.Backpack.Add(picked.ItemDefID, picked.Quantity, s.invRegistry)
			if err != nil {
				s.floorMgr.Drop(sess.RoomID, picked)
				return errorEvent(fmt.Sprintf("Cannot pick up: %v", err)), nil
			}
			if s.questSvc != nil {
				if questMsgs, questErr := s.questSvc.RecordFetch(context.Background(), sess, sess.CharacterID, picked.ItemDefID, picked.Quantity); questErr == nil {
					for _, qm := range questMsgs {
						s.pushMessageToUID(uid, qm)
					}
					if len(questMsgs) > 0 {
						s.pushCharacterSheet(sess)
					}
				}
			}
			return messageEvent(fmt.Sprintf("You pick up %s.", name)), nil
		}
	}
	floorMats := s.floorMgr.MaterialsInRoom(sess.RoomID)
	for matID, qty := range floorMats {
		var matName string
		if s.materialReg != nil {
			if matDef, ok := s.materialReg.Material(matID); ok {
				matName = matDef.Name
			}
		}
		if matName == "" {
			matName = matID
		}
		if !strings.EqualFold(matName, target) && !strings.EqualFold(matID, target) {
			continue
		}
		taken := s.floorMgr.TakeMaterial(sess.RoomID, matID, qty)
		if taken == 0 {
			return errorEvent("Material is no longer there."), nil
		}
		if sess.Materials == nil {
			sess.Materials = make(map[string]int)
		}
		sess.Materials[matID] += taken
		if s.materialRepo != nil {
			_ = s.materialRepo.Add(context.Background(), sess.CharacterID, matID, taken)
		}
		return messageEvent(fmt.Sprintf("You pick up %d %s.", taken, matName)), nil
	}
	return messageEvent("You don't see that here."), nil
}

// handleDropItem drops an item from the player's backpack to the room floor.
//
// Precondition: uid must be a valid connected player; target is the item name.
// Postcondition: On success the item is moved from backpack to floor; returns a message event.
func (s *GameServiceServer) handleDropItem(uid, target string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if s.floorMgr == nil {
		return errorEvent("floor system not available"), nil
	}
	for _, inst := range sess.Backpack.Items() {
		name := inst.ItemDefID
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
				name = def.Name
			}
		}
		if strings.EqualFold(name, target) || strings.EqualFold(inst.ItemDefID, target) {
			if err := sess.Backpack.Remove(inst.InstanceID, inst.Quantity); err != nil {
				return errorEvent(fmt.Sprintf("Cannot drop: %v", err)), nil
			}
			s.floorMgr.Drop(sess.RoomID, inst)
			return messageEvent(fmt.Sprintf("You drop %s.", name)), nil
		}
	}
	return messageEvent("You don't have that."), nil
}

// handleArchetypeSelection acknowledges an archetype selection during character creation.
// Archetype is derived at runtime from the job; this handler satisfies CMD-6 dispatch.
//
// Precondition: uid must be non-empty; req must be non-nil.
// Postcondition: Returns an empty ServerEvent or error if session not found.
func (s *GameServiceServer) handleArchetypeSelection(uid string, req *gamev1.ArchetypeSelectionRequest) (*gamev1.ServerEvent, error) {
	if _, ok := s.sessions.GetPlayer(uid); !ok {
		return nil, fmt.Errorf("handleArchetypeSelection: session not found for uid %q", uid)
	}
	return &gamev1.ServerEvent{}, nil
}

// handleUseEquipment processes a UseEquipment command, invoking the Lua script
// attached to the room equipment instance if one exists.
//
// Precondition: uid must map to an active player session.
// Postcondition: Returns a ServerEvent with the result text or an error.
func (s *GameServiceServer) handleUseEquipment(uid, instanceID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.roomEquipMgr == nil {
		return messageEvent("No equipment available."), nil
	}
	inst := s.roomEquipMgr.GetInstance(sess.RoomID, instanceID)
	if inst == nil {
		return messageEvent("That item is not here."), nil
	}

	// Resolve the zone ID once; used for Lua hooks below.
	room, roomOk := s.world.GetRoom(sess.RoomID)
	zoneID := ""
	if roomOk {
		zoneID = room.ZoneID
	}

	// Fire on_use skill check triggers BEFORE invoking the item Lua script.
	// A "deny" effect blocks script execution and returns immediately.
	// Non-deny outcome messages are collected and prepended to the final response.
	var skillMsgs []string
	for _, trigger := range inst.SkillChecks {
		if trigger.Trigger != "on_use" {
			continue
		}

		abilityScore := s.abilityScoreForSkill(sess, trigger.Skill)
		amod := abilityModFrom(abilityScore)
		rank := ""
		if sess.Skills != nil {
			rank = sess.Skills[trigger.Skill]
		}

		var roll int
		if s.dice != nil {
			roll = s.dice.Src().Intn(20) + 1
		} else {
			roll = 10 // neutral fallback when no dice configured
		}

		checkResult := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

		outcome := trigger.Outcomes.ForOutcome(checkResult.Outcome)
		if outcome != nil {
			if outcome.Effect != nil && outcome.Effect.Type == "deny" {
				// Deny: fire Lua hook then block execution.
				if s.scriptMgr != nil {
					s.scriptMgr.CallHook(zoneID, "on_skill_check", //nolint:errcheck
						lua.LString(uid),
						lua.LString(trigger.Skill),
						lua.LNumber(checkResult.Total),
						lua.LNumber(trigger.DC),
						lua.LString(checkResult.Outcome.String()),
					)
				}
				return messageEvent(outcome.Message), nil
			}
			if outcome.Message != "" {
				skillMsgs = append(skillMsgs, outcome.Message)
			}
			// Apply non-deny effects.
			s.applySkillCheckEffect(sess, outcome.Effect, sess.RoomID)
		}

		if s.scriptMgr != nil {
			s.scriptMgr.CallHook(zoneID, "on_skill_check", //nolint:errcheck
				lua.LString(uid),
				lua.LString(trigger.Skill),
				lua.LNumber(checkResult.Total),
				lua.LNumber(trigger.DC),
				lua.LString(checkResult.Outcome.String()),
			)
		}
	}

	// No deny blocked execution; proceed with item Lua script.
	if inst.Script == "" {
		if len(skillMsgs) > 0 {
			return messageEvent(strings.Join(skillMsgs, "\r\n")), nil
		}
		return messageEvent("Nothing happens."), nil
	}
	if !roomOk {
		if len(skillMsgs) > 0 {
			return messageEvent(strings.Join(skillMsgs, "\r\n")), nil
		}
		return messageEvent("Nothing happens."), nil
	}
	if s.scriptMgr == nil {
		if len(skillMsgs) > 0 {
			return messageEvent(strings.Join(skillMsgs, "\r\n")), nil
		}
		return messageEvent("Nothing happens."), nil
	}
	result, err := s.scriptMgr.CallHook(zoneID, inst.Script, lua.LString(uid))
	if err != nil {
		s.logger.Warn("equipment script error", zap.Error(err))
		return messageEvent("The item malfunctions."), nil
	}
	msg := "You use the item."
	if result != lua.LNil {
		msg = result.String()
	}
	if len(skillMsgs) > 0 {
		msg = strings.Join(skillMsgs, "\r\n") + "\r\n" + msg
	}
	// Interaction trap: fire if this equipment has an armed interaction-trigger trap.
	if s.trapMgr != nil && s.trapTemplates != nil && roomOk {
		s.checkInteractionTrap(uid, sess, room, instanceID)
	}
	return messageEvent(msg), nil
}

// handleRoomEquip processes a RoomEquip command, performing add/remove/list/modify
// operations on the room equipment configuration for the player's current room.
//
// Precondition: uid must map to an active player session; req must be non-nil.
// Postcondition: Returns a ServerEvent with the result text or an error.
func (s *GameServiceServer) handleRoomEquip(uid string, req *gamev1.RoomEquipRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.roomEquipMgr == nil {
		return messageEvent("Room equipment manager not available."), nil
	}
	roomID := sess.RoomID
	switch req.SubCommand {
	case "list":
		cfgs := s.roomEquipMgr.ListConfigs(roomID)
		if len(cfgs) == 0 {
			return messageEvent("No equipment configured for this room."), nil
		}
		var sb strings.Builder
		for _, c := range cfgs {
			sb.WriteString(fmt.Sprintf("  %s (max:%d respawn:%s immovable:%v)\r\n",
				c.ItemID, c.MaxCount, c.RespawnAfter, c.Immovable))
		}
		return messageEvent(sb.String()), nil
	case "add":
		if req.ItemId == "" {
			return messageEvent("Usage: roomequip add <item_id> [max_count] [respawn] [immovable] [script]"), nil
		}
		dur, _ := time.ParseDuration(req.Respawn)
		count := int(req.MaxCount)
		if count < 1 {
			count = 1
		}
		cfg := world.RoomEquipmentConfig{
			ItemID:       req.ItemId,
			MaxCount:     count,
			RespawnAfter: dur,
			Immovable:    req.Immovable,
			Script:       req.Script,
		}
		s.roomEquipMgr.AddConfig(roomID, cfg)
		return messageEvent(fmt.Sprintf("Added %s to room equipment.", req.ItemId)), nil
	case "remove":
		if req.ItemId == "" {
			return messageEvent("Usage: roomequip remove <item_id>"), nil
		}
		if !s.roomEquipMgr.RemoveConfig(roomID, req.ItemId) {
			return messageEvent(fmt.Sprintf("Item %q not found in room equipment.", req.ItemId)), nil
		}
		return messageEvent(fmt.Sprintf("Removed %s from room equipment.", req.ItemId)), nil
	case "modify":
		if req.ItemId == "" {
			return messageEvent("Usage: roomequip modify <item_id> [max_count] [respawn] [immovable] [script]"), nil
		}
		s.roomEquipMgr.RemoveConfig(roomID, req.ItemId)
		dur, _ := time.ParseDuration(req.Respawn)
		count := int(req.MaxCount)
		if count < 1 {
			count = 1
		}
		cfg := world.RoomEquipmentConfig{
			ItemID:       req.ItemId,
			MaxCount:     count,
			RespawnAfter: dur,
			Immovable:    req.Immovable,
			Script:       req.Script,
		}
		s.roomEquipMgr.AddConfig(roomID, cfg)
		return messageEvent(fmt.Sprintf("Modified %s in room equipment.", req.ItemId)), nil
	default:
		return messageEvent("Usage: roomequip <add|remove|list|modify>"), nil
	}
}

// handleMap returns the automap tiles for the player's current zone (view=="zone" or ""),
// or world map zone tiles for all zones with coordinates (view=="world").
//
// Precondition: uid must map to an active player session.
// Postcondition: Returns a ServerEvent with MapResponse containing the appropriate tiles.
func (s *GameServiceServer) handleMap(uid string, req *gamev1.MapRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status == statusInCombat {
		return messageEvent("You cannot use the map while in combat."), nil
	}
	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("You are nowhere."), nil
	}
	zoneID := room.ZoneID
	zone, ok := s.world.GetZone(zoneID)
	if !ok {
		return messageEvent("No map available."), nil
	}

	// World view: populate world_tiles for all zones with non-nil WorldX/WorldY.
	if req.GetView() == "world" {
		// Build a set of zone IDs that have world map positions, for connection filtering.
		worldMapZoneIDs := make(map[string]bool)
		for _, z := range s.world.AllZones() {
			if z.WorldX != nil && z.WorldY != nil {
				worldMapZoneIDs[z.ID] = true
			}
		}

		var worldTiles []*gamev1.WorldZoneTile
		for _, z := range s.world.AllZones() {
			if z.WorldX == nil || z.WorldY == nil {
				continue
			}
			enemy := isEnemyZone(sess.Team, z.ID)
			isDiscovered := len(sess.AutomapCache[z.ID]) > 0 || enemy // enemy zones always visible
			current := z.ID == zone.ID
			var levelRange string
			switch {
			case z.MinLevel > 0 && z.MaxLevel > 0:
				levelRange = fmt.Sprintf("%d-%d", z.MinLevel, z.MaxLevel)
			case z.MinLevel > 0:
				levelRange = fmt.Sprintf("%d+", z.MinLevel)
			case z.MaxLevel > 0:
				levelRange = fmt.Sprintf("1-%d", z.MaxLevel)
			default:
				levelRange = ""
			}

			// Collect IDs of directly connected zones (via any zone-crossing exit from any room).
			connectedSet := make(map[string]bool)
			for _, room := range z.Rooms {
				for _, exit := range room.Exits {
					if exit.TargetRoom == "" {
						continue
					}
					targetRoom, ok := s.world.GetRoom(exit.TargetRoom)
					if !ok {
						continue
					}
					if targetRoom.ZoneID != z.ID && worldMapZoneIDs[targetRoom.ZoneID] {
						connectedSet[targetRoom.ZoneID] = true
					}
				}
			}
			var connectedZoneIDs []string
			for cid := range connectedSet {
				connectedZoneIDs = append(connectedZoneIDs, cid)
			}

			worldTiles = append(worldTiles, &gamev1.WorldZoneTile{
				ZoneId:           z.ID,
				ZoneName:         z.Name,
				WorldX:           int32(*z.WorldX),
				WorldY:           int32(*z.WorldY),
				Discovered:       isDiscovered,
				Current:          current,
				DangerLevel:      z.DangerLevel,
				LevelRange:       levelRange,
				Enemy:            enemy,
				Description:      z.Description,
				ConnectedZoneIds: connectedZoneIDs,
			})
		}
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Map{
				Map: &gamev1.MapResponse{WorldTiles: worldTiles},
			},
		}, nil
	}

	discovered := sess.AutomapCache[zoneID]
	var tiles []*gamev1.MapTile
	for roomID := range discovered {
		r, ok := zone.Rooms[roomID]
		if !ok {
			continue
		}
		var exits []string
		var zoneExits []*gamev1.ZoneExitInfo
		var sameZoneExitTargets []*gamev1.SameZoneExitTarget
		for _, e := range r.Exits {
			exits = append(exits, string(e.Direction))
			if e.TargetRoom != "" {
				if targetRoom, ok := s.world.GetRoom(e.TargetRoom); ok {
					if targetRoom.ZoneID != r.ZoneID {
						info := &gamev1.ZoneExitInfo{
							Direction:  string(e.Direction),
							DestZoneId: targetRoom.ZoneID,
						}
						if destZone, ok := s.world.GetZone(targetRoom.ZoneID); ok {
							info.DestZoneName = destZone.Name
						}
						zoneExits = append(zoneExits, info)
					} else {
						sameZoneExitTargets = append(sameZoneExitTargets, &gamev1.SameZoneExitTarget{
							Direction:    string(e.Direction),
							TargetRoomId: targetRoom.ID,
						})
					}
				}
			}
		}
		// Gate danger level and POIs on physical exploration (BUG-27).
		var effectiveLevelStr string
		var poiSlice []string
		var poiNpcs []*gamev1.PoiWithNpc
		if sess.ExploredCache[zoneID][roomID] {
			effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, r.DangerLevel)
			effectiveLevelStr = string(effectiveLevel)

			// Collect POI type IDs for this explored room (REQ-POI-15..18).
			poiSet := make(map[string]bool)
			if s.npcMgr != nil {
				for _, inst := range s.npcMgr.InstancesInRoom(r.ID) {
					if inst.IsDead() {
						continue
					}
					role := inst.NpcRole
					if role == "" {
						role = maputil.POIRoleFromNPCType(inst.NPCType)
					}
					poiID := maputil.NpcRoleToPOIID(role)
					if poiID != "" {
						poiSet[poiID] = true
						poiNpcs = append(poiNpcs, &gamev1.PoiWithNpc{
							PoiId:   poiID,
							NpcName: inst.Name(),
						})
					}
				}
			}
			for _, eq := range r.Equipment {
				switch {
				case eq.ItemID == "zone_map":
					poiSet["map"] = true
				case eq.CoverTier != "":
					poiSet["cover"] = true
				default:
					poiSet["equipment"] = true
				}
			}
			for id := range poiSet {
				poiSlice = append(poiSlice, id)
			}
			poiSlice = maputil.SortPOIs(poiSlice)
		}

		tiles = append(tiles, &gamev1.MapTile{
			RoomId:                r.ID,
			RoomName:              r.Title,
			X:                     int32(r.MapX),
			Y:                     int32(r.MapY),
			Current:               r.ID == sess.RoomID,
			Exits:                 exits,
			DangerLevel:           effectiveLevelStr,
			Pois:                  poiSlice,
			BossRoom:              r.BossRoom,
			PoiNpcs:               poiNpcs,
			ZoneExits:             zoneExits,
			SameZoneExitTargets:   sameZoneExitTargets,
			Explored:              sess.ExploredCache[zoneID][roomID],
		})
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Map{
			Map: &gamev1.MapResponse{Tiles: tiles},
		},
	}, nil
}

// wireRevealZone wires the engine.map.reveal_zone Lua callback in the script manager.
// It bulk-inserts all rooms in the target zone into character_map_rooms and
// populates AutomapCache[zoneID] for the matching player session.
//
// Precondition: Must be called after s.scriptMgr and s.world are initialized.
// Postcondition: s.scriptMgr.RevealZoneMap is set; calling it from Lua inserts all rooms idempotently.
func (s *GameServiceServer) wireRevealZone() {
	if s.scriptMgr == nil {
		return
	}
	s.scriptMgr.RevealZoneMap = func(uid, zoneID string) {
		zone, ok := s.world.GetZone(zoneID)
		if !ok {
			return
		}
		sess, ok := s.sessions.GetPlayer(uid)
		if !ok {
			return
		}
		if sess.AutomapCache[zoneID] == nil {
			sess.AutomapCache[zoneID] = make(map[string]bool)
		}
		roomIDs := make([]string, 0, len(zone.Rooms))
		for roomID := range zone.Rooms {
			if !sess.AutomapCache[zoneID][roomID] {
				sess.AutomapCache[zoneID][roomID] = true
				roomIDs = append(roomIDs, roomID)
			}
		}
		if s.automapRepo != nil && len(roomIDs) > 0 {
			if err := s.automapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, roomIDs, false); err != nil {
				s.logger.Warn("reveal_zone: bulk insert automap", zap.Error(err))
			}
		}
		// Record zone map use for quest progress.
		if s.questSvc != nil && sess.CharacterID > 0 {
			msgs, err := s.questSvc.RecordZoneMapUse(context.Background(), sess, sess.CharacterID, zoneID)
			if err != nil {
				s.logger.Warn("reveal_zone: RecordZoneMapUse failed",
					zap.String("uid", uid),
					zap.String("zone_id", zoneID),
					zap.Error(err),
				)
			}
			for _, msg := range msgs {
				s.pushMessageToUID(uid, msg)
			}
		}
	}
}

// wireScriptMgrCombatCallbacks wires the engine.combat.* Lua callbacks that depend on
// the combat engine and faction registry into the script manager.
//
// Precondition: Must be called after s.scriptMgr, s.combatH, and s.factionRegistry are initialized.
// Postcondition: GetCombatantsInRoom and GetFactionHostiles are set on s.scriptMgr when non-nil.
func (s *GameServiceServer) wireScriptMgrCombatCallbacks() {
	if s.scriptMgr == nil {
		return
	}
	if s.combatH != nil {
		s.scriptMgr.GetCombatantsInRoom = s.combatH.GetCombatantsInRoom
	}
	if s.factionRegistry != nil {
		reg := *s.factionRegistry
		s.scriptMgr.GetFactionHostiles = func(factionID string) []string {
			def, ok := reg[factionID]
			if !ok {
				return nil
			}
			return def.HostileFactions
		}
	}
}

// handleSkills returns all skill proficiencies for the player's current character.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ServerEvent with SkillsResponse containing all skills.
func (s *GameServiceServer) handleSkills(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.characterSkillsRepo == nil || len(s.allSkills) == 0 {
		return messageEvent("Skill data is not available."), nil
	}
	skills, err := s.characterSkillsRepo.GetAll(context.Background(), sess.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("getting skills for %s: %w", uid, err)
	}
	entries := make([]*gamev1.SkillEntry, 0, len(s.allSkills))
	for _, sk := range s.allSkills {
		prof, ok := skills[sk.ID]
		if !ok {
			prof = "untrained"
		}
		entries = append(entries, &gamev1.SkillEntry{
			SkillId:     sk.ID,
			Name:        sk.Name,
			Ability:     sk.Ability,
			Proficiency: prof,
			Bonus:       int32(skillcheck.ProficiencyBonus(prof)),
			Description: sk.Description,
		})
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_SkillsResponse{
			SkillsResponse: &gamev1.SkillsResponse{Skills: entries},
		},
	}, nil
}

// handleFeats returns all feat entries for the player's current character.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ServerEvent with FeatsResponse containing all feats.
func (s *GameServiceServer) handleFeats(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.characterFeatsRepo == nil || s.featRegistry == nil {
		return messageEvent("Feat data is not available."), nil
	}
	featIDs, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("getting feats for %s: %w", uid, err)
	}

	var entries []*gamev1.FeatEntry
	for _, id := range featIDs {
		f, ok := s.featRegistry.Feat(id)
		if !ok {
			continue
		}
		armorCat := ""
		if f.ID == "armor_training" && sess.FeatureChoices != nil {
			armorCat = armorProfCategoryLabel(sess.FeatureChoices["armor_training"]["armor_category"])
		}
		entries = append(entries, &gamev1.FeatEntry{
			FeatId:        f.ID,
			Name:          f.Name,
			Category:      f.Category,
			Active:        f.Active,
			Description:   f.Description,
			ActivateText:  f.ActivateText,
			IsReaction:    f.Reaction != nil,
			ArmorCategory: armorCat,
			ActionCost:    int32(f.ActionCost),
		})
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_FeatsResponse{
			FeatsResponse: &gamev1.FeatsResponse{Feats: entries},
		},
	}, nil
}

// handleClassFeatures returns all class feature entries for the player's current character.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ClassFeaturesResponse partitioned into archetype vs. job features.
func (s *GameServiceServer) handleClassFeatures(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.characterClassFeaturesRepo == nil || s.classFeatureRegistry == nil {
		return messageEvent("Class feature data is not available."), nil
	}
	featureIDs, err := s.characterClassFeaturesRepo.GetAll(context.Background(), sess.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("getting class features: %w", err)
	}

	var archetypeFeatures []*gamev1.ClassFeatureEntry
	var jobFeatures []*gamev1.ClassFeatureEntry

	for _, id := range featureIDs {
		f, ok := s.classFeatureRegistry.ClassFeature(id)
		if !ok {
			continue
		}
		entry := &gamev1.ClassFeatureEntry{
			FeatureId:    f.ID,
			Name:         f.Name,
			Archetype:    f.Archetype,
			Job:          f.Job,
			Active:       f.Active,
			Description:  f.Description,
			ActivateText: f.ActivateText,
		}
		if f.Archetype != "" {
			archetypeFeatures = append(archetypeFeatures, entry)
		} else {
			jobFeatures = append(jobFeatures, entry)
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ClassFeaturesResponse{
			ClassFeaturesResponse: &gamev1.ClassFeaturesResponse{
				ArchetypeFeatures: archetypeFeatures,
				JobFeatures:       jobFeatures,
			},
		},
	}, nil
}

// handleJobGrants returns the job's feat and technology grant table for the player's current job.
//
// Precondition: uid must resolve to an active session with a loaded character and job.
// Postcondition: Returns a JobGrantsResponse with fixed feat/tech grants at each level.
func (s *GameServiceServer) handleJobGrants(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.jobRegistry == nil || sess.Class == "" {
		return messageEvent("Job grant data is not available."), nil
	}
	job, ok := s.jobRegistry.Job(sess.Class)
	if !ok {
		return messageEvent("Job definition not found."), nil
	}

	var featGrants []*gamev1.JobFeatGrant
	var techGrants []*gamev1.JobTechGrant
	var pendingFeatChoices []*gamev1.PendingFeatChoice

	// Helper to resolve feat name from registry.
	// For armor_training, appends the chosen armor category so the player can see their selection.
	featName := func(id string) string {
		if s.featRegistry != nil {
			if f, ok := s.featRegistry.Feat(id); ok {
				name := f.Name
				if id == "armor_training" && sess.FeatureChoices != nil {
					if cat := armorProfCategoryLabel(sess.FeatureChoices["armor_training"]["armor_category"]); cat != "" {
						name += " (" + cat + " armor)"
					}
				}
				return name
			}
		}
		return id
	}

	// Load the player's actual feat IDs for choice resolution.
	// Choice-pool grants show what the player picked; unresolved pools show the option label.
	playerFeatIDs := make(map[string]bool)
	if s.characterFeatsRepo != nil {
		if ids, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID); err == nil {
			for _, id := range ids {
				playerFeatIDs[id] = true
			}
		}
	}
	// attributedFeatIDs tracks feats already attributed to a grant slot so that a feat chosen
	// from a pool at level N is not duplicated in subsequent levels whose pools overlap.
	attributedFeatIDs := make(map[string]bool)
	// choicePoolIDs is populated after mergedFeatGrants is computed (below).
	// Declared here so addFeatGrants (a closure) can reference it by the time it is called.
	choicePoolIDs := make(map[string]bool)
	// Helper to resolve tech name from technology registry.
	techName := func(id string) string {
		if s.techRegistry != nil {
			if t, ok := s.techRegistry.Get(id); ok {
				return t.Name
			}
		}
		return id
	}
	// Collect all feat grants (fixed, choice pool, and general) from a FeatGrants at a given level.
	// For choice-pool and general grants, show what the player actually chose when that can be
	// determined from playerFeatIDs; otherwise show the selection label.
	addFeatGrants := func(fg *ruleset.FeatGrants, level int) {
		if fg == nil {
			return
		}
		// Fixed feats always granted; mark attributed so general-count slots don't reclaim them.
		for _, id := range fg.Fixed {
			attributedFeatIDs[id] = true
			featGrants = append(featGrants, &gamev1.JobFeatGrant{
				GrantLevel: int32(level),
				FeatId:     id,
				FeatName:   featName(id),
			})
		}
		// Choice pool — show what the player actually picked, or the selection label if not yet chosen.
		// Only count feats that haven't already been attributed to a prior level's choice slot.
		if fg.Choices != nil && fg.Choices.Count > 0 && len(fg.Choices.Pool) > 0 {
			var chosen []string
			for _, id := range fg.Choices.Pool {
				if playerFeatIDs[id] && !attributedFeatIDs[id] {
					chosen = append(chosen, id)
					if len(chosen) == fg.Choices.Count {
						break // cap at the number of choices granted at this level
					}
				}
			}
			if len(chosen) > 0 {
				// Player has made a selection — show the actual feat(s) chosen and mark attributed.
				for _, id := range chosen {
					attributedFeatIDs[id] = true
					featGrants = append(featGrants, &gamev1.JobFeatGrant{
						GrantLevel: int32(level),
						FeatId:     id,
						FeatName:   featName(id),
					})
				}
			} else {
				// Player has not yet chosen — emit an empty grant row and a structured PendingFeatChoice.
				// Options exclude feats the player already owns (from any source).
				featGrants = append(featGrants, &gamev1.JobFeatGrant{
					GrantLevel: int32(level),
					FeatId:     "",
					FeatName:   "",
				})
				opts := make([]*gamev1.FeatOption, 0, len(fg.Choices.Pool))
				for _, id := range fg.Choices.Pool {
					// Resolve legacy PF2E pool IDs to canonical feat IDs before checking
					// ownership. Pool entries may use legacy PF2E names (e.g. "rage") while
					// feats are stored under canonical IDs (e.g. "wrath").
					canonicalID := id
					if s.featRegistry != nil {
						if f, ok := s.featRegistry.Feat(id); ok {
							canonicalID = f.ID
						}
					}
					if playerFeatIDs[canonicalID] {
						continue // player already owns this feat; exclude from choices
					}
					opt := &gamev1.FeatOption{FeatId: canonicalID, Name: featName(canonicalID)}
					if s.featRegistry != nil {
						if f, ok := s.featRegistry.Feat(canonicalID); ok {
							opt.Description = f.Description
							opt.Category = f.Category
						}
					}
					opts = append(opts, opt)
				}
				if len(opts) == 0 {
					// All pool feats are already owned; no choice needed — remove the empty grant row.
					featGrants = featGrants[:len(featGrants)-1]
				} else {
					pendingFeatChoices = append(pendingFeatChoices, &gamev1.PendingFeatChoice{
						GrantLevel: int32(level),
						Count:      int32(fg.Choices.Count),
						Options:    opts,
					})
				}
			}
		}
		// General feat pick — resolve from player's unattributed, non-pool feats (sorted for determinism).
		// Pool-member feats are excluded so they remain available for their specific choice slots.
		// Falls back to the selection label only if no qualifying player feats remain.
		if fg.GeneralCount > 0 {
			// Collect unattributed, non-pool player feat IDs, sorted for deterministic output.
			var unattributed []string
			for id := range playerFeatIDs {
				if !attributedFeatIDs[id] && !choicePoolIDs[id] {
					unattributed = append(unattributed, id)
				}
			}
			sort.Strings(unattributed)
			if len(unattributed) > 0 {
				cap := fg.GeneralCount
				if cap > len(unattributed) {
					cap = len(unattributed)
				}
				for _, id := range unattributed[:cap] {
					attributedFeatIDs[id] = true
					featGrants = append(featGrants, &gamev1.JobFeatGrant{
						GrantLevel: int32(level),
						FeatId:     id,
						FeatName:   featName(id),
					})
				}
			} else {
				label := fmt.Sprintf("Choose %d general feat", fg.GeneralCount)
				if fg.GeneralCount > 1 {
					label += "s"
				}
				featGrants = append(featGrants, &gamev1.JobFeatGrant{
					GrantLevel: int32(level),
					FeatId:     "",
					FeatName:   label,
				})
			}
		}
	}
	// Collect tech grants from a TechnologyGrants at a given level.
	addTechGrants := func(tg *ruleset.TechnologyGrants, level int) {
		if tg == nil {
			return
		}
		for _, id := range tg.Hardwired {
			techGrants = append(techGrants, &gamev1.JobTechGrant{
				GrantLevel: int32(level),
				TechId:     id,
				TechName:   techName(id),
				TechType:   "hardwired",
			})
		}
		if tg.Prepared != nil {
			for _, e := range tg.Prepared.Fixed {
				techGrants = append(techGrants, &gamev1.JobTechGrant{
					GrantLevel: int32(level),
					TechId:     e.ID,
					TechName:   techName(e.ID),
					TechLevel:  int32(e.Level),
					TechType:   "prepared",
				})
			}
			// Emit slot count grants sorted by tech level for deterministic ordering.
			slotLevels := make([]int, 0, len(tg.Prepared.SlotsByLevel))
			for tl := range tg.Prepared.SlotsByLevel {
				slotLevels = append(slotLevels, tl)
			}
			sort.Ints(slotLevels)
			for _, tl := range slotLevels {
				count := tg.Prepared.SlotsByLevel[tl]
				techGrants = append(techGrants, &gamev1.JobTechGrant{
					GrantLevel: int32(level),
					TechLevel:  int32(tl),
					TechName:   fmt.Sprintf("+%d Prepared Slot (Level %d tech)", count, tl),
					TechType:   "prepared_slot",
				})
			}
		}
		if tg.Spontaneous != nil {
			for _, e := range tg.Spontaneous.Fixed {
				techGrants = append(techGrants, &gamev1.JobTechGrant{
					GrantLevel: int32(level),
					TechId:     e.ID,
					TechName:   techName(e.ID),
					TechLevel:  int32(e.Level),
					TechType:   "spontaneous",
				})
			}
			// Emit use pool grants sorted by tech level for deterministic ordering.
			useLevels := make([]int, 0, len(tg.Spontaneous.UsesByLevel))
			for tl := range tg.Spontaneous.UsesByLevel {
				useLevels = append(useLevels, tl)
			}
			sort.Ints(useLevels)
			for _, tl := range useLevels {
				count := tg.Spontaneous.UsesByLevel[tl]
				techGrants = append(techGrants, &gamev1.JobTechGrant{
					GrantLevel: int32(level),
					TechLevel:  int32(tl),
					TechName:   fmt.Sprintf("+%d Use (Level %d tech)", count, tl),
					TechType:   "spontaneous_use",
				})
			}
		}
	}

	// Merge archetype level-up grants with job-specific level-up grants.
	// Archetype grants take the base position; job-specific entries overlay or extend them.
	var archetypeLevelUpFeatGrants map[int]*ruleset.FeatGrants
	var archetypeLevelUpGrants map[int]*ruleset.TechnologyGrants
	var archetypeCreationTechGrants *ruleset.TechnologyGrants
	if s.archetypes != nil {
		if arch, ok := s.archetypes[job.Archetype]; ok {
			archetypeLevelUpFeatGrants = arch.LevelUpFeatGrants
			archetypeLevelUpGrants = arch.LevelUpGrants
			archetypeCreationTechGrants = arch.TechnologyGrants
		}
	}
	mergedFeatGrants := ruleset.MergeFeatLevelUpGrants(archetypeLevelUpFeatGrants, job.LevelUpFeatGrants)

	// Populate choicePoolIDs BEFORE any addFeatGrants calls so the general-count branch
	// can exclude pool feats when resolving which feat the player freely chose.
	// General-count slots MUST NOT claim pool-member feats — they belong to specific choice slots.
	collectPoolIDs := func(fg *ruleset.FeatGrants) {
		if fg != nil && fg.Choices != nil {
			for _, id := range fg.Choices.Pool {
				choicePoolIDs[id] = true
			}
		}
	}
	collectPoolIDs(job.FeatGrants)
	for _, fg := range mergedFeatGrants {
		collectPoolIDs(fg)
	}

	// Level 1 (character creation) grants — after choicePoolIDs is fully populated.
	addFeatGrants(job.FeatGrants, 1)
	addTechGrants(job.TechnologyGrants, 1)
	addTechGrants(archetypeCreationTechGrants, 1)

	// Level-up grants — only up to the player's current level.
	maxLevel := sess.Level
	if maxLevel < 1 {
		maxLevel = 1 // treat level 0 as level 1 (character creation grants already added)
	}
	for level := 2; level <= maxLevel; level++ {
		if fg, ok := mergedFeatGrants[level]; ok {
			addFeatGrants(fg, level)
		}
		// Tech grants: merge archetype and job per-level.
		archTG := archetypeLevelUpGrants[level]
		jobTG := job.LevelUpGrants[level]
		switch {
		case archTG != nil && jobTG != nil:
			addTechGrants(archTG, level)
			addTechGrants(jobTG, level)
		case archTG != nil:
			addTechGrants(archTG, level)
		case jobTG != nil:
			addTechGrants(jobTG, level)
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_JobGrantsResponse{
			JobGrantsResponse: &gamev1.JobGrantsResponse{
				FeatGrants:         featGrants,
				TechGrants:         techGrants,
				PendingFeatChoices: pendingFeatChoices,
			},
		},
	}, nil
}

// handleProficiencies returns all armor/weapon proficiency entries for the player's character.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ProficienciesResponse with one entry per proficiency category.
func (s *GameServiceServer) handleProficiencies(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	level := sess.Level
	if level < 1 {
		level = 1
	}
	entries := buildProficiencyEntries(sess.Proficiencies, level)
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ProficienciesResponse{
			ProficienciesResponse: &gamev1.ProficienciesResponse{
				Proficiencies: entries,
			},
		},
	}, nil
}

// handleLevelUp applies a pending ability boost to the named ability for the player.
//
// Precondition: uid must identify an active session; ability must be one of the six valid ability names.
// Postcondition: if the player has no pending boosts, returns an error message event;
// otherwise increments the named ability by 2, decrements PendingBoosts, persists both,
// and returns a confirmation message event.
func (s *GameServiceServer) handleLevelUp(uid, ability string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.PendingBoosts <= 0 {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: "You have no pending ability boosts."},
			},
		}, nil
	}

	// Compute updated abilities without mutating sess yet.
	updated := sess.Abilities
	switch ability {
	case "brutality":
		updated.Brutality += 2
	case "quickness":
		updated.Quickness += 2
	case "grit":
		updated.Grit += 2
	case "reasoning":
		updated.Reasoning += 2
	case "savvy":
		updated.Savvy += 2
	case "flair":
		updated.Flair += 2
	default:
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: fmt.Sprintf("Unknown ability '%s'.", ability)},
			},
		}, nil
	}

	ctx := context.Background()
	if sess.CharacterID > 0 && s.charSaver != nil {
		if err := s.charSaver.SaveAbilities(ctx, sess.CharacterID, updated); err != nil {
			s.logger.Warn("handleLevelUp: SaveAbilities failed", zap.Error(err))
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Failed to save ability boost. Please try again."},
				},
			}, nil
		}
	}
	if sess.CharacterID > 0 && s.progressRepo != nil {
		if err := s.progressRepo.ConsumePendingBoost(ctx, sess.CharacterID); err != nil {
			s.logger.Warn("handleLevelUp: ConsumePendingBoost failed", zap.Error(err))
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Failed to consume pending boost. Please try again."},
				},
			}, nil
		}
	}

	// Both persistence calls succeeded — apply mutations to session.
	sess.Abilities = updated
	sess.PendingBoosts--

	// Push updated character sheet so Stats tab reflects new ability scores and pending boost count.
	s.pushCharacterSheet(sess)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("Ability boost applied: %s is now %d. Pending boosts remaining: %d.",
					ability, abilityValue(sess.Abilities, ability), sess.PendingBoosts),
			},
		},
	}, nil
}

// abilityValue returns the current score for the named ability from an AbilityScores struct.
//
// Precondition: ability must be one of the six valid ability names.
// Postcondition: returns the current value of the named ability, or 0 if the name is unrecognized.
func abilityValue(a character.AbilityScores, ability string) int {
	switch ability {
	case "brutality":
		return a.Brutality
	case "quickness":
		return a.Quickness
	case "grit":
		return a.Grit
	case "reasoning":
		return a.Reasoning
	case "savvy":
		return a.Savvy
	case "flair":
		return a.Flair
	default:
		return 0
	}
}

// handleCombatDefault persists the player's preferred default combat action.
//
// Precondition: uid must identify an active session; action must be a valid combat action.
// Postcondition: sess.DefaultCombatAction is updated and persisted; a confirmation message event is returned.
func (s *GameServiceServer) handleCombatDefault(uid, action string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	validSet := map[string]bool{}
	for _, a := range command.ValidCombatActions {
		validSet[a] = true
	}
	if !validSet[action] {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: fmt.Sprintf("Invalid combat action %q. Valid actions: attack, strike, bash, dodge, parry, cast, pass, flee.", action)},
			},
		}, nil
	}
	ctx := context.Background()
	if sess.CharacterID > 0 && s.charSaver != nil {
		if err := s.charSaver.SaveDefaultCombatAction(ctx, sess.CharacterID, action); err != nil {
			s.logger.Warn("handleCombatDefault: SaveDefaultCombatAction failed", zap.Error(err))
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Failed to save default combat action. Please try again."},
				},
			}, nil
		}
	}
	sess.DefaultCombatAction = action
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: fmt.Sprintf("Default combat action set to: %s", action)},
		},
	}, nil
}

// buildProficiencyEntries constructs a slice of ProficiencyEntry from a proficiency map.
//
// Precondition: profs may be nil (treated as all untrained); level must be >= 1.
// Postcondition: Returns entries in canonical armor-then-weapon order.
func buildProficiencyEntries(profs map[string]string, level int) []*gamev1.ProficiencyEntry {
	displayNames := map[string]string{
		"unarmored":       "Unarmored",
		"light_armor":     "Light Armor",
		"medium_armor":    "Medium Armor",
		"heavy_armor":     "Heavy Armor",
		"simple_weapons":  "Simple Weapons",
		"simple_ranged":   "Simple Ranged",
		"martial_weapons": "Martial Weapons",
		"martial_ranged":  "Martial Ranged",
		"martial_melee":   "Martial Melee",
		"unarmed":         "Unarmed",
		"specialized":     "Specialized",
	}
	order := []struct{ cat, kind string }{
		{"unarmored", "armor"},
		{"light_armor", "armor"},
		{"medium_armor", "armor"},
		{"heavy_armor", "armor"},
		{"simple_weapons", "weapon"},
		{"simple_ranged", "weapon"},
		{"martial_weapons", "weapon"},
		{"martial_ranged", "weapon"},
		{"martial_melee", "weapon"},
		{"unarmed", "weapon"},
		{"specialized", "weapon"},
	}
	entries := make([]*gamev1.ProficiencyEntry, 0, len(order))
	for _, o := range order {
		rank := "untrained"
		if profs != nil {
			if r, ok := profs[o.cat]; ok && r != "" {
				rank = r
			}
		}
		bonus := combatProficiencyBonusForRank(level, rank)
		entries = append(entries, &gamev1.ProficiencyEntry{
			Category: o.cat,
			Name:     displayNames[o.cat],
			Rank:     rank,
			Bonus:    int32(bonus),
			Kind:     o.kind,
		})
	}
	return entries
}

// combatProficiencyBonusForRank returns the PF2E proficiency bonus.
// Untrained: 0. Trained: level+2. Expert: level+4. Master: level+6. Legendary: level+8.
func combatProficiencyBonusForRank(level int, rank string) int {
	switch rank {
	case "trained":
		return level + 2
	case "expert":
		return level + 4
	case "master":
		return level + 6
	case "legendary":
		return level + 8
	default:
		return 0
	}
}

// handleInteract delegates room-equipment interaction to handleUseEquipment.
//
// Precondition: uid must resolve to an active session; instanceID must be non-empty.
// Postcondition: Returns the same ServerEvent as handleUseEquipment.
func (s *GameServiceServer) handleInteract(uid, instanceID string) (*gamev1.ServerEvent, error) {
	return s.handleUseEquipment(uid, instanceID)
}

// handleTabComplete returns sorted, deduplicated tab-completion candidates for the
// given input prefix.
//
// Precondition: uid must resolve to an active player session.
// Postcondition: Returns a non-nil ServerEvent containing a TabCompleteResponse with
// sorted, deduplicated completions. When prefix contains no space, completes
// non-hidden command names and aliases. When prefix starts with "use " or "interact ",
// completes active feat names and room equipment descriptions matching the partial
// word after the command.
func (s *GameServiceServer) handleTabComplete(uid, prefix string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	lower := strings.ToLower(strings.TrimSpace(prefix))

	seen := make(map[string]struct{})
	var completions []string

	addIfNew := func(candidate string) {
		if _, dup := seen[candidate]; !dup {
			seen[candidate] = struct{}{}
			completions = append(completions, candidate)
		}
	}

	if !strings.Contains(lower, " ") {
		// Single-word prefix: complete command names and aliases (non-hidden).
		for _, cmd := range command.BuiltinCommands() {
			if cmd.Category == command.CategoryHidden {
				continue
			}
			if strings.HasPrefix(cmd.Name, lower) {
				addIfNew(cmd.Name)
			}
			for _, alias := range cmd.Aliases {
				if strings.HasPrefix(alias, lower) {
					addIfNew(alias)
				}
			}
		}
	} else if strings.HasPrefix(lower, "use ") || strings.HasPrefix(lower, "interact ") {
		// Contextual prefix: complete feat names and room equipment descriptions.
		spaceIdx := strings.Index(lower, " ")
		partial := lower[spaceIdx+1:]

		// Complete active feat names.
		if s.characterFeatsRepo != nil && s.featRegistry != nil {
			ctx := context.Background()
			featIDs, err := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID)
			if err == nil {
				for _, id := range featIDs {
					f, ok := s.featRegistry.Feat(id)
					if !ok || !f.Active {
						continue
					}
					name := strings.ToLower(f.Name)
					if strings.HasPrefix(name, partial) {
						addIfNew(name)
					}
					// Also match by feat ID.
					if strings.HasPrefix(f.ID, partial) {
						addIfNew(f.ID)
					}
				}
			}
		}

		// Complete room equipment descriptions.
		if s.roomEquipMgr != nil {
			for _, inst := range s.roomEquipMgr.EquipmentInRoom(sess.RoomID) {
				desc := strings.ToLower(inst.Description)
				if desc == "" {
					continue
				}
				if strings.HasPrefix(desc, partial) {
					addIfNew(desc)
				}
			}
		}
	}

	sort.Strings(completions)

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_TabComplete{
			TabComplete: &gamev1.TabCompleteResponse{
				Completions: completions,
			},
		},
	}, nil
}

// handleUse activates an active feat or class feature for the player, or lists all available active abilities.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ServerEvent with UseResponse containing choices or an activation message.
// targetX and targetY are the 0-based grid coordinates of the AoE burst center; -1 means unset / no AoE.
func (s *GameServiceServer) handleUse(uid, abilityID, targetID string, targetX, targetY int32) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// REQ-AH-8: if abilityID matches a backpack item with a substance_id, dispatch to substance handler.
	if abilityID != "" && s.invRegistry != nil && s.substanceReg != nil && sess.Backpack != nil {
		if itemDef, itemOK := s.invRegistry.Item(abilityID); itemOK && itemDef.SubstanceID != "" {
			instances := sess.Backpack.FindByItemDefID(abilityID)
			if len(instances) > 0 {
				return s.handleConsumeSubstanceItem(uid, abilityID)
			}
		}
	}

	// BUG-120: if abilityID matches a plain consumable item in the backpack (Kind==consumable,
	// no SubstanceID), apply its Effect (if any) and remove one from backpack.
	if abilityID != "" && s.invRegistry != nil && sess.Backpack != nil {
		if itemDef, itemOK := s.invRegistry.Item(abilityID); itemOK && itemDef.Kind == inventory.KindConsumable && itemDef.SubstanceID == "" {
			instances := sess.Backpack.FindByItemDefID(abilityID)
			if len(instances) > 0 {
				msg := fmt.Sprintf("You use the %s.", itemDef.Name)
				if itemDef.Effect != nil {
					adapter := &playerActivateAdapter{sess: sess, svc: s}
					var rng inventory.Roller
					if s.dice != nil {
						rng = &diceRollerAdapter{r: s.dice}
					} else {
						rng = NewDurabilityRoller(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
					}
					result := inventory.ApplyConsumable(adapter, itemDef, rng)
					msg = buildConsumableResultMsg(itemDef.Name, result)
				}
				_ = sess.Backpack.Remove(instances[0].InstanceID, 1)
				s.pushInventory(sess)
				return messageEvent(msg), nil
			}
		}
	}

	// REQ-TSN-6: resolve short name to canonical tech ID before all lookup paths.
	if s.techRegistry != nil {
		if def, ok := s.techRegistry.GetByShortName(abilityID); ok {
			abilityID = def.ID
		}
	}

	if s.characterFeatsRepo == nil && s.characterClassFeaturesRepo == nil && s.preparedTechRepo == nil && s.spontaneousUsePoolRepo == nil && s.innateTechRepo == nil && len(sess.KnownTechs) == 0 && len(sess.InnateTechs) == 0 {
		return messageEvent("Ability data is not available."), nil
	}

	ctx := context.Background()
	var active []*gamev1.FeatEntry

	// Collect active feats
	if s.characterFeatsRepo != nil && s.featRegistry != nil {
		featIDs, err := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("getting feats for use: %w", err)
		}
		for _, id := range featIDs {
			f, ok := s.featRegistry.Feat(id)
			if !ok || !f.Active {
				continue
			}
			// Omit exhausted limited feats from the list.
			if f.PreparedUses > 0 && sess.ActiveFeatUses[f.ID] <= 0 {
				continue
			}
			desc := f.Description
			if f.PreparedUses > 0 {
				desc = fmt.Sprintf("%s (%d uses remaining)", f.Description, sess.ActiveFeatUses[f.ID])
			}
			active = append(active, &gamev1.FeatEntry{
				FeatId:       f.ID,
				Name:         f.Name,
				Category:     f.Category,
				Active:       f.Active,
				Description:  desc,
				ActivateText: f.ActivateText,
				IsReaction:   f.Reaction != nil,
				ActionCost:   int32(f.ActionCost),
			})
		}
	}

	// Collect active class features (appended to same list as feats)
	if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
		cfIDs, err := s.characterClassFeaturesRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("getting class features for use: %w", err)
		}
		for _, id := range cfIDs {
			cf, ok := s.classFeatureRegistry.ClassFeature(id)
			if ok && cf.Active {
				active = append(active, &gamev1.FeatEntry{
					FeatId:       cf.ID,
					Name:         cf.Name,
					Category:     "class_feature",
					Active:       cf.Active,
					Description:  cf.Description,
					ActivateText: cf.ActivateText,
					ActionCost:   int32(cf.ActionCost),
				})
			}
		}
	}

	if abilityID == "" {
		// Append non-expended prepared tech entries to active abilities list.
		if len(sess.PreparedTechs) > 0 {
			counts := make(map[string]int)
			for _, slots := range sess.PreparedTechs {
				for _, slot := range slots {
					if slot != nil && !slot.Expended {
						counts[slot.TechID]++
					}
				}
			}
			for techID, remaining := range counts {
				displayName := techID
				var prepIsReaction bool
				var prepActionCost int32
				if s.techRegistry != nil {
					if def, ok := s.techRegistry.Get(techID); ok {
						displayName = def.Name
						prepIsReaction = def.Reaction != nil
						prepActionCost = int32(def.ActionCost)
					}
				}
				active = append(active, &gamev1.FeatEntry{
					FeatId:      techID,
					Name:        displayName,
					Category:    "prepared_tech",
					Active:      true,
					Description: fmt.Sprintf("%d use(s) remaining", remaining),
					IsReaction:  prepIsReaction,
					ActionCost:  prepActionCost,
				})
			}
		}
		// Append spontaneous tech entries with remaining use counts.
		if len(sess.KnownTechs) > 0 {
			spontLevels := make([]int, 0, len(sess.KnownTechs))
			for l := range sess.KnownTechs {
				spontLevels = append(spontLevels, l)
			}
			sort.Ints(spontLevels)
			for _, l := range spontLevels {
				pool := sess.SpontaneousUsePools[l]
				if pool.Remaining <= 0 {
					continue
				}
				for _, techID := range sess.KnownTechs[l] {
					displayName := techID
					var spontIsReaction bool
					var spontActionCost int32
					if s.techRegistry != nil {
						if def, ok := s.techRegistry.Get(techID); ok {
							displayName = def.Name
							spontIsReaction = def.Reaction != nil
							spontActionCost = int32(def.ActionCost)
						}
					}
					active = append(active, &gamev1.FeatEntry{
						FeatId:      techID,
						Name:        displayName,
						Category:    "spontaneous_tech",
						Active:      true,
						Description: fmt.Sprintf("%s (%d uses remaining at level %d)", displayName, pool.Remaining, l),
						IsReaction:  spontIsReaction,
						ActionCost:  spontActionCost,
					})
				}
			}
		}
		// Innate techs (no-arg list mode)
		if len(sess.InnateTechs) > 0 {
			innateIDs := make([]string, 0, len(sess.InnateTechs))
			for id := range sess.InnateTechs {
				innateIDs = append(innateIDs, id)
			}
			sort.Strings(innateIDs)
			for _, id := range innateIDs {
				slot := sess.InnateTechs[id]
				displayName := id
				var innateIsReaction bool
				var innateActionCost int32
				if s.techRegistry != nil {
					if def, ok := s.techRegistry.Get(id); ok {
						displayName = def.Name
						innateIsReaction = def.Reaction != nil
						innateActionCost = int32(def.ActionCost)
					}
				}
				var desc string
				if slot.MaxUses == 0 {
					desc = fmt.Sprintf("%s (unlimited)", displayName)
				} else if slot.UsesRemaining > 0 {
					desc = fmt.Sprintf("%s (%d uses remaining)", displayName, slot.UsesRemaining)
				} else {
					continue // exhausted — omit
				}
				active = append(active, &gamev1.FeatEntry{
					FeatId:      id,
					Name:        displayName,
					Category:    "innate_tech",
					Active:      true,
					Description: desc,
					IsReaction:  innateIsReaction,
					ActionCost:  innateActionCost,
				})
			}
		}
		// Return list of all active abilities for the client to prompt selection.
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_UseResponse{
				UseResponse: &gamev1.UseResponse{Choices: active},
			},
		}, nil
	}

	// Activate the named ability (feat or class feature).
	// Search feats first, then class features, to obtain ConditionID for condition application.
	if s.characterFeatsRepo != nil && s.featRegistry != nil {
		featIDs, err := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("getting feats for activation: %w", err)
		}
		for _, id := range featIDs {
			f, ok := s.featRegistry.Feat(id)
			if !ok || !f.Active {
				continue
			}
			if strings.EqualFold(f.ID, abilityID) || strings.EqualFold(f.Name, abilityID) {
				// Enforce action point cost when the player is in combat.
				if f.ActionCost > 0 && s.combatH != nil {
					if cbt := s.combatH.ActiveCombatForPlayer(uid); cbt != nil {
						ap := s.combatH.RemainingAP(uid)
						cost := f.ActionCost
						if ap < cost {
							return messageEvent(fmt.Sprintf("Not enough action points to use %s: need %d, have %d.", f.Name, cost, ap)), nil
						}
					}
				}
				// Enforce prepared-use limit when feat has PreparedUses > 0.
				if f.PreparedUses > 0 {
					remaining := sess.ActiveFeatUses[f.ID]
					if remaining <= 0 {
						return messageEvent(fmt.Sprintf("You have no uses of %s remaining. Rest to recover.", f.Name)), nil
					}
					sess.ActiveFeatUses[f.ID] = remaining - 1
					s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
				}
				// Feats with special sequenced actions are handled individually before the
				// generic condition-application path.
				if f.ID == "brutal_charge" {
					return s.handleBrutalCharge(uid, targetID)
				}
				if f.ID == "wrath" {
					return s.handleWrath(uid)
				}
				// REQ-60-1 / REQ-60-2: Overpower requires an empty off-hand and consumes 1 AP.
				if f.ID == "overpower" {
					return s.handleOverpower(uid)
				}
				// REQ-BUG-132: Adrenaline Surge requires the player to be Enraged.
				if f.ID == "adrenaline_surge" {
					enraged := s.mentalStateMgr != nil &&
						s.mentalStateMgr.CurrentSeverity(uid, mentalstate.TrackRage) >= mentalstate.SeverityMod
					inWrath := sess != nil && sess.Conditions != nil && sess.Conditions.Has("wrath_active")
					if !enraged && !inWrath {
						return messageEvent("You must be in Wrath or Enraged to use Adrenaline Surge."), nil
					}
				}
				condID := f.ConditionID
				if condID != "" && s.condRegistry != nil {
					// REQ-AOE-2: When feat has aoe_radius > 0 and target coordinates are provided,
					// apply the condition to every living combatant within Chebyshev distance aoe_radius.
					// REQ-2D-5e: AoE does not distinguish friend from foe — all combatants in radius are affected.
					// Sentinel: targetX < 0 or targetY < 0 means no AoE target (use single-target path instead).
					if f.AoeRadius > 0 && (targetX >= 0 && targetY >= 0) {
						var cbt *combat.Combat
						if s.combatH != nil {
							cbt = s.combatH.ActiveCombatForPlayer(uid)
						}
						if cbt == nil {
							return messageEvent(fmt.Sprintf("You must be in combat to use %s.", f.Name)), nil
						}
						center := combat.Combatant{GridX: int(targetX), GridY: int(targetY)}
						if def, ok := s.condRegistry.Get(condID); ok {
							for _, c := range combat.CombatantsInRadius(cbt, center, f.AoeRadius) {
								if condSet := cbt.Conditions[c.ID]; condSet != nil {
									if err := condSet.Apply(c.ID, def, 1, -1); err != nil {
										s.logger.Warn("failed to apply feat AoE condition",
											zap.String("combatant_id", c.ID),
											zap.String("condition_id", condID),
											zap.Error(err),
										)
									}
								} else {
									s.logger.Warn("feat AoE: no condition set for combatant, skipping",
										zap.String("combatant_id", c.ID),
										zap.String("condition_id", condID),
									)
								}
							}
						} else {
							s.logger.Warn("feat AoE condition not found in registry",
								zap.String("condition_id", condID),
							)
						}
					} else if f.ConditionTarget == "foe" {
						// Apply condition to the combat target (foe).
						foeID := targetID
						if foeID == "" {
							foeID = sess.LastCombatTarget
						}
						if foeID == "" {
							return messageEvent(fmt.Sprintf("You need a target to use %s.", f.Name)), nil
						}
						var cbt *combat.Combat
						if s.combatH != nil {
							cbt = s.combatH.ActiveCombatForPlayer(uid)
						}
						if cbt == nil {
							return messageEvent(fmt.Sprintf("You must be in combat to use %s.", f.Name)), nil
						}
						var foe *combat.Combatant
						for _, c := range cbt.Combatants {
							if c.ID == foeID || strings.EqualFold(c.Name, foeID) {
								foe = c
								break
							}
						}
						if foe == nil {
							return messageEvent("Target not found in combat."), nil
						}
						if def, ok := s.condRegistry.Get(condID); ok {
							if condSet := cbt.Conditions[foe.ID]; condSet != nil {
								if err := condSet.Apply(foe.ID, def, 1, -1); err != nil {
									s.logger.Warn("failed to apply feat condition to foe",
										zap.String("foe_id", foe.ID),
										zap.String("condition_id", condID),
										zap.Error(err),
									)
								}
							}
						} else {
							s.logger.Warn("feat foe condition not found in registry",
								zap.String("condition_id", condID),
							)
						}
					} else {
						// Apply condition to self (default).
						// REQ-FEAT-COND: Apply to combat condition set when in combat so that
						// AC/damage modifiers take effect immediately; fall back to session conditions.
						// RequiresCombat feats may only be activated during an active encounter.
						if f.RequiresCombat {
							var inCombat bool
							if s.combatH != nil {
								inCombat = s.combatH.ActiveCombatForPlayer(uid) != nil
							}
							if !inCombat {
								return messageEvent(fmt.Sprintf("You must be in combat to use %s.", f.Name)), nil
							}
						}
						if def, ok := s.condRegistry.Get(condID); ok {
							var applyErr error
							var selfCbt *combat.Combat
							if s.combatH != nil {
								selfCbt = s.combatH.ActiveCombatForPlayer(uid)
							}
							if selfCbt != nil {
								if applySet := selfCbt.Conditions[sess.UID]; applySet != nil {
									applyErr = applySet.Apply(sess.UID, def, 1, -1)
								}
							} else if sess.Conditions != nil {
								applyErr = sess.Conditions.Apply(sess.UID, def, 1, -1)
							}
							if applyErr != nil {
								s.logger.Warn("failed to apply feat condition",
									zap.String("condition_id", condID),
									zap.Error(applyErr),
								)
							}
						} else {
							s.logger.Warn("feat condition not found in registry",
								zap.String("condition_id", condID),
							)
						}
					}
				}
				msg := f.ActivateText
				if f.PreparedUses > 0 {
					msg = fmt.Sprintf("%s (%d uses remaining.)", f.ActivateText, sess.ActiveFeatUses[f.ID])
				}
				return &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_UseResponse{
						UseResponse: &gamev1.UseResponse{Message: msg},
					},
				}, nil
			}
		}
	}
	if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
		cfIDs, err := s.characterClassFeaturesRepo.GetAll(ctx, sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("getting class features for activation: %w", err)
		}
		for _, id := range cfIDs {
			cf, ok := s.classFeatureRegistry.ClassFeature(id)
			if !ok || !cf.Active {
				continue
			}
			if strings.EqualFold(cf.ID, abilityID) || strings.EqualFold(cf.Name, abilityID) {
				condID := cf.ConditionID
				if condID != "" && s.condRegistry != nil {
					if def, ok := s.condRegistry.Get(condID); ok {
						var applyErr error
						var cbt *combat.Combat
						if s.combatH != nil {
							cbt = s.combatH.ActiveCombatForPlayer(uid)
						}
						if cbt != nil {
							// Apply to the combat condition set so AC/damage modifiers take effect.
							if applySet := cbt.Conditions[sess.UID]; applySet != nil {
								applyErr = applySet.Apply(sess.UID, def, 1, -1)
							}
						} else if sess.Conditions != nil {
							// Outside combat, fall back to session-level conditions.
							applyErr = sess.Conditions.Apply(sess.UID, def, 1, -1)
						}
						if applyErr != nil {
							s.logger.Warn("failed to apply class feature condition",
								zap.String("condition_id", condID),
								zap.Error(applyErr),
							)
						}
					} else {
						s.logger.Warn("class feature condition not found in registry",
							zap.String("condition_id", condID),
						)
					}
				}
				cfMsg := cf.ActivateText
				if condID != "" && s.condRegistry != nil {
					if def, ok := s.condRegistry.Get(condID); ok {
						cfMsg = fmt.Sprintf("%s (%s)", cfMsg, def.Name)
					}
				}
				return &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_UseResponse{
						UseResponse: &gamev1.UseResponse{Message: cfMsg},
					},
				}, nil
			}
		}
	}
	// techAPCost returns the AP cost for a tech in combat, or 0 if not in combat or no cost.
	// Prepared and spontaneous techs spend AP equal to their action_cost field.
	// Innate techs are explicitly exempt (they fire free and immediately).
	techAPCost := func(techDef *technology.TechnologyDef) int {
		if techDef == nil || techDef.ActionCost <= 0 || s.combatH == nil {
			return 0
		}
		if s.combatH.ActiveCombatForPlayer(uid) == nil {
			return 0
		}
		return techDef.ActionCost
	}

	// Attempt prepared tech activation if no feat/class-feature matched.
	if s.preparedTechRepo != nil && len(sess.PreparedTechs) > 0 {
		// Parse level-encoded ability IDs (e.g. "frost_bolt:2" from level-aware hotbar slots).
		// techID is the base tech identifier; targetLevel==0 means search all levels.
		techID, targetLevel := parseTechRef(abilityID)
		// Check AP cost before searching slots so we fail fast without expending anything.
		var preparedTechDef *technology.TechnologyDef
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(techID); ok {
				preparedTechDef = def
			}
		}
		if cost := techAPCost(preparedTechDef); cost > 0 {
			ap := s.combatH.RemainingAP(uid)
			if ap < cost {
				name := techID
				if preparedTechDef != nil {
					name = preparedTechDef.Name
				}
				return messageEvent(fmt.Sprintf("Not enough AP to use %s (need %d, have %d).", name, cost, ap)), nil
			}
		}
		// Build the ordered list of levels to search.
		// When targetLevel > 0, search only that level. Otherwise search all levels ascending.
		var levels []int
		if targetLevel > 0 {
			if _, ok := sess.PreparedTechs[targetLevel]; ok {
				levels = []int{targetLevel}
			}
		} else {
			for lvl := range sess.PreparedTechs {
				levels = append(levels, lvl)
			}
			sort.Ints(levels)
		}
		foundInPrepared := false
		for _, lvl := range levels {
			for idx, slot := range sess.PreparedTechs[lvl] {
				if slot == nil || slot.TechID != techID {
					continue
				}
				foundInPrepared = true
				if slot.Expended {
					continue
				}
				// Found a non-expended slot — expend it then queue for round resolution (in combat)
				// or fire immediately (out of combat).
				if err := s.preparedTechRepo.SetExpended(ctx, sess.CharacterID, lvl, idx, true); err != nil {
					s.logger.Warn("handleUse: SetExpended failed",
						zap.String("uid", uid),
						zap.String("techID", techID),
						zap.Error(err))
				}
				sess.PreparedTechs[lvl][idx].Expended = true
				s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
				// GH #224: heighten delta = slot level - tech native level, clamped
				// to >= 0. preparedTechDef may be nil when the tech registry is
				// unavailable in tests; treat delta as 0 in that case.
				heightenDelta := 0
				if preparedTechDef != nil {
					heightenDelta = lvl - preparedTechDef.Level
					if heightenDelta < 0 {
						heightenDelta = 0
					}
				}
				techDisplay := techID
				if preparedTechDef != nil && preparedTechDef.Name != "" {
					techDisplay = preparedTechDef.Name
				}
				if cost := techAPCost(preparedTechDef); cost > 0 && s.combatH.ActiveCombatForPlayer(uid) != nil {
					// In combat: queue for round resolution at player's initiative.
					if err := s.combatH.QueueTechUse(uid, techID, techDisplay, targetID, cost, targetX, targetY); err != nil {
						return messageEvent(fmt.Sprintf("Could not queue tech: %s", err.Error())), nil
					}
					return nil, nil
				}
				return s.activateTechWithEffectsAndHeighten(sess, uid, techID, targetID, fmt.Sprintf("You activate %s.", techDisplay), nil, targetX, targetY, heightenDelta)
			}
		}
		if foundInPrepared {
			// All prepared copies are expended — fall through to room equip, not innate.
			// REQ-USE-1: fall through to room equipment before reporting no match.
			if evt, err := s.tryRoomEquipFallback(uid, sess.RoomID, techID); evt != nil || err != nil {
				return evt, err
			}
			return messageEvent(fmt.Sprintf("No prepared uses of %s remaining.", techID)), nil
		}
		// Tech not in prepared slots at all — fall through to spontaneous/innate lookup.
	}
	// Spontaneous tech lookup — only if no feat/class-feature/prepared-tech matched.
	if len(sess.KnownTechs) > 0 {
		levels := make([]int, 0, len(sess.KnownTechs))
		for l := range sess.KnownTechs {
			levels = append(levels, l)
		}
		sort.Ints(levels)
		foundLevel := -1
		for _, l := range levels {
			for _, tid := range sess.KnownTechs[l] {
				if tid == abilityID {
					foundLevel = l
					break
				}
			}
			if foundLevel >= 0 {
				break
			}
		}
		if foundLevel < 0 {
			// Tech not in spontaneous pool — fall through to innate/room-equip paths before
			// reporting no match. Do NOT return here: innate techs must be checked next.
			goto innateCheck
		}
		pool := sess.SpontaneousUsePools[foundLevel]
		if pool.Remaining <= 0 {
			return messageEvent(fmt.Sprintf("No level %d uses remaining.", foundLevel)), nil
		}
		// Check AP cost before decrementing the pool.
		var spontTechDef *technology.TechnologyDef
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(abilityID); ok {
				spontTechDef = def
			}
		}
		cost := techAPCost(spontTechDef)
		if cost > 0 {
			ap := s.combatH.RemainingAP(uid)
			if ap < cost {
				name := abilityID
				if spontTechDef != nil {
					name = spontTechDef.Name
				}
				return messageEvent(fmt.Sprintf("Not enough AP to use %s (need %d, have %d).", name, cost, ap)), nil
			}
		}
		// Decrement the spontaneous pool before activating/queueing.
		if s.spontaneousUsePoolRepo != nil {
			if err := s.spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, foundLevel); err != nil {
				s.logger.Warn("handleUse: Decrement spontaneous pool failed",
					zap.String("uid", uid),
					zap.String("techID", abilityID),
					zap.Error(err))
			}
		}
		pool.Remaining--
		sess.SpontaneousUsePools[foundLevel] = pool
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		spontaneousDisplay := abilityID
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(abilityID); ok && def.Name != "" {
				spontaneousDisplay = def.Name
			}
		}
		if cost > 0 && s.combatH.ActiveCombatForPlayer(uid) != nil {
			// In combat: queue for round resolution at player's initiative.
			if err := s.combatH.QueueTechUse(uid, abilityID, spontaneousDisplay, targetID, cost, targetX, targetY); err != nil {
				return messageEvent(fmt.Sprintf("Could not queue tech: %s", err.Error())), nil
			}
			return nil, nil
		}
		return s.activateTechWithEffects(sess, uid, abilityID, targetID, fmt.Sprintf("You activate %s. (%d uses remaining at level %d.)", spontaneousDisplay, pool.Remaining, foundLevel), nil, targetX, targetY)
	}
// innateCheck is the target for goto from the spontaneous path when the tech is not
// found in KnownTechs. This allows innate techs to be checked even when the player
// also has spontaneous techs, preventing "You don't know <tech>" false negatives.
innateCheck:
	// Innate tech activation. Innate techs with action_cost 0 fire immediately (cantrip parity);
	// innate techs with action_cost > 0 queue for round resolution in combat, same as other techs.
	if s.innateTechRepo != nil {
		// Parse level-encoded ability IDs (e.g. "multi_round_kinetic_volley:2") so that the
		// base tech ID is used for the InnateTechs map lookup. InnateTechs is keyed by base ID only.
		innateTechBaseID, _ := parseTechRef(abilityID)
		if slot, ok := sess.InnateTechs[innateTechBaseID]; ok {
			// Look up tech def once — used for reaction check, action cost, and display name.
			var innateTechDef *technology.TechnologyDef
			if s.techRegistry != nil {
				if def, ok2 := s.techRegistry.Get(innateTechBaseID); ok2 {
					innateTechDef = def
				}
			}
			// REQ-CRX6: block manual use for techs that fire as reactions.
			if innateTechDef != nil && innateTechDef.Reaction != nil {
				return messageEvent(fmt.Sprintf("%s fires automatically as a reaction and cannot be activated manually.", innateTechDef.Name)), nil
			}
			// In combat with action_cost > 0: queue for round resolution at player's initiative.
			if innateTechDef != nil && innateTechDef.ActionCost > 0 && s.combatH != nil && s.combatH.ActiveCombatForPlayer(uid) != nil {
				ap := s.combatH.RemainingAP(uid)
				if ap < innateTechDef.ActionCost {
					return messageEvent(fmt.Sprintf("Not enough AP to use %s (need %d, have %d).", innateTechDef.Name, innateTechDef.ActionCost, ap)), nil
				}
				if err := s.combatH.QueueTechUse(uid, innateTechBaseID, innateTechDef.Name, targetID, innateTechDef.ActionCost, targetX, targetY); err != nil {
					return messageEvent(fmt.Sprintf("Could not queue tech: %s", err.Error())), nil
				}
				return nil, nil
			}
			if slot.MaxUses != 0 && slot.UsesRemaining <= 0 {
				return messageEvent(fmt.Sprintf("No uses of %s remaining.", innateTechBaseID)), nil
			}
			name := innateTechBaseID
			if innateTechDef != nil {
				name = innateTechDef.Name
			}
			if slot.MaxUses != 0 {
				if err := s.innateTechRepo.Decrement(ctx, sess.CharacterID, innateTechBaseID); err != nil {
					return nil, fmt.Errorf("handleUse: decrement innate %s: %w", innateTechBaseID, err)
				}
				slot.UsesRemaining--
				sess.InnateTechs[innateTechBaseID] = slot
				s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
				return s.activateTechWithEffects(sess, uid, innateTechBaseID, targetID, fmt.Sprintf("You activate %s. (%d uses remaining.)", name, slot.UsesRemaining), nil, targetX, targetY)
			}
			return s.activateTechWithEffects(sess, uid, innateTechBaseID, targetID, fmt.Sprintf("You activate %s.", name), nil, targetX, targetY)
		}
		// REQ-USE-1: fall through to room equipment before reporting no match.
		if evt, err := s.tryRoomEquipFallback(uid, sess.RoomID, abilityID); evt != nil || err != nil {
			return evt, err
		}
		// GH #236: the tech may exist in the registry (e.g. granted via an
		// archetype pool) but not yet in prepared / known / innate slots on
		// this character. Emit a descriptive message rather than the
		// misleading "you don't have innate tech" wording, which implied the
		// tech was missing entirely.
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(innateTechBaseID); ok && def != nil {
				name := def.Name
				if name == "" {
					name = innateTechBaseID
				}
				return messageEvent(fmt.Sprintf("You don't have %s prepared or available. Prepare it from your tech pool or select it as an innate first.", name)), nil
			}
		}
		return messageEvent(fmt.Sprintf("You don't have %s prepared or available.", innateTechBaseID)), nil
	}
	// REQ-USE-1: fall through to room equipment when no feat/ability matched.
	if evt, err := s.tryRoomEquipFallback(uid, sess.RoomID, abilityID); evt != nil || err != nil {
		return evt, err
	}
	return messageEvent(fmt.Sprintf("You don't have an active ability named %q.", abilityID)), nil
}

// tryRoomEquipFallback attempts to activate room equipment with the given instanceID
// for the player in the given room. Returns nil, nil when no equipment match is found.
//
// Precondition: uid must resolve to an active session; roomID and instanceID are non-empty.
// Postcondition: Returns non-nil event when equipment is found and activated; nil, nil on miss.
func (s *GameServiceServer) tryRoomEquipFallback(uid, roomID, instanceID string) (*gamev1.ServerEvent, error) {
	if s.roomEquipMgr == nil {
		return nil, nil
	}
	inst := s.roomEquipMgr.GetInstance(roomID, instanceID)
	if inst == nil {
		return nil, nil
	}
	return s.handleUseEquipment(uid, inst.InstanceID)
}

// handleAmpedUse activates a spontaneous tech that has amped_effects defined,
// prompting the player to select a slot level when one is not provided in the arg.
// Called from the Session handler (pre-dispatch) when the UseRequest targets
// a spontaneous tech with AmpedLevel > 0.
//
// Precondition: tech is non-nil, tech.UsageType == UsageSpontaneous, tech.AmpedLevel > 0.
// Postcondition: the selected slot-level pool is decremented; resolved tech effects are applied.
func (s *GameServiceServer) handleAmpedUse(
	uid string,
	req *gamev1.UseRequest,
	stream gamev1.GameService_SessionServer,
) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	abilityID := req.GetFeatId()
	arg := req.GetTarget() // "targetID [level]" or "level" or ""

	// Fetch tech definition.
	techDef, ok := s.techRegistry.Get(abilityID)
	if !ok {
		return messageEvent(fmt.Sprintf("Unknown tech: %s.", abilityID)), nil
	}

	// Find which level this tech lives at in the player's spontaneous pool.
	foundLevel := -1
	levels := make([]int, 0, len(sess.KnownTechs))
	for l := range sess.KnownTechs {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		for _, tid := range sess.KnownTechs[l] {
			if tid == abilityID {
				foundLevel = l
				break
			}
		}
		if foundLevel >= 0 {
			break
		}
	}
	if foundLevel < 0 {
		return messageEvent(fmt.Sprintf("You don't know %s.", abilityID)), nil
	}

	// Parse tokens: disambiguate targetID from slotLevel.
	// Any positive-integer token is treated as slotLevel; the rest is targetID.
	// A token starting with a digit but failing positive-int parsing → "Invalid level: <token>."
	tokens := strings.Fields(arg)
	targetID := ""
	slotLevel := 0
	intCount := 0
	for _, tok := range tokens {
		if n, err := strconv.Atoi(tok); err == nil && n > 0 {
			intCount++
			slotLevel = n
		} else if len(tok) > 0 && tok[0] >= '0' && tok[0] <= '9' {
			return messageEvent(fmt.Sprintf("Invalid level: %s.", tok)), nil
		} else {
			if targetID != "" {
				targetID += " "
			}
			targetID += tok
		}
	}
	if intCount > 1 {
		return messageEvent("Invalid arguments: specify at most one level."), nil
	}

	if slotLevel != 0 {
		// Validate explicit level.
		if slotLevel < techDef.Level {
			return messageEvent(fmt.Sprintf("Cannot use %s below its native level (%d).", techDef.ID, techDef.Level)), nil
		}
		pool := sess.SpontaneousUsePools[slotLevel]
		if pool.Remaining <= 0 {
			return messageEvent(fmt.Sprintf("No level-%d uses remaining.", slotLevel)), nil
		}
	} else {
		// No explicit level — build list of valid levels from sess.SpontaneousUsePools.
		poolLevels := make([]int, 0, len(sess.SpontaneousUsePools))
		for l := range sess.SpontaneousUsePools {
			poolLevels = append(poolLevels, l)
		}
		sort.Ints(poolLevels)
		var validOptions []string
		var validLevels []int
		for _, l := range poolLevels {
			if l >= techDef.Level && sess.SpontaneousUsePools[l].Remaining > 0 {
				validOptions = append(validOptions, fmt.Sprintf("%d (%d remaining)", l, sess.SpontaneousUsePools[l].Remaining))
				validLevels = append(validLevels, l)
			}
		}
		if len(validLevels) == 0 {
			return messageEvent("No uses remaining at any level."), nil
		}
		if len(validLevels) == 1 {
			slotLevel = validLevels[0]
		} else {
			choices := &ruleset.FeatureChoices{
				Key:     "slot_level",
				Prompt:  "Use at what level?",
				Options: validOptions,
			}
			chosen, err := s.promptFeatureChoice(stream, "slot_level", choices, sess.Headless)
			if err != nil {
				return nil, fmt.Errorf("handleAmpedUse: prompt: %w", err)
			}
			if n, err2 := strconv.Atoi(strings.Fields(chosen)[0]); err2 == nil {
				slotLevel = n
			} else {
				return messageEvent("Invalid level selection."), nil
			}
		}
	}

	// Decrement the selected slot-level pool.
	ctx := context.Background()
	if s.spontaneousUsePoolRepo != nil {
		if err := s.spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, slotLevel); err != nil {
			s.logger.Warn("handleAmpedUse: Decrement spontaneous pool failed",
				zap.String("uid", uid),
				zap.String("techID", abilityID),
				zap.Error(err))
		}
	}
	pool := sess.SpontaneousUsePools[slotLevel]
	pool.Remaining--
	sess.SpontaneousUsePools[slotLevel] = pool

	// Resolve tech (amped or base) and activate inline (avoids registry re-fetch).
	resolvedTech := technology.TechAtSlotLevel(techDef, slotLevel)

	var cbt *combat.Combat
	if s.combatH != nil {
		cbt = s.combatH.ActiveCombatForPlayer(uid)
	}

	// REQ-AOE-1: When tech has aoe_radius > 0 and target coordinates are provided,
	// collect all living combatants within Chebyshev distance aoe_radius of the target square.
	// Sentinel: techTargetX < 0 or techTargetY < 0 means no AoE target (use single-target path instead).
	var techTargets []*combat.Combatant
	techTargetX, techTargetY := req.GetTargetX(), req.GetTargetY()
	if resolvedTech.AoeRadius > 0 && (techTargetX >= 0 && techTargetY >= 0) && cbt != nil {
		center := combat.Combatant{GridX: int(techTargetX), GridY: int(techTargetY)}
		techTargets = combat.CombatantsInRadius(cbt, center, resolvedTech.AoeRadius)
	} else {
		target, errMsg := s.resolveUseTarget(uid, targetID, resolvedTech)
		if errMsg != "" {
			return messageEvent(errMsg), nil
		}
		if resolvedTech.Targets == technology.TargetsAllEnemies && cbt != nil {
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindNPC && !c.IsDead() {
					techTargets = append(techTargets, c)
				}
			}
		} else if target != nil {
			techTargets = []*combat.Combatant{target}
		}
	}

	activationLine := fmt.Sprintf("%s activated. Level-%d uses remaining: %d.", resolvedTech.Name, slotLevel, pool.Remaining)
	msgs := ResolveTechEffects(sess, resolvedTech, techTargets, cbt, s.condRegistry, globalRandSrc{}, nil)
	allMsgs := append([]string{activationLine}, msgs...)
	return messageEvent(strings.Join(allMsgs, "\n")), nil
}

// resolveUseTarget returns the combat target for the given tech and targetID.
//
// Preconditions:
//   - uid must correspond to a loaded session (not verified here — caller has already fetched sess).
//   - tech must be non-nil.
//
// Postconditions:
//   - Returns (nil, "") for self/utility/no-roll techs — target resolution skipped.
//   - Returns (nil, errorMsg) when the caller has provided an invalid targetID or is not in combat.
//   - Returns (*Combatant, "") when a valid target is resolved.
func (s *GameServiceServer) resolveUseTarget(uid, targetID string, tech *technology.TechnologyDef) (*combat.Combatant, string) {
	// Self-targeting, no-roll, and area techs never need single-target resolution.
	if tech.Targets == "self" || tech.Resolution == "" || tech.Resolution == "none" {
		return nil, ""
	}
	if tech.Targets == technology.TargetsAllEnemies {
		return nil, "" // area targeting — handled by activateTechWithEffects
	}
	if s.combatH == nil {
		if targetID != "" {
			return nil, "You are not in combat."
		}
		return nil, "Specify a target: use <tech> <target>"
	}
	cbt := s.combatH.ActiveCombatForPlayer(uid)
	if cbt == nil {
		if targetID != "" {
			return nil, "You are not in combat."
		}
		return nil, "Specify a target: use <tech> <target>"
	}
	// In combat — find target.
	if targetID == "" {
		// Default: first living NPC combatant.
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC && !c.IsDead() {
				return c, ""
			}
		}
		return nil, "No valid target in combat."
	}
	// Named target — case-insensitive prefix match.
	lower := strings.ToLower(targetID)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && !c.IsDead() &&
			strings.HasPrefix(strings.ToLower(c.Name), lower) {
			return c, ""
		}
	}
	return nil, fmt.Sprintf("No combatant named %q in this fight.", targetID)
}

// activateTechWithEffects resolves tech effects after a use has been expended.
//
// Preconditions:
//   - sess must be non-nil.
//   - uid must match sess.UID.
//   - abilityID must be a valid tech ID known to s.techRegistry (or may be absent for legacy feats).
//   - fallbackMsg is returned verbatim when s.techRegistry is nil or the tech is not registered.
//   - targetX and targetY are the 0-based grid coordinates of an AoE burst center;
//     -1 means unset / no AoE (single-target or all-enemies resolution is used instead).
//
// Postconditions:
//   - If s.techRegistry is nil or the tech is not registered, falls back to fallbackMsg.
//   - Returns a non-nil ServerEvent on success.
func (s *GameServiceServer) activateTechWithEffects(sess *session.PlayerSession, uid, abilityID, targetID, fallbackMsg string, querier RoomQuerier, targetX, targetY int32) (*gamev1.ServerEvent, error) {
	return s.activateTechWithEffectsAndHeighten(sess, uid, abilityID, targetID, fallbackMsg, querier, targetX, targetY, 0)
}

// activateTechWithEffectsAndHeighten is activateTechWithEffects plus a
// heighten delta (slotLevel - techLevel). The delta propagates through
// ResolveTechEffectsWithHeighten so damage effects with Projectiles > 0
// scale up by one projectile per heighten level (GH #224).
func (s *GameServiceServer) activateTechWithEffectsAndHeighten(sess *session.PlayerSession, uid, abilityID, targetID, fallbackMsg string, querier RoomQuerier, targetX, targetY int32, heightenDelta int) (*gamev1.ServerEvent, error) {
	if s.techRegistry == nil {
		return messageEvent(fallbackMsg), nil
	}
	techDef, ok := s.techRegistry.Get(abilityID)
	if !ok {
		return messageEvent(fallbackMsg), nil
	}
	// REQ-FP-3/REQ-FP-4/REQ-FP-5/REQ-FP-6: Enforce Focus Point cost before activation.
	if techDef.FocusCost {
		next, ok := focuspoints.Spend(sess.FocusPoints, sess.MaxFocusPoints)
		if !ok {
			return messageEvent(fmt.Sprintf("Not enough Focus Points. (%d/%d)", sess.FocusPoints, sess.MaxFocusPoints)), nil
		}
		sess.FocusPoints = next
		if s.charSaver != nil {
			if err := s.charSaver.SaveFocusPoints(context.Background(), sess.CharacterID, sess.FocusPoints); err != nil {
				return nil, fmt.Errorf("persist focus points: %w", err)
			}
		}
	}

	var cbt *combat.Combat
	if s.combatH != nil {
		cbt = s.combatH.ActiveCombatForPlayer(uid)
	}

	var techTargets []*combat.Combatant
	// REQ-AOE-1: When tech has aoe_radius > 0 and target coordinates are provided,
	// collect all living combatants within Chebyshev distance aoe_radius of the target square.
	// Sentinel: targetX < 0 or targetY < 0 means no AoE target (use single-target path instead).
	if techDef.AoeRadius > 0 && (targetX >= 0 && targetY >= 0) && cbt != nil {
		center := combat.Combatant{GridX: int(targetX), GridY: int(targetY)}
		techTargets = combat.CombatantsInRadius(cbt, center, techDef.AoeRadius)
	} else {
		target, errMsg := s.resolveUseTarget(uid, targetID, techDef)
		if errMsg != "" {
			return messageEvent(errMsg), nil
		}
		if techDef.Targets == technology.TargetsAllEnemies && cbt != nil { //nolint:gocritic
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindNPC && !c.IsDead() {
					techTargets = append(techTargets, c)
				}
			}
		} else if target != nil {
			techTargets = []*combat.Combatant{target}
		} else if (techDef.Resolution == "" || techDef.Resolution == "none") &&
			techDef.Targets == technology.TargetsSingle && cbt != nil {
			// No-roll single-target tech (e.g. multi_round_kinetic_volley): resolveUseTarget
			// skips resolution for resolution:"none", but on_apply damage effects require an NPC target.
			// Find the default target from active combat.
			if targetID == "" {
				for _, c := range cbt.Combatants {
					if c.Kind == combat.KindNPC && !c.IsDead() {
						techTargets = []*combat.Combatant{c}
						break
					}
				}
			} else {
				lower := strings.ToLower(targetID)
				for _, c := range cbt.Combatants {
					if c.Kind == combat.KindNPC && !c.IsDead() &&
						strings.HasPrefix(strings.ToLower(c.Name), lower) {
						techTargets = []*combat.Combatant{c}
						break
					}
				}
			}
		}
	}

	msgs := ResolveTechEffectsWithHeighten(sess, techDef, techTargets, cbt, s.condRegistry, globalRandSrc{}, querier, heightenDelta)
	return messageEvent(strings.Join(msgs, "\n")), nil
}

// activateTechWithEffectsWithCombat is a variant of activateTechWithEffects that accepts
// an already-resolved *combat.Combat instead of calling ActiveCombatForPlayer internally.
// Use this inside callbacks that are invoked while combatMu is already held to prevent deadlock.
//
// Precondition: sess and cbt may be nil; cbt == nil falls back to no AoE/target-list logic.
// Postcondition: same as activateTechWithEffects.
func (s *GameServiceServer) activateTechWithEffectsWithCombat(sess *session.PlayerSession, uid, abilityID, targetID, fallbackMsg string, querier RoomQuerier, targetX, targetY int32, cbt *combat.Combat) (*gamev1.ServerEvent, error) {
	if s.techRegistry == nil {
		return messageEvent(fallbackMsg), nil
	}
	techDef, ok := s.techRegistry.Get(abilityID)
	if !ok {
		return messageEvent(fallbackMsg), nil
	}
	// REQ-FP-3/REQ-FP-4/REQ-FP-5/REQ-FP-6: Enforce Focus Point cost before activation.
	if techDef.FocusCost {
		next, ok := focuspoints.Spend(sess.FocusPoints, sess.MaxFocusPoints)
		if !ok {
			return messageEvent(fmt.Sprintf("Not enough Focus Points. (%d/%d)", sess.FocusPoints, sess.MaxFocusPoints)), nil
		}
		sess.FocusPoints = next
		if s.charSaver != nil {
			if err := s.charSaver.SaveFocusPoints(context.Background(), sess.CharacterID, sess.FocusPoints); err != nil {
				return nil, fmt.Errorf("persist focus points: %w", err)
			}
		}
	}

	// Use the pre-acquired cbt directly instead of calling resolveUseTarget (which would
	// re-acquire combatMu and deadlock since the caller already holds it).
	var techTargets []*combat.Combatant
	if techDef.AoeRadius > 0 && (targetX >= 0 && targetY >= 0) && cbt != nil {
		center := combat.Combatant{GridX: int(targetX), GridY: int(targetY)}
		techTargets = combat.CombatantsInRadius(cbt, center, techDef.AoeRadius)
	} else if techDef.Targets == technology.TargetsAllEnemies && cbt != nil {
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC && !c.IsDead() {
				techTargets = append(techTargets, c)
			}
		}
	} else if techDef.Targets != "self" && techDef.Targets != technology.TargetsAllEnemies &&
		techDef.Targets != technology.TargetsAllAllies && techDef.Targets != technology.TargetsZone && cbt != nil {
		// Single-target attack, save, or no-roll (resolution none/""): find the named target or
		// default to first living NPC. Includes resolution:"none" techs (e.g. kinetic volley) that
		// still require an NPC target for damage application.
		if targetID == "" {
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindNPC && !c.IsDead() {
					techTargets = []*combat.Combatant{c}
					break
				}
			}
		} else {
			lower := strings.ToLower(targetID)
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindNPC && !c.IsDead() &&
					strings.HasPrefix(strings.ToLower(c.Name), lower) {
					techTargets = []*combat.Combatant{c}
					break
				}
			}
		}
	}

	msgs := ResolveTechEffects(sess, techDef, techTargets, cbt, s.condRegistry, globalRandSrc{}, querier)
	return messageEvent(strings.Join(msgs, "\n")), nil
}

// handleAction resolves a player-activated class feature action.
//
// Precondition: uid must resolve to an active session; req.Name may be empty (list mode).
// Postcondition: Returns nil, nil on success; nil and an error from ActionHandler.Handle on failure.
func (s *GameServiceServer) handleAction(uid string, req *gamev1.ActionRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	ctx := context.Background()
	if err := s.actionH.Handle(ctx, sess, req.GetName(), req.GetTarget()); err != nil {
		return nil, err
	}
	return nil, nil
}

// applySkillCheckEffect applies a mechanical effect from a skill check outcome.
//
// Precondition: sess and effect must not be nil; s.dice must be non-nil for damage effects;
// s.condRegistry may be nil; condition effects are silently skipped when it is nil;
// roomID must be non-empty for reveal effects.
// Postcondition: damage effects reduce sess.CurrentHP to a minimum of 0;
// condition effects add the named condition to sess.Conditions;
// reveal effects un-hide the exit in effect.Target from the given room.
func (s *GameServiceServer) applySkillCheckEffect(sess *session.PlayerSession, effect *skillcheck.Effect, roomID string) {
	if effect == nil {
		return
	}
	switch effect.Type {
	case "damage":
		if effect.Formula == "" || s.dice == nil {
			return
		}
		dmg, err := s.dice.RollExpr(effect.Formula)
		if err != nil {
			s.logger.Warn("skill check damage formula error",
				zap.String("formula", effect.Formula),
				zap.Error(err),
			)
			return
		}
		sess.CurrentHP -= dmg.Total()
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		s.checkNonCombatDeath(sess.UID, sess) // REQ-BUG100-1
	case "condition":
		if effect.ID == "" || sess.Conditions == nil || s.condRegistry == nil {
			return
		}
		def, ok := s.condRegistry.Get(effect.ID)
		if !ok {
			s.logger.Warn("skill check condition not found",
				zap.String("condition_id", effect.ID),
			)
			return
		}
		if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
			s.logger.Warn("skill check condition apply failed",
				zap.String("condition_id", effect.ID),
				zap.Error(err),
			)
		}
	case "reveal":
		if effect.Target == "" || roomID == "" {
			return
		}
		if s.world == nil {
			return
		}
		revealed := s.world.RevealExit(roomID, effect.Target)
		if !revealed {
			s.logger.Debug("skill check reveal: no hidden exit found",
				zap.String("room_id", roomID),
				zap.String("direction", effect.Target),
			)
		}
	}
}

// promptFeatureChoice sends a numbered prompt for the given FeatureChoices block
// over stream and reads a single numeric response.
//
// Precondition: stream must be writable; choices must be non-nil with non-empty Options.
// Postcondition: Returns (selectedValue, nil) on valid input, ("", nil) on invalid input,
// ("", err) on stream or send/recv failure.
// wrapOption formats a single numbered option for display, word-wrapping long
// text at width characters total. The first line begins with prefix; continuation
// lines are indented with spaces equal to len(prefix).
func wrapOption(prefix, text string, width int) string {
	indent := strings.Repeat(" ", len(prefix))
	words := strings.Fields(text)
	if len(words) == 0 {
		return prefix
	}

	var sb strings.Builder
	currentPrefix := prefix
	currentWidth := width - len(prefix)
	lineWords := []string{}
	lineLen := 0

	for _, w := range words {
		// +1 for space separator (except first word on a line)
		addLen := len(w)
		if len(lineWords) > 0 {
			addLen++
		}
		if len(lineWords) > 0 && lineLen+addLen > currentWidth {
			// Flush current line
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(currentPrefix)
			sb.WriteString(strings.Join(lineWords, " "))
			// Switch to continuation prefix/width
			currentPrefix = indent
			currentWidth = width - len(indent)
			lineWords = []string{w}
			lineLen = len(w)
		} else {
			lineWords = append(lineWords, w)
			lineLen += addLen
		}
	}
	// Write final line
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(currentPrefix)
	sb.WriteString(strings.Join(lineWords, " "))
	return sb.String()
}

// choicePromptPayload is the JSON structure sent inside the "\x00choice\x00" sentinel.
type choicePromptPayload struct {
	FeatureID   string           `json:"featureId"`
	Prompt      string           `json:"prompt"`
	Options     []string         `json:"options"`
	SlotContext *techSlotContext `json:"slotContext,omitempty"`
}

// techSlotContext carries prepared-slot metadata to the frontend so it can render
// slot tabs, level headers, and heightened-tier badges.
type techSlotContext struct {
	SlotNum    int `json:"slotNum"`
	TotalSlots int `json:"totalSlots"`
	SlotLevel  int `json:"slotLevel"`
}

// maxChoiceAttempts is the maximum number of stream.Recv iterations before
// promptFeatureChoice gives up waiting for a valid choice.
const maxChoiceAttempts = 100

func (s *GameServiceServer) promptFeatureChoice(
	stream gamev1.GameService_SessionServer,
	featureID string,
	choices *ruleset.FeatureChoices,
	headless bool,
) (string, error) {
	if len(choices.Options) == 0 {
		return "", fmt.Errorf("feature %s has empty Options slice", featureID)
	}

	// In headless mode, auto-select the first option without blocking on stream.Recv().
	if headless {
		return choices.Options[0], nil
	}

	// Send the choice prompt as a sentinel-encoded MessageEvent so that web clients
	// can decode it as a modal dialog, and telnet clients can render it as text.
	payload := choicePromptPayload{
		FeatureID: featureID,
		Prompt:    choices.Prompt,
		Options:   choices.Options,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling choice prompt for %s: %w", featureID, err)
	}
	const choiceSentinel = "\x00choice\x00"
	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: choiceSentinel + string(data)},
		},
	}); sendErr != nil {
		return "", fmt.Errorf("sending choice prompt for %s: %w", featureID, sendErr)
	}

	// Loop up to maxChoiceAttempts, skipping any messages that are not Say or Move.
	// This prevents StatusRequest / MapRequest messages (sent by the web client on
	// connect) from being mis-interpreted as invalid choices.
	for attempt := 0; attempt < maxChoiceAttempts; attempt++ {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			return "", fmt.Errorf("receiving choice for %s: %w", featureID, recvErr)
		}

		// Only process Say and Move messages as potential choices.
		selText := ""
		switch {
		case msg.GetSay() != nil:
			selText = strings.TrimSpace(msg.GetSay().GetMessage())
		case msg.GetMove() != nil:
			selText = strings.TrimSpace(msg.GetMove().GetDirection())
		default:
			// Non-choice message type (e.g. StatusRequest, MapRequest); silently skip.
			continue
		}

		// If the text is not a valid integer at all, skip it. A bare direction like
		// "north" is not a choice attempt. Only an out-of-range integer (e.g. "99")
		// is treated as an invalid selection and breaks the loop.
		var n int
		_, scanErr := fmt.Sscanf(selText, "%d", &n)
		if scanErr != nil {
			// Not a number — silently skip and wait for the next message.
			continue
		}
		idx := -1
		if n >= 1 && n <= len(choices.Options) {
			idx = n - 1
		}

		if idx < 0 {
			// Numeric but out of range — log, notify player, and retry.
			if s.logger != nil {
				s.logger.Warn("invalid feature choice selection",
					zap.String("feature", featureID),
					zap.String("input", selText),
				)
			}
			if sendErr := stream.Send(&gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Invalid selection, please try again."},
				},
			}); sendErr != nil {
				if s.logger != nil {
					s.logger.Warn("sending invalid-selection retry feedback", zap.String("feature", featureID), zap.Error(sendErr))
				}
			}
			continue
		}

		chosen := choices.Options[idx]
		if sendErr := stream.Send(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: choices.Key + " set to: " + chosen},
			},
		}); sendErr != nil {
			// Non-fatal: value is already selected; log and continue.
			if s.logger != nil {
				s.logger.Warn("sending choice confirmation", zap.String("feature", featureID), zap.Error(sendErr))
			}
		}
		return chosen, nil
	}

	// All attempts exhausted or invalid selection received.
	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: "Invalid selection. You will be prompted again on next login."},
		},
	}); sendErr != nil {
		if s.logger != nil {
			s.logger.Warn("sending invalid-selection feedback", zap.String("feature", featureID), zap.Error(sendErr))
		}
		return "", sendErr
	}
	return "", nil
}

// promptTechSlotChoice is like promptFeatureChoice but includes prepared-slot metadata
// (slotCtx) in the JSON payload so that frontend clients can render slot tabs and level headers.
// slotCtx may be nil; when nil the payload is equivalent to promptFeatureChoice.
//
// Precondition: stream must be writable; choices must be non-nil with non-empty Options.
// Postcondition: Returns (selectedValue, nil) on valid input, ("", nil) on invalid input.
func (s *GameServiceServer) promptTechSlotChoice(
	stream gamev1.GameService_SessionServer,
	featureID string,
	choices *ruleset.FeatureChoices,
	slotCtx *TechSlotContext,
	headless bool,
) (string, error) {
	if len(choices.Options) == 0 {
		return "", fmt.Errorf("feature %s has empty Options slice", featureID)
	}
	if headless {
		return choices.Options[0], nil
	}

	var sc *techSlotContext
	if slotCtx != nil {
		sc = &techSlotContext{
			SlotNum:    slotCtx.SlotNum,
			TotalSlots: slotCtx.TotalSlots,
			SlotLevel:  slotCtx.SlotLevel,
		}
	}
	payload := choicePromptPayload{
		FeatureID:   featureID,
		Prompt:      choices.Prompt,
		Options:     choices.Options,
		SlotContext: sc,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling tech slot choice prompt for %s: %w", featureID, err)
	}
	const choiceSentinel = "\x00choice\x00"
	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: choiceSentinel + string(data)},
		},
	}); sendErr != nil {
		return "", fmt.Errorf("sending tech slot choice prompt for %s: %w", featureID, sendErr)
	}

	// Reuse the same receive loop logic as promptFeatureChoice.
	for attempt := 0; attempt < maxChoiceAttempts; attempt++ {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			return "", fmt.Errorf("receiving tech slot choice for %s: %w", featureID, recvErr)
		}
		selText := ""
		switch {
		case msg.GetSay() != nil:
			selText = strings.TrimSpace(msg.GetSay().GetMessage())
		case msg.GetMove() != nil:
			selText = strings.TrimSpace(msg.GetMove().GetDirection())
		default:
			continue
		}
		var n int
		_, scanErr := fmt.Sscanf(selText, "%d", &n)
		if scanErr != nil {
			continue
		}
		idx := -1
		if n >= 1 && n <= len(choices.Options) {
			idx = n - 1
		}
		if idx < 0 {
			// Numeric but out of range — log, notify player, and retry.
			if s.logger != nil {
				s.logger.Warn("invalid tech slot choice selection",
					zap.String("feature", featureID),
					zap.String("input", selText),
				)
			}
			if sendErr := stream.Send(&gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Invalid selection, please try again."},
				},
			}); sendErr != nil {
				if s.logger != nil {
					s.logger.Warn("sending invalid-selection retry feedback", zap.String("feature", featureID), zap.Error(sendErr))
				}
			}
			continue
		}
		chosen := choices.Options[idx]
		if sendErr := stream.Send(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: choices.Key + " set to: " + chosen},
			},
		}); sendErr != nil {
			if s.logger != nil {
				s.logger.Warn("sending tech slot choice confirmation", zap.String("feature", featureID), zap.Error(sendErr))
			}
		}
		return chosen, nil
	}

	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: "Invalid selection. You will be prompted again on next login."},
		},
	}); sendErr != nil {
		if s.logger != nil {
			s.logger.Warn("sending invalid-selection feedback", zap.String("feature", featureID), zap.Error(sendErr))
		}
		return "", sendErr
	}
	return "", nil
}

// recomputeBaseScores returns ability scores before any archetype/region boost choices are applied.
// Computes: base 10 + region modifiers + job key ability (+2).
//
// Precondition: dbChar must not be nil.
// Postcondition: Scores reflect only fixed sources (region modifiers and job key ability).
func recomputeBaseScores(dbChar *character.Character, regions map[string]*ruleset.Region, jobReg *ruleset.JobRegistry) character.AbilityScores {
	base := character.AbilityScores{
		Brutality: 10, Grit: 10, Quickness: 10,
		Reasoning: 10, Savvy: 10, Flair: 10,
	}
	if region, ok := regions[dbChar.Region]; ok {
		for ab, delta := range region.Modifiers {
			switch ab {
			case "brutality":
				base.Brutality += delta
			case "grit":
				base.Grit += delta
			case "quickness":
				base.Quickness += delta
			case "reasoning":
				base.Reasoning += delta
			case "savvy":
				base.Savvy += delta
			case "flair":
				base.Flair += delta
			}
		}
	}
	if jobReg != nil {
		if job, ok := jobReg.Job(dbChar.Class); ok {
			switch job.KeyAbility {
			case "brutality":
				base.Brutality += 2
			case "grit":
				base.Grit += 2
			case "quickness":
				base.Quickness += 2
			case "reasoning":
				base.Reasoning += 2
			case "savvy":
				base.Savvy += 2
			case "flair":
				base.Flair += 2
			}
		}
	}
	return base
}

// pushHPUpdate sends a CharacterInfo event to the player with the current
// and maximum HP from the session. Used to keep the frontend prompt in sync
// after any event that changes sess.CurrentHP or sess.MaxHP.
//
// Precondition: uid must be non-empty; sess must be non-nil.
// Postcondition: CharacterInfo event pushed to sess.Entity; push errors are logged.
func (s *GameServiceServer) pushHPUpdate(uid string, sess *session.PlayerSession) {
	ciEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CharacterInfo{
			CharacterInfo: &gamev1.CharacterInfo{
				CurrentHp: int32(sess.CurrentHP),
				MaxHp:     int32(sess.MaxHP),
			},
		},
	}
	data, marshalErr := proto.Marshal(ciEvt)
	if marshalErr != nil {
		s.logger.Warn("marshalling HP update CharacterInfo", zap.String("uid", uid), zap.Error(marshalErr))
		return
	}
	if pushErr := sess.Entity.Push(data); pushErr != nil {
		s.logger.Warn("pushing HP update CharacterInfo", zap.String("uid", uid), zap.Error(pushErr))
	}
}

// nextSkillRank returns (nextRank, gateLevel, err).
// gateLevel == 0 means the advancement is allowed at the current level.
// If currentRank is "legendary", returns an error.
//
// Precondition: currentRank is one of the five rank strings; level >= 1.
// Postcondition: nextRank is the rank after currentRank; gateLevel is the minimum level required (0 = no gate).
func nextSkillRank(currentRank string, level int) (nextRank string, gateLevel int, err error) {
	type step struct {
		next string
		gate int
	}
	progression := map[string]step{
		"untrained": {"trained", 0},
		"trained":   {"expert", 15},
		"expert":    {"master", 35},
		"master":    {"legendary", 75},
	}
	s, ok := progression[currentRank]
	if !ok {
		return "", 0, fmt.Errorf("skill is already at maximum rank (legendary)")
	}
	if s.gate > 0 && level < s.gate {
		return s.next, s.gate, nil
	}
	return s.next, 0, nil
}

// handleTrainSkill advances a skill proficiency rank for the player.
//
// Precondition: uid must identify an active session; skillID must be a valid skill ID.
// Postcondition: if no pending skill increases, returns error message event;
// if level gate not met, returns error message event;
// otherwise upgrades skill rank, decrements pending count, updates session, returns confirmation.
func (s *GameServiceServer) handleTrainSkill(uid, skillID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.PendingSkillIncreases <= 0 {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: "You have no pending skill increases."},
			},
		}, nil
	}

	currentRank := sess.Skills[skillID]
	if currentRank == "" {
		currentRank = "untrained"
	}

	nextRank, gateLevel, rankErr := nextSkillRank(currentRank, sess.Level)
	if rankErr != nil {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: rankErr.Error()},
			},
		}, nil
	}
	if gateLevel > 0 {
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: fmt.Sprintf("You must be level %d to advance %s to %s.", gateLevel, skillID, nextRank),
				},
			},
		}, nil
	}

	ctx := context.Background()
	if s.characterSkillsRepo != nil {
		if err := s.characterSkillsRepo.UpgradeSkill(ctx, sess.CharacterID, skillID, nextRank); err != nil {
			s.logger.Warn("handleTrainSkill: UpgradeSkill failed", zap.Error(err))
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Failed to upgrade skill. Please try again."},
				},
			}, nil
		}
	}
	if s.progressRepo != nil {
		if err := s.progressRepo.ConsumePendingSkillIncrease(ctx, sess.CharacterID); err != nil {
			s.logger.Warn("handleTrainSkill: ConsumePendingSkillIncrease failed", zap.Error(err))
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: "Failed to consume skill increase. Please try again."},
				},
			}, nil
		}
	}

	// Both persistence calls succeeded — mutate session.
	if sess.Skills == nil {
		sess.Skills = make(map[string]string)
	}
	sess.Skills[skillID] = nextRank
	sess.PendingSkillIncreases--

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You advanced %s from %s to %s. Pending skill increases remaining: %d.",
					skillID, currentRank, nextRank, sess.PendingSkillIncreases),
			},
		},
	}, nil
}

// handleRaiseShield applies the shield_raised condition (+2 AC for one round).
// Requires a shield equipped in the off-hand slot.
//
// Precondition: uid must identify a valid player session.
// Postcondition: Applies shield_raised condition; in combat, deducts 1 AP and updates Combatant ACMod.
func (s *GameServiceServer) handleRaiseShield(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Verify a shield is equipped in the off-hand slot.
	if sess.LoadoutSet == nil {
		return errorEvent("You have no equipment loaded."), nil
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset == nil || preset.OffHand == nil || !preset.OffHand.Def.IsShield() {
		return errorEvent("You must have a shield equipped in the off-hand slot to raise a shield."), nil
	}

	// In combat: spend 1 AP and update the combatant's ACMod.
	if sess.Status == statusInCombat {
		if s.combatH == nil {
			return errorEvent("Combat handler unavailable."), nil
		}
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil
		}
		if err := s.combatH.ApplyCombatantACMod(uid, uid, +2); err != nil {
			s.logger.Warn("handleRaiseShield: ApplyCombatantACMod failed",
				zap.String("uid", uid), zap.Error(err))
			return errorEvent(fmt.Sprintf("Failed to raise shield: %v", err)), nil
		}
	}

	// Apply the shield_raised condition to the player session.
	if s.condRegistry != nil {
		def, ok := s.condRegistry.Get("shield_raised")
		if !ok {
			s.logger.Warn("handleRaiseShield: shield_raised condition not found in registry",
				zap.String("uid", uid))
		} else {
			if sess.Conditions == nil {
				sess.Conditions = condition.NewActiveSet()
			}
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleRaiseShield: Apply shield_raised failed",
					zap.String("uid", uid), zap.Error(err))
			}
		}
	}

	return messageEvent("You raise your shield. (+2 AC until start of next turn)"), nil
}

// coverTierRank returns a numeric rank for cover tier comparison.
//
// Postcondition: Returns 3 for "greater", 2 for "standard", 1 for "lesser", 0 otherwise.
func coverTierRank(tier string) int {
	switch tier {
	case combat.CoverTierGreater:
		return 3
	case combat.CoverTierStandard:
		return 2
	case combat.CoverTierLesser:
		return 1
	default:
		return 0
	}
}

// clearPlayerCover removes any active cover condition and resets combatant cover fields.
// Precondition: uid must identify a valid player session.
// Postcondition: All cover conditions removed; combatant cover fields cleared if in combat.
func (s *GameServiceServer) clearPlayerCover(uid string, sess *session.PlayerSession) {
	if sess.Conditions != nil {
		for _, coverID := range []string{"greater_cover", "standard_cover", "lesser_cover"} {
			sess.Conditions.Remove(uid, coverID)
		}
	}
	if sess.Status == statusInCombat && s.combatH != nil {
		s.combatH.ClearCombatantCover(sess.RoomID, uid)
	}
}

// handleTakeCover applies the best available cover condition from room equipment.
//
// Precondition: uid must identify a valid player session.
// Postcondition: Applies the appropriate cover condition; in combat, deducts 1 AP
// and sets CoverEquipmentID/CoverTier on the Combatant.
func (s *GameServiceServer) handleTakeCover(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Find best cover available in the room.
	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return errorEvent("Cannot determine room layout."), nil
	}
	bestTier := ""
	bestEquipID := ""
	bestHP := 0
	bestDestructible := false
	for _, eq := range room.Equipment {
		if eq.CoverTier == "" {
			continue
		}
		if coverTierRank(eq.CoverTier) > coverTierRank(bestTier) {
			bestTier = eq.CoverTier
			bestEquipID = eq.ItemID
			bestHP = eq.CoverHP
			bestDestructible = eq.CoverDestructible
		}
	}
	if bestTier == "" {
		return messageEvent("There is no cover available in this area."), nil
	}

	// Check if already in equal or better cover.
	currentTier := ""
	if sess.Conditions != nil {
		for _, coverID := range []string{"greater_cover", "standard_cover", "lesser_cover"} {
			if sess.Conditions.Has(coverID) {
				currentTier = strings.TrimSuffix(coverID, "_cover")
				break
			}
		}
	}
	if coverTierRank(currentTier) >= coverTierRank(bestTier) {
		return messageEvent(fmt.Sprintf("You are already in %s cover.", currentTier)), nil
	}

	// In combat: spend 1 AP and set combatant cover fields.
	if sess.Status == statusInCombat {
		if s.combatH == nil {
			return errorEvent("Combat handler unavailable."), nil
		}
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil
		}
		if err := s.combatH.SetCombatantCover(sess.RoomID, uid, bestEquipID, bestTier); err != nil {
			s.logger.Warn("handleTakeCover: SetCombatantCover failed",
				zap.String("uid", uid), zap.Error(err))
			return errorEvent(fmt.Sprintf("Failed to take cover: %v", err)), nil
		}
		// Init cover HP state if destructible and not yet tracked.
		if bestDestructible && bestHP > 0 && s.combatH.GetCoverHP(sess.RoomID, bestEquipID) < 0 {
			s.combatH.InitCoverState(sess.RoomID, bestEquipID, bestHP)
		}
	}

	// Remove any existing cover condition, then apply the new one.
	if sess.Conditions == nil {
		sess.Conditions = condition.NewActiveSet()
	}
	for _, old := range []string{"greater_cover", "standard_cover", "lesser_cover", "in_cover"} {
		sess.Conditions.Remove(uid, old)
	}
	condID := bestTier + "_cover"
	if s.condRegistry == nil {
		return errorEvent("Condition registry unavailable."), nil
	}
	def, ok := s.condRegistry.Get(condID)
	if !ok {
		s.logger.Warn("handleTakeCover: condition not found", zap.String("condID", condID))
		return errorEvent(fmt.Sprintf("Cover condition %q not found.", condID)), nil
	}
	if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
		s.logger.Warn("handleTakeCover: Apply failed", zap.String("uid", uid), zap.Error(err))
	}

	return messageEvent(fmt.Sprintf("You take %s cover. (+%d AC, +%d Ghosting)",
		bestTier, def.ACPenalty, def.StealthBonus)), nil
}

// handleUncover drops the player's current cover condition.
//
// Precondition: uid must identify a valid player session.
// Postcondition: All cover conditions removed; combatant cover cleared if in combat.
func (s *GameServiceServer) handleUncover(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	hasCover := false
	if sess.Conditions != nil {
		for _, coverID := range []string{"greater_cover", "standard_cover", "lesser_cover"} {
			if sess.Conditions.Has(coverID) {
				hasCover = true
				break
			}
		}
	}
	if !hasCover {
		return messageEvent("You are not taking cover."), nil
	}
	s.clearPlayerCover(uid, sess)
	return messageEvent("You leave cover."), nil
}

// handleFirstAid performs a patch_job skill check (DC 15).
// On success, heals 2d8+4 HP (self). Costs 2 AP in combat.
//
// Precondition: uid must identify a valid player session.
// Postcondition: On skill check success, heals player HP and persists via charSaver.
func (s *GameServiceServer) handleFirstAid(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// In combat: spend 2 AP.
	if sess.Status == statusInCombat {
		if s.combatH == nil {
			return errorEvent("Combat handler unavailable."), nil
		}
		if err := s.combatH.SpendAP(uid, 2); err != nil {
			return errorEvent(err.Error()), nil
		}
	}

	// Skill check: 1d20 + patch_job rank bonus vs DC 15.
	if s.dice == nil {
		return nil, fmt.Errorf("handleFirstAid: dice roller not configured")
	}
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleFirstAid: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["patch_job"])
	total := roll + bonus
	dc := 15
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "first_aid"

	if total < dc {
		msg := fmt.Sprintf("First aid check: rolled %d+%d=%d vs DC %d — failure. You fail to apply treatment.",
			roll, bonus, total, dc)
		return messageEvent(msg), nil
	}

	// Success: heal 2d8+4.
	healResult, err := s.dice.RollExpr("2d8+4")
	if err != nil {
		return nil, fmt.Errorf("handleFirstAid: rolling heal: %w", err)
	}
	healed := healResult.Total()
	newHP := sess.CurrentHP + healed
	if newHP > sess.MaxHP {
		newHP = sess.MaxHP
	}
	sess.CurrentHP = newHP

	ctx := context.Background()
	if s.charSaver != nil {
		if saveErr := s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, newHP); saveErr != nil {
			s.logger.Warn("handleFirstAid: saving HP", zap.String("uid", uid), zap.Error(saveErr))
		}
	}

	// Push HP update event.
	hpEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HpUpdate{
			HpUpdate: &gamev1.HpUpdateEvent{
				CurrentHp:      int32(newHP),
				MaxHp:          int32(sess.MaxHP),
				FocusPoints:    int32(sess.FocusPoints),
				MaxFocusPoints: int32(sess.MaxFocusPoints),
			},
		},
	}
	if data, err := proto.Marshal(hpEvt); err == nil {
		_ = sess.Entity.Push(data)
	}

	msg := fmt.Sprintf("First aid check: rolled %d+%d=%d vs DC %d — success! You recover %d HP. (%d/%d)",
		roll, bonus, total, dc, healed, newHP, sess.MaxHP)
	return messageEvent(msg), nil
}

// handleFeint performs a grift skill check against the target NPC's Perception DC.
// On success, applies flat_footed (-2 AC) to the target combatant for this round.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target's ACMod is decremented by 2.
func (s *GameServiceServer) handleFeint(uid string, req *gamev1.FeintRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Feint is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: feint <target>"), nil
	}

	// Find target NPC in room to get Perception DC before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + grift bonus vs target Perception.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleFeint: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["grift"])
	total := roll + bonus
	dc := inst.Awareness
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "feint"

	detail := fmt.Sprintf("Feint (grift DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your feint is transparent."), nil
	}

	// Success: apply flat_footed to NPC combatant (-2 AC).
	if err := s.combatH.ApplyCombatantACMod(uid, inst.ID, -2); err != nil {
		s.logger.Warn("handleFeint: ApplyCombatantACMod failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is caught off-guard (-2 AC this round).", inst.Name())), nil
}

// handleDemoralize performs a smooth_talk skill check against the target NPC's Cool DC (10 + level + AbilityMod(Savvy) + CoolRank bonus).
// On success, applies -1 AC and -1 attack to the target combatant for the encounter.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target's ACMod and AttackMod are each decremented by 1.
func (s *GameServiceServer) handleDemoralize(uid string, req *gamev1.DemoralizeRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Demoralize is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: demoralize <target>"), nil
	}

	// Find target NPC in room to get Level DC before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + smooth_talk bonus vs target Cool DC. Cool DC = 10 + level + AbilityMod(Savvy) + proficiency rank bonus.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDemoralize: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["smooth_talk"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Savvy) + skillRankBonus(inst.CoolRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "demoralize"

	detail := fmt.Sprintf("Demoralize (smooth_talk Cool DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your words fall flat."), nil
	}

	// Success: apply -1 AC and -1 attack to NPC combatant.
	if err := s.combatH.ApplyCombatantACMod(uid, inst.ID, -1); err != nil {
		s.logger.Warn("handleDemoralize: ApplyCombatantACMod failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}
	if err := s.combatH.ApplyCombatantAttackMod(uid, inst.ID, -1); err != nil {
		s.logger.Warn("handleDemoralize: ApplyCombatantAttackMod failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is demoralized (-1 AC, -1 attack).", inst.Name())), nil
}

// handleGrapple performs a muscle skill check against the target NPC's Toughness DC (10 + level + AbilityMod(Brutality) + ToughnessRank bonus).
// On success, applies the grabbed condition (-2 AC) to the target combatant for the encounter.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, the grabbed condition is applied to the target combatant.
func (s *GameServiceServer) handleGrapple(uid string, req *gamev1.GrappleRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Grapple is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: grapple <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + muscle bonus vs target Toughness DC. Toughness DC = 10 + level + AbilityMod(Brutality) + proficiency rank bonus.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleGrapple: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Brutality) + skillRankBonus(inst.ToughnessRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "grapple"

	detail := fmt.Sprintf("Grapple (muscle Toughness DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your grapple attempt fails."), nil
	}

	// Success: apply grabbed condition to NPC combatant.
	if err := s.combatH.ApplyCombatCondition(uid, inst.ID, "grabbed"); err != nil {
		s.logger.Warn("handleGrapple: ApplyCombatCondition failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is grabbed (flat-footed, -2 AC).", inst.Name())), nil
}

// handleTrip performs a muscle skill check against the target NPC's Hustle DC (10 + level + AbilityMod(Quickness) + HustleRank bonus).
// On success, applies the prone condition (-2 attack) to the target combatant for the encounter.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, the prone condition is applied to the target combatant.
func (s *GameServiceServer) handleTrip(uid string, req *gamev1.TripRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Trip is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: trip <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + muscle bonus vs target Hustle DC. Hustle DC = 10 + level + AbilityMod(Quickness) + proficiency rank bonus.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleTrip: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "trip"

	detail := fmt.Sprintf("Trip (muscle Hustle DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your trip attempt fails."), nil
	}

	// Success: apply prone condition to NPC combatant.
	if err := s.combatH.ApplyCombatCondition(uid, inst.ID, "prone"); err != nil {
		s.logger.Warn("handleTrip: ApplyCombatCondition failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is knocked prone (-2 attack rolls).", inst.Name())), nil
}

// handleDisarm performs a muscle skill check against the target NPC's Hustle DC (10 + level + AbilityMod(Quickness) + HustleRank bonus).
// On success, removes the NPC's equipped weapon and drops it to the room floor.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, NPC's WeaponID and WeaponName are cleared; weapon item dropped to floor.
func (s *GameServiceServer) handleDisarm(uid string, req *gamev1.DisarmRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Disarm is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: disarm <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after target confirmed.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + muscle bonus vs target Hustle DC.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDisarm: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "disarm"

	detail := fmt.Sprintf("Disarm (muscle Hustle DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your disarm attempt fails."), nil
	}

	// Success: clear NPC weapon via combat handler.
	weaponItemID, disarmErr := s.combatH.DisarmNPC(uid, inst.ID)
	if disarmErr != nil {
		s.logger.Warn("handleDisarm: DisarmNPC failed",
			zap.String("npc_id", inst.ID), zap.Error(disarmErr))
	}

	// Drop the weapon to the room floor if the NPC had one.
	weaponName := "weapon"
	if weaponItemID != "" && s.floorMgr != nil {
		if s.invRegistry != nil {
			if wDef := s.invRegistry.Weapon(weaponItemID); wDef != nil {
				weaponName = wDef.Name
			}
		}
		dropped := inventory.ItemInstance{
			InstanceID: fmt.Sprintf("disarmed-%s-%d", weaponItemID, time.Now().UnixNano()),
			ItemDefID:  weaponItemID,
			Quantity:   1,
		}
		s.floorMgr.Drop(sess.RoomID, dropped)
	}

	if weaponItemID == "" {
		return messageEvent(detail + fmt.Sprintf(" — success! %s is disarmed, but had no weapon equipped.", inst.Name())), nil
	}
	return messageEvent(detail + fmt.Sprintf(" — success! %s is disarmed. The %s clatters to the floor.", inst.Name(), weaponName)), nil
}

// handleShove performs a muscle skill check against the target NPC's Toughness DC (10 + level + AbilityMod(Brutality) + ToughnessRank bonus).
// On success, the NPC is pushed 5ft; on critical success (beat DC by 10+), pushed 10ft.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, NPC combatant's GridX is updated by 1 cell (5ft) or 2 cells (10ft on crit).
func (s *GameServiceServer) handleShove(uid string, req *gamev1.ShoveRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Shove is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: shove <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + muscle bonus vs target Toughness DC. Toughness DC = 10 + level + AbilityMod(Brutality) + proficiency rank bonus.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleShove: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Brutality) + skillRankBonus(inst.ToughnessRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "shove"

	detail := fmt.Sprintf("Shove (muscle Toughness DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + fmt.Sprintf(" — fail. You fail to shove %s.", inst.Name())), nil
	}

	pushFt := 5
	if total >= dc+10 {
		pushFt = 10
	}

	if err := s.combatH.ShoveNPC(uid, inst.ID, pushFt); err != nil {
		s.logger.Warn("handleShove: ShoveNPC failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	} else {
		s.combatH.FireCombatantMoved(sess.RoomID, inst.ID)
		s.combatH.BroadcastAllPositions(sess.RoomID)
	}

	if pushFt == 10 {
		return messageEvent(detail + fmt.Sprintf(" — critical success! %s is shoved back 10 ft!", inst.Name())), nil
	}
	return messageEvent(detail + fmt.Sprintf(" — success! %s is pushed back 5 ft.", inst.Name())), nil
}

// handleStride moves the player their full speed in the requested direction.
// Combat only; costs 1 AP. Defaults to "toward" the nearest enemy when no direction is given.
// The player moves (effectiveSpeed / 5) grid cells, one cell at a time; Reactive Strikes fire
// for each cell that exits a threatened square.
//
// Precondition: uid must be in active combat.
// Postcondition: Player combatant's GridX/GridY updated; CheckReactiveStrikes fires per threatened-square
// exit; message event returned.
func (s *GameServiceServer) handleStride(uid string, req *gamev1.StrideRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status != statusInCombat {
		return errorEvent("Stride is only available in combat."), nil
	}
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendMovementAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		return errorEvent("No active combat found."), nil
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		return errorEvent("You are not a combatant in this fight."), nil
	}

	dir := req.GetDirection()
	if dir == "" {
		dir = "toward"
	}

	validDirs := map[string]bool{
		"n": true, "s": true, "e": true, "w": true,
		"ne": true, "nw": true, "se": true, "sw": true,
		"toward": true, "away": true,
	}
	if !validDirs[dir] {
		return errorEvent(fmt.Sprintf("Unknown direction %q. Use a compass direction (n/s/e/w/ne/nw/se/sw) or 'toward'/'away'.", dir)), nil
	}

	// Find nearest enemy for toward/away directions.
	var opponent *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer && !c.IsDead() {
			if opponent == nil || combat.CombatRange(*combatant, *c) < combat.CombatRange(*combatant, *opponent) {
				opponent = c
			}
		}
	}

	width, height := cbt.GridWidth, cbt.GridHeight
	if width == 0 {
		width = 20
	}
	if height == 0 {
		height = 20
	}

	// Compute effective speed: 25 ft base minus armor speed penalty (min 5 ft).
	speedFt := s.playerEffectiveSpeedFt(sess)
	steps := speedFt / 5
	if steps < 1 {
		steps = 1
	}

	// Direction delta is fixed for the whole stride (re-evaluated against initial position only).
	dx, dy := combat.CompassDelta(dir, combatant, opponent)

	rsUpdater := func(id string, hp int) {
		if target, ok := s.sessions.GetPlayer(id); ok {
			target.CurrentHP = hp
		}
	}

	var allRSEvents []combat.RoundEvent
	// Move one cell at a time so Reactive Strikes fire at each threatened-square exit.
	for step := 0; step < steps; step++ {
		// REQ-STRIDE-STOP: For "toward" strides, stop when already adjacent (≤ 5 ft).
		if dir == "toward" && opponent != nil && combat.CombatRange(*combatant, *opponent) <= 5 {
			break
		}
		oldX, oldY := combatant.GridX, combatant.GridY
		newX := oldX + dx
		newY := oldY + dy
		if newX < 0 {
			newX = 0
		} else if newX >= width {
			newX = width - 1
		}
		if newY < 0 {
			newY = 0
		} else if newY >= height {
			newY = height - 1
		}
		// If clamped to same cell, no more movement possible in this direction.
		if newX == oldX && newY == oldY {
			break
		}
		// REQ-STRIDE-NOOVERLAP: Do not move onto a cell occupied by another living combatant.
		// GH #227: cover objects block movement until destroyed.
		if combat.CellBlocked(cbt, uid, newX, newY) {
			break
		}
		combatant.GridX = newX
		combatant.GridY = newY
		rsEvents := combat.CheckReactiveStrikes(cbt, uid, oldX, oldY, globalRandSrc{}, rsUpdater)
		allRSEvents = append(allRSEvents, rsEvents...)
		// Stop striding if the player was killed by a Reactive Strike.
		if combatant.IsDead() {
			break
		}
	}

	s.clearPlayerCover(uid, sess)

	msg := fmt.Sprintf("You stride %s (%d ft).", dir, speedFt)
	for _, ev := range allRSEvents {
		msg += "\n" + ev.Narrative
	}
	if room, ok := s.world.GetRoom(sess.RoomID); ok {
		s.checkPressurePlateTraps(uid, sess, room)
	}
	s.combatH.FireCombatantMoved(sess.RoomID, uid)
	s.combatH.BroadcastAllPositions(sess.RoomID)
	return messageEvent(msg), nil
}

// handleMoveTo moves the player to a specific grid cell, deducting 1 or 2 movement AP based on distance.
// Cells within one stride (speedFt/5) cost 1 AP; cells within two strides cost 2 AP.
// Reactive Strikes fire per threatened-square exit, identical to handleStride.
//
// Precondition: uid must be in active combat; target cell must be within 2*strideCells of player.
// Postcondition: Player combatant's GridX/GridY updated; appropriate AP deducted; message event returned.
func (s *GameServiceServer) handleMoveTo(uid string, req *gamev1.MoveToRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status != statusInCombat {
		return errorEvent("Move-to is only available in combat."), nil
	}
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}

	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		return errorEvent("No active combat found."), nil
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		return errorEvent("You are not a combatant in this fight."), nil
	}

	targetX := int(req.GetTargetX())
	targetY := int(req.GetTargetY())

	// Validate target is within grid bounds.
	width, height := cbt.GridWidth, cbt.GridHeight
	if width == 0 {
		width = 20
	}
	if height == 0 {
		height = 20
	}
	if targetX < 0 || targetX >= width || targetY < 0 || targetY >= height {
		return errorEvent("Target cell is outside the combat grid."), nil
	}

	// Compute stride distance (cells per movement action).
	speedFt := s.playerEffectiveSpeedFt(sess)
	strideCells := speedFt / 5
	if strideCells < 1 {
		strideCells = 1
	}

	// Chebyshev distance from current position to target.
	dx := targetX - combatant.GridX
	dy := targetY - combatant.GridY
	absDx := dx
	if absDx < 0 {
		absDx = -absDx
	}
	absDy := dy
	if absDy < 0 {
		absDy = -absDy
	}
	dist := absDx
	if absDy > dist {
		dist = absDy
	}

	if dist == 0 {
		return messageEvent("You are already at that location."), nil
	}

	// Determine number of movement actions required.
	var moveCost int
	switch {
	case dist <= strideCells:
		moveCost = 1
	case dist <= 2*strideCells:
		moveCost = 2
	default:
		return errorEvent(fmt.Sprintf("Target is too far to reach (distance %d, max %d).", dist, 2*strideCells)), nil
	}

	// Check movement AP sufficiency upfront before any deduction.
	if err := s.combatH.CheckMovementAPAvailable(uid, moveCost); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Deduct movement AP atomically now that sufficiency is confirmed.
	for i := 0; i < moveCost; i++ {
		if err := s.combatH.SpendMovementAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil // should never happen after check
		}
	}

	// Move the player cell by cell toward the target using diagonal-first path.
	// Re-read combatant after AP deduction (SpendMovementAP may lock/unlock).
	combatant = cbt.GetCombatant(uid)
	if combatant == nil {
		return errorEvent("Combatant lost during move."), nil
	}

	rsUpdater := func(id string, hp int) {
		if target, ok := s.sessions.GetPlayer(id); ok {
			target.CurrentHP = hp
		}
	}

	var allRSEvents []combat.RoundEvent
	maxCells := moveCost * strideCells
	for step := 0; step < maxCells; step++ {
		if combatant.GridX == targetX && combatant.GridY == targetY {
			break
		}
		// Diagonal-first step toward target.
		stepDx := 0
		stepDy := 0
		if combatant.GridX < targetX {
			stepDx = 1
		} else if combatant.GridX > targetX {
			stepDx = -1
		}
		if combatant.GridY < targetY {
			stepDy = 1
		} else if combatant.GridY > targetY {
			stepDy = -1
		}

		oldX, oldY := combatant.GridX, combatant.GridY
		newX := oldX + stepDx
		newY := oldY + stepDy

		// Clamp to grid bounds.
		if newX < 0 {
			newX = 0
		}
		if newX >= width {
			newX = width - 1
		}
		if newY < 0 {
			newY = 0
		}
		if newY >= height {
			newY = height - 1
		}

		combatant.GridX = newX
		combatant.GridY = newY

		// Fire Reactive Strikes when exiting a threatened square.
		rsEvents := combat.CheckReactiveStrikes(cbt, uid, oldX, oldY, globalRandSrc{}, rsUpdater)
		allRSEvents = append(allRSEvents, rsEvents...)

		// Stop moving if the player was killed by a Reactive Strike.
		if combatant.IsDead() {
			break
		}
	}

	s.combatH.FireCombatantMoved(sess.RoomID, uid)
	s.combatH.BroadcastAllPositions(sess.RoomID)

	msg := fmt.Sprintf("You move to (%d, %d) (%d ft).", targetX, targetY, dist*5)
	for _, ev := range allRSEvents {
		msg += "\n" + ev.Narrative
	}

	return messageEvent(msg), nil
}

// handleBrutalCharge executes the Brutal Charge feat: advance twice and make a melee strike
// as a single 2-AP sequence. The two strides are free (no additional AP); the final attack
// costs 1 AP via the normal Attack path, giving a total cost of 2 AP.
//
// Precondition: uid must be in active combat; combatH must be non-nil.
// Postcondition: Player moves up to 2×speed toward the nearest enemy and queues a melee attack.
func (s *GameServiceServer) handleBrutalCharge(uid, targetID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status != statusInCombat {
		return messageEvent("Brutal Charge requires active combat."), nil
	}
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	// Spend 1 AP for the movement discount (2 free strides instead of 2 AP).
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		return errorEvent("No active combat found."), nil
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		return errorEvent("You are not a combatant in this fight."), nil
	}
	// Find nearest enemy.
	var opponent *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer && !c.IsDead() {
			if opponent == nil || combat.CombatRange(*combatant, *c) < combat.CombatRange(*combatant, *opponent) {
				opponent = c
			}
		}
	}
	if opponent == nil {
		return messageEvent("No targets to charge toward."), nil
	}

	width, height := cbt.GridWidth, cbt.GridHeight
	if width == 0 {
		width = 20
	}
	if height == 0 {
		height = 20
	}
	speedFt := s.playerEffectiveSpeedFt(sess)
	stepsPerStride := speedFt / 5
	if stepsPerStride < 1 {
		stepsPerStride = 1
	}

	rsUpdater := func(id string, hp int) {
		if target, ok := s.sessions.GetPlayer(id); ok {
			target.CurrentHP = hp
		}
	}

	var allRSEvents []combat.RoundEvent
	// Execute two strides at 0 additional AP cost.
	for stride := 0; stride < 2; stride++ {
		if combatant.IsDead() {
			break
		}
		dx, dy := combat.CompassDelta("toward", combatant, opponent)
		for step := 0; step < stepsPerStride; step++ {
			if combat.CombatRange(*combatant, *opponent) <= 5 {
				break
			}
			oldX, oldY := combatant.GridX, combatant.GridY
			newX := oldX + dx
			newY := oldY + dy
			if newX < 0 {
				newX = 0
			} else if newX >= width {
				newX = width - 1
			}
			if newY < 0 {
				newY = 0
			} else if newY >= height {
				newY = height - 1
			}
			if newX == oldX && newY == oldY {
				break
			}
			// GH #227: cover objects block movement until destroyed.
			if combat.CellBlocked(cbt, uid, newX, newY) {
				break
			}
			combatant.GridX = newX
			combatant.GridY = newY
			rsEvents := combat.CheckReactiveStrikes(cbt, uid, oldX, oldY, globalRandSrc{}, rsUpdater)
			allRSEvents = append(allRSEvents, rsEvents...)
			if combatant.IsDead() {
				break
			}
		}
	}

	s.clearPlayerCover(uid, sess)
	s.combatH.BroadcastAllPositions(sess.RoomID)

	if combatant.IsDead() {
		msg := "You charge forward but are cut down by a Reactive Strike!"
		for _, ev := range allRSEvents {
			msg += "\n" + ev.Narrative
		}
		return messageEvent(msg), nil
	}

	// Attack the opponent (costs 1 AP via QueueAction).
	attackEvents, err := s.combatH.Attack(uid, opponent.Name)
	if err != nil {
		return messageEvent(fmt.Sprintf("You close the distance before they can react. (Attack failed: %v)", err)), nil
	}
	if len(attackEvents) == 0 {
		return messageEvent("You close the distance before they can react."), nil
	}
	moveSuffix := fmt.Sprintf(" (charged %d ft)", 2*speedFt)
	for _, ev := range allRSEvents {
		moveSuffix += "\n" + ev.Narrative
	}
	// Append move narrative to the first attack event.
	attackEvents[0].Narrative = "You close the distance before they can react." + moveSuffix + "\n" + attackEvents[0].GetNarrative()
	// Broadcast additional attack events.
	for _, evt := range attackEvents[1:] {
		s.broadcastCombatEvent(sess.RoomID, uid, evt)
	}
	s.pushRoomViewToAllInRoom(sess.RoomID)
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: attackEvents[0]},
	}, nil
}

// playerEffectiveSpeedFt returns the player's effective movement speed in feet.
// Base speed is 25 ft (PF2e default); armor speed penalty is subtracted.
// Result is clamped to a minimum of 5 ft.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns a positive multiple of 5, at least 5.
func (s *GameServiceServer) playerEffectiveSpeedFt(sess *session.PlayerSession) int {
	const baseSpeedFt = 25
	speedPenalty := 0
	if sess.Equipment != nil && s.invRegistry != nil {
		dexMod := 0
		def := sess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(
			s.invRegistry, dexMod, sess.Proficiencies, sess.Level, sess.SetBonusSummary,
		)
		speedPenalty = def.SpeedPenalty
	}
	result := baseSpeedFt - speedPenalty
	if result < 5 {
		result = 5
	}
	return result
}

// handleStep moves the player one grid cell in the requested direction without triggering Reactive Strikes.
// Combat only; costs 1 AP. Step explicitly does NOT trigger Reactive Strikes.
//
// Precondition: uid must be in active combat.
// Postcondition: Player combatant's GridX/GridY updated by one cell; message event returned.
// No CheckReactiveStrikes is called — Step is safe from reactive strikes by rule.
func (s *GameServiceServer) handleStep(uid string, req *gamev1.StepRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status != statusInCombat {
		return errorEvent("Step is only available in combat."), nil
	}
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendMovementAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		return errorEvent("No active combat found."), nil
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		return errorEvent("You are not a combatant in this fight."), nil
	}

	dir := req.GetDirection()
	if dir == "" {
		dir = "toward"
	}
	validDirs := map[string]bool{
		"n": true, "s": true, "e": true, "w": true,
		"ne": true, "nw": true, "se": true, "sw": true,
		"toward": true, "away": true,
	}
	if !validDirs[dir] {
		return errorEvent(fmt.Sprintf("Unknown direction %q. Use 'toward', 'away', or a compass direction.", dir)), nil
	}

	// Find nearest enemy for toward/away.
	var opponent *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer && !c.IsDead() {
			if opponent == nil || combat.CombatRange(*combatant, *c) < combat.CombatRange(*combatant, *opponent) {
				opponent = c
			}
		}
	}

	width, height := cbt.GridWidth, cbt.GridHeight
	if width == 0 {
		width = 20
	}
	if height == 0 {
		height = 20
	}
	dx, dy := combat.CompassDelta(dir, combatant, opponent)
	// REQ-STEP-STOP: For "toward" steps, do not move if already adjacent (≤ 5 ft).
	if dir == "toward" && opponent != nil && combat.CombatRange(*combatant, *opponent) <= 5 {
		dx, dy = 0, 0
	}
	newX := combatant.GridX + dx
	newY := combatant.GridY + dy
	if newX < 0 {
		newX = 0
	} else if newX >= width {
		newX = width - 1
	}
	if newY < 0 {
		newY = 0
	} else if newY >= height {
		newY = height - 1
	}
	// REQ-STEP-NOOVERLAP: Do not move onto a cell occupied by another living combatant.
	// GH #227: cover objects block movement until destroyed.
	if combat.CellBlocked(cbt, uid, newX, newY) {
		newX = combatant.GridX
		newY = combatant.GridY
	}
	combatant.GridX = newX
	combatant.GridY = newY

	s.clearPlayerCover(uid, sess)

	dist := 0
	if opponent != nil {
		dist = combat.CombatRange(*combatant, *opponent)
	}
	msg := fmt.Sprintf("You step %s. Distance to target: %d ft.", dir, dist)
	if room, ok := s.world.GetRoom(sess.RoomID); ok {
		s.checkPressurePlateTraps(uid, sess, room)
	}
	s.combatH.FireCombatantMoved(sess.RoomID, uid)
	s.combatH.BroadcastAllPositions(sess.RoomID)
	return messageEvent(msg), nil
}

// handleTumble attempts to move the player 5 ft through the target NPC's space using Acrobatics.
// On success: player moves 5 ft toward the NPC (no reactive strike from the tumbled-through NPC).
// On failure: player is blocked and the target NPC makes a Reactive Strike against the player.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, player combatant's GridX/GridY updated by CompassDelta (1 cell toward target).
// On failure, NPC makes a reactive strike that may reduce player HP; no position change.
func (s *GameServiceServer) handleTumble(uid string, req *gamev1.TumbleRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Tumble is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: tumble <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	// Spend 1 AP only after the target is confirmed to exist.
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + acrobatics bonus vs target Hustle DC.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleTumble: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["acrobatics"])
	total := roll + bonus
	dc := 10 + inst.Level + combat.AbilityMod(inst.Quickness) + skillRankBonus(inst.HustleRank)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "tumble"

	detail := fmt.Sprintf("Tumble (acrobatics Hustle DC %d): rolled %d+%d=%d", dc, roll, bonus, total)

	if total >= dc {
		// Success: move player 5 ft toward NPC.
		cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
		if !ok {
			return errorEvent("No active combat found."), nil
		}
		combatant := cbt.GetCombatant(uid)
		if combatant == nil {
			return errorEvent("You are not a combatant in this fight."), nil
		}
		// Move player one grid cell toward the NPC by finding it and using CompassDelta.
		var npcCbt *combat.Combatant
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC && c.ID == inst.ID {
				npcCbt = c
				break
			}
		}
		dx, dy := combat.CompassDelta("toward", combatant, npcCbt)
		width, height := cbt.GridWidth, cbt.GridHeight
		if width == 0 {
			width = 20
		}
		if height == 0 {
			height = 20
		}
		newX := combatant.GridX + dx
		newY := combatant.GridY + dy
		if newX < 0 {
			newX = 0
		} else if newX >= width {
			newX = width - 1
		}
		if newY < 0 {
			newY = 0
		} else if newY >= height {
			newY = height - 1
		}
		// REQ-TUMBLE-NOOVERLAP: Do not land on a cell occupied by another living combatant.
		// GH #227: cover objects block movement until destroyed.
		if combat.CellBlocked(cbt, uid, newX, newY) {
			newX = combatant.GridX
			newY = combatant.GridY
		}
		combatant.GridX = newX
		combatant.GridY = newY
		s.clearPlayerCover(uid, sess)

		// Compute new distance to the target NPC.
		dist := 0
		if npcCbt != nil {
			dist = combat.CombatRange(*combatant, *npcCbt)
		}
		return messageEvent(detail + fmt.Sprintf(" — success! You tumble through %s's space! Distance: %d ft.", inst.Name(), dist)), nil
	}

	// Failure: player is blocked; NPC makes a Reactive Strike.
	rsNarrative := fmt.Sprintf("%s makes a reactive strike", inst.Name())
	attkResult, attkErr := s.dice.RollExpr("1d20")
	if attkErr != nil {
		return nil, fmt.Errorf("handleTumble: rolling reactive strike attack: %w", attkErr)
	}
	attkRoll := attkResult.Total() + inst.Level
	// Determine player's effective AC from the combatant record.
	playerAC := 10
	if cbt, cbtOk := s.combatH.GetCombatForRoom(sess.RoomID); cbtOk {
		if playerCbt := cbt.GetCombatant(uid); playerCbt != nil {
			playerAC = playerCbt.AC + playerCbt.ACMod
		}
	}

	if attkRoll >= playerAC {
		// Reactive Strike hits: roll 1d6 damage.
		dmgResult, dmgErr := s.dice.RollExpr("1d6")
		if dmgErr != nil {
			return nil, fmt.Errorf("handleTumble: rolling reactive strike damage: %w", dmgErr)
		}
		dmg := dmgResult.Total()
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		s.checkNonCombatDeath(uid, sess) // REQ-BUG100-1
		rsNarrative += fmt.Sprintf(" (hit for %d damage! You have %d HP remaining)", dmg, sess.CurrentHP)
	} else {
		rsNarrative += " (miss)"
	}

	return messageEvent(detail + fmt.Sprintf(" — failure! You fail to tumble through %s's space! %s.", inst.Name(), rsNarrative)), nil
}

// globalRandSrc implements combat.Source using the global math/rand functions.
type globalRandSrc struct{}

func (globalRandSrc) Intn(n int) int { return rand.Intn(n) }

// maxNPCPerceptionInRoom returns the highest Perception value among all living NPCs in roomID.
// If no living NPCs are present, returns 10 as the base DC.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns an integer >= 10.
func (s *GameServiceServer) maxNPCPerceptionInRoom(roomID string) int {
	insts := s.npcMgr.InstancesInRoom(roomID)
	max := 10
	for _, inst := range insts {
		if !inst.IsDead() && inst.Awareness > max {
			max = inst.Awareness
		}
	}
	return max
}

// maxNPCStealthInRoom returns the highest Stealth value among all living NPCs in roomID.
// A zero Stealth value is treated as 10. Returns 10 if no living NPCs are present.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns an integer >= 10.
func (s *GameServiceServer) maxNPCStealthInRoom(roomID string) int {
	insts := s.npcMgr.InstancesInRoom(roomID)
	max := 10

	// Fetch active combat for this room once so we can check NPC cover conditions.
	var cbt *combat.Combat
	if s.combatH != nil {
		if c, ok := s.combatH.GetCombatForRoom(roomID); ok {
			cbt = c
		}
	}

	for _, inst := range insts {
		if inst.IsDead() {
			continue
		}
		stealth := inst.Stealth
		if stealth == 0 {
			stealth = 10
		}
		// Add stealth bonus from any active cover conditions on this NPC combatant.
		if cbt != nil {
			if condSet, ok := cbt.Conditions[inst.ID]; ok && condSet != nil {
				stealth += condition.StealthBonus(condSet)
			}
		}
		if stealth > max {
			max = stealth
		}
	}
	return max
}

// handleHide performs a stealth skill check against the highest NPC Perception DC in the room.
// On success, applies the hidden condition to the player and sets combatant Hidden flag.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat.
// Postcondition: On success, hidden condition is applied; combatant Hidden is true.
func (s *GameServiceServer) handleHide(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Hide is only available in combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleHide: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["stealth"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "hide"

	detail := fmt.Sprintf("Hide (stealth DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. You fail to hide."), nil
	}

	// Success: apply hidden condition to player session.
	if s.condRegistry != nil {
		if def, ok2 := s.condRegistry.Get("hidden"); ok2 {
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleHide: condition apply failed", zap.Error(err))
			}
		}
	}
	if err := s.combatH.SetCombatantHidden(uid, true); err != nil {
		s.logger.Warn("handleHide: SetCombatantHidden failed", zap.Error(err))
	}

	return messageEvent(detail + " — success! You are hidden."), nil
}

// handleSeek performs a Perception check against the highest NPC Stealth DC in the room.
// On success, sets RevealedUntilRound on all hidden NPC combatants to cbt.Round+1.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat.
// Postcondition: On success, all hidden NPC combatants have RevealedUntilRound > cbt.Round.
func (s *GameServiceServer) handleSeek(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if sess.Status != statusInCombat {
		return errorEvent("Seek is only available in combat."), nil
	}
	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleSeek: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["awareness"])
	total := roll + bonus
	dc := s.maxNPCStealthInRoom(sess.RoomID)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "seek"

	detail := fmt.Sprintf("Seek (stealth DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — you search carefully but find nothing hidden."), nil
	}

	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		return messageEvent(detail + " — no active combat found."), nil
	}

	var revealed []string
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && c.Hidden {
			if err := s.combatH.SetCombatantRevealedUntilRound(sess.RoomID, c.ID, cbt.Round+1); err != nil {
				s.logger.Warn("handleSeek: SetCombatantRevealedUntilRound failed", zap.Error(err))
				continue
			}
			revealed = append(revealed, c.Name)
		}
	}
	if len(revealed) == 0 {
		return messageEvent(detail + " — you carefully scan the area but find no hidden threats."), nil
	}
	return messageEvent(detail + fmt.Sprintf(" — success! You locate: %s.", strings.Join(revealed, ", "))), nil
}

// handleMotive performs a sense motive check against a target NPC.
// In combat: 4-outcome system revealing NPC state. Out of combat: reveals disposition.
//
// Precondition: uid is a valid player session; req.Target names an NPC in the player's room.
// Postcondition: On crit success reveals full NPC state; on success reveals next action or disposition;
//
//	on crit fail sets inst.MotiveBonus=2 or flips disposition to hostile.
func (s *GameServiceServer) handleMotive(uid string, req *gamev1.MotiveRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: motive <target>"), nil
	}

	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return errorEvent("Combat handler unavailable."), nil
		}
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil
		}
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleMotive: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["awareness"])
	total := roll + bonus
	dc := 10 + inst.Hustle
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "motive"

	outcome := combat.OutcomeFor(total, dc)
	detail := fmt.Sprintf("Motive (Hustle DC %d): rolled %d+%d=%d", dc, roll, bonus, total)

	if inCombat {
		return s.handleMotiveInCombat(detail, outcome, inst)
	}
	return s.handleMotiveOutOfCombat(detail, outcome, inst)
}

// handleMotiveInCombat applies the 4-outcome sense motive results during combat.
//
// Precondition: detail is a non-empty roll summary string; inst is non-nil.
// Postcondition: On crit success reveals full NPC state; on success reveals next action;
//
//	on failure returns cannot-read message; on crit fail sets inst.MotiveBonus=2.
func (s *GameServiceServer) handleMotiveInCombat(detail string, outcome combat.Outcome, inst *npc.Instance) (*gamev1.ServerEvent, error) {
	switch outcome {
	case combat.CritSuccess:
		msg := detail + " — critical success!\n"
		msg += fmt.Sprintf("%s: %s\n", inst.Name(), motiveNextAction(inst))
		if len(inst.SenseAbilities) > 0 {
			msg += fmt.Sprintf("Hidden abilities: %s\n", strings.Join(inst.SenseAbilities, ", "))
		}
		if len(inst.Resistances) > 0 {
			msg += "Resistant to: " + formatResistanceMap(inst.Resistances) + "\n"
		}
		if len(inst.Weaknesses) > 0 {
			msg += "Weak to: " + formatResistanceMap(inst.Weaknesses) + "\n"
		}
		return messageEvent(strings.TrimRight(msg, "\n")), nil

	case combat.Success:
		return messageEvent(detail + " — success! " + inst.Name() + " " + motiveNextAction(inst) + "."), nil

	case combat.Failure:
		return messageEvent(detail + " — failure. You cannot read their intentions."), nil

	default: // CritFailure
		inst.MotiveBonus = 2
		return messageEvent(detail + " — critical failure. You misread them completely — they notice."), nil
	}
}

// handleMotiveOutOfCombat applies sense motive results outside of combat.
//
// Precondition: detail is a non-empty roll summary string; inst is non-nil.
// Postcondition: On success reveals disposition; on failure cannot-read; on crit fail flips disposition to hostile.
func (s *GameServiceServer) handleMotiveOutOfCombat(detail string, outcome combat.Outcome, inst *npc.Instance) (*gamev1.ServerEvent, error) {
	switch outcome {
	case combat.CritSuccess, combat.Success:
		return messageEvent(fmt.Sprintf("%s — success! %s seems %s.", detail, inst.Name(), inst.Disposition)), nil

	case combat.Failure:
		return messageEvent(detail + " — failure. You cannot get a read on them."), nil

	default: // CritFailure
		if inst.Disposition == "neutral" || inst.Disposition == "wary" {
			inst.Disposition = "hostile"
		}
		return messageEvent(detail + " — critical failure. You misread them badly."), nil
	}
}

// motiveNextAction returns the "next intended action" heuristic string for an NPC.
//
// Precondition: inst is non-nil; inst.MaxHP > 0.
// Postcondition: Returns a non-empty descriptive string.
func motiveNextAction(inst *npc.Instance) string {
	hpPct := float64(inst.CurrentHP) / float64(inst.MaxHP) * 100
	if hpPct < 25 {
		return "looks ready to flee"
	}
	if len(inst.SenseAbilities) > 0 {
		return "seems to be holding something back"
	}
	return "looks focused on the fight"
}

// formatResistanceMap returns "fire (5), cold (3)" format for a resistance/weakness map.
// Keys are sorted for determinism.
//
// Precondition: m is non-nil.
// Postcondition: Returns a non-empty comma-separated string of "key (val)" pairs sorted by key.
func formatResistanceMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s (%d)", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// handleWrath processes the "wrath" active feat (PF2E Barbarian Rage analogue).
// Costs 1 AP in combat. Applies wrath_active condition (encounter duration, +2 damage,
// blocks concentrate actions). Cannot be reactivated for 1 minute after it ends (REQ-80-1).
//
// Precondition: uid must be a valid player session; condRegistry must contain wrath_active.
// Postcondition: wrath_active condition applied to combat or session conditions; 1 AP spent if in combat.
func (s *GameServiceServer) handleWrath(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	// Block if already in Wrath.
	if sess.Conditions != nil && sess.Conditions.Has("wrath_active") {
		return messageEvent("You are already in Wrath."), nil
	}
	// REQ-80-1: block if within the 1-minute cooldown window.
	if time.Now().Before(sess.WrathCooldownUntil) {
		remaining := time.Until(sess.WrathCooldownUntil).Round(time.Second)
		return messageEvent(fmt.Sprintf("Your fury is spent. You cannot enter Wrath again for %v.", remaining)), nil
	}
	// Spend 1 AP when in active combat.
	if sess.Status == statusInCombat && s.combatH != nil {
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil
		}
	}
	// Apply wrath_active condition.
	if s.condRegistry != nil {
		if def, ok := s.condRegistry.Get("wrath_active"); ok {
			var cbt *combat.Combat
			if s.combatH != nil {
				cbt = s.combatH.ActiveCombatForPlayer(uid)
			}
			if cbt != nil {
				if applySet := cbt.Conditions[sess.UID]; applySet != nil {
					if err := applySet.Apply(sess.UID, def, 1, -1); err != nil {
						s.logger.Warn("handleWrath: failed to apply wrath_active (combat)",
							zap.String("uid", uid), zap.Error(err))
					}
				}
			}
			if sess.Conditions != nil {
				if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
					s.logger.Warn("handleWrath: failed to apply wrath_active (session)",
						zap.String("uid", uid), zap.Error(err))
				}
			}
		}
	}
	return messageEvent("Fury overtakes you. You stop holding back."), nil
}

// overpowerCommitted reports whether the given player has the
// overpower_committed condition active on their combatant or session — true
// when Overpower was used this round and further attacks are blocked.
//
// Precondition: none.
// Postcondition: returns true iff overpower_committed is present on the
// player's combat condition set (preferred) or their session condition set.
func (s *GameServiceServer) overpowerCommitted(uid string) bool {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return false
	}
	if s.combatH != nil {
		if cbt := s.combatH.ActiveCombatForPlayer(uid); cbt != nil {
			if set := cbt.Conditions[sess.UID]; set != nil && set.Has("overpower_committed") {
				return true
			}
		}
	}
	if sess.Conditions != nil && sess.Conditions.Has("overpower_committed") {
		return true
	}
	return false
}

// handleOverpower activates the Overpower feat, applying the brutal_surge_active condition.
//
// REQ-60-1: Overpower costs 2 AP when activated during combat.
// REQ-60-2: Overpower requires an empty off-hand slot (it is a two-handed wind-up strike).
//
// Precondition: uid must be a valid player session; player must be in active combat.
// Postcondition: On success, brutal_surge_active condition is applied and 2 AP is spent.
func (s *GameServiceServer) handleOverpower(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// REQ-60-2: Overpower requires an empty off-hand (two-handed wind-up strike).
	if sess.LoadoutSet != nil {
		if preset := sess.LoadoutSet.ActivePreset(); preset != nil && preset.OffHand != nil {
			return messageEvent("Overpower requires a free off-hand — unequip your off-hand item first."), nil
		}
	}

	// RequiresCombat: player must be in an active encounter.
	var inCombat bool
	if s.combatH != nil {
		inCombat = s.combatH.ActiveCombatForPlayer(uid) != nil
	}
	if !inCombat {
		return messageEvent("You must be in combat to use Overpower."), nil
	}

	// REQ-60-1: Spend 2 AP.
	if err := s.combatH.SpendAP(uid, 2); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Apply brutal_surge_active (encounter-long +dmg/-AC buff) and
	// overpower_committed (GH #231: blocks further attack/strike actions until
	// the start of the player's next turn).
	if s.condRegistry != nil {
		var cbt *combat.Combat
		if s.combatH != nil {
			cbt = s.combatH.ActiveCombatForPlayer(uid)
		}
		applyCondition := func(id string, duration int) {
			def, ok := s.condRegistry.Get(id)
			if !ok {
				return
			}
			if cbt != nil {
				if applySet := cbt.Conditions[sess.UID]; applySet != nil {
					if err := applySet.Apply(sess.UID, def, 1, duration); err != nil {
						s.logger.Warn("handleOverpower: failed to apply condition",
							zap.String("uid", uid), zap.String("cond", id), zap.Error(err))
					}
					return
				}
			}
			if sess.Conditions != nil {
				if err := sess.Conditions.Apply(sess.UID, def, 1, duration); err != nil {
					s.logger.Warn("handleOverpower: failed to apply condition (session)",
						zap.String("uid", uid), zap.String("cond", id), zap.Error(err))
				}
			}
		}
		applyCondition("brutal_surge_active", -1)
		applyCondition("overpower_committed", 1)
	}
	return messageEvent("You put everything into it."), nil
}

// handleCalm attempts to calm the player's worst active mental state via a Grit check.
// In combat: costs all remaining AP. Out of combat: no AP cost.
//
// Precondition: uid must be a valid player session.
// Postcondition: On success, worst active track steps down one severity level.
func (s *GameServiceServer) handleCalm(uid string, _ *gamev1.CalmRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if s.mentalStateMgr == nil {
		return errorEvent("Mental state system unavailable."), nil
	}

	track, sev := s.mentalStateMgr.WorstActiveTrack(uid)
	if sev == mentalstate.SeverityNone {
		return messageEvent("You are mentally composed — nothing to calm."), nil
	}

	if sess.Status == statusInCombat && s.combatH != nil {
		s.combatH.SpendAllAP(uid)
	}

	if s.dice == nil {
		return errorEvent("Dice roller unavailable."), nil
	}
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleCalm: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	gritMod := combat.AbilityMod(sess.Abilities.Grit)
	total := roll + gritMod
	dc := 10 + int(sev)*4
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "calm"

	detail := fmt.Sprintf("Calm (Grit DC %d): rolled %d+%d=%d", dc, roll, gritMod, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your mental state resists your efforts."), nil
	}

	changes := s.mentalStateMgr.Recover(uid, track)
	if s.combatH != nil {
		s.combatH.applyMentalStateChanges(uid, changes)
	} else {
		applyMentalChangesToSession(sess, uid, changes, s.condRegistry, s.logger)
	}
	msg := detail + " — success!"
	if len(changes) > 0 && changes[0].Message != "" {
		msg += " " + changes[0].Message
	}
	return messageEvent(msg), nil
}

// handleHeroPoint dispatches a hero point spend request to the appropriate sub-handler.
//
// Precondition: uid must be a valid player session; req must be non-nil.
// Postcondition: Returns a ServerEvent or an error; delegates to handleHeroPointReroll
// or handleHeroPointStabilize based on req.Subcommand.
func (s *GameServiceServer) handleHeroPoint(uid string, req *gamev1.HeroPointRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	switch req.Subcommand {
	case "reroll":
		return s.handleHeroPointReroll(sess)
	case "stabilize":
		return s.handleHeroPointStabilize(sess)
	default:
		return errorEvent("Unknown heropoint subcommand. Use: heropoint reroll | heropoint stabilize"), nil
	}
}

// handleHeroPointReroll spends 1 hero point to reroll the most recent ability check,
// keeping whichever result is higher.
//
// Precondition: sess.HeroPoints >= 1; sess.LastCheckRoll != 0.
// Postcondition: HeroPoints decremented; LastCheckRoll updated to winner; SaveHeroPoints called.
func (s *GameServiceServer) handleHeroPointReroll(sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
	if sess.HeroPoints < 1 {
		return errorEvent("You have no hero points remaining."), nil
	}
	if sess.LastCheckRoll == 0 {
		return errorEvent("You have no recent check to reroll."), nil
	}

	if s.dice == nil {
		return errorEvent("Dice roller unavailable."), nil
	}
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleHeroPointReroll: rolling d20: %w", err)
	}
	newRoll := rollResult.Total()
	oldRoll := sess.LastCheckRoll

	winner := oldRoll
	if newRoll > oldRoll {
		winner = newRoll
	}

	sess.HeroPoints--
	sess.LastCheckRoll = winner

	if s.charSaver != nil && sess.CharacterID != 0 {
		if hpErr := s.charSaver.SaveHeroPoints(context.Background(), sess.CharacterID, sess.HeroPoints); hpErr != nil {
			s.logger.Warn("handleHeroPointReroll: SaveHeroPoints failed", zap.Error(hpErr))
		}
	}

	msg := fmt.Sprintf("You spend a hero point. Original roll: %d, New roll: %d — keeping %d.", oldRoll, newRoll, winner)
	s.pushCharacterSheet(sess)
	return messageEvent(msg), nil
}

// handleHeroPointStabilize spends 1 hero point to stabilize a dying player at 0 HP.
//
// Precondition: sess.HeroPoints >= 1; sess.Dead == true.
// Postcondition: Dead cleared; CurrentHP set to 0; HeroPoints decremented; SaveHeroPoints called.
func (s *GameServiceServer) handleHeroPointStabilize(sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
	if sess.HeroPoints < 1 {
		return errorEvent("You have no hero points remaining."), nil
	}
	if !sess.Dead {
		return errorEvent("You are not dying — stabilize requires being at death's door."), nil
	}

	sess.Dead = false
	sess.CurrentHP = 0
	sess.HeroPoints--

	if s.charSaver != nil && sess.CharacterID != 0 {
		if hpErr := s.charSaver.SaveHeroPoints(context.Background(), sess.CharacterID, sess.HeroPoints); hpErr != nil {
			s.logger.Warn("handleHeroPointStabilize: SaveHeroPoints failed", zap.Error(hpErr))
		}
	}

	s.pushCharacterSheet(sess)
	return messageEvent("You spend a hero point, pulling back from the brink. You stabilize at 0 HP."), nil
}

// respawnPlayer moves a downed player to their zone's start room and restores them to 1 HP.
//
// Precondition: uid is a valid player session that has been downed in combat.
// Postcondition: sess.Dead cleared; sess.CurrentHP = 1; sess.RoomID = zone start room;
// character state persisted; player receives a narrative message and fresh room view.
func (s *GameServiceServer) respawnPlayer(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}

	// Resolve spawn room: use the zone start room for the player's current room.
	spawnRoomID := ""
	if currentRoom, roomOK := s.world.GetRoom(sess.RoomID); roomOK {
		if zone, zoneOK := s.world.GetZone(currentRoom.ZoneID); zoneOK && zone.StartRoom != "" {
			spawnRoomID = zone.StartRoom
		}
	}
	// Fallback to global start room if zone lookup fails.
	if spawnRoomID == "" {
		if sr := s.world.StartRoom(); sr != nil {
			spawnRoomID = sr.ID
		}
	}
	if spawnRoomID == "" {
		s.logger.Warn("respawnPlayer: no spawn room found", zap.String("uid", uid))
		return
	}

	spawnRoom, ok := s.world.GetRoom(spawnRoomID)
	if !ok {
		s.logger.Warn("respawnPlayer: spawn room not found", zap.String("uid", uid), zap.String("room_id", spawnRoomID))
		return
	}

	oldRoomID := sess.RoomID
	sess.Dead = false
	sess.CurrentHP = sess.MaxHP
	sess.RoomID = spawnRoomID

	// Clear all conditions (encounter, permanent, rounds, until_save) on respawn.
	if sess.Conditions != nil {
		sess.Conditions.ClearAll()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Restore spontaneous tech use pools via DB then reload into session.
	if s.spontaneousUsePoolRepo != nil && sess.CharacterID != 0 {
		if err := s.spontaneousUsePoolRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
			s.logger.Warn("respawnPlayer: restore spontaneous use pools failed", zap.String("uid", uid), zap.Error(err))
		} else {
			pools, err := s.spontaneousUsePoolRepo.GetAll(ctx, sess.CharacterID)
			if err != nil {
				s.logger.Warn("respawnPlayer: reload spontaneous use pools failed", zap.String("uid", uid), zap.Error(err))
			} else {
				sess.SpontaneousUsePools = pools
			}
		}
	}

	// Restore innate tech use slots via DB then reload into session.
	if s.innateTechRepo != nil && sess.CharacterID != 0 {
		if err := s.innateTechRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
			s.logger.Warn("respawnPlayer: restore innate tech slots failed", zap.String("uid", uid), zap.Error(err))
		} else {
			innates, err := s.innateTechRepo.GetAll(ctx, sess.CharacterID)
			if err != nil {
				s.logger.Warn("respawnPlayer: reload innate tech slots failed", zap.String("uid", uid), zap.Error(err))
			} else {
				sess.InnateTechs = innates
			}
		}
	}

	// Un-expend all prepared tech slots in-memory.
	for _, slots := range sess.PreparedTechs {
		for _, slot := range slots {
			if slot != nil {
				slot.Expended = false
			}
		}
	}

	// Restore active feat uses to their PreparedUses maximum.
	if s.characterFeatsRepo != nil && s.featRegistry != nil && sess.ActiveFeatUses != nil && sess.CharacterID != 0 {
		if featIDs, err := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID); err != nil {
			s.logger.Warn("respawnPlayer: failed to load feats for use restoration", zap.String("uid", uid), zap.Error(err))
		} else {
			for _, id := range featIDs {
				f, ok := s.featRegistry.Feat(id)
				if !ok || !f.Active || f.PreparedUses <= 0 {
					continue
				}
				sess.ActiveFeatUses[id] = f.PreparedUses
			}
		}
	}

	// Restore focus points to maximum.
	sess.FocusPoints = sess.MaxFocusPoints

	// Persist new location and HP.
	if s.charSaver != nil && sess.CharacterID != 0 {
		if err := s.charSaver.SaveState(ctx, sess.CharacterID, spawnRoomID, sess.CurrentHP); err != nil {
			s.logger.Warn("respawnPlayer: SaveState failed", zap.String("uid", uid), zap.Error(err))
		}
	}

	// Broadcast departure from combat room.
	s.broadcastRoomEvent(oldRoomID, uid, &gamev1.RoomEvent{
		Player: sess.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
	})

	// Broadcast arrival in spawn room.
	s.broadcastRoomEvent(spawnRoomID, uid, &gamev1.RoomEvent{
		Player: sess.CharName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Push character sheet so HP shows updated.
	s.pushCharacterSheet(sess)

	// Push respawn narrative.
	s.pushMessageToUID(uid, "You collapse and wake up battered at a safe location.")

	// Push room view for new room.
	rv := s.worldH.buildRoomView(uid, spawnRoom)
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
	}
	s.pushEventToUID(uid, evt)
}

// checkNonCombatDeath detects when a player's HP has reached 0 outside of
// combat and triggers the respawn path.  It must be called after every
// non-combat damage application (trap, fall, drown, radiation, skill-check,
// reactive-strike) to ensure consistent death handling (REQ-BUG100-1).
//
// Precondition: sess must be non-nil; uid must match sess.UID.
// Postcondition: If sess.CurrentHP <= 0, sess.Dead is set and respawnPlayer is
// called asynchronously; otherwise this is a no-op.
func (s *GameServiceServer) checkNonCombatDeath(uid string, sess *session.PlayerSession) {
	if sess.CurrentHP > 0 {
		return
	}
	sess.Dead = true
	go s.respawnPlayer(uid)
}

// applyMentalChangesToSession applies mental state condition swaps directly to a session.
// Used outside combat when CombatHandler is not available.
//
// Precondition: sess must be non-nil.
// Postcondition: Conditions listed in changes are removed/applied to the session.
func applyMentalChangesToSession(sess *session.PlayerSession, uid string, changes []mentalstate.StateChange, condReg *condition.Registry, logger *zap.Logger) {
	if sess.Conditions == nil {
		if logger != nil {
			logger.Warn("applyMentalChangesToSession: sess.Conditions is nil; mental state changes lost",
				zap.String("uid", uid),
			)
		}
		return
	}
	if condReg == nil {
		return
	}
	for _, ch := range changes {
		if ch.OldConditionID != "" {
			sess.Conditions.Remove(uid, ch.OldConditionID)
		}
		if ch.NewConditionID != "" {
			def, ok := condReg.Get(ch.NewConditionID)
			if ok {
				if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
					if logger != nil {
						logger.Warn("applyMentalChangesToSession: failed to apply condition",
							zap.String("uid", uid),
							zap.String("condition_id", ch.NewConditionID),
							zap.Error(err),
						)
					}
				}
			}
		}
	}
}

// npcHPTier returns a human-readable HP tier string for the given current/max HP values.
//
// Precondition: maxHP > 0.
// Postcondition: Returns one of "unharmed", "lightly wounded", "bloodied", "badly wounded".
func npcHPTier(currentHP, maxHP int) string {
	if maxHP <= 0 {
		return "badly wounded"
	}
	ratio := float64(currentHP) / float64(maxHP)
	switch {
	case ratio > 0.75:
		return "unharmed"
	case ratio > 0.50:
		return "lightly wounded"
	case ratio > 0.25:
		return "bloodied"
	default:
		return "badly wounded"
	}
}

// handleSneak performs a stealth skill check while hidden.
// On failure, removes the hidden condition and clears combatant Hidden flag.
// Requires the player to already have the hidden condition.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; player must have hidden condition.
// Postcondition: On failure, hidden condition is removed; on success, hidden is maintained.
func (s *GameServiceServer) handleSneak(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Sneak is only available in combat."), nil
	}

	if sess.Conditions == nil || !sess.Conditions.Has("hidden") {
		return errorEvent("You must be hidden to sneak. Use 'hide' first."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleSneak: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["stealth"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "sneak"

	detail := fmt.Sprintf("Sneak (stealth DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		// Failure: remove hidden condition.
		sess.Conditions.Remove(uid, "hidden")
		if err := s.combatH.SetCombatantHidden(uid, false); err != nil {
			s.logger.Warn("handleSneak: SetCombatantHidden false failed", zap.Error(err))
		}
		return messageEvent(detail + " — failure. You have been spotted and lose the hidden condition."), nil
	}

	return messageEvent(detail + " — success! You remain hidden."), nil
}

// handleDivert performs a grift skill check to create a diversion against the highest NPC Perception DC.
// On success, applies the hidden condition to the player and sets combatant Hidden flag.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat.
// Postcondition: On success, hidden condition is applied; combatant Hidden is true.
func (s *GameServiceServer) handleDivert(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Divert is only available in combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDivert: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["grift"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "divert"

	detail := fmt.Sprintf("Divert (grift DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your diversion fails."), nil
	}

	// Success: apply hidden condition to player session.
	if s.condRegistry != nil {
		if def, ok2 := s.condRegistry.Get("hidden"); ok2 {
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleDivert: condition apply failed", zap.Error(err))
			}
		}
	}
	if err := s.combatH.SetCombatantHidden(uid, true); err != nil {
		s.logger.Warn("handleDivert: SetCombatantHidden failed", zap.Error(err))
	}

	return messageEvent(detail + " — success! You slip into the shadows while enemies are distracted."), nil
}

// handleEscape performs a max(muscle, acrobatics) skill check to escape the grabbed condition.
// DC is grabber's Level+14 if the grabber NPC is alive in the room, else 15.
// Requires the player to have the grabbed condition.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; player must have grabbed condition.
// Postcondition: On success, grabbed condition is removed; GrabberID is cleared.
func (s *GameServiceServer) handleEscape(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Escape is only available in combat."), nil
	}

	hasGrabbed := sess.Conditions != nil && sess.Conditions.Has("grabbed")
	hasSubmerged := sess.Conditions != nil && sess.Conditions.Has("submerged")
	if !hasGrabbed && !hasSubmerged {
		return errorEvent("You are not grabbed or submerged."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleEscape: rolling d20: %w", err)
	}
	roll := rollResult.Total()

	mus := skillRankBonus(sess.Skills["muscle"])
	acr := skillRankBonus(sess.Skills["acrobatics"])
	bonus := mus
	if acr > bonus {
		bonus = acr
	}
	total := roll + bonus

	// Determine DC: grabber Level+14 if alive in room, else 15.
	dc := 15
	if sess.GrabberID != "" {
		insts := s.npcMgr.InstancesInRoom(sess.RoomID)
		for _, inst := range insts {
			if inst.ID == sess.GrabberID && !inst.IsDead() {
				dc = inst.Level + 14
				break
			}
		}
	}

	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "escape"

	detail := fmt.Sprintf("Escape (DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. You fail to break free."), nil
	}

	// Success: remove grabbed and/or submerged conditions.
	if hasGrabbed {
		sess.Conditions.Remove(uid, "grabbed")
		sess.GrabberID = ""
	}
	if hasSubmerged {
		sess.Conditions.Remove(uid, "submerged")
	}

	return messageEvent(detail + " — success! You break free."), nil
}

// issueTechTrainerQuests auto-grants find-trainer quests for any L2+ tech traditions
// in the given deferred grants. Called after level-up creates pending trainer slots.
//
// Precondition: sess non-nil; charLevel > 0; deferredGrants non-nil.
// Postcondition: For each tradition in deferredGrants at L2+, if a matching tech_trainer NPC
// has a FindQuestID and the player does not already hold that quest, the quest is accepted.
func (s *GameServiceServer) issueTechTrainerQuests(
	ctx context.Context,
	sess *session.PlayerSession,
	charLevel int,
	deferredGrants *ruleset.TechnologyGrants,
) {
	if deferredGrants == nil || s.questSvc == nil || s.npcMgr == nil {
		return
	}
	// Collect all (tradition, techLevel) pairs in the deferred grants.
	type tradPair struct {
		tradition string
		techLevel int
	}
	var pairs []tradPair
	if deferredGrants.Prepared != nil {
		for lvl := range deferredGrants.Prepared.SlotsByLevel {
			if lvl >= 2 {
				for _, e := range deferredGrants.Prepared.Pool {
					if e.Level == lvl && s.techRegistry != nil {
						def, ok := s.techRegistry.Get(e.ID)
						if ok {
							pairs = append(pairs, tradPair{string(def.Tradition), lvl})
						}
					}
				}
			}
		}
	}
	if deferredGrants.Spontaneous != nil {
		for lvl := range deferredGrants.Spontaneous.KnownByLevel {
			if lvl >= 2 {
				for _, e := range deferredGrants.Spontaneous.Pool {
					if e.Level == lvl && s.techRegistry != nil {
						def, ok := s.techRegistry.Get(e.ID)
						if ok {
							pairs = append(pairs, tradPair{string(def.Tradition), lvl})
						}
					}
				}
			}
		}
	}

	// Deduplicate (tradition, techLevel) pairs and issue quests.
	seen := make(map[string]bool)
	for _, p := range pairs {
		key := fmt.Sprintf("%s:%d", p.tradition, p.techLevel)
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, tmpl := range s.npcMgr.AllTemplates() {
			if tmpl.NPCType != "tech_trainer" || tmpl.TechTrainer == nil {
				continue
			}
			if tmpl.TechTrainer.Tradition != p.tradition {
				continue
			}
			if !tmpl.TechTrainer.OffersLevel(p.techLevel) {
				continue
			}
			questID := tmpl.TechTrainer.FindQuestID
			if questID == "" {
				continue
			}
			// Skip if already active or completed.
			if _, active := sess.GetActiveQuests()[questID]; active {
				continue
			}
			if _, done := sess.GetCompletedQuests()[questID]; done {
				continue
			}
			// Accept the quest (auto-grant, no NPC giver required).
			title, _, err := s.questSvc.Accept(ctx, sess, sess.CharacterID, questID)
			if err != nil {
				s.logger.Warn("issueTechTrainerQuests: Accept failed",
					zap.String("quest_id", questID),
					zap.Error(err),
				)
			} else if title != "" {
				// REQ-BUG195-1: Notify the player that a new quest was added to the log.
				s.pushMessageToUID(sess.UID, fmt.Sprintf("New quest added to your quest log: %s", title))
			}
		}
	}
}

// issueOnboardingQuest auto-grants the onboarding_find_zone_map quest for a new character.
//
// Precondition: sess non-nil; uid non-empty.
// Postcondition: If quest not already active/completed, it is accepted and a console message sent.
// Errors are logged at Warn and do not abort the caller.
func (s *GameServiceServer) issueOnboardingQuest(ctx context.Context, uid string, sess *session.PlayerSession) {
	if s.questSvc == nil || sess.CharacterID <= 0 {
		return
	}
	const questID = "onboarding_find_zone_map"
	// Skip if already active or completed (migration edge case).
	if _, active := sess.GetActiveQuests()[questID]; active {
		return
	}
	if _, done := sess.GetCompletedQuests()[questID]; done {
		return
	}
	title, _, err := s.questSvc.Accept(ctx, sess, sess.CharacterID, questID)
	if err != nil {
		s.logger.Warn("issueOnboardingQuest: Accept failed",
			zap.String("uid", uid),
			zap.String("quest_id", questID),
			zap.Error(err),
		)
		return
	}
	if title != "" {
		s.pushMessageToUID(uid, "New quest: Find Your Bearings — locate the district map terminal on 82nd Avenue.")
	}
}

// applyLevelUpTechGrants processes technology grants for each level gained between
// fromLevel+1 and toLevel (inclusive). Immediate grants (pool <= open slots) are applied
// at once; deferred L2+ grants are stored in sess.PendingTechGrants, persisted, and
// the matching find-trainer quest is auto-issued (REQ-TTA-7).
//
// Precondition: sess non-nil; fromLevel >= 0; toLevel >= fromLevel; sess.CharacterID > 0.
// Postcondition: PendingTechGrants updated; trainer quests issued for deferred L2+ grants.
func (s *GameServiceServer) applyLevelUpTechGrants(ctx context.Context, sess *session.PlayerSession, fromLevel, toLevel int) {
	if s.hardwiredTechRepo == nil || s.jobRegistry == nil || sess.CharacterID <= 0 {
		return
	}
	job, ok := s.jobRegistry.Job(sess.Class)
	if !ok {
		return
	}
	var archetypeLevelUpGrants map[int]*ruleset.TechnologyGrants
	if job.Archetype != "" {
		if archetype, archetypeOK := s.archetypes[job.Archetype]; archetypeOK {
			archetypeLevelUpGrants = archetype.LevelUpGrants
		}
	}
	mergedLevelUpGrants := ruleset.MergeLevelUpGrants(archetypeLevelUpGrants, job.LevelUpGrants)
	if sess.PendingTechGrants == nil {
		sess.PendingTechGrants = make(map[int]*ruleset.TechnologyGrants)
	}
	for lvl := fromLevel + 1; lvl <= toLevel; lvl++ {
		techGrants, hasGrants := mergedLevelUpGrants[lvl]
		if !hasGrants {
			continue
		}
		immediate, deferred := PartitionTechGrants(techGrants)
		if immediate != nil {
			hwBefore := append([]string{}, sess.HardwiredTechs...)
			prepBefore := snapshotPreparedTechIDs(sess.PreparedTechs)
			spontBefore := snapshotKnownTechIDs(sess.KnownTechs)

			if err := LevelUpTechnologies(ctx, sess, sess.CharacterID,
				immediate, s.techRegistry, nil,
				s.hardwiredTechRepo, s.preparedTechRepo,
				s.knownTechRepo, s.innateTechRepo, s.spontaneousUsePoolRepo,
			); err != nil {
				s.logger.Warn("applyLevelUpTechGrants: LevelUpTechnologies failed",
					zap.Int64("character_id", sess.CharacterID),
					zap.Int("level", lvl),
					zap.Error(err))
			}

			for _, id := range newTechIDs(hwBefore, sess.HardwiredTechs) {
				notifMsg := messageEvent(fmt.Sprintf("You gained %s (auto-assigned).", id))
				if data, mErr := proto.Marshal(notifMsg); mErr == nil {
					_ = sess.Entity.Push(data)
				}
			}
			for _, id := range newTechIDsFromPrepared(prepBefore, sess.PreparedTechs) {
				notifMsg := messageEvent(fmt.Sprintf("You gained %s (auto-assigned).", id))
				if data, mErr := proto.Marshal(notifMsg); mErr == nil {
					_ = sess.Entity.Push(data)
				}
			}
			for _, id := range newTechIDsFromSpontaneous(spontBefore, sess.KnownTechs) {
				notifMsg := messageEvent(fmt.Sprintf("You gained %s (auto-assigned).", id))
				if data, mErr := proto.Marshal(notifMsg); mErr == nil {
					_ = sess.Entity.Push(data)
				}
			}
		}
		if deferred != nil {
			sess.PendingTechGrants[lvl] = deferred
			// REQ-TTA-12: persist L2+ pending slots so trainer can resolve on login.
			if s.pendingTechSlotsRepo != nil && sess.CharacterID > 0 {
				if deferred.Prepared != nil {
					for techLvl, slots := range deferred.Prepared.SlotsByLevel {
						if techLvl < 2 {
							continue
						}
						tradition := ""
						for _, e := range deferred.Prepared.Pool {
							if e.Level == techLvl && s.techRegistry != nil {
								if def, ok := s.techRegistry.Get(e.ID); ok {
									tradition = string(def.Tradition)
									break
								}
							}
						}
						for i := 0; i < slots; i++ {
							if err := s.pendingTechSlotsRepo.AddPendingTechSlot(
								ctx, sess.CharacterID, lvl, techLvl, tradition, "prepared",
							); err != nil {
								s.logger.Warn("applyLevelUpTechGrants: AddPendingTechSlot failed", zap.Error(err))
							}
						}
					}
				}
				if deferred.Spontaneous != nil {
					for techLvl, slots := range deferred.Spontaneous.KnownByLevel {
						if techLvl < 2 {
							continue
						}
						tradition := ""
						for _, e := range deferred.Spontaneous.Pool {
							if e.Level == techLvl && s.techRegistry != nil {
								if def, ok := s.techRegistry.Get(e.ID); ok {
									tradition = string(def.Tradition)
									break
								}
							}
						}
						for i := 0; i < slots; i++ {
							if err := s.pendingTechSlotsRepo.AddPendingTechSlot(
								ctx, sess.CharacterID, lvl, techLvl, tradition, "spontaneous",
							); err != nil {
								s.logger.Warn("applyLevelUpTechGrants: AddPendingTechSlot failed", zap.Error(err))
							}
						}
					}
				}
			}
			// REQ-TTA-7: auto-issue find-trainer quests for L2+ pending traditions.
			s.issueTechTrainerQuests(ctx, sess, lvl, deferred)
		}
	}
	if len(sess.PendingTechGrants) > 0 && s.progressRepo != nil {
		levels := make([]int, 0, len(sess.PendingTechGrants))
		for lvl := range sess.PendingTechGrants {
			levels = append(levels, lvl)
		}
		sort.Ints(levels)
		if err := s.progressRepo.SetPendingTechLevels(ctx, sess.CharacterID, levels); err != nil {
			s.logger.Warn("applyLevelUpTechGrants: SetPendingTechLevels failed", zap.Error(err))
		}
		selectNotif := messageEvent("You have pending technology selections! Type 'selecttech' to choose your technologies.")
		if data, mErr := proto.Marshal(selectNotif); mErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
}

// handleGrant awards XP, money, or an item to a named or room-scoped player (editor/admin command).
//
// Precondition: uid identifies an active session with role "editor" or "admin"; req is non-nil.
// Postcondition: on "xp" grant, target.Experience is increased and persisted;
// on "money" grant, target.Currency is increased and persisted;
// on "item" grant, the item is placed on the floor of the editor's current room;
// target receives a console notification; caller receives a success MessageEvent.
func (s *GameServiceServer) handleGrant(uid string, req *gamev1.GrantRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}

	// Handle item grant: drop item on editor's room floor (no target player needed).
	if req.GrantType == "item" {
		if req.ItemId == "" {
			return errorEvent("grant_item requires an item_id"), nil
		}
		if s.floorMgr == nil {
			return errorEvent("floor system not available"), nil
		}
		itemID := req.ItemId
		inst := inventory.ItemInstance{
			InstanceID: fmt.Sprintf("grant-%s-%d", itemID, time.Now().UnixNano()),
			ItemDefID:  itemID,
			Quantity:   1,
		}
		s.floorMgr.Drop(sess.RoomID, inst)
		return messageEvent(fmt.Sprintf("granted %s to room %s.", itemID, sess.RoomID)), nil
	}

	if req.GrantType != "heropoint" && req.Amount <= 0 {
		return errorEvent("amount must be greater than zero"), nil
	}

	// Handle money grant with empty char_name: give to all players in editor's room.
	if req.GrantType == "money" && req.CharName == "" {
		amount := int(req.Amount)
		ctx := context.Background()
		roomPlayers := s.sessions.PlayersInRoomDetails(sess.RoomID)
		count := 0
		for _, p := range roomPlayers {
			if p.UID == uid {
				continue // skip the editor themselves
			}
			p.Currency += amount
			if p.CharacterID > 0 && s.charSaver != nil {
				if err := s.charSaver.SaveCurrency(ctx, p.CharacterID, p.Currency); err != nil {
					s.logger.Warn("handleGrant: SaveCurrency failed", zap.Int64("char_id", p.CharacterID), zap.Error(err))
				}
			}
			s.pushMessageToUID(p.UID, fmt.Sprintf("You received %d credits.", amount))
			count++
		}
		return messageEvent(fmt.Sprintf("granted %d credits to %d player(s) in room.", amount, count)), nil
	}

	target, ok := s.sessions.GetPlayerByCharName(req.CharName)
	if !ok {
		return errorEvent(fmt.Sprintf("player %q not online", req.CharName)), nil
	}

	amount := int(req.Amount)
	ctx := context.Background()

	switch req.GrantType {
	case "xp":
		var levelMsgs []string
		if s.xpSvc != nil {
			oldLevel := target.Level
			result := xp.Award(target.Level, target.Experience, amount, s.xpSvc.Config())
			target.Experience = result.NewXP
			target.Level = result.NewLevel
			target.MaxHP += result.HPGained
			if target.CurrentHP > target.MaxHP {
				target.CurrentHP = target.MaxHP
			}
			target.PendingBoosts += result.NewBoosts
			target.PendingSkillIncreases += result.NewSkillIncreases
			if target.CharacterID > 0 && s.charSaver != nil {
				if pErr := s.charSaver.SaveProgress(ctx, target.CharacterID, target.Level, target.Experience, target.MaxHP, target.PendingBoosts); pErr != nil {
					s.logger.Warn("handleGrant: SaveProgress failed", zap.Error(pErr))
				}
			}
			if result.NewSkillIncreases > 0 && s.progressRepo != nil {
				if err := s.progressRepo.IncrementPendingSkillIncreases(ctx, target.CharacterID, result.NewSkillIncreases); err != nil {
					s.logger.Warn("handleGrant: IncrementPendingSkillIncreases failed", zap.Error(err))
				}
				_ = s.progressRepo.MarkSkillIncreasesInitialized(ctx, target.CharacterID)
			}
			if result.LeveledUp {
				levelMsgs = append(levelMsgs, fmt.Sprintf("*** You reached level %d! ***", result.NewLevel))
				if result.HPGained > 0 {
					levelMsgs = append(levelMsgs, fmt.Sprintf("Max HP increased by %d (now %d).", result.HPGained, target.MaxHP))
				}
				if result.NewBoosts > 0 {
					levelMsgs = append(levelMsgs, "You have a pending ability boost! Type 'levelup' to assign it.")
				}
				if result.NewSkillIncreases > 0 {
					levelMsgs = append(levelMsgs, "You have a pending skill increase! Type 'trainskill <skill>' to advance a skill.")
				}
				// Award 1 hero point on level-up (REQ-AWARD1).
				target.HeroPoints++
				if target.CharacterID > 0 && s.charSaver != nil {
					if hpErr := s.charSaver.SaveHeroPoints(ctx, target.CharacterID, target.HeroPoints); hpErr != nil {
						s.logger.Warn("handleGrant: SaveHeroPoints failed on level-up", zap.Error(hpErr))
					}
				}
				levelMsgs = append(levelMsgs, "You earned 1 hero point!")
				// REQ-BUG99-2: apply technology level-up grants via shared method so all
				// grant paths (admin and organic) behave identically.
				s.applyLevelUpTechGrants(ctx, target, oldLevel, result.NewLevel)
			}
			// Apply feat level-up grants for each level gained.
			// Fixed feats are granted immediately; choice feats are auto-picked
			// from the pool (first available not already owned).
			if s.characterFeatsRepo != nil && s.jobRegistry != nil && target.CharacterID > 0 {
				if job, ok := s.jobRegistry.Job(target.Class); ok {
					var archetypeFeatGrants map[int]*ruleset.FeatGrants
					if job.Archetype != "" {
						if arch, archOK := s.archetypes[job.Archetype]; archOK {
							archetypeFeatGrants = arch.LevelUpFeatGrants
						}
					}
					mergedFeatGrants := ruleset.MergeFeatLevelUpGrants(archetypeFeatGrants, job.LevelUpFeatGrants)
					if len(mergedFeatGrants) > 0 {
						existingFeatIDs, featGetErr := s.characterFeatsRepo.GetAll(ctx, target.CharacterID)
						if featGetErr != nil {
							s.logger.Warn("handleGrant: GetAll feats failed", zap.Error(featGetErr))
						} else {
							existing := make(map[string]bool, len(existingFeatIDs))
							for _, id := range existingFeatIDs {
								existing[id] = true
							}
							for lvl := oldLevel + 1; lvl <= result.NewLevel; lvl++ {
								fg, hasFG := mergedFeatGrants[lvl]
								if !hasFG || fg == nil {
									continue
								}
								grantedIDs, applyErr := ApplyFeatGrant(ctx, target.CharacterID, existing, fg, s.featRegistry, s.characterFeatsRepo)
								if applyErr != nil {
									s.logger.Warn("handleGrant: ApplyFeatGrant failed",
										zap.Int64("character_id", target.CharacterID),
										zap.Int("level", lvl),
										zap.Error(applyErr),
									)
								}
								if s.featLevelGrantsRepo != nil {
									if mErr := s.featLevelGrantsRepo.MarkLevelGranted(ctx, target.CharacterID, lvl); mErr != nil {
										s.logger.Warn("MarkLevelGranted failed",
											zap.Int64("character_id", target.CharacterID),
											zap.Int("level", lvl),
											zap.Error(mErr),
										)
									}
								}
								for _, id := range grantedIDs {
									f, _ := s.featRegistry.Feat(id)
									name := id
									if f != nil && f.Name != "" {
										name = f.Name
									}
									notifMsg := messageEvent(fmt.Sprintf("You gained the feat: %s!", name))
									if data, mErr := proto.Marshal(notifMsg); mErr == nil {
										_ = target.Entity.Push(data)
									}
								}
							}
						}
					}
				}
			}
		} else {
			target.Experience += amount
			if s.charSaver != nil && target.CharacterID > 0 {
				if pErr := s.charSaver.SaveProgress(ctx, target.CharacterID, target.Level, target.Experience, target.MaxHP, target.PendingBoosts); pErr != nil {
					s.logger.Warn("handleGrant: SaveProgress failed", zap.Error(pErr))
				}
			}
		}
		// Send level-up messages to target if any.
		for _, m := range levelMsgs {
			notif := messageEvent(m)
			if data, mErr := proto.Marshal(notif); mErr == nil {
				_ = target.Entity.Push(data)
			}
		}
		// Push updated character sheet so Stats tab reflects new level, HP, and pending boosts.
		if len(levelMsgs) > 0 {
			s.pushCharacterSheet(target)
		}
		// Notify target.
		notif := messageEvent(fmt.Sprintf("You have been granted %d XP by %s.", amount, sess.CharName))
		if data, mErr := proto.Marshal(notif); mErr == nil {
			_ = target.Entity.Push(data)
		}
		return messageEvent(fmt.Sprintf("Granted %d XP to %s.", amount, target.CharName)), nil

	case "money":
		target.Currency += amount
		if s.charSaver != nil && target.CharacterID > 0 {
			if cErr := s.charSaver.SaveCurrency(ctx, target.CharacterID, target.Currency); cErr != nil {
				s.logger.Warn("handleGrant: SaveCurrency failed", zap.Error(cErr))
			}
		}
		// Notify target.
		notif := messageEvent(fmt.Sprintf("You have been granted %d gold by %s.", amount, sess.CharName))
		if data, mErr := proto.Marshal(notif); mErr == nil {
			_ = target.Entity.Push(data)
		}
		return messageEvent(fmt.Sprintf("Granted %d gold to %s.", amount, target.CharName)), nil

	case "heropoint":
		target.HeroPoints++
		if target.CharacterID > 0 && s.charSaver != nil {
			if hpErr := s.charSaver.SaveHeroPoints(ctx, target.CharacterID, target.HeroPoints); hpErr != nil {
				s.logger.Warn("handleGrant: SaveHeroPoints failed", zap.Error(hpErr))
			}
		}
		// Notify target.
		notif := messageEvent(fmt.Sprintf("You have been granted a hero point by %s. You now have %d.", sess.CharName, target.HeroPoints))
		if data, mErr := proto.Marshal(notif); mErr == nil {
			_ = target.Entity.Push(data)
		}
		return messageEvent(fmt.Sprintf("Granted 1 hero point to %s. They now have %d.", target.CharName, target.HeroPoints)), nil

	default:
		return errorEvent(fmt.Sprintf("unknown grant type %q: use 'xp', 'money', or 'heropoint'", req.GrantType)), nil
	}
}

// climbDCForExit returns the effective climb DC for an exit.
// Returns 0 if the exit is not climbable (no explicit DC and no terrain default).
//
// Precondition: exit and terrain are provided.
// Postcondition: Returns DC >= 0; 0 means the exit is not climbable.
func climbDCForExit(exit world.Exit, terrain string) int {
	if exit.ClimbDC > 0 {
		return exit.ClimbDC
	}
	switch terrain {
	case "rubble":
		return 12
	case "cliff":
		return 20
	case "wall":
		return 15
	case "sewer":
		return 10
	}
	return 0
}

// handleClimb processes a ClimbRequest from the player.
//
// Precondition: uid is a valid connected player session; req.Direction is non-empty.
// Postcondition: Player moves via climbable exit on success; fall damage applied on critical failure.
func (s *GameServiceServer) handleClimb(uid string, req *gamev1.ClimbRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if req.GetDirection() == "" {
		return messageEvent("Climb which direction?"), nil
	}

	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("Room not found."), nil
	}

	dir := world.Direction(req.GetDirection())
	exit, found := room.ExitForDirection(dir)
	if !found {
		return messageEvent("There is nothing to climb here."), nil
	}

	dc := climbDCForExit(exit, room.Terrain)
	if dc == 0 {
		return messageEvent("There is nothing to climb here."), nil
	}

	// Spend AP if in combat.
	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return messageEvent("Not enough action points to climb."), nil
		}
		if err := s.combatH.SpendAP(uid, 2); err != nil {
			return messageEvent("Not enough action points to climb."), nil
		}
	}

	// Roll muscle check.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "climb"

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		if _, moveErr := s.worldH.MoveWithContext(uid, dir); moveErr != nil {
			return messageEvent(fmt.Sprintf(
				"You climb successfully (rolled %d+%d=%d vs DC %d) but cannot proceed: %s.",
				roll, bonus, total, dc, moveErr.Error(),
			)), nil
		}
		destRoom, _ := s.world.GetRoom(exit.TargetRoom)
		destTitle := exit.TargetRoom
		if destRoom != nil {
			destTitle = destRoom.Title
		}
		return messageEvent(fmt.Sprintf(
			"You climb successfully (rolled %d+%d=%d vs DC %d). You arrive at %s.",
			roll, bonus, total, dc, destTitle,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You fail to gain purchase on the climb (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		numDice := exit.Height / 10
		if numDice < 1 {
			numDice = 1
		}
		expr := fmt.Sprintf("%dd6", numDice)
		dmgResult, _ := s.dice.RollExpr(expr)
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		s.checkNonCombatDeath(uid, sess) // REQ-BUG100-1
		msg := fmt.Sprintf(
			"You fall! (rolled %d+%d=%d vs DC %d) Taking %d falling damage.",
			roll, bonus, total, dc, dmg,
		)
		if inCombat && sess.Conditions != nil && s.condRegistry != nil {
			if def, ok := s.condRegistry.Get("prone"); ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
				msg += " You are knocked prone."
			}
		}
		return messageEvent(msg), nil
	}
}

// swimDCForExit returns the effective swim DC for an exit given the room terrain.
//
// Precondition: exit and terrain are valid values from the world model.
// Postcondition: Returns exit.SwimDC if non-zero; otherwise returns terrain default; returns 0 if not swimmable.
func swimDCForExit(exit world.Exit, terrain string) int {
	if exit.SwimDC > 0 {
		return exit.SwimDC
	}
	switch terrain {
	case "sewer":
		return 10
	case "river":
		return 15
	case "ocean":
		return 20
	case "flooded":
		return 12
	}
	return 0
}

// handleSwim processes a SwimRequest from the player.
//
// Precondition: uid is a valid connected player session; req.Direction is non-empty.
// Postcondition: Player moves on success; submerged condition applied and 1d6 damage on critical failure.
func (s *GameServiceServer) handleSwim(uid string, req *gamev1.SwimRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if req.GetDirection() == "" {
		return messageEvent("Swim which direction?"), nil
	}

	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("Room not found."), nil
	}

	dir := world.Direction(req.GetDirection())
	exit, found := room.ExitForDirection(dir)
	dc := 0
	if found {
		dc = swimDCForExit(exit, room.Terrain)
	}

	isSubmerged := sess.Conditions != nil && sess.Conditions.Has("submerged")

	if dc == 0 && !isSubmerged {
		return messageEvent("There is no water here."), nil
	}
	if dc == 0 && isSubmerged {
		dc = 12
	}

	// Spend AP if in combat.
	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return messageEvent("Not enough action points to swim."), nil
		}
		if err := s.combatH.SpendAP(uid, 2); err != nil {
			return messageEvent("Not enough action points to swim."), nil
		}
	}

	// Roll muscle check.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "swim"

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		// Clear submerged if present.
		if isSubmerged && sess.Conditions != nil {
			sess.Conditions.Remove(uid, "submerged")
		}
		if found {
			if _, moveErr := s.worldH.MoveWithContext(uid, dir); moveErr != nil {
				return messageEvent(fmt.Sprintf(
					"You swim successfully (rolled %d+%d=%d vs DC %d) but cannot proceed: %s.",
					roll, bonus, total, dc, moveErr.Error(),
				)), nil
			}
		}
		if isSubmerged {
			return messageEvent(fmt.Sprintf(
				"You surface! (rolled %d+%d=%d vs DC %d)", roll, bonus, total, dc,
			)), nil
		}
		destRoom, _ := s.world.GetRoom(exit.TargetRoom)
		destTitle := exit.TargetRoom
		if destRoom != nil {
			destTitle = destRoom.Title
		}
		return messageEvent(fmt.Sprintf(
			"You swim through (rolled %d+%d=%d vs DC %d). You arrive at %s.",
			roll, bonus, total, dc, destTitle,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You struggle against the current (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		dmgResult, _ := s.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		s.checkNonCombatDeath(uid, sess) // REQ-BUG100-1
		msg := fmt.Sprintf(
			"You are pulled under! (rolled %d+%d=%d vs DC %d) Taking %d drowning damage.",
			roll, bonus, total, dc, dmg,
		)
		if s.condRegistry != nil {
			if def, condOk := s.condRegistry.Get("submerged"); condOk && sess.Conditions != nil {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
				msg += " You are submerged."
			}
		}
		return messageEvent(msg), nil
	}
}

// handleDelay banks remaining AP for next round at cost of -2 AC.
//
// Precondition: uid is a valid player in active combat with >= 1 AP remaining.
// Postcondition: 1 AP spent; remaining AP banked (capped at 2) in sess.BankedAP;
//
//	all AP zeroed; player combatant ACMod reduced by 2.
func (s *GameServiceServer) handleDelay(uid string, _ *gamev1.DelayRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if sess.Status != statusInCombat {
		return messageEvent("You cannot delay outside of combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}

	remaining := s.combatH.RemainingAP(uid)
	if remaining < 1 {
		return messageEvent("Not enough AP to delay."), nil
	}

	// Step 1: spend 1 AP for the delay action itself.
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return messageEvent("Not enough AP to delay."), nil
	}

	// Step 2: bank remaining AP after cost, capped at 2.
	postCost := remaining - 1
	banked := postCost
	if banked > 2 {
		banked = 2
	}
	sess.BankedAP = banked

	// Step 3: zero all remaining AP for this round.
	s.combatH.SpendAllAP(uid)

	// Step 4: apply -2 AC penalty to player's combatant for this round.
	s.combatH.ApplyPlayerACMod(uid, -2)

	return messageEvent(fmt.Sprintf(
		"You delay, banking %d AP for next round. You are exposed (-2 AC).", banked,
	)), nil
}

// handleAid queues an Aid action for the player targeting the named ally.
//
// Precondition: uid must identify a valid player session.
// Postcondition: Out-of-combat returns informational message; empty target returns
// informational message; in-combat with valid target delegates to CombatHandler.Aid.
func (s *GameServiceServer) handleAid(uid string, req *gamev1.AidRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if sess.Status != statusInCombat {
		return messageEvent("Aid is only valid in combat."), nil
	}

	target := req.GetTarget()
	if target == "" {
		return messageEvent("Please specify an ally name: aid <ally>"), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}

	events, err := s.combatH.Aid(uid, target)
	if err != nil {
		return messageEvent(err.Error()), nil
	}
	if len(events) == 0 {
		return nil, nil
	}
	for _, evt := range events[1:] {
		s.broadcastCombatEvent(sess.RoomID, uid, evt)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CombatEvent{CombatEvent: events[0]},
	}, nil
}

// handleJoin joins an active combat in the player's pending room.
//
// Precondition: uid must identify an existing player session.
// Postcondition: if PendingCombatJoin is non-empty and the combat exists, the player is
//
//	inserted into the combat, sess.Status is set to statusInCombat, and PendingCombatJoin is cleared.
func (s *GameServiceServer) handleJoin(uid string, _ *gamev1.JoinRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	s.combatH.combatMu.Lock()
	roomID := sess.PendingCombatJoin
	if roomID == "" {
		s.combatH.combatMu.Unlock()
		return messageEvent("No combat to join."), nil
	}

	_, exists := s.combatH.engine.GetCombat(roomID)
	if !exists {
		sess.PendingCombatJoin = ""
		s.combatH.combatMu.Unlock()
		return messageEvent("The combat has ended."), nil
	}
	s.combatH.combatMu.Unlock()

	// Build player combatant outside combatMu (RollInitiative modifies only the struct).
	playerCbt := buildPlayerCombatant(sess, s.combatH)

	// Roll initiative for just this player against existing combatants.
	if s.dice != nil {
		roll := s.dice.Src().Intn(20) + 1
		playerCbt.Initiative = roll + playerCbt.DexMod
	}

	// AddCombatant acquires engine.mu internally.
	if err := s.combatH.engine.AddCombatant(roomID, playerCbt); err != nil {
		return errorEvent(fmt.Sprintf("Could not join combat: %v", err)), nil
	}

	s.combatH.combatMu.Lock()
	sess.Status = statusInCombat
	sess.PendingCombatJoin = ""
	s.combatH.combatMu.Unlock()

	s.broadcastMessage(roomID, uid, &gamev1.MessageEvent{
		Content: fmt.Sprintf("%s joins the combat!", sess.CharName),
	})

	return messageEvent(fmt.Sprintf("You join the combat in %s!", roomID)), nil
}

// handleDecline declines a pending combat join invitation.
//
// Precondition: uid must identify an existing player session.
// Postcondition: if PendingCombatJoin is non-empty it is cleared and a watch message is returned;
//
//	otherwise a "Nothing to decline." message is returned.
func (s *GameServiceServer) handleDecline(uid string, _ *gamev1.DeclineRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	s.combatH.combatMu.Lock()
	defer s.combatH.combatMu.Unlock()

	if sess.PendingCombatJoin == "" {
		return messageEvent("Nothing to decline."), nil
	}

	sess.PendingCombatJoin = ""
	return messageEvent("You stay back and watch the combat from a distance."), nil
}

// notifyCombatJoinIfEligible sets PendingCombatJoin and sends a join prompt if
// the player has just entered a room with active combat and is eligible to join.
//
// Precondition: sess != nil; called after a successful move; combatH.combatMu NOT held.
// Postcondition: if the destination room has active combat and the player is not already
//
//	a combatant, sess.PendingCombatJoin is set to newRoomID and a join prompt is pushed.
func (s *GameServiceServer) notifyCombatJoinIfEligible(sess *session.PlayerSession, newRoomID string) {
	if s.combatH == nil {
		return
	}
	if sess.Status == statusInCombat {
		return
	}

	s.combatH.combatMu.Lock()
	defer s.combatH.combatMu.Unlock()

	cbt, exists := s.combatH.engine.GetCombat(newRoomID)
	if !exists {
		return
	}

	// Ensure the player is not already registered as a combatant.
	for _, c := range cbt.Combatants {
		if c.ID == sess.UID {
			return
		}
	}

	sess.PendingCombatJoin = newRoomID

	if sess.Entity != nil {
		evt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: "Active combat in progress. Join the fight? (join / decline)",
				},
			},
		}
		if data, err := proto.Marshal(evt); err == nil {
			_ = sess.Entity.Push(data)
		}
	}
}

// clearPendingJoinForRoom clears PendingCombatJoin for all players waiting to join
// the combat in roomID (called when that combat ends).
//
// Precondition: roomID is non-empty.
// Postcondition: no player session has PendingCombatJoin == roomID.
func (s *GameServiceServer) clearPendingJoinForRoom(roomID string) {
	for _, sess := range s.sessions.AllPlayers() {
		if sess.PendingCombatJoin == roomID {
			sess.PendingCombatJoin = ""
			if sess.Entity != nil {
				evt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{
							Content: "The combat has ended.",
						},
					},
				}
				if data, err := proto.Marshal(evt); err == nil {
					_ = sess.Entity.Push(data)
				}
			}
		}
	}
}

// triggerPassiveTechsForRoom fires all passive innate technologies for every player
// currently in roomID.
//
// Precondition: roomID may be any string; nil or missing sessions are skipped silently.
// Postcondition: Each player with a passive innate tech receives a push event on their
// BridgeEntity. Players with nil Entity or techs absent from the registry are skipped
// without error. Innate tech use counts are never decremented.
func (s *GameServiceServer) triggerPassiveTechsForRoom(roomID string) {
	if s.techRegistry == nil {
		return
	}
	players := s.sessions.PlayersInRoomDetails(roomID)
	for _, sess := range players {
		if sess.Entity == nil {
			continue
		}
		// NOTE: sess.InnateTechs is read without a per-session lock (consistent with
		// how other handleX methods read session fields). A full session-level mutex
		// would require broader refactoring outside the scope of this change.
		for techID := range sess.InnateTechs {
			def, ok := s.techRegistry.Get(techID)
			if !ok {
				s.logger.Debug("passive tech not in registry", zap.String("techID", techID))
				continue
			}
			if !def.Passive {
				continue
			}
			evt, err := s.activateTechWithEffects(sess, sess.UID, techID, "", "", s, 0, 0)
			if err != nil {
				s.logger.Warn("passive tech activation error",
					zap.String("uid", sess.UID),
					zap.String("techID", techID),
					zap.Error(err))
				continue
			}
			if evt == nil {
				continue
			}
			data, marshalErr := proto.Marshal(evt)
			if marshalErr != nil {
				s.logger.Warn("passive tech marshal error",
					zap.String("uid", sess.UID),
					zap.Error(marshalErr))
				continue
			}
			if pushErr := sess.Entity.Push(data); pushErr != nil {
				s.logger.Warn("error pushing passive tech event",
					zap.String("uid", sess.UID),
					zap.Error(pushErr))
			}
		}
	}
}

// CreaturesInRoom implements RoomQuerier for GameServiceServer.
// It returns all players and NPCs currently in roomID.
// The sensing player (sensingUID) is returned as CreatureInfo{Name: "you"}.
//
// Precondition: roomID and sensingUID are non-empty strings.
// Postcondition: Returns one entry per creature; sensing player entry has Name="you".
func (s *GameServiceServer) CreaturesInRoom(roomID, sensingUID string) []CreatureInfo {
	var result []CreatureInfo

	// Add NPC instances.
	if s.npcH != nil {
		for _, inst := range s.npcH.InstancesInRoom(roomID) {
			result = append(result, CreatureInfo{Name: inst.Name(), Hidden: false})
		}
	}

	// Add players.
	for _, sess := range s.sessions.PlayersInRoomDetails(roomID) {
		if sess.UID == sensingUID {
			result = append(result, CreatureInfo{Name: "you", Hidden: false})
		} else {
			result = append(result, CreatureInfo{Name: sess.CharName, Hidden: false})
		}
	}

	return result
}

// hydrateEquipmentNames sets the Name field of each SlottedItem in eq.Armor and
// eq.Accessories to the ArmorDef.Name from reg, if the definition is found.
//
// Precondition: eq and reg must be non-nil.
// Postcondition: Each slot whose ItemDefID is registered in reg has its Name
// updated to the human-readable ArmorDef.Name; unregistered items are unchanged.
func hydrateEquipmentNames(eq *inventory.Equipment, reg *inventory.Registry) {
	if eq == nil || reg == nil {
		return
	}
	for slot, item := range eq.Armor {
		if item == nil {
			continue
		}
		if def, ok := reg.Armor(item.ItemDefID); ok {
			eq.Armor[slot].Name = def.Name
		}
	}
	for slot, item := range eq.Accessories {
		if item == nil {
			continue
		}
		if def, ok := reg.Armor(item.ItemDefID); ok {
			eq.Accessories[slot].Name = def.Name
		}
	}
}

// handleExplore sets, clears, or queries the player's exploration mode.
//
// Precondition: uid is a connected player UID; req is non-nil.
// Postcondition: sess.ExploreMode is updated; immediate hooks fire for active_sensors and case_it.
func (s *GameServiceServer) handleExplore(uid string, req *gamev1.ExploreRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Blocked in combat (REQ-EXP-4).
	if sess.Status == statusInCombat {
		return errorEvent("You cannot change exploration mode while in combat."), nil
	}

	// Query mode: no argument (REQ-EXP-1 display).
	if req.Mode == "" {
		if sess.ExploreMode == "" {
			return messageEvent("No active exploration mode."), nil
		}
		msg := "Exploration mode: " + exploreDisplayName(sess.ExploreMode)
		if sess.ExploreMode == session.ExploreModeShadow {
			ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget)
			if ally != nil && ally.RoomID == sess.RoomID {
				msg += " [shadowing " + ally.CharName + "]"
			} else if ally != nil {
				msg += " [shadowing " + ally.CharName + " — not present]"
			}
		}
		return messageEvent(msg), nil
	}

	// Clear mode (REQ-EXP-2).
	if req.Mode == "off" {
		sess.ExploreMode = ""
		sess.ExploreShadowTarget = 0
		return messageEvent("Exploration mode cleared."), nil
	}

	// Validate shadow target (REQ-EXP-6).
	if req.Mode == session.ExploreModeShadow {
		if req.ShadowTarget == "" {
			return errorEvent("Usage: explore shadow <player name>"), nil
		}
		target := s.sessions.GetPlayerByCharNameCI(req.ShadowTarget)
		if target == nil || target.RoomID != sess.RoomID {
			return errorEvent(fmt.Sprintf("No player named %q is in this room.", req.ShadowTarget)), nil
		}
		sess.ExploreMode = session.ExploreModeShadow
		sess.ExploreShadowTarget = target.CharacterID
		return messageEvent(fmt.Sprintf("Exploration mode: Shadow [shadowing %s]", target.CharName)), nil
	}

	// Validate mode ID.
	validModes := map[string]string{
		session.ExploreModeLayLow:        "Lay Low",
		session.ExploreModeHoldGround:    "Hold Ground",
		session.ExploreModeActiveSensors: "Active Sensors",
		session.ExploreModeCaseIt:        "Case It",
		session.ExploreModeRunPoint:      "Run Point",
		session.ExploreModePokeAround:    "Poke Around",
	}
	displayName, valid := validModes[req.Mode]
	if !valid {
		return errorEvent(fmt.Sprintf("Unknown exploration mode %q. Valid modes: lay_low, hold_ground, active_sensors, case_it, run_point, shadow <ally>, poke_around.", req.Mode)), nil
	}

	// Set mode — replaces any existing mode (REQ-EXP-3).
	sess.ExploreMode = req.Mode
	sess.ExploreShadowTarget = 0

	// Fire immediate room-entry hooks for active_sensors and case_it (REQ-EXP-5).
	if req.Mode == session.ExploreModeActiveSensors || req.Mode == session.ExploreModeCaseIt {
		if room, ok := s.world.GetRoom(sess.RoomID); ok {
			msgs := s.applyExploreModeOnEntry(uid, sess, room)
			for _, msg := range msgs {
				s.pushMessageToUID(uid, msg)
			}
		}
	}

	return messageEvent(fmt.Sprintf("Exploration mode set: %s.", displayName)), nil
}

// exploreDisplayName returns the human-readable display name for a mode ID.
func exploreDisplayName(mode string) string {
	names := map[string]string{
		session.ExploreModeLayLow:        "Lay Low",
		session.ExploreModeHoldGround:    "Hold Ground",
		session.ExploreModeActiveSensors: "Active Sensors",
		session.ExploreModeCaseIt:        "Case It",
		session.ExploreModeRunPoint:      "Run Point",
		session.ExploreModeShadow:        "Shadow",
		session.ExploreModePokeAround:    "Poke Around",
	}
	if n, ok := names[mode]; ok {
		return n
	}
	return mode
}

package gameserver

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"

	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"

	lua "github.com/yuin/gopher-lua"
)

// errQuit is returned by handleQuit to signal the command loop to stop cleanly.
var errQuit = fmt.Errorf("quit")

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
	world        *world.Manager
	sessions     *session.Manager
	commands     *command.Registry
	worldH       *WorldHandler
	chatH        *ChatHandler
	charSaver    CharacterSaver
	dice         *dice.Roller
	npcH         *NPCHandler
	npcMgr       *npc.Manager
	combatH      *CombatHandler
	scriptMgr    *scripting.Manager
	respawnMgr   *npc.RespawnManager
	floorMgr     *inventory.FloorManager
	roomEquipMgr *inventory.RoomEquipmentManager
	automapRepo  *postgres.AutomapRepository
	invRegistry  *inventory.Registry
	accountAdmin AccountAdmin
	clock        *GameClock
	logger       *zap.Logger
	jobRegistry         *ruleset.JobRegistry
	condRegistry        *condition.Registry
	loadoutsDir         string
	allSkills                   []*ruleset.Skill
	characterSkillsRepo         CharacterSkillsRepository
	characterProficienciesRepo  CharacterProficienciesRepository
	allFeats                   []*ruleset.Feat
	featRegistry               *ruleset.FeatRegistry
	characterFeatsRepo         CharacterFeatsGetter
	allClassFeatures           []*ruleset.ClassFeature
	classFeatureRegistry       *ruleset.ClassFeatureRegistry
	characterClassFeaturesRepo CharacterClassFeaturesGetter
	featureChoicesRepo         CharacterFeatureChoicesRepository
	charAbilityBoostsRepo      postgres.CharacterAbilityBoostsRepository
	archetypes                 map[string]*ruleset.Archetype
	regions                    map[string]*ruleset.Region
	xpSvc                      *xp.Service
	progressRepo               ProgressRepository
	actionH                    *ActionHandler
}

// NewGameServiceServer creates a GameServiceServer with the given dependencies.
//
// Precondition: worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, diceRoller, and logger must be non-nil.
// charSaver may be nil (character state will not be persisted on disconnect).
// respawnMgr may be nil (respawn functionality will be disabled).
// floorMgr may be nil (inventory get/drop will return errors).
// roomEquipMgr may be nil (room equipment will not be shown in look).
// invRegistry may be nil (item name resolution will fall back to ItemDefID).
// clock may be nil (time-of-day events will not be broadcast to sessions).
// jobRegistry may be nil (team affinity effects will not be applied on wear).
// condRegistry may be nil (cross-team condition effects will be skipped).
// loadoutsDir is the path to the archetype loadout YAML directory; empty disables starting inventory grants.
// allSkills may be nil (skill backfill on session startup will be skipped).
// characterSkillsRepo may be nil (skill backfill on session startup will be skipped).
// characterProficienciesRepo may be nil (proficiency backfill on session startup will be skipped).
// allFeats may be nil (feat commands will return a not-available message).
// featRegistry may be nil (feat commands will return a not-available message).
// characterFeatsRepo may be nil (feat commands will return a not-available message).
// charAbilityBoostsRepo may be nil (ability boost persistence will be skipped).
// archetypes may be nil (ability boost archetype lookup will be skipped).
// regions may be nil (ability boost region lookup will be skipped).
// Postcondition: Returns a fully initialised GameServiceServer.
func NewGameServiceServer(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	cmdRegistry *command.Registry,
	worldHandler *WorldHandler,
	chatHandler *ChatHandler,
	logger *zap.Logger,
	charSaver CharacterSaver,
	diceRoller *dice.Roller,
	npcHandler *NPCHandler,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	scriptMgr *scripting.Manager,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	roomEquipMgr *inventory.RoomEquipmentManager,
	automapRepo *postgres.AutomapRepository,
	invRegistry *inventory.Registry,
	accountAdmin AccountAdmin,
	clock *GameClock,
	jobRegistry *ruleset.JobRegistry,
	condRegistry *condition.Registry,
	loadoutsDir string,
	allSkills []*ruleset.Skill,
	characterSkillsRepo CharacterSkillsRepository,
	characterProficienciesRepo CharacterProficienciesRepository,
	allFeats []*ruleset.Feat,
	featRegistry *ruleset.FeatRegistry,
	characterFeatsRepo CharacterFeatsGetter,
	allClassFeatures []*ruleset.ClassFeature,
	classFeatureRegistry *ruleset.ClassFeatureRegistry,
	characterClassFeaturesRepo CharacterClassFeaturesGetter,
	featureChoicesRepo CharacterFeatureChoicesRepository,
	charAbilityBoostsRepo postgres.CharacterAbilityBoostsRepository,
	archetypes map[string]*ruleset.Archetype,
	regions map[string]*ruleset.Region,
	actionH *ActionHandler,
) *GameServiceServer {
	s := &GameServiceServer{
		world:               worldMgr,
		sessions:            sessMgr,
		commands:            cmdRegistry,
		worldH:              worldHandler,
		chatH:               chatHandler,
		charSaver:           charSaver,
		dice:                diceRoller,
		npcH:                npcHandler,
		npcMgr:              npcMgr,
		combatH:             combatHandler,
		scriptMgr:           scriptMgr,
		respawnMgr:          respawnMgr,
		floorMgr:            floorMgr,
		roomEquipMgr:        roomEquipMgr,
		automapRepo:         automapRepo,
		invRegistry:         invRegistry,
		accountAdmin:        accountAdmin,
		clock:               clock,
		logger:              logger,
		jobRegistry:         jobRegistry,
		condRegistry:        condRegistry,
		loadoutsDir:         loadoutsDir,
		allSkills:                  allSkills,
		characterSkillsRepo:        characterSkillsRepo,
		characterProficienciesRepo: characterProficienciesRepo,
		allFeats:                   allFeats,
		featRegistry:               featRegistry,
		characterFeatsRepo:         characterFeatsRepo,
		allClassFeatures:           allClassFeatures,
		classFeatureRegistry:       classFeatureRegistry,
		characterClassFeaturesRepo: characterClassFeaturesRepo,
		featureChoicesRepo:         featureChoicesRepo,
		charAbilityBoostsRepo:      charAbilityBoostsRepo,
		archetypes:                 archetypes,
		regions:                    regions,
		actionH:                    actionH,
	}
	if s.combatH != nil {
		s.combatH.SetOnCombatEnd(func(roomID string) {
			sessions := s.sessions.PlayersInRoomDetails(roomID)
			for _, sess := range sessions {
				if sess.Conditions != nil {
					sess.Conditions.ClearEncounter()
				}
			}
			// Push updated room view so "fighting X" labels clear immediately.
			s.pushRoomViewToAllInRoom(roomID)
		})
		s.worldH.SetCombatHandler(s.combatH)
	}
	return s
}

// SetProgressRepo registers the CharacterProgressRepository used to load pending boosts at login.
//
// Precondition: repo must be non-nil.
// Postcondition: PendingBoosts are loaded from the DB on each player login.
func (s *GameServiceServer) SetProgressRepo(repo ProgressRepository) {
	s.progressRepo = repo
}

// SetXPService registers the XP service used to award experience.
//
// Precondition: svc must be non-nil.
// Postcondition: XP is awarded at kill, room discovery, and skill check sites.
func (s *GameServiceServer) SetXPService(svc *xp.Service) {
	s.xpSvc = svc
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
	defaultCombatAction := "pass"
	if dbChar != nil && dbChar.DefaultCombatAction != "" {
		defaultCombatAction = dbChar.DefaultCombatAction
	}
	genderVal := ""
	if dbChar != nil {
		genderVal = dbChar.Gender
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
	})
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	defer s.cleanupPlayer(uid, username)

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
		discovered, loadErr := s.automapRepo.LoadAll(stream.Context(), characterID)
		if loadErr != nil {
			s.logger.Warn("loading automap", zap.Error(loadErr))
		} else {
			sess.AutomapCache = discovered
		}
	}
	// Record spawn room discovery.
	if spawnRoom != nil {
		zID := spawnRoom.ZoneID
		if sess.AutomapCache[zID] == nil {
			sess.AutomapCache[zID] = make(map[string]bool)
		}
		if !sess.AutomapCache[zID][spawnRoom.ID] {
			sess.AutomapCache[zID][spawnRoom.ID] = true
			if s.automapRepo != nil {
				if err := s.automapRepo.Insert(stream.Context(), characterID, zID, spawnRoom.ID); err != nil {
					s.logger.Warn("persisting spawn room discovery", zap.Error(err))
				}
			}
		}
	}

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
		}

		// Compute defense stats from equipped armor (AC, resistances, weaknesses).
		{
			loginDexMod := (sess.Abilities.Quickness - 10) / 2
			loginDef := sess.Equipment.ComputedDefenses(s.invRegistry, loginDexMod)
			sess.Resistances = loginDef.Resistances
			sess.Weaknesses = loginDef.Weaknesses
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
					archetype := joinReq.Archetype
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
				chosen, promptErr := s.promptFeatureChoice(stream, id, cf.Choices)
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
					chosen, promptErr := s.promptFeatureChoice(stream, id, f.Choices)
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
		sess.FavoredTarget = sess.FeatureChoices["predators_eye"]["favored_target"]
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
					chosen, promptErr := s.promptFeatureChoice(stream, "archetype_boost", choices)
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
					chosen, promptErr := s.promptFeatureChoice(stream, "region_boost", choices)
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

	// Subscribe to game clock ticks for this session (nil-safe: clock may be nil).
	var clockCh chan GameHour
	if s.clock != nil {
		clockCh = make(chan GameHour, 2)
		s.clock.Subscribe(clockCh)
		defer s.clock.Unsubscribe(clockCh)
	}

	// Step 3: Spawn goroutine to forward entity events to stream
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.forwardEvents(ctx, sess.Entity, stream)
	}()

	// Spawn goroutine to forward game clock ticks to stream as TimeOfDayEvents.
	// When the period changes (e.g., Dawn→Morning), also push an updated RoomView
	// through the entity channel so the room display reflects the new flavor text.
	// Only launched when a clock is configured.
	if clockCh != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastPeriod := TimePeriod(roomView.GetPeriod())
			for {
				select {
				case h, ok := <-clockCh:
					if !ok {
						return
					}
					period := h.Period()
					evt := &gamev1.ServerEvent{
						Payload: &gamev1.ServerEvent_TimeOfDay{
							TimeOfDay: &gamev1.TimeOfDayEvent{
								Hour:   int32(h),
								Period: string(period),
							},
						},
					}
					if err := stream.Send(evt); err != nil {
						return
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
			case <-ctx.Done():
				return
			}
		}
	}()

	// Step 4: Main command loop
	err = s.commandLoop(ctx, uid, stream)

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
		return s.handleMap(uid)
	case *gamev1.ClientMessage_SkillsRequest:
		return s.handleSkills(uid)
	case *gamev1.ClientMessage_FeatsRequest:
		return s.handleFeats(uid)
	case *gamev1.ClientMessage_ClassFeaturesRequest:
		return s.handleClassFeatures(uid)
	case *gamev1.ClientMessage_InteractRequest:
		return s.handleInteract(uid, p.InteractRequest.InstanceId)
	case *gamev1.ClientMessage_UseRequest:
		return s.handleUse(uid, p.UseRequest.FeatId)
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
	case *gamev1.ClientMessage_Disarm:
		return s.handleDisarm(uid, p.Disarm)
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
	default:
		return nil, fmt.Errorf("unknown message type")
	}
}

func (s *GameServiceServer) handleMove(uid string, req *gamev1.MoveRequest) (*gamev1.ServerEvent, error) {
	dir := world.Direction(req.Direction)

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

	// Broadcast arrival in new room
	s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
		Player:    sess.CharName,
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
		Direction: string(dir.Opposite()),
	})

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

	// zone_awareness: notify the player when entering a difficult terrain room,
	// unless they possess the zone_awareness passive feat.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		if newRoom.Properties["terrain"] == "difficult" && !sess.PassiveFeats["zone_awareness"] {
			terrainEvt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{
						Content: "The ground here is difficult terrain — your movement feels sluggish.",
						Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
					},
				},
			}
			if data, marshalErr := proto.Marshal(terrainEvt); marshalErr == nil {
				if pushErr := sess.Entity.Push(data); pushErr != nil {
					s.logger.Warn("pushing difficult terrain message to player entity",
						zap.String("uid", uid),
						zap.Error(pushErr),
					)
				}
			}
		}
	}

	// Record automap discovery for the new room.
	if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
		zID := newRoom.ZoneID
		if sess.AutomapCache[zID] == nil {
			sess.AutomapCache[zID] = make(map[string]bool)
		}
		if !sess.AutomapCache[zID][newRoom.ID] {
			sess.AutomapCache[zID][newRoom.ID] = true
			if s.automapRepo != nil {
				if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, zID, newRoom.ID); err != nil {
					s.logger.Warn("persisting map discovery", zap.Error(err))
				}
			}
			// Award room discovery XP for newly discovered rooms.
			if s.xpSvc != nil {
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
					}
				}
			}
		}
	}

	if s.npcH != nil {
		for _, inst := range s.npcH.InstancesInRoom(result.View.RoomId) {
			if taunt, ok := inst.TryTaunt(time.Now()); ok {
				s.broadcastMessage(result.View.RoomId, "", &gamev1.MessageEvent{
					Content: fmt.Sprintf("%s says \"%s\"", inst.Name(), taunt),
				})
				break
			}
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
			if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
				s.logger.Warn("awarding skill check XP", zap.String("uid", uid), zap.Error(xpErr))
			} else {
				msgs = append(msgs, xpMsgs...)
				if len(xpMsgs) > 0 {
					s.pushHPUpdate(uid, sess)
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
				if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
					s.logger.Warn("awarding NPC skill check XP", zap.String("uid", uid), zap.Error(xpErr))
				} else {
					msgs = append(msgs, xpMsgs...)
					if len(xpMsgs) > 0 {
						s.pushHPUpdate(uid, sess)
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

	if s.roomEquipMgr != nil {
		if sess, ok := s.sessions.GetPlayer(uid); ok {
			for _, eq := range s.roomEquipMgr.EquipmentInRoom(sess.RoomID) {
				name := eq.ItemDefID
				if s.invRegistry != nil {
					if def, ok := s.invRegistry.Item(eq.ItemDefID); ok {
						name = def.Name
					}
				}
				view.Equipment = append(view.Equipment, &gamev1.RoomEquipmentItem{
					InstanceId: eq.InstanceID,
					Name:       name,
					Quantity:   1,
					Immovable:  eq.Immovable,
					Usable:     eq.Script != "",
				})
			}
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
				},
			},
		}, nil
	}

	// Fall back to NPC examine.
	if s.npcH != nil {
		view, err := s.npcH.Examine(uid, req.Target)
		if err == nil {
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
			_ = sess.Entity.Push(data)
		}
	}
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
	events, err := s.combatH.Flee(uid)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if ok {
		s.broadcastCombatEvent(sess.RoomID, uid, events[0])
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

// cleanupPlayer removes a player from the session manager, persists character state, and broadcasts departure.
//
// Precondition: uid must be non-empty.
// Postcondition: Player is removed from all tracking; character state is saved if charSaver is configured.
func (s *GameServiceServer) cleanupPlayer(uid, username string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	roomID := sess.RoomID
	characterID := sess.CharacterID
	currentHP := sess.CurrentHP
	charName := sess.CharName

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
			command.HandleEquip(sess, s.invRegistry, sl.Weapon+" main")
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
	result := command.HandleEquip(sess, s.invRegistry, arg)
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
	if inst.AIDomain == "" || aiReg == nil {
		return
	}
	planner, ok := aiReg.PlannerFor(inst.AIDomain)
	if !ok {
		return
	}
	ws := &ai.WorldState{
		NPC: &ai.NPCState{
			UID:        inst.ID,
			Name:       inst.Name(),
			Kind:       "npc",
			HP:         inst.CurrentHP,
			MaxHP:      inst.MaxHP,
			Perception: inst.Perception,
			ZoneID:     zoneID,
			RoomID:     inst.RoomID,
		},
		Room:       &ai.RoomState{ID: inst.RoomID, ZoneID: zoneID},
		Combatants: nil,
	}
	actions, err := planner.Plan(ws)
	if err != nil || len(actions) == 0 {
		return
	}
	for _, a := range actions {
		switch a.Action {
		case "move_random":
			s.npcPatrolRandom(inst)
		default:
			// idle/pass: no-op
		}
	}
	if taunt, ok := inst.TryTaunt(time.Now()); ok {
		s.broadcastMessage(inst.RoomID, "", &gamev1.MessageEvent{
			Content: fmt.Sprintf("%s says \"%s\"", inst.Name(), taunt),
		})
	}
}

// npcPatrolRandom moves the NPC to a random visible exit.
func (s *GameServiceServer) npcPatrolRandom(inst *npc.Instance) {
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
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
				name = def.Name
				kind = def.Kind
				weight = def.Weight
			}
		}
		items = append(items, &gamev1.InventoryItem{
			InstanceId: inst.InstanceID,
			Name:       name,
			Kind:       kind,
			Quantity:   int32(inst.Quantity),
			Weight:     weight * float64(inst.Quantity),
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
		Currency:    inventory.FormatRounds(sess.Currency),
		TotalRounds: int32(sess.Currency),
	}
	return &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_InventoryView{InventoryView: view}}, nil
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
	return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatRounds(sess.Currency))), nil
}

// handleLoadout displays or swaps weapon presets for the player.
//
// Precondition: uid must be a valid connected player with a non-nil LoadoutSet.
// Postcondition: Returns a ServerEvent with the loadout display or swap result.
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	return messageEvent(command.HandleLoadout(sess, req.GetArg())), nil
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
	return messageEvent(command.HandleUnequip(sess, req.GetSlot())), nil
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
	return messageEvent(command.HandleEquipment(sess)), nil
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
		Name:         sess.CharName,
		Level:        int32(sess.Level),
		CurrentHp:    int32(sess.CurrentHP),
		MaxHp:        int32(sess.MaxHP),
		Brutality:    int32(sess.Abilities.Brutality),
		Grit:         int32(sess.Abilities.Grit),
		Quickness:    int32(sess.Abilities.Quickness),
		Reasoning:    int32(sess.Abilities.Reasoning),
		Savvy:        int32(sess.Abilities.Savvy),
		Flair:        int32(sess.Abilities.Flair),
		Currency:     inventory.FormatRounds(sess.Currency),
		Gender:       sess.Gender,
	}

	// Job info from registry.
	if s.jobRegistry != nil {
		if job, ok := s.jobRegistry.Job(sess.Class); ok {
			view.Job = job.Name
			view.Archetype = job.Archetype
		} else {
			view.Job = sess.Class
		}
		view.Team = s.jobRegistry.TeamFor(sess.Class)
	} else {
		view.Job = sess.Class
	}

	// Defense stats (dex mod from Quickness).
	dexMod := (sess.Abilities.Quickness - 10) / 2
	def := sess.Equipment.ComputedDefenses(s.invRegistry, dexMod)
	view.AcBonus = int32(def.ACBonus)
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
				profRank := sess.Proficiencies[def.ProficiencyCategory]
				atkBonus := brutalityMod + combat.CombatProficiencyBonus(weaponLevel, profRank)
				view.MainHandAttackBonus = signedInt(atkBonus)
				view.MainHandDamage = weaponDamageString(def.DamageDice, brutalityMod, def.IsMelee())
			}
			if preset.OffHand != nil {
				def := preset.OffHand.Def
				view.OffHand = def.Name
				profRank := sess.Proficiencies[def.ProficiencyCategory]
				atkBonus := brutalityMod + combat.CombatProficiencyBonus(weaponLevel, profRank)
				view.OffHandAttackBonus = signedInt(atkBonus)
				view.OffHandDamage = weaponDamageString(def.DamageDice, brutalityMod, def.IsMelee())
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
				view.Feats = append(view.Feats, &gamev1.FeatEntry{
					FeatId:       f.ID,
					Name:         f.Name,
					Active:       f.Active,
					Description:  f.Description,
					ActivateText: f.ActivateText,
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
	if s.xpSvc != nil {
		cfg := s.xpSvc.Config()
		if sess.Level < cfg.LevelCap {
			view.XpToNext = int32(xp.XPToLevel(sess.Level+1, cfg.BaseXP))
		}
		// If at level cap, XpToNext remains 0 (zero-value).
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
	if sess.Role != "admin" {
		return errorEvent("permission denied: admin role required"), nil
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
	if sess.Role != "editor" && sess.Role != "admin" {
		return errorEvent("permission denied: editor role required"), nil
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
	if sess.Role != "admin" {
		return errorEvent("permission denied: admin role required"), nil
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
			return messageEvent(fmt.Sprintf("You pick up %s.", name)), nil
		}
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

// handleMap returns the automap tiles for the player's current zone.
//
// Precondition: uid must map to an active player session.
// Postcondition: Returns a ServerEvent with MapResponse containing discovered tiles.
func (s *GameServiceServer) handleMap(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
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
	discovered := sess.AutomapCache[zoneID]
	var tiles []*gamev1.MapTile
	for roomID := range discovered {
		r, ok := zone.Rooms[roomID]
		if !ok {
			continue
		}
		var exits []string
		for _, e := range r.Exits {
			exits = append(exits, string(e.Direction))
		}
		tiles = append(tiles, &gamev1.MapTile{
			RoomId:   r.ID,
			RoomName: r.Title,
			X:        int32(r.MapX),
			Y:        int32(r.MapY),
			Current:  r.ID == sess.RoomID,
			Exits:    exits,
		})
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Map{
			Map: &gamev1.MapResponse{Tiles: tiles},
		},
	}, nil
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
		entries = append(entries, &gamev1.FeatEntry{
			FeatId:       f.ID,
			Name:         f.Name,
			Category:     f.Category,
			Active:       f.Active,
			Description:  f.Description,
			ActivateText: f.ActivateText,
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

// handleUse activates an active feat or class feature for the player, or lists all available active abilities.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ServerEvent with UseResponse containing choices or an activation message.
func (s *GameServiceServer) handleUse(uid, abilityID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if s.characterFeatsRepo == nil && s.characterClassFeaturesRepo == nil {
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
			if ok && f.Active {
				active = append(active, &gamev1.FeatEntry{
					FeatId:       f.ID,
					Name:         f.Name,
					Category:     f.Category,
					Active:       f.Active,
					Description:  f.Description,
					ActivateText: f.ActivateText,
				})
			}
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
				})
			}
		}
	}

	if abilityID == "" {
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
				condID := f.ConditionID
				if condID != "" && sess.Conditions != nil && s.condRegistry != nil {
					if def, ok := s.condRegistry.Get(condID); ok {
						if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
							s.logger.Warn("failed to apply feat condition",
								zap.String("condition_id", condID),
								zap.Error(err),
							)
						}
					} else {
						s.logger.Warn("feat condition not found in registry",
							zap.String("condition_id", condID),
						)
					}
				}
				return &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_UseResponse{
						UseResponse: &gamev1.UseResponse{Message: f.ActivateText},
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
				if condID != "" && sess.Conditions != nil && s.condRegistry != nil {
					if def, ok := s.condRegistry.Get(condID); ok {
						if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
							s.logger.Warn("failed to apply class feature condition",
								zap.String("condition_id", condID),
								zap.Error(err),
							)
						}
					} else {
						s.logger.Warn("class feature condition not found in registry",
							zap.String("condition_id", condID),
						)
					}
				}
				return &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_UseResponse{
						UseResponse: &gamev1.UseResponse{Message: cf.ActivateText},
					},
				}, nil
			}
		}
	}
	return messageEvent(fmt.Sprintf("You don't have an active ability named %q.", abilityID)), nil
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
func (s *GameServiceServer) promptFeatureChoice(
	stream gamev1.GameService_SessionServer,
	featureID string,
	choices *ruleset.FeatureChoices,
) (string, error) {
	if len(choices.Options) == 0 {
		return "", fmt.Errorf("feature %s has empty Options slice", featureID)
	}

	var sb strings.Builder
	sb.WriteString(choices.Prompt)
	sb.WriteString("\n")
	for i, opt := range choices.Options {
		fmt.Fprintf(&sb, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(&sb, "Enter 1-%d:", len(choices.Options))

	if err := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: sb.String()},
		},
	}); err != nil {
		return "", fmt.Errorf("sending choice prompt for %s: %w", featureID, err)
	}

	msg, err := stream.Recv()
	if err != nil {
		return "", fmt.Errorf("receiving choice for %s: %w", featureID, err)
	}

	// Extract selection text from any client message type.
	// The commandLoop may send different message types depending on how the
	// player's input was parsed (e.g. a bare number "1" becomes a MoveRequest
	// because it is not a recognized command name).
	selText := ""
	switch {
	case msg.GetSay() != nil:
		selText = strings.TrimSpace(msg.GetSay().GetMessage())
	case msg.GetMove() != nil:
		selText = strings.TrimSpace(msg.GetMove().GetDirection())
	}

	n := 0
	idx := -1
	if _, scanErr := fmt.Sscanf(selText, "%d", &n); scanErr == nil && n >= 1 && n <= len(choices.Options) {
		idx = n - 1
	}

	if idx < 0 {
		s.logger.Warn("invalid feature choice selection",
			zap.String("feature", featureID),
			zap.String("input", selText),
		)
		if sendErr := stream.Send(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Content: "Invalid selection. You will be prompted again on next login."},
			},
		}); sendErr != nil {
			s.logger.Warn("sending invalid-selection feedback", zap.String("feature", featureID), zap.Error(sendErr))
			return "", sendErr
		}
		return "", nil
	}

	chosen := choices.Options[idx]
	if sendErr := stream.Send(&gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{Content: choices.Key + " set to: " + chosen},
		},
	}); sendErr != nil {
		// Non-fatal: value is already selected; log and continue.
		s.logger.Warn("sending choice confirmation", zap.String("feature", featureID), zap.Error(sendErr))
	}
	return chosen, nil
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

// handleTakeCover applies the in_cover condition (+2 AC for the encounter).
//
// Precondition: uid must identify a valid player session.
// Postcondition: Applies in_cover condition; in combat, deducts 1 AP and updates Combatant ACMod.
func (s *GameServiceServer) handleTakeCover(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
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
			s.logger.Warn("handleTakeCover: ApplyCombatantACMod failed",
				zap.String("uid", uid), zap.Error(err))
			return errorEvent(fmt.Sprintf("Failed to take cover: %v", err)), nil
		}
	}

	// Apply the in_cover condition to the player session.
	if s.condRegistry != nil {
		def, ok := s.condRegistry.Get("in_cover")
		if !ok {
			s.logger.Warn("handleTakeCover: in_cover condition not found in registry",
				zap.String("uid", uid))
		} else {
			if sess.Conditions == nil {
				sess.Conditions = condition.NewActiveSet()
			}
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleTakeCover: Apply in_cover failed",
					zap.String("uid", uid), zap.Error(err))
			}
		}
	}

	return messageEvent("You take cover. (+2 AC for the encounter)"), nil
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
				CurrentHp: int32(newHP),
				MaxHp:     int32(sess.MaxHP),
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
	dc := inst.Perception

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

// handleDemoralize performs a smooth_talk skill check against the target NPC's Level+10 DC.
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

	// Skill check: 1d20 + smooth_talk bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDemoralize: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["smooth_talk"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Demoralize (smooth_talk DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
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

// handleGrapple performs an athletics skill check against the target NPC's Level+10 DC.
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

	// Skill check: 1d20 + athletics bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleGrapple: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Grapple (athletics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
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

// handleTrip performs an athletics skill check against the target NPC's Level+10 DC.
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

	// Skill check: 1d20 + athletics bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleTrip: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Trip (athletics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
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

// handleDisarm performs an athletics skill check against the target NPC's Level+10 DC.
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

	// Skill check: 1d20 + athletics bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDisarm: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Disarm (athletics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
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

	return messageEvent(detail + fmt.Sprintf(" — success! %s is disarmed. The %s clatters to the floor.", inst.Name(), weaponName)), nil
}

// maxNPCPerceptionInRoom returns the highest Perception value among all living NPCs in roomID.
// If no living NPCs are present, returns 10 as the base DC.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns an integer >= 10.
func (s *GameServiceServer) maxNPCPerceptionInRoom(roomID string) int {
	insts := s.npcMgr.InstancesInRoom(roomID)
	max := 10
	for _, inst := range insts {
		if !inst.IsDead() && inst.Perception > max {
			max = inst.Perception
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

// handleEscape performs a max(athletics, acrobatics) skill check to escape the grabbed condition.
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

	if sess.Conditions == nil || !sess.Conditions.Has("grabbed") {
		return errorEvent("You are not grabbed."), nil
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

	ath := skillRankBonus(sess.Skills["athletics"])
	acr := skillRankBonus(sess.Skills["acrobatics"])
	bonus := ath
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

	detail := fmt.Sprintf("Escape (DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. You fail to break free."), nil
	}

	// Success: remove grabbed condition and clear GrabberID.
	sess.Conditions.Remove(uid, "grabbed")
	sess.GrabberID = ""

	return messageEvent(detail + " — success! You break free from the grab."), nil
}

// handleGrant awards XP or money to a named online player (editor/admin command).
//
// Precondition: uid identifies an active session with role "editor" or "admin"; req is non-nil.
// Postcondition: on "xp" grant, target.Experience is increased and persisted;
// on "money" grant, target.Currency is increased and persisted;
// target receives a console notification; caller receives a success MessageEvent.
func (s *GameServiceServer) handleGrant(uid string, req *gamev1.GrantRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	if sess.Role != "editor" && sess.Role != "admin" {
		return errorEvent("permission denied: editor role required"), nil
	}
	if req.Amount <= 0 {
		return errorEvent("amount must be greater than zero"), nil
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

	default:
		return errorEvent(fmt.Sprintf("unknown grant type %q: use 'xp' or 'money'", req.GrantType)), nil
	}
}

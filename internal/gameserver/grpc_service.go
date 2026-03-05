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

	"strings"

	"github.com/cory-johannsen/mud/internal/game/ai"
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
	allSkills           []*ruleset.Skill
	characterSkillsRepo CharacterSkillsRepository
	allFeats                   []*ruleset.Feat
	featRegistry               *ruleset.FeatRegistry
	characterFeatsRepo         CharacterFeatsGetter
	allClassFeatures           []*ruleset.ClassFeature
	classFeatureRegistry       *ruleset.ClassFeatureRegistry
	characterClassFeaturesRepo CharacterClassFeaturesGetter
	featureChoicesRepo         CharacterFeatureChoicesRepository
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
// allFeats may be nil (feat commands will return a not-available message).
// featRegistry may be nil (feat commands will return a not-available message).
// characterFeatsRepo may be nil (feat commands will return a not-available message).
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
	allFeats []*ruleset.Feat,
	featRegistry *ruleset.FeatRegistry,
	characterFeatsRepo CharacterFeatsGetter,
	allClassFeatures []*ruleset.ClassFeature,
	classFeatureRegistry *ruleset.ClassFeatureRegistry,
	characterClassFeaturesRepo CharacterClassFeaturesGetter,
	featureChoicesRepo CharacterFeatureChoicesRepository,
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
		allSkills:           allSkills,
		characterSkillsRepo: characterSkillsRepo,
		allFeats:                   allFeats,
		featRegistry:               featRegistry,
		characterFeatsRepo:         characterFeatsRepo,
		allClassFeatures:           allClassFeatures,
		classFeatureRegistry:       classFeatureRegistry,
		characterClassFeaturesRepo: characterClassFeaturesRepo,
		featureChoicesRepo:         featureChoicesRepo,
	}
	if s.combatH != nil {
		s.combatH.SetOnCombatEnd(func(roomID string) {
			sessions := s.sessions.PlayersInRoomDetails(roomID)
			for _, sess := range sessions {
				if sess.Conditions != nil {
					sess.Conditions.ClearEncounter()
				}
			}
		})
	}
	return s
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
	if characterID > 0 && s.charSaver != nil {
		dbChar, dbErr := s.charSaver.GetByID(stream.Context(), characterID)
		if dbErr != nil {
			s.logger.Warn("failed to load character from DB at login; using zero values",
				zap.Int64("character_id", characterID),
				zap.Error(dbErr),
			)
		} else if dbChar != nil {
			maxHP = dbChar.MaxHP
			abilities = dbChar.Abilities
		}
	}
	sess, err := s.sessions.AddPlayer(session.AddPlayerOptions{
		UID:               uid,
		Username:          username,
		CharName:          charName,
		CharacterID:       characterID,
		RoomID:            spawnRoom.ID,
		CurrentHP:         currentHP,
		MaxHP:             maxHP,
		Abilities:         abilities,
		Role:              role,
		RegionDisplayName: joinReq.RegionDisplay,
		Class:             joinReq.Class,
		Level:             int(joinReq.Level),
	})
	if err != nil {
		return fmt.Errorf("adding player: %w", err)
	}
	defer s.cleanupPlayer(uid, username)

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

	// Load stored feature choices and resolve any missing ones interactively.
	sess.FeatureChoices = make(map[string]map[string]string)
	if characterID > 0 && s.featureChoicesRepo != nil {
		stored, fcErr := s.featureChoicesRepo.GetAll(stream.Context(), characterID)
		if fcErr != nil {
			s.logger.Warn("loading feature choices", zap.Int64("character_id", characterID), zap.Error(fcErr))
		} else {
			sess.FeatureChoices = stored
		}

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

	// Broadcast arrival to other players in the room
	s.broadcastRoomEvent(spawnRoom.ID, uid, &gamev1.RoomEvent{
		Player: charName,
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	})

	// Send initial room view
	roomView := s.worldH.buildRoomView(uid, spawnRoom)
	if err := stream.Send(&gamev1.ServerEvent{
		RequestId: firstMsg.RequestId,
		Payload:   &gamev1.ServerEvent_RoomView{RoomView: roomView},
	}); err != nil {
		return fmt.Errorf("sending initial room view: %w", err)
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
	// Only launched when a clock is configured.
	if clockCh != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case h, ok := <-clockCh:
					if !ok {
						return
					}
					evt := &gamev1.ServerEvent{
						Payload: &gamev1.ServerEvent_TimeOfDay{
							TimeOfDay: &gamev1.TimeOfDayEvent{
								Hour:   int32(h),
								Period: string(h.Period()),
							},
						},
					}
					if err := stream.Send(evt); err != nil {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

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

		outcome := trigger.Outcomes.ForOutcome(result.Outcome)
		if outcome != nil {
			if outcome.Message != "" {
				msgs = append(msgs, outcome.Message)
			}
			s.applySkillCheckEffect(sess, outcome.Effect, room.ID)
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

			outcome := trigger.Outcomes.ForOutcome(result.Outcome)
			if outcome != nil {
				if outcome.Message != "" {
					msgs = append(msgs, outcome.Message)
				}
				// Apply non-deny effects (deny is not applicable for on_greet).
				if outcome.Effect == nil || outcome.Effect.Type != "deny" {
					s.applySkillCheckEffect(sess, outcome.Effect, roomID)
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
	if s.npcH == nil {
		return nil, fmt.Errorf("target %q not found", req.Target)
	}
	view, err := s.npcH.Examine(uid, req.Target)
	if err != nil {
		return nil, err
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_NpcView{NpcView: view},
	}, nil
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
	// Broadcast all events except the first to room (first is returned directly to player).
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

func (s *GameServiceServer) handleEquip(uid string, req *gamev1.EquipRequest) (*gamev1.ServerEvent, error) {
	_, err := s.combatH.Equip(uid, req.GetWeaponId(), req.GetSlot())
	if err != nil {
		return errorEvent(err.Error()), nil
	}
	return messageEvent("Equipped " + req.GetWeaponId() + "."), nil
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
	idx := rand.Intn(len(exits))
	_ = s.npcH.MoveNPC(inst.ID, exits[idx].TargetRoom)
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

	// Armor slots.
	view.Armor = make(map[string]string)
	for slot, item := range sess.Equipment.Armor {
		if item == nil {
			continue
		}
		name := item.Name
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Armor(item.ItemDefID); ok {
				name = def.Name
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
			if preset.MainHand != nil {
				view.MainHand = preset.MainHand.Def.Name
			}
			if preset.OffHand != nil {
				view.OffHand = preset.OffHand.Def.Name
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

	selText := ""
	if say := msg.GetSay(); say != nil {
		selText = strings.TrimSpace(say.GetMessage())
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

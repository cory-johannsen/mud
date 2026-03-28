package gameserver

import (
	"fmt"
	"time"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// RoundDurationMs is the combat round duration in milliseconds.
type RoundDurationMs int

// AITickInterval is the NPC AI tick interval.
type AITickInterval time.Duration

// NewGameCalendarFromRepo loads calendar state and constructs GameCalendar.
// The loaded hour is applied to clock via SetHour so the clock resumes at the
// persisted hour rather than the config default.
func NewGameCalendarFromRepo(repo *postgres.CalendarRepo, clock *GameClock) (*GameCalendar, error) {
	hour, day, month, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("loading calendar state: %w", err)
	}
	clock.SetHour(int32(hour))
	return NewGameCalendar(clock, day, month, repo), nil
}

// NewCommandRegistry builds the command registry with class feature shortcuts.
func NewCommandRegistry(classFeatures []*ruleset.ClassFeature) (*command.Registry, error) {
	allCmds := command.RegisterShortcuts(classFeatures, command.BuiltinCommands())
	return command.NewRegistry(allCmds)
}

// NewSessionManager creates a session manager.
func NewSessionManager() *session.Manager {
	return session.NewManager()
}

// NewChatHandlerProvider creates a ChatHandler.
func NewChatHandlerProvider(sessMgr *session.Manager) *ChatHandler {
	return NewChatHandler(sessMgr)
}

// NewNPCHandlerProvider creates an NPCHandler.
func NewNPCHandlerProvider(npcMgr *npc.Manager, sessMgr *session.Manager) *NPCHandler {
	return NewNPCHandler(npcMgr, sessMgr)
}

// NewWorldHandlerProvider creates a WorldHandler.
func NewWorldHandlerProvider(
	worldMgr *world.Manager,
	sessMgr *session.Manager,
	npcMgr *npc.Manager,
	gameClock *GameClock,
	roomEquipMgr *inventory.RoomEquipmentManager,
	invRegistry *inventory.Registry,
) *WorldHandler {
	return NewWorldHandler(worldMgr, sessMgr, npcMgr, gameClock, roomEquipMgr, invRegistry)
}

// NewCombatHandlerProvider creates a CombatHandler with a nil broadcast function.
// The broadcast function must be wired post-construction via SetBroadcastFn once
// the GRPCService is initialised.
func NewCombatHandlerProvider(
	combatEngine *combat.Engine,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	diceRoller *dice.Roller,
	roundDurationMs RoundDurationMs,
	condRegistry *condition.Registry,
	worldMgr *world.Manager,
	scriptMgr *scripting.Manager,
	invRegistry *inventory.Registry,
	aiRegistry *ai.Registry,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	mentalMgr *mentalstate.Manager,
	logger *zap.Logger,
) *CombatHandler {
	roundDuration := time.Duration(roundDurationMs) * time.Millisecond
	if roundDuration <= 0 {
		roundDuration = 6 * time.Second
	}
	h := NewCombatHandler(combatEngine, npcMgr, sessMgr, diceRoller, nil, roundDuration, condRegistry, worldMgr, scriptMgr, invRegistry, aiRegistry, respawnMgr, floorMgr, mentalMgr)
	h.SetLogger(logger)
	return h
}

// NewActionHandlerProvider creates an ActionHandler.
func NewActionHandlerProvider(
	sessMgr *session.Manager,
	cfReg *ruleset.ClassFeatureRegistry,
	condRegistry *condition.Registry,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	charRepo *postgres.CharacterRepository,
	diceRoller *dice.Roller,
	logger *zap.Logger,
) *ActionHandler {
	return NewActionHandler(sessMgr, cfReg, condRegistry, npcMgr, combatHandler, charRepo, diceRoller, logger)
}

// NewRegenManagerProvider creates a RegenManager.
func NewRegenManagerProvider(
	sessMgr *session.Manager,
	npcMgr *npc.Manager,
	combatHandler *CombatHandler,
	charRepo *postgres.CharacterRepository,
	logger *zap.Logger,
) *RegenManager {
	return NewRegenManager(sessMgr, npcMgr, combatHandler, charRepo, RegenInterval, logger)
}

// NewZoneTickManagerProvider creates a ZoneTickManager.
func NewZoneTickManagerProvider(interval AITickInterval) *ZoneTickManager {
	return NewZoneTickManager(time.Duration(interval))
}

// SetBroadcastFn sets the function used to broadcast combat events to room subscribers.
// Call this after the GRPCService is initialised to wire the circular dependency.
func (h *CombatHandler) SetBroadcastFn(fn func(roomID string, events []*gamev1.CombatEvent)) {
	h.broadcastFn = fn
}

// SetOnMassiveDamage sets the optional callback fired when a player takes ≥50% of their max HP
// in a single combat hit. Used to wire the on_take_damage_in_one_hit_above_threshold drawback
// trigger (REQ-JD-10). May be set to nil to disable.
//
// Precondition: none.
// Postcondition: h.onMassiveDamage == fn.
func (h *CombatHandler) SetOnMassiveDamage(fn func(uid string)) {
	h.onMassiveDamage = fn
}

// HandlerProviders is the wire provider set for game handlers.
var HandlerProviders = wire.NewSet(
	NewChatHandlerProvider,
	NewNPCHandlerProvider,
	NewWorldHandlerProvider,
	NewCombatHandlerProvider,
	NewActionHandlerProvider,
	NewRegenManagerProvider,
	NewZoneTickManagerProvider,
	NewGameCalendarFromRepo,
)

// ServerProviders is the wire provider set for the gRPC service and supporting registries.
var ServerProviders = wire.NewSet(
	NewGameServiceServer,
	NewCommandRegistry,
	NewSessionManager,
	NewAccountRepoAdapter,
	wire.Bind(new(AccountAdmin), new(*AccountRepoAdapter)),
)

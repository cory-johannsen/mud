package gameserver

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// RegenInterval is the default period between regeneration ticks.
const RegenInterval = 30 * time.Second

// regenNPCRate is the flat HP restored per tick for NPCs.
const regenNPCRate = 1

// DetentionChecker is implemented by GameServiceServer to run detention completion checks.
//
// Precondition: sess must not be nil.
// Postcondition: if detention is expired, it is completed and the session updated.
type DetentionChecker interface {
	checkDetentionCompletion(sess *session.PlayerSession)
}

// RegenManager ticks periodically and restores HP to players and NPCs that are
// not currently in combat.
//
// Precondition: sessions, npcMgr, combatH, and charSaver must be non-nil.
type RegenManager struct {
	sessions         *session.Manager
	npcMgr           *npc.Manager
	combatH          *CombatHandler
	charSaver        CharacterSaver
	interval         time.Duration
	logger           *zap.Logger
	detentionChecker DetentionChecker
}

// NewRegenManager constructs a RegenManager.
//
// Precondition: sessions, npcMgr, combatH, and charSaver must be non-nil; interval must be > 0.
// Postcondition: Returns a non-nil *RegenManager ready to Start().
func NewRegenManager(
	sessions *session.Manager,
	npcMgr *npc.Manager,
	combatH *CombatHandler,
	charSaver CharacterSaver,
	interval time.Duration,
	logger *zap.Logger,
) *RegenManager {
	if interval <= 0 {
		panic("gameserver.NewRegenManager: interval must be > 0")
	}
	return &RegenManager{
		sessions:  sessions,
		npcMgr:    npcMgr,
		combatH:   combatH,
		charSaver: charSaver,
		interval:  interval,
		logger:    logger,
	}
}

// SetDetentionChecker registers a DetentionChecker to run during regen ticks.
//
// Precondition: checker may be nil (disables detention checks during regen).
// Postcondition: each regen tick calls checkDetentionCompletion for every player.
func (r *RegenManager) SetDetentionChecker(checker DetentionChecker) {
	r.detentionChecker = checker
}

// Start launches the regen goroutine. Runs until ctx is cancelled.
//
// Postcondition: One regen tick fires per interval until ctx is done.
func (r *RegenManager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.tick(ctx)
			}
		}
	}()
}

// tick performs one regeneration pass over all players and NPC instances.
func (r *RegenManager) tick(ctx context.Context) {
	r.regenPlayers(ctx)
	r.regenNPCs()
}

// regenPlayers heals all idle players who are below max HP.
// Also runs detention completion checks for all players.
// Regen per tick = max(1, GritMod).
func (r *RegenManager) regenPlayers(ctx context.Context) {
	const inCombatStatus = int32(2) // gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT

	for _, sess := range r.sessions.AllPlayers() {
		// Run detention completion check on every regen tick (REQ-WC-14b).
		if r.detentionChecker != nil {
			r.detentionChecker.checkDetentionCompletion(sess)
		}

		if sess.Status == inCombatStatus {
			continue
		}
		if sess.CurrentHP >= sess.MaxHP {
			continue
		}

		gritMod := combat.AbilityMod(sess.Abilities.Grit)
		regen := gritMod
		if regen < 1 {
			regen = 1
		}

		newHP := sess.CurrentHP + regen
		if newHP > sess.MaxHP {
			newHP = sess.MaxHP
		}
		sess.CurrentHP = newHP

		// Persist the updated HP.
		if r.charSaver != nil {
			if err := r.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, newHP); err != nil {
				if r.logger != nil {
					r.logger.Warn("regen: saving player HP",
						zap.String("uid", sess.UID),
						zap.Error(err),
					)
				}
			}
		}

		// Notify the player with a console message.
		msg := fmt.Sprintf("You recover %d HP. (%d/%d)", regen, newHP, sess.MaxHP)
		msgEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: msg,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, err := proto.Marshal(msgEvt); err == nil {
			_ = sess.Entity.Push(data)
		}

		// Send an HP update event so the prompt bar refreshes immediately.
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
	}
}

// regenNPCs heals all NPC instances not currently in combat rooms.
// Regen per tick is regenNPCRate.
func (r *RegenManager) regenNPCs() {
	for _, inst := range r.npcMgr.AllInstances() {
		if r.combatH.IsRoomInCombat(inst.RoomID) {
			continue
		}
		if inst.CurrentHP >= inst.MaxHP {
			continue
		}
		inst.CurrentHP += regenNPCRate
		if inst.CurrentHP > inst.MaxHP {
			inst.CurrentHP = inst.MaxHP
		}
	}
}

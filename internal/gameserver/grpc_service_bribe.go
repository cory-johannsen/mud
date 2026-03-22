package gameserver

import (
	"context"
	"fmt"
	"math"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// zoneMultiplier returns the bribe cost multiplier for a given danger level.
//
// Precondition: dangerLevel is a string from the zone/room DangerLevel field.
// Postcondition: Returns a positive float64 multiplier.
func zoneMultiplier(dangerLevel string) float64 {
	switch dangerLevel {
	case "safe":
		return 0.8
	case "sketchy":
		return 1.0
	case "dangerous":
		return 1.5
	case "all_out_war":
		return 2.5
	default:
		return 1.0
	}
}

// briberCandidate holds the resolved bribe details for a single NPC.
type briberCandidate struct {
	inst     *npc.Instance
	baseCost int
	variance float64
}

// handleBribe processes the bribe command (step 1 of 2).
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sets sess.PendingBribeNPCName and sess.PendingBribeAmount.
func (s *GameServiceServer) handleBribe(uid string, req *gamev1.BribeRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	// Resolve current room and zone.
	room, roomOK := s.world.GetRoom(sess.RoomID)
	if !roomOK {
		return messageEvent("current room not found"), nil
	}
	zoneID := room.ZoneID

	wantedLevel := sess.WantedLevel[zoneID]
	if wantedLevel == 0 {
		return messageEvent("You are not wanted here. There is nothing to clear."), nil
	}

	// Collect bribeable NPCs in the room.
	instances := s.npcMgr.InstancesInRoom(sess.RoomID)
	var candidates []briberCandidate
	for _, inst := range instances {
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil {
			continue
		}
		switch inst.NPCType {
		case "fixer":
			if tmpl.Fixer == nil {
				continue
			}
			if wantedLevel > tmpl.Fixer.MaxWantedLevel {
				continue
			}
			baseCost, hasCost := tmpl.Fixer.BaseCosts[wantedLevel]
			if !hasCost {
				continue
			}
			candidates = append(candidates, briberCandidate{
				inst:     inst,
				baseCost: baseCost,
				variance: tmpl.Fixer.NPCVariance,
			})
		case "guard":
			if tmpl.Guard == nil || !tmpl.Guard.Bribeable {
				continue
			}
			if wantedLevel > tmpl.Guard.MaxBribeWantedLevel {
				continue
			}
			baseCost, hasCost := tmpl.Guard.BaseCosts[wantedLevel]
			if !hasCost {
				continue
			}
			candidates = append(candidates, briberCandidate{
				inst:     inst,
				baseCost: baseCost,
				variance: 1.0,
			})
		}
	}

	if len(candidates) == 0 {
		return messageEvent("There is no one here who can help you with that."), nil
	}

	// Select target NPC.
	var target *briberCandidate
	if req.GetNpcName() == "" {
		if len(candidates) > 1 {
			names := make([]string, len(candidates))
			for i, c := range candidates {
				names[i] = c.inst.Name()
			}
			return messageEvent(fmt.Sprintf(
				"Multiple people here can help you. Specify one: %s",
				strings.Join(names, ", "),
			)), nil
		}
		target = &candidates[0]
	} else {
		lowerTarget := strings.ToLower(req.GetNpcName())
		for i := range candidates {
			if strings.ToLower(candidates[i].inst.Name()) == lowerTarget {
				target = &candidates[i]
				break
			}
		}
		if target == nil {
			return messageEvent(fmt.Sprintf(
				"You don't see %q here who can help you.", req.GetNpcName(),
			)), nil
		}
	}

	// Compute cost: floor(baseCost × zoneMultiplier × npcVariance).
	dangerLevel := room.DangerLevel
	if dangerLevel == "" {
		if zone, zoneOK := s.world.GetZone(zoneID); zoneOK {
			dangerLevel = zone.DangerLevel
		}
	}
	mult := zoneMultiplier(dangerLevel)
	cost := int(math.Floor(float64(target.baseCost) * mult * target.variance))

	// Set pending bribe state.
	sess.PendingBribeNPCName = target.inst.Name()
	sess.PendingBribeAmount = cost

	return messageEvent(fmt.Sprintf(
		"%s will clear your record for %d credits. Type 'bribe confirm' to proceed.",
		target.inst.Name(), cost,
	)), nil
}

// handleBribeConfirm processes the bribe confirmation (step 2 of 2).
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, deducts credits, decrements WantedLevel, clears pending state.
func (s *GameServiceServer) handleBribeConfirm(uid string, req *gamev1.BribeConfirmRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	if sess.PendingBribeNPCName == "" {
		return messageEvent("You have no pending bribe to confirm."), nil
	}

	cost := sess.PendingBribeAmount
	// Clear pending state unconditionally before the credit check so that a failed
	// attempt does not leave stale state on subsequent commands.
	npcName := sess.PendingBribeNPCName
	sess.PendingBribeNPCName = ""
	sess.PendingBribeAmount = 0

	if sess.Currency < cost {
		return messageEvent(fmt.Sprintf(
			"You need %d credits but only have %d.",
			cost, sess.Currency,
		)), nil
	}

	// Resolve current room and zone.
	room, roomOK := s.world.GetRoom(sess.RoomID)
	if !roomOK {
		return messageEvent("current room not found"), nil
	}
	zoneID := room.ZoneID

	// Deduct credits.
	sess.Currency -= cost

	// Decrement wanted level (floor at 0).
	if sess.WantedLevel[zoneID] > 0 {
		sess.WantedLevel[zoneID]--
	}
	newLevel := sess.WantedLevel[zoneID]

	ctx := context.Background()

	// Persist wanted level.
	if s.wantedRepo != nil {
		if err := s.wantedRepo.Upsert(ctx, sess.CharacterID, zoneID, newLevel); err != nil {
			s.logger.Warn("handleBribeConfirm: Upsert wanted level failed",
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}

	// Persist credits.
	if s.charSaver != nil {
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("handleBribeConfirm: SaveCurrency failed",
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}

	if newLevel == 0 {
		return messageEvent(fmt.Sprintf(
			"%s clears your record. You are no longer wanted in this zone.",
			npcName,
		)), nil
	}
	return messageEvent(fmt.Sprintf(
		"%s clears your record. Wanted level reduced to %d.",
		npcName, newLevel,
	)), nil
}

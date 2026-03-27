package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// getOrCreateSeduceConditions returns the condition.ActiveSet for the given NPC instance,
// creating one if it does not yet exist.
//
// Precondition: npcID must be non-empty.
// Postcondition: s.seduceConditions[npcID] is non-nil.
func (s *GameServiceServer) getOrCreateSeduceConditions(npcID string) *condition.ActiveSet {
	if s.seduceConditions == nil {
		s.seduceConditions = make(map[string]*condition.ActiveSet)
	}
	if cs, ok := s.seduceConditions[npcID]; ok {
		return cs
	}
	cs := condition.NewActiveSet()
	s.seduceConditions[npcID] = cs
	return cs
}

// executeSeduce resolves a player's attempt to seduce an NPC (REQ-ZN-7, REQ-ZN-8).
//
// Preconditions:
//   - inst.Gender != "" (genderless NPCs cannot be seduced)
//   - sess.Skills["flair"] is non-empty (player must have some flair rank)
//   - NPC does not already have the "charmed" condition
//   - inst.SeductionRejected[uid] is not true
//
// Resolution: opposed check. playerRoll = d20 + skillRankBonus(sess.Skills["flair"]);
// npcRoll = d20 + inst.Savvy.
// On player success (playerRoll >= npcRoll): NPC gains "charmed" condition.
// On player failure: NPC turns hostile; SeductionRejected[uid] = true.
func (s *GameServiceServer) executeSeduce(
	sess *session.PlayerSession, uid string, inst *npc.Instance,
) (string, error) {
	if inst.Gender == "" {
		return fmt.Sprintf("%s cannot be seduced.", inst.Name()), nil
	}
	if sess.Skills["flair"] == "" {
		return "You lack the charm to attempt seduction.", nil
	}

	cs := s.getOrCreateSeduceConditions(inst.ID)
	if cs.Has("charmed") {
		return fmt.Sprintf("%s is already charmed.", inst.Name()), nil
	}

	if inst.SeductionRejected != nil && inst.SeductionRejected[uid] {
		return fmt.Sprintf("%s is not interested in your advances.", inst.Name()), nil
	}

	flairBonus := skillRankBonus(sess.Skills["flair"])
	var playerRoll, npcRoll int
	if s.dice != nil {
		d20a, err := s.dice.RollExpr("d20")
		if err != nil {
			return "", fmt.Errorf("executeSeduce: rolling d20 for player: %w", err)
		}
		d20b, err := s.dice.RollExpr("d20")
		if err != nil {
			return "", fmt.Errorf("executeSeduce: rolling d20 for NPC: %w", err)
		}
		playerRoll = d20a.Total() + flairBonus
		npcRoll = d20b.Total() + inst.Savvy
	} else {
		// Fallback for tests without a dice source: neutral base of 10.
		playerRoll = 10 + flairBonus
		npcRoll = 10 + inst.Savvy
	}

	if playerRoll >= npcRoll {
		if s.condRegistry != nil {
			if def, ok := s.condRegistry.Get("charmed"); ok {
				_ = cs.Apply(inst.ID, def, 1, -1)
			}
		} else {
			// No registry available (tests without condRegistry); apply a minimal def.
			def := &condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"}
			_ = cs.Apply(inst.ID, def, 1, -1)
		}
		return fmt.Sprintf("You charm %s with your winning smile. They seem... charmed.", inst.Name()), nil
	}

	inst.Disposition = "hostile"
	if inst.SeductionRejected == nil {
		inst.SeductionRejected = make(map[string]bool)
	}
	inst.SeductionRejected[uid] = true
	return fmt.Sprintf("%s rejects your advances and turns hostile!", inst.Name()), nil
}

// handleSeduce is the gRPC dispatch handler for SeduceRequest (REQ-ZN-7).
//
// Precondition: uid must be a registered player; req.Target must name an NPC in the player's room.
// Postcondition: Returns a message event with the seduction outcome.
func (s *GameServiceServer) handleSeduce(uid string, req *gamev1.SeduceRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("handleSeduce: player %q not found", uid)
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: seduce <target>"), nil
	}

	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	msg, err := s.executeSeduce(sess, uid, inst)
	if err != nil {
		return nil, err
	}
	return messageEvent(msg), nil
}

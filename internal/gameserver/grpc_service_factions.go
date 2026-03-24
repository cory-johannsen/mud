package gameserver

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleFaction shows the player's faction, tier, rep, and perks (REQ-FA-37).
//
// Precondition: uid identifies an active player session.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleFaction(uid string, req *gamev1.FactionRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	if sess.FactionID == "" {
		return messageEvent("You have no faction affiliation."), nil
	}
	if s.factionRegistry == nil {
		return messageEvent("Faction system not configured."), nil
	}
	def := s.factionRegistry.ByID(sess.FactionID)
	if def == nil {
		return messageEvent(fmt.Sprintf("Unknown faction %q in registry.", sess.FactionID)), nil
	}
	rep := sess.FactionRep[sess.FactionID]
	tier := s.factionSvc.TierFor(sess.FactionID, rep)
	next := s.factionSvc.NextTier(sess)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n", def.Name))
	if tier != nil {
		sb.WriteString(fmt.Sprintf("Tier:       %s\n", tier.Label))
	}
	sb.WriteString(fmt.Sprintf("Reputation: %d\n", rep))
	if next != nil {
		sb.WriteString(fmt.Sprintf("Next tier:  %s (requires %d rep, need %d more)\n",
			next.Label, next.MinRep, next.MinRep-rep))
	} else {
		sb.WriteString("Max tier reached.\n")
	}
	if tier != nil && tier.PriceDiscount > 0 {
		sb.WriteString(fmt.Sprintf("Discount:   %.0f%% at faction merchants\n", tier.PriceDiscount*100))
	}
	return messageEvent(sb.String()), nil
}

// handleFactionInfo shows public faction info (REQ-FA-38).
//
// Precondition: uid identifies an active player session; req.FactionId is non-empty.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleFactionInfo(uid string, req *gamev1.FactionInfoRequest) (*gamev1.ServerEvent, error) {
	if _, ok := s.sessions.GetPlayer(uid); !ok {
		return messageEvent("player not found"), nil
	}
	if s.factionRegistry == nil {
		return messageEvent("Faction system not configured."), nil
	}
	def := s.factionRegistry.ByID(req.GetFactionId())
	if def == nil {
		return messageEvent(fmt.Sprintf("Unknown faction %q.", req.GetFactionId())), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n", def.Name))
	sb.WriteString(fmt.Sprintf("Zone: %s\n", def.ZoneID))
	sb.WriteString("Tiers:\n")
	for _, t := range def.Tiers {
		sb.WriteString(fmt.Sprintf("  %-16s  min_rep=%-6d  discount=%.0f%%\n",
			t.Label, t.MinRep, t.PriceDiscount*100))
	}
	return messageEvent(sb.String()), nil
}

// handleFactionStanding shows the player's standing in all tracked factions (REQ-FA-39).
//
// Precondition: uid identifies an active player session.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleFactionStanding(uid string, req *gamev1.FactionStandingRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	if len(sess.FactionRep) == 0 {
		return messageEvent("=== Faction Standing ===\nNo standing with any faction yet.\n"), nil
	}

	ids := make([]string, 0, len(sess.FactionRep))
	for id := range sess.FactionRep {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var sb strings.Builder
	sb.WriteString("=== Faction Standing ===\n")
	for _, fid := range ids {
		rep := sess.FactionRep[fid]
		name := fid
		if s.factionRegistry != nil {
			if def := s.factionRegistry.ByID(fid); def != nil {
				name = def.Name
			}
		}
		tierLabel := "Unknown"
		if s.factionSvc != nil {
			if tier := s.factionSvc.TierFor(fid, rep); tier != nil {
				tierLabel = tier.Label
			}
		}
		hostile := ""
		if s.factionSvc != nil && s.factionSvc.IsHostile(fid, sess.FactionID) {
			hostile = " (hostile)"
		}
		sb.WriteString(fmt.Sprintf("  %-20s  %-14s  rep=%-6d%s\n", name, tierLabel, rep, hostile))
	}
	return messageEvent(sb.String()), nil
}

// handleChangeRep processes the change_rep command via a Fixer NPC (REQ-FA-34, 35, 36).
//
// Precondition: uid identifies an active player session; req.FactionId is the target faction.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleChangeRep(uid string, req *gamev1.ChangeRepRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	if sess.FactionID == "" {
		return messageEvent("You have no faction affiliation."), nil
	}

	room, roomOK := s.world.GetRoom(sess.RoomID)
	if !roomOK {
		return messageEvent("current room not found"), nil
	}

	var fixerNPCVariance float64
	foundFixer := false
	if s.npcMgr != nil {
		instances := s.npcMgr.InstancesInRoom(sess.RoomID)
		for _, inst := range instances {
			if inst.NPCType != "fixer" {
				continue
			}
			tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
			if tmpl == nil || tmpl.Fixer == nil {
				continue
			}
			if inst.Cowering {
				continue
			}
			if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
				return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
			}
			fixerNPCVariance = tmpl.Fixer.NPCVariance
			foundFixer = true
			break
		}
	}
	if !foundFixer {
		return messageEvent("There is no Fixer here."), nil
	}

	if s.factionSvc != nil && s.factionSvc.IsAtMaxTier(sess) {
		return messageEvent("You've reached the highest standing with your faction."), nil
	}

	if s.factionConfig == nil {
		return messageEvent("Faction economy not configured."), nil
	}

	currentTierIdx := 1
	if s.factionSvc != nil {
		currentTierIdx = s.factionSvc.CurrentTierIndex(sess)
	}
	baseCost, costOK := s.factionConfig.RepChangeCosts[currentTierIdx]
	if !costOK {
		return messageEvent("No rep change cost configured for your tier."), nil
	}
	zm := zoneMultiplier(room.DangerLevel)
	cost := int(math.Floor(float64(baseCost) * fixerNPCVariance * zm))

	if sess.Currency < cost {
		return messageEvent(fmt.Sprintf("The Fixer wants %d credits to improve your standing. You only have %d.", cost, sess.Currency)), nil
	}

	sess.Currency -= cost
	if s.charSaver != nil {
		if err := s.charSaver.SaveCurrency(context.Background(), sess.CharacterID, sess.Currency); err != nil {
			return messageEvent("Failed to save currency."), nil
		}
	}

	nextTier := s.factionSvc.NextTier(sess)
	amount := s.factionConfig.RepPerFixerService
	if nextTier != nil {
		currentRep := sess.FactionRep[sess.FactionID]
		cap := nextTier.MinRep - 1 - currentRep
		if cap < 0 {
			cap = 0
		}
		if amount > cap {
			amount = cap
		}
	}

	msg, err := s.factionSvc.AwardRep(context.Background(), sess, sess.CharacterID, sess.FactionID, amount)
	if err != nil {
		return messageEvent(fmt.Sprintf("Failed to award rep: %v", err)), nil
	}
	response := fmt.Sprintf("You pay %d credits. Your standing with %s improves.", cost, sess.FactionID)
	if msg != "" {
		response += "\n" + msg
	}
	return messageEvent(response), nil
}

package gameserver

// handleTrainTech and supporting functions for tech_trainer NPC interaction.
//
// REQ-TTA-1: Tech trainers MUST resolve pending L2+ prepared and spontaneous slots.
// REQ-TTA-2: L2+ slots always require a trainer (unconditionally deferred at level-up).
// REQ-TTA-12: Pending slots are persisted in DB and decremented on resolution.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findTechTrainerInRoom locates a tech_trainer NPC by name in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findTechTrainerInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "tech_trainer" {
		return nil, fmt.Sprintf("%s is not a tech trainer.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// handleTrainTech processes a tech training command.
//
// Precondition: uid identifies an active player session; npcName and techID are non-empty.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// When techID is empty, returns a listing of available techs for this tradition.
// When techID is non-empty, resolves one pending slot if the player qualifies.
func (s *GameServiceServer) handleTrainTech(uid, npcName, techID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	inst, errMsg := s.findTechTrainerInRoom(sess.RoomID, npcName)
	if inst == nil {
		return messageEvent(errMsg), nil
	}

	// Enemy faction check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}

	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.TechTrainer == nil {
		return messageEvent("This trainer has no configuration."), nil
	}
	cfg := tmpl.TechTrainer

	// Evaluate prerequisites before showing offerings or training.
	completedQuests := sess.GetCompletedQuests()
	completedIDs := make(map[string]bool, len(completedQuests))
	for qid, completedAt := range completedQuests {
		if completedAt != nil {
			completedIDs[qid] = true
		}
	}
	var tierChecker npc.FactionTierChecker
	if s.factionSvc != nil {
		tierChecker = func(factionID, minTierID string, factionRep map[string]int) bool {
			rep := factionRep
			if rep == nil {
				rep = sess.FactionRep
			}
			return s.factionSvc.PlayerMeetsTier(factionID, minTierID, rep)
		}
	}
	if err := npc.EvalTechTrainPrereqs(cfg.Prerequisites, completedIDs, tierChecker); err != nil {
		return messageEvent(fmt.Sprintf("%s says: %s", inst.Name(), err.Error())), nil
	}

	if techID == "" {
		return s.listTechTrainerOfferings(inst, cfg, sess), nil
	}
	evt := s.doTrainTech(context.Background(), sess, inst, cfg, techID)
	// Push updated character sheet so the client reflects the newly trained tech.
	s.pushCharacterSheet(sess)
	return evt, nil
}

// listTechTrainerOfferings returns a formatted menu of the trainer's tradition and levels.
//
// Precondition: inst, cfg, and sess must not be nil.
// Postcondition: Returns a non-nil ServerEvent listing pending tech options for the player.
func (s *GameServiceServer) listTechTrainerOfferings(inst *npc.Instance, cfg *npc.TechTrainerConfig, sess *session.PlayerSession) *gamev1.ServerEvent {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s's Training (%s tradition) ===\n", inst.Name(), cfg.Tradition))

	// Collect all pending options matching this trainer's tradition.
	options := s.computeTrainableOptions(sess, cfg)
	if len(options) == 0 {
		sb.WriteString("You have no pending technology slots for this tradition.\n")
		sb.WriteString(fmt.Sprintf("Level-up to earn %s tradition slots.\n", cfg.Tradition))
		return messageEvent(sb.String())
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].techLevel != options[j].techLevel {
			return options[i].techLevel < options[j].techLevel
		}
		return options[i].techID < options[j].techID
	})

	sb.WriteString(fmt.Sprintf("%-30s %6s  %8s  %s\n", "Technology", "Level", "Cost", "Usage"))
	for _, opt := range options {
		name := opt.techID
		if s.techRegistry != nil {
			if def, ok := s.techRegistry.Get(opt.techID); ok {
				name = def.Name
			}
		}
		cost := cfg.TrainingCost(opt.techLevel)
		sb.WriteString(fmt.Sprintf("%-30s %6d  %8d  %s\n", name, opt.techLevel, cost, opt.usageType))
	}
	sb.WriteString(fmt.Sprintf("\nUsage: train <npc> <tech_id>\n"))
	return messageEvent(sb.String())
}

// techOption represents one tech available to a player via a trainer.
type techOption struct {
	techID    string
	techLevel int
	usageType string
	charLevel int // character level at which this grant was earned
}

// isTechKnown returns true if the player already has techID in any of their
// trained tech collections (prepared, spontaneous, or hardwired).
//
// Precondition: sess must not be nil.
// Postcondition: Returns true iff techID appears in at least one trained collection.
func isTechKnown(sess *session.PlayerSession, techID string) bool {
	for _, id := range sess.HardwiredTechs {
		if id == techID {
			return true
		}
	}
	for _, slots := range sess.PreparedTechs {
		for _, slot := range slots {
			if slot != nil && slot.TechID == techID {
				return true
			}
		}
	}
	for _, ids := range sess.KnownTechs {
		for _, id := range ids {
			if id == techID {
				return true
			}
		}
	}
	return false
}

// computeTrainableOptions returns all (techID, level, usageType) tuples from the player's
// pending grants that match the trainer's tradition and offered levels, excluding any
// techs the player already knows.
//
// Precondition: sess and cfg must not be nil.
// Postcondition: Returns a non-nil slice (may be empty).
func (s *GameServiceServer) computeTrainableOptions(sess *session.PlayerSession, cfg *npc.TechTrainerConfig) []techOption {
	var opts []techOption
	for charLvl, grants := range sess.PendingTechGrants {
		if grants == nil {
			continue
		}
		// Prepared pool entries.
		if grants.Prepared != nil {
			for techLvl, slots := range grants.Prepared.SlotsByLevel {
				if slots <= 0 {
					continue
				}
				if !cfg.OffersLevel(techLvl) {
					continue
				}
				for _, e := range grants.Prepared.Pool {
					if e.Level != techLvl {
						continue
					}
					if isTechKnown(sess, e.ID) {
						continue
					}
					if s.techRegistry != nil {
						def, ok := s.techRegistry.Get(e.ID)
						if !ok || string(def.Tradition) != cfg.Tradition {
							continue
						}
					}
					opts = append(opts, techOption{
						techID:    e.ID,
						techLevel: techLvl,
						usageType: "prepared",
						charLevel: charLvl,
					})
				}
			}
		}
		// Spontaneous pool entries.
		if grants.Spontaneous != nil {
			for techLvl, slots := range grants.Spontaneous.KnownByLevel {
				if slots <= 0 {
					continue
				}
				if !cfg.OffersLevel(techLvl) {
					continue
				}
				for _, e := range grants.Spontaneous.Pool {
					if e.Level != techLvl {
						continue
					}
					if isTechKnown(sess, e.ID) {
						continue
					}
					if s.techRegistry != nil {
						def, ok := s.techRegistry.Get(e.ID)
						if !ok || string(def.Tradition) != cfg.Tradition {
							continue
						}
					}
					opts = append(opts, techOption{
						techID:    e.ID,
						techLevel: techLvl,
						usageType: "spontaneous",
						charLevel: charLvl,
					})
				}
			}
		}
	}
	return opts
}

// doTrainTech executes the full training flow for a single tech selection.
//
// Precondition: sess, inst, and cfg must not be nil; techID must be non-empty.
// Postcondition: Returns a non-nil ServerEvent; training succeeds or returns denial.
func (s *GameServiceServer) doTrainTech(
	ctx context.Context,
	sess *session.PlayerSession,
	inst *npc.Instance,
	cfg *npc.TechTrainerConfig,
	techID string,
) *gamev1.ServerEvent {
	// Find the matching option in the player's pending grants.
	opts := s.computeTrainableOptions(sess, cfg)
	var matched *techOption
	for i := range opts {
		if opts[i].techID == techID {
			matched = &opts[i]
			break
		}
	}
	if matched == nil {
		return messageEvent(fmt.Sprintf(
			"%s doesn't offer training for %q in your current pending slots.",
			inst.Name(), techID,
		))
	}

	// Validate trainer offers this level.
	if !cfg.OffersLevel(matched.techLevel) {
		return messageEvent(fmt.Sprintf(
			"%s cannot train level-%d %s technologies.",
			inst.Name(), matched.techLevel, cfg.Tradition,
		))
	}

	// Check cost.
	cost := cfg.TrainingCost(matched.techLevel)
	if sess.Currency < cost {
		return messageEvent(fmt.Sprintf(
			"Training costs %d credits but you only have %d.",
			cost, sess.Currency,
		))
	}

	// Resolve the tech def from registry (if available) to get UsageType.
	usageType := matched.usageType
	if s.techRegistry != nil {
		if def, ok := s.techRegistry.Get(techID); ok {
			usageType = string(def.UsageType)
		}
	}

	// Fill the slot in the session.
	switch usageType {
	case string(technology.UsagePrepared):
		if sess.PreparedTechs == nil {
			sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
		}
		// Find the next available slot index: reuse a nil gap if present, otherwise append.
		// This is necessary because BackfillLevelUpTechnologies may delete/rearrange slots,
		// leaving nil entries in the in-memory slice (padded by GetAll). Using len() on a
		// nil-padded slice produces an inflated index that does not survive a DB reload.
		slots := sess.PreparedTechs[matched.techLevel]
		idx := len(slots) // default: append at end
		reusingGap := false
		for i, s := range slots {
			if s == nil {
				idx = i
				reusingGap = true
				break
			}
		}
		newSlot := &session.PreparedSlot{TechID: techID}
		if reusingGap {
			sess.PreparedTechs[matched.techLevel][idx] = newSlot
		} else {
			sess.PreparedTechs[matched.techLevel] = append(
				sess.PreparedTechs[matched.techLevel],
				newSlot,
			)
		}
		// Persist prepared slot to DB.
		if s.preparedTechRepo != nil && sess.CharacterID > 0 {
			if err := s.preparedTechRepo.Set(ctx, sess.CharacterID, matched.techLevel, idx, techID); err != nil {
				s.logger.Warn("doTrainTech: preparedTechRepo.Set failed",
					zap.String("tech_id", techID),
					zap.Error(err),
				)
			}
		}
		// Catalog-based models (wizard, ranger) track all trained techs in KnownTechs.
		// Druid model uses the full pool at rest and does not track individual techs.
		// REQ-TC-11: Trainer populates KnownTechs for wizard and ranger models.
		// REQ-TC-22: For druid model, trainer assigns PreparedTechs but does NOT add to KnownTechs.
		if sess.CastingModel == ruleset.CastingModelWizard || sess.CastingModel == ruleset.CastingModelRanger {
			if sess.KnownTechs == nil {
				sess.KnownTechs = make(map[int][]string)
			}
			if !containsString(sess.KnownTechs[matched.techLevel], techID) {
				sess.KnownTechs[matched.techLevel] = append(sess.KnownTechs[matched.techLevel], techID)
			}
			if s.knownTechRepo != nil && sess.CharacterID > 0 {
				if err := s.knownTechRepo.Add(ctx, sess.CharacterID, techID, matched.techLevel); err != nil {
					s.logger.Warn("doTrainTech: knownTechRepo.Add failed",
						zap.String("tech_id", techID),
						zap.Error(err),
					)
				}
			}
		}

	case string(technology.UsageSpontaneous):
		if sess.KnownTechs == nil {
			sess.KnownTechs = make(map[int][]string)
		}
		sess.KnownTechs[matched.techLevel] = append(
			sess.KnownTechs[matched.techLevel],
			techID,
		)
		// Persist spontaneous slot to DB.
		if s.knownTechRepo != nil && sess.CharacterID > 0 {
			if err := s.knownTechRepo.Add(ctx, sess.CharacterID, techID, matched.techLevel); err != nil {
				s.logger.Warn("doTrainTech: knownTechRepo.Add failed",
					zap.String("tech_id", techID),
					zap.Error(err),
				)
			}
		}

	default:
		// Treat unknown usage type as prepared (defensive fallback).
		if sess.PreparedTechs == nil {
			sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
		}
		idx := len(sess.PreparedTechs[matched.techLevel])
		sess.PreparedTechs[matched.techLevel] = append(
			sess.PreparedTechs[matched.techLevel],
			&session.PreparedSlot{TechID: techID},
		)
		if s.preparedTechRepo != nil && sess.CharacterID > 0 {
			if err := s.preparedTechRepo.Set(ctx, sess.CharacterID, matched.techLevel, idx, techID); err != nil {
				s.logger.Warn("doTrainTech: preparedTechRepo.Set (fallback) failed",
					zap.String("tech_id", techID),
					zap.Error(err),
				)
			}
		}
		// Catalog-based models (wizard, ranger) track all trained techs in KnownTechs.
		// REQ-TC-11: Trainer populates KnownTechs for wizard and ranger models.
		if sess.CastingModel == ruleset.CastingModelWizard || sess.CastingModel == ruleset.CastingModelRanger {
			if sess.KnownTechs == nil {
				sess.KnownTechs = make(map[int][]string)
			}
			if !containsString(sess.KnownTechs[matched.techLevel], techID) {
				sess.KnownTechs[matched.techLevel] = append(sess.KnownTechs[matched.techLevel], techID)
			}
			if s.knownTechRepo != nil && sess.CharacterID > 0 {
				if err := s.knownTechRepo.Add(ctx, sess.CharacterID, techID, matched.techLevel); err != nil {
					s.logger.Warn("doTrainTech: knownTechRepo.Add failed",
						zap.String("tech_id", techID),
						zap.Error(err),
					)
				}
			}
		}
	}

	// Deduct currency.
	sess.Currency -= cost
	if s.charSaver != nil && sess.CharacterID > 0 {
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("doTrainTech: SaveCurrency failed",
				zap.Int64("character_id", sess.CharacterID),
				zap.Error(err),
			)
		}
	}

	// Decrement the pending slot in the DB.
	if s.pendingTechSlotsRepo != nil && sess.CharacterID > 0 {
		if err := s.pendingTechSlotsRepo.DecrementPendingTechSlot(
			ctx, sess.CharacterID, matched.charLevel, matched.techLevel,
			cfg.Tradition, matched.usageType,
		); err != nil {
			s.logger.Warn("doTrainTech: DecrementPendingTechSlot failed",
				zap.Error(err),
			)
		}
	}

	// Consume the pending grant slot in-session.
	s.consumePendingTechGrant(sess, matched)

	// Auto-complete the find-trainer quest if present.
	if cfg.FindQuestID != "" && s.questSvc != nil {
		if _, active := sess.GetActiveQuests()[cfg.FindQuestID]; active {
			if _, err := s.questSvc.Complete(ctx, sess, sess.CharacterID, cfg.FindQuestID); err != nil {
				s.logger.Warn("doTrainTech: quest Complete failed",
					zap.String("quest_id", cfg.FindQuestID),
					zap.Error(err),
				)
			}
		}
	}

	techName := techID
	if s.techRegistry != nil {
		if def, ok := s.techRegistry.Get(techID); ok {
			techName = def.Name
		}
	}
	return messageEvent(fmt.Sprintf(
		"%s installs %s (level %d, %s) into your neural architecture. Cost: %d credits.",
		inst.Name(), techName, matched.techLevel, cfg.Tradition, cost,
	))
}

// consumePendingTechGrant removes one matching slot from the session's PendingTechGrants.
// If the char-level entry becomes empty after removal, it is deleted.
//
// Precondition: sess and matched must not be nil.
// Postcondition: One pending pool entry is removed; the char-level key is deleted when empty.
func (s *GameServiceServer) consumePendingTechGrant(sess *session.PlayerSession, matched *techOption) {
	grants, ok := sess.PendingTechGrants[matched.charLevel]
	if !ok || grants == nil {
		return
	}

	switch matched.usageType {
	case "prepared":
		if grants.Prepared == nil {
			return
		}
		// Remove one pool entry matching the techID at the given level.
		remaining := make([]ruleset.PreparedEntry, 0, len(grants.Prepared.Pool))
		removed := false
		for _, e := range grants.Prepared.Pool {
			if !removed && e.ID == matched.techID && e.Level == matched.techLevel {
				removed = true
				continue
			}
			remaining = append(remaining, e)
		}
		grants.Prepared.Pool = remaining
		// Decrement the slot count.
		if grants.Prepared.SlotsByLevel[matched.techLevel] > 0 {
			grants.Prepared.SlotsByLevel[matched.techLevel]--
			if grants.Prepared.SlotsByLevel[matched.techLevel] == 0 {
				delete(grants.Prepared.SlotsByLevel, matched.techLevel)
			}
		}

	case "spontaneous":
		if grants.Spontaneous == nil {
			return
		}
		remaining := make([]ruleset.SpontaneousEntry, 0, len(grants.Spontaneous.Pool))
		removed := false
		for _, e := range grants.Spontaneous.Pool {
			if !removed && e.ID == matched.techID && e.Level == matched.techLevel {
				removed = true
				continue
			}
			remaining = append(remaining, e)
		}
		grants.Spontaneous.Pool = remaining
		if grants.Spontaneous.KnownByLevel[matched.techLevel] > 0 {
			grants.Spontaneous.KnownByLevel[matched.techLevel]--
			if grants.Spontaneous.KnownByLevel[matched.techLevel] == 0 {
				delete(grants.Spontaneous.KnownByLevel, matched.techLevel)
			}
		}
	}

	// If the grant is now fully empty, remove the char-level key.
	if isPendingGrantEmpty(grants) {
		delete(sess.PendingTechGrants, matched.charLevel)
	}

	// Update persisted pending tech levels to reflect the current set.
	if s.progressRepo != nil && sess.CharacterID > 0 {
		levels := make([]int, 0, len(sess.PendingTechGrants))
		for lvl := range sess.PendingTechGrants {
			levels = append(levels, lvl)
		}
		if err := s.progressRepo.SetPendingTechLevels(context.Background(), sess.CharacterID, levels); err != nil {
			s.logger.Warn("consumePendingTechGrant: SetPendingTechLevels failed",
				zap.Int64("character_id", sess.CharacterID),
				zap.Error(err),
			)
		}
	}
}

// isPendingGrantEmpty returns true iff the grants object has no remaining slots.
//
// Precondition: grants may be nil.
// Postcondition: Returns true when grants is nil or all slot maps are empty.
func isPendingGrantEmpty(grants *ruleset.TechnologyGrants) bool {
	if grants == nil {
		return true
	}
	if grants.Prepared != nil && len(grants.Prepared.SlotsByLevel) > 0 {
		return false
	}
	if grants.Spontaneous != nil && len(grants.Spontaneous.KnownByLevel) > 0 {
		return false
	}
	if len(grants.Hardwired) > 0 {
		return false
	}
	return true
}

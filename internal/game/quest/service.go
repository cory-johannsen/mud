package quest

import (
	"context"
	"fmt"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// XPAwarder awards a pre-computed XP amount to a player.
//
// Precondition: sess must be non-nil; xpAmount >= 0.
type XPAwarder interface {
	AwardXPAmount(ctx context.Context, sess SessionState, characterID int64, xpAmount int) ([]string, error)
}

// InventorySaver persists backpack contents and currency for a character.
//
// Precondition: characterID > 0.
type InventorySaver interface {
	SaveInventory(ctx context.Context, characterID int64, items []inventory.InventoryItem) error
	SaveCurrency(ctx context.Context, characterID int64, currency int) error
}

// SessionState is the minimal set of quest-relevant fields on a player session.
// session.PlayerSession must satisfy this interface (enforced at the call site in gameserver).
//
// Precondition: all map fields must be non-nil before passing to Service methods.
type SessionState interface {
	GetActiveQuests() map[string]*ActiveQuest
	GetCompletedQuests() map[string]*time.Time
	GetBackpack() *inventory.Backpack
	GetCurrency() int
	AddCurrency(delta int)
}

// Service orchestrates quest lifecycle: offering, accepting, progress tracking, completion, and abandonment.
type Service struct {
	registry    QuestRegistry
	repo        QuestRepository
	xpSvc       XPAwarder
	invRegistry *inventory.Registry
	charSaver   InventorySaver
}

// NewService creates a new quest Service.
//
// Precondition: registry and repo must be non-nil. xpSvc, invRegistry, charSaver may be nil
// (rewards will be skipped when nil).
// Postcondition: returns a non-nil Service ready for use.
func NewService(
	registry QuestRegistry,
	repo QuestRepository,
	xpSvc XPAwarder,
	invRegistry *inventory.Registry,
	charSaver InventorySaver,
) *Service {
	return &Service{
		registry:    registry,
		repo:        repo,
		xpSvc:       xpSvc,
		invRegistry: invRegistry,
		charSaver:   charSaver,
	}
}

// SetXPAwarder wires an XPAwarder into the service after construction.
//
// Precondition: awarder must be non-nil.
// Postcondition: subsequent quest completions will call awarder.AwardXPAmount for XP rewards.
func (s *Service) SetXPAwarder(awarder XPAwarder) {
	s.xpSvc = awarder
}

// Registry returns the quest registry.
//
// Postcondition: Returns the QuestRegistry this Service was initialized with (may be empty but not nil).
func (s *Service) Registry() QuestRegistry {
	return s.registry
}

// LoadQuests delegates to the repository.
//
// Precondition: characterID > 0.
// Postcondition: returns all QuestRecords for the character, or an error.
func (s *Service) LoadQuests(ctx context.Context, characterID int64) ([]QuestRecord, error) {
	return s.repo.LoadQuests(ctx, characterID)
}

// HydrateSession populates sess.ActiveQuests and sess.CompletedQuests from the given records.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.ActiveQuests and sess.CompletedQuests are populated from records.
func (s *Service) HydrateSession(sess SessionState, records []QuestRecord) {
	for _, r := range records {
		switch r.Status {
		case "active":
			progress := r.Progress
			if progress == nil {
				progress = make(map[string]int)
			}
			sess.GetActiveQuests()[r.QuestID] = &ActiveQuest{
				QuestID:           r.QuestID,
				ObjectiveProgress: progress,
			}
		case "completed":
			sess.GetCompletedQuests()[r.QuestID] = r.CompletedAt
		case "abandoned":
			// nil sentinel: present in CompletedQuests with nil time.
			sess.GetCompletedQuests()[r.QuestID] = nil
		}
	}
}

// GetOfferable returns all QuestDefs from questIDs that are eligible for the given player.
// A quest is offerable if:
//   - It is not currently active on the session.
//   - All prerequisites are completed (non-nil completedAt in CompletedQuests).
//   - It is not a completed non-repeatable quest.
//   - For repeatable quests with cooldown, the cooldown has elapsed since last completion.
//
// Precondition: sess must be non-nil.
// Postcondition: returned slice contains only defs the player may accept now.
func (s *Service) GetOfferable(sess SessionState, questIDs []string) []*QuestDef {
	active := sess.GetActiveQuests()
	completed := sess.GetCompletedQuests()
	var out []*QuestDef
	for _, id := range questIDs {
		def, ok := s.registry[id]
		if !ok {
			continue
		}
		if _, isActive := active[id]; isActive {
			continue
		}
		prereqsMet := true
		for _, prereq := range def.Prerequisites {
			completedAt, present := completed[prereq]
			if !present || completedAt == nil {
				prereqsMet = false
				break
			}
		}
		if !prereqsMet {
			continue
		}
		if completedAt, done := completed[id]; done {
			if !def.Repeatable {
				continue
			}
			if def.Cooldown != "" && completedAt != nil {
				cooldown, _ := time.ParseDuration(def.Cooldown)
				if time.Since(*completedAt) < cooldown {
					continue
				}
			}
		}
		out = append(out, def)
	}
	return out
}

// Accept marks a quest as accepted for the player, initialises objective progress, and persists.
//
// Precondition: sess non-nil; questID must exist in registry and not already be active.
// Postcondition: sess.ActiveQuests[questID] is populated; repo.SaveQuestStatus called with "active".
// Returns the quest title, a slice of objective descriptions, and any error.
func (s *Service) Accept(ctx context.Context, sess SessionState, characterID int64, questID string) (string, []string, error) {
	def, ok := s.registry[questID]
	if !ok {
		return "", nil, fmt.Errorf("quest %q not found", questID)
	}
	if _, active := sess.GetActiveQuests()[questID]; active {
		return "", nil, fmt.Errorf("quest %q is already active", questID)
	}
	progress := make(map[string]int, len(def.Objectives))
	for _, obj := range def.Objectives {
		progress[obj.ID] = 0
	}
	sess.GetActiveQuests()[questID] = &ActiveQuest{
		QuestID:           questID,
		ObjectiveProgress: progress,
	}
	if err := s.repo.SaveQuestStatus(ctx, characterID, questID, "active", nil); err != nil {
		return "", nil, fmt.Errorf("saving quest status: %w", err)
	}
	objDescs := make([]string, len(def.Objectives))
	for i, obj := range def.Objectives {
		objDescs[i] = obj.Description
	}
	return def.Title, objDescs, nil
}

// RecordKill increments progress for all active kill objectives matching npcTemplateID.
//
// Precondition: sess non-nil; npcTemplateID non-empty.
// Postcondition: matching objectives are incremented (clamped at Quantity); maybeComplete called.
// Returns completion messages if a quest completed, or nil if none completed.
func (s *Service) RecordKill(ctx context.Context, sess SessionState, characterID int64, npcTemplateID string) ([]string, error) {
	return s.recordProgress(ctx, sess, characterID, func(obj QuestObjective) bool {
		return obj.Type == "kill" && obj.TargetID == npcTemplateID
	})
}

// RecordKillWithResults increments progress for kill objectives matching npcTemplateID
// and returns structured CompletionResult data for any quests that completed.
//
// Precondition: sess non-nil; npcTemplateID non-empty.
// Postcondition: matching objectives are incremented; completed quests are removed from ActiveQuests.
// Returns ([]*CompletionResult, []string messages, error).
func (s *Service) RecordKillWithResults(ctx context.Context, sess SessionState, characterID int64, npcTemplateID string) ([]*CompletionResult, []string, error) {
	return s.recordProgressWithResults(ctx, sess, characterID, 1, func(obj QuestObjective) bool {
		return obj.Type == "kill" && obj.TargetID == npcTemplateID
	})
}

// recordProgressWithResults increments by n all matching objectives across active quests
// and returns structured completion results alongside messages.
func (s *Service) recordProgressWithResults(ctx context.Context, sess SessionState, characterID int64, n int, match func(QuestObjective) bool) ([]*CompletionResult, []string, error) {
	var allResults []*CompletionResult
	var allMsgs []string
	for questID, aq := range sess.GetActiveQuests() {
		def, ok := s.registry[questID]
		if !ok {
			continue
		}
		changed := false
		for _, obj := range def.Objectives {
			if !match(obj) {
				continue
			}
			current := aq.ObjectiveProgress[obj.ID]
			if current >= obj.Quantity {
				continue
			}
			next := current + n
			if next > obj.Quantity {
				next = obj.Quantity
			}
			aq.ObjectiveProgress[obj.ID] = next
			changed = true
			if err := s.repo.SaveObjectiveProgress(ctx, characterID, questID, obj.ID, next); err != nil {
				return nil, nil, fmt.Errorf("saving objective progress: %w", err)
			}
		}
		if changed {
			msgs, err := s.maybeComplete(ctx, sess, characterID, questID)
			if err != nil {
				return nil, nil, err
			}
			if len(msgs) > 0 {
				// Quest completed — build a CompletionResult.
				result := &CompletionResult{
					QuestID:       def.ID,
					Title:         def.Title,
					XPReward:      def.Rewards.XP,
					CreditsReward: def.Rewards.Credits,
				}
				for _, ri := range def.Rewards.Items {
					name := ri.ItemID
					if s.invRegistry != nil {
						if itemDef, ok := s.invRegistry.Item(ri.ItemID); ok {
							name = itemDef.Name
						}
					}
					result.ItemRewards = append(result.ItemRewards, fmt.Sprintf("%s x%d", name, ri.Quantity))
				}
				allResults = append(allResults, result)
				allMsgs = append(allMsgs, msgs...)
			}
		}
	}
	return allResults, allMsgs, nil
}

// RecordFetch increments progress for all active fetch objectives matching itemDefID.
//
// Precondition: sess non-nil; itemDefID non-empty; qty >= 1.
// Postcondition: matching objectives are incremented by qty (clamped at Quantity); maybeComplete called.
// Returns completion messages if a quest completed, or nil if none completed.
func (s *Service) RecordFetch(ctx context.Context, sess SessionState, characterID int64, itemDefID string, qty int) ([]string, error) {
	return s.recordProgressN(ctx, sess, characterID, qty, func(obj QuestObjective) bool {
		return obj.Type == "fetch" && obj.TargetID == itemDefID
	})
}

// RecordExplore increments progress for all active explore objectives matching roomID.
//
// Precondition: sess non-nil; roomID non-empty.
// Postcondition: matching objectives are incremented (clamped at Quantity); maybeComplete called.
// Returns completion messages if a quest completed, or nil if none completed.
func (s *Service) RecordExplore(ctx context.Context, sess SessionState, characterID int64, roomID string) ([]string, error) {
	return s.recordProgress(ctx, sess, characterID, func(obj QuestObjective) bool {
		return obj.Type == "explore" && obj.TargetID == roomID
	})
}

// RecordDeliver marks a specific deliver objective as fully complete.
//
// Precondition: sess non-nil; questID and objectiveID non-empty.
// Postcondition: the objective is set to its Quantity value; maybeComplete called.
// Returns completion messages if the quest completed, or nil if it did not.
func (s *Service) RecordDeliver(ctx context.Context, sess SessionState, characterID int64, questID, objectiveID string) ([]string, error) {
	aq, ok := sess.GetActiveQuests()[questID]
	if !ok {
		return nil, fmt.Errorf("quest %q is not active", questID)
	}
	def, ok := s.registry[questID]
	if !ok {
		return nil, fmt.Errorf("quest %q not found in registry", questID)
	}
	for _, obj := range def.Objectives {
		if obj.ID == objectiveID && obj.Type == "deliver" {
			if aq.ObjectiveProgress[obj.ID] < obj.Quantity {
				aq.ObjectiveProgress[obj.ID] = obj.Quantity
				if err := s.repo.SaveObjectiveProgress(ctx, characterID, questID, obj.ID, obj.Quantity); err != nil {
					return nil, fmt.Errorf("saving objective progress: %w", err)
				}
			}
			return s.maybeComplete(ctx, sess, characterID, questID)
		}
	}
	return nil, fmt.Errorf("objective %q not found in quest %q", objectiveID, questID)
}

// recordProgress increments by 1 all matching objectives across active quests.
func (s *Service) recordProgress(ctx context.Context, sess SessionState, characterID int64, match func(QuestObjective) bool) ([]string, error) {
	return s.recordProgressN(ctx, sess, characterID, 1, match)
}

// recordProgressN increments by n all matching objectives across active quests.
// Returns completion messages from any quests that completed as a result.
func (s *Service) recordProgressN(ctx context.Context, sess SessionState, characterID int64, n int, match func(QuestObjective) bool) ([]string, error) {
	var allMsgs []string
	for questID, aq := range sess.GetActiveQuests() {
		def, ok := s.registry[questID]
		if !ok {
			continue
		}
		changed := false
		for _, obj := range def.Objectives {
			if !match(obj) {
				continue
			}
			current := aq.ObjectiveProgress[obj.ID]
			if current >= obj.Quantity {
				continue
			}
			next := current + n
			if next > obj.Quantity {
				next = obj.Quantity
			}
			aq.ObjectiveProgress[obj.ID] = next
			changed = true
			if err := s.repo.SaveObjectiveProgress(ctx, characterID, questID, obj.ID, next); err != nil {
				return nil, fmt.Errorf("saving objective progress: %w", err)
			}
		}
		if changed {
			msgs, err := s.maybeComplete(ctx, sess, characterID, questID)
			if err != nil {
				return nil, err
			}
			allMsgs = append(allMsgs, msgs...)
		}
	}
	return allMsgs, nil
}

// maybeComplete completes the quest if all objectives are satisfied.
//
// Precondition: questID must be active on sess.
// Postcondition: if all objectives met, Complete is called and its messages are returned.
func (s *Service) maybeComplete(ctx context.Context, sess SessionState, characterID int64, questID string) ([]string, error) {
	aq, ok := sess.GetActiveQuests()[questID]
	if !ok {
		return nil, nil
	}
	def, ok := s.registry[questID]
	if !ok {
		return nil, nil
	}
	for _, obj := range def.Objectives {
		if aq.ObjectiveProgress[obj.ID] < obj.Quantity {
			return nil, nil
		}
	}
	return s.Complete(ctx, sess, characterID, questID)
}

// Complete finalises a quest: awards XP/items/credits, moves to CompletedQuests, persists.
//
// Precondition: questID must be active on sess.
// Postcondition: quest removed from ActiveQuests, added to CompletedQuests with non-nil time;
// SaveQuestStatus called with "completed".
// Returns player-facing completion messages.
func (s *Service) Complete(ctx context.Context, sess SessionState, characterID int64, questID string) ([]string, error) {
	if _, ok := sess.GetActiveQuests()[questID]; !ok {
		return nil, fmt.Errorf("quest %q is not active", questID)
	}
	def, ok := s.registry[questID]
	if !ok {
		return nil, fmt.Errorf("quest %q not found in registry", questID)
	}
	now := time.Now()
	if err := s.repo.SaveQuestStatus(ctx, characterID, questID, "completed", &now); err != nil {
		return nil, fmt.Errorf("saving completed quest status: %w", err)
	}
	delete(sess.GetActiveQuests(), questID)
	sess.GetCompletedQuests()[questID] = &now

	// Award XP if service is wired. Capture level-up messages to return to caller.
	var levelUpMsgs []string
	if s.xpSvc != nil && def.Rewards.XP > 0 {
		var err error
		levelUpMsgs, err = s.xpSvc.AwardXPAmount(ctx, sess, characterID, def.Rewards.XP)
		if err != nil {
			return nil, fmt.Errorf("awarding quest XP: %w", err)
		}
	}

	// Award credits if charSaver is wired.
	if s.charSaver != nil && def.Rewards.Credits > 0 {
		sess.AddCurrency(def.Rewards.Credits)
		if err := s.charSaver.SaveCurrency(ctx, characterID, sess.GetCurrency()); err != nil {
			return nil, fmt.Errorf("saving quest currency reward: %w", err)
		}
	}

	// Award item rewards if both registries are wired.
	if s.charSaver != nil && s.invRegistry != nil && sess.GetBackpack() != nil && len(def.Rewards.Items) > 0 {
		for _, reward := range def.Rewards.Items {
			if _, err := sess.GetBackpack().Add(reward.ItemID, reward.Quantity, s.invRegistry); err != nil {
				return nil, fmt.Errorf("adding quest reward item %q: %w", reward.ItemID, err)
			}
		}
		if err := s.charSaver.SaveInventory(ctx, characterID, backpackToInventoryItems(sess.GetBackpack())); err != nil {
			return nil, fmt.Errorf("saving inventory after quest rewards: %w", err)
		}
	}

	return append(CompletionMessage(def, s.invRegistry), levelUpMsgs...), nil
}

// Abandon removes a quest from the player's active quests.
// For non-repeatable quests, confirm must be true; without it a prompt message is returned.
// For repeatable quests, confirm is not required.
//
// Precondition: questID must be active on sess.
// Postcondition (confirmed): quest removed from ActiveQuests; for non-repeatable, added to
// CompletedQuests with nil sentinel; SaveQuestStatus called with "abandoned".
// Postcondition (not confirmed, non-repeatable): no state change; prompt message returned.
func (s *Service) Abandon(ctx context.Context, sess SessionState, characterID int64, questID string, confirm bool) (string, error) {
	if _, ok := sess.GetActiveQuests()[questID]; !ok {
		return "", fmt.Errorf("quest %q is not active", questID)
	}
	def, ok := s.registry[questID]
	if !ok {
		return "", fmt.Errorf("quest %q not found in registry", questID)
	}
	if !def.Repeatable && !confirm {
		return fmt.Sprintf("Abandoning %q is permanent and cannot be retaken. Type 'abandon %s confirm' to proceed.", def.Title, questID), nil
	}
	if err := s.repo.SaveQuestStatus(ctx, characterID, questID, "abandoned", nil); err != nil {
		return "", fmt.Errorf("saving abandoned quest status: %w", err)
	}
	delete(sess.GetActiveQuests(), questID)
	if !def.Repeatable {
		sess.GetCompletedQuests()[questID] = nil
	}
	return fmt.Sprintf("You have abandoned %q.", def.Title), nil
}

// CompletionMessage returns player-facing lines announcing quest completion and rewards.
//
// Precondition: def must be non-nil.
// Postcondition: returned slice is non-empty.
func CompletionMessage(def *QuestDef, invRegistry *inventory.Registry) []string {
	msgs := []string{
		fmt.Sprintf("Quest complete: %s", def.Title),
	}
	if def.Rewards.XP > 0 {
		msgs = append(msgs, fmt.Sprintf("  XP reward: %d", def.Rewards.XP))
	}
	if def.Rewards.Credits > 0 {
		msgs = append(msgs, fmt.Sprintf("  Credits reward: %d", def.Rewards.Credits))
	}
	for _, ri := range def.Rewards.Items {
		name := ri.ItemID
		if invRegistry != nil {
			if itemDef, ok := invRegistry.Item(ri.ItemID); ok {
				name = itemDef.Name
			}
		}
		msgs = append(msgs, fmt.Sprintf("  Item reward: %s x%d", name, ri.Quantity))
	}
	return msgs
}

// backpackToInventoryItems converts a Backpack's contents to a slice of InventoryItem for persistence.
//
// Precondition: bp must be non-nil.
// Postcondition: returned slice has one entry per ItemInstance in the backpack.
func backpackToInventoryItems(bp *inventory.Backpack) []inventory.InventoryItem {
	instances := bp.Items()
	out := make([]inventory.InventoryItem, len(instances))
	for i, inst := range instances {
		out[i] = inventory.InventoryItem{
			ItemDefID: inst.ItemDefID,
			Quantity:  inst.Quantity,
		}
	}
	return out
}

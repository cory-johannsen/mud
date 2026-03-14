package xp

import (
	"context"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// ProgressSaver persists character progress after a level-up.
//
// Precondition: characterID must be > 0.
type ProgressSaver interface {
	SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error
}

// SkillIncreaseSaver persists pending skill increases awarded at level-up.
//
// Precondition: characterID must be > 0; n must be >= 1.
type SkillIncreaseSaver interface {
	IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error
}

// Service orchestrates XP awards and level-up detection.
type Service struct {
	cfg         *XPConfig
	saver       ProgressSaver
	skillSaver  SkillIncreaseSaver
}

// NewService creates a new XP Service.
//
// Precondition: cfg and saver must be non-nil.
// Postcondition: Returns a non-nil Service ready to award XP.
func NewService(cfg *XPConfig, saver ProgressSaver) *Service {
	return &Service{cfg: cfg, saver: saver}
}

// SetSkillIncreaseSaver registers the saver for pending skill increases.
//
// Postcondition: skill increases will be persisted via saver on each level-up that grants them.
func (s *Service) SetSkillIncreaseSaver(saver SkillIncreaseSaver) {
	s.skillSaver = saver
}

// Config returns the XPConfig used by this Service.
//
// Postcondition: Returns a non-nil *XPConfig.
func (s *Service) Config() *XPConfig {
	return s.cfg
}

// AwardKill awards XP for killing an NPC of the given level.
// Returns player-facing notification messages (level-up announcement, boost prompt).
//
// Precondition: sess must be non-nil; npcLevel >= 1.
// Postcondition: sess.Experience, sess.Level, sess.MaxHP updated in-place;
// SaveProgress called and persisted when a level-up occurs.
func (s *Service) AwardKill(ctx context.Context, sess *session.PlayerSession, npcLevel int, characterID int64) ([]string, error) {
	return s.award(ctx, sess, characterID, npcLevel*s.cfg.Awards.KillXPPerNPCLevel)
}

// AwardXPAmount awards a pre-computed XP amount directly to a player.
// Use this instead of AwardKill when splitting a kill reward across multiple
// participants, to avoid re-multiplying by KillXPPerNPCLevel.
//
// Precondition: sess non-nil; xpAmount >= 0.
// Postcondition: same as AwardKill — XP, level, HP updated in-place; persisted.
func (s *Service) AwardXPAmount(ctx context.Context, sess *session.PlayerSession, characterID int64, xpAmount int) ([]string, error) {
	return s.award(ctx, sess, characterID, xpAmount)
}

// AwardRoomDiscovery awards XP for entering a previously unseen room.
// Always returns at least one message (the XP grant) prepended before any level-up messages.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardRoomDiscovery(ctx context.Context, sess *session.PlayerSession, characterID int64) ([]string, error) {
	xpAmount := s.cfg.Awards.NewRoomXP
	levelMsgs, err := s.award(ctx, sess, characterID, xpAmount)
	if err != nil {
		return nil, err
	}
	grant := fmt.Sprintf("You gain %d XP for discovering a new room.", xpAmount)
	return append([]string{grant}, levelMsgs...), nil
}

// AwardSkillCheck awards XP for a successful or crit-successful skill check.
// Always returns at least one message (the XP grant) prepended before any level-up messages.
// Set isCrit=true for crit_success outcomes.
//
// Precondition: sess must be non-nil; skillName must be non-empty; dc >= 0.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardSkillCheck(ctx context.Context, sess *session.PlayerSession, skillName string, dc int, isCrit bool, characterID int64) ([]string, error) {
	base := s.cfg.Awards.SkillCheckSuccessXP
	if isCrit {
		base = s.cfg.Awards.SkillCheckCritSuccessXP
	}
	xpAmount := base + dc*s.cfg.Awards.SkillCheckDCMultiplier
	levelMsgs, err := s.award(ctx, sess, characterID, xpAmount)
	if err != nil {
		return nil, err
	}
	grant := fmt.Sprintf("You gain %d XP for the %s check.", xpAmount, skillName)
	return append([]string{grant}, levelMsgs...), nil
}

// award applies awardXP to sess and handles level-up side effects.
func (s *Service) award(ctx context.Context, sess *session.PlayerSession, characterID int64, awardXP int) ([]string, error) {
	result := Award(sess.Level, sess.Experience, awardXP, s.cfg)
	sess.Experience = result.NewXP
	sess.Level = result.NewLevel
	sess.MaxHP += result.HPGained
	if sess.CurrentHP > sess.MaxHP {
		sess.CurrentHP = sess.MaxHP
	}
	sess.PendingBoosts += result.NewBoosts
	sess.PendingSkillIncreases += result.NewSkillIncreases

	// Always persist XP, even when no level-up occurred.
	if characterID > 0 {
		if err := s.saver.SaveProgress(ctx, characterID, sess.Level, sess.Experience, sess.MaxHP, sess.PendingBoosts); err != nil {
			return nil, fmt.Errorf("saving XP progress: %w", err)
		}
	}

	if !result.LeveledUp {
		return nil, nil
	}

	var msgs []string
	msgs = append(msgs, fmt.Sprintf("*** You reached level %d! ***", result.NewLevel))
	if result.HPGained > 0 {
		msgs = append(msgs, fmt.Sprintf("Max HP increased by %d (now %d).", result.HPGained, sess.MaxHP))
	}
	if result.NewBoosts > 0 {
		msgs = append(msgs, "You have a pending ability boost! Type 'levelup' to assign it.")
	}
	if result.NewSkillIncreases > 0 {
		msgs = append(msgs, "You have a pending skill increase! Type 'trainskill <skill>' to advance a skill.")
	}

	// SaveProgress was already called above (always); only persist skill increases here.
	if characterID > 0 && result.NewSkillIncreases > 0 && s.skillSaver != nil {
		if err := s.skillSaver.IncrementPendingSkillIncreases(ctx, characterID, result.NewSkillIncreases); err != nil {
			return msgs, fmt.Errorf("saving skill increases after level-up: %w", err)
		}
	}

	return msgs, nil
}

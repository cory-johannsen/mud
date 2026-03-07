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

// Service orchestrates XP awards and level-up detection.
type Service struct {
	cfg   *XPConfig
	saver ProgressSaver
}

// NewService creates a new XP Service.
//
// Precondition: cfg and saver must be non-nil.
// Postcondition: Returns a non-nil Service ready to award XP.
func NewService(cfg *XPConfig, saver ProgressSaver) *Service {
	return &Service{cfg: cfg, saver: saver}
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

// AwardRoomDiscovery awards XP for entering a previously unseen room.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardRoomDiscovery(ctx context.Context, sess *session.PlayerSession, characterID int64) ([]string, error) {
	return s.award(ctx, sess, characterID, s.cfg.Awards.NewRoomXP)
}

// AwardSkillCheck awards XP for a successful or crit-successful skill check.
// Set isCrit=true for crit_success outcomes.
//
// Precondition: sess must be non-nil; dc >= 0.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardSkillCheck(ctx context.Context, sess *session.PlayerSession, dc int, isCrit bool, characterID int64) ([]string, error) {
	base := s.cfg.Awards.SkillCheckSuccessXP
	if isCrit {
		base = s.cfg.Awards.SkillCheckCritSuccessXP
	}
	return s.award(ctx, sess, characterID, base+dc*s.cfg.Awards.SkillCheckDCMultiplier)
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

	if characterID > 0 {
		if err := s.saver.SaveProgress(ctx, characterID, sess.Level, sess.Experience, sess.MaxHP, sess.PendingBoosts); err != nil {
			return msgs, fmt.Errorf("saving progress after level-up: %w", err)
		}
	}

	return msgs, nil
}

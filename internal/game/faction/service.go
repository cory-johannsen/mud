package faction

import (
	"context"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// Repository persists and loads per-character faction reputation.
//
// Precondition: All arguments must be valid (characterID > 0, factionID non-empty).
// Postcondition: Mutations are durably persisted before returning.
type Repository interface {
	SaveRep(ctx context.Context, characterID int64, factionID string, rep int) error
	LoadRep(ctx context.Context, characterID int64) (map[string]int, error)
}

// Service provides faction logic operations.
type Service struct {
	reg  FactionRegistry
	repo Repository
}

// NewService creates a Service with the given registry and optional repository.
//
// Precondition: reg must be non-nil.
// Postcondition: Returns a non-nil *Service.
func NewService(reg FactionRegistry) *Service {
	return &Service{reg: reg}
}

// NewServiceWithRepo creates a Service with registry and persistence.
//
// Precondition: reg and repo must be non-nil.
// Postcondition: Returns a non-nil *Service.
func NewServiceWithRepo(reg FactionRegistry, repo Repository) *Service {
	return &Service{reg: reg, repo: repo}
}

// TierFor returns the highest FactionTier whose MinRep <= rep.
//
// Precondition: factionID must be a key in the registry.
// Postcondition: Returns nil only if factionID is unknown; returns first tier if rep is 0.
func (s *Service) TierFor(factionID string, rep int) *FactionTier {
	def, ok := s.reg[factionID]
	if !ok || len(def.Tiers) == 0 {
		return nil
	}
	best := &def.Tiers[0]
	for i := range def.Tiers {
		if def.Tiers[i].MinRep <= rep {
			best = &def.Tiers[i]
		}
	}
	return best
}

// tierIndex returns the 0-based index of the tier with the given ID within the faction, or -1.
func (s *Service) tierIndex(factionID, tierID string) int {
	def, ok := s.reg[factionID]
	if !ok {
		return -1
	}
	for i, t := range def.Tiers {
		if t.ID == tierID {
			return i
		}
	}
	return -1
}

// IsHostile returns true iff factionB appears in factionA's HostileFactions list.
//
// Precondition: none.
// Postcondition: Returns false for unknown faction IDs.
func (s *Service) IsHostile(factionA, factionB string) bool {
	def, ok := s.reg[factionA]
	if !ok {
		return false
	}
	for _, hf := range def.HostileFactions {
		if hf == factionB {
			return true
		}
	}
	return false
}

// DiscountFor returns the price discount fraction for the given faction and rep score.
//
// Precondition: factionID must exist in the registry.
// Postcondition: Returns 0 for unknown factions.
func (s *Service) DiscountFor(factionID string, rep int) float64 {
	tier := s.TierFor(factionID, rep)
	if tier == nil {
		return 0
	}
	return tier.PriceDiscount
}

// IsEnemyOf returns true iff npcFactionID is non-empty and is hostile to sess.FactionID.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns false when npcFactionID is empty.
func (s *Service) IsEnemyOf(sess *session.PlayerSession, npcFactionID string) bool {
	if npcFactionID == "" {
		return false
	}
	return s.IsHostile(npcFactionID, sess.FactionID)
}

// IsAllyOf returns true iff both npcFactionID and sess.FactionID are non-empty
// and are equal (same faction).
//
// Precondition: sess must be non-nil.
// Postcondition: Returns false when either faction ID is empty.
func (s *Service) IsAllyOf(sess *session.PlayerSession, npcFactionID string) bool {
	if npcFactionID == "" || sess.FactionID == "" {
		return false
	}
	return npcFactionID == sess.FactionID
}

// CanEnterRoom returns true if the room has no faction gating, or if the player's
// faction matches the zone owner and the player has sufficient tier.
//
// Precondition: sess, room, and zone must be non-nil.
// Postcondition: Returns true for ungated rooms.
func (s *Service) CanEnterRoom(sess *session.PlayerSession, room *world.Room, zone *world.Zone) bool {
	if room.MinFactionTierID == "" {
		return true
	}
	if zone.FactionID != sess.FactionID {
		return false
	}
	playerTier := s.TierFor(sess.FactionID, sess.FactionRep[sess.FactionID])
	if playerTier == nil {
		return false
	}
	playerIdx := s.tierIndex(sess.FactionID, playerTier.ID)
	requiredIdx := s.tierIndex(sess.FactionID, room.MinFactionTierID)
	return playerIdx >= requiredIdx
}

// CanBuyItem returns false if the item is in any faction's exclusive list
// and the player has not reached the required tier within that faction.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns true for items not in any exclusive list.
func (s *Service) CanBuyItem(sess *session.PlayerSession, itemDefID string) bool {
	for factionID, def := range s.reg {
		for _, ei := range def.ExclusiveItems {
			for _, iid := range ei.ItemIDs {
				if iid != itemDefID {
					continue
				}
				if factionID != sess.FactionID {
					return false
				}
				playerTier := s.TierFor(sess.FactionID, sess.FactionRep[sess.FactionID])
				if playerTier == nil {
					return false
				}
				playerIdx := s.tierIndex(sess.FactionID, playerTier.ID)
				requiredIdx := s.tierIndex(sess.FactionID, ei.TierID)
				return playerIdx >= requiredIdx
			}
		}
	}
	return true
}

// AwardRep increases sess.FactionRep[factionID] by amount, persists via repo,
// and returns a tier-up message if a threshold is crossed.
//
// Precondition: sess must be non-nil; factionID must be non-empty; amount must be > 0.
// Postcondition: sess.FactionRep[factionID] is updated; returns "" if no tier change.
func (s *Service) AwardRep(ctx context.Context, sess *session.PlayerSession, characterID int64, factionID string, amount int) (string, error) {
	if amount <= 0 {
		return "", nil
	}
	def, ok := s.reg[factionID]
	if !ok {
		return "", fmt.Errorf("faction.AwardRep: unknown faction %q", factionID)
	}
	oldRep := sess.FactionRep[factionID]
	oldTier := s.TierFor(factionID, oldRep)
	newRep := oldRep + amount
	sess.FactionRep[factionID] = newRep
	if s.repo != nil {
		if err := s.repo.SaveRep(ctx, characterID, factionID, newRep); err != nil {
			return "", fmt.Errorf("faction.AwardRep: saving rep: %w", err)
		}
	}
	newTier := s.TierFor(factionID, newRep)
	if newTier != nil && oldTier != nil && newTier.ID != oldTier.ID {
		return fmt.Sprintf("You are now a %s of %s!", newTier.Label, def.Name), nil
	}
	return "", nil
}

// MinTierLabelForRoom returns the label of the minimum tier required to enter the room
// and the faction name, for use in blocked-entry messages.
//
// Precondition: room.MinFactionTierID must be non-empty; zone.FactionID must be non-empty.
// Postcondition: Returns ("", "") if the faction or tier is unknown.
func (s *Service) MinTierLabelForRoom(room *world.Room, zone *world.Zone) (tierLabel, factionName string) {
	def, ok := s.reg[zone.FactionID]
	if !ok {
		return "", ""
	}
	for _, t := range def.Tiers {
		if t.ID == room.MinFactionTierID {
			return t.Label, def.Name
		}
	}
	return "", ""
}

// ExclusiveTierLabel returns the tier label and faction name for an exclusive item,
// used in blocked-purchase messages.
//
// Precondition: none.
// Postcondition: Returns ("", "") if not found.
func (s *Service) ExclusiveTierLabel(itemDefID string) (tierLabel, factionName string) {
	for _, def := range s.reg {
		for _, ei := range def.ExclusiveItems {
			for _, iid := range ei.ItemIDs {
				if iid == itemDefID {
					for _, t := range def.Tiers {
						if t.ID == ei.TierID {
							return t.Label, def.Name
						}
					}
				}
			}
		}
	}
	return "", ""
}

// ExclusiveTierSuffix returns the "[<TierLabel>]" suffix for browse display if the item
// is exclusive, or "" if it is not.
//
// Precondition: none.
// Postcondition: Returns "" for non-exclusive items.
func (s *Service) ExclusiveTierSuffix(itemDefID string) string {
	label, _ := s.ExclusiveTierLabel(itemDefID)
	if label == "" {
		return ""
	}
	return fmt.Sprintf("[%s]", label)
}

// CurrentTierIndex returns the 1-based tier index (1=first, 4=max) for the player's
// current faction rep, used by change_rep cost calculation.
//
// Precondition: sess.FactionID must be non-empty.
// Postcondition: Returns 1 for outsider; 4 for max tier.
func (s *Service) CurrentTierIndex(sess *session.PlayerSession) int {
	tier := s.TierFor(sess.FactionID, sess.FactionRep[sess.FactionID])
	if tier == nil {
		return 1
	}
	idx := s.tierIndex(sess.FactionID, tier.ID)
	if idx < 0 {
		return 1
	}
	return idx + 1
}

// IsAtMaxTier returns true iff the player is at the last tier in their faction.
//
// Precondition: sess.FactionID must be non-empty.
// Postcondition: Returns false for unknown factions.
func (s *Service) IsAtMaxTier(sess *session.PlayerSession) bool {
	def, ok := s.reg[sess.FactionID]
	if !ok {
		return false
	}
	return s.CurrentTierIndex(sess) >= len(def.Tiers)
}

// NextTier returns the next FactionTier above the player's current tier, or nil at max.
//
// Precondition: sess.FactionID must be non-empty.
// Postcondition: Returns nil if already at max tier or faction unknown.
func (s *Service) NextTier(sess *session.PlayerSession) *FactionTier {
	def, ok := s.reg[sess.FactionID]
	if !ok {
		return nil
	}
	idx := s.CurrentTierIndex(sess) // 1-based
	if idx >= len(def.Tiers) {
		return nil
	}
	return &def.Tiers[idx]
}

package gameserver

import (
	"context"
	"sort"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ApplyFeatGrant grants all feats described by grants to characterID. Fixed feats
// are granted immediately; choice feats are auto-assigned by picking the first N
// pool entries not already owned (pool order determines priority).
//
// existing is a live map[featID]bool that is updated in place for dedup.
// preExisting is the snapshot of feats owned before this backfill run started;
// it is used to determine how many pool feats the character already "has credit for"
// from persistent storage. This allows grants from the current run to be skipped
// without double-counting across pools.
// featReg may be nil — feat existence validation is skipped when nil.
//
// Precondition: characterID > 0; grants non-nil; featsRepo non-nil.
// Postcondition: Returns the feat IDs that were newly added. existing is updated.
func ApplyFeatGrant(
	ctx context.Context,
	characterID int64,
	existing map[string]bool,
	grants *ruleset.FeatGrants,
	featReg *ruleset.FeatRegistry,
	featsRepo CharacterFeatsRepo,
) ([]string, error) {
	return applyFeatGrantWithBaseline(ctx, characterID, existing, existing, grants, featReg, featsRepo)
}

// applyFeatGrantWithBaseline is the internal implementation that separates the
// "already satisfied" count (uses preExisting snapshot) from the dedup guard
// (uses existing live map). This ensures that feats granted in the current run
// do not count toward satisfying subsequent pool grants from the same backfill.
func applyFeatGrantWithBaseline(
	ctx context.Context,
	characterID int64,
	existing map[string]bool,
	preExisting map[string]bool,
	grants *ruleset.FeatGrants,
	featReg *ruleset.FeatRegistry,
	featsRepo CharacterFeatsRepo,
) ([]string, error) {
	var granted []string

	// Fixed feats: grant any not already owned.
	for _, id := range grants.Fixed {
		if existing[id] {
			continue
		}
		if featReg != nil {
			if _, ok := featReg.Feat(id); !ok {
				continue // skip unknown feats
			}
		}
		if err := featsRepo.Add(ctx, characterID, id); err != nil {
			return granted, err
		}
		existing[id] = true
		granted = append(granted, id)
	}

	// Choice feats: count how many pool entries appear in preExisting (the snapshot
	// loaded from DB at backfill start). This ensures feats granted earlier in the
	// same backfill run do not prematurely satisfy this level's pool count.
	if grants.Choices != nil && grants.Choices.Count > 0 {
		alreadyOwned := 0
		for _, id := range grants.Choices.Pool {
			if preExisting[id] {
				alreadyOwned++
			}
		}
		remaining := grants.Choices.Count - alreadyOwned
		for _, id := range grants.Choices.Pool {
			if remaining <= 0 {
				break
			}
			if existing[id] {
				continue // dedup: skip if already granted (even in this run)
			}
			if featReg != nil {
				if _, ok := featReg.Feat(id); !ok {
					continue
				}
			}
			if err := featsRepo.Add(ctx, characterID, id); err != nil {
				return granted, err
			}
			existing[id] = true
			granted = append(granted, id)
			remaining--
		}
	}

	return granted, nil
}

// BackfillLevelUpFeats retroactively applies all feat level-up grants the player
// should have earned for levels 2..sess.Level but does not yet have. Auto-assigns
// by first-available pool order. Safe to call on every login (idempotent).
//
// Precondition: characterID > 0; featsRepo non-nil.
// Postcondition: character_feats table contains all expected level-up feats.
func BackfillLevelUpFeats(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	mergedFeatGrants map[int]*ruleset.FeatGrants,
	featReg *ruleset.FeatRegistry,
	featsRepo CharacterFeatsRepo,
) error {
	if characterID == 0 || sess.Level < 2 || len(mergedFeatGrants) == 0 {
		return nil
	}

	existingIDs, err := featsRepo.GetAll(ctx, characterID)
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = true
	}

	// Process levels in ascending order for deterministic pool deduplication.
	levels := make([]int, 0, len(mergedFeatGrants))
	for lvl := range mergedFeatGrants {
		if lvl >= 2 && lvl <= sess.Level {
			levels = append(levels, lvl)
		}
	}
	sort.Ints(levels)

	// preExisting is a frozen snapshot of what was in the DB before this run.
	// It is used by applyFeatGrantWithBaseline to count "already satisfied" quota
	// independently of feats granted earlier in the same backfill run.
	preExisting := make(map[string]bool, len(existing))
	for id := range existing {
		preExisting[id] = true
	}

	for _, lvl := range levels {
		grants := mergedFeatGrants[lvl]
		if grants == nil {
			continue
		}
		if _, err := applyFeatGrantWithBaseline(ctx, characterID, existing, preExisting, grants, featReg, featsRepo); err != nil {
			return err
		}
	}
	return nil
}

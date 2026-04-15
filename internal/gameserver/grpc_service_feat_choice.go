package gameserver

// handleChooseFeat resolves one pending feat choice slot for a player.
//
// REQ-FCM-8: handleChooseFeat MUST validate that featID is in the pool and not already owned.
// REQ-FCM-9: handleChooseFeat MUST persist the feat and mark the grant level on success.

import (
	"context"
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleChooseFeat resolves one pending feat choice slot for a player.
//
// Precondition: uid non-empty; grantLevel >= 1; featID non-empty.
// Postcondition: On success — feat persisted, grant level marked, CharacterSheetView and
//
//	JobGrantsResponse pushed to player stream; success MessageEvent returned.
//	On denial — MessageEvent with reason returned; no state modified.
func (s *GameServiceServer) handleChooseFeat(uid string, grantLevel int, featID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Step 1: Get current pending choices.
	grantsEvt, err := s.handleJobGrants(uid)
	if err != nil {
		return nil, fmt.Errorf("handleChooseFeat: failed to load job grants: %w", err)
	}
	jobResp := grantsEvt.GetJobGrantsResponse()
	if jobResp == nil {
		return messageEvent("Feat grant data is not available."), nil
	}

	// Step 2: Find the PendingFeatChoice for the requested grant level.
	var pendingChoice *gamev1.PendingFeatChoice
	for _, pfc := range jobResp.PendingFeatChoices {
		if pfc.GrantLevel == int32(grantLevel) {
			pendingChoice = pfc
			break
		}
	}
	if pendingChoice == nil {
		return messageEvent(fmt.Sprintf("No pending feat choice at level %d.", grantLevel)), nil
	}

	// Step 3: Verify feat_id is in the pool.
	inPool := false
	for _, opt := range pendingChoice.Options {
		if opt.FeatId == featID {
			inPool = true
			break
		}
	}
	if !inPool {
		return messageEvent(fmt.Sprintf("%q is not a valid choice at level %d.", featID, grantLevel)), nil
	}

	// Step 4: Verify player does not already own the feat.
	if s.characterFeatsRepo != nil {
		existing, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("handleChooseFeat: GetAll feats: %w", err)
		}
		for _, id := range existing {
			if id == featID {
				return messageEvent(fmt.Sprintf("You already have %q.", featID)), nil
			}
		}
	}

	// Step 5: Persist the feat.
	if s.characterFeatsRepo != nil {
		if err := s.characterFeatsRepo.Add(context.Background(), sess.CharacterID, featID); err != nil {
			return nil, fmt.Errorf("handleChooseFeat: Add feat: %w", err)
		}
	}

	// Step 6: Mark grant level as fulfilled.
	if s.featLevelGrantsRepo != nil {
		if err := s.featLevelGrantsRepo.MarkLevelGranted(context.Background(), sess.CharacterID, grantLevel); err != nil {
			return nil, fmt.Errorf("handleChooseFeat: MarkLevelGranted: %w", err)
		}
	}

	// Step 7: Push updated CharacterSheetView and JobGrantsResponse to player stream.
	s.pushCharacterSheet(sess)
	if updatedGrantsEvt, err := s.handleJobGrants(uid); err == nil && updatedGrantsEvt != nil {
		s.pushEventToUID(uid, updatedGrantsEvt)
	}

	// Step 8: Return success event.
	featName := featID
	if s.featRegistry != nil {
		if f, ok := s.featRegistry.Feat(featID); ok {
			featName = f.Name
		}
	}
	return messageEvent(fmt.Sprintf("You have learned %s.", featName)), nil
}

package gameserver

import (
	"fmt"
	"sort"
	"strings"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// retrainListEligible returns a message listing the player's retrain-eligible feats.
// Eligible feats are those in PassiveFeats with category "general" or "skill".
//
// Precondition: sess is non-nil; reg is non-nil.
// Postcondition: Returns a message event listing feat IDs and names.
func retrainListEligible(sess *session.PlayerSession, reg *ruleset.FeatRegistry) *gamev1.ServerEvent {
	var lines []string
	for featID := range sess.PassiveFeats {
		f, ok := reg.Feat(featID)
		if !ok || (f.Category != "general" && f.Category != "skill") {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s — %s (%s)", f.ID, f.Name, f.Category))
	}
	if len(lines) == 0 {
		return messageEvent("You have no retrain-eligible feats.")
	}
	sort.Strings(lines)
	return messageEvent("Retrain-eligible feats:\n" + strings.Join(lines, "\n") +
		"\n\nUse: downtime retrain <feat_id>")
}

// retrainListReplacements returns eligible replacements for oldID.
// Validates that oldID is owned and eligible, then lists same-category feats not already held.
//
// Precondition: oldID is non-empty; sess and reg are non-nil.
// Postcondition: Returns a message event; never nil.
func retrainListReplacements(oldID string, sess *session.PlayerSession, reg *ruleset.FeatRegistry) *gamev1.ServerEvent {
	old, ok := reg.Feat(oldID)
	if !ok {
		return messageEvent(fmt.Sprintf("Feat %q not found.", oldID))
	}
	if !sess.PassiveFeats[oldID] {
		return messageEvent(fmt.Sprintf("You do not have feat %q.", oldID))
	}
	if old.Category != "general" && old.Category != "skill" {
		return messageEvent(fmt.Sprintf("Feat %q is not eligible for retraining.", oldID))
	}
	var lines []string
	for _, f := range reg.ByCategory(old.Category) {
		if sess.PassiveFeats[f.ID] {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s — %s", f.ID, f.Name))
	}
	if len(lines) == 0 {
		return messageEvent(fmt.Sprintf("No replacements available for %s.", old.Name))
	}
	sort.Strings(lines)
	return messageEvent(fmt.Sprintf("Replacements for %s (%s):\n%s\n\nUse: downtime retrain %s <new_feat_id>",
		old.Name, old.Category, strings.Join(lines, "\n"), oldID))
}

// validateRetrainPair validates that oldID→newID is a legal retrain swap.
// Returns "" on success or a player-visible error string on failure.
//
// Precondition: oldID and newID are non-empty; sess and reg are non-nil; jobReg may be nil.
// Postcondition: Returns "" if valid; otherwise a non-empty error message.
func validateRetrainPair(oldID, newID string, sess *session.PlayerSession, reg *ruleset.FeatRegistry, jobReg *ruleset.JobRegistry) string {
	old, ok := reg.Feat(oldID)
	if !ok {
		return fmt.Sprintf("Feat %q not found.", oldID)
	}
	nw, ok := reg.Feat(newID)
	if !ok {
		return fmt.Sprintf("Replacement feat %q not found.", newID)
	}
	if !sess.PassiveFeats[oldID] {
		return fmt.Sprintf("You do not have feat %q.", oldID)
	}
	if old.Category != "general" && old.Category != "skill" {
		return fmt.Sprintf("Feat %q is not eligible for retraining.", oldID)
	}
	if old.Category != nw.Category {
		return fmt.Sprintf("Cannot swap %s (%s) for %s (%s): different category.",
			old.Name, old.Category, nw.Name, nw.Category)
	}
	if sess.PassiveFeats[newID] {
		return fmt.Sprintf("You already have feat %q.", newID)
	}
	// REQ-RETRAIN-DT-5: block if oldID is required by any held job.
	if jobReg != nil {
		for _, jobID := range sess.HeldJobs {
			job, ok := jobReg.Job(jobID)
			if !ok {
				continue
			}
			for _, req := range job.AdvancementRequirements.RequiredFeats {
				if req == oldID {
					return fmt.Sprintf("Cannot retrain %s: it is required by your %s job.", old.Name, job.Name)
				}
			}
		}
	}
	return ""
}

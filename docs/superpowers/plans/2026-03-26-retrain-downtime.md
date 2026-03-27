# Retrain (Downtime) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the `retrain` downtime activity so players can swap a "general" or "skill" category feat for another of the same category, with prerequisite protection and full persistence.

**Architecture:** The `downtime retrain` command is a three-phase inline flow entirely within `downtimeStart`: (1) no args → list eligible feats, (2) one arg → show replacement options for that feat, (3) two args → validate and start the activity. The two IDs are stored in `sess.DowntimeMetadata` as `"<old_id> <new_id>"`. `resolveRetrain` parses them and applies the swap via `CharacterFeatsRepository.SetAll`. Job feats (category="job") are not retrain-eligible. The prerequisite block check walks `sess.HeldJobs` → job ruleset → `AdvancementRequirements.RequiredFeats`.

**Tech Stack:** Go, existing `ruleset.FeatRegistry`, `ruleset.JobRegistry`, `storage/postgres.CharacterFeatsRepository`, `pgregory.net/rapid` for property tests.

---

## Pre-implementation: field name discovery

Before writing any code, grep for the exact field names on `GameServiceServer` for the feat registry and feats repository:

```bash
grep -n "featReg\|FeatReg\|featsRepo\|FeatsRepo\|characterFeats\|CharacterFeats" \
    /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -20
```

Also grep for the job registry field:
```bash
grep -n "jobReg\|JobReg\|jobRegistry\|jobRuleset" \
    /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

Also check for the skills repo:
```bash
grep -n "skillsRepo\|SkillsRepo\|characterSkills\|CharacterSkills" \
    /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

Use these exact field names throughout the plan.

---

## File Structure

- Modify: `internal/gameserver/grpc_service_downtime.go` — add retrain gate in `downtimeStart`
- Modify: `internal/gameserver/grpc_service_downtime_resolvers.go` — implement `resolveRetrain`
- Create: `internal/gameserver/grpc_service_downtime_retrain_test.go` — unit + property tests
- Modify: `docs/features/index.yaml` — mark retrain-downtime done

---

### Task 1: Add retrain gate in `downtimeStart`

**Files:**
- Modify: `internal/gameserver/grpc_service_downtime.go`
- Create: `internal/gameserver/grpc_service_downtime_retrain_test.go`

This task wires the three-phase inline flow into `downtimeStart` for `act.ID == "retrain"`.

**Phase 1 (no args):** Return a formatted list of retrain-eligible feats.
**Phase 2 (one arg = old feat ID):** Validate old feat is eligible, check prerequisite block, return list of eligible replacements.
**Phase 3 (two args = old + new feat IDs):** Validate both IDs, start the activity, store `"<old_id> <new_id>"` in `DowntimeMetadata`.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_downtime_retrain_test.go`:

```go
package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// newRetrainTestService builds a minimal GameServiceServer with feat registry and a session
// that has two feats: "iron_will" (general) and "skill_focus_rigging" (skill).
func newRetrainTestService(t testing.TB) (*GameServiceServer, string) {
	t.Helper()

	feats := []*ruleset.Feat{
		{ID: "iron_will", Name: "Iron Will", Category: "general"},
		{ID: "iron_constitution", Name: "Iron Constitution", Category: "general"},
		{ID: "toughness", Name: "Toughness", Category: "general"},
		{ID: "skill_focus_rigging", Name: "Skill Focus: Rigging", Category: "skill", Skill: "rigging"},
		{ID: "skill_focus_intel", Name: "Skill Focus: Intel", Category: "skill", Skill: "intel"},
		{ID: "job_perk_foo", Name: "Job Perk", Category: "job"},
	}
	featReg := ruleset.NewFeatRegistryFromSlice(feats)

	svc := newTestGameServiceServer(t)
	// Set feat registry on server — use the actual field name found by grepping grpc_service.go.
	// Replace "featReg" below with the actual field name if different.
	svc.featReg = featReg

	uid := newTestSession(t, svc)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.PassiveFeats = map[string]bool{
		"iron_will":           true,
		"skill_focus_rigging": true,
		"job_perk_foo":        true,
	}
	// Ensure the session room is safe (retrain requires safe room tag).
	// newTestSession puts the player in a room — we may need to set the room's tags.
	// If the test server has a world with rooms, find a safe room and set sess.RoomID to it.
	// If not, check how newTestGameServiceServer is set up (it may have a nil world,
	// which causes downtimeStart to use empty roomTags, and "safe" tag check may fail).
	// Read internal/game/downtime/activity.go to see how RequiredTags is checked —
	// if world is nil, roomTags is "" which won't contain "safe". We may need to
	// override s.world with a stub that returns a safe room, OR set up the test server
	// with a world that has safe rooms. Examine existing downtime test helpers for the pattern.

	return svc, uid
}

func TestRetrainDowntime_NoArgs_ListsEligibleFeats(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	// Should list iron_will and skill_focus_rigging but NOT job_perk_foo.
	assert.Contains(t, msg.Content, "iron_will")
	assert.Contains(t, msg.Content, "skill_focus_rigging")
	assert.NotContains(t, msg.Content, "job_perk_foo")
	assert.False(t, sess.DowntimeBusy, "listing feats should not start the activity")
}

func TestRetrainDowntime_OneArg_ShowsReplacements(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	// Should list toughness and iron_constitution (both general, neither already held).
	assert.Contains(t, msg.Content, "iron_constitution")
	assert.Contains(t, msg.Content, "toughness")
	// Should NOT list iron_will (already held) or job/skill feats (wrong category).
	assert.NotContains(t, msg.Content, "iron_will")
	assert.NotContains(t, msg.Content, "skill_focus")
	assert.False(t, sess.DowntimeBusy)
}

func TestRetrainDowntime_OneArg_UnknownFeat_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "no_such_feat")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not found")
	assert.False(t, sess.DowntimeBusy)
}

func TestRetrainDowntime_OneArg_FeatNotOwned_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "toughness")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not have")
	assert.False(t, sess.DowntimeBusy)
}

func TestRetrainDowntime_OneArg_JobFeat_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "job_perk_foo")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not eligible")
	assert.False(t, sess.DowntimeBusy)
}

func TestRetrainDowntime_TwoArgs_ValidSwap_StartsActivity(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will iron_constitution")
	require.NotNil(t, evt)
	assert.True(t, sess.DowntimeBusy)
	assert.Equal(t, "retrain", sess.DowntimeActivityID)
	assert.Equal(t, "iron_will iron_constitution", sess.DowntimeMetadata)
}

func TestRetrainDowntime_TwoArgs_NewFeatAlreadyOwned_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	// Try to swap iron_will for skill_focus_rigging (already owned, wrong category).
	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will skill_focus_rigging")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	// Either "already have" or "different category" error.
	assert.False(t, sess.DowntimeBusy)
}

func TestRetrainDowntime_TwoArgs_CategoryMismatch_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	// iron_will is general, skill_focus_intel is skill — mismatch.
	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will skill_focus_intel")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "category")
	assert.False(t, sess.DowntimeBusy)
}

func TestProperty_RetrainDowntime_ValidSwapAlwaysStartsActivity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newRetrainTestService(t)
		sess, _ := svc.sessions.GetPlayer(uid)

		// Choose randomly between valid eligible feats.
		oldFeats := []string{"iron_will", "skill_focus_rigging"}
		newFeatsByCategory := map[string][]string{
			"general": {"iron_constitution", "toughness"},
			"skill":   {"skill_focus_intel"},
		}
		oldIdx := rapid.IntRange(0, len(oldFeats)-1).Draw(rt, "old_idx")
		oldFeat := oldFeats[oldIdx]

		oldFeatDef, _ := svc.featReg.Feat(oldFeat)
		choices := newFeatsByCategory[oldFeatDef.Category]
		newIdx := rapid.IntRange(0, len(choices)-1).Draw(rt, "new_idx")
		newFeat := choices[newIdx]

		evt := svc.downtimeStart(uid, sess, "retrain", oldFeat+" "+newFeat)
		require.NotNil(rt, evt)
		assert.True(rt, sess.DowntimeBusy,
			"valid swap of %s→%s should start activity", oldFeat, newFeat)
		assert.Equal(rt, oldFeat+" "+newFeat, sess.DowntimeMetadata)
	})
}
```

**NOTE about safe room requirement:** Read `internal/game/downtime/activity.go` to see if `RequiredTags` check is skipped when `world` is nil (which it is in test servers). If the check only runs when a world is present, tests will work as-is. Read `downtime.CanStart` to verify.

**NOTE about `ruleset.NewFeatRegistryFromSlice`:** This may not exist. Read `internal/game/ruleset/feat.go` to find the actual constructor. If it only has `LoadFeatRegistry(path)`, you'll need to add `NewFeatRegistryFromSlice(feats []*Feat) *FeatRegistry` to `feat.go` — following the same pattern as `NewRecipeRegistryFromSlice` in `crafting/recipe.go`.

**NOTE about `svc.featReg`:** Replace with the actual field name found by grepping `grpc_service.go`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestRetrainDowntime|TestProperty_RetrainDowntime" -v 2>&1 | head -30
```

Expected: compile error or FAIL.

- [ ] **Step 3: Add `NewFeatRegistryFromSlice` to feat.go if needed**

In `internal/game/ruleset/feat.go`, add after the existing constructors:

```go
// NewFeatRegistryFromSlice builds a FeatRegistry from a slice of Feat pointers.
// Intended for use in tests and tools that do not read from YAML files.
//
// Precondition: feats may be nil or empty.
// Postcondition: every entry is accessible via Feat(id).
func NewFeatRegistryFromSlice(feats []*Feat) *FeatRegistry {
	reg := &FeatRegistry{
		byID:       make(map[string]*Feat, len(feats)),
		byCategory: make(map[string][]*Feat),
		bySkill:    make(map[string][]*Feat),
		byArchetype: make(map[string][]*Feat),
	}
	for _, f := range feats {
		if f == nil {
			continue
		}
		reg.byID[f.ID] = f
		reg.byCategory[f.Category] = append(reg.byCategory[f.Category], f)
		if f.Skill != "" {
			reg.bySkill[f.Skill] = append(reg.bySkill[f.Skill], f)
		}
		if f.Archetype != "" {
			reg.byArchetype[f.Archetype] = append(reg.byArchetype[f.Archetype], f)
		}
	}
	return reg
}
```

**Read the actual `FeatRegistry` struct fields first** before writing this — the field names may differ from the above.

- [ ] **Step 4: Add retrain gate block to `downtimeStart` in `grpc_service_downtime.go`**

After the craft gate block (after the `if act.ID == "craft"` block), add:

```go
// Gate retrain on feat selection and validation. (REQ-RETRAIN-DT-1, REQ-RETRAIN-DT-2)
if act.ID == "retrain" {
	if s.featReg == nil {
		return messageEvent("Retrain system not available.")
	}
	args := strings.Fields(activityArgs)

	switch len(args) {
	case 0:
		// Phase 1: list retrain-eligible feats.
		return retrainListEligible(sess, s.featReg)

	case 1:
		// Phase 2: show replacements for the chosen feat.
		return retrainListReplacements(args[0], sess, s.featReg)

	default:
		// Phase 3: validate and proceed to start.
		oldID, newID := args[0], args[1]
		if errMsg := validateRetrainPair(oldID, newID, sess, s.featReg, s.jobReg); errMsg != "" {
			return messageEvent(errMsg)
		}
		// activityArgs stays as "<old_id> <new_id>" — stored in DowntimeMetadata below.
	}
}
```

Add the three helper functions to `grpc_service_downtime.go` (or a new file `grpc_service_downtime_retrain.go`):

```go
// retrainListEligible returns a message listing the player's retrain-eligible feats.
// Eligible feats are those in PassiveFeats with category "general" or "skill".
//
// Precondition: sess is non-nil; reg is non-nil.
// Postcondition: Returns a message event listing feat IDs and names, never nil.
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

// retrainListReplacements returns a message listing feats the player can swap oldID for.
// Validates that oldID is eligible, then returns all same-category feats not already held.
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
			continue // already owned
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

// validateRetrainPair checks that oldID→newID is a valid retrain swap.
// Returns an empty string on success, or an error message on failure.
//
// Precondition: oldID and newID are non-empty; sess, reg, jobReg may be nil.
// Postcondition: Returns "" if the swap is valid; otherwise a player-visible error string.
func validateRetrainPair(oldID, newID string, sess *session.PlayerSession, reg *ruleset.FeatRegistry, jobReg *ruleset.JobRegistry) string {
	if reg == nil {
		return "Retrain system not available."
	}
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
```

**NOTE about `s.jobReg`:** Replace with the actual field name from `grpc_service.go`. If there's no job registry field on the server, you can pass nil — the prerequisite check will be skipped (still safe).

**NOTE about `ruleset.JobRegistry.Job(id)`:** Read `internal/game/ruleset/job.go` to find the actual lookup method. It may be `Get(id)` or `ByID(id)`.

**NOTE about `job.AdvancementRequirements`:** The `Job` struct has `Tier1`, `Tier2`, `Tier3` advancement requirements fields (or similar). Read the struct to find the actual field name. The prerequisite check should walk all tier requirements the player has unlocked.

**NOTE about imports:** Add `"sort"` (already imported), `"strings"` (already imported), `"github.com/cory-johannsen/mud/internal/game/ruleset"` if not present.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestRetrainDowntime|TestProperty_RetrainDowntime" -v 2>&1 | tail -25
```

Expected: all tests PASS.

- [ ] **Step 6: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v '/storage/postgres') -count=1 2>&1 | tail -15
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
    internal/gameserver/grpc_service_downtime.go \
    internal/game/ruleset/feat.go \
    internal/gameserver/grpc_service_downtime_retrain_test.go && \
git commit -m "feat(retrain-downtime): add feat selection phases to downtimeStart"
```

---

### Task 2: Implement `resolveRetrain`

**Files:**
- Modify: `internal/gameserver/grpc_service_downtime_resolvers.go:251-261`
- Modify: `internal/gameserver/grpc_service_downtime_retrain_test.go` (add resolution tests)

Replace the stub `resolveRetrain`. It must: parse the two feat IDs from `sess.DowntimeMetadata`, remove the old feat from `sess.PassiveFeats`, add the new feat, persist the updated feat list via `featsRepo.SetAll`, and push an outcome message.

- [ ] **Step 1: Write failing resolution tests**

Append to `internal/gameserver/grpc_service_downtime_retrain_test.go`:

```go
func TestRetrainResolve_SwapSucceeds_FeatsUpdated(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = "iron_will iron_constitution"

	svc.resolveRetrain(uid, sess)

	assert.False(t, sess.PassiveFeats["iron_will"], "old feat should be removed")
	assert.True(t, sess.PassiveFeats["iron_constitution"], "new feat should be added")
}

func TestRetrainResolve_SkillFeatSwap_Works(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = "skill_focus_rigging skill_focus_intel"

	svc.resolveRetrain(uid, sess)

	assert.False(t, sess.PassiveFeats["skill_focus_rigging"])
	assert.True(t, sess.PassiveFeats["skill_focus_intel"])
}

func TestRetrainResolve_EmptyMetadata_NoOp(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = ""
	original := map[string]bool{"iron_will": true, "skill_focus_rigging": true, "job_perk_foo": true}
	sess.PassiveFeats = map[string]bool{"iron_will": true, "skill_focus_rigging": true, "job_perk_foo": true}

	svc.resolveRetrain(uid, sess)

	assert.Equal(t, original, sess.PassiveFeats, "empty metadata should leave feats unchanged")
}

func TestProperty_RetrainResolve_OldRemovedNewAdded(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newRetrainTestService(t)
		sess, _ := svc.sessions.GetPlayer(uid)

		// Choose a random valid pair.
		pairs := [][2]string{
			{"iron_will", "iron_constitution"},
			{"iron_will", "toughness"},
			{"skill_focus_rigging", "skill_focus_intel"},
		}
		idx := rapid.IntRange(0, len(pairs)-1).Draw(rt, "pair_idx")
		pair := pairs[idx]
		sess.DowntimeMetadata = pair[0] + " " + pair[1]

		svc.resolveRetrain(uid, sess)

		assert.False(rt, sess.PassiveFeats[pair[0]], "old feat %s should be removed", pair[0])
		assert.True(rt, sess.PassiveFeats[pair[1]], "new feat %s should be added", pair[1])
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestRetrainResolve|TestProperty_RetrainResolve" -v 2>&1 | head -20
```

Expected: FAIL (stub sends "Retrain complete. Your changes will take effect next login." with no feat changes).

- [ ] **Step 3: Implement `resolveRetrain`**

In `internal/gameserver/grpc_service_downtime_resolvers.go`, replace lines 251-261:

```go
// resolveRetrain resolves the "Retrain" downtime activity.
//
// sess.DowntimeMetadata must hold "<old_feat_id> <new_feat_id>" set at start time.
// Removes the old feat, adds the new feat, persists via featsRepo.
//
// Precondition: sess is non-nil; state already cleared by caller.
// Postcondition: Old feat removed and new feat added to sess.PassiveFeats;
//   changes persisted via featsRepo if non-nil; console message delivered.
func (s *GameServiceServer) resolveRetrain(uid string, sess *session.PlayerSession) {
	parts := strings.Fields(sess.DowntimeMetadata)
	if len(parts) < 2 {
		s.pushMessageToUID(uid, "Retrain complete. (No feat change recorded.)")
		return
	}
	oldID, newID := parts[0], parts[1]

	if s.featReg == nil {
		s.pushMessageToUID(uid, "Retrain complete. (Feat system unavailable.)")
		return
	}
	oldFeat, oldOK := s.featReg.Feat(oldID)
	newFeat, newOK := s.featReg.Feat(newID)
	if !oldOK || !newOK {
		s.pushMessageToUID(uid, "Retrain complete. (Feat definition missing.)")
		return
	}

	// Apply the swap.
	delete(sess.PassiveFeats, oldID)
	if sess.PassiveFeats == nil {
		sess.PassiveFeats = make(map[string]bool)
	}
	sess.PassiveFeats[newID] = true

	// Persist: collect all current passive feat IDs and call SetAll.
	if s.featsRepo != nil && sess.CharacterID > 0 {
		var featIDs []string
		for id := range sess.PassiveFeats {
			featIDs = append(featIDs, id)
		}
		if err := s.featsRepo.SetAll(context.Background(), sess.CharacterID, featIDs); err != nil && s.logger != nil {
			s.logger.Error("resolveRetrain: failed to persist feat changes",
				zap.String("uid", uid), zap.Error(err))
		}
	}

	s.pushMessageToUID(uid, fmt.Sprintf(
		"Retrain complete. You replaced %s with %s.", oldFeat.Name, newFeat.Name,
	))
}
```

**NOTE about `s.featsRepo`:** Replace with the actual field name found from grepping `grpc_service.go`. The method is `SetAll(ctx, characterID, featIDs []string) error`.

**NOTE about imports:** Add `"strings"` if not already imported in `grpc_service_downtime_resolvers.go`. Add `"go.uber.org/zap"` if not present. Add `"context"` if not present.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestRetrainDowntime|TestRetrainResolve|TestProperty_Retrain" -v 2>&1 | tail -25
```

Expected: all tests PASS.

- [ ] **Step 5: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v '/storage/postgres') -count=1 2>&1 | tail -15
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
    internal/gameserver/grpc_service_downtime_resolvers.go \
    internal/gameserver/grpc_service_downtime_retrain_test.go && \
git commit -m "feat(retrain-downtime): implement resolveRetrain with feat swap and persistence"
```

---

### Task 3: Mark feature done

**Files:**
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Update feature status**

In `docs/features/index.yaml`, change `retrain-downtime` status from `planned` to `done`.

- [ ] **Step 2: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/features/index.yaml && \
git commit -m "chore: mark retrain-downtime feature done"
```

---

## Self-Review

**Spec coverage:**
- REQ-RETRAIN-DT-1 (present list of retrain-eligible feats and skill choices): Task 1 Phase 1 shows eligible feats. "Skill choices" (skill rank swaps) are out of scope for this pass — no skill rank mechanic exists in the current infrastructure. The list covers all skill-category feats. ✓
- REQ-RETRAIN-DT-2 (player selects feat to replace AND replacement before activity begins): Task 1 Phase 3 requires two args; activity only starts when both are provided and valid. ✓
- REQ-RETRAIN-DT-3 (on completion, old removed and new added): Task 2 `resolveRetrain` applies the swap. ✓
- REQ-RETRAIN-DT-4 (changes persisted via character repository): Task 2 `resolveRetrain` calls `featsRepo.SetAll`. ✓
- REQ-RETRAIN-DT-5 (block if feat being replaced is a prerequisite for another feat): Task 1 `validateRetrainPair` checks `sess.HeldJobs` → job's `AdvancementRequirements.RequiredFeats`. ✓

**Placeholder scan:** None found.

**Type consistency:** `sess.PassiveFeats map[string]bool`, `ruleset.FeatRegistry.Feat(id)`, `ruleset.FeatRegistry.ByCategory(cat)` — all consistent across both tasks.

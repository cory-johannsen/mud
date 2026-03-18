package gameserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// fakePendingTechLevelsRepoInternal is a full ProgressRepository test double
// that tracks SetPendingTechLevels calls.
//
// Precondition: none.
// Postcondition: SetPendingTechLevels records the levels passed; all other methods are no-ops.
type fakePendingTechLevelsRepoInternal struct {
	pendingLevels []int
	setWasCalled  bool
}

func (r *fakePendingTechLevelsRepoInternal) GetProgress(_ context.Context, _ int64) (int, int, int, int, error) {
	return 1, 0, 10, 0, nil
}
func (r *fakePendingTechLevelsRepoInternal) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (r *fakePendingTechLevelsRepoInternal) IncrementPendingSkillIncreases(_ context.Context, _ int64, _ int) error {
	return nil
}
func (r *fakePendingTechLevelsRepoInternal) ConsumePendingBoost(_ context.Context, _ int64) error {
	return nil
}
func (r *fakePendingTechLevelsRepoInternal) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	return nil
}
func (r *fakePendingTechLevelsRepoInternal) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (r *fakePendingTechLevelsRepoInternal) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	return nil
}
func (r *fakePendingTechLevelsRepoInternal) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return r.pendingLevels, nil
}
func (r *fakePendingTechLevelsRepoInternal) SetPendingTechLevels(_ context.Context, _ int64, levels []int) error {
	r.pendingLevels = levels
	r.setWasCalled = true
	return nil
}

// hwRepoInternal is a minimal HardwiredTechRepo for internal tests.
type hwRepoInternal struct{ stored []string }

func (r *hwRepoInternal) GetAll(_ context.Context, _ int64) ([]string, error) { return r.stored, nil }
func (r *hwRepoInternal) SetAll(_ context.Context, _ int64, ids []string) error {
	r.stored = ids
	return nil
}

// prepRepoInternal is a minimal PreparedTechRepo for internal tests.
type prepRepoInternal struct {
	slots map[int][]*session.PreparedSlot
}

func (r *prepRepoInternal) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *prepRepoInternal) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *prepRepoInternal) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *prepRepoInternal) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	return nil
}

// spontRepoInternal is a minimal SpontaneousTechRepo for internal tests.
type spontRepoInternal struct{ techs map[int][]string }

func (r *spontRepoInternal) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	return r.techs, nil
}
func (r *spontRepoInternal) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *spontRepoInternal) DeleteAll(_ context.Context, _ int64) error { r.techs = nil; return nil }

// innateRepoInternal is a minimal InnateTechRepo for internal tests.
type innateRepoInternal struct {
	slots map[string]*session.InnateSlot
}

func (r *innateRepoInternal) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	return r.slots, nil
}
func (r *innateRepoInternal) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses}
	return nil
}
func (r *innateRepoInternal) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *innateRepoInternal) Decrement(_ context.Context, _ int64, _ string) error { return nil }
func (r *innateRepoInternal) RestoreAll(_ context.Context, _ int64) error           { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// drainLevelUpEntityMessages reads all currently buffered events from target's entity
// and decodes them as ServerEvent messages. Returns the content of MessageEvents.
func drainLevelUpEntityMessages(t *testing.T, target *session.PlayerSession) []string {
	t.Helper()
	var msgs []string
	for {
		select {
		case data, ok := <-target.Entity.Events():
			if !ok {
				return msgs
			}
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err == nil {
				if m := evt.GetMessage(); m != nil {
					msgs = append(msgs, m.Content)
				}
			}
		default:
			return msgs
		}
	}
}

// buildLevelUpTechSvc creates a GameServiceServer wired with an xpSvc (BaseXP=100)
// and a job registry containing one job with LevelUpGrants[2] set to the provided grants.
//
// Precondition: progressRepo is non-nil.
// Postcondition: Returns a *GameServiceServer ready for handleGrant tests.
func buildLevelUpTechSvc(
	t *testing.T,
	progressRepo ProgressRepository,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
	levelUpGrants *ruleset.TechnologyGrants,
) *GameServiceServer {
	t.Helper()
	svc := testServiceForGrant(t, grantTestOptions{
		charSaver:    &grantCharSaver{},
		progressRepo: progressRepo,
	})

	cfg := &xp.XPConfig{
		BaseXP:        100,
		HPPerLevel:    5,
		BoostInterval: 5,
		LevelCap:      100,
		Awards: xp.Awards{
			KillXPPerNPCLevel:       50,
			NewRoomXP:               10,
			SkillCheckSuccessXP:     10,
			SkillCheckCritSuccessXP: 25,
			SkillCheckDCMultiplier:  2,
		},
	}
	xpSvc := xp.NewService(cfg, &grantXPProgressSaver{})
	svc.SetXPService(xpSvc)

	job := &ruleset.Job{
		ID:   "test_job",
		Name: "Test Job",
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: levelUpGrants,
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.jobRegistry = jobReg

	svc.SetHardwiredTechRepo(hwRepo)
	svc.SetPreparedTechRepo(prepRepo)
	svc.SetSpontaneousTechRepo(spontRepo)
	svc.SetInnateTechRepo(innateRepo)

	return svc
}

// addTargetWithJobForLevelUp adds a target player with Class="test_job" and CharacterID=2.
//
// Precondition: svc must have a valid session manager.
// Postcondition: Player is in the session manager; session is returned.
func addTargetWithJobForLevelUp(t *testing.T, svc *GameServiceServer, uid, charName string) *session.PlayerSession {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    charName,
		CharacterID: 2,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
		Level:       1,
		Class:       "test_job",
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	return sess
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleGrant_LevelUp_AutoAssignOnly_NoPending verifies REQ-ILT2:
// when the pool has exactly as many entries as open slots (auto-assign), no pending grant is created.
//
// Precondition: LevelUpGrants[2] has 1 slot at level 1 and exactly 1 pool entry.
// Postcondition: target.PendingTechGrants is empty; prepared tech repo contains the auto-assigned tech.
func TestHandleGrant_LevelUp_AutoAssignOnly_NoPending(t *testing.T) {
	pendingRepo := &fakePendingTechLevelsRepoInternal{}
	hwRepo := &hwRepoInternal{}
	prepRepo := &prepRepoInternal{}
	spontRepo := &spontRepoInternal{}
	innateRepo := &innateRepoInternal{}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "auto_tech", Level: 1}},
		},
	}

	svc := buildLevelUpTechSvc(t, pendingRepo, hwRepo, prepRepo, spontRepo, innateRepo, grants)
	addEditorForGrant(t, svc, "editor_auto")
	target := addTargetWithJobForLevelUp(t, svc, "target_auto", "AutoChar")

	_, err := svc.handleGrant("editor_auto", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "AutoChar",
		Amount:    500, // enough to reach level 2 with BaseXP=100 (level 2 needs 400 XP)
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, target.Level, 2, "precondition: target must have leveled up")

	assert.Empty(t, target.PendingTechGrants, "REQ-ILT2: auto-assign must not leave pending grants")
	require.NotNil(t, prepRepo.slots, "prepared tech repo must have slots after auto-assign")
	techIDs := make([]string, 0)
	for _, slots := range prepRepo.slots {
		for _, s := range slots {
			if s != nil {
				techIDs = append(techIDs, s.TechID)
			}
		}
	}
	assert.Contains(t, techIDs, "auto_tech", "REQ-ILT2: auto-assigned tech must be persisted")
}

// TestHandleGrant_LevelUp_PoolChoiceDeferred verifies REQ-ILT1:
// when pool has more entries than open slots, the grant is deferred.
//
// Precondition: LevelUpGrants[2] has 1 slot at level 1 and 2 pool entries.
// Postcondition: target.PendingTechGrants[2] is non-nil; pendingRepo.setWasCalled is true.
func TestHandleGrant_LevelUp_PoolChoiceDeferred(t *testing.T) {
	pendingRepo := &fakePendingTechLevelsRepoInternal{}
	hwRepo := &hwRepoInternal{}
	prepRepo := &prepRepoInternal{}
	spontRepo := &spontRepoInternal{}
	innateRepo := &innateRepoInternal{}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "choice_a", Level: 1},
				{ID: "choice_b", Level: 1},
			},
		},
	}

	svc := buildLevelUpTechSvc(t, pendingRepo, hwRepo, prepRepo, spontRepo, innateRepo, grants)
	addEditorForGrant(t, svc, "editor_defer")
	target := addTargetWithJobForLevelUp(t, svc, "target_defer", "DeferChar")

	_, err := svc.handleGrant("editor_defer", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "DeferChar",
		Amount:    500,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, target.Level, 2, "precondition: target must have leveled up")

	require.NotNil(t, target.PendingTechGrants, "REQ-ILT1: deferred grants must be set")
	assert.NotNil(t, target.PendingTechGrants[2], "REQ-ILT1: PendingTechGrants[2] must be non-nil")
	assert.True(t, pendingRepo.setWasCalled, "REQ-ILT1: SetPendingTechLevels must be called when grants are deferred")
}

// TestHandleGrant_LevelUp_AutoAssign_PushesNotification verifies REQ-ILT3:
// auto-assigned technologies generate a push notification containing "auto-assigned".
//
// Precondition: LevelUpGrants[2] has 1 slot with exactly 1 pool entry (auto-assign scenario).
// Postcondition: at least one pushed message to target entity contains "auto-assigned".
func TestHandleGrant_LevelUp_AutoAssign_PushesNotification(t *testing.T) {
	pendingRepo := &fakePendingTechLevelsRepoInternal{}
	hwRepo := &hwRepoInternal{}
	prepRepo := &prepRepoInternal{}
	spontRepo := &spontRepoInternal{}
	innateRepo := &innateRepoInternal{}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "notify_tech", Level: 1}},
		},
	}

	svc := buildLevelUpTechSvc(t, pendingRepo, hwRepo, prepRepo, spontRepo, innateRepo, grants)
	addEditorForGrant(t, svc, "editor_notify_auto")
	target := addTargetWithJobForLevelUp(t, svc, "target_notify_auto", "NotifyAutoChar")

	_, err := svc.handleGrant("editor_notify_auto", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "NotifyAutoChar",
		Amount:    500,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, target.Level, 2, "precondition: target must have leveled up")

	msgs := drainLevelUpEntityMessages(t, target)
	found := false
	var foundMsg string
	for _, m := range msgs {
		if strings.Contains(m, "auto-assigned") {
			found = true
			foundMsg = m
			break
		}
	}
	assert.True(t, found, "REQ-ILT3: at least one push message must contain 'auto-assigned'; got: %v", msgs)
	assert.Contains(t, foundMsg, "notify_tech", "REQ-ILT3: auto-assigned notification must contain the tech ID; got: %q", foundMsg)
}

// TestHandleGrant_LevelUp_Deferred_PushesSelectTechNotification verifies REQ-ILT4:
// when grants are deferred, a push notification containing "selecttech" is sent to the target.
//
// Precondition: LevelUpGrants[2] has 1 slot with 2 pool entries (deferred scenario).
// Postcondition: at least one pushed message to target entity contains "selecttech".
func TestHandleGrant_LevelUp_Deferred_PushesSelectTechNotification(t *testing.T) {
	pendingRepo := &fakePendingTechLevelsRepoInternal{}
	hwRepo := &hwRepoInternal{}
	prepRepo := &prepRepoInternal{}
	spontRepo := &spontRepoInternal{}
	innateRepo := &innateRepoInternal{}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "sel_a", Level: 1},
				{ID: "sel_b", Level: 1},
			},
		},
	}

	svc := buildLevelUpTechSvc(t, pendingRepo, hwRepo, prepRepo, spontRepo, innateRepo, grants)
	addEditorForGrant(t, svc, "editor_notify_defer")
	target := addTargetWithJobForLevelUp(t, svc, "target_notify_defer", "NotifyDeferChar")

	_, err := svc.handleGrant("editor_notify_defer", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "NotifyDeferChar",
		Amount:    500,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, target.Level, 2, "precondition: target must have leveled up")

	msgs := drainLevelUpEntityMessages(t, target)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "selecttech") {
			found = true
			break
		}
	}
	assert.True(t, found, "REQ-ILT4: at least one push message must contain 'selecttech'; got: %v", msgs)
}

// TestPropertyHandleGrant_LevelUp_PartitionInvariant verifies that for any grant configuration,
// handleGrant either defers (pool > open slots) or auto-assigns (pool <= open slots), never both
// for the same level.
//
// Precondition: pool size nPool >= 1; slot count nSlots >= 1; player starts at level 1 with enough XP to reach level 2.
// Postcondition: PendingTechGrants is non-nil iff nPool > nSlots.
func TestPropertyHandleGrant_LevelUp_PartitionInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nPool := rapid.IntRange(1, 4).Draw(rt, "nPool")
		nSlots := rapid.IntRange(1, 4).Draw(rt, "nSlots")

		pool := make([]ruleset.PreparedEntry, nPool)
		for i := range pool {
			pool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("pool_tech_%d", i), Level: 1}
		}

		grants := &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: nSlots},
				Pool:         pool,
			},
		}

		pendingRepo := &fakePendingTechLevelsRepoInternal{}
		hwRepo := &hwRepoInternal{}
		prepRepo := &prepRepoInternal{}
		spontRepo := &spontRepoInternal{}
		innateRepo := &innateRepoInternal{}

		uid := fmt.Sprintf("prop-player-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		charName := fmt.Sprintf("PropChar%d", rapid.IntRange(0, 9999).Draw(rt, "charName"))

		svc := buildLevelUpTechSvc(t, pendingRepo, hwRepo, prepRepo, spontRepo, innateRepo, grants)
		addEditorForGrant(t, svc, "prop_editor_"+uid)
		target := addTargetWithJobForLevelUp(t, svc, uid, charName)

		_, err := svc.handleGrant("prop_editor_"+uid, &gamev1.GrantRequest{
			GrantType: "xp",
			CharName:  charName,
			Amount:    500, // sufficient to reach level 2 (BaseXP=100 → level 2 needs 400 XP)
		})
		require.NoError(t, err)
		if target.Level < 2 {
			rt.Skip()
		}

		if nPool > nSlots {
			// Deferred: PendingTechGrants must be populated for the new level.
			if target.PendingTechGrants == nil || target.PendingTechGrants[2] == nil {
				rt.Fatalf("nPool(%d) > nSlots(%d) but PendingTechGrants[2] is nil; PendingTechGrants=%v",
					nPool, nSlots, target.PendingTechGrants)
			}
		} else {
			// Immediate: PendingTechGrants must be empty for the new level.
			if target.PendingTechGrants != nil && target.PendingTechGrants[2] != nil {
				rt.Fatalf("nPool(%d) <= nSlots(%d) but PendingTechGrants[2] is non-nil; PendingTechGrants=%v",
					nPool, nSlots, target.PendingTechGrants)
			}
		}
	})
}

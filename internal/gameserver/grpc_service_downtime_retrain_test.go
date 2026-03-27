package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// newRetrainTestService creates a GameServiceServer with a safe-tagged room, a feat registry,
// and a player session that holds iron_will, skill_focus_rigging, and job_perk_foo.
//
// Precondition: none.
// Postcondition: Returns svc and uid; GetPlayer(uid) succeeds; room has "safe" tag.
func newRetrainTestService(t testing.TB) (*GameServiceServer, string) {
	t.Helper()
	const uid = "retrain_uid"
	feats := []*ruleset.Feat{
		{ID: "iron_will", Name: "Iron Will", Category: "general"},
		{ID: "iron_constitution", Name: "Iron Constitution", Category: "general"},
		{ID: "toughness", Name: "Toughness", Category: "general"},
		{ID: "skill_focus_rigging", Name: "Skill Focus: Rigging", Category: "skill", Skill: "rigging"},
		{ID: "skill_focus_intel", Name: "Skill Focus: Intel", Category: "skill", Skill: "intel"},
		{ID: "job_perk_foo", Name: "Job Perk", Category: "job"},
	}
	featReg := ruleset.NewFeatRegistryFromSlice(feats)

	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession(uid, "room1")
	svc := &GameServiceServer{
		sessions:     sMgr,
		world:        wMgr,
		featRegistry: featReg,
	}

	sess, ok := sMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PassiveFeats = map[string]bool{
		"iron_will":           true,
		"skill_focus_rigging": true,
		"job_perk_foo":        true,
	}
	return svc, uid
}

// TestRetrainDowntime_NoArgs_ListsEligibleFeats verifies that starting retrain with no args
// returns a list of eligible feats and does not set DowntimeBusy.
//
// Precondition: sess has iron_will (general), skill_focus_rigging (skill), job_perk_foo (job).
// Postcondition: message contains iron_will and skill_focus_rigging; excludes job_perk_foo.
func TestRetrainDowntime_NoArgs_ListsEligibleFeats(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "iron_will")
	assert.Contains(t, msg.Content, "skill_focus_rigging")
	assert.NotContains(t, msg.Content, "job_perk_foo")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_OneArg_ShowsReplacements verifies that passing one owned general feat
// returns a list of same-category replacements not already owned.
//
// Precondition: sess has iron_will; iron_constitution and toughness are not owned.
// Postcondition: message contains iron_constitution and toughness; does not contain iron_will.
func TestRetrainDowntime_OneArg_ShowsReplacements(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "iron_constitution")
	assert.Contains(t, msg.Content, "toughness")
	// iron_will appears only in the usage hint footer, not as a replacement option.
	assert.NotContains(t, msg.Content, "iron_will — Iron Will")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_OneArg_UnknownFeat_ReturnsError verifies that an unknown feat ID
// returns an error message and does not set DowntimeBusy.
//
// Precondition: "no_such_feat" is not in the registry.
// Postcondition: message contains "not found"; DowntimeBusy remains false.
func TestRetrainDowntime_OneArg_UnknownFeat_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "no_such_feat")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not found")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_OneArg_FeatNotOwned_ReturnsError verifies that specifying an unowned feat
// returns an error message and does not set DowntimeBusy.
//
// Precondition: "toughness" is in registry but not in sess.PassiveFeats.
// Postcondition: message contains "not have"; DowntimeBusy remains false.
func TestRetrainDowntime_OneArg_FeatNotOwned_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "toughness")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not have")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_OneArg_JobFeat_ReturnsError verifies that specifying an owned job-category feat
// returns an eligibility error and does not set DowntimeBusy.
//
// Precondition: "job_perk_foo" is owned but has category "job".
// Postcondition: message contains "not eligible"; DowntimeBusy remains false.
func TestRetrainDowntime_OneArg_JobFeat_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "job_perk_foo")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "not eligible")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_TwoArgs_ValidSwap_StartsActivity verifies that a valid same-category swap
// sets DowntimeBusy and stores the metadata.
//
// Precondition: iron_will is owned general feat; iron_constitution is unowned general feat.
// Postcondition: DowntimeBusy==true; DowntimeActivityID=="retrain"; DowntimeMetadata matches args.
func TestRetrainDowntime_TwoArgs_ValidSwap_StartsActivity(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will iron_constitution")
	require.NotNil(t, evt)
	assert.True(t, sess.DowntimeBusy)
	assert.Equal(t, "retrain", sess.DowntimeActivityID)
	assert.Equal(t, "iron_will iron_constitution", sess.DowntimeMetadata)
}

// TestRetrainDowntime_TwoArgs_CategoryMismatch_ReturnsError verifies that swapping across
// categories returns an error and does not set DowntimeBusy.
//
// Precondition: iron_will is general; skill_focus_intel is skill.
// Postcondition: message contains "category"; DowntimeBusy remains false.
func TestRetrainDowntime_TwoArgs_CategoryMismatch_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will skill_focus_intel")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "category")
	assert.False(t, sess.DowntimeBusy)
}

// TestRetrainDowntime_TwoArgs_NewFeatAlreadyOwned_ReturnsError verifies that swapping to an
// already-owned feat returns an error and does not set DowntimeBusy.
//
// Precondition: iron_will→skill_focus_rigging: category mismatch AND skill_focus_rigging is already owned.
// Postcondition: message is non-empty; DowntimeBusy remains false.
func TestRetrainDowntime_TwoArgs_NewFeatAlreadyOwned_ReturnsError(t *testing.T) {
	svc, uid := newRetrainTestService(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	evt := svc.downtimeStart(uid, sess, "retrain", "iron_will skill_focus_rigging")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.False(t, sess.DowntimeBusy)
}

// TestProperty_RetrainDowntime_ValidSwapAlwaysStartsActivity is a property test verifying that
// any valid same-category swap always sets DowntimeBusy and stores correct metadata.
//
// Precondition: pairs are all valid same-category swaps from unowned to owned.
// Postcondition: DowntimeBusy==true and DowntimeMetadata matches the pair for every draw.
func TestProperty_RetrainDowntime_ValidSwapAlwaysStartsActivity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newRetrainTestService(t)
		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(t, ok)

		pairs := [][2]string{
			{"iron_will", "iron_constitution"},
			{"iron_will", "toughness"},
			{"skill_focus_rigging", "skill_focus_intel"},
		}
		idx := rapid.IntRange(0, len(pairs)-1).Draw(rt, "pair_idx")
		pair := pairs[idx]

		evt := svc.downtimeStart(uid, sess, "retrain", pair[0]+" "+pair[1])
		require.NotNil(rt, evt)
		assert.True(rt, sess.DowntimeBusy, "valid swap %s→%s should start", pair[0], pair[1])
		assert.Equal(rt, pair[0]+" "+pair[1], sess.DowntimeMetadata)
	})
}

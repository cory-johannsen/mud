package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/drawback"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newDrawbackTestServer creates a minimal GameServiceServer with a job registry
// containing a synthetic job that has the given situational drawback trigger
// mapped to the given effect condition, and a condition registry with that condition.
//
// Precondition: trigger is one of the drawback.TriggerOn* constants; effectCondID is non-empty.
// Postcondition: Returns a non-nil server and the registered job.
func newDrawbackTestServer(
	t testing.TB,
	jobID string,
	trigger string,
	effectCondID string,
) (*GameServiceServer, *ruleset.Job) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t.(*testing.T))
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t.(*testing.T), worldMgr, sessMgr, npcManager)

	// Build a condition registry with the effect condition.
	condDef := &condition.ConditionDef{ID: effectCondID, Name: effectCondID, DurationType: "permanent"}
	condReg := condition.NewRegistry()
	condReg.Register(condDef)
	svc.condRegistry = condReg

	// Build a job with the situational drawback.
	job := &ruleset.Job{
		ID:   jobID,
		Name: jobID,
		Drawbacks: []ruleset.DrawbackDef{
			{
				ID:                "test_db",
				Type:              "situational",
				Trigger:           trigger,
				EffectConditionID: effectCondID,
				Duration:          "1h",
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.jobRegistry = jobReg

	// Re-initialise the drawback engine with the new condition registry.
	svc.drawbackEngine = drawback.NewEngine(condReg)

	return svc, job
}

// TestSkillCheckFailure_DrawbackConditionApplied verifies that when a player with a job
// that has an on_fail_skill_check situational drawback fails a skill check (DC=9999,
// dice=nil → roll=10, guaranteed CritFailure), the drawback's effect condition is applied.
//
// REQ-JD-10.
func TestSkillCheckFailure_DrawbackConditionApplied(t *testing.T) {
	const (
		uid          = "db_sc_u1"
		jobID        = "test_skcheck_job"
		effectCondID = "demoralized"
	)

	svc, job := newDrawbackTestServer(t, jobID, drawback.TriggerOnFailSkillCheck, effectCondID)

	// dice == nil → rollD20() returns 10; DC=9999 → total=10 < 9989 → CritFailure.
	svc.dice = nil

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "db_sc_user",
		CharName:  "DBChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     50,
		Role:      "player",
	})
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Jobs = map[string]int{job.ID: 1}
	sess.Conditions = condition.NewActiveSet()

	_ = svc.skillCheckOutcome(sess, "hustle", 9999)

	assert.True(t, sess.Conditions.Has(effectCondID),
		"condition %q must be applied after on_fail_skill_check drawback fires", effectCondID)
}

// TestPropertySkillCheckFailure_DrawbackConditionApplied is a property-based test that
// verifies any guaranteed-failing skill check fires on_fail_skill_check and applies the
// effect condition.
//
// REQ-JD-10, SWENG-5a.
func TestPropertySkillCheckFailure_DrawbackConditionApplied(t *testing.T) {
	const (
		jobID        = "prop_skcheck_job"
		effectCondID = "demoralized"
	)

	rapid.Check(t, func(rt *rapid.T) {
		svc, job := newDrawbackTestServer(t, jobID, drawback.TriggerOnFailSkillCheck, effectCondID)
		svc.dice = nil // rollD20() returns 10; DC=9999 → CritFailure.

		uid := "prop_" + rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid_suffix")

		_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "prop_user",
			CharName:  "PropChar",
			RoomID:    "room_a",
			CurrentHP: 50,
			MaxHP:     50,
			Role:      "player",
		})
		require.NoError(rt, err)

		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(rt, ok)
		sess.Jobs = map[string]int{job.ID: 1}
		sess.Conditions = condition.NewActiveSet()

		_ = svc.skillCheckOutcome(sess, "hustle", 9999)

		assert.True(rt, sess.Conditions.Has(effectCondID),
			"condition must be applied after guaranteed skill check failure")
	})
}

// TestTakeMassiveDamage_DrawbackConditionApplied verifies that when a player takes
// ≥50% of their max HP in a single combat hit, the
// on_take_damage_in_one_hit_above_threshold drawback fires and applies the condition.
//
// REQ-JD-10.
func TestTakeMassiveDamage_DrawbackConditionApplied(t *testing.T) {
	const (
		uid          = "db_dmg_u1"
		jobID        = "test_dmg_job"
		effectCondID = "shaken"
		playerMaxHP  = 20
		// hitDamage = 10 = 50% of 20 → dmg*2 >= maxHP → trigger must fire.
		hitDamage = 10
	)

	svc, job := newDrawbackTestServer(t, jobID, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, effectCondID)

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "db_dmg_user",
		CharName:  "DMGChar",
		RoomID:    "room_a",
		CurrentHP: playerMaxHP,
		MaxHP:     playerMaxHP,
		Role:      "player",
	})
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Jobs = map[string]int{job.ID: 1}
	sess.Conditions = condition.NewActiveSet()

	// Build a minimal combat struct to replicate the threshold check in resolveAndAdvanceLocked.
	playerCombatant := &combat.Combatant{
		ID:        uid,
		Name:      "DMGChar",
		Kind:      combat.KindPlayer,
		CurrentHP: playerMaxHP,
		MaxHP:     playerMaxHP,
		AC:        10,
	}
	cbt := &combat.Combat{
		RoomID:      "room_a",
		Combatants:  []*combat.Combatant{playerCombatant},
		DamageDealt: make(map[string]int),
	}

	// Replicate the threshold logic from resolveAndAdvanceLocked / onMassiveDamage callback.
	onMassiveDamage := func(targetUID string) {
		playerSess, sesOK := svc.sessions.GetPlayer(targetUID)
		if !sesOK || svc.jobRegistry == nil || playerSess.Conditions == nil {
			return
		}
		heldJobs := svc.resolveHeldJobs(playerSess)
		svc.drawbackEngine.FireTrigger(targetUID, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, heldJobs, playerSess.Conditions, time.Now())
	}

	var maxHPFromCbt int
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			maxHPFromCbt = c.MaxHP
			break
		}
	}
	if maxHPFromCbt > 0 && hitDamage*2 >= maxHPFromCbt {
		onMassiveDamage(uid)
	}

	assert.True(t, sess.Conditions.Has(effectCondID),
		"condition %q must be applied after on_take_damage_in_one_hit_above_threshold fires", effectCondID)
}

// TestPropertyTakeMassiveDamage_ThresholdCheck is a property-based test verifying that
// any hit dealing ≥50% of the player's max HP fires the drawback trigger.
//
// REQ-JD-10, SWENG-5a.
func TestPropertyTakeMassiveDamage_ThresholdCheck(t *testing.T) {
	const (
		jobID        = "prop_dmg_job"
		effectCondID = "shaken"
	)

	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(10, 100).Draw(rt, "maxHP")
		// hitDamage must satisfy hitDamage*2 >= maxHP → hitDamage >= ceil(maxHP/2).
		hitDamage := rapid.IntRange((maxHP+1)/2, maxHP).Draw(rt, "hitDamage")

		svc, job := newDrawbackTestServer(t, jobID, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, effectCondID)

		uid := "prop_" + rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid_suffix")

		_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "prop_dmg_user",
			CharName:  "PropDMGChar",
			RoomID:    "room_a",
			CurrentHP: maxHP,
			MaxHP:     maxHP,
			Role:      "player",
		})
		require.NoError(rt, err)

		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(rt, ok)
		sess.Jobs = map[string]int{job.ID: 1}
		sess.Conditions = condition.NewActiveSet()

		playerCombatant := &combat.Combatant{
			ID:    uid,
			Kind:  combat.KindPlayer,
			MaxHP: maxHP,
		}
		cbt := &combat.Combat{
			RoomID:      "room_a",
			Combatants:  []*combat.Combatant{playerCombatant},
			DamageDealt: make(map[string]int),
		}

		onMassiveDamage := func(targetUID string) {
			playerSess, sesOK := svc.sessions.GetPlayer(targetUID)
			if !sesOK || svc.jobRegistry == nil || playerSess.Conditions == nil {
				return
			}
			heldJobs := svc.resolveHeldJobs(playerSess)
			svc.drawbackEngine.FireTrigger(targetUID, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, heldJobs, playerSess.Conditions, time.Now())
		}

		var maxHPFromCbt int
		for _, c := range cbt.Combatants {
			if c.ID == uid {
				maxHPFromCbt = c.MaxHP
				break
			}
		}
		if maxHPFromCbt > 0 && hitDamage*2 >= maxHPFromCbt {
			onMassiveDamage(uid)
		}

		assert.True(rt, sess.Conditions.Has(effectCondID),
			"condition must be applied when hitDamage=%d >= 50%% of maxHP=%d", hitDamage, maxHP)
	})
}

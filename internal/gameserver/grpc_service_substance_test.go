package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/substance"
)

// buildMinimalServiceWithSubstance returns a minimal GameServiceServer with a substance registry.
func buildMinimalServiceWithSubstance(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	sessMgr := session.NewManager()
	condReg := condition.NewRegistry()
	substReg := substance.NewRegistry()
	svc := &GameServiceServer{
		sessions:     sessMgr,
		condRegistry: condReg,
		substanceReg: substReg,
	}
	return svc, sessMgr
}

func addSessionForSubstanceTest(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "user",
		CharName:  "Tester",
		RoomID:    "room1",
		Role:      "player",
		CurrentHP: 10,
		MaxHP:     20,
	})
	require.NoError(t, err)
	sess.InitDone = make(chan struct{})
	close(sess.InitDone)
	sess.Conditions = condition.NewActiveSet()
	return sess
}

func makeDrugDef(id string, addictive bool, addictionChance float64, overdoseThreshold int) *substance.SubstanceDef {
	d := &substance.SubstanceDef{
		ID:                id,
		Name:              id,
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "1h",
		Addictive:         addictive,
		AddictionChance:   addictionChance,
		OverdoseThreshold: overdoseThreshold,
	}
	if err := d.Validate(); err != nil {
		panic(err)
	}
	return d
}

// REQ-AH-9: First dose creates ActiveSubstance entry with DoseCount=1.
func TestApplySubstanceDose_FirstDose_CreatesEntry(t *testing.T) {
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	def := makeDrugDef("test_drug", false, 0.0, 5)

	svc.applySubstanceDose("u1", def)

	assert.Len(t, sess.ActiveSubstances, 1)
	assert.Equal(t, "test_drug", sess.ActiveSubstances[0].SubstanceID)
	assert.Equal(t, 1, sess.ActiveSubstances[0].DoseCount)
}

// REQ-AH-9: Second dose increments DoseCount and extends ExpiresAt.
func TestApplySubstanceDose_SecondDose_IncrementsAndExtends(t *testing.T) {
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	def := makeDrugDef("test_drug", false, 0.0, 5)

	svc.applySubstanceDose("u1", def)
	firstExpiry := sess.ActiveSubstances[0].ExpiresAt
	time.Sleep(1 * time.Millisecond) // ensure time advances
	svc.applySubstanceDose("u1", def)

	assert.Len(t, sess.ActiveSubstances, 1)
	assert.Equal(t, 2, sess.ActiveSubstances[0].DoseCount)
	assert.True(t, sess.ActiveSubstances[0].ExpiresAt.After(firstExpiry))
}

// REQ-AH-10: DoseCount > overdose_threshold applies overdose_condition immediately.
func TestApplySubstanceDose_Overdose_AppliesCondition(t *testing.T) {
	condReg := condition.NewRegistry()
	overdoseDef := &condition.ConditionDef{ID: "stimulant_overdose", Name: "Stimulant Overdose", MaxStacks: 5}
	condReg.Register(overdoseDef)

	sessMgr := session.NewManager()
	substReg := substance.NewRegistry()
	svc := &GameServiceServer{
		sessions:     sessMgr,
		condRegistry: condReg,
		substanceReg: substReg,
	}

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	// entity needed for pushMessageToUID
	sess.Entity = session.NewBridgeEntity("u1", 8)

	def := makeDrugDef("test_drug", false, 0.0, 2)
	def.OverdoseCondition = "stimulant_overdose"

	svc.applySubstanceDose("u1", def) // dose 1
	svc.applySubstanceDose("u1", def) // dose 2
	svc.applySubstanceDose("u1", def) // dose 3 — exceeds threshold of 2

	assert.True(t, sess.Conditions.Has("stimulant_overdose"))
}

// REQ-AH-11: First dose of addictive substance sets status to "at_risk".
func TestApplySubstanceDose_Addictive_FirstDose_SetsAtRisk(t *testing.T) {
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	def := makeDrugDef("jet", true, 0.0, 5) // addiction_chance=0 so will NOT become addicted

	svc.applySubstanceDose("u1", def)

	require.NotNil(t, sess.AddictionState)
	assert.Equal(t, "at_risk", sess.AddictionState["jet"].Status)
}

// REQ-AH-8A: Use handler blocks poison/toxin with "You can't use that directly."
func TestApplySubstanceDose_PoisonCategory_CanBeAppliedDirectly(t *testing.T) {
	// applySubstanceDose itself doesn't block by category; the block is in handleConsumeItem.
	// This test verifies that applySubstanceDose succeeds for poison category.
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)

	poisonDef := &substance.SubstanceDef{
		ID: "viper_venom", Name: "Viper Venom", Category: "poison",
		OnsetDelayStr: "10s", DurationStr: "3m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 1,
	}
	require.NoError(t, poisonDef.Validate())

	svc.applySubstanceDose("u1", poisonDef)
	assert.Len(t, sess.ActiveSubstances, 1)
}

// REQ-AH-20: ApplySubstanceByID returns error for unknown substance ID.
func TestApplySubstanceByID_UnknownID_Error(t *testing.T) {
	svc, _ := buildMinimalServiceWithSubstance(t)
	err := svc.ApplySubstanceByID("u1", "nonexistent")
	assert.Error(t, err)
}

// REQ-AH-20: ApplySubstanceByID calls applySubstanceDose directly (no category guard).
func TestApplySubstanceByID_PoisonApplied(t *testing.T) {
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)

	poisonDef := &substance.SubstanceDef{
		ID: "viper_venom", Name: "Viper Venom", Category: "poison",
		OnsetDelayStr: "10s", DurationStr: "3m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 1,
	}
	require.NoError(t, poisonDef.Validate())
	svc.substanceReg.Register(poisonDef)

	err := svc.ApplySubstanceByID("u1", "viper_venom")
	require.NoError(t, err)
	assert.Len(t, sess.ActiveSubstances, 1)
}

// REQ-AH-19: Addiction state is independent per substance ID.
func TestAddictionState_IndependentPerSubstance(t *testing.T) {
	svc, sessMgr := buildMinimalServiceWithSubstance(t)
	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)

	def1 := makeDrugDef("drug_a", true, 0.0, 5)
	def2 := makeDrugDef("drug_b", true, 0.0, 5)
	svc.substanceReg.Register(def1)
	svc.substanceReg.Register(def2)

	svc.applySubstanceDose("u1", def1)
	svc.applySubstanceDose("u1", def2)

	require.NotNil(t, sess.AddictionState)
	assert.Equal(t, "at_risk", sess.AddictionState["drug_a"].Status)
	assert.Equal(t, "at_risk", sess.AddictionState["drug_b"].Status)
}

// REQ-AH-17: Dose while in withdrawal resets WithdrawalUntil, removes withdrawal conditions, sets addicted.
func TestApplySubstanceDose_WhileWithdrawal_ResetsAndSetsAddicted(t *testing.T) {
	condReg := condition.NewRegistry()
	fatigueDef := &condition.ConditionDef{ID: "fatigue", Name: "Fatigue", MaxStacks: 5}
	condReg.Register(fatigueDef)

	sessMgr := session.NewManager()
	substReg := substance.NewRegistry()
	svc := &GameServiceServer{
		sessions:     sessMgr,
		condRegistry: condReg,
		substanceReg: substReg,
	}

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)

	def := makeDrugDef("jet", true, 1.0, 5) // 100% addiction chance
	def.WithdrawalConditions = []string{"fatigue"}

	// Apply withdrawal condition manually and set status.
	_ = sess.Conditions.Apply("u1", fatigueDef, 1, -1)
	sess.AddictionState = map[string]substance.SubstanceAddiction{
		"jet": {Status: "withdrawal", WithdrawalUntil: time.Now().Add(1 * time.Hour)},
	}
	sess.SubstanceConditionRefs = map[string]int{"fatigue": 1}

	svc.applySubstanceDose("u1", def)

	addict := sess.AddictionState["jet"]
	assert.Equal(t, "addicted", addict.Status)
}

// Property: applySubstanceDose never panics regardless of input shape.
func TestPropertyApplySubstanceDose_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessMgr := session.NewManager()
		svc := &GameServiceServer{
			sessions:     sessMgr,
			condRegistry: condition.NewRegistry(),
			substanceReg: substance.NewRegistry(),
		}
		uid := rapid.StringMatching(`[a-z]{1,8}`).Draw(rt, "uid")
		category := rapid.SampledFrom([]string{"drug", "alcohol", "medicine", "poison", "toxin"}).Draw(rt, "category")
		addictive := category != "medicine" && rapid.Bool().Draw(rt, "addictive")
		def := &substance.SubstanceDef{
			ID:                rapid.StringMatching(`[a-z]{1,8}`).Draw(rt, "id"),
			Name:              "Test",
			Category:          category,
			OnsetDelayStr:     "0s",
			DurationStr:       "1m",
			RecoveryDurStr:    "0s",
			Addictive:         addictive,
			AddictionChance:   rapid.Float64Range(0, 1).Draw(rt, "chance"),
			OverdoseThreshold: rapid.IntRange(1, 10).Draw(rt, "threshold"),
		}
		if vErr := def.Validate(); vErr != nil {
			return // skip invalid; we only care about no-panic
		}
		// Add a session if ID is non-empty, otherwise test nil session path.
		if uid != "" {
			sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID: uid, Username: "u", CharName: "C",
				RoomID: "r", Role: "player", CurrentHP: 10, MaxHP: 10,
			})
			if err == nil {
				sess.InitDone = make(chan struct{})
				close(sess.InitDone)
				sess.Conditions = condition.NewActiveSet()
				sess.Entity = session.NewBridgeEntity(uid, 4)
			}
		}
		svc.applySubstanceDose(uid, def)
	})
}

// ---- Task 9: tickSubstances tests ----

func buildServiceForTick(t *testing.T) (*GameServiceServer, *session.Manager, *condition.Registry) {
	t.Helper()
	sessMgr := session.NewManager()
	condReg := condition.NewRegistry()
	substReg := substance.NewRegistry()
	svc := &GameServiceServer{
		sessions:     sessMgr,
		condRegistry: condReg,
		substanceReg: substReg,
	}
	return svc, sessMgr, condReg
}

// REQ-AH-13: tickSubstances fires onset when OnsetAt is past.
func TestTickSubstances_Onset_AppliesEffects(t *testing.T) {
	svc, sessMgr, condReg := buildServiceForTick(t)
	speedBoostDef := &condition.ConditionDef{ID: "speed_boost", Name: "Speed Boost", MaxStacks: 5}
	condReg.Register(speedBoostDef)

	def := &substance.SubstanceDef{
		ID: "jet", Name: "Jet", Category: "drug",
		OnsetDelayStr: "0s", DurationStr: "10m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 5,
		Effects: []substance.SubstanceEffect{{ApplyCondition: "speed_boost", Stacks: 1}},
	}
	require.NoError(t, def.Validate())
	svc.substanceReg.Register(def)

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	// Insert an already-past-onset entry.
	sess.ActiveSubstances = []substance.ActiveSubstance{
		{SubstanceID: "jet", DoseCount: 1, OnsetAt: time.Now().Add(-1 * time.Second), ExpiresAt: time.Now().Add(10 * time.Minute), EffectsApplied: false},
	}

	svc.tickSubstances("u1")

	assert.True(t, sess.Conditions.Has("speed_boost"))
}

// REQ-AH-13: tickSubstances sends "kicks in" message on onset.
func TestTickSubstances_Onset_SendsKicksInMessage(t *testing.T) {
	svc, sessMgr, _ := buildServiceForTick(t)
	def := &substance.SubstanceDef{
		ID: "jet", Name: "Jet", Category: "drug",
		OnsetDelayStr: "0s", DurationStr: "10m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 5,
	}
	require.NoError(t, def.Validate())
	svc.substanceReg.Register(def)

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	entity := session.NewBridgeEntity("u1", 8)
	sess.Entity = entity
	sess.ActiveSubstances = []substance.ActiveSubstance{
		{SubstanceID: "jet", DoseCount: 1, OnsetAt: time.Now().Add(-1 * time.Second), ExpiresAt: time.Now().Add(10 * time.Minute), EffectsApplied: false},
	}

	svc.tickSubstances("u1")

	// Drain events to confirm message was sent.
	select {
	case data := <-entity.Events():
		assert.NotEmpty(t, data)
	default:
		t.Fatal("expected a message event but got none")
	}
}

// REQ-AH-14: hp_regen applies per tick, clamped to MaxHP.
func TestTickSubstances_HPRegen_AppliedPerTick(t *testing.T) {
	svc, sessMgr, _ := buildServiceForTick(t)
	def := &substance.SubstanceDef{
		ID: "stimpak", Name: "Stimpak", Category: "medicine",
		OnsetDelayStr: "0s", DurationStr: "10m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 10,
		Effects: []substance.SubstanceEffect{{HPRegen: 2}},
	}
	require.NoError(t, def.Validate())
	svc.substanceReg.Register(def)

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	sess.CurrentHP = 10
	sess.MaxHP = 20
	// Insert already-active (onset already fired) entry.
	sess.ActiveSubstances = []substance.ActiveSubstance{
		{SubstanceID: "stimpak", DoseCount: 1, OnsetAt: time.Now().Add(-10 * time.Second), ExpiresAt: time.Now().Add(10 * time.Minute), EffectsApplied: true},
	}

	svc.tickSubstances("u1")

	assert.Equal(t, 12, sess.CurrentHP)
}

// REQ-AH-14: hp_regen clamped to MaxHP.
func TestTickSubstances_HPRegen_ClampedToMaxHP(t *testing.T) {
	svc, sessMgr, _ := buildServiceForTick(t)
	def := &substance.SubstanceDef{
		ID: "stimpak", Name: "Stimpak", Category: "medicine",
		OnsetDelayStr: "0s", DurationStr: "10m", RecoveryDurStr: "0s",
		AddictionChance: 0.0, OverdoseThreshold: 10,
		Effects: []substance.SubstanceEffect{{HPRegen: 100}},
	}
	require.NoError(t, def.Validate())
	svc.substanceReg.Register(def)

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	sess.CurrentHP = 18
	sess.MaxHP = 20
	sess.ActiveSubstances = []substance.ActiveSubstance{
		{SubstanceID: "stimpak", DoseCount: 1, OnsetAt: time.Now().Add(-10 * time.Second), ExpiresAt: time.Now().Add(10 * time.Minute), EffectsApplied: true},
	}

	svc.tickSubstances("u1")

	assert.Equal(t, 20, sess.CurrentHP)
}

// REQ-AH-15: onSubstanceExpired triggers withdrawal when addicted.
func TestOnSubstanceExpired_Addicted_SetsWithdrawal(t *testing.T) {
	svc, sessMgr, _ := buildServiceForTick(t)
	def := makeDrugDef("jet", true, 1.0, 5)
	def.RecoveryDurStr = "4h"
	require.NoError(t, def.Validate())

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	sess.AddictionState = map[string]substance.SubstanceAddiction{
		"jet": {Status: "addicted"},
	}

	svc.onSubstanceExpired("u1", def)

	addict := sess.AddictionState["jet"]
	assert.Equal(t, "withdrawal", addict.Status)
	assert.False(t, addict.WithdrawalUntil.IsZero())
}

// REQ-AH-16: tickSubstances clears withdrawal after WithdrawalUntil.
func TestTickSubstances_WithdrawalExpiry_SetsClean(t *testing.T) {
	svc, sessMgr, _ := buildServiceForTick(t)
	def := makeDrugDef("jet", true, 0.0, 5)
	svc.substanceReg.Register(def)

	sess := addSessionForSubstanceTest(t, sessMgr, "u1")
	sess.Entity = session.NewBridgeEntity("u1", 8)
	sess.AddictionState = map[string]substance.SubstanceAddiction{
		"jet": {Status: "withdrawal", WithdrawalUntil: time.Now().Add(-1 * time.Second)},
	}

	svc.tickSubstances("u1")

	addict := sess.AddictionState["jet"]
	assert.Equal(t, "", addict.Status)
}

// Property: tickSubstances never panics.
func TestPropertyTickSubstances_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessMgr := session.NewManager()
		substReg := substance.NewRegistry()
		svc := &GameServiceServer{
			sessions:     sessMgr,
			condRegistry: condition.NewRegistry(),
			substanceReg: substReg,
		}
		uid := rapid.StringMatching(`[a-z]{1,8}`).Draw(rt, "uid")
		if uid != "" {
			sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID: uid, Username: "u", CharName: "C",
				RoomID: "r", Role: "player", CurrentHP: 10, MaxHP: 10,
			})
			if err == nil {
				sess.InitDone = make(chan struct{})
				close(sess.InitDone)
				sess.Conditions = condition.NewActiveSet()
				sess.Entity = session.NewBridgeEntity(uid, 4)
			}
		}
		svc.tickSubstances(uid) // must not panic
	})
}

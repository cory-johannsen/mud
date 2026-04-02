package xp_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
)

type fakeProgressSaver struct {
	savedLevel  int
	savedXP     int
	savedMaxHP  int
	savedBoosts int
	called      bool
}

func (f *fakeProgressSaver) SaveProgress(_ context.Context, _ int64, level, experience, maxHP, pendingBoosts int) error {
	f.called = true
	f.savedLevel = level
	f.savedXP = experience
	f.savedMaxHP = maxHP
	f.savedBoosts = pendingBoosts
	return nil
}

func testCfg() *xp.XPConfig {
	return &xp.XPConfig{
		BaseXP:        100,
		HPPerLevel:    5,
		BoostInterval: 5,
		SkillLevels:   []int{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100},
		LevelCap:      100,
		Awards: xp.Awards{
			KillXPPerNPCLevel:       50,
			NewRoomXP:               10,
			SkillCheckSuccessXP:     10,
			SkillCheckCritSuccessXP: 25,
			SkillCheckDCMultiplier:  2,
		},
	}
}

func testSess(level, currentXP, maxHP int) *session.PlayerSession {
	sess := &session.PlayerSession{}
	sess.Level = level
	sess.Experience = currentXP
	sess.MaxHP = maxHP
	sess.CurrentHP = maxHP
	return sess
}

func TestService_AwardKill_NoLevelUp(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 0, "")
	require.NoError(t, err)
	assert.Empty(t, msgs)
	assert.Equal(t, 50, sess.Experience)
	assert.False(t, saver.called)
}

func TestService_AwardKill_LevelUp(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 350, 10)

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 1, "")
	require.NoError(t, err)
	assert.Equal(t, 2, sess.Level)
	assert.Equal(t, 400, sess.Experience)
	assert.Equal(t, 15, sess.MaxHP)
	assert.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "level 2")
	assert.True(t, saver.called)
	assert.Equal(t, 2, saver.savedLevel)
}

func TestService_AwardRoomDiscovery(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
	require.NoError(t, err)
	assert.Equal(t, 10, sess.Experience)
}

func TestService_AwardSkillCheck_Success(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardSkillCheck(context.Background(), sess, "parkour", 14, false, 0)
	require.NoError(t, err)
	// 10 + 14×2 = 38
	assert.Equal(t, 38, sess.Experience)
}

func TestService_AwardSkillCheck_CritSuccess(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardSkillCheck(context.Background(), sess, "parkour", 14, true, 0)
	require.NoError(t, err)
	// 25 + 14×2 = 53
	assert.Equal(t, 53, sess.Experience)
}

func TestService_BoostPending_NotifiedAtInterval(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	// Start at level 4, just under level-5 threshold
	startXP := xp.XPToLevel(5, 100) - 1 // 2499
	sess := testSess(4, startXP, 30)

	msgs, err := svc.AwardKill(context.Background(), sess, 3, 1, "") // +150 XP → 2649, level 5
	require.NoError(t, err)
	assert.Equal(t, 5, sess.Level)
	assert.Equal(t, 1, sess.PendingBoosts)
	// Last message should mention levelup command
	assert.Contains(t, msgs[len(msgs)-1], "levelup")
	assert.Equal(t, 1, saver.savedBoosts)
}

type mockSkillIncreaseSaver struct {
	mu    sync.Mutex
	calls []int
	err   error
}

func (m *mockSkillIncreaseSaver) IncrementPendingSkillIncreases(_ context.Context, _ int64, n int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, n)
	return nil
}

func TestService_SkillIncrease_OnIntervalLevelUp(t *testing.T) {
	saver := &fakeProgressSaver{}
	skillSaver := &mockSkillIncreaseSaver{}
	svc := xp.NewService(testCfg(), saver)
	svc.SetSkillIncreaseSaver(skillSaver)

	// Level 3 → 4 (multiple of skill_interval=4): should get 1 skill increase.
	// XPToLevel(3,100)=900, XPToLevel(4,100)=1600; start at 900, need 700 more XP.
	// AwardKill npcLevel=14 gives 700 XP.
	sess := testSess(3, 900, 20)
	msgs, err := svc.AwardKill(context.Background(), sess, 14, 1, "")
	require.NoError(t, err)
	assert.Equal(t, 4, sess.Level)
	assert.Equal(t, 1, sess.PendingSkillIncreases)
	assert.Equal(t, []int{1}, skillSaver.calls)
	// Check message is present
	found := false
	for _, m := range msgs {
		if len(m) > 0 && m == "You have a pending skill increase! Type 'trainskill <skill>' to advance a skill." {
			found = true
		}
	}
	assert.True(t, found, "expected skill increase message in: %v", msgs)
}

func TestService_SkillIncrease_NonIntervalLevelUp_NoIncrease(t *testing.T) {
	saver := &fakeProgressSaver{}
	skillSaver := &mockSkillIncreaseSaver{}
	svc := xp.NewService(testCfg(), saver)
	svc.SetSkillIncreaseSaver(skillSaver)

	// Level 1 → 2 (not a multiple of skill_interval=4): should get 0 skill increases.
	// XPToLevel(1,100)=100, XPToLevel(2,100)=400; start at 100, need 300 more XP.
	// AwardKill npcLevel=6 gives 300 XP.
	sess := testSess(1, 100, 10)
	_, err := svc.AwardKill(context.Background(), sess, 6, 1, "")
	require.NoError(t, err)
	assert.Equal(t, 2, sess.Level)
	assert.Equal(t, 0, sess.PendingSkillIncreases)
	assert.Empty(t, skillSaver.calls)
}

func TestService_SkillIncrease_NoLevelUp_NoIncrease(t *testing.T) {
	saver := &fakeProgressSaver{}
	skillSaver := &mockSkillIncreaseSaver{}
	svc := xp.NewService(testCfg(), saver)
	svc.SetSkillIncreaseSaver(skillSaver)

	sess := testSess(1, 0, 10)
	_, err := svc.AwardKill(context.Background(), sess, 1, 1, "")
	require.NoError(t, err)
	assert.Equal(t, 1, sess.Level)
	assert.Equal(t, 0, sess.PendingSkillIncreases)
	assert.Empty(t, skillSaver.calls)
}

func TestService_CurrentHPCappedAtMaxHP(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 350, 10)
	sess.CurrentHP = 10 // at full HP

	_, err := svc.AwardKill(context.Background(), sess, 1, 1, "")
	require.NoError(t, err)
	// MaxHP went from 10 to 15; CurrentHP was at 10 = old MaxHP, new MaxHP=15, CurrentHP stays 10.
	assert.Equal(t, 15, sess.MaxHP)
	assert.Equal(t, 10, sess.CurrentHP)
}

func TestService_AwardRoomDiscovery_ReturnsGrantMessage(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	msgs, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
	require.NoError(t, err)
	require.NotEmpty(t, msgs, "must return at least the XP grant message")
	assert.Contains(t, msgs[0], "You gain")
	assert.Contains(t, msgs[0], "XP")
	assert.Contains(t, msgs[0], "discovering a new room")
}

func TestService_AwardSkillCheck_ReturnsGrantMessage(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	msgs, err := svc.AwardSkillCheck(context.Background(), sess, "parkour", 14, false, 0)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "You gain")
	assert.Contains(t, msgs[0], "XP")
	assert.Contains(t, msgs[0], "Parkour")
}

// TestService_AwardSkillCheck_SnakeCaseNameConverted verifies that a snake_case
// skill ID is title-cased in the XP grant message.
func TestService_AwardSkillCheck_SnakeCaseNameConverted(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	msgs, err := svc.AwardSkillCheck(context.Background(), sess, "smooth_talk", 12, false, 0)
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "Smooth Talk")
	assert.NotContains(t, msgs[0], "smooth_talk")
}

func TestService_AwardXPAmount_AwardsCorrectXP(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardXPAmount(context.Background(), sess, 0, 25)
	require.NoError(t, err)
	assert.Equal(t, 25, sess.Experience, "expected 25 XP awarded")
}

func TestService_AwardXPAmount_ZeroXP_NoChange(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 50, 10)

	_, err := svc.AwardXPAmount(context.Background(), sess, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, sess.Experience, "0 XP award should not change experience")
}

func TestService_AwardXPAmount_SameAsAwardKill_WhenFullAmount(t *testing.T) {
	// Verify AwardXPAmount with the full kill amount equals AwardKill behavior.
	cfg := testCfg()
	saver1 := &fakeProgressSaver{}
	svc1 := xp.NewService(cfg, saver1)
	sess1 := testSess(1, 0, 10)
	_, err := svc1.AwardKill(context.Background(), sess1, 1, 0, "")
	require.NoError(t, err)

	saver2 := &fakeProgressSaver{}
	svc2 := xp.NewService(cfg, saver2)
	sess2 := testSess(1, 0, 10)
	fullXP := cfg.Awards.KillXPPerNPCLevel * 1 // npcLevel=1
	_, err = svc2.AwardXPAmount(context.Background(), sess2, 0, fullXP)
	require.NoError(t, err)

	assert.Equal(t, sess1.Experience, sess2.Experience, "AwardXPAmount(fullXP) should equal AwardKill")
}

// REQ-T-PROP-A (property, SWENG-5a): AwardXPAmount(0) never changes Experience.
func TestProperty_AwardXPAmount_ZeroAwardNoChange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		saver := &fakeProgressSaver{}
		svc := xp.NewService(testCfg(), saver)
		startXP := rapid.IntRange(0, 1000).Draw(rt, "startXP")
		sess := testSess(1, startXP, 10)

		_, err := svc.AwardXPAmount(context.Background(), sess, 0, 0)
		require.NoError(rt, err)
		require.Equal(rt, startXP, sess.Experience, "zero XP award must not change Experience")
	})
}

// REQ-T-PROP-B (property, SWENG-5a): AwardXPAmount(n) increases Experience by exactly n
// when no level-up boundary is crossed (startXP=0, xpAmount well below level threshold).
func TestProperty_AwardXPAmount_PositiveAwardIncreasesExperience(t *testing.T) {
	cfg := testCfg()
	rapid.Check(t, func(rt *rapid.T) {
		saver := &fakeProgressSaver{}
		svc := xp.NewService(cfg, saver)
		// xpAmount capped at 100 to stay safely below any level threshold.
		xpAmount := rapid.IntRange(1, 100).Draw(rt, "xpAmount")
		sess := testSess(1, 0, 10) // start with 0 XP

		_, err := svc.AwardXPAmount(context.Background(), sess, 0, xpAmount)
		require.NoError(rt, err)
		require.Equal(rt, xpAmount, sess.Experience, "Experience must increase by exactly xpAmount")
	})
}

func TestPropertyService_AwardRoomDiscovery_GrantMessageAlwaysFirst(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		saver := &fakeProgressSaver{}
		svc := xp.NewService(testCfg(), saver)
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		xpAmt := rapid.IntRange(0, 999).Draw(rt, "xp")
		sess := testSess(level, xpAmt, 10)
		msgs, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
		if err != nil {
			rt.Fatal(err)
		}
		if len(msgs) == 0 {
			rt.Fatal("expected at least one message")
		}
		if !strings.Contains(msgs[0], "You gain") {
			rt.Fatalf("first message must be XP grant, got: %q", msgs[0])
		}
	})
}

func TestService_AwardKill_TierScaling_Elite(t *testing.T) {
	cfg := &xp.XPConfig{
		BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, SkillLevels: []int{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100}, LevelCap: 100,
		Awards: xp.Awards{KillXPPerNPCLevel: 50},
		TierMultipliers: map[string]xp.TierMultiplier{
			"standard": {XP: 1.0}, "elite": {XP: 2.0},
		},
	}
	saver := &fakeProgressSaver{}
	svc := xp.NewService(cfg, saver)
	sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}

	// NPC level 3 elite: base = 3*50 = 150, then *2.0 = 300
	_, err := svc.AwardKill(context.Background(), sess, 3, 1, "elite")
	require.NoError(t, err)
	assert.Equal(t, 300, sess.Experience)
}

func TestService_AwardKill_EmptyTierDefaultsToStandard(t *testing.T) {
	cfg := &xp.XPConfig{
		BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, SkillLevels: []int{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100}, LevelCap: 100,
		Awards: xp.Awards{KillXPPerNPCLevel: 50},
		TierMultipliers: map[string]xp.TierMultiplier{
			"standard": {XP: 1.0},
		},
	}
	saver := &fakeProgressSaver{}
	svc := xp.NewService(cfg, saver)
	sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}

	_, err := svc.AwardKill(context.Background(), sess, 2, 1, "")
	require.NoError(t, err)
	assert.Equal(t, 100, sess.Experience) // 2*50*1.0
}

func TestProperty_AwardKill_TierMultipliesMonotonically(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(t, "level")
		mults := map[string]xp.TierMultiplier{
			"minion": {XP: 0.5}, "standard": {XP: 1.0},
			"elite": {XP: 2.0}, "champion": {XP: 3.0}, "boss": {XP: 5.0},
		}
		cfg := &xp.XPConfig{
			BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, SkillLevels: []int{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100}, LevelCap: 100,
			Awards: xp.Awards{KillXPPerNPCLevel: 50},
			TierMultipliers: mults,
		}
		saver := &fakeProgressSaver{}

		tierOrder := []string{"minion", "standard", "elite", "champion", "boss"}
		prevXP := 0
		for _, tier := range tierOrder {
			sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}
			_, err := xp.NewService(cfg, saver).AwardKill(context.Background(), sess, level, 1, tier)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, sess.Experience, prevXP,
				"XP for tier %q must be >= tier below it", tier)
			prevXP = sess.Experience
		}
	})
}

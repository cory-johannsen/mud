package xp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 0)
	require.NoError(t, err)
	assert.Empty(t, msgs)
	assert.Equal(t, 50, sess.Experience)
	assert.False(t, saver.called)
}

func TestService_AwardKill_LevelUp(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 350, 10)

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 1)
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

	_, err := svc.AwardSkillCheck(context.Background(), sess, 14, false, 0)
	require.NoError(t, err)
	// 10 + 14×2 = 38
	assert.Equal(t, 38, sess.Experience)
}

func TestService_AwardSkillCheck_CritSuccess(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardSkillCheck(context.Background(), sess, 14, true, 0)
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

	msgs, err := svc.AwardKill(context.Background(), sess, 3, 1) // +150 XP → 2649, level 5
	require.NoError(t, err)
	assert.Equal(t, 5, sess.Level)
	assert.Equal(t, 1, sess.PendingBoosts)
	// Last message should mention levelup command
	assert.Contains(t, msgs[len(msgs)-1], "levelup")
	assert.Equal(t, 1, saver.savedBoosts)
}

func TestService_CurrentHPCappedAtMaxHP(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 350, 10)
	sess.CurrentHP = 10 // at full HP

	_, err := svc.AwardKill(context.Background(), sess, 1, 1)
	require.NoError(t, err)
	// MaxHP went from 10 to 15; CurrentHP was at 10 = old MaxHP, new MaxHP=15, CurrentHP stays 10.
	assert.Equal(t, 15, sess.MaxHP)
	assert.Equal(t, 10, sess.CurrentHP)
}

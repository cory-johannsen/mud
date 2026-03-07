package xp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/xp"
)

func TestLoadXPConfig_ParsesAllFields(t *testing.T) {
	yaml := `
base_xp: 100
hp_per_level: 5
boost_interval: 5
level_cap: 100
job_level_cap: 20
awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
`
	tmp := filepath.Join(t.TempDir(), "xp_config.yaml")
	require.NoError(t, os.WriteFile(tmp, []byte(yaml), 0644))

	cfg, err := xp.LoadXPConfig(tmp)
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.BaseXP)
	assert.Equal(t, 5, cfg.HPPerLevel)
	assert.Equal(t, 5, cfg.BoostInterval)
	assert.Equal(t, 100, cfg.LevelCap)
	assert.Equal(t, 20, cfg.JobLevelCap)
	assert.Equal(t, 50, cfg.Awards.KillXPPerNPCLevel)
	assert.Equal(t, 10, cfg.Awards.NewRoomXP)
	assert.Equal(t, 10, cfg.Awards.SkillCheckSuccessXP)
	assert.Equal(t, 25, cfg.Awards.SkillCheckCritSuccessXP)
	assert.Equal(t, 2, cfg.Awards.SkillCheckDCMultiplier)
}

func TestLoadXPConfig_MissingFile(t *testing.T) {
	_, err := xp.LoadXPConfig("/nonexistent/xp_config.yaml")
	assert.Error(t, err)
}

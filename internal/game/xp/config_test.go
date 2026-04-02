package xp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/xp"
)

func TestLoadXPConfig_ParsesAllFields(t *testing.T) {
	yamlContent := `
base_xp: 100
hp_per_level: 5
boost_interval: 5
skill_levels: [7, 13, 20]
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
	require.NoError(t, os.WriteFile(tmp, []byte(yamlContent), 0644))

	cfg, err := xp.LoadXPConfig(tmp)
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.BaseXP)
	assert.Equal(t, 5, cfg.HPPerLevel)
	assert.Equal(t, 5, cfg.BoostInterval)
	assert.Equal(t, []int{7, 13, 20}, cfg.SkillLevels)
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
	assert.ErrorContains(t, err, "/nonexistent/xp_config.yaml")
}

func TestLoadXPConfig_MalformedYAML(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(tmp, []byte("key: [[["), 0644))

	_, err := xp.LoadXPConfig(tmp)
	assert.Error(t, err)
}

func TestXPConfig_TierMultipliers_LoadedFromYAML(t *testing.T) {
	data := []byte(`
base_xp: 100
hp_per_level: 5
boost_interval: 5
skill_levels: [7, 13, 20]
level_cap: 100
job_level_cap: 20
awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
  boss_kill_bonus_xp: 200
tier_multipliers:
  minion:    { xp: 0.5, loot: 0.5, hp: 0.75 }
  standard:  { xp: 1.0, loot: 1.0, hp: 1.0  }
  elite:     { xp: 2.0, loot: 1.5, hp: 1.5  }
  champion:  { xp: 3.0, loot: 2.0, hp: 2.0  }
  boss:      { xp: 5.0, loot: 3.0, hp: 3.0  }
`)
	var cfg xp.XPConfig
	err := yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.TierMultipliers, 5)
	assert.InDelta(t, 5.0, cfg.TierMultipliers["boss"].XP, 1e-9)
	assert.InDelta(t, 3.0, cfg.TierMultipliers["boss"].Loot, 1e-9)
	assert.InDelta(t, 3.0, cfg.TierMultipliers["boss"].HP, 1e-9)
	assert.Equal(t, 200, cfg.Awards.BossKillBonusXP)
}

func TestXPConfig_ValidateTiers_MissingTierFatal(t *testing.T) {
	cfg := &xp.XPConfig{
		TierMultipliers: map[string]xp.TierMultiplier{
			"minion": {XP: 0.5, Loot: 0.5, HP: 0.75},
			// missing standard, elite, champion, boss
		},
	}
	err := cfg.ValidateTiers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standard")
}

func TestXPConfig_ValidateTiers_AllPresent(t *testing.T) {
	cfg := &xp.XPConfig{
		TierMultipliers: map[string]xp.TierMultiplier{
			"minion":   {XP: 0.5, Loot: 0.5, HP: 0.75},
			"standard": {XP: 1.0, Loot: 1.0, HP: 1.0},
			"elite":    {XP: 2.0, Loot: 1.5, HP: 1.5},
			"champion": {XP: 3.0, Loot: 2.0, HP: 2.0},
			"boss":     {XP: 5.0, Loot: 3.0, HP: 3.0},
		},
	}
	require.NoError(t, cfg.ValidateTiers())
}

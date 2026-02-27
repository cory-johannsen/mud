package scripting_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/scripting"
)

// repoRoot walks up from the test's working directory to find the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("could not find repo root from %s", wd)
		}
		root = parent
	}
}

// loadScriptDir loads all Lua files in the given directory into the __global__ VM.
func loadScriptDir(t *testing.T, mgr *scripting.Manager, relDir string) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), relDir)
	require.NoError(t, mgr.LoadGlobal(dir, 0))
}

// makeCombatants builds a flat combatant list: nNPCs NPCs ("npc0".."npcN-1")
// followed by nPlayers players ("p0".."pN-1").
func makeCombatants(nNPCs, nPlayers, npcHP, playerHP int) []*scripting.CombatantInfo {
	out := make([]*scripting.CombatantInfo, 0, nNPCs+nPlayers)
	for i := 0; i < nNPCs; i++ {
		out = append(out, &scripting.CombatantInfo{
			UID: fmt.Sprintf("npc%d", i), Name: fmt.Sprintf("NPC %d", i),
			HP: npcHP, MaxHP: 30, AC: 12, Kind: "npc",
		})
	}
	for i := 0; i < nPlayers; i++ {
		out = append(out, &scripting.CombatantInfo{
			UID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Player %d", i),
			HP: playerHP, MaxHP: 100, AC: 14, Kind: "player",
		})
	}
	return out
}

// wireRoom configures GetEntityRoom and GetCombatantsInRoom on mgr using combatants.
func wireRoom(mgr *scripting.Manager, combatants []*scripting.CombatantInfo) {
	mgr.GetEntityRoom = func(uid string) string {
		for _, c := range combatants {
			if c.UID == uid {
				return "room1"
			}
		}
		return ""
	}
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return combatants
	}
}

// --- ganger_has_enemy ---

func TestGanger_HasEnemy_WithEnemy_ReturnsTrue(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	wireRoom(mgr, makeCombatants(1, 1, 20, 80))

	ret, err := mgr.CallHook("__global__", "ganger_has_enemy", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LTrue, ret)
}

func TestGanger_HasEnemy_NoEnemies_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	wireRoom(mgr, makeCombatants(2, 0, 20, 0))

	ret, err := mgr.CallHook("__global__", "ganger_has_enemy", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

// --- ganger_enemy_below_half ---

func TestGanger_EnemyBelowHalf_At40Pct_ReturnsTrue(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	combatants := []*scripting.CombatantInfo{
		{UID: "npc0", Kind: "npc", HP: 20, MaxHP: 30},
		{UID: "p0", Kind: "player", HP: 40, MaxHP: 100}, // 40% HP
	}
	wireRoom(mgr, combatants)

	ret, err := mgr.CallHook("__global__", "ganger_enemy_below_half", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LTrue, ret)
}

func TestGanger_EnemyBelowHalf_At60Pct_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	combatants := []*scripting.CombatantInfo{
		{UID: "npc0", Kind: "npc", HP: 20, MaxHP: 30},
		{UID: "p0", Kind: "player", HP: 60, MaxHP: 100}, // 60% HP
	}
	wireRoom(mgr, combatants)

	ret, err := mgr.CallHook("__global__", "ganger_enemy_below_half", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

func TestGanger_EnemyBelowHalf_ExactlyHalf_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	// 50 HP of 100 max = exactly 50%, not strictly below half.
	combatants := []*scripting.CombatantInfo{
		{UID: "npc0", Kind: "npc", HP: 20, MaxHP: 30},
		{UID: "p0", Kind: "player", HP: 50, MaxHP: 100},
	}
	wireRoom(mgr, combatants)

	ret, err := mgr.CallHook("__global__", "ganger_enemy_below_half", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

func TestGanger_EnemyBelowHalf_NoEnemies_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	wireRoom(mgr, makeCombatants(2, 0, 20, 0))

	ret, err := mgr.CallHook("__global__", "ganger_enemy_below_half", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

// --- scavenger_not_outnumbered ---

func TestScavenger_NotOutnumbered_EqualSides_ReturnsTrue(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	// 2 NPCs vs 1 player: ally=1, enemy=1 → not outnumbered.
	wireRoom(mgr, makeCombatants(2, 1, 20, 80))

	ret, err := mgr.CallHook("__global__", "scavenger_not_outnumbered", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LTrue, ret)
}

func TestScavenger_NotOutnumbered_Outnumbered_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	// 1 NPC vs 3 players: ally=0, enemy=3 → outnumbered.
	wireRoom(mgr, makeCombatants(1, 3, 20, 80))

	ret, err := mgr.CallHook("__global__", "scavenger_not_outnumbered", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

func TestScavenger_NotOutnumbered_SoloVsOnePlayer_ReturnsFalse(t *testing.T) {
	mgr, _ := newTestManager(t)
	loadScriptDir(t, mgr, "content/scripts/ai")
	// 1 NPC vs 1 player: ally=0, enemy=1 → 0 >= 1 is false.
	wireRoom(mgr, makeCombatants(1, 1, 20, 80))

	ret, err := mgr.CallHook("__global__", "scavenger_not_outnumbered", lua.LString("npc0"))
	require.NoError(t, err)
	assert.Equal(t, lua.LFalse, ret)
}

// --- Property tests ---

func TestProperty_GangerEnemyBelowHalf_HPBoundary(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr, _ := newTestManager(t)
		loadScriptDir(t, mgr, "content/scripts/ai")

		maxHP := rapid.IntRange(10, 200).Draw(rt, "max_hp")
		currentHP := rapid.IntRange(0, maxHP).Draw(rt, "hp")

		combatants := []*scripting.CombatantInfo{
			{UID: "npc0", Kind: "npc", HP: 20, MaxHP: 30},
			{UID: "p0", Kind: "player", HP: currentHP, MaxHP: maxHP},
		}
		wireRoom(mgr, combatants)

		ret, err := mgr.CallHook("__global__", "ganger_enemy_below_half", lua.LString("npc0"))
		require.NoError(rt, err)

		// Lua: hp < max_hp * 0.5 (floating point)
		expectBelow := float64(currentHP) < float64(maxHP)*0.5
		if expectBelow {
			assert.Equal(rt, lua.LTrue, ret, "hp=%d max=%d: expected true", currentHP, maxHP)
		} else {
			assert.Equal(rt, lua.LFalse, ret, "hp=%d max=%d: expected false", currentHP, maxHP)
		}
	})
}

func TestProperty_ScavengerNotOutnumbered_AllyVsEnemy(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr, _ := newTestManager(t)
		loadScriptDir(t, mgr, "content/scripts/ai")

		nNPCs := rapid.IntRange(1, 6).Draw(rt, "npcs")
		nPlayers := rapid.IntRange(1, 6).Draw(rt, "players")

		wireRoom(mgr, makeCombatants(nNPCs, nPlayers, 20, 80))

		ret, err := mgr.CallHook("__global__", "scavenger_not_outnumbered", lua.LString("npc0"))
		require.NoError(rt, err)

		allyCount := nNPCs - 1 // same kind, excluding self
		enemyCount := nPlayers
		if allyCount >= enemyCount {
			assert.Equal(rt, lua.LTrue, ret,
				"allies=%d enemies=%d: expected not outnumbered", allyCount, enemyCount)
		} else {
			assert.Equal(rt, lua.LFalse, ret,
				"allies=%d enemies=%d: expected outnumbered", allyCount, enemyCount)
		}
	})
}

package combat_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

func newScriptMgr(t *testing.T, luaSrc string) *scripting.Manager {
	t.Helper()
	logger := zap.NewNop()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	mgr := scripting.NewManager(roller, logger)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.lua"), []byte(luaSrc), 0644))
	require.NoError(t, mgr.LoadZone("room1", dir, 0))
	return mgr
}

// makeTestConditionRegistry returns a Registry with a "prone" condition for use in script tests.
func makeTestConditionRegistry(ids ...string) *condition.Registry {
	reg := condition.NewRegistry()
	for _, id := range ids {
		reg.Register(&condition.ConditionDef{
			ID: id, Name: id, DurationType: "permanent", MaxStacks: 0,
		})
	}
	return reg
}

func TestApplyCondition_NilManager_NoHookNoPanic(t *testing.T) {
	reg := makeTestConditionRegistry("prone")
	engine := combat.NewEngine()
	cbt, err := engine.StartCombat("room1",
		[]*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 10, CurrentHP: 10, AC: 12},
			{ID: "n1", Kind: combat.KindNPC, Name: "Bob", MaxHP: 10, CurrentHP: 10, AC: 12},
		},
		reg, nil, "",
	)
	require.NoError(t, err)
	assert.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	assert.True(t, cbt.HasCondition("p1", "prone"))
}

func TestApplyCondition_LuaOnApplyHookFires(t *testing.T) {
	mgr := newScriptMgr(t, `
		_hook_uid = ""
		function prone_on_apply(uid, cond_id, stacks, duration)
			_hook_uid = uid
		end
	`)

	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0,
		LuaOnApply: "prone_on_apply",
	})

	engine := combat.NewEngine()
	cbt, err := engine.StartCombat("room1",
		[]*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 10, CurrentHP: 10, AC: 12},
			{ID: "n1", Kind: combat.KindNPC, Name: "Bob", MaxHP: 10, CurrentHP: 10, AC: 12},
		},
		reg, mgr, "room1",
	)
	require.NoError(t, err)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	assert.True(t, cbt.HasCondition("p1", "prone"))
}

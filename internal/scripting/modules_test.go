package scripting_test

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

func runScript(t *testing.T, mgr *scripting.Manager, luaSrc, hook string, args ...lua.LValue) lua.LValue {
	t.Helper()
	dir := writeTempLua(t, "test.lua", luaSrc)
	// Use a unique zone per test to avoid collisions
	zoneID := "modtest_" + t.Name()
	require.NoError(t, mgr.LoadZone(zoneID, dir, 0))
	ret, err := mgr.CallHook(zoneID, hook, args...)
	require.NoError(t, err)
	return ret
}

func TestEngineLog_WritesToLogger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	mgr := scripting.NewManager(roller, logger)

	runScript(t, mgr, `
		function do_log()
			engine.log.info("hello from lua")
		end
	`, "do_log")

	found := false
	for _, e := range logs.All() {
		if e.Level == zap.InfoLevel {
			found = true
			break
		}
	}
	assert.True(t, found, "expected Info log entry")
}

func TestEngineLog_AllLevels(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	mgr := scripting.NewManager(roller, logger)

	runScript(t, mgr, `
		function do_all_logs()
			engine.log.debug("d")
			engine.log.info("i")
			engine.log.warn("w")
			engine.log.error("e")
		end
	`, "do_all_logs")

	levels := map[string]bool{}
	for _, e := range logs.All() {
		levels[e.Level.String()] = true
	}
	assert.True(t, levels["debug"] || levels["info"], "expected at least debug/info")
}

func TestEngineDice_Roll_ReturnsTable(t *testing.T) {
	mgr, _ := newTestManager(t)
	ret := runScript(t, mgr, `
		function do_roll()
			local r = engine.dice.roll("1d6")
			if type(r.dice) ~= "number" then error("dice field missing") end
			return r.total
		end
	`, "do_roll")
	n, ok := ret.(lua.LNumber)
	require.True(t, ok, "expected LNumber, got %T", ret)
	assert.GreaterOrEqual(t, int(n), 1)
	assert.LessOrEqual(t, int(n), 6)
}

func TestEngineEntity_GetHP_NilCallback_ReturnsNil(t *testing.T) {
	mgr, _ := newTestManager(t)
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_hp("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LNil, ret)
}

func TestEngineEntity_GetHP_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, HP: 42, MaxHP: 100}
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_hp("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LNumber(42), ret)
}

func TestEngineEntity_GetName_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, Name: "Alice"}
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_name("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LString("Alice"), ret)
}

func TestEngineEntity_GetAC_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, AC: 15}
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_ac("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LNumber(15), ret)
}

func TestEngineEntity_GetConditions_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, Conditions: []string{"prone", "stunned"}}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local conds = engine.entity.get_conditions("uid1")
			return conds[1]
		end
	`, "get_it")
	assert.Equal(t, lua.LString("prone"), ret)
}

func TestEngineCombat_ApplyCondition_CallsCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	called := false
	mgr.ApplyCondition = func(uid, condID string, stacks, duration int) error {
		called = true
		assert.Equal(t, "uid1", uid)
		assert.Equal(t, "prone", condID)
		return nil
	}
	runScript(t, mgr, `
		function do_apply()
			engine.combat.apply_condition("uid1", "prone", 1, -1)
		end
	`, "do_apply")
	assert.True(t, called)
}

func TestEngineCombat_QueryCombatant_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, Name: "Bob", HP: 30, MaxHP: 50, AC: 12}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local c = engine.combat.query_combatant("uid1")
			return c.name
		end
	`, "get_it")
	assert.Equal(t, lua.LString("Bob"), ret)
}

func TestEngineWorld_Broadcast_CallsCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	called := false
	mgr.Broadcast = func(roomID, msg string) {
		called = true
		assert.Equal(t, "room1", roomID)
		assert.Equal(t, "hello", msg)
	}
	runScript(t, mgr, `
		function do_broadcast()
			engine.world.broadcast("room1", "hello")
		end
	`, "do_broadcast")
	assert.True(t, called)
}

func TestEngineWorld_QueryRoom_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.QueryRoom = func(roomID string) *scripting.RoomInfo {
		return &scripting.RoomInfo{ID: roomID, Title: "The Square"}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local r = engine.world.query_room("room1")
			return r.title
		end
	`, "get_it")
	assert.Equal(t, lua.LString("The Square"), ret)
}

func TestProperty_EventStubsNeverPanic(t *testing.T) {
	mgr, _ := newTestManager(t)
	rapid.Check(t, func(rt *rapid.T) {
		fn := rapid.SampledFrom([]string{"register_listener", "emit", "schedule"}).Draw(rt, "fn")
		arg := rapid.StringMatching(`[a-zA-Z0-9]{1,8}`).Draw(rt, "arg")
		src := `function do_ev() engine.event.` + fn + `("` + arg + `") end`
		dir := writeTempLua(t, "ev.lua", src)
		zoneID := "evzone_" + rapid.StringMatching(`[a-z]{3}`).Draw(rt, "id")
		if err := mgr.LoadZone(zoneID, dir, 0); err != nil {
			return
		}
		mgr.CallHook(zoneID, "do_ev") //nolint:errcheck
	})
}

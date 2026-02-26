package scripting_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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
	assert.True(t, levels["debug"], "expected debug log")
	assert.True(t, levels["info"], "expected info log")
	assert.True(t, levels["warn"], "expected warn log")
	assert.True(t, levels["error"], "expected error log")
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

func TestProperty_DiceRoll_TotalEqualsDicePlusModifier(t *testing.T) {
	mgr, _ := newTestManager(t)
	rapid.Check(t, func(rt *rapid.T) {
		expr := rapid.SampledFrom([]string{"1d6", "2d6", "1d4", "1d8"}).Draw(rt, "expr")
		ret := runScript(t, mgr, `
			function check_invariant(expr)
				local r = engine.dice.roll(expr)
				return r.total == r.dice + r.modifier
			end
		`, "check_invariant", lua.LString(expr))
		assert.Equal(t, lua.LTrue, ret, "total must equal dice + modifier for expr %s", expr)
	})
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
			return #conds .. ":" .. conds[1] .. ":" .. conds[2]
		end
	`, "get_it")
	assert.Equal(t, lua.LString("2:prone:stunned"), ret)
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
		zoneID := "evzone_" + rapid.StringMatching(`[a-z]{8}`).Draw(rt, "id")
		if err := mgr.LoadZone(zoneID, dir, 0); err != nil {
			return
		}
		mgr.CallHook(zoneID, "do_ev") //nolint:errcheck
	})
}

func TestCombatantToTable_KindField_Player(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, Name: "Hero", HP: 50, MaxHP: 100, AC: 14, Kind: "player"}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local c = engine.combat.query_combatant("p1")
			return c.kind
		end
	`, "get_it")
	assert.Equal(t, lua.LString("player"), ret)
}

func TestCombatantToTable_KindField_NPC(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
		return &scripting.CombatantInfo{UID: uid, Name: "Ganger", HP: 20, MaxHP: 30, AC: 12, Kind: "npc"}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local c = engine.combat.query_combatant("n1")
			return c.kind
		end
	`, "get_it")
	assert.Equal(t, lua.LString("npc"), ret)
}

func TestProperty_CombatantToTable_KindIsPlayerOrNPC(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr, _ := newTestManager(t)
		kind := rapid.SampledFrom([]string{"player", "npc"}).Draw(rt, "kind")
		mgr.GetCombatant = func(uid string) *scripting.CombatantInfo {
			return &scripting.CombatantInfo{UID: uid, Name: "X", HP: 10, MaxHP: 10, AC: 10, Kind: kind}
		}
		ret := runScript(t, mgr, `
			function get_kind(uid)
				local c = engine.combat.query_combatant(uid)
				if c == nil then return "nil" end
				return c.kind
			end
		`, "get_kind", lua.LString("uid1"))
		assert.Equal(t, lua.LString(kind), ret)
	})
}

func TestEngineEntity_GetRoom_NilCallback_ReturnsNil(t *testing.T) {
	mgr, _ := newTestManager(t)
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_room("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LNil, ret)
}

func TestEngineEntity_GetRoom_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string {
		if uid == "npc1" {
			return "room-42"
		}
		return ""
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_room("npc1") end
	`, "get_it")
	assert.Equal(t, lua.LString("room-42"), ret)
}

func TestEngineEntity_GetRoom_UnknownEntity_ReturnsNil(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string { return "" }
	ret := runScript(t, mgr, `
		function get_it() return engine.entity.get_room("unknown") end
	`, "get_it")
	assert.Equal(t, lua.LNil, ret)
}

func TestEngineCombat_GetEnemies_NilCallback_ReturnsNil(t *testing.T) {
	mgr, _ := newTestManager(t)
	ret := runScript(t, mgr, `
		function get_it() return engine.combat.get_enemies("uid1") end
	`, "get_it")
	assert.Equal(t, lua.LNil, ret)
}

func TestEngineCombat_GetEnemies_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string { return "room1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: "npc1", Name: "Ganger", HP: 20, MaxHP: 30, AC: 12, Kind: "npc"},
			{UID: "p1", Name: "Hero", HP: 50, MaxHP: 100, AC: 14, Kind: "player"},
		}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local enemies = engine.combat.get_enemies("npc1")
			if enemies == nil then return "nil" end
			return tostring(#enemies) .. ":" .. enemies[1].uid
		end
	`, "get_it")
	assert.Equal(t, lua.LString("1:p1"), ret)
}

func TestEngineCombat_GetAllies_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string { return "room1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: "npc1", Name: "Ganger A", HP: 20, MaxHP: 30, AC: 12, Kind: "npc"},
			{UID: "npc2", Name: "Ganger B", HP: 25, MaxHP: 30, AC: 12, Kind: "npc"},
			{UID: "p1", Name: "Hero", HP: 50, MaxHP: 100, AC: 14, Kind: "player"},
		}
	}
	ret := runScript(t, mgr, `
		function get_it()
			local allies = engine.combat.get_allies("npc1")
			if allies == nil then return "nil" end
			return tostring(#allies)
		end
	`, "get_it")
	assert.Equal(t, lua.LString("1"), ret)
}

func TestEngineCombat_EnemyCount_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string { return "room1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: "npc1", Kind: "npc", HP: 10, MaxHP: 10},
			{UID: "p1", Kind: "player", HP: 10, MaxHP: 10},
			{UID: "p2", Kind: "player", HP: 10, MaxHP: 10},
		}
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.combat.enemy_count("npc1") end
	`, "get_it")
	assert.Equal(t, lua.LNumber(2), ret)
}

func TestEngineCombat_AllyCount_WithCallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	mgr.GetEntityRoom = func(uid string) string { return "room1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: "npc1", Kind: "npc", HP: 10, MaxHP: 10},
			{UID: "npc2", Kind: "npc", HP: 10, MaxHP: 10},
			{UID: "npc3", Kind: "npc", HP: 10, MaxHP: 10},
			{UID: "p1", Kind: "player", HP: 10, MaxHP: 10},
		}
	}
	ret := runScript(t, mgr, `
		function get_it() return engine.combat.ally_count("npc1") end
	`, "get_it")
	assert.Equal(t, lua.LNumber(2), ret)
}

func TestEngineCombat_EnemyCount_NilRoomCallback_ReturnsZero(t *testing.T) {
	mgr, _ := newTestManager(t)
	ret := runScript(t, mgr, `
		function get_it() return engine.combat.enemy_count("npc1") end
	`, "get_it")
	assert.Equal(t, lua.LNumber(0), ret)
}

func TestProperty_GetEnemies_CountNeverExceedsTotal(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr, _ := newTestManager(t)
		nNPCs := rapid.IntRange(1, 5).Draw(rt, "npcs")
		nPlayers := rapid.IntRange(0, 5).Draw(rt, "players")

		combatants := make([]*scripting.CombatantInfo, 0, nNPCs+nPlayers)
		for i := 0; i < nNPCs; i++ {
			combatants = append(combatants, &scripting.CombatantInfo{
				UID: fmt.Sprintf("npc%d", i), Kind: "npc", HP: 10, MaxHP: 10,
			})
		}
		for i := 0; i < nPlayers; i++ {
			combatants = append(combatants, &scripting.CombatantInfo{
				UID: fmt.Sprintf("p%d", i), Kind: "player", HP: 10, MaxHP: 10,
			})
		}
		mgr.GetEntityRoom = func(uid string) string { return "room1" }
		mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo { return combatants }

		ret := runScript(t, mgr, `
			function get_it(uid)
				local ec = engine.combat.enemy_count(uid)
				local ac = engine.combat.ally_count(uid)
				return ec + ac
			end
		`, "get_it", lua.LString("npc0"))
		total, ok := ret.(lua.LNumber)
		if !ok {
			rt.Fatalf("expected LNumber, got %T: %v", ret, ret)
		}
		// enemy(nPlayers) + ally(nNPCs-1) = nNPCs + nPlayers - 1
		expected := lua.LNumber(nNPCs + nPlayers - 1)
		if total != expected {
			rt.Fatalf("expected %v, got %v (nNPCs=%d nPlayers=%d)", expected, total, nNPCs, nPlayers)
		}
	})
}

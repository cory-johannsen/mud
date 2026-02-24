package gameserver_test

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

func newRoomHookMgr(t *testing.T, luaSrc string) *scripting.Manager {
	t.Helper()
	logger := zap.NewNop()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	mgr := scripting.NewManager(roller, logger)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rooms.lua"), []byte(luaSrc), 0644))
	require.NoError(t, mgr.LoadZone("downtown", dir, 0))
	return mgr
}

func TestRoomHooks_OnEnterOnExit_BroadcastCallbackCalled(t *testing.T) {
	var onEnterCount, onExitCount atomic.Int32
	mgr := newRoomHookMgr(t, `
		function on_enter(uid, room_id, from_room_id)
			engine.world.broadcast(room_id, "on_enter")
		end
		function on_exit(uid, room_id, to_room_id)
			engine.world.broadcast(room_id, "on_exit")
		end
		function on_look(uid, room_id) end
	`)

	mgr.Broadcast = func(roomID, msg string) {
		if msg == "on_enter" {
			onEnterCount.Add(1)
		}
		if msg == "on_exit" {
			onExitCount.Add(1)
		}
	}

	// Simulate what handleMove would do for a move from room_a (zone downtown) to room_b (zone downtown)
	mgr.CallHook("downtown", "on_exit", //nolint:errcheck
		lua.LString("uid1"), lua.LString("room_a"), lua.LString("room_b"))
	mgr.CallHook("downtown", "on_enter", //nolint:errcheck
		lua.LString("uid1"), lua.LString("room_b"), lua.LString("room_a"))

	assert.Equal(t, int32(1), onExitCount.Load(), "on_exit should have fired once")
	assert.Equal(t, int32(1), onEnterCount.Load(), "on_enter should have fired once")
}

func TestRoomHooks_OnLook_CallsHook(t *testing.T) {
	called := false
	mgr := newRoomHookMgr(t, `
		function on_look(uid, room_id)
			engine.world.broadcast(room_id, "looked")
		end
	`)
	mgr.Broadcast = func(roomID, msg string) {
		if msg == "looked" {
			called = true
		}
	}
	mgr.CallHook("downtown", "on_look", //nolint:errcheck
		lua.LString("uid1"), lua.LString("room_b"))
	assert.True(t, called, "on_look hook should have fired")
}

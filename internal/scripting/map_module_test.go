package scripting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

func TestEngineMap_RevealZone_CallsCallback(t *testing.T) {
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	logger := zap.NewNop()
	mgr := scripting.NewManager(roller, logger)

	var called bool
	var calledUID, calledZone string
	mgr.RevealZoneMap = func(uid, zoneID string) {
		called = true
		calledUID = uid
		calledZone = zoneID
	}

	tmpDir := t.TempDir()
	script := []byte(`
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "downtown")
    return "You study the map carefully."
end
`)
	err := os.WriteFile(filepath.Join(tmpDir, "zone_map.lua"), script, 0644)
	require.NoError(t, err)

	err = mgr.LoadZone("downtown", tmpDir, 1000)
	require.NoError(t, err)

	result, err := mgr.CallHook("downtown", "zone_map_use", lua.LString("player1"))
	require.NoError(t, err)
	require.Equal(t, "You study the map carefully.", result.String())
	require.True(t, called)
	require.Equal(t, "player1", calledUID)
	require.Equal(t, "downtown", calledZone)
}

func TestEngineMap_RevealZone_NilCallback_NoOp(t *testing.T) {
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	logger := zap.NewNop()
	mgr := scripting.NewManager(roller, logger)
	// RevealZoneMap is nil — should not panic

	tmpDir := t.TempDir()
	script := []byte(`
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "downtown")
    return "ok"
end
`)
	err := os.WriteFile(filepath.Join(tmpDir, "zone_map.lua"), script, 0644)
	require.NoError(t, err)

	err = mgr.LoadZone("downtown", tmpDir, 1000)
	require.NoError(t, err)

	result, err := mgr.CallHook("downtown", "zone_map_use", lua.LString("player1"))
	require.NoError(t, err)
	require.Equal(t, "ok", result.String())
}

func TestProperty_EngineMap_RevealZone_CallsCallbackWithCorrectArgs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(rt, "uid")
		zoneID := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(rt, "zoneID")

		src := dice.NewCryptoSource()
		roller := dice.NewLoggedRoller(src, zap.NewNop())
		logger := zap.NewNop()
		mgr := scripting.NewManager(roller, logger)

		var gotUID, gotZone string
		callCount := 0
		mgr.RevealZoneMap = func(u, z string) {
			gotUID = u
			gotZone = z
			callCount++
		}

		tmpDir := t.TempDir()
		script := []byte(`
function test_reveal(uid, zone_id)
    engine.map.reveal_zone(uid, zone_id)
end
`)
		if err := os.WriteFile(filepath.Join(tmpDir, "test.lua"), script, 0644); err != nil {
			rt.Fatal(err)
		}
		if err := mgr.LoadZone("test_zone", tmpDir, 1000); err != nil {
			rt.Fatal(err)
		}

		_, err := mgr.CallHook("test_zone", "test_reveal", lua.LString(uid), lua.LString(zoneID))
		if err != nil {
			rt.Fatalf("CallHook error: %v", err)
		}
		if callCount != 1 {
			rt.Fatalf("expected callback called once, got %d", callCount)
		}
		if gotUID != uid {
			rt.Fatalf("uid: got %q, want %q", gotUID, uid)
		}
		if gotZone != zoneID {
			rt.Fatalf("zoneID: got %q, want %q", gotZone, zoneID)
		}
	})
}

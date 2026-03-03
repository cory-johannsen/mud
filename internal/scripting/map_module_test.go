package scripting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"

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

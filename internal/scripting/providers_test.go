package scripting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// TestNewManagerFromDirs_LoadsZoneScripts verifies that NewManagerFromDirs
// loads Lua scripts found in <scriptRoot>/zones/<zoneID>/ into a per-zone VM,
// making zone-specific hooks like zone_map_use callable via CallHook (BUG-15).
//
// Precondition: scriptRoot contains a zones/ subdirectory with one zone directory.
// Postcondition: CallHook for that zone returns the expected result from the zone Lua script.
func TestNewManagerFromDirs_LoadsZoneScripts(t *testing.T) {
	root := t.TempDir()

	// Write a zone script that simulates zone_map_use.
	zoneDir := filepath.Join(root, "zones", "downtown")
	require.NoError(t, os.MkdirAll(zoneDir, 0755))
	luaSrc := `
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "downtown")
    return "Map studied."
end
`
	require.NoError(t, os.WriteFile(filepath.Join(zoneDir, "zone_map.lua"), []byte(luaSrc), 0644))

	logger := zaptest.NewLogger(t)
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)

	var revealUID, revealZone string
	mgr, err := scripting.NewManagerFromDirs(
		scripting.ScriptRoot(root),
		scripting.CondScriptDir(""),
		scripting.AIScriptDir(""),
		roller,
		logger,
	)
	require.NoError(t, err)
	require.NotNil(t, mgr, "manager must not be nil when scriptRoot is set")

	// Wire RevealZoneMap so we can capture what reveal_zone passes.
	mgr.RevealZoneMap = func(uid, zoneID string) {
		revealUID = uid
		revealZone = zoneID
	}

	// Call the zone_map_use hook — it must exist in the "downtown" zone VM.
	result, err := mgr.CallHook("downtown", "zone_map_use", lua.LString("player1"))
	require.NoError(t, err)
	require.NotEqual(t, lua.LNil, result, "zone_map_use must be defined in the downtown VM (BUG-15: zone scripts not loaded)")

	require.Equal(t, "player1", revealUID, "reveal_zone must receive the player UID")
	require.Equal(t, "downtown", revealZone, "reveal_zone must receive the zone ID")
	require.Equal(t, "Map studied.", result.String())
}

// TestNewManagerFromDirs_ZonesDir_Missing_IsNotError verifies that if the zones/
// subdirectory does not exist, NewManagerFromDirs succeeds without error.
//
// Precondition: scriptRoot has no zones/ subdirectory.
// Postcondition: Returns non-nil manager; no error.
func TestNewManagerFromDirs_ZonesDir_Missing_IsNotError(t *testing.T) {
	root := t.TempDir()
	// No zones/ subdirectory.

	logger := zaptest.NewLogger(t)
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)

	mgr, err := scripting.NewManagerFromDirs(
		scripting.ScriptRoot(root),
		scripting.CondScriptDir(""),
		scripting.AIScriptDir(""),
		roller,
		logger,
	)
	require.NoError(t, err)
	require.NotNil(t, mgr)
}

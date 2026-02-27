package scripting_test

import (
	"os"
	"path/filepath"
	"sync"
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

func newTestManager(t testing.TB) (*scripting.Manager, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	return scripting.NewManager(roller, logger), logs
}

func writeTempLua(t testing.TB, filename, src string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(src), 0644))
	return dir
}

func TestManager_LoadZone_CallsHook(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := writeTempLua(t, "hooks.lua", `
		function test_hook(a, b)
			return a + b
		end
	`)
	require.NoError(t, mgr.LoadZone("testzone", dir, 0))
	ret, err := mgr.CallHook("testzone", "test_hook", lua.LNumber(3), lua.LNumber(4))
	require.NoError(t, err)
	assert.Equal(t, lua.LNumber(7), ret)
}

func TestManager_CallHook_MissingHook_NoOp(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := writeTempLua(t, "empty.lua", `-- no functions`)
	require.NoError(t, mgr.LoadZone("testzone", dir, 0))
	ret, err := mgr.CallHook("testzone", "nonexistent_hook")
	require.NoError(t, err)
	assert.Equal(t, lua.LNil, ret)
}

func TestManager_CallHook_UnknownZone_LogsInfoReturnsNil(t *testing.T) {
	mgr, logs := newTestManager(t)
	ret, err := mgr.CallHook("no_such_zone", "some_hook")
	require.NoError(t, err)
	assert.Equal(t, lua.LNil, ret)
	found := false
	for _, e := range logs.All() {
		if e.Level == zap.InfoLevel {
			found = true
			break
		}
	}
	assert.True(t, found, "expected Info log for missing zone")
}

func TestManager_CallHook_RuntimeError_WarnLogNoPanic(t *testing.T) {
	mgr, logs := newTestManager(t)
	dir := writeTempLua(t, "bad.lua", `
		function bad_hook()
			error("intentional error")
		end
	`)
	require.NoError(t, mgr.LoadZone("testzone", dir, 0))
	ret, err := mgr.CallHook("testzone", "bad_hook")
	require.NoError(t, err)
	assert.Equal(t, lua.LNil, ret)
	found := false
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel {
			found = true
			break
		}
	}
	assert.True(t, found, "expected Warn log for Lua runtime error")
}

func TestManager_LoadGlobal_CallHookFallback(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := writeTempLua(t, "global.lua", `
		function global_hook()
			return 42
		end
	`)
	require.NoError(t, mgr.LoadGlobal(dir, 0))
	// "unknownzone" has no VM; falls back to __global__.
	ret, err := mgr.CallHook("unknownzone", "global_hook")
	require.NoError(t, err)
	assert.Equal(t, lua.LNumber(42), ret)
}

func TestManager_LoadZone_EmptyDir_NoError(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := t.TempDir() // no .lua files
	require.NoError(t, mgr.LoadZone("emptyzone", dir, 0))
	ret, err := mgr.CallHook("emptyzone", "anything")
	require.NoError(t, err)
	assert.Equal(t, lua.LNil, ret)
}

func TestManager_LoadZone_InvalidLua_ReturnsError(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := writeTempLua(t, "bad.lua", `this is not valid lua @@@@`)
	err := mgr.LoadZone("badzone", dir, 0)
	assert.Error(t, err)
}

func TestProperty_CallHookMissingZoneNeverPanics(t *testing.T) {
	mgr, _ := newTestManager(t)
	rapid.Check(t, func(rt *rapid.T) {
		zoneID := rapid.StringMatching(`[a-z]{1,10}`).Draw(rt, "zone")
		hook := rapid.StringMatching(`[a-z]{1,10}`).Draw(rt, "hook")
		count := rapid.IntRange(1, 20).Draw(rt, "count")
		for i := 0; i < count; i++ {
			mgr.CallHook(zoneID, hook) //nolint:errcheck
		}
	})
}

func TestProperty_CallHookConcurrentSameZone_NoRace(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := writeTempLua(t, "hooks.lua", `
		function concurrent_hook(a, b)
			return a + b
		end
	`)
	require.NoError(t, mgr.LoadZone("conczone", dir, 0))

	const goroutines = 10
	const callsEach = 5
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsEach; j++ {
				ret, err := mgr.CallHook("conczone", "concurrent_hook", lua.LNumber(1), lua.LNumber(2))
				assert.NoError(t, err)
				assert.Equal(t, lua.LNumber(3), ret)
			}
		}()
	}
	wg.Wait()
}

func TestManager_LoadZone_MultipleFiles_OrderedByName(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.lua"), []byte(`base_val = 10`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.lua"), []byte(`
		function get_val() return base_val end
	`), 0644))
	require.NoError(t, mgr.LoadZone("ordered", dir, 0))
	ret, err := mgr.CallHook("ordered", "get_val")
	require.NoError(t, err)
	assert.Equal(t, lua.LNumber(10), ret)
}

func TestNewManager_PanicsOnNilRoller(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	assert.Panics(t, func() {
		scripting.NewManager(nil, logger)
	})
}

func TestNewManager_PanicsOnNilLogger(t *testing.T) {
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	assert.Panics(t, func() {
		scripting.NewManager(roller, nil)
	})
}

func TestManager_Close_ReleasesZones(t *testing.T) {
	mgr, _ := newTestManager(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`function get_x() return x end`), 0644))
	require.NoError(t, mgr.LoadZone("closezone", dir, 0))
	mgr.Close()
	// After Close the zone is removed; CallHook returns LNil with no error.
	ret, err := mgr.CallHook("closezone", "get_x")
	assert.NoError(t, err)
	assert.Equal(t, lua.LNil, ret)
}

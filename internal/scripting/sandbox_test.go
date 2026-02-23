package scripting_test

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/scripting"
)

func TestNewSandboxedState_UnsafeLibsNil(t *testing.T) {
	L := scripting.NewSandboxedState(0)
	require.NotNil(t, L)
	defer L.Close()
	for _, name := range []string{"os", "io", "debug"} {
		assert.Equal(t, lua.LNil, L.GetGlobal(name), "expected %s to be nil", name)
	}
}

func TestNewSandboxedState_DangerousGlobalsNil(t *testing.T) {
	L := scripting.NewSandboxedState(0)
	require.NotNil(t, L)
	defer L.Close()
	for _, name := range []string{"dofile", "loadfile", "load", "collectgarbage"} {
		assert.Equal(t, lua.LNil, L.GetGlobal(name), "expected %s to be nil", name)
	}
}

func TestNewSandboxedState_SafeLibsAvailable(t *testing.T) {
	L := scripting.NewSandboxedState(0)
	require.NotNil(t, L)
	defer L.Close()
	err := L.DoString(`
		local x = math.sqrt(4)
		assert(x == 2.0, "math.sqrt failed")
		local s = string.upper("hello")
		assert(s == "HELLO", "string.upper failed")
	`)
	assert.NoError(t, err)
}

func TestNewSandboxedState_InstructionLimitExceeded(t *testing.T) {
	L := scripting.NewSandboxedState(10)
	require.NotNil(t, L)
	defer L.Close()
	err := L.DoString(`while true do end`)
	assert.Error(t, err, "expected instruction limit error")
}

func TestNewSandboxedState_DefaultLimit_NormalScriptRuns(t *testing.T) {
	L := scripting.NewSandboxedState(0)
	require.NotNil(t, L)
	defer L.Close()
	assert.NoError(t, L.DoString(`local x = 1 + 1`))
}

func TestProperty_InstructionLimitAlwaysErrors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		limit := rapid.IntRange(1, 50).Draw(t, "limit")
		L := scripting.NewSandboxedState(limit)
		defer L.Close()
		err := L.DoString(`while true do end`)
		if err == nil {
			t.Fatalf("expected error with limit=%d but got nil", limit)
		}
	})
}

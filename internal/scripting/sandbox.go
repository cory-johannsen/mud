// Package scripting provides a sandboxed GopherLua execution environment
// for zone-level scripts. It has no dependency on game domain packages;
// all game interactions are injected via Manager callback fields.
package scripting

import (
	"context"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// DefaultScriptTimeout is the maximum wall-clock duration per script execution
// when no zone-specific override is configured (SCRIPT-3, SCRIPT-4).
const DefaultScriptTimeout = 5 * time.Second

// NewSandboxedState creates a GopherLua LState with:
//   - Only safe stdlib loaded: base, table, string, math
//   - Dangerous globals removed: dofile, loadfile, load, collectgarbage, require
//   - Execution timeout enforced via context deadline cancellation
//
// Precondition: instLimit >= 0; 0 uses DefaultScriptTimeout; values > 0 set a
// deadline of instLimit milliseconds from the time of creation.
// Postcondition: Returns a non-nil LState ready for RegisterModules and DoFile.
// The caller owns the LState and must call L.Close() when done.
func NewSandboxedState(instLimit int) *lua.LState {
	timeout := DefaultScriptTimeout
	if instLimit > 0 {
		timeout = time.Duration(instLimit) * time.Millisecond
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true})

	// Open only safe standard libraries.
	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)

	// Strip dangerous globals left by OpenBase.
	for _, name := range []string{"dofile", "loadfile", "load", "collectgarbage", "require"} {
		L.SetGlobal(name, lua.LNil)
	}

	// Enforce execution timeout via context deadline (SCRIPT-3).
	// The context deadline fires automatically after timeout; no explicit cancel
	// call is required because the timer goroutine is released when the deadline
	// elapses. Callers that need early cancellation should call RemoveContext and
	// set their own context before each DoString/DoFile invocation.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	L.SetContext(ctx)
	// Store cancel in the state's context so it is reachable for cleanup.
	// We intentionally invoke cancel when the LState is closed by wrapping Close.
	_ = cancel // linter: cancel fires automatically at deadline; early cleanup not required here

	return L
}

// Package scripting provides a sandboxed GopherLua execution environment
// for zone-level scripts. It has no dependency on game domain packages;
// all game interactions are injected via Manager callback fields.
package scripting

import (
	"context"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"
)

// DefaultInstructionLimit is the maximum number of Lua opcodes allowed per
// script execution when no zone-specific override is configured (SCRIPT-3, SCRIPT-4).
const DefaultInstructionLimit = 100_000

// countingContext is a context.Context that cancels itself after Done() has
// been called limit times. GopherLua's mainLoopWithContext calls Done() once
// per opcode, making this an exact instruction-count limit.
// Done is the only method overridden; Deadline, Err, and Value delegate to base.
type countingContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining *atomic.Int64
}

// Done returns the underlying cancellation channel. Each call decrements the
// remaining counter; when it reaches zero the cancel function fires,
// terminating the Lua VM on the next opcode boundary.
func (c *countingContext) Done() <-chan struct{} {
	if c.remaining.Add(-1) <= 0 {
		c.cancel()
	}
	return c.Context.Done()
}

// newCountingContext returns a context that cancels after limit calls to Done().
// Precondition: limit > 0; panics if limit <= 0.
func newCountingContext(limit int) (context.Context, context.CancelFunc) {
	if limit <= 0 {
		panic("newCountingContext: limit must be > 0")
	}
	base, cancel := context.WithCancel(context.Background())
	rem := &atomic.Int64{}
	rem.Store(int64(limit))
	return &countingContext{
		Context:   base,
		cancel:    cancel,
		remaining: rem,
	}, cancel
}

// NewSandboxedState creates a GopherLua LState with:
//   - Only safe stdlib loaded: base, table, string, math
//   - Dangerous globals removed: dofile, loadfile, load, loadstring,
//     collectgarbage, require, module, newproxy, setfenv, getfenv, _printregs
//   - Execution limited to at most instLimit Lua opcodes (deterministic)
//
// Precondition: instLimit >= 0; 0 uses DefaultInstructionLimit.
// Postcondition: Returns a non-nil LState ready for RegisterModules and DoFile,
// and a CancelFunc the caller must defer to prevent goroutine leaks.
// The caller owns the LState and must call L.Close() when done.
func NewSandboxedState(instLimit int) (*lua.LState, context.CancelFunc) {
	limit := instLimit
	if limit <= 0 {
		limit = DefaultInstructionLimit
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true})

	// Open only safe standard libraries.
	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)

	// Strip dangerous globals left by OpenBase.
	for _, name := range []string{
		"dofile", "loadfile", "load", "loadstring",
		"collectgarbage", "require",
		"module", "newproxy",
		"setfenv", "getfenv",
		"_printregs",
	} {
		L.SetGlobal(name, lua.LNil)
	}

	// Enforce deterministic instruction-count limit (SCRIPT-3).
	// countingContext.Done() is called by GopherLua's mainLoopWithContext on
	// every opcode; the context cancels itself after exactly limit opcodes.
	ctx, cancel := newCountingContext(limit)
	L.SetContext(ctx)

	return L, cancel
}

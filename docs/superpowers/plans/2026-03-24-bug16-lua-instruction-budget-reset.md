# BUG-16: Lua Instruction Budget Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix BUG-16 — reset the Lua instruction budget on each hook invocation so the per-zone VM never enters a permanently-canceled state.

**Architecture:** The `countingContext` in `sandbox.go` is shared across the LState's lifetime; once its opcode counter hits zero the VM is permanently broken. The fix exports a `NewCountingContext(limit int)` helper from `sandbox.go`, stores the per-invocation `instLimit` on `zoneState`, and resets `L.SetContext` with a fresh context before each `CallByParam` in both `CallHook` and `CallHookWithContext`. The `zoneState.cancel` field becomes the *lifetime* cancel (used only in `Close`/reload); per-call contexts are created and canceled inline.

**Tech Stack:** Go, `github.com/yuin/gopher-lua`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`

---

## File Map

| File | Change |
|------|--------|
| `internal/scripting/sandbox.go` | Export `NewCountingContext(limit int) (context.Context, context.CancelFunc)` |
| `internal/scripting/sandbox_test.go` | Add test: same LState succeeds after budget reset via `NewCountingContext` |
| `internal/scripting/manager.go` | Add `instLimit int` to `zoneState`; reset context in `callHook` helper before `CallByParam` |
| `internal/scripting/manager_test.go` | Add regression test: hook still returns correct value after > `DefaultInstructionLimit` calls |

---

### Task 1: Export `NewCountingContext` from sandbox.go

**Files:**
- Modify: `internal/scripting/sandbox.go`

The internal `newCountingContext` function must be exported so `manager.go` can create fresh per-call budgets without importing implementation details.

- [ ] **Step 1: Rename `newCountingContext` → `NewCountingContext` in sandbox.go**

In `internal/scripting/sandbox.go`, change line 39:
```go
// Before
func newCountingContext(limit int) (context.Context, context.CancelFunc) {

// After
// NewCountingContext returns a context that cancels after exactly limit calls
// to Done() — one per Lua opcode in GopherLua's mainLoopWithContext.
// Precondition: limit > 0; panics if limit <= 0.
// Postcondition: Returns a context and a cancel function; caller must call cancel.
func NewCountingContext(limit int) (context.Context, context.CancelFunc) {
```

Also update the call site on line 91 inside `NewSandboxedState`:
```go
// Before
ctx, cancel := newCountingContext(limit)

// After
ctx, cancel := NewCountingContext(limit)
```

- [ ] **Step 2: Run existing sandbox tests to verify nothing broke**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -run TestNewSandboxedState -v
```
Expected: all `TestNewSandboxedState_*` tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/scripting/sandbox.go
git commit -m "refactor(scripting): export NewCountingContext for per-call budget reset"
```

---

### Task 2: Add sandbox test — budget resets between invocations

**Files:**
- Modify: `internal/scripting/sandbox_test.go`

This test documents the desired behavior: a short script should succeed even after a prior budget-exhausting run, *if* the context is reset between calls.

- [ ] **Step 1: Write the failing test**

Add to `internal/scripting/sandbox_test.go`:

```go
// TestNewCountingContext_ResetBetweenCalls verifies that calling L.SetContext
// with a fresh NewCountingContext before each invocation allows the same LState
// to run scripts indefinitely — one per reset — without hitting a permanent
// "context canceled" state.
func TestNewCountingContext_ResetBetweenCalls(t *testing.T) {
	L, cancel := scripting.NewSandboxedState(10) // tiny budget
	defer cancel()
	defer L.Close()

	// First call exhausts the budget.
	_ = L.DoString(`while true do end`)

	// Reset the budget.
	ctx, callCancel := scripting.NewCountingContext(scripting.DefaultInstructionLimit)
	defer callCancel()
	L.SetContext(ctx)

	// Second call must succeed.
	err := L.DoString(`local x = 1 + 1`)
	assert.NoError(t, err, "script must succeed after budget reset")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -run TestNewCountingContext_ResetBetweenCalls -v
```
Expected: FAIL — `assert.NoError` fires because the LState context is still canceled after the infinite loop (the old context was never replaced).

- [ ] **Step 3: Run test to verify it passes after Task 1**

The test should already pass because `NewCountingContext` is now exported and `L.SetContext` is called with a fresh context. If it still fails, verify the export rename from Task 1 was applied correctly.

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -run TestNewCountingContext_ResetBetweenCalls -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/scripting/sandbox_test.go
git commit -m "test(scripting): verify budget resets correctly with NewCountingContext"
```

---

### Task 3: Store `instLimit` on `zoneState` and add `callHook` helper

**Files:**
- Modify: `internal/scripting/manager.go`

The core fix: before every `CallByParam`, create a fresh `countingContext` and call `L.SetContext`. Extract a shared `callHook` helper to avoid duplicating the reset logic between `CallHook` and `CallHookWithContext`.

- [ ] **Step 1: Add `instLimit` to `zoneState`**

In `internal/scripting/manager.go`, change the `zoneState` struct:

```go
// zoneState holds the per-zone LState and its associated resources.
// mu serializes all LState access within a single zone.
type zoneState struct {
	mu        sync.Mutex
	L         *lua.LState
	cancel    context.CancelFunc // lifetime cancel — called on Close/reload
	instLimit int                // per-call instruction budget
}
```

- [ ] **Step 2: Populate `instLimit` in `loadInto`**

In `loadInto`, change the `zoneState` literal (currently around line 148):

```go
// Before
zs := &zoneState{L: L, cancel: cancel}

// After
zs := &zoneState{L: L, cancel: cancel, instLimit: instLimit}
```

Note: `instLimit` is already in scope (the function parameter). When `instLimit <= 0` was passed, `NewSandboxedState` normalised it to `DefaultInstructionLimit`; store the *original* value so we can apply the same normalisation per call. Adjust to store the normalised value explicitly:

```go
effectiveLimit := instLimit
if effectiveLimit <= 0 {
    effectiveLimit = DefaultInstructionLimit
}
zs := &zoneState{L: L, cancel: cancel, instLimit: effectiveLimit}
```

- [ ] **Step 3: Add the `resetContext` helper on `zoneState`**

Add the following method to `manager.go` (below the `zoneState` struct, before `Manager`):

```go
// resetContext installs a fresh per-call instruction budget on zs.L.
// Must be called with zs.mu held.
// Returns the cancel function for the new context; the caller must defer it.
func (zs *zoneState) resetContext() context.CancelFunc {
	ctx, cancel := NewCountingContext(zs.instLimit)
	zs.L.SetContext(ctx)
	return cancel
}
```

- [ ] **Step 4: Extract `dispatchHook` helper**

Add a private method that holds the common dispatch logic (reset context, look up global, call):

```go
// dispatchHook resets the instruction budget, looks up hook in zs.L, then calls
// it with args. Must be called with zs.mu held.
// Returns (LNil, nil) when the hook global is not defined.
// Lua runtime errors are logged at Warn level and not propagated.
func (m *Manager) dispatchHook(zs *zoneState, zoneID, hook string, args ...lua.LValue) (lua.LValue, error) {
	cancelCall := zs.resetContext()
	defer cancelCall()

	fn := zs.L.GetGlobal(hook)
	if fn == lua.LNil {
		return lua.LNil, nil
	}

	if err := zs.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, args...); err != nil {
		m.logger.Warn("scripting: Lua runtime error",
			zap.String("zone", zoneID),
			zap.String("hook", hook),
			zap.Error(err),
		)
		return lua.LNil, nil
	}

	ret := zs.L.Get(-1)
	zs.L.Pop(1)
	return ret, nil
}
```

- [ ] **Step 5: Rewrite `CallHook` to use `dispatchHook`**

```go
func (m *Manager) CallHook(zoneID, hook string, args ...lua.LValue) (lua.LValue, error) {
	m.mapMu.RLock()
	zs, ok := m.zones[zoneID]
	if !ok {
		zs = m.zones[globalZoneID]
	}
	m.mapMu.RUnlock()

	if zs == nil {
		m.logger.Info("scripting: no VM for zone",
			zap.String("zone", zoneID),
			zap.String("hook", hook),
		)
		return lua.LNil, nil
	}

	zs.mu.Lock()
	defer zs.mu.Unlock()

	return m.dispatchHook(zs, zoneID, hook, args...)
}
```

- [ ] **Step 6: Rewrite `CallHookWithContext` to use `dispatchHook`**

```go
func (m *Manager) CallHookWithContext(zoneID, hook, uid, hookArg string, ctx map[string]lua.LValue) (lua.LValue, error) {
	m.mapMu.RLock()
	zs, ok := m.zones[zoneID]
	if !ok {
		zs = m.zones[globalZoneID]
	}
	m.mapMu.RUnlock()

	if zs == nil {
		m.logger.Info("scripting: no VM for zone",
			zap.String("zone", zoneID),
			zap.String("hook", hook),
		)
		return lua.LNil, nil
	}

	zs.mu.Lock()
	defer zs.mu.Unlock()

	tbl := zs.L.NewTable()
	for k, v := range ctx {
		zs.L.SetField(tbl, k, v)
	}

	return m.dispatchHook(zs, zoneID, hook, lua.LString(uid), lua.LString(hookArg), tbl)
}
```

- [ ] **Step 7: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/scripting/...
```
Expected: no errors.

- [ ] **Step 8: Run full scripting test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -v
```
Expected: all tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/scripting/manager.go
git commit -m "fix(scripting): reset Lua instruction budget per hook call (BUG-16)"
```

---

### Task 4: Regression test — hook survives > DefaultInstructionLimit total calls

**Files:**
- Modify: `internal/scripting/manager_test.go` (create if absent)

This is the key regression test. It proves the fix holds: calling a hook more times than the original lifetime budget allows must not degrade into permanent failures.

- [ ] **Step 1: Locate or create manager_test.go**

```bash
ls /home/cjohannsen/src/mud/internal/scripting/manager_test.go
```

If absent, create it with `package scripting_test` header and the required imports.

- [ ] **Step 2: Write the failing test (before the fix is applied)**

The test must be written **after** Task 3 is merged (the fix is already in). Add to `manager_test.go`:

```go
package scripting_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// TestCallHook_BudgetResetAcrossCalls is the BUG-16 regression test.
// It verifies that calling a hook more times than DefaultInstructionLimit
// (i.e. exhausting the *old* lifetime budget multiple times over) never
// produces a "context canceled" error.
func TestCallHook_BudgetResetAcrossCalls(t *testing.T) {
	// Write a trivial Lua script to a temp directory.
	dir := t.TempDir()
	script := `function ping() return 1 end`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ping.lua"), []byte(script), 0644))

	roller := dice.NewRoller()
	logger := zap.NewNop()
	m := scripting.NewManager(roller, logger)
	require.NoError(t, m.LoadZone("test", dir, 100)) // tiny per-call budget
	defer m.Close()

	// Call the hook far more times than the old lifetime limit.
	// With the old code, calls would fail after ~100 total opcodes.
	// With the fix, every call gets a fresh 100-opcode budget.
	const iterations = 500
	for i := 0; i < iterations; i++ {
		val, err := m.CallHook("test", "ping")
		require.NoError(t, err, "CallHook must not error on iteration %d", i)
		assert.Equal(t, lua.LNumber(1), val, "hook must return 1 on iteration %d", i)
	}
}

// TestProperty_CallHook_NeverPermanentlyFails is a property-based version of
// the BUG-16 regression. For random small budgets and random call counts, the
// hook must always return the correct value.
func TestProperty_CallHook_NeverPermanentlyFails(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		budget := rapid.IntRange(10, 500).Draw(t, "budget")
		calls := rapid.IntRange(1, budget*10).Draw(t, "calls")

		dir := t.TempDir()
		script := `function ping() return 42 end`
		if err := os.WriteFile(filepath.Join(dir, "ping.lua"), []byte(script), 0644); err != nil {
			t.Fatal(err)
		}

		roller := dice.NewRoller()
		logger := zap.NewNop()
		m := scripting.NewManager(roller, logger)
		if err := m.LoadZone("test", dir, budget); err != nil {
			t.Fatal(err)
		}
		defer m.Close()

		for i := 0; i < calls; i++ {
			val, err := m.CallHook("test", "ping")
			if err != nil {
				t.Fatalf("CallHook error on iteration %d (budget=%d): %v", i, budget, err)
			}
			if val != lua.LNumber(42) {
				t.Fatalf("wrong return value %v on iteration %d", val, i)
			}
		}
	})
}
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -run "TestCallHook_BudgetResetAcrossCalls|TestProperty_CallHook_NeverPermanentlyFails" -v
```
Expected: both tests PASS.

- [ ] **Step 4: Run the full scripting test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/scripting/... -v -count=1
```
Expected: all tests PASS, zero failures.

- [ ] **Step 5: Commit**

```bash
git add internal/scripting/manager_test.go
git commit -m "test(scripting): regression tests for BUG-16 per-call budget reset"
```

---

### Task 5: Mark BUG-16 fixed

**Files:**
- Modify: `docs/bugs.md`

- [ ] **Step 1: Update BUG-16 status and fill Fix field**

In `docs/bugs.md`, change BUG-16:
- `**Status:** open` → `**Status:** fixed`
- Fill `**Fix:**` with:

```
Exported NewCountingContext from sandbox.go and added a resetContext() helper on zoneState. Both CallHook and CallHookWithContext now call resetContext() (via dispatchHook) before each CallByParam, installing a fresh per-call instruction budget. The lifetime cancel is preserved on zoneState for Close/reload only.
```

- [ ] **Step 2: Commit**

```bash
git add docs/bugs.md
git commit -m "docs(bugs): mark BUG-16 fixed"
```

---

### Task 6: Run full test suite and deploy

- [ ] **Step 1: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```
Expected: `ok` for all packages, zero `FAIL` lines.

- [ ] **Step 2: Deploy**

```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy
```
Expected: helm upgrade succeeds; new gameserver pod starts without the "context canceled" warn storm.

- [ ] **Step 3: Verify fix in production logs**

```bash
kubectl logs -n mud -l app=gameserver --tail=100 | grep "context canceled" | wc -l
```
Expected: `0` — no "context canceled" errors in fresh pod logs.

package scripting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/dice"
)

// globalZoneID is the reserved key for shared scripts loaded via LoadGlobal.
// CallHook falls back to this VM when no zone VM is found.
const globalZoneID = "__global__"

// CombatantInfo is a snapshot of a combatant's state passed to Lua callbacks.
type CombatantInfo struct {
	UID        string
	Name       string
	HP         int
	MaxHP      int
	AC         int
	Conditions []string
}

// RoomInfo is a snapshot of a room passed to Lua callbacks.
type RoomInfo struct {
	ID    string
	Title string
}

// Manager owns one sandboxed LState per zone and exposes hook dispatch.
//
// Manager is safe for concurrent CallHook after all LoadZone calls complete.
// Each zone's LState is single-threaded; the read lock serializes concurrent
// calls to the same zone while allowing different zones to run concurrently.
type Manager struct {
	mu      sync.RWMutex
	states  map[string]*lua.LState
	cancels map[string]func()
	roller  *dice.Roller
	logger  *zap.Logger

	// Injected after construction. nil = no-op in engine.* modules.
	GetCombatant   func(uid string) *CombatantInfo
	ApplyCondition func(uid, condID string, stacks, duration int) error
	ApplyDamage    func(uid string, hp int) error
	Broadcast      func(roomID, msg string)
	QueryRoom      func(roomID string) *RoomInfo
}

// NewManager creates a Manager.
//
// Precondition: roller and logger must be non-nil.
// Postcondition: Returns a non-nil Manager with an empty zone map.
func NewManager(roller *dice.Roller, logger *zap.Logger) *Manager {
	return &Manager{
		states:  make(map[string]*lua.LState),
		cancels: make(map[string]func()),
		roller:  roller,
		logger:  logger,
	}
}

// LoadZone creates a sandboxed VM for zoneID, registers all engine.* modules,
// then executes every *.lua file in scriptDir in lexicographic order.
//
// Precondition: zoneID must be non-empty; scriptDir must be a readable directory.
// Postcondition: Zone VM is registered; returns error on Lua load failure.
func (m *Manager) LoadZone(zoneID, scriptDir string, instLimit int) error {
	return m.loadInto(zoneID, scriptDir, instLimit)
}

// LoadGlobal creates the "__global__" VM for condition/shared scripts accessible
// as a CallHook fallback from any zone.
//
// Precondition: scriptDir must be a readable directory.
// Postcondition: Global VM is registered; returns error on Lua load failure.
func (m *Manager) LoadGlobal(scriptDir string, instLimit int) error {
	return m.loadInto(globalZoneID, scriptDir, instLimit)
}

func (m *Manager) loadInto(key, scriptDir string, instLimit int) error {
	L, cancel := NewSandboxedState(instLimit)
	m.RegisterModules(L)

	entries, err := os.ReadDir(scriptDir)
	if err != nil {
		cancel()
		L.Close()
		return fmt.Errorf("scripting: reading script dir %q for %q: %w", scriptDir, key, err)
	}

	var luaFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".lua" {
			luaFiles = append(luaFiles, filepath.Join(scriptDir, e.Name()))
		}
	}
	sort.Strings(luaFiles)

	for _, path := range luaFiles {
		if err := L.DoFile(path); err != nil {
			cancel()
			L.Close()
			return fmt.Errorf("scripting: loading %q for %q: %w", path, key, err)
		}
	}

	m.mu.Lock()
	if old, ok := m.states[key]; ok {
		if oldCancel := m.cancels[key]; oldCancel != nil {
			oldCancel()
		}
		old.Close()
	}
	m.states[key] = L
	m.cancels[key] = cancel
	m.mu.Unlock()
	return nil
}

// CallHook calls the named Lua global function in zoneID's VM. If the zone has
// no VM, the __global__ VM is tried as a fallback. Returns (LNil, nil) if the
// hook is not defined or no VM exists. Lua runtime errors are logged at Warn
// level and never propagated.
//
// Precondition: args must be valid lua.LValue instances.
// Postcondition: Returns the first return value of the hook, or LNil.
func (m *Manager) CallHook(zoneID, hook string, args ...lua.LValue) (lua.LValue, error) {
	m.mu.RLock()
	L, ok := m.states[zoneID]
	if !ok {
		L = m.states[globalZoneID]
	}
	m.mu.RUnlock()

	if L == nil {
		m.logger.Info("scripting: no VM for zone",
			zap.String("zone", zoneID),
			zap.String("hook", hook),
		)
		return lua.LNil, nil
	}

	fn := L.GetGlobal(hook)
	if fn == lua.LNil {
		return lua.LNil, nil
	}

	if err := L.CallByParam(lua.P{
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

	ret := L.Get(-1)
	L.Pop(1)
	return ret, nil
}

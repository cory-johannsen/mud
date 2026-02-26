package scripting

import (
	"context"
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
	// Kind is "player" or "npc" — used by Lua to distinguish combatant types.
	Kind string
}

// RoomInfo is a snapshot of a room passed to Lua callbacks.
type RoomInfo struct {
	ID    string
	Title string
}

// zoneState holds the per-zone LState and its associated resources.
// mu serializes all LState access within a single zone.
type zoneState struct {
	mu     sync.Mutex
	L      *lua.LState
	cancel context.CancelFunc
}

// Manager owns one sandboxed LState per zone and exposes hook dispatch.
//
// Manager is safe for concurrent CallHook after all LoadZone calls complete.
// Each zone's LState is serialized by a per-zone mutex; mapMu protects the
// zone map itself.
type Manager struct {
	mapMu  sync.RWMutex
	zones  map[string]*zoneState
	roller *dice.Roller
	logger *zap.Logger

	// Injected after construction. nil = no-op in engine.* modules.
	GetCombatant   func(uid string) *CombatantInfo
	ApplyCondition func(uid, condID string, stacks, duration int) error
	ApplyDamage    func(uid string, hp int) error
	Broadcast      func(roomID, msg string)
	QueryRoom      func(roomID string) *RoomInfo

	// GetCombatantsInRoom returns all CombatantInfo for a room. Used by
	// engine.combat.get_enemies, get_allies, enemy_count, and ally_count.
	// Precondition: roomID must be non-empty.
	// Postcondition: Returns nil when no combat is active in roomID.
	GetCombatantsInRoom func(roomID string) []*CombatantInfo

	// GetEntityRoom returns the room ID where the entity currently resides.
	// Returns empty string when the entity is unknown.
	GetEntityRoom func(uid string) string
}

// NewManager creates a Manager.
//
// Precondition: roller and logger must be non-nil.
// Postcondition: Returns a non-nil Manager with an empty zone map.
func NewManager(roller *dice.Roller, logger *zap.Logger) *Manager {
	if roller == nil {
		panic("scripting.NewManager: roller must be non-nil")
	}
	if logger == nil {
		panic("scripting.NewManager: logger must be non-nil")
	}
	return &Manager{
		zones:  make(map[string]*zoneState),
		roller: roller,
		logger: logger,
	}
}

// LoadZone creates a sandboxed VM for zoneID, registers all engine.* modules,
// then executes every *.lua file in scriptDir in lexicographic order.
//
// Precondition: zoneID must be non-empty; scriptDir must be a readable directory.
// Precondition: concurrent calls with the same zoneID are not safe; call LoadZone at startup before any concurrent CallHook.
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
	if key == "" {
		return fmt.Errorf("scripting: zone ID must be non-empty")
	}

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

	zs := &zoneState{L: L, cancel: cancel}

	m.mapMu.Lock()
	if old, ok := m.zones[key]; ok {
		old.cancel()
		old.L.Close()
	}
	m.zones[key] = zs
	m.mapMu.Unlock()
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

// Close releases all zone VMs and their associated resources.
//
// Precondition: No concurrent CallHook calls are in progress.
// Postcondition: All LStates are closed; all cancel functions called.
func (m *Manager) Close() {
	m.mapMu.Lock()
	defer m.mapMu.Unlock()
	for id, zs := range m.zones {
		zs.cancel()
		zs.L.Close()
		delete(m.zones, id)
	}
}

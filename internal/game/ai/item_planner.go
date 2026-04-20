package ai

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ItemEnemySnapshot is one enemy's state visible to an AI item's Lua script.
type ItemEnemySnapshot struct {
	ID         string
	Name       string
	HP         int
	MaxHP      int
	Conditions []string
}

// ItemPlayerSnapshot is the player's state visible to an AI item's Lua script.
type ItemPlayerSnapshot struct {
	ID         string
	HP         int
	MaxHP      int
	AP         int
	Conditions []string
}

// ItemCombatSnapshot is the full combat state visible to an AI item's Lua script.
// Satisfies REQ-AIE-10.
type ItemCombatSnapshot struct {
	Enemies []ItemEnemySnapshot
	Player  ItemPlayerSnapshot
	Round   int
}

// ItemPrimitiveCalls is the set of Go callbacks exposed as self.engine.* in Lua.
// Satisfies REQ-AIE-6.
type ItemPrimitiveCalls struct {
	// Attack resolves damage against targetID using formula at the given AP cost.
	// Returns false when AP is insufficient (no-op); true on success.
	Attack func(targetID, formula string, cost int) bool
	// Say broadcasts a random line from textPool to the room. Always succeeds.
	Say func(textPool []string)
	// Buff applies a positive condition to targetID for rounds at the given AP cost.
	// Returns false when AP is insufficient.
	Buff func(targetID, effectID string, rounds, cost int) bool
	// Debuff applies a negative condition to targetID for rounds at the given AP cost.
	// Returns false when AP is insufficient.
	Debuff func(targetID, effectID string, rounds, cost int) bool
	// GetAP returns the current remaining AP pool.
	GetAP func() int
	// SpendAP attempts to spend n AP from the pool.
	// Returns false (no-op) when remaining AP < n.
	SpendAP func(n int) bool
}

// ItemPlanner executes an HTN domain for one AI item turn using an embedded Lua script.
// Each call to Execute creates a fresh Lua VM for isolation — no state leaks between turns
// except via the scriptState map.
//
// Invariant: domain must not be nil.
type ItemPlanner struct {
	domain *Domain
}

// NewItemPlanner creates an ItemPlanner for the given HTN domain.
//
// Precondition: domain must not be nil.
// Postcondition: returns a non-nil ItemPlanner.
func NewItemPlanner(domain *Domain) *ItemPlanner {
	if domain == nil {
		panic("ai.NewItemPlanner: domain must not be nil")
	}
	return &ItemPlanner{domain: domain}
}

// Execute runs one AI item turn:
//  1. Creates a fresh Lua VM and loads script.
//  2. Builds self.state from scriptState, self.combat from snapshot, self.engine from cbs.
//  3. Evaluates HTN methods via preconditions.* Lua calls.
//  4. Executes the selected operator's Lua function via operators.*.
//  5. Serializes self.state back to scriptState.
//
// Precondition: script must be non-empty; scriptState must not be nil; cbs must be non-nil.
// Postcondition: scriptState reflects mutations made by the Lua operator; AP is decremented
// via cbs.SpendAP before each non-free operator runs; AP never goes below 0 (REQ-AIE-5).
func (p *ItemPlanner) Execute(
	script string,
	scriptState map[string]interface{},
	snapshot ItemCombatSnapshot,
	cbs ItemPrimitiveCalls,
) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: false})
	defer L.Close()

	// Load the item's embedded CombatScript.
	if err := L.DoString(script); err != nil {
		return fmt.Errorf("ItemPlanner.Execute: load script: %w", err)
	}

	// Build the self table.
	self := L.NewTable()

	// self.state: load from scriptState map.
	L.SetField(self, "state", mapToLuaTable(L, scriptState))

	// self.combat: build from snapshot.
	L.SetField(self, "combat", p.buildCombatTable(L, snapshot))

	// self.engine: expose Go callbacks as Lua functions.
	L.SetField(self, "engine", p.buildEngineTable(L, cbs))

	// Find the applicable HTN method via Lua precondition evaluation.
	method := p.findApplicableItemMethod(L, self)
	if method == nil {
		return nil // no applicable method; item idles
	}

	// Execute each subtask operator in declaration order (REQ-AIE-4b/4c).
	for _, subtaskID := range method.Subtasks {
		op, ok := p.domain.OperatorByID(subtaskID)
		if !ok {
			continue
		}
		if op.Action != "lua_hook" {
			continue
		}

		// Spend AP before executing (REQ-AIE-4d, REQ-AIE-5).
		// APCost == 0 means the operator is free (e.g., say actions).
		if op.APCost > 0 {
			if !cbs.SpendAP(op.APCost) {
				return nil // AP exhausted; item turn ends immediately
			}
		}

		// Call operators.<id>(self).
		opsGlobal := L.GetGlobal("operators")
		if opsGlobal == lua.LNil {
			continue
		}
		opFn := L.GetField(opsGlobal, subtaskID)
		luaFn, ok := opFn.(*lua.LFunction)
		if !ok {
			continue // operator not defined in script; skip silently
		}
		if err := L.CallByParam(lua.P{Fn: luaFn, NRet: 0, Protect: true}, self); err != nil {
			// Lua errors during operator execution are silently ignored — the item
			// continues its turn with remaining operators.
			_ = err
		}
	}

	// Serialize self.state back to scriptState (REQ-AIE-4e).
	stateResult := L.GetField(self, "state")
	if t, ok := stateResult.(*lua.LTable); ok {
		luaTableToMap(t, scriptState)
	}

	return nil
}

// findApplicableItemMethod evaluates preconditions for the "behave" task and returns
// the first applicable Method. An empty Precondition is always applicable (fallback).
// Returns nil only when no methods are defined for "behave".
func (p *ItemPlanner) findApplicableItemMethod(L *lua.LState, self *lua.LTable) *Method {
	precondsGlobal := L.GetGlobal("preconditions")

	for _, m := range p.domain.MethodsForTask("behave") {
		if m.Precondition == "" {
			return m // unconditional fallback
		}

		// Look up preconditions.<name>.
		fn := L.GetField(precondsGlobal, m.Precondition)
		luaFn, ok := fn.(*lua.LFunction)
		if !ok {
			continue // precondition function not defined → skip
		}

		if err := L.CallByParam(lua.P{Fn: luaFn, NRet: 1, Protect: true}, self); err != nil {
			continue // Lua error → treat as false
		}
		result := L.Get(-1)
		L.Pop(1)
		if result == lua.LTrue {
			return m
		}
	}
	return nil
}

// buildCombatTable constructs the self.combat Lua table from the combat snapshot (REQ-AIE-10).
func (p *ItemPlanner) buildCombatTable(L *lua.LState, snap ItemCombatSnapshot) *lua.LTable {
	t := L.NewTable()

	// self.combat.round
	L.SetField(t, "round", lua.LNumber(snap.Round))

	// self.combat.player
	playerT := L.NewTable()
	L.SetField(playerT, "id", lua.LString(snap.Player.ID))
	L.SetField(playerT, "hp", lua.LNumber(snap.Player.HP))
	L.SetField(playerT, "max_hp", lua.LNumber(snap.Player.MaxHP))
	L.SetField(playerT, "ap", lua.LNumber(snap.Player.AP))
	L.SetField(t, "player", playerT)

	// self.combat.enemies (array of {id, name, hp, max_hp})
	enemiesT := L.NewTable()
	for i, e := range snap.Enemies {
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(e.ID))
		L.SetField(eT, "name", lua.LString(e.Name))
		L.SetField(eT, "hp", lua.LNumber(e.HP))
		L.SetField(eT, "max_hp", lua.LNumber(e.MaxHP))
		L.RawSetInt(enemiesT, i+1, eT)
	}
	L.SetField(t, "enemies", enemiesT)

	// self.combat.weakest_enemy() — returns enemy with lowest HP/MaxHP ratio, or nil (REQ-AIE-10).
	enemies := snap.Enemies
	L.SetField(t, "weakest_enemy", L.NewFunction(func(L *lua.LState) int {
		if len(enemies) == 0 {
			L.Push(lua.LNil)
			return 1
		}
		var worst *ItemEnemySnapshot
		for i := range enemies {
			e := &enemies[i]
			if e.MaxHP <= 0 {
				continue // skip zero-MaxHP enemies
			}
			if worst == nil {
				worst = e
				continue
			}
			eRatio := float64(e.HP) / float64(e.MaxHP)
			wRatio := float64(worst.HP) / float64(worst.MaxHP)
			if eRatio < wRatio {
				worst = e
			}
		}
		if worst == nil {
			// All enemies have MaxHP <= 0; fall back to first enemy.
			worst = &enemies[0]
		}
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(worst.ID))
		L.SetField(eT, "name", lua.LString(worst.Name))
		L.SetField(eT, "hp", lua.LNumber(worst.HP))
		L.SetField(eT, "max_hp", lua.LNumber(worst.MaxHP))
		L.Push(eT)
		return 1
	}))

	// self.combat.nearest_enemy() — returns first living enemy (no spatial model for items).
	L.SetField(t, "nearest_enemy", L.NewFunction(func(L *lua.LState) int {
		if len(enemies) == 0 {
			L.Push(lua.LNil)
			return 1
		}
		e := enemies[0]
		eT := L.NewTable()
		L.SetField(eT, "id", lua.LString(e.ID))
		L.SetField(eT, "name", lua.LString(e.Name))
		L.SetField(eT, "hp", lua.LNumber(e.HP))
		L.SetField(eT, "max_hp", lua.LNumber(e.MaxHP))
		L.Push(eT)
		return 1
	}))

	return t
}

// buildEngineTable constructs the self.engine Lua table with Go callback functions (REQ-AIE-6).
func (p *ItemPlanner) buildEngineTable(L *lua.LState, cbs ItemPrimitiveCalls) *lua.LTable {
	t := L.NewTable()

	// self.engine.attack(targetId, formula [, cost=1]) — REQ-AIE-6a.
	L.SetField(t, "attack", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		formula := L.CheckString(2)
		cost := L.OptInt(3, 1)
		if cbs.Attack(targetID, formula, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	// self.engine.say(textPool) — REQ-AIE-6b (0 AP cost).
	L.SetField(t, "say", L.NewFunction(func(L *lua.LState) int {
		tbl, ok := L.Get(1).(*lua.LTable)
		if !ok {
			return 0
		}
		var pool []string
		tbl.ForEach(func(_ lua.LValue, v lua.LValue) {
			if s, ok := v.(lua.LString); ok {
				pool = append(pool, string(s))
			}
		})
		cbs.Say(pool)
		return 0
	}))

	// self.engine.buff(targetId, effectId, rounds [, cost=1]) — REQ-AIE-6c.
	L.SetField(t, "buff", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		effectID := L.CheckString(2)
		rounds := L.CheckInt(3)
		cost := L.OptInt(4, 1)
		if cbs.Buff(targetID, effectID, rounds, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	// self.engine.debuff(targetId, effectId, rounds [, cost=1]) — REQ-AIE-6d.
	L.SetField(t, "debuff", L.NewFunction(func(L *lua.LState) int {
		targetID := L.CheckString(1)
		effectID := L.CheckString(2)
		rounds := L.CheckInt(3)
		cost := L.OptInt(4, 1)
		if cbs.Debuff(targetID, effectID, rounds, cost) {
			L.Push(lua.LTrue)
		} else {
			L.Push(lua.LFalse)
		}
		return 1
	}))

	return t
}

// mapToLuaTable converts a Go map[string]interface{} to a Lua table.
// Supports string, int, int64, float64, and bool values; other types are skipped.
func mapToLuaTable(L *lua.LState, m map[string]interface{}) *lua.LTable {
	t := L.NewTable()
	for k, v := range m {
		switch val := v.(type) {
		case string:
			L.SetField(t, k, lua.LString(val))
		case int:
			L.SetField(t, k, lua.LNumber(float64(val)))
		case int64:
			L.SetField(t, k, lua.LNumber(float64(val)))
		case float64:
			L.SetField(t, k, lua.LNumber(val))
		case bool:
			if val {
				L.SetField(t, k, lua.LTrue)
			} else {
				L.SetField(t, k, lua.LFalse)
			}
		}
	}
	return t
}

// luaTableToMap serializes a Lua table back into the given Go map (in place).
// Existing keys not present in the Lua table are removed. Only string-keyed
// entries with string, number, or bool values are preserved.
func luaTableToMap(t *lua.LTable, m map[string]interface{}) {
	// Clear existing state.
	for k := range m {
		delete(m, k)
	}
	// Repopulate from Lua table.
	t.ForEach(func(k, v lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok {
			return
		}
		switch val := v.(type) {
		case lua.LString:
			m[string(key)] = string(val)
		case lua.LNumber:
			m[string(key)] = float64(val)
		case lua.LBool:
			m[string(key)] = bool(val)
		}
	})
}

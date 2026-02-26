package scripting

import (
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

// RegisterModules registers all engine.* Lua tables into L.
//
// Precondition: L must be from NewSandboxedState.
// Postcondition: engine global is defined in L with submodules:
//
//	engine.log, engine.dice, engine.entity, engine.combat, engine.world, engine.event.
func (m *Manager) RegisterModules(L *lua.LState) {
	engine := L.NewTable()
	L.SetGlobal("engine", engine)

	L.SetField(engine, "log", m.newLogModule(L))
	L.SetField(engine, "dice", m.newDiceModule(L))
	L.SetField(engine, "entity", m.newEntityModule(L))
	L.SetField(engine, "combat", m.newCombatModule(L))
	L.SetField(engine, "world", m.newWorldModule(L))
	L.SetField(engine, "event", m.newEventModule(L))
}

// newLogModule returns the engine.log table with debug/info/warn/error functions.
//
// Precondition: L must be non-nil.
// Postcondition: Returned table has four callable fields.
func (m *Manager) newLogModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	L.SetField(t, "debug", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		m.logger.Debug(msg)
		return 0
	}))
	L.SetField(t, "info", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		m.logger.Info(msg)
		return 0
	}))
	L.SetField(t, "warn", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		m.logger.Warn(msg)
		return 0
	}))
	L.SetField(t, "error", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		m.logger.Error(msg)
		return 0
	}))
	return t
}

// newDiceModule returns the engine.dice table.
//
// Precondition: L must be non-nil; m.roller must be non-nil.
// Postcondition: engine.dice.roll(expr) returns a table {total, modifier} or nil on error.
func (m *Manager) newDiceModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	L.SetField(t, "roll", L.NewFunction(func(L *lua.LState) int {
		expr := L.CheckString(1)
		result, err := m.roller.RollExpr(expr)
		if err != nil {
			m.logger.Warn("engine.dice.roll: invalid expression",
				zap.String("expr", expr),
				zap.Error(err),
			)
			L.Push(lua.LNil)
			return 1
		}
		tbl := L.NewTable()
		L.SetField(tbl, "total", lua.LNumber(result.Total()))
		L.SetField(tbl, "dice", lua.LNumber(result.Total()-result.Modifier))
		L.SetField(tbl, "modifier", lua.LNumber(result.Modifier))
		L.Push(tbl)
		return 1
	}))
	return t
}

// combatantToTable converts a CombatantInfo snapshot to a Lua table.
//
// Precondition: L and c must be non-nil.
// Postcondition: Returned table has uid, name, hp, max_hp, ac, kind, conditions fields.
func combatantToTable(L *lua.LState, c *CombatantInfo) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "uid", lua.LString(c.UID))
	L.SetField(tbl, "name", lua.LString(c.Name))
	L.SetField(tbl, "hp", lua.LNumber(c.HP))
	L.SetField(tbl, "max_hp", lua.LNumber(c.MaxHP))
	L.SetField(tbl, "ac", lua.LNumber(c.AC))
	L.SetField(tbl, "kind", lua.LString(c.Kind))
	conds := L.NewTable()
	for i, cond := range c.Conditions {
		L.RawSetInt(conds, i+1, lua.LString(cond))
	}
	L.SetField(tbl, "conditions", conds)
	return tbl
}

// newEntityModule returns the engine.entity table.
//
// Precondition: L must be non-nil.
// Postcondition: All functions return LNil when GetCombatant is nil or returns nil.
func (m *Manager) newEntityModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()

	getCombatant := func(L *lua.LState) *CombatantInfo {
		if m.GetCombatant == nil {
			return nil
		}
		uid := L.CheckString(1)
		return m.GetCombatant(uid)
	}

	L.SetField(t, "get_hp", L.NewFunction(func(L *lua.LState) int {
		c := getCombatant(L)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(c.HP))
		return 1
	}))
	L.SetField(t, "get_max_hp", L.NewFunction(func(L *lua.LState) int {
		c := getCombatant(L)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(c.MaxHP))
		return 1
	}))
	L.SetField(t, "get_name", L.NewFunction(func(L *lua.LState) int {
		c := getCombatant(L)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(c.Name))
		return 1
	}))
	L.SetField(t, "get_ac", L.NewFunction(func(L *lua.LState) int {
		c := getCombatant(L)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(c.AC))
		return 1
	}))
	L.SetField(t, "get_conditions", L.NewFunction(func(L *lua.LState) int {
		c := getCombatant(L)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		conds := L.NewTable()
		for i, cond := range c.Conditions {
			L.RawSetInt(conds, i+1, lua.LString(cond))
		}
		L.Push(conds)
		return 1
	}))
	L.SetField(t, "set_attr", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement attribute mutation
		return 0
	}))
	L.SetField(t, "move", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement entity movement
		return 0
	}))
	return t
}

// newCombatModule returns the engine.combat table.
//
// Precondition: L must be non-nil.
// Postcondition: All nil-callback functions are no-ops; query functions return LNil when callback is nil.
func (m *Manager) newCombatModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()

	L.SetField(t, "apply_condition", L.NewFunction(func(L *lua.LState) int {
		if m.ApplyCondition == nil {
			return 0
		}
		uid := L.CheckString(1)
		condID := L.CheckString(2)
		stacks := L.CheckInt(3)
		duration := L.CheckInt(4)
		if err := m.ApplyCondition(uid, condID, stacks, duration); err != nil {
			m.logger.Warn("engine.combat.apply_condition error",
				zap.String("uid", uid),
				zap.String("cond", condID),
				zap.Error(err),
			)
		}
		return 0
	}))
	L.SetField(t, "apply_damage", L.NewFunction(func(L *lua.LState) int {
		if m.ApplyDamage == nil {
			return 0
		}
		uid := L.CheckString(1)
		hp := L.CheckInt(2)
		if err := m.ApplyDamage(uid, hp); err != nil {
			m.logger.Warn("engine.combat.apply_damage error",
				zap.String("uid", uid),
				zap.Int("hp", hp),
				zap.Error(err),
			)
		}
		return 0
	}))
	L.SetField(t, "query_combatant", L.NewFunction(func(L *lua.LState) int {
		if m.GetCombatant == nil {
			L.Push(lua.LNil)
			return 1
		}
		uid := L.CheckString(1)
		c := m.GetCombatant(uid)
		if c == nil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(combatantToTable(L, c))
		return 1
	}))
	L.SetField(t, "initiate", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement combat initiation
		return 0
	}))
	L.SetField(t, "resolve_action", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement action resolution
		return 0
	}))
	return t
}

// newWorldModule returns the engine.world table.
//
// Precondition: L must be non-nil.
// Postcondition: All nil-callback functions are no-ops; query functions return LNil when callback is nil.
func (m *Manager) newWorldModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()

	L.SetField(t, "broadcast", L.NewFunction(func(L *lua.LState) int {
		if m.Broadcast == nil {
			return 0
		}
		roomID := L.CheckString(1)
		msg := L.CheckString(2)
		m.Broadcast(roomID, msg)
		return 0
	}))
	L.SetField(t, "query_room", L.NewFunction(func(L *lua.LState) int {
		if m.QueryRoom == nil {
			L.Push(lua.LNil)
			return 1
		}
		roomID := L.CheckString(1)
		r := m.QueryRoom(roomID)
		if r == nil {
			L.Push(lua.LNil)
			return 1
		}
		tbl := L.NewTable()
		L.SetField(tbl, "id", lua.LString(r.ID))
		L.SetField(tbl, "title", lua.LString(r.Title))
		L.Push(tbl)
		return 1
	}))
	L.SetField(t, "move_entity", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement entity room movement
		return 0
	}))
	return t
}

// newEventModule returns the engine.event table with stub implementations.
//
// Precondition: L must be non-nil.
// Postcondition: All three functions are callable no-ops (stage7 stubs).
func (m *Manager) newEventModule(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	L.SetField(t, "register_listener", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement event listener registration
		return 0
	}))
	L.SetField(t, "emit", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement event emission
		return 0
	}))
	L.SetField(t, "schedule", L.NewFunction(func(L *lua.LState) int {
		// TODO(stage7): implement event scheduling
		return 0
	}))
	return t
}

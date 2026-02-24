package scripting

import lua "github.com/yuin/gopher-lua"

// RegisterModules registers all engine.* Lua tables into L.
//
// Precondition: L must be from NewSandboxedState; m must be non-nil.
// Postcondition: engine global is defined in L.
func (m *Manager) RegisterModules(L *lua.LState) {
	engine := L.NewTable()
	L.SetGlobal("engine", engine)
}

package ai

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ScriptCaller is the interface required by the Planner to evaluate Lua preconditions.
type ScriptCaller interface {
	// CallHook calls a named Lua function in the given zone's VM.
	// Returns (LNil, nil) if the function is not defined.
	CallHook(zoneID, hook string, args ...lua.LValue) (lua.LValue, error)
}

// PlannedAction is one primitive action produced by the planner.
type PlannedAction struct {
	Action string // "attack", "strike", "pass", "reload", etc.
	Target string // resolved target name/UID; empty for pass/reload
}

// Planner evaluates an HTN domain for a single NPC and produces an ordered
// action plan for the current combat round.
//
// Invariant: domain and caller must not be nil.
type Planner struct {
	domain *Domain
	caller ScriptCaller
	zoneID string
}

// NewPlanner constructs a Planner.
//
// Precondition: domain and caller must not be nil.
func NewPlanner(domain *Domain, caller ScriptCaller, zoneID string) *Planner {
	if domain == nil {
		panic("ai.NewPlanner: domain must not be nil")
	}
	if caller == nil {
		panic("ai.NewPlanner: caller must not be nil")
	}
	return &Planner{domain: domain, caller: caller, zoneID: zoneID}
}

// Plan evaluates the HTN domain against state and returns an ordered plan.
//
// Precondition: state and state.NPC must not be nil.
// Postcondition: returns non-nil slice (may be empty); never returns error for Lua failures
// (they are treated as precondition-false).
func (p *Planner) Plan(state *WorldState) ([]PlannedAction, error) {
	if state == nil || state.NPC == nil {
		return nil, fmt.Errorf("ai.Planner.Plan: state and state.NPC must not be nil")
	}

	// Begin with the root task "behave".
	taskQueue := []string{"behave"}
	var result []PlannedAction

	const maxDepth = 32 // guard against infinite loops
	steps := 0

	for len(taskQueue) > 0 && steps < maxDepth {
		steps++
		current := taskQueue[0]
		taskQueue = taskQueue[1:]

		// Primitive operator — resolve and emit.
		if op, ok := p.domain.OperatorByID(current); ok {
			target := state.ResolveTarget(op.Target)
			result = append(result, PlannedAction{Action: op.Action, Target: target})
			continue
		}

		// Abstract task — find applicable method.
		method := p.findApplicableMethod(current, state)
		if method == nil {
			// No applicable method; skip this branch.
			continue
		}

		// Prepend subtasks (preserves ordered decomposition).
		taskQueue = append(method.Subtasks, taskQueue...)
	}

	if result == nil {
		result = []PlannedAction{}
	}
	return result, nil
}

// findApplicableMethod returns the first Method for taskID whose precondition passes,
// or nil if none applies.
//
// Methods are tried in declaration order. An empty Precondition always passes.
func (p *Planner) findApplicableMethod(taskID string, state *WorldState) *Method {
	for _, m := range p.domain.MethodsForTask(taskID) {
		if m.Precondition == "" {
			return m
		}
		val, _ := p.caller.CallHook(p.zoneID, m.Precondition, lua.LString(state.NPC.UID))
		if val == lua.LTrue {
			return m
		}
	}
	return nil
}

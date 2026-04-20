// Package ai implements the Hierarchical Task Network (HTN) planner for NPC behavior.
//
// HTN planning decomposes abstract tasks into primitive operators via ordered methods.
// Method preconditions are evaluated as Lua hooks; operators map to combat actions.
package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Task is an abstract goal that can be decomposed by methods.
//
// Precondition: ID must be non-empty.
type Task struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
}

// Method decomposes a task into an ordered list of subtasks or operator IDs.
//
// Precondition: TaskID, ID, and Subtasks must be non-empty.
// Precondition: Precondition is a Lua function name; empty means always applicable.
type Method struct {
	TaskID       string   `yaml:"task"`
	ID           string   `yaml:"id"`
	Precondition string   `yaml:"precondition"` // Lua function name; empty = always applicable
	Subtasks     []string `yaml:"subtasks"`
	// NativePrecondition is a native WorldState precondition key. When non-empty,
	// it is evaluated by the planner against WorldState fields instead of a Lua hook.
	// Supported values: "in_combat", "not_in_combat", "player_entered_room",
	// "hp_pct_below:<N>", "on_damage_taken", "has_grudge_target".
	NativePrecondition string `yaml:"native_precondition,omitempty"`
}

// Operator is a primitive action that maps directly to a combat or world action.
//
// Precondition: ID and Action must be non-empty.
type Operator struct {
	ID     string `yaml:"id"`
	Action string `yaml:"action"` // "attack", "pass", "strike", "flee", "apply_mental_state"
	Target string `yaml:"target"` // "nearest_enemy", "weakest_enemy", "self", or literal name

	// Track is the mental state track for apply_mental_state operators.
	// One of "rage", "despair", "delirium". Empty string = not a mental state ability.
	Track string `yaml:"track,omitempty"`

	// Severity is the minimum severity to apply: "mild", "moderate", or "severe".
	Severity string `yaml:"severity,omitempty"`

	// CooldownRounds is the number of rounds before this operator can fire again.
	CooldownRounds int `yaml:"cooldown_rounds,omitempty"`

	// APCost is the AP consumed when this operator executes. Zero treated as 1.
	APCost int `yaml:"ap_cost,omitempty"`

	// Strings is the pool of lines for the "say" action; one is chosen at random.
	Strings []string `yaml:"strings,omitempty"`

	// Cooldown is a Go duration string for the "say" action cooldown. Parsed at execute time.
	Cooldown string `yaml:"cooldown,omitempty"`
}

// Domain holds the full HTN domain loaded from a YAML file.
//
// Invariant: all Task, Method, and Operator IDs are unique within their slice.
type Domain struct {
	ID          string      `yaml:"id"`
	Description string      `yaml:"description"`
	Tasks       []*Task     `yaml:"tasks"`
	Methods     []*Method   `yaml:"methods"`
	Operators   []*Operator `yaml:"operators"`
}

// Validate checks all required fields and cross-field constraints.
//
// Postcondition: nil return guarantees non-empty ID, at least one Task with non-empty ID,
// all Method TaskIDs and IDs non-empty with non-empty Subtasks, all Operator IDs and Actions
// non-empty (valid action strings: "attack", "strike", "pass", "flee", "apply_mental_state"),
// no duplicate IDs within any slice, and all cross-references are valid.
func (d *Domain) Validate() error {
	if d.ID == "" {
		return errors.New("ai.Domain: ID must not be empty")
	}
	if len(d.Tasks) == 0 {
		return fmt.Errorf("ai.Domain %q: must have at least one task", d.ID)
	}
	for _, t := range d.Tasks {
		if t.ID == "" {
			return fmt.Errorf("ai.Domain %q: task has empty ID", d.ID)
		}
	}
	for _, m := range d.Methods {
		if m.TaskID == "" || m.ID == "" {
			return fmt.Errorf("ai.Domain %q: method missing TaskID or ID", d.ID)
		}
		if len(m.Subtasks) == 0 {
			return fmt.Errorf("ai.Domain %q method %q: subtasks must not be empty", d.ID, m.ID)
		}
	}
	validActions := map[string]bool{
		"attack": true, "strike": true, "pass": true, "flee": true,
		"apply_mental_state": true, "move_random": true, "say": true,
		"call_for_help": true, "target_weakest": true,
		"lua_hook": true, // AI item operator — Lua function in CombatScript
	}
	for _, op := range d.Operators {
		if op.ID == "" || op.Action == "" {
			return fmt.Errorf("ai.Domain %q: operator missing ID or Action", d.ID)
		}
		if !validActions[op.Action] {
			return fmt.Errorf("ai.Domain %q operator %q: unknown action %q", d.ID, op.ID, op.Action)
		}
	}

	// Check for duplicate Task IDs
	taskIDs := make(map[string]struct{}, len(d.Tasks))
	for _, t := range d.Tasks {
		if _, dup := taskIDs[t.ID]; dup {
			return fmt.Errorf("ai.Domain %q: duplicate task ID %q", d.ID, t.ID)
		}
		taskIDs[t.ID] = struct{}{}
	}

	// Check for duplicate Method IDs
	methodIDs := make(map[string]struct{}, len(d.Methods))
	for _, m := range d.Methods {
		if _, dup := methodIDs[m.ID]; dup {
			return fmt.Errorf("ai.Domain %q: duplicate method ID %q", d.ID, m.ID)
		}
		methodIDs[m.ID] = struct{}{}
	}

	// Check for duplicate Operator IDs
	operatorIDs := make(map[string]struct{}, len(d.Operators))
	for _, op := range d.Operators {
		if _, dup := operatorIDs[op.ID]; dup {
			return fmt.Errorf("ai.Domain %q: duplicate operator ID %q", d.ID, op.ID)
		}
		operatorIDs[op.ID] = struct{}{}
	}

	// Check method TaskID references
	for _, m := range d.Methods {
		if _, ok := taskIDs[m.TaskID]; !ok {
			return fmt.Errorf("ai.Domain %q method %q: TaskID %q references unknown task", d.ID, m.ID, m.TaskID)
		}
	}

	// Build combined set of valid subtask targets (task IDs + operator IDs)
	validSubtasks := make(map[string]struct{}, len(d.Tasks)+len(d.Operators))
	for id := range taskIDs {
		validSubtasks[id] = struct{}{}
	}
	for id := range operatorIDs {
		validSubtasks[id] = struct{}{}
	}
	for _, m := range d.Methods {
		for _, sub := range m.Subtasks {
			if _, ok := validSubtasks[sub]; !ok {
				return fmt.Errorf("ai.Domain %q method %q: subtask %q is neither a task nor an operator", d.ID, m.ID, sub)
			}
		}
	}

	return nil
}

// OperatorByID returns the operator with the given ID, or false if not found.
func (d *Domain) OperatorByID(id string) (*Operator, bool) {
	for _, op := range d.Operators {
		if op.ID == id {
			return op, true
		}
	}
	return nil, false
}

// MethodsForTask returns all methods that decompose taskID, in declaration order.
func (d *Domain) MethodsForTask(taskID string) []*Method {
	var out []*Method
	for _, m := range d.Methods {
		if m.TaskID == taskID {
			out = append(out, m)
		}
	}
	return out
}

// yamlDomainFile wraps the YAML top-level key.
type yamlDomainFile struct {
	Domain *Domain `yaml:"domain"`
}

// LoadDomains reads all *.yaml files from dir and returns parsed Domains.
//
// Precondition: dir must be a readable directory.
// Postcondition: returns error if any YAML file fails to parse or validate.
// Postcondition: returns (nil, nil) if dir contains no .yaml files; callers should treat empty results as a configuration error if domains are required.
func LoadDomains(dir string) ([]*Domain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ai.LoadDomains: reading %q: %w", dir, err)
	}
	var domains []*Domain
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("ai.LoadDomains: reading %s: %w", e.Name(), err)
		}
		var f yamlDomainFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("ai.LoadDomains: parsing %s: %w", e.Name(), err)
		}
		if f.Domain == nil {
			return nil, fmt.Errorf("ai.LoadDomains: %s missing top-level 'domain' key", e.Name())
		}
		if err := f.Domain.Validate(); err != nil {
			return nil, err
		}
		domains = append(domains, f.Domain)
	}
	return domains, nil
}

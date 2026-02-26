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
}

// Operator is a primitive action that maps directly to a combat or world action.
//
// Precondition: ID and Action must be non-empty.
type Operator struct {
	ID     string `yaml:"id"`
	Action string `yaml:"action"` // "attack", "pass", "strike", "flee"
	Target string `yaml:"target"` // "nearest_enemy", "weakest_enemy", "self", or literal name
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
	for _, op := range d.Operators {
		if op.ID == "" || op.Action == "" {
			return fmt.Errorf("ai.Domain %q: operator missing ID or Action", d.ID)
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

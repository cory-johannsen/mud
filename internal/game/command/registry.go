package command

import "fmt"

// Registry maps command names and aliases to Command definitions.
type Registry struct {
	commands map[string]*Command // canonical name → command
	aliases  map[string]string   // alias → canonical name
}

// NewRegistry creates a Registry populated with the given commands.
//
// Precondition: No two commands may share a canonical name or alias.
// Postcondition: Returns a Registry or an error on name/alias collisions.
func NewRegistry(cmds []Command) (*Registry, error) {
	r := &Registry{
		commands: make(map[string]*Command, len(cmds)),
		aliases:  make(map[string]string),
	}

	for i := range cmds {
		cmd := &cmds[i]
		if _, exists := r.commands[cmd.Name]; exists {
			return nil, fmt.Errorf("duplicate command name: %q", cmd.Name)
		}
		if _, exists := r.aliases[cmd.Name]; exists {
			return nil, fmt.Errorf("command name %q conflicts with an existing alias", cmd.Name)
		}
		r.commands[cmd.Name] = cmd

		for _, alias := range cmd.Aliases {
			if _, exists := r.commands[alias]; exists {
				return nil, fmt.Errorf("alias %q conflicts with command name %q", alias, alias)
			}
			if existing, exists := r.aliases[alias]; exists {
				return nil, fmt.Errorf("duplicate alias %q: used by %q and %q", alias, existing, cmd.Name)
			}
			r.aliases[alias] = cmd.Name
		}
	}

	return r, nil
}

// DefaultRegistry creates a Registry with all built-in commands.
//
// Postcondition: Returns a Registry with all built-in commands registered.
func DefaultRegistry() *Registry {
	r, err := NewRegistry(BuiltinCommands())
	if err != nil {
		panic(fmt.Sprintf("building default registry: %v", err))
	}
	return r
}

// Resolve looks up a command by name or alias and returns the command
// along with any remaining arguments.
//
// Postcondition: Returns (command, args, true) if found, or (nil, nil, false).
func (r *Registry) Resolve(input string) (*Command, bool) {
	if cmd, ok := r.commands[input]; ok {
		return cmd, true
	}
	if canonical, ok := r.aliases[input]; ok {
		return r.commands[canonical], true
	}
	return nil, false
}

// Commands returns all registered commands in no particular order.
func (r *Registry) Commands() []*Command {
	result := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// CommandsByCategory returns commands grouped by category.
func (r *Registry) CommandsByCategory() map[string][]*Command {
	categories := make(map[string][]*Command)
	for _, cmd := range r.commands {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}
	return categories
}

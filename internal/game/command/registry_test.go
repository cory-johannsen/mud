package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	assert.NotNil(t, r)
	assert.Greater(t, len(r.Commands()), 0)
}

func TestResolve_CanonicalName(t *testing.T) {
	r := DefaultRegistry()

	cmd, ok := r.Resolve("north")
	assert.True(t, ok)
	assert.Equal(t, "north", cmd.Name)
	assert.Equal(t, HandlerMove, cmd.Handler)
}

func TestResolve_Alias(t *testing.T) {
	r := DefaultRegistry()

	cmd, ok := r.Resolve("n")
	assert.True(t, ok)
	assert.Equal(t, "north", cmd.Name)
}

func TestResolve_NotFound(t *testing.T) {
	r := DefaultRegistry()

	_, ok := r.Resolve("teleport")
	assert.False(t, ok)
}

func TestResolve_AllMovementDirections(t *testing.T) {
	r := DefaultRegistry()
	directions := []struct {
		name  string
		alias string
	}{
		{"north", "n"},
		{"south", "s"},
		{"east", "e"},
		{"west", "w"},
		{"northeast", "ne"},
		{"northwest", "nw"},
		{"southeast", "se"},
		{"southwest", "sw"},
		{"up", "u"},
		{"down", "d"},
	}

	for _, d := range directions {
		cmd, ok := r.Resolve(d.name)
		require.True(t, ok, "canonical name %q not found", d.name)
		assert.Equal(t, d.name, cmd.Name)
		assert.Equal(t, HandlerMove, cmd.Handler)

		aliasCmd, ok := r.Resolve(d.alias)
		require.True(t, ok, "alias %q not found", d.alias)
		assert.Equal(t, d.name, aliasCmd.Name)
	}
}

func TestResolve_AllSystemCommands(t *testing.T) {
	r := DefaultRegistry()

	tests := []struct {
		input   string
		handler string
	}{
		{"look", HandlerLook},
		{"l", HandlerLook},
		{"exits", HandlerExits},
		{"say", HandlerSay},
		{"emote", HandlerEmote},
		{"em", HandlerEmote},
		{"who", HandlerWho},
		{"quit", HandlerQuit},
		{"exit", HandlerQuit},
		{"help", HandlerHelp},
		{"?", HandlerHelp},
	}

	for _, tt := range tests {
		cmd, ok := r.Resolve(tt.input)
		require.True(t, ok, "input %q not found", tt.input)
		assert.Equal(t, tt.handler, cmd.Handler, "input %q wrong handler", tt.input)
	}
}

func TestNewRegistry_DuplicateName(t *testing.T) {
	cmds := []Command{
		{Name: "test", Handler: "a"},
		{Name: "test", Handler: "b"},
	}
	_, err := NewRegistry(cmds)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate command name")
}

func TestNewRegistry_DuplicateAlias(t *testing.T) {
	cmds := []Command{
		{Name: "test1", Aliases: []string{"t"}, Handler: "a"},
		{Name: "test2", Aliases: []string{"t"}, Handler: "b"},
	}
	_, err := NewRegistry(cmds)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate alias")
}

func TestCommandsByCategory(t *testing.T) {
	r := DefaultRegistry()
	cats := r.CommandsByCategory()

	assert.Contains(t, cats, CategoryMovement)
	assert.Contains(t, cats, CategoryWorld)
	assert.Contains(t, cats, CategoryCommunication)
	assert.Contains(t, cats, CategorySystem)
	assert.Len(t, cats[CategoryMovement], 10)
}

func TestPropertyAllAliasesResolveToCanonical(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := DefaultRegistry()
		cmds := r.Commands()
		idx := rapid.IntRange(0, len(cmds)-1).Draw(t, "cmd_idx")
		cmd := cmds[idx]

		// Canonical name should resolve
		resolved, ok := r.Resolve(cmd.Name)
		if !ok {
			t.Fatalf("canonical name %q did not resolve", cmd.Name)
		}
		if resolved.Name != cmd.Name {
			t.Fatalf("canonical name %q resolved to %q", cmd.Name, resolved.Name)
		}

		// All aliases should resolve to same command
		for _, alias := range cmd.Aliases {
			aliasResolved, ok := r.Resolve(alias)
			if !ok {
				t.Fatalf("alias %q did not resolve", alias)
			}
			if aliasResolved.Name != cmd.Name {
				t.Fatalf("alias %q resolved to %q, expected %q", alias, aliasResolved.Name, cmd.Name)
			}
		}
	})
}

func TestIsMovementCommand(t *testing.T) {
	assert.True(t, IsMovementCommand("north"))
	assert.True(t, IsMovementCommand("south"))
	assert.True(t, IsMovementCommand("up"))
	assert.True(t, IsMovementCommand("down"))
	assert.False(t, IsMovementCommand("look"))
	assert.False(t, IsMovementCommand("say"))
}

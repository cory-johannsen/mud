package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestHandleRoomEquip_UnknownSubcommand verifies that an unrecognised subcommand
// returns a usage string.
//
// Precondition: args contains an unrecognised subcommand keyword.
// Postcondition: Return value contains "Usage".
func TestHandleRoomEquip_UnknownSubcommand(t *testing.T) {
	result := command.HandleRoomEquip("bogus")
	assert.Contains(t, result, "Usage")
}

// TestHandleRoomEquip_EmptyArgs verifies that empty args returns a usage string.
//
// Precondition: args is the empty string.
// Postcondition: Return value contains "Usage".
func TestHandleRoomEquip_EmptyArgs(t *testing.T) {
	result := command.HandleRoomEquip("")
	assert.Contains(t, result, "Usage")
}

// TestHandleRoomEquip_AddSubcommand verifies that the "add" subcommand returns
// a non-empty string.
//
// Precondition: args starts with "add".
// Postcondition: Return value is non-empty.
func TestHandleRoomEquip_AddSubcommand(t *testing.T) {
	result := command.HandleRoomEquip("add item1 1 5m false")
	assert.NotEmpty(t, result)
}

// TestHandleRoomEquip_ListSubcommand verifies that the "list" subcommand returns
// a non-empty string.
//
// Precondition: args is "list".
// Postcondition: Return value is non-empty.
func TestHandleRoomEquip_ListSubcommand(t *testing.T) {
	result := command.HandleRoomEquip("list")
	assert.NotEmpty(t, result)
}

// TestHandleRoomEquip_RemoveSubcommand verifies that the "remove" subcommand
// returns a non-empty string.
//
// Precondition: args starts with "remove".
// Postcondition: Return value is non-empty.
func TestHandleRoomEquip_RemoveSubcommand(t *testing.T) {
	result := command.HandleRoomEquip("remove item1")
	assert.NotEmpty(t, result)
}

// TestHandleRoomEquip_ModifySubcommand verifies that the "modify" subcommand
// returns a non-empty string.
//
// Precondition: args starts with "modify".
// Postcondition: Return value is non-empty.
func TestHandleRoomEquip_ModifySubcommand(t *testing.T) {
	result := command.HandleRoomEquip("modify item1 2 10m true")
	assert.NotEmpty(t, result)
}

// TestProperty_HandleRoomEquip_NeverPanics is a property test verifying that
// HandleRoomEquip never panics for any input string.
//
// Precondition: args may be any string.
// Postcondition: HandleRoomEquip returns without panicking.
func TestProperty_HandleRoomEquip_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.String().Draw(rt, "args")
		_ = command.HandleRoomEquip(args)
	})
}

package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleArchetypeSelection_ReturnsNonEmptyString(t *testing.T) {
	result := command.HandleArchetypeSelection("aggressor")
	assert.NotEmpty(t, result)
}

func TestHandleArchetypeSelection_ContainsArchetypeID(t *testing.T) {
	result := command.HandleArchetypeSelection("aggressor")
	assert.Contains(t, result, "aggressor")
}

func TestProperty_HandleArchetypeSelection_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "archetype_id")
		_ = command.HandleArchetypeSelection(id)
	})
}

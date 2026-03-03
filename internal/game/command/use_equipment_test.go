package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleUseEquipment_ReturnsNonEmpty(t *testing.T) {
	result := command.HandleUseEquipment("instance-123")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "instance-123")
}

func TestHandleUseEquipment_EmptyID(t *testing.T) {
	result := command.HandleUseEquipment("")
	assert.NotEmpty(t, result)
}

func TestProperty_HandleUseEquipment_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		_ = command.HandleUseEquipment(id)
	})
}

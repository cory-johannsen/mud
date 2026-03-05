package command_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleSummonItem_NoArgs(t *testing.T) {
	result := command.HandleSummonItem("")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_ItemIDOnly(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle")
	assert.Equal(t, "assault_rifle 1", result)
}

func TestHandleSummonItem_WithQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle 3")
	assert.Equal(t, "assault_rifle 3", result)
}

func TestHandleSummonItem_InvalidQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle abc")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_ZeroQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle 0")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_NegativeQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle -1")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_ExtraArgs(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle 3 garbage")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestPropertyHandleSummonItem_ValidQuantityAlwaysReturnsItemIDAndQty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		qty := rapid.IntRange(1, 1000).Draw(rt, "qty")
		result := command.HandleSummonItem(fmt.Sprintf("some_item %d", qty))
		assert.Equal(t, fmt.Sprintf("some_item %d", qty), result)
	})
}

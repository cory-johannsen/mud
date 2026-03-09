package command_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleGrant_NoArgs_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_UnknownSubcommand_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("items charname 10")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_MissingCharname_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("xp")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_MissingAmount_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("xp charname")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_NonNumericAmount_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("xp charname lots")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_ZeroAmount_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("xp charname 0")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_NegativeAmount_ReturnsUsage(t *testing.T) {
	result := command.HandleGrant("xp charname -5")
	assert.Contains(t, result, "Usage:")
}

func TestHandleGrant_XP_ValidArgs_ReturnsPassthrough(t *testing.T) {
	result := command.HandleGrant("xp alice 500")
	assert.Equal(t, "grant xp alice 500", result)
}

func TestHandleGrant_Money_ValidArgs_ReturnsPassthrough(t *testing.T) {
	result := command.HandleGrant("money bob 100")
	assert.Equal(t, "grant money bob 100", result)
}

func TestHandleGrant_XP_CaseInsensitive_ReturnsPassthrough(t *testing.T) {
	result := command.HandleGrant("XP alice 500")
	assert.Equal(t, "grant xp alice 500", result)
}

func TestPropertyHandleGrant_ValidXP_AlwaysPassthrough(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "name")
		amount := rapid.IntRange(1, 10000).Draw(rt, "amount")
		result := command.HandleGrant(fmt.Sprintf("xp %s %d", name, amount))
		assert.Equal(rt, fmt.Sprintf("grant xp %s %d", name, amount), result)
	})
}

func TestPropertyHandleGrant_ValidMoney_AlwaysPassthrough(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "name")
		amount := rapid.IntRange(1, 10000).Draw(rt, "amount")
		result := command.HandleGrant(fmt.Sprintf("money %s %d", name, amount))
		assert.Equal(rt, fmt.Sprintf("grant money %s %d", name, amount), result)
	})
}

package command_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func validAbilityList() string {
	return strings.Join(command.ValidAbilities, ", ")
}

func TestHandleLevelUp_NoArg_ReturnsUsage(t *testing.T) {
	result := command.HandleLevelUp("")
	assert.Contains(t, result, "Usage: levelup <ability>")
	assert.Contains(t, result, validAbilityList())
}

func TestHandleLevelUp_InvalidAbility_ReturnsError(t *testing.T) {
	result := command.HandleLevelUp("strength")
	assert.Contains(t, result, "Unknown ability 'strength'")
	assert.Contains(t, result, validAbilityList())
}

func TestHandleLevelUp_ValidAbilities_ReturnNormalized(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"brutality", "brutality"},
		{"quickness", "quickness"},
		{"grit", "grit"},
		{"reasoning", "reasoning"},
		{"savvy", "savvy"},
		{"flair", "flair"},
		{"BRUTALITY", "brutality"},
		{"Quickness", "quickness"},
		{"  grit  ", "grit"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result := command.HandleLevelUp(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPropertyHandleLevelUp_InvalidAbilities_AlwaysReturnError(t *testing.T) {
	validSet := make(map[string]bool, len(command.ValidAbilities))
	for _, a := range command.ValidAbilities {
		validSet[a] = true
	}
	rapid.Check(t, func(rt *rapid.T) {
		ability := rapid.StringMatching(`[a-zA-Z]{3,20}`).Draw(rt, "ability")
		normalized := strings.ToLower(strings.TrimSpace(ability))
		if validSet[normalized] {
			rt.Skip()
		}
		result := command.HandleLevelUp(ability)
		if !strings.Contains(result, "Unknown ability") {
			rt.Fatalf("expected error for invalid ability %q, got %q", ability, result)
		}
	})
}

func TestPropertyHandleLevelUp_ValidAbilities_AlwaysReturnNormalized(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ability := rapid.SampledFrom(command.ValidAbilities).Draw(rt, "ability")
		result := command.HandleLevelUp(ability)
		assert.Equal(rt, ability, result)
	})
}

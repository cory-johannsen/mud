package command_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleCombatDefault_NoArgs_ReturnsError(t *testing.T) {
	_, err := command.HandleCombatDefault([]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: combat_default")
}

func TestHandleCombatDefault_InvalidAction_ReturnsError(t *testing.T) {
	_, err := command.HandleCombatDefault([]string{"punch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action")
	assert.Contains(t, err.Error(), "punch")
}

func TestHandleCombatDefault_ValidActions_ReturnNormalizedAction(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"attack", "attack"},
		{"strike", "strike"},
		{"bash", "bash"},
		{"dodge", "dodge"},
		{"parry", "parry"},
		{"cast", "cast"},
		{"pass", "pass"},
		{"flee", "flee"},
		{"ATTACK", "attack"},
		{"Strike", "strike"},
		{"  bash  ", "bash"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := command.HandleCombatDefault([]string{tc.input})
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPropertyHandleCombatDefault_ValidActions_AlwaysSucceed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		action := rapid.SampledFrom(command.ValidCombatActions).Draw(rt, "action")
		result, err := command.HandleCombatDefault([]string{action})
		if err != nil {
			rt.Fatalf("expected no error for valid action %q, got %v", action, err)
		}
		if result != action {
			rt.Fatalf("expected %q, got %q", action, result)
		}
	})
}

func TestPropertyHandleCombatDefault_InvalidActions_AlwaysError(t *testing.T) {
	validSet := make(map[string]bool, len(command.ValidCombatActions))
	for _, a := range command.ValidCombatActions {
		validSet[a] = true
	}
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.StringMatching(`[a-zA-Z]{2,20}`).Draw(rt, "input")
		normalized := strings.ToLower(strings.TrimSpace(input))
		if validSet[normalized] {
			rt.Skip()
		}
		_, err := command.HandleCombatDefault([]string{input})
		if err == nil {
			rt.Fatalf("expected error for invalid action %q, got nil", input)
		}
	})
}

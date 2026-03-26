package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
)

func TestCombatInitiationMessages_AllReasonFormats(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "player_initiated",
			fn:       func() string { return combat.FormatPlayerInitiationMsg("Boss Rat") },
			expected: "You attack Boss Rat.",
		},
		{
			name:     "on_sight",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonOnSight, "") },
			expected: "Scavenger attacks you — attacked on sight.",
		},
		{
			name:     "territory",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Fence Dog", combat.ReasonTerritory, "") },
			expected: "Fence Dog attacks you — defending its territory.",
		},
		{
			name:     "provoked",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonProvoked, "") },
			expected: "Scavenger attacks you — provoked by your attack.",
		},
		{
			name:     "call_for_help",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Grunt", combat.ReasonCallForHelp, "") },
			expected: "Grunt attacks you — responding to a call for help.",
		},
		{
			name:     "wanted",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Zone Guard", combat.ReasonWanted, "") },
			expected: "Zone Guard attacks you — alerted by your wanted status.",
		},
		{
			name:     "protecting_with_name",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "Boss Scav") },
			expected: "Guard Dog attacks you — protecting Boss Scav.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fn())
		})
	}
}

package telnet_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
)

func TestRenderCombatGrid_EmptyGrid(t *testing.T) {
	positions := []*gamev1.CombatantPosition{}
	grid := telnet.RenderCombatGrid(positions, nil, 80)
	lines := strings.Split(strings.TrimRight(grid, "\n"), "\n")
	// Top border + 10 content rows + bottom border = 12 lines minimum.
	require.GreaterOrEqual(t, len(lines), 12, "expected at least 12 lines (border + 10 rows + border)")
	assert.Contains(t, lines[1], ".", "grid row should contain dots for empty squares")
}

func TestRenderCombatGrid_PlayerTokenAtOrigin(t *testing.T) {
	positions := []*gamev1.CombatantPosition{
		{Name: "Alice", X: 0, Y: 0},
	}
	legend := map[string]string{"Alice": "player"}
	grid := telnet.RenderCombatGrid(positions, legend, 80)
	assert.Contains(t, grid, "A", "grid should contain 'A' for Alice")
	assert.Contains(t, grid, "A=Alice", "legend should show A=Alice")
}

func TestRenderCombatGrid_NPCTokenAtRow9(t *testing.T) {
	positions := []*gamev1.CombatantPosition{
		{Name: "Goblin", X: 5, Y: 9},
	}
	legend := map[string]string{"Goblin": "enemy"}
	grid := telnet.RenderCombatGrid(positions, legend, 80)
	assert.Contains(t, grid, "G", "grid should contain 'G' for Goblin")
}

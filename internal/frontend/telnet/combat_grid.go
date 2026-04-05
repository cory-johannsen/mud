package telnet

import (
	"fmt"
	"strings"
	"unicode/utf8"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const combatGridWidth = 10
const combatGridHeight = 10

// RenderCombatGrid renders a 10×10 ASCII combat grid for telnet display.
// positions is a slice of combatant positions (name, X, Y).
// legend maps combatant names to their role: "player", "ally", or "enemy".
// width is the terminal width (minimum 30 used if smaller).
//
// Precondition: positions may be empty; legend may be nil.
// Postcondition: Returns a multi-line string with border, grid, and legend.
func RenderCombatGrid(positions []*gamev1.CombatantPosition, legend map[string]string, width int) string {
	if width < 30 {
		width = 30
	}

	// Build a 10×10 grid of tokens (first letter of name, uppercase).
	grid := [combatGridHeight][combatGridWidth]rune{}
	for y := 0; y < combatGridHeight; y++ {
		for x := 0; x < combatGridWidth; x++ {
			grid[y][x] = '.'
		}
	}
	for _, pos := range positions {
		x, y := int(pos.X), int(pos.Y)
		if x >= 0 && x < combatGridWidth && y >= 0 && y < combatGridHeight {
			r, _ := utf8.DecodeRuneInString(pos.Name)
			if r != utf8.RuneError {
				grid[y][x] = toUpperRune(r)
			}
		}
	}

	var sb strings.Builder

	// Top border: +----...----+
	sb.WriteString("+")
	for x := 0; x < combatGridWidth; x++ {
		sb.WriteString("--")
	}
	sb.WriteString("-+\n")

	// Grid rows (row 0 at top = player side, row 9 at bottom = NPC side).
	for y := 0; y < combatGridHeight; y++ {
		sb.WriteString("|")
		for x := 0; x < combatGridWidth; x++ {
			sb.WriteRune(' ')
			sb.WriteRune(grid[y][x])
		}
		sb.WriteString(" |\n")
	}

	// Bottom border.
	sb.WriteString("+")
	for x := 0; x < combatGridWidth; x++ {
		sb.WriteString("--")
	}
	sb.WriteString("-+\n")

	// Legend.
	if len(positions) > 0 {
		sb.WriteString("Legend: ")
		sep := ""
		for _, pos := range positions {
			r, _ := utf8.DecodeRuneInString(pos.Name)
			token := string(toUpperRune(r))
			sb.WriteString(fmt.Sprintf("%s%s=%s", sep, token, pos.Name))
			sep = ", "
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// toUpperRune converts a lowercase ASCII rune to uppercase.
func toUpperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// internal/frontend/handlers/text_renderer_combat.go
package handlers

import (
	"fmt"
	"strings"
)

// combatLogDisplayLines is the maximum number of log lines shown on screen.
const combatLogDisplayLines = 5

// RenderCombatScreen renders the full combat screen for a telnet client.
// The output uses \r\n line endings (telnet convention).
func RenderCombatScreen(snap CombatRenderSnapshot, width int) string {
	if width < 40 {
		width = 40
	}

	var sb strings.Builder

	// Line 1: round header, centered.
	header := fmt.Sprintf("=== Combat — Round %d ===", snap.Round)
	sb.WriteString(centerPad(header, width))
	sb.WriteString("\r\n")

	// Line 2: battlefield.
	sb.WriteString(truncateLine(RenderBattlefield(snap.TurnOrder, snap.PlayerName, width), width))
	sb.WriteString("\r\n")

	// Divider.
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\r\n")

	// Roster rows in turn order.
	for _, name := range snap.TurnOrder {
		if c, ok := snap.Combatants[name]; ok {
			sb.WriteString(RenderRosterRow(*c, width))
			sb.WriteString("\r\n")
		}
	}

	// Divider.
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\r\n")

	// Combat log: last N messages.
	logLines := snap.Log
	if len(logLines) > combatLogDisplayLines {
		logLines = logLines[len(logLines)-combatLogDisplayLines:]
	}
	for _, msg := range logLines {
		sb.WriteString(truncateLine(msg, width))
		sb.WriteString("\r\n")
	}

	return sb.String()
}

// battlefieldSep is the fixed separator between combatant tokens on the battlefield.
const battlefieldSep = "───"

// RenderBattlefield renders a 1D battlefield with the player on the left and
// enemies on the right. Format: [*Alice]───[Goblin]───[Orc]
// Player token uses a leading '*' marker. The result MUST NOT exceed width
// visible characters.
func RenderBattlefield(turnOrder []string, playerName string, width int) string {
	if len(turnOrder) == 0 {
		return ""
	}

	// Partition into player tokens (left) and NPC tokens (right).
	// Player always appears on the left regardless of initiative order.
	var playerTokens, npcTokens []string
	for _, name := range turnOrder {
		truncated := truncateStr(name, 8)
		if name == playerName {
			playerTokens = append(playerTokens, "[*"+truncated+"]")
		} else {
			npcTokens = append(npcTokens, "["+truncated+"]")
		}
	}
	tokens := append(playerTokens, npcTokens...)

	if len(tokens) == 1 {
		return tokens[0]
	}

	// Join tokens with a fixed 3-dash separator.
	line := strings.Join(tokens, battlefieldSep)

	// Truncate to width if needed.
	runes := []rune(line)
	if len(runes) > width {
		runes = runes[:width]
		line = string(runes)
	}
	return line
}

// RenderRosterRow renders a single combatant's status line.
// Format: > Name         [####....] 20/30  ●●○  [cond1,cond2]
// The ">" marker indicates the combatant whose turn it is.
// The result MUST NOT exceed width visible characters.
func RenderRosterRow(c CombatantState, width int) string {
	// Marker: ">" for current turn, " " otherwise.
	marker := "  "
	if c.IsCurrent {
		marker = "> "
	}

	// Name field: 12 chars, left-aligned, truncated.
	nameField := truncateStr(c.Name, 12)
	nameField = nameField + strings.Repeat(" ", 12-len([]rune(nameField)))

	// AP dots.
	dots := apDots(c.AP, c.MaxAP)

	// Conditions suffix.
	condStr := ""
	if len(c.Conditions) > 0 {
		condStr = " [" + strings.Join(c.Conditions, ",") + "]"
	}

	// Calculate remaining width for the HP bar portion.
	// Layout: marker(2) + name(12) + " " + hpBar + " " + dots + condStr
	// hpBar = "[" + bar + "] " + "current/max"
	// We need to calculate the HP numeric suffix first to size the bar.
	hpSuffix := fmt.Sprintf(" %d/%d", c.HP, c.MaxHP)

	dotsWidth := len([]rune(dots))
	condWidth := len([]rune(condStr))

	// Fixed overhead: marker(2) + name(12) + space(1) + brackets(2) + hpSuffix + space(1) + dots + cond
	fixedWidth := 2 + 12 + 1 + 2 + len([]rune(hpSuffix)) + 1 + dotsWidth + condWidth
	barInnerWidth := width - fixedWidth
	if barInnerWidth < 4 {
		barInnerWidth = 4
	}
	if barInnerWidth > 20 {
		barInnerWidth = 20
	}

	hpBarStr := hpBar(c.HP, c.MaxHP, barInnerWidth)

	row := marker + nameField + " " + hpBarStr + " " + dots + condStr

	// Final safety truncation.
	runes := []rune(row)
	if len(runes) > width {
		runes = runes[:width]
		row = string(runes)
	}

	return row
}

// RenderCombatSummary renders a post-combat summary display.
func RenderCombatSummary(summaryText string, width int) string {
	if width < 40 {
		width = 40
	}

	divider := strings.Repeat("─", width)

	var sb strings.Builder
	sb.WriteString(divider)
	sb.WriteString("\r\n")
	sb.WriteString(centerPad("Combat Over", width))
	sb.WriteString("\r\n")
	sb.WriteString(divider)
	sb.WriteString("\r\n")
	sb.WriteString(truncateLine(summaryText, width))
	sb.WriteString("\r\n")
	sb.WriteString(divider)
	sb.WriteString("\r\n")
	sb.WriteString(centerPad("Returning to room...", width))
	sb.WriteString("\r\n")

	return sb.String()
}

// centerPad centers s within a field of the given width, padding with spaces.
// If s is wider than width, it is truncated.
func centerPad(s string, width int) string {
	runes := []rune(s)
	visLen := len(runes)
	if visLen >= width {
		return string(runes[:width])
	}
	leftPad := (width - visLen) / 2
	rightPad := width - visLen - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// truncateLine truncates a string to fit within width runes.
func truncateLine(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

// truncateStr truncates a string to at most max runes.
func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

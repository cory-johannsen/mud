package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/effect"
	effectrender "github.com/cory-johannsen/mud/internal/game/effect/render"
	"github.com/cory-johannsen/mud/internal/game/maputil"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ansiReset is the ANSI escape sequence to reset all terminal attributes.
const ansiReset = "\033[0m"

// DangerColor returns the ANSI color escape for a danger level.
// Unexplored rooms (empty or unknown danger level) return light gray.
// Precondition: dangerLevel is a DangerLevel string or empty.
// Postcondition: returns an ANSI escape sequence string, never empty.
func DangerColor(dangerLevel string) string {
	switch dangerLevel {
	case "safe":
		return "\033[32m" // green
	case "sketchy":
		return "\033[33m" // yellow
	case "dangerous":
		return "\033[38;5;208m" // orange
	case "all_out_war":
		return "\033[31m" // red
	default:
		return "\033[37m" // light gray
	}
}

// RenderRoomView formats a RoomView as colored Telnet text, capped at maxLines rows.
//
// Precondition: width > 0 for word-wrapping and column layout; if width <= 0
// a fallback single-column layout is used. maxLines > 0 enforces a hard row
// limit so the output never overflows the pinned room region.
// Postcondition: Returns a multi-line ANSI-colored string of at most maxLines
// rows suitable for WriteRoom.
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int, dt gameserver.GameDateTime, activeWeather string) string {
	// Collect all lines in priority order, then trim to maxLines.
	var lines []string

	if activeWeather != "" {
		banner := fmt.Sprintf("*** %s ***", strings.ToUpper(activeWeather))
		padLen := 0
		if width > len(banner) {
			padLen = (width - len(banner)) / 2
		}
		centered := strings.Repeat(" ", padLen) + banner
		lines = append(lines, telnet.Colorize(telnet.BrightCyan, centered))
	}

	if rv.Title != "" {
		dateStr := gameserver.FormatDate(dt.Month, dt.Day)
		periodStr := string(dt.Hour.Period())
		hourStr := dt.Hour.String()
		titlePart := telnet.Colorize(telnet.BrightYellow, rv.Title)
		if rv.ZoneName != "" {
			titlePart = telnet.Colorize(telnet.BrightYellow, fmt.Sprintf("[%s] ", rv.ZoneName)) + titlePart
		}
		header := titlePart +
			telnet.Colorize(telnet.BrightYellow, " \u2014 ") +
			telnet.Colorize(telnet.BrightCyan, fmt.Sprintf("%s %s %s", dateStr, periodStr, hourStr))
		lines = append(lines, header)
	}
	if rv.Description != "" {
		if width > 0 {
			for _, line := range telnet.WrapText(rv.Description, width) {
				lines = append(lines, telnet.Colorize(telnet.White, line))
			}
		} else {
			lines = append(lines, telnet.Colorize(telnet.White, rv.Description))
		}
	}

	// Exits — 4 per row with "Exits: " label inline on the first row.
	if len(rv.Exits) > 0 {
		lines = append(lines, renderExits(rv.Exits, width)...)
	}

	// Other players
	if len(rv.Players) > 0 {
		lines = append(lines, telnet.Colorf(telnet.Green, "Also here: %s", strings.Join(rv.Players, ", ")))
	}

	// Equipment — 4 per row with "Items: " label inline on the first row.
	if len(rv.Equipment) > 0 {
		lines = append(lines, renderEquipment(rv.Equipment, width)...)
	}

	// NPCs present — 2 per row with "NPCs:  " label inline on the first row.
	if len(rv.Npcs) > 0 {
		lines = append(lines, renderNPCs(rv.Npcs, width)...)
	}

	// Enforce maxLines hard limit.
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\r\n")
	}
	return b.String()
}

// npcHealthColor returns the ANSI color escape for an NPC health description.
//
// Postcondition: Returns a non-empty ANSI escape string.
func npcHealthColor(desc string) string {
	switch desc {
	case "unharmed":
		return telnet.Green
	case "barely scratched", "lightly wounded":
		return telnet.BrightGreen
	case "moderately wounded":
		return telnet.Yellow
	case "heavily wounded":
		return telnet.BrightRed
	case "critically wounded":
		return telnet.Red
	default:
		return telnet.White
	}
}

// renderEquipment renders room equipment 4 per row, with "Items: " label inline on the first row.
//
// Precondition: equipment must be non-empty.
// Postcondition: Returns at least one non-empty string.
func renderEquipment(equipment []*gamev1.RoomEquipmentItem, width int) []string {
	const perRow = 4
	label := telnet.Colorize(telnet.BrightWhite, "Items: ")
	labelVisLen := len("Items: ")
	indentStr := strings.Repeat(" ", labelVisLen)

	colW := 0
	if width > labelVisLen {
		colW = (width - labelVisLen) / perRow
	}

	var out []string
	for i := 0; i < len(equipment); i += perRow {
		var rowBuf strings.Builder
		if i == 0 {
			rowBuf.WriteString(label)
		} else {
			rowBuf.WriteString(indentStr)
		}
		for j := 0; j < perRow && i+j < len(equipment); j++ {
			eq := equipment[i+j]
			name := eq.Name
			if eq.Usable {
				name += " [interact]"
			}
			entry := fmt.Sprintf("%s%s%s", telnet.Cyan, name, telnet.Reset)
			if colW > 0 && j < perRow-1 {
				visLen := len(telnet.StripANSI(entry))
				if visLen < colW {
					entry += strings.Repeat(" ", colW-visLen)
				}
			}
			rowBuf.WriteString(entry)
		}
		out = append(out, rowBuf.String())
	}
	return out
}

// npcTypeTag returns a short display tag for a non-combat NPC type, or "" for combat NPCs.
func npcTypeTag(npcType string) string {
	switch npcType {
	case "merchant":
		return "[shop]"
	case "healer":
		return "[healer]"
	case "banker":
		return "[bank]"
	case "job_trainer":
		return "[trainer]"
	case "quest_giver":
		return "[quest]"
	case "guard":
		return "[guard]"
	case "fixer":
		return "[fixer]"
	case "hireling":
		return "[hire]"
	case "motel_keeper":
		return "[motel]"
	case "chip_doc":
		return "[chip doc]"
	case "crafter":
		return "[crafter]"
	case "brothel_keeper":
		return "[brothel]"
	case "black_market_merchant":
		return "[black market]"
	case "tech_trainer":
		return "[tech trainer]"
	default:
		return ""
	}
}

// renderNPCs renders NPC entries 2 per row, with "NPCs:  " label inline on the first row.
// Combat NPCs: "Name (health)" or "Name (health) fighting Target [cond1, cond2]"
// Non-combat NPCs: "Name [type]" — health is omitted since they cannot enter combat.
//
// Precondition: npcs must be non-empty.
// Postcondition: Returns at least one non-empty string.
func renderNPCs(npcs []*gamev1.NpcInfo, width int) []string {
	const perRow = 2
	label := telnet.Colorize(telnet.BrightWhite, "NPCs:  ")
	labelVisLen := len("NPCs:  ")
	indentStr := strings.Repeat(" ", labelVisLen)

	colW := 0
	if width > labelVisLen {
		colW = (width - labelVisLen) / perRow
	}

	var out []string
	for i := 0; i < len(npcs); i += perRow {
		var rowBuf strings.Builder
		if i == 0 {
			rowBuf.WriteString(label)
		} else {
			rowBuf.WriteString(indentStr)
		}
		for j := 0; j < perRow && i+j < len(npcs); j++ {
			n := npcs[i+j]
			tag := npcTypeTag(n.NpcType)
			nameStr := fmt.Sprintf("%s%s%s", telnet.Yellow, n.Name, telnet.Reset)
			var entry string
			if tag != "" {
				// Non-combat NPC: show type tag only, no health
				entry = fmt.Sprintf("%s %s%s%s", nameStr, telnet.Cyan, tag, telnet.Reset)
			} else {
				// Combat NPC: show health
				healthColor := npcHealthColor(n.HealthDescription)
				entry = fmt.Sprintf("%s %s(%s)%s", nameStr, healthColor, n.HealthDescription, telnet.Reset)
			}
			if n.FightingTarget != "" {
				entry += fmt.Sprintf(" %sfighting %s%s",
					telnet.BrightRed, n.FightingTarget, telnet.Reset)
			}
			if len(n.Conditions) > 0 {
				entry += fmt.Sprintf(" %s%s%s",
					telnet.Yellow, strings.Join(n.Conditions, ", "), telnet.Reset)
			}
			// Pad to column width so second column aligns (skip padding on last in row).
			if colW > 0 && j < perRow-1 {
				visLen := len(telnet.StripANSI(entry))
				if visLen < colW {
					entry += strings.Repeat(" ", colW-visLen)
				}
			}
			rowBuf.WriteString(entry)
		}
		out = append(out, rowBuf.String())
	}
	return out
}

// renderExits renders exit entries 4 per row, with "Exits: " label inline on the first row.
//
// Precondition: exits must be non-empty.
// Postcondition: Returns at least one non-empty string; first string begins with "Exits: " label.
func renderExits(exits []*gamev1.ExitInfo, width int) []string {
	const perRow = 4
	const dirW = 10 // visible chars for direction + optional "*"

	label := telnet.Colorize(telnet.Cyan, "Exits: ")
	labelVisLen := len("Exits: ")
	indentStr := strings.Repeat(" ", labelVisLen)

	if width <= 0 {
		var out []string
		for i, e := range exits {
			dir := e.Direction
			if e.Locked {
				dir += "*"
			}
			var entry string
			if e.TargetTitle != "" {
				entry = fmt.Sprintf("%s%-10s%s %s%s%s",
					telnet.BrightCyan, dir, telnet.Reset,
					telnet.Dim, e.TargetTitle, telnet.Reset)
			} else {
				entry = fmt.Sprintf("%s%-10s%s", telnet.BrightCyan, dir, telnet.Reset)
			}
			if i == 0 {
				out = append(out, label+entry)
			} else {
				out = append(out, indentStr+entry)
			}
		}
		return out
	}

	colW := (width - labelVisLen) / perRow
	targetW := colW - dirW - 1
	if targetW < 0 {
		targetW = 0
	}

	var out []string
	for i := 0; i < len(exits); i += perRow {
		var rowBuf strings.Builder
		if i == 0 {
			rowBuf.WriteString(label)
		} else {
			rowBuf.WriteString(indentStr)
		}
		for j := 0; j < perRow && i+j < len(exits); j++ {
			e := exits[i+j]
			dir := e.Direction
			if e.Locked {
				dir += "*"
			}
			paddedDir := fmt.Sprintf("%-*s", dirW, dir)
			if targetW > 0 && e.TargetTitle != "" {
				target := e.TargetTitle
				if len(target) > targetW {
					target = target[:targetW]
				}
				paddedTarget := fmt.Sprintf("%-*s", targetW, target)
				rowBuf.WriteString(fmt.Sprintf("%s%s%s %s%s%s",
					telnet.BrightCyan, paddedDir, telnet.Reset,
					telnet.Dim, paddedTarget, telnet.Reset))
			} else {
				blank := fmt.Sprintf("%-*s", targetW+1, "")
				rowBuf.WriteString(fmt.Sprintf("%s%s%s%s",
					telnet.BrightCyan, paddedDir, telnet.Reset, blank))
			}
		}
		out = append(out, rowBuf.String())
	}
	return out
}


// RenderNpcView formats an NpcView as Telnet text for the examine command.
func RenderNpcView(nv *gamev1.NpcView) string {
	var b strings.Builder
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightYellow, nv.Name))
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.White, nv.Description))
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorf(telnet.Cyan, "Condition: %s  Level: %d", nv.HealthDescription, nv.Level))
	b.WriteString("\r\n")
	return b.String()
}

// RenderMessage formats a MessageEvent as Telnet text.
// RenderMessage formats a MessageEvent as colored Telnet text.
// timePeriod is the current time-of-day period string (e.g. "Morning"); if
// non-empty it is prepended as "[period] " before the message body.
//
// Precondition: me must be non-nil.
// Postcondition: Returns a non-empty string for any non-nil input.
func RenderMessage(me *gamev1.MessageEvent, timePeriod string) string {
	prefix := ""
	if timePeriod != "" {
		prefix = telnet.Colorf(telnet.Dim, "[%s] ", timePeriod)
	}
	switch me.Type {
	case gamev1.MessageType_MESSAGE_TYPE_SAY:
		return prefix + telnet.Colorf(telnet.BrightWhite, "%s says: %s", me.Sender, me.Content)
	case gamev1.MessageType_MESSAGE_TYPE_EMOTE:
		return prefix + telnet.Colorf(telnet.Magenta, "%s %s", me.Sender, me.Content)
	default:
		if me.Sender != "" {
			return prefix + fmt.Sprintf("%s: %s", me.Sender, me.Content)
		}
		return prefix + me.Content
	}
}

// RenderRoomEvent formats a RoomEvent as Telnet text.
func RenderRoomEvent(re *gamev1.RoomEvent) string {
	switch re.Type {
	case gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE:
		if re.Direction != "" {
			return telnet.Colorf(telnet.Green, "%s arrived from the %s.", re.Player, re.Direction)
		}
		return telnet.Colorf(telnet.Green, "%s has arrived.", re.Player)
	case gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART:
		if re.Direction != "" {
			return telnet.Colorf(telnet.Yellow, "%s left to the %s.", re.Player, re.Direction)
		}
		return telnet.Colorf(telnet.Yellow, "%s has left.", re.Player)
	default:
		return fmt.Sprintf("%s did something.", re.Player)
	}
}

// RenderPlayerList formats a PlayerList for telnet display.
//
// Precondition: pl must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderPlayerList(pl *gamev1.PlayerList) string {
	if len(pl.Players) == 0 {
		return telnet.Colorize(telnet.Dim, "Nobody else is here.")
	}
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "Players here:\r\n"))
	for _, p := range pl.Players {
		status := statusLabel(p.Status)
		sb.WriteString(fmt.Sprintf("  %s%s%s — Lvl %d %s — %s — %s\r\n",
			telnet.Green, p.Name, telnet.Reset,
			p.Level, p.Job,
			p.HealthLabel,
			status))
	}
	return sb.String()
}

// statusLabel converts a CombatStatus to a display string.
func statusLabel(s gamev1.CombatStatus) string {
	switch s {
	case gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT:
		return "In Combat"
	case gamev1.CombatStatus_COMBAT_STATUS_RESTING:
		return "Resting"
	case gamev1.CombatStatus_COMBAT_STATUS_UNCONSCIOUS:
		return "Unconscious"
	default:
		return "Idle"
	}
}

// RenderExitList formats an ExitList as Telnet text.
func RenderExitList(el *gamev1.ExitList) string {
	if len(el.Exits) == 0 {
		return telnet.Colorize(telnet.Dim, "There are no obvious exits.")
	}
	var b strings.Builder
	b.WriteString(telnet.Colorize(telnet.Cyan, "Exits:"))
	b.WriteString("\r\n")
	for _, e := range el.Exits {
		label := e.Direction
		if e.Locked {
			label += telnet.Colorize(telnet.Red, " (locked)")
		}
		if e.Hidden {
			label += telnet.Colorize(telnet.Dim, " (hidden)")
		}
		if e.TargetTitle != "" {
			b.WriteString(fmt.Sprintf("  %s%-10s%s %s%s%s\r\n",
				telnet.BrightCyan, label, telnet.Reset,
				telnet.Dim, e.TargetTitle, telnet.Reset))
		} else {
			b.WriteString(fmt.Sprintf("  %s%s%s\r\n",
				telnet.BrightCyan, label, telnet.Reset))
		}
	}
	return b.String()
}

// RenderError formats an ErrorEvent as red Telnet text.
func RenderError(ee *gamev1.ErrorEvent) string {
	return telnet.Colorize(telnet.Red, ee.Message)
}

// RenderRoundStartEvent formats a round-start combat banner.
//
// Postcondition: Returns an ANSI-colored multiline string showing round number, action count, timer, and turn order.
func RenderRoundStartEvent(rs *gamev1.RoundStartEvent) string {
	durationSec := rs.DurationMs / 1000
	order := strings.Join(rs.TurnOrder, ", ")
	return telnet.Colorize(telnet.BrightYellow,
		fmt.Sprintf("=== Round %d begins. Actions: %d. [%ds] ===", rs.Round, rs.ActionsPerTurn, durationSec),
	) + "\r\n" +
		telnet.Colorize(telnet.White, "Turn order: "+order) + "\r\n"
}

// RenderRoundEndEvent formats a round-end combat banner.
//
// Postcondition: Returns an ANSI-colored string indicating round resolution.
func RenderRoundEndEvent(re *gamev1.RoundEndEvent) string {
	return telnet.Colorize(telnet.BrightYellow, fmt.Sprintf("=== Round %d resolved. ===", re.Round)) + "\r\n"
}

// RenderConditionEvent formats a ConditionEvent as colored Telnet text.
//
// Precondition: ce is non-nil.
// Postcondition: returns a non-empty ANSI-colored string.
func RenderConditionEvent(ce *gamev1.ConditionEvent) string {
	if ce.Applied {
		return telnet.Colorf(telnet.BrightRed, "[CONDITION] %s is now %s (stacks: %d).",
			ce.TargetName, ce.ConditionName, ce.Stacks)
	}
	return telnet.Colorf(telnet.Cyan, "[CONDITION] %s fades from %s.",
		ce.ConditionName, ce.TargetName)
}

// RenderQuestCompleteEvent formats a QuestCompleteEvent as colored Telnet text.
//
// Precondition: qce must not be nil.
// Postcondition: Returns a human-readable quest completion announcement.
func RenderQuestCompleteEvent(qce *gamev1.QuestCompleteEvent) string {
	var b strings.Builder
	b.WriteString(telnet.Colorf(telnet.BrightGreen, "✓ Quest Complete: %s", qce.Title))
	b.WriteString("\r\n")
	if qce.XpReward > 0 {
		b.WriteString(telnet.Colorf(telnet.Cyan, "  +%d XP", qce.XpReward))
		b.WriteString("\r\n")
	}
	if qce.CreditsReward > 0 {
		b.WriteString(telnet.Colorf(telnet.Cyan, "  +%d Credits", qce.CreditsReward))
		b.WriteString("\r\n")
	}
	for _, item := range qce.ItemRewards {
		b.WriteString(telnet.Colorf(telnet.Cyan, "  +%s", item))
		b.WriteString("\r\n")
	}
	return strings.TrimRight(b.String(), "\r\n")
}

// RenderCombatEvent formats a CombatEvent as colored Telnet text.
// RenderInventoryView formats an InventoryView as colored Telnet text.
func RenderInventoryView(iv *gamev1.InventoryView) string {
	var b strings.Builder
	b.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Inventory ==="))
	b.WriteString("\r\n")
	if len(iv.Items) == 0 {
		b.WriteString(telnet.Colorize(telnet.Dim, "  Your backpack is empty."))
		b.WriteString("\r\n")
	} else {
		for _, item := range iv.Items {
			qty := ""
			if item.Quantity > 1 {
				qty = fmt.Sprintf(" (x%d)", item.Quantity)
			}
			b.WriteString(fmt.Sprintf("  %s%s%s%s", telnet.BrightWhite, item.Name, telnet.Reset, qty))
			if item.Kind != "" {
				b.WriteString(fmt.Sprintf(" [%s]", item.Kind))
			}
			b.WriteString("\r\n")
		}
	}
	b.WriteString(fmt.Sprintf("  Slots: %d/%d  Weight: %.1f/%.1f",
		iv.UsedSlots, iv.MaxSlots, iv.TotalWeight, iv.MaxWeight))
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("  Currency: %s", iv.Currency))
	return b.String()
}

// RenderCharacterInfo formats a CharacterInfo event as a multi-line Telnet stats block.
//
// Precondition: ci must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderCharacterInfo(ci *gamev1.CharacterInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %s%s%s\r\n", telnet.BrightWhite, ci.Name, telnet.Reset))
	sb.WriteString(fmt.Sprintf("  Region: %s   Job: %s   Level: %d\r\n", ci.Region, ci.Class, ci.Level))
	sb.WriteString(fmt.Sprintf("  HP: %d/%d\r\n", ci.CurrentHp, ci.MaxHp))
	return sb.String()
}

// abilityBonus formats an ability score as its PF2E modifier only.
// e.g. score 14 → "+2", score 10 → "+0", score 8 → "-1"
func abilityBonus(score int32) string {
	mod := (score - 10) / 2
	if mod >= 0 {
		return fmt.Sprintf("+%d", mod)
	}
	return fmt.Sprintf("%d", mod)
}

// coloredAbilityBonus returns abilityBonus(score) wrapped in an ANSI color
// based on the modifier value, with wide bands to support high-level characters:
//
//	≤ -7: BrightRed   (extreme penalty)
//	-6 to -4: Red
//	-3 to -1: Yellow
//	0:        White
//	+1 to +4: Cyan    (teal)
//	+5 to +8: Green
//	+9 to +12: Blue
//	≥ +13: Magenta    (purple)
func coloredAbilityBonus(score int32) string {
	mod := (score - 10) / 2
	s := abilityBonus(score)
	switch {
	case mod <= -7:
		return telnet.Colorize(telnet.BrightRed, s)
	case mod <= -4:
		return telnet.Colorize(telnet.Red, s)
	case mod <= -1:
		return telnet.Colorize(telnet.Yellow, s)
	case mod == 0:
		return telnet.Colorize(telnet.White, s)
	case mod <= 4:
		return telnet.Colorize(telnet.Cyan, s)
	case mod <= 8:
		return telnet.Colorize(telnet.Green, s)
	case mod <= 12:
		return telnet.Colorize(telnet.Blue, s)
	default: // mod >= 13
		return telnet.Colorize(telnet.Magenta, s)
	}
}

// coloredSignedBonus wraps a signed integer bonus (e.g. proficiency bonus) in
// an ANSI color: positive=green, zero=white, negative=red.
func coloredSignedBonus(n int32) string {
	s := signedInt(int(n))
	switch {
	case n > 0:
		return telnet.Colorize(telnet.Green, s)
	case n < 0:
		return telnet.Colorize(telnet.Red, s)
	default:
		return telnet.Colorize(telnet.White, s)
	}
}

// wordWrap splits text into lines no longer than maxWidth visible chars.
// The first line has no prefix; continuation lines are prefixed with indent.
func wordWrap(text string, maxWidth int, indent string) []string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return []string{text}
	}
	var lines []string
	remaining := text
	first := true
	for len(remaining) > 0 {
		limit := maxWidth
		if !first {
			limit = maxWidth - len(indent)
		}
		if limit <= 0 {
			limit = maxWidth
		}
		if len(remaining) <= limit {
			if first {
				lines = append(lines, remaining)
			} else {
				lines = append(lines, indent+remaining)
			}
			break
		}
		// Find last space at or before limit.
		cut := limit
		for cut > 0 && remaining[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = limit // no space found, hard-cut
		}
		if first {
			lines = append(lines, remaining[:cut])
		} else {
			lines = append(lines, indent+remaining[:cut])
		}
		remaining = strings.TrimLeft(remaining[cut:], " ")
		first = false
	}
	return lines
}

// signedInt formats an integer with an explicit sign prefix: +5, -1, +0.
func signedInt(n int) string {
	if n >= 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}

// formatSlotLabel converts a slot key like "left_arm" to "Left Arm".
func formatSlotLabel(slot string) string {
	words := strings.Split(slot, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// proficiencyColorCode returns the ANSI escape code for a given proficiency rank.
// untrained (or unknown) returns "" (no color).
func proficiencyColorCode(rank string) string {
	switch strings.ToLower(rank) {
	case "legendary":
		return telnet.Magenta
	case "master":
		return telnet.Yellow
	case "expert":
		return telnet.Cyan
	case "trained":
		return telnet.White
	default:
		return "" // untrained or unknown — no color
	}
}

// proficiencyColor wraps a proficiency rank string with the appropriate ANSI color.
// untrained receives no color (terminal default); trained through legendary are progressively brighter.
//
// Postcondition: Returns rank wrapped in ANSI color codes for legendary/master/expert/trained,
// or rank unchanged (no ANSI codes) for any other input including empty string.
func proficiencyColor(rank string) string {
	code := proficiencyColorCode(rank)
	normalized := strings.ToLower(rank)
	if code == "" {
		return rank
	}
	return telnet.Colorize(code, normalized)
}

// sheetLine is a rendered line for the character sheet two-column layout.
// text may contain ANSI escape codes; visW is the visible (printable) width.
type sheetLine struct {
	text string
	visW int
}

// sl builds a sheetLine from an already-colored string.
func sl(text string) sheetLine {
	return sheetLine{text: text, visW: len(telnet.StripANSI(text))}
}

// slPlain builds a sheetLine from a plain (no ANSI) string.
func slPlain(text string) sheetLine { return sheetLine{text: text, visW: len(text)} }

// assembleColumns zips left and right line slices into a two-column layout.
// Each row is: left line padded to leftW chars + " | " + right line + \r\n.
//
// Precondition: leftW > 0.
// Postcondition: returns a non-empty string with one output row per max(len(left), len(right)).
func assembleColumns(left, right []sheetLine, leftW int) string {
	var b strings.Builder
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		var l, r sheetLine
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		pad := leftW - l.visW
		if pad < 0 {
			pad = 0
		}
		b.WriteString(l.text)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(" | ")
		b.WriteString(r.text)
		b.WriteString("\r\n")
	}
	return b.String()
}

// RenderCharacterSheet formats a CharacterSheetView as a detailed Telnet character sheet.
//
// When width >= 73, feats and class features are placed in a right column beside
// the stats/skills block.  For narrower terminals a single-column layout is used.
//
// Precondition: csv must be non-nil; width is the terminal width (0 = unknown → single column).
// Postcondition: Returns a non-empty human-readable multiline string.
func RenderCharacterSheet(csv *gamev1.CharacterSheetView, width int) string {
	if csv == nil {
		return telnet.Colorize(telnet.Red, "No character sheet available.")
	}

	const leftW = 50 // visible width of the left column
	const minWidth = 73 // leftW + 3 (" | ") + 20 (minimum right column)
	twoCol := width >= minWidth
	rightW := 0
	if twoCol {
		rightW = width - leftW - 3
	}

	// ── Left column: header, stats, armor, currency, skills ──────────────────
	var left []sheetLine

	left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "=== %s ===", csv.GetName())))
	left = append(left, slPlain(fmt.Sprintf("Job: %s  Archetype: %s", csv.GetJob(), csv.GetArchetype())))
	left = append(left, slPlain(fmt.Sprintf("Team: %s  Level: %d", csv.GetTeam(), csv.GetLevel())))
	left = append(left, slPlain(fmt.Sprintf("Gender: %s", displayGender(csv.GetGender()))))
	left = append(left, slPlain(fmt.Sprintf("HP: %d / %d", csv.GetCurrentHp(), csv.GetMaxHp())))
	left = append(left, slPlain(fmt.Sprintf("Hero Points: %d", csv.GetHeroPoints())))
	if csv.GetPendingBoosts() > 0 {
		left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "Pending Boosts: %d", csv.GetPendingBoosts())))
	}
	if csv.GetPendingSkillIncreases() > 0 {
		left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "Pending Skill Increases: %d", csv.GetPendingSkillIncreases())))
	}
	if csv.GetMaxFocusPoints() > 0 {
		left = append(left, slPlain(fmt.Sprintf("Focus Points: %d / %d",
			csv.GetFocusPoints(), csv.GetMaxFocusPoints())))
	}

	// abilCell returns a fixed-width ability cell: "Label:     +N  " (15 visible chars).
	// The colored modifier is right-aligned after the label.
	abilCell := func(label string, score int32) (colored string, plain string) {
		mod := coloredAbilityBonus(score)
		raw := abilityBonus(score)
		colored = fmt.Sprintf("%-11s%s  ", label+":", mod)
		plain = fmt.Sprintf("%-11s%s  ", label+":", raw)
		return
	}
	left = append(left, slPlain(""))
	left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Abilities ---")))
	brtC, brtP := abilCell("Brutality", csv.GetBrutality())
	grtC, grtP := abilCell("Grit", csv.GetGrit())
	qckC, qckP := abilCell("Quickness", csv.GetQuickness())
	rsnC, rsnP := abilCell("Reasoning", csv.GetReasoning())
	savC, savP := abilCell("Savvy", csv.GetSavvy())
	flrC, flrP := abilCell("Flair", csv.GetFlair())
	left = append(left, sheetLine{text: brtC + grtC + qckC, visW: len(brtP + grtP + qckP)})
	left = append(left, sheetLine{text: rsnC + savC + flrC, visW: len(rsnP + savP + flrP)})

	left = append(left, slPlain(""))
	left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Defense ---")))
	acLine := fmt.Sprintf("AC: %s", telnet.Colorize(telnet.BrightWhite, fmt.Sprintf("%d", csv.GetTotalAc())))
	if csv.GetAcBonus() != 0 || csv.GetCheckPenalty() != 0 || csv.GetSpeedPenalty() != 0 {
		acLine += fmt.Sprintf("  (armor +%d", csv.GetAcBonus())
		if csv.GetCheckPenalty() != 0 {
			acLine += fmt.Sprintf("  check %d", csv.GetCheckPenalty())
		}
		if csv.GetSpeedPenalty() != 0 {
			acLine += fmt.Sprintf("  speed %d", csv.GetSpeedPenalty())
		}
		acLine += ")"
	}
	left = append(left, sl(acLine))
	if len(csv.GetPlayerResistances()) > 0 {
		parts := make([]string, 0, len(csv.GetPlayerResistances()))
		for _, r := range csv.GetPlayerResistances() {
			parts = append(parts, fmt.Sprintf("%s %d", r.GetDamageType(), r.GetValue()))
		}
		left = append(left, sl(telnet.Colorize(telnet.Green, fmt.Sprintf("Resist: %s", strings.Join(parts, "  ")))))
	}
	if len(csv.GetPlayerWeaknesses()) > 0 {
		parts := make([]string, 0, len(csv.GetPlayerWeaknesses()))
		for _, r := range csv.GetPlayerWeaknesses() {
			parts = append(parts, fmt.Sprintf("%s %d", r.GetDamageType(), r.GetValue()))
		}
		left = append(left, sl(telnet.Colorize(telnet.Red, fmt.Sprintf("Weak:   %s", strings.Join(parts, "  ")))))
	}

	left = append(left, slPlain(""))
	left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Saves ---")))
	left = append(left, slPlain(fmt.Sprintf("Toughness: %s  Hustle: %s  Cool: %s",
		signedInt(int(csv.GetToughnessSave())),
		signedInt(int(csv.GetHustleSave())),
		signedInt(int(csv.GetCoolSave())))))
	initiativeMod := signedInt((int(csv.GetQuickness()) - 10) / 2)
	left = append(left, slPlain(fmt.Sprintf("Awareness: %s  Initiative: %s",
		signedInt(int(csv.GetAwareness())), initiativeMod)))

	left = append(left, slPlain(""))
	left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Weapons ---")))
	mainHand := csv.GetMainHand()
	if mainHand == "" {
		left = append(left, slPlain("Main: (none)"))
	} else {
		left = append(left, sl(fmt.Sprintf("Main: %s  %s  %s",
			mainHand,
			telnet.Colorize(telnet.Green, csv.GetMainHandAttackBonus()),
			telnet.Colorize(telnet.Yellow, csv.GetMainHandDamage()))))
	}
	offHand := csv.GetOffHand()
	if offHand == "" {
		left = append(left, slPlain("Off:  (none)"))
	} else {
		left = append(left, sl(fmt.Sprintf("Off:  %s  %s  %s",
			offHand,
			telnet.Colorize(telnet.Green, csv.GetOffHandAttackBonus()),
			telnet.Colorize(telnet.Yellow, csv.GetOffHandDamage()))))
	}

	if armor := csv.GetArmor(); len(armor) > 0 {
		left = append(left, slPlain(""))
		left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Armor ---")))
		for slot, item := range armor {
			if item != "" {
				left = append(left, slPlain(fmt.Sprintf("%s: %s", formatSlotLabel(slot), item)))
			}
		}
	}

	left = append(left, slPlain(""))
	left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Progress ---")))
	var xpLine string
	if csv.GetXpToNext() == 0 {
		xpLine = fmt.Sprintf("XP: %d (max)   Pending Boosts: %d",
			csv.GetExperience(), csv.GetPendingBoosts())
	} else {
		xpLine = fmt.Sprintf("XP: %d / %d   Pending Boosts: %d",
			csv.GetExperience(), csv.GetXpToNext(), csv.GetPendingBoosts())
	}
	left = append(left, slPlain(xpLine))
	// REQ-70-1: Crypto balance appears directly beneath XP.
	// curr is already formatted as "N Crypto"; display it directly.
	if curr := csv.GetCurrency(); curr != "" {
		left = append(left, slPlain(curr))
	}
	if csv.GetPendingBoosts() > 0 {
		left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "  (type 'levelup' to assign)")))
	}
	if csv.GetPendingSkillIncreases() > 0 {
		left = append(left, slPlain(fmt.Sprintf("  Pending Skill Increases: %d", csv.GetPendingSkillIncreases())))
		left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "  (type 'trainskill <skill>' to assign)")))
	}

	if skills := csv.GetSkills(); len(skills) > 0 {
		left = append(left, slPlain(""))
		left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Skills ---")))
		// Render skills 2 per row; each cell is 23 visible chars:
		//   "  name(12) abil(3) rank(4)" = 2+12+1+3+1+4 = 23
		// Ability is abbreviated to 3 chars to keep column width fixed.
		abilAbbrev := map[string]string{
			"quickness": "QCK",
			"brutality": "BRT",
			"reasoning": "RSN",
			"savvy":     "SAV",
			"flair":     "FLR",
			"grit":      "GRT",
		}
		rankAbbrev := map[string]string{
			"untrained": "untr",
			"trained":   "trnd",
			"expert":    "expt",
			"master":    "mstr",
			"legendary": "lgnd",
		}
		abbrevRank := func(rank string) string {
			if a, ok := rankAbbrev[strings.ToLower(rank)]; ok {
				return a
			}
			if len(rank) > 4 {
				return rank[:4]
			}
			return rank
		}
		abbrevAbil := func(ability string) string {
			if a, ok := abilAbbrev[strings.ToLower(ability)]; ok {
				return a
			}
			if len(ability) > 3 {
				return strings.ToUpper(ability[:3])
			}
			return strings.ToUpper(ability)
		}
		// colorRankAbbrev applies proficiencyColorCode to the 4-char abbreviated rank.
		// untrained uses BrightBlack (dim) so it is visually distinct from no-rank.
		colorRankAbbrev := func(rank string) string {
			abbr := fmt.Sprintf("%-4s", abbrevRank(rank))
			code := proficiencyColorCode(rank)
			if code != "" {
				return telnet.Colorize(code, abbr)
			}
			// untrained: dim so it reads as "low" without being invisible
			return telnet.Colorize(telnet.BrightBlack, abbr)
		}
		// cell returns the plain (no ANSI) version for width accounting.
		// Each cell is exactly 23 visible chars: 2+12+1+3+1+4 = 23.
		cell := func(sk *gamev1.SkillEntry) string {
			return fmt.Sprintf("  %-12s %-3s %-4s",
				sk.GetName(), abbrevAbil(sk.GetAbility()), abbrevRank(sk.GetProficiency()))
		}
		// colorCell returns the display version with ANSI color on the rank.
		colorCell := func(sk *gamev1.SkillEntry) string {
			return fmt.Sprintf("  %-12s %-3s %s",
				sk.GetName(), abbrevAbil(sk.GetAbility()), colorRankAbbrev(sk.GetProficiency()))
		}
		for i := 0; i < len(skills); i += 2 {
			a := skills[i]
			if i+1 < len(skills) {
				b := skills[i+1]
				visW := len(cell(a)) + len(cell(b))
				left = append(left, sheetLine{
					text: colorCell(a) + colorCell(b),
					visW: visW,
				})
			} else {
				left = append(left, sheetLine{text: colorCell(a), visW: len(cell(a))})
			}
		}
	}

	// ── Right column: feats and class features ────────────────────────────────
	var right []sheetLine

	if feats := csv.GetFeats(); len(feats) > 0 {
		right = append(right, sl(telnet.Colorize(telnet.BrightCyan, "--- Feats ---")))
		descColW := rightW
		if !twoCol {
			descColW = leftW
		}
		for _, ft := range feats {
			activeTag := ""
			if ft.GetActive() {
				activeTag = telnet.Colorize(telnet.Yellow, " [A]")
			}
			nameLine := fmt.Sprintf("  %s%s", ft.GetName(), activeTag)
			right = append(right, sl(nameLine))
			if desc := ft.GetDescription(); desc != "" {
				for _, line := range wordWrap(desc, descColW-4, "    ") {
					right = append(right, slPlain(fmt.Sprintf("    %s", line)))
				}
			}
		}
	}

	if cfs := csv.GetClassFeatures(); len(cfs) > 0 {
		if len(right) > 0 {
			right = append(right, slPlain(""))
		}
		right = append(right, sl(telnet.Colorize(telnet.BrightCyan, "--- Job Features ---")))
		cfDescColW := rightW
		if !twoCol {
			cfDescColW = leftW
		}

		hasArchetype := false
		for _, cf := range cfs {
			if cf.GetArchetype() == "" {
				continue
			}
			if !hasArchetype {
				right = append(right, sl(telnet.Colorize(telnet.Cyan, "  Archetype:")))
				hasArchetype = true
			}
			activeTag := ""
			if cf.GetActive() {
				activeTag = telnet.Colorize(telnet.Yellow, " [A]")
			}
			right = append(right, sl(fmt.Sprintf("    %s%s", cf.GetName(), activeTag)))
			if desc := cf.GetDescription(); desc != "" {
				for _, line := range wordWrap(desc, cfDescColW-6, "      ") {
					right = append(right, slPlain(fmt.Sprintf("      %s", line)))
				}
			}
		}

		hasJob := false
		for _, cf := range cfs {
			if cf.GetJob() == "" {
				continue
			}
			if !hasJob {
				right = append(right, sl(telnet.Colorize(telnet.Cyan, "  Job:")))
				hasJob = true
			}
			activeTag := ""
			if cf.GetActive() {
				activeTag = telnet.Colorize(telnet.Yellow, " [A]")
			}
			right = append(right, sl(fmt.Sprintf("    %s%s", cf.GetName(), activeTag)))
			if desc := cf.GetDescription(); desc != "" {
				for _, line := range wordWrap(desc, cfDescColW-6, "      ") {
					right = append(right, slPlain(fmt.Sprintf("      %s", line)))
				}
			}
		}
	}

	// ── Proficiencies: second column, below Class Features ───────────────────
	if profs := csv.GetProficiencies(); len(profs) > 0 {
		if len(right) > 0 {
			right = append(right, slPlain(""))
		}
		right = append(right, sl(telnet.Colorize(telnet.BrightCyan, "--- Proficiencies ---")))
		for _, e := range profs {
			rankLabel := fmt.Sprintf("[%s]", e.GetRank())
			bonusLabel := fmt.Sprintf("+%d", e.GetBonus())
			visPlain := fmt.Sprintf("  %-18s %-12s %s", e.GetName(), rankLabel, bonusLabel)
			coloredRank := proficiencyColor(e.GetRank())
			// Bonus uses the same color as the rank label so both match.
			colorCode := proficiencyColorCode(e.GetRank())
			var coloredBonus string
			if colorCode != "" {
				coloredBonus = telnet.Colorize(colorCode, bonusLabel)
			} else {
				coloredBonus = bonusLabel
			}
			text := fmt.Sprintf("  %-18s [%s] %s", e.GetName(), coloredRank, coloredBonus)
			right = append(right, sheetLine{text: text, visW: len(visPlain)})
		}
	}

	// ── Assemble ──────────────────────────────────────────────────────────────
	var out string
	if !twoCol {
		// Single column: left then right, each line terminated with \r\n.
		var b strings.Builder
		for _, l := range left {
			b.WriteString(l.text)
			b.WriteString("\r\n")
		}
		for _, r := range right {
			b.WriteString(r.text)
			b.WriteString("\r\n")
		}
		out = b.String()
	} else {
		out = assembleColumns(left, right, leftW)
	}

	// ── Effects block ─────────────────────────────────────────────────────────
	// Appended after the main sheet body when the server-supplied summary is
	// non-empty. The divider width scales with the terminal width so narrow
	// clients don't wrap; a sensible floor prevents a degenerate empty divider.
	if summary := csv.GetEffectsSummary(); summary != "" {
		dashW := width - len("── Effects ")
		if dashW < 8 {
			dashW = 8
		}
		if dashW > 64 {
			dashW = 64
		}
		var b strings.Builder
		b.WriteString(out)
		b.WriteString("\r\n")
		b.WriteString("── Effects ")
		b.WriteString(strings.Repeat("─", dashW))
		b.WriteString("\r\n")
		b.WriteString(summary)
		if !strings.HasSuffix(summary, "\n") {
			b.WriteString("\r\n")
		}
		out = b.String()
	}
	return out
}

// sortedInt32Set returns the keys of a map[int32]bool in ascending order.
func sortedInt32Set(m map[int32]bool) []int32 {
	out := make([]int32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// insertion sort — tile counts are small
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// RenderMap renders a hybrid ASCII grid + legend from a MapResponse.
// Only rows and columns that contain at least one room are rendered, eliminating
// blank padding rows caused by sparse coordinate systems.
// East/south connectors are only drawn when the neighboring cell is also discovered.
//
// Precondition: resp may be nil or have no tiles; width is the terminal width in columns.
// Postcondition: Returns a non-empty string safe for telnet display.
func RenderMap(resp *gamev1.MapResponse, width int) string {
	if resp == nil || len(resp.Tiles) == 0 {
		return "No map data.\r\n"
	}

	// Build coordinate lookup.
	byCoord := make(map[[2]int32]*gamev1.MapTile)
	for i := range resp.Tiles {
		t := resp.Tiles[i]
		byCoord[[2]int32{t.X, t.Y}] = t
	}

	// Collect unique X and Y values that have rooms, sorted ascending.
	xSet := make(map[int32]bool)
	ySet := make(map[int32]bool)
	for _, t := range resp.Tiles {
		xSet[t.X] = true
		ySet[t.Y] = true
	}
	xs := sortedInt32Set(xSet)
	ys := sortedInt32Set(ySet)

	// Assign legend numbers top-to-bottom, left-to-right.
	numByCoord := make(map[[2]int32]int)
	n := 1
	for _, y := range ys {
		for _, x := range xs {
			if _, ok := byCoord[[2]int32{x, y}]; ok {
				numByCoord[[2]int32{x, y}] = n
				n++
			}
		}
	}

	exitSet := func(t *gamev1.MapTile) map[string]bool {
		s := make(map[string]bool, len(t.Exits))
		for _, e := range t.Exits {
			s[e] = true
		}
		return s
	}

	// Each cell is 4 chars wide: "[ 1]" … "[99]" or "< 1>" … "<99>".
	// East connector between cells: 1 char ("-" or " ").
	// South connector below a cell: "  | " (4 chars) or "    ".
	const cellW = 4

	// Determine whether any tile at the minimum x has a west exit with no in-zone
	// neighbor to the west (i.e., a cross-zone west exit stub is needed).
	hasWestStub := false
	if len(xs) > 0 {
		minX := xs[0]
		for _, y := range ys {
			t := byCoord[[2]int32{minX, y}]
			if t != nil && exitSet(t)["west"] {
				hasWestStub = true
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\r\n")

	for yi, y := range ys {
		// Room row
		for xi, x := range xs {
			// West stub: for the first column, emit "<" if the tile has a west exit
			// beyond the grid (cross-zone or undiscovered), or " " if the west-stub
			// column is active but this tile has no west exit.
			if xi == 0 && hasWestStub {
				t0 := byCoord[[2]int32{x, y}]
				if t0 != nil && exitSet(t0)["west"] {
					sb.WriteString("<")
				} else {
					sb.WriteString(" ")
				}
			}
			t := byCoord[[2]int32{x, y}]
			if t == nil {
				sb.WriteString("    ")
			} else {
				num := numByCoord[[2]int32{x, y}]
				color := DangerColor(t.DangerLevel)
				switch {
				case t.Current:
					sb.WriteString(fmt.Sprintf("%s<%2d>%s", color, num, ansiReset))
				case t.BossRoom:
					sb.WriteString(fmt.Sprintf("%s<BB>%s", color, ansiReset))
				default:
					sb.WriteString(fmt.Sprintf("%s[%2d]%s", color, num, ansiReset))
				}
			}
			// East connector:
			// "-" = discovered neighbor at next column same row
			// ">" = exit exists but no discovered neighbor (unexplored route)
			// " " = no east exit
			if xi < len(xs)-1 {
				nextX := xs[xi+1]
				tEast := byCoord[[2]int32{nextX, y}]
				if t != nil && exitSet(t)["east"] {
					if tEast != nil {
						sb.WriteString("-")
					} else {
						sb.WriteString(">")
					}
				} else {
					sb.WriteString(" ")
				}
			} else if t != nil && exitSet(t)["east"] {
				// Last column with east exit — show stub beyond the grid.
				sb.WriteString(">")
			}
		}
		sb.WriteString("\r\n")

		// POI suffix row (REQ-POI-6,7,8,9): only emit when at least one tile in this row has POIs.
		hasPOIs := false
		for _, x := range xs {
			if tile := byCoord[[2]int32{x, y}]; tile != nil && len(tile.Pois) > 0 {
				hasPOIs = true
				break
			}
		}
		if hasPOIs {
			for xi, x := range xs {
				// West stub padding for POI row.
				if xi == 0 && hasWestStub {
					sb.WriteString(" ")
				}
				tile := byCoord[[2]int32{x, y}]
				var cellPOIs []string
				if tile != nil {
					cellPOIs = tile.Pois
				}
				sb.WriteString(maputil.POISuffixRow(cellPOIs, cellW))
				if xi < len(xs)-1 {
					sb.WriteString(" ") // column separator (matches east connector width)
				}
			}
			sb.WriteString("\r\n")
		}

		// South connector row: emit whenever any room in this row has a south exit,
		// whether or not the neighbor is discovered.
		// "|" = discovered neighbor at next row same column
		// "." = exit exists but no discovered neighbor (unexplored route)
		if yi < len(ys)-1 {
			nextY := ys[yi+1]
			hasSouth := false
			for _, x := range xs {
				t := byCoord[[2]int32{x, y}]
				if t != nil && exitSet(t)["south"] {
					hasSouth = true
					break
				}
			}
			if hasSouth {
				for xi, x := range xs {
					// West stub padding for south connector row.
					if xi == 0 && hasWestStub {
						sb.WriteString(" ")
					}
					t := byCoord[[2]int32{x, y}]
					tSouth := byCoord[[2]int32{x, nextY}]
					if t != nil && exitSet(t)["south"] {
						if tSouth != nil {
							sb.WriteString("  | ")
						} else {
							sb.WriteString("  . ")
						}
					} else {
						sb.WriteString(strings.Repeat(" ", cellW))
					}
					if xi < len(xs)-1 {
						nextX := xs[xi+1]
						// Diagonal connector in separator between columns xi and xi+1.
						// Only meaningful when both axes step by exactly 2 (valid BFS diagonal).
						sep := " "
						if nextX-x == 2 && nextY-y == 2 {
							tNE := byCoord[[2]int32{x, nextY}]     // bottom-left of separator
							tSW := byCoord[[2]int32{nextX, y}]     // top-right of separator
							tSE := byCoord[[2]int32{nextX, nextY}] // bottom-right of separator
							hasFwd := (tNE != nil && exitSet(tNE)["northeast"]) ||
								(tSW != nil && exitSet(tSW)["southwest"])
							hasBack := (t != nil && exitSet(t)["southeast"]) ||
								(tSE != nil && exitSet(tSE)["northwest"])
							switch {
							case hasFwd && hasBack:
								sep = "X"
							case hasFwd:
								sep = "/"
							case hasBack:
								sep = "\\"
							}
						}
						sb.WriteString(sep)
					}
				}
				sb.WriteString("\r\n")
			}
		} else {
			// Last row: emit a trailing south-stub row if any room has a south exit.
			lastY := ys[len(ys)-1]
			hasSouthStub := false
			for _, x := range xs {
				t := byCoord[[2]int32{x, lastY}]
				if t != nil && exitSet(t)["south"] {
					hasSouthStub = true
					break
				}
			}
			if hasSouthStub {
				for xi, x := range xs {
					// West stub padding for trailing south-stub row.
					if xi == 0 && hasWestStub {
						sb.WriteString(" ")
					}
					t := byCoord[[2]int32{x, lastY}]
					if t != nil && exitSet(t)["south"] {
						sb.WriteString("  . ")
					} else {
						sb.WriteString(strings.Repeat(" ", cellW))
					}
					if xi < len(xs)-1 {
						sb.WriteString(" ")
					}
				}
				sb.WriteString("\r\n")
			}
		}
	}

	// ── Legend entries ────────────────────────────────────────────────────
	type legendEntry struct {
		num     int
		name    string
		current bool
	}
	var entries []legendEntry
	for _, y := range ys {
		for _, x := range xs {
			t := byCoord[[2]int32{x, y}]
			if t == nil {
				continue
			}
			entries = append(entries, legendEntry{num: len(entries) + 1, name: t.RoomName, current: t.Current})
		}
	}

	if width <= 0 {
		width = 80
	}

	// ── Two-column layout (width ≥ 100): grid left, legend right ──────────
	if width >= 100 {
		halfWidth := width / 2 // integer division; left gets the smaller half on odd widths

		// Split grid string into lines (strip trailing empty line from final \r\n).
		gridStr := sb.String()
		gridLines := strings.Split(strings.TrimRight(gridStr, "\r\n"), "\r\n")

		// Build legend lines — multi-column to compress height and fill available width.
		legendRightWidth := width - halfWidth - 3 // subtract " │ " separator
		if legendRightWidth < 1 {
			legendRightWidth = 1
		}
		const minLegendColWidth = 20
		numLegendCols := legendRightWidth / minLegendColWidth
		if numLegendCols < 1 {
			numLegendCols = 1
		}
		legendColWidth := legendRightWidth / numLegendCols
		legendNameWidth := legendColWidth - 5 // " *NN." prefix is 5 chars
		if legendNameWidth < 1 {
			legendNameWidth = 1
		}

		legendLines := []string{"Legend:"}
		// POI legend section in two-column layout.
		presentPOISet2 := make(map[string]bool)
		for _, tile := range resp.Tiles {
			for _, id := range tile.Pois {
				presentPOISet2[id] = true
			}
		}
		if len(presentPOISet2) > 0 {
			legendLines = append(legendLines, "Points of Interest")
			for _, pt := range maputil.POITypes {
				if presentPOISet2[pt.ID] {
					legendLines = append(legendLines, fmt.Sprintf("  %s%c\033[0m %s", pt.Color, pt.Symbol, pt.Label))
				}
			}
		}
		for i := 0; i < len(entries); i += numLegendCols {
			var row strings.Builder
			for col := 0; col < numLegendCols; col++ {
				idx := i + col
				if idx >= len(entries) {
					break
				}
				e := entries[idx]
				marker := " "
				if e.current {
					marker = "*"
				}
				cell := fmt.Sprintf("%s%2d.%-*s", marker, e.num, legendNameWidth, e.name)
				if len(cell) > legendColWidth {
					cell = cell[:legendColWidth]
				}
				row.WriteString(cell)
			}
			legendLines = append(legendLines, row.String())
		}

		// Zip grid and legend with │ separator.
		maxLen := len(gridLines)
		if len(legendLines) > maxLen {
			maxLen = len(legendLines)
		}
		var out strings.Builder
		for i := 0; i < maxLen; i++ {
			var gridPart, legendPart string
			if i < len(gridLines) {
				gridPart = gridLines[i]
			}
			if i < len(legendLines) {
				legendPart = legendLines[i]
			}
			// Pad grid part to halfWidth-3 visible chars.
			visLen := len(telnet.StripANSI(gridPart))
			targetW := halfWidth - 3
			if targetW < 0 {
				targetW = 0
			}
			if visLen < targetW {
				gridPart += strings.Repeat(" ", targetW-visLen)
			}
			out.WriteString(gridPart)
			if i < len(legendLines) {
				out.WriteString(" │ ")
				out.WriteString(legendPart)
			}
			out.WriteString("\r\n")
		}
		return out.String()
	}

	// ── Stacked layout (width < 100): current behavior ────────────────────
	const legendCols = 4
	colWidth := width / legendCols
	if colWidth < 22 {
		colWidth = 22
	}
	nameWidth := colWidth - 4 // " *NN." prefix is 4 chars
	sb.WriteString("\r\nLegend:\r\n")

	// POI legend section (REQ-POI-11..14): collect present POI types, emit in table order.
	presentPOISet := make(map[string]bool)
	for _, t := range resp.Tiles {
		for _, id := range t.Pois {
			presentPOISet[id] = true
		}
	}
	if len(presentPOISet) > 0 {
		sb.WriteString("Points of Interest\r\n")
		for _, pt := range maputil.POITypes {
			if presentPOISet[pt.ID] {
				sb.WriteString(fmt.Sprintf("  %s%c\033[0m %s\r\n", pt.Color, pt.Symbol, pt.Label))
			}
		}
	}

	for i := 0; i < len(entries); i += legendCols {
		for col := 0; col < legendCols; col++ {
			idx := i + col
			if idx >= len(entries) {
				break
			}
			e := entries[idx]
			marker := " "
			if e.current {
				marker = "*"
			}
			cell := fmt.Sprintf("%s%2d.%-*s", marker, e.num, nameWidth, e.name)
			if len(cell) > colWidth {
				cell = cell[:colWidth]
			}
			sb.WriteString(cell)
		}
		sb.WriteString("\r\n")
	}
	return sb.String()
}

func RenderCombatEvent(ce *gamev1.CombatEvent) string {
	switch ce.Type {
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK:
		switch ce.Outcome {
		case "critical success":
			// Crit hit: bright yellow with emphasis
			return telnet.Colorf(telnet.BrightYellow, "[Combat] %s", ce.Narrative)
		case "critical failure":
			// Crit miss: bright red with emphasis
			return telnet.Colorf(telnet.BrightRed, "[Combat] %s", ce.Narrative)
		default:
			if ce.Damage > 0 {
				return telnet.Colorf(telnet.Red, "[Combat] %s", ce.Narrative)
			}
			return telnet.Colorf(telnet.BrightWhite, "[Combat] %s", ce.Narrative)
		}
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH:
		return telnet.Colorf(telnet.Red, "[Combat] %s", ce.Narrative)
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE:
		return telnet.Colorf(telnet.Yellow, "[Combat] %s", ce.Narrative)
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_END:
		return telnet.Colorf(telnet.BrightYellow, "[Combat] %s", ce.Narrative)
	default:
		return telnet.Colorf(telnet.White, "[Combat] %s", ce.Narrative)
	}
}

// RenderSkillsResponse formats a SkillsResponse as colored telnet text.
// Skills are grouped by ability score. Trained skills are highlighted in cyan; untrained are dim.
//
// Precondition: sr must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderSkillsResponse(sr *gamev1.SkillsResponse) string {
	abilityOrder := []string{"quickness", "brutality", "reasoning", "savvy", "flair"}
	abilityLabel := map[string]string{
		"quickness": "Quickness",
		"brutality": "Brutality",
		"reasoning": "Reasoning",
		"savvy":     "Savvy",
		"flair":     "Flair",
	}

	byAbility := make(map[string][]*gamev1.SkillEntry)
	for _, sk := range sr.Skills {
		byAbility[sk.Ability] = append(byAbility[sk.Ability], sk)
	}

	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Skills ==="))
	sb.WriteString("\r\n")

	for _, ability := range abilityOrder {
		skills, ok := byAbility[ability]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("\r\n%s:\r\n", abilityLabel[ability]))
		for _, sk := range skills {
			name := fmt.Sprintf("  %-14s", sk.Name)
			bonus := fmt.Sprintf("+%d", sk.Bonus)
			rank := fmt.Sprintf("%-11s", sk.Proficiency)
			line := fmt.Sprintf("%s%s %s  %s", name, bonus, rank, sk.Description)
			if sk.Proficiency != "untrained" {
				sb.WriteString(telnet.Colorize(telnet.Cyan, line))
			} else {
				sb.WriteString(telnet.Colorize(telnet.Dim, line))
			}
			sb.WriteString("\r\n")
		}
	}
	return sb.String()
}

// RenderFeatsResponse formats a FeatsResponse as colored telnet text.
// Feats are grouped by category. Active feats are marked with [active].
//
// Precondition: fr must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderFeatsResponse(fr *gamev1.FeatsResponse) string {
	categoryOrder := []string{"general", "skill", "job"}
	categoryLabel := map[string]string{
		"general": "General",
		"skill":   "Skill",
		"job":     "Job",
	}

	byCategory := make(map[string][]*gamev1.FeatEntry)
	for _, f := range fr.Feats {
		byCategory[f.Category] = append(byCategory[f.Category], f)
	}

	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Feats ==="))
	sb.WriteString("\r\n")

	for _, cat := range categoryOrder {
		feats, ok := byCategory[cat]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("\r\n%s:\r\n", categoryLabel[cat]))
		for _, f := range feats {
			activeTag := ""
			if f.Active {
				activeTag = telnet.Colorize(telnet.BrightYellow, " [active]")
			}
			name := fmt.Sprintf("  %-20s", f.Name)
			sb.WriteString(telnet.Colorize(telnet.Cyan, name))
			sb.WriteString(activeTag)
			sb.WriteString(telnet.Colorize(telnet.Dim, " "+f.Description))
			sb.WriteString("\r\n")
		}
	}
	return sb.String()
}

// RenderClassFeaturesResponse formats a ClassFeaturesResponse for display.
//
// Precondition: resp must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderClassFeaturesResponse(resp *gamev1.ClassFeaturesResponse) string {
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightYellow, "=== Job Features ===\r\n"))

	if len(resp.ArchetypeFeatures) > 0 {
		sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nArchetype Features:\r\n"))
		for _, f := range resp.ArchetypeFeatures {
			activeTag := ""
			if f.Active {
				activeTag = telnet.Colorize(telnet.Green, " [active]")
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s%s\r\n    %s\r\n",
				telnet.BrightWhite, f.Name, telnet.Reset, activeTag, f.Description))
		}
	}

	if len(resp.JobFeatures) > 0 {
		sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nJob Features:\r\n"))
		for _, f := range resp.JobFeatures {
			activeTag := ""
			if f.Active {
				activeTag = telnet.Colorize(telnet.Green, " [active]")
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s%s\r\n    %s\r\n",
				telnet.BrightWhite, f.Name, telnet.Reset, activeTag, f.Description))
		}
	}

	if len(resp.ArchetypeFeatures) == 0 && len(resp.JobFeatures) == 0 {
		sb.WriteString(telnet.Colorize(telnet.Dim, "  No class features assigned.\r\n"))
	}
	return sb.String()
}

// RenderInteractResponse formats an InteractResponse as telnet text.
//
// Precondition: ir must be non-nil.
// Postcondition: Returns the message string from the response.
func RenderInteractResponse(ir *gamev1.InteractResponse) string {
	return ir.Message
}

// RenderUseResponse formats a UseResponse as telnet text.
// If Choices is non-empty, renders an interactive selection list.
// Otherwise renders the activation message.
//
// Precondition: ur must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderUseResponse(ur *gamev1.UseResponse) string {
	if ur.Message != "" {
		return ur.Message
	}
	if len(ur.Choices) == 0 {
		return telnet.Colorize(telnet.Yellow, "You have no active feats.")
	}
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "Active feats:\r\n"))
	for i, f := range ur.Choices {
		sb.WriteString(fmt.Sprintf("  %s%d%s. %s%-20s%s - %s%s%s\r\n",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, f.Name, telnet.Reset,
			telnet.Dim, f.Description, telnet.Reset))
	}
	sb.WriteString(telnet.Colorf(telnet.BrightWhite, "Type: use <feat name> to activate"))
	return sb.String()
}

// RenderProficienciesResponse formats a ProficienciesResponse as colored telnet text.
// Armor and weapon proficiencies are displayed in separate sections.
// Trained entries are highlighted in cyan; untrained entries are dim.
//
// Precondition: pr must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderProficienciesResponse(pr *gamev1.ProficienciesResponse) string {
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Proficiencies ===\r\n"))

	sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nArmor:\r\n"))
	for _, e := range pr.Proficiencies {
		if e.Kind != "armor" {
			continue
		}
		line := fmt.Sprintf("  %-18s %-12s +%d", e.Name, "["+e.Rank+"]", e.Bonus)
		if e.Rank != "untrained" {
			sb.WriteString(telnet.Colorize(telnet.Cyan, line))
		} else {
			sb.WriteString(telnet.Colorize(telnet.Dim, line))
		}
		sb.WriteString("\r\n")
	}

	sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nWeapons:\r\n"))
	for _, e := range pr.Proficiencies {
		if e.Kind != "weapon" {
			continue
		}
		line := fmt.Sprintf("  %-18s %-12s +%d", e.Name, "["+e.Rank+"]", e.Bonus)
		if e.Rank != "untrained" {
			sb.WriteString(telnet.Colorize(telnet.Cyan, line))
		} else {
			sb.WriteString(telnet.Colorize(telnet.Dim, line))
		}
		sb.WriteString("\r\n")
	}

	return sb.String()
}

// displayGender formats a stored gender value for display.
// Strips the "custom:" prefix and capitalizes the first letter.
// Returns the value unchanged if empty.
func displayGender(g string) string {
	if g == "" {
		return g
	}
	if strings.HasPrefix(g, "custom:") {
		g = g[len("custom:"):]
	}
	if len(g) == 0 {
		return g
	}
	return strings.ToUpper(g[:1]) + g[1:]
}

// RenderWorldMap formats a world map from MapResponse.WorldTiles into a colored
// terminal string using a grid-cell layout.
//
// Precondition: resp must be non-nil; width > 0.
// Postcondition: Returns a multi-line ANSI-colored string with legend.
// Layout: two-column (grid left, legend right) at width >= 100; single-column otherwise.
func RenderWorldMap(resp *gamev1.MapResponse, width int) string {
	tiles := resp.GetWorldTiles()
	if len(tiles) == 0 {
		return telnet.Colorize(telnet.BrightYellow, "World Map") + "\r\n" +
			telnet.Colorize(telnet.Dim, "(no zones discovered)") + "\r\n"
	}

	const cellStride = 5 // 4-char cell + 1-char connector slot
	const rowStride = 2  // cell row + connector row

	// Find coordinate bounds for normalization.
	minX, minY := tiles[0].WorldX, tiles[0].WorldY
	for _, t := range tiles {
		if t.WorldX < minX {
			minX = t.WorldX
		}
		if t.WorldY < minY {
			minY = t.WorldY
		}
	}

	// Sort tiles top-to-bottom, left-to-right for legend number assignment.
	sorted := make([]*gamev1.WorldZoneTile, len(tiles))
	copy(sorted, tiles)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].WorldY != sorted[j].WorldY {
			return sorted[i].WorldY < sorted[j].WorldY
		}
		return sorted[i].WorldX < sorted[j].WorldX
	})

	// Assign legend numbers and build a coord→tile index.
	type tileInfo struct {
		tile   *gamev1.WorldZoneTile
		legend int // 1-based
	}
	byCoord := make(map[[2]int32]*tileInfo, len(sorted))
	legendEntries := make([]string, 0, len(sorted))
	for i, t := range sorted {
		num := i + 1
		key := [2]int32{t.WorldX, t.WorldY}
		byCoord[key] = &tileInfo{tile: t, legend: num}
		label := t.ZoneName
		if !t.Discovered {
			label = "???"
		}
		legendEntries = append(legendEntries, fmt.Sprintf("%02d: %s", num, label))
	}

	// Compute grid dimensions.
	maxCol, maxRow := int32(0), int32(0)
	for _, t := range tiles {
		col := (t.WorldX - minX) / 2
		row := (t.WorldY - minY) / 2
		if col > maxCol {
			maxCol = col
		}
		if row > maxRow {
			maxRow = row
		}
	}
	gridCols := int(maxCol) + 1
	gridRows := int(maxRow) + 1

	// Build grid: rendered rows (cell rows + connector rows).
	renderedHeight := gridRows*rowStride - 1 // last row has no connector row below
	if renderedHeight < 1 {
		renderedHeight = 1
	}
	renderedWidth := gridCols * cellStride // each cell is 4 chars wide + 1 connector slot
	if renderedWidth < 4 {
		renderedWidth = 4
	}

	grid := make([][]byte, renderedHeight)
	for i := range grid {
		row := make([]byte, renderedWidth)
		for j := range row {
			row[j] = ' '
		}
		grid[i] = row
	}

	// Place cell bodies and connectors.
	type placedCell struct {
		gridRow, gridCol int
		info             *tileInfo
	}
	var placed []placedCell

	for _, ti := range byCoord {
		col := int((ti.tile.WorldX - minX) / 2)
		row := int((ti.tile.WorldY - minY) / 2)
		cellRow := row * rowStride
		cellCol := col * cellStride

		var cellBody string
		numStr := fmt.Sprintf("%02d", ti.legend)
		if ti.tile.Current {
			cellBody = "<" + numStr + ">"
		} else if ti.tile.Discovered {
			cellBody = "[" + numStr + "]"
		} else {
			cellBody = "[??]"
		}

		if cellRow < len(grid) && cellCol+3 < len(grid[cellRow]) {
			copy(grid[cellRow][cellCol:cellCol+4], []byte(cellBody))
		}

		placed = append(placed, placedCell{gridRow: cellRow, gridCol: cellCol, info: ti})
	}

	// Render east connectors (between cells differing by exactly 2 on X, 0 on Y).
	for _, a := range placed {
		for _, b := range placed {
			if a.info.tile.WorldY != b.info.tile.WorldY {
				continue
			}
			dx := b.info.tile.WorldX - a.info.tile.WorldX
			if dx != 2 {
				continue
			}
			connRow := a.gridRow
			connCol := a.gridCol + 4
			if connRow < len(grid) && connCol < len(grid[connRow]) {
				grid[connRow][connCol] = '-'
			}
		}
	}

	// Render south connectors (between cells differing by exactly 2 on Y, 0 on X).
	for _, a := range placed {
		for _, b := range placed {
			if a.info.tile.WorldX != b.info.tile.WorldX {
				continue
			}
			dy := b.info.tile.WorldY - a.info.tile.WorldY
			if dy != 2 {
				continue
			}
			connRow := a.gridRow + 1
			connCol := a.gridCol + 1
			if connRow < len(grid) && connCol < len(grid[connRow]) {
				grid[connRow][connCol] = '|'
			}
		}
	}

	// Build colored spans per row.
	type coloredSpan struct {
		col  int
		text string // 4-char colored body
	}
	rowSpans := make(map[int][]coloredSpan, len(placed))
	for _, p := range placed {
		var colored string
		numStr := fmt.Sprintf("%02d", p.info.legend)
		if p.info.tile.Current {
			raw := "<" + numStr + ">"
			colored = telnet.Colorize(telnet.BrightWhite, raw)
		} else if p.info.tile.Discovered {
			raw := "[" + numStr + "]"
			colored = DangerColor(p.info.tile.DangerLevel) + raw + ansiReset
		} else {
			raw := "[??]"
			colored = "\033[37m" + raw + ansiReset
		}
		rowSpans[p.gridRow] = append(rowSpans[p.gridRow], coloredSpan{col: p.gridCol, text: colored})
	}

	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightYellow, "World Map") + "\r\n")

	buildGridLines := func() []string {
		var lines []string
		for ri, rawRow := range grid {
			spans, hasSpans := rowSpans[ri]
			if !hasSpans {
				lines = append(lines, string(rawRow))
				continue
			}
			sort.Slice(spans, func(a, b int) bool { return spans[a].col < spans[b].col })
			var rowSB strings.Builder
			pos := 0
			for _, span := range spans {
				if span.col > pos {
					rowSB.Write(rawRow[pos:span.col])
				}
				rowSB.WriteString(span.text)
				pos = span.col + 4
			}
			if pos < len(rawRow) {
				rowSB.Write(rawRow[pos:])
			}
			lines = append(lines, rowSB.String())
		}
		return lines
	}

	gridLines := buildGridLines()

	if width >= 100 {
		// Two-column: grid on left, legend on right.
		for i, gl := range gridLines {
			sb.WriteString(gl)
			if i < len(legendEntries) {
				sb.WriteString("  ")
				sb.WriteString(telnet.Colorize(telnet.Cyan, legendEntries[i]))
			}
			sb.WriteString("\r\n")
		}
		for i := len(gridLines); i < len(legendEntries); i++ {
			sb.WriteString(telnet.Colorize(telnet.Cyan, legendEntries[i]))
			sb.WriteString("\r\n")
		}
	} else {
		// Single-column: grid then legend.
		for _, gl := range gridLines {
			sb.WriteString(gl)
			sb.WriteString("\r\n")
		}
		sb.WriteString("\r\n")
		for _, e := range legendEntries {
			sb.WriteString(telnet.Colorize(telnet.Cyan, e))
			sb.WriteString("\r\n")
		}
	}

	return sb.String()
}

// renderLoadoutView formats a LoadoutView as human-readable telnet text.
//
// Precondition: lv must not be nil.
// Postcondition: Returns a non-empty string with one line per preset.
func renderLoadoutView(lv *gamev1.LoadoutView) string {
	if lv == nil {
		return "No loadout data."
	}
	var sb strings.Builder
	sb.WriteString("Loadout Presets\r\n")
	for i, preset := range lv.Presets {
		label := fmt.Sprintf("  Preset %d", i+1)
		if int32(i) == lv.ActiveIndex {
			label += " [ACTIVE]"
		}
		label += ": "
		mainHand := preset.MainHand
		mainDmg := preset.MainHandDamage
		offHand := preset.OffHand
		offDmg := preset.OffHandDamage
		mainStr := "—"
		if mainHand != "" {
			mainStr = mainHand
			if mainDmg != "" {
				mainStr += " (" + mainDmg + ")"
			}
		}
		offStr := "—"
		if offHand != "" {
			offStr = offHand
			if offDmg != "" {
				offStr += " (" + offDmg + ")"
			}
		}
		sb.WriteString(label + "Main: " + mainStr + " | Off: " + offStr + "\r\n")
	}
	return strings.TrimRight(sb.String(), "\r\n")
}

// choicePromptPayload matches the JSON structure emitted by grpc_service.go
// inside the "\x00choice\x00" sentinel.
type choicePromptPayload struct {
	FeatureID string   `json:"featureId"`
	Prompt    string   `json:"prompt"`
	Options   []string `json:"options"`
}

// renderChoicePrompt formats a feature-choice prompt as human-readable telnet text.
//
// Precondition: payload must not be nil.
// Postcondition: Returns a non-empty string listing the prompt and numbered options.
func renderChoicePrompt(payload *choicePromptPayload) string {
	if payload == nil {
		return "No choice data."
	}
	var sb strings.Builder
	sb.WriteString(payload.Prompt)
	sb.WriteString("\r\n")
	for i, opt := range payload.Options {
		fmt.Fprintf(&sb, "  %d) %s\r\n", i+1, opt)
	}
	fmt.Fprintf(&sb, "Enter 1-%d:", len(payload.Options))
	return strings.TrimRight(sb.String(), "\r\n")
}


// RenderEffectsBlock is a thin forwarder to
// internal/game/effect/render.EffectsBlock. The underlying formatter was
// moved to that package so the gameserver (which has no business importing
// frontend/handlers) can populate CharacterSheetView.EffectsSummary with
// identical content. This wrapper preserves the original function name used
// by the telnet bridge and by the existing effects_render_test.go suite.
//
// Precondition / Postcondition: see effect/render.EffectsBlock.
func RenderEffectsBlock(es *effect.EffectSet, casterNames map[string]string, width int) string {
	return effectrender.EffectsBlock(es, casterNames, width)
}

// RenderDamageBreakdown renders the full verbose breakdown block (MULT-15).
// Only emitted to observers with ShowDamageBreakdown enabled.
//
// Delegates to combat.FormatBreakdownVerbose so the gameserver and frontend
// share a single canonical formatter without introducing a frontend-package
// dependency from the gameserver.
func RenderDamageBreakdown(steps []combat.DamageBreakdownStep) string {
	return combat.FormatBreakdownVerbose(steps)
}

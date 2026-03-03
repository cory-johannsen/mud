package handlers

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// RenderRoomView formats a RoomView as colored Telnet text.
func RenderRoomView(rv *gamev1.RoomView) string {
	var b strings.Builder

	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightYellow, rv.Title))
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.White, rv.Description))
	b.WriteString("\r\n")

	// Exits
	if len(rv.Exits) > 0 {
		b.WriteString(telnet.Colorize(telnet.Cyan, "Exits:"))
		b.WriteString("\r\n")
		for _, e := range rv.Exits {
			label := e.Direction
			if e.Locked {
				label += " (locked)"
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
	}

	// Other players
	if len(rv.Players) > 0 {
		b.WriteString(telnet.Colorf(telnet.Green, "Also here: %s", strings.Join(rv.Players, ", ")))
		b.WriteString("\r\n")
	}

	// NPCs present
	if len(rv.Npcs) > 0 {
		names := make([]string, 0, len(rv.Npcs))
		for _, n := range rv.Npcs {
			names = append(names, n.Name)
		}
		b.WriteString(telnet.Colorf(telnet.Yellow, "NPCs: %s", strings.Join(names, ", ")))
		b.WriteString("\r\n")
	}

	// Room equipment
	for _, eq := range rv.Equipment {
		flags := ""
		if eq.Immovable {
			flags += " [fixed]"
		}
		if eq.Usable {
			flags += " [usable]"
		}
		b.WriteString(fmt.Sprintf("  %s%s%s%s\r\n", telnet.Cyan, eq.Name, telnet.Reset, flags))
	}

	return b.String()
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
func RenderMessage(me *gamev1.MessageEvent) string {
	switch me.Type {
	case gamev1.MessageType_MESSAGE_TYPE_SAY:
		return telnet.Colorf(telnet.BrightWhite, "%s says: %s", me.Sender, me.Content)
	case gamev1.MessageType_MESSAGE_TYPE_EMOTE:
		return telnet.Colorf(telnet.Magenta, "%s %s", me.Sender, me.Content)
	default:
		return fmt.Sprintf("%s: %s", me.Sender, me.Content)
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
	sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", ci.Region, ci.Class, ci.Level))
	sb.WriteString(fmt.Sprintf("  HP: %d/%d\r\n", ci.CurrentHp, ci.MaxHp))
	return sb.String()
}

// abilityBonus formats an ability score as its modifier with the raw score in parentheses.
// e.g. score 14 → "+2 (14)", score 10 → "+0 (10)", score 8 → "-1 (8)"
func abilityBonus(score int32) string {
	mod := (score - 10) / 2
	if mod >= 0 {
		return fmt.Sprintf("+%d (%d)", mod, score)
	}
	return fmt.Sprintf("%d (%d)", mod, score)
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

// RenderCharacterSheet formats a CharacterSheetView as a detailed Telnet character sheet.
//
// Precondition: csv must be non-nil.
// Postcondition: Returns a non-empty human-readable multiline string.
func RenderCharacterSheet(csv *gamev1.CharacterSheetView) string {
	if csv == nil {
		return telnet.Colorize(telnet.Red, "No character sheet available.")
	}
	var b strings.Builder

	// Header
	b.WriteString(telnet.Colorf(telnet.BrightYellow, "=== %s ===", csv.GetName()))
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("Class: %s  Archetype: %s  Team: %s  Level: %d\r\n",
		csv.GetJob(), csv.GetArchetype(), csv.GetTeam(), csv.GetLevel()))
	b.WriteString(fmt.Sprintf("HP: %d / %d\r\n", csv.GetCurrentHp(), csv.GetMaxHp()))

	// Abilities
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Abilities ---"))
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("BRT: %s  GRT: %s  QCK: %s\r\n",
		abilityBonus(csv.GetBrutality()), abilityBonus(csv.GetGrit()), abilityBonus(csv.GetQuickness())))
	b.WriteString(fmt.Sprintf("RSN: %s  SAV: %s  FLR: %s\r\n",
		abilityBonus(csv.GetReasoning()), abilityBonus(csv.GetSavvy()), abilityBonus(csv.GetFlair())))

	// Defense
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Defense ---"))
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("AC Bonus: %d  Check Penalty: %d  Speed Penalty: %d\r\n",
		csv.GetAcBonus(), csv.GetCheckPenalty(), csv.GetSpeedPenalty()))

	// Weapons
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Weapons ---"))
	b.WriteString("\r\n")
	mainHand := csv.GetMainHand()
	if mainHand == "" {
		mainHand = "(none)"
	}
	offHand := csv.GetOffHand()
	if offHand == "" {
		offHand = "(none)"
	}
	b.WriteString(fmt.Sprintf("Main: %s\r\n", mainHand))
	b.WriteString(fmt.Sprintf("Off:  %s\r\n", offHand))

	// Armor
	if armor := csv.GetArmor(); len(armor) > 0 {
		b.WriteString("\r\n")
		b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Armor ---"))
		b.WriteString("\r\n")
		for slot, item := range armor {
			if item != "" {
				b.WriteString(fmt.Sprintf("%s: %s\r\n", formatSlotLabel(slot), item))
			}
		}
	}

	// Currency
	b.WriteString("\r\n")
	b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Currency ---"))
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("%s\r\n", csv.GetCurrency()))

	return b.String()
}

// RenderMap renders a hybrid ASCII grid + legend from a MapResponse.
//
// Precondition: resp may be nil or have no tiles.
// Postcondition: Returns a non-empty string safe for telnet display.
func RenderMap(resp *gamev1.MapResponse) string {
	if resp == nil || len(resp.Tiles) == 0 {
		return "No map data.\r\n"
	}

	// Find grid bounds
	minX, minY := resp.Tiles[0].X, resp.Tiles[0].Y
	maxX, maxY := resp.Tiles[0].X, resp.Tiles[0].Y
	for _, t := range resp.Tiles {
		if t.X < minX {
			minX = t.X
		}
		if t.Y < minY {
			minY = t.Y
		}
		if t.X > maxX {
			maxX = t.X
		}
		if t.Y > maxY {
			maxY = t.Y
		}
	}

	// Build lookup by (x,y)
	byCoord := make(map[[2]int32]*gamev1.MapTile)
	for i := range resp.Tiles {
		t := resp.Tiles[i]
		byCoord[[2]int32{t.X, t.Y}] = t
	}

	exitSet := func(t *gamev1.MapTile) map[string]bool {
		s := make(map[string]bool)
		for _, e := range t.Exits {
			s[e] = true
		}
		return s
	}

	var sb strings.Builder
	sb.WriteString("\r\n")

	for y := minY; y <= maxY; y++ {
		// Room row
		for x := minX; x <= maxX; x++ {
			t := byCoord[[2]int32{x, y}]
			if t == nil {
				sb.WriteString("   ")
			} else if t.Current {
				sb.WriteString("[@]")
			} else {
				sb.WriteString("[#]")
			}
			if x < maxX {
				east := byCoord[[2]int32{x, y}]
				if east != nil && exitSet(east)["east"] {
					sb.WriteString("-")
				} else {
					sb.WriteString(" ")
				}
			}
		}
		sb.WriteString("\r\n")

		// South connector row
		if y < maxY {
			for x := minX; x <= maxX; x++ {
				t := byCoord[[2]int32{x, y}]
				if t != nil && exitSet(t)["south"] {
					sb.WriteString(" | ")
				} else {
					sb.WriteString("   ")
				}
				if x < maxX {
					sb.WriteString(" ")
				}
			}
			sb.WriteString("\r\n")
		}
	}

	// Legend
	sb.WriteString("\r\nLegend:\r\n")
	for i, t := range resp.Tiles {
		marker := " "
		if t.Current {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  %s%2d. %s\r\n", marker, i+1, t.RoomName))
	}

	return sb.String()
}

func RenderCombatEvent(ce *gamev1.CombatEvent) string {
	switch ce.Type {
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK:
		color := telnet.BrightWhite
		if ce.Damage > 0 {
			color = telnet.BrightRed
		}
		return telnet.Colorf(color, "[Combat] %s", ce.Narrative)
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

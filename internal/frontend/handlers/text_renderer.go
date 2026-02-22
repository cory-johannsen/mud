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
		var dirs []string
		for _, e := range rv.Exits {
			dir := e.Direction
			if e.Locked {
				dir += " (locked)"
			}
			dirs = append(dirs, dir)
		}
		b.WriteString(telnet.Colorf(telnet.Cyan, "Exits: %s", strings.Join(dirs, ", ")))
		b.WriteString("\r\n")
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

// RenderPlayerList formats a PlayerList as Telnet text.
func RenderPlayerList(pl *gamev1.PlayerList) string {
	if len(pl.Players) == 0 {
		return telnet.Colorize(telnet.Dim, "Nobody else is here.")
	}
	return telnet.Colorf(telnet.Green, "Players here: %s", strings.Join(pl.Players, ", "))
}

// RenderExitList formats an ExitList as Telnet text.
func RenderExitList(el *gamev1.ExitList) string {
	if len(el.Exits) == 0 {
		return telnet.Colorize(telnet.Dim, "There are no obvious exits.")
	}
	var dirs []string
	for _, e := range el.Exits {
		dir := e.Direction
		if e.Locked {
			dir += telnet.Colorize(telnet.Red, " (locked)")
		}
		if e.Hidden {
			dir += telnet.Colorize(telnet.Dim, " (hidden)")
		}
		dirs = append(dirs, dir)
	}
	return telnet.Colorf(telnet.Cyan, "Exits: %s", strings.Join(dirs, ", "))
}

// RenderError formats an ErrorEvent as red Telnet text.
func RenderError(ee *gamev1.ErrorEvent) string {
	return telnet.Colorize(telnet.Red, ee.Message)
}

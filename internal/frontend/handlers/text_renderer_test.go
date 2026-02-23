package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestRenderRoomView(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "test_room",
		Title:       "Test Room",
		Description: "A dusty chamber.",
		Exits: []*gamev1.ExitInfo{
			{Direction: "north", TargetRoomId: "other"},
			{Direction: "east", TargetRoomId: "another", Locked: true},
		},
		Players: []string{"Bob"},
	}

	rendered := RenderRoomView(rv)
	stripped := telnet.StripANSI(rendered)

	assert.Contains(t, stripped, "Test Room")
	assert.Contains(t, stripped, "dusty chamber")
	assert.Contains(t, stripped, "north")
	assert.Contains(t, stripped, "east (locked)")
	assert.Contains(t, stripped, "Bob")
}

func TestRenderRoomView_NoExitsNoPlayers(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "empty",
		Title:       "Empty Room",
		Description: "Nothing here.",
	}

	rendered := RenderRoomView(rv)
	stripped := telnet.StripANSI(rendered)

	assert.Contains(t, stripped, "Empty Room")
	assert.NotContains(t, stripped, "Exits")
	assert.NotContains(t, stripped, "Also here")
}

func TestRenderMessage_Say(t *testing.T) {
	msg := &gamev1.MessageEvent{
		Sender:  "Alice",
		Content: "hello world",
		Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
	}
	stripped := telnet.StripANSI(RenderMessage(msg))
	assert.Contains(t, stripped, "Alice says: hello world")
}

func TestRenderMessage_Emote(t *testing.T) {
	msg := &gamev1.MessageEvent{
		Sender:  "Alice",
		Content: "waves",
		Type:    gamev1.MessageType_MESSAGE_TYPE_EMOTE,
	}
	stripped := telnet.StripANSI(RenderMessage(msg))
	assert.Contains(t, stripped, "Alice waves")
}

func TestRenderRoomEvent_Arrive(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player:    "Bob",
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
		Direction: "south",
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob arrived from the south")
}

func TestRenderRoomEvent_Depart(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player:    "Bob",
		Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
		Direction: "north",
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob left to the north")
}

func TestRenderRoomEvent_ArriveNoDirection(t *testing.T) {
	evt := &gamev1.RoomEvent{
		Player: "Bob",
		Type:   gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
	}
	stripped := telnet.StripANSI(RenderRoomEvent(evt))
	assert.Contains(t, stripped, "Bob has arrived")
}

func TestRenderPlayerList(t *testing.T) {
	pl := &gamev1.PlayerList{
		Players: []string{"Alice", "Bob"},
	}
	stripped := telnet.StripANSI(RenderPlayerList(pl))
	assert.Contains(t, stripped, "Alice")
	assert.Contains(t, stripped, "Bob")
}

func TestRenderPlayerList_Empty(t *testing.T) {
	pl := &gamev1.PlayerList{}
	stripped := telnet.StripANSI(RenderPlayerList(pl))
	assert.Contains(t, stripped, "Nobody else")
}

func TestRenderExitList(t *testing.T) {
	el := &gamev1.ExitList{
		Exits: []*gamev1.ExitInfo{
			{Direction: "north"},
			{Direction: "east", Locked: true},
		},
	}
	stripped := telnet.StripANSI(RenderExitList(el))
	assert.Contains(t, stripped, "north")
	assert.Contains(t, stripped, "east")
	assert.Contains(t, stripped, "(locked)")
}

func TestRenderExitList_Empty(t *testing.T) {
	el := &gamev1.ExitList{}
	stripped := telnet.StripANSI(RenderExitList(el))
	assert.Contains(t, stripped, "no obvious exits")
}

func TestRenderError(t *testing.T) {
	ee := &gamev1.ErrorEvent{Message: "something went wrong"}
	stripped := telnet.StripANSI(RenderError(ee))
	assert.Equal(t, "something went wrong", stripped)
}

func TestRenderRoundStartEvent(t *testing.T) {
	evt := &gamev1.RoundStartEvent{
		Round: 1, ActionsPerTurn: 3, DurationMs: 6000,
		TurnOrder: []string{"Alice", "Ganger"},
	}
	result := RenderRoundStartEvent(evt)
	assert.Contains(t, result, "Round 1")
	assert.Contains(t, result, "Actions: 3")
	assert.Contains(t, result, "6s")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Ganger")
}

func TestRenderRoundEndEvent(t *testing.T) {
	evt := &gamev1.RoundEndEvent{Round: 2}
	result := RenderRoundEndEvent(evt)
	assert.Contains(t, result, "Round 2")
	assert.Contains(t, result, "resolved")
}

func TestRenderConditionEvent_Applied(t *testing.T) {
	ce := &gamev1.ConditionEvent{
		TargetName:    "Alice",
		ConditionName: "Prone",
		ConditionId:   "prone",
		Stacks:        1,
		Applied:       true,
	}
	result := RenderConditionEvent(ce)
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Prone")
	assert.Contains(t, result, "CONDITION")
}

func TestRenderConditionEvent_Removed(t *testing.T) {
	ce := &gamev1.ConditionEvent{
		TargetName:    "Alice",
		ConditionName: "Frightened",
		ConditionId:   "frightened",
		Stacks:        0,
		Applied:       false,
	}
	result := RenderConditionEvent(ce)
	assert.Contains(t, result, "fades")
	assert.Contains(t, result, "Alice")
}

// TestProperty_RenderConditionEvent_Applied verifies that for any non-empty target name
// and condition name, RenderConditionEvent with Applied=true returns a non-empty string
// containing the target name.
func TestProperty_RenderConditionEvent_Applied(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		target := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "target")
		condition := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "condition")
		stacks := rapid.Int32Range(1, 10).Draw(rt, "stacks")

		ce := &gamev1.ConditionEvent{
			TargetName:    target,
			ConditionName: condition,
			ConditionId:   "test",
			Stacks:        stacks,
			Applied:       true,
		}
		result := RenderConditionEvent(ce)
		assert.NotEmpty(rt, result)
		assert.Contains(rt, telnet.StripANSI(result), target)
	})
}

// TestProperty_RenderConditionEvent_Removed verifies that for any non-empty target name
// and condition name, RenderConditionEvent with Applied=false returns a string containing "fades".
func TestProperty_RenderConditionEvent_Removed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		target := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "target")
		condition := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "condition")

		ce := &gamev1.ConditionEvent{
			TargetName:    target,
			ConditionName: condition,
			ConditionId:   "test",
			Stacks:        0,
			Applied:       false,
		}
		result := RenderConditionEvent(ce)
		assert.Contains(rt, telnet.StripANSI(result), "fades")
	})
}

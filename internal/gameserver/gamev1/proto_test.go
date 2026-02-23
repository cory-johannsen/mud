package gamev1_test

import (
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProto_RoundStartEvent_Roundtrip(t *testing.T) {
	orig := &gamev1.RoundStartEvent{
		Round: 3, ActionsPerTurn: 3, DurationMs: 6000,
		TurnOrder: []string{"Alice", "Ganger"},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.RoundStartEvent{}
	require.NoError(t, proto.Unmarshal(data, got))
	assert.Equal(t, orig.Round, got.Round)
	assert.Equal(t, orig.DurationMs, got.DurationMs)
	assert.Equal(t, orig.TurnOrder, got.TurnOrder)
	assert.Equal(t, orig.ActionsPerTurn, got.ActionsPerTurn)
}

func TestProto_PassRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.ClientMessage{}
	require.NoError(t, proto.Unmarshal(data, got))
	_, ok := got.Payload.(*gamev1.ClientMessage_Pass)
	assert.True(t, ok)
}

func TestProto_StrikeRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: "ganger"}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	got := &gamev1.ClientMessage{}
	require.NoError(t, proto.Unmarshal(data, got))
	strike, ok := got.Payload.(*gamev1.ClientMessage_Strike)
	require.True(t, ok)
	assert.Equal(t, "ganger", strike.Strike.Target)
}

func TestStatusRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		RequestId: "r1",
		Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.ClientMessage
	require.NoError(t, proto.Unmarshal(data, &got))
	_, ok := got.Payload.(*gamev1.ClientMessage_Status)
	assert.True(t, ok)
}

func TestConditionEvent_Roundtrip(t *testing.T) {
	orig := &gamev1.ConditionEvent{
		TargetUid:     "p1",
		TargetName:    "Alice",
		ConditionId:   "prone",
		ConditionName: "Prone",
		Stacks:        1,
		Applied:       true,
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.ConditionEvent
	require.NoError(t, proto.Unmarshal(data, &got))
	assert.Equal(t, orig.ConditionId, got.ConditionId)
	assert.Equal(t, orig.Applied, got.Applied)
	assert.Equal(t, orig.Stacks, got.Stacks)
}

func TestConditionInfo_InRoomView_Roundtrip(t *testing.T) {
	orig := &gamev1.RoomView{
		RoomId: "r1",
		Title:  "Test Room",
		ActiveConditions: []*gamev1.ConditionInfo{
			{Id: "prone", Name: "Prone", Stacks: 1, DurationRemaining: -1},
		},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.RoomView
	require.NoError(t, proto.Unmarshal(data, &got))
	require.Len(t, got.ActiveConditions, 1)
	assert.Equal(t, "prone", got.ActiveConditions[0].Id)
	assert.Equal(t, int32(-1), got.ActiveConditions[0].DurationRemaining)
}

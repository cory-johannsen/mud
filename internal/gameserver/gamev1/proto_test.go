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

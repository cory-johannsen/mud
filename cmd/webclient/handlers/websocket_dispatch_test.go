package handlers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestDispatchWSMessage_CommandText_Move(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "north"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-1", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	move := msg.GetMove()
	require.NotNil(t, move)
	assert.Equal(t, "north", move.Direction)
}

func TestDispatchWSMessage_CommandText_Say(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "say Hello world"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-2", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	say := msg.GetSay()
	require.NotNil(t, say)
	assert.Equal(t, "Hello world", say.Message)
}

func TestDispatchWSMessage_DirectProto_MoveRequest(t *testing.T) {
	env := handlers.WSMessageForTest("MoveRequest", map[string]string{"direction": "south"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-3", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "south", msg.GetMove().GetDirection())
}

func TestDispatchWSMessage_UnknownType_ReturnsError(t *testing.T) {
	env := handlers.WSMessageForTest("BogusRequest", map[string]string{})
	registry := command.DefaultRegistry()
	_, err := handlers.DispatchWSMessageForTest(env, "req-4", registry)
	assert.Error(t, err)
}

func TestDispatchWSMessage_CommandText_Stride_NoArgs(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "stride"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-5", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	stride := msg.GetStride()
	require.NotNil(t, stride)
	assert.Equal(t, "toward", stride.Direction)
}

func TestDispatchWSMessage_CommandText_Stride_Away(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "stride away"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-6", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	stride := msg.GetStride()
	require.NotNil(t, stride)
	assert.Equal(t, "away", stride.Direction)
}

func TestDispatchWSMessage_CommandText_Stride_Toward(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "stride toward"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-7", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	stride := msg.GetStride()
	require.NotNil(t, stride)
	assert.Equal(t, "toward", stride.Direction)
}

func TestDispatchWSMessage_CommandText_Stride_OtherArg(t *testing.T) {
	env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "stride forward"})
	registry := command.DefaultRegistry()
	msg, err := handlers.DispatchWSMessageForTest(env, "req-8", registry)
	require.NoError(t, err)
	require.NotNil(t, msg)
	stride := msg.GetStride()
	require.NotNil(t, stride)
	assert.Equal(t, "toward", stride.Direction)
}

// Uses rapid property-based testing (SWENG-5a).
func TestProperty_StrideDirection_NonAwayDefaultsToToward(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		arg := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_0123456789 "))).
			Filter(func(s string) bool { return s != "away" }).
			Draw(rt, "non_away_arg")
		text := "stride"
		if arg != "" {
			text = "stride " + arg
		}
		env := handlers.WSMessageForTest("CommandText", map[string]string{"text": text})
		registry := command.DefaultRegistry()
		msg, err := handlers.DispatchWSMessageForTest(env, "prop-stride", registry)
		if err != nil {
			rt.Fatalf("DispatchWSMessageForTest returned error: %v", err)
		}
		stride := msg.GetStride()
		if stride == nil {
			rt.Fatalf("expected StrideRequest, got nil")
		}
		if stride.Direction != "toward" {
			rt.Fatalf("expected Direction=toward for arg %q, got %q", arg, stride.Direction)
		}
	})
}

// ── serverEventInner: UseResponse ────────────────────────────────────────────

func TestServerEventInner_UseResponse_Message(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_UseResponse{
			UseResponse: &gamev1.UseResponse{Message: "You strike with Power Strike!"},
		},
	}
	inner, name := handlers.ServerEventInnerForTest(event)
	require.NotNil(t, inner, "UseResponse must not be dropped")
	assert.Equal(t, "UseResponse", name)
	ur, ok := inner.(*gamev1.UseResponse)
	require.True(t, ok, "inner must be *gamev1.UseResponse")
	assert.Equal(t, "You strike with Power Strike!", ur.GetMessage())
}

func TestServerEventInner_UseResponse_Choices(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_UseResponse{
			UseResponse: &gamev1.UseResponse{
				Choices: []*gamev1.FeatEntry{
					{FeatId: "power_strike", Name: "Power Strike"},
				},
			},
		},
	}
	inner, name := handlers.ServerEventInnerForTest(event)
	require.NotNil(t, inner, "UseResponse with choices must not be dropped")
	assert.Equal(t, "UseResponse", name)
}

// ── serverEventInner: Weather ────────────────────────────────────────────

func TestServerEventInner_Weather_BasicMessage(t *testing.T) {
	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Weather{
			Weather: &gamev1.WeatherEvent{WeatherName: "acid_rain", Active: true},
		},
	}
	inner, name := handlers.ServerEventInnerForTest(event)
	require.NotNil(t, inner, "WeatherEvent must not be dropped")
	assert.Equal(t, "WeatherEvent", name)
	we, ok := inner.(*gamev1.WeatherEvent)
	require.True(t, ok, "inner must be *gamev1.WeatherEvent")
	assert.Equal(t, "acid_rain", we.GetWeatherName())
	assert.True(t, we.GetActive())
}

// Uses rapid property-based testing (SWENG-5a).
func TestProperty_ServerEventInner_Weather_AlwaysReturnsWeatherEvent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringOf(rapid.Rune()).Draw(rt, "weather_name")
		active := rapid.Bool().Draw(rt, "active")
		event := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Weather{
				Weather: &gamev1.WeatherEvent{WeatherName: name, Active: active},
			},
		}
		inner, msgName := handlers.ServerEventInnerForTest(event)
		if inner == nil {
			rt.Fatalf("serverEventInner returned nil for Weather payload")
		}
		if msgName != "WeatherEvent" {
			rt.Fatalf("expected msgName=WeatherEvent, got %q", msgName)
		}
		we, ok := inner.(*gamev1.WeatherEvent)
		if !ok {
			rt.Fatalf("expected *gamev1.WeatherEvent, got %T", inner)
		}
		if we.WeatherName != name {
			rt.Fatalf("WeatherName mismatch: got %q, want %q", we.WeatherName, name)
		}
		if we.Active != active {
			rt.Fatalf("Active mismatch: got %v, want %v", we.Active, active)
		}
	})
}

// internal/client/feed/feed_test.go
package feed_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/client/feed"
	"github.com/cory-johannsen/mud/internal/client/render"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestFeed_DefaultCap(t *testing.T) {
	f := feed.New(0)
	// Append 501 events — oldest must be evicted leaving 500
	for i := 0; i < 501; i++ {
		f.Append(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "msg"},
			},
		})
	}
	assert.Equal(t, 500, len(f.Entries()))
}

func TestFeed_CustomCap(t *testing.T) {
	f := feed.New(10)
	for i := 0; i < 15; i++ {
		f.Append(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "x"},
			},
		})
	}
	assert.Equal(t, 10, len(f.Entries()))
}

func TestFeed_TokenAssignment(t *testing.T) {
	cases := []struct {
		event *gamev1.ServerEvent
		token render.ColorToken
	}{
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Sender: "Alice", Content: "hi"},
			}},
			render.ColorSpeech,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CombatEvent{
				CombatEvent: &gamev1.CombatEvent{Narrative: "You hit!"},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoundStart{
				RoundStart: &gamev1.RoundStartEvent{Round: 1},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoundEnd{
				RoundEnd: &gamev1.RoundEndEvent{Round: 1},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
				RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE},
			}},
			render.ColorRoomEvent,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "oops"},
			}},
			render.ColorError,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterInfo{
				CharacterInfo: &gamev1.CharacterInfo{Name: "Zara"},
			}},
			render.ColorStructured,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_InventoryView{
				InventoryView: &gamev1.InventoryView{},
			}},
			render.ColorStructured,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterSheet{
				CharacterSheet: &gamev1.CharacterSheetView{Name: "Zara"},
			}},
			render.ColorStructured,
		},
	}
	for _, tc := range cases {
		f := feed.New(10)
		f.Append(tc.event)
		entries := f.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, tc.token, entries[0].Token, "wrong token for event type")
	}
}

func TestFeed_TextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
		Error: &gamev1.ErrorEvent{Message: "not found"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "not found", entries[0].Text)
}

func TestFeed_CombatTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CombatEvent{
		CombatEvent: &gamev1.CombatEvent{Narrative: "You strike the goblin for 7 damage."},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "You strike the goblin for 7 damage.", entries[0].Text)
}

func TestFeed_MessageTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Message{
		Message: &gamev1.MessageEvent{Sender: "Alice", Content: "Hello!"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "Alice: Hello!", entries[0].Text)
}

func TestFeed_RoomEventTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
		RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE, Direction: "north"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "Bob arrives from the north.", entries[0].Text)
}

func TestFeed_RoomEventDepart(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
		RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART, Direction: "west"},
	}})
	entries := f.Entries()
	assert.Equal(t, "Bob leaves to the west.", entries[0].Text)
}

func TestFeed_Clear(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "x"}}})
	f.Clear()
	assert.Empty(t, f.Entries())
}

func TestFeed_EntriesOrder(t *testing.T) {
	f := feed.New(5)
	for i := 0; i < 3; i++ {
		f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
			Error: &gamev1.ErrorEvent{Message: string(rune('a' + i))},
		}})
		time.Sleep(time.Millisecond)
	}
	entries := f.Entries()
	require.Len(t, entries, 3)
	// oldest→newest
	assert.True(t, entries[0].Timestamp.Before(entries[1].Timestamp))
	assert.True(t, entries[1].Timestamp.Before(entries[2].Timestamp))
}

func TestFeed_GoRoutineSafe(t *testing.T) {
	f := feed.New(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "x"}}})
			_ = f.Entries()
		}()
	}
	wg.Wait()
}

func TestFeed_Property_CapEnforced(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 50).Draw(rt, "cap")
		n := rapid.IntRange(cap, cap*3).Draw(rt, "n")
		f := feed.New(cap)
		for i := 0; i < n; i++ {
			f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "x"},
			}})
		}
		got := len(f.Entries())
		if got != cap {
			rt.Fatalf("expected %d entries, got %d", cap, got)
		}
	})
}

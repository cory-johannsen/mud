// internal/client/feed/feed.go
package feed

import (
	"fmt"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/client/render"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const defaultCap = 500

// Entry is a single message accumulated in the feed.
type Entry struct {
	Timestamp time.Time
	Token     render.ColorToken
	Text      string // pre-extracted human-readable text
}

// Feed accumulates ServerEvent messages as Entry values, enforcing a cap.
// Goroutine-safe.
type Feed struct {
	mu    sync.Mutex
	buf   []Entry
	cap   int
	head  int
	count int
}

// New creates a Feed with the given cap. cap=0 uses the default (500).
func New(cap int) *Feed {
	if cap <= 0 {
		cap = defaultCap
	}
	return &Feed{buf: make([]Entry, cap), cap: cap}
}

// Append adds a ServerEvent to the feed. If the feed is at capacity, the oldest
// entry is evicted.
func (f *Feed) Append(ev *gamev1.ServerEvent) {
	entry := Entry{
		Timestamp: time.Now(),
		Token:     DefaultTokenFor(ev),
		Text:      extractText(ev),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := (f.head + f.count) % f.cap
	if f.count < f.cap {
		f.count++
	} else {
		f.head = (f.head + 1) % f.cap
	}
	f.buf[idx] = entry
}

// Entries returns a snapshot of all entries in oldest→newest order.
func (f *Feed) Entries() []Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Entry, f.count)
	for i := 0; i < f.count; i++ {
		out[i] = f.buf[(f.head+i)%f.cap]
	}
	return out
}

// Clear removes all entries from the feed.
func (f *Feed) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.head = 0
	f.count = 0
}

// DefaultTokenFor returns the default ColorToken for the given ServerEvent.
// Exported so clients can override individual assignments before building their mapper.
func DefaultTokenFor(ev *gamev1.ServerEvent) render.ColorToken {
	if ev == nil {
		return render.ColorSystem
	}
	switch ev.Payload.(type) {
	case *gamev1.ServerEvent_Message:
		return render.ColorSpeech
	case *gamev1.ServerEvent_CombatEvent, *gamev1.ServerEvent_RoundStart, *gamev1.ServerEvent_RoundEnd:
		return render.ColorCombat
	case *gamev1.ServerEvent_RoomEvent:
		return render.ColorRoomEvent
	case *gamev1.ServerEvent_Error:
		return render.ColorError
	case *gamev1.ServerEvent_CharacterInfo, *gamev1.ServerEvent_InventoryView, *gamev1.ServerEvent_CharacterSheet:
		return render.ColorStructured
	default:
		return render.ColorSystem
	}
}

func extractText(ev *gamev1.ServerEvent) string {
	if ev == nil {
		return ""
	}
	switch p := ev.Payload.(type) {
	case *gamev1.ServerEvent_Error:
		if p.Error != nil {
			return p.Error.GetMessage()
		}
	case *gamev1.ServerEvent_CombatEvent:
		if p.CombatEvent != nil {
			return p.CombatEvent.GetNarrative()
		}
	case *gamev1.ServerEvent_Message:
		if p.Message != nil {
			return fmt.Sprintf("%s: %s", p.Message.GetSender(), p.Message.GetContent())
		}
	case *gamev1.ServerEvent_RoomEvent:
		if p.RoomEvent != nil {
			re := p.RoomEvent
			switch re.GetType() {
			case gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE:
				return fmt.Sprintf("%s arrives from the %s.", re.GetPlayer(), re.GetDirection())
			case gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART:
				return fmt.Sprintf("%s leaves to the %s.", re.GetPlayer(), re.GetDirection())
			}
		}
	case *gamev1.ServerEvent_RoundStart:
		if p.RoundStart != nil {
			return fmt.Sprintf("Round %d begins. Turn order: %v", p.RoundStart.GetRound(), p.RoundStart.GetTurnOrder())
		}
	case *gamev1.ServerEvent_RoundEnd:
		if p.RoundEnd != nil {
			return fmt.Sprintf("Round %d ends.", p.RoundEnd.GetRound())
		}
	case *gamev1.ServerEvent_CharacterInfo:
		if p.CharacterInfo != nil {
			ci := p.CharacterInfo
			return fmt.Sprintf("%s (Lv %d) — HP %d/%d", ci.GetName(), ci.GetLevel(), ci.GetCurrentHp(), ci.GetMaxHp())
		}
	case *gamev1.ServerEvent_CharacterSheet:
		if p.CharacterSheet != nil {
			cs := p.CharacterSheet
			return fmt.Sprintf("%s (Lv %d) — HP %d/%d", cs.GetName(), cs.GetLevel(), cs.GetCurrentHp(), cs.GetMaxHp())
		}
	case *gamev1.ServerEvent_InventoryView:
		return "[Inventory]"
	}
	return ""
}

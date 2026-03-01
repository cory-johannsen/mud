package gameserver_test

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestGameHour_Period(t *testing.T) {
	cases := []struct {
		hour   int32
		period gameserver.TimePeriod
	}{
		{0, gameserver.PeriodMidnight},
		{1, gameserver.PeriodLateNight},
		{4, gameserver.PeriodLateNight},
		{5, gameserver.PeriodDawn},
		{6, gameserver.PeriodDawn},
		{7, gameserver.PeriodMorning},
		{11, gameserver.PeriodMorning},
		{12, gameserver.PeriodAfternoon},
		{16, gameserver.PeriodAfternoon},
		{17, gameserver.PeriodDusk},
		{18, gameserver.PeriodDusk},
		{19, gameserver.PeriodEvening},
		{21, gameserver.PeriodEvening},
		{22, gameserver.PeriodNight},
		{23, gameserver.PeriodNight},
	}
	for _, tc := range cases {
		gh := gameserver.GameHour(tc.hour)
		if got := gh.Period(); got != tc.period {
			t.Errorf("hour %d: got %q, want %q", tc.hour, got, tc.period)
		}
	}
}

func TestGameHour_String(t *testing.T) {
	if got := gameserver.GameHour(6).String(); got != "06:00" {
		t.Errorf("got %q, want 06:00", got)
	}
	if got := gameserver.GameHour(18).String(); got != "18:00" {
		t.Errorf("got %q, want 18:00", got)
	}
}

func TestProperty_GameHour_PeriodAlwaysValid(t *testing.T) {
	valid := map[gameserver.TimePeriod]bool{
		gameserver.PeriodMidnight:  true,
		gameserver.PeriodLateNight: true,
		gameserver.PeriodDawn:      true,
		gameserver.PeriodMorning:   true,
		gameserver.PeriodAfternoon: true,
		gameserver.PeriodDusk:      true,
		gameserver.PeriodEvening:   true,
		gameserver.PeriodNight:     true,
	}
	rapid.Check(t, func(t *rapid.T) {
		h := rapid.Int32Range(0, 23).Draw(t, "hour")
		p := gameserver.GameHour(h).Period()
		if !valid[p] {
			t.Fatalf("hour %d returned invalid period %q", h, p)
		}
	})
}

func TestGameClock_AdvancesHour(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()
	defer clk.Unsubscribe(ch)

	// Wait for exactly 2 ticks
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for tick %d", i+1)
		}
	}

	h := clk.CurrentHour()
	if h != 2 {
		t.Errorf("expected hour 2 after 2 ticks from 0, got %d", h)
	}
}

func TestGameClock_Wraps(t *testing.T) {
	clk := gameserver.NewGameClock(23, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()
	defer clk.Unsubscribe(ch)

	// Wait for exactly 1 tick — from 23 should wrap to 0
	select {
	case h := <-ch:
		if h != 0 {
			t.Errorf("expected hour 0 after wrapping from 23, got %d", h)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for wrap tick")
	}
}

func TestGameClock_SubscribeReceivesTick(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()
	defer clk.Unsubscribe(ch)

	select {
	case <-ch:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for tick")
	}
}

func TestGameClock_UnsubscribeStopsDelivery(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()

	// Wait for at least one tick
	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no initial tick")
	}

	clk.Unsubscribe(ch)
	// Drain any buffered tick
	for len(ch) > 0 {
		<-ch
	}

	// Wait long enough for more ticks — none should arrive
	time.Sleep(100 * time.Millisecond)
	if len(ch) > 0 {
		t.Error("received tick after unsubscribe")
	}
}

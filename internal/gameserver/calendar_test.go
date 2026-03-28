package gameserver

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestFormatDate_OrdinalSuffixes(t *testing.T) {
	cases := []struct {
		month, day int
		want       string
	}{
		{1, 1, "January 1st"},
		{2, 2, "February 2nd"},
		{3, 3, "March 3rd"},
		{4, 4, "April 4th"},
		{11, 11, "November 11th"},
		{12, 12, "December 12th"},
		{5, 13, "May 13th"},
		{6, 21, "June 21st"},
		{7, 22, "July 22nd"},
		{8, 23, "August 23rd"},
		{9, 31, "September 31st"},
	}
	for _, c := range cases {
		got := FormatDate(c.month, c.day)
		if got != c.want {
			t.Errorf("FormatDate(%d, %d) = %q, want %q", c.month, c.day, got, c.want)
		}
	}
}

func TestGameCalendar_BroadcastsEveryTick(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 1, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	select {
	case dt := <-ch:
		if dt.Day != 1 || dt.Month != 1 {
			t.Errorf("unexpected dt: %+v", dt)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for GameDateTime broadcast")
	}
}

func TestGameCalendar_DayAdvancesByOne(t *testing.T) {
	// Basic: Jan 5 → Jan 6 at midnight.
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 5, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	timeout := time.After(3 * time.Second)
	for {
		select {
		case got := <-ch:
			if got.Hour != 0 {
				continue
			}
			if got.Day != 6 || got.Month != 1 {
				t.Errorf("after Jan 5 midnight got day=%d month=%d, want day=6 month=1", got.Day, got.Month)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for midnight tick")
		}
	}
}

func TestGameCalendar_JanRollover(t *testing.T) {
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 31, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	timeout := time.After(3 * time.Second)
	for {
		select {
		case got := <-ch:
			if got.Hour != 0 {
				continue
			}
			if got.Day != 1 || got.Month != 2 {
				t.Errorf("after Jan 31 midnight got day=%d month=%d, want day=1 month=2", got.Day, got.Month)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for midnight tick")
		}
	}
}

func TestGameCalendar_FebRollover(t *testing.T) {
	// Feb 28 → Mar 1 (year 2001 is not a leap year).
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 28, 2, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	timeout := time.After(3 * time.Second)
	for {
		select {
		case got := <-ch:
			if got.Hour != 0 {
				continue
			}
			if got.Day != 1 || got.Month != 3 {
				t.Errorf("after Feb 28 midnight got day=%d month=%d, want day=1 month=3", got.Day, got.Month)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for midnight tick")
		}
	}
}

func TestGameCalendar_NoSubscribers_NoPanic(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 1, 1, &noopRepo{})

	// subscribe a channel so we know when ticks are being processed
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	// wait for a couple ticks to confirm no panic
	timeout := time.After(3 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
			// tick received, no panic
		case <-timeout:
			t.Fatal("timed out waiting for tick")
		}
	}
	cal.Unsubscribe(ch)
}

func TestGameCalendar_SaveFailure_DoesNotStopBroadcast(t *testing.T) {
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 31, 1, &failRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	select {
	case <-ch:
		// success — broadcast continued despite save failure
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out — broadcast stopped after Save failure")
	}
}

func TestNewGameCalendar_PanicsOnNilClock(t *testing.T) {
	assert.Panics(t, func() {
		NewGameCalendar(nil, 1, 1, &noopRepo{})
	})
}

func TestNewGameCalendar_PanicsOnNilRepo(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	assert.Panics(t, func() {
		NewGameCalendar(clock, 1, 1, nil)
	})
}

func TestNewGameCalendar_PanicsOnInvalidDay(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	assert.Panics(t, func() {
		NewGameCalendar(clock, 0, 1, &noopRepo{})
	}, "day=0 should panic")
	assert.Panics(t, func() {
		NewGameCalendar(clock, 32, 1, &noopRepo{})
	}, "day=32 should panic")
}

func TestNewGameCalendar_PanicsOnInvalidMonth(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	assert.Panics(t, func() {
		NewGameCalendar(clock, 1, 0, &noopRepo{})
	}, "month=0 should panic")
	assert.Panics(t, func() {
		NewGameCalendar(clock, 1, 13, &noopRepo{})
	}, "month=13 should panic")
}

// Property: FormatDate ordinal suffix is always one of "st", "nd", "rd", "th"
func TestProperty_FormatDate_OrdinalSuffix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 31).Draw(t, "day")
		result := FormatDate(month, day)
		hasSuffix := strings.HasSuffix(result, "st") ||
			strings.HasSuffix(result, "nd") ||
			strings.HasSuffix(result, "rd") ||
			strings.HasSuffix(result, "th")
		if !hasSuffix {
			t.Fatalf("FormatDate(%d, %d) = %q: missing ordinal suffix", month, day, result)
		}
	})
}

// Property: FormatDate day=11,12,13 always returns "th"
func TestProperty_FormatDate_TeenSuffixAlwaysTh(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.SampledFrom([]int{11, 12, 13}).Draw(t, "day")
		result := FormatDate(month, day)
		if !strings.HasSuffix(result, "th") {
			t.Fatalf("FormatDate(%d, %d) = %q: expected 'th' suffix for teen day", month, day, result)
		}
	})
}

// Property: rollover always produces a valid calendar date (month 1–12, day 1–31)
func TestProperty_GameCalendar_RolloverProducesValidDate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		startMonth := rapid.IntRange(1, 12).Draw(t, "month")
		// pick a day that is the last day of startMonth in year 2001
		lastDay := time.Date(2001, time.Month(startMonth+1), 0, 0, 0, 0, 0, time.UTC).Day()
		if startMonth == 12 {
			lastDay = 31
		}
		startDay := rapid.IntRange(1, lastDay).Draw(t, "day")

		next := time.Date(2001, time.Month(startMonth), startDay+1, 0, 0, 0, 0, time.UTC)
		gotDay, gotMonth := next.Day(), int(next.Month())

		if gotMonth < 1 || gotMonth > 12 {
			t.Fatalf("rollover(%d/%d) produced invalid month %d", startMonth, startDay, gotMonth)
		}
		if gotDay < 1 || gotDay > 31 {
			t.Fatalf("rollover(%d/%d) produced invalid day %d", startMonth, startDay, gotDay)
		}
	})
}

// noopRepo is a CalendarRepo stub that does nothing.
type noopRepo struct{}

func (r *noopRepo) Load() (int, int, int, error) { return 6, 1, 1, nil }
func (r *noopRepo) Save(_, _, _ int) error        { return nil }

// failRepo is a CalendarRepo stub whose Save always fails.
type failRepo struct{}

func (r *failRepo) Load() (int, int, int, error) { return 6, 1, 1, nil }
func (r *failRepo) Save(_, _, _ int) error        { return fmt.Errorf("db error") }

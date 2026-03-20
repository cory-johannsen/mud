package gameserver

import (
	"fmt"
	"testing"
	"time"
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

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 6 || got.Month != 1 {
		t.Errorf("after Jan 5 midnight got day=%d month=%d, want day=6 month=1", got.Day, got.Month)
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

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 1 || got.Month != 2 {
		t.Errorf("after Jan 31 midnight got day=%d month=%d, want day=1 month=2", got.Day, got.Month)
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

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 1 || got.Month != 3 {
		t.Errorf("after Feb 28 midnight got day=%d month=%d, want day=1 month=3", got.Day, got.Month)
	}
}

func TestGameCalendar_NoSubscribers_NoPanic(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 1, 1, &noopRepo{})
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()
	time.Sleep(120 * time.Millisecond)
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

// noopRepo is a CalendarRepo stub that does nothing.
type noopRepo struct{}

func (r *noopRepo) Load() (int, int, error) { return 1, 1, nil }
func (r *noopRepo) Save(_, _ int) error     { return nil }

// failRepo is a CalendarRepo stub whose Save always fails.
type failRepo struct{}

func (r *failRepo) Load() (int, int, error) { return 1, 1, nil }
func (r *failRepo) Save(_, _ int) error     { return fmt.Errorf("db error") }

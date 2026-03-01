package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestFlavorText_IndoorAlwaysEmpty(t *testing.T) {
	periods := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight,
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk,
		gameserver.PeriodEvening, gameserver.PeriodNight,
	}
	for _, p := range periods {
		if got := gameserver.FlavorText(p, false); got != "" {
			t.Errorf("period %q indoor: expected empty, got %q", p, got)
		}
	}
}

func TestFlavorText_OutdoorNonEmpty(t *testing.T) {
	periods := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight,
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk,
		gameserver.PeriodEvening, gameserver.PeriodNight,
	}
	for _, p := range periods {
		if got := gameserver.FlavorText(p, true); got == "" {
			t.Errorf("period %q outdoor: expected non-empty flavor text", p)
		}
	}
}

func TestIsDarkPeriod(t *testing.T) {
	dark := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight, gameserver.PeriodNight,
	}
	light := []gameserver.TimePeriod{
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk, gameserver.PeriodEvening,
	}
	for _, p := range dark {
		if !gameserver.IsDarkPeriod(p) {
			t.Errorf("expected %q to be dark", p)
		}
	}
	for _, p := range light {
		if gameserver.IsDarkPeriod(p) {
			t.Errorf("expected %q to be light", p)
		}
	}
}

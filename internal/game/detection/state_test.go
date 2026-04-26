package detection_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/detection"
)

func TestState_Stringers(t *testing.T) {
	cases := map[detection.State]string{
		detection.Observed:   "observed",
		detection.Concealed:  "concealed",
		detection.Hidden:     "hidden",
		detection.Undetected: "undetected",
		detection.Unnoticed:  "unnoticed",
		detection.Invisible:  "invisible",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

func TestState_MissChancePercent(t *testing.T) {
	cases := map[detection.State]int{
		detection.Observed:   0,
		detection.Concealed:  20,
		detection.Hidden:     50,
		detection.Undetected: 50,
		detection.Unnoticed:  100,
		detection.Invisible:  50,
	}
	for s, want := range cases {
		if got := s.MissChancePercent(); got != want {
			t.Errorf("State(%s).MissChancePercent() = %d, want %d", s, got, want)
		}
	}
}

func TestState_ParseState(t *testing.T) {
	for _, s := range []detection.State{detection.Observed, detection.Concealed, detection.Hidden, detection.Undetected, detection.Unnoticed, detection.Invisible} {
		got, ok := detection.ParseState(s.String())
		if !ok || got != s {
			t.Errorf("ParseState(%q) = %v, %v; want %v, true", s.String(), got, ok, s)
		}
	}
	if _, ok := detection.ParseState("nonsense"); ok {
		t.Errorf("ParseState(nonsense) should be !ok")
	}
}

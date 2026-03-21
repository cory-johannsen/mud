package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

func TestDangerColor(t *testing.T) {
	cases := []struct {
		level      string
		wantPrefix string
	}{
		{"safe", "\033[32m"},
		{"sketchy", "\033[33m"},
		{"dangerous", "\033[38;5;208m"},
		{"all_out_war", "\033[31m"},
		{"", "\033[37m"},
		{"unknown", "\033[37m"},
	}
	for _, c := range cases {
		got := handlers.DangerColor(c.level)
		if got != c.wantPrefix {
			t.Errorf("DangerColor(%q) = %q, want %q", c.level, got, c.wantPrefix)
		}
	}
}

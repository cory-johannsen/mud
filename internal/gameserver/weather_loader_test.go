package gameserver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestSeasonForMonth(t *testing.T) {
	cases := []struct {
		month  int
		season string
	}{
		{1, "winter"}, {2, "winter"}, {12, "winter"},
		{3, "spring"}, {4, "spring"}, {5, "spring"},
		{6, "summer"}, {7, "summer"}, {8, "summer"},
		{9, "fall"}, {10, "fall"}, {11, "fall"},
	}
	for _, tc := range cases {
		got := gameserver.SeasonForMonth(tc.month)
		if got != tc.season {
			t.Errorf("SeasonForMonth(%d) = %q, want %q", tc.month, got, tc.season)
		}
	}
}

func TestLoadWeatherTypes_Valid(t *testing.T) {
	content := `
types:
  - id: rain
    name: Rain
    announce: "It rains."
    end_announce: "Rain stopped."
    seasons: [spring, fall]
    weight: 5
    conditions: [reduced_visibility]
`
	path := filepath.Join(t.TempDir(), "weather.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	types, err := gameserver.LoadWeatherTypes(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if types[0].ID != "rain" {
		t.Errorf("expected id=rain, got %q", types[0].ID)
	}
}

func TestLoadWeatherTypes_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weather.yaml")
	if err := os.WriteFile(path, []byte("types: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := gameserver.LoadWeatherTypes(path)
	if err == nil {
		t.Fatal("expected error for empty types, got nil")
	}
}

package danger_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
)

func TestEffectiveDangerLevel(t *testing.T) {
	tests := []struct {
		name       string
		zoneDanger string
		roomDanger string
		want       danger.DangerLevel
	}{
		{
			name:       "zone only",
			zoneDanger: "dangerous",
			roomDanger: "",
			want:       danger.Dangerous,
		},
		{
			name:       "room overrides zone",
			zoneDanger: "dangerous",
			roomDanger: "safe",
			want:       danger.Safe,
		},
		{
			name:       "both empty",
			zoneDanger: "",
			roomDanger: "",
			want:       danger.DangerLevel(""),
		},
		{
			name:       "room empty zone set",
			zoneDanger: "all_out_war",
			roomDanger: "",
			want:       danger.AllOutWar,
		},
		{
			name:       "room set zone empty",
			zoneDanger: "",
			roomDanger: "sketchy",
			want:       danger.Sketchy,
		},
		{
			name:       "invalid zone value passes through",
			zoneDanger: "invalid_level",
			roomDanger: "",
			want:       danger.DangerLevel("invalid_level"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := danger.EffectiveDangerLevel(tc.zoneDanger, tc.roomDanger)
			if got != tc.want {
				t.Errorf("EffectiveDangerLevel(%q, %q) = %q; want %q", tc.zoneDanger, tc.roomDanger, got, tc.want)
			}
		})
	}
}

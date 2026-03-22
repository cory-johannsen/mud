package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/maputil"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestNpcRoleToPOIIDIntegration verifies that NpcRoleToPOIID produces the
// correct POI ID for each NPC type used by handleMap POI population.
func TestNpcRoleToPOIIDIntegration(t *testing.T) {
	cases := []struct {
		npcRole string
		want    string
	}{
		{"merchant", "merchant"},
		{"healer", "healer"},
		{"job_trainer", "trainer"},
		{"guard", "guard"},
		{"quest_giver", "npc"},
		{"", ""},
	}
	for _, tc := range cases {
		inst := &npc.Instance{}
		inst.NpcRole = tc.npcRole
		got := maputil.NpcRoleToPOIID(inst.NpcRole)
		if got != tc.want {
			t.Errorf("NpcRoleToPOIID(%q) = %q, want %q", tc.npcRole, got, tc.want)
		}
	}
}

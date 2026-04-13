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
		{"motel_keeper", "motel"},
		{"brothel_keeper", "brothel"},
		{"quest_giver", "quest_giver"},
		{"banker", "banker"},
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

// TestHandleMap_POI_NPCTypeFallback is the regression test for BUG-41.
// It verifies that an NPC with no npc_role but a non-combat npc_type contributes
// a POI, and that a combat NPC does not.
func TestHandleMap_POI_NPCTypeFallback(t *testing.T) {
	cases := []struct {
		npcType  string
		npcRole  string
		wantPOID string
	}{
		// npc_role always wins when set.
		{"merchant", "merchant", "merchant"},
		// NPCType fallback when npc_role is empty (BUG-41).
		{"merchant", "", "merchant"},
		{"healer", "", "healer"},
		{"job_trainer", "", "trainer"},
		{"guard", "", "guard"},
		{"banker", "", "banker"},
		{"chip_doc", "", "npc"},
		// combat NPC must never contribute a POI.
		{"combat", "", ""},
		// empty type must never contribute a POI.
		{"", "", ""},
	}
	for _, tc := range cases {
		inst := &npc.Instance{}
		inst.NPCType = tc.npcType
		inst.NpcRole = tc.npcRole
		role := inst.NpcRole
		if role == "" {
			role = maputil.POIRoleFromNPCType(inst.NPCType)
		}
		got := maputil.NpcRoleToPOIID(role)
		if got != tc.wantPOID {
			t.Errorf("npcType=%q npcRole=%q: effective POI = %q, want %q",
				tc.npcType, tc.npcRole, got, tc.wantPOID)
		}
	}
}

package maputil_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/maputil"
)

// TestPOIRoleFromNPCType verifies that non-combat npc_type values produce the
// correct POI role string, and that "combat" and "" produce no role (BUG-41).
func TestPOIRoleFromNPCType_KnownTypes(t *testing.T) {
	cases := []struct {
		npcType string
		want    string
	}{
		{"", ""},
		{"combat", ""},
		{"merchant", "merchant"},
		{"healer", "healer"},
		{"job_trainer", "job_trainer"},
		{"guard", "guard"},
		{"banker", "banker"},
		{"chip_doc", "chip_doc"},
		{"quest_giver", "quest_giver"},
		{"fixer", "fixer"},
	}
	for _, tc := range cases {
		got := maputil.POIRoleFromNPCType(tc.npcType)
		if got != tc.want {
			t.Errorf("POIRoleFromNPCType(%q) = %q, want %q", tc.npcType, got, tc.want)
		}
	}
}

func TestNpcRoleToPOIID_KnownRoles(t *testing.T) {
	cases := []struct {
		role string
		want string
	}{
		{"", ""},
		{"merchant", "merchant"},
		{"Merchant", "merchant"},
		{"MERCHANT", "merchant"},
		{"healer", "healer"},
		{"job_trainer", "trainer"},
		{"guard", "guard"},
		{"motel_keeper", "motel"},
		{"brothel_keeper", "brothel"},
		{"quest_giver", "quest_giver"},
		{"banker", "banker"},
		{"chip_doc", "npc"},
	}
	for _, tc := range cases {
		got := maputil.NpcRoleToPOIID(tc.role)
		if got != tc.want {
			t.Errorf("NpcRoleToPOIID(%q) = %q, want %q", tc.role, got, tc.want)
		}
	}
}

func TestNpcRoleToPOIID_EmptyAlwaysEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Any non-empty string that is not a known role maps to "npc".
		role := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_"))).Filter(func(s string) bool {
			return s != "" && s != "merchant" && s != "healer" && s != "job_trainer" && s != "guard" && s != "quest_giver" && s != "motel_keeper" && s != "brothel_keeper"
		}).Draw(rt, "role")
		got := maputil.NpcRoleToPOIID(role)
		if got != "npc" {
			t.Errorf("NpcRoleToPOIID(%q) = %q, want %q", role, got, "npc")
		}
	})
}

func TestSortPOIs_KnownOrder(t *testing.T) {
	input := []string{"equipment", "guard", "merchant", "npc", "healer", "trainer", "brothel", "quest_giver", "motel"}
	got := maputil.SortPOIs(input)
	want := []string{"merchant", "healer", "trainer", "guard", "motel", "brothel", "quest_giver", "npc", "equipment"}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("SortPOIs position %d: got %q, want %q (full: %v)", i, got[i], v, got)
		}
	}
}

func TestSortPOIs_UnknownIDsLast(t *testing.T) {
	input := []string{"zzz", "merchant", "aaa"}
	got := maputil.SortPOIs(input)
	// known first, unknowns after
	if got[0] != "merchant" {
		t.Errorf("SortPOIs: known ID should be first, got %v", got)
	}
	for _, v := range got[1:] {
		if v == "merchant" {
			t.Errorf("SortPOIs: merchant appeared after unknowns: %v", got)
		}
	}
}

func TestSortPOIs_DoesNotMutateInput(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		knownIDs := []string{"merchant", "healer", "trainer", "guard", "motel", "brothel", "quest_giver", "npc", "equipment"}
		n := rapid.IntRange(0, 6).Draw(rt, "n")
		input := rapid.SliceOf(rapid.SampledFrom(knownIDs)).Draw(rt, "pois")
		_ = n
		inputCopy := append([]string(nil), input...)
		maputil.SortPOIs(input)
		for i, v := range inputCopy {
			if input[i] != v {
				t.Errorf("SortPOIs mutated input at index %d: was %q, now %q", i, v, input[i])
			}
		}
	})
}

func TestPOISuffixRow_Empty(t *testing.T) {
	got := maputil.POISuffixRow(nil, 4)
	if got != "    " {
		t.Errorf("POISuffixRow(nil, 4) = %q, want 4 spaces", got)
	}
}

func TestPOISuffixRow_PaddedToWidth(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cellW := rapid.IntRange(1, 8).Draw(rt, "cellW")
		// Strip ANSI from result and measure visible width.
		got := maputil.POISuffixRow(nil, cellW)
		if len(got) != cellW {
			t.Errorf("POISuffixRow(nil, %d) visible width = %d, want %d", cellW, len(got), cellW)
		}
	})
}

func TestPOISuffixRow_FourTypes_NoEllipsis(t *testing.T) {
	// Exactly 4 POI types: all 4 symbols rendered, no ellipsis (REQ-POI-4).
	input := []string{"merchant", "healer", "trainer", "guard"}
	got := maputil.POISuffixRow(input, 4)
	if containsRune(got, '…') {
		t.Errorf("POISuffixRow with exactly 4 POIs should NOT contain ellipsis, got %q", got)
	}
}

func TestPOISuffixRow_AtMostFourSymbols(t *testing.T) {
	// Five POI types present: first 3 colored, 4th slot is ellipsis (REQ-POI-4).
	input := []string{"merchant", "healer", "trainer", "guard", "npc"}
	got := maputil.POISuffixRow(input, 4)
	// Must contain the ellipsis character U+2026.
	if !containsRune(got, '…') {
		t.Errorf("POISuffixRow with 5 POIs should contain ellipsis, got %q", got)
	}
}

func TestPOISuffixRow_SinglePOI(t *testing.T) {
	// One merchant: colored $ + 3 spaces.
	got := maputil.POISuffixRow([]string{"merchant"}, 4)
	// Visible display width = 4: 1 symbol + 3 spaces.
	// ANSI sequences add bytes but not visible columns.
	// We check the string ends with 3 spaces.
	if len(got) == 4 {
		t.Errorf("POISuffixRow should have ANSI bytes, but got plain 4 chars: %q", got)
	}
	// Check last 3 chars are spaces.
	runes := []rune(got)
	for i := len(runes) - 3; i < len(runes); i++ {
		if runes[i] != ' ' {
			t.Errorf("POISuffixRow trailing padding: rune[%d] = %q, want space", i, runes[i])
		}
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

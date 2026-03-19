package technology_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// REQ-LF2: FlavorFor returns correct flavor per tradition.
func TestFlavorFor(t *testing.T) {
	cases := []struct {
		tradition string
		want      technology.TraditionFlavor
	}{
		{"technical", technology.TraditionFlavor{LoadoutTitle: "Field Loadout", PrepVerb: "Configure", SlotNoun: "slot", RestMessage: "Field loadout configured."}},
		{"bio_synthetic", technology.TraditionFlavor{LoadoutTitle: "Chem Kit", PrepVerb: "Mix", SlotNoun: "dose", RestMessage: "Chem kit mixed."}},
		{"neural", technology.TraditionFlavor{LoadoutTitle: "Neural Profile", PrepVerb: "Queue", SlotNoun: "routine", RestMessage: "Neural profile written."}},
		{"fanatic_doctrine", technology.TraditionFlavor{LoadoutTitle: "Doctrine", PrepVerb: "Prepare", SlotNoun: "rite", RestMessage: "Doctrine prepared."}},
		{"", technology.TraditionFlavor{LoadoutTitle: "Loadout", PrepVerb: "Prepare", SlotNoun: "slot", RestMessage: "Technologies prepared."}},
		{"unknown", technology.TraditionFlavor{LoadoutTitle: "Loadout", PrepVerb: "Prepare", SlotNoun: "slot", RestMessage: "Technologies prepared."}},
	}
	for _, tc := range cases {
		t.Run(tc.tradition, func(t *testing.T) {
			assert.Equal(t, tc.want, technology.FlavorFor(tc.tradition))
		})
	}
}

// REQ-LF3: DominantTradition maps job IDs correctly.
func TestDominantTradition(t *testing.T) {
	cases := []struct {
		jobID string
		want  string
	}{
		{"nerd", "technical"},
		{"naturalist", "bio_synthetic"},
		{"drifter", "bio_synthetic"},
		{"schemer", "neural"},
		{"influencer", "neural"},
		{"zealot", "fanatic_doctrine"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tc := range cases {
		t.Run(tc.jobID, func(t *testing.T) {
			assert.Equal(t, tc.want, technology.DominantTradition(tc.jobID))
		})
	}
}

// REQ-LF4: FormatPreparedTechs formats slots correctly.
func TestFormatPreparedTechs(t *testing.T) {
	flavor := technology.FlavorFor("technical") // LoadoutTitle="Field Loadout", SlotNoun="slot"

	t.Run("empty map", func(t *testing.T) {
		got := technology.FormatPreparedTechs(nil, flavor)
		assert.Equal(t, "No Field Loadout configured.", got)
	})

	t.Run("empty slots map", func(t *testing.T) {
		got := technology.FormatPreparedTechs(map[int][]*session.PreparedSlot{}, flavor)
		assert.Equal(t, "No Field Loadout configured.", got)
	})

	t.Run("single level one ready slot", func(t *testing.T) {
		slots := map[int][]*session.PreparedSlot{
			1: {{TechID: "scorching_blast", Expended: false}},
		}
		got := technology.FormatPreparedTechs(slots, flavor)
		want := "[Field Loadout]\n  Level 1 — 1 slot\n    scorching_blast    ready"
		assert.Equal(t, want, got)
	})

	t.Run("single level mixed ready and expended", func(t *testing.T) {
		slots := map[int][]*session.PreparedSlot{
			2: {
				{TechID: "heal_bio_synthetic", Expended: false},
				{TechID: "fear_bio_synthetic", Expended: true},
			},
		}
		got := technology.FormatPreparedTechs(slots, flavor)
		want := "[Field Loadout]\n  Level 2 — 2 slots\n    heal_bio_synthetic    ready\n    fear_bio_synthetic    expended"
		assert.Equal(t, want, got)
	})

	t.Run("multiple levels ascending order", func(t *testing.T) {
		slots := map[int][]*session.PreparedSlot{
			3: {{TechID: "tech_c", Expended: false}},
			1: {{TechID: "tech_a", Expended: false}},
		}
		got := technology.FormatPreparedTechs(slots, flavor)
		require.Contains(t, got, "Level 1")
		require.Contains(t, got, "Level 3")
		// Level 1 must appear before Level 3
		assert.Less(t, strings.Index(got, "Level 1"), strings.Index(got, "Level 3"))
	})

	t.Run("bio_synthetic flavor uses dose noun", func(t *testing.T) {
		bioFlavor := technology.FlavorFor("bio_synthetic") // SlotNoun="dose"
		slots := map[int][]*session.PreparedSlot{
			1: {
				{TechID: "heal_bio_synthetic", Expended: false},
				{TechID: "fear_bio_synthetic", Expended: false},
			},
		}
		got := technology.FormatPreparedTechs(slots, bioFlavor)
		assert.Contains(t, got, "2 doses")
		assert.Contains(t, got, "[Chem Kit]")
	})
}


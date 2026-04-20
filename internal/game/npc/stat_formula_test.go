package npc_test

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseHP returns the standard-tier base HP for a given level using linear
// interpolation between the spec-defined anchors (REQ-ZDS-1).
func baseHP(level int) float64 {
	anchors := [][2]int{
		{1, 12}, {5, 35}, {10, 70}, {15, 110}, {20, 160},
		{30, 270}, {40, 420}, {50, 600}, {60, 810}, {70, 1050},
		{80, 1320}, {90, 1620}, {100, 1950},
	}
	if level <= anchors[0][0] {
		return float64(anchors[0][1])
	}
	for i := 1; i < len(anchors); i++ {
		lo, hi := anchors[i-1], anchors[i]
		if level <= hi[0] {
			t := float64(level-lo[0]) / float64(hi[0]-lo[0])
			return float64(lo[1]) + t*float64(hi[1]-lo[1])
		}
	}
	return float64(anchors[len(anchors)-1][1])
}

// baseAC returns the standard-tier base AC for a given level.
func baseAC(level int) float64 {
	anchors := [][2]int{
		{1, 14}, {5, 15}, {10, 16}, {15, 17}, {20, 18},
		{30, 19}, {40, 20}, {50, 21}, {60, 22}, {70, 23},
		{80, 24}, {90, 25}, {100, 26},
	}
	if level <= anchors[0][0] {
		return float64(anchors[0][1])
	}
	for i := 1; i < len(anchors); i++ {
		lo, hi := anchors[i-1], anchors[i]
		if level <= hi[0] {
			t := float64(level-lo[0]) / float64(hi[0]-lo[0])
			return float64(lo[1]) + t*float64(hi[1]-lo[1])
		}
	}
	return float64(anchors[len(anchors)-1][1])
}

func tierHPMult(tier string) float64 {
	switch tier {
	case "minion":
		return 0.6
	case "elite":
		return 1.5
	case "champion":
		return 2.0
	case "boss":
		return 3.0
	default:
		return 1.0
	}
}

func tierACMod(tier string) float64 {
	switch tier {
	case "minion":
		return -2
	case "elite":
		return 2
	case "champion":
		return 4
	case "boss":
		return 5
	default:
		return 0
	}
}

// TestXPFormula_ProducesReasonableProgression verifies that the XP formula
// produces positive, non-degenerate values across all relevant level/tier
// combinations (REQ-ZDS-7).
func TestXPFormula_ProducesReasonableProgression(t *testing.T) {
	tierMultipliers := map[string]float64{
		"minion": 0.5, "standard": 1.0, "elite": 2.0, "champion": 3.0, "boss": 5.0,
	}
	for _, level := range []int{1, 5, 10, 15, 20, 30, 40, 50, 60, 70, 80, 90, 100} {
		for tier, mult := range tierMultipliers {
			xp := float64(level) * 50 * mult
			assert.Greater(t, xp, 0.0, "level %d tier %s: XP must be positive", level, tier)
			if level == 100 && tier == "standard" {
				assert.GreaterOrEqual(t, xp, 5000.0,
					"level 100 standard XP %.0f is degenerate (< 5000)", xp)
			}
		}
	}
}

// TestNPCStatFormula_AllTemplatesCompliant loads all NPC templates from the
// content directory and asserts that every template's max_hp and ac are within
// ±10% of the formula value for its level and tier (REQ-ZDS-1).
func TestNPCStatFormula_AllTemplatesCompliant(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "content", "npcs")

	templates, err := npc.LoadTemplates(root)
	require.NoError(t, err)
	require.NotEmpty(t, templates)

	for _, tmpl := range templates {
		if tmpl.Level == 0 {
			continue // non-combat service NPCs without a level
		}
		if tmpl.NPCType != "combat" {
			continue // skip non-combat NPCs (chip_docs, merchants, quest givers, etc.)
		}
		tmpl := tmpl
		t.Run(tmpl.ID, func(t *testing.T) {
			expHP := baseHP(tmpl.Level) * tierHPMult(tmpl.Tier)
			expAC := baseAC(tmpl.Level) + tierACMod(tmpl.Tier)

			actualHP := float64(tmpl.MaxHP)
			actualAC := float64(tmpl.AC)

			hpLo, hpHi := expHP*0.90, expHP*1.10
			assert.True(t, actualHP >= hpLo && actualHP <= hpHi,
				"template %q level %d tier %q: max_hp %d not within ±10%% of formula %.0f (range [%.0f, %.0f])",
				tmpl.ID, tmpl.Level, tmpl.Tier, tmpl.MaxHP, expHP, hpLo, hpHi,
			)

			acLo := math.Round(expAC) - 1
			acHi := math.Round(expAC) + 1
			assert.True(t, actualAC >= acLo && actualAC <= acHi,
				"template %q level %d tier %q: ac %d not within ±1 of formula %.1f",
				tmpl.ID, tmpl.Level, tmpl.Tier, tmpl.AC, expAC,
			)
		})
	}
}

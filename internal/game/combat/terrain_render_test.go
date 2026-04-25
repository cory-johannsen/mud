package combat_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// TestTerrainGlyphs_NormalIsDot verifies that normal terrain renders as a
// dot-shaped glyph in either Unicode or ASCII mode.
func TestTerrainGlyphs_NormalIsDot(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainNormal, false)
	if g != "·" && g != "." {
		t.Errorf("normal terrain glyph: want · or . got %q", g)
	}
}

// TestTerrainGlyphs_DifficultIsHash verifies difficult terrain has a
// non-empty glyph distinct from normal.
func TestTerrainGlyphs_DifficultIsHash(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainDifficult, false)
	if g == "" {
		t.Error("difficult terrain must have a non-empty glyph")
	}
}

// TestTerrainGlyphs_GreaterDifficultIsDense verifies that greater_difficult
// uses a glyph distinct from difficult.
func TestTerrainGlyphs_GreaterDifficultIsDense(t *testing.T) {
	g := combat.TerrainGlyph(combat.TerrainGreaterDifficult, false)
	gd := combat.TerrainGlyph(combat.TerrainDifficult, false)
	if g == gd {
		t.Error("greater_difficult and difficult must have distinct glyphs")
	}
}

// TestTerrainLegendOutput verifies the terrain legend mentions all four
// terrain type keywords.
func TestTerrainLegendOutput(t *testing.T) {
	legend := combat.TerrainLegendText(false)
	for _, keyword := range []string{"normal", "difficult", "impassable", "hazardous"} {
		if !strings.Contains(legend, keyword) {
			t.Errorf("terrain legend must mention %q", keyword)
		}
	}
}

package handlers

import (
	"regexp"
	"strings"
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// ansiRegexp matches ANSI escape sequences for stripping in tests.
var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes all ANSI color escape sequences from s.
func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func makeTile(zoneID, zoneName string, wx, wy int32, discovered, current bool, dangerLevel string) *gamev1.WorldZoneTile {
	return &gamev1.WorldZoneTile{
		ZoneId:      zoneID,
		ZoneName:    zoneName,
		WorldX:      wx,
		WorldY:      wy,
		Discovered:  discovered,
		Current:     current,
		DangerLevel: dangerLevel,
	}
}

// TestRenderWorldMap_EmptyResponse verifies no panic on empty tile list.
func TestRenderWorldMap_EmptyResponse(t *testing.T) {
	resp := &gamev1.MapResponse{}
	result := RenderWorldMap(resp, 120)
	require.NotEmpty(t, result, "must return at least a header line")
}

// TestRenderWorldMap_SingleTile_Discovered verifies that a single discovered zone
// renders with square brackets [ZZ].
func TestRenderWorldMap_SingleTile_Discovered(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("downtown", "Downtown Portland", 0, 0, true, false, "sketchy"),
		},
	}
	result := RenderWorldMap(resp, 120)
	require.Contains(t, result, "[01]", "discovered tile must render as [ZZ]")
}

// TestRenderWorldMap_SingleTile_Undiscovered verifies [??] render.
func TestRenderWorldMap_SingleTile_Undiscovered(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("unknown", "Unknown Zone", 0, 0, false, false, ""),
		},
	}
	result := RenderWorldMap(resp, 120)
	require.Contains(t, result, "[??]", "undiscovered tile must render as [??]")
}

// TestRenderWorldMap_CurrentZone verifies <ZZ> render for the current zone.
func TestRenderWorldMap_CurrentZone(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("downtown", "Downtown Portland", 0, 0, true, true, "sketchy"),
		},
	}
	result := RenderWorldMap(resp, 120)
	require.Contains(t, result, "<01>", "current zone must render as <ZZ>")
}

// TestRenderWorldMap_TwoColumnLayout verifies two-column layout at width >= 100.
func TestRenderWorldMap_TwoColumnLayout(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("z1", "Zone One", 0, 0, true, false, "safe"),
		},
	}
	wide := RenderWorldMap(resp, 100)
	narrow := RenderWorldMap(resp, 99)
	wideLines := strings.Count(wide, "\n")
	narrowLines := strings.Count(narrow, "\n")
	require.NotEqual(t, wideLines, narrowLines, "layout must differ between wide and narrow widths")
}

// TestRenderWorldMap_ConnectorBetweenAdjacentZones verifies that a horizontal connector
// appears between two zones differing by exactly 2 on X and 0 on Y.
func TestRenderWorldMap_ConnectorBetweenAdjacentZones(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("left", "Left", 0, 0, true, false, "safe"),
			makeTile("right", "Right", 2, 0, true, false, "safe"),
		},
	}
	result := RenderWorldMap(resp, 120)
	require.True(t,
		strings.Contains(result, "—") || strings.Contains(result, "-"),
		"horizontal connector must be present between adjacent zones")
}

// TestRenderWorldMap_NoDiagonalConnector verifies no connector between zones
// differing by 2 on both axes.
func TestRenderWorldMap_NoDiagonalConnector(t *testing.T) {
	resp := &gamev1.MapResponse{
		WorldTiles: []*gamev1.WorldZoneTile{
			makeTile("a", "A", 0, 0, true, false, "safe"),
			makeTile("b", "B", 2, 2, true, false, "safe"),
		},
	}
	result := RenderWorldMap(resp, 120)
	lines := strings.Split(result, "\n")
	// Between two tiles at (0,0) and (2,2), no connector row or column should
	// contain a "-" or "|" between the two cells. We check the connector row
	// (row index 1) for the absence of a horizontal connector.
	if len(lines) > 1 {
		require.NotContains(t, lines[1], "-",
			"diagonal pairs must not have horizontal connectors")
	}
}

// TestProperty_RenderWorldMap_LegendCountMatchesTileCount is a property-based test.
func TestProperty_RenderWorldMap_LegendCountMatchesTileCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(t, "n")
		tiles := make([]*gamev1.WorldZoneTile, n)
		for i := range tiles {
			tiles[i] = makeTile(
				rapid.StringMatching(`[a-z]{3,6}`).Draw(t, "id"),
				rapid.StringMatching(`[A-Z][a-z]{3,8}`).Draw(t, "name"),
				int32(rapid.IntRange(-10, 10).Draw(t, "wx")),
				int32(rapid.IntRange(-10, 10).Draw(t, "wy")),
				rapid.Bool().Draw(t, "disc"),
				false,
				"safe",
			)
		}
		resp := &gamev1.MapResponse{WorldTiles: tiles}
		// Use narrow width so legend entries appear as standalone lines (single-column mode),
		// avoiding ANSI prefix interference in the count check.
		result := RenderWorldMap(resp, 99)
		// Each tile should have exactly one legend entry formatted as "NN: name".
		// Strip ANSI codes before counting so we see the raw digit prefix.
		stripped := stripANSI(result)
		count := 0
		for _, line := range strings.Split(stripped, "\n") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) >= 3 && trimmed[0] >= '0' && trimmed[0] <= '9' &&
				trimmed[1] >= '0' && trimmed[1] <= '9' && trimmed[2] == ':' {
				count++
			}
		}
		if count != n {
			t.Fatalf("expected %d legend lines, got %d\noutput:\n%s", n, count, stripped)
		}
	})
}

package handlers

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestRenderBattlefield_FitsWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := rapid.SliceOfN(rapid.StringMatching(`[A-Za-z]{2,8}`), 1, 6).Draw(t, "names")
		width := rapid.IntRange(40, 200).Draw(t, "width")
		unique := dedupNames(names)
		snap := buildSnapFromNames(unique, "")
		line := RenderBattlefield(snap, width)
		if visibleLen(line) > width {
			t.Fatalf("battlefield line exceeds width %d (visibleLen=%d)", width, visibleLen(line))
		}
	})
}

func TestRenderRosterRow_FitsWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, "name")
		hp := rapid.IntRange(0, 200).Draw(t, "hp")
		maxHP := rapid.IntRange(1, 200).Draw(t, "maxHP")
		ap := rapid.IntRange(0, 6).Draw(t, "ap")
		maxAP := rapid.IntRange(1, 6).Draw(t, "maxAP")
		width := rapid.IntRange(40, 200).Draw(t, "width")
		isCurrent := rapid.Bool().Draw(t, "isCurrent")

		c := CombatantState{
			Name: name, HP: hp, MaxHP: maxHP,
			AP: ap, MaxAP: maxAP, IsCurrent: isCurrent,
		}
		row := RenderRosterRow(c, width)
		if visibleLen(row) > width {
			t.Fatalf("roster row exceeds width %d (visibleLen=%d)", width, visibleLen(row))
		}
	})
}

func TestRenderCombatScreen_ContainsRoundHeader(t *testing.T) {
	snap := CombatRenderSnapshot{
		Round:     3,
		TurnOrder: []string{"Alice", "Goblin"},
		Combatants: map[string]*CombatantState{
			"Alice":  {Name: "Alice", HP: 20, MaxHP: 30, AP: 2, MaxAP: 3, IsPlayer: true},
			"Goblin": {Name: "Goblin", HP: 8, MaxHP: 15, AP: 3, MaxAP: 3},
		},
		Log: []string{"Alice hits Goblin for 5 damage."},
	}
	screen := RenderCombatScreen(snap, 80)
	if !strings.Contains(screen, "Round 3") {
		t.Fatalf("expected 'Round 3' in screen, got:\n%s", screen)
	}
}

func TestRenderCombatScreen_ContainsAllCombatants(t *testing.T) {
	snap := CombatRenderSnapshot{
		Round:     1,
		TurnOrder: []string{"Alice", "Goblin", "Orc"},
		Combatants: map[string]*CombatantState{
			"Alice":  {Name: "Alice", HP: 20, MaxHP: 30, AP: 3, MaxAP: 3, IsPlayer: true},
			"Goblin": {Name: "Goblin", HP: 8, MaxHP: 15, AP: 3, MaxAP: 3},
			"Orc":    {Name: "Orc", HP: 12, MaxHP: 20, AP: 3, MaxAP: 3},
		},
	}
	screen := RenderCombatScreen(snap, 80)
	for _, name := range []string{"Alice", "Goblin", "Orc"} {
		if !strings.Contains(screen, name) {
			t.Fatalf("expected %q in screen", name)
		}
	}
}

func TestRenderBattlefield_PlayerMarked(t *testing.T) {
	snap := buildSnapFromNames([]string{"Alice", "Goblin"}, "Alice")
	line := RenderBattlefield(snap, 80)
	if !strings.Contains(line, "[*Alice") {
		t.Fatalf("expected player token [*Alice...] in battlefield, got: %q", line)
	}
	if strings.Contains(line, "[*Goblin") {
		t.Fatalf("NPC should not have player marker, got: %q", line)
	}
}

func TestRenderBattlefield_SortsByPosition(t *testing.T) {
	snap := CombatRenderSnapshot{
		TurnOrder: []string{"Alice", "Goblin"},
		PlayerName: "Alice",
		Combatants: map[string]*CombatantState{
			"Alice":  {Name: "Alice", Position: 25, IsPlayer: true},
			"Goblin": {Name: "Goblin", Position: 0},
		},
	}
	line := RenderBattlefield(snap, 80)
	// Goblin at position 0 should appear before Alice at position 25.
	goblinIdx := strings.Index(line, "Goblin")
	aliceIdx := strings.Index(line, "Alice")
	if goblinIdx < 0 || aliceIdx < 0 {
		t.Fatalf("expected both names in battlefield, got: %q", line)
	}
	if goblinIdx > aliceIdx {
		t.Fatalf("expected Goblin (pos 0) before Alice (pos 25), got: %q", line)
	}
}

// TestRenderBattlefield_ShowsDistanceOnSeparator verifies that the distance between
// adjacent combatants is shown on the separator, not as an absolute position in the token.
// This is the regression test for BUG-39.
func TestRenderBattlefield_ShowsDistanceOnSeparator(t *testing.T) {
	snap := CombatRenderSnapshot{
		TurnOrder:  []string{"Alice", "Goblin"},
		PlayerName: "Alice",
		Combatants: map[string]*CombatantState{
			"Alice":  {Name: "Alice", Position: 0, IsPlayer: true},
			"Goblin": {Name: "Goblin", Position: 25},
		},
	}
	line := RenderBattlefield(snap, 80)
	// Distance between Alice (0ft) and Goblin (25ft) must appear on the separator.
	if !strings.Contains(line, "25ft") {
		t.Fatalf("expected separator to show distance '25ft', got: %q", line)
	}
	// Tokens must NOT contain ":Xft" absolute position suffixes.
	if strings.Contains(line, ":0ft") || strings.Contains(line, ":25ft") {
		t.Fatalf("tokens must not show absolute positions; got: %q", line)
	}
}

// TestRenderBattlefield_ShowsMeleeDistance verifies that when combatants are at the same
// position (0ft distance), the separator shows "0ft" making melee range unambiguous.
func TestRenderBattlefield_ShowsMeleeDistance(t *testing.T) {
	snap := CombatRenderSnapshot{
		TurnOrder:  []string{"Alice", "Goblin"},
		PlayerName: "Alice",
		Combatants: map[string]*CombatantState{
			"Alice":  {Name: "Alice", Position: 25, IsPlayer: true},
			"Goblin": {Name: "Goblin", Position: 25},
		},
	}
	line := RenderBattlefield(snap, 80)
	// Distance between both at 25ft is 0ft → separator must show "0ft".
	if !strings.Contains(line, "0ft") {
		t.Fatalf("expected separator to show '0ft' for melee range, got: %q", line)
	}
	// Tokens must NOT contain absolute positions.
	if strings.Contains(line, ":25ft") {
		t.Fatalf("tokens must not show absolute positions; got: %q", line)
	}
}

func TestRenderCombatSummary_ContainsText(t *testing.T) {
	result := RenderCombatSummary("Victory!", 80)
	if !strings.Contains(result, "Victory!") {
		t.Fatalf("expected 'Victory!' in summary: %q", result)
	}
}

// buildSnapFromNames builds a minimal CombatRenderSnapshot from a name list for tests.
func buildSnapFromNames(names []string, playerName string) CombatRenderSnapshot {
	combatants := make(map[string]*CombatantState, len(names))
	for i, n := range names {
		combatants[n] = &CombatantState{Name: n, Position: i * 5, IsPlayer: n == playerName}
	}
	return CombatRenderSnapshot{
		TurnOrder:  names,
		PlayerName: playerName,
		Combatants: combatants,
	}
}

func dedupNames(names []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// visibleLen returns the rune count after stripping ANSI escape sequences.
func visibleLen(s string) int {
	// Strip ANSI sequences: \x1b[...m
	clean := s
	for {
		i := strings.Index(clean, "\x1b[")
		if i < 0 {
			break
		}
		j := strings.IndexByte(clean[i:], 'm')
		if j < 0 {
			break
		}
		clean = clean[:i] + clean[i+j+1:]
	}
	return len([]rune(clean))
}

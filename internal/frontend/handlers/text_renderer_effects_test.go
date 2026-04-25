package handlers

import (
	"strings"
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestRenderCharacterSheet_AppendsEffectsWhenPresent verifies that when the
// CharacterSheetView includes a non-empty EffectsSummary, the rendered sheet
// contains both the Effects header and the summary body.
func TestRenderCharacterSheet_AppendsEffectsWhenPresent(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:           "Test",
		Job:            "gunslinger",
		Gender:         "non-binary",
		Level:          1,
		EffectsSummary: "Effects:\r\n  Heroism (self) attack +1 status (active)\r\n",
	}
	out := RenderCharacterSheet(csv, 80)
	if !strings.Contains(out, "Effects") {
		t.Errorf("expected 'Effects' header in rendered sheet, got:\n%s", out)
	}
	if !strings.Contains(out, "Heroism") {
		t.Errorf("expected 'Heroism' in rendered sheet, got:\n%s", out)
	}
}

// TestRenderCharacterSheet_OmitsEffectsWhenEmpty verifies that an empty
// EffectsSummary does not produce the Effects divider/header in the output.
func TestRenderCharacterSheet_OmitsEffectsWhenEmpty(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:           "Test",
		Job:            "gunslinger",
		Gender:         "non-binary",
		Level:          1,
		EffectsSummary: "",
	}
	out := RenderCharacterSheet(csv, 80)
	if strings.Contains(out, "── Effects") {
		t.Errorf("did not expect '── Effects' divider in sheet with empty summary, got:\n%s", out)
	}
}

// TestRenderCharacterSheet_EffectsWidthAware verifies the Effects divider
// dash count scales with the terminal width (narrower width = fewer dashes).
func TestRenderCharacterSheet_EffectsWidthAware(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:           "Test",
		Job:            "gunslinger",
		Gender:         "non-binary",
		Level:          1,
		EffectsSummary: "Effects:\r\n  (none)\r\n",
	}
	wide := RenderCharacterSheet(csv, 120)
	narrow := RenderCharacterSheet(csv, 40)
	// Count the dash runes in each header line. Width-aware rendering should
	// produce a strictly shorter divider at width=40 than at width=120.
	countDashes := func(s string) int {
		n := 0
		for _, r := range s {
			if r == '─' {
				n++
			}
		}
		return n
	}
	if countDashes(wide) <= countDashes(narrow) {
		t.Errorf("expected more dashes at width=120 than width=40; got wide=%d narrow=%d",
			countDashes(wide), countDashes(narrow))
	}
}

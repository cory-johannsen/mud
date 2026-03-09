package character_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
)

func TestRandomStandardGender_ReturnsStandardValue(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		g := character.RandomStandardGender()
		found := false
		for _, s := range character.StandardGenders {
			if g == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RandomStandardGender() returned non-standard value %q", g)
		}
		seen[g] = true
	}
	// After 200 iterations we expect all 4 values to appear at least once
	if len(seen) < len(character.StandardGenders) {
		t.Logf("Only saw %d of %d standard genders in 200 trials (may be flaky)", len(seen), len(character.StandardGenders))
	}
}

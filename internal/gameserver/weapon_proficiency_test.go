package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveWeaponProficiency_DirectHit verifies that an exact category match
// is returned as-is (GH #241).
func TestResolveWeaponProficiency_DirectHit(t *testing.T) {
	profs := map[string]string{"martial_melee": "expert"}
	rank, key := resolveWeaponProficiency(profs, "martial_melee")
	assert.Equal(t, "expert", rank)
	assert.Equal(t, "martial_melee", key)
}

// TestResolveWeaponProficiency_SimpleMeleeFallsBackToSimpleWeapons verifies
// that a weapon with proficiency_category "simple_melee" picks up a player's
// "simple_weapons" grant (GH #241 root cause).
func TestResolveWeaponProficiency_SimpleMeleeFallsBackToSimpleWeapons(t *testing.T) {
	profs := map[string]string{"simple_weapons": "trained"}
	rank, key := resolveWeaponProficiency(profs, "simple_melee")
	assert.Equal(t, "trained", rank)
	assert.Equal(t, "simple_weapons", key)
}

// TestResolveWeaponProficiency_SimpleRangedFallsBack verifies the same
// fallback for ranged variants.
func TestResolveWeaponProficiency_SimpleRangedFallsBack(t *testing.T) {
	profs := map[string]string{"simple_weapons": "trained"}
	rank, _ := resolveWeaponProficiency(profs, "simple_ranged")
	assert.Equal(t, "trained", rank)
}

// TestResolveWeaponProficiency_MartialMeleeFallsBackThroughMartial verifies
// that a martial_melee weapon picks up a player's martial_weapons grant.
func TestResolveWeaponProficiency_MartialMeleeFallsBackThroughMartial(t *testing.T) {
	profs := map[string]string{"martial_weapons": "trained"}
	rank, key := resolveWeaponProficiency(profs, "martial_melee")
	assert.Equal(t, "trained", rank)
	assert.Equal(t, "martial_weapons", key)
}

// TestResolveWeaponProficiency_MartialMeleeCoversSimple verifies PF2E
// convention "martial implies simple": a player with only simple_weapons still
// counts as trained when using a martial weapon via the simple_weapons
// fallback.
func TestResolveWeaponProficiency_MartialMeleeCoversSimple(t *testing.T) {
	profs := map[string]string{"simple_weapons": "trained"}
	rank, key := resolveWeaponProficiency(profs, "martial_melee")
	assert.Equal(t, "trained", rank)
	assert.Equal(t, "simple_weapons", key)
}

// TestResolveWeaponProficiency_BestRankWins verifies that when multiple
// fallback categories are present, the highest-ranked one is returned.
func TestResolveWeaponProficiency_BestRankWins(t *testing.T) {
	profs := map[string]string{
		"simple_weapons":  "trained",
		"martial_weapons": "expert",
	}
	rank, key := resolveWeaponProficiency(profs, "martial_melee")
	assert.Equal(t, "expert", rank)
	assert.Equal(t, "martial_weapons", key)
}

// TestResolveWeaponProficiency_NoMatch_ReturnsUntrained verifies the default
// untrained result and empty matched-category for an unmatched lookup.
func TestResolveWeaponProficiency_NoMatch_ReturnsUntrained(t *testing.T) {
	profs := map[string]string{"medicine": "trained"}
	rank, key := resolveWeaponProficiency(profs, "martial_melee")
	assert.Equal(t, "untrained", rank)
	assert.Equal(t, "", key)
}

// TestResolveWeaponProficiency_NilProfs verifies defensive behavior.
func TestResolveWeaponProficiency_NilProfs(t *testing.T) {
	rank, key := resolveWeaponProficiency(nil, "simple_melee")
	assert.Equal(t, "untrained", rank)
	assert.Equal(t, "", key)
}

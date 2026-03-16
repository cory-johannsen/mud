package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestThrow_FriendlyFireFalse_EnemyOnly verifies REQ-T8 (friendly_fire: false path):
// only enemy-kind combatants are in the target list.
func TestThrow_FriendlyFireFalse_EnemyOnly(t *testing.T) {
	actor := newCombatant("p1", KindPlayer, 10)
	actor.Name = "Alice"
	actor.Level = 2
	enemy := newCombatant("n1", KindNPC, 10)
	enemy.Name = "Rat"
	ally := newCombatant("p2", KindPlayer, 10)
	ally.Name = "Bob"
	cbt := &Combat{Combatants: []*Combatant{actor, enemy, ally}}

	grenade := newGrenade(15, false)

	targets := explosiveTargetsOf(cbt, actor, grenade)
	assert.Len(t, targets, 1, "with friendly_fire=false, only the enemy should be targeted")
	assert.Equal(t, "n1", targets[0].ID, "only NPC enemy should be in target list")
}

// TestThrow_FriendlyFireTrue_AllyIncluded verifies REQ-T8 (friendly_fire: true path):
// all living non-actor combatants including allies are in the target list.
func TestThrow_FriendlyFireTrue_AllyIncluded(t *testing.T) {
	actor := newCombatant("p1", KindPlayer, 10)
	actor.Name = "Alice"
	actor.Level = 2
	enemy := newCombatant("n1", KindNPC, 10)
	enemy.Name = "Rat"
	ally := newCombatant("p2", KindPlayer, 10)
	ally.Name = "Bob"
	cbt := &Combat{Combatants: []*Combatant{actor, enemy, ally}}

	grenade := newGrenade(10, true)

	targets := explosiveTargetsOf(cbt, actor, grenade)
	assert.Len(t, targets, 2, "with friendly_fire=true, both NPC and ally should be targeted")
	var allyFound bool
	for _, tgt := range targets {
		if tgt.ID == "p2" {
			allyFound = true
		}
	}
	assert.True(t, allyFound, "ally must be in target list when friendly_fire=true")
}

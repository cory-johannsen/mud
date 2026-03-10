package combat_test

import (
	"math/rand"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
)

func TestResolveAttack_PropagatesDamageType(t *testing.T) {
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, StrMod: 2,
		WeaponDamageType: "fire",
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	src := rand.New(rand.NewSource(0))
	result := combat.ResolveAttack(attacker, target, src)
	assert.Equal(t, "fire", result.DamageType)
}

func TestResolveAttack_EmptyDamageTypeWhenNotSet(t *testing.T) {
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, StrMod: 2,
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	src := rand.New(rand.NewSource(0))
	result := combat.ResolveAttack(attacker, target, src)
	assert.Equal(t, "", result.DamageType)
}

func TestResolveAttack_WeaponNamePassedThrough(t *testing.T) {
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, StrMod: 2,
		WeaponName: "Cheap Blade",
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	src := rand.New(rand.NewSource(0))
	result := combat.ResolveAttack(attacker, target, src)
	assert.Equal(t, "Cheap Blade", result.WeaponName)
}

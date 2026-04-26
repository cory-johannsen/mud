package skillaction_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/skillaction"
)

func newCombatWith(t *testing.T, actor, target *combat.Combatant) *combat.Combat {
	t.Helper()
	cbt := &combat.Combat{Combatants: []*combat.Combatant{actor}}
	if target != nil {
		cbt.Combatants = append(cbt.Combatants, target)
	}
	return cbt
}

func mkPlayer(id, name string) *combat.Combatant {
	return &combat.Combatant{ID: id, Kind: combat.KindPlayer, Name: name, MaxHP: 30, CurrentHP: 30, AC: 12}
}

func mkNPC(id, name, faction string) *combat.Combatant {
	return &combat.Combatant{ID: id, Kind: combat.KindNPC, Name: name, FactionID: faction, MaxHP: 20, CurrentHP: 20, AC: 12}
}

func demoralizeDef(t *testing.T) *skillaction.ActionDef {
	def, err := skillaction.Load([]byte(demoralizeYAML), frightenedRegistry(t))
	require.NoError(t, err)
	return def
}

func TestValidateTarget_NPCOnly_RejectsPlayerTarget(t *testing.T) {
	actor := mkPlayer("p1", "Riker")
	target := mkPlayer("p2", "Worf")
	cbt := newCombatWith(t, actor, target)
	err := skillaction.ValidateTarget(skillaction.TargetCtx{Combat: cbt, Actor: actor, Target: target}, demoralizeDef(t))
	require.Error(t, err)
	pe, ok := err.(*skillaction.PreconditionError)
	require.True(t, ok)
	require.Equal(t, "target_kind", pe.Field)
}

func TestValidateTarget_DeadTargetRejected(t *testing.T) {
	actor := mkPlayer("p1", "Riker")
	target := mkNPC("n1", "Thug", "raiders")
	target.CurrentHP = 0
	cbt := newCombatWith(t, actor, target)
	err := skillaction.ValidateTarget(skillaction.TargetCtx{Combat: cbt, Actor: actor, Target: target}, demoralizeDef(t))
	require.Error(t, err)
}

func TestValidateTarget_HappyPath(t *testing.T) {
	actor := mkPlayer("p1", "Riker")
	target := mkNPC("n1", "Thug", "raiders")
	cbt := newCombatWith(t, actor, target)
	err := skillaction.ValidateTarget(skillaction.TargetCtx{Combat: cbt, Actor: actor, Target: target}, demoralizeDef(t))
	require.NoError(t, err)
}

func TestValidateTarget_NoTarget(t *testing.T) {
	actor := mkPlayer("p1", "Riker")
	cbt := newCombatWith(t, actor, nil)
	err := skillaction.ValidateTarget(skillaction.TargetCtx{Combat: cbt, Actor: actor, Target: nil}, demoralizeDef(t))
	require.Error(t, err)
	pe, ok := err.(*skillaction.PreconditionError)
	require.True(t, ok)
	require.Equal(t, "target", pe.Field)
}

package effect_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cory-johannsen/mud/internal/game/effect"
)

func TestEffectSet_NilSafeVersion(t *testing.T) {
	var s *effect.EffectSet
	assert.Equal(t, uint64(0), s.Version())
}

func TestEffectSet_NilSafeAll(t *testing.T) {
	var s *effect.EffectSet
	assert.Nil(t, s.All())
}

func TestEffectSet_ApplyAndRetrieve(t *testing.T) {
	s := effect.NewEffectSet()
	e := effect.Effect{
		EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 2,
	}
	s.Apply(e)
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, "e1", all[0].EffectID)
}

func TestEffectSet_Apply_SameKeyOverwrites(t *testing.T) {
	s := effect.NewEffectSet()
	e1 := effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 2}
	e2 := effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -2, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 3}
	s.Apply(e1)
	s.Apply(e2)
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, -2, all[0].Bonuses[0].Value)
}

func TestEffectSet_RemoveBySource(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "feat:toughness", CasterUID: "uid1",
		Bonuses: []effect.Bonus{{Stat: effect.StatGrit, Value: 1, Type: effect.BonusTypeUntyped}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:prone", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 2})
	s.RemoveBySource("feat:toughness")
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_RemoveByCaster_OnlyLinked(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:blessed", CasterUID: "ally1",
		LinkedToCaster: true,
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:frightened", CasterUID: "enemy1",
		LinkedToCaster: false,
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.RemoveByCaster("ally1")
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_Tick_DecrementsAndExpires(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:dazzled", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 1})
	expired := s.Tick()
	require.Len(t, expired, 0) // DurRemain 1→0; expire next Tick per round semantics
	expired2 := s.Tick()
	assert.Len(t, expired2, 1)
	assert.Len(t, s.All(), 0)
}

func TestEffectSet_ClearEncounter(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:inspired", CasterUID: "",
		DurKind: effect.DurationEncounter})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "feat:resolve", CasterUID: "",
		DurKind: effect.DurationPermanent})
	s.ClearEncounter()
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, "e2", all[0].EffectID)
}

func TestEffectSet_Version_MonotonicallyIncreases(t *testing.T) {
	s := effect.NewEffectSet()
	v0 := s.Version()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "s", Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: 1, Type: effect.BonusTypeItem}}, DurKind: effect.DurationPermanent})
	v1 := s.Version()
	s.RemoveBySource("s")
	v2 := s.Version()
	assert.Less(t, v0, v1)
	assert.Less(t, v1, v2)
}

func TestProperty_EffectSet_Version_AlwaysIncreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := effect.NewEffectSet()
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		prev := s.Version()
		for i := 0; i < n; i++ {
			s.Apply(effect.Effect{
				EffectID: fmt.Sprintf("e%d", i),
				SourceID: fmt.Sprintf("src%d", i),
				Bonuses:  []effect.Bonus{{Stat: effect.StatAttack, Value: i + 1, Type: effect.BonusTypeUntyped}},
				DurKind:  effect.DurationPermanent,
			})
			v := s.Version()
			assert.Greater(rt, v, prev)
			prev = v
		}
	})
}

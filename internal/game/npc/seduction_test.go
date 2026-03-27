package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestNPCTemplate_SeductionProbabilityParsesFromYAML(t *testing.T) {
	data := []byte("id: test\nname: Test\nmax_hp: 10\nlevel: 1\nseduction_probability: 0.4\n")
	var tmpl npc.Template
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.InDelta(t, 0.4, tmpl.SeductionProbability, 0.0001)
}

func TestNPCTemplate_SeductionGenderParsesFromYAML(t *testing.T) {
	data := []byte("id: test\nname: Test\nmax_hp: 10\nlevel: 1\nseduction_gender: male\n")
	var tmpl npc.Template
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.Equal(t, "male", tmpl.SeductionGender)
}

func TestNPCInstance_PropagatesSeductionFields(t *testing.T) {
	tmpl := &npc.Template{
		ID:                   "patron",
		Name:                 "Patron",
		MaxHP:                30,
		Level:                3,
		SeductionProbability: 0.3,
		SeductionGender:      "male",
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.InDelta(t, 0.3, inst.SeductionProbability, 0.0001)
	assert.Equal(t, "male", inst.SeductionGender)
}

func TestProperty_NPC_SeductionProbability_AlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		prob := rapid.Float64Range(0, 1).Draw(rt, "prob")
		tmpl := &npc.Template{
			ID:                   "tmpl",
			Name:                 "NPC",
			MaxHP:                10,
			Level:                1,
			SeductionProbability: prob,
		}
		inst := npc.NewInstance("id", tmpl, "room")
		if inst.SeductionProbability < 0 || inst.SeductionProbability > 1 {
			rt.Fatalf("SeductionProbability %v out of [0,1]", inst.SeductionProbability)
		}
	})
}

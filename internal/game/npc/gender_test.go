package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestNPCTemplate_GenderParsesFromYAML(t *testing.T) {
	var tmpl npc.Template
	data := []byte("id: test_npc\nname: Test NPC\ngender: female\nmax_hp: 10\nlevel: 1\n")
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.Equal(t, "female", tmpl.Gender)
}

func TestNPCInstance_PropagatesGenderFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID:     "soldier",
		Name:   "Soldier",
		Gender: "male",
		MaxHP:  30,
		Level:  2,
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.Equal(t, "male", inst.Gender)
}

func TestNPCInstance_GenderEmpty_WhenTemplateHasNoGender(t *testing.T) {
	tmpl := &npc.Template{
		ID:    "robot",
		Name:  "Robot",
		MaxHP: 20,
		Level: 1,
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.Equal(t, "", inst.Gender)
}

func TestNPCInstance_SeductionRejected_InitiallyNil(t *testing.T) {
	tmpl := &npc.Template{
		ID:    "guard",
		Name:  "Guard",
		MaxHP: 40,
		Level: 3,
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.Nil(t, inst.SeductionRejected)
}

func TestProperty_NPC_SeductionRejected_AlwaysNilOnNewInstance(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hp := rapid.IntRange(1, 200).Draw(rt, "hp")
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		tmpl := &npc.Template{
			ID:    "tmpl",
			Name:  "NPC",
			MaxHP: hp,
			Level: level,
		}
		inst := npc.NewInstance("id", tmpl, "room")
		if inst.SeductionRejected != nil {
			rt.Fatal("SeductionRejected must be nil on new instance")
		}
	})
}

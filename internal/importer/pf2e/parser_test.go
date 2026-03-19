package pf2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer/pf2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestParseSpell_Fireball(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fireball.json"))
	require.NoError(t, err)
	spell, err := pf2e.ParseSpell(data)
	require.NoError(t, err)
	assert.Equal(t, "Fireball", spell.Name)
	assert.Equal(t, 3, spell.System.Level.Value)
	assert.Contains(t, spell.System.Traits.Traditions, "arcane")
	assert.Contains(t, spell.System.Traits.Traditions, "primal")
	assert.Equal(t, "2", spell.System.Time.Value)
	assert.Equal(t, "500 feet", spell.System.Range.Value)
	require.NotNil(t, spell.System.Area)
	assert.Equal(t, "burst", spell.System.Area.Type)
	assert.Equal(t, 20, spell.System.Area.Value)
	require.Contains(t, spell.System.Damage, "0")
	assert.Equal(t, "6d6", spell.System.Damage["0"].Formula)
	assert.Equal(t, "fire", spell.System.Damage["0"].DamageType)
}

func TestParseSpell_MindLink(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "mindlink.json"))
	require.NoError(t, err)
	spell, err := pf2e.ParseSpell(data)
	require.NoError(t, err)
	assert.Equal(t, "Mind Link", spell.Name)
	assert.Equal(t, "touch", spell.System.Range.Value)
	assert.Equal(t, "1 minute", spell.System.Duration.Value)
	assert.Empty(t, spell.System.Damage)
	assert.Nil(t, spell.System.Area)
}

func TestParseSpell_MalformedJSON_ReturnsError(t *testing.T) {
	_, err := pf2e.ParseSpell([]byte(`{not valid json`))
	require.Error(t, err)
}

func TestPropertyParseSpell_NoPanic(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.SliceOf(rapid.Byte()).Draw(rt, "input")
		_, _ = pf2e.ParseSpell(input)
	})
}

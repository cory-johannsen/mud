package npc_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestTemplate_RespawnDelay_ParsesCorrectly(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "5m"
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "5m", tmpl.RespawnDelay)
}

func TestTemplate_RespawnDelay_EmptyByDefault(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "", tmpl.RespawnDelay)
}

func TestProperty_Template_ValidRespawnDelay_ParsesWithoutError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate valid duration strings
		value := rapid.IntRange(1, 3600).Draw(rt, "value")
		unit := rapid.SampledFrom([]string{"s", "m", "h"}).Draw(rt, "unit")
		delay := fmt.Sprintf("%d%s", value, unit)

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "%s"
`, delay))
		tmpl, err := npc.LoadTemplateFromBytes(data)
		require.NoError(rt, err)
		assert.Equal(rt, delay, tmpl.RespawnDelay)
	})
}

func TestProperty_Template_InvalidRespawnDelay_ReturnsError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate invalid duration strings (words that are not valid Go durations)
		invalid := rapid.SampledFrom([]string{"forever", "never", "5 minutes", "1day", "abc"}).Draw(rt, "invalid")

		data := []byte(fmt.Sprintf(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
respawn_delay: "%s"
`, invalid))
		_, err := npc.LoadTemplateFromBytes(data)
		assert.Error(rt, err, "expected error for invalid respawn_delay %q", invalid)
	})
}

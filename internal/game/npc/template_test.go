package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	templates, err := npc.LoadTemplatesFromBytes(data)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "5m", templates[0].RespawnDelay)
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
	templates, err := npc.LoadTemplatesFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "", templates[0].RespawnDelay)
}

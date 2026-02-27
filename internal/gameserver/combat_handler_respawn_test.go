package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCombatHandler_NPCDeath_InstanceRemovedOnCombatEnd verifies that when an
// NPC is removed from npcMgr, it no longer appears in InstancesInRoom.
// The full integration (combat round → death → respawn) is verified in Task 5.
func TestCombatHandler_NPCDeath_InstanceRemovedOnCombatEnd(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID: "ganger", Name: "Ganger", Description: "d",
		Level: 1, MaxHP: 10, AC: 10, RespawnDelay: "1m",
	}
	inst, err := mgr.Spawn(tmpl, "r1")
	require.NoError(t, err)

	err = mgr.Remove(inst.ID)
	require.NoError(t, err)

	assert.Empty(t, mgr.InstancesInRoom("r1"))
}

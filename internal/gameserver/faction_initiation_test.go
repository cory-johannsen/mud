package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
)

// TestFactionInitiation_AllOutWarRoomTriggersHostility verifies that when a JC NPC
// enters an all_out_war room containing a QCE NPC, combat is initiated (REQ-CCF-3a,3b).
func TestFactionInitiation_AllOutWarRoomTriggersHostility(t *testing.T) {
	initiated := false
	var initiatingInst, targetInst *npc.Instance

	room := &world.Room{
		ID:          "cc_the_stage",
		DangerLevel: string(danger.AllOutWar),
	}

	jcInst := &npc.Instance{ID: "jc-1", FactionID: "just_clownin", CurrentHP: 100}
	qceInst := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 80}

	hostiles := map[string][]string{
		"just_clownin": {"queer_clowning_experience", "unwoke_maga_clown_army"},
	}

	initiateFunc := func(attacker, target *npc.Instance, r *world.Room) {
		initiated = true
		initiatingInst = attacker
		targetInst = target
	}

	npcListInRoom := func(roomID string) []*npc.Instance {
		if roomID == "cc_the_stage" {
			return []*npc.Instance{qceInst}
		}
		return nil
	}

	getRoom := func(roomID string) *world.Room { return room }
	getHostiles := func(factionID string) []string { return hostiles[factionID] }

	checkFactionInitiation(jcInst, "cc_the_stage", npcListInRoom, getRoom, getHostiles, initiateFunc)

	assert.True(t, initiated, "combat should be initiated")
	assert.Equal(t, jcInst.ID, initiatingInst.ID)
	assert.Equal(t, qceInst.ID, targetInst.ID, "target is the QCE NPC (lowest HP)")
}

// TestFactionInitiation_SafeRoomDoesNotTrigger verifies that faction initiation
// does NOT fire in non-all_out_war rooms (REQ-CCF-3b).
func TestFactionInitiation_SafeRoomDoesNotTrigger(t *testing.T) {
	for _, dl := range []danger.DangerLevel{danger.Safe, danger.Sketchy, danger.Dangerous} {
		initiated := false
		room := &world.Room{ID: "some-room", DangerLevel: string(dl)}
		jcInst := &npc.Instance{ID: "jc-1", FactionID: "just_clownin", CurrentHP: 100}
		qceInst := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 80}

		checkFactionInitiation(jcInst, "some-room",
			func(string) []*npc.Instance { return []*npc.Instance{qceInst} },
			func(string) *world.Room { return room },
			func(string) []string { return []string{"queer_clowning_experience"} },
			func(_, _ *npc.Instance, _ *world.Room) { initiated = true },
		)
		assert.False(t, initiated, "danger_level %v must not trigger initiation", dl)
	}
}

// TestFactionInitiation_NoFactionIDDoesNotTrigger verifies that an NPC without
// a faction ID does not initiate combat (REQ-CCF-3a).
func TestFactionInitiation_NoFactionIDDoesNotTrigger(t *testing.T) {
	initiated := false
	room := &world.Room{ID: "some-room", DangerLevel: string(danger.AllOutWar)}
	noFactionInst := &npc.Instance{ID: "npc-1", FactionID: "", CurrentHP: 100}
	qceInst := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 80}

	checkFactionInitiation(noFactionInst, "some-room",
		func(string) []*npc.Instance { return []*npc.Instance{qceInst} },
		func(string) *world.Room { return room },
		func(string) []string { return []string{"queer_clowning_experience"} },
		func(_, _ *npc.Instance, _ *world.Room) { initiated = true },
	)
	assert.False(t, initiated, "NPC without faction must not trigger initiation")
}

// TestFactionInitiation_LowestHPTargetSelected verifies that the hostile NPC with
// the lowest HP is selected as the combat target (REQ-CCF-3c).
func TestFactionInitiation_LowestHPTargetSelected(t *testing.T) {
	var targetInst *npc.Instance

	room := &world.Room{ID: "war-room", DangerLevel: string(danger.AllOutWar)}
	jcInst := &npc.Instance{ID: "jc-1", FactionID: "just_clownin", CurrentHP: 100}
	qce1 := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 60}
	qce2 := &npc.Instance{ID: "qce-2", FactionID: "queer_clowning_experience", CurrentHP: 30}
	qce3 := &npc.Instance{ID: "qce-3", FactionID: "queer_clowning_experience", CurrentHP: 90}

	checkFactionInitiation(jcInst, "war-room",
		func(string) []*npc.Instance { return []*npc.Instance{qce1, qce2, qce3} },
		func(string) *world.Room { return room },
		func(string) []string { return []string{"queer_clowning_experience"} },
		func(_, target *npc.Instance, _ *world.Room) { targetInst = target },
	)

	assert.NotNil(t, targetInst)
	assert.Equal(t, "qce-2", targetInst.ID, "lowest HP hostile should be targeted")
}

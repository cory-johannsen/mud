package gameserver

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// TestProperty_FactionInitiation_AllOutWarAlwaysInitiates verifies that whenever
// a faction NPC enters an all_out_war room containing a hostile-faction NPC,
// initiation always fires (REQ-CCF-12e).
func TestProperty_FactionInitiation_AllOutWarAlwaysInitiates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		arrivingFaction := rapid.SampledFrom([]string{
			"just_clownin", "queer_clowning_experience", "unwoke_maga_clown_army",
		}).Draw(rt, "arriving_faction")

		hostileFactions := map[string][]string{
			"just_clownin":              {"queer_clowning_experience", "unwoke_maga_clown_army"},
			"queer_clowning_experience": {"just_clownin", "unwoke_maga_clown_army"},
			"unwoke_maga_clown_army":    {"just_clownin", "queer_clowning_experience"},
		}

		existingFaction := rapid.SampledFrom(hostileFactions[arrivingFaction]).Draw(rt, "existing_faction")

		arrivalHP := rapid.IntRange(1, 2430).Draw(rt, "arrival_hp")
		existingHP := rapid.IntRange(1, 2430).Draw(rt, "existing_hp")

		arrivalInst := &npc.Instance{ID: "arr-1", FactionID: arrivingFaction, CurrentHP: arrivalHP}
		existingInst := &npc.Instance{ID: "ex-1", FactionID: existingFaction, CurrentHP: existingHP}

		room := &world.Room{ID: "cc_the_stage", DangerLevel: string(danger.AllOutWar)}

		initiated := false
		checkFactionInitiation(
			arrivalInst, "cc_the_stage",
			func(string) []*npc.Instance { return []*npc.Instance{existingInst} },
			func(string) *world.Room { return room },
			func(fid string) []string { return hostileFactions[fid] },
			func(_, _ *npc.Instance, _ *world.Room) { initiated = true },
		)

		if !initiated {
			rt.Fatalf("faction initiation did not fire for %s vs %s in all_out_war room",
				arrivingFaction, existingFaction)
		}
	})
}

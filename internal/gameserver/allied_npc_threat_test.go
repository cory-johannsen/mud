package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// TestAlliedNPCDoesNotEngagePlayer verifies that a combat-capable NPC whose
// FactionID matches the player's FactionID does not become hostile,
// even when the NPC disposition defaults to "hostile".
//
// Precondition: NPC has FactionID=="machete", Disposition=="hostile"; player has FactionID=="machete".
// Postcondition: isHostileToPlayers==false after applying allied-faction exclusion.
func TestAlliedNPCDoesNotEngagePlayer(t *testing.T) {
	machete := &faction.FactionDef{
		ID:              "machete",
		Name:            "Team Machete",
		ZoneID:          "ironyard",
		HostileFactions: []string{},
		Tiers: []faction.FactionTier{
			{ID: "outsider", Label: "Outsider", MinRep: 0},
			{ID: "blade", Label: "Blade", MinRep: 100},
			{ID: "cutter", Label: "Cutter", MinRep: 300},
			{ID: "warsmith", Label: "Warsmith", MinRep: 600},
		},
	}
	reg := faction.FactionRegistry{"machete": machete}
	svc := faction.NewService(reg)

	sess := &session.PlayerSession{
		UID:        "player-1",
		FactionID:  "machete",
		FactionRep: map[string]int{"machete": 0},
	}

	inst := &npc.Instance{}
	inst.Disposition = "hostile"
	inst.FactionID = "machete"

	players := []*session.PlayerSession{sess}

	// Mirror the revised threat-assessment logic from grpc_service.go.
	isHostileToPlayers := inst.Disposition == "hostile"
	// Allied-faction exclusion: suppress hostility if any room player is an ally of this NPC.
	if isHostileToPlayers && svc != nil && inst.FactionID != "" {
		for _, p := range players {
			if svc.IsAllyOf(p, inst.FactionID) {
				isHostileToPlayers = false
				break
			}
		}
	}
	// Enemy-faction promotion: non-hostile NPC becomes hostile if any room player is a faction enemy.
	if !isHostileToPlayers && svc != nil && inst.FactionID != "" {
		for _, p := range players {
			if svc.IsEnemyOf(p, inst.FactionID) {
				isHostileToPlayers = true
				break
			}
		}
	}

	if isHostileToPlayers {
		t.Error("allied NPC (machete) should not be hostile to machete player")
	}
}

// TestEnemyFactionNPCEngagesPlayer verifies that a non-hostile NPC whose
// faction is hostile to the player's faction becomes hostile via enemy-faction promotion.
//
// Precondition: NPC has FactionID=="ironclad", Disposition=="neutral"; player has FactionID=="machete"; ironclad lists machete as hostile.
// Postcondition: isHostileToPlayers==true after enemy-faction promotion.
func TestEnemyFactionNPCEngagesPlayer(t *testing.T) {
	machete := &faction.FactionDef{
		ID:              "machete",
		Name:            "Team Machete",
		ZoneID:          "ironyard",
		HostileFactions: []string{},
		Tiers: []faction.FactionTier{
			{ID: "outsider", Label: "Outsider", MinRep: 0},
			{ID: "blade", Label: "Blade", MinRep: 100},
			{ID: "cutter", Label: "Cutter", MinRep: 300},
			{ID: "warsmith", Label: "Warsmith", MinRep: 600},
		},
	}
	ironclad := &faction.FactionDef{
		ID:              "ironclad",
		Name:            "Ironclad",
		ZoneID:          "ironyard",
		HostileFactions: []string{"machete"},
		Tiers: []faction.FactionTier{
			{ID: "outsider", Label: "Outsider", MinRep: 0},
			{ID: "recruit", Label: "Recruit", MinRep: 100},
			{ID: "soldier", Label: "Soldier", MinRep: 300},
			{ID: "commander", Label: "Commander", MinRep: 600},
		},
	}
	reg := faction.FactionRegistry{"machete": machete, "ironclad": ironclad}
	svc := faction.NewService(reg)

	sess := &session.PlayerSession{
		UID:        "player-1",
		FactionID:  "machete",
		FactionRep: map[string]int{"machete": 0},
	}

	inst := &npc.Instance{}
	inst.Disposition = "neutral"
	inst.FactionID = "ironclad"

	players := []*session.PlayerSession{sess}

	isHostileToPlayers := inst.Disposition == "hostile"
	if isHostileToPlayers && svc != nil && inst.FactionID != "" {
		for _, p := range players {
			if svc.IsAllyOf(p, inst.FactionID) {
				isHostileToPlayers = false
				break
			}
		}
	}
	if !isHostileToPlayers && svc != nil && inst.FactionID != "" {
		for _, p := range players {
			if svc.IsEnemyOf(p, inst.FactionID) {
				isHostileToPlayers = true
				break
			}
		}
	}

	if !isHostileToPlayers {
		t.Error("enemy-faction NPC (ironclad) should be hostile to machete player")
	}
}

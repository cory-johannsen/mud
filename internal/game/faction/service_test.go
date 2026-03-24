package faction_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

func makeTestRegistry() faction.FactionRegistry {
	gun := &faction.FactionDef{
		ID: "gun", Name: "Team Gun", ZoneID: "rustbucket",
		HostileFactions: []string{"machete"},
		Tiers: []faction.FactionTier{
			{ID: "outsider", Label: "Outsider", MinRep: 0, PriceDiscount: 0.0},
			{ID: "gunhand", Label: "Gunhand", MinRep: 100, PriceDiscount: 0.05},
			{ID: "sharpshooter", Label: "Sharpshooter", MinRep: 300, PriceDiscount: 0.10},
			{ID: "warchief", Label: "Warchief", MinRep: 600, PriceDiscount: 0.15},
		},
		ExclusiveItems: []faction.FactionExclusiveItems{
			{TierID: "gunhand", ItemIDs: []string{"railpistol"}},
		},
	}
	machete := &faction.FactionDef{
		ID: "machete", Name: "Team Machete", ZoneID: "ironyard",
		HostileFactions: []string{"gun"},
		Tiers: []faction.FactionTier{
			{ID: "outsider", Label: "Outsider", MinRep: 0, PriceDiscount: 0.0},
			{ID: "blade", Label: "Blade", MinRep: 100, PriceDiscount: 0.05},
			{ID: "cutter", Label: "Cutter", MinRep: 300, PriceDiscount: 0.10},
			{ID: "warsmith", Label: "Warsmith", MinRep: 600, PriceDiscount: 0.15},
		},
	}
	return faction.FactionRegistry{"gun": gun, "machete": machete}
}

func TestTierFor_ReturnsFirstTierAtZeroRep(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	tier := svc.TierFor("gun", 0)
	if tier == nil {
		t.Fatal("expected non-nil tier")
	}
	if tier.ID != "outsider" {
		t.Fatalf("expected 'outsider', got %q", tier.ID)
	}
}

func TestTierFor_ReturnsHighestQualifyingTier(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rep := rapid.IntRange(100, 299).Draw(t, "rep")
		svc := faction.NewService(makeTestRegistry())
		tier := svc.TierFor("gun", rep)
		if tier.ID != "gunhand" {
			t.Fatalf("rep=%d: expected 'gunhand', got %q", rep, tier.ID)
		}
	})
}

func TestTierFor_MaxTierAt600Plus(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rep := rapid.IntRange(600, 10000).Draw(t, "rep")
		svc := faction.NewService(makeTestRegistry())
		tier := svc.TierFor("gun", rep)
		if tier.ID != "warchief" {
			t.Fatalf("rep=%d: expected 'warchief', got %q", rep, tier.ID)
		}
	})
}

func TestIsHostile(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	if !svc.IsHostile("gun", "machete") {
		t.Error("gun should be hostile to machete")
	}
	if svc.IsHostile("gun", "gun") {
		t.Error("gun should not be hostile to itself")
	}
	if svc.IsHostile("gun", "unknown") {
		t.Error("gun should not be hostile to unknown faction")
	}
}

func TestDiscountFor_DelegatesToTierFor(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	d := svc.DiscountFor("gun", 150) // gunhand tier
	if d != 0.05 {
		t.Fatalf("expected 0.05, got %v", d)
	}
}

func TestIsEnemyOf_HostileNPCFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 0}}
	if !svc.IsEnemyOf(sess, "machete") {
		t.Error("machete NPC should be enemy of gun player")
	}
	if svc.IsEnemyOf(sess, "") {
		t.Error("empty NPC faction should never be enemy")
	}
	if svc.IsEnemyOf(sess, "gun") {
		t.Error("same-faction NPC should not be enemy")
	}
}

func TestCanEnterRoom_NoGating(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 0}}
	room := &world.Room{MinFactionTierID: ""}
	zone := &world.Zone{FactionID: "gun"}
	if !svc.CanEnterRoom(sess, room, zone) {
		t.Error("ungated room should always be enterable")
	}
}

func TestCanEnterRoom_WrongFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "machete", FactionRep: map[string]int{"machete": 600}}
	room := &world.Room{MinFactionTierID: "gunhand"}
	zone := &world.Zone{FactionID: "gun"}
	if svc.CanEnterRoom(sess, room, zone) {
		t.Error("machete player should not enter gun-gated room")
	}
}

func TestCanEnterRoom_InsufficientTier(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 50}} // outsider
	room := &world.Room{MinFactionTierID: "gunhand"}
	zone := &world.Zone{FactionID: "gun"}
	if svc.CanEnterRoom(sess, room, zone) {
		t.Error("outsider should not enter gunhand-gated room")
	}
}

func TestCanEnterRoom_SufficientTier(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 150}} // gunhand
	room := &world.Room{MinFactionTierID: "gunhand"}
	zone := &world.Zone{FactionID: "gun"}
	if !svc.CanEnterRoom(sess, room, zone) {
		t.Error("gunhand player should be able to enter gunhand-gated room")
	}
}

func TestCanBuyItem_ExclusiveItemLockedAtOutsider(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 0}} // outsider
	if svc.CanBuyItem(sess, "railpistol") {
		t.Error("outsider should not buy gunhand-exclusive item")
	}
}

func TestCanBuyItem_ExclusiveItemUnlockedAtTier(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 150}} // gunhand
	if !svc.CanBuyItem(sess, "railpistol") {
		t.Error("gunhand player should be able to buy railpistol")
	}
}

func TestCanBuyItem_NonExclusiveItem(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "gun", FactionRep: map[string]int{"gun": 0}}
	if !svc.CanBuyItem(sess, "some_normal_item") {
		t.Error("non-exclusive item should always be buyable")
	}
}

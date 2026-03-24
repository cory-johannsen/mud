package faction_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/faction"
)

func TestFactionDef_Validate_RequiresExactlyFourTiers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Filter(func(v int) bool { return v != 4 }).Draw(t, "n")
		tiers := make([]faction.FactionTier, n)
		for i := range tiers {
			tiers[i] = faction.FactionTier{ID: "t", Label: "L", MinRep: i * 10, PriceDiscount: 0.0}
		}
		if n > 0 {
			tiers[0].MinRep = 0
		}
		def := faction.FactionDef{ID: "f", Name: "N", ZoneID: "z", Tiers: tiers}
		if err := def.Validate(); err == nil {
			t.Fatalf("expected error for %d tiers, got nil", n)
		}
	})
}

func TestFactionDef_Validate_FirstTierMustBeZero(t *testing.T) {
	def := faction.FactionDef{
		ID: "f", Name: "N", ZoneID: "z",
		Tiers: []faction.FactionTier{
			{ID: "t1", Label: "L1", MinRep: 5, PriceDiscount: 0.0},
			{ID: "t2", Label: "L2", MinRep: 10, PriceDiscount: 0.05},
			{ID: "t3", Label: "L3", MinRep: 20, PriceDiscount: 0.1},
			{ID: "t4", Label: "L4", MinRep: 30, PriceDiscount: 0.15},
		},
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for first tier MinRep != 0")
	}
}

func TestFactionDef_Validate_StrictlyIncreasingMinRep(t *testing.T) {
	def := faction.FactionDef{
		ID: "f", Name: "N", ZoneID: "z",
		Tiers: []faction.FactionTier{
			{ID: "t1", Label: "L1", MinRep: 0, PriceDiscount: 0.0},
			{ID: "t2", Label: "L2", MinRep: 10, PriceDiscount: 0.05},
			{ID: "t3", Label: "L3", MinRep: 10, PriceDiscount: 0.1}, // duplicate
			{ID: "t4", Label: "L4", MinRep: 30, PriceDiscount: 0.15},
		},
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for non-strictly-increasing MinRep")
	}
}

func TestFactionDef_Validate_PriceDiscountRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bad := rapid.Float64Range(1.001, 10.0).Draw(t, "bad")
		def := faction.FactionDef{
			ID: "f", Name: "N", ZoneID: "z",
			Tiers: []faction.FactionTier{
				{ID: "t1", Label: "L1", MinRep: 0, PriceDiscount: 0.0},
				{ID: "t2", Label: "L2", MinRep: 10, PriceDiscount: 0.05},
				{ID: "t3", Label: "L3", MinRep: 20, PriceDiscount: bad},
				{ID: "t4", Label: "L4", MinRep: 30, PriceDiscount: 0.15},
			},
		}
		if err := def.Validate(); err == nil {
			t.Fatalf("expected error for PriceDiscount %v out of range", bad)
		}
	})
}

func TestFactionDef_Validate_InvalidTierIDReference(t *testing.T) {
	def := faction.FactionDef{
		ID: "f", Name: "N", ZoneID: "z",
		Tiers: []faction.FactionTier{
			{ID: "t1", Label: "L1", MinRep: 0, PriceDiscount: 0.0},
			{ID: "t2", Label: "L2", MinRep: 10, PriceDiscount: 0.05},
			{ID: "t3", Label: "L3", MinRep: 20, PriceDiscount: 0.1},
			{ID: "t4", Label: "L4", MinRep: 30, PriceDiscount: 0.15},
		},
		ExclusiveItems: []faction.FactionExclusiveItems{
			{TierID: "nonexistent", ItemIDs: []string{"item_x"}},
		},
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for unknown TierID in ExclusiveItems")
	}
}

func TestFactionDef_Validate_Happy(t *testing.T) {
	def := faction.FactionDef{
		ID: "gun", Name: "Team Gun", ZoneID: "rustbucket",
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
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactionConfig_Validate(t *testing.T) {
	cfg := faction.FactionConfig{
		RepPerNPCLevel:     5,
		RepPerFixerService: 50,
		RepChangeCosts:     map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg.RepPerNPCLevel = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero RepPerNPCLevel")
	}
}

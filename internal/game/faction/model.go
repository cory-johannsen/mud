package faction

import "fmt"

// FactionTier represents one reputation tier within a faction.
type FactionTier struct {
	ID            string  `yaml:"id"`
	Label         string  `yaml:"label"`
	MinRep        int     `yaml:"min_rep"`
	PriceDiscount float64 `yaml:"price_discount"`
}

// FactionExclusiveItems maps a tier to the item IDs exclusively available at that tier.
type FactionExclusiveItems struct {
	TierID  string   `yaml:"tier_id"`
	ItemIDs []string `yaml:"item_ids"`
}

// FactionGatedRoom maps a room to the minimum tier required to enter it.
type FactionGatedRoom struct {
	RoomID    string `yaml:"room_id"`
	MinTierID string `yaml:"min_tier_id"`
}

// FactionDef defines a faction loaded from content/factions/<id>.yaml.
type FactionDef struct {
	ID              string                  `yaml:"id"`
	Name            string                  `yaml:"name"`
	ZoneID          string                  `yaml:"zone_id"`
	HostileFactions []string                `yaml:"hostile_factions"`
	Tiers           []FactionTier           `yaml:"tiers"`
	ExclusiveItems  []FactionExclusiveItems `yaml:"exclusive_items"`
	GatedRooms      []FactionGatedRoom      `yaml:"gated_rooms"`
}

// Validate checks all structural invariants on the FactionDef.
//
// Precondition: none.
// Postcondition: Returns a non-nil error describing the first violation found.
func (f *FactionDef) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("faction ID must not be empty")
	}
	if f.Name == "" {
		return fmt.Errorf("faction %q: Name must not be empty", f.ID)
	}
	// ZoneID is optional; an empty string means the faction is zone-agnostic.
	if len(f.Tiers) != 4 {
		return fmt.Errorf("faction %q: must have exactly 4 tiers, got %d", f.ID, len(f.Tiers))
	}
	if f.Tiers[0].MinRep != 0 {
		return fmt.Errorf("faction %q: first tier MinRep must be 0, got %d", f.ID, f.Tiers[0].MinRep)
	}
	for i := 1; i < len(f.Tiers); i++ {
		if f.Tiers[i].MinRep <= f.Tiers[i-1].MinRep {
			return fmt.Errorf("faction %q: tier[%d].MinRep %d must be greater than tier[%d].MinRep %d",
				f.ID, i, f.Tiers[i].MinRep, i-1, f.Tiers[i-1].MinRep)
		}
	}
	tierIDs := make(map[string]bool, len(f.Tiers))
	for _, t := range f.Tiers {
		if t.PriceDiscount < 0 || t.PriceDiscount > 1 {
			return fmt.Errorf("faction %q: tier %q PriceDiscount %v is outside [0,1]", f.ID, t.ID, t.PriceDiscount)
		}
		tierIDs[t.ID] = true
	}
	for _, ei := range f.ExclusiveItems {
		if !tierIDs[ei.TierID] {
			return fmt.Errorf("faction %q: ExclusiveItems references unknown tier ID %q", f.ID, ei.TierID)
		}
	}
	for _, gr := range f.GatedRooms {
		if !tierIDs[gr.MinTierID] {
			return fmt.Errorf("faction %q: GatedRooms references unknown tier ID %q", f.ID, gr.MinTierID)
		}
	}
	return nil
}

// FactionConfig holds global faction economy parameters loaded from content/faction_config.yaml.
type FactionConfig struct {
	RepPerNPCLevel     int         `yaml:"rep_per_npc_level"`
	RepPerFixerService int         `yaml:"rep_per_fixer_service"`
	RepChangeCosts     map[int]int `yaml:"rep_change_costs"`
}

// Validate ensures all FactionConfig fields are positive.
//
// Precondition: none.
// Postcondition: Returns a non-nil error if any field is missing or <= 0.
func (c *FactionConfig) Validate() error {
	if c.RepPerNPCLevel <= 0 {
		return fmt.Errorf("faction_config: rep_per_npc_level must be > 0")
	}
	if c.RepPerFixerService <= 0 {
		return fmt.Errorf("faction_config: rep_per_fixer_service must be > 0")
	}
	for i := 1; i <= 4; i++ {
		v, ok := c.RepChangeCosts[i]
		if !ok || v <= 0 {
			return fmt.Errorf("faction_config: rep_change_costs[%d] must be > 0", i)
		}
	}
	return nil
}

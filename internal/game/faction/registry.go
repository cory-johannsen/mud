package faction

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FactionRegistry maps faction ID to its definition.
type FactionRegistry map[string]*FactionDef

// LoadFactions reads all *.yaml files in dir and returns a FactionRegistry.
//
// Precondition: dir must be a readable directory path.
// Postcondition: Returns a non-nil registry or a non-nil error.
func LoadFactions(dir string) (FactionRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("faction.LoadFactions: reading dir %q: %w", dir, err)
	}
	reg := make(FactionRegistry)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("faction.LoadFactions: reading %q: %w", e.Name(), err)
		}
		var def FactionDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("faction.LoadFactions: parsing %q: %w", e.Name(), err)
		}
		if err := def.Validate(); err != nil {
			return nil, fmt.Errorf("faction.LoadFactions: validating %q: %w", e.Name(), err)
		}
		reg[def.ID] = &def
	}
	return reg, nil
}

// Validate performs cross-registry checks against the live world and item data.
//
// Precondition: zoneIDs, roomIDs, itemIDs are non-nil sets of known IDs.
// roomZoneIDs maps roomID → zoneID. zoneOwners maps zoneID → faction ID (empty = unowned).
// Postcondition: Returns nil if all invariants hold, or a non-nil error on the first violation.
func (r FactionRegistry) Validate(zoneIDs, roomIDs, itemIDs map[string]bool, roomZoneIDs map[string]string, zoneOwners map[string]string) error {
	globalExclusiveItems := make(map[string]string) // itemID → factionID that owns it
	for id, def := range r {
		if !zoneIDs[def.ZoneID] {
			return fmt.Errorf("faction %q: ZoneID %q not found in world", id, def.ZoneID)
		}
		for _, hf := range def.HostileFactions {
			if _, ok := r[hf]; !ok {
				return fmt.Errorf("faction %q: HostileFaction %q not in registry", id, hf)
			}
		}
		for _, ei := range def.ExclusiveItems {
			for _, itemID := range ei.ItemIDs {
				if !itemIDs[itemID] {
					return fmt.Errorf("faction %q: ExclusiveItems item %q not in ItemRegistry", id, itemID)
				}
				if owner, conflict := globalExclusiveItems[itemID]; conflict {
					return fmt.Errorf("faction %q: item %q already claimed exclusively by faction %q", id, itemID, owner)
				}
				globalExclusiveItems[itemID] = id
			}
		}
		for _, gr := range def.GatedRooms {
			if !roomIDs[gr.RoomID] {
				return fmt.Errorf("faction %q: GatedRoom %q not found in world rooms", id, gr.RoomID)
			}
			zoneID, hasZone := roomZoneIDs[gr.RoomID]
			if !hasZone {
				return fmt.Errorf("faction %q: GatedRoom %q has no zone mapping", id, gr.RoomID)
			}
			ownerFactionID, owned := zoneOwners[zoneID]
			if !owned || ownerFactionID == "" {
				return fmt.Errorf("faction %q: GatedRoom %q is in zone %q which has no faction owner", id, gr.RoomID, zoneID)
			}
		}
	}
	return nil
}

// ByID returns the FactionDef for the given ID, or nil if not found.
func (r FactionRegistry) ByID(id string) *FactionDef {
	return r[id]
}

package inventory

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConsumableGrant is an item+quantity pair for starting consumables.
type ConsumableGrant struct {
	ItemID   string
	Quantity int
}

// InventoryItem represents a persisted backpack item (item def ID + quantity).
type InventoryItem struct {
	ItemDefID string
	Quantity  int
}

// StartingLoadout is the fully-merged starting kit for a character.
//
// Postcondition: All string fields are item IDs referencing content/items/.
type StartingLoadout struct {
	Weapon      string
	Armor       map[ArmorSlot]string
	Consumables []ConsumableGrant
	Currency    int
}

// StartingLoadoutOverride holds fields from a job's starting_inventory block.
// Only non-zero fields override the base+team loadout.
//
// YAML tags are defined so this struct can be embedded directly in Job YAML files.
// ArmorSlot is `type ArmorSlot string` so map[ArmorSlot]string round-trips cleanly.
type StartingLoadoutOverride struct {
	Weapon      string               `yaml:"weapon"`
	Armor       map[ArmorSlot]string `yaml:"armor"`
	Consumables []consumableEntry    `yaml:"consumables"`
	Currency    int                  `yaml:"currency"`
}

// archetypeLoadoutFile is the YAML structure for content/loadouts/<archetype>.yaml.
type archetypeLoadoutFile struct {
	Archetype   string       `yaml:"archetype"`
	Base        loadoutBlock `yaml:"base"`
	TeamGun     loadoutBlock `yaml:"team_gun"`
	TeamMachete loadoutBlock `yaml:"team_machete"`
}

type loadoutBlock struct {
	Weapon      string            `yaml:"weapon"`
	Armor       map[string]string `yaml:"armor"`
	Consumables []consumableEntry `yaml:"consumables"`
	Currency    int               `yaml:"currency"`
}

type consumableEntry struct {
	Item     string `yaml:"item"`
	Quantity int    `yaml:"quantity"`
}

// LoadStartingLoadout loads and merges the starting loadout for the given archetype and team.
//
// Precondition: dir must be a readable directory; archetype must be non-empty.
// Postcondition: Returns a merged StartingLoadout or an error if the archetype file is missing.
func LoadStartingLoadout(dir, archetype, team, _ string) (*StartingLoadout, error) {
	return LoadStartingLoadoutWithOverride(dir, archetype, team, nil)
}

// LoadStartingLoadoutWithOverride merges archetype base → team section → job override.
//
// Precondition: dir must be a readable directory; archetype must be non-empty.
// Postcondition: Returns a merged StartingLoadout or an error if the archetype file is missing.
func LoadStartingLoadoutWithOverride(dir, archetype, team string, override *StartingLoadoutOverride) (*StartingLoadout, error) {
	path := filepath.Join(dir, archetype+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading loadout for archetype %q: %w", archetype, err)
	}

	var af archetypeLoadoutFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("parsing loadout %q: %w", path, err)
	}

	// Start from base.
	sl := applyBlock(&StartingLoadout{Armor: make(map[ArmorSlot]string)}, af.Base)

	// Apply team section.
	switch team {
	case "gun":
		sl = applyBlock(sl, af.TeamGun)
	case "machete":
		sl = applyBlock(sl, af.TeamMachete)
	}

	// Apply job override.
	if override != nil {
		if override.Weapon != "" {
			sl.Weapon = override.Weapon
		}
		for slot, itemID := range override.Armor {
			sl.Armor[slot] = itemID
		}
		if len(override.Consumables) > 0 {
			sl.Consumables = make([]ConsumableGrant, len(override.Consumables))
			for i, c := range override.Consumables {
				sl.Consumables[i] = ConsumableGrant{ItemID: c.Item, Quantity: c.Quantity}
			}
		}
		if override.Currency != 0 {
			sl.Currency = override.Currency
		}
	}

	return sl, nil
}

func applyBlock(sl *StartingLoadout, b loadoutBlock) *StartingLoadout {
	if b.Weapon != "" {
		sl.Weapon = b.Weapon
	}
	for slotStr, itemID := range b.Armor {
		sl.Armor[ArmorSlot(slotStr)] = itemID
	}
	if len(b.Consumables) > 0 {
		sl.Consumables = make([]ConsumableGrant, len(b.Consumables))
		for i, c := range b.Consumables {
			sl.Consumables[i] = ConsumableGrant{ItemID: c.Item, Quantity: c.Quantity}
		}
	}
	if b.Currency != 0 {
		sl.Currency = b.Currency
	}
	return sl
}

package inventory

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// WeaponsDir is the path to weapon YAML definitions.
type WeaponsDir string

// ItemsDir is the path to item YAML definitions.
type ItemsDir string

// ExplosivesDir is the path to explosive YAML definitions.
type ExplosivesDir string

// ArmorsDir is the path to armor YAML definitions.
type ArmorsDir string

// PreciousMaterialsDir is the path to precious material YAML definitions.
type PreciousMaterialsDir string

// NewRegistryFromDirs loads all inventory definitions into a single Registry.
// condRegistry is used to validate disease_id/toxin_id references in consumable
// effects (REQ-EM-42) and MUST be loaded before this function is called.
func NewRegistryFromDirs(
	weaponsDir WeaponsDir,
	itemsDir ItemsDir,
	explosivesDir ExplosivesDir,
	armorsDir ArmorsDir,
	preciousMaterialsDir PreciousMaterialsDir,
	condRegistry *condition.Registry,
	logger *zap.Logger,
) (*Registry, error) {
	reg := NewRegistry()
	if weaponsDir != "" {
		weapons, err := LoadWeapons(string(weaponsDir))
		if err != nil {
			return nil, fmt.Errorf("loading weapons: %w", err)
		}
		for _, w := range weapons {
			if err := reg.RegisterWeapon(w); err != nil {
				return nil, fmt.Errorf("registering weapon %q: %w", w.ID, err)
			}
		}
		logger.Info("loaded weapon definitions", zap.Int("count", len(weapons)))
	}
	if explosivesDir != "" {
		explosives, err := LoadExplosives(string(explosivesDir))
		if err != nil {
			return nil, fmt.Errorf("loading explosives: %w", err)
		}
		for _, ex := range explosives {
			if err := reg.RegisterExplosive(ex); err != nil {
				return nil, fmt.Errorf("registering explosive %q: %w", ex.ID, err)
			}
		}
		logger.Info("loaded explosive definitions", zap.Int("count", len(explosives)))
	}
	if itemsDir != "" {
		items, err := LoadItems(string(itemsDir))
		if err != nil {
			return nil, fmt.Errorf("loading items: %w", err)
		}
		// REQ-EM-40: all six required consumable IDs must be present.
		if err := ValidateRequiredConsumables(items); err != nil {
			return nil, err
		}
		// REQ-EM-42: disease_id/toxin_id must reference known condition IDs.
		knownConds := make(map[string]bool)
		if condRegistry != nil {
			for _, cd := range condRegistry.All() {
				knownConds[cd.ID] = true
			}
		}
		if err := ValidateConsumableEffects(items, knownConds); err != nil {
			return nil, err
		}
		for _, item := range items {
			if err := reg.RegisterItem(item); err != nil {
				return nil, fmt.Errorf("registering item %q: %w", item.ID, err)
			}
		}
		logger.Info("loaded item definitions", zap.Int("count", len(items)))
	}
	armors, err := LoadArmors(string(armorsDir))
	if err != nil {
		return nil, fmt.Errorf("loading armors: %w", err)
	}
	for _, a := range armors {
		if err := reg.RegisterArmor(a); err != nil {
			return nil, fmt.Errorf("registering armor %q: %w", a.ID, err)
		}
	}
	logger.Info("loaded armor definitions", zap.Int("count", len(armors)))
	if preciousMaterialsDir != "" {
		if err := LoadPreciousMaterials(reg, string(preciousMaterialsDir)); err != nil {
			return nil, fmt.Errorf("loading precious materials: %w", err)
		}
		logger.Info("loaded precious material definitions", zap.Int("count", len(requiredMaterialIDs)*len(requiredGradeIDs)))
	}
	return reg, nil
}

// NewSeededRoomEquipmentManager creates a RoomEquipmentManager seeded with equipment from zone data.
func NewSeededRoomEquipmentManager(worldMgr *world.Manager, logger *zap.Logger) *RoomEquipmentManager {
	mgr := NewRoomEquipmentManager()
	for _, zone := range worldMgr.AllZones() {
		for _, room := range zone.Rooms {
			if len(room.Equipment) > 0 {
				mgr.InitRoom(room.ID, room.Equipment)
			}
		}
	}
	logger.Info("room equipment manager initialized")
	return mgr
}

// Providers is the wire provider set for inventory dependencies.
var Providers = wire.NewSet(
	NewRegistryFromDirs,
	NewFloorManager,
	NewSeededRoomEquipmentManager,
)

package world

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// WorldDir is the path to zone YAML files.
type WorldDir string

// NewManagerFromDir loads zones from dir and constructs a Manager.
func NewManagerFromDir(dir WorldDir, logger *zap.Logger) (*Manager, error) {
	zones, err := LoadZonesFromDir(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading zones from %q: %w", dir, err)
	}
	mgr, err := NewManager(zones)
	if err != nil {
		return nil, fmt.Errorf("creating world manager: %w", err)
	}
	if err := mgr.ValidateExits(); err != nil {
		return nil, fmt.Errorf("validating cross-zone exits: %w", err)
	}
	logger.Info("world loaded",
		zap.Int("zones", mgr.ZoneCount()),
		zap.Int("rooms", mgr.RoomCount()),
	)
	return mgr, nil
}

// Providers is the wire provider set for world dependencies.
var Providers = wire.NewSet(NewManagerFromDir)

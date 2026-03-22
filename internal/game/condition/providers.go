package condition

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// ConditionsDir is the path to condition YAML definitions.
type ConditionsDir string

// MentalConditionsDir is a fixed subdirectory always loaded alongside the main conditions directory.
type MentalConditionsDir string

// NewRegistryFromDir loads condition definitions from dir (and the mental subdir).
func NewRegistryFromDir(dir ConditionsDir, mentalDir MentalConditionsDir, logger *zap.Logger) (*Registry, error) {
	reg, err := LoadDirectory(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading conditions from %q: %w", dir, err)
	}
	mentalReg, err := LoadDirectory(string(mentalDir))
	if err != nil {
		return nil, fmt.Errorf("loading mental conditions from %q: %w", mentalDir, err)
	}
	for _, def := range mentalReg.All() {
		reg.Register(def)
	}
	logger.Info("loaded condition definitions", zap.Int("count", len(reg.All())))
	return reg, nil
}

// Providers is the wire provider set for condition dependencies.
var Providers = wire.NewSet(NewRegistryFromDir)

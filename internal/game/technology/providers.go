package technology

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// TechContentDir is the path to technology YAML content.
type TechContentDir string

// NewRegistryFromDir loads technology definitions; missing dir is a non-fatal warning.
func NewRegistryFromDir(dir TechContentDir, logger *zap.Logger) (*Registry, error) {
	reg, err := Load(string(dir))
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr.Err) {
			log.Printf("WARN: technology content dir %q not found — starting with empty tech registry", dir)
			return NewRegistry(), nil
		}
		return nil, fmt.Errorf("loading technology content: %w", err)
	}
	logger.Info("loaded technology definitions", zap.Int("count", len(reg.All())))
	return reg, nil
}

// Providers is the wire provider set for technology dependencies.
var Providers = wire.NewSet(NewRegistryFromDir)

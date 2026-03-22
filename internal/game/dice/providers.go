package dice

import (
	"github.com/google/wire"
	"go.uber.org/zap"
)

// NewCryptoRoller creates a logged dice roller using a crypto source.
func NewCryptoRoller(logger *zap.Logger) *Roller {
	return NewLoggedRoller(NewCryptoSource(), logger)
}

// Providers is the wire provider set for dice dependencies.
var Providers = wire.NewSet(NewCryptoRoller)

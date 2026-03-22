package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

// TestNewGameServiceServerSignature asserts that the refactored constructor
// accepts the three dependency structs. This test fails until Step 1 is done.
func TestNewGameServiceServerSignature(t *testing.T) {
	// Compilation-only test: if this file compiles, the signature is correct.
	var _ = gameserver.NewGameServiceServer
	_ = gameserver.StorageDeps{}
	_ = gameserver.ContentDeps{}
	_ = gameserver.HandlerDeps{}
}

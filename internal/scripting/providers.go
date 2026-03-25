package scripting

import (
	"os"
	"path/filepath"

	"github.com/google/wire"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/dice"
)

// ScriptRoot is the root directory for Lua scripts; empty disables scripting.
type ScriptRoot string

// CondScriptDir is the directory of global condition scripts.
type CondScriptDir string

// AIScriptDir is the path to Lua AI precondition scripts.
type AIScriptDir string

// NewManagerFromDirs constructs the scripting manager and loads global scripts.
// AI domain loading is performed separately by the gameserver package to avoid import cycles.
// If scriptRoot is empty, scripting is disabled (returns nil, nil).
//
// Zone scripts are loaded from <scriptRoot>/zones/<zoneID>/ directories (REQ-BUG15).
// Each subdirectory name becomes a zone VM key; all *.lua files within are executed.
func NewManagerFromDirs(
	scriptRoot ScriptRoot,
	condScriptDir CondScriptDir,
	aiScriptDir AIScriptDir,
	diceRoller *dice.Roller,
	logger *zap.Logger,
) (*Manager, error) {
	if scriptRoot == "" {
		return nil, nil
	}

	mgr := NewManager(diceRoller, logger)

	// Load global condition scripts.
	if info, err := os.Stat(string(condScriptDir)); err == nil && info.IsDir() {
		if err := mgr.LoadGlobal(string(condScriptDir), 0); err != nil {
			return nil, err
		}
		logger.Info("global condition scripts loaded", zap.String("dir", string(condScriptDir)))
	}

	// Load AI precondition scripts before registering domains.
	if aiScriptDir != "" {
		if _, statErr := os.Stat(string(aiScriptDir)); statErr == nil {
			if err := mgr.LoadGlobal(string(aiScriptDir), DefaultInstructionLimit); err != nil {
				return nil, err
			}
			logger.Info("loaded AI scripts", zap.String("dir", string(aiScriptDir)))
		}
	}

	// Load per-zone scripts from <scriptRoot>/zones/<zoneID>/.
	// Each immediate subdirectory name is used as the zone VM key.
	// Missing zones/ directory is not an error.
	zonesDir := filepath.Join(string(scriptRoot), "zones")
	entries, err := os.ReadDir(zonesDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			zoneID := e.Name()
			zoneScriptDir := filepath.Join(zonesDir, zoneID)
			if loadErr := mgr.LoadZone(zoneID, zoneScriptDir, DefaultInstructionLimit); loadErr != nil {
				return nil, loadErr
			}
			logger.Info("zone scripts loaded", zap.String("zone", zoneID), zap.String("dir", zoneScriptDir))
		}
	}

	return mgr, nil
}

// Providers is the wire provider set for scripting dependencies.
var Providers = wire.NewSet(NewManagerFromDirs)

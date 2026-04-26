package gameserver

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/skillaction"
)

// skillActionCatalog is a process-wide lazy-loaded catalog of ActionDefs read
// from content/skill_actions/. Lazy-loading keeps the Service constructor
// untouched while still letting handlers participate in the unified
// skillaction pipeline. The condition registry is supplied at first-use time
// so apply_condition.id references can be validated.
//
// Thread-safe: guards the load with a sync.Once.
var (
	skillActionOnce sync.Once
	skillActionDefs map[string]*skillaction.ActionDef
	skillActionErr  error
)

// loadSkillActions loads the catalog once and returns it. The first caller's
// condReg seeds the validation set. Subsequent calls reuse the cached map and
// ignore the supplied condReg.
//
// The directory is resolved by walking up from cwd until content/skill_actions
// is found; this matches the discovery patterns used elsewhere for content
// dirs and lets tests resolve the path regardless of cwd.
func loadSkillActions(condReg *condition.Registry) (map[string]*skillaction.ActionDef, error) {
	skillActionOnce.Do(func() {
		dir, err := findSkillActionsDir()
		if err != nil {
			skillActionErr = err
			return
		}
		skillActionDefs, skillActionErr = skillaction.LoadDirectory(dir, condReg)
	})
	return skillActionDefs, skillActionErr
}

// findSkillActionsDir walks up from the current working directory looking for
// content/skill_actions. Returns the absolute path or an error if not found.
func findSkillActionsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := cwd
	for {
		candidate := filepath.Join(cur, "content", "skill_actions")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Hit the filesystem root without finding the directory; fall
			// back to the relative path for callers running with a known cwd.
			return filepath.Join("content", "skill_actions"), nil
		}
		cur = parent
	}
}

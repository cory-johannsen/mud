package gameserver

import (
	"context"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// TechPromptFn presents a list of technology options to the player and returns
// the selected option string. Called only when pool size exceeds open slots.
type TechPromptFn func(options []string) (string, error)

// HardwiredTechRepo defines persistence for hardwired technology assignments.
type HardwiredTechRepo interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
	SetAll(ctx context.Context, characterID int64, techIDs []string) error
}

// PreparedTechRepo defines persistence for prepared technology slot assignments.
type PreparedTechRepo interface {
	GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error)
	Set(ctx context.Context, characterID int64, level, index int, techID string) error
	DeleteAll(ctx context.Context, characterID int64) error
}

// SpontaneousTechRepo defines persistence for spontaneous technology known-slot assignments.
type SpontaneousTechRepo interface {
	GetAll(ctx context.Context, characterID int64) (map[int][]string, error)
	Add(ctx context.Context, characterID int64, techID string, level int) error
	DeleteAll(ctx context.Context, characterID int64) error
}

// InnateTechRepo defines persistence for innate technology assignments.
type InnateTechRepo interface {
	GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)
	Set(ctx context.Context, characterID int64, techID string, maxUses int) error
	DeleteAll(ctx context.Context, characterID int64) error
}

// AssignTechnologies assigns technologies from job and archetype grants to the session
// and persists them. Called during character creation.
//
// Precondition: sess, job, hwRepo, prepRepo, spontRepo, innateRepo are not nil.
// techReg may be nil (tech names/descriptions will not be shown in prompts).
// archetype may be nil (innate technologies will not be assigned).
// Postcondition: If job.TechnologyGrants is nil, all session tech fields remain nil.
func AssignTechnologies(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	archetype *ruleset.Archetype,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
) error {
	if job == nil || job.TechnologyGrants == nil {
		return nil
	}
	grants := job.TechnologyGrants

	// Hardwired
	if len(grants.Hardwired) > 0 {
		sess.HardwiredTechs = append([]string(nil), grants.Hardwired...)
		if err := hwRepo.SetAll(ctx, characterID, sess.HardwiredTechs); err != nil {
			return fmt.Errorf("AssignTechnologies hardwired: %w", err)
		}
	}

	// Innate (from archetype)
	if archetype != nil && len(archetype.InnateTechnologies) > 0 {
		sess.InnateTechs = make(map[string]*session.InnateSlot)
		for _, grant := range archetype.InnateTechnologies {
			sess.InnateTechs[grant.ID] = &session.InnateSlot{MaxUses: grant.UsesPerDay}
			if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
				return fmt.Errorf("AssignTechnologies innate %s: %w", grant.ID, err)
			}
		}
	}

	// Prepared
	if grants.Prepared != nil {
		sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			chosen, err := fillFromPreparedPool(ctx, lvl, slots, grants.Prepared, techReg, promptFn, characterID, prepRepo)
			if err != nil {
				return fmt.Errorf("AssignTechnologies prepared level %d: %w", lvl, err)
			}
			sess.PreparedTechs[lvl] = chosen
		}
	}

	// Spontaneous
	if grants.Spontaneous != nil {
		sess.SpontaneousTechs = make(map[int][]string)
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			chosen, err := fillFromSpontaneousPool(ctx, lvl, known, grants.Spontaneous, techReg, promptFn, characterID, spontRepo)
			if err != nil {
				return fmt.Errorf("AssignTechnologies spontaneous level %d: %w", lvl, err)
			}
			sess.SpontaneousTechs[lvl] = chosen
		}
	}

	return nil
}

// LoadTechnologies loads technology assignments from the database into the session.
// Called at login after loadClassFeatures.
//
// Precondition: sess is not nil.
// Postcondition: All four session technology fields are populated from their respective repos.
func LoadTechnologies(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
) error {
	hw, err := hwRepo.GetAll(ctx, characterID)
	if err != nil {
		return fmt.Errorf("LoadTechnologies hardwired: %w", err)
	}
	sess.HardwiredTechs = hw

	prep, err := prepRepo.GetAll(ctx, characterID)
	if err != nil {
		return fmt.Errorf("LoadTechnologies prepared: %w", err)
	}
	sess.PreparedTechs = prep

	spont, err := spontRepo.GetAll(ctx, characterID)
	if err != nil {
		return fmt.Errorf("LoadTechnologies spontaneous: %w", err)
	}
	sess.SpontaneousTechs = spont

	innate, err := innateRepo.GetAll(ctx, characterID)
	if err != nil {
		return fmt.Errorf("LoadTechnologies innate: %w", err)
	}
	sess.InnateTechs = innate

	return nil
}

// fillFromPreparedPool fills prepared slots from fixed entries and optionally the pool.
// Auto-assigns without prompt when len(pool at level) == open slots.
func fillFromPreparedPool(
	ctx context.Context,
	lvl, slots int,
	grants *ruleset.PreparedGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo PreparedTechRepo,
) ([]*session.PreparedSlot, error) {
	result := make([]*session.PreparedSlot, 0, slots)
	idx := 0

	// Pre-fill from fixed entries at this level.
	for _, e := range grants.Fixed {
		if e.Level == lvl {
			slot := &session.PreparedSlot{TechID: e.ID}
			result = append(result, slot)
			if err := repo.Set(ctx, characterID, lvl, idx, e.ID); err != nil {
				return nil, err
			}
			idx++
		}
	}

	open := slots - len(result)
	if open <= 0 {
		return result, nil
	}

	// Collect pool entries at this level.
	var pool []ruleset.PreparedEntry
	for _, e := range grants.Pool {
		if e.Level == lvl {
			pool = append(pool, e)
		}
	}

	if len(pool) == open {
		// Auto-assign without prompt.
		for _, e := range pool {
			slot := &session.PreparedSlot{TechID: e.ID}
			result = append(result, slot)
			if err := repo.Set(ctx, characterID, lvl, idx, e.ID); err != nil {
				return nil, err
			}
			idx++
		}
		return result, nil
	}

	// Prompt player to choose from pool.
	remaining := make([]ruleset.PreparedEntry, len(pool))
	copy(remaining, pool)
	for open > 0 {
		options := buildPreparedOptions(remaining, techReg)
		chosen, err := promptFn(options)
		if err != nil {
			return nil, err
		}
		techID := parseTechID(chosen)
		slot := &session.PreparedSlot{TechID: techID}
		result = append(result, slot)
		if err := repo.Set(ctx, characterID, lvl, idx, techID); err != nil {
			return nil, err
		}
		idx++
		remaining = removePreparedByID(remaining, techID)
		open--
	}
	return result, nil
}

// fillFromSpontaneousPool fills spontaneous known slots from fixed entries and pool.
func fillFromSpontaneousPool(
	ctx context.Context,
	lvl, known int,
	grants *ruleset.SpontaneousGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo SpontaneousTechRepo,
) ([]string, error) {
	var result []string

	// Pre-fill from fixed entries at this level.
	for _, e := range grants.Fixed {
		if e.Level == lvl {
			result = append(result, e.ID)
			if err := repo.Add(ctx, characterID, e.ID, lvl); err != nil {
				return nil, err
			}
		}
	}

	open := known - len(result)
	if open <= 0 {
		return result, nil
	}

	// Collect pool entries at this level.
	var pool []ruleset.SpontaneousEntry
	for _, e := range grants.Pool {
		if e.Level == lvl {
			pool = append(pool, e)
		}
	}

	if len(pool) == open {
		// Auto-assign without prompt.
		for _, e := range pool {
			result = append(result, e.ID)
			if err := repo.Add(ctx, characterID, e.ID, lvl); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	// Prompt player to choose from pool.
	remaining := make([]ruleset.SpontaneousEntry, len(pool))
	copy(remaining, pool)
	for open > 0 {
		options := buildSpontaneousOptions(remaining, techReg)
		chosen, err := promptFn(options)
		if err != nil {
			return nil, err
		}
		techID := parseTechID(chosen)
		result = append(result, techID)
		if err := repo.Add(ctx, characterID, techID, lvl); err != nil {
			return nil, err
		}
		remaining = removeSpontaneousByID(remaining, techID)
		open--
	}
	return result, nil
}

func buildPreparedOptions(entries []ruleset.PreparedEntry, reg *technology.Registry) []string {
	opts := make([]string, len(entries))
	for i, e := range entries {
		if reg != nil {
			if def, ok := reg.Get(e.ID); ok {
				opts[i] = fmt.Sprintf("%s \u2014 %s", def.ID, def.Description)
				continue
			}
		}
		opts[i] = e.ID
	}
	return opts
}

func buildSpontaneousOptions(entries []ruleset.SpontaneousEntry, reg *technology.Registry) []string {
	opts := make([]string, len(entries))
	for i, e := range entries {
		if reg != nil {
			if def, ok := reg.Get(e.ID); ok {
				opts[i] = fmt.Sprintf("%s \u2014 %s", def.ID, def.Description)
				continue
			}
		}
		opts[i] = e.ID
	}
	return opts
}

// parseTechID extracts the tech ID from a display option string.
// If the option contains " — " (em-dash), the part before it is the ID.
func parseTechID(option string) string {
	for i := 0; i < len(option)-2; i++ {
		if option[i] == ' ' && option[i+1] == '\xe2' && i+3 < len(option) {
			// Check for UTF-8 em-dash (U+2014 = 0xE2 0x80 0x94) followed by space
			if option[i+1] == '\xe2' && option[i+2] == '\x80' && option[i+3] == '\x94' && i+4 < len(option) && option[i+4] == ' ' {
				return option[:i]
			}
		}
	}
	return option
}

func removePreparedByID(entries []ruleset.PreparedEntry, id string) []ruleset.PreparedEntry {
	for i, e := range entries {
		if e.ID == id {
			return append(entries[:i], entries[i+1:]...)
		}
	}
	return entries
}

func removeSpontaneousByID(entries []ruleset.SpontaneousEntry, id string) []ruleset.SpontaneousEntry {
	for i, e := range entries {
		if e.ID == id {
			return append(entries[:i], entries[i+1:]...)
		}
	}
	return entries
}

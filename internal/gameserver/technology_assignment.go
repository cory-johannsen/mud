package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	// SetExpended marks or unmarks a single prepared slot as expended.
	//
	// Precondition: characterID > 0; level >= 1; index >= 0.
	// Postcondition: character_prepared_technologies row has expended = expended.
	SetExpended(ctx context.Context, characterID int64, level, index int, expended bool) error
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

// PendingTechLevelsRepo persists the list of character levels with unresolved
// technology pool selections.
type PendingTechLevelsRepo interface {
	GetPendingTechLevels(ctx context.Context, characterID int64) ([]int, error)
	SetPendingTechLevels(ctx context.Context, characterID int64, levels []int) error
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
			chosen, err := fillFromPreparedPool(ctx, lvl, slots, 0, grants.Prepared, techReg, promptFn, characterID, prepRepo)
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

// LevelUpTechnologies applies a technology grants delta to an existing character's session
// and persists new slot assignments. Called once per character level gained.
//
// Precondition: grants must be nil or valid (validated at YAML load time).
// Postcondition: If grants is nil, returns nil with no changes (no-op).
// Otherwise sess and repos gain all new slots from grants; existing slots are unchanged.
// promptFn may be nil — if nil, the first available pool option is auto-selected.
func LevelUpTechnologies(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	grants *ruleset.TechnologyGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
) error {
	if grants == nil {
		return nil
	}
	// Use first-option fallback when no promptFn is provided (e.g., admin grant path).
	if promptFn == nil {
		promptFn = func(options []string) (string, error) {
			if len(options) == 0 {
				return "", nil
			}
			return options[0], nil
		}
	}

	// Hardwired: append new IDs, skipping duplicates (map-based, order-preserving).
	if len(grants.Hardwired) > 0 {
		existing := make(map[string]bool, len(sess.HardwiredTechs))
		for _, id := range sess.HardwiredTechs {
			existing[id] = true
		}
		for _, id := range grants.Hardwired {
			if !existing[id] {
				sess.HardwiredTechs = append(sess.HardwiredTechs, id)
				existing[id] = true
			}
		}
		if err := hwRepo.SetAll(ctx, characterID, sess.HardwiredTechs); err != nil {
			return fmt.Errorf("LevelUpTechnologies hardwired: %w", err)
		}
	}

	// Prepared: fill new slots starting after existing slot indices.
	// Existing slot slices are dense (no nil gaps), so len gives the correct next index.
	if grants.Prepared != nil {
		existingPrep, err := prepRepo.GetAll(ctx, characterID)
		if err != nil {
			return fmt.Errorf("LevelUpTechnologies prepared GetAll: %w", err)
		}
		if sess.PreparedTechs == nil {
			sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
		}
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			startIdx := len(existingPrep[lvl])
			chosen, err := fillFromPreparedPool(ctx, lvl, slots, startIdx, grants.Prepared, techReg, promptFn, characterID, prepRepo)
			if err != nil {
				return fmt.Errorf("LevelUpTechnologies prepared level %d: %w", lvl, err)
			}
			sess.PreparedTechs[lvl] = append(sess.PreparedTechs[lvl], chosen...)
		}
	}

	// Spontaneous: add new known techs without removing existing ones.
	if grants.Spontaneous != nil {
		if sess.SpontaneousTechs == nil {
			sess.SpontaneousTechs = make(map[int][]string)
		}
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			chosen, err := fillFromSpontaneousPool(ctx, lvl, known, grants.Spontaneous, techReg, promptFn, characterID, spontRepo)
			if err != nil {
				return fmt.Errorf("LevelUpTechnologies spontaneous level %d: %w", lvl, err)
			}
			sess.SpontaneousTechs[lvl] = append(sess.SpontaneousTechs[lvl], chosen...)
		}
	}

	// Innate: level-up grants do not assign innate technologies (archetype-only).

	return nil
}

// RearrangePreparedTechs deletes all existing prepared slots and re-fills them
// by aggregating grants from job.TechnologyGrants and all job.LevelUpGrants
// entries for levels 1..sess.Level.
//
// Precondition: sess, job, prepRepo are non-nil. promptFn must be non-nil.
// Postcondition: sess.PreparedTechs and prepRepo reflect the re-selected slots.
// If sess.PreparedTechs is empty or all level slot counts are zero, returns nil (no-op).
func RearrangePreparedTechs(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	prepRepo PreparedTechRepo,
) error {
	// Build SlotsByLevel from session (source of truth for slot counts).
	slotsByLevel := make(map[int]int)
	for lvl, slots := range sess.PreparedTechs {
		if len(slots) > 0 {
			slotsByLevel[lvl] = len(slots)
		}
	}
	// No-op guard must run before any mutation.
	if len(slotsByLevel) == 0 {
		return nil
	}

	// Aggregate Fixed and Pool from all applicable grants.
	var allFixed []ruleset.PreparedEntry
	var allPool []ruleset.PreparedEntry
	if job.TechnologyGrants != nil && job.TechnologyGrants.Prepared != nil {
		allFixed = append(allFixed, job.TechnologyGrants.Prepared.Fixed...)
		allPool = append(allPool, job.TechnologyGrants.Prepared.Pool...)
	}
	for lvl, grants := range job.LevelUpGrants {
		if lvl > sess.Level {
			continue
		}
		if grants != nil && grants.Prepared != nil {
			allFixed = append(allFixed, grants.Prepared.Fixed...)
			allPool = append(allPool, grants.Prepared.Pool...)
		}
	}

	merged := &ruleset.PreparedGrants{
		SlotsByLevel: slotsByLevel,
		Fixed:        allFixed,
		Pool:         allPool,
	}

	// Clear existing slots before re-filling.
	if err := prepRepo.DeleteAll(ctx, characterID); err != nil {
		return fmt.Errorf("RearrangePreparedTechs DeleteAll: %w", err)
	}
	sess.PreparedTechs = make(map[int][]*session.PreparedSlot)

	// Re-fill each level.
	for lvl, slots := range slotsByLevel {
		chosen, err := fillFromPreparedPool(ctx, lvl, slots, 0, merged, techReg, promptFn, characterID, prepRepo)
		if err != nil {
			return fmt.Errorf("RearrangePreparedTechs level %d: %w", lvl, err)
		}
		sess.PreparedTechs[lvl] = chosen
	}
	return nil
}

// PartitionTechGrants splits grants into immediate (no player choice needed) and
// deferred (pool > open slots, player must choose) parts.
//
// Precondition: grants is non-nil and valid.
// Postcondition: immediate + deferred together cover all grants in the input.
// Either return value may be nil if its category is empty.
func PartitionTechGrants(grants *ruleset.TechnologyGrants) (immediate, deferred *ruleset.TechnologyGrants) {
	var imm, def ruleset.TechnologyGrants

	// Hardwired: always immediate.
	if len(grants.Hardwired) > 0 {
		imm.Hardwired = append(imm.Hardwired, grants.Hardwired...)
	}

	// Prepared: partition per tech level.
	if grants.Prepared != nil {
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			nFixed := 0
			for _, e := range grants.Prepared.Fixed {
				if e.Level == lvl {
					nFixed++
				}
			}
			nPool := 0
			for _, e := range grants.Prepared.Pool {
				if e.Level == lvl {
					nPool++
				}
			}
			open := slots - nFixed
			if nPool <= open {
				if imm.Prepared == nil {
					imm.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
				}
				imm.Prepared.SlotsByLevel[lvl] = slots
				for _, e := range grants.Prepared.Fixed {
					if e.Level == lvl {
						imm.Prepared.Fixed = append(imm.Prepared.Fixed, e)
					}
				}
				for _, e := range grants.Prepared.Pool {
					if e.Level == lvl {
						imm.Prepared.Pool = append(imm.Prepared.Pool, e)
					}
				}
			} else {
				if def.Prepared == nil {
					def.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
				}
				def.Prepared.SlotsByLevel[lvl] = slots
				for _, e := range grants.Prepared.Fixed {
					if e.Level == lvl {
						def.Prepared.Fixed = append(def.Prepared.Fixed, e)
					}
				}
				for _, e := range grants.Prepared.Pool {
					if e.Level == lvl {
						def.Prepared.Pool = append(def.Prepared.Pool, e)
					}
				}
			}
		}
	}

	// Spontaneous: partition per tech level.
	if grants.Spontaneous != nil {
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			nFixed := 0
			for _, e := range grants.Spontaneous.Fixed {
				if e.Level == lvl {
					nFixed++
				}
			}
			nPool := 0
			for _, e := range grants.Spontaneous.Pool {
				if e.Level == lvl {
					nPool++
				}
			}
			open := known - nFixed
			if nPool <= open {
				if imm.Spontaneous == nil {
					imm.Spontaneous = &ruleset.SpontaneousGrants{KnownByLevel: make(map[int]int)}
				}
				imm.Spontaneous.KnownByLevel[lvl] = known
				for _, e := range grants.Spontaneous.Fixed {
					if e.Level == lvl {
						imm.Spontaneous.Fixed = append(imm.Spontaneous.Fixed, e)
					}
				}
				for _, e := range grants.Spontaneous.Pool {
					if e.Level == lvl {
						imm.Spontaneous.Pool = append(imm.Spontaneous.Pool, e)
					}
				}
			} else {
				if def.Spontaneous == nil {
					def.Spontaneous = &ruleset.SpontaneousGrants{KnownByLevel: make(map[int]int)}
				}
				def.Spontaneous.KnownByLevel[lvl] = known
				for _, e := range grants.Spontaneous.Fixed {
					if e.Level == lvl {
						def.Spontaneous.Fixed = append(def.Spontaneous.Fixed, e)
					}
				}
				for _, e := range grants.Spontaneous.Pool {
					if e.Level == lvl {
						def.Spontaneous.Pool = append(def.Spontaneous.Pool, e)
					}
				}
			}
		}
	}

	if len(imm.Hardwired) > 0 || imm.Prepared != nil || imm.Spontaneous != nil {
		immCopy := imm
		immediate = &immCopy
	}
	if def.Prepared != nil || def.Spontaneous != nil {
		defCopy := def
		deferred = &defCopy
	}
	return
}

// ResolvePendingTechGrants interactively resolves all pending tech grants for a session.
// For each entry in sess.PendingTechGrants (ascending level order), calls LevelUpTechnologies
// with a live promptFn. Removes each entry after successful resolution.
//
// Precondition: sess, progressRepo, and all repos are non-nil.
// Postcondition: sess.PendingTechGrants is empty on full success; partially cleared on error.
// SetPendingTechLevels is called after each resolved entry to keep the DB in sync.
func ResolvePendingTechGrants(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
	progressRepo PendingTechLevelsRepo,
) error {
	if len(sess.PendingTechGrants) == 0 {
		return nil
	}
	levels := make([]int, 0, len(sess.PendingTechGrants))
	for lvl := range sess.PendingTechGrants {
		levels = append(levels, lvl)
	}
	sort.Ints(levels)

	for _, lvl := range levels {
		grants := sess.PendingTechGrants[lvl]
		if err := LevelUpTechnologies(ctx, sess, characterID, grants, techReg, promptFn,
			hwRepo, prepRepo, spontRepo, innateRepo,
		); err != nil {
			return fmt.Errorf("ResolvePendingTechGrants level %d: %w", lvl, err)
		}
		delete(sess.PendingTechGrants, lvl)
		remaining := make([]int, 0, len(sess.PendingTechGrants))
		for k := range sess.PendingTechGrants {
			remaining = append(remaining, k)
		}
		sort.Ints(remaining)
		if err := progressRepo.SetPendingTechLevels(ctx, characterID, remaining); err != nil {
			return fmt.Errorf("ResolvePendingTechGrants SetPendingTechLevels: %w", err)
		}
	}
	return nil
}

// fillFromPreparedPool fills prepared slots from fixed entries and optionally the pool.
// Auto-assigns without prompt when len(pool at level) == open slots.
// Precondition: open is assumed >= 0, guaranteed by TechnologyGrants.Validate().
func fillFromPreparedPool(
	ctx context.Context,
	lvl, slots, startIdx int,
	grants *ruleset.PreparedGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo PreparedTechRepo,
) ([]*session.PreparedSlot, error) {
	result := make([]*session.PreparedSlot, 0, slots)
	idx := startIdx

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

// buildOptions formats a slice of tech IDs and levels into display option strings.
// When a registry is provided and has an entry for the ID, the format is "id — description".
// Otherwise the raw ID is used. The levels slice is kept for future use.
func buildOptions(ids []string, levels []int, reg *technology.Registry) []string {
	opts := make([]string, 0, len(ids))
	for i, id := range ids {
		_ = levels[i]
		if reg != nil {
			if def, ok := reg.Get(id); ok {
				desc := def.Description
				if desc == "" {
					desc = def.Name
				}
				opts = append(opts, fmt.Sprintf("%s \u2014 %s", id, desc))
				continue
			}
		}
		opts = append(opts, id)
	}
	return opts
}

func buildPreparedOptions(entries []ruleset.PreparedEntry, reg *technology.Registry) []string {
	ids := make([]string, len(entries))
	levels := make([]int, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
		levels[i] = e.Level
	}
	return buildOptions(ids, levels, reg)
}

func buildSpontaneousOptions(entries []ruleset.SpontaneousEntry, reg *technology.Registry) []string {
	ids := make([]string, len(entries))
	levels := make([]int, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
		levels[i] = e.Level
	}
	return buildOptions(ids, levels, reg)
}

// parseTechID extracts the tech ID from a display option string.
// If the option contains " — " (em-dash with surrounding spaces), the part before it is the ID.
func parseTechID(option string) string {
	id, _, _ := strings.Cut(option, " \u2014 ")
	return strings.TrimSpace(id)
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

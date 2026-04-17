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

// TechPromptFn presents a prompt header and list of technology options to the player and returns
// the selected option string.
type TechPromptFn func(prompt string, options []string) (string, error)

// keepSentinel is the option prefix used to offer "keep current" in pool slot prompts.
// Callers that see this prefix in the chosen string use the existing tech rather than parsing a new ID.
const keepSentinel = "[keep] "

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
	// DeleteAtSpellLevel removes all prepared slots at exactly the given spell level.
	//
	// Precondition: characterID > 0; spellLevel >= 1.
	// Postcondition: No rows with (character_id, slot_level = spellLevel) remain.
	DeleteAtSpellLevel(ctx context.Context, characterID int64, spellLevel int) error
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
	// GetAll returns all innate slots for the character.
	GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)

	// Set initializes or overwrites an innate slot entry.
	// Postcondition: row (characterID, techID) has max_uses=maxUses, uses_remaining=maxUses.
	// Precondition: only called at character creation or full re-assignment, never at login load.
	Set(ctx context.Context, characterID int64, techID string, maxUses int) error

	// DeleteAll removes all innate tech rows for the character.
	DeleteAll(ctx context.Context, characterID int64) error

	// Decrement atomically decrements uses_remaining by 1 if > 0.
	// Precondition: caller has verified UsesRemaining > 0 in session before calling.
	// Postcondition: uses_remaining = max(0, uses_remaining - 1).
	Decrement(ctx context.Context, characterID int64, techID string) error

	// RestoreAll sets uses_remaining = max_uses for all rows of this character.
	// Postcondition: all innate slots are at maximum uses.
	RestoreAll(ctx context.Context, characterID int64) error
}

// SpontaneousUsePoolRepo manages the daily use pool for spontaneous technologies.
//
// Precondition: characterID > 0; techLevel >= 1; uses >= 0.
type SpontaneousUsePoolRepo interface {
	// GetAll returns all use pools for the character.
	// Postcondition: returned map contains one UsePool per initialized tech level; returns an empty map (not nil) if no pools have been initialized.
	GetAll(ctx context.Context, characterID int64) (map[int]session.UsePool, error)

	// Set initializes or overwrites a pool entry.
	// Postcondition: row (characterID, techLevel) has uses_remaining=usesRemaining, max_uses=maxUses.
	Set(ctx context.Context, characterID int64, techLevel, usesRemaining, maxUses int) error

	// Decrement atomically decrements uses_remaining by 1 if > 0.
	// Precondition: caller has verified uses_remaining > 0 in session before calling.
	// Postcondition: uses_remaining = max(0, uses_remaining - 1).
	Decrement(ctx context.Context, characterID int64, techLevel int) error

	// RestoreAll sets uses_remaining = max_uses for all rows of this character.
	// Postcondition: all pools are at maximum.
	RestoreAll(ctx context.Context, characterID int64) error

	// RestorePartial restores each pool by floor(fraction * (max - current)) uses.
	// Precondition: fraction in [0.0, 1.0].
	// Postcondition: all pools for characterID have uses_remaining increased by the partial amount (capped at max_uses).
	RestorePartial(ctx context.Context, characterID int64, fraction float64) error

	// DeleteAll removes all pool entries for the character.
	DeleteAll(ctx context.Context, characterID int64) error
}

// PendingTechLevelsRepo persists the list of character levels with unresolved
// technology pool selections.
type PendingTechLevelsRepo interface {
	GetPendingTechLevels(ctx context.Context, characterID int64) ([]int, error)
	SetPendingTechLevels(ctx context.Context, characterID int64, levels []int) error
}

// PendingTechSlotsRepo persists L2+ technology slots awaiting trainer resolution.
//
// Precondition: characterID > 0; techLevel >= 2; tradition and usageType are non-empty.
type PendingTechSlotsRepo interface {
	// AddPendingTechSlot inserts a row with remaining=1, or increments remaining if row exists.
	// Postcondition: (characterID, charLevel, techLevel, tradition, usageType) row exists with remaining >= 1.
	AddPendingTechSlot(ctx context.Context, characterID int64, charLevel, techLevel int, tradition, usageType string) error

	// GetPendingTechSlots returns all pending slots for the character with remaining > 0.
	GetPendingTechSlots(ctx context.Context, characterID int64) ([]session.PendingTechSlot, error)

	// DecrementPendingTechSlot decrements remaining by 1. If remaining reaches 0, deletes the row.
	// Precondition: row exists and remaining > 0.
	DecrementPendingTechSlot(ctx context.Context, characterID int64, charLevel, techLevel int, tradition, usageType string) error

	// DeleteAllPendingTechSlots removes all rows for the character.
	DeleteAllPendingTechSlots(ctx context.Context, characterID int64) error
}

// AssignTechnologies assigns technologies from job and archetype grants to the session
// and persists them. Called during character creation.
//
// Precondition: sess, hwRepo, prepRepo, spontRepo, innateRepo are not nil.
// job, archetype, and region may be nil; nil values skip the corresponding grant blocks.
// techReg may be nil (tech names/descriptions will not be shown in prompts).
// Postcondition: If both archetype.TechnologyGrants and job.TechnologyGrants are nil,
// all session tech fields remain nil (innate assignment still proceeds).
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
	usePoolRepo SpontaneousUsePoolRepo,
	region *ruleset.Region,
) error {
	// Short-circuit if there is nothing to assign.
	// Archetype innate grants are processed below regardless of job/region.
	hasWork := job != nil || region != nil || (archetype != nil && len(archetype.InnateTechnologies) > 0)
	if !hasWork {
		return nil
	}

	var archetypeGrants *ruleset.TechnologyGrants
	if archetype != nil {
		archetypeGrants = archetype.TechnologyGrants
	}
	var jobGrants *ruleset.TechnologyGrants
	if job != nil {
		jobGrants = job.TechnologyGrants
	}
	grants := ruleset.MergeGrants(archetypeGrants, jobGrants)

	// Validate merged grants before processing.
	if grants != nil {
		if err := grants.Validate(); err != nil {
			jobID := ""
			if job != nil {
				jobID = job.ID
			}
			return fmt.Errorf("AssignTechnologies: invalid merged grants for job %s: %w", jobID, err)
		}
	}

	// Hardwired
	if grants != nil && len(grants.Hardwired) > 0 {
		sess.HardwiredTechs = append([]string(nil), grants.Hardwired...)
		if err := hwRepo.SetAll(ctx, characterID, sess.HardwiredTechs); err != nil {
			return fmt.Errorf("AssignTechnologies hardwired: %w", err)
		}
	}

	// isTechCapable is true when the archetype has a technology tradition.
	// Only tech-capable characters receive innate technology grants (cantrip parity).
	isTechCapable := archetype != nil && technology.DominantTradition(archetype.ID) != ""

	// Innate: initialize map once before both archetype and region blocks
	if sess.InnateTechs == nil && isTechCapable {
		if len(archetype.InnateTechnologies) > 0 ||
			(region != nil && len(region.InnateTechnologies) > 0) {
			sess.InnateTechs = make(map[string]*session.InnateSlot)
		}
	}

	// Innate (from archetype)
	if isTechCapable {
		for _, grant := range archetype.InnateTechnologies {
			sess.InnateTechs[grant.ID] = &session.InnateSlot{
				MaxUses:       grant.UsesPerDay,
				UsesRemaining: grant.UsesPerDay,
			}
			if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
				return fmt.Errorf("AssignTechnologies innate (archetype) %s: %w", grant.ID, err)
			}
		}
	}

	// Innate (from region)
	if region != nil && isTechCapable {
		for _, grant := range region.InnateTechnologies {
			sess.InnateTechs[grant.ID] = &session.InnateSlot{
				MaxUses:       grant.UsesPerDay,
				UsesRemaining: grant.UsesPerDay,
			}
			if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
				return fmt.Errorf("AssignTechnologies innate (region) %s: %w", grant.ID, err)
			}
		}
	}

	// Prepared
	if grants != nil && grants.Prepared != nil {
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
	if grants != nil && grants.Spontaneous != nil {
		sess.SpontaneousTechs = make(map[int][]string)
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			chosen, err := fillFromSpontaneousPool(ctx, lvl, known, grants.Spontaneous, techReg, promptFn, characterID, spontRepo)
			if err != nil {
				return fmt.Errorf("AssignTechnologies spontaneous level %d: %w", lvl, err)
			}
			sess.SpontaneousTechs[lvl] = chosen
		}
		if sess.SpontaneousUsePools == nil {
			sess.SpontaneousUsePools = make(map[int]session.UsePool)
		}
		for level, uses := range grants.Spontaneous.UsesByLevel {
			sess.SpontaneousUsePools[level] = session.UsePool{Remaining: uses, Max: uses}
			if usePoolRepo != nil {
				if err := usePoolRepo.Set(ctx, characterID, level, uses, uses); err != nil {
					return fmt.Errorf("AssignTechnologies: set spontaneous use pool level %d: %w", level, err)
				}
			}
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
	usePoolRepo SpontaneousUsePoolRepo,
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

	if usePoolRepo != nil {
		pools, err := usePoolRepo.GetAll(ctx, characterID)
		if err != nil {
			return fmt.Errorf("LoadTechnologies: load spontaneous use pools: %w", err)
		}
		sess.SpontaneousUsePools = pools
	}

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
	usePoolRepo SpontaneousUsePoolRepo,
) error {
	if grants == nil {
		return nil
	}
	// Use first-option fallback when no promptFn is provided (e.g., admin grant path).
	if promptFn == nil {
		promptFn = func(_ string, options []string) (string, error) {
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
		if sess.SpontaneousUsePools == nil {
			sess.SpontaneousUsePools = make(map[int]session.UsePool)
		}
		for level, uses := range grants.Spontaneous.UsesByLevel {
			existing := sess.SpontaneousUsePools[level]
			newMax := existing.Max + uses
			newRemaining := existing.Remaining + uses
			sess.SpontaneousUsePools[level] = session.UsePool{Remaining: newRemaining, Max: newMax}
			if usePoolRepo != nil {
				if err := usePoolRepo.Set(ctx, characterID, level, newRemaining, newMax); err != nil {
					return fmt.Errorf("LevelUpTechnologies: set spontaneous use pool level %d: %w", level, err)
				}
			}
		}
	}

	// Innate: level-up grants do not assign innate technologies (archetype-only).

	return nil
}

// RearrangePreparedTechs deletes all existing prepared slots and re-fills them
// by aggregating grants from job.TechnologyGrants, archetype.TechnologyGrants,
// job.LevelUpGrants, and archetype.LevelUpGrants for levels 1..sess.Level.
//
// Precondition: sess, job, prepRepo are non-nil. promptFn must be non-nil.
// archetype may be nil (skips archetype pool entries).
// Postcondition: sess.PreparedTechs and prepRepo reflect the re-selected slots.
// If sess.PreparedTechs is empty (no slots in DB yet), returns nil (no-op).
// Otherwise all slot levels from grants are offered, including L2+ deferred levels
// not yet DB-filled (pending trainer), so every earned slot is re-selectable at rest.
func RearrangePreparedTechs(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	archetype *ruleset.Archetype,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	prepRepo PreparedTechRepo,
	sendFn func(string),
	flavor technology.TraditionFlavor,
) error {
	// Aggregate Fixed and Pool from all applicable grants (job + archetype, creation + level-up).
	// Also derive grant-based slot counts so that deferred L2+ slots (stored in PendingTechGrants
	// until trainer-resolved) are included. The final slotsByLevel is the per-level max of
	// grant-derived and session-derived counts, so existing session slots are never silently dropped.
	grantSlots := make(map[int]int)
	var allFixed []ruleset.PreparedEntry
	var allPool []ruleset.PreparedEntry
	if job.TechnologyGrants != nil && job.TechnologyGrants.Prepared != nil {
		for techLvl, cnt := range job.TechnologyGrants.Prepared.SlotsByLevel {
			grantSlots[techLvl] += cnt
		}
		allFixed = append(allFixed, job.TechnologyGrants.Prepared.Fixed...)
		allPool = append(allPool, job.TechnologyGrants.Prepared.Pool...)
	}
	if archetype != nil && archetype.TechnologyGrants != nil && archetype.TechnologyGrants.Prepared != nil {
		for techLvl, cnt := range archetype.TechnologyGrants.Prepared.SlotsByLevel {
			grantSlots[techLvl] += cnt
		}
		allFixed = append(allFixed, archetype.TechnologyGrants.Prepared.Fixed...)
		allPool = append(allPool, archetype.TechnologyGrants.Prepared.Pool...)
	}
	for grantLvl, grants := range job.LevelUpGrants {
		if grantLvl > sess.Level {
			continue
		}
		if grants != nil && grants.Prepared != nil {
			for techLvl, cnt := range grants.Prepared.SlotsByLevel {
				grantSlots[techLvl] += cnt
			}
			allFixed = append(allFixed, grants.Prepared.Fixed...)
			allPool = append(allPool, grants.Prepared.Pool...)
		}
	}
	if archetype != nil {
		for grantLvl, grants := range archetype.LevelUpGrants {
			if grantLvl > sess.Level {
				continue
			}
			if grants != nil && grants.Prepared != nil {
				for techLvl, cnt := range grants.Prepared.SlotsByLevel {
					grantSlots[techLvl] += cnt
				}
				allFixed = append(allFixed, grants.Prepared.Fixed...)
				allPool = append(allPool, grants.Prepared.Pool...)
			}
		}
	}

	// Build final slotsByLevel as per-level max(grantSlots, sessionSlots).
	// Grant count is authoritative for deferred levels not yet in the DB;
	// session count ensures existing slots are not dropped if grants change.
	slotsByLevel := make(map[int]int)
	for lvl, cnt := range grantSlots {
		slotsByLevel[lvl] = cnt
	}
	for lvl, slots := range sess.PreparedTechs {
		if cnt := len(slots); cnt > slotsByLevel[lvl] {
			slotsByLevel[lvl] = cnt
		}
	}

	// No-op guard: skip rearrangement if the player has no prepared slots at all.
	// A player with no slots in the DB hasn't resolved their initial grants via trainer yet;
	// rest is not the mechanism for first-time assignment.
	// Note: slotsByLevel may include grant-derived levels not in sess.PreparedTechs, so we
	// use sess.PreparedTechs as the guard rather than slotsByLevel.
	if len(sess.PreparedTechs) == 0 {
		return nil
	}

	send := func(msg string) {
		if sendFn != nil {
			sendFn(msg)
		}
	}

	send(fmt.Sprintf("%s %s...", flavor.PrepGerund, flavor.LoadoutTitle))

	merged := &ruleset.PreparedGrants{
		SlotsByLevel: slotsByLevel,
		Fixed:        allFixed,
		Pool:         allPool,
	}

	// Snapshot current assignments before clearing — used to offer "keep" options during re-selection.
	prevSlots := make(map[int][]*session.PreparedSlot, len(sess.PreparedTechs))
	for lvl, slots := range sess.PreparedTechs {
		cp := make([]*session.PreparedSlot, len(slots))
		copy(cp, slots)
		prevSlots[lvl] = cp
	}

	// Clear existing slots before re-filling.
	if err := prepRepo.DeleteAll(ctx, characterID); err != nil {
		return fmt.Errorf("RearrangePreparedTechs DeleteAll: %w", err)
	}
	sess.PreparedTechs = make(map[int][]*session.PreparedSlot)

	// Re-fill each level.
	for lvl, slots := range slotsByLevel {
		chosen, err := fillFromPreparedPoolWithSend(ctx, lvl, slots, 0, merged, techReg, promptFn, characterID, prepRepo, send, prevSlots[lvl], flavor)
		if err != nil {
			return fmt.Errorf("RearrangePreparedTechs level %d: %w", lvl, err)
		}
		sess.PreparedTechs[lvl] = chosen
	}
	return nil
}

// FilterGrantsByMaxTechLevel returns a copy of grants containing only tech entries
// at or below maxLevel. Hardwired entries are always included.
// Returns nil if nothing remains after filtering.
//
// Precondition: maxLevel >= 1.
// Postcondition: All returned slots and pool entries have Level <= maxLevel.
func FilterGrantsByMaxTechLevel(grants *ruleset.TechnologyGrants, maxLevel int) *ruleset.TechnologyGrants {
	if grants == nil {
		return nil
	}
	var result ruleset.TechnologyGrants
	result.Hardwired = append(result.Hardwired, grants.Hardwired...)

	if grants.Prepared != nil {
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			if lvl > maxLevel {
				continue
			}
			if result.Prepared == nil {
				result.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
			}
			result.Prepared.SlotsByLevel[lvl] = slots
			for _, e := range grants.Prepared.Fixed {
				if e.Level == lvl {
					result.Prepared.Fixed = append(result.Prepared.Fixed, e)
				}
			}
			for _, e := range grants.Prepared.Pool {
				if e.Level == lvl {
					result.Prepared.Pool = append(result.Prepared.Pool, e)
				}
			}
		}
	}

	if grants.Spontaneous != nil {
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			if lvl > maxLevel {
				continue
			}
			if result.Spontaneous == nil {
				result.Spontaneous = &ruleset.SpontaneousGrants{
					KnownByLevel: make(map[int]int),
					UsesByLevel:  make(map[int]int),
				}
			}
			result.Spontaneous.KnownByLevel[lvl] = known
			if grants.Spontaneous.UsesByLevel != nil {
				result.Spontaneous.UsesByLevel[lvl] = grants.Spontaneous.UsesByLevel[lvl]
			}
			for _, e := range grants.Spontaneous.Fixed {
				if e.Level == lvl {
					result.Spontaneous.Fixed = append(result.Spontaneous.Fixed, e)
				}
			}
			for _, e := range grants.Spontaneous.Pool {
				if e.Level == lvl {
					result.Spontaneous.Pool = append(result.Spontaneous.Pool, e)
				}
			}
		}
	}

	if len(result.Hardwired) == 0 && result.Prepared == nil && result.Spontaneous == nil {
		return nil
	}
	return &result
}

// filterGrantsByMinTechLevel returns grants containing only tech slots at or above minLevel.
// Hardwired entries are not included (they are always immediate).
// Returns nil if nothing remains.
//
// Precondition: minLevel >= 1.
// Postcondition: All returned slots have Level >= minLevel.
func filterGrantsByMinTechLevel(grants *ruleset.TechnologyGrants, minLevel int) *ruleset.TechnologyGrants {
	if grants == nil {
		return nil
	}
	var result ruleset.TechnologyGrants

	if grants.Prepared != nil {
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			if lvl < minLevel {
				continue
			}
			if result.Prepared == nil {
				result.Prepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
			}
			result.Prepared.SlotsByLevel[lvl] = slots
			for _, e := range grants.Prepared.Fixed {
				if e.Level == lvl {
					result.Prepared.Fixed = append(result.Prepared.Fixed, e)
				}
			}
			for _, e := range grants.Prepared.Pool {
				if e.Level == lvl {
					result.Prepared.Pool = append(result.Prepared.Pool, e)
				}
			}
		}
	}

	if grants.Spontaneous != nil {
		for lvl, known := range grants.Spontaneous.KnownByLevel {
			if lvl < minLevel {
				continue
			}
			if result.Spontaneous == nil {
				result.Spontaneous = &ruleset.SpontaneousGrants{
					KnownByLevel: make(map[int]int),
					UsesByLevel:  make(map[int]int),
				}
			}
			result.Spontaneous.KnownByLevel[lvl] = known
			if grants.Spontaneous.UsesByLevel != nil {
				result.Spontaneous.UsesByLevel[lvl] = grants.Spontaneous.UsesByLevel[lvl]
			}
			for _, e := range grants.Spontaneous.Fixed {
				if e.Level == lvl {
					result.Spontaneous.Fixed = append(result.Spontaneous.Fixed, e)
				}
			}
			for _, e := range grants.Spontaneous.Pool {
				if e.Level == lvl {
					result.Spontaneous.Pool = append(result.Spontaneous.Pool, e)
				}
			}
		}
	}

	if result.Prepared == nil && result.Spontaneous == nil {
		return nil
	}
	return &result
}

// PartitionTechGrants splits grants into immediate (no player choice needed) and
// deferred (pool > open slots, player must choose) parts.
//
// Precondition: grants is non-nil and valid.
// Postcondition: immediate + deferred together cover all grants in the input.
// Either return value may be nil if its category is empty.
// REQ-TTA-2: Prepared and spontaneous grants at tech level >= 2 are unconditionally deferred
// (require a trainer); level 1 uses the existing pool-vs-slots partitioning logic.
func PartitionTechGrants(grants *ruleset.TechnologyGrants) (immediate, deferred *ruleset.TechnologyGrants) {
	var imm, def ruleset.TechnologyGrants

	// Hardwired: always immediate.
	if len(grants.Hardwired) > 0 {
		imm.Hardwired = append(imm.Hardwired, grants.Hardwired...)
	}

	// Prepared: partition per tech level.
	if grants.Prepared != nil {
		for lvl, slots := range grants.Prepared.SlotsByLevel {
			// REQ-TTA-2: L2+ always require a trainer — unconditionally deferred.
			if lvl >= 2 {
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
				continue
			}
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
			// REQ-TTA-2: L2+ always require a trainer — unconditionally deferred.
			if lvl >= 2 {
				if def.Spontaneous == nil {
					def.Spontaneous = &ruleset.SpontaneousGrants{
						KnownByLevel: make(map[int]int),
						UsesByLevel:  make(map[int]int),
					}
				}
				def.Spontaneous.KnownByLevel[lvl] = known
				if grants.Spontaneous.UsesByLevel != nil {
					def.Spontaneous.UsesByLevel[lvl] = grants.Spontaneous.UsesByLevel[lvl]
				}
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
				continue
			}
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
					imm.Spontaneous = &ruleset.SpontaneousGrants{
						KnownByLevel: make(map[int]int),
						UsesByLevel:  make(map[int]int),
					}
				}
				imm.Spontaneous.KnownByLevel[lvl] = known
				if grants.Spontaneous.UsesByLevel != nil {
					imm.Spontaneous.UsesByLevel[lvl] = grants.Spontaneous.UsesByLevel[lvl]
				}
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
					def.Spontaneous = &ruleset.SpontaneousGrants{
						KnownByLevel: make(map[int]int),
						UsesByLevel:  make(map[int]int),
					}
				}
				def.Spontaneous.KnownByLevel[lvl] = known
				if grants.Spontaneous.UsesByLevel != nil {
					def.Spontaneous.UsesByLevel[lvl] = grants.Spontaneous.UsesByLevel[lvl]
				}
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
		immediate = new(imm)
	}
	if def.Prepared != nil || def.Spontaneous != nil {
		deferred = new(def)
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
	usePoolRepo SpontaneousUsePoolRepo,
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

		// Split: only auto-resolve L1. L2+ requires a trainer (REQ-TTA-2).
		l1Grants := FilterGrantsByMaxTechLevel(grants, 1)
		l2Grants := filterGrantsByMinTechLevel(grants, 2)

		if l1Grants != nil {
			if err := LevelUpTechnologies(ctx, sess, characterID, l1Grants, techReg, promptFn,
				hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo,
			); err != nil {
				return fmt.Errorf("ResolvePendingTechGrants level %d (L1): %w", lvl, err)
			}
		}

		if l2Grants != nil {
			// Keep L2+ in PendingTechGrants for trainer resolution.
			sess.PendingTechGrants[lvl] = l2Grants
		} else {
			// All grants at this char level are resolved — remove from pending.
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
	}
	return nil
}

// fillFromPreparedPoolWithSend fills prepared slots like fillFromPreparedPool but emits
// per-slot progress messages via send using the provided flavor strings.
// It always prompts the player for each pool slot (no auto-assign shortcut) so that
// all slots are visible and changeable during rearrangement.
// When prevSlots is non-nil and the previously assigned tech is still in the available pool,
// a "[keep]" option is prepended so the player can retain their current selection.
//
// Precondition: send is non-nil; flavor fields are used for SlotNoun display.
// Postcondition: All slots (fixed + pool) are filled; pool slots always prompt the player.
func fillFromPreparedPoolWithSend(
	ctx context.Context,
	lvl, slots, startIdx int,
	grants *ruleset.PreparedGrants,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	characterID int64,
	repo PreparedTechRepo,
	send func(string),
	prevSlots []*session.PreparedSlot,
	flavor technology.TraditionFlavor,
) ([]*session.PreparedSlot, error) {
	result := make([]*session.PreparedSlot, 0, slots)
	idx := startIdx

	// Pre-fill from fixed entries at this level.
	for _, e := range grants.Fixed {
		if e.Level == lvl {
			slotNum := idx - startIdx + 1
			send(fmt.Sprintf("Level %d, %s %d (fixed): %s", lvl, flavor.SlotNoun, slotNum, e.ID))
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

	// Build a set of pool IDs for fast lookup (used to gate the "keep" option).
	poolIDSet := make(map[string]bool, len(pool))
	for _, e := range pool {
		poolIDSet[e.ID] = true
	}

	// fixedCount is the number of fixed slots filled above; pool slots start at this offset.
	fixedCount := len(result)

	// Prompt player to choose from pool for every open slot.
	// The auto-assign shortcut (pool size == open slots) is intentionally omitted here
	// so the player can review and optionally change each slot during rearrangement.
	remaining := make([]ruleset.PreparedEntry, len(pool))
	copy(remaining, pool)
	poolSlotI := 0
	for open > 0 {
		slotNum := idx - startIdx + 1
		send(fmt.Sprintf("Level %d, %s %d of %d: choose from pool", lvl, flavor.SlotNoun, slotNum, slots))

		// Build option list; prepend a "keep" option when the current tech is still available.
		options := buildPreparedOptions(remaining, techReg)
		prevSlotIdx := fixedCount + poolSlotI
		var prevTechID string
		if prevSlots != nil && prevSlotIdx < len(prevSlots) && prevSlots[prevSlotIdx] != nil {
			prevTechID = prevSlots[prevSlotIdx].TechID
		}
		if prevTechID != "" && poolIDSet[prevTechID] {
			keepName := prevTechID
			if techReg != nil {
				if def, ok := techReg.Get(prevTechID); ok {
					keepName = def.Name
				}
			}
			// Remove prevTechID from the regular pool options before prepending the keep
			// option so the same tech is not offered twice (REQ-BUG101-1).
			for i, opt := range options {
				if parseTechID(opt) == prevTechID {
					options = append(options[:i], options[i+1:]...)
					break
				}
			}
			options = append([]string{keepSentinel + "Keep current: " + keepName}, options...)
		}

		slotPrompt := fmt.Sprintf("Choose a Level %d technology to prepare (%s %d of %d):", lvl, flavor.SlotNoun, slotNum, slots)
		chosen, err := promptFn(slotPrompt, options)
		if err != nil {
			return nil, err
		}

		var techID string
		if strings.HasPrefix(chosen, keepSentinel) {
			techID = prevTechID
		} else {
			techID = parseTechID(chosen)
		}

		slot := &session.PreparedSlot{TechID: techID}
		result = append(result, slot)
		if err := repo.Set(ctx, characterID, lvl, idx, techID); err != nil {
			return nil, err
		}
		idx++
		remaining = removePreparedByID(remaining, techID)
		open--
		poolSlotI++
	}
	return result, nil
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
		chosen, err := promptFn(fmt.Sprintf("Choose a Level %d technology to prepare:", lvl), options)
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
		chosen, err := promptFn(fmt.Sprintf("Choose a Level %d technology to learn:", lvl), options)
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
// When a registry is provided and has an entry for the ID, the format is "[id] Name — description".
// Otherwise the raw ID is used. The levels slice is kept for future use.
func buildOptions(ids []string, levels []int, reg *technology.Registry) []string {
	opts := make([]string, 0, len(ids))
	for i, id := range ids {
		lvl := 0
		if i < len(levels) {
			lvl = levels[i]
		}
		if reg != nil {
			if def, ok := reg.Get(id); ok {
				desc := def.Description
				if desc == "" {
					desc = def.Name
				}
				var nameWithLevel string
				if lvl > 0 {
					nameWithLevel = fmt.Sprintf("%s (Lv %d)", def.Name, lvl)
				} else {
					nameWithLevel = def.Name
				}
				opts = append(opts, fmt.Sprintf("[%s] %s \u2014 %s", id, nameWithLevel, desc))
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
// If the option starts with "[", the ID is the text between "[" and "]".
// Otherwise, if the option contains " — " (em-dash with surrounding spaces), the part before it is the ID.
// Falls back to returning the full trimmed option when neither delimiter is present.
func parseTechID(option string) string {
	if strings.HasPrefix(option, "[") {
		end := strings.Index(option, "]")
		if end > 1 {
			return option[1:end]
		}
	}
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

// BackfillLevelUpTechnologies applies any technology level-up grants the player
// should have earned for levels 2..sess.Level but does not yet have. It is safe
// to call on every login: expected vs. actual slot counts are compared and only
// missing grants are applied. Grants are auto-assigned (first-option fallback).
//
// Precondition: characterID > 0; sess.Level >= 1; prepRepo non-nil.
// Postcondition: missing prepared slots are filled and persisted; missing
// hardwired IDs are appended and persisted.
func BackfillLevelUpTechnologies(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	archetype *ruleset.Archetype,
	mergedLevelUpGrants map[int]*ruleset.TechnologyGrants,
	techReg *technology.Registry,
	hwRepo HardwiredTechRepo,
	prepRepo PreparedTechRepo,
	spontRepo SpontaneousTechRepo,
	innateRepo InnateTechRepo,
	usePoolRepo SpontaneousUsePoolRepo,
) (*ruleset.TechnologyGrants, error) {
	if characterID == 0 || sess.Level < 2 || len(mergedLevelUpGrants) == 0 {
		return nil, nil
	}

	// pendingPrepared accumulates L2+ prepared slots that require a trainer visit.
	// REQ-TTA-2: L2+ prepared slots are NEVER auto-assigned; they must go through the
	// trainer system. Any existing pool-assigned L2+ slots are removed and returned here.
	var pendingPrepared *ruleset.PreparedGrants

	// --- Prepared slots backfill ---
	if prepRepo != nil {
		// Compute expected total prepared slots per spell level:
		// creation grants (job + archetype) + all level_up grants through sess.Level.
		type spellAgg struct {
			expectedSlots int
			pool          []ruleset.PreparedEntry
			fixed         []ruleset.PreparedEntry
		}
		bySpellLevel := make(map[int]*spellAgg)

		addPrepGrant := func(pg *ruleset.PreparedGrants, countSlots bool) {
			if pg == nil {
				return
			}
			if countSlots {
				for sl, n := range pg.SlotsByLevel {
					a := bySpellLevel[sl]
					if a == nil {
						a = &spellAgg{}
						bySpellLevel[sl] = a
					}
					a.expectedSlots += n
				}
			}
			for _, e := range pg.Pool {
				a := bySpellLevel[e.Level]
				if a == nil {
					a = &spellAgg{}
					bySpellLevel[e.Level] = a
				}
				a.pool = append(a.pool, e)
			}
			for _, e := range pg.Fixed {
				a := bySpellLevel[e.Level]
				if a == nil {
					a = &spellAgg{}
					bySpellLevel[e.Level] = a
				}
				a.fixed = append(a.fixed, e)
			}
		}

		// Creation grants contribute to the expected total but their pool entries
		// were already assigned — they are not added to the delta pool.
		if job != nil && job.TechnologyGrants != nil {
			addPrepGrant(job.TechnologyGrants.Prepared, true)
		}
		if archetype != nil && archetype.TechnologyGrants != nil {
			addPrepGrant(archetype.TechnologyGrants.Prepared, true)
		}

		// Level-up grants: count slots AND add pool entries.
		for lvl := 2; lvl <= sess.Level; lvl++ {
			g := mergedLevelUpGrants[lvl]
			if g == nil || g.Prepared == nil {
				continue
			}
			addPrepGrant(g.Prepared, true)
		}

		// Read actual state.
		existingPrep, err := prepRepo.GetAll(ctx, characterID)
		if err != nil {
			return nil, fmt.Errorf("BackfillLevelUpTechnologies GetAll: %w", err)
		}

		// Sort spell levels for deterministic order.
		spellLevels := make([]int, 0, len(bySpellLevel))
		for sl := range bySpellLevel {
			spellLevels = append(spellLevels, sl)
		}
		sort.Ints(spellLevels)

		for _, spellLvl := range spellLevels {
			agg := bySpellLevel[spellLvl]

			// REQ-TTA-2: L2+ prepared slots always require a trainer — never auto-assign.
			// For any existing pool-assigned slots at L2+ (previously auto-assigned by the
			// buggy backfill), clear them and return them as pending.
			if spellLvl >= 2 {
				actual := len(existingPrep[spellLvl])
				// Count fixed entries at this spell level — those may stay.
				fixedCount := 0
				for _, e := range agg.fixed {
					if e.Level == spellLvl {
						fixedCount++
					}
				}
				// Pool-assigned = total assigned minus fixed-assigned.
				// Any pool slot at L2+ was auto-assigned without trainer — remove it.
				poolAssigned := actual - fixedCount
				if poolAssigned > 0 {
					// Clear all slots at this level; the fixed ones will be re-applied when the
					// trainer resolves the pending grant.
					if delErr := prepRepo.DeleteAtSpellLevel(ctx, characterID, spellLvl); delErr != nil {
						return nil, fmt.Errorf("BackfillLevelUpTechnologies clear auto-assigned spell level %d: %w", spellLvl, delErr)
					}
					// Remove from session as well.
					if sess.PreparedTechs != nil {
						delete(sess.PreparedTechs, spellLvl)
					}
					actual = fixedCount
					existingPrep[spellLvl] = existingPrep[spellLvl][:fixedCount]
				}
				// Compute remaining pending slots (expected − what's actually assigned).
				pending := agg.expectedSlots - actual
				if pending > 0 {
					if pendingPrepared == nil {
						pendingPrepared = &ruleset.PreparedGrants{SlotsByLevel: make(map[int]int)}
					}
					pendingPrepared.SlotsByLevel[spellLvl] = pending
					pendingPrepared.Pool = append(pendingPrepared.Pool, agg.pool...)
					pendingPrepared.Fixed = append(pendingPrepared.Fixed, agg.fixed...)
				}
				continue
			}

			// Spell level 1: auto-assign as before.
			actual := len(existingPrep[spellLvl])
			delta := agg.expectedSlots - actual
			if delta <= 0 {
				continue
			}
			deltaGrant := &ruleset.TechnologyGrants{
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{spellLvl: delta},
					Pool:         agg.pool,
					Fixed:        agg.fixed,
				},
			}
			if applyErr := LevelUpTechnologies(ctx, sess, characterID, deltaGrant, techReg, nil,
				hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo,
			); applyErr != nil {
				return nil, fmt.Errorf("BackfillLevelUpTechnologies prepared spell level %d: %w", spellLvl, applyErr)
			}
			// Refresh for next spell level's delta check.
			existingPrep, err = prepRepo.GetAll(ctx, characterID)
			if err != nil {
				return nil, fmt.Errorf("BackfillLevelUpTechnologies refresh GetAll: %w", err)
			}
		}
	}

	// --- Hardwired techs backfill ---
	if hwRepo != nil {
		existing := make(map[string]bool, len(sess.HardwiredTechs))
		for _, id := range sess.HardwiredTechs {
			existing[id] = true
		}
		var missing []string
		for lvl := 2; lvl <= sess.Level; lvl++ {
			g := mergedLevelUpGrants[lvl]
			if g == nil {
				continue
			}
			for _, id := range g.Hardwired {
				if !existing[id] {
					missing = append(missing, id)
					existing[id] = true
				}
			}
		}
		if len(missing) > 0 {
			hwGrant := &ruleset.TechnologyGrants{Hardwired: missing}
			if applyErr := LevelUpTechnologies(ctx, sess, characterID, hwGrant, techReg, nil,
				hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo,
			); applyErr != nil {
				return nil, fmt.Errorf("BackfillLevelUpTechnologies hardwired: %w", applyErr)
			}
		}
	}

	// Return any L2+ pending prepared grants so the caller can register them and issue trainer quests.
	if pendingPrepared != nil {
		return &ruleset.TechnologyGrants{Prepared: pendingPrepared}, nil
	}
	return nil, nil
}

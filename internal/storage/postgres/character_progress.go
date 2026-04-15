package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterProgressRepository persists and retrieves character level, XP, max HP,
// and pending ability boost counts.
type CharacterProgressRepository struct {
	pool *pgxpool.Pool
}

// NewCharacterProgressRepository returns a new CharacterProgressRepository.
//
// Precondition: pool must not be nil.
// Postcondition: Returns a non-nil repository backed by the given pool.
func NewCharacterProgressRepository(pool *pgxpool.Pool) *CharacterProgressRepository {
	return &CharacterProgressRepository{pool: pool}
}

// SaveProgress persists level, experience, max_hp for a character and upserts
// the pending ability boost count.
//
// Precondition: id > 0; level >= 1; experience >= 0; maxHP >= 1; pendingBoosts >= 0.
// Postcondition: characters.level, characters.experience, characters.max_hp are updated
// and character_pending_boosts is upserted — all within a single transaction.
func (r *CharacterProgressRepository) SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("SaveProgress begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE characters
		SET level = $2, experience = $3, max_hp = $4
		WHERE id = $1
	`, id, level, experience, maxHP)
	if err != nil {
		return fmt.Errorf("SaveProgress update character: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO character_pending_boosts (character_id, count)
		VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET count = EXCLUDED.count
	`, id, pendingBoosts)
	if err != nil {
		return fmt.Errorf("SaveProgress upsert pending boosts: %w", err)
	}

	return tx.Commit(ctx)
}

// GetProgress returns level, experience, max_hp, and pending_boosts for a character.
// If no character_pending_boosts row exists, pendingBoosts is 0.
//
// Precondition: id > 0.
// Postcondition: Returns (level, experience, maxHP, pendingBoosts, nil) on success.
func (r *CharacterProgressRepository) GetProgress(ctx context.Context, id int64) (level, experience, maxHP, pendingBoosts int, err error) {
	if id <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	err = r.pool.QueryRow(ctx, `
		SELECT c.level, c.experience, c.max_hp, COALESCE(b.count, 0)
		FROM characters c
		LEFT JOIN character_pending_boosts b ON b.character_id = c.id
		WHERE c.id = $1
	`, id).Scan(&level, &experience, &maxHP, &pendingBoosts)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("GetProgress: %w", err)
	}
	return level, experience, maxHP, pendingBoosts, nil
}

// GetPendingSkillIncreases returns the number of unspent skill increases for a character.
//
// Precondition: id > 0.
// Postcondition: Returns the current pending_skill_increases value.
func (r *CharacterProgressRepository) GetPendingSkillIncreases(ctx context.Context, id int64) (int, error) {
	if id <= 0 {
		return 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT pending_skill_increases FROM characters WHERE id = $1`, id,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("GetPendingSkillIncreases: %w", err)
	}
	return n, nil
}

// IncrementPendingSkillIncreases adds n to the character's pending skill increases.
//
// Precondition: id > 0; n >= 1.
// Postcondition: pending_skill_increases increased by n.
func (r *CharacterProgressRepository) IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	if n < 1 {
		return fmt.Errorf("n must be >= 1, got %d", n)
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE characters SET pending_skill_increases = pending_skill_increases + $2 WHERE id = $1`,
		id, n,
	)
	if err != nil {
		return fmt.Errorf("IncrementPendingSkillIncreases: %w", err)
	}
	return nil
}

// ConsumePendingSkillIncrease decrements pending_skill_increases by 1.
// Returns an error containing "no pending skill increases" if none are available.
//
// Precondition: id > 0.
// Postcondition: pending_skill_increases decremented by 1, or error returned.
func (r *CharacterProgressRepository) ConsumePendingSkillIncrease(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE characters SET pending_skill_increases = pending_skill_increases - 1
         WHERE id = $1 AND pending_skill_increases > 0`,
		id,
	)
	if err != nil {
		return fmt.Errorf("ConsumePendingSkillIncrease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("no pending skill increases available for character")
	}
	return nil
}

// IsSkillIncreasesInitialized reports whether the one-time skill-increase backfill
// has already been applied to this character.
//
// Precondition: id > 0.
// Postcondition: Returns the skill_increases_initialized flag value.
func (r *CharacterProgressRepository) IsSkillIncreasesInitialized(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var initialized bool
	err := r.pool.QueryRow(ctx,
		`SELECT skill_increases_initialized FROM characters WHERE id = $1`, id,
	).Scan(&initialized)
	if err != nil {
		return false, fmt.Errorf("IsSkillIncreasesInitialized: %w", err)
	}
	return initialized, nil
}

// MarkSkillIncreasesInitialized sets skill_increases_initialized = true for a character.
//
// Precondition: id > 0.
// Postcondition: skill_increases_initialized is true in the characters table.
func (r *CharacterProgressRepository) MarkSkillIncreasesInitialized(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE characters SET skill_increases_initialized = TRUE WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("MarkSkillIncreasesInitialized: %w", err)
	}
	return nil
}

// GetPendingTechLevels returns the list of character levels with unresolved
// technology pool selections.
//
// Precondition: id > 0.
// Postcondition: Returns all pending tech levels (may be empty slice).
func (r *CharacterProgressRepository) GetPendingTechLevels(ctx context.Context, id int64) ([]int, error) {
	if id <= 0 {
		return nil, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT level FROM character_pending_tech_levels WHERE character_id = $1 ORDER BY level`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("GetPendingTechLevels: %w", err)
	}
	defer rows.Close()
	var levels []int
	for rows.Next() {
		var lvl int
		if err := rows.Scan(&lvl); err != nil {
			return nil, fmt.Errorf("GetPendingTechLevels scan: %w", err)
		}
		levels = append(levels, lvl)
	}
	return levels, rows.Err()
}

// SetPendingTechLevels replaces the stored pending tech levels for a character.
// Pass an empty slice to clear all pending levels.
//
// Precondition: id > 0.
// Postcondition: character_pending_tech_levels contains exactly the given levels.
func (r *CharacterProgressRepository) SetPendingTechLevels(ctx context.Context, id int64, levels []int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("SetPendingTechLevels begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM character_pending_tech_levels WHERE character_id = $1`, id,
	); err != nil {
		return fmt.Errorf("SetPendingTechLevels delete: %w", err)
	}
	for _, lvl := range levels {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_pending_tech_levels (character_id, level) VALUES ($1, $2)`,
			id, lvl,
		); err != nil {
			return fmt.Errorf("SetPendingTechLevels insert level %d: %w", lvl, err)
		}
	}
	return tx.Commit(ctx)
}

// ConsumePendingBoost decrements the pending boost count by 1 for a character.
// Returns an error containing "no pending boosts" if the character has none.
//
// Precondition: id > 0.
// Postcondition: Pending boost count decremented by 1, or error returned if count was 0 or absent.
func (r *CharacterProgressRepository) ConsumePendingBoost(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE character_pending_boosts
		SET count = count - 1
		WHERE character_id = $1 AND count > 0
	`, id)
	if err != nil {
		return fmt.Errorf("ConsumePendingBoost: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("no pending boosts available for character")
	}
	return nil
}

// AddPendingTechSlot inserts or increments a pending tech slot row.
//
// Precondition: characterID > 0; techLevel >= 2.
// Postcondition: Row exists with remaining incremented by 1.
func (r *CharacterProgressRepository) AddPendingTechSlot(ctx context.Context, characterID int64, charLevel, techLevel int, tradition, usageType string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO character_pending_tech_slots
			(character_id, char_level, tech_level, tradition, usage_type, remaining)
		 VALUES ($1, $2, $3, $4, $5, 1)
		 ON CONFLICT (character_id, char_level, tech_level, tradition, usage_type)
		 DO UPDATE SET remaining = character_pending_tech_slots.remaining + 1`,
		characterID, charLevel, techLevel, tradition, usageType,
	)
	if err != nil {
		return fmt.Errorf("AddPendingTechSlot: %w", err)
	}
	return nil
}

// GetPendingTechSlots returns all pending tech slots for the character with remaining > 0.
func (r *CharacterProgressRepository) GetPendingTechSlots(ctx context.Context, characterID int64) ([]session.PendingTechSlot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT char_level, tech_level, tradition, usage_type, remaining
		 FROM character_pending_tech_slots
		 WHERE character_id = $1 AND remaining > 0
		 ORDER BY char_level, tech_level`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetPendingTechSlots: %w", err)
	}
	defer rows.Close()
	var slots []session.PendingTechSlot
	for rows.Next() {
		var s session.PendingTechSlot
		if err := rows.Scan(&s.CharLevel, &s.TechLevel, &s.Tradition, &s.UsageType, &s.Remaining); err != nil {
			return nil, fmt.Errorf("GetPendingTechSlots scan: %w", err)
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

// DecrementPendingTechSlot decrements remaining by 1; deletes the row when remaining reaches 0.
//
// Precondition: row exists and remaining > 0.
// Postcondition: remaining decremented by 1; row deleted if remaining reaches 0.
func (r *CharacterProgressRepository) DecrementPendingTechSlot(ctx context.Context, characterID int64, charLevel, techLevel int, tradition, usageType string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE character_pending_tech_slots
		 SET remaining = remaining - 1
		 WHERE character_id = $1 AND char_level = $2 AND tech_level = $3
		   AND tradition = $4 AND usage_type = $5`,
		characterID, charLevel, techLevel, tradition, usageType,
	)
	if err != nil {
		return fmt.Errorf("DecrementPendingTechSlot: %w", err)
	}
	// Clean up zero-remaining rows.
	_, err = r.pool.Exec(ctx,
		`DELETE FROM character_pending_tech_slots
		 WHERE character_id = $1 AND remaining <= 0`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("DecrementPendingTechSlot cleanup: %w", err)
	}
	return nil
}

// DeleteAllPendingTechSlots removes all pending tech slot rows for the character.
//
// Precondition: characterID > 0.
// Postcondition: No character_pending_tech_slots rows exist for characterID.
func (r *CharacterProgressRepository) DeleteAllPendingTechSlots(ctx context.Context, characterID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM character_pending_tech_slots WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("DeleteAllPendingTechSlots: %w", err)
	}
	return nil
}

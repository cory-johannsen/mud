package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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

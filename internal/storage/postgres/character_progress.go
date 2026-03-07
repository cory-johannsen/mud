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

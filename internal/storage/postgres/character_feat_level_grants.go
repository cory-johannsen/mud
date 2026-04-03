package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFeatLevelGrantsRepository tracks which level-up feat grants have been
// applied to a character, preventing re-processing when pools overlap with
// creation feat pools.
type CharacterFeatLevelGrantsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterFeatLevelGrantsRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterFeatLevelGrantsRepository(db *pgxpool.Pool) *CharacterFeatLevelGrantsRepository {
	return &CharacterFeatLevelGrantsRepository{db: db}
}

// IsLevelGranted reports whether feat level-up grants for the given level have
// already been applied to this character.
//
// Precondition: characterID > 0; level >= 2.
// Postcondition: returns true iff a row exists for (characterID, level).
func (r *CharacterFeatLevelGrantsRepository) IsLevelGranted(ctx context.Context, characterID int64, level int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_feat_level_grants WHERE character_id = $1 AND level = $2)`,
		characterID, level,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("IsLevelGranted character %d level %d: %w", characterID, level, err)
	}
	return exists, nil
}

// MarkLevelGranted records that feat level-up grants for the given level have
// been applied. Idempotent via ON CONFLICT DO NOTHING.
//
// Precondition: characterID > 0; level >= 2.
// Postcondition: row (characterID, level) exists in character_feat_level_grants.
func (r *CharacterFeatLevelGrantsRepository) MarkLevelGranted(ctx context.Context, characterID int64, level int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_feat_level_grants (character_id, level) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		characterID, level,
	)
	if err != nil {
		return fmt.Errorf("MarkLevelGranted character %d level %d: %w", characterID, level, err)
	}
	return nil
}

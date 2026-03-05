package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFavoredTargetRepo persists and retrieves the favored NPC target type per character.
type CharacterFavoredTargetRepo struct {
	db *pgxpool.Pool
}

// NewCharacterFavoredTargetRepo constructs a CharacterFavoredTargetRepo.
// Precondition: db must not be nil.
func NewCharacterFavoredTargetRepo(db *pgxpool.Pool) *CharacterFavoredTargetRepo {
	return &CharacterFavoredTargetRepo{db: db}
}

// Get returns the favored target type for characterID, or "" if none is set.
// Precondition: characterID > 0.
// Postcondition: returns ("", nil) when no row exists.
func (r *CharacterFavoredTargetRepo) Get(ctx context.Context, characterID int64) (string, error) {
	var t string
	err := r.db.QueryRow(ctx,
		`SELECT target_type FROM character_favored_target WHERE character_id = $1`, characterID,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("CharacterFavoredTargetRepo.Get: %w", err)
	}
	return t, nil
}

// Set upserts the favored target type for characterID.
// Precondition: characterID > 0; targetType must be one of "human","robot","animal","mutant".
// Postcondition: exactly one row exists for characterID with target_type = targetType.
func (r *CharacterFavoredTargetRepo) Set(ctx context.Context, characterID int64, targetType string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_favored_target (character_id, target_type)
         VALUES ($1, $2)
         ON CONFLICT (character_id) DO UPDATE SET target_type = EXCLUDED.target_type`,
		characterID, targetType,
	)
	if err != nil {
		return fmt.Errorf("CharacterFavoredTargetRepo.Set: %w", err)
	}
	return nil
}

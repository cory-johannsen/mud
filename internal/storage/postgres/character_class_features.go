package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterClassFeaturesRepository persists per-character class feature lists.
type CharacterClassFeaturesRepository struct {
	db *pgxpool.Pool
}

// NewCharacterClassFeaturesRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterClassFeaturesRepository(db *pgxpool.Pool) *CharacterClassFeaturesRepository {
	return &CharacterClassFeaturesRepository{db: db}
}

// HasClassFeatures reports whether the character has any rows in character_class_features.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one feature row exists.
func (r *CharacterClassFeaturesRepository) HasClassFeatures(ctx context.Context, characterID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_class_features WHERE character_id = $1`, characterID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasClassFeatures: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all class feature IDs for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a slice of feature IDs (may be empty).
func (r *CharacterClassFeaturesRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT feature_id FROM character_class_features WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll class features: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning class feature row: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetAll writes the complete class feature list for a character, replacing any existing rows.
//
// Precondition: characterID > 0; featureIDs must not be nil.
// Postcondition: character_class_features rows match featureIDs exactly.
func (r *CharacterClassFeaturesRepository) SetAll(ctx context.Context, characterID int64, featureIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM character_class_features WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("deleting old class features: %w", err)
	}

	for _, featureID := range featureIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_class_features (character_id, feature_id) VALUES ($1, $2)`,
			characterID, featureID,
		); err != nil {
			return fmt.Errorf("inserting class feature %s: %w", featureID, err)
		}
	}
	return tx.Commit(ctx)
}

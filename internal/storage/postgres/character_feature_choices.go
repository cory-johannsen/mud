package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFeatureChoicesRepo persists and retrieves per-character feature choices.
type CharacterFeatureChoicesRepo struct {
	db *pgxpool.Pool
}

// NewCharacterFeatureChoicesRepo constructs a CharacterFeatureChoicesRepo.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repo.
func NewCharacterFeatureChoicesRepo(db *pgxpool.Pool) *CharacterFeatureChoicesRepo {
	return &CharacterFeatureChoicesRepo{db: db}
}

// GetAll returns all stored choices for characterID as a nested map:
// feature_id → choice_key → value.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterFeatureChoicesRepo) GetAll(ctx context.Context, characterID int64) (map[string]map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT feature_id, choice_key, value
         FROM character_feature_choices
         WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll: %w", err)
	}
	defer rows.Close()

	out := make(map[string]map[string]string)
	for rows.Next() {
		var featureID, choiceKey, value string
		if err := rows.Scan(&featureID, &choiceKey, &value); err != nil {
			return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll scan: %w", err)
		}
		if out[featureID] == nil {
			out[featureID] = make(map[string]string)
		}
		out[featureID][choiceKey] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CharacterFeatureChoicesRepo.GetAll rows: %w", err)
	}
	return out, nil
}

// Set upserts a single choice for characterID.
//
// Precondition: characterID > 0; featureID, choiceKey, and value must be non-empty.
// Postcondition: Exactly one row exists for (character_id, feature_id, choice_key) with the given value.
func (r *CharacterFeatureChoicesRepo) Set(ctx context.Context, characterID int64, featureID, choiceKey, value string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_feature_choices (character_id, feature_id, choice_key, value)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (character_id, feature_id, choice_key) DO UPDATE SET value = EXCLUDED.value`,
		characterID, featureID, choiceKey, value,
	)
	if err != nil {
		return fmt.Errorf("CharacterFeatureChoicesRepo.Set: %w", err)
	}
	return nil
}

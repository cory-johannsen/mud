package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterPreparedTechRepository persists prepared technology slot assignments.
type CharacterPreparedTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterPreparedTechRepository constructs a CharacterPreparedTechRepository.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterPreparedTechRepository(db *pgxpool.Pool) *CharacterPreparedTechRepository {
	return &CharacterPreparedTechRepository{db: db}
}

// GetAll returns prepared slot assignments keyed by slot level.
// Each level maps to an ordered slice indexed by slot_index.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterPreparedTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT slot_level, slot_index, tech_id
         FROM character_prepared_technologies
         WHERE character_id = $1
         ORDER BY slot_level, slot_index`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterPreparedTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	result := make(map[int][]*session.PreparedSlot)
	for rows.Next() {
		var level, index int
		var techID string
		if err := rows.Scan(&level, &index, &techID); err != nil {
			return nil, fmt.Errorf("CharacterPreparedTechRepository.GetAll scan: %w", err)
		}
		for len(result[level]) <= index {
			result[level] = append(result[level], nil)
		}
		result[level][index] = &session.PreparedSlot{TechID: techID}
	}
	return result, rows.Err()
}

// Set upserts a single prepared slot.
//
// Precondition: characterID > 0; level > 0; index >= 0; techID not empty.
// Postcondition: Exactly one row exists for (character_id, slot_level, slot_index) with the given tech_id.
func (r *CharacterPreparedTechRepository) Set(ctx context.Context, characterID int64, level, index int, techID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_prepared_technologies (character_id, slot_level, slot_index, tech_id)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (character_id, slot_level, slot_index) DO UPDATE SET tech_id = EXCLUDED.tech_id`,
		characterID, level, index, techID,
	)
	if err != nil {
		return fmt.Errorf("CharacterPreparedTechRepository.Set: %w", err)
	}
	return nil
}

// DeleteAll removes all prepared slot assignments for the character.
//
// Precondition: characterID > 0.
// Postcondition: No prepared technology rows exist for the character.
func (r *CharacterPreparedTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_prepared_technologies WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return fmt.Errorf("CharacterPreparedTechRepository.DeleteAll: %w", err)
	}
	return nil
}

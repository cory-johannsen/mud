package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterSpontaneousTechRepository persists spontaneous technology known-slot assignments.
type CharacterSpontaneousTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterSpontaneousTechRepository constructs a CharacterSpontaneousTechRepository.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterSpontaneousTechRepository(db *pgxpool.Pool) *CharacterSpontaneousTechRepository {
	return &CharacterSpontaneousTechRepository{db: db}
}

// GetAll returns known spontaneous techs keyed by tech level.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterSpontaneousTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id, level
         FROM character_spontaneous_technologies
         WHERE character_id = $1
         ORDER BY level, tech_id`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterSpontaneousTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	result := make(map[int][]string)
	for rows.Next() {
		var techID string
		var level int
		if err := rows.Scan(&techID, &level); err != nil {
			return nil, fmt.Errorf("CharacterSpontaneousTechRepository.GetAll scan: %w", err)
		}
		result[level] = append(result[level], techID)
	}
	return result, rows.Err()
}

// Add inserts a known spontaneous tech. Duplicate inserts are silently ignored.
//
// Precondition: characterID > 0; techID not empty; level > 0.
// Postcondition: A row for (character_id, tech_id) exists with the given level.
func (r *CharacterSpontaneousTechRepository) Add(ctx context.Context, characterID int64, techID string, level int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_spontaneous_technologies (character_id, tech_id, level)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, tech_id) DO NOTHING`,
		characterID, techID, level,
	)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousTechRepository.Add: %w", err)
	}
	return nil
}

// DeleteAll removes all spontaneous tech assignments for the character.
//
// Precondition: characterID > 0.
// Postcondition: No spontaneous technology rows exist for the character.
func (r *CharacterSpontaneousTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_spontaneous_technologies WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousTechRepository.DeleteAll: %w", err)
	}
	return nil
}

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterInnateTechRepository persists innate technology assignments.
type CharacterInnateTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterInnateTechRepository constructs a CharacterInnateTechRepository.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterInnateTechRepository(db *pgxpool.Pool) *CharacterInnateTechRepository {
	return &CharacterInnateTechRepository{db: db}
}

// GetAll returns innate tech assignments keyed by tech ID.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterInnateTechRepository) GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id, max_uses
         FROM character_innate_technologies
         WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*session.InnateSlot)
	for rows.Next() {
		var techID string
		var maxUses int
		if err := rows.Scan(&techID, &maxUses); err != nil {
			return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll scan: %w", err)
		}
		result[techID] = &session.InnateSlot{MaxUses: maxUses}
	}
	return result, rows.Err()
}

// Set upserts an innate tech assignment.
//
// Precondition: characterID > 0; techID not empty.
// Postcondition: Exactly one row exists for (character_id, tech_id) with the given max_uses.
func (r *CharacterInnateTechRepository) Set(ctx context.Context, characterID int64, techID string, maxUses int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_innate_technologies (character_id, tech_id, max_uses)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, tech_id) DO UPDATE SET max_uses = EXCLUDED.max_uses`,
		characterID, techID, maxUses,
	)
	if err != nil {
		return fmt.Errorf("CharacterInnateTechRepository.Set: %w", err)
	}
	return nil
}

// DeleteAll removes all innate tech assignments for the character.
//
// Precondition: characterID > 0.
// Postcondition: No innate technology rows exist for the character.
func (r *CharacterInnateTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_innate_technologies WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return fmt.Errorf("CharacterInnateTechRepository.DeleteAll: %w", err)
	}
	return nil
}

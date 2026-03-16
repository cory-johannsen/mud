package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterHardwiredTechRepository persists hardwired technology assignments.
type CharacterHardwiredTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterHardwiredTechRepository constructs a CharacterHardwiredTechRepository.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterHardwiredTechRepository(db *pgxpool.Pool) *CharacterHardwiredTechRepository {
	return &CharacterHardwiredTechRepository{db: db}
}

// GetAll returns all hardwired tech IDs for the character in alphabetical order.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil slice (may be empty) and nil error on success.
func (r *CharacterHardwiredTechRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id FROM character_hardwired_technologies WHERE character_id = $1 ORDER BY tech_id`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterHardwiredTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("CharacterHardwiredTechRepository.GetAll scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetAll replaces all hardwired tech assignments for the character atomically.
//
// Precondition: characterID > 0.
// Postcondition: Exactly the provided techIDs are stored for the character.
func (r *CharacterHardwiredTechRepository) SetAll(ctx context.Context, characterID int64, techIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("CharacterHardwiredTechRepository.SetAll begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx,
		`DELETE FROM character_hardwired_technologies WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("CharacterHardwiredTechRepository.SetAll delete: %w", err)
	}
	seen := make(map[string]struct{}, len(techIDs))
	for _, id := range techIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_hardwired_technologies (character_id, tech_id) VALUES ($1, $2)`,
			characterID, id,
		); err != nil {
			return fmt.Errorf("CharacterHardwiredTechRepository.SetAll insert %s: %w", id, err)
		}
	}
	return tx.Commit(ctx)
}

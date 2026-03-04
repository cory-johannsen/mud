package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFeatsRepository persists per-character feat lists.
type CharacterFeatsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterFeatsRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterFeatsRepository(db *pgxpool.Pool) *CharacterFeatsRepository {
	return &CharacterFeatsRepository{db: db}
}

// HasFeats reports whether the character has any rows in character_feats.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one feat row exists.
func (r *CharacterFeatsRepository) HasFeats(ctx context.Context, characterID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_feats WHERE character_id = $1`, characterID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasFeats: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all feat IDs for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a slice of feat IDs (may be empty).
func (r *CharacterFeatsRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT feat_id FROM character_feats WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll feats: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning feat row: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetAll writes the complete feat list for a character, replacing any existing rows.
//
// Precondition: characterID > 0; feats must not be nil.
// Postcondition: character_feats rows match feats exactly.
func (r *CharacterFeatsRepository) SetAll(ctx context.Context, characterID int64, feats []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM character_feats WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("deleting old feats: %w", err)
	}

	for _, featID := range feats {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_feats (character_id, feat_id) VALUES ($1, $2)`,
			characterID, featID,
		); err != nil {
			return fmt.Errorf("inserting feat %s: %w", featID, err)
		}
	}
	return tx.Commit(ctx)
}

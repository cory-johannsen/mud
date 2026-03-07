package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterProficienciesRepository persists per-character armor/weapon proficiency ranks.
type CharacterProficienciesRepository struct {
	db *pgxpool.Pool
}

// NewCharacterProficienciesRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterProficienciesRepository(db *pgxpool.Pool) *CharacterProficienciesRepository {
	return &CharacterProficienciesRepository{db: db}
}

// GetAll returns all proficiency ranks for a character as a category→rank map.
//
// Precondition: characterID > 0.
// Postcondition: Returns an empty map (not nil) if no rows exist.
func (r *CharacterProficienciesRepository) GetAll(ctx context.Context, characterID int64) (map[string]string, error) {
	if characterID <= 0 {
		return make(map[string]string), nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT category, rank FROM character_proficiencies WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll proficiencies: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var cat, rank string
		if err := rows.Scan(&cat, &rank); err != nil {
			return nil, fmt.Errorf("scanning proficiency row: %w", err)
		}
		out[cat] = rank
	}
	return out, rows.Err()
}

// Upsert inserts or updates a single proficiency rank for a character.
// If the (character_id, category) row already exists, the rank is updated.
//
// Precondition: characterID > 0; category and rank must not be empty.
// Postcondition: Exactly one row exists for (characterID, category).
func (r *CharacterProficienciesRepository) Upsert(ctx context.Context, characterID int64, category, rank string) error {
	if characterID <= 0 {
		return fmt.Errorf("Upsert proficiency: characterID must be > 0")
	}
	if category == "" {
		return fmt.Errorf("Upsert proficiency: category must not be empty")
	}
	if rank == "" {
		return fmt.Errorf("Upsert proficiency: rank must not be empty")
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_proficiencies (character_id, category, rank) VALUES ($1, $2, $3)
         ON CONFLICT (character_id, category) DO UPDATE SET rank = EXCLUDED.rank`,
		characterID, category, rank,
	)
	if err != nil {
		return fmt.Errorf("Upsert proficiency %s: %w", category, err)
	}
	return nil
}

package postgres

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterAbilityBoostsRepository persists and retrieves per-character ability boosts.
type CharacterAbilityBoostsRepository interface {
	// GetAll returns all stored boosts for characterID as source → sorted []ability.
	//
	// Precondition: characterID > 0.
	// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
	GetAll(ctx context.Context, characterID int64) (map[string][]string, error)

	// Add inserts a (source, ability) row for characterID.
	// A duplicate insert is idempotent (ON CONFLICT DO NOTHING).
	//
	// Precondition: characterID > 0; source and ability must be non-empty.
	// Postcondition: Exactly one row exists for (character_id, source, ability).
	Add(ctx context.Context, characterID int64, source, ability string) error
}

// PostgresCharacterAbilityBoostsRepository is the pgx-backed implementation.
type PostgresCharacterAbilityBoostsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterAbilityBoostsRepository constructs a PostgresCharacterAbilityBoostsRepository.
//
// Precondition: pool must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterAbilityBoostsRepository(pool *pgxpool.Pool) *PostgresCharacterAbilityBoostsRepository {
	return &PostgresCharacterAbilityBoostsRepository{db: pool}
}

// GetAll implements CharacterAbilityBoostsRepository.
func (r *PostgresCharacterAbilityBoostsRepository) GetAll(ctx context.Context, characterID int64) (map[string][]string, error) {
	if characterID <= 0 {
		return nil, fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	rows, err := r.db.Query(ctx,
		`SELECT source, ability
         FROM character_ability_boosts
         WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("PostgresCharacterAbilityBoostsRepository.GetAll: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]string)
	for rows.Next() {
		var source, ability string
		if err := rows.Scan(&source, &ability); err != nil {
			return nil, fmt.Errorf("PostgresCharacterAbilityBoostsRepository.GetAll scan: %w", err)
		}
		out[source] = append(out[source], ability)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("PostgresCharacterAbilityBoostsRepository.GetAll rows: %w", err)
	}

	for source := range out {
		sort.Strings(out[source])
	}
	return out, nil
}

// Add implements CharacterAbilityBoostsRepository.
func (r *PostgresCharacterAbilityBoostsRepository) Add(ctx context.Context, characterID int64, source, ability string) error {
	if characterID <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	if source == "" {
		return fmt.Errorf("source must not be empty")
	}
	if ability == "" {
		return fmt.Errorf("ability must not be empty")
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_ability_boosts (character_id, source, ability)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, source, ability) DO NOTHING`,
		characterID, source, ability,
	)
	if err != nil {
		return fmt.Errorf("PostgresCharacterAbilityBoostsRepository.Add: %w", err)
	}
	return nil
}

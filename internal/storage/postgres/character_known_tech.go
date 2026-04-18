package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// KnownTechRepo persists the set of technologies a character knows (their catalog).
// For wizard/ranger casting models this is the catalog from which prepared slots are filled.
// For spontaneous casting models this is the full set of known techs.
type KnownTechRepo interface {
	GetAll(ctx context.Context, characterID int64) (map[int][]string, error)
	Add(ctx context.Context, characterID int64, techID string, level int) error
	DeleteAll(ctx context.Context, characterID int64) error
}

// CharacterKnownTechRepository implements KnownTechRepo using PostgreSQL.
//
// Precondition: db must not be nil.
type CharacterKnownTechRepository struct {
	db *pgxpool.Pool
}

// NewCharacterKnownTechRepository constructs a CharacterKnownTechRepository.
//
// Precondition: db must not be nil.
// Postcondition: Returns a fully initialised repository.
func NewCharacterKnownTechRepository(db *pgxpool.Pool) *CharacterKnownTechRepository {
	return &CharacterKnownTechRepository{db: db}
}

// GetAll returns all known techs for a character, keyed by tech level.
//
// Precondition: characterID > 0.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *CharacterKnownTechRepository) GetAll(ctx context.Context, characterID int64) (map[int][]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id, level
         FROM character_known_technologies
         WHERE character_id = $1
         ORDER BY level, tech_id`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterKnownTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	result := make(map[int][]string)
	for rows.Next() {
		var techID string
		var level int
		if err := rows.Scan(&techID, &level); err != nil {
			return nil, fmt.Errorf("CharacterKnownTechRepository.GetAll scan: %w", err)
		}
		result[level] = append(result[level], techID)
	}
	return result, rows.Err()
}

// Add records a tech as known for the character at the given level.
// Silently succeeds if already present (ON CONFLICT DO NOTHING).
//
// Precondition: characterID > 0; techID not empty; level >= 1.
// Postcondition: A row for (character_id, tech_id) exists with the given level.
func (r *CharacterKnownTechRepository) Add(ctx context.Context, characterID int64, techID string, level int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_known_technologies (character_id, tech_id, level)
         VALUES ($1, $2, $3)
         ON CONFLICT (character_id, tech_id) DO NOTHING`,
		characterID, techID, level,
	)
	if err != nil {
		return fmt.Errorf("CharacterKnownTechRepository.Add: %w", err)
	}
	return nil
}

// DeleteAll removes all known tech records for the character.
//
// Precondition: characterID > 0.
// Postcondition: No character_known_technologies rows exist for the character.
func (r *CharacterKnownTechRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_known_technologies WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("CharacterKnownTechRepository.DeleteAll: %w", err)
	}
	return nil
}

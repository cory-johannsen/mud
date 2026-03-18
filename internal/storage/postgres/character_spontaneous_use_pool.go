package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// CharacterSpontaneousUsePoolRepository persists daily use pools for spontaneous technologies.
type CharacterSpontaneousUsePoolRepository struct {
	db *pgxpool.Pool
}

// NewCharacterSpontaneousUsePoolRepository constructs a new repository.
//
// Precondition: db must be non-nil and connected.
func NewCharacterSpontaneousUsePoolRepository(db *pgxpool.Pool) *CharacterSpontaneousUsePoolRepository {
	return &CharacterSpontaneousUsePoolRepository{db: db}
}

// GetAll returns all use pools for the character keyed by tech level.
//
// Postcondition: returned map contains one UsePool per initialized tech level; empty map when no rows exist.
func (r *CharacterSpontaneousUsePoolRepository) GetAll(ctx context.Context, characterID int64) (map[int]session.UsePool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_level, uses_remaining, max_uses FROM character_spontaneous_use_pools WHERE character_id = $1`,
		characterID)
	if err != nil {
		return nil, fmt.Errorf("CharacterSpontaneousUsePoolRepository.GetAll: %w", err)
	}
	defer rows.Close()

	result := make(map[int]session.UsePool)
	for rows.Next() {
		var techLevel, usesRemaining, maxUses int
		if err := rows.Scan(&techLevel, &usesRemaining, &maxUses); err != nil {
			return nil, fmt.Errorf("CharacterSpontaneousUsePoolRepository.GetAll scan: %w", err)
		}
		result[techLevel] = session.UsePool{Remaining: usesRemaining, Max: maxUses}
	}
	return result, rows.Err()
}

// Set initializes or overwrites a pool entry for the given character and tech level.
//
// Precondition: characterID > 0; techLevel >= 1; usesRemaining >= 0; maxUses >= 0.
// Postcondition: row (characterID, techLevel) has uses_remaining=usesRemaining, max_uses=maxUses.
func (r *CharacterSpontaneousUsePoolRepository) Set(ctx context.Context, characterID int64, techLevel, usesRemaining, maxUses int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_spontaneous_use_pools (character_id, tech_level, uses_remaining, max_uses)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (character_id, tech_level) DO UPDATE
		 SET uses_remaining = EXCLUDED.uses_remaining, max_uses = EXCLUDED.max_uses`,
		characterID, techLevel, usesRemaining, maxUses)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousUsePoolRepository.Set: %w", err)
	}
	return nil
}

// Decrement atomically decrements uses_remaining by 1 if > 0.
//
// Precondition: characterID > 0; techLevel >= 1.
// Postcondition: uses_remaining = max(0, uses_remaining - 1).
func (r *CharacterSpontaneousUsePoolRepository) Decrement(ctx context.Context, characterID int64, techLevel int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE character_spontaneous_use_pools
		 SET uses_remaining = GREATEST(0, uses_remaining - 1)
		 WHERE character_id = $1 AND tech_level = $2`,
		characterID, techLevel)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousUsePoolRepository.Decrement: %w", err)
	}
	return nil
}

// RestoreAll sets uses_remaining = max_uses for all rows of this character.
//
// Postcondition: all pools are at maximum.
func (r *CharacterSpontaneousUsePoolRepository) RestoreAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE character_spontaneous_use_pools SET uses_remaining = max_uses WHERE character_id = $1`,
		characterID)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousUsePoolRepository.RestoreAll: %w", err)
	}
	return nil
}

// DeleteAll removes all pool entries for the character.
//
// Postcondition: no rows with character_id remain.
func (r *CharacterSpontaneousUsePoolRepository) DeleteAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_spontaneous_use_pools WHERE character_id = $1`,
		characterID)
	if err != nil {
		return fmt.Errorf("CharacterSpontaneousUsePoolRepository.DeleteAll: %w", err)
	}
	return nil
}

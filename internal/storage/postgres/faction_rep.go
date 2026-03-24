package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FactionRepRepository persists per-character faction reputation scores.
type FactionRepRepository struct {
	db *pgxpool.Pool
}

// NewFactionRepRepository creates a FactionRepRepository backed by the given pool.
//
// Precondition: db must be a valid open connection pool.
// Postcondition: Returns a non-nil *FactionRepRepository.
func NewFactionRepRepository(db *pgxpool.Pool) *FactionRepRepository {
	return &FactionRepRepository{db: db}
}

// SaveRep upserts the rep score for (characterID, factionID).
//
// Precondition: characterID > 0; factionID non-empty.
// Postcondition: The row is inserted or updated atomically.
func (r *FactionRepRepository) SaveRep(ctx context.Context, characterID int64, factionID string, rep int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_faction_rep (character_id, faction_id, rep)
		VALUES ($1, $2, $3)
		ON CONFLICT (character_id, faction_id) DO UPDATE SET rep = EXCLUDED.rep`,
		characterID, factionID, rep,
	)
	if err != nil {
		return fmt.Errorf("FactionRepRepository.SaveRep: %w", err)
	}
	return nil
}

// LoadRep returns all faction_id → rep entries for characterID.
//
// Precondition: characterID > 0.
// Postcondition: Returns an empty (non-nil) map when no rows exist.
func (r *FactionRepRepository) LoadRep(ctx context.Context, characterID int64) (map[string]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT faction_id, rep FROM character_faction_rep WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("FactionRepRepository.LoadRep: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var fid string
		var rep int
		if err := rows.Scan(&fid, &rep); err != nil {
			return nil, fmt.Errorf("FactionRepRepository.LoadRep scan: %w", err)
		}
		result[fid] = rep
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FactionRepRepository.LoadRep rows: %w", err)
	}
	return result, nil
}

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutomapRepository persists per-character discovered room sets.
type AutomapRepository struct {
	db *pgxpool.Pool
}

// NewAutomapRepository creates an AutomapRepository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewAutomapRepository(db *pgxpool.Pool) *AutomapRepository {
	return &AutomapRepository{db: db}
}

// Insert records that characterID has discovered roomID in zoneID.
// Duplicate inserts are silently ignored.
//
// Precondition: characterID >= 1; zoneID and roomID must be non-empty.
// Postcondition: Row exists in character_map_rooms; no error on duplicate.
func (r *AutomapRepository) Insert(ctx context.Context, characterID int64, zoneID, roomID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_map_rooms (character_id, zone_id, room_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		characterID, zoneID, roomID,
	)
	if err != nil {
		return fmt.Errorf("inserting map room (%d, %q, %q): %w", characterID, zoneID, roomID, err)
	}
	return nil
}

// BulkInsert records that characterID has discovered all roomIDs in zoneID.
// Duplicate inserts are silently ignored. Empty roomIDs is a no-op.
//
// Precondition: characterID >= 1; zoneID non-empty; roomIDs may be empty.
// Postcondition: All roomIDs exist in character_map_rooms for the character.
func (r *AutomapRepository) BulkInsert(ctx context.Context, characterID int64, zoneID string, roomIDs []string) error {
	for _, roomID := range roomIDs {
		if err := r.Insert(ctx, characterID, zoneID, roomID); err != nil {
			return err
		}
	}
	return nil
}

// LoadAll returns all discovered rooms for characterID, keyed by zone then room.
//
// Precondition: characterID >= 1.
// Postcondition: Returns non-nil map (may be empty); never returns nil, nil.
func (r *AutomapRepository) LoadAll(ctx context.Context, characterID int64) (map[string]map[string]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT zone_id, room_id FROM character_map_rooms WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading map rooms for character %d: %w", characterID, err)
	}
	defer rows.Close()

	result := make(map[string]map[string]bool)
	for rows.Next() {
		var zoneID, roomID string
		if err := rows.Scan(&zoneID, &roomID); err != nil {
			return nil, fmt.Errorf("scanning map room row: %w", err)
		}
		if result[zoneID] == nil {
			result[zoneID] = make(map[string]bool)
		}
		result[zoneID][roomID] = true
	}
	return result, rows.Err()
}

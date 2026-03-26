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
// Duplicate inserts are silently ignored. If explored is true, an existing
// unexplored row is upgraded to explored via ON CONFLICT UPDATE.
//
// Precondition: characterID >= 1; zoneID and roomID must be non-empty.
// Postcondition: Row exists in character_map_rooms; no error on duplicate.
func (r *AutomapRepository) Insert(ctx context.Context, characterID int64, zoneID, roomID string, explored bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_map_rooms (character_id, zone_id, room_id, explored)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, zone_id, room_id) DO UPDATE
		  SET explored = character_map_rooms.explored OR EXCLUDED.explored`,
		characterID, zoneID, roomID, explored,
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
func (r *AutomapRepository) BulkInsert(ctx context.Context, characterID int64, zoneID string, roomIDs []string, explored bool) error {
	for _, roomID := range roomIDs {
		if err := r.Insert(ctx, characterID, zoneID, roomID, explored); err != nil {
			return err
		}
	}
	return nil
}

// AutomapResult holds both the full known-room set and the explored-only subset.
type AutomapResult struct {
	// AllKnown contains all rooms the character has any record of, keyed by zone then room.
	AllKnown map[string]map[string]bool
	// ExploredOnly contains only rooms the character has physically visited (explored=true).
	ExploredOnly map[string]map[string]bool
}

// LoadAll returns all discovered rooms for characterID split into known and explored sets.
//
// Precondition: characterID >= 1.
// Postcondition: Returns non-nil maps (may be empty); never returns nil, nil.
func (r *AutomapRepository) LoadAll(ctx context.Context, characterID int64) (AutomapResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT zone_id, room_id, explored FROM character_map_rooms WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return AutomapResult{}, fmt.Errorf("loading map rooms for character %d: %w", characterID, err)
	}
	defer rows.Close()

	result := AutomapResult{
		AllKnown:     make(map[string]map[string]bool),
		ExploredOnly: make(map[string]map[string]bool),
	}
	for rows.Next() {
		var zoneID, roomID string
		var explored bool
		if err := rows.Scan(&zoneID, &roomID, &explored); err != nil {
			return AutomapResult{}, fmt.Errorf("scanning map room row: %w", err)
		}
		if result.AllKnown[zoneID] == nil {
			result.AllKnown[zoneID] = make(map[string]bool)
		}
		result.AllKnown[zoneID][roomID] = true
		if explored {
			if result.ExploredOnly[zoneID] == nil {
				result.ExploredOnly[zoneID] = make(map[string]bool)
			}
			result.ExploredOnly[zoneID][roomID] = true
		}
	}
	return result, rows.Err()
}

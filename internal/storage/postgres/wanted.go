package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WantedRepository persists per-player per-zone WantedLevel values.
type WantedRepository struct {
	db *pgxpool.Pool
}

// NewWantedRepository constructs a WantedRepository backed by the given pool.
func NewWantedRepository(db *pgxpool.Pool) *WantedRepository {
	return &WantedRepository{db: db}
}

// Load returns all non-zero wanted levels for the character.
// Rows with wanted_level=0 are never stored; absent rows imply level 0.
// Postcondition: the returned map MUST NOT be nil.
func (r *WantedRepository) Load(ctx context.Context, characterID int64) (map[string]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT zone_id, wanted_level FROM character_wanted_levels WHERE character_id = $1`,
		characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var zoneID string
		var level int
		if err := rows.Scan(&zoneID, &level); err != nil {
			return nil, err
		}
		result[zoneID] = level
	}
	return result, rows.Err()
}

// Upsert sets the wanted level for the character in the given zone.
// If level is 0, the row is deleted (level 0 means no record).
// Precondition: level MUST be in [0, 4].
// Postcondition: when level==0, no row exists for (characterID, zoneID).
func (r *WantedRepository) Upsert(ctx context.Context, characterID int64, zoneID string, level int) error {
	if level == 0 {
		_, err := r.db.Exec(ctx,
			`DELETE FROM character_wanted_levels WHERE character_id = $1 AND zone_id = $2`,
			characterID, zoneID)
		return err
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_wanted_levels (character_id, zone_id, wanted_level)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (character_id, zone_id) DO UPDATE SET wanted_level = EXCLUDED.wanted_level`,
		characterID, zoneID, level)
	return err
}

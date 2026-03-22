package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealerCapacityRepository persists per-template healer daily capacity usage.
type HealerCapacityRepository struct {
	db *pgxpool.Pool
}

// NewHealerCapacityRepository creates a HealerCapacityRepository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewHealerCapacityRepository(db *pgxpool.Pool) *HealerCapacityRepository {
	return &HealerCapacityRepository{db: db}
}

// Save upserts the capacity_used value for the given NPC template.
//
// Precondition: templateID must be non-empty; capacityUsed must be >= 0.
// Postcondition: npc_healer_capacity row is inserted or updated.
func (r *HealerCapacityRepository) Save(ctx context.Context, templateID string, capacityUsed int) error {
	if templateID == "" {
		return fmt.Errorf("HealerCapacityRepository.Save: templateID must be non-empty")
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO npc_healer_capacity (npc_template_id, capacity_used, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (npc_template_id)
			DO UPDATE SET capacity_used = EXCLUDED.capacity_used, updated_at = NOW()`,
		templateID, capacityUsed,
	)
	if err != nil {
		return fmt.Errorf("HealerCapacityRepository.Save: %w", err)
	}
	return nil
}

// LoadAll returns a map of templateID to capacity_used for all stored healers.
//
// Precondition: none.
// Postcondition: Returns a non-nil map (may be empty) and nil error on success.
func (r *HealerCapacityRepository) LoadAll(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `SELECT npc_template_id, capacity_used FROM npc_healer_capacity`)
	if err != nil {
		return nil, fmt.Errorf("HealerCapacityRepository.LoadAll: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var templateID string
		var capacityUsed int
		if err := rows.Scan(&templateID, &capacityUsed); err != nil {
			return nil, fmt.Errorf("HealerCapacityRepository.LoadAll scanning row: %w", err)
		}
		result[templateID] = capacityUsed
	}
	return result, rows.Err()
}

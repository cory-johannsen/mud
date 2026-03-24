package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterMaterialsRepository persists crafting material quantities per character.
type CharacterMaterialsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterMaterialsRepository creates a CharacterMaterialsRepository backed by the given pool.
//
// Precondition: db must be a valid open connection pool.
// Postcondition: Returns a non-nil *CharacterMaterialsRepository.
func NewCharacterMaterialsRepository(db *pgxpool.Pool) *CharacterMaterialsRepository {
	return &CharacterMaterialsRepository{db: db}
}

// Load returns all materials for a character as a materialID→quantity map.
//
// Precondition: characterID > 0.
// Postcondition: Returns an empty (non-nil) map if no materials are found.
func (r *CharacterMaterialsRepository) Load(ctx context.Context, characterID int64) (map[string]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT material_id, quantity FROM character_materials WHERE character_id = $1`,
		characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var id string
		var qty int
		if err := rows.Scan(&id, &qty); err != nil {
			return nil, err
		}
		result[id] = qty
	}
	return result, rows.Err()
}

// Add increments a material quantity, inserting a new row if none exists.
//
// Precondition: characterID > 0, materialID non-empty, amount > 0.
// Postcondition: The stored quantity is increased by amount.
func (r *CharacterMaterialsRepository) Add(ctx context.Context, characterID int64, materialID string, amount int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_materials (character_id, material_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (character_id, material_id)
		DO UPDATE SET quantity = character_materials.quantity + EXCLUDED.quantity`,
		characterID, materialID, amount)
	return err
}

// DeductMany executes all deductions in a single transaction. (REQ-CRAFT-12)
// Rows where quantity reaches zero are deleted.
//
// Precondition: characterID > 0, deductions non-nil.
// Postcondition: All deductions are applied atomically, or none are (on error).
func (r *CharacterMaterialsRepository) DeductMany(ctx context.Context, characterID int64, deductions map[string]int) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for materialID, qty := range deductions {
		if qty == 0 {
			continue
		}
		// Delete the row if the deduction exactly consumes it.
		del, err := tx.Exec(ctx,
			`DELETE FROM character_materials WHERE character_id = $1 AND material_id = $2 AND quantity = $3`,
			characterID, materialID, qty)
		if err != nil {
			return err
		}
		if del.RowsAffected() == 1 {
			// Exact match deleted; deduction satisfied.
			continue
		}
		// Otherwise reduce quantity; quantity must exceed qty for check constraint.
		tag, err := tx.Exec(ctx, `
			UPDATE character_materials
			SET quantity = quantity - $1
			WHERE character_id = $2 AND material_id = $3 AND quantity > $1`,
			qty, characterID, materialID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("insufficient %s", materialID)
		}
	}
	return tx.Commit(ctx)
}

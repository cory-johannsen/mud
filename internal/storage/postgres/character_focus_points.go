package postgres

import (
	"context"
	"fmt"
)

// SaveFocusPoints persists the current focus_points for a character.
//
// Precondition: characterID > 0; focusPoints >= 0.
// Postcondition: characters.focus_points column is updated.
func (r *CharacterRepository) SaveFocusPoints(ctx context.Context, characterID int64, focusPoints int) error {
	if characterID <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	_, err := r.db.Exec(ctx, `UPDATE characters SET focus_points = $2 WHERE id = $1`, characterID, focusPoints)
	return err
}

// LoadFocusPoints returns the stored focus_points for the given character.
// Returns 0 for characters without a persisted count.
//
// Precondition: characterID > 0.
// Postcondition: returns (focusPoints, nil) on success.
func (r *CharacterRepository) LoadFocusPoints(ctx context.Context, characterID int64) (int, error) {
	if characterID <= 0 {
		return 0, fmt.Errorf("characterID must be > 0, got %d", characterID)
	}
	var fp int
	err := r.db.QueryRow(ctx, `SELECT COALESCE(focus_points, 0) FROM characters WHERE id = $1`, characterID).Scan(&fp)
	return fp, err
}

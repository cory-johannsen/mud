package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SaveHotbar persists the player's 10-slot hotbar to the characters table.
// slots is JSON-encoded as a string array. An all-empty hotbar is stored as NULL.
//
// Precondition: characterID > 0.
// Postcondition: characters.hotbar updated; returns ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) SaveHotbar(ctx context.Context, characterID int64, slots [10]string) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveHotbar: characterID must be > 0, got %d", characterID)
	}
	// Store NULL for all-empty hotbar.
	allEmpty := true
	for _, s := range slots {
		if s != "" {
			allEmpty = false
			break
		}
	}
	var encoded *string
	if !allEmpty {
		b, err := json.Marshal(slots[:])
		if err != nil {
			return fmt.Errorf("SaveHotbar: marshal: %w", err)
		}
		s := string(b)
		encoded = &s
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE characters SET hotbar = $2 WHERE id = $1`,
		characterID, encoded,
	)
	if err != nil {
		return fmt.Errorf("SaveHotbar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SaveHotbar: %w", ErrCharacterNotFound)
	}
	return nil
}

// LoadHotbar retrieves the player's 10-slot hotbar from the characters table.
// Returns an all-empty [10]string if the column is NULL (not yet set).
//
// Precondition: characterID > 0.
// Postcondition: Returns a [10]string (never nil); returns ErrCharacterNotFound if no character row.
func (r *CharacterRepository) LoadHotbar(ctx context.Context, characterID int64) ([10]string, error) {
	var raw *string
	err := r.db.QueryRow(ctx,
		`SELECT hotbar FROM characters WHERE id = $1`, characterID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return [10]string{}, ErrCharacterNotFound
		}
		return [10]string{}, fmt.Errorf("LoadHotbar: %w", err)
	}
	if raw == nil {
		return [10]string{}, nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(*raw), &arr); err != nil {
		return [10]string{}, fmt.Errorf("LoadHotbar: unmarshal: %w", err)
	}
	var slots [10]string
	for i := 0; i < len(arr) && i < 10; i++ {
		slots[i] = arr[i]
	}
	return slots, nil
}

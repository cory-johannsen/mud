package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// hotbarSlotJSON is the on-disk JSON representation of a single hotbar slot.
type hotbarSlotJSON struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// MarshalHotbarSlots serialises a [10]HotbarSlot to JSON bytes.
// Returns nil when all slots are empty (stored as NULL in the DB).
//
// Precondition: none.
// Postcondition: Returns nil iff all slots are empty; otherwise valid JSON.
func MarshalHotbarSlots(slots [10]session.HotbarSlot) ([]byte, error) {
	allEmpty := true
	for _, s := range slots {
		if !s.IsEmpty() {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return nil, nil
	}
	arr := make([]hotbarSlotJSON, 10)
	for i, s := range slots {
		arr[i] = hotbarSlotJSON{Kind: s.Kind, Ref: s.Ref}
	}
	return json.Marshal(arr)
}

// UnmarshalHotbarSlots deserialises JSON bytes to a [10]HotbarSlot.
// Auto-migrates the legacy plain-string format (["cmd1","cmd2",...]).
//
// Precondition: data is non-nil and non-empty.
// Postcondition: Returns a valid [10]HotbarSlot; legacy strings become command slots.
func UnmarshalHotbarSlots(data []byte) ([10]session.HotbarSlot, error) {
	// Try new typed format first: array of objects with kind+ref fields.
	var typed []hotbarSlotJSON
	if err := json.Unmarshal(data, &typed); err == nil && len(typed) > 0 {
		// Distinguish new format from old: new format has at least one non-empty kind or ref.
		hasContent := false
		for _, v := range typed {
			if v.Kind != "" || v.Ref != "" {
				hasContent = true
				break
			}
		}
		if hasContent {
			var slots [10]session.HotbarSlot
			for i := 0; i < len(typed) && i < 10; i++ {
				if typed[i].Ref != "" {
					slots[i] = session.HotbarSlot{Kind: typed[i].Kind, Ref: typed[i].Ref}
				}
			}
			return slots, nil
		}
	}

	// Fallback: legacy plain-string format ["cmd1", "cmd2", ...].
	var legacy []string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return [10]session.HotbarSlot{}, fmt.Errorf("UnmarshalHotbarSlots: %w", err)
	}
	var slots [10]session.HotbarSlot
	for i := 0; i < len(legacy) && i < 10; i++ {
		if legacy[i] != "" {
			slots[i] = session.CommandSlot(legacy[i])
		}
	}
	return slots, nil
}

// SaveHotbar persists the player's 10-slot hotbar to the characters table.
//
// Precondition: characterID > 0.
// Postcondition: characters.hotbar updated; returns ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) SaveHotbar(ctx context.Context, characterID int64, slots [10]session.HotbarSlot) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveHotbar: characterID must be > 0, got %d", characterID)
	}
	b, err := MarshalHotbarSlots(slots)
	if err != nil {
		return fmt.Errorf("SaveHotbar: marshal: %w", err)
	}
	var encoded *string
	if b != nil {
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
// Returns an all-empty [10]HotbarSlot if the column is NULL.
// Auto-migrates legacy plain-string format on read.
//
// Precondition: characterID > 0.
// Postcondition: Returns a valid [10]HotbarSlot; returns ErrCharacterNotFound if no character row.
func (r *CharacterRepository) LoadHotbar(ctx context.Context, characterID int64) ([10]session.HotbarSlot, error) {
	var raw *string
	err := r.db.QueryRow(ctx,
		`SELECT hotbar FROM characters WHERE id = $1`, characterID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return [10]session.HotbarSlot{}, ErrCharacterNotFound
		}
		return [10]session.HotbarSlot{}, fmt.Errorf("LoadHotbar: %w", err)
	}
	if raw == nil {
		return [10]session.HotbarSlot{}, nil
	}
	slots, err := UnmarshalHotbarSlots([]byte(*raw))
	if err != nil {
		return [10]session.HotbarSlot{}, fmt.Errorf("LoadHotbar: %w", err)
	}
	return slots, nil
}

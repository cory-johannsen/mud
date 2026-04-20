package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// MarshalHotbars JSON-encodes a slice of 10-slot bars.
// Returns nil if all bars are empty (stored as NULL).
//
// Precondition: bars must be non-nil; each bar has exactly 10 slots.
// Postcondition: Returns nil bytes for all-empty bars.
func MarshalHotbars(bars [][10]session.HotbarSlot) ([]byte, error) {
	allEmpty := true
	for _, bar := range bars {
		for _, sl := range bar {
			if !sl.IsEmpty() {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			break
		}
	}
	if allEmpty {
		return nil, nil
	}
	type barJSON = [10]hotbarSlotJSON
	out := make([]barJSON, len(bars))
	for bi, bar := range bars {
		for si, sl := range bar {
			out[bi][si] = hotbarSlotJSON{Kind: sl.Kind, Ref: sl.Ref}
		}
	}
	return json.Marshal(out)
}

// UnmarshalHotbars decodes a JSON byte slice into a slice of 10-slot bars.
//
// Precondition: data is valid JSON produced by MarshalHotbars.
// Postcondition: Returns decoded bars with exactly 10 slots per bar.
func UnmarshalHotbars(data []byte) ([][10]session.HotbarSlot, error) {
	type barJSON = [10]hotbarSlotJSON
	var raw []barJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	bars := make([][10]session.HotbarSlot, len(raw))
	for bi, bar := range raw {
		for si, sl := range bar {
			bars[bi][si] = session.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		}
	}
	return bars, nil
}

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

// LoadHotbars retrieves all hotbars and the active index for a character.
// Falls back to the legacy hotbar column if hotbars is NULL.
// Returns one empty bar with index 0 if both columns are NULL.
//
// Precondition: characterID > 0.
// Postcondition: Returns at least 1 bar; activeIdx is within [0, len(bars)-1].
func (r *CharacterRepository) LoadHotbars(ctx context.Context, characterID int64) ([][10]session.HotbarSlot, int, error) {
	if characterID <= 0 {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: characterID must be > 0, got %d", characterID)
	}
	var hotbarsJSON, legacyHotbarJSON *string
	var activeIdx int
	err := r.db.QueryRow(ctx,
		`SELECT hotbars, hotbar, active_hotbar_idx FROM characters WHERE id = $1`, characterID,
	).Scan(&hotbarsJSON, &legacyHotbarJSON, &activeIdx)
	if errors.Is(err, pgx.ErrNoRows) {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: %w", ErrCharacterNotFound)
	}
	if err != nil {
		return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: %w", err)
	}
	// Use new hotbars column if present.
	if hotbarsJSON != nil && *hotbarsJSON != "" {
		bars, err := UnmarshalHotbars([]byte(*hotbarsJSON))
		if err != nil {
			return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: unmarshal: %w", err)
		}
		if len(bars) == 0 {
			bars = [][10]session.HotbarSlot{{}}
		}
		if activeIdx >= len(bars) {
			activeIdx = 0
		}
		return bars, activeIdx, nil
	}
	// Legacy migration: unmarshal old hotbar column as bar 0.
	if legacyHotbarJSON != nil && *legacyHotbarJSON != "" {
		bar, err := UnmarshalHotbarSlots([]byte(*legacyHotbarJSON))
		if err != nil {
			return [][10]session.HotbarSlot{{}}, 0, fmt.Errorf("LoadHotbars: legacy unmarshal: %w", err)
		}
		return [][10]session.HotbarSlot{bar}, 0, nil
	}
	return [][10]session.HotbarSlot{{}}, 0, nil
}

// SaveHotbars persists all hotbars and the active index for a character.
// Stores NULL for hotbars if all bars are empty.
//
// Precondition: characterID > 0; bars non-nil; activeIdx in [0, len(bars)-1].
// Postcondition: characters.hotbars and active_hotbar_idx updated.
func (r *CharacterRepository) SaveHotbars(ctx context.Context, characterID int64, bars [][10]session.HotbarSlot, activeIdx int) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveHotbars: characterID must be > 0, got %d", characterID)
	}
	data, err := MarshalHotbars(bars)
	if err != nil {
		return fmt.Errorf("SaveHotbars: marshal: %w", err)
	}
	var hotbarsValue interface{}
	if data != nil {
		hotbarsValue = string(data)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE characters SET hotbars = $2, active_hotbar_idx = $3 WHERE id = $1`,
		characterID, hotbarsValue, activeIdx,
	)
	if err != nil {
		return fmt.Errorf("SaveHotbars: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SaveHotbars: %w", ErrCharacterNotFound)
	}
	return nil
}

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DowntimeState is the persisted form of an active downtime activity.
//
// Precondition: ActivityID and RoomID MUST be non-empty; CompletesAt MUST be in the future.
// Postcondition: All fields are populated after a successful Load.
type DowntimeState struct {
	ActivityID  string
	CompletesAt time.Time
	RoomID      string
	Metadata    string // JSON; empty string means no metadata
}

// CharacterDowntimeRepository persists active downtime state for characters.
//
// Precondition: db must be a valid, open *pgxpool.Pool pointing at the mud database.
// Postcondition: Save/Load/Clear operate atomically on the character_downtime table.
type CharacterDowntimeRepository struct {
	db *pgxpool.Pool
}

// NewCharacterDowntimeRepository returns a new repository backed by db.
//
// Precondition: db must be non-nil.
func NewCharacterDowntimeRepository(db *pgxpool.Pool) *CharacterDowntimeRepository {
	return &CharacterDowntimeRepository{db: db}
}

// Save upserts the downtime state for a character.
//
// Precondition: characterID > 0; state.ActivityID and state.RoomID are non-empty.
// Postcondition: character_downtime row is inserted or updated for characterID.
func (r *CharacterDowntimeRepository) Save(ctx context.Context, characterID int64, state DowntimeState) error {
	var meta *string
	if state.Metadata != "" {
		meta = &state.Metadata
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_downtime (character_id, activity_id, completes_at, room_id, activity_metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (character_id) DO UPDATE SET
			activity_id       = EXCLUDED.activity_id,
			completes_at      = EXCLUDED.completes_at,
			room_id           = EXCLUDED.room_id,
			activity_metadata = EXCLUDED.activity_metadata`,
		characterID, state.ActivityID, state.CompletesAt, state.RoomID, meta)
	return err
}

// Load returns the active downtime state for a character, or nil if none.
//
// Precondition: characterID > 0.
// Postcondition: Returns (*DowntimeState, nil) if a row exists; (nil, nil) if not found.
func (r *CharacterDowntimeRepository) Load(ctx context.Context, characterID int64) (*DowntimeState, error) {
	row := r.db.QueryRow(ctx,
		`SELECT activity_id, completes_at, room_id, activity_metadata FROM character_downtime WHERE character_id = $1`,
		characterID)
	var s DowntimeState
	var meta *string
	err := row.Scan(&s.ActivityID, &s.CompletesAt, &s.RoomID, &meta)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if meta != nil {
		s.Metadata = *meta
	}
	return &s, nil
}

// Clear removes the active downtime state for a character.
//
// Precondition: characterID > 0.
// Postcondition: No row exists in character_downtime for characterID.
func (r *CharacterDowntimeRepository) Clear(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_downtime WHERE character_id = $1`, characterID)
	return err
}

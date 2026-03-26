package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueueEntry is a single entry in the downtime activity queue.
//
// Precondition: Position >= 1; ActivityID is non-empty.
type QueueEntry struct {
	ID           int64
	CharacterID  int64
	Position     int
	ActivityID   string
	ActivityArgs string
}

// CharacterDowntimeQueueRepository manages per-character downtime activity queues.
//
// Precondition: non-nil db connection pool.
// Postcondition: all methods maintain contiguous 1-based position ordering (REQ-DTQ-12).
type CharacterDowntimeQueueRepository struct {
	db *pgxpool.Pool
}

// NewCharacterDowntimeQueueRepository creates a new repository.
//
// Precondition: db is non-nil.
// Postcondition: returns non-nil repository.
func NewCharacterDowntimeQueueRepository(db *pgxpool.Pool) *CharacterDowntimeQueueRepository {
	return &CharacterDowntimeQueueRepository{db: db}
}

// Enqueue adds an entry at the end of the queue (position = max+1).
//
// Precondition: characterID > 0; activityID is non-empty.
// Postcondition: new entry is appended at max(position)+1.
func (r *CharacterDowntimeQueueRepository) Enqueue(ctx context.Context, characterID int64, activityID string, activityArgs string) error {
	var args *string
	if activityArgs != "" {
		args = &activityArgs
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_downtime_queue (character_id, position, activity_id, activity_args)
		VALUES ($1, COALESCE((SELECT MAX(position) FROM character_downtime_queue WHERE character_id = $1), 0) + 1, $2, $3)`,
		characterID, activityID, args)
	return err
}

// ListQueue returns all entries for a character ordered by position.
//
// Precondition: characterID > 0.
// Postcondition: entries are ordered by position ascending; may be empty.
func (r *CharacterDowntimeQueueRepository) ListQueue(ctx context.Context, characterID int64) ([]QueueEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, character_id, position, activity_id, activity_args FROM character_downtime_queue WHERE character_id = $1 ORDER BY position ASC`,
		characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []QueueEntry
	for rows.Next() {
		var e QueueEntry
		var args *string
		if err := rows.Scan(&e.ID, &e.CharacterID, &e.Position, &e.ActivityID, &args); err != nil {
			return nil, err
		}
		if args != nil {
			e.ActivityArgs = *args
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []QueueEntry{}
	}
	return entries, nil
}

// ErrQueuePositionNotFound is returned by RemoveAt when no entry exists at the given position.
var ErrQueuePositionNotFound = errors.New("queue position not found")

// RemoveAt deletes the entry at the given 1-based position and reindexes positions above it.
// Executes within a single transaction (REQ-DTQ-12).
//
// Precondition: characterID > 0; position >= 1.
// Postcondition: positions above deleted entry decremented by 1; contiguous order maintained.
// Returns ErrQueuePositionNotFound if no entry exists at the given position.
func (r *CharacterDowntimeQueueRepository) RemoveAt(ctx context.Context, characterID int64, position int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx,
		`DELETE FROM character_downtime_queue WHERE character_id = $1 AND position = $2`,
		characterID, position)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrQueuePositionNotFound
	}

	_, err = tx.Exec(ctx,
		`UPDATE character_downtime_queue SET position = position - 1 WHERE character_id = $1 AND position > $2`,
		characterID, position)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Clear removes all queue entries for a character.
//
// Precondition: characterID > 0.
// Postcondition: ListQueue returns empty slice for this character.
func (r *CharacterDowntimeQueueRepository) Clear(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_downtime_queue WHERE character_id = $1`, characterID)
	return err
}

// PopHead atomically removes position 1 and reindexes remaining entries (REQ-DTQ-12).
// Returns nil if the queue is empty.
//
// Precondition: characterID > 0.
// Postcondition: if queue was non-empty, returned entry had position=1; remaining entries reindexed.
func (r *CharacterDowntimeQueueRepository) PopHead(ctx context.Context, characterID int64) (*QueueEntry, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var e QueueEntry
	var args *string
	err = tx.QueryRow(ctx,
		`SELECT id, character_id, position, activity_id, activity_args FROM character_downtime_queue WHERE character_id = $1 ORDER BY position ASC LIMIT 1`,
		characterID).Scan(&e.ID, &e.CharacterID, &e.Position, &e.ActivityID, &args)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if args != nil {
		e.ActivityArgs = *args
	}

	_, err = tx.Exec(ctx,
		`DELETE FROM character_downtime_queue WHERE id = $1`, e.ID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx,
		`UPDATE character_downtime_queue SET position = position - 1 WHERE character_id = $1 AND position > $2`,
		characterID, e.Position)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &e, nil
}

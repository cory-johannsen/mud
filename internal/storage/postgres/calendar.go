package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CalendarRepo persists the in-game hour, day, month, and tick to the world_calendar table.
//
// Precondition: db != nil and the world_calendar migration has been applied.
type CalendarRepo struct {
	db *pgxpool.Pool
}

// NewCalendarRepo creates a CalendarRepo backed by db.
//
// Precondition: db != nil and the world_calendar migration has been applied.
func NewCalendarRepo(db *pgxpool.Pool) *CalendarRepo {
	return &CalendarRepo{db: db}
}

// Load returns the persisted hour, day, month, and tick.
// Returns (6, 1, 1, 0, nil) when no row exists (first boot).
func (r *CalendarRepo) Load() (hour, day, month int, tick int64, err error) {
	row := r.db.QueryRow(context.Background(),
		`SELECT hour, day, month, tick FROM world_calendar WHERE id = 1`)
	if scanErr := row.Scan(&hour, &day, &month, &tick); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 6, 1, 1, 0, nil
		}
		return 0, 0, 0, 0, fmt.Errorf("calendar load: %w", scanErr)
	}
	return hour, day, month, tick, nil
}

// Save upserts the single calendar row (id=1) with hour, day, month, and tick.
//
// Precondition: hour in [0,23], day in [1,31], month in [1,12], tick >= 0.
func (r *CalendarRepo) Save(hour, day, month int, tick int64) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("calendar save: hour %d out of range [0, 23]", hour)
	}
	if day < 1 || day > 31 {
		return fmt.Errorf("calendar save: day %d out of range [1,31]", day)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("calendar save: month %d out of range [1,12]", month)
	}
	if tick < 0 {
		return fmt.Errorf("calendar save: tick %d must be >= 0", tick)
	}
	_, err := r.db.Exec(context.Background(), `
		INSERT INTO world_calendar (id, hour, day, month, tick)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		  SET hour = EXCLUDED.hour,
		      day  = EXCLUDED.day,
		      month = EXCLUDED.month,
		      tick = EXCLUDED.tick
	`, hour, day, month, tick)
	if err != nil {
		return fmt.Errorf("calendar save: %w", err)
	}
	return nil
}

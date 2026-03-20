package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CalendarRepo persists the in-game day and month to the world_calendar table.
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

// Load returns the persisted day and month.
// Returns (1, 1, nil) when no row exists (first boot).
func (r *CalendarRepo) Load() (day, month int, err error) {
	row := r.db.QueryRow(context.Background(),
		`SELECT day, month FROM world_calendar WHERE id = 1`)
	if scanErr := row.Scan(&day, &month); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 1, 1, nil
		}
		return 0, 0, fmt.Errorf("calendar load: %w", scanErr)
	}
	return day, month, nil
}

// Save upserts the single calendar row (id=1) with day and month.
//
// Precondition: day in [1,31], month in [1,12].
func (r *CalendarRepo) Save(day, month int) error {
	if day < 1 || day > 31 {
		return fmt.Errorf("calendar save: day %d out of range [1,31]", day)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("calendar save: month %d out of range [1,12]", month)
	}
	_, err := r.db.Exec(context.Background(), `
		INSERT INTO world_calendar (id, day, month)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET day = EXCLUDED.day, month = EXCLUDED.month
	`, day, month)
	if err != nil {
		return fmt.Errorf("calendar save: %w", err)
	}
	return nil
}

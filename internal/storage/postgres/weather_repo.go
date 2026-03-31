package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ActiveWeatherEvent holds the state of a currently active weather event.
type ActiveWeatherEvent struct {
	WeatherType string
	EndTick     int64
}

// WeatherRepo is the interface for persisting weather event state.
//
// Precondition: migration 057 has been applied.
type WeatherRepo interface {
	GetActive(ctx context.Context) (*ActiveWeatherEvent, error)
	GetCooldownEnd(ctx context.Context) (endTick int64, found bool, err error)
	StartEvent(ctx context.Context, weatherType string, endTick int64) error
	EndEvent(ctx context.Context, cooldownEndTick int64) error
	ClearExpired(ctx context.Context) error
}

// PostgresWeatherRepo implements WeatherRepo backed by PostgreSQL.
type PostgresWeatherRepo struct {
	db *pgxpool.Pool
}

// NewWeatherRepo creates a WeatherRepo backed by db.
//
// Precondition: db != nil and migration 057 has been applied.
func NewWeatherRepo(db *pgxpool.Pool) *PostgresWeatherRepo {
	return &PostgresWeatherRepo{db: db}
}

// GetActive returns the currently active weather event, or nil if none is active.
func (r *PostgresWeatherRepo) GetActive(ctx context.Context) (*ActiveWeatherEvent, error) {
	row := r.db.QueryRow(ctx,
		`SELECT weather_type, end_tick FROM weather_events WHERE active = TRUE LIMIT 1`)
	var ev ActiveWeatherEvent
	if err := row.Scan(&ev.WeatherType, &ev.EndTick); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("weather repo GetActive: %w", err)
	}
	return &ev, nil
}

// GetCooldownEnd returns the cooldown end tick from the most recent ended event.
// Returns found=false if no cooldown row exists.
func (r *PostgresWeatherRepo) GetCooldownEnd(ctx context.Context) (int64, bool, error) {
	row := r.db.QueryRow(ctx,
		`SELECT cooldown_end_tick FROM weather_events WHERE active = FALSE ORDER BY id DESC LIMIT 1`)
	var endTick int64
	if err := row.Scan(&endTick); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("weather repo GetCooldownEnd: %w", err)
	}
	return endTick, true, nil
}

// StartEvent inserts a new active weather event row.
//
// Precondition: no other active event exists.
func (r *PostgresWeatherRepo) StartEvent(ctx context.Context, weatherType string, endTick int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO weather_events (weather_type, end_tick, cooldown_end_tick, active)
		 VALUES ($1, $2, 0, TRUE)`,
		weatherType, endTick)
	if err != nil {
		return fmt.Errorf("weather repo StartEvent: %w", err)
	}
	return nil
}

// EndEvent marks the active event as inactive and sets the cooldown end tick.
func (r *PostgresWeatherRepo) EndEvent(ctx context.Context, cooldownEndTick int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE weather_events SET active = FALSE, cooldown_end_tick = $1 WHERE active = TRUE`,
		cooldownEndTick)
	if err != nil {
		return fmt.Errorf("weather repo EndEvent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("weather repo EndEvent: no active event found")
	}
	return nil
}

// ClearExpired deletes all inactive (ended) weather event rows.
func (r *PostgresWeatherRepo) ClearExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM weather_events WHERE active = FALSE`)
	if err != nil {
		return fmt.Errorf("weather repo ClearExpired: %w", err)
	}
	return nil
}

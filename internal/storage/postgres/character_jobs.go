package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterJobsRepository manages the character_jobs table.
//
// Precondition: db must be a valid, open connection pool.
type CharacterJobsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterJobsRepository creates a new CharacterJobsRepository.
//
// Precondition: db must be non-nil and connected.
func NewCharacterJobsRepository(db *pgxpool.Pool) *CharacterJobsRepository {
	return &CharacterJobsRepository{db: db}
}

// AddJob inserts a job for a character. No-op if already present (ON CONFLICT DO NOTHING).
//
// Precondition: characterID > 0; jobID non-empty.
// Postcondition: Row (characterID, jobID) exists in character_jobs.
func (r *CharacterJobsRepository) AddJob(ctx context.Context, characterID int64, jobID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_jobs (character_id, job_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		characterID, jobID)
	if err != nil {
		return fmt.Errorf("AddJob: %w", err)
	}
	return nil
}

// RemoveJob removes a job for a character. No-op if not present.
//
// Precondition: characterID > 0; jobID non-empty.
// Postcondition: Row (characterID, jobID) does not exist in character_jobs.
func (r *CharacterJobsRepository) RemoveJob(ctx context.Context, characterID int64, jobID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_jobs WHERE character_id=$1 AND job_id=$2`,
		characterID, jobID)
	if err != nil {
		return fmt.Errorf("RemoveJob: %w", err)
	}
	return nil
}

// ListJobs returns all job IDs held by a character, ordered alphabetically.
//
// Precondition: characterID > 0.
// Postcondition: Returns a (possibly nil) slice of job ID strings; nil on empty result.
func (r *CharacterJobsRepository) ListJobs(ctx context.Context, characterID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT job_id FROM character_jobs WHERE character_id=$1 ORDER BY job_id`,
		characterID)
	if err != nil {
		return nil, fmt.Errorf("ListJobs query: %w", err)
	}
	defer rows.Close()
	var jobs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ListJobs scan: %w", err)
		}
		jobs = append(jobs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListJobs rows: %w", err)
	}
	return jobs, nil
}

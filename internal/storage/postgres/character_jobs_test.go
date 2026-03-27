package postgres_test

import (
	"context"
	"sort"
	"testing"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterJobsRepository_AddAndList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterJobsRepository(db)

	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "DELETE FROM character_jobs WHERE character_id=$1", ch.ID)
	})

	if err := repo.AddJob(ctx, ch.ID, "thug"); err != nil {
		t.Fatalf("AddJob thug: %v", err)
	}
	if err := repo.AddJob(ctx, ch.ID, "goon"); err != nil {
		t.Fatalf("AddJob goon: %v", err)
	}

	jobs, err := repo.ListJobs(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	sort.Strings(jobs)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d: %v", len(jobs), jobs)
	}
	if jobs[0] != "goon" || jobs[1] != "thug" {
		t.Errorf("unexpected jobs: %v", jobs)
	}
}

func TestCharacterJobsRepository_AddJob_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterJobsRepository(db)

	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "DELETE FROM character_jobs WHERE character_id=$1", ch.ID)
	})

	if err := repo.AddJob(ctx, ch.ID, "thug"); err != nil {
		t.Fatalf("first AddJob: %v", err)
	}
	if err := repo.AddJob(ctx, ch.ID, "thug"); err != nil {
		t.Fatalf("second AddJob (idempotent): %v", err)
	}

	jobs, err := repo.ListJobs(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0] != "thug" {
		t.Errorf("expected exactly [thug], got %v", jobs)
	}
}

func TestCharacterJobsRepository_RemoveJob(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterJobsRepository(db)

	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "DELETE FROM character_jobs WHERE character_id=$1", ch.ID)
	})

	if err := repo.AddJob(ctx, ch.ID, "thug"); err != nil {
		t.Fatalf("AddJob thug: %v", err)
	}
	if err := repo.AddJob(ctx, ch.ID, "goon"); err != nil {
		t.Fatalf("AddJob goon: %v", err)
	}

	if err := repo.RemoveJob(ctx, ch.ID, "thug"); err != nil {
		t.Fatalf("RemoveJob thug: %v", err)
	}

	jobs, err := repo.ListJobs(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListJobs after remove: %v", err)
	}
	if len(jobs) != 1 || jobs[0] != "goon" {
		t.Errorf("expected [goon] after remove, got %v", jobs)
	}
}

func TestCharacterJobsRepository_ListJobs_EmptyForNew(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterJobsRepository(db)

	jobs, err := repo.ListJobs(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected empty jobs for new character, got %v", jobs)
	}
}

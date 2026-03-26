package postgres_test

import (
	"context"
	"testing"
	"time"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestQuestRepository_SaveAndLoad(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewQuestRepository(db)
	ctx := context.Background()

	charRepo := NewCharacterRepository(db)
	charID := createTestCharacter(t, charRepo, ctx).ID

	// SaveQuestStatus active
	if err := repo.SaveQuestStatus(ctx, charID, "q1", "active", nil); err != nil {
		t.Fatalf("SaveQuestStatus: %v", err)
	}
	// SaveObjectiveProgress
	if err := repo.SaveObjectiveProgress(ctx, charID, "q1", "obj1", 2); err != nil {
		t.Fatalf("SaveObjectiveProgress: %v", err)
	}
	// LoadQuests
	records, err := repo.LoadQuests(ctx, charID)
	if err != nil {
		t.Fatalf("LoadQuests: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.QuestID != "q1" || r.Status != "active" {
		t.Fatalf("unexpected record: %+v", r)
	}
	if r.Progress["obj1"] != 2 {
		t.Fatalf("expected progress 2, got %d", r.Progress["obj1"])
	}
}

func TestQuestRepository_SaveQuestStatus_Completed(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewQuestRepository(db)
	ctx := context.Background()

	charRepo := NewCharacterRepository(db)
	charID := createTestCharacter(t, charRepo, ctx).ID

	if err := repo.SaveQuestStatus(ctx, charID, "q2", "active", nil); err != nil {
		t.Fatalf("SaveQuestStatus active: %v", err)
	}
	now := time.Now().UTC()
	if err := repo.SaveQuestStatus(ctx, charID, "q2", "completed", &now); err != nil {
		t.Fatalf("SaveQuestStatus completed: %v", err)
	}
	records, err := repo.LoadQuests(ctx, charID)
	if err != nil {
		t.Fatalf("LoadQuests: %v", err)
	}
	for _, r := range records {
		if r.QuestID == "q2" {
			if r.Status != "completed" {
				t.Fatalf("expected completed, got %q", r.Status)
			}
			if r.CompletedAt == nil {
				t.Fatal("expected non-nil CompletedAt")
			}
		}
	}
}

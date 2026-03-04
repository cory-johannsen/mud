package postgres_test

import (
	"context"
	"testing"
)

func TestCharacterSkillsRepository_HasSkills_FalseForNew(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	has, err := repo.HasSkills(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasSkills: %v", err)
	}
	if has {
		t.Error("expected HasSkills=false for new character")
	}
}

func TestCharacterSkillsRepository_SetAllAndGetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	skills := map[string]string{
		"parkour":  "trained",
		"ghosting": "untrained",
		"muscle":   "trained",
	}
	if err := repo.SetAll(ctx, ch.ID, skills); err != nil {
		t.Fatalf("SetAll: %v", err)
	}

	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != len(skills) {
		t.Fatalf("expected %d skills, got %d", len(skills), len(got))
	}
	if got["parkour"] != "trained" {
		t.Errorf("expected parkour=trained, got %q", got["parkour"])
	}
	if got["ghosting"] != "untrained" {
		t.Errorf("expected ghosting=untrained, got %q", got["ghosting"])
	}
}

func TestCharacterSkillsRepository_HasSkills_TrueAfterSetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	if err := repo.SetAll(ctx, ch.ID, map[string]string{"parkour": "untrained"}); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	has, err := repo.HasSkills(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasSkills: %v", err)
	}
	if !has {
		t.Error("expected HasSkills=true after SetAll")
	}
}

func TestCharacterSkillsRepository_SetAll_Replaces(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewCharacterSkillsRepository(db)
	charRepo := NewCharacterRepository(db)

	ch := createTestCharacter(t, charRepo, ctx)

	if err := repo.SetAll(ctx, ch.ID, map[string]string{"parkour": "trained", "muscle": "trained"}); err != nil {
		t.Fatalf("SetAll first: %v", err)
	}
	// Replace with different set
	if err := repo.SetAll(ctx, ch.ID, map[string]string{"ghosting": "trained"}); err != nil {
		t.Fatalf("SetAll second: %v", err)
	}
	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 skill after replace, got %d", len(got))
	}
	if got["ghosting"] != "trained" {
		t.Errorf("expected ghosting=trained, got %q", got["ghosting"])
	}
}

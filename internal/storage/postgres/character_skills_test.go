package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

func TestUpgradeSkill_SetsRank(t *testing.T) {
	ctx := context.Background()
	repo := NewCharacterSkillsRepository(sharedPool)
	charRepo := NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, repo.SetAll(ctx, ch.ID, map[string]string{"parkour": "untrained"}))
	require.NoError(t, repo.UpgradeSkill(ctx, ch.ID, "parkour", "trained"))

	skills, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "trained", skills["parkour"])
}

func TestUpgradeSkill_InsertsIfMissing(t *testing.T) {
	ctx := context.Background()
	repo := NewCharacterSkillsRepository(sharedPool)
	charRepo := NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, repo.UpgradeSkill(ctx, ch.ID, "muscle", "trained"))

	skills, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "trained", skills["muscle"])
}

func TestUpgradeSkill_InvalidID(t *testing.T) {
	repo := NewCharacterSkillsRepository(sharedPool)
	ctx := context.Background()
	assert.Error(t, repo.UpgradeSkill(ctx, 0, "parkour", "trained"))
	assert.Error(t, repo.UpgradeSkill(ctx, -1, "parkour", "trained"))
}

func TestUpgradeSkill_EmptyArgs(t *testing.T) {
	ctx := context.Background()
	repo := NewCharacterSkillsRepository(sharedPool)
	charRepo := NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	assert.Error(t, repo.UpgradeSkill(ctx, ch.ID, "", "trained"))
	assert.Error(t, repo.UpgradeSkill(ctx, ch.ID, "parkour", ""))
}

func TestPropertyUpgradeSkill_RoundTrip(t *testing.T) {
	repo := NewCharacterSkillsRepository(sharedPool)
	charRepo := NewCharacterRepository(sharedPool)
	ctx := context.Background()
	ranks := []string{"untrained", "trained", "expert", "master", "legendary"}

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		rank := rapid.SampledFrom(ranks).Draw(rt, "rank")
		require.NoError(rt, repo.UpgradeSkill(ctx, ch.ID, "parkour", rank))
		skills, err := repo.GetAll(ctx, ch.ID)
		require.NoError(rt, err)
		if skills["parkour"] != rank {
			rt.Fatalf("expected %q got %q", rank, skills["parkour"])
		}
	})
}

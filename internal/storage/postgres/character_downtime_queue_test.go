package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterDowntimeQueue_EnqueueAndList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeQueueRepository(db)

	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_a", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_b", `{"k":"v"}`))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_c", ""))

	entries, err := repo.ListQueue(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, 1, entries[0].Position)
	assert.Equal(t, "activity_a", entries[0].ActivityID)
	assert.Equal(t, 2, entries[1].Position)
	assert.Equal(t, "activity_b", entries[1].ActivityID)
	assert.Equal(t, 3, entries[2].Position)
	assert.Equal(t, "activity_c", entries[2].ActivityID)
}

func TestCharacterDowntimeQueue_RemoveAt_Reindexes(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeQueueRepository(db)

	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_a", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_b", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_c", ""))

	require.NoError(t, repo.RemoveAt(ctx, ch.ID, 2))

	entries, err := repo.ListQueue(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, 1, entries[0].Position)
	assert.Equal(t, "activity_a", entries[0].ActivityID)
	assert.Equal(t, 2, entries[1].Position)
	assert.Equal(t, "activity_c", entries[1].ActivityID)
}

func TestCharacterDowntimeQueue_PopHead_RemovesFirst(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeQueueRepository(db)

	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_a", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_b", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_c", ""))

	entry, err := repo.PopHead(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, 1, entry.Position)
	assert.Equal(t, "activity_a", entry.ActivityID)

	entries, err := repo.ListQueue(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, 1, entries[0].Position)
	assert.Equal(t, "activity_b", entries[0].ActivityID)
	assert.Equal(t, 2, entries[1].Position)
	assert.Equal(t, "activity_c", entries[1].ActivityID)
}

func TestCharacterDowntimeQueue_PopHead_EmptyQueue_NilResult(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeQueueRepository(db)

	entry, err := repo.PopHead(ctx, ch.ID)
	require.NoError(t, err)
	assert.Nil(t, entry)
}

func TestCharacterDowntimeQueue_Clear(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeQueueRepository(db)

	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_a", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_b", ""))
	require.NoError(t, repo.Enqueue(ctx, ch.ID, "activity_c", ""))

	require.NoError(t, repo.Clear(ctx, ch.ID))

	entries, err := repo.ListQueue(ctx, ch.ID)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

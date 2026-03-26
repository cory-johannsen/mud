package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

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

// TestPropertyCharacterDowntimeQueue_PositionsAlwaysContiguous verifies that after any
// sequence of Enqueue and PopHead operations, the remaining queue positions are always
// a contiguous 1-based sequence (REQ-DTQ-12).
//
// Precondition: any number of enqueue/pop operations in [1,10] range.
// Postcondition: positions == [1, 2, ..., len(entries)] after each operation.
func TestPropertyCharacterDowntimeQueue_PositionsAlwaysContiguous(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ctx := context.Background()
		db := testDB(t)
		charRepo := NewCharacterRepository(db)
		ch := createTestCharacter(t, charRepo, ctx)
		repo := pgstore.NewCharacterDowntimeQueueRepository(db)

		enqueueCount := rapid.IntRange(1, 8).Draw(rt, "enqueueCount")
		for i := 0; i < enqueueCount; i++ {
			err := repo.Enqueue(ctx, ch.ID, fmt.Sprintf("act_%d", i), "")
			require.NoError(t, err)
		}

		popCount := rapid.IntRange(0, enqueueCount).Draw(rt, "popCount")
		for i := 0; i < popCount; i++ {
			_, err := repo.PopHead(ctx, ch.ID)
			require.NoError(t, err)
		}

		entries, err := repo.ListQueue(ctx, ch.ID)
		require.NoError(t, err)
		for i, e := range entries {
			if e.Position != i+1 {
				rt.Fatalf("position not contiguous: entries[%d].Position = %d, want %d", i, e.Position, i+1)
			}
		}
	})
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

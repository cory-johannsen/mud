package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterDowntimeRepository_SaveLoadClear(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeRepository(db)

	state := pgstore.DowntimeState{
		ActivityID:  "earn_creds",
		CompletesAt: time.Now().Add(5 * time.Minute).UTC().Truncate(time.Microsecond),
		RoomID:      "test_room",
		Metadata:    `{"key": "value"}`,
	}

	// Save
	err := repo.Save(ctx, ch.ID, state)
	require.NoError(t, err)

	// Load
	loaded, err := repo.Load(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, state.ActivityID, loaded.ActivityID)
	assert.Equal(t, state.RoomID, loaded.RoomID)
	assert.Equal(t, state.Metadata, loaded.Metadata)
	assert.WithinDuration(t, state.CompletesAt, loaded.CompletesAt, time.Second)

	// Clear
	err = repo.Clear(ctx, ch.ID)
	require.NoError(t, err)

	// Load after clear should return nil
	cleared, err := repo.Load(ctx, ch.ID)
	require.NoError(t, err)
	assert.Nil(t, cleared)
}

func TestCharacterDowntimeRepository_Load_NilForMissing(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeRepository(db)

	result, err := repo.Load(ctx, ch.ID)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCharacterDowntimeRepository_Save_Upsert(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeRepository(db)

	first := pgstore.DowntimeState{
		ActivityID:  "earn_creds",
		CompletesAt: time.Now().Add(5 * time.Minute).UTC().Truncate(time.Microsecond),
		RoomID:      "room_a",
		Metadata:    `{"key": "first"}`,
	}
	second := pgstore.DowntimeState{
		ActivityID:  "craft_item",
		CompletesAt: time.Now().Add(10 * time.Minute).UTC().Truncate(time.Microsecond),
		RoomID:      "room_b",
		Metadata:    `{"key": "second"}`,
	}

	require.NoError(t, repo.Save(ctx, ch.ID, first))
	require.NoError(t, repo.Save(ctx, ch.ID, second))

	loaded, err := repo.Load(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, second.ActivityID, loaded.ActivityID)
	assert.Equal(t, second.RoomID, loaded.RoomID)
	assert.Equal(t, second.Metadata, loaded.Metadata)
}

func TestCharacterDowntimeRepository_Save_EmptyMetadata(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterDowntimeRepository(db)

	state := pgstore.DowntimeState{
		ActivityID:  "rest",
		CompletesAt: time.Now().Add(5 * time.Minute).UTC().Truncate(time.Microsecond),
		RoomID:      "home_room",
		Metadata:    "",
	}

	require.NoError(t, repo.Save(ctx, ch.ID, state))

	loaded, err := repo.Load(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, state.ActivityID, loaded.ActivityID)
	assert.Equal(t, "", loaded.Metadata)
}

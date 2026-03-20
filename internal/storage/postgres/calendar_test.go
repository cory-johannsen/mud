package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCalendarRepo_Load_EmptyTable_ReturnsDefault(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewCalendarRepo(db)
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, day)
	assert.Equal(t, 1, month)
}

func TestCalendarRepo_SaveAndLoad_RoundTrip(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewCalendarRepo(db)
	require.NoError(t, repo.Save(15, 7))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 15, day)
	assert.Equal(t, 7, month)
}

func TestCalendarRepo_Save_Upserts(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewCalendarRepo(db)
	require.NoError(t, repo.Save(1, 1))
	require.NoError(t, repo.Save(28, 2))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 28, day)
	assert.Equal(t, 2, month)
}

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func truncateCalendar(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(context.Background(), `DELETE FROM world_calendar`)
	require.NoError(t, err)
}

func TestCalendarRepo_Load_EmptyTable_ReturnsDefault(t *testing.T) {
	db := testDB(t)
	truncateCalendar(t, db)
	repo := pgstore.NewCalendarRepo(db)
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, day)
	assert.Equal(t, 1, month)
}

func TestCalendarRepo_SaveAndLoad_RoundTrip(t *testing.T) {
	db := testDB(t)
	truncateCalendar(t, db)
	repo := pgstore.NewCalendarRepo(db)
	require.NoError(t, repo.Save(15, 7))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 15, day)
	assert.Equal(t, 7, month)
}

func TestCalendarRepo_Save_Upserts(t *testing.T) {
	db := testDB(t)
	truncateCalendar(t, db)
	repo := pgstore.NewCalendarRepo(db)
	require.NoError(t, repo.Save(1, 1))
	require.NoError(t, repo.Save(28, 2))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 28, day)
	assert.Equal(t, 2, month)
}

func TestCalendarRepo_Property_RoundTrip(t *testing.T) {
	db := testDB(t)
	truncateCalendar(t, db)
	repo := pgstore.NewCalendarRepo(db)
	rapid.Check(t, func(rt *rapid.T) {
		day := rapid.IntRange(1, 31).Draw(rt, "day")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		require.NoError(rt, repo.Save(day, month))
		gotDay, gotMonth, err := repo.Load()
		require.NoError(rt, err)
		assert.Equal(rt, day, gotDay)
		assert.Equal(rt, month, gotMonth)
	})
}

func TestCalendarRepo_Save_RejectsOutOfRange(t *testing.T) {
	db := testDB(t)
	repo := pgstore.NewCalendarRepo(db)
	cases := []struct {
		day, month int
	}{
		{0, 1},
		{32, 1},
		{1, 0},
		{1, 13},
	}
	for _, c := range cases {
		err := repo.Save(c.day, c.month)
		assert.Error(t, err, "Save(%d, %d) should return error", c.day, c.month)
	}
}

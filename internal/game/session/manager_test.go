package session

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestBridgeEntity_Push(t *testing.T) {
	e := NewBridgeEntity("test", 4)
	require.NoError(t, e.Push([]byte("hello")))

	data := <-e.Events()
	assert.Equal(t, []byte("hello"), data)
}

func TestBridgeEntity_PushClosed(t *testing.T) {
	e := NewBridgeEntity("test", 4)
	require.NoError(t, e.Close())
	assert.True(t, e.IsClosed())
	assert.Error(t, e.Push([]byte("fail")))
}

func TestBridgeEntity_PushFull(t *testing.T) {
	e := NewBridgeEntity("test", 1)
	require.NoError(t, e.Push([]byte("first")))
	err := e.Push([]byte("overflow"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer full")
}

func TestBridgeEntity_CloseIdempotent(t *testing.T) {
	e := NewBridgeEntity("test", 4)
	require.NoError(t, e.Close())
	require.NoError(t, e.Close())
	assert.True(t, e.IsClosed())
}

func TestManager_AddPlayer(t *testing.T) {
	m := NewManager()
	sess, err := m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)
	assert.Equal(t, "Alice", sess.Username)
	assert.Equal(t, "room_a", sess.RoomID)
	assert.Equal(t, 1, m.PlayerCount())
}

func TestManager_AddPlayer_BackpackAndCurrency(t *testing.T) {
	m := NewManager()
	sess, err := m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)

	require.NotNil(t, sess.Backpack, "new session must have a non-nil Backpack")
	assert.Equal(t, 20, sess.Backpack.MaxSlots)
	assert.Equal(t, 50.0, sess.Backpack.MaxWeight)
	assert.Equal(t, 0, sess.Backpack.UsedSlots())
	assert.Equal(t, 0, sess.Currency)
}

func TestManager_AddPlayerDuplicate(t *testing.T) {
	m := NewManager()
	_, err := m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)
	_, err = m.AddPlayer("u1", "Alice2", "Alice2", 0, "room_b", 10, "player")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestManager_RemovePlayer(t *testing.T) {
	m := NewManager()
	_, err := m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)

	err = m.RemovePlayer("u1")
	require.NoError(t, err)
	assert.Equal(t, 0, m.PlayerCount())

	players := m.PlayersInRoom("room_a")
	assert.Empty(t, players)
}

func TestManager_RemovePlayerNotFound(t *testing.T) {
	m := NewManager()
	err := m.RemovePlayer("unknown")
	assert.Error(t, err)
}

func TestManager_MovePlayer(t *testing.T) {
	m := NewManager()
	_, err := m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	require.NoError(t, err)

	oldRoom, err := m.MovePlayer("u1", "room_b")
	require.NoError(t, err)
	assert.Equal(t, "room_a", oldRoom)

	sess, ok := m.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "room_b", sess.RoomID)

	assert.Empty(t, m.PlayersInRoom("room_a"))
	assert.Equal(t, []string{"Alice"}, m.PlayersInRoom("room_b"))
}

func TestManager_MovePlayerNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.MovePlayer("unknown", "room_b")
	assert.Error(t, err)
}

func TestManager_PlayersInRoom(t *testing.T) {
	m := NewManager()
	_, _ = m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")
	_, _ = m.AddPlayer("u2", "Bob", "Bob", 0, "room_a", 10, "player")
	_, _ = m.AddPlayer("u3", "Charlie", "Charlie", 0, "room_b", 10, "player")

	roomA := m.PlayersInRoom("room_a")
	assert.Len(t, roomA, 2)
	assert.Contains(t, roomA, "Alice")
	assert.Contains(t, roomA, "Bob")

	roomB := m.PlayersInRoom("room_b")
	assert.Len(t, roomB, 1)
	assert.Contains(t, roomB, "Charlie")

	assert.Empty(t, m.PlayersInRoom("empty_room"))
}

func TestManager_GetPlayer(t *testing.T) {
	m := NewManager()
	_, _ = m.AddPlayer("u1", "Alice", "Alice", 0, "room_a", 10, "player")

	sess, ok := m.GetPlayer("u1")
	assert.True(t, ok)
	assert.Equal(t, "Alice", sess.Username)

	_, ok = m.GetPlayer("unknown")
	assert.False(t, ok)
}

func TestManager_ConcurrentAddRemove(t *testing.T) {
	m := NewManager()
	const n = 100
	var wg sync.WaitGroup

	// Add n players concurrently
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			uid := fmt.Sprintf("u%d", i)
			name := fmt.Sprintf("Player%d", i)
			_, _ = m.AddPlayer(uid, name, name, 0, "room_a", 10, "player")
		}(i)
	}
	wg.Wait()
	assert.Equal(t, n, m.PlayerCount())

	// Remove all concurrently
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = m.RemovePlayer(fmt.Sprintf("u%d", i))
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 0, m.PlayerCount())
	assert.Empty(t, m.PlayersInRoom("room_a"))
}

func TestManager_ConcurrentMove(t *testing.T) {
	m := NewManager()
	const n = 50
	rooms := []string{"room_a", "room_b", "room_c"}

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("P%d", i)
		_, err := m.AddPlayer(fmt.Sprintf("u%d", i), name, name, 0, rooms[0], 10, "player")
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			targetRoom := rooms[(i+1)%len(rooms)]
			_, _ = m.MovePlayer(fmt.Sprintf("u%d", i), targetRoom)
		}(i)
	}
	wg.Wait()

	// Verify total player count is consistent
	assert.Equal(t, n, m.PlayerCount())

	// Verify room counts sum to total
	total := 0
	for _, room := range rooms {
		total += len(m.PlayersInRoom(room))
	}
	assert.Equal(t, n, total)
}

func TestPropertyRoomOccupancyConsistent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := NewManager()
		rooms := []string{"r1", "r2", "r3"}
		numPlayers := rapid.IntRange(1, 20).Draw(t, "num_players")

		// Add players
		for i := 0; i < numPlayers; i++ {
			roomIdx := rapid.IntRange(0, len(rooms)-1).Draw(t, "room_idx")
			uid := fmt.Sprintf("p%d", i)
			name := fmt.Sprintf("Player%d", i)
			_, _ = m.AddPlayer(uid, name, name, 0, rooms[roomIdx], 10, "player")
		}

		// Move some players
		numMoves := rapid.IntRange(0, numPlayers*2).Draw(t, "num_moves")
		for i := 0; i < numMoves; i++ {
			playerIdx := rapid.IntRange(0, numPlayers-1).Draw(t, "move_player")
			roomIdx := rapid.IntRange(0, len(rooms)-1).Draw(t, "move_room")
			_, _ = m.MovePlayer(fmt.Sprintf("p%d", playerIdx), rooms[roomIdx])
		}

		// Remove some players
		numRemoves := rapid.IntRange(0, numPlayers/2).Draw(t, "num_removes")
		for i := 0; i < numRemoves; i++ {
			playerIdx := rapid.IntRange(0, numPlayers-1).Draw(t, "remove_player")
			_ = m.RemovePlayer(fmt.Sprintf("p%d", playerIdx))
		}

		// Verify: total players across all rooms == PlayerCount()
		totalInRooms := 0
		for _, room := range rooms {
			totalInRooms += len(m.PlayersInRoom(room))
		}
		if totalInRooms != m.PlayerCount() {
			t.Fatalf("room occupancy sum %d != player count %d", totalInRooms, m.PlayerCount())
		}
	})
}

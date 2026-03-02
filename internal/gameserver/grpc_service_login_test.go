package gameserver

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockCharSaverFull is a test double for CharacterSaver that allows controlling
// return values and recording whether save methods are called.
//
// It is distinct from the minimal mockCharSaver defined in teleport_handler_test.go
// and adds load-error injection and call-recording for save operations.
type mockCharSaverFull struct {
	// loadWeaponPresetsErr, if non-nil, is returned by LoadWeaponPresets.
	loadWeaponPresetsErr error
	// loadEquipmentErr, if non-nil, is returned by LoadEquipment.
	loadEquipmentErr error

	// saveWeaponPresetsCalled is incremented each time SaveWeaponPresets is called.
	saveWeaponPresetsCalled atomic.Int32
	// saveEquipmentCalled is incremented each time SaveEquipment is called.
	saveEquipmentCalled atomic.Int32
}

// SaveState satisfies CharacterSaver; always succeeds in tests.
func (m *mockCharSaverFull) SaveState(_ context.Context, _ int64, _ string, _ int) error {
	return nil
}

// LoadWeaponPresets returns an error if loadWeaponPresetsErr is set,
// otherwise returns a default-initialized LoadoutSet.
func (m *mockCharSaverFull) LoadWeaponPresets(_ context.Context, _ int64) (*inventory.LoadoutSet, error) {
	if m.loadWeaponPresetsErr != nil {
		return nil, m.loadWeaponPresetsErr
	}
	return inventory.NewLoadoutSet(), nil
}

// SaveWeaponPresets records that it was called and always succeeds.
func (m *mockCharSaverFull) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	m.saveWeaponPresetsCalled.Add(1)
	return nil
}

// LoadEquipment returns an error if loadEquipmentErr is set,
// otherwise returns a default-initialized Equipment.
func (m *mockCharSaverFull) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	if m.loadEquipmentErr != nil {
		return nil, m.loadEquipmentErr
	}
	return inventory.NewEquipment(), nil
}

// SaveEquipment records that it was called and always succeeds.
func (m *mockCharSaverFull) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	m.saveEquipmentCalled.Add(1)
	return nil
}

// LoadInventory satisfies CharacterSaver; always returns an empty slice.
func (m *mockCharSaverFull) LoadInventory(_ context.Context, _ int64) ([]inventory.InventoryItem, error) {
	return nil, nil
}

// SaveInventory satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	return nil
}

// HasReceivedStartingInventory satisfies CharacterSaver; always returns false.
func (m *mockCharSaverFull) HasReceivedStartingInventory(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

// MarkStartingInventoryGranted satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) MarkStartingInventoryGranted(_ context.Context, _ int64) error {
	return nil
}

// testGRPCServerWithSaverFull starts an in-process gRPC server using the supplied
// CharacterSaver and returns a connected client and the session manager.
//
// Precondition: t must be non-nil; saver may be nil (no persistence).
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithSaverFull(t *testing.T, saver CharacterSaver) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		saver, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

// joinWorldWithCharID sends a JoinWorldRequest with a non-zero CharacterId and
// returns the initial RoomView.
//
// Precondition: stream must be an open Session stream; characterID must be > 0
// for charSaver to be invoked on login.
// Postcondition: Returns the first RoomView received from the server.
func joinWorldWithCharID(t *testing.T, stream gamev1.GameService_SessionClient, uid, username string, characterID int64) *gamev1.RoomView {
	t.Helper()
	err := stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         uid,
				Username:    username,
				CharacterId: characterID,
			},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	roomView := resp.GetRoomView()
	require.NotNil(t, roomView, "expected RoomView after join")
	return roomView
}

// TestSession_LoadErrorFallback_LoadoutSet verifies that when LoadWeaponPresets
// returns an error the session is assigned a non-nil default LoadoutSet.
func TestSession_LoadErrorFallback_LoadoutSet(t *testing.T) {
	saver := &mockCharSaverFull{
		loadWeaponPresetsErr: errors.New("db unavailable"),
	}
	client, sessMgr := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	// Give the server a moment to complete the login path.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist")
	assert.NotNil(t, sess.LoadoutSet, "LoadoutSet must be non-nil even when load returns an error")
}

// TestSession_LoadErrorFallback_Equipment verifies that when LoadEquipment
// returns an error the session is assigned a non-nil default Equipment.
func TestSession_LoadErrorFallback_Equipment(t *testing.T) {
	saver := &mockCharSaverFull{
		loadEquipmentErr: errors.New("db unavailable"),
	}
	client, sessMgr := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist")
	assert.NotNil(t, sess.Equipment, "Equipment must be non-nil even when load returns an error")
}

// TestSession_Cleanup_SavesWeaponPresetsAndEquipment verifies that
// SaveWeaponPresets and SaveEquipment are invoked during cleanupPlayer.
func TestSession_Cleanup_SavesWeaponPresetsAndEquipment(t *testing.T) {
	saver := &mockCharSaverFull{}
	client, _ := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	// Quit to trigger cleanupPlayer via the deferred call in Session.
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "quit1",
		Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
	})
	require.NoError(t, err)

	// Consume the Disconnected response.
	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetDisconnected())

	// Close the stream so Session() returns and the deferred cleanupPlayer runs.
	_ = stream.CloseSend()

	// Allow time for the deferred cleanup to complete.
	require.Eventually(t, func() bool {
		return saver.saveWeaponPresetsCalled.Load() > 0 &&
			saver.saveEquipmentCalled.Load() > 0
	}, 2*time.Second, 25*time.Millisecond,
		"SaveWeaponPresets and SaveEquipment must each be called during cleanup")
}

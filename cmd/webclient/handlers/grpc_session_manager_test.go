package handlers

import (
	"context"
	"errors"
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// mockGameServiceClient is a hand-written mock implementing gamev1.GameServiceClient.
// Unused methods panic with "not used in tests".
type mockGameServiceClient struct {
	listSessionsResp *gamev1.AdminListSessionsResponse
	listSessionsErr  error
	kickReq          *gamev1.AdminKickRequest
	kickErr          error
	messageReq       *gamev1.AdminMessageRequest
	messageErr       error
	teleportReq      *gamev1.AdminTeleportRequest
	teleportErr      error
}

func (m *mockGameServiceClient) Session(_ context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[gamev1.ClientMessage, gamev1.ServerEvent], error) {
	panic("not used in tests")
}

func (m *mockGameServiceClient) AdminListSessions(_ context.Context, _ *gamev1.AdminListSessionsRequest, _ ...grpc.CallOption) (*gamev1.AdminListSessionsResponse, error) {
	return m.listSessionsResp, m.listSessionsErr
}

func (m *mockGameServiceClient) AdminKickPlayer(_ context.Context, in *gamev1.AdminKickRequest, _ ...grpc.CallOption) (*gamev1.AdminKickResponse, error) {
	m.kickReq = in
	return &gamev1.AdminKickResponse{}, m.kickErr
}

func (m *mockGameServiceClient) AdminMessagePlayer(_ context.Context, in *gamev1.AdminMessageRequest, _ ...grpc.CallOption) (*gamev1.AdminMessageResponse, error) {
	m.messageReq = in
	return &gamev1.AdminMessageResponse{}, m.messageErr
}

func (m *mockGameServiceClient) AdminTeleportPlayer(_ context.Context, in *gamev1.AdminTeleportRequest, _ ...grpc.CallOption) (*gamev1.AdminTeleportResponse, error) {
	m.teleportReq = in
	return &gamev1.AdminTeleportResponse{}, m.teleportErr
}

// TestGRPCSessionManager_AllSessions_MapsFields verifies that two AdminSessionInfo entries
// are correctly mapped to ManagedSession implementations.
func TestGRPCSessionManager_AllSessions_MapsFields(t *testing.T) {
	mock := &mockGameServiceClient{
		listSessionsResp: &gamev1.AdminListSessionsResponse{
			Sessions: []*gamev1.AdminSessionInfo{
				{
					CharId:     101,
					AccountId:  201,
					PlayerName: "Alice",
					Level:      5,
					RoomId:     "room:start",
					Zone:       "Nexus",
					CurrentHp:  42,
				},
				{
					CharId:     102,
					AccountId:  202,
					PlayerName: "Bob",
					Level:      10,
					RoomId:     "room:dungeon",
					Zone:       "Underworld",
					CurrentHp:  99,
				},
			},
		},
	}

	mgr := NewGRPCSessionManager(mock)
	sessions, err := mgr.AllSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 2)

	assert.Equal(t, int64(101), sessions[0].CharID())
	assert.Equal(t, int64(201), sessions[0].AccountID())
	assert.Equal(t, "Alice", sessions[0].PlayerName())
	assert.Equal(t, 5, sessions[0].Level())
	assert.Equal(t, "room:start", sessions[0].RoomID())
	assert.Equal(t, "Nexus", sessions[0].Zone())
	assert.Equal(t, 42, sessions[0].CurrentHP())

	assert.Equal(t, int64(102), sessions[1].CharID())
	assert.Equal(t, int64(202), sessions[1].AccountID())
	assert.Equal(t, "Bob", sessions[1].PlayerName())
	assert.Equal(t, 10, sessions[1].Level())
	assert.Equal(t, "room:dungeon", sessions[1].RoomID())
	assert.Equal(t, "Underworld", sessions[1].Zone())
	assert.Equal(t, 99, sessions[1].CurrentHP())
}

// TestGRPCSessionManager_AllSessions_RPCError_ReturnsError verifies that an RPC error
// causes AllSessions to return nil sessions and a non-nil error.
func TestGRPCSessionManager_AllSessions_RPCError_ReturnsError(t *testing.T) {
	mock := &mockGameServiceClient{
		listSessionsErr: errors.New("connection refused"),
	}

	mgr := NewGRPCSessionManager(mock)
	sessions, err := mgr.AllSessions()

	assert.Nil(t, sessions)
	assert.Error(t, err)
	assert.Equal(t, "connection refused", err.Error())
}

// TestGRPCSessionManager_GetSession_Found verifies that a session with a matching
// charID is returned.
func TestGRPCSessionManager_GetSession_Found(t *testing.T) {
	mock := &mockGameServiceClient{
		listSessionsResp: &gamev1.AdminListSessionsResponse{
			Sessions: []*gamev1.AdminSessionInfo{
				{CharId: 42, PlayerName: "Target"},
				{CharId: 99, PlayerName: "Other"},
			},
		},
	}

	mgr := NewGRPCSessionManager(mock)
	sess, found := mgr.GetSession(42)

	require.True(t, found)
	require.NotNil(t, sess)
	assert.Equal(t, int64(42), sess.CharID())
	assert.Equal(t, "Target", sess.PlayerName())
}

// TestGRPCSessionManager_GetSession_NotFound verifies that a missing charID
// returns nil, false.
func TestGRPCSessionManager_GetSession_NotFound(t *testing.T) {
	mock := &mockGameServiceClient{
		listSessionsResp: &gamev1.AdminListSessionsResponse{
			Sessions: []*gamev1.AdminSessionInfo{
				{CharId: 1, PlayerName: "Someone"},
			},
		},
	}

	mgr := NewGRPCSessionManager(mock)
	sess, found := mgr.GetSession(9999)

	assert.False(t, found)
	assert.Nil(t, sess)
}

// TestGRPCSessionManager_TeleportPlayer_CallsRPC verifies that TeleportPlayer
// invokes AdminTeleportPlayer with the correct charID and roomID.
func TestGRPCSessionManager_TeleportPlayer_CallsRPC(t *testing.T) {
	mock := &mockGameServiceClient{}
	mgr := NewGRPCSessionManager(mock)

	err := mgr.TeleportPlayer(77, "room:throne")

	require.NoError(t, err)
	require.NotNil(t, mock.teleportReq)
	assert.Equal(t, int64(77), mock.teleportReq.CharId)
	assert.Equal(t, "room:throne", mock.teleportReq.RoomId)
}

// TestGRPCManagedSession_Kick_CallsRPC verifies that Kick invokes AdminKickPlayer
// with the correct charID.
func TestGRPCManagedSession_Kick_CallsRPC(t *testing.T) {
	mock := &mockGameServiceClient{}
	sess := &grpcManagedSession{
		info:   &gamev1.AdminSessionInfo{CharId: 55},
		client: mock,
	}

	err := sess.Kick()

	require.NoError(t, err)
	require.NotNil(t, mock.kickReq)
	assert.Equal(t, int64(55), mock.kickReq.CharId)
}

// TestGRPCManagedSession_SendAdminMessage_CallsRPC verifies that SendAdminMessage
// invokes AdminMessagePlayer with the correct charID and text.
func TestGRPCManagedSession_SendAdminMessage_CallsRPC(t *testing.T) {
	mock := &mockGameServiceClient{}
	sess := &grpcManagedSession{
		info:   &gamev1.AdminSessionInfo{CharId: 33},
		client: mock,
	}

	err := sess.SendAdminMessage("hello admin world")

	require.NoError(t, err)
	require.NotNil(t, mock.messageReq)
	assert.Equal(t, int64(33), mock.messageReq.CharId)
	assert.Equal(t, "hello admin world", mock.messageReq.Text)
}

// TestGRPCManagedSession_Kick_RPCError verifies that Kick propagates the RPC error
// returned by AdminKickPlayer.
func TestGRPCManagedSession_Kick_RPCError(t *testing.T) {
	mock := &mockGameServiceClient{
		kickErr: errors.New("kick rpc failed"),
	}
	sess := &grpcManagedSession{
		info:   &gamev1.AdminSessionInfo{CharId: 55},
		client: mock,
	}

	err := sess.Kick()

	assert.Error(t, err)
	assert.Equal(t, "kick rpc failed", err.Error())
}

// TestGRPCManagedSession_SendAdminMessage_RPCError verifies that SendAdminMessage
// propagates the RPC error returned by AdminMessagePlayer.
func TestGRPCManagedSession_SendAdminMessage_RPCError(t *testing.T) {
	mock := &mockGameServiceClient{
		messageErr: errors.New("message rpc failed"),
	}
	sess := &grpcManagedSession{
		info:   &gamev1.AdminSessionInfo{CharId: 33},
		client: mock,
	}

	err := sess.SendAdminMessage("hello")

	assert.Error(t, err)
	assert.Equal(t, "message rpc failed", err.Error())
}

package handlers

import (
	"context"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcSessionManager implements SessionManager via the gameserver gRPC admin RPCs.
// Precondition: client must be non-nil.
type grpcSessionManager struct {
	client gamev1.GameServiceClient
}

// NewGRPCSessionManager returns a SessionManager backed by the gameserver gRPC admin RPCs.
func NewGRPCSessionManager(client gamev1.GameServiceClient) SessionManager {
	return &grpcSessionManager{client: client}
}

// AllSessions calls AdminListSessions and returns mapped ManagedSession slice.
// On RPC error, returns nil (HTTP layer propagates as 502).
func (g *grpcSessionManager) AllSessions() []ManagedSession {
	resp, err := g.client.AdminListSessions(context.Background(), &gamev1.AdminListSessionsRequest{})
	if err != nil {
		return nil
	}
	out := make([]ManagedSession, 0, len(resp.Sessions))
	for _, info := range resp.Sessions {
		out = append(out, &grpcManagedSession{info: info, client: g.client})
	}
	return out
}

// GetSession finds the session with the given charID.
func (g *grpcSessionManager) GetSession(charID int64) (ManagedSession, bool) {
	sessions := g.AllSessions()
	for _, s := range sessions {
		if s.CharID() == charID {
			return s, true
		}
	}
	return nil, false
}

// TeleportPlayer calls AdminTeleportPlayer RPC.
func (g *grpcSessionManager) TeleportPlayer(charID int64, roomID string) error {
	_, err := g.client.AdminTeleportPlayer(context.Background(), &gamev1.AdminTeleportRequest{
		CharId: charID,
		RoomId: roomID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return err
		}
		return err
	}
	return nil
}

// grpcManagedSession caches one AdminSessionInfo and holds the gRPC client for push ops.
type grpcManagedSession struct {
	info   *gamev1.AdminSessionInfo
	client gamev1.GameServiceClient
}

func (s *grpcManagedSession) CharID() int64      { return s.info.CharId }
func (s *grpcManagedSession) AccountID() int64   { return s.info.AccountId }
func (s *grpcManagedSession) PlayerName() string { return s.info.PlayerName }
func (s *grpcManagedSession) Level() int         { return int(s.info.Level) }
func (s *grpcManagedSession) RoomID() string     { return s.info.RoomId }
func (s *grpcManagedSession) Zone() string       { return s.info.Zone }
func (s *grpcManagedSession) CurrentHP() int     { return int(s.info.CurrentHp) }

func (s *grpcManagedSession) SendAdminMessage(text string) error {
	_, err := s.client.AdminMessagePlayer(context.Background(), &gamev1.AdminMessageRequest{
		CharId: s.info.CharId,
		Text:   text,
	})
	return err
}

func (s *grpcManagedSession) Kick() error {
	_, err := s.client.AdminKickPlayer(context.Background(), &gamev1.AdminKickRequest{
		CharId: s.info.CharId,
	})
	return err
}

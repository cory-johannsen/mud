package handlers

import (
	"context"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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
// On RPC error, returns nil and the error so the HTTP layer can respond with 502.
func (g *grpcSessionManager) AllSessions() ([]ManagedSession, error) {
	resp, err := g.client.AdminListSessions(context.Background(), &gamev1.AdminListSessionsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]ManagedSession, 0, len(resp.Sessions))
	for _, info := range resp.Sessions {
		out = append(out, &grpcManagedSession{info: info, client: g.client})
	}
	return out, nil
}

// GetSession finds the session with the given charID.
// On RPC error, returns nil, false — the session is effectively not found.
func (g *grpcSessionManager) GetSession(charID int64) (ManagedSession, bool) {
	sessions, err := g.AllSessions()
	if err != nil {
		return nil, false
	}
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

// GiveItem calls AdminGiveItem RPC.
func (g *grpcSessionManager) GiveItem(charID int64, itemID string, quantity int) error {
	_, err := g.client.AdminGiveItem(context.Background(), &gamev1.AdminGiveItemRequest{
		CharId:   charID,
		ItemId:   itemID,
		Quantity: int32(quantity),
	})
	return err
}

// GiveCurrency calls AdminGiveCurrency RPC.
func (g *grpcSessionManager) GiveCurrency(charID int64, amount int) error {
	_, err := g.client.AdminGiveCurrency(context.Background(), &gamev1.AdminGiveCurrencyRequest{
		CharId: charID,
		Amount: int32(amount),
	})
	return err
}

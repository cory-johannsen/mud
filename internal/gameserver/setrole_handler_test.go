package gameserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

type mockAccountAdmin struct {
	accounts map[string]AccountInfo
}

func (m *mockAccountAdmin) GetAccountByUsername(_ context.Context, username string) (AccountInfo, error) {
	acct, ok := m.accounts[username]
	if !ok {
		return AccountInfo{}, fmt.Errorf("account %q not found", username)
	}
	return acct, nil
}

func (m *mockAccountAdmin) SetAccountRole(_ context.Context, accountID int64, role string) error {
	for k, acct := range m.accounts {
		if acct.ID == accountID {
			acct.Role = role
			m.accounts[k] = acct
			return nil
		}
	}
	return fmt.Errorf("account not found")
}

func testServiceWithAdmin(t *testing.T, admin AccountAdmin) *GameServiceServer {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	return NewGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, admin, nil, nil, nil)
}

func TestHandleSetRole_AdminSuccess(t *testing.T) {
	admin := &mockAccountAdmin{accounts: map[string]AccountInfo{
		"target": {ID: 2, Username: "target", Role: "player"},
	}}
	svc := testServiceWithAdmin(t, admin)

	_, err := svc.sessions.AddPlayer("u1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleSetRole("u1", &gamev1.SetRoleRequest{
		TargetUsername: "target",
		Role:           "editor",
	})
	require.NoError(t, err)
	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Set role for target")
	assert.Equal(t, "editor", admin.accounts["target"].Role)
}

func TestHandleSetRole_NonAdminDenied(t *testing.T) {
	svc := testServiceWithAdmin(t, nil)

	_, err := svc.sessions.AddPlayer("u1", "player_user", "Player", 1, "room_a", 10, "player", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleSetRole("u1", &gamev1.SetRoleRequest{
		TargetUsername: "someone",
		Role:           "admin",
	})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "permission denied")
}

func TestHandleSetRole_InvalidArgs(t *testing.T) {
	svc := testServiceWithAdmin(t, &mockAccountAdmin{})

	_, err := svc.sessions.AddPlayer("u1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleSetRole("u1", &gamev1.SetRoleRequest{})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "usage")
}

func TestHandleSetRole_TargetNotFound(t *testing.T) {
	admin := &mockAccountAdmin{accounts: map[string]AccountInfo{}}
	svc := testServiceWithAdmin(t, admin)

	_, err := svc.sessions.AddPlayer("u1", "admin_user", "Admin", 1, "room_a", 10, "admin", "", "", 0)
	require.NoError(t, err)

	resp, err := svc.handleSetRole("u1", &gamev1.SetRoleRequest{
		TargetUsername: "nobody",
		Role:           "editor",
	})
	require.NoError(t, err)
	errEvt := resp.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "not found")
}

// Property: non-admin roles always get permission denied.
func TestPropertySetRole_NonAdminAlwaysDenied(t *testing.T) {
	admin := &mockAccountAdmin{accounts: map[string]AccountInfo{}}
	svc := testServiceWithAdmin(t, admin)
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.SampledFrom([]string{"player", "editor"}).Draw(t, "role")

		uid := fmt.Sprintf("u_%s_%d", role, rapid.IntRange(0, 99999).Draw(t, "uid"))
		_, err := svc.sessions.AddPlayer(uid, "user", "User", 1, "room_a", 10, role, "", "", 0)
		if err != nil {
			t.Fatalf("AddPlayer: %v", err)
		}
		defer func() { _ = svc.sessions.RemovePlayer(uid) }()

		resp, err := svc.handleSetRole(uid, &gamev1.SetRoleRequest{
			TargetUsername: "anyone",
			Role:           "admin",
		})
		if err != nil {
			t.Fatalf("handleSetRole: %v", err)
		}
		errEvt := resp.GetError()
		if errEvt == nil {
			t.Fatal("expected error event for non-admin")
		}
	})
}

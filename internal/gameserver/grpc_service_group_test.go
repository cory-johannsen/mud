package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// newGroupSvc mirrors newJoinSvc — same constructor signature.
func newGroupSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// addGroupPlayer adds a player session with the given uid and charName.
func addGroupPlayer(t *testing.T, sessMgr *session.Manager, uid, charName string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: charName,
		RoomID:   "room-1",
		Role:     "player",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	return sess
}

// ---------------------------------------------------------------------------
// handleGroup tests
// ---------------------------------------------------------------------------

// TestHandleGroup_NoArgs_NotInGroup: no args and not in group → "You are not in a group."
func TestHandleGroup_NoArgs_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g1", "Alice")

	resp, err := svc.handleGroup("u_g1", &gamev1.GroupRequest{Args: ""})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "not in a group")
}

// TestHandleGroup_NoArgs_InGroup: no args when in group → shows leader and members.
func TestHandleGroup_NoArgs_InGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g2", "Alice")
	addGroupPlayer(t, sessMgr, "u_g3", "Bob")

	g := sessMgr.CreateGroup("u_g2")
	require.NotNil(t, g)
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_g3"))

	resp, err := svc.handleGroup("u_g2", &gamev1.GroupRequest{Args: ""})
	require.NoError(t, err)
	require.NotNil(t, resp)
	content := resp.GetMessage().Content
	assert.Contains(t, content, "Group")
	assert.Contains(t, content, "Alice")
}

// TestHandleGroup_WithArg_CreatesGroupAndInvites: group <player> creates group and sets PendingGroupInvite.
func TestHandleGroup_WithArg_CreatesGroupAndInvites(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g4", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_g5", "Bob")

	resp, err := svc.handleGroup("u_g4", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "created a group")
	assert.Contains(t, resp.GetMessage().Content, "Bob")
	assert.NotEmpty(t, bobSess.PendingGroupInvite)
}

// TestHandleGroup_WithArg_AlreadyInGroup: caller in group → "You are already in a group."
func TestHandleGroup_WithArg_AlreadyInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g6", "Alice")
	addGroupPlayer(t, sessMgr, "u_g7", "Bob")

	sessMgr.CreateGroup("u_g6")

	resp, err := svc.handleGroup("u_g6", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "already in a group")
}

// TestHandleGroup_WithArg_SelfInvite: arg matches caller CharName → "You cannot invite yourself."
func TestHandleGroup_WithArg_SelfInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g8", "Alice")

	resp, err := svc.handleGroup("u_g8", &gamev1.GroupRequest{Args: "Alice"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "cannot invite yourself")
}

// TestHandleGroup_WithArg_TargetNotOnline: target not found → "Player not found."
func TestHandleGroup_WithArg_TargetNotOnline(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g9", "Alice")

	resp, err := svc.handleGroup("u_g9", &gamev1.GroupRequest{Args: "Charlie"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "not found")
}

// TestHandleGroup_WithArg_TargetAlreadyInGroup: target.GroupID != "" → "<name> is already in a group."
func TestHandleGroup_WithArg_TargetAlreadyInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g10", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_g11", "Bob")
	addGroupPlayer(t, sessMgr, "u_g12", "Carol")

	// Put Bob in a group first.
	g := sessMgr.CreateGroup("u_g12")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_g11"))
	require.NotEmpty(t, bobSess.GroupID)

	resp, err := svc.handleGroup("u_g10", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "already in a group")
}

// TestHandleGroup_WithArg_TargetHasPendingInvite: target has pending invite → "<name> already has a pending group invitation."
func TestHandleGroup_WithArg_TargetHasPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_g13", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_g14", "Bob")
	bobSess.PendingGroupInvite = "some-group-id"

	resp, err := svc.handleGroup("u_g13", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "pending group invitation")
}

// ---------------------------------------------------------------------------
// handleInvite tests
// ---------------------------------------------------------------------------

// TestHandleInvite_NotInGroup: caller not in a group → "You are not in a group."
func TestHandleInvite_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_i1", "Alice")

	resp, err := svc.handleInvite("u_i1", &gamev1.InviteRequest{Player: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "not in a group")
}

// TestHandleInvite_NotLeader: caller in group but not leader → "Only the group leader can invite players."
func TestHandleInvite_NotLeader(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_i2", "Alice")
	addGroupPlayer(t, sessMgr, "u_i3", "Bob")
	addGroupPlayer(t, sessMgr, "u_i4", "Carol")

	g := sessMgr.CreateGroup("u_i2")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_i3"))

	resp, err := svc.handleInvite("u_i3", &gamev1.InviteRequest{Player: "Carol"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "Only the group leader")
}

// TestHandleInvite_GroupFull: group at 8 members → "Group is full (max 8 members)."
func TestHandleInvite_GroupFull(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	// Create 9 players: leader + 7 members = 8 (full) + 1 target.
	addGroupPlayer(t, sessMgr, "u_if0", "Leader")
	g := sessMgr.CreateGroup("u_if0")
	for i := 1; i <= 7; i++ {
		uid := "u_if" + string(rune('0'+i))
		addGroupPlayer(t, sessMgr, uid, "Member"+string(rune('A'+i-1)))
		require.NoError(t, sessMgr.AddGroupMember(g.ID, uid))
	}
	addGroupPlayer(t, sessMgr, "u_if_target", "Target")

	resp, err := svc.handleInvite("u_if0", &gamev1.InviteRequest{Player: "Target"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "full")
}

// ---------------------------------------------------------------------------
// handleAcceptGroup tests
// ---------------------------------------------------------------------------

// TestHandleAcceptGroup_NoPendingInvite: no pending invite → "You have no pending group invitation."
func TestHandleAcceptGroup_NoPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_a1", "Alice")

	resp, err := svc.handleAcceptGroup("u_a1", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "no pending group invitation")
}

// TestHandleAcceptGroup_GroupGone: invite references nonexistent group → clears invite, returns error msg.
func TestHandleAcceptGroup_GroupGone(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	sess := addGroupPlayer(t, sessMgr, "u_a2", "Alice")
	sess.PendingGroupInvite = "nonexistent-group-id"

	resp, err := svc.handleAcceptGroup("u_a2", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "no longer exists")
	assert.Empty(t, sess.PendingGroupInvite)
}

// TestHandleAcceptGroup_Success: accept with pending invite → joined, PendingGroupInvite cleared, GroupID set.
func TestHandleAcceptGroup_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_a3", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_a4", "Bob")

	g := sessMgr.CreateGroup("u_a3")
	bobSess.PendingGroupInvite = g.ID

	resp, err := svc.handleAcceptGroup("u_a4", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "joined")
	assert.Empty(t, bobSess.PendingGroupInvite)
	assert.Equal(t, g.ID, bobSess.GroupID)
}

// ---------------------------------------------------------------------------
// handleDeclineGroup tests
// ---------------------------------------------------------------------------

// TestHandleDeclineGroup_NoPendingInvite: no invite → "You have no pending group invitation."
func TestHandleDeclineGroup_NoPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_d1", "Alice")

	resp, err := svc.handleDeclineGroup("u_d1", &gamev1.DeclineGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "no pending group invitation")
}

// TestHandleDeclineGroup_Success: decline → PendingGroupInvite cleared, success message returned.
func TestHandleDeclineGroup_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_d2", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_d3", "Bob")

	g := sessMgr.CreateGroup("u_d2")
	bobSess.PendingGroupInvite = g.ID

	resp, err := svc.handleDeclineGroup("u_d3", &gamev1.DeclineGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.GetMessage().Content, "declined")
	assert.Empty(t, bobSess.PendingGroupInvite)
}

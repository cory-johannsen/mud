package gameserver

import (
	"fmt"
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
	// Note: leader notification is delivered via session.Push, which writes to the player's
	// gRPC stream. In unit tests there is no real stream attached to the session, so pushed
	// messages to other players cannot be directly asserted here. Integration / e2e tests
	// are required to verify that Alice receives the "Bob declined your group invitation." push.
}

// ---------------------------------------------------------------------------
// handleInvite — additional scenario tests
// ---------------------------------------------------------------------------

// TestHandleInvite_SelfInvite verifies self-invite is rejected.
func TestHandleInvite_SelfInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u1_si", "Alice")
	sessMgr.CreateGroup("u1_si")

	evt, err := svc.handleInvite("u1_si", &gamev1.InviteRequest{Player: "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "You cannot invite yourself.", evt.GetMessage().GetContent())
}

// TestHandleInvite_TargetNotOnline verifies that inviting an offline player returns "Player not found."
func TestHandleInvite_TargetNotOnline(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u1_tno", "Alice")
	sessMgr.CreateGroup("u1_tno")

	evt, err := svc.handleInvite("u1_tno", &gamev1.InviteRequest{Player: "Ghost"})
	require.NoError(t, err)
	assert.Equal(t, "Player not found.", evt.GetMessage().GetContent())
}

// TestHandleInvite_TargetAlreadyInGroup verifies that inviting a player in a group is rejected.
func TestHandleInvite_TargetAlreadyInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u1_taig", "Alice")
	addGroupPlayer(t, sessMgr, "u2_taig", "Bob")
	sessMgr.CreateGroup("u1_taig")
	sessMgr.CreateGroup("u2_taig") // Bob creates his own group

	evt, err := svc.handleInvite("u1_taig", &gamev1.InviteRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob is already in a group.", evt.GetMessage().GetContent())
}

// TestHandleInvite_TargetHasPendingInvite verifies that inviting a player with a pending invite is rejected.
func TestHandleInvite_TargetHasPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u1_thpi", "Alice")
	target := addGroupPlayer(t, sessMgr, "u2_thpi", "Bob")
	sessMgr.CreateGroup("u1_thpi")
	target.PendingGroupInvite = "some-other-group"

	evt, err := svc.handleInvite("u1_thpi", &gamev1.InviteRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob already has a pending group invitation.", evt.GetMessage().GetContent())
}

// TestHandleInvite_Success verifies that a valid invite sets target.PendingGroupInvite.
func TestHandleInvite_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u1_is", "Alice")
	target := addGroupPlayer(t, sessMgr, "u2_is", "Bob")
	group := sessMgr.CreateGroup("u1_is")

	evt, err := svc.handleInvite("u1_is", &gamev1.InviteRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "You invited Bob to the group.", evt.GetMessage().GetContent())
	assert.Equal(t, group.ID, target.PendingGroupInvite)
}

// ---------------------------------------------------------------------------
// handleAcceptGroup — group-full path
// ---------------------------------------------------------------------------

// TestHandleAcceptGroup_GroupFull verifies that accept when group is at capacity clears invite.
func TestHandleAcceptGroup_GroupFull(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "leader_agf", "Leader")
	group := sessMgr.CreateGroup("leader_agf")

	// Fill group to 8 members (leader + 7 additional).
	for i := 1; i <= 7; i++ {
		uid := fmt.Sprintf("m%d_agf", i)
		addGroupPlayer(t, sessMgr, uid, fmt.Sprintf("M%d", i))
		require.NoError(t, sessMgr.AddGroupMember(group.ID, uid))
	}

	// Add a 9th player with a pending invite.
	target := addGroupPlayer(t, sessMgr, "extra_agf", "Extra")
	target.PendingGroupInvite = group.ID

	evt, err := svc.handleAcceptGroup("extra_agf", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "The group is full.", evt.GetMessage().GetContent())
	assert.Empty(t, target.PendingGroupInvite)
}

// ---------------------------------------------------------------------------
// handleUngroup tests
// ---------------------------------------------------------------------------

// TestHandleUngroup_LeaderDisbands: leader calls ungroup → all members' GroupID cleared,
// group removed, leader sees "You disbanded the group.", other members see disband message.
func TestHandleUngroup_LeaderDisbands(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_ul1", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_ul2", "Bob")

	g := sessMgr.CreateGroup("u_ul1")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_ul2"))

	evt, err := svc.handleUngroup("u_ul1", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You disbanded the group.", evt.GetMessage().GetContent())

	// Note: push notifications to remaining members cannot be asserted in unit tests
	// because sessions have no real gRPC stream attached. The notification logic is verified
	// by integration tests and code inspection.

	// Both members should have their GroupID cleared.
	aliceSess, _ := sessMgr.GetPlayer("u_ul1")
	assert.Empty(t, aliceSess.GroupID)
	assert.Empty(t, bobSess.GroupID)

	// Group should no longer exist.
	_, exists := sessMgr.GroupByID(g.ID)
	assert.False(t, exists)
}

// TestHandleUngroup_NonLeaderLeaves: non-leader calls ungroup → caller's GroupID cleared,
// remaining members get "<name> left the group.", caller sees "You left the group."
func TestHandleUngroup_NonLeaderLeaves(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	aliceSess := addGroupPlayer(t, sessMgr, "u_unl1", "Alice")
	addGroupPlayer(t, sessMgr, "u_unl2", "Bob")

	g := sessMgr.CreateGroup("u_unl1")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_unl2"))

	evt, err := svc.handleUngroup("u_unl2", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You left the group.", evt.GetMessage().GetContent())

	// Note: push notifications to remaining members cannot be asserted in unit tests
	// because sessions have no real gRPC stream attached. The notification logic is verified
	// by integration tests and code inspection.

	// Bob's GroupID cleared, Alice's GroupID remains.
	bobSess, _ := sessMgr.GetPlayer("u_unl2")
	assert.Empty(t, bobSess.GroupID)
	assert.NotEmpty(t, aliceSess.GroupID)

	// Group should still exist with Alice.
	grp, exists := sessMgr.GroupByID(g.ID)
	require.True(t, exists)
	assert.Len(t, grp.MemberUIDs, 1)
	assert.Equal(t, "u_unl1", grp.MemberUIDs[0])
}

// TestHandleUngroup_NotInGroup: player not in group → "You are not in a group."
func TestHandleUngroup_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_unig1", "Alice")

	evt, err := svc.handleUngroup("u_unig1", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().GetContent(), "not in a group")
}

// ---------------------------------------------------------------------------
// handleKick tests
// ---------------------------------------------------------------------------

// TestHandleKick_Success: leader kicks member → target.GroupID cleared,
// target removed from MemberUIDs, caller sees kick confirmation, target sees kick message.
func TestHandleKick_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_ks1", "Alice")
	bobSess := addGroupPlayer(t, sessMgr, "u_ks2", "Bob")

	g := sessMgr.CreateGroup("u_ks1")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_ks2"))

	evt, err := svc.handleKick("u_ks1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You kicked Bob from the group.", evt.GetMessage().GetContent())

	// Note: push notifications to target and remaining members cannot be asserted in unit tests
	// because sessions have no real gRPC stream attached. The notification logic is verified
	// by integration tests and code inspection.

	// Bob's GroupID should be cleared.
	assert.Empty(t, bobSess.GroupID)

	// Group should still exist with only Alice.
	grp, exists := sessMgr.GroupByID(g.ID)
	require.True(t, exists)
	assert.Len(t, grp.MemberUIDs, 1)
	assert.Equal(t, "u_ks1", grp.MemberUIDs[0])
}

// TestHandleKick_NotLeader: non-leader tries to kick → "Only the group leader can kick members."
func TestHandleKick_NotLeader(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_knl1", "Alice")
	addGroupPlayer(t, sessMgr, "u_knl2", "Bob")
	addGroupPlayer(t, sessMgr, "u_knl3", "Carol")

	g := sessMgr.CreateGroup("u_knl1")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_knl2"))
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_knl3"))

	evt, err := svc.handleKick("u_knl2", &gamev1.KickRequest{Player: "Carol"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "Only the group leader can kick members.", evt.GetMessage().GetContent())
}

// TestHandleKick_TargetNotInGroup: target not in group → "<name> is not in your group."
func TestHandleKick_TargetNotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_ktnig1", "Alice")
	addGroupPlayer(t, sessMgr, "u_ktnig2", "Bob")

	sessMgr.CreateGroup("u_ktnig1")

	evt, err := svc.handleKick("u_ktnig1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().GetContent(), "not in your group")
}

// TestHandleKick_SelfKick: leader tries to kick themselves → "Use 'ungroup' to disband the group."
func TestHandleKick_SelfKick(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_ksk1", "Alice")
	addGroupPlayer(t, sessMgr, "u_ksk2", "Bob")

	g := sessMgr.CreateGroup("u_ksk1")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "u_ksk2"))

	evt, err := svc.handleKick("u_ksk1", &gamev1.KickRequest{Player: "Alice"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "Use 'ungroup' to disband the group.", evt.GetMessage().GetContent())
}

// TestHandleKick_NotInGroup: caller not in group → "You are not in a group."
func TestHandleKick_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "u_knig1", "Alice")

	evt, err := svc.handleKick("u_knig1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().GetContent(), "not in a group")
}

package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupPlayer_LeaderDisconnect_DisbandGroup verifies REQ-T11:
// when the group leader disconnects, the group is disbanded and all members' GroupID cleared.
func TestCleanupPlayer_LeaderDisconnect_DisbandGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "leader", "Alice")
	member := addGroupPlayer(t, sessMgr, "member", "Bob")

	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	svc.cleanupPlayer("leader", "alice")

	assert.Empty(t, member.GroupID, "member GroupID should be cleared after leader disconnects")
	_, exists := sessMgr.GroupByID(group.ID)
	assert.False(t, exists, "group should be removed from manager after leader disconnects")
}

// TestCleanupPlayer_InviteePendingInvite_InviteCleared verifies REQ-T12:
// when a player with a pending invite disconnects, PendingGroupInvite is cleared.
func TestCleanupPlayer_InviteePendingInvite_InviteCleared(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupPlayer(t, sessMgr, "leader", "Alice")
	invitee := addGroupPlayer(t, sessMgr, "invitee", "Bob")

	group := sessMgr.CreateGroup("leader")
	invitee.PendingGroupInvite = group.ID

	svc.cleanupPlayer("invitee", "bob")

	assert.Empty(t, invitee.PendingGroupInvite, "PendingGroupInvite must be cleared on disconnect")
}

// TestCleanupPlayer_NonLeaderDisconnect_LeaderGroupPreserved verifies that when a non-leader
// disconnects, only that player leaves and the leader's GroupID is unchanged.
func TestCleanupPlayer_NonLeaderDisconnect_LeaderGroupPreserved(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	leader := addGroupPlayer(t, sessMgr, "leader", "Alice")
	addGroupPlayer(t, sessMgr, "member", "Bob")

	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	svc.cleanupPlayer("member", "bob")

	assert.NotEmpty(t, leader.GroupID, "leader GroupID must remain after non-leader disconnects")
	g, exists := sessMgr.GroupByID(group.ID)
	require.True(t, exists, "group must still exist after non-leader disconnects")
	for _, uid := range g.MemberUIDs {
		assert.NotEqual(t, "member", uid, "disconnected member must be removed from MemberUIDs")
	}
}

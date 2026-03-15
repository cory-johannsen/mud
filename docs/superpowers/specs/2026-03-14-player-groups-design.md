# Player Groups — Design Spec

**Date:** 2026-03-14

---

## Goal

Players can form persistent-session groups. Group members in the same room automatically enter combat together when any member initiates a fight. Members in other rooms are notified. Groups dissolve when the leader logs out or disbands.

---

## Feature 1: Data Model

### `Group` struct (new, `internal/game/session/`)

```go
// Group represents a player party. Groups are session-only and not persisted.
type Group struct {
    ID         string   // UUID, assigned at creation
    LeaderUID  string   // UID of the group leader
    MemberUIDs []string // All members including leader; append-only until removal
}
```

### `Manager` additions (`internal/game/session/manager.go`)

```go
// groups maps groupID → *Group. Protected by mu (existing RWMutex).
groups map[string]*Group
```

New methods:
- `CreateGroup(leaderUID string) *Group` — allocates a UUID, stores group, returns it
- `DisbandGroup(groupID string)` — deletes group entry and clears GroupID on all member sessions
- `AddGroupMember(groupID, uid string) error` — appends UID; error if already a member
- `RemoveGroupMember(groupID, uid string)` — removes UID; if leader and only member, disbands
- `GroupByUID(uid string) *Group` — linear scan of groups for membership; returns nil if not in a group
- `GroupByID(groupID string) (*Group, bool)` — O(1) map lookup

### `PlayerSession` additions (`internal/game/session/manager.go`)

```go
// GroupID is the ID of the group this player belongs to. Empty = not in a group.
GroupID string

// PendingGroupInvite holds the groupID of a pending group invitation.
// Empty = no pending invite. Cleared on accept, decline, or group disband.
PendingGroupInvite string
```

---

## Feature 2: Commands

All commands follow CMD-1–7 (handler constant, BuiltinCommands entry, Handle func, proto message, bridge handler, gRPC dispatch, tests).

### `group` — create or list group

- **No args:** Print current group members and leader. If not in a group: `"You are not in a group."`
- **With `<player>`:** Create a new group (caller is leader), send invitation to named player.
  - Error if caller is already in a group: `"You are already in a group. Use 'ungroup' to leave first."`
  - Error if target not online: `"Player not found."`
  - Error if target already in a group: `"<name> is already in a group."`
  - On success: caller sees `"You created a group and invited <name>."` Target receives: `"<leader> has invited you to join their group. (accept / decline)"`

### `invite` — invite player to existing group

- **Leader only.** `invite <player>`
- Error if caller is not in a group: `"You are not in a group."`
- Error if caller is not the leader: `"Only the group leader can invite players."`
- Error if target not online or already in a group: same messages as above.
- Error if target already has a pending invite: `"<name> already has a pending group invitation."`
- On success: caller sees `"You invited <name> to the group."` Target receives join prompt.

### `accept` — accept pending group invite

- Clears `PendingGroupInvite`, adds player to group.
- Error if no pending invite: `"You have no pending group invitation."`
- Error if group no longer exists (leader logged out): `"That group no longer exists."` — clears pending.
- On success: player joins group; all existing members notified: `"<name> joined the group."`

### `decline` — decline pending group invite

- Clears `PendingGroupInvite`.
- Error if no pending invite: `"You have no pending group invitation."`
- On success: player sees `"You declined the group invitation."` Leader notified: `"<name> declined your group invitation."`

### `ungroup` — leave group

- **Non-leader:** Removes self from group. Remaining members notified: `"<name> left the group."` Leader notified.
- **Leader:** Disbands entire group. All members notified: `"The group has been disbanded by <leader>."` All `GroupID` fields cleared.
- If not in a group: `"You are not in a group."`

### `kick` — remove a member

- **Leader only.** `kick <player>`
- Error if caller is not leader: `"Only the group leader can kick members."`
- Error if target is not in the group: `"<name> is not in your group."`
- Error if target is the leader (self-kick): `"Use 'ungroup' to disband the group."`
- On success: target removed, sees `"You were kicked from the group."` Remaining members notified: `"<name> was kicked from the group."`

---

## Feature 3: Auto-Combat

**Location:** `internal/gameserver/combat_handler.go`, in `startCombatLocked`, after the initial combat is started.

**Precondition:** Initiating player is in a group (`sess.GroupID != ""`).

**Algorithm:**

```
group := sessionMgr.GroupByUID(sess.UID)
for each memberUID in group.MemberUIDs where memberUID != sess.UID:
    memberSess, ok := sessionMgr.GetPlayer(memberUID)
    if !ok: continue  // offline
    if memberSess.Status == statusInCombat: continue  // already fighting
    if memberSess.RoomID == sess.RoomID:
        // Same room — auto-join as combatant
        build member combatant (same pattern as handleJoin)
        RollInitiative([memberCbt], dice.Src())
        engine.AddCombatant(roomID, memberCbt)
        memberSess.Status = statusInCombat
        push message to member: "Your group entered combat! You join the fight (initiative <n>)."
    else:
        // Different room — notify only
        push message to member: "Your group is under attack in <roomName>!"
```

**Locking:** `startCombatLocked` is called with `combatMu` already held. `AddCombatant` acquires `engine.mu` internally — no deadlock.

---

## Feature 4: Leader Logout Cleanup

When a player disconnects (`RemovePlayer` in session manager), if `sess.GroupID != ""`:
- If the player is the group leader: disband the group (notify all members online).
- If the player is a non-leader member: remove them from the group (notify remaining members).

This is handled in `grpc_service.go` where disconnect is currently processed, or in a `SetOnDisconnect` hook on the session manager.

---

## Proto Messages

Add to `api/proto/game/v1/game.proto` (next available field numbers after 74):

```protobuf
message GroupRequest { string args = 1; }   // field 75
message InviteRequest { string player = 1; } // field 76
message AcceptRequest {}                      // field 77
message DeclineGroupRequest {}                // field 78
message UngroupRequest {}                     // field 79
message KickRequest { string player = 1; }   // field 80
```

Note: `DeclineGroupRequest` is distinct from the existing `DeclineRequest` (combat decline).

---

## Testing

- REQ-T1 (example): `CreateGroup` sets leader, adds leader to MemberUIDs, stores in manager.
- REQ-T2 (example): `group <player>` sends invite; target's `PendingGroupInvite` is set.
- REQ-T3 (example): `accept` adds player to group; existing members notified.
- REQ-T4 (example): `decline` clears `PendingGroupInvite`; leader notified.
- REQ-T5 (example): `ungroup` by leader disbands group; all members' `GroupID` cleared.
- REQ-T6 (example): `ungroup` by non-leader removes only that player.
- REQ-T7 (example): `kick <player>` removes target; target's `GroupID` cleared.
- REQ-T8 (example): `invite` by non-leader returns error.
- REQ-T9 (example): Auto-combat — same-room member added as combatant when leader initiates.
- REQ-T10 (example): Auto-combat — different-room member receives notification, not added as combatant.
- REQ-T11 (example): Leader logout disbands group; members' `GroupID` cleared.
- REQ-T12 (property): For any group size [1,8], `DisbandGroup` leaves zero players with that `GroupID`.
- REQ-T13 (property): For any sequence of invite/accept/kick operations, `MemberUIDs` never contains duplicates.

# Player Groups — Design Spec

**Date:** 2026-03-14

---

## Goal

Players can form persistent-session groups (max 8 members). Group members in the same room automatically enter combat together when any member initiates a fight. Members in other rooms are notified. Groups dissolve when the leader logs out or disbands.

---

## Feature 1: Data Model

### `Group` struct (new, `internal/game/session/group.go`)

```go
// Group represents a player party. Groups are session-only and not persisted to the database.
// All access is mediated through Manager methods which hold mu.
type Group struct {
    ID         string   // UUID assigned at creation
    LeaderUID  string
    MemberUIDs []string // All members including the leader; no duplicates; max 8
}
```

### `Manager` additions (`internal/game/session/manager.go`)

```go
// groups maps groupID → *Group. Protected by mu (existing sync.RWMutex).
groups map[string]*Group
```

New methods — all acquire **`mu.Lock()`** (write lock) unless noted:

| Method | Lock | Description |
|--------|------|-------------|
| `CreateGroup(leaderUID string) *Group` | Write | Allocate UUID, store group, set session.GroupID on leader |
| `DisbandGroup(groupID string)` | Write | Delete group; clear GroupID on all member sessions |
| `AddGroupMember(groupID, uid string) error` | Write | Append UID if not already present and count < 8; error if duplicate or at cap |
| `RemoveGroupMember(groupID, uid string)` | Write | Remove UID; if only member remaining is the leader after removal, the group persists (solo-leader is valid) |
| `GroupByUID(uid string) *Group` | **Read** | Scan groups for membership; nil if not in a group |
| `GroupByID(groupID string) (*Group, bool)` | **Read** | O(1) map lookup |

**Max group size:** 8 members (including leader). `AddGroupMember` returns an error if already at capacity: `"Group is full (max 8 members)."`.

### `PlayerSession` additions (`internal/game/session/manager.go`)

```go
// GroupID is the ID of the group this player belongs to.
// Empty string means not in a group. Protected by Manager.mu.
GroupID string

// PendingGroupInvite holds the groupID of a pending group invitation.
// Empty string means no pending invite. Protected by Manager.mu.
// Cleared on accept, decline, group disband, or invitee disconnect.
PendingGroupInvite string
```

---

## Feature 2: Commands

All commands follow CMD-1–7 (handler constant → BuiltinCommands → Handle func → proto message → bridge handler → gRPC dispatch → tests).

### `group` — create or list group

**No args:**
- If in a group: print group membership list.
  ```
  Group (leader: <LeaderCharName>):
    <Member1CharName>
    <Member2CharName>
    ...
  ```
- If not in a group: `"You are not in a group."`

**With `<player>` arg:**
- **Preconditions (checked in order):**
  - Caller is already in a group → `"You are already in a group. Use 'ungroup' to leave first."`
  - Target name matches caller's own CharName → `"You cannot invite yourself."`
  - Target not online → `"Player not found."`
  - Target already in a group → `"<name> is already in a group."`
  - Target already has a pending invite → `"<name> already has a pending group invitation."`
- **Postcondition:** New group created with caller as leader. `caller.GroupID` set. `target.PendingGroupInvite` set to new groupID.
- Caller sees: `"You created a group and invited <name>."`
- Target receives: `"<leaderCharName> has invited you to join their group. (accept / decline)"`

### `invite` — invite player to existing group

**Args:** `invite <player>`

- **Preconditions (checked in order):**
  - Caller not in a group → `"You are not in a group."`
  - Caller is not the group leader → `"Only the group leader can invite players."`
  - Target name matches caller's CharName → `"You cannot invite yourself."`
  - Target not online → `"Player not found."`
  - Target already in a group → `"<name> is already in a group."`
  - Target already has a pending invite → `"<name> already has a pending group invitation."`
  - Group at capacity (8 members) → `"Group is full (max 8 members)."`
- **Postcondition:** `target.PendingGroupInvite` set to caller's groupID.
- Caller sees: `"You invited <name> to the group."`
- Target receives: `"<leaderCharName> has invited you to join their group. (accept / decline)"`

### `accept` — accept pending group invite

- **Preconditions (checked in order):**
  - `sess.PendingGroupInvite == ""` → `"You have no pending group invitation."`
  - Referenced group does not exist (leader logged out or disbanded) → `"That group no longer exists."` Clear `PendingGroupInvite`.
  - Group at capacity → `"The group is full."` Clear `PendingGroupInvite`.
- **Postcondition:** `sess.GroupID` set; `sess.PendingGroupInvite` cleared; player added to `MemberUIDs`.
- Player sees: `"You joined <leaderCharName>'s group."`
- All existing group members see: `"<name> joined the group."`

### `decline` — decline pending group invite

- **Preconditions:** `sess.PendingGroupInvite == ""` → `"You have no pending group invitation."`
- **Postcondition:** `sess.PendingGroupInvite` cleared.
- Player sees: `"You declined the group invitation."`
- If group leader is online: leader sees `"<name> declined your group invitation."`
- If group leader is offline: no notification sent (no-op).

### `accept` / `decline` proto messages

These are new proto messages distinct from combat `join`/`decline`. Named `AcceptGroupRequest` and `DeclineGroupRequest` to avoid collisions.

### `ungroup` — leave group

- **Precondition:** `sess.GroupID == ""` → `"You are not in a group."`
- **Non-leader path:**
  - Player removed from group. `sess.GroupID` cleared.
  - All remaining members (including leader) see: `"<name> left the group."`
  - Player sees: `"You left the group."`
- **Leader path:**
  - Group disbanded. All members' `GroupID` cleared.
  - All members (excluding leader) see: `"The group has been disbanded by <leaderCharName>."`
  - Leader sees: `"You disbanded the group."`

### `kick` — remove a member

**Args:** `kick <player>`

- **Preconditions (checked in order):**
  - Caller not in a group → `"You are not in a group."`
  - Caller is not the leader → `"Only the group leader can kick members."`
  - Target not online or target CharName not in group → `"<name> is not in your group."`
  - Target's UID matches leader's UID (self-kick) → `"Use 'ungroup' to disband the group."`
- **Postcondition:** Target removed from `MemberUIDs`. `target.GroupID` explicitly cleared. Solo-leader group (1 member remaining) is valid and persists.
- Target sees: `"You were kicked from the group."`
- All remaining members (including leader) see: `"<name> was kicked from the group."`

---

## Feature 3: Auto-Combat

**Location:** `internal/gameserver/combat_handler.go`, in `startCombatLocked`, immediately after `engine.StartCombat` succeeds and the initiating player's status is set to `statusInCombat`.

**Precondition:** `startCombatLocked` is called with `combatMu` held. `sessionMgr.mu` is NOT held at this point (standard precondition for all combat handler code). Lock order is: `combatMu` → `engine.mu` (via `AddCombatant`). `sessionMgr.mu` is acquired only for reads (`GroupByUID`, `GetPlayer`) which use `RLock` and do not conflict.

**Algorithm:**

```
group := h.sessions.GroupByUID(sess.UID)
if group == nil:
    return  // not in a group
for each memberUID in group.MemberUIDs where memberUID != sess.UID:
    memberSess, ok := h.sessions.GetPlayer(memberUID)
    if !ok: continue  // offline
    if memberSess.Status == statusInCombat: continue  // already fighting
    if memberSess.RoomID == roomID:
        // Same room — auto-join as combatant.
        // Build combatant using exact same pattern as handleJoin in grpc_service.go:
        //   read AC from equipment, load loadout and proficiencies, copy ability mods,
        //   resistances, weaknesses, save ranks — all from memberSess fields.
        memberCbt := buildPlayerCombatant(memberSess, h)
        combat.RollInitiative([]*combat.Combatant{memberCbt}, h.dice.Src())
        if err := h.engine.AddCombatant(roomID, memberCbt); err != nil {
            // Log warning and skip this member; do not abort combat.
            h.logger.Warn("auto-join group member failed", zap.String("uid", memberUID), zap.Error(err))
            continue
        }
        memberSess.Status = statusInCombat
        // Push message to member.
        push: fmt.Sprintf("Your group entered combat! You join the fight (initiative %d).", memberCbt.Initiative)
    else:
        // Different room — notify only.
        push: fmt.Sprintf("Your group is under attack!")
```

**`buildPlayerCombatant` helper:** Extract the player-combatant construction from `handleJoin` in `grpc_service.go` into a shared unexported function `buildPlayerCombatant(sess *session.PlayerSession, h *CombatHandler) *combat.Combatant` in `combat_handler.go`. Both `handleJoin` and the auto-combat path call this function. This removes the duplication.

---

## Feature 4: Disconnect Cleanup

**Location:** `internal/gameserver/grpc_service.go`, in the existing disconnect/stream-close handler (where `sessions.RemovePlayer` is called).

**Precondition:** Player is disconnecting. `combatMu` is NOT held. `sessions.mu` is NOT held.

**Algorithm:**

```
if sess.PendingGroupInvite != "":
    // Notify leader if online.
    group, ok := sessions.GroupByID(sess.PendingGroupInvite)
    if ok:
        leaderSess, ok := sessions.GetPlayer(group.LeaderUID)
        if ok:
            push to leaderSess: "<charName> disconnected before responding to your invitation."
    sess.PendingGroupInvite = ""

if sess.GroupID != "":
    group, ok := sessions.GroupByID(sess.GroupID)
    if !ok: return  // already cleaned up
    if group.LeaderUID == sess.UID:
        // Leader disconnecting — disband.
        for each memberUID in group.MemberUIDs where memberUID != sess.UID:
            memberSess, ok := sessions.GetPlayer(memberUID)
            if ok:
                memberSess.GroupID = ""
                push: "<leaderCharName> disconnected. The group has been disbanded."
        sessions.DisbandGroup(group.ID)
    else:
        // Non-leader disconnecting — remove from group.
        sessions.RemoveGroupMember(group.ID, sess.UID)
        sess.GroupID = ""
        for each remaining member:
            push: "<charName> disconnected and left the group."
```

---

## Proto Messages

Append after the `DeclineRequest` message block in `api/proto/game/v1/game.proto`:

```protobuf
message GroupRequest { string args = 1; }
message InviteRequest { string player = 1; }
message AcceptGroupRequest {}
message DeclineGroupRequest {}
message UngroupRequest {}
message KickRequest { string player = 1; }
```

In the `ClientMessage` oneof `payload`, after `DeclineRequest decline = 74`:

```protobuf
GroupRequest group = 75;
InviteRequest invite = 76;
AcceptGroupRequest accept_group = 77;
DeclineGroupRequest decline_group = 78;
UngroupRequest ungroup = 79;
KickRequest kick = 80;
```

Note: `AcceptGroupRequest` and `DeclineGroupRequest` are distinct from the combat `JoinRequest`/`DeclineRequest` to avoid naming collisions.

---

## Testing

- REQ-T1 (example): `CreateGroup` sets LeaderUID, adds leader to MemberUIDs, stores in manager, sets leader session GroupID.
- REQ-T2 (example): `group <player>` sets `target.PendingGroupInvite` to the new groupID.
- REQ-T3 (example): `accept` adds player to MemberUIDs and sets `sess.GroupID`.
- REQ-T4 (example): `decline` clears `sess.PendingGroupInvite`; if leader online, leader notified.
- REQ-T5 (example): `ungroup` by leader disbands group; all members' `GroupID` cleared.
- REQ-T6 (example): `ungroup` by non-leader removes only that player; leader's GroupID unchanged.
- REQ-T7 (example): `kick <player>` removes target; target's `GroupID` explicitly cleared.
- REQ-T8 (example): `invite` by non-leader returns error message.
- REQ-T9 (example): Auto-combat — same-room group member added as combatant when any member initiates.
- REQ-T10 (example): Auto-combat — different-room group member receives notification, not added as combatant.
- REQ-T11 (example): Leader disconnect disbands group; all members' `GroupID` cleared; members notified.
- REQ-T12 (example): Invitee disconnect with pending invite — leader is notified; `PendingGroupInvite` cleared.
- REQ-T13 (example): `accept` when group no longer exists returns error; `PendingGroupInvite` cleared.
- REQ-T14 (example): `AddGroupMember` returns error when group is at 8-member capacity.
- REQ-T15 (property): For any group size in [1, 8], `DisbandGroup` leaves zero sessions with that `GroupID`.
- REQ-T16 (property): For any sequence of CreateGroup/AddGroupMember/RemoveGroupMember/DisbandGroup operations, `MemberUIDs` never contains duplicates.
- REQ-T17 (property): Concurrent read operations (`GroupByUID`, `GroupByID`) on Manager do not race with concurrent writes (`CreateGroup`, `DisbandGroup`) — verified with `-race` flag.

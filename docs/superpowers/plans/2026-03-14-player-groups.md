# Player Groups Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement persistent-session player groups with group/invite/accept/decline/ungroup/kick commands, auto-combat for same-room members, and disconnect cleanup.

**Architecture:** Group state is held entirely in the session Manager (no database persistence). The Manager.groups map[string]*Group is protected by the existing mu sync.RWMutex. PlayerSession gains GroupID and PendingGroupInvite string fields. All six commands follow the CMD-1–7 pipeline. Auto-combat extracts a shared buildPlayerCombatant helper from handleJoin, then auto-joins same-room group members when any member starts a fight. Disconnect cleanup runs in grpc_service.go cleanupPlayer.

**Tech Stack:** Go, gRPC/protobuf, pgregory.net/rapid (property-based testing), github.com/google/uuid v1.6.0

---

## Chunk 1: Group Data Model and Manager Methods

### Task 1: Group struct and PlayerSession fields

**Files:**
- Create: `internal/game/session/group.go`
- Modify: `internal/game/session/manager.go`

Steps:

- [ ] **Step 1: Write failing tests for Group data model**

In a new file `internal/game/session/group_test.go`, write tests verifying:
- `CreateGroup` allocates a UUID ID, sets LeaderUID, adds leader to MemberUIDs, stores in manager.groups, sets leader session's GroupID
- `AddGroupMember` appends UID if not already present and count < 8; returns error "Group is full (max 8 members)." when at cap; returns error on duplicate
- `RemoveGroupMember` removes UID; solo-leader group (1 member) persists after non-leader removal; no-op if UID not present
- `DisbandGroup` deletes from manager.groups; clears GroupID on all member sessions
- `GroupByUID` scans all groups and returns the group where uid is in MemberUIDs; returns nil if not in any group
- `GroupByID` does O(1) map lookup; returns (group, true) if found, (nil, false) if not
- Property (REQ-T14): `AddGroupMember` returns error when group has 8 members
- Property (REQ-T15): For any group size in [1,8], DisbandGroup leaves zero sessions with that GroupID
- Property (REQ-T16): For any sequence of create/add/remove/disband operations, MemberUIDs never contains duplicates
- Property (REQ-T17): Concurrent reads/writes do not race. Add this test function:

```go
func TestProperty_ConcurrentGroupAccess_NoRace(t *testing.T) {
    mgr := newTestManager(t)
    // Seed some player sessions so GroupByUID has something to scan.
    for i := 0; i < 5; i++ {
        uid := fmt.Sprintf("uid-%d", i)
        mgr.AddPlayer(session.AddPlayerOptions{UID: uid, CharName: fmt.Sprintf("Player%d", i), RoomID: "room1"})
    }
    g := mgr.CreateGroup("uid-0")

    var wg sync.WaitGroup
    const goroutines = 20
    wg.Add(goroutines)
    for i := 0; i < goroutines; i++ {
        i := i
        go func() {
            defer wg.Done()
            if i%3 == 0 {
                _ = mgr.GroupByUID("uid-0")
            } else if i%3 == 1 {
                _, _ = mgr.GroupByID(g.ID)
            } else {
                ng := mgr.CreateGroup(fmt.Sprintf("uid-%d", i%5))
                mgr.DisbandGroup(ng.ID)
            }
        }()
    }
    wg.Wait()
}
```

Run with: `go test ./internal/game/session/... -race -run TestProperty_ConcurrentGroupAccess_NoRace -v`

Run tests: `cd /home/cjohannsen/src/mud && go test ./internal/game/session/... -run TestGroup -v`
Expected: FAIL (types not defined yet)

- [ ] **Step 2: Create group.go**

```go
package session

// Group represents a player party. Groups are session-only and not persisted to the database.
// All access is mediated through Manager methods which hold mu.
type Group struct {
	ID         string   // UUID assigned at creation
	LeaderUID  string
	MemberUIDs []string // All members including the leader; no duplicates; max 8
}
```

- [ ] **Step 3: Add fields to Manager and PlayerSession in manager.go**

Add to Manager struct: `groups map[string]*Group`

Initialize in `NewManager()` alongside `players` and `roomSets`:

```go
return &Manager{
    players:  make(map[string]*PlayerSession),
    roomSets: make(map[string]map[string]bool),
    groups:   make(map[string]*Group),
}
```

Add to PlayerSession struct after the `PendingCombatJoin` field:

```go
// GroupID is the ID of the group this player belongs to.
// Empty string means not in a group. Protected by Manager.mu.
GroupID string

// PendingGroupInvite holds the groupID of a pending group invitation.
// Empty string means no pending invite. Protected by Manager.mu.
// Cleared on accept, decline, group disband, or invitee disconnect.
PendingGroupInvite string
```

- [ ] **Step 4: Add Manager methods**

Add these methods to manager.go (below the existing `AllPlayers` method):

```go
// CreateGroup creates a new group with leaderUID as the sole member and leader.
// It sets the leader's session GroupID.
//
// Precondition: leaderUID must be non-empty and correspond to an active player session.
// Postcondition: Returns the new Group with a UUID ID; manager.groups contains the group;
// leader session's GroupID is set.
func (m *Manager) CreateGroup(leaderUID string) *Group {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	g := &Group{
		ID:         id,
		LeaderUID:  leaderUID,
		MemberUIDs: []string{leaderUID},
	}
	m.groups[id] = g
	if sess, ok := m.players[leaderUID]; ok {
		sess.GroupID = id
	}
	return g
}

// DisbandGroup deletes the group and clears GroupID on all member sessions.
//
// Precondition: groupID must be non-empty.
// Postcondition: Group is removed from manager.groups; all member sessions have GroupID = "".
// No-op if groupID not found.
func (m *Manager) DisbandGroup(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return
	}
	for _, uid := range g.MemberUIDs {
		if sess, ok := m.players[uid]; ok {
			sess.GroupID = ""
		}
	}
	delete(m.groups, groupID)
}

// AddGroupMember appends uid to the group if not already present and count < 8.
// Returns an error if the group is full or uid is already a member.
//
// Precondition: groupID and uid must be non-empty.
// Postcondition: On success, uid is appended to MemberUIDs and uid session's GroupID is set.
func (m *Manager) AddGroupMember(groupID, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}
	for _, existing := range g.MemberUIDs {
		if existing == uid {
			return fmt.Errorf("already a member")
		}
	}
	if len(g.MemberUIDs) >= 8 {
		return fmt.Errorf("Group is full (max 8 members).")
	}
	g.MemberUIDs = append(g.MemberUIDs, uid)
	if sess, ok := m.players[uid]; ok {
		sess.GroupID = groupID
	}
	return nil
}

// RemoveGroupMember removes uid from the group. No-op if uid not present.
//
// Precondition: groupID and uid must be non-empty.
// Postcondition: uid is absent from MemberUIDs; uid session's GroupID is cleared.
// No-op if groupID not found or uid not in MemberUIDs.
func (m *Manager) RemoveGroupMember(groupID, uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return
	}
	filtered := g.MemberUIDs[:0]
	for _, existing := range g.MemberUIDs {
		if existing != uid {
			filtered = append(filtered, existing)
		}
	}
	g.MemberUIDs = filtered
	if sess, ok := m.players[uid]; ok {
		sess.GroupID = ""
	}
}

// GroupByUID scans groups for membership and returns the group containing uid.
// Returns nil if uid is not in any group. Uses read lock.
//
// Postcondition: Returns non-nil Group if uid is a member of any group; nil otherwise.
func (m *Manager) GroupByUID(uid string) *Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, g := range m.groups {
		for _, member := range g.MemberUIDs {
			if member == uid {
				return g
			}
		}
	}
	return nil
}

// GroupByID returns the group with the given ID. Uses read lock.
//
// Postcondition: Returns (group, true) if found; (nil, false) if not.
func (m *Manager) GroupByID(groupID string) (*Group, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.groups[groupID]
	return g, ok
}

// ForEachPlayer iterates all connected players and calls fn for each session.
// Uses read lock. fn must not call any Manager method that acquires a write lock.
//
// Postcondition: fn is called once per connected player session.
func (m *Manager) ForEachPlayer(fn func(*PlayerSession)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.players {
		fn(sess)
	}
}
```

Note: `ForEachPlayer` is required by `handleGroup` (listing members by CharName) and `handleInvite` (looking up targets by name). Add it in the same commit as the other Manager methods.

Add `"github.com/google/uuid"` to manager.go imports (it is already an indirect dependency at v1.6.0; `"fmt"` is already imported).

- [ ] **Step 5: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/session/... -v -race`
Expected: All tests pass including new group tests.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/session/group.go internal/game/session/group_test.go internal/game/session/manager.go
git commit -m "feat(session): add Group struct and Manager methods for player groups"
```

---

## Chunk 2: Proto Messages, Command Constants, and Bridge Handlers

### Task 2: Proto messages and regeneration

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerated: `internal/gameserver/gamev1/game.pb.go` (via make proto)

- [ ] **Step 1: Add proto messages**

Append after the `DeclineRequest` message block in `api/proto/game/v1/game.proto` (after `message DeclineRequest {}`):

```protobuf
// GroupRequest asks the server to create a group or show group info.
// args is optional; when non-empty it is treated as a player name to invite.
message GroupRequest { string args = 1; }

// InviteRequest asks the server to invite a player to the sender's group.
message InviteRequest { string player = 1; }

// AcceptGroupRequest asks the server to accept a pending group invitation.
message AcceptGroupRequest {}

// DeclineGroupRequest asks the server to decline a pending group invitation.
message DeclineGroupRequest {}

// UngroupRequest asks the server to leave (or disband) the sender's group.
message UngroupRequest {}

// KickRequest asks the server to remove a player from the sender's group.
message KickRequest { string player = 1; }
```

In the `ClientMessage` oneof `payload`, after `DeclineRequest decline = 74;`:

```protobuf
GroupRequest group = 75;
InviteRequest invite = 76;
AcceptGroupRequest accept_group = 77;
DeclineGroupRequest decline_group = 78;
UngroupRequest ungroup = 79;
KickRequest kick = 80;
```

- [ ] **Step 2: Regenerate proto**

Run: `cd /home/cjohannsen/src/mud && make proto`
Expected: `internal/gameserver/gamev1/game.pb.go` regenerated, no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): add group command messages (fields 75-80)"
```

### Task 3: Command constants and bridge handlers

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/group.go`
- Create: `internal/game/command/invite.go`
- Create: `internal/game/command/accept_group.go`
- Create: `internal/game/command/decline_group.go`
- Create: `internal/game/command/ungroup.go`
- Create: `internal/game/command/kick.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add handler constants and BuiltinCommands entries**

In `internal/game/command/commands.go`, add constants after `HandlerDecline = "decline"`:

```go
HandlerGroup        = "group"
HandlerInvite       = "invite"
HandlerAcceptGroup  = "acceptgroup"
HandlerDeclineGroup = "declinegroup"
HandlerUngroup      = "ungroup"
HandlerKick         = "kick"
```

Before adding the `accept` command: verify no existing `HandlerAccept` constant or `{Name: "accept", ...}` entry exists in `commands.go`. Run: `grep -n 'accept' internal/game/command/commands.go`. Expected: no match for a standalone `accept` command (combat join uses a different name).

Add to `BuiltinCommands()` after the `{Name: "decline", ...}` entry:

```go
{Name: "group", Help: "Create a group or show group info. 'group' with no args shows current group.", Category: CategoryCommunication, Handler: HandlerGroup},
{Name: "invite", Help: "Invite a player to your group.", Category: CategoryCommunication, Handler: HandlerInvite},
{Name: "accept", Help: "Accept a pending group invitation.", Category: CategoryCommunication, Handler: HandlerAcceptGroup},
{Name: "decline", Help: "Decline a pending group invitation.", Category: CategoryCommunication, Handler: HandlerDeclineGroup},
{Name: "ungroup", Help: "Leave your group. Leaders disband the group for all members.", Category: CategoryCommunication, Handler: HandlerUngroup},
{Name: "kick", Help: "Kick a player from your group (leader only).", Category: CategoryCommunication, Handler: HandlerKick},
```

Note: The user-facing command is `decline` (matching the spec), not `declinegroup`. The handler constant `HandlerDeclineGroup` is the internal identifier, while `Name: "decline"` is what players type.

- [ ] **Step 2: Create command stub files**

Create `internal/game/command/group.go`:

```go
package command

// HandleGroup handles the group command. With no args it shows current group status;
// implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleGroup(_ []string) (string, error) {
	return "", nil
}
```

Create `internal/game/command/invite.go`:

```go
package command

// HandleInvite handles the invite command.
// Implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleInvite(_ []string) (string, error) {
	return "", nil
}
```

Create `internal/game/command/accept_group.go`:

```go
package command

// HandleAcceptGroup handles the accept command for group invitations.
// Implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleAcceptGroup(_ []string) (string, error) {
	return "", nil
}
```

Create `internal/game/command/decline_group.go`:

```go
package command

// HandleDeclineGroup handles the declinegroup command for group invitations.
// Implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleDeclineGroup(_ []string) (string, error) {
	return "", nil
}
```

Create `internal/game/command/ungroup.go`:

```go
package command

// HandleUngroup handles the ungroup command.
// Implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleUngroup(_ []string) (string, error) {
	return "", nil
}
```

Create `internal/game/command/kick.go`:

```go
package command

// HandleKick handles the kick command.
// Implementation is delegated to the gameserver via gRPC.
//
// Postcondition: Always returns empty string (output is produced server-side).
func HandleKick(_ []string) (string, error) {
	return "", nil
}
```

- [ ] **Step 3: Add bridge handlers**

In `internal/frontend/handlers/bridge_handlers.go`, add these functions after `bridgeDecline` following the exact same pattern:

```go
// bridgeGroup builds a GroupRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a GroupRequest.
func bridgeGroup(bctx *bridgeContext) (bridgeResult, error) {
	args := ""
	if len(bctx.parsed.Args) > 0 {
		args = bctx.parsed.Args[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Group{Group: &gamev1.GroupRequest{Args: args}},
	}}, nil
}

// bridgeInvite builds an InviteRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an InviteRequest.
func bridgeInvite(bctx *bridgeContext) (bridgeResult, error) {
	player := ""
	if len(bctx.parsed.Args) > 0 {
		player = bctx.parsed.Args[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Invite{Invite: &gamev1.InviteRequest{Player: player}},
	}}, nil
}

// bridgeAcceptGroup builds an AcceptGroupRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an AcceptGroupRequest.
func bridgeAcceptGroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_AcceptGroup{AcceptGroup: &gamev1.AcceptGroupRequest{}},
	}}, nil
}

// bridgeDeclineGroup builds a DeclineGroupRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DeclineGroupRequest.
func bridgeDeclineGroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_DeclineGroup{DeclineGroup: &gamev1.DeclineGroupRequest{}},
	}}, nil
}

// bridgeUngroup builds an UngroupRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an UngroupRequest.
func bridgeUngroup(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Ungroup{Ungroup: &gamev1.UngroupRequest{}},
	}}, nil
}

// bridgeKick builds a KickRequest message.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a KickRequest.
func bridgeKick(bctx *bridgeContext) (bridgeResult, error) {
	player := ""
	if len(bctx.parsed.Args) > 0 {
		player = bctx.parsed.Args[0]
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Kick{Kick: &gamev1.KickRequest{Player: player}},
	}}, nil
}
```

Register all six in `bridgeHandlerMap` after `command.HandlerDecline: bridgeDecline,`:

```go
command.HandlerGroup:        bridgeGroup,
command.HandlerInvite:       bridgeInvite,
command.HandlerAcceptGroup:  bridgeAcceptGroup,
command.HandlerDeclineGroup: bridgeDeclineGroup,
command.HandlerUngroup:      bridgeUngroup,
command.HandlerKick:         bridgeKick,
```

**Note on args access:** The bridge handlers above use `bctx.parsed.Args`. Verify the field name by checking the `ParseResult` struct in `internal/game/command/` — if the field is named differently (e.g. `Arguments`), use the correct name. The `bridgeContext` struct exposes `parsed command.ParseResult`; use whatever field of `ParseResult` holds the tokenized arguments.

- [ ] **Step 4: Verify ParseResult args field name**

Run: `cd /home/cjohannsen/src/mud && grep -n "Args\|Arguments\|Tokens" internal/game/command/*.go | grep -i "parseresult\|struct\|field" | head -20`

Adjust `bctx.parsed.Args` in the bridge functions above to match the actual field name before proceeding.

- [ ] **Step 5: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/command/... ./internal/frontend/... -v`
Expected: All pass including `TestAllCommandHandlersAreWired`.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/command/commands.go \
        internal/game/command/group.go \
        internal/game/command/invite.go \
        internal/game/command/accept_group.go \
        internal/game/command/decline_group.go \
        internal/game/command/ungroup.go \
        internal/game/command/kick.go \
        internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(command,bridge): add group command constants and bridge handlers"
```

## Chunk 3: gRPC Handlers for All Six Commands

### Task 4: handleGroup, handleInvite, handleAcceptGroup, handleDeclineGroup

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_group_test.go`

---

#### TDD Steps

**Step 1 — Write failing tests first (SWENG-5, SWENG-5a).**

Create `internal/gameserver/grpc_service_group_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newGroupSvc builds a minimal GameServiceServer for group command tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr sharing state.
func newGroupSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, nil, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// addGroupTestPlayer is a helper that adds a player session with Entity initialised.
//
// Precondition: sessMgr must be non-nil; uid, charName, roomID must be non-empty.
// Postcondition: Returns a non-nil *session.PlayerSession with Entity ready for Push.
func addGroupTestPlayer(t *testing.T, sessMgr *session.Manager, uid, charName, roomID string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  uid,
		CharName:  charName,
		RoomID:    roomID,
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)
	return sess
}

// --- REQ-T1: handleGroup (no args) not in group ---

// TestHandleGroup_NoArgs_NotInGroup verifies that calling group with no args when
// not in a group returns the "not in a group" message.
//
// Precondition: Player session exists; GroupID is empty.
// Postcondition: messageEvent("You are not in a group.") returned.
func TestHandleGroup_NoArgs_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You are not in a group.", evt.GetMessage().GetContent())
}

// --- REQ-T1: handleGroup (no args) in group shows membership ---

// TestHandleGroup_NoArgs_InGroup verifies that group with no args when in a group
// prints the formatted membership list.
//
// Precondition: Player session exists; player is group leader with one other member.
// Postcondition: Membership list returned.
func TestHandleGroup_NoArgs_InGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	content := evt.GetMessage().GetContent()
	assert.Contains(t, content, "Group (leader: Alice):")
	assert.Contains(t, content, "Alice")
	assert.Contains(t, content, "Bob")
}

// --- REQ-T2: handleGroup with arg creates group and sets PendingGroupInvite ---

// TestHandleGroup_WithArg_CreatesGroupAndInvites verifies that group <player> creates
// a new group with the caller as leader and sets PendingGroupInvite on the target.
//
// Precondition: Caller not in a group; target online; neither self-invite.
// Postcondition: group created; target.PendingGroupInvite == group.ID; caller.GroupID == group.ID.
func TestHandleGroup_WithArg_CreatesGroupAndInvites(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	target := addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You created a group and invited Bob.", evt.GetMessage().GetContent())

	caller, _ := sessMgr.GetPlayer("u1")
	assert.NotEmpty(t, caller.GroupID)
	assert.NotEmpty(t, target.PendingGroupInvite)
	assert.Equal(t, caller.GroupID, target.PendingGroupInvite)
}

// TestHandleGroup_WithArg_AlreadyInGroup verifies that a caller already in a group
// cannot create another group.
//
// Precondition: Caller has a non-empty GroupID.
// Postcondition: error message returned; no new group created.
func TestHandleGroup_WithArg_AlreadyInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")
	sessMgr.CreateGroup("u1")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "You are already in a group. Use 'ungroup' to leave first.", evt.GetMessage().GetContent())
}

// TestHandleGroup_WithArg_SelfInvite verifies self-invite is rejected.
//
// Precondition: Args matches caller's own CharName.
// Postcondition: "You cannot invite yourself." returned.
func TestHandleGroup_WithArg_SelfInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "You cannot invite yourself.", evt.GetMessage().GetContent())
}

// TestHandleGroup_WithArg_TargetNotOnline verifies that inviting an offline player
// returns "Player not found."
//
// Precondition: Named player is not in any session.
// Postcondition: "Player not found." returned.
func TestHandleGroup_WithArg_TargetNotOnline(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Ghost"})
	require.NoError(t, err)
	assert.Equal(t, "Player not found.", evt.GetMessage().GetContent())
}

// TestHandleGroup_WithArg_TargetAlreadyInGroup verifies that inviting a player already
// in a group returns "<name> is already in a group."
//
// Precondition: Target has a non-empty GroupID.
// Postcondition: error message returned.
func TestHandleGroup_WithArg_TargetAlreadyInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")
	sessMgr.CreateGroup("u2")

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob is already in a group.", evt.GetMessage().GetContent())
}

// TestHandleGroup_WithArg_TargetHasPendingInvite verifies that inviting a player who
// already has a pending invite returns "<name> already has a pending group invitation."
//
// Precondition: Target has a non-empty PendingGroupInvite.
// Postcondition: error message returned.
func TestHandleGroup_WithArg_TargetHasPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	target := addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")
	target.PendingGroupInvite = "some-group-id"

	evt, err := svc.handleGroup("u1", &gamev1.GroupRequest{Args: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob already has a pending group invitation.", evt.GetMessage().GetContent())
}

// --- REQ-T8: handleInvite by non-leader returns error ---

// TestHandleInvite_NotLeader verifies that a non-leader cannot invite players.
//
// Precondition: Caller is in a group but not the leader.
// Postcondition: "Only the group leader can invite players." returned.
func TestHandleInvite_NotLeader(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")
	addGroupTestPlayer(t, sessMgr, "u3", "Carol", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))

	evt, err := svc.handleInvite("u2", &gamev1.InviteRequest{Player: "Carol"})
	require.NoError(t, err)
	assert.Equal(t, "Only the group leader can invite players.", evt.GetMessage().GetContent())
}

// TestHandleInvite_NotInGroup verifies that a player not in a group cannot invite.
//
// Precondition: Caller has empty GroupID.
// Postcondition: "You are not in a group." returned.
func TestHandleInvite_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	evt, err := svc.handleInvite("u1", &gamev1.InviteRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "You are not in a group.", evt.GetMessage().GetContent())
}

// TestHandleInvite_GroupFull verifies that invite is rejected when group is at capacity.
//
// Precondition: Group has 8 members.
// Postcondition: "Group is full (max 8 members)." returned.
func TestHandleInvite_GroupFull(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "leader", "Leader", "room_a")
	group := sessMgr.CreateGroup("leader")

	for i := 1; i <= 7; i++ {
		uid := "member" + string(rune('0'+i))
		addGroupTestPlayer(t, sessMgr, uid, "M"+string(rune('0'+i)), "room_a")
		require.NoError(t, sessMgr.AddGroupMember(group.ID, uid))
	}
	addGroupTestPlayer(t, sessMgr, "extra", "Extra", "room_a")

	evt, err := svc.handleInvite("leader", &gamev1.InviteRequest{Player: "Extra"})
	require.NoError(t, err)
	assert.Equal(t, "Group is full (max 8 members).", evt.GetMessage().GetContent())
}

// --- REQ-T3: handleAcceptGroup adds member and clears invite ---

// TestHandleAcceptGroup_Success verifies that accept adds the player to the group
// and clears PendingGroupInvite.
//
// Precondition: sess.PendingGroupInvite is non-empty; group exists and is not full.
// Postcondition: sess.GroupID == group.ID; sess.PendingGroupInvite == ""; player in MemberUIDs.
func TestHandleAcceptGroup_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")

	target, _ := sessMgr.GetPlayer("u2")
	target.PendingGroupInvite = group.ID

	evt, err := svc.handleAcceptGroup("u2", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You joined Alice's group.", evt.GetMessage().GetContent())
	assert.Equal(t, group.ID, target.GroupID)
	assert.Empty(t, target.PendingGroupInvite)
}

// TestHandleAcceptGroup_NoPendingInvite verifies that accept with no invite returns
// "You have no pending group invitation."
//
// Precondition: sess.PendingGroupInvite is empty.
// Postcondition: error message returned; no state change.
func TestHandleAcceptGroup_NoPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleAcceptGroup("u1", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "You have no pending group invitation.", evt.GetMessage().GetContent())
}

// --- REQ-T13: handleAcceptGroup when group no longer exists ---

// TestHandleAcceptGroup_GroupGone verifies that if the referenced group no longer
// exists, PendingGroupInvite is cleared and "That group no longer exists." is returned.
//
// Precondition: PendingGroupInvite refers to a groupID not in the manager.
// Postcondition: PendingGroupInvite cleared; error message returned.
func TestHandleAcceptGroup_GroupGone(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	target := addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	target.PendingGroupInvite = "nonexistent-group-id"

	evt, err := svc.handleAcceptGroup("u1", &gamev1.AcceptGroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "That group no longer exists.", evt.GetMessage().GetContent())
	assert.Empty(t, target.PendingGroupInvite)
}

// --- REQ-T4: handleDeclineGroup clears invite and optionally notifies leader ---

// TestHandleDeclineGroup_Success verifies that decline clears PendingGroupInvite
// and notifies the leader if online.
//
// Precondition: sess.PendingGroupInvite is non-empty; leader is online.
// Postcondition: PendingGroupInvite cleared; self sees decline message; leader notified.
func TestHandleDeclineGroup_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	target := addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")
	target.PendingGroupInvite = group.ID

	evt, err := svc.handleDeclineGroup("u2", &gamev1.DeclineGroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You declined the group invitation.", evt.GetMessage().GetContent())
	assert.Empty(t, target.PendingGroupInvite)
}

// TestHandleDeclineGroup_NoPendingInvite verifies that decline with no invite returns
// "You have no pending group invitation."
//
// Precondition: sess.PendingGroupInvite is empty.
// Postcondition: error message returned.
func TestHandleDeclineGroup_NoPendingInvite(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleDeclineGroup("u1", &gamev1.DeclineGroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "You have no pending group invitation.", evt.GetMessage().GetContent())
}

// --- REQ-T14: AddGroupMember returns error at capacity ---

// TestAddGroupMember_AtCapacity verifies that AddGroupMember returns an error when
// the group already has 8 members.
//
// Precondition: group.MemberUIDs has 8 entries.
// Postcondition: error returned; MemberUIDs unchanged.
func TestAddGroupMember_AtCapacity(t *testing.T) {
	_, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "leader", "Leader", "room_a")
	group := sessMgr.CreateGroup("leader")

	for i := 1; i <= 7; i++ {
		uid := "m" + string(rune('0'+i))
		addGroupTestPlayer(t, sessMgr, uid, "M"+string(rune('0'+i)), "room_a")
		require.NoError(t, sessMgr.AddGroupMember(group.ID, uid))
	}
	addGroupTestPlayer(t, sessMgr, "overflow", "Overflow", "room_a")
	err := sessMgr.AddGroupMember(group.ID, "overflow")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "full")
}

// --- REQ-T15 (property): DisbandGroup leaves zero sessions with that GroupID ---

// TestProperty_DisbandGroup_ClearsAllGroupIDs verifies that after DisbandGroup,
// no player session retains that GroupID.
//
// Precondition: group has 1–8 members set via CreateGroup + AddGroupMember.
// Postcondition: all member sessions have GroupID == "".
func TestProperty_DisbandGroup_ClearsAllGroupIDs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		_, sessMgr := newGroupSvc(t)
		size := rapid.IntRange(1, 8).Draw(rt, "size")

		addGroupTestPlayer(t, sessMgr, "leader", "Leader", "room_a")
		group := sessMgr.CreateGroup("leader")

		for i := 1; i < size; i++ {
			uid := "m" + string(rune('a'+i))
			addGroupTestPlayer(t, sessMgr, uid, "M"+string(rune('A'+i)), "room_a")
			require.NoError(rt, sessMgr.AddGroupMember(group.ID, uid))
		}

		sessMgr.DisbandGroup(group.ID)

		for _, uid := range group.MemberUIDs {
			if s, ok := sessMgr.GetPlayer(uid); ok {
				assert.Empty(rt, s.GroupID, "session %s should have empty GroupID after disband", uid)
			}
		}
		_, exists := sessMgr.GroupByID(group.ID)
		assert.False(rt, exists, "group should be absent from manager after disband")
	})
}

// --- REQ-T16 (property): MemberUIDs never contains duplicates ---

// TestProperty_GroupMemberUIDs_NoDuplicates verifies that no sequence of
// AddGroupMember calls produces duplicate entries in MemberUIDs.
//
// Precondition: repeated AddGroupMember calls with same UIDs (some duplicates).
// Postcondition: MemberUIDs has no duplicates.
func TestProperty_GroupMemberUIDs_NoDuplicates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		_, sessMgr := newGroupSvc(t)
		addGroupTestPlayer(t, sessMgr, "leader", "Leader", "room_a")
		group := sessMgr.CreateGroup("leader")
		addGroupTestPlayer(t, sessMgr, "m1", "M1", "room_a")
		addGroupTestPlayer(t, sessMgr, "m2", "M2", "room_a")

		// Attempt to add m1 twice and m2 once — second add of m1 must be a no-op or error.
		_ = sessMgr.AddGroupMember(group.ID, "m1")
		_ = sessMgr.AddGroupMember(group.ID, "m1")
		_ = sessMgr.AddGroupMember(group.ID, "m2")

		seen := make(map[string]int)
		g, ok := sessMgr.GroupByID(group.ID)
		require.True(rt, ok)
		for _, uid := range g.MemberUIDs {
			seen[uid]++
			assert.Equal(rt, 1, seen[uid], "duplicate UID %s in MemberUIDs", uid)
		}
	})
}
```

**Step 2 — Implement the handlers.**

In `internal/gameserver/grpc_service.go`, add the following four functions (before `cleanupPlayer`):

```go
// handleGroup handles the 'group' command.
// With no args: lists current group membership or reports not-in-group.
// With args: creates a new group with caller as leader and invites the named player.
//
// Precondition: uid identifies an active session; req is non-nil.
// Postcondition: On group creation, caller.GroupID and target.PendingGroupInvite are set.
func (s *GameServiceServer) handleGroup(uid string, req *gamev1.GroupRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	arg := strings.TrimSpace(req.GetArgs())

	// No-arg path: list group or report absence.
	if arg == "" {
		if sess.GroupID == "" {
			return messageEvent("You are not in a group."), nil
		}
		group, exists := s.sessions.GroupByID(sess.GroupID)
		if !exists {
			sess.GroupID = ""
			return messageEvent("You are not in a group."), nil
		}
		leaderSess, _ := s.sessions.GetPlayer(group.LeaderUID)
		leaderName := ""
		if leaderSess != nil {
			leaderName = leaderSess.CharName
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Group (leader: %s):\n", leaderName))
		for _, memberUID := range group.MemberUIDs {
			if mSess, ok2 := s.sessions.GetPlayer(memberUID); ok2 {
				sb.WriteString(fmt.Sprintf("  %s\n", mSess.CharName))
			}
		}
		return messageEvent(strings.TrimRight(sb.String(), "\n")), nil
	}

	// Arg path: create group and invite.
	if sess.GroupID != "" {
		return messageEvent("You are already in a group. Use 'ungroup' to leave first."), nil
	}
	if strings.EqualFold(arg, sess.CharName) {
		return messageEvent("You cannot invite yourself."), nil
	}

	var targetSess *session.PlayerSession
	s.sessions.ForEachPlayer(func(ps *session.PlayerSession) bool {
		if strings.EqualFold(ps.CharName, arg) {
			targetSess = ps
			return false
		}
		return true
	})
	if targetSess == nil {
		return messageEvent("Player not found."), nil
	}
	if targetSess.GroupID != "" {
		return messageEvent(fmt.Sprintf("%s is already in a group.", targetSess.CharName)), nil
	}
	if targetSess.PendingGroupInvite != "" {
		return messageEvent(fmt.Sprintf("%s already has a pending group invitation.", targetSess.CharName)), nil
	}

	group := s.sessions.CreateGroup(uid)
	targetSess.PendingGroupInvite = group.ID

	// Notify target.
	notif := messageEvent(fmt.Sprintf("%s has invited you to join their group. (accept / decline)", sess.CharName))
	if data, err := proto.Marshal(notif); err == nil {
		_ = targetSess.Entity.Push(data)
	}

	return messageEvent(fmt.Sprintf("You created a group and invited %s.", targetSess.CharName)), nil
}

// handleInvite handles the 'invite <player>' command.
//
// Precondition: uid is an active session; req is non-nil; caller must be group leader.
// Postcondition: On success, target.PendingGroupInvite is set to caller's groupID.
func (s *GameServiceServer) handleInvite(uid string, req *gamev1.InviteRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	if sess.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}
	group, exists := s.sessions.GroupByID(sess.GroupID)
	if !exists {
		sess.GroupID = ""
		return messageEvent("You are not in a group."), nil
	}
	if group.LeaderUID != uid {
		return messageEvent("Only the group leader can invite players."), nil
	}

	arg := strings.TrimSpace(req.GetPlayer())
	if strings.EqualFold(arg, sess.CharName) {
		return messageEvent("You cannot invite yourself."), nil
	}

	var targetSess *session.PlayerSession
	s.sessions.ForEachPlayer(func(ps *session.PlayerSession) bool {
		if strings.EqualFold(ps.CharName, arg) {
			targetSess = ps
			return false
		}
		return true
	})
	if targetSess == nil {
		return messageEvent("Player not found."), nil
	}
	if targetSess.GroupID != "" {
		return messageEvent(fmt.Sprintf("%s is already in a group.", targetSess.CharName)), nil
	}
	if targetSess.PendingGroupInvite != "" {
		return messageEvent(fmt.Sprintf("%s already has a pending group invitation.", targetSess.CharName)), nil
	}
	if len(group.MemberUIDs) >= 8 {
		return messageEvent("Group is full (max 8 members)."), nil
	}

	targetSess.PendingGroupInvite = group.ID

	notif := messageEvent(fmt.Sprintf("%s has invited you to join their group. (accept / decline)", sess.CharName))
	if data, err := proto.Marshal(notif); err == nil {
		_ = targetSess.Entity.Push(data)
	}

	return messageEvent(fmt.Sprintf("You invited %s to the group.", targetSess.CharName)), nil
}

// handleAcceptGroup handles the 'accept' command for group invitations.
//
// Precondition: uid identifies an active session; sess.PendingGroupInvite must be non-empty.
// Postcondition: On success, sess.GroupID set; sess.PendingGroupInvite cleared; existing members notified.
func (s *GameServiceServer) handleAcceptGroup(uid string, _ *gamev1.AcceptGroupRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	if sess.PendingGroupInvite == "" {
		return messageEvent("You have no pending group invitation."), nil
	}

	group, exists := s.sessions.GroupByID(sess.PendingGroupInvite)
	if !exists {
		sess.PendingGroupInvite = ""
		return messageEvent("That group no longer exists."), nil
	}

	if len(group.MemberUIDs) >= 8 {
		sess.PendingGroupInvite = ""
		return messageEvent("The group is full."), nil
	}

	if err := s.sessions.AddGroupMember(group.ID, uid); err != nil {
		sess.PendingGroupInvite = ""
		return messageEvent("The group is full."), nil
	}
	sess.PendingGroupInvite = ""

	// Notify existing members (before this player joined).
	joinMsg := fmt.Sprintf("%s joined the group.", sess.CharName)
	for _, memberUID := range group.MemberUIDs {
		if memberUID == uid {
			continue
		}
		if memberSess, ok2 := s.sessions.GetPlayer(memberUID); ok2 {
			notif := messageEvent(joinMsg)
			if data, err := proto.Marshal(notif); err == nil {
				_ = memberSess.Entity.Push(data)
			}
		}
	}

	leaderName := ""
	if leaderSess, ok2 := s.sessions.GetPlayer(group.LeaderUID); ok2 {
		leaderName = leaderSess.CharName
	}
	return messageEvent(fmt.Sprintf("You joined %s's group.", leaderName)), nil
}

// handleDeclineGroup handles the 'decline' command for group invitations.
//
// Precondition: uid identifies an active session; sess.PendingGroupInvite must be non-empty.
// Postcondition: sess.PendingGroupInvite cleared; if leader online, leader notified.
func (s *GameServiceServer) handleDeclineGroup(uid string, _ *gamev1.DeclineGroupRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	if sess.PendingGroupInvite == "" {
		return messageEvent("You have no pending group invitation."), nil
	}

	groupID := sess.PendingGroupInvite
	sess.PendingGroupInvite = ""

	group, exists := s.sessions.GroupByID(groupID)
	if exists {
		if leaderSess, ok2 := s.sessions.GetPlayer(group.LeaderUID); ok2 {
			notif := messageEvent(fmt.Sprintf("%s declined your group invitation.", sess.CharName))
			if data, err := proto.Marshal(notif); err == nil {
				_ = leaderSess.Entity.Push(data)
			}
		}
	}

	return messageEvent("You declined the group invitation."), nil
}
```

**Step 3 — Wire into `dispatch`.**

In the `dispatch` type switch, after the `case *gamev1.ClientMessage_Decline:` case and before `default:`:

```go
case *gamev1.ClientMessage_Group:
    return s.handleGroup(uid, p.Group)
case *gamev1.ClientMessage_Invite:
    return s.handleInvite(uid, p.Invite)
case *gamev1.ClientMessage_AcceptGroup:
    return s.handleAcceptGroup(uid, p.AcceptGroup)
case *gamev1.ClientMessage_DeclineGroup:
    return s.handleDeclineGroup(uid, p.DeclineGroup)
```

**Step 4 — Run tests.**

```
go test ./internal/gameserver/... -run TestHandleGroup -v -count=1
go test ./internal/gameserver/... -run TestHandleInvite -v -count=1
go test ./internal/gameserver/... -run TestHandleAcceptGroup -v -count=1
go test ./internal/gameserver/... -run TestHandleDeclineGroup -v -count=1
go test ./internal/gameserver/... -run TestAddGroupMember -v -count=1
go test ./internal/gameserver/... -run TestProperty_Disband -v -count=1 -rapid.checks=200
go test ./internal/gameserver/... -run TestProperty_GroupMember -v -count=1 -rapid.checks=200
```

**Step 5 — Commit.**

```
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_group_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): add handleGroup, handleInvite, handleAcceptGroup, handleDeclineGroup with TDD

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: handleUngroup, handleKick

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_group_test.go`

---

#### TDD Steps

**Step 1 — Write failing tests first.**

Append to `internal/gameserver/grpc_service_group_test.go`:

```go
// --- REQ-T5: handleUngroup by leader disbands group ---

// TestHandleUngroup_LeaderDisbands verifies that ungroup by the group leader
// disbands the group and clears all members' GroupIDs.
//
// Precondition: Caller is group leader with at least one other member.
// Postcondition: All members' GroupID == ""; group removed from manager; leader sees disband message.
func TestHandleUngroup_LeaderDisbands(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))

	evt, err := svc.handleUngroup("u1", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You disbanded the group.", evt.GetMessage().GetContent())

	leader, _ := sessMgr.GetPlayer("u1")
	member, _ := sessMgr.GetPlayer("u2")
	assert.Empty(t, leader.GroupID)
	assert.Empty(t, member.GroupID)
	_, exists := sessMgr.GroupByID(group.ID)
	assert.False(t, exists)
}

// --- REQ-T6: handleUngroup by non-leader removes only that player ---

// TestHandleUngroup_NonLeaderLeaves verifies that ungroup by a non-leader removes
// only that player and leaves the leader's GroupID intact.
//
// Precondition: Caller is a non-leader member of the group.
// Postcondition: Caller's GroupID cleared; leader's GroupID unchanged; remaining member notified.
func TestHandleUngroup_NonLeaderLeaves(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))

	evt, err := svc.handleUngroup("u2", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, "You left the group.", evt.GetMessage().GetContent())

	leader, _ := sessMgr.GetPlayer("u1")
	member, _ := sessMgr.GetPlayer("u2")
	assert.NotEmpty(t, leader.GroupID)
	assert.Empty(t, member.GroupID)
}

// TestHandleUngroup_NotInGroup verifies that ungroup when not in a group returns
// "You are not in a group."
//
// Precondition: sess.GroupID is empty.
// Postcondition: error message returned; no state change.
func TestHandleUngroup_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleUngroup("u1", &gamev1.UngroupRequest{})
	require.NoError(t, err)
	assert.Equal(t, "You are not in a group.", evt.GetMessage().GetContent())
}

// --- REQ-T7: handleKick removes target and clears target GroupID ---

// TestHandleKick_Success verifies that kick by the leader removes the target,
// clears the target's GroupID, and notifies all remaining members.
//
// Precondition: Caller is leader; target is in the group; target UID != leader UID.
// Postcondition: target.GroupID == ""; target not in MemberUIDs; remaining see kick message.
func TestHandleKick_Success(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	target := addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))

	evt, err := svc.handleKick("u1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.Empty(t, target.GroupID)
	g, exists := sessMgr.GroupByID(group.ID)
	require.True(t, exists)
	for _, uid := range g.MemberUIDs {
		assert.NotEqual(t, "u2", uid)
	}
}

// TestHandleKick_NotLeader verifies that kick by a non-leader is rejected.
//
// Precondition: Caller is in group but not the leader.
// Postcondition: "Only the group leader can kick members." returned.
func TestHandleKick_NotLeader(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")
	addGroupTestPlayer(t, sessMgr, "u3", "Carol", "room_a")

	group := sessMgr.CreateGroup("u1")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u2"))
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "u3"))

	evt, err := svc.handleKick("u2", &gamev1.KickRequest{Player: "Carol"})
	require.NoError(t, err)
	assert.Equal(t, "Only the group leader can kick members.", evt.GetMessage().GetContent())
}

// TestHandleKick_TargetNotInGroup verifies that kick of a player not in the group
// returns "<name> is not in your group."
//
// Precondition: Named player is online but not a member of the group.
// Postcondition: error message returned.
func TestHandleKick_TargetNotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "u2", "Bob", "room_a")

	sessMgr.CreateGroup("u1")

	evt, err := svc.handleKick("u1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob is not in your group.", evt.GetMessage().GetContent())
}

// TestHandleKick_SelfKick verifies that kicking yourself returns the disband hint.
//
// Precondition: Caller is leader; kick target is caller's own CharName.
// Postcondition: "Use 'ungroup' to disband the group." returned.
func TestHandleKick_SelfKick(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")
	sessMgr.CreateGroup("u1")

	evt, err := svc.handleKick("u1", &gamev1.KickRequest{Player: "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "Use 'ungroup' to disband the group.", evt.GetMessage().GetContent())
}

// TestHandleKick_NotInGroup verifies that kick when not in a group returns
// "You are not in a group."
//
// Precondition: sess.GroupID is empty.
// Postcondition: error message returned.
func TestHandleKick_NotInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "u1", "Alice", "room_a")

	evt, err := svc.handleKick("u1", &gamev1.KickRequest{Player: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "You are not in a group.", evt.GetMessage().GetContent())
}
```

**Step 2 — Implement the handlers.**

In `internal/gameserver/grpc_service.go`, add after `handleDeclineGroup`:

```go
// handleUngroup handles the 'ungroup' command.
// Non-leaders leave the group; the leader disbands it entirely.
//
// Precondition: uid identifies an active session.
// Postcondition: On non-leader exit, sess.GroupID cleared; remaining members notified.
//   On leader exit, all members' GroupID cleared; group removed from manager.
func (s *GameServiceServer) handleUngroup(uid string, _ *gamev1.UngroupRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	if sess.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}

	group, exists := s.sessions.GroupByID(sess.GroupID)
	if !exists {
		sess.GroupID = ""
		return messageEvent("You are not in a group."), nil
	}

	if group.LeaderUID == uid {
		// Leader path: disband.
		disbandMsg := fmt.Sprintf("The group has been disbanded by %s.", sess.CharName)
		for _, memberUID := range group.MemberUIDs {
			if memberUID == uid {
				continue
			}
			if memberSess, ok2 := s.sessions.GetPlayer(memberUID); ok2 {
				memberSess.GroupID = ""
				notif := messageEvent(disbandMsg)
				if data, err := proto.Marshal(notif); err == nil {
					_ = memberSess.Entity.Push(data)
				}
			}
		}
		s.sessions.DisbandGroup(group.ID)
		sess.GroupID = ""
		return messageEvent("You disbanded the group."), nil
	}

	// Non-leader path: leave.
	s.sessions.RemoveGroupMember(group.ID, uid)
	sess.GroupID = ""

	leftMsg := fmt.Sprintf("%s left the group.", sess.CharName)
	updatedGroup, ok2 := s.sessions.GroupByID(group.ID)
	if ok2 {
		for _, memberUID := range updatedGroup.MemberUIDs {
			if memberSess, ok3 := s.sessions.GetPlayer(memberUID); ok3 {
				notif := messageEvent(leftMsg)
				if data, err := proto.Marshal(notif); err == nil {
					_ = memberSess.Entity.Push(data)
				}
			}
		}
	}

	return messageEvent("You left the group."), nil
}

// handleKick handles the 'kick <player>' command.
//
// Precondition: uid identifies an active session; caller must be group leader; target must be a non-leader member.
// Postcondition: target.GroupID cleared; target removed from MemberUIDs; remaining members notified.
func (s *GameServiceServer) handleKick(uid string, req *gamev1.KickRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	if sess.GroupID == "" {
		return messageEvent("You are not in a group."), nil
	}
	group, exists := s.sessions.GroupByID(sess.GroupID)
	if !exists {
		sess.GroupID = ""
		return messageEvent("You are not in a group."), nil
	}
	if group.LeaderUID != uid {
		return messageEvent("Only the group leader can kick members."), nil
	}

	arg := strings.TrimSpace(req.GetPlayer())

	// Find target by CharName among group members.
	var targetSess *session.PlayerSession
	for _, memberUID := range group.MemberUIDs {
		if mSess, ok2 := s.sessions.GetPlayer(memberUID); ok2 {
			if strings.EqualFold(mSess.CharName, arg) {
				targetSess = mSess
				break
			}
		}
	}
	if targetSess == nil {
		return messageEvent(fmt.Sprintf("%s is not in your group.", arg)), nil
	}
	if targetSess.UID == uid {
		return messageEvent("Use 'ungroup' to disband the group."), nil
	}

	s.sessions.RemoveGroupMember(group.ID, targetSess.UID)
	targetSess.GroupID = ""

	// Notify kicked player.
	kicked := messageEvent("You were kicked from the group.")
	if data, err := proto.Marshal(kicked); err == nil {
		_ = targetSess.Entity.Push(data)
	}

	// Notify remaining members including leader.
	kickedMsg := fmt.Sprintf("%s was kicked from the group.", targetSess.CharName)
	updatedGroup, ok2 := s.sessions.GroupByID(group.ID)
	if ok2 {
		for _, memberUID := range updatedGroup.MemberUIDs {
			if memberSess, ok3 := s.sessions.GetPlayer(memberUID); ok3 {
				notif := messageEvent(kickedMsg)
				if data, err := proto.Marshal(notif); err == nil {
					_ = memberSess.Entity.Push(data)
				}
			}
		}
	}

	return messageEvent(fmt.Sprintf("You kicked %s from the group.", targetSess.CharName)), nil
}
```

**Step 3 — Wire into `dispatch`.**

In the `dispatch` type switch, after the `AcceptGroup`/`DeclineGroup` cases:

```go
case *gamev1.ClientMessage_Ungroup:
    return s.handleUngroup(uid, p.Ungroup)
case *gamev1.ClientMessage_Kick:
    return s.handleKick(uid, p.Kick)
```

**Step 4 — Run full test suite.**

```
go test ./internal/gameserver/... -run TestHandleUngroup -v -count=1
go test ./internal/gameserver/... -run TestHandleKick -v -count=1
go test ./internal/gameserver/... -count=1 -timeout=120s
```

**Step 5 — Commit.**

```
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_group_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): add handleUngroup, handleKick; wire all six group commands into dispatch

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Chunk 4: buildPlayerCombatant Helper, Auto-Combat, and Disconnect Cleanup

### Task 6: Extract buildPlayerCombatant and implement auto-combat

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go` (update `handleJoin`)
- Create: `internal/gameserver/grpc_service_auto_combat_test.go`

---

#### TDD Steps

**Step 1 — Write failing tests first.**

Create `internal/gameserver/grpc_service_auto_combat_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newAutoCombatSvc builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager for auto-combat group tests.
//
// Precondition: t must be non-nil.
// Postcondition: All returned values are non-nil and share session state.
func newAutoCombatSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestAutoCombat_SameRoom_GroupMemberJoinsAsCombarant verifies REQ-T9:
// when a player in a group initiates combat, group members in the same room are
// automatically added as combatants and their Status is set to statusInCombat.
//
// Precondition: Two players in the same room are in a group. One initiates combat.
// Postcondition: Both appear in the combat's Combatants list; member status == statusInCombat.
func TestAutoCombat_SameRoom_GroupMemberJoinsAsCombatant(t *testing.T) {
	_, sessMgr, npcMgr, combatHandler := newAutoCombatSvc(t)

	// Add NPC to room_a.
	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	// Add two players in room_a.
	sess1, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "leader", Username: "leader", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	sess2, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "member", Username: "member", CharName: "Bob",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Form a group.
	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	// Leader initiates combat.
	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(sess1, inst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	// Both players must be combatants.
	var leaderFound, memberFound bool
	for _, c := range cbt.Combatants {
		if c.ID == "leader" {
			leaderFound = true
		}
		if c.ID == "member" {
			memberFound = true
		}
	}
	assert.True(t, leaderFound, "leader should be a combatant")
	assert.True(t, memberFound, "group member in same room should be auto-added as combatant")
	assert.Equal(t, statusInCombat, sess2.Status, "member status should be statusInCombat")
}

// TestAutoCombat_DifferentRoom_MemberNotifiedOnly verifies REQ-T10:
// a group member in a different room is NOT added as a combatant but receives
// a notification pushed to their entity.
//
// Precondition: Two players in different rooms are in a group. One initiates combat.
// Postcondition: Distant member not in cbt.Combatants; member.Status != statusInCombat.
func TestAutoCombat_DifferentRoom_MemberNotifiedOnly(t *testing.T) {
	_, sessMgr, npcMgr, combatHandler := newAutoCombatSvc(t)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	sess1, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "leader", Username: "leader", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	sess2, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "member", Username: "member", CharName: "Bob",
		RoomID: "room_b", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(sess1, inst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	for _, c := range cbt.Combatants {
		assert.NotEqual(t, "member", c.ID, "distant member must not be added as combatant")
	}
	assert.NotEqual(t, statusInCombat, sess2.Status, "distant member status must not be statusInCombat")
}
```

**Step 2 — Extract `buildPlayerCombatant` in `combat_handler.go`.**

Add immediately before `startCombatLocked`:

```go
// buildPlayerCombatant constructs a *combat.Combatant for sess using equipment,
// loadout, proficiencies, ability scores, resistances, weaknesses, and save ranks
// stored on the session.
//
// Precondition: sess must be non-nil; h must be non-nil.
// Postcondition: Returns a non-nil *combat.Combatant; Initiative is NOT rolled — callers
//   must call combat.RollInitiative or set Initiative explicitly.
func buildPlayerCombatant(sess *session.PlayerSession, h *CombatHandler) *combat.Combatant {
	const dexMod = 1
	var playerAC int
	if h.invRegistry != nil {
		defStats := sess.Equipment.ComputedDefenses(h.invRegistry, dexMod)
		playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex
	} else {
		playerAC = 10 + dexMod
	}

	cbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.CurrentHP,
		CurrentHP: sess.CurrentHP,
		AC:        playerAC,
		Level:     1,
		StrMod:    2,
		DexMod:    dexMod,
	}

	h.loadoutsMu.Lock()
	if lo, ok := h.loadouts[sess.UID]; ok {
		cbt.Loadout = lo
	}
	h.loadoutsMu.Unlock()

	weaponProfRank := "untrained"
	if cbt.Loadout != nil && cbt.Loadout.MainHand != nil && cbt.Loadout.MainHand.Def != nil {
		cat := cbt.Loadout.MainHand.Def.ProficiencyCategory
		if r, ok := sess.Proficiencies[cat]; ok {
			weaponProfRank = r
		}
	}
	cbt.WeaponProficiencyRank = weaponProfRank

	if cbt.Loadout != nil && cbt.Loadout.MainHand != nil && cbt.Loadout.MainHand.Def != nil {
		cbt.WeaponDamageType = cbt.Loadout.MainHand.Def.DamageType
	}

	cbt.Resistances = sess.Resistances
	cbt.Weaknesses = sess.Weaknesses
	cbt.GritMod = combat.AbilityMod(sess.Abilities.Grit)
	cbt.QuicknessMod = combat.AbilityMod(sess.Abilities.Quickness)
	cbt.SavvyMod = combat.AbilityMod(sess.Abilities.Savvy)
	cbt.ToughnessRank = combat.DefaultSaveRank(sess.Proficiencies["toughness"])
	cbt.HustleRank = combat.DefaultSaveRank(sess.Proficiencies["hustle"])
	cbt.CoolRank = combat.DefaultSaveRank(sess.Proficiencies["cool"])

	return cbt
}
```

**Step 3 — Update `startCombatLocked` to use `buildPlayerCombatant` and add group auto-join.**

Replace the player combatant construction block (lines 1537–1592) with a single call:

```go
playerCbt := buildPlayerCombatant(sess, h)
```

Then, immediately after `sess.Status = int32(2) // gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT` (line 1635), add:

```go
// Auto-join group members who are in the same room.
if group := h.sessions.GroupByUID(sess.UID); group != nil {
    for _, memberUID := range group.MemberUIDs {
        if memberUID == sess.UID {
            continue
        }
        memberSess, ok := h.sessions.GetPlayer(memberUID)
        if !ok {
            continue
        }
        if memberSess.Status == statusInCombat {
            continue
        }
        memberCbt := buildPlayerCombatant(memberSess, h)
        combat.RollInitiative([]*combat.Combatant{memberCbt}, h.dice.Src())
        if memberSess.RoomID == sess.RoomID {
            if addErr := h.engine.AddCombatant(sess.RoomID, memberCbt); addErr != nil {
                h.logger.Warn("auto-join group member failed",
                    zap.String("uid", memberUID),
                    zap.Error(addErr),
                )
                continue
            }
            memberSess.Status = statusInCombat
            joinEvt := &gamev1.ServerEvent{
                Payload: &gamev1.ServerEvent_Message{
                    Message: &gamev1.MessageEvent{
                        Content: fmt.Sprintf("Your group entered combat! You join the fight (initiative %d).", memberCbt.Initiative),
                    },
                },
            }
            if data, marshalErr := proto.Marshal(joinEvt); marshalErr == nil {
                _ = memberSess.Entity.Push(data)
            }
        } else {
            notifyEvt := &gamev1.ServerEvent{
                Payload: &gamev1.ServerEvent_Message{
                    Message: &gamev1.MessageEvent{
                        Content: "Your group is under attack!",
                    },
                },
            }
            if data, marshalErr := proto.Marshal(notifyEvt); marshalErr == nil {
                _ = memberSess.Entity.Push(data)
            }
        }
    }
}
```

**Step 4 — Update `handleJoin` in `grpc_service.go` to call `buildPlayerCombatant`.**

Replace the entire player combatant construction block inside `handleJoin` (the block starting at `const dexMod = 1` through `playerCbt.CoolRank = ...`) with:

```go
// Build player combatant using the shared helper (avoids duplication with startCombatLocked).
playerCbt := buildPlayerCombatant(sess, s.combatH)
```

Then replace the initiative roll block that follows:

```go
// Roll initiative for just this player against existing combatants.
if s.dice != nil {
    roll := s.dice.Src().Intn(20) + 1
    playerCbt.Initiative = roll + playerCbt.DexMod
}
```

This block remains identical — it is not part of `buildPlayerCombatant` because `handleJoin` rolls initiative differently (single roll, not the batch `RollInitiative`). No change needed here.

**Step 5 — Run tests.**

```
go test ./internal/gameserver/... -run TestAutoCombat -v -count=1
go test ./internal/gameserver/... -count=1 -timeout=120s
```

**Step 6 — Commit.**

```
git add internal/gameserver/combat_handler.go \
        internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_auto_combat_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): extract buildPlayerCombatant helper; add group auto-combat on startCombatLocked

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Disconnect Cleanup

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (`cleanupPlayer` function)
- Create: `internal/gameserver/grpc_service_group_disconnect_test.go`

---

#### TDD Steps

**Step 1 — Write failing tests first.**

Create `internal/gameserver/grpc_service_group_disconnect_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupPlayer_LeaderDisconnect_DisbandGroup verifies REQ-T11:
// when the group leader's session is cleaned up, the group is disbanded and all
// remaining members' GroupID fields are cleared.
//
// Precondition: Leader and member are in a group; leader session cleaned up.
// Postcondition: member.GroupID == ""; group absent from manager.
func TestCleanupPlayer_LeaderDisconnect_DisbandGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "leader", "Alice", "room_a")
	member := addGroupTestPlayer(t, sessMgr, "member", "Bob", "room_a")

	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	// Simulate leader disconnect.
	svc.cleanupPlayer("leader", "alice")

	assert.Empty(t, member.GroupID, "member GroupID should be cleared after leader disconnects")
	_, exists := sessMgr.GroupByID(group.ID)
	assert.False(t, exists, "group should be removed from manager after leader disconnects")
}

// TestCleanupPlayer_InviteePendingInvite_LeaderNotified verifies REQ-T12:
// when a player with a pending invite disconnects, the inviting leader's session
// receives a notification and PendingGroupInvite is cleared.
//
// Precondition: Invitee has non-empty PendingGroupInvite; leader is online.
// Postcondition: invitee.PendingGroupInvite == "" after cleanup.
func TestCleanupPlayer_InviteePendingInvite_LeaderNotified(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	addGroupTestPlayer(t, sessMgr, "leader", "Alice", "room_a")
	invitee := addGroupTestPlayer(t, sessMgr, "invitee", "Bob", "room_a")

	group := sessMgr.CreateGroup("leader")
	invitee.PendingGroupInvite = group.ID

	// Simulate invitee disconnect.
	svc.cleanupPlayer("invitee", "bob")

	// PendingGroupInvite must be cleared.
	// (invitee session has been removed by RemovePlayer, so check via the snapshot we hold.)
	assert.Empty(t, invitee.PendingGroupInvite, "PendingGroupInvite must be cleared on disconnect")
}

// TestCleanupPlayer_NonLeaderDisconnect_RemainsInGroup verifies that when a non-leader
// disconnects, only that player leaves and the leader's GroupID is unchanged.
//
// Precondition: Leader and member in group; member session cleaned up.
// Postcondition: leader.GroupID unchanged; group still exists with only the leader.
func TestCleanupPlayer_NonLeaderDisconnect_RemainsInGroup(t *testing.T) {
	svc, sessMgr := newGroupSvc(t)
	leader := addGroupTestPlayer(t, sessMgr, "leader", "Alice", "room_a")
	addGroupTestPlayer(t, sessMgr, "member", "Bob", "room_a")

	group := sessMgr.CreateGroup("leader")
	require.NoError(t, sessMgr.AddGroupMember(group.ID, "member"))

	// Simulate member disconnect.
	svc.cleanupPlayer("member", "bob")

	assert.NotEmpty(t, leader.GroupID, "leader GroupID must remain after non-leader disconnects")
	g, exists := sessMgr.GroupByID(group.ID)
	require.True(t, exists, "group must still exist after non-leader disconnects")
	for _, uid := range g.MemberUIDs {
		assert.NotEqual(t, "member", uid, "disconnected member must be removed from MemberUIDs")
	}
}
```

**Step 2 — Implement the cleanup logic.**

In `grpc_service.go`, locate the `cleanupPlayer` function. **Insert the following block at the very top of the function body, before any existing code:**

```go
// Clear pending group invite on disconnect.
if sess.PendingGroupInvite != "" {
    if grp, ok := h.sessions.GroupByID(sess.PendingGroupInvite); ok {
        if leaderSess, ok := h.sessions.GetPlayer(grp.LeaderUID); ok {
            leaderSess.Push(messageEvent(fmt.Sprintf("%s disconnected before responding to your invitation.", sess.CharName)))
        }
    }
    sess.PendingGroupInvite = ""
}

// Handle group membership on disconnect.
if sess.GroupID != "" {
    if grp, ok := h.sessions.GroupByID(sess.GroupID); ok {
        if grp.LeaderUID == sess.UID {
            // Leader disconnecting — disband the group.
            for _, memberUID := range grp.MemberUIDs {
                if memberUID == sess.UID {
                    continue
                }
                if memberSess, ok := h.sessions.GetPlayer(memberUID); ok {
                    memberSess.GroupID = ""
                    memberSess.Push(messageEvent(fmt.Sprintf("%s disconnected. The group has been disbanded.", sess.CharName)))
                }
            }
            h.sessions.DisbandGroup(grp.ID)
        } else {
            // Non-leader disconnecting — remove from group and notify remaining members.
            remainingUIDs := make([]string, 0, len(grp.MemberUIDs))
            for _, uid := range grp.MemberUIDs {
                if uid != sess.UID {
                    remainingUIDs = append(remainingUIDs, uid)
                }
            }
            h.sessions.RemoveGroupMember(grp.ID, sess.UID)
            for _, uid := range remainingUIDs {
                if memberSess, ok := h.sessions.GetPlayer(uid); ok {
                    memberSess.Push(messageEvent(fmt.Sprintf("%s disconnected and left the group.", sess.CharName)))
                }
            }
        }
    }
}
```

**The rest of the existing `cleanupPlayer` body remains unchanged.**

**Step 3 — Run tests.**

```
go test ./internal/gameserver/... -run TestCleanupPlayer -v -count=1
go test ./internal/gameserver/... -count=1 -timeout=120s -race
```

The `-race` flag satisfies REQ-T17 (concurrent read/write safety on `Manager`).

**Step 4 — Commit.**

```
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_group_disconnect_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): group disconnect cleanup — disband on leader exit; notify leader on invitee disconnect

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Key implementation notes across Chunks 3 and 4

**Push pattern (used throughout):** Push a `*gamev1.ServerEvent` to a specific session by marshalling with `proto.Marshal` and calling `targetSess.Entity.Push(data)`. This is identical to the pattern used in `handleGrant`, `autoQueuePlayersLocked`, and all other places that push to a specific player outside of a broadcast.

**`ForEachPlayer` requirement:** The spec requires looking up players by CharName (not UID). The session manager currently has no `ForEachPlayer` method — this must be added to `internal/game/session/manager.go` alongside the group methods in Chunk 1/2 (the data model tasks). The signature is:

```go
// ForEachPlayer calls fn for each active player session, stopping early if fn returns false.
// Precondition: fn must be non-nil.
// Postcondition: All sessions visited unless fn returns false.
func (m *Manager) ForEachPlayer(fn func(*PlayerSession) bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    for _, sess := range m.players {
        if !fn(sess) {
            return
        }
    }
}
```

**`statusInCombat` constant:** Defined in `internal/gameserver/action_handler.go` as `const statusInCombat = int32(2)`. The auto-combat code in `combat_handler.go` uses `int32(2)` directly with a comment, consistent with existing style in `startCombatLocked` (`sess.Status = int32(2) // gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT`). The test file uses `statusInCombat` from `action_handler.go` since all files are in `package gameserver`.

**Module path:** `github.com/cory-johannsen/mud`
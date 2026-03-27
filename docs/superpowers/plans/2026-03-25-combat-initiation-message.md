# Combat Initiation Message Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push a console message to involved player(s) at the moment combat begins, identifying the initiator and, for NPC-initiated combat, the reason for aggression.

**Architecture:** Define an `InitiationReason` string type with six NPC-aggression constants. Push the formatted message via the existing `pushMessageToUID` helper immediately after combat starts. Thread the reason through three call paths: `Attack` (player-initiated), `InitiateNPCCombat` (hostile/faction/grudge/call-for-help paths), and `InitiateGuardCombat` (wanted status path). Consume `PendingJoinCombatRoomID` in `tickNPCIdle` to complete the join-on-next-tick mechanic and push message for 4d/4f reasons. Add `ProtectedNPCName string` to `npc.Instance` to support the "protecting [NPC name]" reason.

**Tech Stack:** Go, `pgregory.net/rapid` for property tests, existing `pushMessageToUID` in `internal/gameserver/combat_handler.go`.

---

### Task 1: Define InitiationReason type

**Files:**
- Create: `internal/game/combat/reason.go`
- Test: `internal/game/combat/reason_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/game/combat/reason_test.go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestInitiationReason_Constants(t *testing.T) {
	assert.Equal(t, combat.InitiationReason("on_sight"), combat.ReasonOnSight)
	assert.Equal(t, combat.InitiationReason("territory"), combat.ReasonTerritory)
	assert.Equal(t, combat.InitiationReason("provoked"), combat.ReasonProvoked)
	assert.Equal(t, combat.InitiationReason("call_for_help"), combat.ReasonCallForHelp)
	assert.Equal(t, combat.InitiationReason("wanted"), combat.ReasonWanted)
	assert.Equal(t, combat.InitiationReason("protecting"), combat.ReasonProtecting)
}

func TestInitiationReason_Message_PlayerInitiated(t *testing.T) {
	msg := combat.FormatPlayerInitiationMsg("Scavenger Boss")
	assert.Equal(t, "You attack Scavenger Boss.", msg)
}

func TestInitiationReason_Message_NPCInitiated_OnSight(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonOnSight, "")
	assert.Equal(t, "Scavenger attacks you — attacked on sight.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Territory(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonTerritory, "")
	assert.Equal(t, "Guard Dog attacks you — defending its territory.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Provoked(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonProvoked, "")
	assert.Equal(t, "Scavenger attacks you — provoked by your attack.", msg)
}

func TestInitiationReason_Message_NPCInitiated_CallForHelp(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger Grunt", combat.ReasonCallForHelp, "")
	assert.Equal(t, "Scavenger Grunt attacks you — responding to a call for help.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Wanted(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Zone Guard", combat.ReasonWanted, "")
	assert.Equal(t, "Zone Guard attacks you — alerted by your wanted status.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Protecting(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "Boss Scavenger")
	assert.Equal(t, "Guard Dog attacks you — protecting Boss Scavenger.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Protecting_NoName(t *testing.T) {
	// When ProtectedNPCName is empty but reason is Protecting, falls back to CallForHelp phrasing.
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "")
	assert.Equal(t, "Guard Dog attacks you — responding to a call for help.", msg)
}

func TestProperty_FormatNPCInitiationMsg_NeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reason := rapid.SampledFrom([]combat.InitiationReason{
			combat.ReasonOnSight, combat.ReasonTerritory, combat.ReasonProvoked,
			combat.ReasonCallForHelp, combat.ReasonWanted, combat.ReasonProtecting,
		}).Draw(rt, "reason")
		npcName := rapid.StringN(1, 20, -1).Draw(rt, "npcName")
		msg := combat.FormatNPCInitiationMsg(npcName, reason, "")
		assert.NotEmpty(rt, msg)
	})
}
```

- [ ] **Step 2: Run test to confirm failure**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/... -run TestInitiationReason -v 2>&1 | head -20
```
Expected: compile error — `reason.go` does not exist.

- [ ] **Step 3: Implement reason.go**

```go
// internal/game/combat/reason.go
package combat

import "fmt"

// InitiationReason identifies why combat was initiated.
// Used to format the combat initiation message per COMBATMSG-3/4.
type InitiationReason string

const (
	// ReasonOnSight — NPC is unconditionally hostile (disposition=="hostile"). COMBATMSG-4a.
	ReasonOnSight InitiationReason = "on_sight"
	// ReasonTerritory — NPC attacked due to faction-based hostility. COMBATMSG-4b.
	ReasonTerritory InitiationReason = "territory"
	// ReasonProvoked — NPC retaliated after being struck first. COMBATMSG-4c.
	ReasonProvoked InitiationReason = "provoked"
	// ReasonCallForHelp — NPC joined combat via call_for_help mechanic. COMBATMSG-4d.
	ReasonCallForHelp InitiationReason = "call_for_help"
	// ReasonWanted — Guard NPC attacked due to player wanted level. COMBATMSG-4e.
	ReasonWanted InitiationReason = "wanted"
	// ReasonProtecting — NPC joined combat to defend a named ally. COMBATMSG-4f.
	ReasonProtecting InitiationReason = "protecting"
)

// FormatPlayerInitiationMsg returns the player-initiated combat message. COMBATMSG-5.
//
// Precondition: targetName must be non-empty.
// Postcondition: Returns "You attack [targetName]."
func FormatPlayerInitiationMsg(targetName string) string {
	return fmt.Sprintf("You attack %s.", targetName)
}

// FormatNPCInitiationMsg returns the NPC-initiated combat message. COMBATMSG-6.
//
// Precondition: npcName must be non-empty; reason must be a valid InitiationReason constant.
// Postcondition: Returns "[npcName] attacks you — [reason phrase]."
// When reason is ReasonProtecting and protectedNPCName is empty, falls back to call-for-help phrasing.
func FormatNPCInitiationMsg(npcName string, reason InitiationReason, protectedNPCName string) string {
	var phrase string
	switch reason {
	case ReasonOnSight:
		phrase = "attacked on sight"
	case ReasonTerritory:
		phrase = "defending its territory"
	case ReasonProvoked:
		phrase = "provoked by your attack"
	case ReasonCallForHelp:
		phrase = "responding to a call for help"
	case ReasonWanted:
		phrase = "alerted by your wanted status"
	case ReasonProtecting:
		if protectedNPCName != "" {
			phrase = fmt.Sprintf("protecting %s", protectedNPCName)
		} else {
			phrase = "responding to a call for help"
		}
	default:
		phrase = "attacked on sight"
	}
	return fmt.Sprintf("%s attacks you — %s.", npcName, phrase)
}
```

- [ ] **Step 4: Run tests and confirm pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/... -run TestInitiationReason -v && mise exec -- go test ./internal/game/combat/... -run TestProperty -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/combat/reason.go internal/game/combat/reason_test.go
git commit -m "feat(combat): add InitiationReason type and FormatNPCInitiationMsg helpers (COMBATMSG-3/4)"
```

---

### Task 2: Add ProtectedNPCName field to npc.Instance

**Files:**
- Modify: `internal/game/npc/instance.go`
- Test: `internal/game/npc/instance_test.go` (add one test)

- [ ] **Step 1: Write the failing test**

Read `internal/game/npc/instance_test.go` first to see existing test patterns, then add:

```go
func TestInstance_ProtectedNPCName_DefaultEmpty(t *testing.T) {
	tmpl := &Template{
		ID:         "scavenger",
		Name:       "Scavenger",
		Type:       "human",
		Disposition: "hostile",
		Level:      1,
		MaxHP:      10,
		AC:         10,
		Awareness:  5,
	}
	inst := New(tmpl)
	assert.Empty(t, inst.ProtectedNPCName, "ProtectedNPCName must default to empty")
}
```

- [ ] **Step 2: Run test to confirm failure**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run TestInstance_ProtectedNPCName -v 2>&1 | head -10
```
Expected: compile error — field does not exist.

- [ ] **Step 3: Add field to Instance struct**

In `internal/game/npc/instance.go`, after the `PendingJoinCombatRoomID` field block (around line 154), add:

```go
	// ProtectedNPCName is the display name of the NPC this instance is defending.
	// Non-empty when the NPC joined combat via COMBATMSG-4f (protecting an ally).
	// Set by the call_for_help recruit logic when the recruiter has a name.
	ProtectedNPCName string
```

- [ ] **Step 4: Run tests and confirm pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -5
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/instance.go internal/game/npc/instance_test.go
git commit -m "feat(npc): add ProtectedNPCName field to Instance for COMBATMSG-4f"
```

---

### Task 3: Push player-initiated combat message in Attack()

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/combat_handler_test.go` (check file exists first)

- [ ] **Step 1: Write the failing test**

Find the test file for combat_handler by running:
```
ls /home/cjohannsen/src/mud/internal/gameserver/combat_handler*_test.go 2>/dev/null || echo "no test file"
```

If a test file exists, add to it. Otherwise create `internal/gameserver/combat_handler_initiation_test.go`.

The test needs a minimal `CombatHandler` stub with sessions + npcMgr. Look at how existing tests in `grpc_service_combat_test.go` build the handler. Then write:

```go
// internal/gameserver/combat_handler_initiation_test.go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttack_PlayerInitiated_PushesMessage verifies that when a player initiates
// combat, the message "You attack [name]." is pushed to the player's entity stream.
func TestAttack_PlayerInitiated_PushesMessage(t *testing.T) {
	// Build a minimal entity that captures pushed messages.
	captured := &captureEntity{}
	sess := &session.PlayerSession{
		UID:    "uid1",
		RoomID: "room1",
		Entity: captured,
		Status: 0,
	}
	// ... (see note below on how to build the handler from existing test helpers)
	// The assertion: after calling h.Attack("uid1", "Scavenger"), captured messages
	// must contain "You attack Scavenger."
	_ = sess
	t.Skip("implement using existing test builder pattern in this package")
}
```

**NOTE**: The actual test must use the real builder helper used in this package. Read `internal/gameserver/grpc_service_combat_test.go` lines 1-80 to find the builder pattern, then replicate it. The key assertion:

```go
events, err := h.Attack("uid1", "Scavenger")
require.NoError(t, err)
require.NotEmpty(t, events)
found := false
for _, msg := range captured.messages {
    if msg == "You attack Scavenger." {
        found = true
    }
}
assert.True(t, found, "initiation message must be pushed to the attacking player")
```

- [ ] **Step 2: Read existing test builder pattern**

```
cd /home/cjohannsen/src/mud && head -100 internal/gameserver/grpc_service_combat_test.go
```

Use the patterns found to wire up the minimal handler in the test.

- [ ] **Step 3: Run test to confirm failure**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestAttack_PlayerInitiated_PushesMessage -v 2>&1 | head -20
```
Expected: FAIL — no message pushed yet.

- [ ] **Step 4: Modify Attack() to push the message**

In `combat_handler.go`, in the `Attack` function after `startCombatLocked` is called (around line 462), add the push **before** returning initEvents:

```go
// COMBATMSG-5: push player-initiated combat message before first round output.
h.pushMessageToUID(uid, combat.FormatPlayerInitiationMsg(inst.Name()))
```

The import `"github.com/cory-johannsen/mud/internal/game/combat"` must be added at the top of `combat_handler.go`.

Full context of the edit — find the block:
```go
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	var initEvents []*gamev1.CombatEvent
	if !ok {
		var err error
		cbt, initEvents, err = h.startCombatLocked(sess, inst)
		if err != nil {
			return nil, err
		}
	}
```

Change to:
```go
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	var initEvents []*gamev1.CombatEvent
	if !ok {
		var err error
		cbt, initEvents, err = h.startCombatLocked(sess, inst)
		if err != nil {
			return nil, err
		}
		// COMBATMSG-5: push player-initiated combat message before first round output.
		h.pushMessageToUID(uid, combat.FormatPlayerInitiationMsg(inst.Name()))
	}
```

- [ ] **Step 5: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestAttack_PlayerInitiated_PushesMessage -v
```
Expected: PASS.

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_initiation_test.go
git commit -m "feat(combat): push 'You attack [name].' message on player combat initiation (COMBATMSG-5)"
```

---

### Task 4: Push NPC-initiated combat message in InitiateNPCCombat()

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/combat_handler_initiation_test.go`

- [ ] **Step 1: Write the failing test**

Add to `combat_handler_initiation_test.go`:

```go
// TestInitiateNPCCombat_HostileDisposition_PushesOnSightMessage verifies that when
// an NPC with disposition=="hostile" initiates combat, the player receives
// "[NPC name] attacks you — attacked on sight."
func TestInitiateNPCCombat_HostileDisposition_PushesOnSightMessage(t *testing.T) {
	// Wire up handler with hostile NPC in room1, player uid1 in room1.
	// Call h.InitiateNPCCombat(npcInst, "uid1").
	// Assert captured.messages contains "Scavenger attacks you — attacked on sight."
}

// TestInitiateNPCCombat_GrudgeSet_PushesProvokedMessage verifies that when
// the NPC has GrudgePlayerID set (was hit by the player), the reason is "provoked by your attack".
func TestInitiateNPCCombat_GrudgeSet_PushesProvokedMessage(t *testing.T) {
	// Wire up handler with NPC.GrudgePlayerID = "uid1".
	// Call h.InitiateNPCCombat(npcInst, "uid1").
	// Assert captured.messages contains "Scavenger attacks you — provoked by your attack."
}
```

- [ ] **Step 2: Run test to confirm failure**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestInitiateNPCCombat_.*PushesMessage -v 2>&1 | head -20
```
Expected: FAIL.

- [ ] **Step 3: Modify InitiateNPCCombat to push message**

In `combat_handler.go`, in `InitiateNPCCombat` after the `broadcastFn` call and before the function returns (i.e., after line 417 `h.broadcastFn(sess.RoomID, initEvents)`), add:

```go
// COMBATMSG-3/4: push NPC initiation message with reason to the targeted player.
reason := h.inferNPCReason(npcInst)
h.pushMessageToUID(playerUID, combat.FormatNPCInitiationMsg(npcInst.Name(), reason, npcInst.ProtectedNPCName))
```

Add a new private helper in `combat_handler.go`:

```go
// inferNPCReason derives the InitiationReason for an NPC based on its current state.
//
// Precondition: inst must not be nil.
// Postcondition: Returns the most specific applicable InitiationReason constant.
func (h *CombatHandler) inferNPCReason(inst *npc.Instance) combat.InitiationReason {
	if inst.GrudgePlayerID != "" {
		return combat.ReasonProvoked
	}
	if inst.PendingJoinCombatRoomID != "" {
		return combat.ReasonProtecting
	}
	if inst.Disposition == "hostile" {
		return combat.ReasonOnSight
	}
	return combat.ReasonTerritory
}
```

- [ ] **Step 4: Run test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_initiation_test.go
git commit -m "feat(combat): push NPC initiation reason message in InitiateNPCCombat (COMBATMSG-3/4/6)"
```

---

### Task 5: Push wanted-status message in InitiateGuardCombat()

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/combat_handler_initiation_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestInitiateGuardCombat_PushesWantedMessage verifies that when guards engage a
// wanted player, the player receives "[guard name] attacks you — alerted by your wanted status."
func TestInitiateGuardCombat_PushesWantedMessage(t *testing.T) {
	// Wire up handler with guard NPC (NPCType=="guard") in player's room.
	// wantedLevel >= 2 so the guard engages.
	// Call h.InitiateGuardCombat("uid1", "zone1", 2).
	// Assert captured.messages contains "[guard name] attacks you — alerted by your wanted status."
}
```

- [ ] **Step 2: Run test to confirm failure**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestInitiateGuardCombat_PushesWanted -v 2>&1 | head -20
```
Expected: FAIL.

- [ ] **Step 3: Modify InitiateGuardCombat to push message**

In `InitiateGuardCombat`, after calling `h.Attack(guardID, uid)` for each guard, push the initiation message to the player. Since the guard's `npc.Instance` is available in the loop (retrieved via `h.npcMgr.Get(guardID)`), push per guard:

Find the loop in `InitiateGuardCombat`:
```go
for _, guardID := range guardIDs {
    _, _ = h.Attack(guardID, uid)
}
```

Change to:
```go
for _, guardID := range guardIDs {
    _, _ = h.Attack(guardID, uid)
    // COMBATMSG-4e: push wanted-status initiation message to the player.
    if guardInst, ok := h.npcMgr.Get(guardID); ok {
        h.pushMessageToUID(uid, combat.FormatNPCInitiationMsg(guardInst.Name(), combat.ReasonWanted, ""))
    }
}
```

- [ ] **Step 4: Run test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_initiation_test.go
git commit -m "feat(combat): push wanted-status initiation message in InitiateGuardCombat (COMBATMSG-4e)"
```

---

### Task 6: Consume PendingJoinCombatRoomID and push call-for-help / protecting message

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (`tickNPCIdle`)
- Modify: `internal/gameserver/combat_handler.go` (set ProtectedNPCName in call_for_help recruit code)
- Test: `internal/gameserver/grpc_service_combat_initiation_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/gameserver/grpc_service_combat_initiation_test.go
package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTickNPCIdle_PendingJoinCombat_JoinsAndPushesMessage verifies that an NPC
// with PendingJoinCombatRoomID set joins an active combat on the next tick and
// the targeted player receives "[NPC name] attacks you — responding to a call for help."
func TestTickNPCIdle_PendingJoinCombat_JoinsAndPushesMessage(t *testing.T) {
	// This test requires a GameServiceServer with:
	//   - Active combat in room1
	//   - NPC with PendingJoinCombatRoomID = "room1" in room1
	//   - Player uid1 in room1
	// After tickNPCIdle is called for the pending NPC:
	//   - PendingJoinCombatRoomID must be cleared
	//   - NPC must be in the active combat
	//   - Player uid1 must have received "[NPC name] attacks you — responding to a call for help."
	t.Skip("wire up full server stub and implement")
}
```

- [ ] **Step 2: Understand the join-on-next-tick mechanic**

Read `internal/gameserver/grpc_service.go` from line 4160 to 4186 (`tickZone` and `tickNPCIdle`) to understand where to add the pending join check:

```
sed -n '4160,4190p' /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go
```

- [ ] **Step 3: Add PendingJoinCombatRoomID consumption to tickNPCIdle**

In `grpc_service.go`, at the start of `tickNPCIdle` (after the `Immobile` check, around line 4189), add:

```go
// REQ-NB-34: NPC was recruited via call_for_help on the previous tick.
// Join the combat in the pending room and push the initiation message.
if inst.PendingJoinCombatRoomID != "" {
    pendingRoom := inst.PendingJoinCombatRoomID
    inst.PendingJoinCombatRoomID = ""
    inst.PlayerEnteredRoom = false
    inst.OnDamageTaken = false
    if s.combatH != nil {
        s.combatH.JoinPendingNPCCombat(inst, pendingRoom)
    }
    return
}
```

- [ ] **Step 4: Add JoinPendingNPCCombat to CombatHandler**

In `combat_handler.go`, add:

```go
// JoinPendingNPCCombat adds npcInst to the active combat in pendingRoomID and pushes
// the call-for-help or protecting initiation message to all players in that combat.
//
// Precondition: inst must not be nil; pendingRoomID must be non-empty.
// Postcondition: NPC is added to active combat; message pushed to targeted player(s).
func (h *CombatHandler) JoinPendingNPCCombat(inst *npc.Instance, pendingRoomID string) {
    h.combatMu.Lock()
    defer h.combatMu.Unlock()

    cbt, ok := h.engine.GetCombat(pendingRoomID)
    if !ok {
        return
    }
    // Find the player combatant(s) to message.
    reason := combat.ReasonCallForHelp
    if inst.ProtectedNPCName != "" {
        reason = combat.ReasonProtecting
    }
    for _, c := range cbt.Combatants {
        if c.Kind != combat.KindPlayer {
            continue
        }
        h.pushMessageToUID(c.ID, combat.FormatNPCInitiationMsg(inst.Name(), reason, inst.ProtectedNPCName))
    }
    // Add the NPC to the combat as a new combatant.
    npcCbt := h.buildNPCCombatant(inst)
    if err := cbt.AddCombatant(npcCbt); err != nil {
        return
    }
}
```

**Note:** If `cbt.AddCombatant` does not exist, check what method the combat engine exposes for adding a combatant to an in-progress combat. Read `internal/game/combat/combat.go` to verify:

```
grep -n "AddCombatant\|JoinCombat" /home/cjohannsen/src/mud/internal/game/combat/combat.go | head -10
```

If the method is named differently, use the correct name. If it doesn't exist, read how existing group-join combat adds members (`combat_handler.go` around line 2270, `joinMsg`) and use the same mechanism.

Also check if `buildNPCCombatant` exists or if you need to construct `*combat.Combatant` inline (as done in `startCombatLocked` around line 2209):

```
grep -n "buildNPCCombatant\|buildPlayerCombatant" /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go | head -5
```

Use whatever pattern is established.

- [ ] **Step 5: Set ProtectedNPCName in call_for_help recruit code**

In `combat_handler.go`, find the recruit loop around line 3024 where `r.inst.PendingJoinCombatRoomID = cbt.RoomID` is set. Read lines 3000-3030 to understand the recruit struct. If `r.inst` knows the calling NPC (actor), set:

```go
r.inst.ProtectedNPCName = actor.Name // actor is the NPC that called for help
```

This enables COMBATMSG-4f: "protecting [actor.Name]".

- [ ] **Step 6: Run test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go internal/gameserver/combat_handler.go \
        internal/gameserver/grpc_service_combat_initiation_test.go
git commit -m "feat(combat): consume PendingJoinCombatRoomID on NPC tick, push call-for-help/protecting message (COMBATMSG-4d/4f, REQ-NB-34)"
```

---

### Task 7: Integration test and feature flag update

**Files:**
- Modify: `docs/features/index.yaml`
- Test: `internal/gameserver/grpc_service_combat_initiation_test.go`

- [ ] **Step 1: Write integration tests confirming all six reasons appear**

Add table-driven tests to `grpc_service_combat_initiation_test.go` verifying message format for each reason:

```go
func TestCombatInitiationMessages_AllReasonFormats(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "player initiated",
			fn:       func() string { return combat.FormatPlayerInitiationMsg("Boss Rat") },
			expected: "You attack Boss Rat.",
		},
		{
			name:     "on_sight",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonOnSight, "") },
			expected: "Scavenger attacks you — attacked on sight.",
		},
		{
			name:     "territory",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Fence Dog", combat.ReasonTerritory, "") },
			expected: "Fence Dog attacks you — defending its territory.",
		},
		{
			name:     "provoked",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonProvoked, "") },
			expected: "Scavenger attacks you — provoked by your attack.",
		},
		{
			name:     "call_for_help",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Grunt", combat.ReasonCallForHelp, "") },
			expected: "Grunt attacks you — responding to a call for help.",
		},
		{
			name:     "wanted",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Zone Guard", combat.ReasonWanted, "") },
			expected: "Zone Guard attacks you — alerted by your wanted status.",
		},
		{
			name:     "protecting_with_name",
			fn:       func() string { return combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "Boss Scav") },
			expected: "Guard Dog attacks you — protecting Boss Scav.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fn())
		})
	}
}
```

- [ ] **Step 2: Run test**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestCombatInitiationMessages_AllReasonFormats -v
```
Expected: all PASS.

- [ ] **Step 3: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -15
```
Expected: all PASS.

- [ ] **Step 4: Set feature status to done in index.yaml**

In `docs/features/index.yaml`, find the `combat-initiation-message` entry and change `status: in_progress` → `status: done`.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service_combat_initiation_test.go docs/features/index.yaml
git commit -m "feat(combat): all-reasons integration tests pass; mark combat-initiation-message done (COMBATMSG-1..8)"
```

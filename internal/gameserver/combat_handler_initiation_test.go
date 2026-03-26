package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/protobuf/proto"
)

// TestAttack_PlayerInitiated_PushesMessage verifies that when a player initiates
// combat, the message "You attack [name]." is pushed to the player's entity stream.
// COMBATMSG-5.
func TestAttack_PlayerInitiated_PushesMessage(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-init-msg"
	spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-init-1", roomID)

	// Replace the auto-created entity with a buffered one we can read from.
	entity := session.NewBridgeEntity("player-init-1", 32)
	sess.Entity = entity

	_, err := h.Attack("player-init-1", "Goblin")
	if err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}
	h.cancelTimer(roomID)

	// Drain entity events and look for the initiation message.
	const want = "You attack Goblin."
	deadline := time.After(500 * time.Millisecond)
	found := false
outer:
	for {
		select {
		case data := <-entity.Events():
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			msg := evt.GetMessage()
			if msg != nil && msg.GetContent() == want {
				found = true
				break outer
			}
		case <-deadline:
			break outer
		}
	}
	if !found {
		t.Errorf("expected message %q pushed to player entity; none found within 500ms", want)
	}
}

// drainForMessage reads from entity.Events() until content is found or deadline expires.
func drainForMessage(t *testing.T, entity *session.BridgeEntity, want string) bool {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case data := <-entity.Events():
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			if msg := evt.GetMessage(); msg != nil && msg.GetContent() == want {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// TestInitiateNPCCombat_HostileDisposition_PushesOnSightMessage verifies that an NPC
// with disposition=="hostile" initiates combat with "attacked on sight" reason.
// COMBATMSG-4a.
func TestInitiateNPCCombat_HostileDisposition_PushesOnSightMessage(t *testing.T) {
	const roomID = "room-npc-onsight"
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	sess := addTestPlayer(t, h.sessions, "uid-onsight", roomID)
	entity := session.NewBridgeEntity("uid-onsight", 32)
	sess.Entity = entity

	tmpl := &npc.Template{
		ID:          "goblin-hostile",
		Name:        "Goblin",
		Level:       1,
		MaxHP:       20,
		AC:          13,
		Awareness:   2,
		Disposition: "hostile",
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	h.InitiateNPCCombat(inst, "uid-onsight")
	h.cancelTimer(roomID)

	const want = "Goblin attacks you — attacked on sight."
	if !drainForMessage(t, entity, want) {
		t.Errorf("expected message %q pushed to player entity; none found within 500ms", want)
	}
}

// TestInitiateGuardCombat_PushesWantedMessage verifies that when guards engage a
// wanted player, the player receives "[guard name] attacks you — alerted by your wanted status."
// COMBATMSG-4e.
func TestInitiateGuardCombat_PushesWantedMessage(t *testing.T) {
	const roomID = "room-guard-wanted"
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	sess := addTestPlayer(t, h.sessions, "uid-wanted", roomID)
	entity := session.NewBridgeEntity("uid-wanted", 32)
	sess.Entity = entity

	tmpl := &npc.Template{
		ID:        "guard-wanted",
		Name:      "City Guard",
		Level:     1,
		MaxHP:     20,
		AC:        13,
		Awareness: 2,
		NPCType:   "guard",
		Guard:     &npc.GuardConfig{WantedThreshold: 2},
	}
	_, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	h.InitiateGuardCombat("uid-wanted", "zone1", 2)
	h.cancelTimer(roomID)

	const want = "City Guard attacks you — alerted by your wanted status."
	if !drainForMessage(t, entity, want) {
		t.Errorf("expected message %q pushed to player entity; none found within 500ms", want)
	}
}

// TestInitiateNPCCombat_GrudgeSet_PushesProvokedMessage verifies that an NPC with
// GrudgePlayerID set produces "provoked by your attack" reason. COMBATMSG-4c.
func TestInitiateNPCCombat_GrudgeSet_PushesProvokedMessage(t *testing.T) {
	const roomID = "room-npc-grudge"
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	sess := addTestPlayer(t, h.sessions, "uid-grudge", roomID)
	entity := session.NewBridgeEntity("uid-grudge", 32)
	sess.Entity = entity

	tmpl := &npc.Template{
		ID:        "goblin-grudge",
		Name:      "Goblin",
		Level:     1,
		MaxHP:     20,
		AC:        13,
		Awareness: 2,
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	inst.GrudgePlayerID = "uid-grudge"

	h.InitiateNPCCombat(inst, "uid-grudge")
	h.cancelTimer(roomID)

	const want = "Goblin attacks you — provoked by your attack."
	if !drainForMessage(t, entity, want) {
		t.Errorf("expected message %q pushed to player entity; none found within 500ms", want)
	}
}

// TestJoinPendingNPCCombat_PushesCallForHelpMessage verifies that when an NPC with
// PendingJoinCombatRoomID set (and no ProtectedNPCName) joins combat, the player
// receives the call-for-help message. COMBATMSG-4d, REQ-NB-34.
func TestJoinPendingNPCCombat_PushesCallForHelpMessage(t *testing.T) {
	const roomID = "room-cfh"
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	// Establish active combat in roomID with player and first NPC.
	sess := addTestPlayer(t, h.sessions, "uid-cfh", roomID)
	entity := session.NewBridgeEntity("uid-cfh", 32)
	sess.Entity = entity
	spawnTestNPC(t, h.npcMgr, roomID)
	_, err := h.Attack("uid-cfh", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Spawn a second NPC (the recruit) — different template ID to avoid conflict.
	recruitTmpl := &npc.Template{
		ID:        "goblin-recruit",
		Name:      "Goblin Recruit",
		Level:     1,
		MaxHP:     20,
		AC:        13,
		Awareness: 2,
	}
	recruit, err := h.npcMgr.Spawn(recruitTmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn recruit: %v", err)
	}

	// Simulate the pending join: no ProtectedNPCName means call-for-help reason.
	h.JoinPendingNPCCombat(recruit, roomID)

	const want = "Goblin Recruit attacks you — responding to a call for help."
	if !drainForMessage(t, entity, want) {
		t.Errorf("expected message %q; none found within 500ms", want)
	}
}

// TestJoinPendingNPCCombat_PushesProtectingMessage verifies that when an NPC with
// ProtectedNPCName set joins combat, the player receives the protecting message.
// COMBATMSG-4f, REQ-NB-34.
func TestJoinPendingNPCCombat_PushesProtectingMessage(t *testing.T) {
	const roomID = "room-protecting"
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	// Establish active combat in roomID with player and first NPC.
	sess := addTestPlayer(t, h.sessions, "uid-protecting", roomID)
	entity := session.NewBridgeEntity("uid-protecting", 32)
	sess.Entity = entity
	spawnTestNPC(t, h.npcMgr, roomID)
	_, err := h.Attack("uid-protecting", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Spawn a second NPC (the protect-recruit).
	recruitTmpl := &npc.Template{
		ID:        "goblin-protector",
		Name:      "Goblin Protector",
		Level:     1,
		MaxHP:     20,
		AC:        13,
		Awareness: 2,
	}
	recruit, err := h.npcMgr.Spawn(recruitTmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn recruit: %v", err)
	}
	recruit.ProtectedNPCName = "Goblin"

	h.JoinPendingNPCCombat(recruit, roomID)

	const want = "Goblin Protector attacks you — protecting Goblin."
	if !drainForMessage(t, entity, want) {
		t.Errorf("expected message %q; none found within 500ms", want)
	}
}

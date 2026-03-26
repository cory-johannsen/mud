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

package gameserver

import (
	"testing"
	"time"

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

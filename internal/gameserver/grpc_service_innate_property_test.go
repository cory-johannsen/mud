package gameserver

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// REQ-INN8 (property): For N uses, exactly N activations consumed before exhausted.
// handleUse signature: (uid, abilityID string) (*gamev1.ServerEvent, error) — no stream, returns event.
func TestPropertyInnateUse_ExactlyNActivationsConsumed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		// Generate N in [1, 5]
		n := rapid.IntRange(1, 5).Draw(rt, "maxUses")

		uid := fmt.Sprintf("prop-innate-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
		})
		if err != nil {
			rt.Skip()
		}

		repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
			"acid_spit": {MaxUses: n, UsesRemaining: n},
		}}
		svc.SetInnateTechRepo(repo)
		sess.InnateTechs = map[string]*session.InnateSlot{
			"acid_spit": {MaxUses: n, UsesRemaining: n},
		}

		// Activate N times — all should return an activation message containing "acid_spit"
		for i := 0; i < n; i++ {
			evt, err := svc.handleUse(uid, "acid_spit")
			if err != nil {
				rt.Fatalf("activation %d failed: %v", i, err)
			}
			if evt == nil {
				rt.Fatalf("activation %d: nil event returned", i)
			}
			msg := evt.GetMessage().GetContent()
			if !strings.Contains(msg, "acid_spit") {
				rt.Fatalf("activation %d: expected activation message containing 'acid_spit', got: %q", i, msg)
			}
		}

		// (N+1)th activation must return "No uses remaining"
		evt, err := svc.handleUse(uid, "acid_spit")
		if err != nil {
			rt.Fatalf("(N+1)th call failed: %v", err)
		}
		if evt == nil {
			rt.Fatalf("(N+1)th call: nil event returned")
		}
		exhaustedMsg := evt.GetMessage().GetContent()
		if !strings.Contains(exhaustedMsg, "No uses of acid_spit remaining") {
			rt.Fatalf("expected 'No uses of acid_spit remaining' on (N+1)th call, got: %q", exhaustedMsg)
		}

		// UsesRemaining must never be below 0
		if sess.InnateTechs["acid_spit"].UsesRemaining < 0 {
			rt.Errorf("UsesRemaining went below zero: %d", sess.InnateTechs["acid_spit"].UsesRemaining)
		}
	})
}

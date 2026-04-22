package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// Register allocates a buffered channel and Deliver routes a matching
// response to that channel without blocking.
func TestReactionPromptHub_RegisterDeliver_ReceivesResponse(t *testing.T) {
	h := newReactionPromptHub()
	ch := h.Register("p1")

	resp := &gamev1.ReactionResponse{PromptId: "p1", Chosen: "chrome_reflex"}
	h.Deliver(resp)

	select {
	case got := <-ch:
		assert.Equal(t, "p1", got.GetPromptId())
		assert.Equal(t, "chrome_reflex", got.GetChosen())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected response on channel")
	}
}

// Deliver to an unregistered prompt_id is a silent no-op.
func TestReactionPromptHub_DeliverUnknown_DropsSilently(t *testing.T) {
	h := newReactionPromptHub()
	// Must not panic and must not block.
	h.Deliver(&gamev1.ReactionResponse{PromptId: "unknown", Chosen: ""})
}

// Unregister removes the channel so subsequent Deliver calls are dropped.
func TestReactionPromptHub_Unregister_DropsAfterRemoval(t *testing.T) {
	h := newReactionPromptHub()
	ch := h.Register("p2")
	h.Unregister("p2")
	h.Deliver(&gamev1.ReactionResponse{PromptId: "p2", Chosen: "x"})

	select {
	case <-ch:
		t.Fatal("expected no delivery after Unregister")
	case <-time.After(50 * time.Millisecond):
	}
}

// Deliver on a full channel drops the duplicate without blocking.
func TestReactionPromptHub_DeliverWhenFull_DropsWithoutBlocking(t *testing.T) {
	h := newReactionPromptHub()
	h.Register("p3")

	h.Deliver(&gamev1.ReactionResponse{PromptId: "p3", Chosen: "a"})
	done := make(chan struct{})
	go func() {
		// If Deliver blocks on a full channel, this goroutine never exits.
		h.Deliver(&gamev1.ReactionResponse{PromptId: "p3", Chosen: "b"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Deliver blocked when channel was full")
	}
}

// Deliver with a nil response is a silent no-op.
func TestReactionPromptHub_DeliverNil_NoPanic(t *testing.T) {
	h := newReactionPromptHub()
	h.Deliver(nil)
}

// newPromptID returns unique, non-empty identifiers.
func TestNewPromptID_UniqueNonEmpty(t *testing.T) {
	seen := make(map[string]struct{}, 64)
	for i := 0; i < 64; i++ {
		id := newPromptID()
		assert.NotEmpty(t, id)
		_, dup := seen[id]
		assert.False(t, dup, "prompt_id collision: %s", id)
		seen[id] = struct{}{}
	}
}

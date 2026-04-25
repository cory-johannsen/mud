package gameserver

import (
	"sync"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// reactionPromptHub routes ReactionResponse messages from the per-player
// gRPC ClientMessage stream back to the goroutine blocked inside
// buildReactionCallback.
//
// Lifecycle:
//   1. buildReactionCallback generates a prompt_id, calls Register to obtain a
//      response channel, sends a ReactionPromptEvent to the player's stream.
//   2. buildReactionCallback blocks on the channel (or ctx.Done).
//   3. When the player's gRPC dispatch receives a ClientMessage_ReactionResponse
//      the server hands it to Deliver which performs a non-blocking send on the
//      registered channel.
//   4. buildReactionCallback wakes, handles the response, and calls Unregister.
//
// This is the first request/response pattern of its kind in this codebase —
// unlike FeatureChoicePrompt (which is sent as a sentinel-wrapped MessageEvent
// and answered by a ChooseFeatRequest handled synchronously by the gRPC
// server's command dispatcher), the reaction prompt must BLOCK the combat
// resolver goroutine until a response arrives. The hub exists to bridge that
// synchronous-looking API into the asynchronous gRPC Recv loop.
//
// Concurrency: safe for concurrent Register/Deliver/Unregister calls.
type reactionPromptHub struct {
	// channels maps prompt_id to its response channel. sync.Map is used
	// because the hot path is concurrent Register/Deliver from different
	// goroutines with no strong locking invariants to maintain.
	channels sync.Map // map[string]chan *gamev1.ReactionResponse
}

// newReactionPromptHub returns a ready-to-use hub.
func newReactionPromptHub() *reactionPromptHub {
	return &reactionPromptHub{}
}

// Register allocates a buffered response channel for promptID and stores it
// in the hub. The returned channel has capacity 1 so Deliver never blocks.
//
// Precondition: promptID must be non-empty and unique for the lifetime of the
// pending prompt.
// Postcondition: The caller MUST eventually call Unregister(promptID) to free
// the channel entry.
func (h *reactionPromptHub) Register(promptID string) chan *gamev1.ReactionResponse {
	ch := make(chan *gamev1.ReactionResponse, 1)
	h.channels.Store(promptID, ch)
	return ch
}

// Unregister removes the channel for promptID. Safe to call multiple times
// and safe to call on an unknown promptID.
func (h *reactionPromptHub) Unregister(promptID string) {
	h.channels.Delete(promptID)
}

// Deliver routes a ReactionResponse to the channel registered for
// resp.PromptId. Non-blocking: if the channel is full or the promptID is
// unknown (e.g. deadline already elapsed), the response is dropped silently.
//
// Precondition: resp may be nil (nil is a no-op).
// Postcondition: at most one value is placed on the registered channel.
func (h *reactionPromptHub) Deliver(resp *gamev1.ReactionResponse) {
	if resp == nil {
		return
	}
	v, ok := h.channels.Load(resp.GetPromptId())
	if !ok {
		return
	}
	ch, ok := v.(chan *gamev1.ReactionResponse)
	if !ok {
		return
	}
	select {
	case ch <- resp:
	default:
		// Channel full — a response already arrived for this prompt_id.
		// Drop the duplicate to avoid blocking the dispatch goroutine.
	}
}

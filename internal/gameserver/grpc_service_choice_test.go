package gameserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// moveMsg builds a ClientMessage with a MoveRequest, simulating a bare number like "3"
// being parsed as a direction.
func moveMsg(dir string) *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Move{
			Move: &gamev1.MoveRequest{Direction: dir},
		},
	}
}

// statusMsg builds a ClientMessage with a StatusRequest, simulating the web client's
// ws.onopen status poll.
func statusMsg() *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Status{
			Status: &gamev1.StatusRequest{},
		},
	}
}

// mapMsg builds a ClientMessage with a MapRequest, simulating the web client's
// ws.onopen map poll.
func mapMsg() *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Map{
			Map: &gamev1.MapRequest{},
		},
	}
}

// choiceOptions returns a simple FeatureChoices used across multiple tests.
func choiceOptions() *ruleset.FeatureChoices {
	return &ruleset.FeatureChoices{
		Prompt: "Choose your path:",
		Key:    "feat_path",
		Options: []string{
			"Option Alpha",
			"Option Beta",
			"Option Gamma",
		},
	}
}

// REQ-BUG69-1: promptFeatureChoice MUST send a sentinel-encoded choice prompt.
func TestPromptFeatureChoice_SendsSentinelEncodedPrompt(t *testing.T) {
	svc := &GameServiceServer{}
	choices := choiceOptions()

	stream := &fakeSessionStream{
		// Pre-queue a valid choice so the call returns.
		recv: []*gamev1.ClientMessage{sayMsg("2")},
	}

	chosen, err := svc.promptFeatureChoice(stream, "feat_path", choices, false)
	require.NoError(t, err)
	assert.Equal(t, "Option Beta", chosen)

	// First sent event MUST be the sentinel-encoded choice prompt.
	require.NotEmpty(t, stream.sent, "expected at least one sent event")
	promptEvt := stream.sent[0]
	content := promptEvt.GetMessage().GetContent()
	const sentinel = "\x00choice\x00"
	require.True(t, strings.HasPrefix(content, sentinel),
		"first sent event must begin with %q, got %q", sentinel, content)

	// Unmarshal and verify the payload.
	jsonStr := content[len(sentinel):]
	var payload struct {
		FeatureID string   `json:"featureId"`
		Prompt    string   `json:"prompt"`
		Options   []string `json:"options"`
	}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &payload))
	assert.Equal(t, "feat_path", payload.FeatureID)
	assert.Equal(t, "Choose your path:", payload.Prompt)
	assert.Equal(t, []string{"Option Alpha", "Option Beta", "Option Gamma"}, payload.Options)
}

// REQ-BUG69-2: promptFeatureChoice MUST skip non-Say/non-Move messages and wait for
// a valid choice (simulating StatusRequest and MapRequest arriving from the web client
// before the user responds).
func TestPromptFeatureChoice_SkipsNonChoiceMessages(t *testing.T) {
	svc := &GameServiceServer{}
	choices := choiceOptions()

	// StatusRequest and MapRequest arrive first (web client ws.onopen), then valid choice.
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{
			statusMsg(),
			mapMsg(),
			sayMsg("3"),
		},
	}

	chosen, err := svc.promptFeatureChoice(stream, "feat_path", choices, false)
	require.NoError(t, err)
	assert.Equal(t, "Option Gamma", chosen,
		"must skip non-choice messages and resolve to the Say '3' message")
}

// REQ-BUG69-3: promptFeatureChoice MUST skip a non-choice MoveRequest (e.g. "north")
// and still process a subsequent valid numeric choice via MoveRequest direction "1".
func TestPromptFeatureChoice_SkipsNonNumericMove_ThenAcceptsNumericMove(t *testing.T) {
	svc := &GameServiceServer{}
	choices := choiceOptions()

	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{
			moveMsg("north"), // non-numeric — skip
			moveMsg("1"),    // valid numeric choice
		},
	}

	chosen, err := svc.promptFeatureChoice(stream, "feat_path", choices, false)
	require.NoError(t, err)
	assert.Equal(t, "Option Alpha", chosen)
}

// REQ-BUG69-4: promptFeatureChoice in headless mode MUST return the first option without
// reading the stream.
func TestPromptFeatureChoice_Headless_ReturnsFirstOption(t *testing.T) {
	svc := &GameServiceServer{}
	choices := choiceOptions()

	stream := &fakeSessionStream{} // no messages queued — would EOF if Recv called

	chosen, err := svc.promptFeatureChoice(stream, "feat_path", choices, true)
	require.NoError(t, err)
	assert.Equal(t, "Option Alpha", chosen)
	assert.Empty(t, stream.sent, "headless mode must not send any events")
}

// REQ-BUG69-5: promptFeatureChoice MUST send "Invalid selection" and return ("", nil)
// when all attempts are exhausted without a valid numeric choice.
func TestPromptFeatureChoice_ExhaustsAllAttempts_SendsInvalidSelection(t *testing.T) {
	svc := &GameServiceServer{}
	choices := choiceOptions()

	// Queue 20 status messages to exhaust the retry loop.
	recv := make([]*gamev1.ClientMessage, 20)
	for i := range recv {
		recv[i] = statusMsg()
	}
	stream := &fakeSessionStream{recv: recv}

	chosen, err := svc.promptFeatureChoice(stream, "feat_path", choices, false)
	require.NoError(t, err)
	assert.Equal(t, "", chosen, "exhausted loop must return empty string")

	// At least one sent message must mention "Invalid selection".
	found := false
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil && strings.Contains(msg.Content, "Invalid selection") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'Invalid selection' in sent messages after exhausting retries")
}

// TestProperty_PromptFeatureChoice_ValidIndexAlwaysReturnsCorrectOption is a property
// test verifying that any in-range 1-based index sent as a SayRequest always resolves
// to the corresponding option, regardless of option count or position.
//
// Precondition: choices has at least 1 option and index is in [1, len(options)].
// Postcondition: Returns options[index-1] and nil error.
func TestProperty_PromptFeatureChoice_ValidIndexAlwaysReturnsCorrectOption(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate between 1 and 8 options.
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		options := make([]string, n)
		for i := range options {
			options[i] = fmt.Sprintf("opt_%d", i)
		}
		choices := &ruleset.FeatureChoices{
			Prompt:  "Pick one:",
			Key:     "feat_prop",
			Options: options,
		}

		// Pick a valid 1-based index.
		idx := rapid.IntRange(1, n).Draw(rt, "idx")

		svc := &GameServiceServer{}
		stream := &fakeSessionStream{
			recv: []*gamev1.ClientMessage{sayMsg(fmt.Sprintf("%d", idx))},
		}

		chosen, err := svc.promptFeatureChoice(stream, "feat_prop", choices, false)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if chosen != options[idx-1] {
			rt.Fatalf("expected options[%d]=%q, got %q", idx-1, options[idx-1], chosen)
		}
	})
}

// TestProperty_PromptFeatureChoice_NonChoiceMessagesBeforeValidPick verifies that any
// number of non-Say/non-Move messages before a valid pick are skipped correctly.
//
// Precondition: A valid SayRequest choice arrives after 0-10 StatusRequests.
// Postcondition: Returns the correct option regardless of how many skipped messages precede it.
func TestProperty_PromptFeatureChoice_NonChoiceMessagesBeforeValidPick(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		skipCount := rapid.IntRange(0, 10).Draw(rt, "skipCount")
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		options := make([]string, n)
		for i := range options {
			options[i] = fmt.Sprintf("choice_%d", i)
		}
		choices := &ruleset.FeatureChoices{
			Prompt:  "Choose:",
			Key:     "feat_skip",
			Options: options,
		}

		idx := rapid.IntRange(1, n).Draw(rt, "idx")
		recv := make([]*gamev1.ClientMessage, skipCount+1)
		for i := 0; i < skipCount; i++ {
			recv[i] = statusMsg()
		}
		recv[skipCount] = sayMsg(fmt.Sprintf("%d", idx))

		svc := &GameServiceServer{}
		stream := &fakeSessionStream{recv: recv}

		chosen, err := svc.promptFeatureChoice(stream, "feat_skip", choices, false)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if chosen != options[idx-1] {
			rt.Fatalf("expected options[%d]=%q, got %q", idx-1, options[idx-1], chosen)
		}
	})
}

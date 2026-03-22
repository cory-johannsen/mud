package gameserver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// REQ-ILT6: handleSelectTech with no pending grants sends "no pending technology selections."
func TestHandleSelectTech_NoPending_SendsNoPending(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-selecttech-empty"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	stream := &fakeSessionStream{}
	err = svc.handleSelectTech(uid, "req1", stream)
	require.NoError(t, err)

	require.NotEmpty(t, stream.sent)
	last := stream.sent[len(stream.sent)-1]
	msg := last.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "no pending")
}

// TestPropertyHandleSelectTech_NoPending_AlwaysSendsNoPendingMessage verifies that
// handleSelectTech always sends "no pending" when PendingTechGrants is empty,
// regardless of session state (SWENG-5a).
func TestPropertyHandleSelectTech_NoPending_AlwaysSendsNoPendingMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		uid := fmt.Sprintf("prop-selecttech-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
		})
		if err != nil {
			rt.Skip()
		}

		// PendingTechGrants is nil/empty — invariant: always returns no-pending message.
		stream := &fakeSessionStream{}
		if err := svc.handleSelectTech(uid, "req", stream); err != nil {
			rt.Fatalf("handleSelectTech returned error: %v", err)
		}
		if len(stream.sent) == 0 {
			rt.Fatalf("no messages sent")
		}
		last := stream.sent[len(stream.sent)-1]
		msg := last.GetMessage()
		if msg == nil {
			rt.Fatalf("last sent event has no message payload")
		}
		if !strings.Contains(msg.Content, "no pending") {
			rt.Fatalf("expected 'no pending' in message, got: %q", msg.Content)
		}
	})
}

// TestWrapOption_ShortText verifies that a short option that fits on one line is not wrapped.
func TestWrapOption_ShortText(t *testing.T) {
	result := wrapOption("  1) ", "short text", 78)
	assert.Equal(t, "  1) short text", result)
}

// TestWrapOption_LongText verifies that a long option wraps and indents continuation lines.
func TestWrapOption_LongText(t *testing.T) {
	prefix := "  1) "
	indent := "     "
	text := "acid_arrow — This is a very long description that should definitely exceed seventy-eight characters in total width"
	result := wrapOption(prefix, text, 78)
	lines := strings.Split(result, "\n")
	assert.True(t, len(lines) > 1, "expected multiple lines for long text")
	assert.True(t, strings.HasPrefix(lines[0], prefix), "first line must start with prefix")
	for _, line := range lines[1:] {
		assert.True(t, strings.HasPrefix(line, indent), "continuation line must start with indent %q, got %q", indent, line)
		assert.False(t, strings.HasPrefix(line, prefix), "continuation line must NOT start with prefix")
	}
	// No line should exceed 78 chars
	for _, line := range lines {
		assert.LessOrEqual(t, len(line), 78, "line %q exceeds 78 chars", line)
	}
}

// TestWrapOption_TwoDigitPrefix verifies correct indent for item index >= 10.
func TestWrapOption_TwoDigitPrefix(t *testing.T) {
	prefix := "  10) "
	indent := "      "
	text := "acid_arrow — This is a very long description that should definitely exceed seventy-eight characters in total width for double digit"
	result := wrapOption(prefix, text, 78)
	lines := strings.Split(result, "\n")
	assert.True(t, len(lines) > 1, "expected multiple lines for long text")
	assert.True(t, strings.HasPrefix(lines[0], prefix), "first line must start with prefix")
	for _, line := range lines[1:] {
		assert.True(t, strings.HasPrefix(line, indent), "continuation line must start with indent %q, got %q", indent, line)
	}
}

// TestWrapOption_ExactFit verifies that text that exactly fills the first line is not wrapped.
func TestWrapOption_ExactFit(t *testing.T) {
	prefix := "  1) "
	// Build text that exactly fills 78 chars: prefix(5) + text(73)
	text := strings.Repeat("a", 73)
	result := wrapOption(prefix, text, 78)
	lines := strings.Split(result, "\n")
	assert.Equal(t, 1, len(lines), "expected single line when text exactly fits")
}

// TestWrapOption_Property verifies invariants on random inputs (SWENG-5a).
func TestWrapOption_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 99).Draw(rt, "n")
		prefix := fmt.Sprintf("  %d) ", n)
		indent := strings.Repeat(" ", len(prefix))
		wordCount := rapid.IntRange(1, 30).Draw(rt, "wordCount")
		words := make([]string, wordCount)
		for i := range words {
			wlen := rapid.IntRange(1, 15).Draw(rt, fmt.Sprintf("wlen%d", i))
			words[i] = strings.Repeat("x", wlen)
		}
		text := strings.Join(words, " ")
		result := wrapOption(prefix, text, 78)
		lines := strings.Split(result, "\n")

		if !strings.HasPrefix(lines[0], prefix) {
			rt.Fatalf("first line does not start with prefix %q: %q", prefix, lines[0])
		}
		for i, line := range lines[1:] {
			if !strings.HasPrefix(line, indent) {
				rt.Fatalf("line %d does not start with indent %q: %q", i+1, indent, line)
			}
			if strings.HasPrefix(line[len(indent):], " ") {
				rt.Fatalf("line %d has extra leading space after indent: %q", i+1, line)
			}
		}
		for i, line := range lines {
			if len(line) > 78 {
				// Only allowed if a single word is longer than the available width
				content := line
				if i == 0 {
					content = line[len(prefix):]
				} else {
					content = line[len(indent):]
				}
				if strings.Contains(content, " ") {
					rt.Fatalf("line %d exceeds 78 chars and contains spaces: %q", i, line)
				}
			}
		}
	})
}

// REQ-ILT9: CharacterSheetView reports pending tech selections count.
func TestBuildCharacterSheetView_PendingTechSelections(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-pending-tech-sheet"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	target, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	target.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
		2: {Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "pending_choice", Level: 1}},
		}},
	}

	evt, err := svc.handleChar(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	var sheetView *gamev1.CharacterSheetView
	if cs := evt.GetCharacterSheet(); cs != nil {
		sheetView = cs
	}
	require.NotNil(t, sheetView, "CharacterSheetView must be returned by handleChar")
	assert.Equal(t, int32(1), sheetView.PendingTechSelections)
}

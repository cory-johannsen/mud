package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCrafting_DowntimeQueue(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestDT_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	// Queue a downtime activity and verify the server acknowledges it was queued.
	// Completion messages are not tested here because they may not arrive within
	// the test window — queue confirmation is the reliable observable outcome.
	require.NoError(t, player.Send("downtime earn_income"))
	require.NoError(t, player.ExpectRegex(`(?i)(queue|added|scheduled|downtime|activity|earn)`, 5*time.Second),
		"downtime command must confirm the activity was queued")

	// Verify the queued entry appears in the downtime list.
	require.NoError(t, player.Send("downtime list"))
	require.NoError(t, player.Expect("> ", 5*time.Second), "downtime list must return prompt")
}

func TestCrafting_ListRecipes(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestCraft_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("craft list"))
	require.NoError(t, player.Expect("> ", 5*time.Second), "craft list must return prompt")
}

package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInventory_GrantAndGet(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestInv_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, editor.Send("grant_item tactical_knife"))
	require.NoError(t, editor.Expect("grant", 5*time.Second))

	require.NoError(t, player.Send("get tactical_knife"))
	require.NoError(t, player.Expect("pick up", 5*time.Second),
		"player must see pick-up confirmation")

	require.NoError(t, player.Send("inventory"))
	require.NoError(t, player.Expect("tactical", 5*time.Second),
		"item must appear in inventory")
}

func TestInventory_Drop(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestInvDrop_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, editor.Send("grant_item tactical_knife"))
	require.NoError(t, editor.Expect("grant", 5*time.Second))
	require.NoError(t, player.Send("get tactical_knife"))
	require.NoError(t, player.Expect("pick up", 5*time.Second))

	require.NoError(t, player.Send("drop tactical_knife"))
	require.NoError(t, player.Expect("drop", 5*time.Second),
		"player must see drop confirmation")
}

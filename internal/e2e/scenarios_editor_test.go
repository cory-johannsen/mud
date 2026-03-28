package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEditor_SpawnNPC(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestEdNPC_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, editor.Send("spawnnpc feral_dog"))
	require.NoError(t, editor.Expect("spawn", 5*time.Second),
		"editor must confirm NPC spawn")

	require.NoError(t, player.Send("look"))
	require.NoError(t, player.Expect("feral", 5*time.Second),
		"player must see spawned NPC in room description")
}

func TestEditor_GrantItem(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestEdItem_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, editor.Send("grant_item tactical_knife"))
	require.NoError(t, editor.Expect("grant", 5*time.Second),
		"editor must confirm item grant")

	require.NoError(t, player.Send("get tactical_knife"))
	require.NoError(t, player.Expect("pick up", 5*time.Second),
		"player must be able to pick up granted item")
}

func TestEditor_GrantMoney(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestEdMoney_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, editor.Send("grant_money 100"))
	require.NoError(t, editor.Expect("grant", 5*time.Second),
		"editor must confirm money grant")

	require.NoError(t, player.Send("balance"))
	require.NoError(t, player.Expect("100", 5*time.Second),
		"player balance must reflect granted money")
}

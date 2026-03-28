package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHotbar_AssignAndActivate(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestHotbar_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, player.Send("hotbar 1 look"))
	require.NoError(t, player.Expect("Slot 1 set", 5*time.Second),
		"hotbar set must confirm assignment")

	require.NoError(t, player.Send("1"))
	require.NoError(t, player.Expect("Exits:", 5*time.Second),
		"slot activation must execute stored command")
}

func TestHotbar_ClearSlot(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestHotbarClear_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, player.Send("hotbar 2 status"))
	require.NoError(t, player.Expect("Slot 2 set", 5*time.Second))

	require.NoError(t, player.Send("hotbar clear 2"))
	require.NoError(t, player.Expect("Slot 2 cleared", 5*time.Second), "hotbar clear must confirm")

	require.NoError(t, player.Send("2"))
	require.NoError(t, player.Expect("unassigned", 5*time.Second),
		"cleared slot activation must say unassigned")
}

func TestHotbar_ShowList(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestHotbarShow_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("hotbar"))
	require.NoError(t, player.Expect("[10]", 5*time.Second),
		"hotbar show must list all 10 slots")
}

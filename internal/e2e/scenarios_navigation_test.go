package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNavigation_Look(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNavLook_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("look"))
	require.NoError(t, player.Expect("Exits:", 5*time.Second), "look must show exits")
}

func TestNavigation_Move(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNavMove_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("exits"))
	require.NoError(t, player.Expect("Exits:", 5*time.Second))

	require.NoError(t, player.Send("north"))
	err := player.Expect("Exits:", 5*time.Second)
	if err != nil {
		require.NoError(t, player.Expect("no exit", 2*time.Second),
			"server must respond to invalid direction with 'no exit' message")
	}
}

func TestNavigation_InvalidDirection(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNavInvalid_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("xyzzy_not_a_direction"))
	require.NoError(t, player.Expect("> ", 5*time.Second),
		"server must return prompt after unknown direction (no crash)")
}

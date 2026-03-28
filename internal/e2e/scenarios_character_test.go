package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCharacter_CreateAndSelect(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestCreate_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := NewClientForTest(t)
	loginAs(t, player, "claude_player")
	selectCharacter(t, player, charName)

	require.NoError(t, player.Expect("Exits:", 5*time.Second),
		"should see room exits after character select")
}

func TestCharacter_ReloginRestoresState(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestRelogin_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	p1 := NewClientForTest(t)
	loginAs(t, p1, "claude_player")
	selectCharacter(t, p1, charName)
	require.NoError(t, p1.Expect("Exits:", 5*time.Second))
	require.NoError(t, p1.Send("quit"))
	require.NoError(t, p1.Expect("Goodbye", 5*time.Second))

	p2 := NewClientForTest(t)
	loginAs(t, p2, "claude_player")
	selectCharacter(t, p2, charName)
	require.NoError(t, p2.Expect("Exits:", 5*time.Second),
		"re-login should restore character in the same room")
}

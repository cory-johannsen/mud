package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCombat_InitiateAndReceiveOutput(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestCombat_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	require.NoError(t, editor.Send("spawnnpc gang_member"))
	require.NoError(t, editor.Expect("spawn", 5*time.Second))

	player := enterGame(t, charName)
	require.NoError(t, player.Send("attack gang_member"))
	require.NoError(t, player.ExpectRegex(`(attack|damage|round|combat)`, 10*time.Second),
		"attack must trigger combat output")
}

func TestCombat_SubmitAction(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestCombatAction_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	require.NoError(t, editor.Send("spawnnpc gang_member"))
	require.NoError(t, editor.Expect("spawn", 5*time.Second))

	player := enterGame(t, charName)
	require.NoError(t, player.Send("attack gang_member"))
	require.NoError(t, player.ExpectRegex(`(attack|damage|round|combat)`, 10*time.Second))

	require.NoError(t, player.Send("pass"))
	require.NoError(t, player.Expect("> ", 5*time.Second), "pass action must return prompt")
}

func TestCombat_LootDefeatedEnemy(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestCombatLoot_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	// Spawn and immediately kill an NPC via editor command so the player can loot it.
	require.NoError(t, editor.Send("spawnnpc gang_member"))
	require.NoError(t, editor.Expect("spawn", 5*time.Second))
	require.NoError(t, editor.Send("killnpc gang_member"))
	require.NoError(t, editor.Expect("> ", 5*time.Second))

	player := enterGame(t, charName)
	require.NoError(t, player.Send("loot gang_member"))
	require.NoError(t, player.ExpectRegex(`(?i)(loot|item|nothing|empty|found|credit)`, 10*time.Second),
		"loot command must produce a response about items or indicate nothing to loot")
}

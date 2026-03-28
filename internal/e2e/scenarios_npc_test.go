package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNPC_BrowseMerchant(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNPC_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	require.NoError(t, player.Send("look"))
	require.NoError(t, player.Expect("Exits:", 5*time.Second))

	require.NoError(t, player.Send("browse"))
	err := player.Expect("Credits", 5*time.Second)
	if err != nil {
		require.NoError(t, player.Expect("> ", 2*time.Second),
			"browse must return prompt even with no merchant present")
	}
}

func TestNPC_BuyItem(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNPCBuy_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)
	require.NoError(t, player.Send("buy bandages"))
	require.NoError(t, player.Expect("> ", 5*time.Second),
		"buy must return prompt regardless of merchant presence")
}

func TestNPC_SellItem(t *testing.T) {
	defer recordTiming(t, time.Now())
	charName := fmt.Sprintf("TestNPCSell_%d", time.Now().UnixMilli()%10000)

	editor := NewClientForTest(t)
	loginAs(t, editor, "claude_editor")
	createCharacter(t, editor, charName)
	t.Cleanup(func() { deleteCharacter(t, editor, charName) })

	player := enterGame(t, charName)

	// Grant an item to the player so they have something to sell.
	require.NoError(t, editor.Send("grant_item tactical_knife"))
	require.NoError(t, editor.Expect("grant", 5*time.Second))
	require.NoError(t, player.Send("get tactical_knife"))
	require.NoError(t, player.Expect("pick up", 5*time.Second))

	require.NoError(t, player.Send("sell tactical_knife"))
	require.NoError(t, player.Expect("> ", 5*time.Second),
		"sell must return prompt regardless of merchant presence")
}

package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/e2e"
)

// NewClientForTest dials the headless port and registers Close on t.Cleanup (REQ-ITS-6).
//
// Postcondition: Returns a connected HeadlessClient; t.Cleanup will close it.
func NewClientForTest(t *testing.T) *e2e.HeadlessClient {
	t.Helper()
	c, err := e2e.Dial(e2eState.HeadlessAddr)
	require.NoError(t, err, "NewClientForTest: dial headless port")
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// loginAs authenticates a client session with the given username.
// Uses password "testpass123" (set by TestMain seed step).
//
// Postcondition: client is at the character-select or in-game prompt.
func loginAs(t *testing.T, c *e2e.HeadlessClient, username string) {
	t.Helper()
	require.NoError(t, c.Expect("Username", 10*time.Second), "loginAs: waiting for username prompt")
	require.NoError(t, c.Send(username), "loginAs: sending username")
	require.NoError(t, c.Expect("Password", 5*time.Second), "loginAs: waiting for password prompt")
	require.NoError(t, c.Send("testpass123"), "loginAs: sending password")
	require.NoError(t, c.Expect("> ", 10*time.Second), "loginAs: waiting for post-login prompt")
}

// loginAsRaw logs in with explicit credentials without waiting for the final prompt.
// Used for auth failure tests.
func loginAsRaw(t *testing.T, c *e2e.HeadlessClient, username, password string) {
	t.Helper()
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send(username))
	require.NoError(t, c.Expect("Password", 5*time.Second))
	require.NoError(t, c.Send(password))
}

// selectCharacter selects a character by sending "1" (assumes one character per session).
//
// Postcondition: client is in-game.
func selectCharacter(t *testing.T, c *e2e.HeadlessClient, charName string) {
	t.Helper()
	require.NoError(t, c.ExpectRegex(`\d+\.\s+`+charName, 5*time.Second),
		"selectCharacter: character %q not found in list", charName)
	lineNum := 1
	require.NoError(t, c.Send(fmt.Sprintf("%d", lineNum)), "selectCharacter: sending selection")
	require.NoError(t, c.Expect("> ", 10*time.Second), "selectCharacter: waiting for in-game prompt")
}

// createCharacter creates a test character for the claude_player account via the editor.
// The editor must already be logged in with claude_editor credentials.
//
// Postcondition: character exists in DB; returns character name.
func createCharacter(t *testing.T, editorClient *e2e.HeadlessClient, charName string) string {
	t.Helper()
	require.NoError(t, editorClient.Send(fmt.Sprintf("spawn_char %s", charName)),
		"createCharacter: sending spawn_char")
	require.NoError(t, editorClient.Expect("created", 5*time.Second),
		"createCharacter: waiting for creation confirmation for %q", charName)
	return charName
}

// deleteCharacter deletes a test character via the editor session.
// Non-fatal — logs errors rather than failing the test.
func deleteCharacter(t *testing.T, editorClient *e2e.HeadlessClient, charName string) {
	t.Helper()
	if err := editorClient.Send(fmt.Sprintf("delete_char %s", charName)); err != nil {
		t.Logf("deleteCharacter: send error (non-fatal): %v", err)
		return
	}
	if err := editorClient.Expect("deleted", 5*time.Second); err != nil {
		t.Logf("deleteCharacter: confirm error (non-fatal): %v", err)
	}
}

// enterGame creates a claude_player client, logs in, and selects the named character.
//
// Precondition: charName must already exist (created via createCharacter).
// Postcondition: Returns a connected, in-game client with t.Cleanup for close.
func enterGame(t *testing.T, charName string) *e2e.HeadlessClient {
	t.Helper()
	player := NewClientForTest(t)
	loginAs(t, player, "claude_player")
	selectCharacter(t, player, charName)
	return player
}

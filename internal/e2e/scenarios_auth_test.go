package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuth_ValidLogin(t *testing.T) {
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send("claude_player"))
	require.NoError(t, c.Expect("Password", 5*time.Second))
	require.NoError(t, c.Send("testpass123"))
	require.NoError(t, c.Expect("> ", 10*time.Second), "should reach post-login prompt")
}

func TestAuth_InvalidPassword(t *testing.T) {
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	loginAsRaw(t, c, "claude_player", "wrongpassword")
	require.NoError(t, c.Expect("Invalid password", 5*time.Second),
		"server must reject invalid password")
}

func TestAuth_UnknownAccount(t *testing.T) {
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	loginAsRaw(t, c, "nonexistent_user_xyz", "anypassword")
	require.NoError(t, c.Expect("Account not found", 5*time.Second),
		"server must report unknown account")
}

func TestAuth_EmptyUsername(t *testing.T) {
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send(""))
	require.NoError(t, c.Expect("empty", 5*time.Second),
		"server must reject empty username")
}

func TestAuth_QuitBeforeLogin(t *testing.T) {
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send("quit"))
	require.NoError(t, c.Expect("Goodbye", 5*time.Second),
		"server must send goodbye on quit")
}

func TestAuth_AccountLockout(t *testing.T) {
	defer recordTiming(t, time.Now())
	// Attempt 5 consecutive failed logins and expect a lockout or rate-limit response.
	// Each attempt uses a fresh connection because the server closes the connection
	// on an invalid-password failure.
	const failAttempts = 5
	for i := 0; i < failAttempts; i++ {
		c := NewClientForTest(t)
		loginAsRaw(t, c, "claude_player", "wrongpassword")
		// Consume the rejection line; ignore timeout — we may be locked out already.
		_ = c.Expect("Invalid password", 5*time.Second)
	}
	// On the next attempt the server must respond with a lockout or rate-limit message.
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send("claude_player"))
	require.NoError(t, c.Expect("Password", 5*time.Second))
	require.NoError(t, c.Send("wrongpassword"))
	require.NoError(t, c.ExpectRegex(`(?i)(lock|rate.limit|too many|blocked|denied)`, 5*time.Second),
		"server must respond with lockout or rate-limit message after repeated failures")
}

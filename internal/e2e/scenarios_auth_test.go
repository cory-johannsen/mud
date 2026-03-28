package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuth_ValidLogin(t *testing.T) {
	t.Parallel()
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send("claude_player"))
	require.NoError(t, c.Expect("Password", 5*time.Second))
	require.NoError(t, c.Send("testpass123"))
	require.NoError(t, c.Expect("> ", 10*time.Second), "should reach post-login prompt")
}

func TestAuth_InvalidPassword(t *testing.T) {
	t.Parallel()
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	loginAsRaw(t, c, "claude_player", "wrongpassword")
	require.NoError(t, c.Expect("Invalid password", 5*time.Second),
		"server must reject invalid password")
}

func TestAuth_UnknownAccount(t *testing.T) {
	t.Parallel()
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	loginAsRaw(t, c, "nonexistent_user_xyz", "anypassword")
	require.NoError(t, c.Expect("Account not found", 5*time.Second),
		"server must report unknown account")
}

func TestAuth_EmptyUsername(t *testing.T) {
	t.Parallel()
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send(""))
	require.NoError(t, c.Expect("empty", 5*time.Second),
		"server must reject empty username")
}

func TestAuth_QuitBeforeLogin(t *testing.T) {
	t.Parallel()
	defer recordTiming(t, time.Now())
	c := NewClientForTest(t)
	require.NoError(t, c.Expect("Username", 10*time.Second))
	require.NoError(t, c.Send("quit"))
	require.NoError(t, c.Expect("Goodbye", 5*time.Second),
		"server must send goodbye on quit")
}

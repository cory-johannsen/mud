package handlers

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// mockAccountStore implements AccountStore for testing.
type mockAccountStore struct {
	accounts  map[string]postgres.Account
	passwords map[string]string
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{
		accounts:  make(map[string]postgres.Account),
		passwords: make(map[string]string),
	}
}

func (m *mockAccountStore) Create(_ context.Context, username, password string) (postgres.Account, error) {
	if _, exists := m.accounts[username]; exists {
		return postgres.Account{}, postgres.ErrAccountExists
	}
	acct := postgres.Account{
		ID:        int64(len(m.accounts) + 1),
		Username:  username,
		CreatedAt: time.Now(),
	}
	m.accounts[username] = acct
	m.passwords[username] = password
	return acct, nil
}

func (m *mockAccountStore) Authenticate(_ context.Context, username, password string) (postgres.Account, error) {
	acct, exists := m.accounts[username]
	if !exists {
		return postgres.Account{}, postgres.ErrAccountNotFound
	}
	if m.passwords[username] != password {
		return postgres.Account{}, postgres.ErrInvalidCredentials
	}
	return acct, nil
}

// testServer starts a Telnet acceptor with the given handler on a random port
// and returns the listening address. The acceptor is stopped on test cleanup.
func testServer(t *testing.T, handler *AuthHandler) string {
	t.Helper()
	logger := zaptest.NewLogger(t)
	cfg := config.TelnetConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	acc := telnet.NewAcceptor(cfg, handler, logger)
	go func() { _ = acc.ListenAndServe() }()

	deadline := time.After(2 * time.Second)
	for {
		if acc.IsRunning() && acc.Addr() != "" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("acceptor did not start in time")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Cleanup(func() { acc.Stop() })
	return acc.Addr()
}

// testClient connects to addr and returns a raw TCP conn with helpers.
// It maintains a persistent read buffer across readUntil calls, discarding
// only the data up to and including the matched substring.
type testClient struct {
	conn   net.Conn
	t      *testing.T
	buffer string
}

func newTestClient(t *testing.T, addr string) *testClient {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return &testClient{conn: conn, t: t}
}

func (tc *testClient) readUntil(substr string, timeout time.Duration) string {
	tc.t.Helper()

	// Check if we already have the substring in the buffer
	if idx := strings.Index(tc.buffer, substr); idx >= 0 {
		end := idx + len(substr)
		result := tc.buffer[:end]
		tc.buffer = tc.buffer[end:]
		return result
	}

	_ = tc.conn.SetReadDeadline(time.Now().Add(timeout))
	tmp := make([]byte, 4096)
	for {
		n, err := tc.conn.Read(tmp)
		if n > 0 {
			tc.buffer += string(tmp[:n])
			if idx := strings.Index(tc.buffer, substr); idx >= 0 {
				end := idx + len(substr)
				result := tc.buffer[:end]
				tc.buffer = tc.buffer[end:]
				return result
			}
		}
		if err != nil {
			tc.t.Fatalf("reading until %q: got %q, error: %v", substr, tc.buffer, err)
		}
	}
}

func (tc *testClient) send(line string) {
	tc.t.Helper()
	_ = tc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := tc.conn.Write([]byte(line + "\r\n"))
	require.NoError(tc.t, err)
}

// waitForPrompt reads through the welcome banner and initial telnet negotiations
// until the auth prompt "> " is visible. The banner contains "> " inside
// <username> tags, so we wait for "to disconnect." (last banner line) first.
func (tc *testClient) waitForPrompt() string {
	tc.t.Helper()
	return tc.readUntil("to disconnect.", 3*time.Second)
}

func TestWelcomeBannerContainsKeyElements(t *testing.T) {
	stripped := telnet.StripANSI(welcomeBanner)
	assert.Contains(t, stripped, "Post-Collapse Portland")
	assert.Contains(t, stripped, "login")
	assert.Contains(t, stripped, "register")
	assert.Contains(t, stripped, "quit")
}

func TestHandleSession_Quit(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("quit")
	c.readUntil("Goodbye!", 2*time.Second)
}

func TestHandleSession_Exit(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("exit")
	c.readUntil("Goodbye!", 2*time.Second)
}

func TestHandleSession_Help(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("help")
	output := c.readUntil("Disconnect", 2*time.Second)
	stripped := telnet.StripANSI(output)
	assert.Contains(t, stripped, "login")
	assert.Contains(t, stripped, "register")
	assert.Contains(t, stripped, "quit")
}

func TestHandleSession_UnknownCommand(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("foobar")
	output := c.readUntil("available commands", 2*time.Second)
	assert.Contains(t, telnet.StripANSI(output), "foobar")
}

func TestHandleSession_Register(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register testuser password123")
	output := c.readUntil("You may now", 2*time.Second)
	assert.Contains(t, telnet.StripANSI(output), "testuser")
}

func TestHandleSession_RegisterDuplicate(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["testuser"] = postgres.Account{ID: 1, Username: "testuser"}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register testuser password123")
	c.readUntil("already taken", 2*time.Second)
}

func TestHandleSession_RegisterShortUsername(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register ab password123")
	c.readUntil("3-32 characters", 2*time.Second)
}

func TestHandleSession_RegisterShortPassword(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register testuser abc")
	c.readUntil("at least 6", 2*time.Second)
}

func TestHandleSession_RegisterMissingArgs(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register")
	c.readUntil("Usage:", 2*time.Second)
}

func TestHandleSession_LoginNotFound(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("login nobody secret123")
	c.readUntil("Account not found", 2*time.Second)
}

func TestHandleSession_LoginWrongPassword(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["testuser"] = postgres.Account{ID: 1, Username: "testuser"}
	store.passwords["testuser"] = "correctpass"
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("login testuser wrongpass")
	c.readUntil("Invalid password", 2*time.Second)
}

func TestHandleSession_LoginMissingArgs(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("login")
	c.readUntil("Usage:", 2*time.Second)
}

func TestHandleSession_LoginSuccess_GameLoop(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["hero"] = postgres.Account{ID: 1, Username: "hero"}
	store.passwords["hero"] = "secret123"
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("login hero secret123")
	c.readUntil("Welcome back", 2*time.Second)

	// Game loop - look
	c.readUntil("]> ", 3*time.Second)
	c.send("look")
	c.readUntil("Pioneer Courthouse", 2*time.Second)

	// Game loop - help
	c.readUntil("]> ", 3*time.Second)
	c.send("help")
	c.readUntil("Disconnect", 2*time.Second)

	// Game loop - unknown command
	c.readUntil("]> ", 3*time.Second)
	c.send("dance")
	c.readUntil("don't know", 2*time.Second)

	// Game loop - quit
	c.readUntil("]> ", 3*time.Second)
	c.send("quit")
	c.readUntil("Goodbye", 2*time.Second)
}

func TestHandleSession_ServerShutdown(t *testing.T) {
	store := newMockAccountStore()
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(store, logger)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()

	// Close the client connection to simulate disconnect
	c.conn.Close()
}

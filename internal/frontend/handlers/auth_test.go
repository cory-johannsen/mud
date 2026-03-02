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
	"google.golang.org/grpc"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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
		Role:      postgres.RolePlayer,
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

// mockCharacterStore implements CharacterStore for testing.
// It pre-loads a single character so the character flow selects it immediately.
type mockCharacterStore struct {
	chars []*character.Character
}

func newMockCharacterStore(chars ...*character.Character) *mockCharacterStore {
	return &mockCharacterStore{chars: chars}
}

func (m *mockCharacterStore) ListByAccount(_ context.Context, _ int64) ([]*character.Character, error) {
	return m.chars, nil
}

func (m *mockCharacterStore) Create(_ context.Context, c *character.Character) (*character.Character, error) {
	c.ID = int64(len(m.chars) + 1)
	m.chars = append(m.chars, c)
	return c, nil
}

func (m *mockCharacterStore) GetByID(_ context.Context, id int64) (*character.Character, error) {
	for _, c := range m.chars {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

// newAuthHandler builds an AuthHandler with empty regions/classes for tests that
// do not exercise character creation.
func newAuthHandler(t *testing.T, store AccountStore, gsAddr string) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	chars := newMockCharacterStore()
	telnetCfg := config.TelnetConfig{
		IdleTimeout:     5 * time.Minute,
		IdleGracePeriod: time.Minute,
	}
	return NewAuthHandler(store, chars, []*ruleset.Region{}, []*ruleset.Team{}, []*ruleset.Job{}, logger, gsAddr, telnetCfg)
}

// newAuthHandlerWithChar builds an AuthHandler whose character store returns one
// pre-existing character. This causes the character-selection flow to immediately
// select that character and proceed to the game bridge.
func newAuthHandlerWithChar(t *testing.T, store AccountStore, char *character.Character, gsAddr string) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	chars := newMockCharacterStore(char)
	telnetCfg := config.TelnetConfig{
		IdleTimeout:     5 * time.Minute,
		IdleGracePeriod: time.Minute,
	}
	return NewAuthHandler(store, chars, []*ruleset.Region{}, []*ruleset.Team{}, []*ruleset.Job{}, logger, gsAddr, telnetCfg)
}

// testGameServer starts an in-process gRPC game server with a minimal 2-room world
// and returns its listen address. The server is stopped on test cleanup.
func testGameServer(t *testing.T) string {
	t.Helper()

	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone",
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Exits: []world.Exit{
					{Direction: world.North, TargetRoom: "room_b"},
				},
				Properties: map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits: []world.Exit{
					{Direction: world.South, TargetRoom: "room_a"},
				},
				Properties: map[string]string{},
			},
		},
	}

	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	sessMgr := session.NewManager()
	cmdRegistry := command.DefaultRegistry()
	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil)
	chatHandler := gameserver.NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := gameserver.NewGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "")

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	return lis.Addr().String()
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
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("quit")
	c.readUntil("Goodbye!", 2*time.Second)
}

func TestHandleSession_Exit(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("exit")
	c.readUntil("Goodbye!", 2*time.Second)
}

func TestHandleSession_Help(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
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
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("foobar")
	output := c.readUntil("available commands", 2*time.Second)
	assert.Contains(t, telnet.StripANSI(output), "foobar")
}

// doLogin sends the login command then provides username+password interactively.
// It waits for the "Password:" prompt before sending the password.
func (tc *testClient) doLogin(username, password string) {
	tc.t.Helper()
	tc.send("login " + username)
	tc.readUntil("Password:", 2*time.Second)
	tc.send(password)
}

// doRegister sends the register command then provides username, password, and confirmation.
func (tc *testClient) doRegister(username, password, confirm string) {
	tc.t.Helper()
	tc.send("register " + username)
	tc.readUntil("Password:", 2*time.Second)
	tc.send(password)
	tc.readUntil("Confirm password:", 2*time.Second)
	tc.send(confirm)
}

func TestHandleSession_Register(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doRegister("testuser", "password123", "password123")
	output := c.readUntil("You may now", 2*time.Second)
	assert.Contains(t, telnet.StripANSI(output), "testuser")
}

func TestHandleSession_RegisterDuplicate(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["testuser"] = postgres.Account{ID: 1, Username: "testuser"}
	store.passwords["testuser"] = "password123"
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doRegister("testuser", "password123", "password123")
	c.readUntil("already taken", 2*time.Second)
}

func TestHandleSession_RegisterShortUsername(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register ab")
	c.readUntil("3-32 characters", 2*time.Second)
}

func TestHandleSession_RegisterShortPassword(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.send("register testuser")
	c.readUntil("Password:", 2*time.Second)
	c.send("abc")
	c.readUntil("at least 6", 2*time.Second)
}

func TestHandleSession_RegisterPasswordMismatch(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doRegister("testuser", "password123", "different!")
	c.readUntil("do not match", 2*time.Second)
}

func TestHandleSession_RegisterMissingArgs(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	// No username arg — server prompts for it; send an empty line.
	c.waitForPrompt()
	c.send("register")
	c.readUntil("Username:", 2*time.Second)
	c.send("ab") // too short — triggers validation error without needing a password
	c.readUntil("3-32 characters", 2*time.Second)
}

func TestHandleSession_LoginNotFound(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doLogin("nobody", "secret123")
	c.readUntil("Account not found", 2*time.Second)
}

func TestHandleSession_LoginWrongPassword(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["testuser"] = postgres.Account{ID: 1, Username: "testuser"}
	store.passwords["testuser"] = "correctpass"
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doLogin("testuser", "wrongpass")
	c.readUntil("Invalid password", 2*time.Second)
}

func TestHandleSession_LoginMissingArgs(t *testing.T) {
	store := newMockAccountStore()
	store.accounts["testuser"] = postgres.Account{ID: 1, Username: "testuser"}
	store.passwords["testuser"] = "password123"
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	// No username arg — server prompts for it.
	c.waitForPrompt()
	c.send("login")
	c.readUntil("Username:", 2*time.Second)
	c.send("testuser")
	c.readUntil("Password:", 2*time.Second)
	c.send("password123")
	c.readUntil("Logged in as", 2*time.Second)
}

func TestHandleSession_LoginSuccess_GameBridge(t *testing.T) {
	gsAddr := testGameServer(t)

	store := newMockAccountStore()
	store.accounts["hero"] = postgres.Account{ID: 1, Username: "hero"}
	store.passwords["hero"] = "secret123"

	// Pre-load a character so the character flow immediately selects it.
	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10}
	handler := newAuthHandlerWithChar(t, store, char, gsAddr)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doLogin("hero", "secret123")
	c.readUntil("Logged in as", 2*time.Second)

	// Character flow: one character listed, send "1" to select it.
	c.readUntil("Your characters:", 2*time.Second)
	c.send("1")

	// After character selection, should receive initial room view + prompt
	c.readUntil("Room A", 5*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Game bridge - look: response comes back with room view + prompt
	c.send("look")
	c.readUntil("Room A", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Game bridge - move north
	c.send("north")
	c.readUntil("Room B", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Game bridge - exits
	c.send("exits")
	c.readUntil("south", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Game bridge - help (handled locally, prompt re-displayed)
	c.send("help")
	c.readUntil("Movement", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Game bridge - quit
	c.send("quit")
	c.readUntil("Goodbye", 3*time.Second)
}

func TestHandleSession_GameBridge_SayAndEmote(t *testing.T) {
	gsAddr := testGameServer(t)

	store := newMockAccountStore()
	store.accounts["hero"] = postgres.Account{ID: 1, Username: "hero"}
	store.passwords["hero"] = "secret123"

	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10}
	handler := newAuthHandlerWithChar(t, store, char, gsAddr)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doLogin("hero", "secret123")
	c.readUntil("Logged in as", 2*time.Second)
	c.readUntil("Your characters:", 2*time.Second)
	c.send("1")
	c.readUntil("Room A", 5*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Say command
	c.send("say hello world")
	c.readUntil("hero says: hello world", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Emote command
	c.send("emote waves")
	c.readUntil("hero waves", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Who command
	c.send("who")
	c.readUntil("hero", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Say with no args
	c.send("say")
	c.readUntil("Say what?", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Emote with no args
	c.send("emote")
	c.readUntil("Emote what?", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Unknown command (treated as move, returns error)
	c.send("dance")
	c.readUntil("no exit", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	c.send("quit")
	c.readUntil("Goodbye", 3*time.Second)
}

func TestHandleSession_GameBridge_MoveAlias(t *testing.T) {
	gsAddr := testGameServer(t)

	store := newMockAccountStore()
	store.accounts["hero"] = postgres.Account{ID: 1, Username: "hero"}
	store.passwords["hero"] = "secret123"

	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10}
	handler := newAuthHandlerWithChar(t, store, char, gsAddr)
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()
	c.doLogin("hero", "secret123")
	c.readUntil("Logged in as", 2*time.Second)
	c.readUntil("Your characters:", 2*time.Second)
	c.send("1")
	c.readUntil("Room A", 5*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Move using alias 'n' for north
	c.send("n")
	c.readUntil("Room B", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	// Move back using alias 's' for south
	c.send("s")
	c.readUntil("Room A", 3*time.Second)
	c.readUntil("hp]", 3*time.Second)

	c.send("quit")
	c.readUntil("Goodbye", 3*time.Second)
}

func TestHandleSession_ServerShutdown(t *testing.T) {
	store := newMockAccountStore()
	handler := newAuthHandler(t, store, "127.0.0.1:50051")
	addr := testServer(t, handler)
	c := newTestClient(t, addr)

	c.waitForPrompt()

	// Close the client connection to simulate disconnect
	c.conn.Close()
}

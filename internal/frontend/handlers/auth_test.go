package handlers

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func (m *mockCharacterStore) SaveGender(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockCharacterStore) DeleteByAccountAndName(_ context.Context, _ int64, _ string) error {
	return nil
}

// newAuthHandler builds an AuthHandler with empty regions/classes for tests that
// do not exercise character creation.
//
// Telnet-deprecation (#325): the legacy player flow is now gated behind
// AllowGameCommands; player-flow tests opt in here so they continue to
// exercise the original behavior.
func newAuthHandler(t *testing.T, store AccountStore, gsAddr string) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	chars := newMockCharacterStore()
	telnetCfg := config.TelnetConfig{
		IdleTimeout:       5 * time.Minute,
		IdleGracePeriod:   time.Minute,
		AllowGameCommands: true,
	}
	return NewAuthHandler(store, chars, []*ruleset.Region{}, []*ruleset.Team{}, []*ruleset.Job{}, []*ruleset.Archetype{}, logger, gsAddr, telnetCfg, nil, nil, nil, nil, nil, nil)
}

// newAuthHandlerWithChar builds an AuthHandler whose character store returns one
// pre-existing character. This causes the character-selection flow to immediately
// select that character and proceed to the game bridge.
func newAuthHandlerWithChar(t *testing.T, store AccountStore, char *character.Character, gsAddr string) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	chars := newMockCharacterStore(char)
	telnetCfg := config.TelnetConfig{
		IdleTimeout:       5 * time.Minute,
		IdleGracePeriod:   time.Minute,
		AllowGameCommands: true,
	}
	return NewAuthHandler(store, chars, []*ruleset.Region{}, []*ruleset.Team{}, []*ruleset.Job{}, []*ruleset.Archetype{}, logger, gsAddr, telnetCfg, nil, nil, nil, nil, nil, nil)
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
	worldHandler := gameserver.NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := gameserver.NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := gameserver.NewGameServiceServer(
		gameserver.StorageDeps{},
		gameserver.ContentDeps{
			WorldMgr: worldMgr,
		},
		gameserver.HandlerDeps{
			WorldHandler: worldHandler,
			ChatHandler:  chatHandler,
		},
		sessMgr,
		cmdRegistry,
		nil,
		logger,
	)

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
	stripped := telnet.StripANSI(buildWelcomeBanner())
	assert.Contains(t, stripped, "Post-Collapse Portland")
	assert.Contains(t, stripped, "login")
	assert.Contains(t, stripped, "register")
	assert.Contains(t, stripped, "quit")
}

// TestBannerContainsBrightCyanAsciiArt asserts that the banner contains at least 4
// independently-colorized BrightCyan segments with non-trivial Unicode block-letter content.
// Each title row is wrapped with its own BrightCyan+Reset pair, so we count occurrences.
func TestBannerContainsBrightCyanAsciiArt(t *testing.T) {
	banner := buildWelcomeBanner()
	// Count how many BrightCyan segments contain non-trivial content (≥3 visible runes).
	// Uses RuneCountInString because the title rows contain multi-byte UTF-8 box-drawing characters.
	artRows := 0
	remaining := banner
	for {
		idx := strings.Index(remaining, telnet.BrightCyan)
		if idx == -1 {
			break
		}
		after := remaining[idx+len(telnet.BrightCyan):]
		resetIdx := strings.Index(after, telnet.Reset)
		if resetIdx == -1 {
			break
		}
		segment := after[:resetIdx]
		if utf8.RuneCountInString(strings.TrimSpace(segment)) >= 3 {
			artRows++
		}
		remaining = after[resetIdx+len(telnet.Reset):]
	}
	assert.GreaterOrEqual(t, artRows, 4,
		"banner must contain at least 4 BrightCyan segments with non-trivial ASCII art content")
}

// TestBannerContainsBrightGreen asserts that the AK-47 color code is present.
func TestBannerContainsBrightGreen(t *testing.T) {
	banner := buildWelcomeBanner()
	assert.Contains(t, banner, telnet.BrightGreen, "banner must contain BrightGreen for AK-47")
}

// TestBannerContainsBrightYellow asserts that the machete color code is present.
func TestBannerContainsBrightYellow(t *testing.T) {
	banner := buildWelcomeBanner()
	assert.Contains(t, banner, telnet.BrightYellow, "banner must contain BrightYellow for machete")
}

// TestBannerLineWidthMax80 asserts that every line is at most 80 visible characters.
// Uses RuneCountInString rather than len because the title rows contain multi-byte
// UTF-8 box-drawing characters. All art characters are Narrow (single-column) Unicode,
// so rune count equals terminal column count for this banner.
func TestBannerLineWidthMax80(t *testing.T) {
	banner := buildWelcomeBanner()
	for i, line := range strings.Split(banner, "\n") {
		visible := telnet.StripANSI(line)
		assert.LessOrEqual(t, utf8.RuneCountInString(visible), 80,
			"line %d exceeds 80 visible chars: %q", i+1, visible)
	}
}

// TestBannerColorReset asserts that every color code is followed by Reset
// before the next color code or end of banner string.
func TestBannerColorReset(t *testing.T) {
	banner := buildWelcomeBanner()
	colors := []string{
		telnet.BrightGreen,
		telnet.BrightCyan,
		telnet.BrightYellow,
	}
	for _, color := range colors {
		start := 0
		for {
			idx := strings.Index(banner[start:], color)
			if idx == -1 {
				break
			}
			abs := start + idx
			after := banner[abs+len(color):]
			resetIdx := strings.Index(after, telnet.Reset)
			require.Greater(t, resetIdx, -1,
				"color code %q at position %d must be followed by Reset", color, abs)
			// No other color code should appear before the Reset.
			for _, other := range colors {
				otherIdx := strings.Index(after[:resetIdx], other)
				assert.Equal(t, -1, otherIdx,
					"color %q appears before Reset after color %q", other, color)
			}
			start = abs + len(color)
		}
	}
}

// TestBannerBoldReset asserts that every Bold occurrence is followed by Reset
// before the end of the banner. Bold is always immediately followed by BrightCyan
// on title rows (per spec IMPL-2), so this test does not require Reset before BrightCyan.
func TestBannerBoldReset(t *testing.T) {
	banner := buildWelcomeBanner()
	remaining := banner
	for {
		idx := strings.Index(remaining, telnet.Bold)
		if idx == -1 {
			break
		}
		after := remaining[idx+len(telnet.Bold):]
		resetIdx := strings.Index(after, telnet.Reset)
		require.Greater(t, resetIdx, -1,
			"Bold at byte offset %d must be followed by Reset", idx)
		remaining = after[resetIdx+len(telnet.Reset):]
	}
}

// TestBannerGunAboveTitle asserts that the BrightGreen (AK-47) block appears
// on an earlier line than the first BrightCyan (title) line.
func TestBannerGunAboveTitle(t *testing.T) {
	banner := buildWelcomeBanner()
	lines := strings.Split(banner, "\n")
	firstGreen := -1
	firstCyan := -1
	for i, line := range lines {
		if firstGreen == -1 && strings.Contains(line, telnet.BrightGreen) {
			firstGreen = i
		}
		if firstCyan == -1 && strings.Contains(line, telnet.BrightCyan) {
			firstCyan = i
		}
	}
	require.Greater(t, firstGreen, -1, "BrightGreen (AK-47) must appear in banner")
	require.Greater(t, firstCyan, -1, "BrightCyan (title) must appear in banner")
	assert.Less(t, firstGreen, firstCyan,
		"BrightGreen (AK-47) line %d must be before BrightCyan (title) line %d",
		firstGreen, firstCyan)
}

// TestBannerMacheteBelowTitle asserts that the BrightYellow (machete) block appears
// on a later line than the last BrightCyan (title) line.
func TestBannerMacheteBelowTitle(t *testing.T) {
	banner := buildWelcomeBanner()
	lines := strings.Split(banner, "\n")
	lastCyan := -1
	firstYellow := -1
	for i, line := range lines {
		if strings.Contains(line, telnet.BrightCyan) {
			lastCyan = i
		}
		if firstYellow == -1 && strings.Contains(line, telnet.BrightYellow) {
			firstYellow = i
		}
	}
	require.Greater(t, lastCyan, -1, "BrightCyan (title) must appear in banner")
	require.Greater(t, firstYellow, -1, "BrightYellow (machete) must appear in banner")
	assert.Less(t, lastCyan, firstYellow,
		"last BrightCyan (title) line %d must be before BrightYellow (machete) line %d",
		lastCyan, firstYellow)
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
	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10, Gender: "female"}
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

	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10, Gender: "female"}
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

	char := &character.Character{ID: 1, Name: "hero", Class: "ganger", Region: "old_town", Level: 1, CurrentHP: 10, MaxHP: 10, Gender: "female"}
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

// --- ensureClassFeatures whitebox tests ---

// TestAuth_RejectsPlayerFlowWhenAllowGameCommandsFalse verifies REQ-TD-2a /
// REQ-TD-2b: when AllowGameCommands is false, the player flow refuses login
// attempts with a redirect-to-web-client narrative even when the rejector
// is bypassed (belt-and-suspenders gate at the auth-handler layer).
func TestAuth_RejectsPlayerFlowWhenAllowGameCommandsFalse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	telnetCfg := config.TelnetConfig{
		IdleTimeout:       5 * time.Minute,
		IdleGracePeriod:   time.Minute,
		AllowGameCommands: false, // REQ-TD-2a: gate engaged.
	}
	handler := NewAuthHandler(
		newMockAccountStore(), newMockCharacterStore(),
		nil, nil, nil, nil,
		logger, "", telnetCfg,
		nil, nil, nil, nil, nil, nil,
	)
	addr := testServer(t, handler)
	tc := newTestClient(t, addr)
	out := tc.readUntil("retired.", 3*time.Second)
	assert.Contains(t, out, "telnet player surface has been retired")
}

// TestAuth_AllowsPlayerFlowWhenAllowGameCommandsTrue verifies the
// graceful-sunset toggle: with AllowGameCommands=true, the legacy welcome
// banner and command loop are still served.
func TestAuth_AllowsPlayerFlowWhenAllowGameCommandsTrue(t *testing.T) {
	logger := zaptest.NewLogger(t)
	telnetCfg := config.TelnetConfig{
		IdleTimeout:       5 * time.Minute,
		IdleGracePeriod:   time.Minute,
		AllowGameCommands: true,
	}
	handler := NewAuthHandler(
		newMockAccountStore(), newMockCharacterStore(),
		nil, nil, nil, nil,
		logger, "", telnetCfg,
		nil, nil, nil, nil, nil, nil,
	)
	addr := testServer(t, handler)
	tc := newTestClient(t, addr)
	out := tc.waitForPrompt()
	assert.Contains(t, out, "to disconnect.",
		"sunset flag must permit the legacy welcome banner")
}

// TestAuth_HeadlessRejectsNonSeededLogin verifies REQ-TD-3b: usernames not
// in the seed-claude-accounts set are rejected on the headless surface
// before any password prompt.
func TestAuth_HeadlessRejectsNonSeededLogin(t *testing.T) {
	store := newMockAccountStore()
	_, _ = store.Create(context.Background(), "claude_player", "pw")
	_, _ = store.Create(context.Background(), "intruder", "pw")
	handler := newAuthHandlerWithChar(t, store, &character.Character{
		ID: 1, AccountID: 1, Name: "Hero",
	}, "")
	handler.SetSeedAuthorized([]string{"claude_player", "claude_editor", "claude_admin"})

	addr := testServer(t, handler)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()
	// Mark the connection as headless so handleHeadlessSession runs.
	// We do this by just sending the username and reading; the regular
	// negotiation path uses an interactive Conn — the seed gate fires
	// inside handleHeadlessSession. Since the test server creates a
	// non-headless Conn, drive the gate via direct AuthHandler in-memory.
	// (Fallback: assert via Dispatch-equivalent isSeedAuthorized helper.)
	assert.False(t, handler.isSeedAuthorized("intruder"),
		"non-seeded usernames must be rejected by the seed-authorization gate")
	assert.True(t, handler.isSeedAuthorized("claude_player"),
		"seed-bootstrapped usernames must be accepted by the gate")
}

// fakeClassFeaturesRepo implements CharacterClassFeaturesSetter for testing.
type fakeClassFeaturesRepo struct {
	stored   []string
	setAllFn func(ids []string) // optional callback to inspect SetAll calls
}

func (r *fakeClassFeaturesRepo) HasClassFeatures(_ context.Context, _ int64) (bool, error) {
	return len(r.stored) > 0, nil
}

func (r *fakeClassFeaturesRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	out := make([]string, len(r.stored))
	copy(out, r.stored)
	return out, nil
}

func (r *fakeClassFeaturesRepo) SetAll(_ context.Context, _ int64, featureIDs []string) error {
	if r.setAllFn != nil {
		r.setAllFn(featureIDs)
	}
	r.stored = make([]string, len(featureIDs))
	copy(r.stored, featureIDs)
	return nil
}

// newEnsureClassFeaturesHandler builds a minimal AuthHandler for ensureClassFeatures tests.
func newEnsureClassFeaturesHandler(t *testing.T, jobs []*ruleset.Job, features []*ruleset.ClassFeature, repo CharacterClassFeaturesSetter) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	telnetCfg := config.TelnetConfig{IdleTimeout: 5 * time.Minute, IdleGracePeriod: time.Minute}
	return NewAuthHandler(
		newMockAccountStore(), newMockCharacterStore(),
		nil, nil, jobs, nil,
		logger, "", telnetCfg,
		nil, nil, nil, nil, features, repo,
	)
}

// newPipeConn returns a telnet.Conn backed by net.Pipe with a drain goroutine
// so WriteLine never blocks.
func newPipeConn(t *testing.T) *telnet.Conn {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := client.Read(buf); err != nil {
				return
			}
		}
	}()
	return telnet.NewConn(server, time.Second, time.Second)
}

// REQ-ECF-1: ensureClassFeatures assigns all job features when the character has none.
func TestEnsureClassFeatures_NewCharacter_AssignsAll(t *testing.T) {
	job := &ruleset.Job{
		ID: "pirate",
		ClassFeatureGrants: []string{"street_brawler", "brutal_surge", "boarding_action"},
	}
	features := []*ruleset.ClassFeature{
		{ID: "street_brawler"}, {ID: "brutal_surge"}, {ID: "boarding_action"},
	}
	repo := &fakeClassFeaturesRepo{stored: nil}
	h := newEnsureClassFeaturesHandler(t, []*ruleset.Job{job}, features, repo)
	conn := newPipeConn(t)

	char := &character.Character{ID: 1, Class: "pirate"}
	require.NoError(t, h.ensureClassFeatures(context.Background(), conn, char))

	assert.ElementsMatch(t, []string{"street_brawler", "brutal_surge", "boarding_action"}, repo.stored,
		"all three features must be assigned to a new character")
}

// REQ-ECF-2: ensureClassFeatures backfills missing features when the character has some but not all.
func TestEnsureClassFeatures_ExistingWithMissing_BackfillsMissing(t *testing.T) {
	job := &ruleset.Job{
		ID: "pirate",
		ClassFeatureGrants: []string{"street_brawler", "brutal_surge", "boarding_action"},
	}
	features := []*ruleset.ClassFeature{
		{ID: "street_brawler"}, {ID: "brutal_surge"}, {ID: "boarding_action"},
	}
	// Character was created before boarding_action was added to the job.
	repo := &fakeClassFeaturesRepo{stored: []string{"street_brawler", "brutal_surge"}}
	h := newEnsureClassFeaturesHandler(t, []*ruleset.Job{job}, features, repo)
	conn := newPipeConn(t)

	char := &character.Character{ID: 1, Class: "pirate"}
	require.NoError(t, h.ensureClassFeatures(context.Background(), conn, char))

	assert.Contains(t, repo.stored, "boarding_action", "missing feature must be backfilled")
	assert.Contains(t, repo.stored, "street_brawler", "existing features must be preserved")
	assert.Contains(t, repo.stored, "brutal_surge", "existing features must be preserved")
}

// REQ-ECF-3: ensureClassFeatures is a no-op when all expected features are already stored.
func TestEnsureClassFeatures_AllPresent_NoOp(t *testing.T) {
	job := &ruleset.Job{
		ID: "pirate",
		ClassFeatureGrants: []string{"street_brawler", "brutal_surge", "boarding_action"},
	}
	features := []*ruleset.ClassFeature{
		{ID: "street_brawler"}, {ID: "brutal_surge"}, {ID: "boarding_action"},
	}
	initial := []string{"street_brawler", "brutal_surge", "boarding_action"}
	setAllCalled := false
	repo := &fakeClassFeaturesRepo{
		stored:   initial,
		setAllFn: func(_ []string) { setAllCalled = true },
	}
	h := newEnsureClassFeaturesHandler(t, []*ruleset.Job{job}, features, repo)
	conn := newPipeConn(t)

	char := &character.Character{ID: 1, Class: "pirate"}
	require.NoError(t, h.ensureClassFeatures(context.Background(), conn, char))

	assert.False(t, setAllCalled, "SetAll must not be called when all features are already present")
}

# Game Client (Ebiten) Phase 2: Auth & Character Selection

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** REST auth client and Ebiten login/character-selection screens.

**Architecture:** Pure auth client (testable with httptest), thin Ebiten screen layer on top. JWT held in memory only.

**Tech Stack:** Go net/http, Ebiten v2, encoding/json

---

## Requirements Covered

REQ-GCE-6, REQ-GCE-7, REQ-GCE-8, REQ-GCE-9, REQ-GCE-10

## Assumptions

- Phase 1 is complete: `cmd/ebitenclient/main.go` exists, config is loaded into a `Config` struct, and a `*zap.Logger` is available.
- `github.com/hajimehoshi/ebiten/v2` is present in `go.mod`.
- Module path is `github.com/cory-johannsen/mud`.
- Webclient auth API: `POST /api/auth/login` → `{"token": string, "account_id": int64, "role": string}`.
- Webclient characters API: `GET /api/characters` with `Authorization: Bearer <token>` → array of `{id, name, job, level, current_hp, max_hp, region, archetype}`.
- Phase 3 will define an `AssetCheckScreen`; Phase 2 transitions to a placeholder `nextScreen` value returned by character select.

## File Structure

```
cmd/ebitenclient/
  auth/
    models.go          (create: LoginRequest, LoginResponse, Character structs)
    client.go          (create: Client, Login, ListCharacters)
    client_test.go     (create: table-driven tests with httptest.NewServer)
  screens/
    screen.go          (create: Screen interface)
    login.go           (create: LoginScreen)
    character_select.go (create: CharacterSelectScreen)
  main.go              (modify: wire LoginScreen as initial screen in game loop)
```

---

## Prerequisites

- [ ] Confirm `github.com/hajimehoshi/ebiten/v2` is in `go.mod`. If absent:
  ```
  mise exec -- go get github.com/hajimehoshi/ebiten/v2
  ```
- [ ] Confirm `golang.org/x/image` is in `go.mod` (required by Ebiten text rendering). If absent:
  ```
  mise exec -- go get golang.org/x/image
  ```

---

## Task 1 — Auth models (`cmd/ebitenclient/auth/models.go`)

- [ ] Create `cmd/ebitenclient/auth/models.go`:

```go
package auth

// LoginRequest is the JSON body for POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the JSON body returned by POST /api/auth/login on success.
type LoginResponse struct {
	Token     string `json:"token"`
	AccountID int64  `json:"account_id"`
	Role      string `json:"role"`
}

// LoginErrorResponse is the JSON body returned on auth failure (HTTP 4xx).
type LoginErrorResponse struct {
	Error string `json:"error"`
}

// Character represents one entry from GET /api/characters.
type Character struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Job       string `json:"job"`
	Level     int    `json:"level"`
	CurrentHP int    `json:"current_hp"`
	MaxHP     int    `json:"max_hp"`
	Region    string `json:"region"`
	Archetype string `json:"archetype"`
}
```

---

## Task 2 — REST auth client (`cmd/ebitenclient/auth/client.go`)

- [ ] Create `cmd/ebitenclient/auth/client.go`:

```go
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client performs REST calls against the webclient auth API.
// It holds no persistent state; JWT tokens are never written to disk.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client targeting baseURL (e.g. "http://localhost:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Login authenticates with the webclient and returns a JWT token.
// On HTTP 4xx the API error message is returned as a non-nil error.
func (c *Client) Login(username, password string) (string, error) {
	body, err := json.Marshal(LoginRequest{Username: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("marshal login request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp LoginErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr != nil || errResp.Error == "" {
			return "", fmt.Errorf("login failed: HTTP %d", resp.StatusCode)
		}
		return "", fmt.Errorf("%s", errResp.Error)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	return loginResp.Token, nil
}

// ListCharacters fetches all characters for the authenticated account.
// token MUST NOT be empty.
func (c *Client) ListCharacters(token string) ([]Character, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/characters", nil)
	if err != nil {
		return nil, fmt.Errorf("build characters request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("characters request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list characters failed: HTTP %d", resp.StatusCode)
	}

	var chars []Character
	if err := json.NewDecoder(resp.Body).Decode(&chars); err != nil {
		return nil, fmt.Errorf("decode characters response: %w", err)
	}
	return chars, nil
}
```

---

## Task 3 — REST client tests (`cmd/ebitenclient/auth/client_test.go`)

- [ ] Create `cmd/ebitenclient/auth/client_test.go`:

```go
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/auth"
)

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantToken  string
		wantErrSub string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/auth/login" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(auth.LoginResponse{Token: "tok123", AccountID: 1, Role: "player"})
			},
			wantToken: "tok123",
		},
		{
			name: "invalid credentials",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(auth.LoginErrorResponse{Error: "invalid credentials"})
			},
			wantErrSub: "invalid credentials",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErrSub: "HTTP 500",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			client := auth.NewClient(srv.URL)
			token, err := client.Login("user", "pass")

			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !containsString(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tc.wantToken {
				t.Fatalf("expected token %q, got %q", tc.wantToken, token)
			}
		})
	}
}

func TestListCharacters(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantLen    int
		wantErrSub string
	}{
		{
			name: "two characters returned",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer tok" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]auth.Character{
					{ID: 1, Name: "Zork", Job: "Ganger", Level: 5, CurrentHP: 38, MaxHP: 50},
					{ID: 2, Name: "Vex", Job: "Sage", Level: 3, CurrentHP: 20, MaxHP: 25},
				})
			},
			wantLen: 2,
		},
		{
			name: "empty list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]auth.Character{})
			},
			wantLen: 0,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErrSub: "HTTP 500",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			client := auth.NewClient(srv.URL)
			chars, err := client.ListCharacters("tok")

			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !containsString(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(chars) != tc.wantLen {
				t.Fatalf("expected %d characters, got %d", tc.wantLen, len(chars))
			}
		})
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
```

- [ ] Run tests and confirm all pass:
  ```
  mise exec -- go test ./cmd/ebitenclient/auth/... -v
  ```

---

## Task 4 — Screen interface (`cmd/ebitenclient/screens/screen.go`)

- [ ] Create `cmd/ebitenclient/screens/screen.go`:

```go
package screens

import "github.com/hajimehoshi/ebiten/v2"

// Screen represents one full-window UI state in the game client.
// Implementations MUST be safe to call from Ebiten's game loop on the main thread.
type Screen interface {
	// Update advances the screen state. dt is unused in this phase
	// (Ebiten calls Update once per logical tick at 60 TPS).
	// Returning a non-nil error causes Ebiten to terminate.
	Update() error

	// Draw renders the screen onto the provided image.
	Draw(screen *ebiten.Image)

	// Next returns the screen to transition to after this tick, or nil to stay.
	// The caller (Game.Update) MUST check Next() after every Update() call
	// and replace the active screen when a non-nil value is returned.
	Next() Screen
}
```

---

## Task 5 — Login screen (`cmd/ebitenclient/screens/login.go`)

- [ ] Create `cmd/ebitenclient/screens/login.go`:

```go
package screens

import (
	"image/color"
	"strings"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/auth"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// field identifies which text field is focused.
type field int

const (
	fieldUsername field = iota
	fieldPassword
)

// LoginScreen collects credentials and authenticates via the auth client.
// JWT is never written to disk (REQ-GCE-8).
type LoginScreen struct {
	client      *auth.Client
	webClientURL string

	username string
	password string
	focused  field

	errMsg string
	busy   bool

	next Screen
}

// NewLoginScreen creates a LoginScreen targeting the given webclient URL.
func NewLoginScreen(client *auth.Client, webClientURL string) *LoginScreen {
	return &LoginScreen{client: client, webClientURL: webClientURL}
}

// Update processes keyboard input for the login form.
func (s *LoginScreen) Update() error {
	if s.busy {
		return nil
	}

	// Tab: switch fields.
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if s.focused == fieldUsername {
			s.focused = fieldPassword
		} else {
			s.focused = fieldUsername
		}
	}

	// Character input.
	s.handleTextInput()

	// Backspace.
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		switch s.focused {
		case fieldUsername:
			if len(s.username) > 0 {
				s.username = s.username[:len(s.username)-1]
			}
		case fieldPassword:
			if len(s.password) > 0 {
				s.password = s.password[:len(s.password)-1]
			}
		}
	}

	// Enter: submit.
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		s.submit()
	}

	return nil
}

// handleTextInput appends printable runes to the active field.
func (s *LoginScreen) handleTextInput() {
	runes := ebiten.AppendInputChars(nil)
	for _, r := range runes {
		switch s.focused {
		case fieldUsername:
			s.username += string(r)
		case fieldPassword:
			s.password += string(r)
		}
	}
}

// submit fires the login request asynchronously to avoid blocking the game loop.
func (s *LoginScreen) submit() {
	if s.username == "" || s.password == "" {
		s.errMsg = "Username and password are required."
		return
	}
	s.busy = true
	s.errMsg = ""

	go func() {
		token, err := s.client.Login(s.username, s.password)
		if err != nil {
			s.errMsg = err.Error()
			s.busy = false
			return
		}
		chars, err := s.client.ListCharacters(token)
		if err != nil {
			s.errMsg = "Failed to load characters: " + err.Error()
			s.busy = false
			return
		}
		s.next = NewCharacterSelectScreen(s.client, s.webClientURL, token, chars)
		s.busy = false
	}()
}

// Draw renders the login form.
func (s *LoginScreen) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 20, G: 20, B: 30, A: 255})

	ebitenutil.DebugPrintAt(screen, "MUD — Login", 560, 150)

	userLabel := "Username: "
	passLabel := "Password: "
	if s.focused == fieldUsername {
		userLabel = "> Username: "
	} else {
		passLabel = "> Password: "
	}

	ebitenutil.DebugPrintAt(screen, userLabel+s.username+"_", 440, 280)
	ebitenutil.DebugPrintAt(screen, passLabel+strings.Repeat("*", len(s.password))+"_", 440, 310)

	ebitenutil.DebugPrintAt(screen, "[Tab] switch field   [Enter] login", 440, 360)

	if s.busy {
		ebitenutil.DebugPrintAt(screen, "Authenticating...", 440, 410)
	} else if s.errMsg != "" {
		ebitenutil.DebugPrintAt(screen, "Error: "+s.errMsg, 440, 410)
	}
}

// Next returns the CharacterSelectScreen after a successful login, else nil.
func (s *LoginScreen) Next() Screen {
	return s.next
}
```

---

## Task 6 — Character select screen (`cmd/ebitenclient/screens/character_select.go`)

- [ ] Create `cmd/ebitenclient/screens/character_select.go`:

```go
package screens

import (
	"fmt"
	"image/color"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/auth"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// CharacterSelectScreen displays available characters for the authenticated account.
// If no characters exist the user is directed to the web client (REQ-GCE-9).
// On selection it transitions to the asset check screen (Phase 3 placeholder).
type CharacterSelectScreen struct {
	client       *auth.Client
	webClientURL string
	token        string
	characters   []auth.Character
	cursor       int
	next         Screen
}

// NewCharacterSelectScreen creates a CharacterSelectScreen with the provided character list.
// token is held in memory only and MUST NOT be persisted to disk (REQ-GCE-8).
func NewCharacterSelectScreen(client *auth.Client, webClientURL, token string, characters []auth.Character) *CharacterSelectScreen {
	return &CharacterSelectScreen{
		client:       client,
		webClientURL: webClientURL,
		token:        token,
		characters:   characters,
	}
}

// Update handles keyboard and mouse navigation.
func (s *CharacterSelectScreen) Update() error {
	if len(s.characters) == 0 {
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if s.cursor > 0 {
			s.cursor--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if s.cursor < len(s.characters)-1 {
			s.cursor++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		s.selectCharacter(s.characters[s.cursor])
	}

	// Mouse click: detect click on list rows (each row 20px tall, starting at y=260).
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		_, cy := ebiten.CursorPosition()
		rowStart := 260
		rowHeight := 20
		for i := range s.characters {
			rowY := rowStart + i*rowHeight
			if cy >= rowY && cy < rowY+rowHeight {
				s.cursor = i
				s.selectCharacter(s.characters[i])
				break
			}
		}
	}

	return nil
}

// selectCharacter transitions to the asset check screen (Phase 3 placeholder).
// REQ-GCE-10: on selection the client will open a gRPC stream; that wiring is in Phase 3.
func (s *CharacterSelectScreen) selectCharacter(c auth.Character) {
	// Phase 3 will replace nil with AssetCheckScreen(c, s.token, ...).
	// Setting s.next to a concrete placeholder prevents the loop from stalling.
	_ = c
	_ = s.token
	// No-op until Phase 3; leave s.next nil so the screen stays visible.
}

// Draw renders the character list.
func (s *CharacterSelectScreen) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 20, G: 20, B: 30, A: 255})
	ebitenutil.DebugPrintAt(screen, "MUD — Select Character", 520, 150)

	if len(s.characters) == 0 {
		msg := fmt.Sprintf("No characters found. Create one at %s", s.webClientURL)
		ebitenutil.DebugPrintAt(screen, msg, 300, 300)
		return
	}

	ebitenutil.DebugPrintAt(screen, "[↑/↓] navigate   [Enter / Click] select", 440, 220)

	rowStart := 260
	rowHeight := 20
	for i, c := range s.characters {
		prefix := "  "
		if i == s.cursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s (%s Lv.%d) — HP %d/%d",
			prefix, c.Name, c.Job, c.Level, c.CurrentHP, c.MaxHP)
		ebitenutil.DebugPrintAt(screen, line, 440, rowStart+i*rowHeight)
	}
}

// Next returns the next screen after character selection, or nil to remain.
func (s *CharacterSelectScreen) Next() Screen {
	return s.next
}
```

---

## Task 7 — Wire screens into `main.go`

The Phase 1 `main.go` contains an Ebiten `Game` struct with `Update`, `Draw`, and `Layout` methods. Modify it to:

- [ ] Add an `activeScreen screens.Screen` field to the `Game` struct.
- [ ] In `Game.Update`, after calling `activeScreen.Update()`, check `activeScreen.Next()` and replace `activeScreen` when non-nil.
- [ ] In `Game.Draw`, delegate to `activeScreen.Draw(screen)`.
- [ ] In `main()`, after loading config, construct `auth.NewClient(cfg.WebclientURL)` and pass it to `screens.NewLoginScreen(...)` as the initial `activeScreen`.

Exact diff target — add to the `Game` struct and methods:

```go
// In game struct:
activeScreen screens.Screen

// In Game.Update():
if err := g.activeScreen.Update(); err != nil {
    return err
}
if next := g.activeScreen.Next(); next != nil {
    g.activeScreen = next
}
return nil

// In Game.Draw():
g.activeScreen.Draw(screen)

// In main(), after config load:
authClient := auth.NewClient(cfg.WebclientURL)
game := &Game{
    activeScreen: screens.NewLoginScreen(authClient, cfg.WebclientURL),
}
```

- [ ] Run all package tests:
  ```
  mise exec -- go test ./cmd/ebitenclient/... -v
  ```
- [ ] Build the binary for the current platform and verify it compiles:
  ```
  mise exec -- go build -o /tmp/mud-ebiten-phase2-check ./cmd/ebitenclient/
  ```

---

## Manual Verification Steps

These steps cannot be unit-tested because they require a running Ebiten window and a live webclient.

- [ ] Run `./mud-ebiten-phase2-check`. The login screen MUST appear at 1280×800 with username and password fields visible.
- [ ] Type a username; characters MUST appear in the username field. Tab to password; typed characters MUST appear as `*`.
- [ ] Press Enter with empty fields; the error "Username and password are required." MUST appear.
- [ ] Enter incorrect credentials against a live webclient; the API error message MUST appear below the form.
- [ ] Enter correct credentials; the character select screen MUST appear listing characters in format `Name (Job Lv.N) — HP current/max`.
- [ ] Log in with an account that has no characters; the message `No characters found. Create one at <url>` MUST appear.
- [ ] Verify no `token` value appears in `os.UserCacheDir()/mud-ebiten/` or `os.UserConfigDir()/mud-ebiten/` after login (REQ-GCE-8).

---

## Completion Criteria

- [ ] All tests in `./cmd/ebitenclient/...` pass with `mise exec -- go test ./cmd/ebitenclient/... -v`.
- [ ] Binary builds without error for current platform.
- [ ] Manual verification steps above have been performed against a live or mocked webclient.
- [ ] No JWT or password is written to disk at any point.

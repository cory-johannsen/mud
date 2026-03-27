# internal/client Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `internal/client` shared library (5 sub-packages: render, history, auth, feed, session) consumed by `cmd/webclient` and `cmd/ebitenclient`.

**Architecture:** Five acyclic sub-packages: `render` defines color token constants and renderer interfaces; `history` and `auth` are self-contained with no inter-package deps; `feed` imports `render` for `ColorToken`; `session` manages gRPC lifecycle and the client state machine with an injected command parser. All concrete rendering stays in the client binaries.

**Tech Stack:** Go 1.26, `pgregory.net/rapid` (property tests), `net/http/httptest` (auth tests), `google.golang.org/grpc/test/bufconn` (session tests), `github.com/stretchr/testify`, proto types from `internal/gameserver/gamev1`.

---

## File Map

| File | Responsibility |
|---|---|
| `internal/client/render/render.go` | `ColorToken` constants, `FeedEntry`, `CharacterSnapshot`, `FeedRenderer`, `CharacterRenderer`, `ColorMapper` interfaces |
| `internal/client/history/history.go` | `History` ring buffer: `Push`, `Up`, `Down`, `Reset` |
| `internal/client/history/history_test.go` | Property-based tests for cap, cursor, eviction invariants |
| `internal/client/auth/auth.go` | `Client`, all account/character HTTP methods, typed errors |
| `internal/client/auth/auth_test.go` | `httptest.Server`-based tests for all endpoints |
| `internal/client/feed/feed.go` | `Feed` ring buffer, `Entry`, `Append`, `Entries`, `Clear`, `DefaultTokenFor` |
| `internal/client/feed/feed_test.go` | Property-based tests for cap enforcement, token assignment, goroutine safety |
| `internal/client/session/session.go` | `Session` struct, state machine, gRPC stream lifecycle, reconnect backoff |
| `internal/client/session/session_test.go` | bufconn-based tests for state transitions, send/recv, reconnect |

---

## Task 1: `internal/client/render` — Color Tokens & Renderer Interfaces

**Files:**
- Create: `internal/client/render/render.go`

This package has no logic — only constants and interfaces. No test file is required (REQ-IC-5). We verify it compiles cleanly.

- [ ] **Step 1: Create the render package**

```go
// internal/client/render/render.go
package render

import "time"

// ColorToken identifies the semantic color category for a feed entry or UI element.
// Each client maps tokens to its native representation (CSS class, color.RGBA, ANSI escape).
type ColorToken int

const (
	ColorDefault    ColorToken = iota
	ColorCombat              // CombatEvent, RoundStartEvent, RoundEndEvent
	ColorSpeech              // MessageEvent (say/emote)
	ColorRoomEvent           // arrival/departure events
	ColorSystem              // system messages and unclassified events
	ColorError               // ErrorEvent
	ColorStructured          // CharacterInfo, InventoryView, CharacterSheetView
)

// FeedEntry is a single message in the feed panel.
// It mirrors feed.Entry without creating an import cycle.
type FeedEntry struct {
	Timestamp time.Time
	Token     ColorToken
	Text      string // pre-extracted narrative text; clients render this directly
}

// CharacterSnapshot is a point-in-time view of the character panel state.
// It mirrors session.CharacterState without creating an import cycle.
type CharacterSnapshot struct {
	Name       string
	Level      int
	CurrentHP  int
	MaxHP      int
	Conditions []string
	HeroPoints int
	AP         int
}

// FeedRenderer renders a slice of feed entries into the client's native output.
type FeedRenderer interface {
	RenderFeed(entries []FeedEntry) error
}

// CharacterRenderer renders the character panel state into the client's native output.
type CharacterRenderer interface {
	RenderCharacter(snap CharacterSnapshot) error
}

// ColorMapper maps a ColorToken to a client-native value T.
// T is typically color.RGBA (Ebiten), string CSS class (web), or ANSI code (telnet).
type ColorMapper[T any] interface {
	Map(token ColorToken) T
}
```

- [ ] **Step 2: Verify it compiles**

```bash
mise exec -- go build ./internal/client/render/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/client/render/render.go
git commit -m "feat(client/render): add ColorToken constants and renderer interfaces"
```

---

## Task 2: `internal/client/history` — Command Ring Buffer

**Files:**
- Create: `internal/client/history/history.go`
- Create: `internal/client/history/history_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/client/history/history_test.go
package history_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/client/history"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHistory_DefaultCap(t *testing.T) {
	h := history.New(0)
	// Push 101 entries — oldest must be evicted
	for i := 0; i < 101; i++ {
		h.Push(string(rune('a' + i%26)))
	}
	// Navigate all the way back: should reach no further than cap=100 entries
	count := 0
	for h.Up() != "" {
		count++
	}
	assert.Equal(t, 100, count)
}

func TestHistory_UpDown_Empty(t *testing.T) {
	h := history.New(5)
	assert.Equal(t, "", h.Up())
	assert.Equal(t, "", h.Down())
}

func TestHistory_UpDown_Order(t *testing.T) {
	h := history.New(5)
	h.Push("first")
	h.Push("second")
	h.Push("third")

	assert.Equal(t, "third", h.Up())
	assert.Equal(t, "second", h.Up())
	assert.Equal(t, "first", h.Up())
	assert.Equal(t, "", h.Up()) // at oldest
	assert.Equal(t, "first", h.Down())
	assert.Equal(t, "second", h.Down())
	assert.Equal(t, "third", h.Down())
	assert.Equal(t, "", h.Down()) // past newest (live position)
}

func TestHistory_PushResetsCorsor(t *testing.T) {
	h := history.New(5)
	h.Push("a")
	h.Push("b")
	h.Up() // move cursor back
	h.Push("c")
	// After Push, cursor resets: Up() returns "c" (most recent)
	assert.Equal(t, "c", h.Up())
}

func TestHistory_Reset(t *testing.T) {
	h := history.New(5)
	h.Push("x")
	h.Push("y")
	h.Up()
	h.Up()
	h.Reset()
	assert.Equal(t, "y", h.Up()) // cursor back at live position
}

func TestHistory_Property_CapEnforced(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 50).Draw(rt, "cap")
		n := rapid.IntRange(cap, cap*3).Draw(rt, "n")
		h := history.New(cap)
		for i := 0; i < n; i++ {
			h.Push("entry")
		}
		count := 0
		for h.Up() != "" {
			count++
			if count > cap {
				rt.Fatalf("navigated more than cap=%d entries", cap)
			}
		}
		if count != cap {
			rt.Fatalf("expected %d entries, got %d", cap, count)
		}
	})
}

func TestHistory_Property_UpDownSymmetry(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(2, 20).Draw(rt, "cap")
		n := rapid.IntRange(1, cap).Draw(rt, "n")
		h := history.New(cap)
		entries := make([]string, n)
		for i := 0; i < n; i++ {
			entries[i] = rapid.StringMatching(`[a-z]+`).Draw(rt, "entry")
			h.Push(entries[i])
		}
		// Navigate all the way back
		collected := []string{}
		for {
			v := h.Up()
			if v == "" {
				break
			}
			collected = append(collected, v)
		}
		// Navigate all the way forward
		rebuilt := []string{}
		for {
			v := h.Down()
			if v == "" {
				break
			}
			rebuilt = append(rebuilt, v)
		}
		// collected is newest→oldest; rebuilt is oldest→newest
		// rebuilt reversed must equal collected
		for i, j := 0, len(rebuilt)-1; i < j; i, j = i+1, j-1 {
			rebuilt[i], rebuilt[j] = rebuilt[j], rebuilt[i]
		}
		if len(collected) != len(rebuilt) {
			rt.Fatalf("up count %d != down count %d", len(collected), len(rebuilt))
		}
		for i := range collected {
			if collected[i] != rebuilt[i] {
				rt.Fatalf("mismatch at %d: up=%q down=%q", i, collected[i], rebuilt[i])
			}
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/client/history/... -v 2>&1 | head -20
```

Expected: compile error — `history` package does not exist yet.

- [ ] **Step 3: Implement the ring buffer**

```go
// internal/client/history/history.go
package history

const defaultCap = 100

// History is a command ring buffer with ↑/↓ cursor navigation.
// Not goroutine-safe — designed for single-input-goroutine use.
type History struct {
	buf    []string
	cap    int
	head   int // index of oldest entry
	count  int // number of valid entries
	cursor int // offset from newest: 0 = live, 1 = newest, count = oldest
}

// New creates a History with the given capacity. cap=0 uses the default (100).
func New(cap int) *History {
	if cap <= 0 {
		cap = defaultCap
	}
	return &History{buf: make([]string, cap), cap: cap}
}

// Push adds a command to the history and resets the cursor to the live position.
func (h *History) Push(cmd string) {
	if cmd == "" {
		return
	}
	idx := (h.head + h.count) % h.cap
	if h.count < h.cap {
		h.count++
	} else {
		// Overwrite oldest — advance head
		h.head = (h.head + 1) % h.cap
	}
	h.buf[idx] = cmd
	h.cursor = 0
}

// Up navigates toward older entries. Returns "" when already at the oldest entry.
func (h *History) Up() string {
	if h.cursor >= h.count {
		return ""
	}
	h.cursor++
	return h.buf[(h.head+h.count-h.cursor+h.cap)%h.cap]
}

// Down navigates toward newer entries. Returns "" when at the live position.
func (h *History) Down() string {
	if h.cursor <= 0 {
		return ""
	}
	h.cursor--
	if h.cursor == 0 {
		return ""
	}
	return h.buf[(h.head+h.count-h.cursor+h.cap)%h.cap]
}

// Reset returns the cursor to the live position (call after each command submission).
func (h *History) Reset() {
	h.cursor = 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/client/history/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/history/history.go internal/client/history/history_test.go
git commit -m "feat(client/history): add command ring buffer with property-based tests"
```

---

## Task 3: `internal/client/auth` — HTTP Client

**Files:**
- Create: `internal/client/auth/auth.go`
- Create: `internal/client/auth/auth_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/client/auth/auth_test.go
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cory-johannsen/mud/internal/client/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Login_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/auth/login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      "test.jwt.token",
			"account_id": 42,
			"role":       "player",
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	jwt, err := c.Login(context.Background(), "alice", "password")
	require.NoError(t, err)
	assert.Equal(t, "test.jwt.token", jwt)
}

func TestClient_Login_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.Login(context.Background(), "alice", "wrong")
	require.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestClient_Register_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/auth/register", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      "reg.jwt.token",
			"account_id": 99,
			"role":       "player",
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	jwt, err := c.Register(context.Background(), "newuser", "password123")
	require.NoError(t, err)
	assert.Equal(t, "reg.jwt.token", jwt)
}

func TestClient_Register_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "username too short"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.Register(context.Background(), "ab", "password123")
	var ve auth.ErrValidation
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "username too short")
}

func TestClient_Me(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer myjwt", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"account_id": 7,
			"username":   "bob",
			"role":       "admin",
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	acct, err := c.Me(context.Background(), "myjwt")
	require.NoError(t, err)
	assert.Equal(t, int64(7), acct.ID)
	assert.Equal(t, "bob", acct.Username)
	assert.Equal(t, "admin", acct.Role)
}

func TestClient_ListCharacters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/characters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "Zara", "job": "ganger", "level": 3,
				"current_hp": 25, "max_hp": 30, "region": "Northeast", "archetype": "brawler"},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	chars, err := c.ListCharacters(context.Background(), "tok")
	require.NoError(t, err)
	require.Len(t, chars, 1)
	assert.Equal(t, "Zara", chars[0].Name)
	assert.Equal(t, 3, chars[0].Level)
}

func TestClient_CreateCharacter_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/characters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"character": map[string]any{
				"id": 5, "name": "Rex", "job": "runner", "level": 1,
				"current_hp": 15, "max_hp": 15, "region": "Southeast", "archetype": "scout",
			},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	ch, err := c.CreateCharacter(context.Background(), "tok", auth.CreateCharacterRequest{
		Name: "Rex", Job: "runner", Archetype: "scout", Region: "Southeast", Gender: "male",
	})
	require.NoError(t, err)
	assert.Equal(t, "Rex", ch.Name)
}

func TestClient_CreateCharacter_NameTaken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "name taken"})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	_, err := c.CreateCharacter(context.Background(), "tok", auth.CreateCharacterRequest{Name: "Rex"})
	require.ErrorIs(t, err, auth.ErrNameTaken)
}

func TestClient_CheckName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Rex", r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"available": true})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	ok, err := c.CheckName(context.Background(), "Rex")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestClient_CharacterOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/characters/options", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"regions": []string{"Northeast", "Southeast"},
			"jobs":    []string{"ganger", "runner"},
			"archetypes": map[string][]string{
				"ganger": {"brawler", "enforcer"},
				"runner": {"scout", "courier"},
			},
		})
	}))
	defer srv.Close()

	c := auth.New(srv.URL)
	opts, err := c.CharacterOptions(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, []string{"Northeast", "Southeast"}, opts.Regions)
	assert.Equal(t, []string{"brawler", "enforcer"}, opts.Archetypes["ganger"])
}

func TestClient_NetworkError(t *testing.T) {
	c := auth.New("http://127.0.0.1:1") // nothing listening
	_, err := c.Login(context.Background(), "u", "p")
	var ne auth.ErrNetwork
	require.ErrorAs(t, err, &ne)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/client/auth/... -v 2>&1 | head -10
```

Expected: compile error — `auth` package does not exist yet.

- [ ] **Step 3: Implement the auth client**

```go
// internal/client/auth/auth.go
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrUnauthorized is returned when the server responds with HTTP 401.
var ErrUnauthorized = errors.New("unauthorized")

// ErrNameTaken is returned when the server responds with HTTP 409 on character creation.
var ErrNameTaken = errors.New("character name already taken")

// ErrValidation is returned when the server responds with HTTP 400.
type ErrValidation struct{ Message string }

func (e ErrValidation) Error() string { return "validation error: " + e.Message }

// ErrNetwork is returned when the HTTP request itself fails (connection refused, timeout, etc.).
type ErrNetwork struct{ Cause error }

func (e ErrNetwork) Error() string { return "network error: " + e.Cause.Error() }
func (e ErrNetwork) Unwrap() error { return e.Cause }

// Account holds the authenticated account details returned by /api/auth/me.
type Account struct {
	ID       int64
	Username string
	Role     string
}

// CharacterSummary is a character as returned by the character list and creation endpoints.
type CharacterSummary struct {
	ID        int64
	Name      string
	Job       string
	Level     int
	CurrentHP int
	MaxHP     int
	Region    string
	Archetype string
}

// CreateCharacterRequest is the payload for POST /api/characters.
type CreateCharacterRequest struct {
	Name      string
	Job       string
	Archetype string
	Region    string
	Gender    string
}

// CharacterOptions contains the available choices for character creation.
type CharacterOptions struct {
	Regions    []string
	Jobs       []string
	Archetypes map[string][]string // job → available archetypes
}

// Client is an HTTP client for the webclient REST API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Client targeting the given base URL (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{}}
}

// Login authenticates with username/password and returns a JWT on success.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	body := map[string]string{"username": username, "password": password}
	var resp struct {
		Token string `json:"token"`
	}
	if err := c.post(ctx, "/api/auth/login", "", body, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// Register creates an account and returns a JWT on success.
func (c *Client) Register(ctx context.Context, username, password string) (string, error) {
	body := map[string]string{"username": username, "password": password}
	var resp struct {
		Token string `json:"token"`
	}
	if err := c.post(ctx, "/api/auth/register", "", body, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// Me returns the authenticated account details for the given JWT.
func (c *Client) Me(ctx context.Context, jwt string) (*Account, error) {
	var resp struct {
		AccountID int64  `json:"account_id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
	}
	if err := c.get(ctx, "/api/auth/me", jwt, &resp); err != nil {
		return nil, err
	}
	return &Account{ID: resp.AccountID, Username: resp.Username, Role: resp.Role}, nil
}

// ListCharacters returns the characters belonging to the authenticated account.
func (c *Client) ListCharacters(ctx context.Context, jwt string) ([]CharacterSummary, error) {
	var raw []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Job       string `json:"job"`
		Level     int    `json:"level"`
		CurrentHP int    `json:"current_hp"`
		MaxHP     int    `json:"max_hp"`
		Region    string `json:"region"`
		Archetype string `json:"archetype"`
	}
	if err := c.get(ctx, "/api/characters", jwt, &raw); err != nil {
		return nil, err
	}
	out := make([]CharacterSummary, len(raw))
	for i, r := range raw {
		out[i] = CharacterSummary{
			ID: r.ID, Name: r.Name, Job: r.Job, Level: r.Level,
			CurrentHP: r.CurrentHP, MaxHP: r.MaxHP, Region: r.Region, Archetype: r.Archetype,
		}
	}
	return out, nil
}

// CreateCharacter creates a new character and returns its summary.
func (c *Client) CreateCharacter(ctx context.Context, jwt string, req CreateCharacterRequest) (*CharacterSummary, error) {
	body := map[string]string{
		"name": req.Name, "job": req.Job, "archetype": req.Archetype,
		"region": req.Region, "gender": req.Gender,
	}
	var resp struct {
		Character struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Job       string `json:"job"`
			Level     int    `json:"level"`
			CurrentHP int    `json:"current_hp"`
			MaxHP     int    `json:"max_hp"`
			Region    string `json:"region"`
			Archetype string `json:"archetype"`
		} `json:"character"`
	}
	if err := c.post(ctx, "/api/characters", jwt, body, &resp); err != nil {
		return nil, err
	}
	r := resp.Character
	return &CharacterSummary{
		ID: r.ID, Name: r.Name, Job: r.Job, Level: r.Level,
		CurrentHP: r.CurrentHP, MaxHP: r.MaxHP, Region: r.Region, Archetype: r.Archetype,
	}, nil
}

// CheckName returns true if the character name is available.
func (c *Client) CheckName(ctx context.Context, name string) (bool, error) {
	u := "/api/characters/check-name?name=" + url.QueryEscape(name)
	var resp struct {
		Available bool `json:"available"`
	}
	if err := c.get(ctx, u, "", &resp); err != nil {
		return false, err
	}
	return resp.Available, nil
}

// CharacterOptions returns available regions, jobs, and archetypes for character creation.
func (c *Client) CharacterOptions(ctx context.Context, jwt string) (*CharacterOptions, error) {
	var resp struct {
		Regions    []string            `json:"regions"`
		Jobs       []string            `json:"jobs"`
		Archetypes map[string][]string `json:"archetypes"`
	}
	if err := c.get(ctx, "/api/characters/options", jwt, &resp); err != nil {
		return nil, err
	}
	return &CharacterOptions{Regions: resp.Regions, Jobs: resp.Jobs, Archetypes: resp.Archetypes}, nil
}

// --- internal helpers ---

func (c *Client) get(ctx context.Context, path, jwt string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path, jwt string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrNetwork{Cause: err}
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		if out != nil {
			return json.Unmarshal(data, out)
		}
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusConflict:
		return ErrNameTaken
	case http.StatusBadRequest:
		var e struct{ Error string `json:"error"` }
		_ = json.Unmarshal(data, &e)
		return ErrValidation{Message: e.Error}
	default:
		var e struct{ Error string `json:"error"` }
		_ = json.Unmarshal(data, &e)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, e.Error)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/client/auth/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/auth/auth.go internal/client/auth/auth_test.go
git commit -m "feat(client/auth): add HTTP client for webclient REST API"
```

---

## Task 4: `internal/client/feed` — Message Feed

**Files:**
- Create: `internal/client/feed/feed.go`
- Create: `internal/client/feed/feed_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/client/feed/feed_test.go
package feed_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/client/feed"
	"github.com/cory-johannsen/mud/internal/client/render"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestFeed_DefaultCap(t *testing.T) {
	f := feed.New(0)
	// Append 501 events — oldest must be evicted leaving 500
	for i := 0; i < 501; i++ {
		f.Append(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "msg"},
			},
		})
	}
	assert.Equal(t, 500, len(f.Entries()))
}

func TestFeed_CustomCap(t *testing.T) {
	f := feed.New(10)
	for i := 0; i < 15; i++ {
		f.Append(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "x"},
			},
		})
	}
	assert.Equal(t, 10, len(f.Entries()))
}

func TestFeed_TokenAssignment(t *testing.T) {
	cases := []struct {
		event *gamev1.ServerEvent
		token render.ColorToken
	}{
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Sender: "Alice", Content: "hi"},
			}},
			render.ColorSpeech,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CombatEvent{
				CombatEvent: &gamev1.CombatEvent{Narrative: "You hit!"},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoundStart{
				RoundStart: &gamev1.RoundStartEvent{Round: 1},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoundEnd{
				RoundEnd: &gamev1.RoundEndEvent{Round: 1},
			}},
			render.ColorCombat,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
				RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE},
			}},
			render.ColorRoomEvent,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "oops"},
			}},
			render.ColorError,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterInfo{
				CharacterInfo: &gamev1.CharacterInfo{Name: "Zara"},
			}},
			render.ColorStructured,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_InventoryView{
				InventoryView: &gamev1.InventoryView{},
			}},
			render.ColorStructured,
		},
		{
			&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterSheet{
				CharacterSheet: &gamev1.CharacterSheetView{Name: "Zara"},
			}},
			render.ColorStructured,
		},
	}
	for _, tc := range cases {
		f := feed.New(10)
		f.Append(tc.event)
		entries := f.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, tc.token, entries[0].Token, "wrong token for event type")
	}
}

func TestFeed_TextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
		Error: &gamev1.ErrorEvent{Message: "not found"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "not found", entries[0].Text)
}

func TestFeed_CombatTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CombatEvent{
		CombatEvent: &gamev1.CombatEvent{Narrative: "You strike the goblin for 7 damage."},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "You strike the goblin for 7 damage.", entries[0].Text)
}

func TestFeed_MessageTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Message{
		Message: &gamev1.MessageEvent{Sender: "Alice", Content: "Hello!"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "Alice: Hello!", entries[0].Text)
}

func TestFeed_RoomEventTextExtraction(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
		RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE, Direction: "north"},
	}})
	entries := f.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "Bob arrives from the north.", entries[0].Text)
}

func TestFeed_RoomEventDepart(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_RoomEvent{
		RoomEvent: &gamev1.RoomEvent{Player: "Bob", Type: gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART, Direction: "west"},
	}})
	entries := f.Entries()
	assert.Equal(t, "Bob leaves to the west.", entries[0].Text)
}

func TestFeed_Clear(t *testing.T) {
	f := feed.New(10)
	f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "x"}}})
	f.Clear()
	assert.Empty(t, f.Entries())
}

func TestFeed_EntriesOrder(t *testing.T) {
	f := feed.New(5)
	for i := 0; i < 3; i++ {
		f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
			Error: &gamev1.ErrorEvent{Message: string(rune('a' + i))},
		}})
		time.Sleep(time.Millisecond)
	}
	entries := f.Entries()
	require.Len(t, entries, 3)
	// oldest→newest
	assert.True(t, entries[0].Timestamp.Before(entries[1].Timestamp))
	assert.True(t, entries[1].Timestamp.Before(entries[2].Timestamp))
}

func TestFeed_GoRoutineSafe(t *testing.T) {
	f := feed.New(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "x"}}})
			_ = f.Entries()
		}()
	}
	wg.Wait()
}

func TestFeed_Property_CapEnforced(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 50).Draw(rt, "cap")
		n := rapid.IntRange(cap, cap*3).Draw(rt, "n")
		f := feed.New(cap)
		for i := 0; i < n; i++ {
			f.Append(&gamev1.ServerEvent{Payload: &gamev1.ServerEvent_Error{
				Error: &gamev1.ErrorEvent{Message: "x"},
			}})
		}
		got := len(f.Entries())
		if got != cap {
			rt.Fatalf("expected %d entries, got %d", cap, got)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/client/feed/... -v 2>&1 | head -10
```

Expected: compile error — `feed` package does not exist yet.

- [ ] **Step 3: Implement the feed**

```go
// internal/client/feed/feed.go
package feed

import (
	"fmt"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/client/render"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const defaultCap = 500

// Entry is a single message accumulated in the feed.
type Entry struct {
	Timestamp time.Time
	Token     render.ColorToken
	Text      string // pre-extracted human-readable text
}

// Feed accumulates ServerEvent messages as Entry values, enforcing a cap.
// Goroutine-safe.
type Feed struct {
	mu    sync.Mutex
	buf   []Entry
	cap   int
	head  int
	count int
}

// New creates a Feed with the given cap. cap=0 uses the default (500).
func New(cap int) *Feed {
	if cap <= 0 {
		cap = defaultCap
	}
	return &Feed{buf: make([]Entry, cap), cap: cap}
}

// Append adds a ServerEvent to the feed. If the feed is at capacity, the oldest
// entry is evicted.
func (f *Feed) Append(ev *gamev1.ServerEvent) {
	entry := Entry{
		Timestamp: time.Now(),
		Token:     DefaultTokenFor(ev),
		Text:      extractText(ev),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := (f.head + f.count) % f.cap
	if f.count < f.cap {
		f.count++
	} else {
		f.head = (f.head + 1) % f.cap
	}
	f.buf[idx] = entry
}

// Entries returns a snapshot of all entries in oldest→newest order.
func (f *Feed) Entries() []Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Entry, f.count)
	for i := 0; i < f.count; i++ {
		out[i] = f.buf[(f.head+i)%f.cap]
	}
	return out
}

// Clear removes all entries from the feed.
func (f *Feed) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.head = 0
	f.count = 0
}

// DefaultTokenFor returns the default ColorToken for the given ServerEvent.
// Exported so clients can override individual assignments before building their mapper.
func DefaultTokenFor(ev *gamev1.ServerEvent) render.ColorToken {
	if ev == nil {
		return render.ColorSystem
	}
	switch ev.Payload.(type) {
	case *gamev1.ServerEvent_Message:
		return render.ColorSpeech
	case *gamev1.ServerEvent_CombatEvent, *gamev1.ServerEvent_RoundStart, *gamev1.ServerEvent_RoundEnd:
		return render.ColorCombat
	case *gamev1.ServerEvent_RoomEvent:
		return render.ColorRoomEvent
	case *gamev1.ServerEvent_Error:
		return render.ColorError
	case *gamev1.ServerEvent_CharacterInfo, *gamev1.ServerEvent_InventoryView, *gamev1.ServerEvent_CharacterSheet:
		return render.ColorStructured
	default:
		return render.ColorSystem
	}
}

func extractText(ev *gamev1.ServerEvent) string {
	if ev == nil {
		return ""
	}
	switch p := ev.Payload.(type) {
	case *gamev1.ServerEvent_Error:
		if p.Error != nil {
			return p.Error.GetMessage()
		}
	case *gamev1.ServerEvent_CombatEvent:
		if p.CombatEvent != nil {
			return p.CombatEvent.GetNarrative()
		}
	case *gamev1.ServerEvent_Message:
		if p.Message != nil {
			return fmt.Sprintf("%s: %s", p.Message.GetSender(), p.Message.GetContent())
		}
	case *gamev1.ServerEvent_RoomEvent:
		if p.RoomEvent != nil {
			re := p.RoomEvent
			switch re.GetType() {
			case gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE:
				return fmt.Sprintf("%s arrives from the %s.", re.GetPlayer(), re.GetDirection())
			case gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART:
				return fmt.Sprintf("%s leaves to the %s.", re.GetPlayer(), re.GetDirection())
			}
		}
	case *gamev1.ServerEvent_RoundStart:
		if p.RoundStart != nil {
			return fmt.Sprintf("Round %d begins. Turn order: %v", p.RoundStart.GetRound(), p.RoundStart.GetTurnOrder())
		}
	case *gamev1.ServerEvent_RoundEnd:
		if p.RoundEnd != nil {
			return fmt.Sprintf("Round %d ends.", p.RoundEnd.GetRound())
		}
	case *gamev1.ServerEvent_CharacterInfo:
		if p.CharacterInfo != nil {
			ci := p.CharacterInfo
			return fmt.Sprintf("%s (Lv %d) — HP %d/%d", ci.GetName(), ci.GetLevel(), ci.GetCurrentHp(), ci.GetMaxHp())
		}
	case *gamev1.ServerEvent_CharacterSheet:
		if p.CharacterSheet != nil {
			cs := p.CharacterSheet
			return fmt.Sprintf("%s (Lv %d) — HP %d/%d", cs.GetName(), cs.GetLevel(), cs.GetCurrentHp(), cs.GetMaxHp())
		}
	case *gamev1.ServerEvent_InventoryView:
		return "[Inventory]"
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/client/feed/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/feed/feed.go internal/client/feed/feed_test.go
git commit -m "feat(client/feed): add goroutine-safe feed with token assignment and property tests"
```

---

## Task 5: `internal/client/session` — State Machine & gRPC Lifecycle

**Files:**
- Create: `internal/client/session/session.go`
- Create: `internal/client/session/session_test.go`

Note: `google.golang.org/grpc/test/bufconn` is part of the grpc module already present in `go.mod`. Add it as an import — no `go get` needed.

- [ ] **Step 1: Write the failing tests**

```go
// internal/client/session/session_test.go
package session_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/client/session"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// fakeGameServer implements gamev1.GameServiceServer minimally for testing.
type fakeGameServer struct {
	gamev1.UnimplementedGameServiceServer
	recvCh chan *gamev1.ClientMessage
	sendCh chan *gamev1.ServerEvent
}

func (f *fakeGameServer) Session(stream gamev1.GameService_SessionServer) error {
	// Send one event then block until client closes.
	for _, ev := range f.sendCh {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	// Block until stream closes.
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if f.recvCh != nil {
			select {
			case f.recvCh <- msg:
			default:
			}
		}
	}
}

func newTestSession(t *testing.T, fake *fakeGameServer) (*session.Session, func()) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	gamev1.RegisterGameServiceServer(srv, fake)
	go srv.Serve(lis)

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	parser := func(cmd string) (*gamev1.ClientMessage, error) {
		return &gamev1.ClientMessage{}, nil
	}

	s := session.NewWithConn(conn, parser)
	return s, func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}
}

func TestSession_InitialState(t *testing.T) {
	s := session.New("localhost:50051", func(string) (*gamev1.ClientMessage, error) {
		return &gamev1.ClientMessage{}, nil
	})
	state := s.State()
	assert.Equal(t, session.StateDisconnected, state.Current)
	assert.Nil(t, state.Character)
	assert.NoError(t, state.Error)
}

func TestSession_Connect_TransitionsToInGame(t *testing.T) {
	evCh := make(chan *gamev1.ServerEvent, 1)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	err := s.Connect("jwt.token", 42)
	require.NoError(t, err)
	assert.Equal(t, session.StateInGame, s.State().Current)
}

func TestSession_RecvEvent(t *testing.T) {
	ev := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Error{Error: &gamev1.ErrorEvent{Message: "hello"}},
	}
	evCh := make(chan *gamev1.ServerEvent, 1)
	evCh <- ev
	close(evCh)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))

	select {
	case got := <-s.Events():
		require.NotNil(t, got)
		assert.Equal(t, "hello", got.GetError().GetMessage())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSession_Send(t *testing.T) {
	recvCh := make(chan *gamev1.ClientMessage, 1)
	fake := &fakeGameServer{
		sendCh: make(chan *gamev1.ServerEvent),
		recvCh: recvCh,
	}
	close(fake.sendCh)
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	require.NoError(t, s.Send("move north"))

	select {
	case msg := <-recvCh:
		assert.NotNil(t, msg)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive message")
	}
}

func TestSession_Close(t *testing.T) {
	fake := &fakeGameServer{sendCh: make(chan *gamev1.ServerEvent)}
	close(fake.sendCh)
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	require.NoError(t, s.Close())
	assert.Equal(t, session.StateDisconnected, s.State().Current)
}

func TestSession_CharacterStateUpdatedFromCharacterInfo(t *testing.T) {
	ci := &gamev1.CharacterInfo{
		Name: "Zara", Level: 3, CurrentHp: 25, MaxHp: 30,
	}
	ev := &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_CharacterInfo{CharacterInfo: ci}}
	evCh := make(chan *gamev1.ServerEvent, 1)
	evCh <- ev
	close(evCh)
	fake := &fakeGameServer{sendCh: evCh}
	s, cleanup := newTestSession(t, fake)
	defer cleanup()

	require.NoError(t, s.Connect("jwt", 1))
	// Drain the event
	<-s.Events()
	time.Sleep(10 * time.Millisecond)

	state := s.State()
	require.NotNil(t, state.Character)
	assert.Equal(t, "Zara", state.Character.Name)
	assert.Equal(t, 3, state.Character.Level)
	assert.Equal(t, 25, state.Character.CurrentHP)
	assert.Equal(t, 30, state.Character.MaxHP)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/client/session/... -v 2>&1 | head -10
```

Expected: compile error — `session` package does not exist yet.

- [ ] **Step 3: Implement the session**

```go
// internal/client/session/session.go
package session

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/grpc"
)

// SessionState represents the client session state machine state.
type SessionState int

const (
	StateDisconnected    SessionState = iota
	StateAuthenticating
	StateCharacterSelect
	StateInGame
	StateReconnecting
)

// CharacterState is the current character panel data, updated automatically by the recv pump.
type CharacterState struct {
	Name       string
	Level      int
	CurrentHP  int
	MaxHP      int
	Conditions []string
	HeroPoints int
	AP         int
}

// State is a snapshot of the session.
type State struct {
	Current   SessionState
	Character *CharacterState // non-nil when StateInGame
	Error     error           // last terminal error
}

// Session manages a gRPC GameService.Session stream and the client state machine.
type Session struct {
	grpcAddr  string
	cmdParser func(string) (*gamev1.ClientMessage, error)
	conn      *grpc.ClientConn // nil when using grpcAddr, set by NewWithConn

	mu        sync.RWMutex
	state     SessionState
	charState *CharacterState
	lastErr   error

	stream   gamev1.GameService_SessionClient
	cancelFn context.CancelFunc
	events   chan *gamev1.ServerEvent
	sendMu   sync.Mutex
}

// New creates a Session that dials grpcAddr on Connect.
func New(grpcAddr string, cmdParser func(string) (*gamev1.ClientMessage, error)) *Session {
	return &Session{
		grpcAddr:  grpcAddr,
		cmdParser: cmdParser,
		state:     StateDisconnected,
		events:    make(chan *gamev1.ServerEvent, 64),
	}
}

// NewWithConn creates a Session using an existing gRPC connection (used in tests).
func NewWithConn(conn *grpc.ClientConn, cmdParser func(string) (*gamev1.ClientMessage, error)) *Session {
	return &Session{
		conn:      conn,
		cmdParser: cmdParser,
		state:     StateDisconnected,
		events:    make(chan *gamev1.ServerEvent, 64),
	}
}

// Connect opens the gRPC stream and transitions to StateInGame.
// Returns an error if already connected.
func (s *Session) Connect(jwt string, characterID int64) error {
	s.mu.Lock()
	if s.state != StateDisconnected {
		s.mu.Unlock()
		return fmt.Errorf("session already connected (state=%d)", s.state)
	}
	s.state = StateAuthenticating
	s.mu.Unlock()

	conn, err := s.dial()
	if err != nil {
		s.transitionDisconnected(err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := gamev1.NewGameServiceClient(conn)
	stream, err := client.Session(ctx)
	if err != nil {
		cancel()
		s.transitionDisconnected(err)
		return err
	}

	// Send JoinWorldRequest
	joinMsg := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				CharacterId: characterID,
			},
		},
	}
	if err := stream.Send(joinMsg); err != nil {
		cancel()
		s.transitionDisconnected(err)
		return err
	}

	s.mu.Lock()
	s.stream = stream
	s.cancelFn = cancel
	s.state = StateInGame
	s.mu.Unlock()

	go s.recvPump()
	return nil
}

// Send parses cmd and sends it over the gRPC stream.
func (s *Session) Send(cmd string) error {
	msg, err := s.cmdParser(cmd)
	if err != nil {
		return fmt.Errorf("parse command: %w", err)
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.RLock()
	stream := s.stream
	s.mu.RUnlock()
	if stream == nil {
		return fmt.Errorf("not connected")
	}
	return stream.Send(msg)
}

// Events returns the channel of ServerEvents from the gRPC stream.
// The channel is closed when the session transitions to StateDisconnected.
func (s *Session) Events() <-chan *gamev1.ServerEvent {
	return s.events
}

// State returns a snapshot of the current session state.
func (s *Session) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return State{
		Current:   s.state,
		Character: s.charState,
		Error:     s.lastErr,
	}
}

// Close gracefully shuts down the gRPC stream.
func (s *Session) Close() error {
	s.mu.Lock()
	cancel := s.cancelFn
	stream := s.stream
	s.state = StateDisconnected
	s.stream = nil
	s.cancelFn = nil
	s.mu.Unlock()

	var err error
	if stream != nil {
		err = stream.CloseSend()
	}
	if cancel != nil {
		cancel()
	}
	return err
}

// dial returns the gRPC connection, dialing if needed.
func (s *Session) dial() (*grpc.ClientConn, error) {
	if s.conn != nil {
		return s.conn, nil
	}
	return grpc.NewClient(s.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// recvPump reads ServerEvents and publishes them to the events channel.
// It also updates CharacterState from CharacterInfo and CharacterSheet events.
func (s *Session) recvPump() {
	for {
		s.mu.RLock()
		stream := s.stream
		s.mu.RUnlock()
		if stream == nil {
			return
		}
		ev, err := stream.Recv()
		if err != nil {
			s.transitionDisconnected(err)
			close(s.events)
			return
		}
		s.updateCharacterState(ev)
		select {
		case s.events <- ev:
		default:
			// Drop if channel is full to avoid blocking the recv pump.
		}
	}
}

// updateCharacterState updates the stored CharacterState from CharacterInfo
// or CharacterSheetView events. Called from the recv pump.
func (s *Session) updateCharacterState(ev *gamev1.ServerEvent) {
	var cs *CharacterState
	switch p := ev.Payload.(type) {
	case *gamev1.ServerEvent_CharacterInfo:
		if p.CharacterInfo != nil {
			ci := p.CharacterInfo
			cs = &CharacterState{
				Name:      ci.GetName(),
				Level:     int(ci.GetLevel()),
				CurrentHP: int(ci.GetCurrentHp()),
				MaxHP:     int(ci.GetMaxHp()),
			}
		}
	case *gamev1.ServerEvent_CharacterSheet:
		if p.CharacterSheet != nil {
			sheet := p.CharacterSheet
			cs = &CharacterState{
				Name:       sheet.GetName(),
				Level:      int(sheet.GetLevel()),
				CurrentHP:  int(sheet.GetCurrentHp()),
				MaxHP:      int(sheet.GetMaxHp()),
				HeroPoints: int(sheet.GetHeroPoints()),
			}
		}
	}
	if cs != nil {
		s.mu.Lock()
		s.charState = cs
		s.mu.Unlock()
	}
}

func (s *Session) transitionDisconnected(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateDisconnected
	s.lastErr = err
	s.stream = nil
}

// reconnectBackoff attempts to reconnect with exponential backoff.
// Attempts: 2s, 4s, 8s. Transitions to StateDisconnected after the third failure.
// This is called by higher-level client code when the events channel is closed unexpectedly.
func (s *Session) reconnectBackoff(jwt string, characterID int64) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	for _, d := range delays {
		time.Sleep(d)
		s.mu.Lock()
		s.state = StateReconnecting
		s.mu.Unlock()
		s.events = make(chan *gamev1.ServerEvent, 64)
		if err := s.Connect(jwt, characterID); err == nil {
			return
		}
	}
	s.transitionDisconnected(fmt.Errorf("reconnect failed after %d attempts", len(delays)))
}

// ensure atomic is imported to avoid unused import error in builds that reference it
var _ = atomic.Value{}
```

- [ ] **Step 4: Remove unused atomic import**

The `atomic` import above was added speculatively — remove it. Edit `session.go`: delete the `"sync/atomic"` import line and the `var _ = atomic.Value{}` line at the bottom.

- [ ] **Step 5: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/client/session/... -v -count=1 -timeout=30s
```

Expected: all tests PASS.

- [ ] **Step 6: Run the full non-Docker test suite to confirm no regressions**

```bash
mise exec -- go test ./... -short -count=1 -timeout=120s 2>&1 | tail -20
```

Expected: all packages PASS (postgres packages will skip — that is expected with `-short`).

- [ ] **Step 7: Commit**

```bash
git add internal/client/session/session.go internal/client/session/session_test.go
git commit -m "feat(client/session): add gRPC session lifecycle and state machine"
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Task |
|---|---|
| REQ-IC-1: 100% coverage on exported functions | Tasks 2–5 test all exported methods |
| REQ-IC-2: auth tested with httptest.Server | Task 3 |
| REQ-IC-3: session tested with bufconn | Task 5 |
| REQ-IC-4: feed and history use rapid | Tasks 2 and 4 |
| REQ-IC-5: render has no test file | Task 1 (compile only) |
| render: ColorToken constants | Task 1 |
| render: FeedEntry, CharacterSnapshot | Task 1 |
| render: FeedRenderer, CharacterRenderer, ColorMapper | Task 1 |
| history: Push, Up, Down, Reset, cap=100 default | Task 2 |
| auth: Login, Register, Me, ListCharacters, CreateCharacter, CheckName, CharacterOptions | Task 3 |
| auth: ErrUnauthorized, ErrNameTaken, ErrValidation, ErrNetwork | Task 3 |
| feed: Append, Entries, Clear, DefaultTokenFor, cap=500 default, goroutine-safe | Task 4 |
| feed: token assignment table | Task 4 |
| feed: text extraction per event type | Task 4 |
| session: StateDisconnected/Authenticating/CharacterSelect/InGame/Reconnecting | Task 5 |
| session: Connect, Send, Events, State, Close | Task 5 |
| session: CharacterState updated from CharacterInfo/CharacterSheet | Task 5 |
| session: reconnect backoff (2s/4s/8s, 3 attempts) | Task 5 (`reconnectBackoff` implemented) |
| Ebiten spec: character creation now supported | Covered by auth.CreateCharacter + CharacterOptions in Task 3 |

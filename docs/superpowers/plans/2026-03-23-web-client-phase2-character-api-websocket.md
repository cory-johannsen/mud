# Web Client Phase 2: Character API & WebSocket Proxy

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Character CRUD API, WebSocket upgrade handler, and bidirectional gRPC proxy session.

**Architecture:** JWT-gated WS endpoint bridges browser JSON messages to gRPC proto stream; two goroutines per session with ping/pong keepalive.

**Tech Stack:** Go net/http, gorilla/websocket, protojson, pgx v5

---

## Prerequisites

Phase 1 MUST be complete before executing this plan:
- `cmd/webclient/` binary exists with HTTP server, auth middleware, and JWT helpers.
- `internal/config/config.go` has `WebConfig` with `Port` and `JWTSecret`.
- JWT claims carry `account_id int64`, `character_id int64`, and `role string`.
- `gorilla/websocket` added to `go.mod` (see Task 4, Step 1 for add command if absent).

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/webclient/handlers/characters.go` | Create | `GET /api/characters`, `POST /api/characters`, `GET /api/characters/options`, `GET /api/characters/check-name` |
| `cmd/webclient/handlers/characters_test.go` | Create | Unit tests for all four character endpoints |
| `cmd/webclient/session/session.go` | Create | `Session` struct: WS conn, gRPC stream, ping/pong loop, graceful shutdown |
| `cmd/webclient/handlers/websocket.go` | Create | `GET /ws` handler: JWT from query param or header, upgrade, open gRPC stream, spawn goroutines |
| `cmd/webclient/handlers/websocket_test.go` | Create | Integration tests for WebSocket handler using `httptest` + `gorilla/websocket` |

---

## Task 1: Character List and Get-By-ID Handlers (REQ-WC-12)

**Files:**
- Create: `cmd/webclient/handlers/characters.go`
- Create: `cmd/webclient/handlers/characters_test.go`

- [ ] **Step 1: Write failing test for GET /api/characters**

```go
// cmd/webclient/handlers/characters_test.go
package handlers_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/cmd/webclient/handlers"
    "github.com/cory-johannsen/mud/internal/game/character"
)

// stubCharacterRepo implements handlers.CharacterLister for tests.
type stubCharacterRepo struct {
    chars []*character.Character
    err   error
}

func (s *stubCharacterRepo) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
    return s.chars, s.err
}

func TestListCharacters_ReturnsOwnedCharacters(t *testing.T) {
    repo := &stubCharacterRepo{
        chars: []*character.Character{
            {
                ID: 1, Name: "Zork", Class: "ganger", Level: 5,
                CurrentHP: 38, MaxHP: 50, Region: "rustbucket", Team: "gun",
            },
        },
    }
    h := handlers.NewCharacterHandler(repo, nil, nil)

    req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()

    h.ListCharacters(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var resp []handlers.CharacterResponse
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    assert.Len(t, resp, 1)
    assert.Equal(t, int64(1), resp[0].ID)
    assert.Equal(t, "Zork", resp[0].Name)
    assert.Equal(t, "ganger", resp[0].Job)
    assert.Equal(t, 5, resp[0].Level)
    assert.Equal(t, int32(38), resp[0].CurrentHP)
    assert.Equal(t, int32(50), resp[0].MaxHP)
    assert.Equal(t, "rustbucket", resp[0].Region)
    assert.Equal(t, "gun", resp[0].Archetype)
}

func TestListCharacters_EmptyList(t *testing.T) {
    repo := &stubCharacterRepo{chars: []*character.Character{}}
    h := handlers.NewCharacterHandler(repo, nil, nil)

    req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
    req = req.WithContext(handlers.WithAccountID(req.Context(), 99))
    rr := httptest.NewRecorder()

    h.ListCharacters(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var resp []handlers.CharacterResponse
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    assert.Len(t, resp, 0)
}
```

Run (must fail): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestListCharacters`

- [ ] **Step 2: Implement CharacterHandler and ListCharacters**

```go
// cmd/webclient/handlers/characters.go
package handlers

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/cory-johannsen/mud/internal/game/character"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

// CharacterLister returns all characters for an account.
type CharacterLister interface {
    ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error)
}

// CharacterCreator creates a new character and returns it.
type CharacterCreator interface {
    Create(ctx context.Context, c *character.Character) (*character.Character, error)
}

// NameChecker reports whether a character name is available.
type NameChecker interface {
    // IsNameAvailable MUST return true if no character with that name exists.
    IsNameAvailable(ctx context.Context, name string) (bool, error)
}

// CharacterOptions holds ruleset data loaded at startup for the creation wizard.
type CharacterOptions struct {
    Regions    []*ruleset.Region
    Jobs       []*ruleset.Job
    Archetypes []*ruleset.Archetype
}

// CharacterResponse is the JSON shape returned for a single character.
type CharacterResponse struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Job       string `json:"job"`
    Level     int    `json:"level"`
    CurrentHP int32  `json:"current_hp"`
    MaxHP     int32  `json:"max_hp"`
    Region    string `json:"region"`
    Archetype string `json:"archetype"`
}

// CharacterHandler serves all /api/characters endpoints.
//
// Precondition: lister, creator, and checker MUST be non-nil for the
// endpoints that use them; options MUST be non-nil for ListOptions.
type CharacterHandler struct {
    lister  CharacterLister
    creator CharacterCreator
    checker NameChecker
    options *CharacterOptions
}

// NewCharacterHandler creates a CharacterHandler.
//
// Precondition: lister must be non-nil.
// Postcondition: Returns a ready CharacterHandler.
func NewCharacterHandler(lister CharacterLister, creator CharacterCreator, checker NameChecker) *CharacterHandler {
    return &CharacterHandler{lister: lister, creator: creator, checker: checker}
}

// WithOptions sets the ruleset options for the creation wizard endpoints.
//
// Postcondition: Returns the same handler with options set, enabling method chaining.
func (h *CharacterHandler) WithOptions(opts *CharacterOptions) *CharacterHandler {
    h.options = opts
    return h
}

// ListCharacters handles GET /api/characters.
//
// Precondition: Request context MUST carry account_id via WithAccountID.
// Postcondition: Writes JSON array of CharacterResponse; HTTP 500 on store error.
func (h *CharacterHandler) ListCharacters(w http.ResponseWriter, r *http.Request) {
    accountID := AccountIDFromContext(r.Context())
    chars, err := h.lister.ListByAccount(r.Context(), accountID)
    if err != nil {
        http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
        return
    }
    resp := make([]CharacterResponse, 0, len(chars))
    for _, c := range chars {
        resp = append(resp, characterToResponse(c))
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

// characterToResponse maps a Character domain object to its API response shape.
func characterToResponse(c *character.Character) CharacterResponse {
    return CharacterResponse{
        ID:        c.ID,
        Name:      c.Name,
        Job:       c.Class,
        Level:     c.Level,
        CurrentHP: int32(c.CurrentHP),
        MaxHP:     int32(c.MaxHP),
        Region:    c.Region,
        Archetype: c.Team,
    }
}
```

Run (must pass): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestListCharacters`

---

## Task 2: Character Creation Handler (REQ-WC-13)

**Files:**
- Modify: `cmd/webclient/handlers/characters.go` — add `CreateCharacter`
- Modify: `cmd/webclient/handlers/characters_test.go` — add creation tests

- [ ] **Step 1: Write failing tests for POST /api/characters**

```go
// cmd/webclient/handlers/characters_test.go (append)

type stubCreator struct {
    result *character.Character
    err    error
}

func (s *stubCreator) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
    return s.result, s.err
}

func TestCreateCharacter_Success(t *testing.T) {
    created := &character.Character{
        ID: 7, Name: "Mira", Class: "ganger", Team: "gun",
        Region: "rustbucket", Gender: "female", Level: 1,
        CurrentHP: 20, MaxHP: 20,
    }
    creator := &stubCreator{result: created}
    lister := &stubCharacterRepo{}
    h := handlers.NewCharacterHandler(lister, creator, nil)

    body := `{"name":"Mira","job":"ganger","archetype":"gun","region":"rustbucket","gender":"female"}`
    req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()

    h.CreateCharacter(rr, req)

    require.Equal(t, http.StatusCreated, rr.Code)
    var resp struct {
        Character handlers.CharacterResponse `json:"character"`
    }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    assert.Equal(t, int64(7), resp.Character.ID)
    assert.Equal(t, "Mira", resp.Character.Name)
}

func TestCreateCharacter_NameTooShort(t *testing.T) {
    h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
    body := `{"name":"ab","job":"ganger","archetype":"gun","region":"rustbucket","gender":"female"}`
    req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()
    h.CreateCharacter(rr, req)
    assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_NameTooLong(t *testing.T) {
    h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
    body := `{"name":"ThisNameIsWayTooLongForValidation","job":"ganger","archetype":"gun","region":"rustbucket","gender":"female"}`
    req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()
    h.CreateCharacter(rr, req)
    assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_MissingRequiredField(t *testing.T) {
    h := handlers.NewCharacterHandler(&stubCharacterRepo{}, &stubCreator{}, nil)
    // gender is omitted
    body := `{"name":"Mira","job":"ganger","archetype":"gun","region":"rustbucket"}`
    req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()
    h.CreateCharacter(rr, req)
    assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateCharacter_NameTaken(t *testing.T) {
    import_postgres "github.com/cory-johannsen/mud/internal/storage/postgres"
    creator := &stubCreator{err: import_postgres.ErrCharacterNameTaken}
    h := handlers.NewCharacterHandler(&stubCharacterRepo{}, creator, nil)
    body := `{"name":"Zork","job":"ganger","archetype":"gun","region":"rustbucket","gender":"male"}`
    req := httptest.NewRequest(http.MethodPost, "/api/characters", strings.NewReader(body))
    req = req.WithContext(handlers.WithAccountID(req.Context(), 42))
    rr := httptest.NewRecorder()
    h.CreateCharacter(rr, req)
    assert.Equal(t, http.StatusConflict, rr.Code)
}
```

Run (must fail): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestCreateCharacter`

- [ ] **Step 2: Implement CreateCharacter**

```go
// cmd/webclient/handlers/characters.go (append to existing file)

import (
    // add to existing imports:
    "errors"
    "strings"

    "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// createCharacterRequest is the JSON payload for POST /api/characters.
type createCharacterRequest struct {
    Name      string `json:"name"`
    Job       string `json:"job"`
    Archetype string `json:"archetype"`
    Region    string `json:"region"`
    Gender    string `json:"gender"`
}

// validate returns an error message if the request is invalid, or empty string if valid.
func (r createCharacterRequest) validate() string {
    n := strings.TrimSpace(r.Name)
    if len(n) < 3 || len(n) > 20 {
        return "name must be 3–20 characters"
    }
    if r.Job == "" {
        return "job is required"
    }
    if r.Archetype == "" {
        return "archetype is required"
    }
    if r.Region == "" {
        return "region is required"
    }
    if r.Gender == "" {
        return "gender is required"
    }
    return ""
}

// CreateCharacter handles POST /api/characters.
//
// Precondition: Request context MUST carry account_id; creator MUST be non-nil.
// Postcondition: Returns HTTP 201 with {"character": CharacterResponse} on success,
//   HTTP 400 on validation failure, HTTP 409 if name taken, HTTP 500 on store error.
func (h *CharacterHandler) CreateCharacter(w http.ResponseWriter, r *http.Request) {
    var req createCharacterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }
    if msg := req.validate(); msg != "" {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        _ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
        return
    }
    accountID := AccountIDFromContext(r.Context())
    c := &character.Character{
        AccountID: accountID,
        Name:      strings.TrimSpace(req.Name),
        Class:     req.Job,
        Team:      req.Archetype,
        Region:    req.Region,
        Gender:    req.Gender,
        Level:     1,
    }
    created, err := h.creator.Create(r.Context(), c)
    if err != nil {
        if errors.Is(err, postgres.ErrCharacterNameTaken) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusConflict)
            _ = json.NewEncoder(w).Encode(map[string]string{"error": "name already taken"})
            return
        }
        http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]any{"character": characterToResponse(created)})
}
```

Run (must pass): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestCreateCharacter`

---

## Task 3: Characters Options and Check-Name Endpoints (REQ-WC-14, REQ-WC-35)

**Files:**
- Modify: `cmd/webclient/handlers/characters.go` — add `ListOptions`, `CheckName`
- Modify: `cmd/webclient/handlers/characters_test.go` — add option/check-name tests

- [ ] **Step 1: Write failing tests**

```go
// cmd/webclient/handlers/characters_test.go (append)

import "strings"

type stubNameChecker struct {
    available bool
    err       error
}

func (s *stubNameChecker) IsNameAvailable(ctx context.Context, name string) (bool, error) {
    return s.available, s.err
}

func TestListOptions_ReturnsRulesetData(t *testing.T) {
    opts := &handlers.CharacterOptions{
        Regions:    []*ruleset.Region{{ID: "rustbucket", Name: "Rustbucket Ridge"}},
        Jobs:       []*ruleset.Job{{ID: "ganger", Name: "Ganger"}},
        Archetypes: []*ruleset.Archetype{{ID: "gun", Name: "Gun"}},
    }
    h := handlers.NewCharacterHandler(nil, nil, nil).WithOptions(opts)

    req := httptest.NewRequest(http.MethodGet, "/api/characters/options", nil)
    rr := httptest.NewRecorder()
    h.ListOptions(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var body map[string]any
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
    assert.Contains(t, body, "regions")
    assert.Contains(t, body, "jobs")
    assert.Contains(t, body, "archetypes")
}

func TestCheckName_Available(t *testing.T) {
    checker := &stubNameChecker{available: true}
    h := handlers.NewCharacterHandler(nil, nil, checker)

    req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name?name=Zork", nil)
    rr := httptest.NewRecorder()
    h.CheckName(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var body map[string]bool
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
    assert.True(t, body["available"])
}

func TestCheckName_Taken(t *testing.T) {
    checker := &stubNameChecker{available: false}
    h := handlers.NewCharacterHandler(nil, nil, checker)

    req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name?name=Zork", nil)
    rr := httptest.NewRecorder()
    h.CheckName(rr, req)

    require.Equal(t, http.StatusOK, rr.Code)
    var body map[string]bool
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
    assert.False(t, body["available"])
}

func TestCheckName_MissingParam(t *testing.T) {
    h := handlers.NewCharacterHandler(nil, nil, &stubNameChecker{})
    req := httptest.NewRequest(http.MethodGet, "/api/characters/check-name", nil)
    rr := httptest.NewRecorder()
    h.CheckName(rr, req)
    assert.Equal(t, http.StatusBadRequest, rr.Code)
}
```

Run (must fail): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestListOptions|TestCheckName`

- [ ] **Step 2: Implement ListOptions and CheckName**

```go
// cmd/webclient/handlers/characters.go (append)

// regionResponse is the API shape for a Region.
type regionResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

// jobResponse is the API shape for a Job.
type jobResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Archetype   string `json:"archetype"`
    KeyAbility  string `json:"key_ability"`
    HitPointsPerLevel int `json:"hit_points_per_level"`
}

// archetypeResponse is the API shape for an Archetype.
type archetypeResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

// ListOptions handles GET /api/characters/options.
//
// Precondition: h.options MUST be non-nil (set via WithOptions at startup).
// Postcondition: Returns JSON with regions, jobs, and archetypes arrays.
func (h *CharacterHandler) ListOptions(w http.ResponseWriter, r *http.Request) {
    if h.options == nil {
        http.Error(w, `{"error":"options not loaded"}`, http.StatusInternalServerError)
        return
    }
    regions := make([]regionResponse, 0, len(h.options.Regions))
    for _, reg := range h.options.Regions {
        regions = append(regions, regionResponse{
            ID:          reg.ID,
            Name:        reg.Name,
            Description: reg.Description,
        })
    }
    jobs := make([]jobResponse, 0, len(h.options.Jobs))
    for _, job := range h.options.Jobs {
        jobs = append(jobs, jobResponse{
            ID:                job.ID,
            Name:              job.Name,
            Description:       job.Description,
            Archetype:         job.Archetype,
            KeyAbility:        job.KeyAbility,
            HitPointsPerLevel: job.HitPointsPerLevel,
        })
    }
    archetypes := make([]archetypeResponse, 0, len(h.options.Archetypes))
    for _, arch := range h.options.Archetypes {
        archetypes = append(archetypes, archetypeResponse{
            ID:          arch.ID,
            Name:        arch.Name,
            Description: arch.Description,
        })
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]any{
        "regions":    regions,
        "jobs":       jobs,
        "archetypes": archetypes,
    })
}

// CheckName handles GET /api/characters/check-name?name=<value>.
//
// Precondition: h.checker MUST be non-nil.
// Postcondition: Returns {"available": bool}; HTTP 400 if name query param is absent.
func (h *CharacterHandler) CheckName(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("name")
    if strings.TrimSpace(name) == "" {
        http.Error(w, `{"error":"name query parameter is required"}`, http.StatusBadRequest)
        return
    }
    available, err := h.checker.IsNameAvailable(r.Context(), name)
    if err != nil {
        http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]bool{"available": available})
}
```

Add `IsNameAvailable` to `CharacterRepository` in `internal/storage/postgres/character.go`:

```go
// IsNameAvailable returns true if no character with the given name exists.
//
// Precondition: name must be non-empty.
// Postcondition: Returns true if name is unused across all accounts; false otherwise.
func (r *CharacterRepository) IsNameAvailable(ctx context.Context, name string) (bool, error) {
    var count int
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM characters WHERE lower(name) = lower($1)`, name,
    ).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("checking name availability: %w", err)
    }
    return count == 0, nil
}
```

Run (must pass): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestListOptions|TestCheckName`

Run full handler suite: `mise exec -- go test ./cmd/webclient/handlers/... -v`

---

## Task 4: WebSocket Session Struct (REQ-WC-18, REQ-WC-19)

**Files:**
- Create: `cmd/webclient/session/session.go`

- [ ] **Step 1: Add gorilla/websocket to go.mod if absent**

```bash
# Run from repo root:
mise exec -- go get github.com/gorilla/websocket
mise exec -- go mod vendor
```

Verify: `grep gorilla/websocket go.mod`

- [ ] **Step 2: Write failing test for Session ping/pong and shutdown**

```go
// cmd/webclient/session/session_test.go
package session_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/gorilla/websocket"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/cmd/webclient/session"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// echoServer upgrades, reads one message, echoes it, then waits for close.
func echoServer(t *testing.T) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        require.NoError(t, err)
        defer conn.Close()
        conn.SetPingHandler(func(appData string) error {
            return conn.WriteMessage(websocket.PongMessage, []byte(appData))
        })
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                return
            }
        }
    }))
}

func TestSession_CancelContextClosesSession(t *testing.T) {
    srv := echoServer(t)
    defer srv.Close()

    url := "ws" + strings.TrimPrefix(srv.URL, "http")
    wsConn, _, err := websocket.DefaultDialer.Dial(url, nil)
    require.NoError(t, err)

    ctx, cancel := context.WithCancel(context.Background())
    sess := session.New(ctx, cancel, wsConn, nil)

    done := make(chan struct{})
    go func() {
        sess.Wait()
        close(done)
    }()

    cancel() // trigger shutdown
    select {
    case <-done:
        // ok
    case <-time.After(2 * time.Second):
        t.Fatal("session did not shut down after context cancel")
    }
}

func TestSession_PingInterval(t *testing.T) {
    srv := echoServer(t)
    defer srv.Close()

    url := "ws" + strings.TrimPrefix(srv.URL, "http")
    wsConn, _, err := websocket.DefaultDialer.Dial(url, nil)
    require.NoError(t, err)

    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    sess := session.New(ctx, cancel, wsConn, nil)
    sess.SetPingInterval(50 * time.Millisecond)
    sess.SetPongTimeout(30 * time.Millisecond)

    // Session should run without error for a short period while pongs arrive.
    done := make(chan struct{})
    go func() {
        sess.Wait()
        close(done)
    }()

    select {
    case <-done:
        // ok — context expired cleanly
    case <-time.After(500 * time.Millisecond):
        t.Fatal("session did not exit after context timeout")
    }
    assert.NoError(t, sess.Err())
}
```

Run (must fail): `mise exec -- go test ./cmd/webclient/session/... -v -run TestSession`

- [ ] **Step 3: Implement session.Session**

```go
// cmd/webclient/session/session.go
package session

import (
    "context"
    "sync"
    "time"

    "github.com/gorilla/websocket"

    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const (
    defaultPingInterval = 30 * time.Second
    defaultPongTimeout  = 10 * time.Second
)

// Session manages a single player's WebSocket connection and optional gRPC stream.
//
// Precondition: wsConn must be a valid, open WebSocket connection.
// Postcondition: Session is alive until ctx is cancelled, WS closes, or gRPC stream closes.
type Session struct {
    ctx         context.Context
    cancel      context.CancelFunc
    wsConn      *websocket.Conn
    stream      gamev1.GameService_SessionClient // may be nil in tests
    pingInterval time.Duration
    pongTimeout  time.Duration
    wg           sync.WaitGroup
    err          error
    errMu        sync.Mutex
}

// New creates a Session.
//
// Precondition: ctx and cancel must correspond; wsConn must be non-nil.
// Postcondition: Returns a Session ready to have goroutines launched via Run().
func New(ctx context.Context, cancel context.CancelFunc, wsConn *websocket.Conn, stream gamev1.GameService_SessionClient) *Session {
    return &Session{
        ctx:          ctx,
        cancel:       cancel,
        wsConn:       wsConn,
        stream:       stream,
        pingInterval: defaultPingInterval,
        pongTimeout:  defaultPongTimeout,
    }
}

// SetPingInterval overrides the default 30s ping interval (used in tests).
func (s *Session) SetPingInterval(d time.Duration) { s.pingInterval = d }

// SetPongTimeout overrides the default 10s pong timeout (used in tests).
func (s *Session) SetPongTimeout(d time.Duration) { s.pongTimeout = d }

// Run starts the ping/pong keepalive loop.
// Callers MUST also start WS-to-gRPC and gRPC-to-WS goroutines separately
// (done by the WebSocket handler after calling Run).
//
// Postcondition: goroutine launched; call Wait() to block until session ends.
func (s *Session) Run() {
    s.wsConn.SetPongHandler(func(string) error {
        return s.wsConn.SetReadDeadline(time.Time{}) // reset deadline on pong
    })
    s.wg.Add(1)
    go s.pingLoop()
}

// Wait blocks until the session has fully stopped.
func (s *Session) Wait() { s.wg.Wait() }

// Err returns the first non-nil error that caused the session to stop.
// MUST be called after Wait() returns.
func (s *Session) Err() error {
    s.errMu.Lock()
    defer s.errMu.Unlock()
    return s.err
}

// Close sends a WebSocket close frame with the given code and cancels the session context.
//
// Postcondition: Context is cancelled; WS close message sent best-effort.
func (s *Session) Close(code int, text string) {
    _ = s.wsConn.WriteMessage(
        websocket.CloseMessage,
        websocket.FormatCloseMessage(code, text),
    )
    s.cancel()
}

func (s *Session) setErr(err error) {
    s.errMu.Lock()
    defer s.errMu.Unlock()
    if s.err == nil {
        s.err = err
    }
}

// pingLoop sends a ping every s.pingInterval. If no pong is received within
// s.pongTimeout the session is closed.
func (s *Session) pingLoop() {
    defer s.wg.Done()
    ticker := time.NewTicker(s.pingInterval)
    defer ticker.Stop()
    for {
        select {
        case <-s.ctx.Done():
            _ = s.wsConn.Close()
            return
        case <-ticker.C:
            deadline := time.Now().Add(s.pongTimeout)
            if err := s.wsConn.SetWriteDeadline(deadline); err != nil {
                s.setErr(err)
                s.cancel()
                return
            }
            if err := s.wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
                s.setErr(err)
                s.cancel()
                return
            }
            // Pong handler resets the read deadline; set a deadline so we detect
            // a missing pong by the time the next ping is due.
            if err := s.wsConn.SetReadDeadline(deadline); err != nil {
                s.setErr(err)
                s.cancel()
                return
            }
        }
    }
}
```

Run (must pass): `mise exec -- go test ./cmd/webclient/session/... -v -run TestSession`

---

## Task 5: WebSocket Handler — JWT Validation, Upgrade, gRPC Goroutines (REQ-WC-15, REQ-WC-16, REQ-WC-17, REQ-WC-19, REQ-WC-20, REQ-WC-30)

**Files:**
- Create: `cmd/webclient/handlers/websocket.go`
- Create: `cmd/webclient/handlers/websocket_test.go`

- [ ] **Step 1: Write failing test for WS handler JWT rejection**

```go
// cmd/webclient/handlers/websocket_test.go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gorilla/websocket"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/cmd/webclient/handlers"
)

func TestWSHandler_RejectsInvalidJWT(t *testing.T) {
    h := handlers.NewWSHandler("test-secret", nil, nil)
    srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
    defer srv.Close()

    url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=not-a-valid-jwt"
    _, resp, err := websocket.DefaultDialer.Dial(url, nil)
    require.Error(t, err)
    assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWSHandler_RejectsMissingToken(t *testing.T) {
    h := handlers.NewWSHandler("test-secret", nil, nil)
    srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
    defer srv.Close()

    url := "ws" + strings.TrimPrefix(srv.URL, "http")
    _, resp, err := websocket.DefaultDialer.Dial(url, nil)
    require.Error(t, err)
    assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
```

Run (must fail): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestWSHandler`

- [ ] **Step 2: Implement WSHandler**

```go
// cmd/webclient/handlers/websocket.go
package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"

    "github.com/gorilla/websocket"
    "go.uber.org/zap"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/encoding/protojson"
    "google.golang.org/protobuf/proto"

    "github.com/cory-johannsen/mud/cmd/webclient/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var wsUpgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    CheckOrigin:     func(r *http.Request) bool { return true },
}

// GameDialer opens a gRPC connection and returns a Session stream.
type GameDialer interface {
    Session(ctx context.Context, opts ...grpc.CallOption) (gamev1.GameService_SessionClient, error)
}

// CharacterGetter loads a character by ID.
type CharacterGetter interface {
    GetByID(ctx context.Context, id int64) (*character.Character, error)
}

// WSHandler handles GET /ws.
//
// Precondition: jwtSecret must be the same secret used when issuing tokens.
// Precondition: dialer must be a valid connected GameServiceClient (set at startup).
type WSHandler struct {
    jwtSecret string
    dialer    GameDialer
    charGetter CharacterGetter
    logger    *zap.Logger
    wsMu      sync.Mutex // guards writes to individual WS conns
}

// NewWSHandler creates a WSHandler.
//
// Precondition: jwtSecret must be non-empty; logger may be nil (falls back to zap.NewNop).
func NewWSHandler(jwtSecret string, dialer GameDialer, charGetter CharacterGetter) *WSHandler {
    return &WSHandler{
        jwtSecret:  jwtSecret,
        dialer:     dialer,
        charGetter: charGetter,
        logger:     zap.NewNop(),
    }
}

// WithLogger attaches a logger to the handler.
func (h *WSHandler) WithLogger(l *zap.Logger) *WSHandler {
    h.logger = l
    return h
}

// wsMessage is the JSON envelope for all WebSocket frames.
type wsMessage struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// ServeHTTP implements http.Handler for GET /ws.
//
// Postcondition: Upgrades to WebSocket on valid JWT; returns 401 otherwise.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    claims, err := extractAndValidateJWT(r, h.jwtSecret)
    if err != nil {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    characterID := claims.CharacterID
    if characterID == 0 {
        http.Error(w, `{"error":"character_id claim missing"}`, http.StatusUnauthorized)
        return
    }

    char, err := h.charGetter.GetByID(r.Context(), characterID)
    if err != nil {
        http.Error(w, `{"error":"character not found"}`, http.StatusUnauthorized)
        return
    }

    // Verify ownership: JWT account_id must match character's account_id.
    if char.AccountID != claims.AccountID {
        http.Error(w, `{"error":"character not owned by account"}`, http.StatusUnauthorized)
        return
    }

    wsConn, err := wsUpgrader.Upgrade(w, r, nil)
    if err != nil {
        h.logger.Error("websocket upgrade failed", zap.Error(err))
        return
    }

    ctx, cancel := context.WithCancel(context.Background())

    stream, err := h.dialer.Session(ctx)
    if err != nil {
        cancel()
        _ = wsConn.Close()
        h.logger.Error("failed to open gRPC session stream", zap.Error(err))
        return
    }

    // Send JoinWorldRequest with character metadata.
    joinMsg := &gamev1.ClientMessage{
        RequestId: "join-0",
        Payload: &gamev1.ClientMessage_JoinWorld{
            JoinWorld: &gamev1.JoinWorldRequest{
                CharacterId: fmt.Sprintf("%d", char.ID),
                Name:        char.Name,
                Class:       char.Class,
                Team:        char.Team,
                Region:      char.Region,
                Gender:      char.Gender,
            },
        },
    }
    if err := stream.Send(joinMsg); err != nil {
        cancel()
        _ = wsConn.Close()
        h.logger.Error("failed to send JoinWorldRequest", zap.Error(err))
        return
    }

    sess := session.New(ctx, cancel, wsConn, stream)
    sess.Run()

    var wg sync.WaitGroup
    wg.Add(2)

    // WS → gRPC goroutine.
    go func() {
        defer wg.Done()
        defer sess.Close(websocket.CloseNormalClosure, "")
        h.wsToGRPC(ctx, wsConn, stream)
    }()

    // gRPC → WS goroutine.
    go func() {
        defer wg.Done()
        defer sess.Close(websocket.CloseGoingAway, "server disconnected")
        h.grpcToWS(ctx, stream, wsConn)
    }()

    wg.Wait()
    sess.Wait()
}

// wsToGRPC reads JSON frames from the WebSocket and forwards them as ClientMessage protos.
//
// Postcondition: Returns when the WebSocket is closed or context is cancelled.
func (h *WSHandler) wsToGRPC(ctx context.Context, wsConn *websocket.Conn, stream gamev1.GameService_SessionClient) {
    registry := command.DefaultRegistry()
    requestID := 0
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        _, data, err := wsConn.ReadMessage()
        if err != nil {
            return
        }
        var env wsMessage
        if err := json.Unmarshal(data, &env); err != nil {
            h.logger.Warn("invalid ws message envelope", zap.Error(err))
            continue
        }
        requestID++
        reqID := fmt.Sprintf("ws-%d", requestID)

        msg, err := dispatchWSMessage(env, reqID, registry)
        if err != nil {
            h.logger.Warn("dispatch failed", zap.String("type", env.Type), zap.Error(err))
            continue
        }
        if msg == nil {
            continue
        }
        if err := stream.Send(msg); err != nil {
            return
        }
    }
}

// grpcToWS reads ServerEvent protos from the gRPC stream and writes JSON frames to the WS.
//
// Postcondition: Returns when the gRPC stream is closed.
func (h *WSHandler) grpcToWS(ctx context.Context, stream gamev1.GameService_SessionClient, wsConn *websocket.Conn) {
    marshaler := protojson.MarshalOptions{EmitUnpopulated: false}
    for {
        event, err := stream.Recv()
        if err != nil {
            return
        }
        msgName := string(proto.MessageName(event).Name())
        payload, err := marshaler.Marshal(event)
        if err != nil {
            h.logger.Error("failed to marshal ServerEvent", zap.Error(err))
            continue
        }
        env := wsMessage{
            Type:    msgName,
            Payload: json.RawMessage(payload),
        }
        frame, err := json.Marshal(env)
        if err != nil {
            h.logger.Error("failed to marshal ws envelope", zap.Error(err))
            continue
        }
        if err := wsConn.WriteMessage(websocket.TextMessage, frame); err != nil {
            return
        }
    }
}

// dispatchWSMessage converts a wsMessage envelope into a ClientMessage proto (REQ-WC-30).
// It accepts two forms:
//   - {"type": "CommandText", "payload": {"text": "move north"}} — raw command string
//   - {"type": "MoveRequest", "payload": {"direction": "north"}}  — direct proto dispatch
//
// Postcondition: Returns a non-nil ClientMessage or nil if the message should be skipped.
func dispatchWSMessage(env wsMessage, reqID string, registry *command.Registry) (*gamev1.ClientMessage, error) {
    // Raw text command form: re-use existing command registry (REQ-WC-30).
    if env.Type == "CommandText" {
        var body struct {
            Text string `json:"text"`
        }
        if err := json.Unmarshal(env.Payload, &body); err != nil {
            return nil, fmt.Errorf("parsing CommandText payload: %w", err)
        }
        return buildClientMessageFromText(reqID, body.Text, registry)
    }
    // Direct proto dispatch form — marshal payload into the named proto message.
    msg, err := protoMessageByName(env.Type)
    if err != nil {
        return nil, fmt.Errorf("unknown message type %q: %w", env.Type, err)
    }
    if err := protojson.Unmarshal(env.Payload, msg); err != nil {
        return nil, fmt.Errorf("unmarshalling %q: %w", env.Type, err)
    }
    return wrapProtoAsClientMessage(reqID, env.Type, msg)
}
```

- [ ] **Step 3: Add helper functions for proto dispatch**

```go
// cmd/webclient/handlers/websocket_dispatch.go
package handlers

import (
    "fmt"
    "strings"

    "google.golang.org/protobuf/proto"

    "github.com/cory-johannsen/mud/internal/game/command"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildClientMessageFromText converts a raw text command (e.g. "move north") to
// a ClientMessage using the same command registry as the telnet frontend (REQ-WC-30).
//
// Postcondition: Returns a ClientMessage or an error if the command is unrecognised.
func buildClientMessageFromText(reqID, text string, registry *command.Registry) (*gamev1.ClientMessage, error) {
    parsed := command.Parse(strings.TrimSpace(text))
    if parsed.Command == "" {
        return nil, nil
    }
    cmd, ok := registry.Resolve(parsed.Command)
    if !ok {
        // Treat as a move attempt (custom exit names).
        return buildMoveClientMessage(reqID, parsed.Command), nil
    }
    bctx := &webBridgeContext{reqID: reqID, cmd: cmd, parsed: parsed}
    return buildMessageFromCommand(bctx)
}

// buildMoveClientMessage creates a MoveRequest ClientMessage for an exit name.
func buildMoveClientMessage(reqID, direction string) *gamev1.ClientMessage {
    return &gamev1.ClientMessage{
        RequestId: reqID,
        Payload: &gamev1.ClientMessage_Move{
            Move: &gamev1.MoveRequest{Direction: direction},
        },
    }
}

// webBridgeContext is a minimal stand-in for the telnet bridgeContext, used to
// call the shared command-to-proto mapping without a telnet.Conn dependency.
type webBridgeContext struct {
    reqID  string
    cmd    *command.Command
    parsed command.ParseResult
}

// buildMessageFromCommand maps a resolved Command to a ClientMessage proto.
// This function covers the same handler cases as bridgeHandlerMap without any
// telnet.Conn interaction — pure proto construction only.
//
// Postcondition: Returns a non-nil ClientMessage for supported handlers.
// Returns an error for handlers that require interactive telnet output only.
func buildMessageFromCommand(bctx *webBridgeContext) (*gamev1.ClientMessage, error) {
    reqID := bctx.reqID
    parsed := bctx.parsed
    arg := ""
    if len(parsed.Args) > 0 {
        arg = parsed.Args[0]
    }
    rawArgs := parsed.RawArgs

    switch bctx.cmd.Handler {
    case command.HandlerMove:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Move{Move: &gamev1.MoveRequest{Direction: parsed.Command}}}, nil
    case command.HandlerLook:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}}}, nil
    case command.HandlerSay:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: rawArgs}}}, nil
    case command.HandlerEmote:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Message: rawArgs}}}, nil
    case command.HandlerExits:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}}}, nil
    case command.HandlerWho:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}}}, nil
    case command.HandlerQuit:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}}}, nil
    case command.HandlerExamine:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: rawArgs}}}, nil
    case command.HandlerAttack:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: arg}}}, nil
    case command.HandlerFlee:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}}}, nil
    case command.HandlerPass:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}}}, nil
    case command.HandlerStrike:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: arg}}}, nil
    case command.HandlerStatus:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}}}, nil
    case command.HandlerInventory:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_InventoryReq{InventoryReq: &gamev1.InventoryRequest{}}}, nil
    case command.HandlerMap:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Map{Map: &gamev1.MapRequest{}}}, nil
    case command.HandlerSkills:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_SkillsRequest{SkillsRequest: &gamev1.SkillsRequest{}}}, nil
    case command.HandlerFeats:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_FeatsRequest{FeatsRequest: &gamev1.FeatsRequest{}}}, nil
    case command.HandlerCharSheet:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_CharSheet{CharSheet: &gamev1.CharacterSheetRequest{}}}, nil
    case command.HandlerRest:
        return &gamev1.ClientMessage{RequestId: reqID,
            Payload: &gamev1.ClientMessage_Rest{Rest: &gamev1.RestRequest{}}}, nil
    default:
        return nil, fmt.Errorf("handler %q not supported in web client dispatch", bctx.cmd.Handler)
    }
}

// protoMessageByName returns an empty proto.Message for a given short message name.
// The name is the proto message name without the package prefix (e.g. "MoveRequest").
//
// Postcondition: Returns a non-nil proto.Message or error if name is unknown.
func protoMessageByName(name string) (proto.Message, error) {
    typeMap := map[string]func() proto.Message{
        "MoveRequest":      func() proto.Message { return &gamev1.MoveRequest{} },
        "LookRequest":      func() proto.Message { return &gamev1.LookRequest{} },
        "SayRequest":       func() proto.Message { return &gamev1.SayRequest{} },
        "EmoteRequest":     func() proto.Message { return &gamev1.EmoteRequest{} },
        "AttackRequest":    func() proto.Message { return &gamev1.AttackRequest{} },
        "FleeRequest":      func() proto.Message { return &gamev1.FleeRequest{} },
        "ExamineRequest":   func() proto.Message { return &gamev1.ExamineRequest{} },
        "ExitsRequest":     func() proto.Message { return &gamev1.ExitsRequest{} },
        "WhoRequest":       func() proto.Message { return &gamev1.WhoRequest{} },
        "QuitRequest":      func() proto.Message { return &gamev1.QuitRequest{} },
        "PassRequest":      func() proto.Message { return &gamev1.PassRequest{} },
        "StrikeRequest":    func() proto.Message { return &gamev1.StrikeRequest{} },
        "StatusRequest":    func() proto.Message { return &gamev1.StatusRequest{} },
        "InventoryRequest": func() proto.Message { return &gamev1.InventoryRequest{} },
        "MapRequest":       func() proto.Message { return &gamev1.MapRequest{} },
        "SkillsRequest":    func() proto.Message { return &gamev1.SkillsRequest{} },
        "FeatsRequest":     func() proto.Message { return &gamev1.FeatsRequest{} },
        "CharacterSheetRequest": func() proto.Message { return &gamev1.CharacterSheetRequest{} },
        "RestRequest":      func() proto.Message { return &gamev1.RestRequest{} },
    }
    factory, ok := typeMap[name]
    if !ok {
        return nil, fmt.Errorf("unknown proto message name: %q", name)
    }
    return factory(), nil
}

// wrapProtoAsClientMessage wraps a fully populated proto.Message in a ClientMessage oneof.
//
// Postcondition: Returns a non-nil ClientMessage or error for unrecognised types.
func wrapProtoAsClientMessage(reqID, typeName string, msg proto.Message) (*gamev1.ClientMessage, error) {
    cm := &gamev1.ClientMessage{RequestId: reqID}
    switch m := msg.(type) {
    case *gamev1.MoveRequest:
        cm.Payload = &gamev1.ClientMessage_Move{Move: m}
    case *gamev1.LookRequest:
        cm.Payload = &gamev1.ClientMessage_Look{Look: m}
    case *gamev1.SayRequest:
        cm.Payload = &gamev1.ClientMessage_Say{Say: m}
    case *gamev1.EmoteRequest:
        cm.Payload = &gamev1.ClientMessage_Emote{Emote: m}
    case *gamev1.AttackRequest:
        cm.Payload = &gamev1.ClientMessage_Attack{Attack: m}
    case *gamev1.FleeRequest:
        cm.Payload = &gamev1.ClientMessage_Flee{Flee: m}
    case *gamev1.ExamineRequest:
        cm.Payload = &gamev1.ClientMessage_Examine{Examine: m}
    case *gamev1.ExitsRequest:
        cm.Payload = &gamev1.ClientMessage_Exits{Exits: m}
    case *gamev1.WhoRequest:
        cm.Payload = &gamev1.ClientMessage_Who{Who: m}
    case *gamev1.QuitRequest:
        cm.Payload = &gamev1.ClientMessage_Quit{Quit: m}
    case *gamev1.PassRequest:
        cm.Payload = &gamev1.ClientMessage_Pass{Pass: m}
    case *gamev1.StrikeRequest:
        cm.Payload = &gamev1.ClientMessage_Strike{Strike: m}
    case *gamev1.StatusRequest:
        cm.Payload = &gamev1.ClientMessage_Status{Status: m}
    case *gamev1.InventoryRequest:
        cm.Payload = &gamev1.ClientMessage_InventoryReq{InventoryReq: m}
    case *gamev1.MapRequest:
        cm.Payload = &gamev1.ClientMessage_Map{Map: m}
    case *gamev1.SkillsRequest:
        cm.Payload = &gamev1.ClientMessage_SkillsRequest{SkillsRequest: m}
    case *gamev1.FeatsRequest:
        cm.Payload = &gamev1.ClientMessage_FeatsRequest{FeatsRequest: m}
    case *gamev1.CharacterSheetRequest:
        cm.Payload = &gamev1.ClientMessage_CharSheet{CharSheet: m}
    case *gamev1.RestRequest:
        cm.Payload = &gamev1.ClientMessage_Rest{Rest: m}
    default:
        return nil, fmt.Errorf("no ClientMessage oneof for type %q", typeName)
    }
    return cm, nil
}
```

- [ ] **Step 4: Write dispatch unit tests**

```go
// cmd/webclient/handlers/websocket_dispatch_test.go
package handlers_test

import (
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/cmd/webclient/handlers"
    "github.com/cory-johannsen/mud/internal/game/command"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestDispatchWSMessage_CommandText_Move(t *testing.T) {
    env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "north"})
    registry := command.DefaultRegistry()
    msg, err := handlers.DispatchWSMessageForTest(env, "req-1", registry)
    require.NoError(t, err)
    require.NotNil(t, msg)
    move := msg.GetMove()
    require.NotNil(t, move)
    assert.Equal(t, "north", move.Direction)
}

func TestDispatchWSMessage_CommandText_Say(t *testing.T) {
    env := handlers.WSMessageForTest("CommandText", map[string]string{"text": "say Hello world"})
    registry := command.DefaultRegistry()
    msg, err := handlers.DispatchWSMessageForTest(env, "req-2", registry)
    require.NoError(t, err)
    require.NotNil(t, msg)
    say := msg.GetSay()
    require.NotNil(t, say)
    assert.Equal(t, "Hello world", say.Message)
}

func TestDispatchWSMessage_DirectProto_MoveRequest(t *testing.T) {
    env := handlers.WSMessageForTest("MoveRequest", map[string]string{"direction": "south"})
    registry := command.DefaultRegistry()
    msg, err := handlers.DispatchWSMessageForTest(env, "req-3", registry)
    require.NoError(t, err)
    require.NotNil(t, msg)
    assert.Equal(t, "south", msg.GetMove().GetDirection())
}

func TestDispatchWSMessage_UnknownType_ReturnsError(t *testing.T) {
    env := handlers.WSMessageForTest("BogusRequest", map[string]string{})
    registry := command.DefaultRegistry()
    _, err := handlers.DispatchWSMessageForTest(env, "req-4", registry)
    assert.Error(t, err)
}
```

Add test-export shims (unexported functions exposed via `_test.go` export file):

```go
// cmd/webclient/handlers/export_test.go
package handlers

import "github.com/cory-johannsen/mud/internal/game/command"

// WSMessageForTest constructs a wsMessage for use in tests.
func WSMessageForTest(msgType string, payload any) wsMessage {
    raw, _ := json.Marshal(payload)
    return wsMessage{Type: msgType, Payload: raw}
}

// DispatchWSMessageForTest exposes dispatchWSMessage for unit tests.
func DispatchWSMessageForTest(env wsMessage, reqID string, registry *command.Registry) (*gamev1.ClientMessage, error) {
    return dispatchWSMessage(env, reqID, registry)
}
```

Run (must pass): `mise exec -- go test ./cmd/webclient/handlers/... -v -run TestDispatch`

- [ ] **Step 5: Run full Phase 2 test suite**

```bash
mise exec -- go test ./cmd/webclient/... ./cmd/webclient/session/... -v
```

All tests MUST pass with 0 failures before this task is marked complete.

---

## Context Helpers (used across Tasks 1–5)

These functions MUST exist in `cmd/webclient/handlers/context.go` (created in Phase 1). If absent, add them now:

```go
// cmd/webclient/handlers/context.go
package handlers

import "context"

type contextKey int

const (
    accountIDKey  contextKey = iota
    characterIDKey
    roleKey
)

// WithAccountID stores an account ID in the context.
func WithAccountID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, accountIDKey, id)
}

// AccountIDFromContext retrieves the account ID from the context.
// Returns 0 if not set.
func AccountIDFromContext(ctx context.Context) int64 {
    v, _ := ctx.Value(accountIDKey).(int64)
    return v
}

// WithCharacterID stores a character ID in the context.
func WithCharacterID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, characterIDKey, id)
}

// CharacterIDFromContext retrieves the character ID from the context.
func CharacterIDFromContext(ctx context.Context) int64 {
    v, _ := ctx.Value(characterIDKey).(int64)
    return v
}
```

The JWT helper `extractAndValidateJWT` MUST also be defined (Phase 1). It MUST return a claims struct that includes `AccountID int64`, `CharacterID int64`, and `Role string`. The function MUST accept both `Authorization: Bearer <token>` header and `?token=<JWT>` query parameter.

---

## Wire-Up in main.go (after all tasks pass)

Add to `cmd/webclient/main.go` router setup:

```go
// In the HTTP mux setup block:

charHandler := handlers.NewCharacterHandler(charRepo, charRepo, charRepo).
    WithOptions(&handlers.CharacterOptions{
        Regions:    regions,    // loaded via ruleset.LoadRegions at startup
        Jobs:       jobs,       // loaded via ruleset.LoadJobs at startup
        Archetypes: archetypes, // loaded via ruleset.LoadArchetypes at startup
    })

wsHandler := handlers.NewWSHandler(cfg.Web.JWTSecret, gameClient, charRepo).
    WithLogger(logger)

mux.Handle("GET /api/characters", authMiddleware(http.HandlerFunc(charHandler.ListCharacters)))
mux.Handle("POST /api/characters", authMiddleware(http.HandlerFunc(charHandler.CreateCharacter)))
mux.Handle("GET /api/characters/options", authMiddleware(http.HandlerFunc(charHandler.ListOptions)))
mux.Handle("GET /api/characters/check-name", authMiddleware(http.HandlerFunc(charHandler.CheckName)))
mux.Handle("GET /ws", http.HandlerFunc(wsHandler.ServeHTTP))
```

Note: `/ws` does its own JWT validation (accepts query param token) so it MUST NOT be wrapped by `authMiddleware`.

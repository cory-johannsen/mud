# internal/client Architecture Spec

**Date:** 2026-03-26
**Status:** approved
**Scope:** Shared client library consumed by `cmd/webclient` and `cmd/ebitenclient`

---

## Overview

`internal/client` is a set of layered sub-packages providing the shared protocol, state, and rendering-contract layer for all game client binaries. It eliminates duplication between the web client and Ebiten client across six concerns: authentication, session lifecycle, message feed, character state, command history, and rendering contracts.

The package is Go-only. React/TypeScript rendering in `cmd/webclient/ui` is informed by the shared type shapes but does not import this package directly. The Ebiten client (`cmd/ebitenclient`) and the Go HTTP layer of `cmd/webclient` both import sub-packages as needed.

---

## Package Structure

```
internal/client/
  auth/       — HTTP client: login, register, character list, character creation, name check
  session/    — gRPC session lifecycle + session state machine
  feed/       — ServerEvent accumulation, color token assignment, cap enforcement
  history/    — command ring buffer (↑/↓ navigation, in-memory only)
  render/     — renderer interfaces + color token constants
```

**Dependency rule:** Sub-packages have one permitted dependency direction: `feed` and `session` may import `render` (for `ColorToken`), but no other cross-sub-package imports are allowed. All other coupling flows through the client binary's wiring layer. Each sub-package otherwise imports only `internal/gameserver/gamev1` (proto types) and the Go standard library, with the exception of `session/`, which additionally accepts `internal/command/parse.go` as an injected function.

The `render` package interfaces use locally-defined parameter types (not `session.State` or `feed.Entry`) to avoid import cycles — each client binary wires the concrete types together at the call site.

---

## Sub-Package Specifications

### `internal/client/render`

Defines the rendering contract interfaces and color token constants. Contains no rendering logic.

#### Color Tokens

```go
type ColorToken int

const (
    ColorDefault    ColorToken = iota
    ColorCombat              // CombatEvent, RoundStartEvent, RoundEndEvent
    ColorSpeech              // MessageEvent (say/emote)
    ColorRoomEvent           // arrival/departure events
    ColorSystem              // system messages, unclassified events
    ColorError               // ErrorEvent
    ColorStructured          // CharacterInfo, InventoryView, CharacterSheetView
)
```

Each client maps tokens to its native representation:
- **Ebiten:** `color.RGBA` values loaded from `colours.yaml` in the asset pack; unknown tokens fall back to `ColorDefault`
- **Web (React):** CSS class names (e.g. `feed-combat`, `feed-speech`)
- **Telnet (future):** ANSI escape sequences

#### Interfaces

```go
// FeedEntry is a locally-defined mirror of feed.Entry, avoiding an import cycle.
type FeedEntry struct {
    Timestamp time.Time
    Token     ColorToken
    Text      string
}

// CharacterSnapshot is a locally-defined mirror of session.CharacterState.
type CharacterSnapshot struct {
    Name       string
    Level      int
    CurrentHP  int
    MaxHP      int
    Conditions []string
    HeroPoints int
    AP         int
}

// FeedRenderer renders a slice of feed entries.
type FeedRenderer interface {
    RenderFeed(entries []FeedEntry) error
}

// CharacterRenderer renders the character panel state.
type CharacterRenderer interface {
    RenderCharacter(snap CharacterSnapshot) error
}

// ColorMapper maps a ColorToken to a client-native value T (CSS class, RGBA, ANSI code).
type ColorMapper[T any] interface {
    Map(token ColorToken) T
}
```

Client binaries convert `feed.Entry` → `render.FeedEntry` and `session.CharacterState` → `render.CharacterSnapshot` at the call site. The fields are identical; the copy is intentional to preserve the acyclic package graph.

---

### `internal/client/session`

Manages the gRPC `GameService.Session` bidirectional stream and the client-side session state machine.

#### State Machine

```
Disconnected ──connect──► Authenticating ──success──► CharacterSelect
                               │                            │
                           failure                      selected
                               │                            │
                          Disconnected               InGame ──lost──► Reconnecting
                                                                           │
                                                               ──success──► InGame
                                                               ──failure──► Disconnected
```

Reconnect uses exponential backoff: 3 attempts at 2s, 4s, 8s delays. After the third failure the state machine transitions to `Disconnected` with the terminal error stored in `State.Error`.

#### Types

```go
type SessionState int

const (
    StateDisconnected   SessionState = iota
    StateAuthenticating
    StateCharacterSelect
    StateInGame
    StateReconnecting
)

type CharacterState struct {
    Name       string
    Level      int
    CurrentHP  int
    MaxHP      int
    Conditions []string
    HeroPoints int
    AP         int
}

type State struct {
    Current   SessionState
    Character *CharacterState // non-nil when StateInGame
    Error     error           // last terminal error, non-nil when StateDisconnected after failure
}
```

#### API

```go
// New creates a Session. cmdParser is internal/command/parse.go's function,
// injected to avoid a direct dependency on internal/game/command.
func New(grpcAddr string, cmdParser func(string) (*gamev1.ClientMessage, error)) *Session

// Connect opens the gRPC stream, sends JoinWorldRequest, transitions to StateInGame.
// Returns ErrAlreadyConnected if Current != StateDisconnected.
func (s *Session) Connect(jwt string, characterID int64) error

// Send parses cmd via cmdParser and calls stream.Send. Blocks until sent or error.
func (s *Session) Send(cmd string) error

// Events returns a channel of ServerEvents received from the gRPC stream.
// The channel is closed when the session transitions to StateDisconnected.
func (s *Session) Events() <-chan *gamev1.ServerEvent

// State returns a snapshot of the current session state.
func (s *Session) State() State

// Close gracefully shuts down the gRPC stream (calls CloseSend, drains Events).
func (s *Session) Close() error
```

Two goroutines run internally per session:
- **Recv pump:** reads `ServerEvent` from the gRPC stream, publishes to `Events()` channel, updates `CharacterState` from `CharacterInfo` / `CharacterSheetView` events.
- **Send queue:** serializes `stream.Send` calls from potentially concurrent callers.

`CharacterState` is updated automatically by the recv pump whenever a `CharacterInfo` or `CharacterSheetView` `ServerEvent` is received — callers do not update it manually.

---

### `internal/client/auth`

HTTP client for the webclient REST API. Used by both `cmd/ebitenclient` (all methods) and `cmd/webclient`'s Go layer where it needs to call its own API during testing or admin flows.

#### Types

```go
type Account struct {
    ID       int64
    Username string
    Role     string
}

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

type CreateCharacterRequest struct {
    Name      string
    Job       string
    Archetype string
    Region    string
    Gender    string
}

type CharacterOptions struct {
    Regions    []string
    Jobs       []string
    Archetypes map[string][]string // job → available archetypes
}
```

#### Errors

```go
var ErrUnauthorized = errors.New("unauthorized")
var ErrNameTaken    = errors.New("character name already taken")

type ErrValidation struct{ Message string }
func (e ErrValidation) Error() string { return "validation error: " + e.Message }

type ErrNetwork struct{ Cause error }
func (e ErrNetwork) Error() string { return "network error: " + e.Cause.Error() }
func (e ErrNetwork) Unwrap() error { return e.Cause }
```

#### API

```go
func New(baseURL string) *Client

// Account
func (c *Client) Login(ctx context.Context, username, password string) (jwt string, err error)
func (c *Client) Register(ctx context.Context, username, password string) (jwt string, err error)
func (c *Client) Me(ctx context.Context, jwt string) (*Account, error)

// Characters
func (c *Client) ListCharacters(ctx context.Context, jwt string) ([]CharacterSummary, error)
func (c *Client) CreateCharacter(ctx context.Context, jwt string, req CreateCharacterRequest) (*CharacterSummary, error)
func (c *Client) CheckName(ctx context.Context, name string) (available bool, err error)
func (c *Client) CharacterOptions(ctx context.Context, jwt string) (*CharacterOptions, error)
```

---

### `internal/client/feed`

Accumulates `ServerEvent` messages as `Entry` values, assigns color tokens, and enforces a configurable cap (default 500).

#### Types

```go
type Entry struct {
    Timestamp time.Time
    Token     render.ColorToken
    Text      string // pre-extracted narrative text; clients render this string directly
}
```

`Entry.Text` is extracted from the proto oneof field at `Append` time:
- `CombatEvent` → `narrative` field
- `MessageEvent` → `text` field
- `ErrorEvent` → `message` field
- `RoomEvent` → formatted arrival/departure string
- `CharacterInfo`, `InventoryView`, `CharacterSheetView` → structured summary line
- All others → proto message name + any available description field

#### Token Assignment

| ServerEvent oneof field | ColorToken |
|---|---|
| `MessageEvent` | `ColorSpeech` |
| `CombatEvent`, `RoundStartEvent`, `RoundEndEvent` | `ColorCombat` |
| `RoomEvent` | `ColorRoomEvent` |
| `ErrorEvent` | `ColorError` |
| `CharacterInfo`, `InventoryView`, `CharacterSheetView` | `ColorStructured` |
| all others | `ColorSystem` |

#### API

```go
func New(cap int) *Feed  // cap=0 → default 500

func (f *Feed) Append(ev *gamev1.ServerEvent)  // maps event → Entry, enforces cap (evicts oldest)
func (f *Feed) Entries() []Entry               // snapshot, oldest→newest order
func (f *Feed) Clear()

// DefaultTokenFor is exported so clients can override individual token assignments
// before constructing their own mapping.
func DefaultTokenFor(ev *gamev1.ServerEvent) render.ColorToken
```

`Feed` is goroutine-safe (`Append` and `Entries` may be called from different goroutines).

---

### `internal/client/history`

Command ring buffer. Not goroutine-safe — single-input-goroutine assumption consistent with both client designs. Not persisted between sessions.

```go
func New(cap int) *History  // cap=0 → default 100

func (h *History) Push(cmd string)  // adds entry; evicts oldest when full; resets cursor
func (h *History) Up() string       // navigate toward older entries; "" at oldest
func (h *History) Down() string     // navigate toward newer entries; "" past newest
func (h *History) Reset()           // return cursor to live position (call after each submission)
```

`Push` implicitly calls `Reset` so the cursor always starts from the most recent entry after a new command is submitted.

---

## Impact on Existing Specs

### `web-client` spec changes required

- REQ-WC-30: `internal/command/parse.go` extraction is already planned — no change.
- REQ-WC-27 (Feed colors), REQ-WC-28 (Character panel), REQ-WC-29 (Command history): add note that the data layer for these comes from `internal/client/{feed,session,history}`; React rendering remains in `cmd/webclient/ui`.

### `game-client-ebiten` spec changes required

- REQ-GCE-9: Remove "character creation not supported in this client" restriction. Character creation uses `internal/client/auth.CharacterOptions` and `CreateCharacter`.
- REQ-GCE-17 (`colours.yaml`): Add that Ebiten's `ColorMapper` implementation reads from `colours.yaml`; token names in the YAML MUST match `render.ColorToken` constant names.
- REQ-GCE-23 (Feed), REQ-GCE-24 (Character panel), REQ-GCE-28 (History): add note that concrete implementations use `internal/client/{feed,session,history}`.
- REQ-GCE-29 (command parse): already planned as shared — no change.

---

## Testing Strategy

- REQ-IC-1: Each sub-package MUST have unit tests achieving 100% statement coverage on all exported functions.
- REQ-IC-2: `auth.Client` MUST be tested against an `httptest.Server` stub — no live network calls in tests.
- REQ-IC-3: `session.Session` MUST be tested with a mock gRPC server using `google.golang.org/grpc/test/bufconn`.
- REQ-IC-4: `feed.Feed` and `history.History` MUST use property-based tests (rapid) for cap enforcement and cursor navigation invariants.
- REQ-IC-5: `render` package has no logic — interfaces and constants only; no test file required.

---

## Out of Scope

- Telnet client reuse — the existing `cmd/frontend` telnet client is not a consumer of this package.
- Admin API client — the admin HTTP endpoints in `cmd/webclient` are operator-only; no shared client is needed.
- Asset pack loading — Ebiten-specific; stays in `cmd/ebitenclient`.
- WebSocket proxy — web-client-specific; stays in `cmd/webclient`.

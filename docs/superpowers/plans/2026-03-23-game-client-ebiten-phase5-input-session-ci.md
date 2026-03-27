# Game Client (Ebiten) Phase 5: Input, Session & CI Builds

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** gRPC session lifecycle, full input handling, command dispatch, and cross-platform GitHub Actions CI builds.

**Architecture:** Session manages gRPC stream with goroutines. CommandDispatcher maps tokenized input to proto messages. InputHandler bridges Ebiten events to dispatcher. CI matrix builds four platform binaries on tag.

**Tech Stack:** Go gRPC, google.golang.org/grpc, internal/game/command, Ebiten v2, GitHub Actions

---

## Requirements Covered

- REQ-GCE-27: Mouse click handling (exit indicators, NPC sprites, Send button)
- REQ-GCE-28: Keyboard input (text, Enter submit, ↑/↓ history, Tab autocomplete, Escape clear)
- REQ-GCE-29: All submitted text parsed by `internal/game/command/parse.go`; no duplicate parsing logic
- REQ-GCE-30: gRPC stream error overlay (UNAUTHENTICATED → login; other → reconnect overlay)
- REQ-GCE-32: Cross-compiled for linux/amd64, windows/amd64, darwin/amd64, darwin/arm64
- REQ-GCE-33: GitHub Actions ebiten-release.yml — build matrix, linux smoke test, attach to release
- REQ-GCE-34: GitHub Actions release-assets.yml — build and upload asset zip + sha256
- REQ-GCE-35: Makefile targets `build-ebiten` and `package-assets`

## Assumptions

- Phases 1–4 complete: binary skeleton, auth/character-select screens, asset pack download, rendering pipeline all functional.
- `cmd/ebitenclient/game/screen.go` defines `GameScreen` with fields `sendCh chan *gamev1.ClientMessage`, `errCh chan error`, and `roomView *gamev1.RoomView` (set by Phase 4 rendering tasks).
- `cmd/ebitenclient/game/state.go` exposes `State` with `NPCNames() []string` returning names from current `RoomView`.
- The proto package path is `github.com/cory-johannsen/mud/internal/gameserver/gamev1`.
- `grpc.NewClient` from `google.golang.org/grpc` is already a dependency (used by existing services).
- The `assets/version.txt` file exists and contains a monotonically increasing integer version string.

---

## Task 1 — Inspect proto for exact ClientMessage field names

- [ ] Read `api/proto/game/v1/game.proto` in full to confirm the exact field names for:
  - `MoveRequest` (direction field name)
  - `SayRequest` (message field name)
  - `LookRequest` (no fields)
  - `AttackRequest` (target field name)
  - `EmoteRequest` (message field name)
  - `ExamineRequest` (target field name)
  - `QuitRequest` (no fields)
  - `WhoRequest` (no fields)
  - `ExitsRequest` (no fields)
  - `FleeRequest` (no fields)
- [ ] Record exact proto-generated Go field names (snake_case becomes CamelCase in Go struct).
- [ ] Confirm `JoinWorldRequest` fields: `uid`, `username`, `character_id`, `character_name`, `current_hp`, `location`, `role`, `region_display`, `class`, `level` — verify each exists in the proto.

**Expected output:** a Go comment block in `dispatch.go` listing verified field names before the dispatch table.

---

## Task 2 — CommandDispatcher

**File:** `cmd/ebitenclient/session/dispatch.go`

- [ ] Define `CommandDispatcher` struct (stateless; no fields required).
- [ ] Implement `func (d *CommandDispatcher) Dispatch(line string) (*gamev1.ClientMessage, error)`:
  - Call `command.Parse(line)` from `internal/game/command`.
  - Switch on `ParseResult.Command`:
    - `"move"`: require `len(Args) >= 1`; return `ClientMessage{Payload: &gamev1.ClientMessage_Move{Move: &gamev1.MoveRequest{Direction: Args[0]}}}`.
    - `"say"`: require `RawArgs != ""`; return `ClientMessage{Payload: &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: RawArgs}}}`.
    - `"look"`: return `ClientMessage{Payload: &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}}}`.
    - `"attack"`: require `len(Args) >= 1`; return `ClientMessage{Payload: &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: Args[0]}}}`.
    - `"emote"`: require `RawArgs != ""`; return `ClientMessage{Payload: &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Message: RawArgs}}}`.
    - `"examine"`: require `len(Args) >= 1`; return `ClientMessage{Payload: &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: Args[0]}}}`.
    - `"who"`: return `ClientMessage{Payload: &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}}}`.
    - `"exits"`: return `ClientMessage{Payload: &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}}}`.
    - `"flee"`: return `ClientMessage{Payload: &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}}}`.
    - `"quit"`: return `nil, ErrQuit` (sentinel error; caller triggers graceful shutdown).
    - default: return `ClientMessage{Payload: &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: line}}}` with a secondary error `ErrUnknownCommand` so the caller MAY display feedback.
  - Populate `RequestId` on every message using `fmt.Sprintf("ebiten-%d", time.Now().UnixNano())`.
- [ ] Define `var ErrQuit = errors.New("quit")` and `var ErrUnknownCommand = errors.New("unknown command")`.

**File:** `cmd/ebitenclient/session/dispatch_test.go`

- [ ] Table-driven tests covering all 11 command cases above plus:
  - Empty input → error (Parse returns empty Command).
  - `"move"` without direction → error (missing argument).
  - `"say"` without text → error (empty RawArgs).
  - `"attack npc name with spaces"` → target is `"npc"` (first token only).
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/session/... -v -run TestCommandDispatcher`; all tests MUST pass.

---

## Task 3 — Session struct

**File:** `cmd/ebitenclient/session/session.go`

- [ ] Define `Config` struct:
  ```go
  type Config struct {
      GameserverAddr string
      UID            string
      Username       string
      CharacterID    string
      CharacterName  string
      CurrentHP      int32
      Location       string
      Role           string
      RegionDisplay  string
      Class          string
      Level          int32
  }
  ```
- [ ] Define `Session` struct:
  ```go
  type Session struct {
      conn   *grpc.ClientConn
      stream gamev1.GameService_SessionClient
      sendCh chan *gamev1.ClientMessage
      EventCh chan *gamev1.ServerEvent
      ErrCh  chan error
      cancel context.CancelFunc
      wg     sync.WaitGroup
  }
  ```
- [ ] Implement `func New(ctx context.Context, cfg Config) (*Session, error)`:
  - Dial with `grpc.NewClient(cfg.GameserverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))`.
  - Create `GameService` stub and call `stub.Session(streamCtx)`.
  - Send `JoinWorldRequest` with all fields from `cfg` as the first message.
  - Initialise `sendCh` (buffered 64), `EventCh` (buffered 256), `ErrCh` (buffered 1).
  - Launch `sendLoop` and `recvLoop` goroutines; add to `wg`.
  - Return `*Session, nil` on success; close conn and return error on any failure.
- [ ] Implement `sendLoop`:
  - `for msg := range s.sendCh { stream.Send(msg) }` with context cancellation guard.
  - On `ctx.Done()`: return.
- [ ] Implement `recvLoop`:
  - Loop `stream.Recv()`. On success push to `EventCh`.
  - On error: call `s.handleStreamError(err)`; return.
- [ ] Implement `func (s *Session) Send(msg *gamev1.ClientMessage)`:
  - Non-blocking send: `select { case s.sendCh <- msg: default: }` (drop if full, log warning).
- [ ] Implement `func (s *Session) Close() error`:
  - Call `s.cancel()`.
  - Call `s.stream.CloseSend()`.
  - Wait up to 2 seconds for `s.wg` via a `done` channel; if timeout exceeded, log warning.
  - Return `s.conn.Close()`.
- [ ] Implement `handleStreamError(err error)`:
  - Non-blocking push to `ErrCh`: `select { case s.ErrCh <- err: default: }`.

**File:** `cmd/ebitenclient/session/session_test.go`

- [ ] Implement a minimal in-process mock gRPC server using `google.golang.org/grpc/test/bufconn`:
  - `bufconn.Listen(1<<20)`, register a `GameService` server that echoes a canned `ServerEvent` for each `ClientMessage` received.
  - Override dial to use `grpc.WithContextDialer` pointing at the bufconn listener.
- [ ] Test `New`: verifies `JoinWorldRequest` is received by mock server as first message.
- [ ] Test `Send` → `EventCh`: sends a command via `Session.Send`, asserts mock echoes back and `EventCh` receives the event.
- [ ] Test `Close`: calls `Close()` within 3 seconds; asserts no goroutine leak (use `goleak` or manual wg timeout).
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/session/... -v -run TestSession`; all MUST pass.

---

## Task 4 — Session error handling

**File:** `cmd/ebitenclient/session/session.go` (additions)

- [ ] Implement `func IsUnauthenticated(err error) bool`:
  - Import `google.golang.org/grpc/status` and `google.golang.org/grpc/codes`.
  - Return `status.Code(err) == codes.Unauthenticated`.
- [ ] Export `ErrStreamClosed = errors.New("stream closed")` for EOF wrapping.
- [ ] In `recvLoop`, on `io.EOF`: push `ErrStreamClosed` to `ErrCh`.

**File:** `cmd/ebitenclient/game/screen.go` (additions — wire-in task covered by Task 7)

- [ ] Document (in a code comment) that `GameScreen.Update()` MUST drain `session.ErrCh` each tick:
  - If `IsUnauthenticated(err)` → transition to login screen by setting `game.screen = newLoginScreen(...)`.
  - Otherwise → set `game.reconnectErr = err.Error()` and render a reconnect overlay.

**File:** `cmd/ebitenclient/session/session_test.go` (additions)

- [ ] Test `IsUnauthenticated`: construct a gRPC status error with `codes.Unauthenticated`; assert returns `true`. Construct with `codes.Unavailable`; assert returns `false`.
- [ ] Test stream-closed path: mock server closes stream; assert `ErrCh` receives `ErrStreamClosed`.
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/session/... -v -run TestError`; all MUST pass.

---

## Task 5 — Graceful shutdown on window close

**File:** `cmd/ebitenclient/game/screen.go` (additions)

- [ ] Implement `func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int)` — already required by Ebiten; confirm it exists from Phase 1.
- [ ] In the main game loop entry point (`cmd/ebitenclient/main.go`), wrap `ebiten.RunGame(g)` return:
  ```go
  if err := ebiten.RunGame(g); err != nil && !errors.Is(err, ebiten.Termination) {
      log.Fatal(err)
  }
  if g.session != nil {
      shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
      defer cancel()
      _ = g.session.CloseWithContext(shutdownCtx)
  }
  ```
- [ ] Rename `Session.Close()` to `Session.CloseWithContext(ctx context.Context) error` — accept a context for the 2-second drain wait instead of using a hardcoded timer.
- [ ] Update `session_test.go` to call `CloseWithContext` with a test context.
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/... -v`; all MUST pass.

---

## Task 6 — InputHandler: keyboard

**File:** `cmd/ebitenclient/game/input.go`

- [ ] Define `InputHandler` struct:
  ```go
  type InputHandler struct {
      buffer   []rune
      history  []string   // capped at 100; index 0 = oldest
      histIdx  int        // -1 = not navigating; 0..len-1 = current position
      tabState tabComplete
  }

  type tabComplete struct {
      candidates []string
      idx        int
      active     bool
  }
  ```
- [ ] Implement `func NewInputHandler() *InputHandler` — returns zero-value with `histIdx = -1`.
- [ ] Implement `func (h *InputHandler) HandleKeys(npcs []string) (submitted string, ok bool)`:
  - Call `inpututil.AppendJustPressedKeys(nil)` to enumerate pressed keys.
  - Printable keys (space through tilde in ebiten key range): append rune to `h.buffer`; reset `h.histIdx = -1`; deactivate tab state.
  - `KeyEnter`: if `len(h.buffer) > 0`, capture `line := string(h.buffer)`, push to history (cap at 100, drop oldest when over), clear buffer, reset `histIdx = -1`, return `line, true`.
  - `KeyUp`: if `histIdx < len(history)-1`, increment `histIdx`, set buffer to `[]rune(history[len(history)-1-histIdx])`.
  - `KeyDown`: if `histIdx > 0`, decrement `histIdx`, set buffer from history; if `histIdx == -1`, clear buffer.
  - `KeyTab`: populate or cycle `tabState` from `npcs`; replace buffer with `"attack " + candidate`.
  - `KeyEscape`: clear `h.buffer`; reset `histIdx = -1`; deactivate tab state.
  - Return `"", false` when no submission.
- [ ] Implement `func (h *InputHandler) Buffer() string` — returns `string(h.buffer)` for rendering.
- [ ] Implement `func (h *InputHandler) SetBuffer(s string)` — replaces buffer (used by mouse NPC click).

**File:** `cmd/ebitenclient/game/input_test.go`

- [ ] Test `HandleKeys` for each key path listed above.
- [ ] Test history cap: add 101 commands; verify `len(history) == 100` and oldest is dropped.
- [ ] Test history navigation: add 3 commands; press ↑ twice; verify buffer matches expected history entry.
- [ ] Test Tab autocomplete: provide `["Ganger", "Bandit"]`; press Tab; verify buffer is `"attack Ganger"`; press Tab again; verify `"attack Bandit"`; press Tab again; verify cycles back to `"attack Ganger"`.
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/game/... -v -run TestInputHandler`; all MUST pass.

---

## Task 7 — InputHandler: mouse

**File:** `cmd/ebitenclient/game/input.go` (additions)

- [ ] Define `ExitIndicator` struct (shared with renderer from Phase 4):
  ```go
  type ExitIndicator struct {
      Direction string
      Bounds    image.Rectangle
  }
  ```
- [ ] Define `NPCSprite` struct:
  ```go
  type NPCSprite struct {
      Name   string
      Bounds image.Rectangle
  }
  ```
- [ ] Define `SendButtonBounds image.Rectangle` (package-level var set by renderer each frame).
- [ ] Implement `func (h *InputHandler) HandleMouse(exits []ExitIndicator, npcs []NPCSprite, sendBounds image.Rectangle) (submitted string, ok bool)`:
  - On left mouse button just pressed (`inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)`):
    - Get cursor position via `ebiten.CursorPosition()`.
    - Iterate `exits`: if cursor is inside `indicator.Bounds`, set `line = "move " + indicator.Direction`, return `line, true` (auto-dispatch).
    - Iterate `npcs`: if cursor is inside `npc.Bounds`, call `h.SetBuffer("attack " + npc.Name)`, return `"", false` (populate buffer, do not dispatch).
    - If cursor inside `sendBounds` and `len(h.buffer) > 0`: behave as Enter — capture, push history, clear, return `line, true`.
  - Return `"", false` if no hit.

**File:** `cmd/ebitenclient/game/input_test.go` (additions)

- [ ] Test exit click: create a fake `ExitIndicator{Direction: "north", Bounds: image.Rect(10,10,50,50)}`; simulate cursor at (30, 30); assert returns `"move north", true`.
- [ ] Test NPC click: simulate cursor on NPC bounds; assert `Buffer()` is `"attack Ganger"` and `ok == false`.
- [ ] Test Send button: set buffer to `"look"`; simulate cursor on send bounds; assert returns `"look", true` and buffer is cleared.
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/game/... -v -run TestMouse`; all MUST pass.

---

## Task 8 — Wire session + input into GameScreen

**File:** `cmd/ebitenclient/game/screen.go`

- [ ] Add fields to `GameScreen`:
  ```go
  session      *session.Session
  dispatcher   *session.CommandDispatcher
  inputHandler *game.InputHandler
  reconnectErr string
  ```
- [ ] In `GameScreen.Update()`:
  1. Call `h.HandleKeys(g.state.NPCNames())` → if submitted: call `g.dispatcher.Dispatch(line)` → on `ErrQuit` trigger graceful shutdown; on valid msg call `g.session.Send(msg)`; on `ErrUnknownCommand` add feedback to feed panel with text `"Unknown command: " + line`.
  2. Call `h.HandleMouse(g.exitIndicators, g.npcSprites, g.sendButtonBounds)` → if submitted: same dispatch path.
  3. Drain `g.session.EventCh` (non-blocking, up to 64 events per tick) → apply to game state.
  4. Drain `g.session.ErrCh` (non-blocking, one event) → if `IsUnauthenticated`: transition to login; else set `g.reconnectErr`.
- [ ] In `GameScreen.Draw(screen *ebiten.Image)`:
  - If `g.reconnectErr != ""`: render a semi-transparent dark overlay; draw error text centred; draw a "Reconnect" button that on click transitions to character select and sets `g.reconnectErr = ""`.
- [ ] In character-select screen's `onSelect` callback (from Phase 2):
  - Call `session.New(ctx, cfg)` with character data populated from selected character.
  - On error: display error and stay on character-select screen.
  - On success: set `g.session` on `GameScreen`; transition to `GameScreen`.
- [ ] Run: `mise exec -- go test ./cmd/ebitenclient/... -v`; all MUST pass.

---

## Task 9 — Makefile targets

**File:** `Makefile`

- [ ] Add `build-ebiten` and `package-assets` to the `.PHONY` line.
- [ ] Add after existing `build-*` targets:
  ```makefile
  build-ebiten: proto
  	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/mud-$(GOOS)-$(GOARCH)$(BINARY_EXT) ./cmd/ebitenclient

  package-assets:
  	$(eval ASSET_VERSION := $(shell cat assets/version.txt))
  	cd assets && zip -r ../dist/mud-assets-v$(ASSET_VERSION).zip .
  	sha256sum dist/mud-assets-v$(ASSET_VERSION).zip > dist/mud-assets-v$(ASSET_VERSION).sha256
  ```
- [ ] Add `dist/` to `.gitignore` if not already present.
- [ ] Set `GOOS ?= $(shell go env GOOS)`, `GOARCH ?= $(shell go env GOARCH)`, and `BINARY_EXT` logic:
  ```makefile
  BINARY_EXT :=
  ifeq ($(GOOS),windows)
  BINARY_EXT := .exe
  endif
  ```
- [ ] Verify `make build-ebiten` compiles without error on the host platform.
- [ ] Verify `make package-assets` produces `dist/mud-assets-v{N}.zip` and `dist/mud-assets-v{N}.sha256`.

---

## Task 10 — GitHub Actions: ebiten-release.yml

**File:** `.github/workflows/ebiten-release.yml`

- [ ] Create the file with the following content (exact YAML, no placeholders):

```yaml
name: Ebiten Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    name: Build ${{ matrix.GOOS }}/${{ matrix.GOARCH }}
    runs-on: ${{ matrix.os }}
    permissions:
      contents: write
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            GOOS: linux
            GOARCH: amd64
            suffix: ""
            artifact: mud-linux-amd64
          - os: ubuntu-latest
            GOOS: windows
            GOARCH: amd64
            suffix: ".exe"
            artifact: mud-windows-amd64.exe
          - os: macos-latest
            GOOS: darwin
            GOARCH: amd64
            suffix: ""
            artifact: mud-darwin-amd64
          - os: macos-latest
            GOOS: darwin
            GOARCH: arm64
            suffix: ""
            artifact: mud-darwin-arm64

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install protoc (ubuntu)
        if: matrix.os == 'ubuntu-latest'
        run: |
          sudo apt-get update -q
          sudo apt-get install -y protobuf-compiler
          go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
          go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

      - name: Install protoc (macos)
        if: matrix.os == 'macos-latest'
        run: |
          brew install protobuf
          go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
          go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

      - name: Generate proto
        run: make proto

      - name: Build
        env:
          GOOS: ${{ matrix.GOOS }}
          GOARCH: ${{ matrix.GOARCH }}
        run: GOOS=${{ matrix.GOOS }} GOARCH=${{ matrix.GOARCH }} make build-ebiten

      - name: Rename artifact
        run: |
          mv bin/mud-${{ matrix.GOOS }}-${{ matrix.GOARCH }}${{ matrix.suffix }} \
             bin/${{ matrix.artifact }}

      - name: Smoke test (linux only)
        if: matrix.GOOS == 'linux'
        run: |
          ./bin/${{ matrix.artifact }} --version
          echo "smoke test passed"

      - name: Upload to GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: bin/${{ matrix.artifact }}
```

- [ ] Confirm the workflow file is syntactically valid YAML (no tab characters in indentation).
- [ ] Confirm `make build-ebiten` is the sole build command invoked; no duplicate logic.
- [ ] Note: the `--version` flag MUST be implemented in `cmd/ebitenclient/main.go` (print version string and exit 0). Add this to `main.go` if not already present from Phase 1.

---

## Task 11 — GitHub Actions: release-assets.yml

**File:** `.github/workflows/release-assets.yml`

- [ ] Create the file with the following content:

```yaml
name: Release Assets

on:
  workflow_dispatch:
  push:
    paths:
      - 'assets/**'
    branches:
      - main

jobs:
  package:
    name: Package and upload assets
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Read asset version
        id: version
        run: echo "ASSET_VERSION=$(cat assets/version.txt)" >> "$GITHUB_OUTPUT"

      - name: Package assets
        run: |
          mkdir -p dist
          make package-assets

      - name: Create or update GitHub Release for assets
        uses: softprops/action-gh-release@v2
        with:
          tag_name: assets-v${{ steps.version.outputs.ASSET_VERSION }}
          name: "Asset Pack v${{ steps.version.outputs.ASSET_VERSION }}"
          files: |
            dist/mud-assets-v${{ steps.version.outputs.ASSET_VERSION }}.zip
            dist/mud-assets-v${{ steps.version.outputs.ASSET_VERSION }}.sha256
```

- [ ] Confirm the workflow triggers on both `workflow_dispatch` and pushes to `main` that touch `assets/**`.
- [ ] Confirm the release tag convention matches `mud-assets-v{N}` referenced in REQ-GCE-12.

---

## Task 12 — Full test suite verification

- [ ] Run `mise exec -- go test ./cmd/ebitenclient/... -v` and confirm 100% pass.
- [ ] Run `mise exec -- go build ./cmd/ebitenclient/...` and confirm no build errors.
- [ ] Run `make build-ebiten` and confirm binary produced in `bin/`.
- [ ] Run `make package-assets` and confirm `dist/mud-assets-v{N}.zip` and `.sha256` are produced.
- [ ] Manually inspect `.github/workflows/ebiten-release.yml` and `.github/workflows/release-assets.yml` for YAML validity.

---

## File Checklist

| File | Status |
|---|---|
| `cmd/ebitenclient/session/dispatch.go` | - [ ] |
| `cmd/ebitenclient/session/dispatch_test.go` | - [ ] |
| `cmd/ebitenclient/session/session.go` | - [ ] |
| `cmd/ebitenclient/session/session_test.go` | - [ ] |
| `cmd/ebitenclient/game/input.go` | - [ ] |
| `cmd/ebitenclient/game/input_test.go` | - [ ] |
| `cmd/ebitenclient/game/screen.go` (additions) | - [ ] |
| `cmd/ebitenclient/main.go` (`--version` flag + shutdown hook) | - [ ] |
| `Makefile` (`build-ebiten`, `package-assets`) | - [ ] |
| `.github/workflows/ebiten-release.yml` | - [ ] |
| `.github/workflows/release-assets.yml` | - [ ] |

---

## Definition of Done

- REQ-GCE-27: Mouse clicks on exits auto-dispatch `"move {direction}"`; NPC clicks populate buffer without dispatching; Send button dispatches buffer.
- REQ-GCE-28: Keyboard history (100 entries, in-memory), Tab NPC autocomplete, Escape clear all functional.
- REQ-GCE-29: Zero command-parsing logic outside `internal/game/command/Parse`; dispatcher delegates entirely to it.
- REQ-GCE-30: `UNAUTHENTICATED` gRPC error returns user to login screen; all other stream errors show reconnect overlay.
- REQ-GCE-32: `make build-ebiten` cross-compiles for all four targets when `GOOS`/`GOARCH` are set.
- REQ-GCE-33: `ebiten-release.yml` builds all four binaries on `v*` tag; linux binary smoke-tested with `--version`.
- REQ-GCE-34: `release-assets.yml` packages and uploads asset zip + sha256 on manual trigger or `assets/` change.
- REQ-GCE-35: Both `make build-ebiten` and `make package-assets` targets present and functional.
- All tests pass at 100%.

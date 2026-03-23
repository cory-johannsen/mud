# Game Client (Ebiten)

**Slug:** game-client-ebiten
**Status:** spec
**Priority:** 440
**Category:** ui
**Effort:** XXL

## Overview

A native cross-platform game client (Linux amd64, Windows amd64, macOS amd64/arm64) built on `github.com/hajimehoshi/ebiten/v2`. Connects directly to the gameserver gRPC stream for maximum performance. Renders a tile-based 2D scene per room, with ComfyUI-generated sprite sheets for zones, NPCs, and items, and frame-cycled combat animations. Supports both keyboard command entry and mouse-driven interaction. Distributed as pre-built binaries via GitHub Releases alongside a separately versioned asset pack.

## Architecture

```
cmd/ebitenclient/ (Go binary, cross-compiled)
     │
     ├── Auth:       POST webclient /api/auth/login → JWT
     ├── Characters: GET  webclient /api/characters → character select screen
     │
     ├── input text → internal/command/parse.go → ClientMessage proto
     │                              ↓
     ├── gRPC direct: GameService.Session stream → JoinWorldRequest
     │       ├── Send goroutine: ClientMessage → stream.Send()
     │       └── Recv goroutine: ServerEvent → game state → re-render
     │
     └── Ebiten render loop (60fps): game state → tiles + sprites + UI panels
```

**Key constraints:**
- Gameserver gRPC service is unchanged — same bidirectional stream, same proto (`api/proto/game/v1/game.proto`)
- Auth and character selection go via the webclient REST API (deployed alongside gameserver)
- `internal/command/parse.go` is imported directly into the binary: raw text input is parsed into `ClientMessage` protos client-side before being sent over gRPC. All input — keyboard and mouse — synthesises a command string and routes it through `parse.go`.
- Assets are not embedded in the binary; downloaded as a versioned asset pack on first run or when a newer pack is available

---

## Requirements

### 1. Binary & Configuration

- REQ-GCE-1: A new `cmd/ebitenclient/` binary MUST be created, separate from `cmd/frontend/` and `cmd/webclient/`.
- REQ-GCE-2: A `config.yaml` MUST be read from `os.UserConfigDir()/mud-ebiten/config.yaml`. It MUST contain: `webclient_url` (default `http://localhost:8080`), `gameserver_addr` (default `localhost:50051`), `github_releases_url` (default `https://api.github.com/repos/cory-johannsen/mud/releases/latest`), and `log_level` (default `info`). On first run, if the file does not exist, defaults MUST be written to disk. If the write fails, the client MUST log a warning and continue with in-memory defaults.
- REQ-GCE-3: The client MUST present an initial window of 1280×800 pixels. The window MUST be resizable; all layout regions MUST scale proportionally on resize. A minimum window size of 800×600 MUST be enforced.
- REQ-GCE-4: The window title MUST be `"Mud"` on login and character-select screens, and `"Mud — {character_name}"` during an active game session.
- REQ-GCE-5: The client MUST write structured log output to `os.UserCacheDir()/mud-ebiten/client.log`. Log level MUST be controlled by the `log_level` config field.
- REQ-GCE-6: On window close during an active session, the client MUST call `CloseSend()` on the gRPC stream, wait up to 2 seconds for the server to acknowledge, then exit.

### 2. Authentication & Character Selection

- REQ-GCE-7: The client MUST display a login screen on startup with username and password fields. It MUST authenticate by sending `POST {webclient_url}/api/auth/login` with `{"username": string, "password": string}`. On auth failure, the error message from the API MUST be displayed and the user MAY retry.
- REQ-GCE-8: JWT tokens MUST NOT be persisted to disk between sessions. The user MUST re-authenticate on each launch.
- REQ-GCE-9: After successful login, the client MUST fetch `GET {webclient_url}/api/characters` (with `Authorization: Bearer <token>`) and display a character selection screen listing available characters with name, job, level, and HP. If the account has no characters, a message MUST inform the user that character creation is not supported in this client and direct them to the web client.
- REQ-GCE-10: On character selection, the client MUST open a `GameService.Session` gRPC stream and send a `JoinWorldRequest` with fields populated from the selected character, as defined in `api/proto/game/v1/game.proto`. If the stream cannot be opened, the client MUST display the error and return the user to the character selection screen.

### 3. Asset Pack

- REQ-GCE-11: Asset source files MUST reside in the `assets/` directory at the repository root. This directory is the sole input to `make package-assets`.
- REQ-GCE-12: On startup (before the login screen), the client MUST fetch the latest release metadata from `github_releases_url`. The asset pack version MUST be a monotonically increasing integer stored in `version.txt`. If the local version (read from `os.UserCacheDir()/mud-ebiten/assets/version.txt`) is absent or numerically lower than the remote version, the client MUST display a download progress screen and fetch `mud-assets-v{N}.zip`. If the network is unreachable but a local pack already exists, the client MUST log a warning and proceed with the local pack. If no local pack exists and the network is unreachable, the client MUST display an error and exit.
- REQ-GCE-13: If the asset pack download fails, the client MUST display the error reason and a Retry button. The client MUST NOT proceed to the login screen without a valid asset pack.
- REQ-GCE-14: The GitHub Release MUST include a `mud-assets-v{N}.sha256` file. The client MUST verify the SHA-256 checksum of the downloaded zip before extraction. If verification fails, the client MUST delete the partial download and display an error.
- REQ-GCE-15: The asset pack MUST be a zip file (`mud-assets-v{N}.zip`) with the following structure:
  ```
  mud-assets-v{N}/
    version.txt
    tiles.yaml
    colours.yaml
    tilesets/
      zones/        # one PNG sprite sheet per zone category
      npcs/         # one PNG sprite sheet per NPC category
      items/        # item category sprites
      ui/           # HP bars, condition badges, UI chrome
    animations/
      combat/       # per-NPC-category attack/hit/death frame strips
  ```
- REQ-GCE-16: `tiles.yaml` MUST map zone categories, NPC types, and item categories to sprite sheet paths and tile coordinates. Tile coordinates MUST be specified as `{x: int, y: int, w: int, h: int}` in pixels within the sprite sheet PNG. Animation fps for each NPC category MUST be defined in `tiles.yaml`; the default fps if absent MUST be 12.
- REQ-GCE-17: `colours.yaml` MUST map `ServerEvent` oneof field names (as defined in `api/proto/game/v1/game.proto`) to hex colour values. This allows colour assignments to be updated without a binary release. The client MUST apply these colours when rendering the Feed panel. Unknown event types MUST fall back to the colour defined under the `default` key in `colours.yaml`.
- REQ-GCE-18: All sprites MUST be generated via ComfyUI. The asset pack MUST be built and published independently of the game binary, attached to GitHub Releases as a separate artifact.
- REQ-GCE-19: The client MUST resolve `RoomView.zone_name` to a zone category via a lookup table in `tiles.yaml`. Unrecognised zone names MUST fall back to a default tileset.

### 4. Room View & Rendering

- REQ-GCE-20: The game screen MUST be divided into four regions matching the ASCII wireframe below. Region proportions are targets; implementations MAY vary up to ±5% to accommodate font metrics and integer pixel boundaries:
  - **Scene** (full width, upper ~70%): zone background, NPC sprites, player sprite, exit indicators
  - **Feed** (left ~60%, lower ~25%): scrollable colour-coded message log
  - **Character** (right ~40%, lower ~25%): name, HP bar, conditions, hero points
  - **Input** (full width, bottom ~5%): text field and Send button
- REQ-GCE-21: The scene MUST render the zone background tile scaled to fill the full scene width. The player sprite MUST be anchored bottom-centre. A maximum of 6 NPC sprites MUST be displayed, evenly spaced across the scene width. If `RoomView.npcs` contains more than 6 entries, the 6th sprite MUST display a count badge showing the total number of additional NPCs not rendered.
- REQ-GCE-22: Exit indicators MUST be rendered at scene edges corresponding to direction (N=top, S=bottom, E=right, W=left) for each exit present in `RoomView.exits`. Each indicator MUST be clickable (see REQ-GCE-27).
- REQ-GCE-23: The Feed panel MUST accumulate up to the last 500 `ServerEvent` messages and auto-scroll to the latest; older messages MUST be discarded when the limit is exceeded. Message type colours MUST be loaded from `colours.yaml` at startup.
- REQ-GCE-24: The Character panel MUST display: character name, HP as a colour-coded progress bar (green >50%, yellow >25%, red ≤25%), current/max HP from `CharacterInfo.current_hp` / `CharacterInfo.max_hp`, active conditions from `RoomView.active_conditions`, and hero points from `ServerEvent.character_sheet.hero_points` (`CharacterSheetView` field 42, delivered as `ServerEvent` oneof field 17).

### 5. Combat Animations

- REQ-GCE-25: When a `CombatEvent` with `type = COMBAT_EVENT_TYPE_ATTACK` is received, the sprite matching `CombatEvent.attacker` MUST play its attack animation by cycling frames at the fps defined in `tiles.yaml`. The sprite matching `CombatEvent.target` MUST play a hit-flash frame.
- REQ-GCE-26: When a `CombatEvent` with `type = COMBAT_EVENT_TYPE_DEATH` is received, the sprite matching `CombatEvent.target` MUST play its death animation frames. Scene state is server-authoritative: the sprite MUST be removed only when a subsequent `RoomView` is received that no longer includes that NPC. Animations MUST be queued per sprite — concurrent events on the same sprite MUST play sequentially.

### 6. Input Handling

- REQ-GCE-27: Mouse clicks MUST be handled as follows:
  - Exit indicator click → synthesises the command string `"move {direction}"` and routes it through `internal/command/parse.go`
  - NPC sprite click → populates the input field with `"attack {npc_name}"` regardless of combat state; the user MAY edit before submitting
  - Send button click → submits the input field
- REQ-GCE-28: Keyboard input MUST be handled as follows:
  - Printable keys append to the input field; Enter submits
  - ↑/↓ navigate command history (last 100 commands, in-memory only, not persisted between sessions)
  - Tab cycles through visible NPC names for auto-complete
  - Escape clears the input field
- REQ-GCE-29: All submitted text MUST be parsed by `internal/command/parse.go` to produce a `ClientMessage` proto, which is then sent over the gRPC stream. The Ebiten client MUST NOT contain duplicate command parsing logic.

### 7. Network Error Handling

- REQ-GCE-30: If the gRPC stream closes unexpectedly or returns a non-OK status, the client MUST display an overlay with the error reason and a Reconnect button that returns the user to the character selection screen. If the gRPC status is `UNAUTHENTICATED`, the client MUST instead return the user to the login screen.

### 8. Asset Integrity

- REQ-GCE-31: If `tiles.yaml` is missing or unparseable after asset pack extraction, the client MUST delete the extracted pack directory, log an error, and re-enter the asset download flow (REQ-GCE-12).

### 9. Build & Distribution

- REQ-GCE-32: The binary MUST be cross-compiled for four targets: `linux/amd64`, `windows/amd64`, `darwin/amd64`, `darwin/arm64`.
- REQ-GCE-33: A GitHub Actions workflow MUST build all four binary targets on git tag `v*` and attach them to the GitHub Release: `mud-linux-amd64`, `mud-windows-amd64.exe`, `mud-darwin-amd64`, `mud-darwin-arm64`. The Linux binary MUST be smoke-tested on a Debian-based CI runner before the release is published; the smoke test MUST invoke the binary with `--version` and assert exit code 0.
- REQ-GCE-34: A separate `release-assets` GitHub Actions workflow MUST build and upload `mud-assets-v{N}.zip` and `mud-assets-v{N}.sha256` to GitHub Releases when triggered manually or on changes to the `assets/` directory.
- REQ-GCE-35: The following Makefile targets MUST be added:
  - `make build-ebiten` — builds the binary for the current platform
  - `make package-assets` — zips `assets/` into `mud-assets-v{N}.zip` and generates the SHA-256 file, using the version from `assets/version.txt`

---

## UI Layout (ASCII)

```
┌─────────────────────────────────────────────────────┐
│ SCENE (full width, ~70% height)                     │
│  [zone background tile — full width]                │
│                                                     │
│  [NPC]  [NPC]  [NPC]  [NPC]  [NPC]  [NPC +N]       │
│                                                     │
│                        [player sprite]              │
│  [N exit]                                           │
│                              [E exit]               │
│  [W exit]                   [S exit]                │
├────────────────────┬────────────────────────────────┤
│ FEED (~60% width)  │ CHARACTER (~40% width)         │
│ [18:44] You strike │ Zork  Lv5  Ganger              │
│ Shady Stan for 7   │ ████████░░ 38/50 HP            │
│ damage.            │ ⚡ Bleeding                    │
│ [18:44] Shady Stan │ ✦ Hero: 1                      │
│ slashes you for 4. │                                │
├────────────────────┴────────────────────────────────┤
│ > ____________________________________________[Send] │
└─────────────────────────────────────────────────────┘
```

---

## Dependencies

- `web-client` — auth and character APIs used at login/character-select (REQ-GCE-7, REQ-GCE-9)
- `internal/command/parse.go` — shared command parser extracted in web-client Phase 2 (REQ-GCE-29)

---

## Out of Scope

- Character creation (use the web client)
- Inventory / equipment UI (future iteration)
- Map panel (future iteration)
- Audio / sound effects
- Mobile platforms (iOS, Android)
- OAuth / SSO login

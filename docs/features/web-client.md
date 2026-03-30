# Web Game Client

**Slug:** web-client
**Status:** done
**Priority:** 430
**Category:** ui
**Effort:** XXL

## Overview

A browser-based game client that runs alongside the existing telnet frontend on a separate port. Players authenticate via a REST API, select a character, then connect to the gameserver over a persistent WebSocket. A Go WebSocket proxy bridges the browser session to the existing gRPC bidirectional stream — no changes to the gameserver proto or logic are required. The UI is a modern React/TypeScript single-page application with a structured panel layout. An integrated admin interface gives operators account management, live player inspection, zone/room editing, NPC spawning, and a live event log.

## Architecture

```
Browser (React/TS)
     │
     │  HTTPS / WSS
     ▼
cmd/webclient (Go HTTP server, port 8080)
     ├── POST /api/auth/*          → Postgres (accounts, characters)
     ├── GET  /api/characters      → Postgres
     ├── GET  /ws?token=JWT        → WebSocket ↔ gRPC proxy
     │         ├── WS reader goroutine: JSON → ClientMessage → stream.Send()
     │         └── gRPC reader goroutine: ServerEvent → JSON → WS write
     └── /api/admin/*              → Postgres + GameServiceServer admin calls
          └── GET /api/admin/events → SSE live log stream
```

**Key constraints:**
- Gameserver gRPC service is unchanged — same bidirectional stream, same proto
- WebSocket messages are JSON-encoded proto via `protojson` (Go) and `@bufbuild/protobuf` (TypeScript)
- JWT (HS256, 24h TTL) carries `account_id`, `character_id`, `role`
- React app is embedded in the Go binary via `//go:embed ui/dist` for production; served from filesystem in dev via `WEB_STATIC_DIR` env var

---

## Requirements

### 1. Go Web Server (`cmd/webclient/`)

- REQ-WC-1: A new `cmd/webclient/` binary MUST be created, separate from `cmd/frontend/`. It MUST share the existing `internal/storage/postgres/` packages (accounts, characters) and connect to the same PostgreSQL database.
- REQ-WC-2: The web server MUST use the Go standard library `net/http` (no external HTTP framework). It MUST listen on the port defined by `config.Web.Port` (YAML: `web.port`, default `8080`).
- REQ-WC-3: The server MUST serve the React application as static files. In production mode (`WEB_STATIC_DIR` env var unset), files MUST be served from an embedded `//go:embed ui/dist` filesystem. In dev mode (`WEB_STATIC_DIR` set), files MUST be served from that directory path, enabling hot-reload without binary rebuilds.
- REQ-WC-4: All non-API, non-WS routes (`/api/*` excluded) MUST serve `index.html` to support React client-side routing.
- REQ-WC-5: The server MUST connect to the gameserver via gRPC at `config.GameServer.Addr()` (same as the telnet frontend). The gRPC connection MUST be created at startup and shared across all WebSocket sessions.
- REQ-WC-6: A `WebConfig` struct MUST be added to `internal/config/config.go` with fields `Port int` (YAML: `port`, default 8080) and `JWTSecret string` (YAML: `jwt_secret`; fatal startup error if empty in production mode). Wire injection MUST include `WebConfig`.

### 2. Authentication API

- REQ-WC-7: `POST /api/auth/login` MUST accept `{"username": string, "password": string}`, verify credentials via `AccountStore.Authenticate`, and return `{"token": string, "account_id": int64, "role": string}` on success, or HTTP 401 on failure.
- REQ-WC-8: `POST /api/auth/register` MUST accept `{"username": string, "password": string}`, create the account via `AccountStore.Create`, and return the same token response. Username validation: 3–20 alphanumeric characters plus underscores. Password: minimum 8 characters. Return HTTP 400 with `{"error": string}` on validation failure.
- REQ-WC-9: JWT tokens MUST use HS256, carry claims `account_id int64`, `role string`, `exp int64` (Unix, 24h from issue), and be signed with `WebConfig.JWTSecret`.
- REQ-WC-10: All `/api/` routes except `/api/auth/login` and `/api/auth/register` MUST require a valid JWT in the `Authorization: Bearer <token>` header. Invalid or expired tokens MUST return HTTP 401.
- REQ-WC-11: `GET /api/auth/me` MUST return `{"account_id": int64, "username": string, "role": string}` for the authenticated user.

### 3. Character API

- REQ-WC-12: `GET /api/characters` MUST return the list of characters for the authenticated account as a JSON array, each entry containing `id`, `name`, `job`, `level`, `current_hp`, `max_hp`, `region`, `archetype`.
- REQ-WC-13: `POST /api/characters` MUST accept a character creation payload and create the character via existing `CharacterStore.Create`. Required fields: `name` (3–20 chars), `job`, `archetype`, `region`, `gender`. Return `{"character": {...}}` on success, HTTP 400 on validation failure, HTTP 409 if name already taken.
- REQ-WC-14: `GET /api/characters/options` MUST return the available regions, jobs, archetypes, and starting stats for the character creation wizard, loaded from the same ruleset data the telnet frontend uses.

### 4. WebSocket Game Session

- REQ-WC-15: `GET /ws` MUST upgrade to WebSocket if the request carries a valid JWT (passed as query parameter `?token=<JWT>` or `Authorization` header). The JWT MUST contain a `character_id` claim identifying which character to play. On upgrade failure (invalid JWT, character not owned by account), return HTTP 401 before upgrade.
- REQ-WC-16: On WebSocket connection, the server MUST establish a new `GameService_SessionClient` gRPC stream and send a `JoinWorldRequest` built from the character's stored metadata (same fields as the telnet frontend sends).
- REQ-WC-17: Two goroutines MUST be spawned per session:
  - **WS→gRPC:** reads text frames from the WebSocket, decodes `{"type": string, "payload": object}` JSON, marshals the payload into the appropriate `ClientMessage` proto using `protojson`, and calls `stream.Send()`.
  - **gRPC→WS:** reads `ServerEvent` from the gRPC stream, marshals to `{"type": string, "payload": object}` JSON using `protojson`, and writes text frames to the WebSocket.
- REQ-WC-18: The WebSocket MUST send periodic ping frames every 30 seconds. If a pong is not received within 10 seconds, the session MUST be closed and the gRPC stream cancelled.
- REQ-WC-19: On WebSocket close (client disconnect), the gRPC stream MUST be cancelled. On gRPC stream close (server-side disconnect, e.g. `Disconnected` event), the WebSocket MUST be closed with code 1001 (Going Away).
- REQ-WC-20: The `type` field in WebSocket messages MUST be the proto message name without package prefix (e.g. `"MoveRequest"`, `"RoomView"`, `"CombatEvent"`). The TypeScript client MUST use a discriminated union type to handle each message type.

### 5. React Application Structure

- REQ-WC-21: The React application MUST be in `cmd/webclient/ui/` and built with **Vite + React 18 + TypeScript**. Build output goes to `cmd/webclient/ui/dist/`. A `make ui-build` Makefile target MUST run `npm run build` in that directory.
- REQ-WC-22: TypeScript proto types MUST be generated from `api/proto/game/v1/game.proto` using `@bufbuild/protobuf` + `@connectrpc/connect`. A `make proto-ts` Makefile target MUST regenerate them into `cmd/webclient/ui/src/proto/`.
- REQ-WC-23: The application MUST have three top-level routes:
  - `/login` — unauthenticated; login and registration forms
  - `/characters` — authenticated; character list and character creation wizard
  - `/game` — authenticated, character selected; game session view
  - `/admin` — authenticated, role `admin` or `moderator`; admin dashboard
  - All routes except `/login` MUST redirect to `/login` if JWT is absent or expired.

### 6. Game UI Layout

- REQ-WC-24: The game view MUST use a responsive CSS Grid layout with the following named panels:
  - **Room** (top-left, ~40% width): room title, description, exits, NPCs, floor items
  - **Map** (top-right, ~20% width): ASCII map from `MapResponse`, zone name, legend
  - **Feed** (center, ~60% width × 60% height): scrollable message feed
  - **Character** (right, ~20% width): HP bar, AP display, active conditions
  - **Input** (bottom, full width): command text field + Send button
- REQ-WC-25: The **Room panel** MUST display: room title (styled header), description (paragraph), exits as clickable buttons that auto-submit a `MoveRequest`, NPCs as a list with names, and floor items as a list.
- REQ-WC-26: The **Map panel** MUST display the raw ASCII map string from `MapResponse` in a `<pre>` block with monospace font. A "Map" button in the panel header MUST send a `MapRequest` to refresh it.
- REQ-WC-27: The **Feed panel** MUST accumulate all `ServerEvent` messages as styled entries and scroll to the latest on new messages. Message types MUST be visually distinguished by color/icon:
  - `MessageEvent` (say/emote) — cyan
  - `CombatEvent` / `RoundStartEvent` / `RoundEndEvent` — red/orange
  - `RoomEvent` (arrival/departure) — dim/italic
  - `ErrorEvent` — bright red
  - `CharacterInfo`, `InventoryView`, `CharacterSheetView` — white/structured
  - System messages — yellow
- REQ-WC-28: The **Character panel** MUST display: character name, HP as a color-coded progress bar (green > 50%, yellow > 25%, red ≤ 25%), current/max HP numbers, active conditions as badge chips, and hero points.
- REQ-WC-29: The **Input panel** MUST contain: a text input field that auto-focuses on page load and after each command submission, a Send button, and command history navigation (↑/↓ arrow keys cycle through the last 100 commands). Pressing Enter MUST submit the command.
- REQ-WC-30: Commands typed in the input field MUST be parsed and dispatched as `ClientMessage` protos exactly as the telnet frontend does, using the same command-string format (e.g. `"move north"`, `"attack goblin"`, `"say Hello"`). The web client MUST NOT re-implement command parsing — a shared `internal/command/parse.go` function MUST be extracted and used by both frontends.
- REQ-WC-31: The game UI MUST support a **drawer panel** toggled by toolbar buttons for: Inventory (renders `InventoryView`), Equipment (renders current equipment from `CharacterSheetView`), Skills, and Feats. Drawers slide over the Feed panel. Only one drawer is open at a time.
- REQ-WC-32: The UI MUST display a **combat mode indicator** when a `RoundStartEvent` is received — a highlighted banner showing the round number and active combatants. The banner MUST be dismissed when `RoundEndEvent` is received.

### 7. Character Creation Wizard

- REQ-WC-33: The character creation wizard MUST be a multi-step form:
  - Step 1: Region selection (cards with lore description from `GET /api/characters/options`)
  - Step 2: Job selection (cards with stat preview)
  - Step 3: Archetype selection (cards filtered by chosen job)
  - Step 4: Name and gender input with live name availability check (`GET /api/characters/check-name?name=X`)
- REQ-WC-34: At each step, a sidebar MUST preview the character's starting stats based on current selections, updating live.
- REQ-WC-35: `GET /api/characters/check-name` MUST return `{"available": bool}`, checking uniqueness via `CharacterStore`.

### 8. Admin Interface

- REQ-WC-36: The admin dashboard MUST be accessible at `/admin` and require JWT role `admin` or `moderator`. All `/api/admin/*` routes MUST enforce the same role check (HTTP 403 if insufficient role).
- REQ-WC-37: **Online Players tab** — `GET /api/admin/players` MUST return all currently connected sessions (player name, character level, room ID, zone, current HP, account ID). The admin MUST be able to:
  - **Teleport** a player: `POST /api/admin/players/:char_id/teleport` with `{"room_id": string}` — sends a `TeleportRequest` via a dedicated admin gRPC stream
  - **Kick** a player: `POST /api/admin/players/:char_id/kick` — sends a `QuitRequest` on their behalf
  - **Message** a player: `POST /api/admin/players/:char_id/message` with `{"text": string}` — sends a `MessageEvent` to their stream
- REQ-WC-38: **Account Management tab** — `GET /api/admin/accounts?q=<search>` MUST search accounts by username prefix (case-insensitive). `PUT /api/admin/accounts/:id` MUST accept `{"role": string, "banned": bool}` and update the account. Banned accounts MUST be rejected at WebSocket upgrade with HTTP 403.
- REQ-WC-39: **Zone/Room Editor tab** — `GET /api/admin/zones` MUST return all loaded zones (id, name, danger_level, room_count). `GET /api/admin/zones/:zone_id/rooms` MUST return all rooms in the zone with exits. `PUT /api/admin/rooms/:room_id` MUST accept a partial room update (description, title, danger_level) and write it to the zone YAML file via `world.WorldEditor` (the same atomic write+hot-reload mechanism used by the editor-commands feature).
- REQ-WC-40: **NPC Spawner tab** — `POST /api/admin/rooms/:room_id/spawn-npc` MUST accept `{"npc_id": string, "count": int}` and send a `SpawnNPCRequest` via the admin gRPC stream. `GET /api/admin/npcs` MUST return all loaded NPC template IDs and names for the dropdown.
- REQ-WC-41: **Live Log tab** — `GET /api/admin/events` MUST be a Server-Sent Events (SSE) stream. The web server MUST maintain an in-process event bus; all `ServerEvent` messages from all active player sessions MUST be published to this bus. The SSE stream MUST support filtering by type via query param `?types=CombatEvent,MessageEvent`. The React UI MUST display events as a real-time scrolling log with type badges and timestamps.

### 9. Frontend Build & Dev Tooling

- REQ-WC-42: The `cmd/webclient/ui/` directory MUST contain a standard Vite project with `package.json`, `vite.config.ts`, `tsconfig.json`. Required dependencies: `react`, `react-dom`, `react-router-dom`, `@bufbuild/protobuf`, `@connectrpc/connect`. Dev dependencies: `vite`, `typescript`, `@vitejs/plugin-react`.
- REQ-WC-43: In dev mode, Vite's dev server (port 5173) MUST proxy `/api/*` and `/ws` to the Go server (port 8080) via `vite.config.ts` proxy configuration. This allows `npm run dev` for hot-module replacement without a full build.
- REQ-WC-44: A `make ui-install` Makefile target MUST run `npm install` in `cmd/webclient/ui/`. `make ui-build` MUST run `npm run build`. Both targets MUST be prerequisites of `make build` so the binary is never built without a fresh UI dist.
- REQ-WC-45: The `.gitignore` MUST exclude `cmd/webclient/ui/node_modules/` and `cmd/webclient/ui/dist/` (dist is embedded at build time, not committed).

### 10. Deployment & Configuration

- REQ-WC-46: The `configs/dev.yaml` MUST gain a `web:` section: `port: 8080`, `jwt_secret: dev-secret-change-in-prod`.
- REQ-WC-47: The Helm chart values (`deployments/k8s/mud/values.yaml`) MUST add a `webClient` service with `port: 8080`, `containerPort: 8080`, and the JWT secret as a Kubernetes secret reference.
- REQ-WC-48: The `cmd/webclient/main.go` MUST start the HTTP server and log the listening address. It MUST handle OS signals (SIGINT, SIGTERM) for graceful shutdown: stop accepting new connections, allow in-flight WebSocket sessions to complete their current command, then close after a 10-second drain timeout.
- REQ-WC-49: The `cmd/webclient/` binary MUST be built by `make build` alongside the existing `cmd/frontend/` and `cmd/gameserver/` binaries. The Dockerfile MUST produce a `webclient` image pushed to the registry.

---

## UI Wireframe (ASCII)

```
┌─────────────────────────────────────────────────────────────────────┐
│  🗺 Gunchete                           [Inventory] [Sheet] [Map] ⚙  │
├───────────────────────────┬─────────────┬──────────────────────────┤
│ ROOM                      │ MAP         │ CHARACTER                │
│ ─────────────────         │ ···[·]··    │ ─────────────────        │
│ The Rusty Nail            │ ·[·]·[·]·   │ Zork  Lv 5  Ganger      │
│                           │ ···[·]··    │ ████████░░░ 38/50 HP     │
│ A dive bar reeking of     │             │                          │
│ motor oil and regret.     │ Zone: Rust- │ ⚡ Bleeding              │
│                           │ bucket Ridge│ 🔥 On Fire               │
│ Exits: [N] [W] [E]        │             │                          │
│                           │ [Refresh]   │ ✦ Hero: 1               │
│ NPCs: Reg the Bartender   │             │                          │
│       Shady Stan          │             │                          │
├───────────────────────────┴─────────────┴──────────────────────────┤
│ FEED                                                                │
│ ─────────────────────────────────────────────────────────────────  │
│ [18:42] Reg says: "What'll it be?"                                  │
│ [18:43] You say: "Information."                                     │
│ [18:44] ⚔ Combat starts! Shady Stan draws a knife.                  │
│ [18:44] You strike Shady Stan for 7 damage.                         │
│ [18:44] Shady Stan slashes at you for 4 damage. (HP: 38/50)         │
│                                                                     │
│                                                             ▼ latest│
├─────────────────────────────────────────────────────────────────────┤
│ > _____________________________________________ [Send]               │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Dependencies

- `zone-content-expansion` / `editor-commands` — `WorldEditor` used by admin room editor (REQ-WC-39)
- No other feature dependencies — the web client is a new frontend layer over the existing gameserver

---

## Out of Scope

- Mobile native app (separate feature)
- WebRTC voice chat
- Graphical tile rendering (separate `game-client-ebiten` feature)
- OAuth / SSO login (username+password only in this version)

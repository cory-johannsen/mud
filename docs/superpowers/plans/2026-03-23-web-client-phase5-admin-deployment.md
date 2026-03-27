# Web Client Phase 5: Admin Interface & Deployment

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admin dashboard (players, accounts, zone editor, NPC spawner, live log) and full k8s deployment wiring.

**Architecture:** Role-gated admin API with SSE event bus, React tabbed admin UI, Helm chart updates for webclient service.

**Tech Stack:** Go net/http, SSE, React 18, TypeScript, Helm, Docker

---

## Requirements Covered

- REQ-WC-36: Admin dashboard at `/admin`, JWT role `admin` or `moderator`, all `/api/admin/*` routes enforce same role check (HTTP 403 if insufficient).
- REQ-WC-37: Online Players tab — list sessions, teleport/kick/message actions via admin gRPC stream.
- REQ-WC-38: Account Management tab — search by username prefix, update role/banned; banned accounts rejected at WebSocket upgrade with HTTP 403.
- REQ-WC-39: Zone/Room Editor tab — list zones, list rooms, inline room update via `world.WorldEditor`.
- REQ-WC-40: NPC Spawner tab — spawn NPC into room via admin gRPC stream; list NPC templates for dropdown.
- REQ-WC-41: Live Log tab — SSE stream of all `ServerEvent` messages from all active sessions, filterable by type.
- REQ-WC-46: `configs/dev.yaml` gains `web:` section (`port: 8080`, `jwt_secret: dev-secret-change-in-prod`).
- REQ-WC-47: `deployments/k8s/mud/values.yaml` gains `webClient` service entry; JWT secret via k8s Secret.
- REQ-WC-48: `cmd/webclient/main.go` starts HTTP server, logs listening address, handles SIGINT/SIGTERM with 10-second drain.
- REQ-WC-49: `make build` includes `build-webclient`; Dockerfile produces `webclient` image pushed to registry.

## Assumptions & Prerequisites

- Phases 1–4 are complete: `cmd/webclient/` binary exists with auth, character, WebSocket proxy, React scaffold, and session manager.
- The session manager (Phase 1) exposes `AllSessions() []Session` where each `Session` has `CharID`, `AccountID`, `PlayerName`, `Level`, `RoomID`, `Zone`, `CurrentHP`, and a method `SendAdminMessage(text string) error` and `Kick() error`.
- The gRPC admin stream for teleport and NPC spawn uses `GameService_AdminSessionClient` established once at startup and shared; if unavailable, handlers return HTTP 503.
- `world.WorldEditor` from the `editor-commands` feature is importable from `internal/world`.
- `internal/storage/postgres` provides `AccountStore.SearchByUsernamePrefix(ctx, prefix string) ([]Account, error)` and `AccountStore.UpdateRoleAndBanned(ctx, id int64, role string, banned bool) error`.
- The `//go:embed ui/dist` embed directive is already present in `cmd/webclient/main.go` from Phase 1.

---

## Task 1: EventBus

**File:** `cmd/webclient/eventbus/eventbus.go`

**Goal:** In-process pub/sub for fan-out of `ServerEvent` to all SSE subscribers.

### Steps

- [ ] 1.1 Write failing test in `cmd/webclient/eventbus/eventbus_test.go`:
  - Property: publish N events; all M subscribers each receive all N events in order.
  - Property: subscriber that falls behind (slow consumer) is dropped without blocking the bus after a configurable buffer fills.
  - Verify `Unsubscribe` removes the channel and no further events are delivered.
  - Run: `go test ./cmd/webclient/eventbus/... 2>&1` — expect compile or test failure.

- [ ] 1.2 Implement `cmd/webclient/eventbus/eventbus.go`:
  ```
  package eventbus

  // Event is a JSON-serialisable envelope.
  type Event struct {
      Type    string          // proto message name, e.g. "CombatEvent"
      Payload json.RawMessage // protojson-encoded ServerEvent payload
      Time    time.Time
  }

  type EventBus struct { ... }

  // New returns a running EventBus. bufSize is the per-subscriber channel buffer.
  func New(bufSize int) *EventBus

  // Subscribe returns a channel that receives published events.
  // The caller MUST call Unsubscribe when done.
  func (b *EventBus) Subscribe() (<-chan Event, func())

  // Publish sends e to all current subscribers.
  // Subscribers whose buffer is full are dropped (non-blocking send).
  func (b *EventBus) Publish(e Event)
  ```
  - Internal: RWMutex-protected map of subscriber ID → buffered channel.
  - `Publish` iterates under read lock; non-blocking send; if send would block, delete subscriber under write lock and close its channel.
  - `Unsubscribe` (the returned func) deletes from map and closes channel under write lock (idempotent).

- [ ] 1.3 Run tests: `go test -race ./cmd/webclient/eventbus/... 2>&1` — expect 100% pass.

- [ ] 1.4 Wire EventBus into session manager (Phase 1): after the gRPC→WS goroutine writes a frame, call `bus.Publish(eventbus.Event{Type: msgType, Payload: rawPayload, Time: time.Now()})`. The session manager must accept `*eventbus.EventBus` as a constructor argument.

- [ ] 1.5 Wire EventBus into `cmd/webclient/main.go` startup: `bus := eventbus.New(256)` and pass to session manager constructor.

- [ ] 1.6 Run full webclient tests: `go test -race ./cmd/webclient/... 2>&1` — expect 100% pass.

---

## Task 2: Admin Middleware

**File:** `cmd/webclient/middleware/admin.go`

**Goal:** HTTP middleware that enforces JWT role `admin` or `moderator` on all `/api/admin/*` routes.

### Steps

- [ ] 2.1 Write failing test in `cmd/webclient/middleware/admin_test.go`:
  - Property: requests with role `admin` pass through; role `moderator` passes through; role `player` returns HTTP 403; missing/invalid JWT returns HTTP 401.
  - Run: `go test ./cmd/webclient/middleware/... 2>&1` — expect failure.

- [ ] 2.2 Implement `RequireAdminRole(next http.Handler) http.Handler`:
  - Extract JWT claims from context (set by the auth middleware from Phase 2).
  - If claims absent: respond HTTP 401 `{"error":"unauthorized"}`.
  - If role not `admin` or `moderator`: respond HTTP 403 `{"error":"forbidden"}`.
  - Otherwise call `next.ServeHTTP`.

- [ ] 2.3 Register `/api/admin/` route prefix in `cmd/webclient/main.go` router: wrap the admin sub-router with both the existing JWT auth middleware and the new `RequireAdminRole` middleware.

- [ ] 2.4 Run tests: `go test -race ./cmd/webclient/middleware/... 2>&1` — expect 100% pass.

---

## Task 3: Admin Players API

**File:** `cmd/webclient/handlers/admin.go` (initial creation; subsequent tasks append to same file)

**Goal:** Implement `GET /api/admin/players`, `POST /api/admin/players/:char_id/teleport`, `POST /api/admin/players/:char_id/kick`, `POST /api/admin/players/:char_id/message`.

### Steps

- [ ] 3.1 Write failing tests in `cmd/webclient/handlers/admin_test.go`:
  - `GET /api/admin/players` with 0, 1, and 3 fake sessions in the session manager; verify JSON shape matches spec.
  - `POST /api/admin/players/:char_id/teleport` with valid and invalid `room_id`; verify session manager `Teleport` called; verify HTTP 404 when `char_id` not found.
  - `POST /api/admin/players/:char_id/kick`; verify session `Kick` called; verify HTTP 404 when not found.
  - `POST /api/admin/players/:char_id/message` with `{"text":"hello"}`; verify `SendAdminMessage` called; verify HTTP 400 on empty text.
  - Run: `go test ./cmd/webclient/handlers/... 2>&1` — expect failure.

- [ ] 3.2 Define `AdminHandler` struct in `cmd/webclient/handlers/admin.go`:
  ```go
  type AdminHandler struct {
      sessions SessionManager   // interface: AllSessions, GetSession, AdminGRPCStream
      accounts AccountStore
      world    WorldEditor
      bus      *eventbus.EventBus
  }
  ```

- [ ] 3.3 Implement `HandleListPlayers(w http.ResponseWriter, r *http.Request)`:
  - Call `sessions.AllSessions()`.
  - Map to `[]PlayerInfo{CharID, Name, Level, RoomID, Zone, CurrentHP, AccountID}`.
  - JSON encode and write HTTP 200.

- [ ] 3.4 Implement `HandleTeleportPlayer(w http.ResponseWriter, r *http.Request)`:
  - Extract `:char_id` from URL path.
  - Decode `{"room_id": string}` body; return HTTP 400 if missing.
  - Look up session; return HTTP 404 if not found.
  - Send `TeleportRequest{CharacterId: charID, RoomId: roomID}` over `sessions.AdminGRPCStream()`.
  - Return HTTP 200 `{}` on success, HTTP 503 if admin stream unavailable.

- [ ] 3.5 Implement `HandleKickPlayer(w http.ResponseWriter, r *http.Request)`:
  - Look up session; return HTTP 404 if not found.
  - Call `session.Kick()`; return HTTP 200 `{}`.

- [ ] 3.6 Implement `HandleMessagePlayer(w http.ResponseWriter, r *http.Request)`:
  - Decode `{"text": string}` body; return HTTP 400 if empty.
  - Look up session; return HTTP 404 if not found.
  - Call `session.SendAdminMessage(text)`; return HTTP 200 `{}`.

- [ ] 3.7 Register routes in `cmd/webclient/main.go`:
  ```
  mux.Handle("GET /api/admin/players", adminMW(http.HandlerFunc(ah.HandleListPlayers)))
  mux.Handle("POST /api/admin/players/{char_id}/teleport", adminMW(http.HandlerFunc(ah.HandleTeleportPlayer)))
  mux.Handle("POST /api/admin/players/{char_id}/kick", adminMW(http.HandlerFunc(ah.HandleKickPlayer)))
  mux.Handle("POST /api/admin/players/{char_id}/message", adminMW(http.HandlerFunc(ah.HandleMessagePlayer)))
  ```

- [ ] 3.8 Run tests: `go test -race ./cmd/webclient/handlers/... 2>&1` — expect 100% pass.

---

## Task 4: Admin Accounts API + Ban Enforcement

**Goal:** `GET /api/admin/accounts?q=`, `PUT /api/admin/accounts/:id`, ban check at WebSocket upgrade.

### Steps

- [ ] 4.1 Write failing tests in `cmd/webclient/handlers/admin_test.go`:
  - `GET /api/admin/accounts?q=foo` — mock store returns 2 accounts; verify JSON array with `id`, `username`, `role`, `banned`.
  - `PUT /api/admin/accounts/42` with `{"role":"moderator","banned":false}` — verify store `UpdateRoleAndBanned` called with correct args.
  - `PUT /api/admin/accounts/42` with invalid role string — verify HTTP 400.
  - WebSocket upgrade for a banned account — verify HTTP 403 before upgrade.
  - Run: `go test ./cmd/webclient/handlers/... 2>&1` — expect failure.

- [ ] 4.2 Add `AccountStore` interface requirement: `SearchByUsernamePrefix(ctx, prefix string) ([]Account, error)` and `UpdateRoleAndBanned(ctx, id int64, role string, banned bool) error`. If these methods do not yet exist in `internal/storage/postgres`, add them following TDD: write test → implement → verify.

- [ ] 4.3 Implement `HandleSearchAccounts(w http.ResponseWriter, r *http.Request)`:
  - Read `q` query param (empty string returns all, up to 100).
  - Call `accounts.SearchByUsernamePrefix(ctx, q)`.
  - Return `[]AccountInfo{ID, Username, Role, Banned}` as JSON.

- [ ] 4.4 Implement `HandleUpdateAccount(w http.ResponseWriter, r *http.Request)`:
  - Extract `:id` from path; parse as int64; return HTTP 400 on parse failure.
  - Decode body `{"role": string, "banned": bool}`.
  - Validate role is one of `"player"`, `"moderator"`, `"admin"`; return HTTP 400 otherwise.
  - Call `accounts.UpdateRoleAndBanned(ctx, id, role, banned)`.
  - Return HTTP 200 `{}`.

- [ ] 4.5 Enforce ban at WebSocket upgrade in `cmd/webclient/handlers/ws.go` (Phase 3):
  - After JWT validation, load account via `accounts.GetByID(ctx, accountID)`.
  - If `account.Banned` is true, respond HTTP 403 `{"error":"account banned"}` and return before upgrade.

- [ ] 4.6 Register routes:
  ```
  mux.Handle("GET /api/admin/accounts", adminMW(http.HandlerFunc(ah.HandleSearchAccounts)))
  mux.Handle("PUT /api/admin/accounts/{id}", adminMW(http.HandlerFunc(ah.HandleUpdateAccount)))
  ```

- [ ] 4.7 Run tests: `go test -race ./cmd/webclient/... 2>&1` — expect 100% pass.

---

## Task 5: Admin Zone/Room Editor API

**Goal:** `GET /api/admin/zones`, `GET /api/admin/zones/:zone_id/rooms`, `PUT /api/admin/rooms/:room_id`.

### Steps

- [ ] 5.1 Write failing tests in `cmd/webclient/handlers/admin_test.go`:
  - `GET /api/admin/zones` — mock world returns 2 zones; verify JSON `[{id, name, danger_level, room_count}]`.
  - `GET /api/admin/zones/zone1/rooms` — verify JSON array of rooms with exits.
  - `PUT /api/admin/rooms/room1` with `{"title":"New Title","description":"Desc","danger_level":2}` — verify `WorldEditor.UpdateRoom` called with correct args.
  - `PUT /api/admin/rooms/room1` with unknown `room_id` — verify HTTP 404.
  - Run: `go test ./cmd/webclient/handlers/... 2>&1` — expect failure.

- [ ] 5.2 Define `WorldEditor` interface used by `AdminHandler`:
  ```go
  type WorldEditor interface {
      AllZones() []ZoneSummary        // {ID, Name, DangerLevel, RoomCount}
      RoomsInZone(zoneID string) ([]RoomSummary, error)  // {ID, Title, Exits map[string]string}
      UpdateRoom(roomID string, patch RoomPatch) error    // {Title, Description, DangerLevel — zero value = no change}
  }
  ```
  Delegate to `internal/world.WorldEditor` methods. If exact method signatures differ, add thin adapter.

- [ ] 5.3 Implement `HandleListZones(w http.ResponseWriter, r *http.Request)`:
  - Call `world.AllZones()`; JSON encode; HTTP 200.

- [ ] 5.4 Implement `HandleListRooms(w http.ResponseWriter, r *http.Request)`:
  - Extract `:zone_id`; call `world.RoomsInZone(zoneID)`.
  - Return HTTP 404 if zone not found; otherwise JSON 200.

- [ ] 5.5 Implement `HandleUpdateRoom(w http.ResponseWriter, r *http.Request)`:
  - Extract `:room_id`; decode partial body.
  - Call `world.UpdateRoom(roomID, patch)`.
  - Return HTTP 404 if not found; HTTP 200 `{}` on success.

- [ ] 5.6 Register routes:
  ```
  mux.Handle("GET /api/admin/zones", adminMW(http.HandlerFunc(ah.HandleListZones)))
  mux.Handle("GET /api/admin/zones/{zone_id}/rooms", adminMW(http.HandlerFunc(ah.HandleListRooms)))
  mux.Handle("PUT /api/admin/rooms/{room_id}", adminMW(http.HandlerFunc(ah.HandleUpdateRoom)))
  ```

- [ ] 5.7 Run tests: `go test -race ./cmd/webclient/handlers/... 2>&1` — expect 100% pass.

---

## Task 6: Admin NPC Spawner API

**Goal:** `GET /api/admin/npcs`, `POST /api/admin/rooms/:room_id/spawn-npc`.

### Steps

- [ ] 6.1 Write failing tests:
  - `GET /api/admin/npcs` — mock world returns 3 NPC templates; verify `[{id, name}]` JSON.
  - `POST /api/admin/rooms/room1/spawn-npc` with `{"npc_id":"goblin","count":2}` — verify `SpawnNPCRequest` sent over admin gRPC stream.
  - `POST ...` with `count` ≤ 0 — verify HTTP 400.
  - `POST ...` with unknown `npc_id` — verify HTTP 404.
  - Run: `go test ./cmd/webclient/handlers/... 2>&1` — expect failure.

- [ ] 6.2 Add `AllNPCTemplates() []NPCTemplate` method to `WorldEditor` interface; implement via `internal/world` delegation.

- [ ] 6.3 Implement `HandleListNPCs(w http.ResponseWriter, r *http.Request)`:
  - Call `world.AllNPCTemplates()`; return `[{ID, Name}]` JSON.

- [ ] 6.4 Implement `HandleSpawnNPC(w http.ResponseWriter, r *http.Request)`:
  - Extract `:room_id`.
  - Decode `{"npc_id": string, "count": int}`; return HTTP 400 if `count` ≤ 0 or `npc_id` empty.
  - Verify NPC template exists; return HTTP 404 if not.
  - Send `SpawnNPCRequest{RoomId: roomID, NpcId: npcID, Count: int32(count)}` over admin gRPC stream.
  - Return HTTP 503 if stream unavailable; HTTP 200 `{}` on success.

- [ ] 6.5 Register routes:
  ```
  mux.Handle("GET /api/admin/npcs", adminMW(http.HandlerFunc(ah.HandleListNPCs)))
  mux.Handle("POST /api/admin/rooms/{room_id}/spawn-npc", adminMW(http.HandlerFunc(ah.HandleSpawnNPC)))
  ```

- [ ] 6.6 Run tests: `go test -race ./cmd/webclient/handlers/... 2>&1` — expect 100% pass.

---

## Task 7: Admin SSE Events Endpoint

**Goal:** `GET /api/admin/events` — SSE stream, filterable by `?types=`.

### Steps

- [ ] 7.1 Write failing tests in `cmd/webclient/handlers/admin_test.go`:
  - SSE handler: subscribe, publish 3 events of types `CombatEvent`, `MessageEvent`, `RoomEvent`; read from `httptest.ResponseRecorder` using `bufio.Scanner`; verify all 3 arrive as `data: {...}\n\n` lines.
  - With `?types=CombatEvent`, verify only 1 event arrives.
  - Client disconnect (close `r.Context()`): verify subscription is cleaned up (bus subscriber count decremented).
  - Run: `go test ./cmd/webclient/handlers/... 2>&1` — expect failure.

- [ ] 7.2 Implement `HandleAdminEvents(w http.ResponseWriter, r *http.Request)`:
  ```
  func (ah *AdminHandler) HandleAdminEvents(w http.ResponseWriter, r *http.Request) {
      // 1. Parse ?types= query param into a set; empty = all types.
      // 2. Assert w implements http.Flusher; return HTTP 500 if not.
      // 3. Set headers: Content-Type: text/event-stream, Cache-Control: no-cache, X-Accel-Buffering: no.
      // 4. ch, unsub := ah.bus.Subscribe(); defer unsub().
      // 5. Loop: select on ch and r.Context().Done().
      //    - On event: if types filter non-empty and event.Type not in filter, skip.
      //    - Otherwise: fmt.Fprintf(w, "data: %s\n\n", jsonEncode(event)); flusher.Flush().
      //    - On ctx done: return.
  }
  ```

- [ ] 7.3 Register route:
  ```
  mux.Handle("GET /api/admin/events", adminMW(http.HandlerFunc(ah.HandleAdminEvents)))
  ```

- [ ] 7.4 Run tests: `go test -race ./cmd/webclient/handlers/... 2>&1` — expect 100% pass.

---

## Task 8: React Admin UI

**Files:**
- `cmd/webclient/ui/src/pages/AdminPage.tsx`
- `cmd/webclient/ui/src/admin/PlayersTab.tsx`
- `cmd/webclient/ui/src/admin/AccountsTab.tsx`
- `cmd/webclient/ui/src/admin/ZoneEditorTab.tsx`
- `cmd/webclient/ui/src/admin/NpcSpawnerTab.tsx`
- `cmd/webclient/ui/src/admin/LiveLogTab.tsx`

**Goal:** Full admin dashboard as specified in REQ-WC-36 through REQ-WC-41.

### Steps

- [ ] 8.1 Create `cmd/webclient/ui/src/pages/AdminPage.tsx`:
  - Import all five tab components.
  - Render a tab bar with labels: "Online Players", "Accounts", "Zone Editor", "NPC Spawner", "Live Log".
  - State: `activeTab: string`; default `"players"`.
  - Conditionally render the active tab component below the tab bar.
  - Apply `role="admin"` check: if JWT role is not `admin` or `moderator`, render a "403 Forbidden" message and return early.
  - Add route `/admin` in `App.tsx` pointing to `AdminPage` (protected route, same pattern as `/game`).

- [ ] 8.2 Create `cmd/webclient/ui/src/admin/PlayersTab.tsx`:
  - On mount: `GET /api/admin/players`; populate table with columns: Name, Level, Zone, Room ID, HP, Account ID, Actions.
  - Actions column per row:
    - **Teleport** button: opens inline form with `room_id` text input; on submit calls `POST /api/admin/players/:char_id/teleport`; shows success/error toast.
    - **Kick** button: confirms via `window.confirm`; calls `POST /api/admin/players/:char_id/kick`; removes row on success.
    - **Message** button: opens inline form with `text` input; calls `POST /api/admin/players/:char_id/message`.
  - Auto-refresh every 10 seconds via `setInterval`; clear on unmount.

- [ ] 8.3 Create `cmd/webclient/ui/src/admin/AccountsTab.tsx`:
  - Search bar: debounced (300 ms) calls `GET /api/admin/accounts?q=<input>`.
  - Results table: columns Username, Role, Banned, Actions.
  - **Edit** button per row: inline dropdowns for Role (`player`, `moderator`, `admin`) and a Banned checkbox; **Save** button calls `PUT /api/admin/accounts/:id`; shows success/error toast.
  - Banned row: style with red background or strikethrough.

- [ ] 8.4 Create `cmd/webclient/ui/src/admin/ZoneEditorTab.tsx`:
  - Left panel: list of zones from `GET /api/admin/zones`; clicking a zone loads `GET /api/admin/zones/:zone_id/rooms` into the right panel.
  - Right panel: table of rooms with columns: Room ID, Title, Exits.
  - Clicking a room row expands an inline editor form with fields: Title (text), Description (textarea), Danger Level (number); **Save** calls `PUT /api/admin/rooms/:room_id`; shows success/error toast; closes form on success.

- [ ] 8.5 Create `cmd/webclient/ui/src/admin/NpcSpawnerTab.tsx`:
  - On mount: `GET /api/admin/npcs`; populate NPC dropdown.
  - Form fields: NPC (select), Count (number, min 1), Room ID (text input).
  - **Spawn** button: calls `POST /api/admin/rooms/:room_id/spawn-npc`; shows success/error toast.
  - Disable **Spawn** button while request in flight.

- [ ] 8.6 Create `cmd/webclient/ui/src/admin/LiveLogTab.tsx`:
  - Type filter: multi-select checkboxes for `CombatEvent`, `MessageEvent`, `RoomEvent`, `ErrorEvent`, and a catch-all "Other".
  - On mount: open `EventSource` to `GET /api/admin/events?types=<selected>`.
  - On `message` event: prepend to log array (newest first); cap at 500 entries.
  - Each entry: timestamp badge, type badge (color-coded matching Feed panel color scheme from REQ-WC-27), JSON payload summary.
  - **Clear** button: empties log array in state.
  - Reconnect on `EventSource` error (close + reopen after 2 s delay); log "[reconnecting...]" entry.
  - On filter change: close old `EventSource`; open new one with updated `?types=` param.
  - On unmount: close `EventSource`.

- [ ] 8.7 Build verification:
  ```
  cd cmd/webclient/ui && npm run build 2>&1
  ```
  Expect zero TypeScript errors and zero Vite build errors. Fix any type errors before proceeding.

---

## Task 9: Deployment Wiring

**Goal:** `configs/dev.yaml`, `values.yaml`, Helm templates, Makefile, Dockerfile for webclient.

### Steps

- [ ] 9.1 Edit `configs/dev.yaml` — append `web:` section:
  ```yaml
  web:
    port: 8080
    jwt_secret: dev-secret-change-in-prod
  ```

- [ ] 9.2 Edit `deployments/k8s/mud/values.yaml` — append `webClient:` block after `frontend:`:
  ```yaml
  webClient:
    repository: mud-webclient
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 256Mi
  ```
  The JWT secret is provided at deploy time via `--set webClient.jwtSecret=...` (never committed). Add to `values.yaml` comment:
  ```yaml
  # webClient.jwtSecret: ""  # required; pass via --set webClient.jwtSecret=...
  ```

- [ ] 9.3 Create `deployments/k8s/mud/templates/webclient/deployment.yaml`:
  ```yaml
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: webclient
    namespace: mud
  spec:
    replicas: 1
    strategy:
      type: RollingUpdate
      rollingUpdate:
        maxUnavailable: 0
        maxSurge: 1
    selector:
      matchLabels:
        app: webclient
    template:
      metadata:
        labels:
          app: webclient
      spec:
        containers:
          - name: webclient
            image: {{ .Values.image.registry }}/{{ .Values.webClient.repository }}:{{ .Values.image.tag }}
            imagePullPolicy: {{ .Values.image.pullPolicy }}
            ports:
              - containerPort: 8080
            env:
              - name: MUD_GAMESERVER_GRPC_HOST
                value: gameserver.mud.svc.cluster.local
              - name: MUD_GAMESERVER_GRPC_PORT
                value: "50051"
              - name: MUD_DATABASE_HOST
                value: postgres.mud.svc.cluster.local
              - name: MUD_DATABASE_PORT
                value: "5432"
              - name: MUD_DATABASE_USER
                valueFrom:
                  secretKeyRef:
                    name: mud-credentials
                    key: db-user
              - name: MUD_DATABASE_PASSWORD
                valueFrom:
                  secretKeyRef:
                    name: mud-credentials
                    key: db-password
              - name: MUD_DATABASE_NAME
                valueFrom:
                  secretKeyRef:
                    name: mud-credentials
                    key: db-name
              - name: MUD_WEB_JWT_SECRET
                valueFrom:
                  secretKeyRef:
                    name: mud-credentials
                    key: web-jwt-secret
              - name: MUD_LOGGING_LEVEL
                value: {{ .Values.logging.level | quote }}
              - name: MUD_LOGGING_FORMAT
                value: {{ .Values.logging.format | quote }}
            readinessProbe:
              httpGet:
                path: /api/health
                port: 8080
              initialDelaySeconds: 5
              periodSeconds: 5
            resources:
              {{- toYaml .Values.webClient.resources | nindent 12 }}
  ```

- [ ] 9.4 Create `deployments/k8s/mud/templates/webclient/service.yaml`:
  ```yaml
  apiVersion: v1
  kind: Service
  metadata:
    name: webclient
    namespace: mud
  spec:
    selector:
      app: webclient
    ports:
      - protocol: TCP
        port: 8080
        targetPort: 8080
    type: ClusterIP
  ```

- [ ] 9.5 Add `web-jwt-secret` key to the existing `deployments/k8s/mud/templates/secret.yaml` (or the equivalent secret template). The secret value MUST be passed at deploy time via `--set webClient.jwtSecret=...`. Add the key:
  ```yaml
  web-jwt-secret: {{ .Values.webClient.jwtSecret | b64enc | quote }}
  ```

- [ ] 9.6 Create `deployments/docker/Dockerfile.webclient`:
  ```dockerfile
  FROM node:22-alpine AS ui-builder
  WORKDIR /ui
  COPY cmd/webclient/ui/package.json cmd/webclient/ui/package-lock.json ./
  RUN npm ci
  COPY cmd/webclient/ui/ ./
  RUN npm run build

  FROM golang:1.26-alpine AS builder
  RUN apk add --no-cache git ca-certificates
  WORKDIR /build
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  COPY --from=ui-builder /ui/dist ./cmd/webclient/ui/dist
  ARG VERSION=dev
  RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-X github.com/cory-johannsen/mud/internal/version.Version=${VERSION}" \
      -o /bin/webclient ./cmd/webclient

  FROM gcr.io/distroless/static-debian12:nonroot
  COPY --from=builder /bin/webclient /bin/webclient
  COPY --from=builder /build/configs /configs
  COPY --from=builder /build/content /content
  EXPOSE 8080
  ENTRYPOINT ["/bin/webclient"]
  CMD ["-config", "/configs/dev.yaml"]
  ```

- [ ] 9.7 Edit `Makefile`:

  9.7a Add `build-webclient` target after `build-seed-claude-accounts`:
  ```makefile
  build-webclient: ui-build proto
  	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/webclient ./cmd/webclient
  ```

  9.7b Add `ui-install` and `ui-build` targets:
  ```makefile
  ui-install:
  	cd cmd/webclient/ui && npm install

  ui-build: ui-install
  	cd cmd/webclient/ui && npm run build
  ```

  9.7c Add `proto-ts` target:
  ```makefile
  proto-ts:
  	cd cmd/webclient/ui && npx buf generate ../../../../api/proto
  ```

  9.7d Extend the top-level `build` target to include `build-webclient`:
  ```makefile
  build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole build-seed-claude-accounts build-webclient
  ```

  9.7e Add webclient image to `docker-push`:
  ```makefile
  	docker build --build-arg VERSION=$(VERSION) -t $(REGISTRY)/mud-webclient:$(IMAGE_TAG) -f deployments/docker/Dockerfile.webclient .
  	docker push $(REGISTRY)/mud-webclient:$(IMAGE_TAG)
  ```

  9.7f Update `.PHONY` to include `build-webclient ui-install ui-build proto-ts`.

- [ ] 9.8 Verify Helm template renders without error:
  ```
  helm template mud deployments/k8s/mud \
    --set db.password=test \
    --set webClient.jwtSecret=test-secret \
    --namespace mud 2>&1 | head -60
  ```
  Expect: no error, YAML output includes `Deployment/webclient` and `Service/webclient`.

- [ ] 9.9 Verify Docker build (without pushing):
  ```
  docker build --build-arg VERSION=test -t mud-webclient:test -f deployments/docker/Dockerfile.webclient . 2>&1 | tail -10
  ```
  Expect: `Successfully built` (or `writing image ... done`). Fix any build errors before proceeding.

- [ ] 9.10 Update `deployments/k8s/mud/values-prod.yaml` (if it exists) or note in `values.yaml` comment: `webClient.jwtSecret` MUST be passed via `--set` or a sealed-secret; MUST NOT be committed in plaintext.

- [ ] 9.11 Add to `.gitignore`:
  ```
  cmd/webclient/ui/node_modules/
  cmd/webclient/ui/dist/
  ```

---

## Task 10: Final Integration Verification

**Goal:** End-to-end smoke test confirming all Phase 5 pieces work together.

### Steps

- [ ] 10.1 Run full test suite:
  ```
  make test-fast 2>&1
  ```
  Expect: 100% pass. Fix any failures before proceeding.

- [ ] 10.2 Run full build:
  ```
  make build 2>&1
  ```
  Expect: `bin/webclient` exists with nonzero size; React dist embedded.

- [ ] 10.3 Start the webclient binary locally and verify admin endpoints respond with auth enforcement:
  ```
  ./bin/webclient -config configs/dev.yaml &
  # Get a player token (role=player):
  curl -s -X POST http://localhost:8080/api/auth/login -d '{"username":"testplayer","password":"testpass"}' | jq .token
  # Attempt admin endpoint with player token — expect HTTP 403:
  curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $PLAYER_TOKEN" http://localhost:8080/api/admin/players
  # Get an admin token (role=admin):
  curl -s -X POST http://localhost:8080/api/auth/login -d '{"username":"admin","password":"adminpass"}' | jq .token
  # Attempt admin endpoint with admin token — expect HTTP 200:
  curl -s -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/api/admin/players
  # Kill webclient:
  kill %1
  ```

- [ ] 10.4 Verify SSE stream:
  ```
  curl -N -H "Authorization: Bearer $ADMIN_TOKEN" "http://localhost:8080/api/admin/events?types=MessageEvent" &
  # (events will appear in real time as players connect and chat)
  kill %1
  ```

- [ ] 10.5 Deploy to k8s:
  ```
  make k8s-redeploy 2>&1
  ```
  Verify `webclient` pod reaches `Running` state:
  ```
  kubectl get pods -n mud -l app=webclient
  ```

- [ ] 10.6 Commit all changes:
  - Stage: `configs/dev.yaml`, `deployments/`, `Makefile`, `cmd/webclient/handlers/admin.go`, `cmd/webclient/handlers/admin_test.go`, `cmd/webclient/eventbus/`, `cmd/webclient/middleware/admin.go`, `cmd/webclient/ui/src/pages/AdminPage.tsx`, `cmd/webclient/ui/src/admin/`, `.gitignore`.
  - Commit message: `feat(web-client): Phase 5 admin interface and deployment wiring (REQ-WC-36–41, REQ-WC-46–49)`.

---

## File Index

| Path | Action | Purpose |
|---|---|---|
| `cmd/webclient/eventbus/eventbus.go` | Create | In-process pub/sub for SSE fan-out |
| `cmd/webclient/eventbus/eventbus_test.go` | Create | Property tests for EventBus |
| `cmd/webclient/middleware/admin.go` | Create | Role-check middleware |
| `cmd/webclient/middleware/admin_test.go` | Create | Tests for admin middleware |
| `cmd/webclient/handlers/admin.go` | Create | All /api/admin/* handlers |
| `cmd/webclient/handlers/admin_test.go` | Create | Tests for admin handlers |
| `cmd/webclient/ui/src/pages/AdminPage.tsx` | Create | Tab container page |
| `cmd/webclient/ui/src/admin/PlayersTab.tsx` | Create | Online players management |
| `cmd/webclient/ui/src/admin/AccountsTab.tsx` | Create | Account search and edit |
| `cmd/webclient/ui/src/admin/ZoneEditorTab.tsx` | Create | Zone/room inline editor |
| `cmd/webclient/ui/src/admin/NpcSpawnerTab.tsx` | Create | NPC spawn form |
| `cmd/webclient/ui/src/admin/LiveLogTab.tsx` | Create | SSE live event log |
| `configs/dev.yaml` | Modify | Add `web:` section |
| `deployments/k8s/mud/values.yaml` | Modify | Add `webClient:` block |
| `deployments/k8s/mud/templates/webclient/deployment.yaml` | Create | k8s Deployment |
| `deployments/k8s/mud/templates/webclient/service.yaml` | Create | k8s Service |
| `deployments/k8s/mud/templates/secret.yaml` | Modify | Add `web-jwt-secret` key |
| `deployments/docker/Dockerfile.webclient` | Create | Multi-stage Docker build |
| `Makefile` | Modify | Add webclient build/push targets |
| `.gitignore` | Modify | Exclude node_modules and dist |

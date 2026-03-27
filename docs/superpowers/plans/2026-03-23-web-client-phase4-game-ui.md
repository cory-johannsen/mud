# Web Client Phase 4: Game UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Full game session view with Room, Map, Feed, Character, and Input panels, plus drawers and combat mode indicator.

**Architecture:** CSS Grid layout, GameContext managing WebSocket state, panel components consuming shared game state.

**Tech Stack:** React 18, TypeScript, CSS Grid, WebSocket API

**Requirements covered:** REQ-WC-24, REQ-WC-25, REQ-WC-26, REQ-WC-27, REQ-WC-28, REQ-WC-29, REQ-WC-30, REQ-WC-31, REQ-WC-32

**Assumes:** Phase 1–3 complete — Go backend running, React app scaffolded with auth and character selection, proto types generated under `cmd/webclient/ui/src/proto/`.

---

## Key Design Decisions

### REQ-WC-30: Command Parsing

REQ-WC-30 requires that command parsing NOT be re-implemented in the web client — a shared `internal/command/parse.go` function MUST be extracted. However, Phase 4 is a pure frontend phase. To preserve that scope boundary without duplicating logic, the InputPanel sends the raw command string to the Go WebSocket proxy as a `CommandRequest` type. The proxy then calls `internal/command/parse.go` to dispatch the correct `ClientMessage` proto. The web client MAY additionally pre-parse a small set of common commands (move, say, attack) client-side for UX purposes (instant clickable exits), but the authoritative parse path is always server-side. The plan includes a step for implementing the client-side shortcut dispatch only — the shared parse.go extraction is a separate backend task noted in Task 7.

### MapResponse ASCII Rendering

`MapResponse` does not contain a pre-rendered ASCII string. It contains `repeated MapTile` records with `(x, y)` grid coordinates. The MapPanel MUST render these into ASCII art client-side by iterating the tile grid.

### Drawer Scope

REQ-WC-31 lists Inventory, Equipment, Skills, and Feats drawers. Phase 4 implements Inventory and Equipment fully (proto messages exist for both). Skills and Feats drawers are included as stubs (rendering the raw proto data) to satisfy the REQ without deferring it.

---

## File Structure

```
cmd/webclient/ui/src/
  pages/
    GamePage.tsx                  (CSS Grid layout container)
  game/
    GameContext.tsx               (WebSocket connection, message dispatch, game state)
    CombatBanner.tsx              (RoundStartEvent / RoundEndEvent indicator)
    useCommandHistory.ts          (Hook: last 100 commands, ↑↓ navigation)
    panels/
      RoomPanel.tsx               (title, description, exits, NPCs, floor items)
      MapPanel.tsx                (ASCII map from MapTile grid, Refresh button)
      FeedPanel.tsx               (accumulating messages, color by type, auto-scroll)
      CharacterPanel.tsx          (HP bar, conditions, hero points)
      InputPanel.tsx              (text input, Send, command history ↑↓)
    drawers/
      DrawerContainer.tsx         (shared slide-over wrapper, one-open-at-a-time logic)
      InventoryDrawer.tsx         (renders InventoryView proto)
      EquipmentDrawer.tsx         (renders CharacterSheetView equipment fields)
      SkillsDrawer.tsx            (renders SkillsResponse / CharacterSheetView.skills stub)
      FeatsDrawer.tsx             (renders CharacterSheetView.feats stub)
  styles/
    game.css                      (CSS Grid layout definitions, panel styles)
```

---

## Task 1: GameContext — WebSocket connection, message dispatch, game state

- [ ] 1.1 Create `cmd/webclient/ui/src/game/GameContext.tsx`

**State managed by GameContext:**

```typescript
interface GameState {
  connected: boolean;
  roomView: RoomView | null;
  characterInfo: CharacterInfo | null;
  characterSheet: CharacterSheetView | null;
  inventoryView: InventoryView | null;
  mapTiles: MapTile[];
  feedEntries: FeedEntry[];
  combatRound: RoundStartEvent | null;  // non-null = in combat
}

interface FeedEntry {
  id: string;           // crypto.randomUUID()
  timestamp: Date;
  type: FeedEntryType;
  text: string;
  raw: ServerEvent;
}

type FeedEntryType =
  | 'message'
  | 'combat'
  | 'round_start'
  | 'round_end'
  | 'room_event'
  | 'error'
  | 'character_info'
  | 'system';
```

**Context shape:**

```typescript
interface GameContextValue {
  state: GameState;
  sendMessage: (type: string, payload: object) => void;
  sendCommand: (raw: string) => void;
}
```

**Implementation requirements:**

- REQ-CTX-1: `GameContext` MUST connect to `ws://<host>/ws?token=<JWT>` on mount. The JWT MUST be read from `localStorage` key `auth_token` (set by Phase 3 login flow).
- REQ-CTX-2: `sendMessage(type, payload)` MUST JSON-serialize `{"type": type, "payload": payload}` and call `ws.send()`. If the socket is not yet `OPEN`, the message MUST be queued and flushed on open.
- REQ-CTX-3: `sendCommand(raw)` MUST send `{"type": "CommandRequest", "payload": {"command": raw}}` via `sendMessage`. This is the primary dispatch path for all typed commands.
- REQ-CTX-4: Incoming frames MUST be parsed as `{"type": string, "payload": object}`. The dispatch table MUST handle: `RoomView`, `CharacterInfo`, `CharacterSheetView`, `InventoryView`, `MapResponse`, `MessageEvent`, `RoomEvent`, `CombatEvent`, `RoundStartEvent`, `RoundEndEvent`, `ErrorEvent`, `Disconnected`. Unknown types MUST be appended to feed as `system` entries.
- REQ-CTX-5: On `RoomView`, update `state.roomView` AND append a `room_event` feed entry showing the room title.
- REQ-CTX-6: On `RoundStartEvent`, set `state.combatRound`. On `RoundEndEvent`, set `state.combatRound` to `null`.
- REQ-CTX-7: On `Disconnected`, close WebSocket and navigate to `/characters`.
- REQ-CTX-8: WebSocket MUST reconnect automatically (exponential backoff: 1s, 2s, 4s, 8s, max 30s) on unexpected close (code !== 1000 and code !== 1001).
- REQ-CTX-9: `GameContext.Provider` MUST be rendered in `GamePage.tsx` and wrap all child panels.

**Verification:**
- `npm run build` passes with zero TypeScript errors.
- Manual: open `/game`, check browser DevTools Network tab for WebSocket upgrade at `/ws?token=...`, verify frames are received after joining.

---

## Task 2: CSS Grid layout — GamePage.tsx and game.css

- [ ] 2.1 Create `cmd/webclient/ui/src/styles/game.css`
- [ ] 2.2 Create `cmd/webclient/ui/src/pages/GamePage.tsx`

**CSS Grid layout (game.css):**

```
Named grid areas (desktop ≥1024px wide):
┌─────────────────────────────────────────────────────┐
│ toolbar (full width, ~40px)                         │
├──────────────────┬────────────┬─────────────────────┤
│ room (~40%)      │ map (~20%) │ character (~40%)    │
│                  │            │                     │
│ (min 200px)      │ (min 160px)│ (min 200px)         │
├──────────────────┴────────────┴─────────────────────┤
│ feed (full width, flex-grow, min 200px)             │
├─────────────────────────────────────────────────────┤
│ input (full width, ~60px)                           │
└─────────────────────────────────────────────────────┘

Grid template:
  grid-template-areas:
    "toolbar toolbar toolbar"
    "room    map     character"
    "feed    feed    feed"
    "input   input   input";
  grid-template-columns: 40fr 20fr 40fr;
  grid-template-rows: auto minmax(200px, 35vh) 1fr auto;
```

Responsive (< 1024px): stack vertically — toolbar, room, character, map, feed, input.

**GamePage.tsx requirements:**

- REQ-GP-1: `GamePage` MUST be a route-protected page component that redirects to `/login` if no JWT is present.
- REQ-GP-2: `GamePage` MUST render `GameContext.Provider` as its root.
- REQ-GP-3: The toolbar row MUST contain: zone name (from `state.roomView?.zone_name`), drawer toggle buttons (`[Inventory] [Equipment] [Skills] [Feats]`), and a settings icon.
- REQ-GP-4: `CombatBanner` MUST be rendered between the toolbar and the panel grid, conditionally when `state.combatRound !== null`.
- REQ-GP-5: `DrawerContainer` MUST overlay the Feed panel area. It MUST track which drawer is open via local `useState<DrawerType | null>` in `GamePage`, passed to `DrawerContainer` and the toolbar buttons.

**Verification:**
- `npm run build` passes.
- Manual: navigate to `/game`; confirm 5 grid areas render; confirm toolbar shows zone name once a `RoomView` arrives.

---

## Task 3: RoomPanel — title, description, clickable exits, NPCs, items

- [ ] 3.1 Create `cmd/webclient/ui/src/game/panels/RoomPanel.tsx`

**Props:** none (reads `state.roomView` from `GameContext`).

**Implementation requirements:**

- REQ-RP-1: If `state.roomView` is `null`, MUST render a placeholder: `<p className="room-loading">Connecting…</p>`.
- REQ-RP-2: Room title MUST be rendered as `<h2 className="room-title">`.
- REQ-RP-3: Description MUST be rendered as `<p className="room-description">`.
- REQ-RP-4: Exits MUST be rendered as `<div className="room-exits">` containing one `<button>` per exit in `RoomView.exits`. Button text is the direction label (capitalized). Hidden exits MUST NOT be rendered. Locked exits MUST render with a lock indicator (`direction*`). Clicking an exit MUST call `sendMessage('MoveRequest', { direction: exit.direction })`.
- REQ-RP-5: NPCs MUST be rendered as `<ul className="room-npcs">`. Each `NpcInfo` MUST show name and `health_description`. If `fighting_target` is non-empty, append ` ⚔` to the name.
- REQ-RP-6: Floor items MUST be rendered as `<ul className="room-items">`. Each `FloorItem` MUST show the item name. Clicking a floor item name MUST call `sendMessage('GetItemRequest', { target: item.name })`.
- REQ-RP-7: Other players in the room MUST be rendered as `<ul className="room-players">` showing each name from `RoomView.players`.

**Verification:**
- `npm run build` passes.
- Manual: enter a room; confirm title, description, exit buttons; click an exit button and confirm a `MoveRequest` WS frame is sent (check DevTools).

---

## Task 4: MapPanel — ASCII map rendering from MapTile grid

- [ ] 4.1 Create `cmd/webclient/ui/src/game/panels/MapPanel.tsx`

**Props:** none (reads `state.mapTiles` from `GameContext`).

**ASCII rendering algorithm:**

The map MUST be rendered by:
1. Finding the bounding box: `minX`, `maxX`, `minY`, `maxY` from all `MapTile.x` / `MapTile.y`.
2. Building a 2D character grid with one cell per tile coordinate. Each room occupies a 3×3 character block (walls + exits), separated by 1-char gutters. The current room (`tile.current === true`) renders as `[*]`; all others as `[ ]`. Boss rooms render as `[B]`.
3. Exit connections MUST be rendered as corridor characters between room blocks: `─` (east/west), `│` (north/south), `╱` (diagonal, if any).
4. The assembled grid MUST be joined to a single string and placed in `<pre className="map-ascii">`.

**Implementation requirements:**

- REQ-MP-1: `MapPanel` header MUST show "Map" as `<h3>` and a `<button onClick={refreshMap}>Refresh</button>` that calls `sendMessage('MapRequest', { view: 'zone' })`.
- REQ-MP-2: On mount, `MapPanel` MUST call `sendMessage('MapRequest', { view: 'zone' })` automatically via `useEffect` (fires once after connect).
- REQ-MP-3: If `state.mapTiles` is empty, MUST render `<p className="map-empty">No map data.</p>`.
- REQ-MP-4: The `<pre>` MUST have `style={{ fontFamily: 'monospace', fontSize: '0.7rem', lineHeight: '1', overflow: 'auto' }}`.
- REQ-MP-5: The ASCII rendering logic MUST live in a pure function `renderMapTiles(tiles: MapTile[]): string` in a sibling file `cmd/webclient/ui/src/game/mapRenderer.ts` (testable independently).

**Verification:**
- `npm run build` passes.
- Manual: open Map panel; click Refresh; confirm ASCII map renders with at least the current room visible as `[*]`.

---

## Task 5: CharacterPanel — HP bar, conditions, hero points

- [ ] 5.1 Create `cmd/webclient/ui/src/game/panels/CharacterPanel.tsx`

**Props:** none (reads `state.characterInfo` and `state.characterSheet` from `GameContext`).

**Implementation requirements:**

- REQ-CP-1: If both `characterInfo` and `characterSheet` are `null`, MUST render `<p>Loading…</p>`.
- REQ-CP-2: Character name MUST be rendered as `<h3 className="char-name">`. Job and level MUST appear below as `<span className="char-class">`.
- REQ-CP-3: HP bar MUST be `<div className="hp-bar-track"><div className="hp-bar-fill" /></div>`. The fill width MUST be `(currentHp / maxHp) * 100` percent. The fill color class MUST be:
  - `hp-green` when `currentHp / maxHp > 0.5`
  - `hp-yellow` when `currentHp / maxHp > 0.25`
  - `hp-red` when `currentHp / maxHp <= 0.25`
- REQ-CP-4: HP numbers MUST be rendered as `<span>{currentHp} / {maxHp} HP</span>` below the bar.
- REQ-CP-5: Active conditions MUST be sourced from `state.roomView?.active_conditions` (type `ConditionInfo[]`). Each condition MUST render as `<span className="condition-badge">{condition.name}</span>`.
- REQ-CP-6: Hero points MUST render as `<span className="hero-points">✦ Hero: {heroPoints}</span>` using `characterSheet.hero_points`. If `characterSheet` is `null`, hero points MUST NOT render.
- REQ-CP-7: Action points MUST render if `characterInfo` is present (sourced from the `RoundStartEvent.actions_per_turn` stored in `state.combatRound`). Outside combat, this section MUST be hidden.

**Verification:**
- `npm run build` passes.
- Manual: join game; confirm HP bar renders with correct color; confirm conditions appear after a `RoundStartEvent`.

---

## Task 6: FeedPanel — accumulating messages, color by type, auto-scroll

- [ ] 6.1 Create `cmd/webclient/ui/src/game/panels/FeedPanel.tsx`

**Props:** none (reads `state.feedEntries` from `GameContext`).

**Color mapping (CSS classes applied to each feed row):**

| `FeedEntryType`          | CSS class           | Visual                      |
|--------------------------|---------------------|-----------------------------|
| `message`                | `feed-message`      | cyan text                   |
| `combat`                 | `feed-combat`       | red text                    |
| `round_start`            | `feed-round-start`  | orange text, bold           |
| `round_end`              | `feed-round-end`    | orange text, dim            |
| `room_event`             | `feed-room-event`   | dim gray, italic            |
| `error`                  | `feed-error`        | bright red text, bold       |
| `character_info`         | `feed-structured`   | white text                  |
| `system`                 | `feed-system`       | yellow text                 |

**Implementation requirements:**

- REQ-FP-1: Feed MUST be a `<div ref={scrollRef} className="feed-scroll">` containing one `<div className={`feed-entry ${entry.type}`}>` per `FeedEntry`.
- REQ-FP-2: Each entry MUST include a timestamp prefix: `[HH:MM]` using `entry.timestamp`.
- REQ-FP-3: Auto-scroll MUST use a `useEffect` that calls `scrollRef.current.scrollTop = scrollRef.current.scrollHeight` whenever `feedEntries.length` changes.
- REQ-FP-4: Auto-scroll MUST be suppressed when the user has manually scrolled up (detect via `onScroll`: if `scrollTop < scrollHeight - clientHeight - 50`, set a `userScrolled` ref to `true`; reset to `false` when user scrolls to the bottom manually).
- REQ-FP-5: `MessageEvent` entries MUST render as `{sender}: {content}`. `CombatEvent` entries MUST render `narrative` if non-empty, else `{attacker} → {target}: {damage} dmg`. `RoundStartEvent` entries MUST render `⚔ Round {round} — turn order: {turn_order.join(', ')}`. `RoomEvent` entries MUST render `{player} arrived` / `{player} left`. `ErrorEvent` entries MUST render `⚠ {message}`.
- REQ-FP-6: Feed entries MUST be capped at 500. When the limit is exceeded, the oldest entries MUST be discarded (implemented in `GameContext` reducer, not in `FeedPanel`).

**Verification:**
- `npm run build` passes.
- Manual: say something in-game; confirm a cyan feed entry appears and the panel scrolls to bottom. Scroll up manually; confirm new messages do not force-scroll.

---

## Task 7: InputPanel — text input, Send, command history

- [ ] 7.1 Create `cmd/webclient/ui/src/game/useCommandHistory.ts`
- [ ] 7.2 Create `cmd/webclient/ui/src/game/panels/InputPanel.tsx`

**useCommandHistory.ts:**

```typescript
// Hook managing a capped ring buffer of the last 100 commands.
// Returns: { history, push, navigateUp, navigateDown, currentValue, reset }
```

- REQ-IH-1: `push(cmd)` MUST prepend the command to the front of the history array and cap the array at 100 entries.
- REQ-IH-2: `navigateUp()` MUST move the cursor toward older entries, returning the entry at the new cursor position.
- REQ-IH-3: `navigateDown()` MUST move the cursor toward newer entries, returning the entry at the new cursor position. When the cursor passes the newest entry, MUST return `''` (empty — the live input state).
- REQ-IH-4: `reset()` MUST reset the cursor to position `-1` (no history selected).

**InputPanel.tsx:**

- REQ-IP-1: The panel MUST render `<form onSubmit={handleSubmit}><input .../><button>Send</button></form>`.
- REQ-IP-2: The input MUST `autoFocus` on mount. After each submit, `inputRef.current.focus()` MUST be called.
- REQ-IP-3: `onKeyDown` MUST handle:
  - `ArrowUp` → `navigateUp()`, set input value to returned history entry, prevent default (stops cursor movement).
  - `ArrowDown` → `navigateDown()`, set input value to returned entry or `''`.
  - `Enter` (without Shift) → submit form.
- REQ-IP-4: On submit: if input is non-empty, call `sendCommand(value)`, call `history.push(value)`, clear the input, call `history.reset()`.
- REQ-IP-5: `sendCommand` MUST additionally pre-parse move shortcuts: if the command matches `^(n|s|e|w|north|south|east|west|up|down|northeast|northwest|southeast|southwest|ne|nw|se|sw)$` (case-insensitive), dispatch `sendMessage('MoveRequest', { direction: normalizedDir })` directly instead of a `CommandRequest`. All other commands MUST go through `sendCommand` as `CommandRequest`.

> **Note (REQ-WC-30):** Extracting `internal/command/parse.go` as a shared library is a required backend task. It MUST be tracked as a separate work item and implemented before Phase 5 (admin UI), so the WebSocket proxy can call the shared parse function server-side. The client-side shortcut in REQ-IP-5 is a UX convenience only for the most common commands.

**Verification:**
- `npm run build` passes.
- Manual: type `north`, press Enter; confirm `MoveRequest` WS frame. Press ↑; confirm previous command appears. Press ↑ again; confirm older command. Press ↓ to return to empty.

---

## Task 8: Drawer system — Inventory, Equipment, Skills, Feats

- [ ] 8.1 Create `cmd/webclient/ui/src/game/drawers/DrawerContainer.tsx`
- [ ] 8.2 Create `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`
- [ ] 8.3 Create `cmd/webclient/ui/src/game/drawers/EquipmentDrawer.tsx`
- [ ] 8.4 Create `cmd/webclient/ui/src/game/drawers/SkillsDrawer.tsx`
- [ ] 8.5 Create `cmd/webclient/ui/src/game/drawers/FeatsDrawer.tsx`

**DrawerContainer.tsx:**

Props: `openDrawer: 'inventory' | 'equipment' | 'skills' | 'feats' | null`, `onClose: () => void`.

- REQ-DR-1: `DrawerContainer` MUST position itself as `position: absolute; inset: 0; z-index: 10` relative to the Feed panel grid area. When `openDrawer` is `null`, MUST render nothing (`return null`).
- REQ-DR-2: The container MUST render a backdrop `<div className="drawer-backdrop" onClick={onClose}>` and the active drawer as a slide-in panel from the right (`transform: translateX(0)` when open, `translateX(100%)` when closing, with CSS transition).
- REQ-DR-3: A close button `✕` MUST be rendered in the drawer header.

**InventoryDrawer.tsx:**

- REQ-INV-1: On open (when `openDrawer === 'inventory'`), MUST dispatch `sendMessage('InventoryRequest', {})` via `useEffect` if `state.inventoryView` is `null`.
- REQ-INV-2: MUST render a table with columns: Name, Kind, Qty, Weight. Summary row: `{usedSlots}/{maxSlots} slots`, `{totalWeight.toFixed(1)} kg`, `{currency}`.

**EquipmentDrawer.tsx:**

- REQ-EQ-1: On open, MUST dispatch `sendMessage('CharacterSheetRequest', {})` if `state.characterSheet` is `null`.
- REQ-EQ-2: MUST render slots: Main Hand (`main_hand` + `main_hand_attack_bonus` + `main_hand_damage`), Off Hand, Armor (map entries from `armor`), Accessories (map entries from `accessories`). Empty slots MUST show `—`.

**SkillsDrawer.tsx (stub):**

- REQ-SK-1: On open, MUST dispatch `sendMessage('CharacterSheetRequest', {})` if `state.characterSheet` is `null`.
- REQ-SK-2: MUST render `characterSheet.skills` as a table: Skill, Ability, Proficiency, Bonus. If no sheet, show `<p>Loading…</p>`.

**FeatsDrawer.tsx (stub):**

- REQ-FT-1: On open, MUST dispatch `sendMessage('CharacterSheetRequest', {})` if `state.characterSheet` is `null`.
- REQ-FT-2: MUST render `characterSheet.feats` as a list showing feat name and description. If no sheet, show `<p>Loading…</p>`.

**Verification:**
- `npm run build` passes.
- Manual: click `[Inventory]` in toolbar; confirm drawer slides over feed panel; confirm `InventoryRequest` WS frame sent; confirm item list renders. Click `✕`; confirm drawer closes. Click `[Equipment]`; confirm only Equipment drawer is open.

---

## Task 9: CombatBanner — RoundStartEvent / RoundEndEvent indicator

- [ ] 9.1 Create `cmd/webclient/ui/src/game/CombatBanner.tsx`

**Props:** none (reads `state.combatRound` from `GameContext`).

- REQ-CB-1: If `state.combatRound` is `null`, MUST render nothing (`return null`).
- REQ-CB-2: When `state.combatRound` is set, MUST render a full-width `<div className="combat-banner">` between the toolbar and the panel grid (see `GamePage.tsx` Task 2).
- REQ-CB-3: Banner content MUST include:
  - `⚔ COMBAT — Round {combatRound.round}` as a `<strong>` element.
  - Turn order as `<span>Turn order: {combatRound.turn_order.join(' → ')}</span>`.
  - Actions per turn as `<span>{combatRound.actions_per_turn} actions</span>`.
- REQ-CB-4: Banner MUST use a high-contrast style: `background: #8b0000; color: #fff; padding: 6px 16px; font-size: 0.9rem; display: flex; gap: 16px; align-items: center`.
- REQ-CB-5: Banner MUST animate in via CSS: `animation: bannerSlideIn 0.2s ease-out` (defined in `game.css`).

**Verification:**
- `npm run build` passes.
- Manual: trigger combat (attack an NPC); confirm red banner appears with round number and turn order. Confirm banner disappears when combat ends (after `RoundEndEvent`).

---

## Completion Criteria

All 9 tasks complete when:

- [ ] `npm run build` exits 0 with zero TypeScript errors (run from `cmd/webclient/ui/`).
- [ ] All 9 task manual verification steps pass against a live local dev server.
- [ ] No `TODO`, placeholder, or incomplete code exists in any created file (AGENT-1, AGENT-2).
- [ ] All created files are under ~400 lines each; logic exceeding that MUST be split into helper modules.

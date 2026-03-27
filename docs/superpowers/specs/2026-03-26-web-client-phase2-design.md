# Web Client Phase 2 — Tiled Scene Design

**Date:** 2026-03-26
**Status:** approved
**Scope:** Adds a PixiJS tiled scene panel to the existing React web client (Phase 1)

---

## Overview

Phase 2 refactors the web client's Room panel to include a tile-based 2D scene rendered with PixiJS (WebGL), matching the visual presentation of the Ebiten client. The scene sits **above** the existing room text (title, description, exits, NPCs) inside the Room panel — the overall page grid layout is unchanged. Assets are loaded from the same versioned asset pack used by the Ebiten client, fetched from GitHub Releases at page load before the login screen.

A new shared Go sub-package `internal/client/assets` provides the GitHub Releases version-check logic used by both the Ebiten client and the web server's asset proxy endpoint.

---

## Architecture

```
Browser (React/TS)
  │
  ├── AssetPackContext (page load, before login)
  │     ├── GET /api/assets/version → AssetVersion
  │     ├── Compare vs localStorage cached version
  │     ├── If mismatch: download zip → verify SHA-256 → extract → IndexedDB
  │     └── Expose { status, progress, textures: PixiTextureMap, tilesConfig }
  │
  ├── ScenePanel (PixiJS canvas via React ref, inside Room panel)
  │     ├── BackgroundLayer  — zone background sprite
  │     ├── NpcLayer         — up to 6 NPC sprites + count badge
  │     ├── PlayerLayer      — player sprite anchored bottom-center
  │     ├── ExitLayer        — N/S/E/W indicators, clickable → session.Send
  │     └── AnimationLayer   — CombatAnimationQueue per sprite
  │
  └── Room panel (unchanged Phase 1 text: title, description, exits, NPCs)

cmd/webclient (Go)
  └── GET /api/assets/version
        └── internal/client/assets.FetchLatestVersion → JSON response
```

---

## Requirements

### 1. `internal/client/assets` Sub-Package

- REQ-WC2-1: A new `internal/client/assets` sub-package MUST be added to the shared client library. It MUST follow the same dependency rule as all `internal/client` sub-packages: import only the Go standard library.
- REQ-WC2-2: `assets.AssetVersion` MUST be a struct with fields `Version int`, `DownloadURL string`, `SHA256URL string`.
- REQ-WC2-3: `assets.FetchLatestVersion(ctx context.Context, releasesURL string) (*AssetVersion, error)` MUST query the GitHub Releases API at `releasesURL`, parse the latest release, locate the artifact whose name matches `mud-assets-v{N}.zip`, and return an `AssetVersion`. It MUST return `ErrNoRelease` if no matching artifact is found, and `ErrNetwork` (mirroring the type defined in `internal/client/auth`) on HTTP/connection failure.
- REQ-WC2-4: `assets.ParseVersion(s string) (int, error)` MUST parse a version integer from a `version.txt` string (e.g. `"7\n"` → `7`).
- REQ-WC2-5: `assets.Client` MUST be tested against an `httptest.Server` stub. `ParseVersion` MUST be tested with table-driven tests covering valid input, leading/trailing whitespace, and invalid input.

### 2. Go Asset Proxy Endpoint

- REQ-WC2-6: `cmd/webclient` MUST expose `GET /api/assets/version` (authenticated: valid JWT required). It MUST call `assets.FetchLatestVersion` using the releases URL from `WebConfig.GitHubReleasesURL` (new field; YAML: `github_releases_url`; fatal startup error if empty). It MUST return `{"version": int, "download_url": string, "sha256_url": string}` on success, HTTP 502 with `{"error": string}` on upstream failure.
- REQ-WC2-7: `WebConfig` MUST gain a `GitHubReleasesURL string` field (YAML: `github_releases_url`). `configs/dev.yaml` MUST set it to `https://api.github.com/repos/cory-johannsen/mud/releases/latest`.

### 3. Asset Pack Loading (TypeScript)

- REQ-WC2-8: An `AssetPackContext` React context MUST manage the full asset lifecycle. It MUST be initialised at application root (above all routes including `/login`) so the download screen is shown before the login screen.
- REQ-WC2-9: `AssetPackContext` load sequence:
  1. Fetch `GET /api/assets/version` (no JWT required at this stage — endpoint MUST be exempted from auth).
  2. Read cached version from `localStorage` key `mud-asset-version`.
  3. If versions match and IndexedDB contains the extracted assets: load textures → ready.
  4. If version mismatch or cache absent: display download progress screen; fetch zip from `download_url`; verify SHA-256 against `sha256_url`; extract into IndexedDB; store new version in `localStorage`; proceed.
  5. If network unavailable and cache present: log warning, proceed with cached assets.
  6. If network unavailable and no cache: display `AssetErrorScreen` with a Retry button; do NOT proceed to login.
- REQ-WC2-10: `AssetPackContext` MUST expose `{ status: AssetStatus, progress: number, textures: PixiTextureMap, tilesConfig: TilesConfig }`. `AssetStatus` is `"loading" | "downloading" | "ready" | "error"`. `progress` is 0–100 during download.
- REQ-WC2-11: `TilesConfig` MUST be parsed from the `tiles.yaml` in the asset pack. It MUST map zone categories, NPC types, and item categories to sprite sheet paths and tile coordinates `{x, y, w, h}`. Animation fps per NPC category MUST default to `12` if absent.
- REQ-WC2-12: `PixiTextureMap` MUST be a `Map<string, PIXI.Texture>` keyed by sprite sheet path, loaded via `PIXI.Assets.load`. All sprite sheets listed in `tiles.yaml` MUST be loaded eagerly on asset pack ready.

### 4. Room Panel Layout

- REQ-WC2-13: The Room panel MUST be split into a vertical two-part layout:
  - **Scene sub-panel** (~60% of Room panel height): PixiJS canvas.
  - **Text sub-panel** (~40% of Room panel height): unchanged Phase 1 room text (title, description, clickable exit buttons, NPC list, floor items).
- REQ-WC2-14: The overall CSS Grid layout (Room | Map | Feed | Character | Input panels) MUST be unchanged. Only the Room panel's internal structure changes.
- REQ-WC2-15: When `AssetPackContext.status` is not `"ready"`, the Scene sub-panel MUST be hidden and the Text sub-panel MUST expand to fill the full Room panel height (graceful degradation during loading).

### 5. PixiJS Scene Component

- REQ-WC2-16: A `ScenePanel` React component MUST mount a PixiJS `Application` on a `<canvas>` element via `useRef` on mount, and destroy it on unmount. It MUST consume `AssetPackContext` for textures and `TilesConfig`.
- REQ-WC2-17: `ScenePanel` MUST maintain four internal PixiJS layers (as `PIXI.Container` children of the stage): `BackgroundLayer`, `NpcLayer`, `PlayerLayer`, `ExitLayer`. A fifth `AnimationLayer` sits above all others for combat animations.
- REQ-WC2-18: **BackgroundLayer** — one `PIXI.Sprite` per zone background. Zone name from `RoomView.zone_name` MUST be resolved to a zone category via `TilesConfig`. Unrecognised zone names MUST fall back to the `default` tileset entry.
- REQ-WC2-19: **NpcLayer** — up to 6 `PIXI.Sprite` objects, evenly spaced across the scene width. If `RoomView.npcs` contains more than 6 entries, the 6th sprite MUST display a `PIXI.Text` count badge showing `+{N}` additional NPCs. NPC category MUST be resolved via NPC name lookup in `TilesConfig`; unknown NPCs MUST use the `default-npc` sprite.
- REQ-WC2-20: **PlayerLayer** — one `PIXI.Sprite` anchored bottom-center of the scene.
- REQ-WC2-21: **ExitLayer** — up to 4 exit indicator sprites (N=top-center, S=bottom-center, E=right-center, W=left-center) for each exit present in `RoomView.exits`. Each indicator MUST be `eventMode: "static"` with a `pointerdown` handler that calls `session.Send("move {direction}")`.
- REQ-WC2-22: `ScenePanel` MUST subscribe to `session.Events()` and handle:
  - `RoomView` → rebuild all four layers from the new room state.
  - `CombatEvent` (ATTACK) → enqueue attack animation on attacker sprite, enqueue hit-flash on target sprite.
  - `CombatEvent` (DEATH) → enqueue death animation on target sprite; sprite MUST remain until a subsequent `RoomView` omits that NPC.

### 6. Combat Animations

- REQ-WC2-23: A `CombatAnimationQueue` TypeScript class MUST manage a per-sprite animation queue. Animations MUST play sequentially — a new animation on a sprite MUST not interrupt the current one but MUST be queued. `enqueue(spriteId: string, type: AnimationType): void` MUST be the public API.
- REQ-WC2-24: Three `AnimationType` values MUST be supported: `"attack"`, `"hit-flash"`, `"death"`.
  - `attack`: play `PIXI.AnimatedSprite` using the NPC category's attack frame strip at the fps from `TilesConfig`; return to idle frame on completion.
  - `hit-flash`: overlay a white-tinted `PIXI.Sprite` for 80ms; remove on completion.
  - `death`: play the death frame strip; hold the last frame until the sprite is removed by a `RoomView` update.
- REQ-WC2-25: The player sprite MUST play a hit-flash when `CombatEvent.target` matches `session.State().Character.Name`.
- REQ-WC2-26: Animation fps for each NPC category MUST be read from `TilesConfig`. The default fps if absent in `tiles.yaml` MUST be 12.

### 7. Feature Split in Feature Index

- REQ-WC2-27: The existing `web-client` feature entry (Phase 1) MUST remain unchanged in scope and status.
- REQ-WC2-28: A new `web-client-phase2` feature entry MUST be added to `docs/features/index.yaml` at priority 431, status `spec`, with dependencies `web-client` and `internal-client`.
- REQ-WC2-29: `game-client-ebiten` dependencies MUST be updated to add `web-client-phase2` (the shared asset pack format and GitHub release workflow originate here).
- REQ-WC2-30: The `internal-client` feature entry MUST be updated to document `internal/client/assets` as a 6th sub-package.

---

## Updated `internal/client` Package Structure

```
internal/client/
  assets/     — GitHub Releases version check; AssetVersion type; ParseVersion
  auth/       — HTTP client: login, register, character list/create/options
  feed/       — ServerEvent accumulation, color token assignment, cap enforcement
  history/    — command ring buffer (↑/↓ navigation, in-memory only)
  render/     — renderer interfaces + color token constants
  session/    — gRPC session lifecycle + state machine
```

---

## New Dependencies

- `pixi.js` v8 — WebGL 2D renderer (added to `cmd/webclient/ui/package.json`)
- `js-yaml` — parse `tiles.yaml` in the browser (added to `cmd/webclient/ui/package.json`)
- No new Go module dependencies

---

## Out of Scope

- Inventory / equipment scene overlays (future iteration)
- Audio / sound effects
- Map panel sprites (ASCII map from Phase 1 unchanged)
- Mobile layout adjustments
- Admin interface changes

# Spec: Web Game Client — Phase 2 (PixiJS Tiled Room Scene)

**GitHub Issue:** cory-johannsen/mud#13
**Date:** 2026-04-12
**Supersedes:** `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md`
**See also:** `docs/superpowers/specs/2026-04-13-svg-map-rework.md` (map work split to cory-johannsen/mud#51)

---

## Overview

Phase 2 adds a PixiJS tiled scene to the Room panel — sprite-based background, NPC, player, and exit layers with combat animations. The map improvements (combat grid, SVG zone/world maps) were split to cory-johannsen/mud#51.

---

## Requirements

All requirements carry forward from the approved design in `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md` without modification. The full requirement set (REQ-WC2-1 through REQ-WC2-24) in that document remains the canonical spec. Summary:

### REQ-PC-1: `internal/client/assets` sub-package (`= REQ-WC2-1..5`)

- REQ-PC-1a: New `internal/client/assets` package, standard-library-only dependencies
- REQ-PC-1b: `AssetVersion` struct: `Version int`, `DownloadURL string`, `SHA256URL string`
- REQ-PC-1c: `FetchLatestVersion(ctx, releasesURL)` — queries GitHub Releases API, returns `AssetVersion` or `ErrNoRelease`/`ErrNetwork`
- REQ-PC-1d: `ParseVersion(s string) (int, error)` — parses version int from `version.txt` string
- REQ-PC-1e: All functions tested against `httptest.Server`

### REQ-PC-2: Go asset proxy endpoint (`= REQ-WC2-6..7`)

- REQ-PC-2a: `GET /api/assets/version` (auth-exempt) — calls `FetchLatestVersion`, returns `{"version", "download_url", "sha256_url"}`
- REQ-PC-2b: `WebConfig.GitHubReleasesURL` field; `configs/dev.yaml` sets it to the GitHub releases API URL

### REQ-PC-3: Asset pack loading — TypeScript (`= REQ-WC2-8..12`)

- REQ-PC-3a: `AssetPackContext` initialised at app root, above all routes
- REQ-PC-3b: Load sequence: fetch version → compare localStorage cache → download+verify+extract if mismatch → load textures
- REQ-PC-3c: Exposes `{ status: AssetStatus, progress: number, textures: PixiTextureMap, tilesConfig: TilesConfig }`
- REQ-PC-3d: `TilesConfig` parsed from `tiles.yaml` in asset pack
- REQ-PC-3e: `AssetErrorScreen` shown when no network and no cache

### REQ-PC-4: Room panel layout (`= REQ-WC2-13..15`)

- REQ-PC-4a: Room panel split: Scene sub-panel (~60%) + Text sub-panel (~40%)
- REQ-PC-4b: Overall CSS Grid layout unchanged
- REQ-PC-4c: Scene sub-panel hidden when `AssetPackContext.status !== "ready"`

### REQ-PC-5: `ScenePanel` React component (`= REQ-WC2-16..22`)

- REQ-PC-5a: Mounts PixiJS `Application` on `<canvas>` via `useRef`; destroys on unmount
- REQ-PC-5b: Four layers: `BackgroundLayer`, `NpcLayer`, `PlayerLayer`, `ExitLayer` + `AnimationLayer`
- REQ-PC-5c: Background resolves zone name → zone category via `TilesConfig`; defaults to `default` tileset
- REQ-PC-5d: NPC layer: up to 6 sprites evenly spaced; 7th+ shown as `+N` badge
- REQ-PC-5e: Exit indicators: N/S/E/W sprites, `pointerdown` → `session.Send("move {direction}")`
- REQ-PC-5f: Subscribes to `RoomView` (rebuild layers) and `CombatEvent` (enqueue animations)

### REQ-PC-6: `CombatAnimationQueue` (`= REQ-WC2-23..24`)

- REQ-PC-6a: Per-sprite sequential animation queue; `enqueue(spriteId, type)` public API
- REQ-PC-6b: Three types: `attack` (animated sprite strip), `hit-flash` (white overlay 80ms), `death` (death strip, hold last frame)

---

## Files to Modify

- `internal/client/assets/` (new package)
- `cmd/webclient/` — asset proxy endpoint, `WebConfig.GitHubReleasesURL`, `configs/dev.yaml`
- `cmd/webclient/ui/src/AssetPackContext.tsx` (new)
- `cmd/webclient/ui/src/AssetErrorScreen.tsx` (new)
- `cmd/webclient/ui/src/game/ScenePanel.tsx` (new)
- `cmd/webclient/ui/src/game/CombatAnimationQueue.ts` (new)
- `cmd/webclient/ui/src/game/panels/RoomPanel.tsx` — split into Scene + Text sub-panels

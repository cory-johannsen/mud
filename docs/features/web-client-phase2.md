# Web Game Client — Phase 2 (Tiled Scene)

**Slug:** web-client-phase2
**Status:** spec
**Priority:** 431
**Category:** ui
**Effort:** XL

## Overview

Phase 2 refactors the web client's Room panel to include a tile-based 2D scene rendered with PixiJS (WebGL), matching the visual style of the Ebiten client. The scene sits above the existing room text inside the Room panel. Assets are loaded from the same versioned asset pack used by the Ebiten client, fetched from GitHub Releases at page load before the login screen.

## What's New vs Phase 1

| Concern | Phase 1 | Phase 2 |
|---|---|---|
| Room panel | Text only (title, description, exits) | Scene canvas (top) + text (bottom) |
| Asset loading | None | Asset pack from GitHub Releases, cached in IndexedDB |
| NPC rendering | Text list | PixiJS sprites, up to 6 |
| Combat | Feed messages only | Feed messages + sprite animations |
| Renderer | React/CSS | PixiJS (WebGL) inside React ref |

## Architecture

See `docs/superpowers/specs/2026-03-26-web-client-phase2-design.md` for the full design.

## New Shared Code

`internal/client/assets` — GitHub Releases version-check logic shared with the Ebiten client.

## Dependencies

- `web-client` — Phase 1 must be complete; all Go endpoints and React shell reused
- `internal-client` — shared session, feed, render contract, and new assets sub-package

# Web Client Phase 2 — Tiled Scene Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a PixiJS WebGL tiled scene panel to the existing React web client, loading sprites from the shared Ebiten asset pack fetched from GitHub Releases at page load.

**Architecture:** `internal/client/assets` provides GitHub Releases version-check logic shared by both Go clients. The Go web server proxies the version check via `GET /api/assets/version`. The React app downloads, caches (IndexedDB), and serves the asset pack via `AssetPackContext`; `ScenePanel` mounts a PixiJS canvas inside the Room panel above the existing text content; `CombatAnimationQueue` manages per-sprite frame animations driven by `CombatEvent` messages.

**Tech Stack:** Go 1.26, PixiJS v8, js-yaml, Vitest (TypeScript unit tests), `pgregory.net/rapid` (Go property tests), `net/http/httptest` (Go HTTP tests).

> **Prerequisite:** `web-client` Phase 1 (`cmd/webclient/`) must be fully implemented before this plan. All TypeScript tasks assume the Phase 1 React app exists at `cmd/webclient/ui/src/`.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/client/assets/assets.go` | Create | `AssetVersion` type, `FetchLatestVersion`, `ParseVersion`, `ErrNoRelease`, `ErrNetwork` |
| `internal/client/assets/assets_test.go` | Create | httptest + table-driven tests |
| `internal/config/config.go` | Modify | Add `WebConfig.GitHubReleasesURL` field |
| `configs/dev.yaml` | Modify | Add `web.github_releases_url` |
| `cmd/webclient/main.go` | Modify | Wire `/api/assets/version` handler; add `GitHubReleasesURL` validation |
| `cmd/webclient/ui/package.json` | Modify | Add `pixi.js`, `js-yaml`, `@types/js-yaml`, `vitest`, `@vitest/coverage-v8`, `jsdom` |
| `cmd/webclient/ui/vite.config.ts` | Modify | Add Vitest config block |
| `cmd/webclient/ui/src/client/assets/types.ts` | Create | `AssetVersion`, `TilesConfig`, `TileCoord`, `NpcTile`, `AnimTile`, `PixiTextureMap` TypeScript types |
| `cmd/webclient/ui/src/client/assets/tilesConfig.ts` | Create | `parseTilesConfig(yaml: string): TilesConfig` |
| `cmd/webclient/ui/src/client/assets/tilesConfig.test.ts` | Create | Vitest unit tests for `parseTilesConfig` |
| `cmd/webclient/ui/src/client/assets/idb.ts` | Create | Thin IndexedDB helpers: `storeAssetFiles`, `loadAssetFiles`, `clearAssetFiles` |
| `cmd/webclient/ui/src/client/assets/AssetPackContext.tsx` | Create | React context: download, verify, cache, expose textures + TilesConfig |
| `cmd/webclient/ui/src/client/assets/AssetDownloadScreen.tsx` | Create | Progress/error screen shown before login |
| `cmd/webclient/ui/src/client/scene/CombatAnimationQueue.ts` | Create | Per-sprite animation queue: `enqueue`, sequential playback |
| `cmd/webclient/ui/src/client/scene/CombatAnimationQueue.test.ts` | Create | Vitest unit tests (mocked PixiJS) |
| `cmd/webclient/ui/src/client/scene/ScenePanel.tsx` | Create | PixiJS Application in React ref; four layers + AnimationLayer |
| `cmd/webclient/ui/src/components/RoomPanel.tsx` | Modify | Split into scene sub-panel (top) + text sub-panel (bottom) |

---

## Task 1: `internal/client/assets` — Go Asset Version Package

**Files:**
- Create: `internal/client/assets/assets.go`
- Create: `internal/client/assets/assets_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/client/assets/assets_test.go
package assets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cory-johannsen/mud/internal/client/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatestVersion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v7",
			"assets": []map[string]any{
				{"name": "mud-assets-v7.zip", "browser_download_url": "https://example.com/mud-assets-v7.zip"},
				{"name": "mud-assets-v7.sha256", "browser_download_url": "https://example.com/mud-assets-v7.sha256"},
			},
		})
	}))
	defer srv.Close()

	v, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, 7, v.Version)
	assert.Equal(t, "https://example.com/mud-assets-v7.zip", v.DownloadURL)
	assert.Equal(t, "https://example.com/mud-assets-v7.sha256", v.SHA256URL)
}

func TestFetchLatestVersion_NoMatchingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1",
			"assets":   []map[string]any{},
		})
	}))
	defer srv.Close()

	_, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	require.ErrorIs(t, err, assets.ErrNoRelease)
}

func TestFetchLatestVersion_NetworkError(t *testing.T) {
	_, err := assets.FetchLatestVersion(context.Background(), "http://127.0.0.1:1")
	var ne assets.ErrNetwork
	require.ErrorAs(t, err, &ne)
}

func TestFetchLatestVersion_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	require.Error(t, err)
	require.NotErrorIs(t, err, assets.ErrNoRelease)
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"7", 7, false},
		{"7\n", 7, false},
		{"  7  \n", 7, false},
		{"42", 42, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-1", 0, true},
	}
	for _, tc := range cases {
		got, err := assets.ParseVersion(tc.input)
		if tc.wantErr {
			require.Error(t, err, "input=%q", tc.input)
		} else {
			require.NoError(t, err, "input=%q", tc.input)
			assert.Equal(t, tc.want, got, "input=%q", tc.input)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
mise exec -- go test ./internal/client/assets/... -v 2>&1 | head -10
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Implement the assets package**

```go
// internal/client/assets/assets.go
package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ErrNoRelease is returned when the GitHub Releases API returns a release
// with no mud-assets-v{N}.zip artifact.
var ErrNoRelease = errors.New("no asset release found")

// ErrNetwork is returned when the HTTP request itself fails.
type ErrNetwork struct{ Cause error }

func (e ErrNetwork) Error() string { return "network error: " + e.Cause.Error() }
func (e ErrNetwork) Unwrap() error { return e.Cause }

// AssetVersion describes a published asset pack release.
type AssetVersion struct {
	Version     int
	DownloadURL string
	SHA256URL   string
}

// FetchLatestVersion queries the GitHub Releases API at releasesURL, locates the
// mud-assets-v{N}.zip artifact, and returns the version and download URLs.
// Returns ErrNoRelease if the release has no matching artifact.
// Returns ErrNetwork on HTTP/connection failure.
func FetchLatestVersion(ctx context.Context, releasesURL string) (*AssetVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, ErrNetwork{Cause: err}
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ErrNetwork{Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrNetwork{Cause: err}
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("parse github releases response: %w", err)
	}

	var zipURL, sha256URL string
	var version int
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, ".zip") && strings.HasPrefix(a.Name, "mud-assets-v") {
			zipURL = a.BrowserDownloadURL
			// Extract version from "mud-assets-v{N}.zip"
			inner := strings.TrimPrefix(a.Name, "mud-assets-v")
			inner = strings.TrimSuffix(inner, ".zip")
			v, err := strconv.Atoi(inner)
			if err != nil {
				return nil, fmt.Errorf("parse asset version from filename %q: %w", a.Name, err)
			}
			version = v
		}
		if strings.HasSuffix(a.Name, ".sha256") && strings.HasPrefix(a.Name, "mud-assets-v") {
			sha256URL = a.BrowserDownloadURL
		}
	}

	if zipURL == "" {
		return nil, ErrNoRelease
	}

	return &AssetVersion{
		Version:     version,
		DownloadURL: zipURL,
		SHA256URL:   sha256URL,
	}, nil
}

// ParseVersion parses a version integer from a version.txt string.
// Leading/trailing whitespace is trimmed. Returns an error for empty,
// non-numeric, or negative values.
func ParseVersion(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty version string")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", s, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("version must be non-negative, got %d", v)
	}
	return v, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
mise exec -- go test ./internal/client/assets/... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/assets/assets.go internal/client/assets/assets_test.go
git commit -m "feat(client/assets): add GitHub Releases version-check package"
```

---

## Task 2: Go Config + `/api/assets/version` Endpoint

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/dev.yaml`
- Modify: `cmd/webclient/main.go`

**Prerequisite:** Phase 1 `cmd/webclient/` must exist. This task adds one field to `WebConfig` and wires one HTTP handler.

- [ ] **Step 1: Add `GitHubReleasesURL` to `WebConfig`**

In `internal/config/config.go`, locate the `WebConfig` struct added in Phase 1 and add the new field:

```go
// WebConfig holds web client server settings.
type WebConfig struct {
	Port              int    `mapstructure:"port"`
	JWTSecret         string `mapstructure:"jwt_secret"`
	GitHubReleasesURL string `mapstructure:"github_releases_url"`
}
```

- [ ] **Step 2: Add the URL to `configs/dev.yaml`**

Add the following under the existing `web:` block in `configs/dev.yaml`:

```yaml
web:
  port: 8080
  jwt_secret: dev-secret-change-in-prod
  github_releases_url: https://api.github.com/repos/cory-johannsen/mud/releases/latest
```

- [ ] **Step 3: Add validation for the new field**

In `internal/config/config.go`, locate the `validateWeb` function (or `Validate` method on `WebConfig`) added in Phase 1. Add a check:

```go
if c.Web.GitHubReleasesURL == "" {
    errs = append(errs, "web.github_releases_url is required")
}
```

- [ ] **Step 4: Write a failing test for the new endpoint**

Add to the Phase 1 web server test file (e.g. `cmd/webclient/assets_handler_test.go`):

```go
// cmd/webclient/assets_handler_test.go
package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetsVersionHandler_Success(t *testing.T) {
	// Stub the GitHub Releases upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v3",
			"assets": []map[string]any{
				{"name": "mud-assets-v3.zip", "browser_download_url": "https://cdn.example.com/mud-assets-v3.zip"},
				{"name": "mud-assets-v3.sha256", "browser_download_url": "https://cdn.example.com/mud-assets-v3.sha256"},
			},
		})
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL) // helper from Phase 1 test suite
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/version", nil)
	req.Header.Set("Authorization", "Bearer "+validJWT(t)) // helper from Phase 1 test suite
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Version     int    `json:"version"`
		DownloadURL string `json:"download_url"`
		SHA256URL   string `json:"sha256_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 3, body.Version)
	assert.Equal(t, "https://cdn.example.com/mud-assets-v3.zip", body.DownloadURL)
}

func TestAssetsVersionHandler_UpstreamFailure(t *testing.T) {
	srv := newTestServer(t, "http://127.0.0.1:1") // unreachable upstream
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/version", nil)
	req.Header.Set("Authorization", "Bearer "+validJWT(t))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestAssetsVersionHandler_RequiresNoAuth(t *testing.T) {
	// Per REQ-WC2-9: /api/assets/version is exempted from JWT auth
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1",
			"assets": []map[string]any{
				{"name": "mud-assets-v1.zip", "browser_download_url": "https://cdn.example.com/mud-assets-v1.zip"},
				{"name": "mud-assets-v1.sha256", "browser_download_url": "https://cdn.example.com/mud-assets-v1.sha256"},
			},
		})
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/assets/version", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 5: Run tests to confirm they fail**

```bash
mise exec -- go test ./cmd/webclient/... -run TestAssetsVersion -v 2>&1 | head -20
```

Expected: FAIL — handler not registered yet.

- [ ] **Step 6: Wire the handler in `cmd/webclient/main.go`**

In the HTTP router setup (wherever Phase 1 registers routes), add:

```go
// GET /api/assets/version — no auth required (called before login screen)
mux.HandleFunc("GET /api/assets/version", func(w http.ResponseWriter, r *http.Request) {
    av, err := assets.FetchLatestVersion(r.Context(), cfg.Web.GitHubReleasesURL)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadGateway)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "version":      av.Version,
        "download_url": av.DownloadURL,
        "sha256_url":   av.SHA256URL,
    })
})
```

Add the import: `"github.com/cory-johannsen/mud/internal/client/assets"`

Also ensure `/api/assets/version` is in the JWT exemption list alongside `/api/auth/login` and `/api/auth/register`.

- [ ] **Step 7: Run tests to confirm they pass**

```bash
mise exec -- go test ./cmd/webclient/... -v -count=1 -timeout=30s
```

Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go configs/dev.yaml cmd/webclient/
git commit -m "feat(webclient): add /api/assets/version endpoint and WebConfig.GitHubReleasesURL"
```

---

## Task 3: TypeScript Foundation — Types, TilesConfig Parser, Vitest

**Files:**
- Modify: `cmd/webclient/ui/package.json`
- Modify: `cmd/webclient/ui/vite.config.ts`
- Create: `cmd/webclient/ui/src/client/assets/types.ts`
- Create: `cmd/webclient/ui/src/client/assets/tilesConfig.ts`
- Create: `cmd/webclient/ui/src/client/assets/tilesConfig.test.ts`

- [ ] **Step 1: Add dependencies to `package.json`**

In `cmd/webclient/ui/package.json`, add to `dependencies`:
```json
"pixi.js": "^8.0.0",
"js-yaml": "^4.1.0"
```

Add to `devDependencies`:
```json
"@types/js-yaml": "^4.0.9",
"vitest": "^2.0.0",
"@vitest/coverage-v8": "^2.0.0",
"jsdom": "^25.0.0",
"@vitest/ui": "^2.0.0"
```

Add to `scripts`:
```json
"test": "vitest run",
"test:watch": "vitest",
"test:coverage": "vitest run --coverage"
```

- [ ] **Step 2: Add Vitest config to `vite.config.ts`**

```typescript
// cmd/webclient/ui/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
    },
  },
})
```

- [ ] **Step 3: Install dependencies**

```bash
cd cmd/webclient/ui && npm install
```

Expected: `node_modules/pixi.js`, `node_modules/js-yaml`, `node_modules/vitest` all present.

- [ ] **Step 4: Write the failing TilesConfig test**

```typescript
// cmd/webclient/ui/src/client/assets/tilesConfig.test.ts
import { describe, it, expect } from 'vitest'
import { parseTilesConfig } from './tilesConfig'

const SAMPLE_YAML = `
zones:
  rustbucket:
    sheet: tilesets/zones/rustbucket.png
    tile: {x: 0, y: 0, w: 320, h: 240}
  default:
    sheet: tilesets/zones/default.png
    tile: {x: 0, y: 0, w: 320, h: 240}

npcs:
  ganger:
    sheet: tilesets/npcs/ganger.png
    idle: {x: 0, y: 0, w: 32, h: 48}
    fps: 8
  default-npc:
    sheet: tilesets/npcs/default.png
    idle: {x: 0, y: 0, w: 32, h: 48}

animations:
  ganger:
    attack:
      sheet: animations/combat/ganger-attack.png
      frames: [{x: 0, y: 0, w: 32, h: 48}, {x: 32, y: 0, w: 32, h: 48}]
    death:
      sheet: animations/combat/ganger-death.png
      frames: [{x: 0, y: 0, w: 32, h: 48}]
`

describe('parseTilesConfig', () => {
  it('parses zone tiles', () => {
    const cfg = parseTilesConfig(SAMPLE_YAML)
    expect(cfg.zones['rustbucket'].sheet).toBe('tilesets/zones/rustbucket.png')
    expect(cfg.zones['rustbucket'].tile).toEqual({ x: 0, y: 0, w: 320, h: 240 })
  })

  it('parses npc tiles with fps', () => {
    const cfg = parseTilesConfig(SAMPLE_YAML)
    expect(cfg.npcs['ganger'].sheet).toBe('tilesets/npcs/ganger.png')
    expect(cfg.npcs['ganger'].fps).toBe(8)
  })

  it('defaults fps to 12 when absent', () => {
    const cfg = parseTilesConfig(SAMPLE_YAML)
    expect(cfg.npcs['default-npc'].fps).toBe(12)
  })

  it('parses animation frame strips', () => {
    const cfg = parseTilesConfig(SAMPLE_YAML)
    expect(cfg.animations['ganger'].attack.frames).toHaveLength(2)
    expect(cfg.animations['ganger'].attack.frames[0]).toEqual({ x: 0, y: 0, w: 32, h: 48 })
  })

  it('throws on invalid yaml', () => {
    expect(() => parseTilesConfig('{')).toThrow()
  })

  it('throws when zones key missing', () => {
    expect(() => parseTilesConfig('npcs: {}')).toThrow(/zones/)
  })
})
```

- [ ] **Step 5: Run tests to confirm they fail**

```bash
cd cmd/webclient/ui && npm test 2>&1 | head -20
```

Expected: FAIL — `tilesConfig.ts` does not exist yet.

- [ ] **Step 6: Create the types and parser**

```typescript
// cmd/webclient/ui/src/client/assets/types.ts
import type * as PIXI from 'pixi.js'

export interface TileCoord {
  x: number
  y: number
  w: number
  h: number
}

export interface ZoneTile {
  sheet: string
  tile: TileCoord
}

export interface NpcTile {
  sheet: string
  idle: TileCoord
  fps: number
}

export interface AnimFrames {
  sheet: string
  frames: TileCoord[]
}

export interface NpcAnimations {
  attack?: AnimFrames
  death?: AnimFrames
}

export interface TilesConfig {
  zones: Record<string, ZoneTile>
  npcs: Record<string, NpcTile>
  animations: Record<string, NpcAnimations>
}

export type PixiTextureMap = Map<string, PIXI.Texture>

export type AssetStatus = 'loading' | 'downloading' | 'ready' | 'error'

export interface AssetVersion {
  version: number
  downloadUrl: string
  sha256Url: string
}
```

```typescript
// cmd/webclient/ui/src/client/assets/tilesConfig.ts
import yaml from 'js-yaml'
import type { TilesConfig, NpcTile } from './types'

export function parseTilesConfig(raw: string): TilesConfig {
  const doc = yaml.load(raw) as Record<string, unknown>

  if (!doc || typeof doc !== 'object') {
    throw new Error('tiles.yaml: expected a YAML object at root')
  }
  if (!doc['zones'] || typeof doc['zones'] !== 'object') {
    throw new Error('tiles.yaml: missing required "zones" key')
  }
  if (!doc['npcs'] || typeof doc['npcs'] !== 'object') {
    throw new Error('tiles.yaml: missing required "npcs" key')
  }

  const rawNpcs = doc['npcs'] as Record<string, Omit<NpcTile, 'fps'> & { fps?: number }>
  const npcs: TilesConfig['npcs'] = {}
  for (const [name, tile] of Object.entries(rawNpcs)) {
    npcs[name] = { ...tile, fps: tile.fps ?? 12 }
  }

  return {
    zones: doc['zones'] as TilesConfig['zones'],
    npcs,
    animations: (doc['animations'] ?? {}) as TilesConfig['animations'],
  }
}
```

- [ ] **Step 7: Run tests to confirm they pass**

```bash
cd cmd/webclient/ui && npm test 2>&1
```

Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/webclient/ui/package.json cmd/webclient/ui/vite.config.ts \
        cmd/webclient/ui/package-lock.json \
        cmd/webclient/ui/src/client/assets/types.ts \
        cmd/webclient/ui/src/client/assets/tilesConfig.ts \
        cmd/webclient/ui/src/client/assets/tilesConfig.test.ts
git commit -m "feat(webclient/ui): add TilesConfig parser, asset types, Vitest setup"
```

---

## Task 4: `AssetPackContext` — Asset Lifecycle Management

**Files:**
- Create: `cmd/webclient/ui/src/client/assets/idb.ts`
- Create: `cmd/webclient/ui/src/client/assets/AssetPackContext.tsx`
- Create: `cmd/webclient/ui/src/client/assets/AssetDownloadScreen.tsx`
- Modify: `cmd/webclient/ui/src/main.tsx` (wrap app in `AssetPackProvider`)

- [ ] **Step 1: Create the IndexedDB helper**

```typescript
// cmd/webclient/ui/src/client/assets/idb.ts

const DB_NAME = 'mud-assets'
const DB_VERSION = 1
const STORE_NAME = 'files'

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      req.result.createObjectStore(STORE_NAME)
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function storeAssetFiles(files: Record<string, ArrayBuffer>): Promise<void> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readwrite')
  const store = tx.objectStore(STORE_NAME)
  for (const [name, buf] of Object.entries(files)) {
    store.put(buf, name)
  }
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function loadAssetFile(name: string): Promise<ArrayBuffer | null> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readonly')
  const store = tx.objectStore(STORE_NAME)
  return new Promise((resolve, reject) => {
    const req = store.get(name)
    req.onsuccess = () => resolve(req.result ?? null)
    req.onerror = () => reject(req.error)
  })
}

export async function clearAssetFiles(): Promise<void> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readwrite')
  tx.objectStore(STORE_NAME).clear()
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}
```

- [ ] **Step 2: Create `AssetPackContext`**

```typescript
// cmd/webclient/ui/src/client/assets/AssetPackContext.tsx
import React, { createContext, useContext, useEffect, useState } from 'react'
import * as PIXI from 'pixi.js'
import { parseTilesConfig } from './tilesConfig'
import { storeAssetFiles, loadAssetFile, clearAssetFiles } from './idb'
import type { AssetStatus, AssetVersion, PixiTextureMap, TilesConfig } from './types'

const VERSION_KEY = 'mud-asset-version'

interface AssetPackContextValue {
  status: AssetStatus
  progress: number     // 0–100 during download
  error: string | null
  textures: PixiTextureMap
  tilesConfig: TilesConfig | null
}

const AssetPackContext = createContext<AssetPackContextValue>({
  status: 'loading',
  progress: 0,
  error: null,
  textures: new Map(),
  tilesConfig: null,
})

export function useAssetPack(): AssetPackContextValue {
  return useContext(AssetPackContext)
}

export function AssetPackProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AssetStatus>('loading')
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [textures, setTextures] = useState<PixiTextureMap>(new Map())
  const [tilesConfig, setTilesConfig] = useState<TilesConfig | null>(null)

  useEffect(() => {
    loadAssets().catch(e => {
      setError(String(e))
      setStatus('error')
    })
  }, [])

  async function loadAssets() {
    // 1. Fetch version from Go proxy
    const versionResp = await fetch('/api/assets/version')
    if (!versionResp.ok) throw new Error(`Failed to fetch asset version: ${versionResp.status}`)
    const versionData: AssetVersion = await versionResp.json().then(d => ({
      version: d.version,
      downloadUrl: d.download_url,
      sha256Url: d.sha256_url,
    }))

    // 2. Check local cache
    const cachedVersion = localStorage.getItem(VERSION_KEY)
    const isCached = cachedVersion === String(versionData.version)

    if (isCached) {
      const tilesYaml = await loadAssetFile('tiles.yaml')
      if (tilesYaml) {
        await activatePack(tilesYaml)
        return
      }
    }

    // 3. Download + verify
    setStatus('downloading')
    const zipBuf = await downloadWithProgress(versionData.downloadUrl, setProgress)

    // Verify SHA-256
    const sha256Resp = await fetch(versionData.sha256Url)
    if (!sha256Resp.ok) throw new Error('Failed to fetch SHA-256 checksum')
    const expectedHash = (await sha256Resp.text()).split(/\s+/)[0].toLowerCase()
    const actualHashBuf = await crypto.subtle.digest('SHA-256', zipBuf)
    const actualHash = Array.from(new Uint8Array(actualHashBuf))
      .map(b => b.toString(16).padStart(2, '0'))
      .join('')
    if (actualHash !== expectedHash) {
      await clearAssetFiles()
      throw new Error('Asset pack SHA-256 verification failed')
    }

    // 4. Extract zip (using DecompressionStream API — no extra lib needed)
    const files = await extractZip(zipBuf)

    // 5. Store in IndexedDB + localStorage
    await storeAssetFiles(files)
    localStorage.setItem(VERSION_KEY, String(versionData.version))

    const tilesYaml = files['tiles.yaml']
    if (!tilesYaml) throw new Error('Asset pack missing tiles.yaml')
    await activatePack(tilesYaml)
  }

  async function activatePack(tilesYamlBuf: ArrayBuffer) {
    const yamlText = new TextDecoder().decode(tilesYamlBuf)
    const cfg = parseTilesConfig(yamlText)
    setTilesConfig(cfg)

    // Load all sprite sheets listed in tiles.yaml into PixiJS
    const allSheets = new Set<string>()
    Object.values(cfg.zones).forEach(z => allSheets.add(z.sheet))
    Object.values(cfg.npcs).forEach(n => allSheets.add(n.sheet))
    Object.values(cfg.animations).forEach(a => {
      if (a.attack) allSheets.add(a.attack.sheet)
      if (a.death) allSheets.add(a.death.sheet)
    })

    const textureMap: PixiTextureMap = new Map()
    for (const sheet of allSheets) {
      const buf = await loadAssetFile(sheet)
      if (!buf) continue
      const blob = new Blob([buf], { type: 'image/png' })
      const url = URL.createObjectURL(blob)
      const texture = await PIXI.Assets.load(url)
      textureMap.set(sheet, texture)
    }

    setTextures(textureMap)
    setStatus('ready')
  }

  return (
    <AssetPackContext.Provider value={{ status, progress, error, textures, tilesConfig }}>
      {children}
    </AssetPackContext.Provider>
  )
}

async function downloadWithProgress(
  url: string,
  onProgress: (pct: number) => void,
): Promise<ArrayBuffer> {
  const resp = await fetch(url)
  if (!resp.ok) throw new Error(`Download failed: ${resp.status}`)
  const total = Number(resp.headers.get('content-length') ?? 0)
  const reader = resp.body!.getReader()
  const chunks: Uint8Array[] = []
  let received = 0
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
    received += value.length
    if (total > 0) onProgress(Math.round((received / total) * 100))
  }
  const full = new Uint8Array(received)
  let offset = 0
  for (const chunk of chunks) {
    full.set(chunk, offset)
    offset += chunk.length
  }
  return full.buffer
}

// extractZip extracts a zip ArrayBuffer into a flat map of filename → ArrayBuffer.
// Uses the browser's native DecompressionStream for gzip; zip parsing is manual.
async function extractZip(buf: ArrayBuffer): Promise<Record<string, ArrayBuffer>> {
  // Parse ZIP local file headers (PKZIP format).
  const view = new DataView(buf)
  const files: Record<string, ArrayBuffer> = {}
  let offset = 0
  while (offset < buf.byteLength - 4) {
    const sig = view.getUint32(offset, true)
    if (sig !== 0x04034b50) break // local file header signature
    const compression = view.getUint16(offset + 8, true)
    const nameLen = view.getUint16(offset + 26, true)
    const extraLen = view.getUint16(offset + 28, true)
    const compressedSize = view.getUint32(offset + 18, true)
    const nameBytes = new Uint8Array(buf, offset + 30, nameLen)
    const name = new TextDecoder().decode(nameBytes)
    const dataOffset = offset + 30 + nameLen + extraLen
    const compressedData = buf.slice(dataOffset, dataOffset + compressedSize)

    if (compression === 0) {
      // Stored (no compression)
      files[name] = compressedData
    } else if (compression === 8) {
      // Deflate
      const ds = new DecompressionStream('deflate-raw')
      const writer = ds.writable.getWriter()
      writer.write(new Uint8Array(compressedData))
      writer.close()
      const decompressed = await new Response(ds.readable).arrayBuffer()
      files[name] = decompressed
    }
    offset = dataOffset + compressedSize
  }
  return files
}
```

- [ ] **Step 3: Create `AssetDownloadScreen`**

```typescript
// cmd/webclient/ui/src/client/assets/AssetDownloadScreen.tsx
import React from 'react'
import { useAssetPack } from './AssetPackContext'

export function AssetDownloadScreen({ children }: { children: React.ReactNode }) {
  const { status, progress, error } = useAssetPack()

  if (status === 'ready') return <>{children}</>

  if (status === 'error') {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100vh', gap: '1rem' }}>
        <h2>Asset Pack Error</h2>
        <p>{error}</p>
        <button onClick={() => window.location.reload()}>Retry</button>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100vh', gap: '1rem' }}>
      <h2>Loading Game Assets…</h2>
      {status === 'downloading' && (
        <>
          <progress value={progress} max={100} style={{ width: '300px' }} />
          <span>{progress}%</span>
        </>
      )}
      {status === 'loading' && <span>Checking asset version…</span>}
    </div>
  )
}
```

- [ ] **Step 4: Wrap the app in `AssetPackProvider` and `AssetDownloadScreen`**

In `cmd/webclient/ui/src/main.tsx`, wrap the router/app:

```typescript
// cmd/webclient/ui/src/main.tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AssetPackProvider } from './client/assets/AssetPackContext'
import { AssetDownloadScreen } from './client/assets/AssetDownloadScreen'
import { App } from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AssetPackProvider>
      <AssetDownloadScreen>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </AssetDownloadScreen>
    </AssetPackProvider>
  </React.StrictMode>,
)
```

- [ ] **Step 5: Verify the UI builds without errors**

```bash
cd cmd/webclient/ui && npm run build 2>&1 | tail -10
```

Expected: build succeeds with no TypeScript errors.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/client/assets/
git add cmd/webclient/ui/src/main.tsx
git commit -m "feat(webclient/ui): add AssetPackContext, AssetDownloadScreen, IndexedDB helpers"
```

---

## Task 5: Room Panel Layout Split

**Files:**
- Modify: `cmd/webclient/ui/src/components/RoomPanel.tsx`

This task splits the existing Room panel into a two-part vertical stack: scene canvas (top, 60%) + room text (bottom, 40%). The scene canvas placeholder is rendered here; `ScenePanel` is wired in Task 6.

- [ ] **Step 1: Write the failing test**

```typescript
// cmd/webclient/ui/src/components/RoomPanel.test.tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoomPanel } from './RoomPanel'

// Minimal mock RoomView matching the proto shape used by Phase 1
const mockRoomView = {
  title: 'The Rusty Nail',
  description: 'A dive bar.',
  exits: [{ direction: 'north' }, { direction: 'west' }],
  npcs: [],
  floor_items: [],
  zone_name: 'rustbucket',
}

it('renders room title in text sub-panel', () => {
  render(<RoomPanel roomView={mockRoomView} sceneNode={null} />)
  expect(screen.getByText('The Rusty Nail')).toBeTruthy()
})

it('renders scene slot when sceneNode provided', () => {
  const scene = <div data-testid="scene-slot" />
  render(<RoomPanel roomView={mockRoomView} sceneNode={scene} />)
  expect(screen.getByTestId('scene-slot')).toBeTruthy()
})

it('expands text panel to full height when sceneNode is null', () => {
  const { container } = render(<RoomPanel roomView={mockRoomView} sceneNode={null} />)
  const textPanel = container.querySelector('.room-text-panel') as HTMLElement
  expect(textPanel.style.flex).toBe('1')
})
```

Add `@testing-library/react` and `@testing-library/user-event` to devDependencies in `package.json`:
```json
"@testing-library/react": "^16.0.0",
"@testing-library/user-event": "^14.0.0"
```
Then run `npm install` in `cmd/webclient/ui/`.

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cmd/webclient/ui && npm test -- --reporter=verbose 2>&1 | grep -E "FAIL|PASS|Error" | head -10
```

Expected: FAIL — `RoomPanel` does not accept `sceneNode` prop yet.

- [ ] **Step 3: Modify `RoomPanel.tsx`**

Replace the existing Phase 1 `RoomPanel` component with this version (preserving all existing room text rendering logic inside the `.room-text-panel` div):

```typescript
// cmd/webclient/ui/src/components/RoomPanel.tsx
import React from 'react'
import type { RoomView } from '../proto/game/v1/game_pb'  // Phase 1 generated proto type

interface RoomPanelProps {
  roomView: RoomView | null
  sceneNode: React.ReactNode | null  // null when assets not ready
}

export function RoomPanel({ roomView, sceneNode }: RoomPanelProps) {
  return (
    <div className="room-panel" style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Scene sub-panel — hidden when sceneNode is null */}
      {sceneNode && (
        <div className="room-scene-panel" style={{ flex: '0 0 60%', overflow: 'hidden' }}>
          {sceneNode}
        </div>
      )}

      {/* Text sub-panel — expands to fill when no scene */}
      <div
        className="room-text-panel"
        style={{ flex: '1', overflow: 'auto', padding: '0.5rem' }}
      >
        {roomView && (
          <>
            <h3>{roomView.title}</h3>
            <p>{roomView.description}</p>
            <div className="exits">
              {roomView.exits.map(exit => (
                <button key={exit.direction} className="exit-btn">
                  {exit.direction.toUpperCase()}
                </button>
              ))}
            </div>
            {roomView.npcs.length > 0 && (
              <div className="npcs">
                <strong>NPCs:</strong> {roomView.npcs.map(n => n.name).join(', ')}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Update callers of `RoomPanel` in the game layout**

Find the existing game layout component (e.g. `cmd/webclient/ui/src/components/GameLayout.tsx` or `cmd/webclient/ui/src/pages/Game.tsx`) and pass `sceneNode`:

```typescript
// In the game view component — pass sceneNode={null} for now; Task 6 provides ScenePanel
<RoomPanel roomView={currentRoomView} sceneNode={null} />
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/components/RoomPanel.tsx \
        cmd/webclient/ui/src/components/RoomPanel.test.tsx \
        cmd/webclient/ui/package.json cmd/webclient/ui/package-lock.json
git commit -m "feat(webclient/ui): split RoomPanel into scene+text vertical stack"
```

---

## Task 6: `ScenePanel` — PixiJS Scene

**Files:**
- Create: `cmd/webclient/ui/src/client/scene/ScenePanel.tsx`
- Modify: Game layout component to pass `<ScenePanel>` as `sceneNode`

- [ ] **Step 1: Write the failing test**

```typescript
// cmd/webclient/ui/src/client/scene/ScenePanel.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import React from 'react'

// Mock pixi.js before importing ScenePanel
vi.mock('pixi.js', () => {
  const Container = vi.fn(() => ({ addChild: vi.fn(), removeChildren: vi.fn(), children: [] }))
  const Sprite = vi.fn(() => ({ position: { set: vi.fn() }, anchor: { set: vi.fn() }, width: 0, height: 0, texture: null, eventMode: '', on: vi.fn() }))
  const Text = vi.fn(() => ({ position: { set: vi.fn() } }))
  const Application = vi.fn().mockImplementation(() => ({
    init: vi.fn().mockResolvedValue(undefined),
    canvas: document.createElement('canvas'),
    stage: { addChild: vi.fn() },
    renderer: { resize: vi.fn() },
    destroy: vi.fn(),
  }))
  const Assets = { load: vi.fn().mockResolvedValue({}) }
  const Texture = { from: vi.fn(() => ({})) }
  return { Application, Container, Sprite, Text, Assets, Texture }
})

const mockTilesConfig = {
  zones: { rustbucket: { sheet: 'tilesets/zones/rustbucket.png', tile: { x: 0, y: 0, w: 320, h: 240 } }, 'default': { sheet: 'tilesets/zones/default.png', tile: { x: 0, y: 0, w: 320, h: 240 } } },
  npcs: { 'default-npc': { sheet: 'tilesets/npcs/default.png', idle: { x: 0, y: 0, w: 32, h: 48 }, fps: 12 } },
  animations: {},
}

const mockTextures = new Map([['tilesets/zones/rustbucket.png', {}], ['tilesets/npcs/default.png', {}]])

vi.mock('../../client/assets/AssetPackContext', () => ({
  useAssetPack: () => ({ status: 'ready', textures: mockTextures, tilesConfig: mockTilesConfig }),
}))

import { ScenePanel } from './ScenePanel'

describe('ScenePanel', () => {
  it('renders a canvas element', async () => {
    const { container } = render(
      <ScenePanel
        roomView={{ title: 'Test', description: '', exits: [], npcs: [], floor_items: [], zone_name: 'rustbucket' }}
        onMove={vi.fn()}
      />
    )
    // PixiJS attaches canvas — the container div should exist
    expect(container.querySelector('.scene-panel')).toBeTruthy()
  })

  it('falls back to default zone when zone_name is unknown', async () => {
    // Should not throw
    expect(() => render(
      <ScenePanel
        roomView={{ title: 'Test', description: '', exits: [], npcs: [], floor_items: [], zone_name: 'unknown-zone-xyz' }}
        onMove={vi.fn()}
      />
    )).not.toThrow()
  })
})
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cmd/webclient/ui && npm test -- --reporter=verbose 2>&1 | grep -E "ScenePanel|Error" | head -10
```

Expected: FAIL — `ScenePanel` does not exist yet.

- [ ] **Step 3: Implement `ScenePanel`**

```typescript
// cmd/webclient/ui/src/client/scene/ScenePanel.tsx
import React, { useEffect, useRef } from 'react'
import * as PIXI from 'pixi.js'
import { useAssetPack } from '../assets/AssetPackContext'
import type { TilesConfig, PixiTextureMap } from '../assets/types'

interface RoomViewLike {
  title: string
  description: string
  zone_name: string
  exits: Array<{ direction: string }>
  npcs: Array<{ name: string }>
  floor_items: Array<unknown>
}

interface ScenePanelProps {
  roomView: RoomViewLike | null
  onMove: (direction: string) => void
}

export function ScenePanel({ roomView, onMove }: ScenePanelProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const appRef = useRef<PIXI.Application | null>(null)
  const layersRef = useRef<{
    bg: PIXI.Container
    npcs: PIXI.Container
    player: PIXI.Container
    exits: PIXI.Container
    anims: PIXI.Container
  } | null>(null)
  const { textures, tilesConfig } = useAssetPack()

  // Mount PixiJS application
  useEffect(() => {
    if (!containerRef.current) return
    const app = new PIXI.Application()
    appRef.current = app

    app.init({ resizeTo: containerRef.current, backgroundAlpha: 0 }).then(() => {
      containerRef.current!.appendChild(app.canvas)
      const bg = new PIXI.Container()
      const npcs = new PIXI.Container()
      const player = new PIXI.Container()
      const exits = new PIXI.Container()
      const anims = new PIXI.Container()
      app.stage.addChild(bg, npcs, player, exits, anims)
      layersRef.current = { bg, npcs, player, exits, anims }
    })

    return () => {
      app.destroy(true)
      appRef.current = null
      layersRef.current = null
    }
  }, [])

  // Re-render scene when roomView changes
  useEffect(() => {
    if (!layersRef.current || !roomView || !tilesConfig || !textures) return
    const app = appRef.current!
    const { bg, npcs, player, exits } = layersRef.current
    const w = app.canvas.width
    const h = app.canvas.height

    renderBackground(bg, roomView.zone_name, tilesConfig, textures, w, h)
    renderNpcs(npcs, roomView.npcs, tilesConfig, textures, w, h)
    renderPlayer(player, textures, w, h)
    renderExits(exits, roomView.exits, textures, w, h, onMove)
  }, [roomView, tilesConfig, textures])

  return <div className="scene-panel" ref={containerRef} style={{ width: '100%', height: '100%' }} />
}

function renderBackground(
  layer: PIXI.Container,
  zoneName: string,
  cfg: TilesConfig,
  textures: PixiTextureMap,
  w: number,
  h: number,
) {
  layer.removeChildren()
  const zoneCfg = cfg.zones[zoneName] ?? cfg.zones['default']
  if (!zoneCfg) return
  const texture = textures.get(zoneCfg.sheet)
  if (!texture) return
  const sprite = new PIXI.Sprite(texture)
  sprite.width = w
  sprite.height = h
  layer.addChild(sprite)
}

function renderNpcs(
  layer: PIXI.Container,
  npcList: Array<{ name: string }>,
  cfg: TilesConfig,
  textures: PixiTextureMap,
  w: number,
  h: number,
) {
  layer.removeChildren()
  const visible = npcList.slice(0, 6)
  const spacing = w / (visible.length + 1)
  visible.forEach((npc, i) => {
    const category = resolveNpcCategory(npc.name, cfg)
    const npcCfg = cfg.npcs[category] ?? cfg.npcs['default-npc']
    if (!npcCfg) return
    const texture = textures.get(npcCfg.sheet)
    if (!texture) return
    const sprite = new PIXI.Sprite(texture)
    sprite.anchor.set(0.5, 1)
    sprite.position.set(spacing * (i + 1), h * 0.9)
    layer.addChild(sprite)

    // Count badge if more than 6 NPCs
    if (i === 5 && npcList.length > 6) {
      const badge = new PIXI.Text({ text: `+${npcList.length - 6}`, style: { fill: 0xffffff, fontSize: 14 } })
      badge.position.set(sprite.x + 10, sprite.y - sprite.height - 4)
      layer.addChild(badge)
    }
  })
}

function renderPlayer(layer: PIXI.Container, textures: PixiTextureMap, w: number, h: number) {
  layer.removeChildren()
  const texture = textures.get('tilesets/ui/player.png')
  if (!texture) return
  const sprite = new PIXI.Sprite(texture)
  sprite.anchor.set(0.5, 1)
  sprite.position.set(w / 2, h * 0.95)
  layer.addChild(sprite)
}

const EXIT_POSITIONS: Record<string, [number, number]> = {
  north: [0.5, 0.05],
  south: [0.5, 0.95],
  east: [0.95, 0.5],
  west: [0.05, 0.5],
}

function renderExits(
  layer: PIXI.Container,
  exitList: Array<{ direction: string }>,
  textures: PixiTextureMap,
  w: number,
  h: number,
  onMove: (dir: string) => void,
) {
  layer.removeChildren()
  for (const exit of exitList) {
    const pos = EXIT_POSITIONS[exit.direction.toLowerCase()]
    if (!pos) continue
    const texture = textures.get(`tilesets/ui/exit-${exit.direction.toLowerCase()}.png`)
      ?? textures.get('tilesets/ui/exit-default.png')
    if (!texture) continue
    const sprite = new PIXI.Sprite(texture)
    sprite.anchor.set(0.5, 0.5)
    sprite.position.set(w * pos[0], h * pos[1])
    sprite.eventMode = 'static'
    sprite.cursor = 'pointer'
    sprite.on('pointerdown', () => onMove(exit.direction.toLowerCase()))
    layer.addChild(sprite)
  }
}

function resolveNpcCategory(npcName: string, cfg: TilesConfig): string {
  const lower = npcName.toLowerCase()
  for (const category of Object.keys(cfg.npcs)) {
    if (lower.includes(category)) return category
  }
  return 'default-npc'
}
```

- [ ] **Step 4: Wire `ScenePanel` into the game layout**

In the game view component (e.g. `cmd/webclient/ui/src/pages/Game.tsx`), import and pass `ScenePanel`:

```typescript
import { ScenePanel } from '../client/scene/ScenePanel'
import { useAssetPack } from '../client/assets/AssetPackContext'

// Inside the component:
const { status } = useAssetPack()
const sceneNode = status === 'ready' ? (
  <ScenePanel roomView={currentRoomView} onMove={dir => session.Send(`move ${dir}`)} />
) : null

// Pass to RoomPanel:
<RoomPanel roomView={currentRoomView} sceneNode={sceneNode} />
```

- [ ] **Step 5: Run all tests**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -15
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/client/scene/ScenePanel.tsx \
        cmd/webclient/ui/src/client/scene/ScenePanel.test.tsx
git commit -m "feat(webclient/ui): add PixiJS ScenePanel with four layers"
```

---

## Task 7: `CombatAnimationQueue` + Combat Animations

**Files:**
- Create: `cmd/webclient/ui/src/client/scene/CombatAnimationQueue.ts`
- Create: `cmd/webclient/ui/src/client/scene/CombatAnimationQueue.test.ts`
- Modify: `cmd/webclient/ui/src/client/scene/ScenePanel.tsx`

- [ ] **Step 1: Write the failing test**

```typescript
// cmd/webclient/ui/src/client/scene/CombatAnimationQueue.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { CombatAnimationQueue } from './CombatAnimationQueue'

describe('CombatAnimationQueue', () => {
  it('enqueues and plays an animation', () => {
    const q = new CombatAnimationQueue()
    const play = vi.fn((onDone: () => void) => onDone())
    q.enqueue('npc-1', 'attack', play)
    expect(play).toHaveBeenCalledOnce()
  })

  it('queues a second animation while first is playing', () => {
    const q = new CombatAnimationQueue()
    let firstDone: (() => void) | null = null
    const play1 = vi.fn((onDone: () => void) => { firstDone = onDone })
    const play2 = vi.fn((onDone: () => void) => onDone())

    q.enqueue('npc-1', 'attack', play1)
    q.enqueue('npc-1', 'hit-flash', play2)

    expect(play1).toHaveBeenCalledOnce()
    expect(play2).not.toHaveBeenCalled()

    firstDone!()
    expect(play2).toHaveBeenCalledOnce()
  })

  it('different sprites have independent queues', () => {
    const q = new CombatAnimationQueue()
    const play1 = vi.fn((onDone: () => void) => onDone())
    const play2 = vi.fn((onDone: () => void) => onDone())
    q.enqueue('npc-1', 'attack', play1)
    q.enqueue('npc-2', 'attack', play2)
    expect(play1).toHaveBeenCalledOnce()
    expect(play2).toHaveBeenCalledOnce()
  })

  it('clears a sprite queue', () => {
    const q = new CombatAnimationQueue()
    let firstDone: (() => void) | null = null
    const play1 = vi.fn((onDone: () => void) => { firstDone = onDone })
    const play2 = vi.fn((onDone: () => void) => onDone())

    q.enqueue('npc-1', 'attack', play1)
    q.enqueue('npc-1', 'death', play2)
    q.clear('npc-1')
    firstDone!()
    expect(play2).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cmd/webclient/ui && npm test -- --reporter=verbose 2>&1 | grep -E "CombatAnimation|Error" | head -10
```

Expected: FAIL — `CombatAnimationQueue` does not exist.

- [ ] **Step 3: Implement `CombatAnimationQueue`**

```typescript
// cmd/webclient/ui/src/client/scene/CombatAnimationQueue.ts

export type AnimationType = 'attack' | 'hit-flash' | 'death'
export type AnimationPlayer = (onDone: () => void) => void

interface QueueEntry {
  type: AnimationType
  play: AnimationPlayer
}

export class CombatAnimationQueue {
  private queues = new Map<string, QueueEntry[]>()
  private playing = new Set<string>()

  enqueue(spriteId: string, type: AnimationType, play: AnimationPlayer): void {
    if (!this.queues.has(spriteId)) this.queues.set(spriteId, [])
    this.queues.get(spriteId)!.push({ type, play })
    if (!this.playing.has(spriteId)) {
      this.advance(spriteId)
    }
  }

  clear(spriteId: string): void {
    this.queues.set(spriteId, [])
    // Do not cancel in-flight animation — it will complete and find an empty queue
  }

  private advance(spriteId: string): void {
    const queue = this.queues.get(spriteId)
    if (!queue || queue.length === 0) {
      this.playing.delete(spriteId)
      return
    }
    const entry = queue.shift()!
    this.playing.add(spriteId)
    entry.play(() => this.advance(spriteId))
  }
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 5: Wire combat animations into `ScenePanel`**

Add to `ScenePanel.tsx`. Import `CombatAnimationQueue` and wire the `CombatEvent` handler:

```typescript
// Add to imports in ScenePanel.tsx
import { CombatAnimationQueue, type AnimationType } from './CombatAnimationQueue'

// Add inside ScenePanel component (after layersRef):
const queueRef = useRef(new CombatAnimationQueue())

// Add a second useEffect that subscribes to combat events:
useEffect(() => {
  if (!roomView) return
  // This effect re-runs when session.Events() fires; in practice the game
  // view component drives this by passing down derived props. The animation
  // handlers are wired to the session event stream in the parent game view.
  // See the combatEvent handler below — call playCombatAnimation from the parent.
}, [roomView])

// Export a handler for the parent to call on CombatEvent:
// combatAnimations are driven by the parent game view component passing
// CombatEvent data as props. Add to ScenePanelProps:
```

Update `ScenePanelProps` and `ScenePanel` to accept `combatEvent`:

```typescript
// Updated ScenePanelProps
interface ScenePanelProps {
  roomView: RoomViewLike | null
  onMove: (direction: string) => void
  combatEvent?: {
    type: 'ATTACK' | 'DEATH'
    attacker: string
    target: string
  } | null
  characterName?: string
}

// Add inside ScenePanel component after queueRef:
useEffect(() => {
  if (!combatEvent || !layersRef.current || !tilesConfig || !textures) return
  const { anims } = layersRef.current
  const queue = queueRef.current

  const buildPlayer = (type: AnimationType, npcName: string): AnimationPlayer => (onDone) => {
    // Hit-flash: white tint for 80ms
    if (type === 'hit-flash') {
      const texture = textures.get('tilesets/ui/player.png')
      if (!texture) { onDone(); return }
      const flash = new PIXI.Sprite(texture)
      flash.tint = 0xffffff
      anims.addChild(flash)
      setTimeout(() => { anims.removeChild(flash); onDone() }, 80)
      return
    }
    // attack/death: play AnimatedSprite frames from animations config
    const animCfg = tilesConfig.animations[resolveNpcCategory(npcName, tilesConfig)]
    const anim = type === 'attack' ? animCfg?.attack : animCfg?.death
    if (!anim) { onDone(); return }
    const sheet = textures.get(anim.sheet)
    if (!sheet) { onDone(); return }
    const frameTextures = anim.frames.map(f => new PIXI.Texture({
      source: sheet.source,
      frame: new PIXI.Rectangle(f.x, f.y, f.w, f.h),
    }))
    const animSprite = new PIXI.AnimatedSprite(frameTextures)
    const npcCfg = tilesConfig.npcs[resolveNpcCategory(npcName, tilesConfig)]
    animSprite.animationSpeed = (npcCfg?.fps ?? 12) / 60
    animSprite.loop = false
    animSprite.onComplete = () => { anims.removeChild(animSprite); onDone() }
    anims.addChild(animSprite)
    animSprite.play()
  }

  if (combatEvent.type === 'ATTACK') {
    queue.enqueue(combatEvent.attacker, 'attack', buildPlayer('attack', combatEvent.attacker))
    const targetId = combatEvent.target === characterName ? '__player__' : combatEvent.target
    queue.enqueue(targetId, 'hit-flash', buildPlayer('hit-flash', combatEvent.target))
  } else if (combatEvent.type === 'DEATH') {
    queue.enqueue(combatEvent.target, 'death', buildPlayer('death', combatEvent.target))
  }
}, [combatEvent])
```

In the game view component, wire `CombatEvent` from `session.Events()` into `ScenePanel`:

```typescript
// In the game view component (e.g. Game.tsx), track the latest combat event:
const [combatEvent, setCombatEvent] = useState<ScenePanelProps['combatEvent']>(null)

// In the event subscription loop:
if (ev.getCombatEvent()) {
  const ce = ev.getCombatEvent()!
  const typeStr = ce.getType() === CombatEventType.COMBAT_EVENT_TYPE_ATTACK ? 'ATTACK' : 'DEATH'
  setCombatEvent({ type: typeStr, attacker: ce.getAttacker(), target: ce.getTarget() })
}

// Pass to ScenePanel:
<ScenePanel
  roomView={currentRoomView}
  onMove={dir => session.Send(`move ${dir}`)}
  combatEvent={combatEvent}
  characterName={session.State().Character?.Name ?? ''}
/>
```

- [ ] **Step 6: Run all tests**

```bash
cd cmd/webclient/ui && npm test 2>&1 | tail -15
```

Expected: all tests PASS.

- [ ] **Step 7: Run Go tests to confirm no regressions**

```bash
mise exec -- go test ./... -short -count=1 -timeout=120s 2>&1 | tail -10
```

Expected: all packages PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/webclient/ui/src/client/scene/CombatAnimationQueue.ts \
        cmd/webclient/ui/src/client/scene/CombatAnimationQueue.test.ts \
        cmd/webclient/ui/src/client/scene/ScenePanel.tsx
git commit -m "feat(webclient/ui): add CombatAnimationQueue and combat sprite animations"
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|---|---|
| REQ-WC2-1: `internal/client/assets` sub-package | Task 1 |
| REQ-WC2-2: `AssetVersion` struct | Task 1 |
| REQ-WC2-3: `FetchLatestVersion`, `ErrNoRelease`, `ErrNetwork` | Task 1 |
| REQ-WC2-4: `ParseVersion` | Task 1 |
| REQ-WC2-5: httptest + table-driven tests | Task 1 |
| REQ-WC2-6: `GET /api/assets/version` endpoint, HTTP 502 on failure | Task 2 |
| REQ-WC2-7: `WebConfig.GitHubReleasesURL`, `configs/dev.yaml` | Task 2 |
| REQ-WC2-8: `AssetPackContext` above all routes | Task 4 |
| REQ-WC2-9: Load sequence (version check, cache, download, verify, IndexedDB) | Task 4 |
| REQ-WC2-9: `/api/assets/version` exempted from auth | Task 2 |
| REQ-WC2-10: `AssetStatus`, `progress`, `textures`, `tilesConfig` exposed | Task 4 |
| REQ-WC2-11: `TilesConfig` from `tiles.yaml`, fps default 12 | Task 3 |
| REQ-WC2-12: `PixiTextureMap`, eager load all sheets | Task 4 |
| REQ-WC2-13: Room panel split 60/40, scene top, text bottom | Task 5 |
| REQ-WC2-14: Outer grid layout unchanged | Task 5 |
| REQ-WC2-15: Scene hidden, text expands when assets not ready | Task 5 |
| REQ-WC2-16: `ScenePanel` mounts PixiJS via `useRef`, destroys on unmount | Task 6 |
| REQ-WC2-17: Five layers (bg, npcs, player, exits, anims) | Task 6 |
| REQ-WC2-18: BackgroundLayer, zone fallback to default | Task 6 |
| REQ-WC2-19: NpcLayer up to 6 sprites, count badge | Task 6 |
| REQ-WC2-20: PlayerLayer anchored bottom-center | Task 6 |
| REQ-WC2-21: ExitLayer clickable indicators → `session.Send` | Task 6 |
| REQ-WC2-22: `ScenePanel` handles `RoomView`, `CombatEvent` ATTACK, `CombatEvent` DEATH | Task 7 |
| REQ-WC2-23: `CombatAnimationQueue` sequential per-sprite | Task 7 |
| REQ-WC2-24: attack, hit-flash (80ms), death animations | Task 7 |
| REQ-WC2-25: Player sprite hit-flash on `CombatEvent.target == character name` | Task 7 |
| REQ-WC2-26: Animation fps from `TilesConfig`, default 12 | Task 7 |
| REQ-WC2-27/28: Feature index split (handled in brainstorming — already done) | — |
| REQ-WC2-29: `game-client-ebiten` deps updated (already done) | — |
| REQ-WC2-30: `internal-client` feature updated (already done) | — |

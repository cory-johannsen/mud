# Plan: Web Game Client — Phase 2 (PixiJS Tiled Room Scene)

**GitHub Issue:** cory-johannsen/mud#13
**Spec:** `docs/superpowers/specs/2026-04-12-web-client-phase2.md`
**Date:** 2026-04-12
**See also:** Map rework plan at `docs/superpowers/plans/2026-04-13-svg-map-rework.md` (cory-johannsen/mud#51)

---

## Step 1 — `internal/client/assets` sub-package (REQ-PC-1)

**File:** `internal/client/assets/` (new package)

**TDD first:** Create `assets_test.go` with `httptest.Server`-backed tests for `FetchLatestVersion` (success, `ErrNoRelease`, `ErrNetwork`) and table-driven tests for `ParseVersion` (valid, whitespace, invalid).

Then implement `assets.go`:
```go
type AssetVersion struct {
    Version     int
    DownloadURL string
    SHA256URL   string
}

var ErrNoRelease = errors.New("no matching asset release found")
var ErrNetwork   = errors.New("network error fetching releases")

func FetchLatestVersion(ctx context.Context, releasesURL string) (*AssetVersion, error)
func ParseVersion(s string) (int, error)
```

```bash
mise exec -- go test ./internal/client/assets/... -count=1 -v
```

---

## Step 2 — Go asset proxy endpoint (REQ-PC-2)

**Files:** `cmd/webclient/`

1. Add `GitHubReleasesURL string \`yaml:"github_releases_url"\`` to `WebConfig`
2. Fatal startup error if `GitHubReleasesURL` is empty
3. Add `GET /api/assets/version` route — auth-exempt, calls `assets.FetchLatestVersion`, returns JSON
4. Update `configs/dev.yaml` with `github_releases_url: https://api.github.com/repos/cory-johannsen/mud/releases/latest`

```bash
mise exec -- go build ./cmd/webclient/...
```

---

## Step 3 — TypeScript: `AssetPackContext` (REQ-PC-3)

**Files:** `cmd/webclient/ui/src/`

**New files:** `AssetPackContext.tsx`, `AssetErrorScreen.tsx`

```typescript
export type AssetStatus = 'loading' | 'downloading' | 'ready' | 'error'
export interface AssetPackContextValue {
  status: AssetStatus
  progress: number          // 0-100 during download
  textures: PixiTextureMap  // Map<string, PIXI.Texture>
  tilesConfig: TilesConfig
}
```

Load sequence (REQ-PC-3b):
1. `GET /api/assets/version` (no JWT)
2. Compare vs `localStorage['mud-asset-version']`
3. If match + IndexedDB cache: load textures → `ready`
4. If mismatch/missing: download zip → SHA-256 verify → extract to IndexedDB → store version → load textures → `ready`
5. No network + cache present: warn, proceed with cache
6. No network + no cache: render `AssetErrorScreen` with Retry button

Mount `AssetPackContext.Provider` at app root above all routes.

---

## Step 4 — Room panel layout split (REQ-PC-4)

**File:** `cmd/webclient/ui/src/game/panels/RoomPanel.tsx` (or equivalent room panel component)

Split the room panel into two vertical sub-panels:
- Scene sub-panel: `flex: 0.6` — holds `<ScenePanel />`; hidden when `status !== 'ready'`
- Text sub-panel: `flex: 0.4` (expands to `flex: 1` when scene hidden) — existing room text (title, description, exits, NPC list, floor items)

Overall CSS Grid layout unchanged.

---

## Step 5 — `ScenePanel` React component (REQ-PC-5)

**File:** `cmd/webclient/ui/src/game/ScenePanel.tsx` (new)

```typescript
export function ScenePanel(): JSX.Element {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const { textures, tilesConfig } = useAssetPack()
  const { state, session } = useGame()
  // mount PixiJS Application on canvasRef, destroy on unmount
  // maintain layers: background, npc, player, exit, animation
}
```

Layer responsibilities:
- **BackgroundLayer**: one `PIXI.Sprite` — zone name → `TilesConfig` category → sprite
- **NpcLayer**: up to 6 `PIXI.Sprite` evenly spaced; `+N` `PIXI.Text` badge for overflow
- **PlayerLayer**: one `PIXI.Sprite` bottom-center
- **ExitLayer**: N/S/E/W sprites, `eventMode: "static"`, `pointerdown` → `session.Send("move {dir}")`
- **AnimationLayer**: above all others; `CombatAnimationQueue` drives animations

Subscribe to `session.Events()`:
- `RoomView` → rebuild all four layers
- `CombatEvent` (ATTACK) → enqueue attack on attacker + hit-flash on target
- `CombatEvent` (DEATH) → enqueue death on target; sprite persists until next `RoomView`

---

## Step 6 — `CombatAnimationQueue` (REQ-PC-6)

**File:** `cmd/webclient/ui/src/game/CombatAnimationQueue.ts` (new)

```typescript
export type AnimationType = 'attack' | 'hit-flash' | 'death'

export class CombatAnimationQueue {
  enqueue(spriteId: string, type: AnimationType): void
  // Per-sprite FIFO; plays next only after current finishes
}
```

Animation implementations:
- `attack`: `PIXI.AnimatedSprite` using category attack frames at `TilesConfig` fps; return to idle on completion
- `hit-flash`: white-tinted `PIXI.Sprite` overlay for 80ms
- `death`: death frame strip; hold last frame until external removal

---

## Step 7 — Full integration test and build

```bash
mise exec -- go test ./internal/client/... -count=1
cd cmd/webclient/ui && npm run build
```

---

## Dependency Order

```
Step 1 (assets pkg) ──▶ Step 2 (proxy endpoint)
Step 3 (AssetPackContext) ──▶ Step 4 (room panel layout)
Step 4 ──▶ Step 5 (ScenePanel)
Step 5 ──▶ Step 6 (CombatAnimationQueue)
Step 2 + Step 6 ──▶ Step 7 (full build)
```

Steps 1 and 3 are independent and can run in parallel.

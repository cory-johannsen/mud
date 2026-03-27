# Game Client (Ebiten) Phase 3: Asset Pack

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Asset pack version check, download, SHA-256 verification, extraction, and tile/colour registry loading.

**Architecture:** Pure functions for version comparison, download, and YAML parsing — all unit-testable with httptest and temp dirs. Ebiten download screen on top.

**Tech Stack:** Go net/http, crypto/sha256, archive/zip, gopkg.in/yaml.v3, Ebiten v2

---

## Requirements Covered

- REQ-GCE-11: Asset source files reside in `assets/` at repo root; sole input to `make package-assets`
- REQ-GCE-12: Version check on startup; download if local < remote; graceful degradation if network unreachable with local pack; error + exit if no local pack and network unreachable
- REQ-GCE-13: Download failure shows error reason and Retry button; client MUST NOT proceed without valid pack
- REQ-GCE-14: SHA-256 checksum verification before extraction; partial download deleted on failure
- REQ-GCE-15: Asset pack zip structure with `version.txt`, `tiles.yaml`, `colours.yaml`, tilesets, animations
- REQ-GCE-16: `tiles.yaml` maps zone categories, NPC types, item categories to sprite coords; default fps=12 if absent
- REQ-GCE-17: `colours.yaml` maps ServerEvent field names to hex colours; fallback to `default` key
- REQ-GCE-18: Asset pack published independently of binary, attached to GitHub Releases
- REQ-GCE-19: Zone name → zone category lookup in `tiles.yaml`; unrecognised names fall back to default tileset
- REQ-GCE-31: Missing/unparseable `tiles.yaml` after extraction triggers delete of extracted dir and re-enter download flow

## Assumes

- Phase 1 complete: `cmd/ebitenclient/` binary scaffolding, config loading, window initialisation
- Phase 2 complete: Login screen, character select screen, gRPC session wiring

---

## File Inventory

| File | Purpose |
|------|---------|
| `cmd/ebitenclient/assets/downloader.go` | Version check, zip + sha256 download, checksum verification, extraction |
| `cmd/ebitenclient/assets/downloader_test.go` | httptest-based tests for all downloader paths |
| `cmd/ebitenclient/assets/loader.go` | Parse `tiles.yaml` and `colours.yaml` into Go structs |
| `cmd/ebitenclient/assets/loader_test.go` | YAML parsing tests including missing fields and malformed input |
| `cmd/ebitenclient/assets/registry.go` | TileRegistry: zone category → TileRef; NPC type → AnimRef; colour lookups |
| `cmd/ebitenclient/assets/registry_test.go` | Lookup tests including fallback to default tileset and default fps |
| `cmd/ebitenclient/screens/download.go` | Ebiten download progress screen |
| `assets/version.txt` | Initial content: `1` |

---

## Key Data Structures

```go
// tiles.yaml structure
type TilesConfig struct {
    Default TileRef            `yaml:"default"`
    Zones   map[string]TileRef `yaml:"zones"`
    NPCs    map[string]AnimRef `yaml:"npcs"`
    Items   map[string]TileRef `yaml:"items"`
}

type TileRef struct {
    Sheet string `yaml:"sheet"`
    X     int    `yaml:"x"`
    Y     int    `yaml:"y"`
    W     int    `yaml:"w"`
    H     int    `yaml:"h"`
}

type AnimRef struct {
    Sheet  string `yaml:"sheet"`
    X      int    `yaml:"x"`
    Y      int    `yaml:"y"`
    W      int    `yaml:"w"`
    H      int    `yaml:"h"`
    Frames int    `yaml:"frames"`
    FPS    int    `yaml:"fps"` // default 12 if absent or zero
}

// colours.yaml structure
type ColoursConfig struct {
    Default string            `yaml:"default"`
    Events  map[string]string `yaml:"events"` // event type name → hex colour
}
```

---

## Task 1: Version Check Logic

- [ ] **1.1** Create `cmd/ebitenclient/assets/` directory (package `assets`).
- [ ] **1.2** Implement `FetchRemoteVersion(ctx context.Context, releasesURL string) (int, string, string, error)` in `downloader.go`:
  - GET `releasesURL` → parse JSON response
  - Extract assets array; find asset with name matching `mud-assets-v{N}.zip`; parse `N` as integer
  - Return `(version int, zipURL string, sha256URL string, err error)`
  - Return `ErrNoAssetFound` (sentinel) if no matching asset exists in the response
- [ ] **1.3** Implement `ReadLocalVersion(cacheDir string) (int, error)` in `downloader.go`:
  - Read `{cacheDir}/version.txt`; parse integer
  - Return `(0, nil)` if file does not exist (treat as version 0, triggers download)
- [ ] **1.4** Implement `NeedsUpdate(localVersion, remoteVersion int) bool` in `downloader.go` — pure function.
- [ ] **1.5** Write `downloader_test.go` — `TestFetchRemoteVersion`:
  - Property: valid JSON with `mud-assets-v{N}.zip` asset → returned version equals N for any positive N
  - Table-driven: malformed JSON returns error; missing zip asset returns `ErrNoAssetFound`; version parsed correctly from name
  - Use `net/http/httptest.NewServer` for all HTTP calls
- [ ] **1.6** Write `TestReadLocalVersion`:
  - Property: write any integer to temp dir `version.txt` → `ReadLocalVersion` returns same integer
  - Table-driven: missing file → (0, nil); non-integer content → error
- [ ] **1.7** Write `TestNeedsUpdate`:
  - Property: `NeedsUpdate(local, remote)` is true iff `remote > local` for all non-negative integers
- [ ] **1.8** Run tests: `mise exec -- go test ./cmd/ebitenclient/assets/... -v -run TestFetchRemoteVersion` and `TestReadLocalVersion` and `TestNeedsUpdate` — all MUST pass.

---

## Task 2: Downloader — Fetch, Verify, Extract

- [ ] **2.1** Implement `DownloadFile(ctx context.Context, url, destPath string) error` in `downloader.go`:
  - GET `url`; stream response body to `destPath` using `io.Copy`
  - Create parent directories if absent
  - Return error on non-200 status or I/O failure
- [ ] **2.2** Implement `VerifyChecksum(zipPath, sha256Path string) error` in `downloader.go`:
  - Read sha256 file; parse expected hex digest (format: `<hex>  <filename>` or bare `<hex>`)
  - Compute `crypto/sha256` of zip file
  - Return `ErrChecksumMismatch` (sentinel) if digests differ
- [ ] **2.3** Implement `ExtractZip(zipPath, destDir string) error` in `downloader.go`:
  - Open zip; iterate entries; validate no path traversal (`..` components)
  - Extract files into `destDir`; preserve directory structure
  - Return error on any I/O failure; partial extraction is left on disk for the caller to clean up
- [ ] **2.4** Implement `DownloadAndInstall(ctx context.Context, zipURL, sha256URL string, version int, cacheDir string, progress func(downloaded, total int64)) error` in `downloader.go`:
  - Download zip to temp file; download sha256 to temp file
  - Call `VerifyChecksum`; on failure delete both temp files, return `ErrChecksumMismatch`
  - Call `ExtractZip` to `{cacheDir}/`; on failure return error (caller handles cleanup per REQ-GCE-31)
  - Write `{cacheDir}/version.txt` with `strconv.Itoa(version)`
  - Call `progress` callback periodically with bytes downloaded and total content-length (0 if unknown)
- [ ] **2.5** Write `TestDownloadFile` in `downloader_test.go`:
  - Property: server returns random bytes of random length → file written contains exactly those bytes
  - Table-driven: 404 response → error; 200 with empty body → zero-byte file created
- [ ] **2.6** Write `TestVerifyChecksum`:
  - Property: write random bytes to temp file; compute sha256; write correct digest file → verify succeeds
  - Table-driven: wrong digest → `ErrChecksumMismatch`; malformed digest file → error
- [ ] **2.7** Write `TestExtractZip`:
  - Property: create zip with random filenames/content → extract → all files present with correct content
  - Table-driven: path traversal entry (`../../evil`) → error; empty zip → no files, no error
- [ ] **2.8** Write `TestDownloadAndInstall`:
  - Use httptest servers for zip and sha256 URLs
  - Happy path: zip extracted, version.txt written
  - Checksum mismatch: temp files deleted, error returned, no version.txt
- [ ] **2.9** Run tests: `mise exec -- go test ./cmd/ebitenclient/assets/... -v -run TestDownloadFile` etc. — all MUST pass.

---

## Task 3: tiles.yaml Loader and TileRegistry

- [ ] **3.1** Implement `LoadTilesConfig(path string) (*TilesConfig, error)` in `loader.go`:
  - Read YAML file at `path` using `gopkg.in/yaml.v3`
  - Return error if file is missing or YAML is malformed
  - Apply default FPS: after unmarshal, iterate all `NPCs` entries; if `FPS == 0`, set `FPS = 12`
- [ ] **3.2** Implement `TileRegistry` struct in `registry.go` with constructor `NewTileRegistry(cfg *TilesConfig) *TileRegistry`.
- [ ] **3.3** Implement `(r *TileRegistry) LookupZone(zoneName string) TileRef`:
  - Return `cfg.Zones[zoneName]` if present; otherwise return `cfg.Default` (REQ-GCE-19)
- [ ] **3.4** Implement `(r *TileRegistry) LookupNPC(npcType string) AnimRef`:
  - Return `cfg.NPCs[npcType]` if present; otherwise return an `AnimRef` derived from `cfg.Default` with `Frames=1`, `FPS=12`
- [ ] **3.5** Implement `(r *TileRegistry) LookupItem(itemCategory string) TileRef`:
  - Return `cfg.Items[itemCategory]` if present; otherwise return `cfg.Default`
- [ ] **3.6** Write `TestLoadTilesConfig` in `loader_test.go`:
  - Property: generate random TilesConfig, marshal to YAML, write to temp file → unmarshal equals original (round-trip)
  - Table-driven: missing file → error; malformed YAML → error; `fps: 0` or absent → normalised to 12
- [ ] **3.7** Write `TestTileRegistry` in `registry_test.go`:
  - Table-driven: known zone name → correct TileRef; unknown zone name → Default TileRef
  - Table-driven: known NPC type → correct AnimRef with correct FPS; unknown type → default AnimRef with FPS=12
  - Property: for any `TilesConfig` with at least one zone entry, all registered zone names resolve without falling back to default
- [ ] **3.8** Run tests: `mise exec -- go test ./cmd/ebitenclient/assets/... -v -run TestLoadTilesConfig` and `TestTileRegistry` — all MUST pass.

---

## Task 4: colours.yaml Loader

- [ ] **4.1** Implement `LoadColoursConfig(path string) (*ColoursConfig, error)` in `loader.go`:
  - Read YAML file at `path` using `gopkg.in/yaml.v3`
  - Return error if file is missing or YAML is malformed
  - Validate that `Default` field is non-empty; return error if absent
- [ ] **4.2** Implement `(cfg *ColoursConfig) EventColour(eventType string) string` in `loader.go`:
  - Return `cfg.Events[eventType]` if present; otherwise return `cfg.Default`
- [ ] **4.3** Write `TestLoadColoursConfig` in `loader_test.go`:
  - Property: generate random ColoursConfig, marshal to YAML, write to temp file → unmarshal equals original
  - Table-driven: missing file → error; malformed YAML → error; missing `default` key → error
- [ ] **4.4** Write `TestEventColour` in `loader_test.go`:
  - Table-driven: known event type → correct hex colour; unknown event type → Default colour
  - Property: for any ColoursConfig, `EventColour` always returns a non-empty string
- [ ] **4.5** Run tests: `mise exec -- go test ./cmd/ebitenclient/assets/... -v -run TestLoadColoursConfig` and `TestEventColour` — all MUST pass.

---

## Task 5: Malformed tiles.yaml Handling (REQ-GCE-31)

- [ ] **5.1** Implement `ValidatePackIntegrity(cacheDir string) error` in `loader.go`:
  - Attempt `LoadTilesConfig("{cacheDir}/tiles.yaml")`
  - On error (missing or unparseable): call `os.RemoveAll(cacheDir)`, return a wrapped error containing the original cause
  - On success: return nil
- [ ] **5.2** Write `TestValidatePackIntegrity` in `loader_test.go`:
  - Table-driven: valid tiles.yaml in temp dir → nil error, dir still exists
  - Table-driven: missing tiles.yaml → error returned AND temp dir removed
  - Table-driven: malformed tiles.yaml → error returned AND temp dir removed
- [ ] **5.3** Run tests: `mise exec -- go test ./cmd/ebitenclient/assets/... -v -run TestValidatePackIntegrity` — all MUST pass.

---

## Task 6: Download Progress Ebiten Screen

- [ ] **6.1** Create `cmd/ebitenclient/screens/download.go` in package `screens`.
- [ ] **6.2** Define `DownloadScreen` struct:
  ```go
  type DownloadScreen struct {
      version    int
      downloaded int64
      total      int64
      err        error
      done       bool
      retryFn    func()
      transitionFn func() // called when download completes; transitions to login screen
  }
  ```
- [ ] **6.3** Implement `(s *DownloadScreen) Update() error`:
  - No-op if `done` or `err != nil`; Ebiten input not consumed on this screen (passive display)
  - Detect mouse click on Retry button bounds when `err != nil`; call `s.retryFn()` and clear `err`
  - If `done`: call `s.transitionFn()` once
- [ ] **6.4** Implement `(s *DownloadScreen) Draw(screen *ebiten.Image)`:
  - Clear background to dark colour
  - Render centred status text: `"Downloading assets v{N}... X% (Y MB / Z MB)"` where X = `downloaded*100/total` (clamped 0–100; show 0% if total == 0), Y = `downloaded/1_048_576` (truncated), Z = `total/1_048_576` (truncated)
  - When `err != nil`: render error text and a Retry button rectangle with label `"Retry"`
  - When `done` and transition not yet triggered: render `"Assets ready."` briefly before transition fires
- [ ] **6.5** Implement `(s *DownloadScreen) Layout(outsideW, outsideH int) (int, int)` — return `(outsideW, outsideH)`.
- [ ] **6.6** Implement `(s *DownloadScreen) SetProgress(downloaded, total int64)` — safe to call from non-Ebiten goroutine via `atomic` or channel; Ebiten's `Update` reads the latest values.
- [ ] **6.7** Implement `(s *DownloadScreen) SetError(err error)` and `(s *DownloadScreen) SetDone()` — same thread-safety requirement.
- [ ] **6.8** No unit tests for the Ebiten draw path (not testable without a display); ensure package compiles cleanly: `mise exec -- go build ./cmd/ebitenclient/screens/...`.

---

## Task 7: Wire Asset Check into Startup Flow

- [ ] **7.1** In `cmd/ebitenclient/main.go` (created in Phase 1), add `runAssetCheck(cfg *Config, setScreen func(ebiten.Game))` function:
  - Call `FetchRemoteVersion(ctx, cfg.GithubReleasesURL)`
  - On network error: call `ReadLocalVersion(cacheDir)`; if local version > 0 log warning and return nil; if local version == 0 return error
  - Call `ReadLocalVersion(cacheDir)` for comparison
  - If `NeedsUpdate(local, remote)`:
    - Create `DownloadScreen` and call `setScreen` to display it
    - Launch goroutine: call `DownloadAndInstall`; on progress call `screen.SetProgress`; on error call `screen.SetError`; on success call `ValidatePackIntegrity` then `screen.SetDone`
    - If `ValidatePackIntegrity` returns error: call `screen.SetError(err)` (re-download triggered by Retry)
  - If up-to-date: call `ValidatePackIntegrity`; on error trigger download (re-enter flow)
- [ ] **7.2** Ensure `runAssetCheck` is called before the login screen is displayed (Phase 2 wiring point).
- [ ] **7.3** Verify compilation: `mise exec -- go build ./cmd/ebitenclient/...`.

---

## Task 8: assets/version.txt and Makefile Target

- [ ] **8.1** Create `assets/version.txt` with content `1` (no trailing newline).
- [ ] **8.2** Add `make package-assets` target to `Makefile`:
  ```makefile
  .PHONY: package-assets
  package-assets:
  	$(eval VERSION := $(shell cat assets/version.txt))
  	zip -r mud-assets-v$(VERSION).zip assets/
  	sha256sum mud-assets-v$(VERSION).zip > mud-assets-v$(VERSION).sha256
  ```
- [ ] **8.3** Verify target runs without error: `mise exec -- make package-assets` (requires `zip` and `sha256sum` on PATH).
- [ ] **8.4** Run full asset package test suite: `mise exec -- go test ./cmd/ebitenclient/... -v` — all tests MUST pass.

---

## Acceptance Criteria

- REQ-GCE-11: `assets/version.txt` exists; `make package-assets` produces `mud-assets-v1.zip` and `mud-assets-v1.sha256`.
- REQ-GCE-12: `FetchRemoteVersion` + `ReadLocalVersion` + `NeedsUpdate` cover all version-check branches; network-error-with-local and network-error-without-local paths tested.
- REQ-GCE-13: `DownloadScreen` shows Retry button on error; `transitionFn` not called until download + validation succeeds.
- REQ-GCE-14: `VerifyChecksum` detects mismatch; `DownloadAndInstall` deletes temp files on mismatch.
- REQ-GCE-15: `ExtractZip` produces the expected directory tree from a well-formed zip.
- REQ-GCE-16: `LoadTilesConfig` applies default FPS=12; `TileRegistry.LookupZone/NPC/Item` all work.
- REQ-GCE-17: `LoadColoursConfig` + `EventColour` tested; unknown event falls back to default colour.
- REQ-GCE-18: `make package-assets` Makefile target produces release artifacts.
- REQ-GCE-19: `TileRegistry.LookupZone` falls back to `Default` for unrecognised zone names.
- REQ-GCE-31: `ValidatePackIntegrity` deletes extracted directory on missing/malformed `tiles.yaml`.
- All tests pass: `mise exec -- go test ./cmd/ebitenclient/... -v` exits 0.
- Binary compiles: `mise exec -- go build ./cmd/ebitenclient/...` exits 0.

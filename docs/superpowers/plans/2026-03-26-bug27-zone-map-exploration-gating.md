# BUG-27: Zone Map Exploration Gating Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gate danger level and POI display on the zone map behind physical room exploration — rooms revealed by the zone_map item show location only; danger level and POIs only appear after the player visits a room on foot.

**Architecture:** Add an `explored` boolean column to `character_map_rooms` so the DB distinguishes map-known rooms from physically-visited rooms. Surface this as a separate `ExploredCache` on `PlayerSession`. `handleMap()` uses `ExploredCache` membership to decide whether to populate `DangerLevel` and `Pois` fields on each `MapTile`.

**Tech Stack:** Go, PostgreSQL, pgx, rapid (property-based testing)

---

## Root Cause

`AutomapCache` conflates two different discovery mechanisms:

1. **Physical travel** — player moves into a room via `grpc_service_travel.go:116-127`; the room is inserted into `character_map_rooms` and `AutomapCache`.
2. **Zone map item use** — `wireRevealZone()` in `grpc_service.go:5642-5677` bulk-inserts *all* rooms in the zone into `character_map_rooms` and `AutomapCache`.

`handleMap()` (`grpc_service.go:5588-5634`) iterates `AutomapCache[zoneID]` and unconditionally populates `DangerLevel` (line 5599) and `Pois` (lines 5601-5621) for every room, regardless of whether the player physically entered it.

**Fix:** distinguish "known" (map-revealed) from "explored" (physically visited) at the DB and session layer, then gate tile details on the `ExploredCache`.

---

## Files

| Action | Path | Purpose |
|--------|------|---------|
| Create | `migrations/011_character_map_rooms_explored.up.sql` | Add `explored` column (DEFAULT TRUE for existing rows) |
| Create | `migrations/011_character_map_rooms_explored.down.sql` | Drop `explored` column |
| Modify | `internal/storage/postgres/automap.go` | `Insert`/`BulkInsert` gain `explored bool`; `LoadAll` returns two maps |
| Modify | `internal/storage/postgres/automap_test.go` | Update tests for new signatures; add explored-gating tests |
| Modify | `internal/game/session/manager.go` | Add `ExploredCache map[string]map[string]bool` to `PlayerSession` |
| Modify | `internal/gameserver/grpc_service.go` | Login populates `ExploredCache`; `wireRevealZone` passes `explored=false`; `handleMap` gates details |
| Modify | `internal/gameserver/grpc_service_travel.go` | Travel `Insert` passes `explored=true`; populates `ExploredCache` |

---

### Task 1: Database migration — add `explored` column

**Files:**
- Create: `migrations/011_character_map_rooms_explored.up.sql`
- Create: `migrations/011_character_map_rooms_explored.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/011_character_map_rooms_explored.up.sql
ALTER TABLE character_map_rooms
    ADD COLUMN explored BOOLEAN NOT NULL DEFAULT TRUE;
```

`DEFAULT TRUE` grandfathers all existing rows as "explored" so players do not lose their current map detail.

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/011_character_map_rooms_explored.down.sql
ALTER TABLE character_map_rooms DROP COLUMN explored;
```

- [ ] **Step 3: Apply the migration**

```bash
mise exec -- go run ./cmd/migrate up
```

Expected: migration 011 applied with no errors.

- [ ] **Step 4: Commit**

```bash
git add migrations/011_character_map_rooms_explored.up.sql migrations/011_character_map_rooms_explored.down.sql
git commit -m "feat(automap): add explored column to character_map_rooms (BUG-27)"
```

---

### Task 2: Update `AutomapRepository` — `Insert`, `BulkInsert`, `LoadAll`

**Files:**
- Modify: `internal/storage/postgres/automap.go`
- Modify: `internal/storage/postgres/automap_test.go`

#### Step 2a — Write failing tests first

- [ ] **Step 1: Write failing tests**

Open `internal/storage/postgres/automap_test.go`. Add these test cases after the existing ones:

```go
func TestAutomapRepository_Insert_ExploredFalse_NotInExploredMap(t *testing.T) {
	db := testDB(t)
	repo := &AutomapRepository{db: db}
	charID := insertTestCharacter(t, db)

	require.NoError(t, repo.Insert(context.Background(), charID, "zone1", "room1", false))

	all, explored, err := repo.LoadAll(context.Background(), charID)
	require.NoError(t, err)
	assert.True(t, all["zone1"]["room1"], "room1 should appear in all-known map")
	assert.False(t, explored["zone1"]["room1"], "room1 should NOT appear in explored map when inserted with explored=false")
}

func TestAutomapRepository_Insert_ExploredTrue_InBothMaps(t *testing.T) {
	db := testDB(t)
	repo := &AutomapRepository{db: db}
	charID := insertTestCharacter(t, db)

	require.NoError(t, repo.Insert(context.Background(), charID, "zone1", "room2", true))

	all, explored, err := repo.LoadAll(context.Background(), charID)
	require.NoError(t, err)
	assert.True(t, all["zone1"]["room2"], "room2 should appear in all-known map")
	assert.True(t, explored["zone1"]["room2"], "room2 should appear in explored map when inserted with explored=true")
}

func TestAutomapRepository_Insert_ExploreUpgrade_NeverDowngrades(t *testing.T) {
	// A room inserted as explored=true must not become unexplored if inserted again as explored=false.
	db := testDB(t)
	repo := &AutomapRepository{db: db}
	charID := insertTestCharacter(t, db)

	require.NoError(t, repo.Insert(context.Background(), charID, "zone1", "room3", true))
	require.NoError(t, repo.Insert(context.Background(), charID, "zone1", "room3", false))

	_, explored, err := repo.LoadAll(context.Background(), charID)
	require.NoError(t, err)
	assert.True(t, explored["zone1"]["room3"], "explored flag must not be downgraded from true to false")
}

func TestAutomapRepository_BulkInsert_NotExplored_OnlyInAllMap(t *testing.T) {
	db := testDB(t)
	repo := &AutomapRepository{db: db}
	charID := insertTestCharacter(t, db)

	require.NoError(t, repo.BulkInsert(context.Background(), charID, "zone2", []string{"ra", "rb", "rc"}, false))

	all, explored, err := repo.LoadAll(context.Background(), charID)
	require.NoError(t, err)
	for _, id := range []string{"ra", "rb", "rc"} {
		assert.True(t, all["zone2"][id], "%s should be in all-known map", id)
		assert.False(t, explored["zone2"][id], "%s must NOT be in explored map (zone_map reveal)", id)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
mise exec -- go test ./internal/storage/postgres/... -run 'TestAutomapRepository_Insert_ExploredFalse|TestAutomapRepository_Insert_ExploredTrue|TestAutomapRepository_Insert_ExploreUpgrade|TestAutomapRepository_BulkInsert_NotExplored' -v 2>&1 | head -40
```

Expected: compilation errors because `Insert`, `BulkInsert`, and `LoadAll` have wrong signatures.

#### Step 2b — Implement the changes

- [ ] **Step 3: Rewrite `internal/storage/postgres/automap.go`**

Replace the entire file with:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutomapRepository persists per-character room discovery.
type AutomapRepository struct {
	db *pgxpool.Pool
}

// NewAutomapRepository constructs an AutomapRepository backed by db.
func NewAutomapRepository(db *pgxpool.Pool) *AutomapRepository {
	return &AutomapRepository{db: db}
}

// Insert records that characterID has discovered roomID in zoneID.
// explored=true means the player physically entered the room.
// explored=false means the room was revealed by a map item.
// On conflict, explored is set to (existing OR new) — never downgraded.
//
// Precondition: characterID >= 1; zoneID and roomID are non-empty.
func (r *AutomapRepository) Insert(ctx context.Context, characterID int64, zoneID, roomID string, explored bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO character_map_rooms (character_id, zone_id, room_id, explored)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, zone_id, room_id)
		DO UPDATE SET explored = character_map_rooms.explored OR EXCLUDED.explored`,
		characterID, zoneID, roomID, explored,
	)
	if err != nil {
		return fmt.Errorf("inserting map room (%d, %q, %q, explored=%v): %w", characterID, zoneID, roomID, explored, err)
	}
	return nil
}

// BulkInsert calls Insert for each roomID with the given explored value.
// explored=false is used for zone_map item reveals; explored=true for physical travel.
//
// Precondition: characterID >= 1; zoneID is non-empty.
func (r *AutomapRepository) BulkInsert(ctx context.Context, characterID int64, zoneID string, roomIDs []string, explored bool) error {
	for _, roomID := range roomIDs {
		if err := r.Insert(ctx, characterID, zoneID, roomID, explored); err != nil {
			return err
		}
	}
	return nil
}

// LoadAll returns two caches for characterID:
//   - allKnown: every room the character has seen (map-revealed or physically visited)
//   - exploredOnly: rooms the character physically entered
//
// Precondition: characterID >= 1.
// Postcondition: both maps are non-nil (may be empty); never returns nil, nil, nil.
func (r *AutomapRepository) LoadAll(ctx context.Context, characterID int64) (allKnown, exploredOnly map[string]map[string]bool, err error) {
	rows, queryErr := r.db.Query(ctx, `
		SELECT zone_id, room_id, explored FROM character_map_rooms WHERE character_id = $1`,
		characterID,
	)
	if queryErr != nil {
		return nil, nil, fmt.Errorf("loading map rooms for character %d: %w", characterID, queryErr)
	}
	defer rows.Close()

	allKnown = make(map[string]map[string]bool)
	exploredOnly = make(map[string]map[string]bool)
	for rows.Next() {
		var zoneID, roomID string
		var roomExplored bool
		if scanErr := rows.Scan(&zoneID, &roomID, &roomExplored); scanErr != nil {
			return nil, nil, fmt.Errorf("scanning map room row: %w", scanErr)
		}
		if allKnown[zoneID] == nil {
			allKnown[zoneID] = make(map[string]bool)
		}
		allKnown[zoneID][roomID] = true
		if roomExplored {
			if exploredOnly[zoneID] == nil {
				exploredOnly[zoneID] = make(map[string]bool)
			}
			exploredOnly[zoneID][roomID] = true
		}
	}
	return allKnown, exploredOnly, rows.Err()
}
```

- [ ] **Step 4: Run new tests — expect pass**

```bash
mise exec -- go test ./internal/storage/postgres/... -run 'TestAutomapRepository_Insert_ExploredFalse|TestAutomapRepository_Insert_ExploredTrue|TestAutomapRepository_Insert_ExploreUpgrade|TestAutomapRepository_BulkInsert_NotExplored' -v 2>&1 | tail -20
```

Expected: all 4 new tests PASS.

- [ ] **Step 5: Fix existing tests broken by signature change**

The existing tests call `repo.Insert(ctx, charID, zone, room)` (3 args after ctx) and `repo.BulkInsert(ctx, charID, zone, rooms)` (no explored arg). Update each call site in `automap_test.go`:

- `repo.Insert(ctx, charID, zone, room)` → `repo.Insert(ctx, charID, zone, room, true)`
- `repo.BulkInsert(ctx, charID, zone, rooms)` → `repo.BulkInsert(ctx, charID, zone, rooms, true)`
- `repo.LoadAll(ctx, charID)` returns 3 values now — update callers: `all, _, err := repo.LoadAll(...)` where the explored map isn't needed in the old tests.

- [ ] **Step 6: Run all automap repo tests**

```bash
mise exec -- go test ./internal/storage/postgres/... -v 2>&1 | tail -30
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/storage/postgres/automap.go internal/storage/postgres/automap_test.go
git commit -m "feat(automap): add explored bool to Insert/BulkInsert; LoadAll returns allKnown+exploredOnly (BUG-27)"
```

---

### Task 3: Add `ExploredCache` to `PlayerSession`

**Files:**
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Add the field**

In `internal/game/session/manager.go`, find the `AutomapCache` field (around line 79) and add `ExploredCache` immediately after:

```go
// AutomapCache holds discovered rooms keyed by zone ID then room ID.
// Populated at login from the database; written through on each new discovery.
AutomapCache map[string]map[string]bool

// ExploredCache holds rooms the player has physically entered (not just map-revealed),
// keyed by zone ID then room ID. Used to gate danger level and POI display on the map.
ExploredCache map[string]map[string]bool
```

- [ ] **Step 2: Build to catch compile errors**

```bash
mise exec -- go build ./... 2>&1 | head -30
```

Expected: compilation errors in `grpc_service.go` and `grpc_service_travel.go` due to `LoadAll` signature change.

- [ ] **Step 3: Commit**

```bash
git add internal/game/session/manager.go
git commit -m "feat(session): add ExploredCache field to PlayerSession (BUG-27)"
```

---

### Task 4: Update login — populate both caches from `LoadAll`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (login section, ~line 782)

- [ ] **Step 1: Find the LoadAll call at login**

In `grpc_service.go` around line 782:
```go
discovered, loadErr := s.automapRepo.LoadAll(stream.Context(), characterID)
if loadErr != nil {
    s.logger.Warn("loading automap", zap.Error(loadErr))
} else {
    sess.AutomapCache = discovered
}
```

- [ ] **Step 2: Replace with two-cache version**

```go
allKnown, exploredOnly, loadErr := s.automapRepo.LoadAll(stream.Context(), characterID)
if loadErr != nil {
    s.logger.Warn("loading automap", zap.Error(loadErr))
} else {
    sess.AutomapCache = allKnown
    sess.ExploredCache = exploredOnly
}
```

- [ ] **Step 3: Build**

```bash
mise exec -- go build ./... 2>&1 | head -30
```

Expected: fewer errors — login now compiles. Remaining errors are in travel and wireRevealZone.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "fix(login): populate ExploredCache from LoadAll at character login (BUG-27)"
```

---

### Task 5: Update travel — mark rooms as `explored=true`

**Files:**
- Modify: `internal/gameserver/grpc_service_travel.go` (~line 116)

- [ ] **Step 1: Find the travel discovery block**

Around line 116 in `grpc_service_travel.go`:
```go
if !sess.AutomapCache[destRoom.ZoneID][destRoom.ID] {
    sess.AutomapCache[destRoom.ZoneID][destRoom.ID] = true
    if s.automapRepo != nil {
        if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, destRoom.ZoneID, destRoom.ID); err != nil {
            s.logger.Warn("persisting travel room discovery", zap.Error(err))
        }
    }
}
```

- [ ] **Step 2: Replace with explored=true and dual-cache update**

```go
if !sess.AutomapCache[destRoom.ZoneID][destRoom.ID] {
    sess.AutomapCache[destRoom.ZoneID][destRoom.ID] = true
    if s.automapRepo != nil {
        if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, destRoom.ZoneID, destRoom.ID, true); err != nil {
            s.logger.Warn("persisting travel room discovery", zap.Error(err))
        }
    }
}
// Always mark as explored on physical entry (may have been map-revealed previously).
if sess.ExploredCache == nil {
    sess.ExploredCache = make(map[string]map[string]bool)
}
if sess.ExploredCache[destRoom.ZoneID] == nil {
    sess.ExploredCache[destRoom.ZoneID] = make(map[string]bool)
}
if !sess.ExploredCache[destRoom.ZoneID][destRoom.ID] {
    sess.ExploredCache[destRoom.ZoneID][destRoom.ID] = true
    // If room was already in AutomapCache (map-revealed), upgrade explored flag in DB.
    if sess.AutomapCache[destRoom.ZoneID][destRoom.ID] && s.automapRepo != nil {
        if err := s.automapRepo.Insert(context.Background(), sess.CharacterID, destRoom.ZoneID, destRoom.ID, true); err != nil {
            s.logger.Warn("upgrading room to explored", zap.Error(err))
        }
    }
}
```

- [ ] **Step 3: Build**

```bash
mise exec -- go build ./... 2>&1 | head -30
```

Expected: travel compiles. Only wireRevealZone remains broken.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service_travel.go
git commit -m "fix(travel): mark rooms explored=true on physical entry; upgrade map-revealed rooms (BUG-27)"
```

---

### Task 6: Update `wireRevealZone` — bulk insert with `explored=false`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (wireRevealZone, ~line 5672)

- [ ] **Step 1: Find the BulkInsert call in wireRevealZone**

```go
if err := s.automapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, roomIDs); err != nil {
```

- [ ] **Step 2: Add `false` as the explored argument**

```go
if err := s.automapRepo.BulkInsert(context.Background(), sess.CharacterID, zoneID, roomIDs, false); err != nil {
```

Note: `wireRevealZone` must NOT populate `sess.ExploredCache` — that is intentional; rooms remain unexplored until physically entered.

- [ ] **Step 3: Build — should be clean**

```bash
mise exec -- go build ./... 2>&1 | head -30
```

Expected: zero errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "fix(zone-map): reveal_zone inserts rooms with explored=false; ExploredCache not updated (BUG-27)"
```

---

### Task 7: Gate `DangerLevel` and `Pois` on `ExploredCache` in `handleMap`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleMap, lines 5595-5633)

- [ ] **Step 1: Write a failing property test**

In `internal/gameserver/` find the existing test file for handleMap (likely `grpc_service_test.go` or a dedicated map test). Add:

```go
func TestHandleMap_ZoneMapRevealedRooms_HideDetailsUntilExplored(t *testing.T) {
    // Build a minimal GameServiceServer with a two-room zone.
    // Mark room A as map-revealed only (AutomapCache only, not ExploredCache).
    // Mark room B as physically explored (both caches).
    // Call handleMap and assert:
    //   - Room A tile: DangerLevel == "" and Pois is empty.
    //   - Room B tile: DangerLevel != "" and/or Pois may be populated.

    zone := &world.Zone{
        ID:          "testzone",
        Name:        "Test Zone",
        DangerLevel: "dangerous",
        Rooms: map[string]*world.Room{
            "roomA": {ID: "roomA", Title: "Room A", MapX: 0, MapY: 0, Exits: nil},
            "roomB": {ID: "roomB", Title: "Room B", MapX: 1, MapY: 0, Exits: nil},
        },
    }
    w := world.NewWorldFromZones([]*world.Zone{zone})
    sess := &session.PlayerSession{
        RoomID: "roomA",
        AutomapCache: map[string]map[string]bool{
            "testzone": {"roomA": true, "roomB": true},
        },
        ExploredCache: map[string]map[string]bool{
            "testzone": {"roomB": true}, // roomA is map-revealed only
        },
    }
    sessions := session.NewManagerWithSessions(map[string]*session.PlayerSession{"uid1": sess})
    svc := &GameServiceServer{world: w, sessions: sessions}

    evt, err := svc.handleMap("uid1", &gamev1.MapRequest{})
    require.NoError(t, err)
    tiles := evt.GetMap().GetTiles()

    tileByID := make(map[string]*gamev1.MapTile)
    for _, tile := range tiles {
        tileByID[tile.RoomId] = tile
    }

    tileA := tileByID["roomA"]
    require.NotNil(t, tileA, "roomA must appear on map (it is in AutomapCache)")
    assert.Empty(t, tileA.DangerLevel, "roomA DangerLevel must be empty before exploration")
    assert.Empty(t, tileA.Pois, "roomA Pois must be empty before exploration")

    tileB := tileByID["roomB"]
    require.NotNil(t, tileB, "roomB must appear on map")
    assert.Equal(t, "dangerous", tileB.DangerLevel, "roomB DangerLevel must be populated after exploration")
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleMap_ZoneMapRevealedRooms_HideDetailsUntilExplored -v 2>&1 | tail -20
```

Expected: FAIL — roomA.DangerLevel is "dangerous" instead of "".

- [ ] **Step 3: Update `handleMap` to gate details on `ExploredCache`**

In `grpc_service.go`, inside the zone-map tile-building loop (around lines 5595-5633), replace the unconditional detail population:

```go
// Before:
effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, r.DangerLevel)

// Collect POI type IDs for this explored room (REQ-POI-15..18).
poiSet := make(map[string]bool)
// ... POI collection ...
poiSlice = maputil.SortPOIs(poiSlice)

tiles = append(tiles, &gamev1.MapTile{
    RoomId:      r.ID,
    RoomName:    r.Title,
    X:           int32(r.MapX),
    Y:           int32(r.MapY),
    Current:     r.ID == sess.RoomID,
    Exits:       exits,
    DangerLevel: string(effectiveLevel),
    Pois:        poiSlice,
    BossRoom:    r.BossRoom,
})
```

Replace with:

```go
// Only reveal danger level and POIs for rooms the player has physically explored.
// Map-revealed rooms (via zone_map item) show location only.
physicallyExplored := sess.ExploredCache[zoneID][r.ID]

var dangerLevel string
var poiSlice []string
if physicallyExplored {
    effectiveLevel := danger.EffectiveDangerLevel(zone.DangerLevel, r.DangerLevel)
    dangerLevel = string(effectiveLevel)

    // Collect POI type IDs for this explored room (REQ-POI-15..18).
    poiSet := make(map[string]bool)
    if s.npcMgr != nil {
        for _, inst := range s.npcMgr.InstancesInRoom(r.ID) {
            if inst.IsDead() || inst.NpcRole == "" {
                continue
            }
            poiID := maputil.NpcRoleToPOIID(inst.NpcRole)
            if poiID != "" {
                poiSet[poiID] = true
            }
        }
    }
    if len(r.Equipment) > 0 {
        poiSet["equipment"] = true
    }
    poiSlice = make([]string, 0, len(poiSet))
    for id := range poiSet {
        poiSlice = append(poiSlice, id)
    }
    poiSlice = maputil.SortPOIs(poiSlice)
}

tiles = append(tiles, &gamev1.MapTile{
    RoomId:      r.ID,
    RoomName:    r.Title,
    X:           int32(r.MapX),
    Y:           int32(r.MapY),
    Current:     r.ID == sess.RoomID,
    Exits:       exits,
    DangerLevel: dangerLevel,
    Pois:        poiSlice,
    BossRoom:    r.BossRoom,
})
```

- [ ] **Step 4: Run the new test — expect pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleMap_ZoneMapRevealedRooms_HideDetailsUntilExplored -v 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 5: Run the full test suite**

```bash
mise exec -- go test ./... 2>&1 | tail -30
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "fix(map): gate DangerLevel and Pois on ExploredCache — fixes BUG-27"
```

---

### Task 8: Update `docs/bugs.md` — record root cause and link plan

**Files:**
- Modify: `docs/bugs.md`

- [ ] **Step 1: Update BUG-27 entry**

Find the BUG-27 entry and update the description and add a Root Cause and Plan link:

```markdown
### BUG-27: Zone map exposes danger level and POIs for unexplored rooms
**Severity:** medium
**Status:** in_progress
**Category:** World
**Description:** Using the Zone Map reveals danger level and points of interest for all rooms in the zone, including rooms the player has never visited; only room existence should be revealed — danger level and POIs must require the player to actually explore each room.
**Root Cause:** `AutomapCache` conflates map-revealed rooms (via `wireRevealZone`) with physically-visited rooms (via travel). `handleMap()` iterates `AutomapCache` and unconditionally populates `DangerLevel` and `Pois` for every room regardless of how it was discovered (`grpc_service.go:5599-5621`).
**Plan:** `docs/superpowers/plans/2026-03-26-bug27-zone-map-exploration-gating.md`
**Steps:** Use zone_map item in any zone; observe map output; rooms never physically entered show danger level and POI symbols.
**Fix:**
```

- [ ] **Step 2: Commit**

```bash
git add docs/bugs.md
git commit -m "docs(bugs): document root cause and plan link for BUG-27"
```

---

## Self-Review

### Spec coverage
- [x] Zone map reveals all rooms → rooms still appear in `AutomapCache`, shown on map
- [x] Danger level hidden until explored → gated on `ExploredCache` in handleMap
- [x] POIs hidden until explored → same gate
- [x] Existing explored rooms unaffected → `DEFAULT TRUE` migration grandfathers all rows
- [x] Previously map-revealed rooms upgrade to explored on physical entry → travel block handles upgrade insert
- [x] Zone map re-use after exploring some rooms → `ON CONFLICT DO UPDATE SET explored = explored OR EXCLUDED.explored` preserves explored flag

### Placeholder scan
None found.

### Type consistency
- `AutomapRepository.Insert` signature is consistent across Tasks 2, 5, 6
- `AutomapRepository.BulkInsert` signature consistent across Tasks 2, 6
- `AutomapRepository.LoadAll` returns `(allKnown, exploredOnly map[string]map[string]bool, err error)` consistently across Tasks 2, 4
- `sess.ExploredCache` field added in Task 3, used in Tasks 4, 5, 7

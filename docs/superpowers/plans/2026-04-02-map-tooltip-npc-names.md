# Map Tooltip NPC Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show NPC names alongside POI labels in the zone map room tooltip (e.g., "$ Merchant — Sgt. Mack" instead of just "$ Merchant").

**Architecture:** Add a `PoiWithNpc` message to the proto with `poi_id` and `npc_name` fields, add a `repeated PoiWithNpc poi_npcs` field to `MapTile`. The gameserver populates it from live NPC instances (which already have names available). The `RoomTooltip` component reads the new field and appends the NPC name when present.

**Tech Stack:** Protocol Buffers 3, Go (gameserver), TypeScript/React (webclient)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `api/proto/game/v1/game.proto` | Add `PoiWithNpc` message; add `poi_npcs` field to `MapTile` |
| Modify | `internal/gameserver/gamev1/game.pb.go` | Regenerated via `make proto` — do not edit by hand |
| Modify | `internal/gameserver/grpc_service.go` | Populate `PoiNpcs` in `handleMap` from NPC instance names |
| Modify | `cmd/webclient/ui/src/proto/index.ts` | Add `PoiWithNpc` interface; add `poiNpcs`/`poi_npcs` to `MapTile` |
| Modify | `cmd/webclient/ui/src/game/RoomTooltip.tsx` | Read `poiNpcs` and render NPC name after label |
| Modify | `cmd/webclient/ui/src/game/RoomTooltip.test.tsx` | Add/update tests for NPC name rendering |

---

### Task 1: Add PoiWithNpc to proto and regenerate

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify (generated): `internal/gameserver/gamev1/game.pb.go`

The current `MapTile` message ends at field 9 (`boss_room`). We add a new `PoiWithNpc` message and field 10.

- [ ] **Step 1: Edit `api/proto/game/v1/game.proto`**

Find the `MapTile` message (around line 729) and the line after it. Insert the new message *before* `MapTile` and add field 10 to `MapTile`:

```protobuf
// PoiWithNpc pairs a POI type ID with the name of the NPC that provides it.
message PoiWithNpc {
    string poi_id   = 1; // e.g. "merchant", "healer", "guard"
    string npc_name = 2; // NPC display name e.g. "Sgt. Mack"
}

// MapTile represents one discovered room on the automap grid.
message MapTile {
    string room_id   = 1;
    string room_name = 2;
    int32  x         = 3;
    int32  y         = 4;
    bool   current   = 5;
    repeated string exits = 6;
    string danger_level  = 7;
    repeated string pois = 8; // POI type IDs present in this room e.g. ["merchant", "equipment"]
    bool   boss_room     = 9; // true when this room is a boss arena (REQ-AE-24)
    repeated PoiWithNpc poi_npcs = 10; // NPC-backed POIs with their display names
}
```

- [ ] **Step 2: Regenerate Go proto bindings**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected output: no errors. The file `internal/gameserver/gamev1/game.pb.go` will be updated to include `PoiWithNpc` struct and `PoiNpcs []*PoiWithNpc` field on `MapTile`.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/cjohannsen/src/mud && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): add PoiWithNpc message and poi_npcs field to MapTile"
```

---

### Task 2: Populate poi_npcs in the gameserver map handler

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (around line 6420)

The current code builds `poiSet` (a `map[string]bool`) from live NPC instances but discards the instance names. We add a parallel `poiNpcs` slice that records the NPC name for each NPC-backed POI.

- [ ] **Step 1: Locate the exact code to change**

The block to replace is in `handleMap` (around lines 6420–6463):

```go
// Collect POI type IDs for this explored room (REQ-POI-15..18).
poiSet := make(map[string]bool)
if s.npcMgr != nil {
    for _, inst := range s.npcMgr.InstancesInRoom(r.ID) {
        if inst.IsDead() {
            continue
        }
        role := inst.NpcRole
        if role == "" {
            role = maputil.POIRoleFromNPCType(inst.NPCType)
        }
        poiID := maputil.NpcRoleToPOIID(role)
        if poiID != "" {
            poiSet[poiID] = true
        }
    }
}
for _, eq := range r.Equipment {
    switch {
    case eq.ItemID == "zone_map":
        poiSet["map"] = true
    case eq.CoverTier != "":
        poiSet["cover"] = true
    default:
        poiSet["equipment"] = true
    }
}
for id := range poiSet {
    poiSlice = append(poiSlice, id)
}
poiSlice = maputil.SortPOIs(poiSlice)
```

And the `MapTile` construction that follows:
```go
tiles = append(tiles, &gamev1.MapTile{
    RoomId:      r.ID,
    RoomName:    r.Title,
    X:           int32(r.MapX),
    Y:           int32(r.MapY),
    Current:     r.ID == sess.RoomID,
    Exits:       exits,
    DangerLevel: effectiveLevelStr,
    Pois:        poiSlice,
    BossRoom:    r.BossRoom,
})
```

- [ ] **Step 2: Write the updated code**

Replace the POI collection block and MapTile construction:

```go
// Collect POI type IDs for this explored room (REQ-POI-15..18).
poiSet := make(map[string]bool)
var poiNpcs []*gamev1.PoiWithNpc
if s.npcMgr != nil {
    for _, inst := range s.npcMgr.InstancesInRoom(r.ID) {
        if inst.IsDead() {
            continue
        }
        role := inst.NpcRole
        if role == "" {
            role = maputil.POIRoleFromNPCType(inst.NPCType)
        }
        poiID := maputil.NpcRoleToPOIID(role)
        if poiID != "" {
            poiSet[poiID] = true
            poiNpcs = append(poiNpcs, &gamev1.PoiWithNpc{
                PoiId:   poiID,
                NpcName: inst.Name(),
            })
        }
    }
}
for _, eq := range r.Equipment {
    switch {
    case eq.ItemID == "zone_map":
        poiSet["map"] = true
    case eq.CoverTier != "":
        poiSet["cover"] = true
    default:
        poiSet["equipment"] = true
    }
}
for id := range poiSet {
    poiSlice = append(poiSlice, id)
}
poiSlice = maputil.SortPOIs(poiSlice)
```

And update the MapTile construction:

```go
tiles = append(tiles, &gamev1.MapTile{
    RoomId:      r.ID,
    RoomName:    r.Title,
    X:           int32(r.MapX),
    Y:           int32(r.MapY),
    Current:     r.ID == sess.RoomID,
    Exits:       exits,
    DangerLevel: effectiveLevelStr,
    Pois:        poiSlice,
    BossRoom:    r.BossRoom,
    PoiNpcs:     poiNpcs,
})
```

- [ ] **Step 3: Run the Go test suite**

```bash
cd /home/cjohannsen/src/mud && make test
```

Expected: all tests pass (including existing map-related tests). The change is additive — `PoiNpcs` is nil when no NPC-backed POIs exist, which is a valid zero value for a repeated field.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(map): populate poi_npcs with NPC display names in zone map tiles"
```

---

### Task 3: Add PoiWithNpc TypeScript interface and update MapTile

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts`

The current `MapTile` interface (line 276) ends at `name?: string`. We add a `PoiWithNpc` interface and two new fields on `MapTile`.

- [ ] **Step 1: Add `PoiWithNpc` interface before `MapTile`**

In `cmd/webclient/ui/src/proto/index.ts`, insert this directly before the `export interface MapTile` declaration:

```typescript
export interface PoiWithNpc {
  poiId?: string
  poi_id?: string
  npcName?: string
  npc_name?: string
}
```

- [ ] **Step 2: Add `poiNpcs` and `poi_npcs` fields to `MapTile`**

In the `MapTile` interface, add after `name?: string`:

```typescript
  poiNpcs?: PoiWithNpc[]
  poi_npcs?: PoiWithNpc[]
```

After both changes, `MapTile` should look like:

```typescript
export interface PoiWithNpc {
  poiId?: string
  poi_id?: string
  npcName?: string
  npc_name?: string
}

export interface MapTile {
  roomId?: string
  roomName?: string
  x?: number
  y?: number
  current?: boolean
  exits?: string[]
  dangerLevel?: string
  danger_level?: string
  pois?: string[]
  bossRoom?: boolean
  boss?: boolean
  name?: string
  poiNpcs?: PoiWithNpc[]
  poi_npcs?: PoiWithNpc[]
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build
```

Expected: `✓ built in` — no type errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/webclient/ui/src/proto/index.ts
git commit -m "feat(proto/ts): add PoiWithNpc interface and poi_npcs field to MapTile"
```

---

### Task 4: Update RoomTooltip to display NPC names

**Files:**
- Modify: `cmd/webclient/ui/src/game/RoomTooltip.tsx`
- Modify: `cmd/webclient/ui/src/game/RoomTooltip.test.tsx`

The current POI rendering in `RoomTooltip` shows only the label. We add NPC name lookup from `tile.poiNpcs ?? tile.poi_npcs`.

- [ ] **Step 1: Write failing tests first**

In `cmd/webclient/ui/src/game/RoomTooltip.test.tsx`, add these tests to the existing `describe('RoomTooltip', ...)` block:

```typescript
  it('shows NPC name alongside POI label when poiNpcs is provided', () => {
    const tileWithNpcs: MapTile = {
      ...tile,
      pois: ['merchant'],
      poiNpcs: [{ poiId: 'merchant', npcName: 'Sgt. Mack' }],
    }
    render(<RoomTooltip tile={tileWithNpcs} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText(/Merchant.*Sgt\. Mack/)).toBeDefined()
  })

  it('shows multiple NPC names comma-separated for same POI type', () => {
    const tileWithMultiple: MapTile = {
      ...tile,
      pois: ['merchant'],
      poiNpcs: [
        { poiId: 'merchant', npcName: 'Sgt. Mack' },
        { poiId: 'merchant', npcName: 'Ellie Mack' },
      ],
    }
    render(<RoomTooltip tile={tileWithMultiple} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText(/Sgt\. Mack/)).toBeDefined()
    expect(screen.getByText(/Ellie Mack/)).toBeDefined()
  })

  it('shows label only (no dash) when no poiNpcs entry for that POI type', () => {
    const tileWithEquipment: MapTile = {
      ...tile,
      pois: ['map'],
      poiNpcs: [],
    }
    render(<RoomTooltip tile={tileWithEquipment} pos={{ x: 100, y: 200 }} />)
    // "Map" label is shown with no " — NpcName" suffix
    expect(screen.getByText('Map')).toBeDefined()
    expect(screen.queryByText(/Map.*—/)).toBeNull()
  })

  it('uses poi_id snake_case fallback in poiNpcs entries', () => {
    const tileSnakeCase: MapTile = {
      ...tile,
      pois: ['healer'],
      poi_npcs: [{ poi_id: 'healer', npc_name: 'Tina Wires' }],
    }
    render(<RoomTooltip tile={tileSnakeCase} pos={{ x: 100, y: 200 }} />)
    expect(screen.getByText(/Healer.*Tina Wires/)).toBeDefined()
  })
```

Also add the `PoiWithNpc` import at the top of the test file (after `MapTile`):

```typescript
import type { MapTile } from '../proto'
```

(No change needed — `PoiWithNpc` is inlined via `MapTile.poiNpcs` typing.)

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm test -- RoomTooltip.test.tsx
```

Expected: the 4 new tests FAIL (tooltip still shows label without NPC name).

- [ ] **Step 3: Update RoomTooltip.tsx**

In `cmd/webclient/ui/src/game/RoomTooltip.tsx`, add `PoiWithNpc` to the proto import:

```typescript
import type { MapTile, PoiWithNpc } from '../proto'
```

Inside the `RoomTooltip` function body, after the existing variable extractions, add:

```typescript
  const poiNpcs: PoiWithNpc[] = tile.poiNpcs ?? tile.poi_npcs ?? []
```

Replace the POI rendering block (the `{pois.length > 0 && (...)}` section) with:

```tsx
      {/* POIs */}
      {pois.length > 0 && (
        <div style={{ marginBottom: '0.2rem' }}>
          <div style={{ color: '#666', marginBottom: '0.1rem' }}>Points of Interest:</div>
          {pois.map(id => {
            const pt = POI_TYPES.find(p => p.id === id)
            const matching = poiNpcs.filter(p => (p.poiId ?? p.poi_id) === id)
            const npcLabel = matching.length > 0
              ? matching.map(p => p.npcName ?? p.npc_name ?? '').filter(Boolean).join(', ')
              : ''
            return (
              <div key={id} style={{ paddingLeft: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                <span style={{ color: pt?.color ?? '#ccc' }}>{pt?.symbol ?? '?'}</span>
                <span>
                  {pt?.label ?? id}
                  {npcLabel && <span style={{ color: '#aaa' }}> — {npcLabel}</span>}
                </span>
              </div>
            )
          })}
        </div>
      )}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm test -- RoomTooltip.test.tsx
```

Expected: all tests PASS (original 10 + 4 new = 14 tests).

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm test
```

Expected: all tests pass (25 total across 4 test files).

- [ ] **Step 6: Commit**

```bash
git add cmd/webclient/ui/src/game/RoomTooltip.tsx cmd/webclient/ui/src/game/RoomTooltip.test.tsx
git commit -m "feat(map): show NPC names alongside POI labels in room tooltip"
```

---

### Task 5: Build and deploy

**Files:** None (build + deploy only)

- [ ] **Step 1: Full TypeScript build**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build
```

Expected: `✓ built in` — no type errors.

- [ ] **Step 2: Deploy**

```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy
```

Expected: All three images built and pushed; `Release "mud" has been upgraded. Happy Helming!`

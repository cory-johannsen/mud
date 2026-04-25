# Structured Battle Maps with Adjustable Size — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hard-coded 20×20 combat grid with a per-room `CombatMap` block. Add the model + loader validation, the `StartCombat` integration that reads spawn cells / stack axis, the telnet renderer's runtime-dimensions adoption, and an admin UI tab for authoring. Web client already adapts to wire-payload `gridWidth/Height` — just verify nothing is hard-coded to 20.

**Spec:** [docs/superpowers/specs/2026-04-25-structured-battle-maps.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-structured-battle-maps.md) (PR [#291](https://github.com/cory-johannsen/mud/pull/291))

**Architecture:** Five small surgical changes. (1) `Room.CombatMap *CombatMap` struct with width/height/spawns/stack axis + loader validation (`[5,30]`, in-bounds spawns, distinct spawn cells). (2) `engine.go:StartCombat` reads `room.CombatMap` (or default 20×20), places players/NPCs at declared cells with stacking + boundary clamping, drops out-of-bounds cover/hazard cells with warning. (3) Telnet renderer uses `cbt.GridWidth/Height` instead of constants; clipping strategy for terminal-narrower-than-grid is half-width characters per Q1. (4) Web client already adapts; verify auto-fit covers 5×5 to 30×30 and confirm AoE / movement bounds don't hard-code 20. (5) Admin UI tab with width/height inputs, preview-grid spawn-cell pickers, and a small scaled preview to catch unreadable 30×30 authoring.

**Tech Stack:** Go (`internal/game/world/`, `internal/game/combat/`, `internal/frontend/telnet/`), `pgregory.net/rapid` for property tests, React/TypeScript (`cmd/webclient/ui/src/admin/`).

**Prerequisite:** None hard. #250 (AoE) and #251 (NPC movement) plans both consume `cbt.GridWidth/Height`; this plan verifies they remain dimension-agnostic.

**Note on spec PR**: Spec is on PR #291, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/world/model.go` (`Room.CombatMap`; `CombatMap` struct; `Cell` struct) |
| Modify | `internal/game/world/loader.go` and `loader_test.go` (validation) |
| Modify | `internal/game/combat/engine.go` (`StartCombat` read+placement) |
| Modify | `internal/game/combat/engine_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestStartCombat_GridSize_Property/` |
| Modify | `internal/frontend/telnet/combat_grid.go` (use runtime dimensions; clipping strategy) |
| Modify | `internal/frontend/telnet/combat_grid_test.go` |
| Modify | `cmd/webclient/ui/src/game/panels/MapPanel.tsx` (cell-size auto-fit smoke test only) |
| Modify | `cmd/webclient/ui/src/game/panels/MapPanel.test.tsx` (range coverage 5×5 to 30×30) |
| Create | `cmd/webclient/ui/src/admin/RoomCombatMapTab.tsx` |
| Create | `cmd/webclient/ui/src/admin/RoomCombatMapTab.test.tsx` |
| Modify | `api/proto/game/v1/game.proto` (verify `grid_width` / `grid_height` already on `CombatStartView`) |
| Modify | `docs/architecture/combat.md` |
| Optional | `tools/world_lint/combat_map_check.go` (BMAP-Q2 linter) |

---

### Task 1: Room schema — `CombatMap` struct + loader validation

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 1: Failing tests** (BMAP-1, BMAP-2):

```go
func TestLoadRoom_CombatMapDefaults(t *testing.T) {
    r, _ := world.LoadRoom([]byte(`
id: test_room
combat_map: {}
`))
    require.Equal(t, 20, r.CombatMap.Width)
    require.Equal(t, 20, r.CombatMap.Height)
    require.Equal(t, world.Cell{X: 0,  Y: 10}, r.CombatMap.PlayerSpawn)
    require.Equal(t, world.Cell{X: 19, Y: 10}, r.CombatMap.NPCSpawn)
    require.Equal(t, world.AxisY, r.CombatMap.NPCStackAxis)
}

func TestLoadRoom_CombatMapNarrowDimensions(t *testing.T) {
    r, _ := world.LoadRoom([]byte(`
id: alley
combat_map:
  width: 12
  height: 8
  player_spawn: { x: 1, y: 4 }
  npc_spawn: { x: 10, y: 4 }
  npc_stack_axis: y
`))
    require.Equal(t, 12, r.CombatMap.Width)
    require.Equal(t, world.Cell{X: 10, Y: 4}, r.CombatMap.NPCSpawn)
}

func TestLoadRoom_CombatMapRejectsOutOfBoundsSpawn(t *testing.T) {
    _, err := world.LoadRoom(yamlWithSpawn(world.Cell{X: 99, Y: 0}, world.Cell{X: 0, Y: 0}))
    require.Error(t, err)
    require.Contains(t, err.Error(), "player_spawn")
}

func TestLoadRoom_CombatMapRejectsCoincidentSpawns(t *testing.T) {
    _, err := world.LoadRoom(yamlWithSpawn(world.Cell{X: 5, Y: 5}, world.Cell{X: 5, Y: 5}))
    require.Error(t, err)
    require.Contains(t, err.Error(), "must differ")
}

func TestLoadRoom_CombatMapRejectsTooSmallTooLarge(t *testing.T) {
    for _, dim := range []int{4, 31, 100} {
        _, err := world.LoadRoom(yamlWithDim(dim, dim))
        require.Error(t, err, "dim %d must error", dim)
    }
}

func TestLoadRoom_NoCombatMapBlock_LeavesNil(t *testing.T) {
    r, _ := world.LoadRoom([]byte(`id: plain_room`))
    require.Nil(t, r.CombatMap)
}
```

- [ ] **Step 2: Implement** the struct + validation:

```go
type Cell struct{ X, Y int }

type Axis int
const (
    AxisX Axis = iota
    AxisY
)

type CombatMap struct {
    Width        int
    Height       int
    PlayerSpawn  Cell
    NPCSpawn     Cell
    NPCStackAxis Axis
}

type Room struct {
    // ... existing ...
    CombatMap *CombatMap
}

func (cm *CombatMap) ApplyDefaults() {
    if cm.Width  == 0 { cm.Width  = 20 }
    if cm.Height == 0 { cm.Height = 20 }
    if cm.PlayerSpawn == (Cell{}) { cm.PlayerSpawn = Cell{X: 0,           Y: cm.Height / 2} }
    if cm.NPCSpawn   == (Cell{}) { cm.NPCSpawn   = Cell{X: cm.Width - 1, Y: cm.Height / 2} }
}

func (cm *CombatMap) Validate() error {
    if cm.Width < 5 || cm.Width > 30 { return fmt.Errorf("combat_map.width %d out of [5,30]", cm.Width) }
    if cm.Height < 5 || cm.Height > 30 { return fmt.Errorf("combat_map.height %d out of [5,30]", cm.Height) }
    if !inBounds(cm.PlayerSpawn, cm) { return fmt.Errorf("player_spawn %+v out of bounds", cm.PlayerSpawn) }
    if !inBounds(cm.NPCSpawn, cm) { return fmt.Errorf("npc_spawn %+v out of bounds", cm.NPCSpawn) }
    if cm.PlayerSpawn == cm.NPCSpawn { return fmt.Errorf("player_spawn and npc_spawn must differ") }
    return nil
}
```

The loader applies defaults *then* validates so omitted fields succeed.

---

### Task 2: `StartCombat` integration — placement + clamping + cover validation

**Files:**
- Modify: `internal/game/combat/engine.go`
- Modify: `internal/game/combat/engine_test.go`
- Create: `internal/game/combat/testdata/rapid/TestStartCombat_GridSize_Property/`

- [ ] **Step 1: Failing tests** (BMAP-3..7, BMAP-18, BMAP-19):

```go
func TestStartCombat_NoCombatMap_DefaultsTo20x20(t *testing.T) {
    cbt, _ := startCombatWithRoom(t, plainRoomNoMap, players: 1, npcs: 1)
    require.Equal(t, 20, cbt.GridWidth)
    require.Equal(t, 20, cbt.GridHeight)
    require.Equal(t, combat.Cell{X: 0,  Y: 10}, cellOf(cbt.Player(0)))
    require.Equal(t, combat.Cell{X: 19, Y: 10}, cellOf(cbt.NPC(0)))
}

func TestStartCombat_CustomMap_HonorsDimensionsAndSpawns(t *testing.T) {
    room := roomWithMap(12, 8, world.Cell{X: 1, Y: 4}, world.Cell{X: 10, Y: 4}, world.AxisY)
    cbt, _ := startCombatWithRoom(t, room, players: 1, npcs: 1)
    require.Equal(t, 12, cbt.GridWidth)
    require.Equal(t, 8,  cbt.GridHeight)
    require.Equal(t, combat.Cell{X: 1,  Y: 4}, cellOf(cbt.Player(0)))
    require.Equal(t, combat.Cell{X: 10, Y: 4}, cellOf(cbt.NPC(0)))
}

func TestStartCombat_NPCStackingClampsAtBoundary(t *testing.T) {
    room := roomWithMap(12, 8, world.Cell{X: 1, Y: 4}, world.Cell{X: 10, Y: 7}, world.AxisY)
    cbt, log := startCombatWithRoom(t, room, players: 1, npcs: 4) // y range 7..3
    require.Equal(t, combat.Cell{X: 10, Y: 7}, cellOf(cbt.NPC(0)))
    require.Equal(t, combat.Cell{X: 10, Y: 6}, cellOf(cbt.NPC(1)))
    require.Equal(t, combat.Cell{X: 10, Y: 5}, cellOf(cbt.NPC(2)))
    require.Equal(t, combat.Cell{X: 10, Y: 4}, cellOf(cbt.NPC(3)))
    require.NotContains(t, log.AllLines(), "warning") // all fit
}

func TestStartCombat_NPCStackingWrapsWhenColumnFull(t *testing.T) {
    // BMAP-Q3 — wrap to previous column at same starting y when column fills.
    room := roomWithMap(10, 5, world.Cell{X: 0, Y: 2}, world.Cell{X: 9, Y: 4}, world.AxisY) // only 5 vertical
    cbt, _ := startCombatWithRoom(t, room, players: 1, npcs: 7)
    require.Equal(t, combat.Cell{X: 9, Y: 4}, cellOf(cbt.NPC(0)))
    require.Equal(t, combat.Cell{X: 9, Y: 0}, cellOf(cbt.NPC(4)))
    require.Equal(t, combat.Cell{X: 8, Y: 4}, cellOf(cbt.NPC(5)), "wrap to column 8 at start y")
}

func TestStartCombat_OutOfBoundsCoverDropped(t *testing.T) {
    room := roomWithMap(10, 10, ..., withCover(world.Cell{X: 25, Y: 5}, world.Cell{X: 5, Y: 5}))
    cbt, log := startCombatWithRoom(t, room, players: 1, npcs: 1)
    require.Len(t, cbt.CoverObjects, 1) // only in-bounds survived
    require.Contains(t, log.AllLines(), "cover at (25,5) out of bounds")
}

func TestStartCombat_PlayerStackingPerpendicular(t *testing.T) {
    // BMAP-6: players stack along the opposite axis to npc_stack_axis.
    room := roomWithMap(10, 10, world.Cell{X: 0, Y: 5}, world.Cell{X: 9, Y: 5}, world.AxisY)
    cbt, _ := startCombatWithRoom(t, room, players: 3, npcs: 1)
    // Players stack along x (perpendicular to y-axis NPCs)
    require.Equal(t, combat.Cell{X: 0, Y: 5}, cellOf(cbt.Player(0)))
    require.Equal(t, combat.Cell{X: 1, Y: 5}, cellOf(cbt.Player(1)))
    require.Equal(t, combat.Cell{X: 2, Y: 5}, cellOf(cbt.Player(2)))
}
```

- [ ] **Step 2: Implement** in `engine.go:StartCombat`:

```go
cm := room.CombatMap
if cm == nil {
    cm = defaultCombatMap()
}
cbt.GridWidth, cbt.GridHeight = cm.Width, cm.Height

placePlayer(cbt, players, cm.PlayerSpawn, perpendicular(cm.NPCStackAxis))
placeNPCs(cbt, npcs, cm.NPCSpawn, cm.NPCStackAxis)

// Validate cover/hazards
for _, c := range room.CoverObjects {
    if !cellInBounds(c.X, c.Y, cm.Width, cm.Height) {
        log.Warn().Msgf("cover at (%d,%d) out of bounds", c.X, c.Y)
        continue
    }
    cbt.CoverObjects = append(cbt.CoverObjects, c)
}
```

`placeNPCs` walks along the axis from `NPCSpawn`, decrementing the axis coord each step and wrapping to the previous column / row when the axis hits its boundary (BMAP-5, BMAP-Q3).

- [ ] **Step 3: Property test**:

```go
func TestProperty_StartCombat_AllSpawnsInBoundsNoOverlap(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        w := rapid.IntRange(5, 30).Draw(t, "w")
        h := rapid.IntRange(5, 30).Draw(t, "h")
        ps := arbitraryCellIn(t, w, h)
        ns := differentCellFrom(t, ps, w, h)
        nPlayers := rapid.IntRange(1, 4).Draw(t, "p")
        nNPCs := rapid.IntRange(1, 8).Draw(t, "n")
        room := roomWithMap(w, h, ps, ns, world.AxisY)
        cbt, _ := startCombatWithRoom(t, room, players: nPlayers, npcs: nNPCs)
        cells := allCombatantCells(cbt)
        for _, c := range cells {
            require.True(t, c.X >= 0 && c.X < w && c.Y >= 0 && c.Y < h)
        }
        require.Equal(t, len(cells), uniqueCells(cells), "no overlapping spawns")
    })
}
```

---

### Task 3: Telnet renderer — runtime dimensions + clipping

**Files:**
- Modify: `internal/frontend/telnet/combat_grid.go`
- Modify: `internal/frontend/telnet/combat_grid_test.go`

- [ ] **Step 1: Checkpoint (BMAP-Q1).** Confirm with user the clipping strategy when grid > terminal width:
  - Option A: half-width characters (each cell = 1 char instead of 2).
  - Option B: horizontal scroll viewport.
  - Option C: snapshot view.

  Plan default = Option A (half-width). Reversible.

- [ ] **Step 2: Failing tests** (BMAP-8, BMAP-9, BMAP-10):

```go
func TestCombatGrid_UsesRuntimeDimensions(t *testing.T) {
    cbt := buildCombatWithSize(t, 12, 8)
    out := telnet.RenderCombatGrid(cbt, terminalWidth: 200)
    require.Equal(t, 8, lineCount(out, gridLines))
    require.Equal(t, 12, cellsPerLine(out))
}

func TestCombatGrid_ClipsAtNarrowTerminalWithHalfWidth(t *testing.T) {
    cbt := buildCombatWithSize(t, 30, 30)
    out := telnet.RenderCombatGrid(cbt, terminalWidth: 80)
    require.Contains(t, out, "[map clipped]") // BMAP-9 annotation
    require.LessOrEqual(t, maxLineLength(out), 80)
}

func TestCombatGrid_HalfWidthRenderingPreservesEachCell(t *testing.T) {
    cbt := buildCombatWithSize(t, 30, 30, withCombatantAt(t, 0, 0))
    out := telnet.RenderCombatGrid(cbt, terminalWidth: 80)
    require.Contains(t, out, "@") // first-cell combatant glyph still visible
}
```

- [ ] **Step 3: Implement**. Drop the constant references; consume `cbt.GridWidth/Height`. The half-width path emits one character per cell (instead of two) and prints a single `[map clipped]` annotation when narrowing was applied.

---

### Task 4: Web verification — auto-fit covers full size range

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/MapPanel.tsx` (only if hard-coded 20s exist)
- Modify: `cmd/webclient/ui/src/game/panels/MapPanel.test.tsx`

- [ ] **Step 1: Audit.** Grep `MapPanel.tsx` for any literal `20`. Spec says the file already adapts; confirm.

- [ ] **Step 2: Failing test** (BMAP-12, BMAP-13):

```ts
test("MapPanel auto-fits cell size for 5x5", () => {
  render(<MapPanel gridWidth={5} gridHeight={5} />);
  const cells = screen.getAllByTestId("grid-cell");
  expect(cells).toHaveLength(25);
  // Cell dim should be larger than at 30×30 for the same container width.
  const cellDim = parseInt(cells[0].style.width);
  expect(cellDim).toBeGreaterThan(40);
});

test("MapPanel auto-fits cell size for 30x30", () => {
  render(<MapPanel gridWidth={30} gridHeight={30} />);
  const cells = screen.getAllByTestId("grid-cell");
  expect(cells).toHaveLength(900);
});

test("AoE template placement bounds use server gridWidth/Height", () => {
  render(<AoEPreview gridWidth={10} gridHeight={10} template={lineTemplateAt(9, 5, "east", 30)} />);
  const cells = screen.getAllByTestId("aoe-cell");
  // Line goes east from (9,5); only cell (9,5) is in-bounds; cells (10..) are clipped.
  expect(cells).toHaveLength(1);
});
```

- [ ] **Step 3:** Confirm no hard-coded 20 remains. Tests pass.

---

### Task 5: Admin UI — `RoomCombatMapTab`

**Files:**
- Create: `cmd/webclient/ui/src/admin/RoomCombatMapTab.tsx`
- Create: `cmd/webclient/ui/src/admin/RoomCombatMapTab.test.tsx`

- [ ] **Step 1: Failing component tests** (BMAP-14, BMAP-15, BMAP-16):

```ts
test("RoomCombatMapTab edits width and height with [5,30] validation", () => {
  render(<RoomCombatMapTab room={blankRoom} />);
  fireEvent.change(screen.getByLabelText("Width"), { target: { value: "4" } });
  expect(screen.getByText("Width must be 5–30")).toBeVisible();
  fireEvent.change(screen.getByLabelText("Width"), { target: { value: "12" } });
  expect(screen.queryByText("Width must be 5–30")).toBeNull();
});

test("Spawn cell pickers select on the preview grid", () => {
  render(<RoomCombatMapTab room={blankRoom} />);
  fireEvent.click(screen.getByLabelText("Set player spawn"));
  fireEvent.click(screen.getByTestId("preview-cell-3-4"));
  expect(screen.getByText(/Player spawn: \(3, 4\)/)).toBeVisible();
});

test("Stack axis toggle changes between x and y", () => {
  render(<RoomCombatMapTab room={blankRoom} />);
  fireEvent.click(screen.getByLabelText("Stack axis"));
  expect(screen.getByLabelText("X")).toBeChecked();
});

test("Save dispatches UpdateRoomCombatMap", () => {
  const dispatch = jest.fn();
  render(<RoomCombatMapTab room={blankRoom} dispatch={dispatch} />);
  // edit some fields...
  fireEvent.click(screen.getByRole("button", { name: /save/i }));
  expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({
    type: "UpdateRoomCombatMap",
    payload: expect.objectContaining({ width: expect.any(Number), height: expect.any(Number) }),
  }));
});

test("Preview grid scales cells based on width/height to keep readable", () => {
  const { rerender } = render(<RoomCombatMapTab room={withMap(5, 5)} />);
  const small = parseInt(screen.getAllByTestId("preview-cell-0-0")[0].style.width);
  rerender(<RoomCombatMapTab room={withMap(30, 30)} />);
  const large = parseInt(screen.getAllByTestId("preview-cell-0-0")[0].style.width);
  expect(small).toBeGreaterThan(large);
});
```

- [ ] **Step 2: Implement** the tab:

```tsx
const RoomCombatMapTab = ({ room, dispatch }: Props) => {
  const [map, setMap] = useState(room.combatMap || defaultMap());
  const errors = validate(map);
  return (
    <Tab>
      <DimInput label="Width"  value={map.width}  onChange={...} error={errors.width} />
      <DimInput label="Height" value={map.height} onChange={...} error={errors.height} />
      <SpawnPicker label="Player spawn" cell={map.playerSpawn} grid={map} onPick={...} />
      <SpawnPicker label="NPC spawn"    cell={map.npcSpawn}    grid={map} onPick={...} />
      <AxisToggle value={map.npcStackAxis} onChange={...} />
      <Preview map={map} />
      <SaveButton disabled={hasErrors(errors)} onClick={() => dispatch({type:"UpdateRoomCombatMap", payload: map})} />
    </Tab>
  );
};
```

- [ ] **Step 3: Save path** — backend `UpdateRoomCombatMap` mutator already exists if rooms are DB-loaded; if YAML-loaded, the save writes the file. Verify which storage applies and wire accordingly.

---

### Task 6: Optional content linter

**Files:**
- Optional: `tools/world_lint/combat_map_check.go`

- [ ] **Step 1: Decide** whether to ship the lint check (BMAP-Q2). Recommendation: yes, warn-only on hot reload.

- [ ] **Step 2 (if shipping): Failing test**:

```go
func TestLint_DetectsCoverOutsideCombatMap(t *testing.T) {
    issues := worldlint.CheckCombatMap(roomWithMap(10, 10, ..., withCover(world.Cell{X: 25, Y: 5})))
    require.Contains(t, issues, "room <id>: cover at (25,5) outside 10×10 combat map")
}
```

- [ ] **Step 3: Implement** as a simple cross-check function callable from the existing world-load lint pass.

---

### Task 7: Architecture documentation update

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Battle Maps" section** documenting:
  - The `Room.CombatMap` schema and its defaults.
  - The `[5,30]` size range and the spawn-validation rules.
  - The placement algorithm with stacking + clamping + wrap.
  - The cover/hazard out-of-bounds handling.
  - The telnet clipping strategy (half-width per Q1).
  - The admin tab UX contract.
  - Open question resolutions (BMAP-Q1..Q5).

- [ ] **Step 2: Cross-link** spec, plan, `engine.go:StartCombat`, the admin tab, and the related ticket consumers (#250 / #251 / #248).

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- Telnet smoke test: declare a 12×8 combat map in a test room, enter combat, verify the rendered grid is 12×8; spawn a 30×30 combat map; confirm the half-width rendering with the `[map clipped]` annotation.
- Web smoke test: same scenarios in `MapPanel.tsx`; verify cells auto-fit; verify AoE placement bounds are correct on the smaller grids.
- Admin smoke test: open the new tab on a room without a combat map, set width=12 / height=8, set spawns, save, re-enter combat, verify the new map applies.

---

## Rollout / Open Questions Resolved at Plan Time

- **BMAP-Q1**: Telnet narrowing uses half-width characters with `[map clipped]` annotation. Confirmable at Task 3 checkpoint.
- **BMAP-Q2**: Linter check is warn-only on hot reload. Optional task.
- **BMAP-Q3**: NPC stacking wraps to the previous column at the same starting y when the column fills.
- **BMAP-Q4**: Per-encounter overrides deferred. Per-room only in v1.
- **BMAP-Q5**: Admin preview shows scaled cells matching typical web pixel size to discourage authoring unreadable 30×30 grids.

## Non-Goals Reaffirmed

Per spec §2.2:

- No per-cell terrain (#248).
- No cover-object editor in this admin tab.
- No background art / lighting overlays.
- No dynamic mid-combat resize.
- No procedural map sizing.
- No per-encounter override.

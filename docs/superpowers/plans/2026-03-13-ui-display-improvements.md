# UI Display Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Three UI improvements: active conditions in the player prompt, NPC conditions in the room view, and a side-by-side map+legend layout at wide terminals.

**Architecture:** Task 1 modifies `BuildPrompt` to accept conditions and threads condition state through the frontend event loop. Task 2 extends the `NpcInfo` proto, populates conditions in `buildRoomView`, and renders them in `renderNPCs`. Task 3 restructures the `RenderMap` assembly section to produce a side-by-side layout at width ≥ 100.

**Tech Stack:** Go, protobuf (`make proto`), pgregory.net/rapid (property-based testing)

---

## Chunk 1: Task 1 — Player Prompt Conditions

### Task 1: Player Prompt Shows Active Conditions

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/game_bridge_test.go`

**Context:**

`BuildPrompt` currently has this signature:
```go
func BuildPrompt(name, period, hour string, currentHP, maxHP int32) string
```
It returns: `[Name] [Period HH:00] [HP/MaxHPhp]> `

The `buildCurrentPrompt` closure (defined around line 191) wraps `BuildPrompt`:
```go
buildCurrentPrompt := func() string {
    tod := currentTime.Load().(*gamev1.TimeOfDayEvent)
    return BuildPrompt(char.Name, tod.Period, fmt.Sprintf("%02d:00", tod.Hour), currentHP.Load(), maxHP.Load())
}
```

This closure is passed as `buildPrompt func() string` to both `commandLoop` and `forwardServerEvents`. All 25+ `buildPrompt()` call sites call this closure — they do NOT need individual changes. Only the closure definition and `BuildPrompt` itself need updating.

`ConditionEvent` is handled in `forwardServerEvents` at line 639:
```go
case *gamev1.ServerEvent_ConditionEvent:
    ce := p.ConditionEvent
    if ce.ConditionId == "" { ... continue }
    text = RenderConditionEvent(ce)
```
`ce.GetApplied()` is `true` when applied, `false` when removed. `ce.GetConditionId()` is the ID. `ce.GetConditionName()` is the display name.

Both `commandLoop` and `forwardServerEvents` share `buildCurrentPrompt`, which runs in concurrent goroutines → use `sync.Mutex` to protect the conditions map.

- [ ] **Step 1: Write failing tests for new `BuildPrompt` signature**

Add to `internal/frontend/handlers/game_bridge_test.go`, inside the existing `package handlers_test` block, after existing tests:

```go
func TestBuildPrompt_NoConditions_FormatUnchanged(t *testing.T) {
    got := handlers.BuildPrompt("Thorald", "Morning", "09:00", 50, 60, nil)
    if !strings.HasSuffix(got, "> ") {
        t.Errorf("prompt must end with '> ', got %q", got)
    }
    // Strip ANSI codes before counting brackets to avoid false positives from escape sequences.
    stripped := telnet.StripANSI(got)
    // Should have exactly 3 bracket groups: [Name], [Period Hour], [HP/MaxHPhp]
    if strings.Count(stripped, "[") != 3 {
        t.Errorf("expected exactly 3 bracket groups (no conditions), got %q", stripped)
    }
}

func TestBuildPrompt_OneCondition(t *testing.T) {
    got := handlers.BuildPrompt("Thorald", "Morning", "09:00", 50, 60, []string{"Panicked"})
    if !strings.Contains(got, "[Panicked]") {
        t.Errorf("expected [Panicked] in prompt, got %q", got)
    }
    if !strings.HasSuffix(got, "> ") {
        t.Errorf("prompt must end with '> ', got %q", got)
    }
}

func TestBuildPrompt_MultipleConditions(t *testing.T) {
    got := handlers.BuildPrompt("Thorald", "Morning", "09:00", 50, 60, []string{"Panicked", "Grabbed"})
    if !strings.Contains(got, "[Panicked]") {
        t.Errorf("expected [Panicked] in prompt, got %q", got)
    }
    if !strings.Contains(got, "[Grabbed]") {
        t.Errorf("expected [Grabbed] in prompt, got %q", got)
    }
}

func TestProperty_BuildPrompt_ConditionsAllPresent(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        n := rapid.IntRange(0, 5).Draw(rt, "n")
        conds := make([]string, n)
        for i := range conds {
            conds[i] = rapid.StringMatching(`[A-Za-z]{1,15}`).Draw(rt, fmt.Sprintf("cond%d", i))
        }
        name := rapid.StringMatching(`[A-Za-z]{1,10}`).Draw(rt, "name")
        period := rapid.SampledFrom([]string{"Morning", "Night", "Dawn"}).Draw(rt, "period")
        maxHP := rapid.Int32Range(1, 100).Draw(rt, "maxHP")
        curHP := rapid.Int32Range(0, maxHP).Draw(rt, "curHP")

        got := handlers.BuildPrompt(name, period, "08:00", curHP, maxHP, conds)
        for _, c := range conds {
            if !strings.Contains(got, "["+c+"]") {
                rt.Errorf("condition %q not found in prompt %q", c, got)
            }
        }
        if !strings.HasSuffix(got, "> ") {
            rt.Errorf("prompt must end with '> ', got %q", got)
        }
    })
}
```

Update the `import` block in `game_bridge_test.go` to add `"fmt"` if not already present, and add `"github.com/cory-johannsen/mud/internal/frontend/telnet"` for `telnet.StripANSI`.

- [ ] **Step 2: Update all existing test calls to `BuildPrompt` to pass `nil` conditions**

Search `game_bridge_test.go` for all existing `handlers.BuildPrompt(...)` calls (there are several in `TestBuildPrompt_Format`, `TestBuildPrompt_HealthColors`, `TestBuildPrompt_AllPeriods`, and `TestProperty_BuildPrompt_AlwaysEndsWithPromptSuffix`). Add `nil` as the last argument to each. Example:
```go
// Before:
got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 45, 60)
// After:
got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 45, 60, nil)
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... 2>&1 | head -30
```

Expected: compile error — `BuildPrompt` has wrong number of arguments.

- [ ] **Step 4: Update `BuildPrompt` signature and implementation**

In `internal/frontend/handlers/game_bridge.go`, replace the `BuildPrompt` function:

```go
// BuildPrompt constructs the colored telnet prompt string.
//
// Precondition: maxHP > 0; name, period, and hour must be non-empty.
// Postcondition: Returns a non-empty string ending with "> ".
func BuildPrompt(name, period, hour string, currentHP, maxHP int32, conditions []string) string {
	// Name segment
	nameSeg := telnet.Colorf(telnet.BrightCyan, "[%s]", name)

	// Time segment — color by period
	var timeColor string
	switch period {
	case "Dawn":
		timeColor = telnet.Yellow
	case "Morning":
		timeColor = telnet.BrightYellow
	case "Afternoon":
		timeColor = telnet.White
	case "Dusk":
		timeColor = telnet.BrightRed
	case "Evening":
		timeColor = telnet.Magenta
	default: // Night, Midnight, Late Night
		timeColor = telnet.Blue
	}
	timeSeg := telnet.Colorf(timeColor, "[%s %s]", period, hour)

	// HP segment — color by percentage
	if maxHP <= 0 {
		maxHP = 1
	}
	pct := float64(currentHP) / float64(maxHP)
	var hpColor string
	switch {
	case pct >= 0.75:
		hpColor = telnet.BrightGreen
	case pct >= 0.40:
		hpColor = telnet.Yellow
	default:
		hpColor = telnet.Red
	}
	hpSeg := telnet.Colorf(hpColor, "[%d/%dhp]", currentHP, maxHP)

	// Condition segments — BrightMagenta, one per active condition
	var condSegs []string
	for _, c := range conditions {
		condSegs = append(condSegs, telnet.Colorf(telnet.BrightMagenta, "[%s]", c))
	}

	parts := []string{nameSeg, timeSeg, hpSeg}
	parts = append(parts, condSegs...)
	return strings.Join(parts, " ") + "> "
}
```

Add `"slices"` and `"maps"` to the import block if needed (they're used in the closure, see next step).

- [ ] **Step 5: Add `activeConditions` map + mutex and update `buildCurrentPrompt` closure**

In the `gameBridge` function (around line 180, after the `maxHP` initialization), add:

```go
// activeConditions tracks condition ID → display name for the prompt.
// Protected by condMu because buildCurrentPrompt and the event loop share it.
var condMu sync.Mutex
activeConditions := make(map[string]string)
```

Replace the `buildCurrentPrompt` closure (around line 191):

```go
buildCurrentPrompt := func() string {
    tod := currentTime.Load().(*gamev1.TimeOfDayEvent)
    condMu.Lock()
    sortedIDs := slices.Sorted(maps.Keys(activeConditions))
    names := make([]string, 0, len(sortedIDs))
    for _, id := range sortedIDs {
        names = append(names, activeConditions[id])
    }
    condMu.Unlock()
    return BuildPrompt(char.Name, tod.Period, fmt.Sprintf("%02d:00", tod.Hour), currentHP.Load(), maxHP.Load(), names)
}
```

Add to the import block in `game_bridge.go`:
```go
"maps"
"slices"
```

- [ ] **Step 6: Pass `condMu` and `activeConditions` to `forwardServerEvents` and update the `ConditionEvent` handler**

`forwardServerEvents` is a method with a fixed signature — it cannot access locals from `gameBridge` directly. Add two parameters at the end of its signature:

```go
// Before (line 536):
func (h *AuthHandler) forwardServerEvents(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, currentRoom *atomic.Value, currentTime *atomic.Value, currentHP *atomic.Int32, maxHP *atomic.Int32, lastRoomView *atomic.Value, buildPrompt func() string) {

// After:
func (h *AuthHandler) forwardServerEvents(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, currentRoom *atomic.Value, currentTime *atomic.Value, currentHP *atomic.Int32, maxHP *atomic.Int32, lastRoomView *atomic.Value, buildPrompt func() string, condMu *sync.Mutex, activeConditions map[string]string) {
```

Find the call site of `forwardServerEvents` in `gameBridge` (there is one call via `go h.forwardServerEvents(...)` or similar) and add `&condMu, activeConditions` as the last two arguments.

Then in `forwardServerEvents`, find the `ConditionEvent` case (around line 639) and update it:

```go
case *gamev1.ServerEvent_ConditionEvent:
    ce := p.ConditionEvent
    if ce.ConditionId == "" {
        if conn.IsSplitScreen() {
            _ = conn.WriteConsole(telnet.Colorize(telnet.Cyan, "No active conditions."))
            _ = conn.WritePromptSplit(buildPrompt())
        } else {
            _ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "No active conditions."))
            _ = conn.WritePrompt(buildPrompt())
        }
        continue
    }
    condMu.Lock()
    if ce.GetApplied() {
        activeConditions[ce.GetConditionId()] = ce.GetConditionName()
    } else {
        delete(activeConditions, ce.GetConditionId())
    }
    condMu.Unlock()
    text = RenderConditionEvent(ce)
```

- [ ] **Step 7: Run tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... -v -count=1 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 8: Run full suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

Expected: no failures.

- [ ] **Step 9: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/game_bridge_test.go
git commit -m "feat(frontend): show active conditions in player prompt"
```

---

## Chunk 2: Task 2 — NPC Conditions in Room View

### Task 2: NPC Conditions in Room View

**Files:**
- Modify: `api/proto/game/v1/game.proto` (add `conditions` field to `NpcInfo`)
- Run: `make proto`
- Modify: `internal/gameserver/world_handler.go` (populate `NpcInfo.Conditions`)
- Modify: `internal/frontend/handlers/text_renderer.go` (render conditions in `renderNPCs`)
- Modify: `internal/frontend/handlers/text_renderer_test.go` (tests)

**Context:**

Current `NpcInfo` proto (line 291 in `game.proto`):
```proto
message NpcInfo {
  string instance_id         = 1;
  string name                = 2;
  string health_description  = 3;
  string fighting_target     = 4;
}
```

`buildRoomView` in `world_handler.go` already calls `h.combatH.FightingTargetName(inst.ID)` — same pattern applies for conditions. `h.combatH` is a `*CombatHandler`. `inst.ID` is the NPC instance ID.

`GetCombatConditionSet(uid, targetID string) (*condition.ActiveSet, bool)` looks up the combat in the player's room (`uid` is the viewing player). Since `buildRoomView` is always called with the viewing player's UID, this is correct.

`ActiveSet.All()` returns `[]*ActiveCondition`. Each entry has `ac.Def.Name` (the display name string).

`renderNPCs` in `text_renderer.go` currently builds each NPC entry:
```go
entry := fmt.Sprintf("%s%s%s %s(%s)%s",
    telnet.Yellow, n.Name, telnet.Reset,
    healthColor, n.HealthDescription, telnet.Reset)
if n.FightingTarget != "" {
    entry += fmt.Sprintf(" %sfighting %s%s",
        telnet.BrightRed, n.FightingTarget, telnet.Reset)
}
```

- [ ] **Step 1: Write failing tests**

`text_renderer_test.go` is in `package handlers` (same package as `text_renderer.go`), so it can call `renderNPCs` directly — no exported wrapper needed.

Add to `internal/frontend/handlers/text_renderer_test.go`:

```go
func TestRenderNPCs_ConditionsShown(t *testing.T) {
    npcs := []*gamev1.NpcInfo{
        {
            Name:              "Goblin",
            HealthDescription: "lightly wounded",
            FightingTarget:    "Hero",
            Conditions:        []string{"prone", "grabbed"},
        },
    }
    lines := renderNPCs(npcs, 120)
    joined := strings.Join(lines, "\n")
    stripped := telnet.StripANSI(joined)
    if !strings.Contains(stripped, "prone") {
        t.Errorf("expected 'prone' in NPC display, got %q", stripped)
    }
    if !strings.Contains(stripped, "grabbed") {
        t.Errorf("expected 'grabbed' in NPC display, got %q", stripped)
    }
}

func TestRenderNPCs_NoConditions(t *testing.T) {
    npcs := []*gamev1.NpcInfo{
        {
            Name:              "Goblin",
            HealthDescription: "unharmed",
            Conditions:        nil,
        },
    }
    lines := renderNPCs(npcs, 120)
    joined := strings.Join(lines, "\n")
    stripped := telnet.StripANSI(joined)
    // With no conditions, the stripped output should contain "Goblin" and "unharmed" but no condition list.
    if strings.Contains(stripped, "prone") || strings.Contains(stripped, "grabbed") {
        t.Errorf("unexpected condition text in %q", stripped)
    }
}

func TestProperty_RenderNPCs_AllConditionsPresent(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        n := rapid.IntRange(0, 4).Draw(rt, "n")
        conds := make([]string, n)
        for i := range conds {
            conds[i] = rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, fmt.Sprintf("cond%d", i))
        }
        npc := &gamev1.NpcInfo{
            Name:              "Goblin",
            HealthDescription: "unharmed",
            Conditions:        conds,
        }
        lines := renderNPCs([]*gamev1.NpcInfo{npc}, 120)
        joined := strings.Join(lines, "\n")
        stripped := telnet.StripANSI(joined)
        for _, c := range conds {
            if !strings.Contains(stripped, c) {
                rt.Errorf("condition %q missing from NPC display %q", c, stripped)
            }
        }
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... 2>&1 | head -20
```

Expected: compile error — `NpcInfo` has no `Conditions` field.

- [ ] **Step 3: Add `conditions` field to `NpcInfo` proto**

In `api/proto/game/v1/game.proto`, find `NpcInfo` and add field 5:

```proto
message NpcInfo {
  string instance_id         = 1;
  string name                = 2;
  string health_description  = 3;
  string fighting_target     = 4;
  repeated string conditions = 5; // display names of active conditions (empty when not in combat or no conditions)
}
```

- [ ] **Step 4: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
make proto
```

Expected: `game.pb.go` regenerated with `Conditions []string` on `NpcInfo`.

- [ ] **Step 5: Populate `NpcInfo.Conditions` in `buildRoomView`**

In `internal/gameserver/world_handler.go`, update the NPC loop (around line 186):

```go
for _, inst := range instances {
    if !inst.IsDead() {
        fightingTarget := ""
        var condNames []string
        if h.combatH != nil {
            fightingTarget = h.combatH.FightingTargetName(inst.ID)
            // Precondition: uid is the viewing player's UID (same room).
            if activeSet, ok := h.combatH.GetCombatConditionSet(uid, inst.ID); ok {
                for _, ac := range activeSet.All() {
                    condNames = append(condNames, ac.Def.Name)
                }
            }
        }
        npcInfos = append(npcInfos, &gamev1.NpcInfo{
            InstanceId:        inst.ID,
            Name:              inst.Name(),
            HealthDescription: inst.HealthDescription(),
            FightingTarget:    fightingTarget,
            Conditions:        condNames,
        })
    }
}
```

Add import for `"github.com/cory-johannsen/mud/internal/game/condition"` if not already present (it may already be imported).

- [ ] **Step 6: Render conditions in `renderNPCs`**

In `internal/frontend/handlers/text_renderer.go`, update `renderNPCs` to append conditions after the fighting target:

```go
if n.FightingTarget != "" {
    entry += fmt.Sprintf(" %sfighting %s%s",
        telnet.BrightRed, n.FightingTarget, telnet.Reset)
}
if len(n.Conditions) > 0 {
    entry += fmt.Sprintf(" %s[%s]%s",
        telnet.Yellow, strings.Join(n.Conditions, ", "), telnet.Reset)
}
```

- [ ] **Step 7: Run tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... ./internal/gameserver/... -v -count=1 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 8: Run full suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

Expected: no failures.

- [ ] **Step 9: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go internal/gameserver/world_handler.go internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(gameserver): show NPC conditions in room view"
```

---

## Chunk 3: Task 3 — Map 2-Column Layout

### Task 3: Map Grid + Legend Side-by-Side

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` (restructure `RenderMap` assembly)
- Modify: `internal/frontend/handlers/text_renderer_test.go` (tests)

**Context:**

`RenderMap` in `text_renderer.go` currently:
1. Builds the grid into a `strings.Builder` (`sb`)
2. Appends `"\r\nLegend:\r\n"` and then legend entries in 4 columns

The grid-building section (lines 1010–1178) produces lines by writing directly to `sb`. The legend section (lines 1179–1226) reads `entries []legendEntry`.

The restructuring only touches the **assembly** — grid lines collected into `[]string`, legend entries assembled into `[]string`, then zipped for width ≥ 100.

The grid-building code uses `sb.WriteString(...)`. We need to collect grid lines into a `[]string` instead. The grid rows are written with `\r\n` terminators. The simplest approach: build grid into `sb` as before, then split by `\r\n` to get lines.

**Algorithm (width ≥ 100):**
- `halfWidth = width / 2` (integer division; odd widths give left half the smaller piece)
- Grid rendered at full width first, then lines left-padded/truncated to `halfWidth - 3`
- Legend entries: ` *NN. RoomName` (1 per line, no columns)
- Zip with ` │ ` separator; pad shorter side with spaces

- [ ] **Step 1: Write failing tests**

Add to `internal/frontend/handlers/text_renderer_test.go`:

```go
func TestRenderMap_WideLayout_HasSeparator(t *testing.T) {
    resp := &gamev1.MapResponse{
        Tiles: []*gamev1.MapTile{
            {RoomId: "r1", RoomName: "Start", X: 0, Y: 0, Current: true, Exits: []string{"east"}},
            {RoomId: "r2", RoomName: "East Room", X: 1, Y: 0, Current: false, Exits: []string{"west"}},
        },
    }
    out := handlers.RenderMap(resp, 120)
    if !strings.Contains(out, "│") {
        t.Errorf("expected '│' separator in wide map layout, got:\n%s", out)
    }
    if !strings.Contains(out, "Start") {
        t.Errorf("expected 'Start' in legend, got:\n%s", out)
    }
    if !strings.Contains(out, "East Room") {
        t.Errorf("expected 'East Room' in legend, got:\n%s", out)
    }
}

func TestRenderMap_NarrowLayout_NoSeparator(t *testing.T) {
    resp := &gamev1.MapResponse{
        Tiles: []*gamev1.MapTile{
            {RoomId: "r1", RoomName: "Start", X: 0, Y: 0, Current: true, Exits: []string{}},
        },
    }
    out := handlers.RenderMap(resp, 80)
    if strings.Contains(out, "│") {
        t.Errorf("unexpected '│' separator in narrow map layout (width=80), got:\n%s", out)
    }
    if !strings.Contains(out, "Legend:") {
        t.Errorf("expected 'Legend:' in stacked layout, got:\n%s", out)
    }
}

func TestProperty_RenderMap_AllLegendEntriesPresent(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        n := rapid.IntRange(1, 10).Draw(rt, "n")
        tiles := make([]*gamev1.MapTile, n)
        for i := range tiles {
            tiles[i] = &gamev1.MapTile{
                RoomId:   fmt.Sprintf("r%d", i),
                RoomName: fmt.Sprintf("Room%d", i),
                X:        int32(i),
                Y:        0,
                Current:  i == 0,
                Exits:    []string{},
            }
        }
        width := rapid.IntRange(100, 200).Draw(rt, "width")
        resp := &gamev1.MapResponse{Tiles: tiles}
        out := handlers.RenderMap(resp, width)
        for _, t2 := range tiles {
            if !strings.Contains(out, t2.RoomName) {
                rt.Errorf("legend entry %q missing from wide map output", t2.RoomName)
            }
        }
        if !strings.Contains(out, "│") {
            rt.Errorf("expected '│' separator at width=%d", width)
        }
    })
}

func TestProperty_RenderMap_NarrowUnchanged(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        n := rapid.IntRange(1, 5).Draw(rt, "n")
        tiles := make([]*gamev1.MapTile, n)
        for i := range tiles {
            tiles[i] = &gamev1.MapTile{
                RoomId:   fmt.Sprintf("r%d", i),
                RoomName: fmt.Sprintf("Room%d", i),
                X:        int32(i),
                Y:        0,
                Current:  i == 0,
            }
        }
        width := rapid.IntRange(1, 99).Draw(rt, "width")
        resp := &gamev1.MapResponse{Tiles: tiles}
        out := handlers.RenderMap(resp, width)
        if strings.Contains(out, "│") {
            rt.Errorf("unexpected '│' separator at narrow width=%d", width)
        }
        if !strings.Contains(out, "Legend:") {
            rt.Errorf("expected 'Legend:' in stacked output at width=%d", width)
        }
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... -run TestRenderMap -v 2>&1 | head -30
```

Expected: FAIL — no `│` in current output.

- [ ] **Step 3: Restructure `RenderMap` assembly**

In `internal/frontend/handlers/text_renderer.go`, after the grid-building section ends (just before the legend section, around line 1179), replace everything from the legend section to the end of `RenderMap`:

```go
	// ── Legend entries ────────────────────────────────────────────────────
	type legendEntry struct {
		num     int
		name    string
		current bool
	}
	var entries []legendEntry
	for _, y := range ys {
		for _, x := range xs {
			t := byCoord[[2]int32{x, y}]
			if t == nil {
				continue
			}
			entries = append(entries, legendEntry{num: len(entries) + 1, name: t.RoomName, current: t.Current})
		}
	}

	if width <= 0 {
		width = 80
	}

	// ── Two-column layout (width ≥ 100): grid left, legend right ──────────
	if width >= 100 {
		halfWidth := width / 2 // integer division; left gets the smaller half on odd widths

		// Split grid string into lines (strip trailing empty line from final \r\n).
		gridStr := sb.String()
		gridLines := strings.Split(strings.TrimRight(gridStr, "\r\n"), "\r\n")

		// Build legend lines (one entry per line, no multi-column layout).
		legendLines := make([]string, 0, len(entries))
		for _, e := range entries {
			marker := " "
			if e.current {
				marker = "*"
			}
			legendLines = append(legendLines, fmt.Sprintf("%s%2d. %s", marker, e.num, e.name))
		}

		// Zip grid and legend with │ separator.
		maxLen := len(gridLines)
		if len(legendLines) > maxLen {
			maxLen = len(legendLines)
		}
		var out strings.Builder
		for i := 0; i < maxLen; i++ {
			var gridPart, legendPart string
			if i < len(gridLines) {
				gridPart = gridLines[i]
			}
			if i < len(legendLines) {
				legendPart = legendLines[i]
			}
			// Pad or truncate grid part to halfWidth-3 visible chars.
			visLen := len(telnet.StripANSI(gridPart))
			targetW := halfWidth - 3
			if targetW < 0 {
				targetW = 0
			}
			if visLen < targetW {
				gridPart += strings.Repeat(" ", targetW-visLen)
			}
			out.WriteString(gridPart)
			if i < len(legendLines) {
				out.WriteString(" │ ")
				out.WriteString(legendPart)
			}
			out.WriteString("\r\n")
		}
		return out.String()
	}

	// ── Stacked layout (width < 100): current behavior ────────────────────
	const legendCols = 4
	colWidth := width / legendCols
	if colWidth < 22 {
		colWidth = 22
	}
	nameWidth := colWidth - 4
	sb.WriteString("\r\nLegend:\r\n")
	for i := 0; i < len(entries); i += legendCols {
		for col := 0; col < legendCols; col++ {
			idx := i + col
			if idx >= len(entries) {
				break
			}
			e := entries[idx]
			marker := " "
			if e.current {
				marker = "*"
			}
			cell := fmt.Sprintf("%s%2d.%-*s", marker, e.num, nameWidth, e.name)
			if len(cell) > colWidth {
				cell = cell[:colWidth]
			}
			sb.WriteString(cell)
		}
		sb.WriteString("\r\n")
	}
	return sb.String()
```

**Important:** The `type legendEntry struct { ... }` block must be moved out of the original position (lines 1181–1195) and placed in the new location above. Remove the old legend section entirely (lines 1179–1229). The replacement above is the complete new legend+assembly section.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/handlers/... -v -count=1 2>&1 | tail -30
```

Expected: all pass including new map tests.

- [ ] **Step 5: Run full suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```

Expected: no failures.

- [ ] **Step 6: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, mark the three features complete:
- `- [x] Player prompt should include mental state if applicable`
- `- [x] Room section should include NPC mental state if applicable`
- `- [x] map and legend should be presented in 2-column view.`

- [ ] **Step 7: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go docs/requirements/FEATURES.md
git commit -m "feat(frontend): map+legend side-by-side layout at wide terminals"
```

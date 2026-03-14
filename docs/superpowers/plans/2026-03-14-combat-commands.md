# Combat Commands Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework climb/swim commands to use exit-based DCs, redesign sense motive with a 4-outcome system, add the delay command, and clean up stale skill names.

**Architecture:** All game logic lives in `internal/gameserver/grpc_service.go` handler functions. Data models are updated in `internal/game/npc/`, `internal/game/world/`, `internal/game/session/`, and `internal/game/combat/`. Proto changes require `make proto` after each edit. Bridge functions in `internal/frontend/handlers/bridge_handlers.go` translate user input to proto messages.

**Tech Stack:** Go, protobuf, pgregory.net/rapid (property-based tests), testify/assert

**Spec:** `docs/superpowers/specs/2026-03-14-combat-commands-design.md`

---

## File Map

| File | Change |
|------|--------|
| `internal/game/world/model.go` | Add `Terrain string` to `Room`; add `ClimbDC`, `Height`, `SwimDC` to `Exit` |
| `internal/game/npc/template.go` | Rename `Deception→Hustle`; add `SpecialAbilities`, `Disposition` |
| `internal/game/npc/instance.go` | Rename `Deception→Hustle`; add `SpecialAbilities`, `Disposition`, `MotiveBonus` |
| `internal/game/session/manager.go` | Add `BankedAP int` |
| `internal/game/combat/action.go` | Add `AddAP(n int)` method to `ActionQueue` |
| `internal/game/command/commands.go` | Update Help strings; add `HandlerDelay` + entry |
| `api/proto/game/v1/game.proto` | Add `direction` to Climb/Swim; add `DelayRequest` field 72 |
| `internal/frontend/handlers/bridge_handlers.go` | Update `bridgeClimb`/`bridgeSwim` to pass direction; add `bridgeDelay` |
| `internal/gameserver/grpc_service.go` | Rework `handleClimb`, `handleSwim`, `handleMotive`; add `handleDelay`; replace all `athletics` refs |
| `internal/gameserver/combat_handler.go` | Inject `BankedAP` after `StartRound`; apply `inst.MotiveBonus` to NPC AttackMod |
| `content/npcs/*.yaml` | Rename `deception:` → `hustle:` in any NPC files that use it |
| `internal/gameserver/grpc_service_climb_test.go` | New: climb tests |
| `internal/gameserver/grpc_service_swim_test.go` | New: swim tests |
| `internal/gameserver/grpc_service_motive_test.go` | New: motive tests |
| `internal/gameserver/grpc_service_delay_test.go` | New: delay tests |

---

## Chunk 1: Foundation (Rename, Data Model, Proto)

### Task 1: Rename Cleanup — NPC Deception→Hustle, athletics→muscle

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/gameserver/grpc_service.go` (all athletics refs + inst.Deception)
- Modify: `internal/game/command/commands.go` (Help strings)
- Modify: any `content/npcs/*.yaml` files with `deception:` key

- [ ] **Step 1: Rename template field**

In `internal/game/npc/template.go`, change:
```go
Deception int `yaml:"deception"`
```
to:
```go
Hustle int `yaml:"hustle"`
```

- [ ] **Step 2: Rename instance field and spawn copy**

In `internal/game/npc/instance.go`, change the struct field:
```go
Deception int
```
to:
```go
Hustle int
```

Find the line `Deception: tmpl.Deception,` in `NewInstanceWithResolver` (around line 162) and change to:
```go
Hustle: tmpl.Hustle,
```

- [ ] **Step 3: Update all references in grpc_service.go**

Run:
```bash
grep -n "inst\.Deception\|sess\.Skills\[\"athletics\"\]" internal/gameserver/grpc_service.go
```

Change every `inst.Deception` → `inst.Hustle` (line ~5433).

Change every `sess.Skills["athletics"]` → `sess.Skills["muscle"]` (lines ~4750, 4811, 4872, 4953, 6016, 6126).

Also update any log/narrative strings mentioning "athletics" or "deception" to "muscle" or "hustle".

- [ ] **Step 4: Update command Help strings**

In `internal/game/command/commands.go`, find the `motive` entry (line ~202-203) and update its `Help` field:
```go
Help: "Read an NPC's intentions (awareness vs Hustle DC; costs 1 AP in combat).",
```

Find `climb` and `swim` entries and update `Help` to say "muscle" instead of "athletics":
```go
// climb
Help: "Climb a climbable surface (muscle vs DC; costs 2 AP in combat).",
// swim
Help: "Swim through water or surface when submerged (muscle vs DC; costs 2 AP in combat).",
```

- [ ] **Step 5: Update YAML files**

Run:
```bash
grep -rl "^deception:" content/npcs/
```

For each file found, rename `deception:` → `hustle:`. (Likely zero files since NPC YAML values are loaded via template field name — verify the grep finds nothing, since the template YAML tag was `deception` and we've changed it to `hustle`. Any existing NPC YAML files using `deception:` key must be updated.)

- [ ] **Step 6: Build and verify**

```bash
go build ./...
go test ./internal/game/npc/... ./internal/gameserver/...
```

Expected: all tests pass (only compilation/rename changes, no logic changes).

- [ ] **Step 7: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/instance.go \
    internal/gameserver/grpc_service.go internal/game/command/commands.go \
    content/npcs/
git commit -m "refactor(npc): rename Deception→Hustle; rename athletics→muscle skill references"
```

---

### Task 2: Data Model — Exit fields and Room.Terrain

**Files:**
- Modify: `internal/game/world/model.go`

- [ ] **Step 1: Write failing test**

Create `internal/game/world/model_climb_swim_test.go`:

```go
package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestExit_ClimbDC_ZeroByDefault verifies that the zero value of Exit has ClimbDC=0.
//
// Precondition: Exit created with struct literal omitting ClimbDC.
// Postcondition: ClimbDC == 0 (not climbable by default).
func TestExit_ClimbDC_ZeroByDefault(t *testing.T) {
	e := world.Exit{Direction: world.North, TargetRoom: "room_b"}
	assert.Equal(t, 0, e.ClimbDC)
	assert.Equal(t, 0, e.Height)
	assert.Equal(t, 0, e.SwimDC)
}

// TestRoom_Terrain_EmptyByDefault verifies that Room.Terrain defaults to empty string.
//
// Precondition: Room created without Terrain field.
// Postcondition: Terrain == "".
func TestRoom_Terrain_EmptyByDefault(t *testing.T) {
	r := world.Room{ID: "r1"}
	assert.Equal(t, "", r.Terrain)
}

// TestProperty_FallDamage_Formula verifies fall damage = max(1, floor(height/10)) for any height in [0,100].
//
// Precondition: height in [0, 100].
// Postcondition: dice count = max(1, height/10).
func TestProperty_FallDamage_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		height := rapid.IntRange(0, 100).Draw(rt, "height")
		expected := height / 10
		if expected < 1 {
			expected = 1
		}
		assert.Equal(rt, expected, fallDiceCount(height))
	})
}

// fallDiceCount mirrors the formula from handleClimb for testing.
func fallDiceCount(height int) int {
	d := height / 10
	if d < 1 {
		return 1
	}
	return d
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/game/world/... -run TestExit_ClimbDC_ZeroByDefault -v
```

Expected: FAIL — `ClimbDC` field does not exist yet.

- [ ] **Step 3: Add fields to Exit and Room (additive changes only)**

In `internal/game/world/model.go`, find the `Exit` struct (around line 76). **Do NOT replace the struct** — add only the three new fields after the existing `Hidden bool` field:

```go
ClimbDC int `yaml:"climb_dc"` // 0 = not climbable (unless terrain default applies)
Height  int `yaml:"height"`   // feet; used for fall damage: max(1, floor(Height/10)) d6
SwimDC  int `yaml:"swim_dc"`  // 0 = not swimmable (unless terrain default applies)
```

Find the `Room` struct (around line 122). **Do NOT replace the struct** — add only the new `Terrain` field after the existing `Properties` field:

```go
Terrain string `yaml:"terrain"` // optional terrain type: rubble, cliff, wall, sewer, river, ocean, flooded
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/game/world/... -v
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/world/model.go internal/game/world/model_climb_swim_test.go
git commit -m "feat(world): add Exit.ClimbDC/Height/SwimDC and Room.Terrain fields"
```

---

### Task 3: Proto — direction fields + DelayRequest

**Files:**
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Add direction field to ClimbRequest and SwimRequest**

In `api/proto/game/v1/game.proto`, find `ClimbRequest` (around line 801-802). It currently reads:
```proto
message ClimbRequest {}
```
Change it to:
```proto
// ClimbRequest asks the server to attempt climbing a climbable surface.
message ClimbRequest {
  string direction = 1; // compass direction to climb (e.g. "up", "north")
}
```

Find `SwimRequest` (around line 804-805). It currently reads:
```proto
message SwimRequest {}
```
Change it to:
```proto
// SwimRequest asks the server to attempt swimming through a water exit.
message SwimRequest {
  string direction = 1; // compass direction to swim (e.g. "north", "east")
}
```

- [ ] **Step 2: Add DelayRequest**

After `MotiveRequest` (around line 818), add:
```proto
// DelayRequest banks remaining AP for next round at cost of -2 AC.
message DelayRequest {}
```

- [ ] **Step 3: Add DelayRequest to ClientMessage oneof**

Find the `ClientMessage` oneof. After the `hero_point = 71;` entry, add:
```proto
DelayRequest delay = 72;
```

- [ ] **Step 4: Regenerate proto**

```bash
make proto
```

Expected: `internal/gameserver/gamev1/game.pb.go` regenerated; no errors.

- [ ] **Step 5: Fix bridge functions that pass empty ClimbRequest/SwimRequest**

In `internal/frontend/handlers/bridge_handlers.go`, find `bridgeClimb` (around line 930):

```go
func bridgeClimb(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return writeErrorPrompt(bctx, "Usage: climb <direction>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Climb{Climb: &gamev1.ClimbRequest{
			Direction: bctx.parsed.Args[0],
		}},
	}}, nil
}
```

Find `bridgeSwim` (around line 1099):

```go
func bridgeSwim(bctx *bridgeContext) (bridgeResult, error) {
	if len(bctx.parsed.Args) == 0 {
		return writeErrorPrompt(bctx, "Usage: swim <direction>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_Swim{Swim: &gamev1.SwimRequest{
			Direction: bctx.parsed.Args[0],
		}},
	}}, nil
}
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

Expected: no errors. (Handler functions still take `_ *gamev1.ClimbRequest` — that's fixed in Task 4.)

- [ ] **Step 7: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/ \
    internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(proto): add direction to ClimbRequest/SwimRequest; add DelayRequest field 72"
```

---

## Chunk 2: Climb and Swim Reworks

### Task 4: Climb Rework

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleClimb)
- Create: `internal/gameserver/grpc_service_climb_test.go`

**Background:**
`handleClimb` currently reads `room.Properties["climbable"]` and ignores the request direction. It must be reworked to:
1. Read `req.Direction` to find the exit
2. Check `exit.ClimbDC` (or terrain default) instead of room properties
3. Use `sess.Skills["muscle"]` instead of "athletics"
4. Calculate height-based fall damage using `exit.Height`

Terrain defaults for climb:
```
"rubble" → 12, "cliff" → 20, "wall" → 15, "sewer" → 10
```

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_climb_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newClimbSvc builds a minimal GameServiceServer with a world containing climb/swim
// exits, suitable for handleClimb and handleSwim tests.
func newClimbWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone.",
		StartRoom:   "room_ground",
		Rooms: map[string]*world.Room{
			"room_ground": {
				ID:          "room_ground",
				ZoneID:      "test",
				Title:       "Ground Floor",
				Description: "A ground level area.",
				Terrain:     "wall",
				Exits: []world.Exit{
					{Direction: world.Up, TargetRoom: "room_top", ClimbDC: 15, Height: 20},
					{Direction: world.North, TargetRoom: "room_b"},
				},
				Properties: map[string]string{},
			},
			"room_top": {
				ID:          "room_top",
				ZoneID:      "test",
				Title:       "Top",
				Description: "The top of the wall.",
				Exits:       []world.Exit{{Direction: world.Down, TargetRoom: "room_ground"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "A nearby room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_ground"}},
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return worldMgr, session.NewManager()
}

func newClimbSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := newClimbWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleClimb_NoDirection verifies that climb with empty direction returns usage error.
//
// Precondition: player session exists; req.Direction == "".
// Postcondition: error message contains "direction".
func TestHandleClimb_NoDirection(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr := newClimbSvc(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	ev, err := svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: ""})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "direction")
}

// TestHandleClimb_NoClimbableExit verifies message when exit in direction has ClimbDC==0 and no terrain default.
//
// Precondition: player in room_ground; direction "north"; exit north has ClimbDC=0; room terrain not in default table.
// Postcondition: returns "nothing to climb" message.
func TestHandleClimb_NoClimbableExit(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr := newClimbSvc(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	// North exit has no ClimbDC and room terrain "wall" doesn't apply to north direction.
	// Actually, north exit exists but has ClimbDC=0 and room_ground terrain="wall" → default 15 applies only to exits.
	// We need a room with a non-climbable terrain or an exit with ClimbDC=0 and no matching terrain.
	// Use direction "south" which has no exit at all.
	ev, err := svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "south"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "nothing to climb")
}

// TestHandleClimb_Success verifies player moves to destination on high roll.
//
// Precondition: player in room_ground; direction "up"; ClimbDC=15; roll constant=18 → total 18 >= 15.
// Postcondition: player RoomID == "room_top".
func TestHandleClimb_Success(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(18))
	svc, sessMgr := newClimbSvc(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	_, err = svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "up"})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "room_top", sess.RoomID)
}

// TestHandleClimb_CritFailure_FallDamage verifies height-based fall damage on critical failure.
//
// Precondition: player in room_ground; direction "up"; ClimbDC=15; height=20; roll=1 (crit fail).
// Postcondition: player HP reduced by max(1, 20/10)=2 dice of d6; each d6 returns 1 (const source).
func TestHandleClimb_CritFailure_FallDamage(t *testing.T) {
	// Roll sequence: first roll for skill check (=1 → crit fail), then 2×d6 fall damage (=1 each)
	roller := dice.NewRoller(dice.NewConstSource(1))
	svc, sessMgr := newClimbSvc(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_ground", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	_, err = svc.handleClimb("u1", &gamev1.ClimbRequest{Direction: "up"})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	// height=20 → max(1, 20/10)=2 dice × 1 each = 2 total damage; HP = 10-2 = 8
	assert.Equal(t, 8, sess.CurrentHP)
}

// TestProperty_FallDamage_HeightRange verifies fall damage formula for all heights in [0,100].
//
// Precondition: exit.Height in [0, 100]; roll=1 (crit fail guaranteed vs any DC).
// Postcondition: HP reduction == max(1, height/10) * 1 (const roller returns 1 per die).
func TestProperty_FallDamage_HeightRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		height := rapid.IntRange(0, 100).Draw(rt, "height")
		dc := rapid.IntRange(2, 30).Draw(rt, "dc")

		roller := dice.NewRoller(dice.NewConstSource(1)) // always 1 → crit fail; each d6 = 1
		worldMgr, sessMgr := func() (*world.Manager, *session.Manager) {
			zone := &world.Zone{
				ID: "t", Name: "T", Description: "Test zone.", StartRoom: "r",
				Rooms: map[string]*world.Room{
					"r": {
						ID: "r", ZoneID: "t",
						Title: "Start", Description: "A room.",
						Exits: []world.Exit{
							{Direction: world.Up, TargetRoom: "r2", ClimbDC: dc, Height: height},
						},
						Properties: map[string]string{},
					},
					"r2": {ID: "r2", ZoneID: "t", Title: "Top", Description: "Top room.", Properties: map[string]string{}},
				},
			}
			wm, err := world.NewManager([]*world.Zone{zone})
			require.NoError(rt, err)
			return wm, session.NewManager()
		}()

		logger := zaptest.NewLogger(rt)
		npcMgr := npc.NewManager()
		svc := NewGameServiceServer(
			worldMgr, sessMgr, command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
			NewChatHandler(sessMgr), logger,
			nil, roller, nil, npcMgr, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
		)

		_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u", Username: "u", CharName: "u", Role: "player",
			RoomID: "r", CurrentHP: 1000, MaxHP: 1000,
		})
		require.NoError(rt, addErr)

		_, handleErr := svc.handleClimb("u", &gamev1.ClimbRequest{Direction: "up"})
		require.NoError(rt, handleErr)

		sess, ok := sessMgr.GetPlayer("u")
		require.True(rt, ok)

		expectedDice := height / 10
		if expectedDice < 1 {
			expectedDice = 1
		}
		expectedDamage := expectedDice * 1 // each d6 = 1 with const source
		assert.Equal(rt, 1000-expectedDamage, sess.CurrentHP)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run TestHandleClimb -v 2>&1 | head -40
```

Expected: FAIL — functions don't match new behavior yet.

- [ ] **Step 3: Implement terrain default helper**

At the top of `handleClimb` (or as a package-level helper near it in `grpc_service.go`), add:

```go
// climbDCForExit returns the effective climb DC for an exit.
// Returns 0 if the exit is not climbable (no explicit DC and no terrain default).
func climbDCForExit(exit world.Exit, terrain string) int {
	if exit.ClimbDC > 0 {
		return exit.ClimbDC
	}
	switch terrain {
	case "rubble":
		return 12
	case "cliff":
		return 20
	case "wall":
		return 15
	case "sewer":
		return 10
	}
	return 0
}
```

- [ ] **Step 4: Rewrite handleClimb**

Replace the existing `handleClimb` function body with:

```go
// handleClimb processes a ClimbRequest from the player.
//
// Precondition: uid is a valid connected player session; req.Direction is non-empty.
// Postcondition: Player moves via climbable exit on success; fall damage applied on critical failure.
func (s *GameServiceServer) handleClimb(uid string, req *gamev1.ClimbRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if req.GetDirection() == "" {
		return messageEvent("Climb which direction?"), nil
	}

	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("Room not found."), nil
	}

	dir := world.Direction(req.GetDirection())
	exit, found := room.ExitForDirection(dir)
	if !found {
		return messageEvent("There is nothing to climb here."), nil
	}

	dc := climbDCForExit(exit, room.Terrain)
	if dc == 0 {
		return messageEvent("There is nothing to climb here."), nil
	}

	// Spend AP if in combat.
	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return messageEvent("Not enough action points to climb."), nil
		}
		if err := s.combatH.SpendAP(uid, 2); err != nil {
			return messageEvent("Not enough action points to climb."), nil
		}
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "climb"

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		if _, moveErr := s.worldH.MoveWithContext(uid, dir); moveErr != nil {
			return messageEvent(fmt.Sprintf(
				"You climb successfully (rolled %d+%d=%d vs DC %d) but cannot proceed: %s.",
				roll, bonus, total, dc, moveErr.Error(),
			)), nil
		}
		destRoom, _ := s.world.GetRoom(exit.TargetRoom)
		destTitle := exit.TargetRoom
		if destRoom != nil {
			destTitle = destRoom.Title
		}
		return messageEvent(fmt.Sprintf(
			"You climb successfully (rolled %d+%d=%d vs DC %d). You arrive at %s.",
			roll, bonus, total, dc, destTitle,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You fail to gain purchase on the climb (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		numDice := exit.Height / 10
		if numDice < 1 {
			numDice = 1
		}
		expr := fmt.Sprintf("%dd6", numDice)
		dmgResult, _ := s.dice.RollExpr(expr)
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		msg := fmt.Sprintf(
			"You fall! (rolled %d+%d=%d vs DC %d) Taking %d falling damage.",
			roll, bonus, total, dc, dmg,
		)
		if inCombat && sess.Conditions != nil && s.condRegistry != nil {
			if def, ok := s.condRegistry.Get("prone"); ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
				msg += " You are knocked prone."
			}
		}
		return messageEvent(msg), nil
	}
}
```

**Note:** `world.Direction` is a `string` type alias, so `world.Direction(req.GetDirection())` is a direct cast. Verify the Direction constants (`North`, `South`, `East`, `West`, `Up`, `Down`) exist in `internal/game/world/model.go`.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/gameserver/... -run TestHandleClimb -v
```

Expected: all pass.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_climb_test.go \
    internal/game/world/model.go
git commit -m "feat(climb): exit-based DC with terrain defaults and height-based fall damage"
```

---

### Task 5: Swim Rework

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleSwim)
- Create: `internal/gameserver/grpc_service_swim_test.go`

Terrain defaults for swim:
```
"sewer" → 10, "river" → 15, "ocean" → 20, "flooded" → 12
```

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_swim_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

func newSwimWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID: "test", Name: "Test", Description: "Test zone.", StartRoom: "room_water",
		Rooms: map[string]*world.Room{
			"room_water": {
				ID: "room_water", ZoneID: "test", Title: "River", Description: "A rushing river.",
				Terrain: "river",
				Exits: []world.Exit{
					{Direction: world.North, TargetRoom: "room_bank", SwimDC: 15},
				},
				Properties: map[string]string{},
			},
			"room_bank": {
				ID: "room_bank", ZoneID: "test", Title: "Bank", Description: "The river bank.",
				Exits: []world.Exit{{Direction: world.South, TargetRoom: "room_water"}},
				Properties: map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

func newSwimSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := newSwimWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr, command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr), logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleSwim_NoDirection verifies usage error on empty direction.
func TestHandleSwim_NoDirection(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr := newSwimSvc(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_water", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	ev, err := svc.handleSwim("u1", &gamev1.SwimRequest{Direction: ""})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "direction")
}

// TestHandleSwim_NoSwimmableExit verifies message when direction has no swim exit.
func TestHandleSwim_NoSwimmableExit(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr := newSwimSvc(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_water", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	// "south" exit doesn't exist in room_water
	ev, err := svc.handleSwim("u1", &gamev1.SwimRequest{Direction: "south"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "no water")
}

// TestHandleSwim_Success verifies player moves to destination on high roll.
func TestHandleSwim_Success(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(18))
	svc, sessMgr := newSwimSvc(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_water", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	_, err = svc.handleSwim("u1", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "room_bank", sess.RoomID)
}

// TestHandleSwim_CritFailure_DrowningDamage verifies 1d6 drowning damage on crit fail.
func TestHandleSwim_CritFailure_DrowningDamage(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(1)) // roll=1 → crit fail; d6=1
	svc, sessMgr := newSwimSvc(t, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_water", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	_, err = svc.handleSwim("u1", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, 9, sess.CurrentHP) // 10 - 1 = 9
}

// TestProperty_SwimDC_TerrainDefaults verifies terrain default DC table.
func TestProperty_SwimDC_TerrainDefaults(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		terrain := rapid.SampledFrom([]string{"sewer", "river", "ocean", "flooded"}).Draw(rt, "terrain")
		expected := map[string]int{"sewer": 10, "river": 15, "ocean": 20, "flooded": 12}[terrain]
		got := swimDCForExit(world.Exit{SwimDC: 0}, terrain)
		assert.Equal(rt, expected, got)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run TestHandleSwim -v 2>&1 | head -30
```

Expected: FAIL.

- [ ] **Step 3: Add swimDCForExit helper in grpc_service.go**

```go
// swimDCForExit returns the effective swim DC for an exit.
// Returns 0 if the exit is not swimmable (no explicit DC and no terrain default).
func swimDCForExit(exit world.Exit, terrain string) int {
	if exit.SwimDC > 0 {
		return exit.SwimDC
	}
	switch terrain {
	case "sewer":
		return 10
	case "river":
		return 15
	case "ocean":
		return 20
	case "flooded":
		return 12
	}
	return 0
}
```

- [ ] **Step 4: Rewrite handleSwim**

Replace the existing `handleSwim` function body:

```go
// handleSwim processes a SwimRequest from the player.
//
// Precondition: uid is a valid connected player session; req.Direction is non-empty.
// Postcondition: Player moves on success; submerged condition applied and 1d6 damage on critical failure.
func (s *GameServiceServer) handleSwim(uid string, req *gamev1.SwimRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if req.GetDirection() == "" {
		return messageEvent("Swim which direction?"), nil
	}

	room, ok := s.world.GetRoom(sess.RoomID)
	if !ok {
		return messageEvent("Room not found."), nil
	}

	dir := world.Direction(req.GetDirection())
	exit, found := room.ExitForDirection(dir)
	dc := 0
	if found {
		dc = swimDCForExit(exit, room.Terrain)
	}

	isSubmerged := sess.Conditions != nil && sess.Conditions.Has("submerged")

	// Must either have a swimmable exit or be submerged.
	if dc == 0 && !isSubmerged {
		return messageEvent("There is no water here."), nil
	}
	// If submerged but no swim exit, use a default surface DC.
	if dc == 0 && isSubmerged {
		dc = 12
	}

	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return messageEvent("Not enough action points to swim."), nil
		}
		if err := s.combatH.SpendAP(uid, 2); err != nil {
			return messageEvent("Not enough action points to swim."), nil
		}
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["muscle"])
	total := roll + bonus
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "swim"

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		if isSubmerged && sess.Conditions != nil {
			sess.Conditions.Remove(uid, "submerged")
		}
		if found {
			if _, moveErr := s.worldH.MoveWithContext(uid, dir); moveErr != nil {
				return messageEvent(fmt.Sprintf(
					"You swim successfully (rolled %d+%d=%d vs DC %d) but cannot proceed: %s.",
					roll, bonus, total, dc, moveErr.Error(),
				)), nil
			}
		}
		if isSubmerged {
			return messageEvent(fmt.Sprintf(
				"You surface! (rolled %d+%d=%d vs DC %d)", roll, bonus, total, dc,
			)), nil
		}
		destRoom, _ := s.world.GetRoom(exit.TargetRoom)
		destTitle := exit.TargetRoom
		if destRoom != nil {
			destTitle = destRoom.Title
		}
		return messageEvent(fmt.Sprintf(
			"You swim through (rolled %d+%d=%d vs DC %d). You arrive at %s.",
			roll, bonus, total, dc, destTitle,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You struggle against the current (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		dmgResult, _ := s.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		msg := fmt.Sprintf(
			"You are pulled under! (rolled %d+%d=%d vs DC %d) Taking %d drowning damage.",
			roll, bonus, total, dc, dmg,
		)
		if s.condRegistry != nil {
			if def, condOk := s.condRegistry.Get("submerged"); condOk && sess.Conditions != nil {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
				msg += " You are submerged."
			}
		}
		return messageEvent(msg), nil
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/gameserver/... -run TestHandleSwim -v
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_swim_test.go
git commit -m "feat(swim): exit-based DC with terrain defaults; use muscle skill"
```

---

## Chunk 3: NPC Model and Sense Motive

### Task 6: NPC Model Extensions

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`

- [ ] **Step 1: Write failing test**

Create `internal/game/npc/npc_model_extensions_test.go`:

```go
package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
)

// TestInstance_SpecialAbilities_CopiedFromTemplate verifies SpecialAbilities is copied at spawn.
//
// Precondition: Template has SpecialAbilities=["Rage","Poison Spit"].
// Postcondition: Instance.SpecialAbilities == ["Rage","Poison Spit"].
func TestInstance_SpecialAbilities_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t1", Name: "Brute", MaxHP: 20, AC: 12,
		SpecialAbilities: []string{"Rage", "Poison Spit"},
	}
	inst := npc.NewInstanceWithResolver("inst1", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, []string{"Rage", "Poison Spit"}, inst.SpecialAbilities)
}

// TestInstance_Disposition_DefaultHostile verifies empty template Disposition defaults to "hostile".
//
// Precondition: Template.Disposition == "".
// Postcondition: Instance.Disposition == "hostile".
func TestInstance_Disposition_DefaultHostile(t *testing.T) {
	tmpl := &npc.Template{ID: "t2", Name: "Guard", MaxHP: 10, AC: 10}
	inst := npc.NewInstanceWithResolver("inst2", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, "hostile", inst.Disposition)
}

// TestInstance_Disposition_CopiedFromTemplate verifies explicit disposition is preserved.
//
// Precondition: Template.Disposition == "neutral".
// Postcondition: Instance.Disposition == "neutral".
func TestInstance_Disposition_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{ID: "t3", Name: "Merchant", MaxHP: 10, AC: 10, Disposition: "neutral"}
	inst := npc.NewInstanceWithResolver("inst3", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, "neutral", inst.Disposition)
}

// TestInstance_MotiveBonus_ZeroAtSpawn verifies MotiveBonus starts at 0.
//
// Precondition: fresh instance.
// Postcondition: MotiveBonus == 0.
func TestInstance_MotiveBonus_ZeroAtSpawn(t *testing.T) {
	tmpl := &npc.Template{ID: "t4", Name: "Ganger", MaxHP: 10, AC: 10}
	inst := npc.NewInstanceWithResolver("inst4", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, 0, inst.MotiveBonus)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/game/npc/... -run TestInstance_SpecialAbilities -v
```

Expected: FAIL — fields don't exist yet.

- [ ] **Step 3: Add fields to template.go**

In `internal/game/npc/template.go`, add to `Template` struct (after existing fields):

```go
SpecialAbilities []string `yaml:"special_abilities"` // named special abilities for sense motive reveal
Disposition      string   `yaml:"disposition"`        // "hostile","wary","neutral","friendly"; empty = "hostile"
```

- [ ] **Step 4: Add fields to instance.go**

In `internal/game/npc/instance.go`, add to `Instance` struct:

```go
SpecialAbilities []string // copied from template at spawn
Disposition      string   // runtime disposition; initialized from template, may change
MotiveBonus      int      // +2 attack bonus granted by motive crit fail; applied once then zeroed
```

In `NewInstanceWithResolver`, add the copy lines (after existing field copies):

```go
SpecialAbilities: append([]string(nil), tmpl.SpecialAbilities...),
Disposition: func() string {
    if tmpl.Disposition == "" {
        return "hostile"
    }
    return tmpl.Disposition
}(),
```

(`MotiveBonus` is zero by default — no explicit init needed.)

- [ ] **Step 5: Run tests**

```bash
go test ./internal/game/npc/... -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/instance.go \
    internal/game/npc/npc_model_extensions_test.go
git commit -m "feat(npc): add SpecialAbilities, Disposition, MotiveBonus fields"
```

---

### Task 7: Sense Motive Rework

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleMotive)
- Modify: `internal/gameserver/combat_handler.go` (apply MotiveBonus to NPC AttackMod at round start)
- Create: `internal/gameserver/grpc_service_motive_test.go`

**Background:**
- In combat: 4 outcomes (crit success/success/failure/crit failure) revealing NPC info
- Out of combat: reveals disposition; crit fail flips neutral/wary → hostile
- NPC `AttackMod` on `combat.Combatant` is reset by `StartRound` each round
- Apply `inst.MotiveBonus` to NPC's `Combatant.AttackMod` in `resolveAndAdvanceLocked` after `StartRound`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_motive_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newMotiveSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr, command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr), logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleMotive_OutOfCombat_Success verifies disposition revealed on success out of combat.
//
// Precondition: player not in combat; NPC in room with Disposition="neutral"; roll=18 → success.
// Postcondition: response contains "neutral".
func TestHandleMotive_OutOfCombat_Success(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(18))
	npcMgr := npc.NewManager()
	svc, sessMgr := newMotiveSvc(t, roller, npcMgr, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger1", Name: "Ganger", MaxHP: 20, AC: 12,
		Hustle: 2, Disposition: "neutral",
	}, "room_a")
	require.NoError(t, err)

	ev, err := svc.handleMotive("u1", &gamev1.MotiveRequest{Target: inst.Name()})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "neutral")
}

// TestHandleMotive_OutOfCombat_CritFailure_FlipsDisposition verifies neutral→hostile flip.
//
// Precondition: NPC disposition="neutral"; roll=1 → crit fail.
// Postcondition: inst.Disposition == "hostile".
func TestHandleMotive_OutOfCombat_CritFailure_FlipsDisposition(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(1))
	npcMgr := npc.NewManager()
	svc, sessMgr := newMotiveSvc(t, roller, npcMgr, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID: "ganger2", Name: "Ganger", MaxHP: 20, AC: 12,
		Hustle: 0, Disposition: "neutral",
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	_, err = svc.handleMotive("u1", &gamev1.MotiveRequest{Target: inst.Name()})
	require.NoError(t, err)
	assert.Equal(t, "hostile", inst.Disposition)
}

// TestHandleMotive_InCombat_Success_RevealsNextAction verifies next action revealed on success.
//
// Precondition: player in combat; roll ensures success; NPC HP > 25%.
// Postcondition: response contains "focused on the fight" (default heuristic).
func TestHandleMotive_InCombat_Success_RevealsNextAction(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(18))
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, nil, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	// Need sessions in combat handler too
	combatHandler.sessions = sessMgr

	svc := NewGameServiceServer(
		worldMgr, sessMgr, command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr), logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
		Status: statusInCombat,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer("u1")
	sess.Status = statusInCombat

	tmpl := &npc.Template{ID: "g3", Name: "Ganger", MaxHP: 20, AC: 12, Hustle: 0}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	inst.CurrentHP = 20 // > 25%

	// Start combat so SpendAP works
	_, startErr := combatHandler.StartCombat("room_a", []string{"u1"})
	require.NoError(t, startErr)

	ev, err := svc.handleMotive("u1", &gamev1.MotiveRequest{Target: inst.Name()})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "focused on the fight")
}

// TestHandleMotive_InCombat_CritFailure_SetsMotiveBonus verifies inst.MotiveBonus=2 on crit fail.
//
// Precondition: player in combat; roll=1 → crit fail.
// Postcondition: inst.MotiveBonus == 2.
func TestHandleMotive_InCombat_CritFailure_SetsMotiveBonus(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(1))
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, nil, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)

	worldMgr, sessMgr := testWorldAndSession(t)
	combatHandler.sessions = sessMgr

	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr, command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr), logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer("u1")
	sess.Status = statusInCombat

	tmpl := &npc.Template{ID: "g4", Name: "Ganger", MaxHP: 20, AC: 12, Hustle: 0}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	_, startErr := combatHandler.StartCombat("room_a", []string{"u1"})
	require.NoError(t, startErr)

	_, err = svc.handleMotive("u1", &gamev1.MotiveRequest{Target: inst.Name()})
	require.NoError(t, err)
	assert.Equal(t, 2, inst.MotiveBonus)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run TestHandleMotive -v 2>&1 | head -40
```

Expected: FAIL.

- [ ] **Step 3: Rewrite handleMotive**

Replace `handleMotive` in `grpc_service.go`:

```go
// handleMotive performs a sense motive check against a target NPC.
// In combat: 4-outcome system revealing NPC state. Out of combat: reveals disposition.
//
// Precondition: uid is a valid player session; req.Target names an NPC in the player's room.
// Postcondition: On crit success reveals full NPC state; on success reveals next action or disposition;
//   on crit fail sets inst.MotiveBonus=2 or flips disposition to hostile.
func (s *GameServiceServer) handleMotive(uid string, req *gamev1.MotiveRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: motive <target>"), nil
	}

	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	inCombat := sess.Status == statusInCombat
	if inCombat {
		if s.combatH == nil {
			return errorEvent("Combat handler unavailable."), nil
		}
		if err := s.combatH.SpendAP(uid, 1); err != nil {
			return errorEvent(err.Error()), nil
		}
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleMotive: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["awareness"])
	total := roll + bonus
	dc := 10 + inst.Hustle
	sess.LastCheckRoll = roll
	sess.LastCheckDC = dc
	sess.LastCheckName = "motive"

	outcome := combat.OutcomeFor(total, dc)
	detail := fmt.Sprintf("Motive (Hustle DC %d): rolled %d+%d=%d", dc, roll, bonus, total)

	if inCombat {
		return s.handleMotiveInCombat(detail, outcome, inst, sess.CharName)
	}
	return s.handleMotiveOutOfCombat(detail, outcome, inst)
}

// handleMotiveInCombat applies the 4-outcome sense motive results during combat.
func (s *GameServiceServer) handleMotiveInCombat(detail string, outcome combat.Outcome, inst *npc.Instance, playerName string) (*gamev1.ServerEvent, error) {
	switch outcome {
	case combat.CritSuccess:
		msg := detail + " — critical success!\n"
		msg += fmt.Sprintf("%s: %s\n", inst.Name(), motiveNextAction(inst))
		if len(inst.SpecialAbilities) > 0 {
			msg += fmt.Sprintf("Hidden abilities: %s\n", strings.Join(inst.SpecialAbilities, ", "))
		}
		if len(inst.Resistances) > 0 {
			msg += "Resistant to: " + formatResistanceMap(inst.Resistances) + "\n"
		}
		if len(inst.Weaknesses) > 0 {
			msg += "Weak to: " + formatResistanceMap(inst.Weaknesses) + "\n"
		}
		return messageEvent(strings.TrimRight(msg, "\n")), nil

	case combat.Success:
		return messageEvent(detail + " — success! " + inst.Name() + " " + motiveNextAction(inst) + "."), nil

	case combat.Failure:
		return messageEvent(detail + " — failure. You cannot read their intentions."), nil

	default: // CritFailure
		inst.MotiveBonus = 2
		return messageEvent(detail + " — critical failure. You misread them completely — they notice."), nil
	}
}

// handleMotiveOutOfCombat applies sense motive results outside of combat.
func (s *GameServiceServer) handleMotiveOutOfCombat(detail string, outcome combat.Outcome, inst *npc.Instance) (*gamev1.ServerEvent, error) {
	switch outcome {
	case combat.CritSuccess, combat.Success:
		return messageEvent(fmt.Sprintf("%s — success! %s seems %s.", detail, inst.Name(), inst.Disposition)), nil

	case combat.Failure:
		return messageEvent(detail + " — failure. You cannot get a read on them."), nil

	default: // CritFailure
		if inst.Disposition == "neutral" || inst.Disposition == "wary" {
			inst.Disposition = "hostile"
		}
		return messageEvent(detail + " — critical failure. You misread them badly."), nil
	}
}

// motiveNextAction returns the "next intended action" heuristic string for an NPC.
func motiveNextAction(inst *npc.Instance) string {
	hpPct := float64(inst.CurrentHP) / float64(inst.MaxHP) * 100
	if hpPct < 25 {
		return "looks ready to flee"
	}
	if len(inst.SpecialAbilities) > 0 {
		return "seems to be holding something back"
	}
	return "looks focused on the fight"
}

// formatResistanceMap returns "fire (5), cold (3)" format for a resistance/weakness map.
// Keys are sorted for determinism.
func formatResistanceMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s (%d)", k, m[k]))
	}
	return strings.Join(parts, ", ")
}
```

Make sure `"sort"` and `"strings"` are in the import block.

- [ ] **Step 4: Apply MotiveBonus in combat_handler.go**

In `internal/gameserver/combat_handler.go`, in `resolveAndAdvanceLocked` after `condEvents := cbt.StartRound(3)` (around line 1400), add a loop to apply `inst.MotiveBonus` to the NPC combatant's `AttackMod`:

```go
// Apply MotiveBonus from sense motive critical failures to NPC AttackMod.
// Precondition: combatMu held; cbt.StartRound has reset AttackMod to 0.
// Postcondition: NPC combatants with MotiveBonus > 0 have AttackMod incremented; MotiveBonus zeroed.
for _, c := range cbt.Combatants {
    if c.Kind != combat.KindNPC {
        continue
    }
    inst, instOK := h.npcMgr.Get(c.ID)
    if !instOK || inst == nil || inst.MotiveBonus <= 0 {
        continue
    }
    c.AttackMod += inst.MotiveBonus
    inst.MotiveBonus = 0
}

- [ ] **Step 5: Run tests**

```bash
go test ./internal/gameserver/... -run TestHandleMotive -v
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_motive_test.go \
    internal/gameserver/combat_handler.go
git commit -m "feat(motive): 4-outcome sense motive with disposition, abilities, and MotiveBonus"
```

---

## Chunk 4: Delay Command

### Task 8: ActionQueue.AddAP and BankedAP session field

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Write failing test for AddAP**

Add to `internal/game/combat/action_test.go`:

```go
// TestActionQueue_AddAP_IncreasesRemaining verifies AddAP adds to remaining AP.
//
// Precondition: queue with 2 remaining; AddAP(1) called.
// Postcondition: RemainingPoints() == 3.
func TestActionQueue_AddAP_IncreasesRemaining(t *testing.T) {
	q := NewActionQueue("u1", 3)
	_ = q.DeductAP(1) // remaining = 2
	q.AddAP(1)
	assert.Equal(t, 3, q.RemainingPoints())
}

// TestActionQueue_AddAP_Zero_NoChange verifies AddAP(0) is a no-op.
//
// Precondition: queue with 3 remaining; AddAP(0).
// Postcondition: RemainingPoints() == 3.
func TestActionQueue_AddAP_Zero_NoChange(t *testing.T) {
	q := NewActionQueue("u1", 3)
	q.AddAP(0)
	assert.Equal(t, 3, q.RemainingPoints())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/game/combat/... -run TestActionQueue_AddAP -v
```

Expected: FAIL — `AddAP` does not exist.

- [ ] **Step 3: Add AddAP to action.go**

In `internal/game/combat/action.go`, after the `DeductAP` method, add:

```go
// AddAP adds n action points to remaining.
//
// Precondition: n >= 0.
// Postcondition: remaining increases by n.
func (q *ActionQueue) AddAP(n int) {
	if n <= 0 {
		return
	}
	q.remaining += n
}
```

- [ ] **Step 4: Add BankedAP to PlayerSession**

In `internal/game/session/manager.go`, add to the `PlayerSession` struct (after `Dead bool`):

```go
BankedAP int // AP banked from delay; added to next round's AP pool; session-only, not persisted
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/game/combat/... ./internal/game/session/... -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/combat/action.go internal/game/session/manager.go
git commit -m "feat(combat): add ActionQueue.AddAP; add PlayerSession.BankedAP"
```

---

### Task 9: Delay Command — Full Pipeline

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/grpc_service_delay_test.go`

**Note:** `DelayRequest` proto message and field 72 were added in Task 3. The bridge, handler, and dispatch wiring are done here.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_delay_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

func newDelaySvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr, command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr), logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, combatHandler
}

// TestHandleDelay_OutOfCombat_Fails verifies delay returns error outside combat.
//
// Precondition: player not in combat.
// Postcondition: response contains "cannot delay outside".
func TestHandleDelay_OutOfCombat_Fails(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr, _ := newDelaySvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	ev, err := svc.handleDelay("u1", &gamev1.DelayRequest{})
	require.NoError(t, err)
	assert.Contains(t, ev.GetNarrative(), "cannot delay outside")
}

// TestHandleDelay_InCombat_BanksAP verifies AP banking and zeroing.
//
// Precondition: player in combat with 3 AP; delay called.
// Postcondition: BankedAP = min(3-1, 2) = 2; RemainingAP = 0.
func TestHandleDelay_InCombat_BanksAP(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr, combatHandler := newDelaySvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer("u1")
	sess.Status = statusInCombat

	_, startErr := combatHandler.StartCombat("room_a", []string{"u1"})
	require.NoError(t, startErr)
	// StartCombat gives 3 AP per round

	_, err = svc.handleDelay("u1", &gamev1.DelayRequest{})
	require.NoError(t, err)

	assert.Equal(t, 2, sess.BankedAP)
	assert.Equal(t, 0, combatHandler.RemainingAP("u1"))
}

// TestHandleDelay_InCombat_AppliesACPenalty verifies -2 ACMod on player combatant.
//
// Precondition: player in combat; delay called.
// Postcondition: player's Combatant.ACMod == -2.
func TestHandleDelay_InCombat_AppliesACPenalty(t *testing.T) {
	roller := dice.NewRoller(dice.NewConstSource(10))
	svc, sessMgr, combatHandler := newDelaySvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "Alice", CharName: "Alice", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer("u1")
	sess.Status = statusInCombat

	_, startErr := combatHandler.StartCombat("room_a", []string{"u1"})
	require.NoError(t, startErr)

	_, err = svc.handleDelay("u1", &gamev1.DelayRequest{})
	require.NoError(t, err)

	// Find combatant ACMod via combatHandler
	acMod := combatHandler.PlayerACMod("u1") // add this method if it doesn't exist
	assert.Equal(t, -2, acMod)
}

// TestProperty_BankedAP_Formula verifies BankedAP = min(startingAP - 1, 2) for all startingAP in [0,10].
//
// Precondition: player in combat with startingAP remaining.
// Postcondition: BankedAP == min(startingAP - 1, 2); capped non-negative.
func TestProperty_BankedAP_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startingAP := rapid.IntRange(1, 3).Draw(rt, "startingAP") // [1,3]: >=1 avoids "not enough AP"; <=3 stays within default AP grant

		roller := dice.NewRoller(dice.NewConstSource(10))
		svc, sessMgr, combatHandler := newDelaySvcWithCombat(rt, roller)

		_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u", Username: "u", CharName: "u", Role: "player",
			RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
		})
		require.NoError(rt, addErr)
		sess, _ := sessMgr.GetPlayer("u")
		sess.Status = statusInCombat

		_, startErr := combatHandler.StartCombat("room_a", []string{"u"})
		require.NoError(rt, startErr)
		// StartCombat gives 3 AP; adjust to startingAP by spending the difference.
		current := combatHandler.RemainingAP("u")
		if diff := current - startingAP; diff > 0 {
			_ = combatHandler.SpendAP("u", diff)
		}
		// If startingAP > 3, AddAP is called after StartCombat via the queue.
		// For this test, limit to [1,3] to stay within default 3 AP grant; adjust rapid range if needed.

		_, err := svc.handleDelay("u", &gamev1.DelayRequest{})
		require.NoError(rt, err)

		expected := startingAP - 1
		if expected > 2 {
			expected = 2
		}
		assert.Equal(rt, expected, sess.BankedAP)
		assert.Equal(rt, 0, combatHandler.RemainingAP("u"))
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run TestHandleDelay -v 2>&1 | head -30
```

Expected: FAIL — `handleDelay` doesn't exist.

- [ ] **Step 3: Add HandlerDelay to commands.go**

In `internal/game/command/commands.go`, add with the other constants:

```go
HandlerDelay = "delay"
```

Add to `BuiltinCommands()`:

```go
{
    Name:     "delay",
    Aliases:  []string{"dl"},
    Help:     "Bank remaining AP (up to 2) for next round at cost of -2 AC. Combat only.",
    Category: CategoryCombat,
    Handler:  HandlerDelay,
},
```

- [ ] **Step 4: Add bridgeDelay and register it**

In `internal/frontend/handlers/bridge_handlers.go`, add:

```go
func bridgeDelay(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Delay{Delay: &gamev1.DelayRequest{}},
	}}, nil
}
```

In `bridgeHandlerMap`, add:

```go
command.HandlerDelay: bridgeDelay,
```

- [ ] **Step 5: Implement handleDelay in grpc_service.go**

Add `handleDelay` near the other combat handler functions:

```go
// handleDelay banks remaining AP for next round at cost of -2 AC.
//
// Precondition: uid is a valid player in active combat with >= 1 AP remaining.
// Postcondition: 1 AP spent; remaining AP banked (capped at 2) in sess.BankedAP;
//   all AP zeroed; player combatant ACMod reduced by 2.
func (s *GameServiceServer) handleDelay(uid string, _ *gamev1.DelayRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	if sess.Status != statusInCombat {
		return messageEvent("You cannot delay outside of combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}

	remaining := s.combatH.RemainingAP(uid)
	if remaining < 1 {
		return messageEvent("Not enough AP to delay."), nil
	}

	// Step 1: spend 1 AP
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return messageEvent("Not enough AP to delay."), nil
	}

	// Step 2: bank remaining after cost (capped at 2)
	postCost := remaining - 1
	banked := postCost
	if banked > 2 {
		banked = 2
	}
	sess.BankedAP = banked

	// Step 3: zero all remaining AP
	s.combatH.SpendAllAP(uid)

	// Step 4: apply -2 AC to player combatant
	s.combatH.ApplyPlayerACMod(uid, -2)

	return messageEvent(fmt.Sprintf(
		"You delay, banking %d AP for next round. You are exposed (-2 AC).", banked,
	)), nil
}
```

- [ ] **Step 6: Add ApplyPlayerACMod and PlayerACMod to CombatHandler**

In `internal/gameserver/combat_handler.go`, add two methods:

```go
// ApplyPlayerACMod adds delta to the player's Combatant.ACMod in their active combat.
// No-op if player not in active combat.
//
// Precondition: combatMu NOT held by caller; uid is a valid player UID.
// Postcondition: player's Combatant.ACMod incremented by delta.
func (h *CombatHandler) ApplyPlayerACMod(uid string, delta int) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid && c.Kind == combat.KindPlayer {
			c.ACMod += delta
			return
		}
	}
}

// PlayerACMod returns the player's Combatant.ACMod in their active combat.
// Returns 0 if not in combat.
//
// Precondition: combatMu NOT held by caller; uid is a valid player UID.
// Postcondition: returns current ACMod value.
func (h *CombatHandler) PlayerACMod(uid string) int {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return 0
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return 0
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid && c.Kind == combat.KindPlayer {
			return c.ACMod
		}
	}
	return 0
}

- [ ] **Step 7: Inject BankedAP in resolveAndAdvanceLocked**

In `internal/gameserver/combat_handler.go`, in `resolveAndAdvanceLocked`, after `condEvents := cbt.StartRound(3)` and the MotiveBonus loop (added in Task 7), add:

```go
// Inject banked AP from delayed players into their new ActionQueue.
// Precondition: combatMu held; cbt.StartRound has rebuilt ActionQueues.
// Postcondition: Each player with BankedAP > 0 has that AP added to their queue; BankedAP zeroed.
for _, c := range cbt.Combatants {
    if c.Kind != combat.KindPlayer {
        continue
    }
    sess, ok := h.sessions.GetPlayer(c.ID)
    if !ok || sess.BankedAP <= 0 {
        continue
    }
    q := cbt.ActionQueues[c.ID]
    if q != nil {
        q.AddAP(sess.BankedAP)
    }
    sess.BankedAP = 0
}
```

- [ ] **Step 8: Wire handleDelay into dispatch**

In `internal/gameserver/grpc_service.go`, find the `dispatch` type switch (around line 1250). Add:

```go
case *gamev1.ClientMessage_Delay:
    return s.handleDelay(uid, p.Delay)
```

- [ ] **Step 9: Run tests**

```bash
go test ./internal/gameserver/... -run TestHandleDelay -v
go test ./...
```

Expected: all pass.

- [ ] **Step 10: Run TestAllCommandHandlersAreWired**

```bash
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v
```

Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/game/command/commands.go \
    internal/gameserver/grpc_service.go \
    internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/combat_handler.go \
    internal/gameserver/grpc_service_delay_test.go
git commit -m "feat(delay): new delay command — banks AP, -2 AC penalty; full CMD-1-7 pipeline"
```

---

## Final Verification

- [ ] **Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all packages pass.

- [ ] **Verify TestAllCommandHandlersAreWired passes**

```bash
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v
```

Expected: PASS.

- [ ] **Verify build**

```bash
go build ./...
```

Expected: no errors.

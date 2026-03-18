# Passive Tech Mechanics Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `seismic_sense` fire automatically for all players in a room when room state changes (player enters/exits), with no explicit activation required.

**Architecture:** Add `Passive bool` to `TechnologyDef` (data model + YAML); add `triggerPassiveTechsForRoom(roomID string)` to `GameServiceServer` which finds all players in the room via `sessions.PlayersInRoomDetails` and fires passive innate techs via the existing `activateTechWithEffects` + entity push; wire into `handleMove` for both source and destination rooms.

**Tech Stack:** Go 1.22+, `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify`.

**Spec:** `docs/superpowers/specs/2026-03-18-passive-tech-design.md`

---

## Key Existing APIs

### TechnologyDef struct (`internal/game/technology/model.go`, lines 165–182)

```go
type TechnologyDef struct {
    ID          string    `yaml:"id"`
    Name        string    `yaml:"name"`
    Description string    `yaml:"description,omitempty"`
    Tradition   Tradition `yaml:"tradition"`
    Level       int       `yaml:"level"`
    UsageType   UsageType `yaml:"usage_type"`
    ActionCost  int       `yaml:"action_cost"`
    Range       Range     `yaml:"range"`
    Targets     Targets   `yaml:"targets"`
    Duration    string    `yaml:"duration"`
    SaveType    string    `yaml:"save_type,omitempty"`
    SaveDC      int       `yaml:"save_dc,omitempty"`
    Resolution   string        `yaml:"resolution,omitempty"`
    Effects      TieredEffects `yaml:"effects,omitempty"`
    AmpedLevel   int           `yaml:"amped_level,omitempty"`
    AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
}
```

`Passive bool` field is NOT yet present — this is the first change of Chunk 1.

### Validate signature (`internal/game/technology/model.go`, line 187)

```go
func (t *TechnologyDef) Validate() error
```

### activateTechWithEffects signature (`internal/gameserver/grpc_service.go`, line 4971)

```go
func (s *GameServiceServer) activateTechWithEffects(
    sess *session.PlayerSession,
    uid, abilityID, targetID, fallbackMsg string,
) (*gamev1.ServerEvent, error)
```

Returns a `*gamev1.ServerEvent` (always non-nil on success) and an error. When `techRegistry` is nil or the tech is unknown, returns a message event with `fallbackMsg` and `nil` error.

### PlayersInRoomDetails signature (`internal/game/session/manager.go`, line 340)

```go
func (m *Manager) PlayersInRoomDetails(roomID string) []*PlayerSession
```

Returns a non-nil slice (may be empty); each element is a non-nil `*PlayerSession`.

### PlayerSession.UID field (`internal/game/session/manager.go`, line 18)

`PlayerSession` has a `UID string` field at line 18. Use `sess.UID` directly.

### BridgeEntity.Push (`internal/game/session/entity.go`, line 43)

```go
func (e *BridgeEntity) Push(data []byte) error
```

`PlayerSession.Entity` is `*session.BridgeEntity`. Push returns an error if the entity is closed or the buffer is full.

### handleMove broadcast section (`internal/gameserver/grpc_service.go`, lines 1457–1469)

```go
// Broadcast departure from old room
s.broadcastRoomEvent(result.OldRoomID, uid, &gamev1.RoomEvent{
    Player:    sess.CharName,
    Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
    Direction: string(dir),
})

// Broadcast arrival in new room
s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
    Player:    sess.CharName,
    Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
    Direction: string(dir.Opposite()),
})
```

Insertion point for source room passive trigger: after line 1462 (after departure broadcast).
Insertion point for destination room passive trigger: after line 1469 (after arrival broadcast).

### testMinimalService (`internal/gameserver/grpc_service_grant_test.go`, line 168)

```go
func testMinimalService(t *testing.T, sessMgr *session.Manager) *GameServiceServer
```

Returns a `*GameServiceServer` with nil `techRegistry`. Tests that need a tech registry must call `svc.SetTechRegistry(r)` after construction. The registry setter is at `grpc_service.go` line 368:

```go
func (s *GameServiceServer) SetTechRegistry(r *technology.Registry) { s.techRegistry = r }
```

### Technology Registry (`internal/game/technology/`)

```go
func (r *Registry) Get(id string) (*TechnologyDef, bool)
```

---

## Chunk 1: Data model

### Task 1: Add Passive field to TechnologyDef + update seismic_sense.yaml

**Files touched:**
- `internal/game/technology/model.go`
- `internal/game/technology/model_test.go`
- `content/technologies/innate/seismic_sense.yaml`

#### Step 1.1 — Write failing tests

Add to `internal/game/technology/model_test.go` (after existing tests, before the closing of the file):

```go
// REQ-PTM2: Passive: true + action_cost > 0 fails validation
func TestValidate_REQ_PTM2_PassiveRequiresZeroActionCost(t *testing.T) {
    d := validDef()
    d.Passive = true
    d.ActionCost = 1
    err := d.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "passive")
}

// REQ-PTM1: Passive: true + action_cost == 0 passes validation
func TestValidate_REQ_PTM1_PassiveWithZeroActionCostValid(t *testing.T) {
    d := validDef()
    d.Passive = true
    d.ActionCost = 0
    require.NoError(t, d.Validate())
}

// REQ-PTM1: YAML round-trip — seismic_sense has passive: true and action_cost: 0
func TestSeismicSense_IsPassive(t *testing.T) {
    data, err := os.ReadFile("../../../content/technologies/innate/seismic_sense.yaml")
    require.NoError(t, err)
    var def technology.TechnologyDef
    require.NoError(t, yaml.Unmarshal(data, &def))
    assert.True(t, def.Passive)
    assert.Equal(t, 0, def.ActionCost)
}
```

Also add `"os"` to the import block in `model_test.go`.

Run to confirm RED:

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... 2>&1 | tail -20
```

Expected: compilation errors (`d.Passive undefined`) confirming tests fail as required.

#### Step 1.2 — Add Passive field to TechnologyDef

In `internal/game/technology/model.go`, add `Passive bool` to `TechnologyDef` after the `AmpedEffects` field:

```go
// TechnologyDef defines a single technology — the game's analog of a PF2E spell.
// ...
type TechnologyDef struct {
    ID          string    `yaml:"id"`
    Name        string    `yaml:"name"`
    Description string    `yaml:"description,omitempty"`
    Tradition   Tradition `yaml:"tradition"`
    Level       int       `yaml:"level"`
    UsageType   UsageType `yaml:"usage_type"`
    ActionCost  int       `yaml:"action_cost"`
    Range       Range     `yaml:"range"`
    Targets     Targets   `yaml:"targets"`
    Duration    string    `yaml:"duration"`
    SaveType    string    `yaml:"save_type,omitempty"`
    SaveDC      int       `yaml:"save_dc,omitempty"`
    Resolution   string        `yaml:"resolution,omitempty"`
    Effects      TieredEffects `yaml:"effects,omitempty"`
    AmpedLevel   int           `yaml:"amped_level,omitempty"`
    AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
    // Passive indicates this technology fires automatically on room state changes
    // and requires no player action. When true, ActionCost must be 0.
    Passive bool `yaml:"passive,omitempty"`
}
```

#### Step 1.3 — Update Validate() to enforce REQ-PTM2

In `internal/game/technology/model.go`, inside `Validate()`, add after the `ActionCost` is available (insert before the `if !validRanges[t.Range]` check, i.e., after the `UsageType` check at line ~200):

```go
if t.Passive && t.ActionCost != 0 {
    return fmt.Errorf("passive technology %q must have action_cost 0, got %d", t.ID, t.ActionCost)
}
```

#### Step 1.4 — Update seismic_sense.yaml

Edit `content/technologies/innate/seismic_sense.yaml`: change `action_cost: 1` to `action_cost: 0` and add `passive: true` after the `action_cost` line:

```yaml
id: seismic_sense
name: Seismic Sense
description: Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.
tradition: technical
level: 1
usage_type: innate
action_cost: 0
passive: true
range: zone
targets: single
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: utility
      description: "Your bone-conduction implants detect ground vibrations. You sense the movement of all creatures in the room through the floor."
```

#### Step 1.5 — Run tests GREEN

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/technology/... -v 2>&1 | tail -30
```

Expected: all tests pass including the three new ones.

#### Step 1.6 — Run full suite

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | tail -40
```

Expected: all packages `ok`, none `FAIL`.

#### Step 1.7 — Commit

```bash
cd /home/cjohannsen/src/mud && git add \
    internal/game/technology/model.go \
    internal/game/technology/model_test.go \
    content/technologies/innate/seismic_sense.yaml && \
git commit -m "$(cat <<'EOF'
feat(technology): add Passive field to TechnologyDef; mark seismic_sense passive (REQ-PTM1, REQ-PTM2, REQ-PTM3)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Chunk 2: triggerPassiveTechsForRoom + handleMove wiring

### Task 2: Implement triggerPassiveTechsForRoom + wire into handleMove

**Files touched:**
- `internal/gameserver/grpc_service.go`
- `internal/gameserver/grpc_service_passive_test.go` (new)

#### Step 2.1 — Write failing tests (new file)

Create `internal/gameserver/grpc_service_passive_test.go`:

```go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
    "go.uber.org/zap/zaptest"
)

// buildPassiveRegistry builds a minimal *technology.Registry containing a single passive tech.
//
// Precondition: techID must be non-empty.
// Postcondition: Returns a non-nil *technology.Registry with one passive TechnologyDef registered.
func buildPassiveRegistry(t *testing.T, techID string) *technology.Registry {
    t.Helper()
    def := &technology.TechnologyDef{
        ID:         techID,
        Name:       "Test Passive Tech",
        Tradition:  technology.TraditionTechnical,
        Level:      1,
        UsageType:  technology.UsageInnate,
        ActionCost: 0,
        Passive:    true,
        Range:      technology.RangeZone,
        Targets:    technology.TargetsSingle,
        Duration:   "instant",
        Resolution: "none",
        Effects: technology.TieredEffects{
            OnApply: []technology.TechEffect{
                {Type: technology.EffectUtility, Description: "passive fires"},
            },
        },
    }
    require.NoError(t, def.Validate())
    reg := technology.NewRegistry()
    reg.Register(def)
    return reg
}

// addPlayerWithInnate adds a player with a single innate tech slot to sessMgr.
//
// Precondition: sessMgr must be non-nil; uid and roomID must be non-empty.
// Postcondition: Player is in sessMgr with InnateTechs set; the *PlayerSession is returned.
func addPlayerWithInnate(t *testing.T, sessMgr *session.Manager, uid, roomID, techID string) *session.PlayerSession {
    t.Helper()
    _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID:      uid,
        Username: uid,
        CharName: uid,
        RoomID:   roomID,
        Role:     "player",
    })
    require.NoError(t, err)
    sess, ok := sessMgr.GetPlayer(uid)
    require.True(t, ok)
    sess.InnateTechs = map[string]*session.InnateSlot{
        techID: {TechID: techID},
    }
    return sess
}

// TestTriggerPassiveTechsForRoom_PassiveFires verifies that a player with a passive
// innate tech and a non-nil entity receives an event when triggerPassiveTechsForRoom
// is called for their room (REQ-PTM4, REQ-PTM6).
func TestTriggerPassiveTechsForRoom_PassiveFires(t *testing.T) {
    const (
        uid    = "player-passive-1"
        roomID = "room_passive"
        techID = "seismic_sense"
    )
    sessMgr := session.NewManager()
    svc := testMinimalService(t, sessMgr)
    svc.SetTechRegistry(buildPassiveRegistry(t, techID))

    sess := addPlayerWithInnate(t, sessMgr, uid, roomID, techID)

    // Drain any existing events to get a clean baseline.
    for len(sess.Entity.Events()) > 0 {
        <-sess.Entity.Events()
    }

    svc.triggerPassiveTechsForRoom(roomID)

    // The entity channel must have received exactly one event.
    assert.Equal(t, 1, len(sess.Entity.Events()), "expected one event pushed to entity")
}

// TestTriggerPassiveTechsForRoom_NonPassiveDoesNotFire verifies that a player with
// a non-passive innate tech does not receive an event (REQ-PTM4).
func TestTriggerPassiveTechsForRoom_NonPassiveDoesNotFire(t *testing.T) {
    const (
        uid      = "player-nonpassive-1"
        roomID   = "room_nonpassive"
        techID   = "active_tech"
    )
    sessMgr := session.NewManager()
    svc := testMinimalService(t, sessMgr)

    // Register a non-passive tech.
    def := &technology.TechnologyDef{
        ID:         techID,
        Name:       "Active Tech",
        Tradition:  technology.TraditionTechnical,
        Level:      1,
        UsageType:  technology.UsageInnate,
        ActionCost: 1,
        Passive:    false,
        Range:      technology.RangeZone,
        Targets:    technology.TargetsSingle,
        Duration:   "instant",
        Resolution: "none",
        Effects: technology.TieredEffects{
            OnApply: []technology.TechEffect{
                {Type: technology.EffectUtility, Description: "active fires"},
            },
        },
    }
    require.NoError(t, def.Validate())
    reg := technology.NewRegistry()
    reg.Register(def)
    svc.SetTechRegistry(reg)

    sess := addPlayerWithInnate(t, sessMgr, uid, roomID, techID)

    svc.triggerPassiveTechsForRoom(roomID)

    assert.Equal(t, 0, len(sess.Entity.Events()), "non-passive tech must not push any event")
}

// TestTriggerPassiveTechsForRoom_NilEntitySkipped verifies that a player with a nil
// entity is skipped silently without panic (REQ-PTM4).
func TestTriggerPassiveTechsForRoom_NilEntitySkipped(t *testing.T) {
    const (
        uid    = "player-nil-entity"
        roomID = "room_nil_entity"
        techID = "seismic_sense"
    )
    sessMgr := session.NewManager()
    svc := testMinimalService(t, sessMgr)
    svc.SetTechRegistry(buildPassiveRegistry(t, techID))

    sess := addPlayerWithInnate(t, sessMgr, uid, roomID, techID)
    sess.Entity = nil

    // Must not panic.
    require.NotPanics(t, func() {
        svc.triggerPassiveTechsForRoom(roomID)
    })
}

// TestTriggerPassiveTechsForRoom_TechNotInRegistrySkipped verifies that an innate tech
// with no registry definition is skipped silently without panic (REQ-PTM4).
func TestTriggerPassiveTechsForRoom_TechNotInRegistrySkipped(t *testing.T) {
    const (
        uid    = "player-missing-tech"
        roomID = "room_missing_tech"
        techID = "unknown_passive_tech"
    )
    sessMgr := session.NewManager()
    svc := testMinimalService(t, sessMgr)
    // Registry is empty — techID has no definition.
    svc.SetTechRegistry(technology.NewRegistry())

    sess := addPlayerWithInnate(t, sessMgr, uid, roomID, techID)

    // Must not panic and entity must receive no events.
    require.NotPanics(t, func() {
        svc.triggerPassiveTechsForRoom(roomID)
    })
    assert.Equal(t, 0, len(sess.Entity.Events()))
}

// TestPropertyTriggerPassiveTechsForRoom_AllPlayersReceive is a property-based test
// verifying that every player in the room with a passive innate tech receives exactly
// one event, regardless of player count (REQ-PTM4, REQ-PTM6).
func TestPropertyTriggerPassiveTechsForRoom_AllPlayersReceive(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        numPlayers := rapid.IntRange(1, 5).Draw(rt, "numPlayers")
        const roomID = "room_prop"
        const techID = "seismic_sense"

        sessMgr := session.NewManager()
        svc := testMinimalService(t, sessMgr)
        svc.SetTechRegistry(buildPassiveRegistry(t, techID))

        sessions := make([]*session.PlayerSession, 0, numPlayers)
        for i := 0; i < numPlayers; i++ {
            uid := fmt.Sprintf("player-prop-%d", i)
            sess := addPlayerWithInnate(t, sessMgr, uid, roomID, techID)
            // Drain any pre-existing events.
            for len(sess.Entity.Events()) > 0 {
                <-sess.Entity.Events()
            }
            sessions = append(sessions, sess)
        }

        svc.triggerPassiveTechsForRoom(roomID)

        for i, sess := range sessions {
            if len(sess.Entity.Events()) != 1 {
                rt.Fatalf("player %d: expected 1 event, got %d", i, len(sess.Entity.Events()))
            }
        }
    })
}
```

Run to confirm RED (method does not exist yet):

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestTriggerPassive 2>&1 | tail -20
```

Expected: compilation error — `svc.triggerPassiveTechsForRoom undefined`.

#### Step 2.2 — Implement triggerPassiveTechsForRoom

Add the following method to `internal/gameserver/grpc_service.go` (insert after `activateTechWithEffects` near line 5000):

```go
// triggerPassiveTechsForRoom fires all passive innate technologies for every player
// currently in roomID.
//
// Precondition: roomID may be any string; nil or missing sessions are skipped silently.
// Postcondition: Each player with a passive innate tech in their InnateTechs map receives
// a push event on their BridgeEntity. Players with nil Entity or techs absent from the
// registry are skipped without error. Innate tech use counts are never decremented.
func (s *GameServiceServer) triggerPassiveTechsForRoom(roomID string) {
    if s.techRegistry == nil {
        return
    }
    players := s.sessions.PlayersInRoomDetails(roomID)
    for _, sess := range players {
        if sess.Entity == nil {
            continue
        }
        for techID := range sess.InnateTechs {
            def, ok := s.techRegistry.Get(techID)
            if !ok {
                s.logger.Debug("passive tech not in registry", zap.String("techID", techID))
                continue
            }
            if !def.Passive {
                continue
            }
            evt, err := s.activateTechWithEffects(sess, sess.UID, techID, "", "")
            if err != nil {
                s.logger.Warn("passive tech activation error",
                    zap.String("uid", sess.UID),
                    zap.String("techID", techID),
                    zap.Error(err))
                continue
            }
            if evt == nil {
                continue
            }
            data, marshalErr := proto.Marshal(evt)
            if marshalErr != nil {
                s.logger.Warn("passive tech marshal error",
                    zap.String("uid", sess.UID),
                    zap.Error(marshalErr))
                continue
            }
            if pushErr := sess.Entity.Push(data); pushErr != nil {
                s.logger.Warn("pushing passive tech event",
                    zap.String("uid", sess.UID),
                    zap.Error(pushErr))
            }
        }
    }
}
```

#### Step 2.3 — Wire into handleMove

In `internal/gameserver/grpc_service.go`, after the departure broadcast block (after line 1462) and after the arrival broadcast block (after line 1469), insert the two calls:

```go
    // Broadcast departure from old room
    s.broadcastRoomEvent(result.OldRoomID, uid, &gamev1.RoomEvent{
        Player:    sess.CharName,
        Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_DEPART,
        Direction: string(dir),
    })
    // Notify remaining players in source room of the departure via passive techs.
    s.triggerPassiveTechsForRoom(result.OldRoomID)

    // Broadcast arrival in new room
    s.broadcastRoomEvent(result.View.RoomId, uid, &gamev1.RoomEvent{
        Player:    sess.CharName,
        Type:      gamev1.RoomEventType_ROOM_EVENT_TYPE_ARRIVE,
        Direction: string(dir.Opposite()),
    })
    // Notify all players in destination room of the arrival via passive techs.
    s.triggerPassiveTechsForRoom(result.View.RoomId)
```

Note: by the time `triggerPassiveTechsForRoom` is called for `result.OldRoomID`, the session manager has already moved the player to the new room (via `MovePlayer` called earlier in `handleMove`), so the departing player is NOT present in the source room and will not receive source-room passive output. This satisfies REQ-PTM5.

#### Step 2.4 — Run tests GREEN

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestTriggerPassive -v 2>&1 | tail -30
```

Expected: all `TestTriggerPassive*` tests pass.

#### Step 2.5 — Run full suite

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | tail -40
```

Expected: all packages `ok`, none `FAIL`.

#### Step 2.6 — Commit

```bash
cd /home/cjohannsen/src/mud && git add \
    internal/gameserver/grpc_service.go \
    internal/gameserver/grpc_service_passive_test.go && \
git commit -m "$(cat <<'EOF'
feat(gameserver): add triggerPassiveTechsForRoom; wire into handleMove for passive tech auto-fire (REQ-PTM4, REQ-PTM5, REQ-PTM6)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Chunk 3: FEATURES.md

### Task 3: Mark Passive Tech Mechanics complete in FEATURES.md

#### Step 3.1 — Locate and update FEATURES.md

```bash
grep -n "passive\|Passive\|seismic" /home/cjohannsen/src/mud/docs/FEATURES.md | head -20
```

Find the relevant entry and mark it complete (change `[ ]` to `[x]`, or add the entry if absent).

If no entry exists, add the following under the appropriate section (Innate Technologies or a new Passive Tech section):

```markdown
- [x] Passive Tech Mechanics — `seismic_sense` fires automatically on room enter/exit for all players in room (REQ-PTM1–PTM6)
```

#### Step 3.2 — Commit

```bash
cd /home/cjohannsen/src/mud && git add docs/FEATURES.md && \
git commit -m "$(cat <<'EOF'
docs(features): mark Passive Tech Mechanics complete

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Implementation Notes

### Import requirements for grpc_service_passive_test.go

The test file uses `fmt` for `fmt.Sprintf` in the property test. Ensure the import block includes:

```go
import (
    "fmt"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/technology"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)
```

Note: `zaptest` import is NOT needed in the test file since `testMinimalService` constructs its own logger internally.

### technology.NewRegistry() and Register()

Before writing tests, verify the Registry constructor and Register method signatures:

```bash
grep -n "func NewRegistry\|func.*Register" /home/cjohannsen/src/mud/internal/game/technology/registry.go | head -10
```

If the method is named differently (e.g., `Add` instead of `Register`), update the test helper accordingly.

### InnateSlot struct

`session.InnateSlot` is referenced in `PlayerSession.InnateTechs map[string]*InnateSlot`. Verify the struct definition before writing tests:

```bash
grep -n "InnateSlot" /home/cjohannsen/src/mud/internal/game/session/*.go | head -10
```

### proto.Marshal import

`triggerPassiveTechsForRoom` calls `proto.Marshal(evt)`. The `grpc_service.go` file already imports `google.golang.org/protobuf/proto` — confirm with:

```bash
grep "proto\." /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -5
```

If the import uses `github.com/golang/protobuf/proto` instead, use that package's `Marshal` function signature.

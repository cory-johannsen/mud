# Zone Effect Triggers Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rooms can declare mental-state effects (Rage, Despair, Delirium, Fear) that fire on player entry and each combat round, with cooldown immunity after successful Grit saves.

**Architecture:** `RoomEffect` is a new value type on the `Room` struct loaded from YAML. `PlayerSession` gains `ZoneEffectCooldowns map[string]int64` keyed by `roomID:track`. Effects are checked in `autoQueueNPCsLocked` (combat) and `handleMove` (non-combat). Save resolution is binary (`d20 + GritMod vs BaseDC`) with no proficiency bonus — consistent between both paths. Cooldown keys from prior rooms are decremented across all rounds regardless of current room.

**Tech Stack:** Go 1.26, gopkg.in/yaml.v3, pgregory.net/rapid

**Spec:** `docs/superpowers/specs/2026-03-15-zone-effect-triggers-design.md`

---

## Chunk 1: Data Model

### Task 1: RoomEffect type, Room struct, YAML loader

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`
- Test: `internal/game/world/zone_effect_test.go` (create)

**Context:** `Room` struct is at `internal/game/world/model.go:126`. `yamlRoom` struct is at `internal/game/world/loader.go:49`. Room construction is in `convertYAMLZone` at `loader.go:141`. `LoadZoneFromBytes(data []byte) (*Zone, error)` already exists at `loader.go:86` — no need to create it.

- [ ] **Step 1: Write failing tests**

Create `internal/game/world/zone_effect_test.go`:

```go
package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const zoneWithEffectsYAML = `
zone:
  id: test_zone
  name: Test Zone
  description: A test zone.
  start_room: room_a
  rooms:
    - id: room_a
      title: Room A
      description: A room with a zone effect.
      map_x: 0
      map_y: 0
      effects:
        - track: despair
          severity: mild
          base_dc: 12
          cooldown_rounds: 3
          cooldown_minutes: 5
    - id: room_b
      title: Room B
      description: A room with no effects.
      map_x: 1
      map_y: 0
`

func TestRoomEffect_LoadedFromYAML(t *testing.T) {
	zone, err := world.LoadZoneFromBytes([]byte(zoneWithEffectsYAML))
	require.NoError(t, err)

	roomA, ok := zone.Rooms["room_a"]
	require.True(t, ok)
	require.Len(t, roomA.Effects, 1)

	e := roomA.Effects[0]
	assert.Equal(t, "despair", e.Track)
	assert.Equal(t, "mild", e.Severity)
	assert.Equal(t, 12, e.BaseDC)
	assert.Equal(t, 3, e.CooldownRounds)
	assert.Equal(t, 5, e.CooldownMinutes)
}

func TestRoomEffect_NoEffects_EmptySlice(t *testing.T) {
	zone, err := world.LoadZoneFromBytes([]byte(zoneWithEffectsYAML))
	require.NoError(t, err)

	roomB, ok := zone.Rooms["room_b"]
	require.True(t, ok)
	assert.Empty(t, roomB.Effects, "room with no effects declaration should have empty slice")
}

func TestRoomEffect_MultipleEffects(t *testing.T) {
	const twoEffectsYAML = `
zone:
  id: test
  name: Test
  description: Test.
  start_room: r
  rooms:
    - id: r
      title: R
      description: R.
      map_x: 0
      map_y: 0
      effects:
        - track: despair
          severity: mild
          base_dc: 12
          cooldown_rounds: 3
          cooldown_minutes: 5
        - track: delirium
          severity: moderate
          base_dc: 14
          cooldown_rounds: 4
          cooldown_minutes: 10
`
	zone, err := world.LoadZoneFromBytes([]byte(twoEffectsYAML))
	require.NoError(t, err)
	r, ok := zone.Rooms["r"]
	require.True(t, ok)
	assert.Len(t, r.Effects, 2)
}

func TestRoomEffect_ZeroEffects_NoYAMLKey(t *testing.T) {
	// Rooms without effects: key should be missing; should unmarshal cleanly.
	const noEffectsYAML = `
zone:
  id: test
  name: Test
  description: Test.
  start_room: r
  rooms:
    - id: r
      title: R
      description: R.
      map_x: 0
      map_y: 0
`
	zone, err := world.LoadZoneFromBytes([]byte(noEffectsYAML))
	require.NoError(t, err)
	r, ok := zone.Rooms["r"]
	require.True(t, ok)
	assert.Empty(t, r.Effects)
}

// Ensure RoomEffect fields are exported (compile-time check via field access).
func TestRoomEffect_FieldsExported(t *testing.T) {
	e := world.RoomEffect{
		Track:           "rage",
		Severity:        "mild",
		BaseDC:          10,
		CooldownRounds:  2,
		CooldownMinutes: 3,
	}
	assert.Equal(t, "rage", e.Track)
	_ = yaml.Marshal(e) // must not panic
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestRoomEffect -v`
Expected: FAIL — `RoomEffect` type not defined.

- [ ] **Step 2: Add RoomEffect type to model.go**

In `internal/game/world/model.go`, add before the `Room` struct:

```go
// RoomEffect declares a persistent mental-state aura for a room.
// Effects fire on room entry and at the start of each combat round.
// Save resolution is binary: d20 + GritMod vs BaseDC (no proficiency bonus).
type RoomEffect struct {
	// Track is the mental state track to trigger.
	// One of "rage", "despair", "delirium", "fear".
	Track string `yaml:"track"`

	// Severity is the minimum severity to apply.
	// One of "mild", "moderate", "severe".
	Severity string `yaml:"severity"`

	// BaseDC is the Grit save difficulty. Effective save: d20 + GritMod vs BaseDC.
	BaseDC int `yaml:"base_dc"`

	// CooldownRounds is rounds of immunity after a successful in-combat save.
	CooldownRounds int `yaml:"cooldown_rounds"`

	// CooldownMinutes is minutes of immunity after a successful out-of-combat save.
	CooldownMinutes int `yaml:"cooldown_minutes"`
}
```

Add `Effects []RoomEffect` to the `Room` struct after `SkillChecks`:
```go
// Effects lists persistent mental-state auras that apply to players in this room.
Effects []RoomEffect
```

- [ ] **Step 3: Add Effects to loader.go**

In `internal/game/world/loader.go`:

Add `Effects []RoomEffect \`yaml:"effects"\`` to `yamlRoom` struct after `SkillChecks`:
```go
Effects []RoomEffect `yaml:"effects"`
```

In `convertYAMLZone`, inside the room construction block after `SkillChecks: yr.SkillChecks,`, add:
```go
Effects: yr.Effects,
```

- [ ] **Step 4: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v -count=1`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/world/model.go \
        internal/game/world/loader.go \
        internal/game/world/zone_effect_test.go
git commit -m "$(cat <<'EOF'
feat(world): add RoomEffect type and Effects field to Room; load from YAML

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: ZoneEffectCooldowns on PlayerSession

**Files:**
- Modify: `internal/game/session/manager.go`
- Test: `internal/game/session/zone_cooldown_test.go` (create)

**Context:** `PlayerSession` struct is at `internal/game/session/manager.go:15`. The last field is `PendingGroupInvite string`.

- [ ] **Step 1: Write failing tests**

Create `internal/game/session/zone_cooldown_test.go`:

```go
package session_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerSession_ZoneEffectCooldowns_NilByDefault(t *testing.T) {
	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1", CharName: "Alice", RoomID: "room1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	assert.Nil(t, sess.ZoneEffectCooldowns, "ZoneEffectCooldowns should be nil by default")
}

func TestPlayerSession_ZoneEffectCooldowns_LazyInit(t *testing.T) {
	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1", CharName: "Alice", RoomID: "room1",
		CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Should not panic on lazy init.
	if sess.ZoneEffectCooldowns == nil {
		sess.ZoneEffectCooldowns = make(map[string]int64)
	}
	sess.ZoneEffectCooldowns["room1:despair"] = 3

	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room1:despair"])
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/session/... -run TestPlayerSession_ZoneEffect -v`
Expected: FAIL — `ZoneEffectCooldowns` field not found.

- [ ] **Step 2: Add ZoneEffectCooldowns to PlayerSession**

In `internal/game/session/manager.go`, add after `PendingGroupInvite string`:

```go
// ZoneEffectCooldowns maps "roomID:track" to an immunity value.
// In combat: value is rounds remaining (decremented each round; 0 = ready to fire).
// Out of combat: value is Unix timestamp (seconds) of expiry; 0 = ready to fire.
// Nil until first use; initialized lazily on first write.
ZoneEffectCooldowns map[string]int64
```

- [ ] **Step 3: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/session/... -v -count=1 -race`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/session/manager.go \
        internal/game/session/zone_cooldown_test.go
git commit -m "$(cat <<'EOF'
feat(session): add ZoneEffectCooldowns to PlayerSession for zone effect immunity tracking

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Chunk 2: Combat and Movement Execution

### Task 3: Combat round zone effect application

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/grpc_service_zone_effect_test.go` (create)

**Context:**
- `autoQueueNPCsLocked` is at `combat_handler.go:1914`.
- NPC ability cooldown decrement loop is at lines 1916–1930.
- The room lookup for NPC auto-cover is at lines 1931–1936: `if r, ok := h.worldMgr.GetRoom(cbt.RoomID); ok { room = r }`
- `h.mentalStateMgr` is the mental state manager (may be nil).
- `abilityTrack(s string) (mentalstate.Track, bool)` is at `combat_handler.go:2121`.
- `abilitySeverity(s string) (mentalstate.Severity, bool)` is at `combat_handler.go:2136`.
- `h.applyMentalStateChanges(uid string, changes []mentalstate.StateChange) []string` is at `combat_handler.go:2796`.
- `h.sessions.GetPlayer(uid string) (*session.PlayerSession, bool)` returns the player session.
- `combat.AbilityMod(score int) int` computes modifier from ability score.
- The dice source is `h.dice.Src()` (returns a `combat.Source` which has `.Intn(n int) int`).
- The `CombatHandler` struct has `worldMgr world.WorldManager` field.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_zone_effect_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"pgregory.net/rapid"
)

// newZoneEffectCombatHandler creates a CombatHandler with a world containing
// room_a with the given zone effect, and a fixed dice source.
//
// testWorldAndSession creates a world with room_a and room_b. The effect is
// injected into room_a by mutating the Room pointer after world creation.
func newZoneEffectCombatHandler(t *testing.T, diceVal int, effect world.RoomEffect) (*CombatHandler, *session.Manager, *npc.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	src := &fixedDiceSource{val: diceVal}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	npcMgr := npc.NewManager()

	// Inject zone effect into room_a.
	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{effect}
	}

	mentalMgr := mentalstate.NewManager()

	// NewCombatHandler param order (14 params):
	// engine, npcMgr, sessions, diceRoller, broadcastFn, roundDuration,
	// condRegistry, worldMgr, scriptMgr, invRegistry, aiRegistry,
	// respawnMgr, floorMgr, mentalStateMgr
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)
	return ch, sessMgr, npcMgr
}

// TestZoneEffect_Combat_FailedSave_AppliesTrigger verifies REQ-T1:
// a player in a room with a despair effect fails the save and ApplyTrigger is called.
func TestZoneEffect_Combat_FailedSave_AppliesTrigger(t *testing.T) {
	// dice val = 0 → roll = 1, GritMod = 0 → total = 1 < BaseDC 12 → fail.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	// Run autoQueueNPCsLocked which should trigger zone effect check.
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	// Cooldown should NOT be set on a failed save (failed saves don't grant immunity).
	if sess.ZoneEffectCooldowns != nil {
		assert.Zero(t, sess.ZoneEffectCooldowns["room_a:despair"],
			"failed save must not set cooldown")
	}
	// The mental state manager on ch should show despair applied.
	// ch.mentalStateMgr is accessible in package gameserver (internal test).
	assert.Equal(t, mentalstate.SeverityMild, ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair),
		"ApplyTrigger must have been called: despair should be mild after failed save")
}

// TestZoneEffect_Combat_SuccessfulSave_SetsCooldown verifies REQ-T4:
// a player makes a successful save and cooldown is set.
func TestZoneEffect_Combat_SuccessfulSave_SetsCooldown(t *testing.T) {
	// dice val = 19 → roll = 20, GritMod = 0 → total = 20 >= BaseDC 12 → success.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 19, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	require.NotNil(t, sess.ZoneEffectCooldowns)
	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room_a:despair"],
		"successful save must set cooldown to CooldownRounds")
}

// TestZoneEffect_Combat_WithCooldown_Skipped verifies REQ-T2:
// effect is skipped when cooldown > 0.
func TestZoneEffect_Combat_WithCooldown_Skipped(t *testing.T) {
	// dice val = 0 → would fail. But cooldown is pre-set to 3.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Pre-set cooldown.
	sess.ZoneEffectCooldowns = map[string]int64{"room_a:despair": 3}

	ch.combatMu.Lock()
	cbt, _, combatErr := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, combatErr)

	// Conditions should remain empty (no trigger fired).
	condsBefore := 0
	if sess.Conditions != nil {
		condsBefore = len(sess.Conditions.All())
	}

	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	condsAfter := 0
	if sess.Conditions != nil {
		condsAfter = len(sess.Conditions.All())
	}
	assert.Equal(t, condsBefore, condsAfter, "no new conditions when effect is on cooldown")
}

// TestZoneEffect_Combat_NPCSkipped verifies REQ-T7:
// NPC combatants are not subject to zone effects.
func TestZoneEffect_Combat_NPCSkipped(t *testing.T) {
	// If NPC were checked, a failing save roll would cause issues — but NPCs have no session.
	// This test verifies no panic and NPC state unchanged.
	effect := world.RoomEffect{
		Track: "rage", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	// Should not panic even with NPC combatant in room that has zone effect.
	assert.NotPanics(t, func() {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	})
	// NPC HP unchanged.
	assert.Equal(t, 10, inst.CurrentHP)
}

// TestZoneEffect_Combat_MentalStateMgrNil_NoPanic verifies REQ-T12.
func TestZoneEffect_Combat_MentalStateMgrNil_NoPanic(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	npcMgr := npc.NewManager()

	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{{Track: "despair", Severity: "mild", BaseDC: 12, CooldownRounds: 3}}
	}

	// Build handler with nil mentalStateMgr (param 14 = nil).
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, nil,
	)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	}, "nil mentalStateMgr must not panic")
}

// TestZoneEffect_Combat_CooldownDecrement_ReachesZero verifies REQ-T3:
// after N rounds equal to CooldownRounds, the cooldown reaches 0 and effect fires again.
func TestZoneEffect_Combat_CooldownDecrement_ReachesZero(t *testing.T) {
	// Successful save (roll=20) sets cooldown to 3; then 3 rounds of failure (roll=1) should fire.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}

	// Use seq source: round 1=19 (success, sets cooldown=3), rounds 2-5=0 (fail, but immune until cooldown=0).
	worldMgr, sessMgr := testWorldAndSession(t)
	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{effect}
	}
	npcMgr := npc.NewManager()
	mentalMgr := mentalstate.NewManager()
	// seqSource cycles: [19, 0, 0, 0, 0, ...]
	src := newSeqSource(19, 0, 0, 0, 0)
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	// Round 1: roll=19, success → cooldown set to 3.
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()
	require.NotNil(t, sess.ZoneEffectCooldowns)
	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room_a:despair"])

	// Rounds 2-4: each decrements cooldown (3→2→1→0).
	for i := 0; i < 3; i++ {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	}
	assert.Equal(t, int64(0), sess.ZoneEffectCooldowns["room_a:despair"],
		"after 3 decrements, cooldown should reach 0")

	// Round 5: dice now returns 0 (roll=1) → save fails → effect fires again.
	// Verify by checking mental state is applied (despair should be triggered).
	severityBefore := ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair)
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()
	severityAfter := ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair)
	// Severity should have increased (or stayed at max) — ApplyTrigger was called again.
	assert.GreaterOrEqual(t, int(severityAfter), int(severityBefore),
		"after cooldown reaches 0, effect must fire again on the next round with failing save")
}

// TestZoneEffect_Combat_CrossRoom_CooldownDecrement verifies REQ-T13:
// cooldowns from previously-visited rooms are decremented regardless of current room,
// preventing immunity gaming by room-swapping.
func TestZoneEffect_Combat_CrossRoom_CooldownDecrement(t *testing.T) {
	// Player has a cooldown from old_room:despair; current combat is in room_a.
	effect := world.RoomEffect{
		Track: "fear", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 19, effect) // success roll

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst := npcMgr.Spawn(tmpl, "room_a")
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Pre-inject a cooldown for a previously-visited room.
	sess.ZoneEffectCooldowns = map[string]int64{"old_room:despair": 2}

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	// Run one round — old_room:despair cooldown should decrement to 1.
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	assert.Equal(t, int64(1), sess.ZoneEffectCooldowns["old_room:despair"],
		"cross-room cooldown must be decremented even when not in old_room")
}

// TestProperty_ZoneEffect_Combat_AnyTrack verifies REQ-T9 (property-based):
// For any track, a failing save calls ApplyTrigger and does NOT set a cooldown.
// Uses outer *testing.T to create shared infrastructure (required by testWorldAndSession).
func TestProperty_ZoneEffect_Combat_AnyTrack(t *testing.T) {
	trackNames := []string{"rage", "despair", "delirium", "fear"}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, 3).Draw(rt, "track_idx")
		trackName := trackNames[idx]

		effect := world.RoomEffect{
			Track: trackName, Severity: "mild",
			BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
		}
		// diceVal=0 → roll=1, GritMod=0 → total=1 < BaseDC=12 → always fails.
		ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

		tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
		inst := npcMgr.Spawn(tmpl, "room_a")
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: rt.Name(), Username: rt.Name(), CharName: "X",
			RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
		})
		require.NoError(rt, err)

		ch.combatMu.Lock()
		cbt, _, err := ch.startCombatLocked(sess, inst)
		ch.combatMu.Unlock()
		require.NoError(rt, err)

		assert.NotPanics(rt, func() {
			ch.combatMu.Lock()
			ch.autoQueueNPCsLocked(cbt)
			ch.combatMu.Unlock()
		})
		// Failed save must NOT set cooldown (REQ-T9 invariant).
		if sess.ZoneEffectCooldowns != nil {
			assert.Zero(rt, sess.ZoneEffectCooldowns["room_a:"+trackName],
				"failed save must not set cooldown for track %s", trackName)
		}
		// ApplyTrigger must have been called — mental state should be non-zero.
		// Map track name to mentalstate.Track constant.
		trackConst := map[string]mentalstate.Track{
			"rage":     mentalstate.TrackRage,
			"despair":  mentalstate.TrackDespair,
			"delirium": mentalstate.TrackDelirium,
			"fear":     mentalstate.TrackFear,
		}[trackName]
		uid := rt.Name()
		assert.NotEqual(rt, mentalstate.SeverityNone, ch.mentalStateMgr.CurrentSeverity(uid, trackConst),
			"ApplyTrigger must set non-zero severity for track %s after failed save", trackName)
	})
}

**Pre-implementation verification (run before writing tests):**
- `fixedDiceSource` is defined in `grpc_service_test.go:36` — `type fixedDiceSource struct{ val int }` — do NOT redeclare it.
- `testWorldAndSession` is defined in `world_handler_test.go:19` — it creates rooms `room_a` and `room_b`.
- `makeTestConditionRegistry` is defined in `combat_handler_test.go:69`.
- `testRoundDuration` is defined in `combat_handler_test.go:62`.
- `NewCombatHandler` has 14 parameters — worldMgr is param 8, mentalStateMgr is param 14.
- Use `mentalstate.NewManager()` directly (no helper needed).
- Use `dice.NewLoggedRoller(src, zap.NewNop())` directly (no makeDiceRoller helper exists).

Run: `cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestZoneEffect_Combat -v`
Expected: FAIL — zone effect logic not implemented yet.

- [ ] **Step 2: Implement zone effect checks in autoQueueNPCsLocked**

In `internal/gameserver/combat_handler.go`, locate `autoQueueNPCsLocked` at line 1914.

After the existing NPC ability cooldown decrement loop (which ends around line 1930), add a new block before the `// Fetch room once for NPC auto-cover checks.` comment:

```go
// Decrement zone effect cooldowns and apply room effects to living players.
if h.mentalStateMgr != nil && h.worldMgr != nil {
    if room, ok := h.worldMgr.GetRoom(cbt.RoomID); ok && len(room.Effects) > 0 {
        for _, c := range cbt.Combatants {
            if c.Kind != combat.KindPlayer || c.IsDead() {
                continue
            }
            sess, ok := h.sessions.GetPlayer(c.ID)
            if !ok {
                continue
            }
            // Decrement all zone effect cooldowns for this player (all keys,
            // regardless of current room — prevents gaming immunity by room-swapping).
            // Clamp to 0 to avoid underflow.
            for k := range sess.ZoneEffectCooldowns {
                if sess.ZoneEffectCooldowns[k] > 0 {
                    sess.ZoneEffectCooldowns[k]--
                }
                // Clamp: value is always >= 0.
                if sess.ZoneEffectCooldowns[k] < 0 {
                    sess.ZoneEffectCooldowns[k] = 0
                }
            }
            // Check each room effect.
            for _, effect := range room.Effects {
                key := cbt.RoomID + ":" + effect.Track
                if sess.ZoneEffectCooldowns[key] > 0 {
                    continue // immune
                }
                track, trackOK := abilityTrack(effect.Track)
                sev, sevOK := abilitySeverity(effect.Severity)
                if !trackOK || !sevOK {
                    continue
                }
                // Binary Will save: d20 + GritMod vs BaseDC (no proficiency bonus).
                gritMod := combat.AbilityMod(sess.Abilities.Grit)
                roll := h.dice.Src().Intn(20) + 1
                total := roll + gritMod
                if total < effect.BaseDC {
                    // Failed save — apply trigger; no cooldown set.
                    changes := h.mentalStateMgr.ApplyTrigger(c.ID, track, sev)
                    h.applyMentalStateChanges(c.ID, changes)
                } else {
                    // Successful save — set cooldown immunity.
                    if sess.ZoneEffectCooldowns == nil {
                        sess.ZoneEffectCooldowns = make(map[string]int64)
                    }
                    sess.ZoneEffectCooldowns[key] = int64(effect.CooldownRounds)
                }
            }
        }
    }
}
```

**Note on imports:** `combat.AbilityMod` is already used in `buildPlayerCombatant`. No new imports needed for the combat package. `h.dice.Src()` is the dice source. If `h.worldMgr` field does not exist on `CombatHandler`, check the struct definition at lines 40-80 of `combat_handler.go`. If missing, it must be added.

Check current `CombatHandler` struct fields:
```bash
grep -n "worldMgr\|world\.Manager\|WorldManager" internal/gameserver/combat_handler.go | head -10
```

If `worldMgr` is not present, add it to the `CombatHandler` struct:
```go
worldMgr world.WorldManager // optional; used for zone effect checks
```
And wire it in `NewCombatHandler` — check whether worldMgr is already a parameter by reading lines 88-110.

- [ ] **Step 3: Run tests**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestZoneEffect_Combat -v -count=1`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go \
        internal/gameserver/grpc_service_zone_effect_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): apply zone effect triggers to players in autoQueueNPCsLocked

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Out-of-combat zone effects in handleMove

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_zone_effect_test.go` (append)

**Context:**
- `handleMove` is at `grpc_service.go:1285`. Uses `s.world.GetRoom(...)`.
- `s.mentalStateMgr` is the mental state manager (may be nil).
- `s.dice` is the dice roller (may be nil in tests — guard it).
- `abilityTrack(s string) (mentalstate.Track, bool)` and `abilitySeverity(s string) (mentalstate.Severity, bool)` are package-level functions in `combat_handler.go` — reuse them directly.
- `StateChange.Message` is the narrative field (not `.Narrative`).
- `time.Now().Unix()` is used inline for the timestamp (no `timeNow` indirection needed).
- The test world (`testWorldAndSession`) has room_a → room_b (North exit). The `addGroupPlayer` helper exists in `grpc_service_group_test.go`.
- To make tests controllable, extract zone effect logic into `applyRoomEffectsOnEntry` (a method on `GameServiceServer`) and test it directly.

- [ ] **Step 1: Write failing tests (append to existing test file)**

Append to `internal/gameserver/grpc_service_zone_effect_test.go`:

```go
// newMentalStateSvc creates a GameServiceServer with a real mental state manager
// and a fixed dice source for controllable zone effect tests.
//
// NewGameServiceServer parameter positions (37 total):
//   1:worldMgr 2:sessMgr 3:cmdRegistry 4:worldHandler 5:chatHandler 6:logger
//   7:charSaver 8:diceRoller 9:npcHandler 10:npcMgr 11:combatHandler 12:scriptMgr
//   13:respawnMgr 14:floorMgr 15:roomEquipMgr 16:automapRepo 17:invRegistry 18:accountAdmin
//   19:clock 20:jobRegistry 21:condRegistry 22:loadoutsDir
//   23:allSkills 24:characterSkillsRepo 25:characterProficienciesRepo
//   26:allFeats 27:featRegistry 28:characterFeatsRepo
//   29:allClassFeatures 30:classFeatureRegistry 31:characterClassFeaturesRepo
//   32:featureChoicesRepo 33:charAbilityBoostsRepo 34:archetypes 35:regions
//   36:mentalStateMgr 37:actionH
func newMentalStateSvc(t *testing.T, diceVal int) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	src := &fixedDiceSource{val: diceVal}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	mentalMgr := mentalstate.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,                                          // 1-2
		command.DefaultRegistry(),                                   // 3
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil), // 4
		NewChatHandler(sessMgr),                                    // 5
		zap.NewNop(),                                               // 6: logger
		nil, roller, nil, npcMgr, combatHandler, nil,              // 7-12
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",            // 13-22
		nil, nil, nil,                                              // 23-25
		nil, nil, nil,                                              // 26-28
		nil, nil, nil, nil, nil, nil, nil,                          // 29-35
		mentalMgr, nil,                                             // 36-37
	)
	return svc, sessMgr
}

// TestZoneEffect_Move_FailedSave_AppliesTrigger verifies REQ-T5:
// applyRoomEffectsOnEntry on a room with a fear effect; save fails → mental state set.
func TestZoneEffect_Move_FailedSave_AppliesTrigger(t *testing.T) {
	// diceVal = 0 → roll = 1, GritMod = 0 → total = 1 < BaseDC 12 → fail.
	svc, sessMgr := newMentalStateSvc(t, 0)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{{
			Track: "fear", Severity: "mild",
			BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
		}},
	}

	now := int64(0)
	svc.applyRoomEffectsOnEntry(sess, "p1", room, now)

	// Cooldown must NOT be set on a failed save.
	if sess.ZoneEffectCooldowns != nil {
		assert.Zero(t, sess.ZoneEffectCooldowns["room_b:fear"], "failed save must not set cooldown")
	}
	// Player should have a fear condition applied.
	require.NotNil(t, svc.mentalStateMgr)
	assert.Equal(t, mentalstate.SeverityMild, svc.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackFear))
}

// TestZoneEffect_Move_SuccessfulSave_SetsCooldown verifies REQ-T6:
// applyRoomEffectsOnEntry on a room with a fear effect; save succeeds → cooldown set.
func TestZoneEffect_Move_SuccessfulSave_SetsCooldown(t *testing.T) {
	// diceVal = 19 → roll = 20, GritMod = 0 → total = 20 >= BaseDC 12 → success.
	svc, sessMgr := newMentalStateSvc(t, 19)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{{
			Track: "fear", Severity: "mild",
			BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
		}},
	}

	now := int64(1000000)
	svc.applyRoomEffectsOnEntry(sess, "p1", room, now)

	require.NotNil(t, sess.ZoneEffectCooldowns)
	expected := now + int64(5)*60
	assert.Equal(t, expected, sess.ZoneEffectCooldowns["room_b:fear"],
		"successful save must set cooldown to now + CooldownMinutes*60")
	// No mental state condition should be applied on a successful save.
	assert.Equal(t, mentalstate.SeverityNone, svc.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackFear))
}

// TestZoneEffect_Move_TwoEffects_IndependentCooldowns verifies REQ-T10:
// two effects in the same room have independent keys and cooldowns.
func TestZoneEffect_Move_TwoEffects_IndependentCooldowns(t *testing.T) {
	// High roll → both saves succeed → both cooldowns set.
	svc, sessMgr := newMentalStateSvc(t, 19)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	room := &world.Room{
		ID: "room_b",
		Effects: []world.RoomEffect{
			{Track: "despair", Severity: "mild", BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5},
			{Track: "delirium", Severity: "mild", BaseDC: 12, CooldownRounds: 4, CooldownMinutes: 10},
		},
	}

	now := int64(1000000)
	svc.applyRoomEffectsOnEntry(sess, "p1", room, now)

	require.NotNil(t, sess.ZoneEffectCooldowns)
	assert.Equal(t, now+5*60, sess.ZoneEffectCooldowns["room_b:despair"], "despair cooldown")
	assert.Equal(t, now+10*60, sess.ZoneEffectCooldowns["room_b:delirium"], "delirium cooldown")
}

// TestZoneEffect_Move_NoEffects_NoOp verifies REQ-T11:
// a room with no effects is a no-op; ZoneEffectCooldowns unchanged.
func TestZoneEffect_Move_NoEffects_NoOp(t *testing.T) {
	svc, sessMgr := newMentalStateSvc(t, 0)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	room := &world.Room{ID: "room_b", Effects: nil}
	svc.applyRoomEffectsOnEntry(sess, "p1", room, 0)

	assert.Nil(t, sess.ZoneEffectCooldowns, "no effects → ZoneEffectCooldowns stays nil")
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestZoneEffect_Move -v`
Expected: FAIL — `applyRoomEffectsOnEntry` not defined.

**Note:** The `newMentalStateSvc` helper passes `roller` and `mentalMgr` as the last two parameters of `NewGameServiceServer`. Verify the exact parameter count by running:
```bash
grep -n "func NewGameServiceServer" internal/gameserver/grpc_service.go
```
and counting parameters. Adjust the `nil` padding in the call above accordingly.

- [ ] **Step 2: Add applyRoomEffectsOnEntry method and wire into handleMove**

In `internal/gameserver/grpc_service.go`, add a new method (near the `handleMove` function):

```go
// applyRoomEffectsOnEntry applies all zone effects for the given room to sess.
//
// Precondition: sess and room must not be nil; uid is the player's UID; now is the current
//   Unix timestamp in seconds (used for cooldown expiry).
// Postcondition: For each effect in room.Effects, if the player is not immune (cooldown > now),
//   a binary Will save is resolved (d20 + GritMod vs BaseDC, no proficiency).
//   Failed save: ApplyTrigger called; narrative pushed to player stream; no cooldown set.
//   Successful save: ZoneEffectCooldowns[roomID:track] set to now + CooldownMinutes*60.
func (s *GameServiceServer) applyRoomEffectsOnEntry(
	sess *session.PlayerSession, uid string, room *world.Room, now int64,
) {
	if s.mentalStateMgr == nil || len(room.Effects) == 0 {
		return
	}
	for _, effect := range room.Effects {
		key := room.ID + ":" + effect.Track
		if sess.ZoneEffectCooldowns != nil && sess.ZoneEffectCooldowns[key] > now {
			continue // immune
		}
		track, trackOK := abilityTrack(effect.Track)
		sev, sevOK := abilitySeverity(effect.Severity)
		if !trackOK || !sevOK {
			continue
		}
		gritMod := combat.AbilityMod(sess.Abilities.Grit)
		var roll int
		if s.dice != nil {
			roll = s.dice.Src().Intn(20) + 1
		} else {
			roll = 10 // fallback for nil dice (should not happen in production)
		}
		total := roll + gritMod
		if total < effect.BaseDC {
			// Failed save — apply trigger; push narrative to player stream.
			changes := s.mentalStateMgr.ApplyTrigger(uid, track, sev)
			for _, ch := range changes {
				if ch.Message != "" && sess.Entity != nil {
					evt := messageEvent(ch.Message)
					if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
						_ = sess.Entity.Push(data)
					}
				}
			}
		} else {
			// Successful save — set timestamp cooldown.
			if sess.ZoneEffectCooldowns == nil {
				sess.ZoneEffectCooldowns = make(map[string]int64)
			}
			sess.ZoneEffectCooldowns[key] = now + int64(effect.CooldownMinutes)*60
		}
	}
}
```

In `handleMove`, after the existing skill check block (lines ~1338–1365) and before the automap block (starting around line ~1386), add:

```go
// Apply zone effects for the new room (out-of-combat path).
if newRoom, zoneOK := s.world.GetRoom(result.View.RoomId); zoneOK {
    s.applyRoomEffectsOnEntry(sess, uid, newRoom, time.Now().Unix())
}
```

**Import notes:**
- `"time"` is likely already imported; if not, add it.
- `combat.AbilityMod` — add `"github.com/cory-johannsen/mud/internal/game/combat"` to imports if not already present (check with `grep -n "\"github.com/cory-johannsen/mud/internal/game/combat\"" internal/gameserver/grpc_service.go`).

- [ ] **Step 3: Build check**

Run: `cd /home/cjohannsen/src/mud && go build ./... 2>&1`
Expected: No errors.

- [ ] **Step 4: Run full test suite**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1 -timeout=120s 2>&1 | tail -20`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_zone_effect_test.go
git commit -m "$(cat <<'EOF'
feat(gameserver): apply zone effect triggers on room entry in handleMove

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Chunk 3: Content

### Task 5: Add zone effects to existing room content

**Files:**
- Modify: multiple `content/zones/*.yaml` files

**Goal:** Apply effects to rooms where they make thematic sense. Use the table below as a guide. The implementer should read each zone file and find the stated rooms by their IDs.

| Zone File | Room ID | Track | Severity | BaseDC | CooldownRounds | CooldownMinutes | Rationale |
|-----------|---------|-------|----------|--------|----------------|-----------------|-----------|
| `content/zones/felony_flats.yaml` | `flats_powell_overpass` | despair | mild | 11 | 3 | 5 | Chemical atmosphere, desolate overpass |
| `content/zones/felony_flats.yaml` | `flats_motel_courtyard` | fear | mild | 10 | 3 | 5 | Dangerous, hostile territory |
| `content/zones/battleground.yaml` | First room with `atmosphere: hostile` or `propaganda` | rage | mild | 12 | 3 | 5 | Oppressive ideological atmosphere |

Find the exact room IDs by searching: `grep -n "    id:\|atmosphere:\|propaganda" content/zones/battleground.yaml | head -20`

For each room identified, add under its `properties:` section (or after `skill_checks: []` if present):

```yaml
effects:
  - track: despair
    severity: mild
    base_dc: 11
    cooldown_rounds: 3
    cooldown_minutes: 5
```

Adjust track/severity/DC per the table above for each room.

- [ ] **Step 1: Identify exact room IDs and add effects**

For each target zone file:
1. Open the file
2. Find the target room by its ID
3. Add the `effects:` block after `skill_checks:` (or after the last property block if no skill_checks)
4. Use the values from the table

- [ ] **Step 2: Verify zone files parse correctly**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v -count=1`
Expected: All pass (loader tests will re-parse all zone files if they exist).

Also run: `cd /home/cjohannsen/src/mud && go build ./... 2>&1`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud
git add content/zones/
git commit -m "$(cat <<'EOF'
content: add zone effect triggers to atmospheric rooms in felony flats and battleground

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: Run full suite**

Run: `cd /home/cjohannsen/src/mud && go test ./... -count=1 -timeout=120s 2>&1 | tail -20`
Expected: All pass.

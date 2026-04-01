# Plan: Multi-Zone Roving NPCs

**Feature:** `docs/features/roving-npcs.md`
**Date:** 2026-03-30
**Branch:** `feature/roving-npcs`

---

## Context

NPCs currently patrol within a single zone using a BFS wander radius from a home room.
This plan adds roving NPCs that follow a configured multi-zone route and optionally deviate
to explore. The first roving NPC is Dayton James Weber — The Cornhole Quad (boss-tier combat).

**Key existing patterns to follow:**
- `internal/game/npc/template.go` — Template struct; add `Roving *RovingConfig` field
- `internal/game/npc/instance.go` — Instance struct; add roving tracking fields
- `npc.Manager.Move(id, newRoomID)` — existing thread-safe room relocation
- Boss ability system (`BossAbility`, `BossAbilityEffect`) in `internal/game/npc/boss_ability.go`
- Zone tick pattern: `StartZoneTicks` / `StartWantedDecayHook` in `grpc_service.go`
- `s.pushEventToUID(uid, evt)` to send events to specific players
- `s.sessions.AllPlayers()` to iterate all online players
- `s.world.GetRoom(roomID)` to look up room data (exits, zone)
- `content/ai/*.yaml` for HTN domain format
- `content/npcs/yo_yo_master.yaml` as a nearby boss NPC example

---

## Task 1: Template + Instance data model

**Files:**
- `internal/game/npc/template.go`
- `internal/game/npc/instance.go`

**Spec:** REQ-ROV-1 through REQ-ROV-5

### Template changes

Add after the `Immobile` field:

```go
// RovingConfig holds multi-zone route configuration for roving NPCs.
// Precondition: Route must be non-empty if Roving is non-nil.
type RovingConfig struct {
    // Route is an ordered list of room IDs the NPC visits. May span zones.
    Route []string `yaml:"route"`
    // TravelInterval is the duration between room transitions (e.g. "3m"). Default "5m".
    TravelInterval string `yaml:"travel_interval"`
    // ExploreProbability is the chance [0,1] to deviate from route to a random adjacent room.
    ExploreProbability float64 `yaml:"explore_probability"`
}
```

Add field to `Template`:
```go
// Roving, when non-nil, makes this NPC traverse a multi-zone route autonomously.
// Only valid when NPCType == "combat" and Tier == "boss", or NPCType != "combat".
Roving *RovingConfig `yaml:"roving,omitempty"`
```

Add to `Template.Validate()`:
- If `Roving != nil && len(Roving.Route) == 0` → error "roving.route must not be empty"
- If `Roving != nil && NPCType == "combat" && Tier != "boss"` → error "roving combat NPCs must have tier: boss"
- If `Roving != nil && (Roving.ExploreProbability < 0 || Roving.ExploreProbability > 1)` → error

### Instance changes

Add to `Instance`:
```go
// Roving state — only populated when Template.Roving != nil.
RovingRouteIndex int       // current index in RovingConfig.Route
RovingRouteDir   int       // +1 (forward) or -1 (backward)
RovingNextMoveAt time.Time // move when time.Now() >= this
RovingPausedUntil time.Time // pause movement until this time (set on combat entry)
```

In `Manager.Spawn` (or wherever Instance is initialised from Template), initialise:
```go
if tmpl.Roving != nil {
    inst.RovingRouteDir = 1
    inst.RovingRouteIndex = 0
    inst.RovingNextMoveAt = time.Now().Add(parseTravelInterval(tmpl.Roving.TravelInterval))
}
```

Add helper `parseTravelInterval(s string) time.Duration` — parses duration string, defaults 5m.

### Tests

Property test in `internal/game/npc/template_test.go` (or new `roving_test.go`):
- `TestProperty_RovingConfig_Validate_RouteRequired` — RovingConfig with empty Route → validation error
- `TestProperty_RovingConfig_Validate_CombatMustBeBoss` — combat NPC with roving but non-boss tier → validation error
- `TestProperty_RovingConfig_Validate_ProbabilityRange` — explore_probability outside [0,1] → error

---

## Task 2: RovingManager

**Files:**
- `internal/game/npc/roving.go` (new)
- `internal/game/npc/roving_test.go` (new)

**Spec:** REQ-ROV-6 through REQ-ROV-12

### Interface

```go
package npc

import (
    "context"
    "math/rand"
    "time"
)

// WorldRoomReader is the subset of world.Manager needed by RovingManager.
type WorldRoomReader interface {
    GetRoom(id string) (*world.Room, bool)
}

// RovingMoveFunc is called when the manager decides to move an NPC.
// It moves the NPC to newRoomID and returns the direction string used for
// notification ("" if unknown). The caller broadcasts notifications.
type RovingMoveFunc func(instID, fromRoomID, toRoomID string)

// RovingManager drives autonomous multi-zone NPC movement.
type RovingManager struct {
    mu       sync.Mutex
    npcMgr   *Manager
    world    WorldRoomReader
    onMove   RovingMoveFunc
    tickRate time.Duration
    stop     chan struct{}
}

func NewRovingManager(npcMgr *Manager, world WorldRoomReader, onMove RovingMoveFunc) *RovingManager

// Register adds a roving NPC to tracking. No-op if inst.Template.Roving == nil.
func (rm *RovingManager) Register(inst *Instance, tmpl *Template)

// Unregister removes an NPC (e.g. on death).
func (rm *RovingManager) Unregister(instID string)

// PauseFor sets PausedUntil = now + d for the given instance (called on combat entry).
func (rm *RovingManager) PauseFor(instID string, d time.Duration)

// Start begins the background tick goroutine. Blocks until ctx is cancelled.
func (rm *RovingManager) Start(ctx context.Context)
```

### Tick logic (private)

```
func (rm *RovingManager) tick(now time.Time):
  for each tracked entry (instID, inst, tmpl, route):
    if now < inst.RovingNextMoveAt: continue
    if now < inst.RovingPausedUntil: continue
    inst = rm.npcMgr.Instance(instID)  // re-fetch live state
    if inst == nil: rm.Unregister(instID); continue

    currentRoom, ok := rm.world.GetRoom(inst.RoomID)
    if !ok: continue

    var nextRoomID string
    if rand.Float64() < tmpl.Roving.ExploreProbability && len(currentRoom.Exits) > 0:
      // deviate: pick random exit
      exit := currentRoom.Exits[rand.Intn(len(currentRoom.Exits))]
      nextRoomID = exit.TargetRoom
    else:
      // follow route
      nextIdx := inst.RovingRouteIndex + inst.RovingRouteDir
      if nextIdx >= len(route):
        inst.RovingRouteDir = -1
        nextIdx = len(route) - 2  // bounce back
        if nextIdx < 0: nextIdx = 0
      if nextIdx < 0:
        inst.RovingRouteDir = +1
        nextIdx = 1
        if nextIdx >= len(route): nextIdx = 0
      nextRoomID = route[nextIdx]
      inst.RovingRouteIndex = nextIdx

    interval := parseTravelInterval(tmpl.Roving.TravelInterval)
    inst.RovingNextMoveAt = now.Add(interval)

    fromRoom := inst.RoomID
    rm.npcMgr.Move(instID, nextRoomID)
    rm.onMove(instID, fromRoom, nextRoomID)
```

### Tests

- `TestProperty_RovingManager_RouteFollowing` — NPC with 3-room route advances in order and bounces
- `TestProperty_RovingManager_ExploreDeviation` — with explore_probability=1.0, NPC always picks a random adjacent room
- `TestProperty_RovingManager_PauseRespected` — PauseFor blocks movement until pause expires
- `TestProperty_RovingManager_UnregisterOnDeath` — after Unregister, NPC no longer ticked

---

## Task 3: GameServiceServer integration

**Files:**
- `internal/gameserver/grpc_service.go`
- `internal/gameserver/grpc_service_roving.go` (new)

**Spec:** REQ-ROV-11, REQ-ROV-13, REQ-ROV-14

### New file: grpc_service_roving.go

```go
package gameserver

// startRovingNPCs initialises the RovingManager, registers all instances
// from loaded templates with Roving != nil, and starts the background goroutine.
func (s *GameServiceServer) startRovingNPCs(ctx context.Context)

// stopRovingNPCs stops the roving goroutine.
func (s *GameServiceServer) stopRovingNPCs()

// onRovingMove is the RovingMoveFunc callback. It:
//   1. Finds players in fromRoomID and sends "X leaves." MessageEvent
//   2. Finds players in toRoomID and sends "X arrives." MessageEvent
func (s *GameServiceServer) onRovingMove(instID, fromRoomID, toRoomID string)

// pauseRovingOnCombat is wired to NPC OnDamageTaken: calls rovingMgr.PauseFor(instID, 10*time.Minute).
func (s *GameServiceServer) pauseRovingOnCombat(instID string)
```

### Wire-up in grpc_service.go

In `NewGameServiceServer` (or a post-init Start method): construct `RovingManager`, iterate
`npcMgr.AllInstances()`, register those with `Template.Roving != nil`.

Add `startRovingNPCs` / `stopRovingNPCs` to the server lifecycle alongside `StartZoneTicks`.

In NPC `OnDamageTaken` callback site: call `pauseRovingOnCombat(instID)`.

In NPC death handler: call `rovingMgr.Unregister(instID)`.

### Notification format

- Leave: `"Dayton James Weber — The Cornhole Quad leaves to the north."`
- Arrive: `"Dayton James Weber — The Cornhole Quad arrives from the south."`
- Direction detected by comparing `fromRoom.Exits` to find which exit targets `toRoomID`.
- If no matching exit found (e.g. teleport): `"<name> appears in the room."`

### Tests

- `TestRovingMove_NotifiesPlayersInOriginRoom`
- `TestRovingMove_NotifiesPlayersInDestinationRoom`
- `TestRovingMove_NoNotificationWhenRoomEmpty`

---

## Task 4: Content — cornhole sack item + AI domain

**Files:**
- `content/items/cornhole_sack.yaml` (new)
- `content/ai/cornhole_quad_combat.yaml` (new)

**Spec:** REQ-ROV-16, REQ-ROV-22, REQ-ROV-23

### content/items/cornhole_sack.yaml

```yaml
id: cornhole_sack
name: Cornhole Sack
description: A canvas bag filled with something that smells faintly of shrimp and gunpowder.
  Handle with care. Probably.
kind: misc
weight: 0.3
value: 12
tags:
  - cornhole
  - throwable
```

### content/ai/cornhole_quad_combat.yaml

HTN domain with:
- `task: combat_turn` with methods:
  - `method: throw_sack` — precondition `in_combat`, operator `boss_ability` with `ability_id: exploding_cornhole_toss`
  - `method: shoot` — precondition `in_combat`, operator `attack` targeting `nearest_enemy`
  - `method: taunt_and_shoot` — precondition `in_combat`, operators `say` then `attack`
- `operators`:
  - `attack`, target `nearest_enemy`, ap_cost 2
  - `say`, strings: (taunts from NPC template), cooldown "15s"
  - `boss_ability`, ability_id `exploding_cornhole_toss`

---

## Task 5: Content — The Cornhole Quad NPC + feature docs

**Files:**
- `content/npcs/dayton_james_weber.yaml` (new)
- `docs/features/roving-npcs.md` (already written — update status to `done` when shipped)
- `docs/features/index.yaml` (update status)

**Spec:** REQ-ROV-17 through REQ-ROV-21

### content/npcs/dayton_james_weber.yaml

```yaml
id: dayton_james_weber
name: "Dayton James Weber — The Cornhole Quad"
description: >
  A quadruple amputee of remarkable self-determination, Dayton cruises the Portland
  streets in a heavily modified Isuzu Rodeo with a custom firing rig bolted to the
  dashboard. Four limbs gone, zero fucks remaining. He navigates by biting the wheel
  and fires a 9mm with an adaptive trigger system. His trunk is full of cornhole sacks
  wired to explode on impact. "The sport builds character," he tells anyone who survives
  long enough to hear it.
type: human
gender: male
level: 10
max_hp: 160
ac: 17
awareness: 12
tier: boss
npc_type: combat
ai_domain: cornhole_quad_combat
respawn_delay: "60m"
abilities:
  brutality: 14
  quickness: 16
  grit: 18
  reasoning: 12
  savvy: 14
  flair: 18
weapon:
  - id: glock_17
    weight: 5
armor:
  - id: light_jacket
    weight: 3
boss_abilities:
  - id: exploding_cornhole_toss
    name: "Exploding Cornhole Toss"
    trigger: on_damage_taken
    cooldown: "3r"
    effect:
      aoe_damage_expr: "2d8"
      aoe_condition: nausea
loot:
  currency:
    min: 200
    max: 800
  items:
    - item: cornhole_sack
      chance: 0.75
      min_qty: 1
      max_qty: 3
    - item: glock_17
      chance: 0.40
      min_qty: 1
      max_qty: 1
    - item: tactical_vest
      chance: 0.25
      min_qty: 1
      max_qty: 1
roving:
  route:
    - sei_holgate_blvd
    - csp_main_entrance
    - csp_shrimp_plaza
    - csp_amphitheater
    - csp_fountain
    - csp_main_entrance
    - sei_holgate_blvd
  travel_interval: "4m"
  explore_probability: 0.10
taunts:
  - "The cornhole never misses. Only amateurs miss."
  - "Four limbs, zero regrets, infinite cornhole."
  - "You think this is a game? It IS a game. That's the point."
  - "I've been through worse than you. My arms went through worse than you."
  - "CORNHOLE SUPREMACY."
  - "Adaptive rig. Adaptive mindset. You should try it."
  - "This Rodeo has seen things. So has this sack."
  - "The bag hits harder when you're crying."
  - "Most people quit when they lose one limb. I'm just getting started."
  - "You know what the Isuzu Rodeo never had? Mercy. Neither do I."
```

---

## Commit Strategy

Each task should be committed independently:
1. `feat(npc): add RovingConfig to Template and roving state to Instance`
2. `feat(npc): add RovingManager background movement engine`
3. `feat(gameserver): wire RovingManager + room entry/exit notifications`
4. `content: add cornhole_sack item and cornhole_quad_combat AI domain`
5. `content: add Dayton James Weber roving boss NPC + feature docs`

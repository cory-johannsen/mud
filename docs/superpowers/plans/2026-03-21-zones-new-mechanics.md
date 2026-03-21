# Zones-New Phase 1: Mechanics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add zone-level effect inheritance, typed terrain conditions, seduction command, NPC gender, charmed condition, and 17 new condition YAML files — eliminating the hardcoded `RoomEffect.Track` enum.

**Architecture:** Zone effects propagate to rooms at load time via `convertYAMLZone`. `RoomEffect.Track` becomes a free condition ID resolved via `condRegistry` at runtime. Terrain types are `ConditionDef` entries with `MoveAPCost`/`SkillPenalties` fields. Seduction is a player command handler. Charmed NPC saves fire in the zone tick using `combat.ResolveSave("cool", ...)`.

**Tech Stack:** Go, yaml.v3, `internal/game/condition`, `internal/game/world`, `internal/game/npc`, `internal/gameserver/grpc_service.go`, `internal/gameserver/zone_tick.go`, `content/conditions/`

---

## Implementation Notes

**Critical divergences from spec language:**

- **REQ-ZN-7 flair check**: `sess.Skills` is `map[string]string` (rank strings). "Flair skill rank > 0" → `sess.Skills["flair"] != ""`.
- **REQ-ZN-2 effect application**: `applyRoomEffectsOnEntry` currently calls `mentalStateMgr.ApplyTrigger` via `abilityTrack`/`abilitySeverity`. Replace with `condRegistry.Get(effect.Track)` → `sess.Conditions.Apply`.
- **REQ-ZN-4 legacy tracks**: YAML files use `track: rage` etc. Add condition definitions with IDs `"rage"`, `"despair"`, `"delirium"`, `"fear"` so these remain valid without modifying zone YAMLs.
- **REQ-ZN-9 charmed save**: "Savvy saving throw" maps to `combat.ResolveSave("cool", cbt, 15, src)` (Cool = Will-equivalent; uses `SavvyMod` + `CoolRank` proficiency).
- **"seduce" is a player command** (not HTN operator — HTN is the NPC planning system). Implement as a command handler using `npcMgr.FindInRoom(roomID, target)`.
- **Respawn creates a new Instance**: `RespawnManager` calls `mgr.Spawn(tmpl, roomID)` which calls `NewInstanceWithResolver` — so `Conditions` and `SeductionRejected` are nil by default on every respawn. No explicit reset needed.
- **`SkillPenalties` sign convention**: Negative values = penalty (e.g., `acrobatics: -2`); positive values = bonus (e.g., `stealth: 1` for dense vegetation cover). Consistent with the existing `AttackBonus`/`AttackPenalty` dual-field pattern.
- **`npc.Instance` has no `Conditions` field** — must be added (Task 8a below) before charmed logic can compile.
- **`npc.Manager.AllInstances()`** returns all live NPCs; filter by zone via `world.Manager.GetZone(zoneID)`.
- **`npc.Manager.FindInRoom(roomID, target)`** is the correct method for locating NPCs by name.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/game/condition/definition.go` | Modify | Add `MoveAPCost`, `SkillPenalties` to `ConditionDef` |
| `internal/game/condition/definition_test.go` | Modify | Tests for new fields; property-based test for all YAML files |
| `internal/game/world/model.go` | Modify | Add `ZoneEffects []RoomEffect` to `Zone`; add `ValidateWithRegistry` |
| `internal/game/world/model_test.go` | Modify | Tests for propagation and ValidateWithRegistry |
| `internal/game/world/loader.go` | Modify | Add `ZoneEffects` to `yamlZone`; propagate in `convertYAMLZone` |
| `internal/game/world/loader_test.go` | Modify | Test zone effect propagation from YAML |
| `internal/game/npc/instance.go` | Modify | Add `Conditions *condition.ActiveSet`, `Gender string`, `SeductionRejected map[string]bool`; initialize in `NewInstanceWithResolver` |
| `internal/game/npc/template.go` | Modify | Add `Gender string` |
| `internal/game/npc/npc_test.go` | Modify | Tests for Gender propagation and Conditions initialization |
| `internal/gameserver/grpc_service.go` | Modify | Replace `applyRoomEffectsOnEntry` Track enum; add `handleSeduce`; replace terrain check; wire `ValidateWithRegistry` |
| `internal/gameserver/grpc_service_zone_effect_test.go` | Modify | Tests for condition registry-based effect application |
| `internal/gameserver/grpc_service_seduce_test.go` | Create | Tests for `handleSeduce` |
| `internal/gameserver/grpc_service_move_test.go` | Modify | Tests for terrain condition AP stacking |
| `internal/gameserver/zone_tick.go` | Modify | Add charmed NPC save at end of tick |
| `internal/gameserver/zone_tick_test.go` | Modify | Tests for charmed NPC save |
| `content/conditions/rage.yaml` etc. (17 files) | Create | Condition YAML definitions |

---

## Task 1: Add `MoveAPCost` and `SkillPenalties` to `ConditionDef`

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/game/condition/definition_test.go
func TestConditionDef_NewFields_ParsedFromYAML(t *testing.T) {
    dir := t.TempDir()
    yaml := []byte(`
id: terrain_mud
name: Muddy Ground
description: Thick mud.
move_ap_cost: 1
skill_penalties:
  acrobatics: -2
`)
    require.NoError(t, os.WriteFile(filepath.Join(dir, "terrain_mud.yaml"), yaml, 0644))
    reg, err := LoadDirectory(dir)
    require.NoError(t, err)
    def, ok := reg.Get("terrain_mud")
    require.True(t, ok)
    assert.Equal(t, 1, def.MoveAPCost)
    assert.Equal(t, map[string]int{"acrobatics": -2}, def.SkillPenalties)
}

func TestConditionDef_ZeroMoveAPCost_DefaultsToZero(t *testing.T) {
    dir := t.TempDir()
    yaml := []byte(`id: rage\nname: Rage\ndescription: Anger.\n`)
    require.NoError(t, os.WriteFile(filepath.Join(dir, "rage.yaml"), yaml, 0644))
    reg, err := LoadDirectory(dir)
    require.NoError(t, err)
    def, ok := reg.Get("rage")
    require.True(t, ok)
    assert.Equal(t, 0, def.MoveAPCost)
    assert.Nil(t, def.SkillPenalties)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestConditionDef_NewFields -v
```
Expected: FAIL — unknown field `move_ap_cost`.

- [ ] **Step 3: Add fields to `ConditionDef`**

In `internal/game/condition/definition.go`, add after `SkillPenalty int`:

```go
// MoveAPCost is the extra AP cost deducted when leaving a room bearing this terrain condition.
// Zero means no movement AP penalty. Only meaningful for terrain_ conditions.
MoveAPCost int `yaml:"move_ap_cost"`
// SkillPenalties maps canonical skill IDs (lowercase, underscore-separated) to modifiers.
// Negative values are penalties; positive values are bonuses. Nil means no skill effects.
SkillPenalties map[string]int `yaml:"skill_penalties"`
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/condition/definition.go internal/game/condition/definition_test.go
git commit -m "feat(condition): add MoveAPCost and SkillPenalties to ConditionDef (REQ-ZN-11)"
```

---

## Task 2: Add property-based test for condition YAML loading

**Files:**
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1: Write property-based test**

```go
// Tests that every condition file in content/conditions/ loads with valid required fields.
func TestProperty_AllContentConditions_ValidRequired(t *testing.T) {
    reg, err := LoadDirectory("../../../content/conditions")
    // content/conditions/ may not exist yet; skip if absent.
    if os.IsNotExist(err) {
        t.Skip("content/conditions not yet populated")
    }
    require.NoError(t, err)
    for _, def := range reg.All() {
        t.Run(def.ID, func(t *testing.T) {
            assert.NotEmpty(t, def.ID, "ID must not be empty")
            assert.NotEmpty(t, def.Name, "Name must not be empty")
            validDurationTypes := map[string]bool{
                "rounds": true, "until_save": true, "permanent": true, "": true,
            }
            assert.True(t, validDurationTypes[def.DurationType],
                "DurationType %q is not a valid value", def.DurationType)
        })
    }
}
```

- [ ] **Step 2: Run test (expect skip until Task 6 adds YAML files)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestProperty_AllContent -v
```
Expected: SKIP (directory absent) or PASS. Will fully exercise after Task 6.

- [ ] **Step 3: Commit**

```bash
git add internal/game/condition/definition_test.go
git commit -m "test(condition): add property-based test for content/conditions/ YAML validity (SWENG-5a)"
```

---

## Task 3: Add `Zone.ValidateWithRegistry`

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/model_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/world/model_test.go

func TestZone_ValidateWithRegistry_UnknownTrack(t *testing.T) {
    zone := validTestZone()
    room := zone.Rooms[zone.StartRoom]
    room.Effects = []RoomEffect{{Track: "ghost_zone", BaseDC: 12}}
    reg := condition.NewRegistry()
    err := zone.ValidateWithRegistry(reg, nil)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "ghost_zone")
}

func TestZone_ValidateWithRegistry_KnownTrackOK(t *testing.T) {
    zone := validTestZone()
    room := zone.Rooms[zone.StartRoom]
    room.Effects = []RoomEffect{{Track: "rage", BaseDC: 12}}
    reg := condition.NewRegistry()
    reg.Register(&condition.ConditionDef{ID: "rage", Name: "Rage"})
    assert.NoError(t, zone.ValidateWithRegistry(reg, nil))
}

func TestZone_ValidateWithRegistry_ZoneEffectUnknownTrack(t *testing.T) {
    zone := validTestZone()
    zone.ZoneEffects = []RoomEffect{{Track: "horror", BaseDC: 10}}
    reg := condition.NewRegistry()
    err := zone.ValidateWithRegistry(reg, nil)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "horror")
}

func TestZone_ValidateWithRegistry_UnknownSkillPenaltyKey(t *testing.T) {
    zone := validTestZone()
    room := zone.Rooms[zone.StartRoom]
    room.Effects = []RoomEffect{{Track: "terrain_mud", BaseDC: 10}}
    reg := condition.NewRegistry()
    reg.Register(&condition.ConditionDef{
        ID: "terrain_mud", Name: "Mud",
        SkillPenalties: map[string]int{"typo_skill": -1},
    })
    skills := []*ruleset.Skill{{ID: "acrobatics"}, {ID: "flair"}}
    err := zone.ValidateWithRegistry(reg, skills)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "typo_skill")
}

func TestZone_ValidateWithRegistry_ValidSkillPenaltyKey(t *testing.T) {
    zone := validTestZone()
    room := zone.Rooms[zone.StartRoom]
    room.Effects = []RoomEffect{{Track: "terrain_mud", BaseDC: 10}}
    reg := condition.NewRegistry()
    reg.Register(&condition.ConditionDef{
        ID: "terrain_mud", Name: "Mud",
        SkillPenalties: map[string]int{"acrobatics": -2},
    })
    skills := []*ruleset.Skill{{ID: "acrobatics"}}
    assert.NoError(t, zone.ValidateWithRegistry(reg, skills))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestZone_ValidateWithRegistry -v
```

- [ ] **Step 3: Implement `ValidateWithRegistry` in `model.go`**

```go
// ValidateWithRegistry checks all RoomEffect.Track values and ConditionDef.SkillPenalties keys
// against the provided registries.
//
// Precondition: condReg must not be nil.
// Postcondition: Returns nil if all tracks and skill keys are known; first error otherwise.
func (z *Zone) ValidateWithRegistry(condReg *condition.Registry, skills []*ruleset.Skill) error {
    skillIDs := make(map[string]bool, len(skills))
    for _, s := range skills {
        skillIDs[s.ID] = true
    }

    checkDef := func(def *condition.ConditionDef, context string) error {
        for k := range def.SkillPenalties {
            if len(skills) > 0 && !skillIDs[k] {
                return fmt.Errorf("%s: condition %q has unknown skill penalty key %q", context, def.ID, k)
            }
        }
        return nil
    }

    checkTrack := func(track, context string) error {
        def, ok := condReg.Get(track)
        if !ok {
            return fmt.Errorf("%s: unknown RoomEffect track %q", context, track)
        }
        return checkDef(def, context)
    }

    for _, effect := range z.ZoneEffects {
        if err := checkTrack(effect.Track, fmt.Sprintf("zone %q zone_effects", z.ID)); err != nil {
            return err
        }
    }
    for _, room := range z.Rooms {
        for _, effect := range room.Effects {
            if err := checkTrack(effect.Track, fmt.Sprintf("zone %q room %q", z.ID, room.ID)); err != nil {
                return err
            }
        }
    }
    return nil
}
```

Add import `"github.com/cory-johannsen/mud/internal/game/condition"` and `"github.com/cory-johannsen/mud/internal/game/ruleset"` if not present.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/world/model.go internal/game/world/model_test.go
git commit -m "feat(world): add Zone.ValidateWithRegistry for track and skill penalty validation (REQ-ZN-3, REQ-ZN-11)"
```

---

## Task 4: Add `ZoneEffects` to `Zone` and propagate to rooms at load time

**Files:**
- Modify: `internal/game/world/model.go` (add `ZoneEffects` field to `Zone`)
- Modify: `internal/game/world/loader.go` (add to `yamlZone`; propagate in `convertYAMLZone`)
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/world/loader_test.go
func TestLoadZoneFromBytes_ZoneEffectsPropagatedToRooms(t *testing.T) {
    data := []byte(`
zone:
  id: test_zone
  name: Test Zone
  description: A test zone.
  start_room: room_a
  danger_level: safe
  zone_effects:
    - track: rage
      severity: mild
      base_dc: 12
      cooldown_minutes: 5
  rooms:
    - id: room_a
      title: Room A
      description: First room.
      map_x: 0
      map_y: 0
`)
    zone, err := LoadZoneFromBytes(data)
    require.NoError(t, err)
    room := zone.Rooms["room_a"]
    require.Len(t, room.Effects, 1)
    assert.Equal(t, "rage", room.Effects[0].Track)
}

func TestLoadZoneFromBytes_RoomEffectsAppendedAfterZoneEffects(t *testing.T) {
    data := []byte(`
zone:
  id: test_zone
  name: Test Zone
  description: A test zone.
  start_room: room_a
  danger_level: safe
  zone_effects:
    - track: rage
      severity: mild
      base_dc: 12
      cooldown_minutes: 5
  rooms:
    - id: room_a
      title: Room A
      description: First room.
      map_x: 0
      map_y: 0
      effects:
        - track: fear
          severity: mild
          base_dc: 10
          cooldown_minutes: 3
`)
    zone, err := LoadZoneFromBytes(data)
    require.NoError(t, err)
    room := zone.Rooms["room_a"]
    require.Len(t, room.Effects, 2)
    assert.Equal(t, "rage", room.Effects[0].Track)  // zone effect first
    assert.Equal(t, "fear", room.Effects[1].Track)  // room effect appended
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run TestLoadZoneFromBytes_ZoneEffects -v
```
Expected: FAIL — `zone_effects` not parsed.

- [ ] **Step 3: Add `ZoneEffects` to `Zone` struct and `yamlZone`**

In `internal/game/world/model.go`, add to `Zone` struct:
```go
// ZoneEffects are effects propagated to every room in this zone at load time.
ZoneEffects []RoomEffect
```

In `internal/game/world/loader.go`, add to `yamlZone`:
```go
ZoneEffects []RoomEffect `yaml:"zone_effects"`
```

- [ ] **Step 4: Propagate in `convertYAMLZone`**

In `convertYAMLZone`, after the room loop populates `zone.Rooms`, add:

```go
// Propagate zone-level effects to every room (REQ-ZN-1).
// Zone effects are prepended; room's own effects follow.
for _, room := range zone.Rooms {
    if len(yz.ZoneEffects) > 0 {
        combined := make([]RoomEffect, len(yz.ZoneEffects), len(yz.ZoneEffects)+len(room.Effects))
        copy(combined, yz.ZoneEffects)
        room.Effects = append(combined, room.Effects...)
    }
}
zone.ZoneEffects = yz.ZoneEffects
```

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/world/model.go internal/game/world/loader.go internal/game/world/loader_test.go
git commit -m "feat(world): add ZoneEffects propagation to rooms at load time (REQ-ZN-1)"
```

---

## Task 5: Replace `applyRoomEffectsOnEntry` Track enum with condition registry

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_zone_effect_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/grpc_service_zone_effect_test.go

func TestApplyRoomEffectsOnEntry_AppliesConditionViaRegistry(t *testing.T) {
    condReg := condition.NewRegistry()
    rageDef := &condition.ConditionDef{ID: "rage", Name: "Rage", DurationType: "permanent"}
    condReg.Register(rageDef)

    // Use a seeded dice roller so BaseDC=99 guarantees failure.
    roller := dice.NewRoller(rand.New(rand.NewSource(42)))
    logger, _ := zap.NewDevelopment()
    svc := &GameServiceServer{
        condRegistry: condReg,
        dice:         roller,
        logger:       logger,
    }
    sess := &session.PlayerSession{
        Conditions: condition.NewActiveSet(),
    }
    room := &world.Room{
        ID:      "r1",
        Effects: []world.RoomEffect{{Track: "rage", Severity: "mild", BaseDC: 99}},
    }
    svc.applyRoomEffectsOnEntry(sess, "uid1", room, time.Now().Unix())
    assert.True(t, sess.Conditions.Has("rage"))
}

func TestApplyRoomEffectsOnEntry_UnknownTrackLogsAndSkips(t *testing.T) {
    condReg := condition.NewRegistry() // empty
    logger, _ := zap.NewDevelopment()
    svc := &GameServiceServer{condRegistry: condReg, logger: logger}
    sess := &session.PlayerSession{Conditions: condition.NewActiveSet()}
    room := &world.Room{
        ID:      "r1",
        Effects: []world.RoomEffect{{Track: "unknown_xyz", BaseDC: 5}},
    }
    // Must not panic.
    svc.applyRoomEffectsOnEntry(sess, "uid1", room, time.Now().Unix())
    assert.False(t, sess.Conditions.Has("unknown_xyz"))
}

func TestApplyRoomEffectsOnEntry_CooldownPreventsReapply(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "rage", Name: "Rage"})
    logger, _ := zap.NewDevelopment()
    roller := dice.NewRoller(rand.New(rand.NewSource(0)))
    svc := &GameServiceServer{condRegistry: condReg, dice: roller, logger: logger}
    sess := &session.PlayerSession{
        Conditions:         condition.NewActiveSet(),
        ZoneEffectCooldowns: map[string]int64{"r1:rage": time.Now().Add(10 * time.Minute).Unix()},
    }
    room := &world.Room{
        ID:      "r1",
        Effects: []world.RoomEffect{{Track: "rage", BaseDC: 99}},
    }
    svc.applyRoomEffectsOnEntry(sess, "uid1", room, time.Now().Unix())
    assert.False(t, sess.Conditions.Has("rage"), "should be skipped due to cooldown")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestApplyRoomEffectsOnEntry -v
```

- [ ] **Step 3: Rewrite `applyRoomEffectsOnEntry`**

```go
func (s *GameServiceServer) applyRoomEffectsOnEntry(
    sess *session.PlayerSession, uid string, room *world.Room, now int64,
) {
    if len(room.Effects) == 0 {
        return
    }
    for _, effect := range room.Effects {
        key := room.ID + ":" + effect.Track
        if sess.ZoneEffectCooldowns != nil && sess.ZoneEffectCooldowns[key] > now {
            continue
        }
        def, ok := s.condRegistry.Get(effect.Track)
        if !ok {
            s.logger.Warn("applyRoomEffectsOnEntry: unknown condition track; skipping",
                zap.String("track", effect.Track),
                zap.String("room", room.ID))
            continue
        }
        gritMod := combat.AbilityMod(sess.Abilities.Grit)
        var roll int
        if s.dice != nil {
            roll = s.dice.Src().Intn(20) + 1
        } else {
            roll = 10
        }
        if roll+gritMod < effect.BaseDC {
            if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
                s.logger.Warn("applying room effect condition",
                    zap.String("id", def.ID), zap.Error(err))
            }
            if def.Description != "" && sess.Entity != nil {
                evt := messageEvent(def.Description)
                if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
                    _ = sess.Entity.Push(data)
                }
            }
        } else {
            if sess.ZoneEffectCooldowns == nil {
                sess.ZoneEffectCooldowns = make(map[string]int64)
            }
            sess.ZoneEffectCooldowns[key] = now + int64(effect.CooldownMinutes)*60
        }
    }
}
```

- [ ] **Step 4: Remove unused `abilityTrack`/`abilitySeverity` helpers if only used here**

```bash
grep -n "abilityTrack\|abilitySeverity" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go
```

If only in `applyRoomEffectsOnEntry`, delete those helper functions and update the `mentalStateMgr == nil` guard removal.

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -v -timeout 120s
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_zone_effect_test.go
git commit -m "feat(gameserver): replace RoomEffect Track enum with condition registry lookup (REQ-ZN-2)"
```

---

## Task 6: Wire `ValidateWithRegistry` into startup and add condition YAML files

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: 17 files in `content/conditions/`

- [ ] **Step 1: Find startup zone validation location**

```bash
grep -n "ValidateExits\|LoadZonesFromDir\|condRegistry\|allSkills" \
  /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -30
```

- [ ] **Step 2: Add `ValidateWithRegistry` call after exits validation**

After `world.ValidateExits()` (or equivalent startup zone check), add:

```go
for _, zone := range worldMgr.AllZones() {
    if err := zone.ValidateWithRegistry(condRegistry, allSkills); err != nil {
        return nil, fmt.Errorf("zone registry validation: %w", err)
    }
}
```

(Use exact variable names from the startup code.)

- [ ] **Step 3: Check which condition files already exist**

```bash
ls /home/cjohannsen/src/mud/content/conditions/ 2>/dev/null || echo "directory missing"
```

- [ ] **Step 4: Create condition YAML files**

Create any that do not already exist. Use `Write` tool for each:

**Legacy track conditions** (`rage`, `despair`, `delirium`, `fear`):

```yaml
# content/conditions/rage.yaml
id: rage
name: Rage
description: Uncontrollable anger surges through you.
duration_type: permanent
attack_bonus: 2
ac_penalty: 2
```

```yaml
# content/conditions/despair.yaml
id: despair
name: Despair
description: Hopelessness saps your will to act.
duration_type: permanent
attack_penalty: 2
```

```yaml
# content/conditions/delirium.yaml
id: delirium
name: Delirium
description: Reality fractures. Nothing feels real.
duration_type: permanent
skill_penalty: 2
```

```yaml
# content/conditions/fear.yaml
id: fear
name: Fear
description: Dread takes hold and your hands shake.
duration_type: permanent
attack_penalty: 1
ac_penalty: 1
```

**New atmospheric conditions** (REQ-ZN-5):

```yaml
# content/conditions/horror.yaml
id: horror
name: Horror
description: Soul-crushing dread freezes you in place.
duration_type: permanent
attack_penalty: 2
skill_penalty: 2
```

```yaml
# content/conditions/nausea.yaml
id: nausea
name: Nausea
description: Your stomach churns. Every movement is effort.
duration_type: permanent
attack_penalty: 1
skill_penalty: 1
```

```yaml
# content/conditions/reduced_visibility.yaml
id: reduced_visibility
name: Reduced Visibility
description: Haze or darkness limits your sight.
duration_type: permanent
attack_penalty: 2
```

```yaml
# content/conditions/temptation.yaml
id: temptation
name: Temptation
description: Irresistible urges pull at your focus.
duration_type: permanent
skill_penalty: 1
```

```yaml
# content/conditions/revulsion.yaml
id: revulsion
name: Revulsion
description: Profound disgust makes it hard to concentrate.
duration_type: permanent
skill_penalty: 1
```

```yaml
# content/conditions/sonic_assault.yaml
id: sonic_assault
name: Sonic Assault
description: Punishing noise hammers your senses without mercy.
duration_type: permanent
attack_penalty: 1
skill_penalty: 2
```

```yaml
# content/conditions/charmed.yaml
id: charmed
name: Charmed
description: You find yourself inexplicably drawn to them.
duration_type: until_save
```

**Terrain conditions** (REQ-ZN-12):

```yaml
# content/conditions/terrain_rubble.yaml
id: terrain_rubble
name: Rubble
description: Broken concrete and debris make every step treacherous.
duration_type: permanent
move_ap_cost: 1
skill_penalties:
  acrobatics: -2
```

```yaml
# content/conditions/terrain_mud.yaml
id: terrain_mud
name: Muddy Ground
description: Thick mud grips your boots with every step.
duration_type: permanent
move_ap_cost: 1
skill_penalties:
  acrobatics: -1
```

```yaml
# content/conditions/terrain_flooded.yaml
id: terrain_flooded
name: Flooded
description: Standing water slows movement and masks hazards underfoot.
duration_type: permanent
move_ap_cost: 2
skill_penalties:
  acrobatics: -2
  stealth: -2
```

```yaml
# content/conditions/terrain_ice.yaml
id: terrain_ice
name: Icy Surface
description: Slick ice threatens your footing at every turn.
duration_type: permanent
move_ap_cost: 1
skill_penalties:
  acrobatics: -3
```

```yaml
# content/conditions/terrain_dense_vegetation.yaml
id: terrain_dense_vegetation
name: Dense Vegetation
description: Tangled undergrowth tears at you as you push through.
duration_type: permanent
move_ap_cost: 1
skill_penalties:
  stealth: 1
```

Note: `stealth: 1` is a positive bonus (+1 to stealth) — vegetation provides cover. Negative would penalize stealth.

- [ ] **Step 5: Run property-based test and full suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestProperty_AllContent -v
cd /home/cjohannsen/src/mud && go test ./... -timeout 120s 2>&1 | tail -20
```
Expected: property test covers all 17 files; full suite passes.

- [ ] **Step 6: Commit**

```bash
git add content/conditions/ internal/gameserver/grpc_service.go
git commit -m "feat(content): add 17 condition YAML files; wire ValidateWithRegistry at startup (REQ-ZN-3, REQ-ZN-4, REQ-ZN-5, REQ-ZN-10, REQ-ZN-12)"
```

---

## Task 7: Add `Gender` to `npc.Template` and `npc.Instance`

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/npc_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/npc_test.go
func TestNewInstanceWithResolver_PropagatesGender(t *testing.T) {
    tmpl := &npc.Template{
        ID: "test_npc", Name: "Test NPC", Level: 1, MaxHP: 10, AC: 10,
        Gender: "male",
    }
    inst := npc.NewInstanceWithResolver(tmpl, "room_1", func(string) int { return 0 })
    assert.Equal(t, "male", inst.Gender)
}

func TestNewInstanceWithResolver_EmptyGenderPropagates(t *testing.T) {
    tmpl := &npc.Template{
        ID: "robot", Name: "Robot", Level: 1, MaxHP: 10, AC: 10,
        Gender: "",
    }
    inst := npc.NewInstanceWithResolver(tmpl, "room_1", func(string) int { return 0 })
    assert.Equal(t, "", inst.Gender)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestNewInstanceWithResolver_Propagates -v
```

- [ ] **Step 3: Add `Gender` to `Template`**

In `internal/game/npc/template.go`, add (near `NPCType`):
```go
// Gender is the NPC's gender string (e.g., "male", "female", "nonbinary").
// Empty string means no gender (genderless, robotic, etc.).
Gender string `yaml:"gender"`
```

- [ ] **Step 4: Add `Gender` to `Instance` and propagate**

In `internal/game/npc/instance.go`, add to `Instance` struct:
```go
// Gender is propagated from the template at spawn. Runtime-only; not persisted.
Gender string
```

In `NewInstanceWithResolver`, add:
```go
Gender: template.Gender,
```

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/npc_test.go
git commit -m "feat(npc): add Gender field to Template and Instance (REQ-ZN-6)"
```

---

## Task 8: Add `Conditions *condition.ActiveSet` to `npc.Instance`

This is required before any charmed or seduction logic can compile.

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/npc_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/npc/npc_test.go
func TestNewInstanceWithResolver_ConditionsInitialized(t *testing.T) {
    tmpl := &npc.Template{ID: "t", Name: "N", Level: 1, MaxHP: 10, AC: 10}
    inst := npc.NewInstanceWithResolver(tmpl, "r", func(string) int { return 0 })
    require.NotNil(t, inst.Conditions)
}

func TestInstance_Conditions_CanApplyCondition(t *testing.T) {
    tmpl := &npc.Template{ID: "t", Name: "N", Level: 1, MaxHP: 10, AC: 10}
    inst := npc.NewInstanceWithResolver(tmpl, "r", func(string) int { return 0 })
    def := &condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"}
    err := inst.Conditions.Apply(inst.ID, def, 1, -1)
    require.NoError(t, err)
    assert.True(t, inst.Conditions.Has("charmed"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestNewInstanceWithResolver_Conditions -v
```

- [ ] **Step 3: Add `Conditions` to `Instance` and initialize**

In `internal/game/npc/instance.go`, add to `Instance` struct:
```go
// Conditions tracks all active conditions on this NPC instance. Never nil.
Conditions *condition.ActiveSet
```

In `NewInstanceWithResolver`, add:
```go
Conditions: condition.NewActiveSet(),
```

Add import `"github.com/cory-johannsen/mud/internal/game/condition"` if not already present.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/instance.go internal/game/npc/npc_test.go
git commit -m "feat(npc): add Conditions ActiveSet to Instance; initialized at spawn (REQ-ZN-8, REQ-ZN-9)"
```

---

## Task 9: Add `SeductionRejected` to `npc.Instance`

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/npc_test.go`

Note: Respawn creates a new `Instance` via `NewInstanceWithResolver` — `SeductionRejected` is nil by default, satisfying the "reset on respawn" requirement without any extra code.

- [ ] **Step 1: Write failing test**

```go
func TestNewInstanceWithResolver_SeductionRejectedNilAtSpawn(t *testing.T) {
    tmpl := &npc.Template{ID: "t", Name: "N", Level: 1, MaxHP: 10, AC: 10}
    inst := npc.NewInstanceWithResolver(tmpl, "r", func(string) int { return 0 })
    assert.Nil(t, inst.SeductionRejected)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestNewInstanceWithResolver_SeductionRejected -v
```

- [ ] **Step 3: Add field to `Instance`**

```go
// SeductionRejected maps player UIDs to true when seduction was attempted and rejected.
// Runtime-only; nil by default; nil on each respawn (new instance created).
SeductionRejected map[string]bool
```

`NewInstanceWithResolver` does not need to initialize this — nil map reads as false for all keys.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/instance.go internal/game/npc/npc_test.go
git commit -m "feat(npc): add SeductionRejected field to Instance (REQ-ZN-8)"
```

---

## Task 10: Add `seduce <npc>` player command handler

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_seduce_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/grpc_service_seduce_test.go
package gameserver_test

import (
    "math/rand"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/world"
    "github.com/cory-johannsen/mud/internal/gameserver"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap"
)

func setupSeduceTest(t *testing.T) (*gameserver.GameServiceServer, *session.PlayerSession, *npc.Instance) {
    t.Helper()
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"})

    npcMgr := npc.NewManager()
    tmpl := &npc.Template{ID: "guard", Name: "Guard", Level: 1, MaxHP: 10, AC: 10, Gender: "male"}
    inst, err := npcMgr.Spawn(tmpl, "room_a")
    require.NoError(t, err)

    sess := &session.PlayerSession{
        RoomID:     "room_a",
        Skills:     map[string]string{"flair": "trained"},
        Conditions: condition.NewActiveSet(),
    }
    sessMgr := session.NewManager()
    sessMgr.SetPlayer("uid1", sess)

    logger, _ := zap.NewDevelopment()
    // Use seeded roller: seed 0 produces known sequence.
    roller := dice.NewRoller(rand.New(rand.NewSource(0)))
    svc := gameserver.NewTestServer(t, gameserver.TestServerConfig{
        CondRegistry: condReg,
        NPCManager:   npcMgr,
        Sessions:     sessMgr,
        Dice:         roller,
        Logger:       logger,
    })
    return svc, sess, inst
}

func TestHandleSeduce_NoFlairSkill_Fails(t *testing.T) {
    svc, sess, _ := setupSeduceTest(t)
    sess.Skills = map[string]string{} // no flair
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "Flair")
}

func TestHandleSeduce_NoGender_Fails(t *testing.T) {
    svc, _, inst := setupSeduceTest(t)
    inst.Gender = ""
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "not susceptible")
}

func TestHandleSeduce_AlreadyRejected_Fails(t *testing.T) {
    svc, _, inst := setupSeduceTest(t)
    inst.SeductionRejected = map[string]bool{"uid1": true}
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "isn't interested")
}

func TestHandleSeduce_AlreadyCharmed_Fails(t *testing.T) {
    svc, _, inst := setupSeduceTest(t)
    charmedDef := &condition.ConditionDef{ID: "charmed", Name: "Charmed"}
    _ = inst.Conditions.Apply(inst.ID, charmedDef, 1, -1)
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "already charmed")
}

func TestHandleSeduce_PlayerWins_NPCGainsCharmed(t *testing.T) {
    // Seed dice so player roll >> NPC roll (player wins opposed check).
    // With seed 99, adjust seed until player reliably wins — or inject
    // a deterministic resolver. For test reliability, use a mock dice that
    // always returns 20 for first call (player) and 1 for second (NPC).
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"})
    npcMgr := npc.NewManager()
    tmpl := &npc.Template{ID: "guard", Name: "Guard", Level: 1, MaxHP: 10, AC: 10, Gender: "male", Savvy: 1}
    inst, _ := npcMgr.Spawn(tmpl, "room_a")
    sess := &session.PlayerSession{
        RoomID:     "room_a",
        Skills:     map[string]string{"flair": "legendary"},
        Conditions: condition.NewActiveSet(),
    }
    sessMgr := session.NewManager()
    sessMgr.SetPlayer("uid1", sess)
    // Use a fixed-high-roll dice stub: always returns 19 (0-indexed = 19 from Intn(20)).
    roller := dice.NewFixedRoller([]int{19, 0}) // player: 19+1=20; NPC: 0+1=1
    logger, _ := zap.NewDevelopment()
    svc := gameserver.NewTestServer(t, gameserver.TestServerConfig{
        CondRegistry: condReg, NPCManager: npcMgr, Sessions: sessMgr, Dice: roller, Logger: logger,
    })
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "charmed")
    assert.True(t, inst.Conditions.Has("charmed"))
}

func TestHandleSeduce_PlayerLoses_NPCHostileAndRejected(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "charmed", Name: "Charmed"})
    npcMgr := npc.NewManager()
    tmpl := &npc.Template{ID: "guard", Name: "Guard", Level: 1, MaxHP: 10, AC: 10, Gender: "male", Savvy: 20}
    inst, _ := npcMgr.Spawn(tmpl, "room_a")
    sess := &session.PlayerSession{
        RoomID: "room_a", Skills: map[string]string{"flair": "trained"}, Conditions: condition.NewActiveSet(),
    }
    sessMgr := session.NewManager()
    sessMgr.SetPlayer("uid1", sess)
    roller := dice.NewFixedRoller([]int{0, 19}) // player: 1; NPC: 20
    logger, _ := zap.NewDevelopment()
    svc := gameserver.NewTestServer(t, gameserver.TestServerConfig{
        CondRegistry: condReg, NPCManager: npcMgr, Sessions: sessMgr, Dice: roller, Logger: logger,
    })
    events := svc.ExposedHandleSeduce("uid1", []string{"Guard"})
    require.Len(t, events, 1)
    assert.Contains(t, extractMessage(events[0]), "enemy")
    assert.Equal(t, "hostile", inst.Disposition)
    assert.True(t, inst.SeductionRejected["uid1"])
}

// extractMessage pulls the string content from a ServerEvent message payload.
func extractMessage(evt *gamev1.ServerEvent) string {
    if m := evt.GetMessage(); m != nil {
        return m.Content
    }
    return ""
}
```

**Note:** If `dice.NewFixedRoller` does not exist, add it to `internal/game/dice/` as part of this task. Also add `ExposedHandleSeduce` as a thin test-export shim if `handleSeduce` is unexported. If the test infra pattern uses a different approach (look at existing test server setup in `grpc_service_test.go`), follow that pattern exactly.

- [ ] **Step 2: Check existing test server setup pattern**

```bash
grep -n "NewTestServer\|testServer\|GameServiceServer{" \
  /home/cjohannsen/src/mud/internal/gameserver/grpc_service_test.go | head -20
```

Adjust test setup to match the existing pattern.

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleSeduce -v
```

- [ ] **Step 4: Implement `handleSeduce`**

```go
// handleSeduce processes the "seduce <npc>" player command.
//
// Preconditions: player has flair skill (sess.Skills["flair"] != ""), NPC has a Gender,
//   NPC is not already charmed, NPC has not rejected this player.
// Postcondition: on player win, NPC gains charmed condition;
//   on player loss, NPC disposition → "hostile", SeductionRejected[uid] = true.
func (s *GameServiceServer) handleSeduce(uid string, args []string) []*gamev1.ServerEvent {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil
    }
    if sess.Skills["flair"] == "" {
        return []*gamev1.ServerEvent{messageEvent("You lack the Flair skill for seduction.")}
    }
    if len(args) == 0 {
        return []*gamev1.ServerEvent{messageEvent("Seduce whom?")}
    }
    target := strings.Join(args, " ")
    inst := s.npcMgr.FindInRoom(sess.RoomID, target)
    if inst == nil {
        return []*gamev1.ServerEvent{messageEvent("You don't see " + target + " here.")}
    }
    if inst.Gender == "" {
        return []*gamev1.ServerEvent{messageEvent(inst.Name() + " is not susceptible to seduction.")}
    }
    if inst.Conditions.Has("charmed") {
        return []*gamev1.ServerEvent{messageEvent(inst.Name() + " is already charmed.")}
    }
    if inst.SeductionRejected != nil && inst.SeductionRejected[uid] {
        return []*gamev1.ServerEvent{messageEvent(inst.Name() + " isn't interested in anything you have to say.")}
    }

    // Opposed check: player Flair rank bonus vs NPC Savvy ability mod.
    playerRoll := s.dice.Src().Intn(20) + 1 + skillRankBonus(sess.Skills["flair"])
    npcRoll := s.dice.Src().Intn(20) + 1 + combat.AbilityMod(inst.Savvy)

    if playerRoll >= npcRoll {
        charmedDef, ok := s.condRegistry.Get("charmed")
        if ok {
            _ = inst.Conditions.Apply(inst.ID, charmedDef, 1, -1)
        }
        return []*gamev1.ServerEvent{messageEvent(inst.Name() + " seems charmed by you.")}
    }
    inst.Disposition = "hostile"
    if inst.SeductionRejected == nil {
        inst.SeductionRejected = make(map[string]bool)
    }
    inst.SeductionRejected[uid] = true
    return []*gamev1.ServerEvent{messageEvent(inst.Name() + " glares at you. You've made an enemy.")}
}
```

- [ ] **Step 5: Register the command**

Find the command registration block (search for `s.commands.Register` or similar):

```bash
grep -n "commands.Register\|RegisterCommand" \
  /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -20
```

Add registration in the same style as existing commands:
```go
s.commands.Register("seduce", func(uid string, args []string) []*gamev1.ServerEvent {
    return s.handleSeduce(uid, args)
})
```

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleSeduce -v
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_seduce_test.go
git commit -m "feat(gameserver): add seduce command handler (REQ-ZN-7, REQ-ZN-8)"
```

---

## Task 11: Add charmed NPC save at end of zone tick

**Files:**
- Modify: `internal/gameserver/zone_tick.go`
- Modify: `internal/gameserver/zone_tick_test.go`

The charmed save uses `combat.ResolveSave("cool", cbt, 15, src)` where `cbt` is constructed from the NPC instance. On `Success` or `CritSuccess`, remove `charmed`.

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/zone_tick_test.go

func TestCharmedNPCSave_HighSavvy_RemovesCondition(t *testing.T) {
    // NPC with Savvy=20 (huge mod), CoolRank="legendary" — save almost always passes.
    npcMgr := npc.NewManager()
    condReg := condition.NewRegistry()
    charmedDef := &condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"}
    condReg.Register(charmedDef)

    tmpl := &npc.Template{ID: "npc1", Name: "NPC", Level: 5, MaxHP: 20, AC: 12, Savvy: 20}
    inst, _ := npcMgr.Spawn(tmpl, "room_a")
    _ = inst.Conditions.Apply(inst.ID, charmedDef, 1, -1)

    // dice always returns 19 (index 0-based from Intn(20)) → roll 20.
    roller := dice.NewFixedRoller([]int{19})
    logger, _ := zap.NewDevelopment()
    svc := &GameServiceServer{npcMgr: npcMgr, condRegistry: condReg, dice: roller, logger: logger}

    // Run the charmed save tick for zone containing room_a.
    zone := &world.Zone{ID: "zone_a", Rooms: map[string]*world.Room{"room_a": {ID: "room_a"}}}
    svc.tickCharmedNPCSaves(zone)

    assert.False(t, inst.Conditions.Has("charmed"), "charmed should be removed on successful save")
}

func TestCharmedNPCSave_LowSavvy_ConditionRemains(t *testing.T) {
    npcMgr := npc.NewManager()
    condReg := condition.NewRegistry()
    charmedDef := &condition.ConditionDef{ID: "charmed", Name: "Charmed", DurationType: "until_save"}
    condReg.Register(charmedDef)

    tmpl := &npc.Template{ID: "npc2", Name: "NPC", Level: 1, MaxHP: 10, AC: 10, Savvy: 1} // AbilityMod(1)=−5
    inst, _ := npcMgr.Spawn(tmpl, "room_a")
    _ = inst.Conditions.Apply(inst.ID, charmedDef, 1, -1)

    // dice always returns 0 → roll 1. 1 + AbilityMod(1) = 1 + (-5) = -4 < 15 → fail.
    roller := dice.NewFixedRoller([]int{0})
    logger, _ := zap.NewDevelopment()
    svc := &GameServiceServer{npcMgr: npcMgr, condRegistry: condReg, dice: roller, logger: logger}

    zone := &world.Zone{ID: "zone_a", Rooms: map[string]*world.Room{"room_a": {ID: "room_a"}}}
    svc.tickCharmedNPCSaves(zone)

    assert.True(t, inst.Conditions.Has("charmed"), "charmed should remain on failed save")
}

func TestCharmedNPCSave_NoCharmedCondition_NoOp(t *testing.T) {
    npcMgr := npc.NewManager()
    condReg := condition.NewRegistry()
    tmpl := &npc.Template{ID: "npc3", Name: "NPC", Level: 1, MaxHP: 10, AC: 10}
    inst, _ := npcMgr.Spawn(tmpl, "room_a")
    // No charmed condition applied.

    roller := dice.NewFixedRoller([]int{})
    logger, _ := zap.NewDevelopment()
    svc := &GameServiceServer{npcMgr: npcMgr, condRegistry: condReg, dice: roller, logger: logger}
    zone := &world.Zone{ID: "zone_a", Rooms: map[string]*world.Room{"room_a": {ID: "room_a"}}}

    // Must not panic or call dice (no charmed NPC present).
    svc.tickCharmedNPCSaves(zone)
    assert.False(t, inst.Conditions.Has("charmed"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestCharmedNPCSave -v
```

- [ ] **Step 3: Implement `tickCharmedNPCSaves` helper**

Add to `internal/gameserver/zone_tick.go`:

```go
// tickCharmedNPCSaves runs charmed NPC saves for all NPCs in the given zone.
// Called at the end of each zone tick (REQ-ZN-9).
//
// Precondition: zone must not be nil.
// Postcondition: For each NPC in the zone's rooms that has the "charmed" condition,
// a Cool saving throw (DC 15) is resolved; on Success or CritSuccess, "charmed" is removed.
func (s *GameServiceServer) tickCharmedNPCSaves(zone *world.Zone) {
    const charmedDC = 15
    roomIDs := make(map[string]bool, len(zone.Rooms))
    for id := range zone.Rooms {
        roomIDs[id] = true
    }
    for _, inst := range s.npcMgr.AllInstances() {
        if !roomIDs[inst.RoomID] {
            continue
        }
        if !inst.Conditions.Has("charmed") {
            continue
        }
        cbt := &combat.Combatant{
            ID:       inst.ID,
            Level:    inst.Level,
            SavvyMod: combat.AbilityMod(inst.Savvy),
            CoolRank: inst.CoolRank,
        }
        outcome := combat.ResolveSave("cool", cbt, charmedDC, s.dice.Src())
        if outcome == combat.Success || outcome == combat.CritSuccess {
            inst.Conditions.Remove("charmed")
        }
    }
}
```

- [ ] **Step 4: Call `tickCharmedNPCSaves` in zone tick callback**

Find the zone tick callback registration in `zone_tick.go` (where `RegisterTick` is called). Add the call at the end of each tick:

```go
zone, ok := s.world.GetZone(zoneID)
if ok {
    s.tickCharmedNPCSaves(zone)
}
```

(Add this inside the existing tick function body.)

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestCharmedNPCSave -v
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/zone_tick.go internal/gameserver/zone_tick_test.go
git commit -m "feat(gameserver): add charmed NPC Cool save at end of zone tick (REQ-ZN-9)"
```

---

## Task 12: Replace terrain Properties check with condition-based movement check

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_move_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/gameserver/grpc_service_move_test.go

func TestHandleMove_SingleTerrainCondition_AppliesAPCost(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{
        ID: "terrain_mud", Name: "Muddy Ground",
        Description: "Thick mud grips your boots.",
        MoveAPCost:  1,
    })
    // Build room with terrain_mud in Effects (already propagated).
    room := &world.Room{
        ID: "room_b",
        Effects: []world.RoomEffect{{Track: "terrain_mud"}},
    }
    // sess with enough AP.
    sess := &session.PlayerSession{AP: 3, PassiveFeats: map[string]bool{}}
    // Exercise the terrain check portion of handleMove via a helper or test the
    // helper directly. If applyTerrainAPCosts is extracted, call it:
    svc := &GameServiceServer{condRegistry: condReg}
    msgs := svc.applyTerrainAPCosts(sess, "uid1", room)
    assert.Equal(t, 2, sess.AP, "AP should decrease by 1 (mud)")
    assert.Len(t, msgs, 1)
    assert.Contains(t, msgs[0], "Muddy Ground")
}

func TestHandleMove_TwoTerrainConditions_StacksAP(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "terrain_mud",    Name: "Mud",    MoveAPCost: 1})
    condReg.Register(&condition.ConditionDef{ID: "terrain_rubble", Name: "Rubble", MoveAPCost: 1})
    room := &world.Room{
        ID: "room_c",
        Effects: []world.RoomEffect{{Track: "terrain_mud"}, {Track: "terrain_rubble"}},
    }
    sess := &session.PlayerSession{AP: 5, PassiveFeats: map[string]bool{}}
    svc := &GameServiceServer{condRegistry: condReg}
    msgs := svc.applyTerrainAPCosts(sess, "uid1", room)
    assert.Equal(t, 3, sess.AP, "AP should decrease by 2 (mud+rubble)")
    assert.Len(t, msgs, 2, "one message per terrain condition")
    // Messages ordered alphabetically by condition ID.
    assert.Contains(t, msgs[0], "Mud")
    assert.Contains(t, msgs[1], "Rubble")
}

func TestHandleMove_ZoneAwareness_SuppressesMessageNotAP(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "terrain_mud", Name: "Mud", MoveAPCost: 1})
    room := &world.Room{
        ID:      "room_d",
        Effects: []world.RoomEffect{{Track: "terrain_mud"}},
    }
    sess := &session.PlayerSession{AP: 3, PassiveFeats: map[string]bool{"zone_awareness": true}}
    svc := &GameServiceServer{condRegistry: condReg}
    msgs := svc.applyTerrainAPCosts(sess, "uid1", room)
    assert.Equal(t, 2, sess.AP, "AP cost still applied")
    assert.Empty(t, msgs, "messages suppressed by zone_awareness")
}

func TestHandleMove_NonTerrainCondition_NotAffected(t *testing.T) {
    condReg := condition.NewRegistry()
    condReg.Register(&condition.ConditionDef{ID: "nausea", Name: "Nausea", MoveAPCost: 0})
    room := &world.Room{
        ID:      "room_e",
        Effects: []world.RoomEffect{{Track: "nausea"}},
    }
    sess := &session.PlayerSession{AP: 3, PassiveFeats: map[string]bool{}}
    svc := &GameServiceServer{condRegistry: condReg}
    msgs := svc.applyTerrainAPCosts(sess, "uid1", room)
    assert.Equal(t, 3, sess.AP, "no AP cost for non-terrain condition")
    assert.Empty(t, msgs)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleMove_.*Terrain -v
```

- [ ] **Step 3: Extract `applyTerrainAPCosts` helper and replace old check**

Add to `grpc_service.go`:

```go
// applyTerrainAPCosts deducts AP for all terrain_ conditions in the room and returns
// message strings for each (empty if zone_awareness suppresses messages).
//
// Precondition: sess and room must not be nil.
// Postcondition: sess.AP reduced by sum of all terrain_ MoveAPCost values;
//   one message string per terrain condition returned, alphabetical by condition ID,
//   unless sess.PassiveFeats["zone_awareness"] is true (messages suppressed, AP still deducted).
func (s *GameServiceServer) applyTerrainAPCosts(
    sess *session.PlayerSession, uid string, room *world.Room,
) []string {
    var terrainDefs []*condition.ConditionDef
    for _, effect := range room.Effects {
        if !strings.HasPrefix(effect.Track, "terrain_") {
            continue
        }
        def, ok := s.condRegistry.Get(effect.Track)
        if !ok || def.MoveAPCost <= 0 {
            continue
        }
        terrainDefs = append(terrainDefs, def)
    }
    if len(terrainDefs) == 0 {
        return nil
    }
    sort.Slice(terrainDefs, func(i, j int) bool { return terrainDefs[i].ID < terrainDefs[j].ID })
    totalAP := 0
    for _, def := range terrainDefs {
        totalAP += def.MoveAPCost
    }
    sess.AP -= totalAP
    if sess.PassiveFeats["zone_awareness"] {
        return nil
    }
    msgs := make([]string, 0, len(terrainDefs))
    for _, def := range terrainDefs {
        msgs = append(msgs, def.Name+": "+def.Description)
    }
    return msgs
}
```

- [ ] **Step 4: Replace old terrain check in `handleMove`**

Find the old block around line 1640 in `grpc_service.go`:

```go
if newRoom.Properties["terrain"] == "difficult" && !sess.PassiveFeats["zone_awareness"] {
    ...
}
```

Replace with:

```go
if terrainMsgs := s.applyTerrainAPCosts(sess, uid, newRoom); len(terrainMsgs) > 0 {
    for _, msg := range terrainMsgs {
        evt := messageEvent(msg)
        if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
            _ = sess.Entity.Push(data)
        }
    }
}
```

Add `"sort"` and `"strings"` imports if not already present.

- [ ] **Step 5: Run all tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... -timeout 120s 2>&1 | tail -30
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_move_test.go
git commit -m "feat(gameserver): replace terrain property check with terrain_ condition AP stacking (REQ-ZN-13, REQ-ZN-14)"
```

---

## Final Verification

- [ ] Run full test suite:

```bash
cd /home/cjohannsen/src/mud && go test ./... -timeout 180s
```

- [ ] Confirm property-based test covers all 17 condition YAML files:

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run TestProperty_AllContent -v
```

- [ ] Confirm no linter warnings:

```bash
cd /home/cjohannsen/src/mud && go vet ./...
```

- [ ] All 14 requirements covered by at least one test:

| REQ | Covered by |
|-----|-----------|
| REQ-ZN-1 | `TestLoadZoneFromBytes_ZoneEffectsPropagatedToRooms` |
| REQ-ZN-2 | `TestApplyRoomEffectsOnEntry_AppliesConditionViaRegistry` |
| REQ-ZN-3 | `TestZone_ValidateWithRegistry_UnknownTrack` |
| REQ-ZN-4 | `TestProperty_AllContentConditions_ValidRequired` (rage/despair/delirium/fear files) |
| REQ-ZN-5 | `TestProperty_AllContentConditions_ValidRequired` (horror/nausea/etc. files) |
| REQ-ZN-6 | `TestNewInstanceWithResolver_PropagatesGender` |
| REQ-ZN-7 | `TestHandleSeduce_NoFlairSkill_Fails` |
| REQ-ZN-8 | `TestHandleSeduce_PlayerWins_NPCGainsCharmed`, `TestHandleSeduce_PlayerLoses_NPCHostileAndRejected` |
| REQ-ZN-9 | `TestCharmedNPCSave_HighSavvy_RemovesCondition` |
| REQ-ZN-10 | `TestProperty_AllContentConditions_ValidRequired` (charmed.yaml) |
| REQ-ZN-11 | `TestZone_ValidateWithRegistry_UnknownSkillPenaltyKey` |
| REQ-ZN-12 | `TestProperty_AllContentConditions_ValidRequired` (terrain_*.yaml files) |
| REQ-ZN-13 | `TestHandleMove_TwoTerrainConditions_StacksAP`, `TestHandleMove_ZoneAwareness_SuppressesMessageNotAP` |
| REQ-ZN-14 | Enforced structurally by prefix rules; `TestHandleMove_NonTerrainCondition_NotAffected` |

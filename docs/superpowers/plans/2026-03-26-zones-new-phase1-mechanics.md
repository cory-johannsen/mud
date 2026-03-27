# Zones-New Phase 1: Mechanics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 1 mechanics for the New Zones feature: zone-level effect inheritance, track unification via condition registry, new condition definitions, typed terrain conditions, NPC gender field, and the seduction mechanic.

**Architecture:** `world.Zone` gains a `ZoneEffects` field propagated to rooms at load time; zone effect application switches from the hardcoded `abilityTrack`/`abilitySeverity` enum to direct `conditionRegistry.Get` + `condition.ActiveSet.Apply`; `ConditionDef` gains `MoveAPCost` and `SkillPenalties` fields enabling terrain-typed conditions; the movement handler replaces the `Properties["terrain"]=="difficult"` check with terrain condition accumulation; `npc.Template` and `npc.Instance` gain a `Gender` field; a player `seduce <npc>` command is added with an opposed Flair vs Savvy resolution.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property-based tests), gRPC gameserver, HTN AI system

---

## File Map

**Modified:**
- `internal/game/world/model.go` — add `ZoneEffects []RoomEffect` to `Zone`
- `internal/game/world/loader.go` — add `ZoneEffects` to `yamlZone`, propagate in `convertYAMLZone`
- `internal/game/world/model.go` — add `ValidateWithConditions(*condition.Registry) error` to `Zone`
- `internal/game/condition/definition.go` — add `MoveAPCost int` and `SkillPenalties map[string]int` to `ConditionDef`
- `internal/gameserver/grpc_service.go` — replace `abilityTrack`/`abilitySeverity` in `applyRoomEffectsOnEntry`; replace terrain `Properties["terrain"]` check
- `internal/game/npc/template.go` — add `Gender string` field to `Template`
- `internal/game/npc/instance.go` — add `Gender string` and `SeductionRejected map[string]bool` to `Instance`; propagate `Gender` in constructor
- `internal/gameserver/grpc_service_commands.go` (or equivalent command dispatch) — add `seduce` command handler
- `internal/gameserver/combat_handler.go` — add charmed Savvy save at end of round

**Created:**
- `content/conditions/fear.yaml` — base fear zone effect condition (REQ-ZN-4)
- `content/conditions/rage.yaml` — base rage zone effect condition (REQ-ZN-4)
- `content/conditions/despair.yaml` — base despair zone effect condition (REQ-ZN-4)
- `content/conditions/delirium.yaml` — base delirium zone effect condition (REQ-ZN-4)
- `content/conditions/horror.yaml` (REQ-ZN-5)
- `content/conditions/reduced_visibility.yaml` (REQ-ZN-5)
- `content/conditions/temptation.yaml` (REQ-ZN-5)
- `content/conditions/revulsion.yaml` (REQ-ZN-5)
- `content/conditions/sonic_assault.yaml` (REQ-ZN-5)
- `content/conditions/charmed.yaml` — duration_type: until_save (REQ-ZN-5)
- `content/conditions/terrain_rubble.yaml` (REQ-ZN-12)
- `content/conditions/terrain_mud.yaml` (REQ-ZN-12)
- `content/conditions/terrain_flooded.yaml` (REQ-ZN-12)
- `content/conditions/terrain_ice.yaml` (REQ-ZN-12)
- `content/conditions/terrain_dense_vegetation.yaml` (REQ-ZN-12)
- `internal/gameserver/grpc_service_seduce.go` — seduce command implementation
- `internal/gameserver/grpc_service_seduce_test.go` — TDD tests for seduce
- `internal/gameserver/grpc_service_zones_phase1_test.go` — TDD tests for zone effect inheritance, terrain, gender

---

## Task 1: Zone Effect Inheritance

**REQ-ZN-1**: `world.Zone` gains `ZoneEffects []RoomEffect`; loader propagates to every room at load time.

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/world/loader_test.go`:

```go
func TestLoadZone_ZoneEffects_PropagatedToRooms(t *testing.T) {
	data := []byte(`
zone:
  id: testzone
  name: Test Zone
  start_room: room1
  zone_effects:
    - track: fear
      severity: mild
      base_dc: 12
      cooldown_rounds: 3
      cooldown_minutes: 5
  rooms:
    - id: room1
      title: Room One
      description: A room.
      map_x: 0
      map_y: 0
    - id: room2
      title: Room Two
      description: Another room.
      map_x: 1
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	for _, room := range zone.Rooms {
		require.Len(t, room.Effects, 1, "zone_effects must be propagated to room %q", room.ID)
		assert.Equal(t, "fear", room.Effects[0].Track)
		assert.Equal(t, "mild", room.Effects[0].Severity)
		assert.Equal(t, 12, room.Effects[0].BaseDC)
	}
}

func TestLoadZone_ZoneEffects_DoNotOverrideRoomEffects(t *testing.T) {
	data := []byte(`
zone:
  id: testzone
  name: Test Zone
  start_room: room1
  zone_effects:
    - track: fear
      severity: mild
      base_dc: 10
      cooldown_rounds: 2
      cooldown_minutes: 3
  rooms:
    - id: room1
      title: Room One
      description: A room.
      map_x: 0
      map_y: 0
      effects:
        - track: rage
          severity: moderate
          base_dc: 14
          cooldown_rounds: 4
          cooldown_minutes: 6
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["room1"]
	require.Len(t, room.Effects, 2)
	tracks := []string{room.Effects[0].Track, room.Effects[1].Track}
	assert.Contains(t, tracks, "fear")
	assert.Contains(t, tracks, "rage")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run "TestLoadZone_ZoneEffects" -v 2>&1 | head -30
```

Expected: FAIL — `zone_effects` field unknown in YAML.

- [ ] **Step 3: Add `ZoneEffects` to `world.Zone`**

In `internal/game/world/model.go`, add after the `SettlementDC` field (line ~347):

```go
// ZoneEffects lists persistent aura effects that apply to ALL rooms in this zone.
// At load time these are appended to each room's Effects slice (REQ-ZN-1).
ZoneEffects []RoomEffect `yaml:"zone_effects,omitempty"`
```

- [ ] **Step 4: Add `ZoneEffects` to `yamlZone` in loader.go**

In `internal/game/world/loader.go`, add to `yamlZone` struct after `WorldY`:

```go
ZoneEffects []yamlRoomEffect `yaml:"zone_effects,omitempty"`
```

Also add `yamlRoomEffect` struct if `RoomEffect` is not already a separate YAML type (check the loader — if room effects are already in a yamlRoom, reuse that type):

```go
// yamlRoomEffect is the YAML representation of a room effect aura.
type yamlRoomEffect struct {
	Track           string `yaml:"track"`
	Severity        string `yaml:"severity"`
	BaseDC          int    `yaml:"base_dc"`
	CooldownRounds  int    `yaml:"cooldown_rounds"`
	CooldownMinutes int    `yaml:"cooldown_minutes"`
}
```

Note: check if the room effects in `yamlRoom` already uses a struct or uses `RoomEffect` directly. If it already uses `RoomEffect` directly via YAML tags, reuse that.

- [ ] **Step 5: Propagate ZoneEffects in `convertYAMLZone`**

In `internal/game/world/loader.go`, in `convertYAMLZone`, after all rooms have been added to `zone.Rooms`, add:

```go
// Propagate zone-level effects to every room (REQ-ZN-1).
if len(yz.ZoneEffects) > 0 {
	var zoneEffects []RoomEffect
	for _, ze := range yz.ZoneEffects {
		zoneEffects = append(zoneEffects, RoomEffect{
			Track:           ze.Track,
			Severity:        ze.Severity,
			BaseDC:          ze.BaseDC,
			CooldownRounds:  ze.CooldownRounds,
			CooldownMinutes: ze.CooldownMinutes,
		})
	}
	zone.ZoneEffects = zoneEffects
	for _, room := range zone.Rooms {
		room.Effects = append(room.Effects, zoneEffects...)
	}
}
```

Also add `zone.ZoneEffects = zoneEffects` to the Zone struct initialization.

- [ ] **Step 6: Run tests to verify passing**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -run "TestLoadZone_ZoneEffects" -v 2>&1
```

Expected: PASS

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/world/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/world/model.go internal/game/world/loader.go internal/game/world/loader_test.go
git commit -m "feat(zones): add ZoneEffects field with room propagation (REQ-ZN-1)"
```

---

## Task 2: New Condition Definitions

**REQ-ZN-4**: Add base track conditions for "fear", "rage", "despair", "delirium".
**REQ-ZN-5**: Add "horror", "reduced_visibility", "temptation", "revulsion", "sonic_assault", "charmed".

**Files:**
- Create: `content/conditions/fear.yaml`
- Create: `content/conditions/rage.yaml`
- Create: `content/conditions/despair.yaml`
- Create: `content/conditions/delirium.yaml`
- Create: `content/conditions/horror.yaml`
- Create: `content/conditions/reduced_visibility.yaml`
- Create: `content/conditions/temptation.yaml`
- Create: `content/conditions/revulsion.yaml`
- Create: `content/conditions/sonic_assault.yaml`
- Create: `content/conditions/charmed.yaml`
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/condition/definition_test.go`:

```go
func TestLoadDirectory_ZoneEffectConditionsPresent(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)

	trackIDs := []string{"fear", "rage", "despair", "delirium"}
	for _, id := range trackIDs {
		_, ok := reg.Get(id)
		assert.True(t, ok, "base track condition %q must exist", id)
	}

	newIDs := []string{"horror", "reduced_visibility", "temptation", "revulsion", "sonic_assault", "charmed"}
	for _, id := range newIDs {
		_, ok := reg.Get(id)
		assert.True(t, ok, "new zone effect condition %q must exist", id)
	}
}

func TestLoadDirectory_CharmedCondition_IsUntilSave(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	def, ok := reg.Get("charmed")
	require.True(t, ok)
	assert.Equal(t, "until_save", def.DurationType)
	assert.True(t, def.IsMentalCondition)
}

func TestLoadDirectory_TerrainConditionsNotInREQZN5(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	// None of the REQ-ZN-5 IDs should have the terrain_ prefix (REQ-ZN-14)
	zn5IDs := []string{"horror", "reduced_visibility", "temptation", "revulsion", "sonic_assault", "charmed", "fear", "rage", "despair", "delirium"}
	for _, id := range zn5IDs {
		assert.False(t, strings.HasPrefix(id, "terrain_"), "condition %q must not use terrain_ prefix", id)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestLoadDirectory_ZoneEffect|TestLoadDirectory_Charmed|TestLoadDirectory_TerrainCond" -v 2>&1 | head -30
```

Expected: FAIL — conditions not found.

- [ ] **Step 3: Create base track condition YAMLs**

Create `content/conditions/fear.yaml`:
```yaml
id: fear
name: Fear
description: |
  A wave of dread washes over you. Your hands tremble and your concentration falters.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/rage.yaml`:
```yaml
id: rage
name: Rage
description: |
  A burning fury rises in you. Your judgment clouds as aggression takes hold.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 1
ac_penalty: -1
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/despair.yaml`:
```yaml
id: despair
name: Despair
description: |
  A hollow emptiness settles over you. Every action feels pointless.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/delirium.yaml`:
```yaml
id: delirium
name: Delirium
description: |
  Reality shifts around you. Sounds and images blur into a dreamlike haze.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 4: Create new zone effect condition YAMLs**

Create `content/conditions/horror.yaml`:
```yaml
id: horror
name: Horror
description: |
  An overwhelming sense of wrongness floods your mind. What you see defies all reason.
duration_type: rounds
max_stacks: 0
attack_penalty: -1
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 2
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/reduced_visibility.yaml`:
```yaml
id: reduced_visibility
name: Reduced Visibility
description: |
  Steam, fog, or darkness limits your sight to a few feet. Everything is obscured.
duration_type: rounds
max_stacks: 0
attack_penalty: -2
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/temptation.yaml`:
```yaml
id: temptation
name: Temptation
description: |
  Carnal desire tugs at your attention. Focusing on anything else is harder than it should be.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/revulsion.yaml`:
```yaml
id: revulsion
name: Revulsion
description: |
  A wave of disgust rolls through you. Your stomach turns and your focus slips.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/sonic_assault.yaml`:
```yaml
id: sonic_assault
name: Sonic Assault
description: |
  Relentless bass and screeching synths batter your eardrums. Thinking is nearly impossible.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 2
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/charmed.yaml`:
```yaml
id: charmed
name: Charmed
description: |
  You feel an inexplicable fondness for someone. Your defenses around them soften.
duration_type: until_save
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: true
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 5: Run tests to verify passing**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestLoadDirectory_ZoneEffect|TestLoadDirectory_Charmed|TestLoadDirectory_TerrainCond" -v 2>&1
```

Expected: PASS

- [ ] **Step 6: Run full condition test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/conditions/fear.yaml content/conditions/rage.yaml content/conditions/despair.yaml content/conditions/delirium.yaml content/conditions/horror.yaml content/conditions/reduced_visibility.yaml content/conditions/temptation.yaml content/conditions/revulsion.yaml content/conditions/sonic_assault.yaml content/conditions/charmed.yaml internal/game/condition/definition_test.go
git commit -m "feat(conditions): add base track and new zone effect condition definitions (REQ-ZN-4, REQ-ZN-5)"
```

---

## Task 3: ConditionDef Terrain Fields + Terrain Condition YAMLs

**REQ-ZN-11**: `ConditionDef` gains `MoveAPCost int` and `SkillPenalties map[string]int`.
**REQ-ZN-12**: Add `terrain_rubble`, `terrain_mud`, `terrain_flooded`, `terrain_ice`, `terrain_dense_vegetation`.

**Files:**
- Modify: `internal/game/condition/definition.go`
- Create: `content/conditions/terrain_rubble.yaml`
- Create: `content/conditions/terrain_mud.yaml`
- Create: `content/conditions/terrain_flooded.yaml`
- Create: `content/conditions/terrain_ice.yaml`
- Create: `content/conditions/terrain_dense_vegetation.yaml`
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/condition/definition_test.go`:

```go
func TestConditionDef_MoveAPCostField_ParsesFromYAML(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`
id: terrain_test
name: Test Terrain
description: "Test terrain condition."
duration_type: rounds
move_ap_cost: 1
skill_penalties:
  hustle: 2
  rigging: 1
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "terrain_test.yaml"), data, 0644))
	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	def, ok := reg.Get("terrain_test")
	require.True(t, ok)
	assert.Equal(t, 1, def.MoveAPCost)
	require.NotNil(t, def.SkillPenalties)
	assert.Equal(t, 2, def.SkillPenalties["hustle"])
	assert.Equal(t, 1, def.SkillPenalties["rigging"])
}

func TestLoadDirectory_TerrainConditionsPresent(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	ids := []string{"terrain_rubble", "terrain_mud", "terrain_flooded", "terrain_ice", "terrain_dense_vegetation"}
	for _, id := range ids {
		def, ok := reg.Get(id)
		require.True(t, ok, "terrain condition %q must exist", id)
		assert.True(t, def.MoveAPCost > 0 || len(def.SkillPenalties) > 0,
			"terrain condition %q must set move_ap_cost or skill_penalties", id)
	}
}

func TestProperty_TerrainConditions_AlwaysHaveTerrainPrefix(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	for _, def := range reg.All() {
		if def.MoveAPCost > 0 {
			assert.True(t, strings.HasPrefix(def.ID, "terrain_"),
				"condition %q has MoveAPCost>0 but missing terrain_ prefix", def.ID)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestConditionDef_MoveAPCost|TestLoadDirectory_TerrainCond|TestProperty_TerrainCond" -v 2>&1 | head -30
```

Expected: FAIL — `MoveAPCost` and `SkillPenalties` fields not defined.

- [ ] **Step 3: Add `MoveAPCost` and `SkillPenalties` to `ConditionDef`**

In `internal/game/condition/definition.go`, add after `SkillPenalty int`:

```go
// MoveAPCost is the additional AP cost imposed on the bearer when they move.
// Only meaningful for terrain conditions (ID prefix "terrain_"). Default 0 = no extra cost.
MoveAPCost int `yaml:"move_ap_cost"`
// SkillPenalties maps skill ID → penalty applied while this condition is active.
// Keys must be canonical skill IDs (lowercase, underscore-separated).
// nil and empty map are equivalent.
SkillPenalties map[string]int `yaml:"skill_penalties,omitempty"`
```

- [ ] **Step 4: Create terrain condition YAMLs**

Create `content/conditions/terrain_rubble.yaml`:
```yaml
id: terrain_rubble
name: Rubble
description: |
  Broken concrete and debris cover the ground, making each step treacherous.
duration_type: permanent
max_stacks: 0
move_ap_cost: 1
skill_penalties:
  hustle: 1
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/terrain_mud.yaml`:
```yaml
id: terrain_mud
name: Mud
description: |
  Thick, clinging mud sucks at your boots with every step.
duration_type: permanent
max_stacks: 0
move_ap_cost: 1
skill_penalties:
  hustle: 2
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/terrain_flooded.yaml`:
```yaml
id: terrain_flooded
name: Flooded
description: |
  Standing water slows your movement and soaks your gear.
duration_type: permanent
max_stacks: 0
move_ap_cost: 1
skill_penalties:
  hustle: 1
  rigging: 1
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/terrain_ice.yaml`:
```yaml
id: terrain_ice
name: Ice
description: |
  Slick ice sends your footing scrambling with every step.
duration_type: permanent
max_stacks: 0
move_ap_cost: 1
skill_penalties:
  hustle: 3
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/terrain_dense_vegetation.yaml`:
```yaml
id: terrain_dense_vegetation
name: Dense Vegetation
description: |
  Thick undergrowth tangles around your legs and blocks your path.
duration_type: permanent
max_stacks: 0
move_ap_cost: 1
skill_penalties:
  hustle: 1
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 0
restrict_actions: []
prevents_movement: false
prevents_commands: false
prevents_targeting: false
is_mental_condition: false
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 5: Run tests to verify passing**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -run "TestConditionDef_MoveAPCost|TestLoadDirectory_TerrainCond|TestProperty_TerrainCond" -v 2>&1
```

Expected: PASS

- [ ] **Step 6: Run full condition test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/condition/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/condition/definition.go content/conditions/terrain_rubble.yaml content/conditions/terrain_mud.yaml content/conditions/terrain_flooded.yaml content/conditions/terrain_ice.yaml content/conditions/terrain_dense_vegetation.yaml internal/game/condition/definition_test.go
git commit -m "feat(conditions): add MoveAPCost/SkillPenalties fields and terrain condition definitions (REQ-ZN-11, REQ-ZN-12)"
```

---

## Task 4: Zone Effect Track Unification via Condition Registry

**REQ-ZN-2**: Replace `abilityTrack`/`abilitySeverity` in `applyRoomEffectsOnEntry` with `conditionRegistry.Get` + `condition.ActiveSet.Apply`.
**REQ-ZN-3**: Add `Zone.ValidateWithConditions(*condition.Registry) error` called at startup.

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`
- Create: `internal/gameserver/grpc_service_zones_phase1_test.go`

Note: The `abilityTrack`/`abilitySeverity` functions in `combat_handler.go` are NOT removed — they are used by the combat-round mental state system which is independent. Only the zone effect application path (`applyRoomEffectsOnEntry`) is changed.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_zones_phase1_test.go`:

```go
package gameserver_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newZonePhase1TestService returns a minimal GameServiceServer with a condRegistry
// containing "fear", "horror" conditions.
func newZonePhase1TestService(t *testing.T) *GameServiceServer {
	t.Helper()
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:            "fear",
		Name:          "Fear",
		DurationType:  "rounds",
		IsMentalCondition: true,
		SkillPenalty:  1,
	})
	condReg.Register(&condition.ConditionDef{
		ID:           "horror",
		Name:         "Horror",
		DurationType: "rounds",
		IsMentalCondition: true,
		SkillPenalty: 2,
	})
	return &GameServiceServer{
		condRegistry: condReg,
	}
}

// TestApplyRoomEffectsOnEntry_KnownCondition verifies that a room effect with a known
// condition ID applies the condition to the player session.
func TestApplyRoomEffectsOnEntry_KnownCondition(t *testing.T) {
	svc := newZonePhase1TestService(t)
	sess := &session.PlayerSession{
		Conditions:          condition.NewActiveSet(),
		ZoneEffectCooldowns: make(map[string]int64),
	}
	// Abilities.Grit = 0, so mod = 0; roll is determined by the zero dice fallback (10)
	// BaseDC = 20: 10 < 20, so save fails and condition is applied.
	room := &world.Room{
		ID: "room1",
		Effects: []world.RoomEffect{
			{Track: "fear", Severity: "mild", BaseDC: 20, CooldownRounds: 2, CooldownMinutes: 3},
		},
	}
	svc.applyRoomEffectsOnEntry(sess, "p1", room, 0)
	_, active := sess.Conditions.Get("fear")
	assert.True(t, active, "fear condition must be applied on failed save")
}

// TestApplyRoomEffectsOnEntry_UnknownCondition_Skipped verifies that an unknown track ID
// is silently skipped (with a log warning) and does not panic.
func TestApplyRoomEffectsOnEntry_UnknownCondition_Skipped(t *testing.T) {
	svc := newZonePhase1TestService(t)
	sess := &session.PlayerSession{
		Conditions:          condition.NewActiveSet(),
		ZoneEffectCooldowns: make(map[string]int64),
	}
	room := &world.Room{
		ID: "room1",
		Effects: []world.RoomEffect{
			{Track: "nonexistent_condition", Severity: "mild", BaseDC: 5},
		},
	}
	// Must not panic.
	require.NotPanics(t, func() {
		svc.applyRoomEffectsOnEntry(sess, "p1", room, 0)
	})
	// No condition applied.
	assert.Equal(t, 0, sess.Conditions.Count())
}

// TestApplyRoomEffectsOnEntry_CooldownRespected verifies that conditions on cooldown are skipped.
func TestApplyRoomEffectsOnEntry_CooldownRespected(t *testing.T) {
	svc := newZonePhase1TestService(t)
	now := time.Now().Unix()
	sess := &session.PlayerSession{
		Conditions: condition.NewActiveSet(),
		ZoneEffectCooldowns: map[string]int64{
			"room1:fear": now + 300, // 5 minutes from now
		},
	}
	room := &world.Room{
		ID: "room1",
		Effects: []world.RoomEffect{
			{Track: "fear", Severity: "mild", BaseDC: 20, CooldownMinutes: 5},
		},
	}
	svc.applyRoomEffectsOnEntry(sess, "p1", room, now)
	_, active := sess.Conditions.Get("fear")
	assert.False(t, active, "condition on cooldown must not be applied")
}
```

- [ ] **Step 2: Check the `condition.ActiveSet` API**

Read `internal/game/condition/active.go` to confirm the correct method signature for `Apply` and `Get`. The plan uses `sess.Conditions.Apply(def, ...)` and `sess.Conditions.Get(id)`. Adjust if the actual API differs.

```bash
cat /home/cjohannsen/src/mud/internal/game/condition/active.go
```

Note the exact method signatures and update test code accordingly.

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestApplyRoomEffectsOnEntry" -v 2>&1 | head -30
```

Expected: FAIL — new code path not yet implemented.

- [ ] **Step 4: Add `Zone.ValidateWithConditions` to `model.go`**

In `internal/game/world/model.go`, add after `Validate()`:

```go
// ValidateWithConditions checks that every RoomEffect.Track value (on both zone-level
// and room-level effects) exists in the given condition registry.
//
// Precondition: reg must not be nil. z must have already passed Validate().
// Postcondition: Returns nil if all Track IDs are registered, or a descriptive error.
func (z *Zone) ValidateWithConditions(reg interface {
	Get(string) (*condition.ConditionDef, bool)
}) error {
	checkEffect := func(roomID, trackID string) error {
		if _, ok := reg.Get(trackID); !ok {
			return fmt.Errorf("zone %q: room %q: effect track %q not found in condition registry", z.ID, roomID, trackID)
		}
		return nil
	}
	// Validate zone-level effects.
	for _, eff := range z.ZoneEffects {
		if err := checkEffect("<zone>", eff.Track); err != nil {
			return err
		}
	}
	// Validate room-level effects.
	for id, room := range z.Rooms {
		for _, eff := range room.Effects {
			if err := checkEffect(id, eff.Track); err != nil {
				return err
			}
		}
	}
	return nil
}
```

Note: The import for `condition` must be added to model.go. To avoid a circular import (world importing condition), use an interface parameter for the registry rather than the concrete type:

```go
// ValidateWithConditions checks that every RoomEffect.Track value exists in reg.
// reg is any type with a Get(string) (bool) lookup — typically *condition.Registry.
func (z *Zone) ValidateWithConditions(reg interface {
	Has(id string) bool
}) error {
```

Or add a `Has(id string) bool` method to `condition.Registry` and use that.

Check whether `internal/game/world` already imports `condition`. If not, add a `Has` method to `condition.Registry` and use an interface to avoid circular imports.

- [ ] **Step 5: Add `Has` to `condition.Registry`**

In `internal/game/condition/definition.go`, add after `Get`:

```go
// Has returns true if id is registered.
func (r *Registry) Has(id string) bool {
	_, ok := r.defs[id]
	return ok
}
```

- [ ] **Step 6: Rewrite `applyRoomEffectsOnEntry` in `grpc_service.go`**

Find `applyRoomEffectsOnEntry` at line ~2641 in `internal/gameserver/grpc_service.go`. Replace the body:

```go
func (s *GameServiceServer) applyRoomEffectsOnEntry(
	sess *session.PlayerSession, uid string, room *world.Room, now int64,
) {
	if len(room.Effects) == 0 {
		return
	}
	for _, effect := range room.Effects {
		key := room.ID + ":" + effect.Track

		// REQ-ZN-2: resolve condition via registry; skip and warn if not found.
		if s.condRegistry == nil {
			continue
		}
		def, ok := s.condRegistry.Get(effect.Track)
		if !ok {
			s.logger.Warn("applyRoomEffectsOnEntry: unknown condition track",
				zap.String("track", effect.Track),
				zap.String("roomID", room.ID),
			)
			continue
		}

		if sess.ZoneEffectCooldowns != nil && sess.ZoneEffectCooldowns[key] > now {
			continue // immune: cooldown has not expired
		}

		gritMod := combat.AbilityMod(sess.Abilities.Grit)
		var roll int
		if s.dice != nil {
			roll = s.dice.Src().Intn(20) + 1
		} else {
			roll = 10
		}
		total := roll + gritMod

		if total < effect.BaseDC {
			// Failed save: apply condition directly via ActiveSet.
			if sess.Conditions != nil {
				sess.Conditions.Apply(def)
				msg := fmt.Sprintf("You feel %s wash over you.", strings.ToLower(def.Name))
				if sess.Entity != nil {
					evt := messageEvent(msg)
					if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
						_ = sess.Entity.Push(data)
					}
				}
			}
		} else {
			// Successful save: record immunity cooldown.
			if sess.ZoneEffectCooldowns == nil {
				sess.ZoneEffectCooldowns = make(map[string]int64)
			}
			sess.ZoneEffectCooldowns[key] = now + int64(effect.CooldownMinutes)*60
		}
	}
}
```

Note: Check the exact `condition.ActiveSet.Apply` signature from `active.go` (Step 2) and adjust the call accordingly.

- [ ] **Step 7: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestApplyRoomEffectsOnEntry" -v 2>&1
```

Expected: PASS

- [ ] **Step 8: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | tail -20
```

Expected: all PASS. Fix any compilation errors from the ActiveSet API change.

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/game/world/model.go internal/game/condition/definition.go internal/gameserver/grpc_service_zones_phase1_test.go
git commit -m "feat(zones): replace hardcoded track enum with conditionRegistry lookup (REQ-ZN-2, REQ-ZN-3)"
```

---

## Task 5: Terrain Movement Handler Refactor

**REQ-ZN-13**: Remove `Properties["terrain"]=="difficult"` check; replace with terrain condition accumulation.
**REQ-ZN-14**: Ensure terrain condition IDs don't appear in REQ-ZN-5 list (enforced by naming convention and test).

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_move_test.go`
- Create (new test cases): add to existing move test file

- [ ] **Step 1: Read the existing terrain test to understand the test helper**

```bash
cat /home/cjohannsen/src/mud/internal/gameserver/grpc_service_move_test.go | head -100
```

Note the `newDifficultTerrainWorld` helper function and how the service is constructed.

- [ ] **Step 2: Write failing tests for new terrain behavior**

Add to `internal/gameserver/grpc_service_move_test.go`:

```go
// newTerrainConditionWorld creates a world where room_b has terrain_mud zone effect propagated.
// condReg must include "terrain_mud" definition with MoveAPCost: 1.
func newTerrainConditionWorld(condReg *condition.Registry) (*world.Manager, *condition.Registry) {
	terrain := condReg
	if terrain == nil {
		terrain = condition.NewRegistry()
		terrain.Register(&condition.ConditionDef{
			ID:          "terrain_mud",
			Name:        "Mud",
			DurationType: "permanent",
			MoveAPCost:  1,
		})
	}
	// Build world with room_b having terrain_mud condition via its Effects slice.
	// (Zone propagation already tested in world/loader_test.go; here we set it directly.)
	rm := world.NewManager()
	zoneA := &world.Zone{
		ID:        "zone_a",
		Name:      "Zone A",
		StartRoom: "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:    "room_a",
				ZoneID: "zone_a",
				Title: "Room A",
				Description: "Room A",
				Exits: []world.Exit{{Direction: "north", TargetRoom: "room_b"}},
			},
			"room_b": {
				ID:    "room_b",
				ZoneID: "zone_a",
				Title: "Room B",
				Description: "Room B",
				Exits: []world.Exit{{Direction: "south", TargetRoom: "room_a"}},
				Effects: []world.RoomEffect{
					{Track: "terrain_mud", Severity: "", BaseDC: 0, CooldownRounds: 0, CooldownMinutes: 0},
				},
			},
		},
	}
	rm.AddZone(zoneA)
	return rm, terrain
}

// TestMove_TerrainCondition_DeductsAP verifies that moving into a room with a terrain_
// condition deducts the condition's MoveAPCost from the player.
func TestMove_TerrainCondition_DeductsAP(t *testing.T) {
	// This test verifies REQ-ZN-13: terrain_ conditions with MoveAPCost > 0
	// impose AP cost on the player when entering that room.
	// Implementation note: the movement handler must be updated to check
	// room.Effects for terrain_ prefix conditions after the terrain Properties check is removed.
	// The exact AP deduction mechanism depends on the existing AP deduction pattern —
	// verify by reading the movement handler code and adjust this test accordingly.
	t.Skip("implement after replacing terrain Properties check in movement handler")
}

// TestMove_TerrainCondition_MessageSent verifies that a terrain_ condition message is sent to
// the player (unless they have zone_awareness).
func TestMove_TerrainCondition_MessageSent(t *testing.T) {
	t.Skip("implement after replacing terrain Properties check in movement handler")
}

// TestMove_ZoneAwareness_SuppressesTerrainMessage verifies that zone_awareness suppresses
// terrain messages but NOT the AP cost deduction (REQ-ZN-13).
func TestMove_ZoneAwareness_SuppressesTerrainMessageButNotAPCost(t *testing.T) {
	t.Skip("implement after replacing terrain Properties check in movement handler")
}
```

Note: The terrain AP deduction mechanism needs to be read from the existing code before these tests can be un-skipped. Read the movement handler to find how AP is currently deducted, then fill in the test body.

- [ ] **Step 3: Read the movement handler terrain section**

```bash
grep -n "Properties\[.terrain.\]\|zone_awareness\|terrainEvt\|AP\|ap" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -40
```

Identify:
1. Exact line range of the `Properties["terrain"]=="difficult"` block to remove
2. How AP costs are applied to the player during movement (search for AP deduction in move handler)

- [ ] **Step 4: Replace the `Properties["terrain"]=="difficult"` block**

In `internal/gameserver/grpc_service.go`, find and REPLACE the block at lines ~2263–2284 (the terrain Properties check):

```go
// REQ-ZN-13: collect terrain_ conditions from the room's effective condition set
// and deduct their MoveAPCost from the player. Send one message per condition
// (ordered alphabetically by ID), suppressed if player has zone_awareness feat.
if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok && s.condRegistry != nil {
	// Collect all terrain_ conditions sorted by ID.
	var terrainConds []*condition.ConditionDef
	for _, eff := range newRoom.Effects {
		if !strings.HasPrefix(eff.Track, "terrain_") {
			continue
		}
		def, defOK := s.condRegistry.Get(eff.Track)
		if !defOK || def.MoveAPCost <= 0 {
			continue
		}
		terrainConds = append(terrainConds, def)
	}
	sort.Slice(terrainConds, func(i, j int) bool {
		return terrainConds[i].ID < terrainConds[j].ID
	})
	totalAPCost := 0
	for _, def := range terrainConds {
		totalAPCost += def.MoveAPCost
		if !sess.PassiveFeats["zone_awareness"] {
			msg := fmt.Sprintf("The ground here is %s — movement costs extra AP.", strings.ToLower(def.Name))
			terrainEvt := messageEvent(msg)
			if data, marshalErr := proto.Marshal(terrainEvt); marshalErr == nil {
				if pushErr := sess.Entity.Push(data); pushErr != nil {
					s.logger.Warn("pushing terrain message to player entity",
						zap.String("uid", uid),
						zap.String("condID", def.ID),
						zap.Error(pushErr),
					)
				}
			}
		}
	}
	// Deduct total terrain AP cost from the player's current AP (REQ-ZN-13).
	// After reading Step 3 output, identify the AP field on PlayerSession
	// (search for "ActionPoints" or "AP" or "ap" in session.PlayerSession and
	//  the existing combat move cost deduction). Then replace this block.
	// If the server uses sess.ActionPoints (or similar), subtract totalAPCost here.
	// If AP is only deducted during combat via the combat.Combatant, find the combatant
	// for this player and call combatant.DeductAP(totalAPCost) or equivalent.
	// Example (adjust field name to match actual struct):
	//   sess.ActionPoints = max(0, sess.ActionPoints - totalAPCost)
	if totalAPCost > 0 {
		sess.ActionPoints -= totalAPCost // replace "ActionPoints" with actual field name from Step 3
		if sess.ActionPoints < 0 {
			sess.ActionPoints = 0
		}
	}
}
```

Important: After reading the movement handler in Step 3, replace the `_ = totalAPCost` placeholder with the actual AP deduction mechanism.

- [ ] **Step 5: Remove `Properties["terrain"]` from existing move tests**

In `internal/gameserver/grpc_service_move_test.go`, the existing `newDifficultTerrainWorld` creates rooms with `Properties: map[string]string{"terrain": "difficult"}`. After the change, this no longer triggers any behavior.

Update the helper to use `Effects: []world.RoomEffect{{Track: "terrain_mud", ...}}` instead, and update the test expectations to match the new message format ("Mud" instead of "difficult terrain").

The existing tests:
- `TestMove_DifficultTerrain_MessageSent` → update to use terrain condition Effects
- `TestMove_DifficultTerrain_ZoneAwareness_NoMessage` → update
- `TestMove_NormalTerrain_NoMessage` → keep as-is (no effects = no message)
- `TestProperty_ZoneAwareness_NeverReceivesDifficultTerrainMessage` → update

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestMove.*Terrain|TestProperty.*Terrain" -v 2>&1
```

Expected: PASS

- [ ] **Step 7: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_move_test.go
git commit -m "feat(zones): replace Properties terrain check with typed terrain conditions (REQ-ZN-13, REQ-ZN-14)"
```

---

## Task 6: NPC Gender Field

**REQ-ZN-6**: `npc.Template` gains `Gender string`; `npc.Instance` propagates it at spawn.

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/instance_test.go` or create test file

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/instance_test.go` (or create `internal/game/npc/gender_test.go`):

```go
func TestNPCInstance_PropagatesGenderFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID:     "soldier",
		Name:   "Soldier",
		Gender: "male",
		MaxHP:  30,
		Level:  2,
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.Equal(t, "male", inst.Gender)
}

func TestNPCInstance_GenderEmpty_WhenTemplateHasNoGender(t *testing.T) {
	tmpl := &npc.Template{
		ID:    "robot",
		Name:  "Robot",
		MaxHP: 20,
		Level: 1,
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.Equal(t, "", inst.Gender)
}

func TestNPCTemplate_GenderParsesFromYAML(t *testing.T) {
	// Verify the yaml tag "gender" is present on Template.Gender.
	// This test confirms correct struct tag by checking the loaded template.
	tmpl := npc.Template{}
	data := []byte(`
id: test_npc
name: Test NPC
gender: female
max_hp: 10
level: 1
`)
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.Equal(t, "female", tmpl.Gender)
}

// TestProperty_NPC_SeductionRejected_InitiallyNil verifies that SeductionRejected
// is nil on a freshly spawned NPC instance.
func TestProperty_NPC_SeductionRejected_InitiallyNil(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz"))).Draw(rt, "name")
		tmpl := &npc.Template{
			ID:    "tmpl",
			Name:  name,
			MaxHP: rapid.IntRange(1, 100).Draw(rt, "hp"),
			Level: rapid.IntRange(1, 20).Draw(rt, "level"),
		}
		inst := npc.NewInstance("id", tmpl, "room")
		if inst.SeductionRejected != nil {
			rt.Fatal("SeductionRejected must be nil on new instance")
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestNPCInstance_Propagates|TestNPCInstance_Gender|TestNPCTemplate_Gender|TestProperty_NPC_Seduction" -v 2>&1 | head -30
```

Expected: FAIL — `Gender` and `SeductionRejected` fields not defined.

- [ ] **Step 3: Add `Gender` to `npc.Template`**

In `internal/game/npc/template.go`, add after the `Type` field:

```go
// Gender is the NPC's gender string. Empty string means no gender (e.g. robots, animals).
// Used as a precondition for the seduce command (REQ-ZN-6).
Gender string `yaml:"gender"`
```

- [ ] **Step 4: Add `Gender` and `SeductionRejected` to `npc.Instance`**

In `internal/game/npc/instance.go`, add after the `Type` field:

```go
// Gender is propagated from Template.Gender at spawn (REQ-ZN-6).
// Runtime-only: no YAML tag. Per-instance override not supported.
Gender string
// SeductionRejected maps player UID → true when this NPC has rejected a seduction
// attempt from that player (REQ-ZN-8). Runtime-only: no YAML tag. Nil until first rejection.
SeductionRejected map[string]bool
```

- [ ] **Step 5: Propagate `Gender` in NPC instance constructor**

In `internal/game/npc/instance.go`, find `NewInstance` (and/or `NewInstanceWithResolver`). Add `Gender` propagation:

```go
// After the line that copies Type from template:
Gender: tmpl.Gender,
```

Also ensure `SeductionRejected` is NOT initialized (leave nil; it's initialized on first use).

At respawn: find the respawn logic (likely in `npc/manager.go` or the handler that recreates instances). Ensure that when an NPC is respawned, the new instance has `SeductionRejected: nil` (this is automatic since `NewInstance` doesn't set it).

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestNPCInstance_Propagates|TestNPCInstance_Gender|TestNPCTemplate_Gender|TestProperty_NPC_Seduction" -v 2>&1
```

Expected: PASS

- [ ] **Step 7: Run full NPC test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/gender_test.go
git commit -m "feat(npc): add Gender and SeductionRejected fields to Template and Instance (REQ-ZN-6)"
```

---

## Task 7: Seduction Command + Charmed Save

**REQ-ZN-7**: `seduce <npc>` player command with preconditions.
**REQ-ZN-8**: Opposed Flair vs NPC Savvy; charmed on success; hostile+SeductionRejected on failure.
**REQ-ZN-9**: Charmed NPCs make Savvy save vs DC 15 at end of each round.
**REQ-ZN-10**: Charmed NPCs treat the player as allied (skip them in attack selection).

**Files:**
- Create: `internal/gameserver/grpc_service_seduce.go`
- Create: `internal/gameserver/grpc_service_seduce_test.go`
- Modify: `internal/gameserver/combat_handler.go` — add charmed Savvy save at round end

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_seduce_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newSeduceTestService returns a GameServiceServer configured for seduce tests.
// condReg must include "charmed" condition with duration_type: until_save.
func newSeduceTestService(t *testing.T) *GameServiceServer {
	t.Helper()
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "charmed",
		Name:         "Charmed",
		DurationType: "until_save",
		IsMentalCondition: true,
	})
	return buildTestServiceWithCondReg(t, condReg) // use existing test helper or create minimal one
}

// TestSeduce_NoGender_Rejected verifies that attempting to seduce a genderless NPC fails.
func TestSeduce_NoGender_Rejected(t *testing.T) {
	svc := newSeduceTestService(t)
	tmpl := &npc.Template{ID: "robot", Name: "Robot", MaxHP: 20, Level: 1}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	// Gender is empty by default (robot has no gender).
	sess := &session.PlayerSession{Skills: map[string]int{"flair": 5}}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "cannot be seduced")
}

// TestSeduce_NoFlair_Rejected verifies that a player with zero flair cannot seduce.
func TestSeduce_NoFlair_Rejected(t *testing.T) {
	svc := newSeduceTestService(t)
	tmpl := &npc.Template{ID: "npc1", Name: "Guard", MaxHP: 30, Level: 2, Gender: "female"}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	sess := &session.PlayerSession{Skills: map[string]int{"flair": 0}}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "You lack the charm")
}

// TestSeduce_AlreadyCharmed_Rejected verifies that already-charmed NPC cannot be seduced again.
func TestSeduce_AlreadyCharmed_Rejected(t *testing.T) {
	svc := newSeduceTestService(t)
	condReg := svc.condRegistry
	charmedDef, _ := condReg.Get("charmed")
	tmpl := &npc.Template{ID: "npc1", Name: "Guard", MaxHP: 30, Level: 2, Gender: "male"}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	inst.Conditions = condition.NewActiveSet()
	inst.Conditions.Apply(charmedDef)
	sess := &session.PlayerSession{Skills: map[string]int{"flair": 8}}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "already charmed")
}

// TestSeduce_PreviouslyRejected_Rejected verifies that SeductionRejected blocks re-attempt.
func TestSeduce_PreviouslyRejected_Rejected(t *testing.T) {
	svc := newSeduceTestService(t)
	tmpl := &npc.Template{ID: "npc1", Name: "Guard", MaxHP: 30, Level: 2, Gender: "male"}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	inst.SeductionRejected = map[string]bool{"p1": true}
	sess := &session.PlayerSession{Skills: map[string]int{"flair": 8}}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "not interested")
}

// TestSeduce_Success_NPCCharmed verifies that high Flair > NPC Savvy → NPC gains charmed.
func TestSeduce_Success_NPCCharmed(t *testing.T) {
	svc := newSeduceTestService(t)
	// Set NPC Savvy very low so player always wins (dice fallback = 10; 10 + 20 > 10 + (-5)).
	tmpl := &npc.Template{ID: "npc1", Name: "Hostess", MaxHP: 20, Level: 1, Gender: "female",
		Abilities: npc.Abilities{Savvy: -5}}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	inst.Conditions = condition.NewActiveSet()
	sess := &session.PlayerSession{
		Skills: map[string]int{"flair": 20},
	}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "charmed")
	_, active := inst.Conditions.Get("charmed")
	assert.True(t, active, "NPC must have charmed condition on player success")
}

// TestSeduce_Failure_NPCTurnsHostile verifies that low Flair < NPC Savvy → NPC turns hostile.
func TestSeduce_Failure_NPCTurnsHostile(t *testing.T) {
	svc := newSeduceTestService(t)
	// NPC Savvy so high player always loses (dice fallback = 10; 10 + 1 < 10 + 100).
	tmpl := &npc.Template{ID: "npc1", Name: "Bouncer", MaxHP: 40, Level: 3, Gender: "male",
		Abilities: npc.Abilities{Savvy: 100}}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	inst.Conditions = condition.NewActiveSet()
	sess := &session.PlayerSession{
		Skills: map[string]int{"flair": 1},
	}
	msg, err := svc.executeSeduce(sess, "p1", inst)
	require.NoError(t, err)
	assert.Contains(t, msg, "hostile")
	assert.Equal(t, "hostile", inst.Disposition)
	require.NotNil(t, inst.SeductionRejected)
	assert.True(t, inst.SeductionRejected["p1"])
}

// TestProperty_Seduce_HighFlairAlwaysCharmes verifies that flair >= NPC savvy + 10 always succeeds.
// Uses dice fallback (10) so playerRoll = 10+flair, npcRoll = 10+savvy; player wins iff flair > savvy.
func TestProperty_Seduce_HighFlairAlwaysCharmes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc := newSeduceTestService(t)
		npcSavvy := rapid.IntRange(-5, 5).Draw(rt, "savvy")
		playerFlair := npcSavvy + rapid.IntRange(1, 20).Draw(rt, "flair_bonus") // always > savvy
		tmpl := &npc.Template{ID: "npc1", Name: "Target", MaxHP: 20, Level: 1, Gender: "female",
			Abilities: npc.Abilities{Savvy: npcSavvy}}
		inst := npc.NewInstance("inst1", tmpl, "room1")
		inst.Conditions = condition.NewActiveSet()
		sess := &session.PlayerSession{
			Skills: map[string]int{"flair": playerFlair},
		}
		_, err := svc.executeSeduce(sess, "p1", inst)
		require.NoError(rt, err)
		_, active := inst.Conditions.Get("charmed")
		if !active {
			rt.Fatalf("expected charmed with flair=%d vs savvy=%d", playerFlair, npcSavvy)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestSeduce|TestProperty_Seduce" -v 2>&1 | head -30
```

Expected: FAIL — `executeSeduce` not defined.

- [ ] **Step 3: Check `condition.ActiveSet` API for NPC use**

Read `internal/game/condition/active.go` to confirm:
- `Apply(def *ConditionDef)` method signature
- `Get(id string) (*ActiveCondition, bool)` method signature
- `NewActiveSet()` constructor

Verify `npc.Instance` has a `Conditions` field (check `internal/game/npc/instance.go`). If not, add `Conditions *condition.ActiveSet` to Instance.

- [ ] **Step 4: Implement `executeSeduce` in `grpc_service_seduce.go`**

Create `internal/gameserver/grpc_service_seduce.go`:

```go
package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/session"
)

// executeSeduce resolves a player's attempt to seduce an NPC (REQ-ZN-7, REQ-ZN-8).
//
// Preconditions checked:
//   - inst.Gender != "" (genderless NPCs cannot be seduced)
//   - sess.Skills["flair"] > 0 (player must have flair skill)
//   - NPC does not already have the "charmed" condition
//   - inst.SeductionRejected[uid] is not true
//
// Resolution: opposed check. playerRoll = d20 + sess.Skills["flair"].
// npcRoll = d20 + inst.Savvy.
// On player success (playerRoll >= npcRoll): NPC gains "charmed" condition.
// On player failure (playerRoll < npcRoll): NPC turns hostile; SeductionRejected[uid] = true.
//
// Returns a message string and nil error. Non-nil error only for internal failures.
func (s *GameServiceServer) executeSeduce(
	sess *session.PlayerSession, uid string, inst *npc.Instance,
) (string, error) {
	// Precondition: NPC must have a gender.
	if inst.Gender == "" {
		return fmt.Sprintf("%s cannot be seduced.", inst.Name()), nil
	}
	// Precondition: player must have flair.
	if sess.Skills["flair"] <= 0 {
		return "You lack the charm to attempt seduction.", nil
	}
	// Precondition: NPC must not already be charmed.
	if inst.Conditions != nil {
		if _, active := inst.Conditions.Get("charmed"); active {
			return fmt.Sprintf("%s is already charmed.", inst.Name()), nil
		}
	}
	// Precondition: NPC must not have previously rejected this player.
	if inst.SeductionRejected != nil && inst.SeductionRejected[uid] {
		return fmt.Sprintf("%s is not interested in your advances.", inst.Name()), nil
	}

	// Opposed skill check: player Flair vs NPC Savvy.
	var playerRoll, npcRoll int
	if s.dice != nil {
		playerRoll = s.dice.Src().Intn(20) + 1 + sess.Skills["flair"]
		npcRoll = s.dice.Src().Intn(20) + 1 + inst.Savvy
	} else {
		playerRoll = 10 + sess.Skills["flair"]
		npcRoll = 10 + inst.Savvy
	}

	if playerRoll >= npcRoll {
		// Player success: NPC gains charmed condition.
		if s.condRegistry != nil {
			if def, ok := s.condRegistry.Get("charmed"); ok && inst.Conditions != nil {
				inst.Conditions.Apply(def)
			}
		}
		return fmt.Sprintf("You charm %s with your winning smile. They seem... charmed.", inst.Name()), nil
	}

	// Player failure: NPC turns hostile and records rejection.
	inst.Disposition = "hostile"
	if inst.SeductionRejected == nil {
		inst.SeductionRejected = make(map[string]bool)
	}
	inst.SeductionRejected[uid] = true
	return fmt.Sprintf("%s rejects your advances and turns hostile!", inst.Name()), nil
}
```

- [ ] **Step 5: Wire up `seduce` as a player command**

Find the command dispatch in the gameserver (search for `"rob"` or `"taunt"` command handlers to find the pattern). Add `seduce` handling:

```bash
grep -n '"rob"\|"taunt"\|case "rob"\|handleRob\|handleTaunt' /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -20
```

In the command dispatch, add a case for "seduce":

```go
case "seduce":
	if len(args) < 1 {
		s.pushMessageToUID(uid, "Usage: seduce <npc_name>")
		return &gamev1.HandleCommandResponse{}, nil
	}
	// Find NPC in current room by name.
	target := strings.Join(args, " ")
	inst := s.findNPCInRoomByName(uid, target)
	if inst == nil {
		s.pushMessageToUID(uid, fmt.Sprintf("You don't see %q here.", target))
		return &gamev1.HandleCommandResponse{}, nil
	}
	msg, err := s.executeSeduce(sess, uid, inst)
	if err != nil {
		s.logger.Error("executeSeduce error", zap.String("uid", uid), zap.Error(err))
	}
	s.pushMessageToUID(uid, msg)
	return &gamev1.HandleCommandResponse{}, nil
```

Note: Use the existing `findNPCInRoomByName` or equivalent helper. Check what helper is used by `rob` or `taunt` and reuse it.

- [ ] **Step 6: Add charmed Savvy save at end of each round**

Find the round-end NPC processing in `internal/gameserver/combat_handler.go`. Search for where NPC conditions are ticked:

```bash
grep -n "AdvanceRound\|endOfRound\|RoundEnd\|npc.*tick\|charmed" /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go | head -20
```

In the round-end processing loop over NPC instances, add:

```go
// REQ-ZN-9: charmed NPCs make Savvy save vs DC 15 at round end.
if inst.Conditions != nil {
	if _, charmed := inst.Conditions.Get("charmed"); charmed {
		const charmedSaveDC = 15
		var roll int
		if h.dice != nil {
			roll = h.dice.Src().Intn(20) + 1
		} else {
			roll = 10
		}
		total := roll + inst.Savvy
		if total >= charmedSaveDC {
			inst.Conditions.Remove("charmed")
			// Notify players in room.
			for _, p := range h.sessionMgr.PlayersInRoom(inst.RoomID) {
				h.pushMessage(p.UID, fmt.Sprintf("%s snaps out of their charmed state.", inst.Name()))
			}
		}
	}
}
```

- [ ] **Step 7: Implement charmed allied treatment (REQ-ZN-10)**

Find the combat target selection code in `combat_handler.go` where NPCs choose attack targets. Search for the selection function:

```bash
grep -n "pickTarget\|selectTarget\|NearestEnemy\|Target.*player\|attack.*target" /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go | head -20
```

In the NPC target selection logic, add a check: if the NPC is charmed and the target player is the one who charmed them, skip that player:

```go
// REQ-ZN-10: charmed NPCs treat the charming player as allied; skip them.
if inst.Conditions != nil {
	if _, charmed := inst.Conditions.Get("charmed"); charmed {
		// Skip — charmed NPC does not attack any player (treat all as allied).
		// If more granular tracking is needed in the future, store the charming player UID.
		continue
	}
}
```

Note: The exact integration point depends on the combat target loop. Read the actual target selection code before making this change.

- [ ] **Step 8: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestSeduce|TestProperty_Seduce" -v 2>&1
```

Expected: PASS

- [ ] **Step 9: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
```

Expected: all PASS. Fix any compilation errors.

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service_seduce.go internal/gameserver/grpc_service_seduce_test.go internal/gameserver/grpc_service.go internal/gameserver/combat_handler.go
git commit -m "feat(zones): add seduce command, charmed condition save, and allied treatment (REQ-ZN-7–10)"
```

---

## Final Verification

- [ ] **Run full test suite one final time**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Build server binary to verify compilation**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors

- [ ] **Run linter**

```bash
cd /home/cjohannsen/src/mud && go vet ./... 2>&1
```

Expected: no warnings

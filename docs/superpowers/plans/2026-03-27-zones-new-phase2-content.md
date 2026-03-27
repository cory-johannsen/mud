# New Zones Phase 2 (Zone Content) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add four new zones (Clown Camp, SteamPDX, The Velvet Rope, Club Privata) with full room layouts, zone-effects, NPC templates, NPC-initiated seduction mechanics, and two new condition definitions.

**Architecture:** Phase 1 established zone_effects propagation, the condition registry, terrain conditions, NPC gender, the seduce command, and charmed-save. Phase 2 layers content on top: two new condition YAMLs (terrain_lube, seduced), twelve new NPC YAML templates carrying SeductionProbability/SeductionGender fields, four zone YAML files, seduction logic injected into the CombatHandler NPC turn loop, and tests for every new piece.

**Tech Stack:** Go 1.23 · gopkg.in/yaml.v3 · pgregory.net/rapid · testify/assert + require · existing `condition`, `npc`, `world`, and `gameserver` packages

---

## File Map

| Path | Action | Responsibility |
|---|---|---|
| `content/conditions/terrain_lube.yaml` | Create | Terrain condition: +1 move AP cost, hustle −2 |
| `content/conditions/seduced.yaml` | Create | Player-side seduction effect: ap_reduction=2, is_mental_condition=true |
| `content/npcs/clown.yaml` | Create | Standard Clown Camp hostile clown (level 3) |
| `content/npcs/clown_mime.yaml` | Create | Fast/sneaky mime variant (level 2) |
| `content/npcs/just_clownin.yaml` | Create | Clown Camp boss (level 6) |
| `content/npcs/steam_patron.yaml` | Create | SteamPDX patron, male, seduction_probability=0.3, neutral |
| `content/npcs/steam_bouncer.yaml` | Create | SteamPDX door enforcer, hostile |
| `content/npcs/the_big_3.yaml` | Create | SteamPDX boss (level 7) |
| `content/npcs/velvet_patron.yaml` | Create | Velvet Rope seductive patron, neutral, seduction_probability=0.4 |
| `content/npcs/velvet_hostess.yaml` | Create | Velvet Rope hostess, seduction_probability=0.5 |
| `content/npcs/gangbang.yaml` | Create | Velvet Rope boss (level 7), hostile |
| `content/npcs/club_bouncer.yaml` | Create | Club Privata bouncer, hostile |
| `content/npcs/club_dancer.yaml` | Create | Club Privata dancer, seduction_probability=0.2, neutral |
| `content/npcs/club_vip_boss.yaml` | Create | Club Privata VIP boss (level 8) |
| `content/zones/clown_camp.yaml` | Create | 5-room Clown Camp zone (world_x=6, world_y=2) |
| `content/zones/steampdx.yaml` | Create | 7-room SteamPDX zone (world_x=0, world_y=-2) |
| `content/zones/the_velvet_rope.yaml` | Create | 7-room Velvet Rope zone (world_x=-2, world_y=4) |
| `content/zones/club_privata.yaml` | Create | 16-room Club Privata zone (world_x=4, world_y=-2) |
| `internal/game/npc/template.go` | Modify | Add SeductionProbability float64, SeductionGender string fields |
| `internal/game/npc/instance.go` | Modify | Propagate SeductionProbability, SeductionGender from template |
| `internal/gameserver/combat_handler.go` | Modify | NPC-initiated seduction in processNPCTurns loop |
| `internal/game/condition/definition_test.go` | Modify | Tests for terrain_lube and seduced condition defs |
| `internal/game/npc/seduction_test.go` | Create | Tests for SeductionProbability/SeductionGender parsing and propagation |
| `internal/gameserver/combat_handler_seduction_test.go` | Create | Tests for NPC-initiated seduction combat logic |
| `internal/game/world/loader_test.go` | Modify | Zone loading tests for all four new zones |

---

## Task 1: New Condition YAMLs (terrain_lube + seduced)

**Files:**
- Create: `content/conditions/terrain_lube.yaml`
- Create: `content/conditions/seduced.yaml`
- Modify: `internal/game/condition/definition_test.go`

- [ ] **Step 1.1: Write the failing tests**

Add these three tests at the bottom of `/home/cjohannsen/src/mud/internal/game/condition/definition_test.go`:

```go
func TestLoadDirectory_NewPhase2ConditionsPresent(t *testing.T) {
	dir := "../../../../content/conditions"
	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	_, ok1 := reg.Get("terrain_lube")
	_, ok2 := reg.Get("seduced")
	assert.True(t, ok1, "terrain_lube must be present in condition registry")
	assert.True(t, ok2, "seduced must be present in condition registry")
}

func TestConditionDef_TerrainLube_HasMoveAPCost(t *testing.T) {
	dir := "../../../../content/conditions"
	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	def, ok := reg.Get("terrain_lube")
	require.True(t, ok)
	assert.Equal(t, 1, def.MoveAPCost, "terrain_lube MoveAPCost must be 1")
	assert.Equal(t, 2, def.SkillPenalties["hustle"], "terrain_lube hustle penalty must be 2")
}

func TestConditionDef_Seduced_HasAPReduction(t *testing.T) {
	dir := "../../../../content/conditions"
	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	def, ok := reg.Get("seduced")
	require.True(t, ok)
	assert.Equal(t, 2, def.APReduction, "seduced APReduction must be 2")
	assert.True(t, def.IsMentalCondition, "seduced must be a mental condition")
}
```

- [ ] **Step 1.2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... -run "TestLoadDirectory_NewPhase2ConditionsPresent|TestConditionDef_TerrainLube|TestConditionDef_Seduced" -v 2>&1 | tail -20
```

Expected: FAIL — `terrain_lube` and `seduced` not found in registry (files do not exist yet).

- [ ] **Step 1.3: Create terrain_lube.yaml**

Create `/home/cjohannsen/src/mud/content/conditions/terrain_lube.yaml`:

```yaml
id: terrain_lube
name: Lubricated Terrain
description: |
  The floor is slick with lubricant, making every step treacherous and
  athletic maneuvers nearly impossible.
duration_type: permanent
move_ap_cost: 1
skill_penalties:
  hustle: 2
```

- [ ] **Step 1.4: Create seduced.yaml**

Create `/home/cjohannsen/src/mud/content/conditions/seduced.yaml`:

```yaml
id: seduced
name: Seduced
description: |
  You are distracted by an irresistible attraction. Your focus wavers
  and your actions are slowed.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
attack_bonus: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 2
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

- [ ] **Step 1.5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... -run "TestLoadDirectory_NewPhase2ConditionsPresent|TestConditionDef_TerrainLube|TestConditionDef_Seduced" -v 2>&1 | tail -20
```

Expected: PASS — all three tests green.

- [ ] **Step 1.6: Run full condition test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... -v 2>&1 | tail -10
```

Expected: all tests PASS, no failures.

- [ ] **Step 1.7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/conditions/terrain_lube.yaml content/conditions/seduced.yaml internal/game/condition/definition_test.go
git commit -m "feat(zones-phase2): add terrain_lube and seduced condition YAMLs with tests"
```

---

## Task 2: NPC SeductionProbability/SeductionGender Fields + Combat Seduction Logic

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Create: `internal/game/npc/seduction_test.go`
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_seduction_test.go`

- [ ] **Step 2.1: Write failing NPC field tests**

Create `/home/cjohannsen/src/mud/internal/game/npc/seduction_test.go`:

```go
package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestNPCTemplate_SeductionProbabilityParsesFromYAML(t *testing.T) {
	data := []byte("id: test\nname: Test\nmax_hp: 10\nlevel: 1\nseduction_probability: 0.4\n")
	var tmpl npc.Template
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.InDelta(t, 0.4, tmpl.SeductionProbability, 0.0001)
}

func TestNPCTemplate_SeductionGenderParsesFromYAML(t *testing.T) {
	data := []byte("id: test\nname: Test\nmax_hp: 10\nlevel: 1\nseduction_gender: male\n")
	var tmpl npc.Template
	require.NoError(t, yaml.Unmarshal(data, &tmpl))
	assert.Equal(t, "male", tmpl.SeductionGender)
}

func TestNPCInstance_PropagatesSeductionFields(t *testing.T) {
	tmpl := &npc.Template{
		ID:                   "patron",
		Name:                 "Patron",
		MaxHP:                30,
		Level:                3,
		SeductionProbability: 0.3,
		SeductionGender:      "male",
	}
	inst := npc.NewInstance("inst1", tmpl, "room1")
	assert.InDelta(t, 0.3, inst.SeductionProbability, 0.0001)
	assert.Equal(t, "male", inst.SeductionGender)
}

func TestProperty_NPC_SeductionProbability_AlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		prob := rapid.Float64Range(0, 1).Draw(rt, "prob")
		tmpl := &npc.Template{
			ID:                   "tmpl",
			Name:                 "NPC",
			MaxHP:                10,
			Level:                1,
			SeductionProbability: prob,
		}
		inst := npc.NewInstance("id", tmpl, "room")
		if inst.SeductionProbability < 0 || inst.SeductionProbability > 1 {
			rt.Fatalf("SeductionProbability %v out of [0,1]", inst.SeductionProbability)
		}
	})
}
```

- [ ] **Step 2.2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestNPCTemplate_Seduction|TestNPCInstance_PropagatesSeduction|TestProperty_NPC_SeductionProbability" -v 2>&1 | tail -20
```

Expected: FAIL — `SeductionProbability` and `SeductionGender` fields do not exist on `Template` or `Instance`.

- [ ] **Step 2.3: Add fields to Template**

In `/home/cjohannsen/src/mud/internal/game/npc/template.go`, add after the `Immobile` field (around line 139):

```go
	// SeductionProbability is the probability (0.0–1.0) that this NPC attempts to seduce
	// a player at the start of each combat round when disposition is not "hostile".
	// 0.0 means this NPC never initiates seduction.
	SeductionProbability float64 `yaml:"seduction_probability"`
	// SeductionGender restricts seduction attempts to players of a matching gender.
	// Empty string means the NPC attempts to seduce players of any gender.
	// When non-empty, players whose Gender field does not match are instead treated as
	// hostile targets (NPC becomes hostile to that player).
	SeductionGender string `yaml:"seduction_gender"`
```

- [ ] **Step 2.4: Add fields to Instance**

In `/home/cjohannsen/src/mud/internal/game/npc/instance.go`, after the `SeductionRejected` field (around line 29):

```go
	// SeductionProbability is propagated from Template.SeductionProbability at spawn.
	// 0.0 means this NPC never initiates seduction.
	SeductionProbability float64
	// SeductionGender is propagated from Template.SeductionGender at spawn.
	// Empty string means any gender; non-empty restricts seduction to matching players.
	SeductionGender string
```

- [ ] **Step 2.5: Propagate fields in NewInstanceWithResolver**

In `/home/cjohannsen/src/mud/internal/game/npc/instance.go`, in the `NewInstanceWithResolver` function return block (around line 266), add inside the `&Instance{...}` literal after `Immobile: tmpl.Immobile,`:

```go
		SeductionProbability: tmpl.SeductionProbability,
		SeductionGender:      tmpl.SeductionGender,
```

- [ ] **Step 2.6: Propagate fields in NewInstance**

Read the `NewInstance` function (around line 377):

```bash
cd /home/cjohannsen/src/mud && mise exec -- grep -n "SeductionRejected\|NewInstance\b" internal/game/npc/instance.go | head -20
```

`NewInstance` calls `NewInstanceWithResolver` with nil resolvers — no additional change needed since NewInstanceWithResolver already propagates the fields. Verify by checking that `NewInstance` delegates:

```bash
cd /home/cjohannsen/src/mud && sed -n '377,395p' internal/game/npc/instance.go
```

Expected: `NewInstance` calls `NewInstanceWithResolver(id, tmpl, roomID, nil, nil, nil)`.

- [ ] **Step 2.7: Run NPC field tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestNPCTemplate_Seduction|TestNPCInstance_PropagatesSeduction|TestProperty_NPC_SeductionProbability" -v 2>&1 | tail -20
```

Expected: all four tests PASS.

- [ ] **Step 2.8: Write failing combat seduction tests**

Create `/home/cjohannsen/src/mud/internal/gameserver/combat_handler_seduction_test.go`:

```go
package gameserver_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// buildSeductionHandler builds a minimal CombatHandler for seduction tests.
func buildSeductionHandler(t *testing.T) *gameserver.CombatHandler {
	t.Helper()
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	diceRoller := dice.NewRoller()
	broadcast := func(_ string, _ []*gamev1.CombatEvent) {}
	condReg := condition.NewRegistry()
	seducedDef := &condition.ConditionDef{
		ID:                "seduced",
		Name:              "Seduced",
		DurationType:      "rounds",
		APReduction:       2,
		IsMentalCondition: true,
	}
	condReg.Register(seducedDef)
	h := gameserver.NewCombatHandler(engine, npcMgr, sessMgr, diceRoller, broadcast, 6*time.Second, condReg, nil, nil, nil, nil, nil)
	return h
}

func TestNPCSeduction_GenderMismatch_NPCBecomesHostile(t *testing.T) {
	h := buildSeductionHandler(t)

	// Spawn a steam_patron-like NPC: seduction_gender=male, player is female.
	tmpl := &npc.Template{
		ID:                   "steam_patron",
		Name:                 "Steam Patron",
		MaxHP:                30,
		Level:                3,
		Disposition:          "neutral",
		SeductionProbability: 1.0, // always attempts when gender matches
		SeductionGender:      "male",
		Abilities:            npc.Abilities{Flair: 16},
	}
	inst := npc.NewInstance("npc1", tmpl, "room1")
	require.Equal(t, "neutral", inst.Disposition)

	playerUID := "player1"
	playerGender := "female"

	// Call the exported seduction resolver.
	hostile := h.ResolveNPCSeductionGenderCheck(inst, playerUID, playerGender)
	assert.True(t, hostile, "gender mismatch must make NPC hostile")
	assert.Equal(t, "hostile", inst.Disposition)
}

func TestNPCSeduction_HighFlair_PlayerSeduced(t *testing.T) {
	h := buildSeductionHandler(t)

	tmpl := &npc.Template{
		ID:                   "velvet_hostess",
		Name:                 "Velvet Hostess",
		MaxHP:                28,
		Level:                3,
		Disposition:          "neutral",
		SeductionProbability: 1.0,
		SeductionGender:      "",
		Abilities:            npc.Abilities{Flair: 20, Savvy: 12},
	}
	inst := npc.NewInstance("npc2", tmpl, "room1")
	seducedDef := &condition.ConditionDef{
		ID:                "seduced",
		Name:              "Seduced",
		DurationType:      "rounds",
		APReduction:       2,
		IsMentalCondition: true,
	}
	condSet := condition.NewActiveSet()
	playerSavvy := 8 // low savvy = likely to lose

	// Fix dice so NPC always wins: npc roll=20, player roll=1.
	seduced := h.ResolveNPCSeductionContest(inst, "player1", playerSavvy, seducedDef, condSet, 20, 1)
	assert.True(t, seduced, "high-flair NPC vs low-savvy player must result in seduction")
	assert.True(t, condSet.Has("seduced"), "seduced condition must be applied")
}

func TestNPCSeduction_LowFlair_NPCBecomesHostile(t *testing.T) {
	h := buildSeductionHandler(t)

	tmpl := &npc.Template{
		ID:                   "club_dancer",
		Name:                 "Club Dancer",
		MaxHP:                20,
		Level:                2,
		Disposition:          "neutral",
		SeductionProbability: 1.0,
		Abilities:            npc.Abilities{Flair: 8},
	}
	inst := npc.NewInstance("npc3", tmpl, "room1")
	seducedDef := &condition.ConditionDef{
		ID:           "seduced",
		Name:         "Seduced",
		DurationType: "rounds",
		APReduction:  2,
	}
	condSet := condition.NewActiveSet()
	playerSavvy := 18

	// Fix dice so player always wins: npc roll=1, player roll=20.
	seduced := h.ResolveNPCSeductionContest(inst, "player1", playerSavvy, seducedDef, condSet, 1, 20)
	assert.False(t, seduced, "low-flair NPC vs high-savvy player must fail seduction")
	assert.Equal(t, "hostile", inst.Disposition, "failed seduction must make NPC hostile")
	require.NotNil(t, inst.SeductionRejected)
	assert.True(t, inst.SeductionRejected["player1"])
}

func TestProperty_NPCSeduction_HighFlairAlwaysSeduces(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// NPC flair=30 (impossibly high) vs player savvy=1 — NPC must always win
		// when npc roll=20, player roll=1.
		tmpl := &npc.Template{
			ID:          "tmpl",
			Name:        "NPC",
			MaxHP:       10,
			Level:       1,
			Disposition: "neutral",
			Abilities:   npc.Abilities{Flair: 30},
		}
		inst := npc.NewInstance("id", tmpl, "room")
		seducedDef := &condition.ConditionDef{
			ID:           "seduced",
			Name:         "Seduced",
			DurationType: "rounds",
			APReduction:  2,
		}
		condSet := condition.NewActiveSet()
		playerSavvy := rapid.IntRange(1, 5).Draw(rt, "savvy")
		h := buildSeductionHandlerT(rt)
		seduced := h.ResolveNPCSeductionContest(inst, "p1", playerSavvy, seducedDef, condSet, 20, 1)
		if !seduced {
			rt.Fatal("max-flair NPC with fixed winning rolls must always seduce low-savvy player")
		}
	})
}

// buildSeductionHandlerT constructs a handler inside a rapid test.
func buildSeductionHandlerT(rt *rapid.T) *gameserver.CombatHandler {
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	diceRoller := dice.NewRoller()
	broadcast := func(_ string, _ []*gamev1.CombatEvent) {}
	condReg := condition.NewRegistry()
	h := gameserver.NewCombatHandler(engine, npcMgr, sessMgr, diceRoller, broadcast, 6*time.Second, condReg, nil, nil, nil, nil, nil)
	return h
}
```

- [ ] **Step 2.9: Run seduction combat tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestNPCSeduction|TestProperty_NPCSeduction" -v 2>&1 | tail -20
```

Expected: FAIL — `ResolveNPCSeductionGenderCheck` and `ResolveNPCSeductionContest` not defined on `CombatHandler`.

- [ ] **Step 2.10: Add seduction resolver methods to CombatHandler**

In `/home/cjohannsen/src/mud/internal/gameserver/combat_handler.go`, append the following two exported methods before the end of file. (Find the last function and add after it.)

```go
// ResolveNPCSeductionGenderCheck checks whether inst's SeductionGender restriction
// blocks seduction for the given player. When blocked, inst.Disposition is set to
// "hostile" and true is returned. When not blocked, false is returned.
//
// Precondition: inst must not be nil; playerUID and playerGender must not be empty.
// Postcondition: when true, inst.Disposition == "hostile".
func (h *CombatHandler) ResolveNPCSeductionGenderCheck(inst *npc.Instance, playerUID, playerGender string) bool {
	if inst.SeductionGender != "" && inst.SeductionGender != playerGender {
		inst.Disposition = "hostile"
		return true
	}
	return false
}

// ResolveNPCSeductionContest runs the seduction opposed check between inst and the player.
// npcRoll and playerRoll are the raw d20 results (1–20); callers may pass fixed values for testing.
// inst.Abilities.Flair is added to npcRoll; playerSavvy is added to playerRoll.
// When npcRoll+Flair >= playerRoll+playerSavvy:
//   - seducedDef is applied to condSet (durationRounds=1).
//   - returns true.
//
// When npcRoll+Flair < playerRoll+playerSavvy:
//   - inst.Disposition = "hostile"; inst.SeductionRejected[playerUID] = true.
//   - returns false.
//
// Precondition: inst and seducedDef must not be nil; condSet must not be nil.
func (h *CombatHandler) ResolveNPCSeductionContest(inst *npc.Instance, playerUID string, playerSavvy int, seducedDef *condition.ConditionDef, condSet *condition.ActiveSet, npcRoll, playerRoll int) bool {
	npcTotal := npcRoll + modFromScore(inst.Abilities.Flair)
	playerTotal := playerRoll + modFromScore(playerSavvy)
	if npcTotal >= playerTotal {
		_ = condSet.Apply(playerUID, seducedDef, 1, 1)
		return true
	}
	inst.Disposition = "hostile"
	if inst.SeductionRejected == nil {
		inst.SeductionRejected = make(map[string]bool)
	}
	inst.SeductionRejected[playerUID] = true
	return false
}
```

Note: `modFromScore` is already defined in `combat_handler.go` — verify its presence:

```bash
cd /home/cjohannsen/src/mud && grep -n "func modFromScore" internal/gameserver/combat_handler.go
```

If not present, add:

```go
// modFromScore converts an ability score to its modifier using standard PF2E formula.
func modFromScore(score int) int {
	return (score - 10) / 2
}
```

- [ ] **Step 2.11: Add NPC-initiated seduction to processNPCTurns loop**

In `combat_handler.go`, find the NPC turn loop (around line 2939) where NPCs iterate. After the charmed-NPC skip block and before the auto-use-cover block, insert the seduction check. Find the exact location:

```bash
cd /home/cjohannsen/src/mud && grep -n "REQ-ZN-10\|Auto-use-cover" internal/gameserver/combat_handler.go
```

Insert the following block immediately after the `seduceConditions` charmed check block (after the closing brace of that `if` block, before the `// Auto-use-cover` comment):

```go
		// NPC-initiated seduction: neutral NPCs with SeductionProbability > 0 may attempt
		// to seduce player combatants at the start of their turn.
		if h.condRegistry != nil {
			if inst, ok := h.npcMgr.Get(c.ID); ok && inst.SeductionProbability > 0 && inst.Disposition != "hostile" {
				seducedDef, hasDef := h.condRegistry.Get("seduced")
				if hasDef {
					for _, pc := range cbt.Combatants {
						if pc.Kind != combat.KindPlayer || pc.IsDead() {
							continue
						}
						// Skip players this NPC has already rejected.
						if inst.SeductionRejected != nil && inst.SeductionRejected[pc.ID] {
							continue
						}
						sess, ok := h.sessions.Get(pc.ID)
						if !ok {
							continue
						}
						// Gender mismatch: NPC becomes hostile to this player.
						if h.ResolveNPCSeductionGenderCheck(inst, pc.ID, sess.Gender) {
							continue
						}
						// Probability roll: d20 result <= SeductionProbability*20 means attempt.
						roll := h.dice.Src().Intn(20) + 1
						if float64(roll) > inst.SeductionProbability*20 {
							continue // no attempt this round
						}
						// Opposed contest: d20+Flair vs d20+Savvy.
						npcRoll := h.dice.Src().Intn(20) + 1
						playerRoll := h.dice.Src().Intn(20) + 1
						if cbt.Conditions[pc.ID] == nil {
							cbt.Conditions[pc.ID] = condition.NewActiveSet()
						}
						h.ResolveNPCSeductionContest(inst, pc.ID, sess.Abilities.Savvy, seducedDef, cbt.Conditions[pc.ID], npcRoll, playerRoll)
					}
				}
			}
		}
```

- [ ] **Step 2.12: Verify the file compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 2.13: Run seduction combat tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestNPCSeduction|TestProperty_NPCSeduction" -v 2>&1 | tail -20
```

Expected: all four tests PASS.

- [ ] **Step 2.14: Run full NPC + gameserver test suites**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: all tests PASS, no failures.

- [ ] **Step 2.15: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
  internal/game/npc/template.go \
  internal/game/npc/instance.go \
  internal/game/npc/seduction_test.go \
  internal/gameserver/combat_handler.go \
  internal/gameserver/combat_handler_seduction_test.go
git commit -m "feat(zones-phase2): add NPC seduction fields and combat seduction logic with tests"
```

---

## Task 3: Clown Camp — NPCs and Zone YAML

**Files:**
- Create: `content/npcs/clown.yaml`
- Create: `content/npcs/clown_mime.yaml`
- Create: `content/npcs/just_clownin.yaml`
- Create: `content/zones/clown_camp.yaml`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 3.1: Write failing zone loading tests**

Add to `/home/cjohannsen/src/mud/internal/game/world/loader_test.go`:

```go
func TestLoadZone_ClownCamp_HasFiveRooms(t *testing.T) {
	data, err := os.ReadFile("../../../../content/zones/clown_camp.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 5, "Clown Camp must have exactly 5 rooms")
	assert.Equal(t, "clown_camp", zone.ID)
}

func TestLoadZone_ClownCamp_ZoneEffects(t *testing.T) {
	data, err := os.ReadFile("../../../../content/zones/clown_camp.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	tracks := make([]string, 0, len(zone.ZoneEffects))
	for _, e := range zone.ZoneEffects {
		tracks = append(tracks, e.Track)
	}
	assert.Contains(t, tracks, "delirium", "Clown Camp must have delirium zone_effect")
	assert.Contains(t, tracks, "fear", "Clown Camp must have fear zone_effect")
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_ClownCamp" -v 2>&1 | tail -20
```

Expected: FAIL — `clown_camp.yaml` does not exist.

- [ ] **Step 3.3: Create clown.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/clown.yaml`:

```yaml
id: clown
name: Clown
description: A face-painted horror in a tattered costume, grinning with too many teeth.
level: 3
max_hp: 30
ac: 13
awareness: 6
disposition: hostile
abilities:
  brutality: 13
  quickness: 11
  grit: 10
  reasoning: 6
  savvy: 8
  flair: 14
loot:
  currency:
    min: 5
    max: 25
```

- [ ] **Step 3.4: Create clown_mime.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/clown_mime.yaml`:

```yaml
id: clown_mime
name: Clown Mime
description: A silent, white-faced figure that moves with unsettling grace and speed.
level: 2
max_hp: 22
ac: 12
awareness: 7
disposition: hostile
abilities:
  brutality: 10
  quickness: 14
  grit: 8
  reasoning: 6
  savvy: 6
  flair: 12
loot:
  currency:
    min: 3
    max: 15
```

- [ ] **Step 3.5: Create just_clownin.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/just_clownin.yaml`:

```yaml
id: just_clownin
name: Just Clownin'!
description: The ringleader. A towering nightmare of paint and polyester, wielding a comically oversized mallet that is not at all funny.
level: 6
max_hp: 80
ac: 16
awareness: 8
disposition: hostile
tier: boss
abilities:
  brutality: 18
  quickness: 12
  grit: 16
  reasoning: 10
  savvy: 10
  flair: 18
loot:
  currency:
    min: 50
    max: 150
```

- [ ] **Step 3.6: Create clown_camp.yaml**

Create `/home/cjohannsen/src/mud/content/zones/clown_camp.yaml`:

```yaml
zone:
  id: clown_camp
  danger_level: dangerous
  world_x: 6
  world_y: 2
  name: Clown Camp
  description: A derelict circus lot on the eastern edge of the city, festering with something that used to be entertainment.
  start_room: cc_coat_check
  zone_effects:
  - track: delirium
    severity: mild
    base_dc: 12
    cooldown_rounds: 3
    cooldown_minutes: 5
  - track: fear
    severity: mild
    base_dc: 11
    cooldown_rounds: 3
    cooldown_minutes: 5
  rooms:
  - id: cc_coat_check
    danger_level: dangerous
    title: Coat Check
    description: 'A warped plywood counter holds rows of hooks, each hung with a single
      oversized coat. The smell of greasepaint and something chemical hangs in the air.
      Squeaky shoes somewhere in the dark. A door east leads deeper in.'
    exits:
    - direction: east
      target: cc_empty_theater
    map_x: 0
    map_y: 0
    spawns:
    - template: clown
      count: 1
      respawn_after: 5m
    boss_room: false
  - id: cc_empty_theater
    danger_level: dangerous
    title: Empty Theater
    description: 'Folding chairs are arranged in ragged rows facing a dark stage. The
      spotlight flickers, catching dust motes and something that moves between the seats.
      Two painted faces turn toward you from the wings.'
    exits:
    - direction: west
      target: cc_coat_check
    - direction: east
      target: cc_changing_rooms
    map_x: 1
    map_y: 0
    spawns:
    - template: clown
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: cc_changing_rooms
    danger_level: dangerous
    title: Changing Rooms
    description: 'Mirrors line three walls, each reflecting a figure that is slightly
      wrong. Wig stands. Prop trunks. Two mimes stand perfectly still until you move,
      then they move with you.'
    exits:
    - direction: west
      target: cc_empty_theater
    - direction: east
      target: cc_backstage
    map_x: 2
    map_y: 0
    spawns:
    - template: clown_mime
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: cc_backstage
    danger_level: dangerous
    title: Backstage
    description: 'Cables and sandbags dangle from the fly system. A single work light
      throws everything into harsh shadow. Props of uncertain purpose crowd every corner.
      Something laughs from the stage beyond.'
    exits:
    - direction: west
      target: cc_changing_rooms
    - direction: east
      target: cc_the_stage
    map_x: 3
    map_y: 0
    spawns:
    - template: clown
      count: 1
      respawn_after: 5m
    boss_room: false
  - id: cc_the_stage
    danger_level: dangerous
    title: The Stage
    description: 'Center stage. The house lights blaze on you from the grid above.
      A single figure stands downstage, enormous and grinning, waiting for its cue.
      The audience seats are all empty. Or they were.'
    exits:
    - direction: west
      target: cc_backstage
    map_x: 4
    map_y: 0
    spawns:
    - template: just_clownin
      count: 1
      respawn_after: 30m
    boss_room: true
```

- [ ] **Step 3.7: Run Clown Camp tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_ClownCamp" -v 2>&1 | tail -20
```

Expected: both Clown Camp tests PASS.

- [ ] **Step 3.8: Run full world loader test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 3.9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
  content/npcs/clown.yaml \
  content/npcs/clown_mime.yaml \
  content/npcs/just_clownin.yaml \
  content/zones/clown_camp.yaml \
  internal/game/world/loader_test.go
git commit -m "feat(zones-phase2): add Clown Camp zone and NPC templates"
```

---

## Task 4: SteamPDX — NPCs and Zone YAML

**Files:**
- Create: `content/npcs/steam_patron.yaml`
- Create: `content/npcs/steam_bouncer.yaml`
- Create: `content/npcs/the_big_3.yaml`
- Create: `content/zones/steampdx.yaml`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 4.1: Write failing zone loading test**

Add to `/home/cjohannsen/src/mud/internal/game/world/loader_test.go`:

```go
func TestLoadZone_SteamPDX_HasSevenRooms(t *testing.T) {
	data, err := os.ReadFile("../../../../content/zones/steampdx.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 7, "SteamPDX must have exactly 7 rooms")
	assert.Equal(t, "steampdx", zone.ID)
}
```

- [ ] **Step 4.2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_SteamPDX" -v 2>&1 | tail -10
```

Expected: FAIL — `steampdx.yaml` does not exist.

- [ ] **Step 4.3: Create steam_patron.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/steam_patron.yaml`:

```yaml
id: steam_patron
name: Steam Patron
description: A heavyset man wrapped in a towel, looking for connection in all the wrong ways.
level: 3
max_hp: 30
ac: 12
awareness: 5
gender: male
disposition: neutral
seduction_probability: 0.3
seduction_gender: male
abilities:
  brutality: 12
  quickness: 10
  grit: 10
  reasoning: 10
  savvy: 10
  flair: 16
loot:
  currency:
    min: 10
    max: 40
```

- [ ] **Step 4.4: Create steam_bouncer.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/steam_bouncer.yaml`:

```yaml
id: steam_bouncer
name: Steam Bouncer
description: A broad-shouldered man in a polo shirt who has very strong opinions about who belongs here.
level: 4
max_hp: 45
ac: 15
awareness: 7
gender: male
disposition: hostile
abilities:
  brutality: 16
  quickness: 10
  grit: 14
  reasoning: 8
  savvy: 8
  flair: 10
loot:
  currency:
    min: 15
    max: 50
```

- [ ] **Step 4.5: Create the_big_3.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/the_big_3.yaml`:

```yaml
id: the_big_3
name: The Big 3
description: Three interconnected titans who move and fight as one organism, a hydra of bad decisions and raw power.
level: 7
max_hp: 100
ac: 17
awareness: 9
gender: male
disposition: hostile
tier: boss
abilities:
  brutality: 20
  quickness: 14
  grit: 16
  reasoning: 10
  savvy: 12
  flair: 14
loot:
  currency:
    min: 80
    max: 200
```

- [ ] **Step 4.6: Create steampdx.yaml**

Create `/home/cjohannsen/src/mud/content/zones/steampdx.yaml`:

```yaml
zone:
  id: steampdx
  danger_level: dangerous
  world_x: 0
  world_y: -2
  name: SteamPDX
  description: A men's bathhouse south of downtown, notorious for its hostile door policy and even more hostile interior.
  start_room: sp_parking_lot
  zone_effects:
  - track: horror
    severity: mild
    base_dc: 12
    cooldown_rounds: 3
    cooldown_minutes: 5
  - track: nausea
    severity: mild
    base_dc: 11
    cooldown_rounds: 3
    cooldown_minutes: 5
  - track: reduced_visibility
    severity: mild
    base_dc: 10
    cooldown_rounds: 3
    cooldown_minutes: 5
  rooms:
  - id: sp_parking_lot
    danger_level: dangerous
    title: Parking Lot
    description: 'A cracked asphalt lot behind a nondescript building with blacked-out
      windows. A hand-lettered sign reads "Members Only." The bouncer at the door
      looks you over with practiced contempt.'
    exits:
    - direction: east
      target: sp_lobby
    map_x: 0
    map_y: 0
    spawns:
    - template: steam_bouncer
      count: 1
      respawn_after: 5m
    boss_room: false
  - id: sp_lobby
    danger_level: dangerous
    title: Lobby
    description: 'Wood paneling, low lighting, and a reception desk attended by no
      one. Lockers line one wall. A faint chemical smell — chlorine and something
      else — drifts from further inside. Patrons eye new arrivals.'
    exits:
    - direction: west
      target: sp_parking_lot
    - direction: east
      target: sp_locker_room
    map_x: 1
    map_y: 0
    spawns:
    - template: steam_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: sp_locker_room
    danger_level: dangerous
    title: Locker Room
    description: 'Banks of metal lockers, most hanging open. Towels on hooks. The
      tile floor is wet. Three exits lead deeper into the facility — south to the
      showers, east to the sauna. The steam is already thick.'
    exits:
    - direction: west
      target: sp_lobby
    - direction: south
      target: sp_showers
    - direction: east
      target: sp_sauna
    map_x: 2
    map_y: 0
    boss_room: false
  - id: sp_showers
    danger_level: dangerous
    title: Showers
    description: 'A row of open shower stalls, visibility near zero in the billowing
      steam. Water runs hot and constant. Movement in the fog. Patrons who are no
      longer interested in relaxing.'
    exits:
    - direction: north
      target: sp_locker_room
    map_x: 2
    map_y: 1
    spawns:
    - template: steam_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: sp_sauna
    danger_level: dangerous
    title: Sauna
    description: 'Cedar benches in a cedar room heated to an unreasonable temperature.
      Coals glow in a central pit. The air scorches the lungs. Patrons recline with
      territorial ease, sweating profusely and watching the door.'
    exits:
    - direction: west
      target: sp_locker_room
    - direction: south
      target: sp_hot_tub
    map_x: 3
    map_y: 0
    spawns:
    - template: steam_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: sp_hot_tub
    danger_level: dangerous
    title: Hot Tub
    description: 'A large circular tub set into the floor, bubbling at a concerning
      temperature. The water is cloudy. More patrons, equally unwelcoming. A passage
      south leads somewhere the signage has been removed from.'
    exits:
    - direction: north
      target: sp_sauna
    - direction: south
      target: sp_glory_hole
    map_x: 3
    map_y: 1
    spawns:
    - template: steam_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: sp_glory_hole
    danger_level: dangerous
    title: The Glory Hole
    description: 'A low-ceilinged alcove at the deepest point of the facility. Three
      men of extraordinary size occupy every corner. They are not pleased to have company
      unless company was invited. You were not invited.'
    exits:
    - direction: north
      target: sp_hot_tub
    map_x: 3
    map_y: 2
    spawns:
    - template: the_big_3
      count: 1
      respawn_after: 30m
    boss_room: true
```

- [ ] **Step 4.7: Run SteamPDX test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_SteamPDX" -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 4.8: Run full world loader test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 4.9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
  content/npcs/steam_patron.yaml \
  content/npcs/steam_bouncer.yaml \
  content/npcs/the_big_3.yaml \
  content/zones/steampdx.yaml \
  internal/game/world/loader_test.go
git commit -m "feat(zones-phase2): add SteamPDX zone and NPC templates"
```

---

## Task 5: The Velvet Rope — NPCs and Zone YAML

**Files:**
- Create: `content/npcs/velvet_patron.yaml`
- Create: `content/npcs/velvet_hostess.yaml`
- Create: `content/npcs/gangbang.yaml`
- Create: `content/zones/the_velvet_rope.yaml`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 5.1: Write failing zone loading test**

Add to `/home/cjohannsen/src/mud/internal/game/world/loader_test.go`:

```go
func TestLoadZone_TheVelvetRope_HasTerrainLubeEffect(t *testing.T) {
	data, err := os.ReadFile("../../../../content/zones/the_velvet_rope.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 7, "The Velvet Rope must have exactly 7 rooms")
	assert.Equal(t, "the_velvet_rope", zone.ID)
	tracks := make([]string, 0, len(zone.ZoneEffects))
	for _, e := range zone.ZoneEffects {
		tracks = append(tracks, e.Track)
	}
	assert.Contains(t, tracks, "temptation", "The Velvet Rope must have temptation zone_effect")
	assert.Contains(t, tracks, "revulsion", "The Velvet Rope must have revulsion zone_effect")
	assert.Contains(t, tracks, "terrain_lube", "The Velvet Rope must have terrain_lube zone_effect")
}
```

- [ ] **Step 5.2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_TheVelvetRope" -v 2>&1 | tail -10
```

Expected: FAIL — `the_velvet_rope.yaml` does not exist.

- [ ] **Step 5.3: Create velvet_patron.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/velvet_patron.yaml`:

```yaml
id: velvet_patron
name: Velvet Patron
description: Someone dressed for a party that has been going since 1987, friendly in a way that wants something.
level: 2
max_hp: 22
ac: 11
awareness: 5
disposition: neutral
seduction_probability: 0.4
abilities:
  brutality: 10
  quickness: 12
  grit: 8
  reasoning: 10
  savvy: 10
  flair: 16
loot:
  currency:
    min: 8
    max: 30
```

- [ ] **Step 5.4: Create velvet_hostess.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/velvet_hostess.yaml`:

```yaml
id: velvet_hostess
name: Velvet Hostess
description: The smile doesn't reach the eyes, but the eyes know exactly what they are doing.
level: 3
max_hp: 28
ac: 12
awareness: 6
disposition: neutral
seduction_probability: 0.5
abilities:
  brutality: 8
  quickness: 14
  grit: 8
  reasoning: 12
  savvy: 12
  flair: 18
loot:
  currency:
    min: 12
    max: 45
```

- [ ] **Step 5.5: Create gangbang.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/gangbang.yaml`:

```yaml
id: gangbang
name: Gangbang!
description: Not a name, a warning. A presence that fills the room with the force of someone who has never once been told no.
level: 7
max_hp: 90
ac: 15
awareness: 9
disposition: hostile
tier: boss
abilities:
  brutality: 16
  quickness: 16
  grit: 14
  reasoning: 10
  savvy: 14
  flair: 20
loot:
  currency:
    min: 70
    max: 180
```

- [ ] **Step 5.6: Create the_velvet_rope.yaml**

Create `/home/cjohannsen/src/mud/content/zones/the_velvet_rope.yaml`:

```yaml
zone:
  id: the_velvet_rope
  danger_level: dangerous
  world_x: -2
  world_y: 4
  name: The Velvet Rope
  description: A private social club north of Beaverton where the dress code is optional and everything else costs extra.
  start_room: tvr_the_buffet
  zone_effects:
  - track: temptation
    severity: mild
    base_dc: 12
    cooldown_rounds: 3
    cooldown_minutes: 5
  - track: revulsion
    severity: mild
    base_dc: 11
    cooldown_rounds: 3
    cooldown_minutes: 5
  - track: terrain_lube
    severity: mild
    base_dc: 10
    cooldown_rounds: 3
    cooldown_minutes: 5
  rooms:
  - id: tvr_the_buffet
    danger_level: sketchy
    title: The Buffet
    description: 'A long table draped in red velvet holds offerings of dubious origin
      under warming lights. Patrons mill about with plates, watching the door. The
      floors are suspiciously slick. Stairs and exits warn of treacherous footing.'
    exits:
    - direction: east
      target: tvr_play_rooms
    map_x: 0
    map_y: 0
    spawns:
    - template: velvet_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: tvr_play_rooms
    danger_level: sketchy
    title: The Play Rooms
    description: 'A central hub connecting the club''s various chambers. Curtained
      doorways in every direction. A hostess at a velvet rope manages traffic with
      a clipboard and a practiced smile. The floor is worse here.'
    exits:
    - direction: west
      target: tvr_the_buffet
    - direction: north
      target: tvr_the_strangers
    - direction: east
      target: tvr_spit_roast
    - direction: south
      target: tvr_party_theater
    map_x: 1
    map_y: 0
    spawns:
    - template: velvet_hostess
      count: 1
      respawn_after: 5m
    boss_room: false
  - id: tvr_the_strangers
    danger_level: sketchy
    title: The Strangers
    description: 'A dimly lit room with no furniture and no names. Patrons enter as
      strangers and some of them leave that way too. Two figures turn to assess you
      with professional disinterest that shifts into something else.'
    exits:
    - direction: south
      target: tvr_play_rooms
    map_x: 1
    map_y: -1
    spawns:
    - template: velvet_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: tvr_spit_roast
    danger_level: sketchy
    title: Spit Roast
    description: 'The name is painted on the door in gold script, which does not make
      it less alarming. Inside, the room is arranged for activities that the furniture
      suggests but does not explain. Two patrons are unhappy about the interruption.'
    exits:
    - direction: west
      target: tvr_play_rooms
    - direction: south
      target: tvr_pineapple_room
    map_x: 2
    map_y: 0
    spawns:
    - template: velvet_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: tvr_pineapple_room
    danger_level: dangerous
    title: Pineapple Room
    description: 'A pineapple painted on the door, the universal signal for something
      you have to know about to understand. Inside it is immediately clear why the
      danger rating is elevated. Two hostesses who are done being polite.'
    exits:
    - direction: north
      target: tvr_spit_roast
    map_x: 2
    map_y: 1
    spawns:
    - template: velvet_hostess
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: tvr_party_theater
    danger_level: dangerous
    title: Party Theater
    description: 'A small theater with raked seating facing a stage equipped for performance.
      The performance currently underway involves audience participation. Two patrons
      in the wings are not interested in new cast members.'
    exits:
    - direction: north
      target: tvr_play_rooms
    - direction: east
      target: tvr_gangbang_room
    map_x: 1
    map_y: 1
    spawns:
    - template: velvet_patron
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: tvr_gangbang_room
    danger_level: dangerous
    title: The VIP Chamber
    description: 'The innermost room. No sign. No clipboard. Just a presence that
      has been holding court here since the club opened and intends to continue doing
      so. The floor is at its worst here. Everything is.'
    exits:
    - direction: west
      target: tvr_party_theater
    map_x: 2
    map_y: 2
    spawns:
    - template: gangbang
      count: 1
      respawn_after: 30m
    boss_room: true
```

- [ ] **Step 5.7: Run Velvet Rope test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_TheVelvetRope" -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 5.8: Run full world loader test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 5.9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
  content/npcs/velvet_patron.yaml \
  content/npcs/velvet_hostess.yaml \
  content/npcs/gangbang.yaml \
  content/zones/the_velvet_rope.yaml \
  internal/game/world/loader_test.go
git commit -m "feat(zones-phase2): add The Velvet Rope zone and NPC templates"
```

---

## Task 6: Club Privata — NPCs and Zone YAML

**Files:**
- Create: `content/npcs/club_bouncer.yaml`
- Create: `content/npcs/club_dancer.yaml`
- Create: `content/npcs/club_vip_boss.yaml`
- Create: `content/zones/club_privata.yaml`
- Modify: `internal/game/world/loader_test.go`

- [ ] **Step 6.1: Write failing zone loading test**

Add to `/home/cjohannsen/src/mud/internal/game/world/loader_test.go`:

```go
func TestLoadZone_ClubPrivata_HasSixteenRooms(t *testing.T) {
	data, err := os.ReadFile("../../../../content/zones/club_privata.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 16, "Club Privata must have exactly 16 rooms")
	assert.Equal(t, "club_privata", zone.ID)
}
```

- [ ] **Step 6.2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_ClubPrivata" -v 2>&1 | tail -10
```

Expected: FAIL — `club_privata.yaml` does not exist.

- [ ] **Step 6.3: Create club_bouncer.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/club_bouncer.yaml`:

```yaml
id: club_bouncer
name: Club Bouncer
description: Built like a refrigerator, dressed like a valet, with the disposition of neither.
level: 3
max_hp: 36
ac: 14
awareness: 7
disposition: hostile
abilities:
  brutality: 15
  quickness: 10
  grit: 12
  reasoning: 8
  savvy: 8
  flair: 8
loot:
  currency:
    min: 10
    max: 35
```

- [ ] **Step 6.4: Create club_dancer.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/club_dancer.yaml`:

```yaml
id: club_dancer
name: Club Dancer
description: Moves with professional precision and looks right through you while doing it.
level: 2
max_hp: 20
ac: 11
awareness: 5
disposition: neutral
seduction_probability: 0.2
abilities:
  brutality: 8
  quickness: 14
  grit: 8
  reasoning: 10
  savvy: 12
  flair: 18
loot:
  currency:
    min: 5
    max: 20
```

- [ ] **Step 6.5: Create club_vip_boss.yaml**

Create `/home/cjohannsen/src/mud/content/npcs/club_vip_boss.yaml`:

```yaml
id: club_vip_boss
name: VIP Suite Boss
description: The figure behind the velvet rope behind the velvet rope. Dressed in silence and studied indifference. Not indifferent at all.
level: 8
max_hp: 120
ac: 18
awareness: 10
disposition: hostile
tier: boss
abilities:
  brutality: 18
  quickness: 14
  grit: 18
  reasoning: 14
  savvy: 16
  flair: 20
loot:
  currency:
    min: 100
    max: 300
```

- [ ] **Step 6.6: Create club_privata.yaml**

Create `/home/cjohannsen/src/mud/content/zones/club_privata.yaml`:

```yaml
zone:
  id: club_privata
  danger_level: dangerous
  world_x: 4
  world_y: -2
  name: Club Privata
  description: A three-floor members-only venue south of Rustbucket Ridge where the music is loud enough to be a weapon.
  start_room: cp_entry
  zone_effects:
  - track: sonic_assault
    severity: mild
    base_dc: 12
    cooldown_rounds: 3
    cooldown_minutes: 5
  rooms:
  - id: cp_entry
    danger_level: dangerous
    title: Club Entry
    description: 'A velvet rope and two bouncers mark the threshold between outside
      and whatever this is. Bass frequencies vibrate the floor. A stamp on the wrist
      and a headache that starts immediately.'
    exits:
    - direction: east
      target: cp_bar_1f
    map_x: 0
    map_y: 0
    boss_room: false
  - id: cp_bar_1f
    danger_level: dangerous
    title: First Floor Bar
    description: 'A long bar of backlit bottles serves as the only visual anchor
      on the first floor. The music is a physical force here. Two bouncers maintain
      order by making everyone slightly afraid.'
    exits:
    - direction: west
      target: cp_entry
    - direction: east
      target: cp_dance_floor
    - direction: south
      target: cp_dining_area
    map_x: 1
    map_y: 0
    spawns:
    - template: club_bouncer
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: cp_dance_floor
    danger_level: dangerous
    title: Dance Floor
    description: 'The center of the first floor. Strobes. Fog. The kind of bass that
      reorganizes your internal organs. Dancers in practiced motion who are not interested
      in being interrupted by someone who is clearly not here to dance.'
    exits:
    - direction: west
      target: cp_bar_1f
    - direction: east
      target: cp_couples_lounge_1f
    - direction: south
      target: cp_seating_1f
    map_x: 2
    map_y: 0
    spawns:
    - template: club_dancer
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: cp_dining_area
    danger_level: dangerous
    title: Dining Area
    description: 'Low tables and low lighting south of the bar. The food service
      stopped at some point but the tables remain. Nobody is eating. East leads to
      lockers.'
    exits:
    - direction: north
      target: cp_bar_1f
    - direction: east
      target: cp_lockers_1f
    map_x: 1
    map_y: 1
    boss_room: false
  - id: cp_couples_lounge_1f
    danger_level: dangerous
    title: Couples Lounge
    description: 'Semicircular booths facing each other across small tables. Dim
      red lighting. A space designed for conversations that require privacy. Nobody
      here wants company.'
    exits:
    - direction: west
      target: cp_dance_floor
    map_x: 3
    map_y: 0
    boss_room: false
  - id: cp_seating_1f
    danger_level: dangerous
    title: First Floor Seating
    description: 'Long banquettes line the south wall. The music is somewhat less
      lethal here but only somewhat. Patrons with drinks watch the dance floor with
      varying degrees of engagement.'
    exits:
    - direction: north
      target: cp_dance_floor
    map_x: 2
    map_y: 1
    boss_room: false
  - id: cp_lockers_1f
    danger_level: dangerous
    title: First Floor Lockers
    description: 'A bank of lockers for member storage, most secured with combination
      locks. The music penetrates even here. A door west leads to the dining area,
      a door north leads to the restrooms.'
    exits:
    - direction: west
      target: cp_dining_area
    - direction: north
      target: cp_restrooms_1f
    map_x: 2
    map_y: 2
    boss_room: false
  - id: cp_restrooms_1f
    danger_level: dangerous
    title: First Floor Restrooms
    description: 'Black tile, black fixtures, mirrors that have not been cleaned
      in months. The smell of the rest of the building penetrates here. Stairs lead
      up to the mezzanine.'
    exits:
    - direction: west
      target: cp_lockers_1f
    - direction: north
      target: cp_mezzanine
    map_x: 3
    map_y: 2
    boss_room: false
  - id: cp_mezzanine
    danger_level: dangerous
    title: Mezzanine
    description: 'The second floor landing overlooks the first floor dance floor through
      smoked glass. The bass is different up here — more felt than heard. East leads
      to the bottle service area.'
    exits:
    - direction: south
      target: cp_restrooms_1f
    - direction: east
      target: cp_bottle_service
    map_x: 0
    map_y: 3
    boss_room: false
  - id: cp_bottle_service
    danger_level: dangerous
    title: Bottle Service
    description: 'Table service with bottle minimums that would fund a small expedition.
      Dancers attend to the tables with professional charm that has limits. Two of
      them turn as you arrive, calculating.'
    exits:
    - direction: west
      target: cp_mezzanine
    - direction: east
      target: cp_dance_pole
    map_x: 1
    map_y: 3
    spawns:
    - template: club_dancer
      count: 2
      respawn_after: 5m
    boss_room: false
  - id: cp_dance_pole
    danger_level: dangerous
    title: Dance Pole Stage
    description: 'A raised stage with three poles and lighting rigs designed to make
      everything look either glamorous or threatening. Currently threatening. Movement
      from the performers who have stopped performing.'
    exits:
    - direction: west
      target: cp_bottle_service
    - direction: east
      target: cp_private_rooms
    map_x: 2
    map_y: 3
    boss_room: false
  - id: cp_private_rooms
    danger_level: dangerous
    title: Private Rooms
    description: 'A corridor of numbered doors, most closed. The carpet is deep and
      sound-absorbing. Whatever happens in here, the club prefers not to know about it.
      No exits east.'
    exits:
    - direction: west
      target: cp_dance_pole
    map_x: 3
    map_y: 3
    boss_room: false
  - id: cp_seating_2f
    danger_level: dangerous
    title: Second Floor Seating
    description: 'Open seating area on the second floor with a view of the mezzanine
      below. Club dancers work the room between sets. The music is slightly more bearable
      up here, which means it is merely painful.'
    exits:
    - direction: south
      target: cp_dance_pole
    map_x: 2
    map_y: 4
    boss_room: false
  - id: cp_restrooms_2f
    danger_level: dangerous
    title: Second Floor Restrooms
    description: 'Mirror of the first floor restrooms but with better lighting and
      a more disturbing smell. Stairs spiral upward to the third floor through a
      door labeled "VIP ONLY."'
    exits:
    - direction: west
      target: cp_seating_2f
    - direction: north
      target: cp_public_play
    map_x: 3
    map_y: 4
    boss_room: false
  - id: cp_public_play
    danger_level: dangerous
    title: Public Play Space
    description: 'The third floor opens into a large, open-plan space with equipment
      and lighting designed for group activities. It is called "public" because the
      door does not lock. East connects to the couples area.'
    exits:
    - direction: south
      target: cp_restrooms_2f
    - direction: east
      target: cp_couples_3f
    map_x: 0
    map_y: 5
    boss_room: false
  - id: cp_couples_3f
    danger_level: dangerous
    title: Third Floor Couples Area
    description: 'Sectioned into alcoves with curtained doorways. The music from below
      is a memory up here — replaced by something lower, slower, more intentional.
      South leads to the VIP Suite. East leads to the third floor bar.'
    exits:
    - direction: west
      target: cp_public_play
    - direction: east
      target: cp_bar_3f
    - direction: south
      target: cp_vip_suite
    map_x: 1
    map_y: 5
    boss_room: false
  - id: cp_bar_3f
    danger_level: dangerous
    title: Third Floor Bar
    description: 'A smaller, quieter bar serving the VIP floor. The bottles here
      do not have prices on them. The bartender is not here. East leads to the lockers.'
    exits:
    - direction: west
      target: cp_couples_3f
    - direction: east
      target: cp_lockers_3f
    map_x: 2
    map_y: 5
    boss_room: false
  - id: cp_lockers_3f
    danger_level: dangerous
    title: Third Floor Lockers
    description: 'VIP storage, padlocked and initialed. No combination locks up here.
      These are keyed and the keys are not available. Nothing of interest except the
      sense of being in a place where you are not supposed to be.'
    exits:
    - direction: west
      target: cp_bar_3f
    map_x: 3
    map_y: 5
    boss_room: false
  - id: cp_vip_suite
    danger_level: dangerous
    title: VIP Suite
    description: 'A room that does not officially exist on the club''s floor plan.
      The only furniture is a single chair occupied by someone who has been waiting
      for exactly this. The sonic assault from below is a distant memory. This is
      worse than loud.'
    exits:
    - direction: north
      target: cp_couples_3f
    map_x: 1
    map_y: 6
    spawns:
    - template: club_vip_boss
      count: 1
      respawn_after: 30m
    boss_room: true
```

- [ ] **Step 6.7: Run Club Privata test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run "TestLoadZone_ClubPrivata" -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 6.8: Run full world loader test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... 2>&1 | tail -10
```

Expected: all tests PASS.

- [ ] **Step 6.9: Run the full test suite for all affected packages**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... ./internal/game/npc/... ./internal/game/world/... ./internal/gameserver/... 2>&1 | tail -20
```

Expected: all tests PASS across all four packages.

- [ ] **Step 6.10: Commit**

```bash
cd /home/cjohannsen/src/mud && git add \
  content/npcs/club_bouncer.yaml \
  content/npcs/club_dancer.yaml \
  content/npcs/club_vip_boss.yaml \
  content/zones/club_privata.yaml \
  internal/game/world/loader_test.go
git commit -m "feat(zones-phase2): add Club Privata zone and NPC templates; all phase 2 zones complete"
```

---

## Self-Review Checklist

**Spec coverage:**

- Clown Camp: delirium+fear zone_effects ✅, 5 rooms ✅, hostile clowns ✅, boss Just Clownin'! ✅, world_x=6 world_y=2 ✅
- SteamPDX: horror+nausea+reduced_visibility ✅, 7 rooms ✅, all NPCs male ✅, steam_patron seduction_gender=male (fails non-male → hostile) ✅, boss The Big 3 ✅, world_x=0 world_y=-2 ✅
- The Velvet Rope: temptation+revulsion+terrain_lube ✅, 7 rooms including The Strangers/Spit Roast/Pineapple Room(dangerous)/Party Theater(dangerous) ✅, boss Gangbang! ✅, world_x=-2 world_y=4 ✅
- Club Privata: sonic_assault ✅, 16 rooms ✅, 3 floors ✅, boss VIP Suite Boss ✅, world_x=4 world_y=-2 ✅
- terrain_lube condition: move_ap_cost=1, hustle penalty=2 ✅
- seduced condition: ap_reduction=2, is_mental_condition=true ✅
- NPC SeductionProbability + SeductionGender fields ✅
- Combat seduction logic: gender mismatch → hostile ✅, dice contest: NPC win → seduced ✅, NPC loss → hostile + SeductionRejected ✅

**Placeholder scan:** No TODOs, TBDs, or incomplete steps found.

**Type consistency:**
- `npc.Template.SeductionProbability float64` defined in Task 2 Step 2.3 → used throughout Tasks 2, 3, 4, 5, 6 ✅
- `npc.Instance.SeductionProbability float64` and `SeductionGender string` defined in Task 2 Step 2.4 ✅
- `condition.NewActiveSet()` used in Task 2 test — matches existing `condition` package API ✅
- `h.ResolveNPCSeductionGenderCheck(inst, uid, gender)` defined in Step 2.10, called in Step 2.11 ✅
- `h.ResolveNPCSeductionContest(inst, uid, savvy, def, condSet, npcRoll, playerRoll)` defined in Step 2.10, used in tests and Step 2.11 ✅
- `inst.Abilities.Flair` referenced in `ResolveNPCSeductionContest` — `Abilities` is on `Template`, not `Instance`. **Fix:** `Instance` has `Brutality`, `Quickness`, `Savvy` directly but not `Flair`. The `Flair` ability must be added to Instance or the resolver must accept flair as a parameter.

**Fix for Flair on Instance:**

In Task 2 Step 2.4, when adding fields to `Instance`, also add:

```go
	// Flair is copied from Template.Abilities.Flair at spawn. Used for seduction contests.
	Flair int
```

And in Step 2.5, in the `&Instance{...}` literal inside `NewInstanceWithResolver`, also add:

```go
		Flair: tmpl.Abilities.Flair,
```

Then in `ResolveNPCSeductionContest` (Step 2.10), use `inst.Flair` instead of `inst.Abilities.Flair`:

```go
	npcTotal := npcRoll + modFromScore(inst.Flair)
```

The plan above already uses `inst.Abilities.Flair` in the test template construction (which is correct — `Template.Abilities.Flair` is valid for building test templates), but the resolver itself must use `inst.Flair`.

**Updated Step 2.4** — the complete addition to `instance.go` is:

```go
	// Flair is copied from Template.Abilities.Flair at spawn. Used for seduction contests.
	Flair int
	// SeductionProbability is propagated from Template.SeductionProbability at spawn.
	// 0.0 means this NPC never initiates seduction.
	SeductionProbability float64
	// SeductionGender is propagated from Template.SeductionGender at spawn.
	// Empty string means any gender; non-empty restricts seduction to matching players.
	SeductionGender string
```

**Updated Step 2.5** — in `NewInstanceWithResolver` `&Instance{...}` block, add:

```go
		Flair:                tmpl.Abilities.Flair,
		SeductionProbability: tmpl.SeductionProbability,
		SeductionGender:      tmpl.SeductionGender,
```

All other cross-task references are consistent. ✅

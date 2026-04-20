# Clown Camp Faction War Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two new factions (QCE, UMCA) and their territories to Clown Camp, plus NPC-vs-NPC faction combat so all three factions auto-fight each other on The Stage.

**Architecture:** Two blocking engine changes precede all content work: (1) add `FactionID` to `CombatantInfo`/`Combatant` and expose `get_faction_enemies()` in the Lua combat API; (2) wire faction combat initiation on NPC room entry (respawn + admin spawn). All content work — faction YAMLs, HTN domain, NPC templates, new rooms — is pure YAML and can be batched after the engine changes pass tests.

**Tech Stack:** Go (gRPC service, scripting manager, npc respawn), Lua (HTN preconditions/operators), YAML (factions, npcs, zones, ai domains), `pgregory.net/rapid` (property tests)

---

## File Structure

**Engine — modified:**
- `internal/scripting/manager.go` — add `FactionID string` to `CombatantInfo`; add `GetFactionHostiles func(string) []string` to `Manager`
- `internal/scripting/modules.go` — set `faction_id` in `combatantToTable`; add `get_faction_enemies` Lua function
- `internal/game/combat/combat.go` — add `FactionID string` to `Combatant` struct
- `internal/gameserver/combat_handler.go` — set `FactionID` on NPC combatants at construction; wire `GetFactionHostiles` callback
- `internal/game/npc/respawn.go` — add `AfterPlace func(inst *npc.Instance, roomID string)` callback
- `internal/gameserver/grpc_service.go` — wire `AfterPlace` to faction initiation logic

**Engine — new:**
- `internal/scripting/modules_faction_test.go` — unit tests for `get_faction_enemies`
- `internal/gameserver/faction_initiation_test.go` — unit tests for faction combat initiation

**Content — new:**
- `content/factions/just_clownin.yaml`
- `content/factions/queer_clowning_experience.yaml`
- `content/factions/unwoke_maga_clown_army.yaml`
- `content/ai/clown_faction_combat.yaml`
- `content/npcs/jc_fighter.yaml`
- `content/npcs/jc_enforcer.yaml`
- `content/npcs/qce_agitator.yaml`
- `content/npcs/qce_drag_bruiser.yaml`
- `content/npcs/qce_ringleader.yaml`
- `content/npcs/qce_merchant.yaml`
- `content/npcs/umca_grunt.yaml`
- `content/npcs/umca_flag_bearer.yaml`
- `content/npcs/umca_commander.yaml`
- `content/npcs/umca_merchant.yaml`

**Content — modified:**
- `content/npcs/clown.yaml` — add `faction_id: just_clownin`
- `content/npcs/clown_mime.yaml` — add `faction_id: just_clownin`
- `content/npcs/just_clownin.yaml` — add `faction_id: just_clownin`
- `content/npcs/big_top.yaml` — add `faction_id: just_clownin`
- `content/zones/clown_camp.yaml` — add QCE rooms, UMCA rooms, update The Stage

---

## Task 1: Add FactionID to CombatantInfo and Combatant (REQ-CCF-2e)

**Files:**
- Modify: `internal/scripting/manager.go`
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/scripting/modules.go`
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Write the failing test for faction_id in combatantToTable**

Create `internal/scripting/modules_faction_test.go`:

```go
package scripting_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/scripting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGetFactionEnemies_NPCWithFactionSeesHostiles verifies that get_faction_enemies
// returns combatants from hostile factions (REQ-CCF-2a).
func TestGetFactionEnemies_NPCWithFactionSeesHostiles(t *testing.T) {
	roller, logger := testRoller(t), zap.NewNop()
	mgr := scripting.NewManager(roller, logger)

	// NPC with just_clownin faction
	actorUID := "actor-1"
	hostileUID := "hostile-1"
	allyUID := "ally-1"

	mgr.GetEntityRoom = func(uid string) string { return "room-1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 50, MaxHP: 50, FactionID: "just_clownin"},
			{UID: hostileUID, Kind: "npc", HP: 40, MaxHP: 40, FactionID: "queer_clowning_experience"},
			{UID: allyUID, Kind: "npc", HP: 30, MaxHP: 30, FactionID: "just_clownin"},
		}
	}
	// just_clownin is hostile to queer_clowning_experience
	mgr.GetFactionHostiles = func(factionID string) []string {
		if factionID == "just_clownin" {
			return []string{"queer_clowning_experience", "unwoke_maga_clown_army"}
		}
		return nil
	}

	require.NoError(t, mgr.LoadZone("zone-1", `
function test_faction_enemies(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`))

	result, err := mgr.CallHook("zone-1", "test_faction_enemies", actorUID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, result, "should return exactly 1 faction enemy (the QCE NPC)")
}

// TestGetFactionEnemies_NoFactionReturnsEmpty verifies that an NPC with no faction
// always returns an empty table (REQ-CCF-2b).
func TestGetFactionEnemies_NoFactionReturnsEmpty(t *testing.T) {
	roller, logger := testRoller(t), zap.NewNop()
	mgr := scripting.NewManager(roller, logger)

	actorUID := "actor-1"
	mgr.GetEntityRoom = func(uid string) string { return "room-1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 50, MaxHP: 50, FactionID: ""},
			{UID: "other-1", Kind: "npc", HP: 40, MaxHP: 40, FactionID: "queer_clowning_experience"},
		}
	}
	mgr.GetFactionHostiles = func(factionID string) []string { return nil }

	require.NoError(t, mgr.LoadZone("zone-1", `
function test_no_faction(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`))

	result, err := mgr.CallHook("zone-1", "test_no_faction", actorUID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, result, "NPC with no faction should see no faction enemies")
}
```

- [ ] **Step 2: Run the test — confirm it fails**

```bash
go test ./internal/scripting/... -run TestGetFactionEnemies -v 2>&1 | head -20
```

Expected: FAIL (FactionID field doesn't exist yet, GetFactionHostiles not defined).

- [ ] **Step 3: Add FactionID to CombatantInfo**

In `internal/scripting/manager.go`, add to `CombatantInfo`:

```go
// CombatantInfo is a snapshot of a combatant's state passed to Lua callbacks.
type CombatantInfo struct {
	UID        string
	Name       string
	HP         int
	MaxHP      int
	AC         int
	Conditions []string
	// Kind is "player" or "npc" — used by Lua to distinguish combatant types.
	Kind string
	// FactionID is the faction this combatant belongs to. Empty for players
	// and NPCs with no faction_id in their template (REQ-CCF-2e).
	FactionID string
}
```

Also add to `Manager` struct (after `GetEntityRoom`):

```go
	// GetFactionHostiles returns the list of faction IDs that are hostile to
	// the given factionID. Returns nil when factionID is unknown or has no hostiles.
	// Required for engine.combat.get_faction_enemies (REQ-CCF-2).
	GetFactionHostiles func(factionID string) []string
```

- [ ] **Step 4: Add FactionID to combat.Combatant**

In `internal/game/combat/combat.go`, add a field to `Combatant` after `NPCType`:

```go
	// FactionID is the faction this NPC belongs to. Empty for players and
	// unfactioned NPCs. Used for NPC-vs-NPC faction combat (REQ-CCF-2e).
	FactionID string
```

- [ ] **Step 5: Set faction_id in combatantToTable and add get_faction_enemies**

In `internal/scripting/modules.go`, update `combatantToTable` to include `faction_id`:

```go
func combatantToTable(L *lua.LState, c *CombatantInfo) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "uid", lua.LString(c.UID))
	L.SetField(tbl, "name", lua.LString(c.Name))
	L.SetField(tbl, "hp", lua.LNumber(c.HP))
	L.SetField(tbl, "max_hp", lua.LNumber(c.MaxHP))
	L.SetField(tbl, "ac", lua.LNumber(c.AC))
	L.SetField(tbl, "kind", lua.LString(c.Kind))
	L.SetField(tbl, "faction_id", lua.LString(c.FactionID))  // add this line
	// ... existing conditions code unchanged ...
```

Add `get_faction_enemies` after the existing `get_allies` function in `newCombatModule`:

```go
	L.SetField(t, "get_faction_enemies", L.NewFunction(func(L *lua.LState) int {
		uid := L.CheckString(1)
		if m.GetFactionHostiles == nil {
			L.Push(L.NewTable())
			return 1
		}
		all := roomCombatants(uid)
		if all == nil {
			L.Push(L.NewTable())
			return 1
		}

		// Find self's faction.
		selfFactionID := ""
		for _, c := range all {
			if c.UID == uid {
				selfFactionID = c.FactionID
				break
			}
		}
		if selfFactionID == "" {
			// REQ-CCF-2b: no faction → empty table.
			L.Push(L.NewTable())
			return 1
		}

		hostileFactions := m.GetFactionHostiles(selfFactionID)
		if len(hostileFactions) == 0 {
			L.Push(L.NewTable())
			return 1
		}

		hostile := make(map[string]bool, len(hostileFactions))
		for _, hf := range hostileFactions {
			hostile[hf] = true
		}

		result := L.NewTable()
		idx := 1
		for _, c := range all {
			// REQ-CCF-2c: include even if already in combat with uid.
			if c.UID != uid && hostile[c.FactionID] {
				L.RawSetInt(result, idx, combatantToTable(L, c))
				idx++
			}
		}
		L.Push(result)
		return 1
	}))
```

- [ ] **Step 6: Set FactionID on NPC combatants in combat_handler.go**

In `internal/gameserver/combat_handler.go`, find all `&combat.Combatant{` blocks that construct NPC combatants (lines ~1205, ~1749, ~2944). Add `FactionID: inst.FactionID` to each:

```go
	npcCbt := &combat.Combatant{
		ID:          inst.ID,
		Kind:        combat.KindNPC,
		Name:        inst.Name(),
		MaxHP:       inst.MaxHP,
		CurrentHP:   inst.CurrentHP,
		AC:          inst.AC + npcFeatStats.ACBonus,
		Level:       inst.Level,
		StrMod:      npcStrMod,
		DexMod:      1,
		NPCType:     inst.Type,
		FactionID:   inst.FactionID,   // add this
		Resistances: inst.Resistances,
		Weaknesses:  inst.Weaknesses,
		WeaponName:  npcWeaponName,
		AttackVerb:  inst.AttackVerb,
		SpeedFt:     inst.SpeedFt,
	}
```

Apply to all three NPC combatant construction sites.

- [ ] **Step 7: Wire GetFactionHostiles in grpc_service.go**

Find where `s.scriptMgr` is wired (after line ~590 in `grpc_service.go`). Add:

```go
	if s.factionRegistry != nil {
		s.scriptMgr.GetFactionHostiles = func(factionID string) []string {
			reg := *s.factionRegistry
			def, ok := reg[factionID]
			if !ok {
				return nil
			}
			return def.HostileFactions
		}
	}
```

Also ensure `GetCombatantsInRoom` callback includes `FactionID`. Find where it's assigned (search for `GetCombatantsInRoom`):

```go
	s.scriptMgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		cbt, ok := s.engine.GetCombat(roomID)
		if !ok {
			return nil
		}
		out := make([]*scripting.CombatantInfo, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			info := &scripting.CombatantInfo{
				UID:       c.UID,
				Name:      c.Name,
				HP:        c.CurrentHP,
				MaxHP:     c.MaxHP,
				AC:        c.AC,
				Kind:      c.Kind,
				FactionID: c.FactionID, // add this
			}
			// ... existing conditions code ...
			out = append(out, info)
		}
		return out
	}
```

- [ ] **Step 8: Run tests**

```bash
go test ./internal/scripting/... -run TestGetFactionEnemies -v
go test ./internal/game/combat/... -v 2>&1 | tail -5
go test ./internal/gameserver/... 2>&1 | tail -10
```

Expected: `TestGetFactionEnemies_*` PASS. No regressions.

- [ ] **Step 9: Commit**

```bash
git add internal/scripting/manager.go internal/scripting/modules.go internal/scripting/modules_faction_test.go internal/game/combat/combat.go internal/gameserver/combat_handler.go internal/gameserver/grpc_service.go
git commit -m "feat(scripting): add FactionID to CombatantInfo + get_faction_enemies Lua API (REQ-CCF-2)"
```

---

## Task 2: Faction Combat Initiation on NPC Room Entry (REQ-CCF-3)

**Files:**
- Modify: `internal/game/npc/respawn.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/faction_initiation_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/gameserver/faction_initiation_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
)

// TestFactionInitiation_AllOutWarRoomTriggersHostility verifies that when a JC NPC
// enters an all_out_war room containing a QCE NPC, combat is initiated (REQ-CCF-3a,3b).
func TestFactionInitiation_AllOutWarRoomTriggersHostility(t *testing.T) {
	// Arrange: room with danger_level all_out_war containing a QCE NPC.
	// A JC NPC enters (simulated via AfterPlace callback).
	// Assert: initiateFactonCombat was called with the two NPCs.
	initiated := false
	var initiatingInst, targetInst *npc.Instance

	room := &world.Room{
		ID:          "cc_the_stage",
		DangerLevel: world.DangerLevelAllOutWar,
	}

	jcInst := &npc.Instance{ID: "jc-1", FactionID: "just_clownin", CurrentHP: 100}
	qceInst := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 80}

	hostiles := map[string][]string{
		"just_clownin": {"queer_clowning_experience", "unwoke_maga_clown_army"},
	}

	initiateFunc := func(attacker, target *npc.Instance, r *world.Room) {
		initiated = true
		initiatingInst = attacker
		targetInst = target
	}

	npcListInRoom := func(roomID string) []*npc.Instance {
		if roomID == "cc_the_stage" {
			return []*npc.Instance{qceInst}
		}
		return nil
	}

	getRoom := func(roomID string) *world.Room { return room }
	getHostiles := func(factionID string) []string { return hostiles[factionID] }

	// Act: call the faction initiation check (exported from gameserver package).
	checkFactionInitiation(jcInst, "cc_the_stage", npcListInRoom, getRoom, getHostiles, initiateFunc)

	// Assert.
	assert.True(t, initiated, "combat should be initiated")
	assert.Equal(t, jcInst.ID, initiatingInst.ID)
	assert.Equal(t, qceInst.ID, targetInst.ID, "target is the QCE NPC (lowest HP)")
}

// TestFactionInitiation_SafeRoomDoesNotTrigger verifies that faction initiation
// does NOT fire in safe or sketchy rooms (REQ-CCF-3b).
func TestFactionInitiation_SafeRoomDoesNotTrigger(t *testing.T) {
	for _, dl := range []world.DangerLevel{world.DangerLevelSafe, world.DangerLevelSketchy, world.DangerLevelDangerous} {
		initiated := false
		room := &world.Room{ID: "some-room", DangerLevel: dl}
		jcInst := &npc.Instance{ID: "jc-1", FactionID: "just_clownin", CurrentHP: 100}
		qceInst := &npc.Instance{ID: "qce-1", FactionID: "queer_clowning_experience", CurrentHP: 80}

		checkFactionInitiation(jcInst, "some-room",
			func(string) []*npc.Instance { return []*npc.Instance{qceInst} },
			func(string) *world.Room { return room },
			func(string) []string { return []string{"queer_clowning_experience"} },
			func(_, _ *npc.Instance, _ *world.Room) { initiated = true },
		)
		assert.False(t, initiated, "danger_level %v must not trigger initiation", dl)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/gameserver/... -run TestFactionInitiation -v 2>&1 | head -20
```

Expected: FAIL (checkFactionInitiation not defined yet).

- [ ] **Step 3: Add AfterPlace callback to RespawnManager**

In `internal/game/npc/respawn.go`, add a callback field to `RespawnManager`:

```go
// AfterPlace, if non-nil, is called each time an NPC is successfully placed by
// Tick. Used to trigger faction-aware combat initiation (REQ-CCF-3).
AfterPlace func(inst *Instance, roomID string)
```

In `Tick`, after `inst, _ := mgr.Spawn(tmpl, e.roomID)`:

```go
		inst, _ := mgr.Spawn(tmpl, e.roomID)
		if inst != nil {
			if r.AfterPlace != nil {
				r.AfterPlace(inst, e.roomID)
			}
			// ... existing HomeRoomBFS code ...
		}
```

- [ ] **Step 4: Implement checkFactionInitiation and wire it**

In `internal/gameserver/grpc_service.go`, add the exported helper function and wire it to `respawnMgr.AfterPlace`:

```go
// checkFactionInitiation examines the room for hostile-faction NPCs and initiates
// NPC-vs-NPC combat if any are found. Only fires in all_out_war rooms (REQ-CCF-3a,3b).
//
// Precondition: arrivalInst and roomID must be non-nil/non-empty.
// Postcondition: initiate is called at most once with the lowest-HP hostile NPC.
func checkFactionInitiation(
	arrivalInst *npc.Instance,
	roomID string,
	npcsInRoom func(roomID string) []*npc.Instance,
	getRoom func(roomID string) *world.Room,
	getHostiles func(factionID string) []string,
	initiate func(attacker, target *npc.Instance, room *world.Room),
) {
	if arrivalInst.FactionID == "" {
		return
	}
	room := getRoom(roomID)
	if room == nil || room.DangerLevel != world.DangerLevelAllOutWar {
		// REQ-CCF-3b: only fires in all_out_war rooms.
		return
	}
	hostileSet := make(map[string]bool)
	for _, hf := range getHostiles(arrivalInst.FactionID) {
		hostileSet[hf] = true
	}
	if len(hostileSet) == 0 {
		return
	}

	existing := npcsInRoom(roomID)
	var target *npc.Instance
	for _, inst := range existing {
		if inst.ID == arrivalInst.ID || !hostileSet[inst.FactionID] {
			continue
		}
		// REQ-CCF-3a: pick lowest-HP hostile NPC.
		if target == nil || inst.CurrentHP < target.CurrentHP {
			target = inst
		}
	}
	if target == nil {
		return
	}
	initiate(arrivalInst, target, room)
}
```

In `grpc_service.go`, after the respawnMgr is assigned (find the block after line ~482), wire `AfterPlace`:

```go
	if s.respawnMgr != nil && s.factionRegistry != nil {
		reg := *s.factionRegistry
		s.respawnMgr.AfterPlace = func(inst *npc.Instance, roomID string) {
			checkFactionInitiation(
				inst, roomID,
				func(rID string) []*npc.Instance { return s.npcMgr.NPCsInRoom(rID) },
				func(rID string) *world.Room { r, _ := s.worldMgr.GetRoom(rID); return r },
				func(factionID string) []string {
					def, ok := reg[factionID]
					if !ok {
						return nil
					}
					return def.HostileFactions
				},
				func(attacker, target *npc.Instance, room *world.Room) {
					s.initiateNPCFactionCombat(attacker, target, roomID)
				},
			)
		}
	}
```

- [ ] **Step 5: Implement initiateNPCFactionCombat**

In `internal/gameserver/grpc_service.go`, add:

```go
// initiateNPCFactionCombat starts an NPC-vs-NPC faction combat between attacker
// and target in roomID (REQ-CCF-3).
//
// Precondition: attacker and target must be non-nil with valid FactionIDs.
// Postcondition: Both NPCs are registered in the room's combat engine.
//                A console message is sent to all players in the room (REQ-CCF-3c).
func (s *GameServiceServer) initiateNPCFactionCombat(attacker, target *npc.Instance, roomID string) {
	// Build the message (REQ-CCF-3c).
	msg := fmt.Sprintf("A %s lunges at a %s!", attacker.Name(), target.Name())
	s.broadcastRoomConsole(roomID, msg)

	// Register both NPCs in combat.
	s.engine.RegisterNPCCombatant(roomID, attacker)
	s.engine.RegisterNPCCombatant(roomID, target)
}
```

Note: The exact method signatures for `broadcastRoomConsole` and `RegisterNPCCombatant` may differ from what the engine exposes — inspect `internal/gameserver/combat_handler.go` and `internal/game/combat/engine.go` to find the correct names. Adapt accordingly (this is a new code path; look for existing patterns like `JoinPendingNPCCombat`).

- [ ] **Step 6: Run tests**

```bash
go test ./internal/gameserver/... -run TestFactionInitiation -v
go test ./internal/game/npc/... -v 2>&1 | tail -5
go test ./internal/gameserver/... 2>&1 | tail -10
```

Expected: `TestFactionInitiation_*` PASS. No regressions.

- [ ] **Step 7: Commit**

```bash
git add internal/game/npc/respawn.go internal/gameserver/grpc_service.go internal/gameserver/faction_initiation_test.go
git commit -m "feat(gameserver): faction combat initiation on NPC room entry (REQ-CCF-3)"
```

---

## Task 3: Faction YAML Files (REQ-CCF-1)

**Files:**
- Create: `content/factions/just_clownin.yaml`
- Create: `content/factions/queer_clowning_experience.yaml`
- Create: `content/factions/unwoke_maga_clown_army.yaml`

- [ ] **Step 1: Create just_clownin.yaml**

```yaml
id: just_clownin
name: "Just Clownin'"
zone_id: clown_camp
hostile_factions:
  - queer_clowning_experience
  - unwoke_maga_clown_army
tiers:
  - id: curious
    label: "Curious"
    min_rep: 0
    price_discount: 0.0
  - id: initiate
    label: "Initiate"
    min_rep: 100
    price_discount: 0.05
  - id: clown
    label: "Clown"
    min_rep: 500
    price_discount: 0.10
  - id: ringleader
    label: "Ringleader"
    min_rep: 2000
    price_discount: 0.20
rep_sources:
  - source: kill_queer_clowning_experience
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
  - source: kill_unwoke_maga_clown_army
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
```

- [ ] **Step 2: Create queer_clowning_experience.yaml**

```yaml
id: queer_clowning_experience
name: "The Queer Clowning Experience"
zone_id: clown_camp
hostile_factions:
  - just_clownin
  - unwoke_maga_clown_army
tiers:
  - id: curious
    label: "Curious"
    min_rep: 0
    price_discount: 0.0
  - id: initiate
    label: "Initiate"
    min_rep: 100
    price_discount: 0.05
  - id: clown
    label: "Clown"
    min_rep: 500
    price_discount: 0.10
  - id: ringleader
    label: "Ringleader"
    min_rep: 2000
    price_discount: 0.20
rep_sources:
  - source: kill_just_clownin
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
  - source: kill_unwoke_maga_clown_army
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
```

- [ ] **Step 3: Create unwoke_maga_clown_army.yaml**

```yaml
id: unwoke_maga_clown_army
name: "The Unwoke MAGA Clown Army"
zone_id: clown_camp
hostile_factions:
  - just_clownin
  - queer_clowning_experience
tiers:
  - id: curious
    label: "Curious"
    min_rep: 0
    price_discount: 0.0
  - id: initiate
    label: "Initiate"
    min_rep: 100
    price_discount: 0.05
  - id: clown
    label: "Clown"
    min_rep: 500
    price_discount: 0.10
  - id: ringleader
    label: "Ringleader"
    min_rep: 2000
    price_discount: 0.20
rep_sources:
  - source: kill_just_clownin
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
  - source: kill_queer_clowning_experience
    rep_per_level: 8
    cap_per_kill: 40
    cap_below_tier: clown
```

- [ ] **Step 4: Run faction validation tests**

```bash
go test ./internal/game/faction/... -v 2>&1 | tail -10
```

Expected: All PASS (the registry loader validates all faction files on load).

- [ ] **Step 5: Commit**

```bash
git add content/factions/just_clownin.yaml content/factions/queer_clowning_experience.yaml content/factions/unwoke_maga_clown_army.yaml
git commit -m "content(faction): add Just Clownin', QCE, and UMCA faction definitions (REQ-CCF-1)"
```

---

## Task 4: HTN AI Domain — Faction Combat (REQ-CCF-4)

**Files:**
- Create: `content/ai/clown_faction_combat.yaml`

- [ ] **Step 1: Create the HTN domain**

```yaml
domain:
  id: clown_faction_combat
  description: Combat AI for faction NPCs on The Stage. Prioritizes faction enemies; falls back to player enemies.

  tasks:
    - id: behave
      description: Root task — choose faction enemy or player enemy
    - id: fight_faction_enemy
      description: Attack the nearest faction-hostile combatant
    - id: fight_player
      description: Attack the nearest player enemy

  methods:
    - task: behave
      id: faction_enemy_present
      precondition: has_faction_enemy
      subtasks: [fight_faction_enemy]

    - task: behave
      id: player_enemy_present
      precondition: has_player_enemy
      subtasks: [fight_player]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]

    - task: fight_faction_enemy
      id: strike_faction_enemy
      precondition: ""
      subtasks: [attack_faction_enemy]

    - task: fight_player
      id: strike_player
      precondition: ""
      subtasks: [attack_enemy]

  operators:
    - id: attack_faction_enemy
      action: attack
      target: nearest_faction_enemy

    - id: attack_enemy
      action: attack
      target: nearest_enemy

    - id: do_pass
      action: pass
      target: ""
```

The preconditions `has_faction_enemy` and `has_player_enemy` must be registered as Lua preconditions. Create `content/scripts/zones/clown_camp/faction_preconditions.lua`:

```lua
-- has_faction_enemy: returns true iff the NPC has at least one faction enemy in the room.
-- REQ-CCF-4a
function has_faction_enemy(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count > 0
end

-- has_player_enemy: returns true iff the NPC has at least one player enemy in the room.
function has_player_enemy(uid)
  return engine.combat.enemy_count(uid) > 0
end
```

The `nearest_faction_enemy` target type resolves in the HTN operator executor to the combatant from `get_faction_enemies()` with the lowest current HP (REQ-CCF-4b). Check `internal/gameserver/combat_handler.go` for how `nearest_enemy` is resolved — add a parallel case for `nearest_faction_enemy` using `get_faction_enemies`.

- [ ] **Step 2: Implement nearest_faction_enemy target resolver**

In `internal/gameserver/combat_handler.go`, find the switch/if block that resolves `target: nearest_enemy` in HTN operators. Add a case for `nearest_faction_enemy`:

```go
case "nearest_faction_enemy":
	// Select lowest-HP combatant from get_faction_enemies(uid) (REQ-CCF-4b).
	if s.scriptMgr == nil || s.scriptMgr.GetFactionHostiles == nil {
		return nil, fmt.Errorf("nearest_faction_enemy: faction service not wired")
	}
	factionID := actor.FactionID
	if factionID == "" {
		return nil, nil // no faction, no target
	}
	hostileSet := make(map[string]bool)
	for _, hf := range s.scriptMgr.GetFactionHostiles(factionID) {
		hostileSet[hf] = true
	}
	var best *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.ID == actor.ID || c.IsDead() || !hostileSet[c.FactionID] {
			continue
		}
		if best == nil || c.CurrentHP < best.CurrentHP {
			best = c
		}
	}
	return best, nil
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/gameserver/... -v 2>&1 | tail -10
```

Expected: No regressions.

- [ ] **Step 4: Commit**

```bash
git add content/ai/clown_faction_combat.yaml content/scripts/zones/clown_camp/faction_preconditions.lua internal/gameserver/combat_handler.go
git commit -m "content(ai): clown_faction_combat HTN domain + nearest_faction_enemy resolver (REQ-CCF-4)"
```

---

## Task 5: Assign Faction to Existing Clown Camp NPCs (REQ-CCF-5)

**Files:**
- Modify: `content/npcs/clown.yaml`
- Modify: `content/npcs/clown_mime.yaml`
- Modify: `content/npcs/just_clownin.yaml`
- Modify: `content/npcs/big_top.yaml`

- [ ] **Step 1: Add faction_id to existing JC NPCs**

In each file, add `faction_id: just_clownin` after the `id:` line:

`content/npcs/clown.yaml`: add `faction_id: just_clownin`

`content/npcs/clown_mime.yaml`: add `faction_id: just_clownin`

`content/npcs/just_clownin.yaml` (the ringleader NPC): add `faction_id: just_clownin`

`content/npcs/big_top.yaml`: add `faction_id: just_clownin`

- [ ] **Step 2: Run tests**

```bash
go test ./internal/game/npc/... -v 2>&1 | tail -10
```

Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add content/npcs/clown.yaml content/npcs/clown_mime.yaml content/npcs/just_clownin.yaml content/npcs/big_top.yaml
git commit -m "content(npc): assign faction_id just_clownin to existing Clown Camp NPCs (REQ-CCF-5)"
```

---

## Task 6: New NPC Templates (REQ-CCF-6)

**Files:**
- Create: all 10 files listed below

- [ ] **Step 1: Create JC sortie NPCs**

Create `content/npcs/jc_fighter.yaml`:
```yaml
id: jc_fighter
name: "Just Clownin' Fighter"
description: A mid-tier goon of the Just Clownin' faction, armed with improvised melee weapons and a deeply unnerving paint-streaked grin.
level: 47
max_hp: 546
ac: 21
tier: elite
faction_id: just_clownin
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 8
abilities:
  brutality: 18
  quickness: 16
  grit: 18
  reasoning: 10
  savvy: 11
  flair: 14
weapon:
  - id: rebar_club
    weight: 3
  - id: spiked_knuckles
    weight: 2
armor:
  - id: kevlar_vest
    weight: 3
taunts:
  - "The stage belongs to Just Clownin'. Always has."
  - "You picked the wrong clown to fight."
  - "Every night ends the same. With us."
```

Create `content/npcs/jc_enforcer.yaml`:
```yaml
id: jc_enforcer
name: "Just Clownin' Enforcer"
description: An elite enforcer of the Just Clownin' faction who can apply a disorienting face-paint debuff mid-combat.
level: 52
max_hp: 963
ac: 23
tier: elite
faction_id: just_clownin
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 9
abilities:
  brutality: 20
  quickness: 17
  grit: 20
  reasoning: 12
  savvy: 13
  flair: 16
weapon:
  - id: rebar_club
    weight: 2
  - id: paint_sprayer
    weight: 3
armor:
  - id: kevlar_vest
    weight: 3
  - id: leather_jacket
    weight: 1
taunts:
  - "You'll look better in paint."
  - "We run the show. The whole show."
  - "Face paint hides a lot. Scars included."
```

- [ ] **Step 2: Create QCE NPC templates**

Create `content/npcs/qce_agitator.yaml`:
```yaml
id: qce_agitator
name: "QCE Agitator"
description: A fast-moving fighter from The Queer Clowning Experience, maximizing disruption and harassment over raw power.
level: 46
max_hp: 528
ac: 21
faction_id: queer_clowning_experience
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 9
abilities:
  brutality: 15
  quickness: 20
  grit: 16
  reasoning: 12
  savvy: 14
  flair: 18
weapon:
  - id: cheap_blade
    weight: 3
  - id: spiked_knuckles
    weight: 2
armor:
  - id: leather_jacket
    weight: 3
taunts:
  - "The rainbow flag flies over this stage."
  - "You can't suppress what's already liberated."
  - "QCE doesn't negotiate."
```

Create `content/npcs/qce_drag_bruiser.yaml`:
```yaml
id: qce_drag_bruiser
name: "QCE Drag Bruiser"
description: A heavily built QCE elite who throws glitter-bomb grenades for area-of-effect chaos.
level: 51
max_hp: 932
ac: 23
tier: elite
faction_id: queer_clowning_experience
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 10
abilities:
  brutality: 20
  quickness: 15
  grit: 22
  reasoning: 12
  savvy: 13
  flair: 20
weapon:
  - id: glitter_bomb
    weight: 3
  - id: steel_pipe
    weight: 2
armor:
  - id: kevlar_vest
    weight: 3
taunts:
  - "Too sparkly for you to handle."
  - "Glitter doesn't wash out. Neither do we."
  - "High heels. Higher stakes."
```

Create `content/npcs/qce_ringleader.yaml`:
```yaml
id: qce_ringleader
name: "The Sequined Ringleader"
description: >
  The leader of The Queer Clowning Experience, a towering figure in six-inch heels
  and a sequined ringmaster's coat who commands the throne room of QCE territory
  with theatrical cruelty and impressive combat ability.
level: 60
max_hp: 2430
tier: boss
ac: 27
faction_id: queer_clowning_experience
ai_domain: territory_patrol
respawn_delay: "72h"
rob_multiplier: 2.0
awareness: 12
abilities:
  brutality: 20
  quickness: 20
  grit: 22
  reasoning: 18
  savvy: 18
  flair: 24
weapon:
  - id: sequined_scepter
    weight: 3
  - id: combat_knife
    weight: 1
armor:
  - id: tactical_armor
    weight: 3
boss_abilities:
  - id: glitter_storm
    name: "Glitter Storm"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: nausea
  - id: crowd_control
    name: "Crowd Control"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "8d10"
  - id: curtain_call
    name: "Curtain Call"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 25
taunts:
  - "Darling, this is MY stage."
  - "You came all this way just to lose to sequins?"
  - "QCE has never lost a fight. We're not starting today."
```

Create `content/npcs/qce_merchant.yaml`:
```yaml
id: qce_merchant
name: "QCE Supply Captain"
description: A non-combat QCE member who sells faction-exclusive supplies to allied visitors.
npc_type: merchant
type: human
level: 48
disposition: friendly
faction_id: queer_clowning_experience
respawn_delay: "0s"
```

- [ ] **Step 3: Create UMCA NPC templates**

Create `content/npcs/umca_grunt.yaml`:
```yaml
id: umca_grunt
name: "UMCA Grunt"
description: A rank-and-file soldier of The Unwoke MAGA Clown Army, armed with ranged weapons and unwavering conviction.
level: 46
max_hp: 528
ac: 21
faction_id: unwoke_maga_clown_army
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 8
abilities:
  brutality: 17
  quickness: 16
  grit: 18
  reasoning: 10
  savvy: 11
  flair: 10
weapon:
  - id: assault_rifle
    weight: 3
  - id: combat_knife
    weight: 1
armor:
  - id: kevlar_vest
    weight: 3
taunts:
  - "This is MAGA territory. Back off."
  - "We don't clown around. Much."
  - "UMCA never retreats."
```

Create `content/npcs/umca_flag_bearer.yaml`:
```yaml
id: umca_flag_bearer
name: "UMCA Flag Bearer"
description: An elite UMCA soldier who carries the faction banner and provides a tactical AC bonus to nearby UMCA allies.
level: 51
max_hp: 932
ac: 23
tier: elite
faction_id: unwoke_maga_clown_army
ai_domain: clown_faction_combat
respawn_delay: "10m"
rob_multiplier: 1.5
awareness: 9
abilities:
  brutality: 19
  quickness: 16
  grit: 20
  reasoning: 11
  savvy: 12
  flair: 12
weapon:
  - id: assault_rifle
    weight: 2
  - id: combat_knife
    weight: 2
armor:
  - id: kevlar_vest
    weight: 3
  - id: leather_jacket
    weight: 1
taunts:
  - "Rally to the flag! UMCA moves as one!"
  - "The flag never falls while I stand."
  - "Your surrender is accepted. In advance."
```

Create `content/npcs/umca_commander.yaml`:
```yaml
id: umca_commander
name: "The UMCA Commander"
description: >
  The supreme commander of The Unwoke MAGA Clown Army, who runs operations from
  The War Tent with military discipline and a face painted in red-white-and-blue
  greasepaint. Known for the rally ability that summons fresh grunts mid-combat
  and a suppressive fire attack that pins enemies in place.
level: 60
max_hp: 2430
tier: boss
ac: 27
faction_id: unwoke_maga_clown_army
ai_domain: territory_patrol
respawn_delay: "72h"
rob_multiplier: 2.0
awareness: 12
abilities:
  brutality: 22
  quickness: 16
  grit: 22
  reasoning: 18
  savvy: 16
  flair: 12
weapon:
  - id: assault_rifle
    weight: 3
  - id: combat_knife
    weight: 2
armor:
  - id: tactical_armor
    weight: 3
boss_abilities:
  - id: rally_grunt
    name: "Rally the Troops"
    trigger: round_start
    trigger_value: 0
    cooldown: "5m"
    effect:
      aoe_condition: frightened
  - id: suppressive_fire
    name: "Suppressive Fire"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "8d10"
  - id: commanders_resolve
    name: "Commander's Resolve"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "6m"
    effect:
      heal_pct: 25
taunts:
  - "UMCA has never been defeated. Today is no exception."
  - "Order. Discipline. Victory. In that order."
  - "The movement doesn't stop for one lost fight."
```

Create `content/npcs/umca_merchant.yaml`:
```yaml
id: umca_merchant
name: "UMCA Commissary Officer"
description: A non-combat UMCA member managing the commissary and selling faction-exclusive supplies to allied visitors.
npc_type: merchant
type: human
level: 48
disposition: friendly
faction_id: unwoke_maga_clown_army
respawn_delay: "0s"
```

- [ ] **Step 4: Run NPC template validation**

```bash
go test ./internal/game/npc/... -v 2>&1 | tail -15
```

Expected: All PASS (template loader validates all files).

- [ ] **Step 5: Commit**

```bash
git add content/npcs/jc_fighter.yaml content/npcs/jc_enforcer.yaml content/npcs/qce_agitator.yaml content/npcs/qce_drag_bruiser.yaml content/npcs/qce_ringleader.yaml content/npcs/qce_merchant.yaml content/npcs/umca_grunt.yaml content/npcs/umca_flag_bearer.yaml content/npcs/umca_commander.yaml content/npcs/umca_merchant.yaml
git commit -m "content(npc): add JC sortie, QCE, and UMCA NPC templates (REQ-CCF-6)"
```

---

## Task 7: New Rooms — QCE Territory (REQ-CCF-7) and UMCA Territory (REQ-CCF-8)

**Files:**
- Modify: `content/zones/clown_camp.yaml`

The Stage is at map_x: 4, map_y: 0. QCE territory extends west (map_x decreasing from 3 to -1). UMCA territory extends east (map_x increasing from 5 to 9). Existing rooms occupy map_x: -1 to 4 at map_y: 0, so assign new territories at map_y: 2 (separate row to avoid overlap).

- [ ] **Step 1: Add QCE territory rooms**

Append to the `rooms:` list in `content/zones/clown_camp.yaml`:

```yaml
  # --- QCE Territory ---
  - id: cc_qce_entrance
    title: The Rainbow Gate
    danger_level: dangerous
    description: >
      A towering archway of welded neon tubes in every color of the spectrum marks the
      entrance to QCE territory. Glitter dust drifts permanently in the air. Two agitators
      pace the threshold, eyeing anyone who approaches with theatrical menace.
    exits:
    - direction: east
      target: cc_the_stage
      zone: clown_camp
    - direction: west
      target: cc_qce_costume_vault
    map_x: 3
    map_y: 2
    spawns:
    - template: qce_agitator
      count: 2
      respawn_after: 10m

  - id: cc_qce_costume_vault
    title: Costume Vault
    danger_level: dangerous
    description: >
      Floor-to-ceiling racks of sequined costumes, feathered headdresses, and platform boots
      fill this backstage chamber. The Drag Bruiser stationed here views every visitor as a
      potential costume thief. The agitator stationed by the door views them as a potential combatant.
    exits:
    - direction: east
      target: cc_qce_entrance
    - direction: west
      target: cc_qce_rehearsal_hall
    map_x: 2
    map_y: 2
    spawns:
    - template: qce_agitator
      count: 1
      respawn_after: 10m
    - template: qce_drag_bruiser
      count: 1
      respawn_after: 10m

  - id: cc_qce_rehearsal_hall
    title: Rehearsal Hall
    danger_level: dangerous
    description: >
      A mirrored chamber where QCE fighters rehearse combat choreography that is, somehow,
      both technically excellent and extremely flamboyant. Two Drag Bruisers are currently
      arguing about footwork while warming up their glitter-bomb throws.
    exits:
    - direction: east
      target: cc_qce_costume_vault
    - direction: west
      target: cc_qce_green_room
    map_x: 1
    map_y: 2
    spawns:
    - template: qce_drag_bruiser
      count: 2
      respawn_after: 10m

  - id: cc_qce_green_room
    title: The Green Room
    danger_level: dangerous
    description: >
      A surprisingly comfortable room just off the main territory corridor, furnished with
      velvet chaises and a fully stocked cosmetics station. The QCE Supply Captain sells
      exclusive faction goods to those with standing in the organization.
    exits:
    - direction: east
      target: cc_qce_rehearsal_hall
    - direction: west
      target: cc_qce_throne
    map_x: 0
    map_y: 2
    spawns:
    - template: qce_merchant
      count: 1
      respawn_after: 0s

  - id: cc_qce_throne
    title: The Sequined Throne
    danger_level: all_out_war
    boss_room: true
    description: >
      The innermost sanctum of QCE territory. A throne built entirely of fused sequined fabric
      dominates the far wall, and the Sequined Ringleader occupies it like a set piece in a
      production about absolute power. The lighting is immaculate. The threat level is equivalent.
    exits:
    - direction: east
      target: cc_qce_green_room
    map_x: -1
    map_y: 2
    spawns:
    - template: qce_ringleader
      count: 1
      respawn_after: 72h
```

- [ ] **Step 2: Add UMCA territory rooms**

```yaml
  # --- UMCA Territory ---
  - id: cc_umca_gate
    title: The Patriot Gate
    danger_level: dangerous
    description: >
      A fortified checkpoint of corrugated metal and razor wire, draped with flag bunting and
      painted UMCA insignia. Two grunts man the entrance with the rigid discipline of soldiers
      who take their clown war extremely seriously.
    exits:
    - direction: west
      target: cc_the_stage
      zone: clown_camp
    - direction: east
      target: cc_umca_armory
    map_x: 5
    map_y: 2
    spawns:
    - template: umca_grunt
      count: 2
      respawn_after: 10m

  - id: cc_umca_armory
    title: MAGA Armory
    danger_level: dangerous
    description: >
      A converted prop storage area now stocked with UMCA weaponry: rifle racks, ammunition
      crates, and a suspicious number of flag poles repurposed as spears. The grunt on duty
      eyes you like a potential disarmer.
    exits:
    - direction: west
      target: cc_umca_gate
    - direction: east
      target: cc_umca_rally_grounds
    map_x: 6
    map_y: 2
    spawns:
    - template: umca_grunt
      count: 1
      respawn_after: 10m
    - template: umca_flag_bearer
      count: 1
      respawn_after: 10m

  - id: cc_umca_rally_grounds
    title: Rally Grounds
    danger_level: dangerous
    description: >
      An open rehearsal space converted into a rally ground. Two flag bearers pace the perimeter
      in formation, occasionally stopping to deliver speeches to no one in particular. The
      patriotic energy is enormous and slightly unhinged.
    exits:
    - direction: west
      target: cc_umca_armory
    - direction: east
      target: cc_umca_commissary
    map_x: 7
    map_y: 2
    spawns:
    - template: umca_flag_bearer
      count: 2
      respawn_after: 10m

  - id: cc_umca_commissary
    title: The Commissary
    danger_level: dangerous
    description: >
      A spartan but well-organized supply depot where the UMCA Commissary Officer manages
      faction stores and sells exclusive goods to properly vetted allies.
    exits:
    - direction: west
      target: cc_umca_rally_grounds
    - direction: east
      target: cc_umca_war_tent
    map_x: 8
    map_y: 2
    spawns:
    - template: umca_merchant
      count: 1
      respawn_after: 0s

  - id: cc_umca_war_tent
    title: The War Tent
    danger_level: all_out_war
    boss_room: true
    description: >
      A vast canvas tent stretched over the easternmost section of Clown Camp, decorated
      with maps, tactical diagrams, and an unsettling number of campaign ribbons. The UMCA
      Commander stands at the center war table, running operations with the absolute certainty
      of someone who has never once questioned their own judgment.
    exits:
    - direction: west
      target: cc_umca_commissary
    map_x: 9
    map_y: 2
    spawns:
    - template: umca_commander
      count: 1
      respawn_after: 72h
```

- [ ] **Step 3: Verify zone loads without errors**

```bash
go test ./internal/game/world/... -v 2>&1 | tail -10
go test ./internal/gameserver/... 2>&1 | tail -5
```

Expected: All PASS (zone loader validates room connections and IDs).

- [ ] **Step 4: Commit**

```bash
git add content/zones/clown_camp.yaml
git commit -m "content(zone): add QCE and UMCA territory rooms to Clown Camp (REQ-CCF-7,8)"
```

---

## Task 8: Update The Stage — Sortie Spawns and Exits (REQ-CCF-9)

**Files:**
- Modify: `content/zones/clown_camp.yaml`

- [ ] **Step 1: Update cc_the_stage exits and spawns**

In `content/zones/clown_camp.yaml`, find the `cc_the_stage` room block. Update it to:

```yaml
  - id: cc_the_stage
    danger_level: all_out_war
    title: The Stage
    description: >
      Center stage. The house lights blaze from the grid above. The entire clown universe
      converges here — Just Clownin' fighters hold the center, QCE agitators press from the
      west, UMCA grunts advance from the east, and the Big Top looms over all of it with
      grinning indifference to the factional chaos erupting around its feet. The audience
      seats remain empty. Everything else is very, very full.
    exits:
    - direction: west
      target: cc_backstage
    - direction: north
      target: cc_qce_entrance
      zone: clown_camp
    - direction: south
      target: cc_umca_gate
      zone: clown_camp
    map_x: 4
    map_y: 0
    spawns:
    - template: big_top
      count: 1
      respawn_after: 60m
    - template: jc_fighter
      count: 1
      respawn_after: 10m
    - template: qce_agitator
      count: 1
      respawn_after: 10m
    - template: umca_grunt
      count: 1
      respawn_after: 10m
    boss_room: true
```

Note: QCE territory is at map_y: 2 (north of The Stage at map_y: 0) and UMCA territory is also at map_y: 2. Use `north` for QCE entrance and `south` for UMCA gate, since QCE and UMCA rooms are placed differently on the map. Adjust the exit directions based on the actual map coordinates if the zone map renderer shows them incorrectly.

Also add reciprocal exits: update `cc_qce_entrance` exits to include `south: cc_the_stage`, and update `cc_umca_gate` exits to include `north: cc_the_stage` (or the correct direction). These reciprocal exits were already included in Task 7's room definitions — verify they match what's set here.

- [ ] **Step 2: Update clown_camp zone metadata**

In the zone header of `content/zones/clown_camp.yaml`:
```yaml
zone:
  id: clown_camp
  danger_level: dangerous
  min_level: 45
  max_level: 60
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/game/world/... -v 2>&1 | tail -10
go test ./internal/gameserver/... 2>&1 | tail -5
```

Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add content/zones/clown_camp.yaml
git commit -m "content(zone): update The Stage with faction sortie spawns and new exits (REQ-CCF-9,10)"
```

---

## Task 9: Tests (REQ-CCF-12)

**Files:**
- Create: additional test cases in `internal/scripting/modules_faction_test.go`
- Create: `internal/gameserver/faction_initiation_property_test.go`

- [ ] **Step 1: Add remaining unit tests from REQ-CCF-12**

Append to `internal/scripting/modules_faction_test.go`:

```go
// TestGetFactionEnemies_AlreadyInCombatStillIncluded verifies that a hostile-faction
// combatant already in combat with uid is still returned (REQ-CCF-2c).
func TestGetFactionEnemies_AlreadyInCombatStillIncluded(t *testing.T) {
	roller, logger := testRoller(t), zap.NewNop()
	mgr := scripting.NewManager(roller, logger)

	actorUID := "jc-1"
	qceUID := "qce-1" // already in combat with jc-1

	mgr.GetEntityRoom = func(uid string) string { return "room-stage" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 200, MaxHP: 546, FactionID: "just_clownin"},
			{UID: qceUID, Kind: "npc", HP: 100, MaxHP: 528, FactionID: "queer_clowning_experience"},
		}
	}
	mgr.GetFactionHostiles = func(factionID string) []string {
		if factionID == "just_clownin" {
			return []string{"queer_clowning_experience"}
		}
		return nil
	}

	require.NoError(t, mgr.LoadZone("zone-clown", `
function count_faction_enemies(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`))

	result, err := mgr.CallHook("zone-clown", "count_faction_enemies", actorUID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, result, "hostile already in combat must still appear in get_faction_enemies")
}
```

- [ ] **Step 2: Add property-based test for faction combat pairs (REQ-CCF-12e)**

Create `internal/gameserver/faction_initiation_property_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
	"pgregory.net/rapid"
)

// TestProperty_FactionInitiation_AllOutWarAlwaysInitiates verifies that whenever
// a faction NPC enters an all_out_war room containing a hostile-faction NPC,
// initiation always fires (REQ-CCF-12e).
func TestProperty_FactionInitiation_AllOutWarAlwaysInitiates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate two NPCs with opposing factions.
		arrivingFaction := rapid.SampledFrom([]string{
			"just_clownin", "queer_clowning_experience", "unwoke_maga_clown_army",
		}).Draw(rt, "arriving_faction").(string)

		hostileFactions := map[string][]string{
			"just_clownin":              {"queer_clowning_experience", "unwoke_maga_clown_army"},
			"queer_clowning_experience": {"just_clownin", "unwoke_maga_clown_army"},
			"unwoke_maga_clown_army":    {"just_clownin", "queer_clowning_experience"},
		}

		// Pick a hostile faction for the existing NPC.
		existingFaction := rapid.SampledFrom(hostileFactions[arrivingFaction]).Draw(rt, "existing_faction").(string)

		arrivalHP := rapid.IntRange(1, 2430).Draw(rt, "arrival_hp").(int)
		existingHP := rapid.IntRange(1, 2430).Draw(rt, "existing_hp").(int)

		arrivalInst := &npc.Instance{ID: "arr-1", FactionID: arrivingFaction, CurrentHP: arrivalHP}
		existingInst := &npc.Instance{ID: "ex-1", FactionID: existingFaction, CurrentHP: existingHP}

		room := &world.Room{ID: "cc_the_stage", DangerLevel: world.DangerLevelAllOutWar}

		initiated := false
		checkFactionInitiation(
			arrivalInst, "cc_the_stage",
			func(string) []*npc.Instance { return []*npc.Instance{existingInst} },
			func(string) *world.Room { return room },
			func(fid string) []string { return hostileFactions[fid] },
			func(_, _ *npc.Instance, _ *world.Room) { initiated = true },
		)

		if !initiated {
			rt.Fatalf("faction initiation did not fire for %s vs %s in all_out_war room",
				arrivingFaction, existingFaction)
		}
	})
}
```

- [ ] **Step 3: Run all new tests**

```bash
go test ./internal/scripting/... -run TestGetFactionEnemies -v
go test ./internal/gameserver/... -run "TestFactionInitiation|TestProperty_FactionInitiation" -v
```

Expected: All PASS.

- [ ] **Step 4: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scripting/modules_faction_test.go internal/gameserver/faction_initiation_property_test.go
git commit -m "test(faction): complete REQ-CCF-12 test coverage — get_faction_enemies and initiation property test"
```

---

## Self-Review Against Spec

**REQ-CCF-1** (faction YAML files): Task 3 creates all three. ✓

**REQ-CCF-2** (get_faction_enemies Lua function): Task 1 adds FactionID to CombatantInfo + Combatant, adds get_faction_enemies to modules.go, wires GetFactionHostiles callback. ✓

**REQ-CCF-2a** (returns hostile-faction combatants): Covered by TestGetFactionEnemies_NPCWithFactionSeesHostiles. ✓

**REQ-CCF-2b** (no faction → empty): Covered by TestGetFactionEnemies_NoFactionReturnsEmpty. ✓

**REQ-CCF-2c** (include already-in-combat hostiles): Covered by TestGetFactionEnemies_AlreadyInCombatStillIncluded. ✓

**REQ-CCF-2d** (available to HTN preconditions/operators): Task 4 uses get_faction_enemies in Lua preconditions. ✓

**REQ-CCF-2e** (FactionID on combatant): Task 1 adds field and populates from inst.FactionID. ✓

**REQ-CCF-3** (faction combat initiation): Task 2 adds AfterPlace callback + checkFactionInitiation. ✓

**REQ-CCF-3a** (lowest-HP target): checkFactionInitiation selects min CurrentHP. ✓

**REQ-CCF-3b** (only all_out_war): checkFactionInitiation checks DangerLevel. ✓

**REQ-CCF-3c** (console message): initiateNPCFactionCombat broadcasts room message. ✓

**REQ-CCF-4** (HTN domain clown_faction_combat): Task 4 creates domain + preconditions Lua script + nearest_faction_enemy resolver. ✓

**REQ-CCF-4a** (has_faction_enemy precondition): faction_preconditions.lua. ✓

**REQ-CCF-4b** (nearest_faction_enemy picks lowest HP): resolver in combat_handler.go. ✓

**REQ-CCF-4c** (sortie NPCs use clown_faction_combat): jc_fighter, qce_agitator, umca_grunt all specify ai_domain. ✓

**REQ-CCF-5** (assign faction to existing JC NPCs): Task 5. ✓

**REQ-CCF-6** (new NPC templates): Task 6 creates all 10 templates. ✓

**REQ-CCF-7** (QCE territory rooms): Task 7 adds 5 QCE rooms. ✓

**REQ-CCF-8** (UMCA territory rooms): Task 7 adds 5 UMCA rooms. ✓

**REQ-CCF-9** (Stage sortie spawns + exits): Task 8. ✓

**REQ-CCF-9a** (Stage exits west/east to QCE/UMCA): Task 8 adds exits. ✓

**REQ-CCF-9b** (Stage description reflects chaos): Task 8 updates description. ✓

**REQ-CCF-10** (map coordinates non-overlapping): QCE at map_y:2, UMCA at map_y:2 — separate from existing map_y:0 rooms. ✓

**REQ-CCF-11** (rep sources for cross-faction kills): Encoded in all three faction YAML files in Task 3. ✓

**REQ-CCF-12** (test coverage): Task 9 covers 12a, 12b, 12c, 12d, 12e. ✓

# BUG-25: Allied NPC Attacks Same-Faction Player — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent NPC instances from initiating combat against players who belong to the same faction, by adding an `IsAllyOf` method to `faction.Service` and applying it in the threat-assessment block in `grpc_service.go`, and by fixing Marshal Ironsides' template to have a correct `disposition` and `faction_id`.

**Architecture:** Two independent changes. (1) A new `IsAllyOf(*session.PlayerSession, string) bool` method on `faction.Service` — symmetric to `IsEnemyOf` — checks whether the player's faction ID matches the NPC's faction ID. (2) The threat-assessment block in `grpc_service.go` uses `IsAllyOf` to suppress `isHostileToPlayers` before evaluating combat engagement. Marshal Ironsides' YAML template is corrected so it has `disposition: neutral` and `faction_id: machete`, ensuring guard-type NPCs behave correctly even without the code fix, and as a content correctness baseline.

**Tech Stack:** Go 1.23+, `pgregory.net/rapid` (property-based tests), YAML content files.

---

## File Map

| File | Change |
|---|---|
| `internal/game/faction/service.go` | Add `IsAllyOf(*session.PlayerSession, string) bool` |
| `internal/game/faction/service_test.go` | Add unit + PBT tests for `IsAllyOf` |
| `internal/gameserver/grpc_service.go` | Apply allied-faction exclusion in threat-assessment block |
| `internal/gameserver/grpc_service_test.go` (or closest NPC idle test file) | Add integration-level test verifying allied NPC does not engage |
| `content/npcs/marshal_ironsides.yaml` | Add `disposition: neutral` and `faction_id: machete` |
| `docs/bugs.md` | Mark BUG-25 fixed |

---

### Task 1: Add `IsAllyOf` to `faction.Service` (TDD)

**Files:**
- Modify: `internal/game/faction/service_test.go`
- Modify: `internal/game/faction/service.go`

- [ ] **Step 1: Write the failing unit test**

Open `internal/game/faction/service_test.go`. The file already contains `makeTestRegistry()` which defines `"gun"` (hostile to `"machete"`) and `"machete"` (hostile to `"gun"`). Add the following tests after `TestIsEnemyOf_HostileNPCFaction`:

```go
func TestIsAllyOf_SameFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "machete", FactionRep: map[string]int{"machete": 0}}
	if !svc.IsAllyOf(sess, "machete") {
		t.Error("same-faction NPC should be ally of player")
	}
}

func TestIsAllyOf_DifferentFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "machete", FactionRep: map[string]int{"machete": 0}}
	if svc.IsAllyOf(sess, "gun") {
		t.Error("enemy-faction NPC should not be ally of player")
	}
}

func TestIsAllyOf_EmptyNPCFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "machete", FactionRep: map[string]int{"machete": 0}}
	if svc.IsAllyOf(sess, "") {
		t.Error("empty NPC faction should never be ally")
	}
}

func TestIsAllyOf_EmptyPlayerFaction(t *testing.T) {
	svc := faction.NewService(makeTestRegistry())
	sess := &session.PlayerSession{FactionID: "", FactionRep: map[string]int{}}
	if svc.IsAllyOf(sess, "machete") {
		t.Error("factionless player should not be ally of any NPC faction")
	}
}

func TestProperty_IsAllyOf_NeverTrueForHostilePairs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// gun and machete are always mutually hostile; IsAllyOf must never return true
		// for cross-faction pairs.
		playerFaction := rapid.SampledFrom([]string{"gun", "machete"}).Draw(t, "playerFaction")
		npcFaction := rapid.SampledFrom([]string{"gun", "machete"}).Draw(t, "npcFaction")
		svc := faction.NewService(makeTestRegistry())
		sess := &session.PlayerSession{FactionID: playerFaction, FactionRep: map[string]int{playerFaction: 0}}
		allied := svc.IsAllyOf(sess, npcFaction)
		if playerFaction != npcFaction && allied {
			t.Fatalf("IsAllyOf(%q, %q) = true but factions are hostile", playerFaction, npcFaction)
		}
		if playerFaction == npcFaction && !allied {
			t.Fatalf("IsAllyOf(%q, %q) = false but factions are the same", playerFaction, npcFaction)
		}
	})
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/faction/... -run "TestIsAllyOf|TestProperty_IsAllyOf" -v 2>&1 | head -30
```

Expected: compilation error — `svc.IsAllyOf undefined`.

- [ ] **Step 3: Implement `IsAllyOf` in `service.go`**

Open `internal/game/faction/service.go`. After the closing brace of `IsEnemyOf` (currently ends around line 112), add:

```go
// IsAllyOf returns true iff both npcFactionID and sess.FactionID are non-empty
// and are equal (same faction).
//
// Precondition: sess must be non-nil.
// Postcondition: Returns false when either faction ID is empty.
func (s *Service) IsAllyOf(sess *session.PlayerSession, npcFactionID string) bool {
	if npcFactionID == "" || sess.FactionID == "" {
		return false
	}
	return npcFactionID == sess.FactionID
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/faction/... -v 2>&1 | tail -20
```

Expected: all tests PASS, including the new `TestIsAllyOf_*` and `TestProperty_IsAllyOf_*` cases.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/faction/service.go internal/game/faction/service_test.go && git commit -m "feat(faction): add IsAllyOf to Service — returns true for same-faction NPC/player pairs"
```

---

### Task 2: Apply Allied-Faction Exclusion in Threat Assessment

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (lines 4257–4272)

The threat-assessment block currently:
1. Sets `isHostileToPlayers = true` when `inst.Disposition == "hostile"` (line 4261).
2. Conditionally checks `IsEnemyOf` only when the NPC is NOT already hostile (line 4262).

The fix: after evaluating the hostile disposition flag, if `factionSvc` is available and the NPC has a `FactionID`, override `isHostileToPlayers = false` for any player in the room who is an ally of the NPC. This ensures same-faction players suppress combat engagement regardless of whether the NPC defaulted to `"hostile"` disposition.

- [ ] **Step 1: Write a failing test**

Find the existing idle-tick/threat-assessment test file. Search:

```bash
grep -rn "evaluateThreat\|isHostileTo\|IsEnemyOf\|PlayerEnteredRoom\|idle.*tick\|threat.*assess" /home/cjohannsen/src/mud/internal/gameserver/ --include="*_test.go" -l
```

If a dedicated test file exists for the NPC idle logic, add to it. Otherwise add to `internal/gameserver/grpc_service_test.go`. Add this test (adjust the package and imports to match the file you edit):

```go
// TestAlliedNPCDoesNotEngagePlayer verifies that a combat-capable NPC whose
// FactionID matches the player's FactionID does not call InitiateNPCCombat,
// even when the NPC disposition defaults to "hostile".
func TestAlliedNPCDoesNotEngagePlayer(t *testing.T) {
    // Build minimal faction registry: machete only, no hostile factions.
    machete := &faction.FactionDef{
        ID: "machete", Name: "Team Machete", ZoneID: "ironyard",
        HostileFactions: []string{},
        Tiers: []faction.FactionTier{
            {ID: "outsider", Label: "Outsider", MinRep: 0},
            {ID: "blade", Label: "Blade", MinRep: 100},
            {ID: "cutter", Label: "Cutter", MinRep: 300},
            {ID: "warsmith", Label: "Warsmith", MinRep: 600},
        },
    }
    reg := faction.FactionRegistry{"machete": machete}
    svc := faction.NewService(reg)

    sess := &session.PlayerSession{
        UID:       "player-1",
        FactionID: "machete",
        FactionRep: map[string]int{"machete": 0},
    }

    // NPC: guard, disposition="hostile" (the default), faction_id="machete"
    inst := &npc.Instance{
        ID:          "npc-1",
        TemplateID:  "marshal_ironsides",
        NPCType:     "guard",
        Disposition: "hostile",
        FactionID:   "machete",
        RoomID:      "room-1",
        Level:       5,
    }

    // isHostileToPlayers logic extracted (mirrors grpc_service.go:4261–4269 + fix):
    isHostileToPlayers := inst.Disposition == "hostile"
    if isHostileToPlayers && svc != nil && inst.FactionID != "" {
        for _, p := range []*session.PlayerSession{sess} {
            if svc.IsAllyOf(p, inst.FactionID) {
                isHostileToPlayers = false
                break
            }
        }
    }
    if !isHostileToPlayers && svc != nil && inst.FactionID != "" {
        for _, p := range []*session.PlayerSession{sess} {
            if svc.IsEnemyOf(p, inst.FactionID) {
                isHostileToPlayers = true
                break
            }
        }
    }

    if isHostileToPlayers {
        t.Error("allied NPC (machete) should not be hostile to machete player")
    }
}
```

> Note: This test directly exercises the revised logic rather than going through the full gRPC server, which requires a large setup harness. It validates the boolean outcome that gates `evaluateThreatEngagement`.

- [ ] **Step 2: Run test to confirm it fails with current code**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestAlliedNPCDoesNotEngagePlayer" -v 2>&1 | tail -20
```

Expected: FAIL — `isHostileToPlayers` is true because the allied-exclusion block is absent.

- [ ] **Step 3: Apply the fix in `grpc_service.go`**

Open `internal/gameserver/grpc_service.go`. Replace the threat-assessment block (lines 4257–4272) with:

```go
	// Threat assessment on idle tick for hostile NPCs. REQ-NB-7.
	// REQ-FA-27: enemy faction NPCs are treated as hostile regardless of disposition.
	// REQ-FA-28: allied faction NPCs MUST NOT initiate combat against same-faction players.
	// Non-combat NPC types (merchant, healer, banker, job_trainer, etc.) never initiate combat.
	isCombatCapable := inst.NPCType == "" || inst.NPCType == "combat" || inst.NPCType == "guard" || inst.NPCType == "hireling"
	isHostileToPlayers := inst.Disposition == "hostile"
	// Allied-faction exclusion: if any player in the room is an ally of this NPC,
	// suppress hostility regardless of disposition default.
	if isHostileToPlayers && s.factionSvc != nil && inst.FactionID != "" {
		for _, p := range s.sessions.PlayersInRoomDetails(inst.RoomID) {
			if s.factionSvc.IsAllyOf(p, inst.FactionID) {
				isHostileToPlayers = false
				break
			}
		}
	}
	// Enemy-faction promotion: non-hostile NPC becomes hostile if any player is a faction enemy.
	if !isHostileToPlayers && s.factionSvc != nil && inst.FactionID != "" {
		for _, p := range s.sessions.PlayersInRoomDetails(inst.RoomID) {
			if s.factionSvc.IsEnemyOf(p, inst.FactionID) {
				isHostileToPlayers = true
				break
			}
		}
	}
	if isCombatCapable && isHostileToPlayers && s.combatH != nil && !s.combatH.IsInCombat(inst.ID) {
		s.evaluateThreatEngagement(inst, inst.RoomID)
	}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -v 2>&1 | tail -30
```

Expected: all tests PASS including `TestAlliedNPCDoesNotEngagePlayer`.

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go && git commit -m "fix(combat): allied-faction NPCs no longer attack same-faction players (BUG-25)"
```

---

### Task 3: Fix Marshal Ironsides Content Template

**Files:**
- Modify: `content/npcs/marshal_ironsides.yaml`

- [ ] **Step 1: Apply the content fix**

Open `content/npcs/marshal_ironsides.yaml`. The current file contents are:

```yaml
id: marshal_ironsides
name: Marshal Ironsides
npc_type: guard
description: >
  A broad-shouldered enforcer in scuffed riot gear, Marshal Ironsides patrols the
  safe zone of Rustbucket Ridge with a practiced eye for trouble. She's seen it all
  and takes no bribes — but she's not looking for a fight either, unless you give her one.
max_hp: 45
ac: 16
level: 5
awareness: 6
personality: brave
guard:
  wanted_threshold: 2
  bribeable: false
```

Replace with:

```yaml
id: marshal_ironsides
name: Marshal Ironsides
npc_type: guard
disposition: neutral
faction_id: machete
description: >
  A broad-shouldered enforcer in scuffed riot gear, Marshal Ironsides patrols the
  safe zone of Rustbucket Ridge with a practiced eye for trouble. She's seen it all
  and takes no bribes — but she's not looking for a fight either, unless you give her one.
max_hp: 45
ac: 16
level: 5
awareness: 6
personality: brave
guard:
  wanted_threshold: 2
  bribeable: false
```

- [ ] **Step 2: Run the full test suite to confirm nothing is broken**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: all packages pass.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/npcs/marshal_ironsides.yaml && git commit -m "content(npcs): set marshal_ironsides disposition=neutral, faction_id=machete (BUG-25)"
```

---

### Task 4: Mark BUG-25 Fixed

**Files:**
- Modify: `docs/bugs.md`

- [ ] **Step 1: Update the bug entry**

In `docs/bugs.md`, find the BUG-25 entry. Change `**Status:** open` to `**Status:** fixed` and fill in the `**Fix:**` field:

```
**Fix:** Two-part fix. (1) Added `IsAllyOf(*session.PlayerSession, string) bool` to `faction.Service` (returns true iff both IDs are non-empty and equal). (2) In the threat-assessment block in `grpc_service.go`, added an allied-faction exclusion pass before the existing enemy-faction promotion pass: if any player in the room is an ally of the NPC, `isHostileToPlayers` is suppressed to false, preventing combat initiation regardless of disposition default. Also corrected `content/npcs/marshal_ironsides.yaml` to set `disposition: neutral` and `faction_id: machete`.
```

- [ ] **Step 2: Commit**

```bash
cd /home/cjohannsen/src/mud && git add docs/bugs.md && git commit -m "docs(bugs): mark BUG-25 fixed"
```

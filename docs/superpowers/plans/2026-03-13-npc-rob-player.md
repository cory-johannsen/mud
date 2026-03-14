# NPC Rob Player on Defeat — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a player is defeated in combat, each living NPC that has a rob multiplier steals a percentage of the player's currency and holds it in their wallet until they die.

**Architecture:** Three changes — (1) add `RobMultiplier`/`RobPercent`/`Currency` fields to `npc.Template` and `npc.Instance`; (2) insert a rob loop into `combat_handler.go` at the player-defeat branch; (3) include `inst.Currency` in loot payout when NPCs die. YAML files for human-type combat NPCs get `rob_multiplier` values.

**Tech Stack:** Go, `pgregory.net/rapid` (property tests), YAML.

---

## Background: Key Code Locations

- **Defeat detection:** `internal/gameserver/combat_handler.go:1364` — `if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers()`
- **Player-defeat branch:** lines 1368–1380 — `endNarrative = "Everything goes dark."` then `broadcastFn`, then `removeDeadNPCsLocked`
- **Loot payout:** `combat_handler.go:2043` — `removeDeadNPCsLocked`; currency awarded at line 2062: `killer.Currency += result.Currency`
- **Session access:** `h.sessions.GetPlayer(c.ID)` — used throughout `combat_handler.go`
- **Currency persistence:** `h.currencySaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency)` — same pattern as existing loot currency save
- **`firstLivingPlayer`:** `combat_handler.go:2162` — helper that returns the first non-dead player session

---

## Chunk 1: NPC Data Model

### Task 1: Add RobMultiplier to Template; RobPercent and Currency to Instance

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`
- Test: `internal/game/npc/template_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/template_test.go`:

```go
// TestTemplate_RobMultiplier_DefaultsToZero verifies that rob_multiplier defaults
// to 0.0 when not present in YAML.
//
// Precondition: YAML has no rob_multiplier field.
// Postcondition: tmpl.RobMultiplier == 0.0.
func TestTemplate_RobMultiplier_DefaultsToZero(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
`
	tmpl, err := LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, 0.0, tmpl.RobMultiplier)
}

// TestTemplate_RobMultiplier_ParsesFromYAML verifies that rob_multiplier round-trips
// through YAML parsing.
//
// Precondition: YAML specifies rob_multiplier: 1.5.
// Postcondition: tmpl.RobMultiplier == 1.5.
func TestTemplate_RobMultiplier_ParsesFromYAML(t *testing.T) {
	yamlData := `
id: test-npc
name: Test
level: 1
max_hp: 10
ac: 10
perception: 0
rob_multiplier: 1.5
`
	tmpl, err := LoadTemplateFromBytes([]byte(yamlData))
	require.NoError(t, err)
	assert.Equal(t, 1.5, tmpl.RobMultiplier)
}

// TestInstance_RobPercent_ZeroWhenMultiplierZero verifies that Instance.RobPercent
// is 0 when the template RobMultiplier is 0.
//
// Precondition: tmpl.RobMultiplier == 0.
// Postcondition: inst.RobPercent == 0.
func TestInstance_RobPercent_ZeroWhenMultiplierZero(t *testing.T) {
	tmpl := &Template{
		ID: "t1", Name: "T", Level: 5, MaxHP: 10, AC: 10, Perception: 0,
		RobMultiplier: 0.0,
	}
	inst := NewInstance("i1", tmpl, "room1")
	assert.Equal(t, 0.0, inst.RobPercent)
	assert.Equal(t, 0, inst.Currency)
}

// TestProperty_Instance_RobPercent_InRange verifies that for any RobMultiplier > 0
// and level in [1,20], inst.RobPercent is in [5.0, 30.0].
//
// Uses rapid property-based testing (SWENG-5a).
func TestProperty_Instance_RobPercent_InRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		multiplier := rapid.Float64Range(0.1, 3.0).Draw(rt, "multiplier")

		tmpl := &Template{
			ID: "prop-rob", Name: "T", Level: level, MaxHP: 10, AC: 10, Perception: 0,
			RobMultiplier: multiplier,
		}
		inst := NewInstance(fmt.Sprintf("i-%d", level), tmpl, "room1")
		assert.GreaterOrEqual(rt, inst.RobPercent, 5.0,
			"RobPercent must be >= 5.0 when multiplier > 0")
		assert.LessOrEqual(rt, inst.RobPercent, 30.0,
			"RobPercent must be <= 30.0")
	})
}
```

Ensure `"fmt"` and `"pgregory.net/rapid"` are imported in the test file (check existing imports first).

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_RobMultiplier|TestInstance_RobPercent|TestProperty_Instance_RobPercent" -v 2>&1 | tail -20
```

Expected: `FAIL` — fields don't exist yet.

- [ ] **Step 3: Add RobMultiplier to Template**

In `internal/game/npc/template.go`, add after `CoolRank` field (the last field in Template struct):

```go
// RobMultiplier controls whether and how aggressively this NPC robs defeated
// players. 0.0 = never robs (default). 1.0 = baseline human aggression.
// Values > 1.0 represent especially predatory NPCs.
// Used at spawn to compute Instance.RobPercent.
RobMultiplier float64 `yaml:"rob_multiplier"`
```

- [ ] **Step 4: Add RobPercent and Currency to Instance**

In `internal/game/npc/instance.go`, add after the `CoolRank` field:

```go
// RobPercent is the fraction of a defeated player's currency this NPC steals,
// as a percentage in [5.0, 30.0]. 0 means this NPC never robs.
// Computed once at spawn from template RobMultiplier, level, and randomness.
RobPercent float64
// Currency is the NPC's wallet accumulated from robbing players.
// Added to loot payout when the NPC dies. Zero at spawn.
Currency int
```

- [ ] **Step 5: Add import for math in instance.go**

Check if `"math"` is already imported in `internal/game/npc/instance.go`. If not, add it. Also check for `"math/rand"` (already used for `pickWeighted`).

- [ ] **Step 6: Compute RobPercent in NewInstanceWithResolver**

In `NewInstanceWithResolver`, after the existing `CoolRank: tmpl.CoolRank,` assignment, add:

```go
RobPercent:    computeRobPercent(tmpl.RobMultiplier, tmpl.Level),
Currency:      0,
```

Then add the helper function at package level in `instance.go` (after `NewInstance`):

```go
// computeRobPercent calculates the rob percentage for an NPC at spawn time.
// Returns 0 if multiplier is 0 (NPC does not rob).
// Otherwise returns clamp((rand(5,20) + min(level,10)) * multiplier, 5.0, 30.0).
//
// Precondition: level >= 1.
// Postcondition: returns 0 if multiplier == 0; returns value in [5.0, 30.0] otherwise.
func computeRobPercent(multiplier float64, level int) float64 {
	if multiplier == 0 {
		return 0
	}
	base := 5 + rand.Intn(16) // [5, 20]
	levelBonus := level
	if levelBonus > 10 {
		levelBonus = 10
	}
	raw := float64(base+levelBonus) * multiplier
	if raw < 5.0 {
		raw = 5.0
	}
	if raw > 30.0 {
		raw = 30.0
	}
	return raw
}
```

- [ ] **Step 7: Run tests — expect PASS**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_RobMultiplier|TestInstance_RobPercent|TestProperty_Instance_RobPercent" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Run full npc package suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/template_test.go && git commit -m "feat(npc): add RobMultiplier/RobPercent/Currency fields for player robbery"
```

---

## Chunk 2: Rob Trigger and Loot Payout

### Task 2: Rob loop at player defeat + inst.Currency in loot payout

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/grpc_service_rob_test.go` (new file)

**Context for implementer:**

- `combat_handler.go:1364`: `if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers()` — defeat branch
- `combat_handler.go:1368`: `endNarrative = "Everything goes dark."` — player-defeat path
- `combat_handler.go:1375`: `h.broadcastFn(roomID, events)` — sends end event BEFORE rob
- `combat_handler.go:1376`: `h.removeDeadNPCsLocked(cbt)` — loot payout
- `combat_handler.go:2057–2072`: existing loot currency payout in `removeDeadNPCsLocked`
- Player sessions accessed via `h.sessions.GetPlayer(c.ID)` where `c.Kind == combat.KindPlayer`
- Currency saved via `h.currencySaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency)`
- `h.currencySaver` may be nil — check before calling
- Rob events need to be INCLUDED in the broadcast (`events` slice) so the player sees them before "Everything goes dark." Append rob events to `events` BEFORE the end event.

- [ ] **Step 1: Write failing tests (new file)**

Create `internal/gameserver/grpc_service_rob_test.go`. This file includes ALL tests for this task — both the rob trigger tests and the loot payout test — so that all tests are written before any implementation begins:

```go
package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
	"go.uber.org/zap/zaptest"
)

// newRobCombatHandler builds a minimal CombatHandler for rob tests.
func newRobCombatHandler(t *testing.T, roller *dice.Roller) (*CombatHandler, *session.Manager, *npc.Manager) {
	t.Helper()
	_, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	return combatHandler, sessMgr, npcMgr
}

// TestRob_PlayerDefeated_NPCReceivesCurrency verifies that when a player is defeated
// in combat, a living NPC with RobPercent > 0 gains currency from the player.
//
// Precondition: player has 100 currency; NPC has RobMultiplier=1.0, Level=1
//   (RobPercent in [5,20] after clamping with level bonus).
// Postcondition: inst.Currency > 0; sess.Currency < 100; total unchanged.
func TestRob_PlayerDefeated_NPCReceivesCurrency(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Use fixed dice so NPC always hits player to death.
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)
	combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

	const roomID = "room_rob_basic"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-basic", Name: "Robber", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
		Abilities:     npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
		RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)
	require.Greater(t, inst.RobPercent, 0.0, "NPC must have non-zero RobPercent")

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_basic", Username: "Hero", CharName: "Hero",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 100

	_, err = combatHandler.Attack("u_rob_basic", "Robber")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drive combat until player is defeated.
	for i := 0; i < 20; i++ {
		if sess.CurrentHP <= 0 {
			break
		}
		combatHandler.mu.Lock()
		cbt := combatHandler.engine.GetCombat(roomID)
		combatHandler.mu.Unlock()
		if cbt == nil {
			break
		}
		combatHandler.resolveRound(roomID, cbt)
	}

	// After defeat, rob must have occurred.
	assert.Less(t, sess.Currency, 100, "player currency must have decreased after rob")
	npcCurrency := inst.Currency
	assert.Greater(t, npcCurrency, 0, "NPC must have non-zero currency after robbing player")
	assert.Equal(t, 100, sess.Currency+npcCurrency,
		"total currency must be conserved (player + NPC = original)")
}

// TestRob_BrokePlayer_NoRob verifies that a player with 0 currency is not robbed
// (no message, NPC currency stays 0).
//
// Precondition: player has 0 currency; NPC has RobPercent > 0.
// Postcondition: inst.Currency == 0; player currency unchanged (0).
func TestRob_BrokePlayer_NoRob(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)
	combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

	const roomID = "room_rob_broke"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-broke", Name: "Robber", Level: 5, MaxHP: 10, AC: 10, Perception: 0,
		Abilities:     npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
		RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_broke", Username: "Pauper", CharName: "Pauper",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 0

	_, err = combatHandler.Attack("u_rob_broke", "Robber")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	for i := 0; i < 20; i++ {
		if sess.CurrentHP <= 0 {
			break
		}
		combatHandler.mu.Lock()
		cbt := combatHandler.engine.GetCombat(roomID)
		combatHandler.mu.Unlock()
		if cbt == nil {
			break
		}
		combatHandler.resolveRound(roomID, cbt)
	}

	assert.Equal(t, 0, inst.Currency, "NPC currency must stay 0 when player is broke")
	assert.Equal(t, 0, sess.Currency, "player currency must stay 0")
}

// TestRob_MultipleNPCs_SequentialDeduction verifies that two robbing NPCs each
// take from the player's remaining currency sequentially.
//
// Precondition: player has 100 currency; two NPCs each with RobPercent=20.
// Postcondition: NPC1 takes 20, NPC2 takes 16 (from remaining 80); total conserved.
func TestRob_MultipleNPCs_SequentialDeduction(t *testing.T) {
	// This test directly calls the internal rob logic via robPlayersLocked.
	// We build a CombatHandler and call robPlayersLocked directly.
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 0}, logger)
	combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

	const roomID = "room_rob_multi"
	inst1, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-m1", Name: "Robber1", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
	}, roomID)
	require.NoError(t, err)
	inst1.RobPercent = 20.0 // set directly to ensure deterministic value

	inst2, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-m2", Name: "Robber2", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
	}, roomID)
	require.NoError(t, err)
	inst2.RobPercent = 20.0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_multi", Username: "Victim", CharName: "Victim",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 100

	// Build a minimal combat with one dead player and two living NPCs.
	cbt := &combat.Combat{
		RoomID: roomID,
		Combatants: []*combat.Combatant{
			{ID: "u_rob_multi", Kind: combat.KindPlayer, Dead: true},
			{ID: inst1.ID, Kind: combat.KindNPC, Dead: false},
			{ID: inst2.ID, Kind: combat.KindNPC, Dead: false},
		},
	}

	combatHandler.robPlayersLocked(cbt)

	total := sess.Currency + inst1.Currency + inst2.Currency
	assert.Equal(t, 100, total, "total currency must be conserved")
	assert.Equal(t, 64, sess.Currency, "player must have 64 after two sequential 20% robs")
	assert.Equal(t, 20, inst1.Currency, "NPC1 must have taken 20")
	assert.Equal(t, 16, inst2.Currency, "NPC2 must have taken 16 from remaining 80")
}

// TestRob_NonRobNPC_NoEffect verifies that an NPC with RobPercent=0 does not rob.
//
// Precondition: NPC with RobPercent=0; player has 50 currency.
// Postcondition: inst.Currency == 0; player currency unchanged.
func TestRob_NonRobNPC_NoEffect(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 0}, logger)
	combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

	const roomID = "room_rob_zero"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "robot-norob", Name: "Robot", Level: 3, MaxHP: 10, AC: 10, Perception: 0,
	}, roomID)
	require.NoError(t, err)
	// RobPercent defaults to 0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_zero", Username: "Victim", CharName: "Victim",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 50

	cbt := &combat.Combat{
		RoomID: roomID,
		Combatants: []*combat.Combatant{
			{ID: "u_rob_zero", Kind: combat.KindPlayer, Dead: true},
			{ID: inst.ID, Kind: combat.KindNPC, Dead: false},
		},
	}

	combatHandler.robPlayersLocked(cbt)

	assert.Equal(t, 0, inst.Currency)
	assert.Equal(t, 50, sess.Currency)
}

// TestProperty_Rob_CurrencyConserved verifies that total currency is always
// conserved across arbitrary player currency and NPC rob percentages.
func TestProperty_Rob_CurrencyConserved(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		playerCurrency := rapid.IntRange(0, 10000).Draw(rt, "playerCurrency")
		robPercent := rapid.Float64Range(5.0, 30.0).Draw(rt, "robPercent")

		logger := zaptest.NewLogger(t)
		roller := dice.NewLoggedRoller(&fixedDiceSource{val: 0}, logger)
		combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

		roomID := fmt.Sprintf("room_prop_%d", playerCurrency)
		uid := fmt.Sprintf("u_prop_%d", playerCurrency)

		inst, err := npcMgr.Spawn(&npc.Template{
			ID: fmt.Sprintf("robber-prop-%d", playerCurrency), Name: "R",
			Level: 1, MaxHP: 10, AC: 10, Perception: 0,
		}, roomID)
		require.NoError(rt, err)
		inst.RobPercent = robPercent

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: "V", CharName: "V",
			RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
		})
		require.NoError(rt, err)
		sess.Currency = playerCurrency

		cbt := &combat.Combat{
			RoomID: roomID,
			Combatants: []*combat.Combatant{
				{ID: uid, Kind: combat.KindPlayer, Dead: true},
				{ID: inst.ID, Kind: combat.KindNPC, Dead: false},
			},
		}

		combatHandler.robPlayersLocked(cbt)

		total := sess.Currency + inst.Currency
		assert.Equal(rt, playerCurrency, total,
			"total currency must be conserved")
		assert.GreaterOrEqual(rt, sess.Currency, 0,
			"player currency must not go negative")
	})
}

// TestRob_LootPayoutIncludesRobCurrency verifies that when an NPC that robbed a
// player is killed, the killer receives both loot-table currency and the robbed currency.
//
// Precondition: inst.Currency=25 (robbed); loot table generates 10 currency.
// Postcondition: killer receives 35 total; inst.Currency is zeroed.
func TestRob_LootPayoutIncludesRobCurrency(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 0}, logger)
	combatHandler, sessMgr, npcMgr := newRobCombatHandler(t, roller)

	const roomID = "room_rob_loot"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-loot", Name: "Robber", Level: 1, MaxHP: 10, AC: 10, Perception: 0,
		Loot: &npc.LootTable{Currency: npc.CurrencyRange{Min: 10, Max: 10}},
	}, roomID)
	require.NoError(t, err)
	inst.Currency = 25 // simulate having robbed a player

	killer, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_killer", Username: "Killer", CharName: "Killer",
		RoomID: roomID, CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	killer.Currency = 0

	// Mark NPC as dead in a fake combat.
	cbt := &combat.Combat{
		RoomID: roomID,
		Combatants: []*combat.Combatant{
			{ID: "u_rob_killer", Kind: combat.KindPlayer, Dead: false},
			{ID: inst.ID, Kind: combat.KindNPC, Dead: true},
		},
	}

	combatHandler.mu.Lock()
	combatHandler.removeDeadNPCsLocked(cbt)
	combatHandler.mu.Unlock()

	assert.Equal(t, 35, killer.Currency, "killer must receive loot (10) + robbed (25) = 35")
	assert.Equal(t, 0, inst.Currency, "NPC wallet must be zeroed after payout")
}
```

**Important:** `robPlayersLocked` does not yet exist — the test will fail to compile. That is expected at this stage.

- [ ] **Step 2: Confirm test file does not compile yet**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1 | head -10
```

Expected: compile error about `robPlayersLocked` undefined.

- [ ] **Step 3: Implement robPlayersLocked in combat_handler.go**

Add the following method to `combat_handler.go` (near the `firstLivingPlayer` helper at line ~2162):

```go
// robPlayersLocked executes the robbery sequence when all players are defeated.
// For each living NPC with RobPercent > 0, a fraction of each dead player's
// remaining currency is transferred to the NPC's Currency wallet.
// Rob messages are appended to the returned events slice for broadcast.
//
// Precondition: combatMu is held; cbt must not be nil.
// Postcondition: Each living robbing NPC has inst.Currency incremented by stolen
// amount; each dead player session has Currency decremented by same; events
// returned contain one narrative event per robbery that occurred.
func (h *CombatHandler) robPlayersLocked(cbt *combat.Combat) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
	var robbedSessions []*session.PlayerSession

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		inst, ok := h.npcMgr.Get(c.ID)
		if !ok || inst.RobPercent <= 0 {
			continue
		}
		// Rob each dead player.
		for _, pc := range cbt.Combatants {
			if pc.Kind != combat.KindPlayer || !pc.IsDead() {
				continue
			}
			sess, ok := h.sessions.GetPlayer(pc.ID)
			if !ok {
				continue
			}
			stolen := int(math.Floor(float64(sess.Currency) * inst.RobPercent / 100.0))
			if stolen <= 0 {
				continue
			}
			inst.Currency += stolen
			sess.Currency -= stolen
			robbedSessions = append(robbedSessions, sess)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_NARRATIVE,
				Narrative: fmt.Sprintf("The %s rifles through your pockets, taking %d rounds.", inst.Name(), stolen),
			})
		}
	}

	// Persist updated currency for all robbed players.
	if h.currencySaver != nil {
		for _, sess := range robbedSessions {
			if saveErr := h.currencySaver.SaveCurrency(context.Background(), sess.CharacterID, sess.Currency); saveErr != nil && h.logger != nil {
				h.logger.Warn("robPlayersLocked: SaveCurrency failed",
					zap.String("uid", sess.UID),
					zap.Int64("character_id", sess.CharacterID),
					zap.Error(saveErr),
				)
			}
		}
	}
	return events
}
```

Ensure `"math"` is imported in `combat_handler.go`. Check existing imports:

```bash
grep -n '"math"' /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go
```

If not present, add it to the import block.

- [ ] **Step 4: Call robPlayersLocked at player defeat**

In `combat_handler.go` at the player-defeat branch (around line 1368), insert the rob call BEFORE appending the end event so rob narratives are sent together with the defeat message:

**Find this block (around line 1364–1380):**
```go
if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers() {
    var endNarrative string
    if !cbt.HasLivingNPCs() {
        endNarrative = "Combat is over. You stand victorious."
    } else {
        endNarrative = "Everything goes dark."
    }
    events = append(events, &gamev1.CombatEvent{
        Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
        Narrative: endNarrative,
    })
    h.broadcastFn(roomID, events)
    h.removeDeadNPCsLocked(cbt)
    ...
```

**Replace with:**
```go
if !cbt.HasLivingNPCs() || !cbt.HasLivingPlayers() {
    var endNarrative string
    if !cbt.HasLivingNPCs() {
        endNarrative = "Combat is over. You stand victorious."
    } else {
        endNarrative = "Everything goes dark."
        // Rob defeated players before broadcasting the end event.
        robEvents := h.robPlayersLocked(cbt)
        events = append(events, robEvents...)
    }
    events = append(events, &gamev1.CombatEvent{
        Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
        Narrative: endNarrative,
    })
    h.broadcastFn(roomID, events)
    h.removeDeadNPCsLocked(cbt)
    ...
```

- [ ] **Step 5: Add inst.Currency to loot payout in removeDeadNPCsLocked**

In `removeDeadNPCsLocked` (around line 2057), update the loot currency block:

**Old:**
```go
if inst.Loot != nil {
    result := npc.GenerateLoot(*inst.Loot)
    // Award currency to the first living player.
    if result.Currency > 0 {
        if killer := h.firstLivingPlayer(cbt); killer != nil {
            killer.Currency += result.Currency
```

**New:**
```go
if inst.Loot != nil {
    result := npc.GenerateLoot(*inst.Loot)
    // Award currency (loot table + robbed wallet) to the first living player.
    totalCurrency := result.Currency + inst.Currency
    inst.Currency = 0 // zero wallet after payout
    if totalCurrency > 0 {
        if killer := h.firstLivingPlayer(cbt); killer != nil {
            killer.Currency += totalCurrency
```

Also update the guard from `if result.Currency > 0` to `if totalCurrency > 0` and replace `result.Currency` with `totalCurrency` in the `SaveCurrency` call below it.

**Important:** If `inst.Loot == nil`, the inst.Currency still needs to be paid out. Add an else branch:

```go
} else if inst.Currency > 0 {
    // NPC has no loot table but has robbed currency — still pay out.
    if killer := h.firstLivingPlayer(cbt); killer != nil {
        killer.Currency += inst.Currency
        inst.Currency = 0
        if h.currencySaver != nil {
            if saveErr := h.currencySaver.SaveCurrency(context.Background(), killer.CharacterID, killer.Currency); saveErr != nil && h.logger != nil {
                h.logger.Warn("SaveCurrency failed after rob payout",
                    zap.String("uid", killer.UID),
                    zap.Int64("character_id", killer.CharacterID),
                    zap.Error(saveErr),
                )
            }
        }
    }
}
```

- [ ] **Step 7: Run all rob tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestRob|TestProperty_Rob" -v 2>&1 | tail -30
```

Expected: all pass. Note: `TestRob_PlayerDefeated_NPCReceivesCurrency` uses the full combat loop and may require `resolveRound` to be accessible — if it is unexported and not available in tests, use the existing `Attack`+round-driving pattern from other test files (e.g., `grpc_service_grapple_test.go` uses `combatHandler.cancelTimer`). Read existing combat tests for the pattern before writing.

- [ ] **Step 8: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/gameserver/grpc_service_rob_test.go && git commit -m "feat(gameserver): NPCs rob defeated players; rob currency paid out on NPC death"
```

---

## Chunk 3: YAML Updates and FEATURES.md

### Task 3: Update NPC YAML files and mark feature complete

**Files:**
- Modify: selected files in `content/npcs/`
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Add rob_multiplier to human/mutant combat NPCs**

Add `rob_multiplier: 1.0` to each of these files (after the `abilities:` block):
- `content/npcs/ganger.yaml`
- `content/npcs/highway_bandit.yaml`
- `content/npcs/tarmac_raider.yaml`
- `content/npcs/mill_plain_thug.yaml`
- `content/npcs/motel_raider.yaml`
- `content/npcs/river_pirate.yaml`
- `content/npcs/strip_mall_scav.yaml`
- `content/npcs/industrial_scav.yaml`
- `content/npcs/outlet_scavenger.yaml`
- `content/npcs/scavenger.yaml`
- `content/npcs/alberta_drifter.yaml`
- `content/npcs/terminal_squatter.yaml`
- `content/npcs/cargo_cultist.yaml`

Add `rob_multiplier: 1.5` to:
- `content/npcs/lieutenant.yaml`
- `content/npcs/brew_warlord.yaml`
- `content/npcs/gravel_pit_boss.yaml`
- `content/npcs/commissar.yaml`
- `content/npcs/bridge_troll.yaml`

All other NPC YAML files: leave unchanged (default 0.0 = no rob).

- [ ] **Step 2: Verify YAML files load correctly**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v -run "TestLoadTemplates" 2>&1 | tail -10
```

If no `TestLoadTemplates` exists, run the full suite:

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 3: Mark FEATURES.md complete**

In `docs/requirements/FEATURES.md`, find:
```
- [ ] NPCs rob the player if the player is defeated in combat
```
Change to:
```
- [x] NPCs rob the player if the player is defeated in combat
```
(The sub-items `rob for 5-30%...` and `percentage taken randomized...` are implementation details — mark the parent item only.)

- [ ] **Step 4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/npcs/ docs/requirements/FEATURES.md && git commit -m "feat(content): set rob_multiplier on combat NPCs; mark feature complete"
```

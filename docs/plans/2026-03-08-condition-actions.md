# Condition-Applying Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Implement six PF2E actions that apply/interact with conditions (grapple, trip, hide, sneak, divert, escape), two new condition YAMLs (grabbed, hidden), extend sneak attack to trigger on grabbed targets and hidden attackers, and add hidden flat check for NPC attacks.

**Architecture:** Six new commands through the full CMD-1→CMD-7 pipeline. Infrastructure tasks add Combatant.Hidden bool, PlayerSession.GrabberID, CombatHandler.ApplyCombatCondition, CombatHandler.SetCombatantHidden, and extend applyPassiveFeats + ResolveRound for hidden mechanics. All resolver changes stay in round.go and combat_handler.go.

**Tech Stack:** Go, existing proto/gRPC infrastructure, pgregory.net/rapid for property tests.

---

## Codebase Context (Read This Before Implementing)

### Key File Locations
- Commands constants/registry: `internal/game/command/commands.go`
- Command parsers: `internal/game/command/<name>.go` (see feint.go, demoralize.go)
- gRPC handlers: `internal/gameserver/grpc_service.go` (dispatch type switch at line ~1131)
- Combat handler methods: `internal/gameserver/combat_handler.go`
- Combatant struct: `internal/game/combat/combat.go` (AttackMod is the last field, line ~100)
- Combat engine: `internal/game/combat/engine.go` (Combat struct)
- Round resolver: `internal/game/combat/round.go` (applyPassiveFeats at line ~216)
- Player session: `internal/game/session/manager.go` (PlayerSession struct)
- Bridge handlers: `internal/frontend/handlers/bridge_handlers.go` (bridgeHandlerMap at line ~49)
- Proto definition: `api/proto/game/v1/game.proto` (ClientMessage oneof, last field is demoralize=53)
- Condition YAMLs: `content/conditions/`
- NPC manager: `internal/game/npc/manager.go` (FindInRoom, InstancesInRoom, Get)

### Existing Patterns

**Command parser** (`internal/game/command/feint.go`):
```go
package command

type FeintRequest struct{ Target string }

func HandleFeint(args []string) (*FeintRequest, error) {
    req := &FeintRequest{}
    if len(args) >= 1 {
        req.Target = args[0]
    }
    return req, nil
}
```

**Bridge handler** (`internal/frontend/handlers/bridge_handlers.go`, bridgeFeint):
```go
func bridgeFeint(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorPrompt(bctx, "Usage: feint <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Feint{Feint: &gamev1.FeintRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}
```

**gRPC handler** (`internal/gameserver/grpc_service.go`, handleFeint):
```go
func (s *GameServiceServer) handleFeint(uid string, req *gamev1.FeintRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok { return nil, fmt.Errorf("player %q not found", uid) }
    if sess.Status != statusInCombat { return errorEvent("Feint is only available in combat."), nil }
    if req.GetTarget() == "" { return errorEvent("Usage: feint <target>"), nil }
    inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
    if inst == nil { return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil }
    if s.combatH == nil { return errorEvent("Combat handler unavailable."), nil }
    if err := s.combatH.SpendAP(uid, 1); err != nil { return errorEvent(err.Error()), nil }
    rollResult, err := s.dice.RollExpr("1d20")
    if err != nil { return nil, fmt.Errorf("handleFeint: rolling d20: %w", err) }
    roll := rollResult.Total()
    bonus := skillRankBonus(sess.Skills["grift"])
    total := roll + bonus
    dc := inst.Perception
    detail := fmt.Sprintf("Feint (grift DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
    if total < dc { return messageEvent(detail + " — failure. Your feint is transparent."), nil }
    if err := s.combatH.ApplyCombatantACMod(uid, inst.ID, -2); err != nil {
        s.logger.Warn("handleFeint: ApplyCombatantACMod failed", zap.String("npc_id", inst.ID), zap.Error(err))
    }
    return messageEvent(detail + fmt.Sprintf(" — success! %s is caught off-guard (-2 AC this round).", inst.Name())), nil
}
```

**Dispatch type switch** (grpc_service.go ~line 1131):
```go
case *gamev1.ClientMessage_Feint:
    return s.handleFeint(uid, p.Feint)
case *gamev1.ClientMessage_Demoralize:
    return s.handleDemoralize(uid, p.Demoralize)
```

**Combat.ApplyCondition** signature (engine.go line 214):
```go
func (c *Combat) ApplyCondition(uid, condID string, stacks, duration int) error
```
- `uid` is the combatant's ID in the combat.
- `condID` must be registered in `c.condRegistry`.
- `stacks=1, duration=-1` for permanent; `duration=1` for 1 round; `duration=-1` for encounter.

**ApplyCombatantACMod** (combat_handler.go line 316):
```go
func (h *CombatHandler) ApplyCombatantACMod(uid, targetID string, mod int) error
```
- `uid` is the acting player (used to find the room/combat).
- `targetID` is the combatant ID to modify.

**Condition YAML format** (`content/conditions/flat_footed.yaml`):
```yaml
id: flat_footed
name: Flat-Footed
description: |
  You are caught off guard, suffering -2 AC until the start of your next turn.
duration_type: round
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Alias conflicts to avoid:**
- `gr` is already used by `throw` command — do NOT use `gr` for grapple. Use `grp`.
- `cd` is already used by `combat_default` command — do NOT use `cd` for divert. Use `div`.

**Key helpers available in grpc_service.go package:**
- `skillRankBonus(rank string) int` — defined in `internal/gameserver/action_handler.go`
- `statusInCombat = int32(2)` — defined in `internal/gameserver/action_handler.go`
- `errorEvent(msg string) *gamev1.ServerEvent` — helper in grpc_service.go
- `messageEvent(msg string) *gamev1.ServerEvent` — helper in grpc_service.go
- `s.npcMgr.FindInRoom(roomID, target string) *npc.Instance`
- `s.npcMgr.InstancesInRoom(roomID string) []*npc.Instance`
- `s.npcMgr.Get(id string) (*npc.Instance, bool)`
- `s.condRegistry.Get(condID string) (*condition.Definition, bool)`
- `sess.Conditions.Has(condID string) bool`
- `sess.Conditions.Remove(uid, condID string)`
- `sess.Conditions.Apply(uid string, def *condition.Definition, stacks, duration int) error`
- `s.combatH.SpendAP(uid string, cost int) error`
- `s.combatH.ApplyCombatantACMod(uid, targetID string, mod int) error`

---

## Task 1: New Condition YAML Files

**Files to create:**
- `content/conditions/grabbed.yaml`
- `content/conditions/hidden.yaml`

### grabbed.yaml
```yaml
id: grabbed
name: Grabbed
description: |
  You are held in place. You are flat-footed (-2 AC) while grabbed.
  Immobilized effect is pending implementation.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

### hidden.yaml
```yaml
id: hidden
name: Hidden
description: |
  You are concealed. Attackers must succeed on a DC 11 flat check to hit you.
  Being targeted by an attack breaks concealment.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

### Tests

Write tests in `internal/game/condition/condition_test.go` (or the existing test file for the condition package — check what exists with `ls internal/game/condition/`):

```go
func TestGrabbedConditionLoads(t *testing.T) {
    reg, err := condition.LoadRegistry("../../../content/conditions")
    require.NoError(t, err)
    def, ok := reg.Get("grabbed")
    require.True(t, ok, "grabbed condition must be in registry")
    assert.Equal(t, "grabbed", def.ID)
    assert.Equal(t, "Grabbed", def.Name)
    assert.Equal(t, 2, def.ACPenalty)
    assert.Equal(t, 0, def.AttackPenalty)
    assert.Equal(t, "rounds", def.DurationType)
}

func TestHiddenConditionLoads(t *testing.T) {
    reg, err := condition.LoadRegistry("../../../content/conditions")
    require.NoError(t, err)
    def, ok := reg.Get("hidden")
    require.True(t, ok, "hidden condition must be in registry")
    assert.Equal(t, "hidden", def.ID)
    assert.Equal(t, "Hidden", def.Name)
    assert.Equal(t, 0, def.ACPenalty)
    assert.Equal(t, 0, def.AttackPenalty)
    assert.Equal(t, "encounter", def.DurationType)
}
```

**Important:** Check the exact field names on the condition Definition struct before writing tests (read `internal/game/condition/condition.go` or similar).

Run: `go test ./internal/game/condition/... -v -run "TestGrabbedCondition|TestHiddenCondition"`

Commit: `git commit -m "feat: add grabbed and hidden condition YAML files"`

---

## Task 2: Infrastructure — Combatant.Hidden, PlayerSession.GrabberID, CombatHandler Methods

### Step 2a: Add Combatant.Hidden to `internal/game/combat/combat.go`

After the `AttackMod int` field (currently the last field, line ~100), add:

```go
// Hidden is true when this combatant is concealed. Attackers must pass a DC 11 flat check.
// For player combatants: set by hide/divert actions; cleared when the player attacks or is targeted.
// For NPC combatants: unused (always false).
Hidden bool
```

### Step 2b: Add GrabberID to `internal/game/session/manager.go`

After the `Weaknesses` field (currently the last field, line ~88), add:

```go
// GrabberID is the NPC instance ID of the NPC currently grappling this player.
// Empty string when the player is not grabbed. Set by grapple; cleared by escape.
GrabberID string
```

### Step 2c: Add ApplyCombatCondition to `internal/gameserver/combat_handler.go`

Add after `ApplyCombatantAttackMod` (line ~365):

```go
// ApplyCombatCondition applies condID (stacks=1, duration=-1) to the combatant identified by
// targetID in the active combat for the room where uid is fighting.
//
// Precondition: uid must be in active combat; targetID must be a valid combatant ID in that combat;
// condID must be registered in the combat's condition registry.
// Postcondition: The condition is active on the target combatant; returns nil on success.
func (h *CombatHandler) ApplyCombatCondition(uid, targetID, condID string) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("no active combat in room %q", sess.RoomID)
	}
	return cbt.ApplyCondition(targetID, condID, 1, -1)
}

// SetCombatantHidden sets the Hidden field on the combatant identified by uid
// in the active combat for that player's room.
//
// Precondition: uid must be in active combat.
// Postcondition: The combatant's Hidden field equals hidden; returns nil on success.
func (h *CombatHandler) SetCombatantHidden(uid string, hidden bool) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("no active combat in room %q", sess.RoomID)
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			c.Hidden = hidden
			return nil
		}
	}
	return fmt.Errorf("combatant %q not found in combat", uid)
}
```

### Step 2d: Tests (write BEFORE implementing)

Add tests to `internal/gameserver/combat_handler_test.go` (or create `internal/gameserver/combat_handler_condition_test.go`):

Look at how existing combat handler tests set up their fixtures — check `combat_handler_htn_test.go` and `combat_handler_respawn_test.go` to understand the test helper pattern (how they create a `CombatHandler`, `npcMgr`, `sessionMgr`, `engine`).

```go
// TestCombatHandler_ApplyCombatCondition_NotInCombat verifies that ApplyCombatCondition
// returns an error when the player is not in active combat.
func TestCombatHandler_ApplyCombatCondition_NotInCombat(t *testing.T) {
    // Set up handler with a player session but no active combat.
    // Call ApplyCombatCondition(uid, "npc1", "grabbed").
    // Assert: error is non-nil.
}

// TestCombatHandler_ApplyCombatCondition_UnknownCondition verifies that ApplyCombatCondition
// returns an error when condID is not in the condition registry.
func TestCombatHandler_ApplyCombatCondition_UnknownCondition(t *testing.T) {
    // Set up handler with active combat.
    // Call ApplyCombatCondition(uid, npcID, "nonexistent_condition_id").
    // Assert: error is non-nil (propagated from cbt.ApplyCondition).
}

// TestCombatHandler_ApplyCombatCondition_Success verifies that ApplyCombatCondition
// applies the condition to the target combatant in the active combat.
func TestCombatHandler_ApplyCombatCondition_Success(t *testing.T) {
    // Set up handler with active combat; grabbed must be in the condition registry.
    // Call ApplyCombatCondition(uid, npcID, "grabbed").
    // Assert: error is nil; cbt.Conditions[npcID].Has("grabbed") == true.
}

// TestCombatHandler_SetCombatantHidden_NotInCombat verifies error when player not in combat.
func TestCombatHandler_SetCombatantHidden_NotInCombat(t *testing.T) {
    // Set up handler with no active combat.
    // Call SetCombatantHidden(uid, true).
    // Assert: error is non-nil.
}

// TestCombatHandler_SetCombatantHidden_Success verifies Hidden field is set on combatant.
func TestCombatHandler_SetCombatantHidden_Success(t *testing.T) {
    // Set up handler with active combat where uid is a player combatant.
    // Call SetCombatantHidden(uid, true).
    // Assert: error is nil; the combatant with ID == uid has Hidden == true.
}
```

Run: `go test ./internal/gameserver/... -run "TestCombatHandler_ApplyCombatCondition|TestCombatHandler_SetCombatantHidden" -v`

Run full suite: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add Combatant.Hidden, PlayerSession.GrabberID, ApplyCombatCondition, SetCombatantHidden"`

---

## Task 3: Extend Sneak Attack (sucker_punch) in applyPassiveFeats

**File:** `internal/game/combat/round.go`

### Current code (line ~227):
```go
spMet := cbt.Conditions[target.ID] != nil && cbt.Conditions[target.ID].Has("flat_footed") && dmg > 0
if spMet {
    spBonus = src.Intn(6) + 1
}
bonus += hookPassiveFeatCheck(cbt, actor.ID, target.ID, "sucker_punch", spBonus, spMet)
```

### Replace with:
```go
targetOffGuard := cbt.Conditions[target.ID] != nil &&
    (cbt.Conditions[target.ID].Has("flat_footed") || cbt.Conditions[target.ID].Has("grabbed"))
actorHidden := actor.Hidden
spMet := (targetOffGuard || actorHidden) && dmg > 0
if spMet {
    spBonus = src.Intn(6) + 1
}
bonus += hookPassiveFeatCheck(cbt, actor.ID, target.ID, "sucker_punch", spBonus, spMet)
// Attacking from hidden always breaks concealment.
if actorHidden {
    actor.Hidden = false
}
```

The `actor.Hidden = false` line MUST be placed after the `hookPassiveFeatCheck` call but before the function returns. This ensures sneak attack triggers on the concealed attack, then concealment is lost.

### Tests (write BEFORE modifying round.go)

Add to `internal/game/combat/round_passive_hook_test.go` or create `internal/game/combat/round_sneak_hidden_test.go`:

```go
// TestApplyPassiveFeats_SuckerPunch_OnGrabbed verifies sneak attack triggers when target has grabbed.
func TestApplyPassiveFeats_SuckerPunch_OnGrabbed(t *testing.T) {
    // Set up combat with player actor (has sucker_punch passive feat) and NPC target.
    // Apply "grabbed" condition to target via cbt.ApplyCondition(target.ID, "grabbed", 1, -1).
    // Call applyPassiveFeats(cbt, actor, target, 5, fixedSrc).
    // Assert: returned bonus > 0 (1d6 sneak attack).
}

// TestApplyPassiveFeats_SuckerPunch_OnHidden verifies sneak attack triggers when actor.Hidden=true.
func TestApplyPassiveFeats_SuckerPunch_OnHidden(t *testing.T) {
    // Set up combat with player actor (sucker_punch) and NPC target (no conditions).
    // Set actor.Hidden = true.
    // Call applyPassiveFeats(cbt, actor, target, 5, fixedSrc).
    // Assert: returned bonus > 0; actor.Hidden == false after call.
}

// TestApplyPassiveFeats_SuckerPunch_NotTriggeredIfNoCondition verifies no bonus without trigger.
func TestApplyPassiveFeats_SuckerPunch_NotTriggeredIfNoCondition(t *testing.T) {
    // Set up combat with player actor (sucker_punch) and NPC target (no conditions, not hidden).
    // Call applyPassiveFeats(cbt, actor, target, 5, fixedSrc).
    // Assert: returned bonus == 0.
}

// TestApplyPassiveFeats_SuckerPunch_HiddenClearedEvenOnMiss verifies Hidden cleared when dmg==0.
func TestApplyPassiveFeats_SuckerPunch_HiddenClearedEvenOnMiss(t *testing.T) {
    // Set actor.Hidden = true, pass dmg=0.
    // Assert: bonus == 0 (miss); actor.Hidden == false after call.
}

// TestPropertyApplyPassiveFeats_SuckerPunch_NeverNegative is a property test.
func TestPropertyApplyPassiveFeats_SuckerPunch_NeverNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        dmg := rapid.IntRange(0, 100).Draw(rt, "dmg")
        hidden := rapid.Bool().Draw(rt, "hidden")
        // Set up combat, optionally set Hidden, call applyPassiveFeats.
        // Assert bonus >= 0.
    })
}
```

Run: `go test ./internal/game/combat/... -run "TestApplyPassiveFeats_SuckerPunch|TestPropertyApplyPassiveFeats_SuckerPunch" -v`

Run full suite: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: extend sucker_punch sneak attack to trigger on grabbed target or hidden attacker"`

---

## Task 4: Hidden Flat Check for NPC Attacks Against Hidden Player

**File:** `internal/game/combat/round.go`

### Where to insert

In `ResolveRound`, inside the `ActionAttack` case, just before `atkBonus := condition.AttackBonus(...)` (or equivalent — find where the NPC attack roll is computed). Add:

```go
// Hidden flat check: if the target player is hidden, the NPC must succeed on a DC 11 flat check.
if actor.Kind == KindNPC && target != nil && target.Kind == KindPlayer && target.Hidden {
    target.Hidden = false // being targeted always breaks concealment
    flatRoll := src.Intn(20) + 1
    if flatRoll <= 10 {
        events = append(events, RoundEvent{
            ActionType: ActionAttack,
            ActorID:    actor.ID,
            ActorName:  actor.Name,
            Narrative:  fmt.Sprintf("%s attacks %s but fails to locate them (flat check %d)!", actor.Name, target.Name, flatRoll),
        })
        continue // skip the rest of the attack
    }
    // Flat check passed — attack proceeds normally against now-revealed target.
}
```

Apply the same check to `ActionStrike` for the first strike. If the flat check fails, skip BOTH strikes and emit a single flat-check-fail narrative event.

**Important:** Read the existing ResolveRound code carefully before adding the flat check. Find the exact location of the NPC attack resolution for both ActionAttack and ActionStrike cases. The `continue` keyword works if the attack cases are inside a loop over actions; if not, use `goto` or refactor the case body into a helper.

### Tests (write BEFORE modifying round.go)

Add to `internal/game/combat/round_test.go` or create `internal/game/combat/round_hidden_test.go`:

```go
// TestResolveRound_HiddenFlatCheckFail_MissesAttack verifies NPC misses when flat check ≤ 10.
func TestResolveRound_HiddenFlatCheckFail_MissesAttack(t *testing.T) {
    // Set up combat: NPC actor, player target with target.Hidden = true.
    // Use a fixed source that returns 10 or less for the flat check roll.
    // Call ResolveRound.
    // Assert: no damage events; narrative contains "flat check"; target.Hidden == false.
}

// TestResolveRound_HiddenFlatCheckPass_HitsNormally verifies normal attack when flat check > 10.
func TestResolveRound_HiddenFlatCheckPass_HitsNormally(t *testing.T) {
    // Set up: NPC actor, player target with Hidden = true.
    // Use a fixed source that returns > 10 for flat check and a high value for the attack roll.
    // Call ResolveRound.
    // Assert: damage event present; target.Hidden == false after round.
}

// TestResolveRound_HiddenClearedAfterNPCAttack verifies Hidden is always false after NPC targets player.
func TestResolveRound_HiddenClearedAfterNPCAttack(t *testing.T) {
    // Set target.Hidden = true; run ResolveRound regardless of flat check result.
    // Assert: target.Hidden == false.
}

// TestPropertyResolveRound_HiddenFlatCheck_NeverDamageOnFail is a property test.
func TestPropertyResolveRound_HiddenFlatCheck_NeverDamageOnFail(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        // For any scenario where flat check roll ≤ 10, damage to the hidden player is 0.
        flatRoll := rapid.IntRange(1, 10).Draw(rt, "flatRoll")
        // Build a source that produces flatRoll then arbitrary values.
        // Assert damage == 0.
    })
}
```

Run: `go test ./internal/game/combat/... -run "TestResolveRound_Hidden|TestPropertyResolveRound_Hidden" -v`

Run full suite: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add hidden flat check for NPC attacks against concealed players"`

---

## Task 5: grapple Command — CMD-1 through CMD-7

**Syntax:** `grapple <target>`. **AP:** 1. **Skill:** athletics. **DC:** inst.Level+10. **On success:** ApplyCombatCondition(grabbed). **Combat only.**

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant (in the `const` block with HandlerFeint, HandlerDemoralize):
```go
HandlerGrapple = "grapple"
```

Add entry to `BuiltinCommands()` after the demoralize entry:
```go
{Name: "grapple", Aliases: []string{"grp"}, Help: "Grapple a target (athletics vs Level+10 DC; success applies grabbed condition, target is -2 AC for encounter). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerGrapple},
```

**Note:** Do NOT use alias `gr` — it is already taken by `throw`. Use `grp`.

### CMD-3: `internal/game/command/grapple.go`

```go
package command

// GrappleRequest is the parsed form of the grapple command.
//
// Precondition: Target may be empty (handler will return an error in that case).
type GrappleRequest struct {
	Target string
}

// HandleGrapple parses the arguments for the "grapple" command.
//
// Precondition: args is the slice of words following "grapple" (may be empty).
// Postcondition: Returns a non-nil *GrappleRequest and nil error always.
func HandleGrapple(args []string) (*GrappleRequest, error) {
	req := &GrappleRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}
```

Tests for grapple.go (`internal/game/command/grapple_test.go`):
```go
func TestHandleGrapple_EmptyArgs(t *testing.T) {
    req, err := HandleGrapple(nil)
    require.NoError(t, err)
    assert.Equal(t, "", req.Target)
}

func TestHandleGrapple_WithTarget(t *testing.T) {
    req, err := HandleGrapple([]string{"bandit"})
    require.NoError(t, err)
    assert.Equal(t, "bandit", req.Target)
}
```

### CMD-4: `api/proto/game/v1/game.proto`

Add message (after DemoralizeRequest message definition):
```proto
message GrappleRequest {
  string target = 1;
}
```

Add to ClientMessage oneof (after `DemoralizeRequest demoralize = 53;`):
```proto
GrappleRequest grapple = 54;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register in `bridgeHandlerMap`:
```go
command.HandlerGrapple: bridgeGrapple,
```

Add function:
```go
// bridgeGrapple builds a GrappleRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a GrappleRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeGrapple(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: grapple <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Grapple{Grapple: &gamev1.GrappleRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}
```

Verify: `go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v`

### CMD-6: `internal/gameserver/grpc_service.go`

Add to dispatch type switch (after the Demoralize case):
```go
case *gamev1.ClientMessage_Grapple:
    return s.handleGrapple(uid, p.Grapple)
```

Add handler function (after handleDemoralize):
```go
// handleGrapple performs an athletics check against the target NPC's Level+10 DC.
// On success, applies the grabbed condition to the target combatant for the encounter.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target has grabbed condition applied (ACMod behavior from condition YAML).
func (s *GameServiceServer) handleGrapple(uid string, req *gamev1.GrappleRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Grapple is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: grapple <target>"), nil
	}

	// Find target NPC in room before spending AP.
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	// Skill check: 1d20 + athletics bonus vs target Level+10.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleGrapple: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Grapple (athletics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your grapple attempt fails."), nil
	}

	// Success: apply grabbed condition to NPC combatant.
	if err := s.combatH.ApplyCombatCondition(uid, inst.ID, "grabbed"); err != nil {
		s.logger.Warn("handleGrapple: ApplyCombatCondition failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is grabbed (flat-footed, -2 AC).", inst.Name())), nil
}
```

### Tests: `internal/gameserver/grpc_service_test.go`

Follow the pattern of existing handleFeint and handleDemoralize tests. Each test creates a `GameServiceServer` test fixture, a player session, and (for success cases) an active combat.

```go
func TestHandleGrapple_NoSession(t *testing.T) {
    // Call handleGrapple with uid that has no session.
    // Assert: returns non-nil error (not a *gamev1.ServerEvent error).
}

func TestHandleGrapple_NotInCombat(t *testing.T) {
    // Player session exists with Status != statusInCombat.
    // Assert: returns errorEvent("Grapple is only available in combat.").
}

func TestHandleGrapple_EmptyTarget(t *testing.T) {
    // Player in combat; req.Target == "".
    // Assert: returns errorEvent("Usage: grapple <target>").
}

func TestHandleGrapple_TargetNotFound(t *testing.T) {
    // Player in combat; NPC not in room.
    // Assert: returns errorEvent with "not found".
}

func TestHandleGrapple_RollBelowDC_Failure(t *testing.T) {
    // Player in combat; NPC in room; rigged dice to roll below DC.
    // Assert: no grabbed condition on NPC; message contains "failure".
}

func TestHandleGrapple_RollAboveDC_Success(t *testing.T) {
    // Player in combat; NPC in room; rigged dice to roll above DC.
    // Assert: grabbed condition applied to NPC combatant; message contains "success".
}
```

Run: `go test ./internal/gameserver/... -run "TestHandleGrapple" -v`

Run full suite: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add grapple command — CMD-1 through CMD-7"`

---

## Task 6: trip Command — CMD-1 through CMD-7

**Syntax:** `trip <target>`. **AP:** 1. **Skill:** athletics. **DC:** inst.Level+10. **On success:** ApplyCombatCondition(prone). **Combat only.**

Prone condition already exists at `content/conditions/prone.yaml` (duration_type: permanent, attack_penalty: 2).

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant:
```go
HandlerTrip = "trip"
```

Add to `BuiltinCommands()` after grapple:
```go
{Name: "trip", Aliases: []string{"trp"}, Help: "Trip a target (athletics vs Level+10 DC; success applies prone, -2 attack for encounter). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerTrip},
```

### CMD-3: `internal/game/command/trip.go`

```go
package command

// TripRequest is the parsed form of the trip command.
//
// Precondition: Target may be empty (handler will return an error in that case).
type TripRequest struct {
	Target string
}

// HandleTrip parses the arguments for the "trip" command.
//
// Precondition: args is the slice of words following "trip" (may be empty).
// Postcondition: Returns a non-nil *TripRequest and nil error always.
func HandleTrip(args []string) (*TripRequest, error) {
	req := &TripRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}
```

Tests (`internal/game/command/trip_test.go`): same pattern as grapple_test.go.

### CMD-4: `api/proto/game/v1/game.proto`

```proto
message TripRequest {
  string target = 1;
}
```

In ClientMessage oneof:
```proto
TripRequest trip = 55;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register:
```go
command.HandlerTrip: bridgeTrip,
```

```go
// bridgeTrip builds a TripRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a TripRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeTrip(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: trip <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Trip{Trip: &gamev1.TripRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}
```

### CMD-6: `internal/gameserver/grpc_service.go`

Dispatch:
```go
case *gamev1.ClientMessage_Trip:
    return s.handleTrip(uid, p.Trip)
```

Handler (identical to handleGrapple but uses `"prone"` condition and different messaging):
```go
// handleTrip performs an athletics check against the target NPC's Level+10 DC.
// On success, applies the prone condition to the target combatant.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target has prone condition applied (-2 attack rolls).
func (s *GameServiceServer) handleTrip(uid string, req *gamev1.TripRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Trip is only available in combat."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: trip <target>"), nil
	}

	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleTrip: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus
	dc := inst.Level + 10

	detail := fmt.Sprintf("Trip (athletics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your trip attempt fails."), nil
	}

	if err := s.combatH.ApplyCombatCondition(uid, inst.ID, "prone"); err != nil {
		s.logger.Warn("handleTrip: ApplyCombatCondition failed",
			zap.String("npc_id", inst.ID), zap.Error(err))
	}

	return messageEvent(detail + fmt.Sprintf(" — success! %s is knocked prone (-2 attack rolls).", inst.Name())), nil
}
```

Tests (`grpc_service_test.go`): same 6-test pattern as grapple, substituting trip.

Run: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add trip command — CMD-1 through CMD-7"`

---

## Task 7: hide Command — CMD-1 through CMD-7

**Syntax:** `hide`. **AP:** 1. **Skill:** stealth. **DC:** highest NPC Perception in room (min 10). **On success:** apply hidden condition to player session + SetCombatantHidden(uid, true). **Combat only.**

### Helper function to add in `internal/gameserver/grpc_service.go`

Add near the other helper functions (e.g., after the skillRankBonus usage section):

```go
// maxNPCPerceptionInRoom returns the highest Perception value among living NPCs in roomID.
// Returns 10 if no living NPCs are present (minimum DC).
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns the max NPC Perception in the room, minimum 10.
func (s *GameServiceServer) maxNPCPerceptionInRoom(roomID string) int {
	insts := s.npcMgr.InstancesInRoom(roomID)
	max := 10
	for _, inst := range insts {
		if !inst.IsDead() && inst.Perception > max {
			max = inst.Perception
		}
	}
	return max
}
```

**Note:** `npc.Instance.IsDead()` returns true when `CurrentHP <= 0`. Check the Instance struct in `internal/game/npc/instance.go` to verify the method name.

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant:
```go
HandlerHide = "hide"
```

Add to `BuiltinCommands()`:
```go
{Name: "hide", Aliases: nil, Help: "Attempt to hide (stealth vs highest NPC Perception DC; success applies hidden condition). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerHide},
```

### CMD-3: `internal/game/command/hide.go`

```go
package command

// HideRequest is the parsed form of the hide command.
//
// Precondition: No arguments required.
type HideRequest struct{}

// HandleHide parses the arguments for the "hide" command.
//
// Precondition: args may be empty (no arguments needed).
// Postcondition: Returns a non-nil *HideRequest and nil error always.
func HandleHide(args []string) (*HideRequest, error) {
	return &HideRequest{}, nil
}
```

Tests (`internal/game/command/hide_test.go`):
```go
func TestHandleHide_NoArgs(t *testing.T) {
    req, err := HandleHide(nil)
    require.NoError(t, err)
    assert.NotNil(t, req)
}

func TestHandleHide_ExtraArgsIgnored(t *testing.T) {
    req, err := HandleHide([]string{"extra", "args"})
    require.NoError(t, err)
    assert.NotNil(t, req)
}
```

### CMD-4: `api/proto/game/v1/game.proto`

```proto
message HideRequest {}
```

In ClientMessage oneof:
```proto
HideRequest hide = 56;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register:
```go
command.HandlerHide: bridgeHide,
```

```go
// bridgeHide builds a HideRequest (no arguments required).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a HideRequest.
func bridgeHide(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Hide{Hide: &gamev1.HideRequest{}},
	}}, nil
}
```

### CMD-6: `internal/gameserver/grpc_service.go`

Dispatch:
```go
case *gamev1.ClientMessage_Hide:
    return s.handleHide(uid)
```

Handler:
```go
// handleHide performs a stealth check against the highest NPC Perception DC in the room.
// On success, applies the hidden condition to the player's session and sets combatant Hidden=true.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat.
// Postcondition: On success, sess.Conditions has "hidden"; combatant.Hidden == true.
func (s *GameServiceServer) handleHide(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Hide is only available in combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleHide: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["stealth"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)

	detail := fmt.Sprintf("Hide (stealth DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. You fail to hide."), nil
	}

	// Apply hidden condition to player session.
	if s.condRegistry != nil {
		if def, ok := s.condRegistry.Get("hidden"); ok {
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleHide: condition apply failed", zap.Error(err))
			}
		} else {
			s.logger.Warn("handleHide: hidden condition not in registry")
		}
	}

	// Mark combatant as hidden in the combat engine.
	if err := s.combatH.SetCombatantHidden(uid, true); err != nil {
		s.logger.Warn("handleHide: SetCombatantHidden failed", zap.Error(err))
	}

	return messageEvent(detail + " — success! You slip into the shadows."), nil
}
```

Tests (`grpc_service_test.go`):
```go
func TestHandleHide_NoSession(t *testing.T) { /* uid has no session; assert non-nil error */ }
func TestHandleHide_NotInCombat(t *testing.T) { /* Status != statusInCombat; assert errorEvent */ }
func TestHandleHide_SpendAPFail(t *testing.T) { /* insufficient AP; assert errorEvent */ }
func TestHandleHide_RollBelow_Failure(t *testing.T) { /* roll < DC; assert failure message; hidden not applied */ }
func TestHandleHide_RollAbove_Success(t *testing.T) {
    // roll >= DC; assert:
    // - success message
    // - sess.Conditions.Has("hidden") == true
    // - combatant.Hidden == true (retrieve via h.combatH.GetCombatant(uid, uid))
}
```

Run: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add hide command — CMD-1 through CMD-7"`

---

## Task 8: sneak Command — CMD-1 through CMD-7

**Syntax:** `sneak`. **AP:** 1. **Skill:** stealth. **DC:** highest NPC Perception in room. **Requires:** player has hidden condition. **On success:** maintain hidden. **On fail:** remove hidden condition + SetCombatantHidden(uid, false). **Combat only.**

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant:
```go
HandlerSneak = "sneak"
```

Add to `BuiltinCommands()`:
```go
{Name: "sneak", Aliases: nil, Help: "Move while hidden (stealth vs highest NPC Perception DC; fail removes hidden). Requires hidden condition. Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerSneak},
```

### CMD-3: `internal/game/command/sneak.go`

```go
package command

// SneakRequest is the parsed form of the sneak command.
//
// Precondition: No arguments required.
type SneakRequest struct{}

// HandleSneak parses the arguments for the "sneak" command.
//
// Precondition: args may be empty (no arguments needed).
// Postcondition: Returns a non-nil *SneakRequest and nil error always.
func HandleSneak(args []string) (*SneakRequest, error) {
	return &SneakRequest{}, nil
}
```

Tests (`internal/game/command/sneak_test.go`): same as hide_test.go pattern.

### CMD-4: `api/proto/game/v1/game.proto`

```proto
message SneakRequest {}
```

In ClientMessage oneof:
```proto
SneakRequest sneak = 57;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register:
```go
command.HandlerSneak: bridgeSneak,
```

```go
// bridgeSneak builds a SneakRequest (no arguments required).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a SneakRequest.
func bridgeSneak(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Sneak{Sneak: &gamev1.SneakRequest{}},
	}}, nil
}
```

### CMD-6: `internal/gameserver/grpc_service.go`

Dispatch:
```go
case *gamev1.ClientMessage_Sneak:
    return s.handleSneak(uid)
```

Handler:
```go
// handleSneak performs a stealth check against the highest NPC Perception DC in the room.
// Requires the player to currently have the hidden condition.
// On failure, removes hidden condition and clears combatant Hidden flag.
// On success, hidden is maintained.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat and have the hidden condition.
// Postcondition: On success, hidden is maintained. On failure, hidden is removed.
func (s *GameServiceServer) handleSneak(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Sneak is only available in combat."), nil
	}

	if sess.Conditions == nil || !sess.Conditions.Has("hidden") {
		return errorEvent("You must be hidden to sneak. Use 'hide' first."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleSneak: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["stealth"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)

	detail := fmt.Sprintf("Sneak (stealth DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		// Failure: remove hidden.
		if sess.Conditions != nil {
			sess.Conditions.Remove(uid, "hidden")
		}
		if err := s.combatH.SetCombatantHidden(uid, false); err != nil {
			s.logger.Warn("handleSneak: SetCombatantHidden failed", zap.Error(err))
		}
		return messageEvent(detail + " — failure. You are detected and lose hidden."), nil
	}

	return messageEvent(detail + " — success! You move while remaining hidden."), nil
}
```

Tests (`grpc_service_test.go`):
```go
func TestHandleSneak_NoSession(t *testing.T) { /* assert non-nil error */ }
func TestHandleSneak_NotInCombat(t *testing.T) { /* assert errorEvent */ }
func TestHandleSneak_NotHidden(t *testing.T) {
    // Player in combat but does not have hidden condition.
    // Assert: errorEvent("You must be hidden to sneak. Use 'hide' first.")
}
func TestHandleSneak_SpendAPFail(t *testing.T) { /* player hidden; insufficient AP */ }
func TestHandleSneak_RollBelow_RemovesHidden(t *testing.T) {
    // Player hidden; roll < DC.
    // Assert: sess.Conditions.Has("hidden") == false; combatant.Hidden == false; failure message.
}
func TestHandleSneak_RollAbove_MaintainsHidden(t *testing.T) {
    // Player hidden; roll >= DC.
    // Assert: sess.Conditions.Has("hidden") == true; combatant.Hidden == true; success message.
}
```

Run: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add sneak command — CMD-1 through CMD-7"`

---

## Task 9: divert Command (Create Diversion) — CMD-1 through CMD-7

**Syntax:** `divert`. **AP:** 1. **Skill:** grift (deception). **DC:** highest NPC Perception in room. **On success:** apply hidden condition + SetCombatantHidden(uid, true). **Combat only.**

This is functionally identical to `hide` but uses the `grift` skill instead of `stealth`. The hidden effect lasts until the player attacks or is targeted (same as hide).

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant:
```go
HandlerDivert = "divert"
```

Add to `BuiltinCommands()`:
```go
{Name: "divert", Aliases: []string{"div"}, Help: "Create a diversion (grift vs highest NPC Perception DC; success applies hidden condition). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerDivert},
```

**Note:** Do NOT use alias `cd` — it is already taken by `combat_default`. Use `div`.

### CMD-3: `internal/game/command/divert.go`

```go
package command

// DivertRequest is the parsed form of the divert command.
//
// Precondition: No arguments required.
type DivertRequest struct{}

// HandleDivert parses the arguments for the "divert" command.
//
// Precondition: args may be empty (no arguments needed).
// Postcondition: Returns a non-nil *DivertRequest and nil error always.
func HandleDivert(args []string) (*DivertRequest, error) {
	return &DivertRequest{}, nil
}
```

Tests (`internal/game/command/divert_test.go`): same as hide_test.go pattern.

### CMD-4: `api/proto/game/v1/game.proto`

```proto
message DivertRequest {}
```

In ClientMessage oneof:
```proto
DivertRequest divert = 58;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register:
```go
command.HandlerDivert: bridgeDivert,
```

```go
// bridgeDivert builds a DivertRequest (no arguments required).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a DivertRequest.
func bridgeDivert(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Divert{Divert: &gamev1.DivertRequest{}},
	}}, nil
}
```

### CMD-6: `internal/gameserver/grpc_service.go`

Dispatch:
```go
case *gamev1.ClientMessage_Divert:
    return s.handleDivert(uid)
```

Handler:
```go
// handleDivert performs a grift skill check against the highest NPC Perception DC in the room.
// On success, applies the hidden condition to the player's session and sets combatant Hidden=true.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat.
// Postcondition: On success, sess.Conditions has "hidden"; combatant.Hidden == true.
func (s *GameServiceServer) handleDivert(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Divert is only available in combat."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleDivert: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["grift"])
	total := roll + bonus
	dc := s.maxNPCPerceptionInRoom(sess.RoomID)

	detail := fmt.Sprintf("Create Diversion (grift DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your diversion falls flat."), nil
	}

	// Apply hidden condition to player session.
	if s.condRegistry != nil {
		if def, ok := s.condRegistry.Get("hidden"); ok {
			if err := sess.Conditions.Apply(uid, def, 1, -1); err != nil {
				s.logger.Warn("handleDivert: condition apply failed", zap.Error(err))
			}
		} else {
			s.logger.Warn("handleDivert: hidden condition not in registry")
		}
	}

	if err := s.combatH.SetCombatantHidden(uid, true); err != nil {
		s.logger.Warn("handleDivert: SetCombatantHidden failed", zap.Error(err))
	}

	return messageEvent(detail + " — success! You create a diversion and slip into the shadows."), nil
}
```

Tests (`grpc_service_test.go`): same 5-test pattern as handleHide tests.

Run: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add divert command (Create Diversion) — CMD-1 through CMD-7"`

---

## Task 10: escape Command — CMD-1 through CMD-7

**Syntax:** `escape`. **AP:** 1. **Skill:** max(athletics, acrobatics). **Requires:** player has grabbed condition. **DC:** grabber's Level+14 (if GrabberID set and NPC alive in room), else 15. **On success:** remove grabbed from self; clear GrabberID. **Combat only.**

### CMD-1 and CMD-2: `internal/game/command/commands.go`

Add constant:
```go
HandlerEscape = "escape"
```

Add to `BuiltinCommands()`:
```go
{Name: "escape", Aliases: []string{"esc"}, Help: "Escape from grabbed condition (max athletics/acrobatics vs DC; success removes grabbed). Requires grabbed. Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerEscape},
```

### CMD-3: `internal/game/command/escape.go`

```go
package command

// EscapeRequest is the parsed form of the escape command.
//
// Precondition: No arguments required.
type EscapeRequest struct{}

// HandleEscape parses the arguments for the "escape" command.
//
// Precondition: args may be empty (no arguments needed).
// Postcondition: Returns a non-nil *EscapeRequest and nil error always.
func HandleEscape(args []string) (*EscapeRequest, error) {
	return &EscapeRequest{}, nil
}
```

Tests (`internal/game/command/escape_test.go`): same pattern as hide_test.go.

### CMD-4: `api/proto/game/v1/game.proto`

```proto
message EscapeRequest {}
```

In ClientMessage oneof:
```proto
EscapeRequest escape = 59;
```

Run: `make proto`

### CMD-5: `internal/frontend/handlers/bridge_handlers.go`

Register:
```go
command.HandlerEscape: bridgeEscape,
```

```go
// bridgeEscape builds an EscapeRequest (no arguments required).
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an EscapeRequest.
func bridgeEscape(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Escape{Escape: &gamev1.EscapeRequest{}},
	}}, nil
}
```

### CMD-6: `internal/gameserver/grpc_service.go`

Dispatch:
```go
case *gamev1.ClientMessage_Escape:
    return s.handleEscape(uid)
```

Handler:
```go
// handleEscape performs a max(athletics, acrobatics) check against the grabber's DC.
// Requires the player to currently have the grabbed condition.
// On success, removes the grabbed condition and clears GrabberID.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat and have the grabbed condition.
// Postcondition: On success, grabbed condition removed and GrabberID cleared.
func (s *GameServiceServer) handleEscape(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if sess.Status != statusInCombat {
		return errorEvent("Escape is only available in combat."), nil
	}

	if sess.Conditions == nil || !sess.Conditions.Has("grabbed") {
		return errorEvent("You are not grabbed."), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleEscape: rolling d20: %w", err)
	}
	roll := rollResult.Total()

	// Use the better of athletics or acrobatics.
	athBonus := skillRankBonus(sess.Skills["athletics"])
	acrBonus := skillRankBonus(sess.Skills["acrobatics"])
	bonus := athBonus
	if acrBonus > bonus {
		bonus = acrBonus
	}
	total := roll + bonus

	// DC: use grabber's Level+14 if GrabberID is set and the grabber is alive in this room.
	dc := 15
	if sess.GrabberID != "" {
		if grabber, found := s.npcMgr.Get(sess.GrabberID); found && !grabber.IsDead() && grabber.RoomID == sess.RoomID {
			dc = grabber.Level + 14
		}
	}

	detail := fmt.Sprintf("Escape (athletics/acrobatics DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + " — failure. You remain grabbed."), nil
	}

	// Success: remove grabbed condition and clear the grabber reference.
	if sess.Conditions != nil {
		sess.Conditions.Remove(uid, "grabbed")
	}
	sess.GrabberID = ""

	return messageEvent(detail + " — success! You break free from the grapple."), nil
}
```

**Note:** `grabber.RoomID` — check that `npc.Instance` has a `RoomID` field. Look at `internal/game/npc/instance.go` before implementing. If the field name differs, use it correctly.

Tests (`grpc_service_test.go`):
```go
func TestHandleEscape_NoSession(t *testing.T) { /* assert non-nil error */ }
func TestHandleEscape_NotInCombat(t *testing.T) { /* assert errorEvent */ }
func TestHandleEscape_NotGrabbed(t *testing.T) {
    // Player in combat but does not have grabbed condition.
    // Assert: errorEvent("You are not grabbed.")
}
func TestHandleEscape_SpendAPFail(t *testing.T) { /* player grabbed; insufficient AP */ }
func TestHandleEscape_RollBelow_StillGrabbed(t *testing.T) {
    // Roll < DC; assert: grabbed condition still present; failure message.
}
func TestHandleEscape_RollAbove_Success(t *testing.T) {
    // Roll >= DC; assert: grabbed condition removed; GrabberID == ""; success message.
}
```

Run: `go test ./... -count=1 2>&1 | tail -10`

Commit: `git commit -m "feat: add escape command — CMD-1 through CMD-7"`

---

## Task 11: FEATURES.md Updates

**File:** `docs/requirements/FEATURES.md`

Find the section for PF2E combat actions or equivalent. Mark the six new actions as complete (`[x]`) and add an Immobilized stub.

Locate where grapple, trip, hide, sneak, create diversion, and escape appear in the features file. If they are not listed, add them under the appropriate section (likely "Advanced combat mechanics" or "Combat actions").

Mark complete:
```
- [x] Grapple — Athletics vs Level+10 DC; success applies grabbed condition (-2 AC, flat-footed)
- [x] Trip — Athletics vs Level+10 DC; success applies prone condition (-2 attack rolls)
- [x] Hide — Stealth vs highest NPC Perception DC; success applies hidden condition
- [x] Sneak — Stealth vs highest NPC Perception DC while hidden; failure removes hidden
- [x] Create Diversion (divert) — Grift vs highest NPC Perception DC; success applies hidden condition
- [x] Escape — Max(athletics, acrobatics) vs grabber DC; success removes grabbed condition
```

Add stub for future work:
```
- [ ] Immobilized — prevent grabbed creatures from moving between rooms
```

Commit: `git commit -m "docs: mark condition-applying actions complete in FEATURES.md; add Immobilized stub"`

---

## Alias Conflict Reference

This table lists all alias conflicts to avoid:

| Alias | Already Used By | Consequence |
|-------|----------------|-------------|
| `gr`  | `throw`        | Do not use for grapple; use `grp` |
| `cd`  | `combat_default` | Do not use for divert; use `div` |
| `tr`  | none currently | Could use for trip, but `trp` is safer |

---

## Test Execution Order

After each task, run the full suite:
```
go test ./... -count=1 2>&1 | tail -10
```

For targeted runs during development:
```bash
# Task 1
go test ./internal/game/condition/... -v -run "TestGrabbedCondition|TestHiddenCondition"

# Task 2
go test ./internal/gameserver/... -run "TestCombatHandler_ApplyCombatCondition|TestCombatHandler_SetCombatantHidden" -v

# Task 3
go test ./internal/game/combat/... -run "TestApplyPassiveFeats_SuckerPunch|TestPropertyApplyPassiveFeats" -v

# Task 4
go test ./internal/game/combat/... -run "TestResolveRound_Hidden|TestPropertyResolveRound_Hidden" -v

# Task 5
go test ./internal/gameserver/... -run "TestHandleGrapple" -v
go test ./internal/frontend/handlers/... -run "TestAllCommandHandlersAreWired" -v

# Task 6
go test ./internal/gameserver/... -run "TestHandleTrip" -v

# Task 7
go test ./internal/gameserver/... -run "TestHandleHide" -v

# Task 8
go test ./internal/gameserver/... -run "TestHandleSneak" -v

# Task 9
go test ./internal/gameserver/... -run "TestHandleDivert" -v

# Task 10
go test ./internal/gameserver/... -run "TestHandleEscape" -v
```

---

## Files Modified Summary

| Task | Files Modified/Created |
|------|----------------------|
| 1 | `content/conditions/grabbed.yaml` (new), `content/conditions/hidden.yaml` (new) |
| 2 | `internal/game/combat/combat.go`, `internal/game/session/manager.go`, `internal/gameserver/combat_handler.go`, `internal/gameserver/combat_handler_condition_test.go` (new) |
| 3 | `internal/game/combat/round.go`, `internal/game/combat/round_sneak_hidden_test.go` (new) |
| 4 | `internal/game/combat/round.go`, `internal/game/combat/round_hidden_test.go` (new) |
| 5 | `commands.go`, `grapple.go` (new), `grapple_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 6 | `commands.go`, `trip.go` (new), `trip_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 7 | `commands.go`, `hide.go` (new), `hide_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 8 | `commands.go`, `sneak.go` (new), `sneak_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 9 | `commands.go`, `divert.go` (new), `divert_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 10 | `commands.go`, `escape.go` (new), `escape_test.go` (new), `game.proto`, `bridge_handlers.go`, `grpc_service.go`, `grpc_service_test.go` |
| 11 | `docs/requirements/FEATURES.md` |

# PF2E Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Implement five PF2E actions (raise_shield, take_cover, first_aid, feint, demoralize) as standalone commands and stub all remaining PF2E actions in FEATURES.md.

**Architecture:** Eight tasks. Task 1 creates four condition YAML files. Task 2 adds `ACMod`/`AttackMod` fields to `Combatant`, `DeductAP` to `ActionQueue`, and helper methods to `CombatHandler` so combat-aware handlers can spend AP and apply mid-round AC modifiers. Tasks 3–7 each implement one command via the full CMD-1 through CMD-7 pipeline. Task 8 updates FEATURES.md with stubs for all remaining PF2E actions.

**Tech Stack:** Go, YAML, protobuf; no new dependencies.

---

## Background: CMD-1 through CMD-7 Pipeline

Every new command requires ALL of the following:
- **CMD-1**: Add `HandlerXxx = "xxx"` constant to `internal/game/command/commands.go`
- **CMD-2**: Append `Command{...}` entry to `BuiltinCommands()` in the same file
- **CMD-3**: Implement `HandleXxx` in `internal/game/command/xxx.go` with full TDD coverage
- **CMD-4**: Add proto message to `api/proto/game/v1/game.proto` and run `make proto`
- **CMD-5**: Add `bridgeXxx` function and map entry to `internal/frontend/handlers/bridge_handlers.go`; `TestAllCommandHandlersAreWired` must pass
- **CMD-6**: Implement `handleXxx` in `internal/gameserver/grpc_service.go` and wire into `dispatch`
- **CMD-7**: All tests pass

## Background: Key File Locations

- `internal/game/command/commands.go` — constants and BuiltinCommands
- `api/proto/game/v1/game.proto` — proto oneof (last field number is `action = 48`)
- `internal/frontend/handlers/bridge_handlers.go` — bridgeHandlerMap
- `internal/gameserver/grpc_service.go` — dispatch switch and handler functions
- `internal/game/combat/combat.go` — `Combatant` struct
- `internal/game/combat/resolver.go` — `ResolveAttack` function
- `internal/game/combat/action.go` — `ActionQueue` struct
- `internal/gameserver/combat_handler.go` — `CombatHandler` methods
- `content/conditions/` — condition YAML files

## Background: Condition YAML Format

```yaml
id: <id>
name: <Name>
description: |
  <description text>
duration_type: round   # or: encounter, permanent, rounds
max_stacks: 0          # 0 = no stacking
attack_penalty: 0      # positive = reduces attack by this value per stack
ac_penalty: 0          # positive = reduces AC by this value per stack
damage_bonus: 0        # positive = bonus to damage
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

## Background: Session / Shield Check

To check if the player has a shield equipped in combat:

```go
sess.LoadoutSet.ActivePreset()  // returns *inventory.WeaponPreset or nil
preset.OffHand                  // *EquippedWeapon or nil
preset.OffHand.Def.IsShield()   // bool
```

`inventory.LoadoutSet` is at `sess.LoadoutSet`. `inventory.WeaponPreset.OffHand` is the off-hand slot. `WeaponDef.IsShield()` returns `w.Kind == WeaponKindShield`.

## Background: Applying Self-Conditions

```go
def, ok := s.condRegistry.Get("shield_raised")
if !ok { /* error */ }
if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil { /* error */ }
```

## Background: Session Status Check (In Combat)

```go
const statusInCombat = int32(2)  // already defined in action_handler.go
sess.Status == statusInCombat    // true when player is in combat
```

## Background: Skill Bonus Calculation

```go
rank := sess.Skills[skillID]      // e.g. sess.Skills["grift"]
bonus := skillRankBonus(rank)     // already defined in action_handler.go: trained=2, expert=4, master=6, legendary=8
```

## Background: NPC Perception for Feint

The NPC's `Perception` field is on `npc.Instance`. Use `npcMgr.FindInRoom(sess.RoomID, target)` to get `*npc.Instance`. The DC for the feint check is `inst.Perception`.

---

## Task 1: Condition YAML files

**Files:**
- Create: `content/conditions/shield_raised.yaml`
- Create: `content/conditions/in_cover.yaml`
- Create: `content/conditions/flat_footed.yaml`
- Create: `content/conditions/frightened.yaml`
- Modify: `internal/game/condition/loader_test.go` (or wherever the condition count test lives; search for `LoadDirectory`)

### Step 1: Find the condition loader test

```bash
grep -rn "LoadDirectory\|condition.*count\|len.*conditions" internal/game/condition/ --include="*.go"
```

There may be a test that checks the total number of loaded conditions. If so, update the expected count to include the 4 new conditions.

### Step 2: Create the four YAML files

**`content/conditions/shield_raised.yaml`:**
```yaml
id: shield_raised
name: Shield Raised
description: |
  You have raised your shield, gaining +2 AC until the start of your next turn.
duration_type: round
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

**`content/conditions/in_cover.yaml`:**
```yaml
id: in_cover
name: In Cover
description: |
  You are taking cover, gaining +2 AC until you move or the encounter ends.
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

**`content/conditions/flat_footed.yaml`:**
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

**`content/conditions/frightened.yaml`:**
```yaml
id: frightened
name: Frightened
description: |
  You are shaken by fear, suffering -1 to attack rolls and AC per stack.
duration_type: encounter
max_stacks: 3
attack_penalty: 1
ac_penalty: 1
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

### Step 3: Run the condition tests

```bash
go test ./internal/game/condition/... -v 2>&1 | tail -20
```

Expected: all PASS. Fix any count-based test by incrementing the expected count by 4.

### Step 4: Run full suite

```bash
go test ./... -count=1 2>&1 | tail -10
```

Expected: all PASS.

### Step 5: Commit

```bash
git add content/conditions/shield_raised.yaml content/conditions/in_cover.yaml \
        content/conditions/flat_footed.yaml content/conditions/frightened.yaml
git add internal/game/condition/
git commit -m "feat: add shield_raised, in_cover, flat_footed, frightened conditions"
```

---

## Task 2: Combat infrastructure — ACMod/AttackMod and SpendAP

**Files:**
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/game/combat/resolver.go`
- Modify: `internal/game/combat/action.go`
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/game/combat/resolver_test.go`
- Test: `internal/game/combat/action_test.go`
- Test: `internal/gameserver/combat_handler_test.go` (if it exists; otherwise skip)

### Step 1: Write failing tests

In `internal/game/combat/resolver_test.go`, add:

```go
func TestResolveAttack_ACMod_ReducesEffectiveAC(t *testing.T) {
    // A large attack total that hits AC 15 should miss AC 15 when ACMod=+10
    src := &deterministicSource{val: 14} // d20=15
    attacker := &Combatant{ID: "a", Level: 1, StrMod: 0, AC: 10, CurrentHP: 10, MaxHP: 10}
    target := &Combatant{ID: "b", Level: 1, AC: 15, ACMod: 10, CurrentHP: 10, MaxHP: 10}
    result := ResolveAttack(attacker, target, src)
    // total = 15, effectiveAC = 25 → miss
    assert.Equal(t, Failure, result.Outcome, "ACMod should raise effective AC and cause a miss")
}

func TestResolveAttack_AttackMod_ReducesAttackTotal(t *testing.T) {
    src := &deterministicSource{val: 19} // d20=20, normally crit vs AC10
    attacker := &Combatant{ID: "a", Level: 1, StrMod: 0, AttackMod: -5, AC: 10, CurrentHP: 10, MaxHP: 10}
    target := &Combatant{ID: "b", Level: 1, AC: 10, CurrentHP: 10, MaxHP: 10}
    result := ResolveAttack(attacker, target, src)
    // total = 20 - 5 = 15 vs AC 10 → success (not crit, since 15 < 20)
    assert.Equal(t, Success, result.Outcome)
}
```

Note: check whether `deterministicSource` already exists in the resolver tests. If not, add:

```go
type deterministicSource struct{ val int }
func (s *deterministicSource) Intn(n int) int { return s.val % n }
```

In `internal/game/combat/action_test.go`, add:

```go
func TestActionQueue_DeductAP_Success(t *testing.T) {
    q := NewActionQueue("u1", 3)
    err := q.DeductAP(1)
    require.NoError(t, err)
    assert.Equal(t, 2, q.RemainingPoints())
}

func TestActionQueue_DeductAP_InsufficientAP(t *testing.T) {
    q := NewActionQueue("u1", 1)
    err := q.DeductAP(2)
    require.Error(t, err)
    assert.Equal(t, 1, q.RemainingPoints(), "AP must not change on failure")
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/combat/... -run "TestResolveAttack_ACMod|TestResolveAttack_AttackMod|TestActionQueue_DeductAP" -v 2>&1 | tail -20
```

Expected: compile error or FAIL — fields/methods not defined yet.

### Step 3: Add ACMod and AttackMod to Combatant

In `internal/game/combat/combat.go`, add to the `Combatant` struct after the `Weaknesses` field:

```go
// ACMod is a temporary mid-round AC modifier applied by conditions (e.g. flat_footed, shield_raised).
// Negative values reduce effective AC; positive values increase it.
ACMod int
// AttackMod is a temporary mid-round attack roll modifier applied by conditions (e.g. frightened).
// Negative values reduce the attacker's roll total.
AttackMod int
```

### Step 4: Update ResolveAttack in resolver.go

In `internal/game/combat/resolver.go`, find:

```go
outcome := OutcomeFor(atkTotal, target.AC)
```

Replace with:

```go
outcome := OutcomeFor(atkTotal+attacker.AttackMod, target.AC+target.ACMod)
```

**Important:** Also update the `AttackTotal` field in the returned result so it reflects the actual effective roll:

```go
return AttackResult{
    AttackerID:  attacker.ID,
    TargetID:    target.ID,
    AttackRoll:  d20,
    AttackTotal: atkTotal + attacker.AttackMod,
    Outcome:     outcome,
    BaseDamage:  baseDmg,
    DamageRoll:  []int{dmgDie},
    DamageType:  attacker.WeaponDamageType,
}
```

### Step 5: Add DeductAP to ActionQueue

In `internal/game/combat/action.go`, add after `RemainingPoints()`:

```go
// DeductAP reduces remaining action points by cost without queuing an action.
// Use for out-of-queue actions (raise_shield, take_cover, feint, demoralize, first_aid).
//
// Precondition: cost > 0.
// Postcondition: remaining decremented by cost on success; unchanged on error.
func (q *ActionQueue) DeductAP(cost int) error {
    if q.remaining < cost {
        return fmt.Errorf("not enough AP: have %d, need %d", q.remaining, cost)
    }
    q.remaining -= cost
    return nil
}
```

### Step 6: Add SpendAP and ApplyCombatantACMod to CombatHandler

In `internal/gameserver/combat_handler.go`, add after `RemainingAP`:

```go
// SpendAP deducts cost AP from the combatant uid's action queue in their active combat.
//
// Precondition: uid must be non-empty; cost must be > 0.
// Postcondition: Returns nil on success; error if player not in combat or insufficient AP.
func (h *CombatHandler) SpendAP(uid string, cost int) error {
    sess, ok := h.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("player %q not found", uid)
    }

    h.combatMu.Lock()
    defer h.combatMu.Unlock()

    cbt, ok := h.engine.GetCombat(sess.RoomID)
    if !ok {
        return fmt.Errorf("player %q is not in active combat", uid)
    }

    q, ok := cbt.ActionQueues[uid]
    if !ok {
        return fmt.Errorf("no action queue for player %q", uid)
    }
    return q.DeductAP(cost)
}

// ApplyCombatantACMod adds mod to the named combatant's ACMod in the player's active combat.
// Use to apply mid-round AC modifiers from feint (negative) or raise_shield/take_cover (positive).
//
// Precondition: uid must be a player in active combat; targetID must be a combatant in that combat.
// Postcondition: Returns nil on success.
func (h *CombatHandler) ApplyCombatantACMod(uid, targetID string, mod int) error {
    sess, ok := h.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("player %q not found", uid)
    }

    h.combatMu.Lock()
    defer h.combatMu.Unlock()

    cbt, ok := h.engine.GetCombat(sess.RoomID)
    if !ok {
        return fmt.Errorf("player %q is not in active combat", uid)
    }

    for _, c := range cbt.Combatants {
        if c.ID == targetID {
            c.ACMod += mod
            return nil
        }
    }
    return fmt.Errorf("combatant %q not found in combat", targetID)
}

// ApplyCombatantAttackMod adds mod to the named combatant's AttackMod in the player's active combat.
// Use to apply attack penalties (e.g. demoralize, frightened).
//
// Precondition: uid must be a player in active combat; targetID must be a combatant in that combat.
// Postcondition: Returns nil on success.
func (h *CombatHandler) ApplyCombatantAttackMod(uid, targetID string, mod int) error {
    sess, ok := h.sessions.GetPlayer(uid)
    if !ok {
        return fmt.Errorf("player %q not found", uid)
    }

    h.combatMu.Lock()
    defer h.combatMu.Unlock()

    cbt, ok := h.engine.GetCombat(sess.RoomID)
    if !ok {
        return fmt.Errorf("player %q is not in active combat", uid)
    }

    for _, c := range cbt.Combatants {
        if c.ID == targetID {
            c.AttackMod += mod
            return nil
        }
    }
    return fmt.Errorf("combatant %q not found in combat", targetID)
}
```

### Step 7: Run the new tests

```bash
go test ./internal/game/combat/... -run "TestResolveAttack_ACMod|TestResolveAttack_AttackMod|TestActionQueue_DeductAP" -v
```

Expected: all PASS.

### Step 8: Run full suite

```bash
go test ./... -count=1 2>&1 | tail -10
```

Expected: all PASS.

### Step 9: Commit

```bash
git add internal/game/combat/combat.go internal/game/combat/resolver.go \
        internal/game/combat/action.go internal/gameserver/combat_handler.go \
        internal/game/combat/resolver_test.go internal/game/combat/action_test.go
git commit -m "feat: add ACMod/AttackMod to Combatant, DeductAP to ActionQueue, SpendAP/ApplyCombatantACMod to CombatHandler"
```

---

## Task 3: `raise` command (Raise a Shield)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/raise_shield.go`
- Create: `internal/game/command/raise_shield_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing test

Create `internal/game/command/raise_shield_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleRaiseShield_NoArgs(t *testing.T) {
    req, err := command.HandleRaiseShield(nil)
    require.NoError(t, err)
    assert.NotNil(t, req)
}

func TestHandleRaiseShield_WithArgs_Ignored(t *testing.T) {
    req, err := command.HandleRaiseShield([]string{"extra"})
    require.NoError(t, err)
    assert.NotNil(t, req)
}
```

### Step 2: Run to verify fail

```bash
go test ./internal/game/command/... -run "TestHandleRaiseShield" -v 2>&1 | tail -10
```

Expected: compile error — function not defined.

### Step 3: Implement CMD-1, CMD-2, CMD-3

**CMD-1 + CMD-2**: In `internal/game/command/commands.go`:

Add constant after `HandlerAction`:
```go
HandlerRaiseShield = "raise_shield"
```

Add entry to `BuiltinCommands()`:
```go
{Name: "raise", Aliases: []string{"rs"}, Help: "Raise your shield (+2 AC until start of next turn). Requires a shield in the off-hand slot.", Category: CategoryCombat, Handler: HandlerRaiseShield},
```

**CMD-3**: Create `internal/game/command/raise_shield.go`:

```go
package command

// RaiseShieldRequest is the parsed form of the raise command.
//
// Precondition: none.
type RaiseShieldRequest struct{}

// HandleRaiseShield parses the arguments for the "raise" command.
// Arguments are ignored — raise takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *RaiseShieldRequest and nil error always.
func HandleRaiseShield(args []string) (*RaiseShieldRequest, error) {
    return &RaiseShieldRequest{}, nil
}
```

### Step 4: Run CMD-3 tests

```bash
go test ./internal/game/command/... -run "TestHandleRaiseShield" -v
```

Expected: all PASS.

### Step 5: CMD-4 — Add proto message

In `api/proto/game/v1/game.proto`:

After `ActionRequest action = 48;` in the `ClientMessage` oneof, add:
```protobuf
RaiseShieldRequest raise_shield = 49;
```

After `message ActionRequest { ... }` (near the end), add:
```protobuf
// RaiseShieldRequest asks the server to raise the player's shield.
message RaiseShieldRequest {}
```

Run:
```bash
make proto
```

Expected: `internal/gameserver/gamev1/game.pb.go` regenerated without errors.

### Step 6: CMD-5 — Bridge handler

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap`:
```go
command.HandlerRaiseShield: bridgeRaiseShield,
```

Add function after `bridgeStatus`:
```go
// bridgeRaiseShield builds a RaiseShieldRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a RaiseShieldRequest; done is false.
func bridgeRaiseShield(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_RaiseShield{RaiseShield: &gamev1.RaiseShieldRequest{}},
    }}, nil
}
```

Verify:
```bash
go test ./internal/frontend/handlers/... -run "TestAllCommandHandlersAreWired" -v
```

Expected: PASS.

### Step 7: CMD-6 — gRPC handler

In `internal/gameserver/grpc_service.go`:

Add case to the `dispatch` type switch (after the `ActionRequest` case):
```go
case *gamev1.ClientMessage_RaiseShield:
    return s.handleRaiseShield(uid)
```

Add handler function near the other combat handlers:

```go
// handleRaiseShield applies the shield_raised condition (+2 AC for one round).
// Requires a shield equipped in the off-hand slot.
//
// Precondition: uid must identify a valid player session.
// Postcondition: Applies shield_raised condition; in combat, deducts 1 AP and updates Combatant ACMod.
func (s *GameServiceServer) handleRaiseShield(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    // Verify a shield is equipped in the off-hand slot.
    if sess.LoadoutSet == nil {
        return s.errorEvent("You have no equipment loaded."), nil
    }
    preset := sess.LoadoutSet.ActivePreset()
    if preset == nil || preset.OffHand == nil || !preset.OffHand.Def.IsShield() {
        return s.errorEvent("You must have a shield equipped in the off-hand slot to raise a shield."), nil
    }

    // In combat: spend 1 AP and update the combatant's ACMod.
    if sess.Status == statusInCombat {
        if err := s.combatH.SpendAP(uid, 1); err != nil {
            return s.errorEvent(err.Error()), nil
        }
        // Update the player combatant's ACMod for this round.
        if err := s.combatH.ApplyCombatantACMod(uid, uid, +2); err != nil {
            s.logger.Warn("handleRaiseShield: ApplyCombatantACMod failed", zap.String("uid", uid), zap.Error(err))
        }
    }

    // Apply the shield_raised condition to the player session.
    if s.condRegistry != nil {
        def, ok := s.condRegistry.Get("shield_raised")
        if ok {
            if sess.Conditions == nil {
                sess.Conditions = condition.NewActiveSet()
            }
            _ = sess.Conditions.Apply(uid, def, 1, -1)
        }
    }

    return s.messageEvent("You raise your shield. (+2 AC until start of next turn)"), nil
}
```

Note: Check if `s.errorEvent` and `s.messageEvent` helper functions exist in grpc_service.go. If not, use the existing pattern for building `*gamev1.ServerEvent` with a `MessageEvent` or `ErrorEvent` payload (look at how other handlers return messages).

Also check: `statusInCombat` is defined in `action_handler.go` as `const statusInCombat = int32(2)`. You can use it directly since both files are in the `gameserver` package.

### Step 8: Run full suite

```bash
go test ./... -count=1 2>&1 | tail -10
```

Expected: all PASS.

### Step 9: Commit

```bash
git add internal/game/command/commands.go \
        internal/game/command/raise_shield.go \
        internal/game/command/raise_shield_test.go \
        api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add raise command (raise_shield) — CMD-1 through CMD-7"
```

---

## Task 4: `cover` command (Take Cover)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/take_cover.go`
- Create: `internal/game/command/take_cover_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing test

Create `internal/game/command/take_cover_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleTakeCover_NoArgs(t *testing.T) {
    req, err := command.HandleTakeCover(nil)
    require.NoError(t, err)
    assert.NotNil(t, req)
}
```

### Step 2: Run to verify fail

```bash
go test ./internal/game/command/... -run "TestHandleTakeCover" -v 2>&1 | tail -10
```

### Step 3: Implement CMD-1, CMD-2, CMD-3

**CMD-1 + CMD-2** in `internal/game/command/commands.go`:

```go
HandlerTakeCover = "take_cover"
```

```go
{Name: "cover", Aliases: []string{"tc"}, Help: "Take cover (+2 AC for the encounter). Costs 1 AP in combat.", Category: CategoryCombat, Handler: HandlerTakeCover},
```

**CMD-3** — `internal/game/command/take_cover.go`:

```go
package command

// TakeCoverRequest is the parsed form of the cover command.
type TakeCoverRequest struct{}

// HandleTakeCover parses the arguments for the "cover" command.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *TakeCoverRequest and nil error always.
func HandleTakeCover(args []string) (*TakeCoverRequest, error) {
    return &TakeCoverRequest{}, nil
}
```

### Step 4: CMD-4 — Proto

In `api/proto/game/v1/game.proto`:

Add to oneof:
```protobuf
TakeCoverRequest take_cover = 50;
```

Add message:
```protobuf
// TakeCoverRequest asks the server to have the player take cover.
message TakeCoverRequest {}
```

```bash
make proto
```

### Step 5: CMD-5 — Bridge handler

In `bridge_handlers.go`:
```go
command.HandlerTakeCover: bridgeTakeCover,
```

```go
// bridgeTakeCover builds a TakeCoverRequest.
func bridgeTakeCover(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_TakeCover{TakeCover: &gamev1.TakeCoverRequest{}},
    }}, nil
}
```

```bash
go test ./internal/frontend/handlers/... -run "TestAllCommandHandlersAreWired" -v
```

### Step 6: CMD-6 — gRPC handler

Add to dispatch:
```go
case *gamev1.ClientMessage_TakeCover:
    return s.handleTakeCover(uid)
```

Add handler:

```go
// handleTakeCover applies the in_cover condition (+2 AC for the encounter).
//
// Precondition: uid must identify a valid player session.
// Postcondition: Applies in_cover condition; in combat, deducts 1 AP and updates Combatant ACMod.
func (s *GameServiceServer) handleTakeCover(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    // In combat: spend 1 AP and update combatant ACMod.
    if sess.Status == statusInCombat {
        if err := s.combatH.SpendAP(uid, 1); err != nil {
            return s.errorEvent(err.Error()), nil
        }
        if err := s.combatH.ApplyCombatantACMod(uid, uid, +2); err != nil {
            s.logger.Warn("handleTakeCover: ApplyCombatantACMod failed", zap.String("uid", uid), zap.Error(err))
        }
    }

    // Apply the in_cover condition to the player session.
    if s.condRegistry != nil {
        def, ok := s.condRegistry.Get("in_cover")
        if ok {
            if sess.Conditions == nil {
                sess.Conditions = condition.NewActiveSet()
            }
            _ = sess.Conditions.Apply(uid, def, 1, -1)
        }
    }

    return s.messageEvent("You take cover. (+2 AC for the encounter)"), nil
}
```

### Step 7: Run full suite and commit

```bash
go test ./... -count=1 2>&1 | tail -10
```

```bash
git add internal/game/command/commands.go \
        internal/game/command/take_cover.go \
        internal/game/command/take_cover_test.go \
        api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add cover command (take_cover) — CMD-1 through CMD-7"
```

---

## Task 5: `aid` command (First Aid)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/first_aid.go`
- Create: `internal/game/command/first_aid_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing tests

Create `internal/game/command/first_aid_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleFirstAid_NoArgs(t *testing.T) {
    req, err := command.HandleFirstAid(nil)
    require.NoError(t, err)
    assert.NotNil(t, req)
}
```

### Step 2: Implement CMD-1, CMD-2, CMD-3

**CMD-1 + CMD-2**:
```go
HandlerFirstAid = "first_aid"
```
```go
{Name: "aid", Aliases: []string{"fa"}, Help: "Apply first aid (patch_job DC 15; success heals 2d8+4 HP). Costs 2 AP in combat.", Category: CategoryCombat, Handler: HandlerFirstAid},
```

**CMD-3** — `internal/game/command/first_aid.go`:

```go
package command

// FirstAidRequest is the parsed form of the aid command.
type FirstAidRequest struct{}

// HandleFirstAid parses the arguments for the "aid" command.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *FirstAidRequest and nil error always.
func HandleFirstAid(args []string) (*FirstAidRequest, error) {
    return &FirstAidRequest{}, nil
}
```

### Step 3: CMD-4 — Proto

```protobuf
FirstAidRequest first_aid = 51;
```
```protobuf
// FirstAidRequest asks the server to apply first aid to the player.
message FirstAidRequest {}
```

```bash
make proto
```

### Step 4: CMD-5 — Bridge handler

```go
command.HandlerFirstAid: bridgeFirstAid,
```

```go
// bridgeFirstAid builds a FirstAidRequest.
func bridgeFirstAid(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_FirstAid{FirstAid: &gamev1.FirstAidRequest{}},
    }}, nil
}
```

### Step 5: CMD-6 — gRPC handler

Add to dispatch:
```go
case *gamev1.ClientMessage_FirstAid:
    return s.handleFirstAid(uid)
```

Add handler. Note: `s.dice` is the `*dice.Roller` field. `skillRankBonus` is defined in `action_handler.go` in the same package.

```go
// handleFirstAid performs a patch_job skill check (DC 15).
// On success, heals 2d8+4 HP (self). Costs 2 AP in combat.
//
// Precondition: uid must identify a valid player session.
// Postcondition: On skill check success, heals player HP and persists via charSaver.
func (s *GameServiceServer) handleFirstAid(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    // In combat: spend 2 AP.
    if sess.Status == statusInCombat {
        if err := s.combatH.SpendAP(uid, 2); err != nil {
            return s.errorEvent(err.Error()), nil
        }
    }

    // Skill check: 1d20 + patch_job rank bonus vs DC 15.
    rollResult, err := s.dice.RollExpr("1d20")
    if err != nil {
        return nil, fmt.Errorf("handleFirstAid: rolling d20: %w", err)
    }
    roll := rollResult.Total()
    bonus := skillRankBonus(sess.Skills["patch_job"])
    total := roll + bonus
    dc := 15

    if total < dc {
        msg := fmt.Sprintf("First aid check: rolled %d+%d=%d vs DC %d — failure. You fail to apply treatment.", roll, bonus, total, dc)
        return s.messageEvent(msg), nil
    }

    // Success: heal 2d8+4.
    healResult, err := s.dice.RollExpr("2d8+4")
    if err != nil {
        return nil, fmt.Errorf("handleFirstAid: rolling heal: %w", err)
    }
    healed := healResult.Total()
    newHP := sess.CurrentHP + healed
    if newHP > sess.MaxHP {
        newHP = sess.MaxHP
    }
    sess.CurrentHP = newHP

    ctx := context.Background()
    if s.charSaver != nil {
        if saveErr := s.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, newHP); saveErr != nil {
            s.logger.Warn("handleFirstAid: saving HP", zap.String("uid", uid), zap.Error(saveErr))
        }
    }

    // Push HP update event.
    hpEvt := &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_HpUpdate{
            HpUpdate: &gamev1.HpUpdateEvent{
                CurrentHp: int32(newHP),
                MaxHp:     int32(sess.MaxHP),
            },
        },
    }
    if data, err := proto.Marshal(hpEvt); err == nil {
        _ = sess.Entity.Push(data)
    }

    msg := fmt.Sprintf("First aid check: rolled %d+%d=%d vs DC %d — success! You recover %d HP. (%d/%d)", roll, bonus, total, dc, healed, newHP, sess.MaxHP)
    return s.messageEvent(msg), nil
}
```

Note: Check imports in grpc_service.go. Ensure `"context"` and `"google.golang.org/protobuf/proto"` are present (they almost certainly are).

### Step 6: Run full suite and commit

```bash
go test ./... -count=1 2>&1 | tail -10
```

```bash
git add internal/game/command/commands.go \
        internal/game/command/first_aid.go \
        internal/game/command/first_aid_test.go \
        api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add aid command (first_aid) — CMD-1 through CMD-7"
```

---

## Task 6: `feint` command

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/feint.go`
- Create: `internal/game/command/feint_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing tests

Create `internal/game/command/feint_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleFeint_WithTarget(t *testing.T) {
    req, err := command.HandleFeint([]string{"bandit"})
    require.NoError(t, err)
    assert.Equal(t, "bandit", req.Target)
}

func TestHandleFeint_NoArgs_ReturnsEmptyTarget(t *testing.T) {
    req, err := command.HandleFeint(nil)
    require.NoError(t, err)
    assert.Equal(t, "", req.Target)
}
```

### Step 2: Implement CMD-1, CMD-2, CMD-3

**CMD-1 + CMD-2**:
```go
HandlerFeint = "feint"
```
```go
{Name: "feint", Aliases: nil, Help: "Feint against a target (grift vs Perception DC; success applies flat_footed -2 AC for 1 round). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerFeint},
```

**CMD-3** — `internal/game/command/feint.go`:

```go
package command

// FeintRequest is the parsed form of the feint command.
//
// Precondition: Target may be empty (handler will return error in that case).
type FeintRequest struct {
    Target string
}

// HandleFeint parses the arguments for the "feint" command.
//
// Precondition: args is the slice of words following "feint" (may be empty).
// Postcondition: Returns a non-nil *FeintRequest and nil error always.
func HandleFeint(args []string) (*FeintRequest, error) {
    req := &FeintRequest{}
    if len(args) >= 1 {
        req.Target = args[0]
    }
    return req, nil
}
```

### Step 3: CMD-4 — Proto

```protobuf
FeintRequest feint = 52;
```
```protobuf
// FeintRequest asks the server to feint against a target NPC.
message FeintRequest {
    string target = 1;
}
```

```bash
make proto
```

### Step 4: CMD-5 — Bridge handler

```go
command.HandlerFeint: bridgeFeint,
```

```go
// bridgeFeint builds a FeintRequest with the target name.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FeintRequest; done is false.
func bridgeFeint(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Feint{Feint: &gamev1.FeintRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}
```

### Step 5: CMD-6 — gRPC handler

Add to dispatch:
```go
case *gamev1.ClientMessage_Feint:
    return s.handleFeint(uid, p.Feint)
```

Add handler:

```go
// handleFeint performs a grift skill check against the target NPC's Perception DC.
// On success, applies flat_footed (-2 AC) to the target combatant for this round.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target's ACMod is decremented by 2.
func (s *GameServiceServer) handleFeint(uid string, req *gamev1.FeintRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    if sess.Status != statusInCombat {
        return s.errorEvent("Feint is only available in combat."), nil
    }

    if req.GetTarget() == "" {
        return s.errorEvent("Usage: feint <target>"), nil
    }

    // Spend 1 AP.
    if err := s.combatH.SpendAP(uid, 1); err != nil {
        return s.errorEvent(err.Error()), nil
    }

    // Find target NPC in room to get Perception DC.
    inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
    if inst == nil {
        return s.errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
    }

    // Skill check: 1d20 + grift bonus vs target Perception.
    rollResult, err := s.dice.RollExpr("1d20")
    if err != nil {
        return nil, fmt.Errorf("handleFeint: rolling d20: %w", err)
    }
    roll := rollResult.Total()
    bonus := skillRankBonus(sess.Skills["grift"])
    total := roll + bonus
    dc := inst.Perception

    detail := fmt.Sprintf("Feint (grift DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
    if total < dc {
        return s.messageEvent(detail + " — failure. Your feint is transparent."), nil
    }

    // Success: apply flat_footed to NPC combatant.
    if err := s.combatH.ApplyCombatantACMod(uid, inst.ID, -2); err != nil {
        s.logger.Warn("handleFeint: ApplyCombatantACMod failed", zap.String("npc_id", inst.ID), zap.Error(err))
    }

    return s.messageEvent(detail + fmt.Sprintf(" — success! %s is caught off-guard (-2 AC this round).", inst.Name())), nil
}
```

### Step 6: Run full suite and commit

```bash
go test ./... -count=1 2>&1 | tail -10
```

```bash
git add internal/game/command/commands.go \
        internal/game/command/feint.go \
        internal/game/command/feint_test.go \
        api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add feint command — CMD-1 through CMD-7"
```

---

## Task 7: `demoralize` command

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/demoralize.go`
- Create: `internal/game/command/demoralize_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing tests

Create `internal/game/command/demoralize_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleDemoralize_WithTarget(t *testing.T) {
    req, err := command.HandleDemoralize([]string{"thug"})
    require.NoError(t, err)
    assert.Equal(t, "thug", req.Target)
}

func TestHandleDemoralize_NoArgs_ReturnsEmptyTarget(t *testing.T) {
    req, err := command.HandleDemoralize(nil)
    require.NoError(t, err)
    assert.Equal(t, "", req.Target)
}
```

### Step 2: Implement CMD-1, CMD-2, CMD-3

**CMD-1 + CMD-2**:
```go
HandlerDemoralize = "demoralize"
```
```go
{Name: "demoralize", Aliases: []string{"dm"}, Help: "Demoralize a target (smooth_talk vs target Level+10 DC; success applies frightened -1 attack/-1 AC). Combat only, costs 1 AP.", Category: CategoryCombat, Handler: HandlerDemoralize},
```

**CMD-3** — `internal/game/command/demoralize.go`:

```go
package command

// DemoralizeRequest is the parsed form of the demoralize command.
type DemoralizeRequest struct {
    Target string
}

// HandleDemoralize parses the arguments for the "demoralize" command.
//
// Precondition: args is the slice of words following "demoralize" (may be empty).
// Postcondition: Returns a non-nil *DemoralizeRequest and nil error always.
func HandleDemoralize(args []string) (*DemoralizeRequest, error) {
    req := &DemoralizeRequest{}
    if len(args) >= 1 {
        req.Target = args[0]
    }
    return req, nil
}
```

### Step 3: CMD-4 — Proto

```protobuf
DemoralizeRequest demoralize = 53;
```
```protobuf
// DemoralizeRequest asks the server to demoralize a target NPC.
message DemoralizeRequest {
    string target = 1;
}
```

```bash
make proto
```

### Step 4: CMD-5 — Bridge handler

```go
command.HandlerDemoralize: bridgeDemoralize,
```

```go
// bridgeDemoralize builds a DemoralizeRequest with the target name.
func bridgeDemoralize(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Demoralize{Demoralize: &gamev1.DemoralizeRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}
```

### Step 5: CMD-6 — gRPC handler

Add to dispatch:
```go
case *gamev1.ClientMessage_Demoralize:
    return s.handleDemoralize(uid, p.Demoralize)
```

Add handler. The DC for demoralize is `inst.Level + 10` (a reasonable Will-analog: higher-level NPCs are harder to intimidate):

```go
// handleDemoralize performs a smooth_talk skill check against the target NPC's Will DC (Level+10).
// On success, applies frightened (-1 attack, -1 AC) to the target combatant for this round.
// Combat only; costs 1 AP.
//
// Precondition: uid must be in active combat; req.Target must name an NPC in the room.
// Postcondition: On success, target's ACMod and AttackMod are each decremented by 1.
func (s *GameServiceServer) handleDemoralize(uid string, req *gamev1.DemoralizeRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    if sess.Status != statusInCombat {
        return s.errorEvent("Demoralize is only available in combat."), nil
    }

    if req.GetTarget() == "" {
        return s.errorEvent("Usage: demoralize <target>"), nil
    }

    // Spend 1 AP.
    if err := s.combatH.SpendAP(uid, 1); err != nil {
        return s.errorEvent(err.Error()), nil
    }

    // Find target NPC.
    inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
    if inst == nil {
        return s.errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
    }

    // Skill check: 1d20 + smooth_talk vs DC = target Level + 10.
    rollResult, err := s.dice.RollExpr("1d20")
    if err != nil {
        return nil, fmt.Errorf("handleDemoralize: rolling d20: %w", err)
    }
    roll := rollResult.Total()
    bonus := skillRankBonus(sess.Skills["smooth_talk"])
    total := roll + bonus
    dc := inst.Level + 10

    detail := fmt.Sprintf("Demoralize (smooth_talk DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
    if total < dc {
        return s.messageEvent(detail + " — failure. Your intimidation falls flat."), nil
    }

    // Success: apply frightened to NPC combatant (-1 AC, -1 attack).
    if err := s.combatH.ApplyCombatantACMod(uid, inst.ID, -1); err != nil {
        s.logger.Warn("handleDemoralize: ApplyCombatantACMod failed", zap.String("npc_id", inst.ID), zap.Error(err))
    }
    if err := s.combatH.ApplyCombatantAttackMod(uid, inst.ID, -1); err != nil {
        s.logger.Warn("handleDemoralize: ApplyCombatantAttackMod failed", zap.String("npc_id", inst.ID), zap.Error(err))
    }

    return s.messageEvent(detail + fmt.Sprintf(" — success! %s is shaken (-1 attack/-1 AC this round).", inst.Name())), nil
}
```

### Step 6: Run full suite and commit

```bash
go test ./... -count=1 2>&1 | tail -10
```

```bash
git add internal/game/command/commands.go \
        internal/game/command/demoralize.go \
        internal/game/command/demoralize_test.go \
        api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add demoralize command — CMD-1 through CMD-7"
```

---

## Task 8: FEATURES.md stubs and mark implemented actions complete

**Files:**
- Modify: `docs/requirements/FEATURES.md`

### Step 1: Find the Actions section

```bash
grep -n "Actions\|PF2E\|pf2e\|raise_shield\|take_cover\|first_aid\|feint\|demoralize" docs/requirements/FEATURES.md | head -30
```

### Step 2: Update the FEATURES.md

Under the existing "Full import of all PF2E actions" section, mark the five implemented actions as `[x]` and add stub entries for all remaining actions under new categories.

**Mark implemented:**
```
- [x] Raise a Shield — raise shield equipped in off-hand (+2 AC, 1 AP, duration: round)
- [x] Take Cover — take cover (+2 AC, 1 AP, duration: encounter)
- [x] First Aid — patch_job DC 15 skill check; success heals 2d8+4 HP (2 AP)
- [x] Feint — grift vs target Perception DC; success applies flat_footed (-2 AC, 1 AP, combat only)
- [x] Demoralize — smooth_talk vs target Level+10 DC; success applies frightened (-1 attack/-1 AC, 1 AP, combat only)
```

**Add under new section `### Combat > Athletics Actions`:**
```
- [ ] Grapple — Athletics vs Fortitude DC; applies Grabbed condition to target
- [ ] Shove — Athletics vs Fortitude DC; pushes target back
- [ ] Trip — Athletics vs Reflex DC; applies Prone condition to target
- [ ] Disarm — Athletics vs Reflex DC; causes target to drop held item
- [ ] Climb — Athletics vs surface DC; allows vertical movement
- [ ] Swim — Athletics vs current DC; allows movement through water
```

**Add under new section `### Combat > Tactical Actions`:**
```
- [ ] Step — move 5 ft without triggering reactions (1 AP, combat only)
- [ ] Seek — Perception check to detect hidden creatures or objects (1 AP)
- [ ] Sense Motive — Perception vs Deception DC to detect lies or intent (1 AP)
- [ ] Escape — Athletics or Acrobatics vs grappler's DC to break free from Grabbed
- [ ] Delay — forfeit current initiative position to act later in the round
```

**Add under new section `### Combat > Stealth & Deception Actions`:**
```
- [ ] Hide — Stealth vs Perception DC; become Hidden (1 AP, combat)
- [ ] Sneak — Stealth vs Perception DC; move while remaining Hidden (1 AP)
- [ ] Create Diversion — Deception vs Perception DC; momentarily become Hidden (1 AP)
- [ ] Tumble Through — Acrobatics vs Reflex DC; move through enemy-occupied space (1 AP)
```

**Add under new section `### General Actions`:**
```
- [ ] Aid — skill check to grant an ally +2 circumstance bonus on their next check
- [ ] Ready — prepare a reaction to trigger on a specified condition (2 AP)
- [ ] Hero Point — spend a hero point to reroll a check or avoid being knocked out
```

**Add under new section `### Exploration System`:**
```
- [ ] Avoid Notice — use Stealth to avoid detection while exploring between rooms
- [ ] Defend — enter a defensive posture (+2 AC) while exploring
- [ ] Detect Magic — Arcana/tech_lore check to sense magical or tech auras in room
- [ ] Search — Perception check to find hidden objects, creatures, or traps in room
- [ ] Scout — move ahead of the group and report threats before the party arrives
- [ ] Follow the Expert — follow a skilled ally to share their proficiency bonus
- [ ] Investigate — gather clues about a room, object, or situation
- [ ] Refocus — spend 10 minutes to restore a Focus Point for special abilities
```

**Add under new section `### Downtime System`:**
```
- [ ] Earn Income — skill check (hustle, grift, or smooth_talk) to earn credits over a downtime period
- [ ] Craft — Craft check to build an item from components over a downtime period
- [ ] Treat Disease — patch_job check to reduce disease severity over a downtime period
- [ ] Subsist — wasteland or hustle check to find food and shelter
- [ ] Create Forgery — grift check to produce convincing fake documents
- [ ] Long-Term Rest — extended rest (8+ hours) to recover HP and remove encounter-duration conditions
- [ ] Retrain (extend) — extend existing trainskill command to support feat and archetype retraining
```

**Add under new section `### Gear System`:**
```
- [ ] Repair — rigging check to restore a broken item to functionality
- [ ] Affix a Precious Material — rigging check to permanently add a material property to a weapon or armor
- [ ] Swap — swap equipped weapon or armor loadout mid-exploration without AP cost
```

### Step 3: Run tests (no code changes, just verify nothing broke)

```bash
go test ./... -count=1 2>&1 | tail -10
```

### Step 4: Commit

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: add PF2E action stubs to FEATURES.md under new categories"
```

---

## Verification

After all 8 tasks are complete:

```bash
go test ./... -count=1 2>&1 | tail -10
```

All tests must pass. Then proceed to `superpowers:finishing-a-development-branch`.

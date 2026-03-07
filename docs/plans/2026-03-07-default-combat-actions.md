# Default Combat Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Allow players to set a persistent default combat action (`attack`, `strike`, `pass`, etc.) that auto-queues each round when they haven't typed a command.

**Architecture:** Four concerns in sequence: (1) DB migration + repository method + Character/Session model fields; (2) `combat_default` command (CMD-1 through CMD-7); (3) `LastCombatTarget` tracking in `CombatHandler.Attack/Strike`; (4) `autoQueuePlayersLocked` in `resolveAndAdvanceLocked`.

**Tech Stack:** Go, pgx/v5, protobuf, zap, pgregory.net/rapid (property tests)

---

### Task 1: DB migration + repository + model fields

**Files:**
- Modify: `internal/storage/postgres/main_test.go` (add column to migration SQL)
- Modify: `internal/game/character/model.go` (add `DefaultCombatAction string` field)
- Modify: `internal/storage/postgres/character.go` (add `SaveDefaultCombatAction` + load in scan)
- Modify: `internal/game/session/manager.go` (add `DefaultCombatAction string` and `LastCombatTarget string` fields)
- Modify: `internal/gameserver/grpc_service.go` (populate `sess.DefaultCombatAction` at join)
- Test: `internal/storage/postgres/character_progress_test.go` (or a new file for this method)

**Context:**
- `internal/game/character/model.go:24` — `Character` struct; add `DefaultCombatAction string` after `CurrentHP int`
- `internal/storage/postgres/character.go` — add `SaveDefaultCombatAction(ctx, id int64, action string) error` using the pattern from `SaveAbilities` (single UPDATE). Also update the SELECT that loads `dbChar` to include `default_combat_action` and scan it into `dbChar.DefaultCombatAction`.
- `internal/game/session/manager.go` — `PlayerSession` struct (line 12); add after `PendingBoosts int`:
  ```go
  DefaultCombatAction string // persisted; default "pass"
  LastCombatTarget     string // in-memory; last explicit attack/strike target
  ```
- `internal/gameserver/grpc_service.go` ~line 368 — where `dbChar` fields are read into session at join:
  ```go
  sess.DefaultCombatAction = dbChar.DefaultCombatAction
  if sess.DefaultCombatAction == "" {
      sess.DefaultCombatAction = "pass"
  }
  ```
- `internal/storage/postgres/main_test.go` `applyAllMigrations` — add inside the single SQL block:
  ```sql
  ALTER TABLE characters ADD COLUMN IF NOT EXISTS default_combat_action TEXT NOT NULL DEFAULT 'pass';
  ```

**Step 1: Write failing test for SaveDefaultCombatAction**

In `internal/storage/postgres/character_test.go` (or a new `character_combat_test.go`), add:

```go
func TestSaveDefaultCombatAction_RoundTrip(t *testing.T) {
    ctx := context.Background()
    repo := pgstore.NewCharacterRepository(sharedPool)
    ch := createTestCharacter(t, repo, ctx)

    err := repo.SaveDefaultCombatAction(ctx, ch.ID, "attack")
    require.NoError(t, err)

    loaded, err := repo.GetByID(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, "attack", loaded.DefaultCombatAction)
}

func TestSaveDefaultCombatAction_InvalidID_ReturnsError(t *testing.T) {
    ctx := context.Background()
    repo := pgstore.NewCharacterRepository(sharedPool)
    err := repo.SaveDefaultCombatAction(ctx, 0, "attack")
    require.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/storage/postgres/... -run TestSaveDefaultCombatAction -timeout 60s -v
```
Expected: FAIL — `SaveDefaultCombatAction` undefined

**Step 3: Add migration column to test setup**

In `internal/storage/postgres/main_test.go`, inside `applyAllMigrations`, append to the SQL string:
```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS default_combat_action TEXT NOT NULL DEFAULT 'pass';
```

**Step 4: Add `DefaultCombatAction` to `Character` struct**

In `internal/game/character/model.go` after `CurrentHP int`:
```go
DefaultCombatAction string // persisted default combat action; "pass" when unset
```

**Step 5: Implement `SaveDefaultCombatAction` in character.go**

```go
// SaveDefaultCombatAction persists the player's preferred default combat action.
//
// Precondition: characterID > 0; action must be a non-empty valid action string.
// Postcondition: characters.default_combat_action updated for the given ID.
func (r *CharacterRepository) SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error {
    if characterID <= 0 {
        return fmt.Errorf("SaveDefaultCombatAction: characterID must be > 0")
    }
    if action == "" {
        return fmt.Errorf("SaveDefaultCombatAction: action must be non-empty")
    }
    tag, err := r.db.Exec(ctx,
        `UPDATE characters SET default_combat_action = $2 WHERE id = $1`,
        characterID, action,
    )
    if err != nil {
        return fmt.Errorf("SaveDefaultCombatAction: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return fmt.Errorf("SaveDefaultCombatAction: character %d not found", characterID)
    }
    return nil
}
```

**Step 6: Update `GetByID` SELECT to load `default_combat_action`**

Find the SELECT in `character.go` that populates a `Character`. Add `default_combat_action` to the column list and scan it into `ch.DefaultCombatAction`. The exact query and scan varies — read the file first to find the right location.

**Step 7: Add session fields**

In `internal/game/session/manager.go`, add after `PendingBoosts int`:
```go
DefaultCombatAction string // persisted default combat action; "pass" when unset
LastCombatTarget     string // last explicit attack/strike target; in-memory only
```

**Step 8: Populate session at join**

In `internal/gameserver/grpc_service.go`, in the join handler where `dbChar` fields are copied to `sess` (around line 368):
```go
sess.DefaultCombatAction = dbChar.DefaultCombatAction
if sess.DefaultCombatAction == "" {
    sess.DefaultCombatAction = "pass"
}
```

**Step 9: Run tests**

```bash
go test ./internal/storage/postgres/... -timeout 120s -v -run TestSaveDefaultCombatAction
go build ./...
```
Expected: PASS

**Step 10: Commit**

```bash
git add internal/game/character/model.go internal/game/session/manager.go \
    internal/storage/postgres/character.go internal/storage/postgres/main_test.go \
    internal/gameserver/grpc_service.go internal/storage/postgres/character_test.go
git commit -m "feat: add default_combat_action to character model, session, and DB"
```

---

### Task 2: `combat_default` command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go` (CMD-1, CMD-2)
- Create: `internal/game/command/combat_default.go` (CMD-3)
- Create: `internal/game/command/combat_default_test.go` (CMD-3 tests)
- Modify: `api/proto/game/v1/game.proto` (CMD-4)
- Modify: `internal/frontend/handlers/bridge_handlers.go` (CMD-5)
- Modify: `internal/gameserver/grpc_service.go` (CMD-6)

**Context:**
- Model after `levelup.go` / `levelup_test.go` exactly — same structure
- Valid actions: `attack`, `strike`, `pass`, `flee`, `reload`, `fire_burst`, `fire_automatic`, `throw`
- `HandleCombatDefault(rawArgs string) string` — returns normalized action name or usage/error string
- Export `ValidCombatActions []string` (same pattern as `ValidAbilities`)
- Bridge: check if result starts with `"Usage:"` or `"Unknown action"` → write error; otherwise send proto
- gRPC handler: `handleCombatDefault(uid, action string)` — update `sess.DefaultCombatAction`, call `charRepo.SaveDefaultCombatAction`, send confirmation
- `charRepo` must satisfy `CharacterSaver` interface — add `SaveDefaultCombatAction` to the interface
- Highest proto field in `ClientMessage` oneof: **45**. Use **46** for `CombatDefaultRequest`

**Step 1: Write failing command handler tests**

Create `internal/game/command/combat_default_test.go`:

```go
package command_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
)

func TestHandleCombatDefault_NoArg_ReturnsUsage(t *testing.T) {
    result := command.HandleCombatDefault("")
    assert.Contains(t, result, "Usage:")
}

func TestHandleCombatDefault_InvalidAction_ReturnsError(t *testing.T) {
    result := command.HandleCombatDefault("dance")
    assert.Contains(t, result, "Unknown action")
}

func TestHandleCombatDefault_ValidActions(t *testing.T) {
    for _, action := range command.ValidCombatActions {
        t.Run(action, func(t *testing.T) {
            result := command.HandleCombatDefault(action)
            assert.Equal(t, action, result)
        })
    }
}

func TestHandleCombatDefault_CaseInsensitive(t *testing.T) {
    assert.Equal(t, "attack", command.HandleCombatDefault("ATTACK"))
    assert.Equal(t, "strike", command.HandleCombatDefault("Strike"))
}

func TestPropertyHandleCombatDefault_InvalidAlwaysError(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        s := rapid.StringMatching(`[^a-z_].*`).Draw(rt, "invalid")
        result := command.HandleCombatDefault(s)
        assert.True(rt, len(result) > 0)
        for _, a := range command.ValidCombatActions {
            if s == a {
                return // skip if accidentally valid
            }
        }
        assert.Contains(rt, result, "Unknown action")
    })
}

func TestPropertyHandleCombatDefault_ValidAlwaysReturnsNormalized(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        action := rapid.SampledFrom(command.ValidCombatActions).Draw(rt, "action")
        result := command.HandleCombatDefault(action)
        assert.Equal(rt, action, result)
    })
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/game/command/... -run TestHandleCombatDefault -timeout 30s -v
```
Expected: FAIL — `HandleCombatDefault` undefined

**Step 3: Add CMD-1 and CMD-2 to commands.go**

In `internal/game/command/commands.go`:

Add constant (after `HandlerLevelUp`):
```go
HandlerCombatDefault = "combat_default"
```

Add to `BuiltinCommands()` (after the levelup entry):
```go
{Name: "combat_default", Aliases: []string{"cd"}, Help: "Set your default combat action (attack/strike/pass/flee/reload/fire_burst/fire_automatic/throw)", Category: CategoryCombat, Handler: HandlerCombatDefault},
```

**Step 4: Create combat_default.go**

```go
package command

import (
    "strings"
)

// ValidCombatActions is the set of action names accepted by HandleCombatDefault.
var ValidCombatActions = []string{
    "attack", "strike", "pass", "flee",
    "reload", "fire_burst", "fire_automatic", "throw",
}

var validCombatActionMap map[string]bool

func init() {
    validCombatActionMap = make(map[string]bool, len(ValidCombatActions))
    for _, a := range ValidCombatActions {
        validCombatActionMap[a] = true
    }
}

// HandleCombatDefault validates and normalizes the default combat action argument.
//
// Precondition: rawArgs is the raw argument string from the player (may be empty or mixed case).
// Postcondition:
//   - Empty rawArgs → usage string.
//   - Unrecognized action → "Unknown action" error string.
//   - Valid action → normalized lowercase action name.
func HandleCombatDefault(rawArgs string) string {
    arg := strings.ToLower(strings.TrimSpace(rawArgs))
    if arg == "" {
        return "Usage: combat_default <action>\nValid actions: attack, strike, pass, flee, reload, fire_burst, fire_automatic, throw"
    }
    if !validCombatActionMap[arg] {
        return "Unknown action '" + arg + "'. Valid: attack, strike, pass, flee, reload, fire_burst, fire_automatic, throw"
    }
    return arg
}
```

**Step 5: Run command tests**

```bash
go test ./internal/game/command/... -run TestHandleCombatDefault -timeout 30s -v
```
Expected: PASS

**Step 6: CMD-4 — add proto message**

In `api/proto/game/v1/game.proto`:

Add message (near LevelUpRequest):
```proto
message CombatDefaultRequest {
  string action = 1;
}
```

Add to `ClientMessage` oneof (field 46):
```proto
CombatDefaultRequest combat_default = 46;
```

Run:
```bash
make proto
```
Expected: `internal/gameserver/gamev1/game.pb.go` regenerated with `CombatDefaultRequest`

**Step 7: CMD-5 — add bridge handler**

In `internal/frontend/handlers/bridge_handlers.go`:

Add function (near `bridgeLevelUp`):
```go
func bridgeCombatDefault(bctx *bridgeContext) (bridgeResult, error) {
    result := command.HandleCombatDefault(bctx.parsed.RawArgs)
    if strings.HasPrefix(result, "Usage:") || strings.HasPrefix(result, "Unknown action") {
        return writeErrorPrompt(bctx, result)
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_CombatDefault{
            CombatDefault: &gamev1.CombatDefaultRequest{Action: result},
        },
    }}, nil
}
```

Register in `bridgeHandlerMap`:
```go
command.HandlerCombatDefault: bridgeCombatDefault,
```

Run:
```bash
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -timeout 90s -v
```
Expected: PASS

**Step 8: CMD-6 — add gRPC handler**

First, add `SaveDefaultCombatAction` to the `CharacterSaver` interface in `grpc_service.go`:
```go
SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error
```

Also update any mock `CharacterSaver` structs in test files (`grpc_service_login_test.go`, `teleport_handler_test.go`) with a no-op method:
```go
func (m *mockCharSaver) SaveDefaultCombatAction(ctx context.Context, characterID int64, action string) error {
    return nil
}
```

Add dispatch case to the switch:
```go
case *gamev1.ClientMessage_CombatDefault:
    return s.handleCombatDefault(uid, p.CombatDefault.Action)
```

Add handler method:
```go
// handleCombatDefault persists and applies the player's preferred default combat action.
//
// Precondition: uid must be non-empty; action must be a valid combat action string.
// Postcondition: sess.DefaultCombatAction updated; persisted to DB; confirmation sent.
func (s *GameServiceServer) handleCombatDefault(uid, action string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("handleCombatDefault: session not found for %s", uid)
    }

    if action == "" {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{
                    Content: "Usage: combat_default <action>",
                    Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
                },
            },
        }, nil
    }

    ctx := context.Background()
    if err := s.charSaver.SaveDefaultCombatAction(ctx, sess.CharacterID, action); err != nil {
        s.logger.Warn("saving default combat action", zap.String("uid", uid), zap.Error(err))
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Error{
                Error: &gamev1.ErrorEvent{Message: "Failed to save default action."},
            },
        }, nil
    }

    sess.DefaultCombatAction = action

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_Message{
            Message: &gamev1.MessageEvent{
                Content: fmt.Sprintf("Default combat action set to: %s", action),
                Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
            },
        },
    }, nil
}
```

**Step 9: Build and test**

```bash
go build ./...
go test ./... -timeout 180s
```
Expected: all PASS

**Step 10: Commit**

```bash
git add internal/game/command/combat_default.go internal/game/command/combat_default_test.go \
    internal/game/command/commands.go api/proto/game/v1/game.proto \
    internal/gameserver/gamev1/game.pb.go \
    internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/grpc_service.go
git commit -m "feat: combat_default command (CMD-1 through CMD-7)"
```

---

### Task 3: LastCombatTarget tracking + autoQueuePlayersLocked

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

**Context:**

`Attack()` is at ~line 127 and `Strike()` is at ~line 188 in `combat_handler.go`. Both receive a `uid` and `target` string. Add `sess.LastCombatTarget = target` after loading the session in each method.

`resolveAndAdvanceLocked` is at ~line 563. It calls `autoQueueNPCsLocked(cbt)` at ~line 626 (for the NEXT round). Add `autoQueuePlayersLocked(cbt)` call in the SAME location — just before or after `autoQueueNPCsLocked`.

The `CombatHandler` struct has access to `h.sessions` (session store). Use `h.sessions.GetPlayer(c.ID)` for player combatants (where `c.Kind == combat.KindPlayer`).

`h.invRegistry` is available on `CombatHandler` for checking weapon types.

**Step 1: Write tests for autoQueuePlayersLocked**

Create `internal/gameserver/combat_default_test.go`:

```go
package gameserver_test

// Test that when a player has default action "attack" and a queued target,
// autoQueuePlayersLocked enqueues an attack for them before round resolution.
// This is an integration test using the existing combat test infrastructure.
// Model after existing combat handler tests in the package.

// At minimum test:
// - Default "pass": player with no queued action gets ActionPass queued
// - Default "attack" with LastCombatTarget set: ActionAttack queued against that target
// - Default "attack" with no LastCombatTarget: ActionAttack queued against first living NPC
// - Default "attack" with no living NPCs: ActionPass queued (fallback)
// - Default "reload" with no reloadable weapon: ActionPass queued (fallback)
```

Look at existing tests in `internal/gameserver/` for the test infrastructure pattern (mock sessions, mock combat engine). Write concrete tests based on what you find.

**Step 2: Run to verify failure**

```bash
go test ./internal/gameserver/... -run TestAutoQueuePlayers -timeout 60s -v
```
Expected: FAIL — `autoQueuePlayersLocked` undefined

**Step 3: Track LastCombatTarget in Attack() and Strike()**

In `combat_handler.go` `Attack()` method, after `sess, ok := h.sessions.GetPlayer(uid)` and the ok check:
```go
sess.LastCombatTarget = target
```

In `Strike()` method, same pattern:
```go
sess.LastCombatTarget = target
```

**Step 4: Implement autoQueuePlayersLocked**

Add after `autoQueueNPCsLocked`:

```go
// autoQueuePlayersLocked enqueues the default action for any living player
// combatant that has not yet queued an action for this round.
//
// Precondition: combatMu must be held; cbt must not be nil.
// Postcondition: All living players without a queued action have ActionPass or
// their default action queued. Falls back to ActionPass when the default action
// cannot be executed (no valid target, no reloadable weapon, etc.).
func (h *CombatHandler) autoQueuePlayersLocked(cbt *combat.Combat) {
    for _, c := range cbt.Combatants {
        if c.Kind != combat.KindPlayer || c.IsDead() {
            continue
        }
        q, ok := cbt.ActionQueues[c.ID]
        if !ok || q.IsSubmitted() {
            continue // already has actions queued
        }

        sess, ok := h.sessions.GetPlayer(c.ID)
        if !ok {
            continue
        }

        defaultAction := sess.DefaultCombatAction
        if defaultAction == "" {
            defaultAction = "pass"
        }

        queued := h.tryQueueDefaultLocked(cbt, c, sess, defaultAction)
        if !queued {
            _ = cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionPass})
        }
    }
}

// tryQueueDefaultLocked attempts to queue the player's default action.
// Returns true if the action was successfully queued, false if fallback is needed.
//
// Precondition: combatMu must be held; actor and sess must not be nil.
// Postcondition: Returns true iff the action was queued successfully.
func (h *CombatHandler) tryQueueDefaultLocked(cbt *combat.Combat, actor *combat.Combatant, sess *session.PlayerSession, action string) bool {
    switch action {
    case "attack", "strike":
        target := h.resolveTargetLocked(cbt, actor, sess)
        if target == "" {
            return false
        }
        actionType := combat.ActionAttack
        if action == "strike" {
            actionType = combat.ActionStrike
        }
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: actionType, Target: target})
        return err == nil

    case "pass":
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionPass})
        return err == nil

    case "flee":
        // Flee is always queued; the flee resolution handles success/failure.
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionFlee})
        return err == nil

    case "reload":
        weaponID := h.findReloadableWeaponLocked(sess)
        if weaponID == "" {
            return false
        }
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionReload, WeaponID: weaponID})
        return err == nil

    case "fire_burst":
        target := h.resolveTargetLocked(cbt, actor, sess)
        weaponID := h.findBurstCapableWeaponLocked(sess)
        if target == "" || weaponID == "" {
            return false
        }
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionFireBurst, Target: target, WeaponID: weaponID})
        return err == nil

    case "fire_automatic":
        target := h.resolveTargetLocked(cbt, actor, sess)
        weaponID := h.findAutoCapableWeaponLocked(sess)
        if target == "" || weaponID == "" {
            return false
        }
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionFireAutomatic, Target: target, WeaponID: weaponID})
        return err == nil

    case "throw":
        target := h.resolveTargetLocked(cbt, actor, sess)
        explosiveID := h.findExplosiveLocked(sess)
        if target == "" || explosiveID == "" {
            return false
        }
        err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionThrow, Target: target, ExplosiveID: explosiveID})
        return err == nil

    default:
        return false
    }
}

// resolveTargetLocked returns the best target name for the player's default attack:
// sess.LastCombatTarget if still alive in combat, otherwise the first living NPC.
// Returns "" if no valid target exists.
//
// Precondition: combatMu must be held.
// Postcondition: Returns a non-empty target name or "".
func (h *CombatHandler) resolveTargetLocked(cbt *combat.Combat, actor *combat.Combatant, sess *session.PlayerSession) string {
    // Try last explicit target first.
    if sess.LastCombatTarget != "" {
        for _, c := range cbt.Combatants {
            if c.Name == sess.LastCombatTarget && !c.IsDead() && c.Kind == combat.KindNPC {
                return c.Name
            }
        }
    }
    // Fall back to first living NPC.
    for _, c := range cbt.Combatants {
        if c.Kind == combat.KindNPC && !c.IsDead() {
            return c.Name
        }
    }
    return ""
}

// findReloadableWeaponLocked returns the weapon slot ID of the first equipped
// ranged weapon that has zero ammo, or "" if none found.
//
// Precondition: combatMu must be held; sess must not be nil.
// Postcondition: Returns a non-empty slot ID or "".
func (h *CombatHandler) findReloadableWeaponLocked(sess *session.PlayerSession) string {
    if sess.LoadoutSet == nil {
        return ""
    }
    active := sess.LoadoutSet.Active()
    if active == nil {
        return ""
    }
    for slotID, slot := range active.Slots {
        if slot.ItemDefID != "" && slot.AmmoCount == 0 && h.isRangedWeapon(slot.ItemDefID) {
            return slotID
        }
    }
    return ""
}

// findBurstCapableWeaponLocked returns the slot ID of the first equipped weapon
// that supports burst fire, or "" if none found.
//
// Precondition: combatMu must be held; sess must not be nil.
// Postcondition: Returns a non-empty slot ID or "".
func (h *CombatHandler) findBurstCapableWeaponLocked(sess *session.PlayerSession) string {
    if sess.LoadoutSet == nil {
        return ""
    }
    active := sess.LoadoutSet.Active()
    if active == nil {
        return ""
    }
    for slotID, slot := range active.Slots {
        if slot.ItemDefID != "" && h.isBurstCapable(slot.ItemDefID) {
            return slotID
        }
    }
    return ""
}

// findAutoCapableWeaponLocked returns the slot ID of the first equipped weapon
// that supports automatic fire, or "" if none found.
func (h *CombatHandler) findAutoCapableWeaponLocked(sess *session.PlayerSession) string {
    if sess.LoadoutSet == nil {
        return ""
    }
    active := sess.LoadoutSet.Active()
    if active == nil {
        return ""
    }
    for slotID, slot := range active.Slots {
        if slot.ItemDefID != "" && h.isAutoCapable(slot.ItemDefID) {
            return slotID
        }
    }
    return ""
}

// findExplosiveLocked returns the item def ID of the first explosive in the
// player's inventory, or "" if none found.
func (h *CombatHandler) findExplosiveLocked(sess *session.PlayerSession) string {
    if sess.Backpack == nil {
        return ""
    }
    for _, item := range sess.Backpack.Items() {
        if h.invRegistry != nil && h.isExplosive(item.ItemDefID) {
            return item.ItemDefID
        }
    }
    return ""
}
```

You must also check the existing inventory/weapon helpers (`isRangedWeapon`, `isBurstCapable`, `isAutoCapable`, `isExplosive`) — these may already exist on `CombatHandler`. Search the file for these helpers before adding them. If they don't exist, add simple implementations that use `h.invRegistry.FindItem(itemDefID)` and check `item.WeaponType` or `item.Tags`.

**Step 5: Wire autoQueuePlayersLocked into resolveAndAdvanceLocked**

In `resolveAndAdvanceLocked`, find where `autoQueueNPCsLocked(cbt)` is called for the NEXT round (around line 626). Add the player auto-queue call immediately before it:

```go
h.autoQueuePlayersLocked(cbt)
h.autoQueueNPCsLocked(cbt)
```

**Step 6: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 180s
```
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/combat_default_test.go
git commit -m "feat: auto-queue player default combat action each round"
```

---

### Task 4: Full test suite + deploy

**Step 1: Run full test suite**

```bash
go test ./... -timeout 180s
```
Expected: all PASS

**Step 2: Update FEATURES.md**

Mark `Default combat actions` as `[x]` in `docs/requirements/FEATURES.md`.

**Step 3: Deploy**

```bash
make k8s-redeploy
```

**Step 4: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark default combat actions complete"
```

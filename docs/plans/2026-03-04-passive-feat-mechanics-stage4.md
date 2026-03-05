# Passive Feat Mechanics — Stage 4 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Implement mechanical effects for `sucker_punch`, `predators_eye`, `street_brawler`, and `zone_awareness` passive class features, plus the `on_passive_feat_check` Lua hook and favored target type selection at character creation.

**Architecture:** `PlayerSession` gains `PassiveFeats map[string]bool` and `FavoredTarget string` populated at login. `ResolveRound` gains passive bonus evaluation after the existing `AttackBonus`/`ACBonus` application. The `flat_footed` condition is applied to NPCs at combat start and cleared after their first action. A new DB table and migration store the favored target type.

**Tech Stack:** Go, `pgregory.net/rapid`, PostgreSQL, existing `condition.ActiveSet`/`condition.Registry`, `scripting.Manager.CallHook`

---

## Background & Key Facts

- **`Combatant`** — `internal/game/combat/combat.go:41`. Fields: `ID`, `Kind`, `Name`, `MaxHP`, `CurrentHP`, `AC`, `Level`, `StrMod`, `DexMod`, `Initiative`, `Dead`, `Loadout`. No passive feat field — passive checks use the player session looked up by `Combatant.ID`.
- **`PlayerSession`** — `internal/game/session/manager.go:13`. Current fields end at `Conditions *condition.ActiveSet`. **Missing:** `PassiveFeats map[string]bool`, `FavoredTarget string`.
- **`ResolveRound`** — `internal/game/combat/round.go:142`. `AttackBonus`/`ACBonus` applied at lines 178-182 (ActionAttack) and 206-210 (ActionStrike first hit). `DamageBonus` from conditions is NOT called. The hook `hookDamageRoll` fires after `r.EffectiveDamage()` at line 186.
- **`ResolveRound` passive feat access** — `ResolveRound(cbt *Combat, src Source, targetUpdater func(...))`. The `Combat` struct has a `sessionGetter func(uid string) (*session.PlayerSession, bool)` field (to be added in Task 1) for passive feat lookup.
- **`condition.DamageBonus`** — `internal/game/condition/modifiers.go:50`. Sums `Def.DamageBonus * stacks` for all active conditions. Not yet called in `round.go`.
- **`CombatHandler.Flee`** — `internal/gameserver/combat_handler.go:243`. On success (`playerTotal > npcTotal`), calls `h.removeCombatant(cbt, uid)` at line 275. Street brawler AoO fires here for each remaining player.
- **`handleMove`** — `internal/gameserver/grpc_service.go:716`. Calls `s.worldH.MoveWithContext(uid, dir)`. Difficult terrain check fires after move succeeds, before broadcasting departure/arrival.
- **Migration convention** — `migrations/{NNN}_{snake_case}.{up|down}.sql`. Current highest: `013`. Next: `014`.
- **Condition YAML fields** — `id`, `name`, `description`, `duration_type`, `max_stacks`, `attack_penalty`, `ac_penalty`, `speed_penalty`, `restrict_actions`, `lua_on_apply`, `lua_on_remove`, `lua_on_tick`.
- **`on_skill_check` hook pattern** — `s.scriptMgr.CallHook(zoneID, "on_skill_check", lua.LString(uid), ...)` — see `grpc_service.go:923`.
- **`Combatant.Kind`** — `KindPlayer` or `KindNPC`. `KindNPC` combatants always start as `flat_footed`.
- **`Combat.Conditions`** — `map[string]*condition.ActiveSet` keyed by combatant ID. Used for `AttackBonus`/`ACBonus`. Used by `flat_footed` too.
- **`Combat` struct location** — NOT in `combat.go`. Search for `type Combat struct` in `internal/game/combat/`. It is likely `engine.go` or `combat_state.go`.
- **Session loading order** (grpc_service.go ~340-486): weapon presets → equipment → inventory → starting kit → skill backfill → skill load → `sess.Conditions = condition.NewActiveSet()`. Passive feat cache and favored target load goes **after** `Conditions` init.

---

## Task 1: Wire `DamageBonus` into `ResolveRound`

**Files:**
- Modify: `internal/game/combat/round.go`
- Test: `internal/game/combat/round_test.go` (create or add)

**Context:** `condition.DamageBonus(s *ActiveSet)` exists but is never called. It must be added to the damage calculation in `ResolveRound` for both `ActionAttack` and `ActionStrike`. The bonus is added AFTER `r.EffectiveDamage()` and BEFORE `hookDamageRoll`.

**Step 1: Write failing test**

Find or create `internal/game/combat/round_test.go`. Add:

```go
func TestResolveRound_ConditionDamageBonusApplied(t *testing.T) {
    // Build a combat with one player (actor) and one NPC (target).
    // Apply a condition with DamageBonus=5 to actor's condition set in cbt.Conditions.
    // Use a fixed dice source that always rolls max (guarantees a hit).
    // Resolve one round.
    // Assert the damage dealt to target.CurrentHP reflects the +5 bonus.
}

func TestProperty_ResolveRound_DamageBonusNeverNegatesHit(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        bonus := rapid.IntRange(0, 20).Draw(rt, "bonus")
        // Build combat with condition DamageBonus=bonus, guaranteed hit
        // Assert finalHP <= initialHP (damage is non-negative)
    })
}
```

Study existing `round_test.go` (if it exists) for the test setup pattern — particularly how `Combat`, `Combatant`, and `Source` are constructed.

Run:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test -run "TestResolveRound_ConditionDamageBonus|TestProperty_ResolveRound_DamageBonus" -v ./internal/game/combat/...
```
Expected: FAIL — condition bonus not applied so HP delta is wrong.

**Step 2: Add `DamageBonus` to `ActionAttack` in `ResolveRound`**

In `internal/game/combat/round.go`, in the `ActionAttack` case, after line 185 (`dmg := r.EffectiveDamage()`), add:

```go
dmg += condition.DamageBonus(cbt.Conditions[actor.ID])
```

**Step 3: Add `DamageBonus` to `ActionStrike` in `ResolveRound`**

Find the `ActionStrike` case. Apply the same pattern to both the first and second strike damage calculations.

**Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test -run "TestResolveRound_ConditionDamageBonus|TestProperty_ResolveRound_DamageBonus" -v ./internal/game/combat/...
```
Expected: PASS

**Step 5: Run full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/...
```

**Step 6: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/round_test.go
git commit -m "feat: wire condition.DamageBonus into ResolveRound attack and strike damage"
```

---

## Task 2: Add `PassiveFeats` and `FavoredTarget` to `PlayerSession`; populate at login

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_login_test.go` (or nearest login test file)

**Context:** Combat passive checks need per-player passive feat IDs without a DB query per attack. `PlayerSession` gains two new fields populated at login from the existing `characterClassFeaturesRepo` and a new `characterFavoredTargetRepo`.

NOTE: Do NOT add a DB repo for favored target in this task — that comes in Task 3. In this task, load class features into `PassiveFeats` only.

**Step 1: Add fields to `PlayerSession`**

In `internal/game/session/manager.go`, add after `Conditions`:

```go
// PassiveFeats holds the IDs of all passive class features and feats for this character.
// Populated at login; used by combat passive checks without additional DB queries.
PassiveFeats map[string]bool

// FavoredTarget is the NPC type favored by the predators_eye class feature.
// Empty string means unset (either feature not held, or not yet chosen).
FavoredTarget string
```

**Step 2: Write failing test**

In the login test file, add:

```go
func TestSession_PassiveFeatsPopulatedAtLogin(t *testing.T) {
    // Build a server with a mock characterClassFeaturesRepo that returns
    // ["sucker_punch", "zone_awareness"] for characterID=1
    // Run session initialization
    // Assert sess.PassiveFeats["sucker_punch"] == true
    // Assert sess.PassiveFeats["zone_awareness"] == true
    // Assert len(sess.PassiveFeats) == 2
}
```

Run to verify FAIL.

**Step 3: Populate `PassiveFeats` at login**

In `internal/gameserver/grpc_service.go`, in the session initialization block (after `sess.Conditions = condition.NewActiveSet()`), add:

```go
// Populate passive feat cache from class features.
sess.PassiveFeats = make(map[string]bool)
if characterID > 0 && s.characterClassFeaturesRepo != nil {
    cfIDs, cfErr := s.characterClassFeaturesRepo.GetAll(stream.Context(), characterID)
    if cfErr != nil {
        s.logger.Warn("loading class features for passive cache", zap.Error(cfErr))
    } else {
        for _, id := range cfIDs {
            cf, ok := s.classFeatureRegistry.Get(id)
            if ok && !cf.Active {
                sess.PassiveFeats[id] = true
            }
        }
    }
}
```

Find the `classFeatureRegistry` field on `GameServiceServer` — it may be named differently. Search for how `handleClassFeatures` looks up features to find the correct field name.

**Step 4: Run tests, then full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test -run "TestSession_PassiveFeatsPopulatedAtLogin" -v ./internal/gameserver/...
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/...
```

**Step 5: Commit**

```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_login_test.go
git commit -m "feat: add PassiveFeats and FavoredTarget to PlayerSession; populate PassiveFeats at login"
```

---

## Task 3: `flat_footed` condition + DB migration for favored target

**Files:**
- Create: `content/conditions/flat_footed.yaml`
- Create: `migrations/014_character_favored_target.up.sql`
- Create: `migrations/014_character_favored_target.down.sql`
- Create: `internal/storage/postgres/character_favored_target.go`

**Context:** `flat_footed` is a new condition with `duration_type: rounds` applied to NPCs at combat start. The `character_favored_target` table stores one row per character.

**Step 1: Create `flat_footed.yaml`**

```yaml
id: flat_footed
name: Flat-Footed
description: |
  This combatant has not yet acted in combat and is unprepared for incoming attacks.
  Attackers with Sucker Punch deal +1d6 bonus damage to flat-footed targets.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Create DB migration**

`migrations/014_character_favored_target.up.sql`:
```sql
CREATE TABLE character_favored_target (
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    target_type   TEXT   NOT NULL,
    PRIMARY KEY (character_id)
);
```

`migrations/014_character_favored_target.down.sql`:
```sql
DROP TABLE IF EXISTS character_favored_target;
```

**Step 3: Create `character_favored_target.go` repo**

Create `internal/storage/postgres/character_favored_target.go`:

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
)

// CharacterFavoredTargetRepo persists and retrieves the favored NPC target type per character.
type CharacterFavoredTargetRepo struct {
    db *sql.DB
}

// NewCharacterFavoredTargetRepo constructs a CharacterFavoredTargetRepo.
// Precondition: db must not be nil.
func NewCharacterFavoredTargetRepo(db *sql.DB) *CharacterFavoredTargetRepo {
    return &CharacterFavoredTargetRepo{db: db}
}

// Get returns the favored target type for characterID, or "" if none is set.
// Precondition: characterID > 0.
// Postcondition: returns ("", nil) when no row exists.
func (r *CharacterFavoredTargetRepo) Get(ctx context.Context, characterID int64) (string, error) {
    var t string
    err := r.db.QueryRowContext(ctx,
        `SELECT target_type FROM character_favored_target WHERE character_id = $1`, characterID,
    ).Scan(&t)
    if errors.Is(err, sql.ErrNoRows) {
        return "", nil
    }
    return t, err
}

// Set upserts the favored target type for characterID.
// Precondition: characterID > 0; targetType must be one of "human","robot","animal","mutant".
// Postcondition: exactly one row exists for characterID with target_type = targetType.
func (r *CharacterFavoredTargetRepo) Set(ctx context.Context, characterID int64, targetType string) error {
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO character_favored_target (character_id, target_type)
         VALUES ($1, $2)
         ON CONFLICT (character_id) DO UPDATE SET target_type = EXCLUDED.target_type`,
        characterID, targetType,
    )
    return err
}
```

**Step 4: Write unit tests for the repo**

In a new file `internal/storage/postgres/character_favored_target_test.go`, use table-driven tests with a real DB connection (follow the pattern of other postgres tests in the package — they use `testDB` or similar setup). If a DB is not available in tests, write pure mock tests for the interface.

**Step 5: Run migration locally**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go run ./cmd/migrate/... up 2>&1 | tail -5
```
(Or whatever the migration run command is — check `Makefile` for `migrate` target.)

**Step 6: Commit**

```bash
git add content/conditions/flat_footed.yaml migrations/ internal/storage/postgres/character_favored_target.go internal/storage/postgres/character_favored_target_test.go
git commit -m "feat: flat_footed condition YAML; character_favored_target DB table and repo"
```

---

## Task 4: Load `FavoredTarget` at login; prompt at creation and for existing characters

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service.go` (character creation handler)
- Test: `internal/gameserver/grpc_service_login_test.go`

**Context:** Load favored target at login into `sess.FavoredTarget`. If `predators_eye` is in `PassiveFeats` and `FavoredTarget == ""`, prompt the player immediately.

Valid types: `human`, `robot`, `animal`, `mutant`.

**Step 1: Wire `CharacterFavoredTargetRepo` into `GameServiceServer`**

Add field to `GameServiceServer` struct:
```go
favoredTargetRepo *postgres.CharacterFavoredTargetRepo
```

Wire it in `NewGameServiceServer` (or `main.go`).

**Step 2: Load `FavoredTarget` at login**

In `grpc_service.go`, after `PassiveFeats` population:

```go
// Load favored target for predators_eye.
if characterID > 0 && s.favoredTargetRepo != nil {
    ft, ftErr := s.favoredTargetRepo.Get(stream.Context(), characterID)
    if ftErr != nil {
        s.logger.Warn("loading favored target", zap.Error(ftErr))
    } else {
        sess.FavoredTarget = ft
    }
}
```

**Step 3: Prompt if predators_eye is held but no target set**

After loading, if `sess.PassiveFeats["predators_eye"] && sess.FavoredTarget == ""`, send the player a selection prompt:

```go
if sess.PassiveFeats["predators_eye"] && sess.FavoredTarget == "" {
    // Send a list prompt to the player:
    // "Your Predator's Eye feat requires a favored target type."
    // "Choose one: 1) human  2) robot  3) animal  4) mutant"
    // Read a reply (look at how character creation prompts work in the session handler)
    // Persist via s.favoredTargetRepo.Set(...)
    // Set sess.FavoredTarget
}
```

Study the character creation flow in `grpc_service.go` to find the pattern for sending a prompt and reading a selection.

**Step 4: Prompt during character creation**

In the character creation section of `grpc_service.go`, after class features are assigned, check if `predators_eye` is among the assigned features. If so, present the same prompt.

**Step 5: Write failing tests**

```go
func TestSession_FavoredTargetLoadedAtLogin(t *testing.T) {
    // mock favoredTargetRepo returns "robot" for characterID=1
    // run session init
    // assert sess.FavoredTarget == "robot"
}

func TestSession_FavoredTargetPromptedWhenMissing(t *testing.T) {
    // mock favoredTargetRepo returns "" for characterID=1
    // sess.PassiveFeats["predators_eye"] = true
    // assert player receives a prompt message
    // assert favoredTargetRepo.Set was called after selection
}
```

**Step 6: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... 2>&1 | tail -10
```

**Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_login_test.go
git commit -m "feat: load FavoredTarget at login; prompt predators_eye characters to choose favored type"
```

---

## Task 5: `sucker_punch` passive in `ResolveRound`

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/combat.go` or wherever `Combat` struct is defined — add `sessionGetter`
- Test: `internal/game/combat/round_test.go`

**Context:** `sucker_punch` adds +1d6 damage when the attacker has the passive AND the target has the `flat_footed` condition. The `flat_footed` condition is in `cbt.Conditions[target.ID]`.

`ResolveRound` needs access to passive feat data. Add a `sessionGetter func(uid string) (*session.PlayerSession, bool)` field to the `Combat` struct. Populated by `CombatHandler` when starting combat.

**Step 1: Add `sessionGetter` to `Combat` struct**

Find `type Combat struct` (likely in `internal/game/combat/engine.go` or `combat_state.go`). Add:

```go
sessionGetter func(uid string) (*session.PlayerSession, bool)
```

Add a setter `SetSessionGetter(fn func(uid string) (*session.PlayerSession, bool))` following the pattern of other setters.

**Step 2: Apply `flat_footed` condition to NPCs at combat start**

Find the function that initializes a new `Combat` (likely in `CombatHandler.startCombatLocked`). After adding NPC combatants to the combat, for each NPC combatant:

```go
flatFootedDef, ok := s.condRegistry.Get("flat_footed")
if ok {
    if cbt.Conditions[npc.ID] == nil {
        cbt.Conditions[npc.ID] = condition.NewActiveSet()
    }
    _ = cbt.Conditions[npc.ID].Apply(npc.ID, flatFootedDef, 1, 1) // stacks=1, duration=1 round
}
```

**Step 3: Clear `flat_footed` after NPC's first action**

In `ResolveRound`, after an NPC combatant's action resolves (inside the `for _, action := range q.QueuedActions()` loop, after appending the event), if `actor.Kind == KindNPC`:

```go
if actor.Kind == KindNPC {
    if cbt.Conditions[actor.ID] != nil {
        cbt.Conditions[actor.ID].Remove("flat_footed")
    }
}
```

**Step 4: Write failing test**

```go
func TestResolveRound_SuckerPunch_FlatFooted(t *testing.T) {
    // Build combat with:
    //   - player actor with PassiveFeats["sucker_punch"] = true
    //   - NPC target with flat_footed condition in cbt.Conditions[npc.ID]
    //   - sessionGetter returning a session with PassiveFeats["sucker_punch"]=true
    //   - fixed dice source (all rolls max)
    // Resolve one round
    // Assert damage to NPC includes +1d6 (at max dice, +6)
}

func TestResolveRound_SuckerPunch_NotFlatFooted_NoBonus(t *testing.T) {
    // Same but no flat_footed on target
    // Assert damage does NOT include the +6 bonus
}

func TestProperty_SuckerPunch_DamageNonNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        hasFeat := rapid.Bool().Draw(rt, "hasFeat")
        isFlatFooted := rapid.Bool().Draw(rt, "isFlatFooted")
        // Assert target.CurrentHP never goes above initialHP (damage is non-negative)
    })
}
```

Run to verify FAIL.

**Step 5: Implement `sucker_punch` bonus in `ResolveRound`**

In `ResolveRound`, in the `ActionAttack` case, after `dmg += condition.DamageBonus(cbt.Conditions[actor.ID])` (added in Task 1) and before `hookDamageRoll`:

```go
// sucker_punch: +1d6 damage vs flat-footed targets.
if actor.Kind == KindPlayer && cbt.sessionGetter != nil {
    if ps, ok := cbt.sessionGetter(actor.ID); ok && ps.PassiveFeats["sucker_punch"] {
        if cbt.Conditions[target.ID] != nil && cbt.Conditions[target.ID].Has("flat_footed") {
            if dmg > 0 { // only bonus damage on a hit
                bonus, _ := src.RollExpr("1d6")
                dmg += bonus.Total()
            }
        }
    }
}
```

Apply the same pattern to `ActionStrike`.

**Step 6: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/... ./internal/gameserver/...
```

**Step 7: Commit**

```bash
git add internal/game/combat/ internal/gameserver/combat_handler.go
git commit -m "feat: sucker_punch passive +1d6 vs flat-footed; apply flat_footed to NPCs at combat start"
```

---

## Task 6: `predators_eye` passive in `ResolveRound`

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/combat.go` (or wherever `Combatant` is defined) — add `NPCType string`
- Test: `internal/game/combat/round_test.go`

**Context:** `predators_eye` adds +1d8 precision damage when attacker has the feat AND `target.NPCType == sess.FavoredTarget`. `Combatant` needs an `NPCType string` field populated when NPC combatants are added to combat.

**Step 1: Add `NPCType string` to `Combatant`**

In `internal/game/combat/combat.go` (the `Combatant` struct at line 41):

```go
// NPCType is the category of this combatant for predators_eye matching.
// Empty string for player combatants.
NPCType string
```

**Step 2: Populate `NPCType` when adding NPC combatants**

Find where NPC `Combatant` structs are constructed in `CombatHandler.startCombatLocked`. Set `NPCType` from the NPC definition's `Type` or `Category` field. Read the NPC model to find the correct field.

**Step 3: Write failing test**

```go
func TestResolveRound_PredatorsEye_MatchingType(t *testing.T) {
    // Player actor: PassiveFeats["predators_eye"]=true, FavoredTarget="robot"
    // NPC target: NPCType="robot"
    // Fixed dice (max rolls)
    // Assert damage includes +1d8 (at max, +8)
}

func TestResolveRound_PredatorsEye_NonMatchingType_NoBonus(t *testing.T) {
    // NPC target: NPCType="animal" — FavoredTarget="robot"
    // Assert no +1d8 bonus
}

func TestProperty_PredatorsEye_DamageNonNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        favored := rapid.SampledFrom([]string{"human","robot","animal","mutant"}).Draw(rt, "favored")
        npcType := rapid.SampledFrom([]string{"human","robot","animal","mutant"}).Draw(rt, "npcType")
        // Assert finalHP <= initialHP
    })
}
```

Run to verify FAIL.

**Step 4: Implement `predators_eye` in `ResolveRound`**

In the `ActionAttack` case, after `sucker_punch` check, add:

```go
// predators_eye: +1d8 precision damage vs favored target type.
if actor.Kind == KindPlayer && cbt.sessionGetter != nil {
    if ps, ok := cbt.sessionGetter(actor.ID); ok && ps.PassiveFeats["predators_eye"] {
        if ps.FavoredTarget != "" && target.NPCType == ps.FavoredTarget && dmg > 0 {
            bonus, _ := src.RollExpr("1d8")
            dmg += bonus.Total()
        }
    }
}
```

Apply same to `ActionStrike`.

**Step 5: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/... ./internal/gameserver/...
```

**Step 6: Commit**

```bash
git add internal/game/combat/ internal/gameserver/
git commit -m "feat: predators_eye passive +1d8 vs favored NPC type"
```

---

## Task 7: `street_brawler` AoO on flee

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Test: `internal/gameserver/combat_handler_test.go`

**Context:** When a player successfully flees (`playerTotal > npcTotal` in `CombatHandler.Flee`), before removing the combatant, check each REMAINING player in combat. If a remaining player has `street_brawler` in `PassiveFeats`, fire one free `ResolveAttack` against the fleeing player and broadcast the result.

**Step 1: Write failing test**

```go
func TestCombatHandler_StreetBrawler_AoO_OnFlee(t *testing.T) {
    // Set up a combat with:
    //   - player A (fleeing)
    //   - player B with PassiveFeats["street_brawler"]=true (staying)
    // Player A flees successfully
    // Assert that a combat event for an attack from player B against player A
    //   is included in the returned events
}

func TestCombatHandler_StreetBrawler_AoO_NotTriggeredWhenFleeFails(t *testing.T) {
    // Flee roll fails for player A
    // Assert NO AoO event from player B
}
```

Run to verify FAIL.

**Step 2: Implement AoO in `Flee`**

In `CombatHandler.Flee`, in the success branch after `h.removeCombatant(cbt, uid)`, BEFORE the combat-end check, add:

```go
// street_brawler: remaining players with this passive get a free attack vs the fleeing player.
fleeCbt := h.findCombatant(cbt, uid) // may already be removed; use the saved combatant reference
for _, other := range cbt.Combatants {
    if other.ID == uid || other.IsDead() || other.Kind != KindPlayer {
        continue
    }
    otherSess, ok := h.sessions.GetPlayer(other.ID)
    if !ok || !otherSess.PassiveFeats["street_brawler"] {
        continue
    }
    // Free attack vs fleeing player using existing dice source
    aooResult := combat.ResolveAttack(other, playerCbt, h.diceSrc)
    aooNarrative := fmt.Sprintf("%s lashes out at the fleeing %s: %s (total %d).",
        other.Name, playerCbt.Name, aooResult.Outcome, aooResult.AttackTotal)
    events = append(events, &gamev1.CombatEvent{
        Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
        Attacker:  other.Name,
        Target:    playerCbt.Name,
        Narrative: aooNarrative,
    })
}
```

Note: `playerCbt` should be captured before `h.removeCombatant`. Adjust variable names to match the actual code in `Flee`.

**Step 3: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/...
```

**Step 4: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_test.go
git commit -m "feat: street_brawler passive — attack of opportunity when enemy flees"
```

---

## Task 8: `zone_awareness` — difficult terrain message in movement handler

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (`handleMove`)
- Test: `internal/gameserver/grpc_service_test.go`

**Context:** After a successful move, check if the new room has `Properties["terrain"] == "difficult"`. If so AND the player lacks `zone_awareness`, send a flavor message. Player with `zone_awareness` gets no message.

**Step 1: Write failing test**

```go
func TestHandleMove_DifficultTerrain_MessageSentWithoutFeat(t *testing.T) {
    // Room has Properties["terrain"] = "difficult"
    // Player session has PassiveFeats["zone_awareness"] = false
    // Call handleMove
    // Assert returned events include a narrative about difficult terrain
}

func TestHandleMove_DifficultTerrain_NoMessageWithFeat(t *testing.T) {
    // Same room, but player has PassiveFeats["zone_awareness"] = true
    // Assert NO difficult terrain narrative
}

func TestHandleMove_NoDifficultTerrain_NoMessage(t *testing.T) {
    // Room has no terrain property
    // Assert NO difficult terrain narrative regardless of feat
}
```

Run to verify FAIL.

**Step 2: Add terrain check to `handleMove`**

In `internal/gameserver/grpc_service.go`, in `handleMove` (line 716), after the arrival broadcast (after line ~741), add:

```go
// zone_awareness: check for difficult terrain in the destination room.
if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
    if newRoom.Properties["terrain"] == "difficult" {
        sess, sOk := s.sessions.GetPlayer(uid)
        if sOk && !sess.PassiveFeats["zone_awareness"] {
            // Send a message to the player about difficult terrain.
            // Use the existing pattern for pushing a message event to a single player.
            s.sendPlayerMessage(uid, "The difficult terrain slows your movement.")
        }
    }
}
```

Find the correct method to send a single-player message — look at how other commands push narrative text to a specific player.

**Step 3: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/...
```

**Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat: zone_awareness passive — suppress difficult terrain message for aware players"
```

---

## Task 9: `on_passive_feat_check` Lua hook

**Files:**
- Modify: `internal/game/combat/round.go`
- Test: `internal/game/combat/round_test.go`

**Context:** After each passive feat bonus is calculated (both `sucker_punch` and `predators_eye`), fire `on_passive_feat_check(uid, feat_id, context_table)` where `context_table` has keys: `target_uid`, `damage_bonus`, `outcome` ("met" or "not_met"), `target_type`. The hook can return an integer to override `damage_bonus`, or `nil` to accept the default.

Add a new `hookPassiveFeatCheck` helper in `round.go` following the pattern of `hookAttackRoll` and `hookDamageRoll`.

**Step 1: Write failing test**

```go
func TestHookPassiveFeatCheck_OverridesDamageBonus(t *testing.T) {
    // Build a scriptMgr mock that returns 99 for "on_passive_feat_check"
    // sucker_punch fires, normally would add +1d6
    // Assert damage includes 99 instead
}

func TestHookPassiveFeatCheck_NilReturnKeepsDefault(t *testing.T) {
    // scriptMgr returns nil
    // Assert original bonus is used
}
```

Run to verify FAIL.

**Step 2: Add `hookPassiveFeatCheck` helper**

In `internal/game/combat/round.go`, add:

```go
// hookPassiveFeatCheck invokes the on_passive_feat_check Lua hook and returns
// the (possibly overridden) damage bonus.
// Precondition: actor, target must be non-nil.
// Postcondition: returns bonus unchanged when no hook is defined or hook returns nil/non-number.
func hookPassiveFeatCheck(cbt *Combat, actorID, featID, targetID string, bonus int, met bool) int {
    if cbt.scriptMgr == nil {
        return bonus
    }
    outcome := "not_met"
    if met {
        outcome = "met"
    }
    ctx := cbt.scriptMgr.NewTable()
    ctx.RawSetString("target_uid", lua.LString(targetID))
    ctx.RawSetString("damage_bonus", lua.LNumber(float64(bonus)))
    ctx.RawSetString("outcome", lua.LString(outcome))
    ret, _ := cbt.scriptMgr.CallHook(cbt.zoneID, "on_passive_feat_check",
        lua.LString(actorID),
        lua.LString(featID),
        ctx,
    )
    if n, ok := ret.(lua.LNumber); ok {
        return int(n)
    }
    return bonus
}
```

**Step 3: Wire hook after each passive bonus**

After the `sucker_punch` bonus calculation, call:
```go
suckerBonus = hookPassiveFeatCheck(cbt, actor.ID, "sucker_punch", target.ID, suckerBonus, true)
dmg += suckerBonus
```

After the `predators_eye` bonus calculation:
```go
eyeBonus = hookPassiveFeatCheck(cbt, actor.ID, "predators_eye", target.ID, eyeBonus, true)
dmg += eyeBonus
```

Fire the hook with `met=false` even when the condition is not met (bonus=0), so Lua scripts can observe the check:
```go
_ = hookPassiveFeatCheck(cbt, actor.ID, "sucker_punch", target.ID, 0, false)
```

**Step 4: Run tests + full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/combat/... ./internal/gameserver/...
```

**Step 5: Commit**

```bash
git add internal/game/combat/round.go internal/game/combat/round_test.go
git commit -m "feat: on_passive_feat_check Lua hook after sucker_punch and predators_eye evaluation"
```

---

## Task 10: Sample content, FEATURES.md, deploy

**Files:**
- Modify: `content/class_features.yaml` — update descriptions for `sucker_punch`, `predators_eye`, `zone_awareness`
- Modify: `docs/requirements/FEATURES.md` — mark Stage 4 complete
- Add difficult terrain to one room in `content/zones/` for testing

**Step 1: Update `sucker_punch` description**

In `content/class_features.yaml`, for `sucker_punch`, update description to match the implemented behavior:
```yaml
description: "Deal +1d6 bonus damage to targets that have not yet acted in combat (flat-footed)."
```

**Step 2: Update `predators_eye` description**

```yaml
description: "Choose a favored target type (human, robot, animal, or mutant). Deal +1d8 precision damage to targets of that type."
```

**Step 3: Add difficult terrain to one room**

Pick a room in an existing zone (e.g., Felony Flats). Add to its properties:
```yaml
properties:
  terrain: difficult
```

**Step 4: Mark Stage 4 complete in FEATURES.md**

Change:
```markdown
  - [ ] **Passive feat/class feature mechanics — Stage 4**
    - [ ] `sucker_punch` — sneak attack damage bonus when attacking from stealth
    - [ ] `street_brawler` — attack of opportunity when enemy leaves threat range
    - [ ] `predators_eye` — bonus to first attack vs unaware target
    - [ ] `zone_awareness` — removes difficult terrain movement penalty
    - [ ] Lua hook `on_passive_feat_check(uid, feat_id, context)` for custom passive logic
```
To:
```markdown
  - [x] **Passive feat/class feature mechanics — Stage 4**
    - [x] `sucker_punch` — +1d6 damage vs flat-footed (unacted) targets
    - [x] `street_brawler` — attack of opportunity when enemy flees combat
    - [x] `predators_eye` — +1d8 precision damage vs favored NPC type (chosen at creation)
    - [x] `zone_awareness` — suppresses difficult terrain flavor message
    - [x] Lua hook `on_passive_feat_check(uid, feat_id, context)` for custom passive logic
```

**Step 5: Build + test**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... && mise exec -- go test ./...
```

**Step 6: Commit**

```bash
git add content/ docs/requirements/FEATURES.md
git commit -m "feat: Stage 4 content updates — descriptions, difficult terrain sample, FEATURES.md"
```

**Step 7: Deploy**

```bash
make helm-upgrade DB_PASSWORD=mud 2>&1 | tail -5
```

Report Helm revision number.

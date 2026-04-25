# Take Cover Action — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the existing `handleTakeCover` to PF2E-aligned semantics: require existing cover, apply a single `taking_cover` condition that adds `+1` cover tier (capped at Greater), expire on move *or* attack, integrate with #247's positional cover model. Replace the legacy tier-named conditions (`lesser_cover` / `standard_cover` / `greater_cover`) with a load-time silent migration.

**Spec:** [docs/superpowers/specs/2026-04-25-take-cover-action.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-take-cover-action.md) (PR [#293](https://github.com/cory-johannsen/mud/pull/293))

**Architecture:** Five small surgical changes. (1) New `ActionTakeCover` action constant with cost 1. (2) `handleTakeCover` migrated to validate against #247's positional cover routine; on success spends AP and applies the `taking_cover` condition. (3) `getEffectiveCoverTier(target, attacker)` lives next to #247's positional routine; the attack resolver calls it instead of reading `Combatant.CoverTier`. (4) Move actions (`ActionStride`, `ActionMoveTraitStride`, `MoveToRequest`, forced movement) and attack actions (Strike / Attack / Throw / FireBurst / FireAutomatic / UseAbility-with-attack-roll / UseTech-with-attack-roll) remove `taking_cover` after they resolve; per the PF2E-strict reading the *first* attack benefits then ends the condition. (5) Load-time migration silently translates legacy tier-named conditions into nothing — positional cover takes over.

**Tech Stack:** Go (`internal/game/combat/`, `internal/gameserver/`), `pgregory.net/rapid` for property tests, telnet, React/TypeScript (`cmd/webclient/ui/src/combat/`).

**Prerequisite:** #247 (Cover bonuses) MUST merge first. The plan calls into #247's positional cover routine; without it, `getEffectiveCoverTier` has no base tier to consult. #259 (bonus types) is a soft dep — emits the cover bonus as `circumstance` per BTYPE-16 contract.

**User confirmation checkpoint:** TKCV-Q1 — does the *first* attack after Take Cover benefit from the elevation (PF2E-strict) or not (simplified)? Plan default is PF2E-strict per spec recommendation.

**Note on spec PR**: Spec is on PR #293, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/combat/action.go` (`ActionTakeCover` constant; `Cost()`) |
| Modify | `internal/game/combat/action_test.go` |
| Create | `internal/game/combat/cover.go` (`getEffectiveCoverTier`) — lives next to #247's positional routine |
| Create | `internal/game/combat/take_cover.go` (apply + expire helpers) |
| Create | `internal/game/combat/take_cover_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestTakeCover_Property/` |
| Modify | `internal/gameserver/grpc_service.go` (`handleTakeCover` — migrate semantics) |
| Modify | `internal/gameserver/grpc_service_test.go` (existing handler tests — update assertions) |
| Modify | `internal/game/combat/round.go` (move + attack expiry hooks) |
| Modify | `internal/game/condition/migration.go` or load path (legacy tier-named condition silent translate) |
| Create | `content/conditions/taking_cover.yaml` |
| Modify | `cmd/webclient/ui/src/combat/CombatActionBar.tsx` (Take Cover button — gated) |
| Modify | `cmd/webclient/ui/src/combat/CombatActionBar.test.tsx` |
| Modify | `internal/frontend/handlers/take_cover_command.go` (existing telnet command — uses new semantics via the migrated handler) |
| Modify | `docs/architecture/combat.md` |

---

### Task 1: `ActionTakeCover` constant + `taking_cover` condition

**Files:**
- Modify: `internal/game/combat/action.go`
- Modify: `internal/game/combat/action_test.go`
- Create: `content/conditions/taking_cover.yaml`

- [ ] **Step 1: Failing tests** (TKCV-1, TKCV-3):

```go
func TestActionTakeCover_CostIsOne(t *testing.T) {
    require.Equal(t, 1, combat.ActionTakeCover.Cost())
}

func TestActionTakeCover_RegisteredInActionTypeSwitch(t *testing.T) {
    // Sanity: the action must round-trip through any switch over ActionType.
    require.Equal(t, "take_cover", combat.ActionTakeCover.String())
}

func TestConditionCatalog_TakingCoverLoads(t *testing.T) {
    cat := condition.LoadCatalog(t, "content/conditions")
    c := cat.ByID("taking_cover")
    require.NotNil(t, c)
    require.Empty(t, c.Effects, "TKCV-3: taking_cover declares no direct stat bonuses; elevation handled in code")
}
```

- [ ] **Step 2: Implement**:

```go
const ActionTakeCover ActionType = ActionLast + 2 // adjacent to ActionMoveTraitStride

func (a ActionType) Cost() int {
    switch a {
    // ... existing ...
    case ActionTakeCover:
        return 1
    }
}
```

- [ ] **Step 3: Author** the condition:

```yaml
# content/conditions/taking_cover.yaml
id: taking_cover
display_name: Taking Cover
description: |
  You crouch behind your cover, increasing its protection by one rank.
  Ends when you move or attack.
stacking: replace_if_higher
effects: []
tags: [cover_elevation]
```

The empty `effects:` is intentional — the elevation is handled by `getEffectiveCoverTier` in code (Task 2). The `tags: [cover_elevation]` is a flag the resolver reads.

---

### Task 2: `getEffectiveCoverTier` integration with #247

**Files:**
- Create: `internal/game/combat/cover.go`
- Modify: `internal/game/combat/round.go` (resolver consults the new helper)

- [ ] **Step 1: Failing tests** (TKCV-7, TKCV-8, TKCV-9):

```go
func TestGetEffectiveCoverTier_NoBaseCoverIgnoresElevation(t *testing.T) {
    cbt, target, attacker := setupOpenField(t)
    target.ApplyCondition("taking_cover", combat.DurationEncounter)
    require.Equal(t, combat.CoverNone, combat.GetEffectiveCoverTier(cbt, target, attacker))
}

func TestGetEffectiveCoverTier_ElevatesLesserToStandard(t *testing.T) {
    cbt, target, attacker := setupWithLesserCoverFor(t)
    target.ApplyCondition("taking_cover", combat.DurationEncounter)
    require.Equal(t, combat.CoverStandard, combat.GetEffectiveCoverTier(cbt, target, attacker))
}

func TestGetEffectiveCoverTier_ElevatesStandardToGreater(t *testing.T) {
    cbt, target, attacker := setupWithStandardCoverFor(t)
    target.ApplyCondition("taking_cover", combat.DurationEncounter)
    require.Equal(t, combat.CoverGreater, combat.GetEffectiveCoverTier(cbt, target, attacker))
}

func TestGetEffectiveCoverTier_GreaterStaysGreater(t *testing.T) {
    cbt, target, attacker := setupWithGreaterCoverFor(t)
    target.ApplyCondition("taking_cover", combat.DurationEncounter)
    require.Equal(t, combat.CoverGreater, combat.GetEffectiveCoverTier(cbt, target, attacker))
}

func TestGetEffectiveCoverTier_NoElevationWithoutCondition(t *testing.T) {
    cbt, target, attacker := setupWithLesserCoverFor(t)
    require.Equal(t, combat.CoverLesser, combat.GetEffectiveCoverTier(cbt, target, attacker))
}

func TestResolver_UsesGetEffectiveCoverTier(t *testing.T) {
    // The resolver MUST call GetEffectiveCoverTier, not read Combatant.CoverTier.
    cbt, target, attacker := setupWithLesserCoverFor(t)
    target.ApplyCondition("taking_cover", combat.DurationEncounter)
    target.LegacyCoverTier = combat.CoverNone // set the legacy field to None
    res := combat.ResolveAttack(cbt, attacker, target, weaponNamed(attacker, "pistol"))
    // The circumstance bonus from cover must reflect Standard tier (Lesser + 1).
    require.Equal(t, +2, contributionFromSource(res, "cover:").Bonus.Value)
}
```

- [ ] **Step 2: Implement** in `cover.go`:

```go
func GetEffectiveCoverTier(cbt *Combat, target, attacker *Combatant) CoverTier {
    base := PositionalCoverTier(cbt, target, attacker) // from #247
    if base == CoverNone {
        return CoverNone // elevation requires existing cover
    }
    if !target.HasActive("taking_cover") {
        return base
    }
    if base == CoverGreater {
        return CoverGreater
    }
    return base + 1 // Lesser → Standard, Standard → Greater
}
```

- [ ] **Step 3: Update the resolver** at the call site that previously read `Combatant.CoverTier`. The cover bonus is emitted as `circumstance` typed via the typed-bonus pipeline (TKCV-9 / BTYPE-16).

- [ ] **Step 4:** The legacy `Combatant.CoverTier` field stays readable for one release cycle (TKCV-8 / TKCV-12); set by `GetEffectiveCoverTier` after each call so any non-migrated reader sees the up-to-date value.

---

### Task 3: `handleTakeCover` migration

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_test.go`
- Create: `internal/game/combat/take_cover.go`
- Create: `internal/game/combat/take_cover_test.go`

- [ ] **Step 1: Failing tests** (TKCV-2, TKCV-17, TKCV-18):

```go
func TestHandleTakeCover_RejectsWithoutCover(t *testing.T) {
    s := setupOpenField(t)
    res, err := s.HandleTakeCover(ctx, &gamev1.TakeCoverRequest{CharacterId: "c1"})
    require.NoError(t, err)
    require.Equal(t, "There's nothing to take cover behind.", res.Narrative)
    require.Equal(t, initialAP, characterAP(s, "c1"), "no AP consumed on rejection")
}

func TestHandleTakeCover_AppliesCondition(t *testing.T) {
    s := setupWithLesserCoverFor(t)
    res, err := s.HandleTakeCover(ctx, &gamev1.TakeCoverRequest{CharacterId: "c1"})
    require.NoError(t, err)
    require.True(t, character(s, "c1").HasActive("taking_cover"))
    require.Contains(t, res.Narrative, "takes cover behind")
    require.Equal(t, initialAP-1, characterAP(s, "c1"))
}

func TestHandleTakeCover_NamesHighestTierSourceInNarrative(t *testing.T) {
    s := setupWithCoverSourceCalled(t, "rusted_dumpster")
    res, _ := s.HandleTakeCover(ctx, &gamev1.TakeCoverRequest{CharacterId: "c1"})
    require.Contains(t, res.Narrative, "rusted dumpster")
}

func TestHandleTakeCover_NoMostRecentAttacker_UsesWorstCase(t *testing.T) {
    // No attacker has fired yet this combat. The validation evaluates against the worst-case
    // tier for any potential attacker (TKCV-2 / TKCV-Q2 fallback) per the spec rule.
    s := setupWithCoverFromOneEnemyButNoAttackYet(t)
    res, err := s.HandleTakeCover(ctx, &gamev1.TakeCoverRequest{CharacterId: "c1"})
    require.NoError(t, err)
    require.True(t, character(s, "c1").HasActive("taking_cover"))
}
```

- [ ] **Step 2: Implement** in `take_cover.go`:

```go
func TakeCoverApply(cbt *Combat, char *Combatant) (sourceName string, ok bool) {
    base, src := highestCoverFor(cbt, char)
    if base == CoverNone {
        return "", false
    }
    char.ApplyCondition("taking_cover", DurationEncounter)
    return src, true
}

// highestCoverFor evaluates positional cover against the most recent attacker;
// when no attacker is on record, falls back to the worst-case across all enemies
// (TKCV-Q2 simplification — evaluate against the *most recent* attacker; if none,
// allow when player has any cover from any cell within MaxRange of any enemy).
func highestCoverFor(cbt *Combat, char *Combatant) (CoverTier, string) {
    if recent := cbt.MostRecentAttackerOf(char); recent != nil {
        return PositionalCoverTier(cbt, char, recent), namedCoverSource(cbt, char, recent)
    }
    var best CoverTier = CoverNone
    var src string
    for _, e := range cbt.Enemies(char) {
        if t := PositionalCoverTier(cbt, char, e); t > best {
            best, src = t, namedCoverSource(cbt, char, e)
        }
    }
    return best, src
}
```

- [ ] **Step 3:** The migrated `handleTakeCover` becomes a thin wrapper:

```go
func (s *Service) HandleTakeCover(ctx context.Context, req *gamev1.TakeCoverRequest) (*gamev1.TakeCoverResponse, error) {
    char := s.LoadCharacter(req.CharacterId)
    sourceName, ok := combat.TakeCoverApply(s.Combat(), char)
    if !ok {
        return &gamev1.TakeCoverResponse{Narrative: "There's nothing to take cover behind."}, nil
    }
    if err := s.SpendAP(char, combat.ActionTakeCover.Cost()); err != nil {
        return nil, err
    }
    return &gamev1.TakeCoverResponse{Narrative: fmt.Sprintf("%s takes cover behind %s.", char.DisplayName, sourceName)}, nil
}
```

- [ ] **Step 4: Drop the legacy** `SetCombatantCover` writes from this handler. The legacy field is now derived (per Task 2) — never written by the new code path.

---

### Task 4: Expiry triggers — move + attack

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/take_cover.go`
- Modify: `internal/game/combat/take_cover_test.go`

- [ ] **Step 1: Checkpoint (TKCV-Q1).** Confirm with user:
  - Option A (PF2E-strict, plan default): first attack after Take Cover benefits from the elevation, then condition ends.
  - Option B (simplified): condition ends *before* the first attack resolves; first attack does not benefit.

  Implementation differs by one line — the `RemoveCondition` call is either after `ResolveAttack` (A) or before (B).

- [ ] **Step 2: Failing tests** (TKCV-4, TKCV-5, TKCV-6):

```go
func TestExpiry_StrideRemovesTakingCover(t *testing.T) {
    cbt, char := setupWithTakingCover(t)
    combat.ExecuteStride(cbt, char, dir: "north")
    require.False(t, char.HasActive("taking_cover"))
    require.Contains(t, cbt.LastNarrative(char), "cover is broken")
}

func TestExpiry_MoveTraitStrideRemovesTakingCover(t *testing.T) { ... }
func TestExpiry_MoveToRemovesTakingCover(t *testing.T) { ... }
func TestExpiry_ForcedMovementRemovesTakingCover(t *testing.T) { ... }

func TestExpiry_FirstAttackBenefitsThenEnds_PF2EStrict(t *testing.T) {
    // TKCV-Q1 default: first attack benefits, then condition ends.
    cbt, target := setupWithTakingCoverAndLesserBaseCover(t)
    char := setupOpponent(t, cbt)
    res1 := combat.ResolveAttack(cbt, char, target, weaponNamed(char, "pistol"))
    require.Equal(t, +2, contributionFromSource(res1, "cover:").Bonus.Value, "first attack: Standard tier (elevated)")

    // Now char attacks again as a separate action; taking_cover is gone, base reverts to Lesser.
    res2 := combat.ResolveAttack(cbt, char, target, weaponNamed(char, "pistol"))
    require.Equal(t, +1, contributionFromSource(res2, "cover:").Bonus.Value, "second attack: base Lesser (elevation gone)")
}

func TestExpiry_AnyAttackActionRemovesTakingCover(t *testing.T) {
    for _, action := range []combat.ActionType{
        combat.ActionAttack, combat.ActionStrike, combat.ActionFireBurst,
        combat.ActionFireAutomatic, combat.ActionThrow,
    } {
        cbt, char := setupWithTakingCover(t)
        target := setupTarget(t, cbt)
        executeAction(cbt, char, target, action)
        require.False(t, char.HasActive("taking_cover"), "action %v must remove taking_cover", action)
    }
}

func TestExpiry_CombatEndRemovesSilently(t *testing.T) {
    cbt, char := setupWithTakingCover(t)
    cbt.End()
    require.False(t, char.HasActive("taking_cover"))
    require.NotContains(t, cbt.LastNarrative(char), "cover is broken")
}
```

- [ ] **Step 3: Implement** the expiry hooks in `round.go`:

```go
// In the move action resolution path (Stride, MoveTraitStride, MoveTo, Forced):
if char.HasActive("taking_cover") {
    char.RemoveCondition("taking_cover")
    cbt.Emit(fmt.Sprintf("%s's cover is broken.", char.DisplayName), TargetCharacter(char))
}

// In the attack action resolution path, AFTER ResolveAttack returns (PF2E-strict):
if char.HasActive("taking_cover") {
    char.RemoveCondition("taking_cover")
    cbt.Emit(fmt.Sprintf("%s's cover is broken.", char.DisplayName), TargetCharacter(char))
}

// In the combat-end path:
if char.HasActive("taking_cover") {
    char.RemoveCondition("taking_cover") // silent
}
```

- [ ] **Step 4:** Property test:

```go
func TestProperty_TakeCover_NeverExceedsGreater(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        baseTier := rapid.SampledFrom([]combat.CoverTier{combat.CoverNone, combat.CoverLesser, combat.CoverStandard, combat.CoverGreater}).Draw(t, "base")
        cbt, target, attacker := setupWithBaseCover(baseTier)
        if rapid.Bool().Draw(t, "applyTakingCover") {
            target.ApplyCondition("taking_cover", combat.DurationEncounter)
        }
        require.LessOrEqual(t, combat.GetEffectiveCoverTier(cbt, target, attacker), combat.CoverGreater)
    })
}

func TestProperty_TakeCover_ResolverNeverReadsLegacyCoverField(t *testing.T) {
    // Compile-time + reflective audit: the resolver MUST call GetEffectiveCoverTier.
    // Implemented as a small reflection check on the Resolver function bytecode (or as a code-grep test).
    require.False(t, resolverReadsLegacyCoverTierField(t))
}
```

---

### Task 5: Legacy condition migration

**Files:**
- Modify: `internal/game/condition/migration.go` (or wherever load-time character migration lives)
- Modify: `internal/game/condition/migration_test.go`

- [ ] **Step 1: Failing tests** (TKCV-10):

```go
func TestLoadCharacter_LegacyTierConditionsTranslatedSilently(t *testing.T) {
    s := setupSession(t)
    char := saveCharacterWithCondition(t, s, "lesser_cover", combat.DurationEncounter)
    s.Reload()
    char = s.LoadCharacter(char.ID)
    require.False(t, char.HasActive("lesser_cover"))
    require.False(t, char.HasActive("taking_cover"))
    // Position-derived cover handles tier going forward.
}

func TestLoadCharacter_NoNarrativeOnSilentMigration(t *testing.T) {
    log := captureLog(t)
    saveCharacterWithCondition(t, s, "standard_cover", combat.DurationEncounter)
    s.Reload()
    require.NotContains(t, log.AllLines(), "translated") // truly silent
}
```

- [ ] **Step 2: Implement** in the load path:

```go
var legacyCoverTierConditions = map[string]bool{
    "lesser_cover":   true,
    "standard_cover": true,
    "greater_cover":  true,
}

func MigrateLegacyConditions(char *Combatant) {
    char.Conditions = slices.DeleteFunc(char.Conditions, func(c ActiveCondition) bool {
        return legacyCoverTierConditions[c.ConditionID]
    })
}
```

Called from the character-load path right before `BuildCombatantEffects`. No narrative emitted.

- [ ] **Step 3:** Per TKCV-11, the legacy YAML files (`lesser_cover.yaml` etc.) stay in `content/conditions/` for one release cycle — removal is a separate follow-on PR.

---

### Task 6: UX — telnet command + web button

**Files:**
- Modify: `internal/frontend/handlers/take_cover_command.go` (existing)
- Modify: `cmd/webclient/ui/src/combat/CombatActionBar.tsx`
- Modify: `cmd/webclient/ui/src/combat/CombatActionBar.test.tsx`

- [ ] **Step 1: Failing telnet tests** (TKCV-13):

```go
func TestTakeCoverCommand_RejectsWithoutCover(t *testing.T) {
    h := newHandlerWithoutCover(t)
    out := h.Run("take cover")
    require.Contains(t, out, "There's nothing to take cover behind.")
}

func TestTakeCoverCommand_SucceedsWithCover(t *testing.T) {
    h := newHandlerWithLesserCover(t)
    out := h.Run("take cover")
    require.Contains(t, out, "takes cover behind")
    require.True(t, h.Character().HasActive("taking_cover"))
}
```

- [ ] **Step 2: Telnet** — already wired to `handleTakeCover`; the command works after Task 3's migration. No new code needed beyond confirming the wire path.

- [ ] **Step 3: Failing web tests** (TKCV-14, TKCV-15):

```ts
test("Take Cover button greyed when player has no cover", () => {
  render(<CombatActionBar character={withoutCover} />);
  const btn = screen.getByRole("button", { name: /take cover/i });
  expect(btn).toBeDisabled();
  expect(btn).toHaveAttribute("title", expect.stringContaining("no cover here"));
});

test("Take Cover button enabled when player has lesser cover", () => {
  render(<CombatActionBar character={withLesserCover} />);
  const btn = screen.getByRole("button", { name: /take cover/i });
  expect(btn).not.toBeDisabled();
});

test("taking_cover badge appears on combat status panel", () => {
  render(<CombatStatusPanel character={withTakingCoverActive} />);
  expect(screen.getByText(/taking cover/i)).toBeVisible();
});

test("badge clears when condition expires", () => {
  const { rerender } = render(<CombatStatusPanel character={withTakingCoverActive} />);
  rerender(<CombatStatusPanel character={withoutTakingCover} />);
  expect(screen.queryByText(/taking cover/i)).toBeNull();
});
```

- [ ] **Step 4: Implement** the button gating. The existing `CombatActionBar` reads the per-character action availability from server state; add a derived `canTakeCover` boolean computed server-side from `GetEffectiveCoverTier > CoverNone`.

---

### Task 7: Architecture documentation update

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Take Cover" section** documenting:
  - The `ActionTakeCover` cost-1 action.
  - The `taking_cover` condition shape and the in-code elevation behaviour.
  - The `GetEffectiveCoverTier` composition rule (`min(Greater, base + 1)`; `None` stays `None`).
  - The expiry triggers (move + attack + combat end).
  - The legacy condition silent migration.
  - The PF2E-strict TKCV-Q1 reading and the alternative.
  - Cross-link to #247, #259, the spec, and this plan.

- [ ] **Step 2:** Verify GitHub markdown preview renders cleanly.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- Telnet smoke test: stand on a cell with no cover, run `take cover`, verify rejection without AP cost; move adjacent to cover, run `take cover`, verify the narrative names the source and the AP drops; an enemy attacks, verify the elevated cover bonus appears in the combat-log breakdown; `stride north`, verify the "cover is broken" narrative; combat ends, verify silent removal.
- Web smoke test: same scenarios; verify the action-bar button gates correctly; verify the `taking cover` badge appears and disappears.

---

## Rollout / Open Questions Resolved at Plan Time

- **TKCV-Q1**: PF2E-strict — first attack benefits, then condition ends. Confirmable at Task 4 checkpoint.
- **TKCV-Q2**: Validation evaluates against the most recent attacker; falls back to worst-case-across-enemies when no attacker on record. Defer perf optimisation.
- **TKCV-Q3**: NPC HTN integration deferred to TKCV-F1.
- **TKCV-Q4**: Legacy tier-named conditions silently translated at load; no DB migration. Future cleanup PR removes the YAML files after one release cycle.
- **TKCV-Q5**: `getEffectiveCoverTier` lives in the `combat` package alongside #247's positional routine; resolver calls it as a leaf function, no circular dep.

## Non-Goals Reaffirmed

Per spec §2.2:

- No positional cover model (#247 owns it).
- No further-improvement-when-already-Greater action.
- No cover-from-cover.
- No darkness / smoke / opacity (#267).
- No cover-related feats (`Diehard`, `Reactive Cover`, etc.).
- No auto-take-cover on entering a position with cover.

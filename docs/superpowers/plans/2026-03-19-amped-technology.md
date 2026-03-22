# Amped Technology Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow players to expend a higher-level spontaneous use slot when activating a spontaneous tech that defines `amped_effects`, firing the amped effects when the slot level meets or exceeds `amped_level`.

**Architecture:** A pure `TechAtSlotLevel` helper in the technology package selects amped vs base effects. A new `handleAmpedUse` function in the gameserver handles the interactive level-selection flow and inlines the activation (bypassing the registry re-fetch in `activateTechWithEffects`). A pre-dispatch intercept in the `Session` handler routes ampable spontaneous UseRequests to `handleAmpedUse` before they reach `dispatch` — the same pattern used by `handleRest` and `handleSelectTech`. All existing paths (`handleUse`, `dispatch`) are unchanged.

**Tech Stack:** Go, gRPC bidirectional streaming, `pgregory.net/rapid` for property-based tests

---

## Implementation Note: Pre-Dispatch Intercept Pattern (REQ-AMP5 deviation)

The spec (REQ-AMP5) says "add `stream` to `handleUse`", but `handleUse` is called from `dispatch` which has no stream access. The correct implementation follows the existing `handleRest` pattern: intercept ampable UseRequests **before** `dispatch` in the `Session` handler (around line 1209 of `grpc_service.go`), where the stream is available. Non-ampable UseRequests fall through to `dispatch → handleUse` unchanged.

This satisfies the **intent** of REQ-AMP5 (interactive level prompting via stream is enabled for ampable spontaneous techs) without modifying `handleUse` or `dispatch`.

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `TechAtSlotLevel` function |
| `internal/game/technology/model_test.go` | Add unit tests for `TechAtSlotLevel` |
| `internal/gameserver/grpc_service.go` | Add `handleAmpedUse`; add pre-dispatch intercept in `Session` handler |
| `internal/gameserver/grpc_service_use_test.go` | New: integration tests for amped use |
| `docs/features/technology.md` | Mark Amped Technology checkbox |

---

## Task 1: `TechAtSlotLevel` — pure helper with tests

**Files:**
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/technology/model_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/technology/model_test.go`:

```go
// REQ-AMP1–4: TechAtSlotLevel selects amped or base effects by slot level.
func TestTechAtSlotLevel(t *testing.T) {
	baseEffects := technology.TieredEffects{
		OnApply: []technology.TechEffect{{Type: technology.EffectUtility, Description: "base"}},
	}
	ampedEffects := technology.TieredEffects{
		OnApply: []technology.TechEffect{{Type: technology.EffectUtility, Description: "amped"}},
	}
	tech := &technology.TechnologyDef{
		ID:           "test_tech",
		Name:         "Test Tech",
		Level:        1,
		AmpedLevel:   3,
		Effects:      baseEffects,
		AmpedEffects: ampedEffects,
	}

	t.Run("below amped level returns original", func(t *testing.T) {
		result := technology.TechAtSlotLevel(tech, 2)
		assert.Same(t, tech, result, "must return original pointer, not a copy")
		assert.Equal(t, "base", result.Effects.OnApply[0].Description)
	})

	t.Run("at amped level returns copy with amped effects", func(t *testing.T) {
		result := technology.TechAtSlotLevel(tech, 3)
		assert.NotSame(t, tech, result, "must return a copy, not the original")
		assert.Equal(t, "amped", result.Effects.OnApply[0].Description)
	})

	t.Run("above amped level returns copy with amped effects", func(t *testing.T) {
		result := technology.TechAtSlotLevel(tech, 5)
		assert.NotSame(t, tech, result)
		assert.Equal(t, "amped", result.Effects.OnApply[0].Description)
	})

	t.Run("zero amped level always returns original", func(t *testing.T) {
		noAmp := &technology.TechnologyDef{
			ID:      "plain",
			Level:   1,
			Effects: baseEffects,
		}
		for _, level := range []int{0, 1, 5, 99} {
			result := technology.TechAtSlotLevel(noAmp, level)
			assert.Same(t, noAmp, result, "level %d: must return original when AmpedLevel==0", level)
		}
	})

	t.Run("original is never mutated", func(t *testing.T) {
		result := technology.TechAtSlotLevel(tech, 3)
		result.Effects = technology.TieredEffects{}
		assert.Equal(t, "base", tech.Effects.OnApply[0].Description, "original must be unchanged")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... -run TestTechAtSlotLevel -v 2>&1 | head -20
```

Expected: compile error — `TechAtSlotLevel` undefined.

- [ ] **Step 3: Implement `TechAtSlotLevel` in `model.go`**

Add at the end of `internal/game/technology/model.go` (after the `Validate` method):

```go
// TechAtSlotLevel returns the tech definition to use when activating at the given slot level.
// When slotLevel >= tech.AmpedLevel and AmpedLevel > 0, returns a shallow copy of tech
// with Effects replaced by AmpedEffects.
// Otherwise returns tech unchanged.
//
// Precondition: tech is non-nil; slotLevel >= 0.
// Postcondition: the original tech is never mutated.
func TechAtSlotLevel(tech *TechnologyDef, slotLevel int) *TechnologyDef {
	if tech.AmpedLevel > 0 && slotLevel >= tech.AmpedLevel {
		copy := *tech
		copy.Effects = tech.AmpedEffects
		return &copy
	}
	return tech
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... -v 2>&1 | tail -20
```

Expected: All tests PASS including `TestTechAtSlotLevel`.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/technology/model.go internal/game/technology/model_test.go
git commit -m "feat: add TechAtSlotLevel helper for amped technology activation"
```

---

## Task 2: `handleAmpedUse` + pre-dispatch intercept + integration tests

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_use_test.go`
- Modify: `docs/features/technology.md`

### Background reading

Before coding, read these sections of `internal/gameserver/grpc_service.go`:

1. **Lines ~1200–1260** — the `Session` handler main loop. Find where `handleRest` is intercepted before `dispatch`. The amped intercept goes in the same location.
2. **Lines 4640–4910** — the full `handleUse` function. Understand how it looks up spontaneous techs (`sess.SpontaneousTechs`), checks the pool (`sess.SpontaneousUsePools`), decrements, and calls `activateTechWithEffects`.
3. **Lines 4990–5020** — `activateTechWithEffects`. The `handleAmpedUse` function must replicate `resolveUseTarget` + target-building + `ResolveTechEffects` to avoid the registry re-fetch.
4. **Lines 5105–5120** — `promptFeatureChoice`. Understand the `*ruleset.FeatureChoices` argument.

### Step-by-step

- [ ] **Step 1: Write failing integration tests**

Create `internal/gameserver/grpc_service_use_test.go`:

```go
package gameserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamev1 "github.com/cory-johannsen/mud/api/proto/gen/game/v1"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)

// helpers — adapt to match how other grpc_service tests build a server and session.
// Read an existing test file (e.g. grpc_service_rest_test.go) to copy the
// minimal server/session setup pattern before implementing these tests.

// REQ-AMP2: slotLevel >= AmpedLevel activates amped effects.
func TestHandleAmpedUse_ExplicitLevel_AmpsAtThreshold(t *testing.T) {
	// Build a minimal server with:
	//   - techRegistry containing a spontaneous tech with AmpedLevel=3 and distinct amped effects
	//   - a session with SpontaneousTechs[1] = ["neural_static"] and SpontaneousUsePools[3]{Remaining:2,Max:2}
	//   - a fakeSpontaneousRepo
	// Call handleAmpedUse (or trigger via Session handler) with level=3
	// Assert the response message reflects amped effects fired (not base effects)
	// Assert pool[3].Remaining was decremented
	t.Skip("implement after reading test setup patterns in grpc_service_rest_test.go")
}

// REQ-AMP2: slotLevel < AmpedLevel activates base effects, but decrements the specified pool.
func TestHandleAmpedUse_ExplicitLevel_BelowThreshold_BaseEffects(t *testing.T) {
	t.Skip("implement after reading test setup patterns")
}

// REQ-AMP8: depleted slot level returns error.
func TestHandleAmpedUse_DepletedSlot_ReturnsError(t *testing.T) {
	t.Skip("implement after reading test setup patterns")
}

// REQ-AMP6: non-integer level token returns error.
func TestHandleAmpedUse_InvalidLevelToken_ReturnsError(t *testing.T) {
	t.Skip("implement after reading test setup patterns")
}

// REQ-AMP7: level below tech's native level returns error.
func TestHandleAmpedUse_LevelBelowNative_ReturnsError(t *testing.T) {
	t.Skip("implement after reading test setup patterns")
}

// REQ-AMP9: zero valid levels returns "No uses remaining at any level."
func TestHandleAmpedUse_NoValidLevels_ReturnsError(t *testing.T) {
	t.Skip("implement after reading test setup patterns")
}
```

**Important:** After writing this skeleton, read `internal/gameserver/grpc_service_rest_test.go` (or another test file in the gameserver package) to understand how the test server and sessions are constructed. Replace each `t.Skip` with a real implementation using that pattern. The tests should fail (not skip) before production code is added.

- [ ] **Step 2: Run tests to verify they fail (not skip)**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleAmpedUse -v 2>&1 | tail -20
```

Expected: Tests FAIL (not SKIP) — `handleAmpedUse` does not exist yet.

- [ ] **Step 3: Implement `handleAmpedUse`**

Add a new method to `GameServiceServer` in `internal/gameserver/grpc_service.go`. Place it near `handleUse` (around line 4940).

```go
// handleAmpedUse activates a spontaneous tech that has amped_effects defined,
// prompting the player to select a slot level when one is not provided in the arg.
// This is called from the Session handler (pre-dispatch) when the UseRequest targets
// a spontaneous tech with AmpedLevel > 0.
//
// Precondition: tech is non-nil, tech.UsageType == UsageTypeSpontaneous, tech.AmpedLevel > 0.
// Postcondition: the selected slot-level pool is decremented; resolved tech effects are applied.
func (s *GameServiceServer) handleAmpedUse(
	uid string,
	req *gamev1.UseRequest,
	stream gamev1.GameService_SessionServer,
) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	abilityID := req.GetFeatId()
	arg := req.GetTarget() // "targetID [level]" or "level" or ""

	// Fetch tech definition.
	techDef, ok := s.techRegistry.Get(abilityID)
	if !ok {
		return messageEvent(fmt.Sprintf("Unknown tech: %s.", abilityID)), nil
	}

	// Find which level this tech lives at in the player's spontaneous pool.
	foundLevel := -1
	levels := make([]int, 0, len(sess.SpontaneousTechs))
	for l := range sess.SpontaneousTechs {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		for _, tid := range sess.SpontaneousTechs[l] {
			if tid == abilityID {
				foundLevel = l
				break
			}
		}
		if foundLevel >= 0 {
			break
		}
	}
	if foundLevel < 0 {
		return messageEvent(fmt.Sprintf("You don't know %s.", abilityID)), nil
	}

	// Parse tokens: disambiguate targetID from slotLevel.
	// REQ-AMP5A: any token that parses as a positive integer is slotLevel;
	// the remaining token(s) form targetID.
	// REQ-AMP6: a token that starts with a digit but fails positive-int parsing
	// (e.g. "5x", "0", "-1", "3.0") is treated as a malformed level → error.
	// NPC/entity names never start with a digit, so this is unambiguous.
	// REQ-AMP5A: two positive-integer tokens → error.
	tokens := strings.Fields(arg)
	targetID := ""
	slotLevel := 0
	intCount := 0
	for _, tok := range tokens {
		if n, err := strconv.Atoi(tok); err == nil && n > 0 {
			intCount++
			slotLevel = n
		} else if len(tok) > 0 && tok[0] >= '0' && tok[0] <= '9' {
			// REQ-AMP6: starts with digit but not a valid positive integer.
			return messageEvent(fmt.Sprintf("Invalid level: %s.", tok)), nil
		} else {
			if targetID != "" {
				targetID += " "
			}
			targetID += tok
		}
	}
	if intCount > 1 {
		return messageEvent("Invalid arguments: specify at most one level."), nil
	}
	if slotLevel != 0 {
		// Validate explicit level.
		if slotLevel < techDef.Level {
			return messageEvent(fmt.Sprintf("Cannot use %s below its native level (%d).", techDef.ID, techDef.Level)), nil
		}
		pool := sess.SpontaneousUsePools[slotLevel]
		if pool.Remaining <= 0 {
			return messageEvent(fmt.Sprintf("No level-%d uses remaining.", slotLevel)), nil
		}
	} else {
		// Prompt: build valid levels from sess.SpontaneousUsePools.
		poolLevels := make([]int, 0, len(sess.SpontaneousUsePools))
		for l := range sess.SpontaneousUsePools {
			poolLevels = append(poolLevels, l)
		}
		sort.Ints(poolLevels)
		var validOptions []string
		var validLevels []int
		for _, l := range poolLevels {
			if l >= techDef.Level && sess.SpontaneousUsePools[l].Remaining > 0 {
				validOptions = append(validOptions, fmt.Sprintf("%d (%d remaining)", l, sess.SpontaneousUsePools[l].Remaining))
				validLevels = append(validLevels, l)
			}
		}
		if len(validLevels) == 0 {
			return messageEvent("No uses remaining at any level."), nil
		}
		if len(validLevels) == 1 {
			slotLevel = validLevels[0]
		} else {
			choices := &ruleset.FeatureChoices{
				Key:     "slot_level",
				Prompt:  "Use at what level?",
				Options: validOptions,
			}
			chosen, err := s.promptFeatureChoice(stream, "slot_level", choices)
			if err != nil {
				return nil, fmt.Errorf("handleAmpedUse: prompt: %w", err)
			}
			// Parse the chosen option (format: "N (M remaining)") — extract N.
			if n, err := strconv.Atoi(strings.Fields(chosen)[0]); err == nil {
				slotLevel = n
			} else {
				return messageEvent("Invalid level selection."), nil
			}
		}
	}

	// Decrement the selected slot-level pool.
	ctx := context.Background()
	if s.spontaneousUsePoolRepo != nil {
		if err := s.spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, slotLevel); err != nil {
			s.logger.Warn("handleAmpedUse: Decrement spontaneous pool failed",
				zap.String("uid", uid),
				zap.String("techID", abilityID),
				zap.Error(err))
		}
	}
	pool := sess.SpontaneousUsePools[slotLevel]
	pool.Remaining--
	sess.SpontaneousUsePools[slotLevel] = pool

	// Resolve tech (amped or base) and activate inline (avoids registry re-fetch).
	resolvedTech := technology.TechAtSlotLevel(techDef, slotLevel)

	target, errMsg := s.resolveUseTarget(uid, targetID, resolvedTech)
	if errMsg != "" {
		return messageEvent(errMsg), nil
	}
	var cbt *combat.Combat
	if s.combatH != nil {
		cbt = s.combatH.ActiveCombatForPlayer(uid)
	}
	var techTargets []*combat.Combatant
	if resolvedTech.Targets == technology.TargetsAllEnemies && cbt != nil { //nolint:gocritic
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC && !c.IsDead() {
				techTargets = append(techTargets, c)
			}
		}
	} else if target != nil {
		techTargets = []*combat.Combatant{target}
	}

	activationLine := fmt.Sprintf("%s activated. Level-%d uses remaining: %d.", resolvedTech.Name, slotLevel, pool.Remaining)
	msgs := ResolveTechEffects(sess, resolvedTech, techTargets, cbt, s.condRegistry, globalRandSrc{}, nil)
	allMsgs := append([]string{activationLine}, msgs...)
	return messageEvent(strings.Join(allMsgs, "\n")), nil
}
```

**Check imports:** `strconv` must be added to the import block in `grpc_service.go` if not already present. Also ensure `technology` package is imported.

- [ ] **Step 4: Add pre-dispatch intercept in `Session` handler**

In `internal/gameserver/grpc_service.go`, find the pre-dispatch intercept block (around line 1209) where `handleRest` is checked. Add the amped-use intercept immediately after the rest intercept block:

```go
// Pre-dispatch: intercept UseRequest for spontaneous techs with AmpedEffects.
if p, ok := msg.Payload.(*gamev1.ClientMessage_UseRequest); ok {
	techID := p.UseRequest.GetFeatId()
	if s.techRegistry != nil {
		if techDef, found := s.techRegistry.Get(techID); found &&
			techDef.UsageType == technology.UsageTypeSpontaneous &&
			techDef.AmpedLevel > 0 {
			resp, err := s.handleAmpedUse(uid, p.UseRequest, stream)
			if err != nil {
				return err
			}
			if resp != nil {
				resp.RequestId = msg.RequestId
				if sendErr := stream.Send(resp); sendErr != nil {
					return sendErr
				}
			}
			continue
		}
	}
}
```

**Important:** The `continue` statement assumes this block is inside the `for` loop that receives messages. Read the surrounding code carefully — if the structure differs (e.g. it's not a `for` loop but uses `goto` or something else), adapt accordingly. The key invariant: if the ampable intercept fires, `dispatch` must NOT also process the same message.

- [ ] **Step 5: Replace test skips with real implementations**

Go back to `grpc_service_use_test.go`. Read `grpc_service_rest_test.go` to understand the test server/session construction pattern. Replace each `t.Skip` with a real test that:
- Builds a `GameServiceServer` with the minimal dependencies (techRegistry, sessionManager, spontaneousUsePoolRepo)
- Inserts a test session with `SpontaneousTechs` and `SpontaneousUsePools` populated
- Calls `handleAmpedUse` (or triggers via a mock stream) with the appropriate `UseRequest`
- Asserts the correct pool was decremented and the correct effects fired

For the prompt test cases (level selection), use a mock stream that returns a predetermined choice.

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleAmpedUse -v 2>&1 | tail -30
```

Expected: All `TestHandleAmpedUse*` tests PASS.

Run full suite:

```bash
mise exec -- go test ./internal/game/technology/... ./internal/gameserver/... 2>&1 | tail -10
```

Expected: All tests PASS.

- [ ] **Step 7: Mark feature checkbox**

In `docs/features/technology.md`, find:

```
      - [ ] Heightened Spells -> Amped Technology — expend a higher-level spontaneous use slot to activate a tech at amped power level; uses `AmpedEffects` defined per tech (Sub-project: Amped Technology, depends on Tech Effect Resolution)
```

Change `- [ ]` to `- [x]`.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_use_test.go \
        docs/features/technology.md
git commit -m "feat: amped technology — use higher-level slot for enhanced spontaneous tech effects"
```

# Amped Technology Design

**Date:** 2026-03-19
**Feature:** Amped Technology — heightened spell analog for spontaneous techs

---

## Goal

Allow players to expend a higher-level spontaneous use slot when activating a spontaneous tech that defines `amped_effects`. When the slot level meets or exceeds the tech's `amped_level`, the amped effects fire instead of the base effects. All other tech usage paths (prepared, innate, hardwired, class features) are unchanged.

---

## Scope

In scope: `TechAtSlotLevel` helper, `use <tech> [target] [level]` syntax extension for spontaneous techs, slot-level prompt when level is omitted on an ampable tech, decrementing the correct slot-level pool, adding `stream` parameter to `handleUse` to enable interactive prompting.

Out of scope: multiple amp tiers (only one `amped_level` / `amped_effects` pair per tech), amping prepared or innate techs, changes to `activateTechWithEffects`, `ResolveTechEffects`, or any repo interface.

---

## Data Model

`TechnologyDef` already carries two fields (no schema changes required):

```go
AmpedLevel  int           `yaml:"amped_level,omitempty"`
AmpedEffects TieredEffects `yaml:"amped_effects,omitempty"`
```

Validation (already enforced): `AmpedLevel > 0` iff `AmpedEffects` is non-empty.

---

## Requirements

### REQ-AMP1
`TechAtSlotLevel(tech *TechnologyDef, slotLevel int) *TechnologyDef` MUST be defined as an exported function in `internal/game/technology/model.go`.

### REQ-AMP2
When `tech.AmpedLevel > 0` and `slotLevel >= tech.AmpedLevel`, `TechAtSlotLevel` MUST return a shallow copy of `tech` with `Effects` replaced by `tech.AmpedEffects`. In all other cases it MUST return `tech` unchanged (no copy).

### REQ-AMP3
`TechAtSlotLevel` MUST NOT mutate the original `tech` argument.

### REQ-AMP4
Unit tests in `internal/game/technology/model_test.go` MUST cover: `slotLevel < tech.AmpedLevel` returns original, `slotLevel == tech.AmpedLevel` returns copy with amped effects, `slotLevel > tech.AmpedLevel` returns copy with amped effects, `tech.AmpedLevel == 0` always returns original regardless of `slotLevel`.

### REQ-AMP5
`handleUse` MUST be extended to accept `stream gamev1.GameService_SessionServer` as a new parameter after the existing parameters `(uid, abilityID, targetID string)`. All existing call sites MUST be updated to pass the stream. This enables interactive level prompting via `promptFeatureChoice`.

### REQ-AMP5A
The spontaneous-tech branch of `handleUse` MUST parse an optional level token from the command arg as an integer `slotLevel`. This parsing MUST only occur when `tech.UsageType == UsageTypeSpontaneous && tech.AmpedLevel > 0`. All other usage paths MUST be unaffected. Token disambiguation: after splitting the arg on whitespace, any token that parses as a positive integer is treated as `slotLevel`; the remaining token (if any) is treated as `targetID`. NPC/entity names are never bare integers, so this is unambiguous. If two integer tokens are present in the arg, `handleUse` MUST return an error event: `"Invalid arguments: specify at most one level."`.

### REQ-AMP6
If the third token is present but cannot be parsed as a positive integer, `handleUse` MUST return an error event: `"Invalid level: <token>."`.

### REQ-AMP7
If `slotLevel < tech.Level`, `handleUse` MUST return an error event: `"Cannot use <tech.ID> below its native level (<tech.Level>)."`.

### REQ-AMP8
If `pool[slotLevel].Remaining == 0`, `handleUse` MUST return an error event: `"No level-<slotLevel> uses remaining."`.

### REQ-AMP9
When `tech.UsageType == UsageTypeSpontaneous && tech.AmpedLevel > 0` and no level token is provided, `handleUse` MUST build the set of valid levels by sorting all keys in `sess.SpontaneousUsePools` in ascending order and filtering to those where `L >= tech.Level` and `pool[L].Remaining > 0`. If this set is empty, `handleUse` MUST return an error event: `"No uses remaining at any level."`. If exactly one valid level exists, `handleUse` MUST auto-select it without prompting. Otherwise, `handleUse` MUST prompt the player using `promptFeatureChoice` (via the `stream` parameter added in REQ-AMP5) with the message `"Use at what level?"` and options formatted as `"<L> (<remaining> remaining)"` in ascending level order.

### REQ-AMP10
After determining `slotLevel`, `handleUse` MUST decrement `pool[slotLevel]` (not `pool[tech.Level]`). The decrement MUST use `spontaneousUsePoolRepo.Decrement(ctx, sess.CharacterID, slotLevel)` and update `sess.SpontaneousUsePools[slotLevel]`.

### REQ-AMP11
`handleUse` MUST call `TechAtSlotLevel(tech, slotLevel)` to obtain `resolvedTech`. Because `activateTechWithEffects` re-fetches the tech definition from the registry by ID, the amped branch MUST NOT call `activateTechWithEffects`. Instead it MUST inline the equivalent activation: resolve the combat target via `s.resolveUseTarget`, build `techTargets`, call `ResolveTechEffects(sess, resolvedTech, techTargets, cbt, s.condRegistry, globalRandSrc{}, nil)` directly, and return the resulting messages as a `messageEvent`. `activateTechWithEffects` and `ResolveTechEffects` signatures MUST NOT change.

### REQ-AMP12
After decrement, the response event MUST include a line `"<tech.Name> activated. Level-<slotLevel> uses remaining: <N>."` prepended to any effect messages, where `<N>` is `sess.SpontaneousUsePools[slotLevel].Remaining` after the decrement.

### REQ-AMP13
`go test ./internal/game/technology/... ./internal/gameserver/...` MUST pass after all changes are applied.

---

## Architecture

### Data Flow

```
handleUse (spontaneous + AmpedLevel > 0 branch only)
  └─ parse slotLevel from integer token in args (or prompt if omitted)
  └─ validate: slotLevel >= tech.Level, pool[slotLevel].Remaining > 0
  └─ resolvedTech := TechAtSlotLevel(tech, slotLevel)
  └─ spontaneousUsePoolRepo.Decrement(ctx, characterID, slotLevel)
  └─ sess.SpontaneousUsePools[slotLevel].Remaining--
  └─ target := s.resolveUseTarget(uid, targetID, resolvedTech)
  └─ msgs := ResolveTechEffects(sess, resolvedTech, techTargets, cbt, condRegistry, src, nil)
  └─ return messageEvent(activationLine + "\n" + strings.Join(msgs, "\n"))
```

### New Function Signature

```go
// TechAtSlotLevel returns the tech to activate at the given slot level.
// When slotLevel >= tech.AmpedLevel and AmpedLevel > 0, returns a shallow
// copy of tech with Effects replaced by AmpedEffects.
// Otherwise returns tech unchanged.
//
// Precondition: tech is non-nil; slotLevel >= 0.
// Postcondition: original tech is never mutated.
func TechAtSlotLevel(tech *TechnologyDef, slotLevel int) *TechnologyDef
```

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `TechAtSlotLevel` |
| `internal/game/technology/model_test.go` | Add unit tests for `TechAtSlotLevel` (REQ-AMP4) |
| `internal/gameserver/grpc_service.go` | Add `stream` param to `handleUse`; extend spontaneous branch: level parsing, prompt, `TechAtSlotLevel`, decrement at `slotLevel`, inline activation via `ResolveTechEffects` |
| `internal/gameserver/grpc_service_test.go` (or new file) | Integration tests: explicit amp, below-threshold base effects, depleted slot error, invalid token error, non-spontaneous ignores level |

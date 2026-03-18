---
name: mud-technology
description: Technology system — innate slots, spontaneous selection, tech effect resolution
type: reference
---

## Trigger

Load this skill when working on:
- Technology YAML content (adding or modifying `content/technologies/**/*.yaml`)
- Technology model types (`internal/game/technology/`)
- Technology assignment at character creation or level-up (`internal/gameserver/technology_assignment.go`)
- Tech effect resolution (`internal/gameserver/tech_effect_resolver.go`)
- The `handleUse`, `handleRest`, or `handleChar` gRPC handlers as they relate to technologies
- Database repositories for any of the four technology usage types

## Responsibility Boundary

The technology system spans three distinct sub-areas:

1. **`internal/game/technology/`** — model and registry (pure data, no side effects)
   - `model.go`: defines `TechnologyDef`, `TieredEffects`, `TechEffect`, and all typed constants (`Tradition`, `UsageType`, `Range`, `Targets`, `EffectType`). Contains `Validate()` for structural correctness.
   - `registry.go`: in-memory `Registry` loaded from YAML at startup. Provides `Get(id)`, `All()`, `ByTradition()`, `ByTraditionAndLevel()`, `ByUsageType()`. Fail-fast `Load(dir)` walks a directory recursively.

2. **`internal/gameserver/technology_assignment.go`** — assigns techs to characters at creation/level-up
   - `AssignTechnologies`: called at character creation; processes hardwired, innate (archetype + region), prepared, and spontaneous grants from `ruleset.Job`, `ruleset.Archetype`, and `ruleset.Region`.
   - `LevelUpTechnologies`: adds incremental grants at level gain; innate grants are archetype-only and not added here.
   - `LoadTechnologies`: called at login to hydrate `PlayerSession` from all four repos.
   - `RearrangePreparedTechs`: full re-selection of prepared slots (e.g., after job respec).
   - `PartitionTechGrants` / `ResolvePendingTechGrants`: deferred pool selection when player choice is required.

3. **`internal/gameserver/tech_effect_resolver.go`** — resolves UseRequest into game effects
   - `ResolveTechEffects`: dispatches by `tech.Resolution` ("save", "attack", "none"), selects the appropriate tier of `TieredEffects`, and applies each `TechEffect` (damage, heal, condition, movement, utility) to the target(s).
   - Does not expend uses — the caller (`handleUse`) has already decremented slots/pools before calling.

## Key Files

| Path | Purpose |
|------|---------|
| `internal/game/technology/model.go` | `TechnologyDef`, `TieredEffects`, `TechEffect`, all constants, `Validate()` |
| `internal/game/technology/registry.go` | `Registry` — load, index, query |
| `internal/gameserver/technology_assignment.go` | Assignment, load, level-up, repo interfaces |
| `internal/gameserver/tech_effect_resolver.go` | Effect resolution — `ResolveTechEffects` |
| `internal/gameserver/grpc_service_tech_helpers.go` | Snapshot/diff helpers for prepared/spontaneous slot change tracking |
| `internal/game/session/technology.go` | `InnateSlot`, `PreparedSlot`, `UsePool` — runtime session state |
| `internal/storage/postgres/character_innate_tech.go` | DB persistence for innate slots (Get/Set/Decrement/RestoreAll/DeleteAll) |
| `content/technologies/neural/mind_spike.yaml` | Canonical example of a save-based spontaneous tech with tiered effects |
| `content/technologies/innate/` | All 11 region innate technology definitions |
| `api/proto/game/v1/game.proto` | `UseMessage` (ability_id + target), `InnateSlotView` (tech_id, uses_remaining, max_uses) |

## Core Data Structures

### `TechnologyDef` (in `model.go`)

```go
type TechnologyDef struct {
    ID           string        // unique snake_case identifier (YAML: id)
    Name         string        // display name
    Description  string        // flavor text
    Tradition    Tradition     // "technical" | "neural" | "bio_synthetic" | "fanatic_doctrine" (YAML string values)
    Level        int           // 1–10
    UsageType    UsageType     // "hardwired" | "prepared" | "spontaneous" | "innate"
    ActionCost   int           // actions to activate
    Range        Range         // "self" | "melee" | "ranged" | "zone"
    Targets      Targets       // "single" | "all_enemies" | "all_allies" | "zone"
    Duration     string        // e.g. "instant", "rounds:1", "minutes:1"
    SaveType     string        // "toughness" | "hustle" | "cool" — only when Resolution=="save"
    SaveDC       int           // DC for save — only when Resolution=="save"
    Resolution   string        // "save" | "attack" | "none" (empty == "none")
    Effects      TieredEffects // tiered effect lists by outcome
    AmpedLevel   int           // minimum level for amped activation (0 = no amped variant)
    AmpedEffects TieredEffects // effects when activated at AmpedLevel or higher
}
```

### `TieredEffects` (in `model.go`)

```go
type TieredEffects struct {
    OnCritSuccess []TechEffect  // save: critical success
    OnSuccess     []TechEffect  // save: success
    OnFailure     []TechEffect  // save: failure
    OnCritFailure []TechEffect  // save: critical failure
    OnMiss        []TechEffect  // attack: miss
    OnHit         []TechEffect  // attack: hit
    OnCritHit     []TechEffect  // attack: critical hit
    OnApply       []TechEffect  // no-roll: always applied
}
```

### `TechEffect` (in `model.go`)

Key fields (others are type-specific):
- `Type EffectType` — "damage" | "heal" | "condition" | "movement" | "utility" | "drain" | "skill_check" | "zone" | "summon"
- `Dice string` — dice expression e.g. "2d6" (damage/heal/drain)
- `DamageType string` — e.g. "mental", "acid", "bludgeoning"
- `Amount int` — flat bonus added to dice roll
- `ConditionID string` — condition registry ID (condition effects)
- `Value int` — condition stack count
- `Duration string` — "rounds:N" | "minutes:N" | "instant"
- `Distance int`, `Direction string` — movement effects ("toward" | "away")
- `UtilityType string`, `Description string` — utility effects

### `InnateSlot` (in `internal/game/session/technology.go`)

```go
type InnateSlot struct {
    MaxUses       int  // 0 = unlimited
    UsesRemaining int  // current remaining uses; restored on rest
}
```

### `UseMessage` / `UseResponse` proto fields

`UseMessage`:
- `ability_id string` — tech ID to activate
- `target string` — optional combatant name; empty = default combat target or self

`UseResponse` (via `ServerEvent` MessageEvent):
- Human-readable result lines joined by newline (e.g., "Nox Raider fails: 4 mental damage.")

`InnateSlotView` (in `CharacterSheetView`):
- `tech_id string`
- `uses_remaining int32`
- `max_uses int32`

## Primary Data Flow

### UseRequest path

1. Player sends `UseMessage{ability_id, target}` proto over gRPC stream.
2. `grpc_service.go` dispatch routes to `handleUse(uid, abilityID, targetID, stream)`.
3. `handleUse` loads the player's `*session.PlayerSession` from the session store.
4. Looks up the tech in the registry: `s.techRegistry.Get(abilityID)`.
5. Resolves target: if `tech.Targets == "self"` or `tech.Resolution == "none"`, target is nil; otherwise resolves `targetID` against active combat or returns an error.
6. Dispatches by usage type to check and expend uses:
   - **hardwired**: always available — no slot check or decrement.
   - **prepared**: finds the slot in `sess.PreparedTechs[level]` where `!slot.Expended`; calls `prepRepo.SetExpended(...)`.
   - **spontaneous**: checks `sess.SpontaneousUsePools[level].Remaining > 0`; calls `spontUsePoolRepo.Decrement(...)`.
   - **innate**: checks `slot.MaxUses == 0` (unlimited) or `slot.UsesRemaining > 0`; if limited calls `innateRepo.Decrement(...)`; decrements `slot.UsesRemaining` in session.
7. Calls `ResolveTechEffects(sess, techDef, targets, combatEngine, condRegistry, src)` in `tech_effect_resolver.go`.
8. Resolver dispatches by `tech.Resolution`:
   - **"save"**: calls `combat.ResolveSave(saveType, target, saveDC, src)` → outcome → selects tier → applies effects.
   - **"attack"**: rolls 1d20 + `techAttackMod(sess, tech)` vs `target.AC` → CritHit/Hit/Miss → selects tier → applies effects.
   - **"none"**: applies `Effects.OnApply` directly.
9. Effects applied per `TechEffect.Type`: damage decrements `target.CurrentHP` (floor 0); heal increments `sess.CurrentHP` (cap `MaxHP`); condition calls `condRegistry` + combat state; movement adjusts `target.Position`.
10. Returns `UseResponse` as a `ServerEvent` MessageEvent with result lines joined by newline.

### Character creation assignment path

1. `AssignTechnologies(ctx, sess, characterID, job, archetype, techReg, promptFn, hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo, region)` called from `grpc_service.go` create-character handler.
2. Merges grants from `archetype.TechnologyGrants` and `job.TechnologyGrants` via `ruleset.MergeGrants`.
3. Assigns hardwired IDs to `sess.HardwiredTechs`; persists via `hwRepo.SetAll`.
4. Assigns innate slots from archetype and region grants to `sess.InnateTechs`; persists via `innateRepo.Set` (initializes `uses_remaining = max_uses`).
5. Fills prepared slots (fixed first, then pool — prompts player if pool > open slots) via `fillFromPreparedPool`; persists via `prepRepo.Set`.
6. Fills spontaneous known techs similarly; initializes `SpontaneousUsePools` via `usePoolRepo.Set`.

## Invariants & Contracts

- **Registry is read-only after load**: `Load()` is fail-fast; `Register()` is for tests only.
- **`InnateSlot.MaxUses == 0` means unlimited**: `handleUse` must skip decrement when `MaxUses == 0`. Checking `UsesRemaining` alone is insufficient.
- **`innateRepo.Set` resets `uses_remaining` to `max_uses`**: only call at creation or re-assignment, never at login. Login uses `GetAll`.
- **`ResolveTechEffects` does not expend uses**: the caller (`handleUse`) must expend before calling the resolver.
- **`target.CurrentHP` and `sess.CurrentHP` never go below 0**: enforced in `applyEffect` with floor clamp.
- **`sess.CurrentHP` never exceeds `sess.MaxHP`**: enforced in heal branch of `applyEffect`.
- **`Validate()` enforces Resolution/SaveType/SaveDC consistency**: `resolution:"save"` requires non-empty `SaveType` and `SaveDC > 0`; `resolution:"attack"` requires empty `SaveType`; `resolution:""/"none"` requires both empty/zero.
- **Condition effects silently skip when `condRegistry` or `cbt` is nil**: out-of-combat use of condition-applying techs is a no-op with no error.
- **Traditions are YAML string values only**: `TraditionNeural`, `TraditionBioSynthetic`, etc. are typed string constants used for switch dispatch in `techAttackMod`. They are not sub-systems with separate code paths.

## Extension Points

### How to add a new technology (YAML + wiring)

1. Create a YAML file in the appropriate `content/technologies/<tradition>/` subdirectory (or `content/technologies/innate/` for innate techs).
2. Set all required fields: `id`, `name`, `tradition`, `level`, `usage_type`, `action_cost`, `range`, `targets`, `duration`.
3. If `usage_type` is not `innate`, set `resolution` (`"save"`, `"attack"`, or `"none"`).
   - For `"save"`: also set `save_type` and `save_dc`.
4. Populate `effects:` with at least one effect in the appropriate tier(s):
   - `on_apply:` for `resolution:"none"`
   - `on_success:`/`on_failure:` etc. for `resolution:"save"`
   - `on_hit:`/`on_miss:`/`on_crit_hit:` for `resolution:"attack"`
5. Run `make test` — `registry_test.go` loads all YAML files and will catch validation errors.
6. To grant the tech to a character class: add the tech ID to the relevant job or archetype YAML under `technology_grants.hardwired`, `technology_grants.prepared.fixed`, `technology_grants.spontaneous.fixed`, or the `pool` equivalents.
7. To grant the tech as a region innate: add an entry to the region YAML under `innate_technologies:` with `id:` and `uses_per_day:` (0 = unlimited).

### How to add a new effect type

1. Add a constant to `model.go`: `EffectFoo EffectType = "foo"`.
2. Add it to `validEffectTypes` in `model.go`.
3. Add a case to `applyEffect` in `tech_effect_resolver.go` that returns a result message string.
4. Write property-based tests (SWENG-5a) covering the new effect type.

## Common Pitfalls

- **`innateRepo.Set` vs `GetAll` at login**: `Set` overwrites `uses_remaining` to `max_uses`. Call only at character creation or re-assignment; use `GetAll` at login.
- **Empty `resolution` field**: treated as `"none"` — safe for innate techs where effect resolution is deferred. Do not set `save_type`/`save_dc` on such techs or `Validate()` will reject them.
- **`TieredEffects.AllEffects()` for validation**: `Validate()` requires at least one effect total across all tiers. An innate tech with no effects will fail validation. If effects are intentionally deferred, add a minimal `on_apply: [{type: utility}]` entry.
- **`techAttackMod` tradition mapping**: neural→Savvy, bio_synthetic→Grit, all others→Quickness. The fanatic_doctrine and technical traditions both map to Quickness.
- **Save tier fallback**: `CritSuccess` falls back to `OnSuccess` when `OnCritSuccess` is empty; `CritFailure` falls back to `OnFailure` when `OnCritFailure` is empty. This mirrors PF2E convention.
- **Area targets (`all_enemies`)**: the caller in `handleUse` is responsible for building the full `[]*combat.Combatant` slice. `ResolveTechEffects` iterates the slice but does not query the combat engine itself.
- **`range` values**: the `Registry` and model use `Range` typed constants (`"self"`, `"melee"`, `"ranged"`, `"zone"`). Innate techs that use `"touch"`, `"close"`, or `"emanation"` as range in their YAML will fail `Validate()` — those are not yet valid `Range` values; add them to `validRanges` if needed.

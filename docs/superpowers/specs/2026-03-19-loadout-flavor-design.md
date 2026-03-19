# Loadout Flavor Design

**Date:** 2026-03-19
**Feature:** Spellbook / Memorization Analog — per-tradition flavor for prepared tech management

---

## Goal

Replace all generic "technologies prepared" player-facing copy with per-tradition lore-flavored language. Extend the existing `loadout` command to also display prepared techs (alongside weapon presets). Add `prep` and `kit` as aliases for `loadout`. Rework the `rest` interactive prep flow to use tradition-aware prompts and slot labels.

No database schema changes. No mechanic changes — slots, expending, and rearrangement logic are unchanged.

---

## Scope

In scope: `TraditionFlavor` data type, `FlavorFor`, `DominantTradition`, `FormatPreparedTechs`, extending `loadout` command, `prep`/`kit` aliases, tradition-flavored rest prompts and completion message.

Out of scope: spontaneous tech display, innate tech display, hardwired tech display, changes to slot progression or expending logic.

---

## Tradition Flavor Table

| Job IDs | Tradition | `LoadoutTitle` | `PrepVerb` | `SlotNoun` | `RestMessage` |
|---|---|---|---|---|---|
| `nerd` | `technical` | `Field Loadout` | `Configure` | `slot` | `Field loadout configured.` |
| `naturalist`, `drifter` | `bio_synthetic` | `Chem Kit` | `Mix` | `dose` | `Chem kit mixed.` |
| `schemer`, `influencer` | `neural` | `Neural Profile` | `Queue` | `routine` | `Neural profile written.` |
| `zealot` | `fanatic_doctrine` | `Doctrine` | `Prepare` | `rite` | `Doctrine prepared.` |
| *(all others)* | *(fallback)* | `Loadout` | `Prepare` | `slot` | `Technologies prepared.` |

---

## Requirements

### REQ-LF1
`TraditionFlavor` MUST be defined as a struct in `internal/game/technology/flavor.go` with four exported string fields: `LoadoutTitle`, `PrepVerb`, `SlotNoun`, and `RestMessage`.

### REQ-LF2
`FlavorFor(tradition string) TraditionFlavor` MUST be defined as an exported function in `internal/game/technology/flavor.go`. It MUST return the correct `TraditionFlavor` for `"technical"`, `"bio_synthetic"`, `"neural"`, and `"fanatic_doctrine"`. For any other string (including `""`) it MUST return the fallback flavor: `{LoadoutTitle: "Loadout", PrepVerb: "Prepare", SlotNoun: "slot", RestMessage: "Technologies prepared."}`.

### REQ-LF3
`DominantTradition(jobID string) string` MUST be defined as an exported function in `internal/game/technology/flavor.go`. It MUST return `"technical"` for `"nerd"`, `"bio_synthetic"` for `"naturalist"` and `"drifter"`, `"neural"` for `"schemer"` and `"influencer"`, `"fanatic_doctrine"` for `"zealot"`, and `""` for all other job IDs.

### REQ-LF4
`FormatPreparedTechs(slots map[int][]*session.PreparedSlot, flavor TraditionFlavor) string` MUST be defined as an exported function in `internal/game/technology/flavor.go`. It MUST format prepared tech slots grouped by level in ascending level order using the following layout:
```
[<LoadoutTitle>]
  Level <N> — <count> <slotNoun>(s)
    <techID>    ready
    <techID>    expended
```
When `slots` is empty or all levels have zero slots, it MUST return `"No <LoadoutTitle> configured."`. The plural suffix `(s)` MUST be omitted when `count == 1` (i.e., `"1 slot"` not `"1 slot(s)"`).

### REQ-LF5
Table-driven unit tests in `internal/game/technology/flavor_test.go` MUST cover:
- All named job IDs for `DominantTradition` and all unmapped job IDs (fallback)
- All four tradition values plus the fallback case for `FlavorFor`
- `FormatPreparedTechs` with: empty map, single level with one ready slot, single level with mixed ready/expended slots, multiple levels

### REQ-LF6
`RearrangePreparedTechs` in `internal/gameserver/technology_assignment.go` MUST accept two new parameters appended after the existing `prepRepo PreparedTechRepo` parameter: `sendFn func(string)` and `flavor technology.TraditionFlavor`. A nil `sendFn` MUST be treated as a no-op (no panic). All existing callers MUST be updated to pass the appropriate `sendFn` and `flavor`.

### REQ-LF7
When `RearrangePreparedTechs` begins (after the no-op guard), it MUST call `sendFn` with `"<PrepVerb>ing <LoadoutTitle>..."` (e.g., `"Configuring Field Loadout..."`, `"Mixing Chem Kit..."`).

### REQ-LF8
When auto-assigning a fixed slot during `RearrangePreparedTechs`, it MUST call `sendFn` with `"Level <N>, <slotNoun> <idx> (fixed): <techID>"` where `<idx>` is the 1-based slot index within the level. When prompting the player to fill an open pool slot, it MUST call `sendFn` with `"Level <N>, <slotNoun> <idx>: choose from pool"` before issuing the prompt.

### REQ-LF9
The `rest` completion message in `handleRest` (currently `"You finish your rest. HP restored to maximum and technologies prepared."`) MUST be replaced with `"You finish your rest. HP restored to maximum. <flavor.RestMessage>"`, where `flavor` is `FlavorFor(DominantTradition(sess.Class))`.

### REQ-LF10
The `handleRest` function MUST compute `flavor := technology.FlavorFor(technology.DominantTradition(sess.Class))` and pass it (along with a `sendFn` that sends a message to the player's stream) to `RearrangePreparedTechs`.

### REQ-LF11
The existing `loadout` command entry in `internal/game/command/commands.go` MUST have `"prep"` and `"kit"` added to its `Aliases` slice (alongside the existing `"lo"` alias).

### REQ-LF12
`handleLoadout` in `internal/gameserver/grpc_service.go` MUST be extended: when `req.GetArg()` is `""` (no argument), it MUST return a message combining the weapon preset section (from `command.HandleLoadout(sess, "")`) and the prepared tech section (from `technology.FormatPreparedTechs(sess.PreparedTechs, flavor)`) separated by a blank line. When `req.GetArg()` is non-empty, the existing weapon preset swap behavior is unchanged.

### REQ-LF13
`go test ./internal/game/technology/... ./internal/game/command/... ./internal/gameserver/...` MUST pass after all changes are applied.

---

## Architecture

### Data Flow

```
handleRest
  └─ flavor := FlavorFor(DominantTradition(sess.Class))
  └─ RearrangePreparedTechs(..., sendFn, flavor)
       └─ sendFn("Configuring Field Loadout...")
       └─ for each level with slots:
            └─ fixed slots: sendFn("Level N, slot idx (fixed): techID")
            └─ open slots:  sendFn("Level N, slot idx: choose from pool") then promptFn(options)
  └─ sendFn("You finish your rest. HP restored to maximum. Field loadout configured.")

handleLoadout (arg == "")
  └─ flavor := FlavorFor(DominantTradition(sess.Class))
  └─ weaponSection := command.HandleLoadout(sess, "")
  └─ prepSection   := technology.FormatPreparedTechs(sess.PreparedTechs, flavor)
  └─ return weaponSection + "\n\n" + prepSection

loadout / prep / kit → same handler (command.HandlerLoadout)
```

### Modified Signatures

```go
// flavor.go — new file in internal/game/technology/
type TraditionFlavor struct {
    LoadoutTitle string
    PrepVerb     string
    SlotNoun     string
    RestMessage  string
}
func FlavorFor(tradition string) TraditionFlavor
func DominantTradition(jobID string) string
func FormatPreparedTechs(slots map[int][]*session.PreparedSlot, flavor TraditionFlavor) string

// technology_assignment.go — RearrangePreparedTechs gains sendFn and flavor params
func RearrangePreparedTechs(
    ctx context.Context,
    sess *session.PlayerSession,
    characterID int64,
    job *ruleset.Job,
    techReg *technology.Registry,
    promptFn TechPromptFn,
    prepRepo PreparedTechRepo,
    sendFn func(string),               // NEW — nil-safe
    flavor technology.TraditionFlavor, // NEW
) error
```

---

## File Map

| File | Change |
|------|--------|
| `internal/game/technology/flavor.go` | New: `TraditionFlavor`, `FlavorFor`, `DominantTradition`, `FormatPreparedTechs` |
| `internal/game/technology/flavor_test.go` | New: table-driven tests for all flavor values, job mappings, and `FormatPreparedTechs` |
| `internal/game/command/commands.go` | Add `"prep"` and `"kit"` to loadout command `Aliases` |
| `internal/gameserver/technology_assignment.go` | Add `sendFn` and `flavor` params to `RearrangePreparedTechs`; use them for opening message and slot notifications |
| `internal/gameserver/grpc_service.go` | Extend `handleLoadout` for no-arg combined view; update `handleRest` to compute and pass flavor + sendFn; update rest completion message |

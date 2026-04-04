---
name: Brothel Expansion
description: Adds a new 'brothel_keeper' NPC type and brothel room to every zone — cheaper rest with disease risk, robbery risk, and a 1-day +1 Flair bonus. Each brothel also houses a black market merchant and the zone's Fixer.
type: spec
---

# Brothel Expansion

## Problem

Motels are the only paid rest option. The brothel adds a second, cheaper rest venue with risk/reward trade-offs (disease, robbery, Flair bonus), collocates the black market merchant and Fixer in one lore-coherent room, and gives every zone a consistent underworld hub.

## Scope

- REQ-BR-1: A new `brothel_keeper` NPC type MUST be added alongside `motel_keeper`.
- REQ-BR-2: `BrothelConfig` MUST have fields: `rest_cost int`, `disease_chance float64`, `robbery_chance float64`, `disease_pool []string`, `flair_bonus_duration string`.
- REQ-BR-3: `BrothelConfig.Validate()` MUST reject: `rest_cost <= 0`, `disease_chance` or `robbery_chance` outside `[0.0, 1.0]`, empty `disease_pool`, `flair_bonus_duration` that does not parse as a valid Go duration.
- REQ-BR-4: `handleRest` MUST dispatch to `handleBrothelRest` when a `brothel_keeper` NPC is present in a Safe room.
- REQ-BR-5: The common long-rest restoration logic MUST be extracted into `applyLongRestEffects()` and called by both `handleMotelRest` and `handleBrothelRest`.
- REQ-BR-6: `handleBrothelRest` MUST block rest if the player has insufficient crypto, showing the cost.
- REQ-BR-7: `handleBrothelRest` MUST deduct `RestCost` crypto, then call `applyLongRestEffects`.
- REQ-BR-8: `handleBrothelRest` MUST apply the `flair_bonus_1` condition for `FlairBonusDur` after rest. Message: `"You feel unusually confident. (+1 Flair)"` (duration not shown in message; duration is content-configured).
- REQ-BR-9: `handleBrothelRest` MUST roll `rand.Float64() < DiseaseChance`; on hit, select a random substance ID from `DiseasePool` and call `ApplySubstanceByID`. Send a console message naming the disease.
- REQ-BR-10: `handleBrothelRest` MUST roll `rand.Float64() < RobberyChance`; on hit, steal 5% of the player's crypto (rounded down, minimum 1 if player has any) and remove up to 5% of backpack item count (random items; stackable items lose one stack). Persist currency and inventory. Send message: `"You wake to find someone has gone through your belongings."`
- REQ-BR-11: Disease and robbery rolls MUST be independent — both can trigger in the same rest.
- REQ-BR-12: Rest MUST complete (full restoration + Flair bonus applied) before disease and robbery rolls.
- REQ-BR-13: If `ApplySubstanceByID` returns an error, log a warning and do not block rest completion.
- REQ-BR-14: 10 new `SubstanceDef` YAML files MUST be created in `content/substances/` with `category: disease`, `addictive: false`, and mixed effects (ability penalties and conditions).
- REQ-BR-15: A new condition `flair_bonus_1` MUST be created in `content/conditions/` with a +1 Flair modifier.
- REQ-BR-16: A new `merchant_type` value `"black_market"` MUST be added to `MerchantConfig`'s valid type list. Black market merchants stock contraband, drugs, and restricted weapons.
- REQ-BR-17: Every zone (16 total) MUST have one new brothel room: `danger_level: safe`, connected to the zone's safe cluster, with lore-appropriate name.
- REQ-BR-18: Each brothel room MUST contain three NPCs: `brothel_keeper`, black market merchant, and the zone's Fixer (relocated from its current room).
- REQ-BR-19: The `POITypes` table MUST add two new entries: `{ID:"brothel", Symbol:'B', Color:"\033[91m", Label:"Brothel"}` and `{ID:"motel", Symbol:'R', Color:"\033[95m", Label:"Motel"}`.
- REQ-BR-20: `NpcRoleToPOIID` MUST map `"brothel_keeper"` → `"brothel"` and `"motel_keeper"` → `"motel"`.
- REQ-BR-21: Default `disease_chance` MUST be 0.15; default `robbery_chance` MUST be 0.20.
- REQ-BR-22: The brothel's `rest_cost` MUST be less than the same zone's motel `rest_cost` (content constraint; enforced by convention, not code).

## Architecture

### New Types

**`BrothelConfig`** in `internal/game/npc/noncombat.go`:

```go
type BrothelConfig struct {
    RestCost        int      `yaml:"rest_cost"`
    DiseaseChance   float64  `yaml:"disease_chance"`
    RobberyChance   float64  `yaml:"robbery_chance"`
    DiseasePool     []string `yaml:"disease_pool"`
    FlairBonusDur   string   `yaml:"flair_bonus_duration"`
}
```

**`BrothelConfig.Validate()`** enforces REQ-BR-3.

`npc.Template` gains `Brothel *BrothelConfig` field (yaml: `brothel`). `"brothel_keeper"` added to `validNPCTypes` and its validation case added in `Template.Validate()`.

### Rest Handler Refactor

`applyLongRestEffects(uid string, sess *session.PlayerSession, sendMsg func(string) error, requestID string, stream gamev1.GameService_SessionServer) error` — extracted from `handleMotelRest`; performs HP + tech pool + prepared tech restoration.

`handleMotelRest` calls `applyLongRestEffects` (no behavior change).

`handleBrothelRest(uid, sess, brothelNPC, sendMsg, requestID, stream)`:
1. Check and deduct crypto (REQ-BR-6/7)
2. `applyLongRestEffects` (REQ-BR-7)
3. Apply `flair_bonus_1` condition (REQ-BR-8)
4. Disease roll (REQ-BR-9)
5. Robbery roll (REQ-BR-10)

`handleRest` gains a second NPC scan pass for `brothel_keeper` (REQ-BR-4).

### Robbery Logic

```
stolenCrypto = max(1, floor(currency * 0.05))  // if currency > 0
stolenItems  = random selection of floor(backpackCount * 0.05) items (min 0)
               stackable items lose one stack; non-stackable items removed
```

### Disease Pool (10 substances)

All in `content/substances/`, `category: disease`, `addictive: false`:

| ID | Name | Effect |
|----|------|--------|
| `street_fever` | Street Fever | -1 Grit condition for 4h, 1d4 HP drain per tick for 1h |
| `crotch_rot` | Crotch Rot | `enfeebled 1` condition for 8h |
| `swamp_itch` | Swamp Itch | -1 Savvy condition for 6h, `sickened 1` for 2h |
| `track_rash` | Track Rash | `sickened 1` for 4h, HP drain 1 per tick for 30m |
| `gutter_flu` | Gutter Flu | -1 Grit and -1 Flair conditions for 6h |
| `rust_pox` | Rust Pox | `enfeebled 2` for 2h |
| `neon_blight` | Neon Blight | -1 Savvy and -1 Grit conditions for 8h |
| `wet_lung` | Wet Lung | HP drain 2 per tick for 2h, `sickened 1` for 1h |
| `chrome_mange` | Chrome Mange | -1 Flair condition for 12h |
| `black_tongue` | Black Tongue | `sickened 2` for 3h |

### Map POI Updates

`POITypes` in `internal/game/maputil/poi.go` gains:
```go
{ID: "motel",   Symbol: 'R', Color: "\033[95m", Label: "Motel"},
{ID: "brothel", Symbol: 'B', Color: "\033[91m", Label: "Brothel"},
```

`NpcRoleToPOIID` maps `"motel_keeper"` → `"motel"`, `"brothel_keeper"` → `"brothel"`.

### New Condition

`content/conditions/flair_bonus_1.yaml` — +1 Flair modifier, duration set at application time by `FlairBonusDur`.

## Feature Index Entry

Slug: `brothel-expansion`
Category: `world`
Effort: `L`
Dependencies: `resting`, `advanced-health`, `factions`, `non-combat-npcs`, `map-poi`

## Testing

- REQ-BR-T1: `BrothelConfig.Validate()` MUST be property-tested for all invalid field combinations.
- REQ-BR-T2: `handleBrothelRest` with insufficient credits MUST block rest.
- REQ-BR-T3: `handleBrothelRest` with sufficient credits MUST apply full restoration and Flair bonus.
- REQ-BR-T4: Disease roll at 100% chance MUST call `ApplySubstanceByID` with a pool member.
- REQ-BR-T5: Robbery roll at 100% chance MUST deduct crypto and remove backpack items.
- REQ-BR-T6: Disease and robbery rolls at 0% chance MUST leave player state unchanged.
- REQ-BR-T7: `NpcRoleToPOIID("motel_keeper")` MUST return `"motel"`.
- REQ-BR-T8: `NpcRoleToPOIID("brothel_keeper")` MUST return `"brothel"`.
- REQ-BR-T9: All 10 disease `SubstanceDef` files MUST load without validation errors.

## Files

| File | Action |
|------|--------|
| `internal/game/npc/noncombat.go` | Modify — add `BrothelConfig` struct and `Validate()` |
| `internal/game/npc/template.go` | Modify — add `Brothel *BrothelConfig` field; add `"brothel_keeper"` to valid types |
| `internal/game/npc/template_test.go` | Modify — add `BrothelConfig.Validate()` property tests |
| `internal/gameserver/grpc_service.go` | Modify — extract `applyLongRestEffects`; add `handleBrothelRest`; update `handleRest` dispatch |
| `internal/gameserver/grpc_service_rest_test.go` | Modify — add brothel rest test cases |
| `internal/game/maputil/poi.go` | Modify — add `"motel"` and `"brothel"` POI types; update `NpcRoleToPOIID` |
| `internal/game/maputil/poi_test.go` | Modify — add motel and brothel mapping tests |
| `content/substances/street_fever.yaml` | Create |
| `content/substances/crotch_rot.yaml` | Create |
| `content/substances/swamp_itch.yaml` | Create |
| `content/substances/track_rash.yaml` | Create |
| `content/substances/gutter_flu.yaml` | Create |
| `content/substances/rust_pox.yaml` | Create |
| `content/substances/neon_blight.yaml` | Create |
| `content/substances/wet_lung.yaml` | Create |
| `content/substances/chrome_mange.yaml` | Create |
| `content/substances/black_tongue.yaml` | Create |
| `content/conditions/flair_bonus_1.yaml` | Create |
| `content/npcs/<zone>_brothel_keeper.yaml` | Create ×16 — one per zone |
| `content/npcs/<zone>_black_market_merchant.yaml` | Create ×16 — one per zone |
| `content/zones/<zone>.yaml` | Modify ×16 — add brothel room, relocate Fixer |
| `docs/features/brothel-expansion.md` | Create — feature doc |
| `docs/features/index.yaml` | Modify — add entry |

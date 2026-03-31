# Motel NPCs Design

**Date:** 2026-03-31
**Feature:** Add a motel keeper NPC to each of the 17 zones so players can use the `rest` command in safe rooms.

---

## Background

The motel rest mechanic is fully implemented in `handleRest` / `handleMotelRest` (`internal/gameserver/grpc_service.go`). When a player uses `rest` in a safe room, the handler searches for an NPC with `Instance.RestCost > 0`. If found, it charges the player and applies a full long rest. If not found, it returns "There is no motel here to rest at."

`RestCost` exists on `Instance` but is never populated in production — there is no YAML field or loader for it. All production NPCs spawn with `RestCost = 0`, making motel rest unreachable.

---

## Design

### REQ-MOT-1: New `motel_keeper` NPC Type

A new `npc_type: motel_keeper` is added to the set of valid NPC types in `internal/game/npc/template.go`. Template validation MUST reject any `motel_keeper` NPC that is missing a `motel:` config block or has `rest_cost <= 0`.

### REQ-MOT-2: `MotelConfig` Struct

```go
// MotelConfig holds configuration for a motel_keeper NPC (REQ-MOT-1).
type MotelConfig struct {
    RestCost int `yaml:"rest_cost"`
}
```

Added to `Template`:

```go
Motel *MotelConfig `yaml:"motel,omitempty"`
```

### REQ-MOT-3: Instance Population

In `NewInstanceWithResolver`, after the existing instance fields are set:

```go
if tmpl.Motel != nil {
    inst.RestCost = tmpl.Motel.RestCost
}
```

No changes to `handleMotelRest` or `handleRest` are required.

### REQ-MOT-4: YAML Format

```yaml
- id: <zone>_motel_keeper
  name: "<lore name>"
  npc_type: motel_keeper
  type: human
  description: "<lore description>"
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: <N>
```

`motel_keeper` NPCs MUST NOT have weapon, armor, loot, or combat config blocks. They are non-combat NPCs.

---

## Zone Assignments

One motel keeper per zone. Placed in the designated hub safe room. Wooklyn requires a new `content/npcs/non_combat/wooklyn.yaml` file.

| Zone | Template ID | Hub Room | NPC Name | Rest Cost |
|------|-------------|----------|----------|-----------|
| Felony Flats | `felony_flats_motel_keeper` | `flats_safe_house` | Crash Pad Runner | 25 cr |
| Rustbucket Ridge | `rustbucket_ridge_motel_keeper` | `grinders_row` | Scrap Inn Clerk | 35 cr |
| SE Industrial | `se_industrial_motel_keeper` | `sei_break_room` | Shift Boss | 35 cr |
| Ross Island | `ross_island_motel_keeper` | `ross_dock_shack` | Dock Keeper | 40 cr |
| Sauvie Island | `sauvie_island_motel_keeper` | `sauvie_farm_stand` | Farm Host | 40 cr |
| The Couve | `the_couve_motel_keeper` | `couve_the_crossing` | Crossing Keeper | 45 cr |
| Vantucky | `vantucky_motel_keeper` | `vantucky_the_compound` | Compound Steward | 45 cr |
| Wooklyn | `wooklyn_motel_keeper` | `tofteville_market` | Camp Steward | 45 cr |
| Battleground | `battleground_motel_keeper` | `battle_neutral_commons` | Commons Steward | 50 cr |
| Troutdale | `troutdale_motel_keeper` | `trout_neutral_motel` | Motel Clerk | 50 cr |
| NE Portland | `ne_portland_motel_keeper` | `ne_neutral_bunker` | Bunker Host | 55 cr |
| Aloha | `aloha_motel_keeper` | `aloha_the_bazaar` | Bazaar Host | 60 cr |
| Beaverton | `beaverton_motel_keeper` | `beav_neutral_market` | Extended Stay Clerk | 75 cr |
| Hillsboro | `hillsboro_motel_keeper` | `hills_waystation` | Waystation Keeper | 75 cr |
| Downtown | `downtown_motel_keeper` | `market_district` | Night Manager | 100 cr |
| Lake Oswego | `lake_oswego_motel_keeper` | `lo_the_commons` | Estate Concierge | 120 cr |
| PDX International | `pdx_international_motel_keeper` | `pdx_neutral_terminal` | Terminal Agent | 150 cr |

---

## NPC Descriptions

| Zone | Description |
|------|-------------|
| Felony Flats | Rents floor space in the safe house, three to a room, no questions asked. Bring your own bedroll. |
| Rustbucket Ridge | Runs a row of converted shipping containers fitted with cots. Cold in winter, hot in summer, safe year-round. |
| SE Industrial | Manages the break room cots between shifts. The fluorescent lights never fully turn off. You get used to it. |
| Ross Island | Rents a cot in the corner of the dock shack. The river noise keeps some people up. Others find it peaceful. |
| Sauvie Island | Rents bunk space in the old farmhouse. Breakfast is not included, but the smell of coffee at dawn is free. |
| The Couve | Runs the only beds at the river crossing. Charges by the night, cash only, no exceptions. |
| Vantucky | Manages sleeping quarters inside the compound walls. Strict check-in, strict lights-out. Worth it for the security. |
| Wooklyn | Keeps the caravan bunk tent. Communal sleeping, communal rules. No violence, no exceptions, no refunds. |
| Battleground | Manages sleeping quarters in the neutral commons. Militia affiliation checked at the door; inside, everyone's just tired. |
| Troutdale | Runs what's left of the highway motel — half the rooms still intact, the sign still lights up at night. Old habit. |
| NE Portland | Rents bunks in the reinforced neutral bunker. Not fancy, but you'll wake up in the morning. |
| Aloha | Rents back rooms above the market stalls. The beds are clean, the noise never fully stops, but the price is fair. |
| Beaverton | Works the front desk of a former corporate extended-stay hotel, still running on backup power. Hot water, sometimes. |
| Hillsboro | Manages the waystation's bunk room. Former tech corridor worker, now runs the most organized lodging west of Portland. |
| Downtown | Runs the last functioning hotel in downtown Portland. The rates are steep, the sheets are clean, the door locks work. |
| Lake Oswego | Manages guest quarters in a former lakeside estate. Speaks quietly, prices loudly. The linens are actual linen. |
| PDX International | Manages the last operational terminal hotel. Prices reflect demand, not quality. Knows it. |

---

## Spawn Configuration

Each hub room gains one spawn entry:

```yaml
- template: <zone>_motel_keeper
  count: 1
  respawn_after: 0s
```

`respawn_after: 0s` matches the pattern used for chip docs and other permanent non-combat NPCs.

---

## Test Coverage

- REQ-MOT-5: A unit test MUST verify that `NewInstanceWithResolver` sets `RestCost` from `tmpl.Motel.RestCost` when `Motel` is non-nil.
- REQ-MOT-6: A unit test MUST verify that template validation rejects a `motel_keeper` with no `motel:` block.
- REQ-MOT-7: A unit test MUST verify that template validation rejects a `motel_keeper` with `rest_cost: 0`.
- REQ-MOT-8: Existing `handleMotelRest` tests in `grpc_service_rest_test.go` cover the rest flow; no new integration tests required for the rest mechanic itself.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/game/npc/template.go` | Add `MotelConfig` struct, `Motel *MotelConfig` field on `Template`, validate `motel_keeper` type |
| `internal/game/npc/instance.go` | Populate `RestCost` from `tmpl.Motel` in `NewInstanceWithResolver` |
| `internal/game/npc/template_test.go` | Unit tests REQ-MOT-5, REQ-MOT-6, REQ-MOT-7 |
| `content/npcs/non_combat/*.yaml` (16 files) | Add `<zone>_motel_keeper` entry to each existing file |
| `content/npcs/non_combat/wooklyn.yaml` | New file with `wooklyn_motel_keeper` entry |
| `content/zones/*.yaml` (17 files) | Add spawn entry to hub safe room in each zone |

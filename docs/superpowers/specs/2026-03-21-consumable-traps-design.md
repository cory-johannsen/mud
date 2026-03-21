# Consumable Traps Design

## Overview

Consumable traps are player-deployable trap items that reference existing `TrapTemplate` definitions. Players carry them in their backpack, deploy them via a `deploy` command, and the trap arms in the room at the player's current combat position. In combat, the trap fires when an NPC moves within trigger range; outside combat, it fires on the next room entry. All consumable traps are one-shot regardless of the template's `reset_mode`.

---

## Data Model

### `TrapTemplate` — new fields

| Field | Type | YAML key | Default | Description |
|---|---|---|---|---|
| `TriggerRangeFt` | `int` | `trigger_range_ft` | `5` | Distance in feet at which the trap fires when a combatant moves within range |
| `BlastRadiusFt` | `int` | `blast_radius_ft` | `0` | Radius in feet for AoE payload; `0` = single target (triggering combatant only) |

All existing `content/traps/*.yaml` files gain these two fields with appropriate defaults:

| Template | `trigger_range_ft` | `blast_radius_ft` |
|---|---|---|
| mine | 5 | 10 |
| pit | 5 | 0 |
| bear_trap | 5 | 0 |
| trip_wire | 5 | 5 |
| honkeypot_charmer | 5 | 0 |
| pressure_plate_mine | 5 | 10 |

### `TrapInstanceState` — new fields

| Field | Type | Description |
|---|---|---|
| `DeployPosition` | `int` | Player's `Position` (feet) at deploy time; `0` for world/static traps |
| `IsConsumable` | `bool` | `true` for player-deployed traps; enforces one-shot behavior regardless of template `reset_mode` |

### `ItemDef` — new field and kind

`ItemDef` gains `TrapTemplateRef string` (YAML: `trap_template_ref`). Valid only when `Kind == "trap"`.

New item kind: `"trap"`.

### New content: `content/items/deployable_traps.yaml`

Five deployable trap item definitions, one per trap template:

```yaml
- id: deployable_mine
  name: Deployable Mine
  kind: trap
  trap_template_ref: mine
  weight: 2.0
  stackable: true
  max_stack: 5
  value: 300

- id: deployable_pit_trap
  name: Pit Trap Kit
  kind: trap
  trap_template_ref: pit
  weight: 3.0
  stackable: true
  max_stack: 3
  value: 150

- id: deployable_bear_trap
  name: Bear Trap
  kind: trap
  trap_template_ref: bear_trap
  weight: 4.0
  stackable: true
  max_stack: 3
  value: 120

- id: deployable_trip_wire
  name: Trip Wire
  kind: trap
  trap_template_ref: trip_wire
  weight: 0.5
  stackable: true
  max_stack: 10
  value: 80

- id: deployable_honkeypot
  name: Honkeypot Device
  kind: trap
  trap_template_ref: honkeypot_charmer
  weight: 1.0
  stackable: true
  max_stack: 5
  value: 250
```

---

## Deploy Command Flow

### Proto

```proto
message DeployTrapRequest {
    string item_name = 1;
}
// Added to ClientMessage.payload oneof at next available field number
```

### Command registration

- Handler key: `deploy_trap`
- Alias: `deploy`
- Category: `combat`
- Help text: `deploy <item> — arm a trap item at your current position (1 AP in combat)`

### Handler logic (`handleDeployTrap`)

**Preconditions:**
- Player session exists
- Named item is in backpack with `kind == "trap"`

**In combat:**
1. Verify player has ≥1 AP; return "Not enough AP to deploy a trap." if not
2. Spend 1 AP
3. Remove 1 from item stack in backpack
4. Look up `TrapTemplate` by `ItemDef.TrapTemplateRef`
5. Generate `instanceID = TrapInstanceID(zoneID, roomID, "consumable", newUUID())`
6. Call `trapMgr.AddConsumableTrap(instanceID, tmpl, sess.Combatant.Position)`
7. Push message: `"You arm a [item name] at your position."`

**Outside combat:**
- Steps 3–7 only (no AP check/spend)
- `DeployPosition = 0` (world-trap semantics; fires on room entry)
- Push message: `"You arm a [item name] here."`

**Error cases:**
- Item not found: `"You don't have a [item name]."`
- Item not a trap: `"You can't deploy that."`
- Template not found: logged as error; return `"That trap is broken — contact an admin."`

---

## Combat Trigger Logic

### Hook: `SetOnCombatantMoved`

`CombatHandler` gains:
- `onCombatantMoved func(roomID, movedCombatantID string)` field
- `SetOnCombatantMoved(fn func(roomID, movedCombatantID string))` setter

The callback fires in `resolveAndAdvanceLocked` after any `ActionStride`, `ActionStep`, or `ActionShove` resolves — after the position update, before the next action.

### `checkConsumableTraps(roomID, movedCombatantID string)`

Called from the `onCombatantMoved` callback wired in `WireConsumableTrapTrigger()`.

```
zone, room = lookup by roomID
for each instanceID in trapMgr.TrapsForRoom(zone.ID, room.ID):
    instance = trapMgr.GetTrap(instanceID)
    if !instance.Armed || !instance.IsConsumable: continue
    tmpl = trapTemplates[instance.TemplateID]
    movedCombatant = combat engine lookup by movedCombatantID
    dist = abs(movedCombatant.Position - instance.DeployPosition)
    if dist > tmpl.TriggerRangeFt: continue
    // Fire
    if tmpl.BlastRadiusFt == 0:
        fireTrap(movedCombatantID, ..., tmpl, instanceID, dangerLevel, false)
    else:
        for each combatant in room where abs(combatant.Position - instance.DeployPosition) <= tmpl.BlastRadiusFt:
            fireTrap(combatant.ID, ..., tmpl, instanceID, dangerLevel, false)
    trapMgr.Disarm(instanceID)  // always one-shot
```

### `fireTrap` blast radius extension

`fireTrap` gains a `targetUID string` parameter (replacing the implicit single-target assumption) so blast-radius fan-out can call it once per affected combatant. Existing callers pass the single relevant UID.

---

## Out-of-Combat Behavior

Consumable traps deployed outside combat use `TriggerEntry` semantics. `checkEntryTraps` already iterates all armed traps in the room. Since `IsConsumable == true` traps are disarmed immediately after firing (in `fireTrap`), and `DeployPosition == 0`, they behave identically to one-shot room traps.

No changes required to `checkEntryTraps`.

---

## Requirements

- REQ-CTR-1: A `deploy <item>` command MUST exist, costing 1 AP in combat and 0 AP outside combat.
- REQ-CTR-2: The `deploy` command MUST only accept items with `kind == "trap"`.
- REQ-CTR-3: Deploying a trap MUST remove exactly 1 unit from the item stack in the player's backpack.
- REQ-CTR-4: A deployed trap MUST be armed at the player's current `Position` in combat, or at position 0 outside combat.
- REQ-CTR-5: In combat, a deployed trap MUST fire when a combatant moves within `TriggerRangeFt` feet of the trap's `DeployPosition`.
- REQ-CTR-6: A consumable trap with `BlastRadiusFt == 0` MUST apply its payload to the triggering combatant only.
- REQ-CTR-7: A consumable trap with `BlastRadiusFt > 0` MUST apply its payload to all combatants within `BlastRadiusFt` feet of `DeployPosition` at trigger time, including the deploying player.
- REQ-CTR-8: A consumable trap MUST be one-shot — it MUST be disarmed immediately after firing regardless of the template's `reset_mode`.
- REQ-CTR-9: A consumable trap deployed outside combat MUST fire on the next room entry that meets the trigger conditions.
- REQ-CTR-10: Deployed consumable traps MUST be detectable and disarmable via the existing `disarm` command.
- REQ-CTR-11: `TrapTemplate` MUST expose `trigger_range_ft` (default 5) and `blast_radius_ft` (default 0) fields.
- REQ-CTR-12: `ItemDef` MUST support `kind: trap` with a `trap_template_ref` field referencing a `TrapTemplate` ID.

---

## Files Changed

| File | Change |
|---|---|
| `internal/game/trap/template.go` | Add `TriggerRangeFt`, `BlastRadiusFt` fields |
| `internal/game/trap/manager.go` | Add `DeployPosition`, `IsConsumable` to `TrapInstanceState`; add `AddConsumableTrap` helper |
| `internal/game/inventory/item.go` | Add `TrapTemplateRef string`; add `"trap"` kind |
| `internal/gameserver/combat_handler.go` | Add `onCombatantMoved` callback + `SetOnCombatantMoved` setter; call after Stride/Step/Shove |
| `internal/gameserver/grpc_service_trap.go` | Add `WireConsumableTrapTrigger()`, `checkConsumableTraps()`, blast fan-out in `fireTrap` |
| `internal/gameserver/grpc_service_deploy_trap.go` | New: `handleDeployTrap` handler |
| `internal/gameserver/grpc_service_deploy_trap_test.go` | New: deploy command tests |
| `internal/gameserver/grpc_service.go` | Wire deploy dispatch + `WireConsumableTrapTrigger()` |
| `api/proto/game/v1/game.proto` | Add `DeployTrapRequest` + `ClientMessage` field |
| `internal/game/command/commands.go` | Add `deploy_trap` command |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeDeployTrap` handler |
| `content/traps/*.yaml` | Add `trigger_range_ft` + `blast_radius_ft` to all 5 trap YAMLs |
| `content/items/deployable_traps.yaml` | New: 5 deployable trap item definitions |

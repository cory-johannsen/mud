# Consumable Traps Design

## Overview

Consumable traps are player-deployable trap items that reference existing `TrapTemplate` definitions. Players carry them in their backpack, deploy them via a `deploy` command, and the trap arms in the room at the player's current combat position. In combat, the trap fires when a combatant moves within trigger range of the deploy position; outside combat, it fires on the next room entry. All consumable traps are one-shot regardless of the template's `reset_mode`.

A deployed trap is **position-anchored** — it stays at `DeployPosition` regardless of any subsequent movement by the deploying player.

---

## Data Model

### `TrapTemplate` — new fields

| Field | Type | YAML key | Default | Description |
|---|---|---|---|---|
| `TriggerRangeFt` | `int` | `trigger_range_ft` | `5` | Distance in feet at which the trap fires when a combatant moves within range |
| `BlastRadiusFt` | `int` | `blast_radius_ft` | `0` | Radius in feet for AoE payload; `0` = single target (triggering combatant only) |

A zero value for `TriggerRangeFt` is treated as `5` (the default). A zero value for `BlastRadiusFt` means single-target.

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
| `DeployPosition` | `int` | Player's combat position (feet along the 1D combat axis) at deploy time; `0` for world/static traps and out-of-combat deployments |
| `IsConsumable` | `bool` | `true` for player-deployed traps; enforces one-shot behavior regardless of template `reset_mode` |

Thread safety: `DeployPosition` and `IsConsumable` are set once at creation by `AddConsumableTrap` and are read-only thereafter. Callers of `GetTrap` MUST NOT mutate these fields; the `TrapManager` mutex governs the map but not field mutations after insertion.

### `TrapInstanceID` kind values

The `TrapInstanceID(zoneID, roomID, kind, id string) string` helper accepts three valid `kind` values:
- `"room"` — world-defined room-level trap
- `"equip"` — world-defined equipment-level trap
- `"consumable"` — player-deployed consumable trap

A new exported constant `TrapKindConsumable = "consumable"` MUST be added to the `trap` package alongside the existing kind usage.

### `AddConsumableTrap` method signature

```go
// AddConsumableTrap arms a player-deployed consumable trap at the given deploy position.
// Precondition: instanceID is unique within this TrapManager.
// Postcondition: GetTrap(instanceID) returns a state with Armed=true, IsConsumable=true, DeployPosition=deployPos.
func (m *TrapManager) AddConsumableTrap(instanceID string, tmpl *TrapTemplate, deployPos int) error
```

### `ItemDef` — new field and kind

`ItemDef` gains `TrapTemplateRef string` (YAML: `trap_template_ref`). Valid only when `Kind == "trap"`. The `Registry` MUST validate that `TrapTemplateRef` is non-empty when `Kind == "trap"` at registration time; a missing ref is a fatal load error.

New item kind: `"trap"`.

### New content: `content/items/deployable_traps.yaml`

Six deployable trap item definitions, one per trap template:

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

- id: deployable_pressure_plate_mine
  name: Pressure Plate Mine
  kind: trap
  trap_template_ref: pressure_plate_mine
  weight: 2.5
  stackable: true
  max_stack: 3
  value: 350
```

---

## Deploy Command Flow

### Proto

```proto
message DeployTrapRequest {
    string item_name = 1;
}
// Added to ClientMessage.payload oneof at field 85
DeployTrapRequest deploy_trap = 85;
```

### Command registration

- Handler key: `deploy_trap`
- Alias: `deploy`
- Category: `combat`
- Help text: `deploy <item> — arm a trap item at your current position (1 AP in combat)`

### Player combat position

The player's combat position is retrieved via `s.combatH.CombatantPosition(uid string) int`, a new method on `CombatHandler` that looks up the `Combatant` for the given UID in the active combat and returns its `Position` field. Returns `0` if no combat is active or the combatant is not found.

### Handler logic (`handleDeployTrap`)

**Preconditions:**
- Player session exists
- Named item is in backpack with `kind == "trap"`

**In combat:**
1. Verify player has ≥1 AP; return `"Not enough AP to deploy a trap."` if not
2. Spend 1 AP
3. Find item in backpack by name; verify `Kind == "trap"`
4. Look up `TrapTemplate` by `ItemDef.TrapTemplateRef`; return error message if not found
5. Remove 1 from item stack in backpack
6. Get player position: `deployPos = s.combatH.CombatantPosition(uid)`
7. Generate `instanceID = TrapInstanceID(zoneID, roomID, TrapKindConsumable, newUUID())`
8. Call `trapMgr.AddConsumableTrap(instanceID, tmpl, deployPos)`
9. Push message: `"You arm a [item name] at your position."`

**Outside combat:**
- Steps 3–4, then 5, then skip 6 (deployPos = 0), then 7–9
- Push message: `"You arm a [item name] here."`

**Error cases:**
- Item not found in backpack: `"You don't have a [item name]."`
- Item found but not a trap: `"You can't deploy that."`
- Template not found: log error; return `"That trap is broken — contact an admin."`

**Limitation (known, out of scope):** A combatant already within `TriggerRangeFt` of `DeployPosition` at the moment of deployment will not trigger the trap — the positional check only runs on movement. This is acceptable; players must deploy traps in anticipation of enemy movement.

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
activeCombat = s.combatH.GetCombat(roomID)
movedCombatant = activeCombat.CombatantByID(movedCombatantID)
if movedCombatant == nil: return

for each instanceID in trapMgr.TrapsForRoom(zone.ID, room.ID):
    instance = trapMgr.GetTrap(instanceID)
    if !instance.Armed || !instance.IsConsumable: continue
    tmpl = trapTemplates[instance.TemplateID]
    if tmpl == nil: continue

    dist = abs(movedCombatant.Position - instance.DeployPosition)
    if dist > effectiveTriggerRange(tmpl): continue

    // Multiple overlapping consumable traps all fire independently in loop order.
    triggerRange := effectiveTriggerRange(tmpl)  // max(tmpl.TriggerRangeFt, 5) if zero
    affected = collectAffectedCombatants(activeCombat, instance.DeployPosition, tmpl.BlastRadiusFt)
    for each target in affected:
        fireConsumableTrapOnCombatant(target, tmpl, instanceID, dangerLevel)
    trapMgr.Disarm(instanceID)  // always one-shot
```

Multiple overlapping consumable traps all fire independently — no "first trap wins" rule. All traps whose `TriggerRangeFt` is satisfied by the mover's position fire on the same movement action.

### `fireConsumableTrapOnCombatant`

A new internal function (separate from the existing `fireTrap`) that applies a trap's payload to a single `*combat.Combatant` target, which may be either a player or an NPC.

```go
func (s *GameServiceServer) fireConsumableTrapOnCombatant(
    target *combat.Combatant,
    tmpl *trap.TrapTemplate,
    instanceID, dangerLevel string,
) {
    result := trap.ResolveTrigger(tmpl, dangerLevel, s.trapTemplates)
    dmg := s.dice.RollExpr(result.DamageFormula)  // 0 if formula empty
    target.ApplyDamage(dmg)
    // Apply condition if result.ConditionID != "" and target is a player:
    if sess, ok := s.sessions.Get(target.ID); ok {
        // apply condition via condRegistry (player)
        pushMessage(sess, fmt.Sprintf("A %s triggers!", tmpl.Name))
    } else {
        // NPC: damage only via target.ApplyDamage — no condition application to NPCs in this feature
        // NPC message broadcast to room players via s.sessions.AllInRoom(roomID)
    }
}
```

**NPC targets:** Damage is applied via `combat.Combatant.ApplyDamage(amount int)`. Conditions are not applied to NPCs in this feature (NPC condition application is out of scope). All players in the room receive a message: `"A [trap name] catches [NPC name]!"`.

**Player targets:** Damage and condition application follow the same path as the existing `fireTrap` `applyToPlayer` internal logic.

The existing `fireTrap` function is **not modified**. `fireConsumableTrapOnCombatant` is a new function specific to the consumable trap combat path, avoiding any interaction with `fireTrap`'s internal `result.AoE` fan-out.

---

## Out-of-Combat Behavior

Consumable traps deployed outside combat use `TriggerEntry` semantics. `checkEntryTraps` already iterates all armed traps in the room. Since `IsConsumable == true` traps call `trapMgr.Disarm(instanceID)` in `fireTrap` after firing (the existing one-shot path via `ResetMode == ResetOneShot`), out-of-combat deployments behave identically to one-shot room traps without any change to `checkEntryTraps`.

Out-of-combat deployed consumable traps fire against players who enter the room (using the existing player-targeting path in `fireTrap`). Firing against NPCs that enter the room out-of-combat is out of scope.

---

## Requirements

- REQ-CTR-1: A `deploy <item>` command MUST exist, costing 1 AP in combat and 0 AP outside combat.
- REQ-CTR-2: The `deploy` command MUST only accept items with `kind == "trap"`.
- REQ-CTR-3: Deploying a trap MUST remove exactly 1 unit from the item stack in the player's backpack.
- REQ-CTR-4: A deployed trap MUST be armed at the player's current combat position in combat, or at position 0 outside combat.
- REQ-CTR-5: A deployed trap MUST be position-anchored — subsequent movement by the deploying player MUST NOT change the trap's `DeployPosition`.
- REQ-CTR-6: In combat, a deployed trap MUST fire when a combatant moves within `TriggerRangeFt` feet of `DeployPosition`.
- REQ-CTR-7: Multiple overlapping consumable traps MUST all fire independently when a combatant enters their trigger range on the same movement action.
- REQ-CTR-8: A consumable trap with `BlastRadiusFt == 0` MUST apply its payload to the triggering combatant only.
- REQ-CTR-9: A consumable trap with `BlastRadiusFt > 0` MUST apply its payload to all combatants within `BlastRadiusFt` feet of `DeployPosition` at trigger time, including the deploying player if in range.
- REQ-CTR-10: A consumable trap MUST be one-shot — it MUST be disarmed immediately after firing regardless of the template's `reset_mode`.
- REQ-CTR-11: A consumable trap deployed outside combat MUST fire on the next room entry that meets the trigger conditions.
- REQ-CTR-12: Deployed consumable traps MUST be detectable and disarmable via the existing `disarm` command.
- REQ-CTR-13: `TrapTemplate` MUST expose `trigger_range_ft` (default 5) and `blast_radius_ft` (default 0) fields.
- REQ-CTR-14: `ItemDef` MUST support `kind: trap` with a `trap_template_ref` field referencing a `TrapTemplate` ID.
- REQ-CTR-15: The `trap` package MUST export a `TrapKindConsumable = "consumable"` constant.
- REQ-CTR-16: `TrapManager` MUST provide `AddConsumableTrap(instanceID string, tmpl *TrapTemplate, deployPos int) error`.
- REQ-CTR-17: `CombatHandler` MUST provide `CombatantPosition(uid string) int` returning the combatant's current position in feet, or 0 if not in combat.

---

## Files Changed

| File | Change |
|---|---|
| `internal/game/trap/template.go` | Add `TriggerRangeFt`, `BlastRadiusFt` fields |
| `internal/game/trap/manager.go` | Add `DeployPosition`, `IsConsumable` to `TrapInstanceState`; add `AddConsumableTrap`; add `TrapKindConsumable` constant |
| `internal/game/inventory/item.go` | Add `TrapTemplateRef string`; add `"trap"` kind; validate `TrapTemplateRef` non-empty at registration |
| `internal/gameserver/combat_handler.go` | Add `onCombatantMoved` callback + `SetOnCombatantMoved` setter; add `CombatantPosition(uid) int`; call callback after Stride/Step/Shove |
| `internal/gameserver/grpc_service_trap.go` | Add `WireConsumableTrapTrigger()`, `checkConsumableTraps()`, `fireConsumableTrapOnCombatant()` |
| `internal/gameserver/grpc_service_deploy_trap.go` | New: `handleDeployTrap` handler |
| `internal/gameserver/grpc_service_deploy_trap_test.go` | New: deploy command tests |
| `internal/gameserver/grpc_service.go` | Wire deploy dispatch + `WireConsumableTrapTrigger()` |
| `api/proto/game/v1/game.proto` | Add `DeployTrapRequest`; add `deploy_trap = 85` to `ClientMessage.payload` oneof |
| `internal/game/command/commands.go` | Add `deploy_trap` command with alias `deploy` |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeDeployTrap` handler |
| `content/traps/*.yaml` | Add `trigger_range_ft` + `blast_radius_ft` to all 6 trap YAMLs |
| `content/items/deployable_traps.yaml` | New: 6 deployable trap item definitions |
| `docs/features/consumable-traps.md` | Update requirements to reflect approved design (out-of-combat allowed, positional trigger) |

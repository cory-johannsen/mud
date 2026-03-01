# Weapon and Armor Library — Design Document

**Date:** 2026-02-28
**Status:** Approved

---

## Overview

Expand the existing item/equipment system from 8 weapons and no armor content to a comprehensive library of ~75 lore-friendly items spanning two tiers (street-level and military/corporate surplus). All items fit the post-apocalyptic cyberpunk aesthetic. Each item may carry a team affinity (Gun or Machete) with optional mechanical side effects when equipped by the opposing team.

---

## Architecture

A new `ArmorDef` struct (parallel to `WeaponDef`) holds per-slot armor stats. `SlottedItem` continues to reference only a def ID — no struct changes needed. `Equipment.ComputedDefenses` aggregates all equipped slots into a `DefenseStats` value. The `Registry` gains `RegisterArmor` / `Armor(id)` / `AllArmors()`. Both `ArmorDef` and `WeaponDef` gain `TeamAffinity` and `CrossTeamEffect` fields. Cross-team effects are applied transiently at equip time via the existing condition system and recomputed on login.

---

## Components

### ArmorDef (`internal/game/inventory/armor.go`)

New struct loaded from `content/armor/<id>.yaml`:

```go
type ArmorDef struct {
    ID            string
    Name          string
    Description   string
    Slot          ArmorSlot
    ACBonus       int
    DexCap        int     // max dex modifier applied to AC
    CheckPenalty  int     // negative; applied to STR/DEX skill checks
    SpeedPenalty  int     // feet reduction (0, -5, -10)
    StrengthReq   int     // below this STR, speed penalty applies regardless
    Bulk          int
    Group         string  // leather/chain/composite/plate
    Traits        []string
    TeamAffinity  string  // "gun", "machete", or ""
    CrossTeamEffect *CrossTeamEffect
}

type CrossTeamEffect struct {
    Kind  string // "condition" or "penalty"
    Value string // condition ID (e.g. "clumsy-1") or penalty magnitude (e.g. "-2")
}
```

YAML schema example:
```yaml
id: tactical_vest
name: Tactical Vest
description: Salvaged military-grade composite plating strapped over a kevlar underlayer.
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 14
bulk: 2
group: composite
traits: [gun-team]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

### WeaponDef extensions (`internal/game/inventory/weapon.go`)

`WeaponDef` gains two new optional fields:
```go
TeamAffinity     string
CrossTeamEffect  *CrossTeamEffect
```

### DefenseStats (`internal/game/inventory/armor.go`)

```go
type DefenseStats struct {
    ACBonus      int
    EffectiveDex int // min(dexMod, strictest DexCap across equipped slots)
    CheckPenalty int // sum of all slot check penalties
    SpeedPenalty int // sum, applied only if STR < StrengthReq
    StrengthReq  int // max strength requirement across all slots
}
```

### Equipment.ComputedDefenses (`internal/game/inventory/equipment.go`)

Pure function — no side effects, fully testable:

```go
func (e *Equipment) ComputedDefenses(dexMod int) DefenseStats
```

Rules:
- `ACBonus` = sum of `ac_bonus` across all equipped armor slots
- `EffectiveDex` = `min(dexMod, min(DexCap) across equipped slots)`; if no slots equipped, = `dexMod`
- `CheckPenalty` = sum of all `CheckPenalty` values (negative values accumulate)
- `SpeedPenalty` = sum of `SpeedPenalty` values, applied only when character STR < `StrengthReq`
- `StrengthReq` = max `strength_req` across all equipped slots
- Total AC (computed by caller) = 10 + ACBonus + EffectiveDex

### Registry additions (`internal/game/inventory/registry.go`)

```go
RegisterArmor(def *ArmorDef)
Armor(id string) (*ArmorDef, bool)
AllArmors() []*ArmorDef
```

### Team Affinity Check (`internal/gameserver/grpc_service.go`)

At equip time for both weapons and armor:
1. Look up def from registry
2. If `TeamAffinity != ""` and player team != `TeamAffinity` and `CrossTeamEffect != nil`:
   - `kind: condition` → apply named condition via existing condition system
   - `kind: penalty` → add flat check penalty to session
3. On unequip: remove any applied cross-team effect
4. On session load: recompute from all currently equipped items

---

## Content Plan

### Weapons (~40 total)

| Category | Count | Team Affinity |
|----------|-------|---------------|
| Melee — Machete-team | 8 | machete |
| Melee — Gun-team | 4 | gun |
| Pistols | 6 | gun |
| SMGs | 4 | gun |
| Shotguns | 4 | gun |
| Rifles | 6 | gun |
| Explosives | 4 | none |
| Exotic | 4 | none |

**Melee (Machete-team):** machete (existing → add affinity), vibroblade, mono-wire whip, chainsaw, rebar club, tactical hatchet, ceramic shiv, cleaver

**Melee (Gun-team):** combat knife, spiked knuckles, stun baton, bayonet

**Pistols:** ganger pistol (existing), holdout derringer, heavy revolver, smartgun, flechette pistol, EMP pistol

**SMGs:** street sweeper SMG, corp security SMG, suppressed SMG, cyber-linked SMG

**Shotguns:** combat shotgun (existing), sawn-off, riot shotgun, flechette spreader

**Rifles:** assault rifle, sniper rifle, battle rifle, laser rifle, railgun carbine, anti-materiel rifle

**Explosives:** frag grenade (existing), pipe bomb (existing), incendiary grenade, EMP grenade

**Exotic:** flamethrower, net launcher, sonic disruptor, grapple gun

### Armor (~35 total)

| Slot | Count | Notes |
|------|-------|-------|
| Head | 5 | ballistic cap, combat helmet, riot visor, corp security helm, neural interface helm |
| Torso | 6 | leather jacket, tactical vest, kevlar vest, corp suit liner, military plate, exo-frame torso |
| Left Arm | 4 | arm guards, tactical vambrace, ballistic sleeve, exo-frame arm |
| Right Arm | 4 | same variants as left arm |
| Hands | 4 | fingerless gloves, tactical gloves, armored gauntlets, shock gloves |
| Left Leg | 4 | leg guards, tactical greaves, ballistic leggings, exo-frame leg |
| Right Leg | 4 | same variants as left leg |
| Feet | 4 | street boots, tactical boots, mag-boots, exo-frame feet |

---

## Data Flow

1. Server starts → `Registry` loads all YAML files from `content/armor/` alongside existing `content/weapons/` and `content/items/`
2. Player equips item → `handleEquip` resolves def, calls affinity check, applies `CrossTeamEffect` if mismatched
3. Combat damage calculation → calls `Equipment.ComputedDefenses(dexMod)` to get total AC
4. Player unequips → cross-team effect removed from session
5. Player logs in → all equipped items reloaded from DB, affinity effects reapplied

---

## Error Handling

- Unknown armor def ID at equip: return error event to client
- Invalid slot in YAML: registry `RegisterArmor` panics at startup (fail fast)
- `ComputedDefenses` with no equipped slots: returns zero-value `DefenseStats` (no armor = no penalty, no bonus)
- `CrossTeamEffect` with unknown condition ID: logged at warn, effect skipped

---

## Testing

- **`ArmorDef` loading**: table-driven tests — all YAML fields parsed correctly, invalid slot name panics registry
- **`ComputedDefenses`**: property-based test — for any combination of equipped slots, `ACBonus` = sum of individual bonuses, `EffectiveDex` ≤ any single slot's dex cap, `CheckPenalty` = sum
- **`Registry.Armor`**: table-driven — all registered armors retrievable by ID, unknown ID returns `false`
- **Team affinity at equip**: table-driven — matching team = no effect; mismatched + `cross_team_effect` = condition/penalty applied; mismatched + no effect = nothing applied
- **Weapon affinity**: same pattern extended to `WeaponDef`
- **Content completeness**: property test asserting every armor `slot` value is a valid `ArmorSlot`, every `armor_ref` in item YAMLs resolves in registry

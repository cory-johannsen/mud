---
issue: 185
title: Zone difficulty upgrade — scale zones to player level cap of 100
slug: zone-difficulty-scaling
date: 2026-04-19
---

## Summary

All existing zones cap out at level 12, while the player level cap is 100.
This spec defines a five-tier difficulty framework spanning levels 1–100,
assigns every existing zone to a tier, prescribes NPC stat formulas for each
level, and identifies the new zones required to fill the mid-to-high level
gaps.

---

## Difficulty Tier Framework

Five tiers divide the 1–100 level range. Each tier has a name, a level band,
and a minimum-to-maximum NPC stat range that implementers MUST target when
authoring or updating NPC templates.

| Tier | Name | Player Level Band | Zone Danger Level Range |
|---|---|---|---|
| 1 | Desperate Streets | 1 – 15 | safe → dangerous |
| 2 | Armed & Dangerous | 16 – 35 | sketchy → dangerous |
| 3 | Warlord Territory | 36 – 60 | dangerous → all_out_war |
| 4 | Apex Predator | 61 – 80 | dangerous → all_out_war |
| 5 | End Times | 81 – 100 | all_out_war |

---

## NPC Stat Scaling Formula

These formulas apply to `standard` tier NPCs. Multiply by the tier modifier for
higher-tier variants.

| NPC Tier | HP Modifier | XP Modifier | AC Modifier |
|---|---|---|---|
| minion | ×0.6 | ×0.5 | −2 |
| standard | ×1.0 | ×1.0 | ±0 |
| elite | ×1.5 | ×2.0 | +2 |
| champion | ×2.0 | ×3.0 | +4 |
| boss | ×3.0 | ×5.0 | +5 |

### Base Stats by Level

| NPC Level | Max HP | AC | Primary Ability Score | Attacks/Round | Damage (avg) |
|---|---|---|---|---|---|
| 1 | 12 | 14 | 2 | 1 | 4 |
| 5 | 35 | 15 | 3 | 1 | 8 |
| 10 | 70 | 16 | 4 | 1–2 | 12 |
| 15 | 110 | 17 | 5 | 2 | 16 |
| 20 | 160 | 18 | 6 | 2 | 22 |
| 30 | 270 | 19 | 8 | 2 | 32 |
| 40 | 420 | 20 | 10 | 2–3 | 45 |
| 50 | 600 | 21 | 12 | 3 | 60 |
| 60 | 810 | 22 | 14 | 3 | 78 |
| 70 | 1050 | 23 | 16 | 3 | 98 |
| 80 | 1320 | 24 | 18 | 3–4 | 120 |
| 90 | 1620 | 25 | 20 | 4 | 145 |
| 100 | 1950 | 26 | 22 | 4 | 172 |

Interpolate linearly for levels between the anchors above.

### REQ-ZDS-1: NPC Template Compliance
Every NPC template MUST have `level`, `max_hp`, and `ac` set within ±10% of
the formula values for its assigned level. Templates outside this range MUST
be corrected as part of this work.

---

## Zone Tier Assignments

### Tier 1 — Desperate Streets (Levels 1–15)

These zones remain the entry-level content. NPCs MUST be updated to cover the
full 1–15 band rather than stopping at 6–8.

| Zone | Current Max Level | Target Level Band | Boss Level Target |
|---|---|---|---|
| Downtown Portland | 6 | 1 – 10 | 10 |
| Felony Flats | 6 | 1 – 10 | 10 |
| The Couve | 6 | 3 – 12 | 12 |
| Troutdale | 6 | 3 – 12 | 12 |
| NE Portland | 8 | 5 – 15 | 15 |
| PDX International | 7 | 5 – 15 | 15 |
| Battleground | 6 | 3 – 12 | 12 |
| Hillsboro | 7 | 5 – 15 | 15 |

### Tier 2 — Armed & Dangerous (Levels 16–35)

Mid-early content. Most existing Tier 2 zones need heavy NPC upgrades.

| Zone | Current Max Level | Target Level Band | Boss Level Target |
|---|---|---|---|
| Beaverton | 6 | 16 – 25 | 25 |
| Vantucky | (varies) | 16 – 25 | 25 |
| Rustbucket Ridge | 7 | 20 – 30 | 30 |
| Sauvie Island | 7 | 20 – 30 | 30 |
| Colonel Summers Park | 8 | 22 – 32 | 32 |
| Ross Island | 7 | 22 – 32 | 32 |
| SE Industrial | 7 | 25 – 35 | 35 |
| Aloha | 5 | 16 – 22 | 22 |

### Tier 3 — Warlord Territory (Levels 36–60)

Current zones at this tier:

| Zone | Current Max Level | Target Level Band | Boss Level Target |
|---|---|---|---|
| Oregon Country Fair | 12 | 36 – 50 | 50 |
| Wooklyn | 10 | 36 – 48 | 48 |
| SteamPDX | 7 | 40 – 55 | 55 |
| Club Privata | 8 | 40 – 55 | 55 |
| The Velvet Rope | 7 | 40 – 55 | 55 |
| Clown Camp | 7 | 45 – 60 | 60 |
| Lake Oswego | 7 | 45 – 60 | 60 |

### Tier 4 — Apex Predator (Levels 61–80)

**No existing zones cover this tier.** Three new zones MUST be created.

| Zone (New) | Target Level Band | Boss Level Target | Suggested Theme |
|---|---|---|---|
| Mount Hood Stronghold | 61 – 70 | 70 | Militia bunker complex, radiation zones |
| The Willamette Wastelands | 65 – 75 | 75 | River faction war, post-flood ruins |
| The Portland Heights Citadel | 70 – 80 | 80 | Former wealthy enclave now a warlord fortress |

### Tier 5 — End Times (Levels 81–100)

**No existing zones cover this tier.** Two new zones MUST be created.

| Zone (New) | Target Level Band | Boss Level Target | Suggested Theme |
|---|---|---|---|
| The Exclusion Zone | 81 – 92 | 92 | Quarantined bio-hazard district, mutant enemies |
| The Vault (End-Game) | 93 – 100 | 100 | Pre-collapse government installation; final boss |

---

## Per-Zone Change Requirements

### REQ-ZDS-2: Existing Zone NPC Upgrades
For each zone listed in Tiers 1–3, implementers MUST:

- REQ-ZDS-2a: Update all standard NPC templates to have `level` within the
  zone's target level band.
- REQ-ZDS-2b: Update `max_hp` and `ac` to match the formula at the NPC's new
  level (within ±10%).
- REQ-ZDS-2c: Update the zone's `min_level` / `max_level` metadata to match
  the target band.

### REQ-ZDS-3: Boss NPC Upgrades
For each zone's boss NPC, implementers MUST:

- REQ-ZDS-3a: Set boss NPC `level` to the Boss Level Target for that zone.
- REQ-ZDS-3b: Set `tier: boss`.
- REQ-ZDS-3c: Ensure `max_hp` = formula value × 3.0 (boss modifier).
- REQ-ZDS-3d: Add or update `boss_abilities` with at minimum: one AOE attack,
  one self-heal or damage-reduction ability, one phase-change ability (triggered
  at ≤50% HP).

### REQ-ZDS-4: Danger Level Calibration
Room `danger_level` values MUST be updated to reflect the zone's tier:

- REQ-ZDS-4a: Tier 1 zones MUST use `safe` / `sketchy` for normal rooms and
  `dangerous` for boss approach rooms.
- REQ-ZDS-4b: Tier 2 zones MUST use `sketchy` / `dangerous` for normal rooms
  and `all_out_war` for boss rooms.
- REQ-ZDS-4c: Tier 3+ zones MUST use `dangerous` for normal rooms and
  `all_out_war` for boss rooms.

### REQ-ZDS-5: New Zone Requirements (Tiers 4 & 5)
Each new zone MUST conform to the existing zone YAML structure and MUST include:

- REQ-ZDS-5a: A minimum of 15 rooms.
- REQ-ZDS-5b: At least 3 NPC templates (minion, standard, elite) exclusive to
  the zone.
- REQ-ZDS-5c: One boss room (`boss_room: true`) with a boss NPC at the Boss
  Level Target.
- REQ-ZDS-5d: A zone map item placed in a discoverable room.
- REQ-ZDS-5e: A quest giver NPC offering at least one zone-specific quest.
- REQ-ZDS-5f: A fixer, motel keeper, or black market merchant for player
  services.
- REQ-ZDS-5g: `world_x` / `world_y` coordinates that place the zone
  geographically plausible on the world map.

### REQ-ZDS-6: Loot Table Scaling
- REQ-ZDS-6a: NPC templates at level ≥ 20 MUST have `rob_multiplier ≥ 1.2`.
- REQ-ZDS-6b: NPC templates at level ≥ 50 MUST have `rob_multiplier ≥ 1.5`.
- REQ-ZDS-6c: Boss NPCs MUST have `rob_multiplier ≥ 2.0`.

### REQ-ZDS-7: XP Reward Validation
- REQ-ZDS-7a: The existing XP formula (`npc_level × 50 × tier_multiplier`) MUST
  be verified to produce meaningful progression at all levels 1–100.
- REQ-ZDS-7b: If the formula produces degenerate rewards at high levels (too
  fast or too slow), a corrective coefficient MUST be introduced to the XP
  config rather than hardcoding per-NPC XP values.

---

## Implementation Order

This is large content work. The recommended delivery order is:

1. **Tier 1 zone upgrades** — fixes low-level content first, immediate player impact.
2. **Tier 2 zone upgrades** — mid-early content, follows naturally.
3. **Tier 3 zone upgrades** — existing exotic zones scaled to mid-game.
4. **Tier 4 new zones** — first new zone pair (must be spec'd independently before implementation).
5. **Tier 5 new zones** — end-game content (must be spec'd independently before implementation).

### REQ-ZDS-8: Incremental Delivery
Each tier's zone upgrades MUST be delivered and tested independently. A zone
MUST NOT be committed until:
- REQ-ZDS-8a: All NPC templates pass the stat formula check (REQ-ZDS-1).
- REQ-ZDS-8b: The zone map is connected to the world (no disconnected rooms).
- REQ-ZDS-8c: The boss room is reachable and the boss spawns correctly.

---

## Out of Scope

- Changing the XP formula itself (REQ-ZDS-7b only adds a coefficient if needed).
- Rewriting the danger level engine.
- Adding new item/equipment types for high-level loot (tracked separately).
- UI changes for difficulty display.

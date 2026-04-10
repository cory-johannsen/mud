# Plan: The Barrel House and The Keg Hole — Dangerous Zone Upgrade

**GitHub Issue:** cory-johannsen/mud#34
**Spec:** `docs/superpowers/specs/2026-04-10-barrel-house-keg-hole-dangerous.md`
**Date:** 2026-04-10

## Note: Spec File Path Correction

The spec references `content/npcs/combat/rustbucket_ridge.yaml` but that directory does not exist. All combat NPCs are individual files directly in `content/npcs/`. New templates will be created as `content/npcs/barrel_house_enforcer.yaml` and `content/npcs/big_grizz.yaml`.

---

## Step 1 — Create `barrel_house_enforcer` NPC template (REQ-1c, REQ-1d, REQ-1e)

**File:** `content/npcs/barrel_house_enforcer.yaml` (new file)

Modelled on the level-2 baseline (`rail_gang_raider`: HP 22, AC 13). Narrative: Pete's hired muscle, a rough distillery enforcer.

```yaml
id: barrel_house_enforcer
name: Barrel House Enforcer
description: A thick-necked bruiser in a stained work apron, wielding a length of pipe. Pete pays well for loyalty—and for cracking skulls.
level: 2
max_hp: 18
ac: 13
awareness: 6
ai_domain: territory_patrol
respawn_delay: "5m"
abilities:
  brutality: 16
  quickness: 11
  grit: 14
  reasoning: 8
  savvy: 9
  flair: 7
loot:
  currency:
    min: 15
    max: 50
  items:
    - item: scrap_metal
      chance: 0.5
      min_qty: 1
      max_qty: 3
    - item: medkit
      chance: 0.15
      min_qty: 1
      max_qty: 1
weapon:
  - id: steel_pipe
    weight: 3
  - id: cheap_blade
    weight: 2
armor:
  - id: leather_jacket
    weight: 3
  - id: tactical_vest
    weight: 1
taunts:
  - "This place ain't for tourists."
  - "Pete don't like visitors."
  - "You picked the wrong distillery to wander into."
  - "Last guy who snooped around here is still in one of those barrels."
```

---

## Step 2 — Create `big_grizz` Boss NPC template (REQ-2c, REQ-2d, REQ-2e, REQ-2f)

**File:** `content/npcs/big_grizz.yaml` (new file)

Modelled on the level-4 baseline (`gang_enforcer`: HP 40, AC 15) but heavier. Narrative: former bar bouncer, massive, runs The Keg Hole.

```yaml
id: big_grizz
name: Big Grizz
description: A mountain of a man with arms like industrial pistons and a face that's absorbed more punches than most fighters throw in a lifetime. He runs The Keg Hole with iron fists and doesn't need a weapon to make his point—though he keeps a length of rebar behind the bar just in case.
level: 4
max_hp: 50
ac: 15
awareness: 7
is_boss: true
ai_domain: territory_patrol
respawn_delay: "15m"
abilities:
  brutality: 18
  quickness: 10
  grit: 16
  reasoning: 9
  savvy: 11
  flair: 10
loot:
  currency:
    min: 80
    max: 200
  items:
    - item: medkit
      chance: 0.4
      min_qty: 1
      max_qty: 2
    - item: scrap_metal
      chance: 0.6
      min_qty: 2
      max_qty: 5
weapon:
  - id: rebar_club
    weight: 4
  - id: steel_pipe
    weight: 1
armor:
  - id: kevlar_vest
    weight: 3
  - id: leather_jacket
    weight: 2
taunts:
  - "You've got nerve walking in here."
  - "I've thrown out bigger than you."
  - "This is my bar. My rules."
  - "Last person who started trouble here woke up in the gutter three blocks away."
  - "Grizz don't ask twice."
```

**Note:** Verify that `is_boss: true` is a recognized field in the NPC YAML loader (`internal/game/npc/` or equivalent). If not present, add support for it in the loader and NPC struct before this step is complete.

---

## Step 3 — Update `the_barrel_house` in zone YAML (REQ-1a, REQ-1b, REQ-1e)

**File:** `content/zones/rustbucket_ridge.yaml` (lines 576-611)

Changes:
- `danger_level: sketchy` → `danger_level: dangerous`
- Replace single `scavenger` spawn with 2x `barrel_house_enforcer`

```yaml
  danger_level: dangerous
  spawns:
  - template: barrel_house_enforcer
    count: 2
    respawn_after: 5m
```

---

## Step 4 — Update `the_keg_hole` in zone YAML (REQ-2a, REQ-2b, REQ-2g)

**File:** `content/zones/rustbucket_ridge.yaml` (lines 648-681)

Changes:
- `danger_level: sketchy` → `danger_level: dangerous`
- Replace single `ganger` spawn with 1x `barrel_house_enforcer` + 1x `big_grizz`

```yaml
  danger_level: dangerous
  spawns:
  - template: barrel_house_enforcer
    count: 1
    respawn_after: 5m
  - template: big_grizz
    count: 1
    respawn_after: 15m
```

---

## Step 5 — Run tests and verify content loads

```bash
mise exec -- go test ./internal/... -count=1 -run "NPC\|Zone\|Content\|Spawn"
mise exec -- go test ./... -count=1
```

Verify:
- Both new NPC templates load without error
- Zone spawns for `the_barrel_house` and `the_keg_hole` reference valid templates
- No adjacent rooms were modified (REQ-3a)

---

## Dependency Order

```
Step 1 (barrel_house_enforcer NPC) ──┬──▶ Step 3 (Barrel House zone)
Step 2 (big_grizz NPC)              ─┤──▶ Step 4 (Keg Hole zone)
                                      └──▶ Step 5 (test suite)
Steps 3 + 4 ──▶ Step 5
```

Steps 1 and 2 are independent and can run in parallel.
Steps 3 and 4 are independent and can run in parallel after their respective NPC files exist.

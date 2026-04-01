# Oregon Country Fair Zone Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Oregon Country Fair zone (Veneta, OR) — 34 rooms across three faction territories (Wooks/Juggalos/Tweakers) and neutral contested space; two new factions (juggalos, tweakers); 9 new NPC types; tweaker_crystal substance; three bosses; faction quest givers.

**Architecture:** Primarily YAML content (zone, NPCs, substances, factions). Two small Go additions: juggalette Faygo splash on-hit item effect (new item effect type), tweaker_paranoid immediate call-for-help behavior. Wooks faction YAML updated to set zone_id to "". All three faction safe clusters have quest_giver stub NPCs.

**Tech Stack:** Go, YAML, existing NPC/zone/substance/faction/HTN packages

**Prerequisites:** wooklyn plan must be implemented first (provides AmbientSubstance room field and ticker extension, wooks faction YAML, wook_spore substance).

**Implementation Order Note:** map-poi → non-human-npcs → npc-behaviors → advanced-enemies → factions → wooklyn → **oregon-country-fair**.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `content/substances/tweaker_crystal.yaml` | Create | tweaker_crystal stimulant substance definition (REQ-OCF-27) |
| `content/factions/juggalos.yaml` | Create | Juggalos faction: 4 tiers, exclusive items, hostile factions (REQ-OCF-29/30/31) |
| `content/factions/tweakers.yaml` | Create | Tweakers faction: 4 tiers, exclusive items, hostile factions (REQ-OCF-32/33/34) |
| `content/factions/wooks.yaml` | Modify | Set zone_id: ""; add kill_juggalo/kill_tweaker rep_sources (REQ-OCF-35, REQ-OCF-36) |
| `content/npcs/juggalo.yaml` | Create | Base Juggalo combat NPC (REQ-OCF-14) |
| `content/npcs/juggalette.yaml` | Create | Juggalette with faygo_splash on-hit (REQ-OCF-15) |
| `content/npcs/juggalo_prophet.yaml` | Create | Elder Juggalo with HTN say operator (REQ-OCF-16) |
| `content/npcs/violent_jimmy.yaml` | Create | Juggalo boss: faygo_bomb/hatchet_dance/dark_carnival_prayer (REQ-OCF-17) |
| `content/npcs/tweaker.yaml` | Create | Base Tweaker: high Quickness, frenetic double-attack (REQ-OCF-18) |
| `content/npcs/tweaker_paranoid.yaml` | Create | Paranoid Tweaker: immediate call_for_help on room entry (REQ-OCF-19) |
| `content/npcs/tweaker_cook.yaml` | Create | Tweaker Cook: applies tweaker_crystal on hit (REQ-OCF-20) |
| `content/npcs/crystal_karen.yaml` | Create | Tweaker boss: paranoid_burst/meth_bomb/speed_rush (REQ-OCF-21) |
| `content/npcs/spiral_king.yaml` | Create | Wook boss: eternal_groove/spore_cloud/spiral_vision (REQ-OCF-22) |
| `content/ai/juggalo_prophet_combat.yaml` | Create | HTN domain with say operator and ICP scripture lines (REQ-OCF-16) |
| `content/ai/tweaker_paranoid_combat.yaml` | Create | HTN domain with immediate call_for_help operator (REQ-OCF-19) |
| `content/zones/oregon_country_fair.yaml` | Create | 34-room zone: all territories, ambient_substance, boss rooms, hazards (REQ-OCF-1–13) |
| `internal/gameserver/grpc_service_quest_giver.go` | Create | Quest giver stub handler returning "time isn't right yet" message (REQ-OCF-39) |
| `internal/gameserver/grpc_service_quest_giver_test.go` | Create | TDD + property-based tests for quest giver handler (REQ-OCF-39) |
| `internal/gameserver/oregon_country_fair_integration_test.go` | Create | Integration tests for OCF zone loading and faction cross-validation |
| `docs/features/index.yaml` | Modify | Set oregon-country-fair status: planned |

---

## Task 1: tweaker_crystal Substance YAML

**Files:**
- Create: `content/substances/tweaker_crystal.yaml`

**REQ coverage:** REQ-OCF-27

- [ ] **Step 1.1: Verify SubstanceRegistry and YAML format from wooklyn plan**

```bash
ls /home/cjohannsen/src/mud/content/substances/
```

Expected: `wook_spore.yaml` (created by wooklyn plan) plus other substance files. Confirm YAML schema matches `internal/game/substance/definition.go`.

- [ ] **Step 1.2: Create `content/substances/tweaker_crystal.yaml`**

```yaml
id: tweaker_crystal
name: "Tweaker Crystal"
category: stimulant
onset_delay: 0s
duration: 300s
recovery_duration: 120s
addiction_potential: high
addiction_chance: 0.35
overdose_threshold: 4
effects:
  - attribute: quickness
    modifier: 4
  - attribute: reasoning
    modifier: -3
  - attribute: savvy
    modifier: -2
withdrawal_effects:
  - attribute: quickness
    modifier: -2
  - attribute: grit
    modifier: -1
```

- [ ] **Step 1.3: Verify startup validation**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i tweaker_crystal
```

Expected: no errors — `tweaker_crystal` loads from SubstanceRegistry at startup.

---

## Task 2: Juggalos Faction YAML

**Files:**
- Create: `content/factions/juggalos.yaml`

**REQ coverage:** REQ-OCF-29, REQ-OCF-30, REQ-OCF-31

- [ ] **Step 2.1: Create `content/factions/juggalos.yaml`**

```yaml
id: juggalos
name: "The Juggalos"
zone_id: ""
hostile_factions:
  - tweakers
  - wooks
tiers:
  - id: normie
    label: "Normie"
    min_rep: 0
    price_discount: 0.0
  - id: down
    label: "Down with the Clown"
    min_rep: 10
    price_discount: 0.05
  - id: wicked
    label: "Wicked"
    min_rep: 25
    price_discount: 0.15
  - id: family
    label: "Juggalo Family"
    min_rep: 50
    price_discount: 0.25
exclusive_items:
  - tier_id: family
    item_ids:
      - hatchet_man_pendant
      - faygo_grenade
      - icp_mixtape
rep_sources:
  - source: kill_tweaker
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: wicked
  - source: kill_wook
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: wicked
  - source: quest_completion
    rep_per_completion: 15
  - source: fixer_payment
    rep_per_payment: 10
```

- [ ] **Step 2.2: Validate faction YAML loads without error**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i juggalo
```

Expected: no errors.

---

## Task 3: Tweakers Faction YAML

**Files:**
- Create: `content/factions/tweakers.yaml`

**REQ coverage:** REQ-OCF-32, REQ-OCF-33, REQ-OCF-34

- [ ] **Step 3.1: Create `content/factions/tweakers.yaml`**

```yaml
id: tweakers
name: "The Tweakers"
zone_id: ""
hostile_factions:
  - juggalos
  - wooks
tiers:
  - id: paranoid_stranger
    label: "Paranoid Stranger"
    min_rep: 0
    price_discount: 0.0
  - id: known
    label: "Known Associate"
    min_rep: 10
    price_discount: 0.05
  - id: trusted
    label: "Trusted Tweaker"
    min_rep: 25
    price_discount: 0.15
  - id: inner_circle
    label: "Inner Circle"
    min_rep: 50
    price_discount: 0.30
exclusive_items:
  - tier_id: inner_circle
    item_ids:
      - crystal_shard_pipe
      - speed_rig
      - paranoia_grenade
rep_sources:
  - source: kill_juggalo
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: trusted
  - source: kill_wook
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: trusted
  - source: quest_completion
    rep_per_completion: 15
  - source: fixer_payment
    rep_per_payment: 10
```

- [ ] **Step 3.2: Validate faction YAML loads without error**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i tweaker
```

Expected: no errors.

---

## Task 4: Update Wooks Faction zone_id

**Files:**
- Modify: `content/factions/wooks.yaml`

**REQ coverage:** REQ-OCF-35, REQ-OCF-36

- [ ] **Step 4.1: Update `content/factions/wooks.yaml`**

Change the `zone_id` field from `wooklyn` to `""`:

```bash
grep -n "zone_id" /home/cjohannsen/src/mud/content/factions/wooks.yaml
```

Expected: line reading `zone_id: wooklyn`. Edit that line to read `zone_id: ""`.

- [ ] **Step 4.2: Add `kill_juggalo` and `kill_tweaker` rep sources to `content/factions/wooks.yaml`**

Locate the `rep_sources` block in `wooks.yaml` and add two new entries matching the rate of the existing `kill_non_wook` entry:

```yaml
  - source: kill_juggalo
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: trusted
  - source: kill_tweaker
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: trusted
```

This ensures kills of Juggalos and Tweakers in the OCF zone award Wook rep at the same rate as killing non-wooks in Wooklyn (REQ-OCF-36).

- [ ] **Step 4.3: Verify wooks zone still loads correctly**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i wook
```

Expected: no errors. Wooklyn zone continues to load; wooks faction now has empty zone_id and new kill_juggalo/kill_tweaker rep sources.

- [ ] **Step 4.4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```

Expected: all PASS.

---

## Task 5: NPC YAML Files — Juggalo Types

**Files:**
- Create: `content/npcs/juggalo.yaml`
- Create: `content/npcs/juggalette.yaml`
- Create: `content/npcs/juggalo_prophet.yaml`
- Create: `content/npcs/violent_jimmy.yaml`

**REQ coverage:** REQ-OCF-14, REQ-OCF-15, REQ-OCF-16, REQ-OCF-17, REQ-OCF-23, REQ-OCF-44

- [ ] **Step 5.1: Create `content/npcs/juggalo.yaml`**

```yaml
id: juggalo
name: Juggalo
description: A face-painted warrior in torn ICP merch, wielding a hatchet with unhinged enthusiasm.
level: 4
max_hp: 38
ac: 13
awareness: 7
faction_id: juggalos
attack_verb: "swings a hatchet at"
ai_domain: ganger_combat
respawn_delay: "5m"
abilities:
  brutality: 14
  quickness: 12
  grit: 14
  reasoning: 8
  savvy: 9
  flair: 11
weapon:
  - id: hatchet
    weight: 3
  - id: cheap_blade
    weight: 1
armor:
  - id: leather_jacket
    weight: 2
loot:
  currency:
    min: 5
    max: 20
  items:
    - item: faygo_bottle
      chance: 0.30
      min_qty: 1
      max_qty: 1
disposition:
  hostile_below_rep: 10
  faction_id: juggalos
```

- [ ] **Step 5.2: Create `content/npcs/juggalette.yaml`**

```yaml
id: juggalette
name: Juggalette
description: A face-painted Juggalo woman in torn fishnet and ICP merch, arm cocked back with a full Faygo bottle.
level: 4
max_hp: 35
ac: 13
awareness: 8
faction_id: juggalos
attack_verb: "hurls a Faygo bottle at"
ai_domain: ganger_combat
respawn_delay: "5m"
on_hit_effect: faygo_splash
abilities:
  brutality: 12
  quickness: 14
  grit: 12
  reasoning: 9
  savvy: 11
  flair: 13
weapon:
  - id: faygo_bottle
    weight: 3
  - id: cheap_blade
    weight: 1
armor:
  - id: leather_jacket
    weight: 2
loot:
  currency:
    min: 5
    max: 20
  items:
    - item: faygo_bottle
      chance: 0.50
      min_qty: 1
      max_qty: 2
disposition:
  hostile_below_rep: 10
  faction_id: juggalos
```

- [ ] **Step 5.3: Create `content/npcs/juggalo_prophet.yaml`**

```yaml
id: juggalo_prophet
name: "Juggalo Prophet"
description: An elder Juggalo in full ceremonial face paint and a black robe, reciting the gospel of the Dark Carnival.
level: 7
max_hp: 72
ac: 15
awareness: 9
faction_id: juggalos
attack_verb: "raises a hatchet at"
ai_domain: juggalo_prophet_combat
respawn_delay: "10m"
abilities:
  brutality: 16
  quickness: 10
  grit: 18
  reasoning: 12
  savvy: 11
  flair: 14
weapon:
  - id: hatchet
    weight: 3
armor:
  - id: leather_jacket
    weight: 2
  - id: kevlar_vest
    weight: 1
loot:
  currency:
    min: 20
    max: 60
  items:
    - item: faygo_bottle
      chance: 0.50
      min_qty: 1
      max_qty: 2
disposition:
  hostile_below_rep: 10
  faction_id: juggalos
allowed_danger_levels:
  - dangerous
  - all_out_war
```

- [ ] **Step 5.4: Create `content/npcs/violent_jimmy.yaml`**

```yaml
id: violent_jimmy
name: "Violent Jimmy"
description: An enormous face-painted warrior in a velvet robe. The Juggalo boss radiates Dark Carnival authority.
level: 12
max_hp: 160
ac: 18
awareness: 11
faction_id: juggalos
attack_verb: "raises a hatchet at"
ai_domain: ganger_combat
respawn_delay: "72h"
tier: boss
abilities:
  brutality: 20
  quickness: 14
  grit: 20
  reasoning: 11
  savvy: 12
  flair: 16
weapon:
  - id: hatchet
    weight: 1
armor:
  - id: kevlar_vest
    weight: 1
boss_abilities:
  - id: faygo_bomb
    description: "Violent Jimmy hurls a Faygo bomb — AoE acid damage to all in room."
    effect_type: aoe_damage
    damage: "2d6"
    damage_type: acid
    target: room
    cooldown: 3
  - id: hatchet_dance
    description: "Violent Jimmy spins in a hatchet frenzy — attacks twice in one action."
    effect_type: multi_attack
    attack_count: 2
    cooldown: 2
  - id: dark_carnival_prayer
    description: "Violent Jimmy prays to the Dark Carnival — heals self for 2d8."
    effect_type: self_heal
    heal: "2d8"
    cooldown: 999
    max_uses: 1
loot:
  currency:
    min: 200
    max: 500
  items:
    - item: hatchet_man_pendant
      chance: 0.75
      min_qty: 1
      max_qty: 1
    - item: faygo_grenade
      chance: 0.50
      min_qty: 1
      max_qty: 2
allowed_rooms:
  - violent_jimmys_tent
```

---

## Task 6: NPC YAML Files — Tweaker Types

**Files:**
- Create: `content/npcs/tweaker.yaml`
- Create: `content/npcs/tweaker_paranoid.yaml`
- Create: `content/npcs/tweaker_cook.yaml`
- Create: `content/npcs/crystal_karen.yaml`

**REQ coverage:** REQ-OCF-18, REQ-OCF-19, REQ-OCF-20, REQ-OCF-21, REQ-OCF-24, REQ-OCF-44

- [ ] **Step 6.1: Create `content/npcs/tweaker.yaml`**

```yaml
id: tweaker
name: Tweaker
description: A gaunt, manic figure twitching with paranoid energy. Fast, unpredictable, and desperate.
level: 3
max_hp: 22
ac: 12
awareness: 9
faction_id: tweakers
attack_verb: "claws at"
ai_domain: ganger_combat
respawn_delay: "5m"
attacks_per_round: 2
abilities:
  brutality: 10
  quickness: 18
  grit: 10
  reasoning: 7
  savvy: 8
  flair: 8
weapon:
  - id: cheap_blade
    weight: 3
  - id: improvised_weapon
    weight: 2
armor: []
loot:
  currency:
    min: 1
    max: 15
  items:
    - item: scrap_metal
      chance: 0.40
      min_qty: 1
      max_qty: 3
disposition:
  hostile_below_rep: 10
  faction_id: tweakers
```

- [ ] **Step 6.2: Create `content/npcs/tweaker_paranoid.yaml`**

```yaml
id: tweaker_paranoid
name: "Paranoid Tweaker"
description: Eyes darting in every direction, this tweaker screams for backup the moment they spot an intruder.
level: 3
max_hp: 20
ac: 12
awareness: 14
faction_id: tweakers
attack_verb: "screams and swings at"
ai_domain: tweaker_paranoid_combat
respawn_delay: "5m"
on_hit_condition:
  condition_id: paranoid
  attribute_modifier:
    attribute: savvy
    modifier: -2
    duration_rounds: 2
  save:
    attribute: grit
    dc: 14
abilities:
  brutality: 10
  quickness: 16
  grit: 10
  reasoning: 6
  savvy: 7
  flair: 7
weapon:
  - id: cheap_blade
    weight: 3
armor: []
loot:
  currency:
    min: 1
    max: 10
  items:
    - item: scrap_metal
      chance: 0.30
      min_qty: 1
      max_qty: 2
disposition:
  hostile_below_rep: 10
  faction_id: tweakers
```

- [ ] **Step 6.3: Create `content/npcs/tweaker_cook.yaml`**

```yaml
id: tweaker_cook
name: "Tweaker Cook"
description: An older, methodical tweaker in a stained lab coat, managing the operation with lethal precision.
level: 6
max_hp: 55
ac: 14
awareness: 10
faction_id: tweakers
attack_verb: "sprays a cloud at"
ai_domain: ganger_combat
respawn_delay: "10m"
on_hit_substance: tweaker_crystal
abilities:
  brutality: 12
  quickness: 14
  grit: 14
  reasoning: 15
  savvy: 12
  flair: 9
weapon:
  - id: chemical_sprayer
    weight: 1
armor:
  - id: leather_jacket
    weight: 2
loot:
  currency:
    min: 30
    max: 80
  items:
    - item: crystal_shard_pipe
      chance: 0.40
      min_qty: 1
      max_qty: 1
    - item: scrap_metal
      chance: 0.50
      min_qty: 2
      max_qty: 4
disposition:
  hostile_below_rep: 10
  faction_id: tweakers
allowed_danger_levels:
  - dangerous
  - all_out_war
```

- [ ] **Step 6.4: Create `content/npcs/crystal_karen.yaml`**

```yaml
id: crystal_karen
name: "Crystal Karen"
description: A brilliant, terrifying woman in a lab coat covered in chemical burns. She runs the operation. She runs everything.
level: 12
max_hp: 145
ac: 17
awareness: 13
faction_id: tweakers
attack_verb: "hurls chemicals at"
ai_domain: ganger_combat
respawn_delay: "72h"
tier: boss
abilities:
  brutality: 14
  quickness: 20
  grit: 16
  reasoning: 20
  savvy: 16
  flair: 10
weapon:
  - id: chemical_sprayer
    weight: 1
armor:
  - id: kevlar_vest
    weight: 1
boss_abilities:
  - id: paranoid_burst
    description: "Crystal Karen releases paranoid energy — all players make Savvy-DC-16 save or lose 1 AP next round."
    effect_type: aoe_condition
    condition_id: paranoid
    ap_loss: 1
    save:
      attribute: savvy
      dc: 16
    target: room_players
    cooldown: 3
  - id: meth_bomb
    description: "Crystal Karen detonates a meth bomb — immediate tweaker_crystal dose to all players in room."
    effect_type: aoe_substance
    substance_id: tweaker_crystal
    target: room_players
    cooldown: 4
  - id: speed_rush
    description: "Crystal Karen surges on her own product — gains 2 extra AP next round."
    effect_type: self_ap_gain
    ap_gain: 2
    cooldown: 4
    max_uses: 2
loot:
  currency:
    min: 200
    max: 500
  items:
    - item: speed_rig
      chance: 0.75
      min_qty: 1
      max_qty: 1
    - item: paranoia_grenade
      chance: 0.50
      min_qty: 1
      max_qty: 2
allowed_rooms:
  - crystal_karens_lab
```

---

## Task 7: NPC YAML — Spiral King (Wook Boss)

**Files:**
- Create: `content/npcs/spiral_king.yaml`

**REQ coverage:** REQ-OCF-22, REQ-OCF-25, REQ-OCF-44

- [ ] **Step 7.1: Create `content/npcs/spiral_king.yaml`**

```yaml
id: spiral_king
name: "The Spiral King"
description: An ancient wook elder draped in a robe of living vines and owl feathers, eyes like twin galaxies.
level: 12
max_hp: 150
ac: 16
awareness: 14
faction_id: wooks
attack_verb: "gestures with cosmic authority at"
ai_domain: ganger_combat
respawn_delay: "72h"
tier: boss
abilities:
  brutality: 14
  quickness: 12
  grit: 18
  reasoning: 18
  savvy: 16
  flair: 20
weapon:
  - id: gnarled_staff
    weight: 1
armor: []
boss_abilities:
  - id: eternal_groove
    description: "The Spiral King warps time — all players lose 1 AP next round from psychedelic time distortion."
    effect_type: aoe_ap_drain
    ap_loss: 1
    target: room_players
    cooldown: 3
  - id: spore_cloud
    description: "The Spiral King exhales a spore cloud — AoE wook_spore dose to all in room."
    effect_type: aoe_substance
    substance_id: wook_spore
    target: room_players
    cooldown: 4
  - id: spiral_vision
    description: "The Spiral King opens a spiral vision — single target Reasoning-DC-15 save or stunned for 1 round."
    effect_type: single_condition
    condition_id: stunned
    duration_rounds: 1
    save:
      attribute: reasoning
      dc: 15
    target: single_player
    cooldown: 2
loot:
  currency:
    min: 200
    max: 500
  items:
    - item: vine_robe
      chance: 0.60
      min_qty: 1
      max_qty: 1
    - item: wook_spore_vial
      chance: 0.75
      min_qty: 1
      max_qty: 3
allowed_rooms:
  - the_spiral_kings_grove
```

---

## Task 8: HTN AI Domain Files

**Files:**
- Create: `content/ai/juggalo_prophet_combat.yaml`
- Create: `content/ai/tweaker_paranoid_combat.yaml`

**REQ coverage:** REQ-OCF-16, REQ-OCF-19

- [ ] **Step 8.1: Create `content/ai/juggalo_prophet_combat.yaml`**

```yaml
id: juggalo_prophet_combat
operators:
  - name: proclaim_scripture
    action: say
    preconditions:
      - key: player_entered_room
        value: true
    strings:
      - "The Carnival is not a place! It's a state of mind!"
      - "Thy wicked shall receive no love! Whoop whoop!"
      - "The Dark Carnival comes for all — even you, Normie!"
      - "Juggalos ain't a gang — we a family! And you ain't in it!"
      - "Ashes, ashes, everybody falls but us!"
    cooldown: 60s
  - name: attack_enemy
    action: attack
    preconditions:
      - key: in_combat
        value: true
```

- [ ] **Step 8.2: Create `content/ai/tweaker_paranoid_combat.yaml`**

```yaml
id: tweaker_paranoid_combat
operators:
  - name: sound_alarm
    action: call_for_help
    preconditions:
      - key: player_entered_room
        value: true
    cooldown: 0s
  - name: attack_enemy
    action: attack
    preconditions:
      - key: in_combat
        value: true
```

---

## Task 9: Zone YAML — oregon_country_fair

**Files:**
- Create: `content/zones/oregon_country_fair.yaml`

**REQ coverage:** REQ-OCF-1 through REQ-OCF-13, REQ-OCF-23 through REQ-OCF-26, REQ-OCF-40 through REQ-OCF-44

- [ ] **Step 9.1: Create `content/zones/oregon_country_fair.yaml`**

```yaml
zone:
  id: oregon_country_fair
  name: "Oregon Country Fair"
  description: >
    The Veneta fairgrounds, 13 miles west of Eugene on Hwy 126 — a forested
    river-adjacent festival site turned three-way warzone. Wooks, Juggalos,
    and Tweakers each control a pocket of the fairgrounds and fight
    continuously for dominance.
  danger_level: dangerous
  faction_id: ""
  world_x: -2
  world_y: -3
  start_room: ocf_main_gate

  rooms:

  # ── Neutral / Contested ───────────────────────────────────────────────────

  - id: ocf_main_gate
    title: "OCF Main Gate"
    description: >
      The old fairground entrance arch, now a checkpoint contested by all
      three factions. Faction banners have been torn down and replaced
      with each other's multiple times. Nothing useful remains nailed up.
    danger_level: dangerous
    map_x: 0
    map_y: 0
    exits:
      - direction: east
        target: ocf_main_stage_ruins
      - direction: south
        target: ocf_market_row
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: tweaker
        count: 1
        respawn_after: 5m

  - id: ocf_main_stage_ruins
    title: "Main Stage Ruins"
    description: >
      The destroyed main performance stage, its skeletal rigging still arching
      overhead. Open combat ground — everyone sprints for cover behind speaker
      stacks and collapsed scaffolding.
    danger_level: dangerous
    map_x: 2
    map_y: 0
    exits:
      - direction: west
        target: ocf_main_gate
      - direction: south
        target: chela_mela_meadow
      - direction: east
        target: the_no_mans_meadow
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: tweaker
        count: 1
        respawn_after: 5m

  - id: chela_mela_meadow
    title: "Chela Mela Meadow"
    description: >
      The large open meadow at the center of the fairgrounds. Crossfire from
      all three factions criss-crosses the clearing. Stay low.
    danger_level: dangerous
    map_x: 2
    map_y: 2
    exits:
      - direction: north
        target: ocf_main_stage_ruins
      - direction: west
        target: ocf_craft_row
      - direction: south
        target: forest_junction_north
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: wook
        count: 1
        respawn_after: 5m
      - template: tweaker
        count: 1
        respawn_after: 5m

  - id: ocf_craft_row
    title: "Craft Row"
    description: >
      Former craft booths, scavenged down to bare posts and torn canvas.
      Factions use the stalls as sniper nests.
    danger_level: dangerous
    map_x: 0
    map_y: 2
    exits:
      - direction: east
        target: chela_mela_meadow
      - direction: south
        target: ocf_market_row
    spawns:
      - template: tweaker
        count: 1
        respawn_after: 5m
      - template: juggalo
        count: 1
        respawn_after: 5m

  - id: ocf_market_row
    title: "Market Row"
    description: >
      Former food vendor row, the only place in the fairgrounds with an
      uneasy truce. The chip doc operates out of a rusted food cart.
    danger_level: sketchy
    map_x: 0
    map_y: 4
    exits:
      - direction: north
        target: ocf_main_gate
      - direction: east
        target: ocf_craft_row
      - direction: south
        target: the_long_tom_crossing
    spawns:
      - template: chip_doc
        count: 1
        respawn_after: 0s

  - id: the_long_tom_crossing
    title: "Long Tom Crossing"
    description: >
      Bridge over the Long Tom River, a contested chokepoint. Factions fight
      for control of the crossing — whoever holds it cuts off reinforcements.
    danger_level: dangerous
    map_x: 0
    map_y: 6
    exits:
      - direction: north
        target: ocf_market_row
      - direction: east
        target: the_mud_pit
      - direction: south
        target: forest_junction_south
      - direction: west
        target: wook_river_camp
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: wook
        count: 1
        respawn_after: 5m

  - id: the_mud_pit
    title: "The Mud Pit"
    description: >
      A former art installation that flooded and never drained. Now a
      battlefield with thigh-deep mud that slows everyone equally.
    danger_level: dangerous
    map_x: 2
    map_y: 6
    exits:
      - direction: west
        target: the_long_tom_crossing
      - direction: north
        target: forest_junction_north
    spawns:
      - template: tweaker
        count: 1
        respawn_after: 5m
      - template: juggalo
        count: 1
        respawn_after: 5m

  - id: forest_junction_north
    title: "Forest Junction North"
    description: >
      A wooded path splitting northward into Juggalo territory and westward
      deeper into the contested zone. The canopy swallows the sound of battle.
    danger_level: dangerous
    map_x: 2
    map_y: 4
    exits:
      - direction: north
        target: chela_mela_meadow
      - direction: south
        target: the_mud_pit
      - direction: west
        target: the_no_mans_meadow
      - direction: east
        target: the_big_top
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: wook
        count: 1
        respawn_after: 5m

  - id: forest_junction_south
    title: "Forest Junction South"
    description: >
      A wooded path splitting into Tweaker territory and toward Wook territory.
      Tweaker trip-wires have been reported along the eastern edge.
    danger_level: dangerous
    map_x: 0
    map_y: 8
    exits:
      - direction: north
        target: the_long_tom_crossing
      - direction: east
        target: the_trailer_cluster
      - direction: south
        target: the_no_mans_meadow
    spawns:
      - template: tweaker_paranoid
        count: 1
        respawn_after: 5m
      - template: juggalo
        count: 1
        respawn_after: 5m

  - id: the_no_mans_meadow
    title: "No Man's Meadow"
    description: >
      The open field where all three territories converge. All-out war erupts
      here without warning. Do not linger.
    danger_level: all_out_war
    map_x: 2
    map_y: 8
    exits:
      - direction: north
        target: forest_junction_north
      - direction: west
        target: forest_junction_south
      - direction: east
        target: ocf_main_stage_ruins
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: tweaker
        count: 1
        respawn_after: 5m
      - template: wook
        count: 1
        respawn_after: 5m

  # ── Juggalo Territory ─────────────────────────────────────────────────────

  - id: the_big_top
    title: "The Big Top"
    description: >
      A massive circus tent, somehow still standing, painted in Juggalo black
      and white. This is the anchor of Juggalo safe space at the OCF.
    danger_level: safe
    map_x: 4
    map_y: 4
    exits:
      - direction: west
        target: forest_junction_north
      - direction: north
        target: the_faygo_fountain
      - direction: south
        target: the_gathering_ground
    spawns:
      - template: juggalo_merchant
        count: 1
        respawn_after: 0s
      - template: juggalo_banker
        count: 1
        respawn_after: 0s

  - id: the_faygo_fountain
    title: "The Faygo Fountain"
    description: >
      A sacred Faygo dispenser, constantly flowing in an improvised aluminum
      shrine. The Juggalo healer tends to the wounded here.
    danger_level: safe
    map_x: 4
    map_y: 2
    exits:
      - direction: south
        target: the_big_top
    spawns:
      - template: juggalo_healer
        count: 1
        respawn_after: 0s

  - id: the_gathering_ground
    title: "The Gathering Ground"
    description: >
      An ICP shrine and gathering space, ringed with spray-painted faces of
      Dark Carnival figures. Quest givers and fixers operate here.
    danger_level: safe
    map_x: 4
    map_y: 6
    exits:
      - direction: north
        target: the_big_top
      - direction: east
        target: icp_shrine
    spawns:
      - template: juggalo_quest_giver
        count: 1
        respawn_after: 0s
      - template: juggalo_fixer
        count: 1
        respawn_after: 0s

  - id: icp_shrine
    title: "ICP Shrine"
    description: >
      An altar of Dark Carnival scripture, candles, and Faygo empties.
      Juggalo prophets recite scripture here between battles.
    danger_level: sketchy
    map_x: 6
    map_y: 6
    exits:
      - direction: west
        target: the_gathering_ground
      - direction: east
        target: the_carnival_arcade
    spawns:
      - template: juggalo
        count: 1
        respawn_after: 5m
      - template: juggalo_prophet
        count: 1
        respawn_after: 10m

  - id: the_carnival_arcade
    title: "Carnival Arcade"
    description: >
      Former midway games, now trials of Juggalo worthiness. Win the ring
      toss or take a hatchet to the throat — those are the options.
    danger_level: sketchy
    map_x: 8
    map_y: 6
    exits:
      - direction: west
        target: icp_shrine
      - direction: north
        target: the_hatchet_barracks
    spawns:
      - template: juggalo
        count: 2
        respawn_after: 5m

  - id: the_hatchet_barracks
    title: "Hatchet Barracks"
    description: >
      Juggalo fighter quarters: rows of cots, hatchet racks, and Faygo
      cases stacked to the ceiling. Always occupied, always armed.
    danger_level: dangerous
    map_x: 8
    map_y: 4
    exits:
      - direction: south
        target: the_carnival_arcade
      - direction: west
        target: the_dark_carnival_stage
    spawns:
      - template: juggalo
        count: 2
        respawn_after: 5m
      - template: juggalo_prophet
        count: 1
        respawn_after: 10m

  - id: the_dark_carnival_stage
    title: "Dark Carnival Stage"
    description: >
      A ritual performance space draped in black velvet and strobing with
      colored lights. Juggalo prophets perform here between raids.
    danger_level: dangerous
    map_x: 6
    map_y: 4
    exits:
      - direction: east
        target: the_hatchet_barracks
      - direction: south
        target: violent_jimmys_tent
    spawns:
      - template: juggalo
        count: 2
        respawn_after: 5m
      - template: juggalo_prophet
        count: 1
        respawn_after: 10m

  - id: violent_jimmys_tent
    title: "Violent Jimmy's Tent"
    description: >
      The boss tent, draped in velvet, lit with black lights and strobe.
      Violent Jimmy holds court here, surrounded by his most devoted prophets.
    danger_level: all_out_war
    boss_room: true
    map_x: 6
    map_y: 2
    exits:
      - direction: north
        target: the_dark_carnival_stage
    hazards:
      - id: faygo_rain
        description: "Faygo rains from the ceiling — 1d4 acid damage to all players at round start."
        effect_type: aoe_damage
        damage: "1d4"
        damage_type: acid
        trigger: round_start
        target: room_players
    spawns:
      - template: violent_jimmy
        count: 1
        respawn_after: 72h
      - template: juggalo_prophet
        count: 2
        respawn_after: 10m

  # ── Tweaker Territory ─────────────────────────────────────────────────────

  - id: the_trailer_cluster
    title: "Trailer Cluster"
    description: >
      A ring of converted trailers, welded together into a fortress. The
      anchor of Tweaker safe space. The air smells of industrial chemicals.
    danger_level: safe
    ambient_substance: tweaker_crystal
    map_x: -2
    map_y: 8
    exits:
      - direction: west
        target: forest_junction_south
      - direction: north
        target: the_cook_shed_anteroom
      - direction: south
        target: tweaker_command_post
    spawns:
      - template: tweaker_merchant
        count: 1
        respawn_after: 0s
      - template: tweaker_banker
        count: 1
        respawn_after: 0s

  - id: the_cook_shed_anteroom
    title: "Cook Shed Anteroom"
    description: >
      Supply intake for the meth operation. Shelves of precursor chemicals
      and coded inventory lists line the walls.
    danger_level: safe
    ambient_substance: tweaker_crystal
    map_x: -2
    map_y: 6
    exits:
      - direction: south
        target: the_trailer_cluster
    spawns:
      - template: tweaker_healer
        count: 1
        respawn_after: 0s

  - id: tweaker_command_post
    title: "Tweaker Command Post"
    description: >
      Central nerve center: one wall covered entirely in string, photos,
      and incomprehensible notes. Crystal Karen's lieutenants plan here.
    danger_level: safe
    ambient_substance: tweaker_crystal
    map_x: -2
    map_y: 10
    exits:
      - direction: north
        target: the_trailer_cluster
      - direction: east
        target: tweaker_watchtower
    spawns:
      - template: tweaker_quest_giver
        count: 1
        respawn_after: 0s
      - template: tweaker_fixer
        count: 1
        respawn_after: 0s

  - id: tweaker_watchtower
    title: "Tweaker Watchtower"
    description: >
      An elevated lookout built from scaffolding and corrugated metal.
      Paranoid Tweakers scan the perimeter constantly.
    danger_level: sketchy
    ambient_substance: tweaker_crystal
    map_x: 0
    map_y: 10
    exits:
      - direction: west
        target: tweaker_command_post
      - direction: east
        target: the_junk_maze
    spawns:
      - template: tweaker_paranoid
        count: 2
        respawn_after: 5m

  - id: the_junk_maze
    title: "Junk Maze"
    description: >
      A labyrinth of scavenged materials — chain-link, rebar, car doors —
      assembled into a disorienting maze of corridors and dead ends.
    danger_level: sketchy
    ambient_substance: tweaker_crystal
    map_x: 2
    map_y: 10
    exits:
      - direction: west
        target: tweaker_watchtower
      - direction: south
        target: the_paranoia_corridor
    spawns:
      - template: tweaker
        count: 2
        respawn_after: 5m

  - id: the_paranoia_corridor
    title: "Paranoia Corridor"
    description: >
      A dark passage strung with trip-wire alarms. Every shadow is a threat.
      Every sound is a signal. The Tweakers here are on a hair trigger.
    danger_level: dangerous
    ambient_substance: tweaker_crystal
    map_x: 2
    map_y: 12
    exits:
      - direction: north
        target: the_junk_maze
      - direction: south
        target: the_hunting_grounds
    spawns:
      - template: tweaker_paranoid
        count: 1
        respawn_after: 5m
      - template: tweaker_cook
        count: 1
        respawn_after: 10m
      - template: tweaker
        count: 1
        respawn_after: 5m

  - id: the_hunting_grounds
    title: "Hunting Grounds"
    description: >
      Tweaker patrol sweep zone — a killing floor of open ground between
      the corridor and the lab. Anyone here is prey.
    danger_level: dangerous
    ambient_substance: tweaker_crystal
    map_x: 2
    map_y: 14
    exits:
      - direction: north
        target: the_paranoia_corridor
      - direction: west
        target: crystal_karens_lab
    spawns:
      - template: tweaker
        count: 2
        respawn_after: 5m
      - template: tweaker_cook
        count: 1
        respawn_after: 10m

  - id: crystal_karens_lab
    title: "Crystal Karen's Lab"
    description: >
      The production heart of the Tweaker operation. Stainless steel tables,
      boiling flasks, and Crystal Karen holding court in her chemical-scorched
      lab coat.
    danger_level: all_out_war
    boss_room: true
    ambient_substance: tweaker_crystal
    map_x: 0
    map_y: 14
    exits:
      - direction: east
        target: the_hunting_grounds
    hazards:
      - id: toxic_fumes
        description: "Toxic fumes pervade the lab — applies tweaker_crystal micro-dose to all players at round start."
        effect_type: aoe_substance
        substance_id: tweaker_crystal
        trigger: round_start
        target: room_players
      - id: trip_wire_grid
        description: "Trip wires criss-cross the floor — Quickness-DC-12 save on room entry or take 1d6 damage."
        effect_type: entry_save
        damage: "1d6"
        save:
          attribute: quickness
          dc: 12
        trigger: room_entry
        target: entering_player
    spawns:
      - template: crystal_karen
        count: 1
        respawn_after: 72h
      - template: tweaker_cook
        count: 2
        respawn_after: 10m

  # ── Wook Territory ────────────────────────────────────────────────────────

  - id: wook_river_camp
    title: "Wook River Camp"
    description: >
      A riverside drum circle camp, the Wook anchor at the OCF. The Long Tom
      River murmurs past. Drums sound constantly.
    danger_level: safe
    ambient_substance: wook_spore
    map_x: -2
    map_y: 6
    exits:
      - direction: east
        target: the_long_tom_crossing
      - direction: north
        target: the_healing_waters_ocf
      - direction: south
        target: the_wook_council_fire
    spawns:
      - template: wook_merchant
        count: 1
        respawn_after: 0s
      - template: wook_banker
        count: 1
        respawn_after: 0s

  - id: the_healing_waters_ocf
    title: "Healing Waters (OCF)"
    description: >
      A sacred access point to the Long Tom River. Wooks believe the water
      here carries special properties. The healer tends to wounded here.
    danger_level: safe
    ambient_substance: wook_spore
    map_x: -2
    map_y: 4
    exits:
      - direction: south
        target: wook_river_camp
    spawns:
      - template: wook_healer
        count: 1
        respawn_after: 0s

  - id: the_wook_council_fire
    title: "Wook Council Fire"
    description: >
      An evening council fire circle, ringed with driftwood seats. The council
      never stops burning. Quest givers deliberate here.
    danger_level: safe
    ambient_substance: wook_spore
    map_x: -2
    map_y: 8
    exits:
      - direction: north
        target: wook_river_camp
      - direction: west
        target: mushroom_grove_ocf
    spawns:
      - template: wook_quest_giver
        count: 1
        respawn_after: 0s
      - template: wook_fixer
        count: 1
        respawn_after: 0s

  - id: mushroom_grove_ocf
    title: "Mushroom Grove (OCF)"
    description: >
      A dense cluster of enormous mushrooms, sacred to the Wooks. The caps
      glow faintly in the dark. Entering feels like stepping into another world.
    danger_level: sketchy
    ambient_substance: wook_spore
    map_x: -4
    map_y: 8
    exits:
      - direction: east
        target: the_wook_council_fire
      - direction: south
        target: the_wook_meditation_field
    spawns:
      - template: wook
        count: 2
        respawn_after: 5m

  - id: the_wook_meditation_field
    title: "Wook Meditation Field"
    description: >
      A silent meditation area, sacred to the Wooks. Combat shatters the
      silence — and the Wooks' patience.
    danger_level: sketchy
    ambient_substance: wook_spore
    map_x: -4
    map_y: 10
    exits:
      - direction: north
        target: mushroom_grove_ocf
      - direction: south
        target: the_wook_deep_forest_ocf
    spawns:
      - template: wook
        count: 2
        respawn_after: 5m

  - id: the_wook_deep_forest_ocf
    title: "Wook Deep Forest (OCF)"
    description: >
      Old-growth forest along the river. Massive firs blot out the sky.
      Wook enforcers patrol these shadows.
    danger_level: dangerous
    ambient_substance: wook_spore
    map_x: -4
    map_y: 12
    exits:
      - direction: north
        target: the_wook_meditation_field
      - direction: south
        target: the_wook_outpost_wall
    spawns:
      - template: wook
        count: 2
        respawn_after: 5m
      - template: wook_shaman
        count: 1
        respawn_after: 10m

  - id: the_wook_outpost_wall
    title: "Wook Outpost Wall"
    description: >
      Perimeter defense: woven logs and rope, decorated with shells and feathers.
      Wook guards stand watch day and night.
    danger_level: dangerous
    ambient_substance: wook_spore
    map_x: -4
    map_y: 14
    exits:
      - direction: north
        target: the_wook_deep_forest_ocf
      - direction: west
        target: the_spiral_kings_grove
    spawns:
      - template: wook_enforcer
        count: 2
        respawn_after: 5m
      - template: wook_shaman
        count: 1
        respawn_after: 10m

  - id: the_spiral_kings_grove
    title: "The Spiral King's Grove"
    description: >
      An ancient oak clearing, the oldest living space at the OCF. The Spiral
      King's presence warps the air — colors bleed, time pools.
    danger_level: all_out_war
    boss_room: true
    ambient_substance: wook_spore
    map_x: -6
    map_y: 14
    exits:
      - direction: east
        target: the_wook_outpost_wall
    hazards:
      - id: psychedelic_fog
        description: "Psychedelic fog pervades the grove — applies wook_spore micro-dose to all players at round start."
        effect_type: aoe_substance
        substance_id: wook_spore
        trigger: round_start
        target: room_players
    spawns:
      - template: spiral_king
        count: 1
        respawn_after: 72h
      - template: wook_shaman
        count: 2
        respawn_after: 10m
```

- [ ] **Step 9.2: Verify zone loads at startup**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i oregon
```

Expected: no errors. Zone `oregon_country_fair` loads with 34 rooms.

- [ ] **Step 9.3: Verify ambient_substance validation**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1 | grep -i ambient
```

Expected: no errors. All `ambient_substance` IDs (`tweaker_crystal`, `wook_spore`) resolve in SubstanceRegistry.

---

## Task 10: Quest Giver Stub Handler

**Files:**
- Create: `internal/gameserver/grpc_service_quest_giver.go`
- Create: `internal/gameserver/grpc_service_quest_giver_test.go`

**REQ coverage:** REQ-OCF-37, REQ-OCF-38, REQ-OCF-39

- [ ] **Step 10.1: Write failing tests**

```go
// internal/gameserver/grpc_service_quest_giver_test.go
package gameserver_test

import (
	"context"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestHandleQuestGiver_ReturnsStubMessage(t *testing.T) {
	msg, err := gameserver.HandleQuestGiverInteract(context.Background(), "juggalo_quest_giver", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantSubstr = "time isn't right yet"
	if !strings.Contains(msg, wantSubstr) {
		t.Fatalf("expected message containing %q, got %q", wantSubstr, msg)
	}
}

func TestHandleQuestGiver_AllFactionQuestGivers_ReturnStub(t *testing.T) {
	givers := []string{
		"juggalo_quest_giver",
		"tweaker_quest_giver",
		"wook_quest_giver",
	}
	for _, g := range givers {
		msg, err := gameserver.HandleQuestGiverInteract(context.Background(), g, "")
		if err != nil {
			t.Fatalf("giver %q: unexpected error: %v", g, err)
		}
		if msg == "" {
			t.Fatalf("giver %q: expected non-empty message", g)
		}
	}
}

func TestProperty_HandleQuestGiver_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		giverID := rapid.String().Draw(rt, "giver_id")
		playerID := rapid.String().Draw(rt, "player_id")
		_, _ = gameserver.HandleQuestGiverInteract(context.Background(), giverID, playerID)
	})
}

```

Run:

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleQuestGiver -v 2>&1 | head -20
```

Expected: FAIL — function does not exist yet.

- [ ] **Step 10.2: Implement `internal/gameserver/grpc_service_quest_giver.go`**

```go
// Package gameserver implements the gRPC game service.
package gameserver

import (
	"context"
	"fmt"
)

// stubQuestGiverMessage is the message displayed by quest giver NPCs before
// the quests feature is implemented.
const stubQuestGiverMessage = "I've got work for you, but the time isn't right yet."

// HandleQuestGiverInteract is a no-op handler for quest giver NPC interactions.
// It returns the stub message for all quest giver types until the quests
// feature (quests plan) is fully implemented and wired in.
//
// Precondition: giverID is a valid quest giver NPC template ID.
// Postcondition: Returns (stubQuestGiverMessage, nil) for any giverID.
func HandleQuestGiverInteract(_ context.Context, giverID string, playerID string) (string, error) {
	if giverID == "" {
		return "", fmt.Errorf("quest giver NPC ID must not be empty")
	}
	return stubQuestGiverMessage, nil
}
```

- [ ] **Step 10.3: Run tests and verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleQuestGiver -v
```

Expected: all PASS.

---

## Task 11: Integration Tests

**Files:**
- Create: `internal/gameserver/oregon_country_fair_integration_test.go`

**REQ coverage:** REQ-OCF-1–13, REQ-OCF-23–26, REQ-OCF-29–35, REQ-OCF-40–44

- [ ] **Step 11.1: Write integration tests**

```go
// internal/gameserver/oregon_country_fair_integration_test.go
package gameserver_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// TestOCFZone_Loads verifies the zone loads from YAML.
func TestOCFZone_Loads(t *testing.T) {
	zones, err := world.LoadZoneDirectory("../../content/zones")
	if err != nil {
		t.Fatalf("LoadZoneDirectory: %v", err)
	}
	var ocf *world.Zone
	for i := range zones {
		if zones[i].ID == "oregon_country_fair" {
			ocf = &zones[i]
			break
		}
	}
	if ocf == nil {
		t.Fatal("zone oregon_country_fair not found")
	}
	if len(ocf.Rooms) < 30 || len(ocf.Rooms) > 45 {
		t.Errorf("expected 30–45 rooms, got %d", len(ocf.Rooms))
	}
	if ocf.FactionID != "" {
		t.Errorf("expected empty faction_id, got %q", ocf.FactionID)
	}
}

// TestOCFZone_ThreeBossRooms verifies exactly 3 boss rooms exist.
func TestOCFZone_ThreeBossRooms(t *testing.T) {
	zones, err := world.LoadZoneDirectory("../../content/zones")
	if err != nil {
		t.Fatalf("LoadZoneDirectory: %v", err)
	}
	var ocf *world.Zone
	for i := range zones {
		if zones[i].ID == "oregon_country_fair" {
			ocf = &zones[i]
			break
		}
	}
	if ocf == nil {
		t.Fatal("zone oregon_country_fair not found")
	}
	bossRooms := []string{
		"violent_jimmys_tent",
		"crystal_karens_lab",
		"the_spiral_kings_grove",
	}
	roomByID := make(map[string]*world.Room, len(ocf.Rooms))
	for i := range ocf.Rooms {
		roomByID[ocf.Rooms[i].ID] = &ocf.Rooms[i]
	}
	for _, id := range bossRooms {
		r, ok := roomByID[id]
		if !ok {
			t.Errorf("boss room %q not found", id)
			continue
		}
		if !r.BossRoom {
			t.Errorf("room %q: expected boss_room: true", id)
		}
		if len(r.Hazards) == 0 {
			t.Errorf("boss room %q: expected at least one hazard", id)
		}
	}
}

// TestOCFZone_AmbientSubstances verifies ambient_substance values reference valid substances.
func TestOCFZone_AmbientSubstances(t *testing.T) {
	reg, err := substance.LoadDirectory("../../content/substances")
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	zones, err := world.LoadZoneDirectory("../../content/zones")
	if err != nil {
		t.Fatalf("LoadZoneDirectory: %v", err)
	}
	var ocf *world.Zone
	for i := range zones {
		if zones[i].ID == "oregon_country_fair" {
			ocf = &zones[i]
			break
		}
	}
	if ocf == nil {
		t.Fatal("zone oregon_country_fair not found")
	}
	for _, r := range ocf.Rooms {
		if r.AmbientSubstance == "" {
			continue
		}
		if _, ok := reg.Get(r.AmbientSubstance); !ok {
			t.Errorf("room %q: ambient_substance %q not found in registry", r.ID, r.AmbientSubstance)
		}
	}
}

// TestOCFFactions_Load verifies all three factions load.
func TestOCFFactions_Load(t *testing.T) {
	reg, err := faction.LoadDirectory("../../content/factions")
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	for _, id := range []string{"juggalos", "tweakers", "wooks"} {
		f, ok := reg.Get(id)
		if !ok {
			t.Errorf("faction %q not found", id)
			continue
		}
		if err := f.Validate(); err != nil {
			t.Errorf("faction %q invalid: %v", id, err)
		}
	}
}

// TestOCFFactions_WooksZoneID verifies wooks faction has empty zone_id.
func TestOCFFactions_WooksZoneID(t *testing.T) {
	reg, err := faction.LoadDirectory("../../content/factions")
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	wooks, ok := reg.Get("wooks")
	if !ok {
		t.Fatal("wooks faction not found")
	}
	if wooks.ZoneID != "" {
		t.Errorf("expected wooks zone_id empty, got %q", wooks.ZoneID)
	}
}

// TestOCFSubstance_TweakerCrystal verifies tweaker_crystal substance loads correctly.
func TestOCFSubstance_TweakerCrystal(t *testing.T) {
	reg, err := substance.LoadDirectory("../../content/substances")
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}
	sub, ok := reg.Get("tweaker_crystal")
	if !ok {
		t.Fatal("tweaker_crystal substance not found")
	}
	if sub.Category != "stimulant" {
		t.Errorf("expected category stimulant, got %q", sub.Category)
	}
}

// TestOCFZone_SafeClusterRoomsHaveZeroSpawns verifies safe rooms in all
// three safe clusters have no combat spawns.
func TestOCFZone_SafeClusterRoomsHaveZeroSpawns(t *testing.T) {
	safeClusterIDs := map[string]struct{}{
		"the_big_top":           {},
		"the_faygo_fountain":    {},
		"the_gathering_ground":  {},
		"the_trailer_cluster":   {},
		"the_cook_shed_anteroom": {},
		"tweaker_command_post":  {},
		"wook_river_camp":       {},
		"the_healing_waters_ocf": {},
		"the_wook_council_fire": {},
	}
	zones, err := world.LoadZoneDirectory("../../content/zones")
	if err != nil {
		t.Fatalf("LoadZoneDirectory: %v", err)
	}
	var ocf *world.Zone
	for i := range zones {
		if zones[i].ID == "oregon_country_fair" {
			ocf = &zones[i]
			break
		}
	}
	if ocf == nil {
		t.Fatal("zone oregon_country_fair not found")
	}
	combatTemplates := map[string]struct{}{
		"juggalo": {}, "juggalette": {}, "juggalo_prophet": {},
		"tweaker": {}, "tweaker_paranoid": {}, "tweaker_cook": {},
		"wook": {}, "wook_shaman": {}, "wook_enforcer": {},
	}
	for _, r := range ocf.Rooms {
		if _, isSafe := safeClusterIDs[r.ID]; !isSafe {
			continue
		}
		for _, sp := range r.Spawns {
			if _, isCombat := combatTemplates[sp.Template]; isCombat {
				t.Errorf("safe room %q has combat spawn %q", r.ID, sp.Template)
			}
		}
	}
}

// TestProperty_OCFZone_RoomsNeverPanic verifies zone room iteration never panics.
func TestProperty_OCFZone_RoomsNeverPanic(t *testing.T) {
	zones, err := world.LoadZoneDirectory("../../content/zones")
	if err != nil {
		t.Fatalf("LoadZoneDirectory: %v", err)
	}
	rapid.Check(t, func(rt *rapid.T) {
		for _, z := range zones {
			if z.ID != "oregon_country_fair" {
				continue
			}
			idx := rapid.IntRange(0, len(z.Rooms)-1).Draw(rt, "idx")
			_ = z.Rooms[idx].ID
		}
	})
}
```

- [ ] **Step 11.2: Run integration tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestOCF -v 2>&1
```

Expected: all PASS after all YAML files are in place.

---

## Task 12: Update docs/features/index.yaml

**Files:**
- Modify: `docs/features/index.yaml`

**REQ coverage:** (meta — keeps index in sync)

- [ ] **Step 12.1: Update status field**

In `docs/features/index.yaml`, find the entry for `oregon-country-fair` and change:

```yaml
    status: spec
```

to:

```yaml
    status: planned
```

- [ ] **Step 12.2: Verify index parses correctly**

```bash
cd /home/cjohannsen/src/mud && python3 -c "import yaml; yaml.safe_load(open('docs/features/index.yaml'))" && echo "OK"
```

Expected: `OK`.

---

## Task 13: Full Test Suite Verification

**REQ coverage:** SWENG-6, SWENG-6A

- [ ] **Step 13.1: Run complete test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -30
```

Expected: all PASS. No regressions.

- [ ] **Step 13.2: Verify startup with all content loaded**

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/gameserver/... --validate-only 2>&1
```

Expected: clean exit — zone loads, all factions load, all substances load, ambient_substance IDs validate.

---

## Task 14: Non-Combat NPC Content YAML

**Files:**
- Create: `content/npcs/juggalo_merchant.yaml`
- Create: `content/npcs/juggalo_healer.yaml`
- Create: `content/npcs/juggalo_fixer.yaml`
- Create: `content/npcs/juggalo_banker.yaml`
- Create: `content/npcs/juggalo_quest_giver.yaml`
- Create: `content/npcs/tweaker_merchant.yaml`
- Create: `content/npcs/tweaker_healer.yaml`
- Create: `content/npcs/tweaker_fixer.yaml`
- Create: `content/npcs/tweaker_banker.yaml`
- Create: `content/npcs/tweaker_quest_giver.yaml`
- Create: `content/npcs/wook_quest_giver.yaml`
- Create: `content/npcs/chip_doc_ocf.yaml`
- Modify: `content/zones/oregon_country_fair.yaml` — update chip_doc spawn template from `chip_doc` to `chip_doc_ocf`

**REQ coverage:** REQ-OCF-45 through REQ-OCF-55, REQ-OCF-37, REQ-OCF-39

All NPC YAML files follow the schema from `content/npcs/wook_merchant.yaml`, `wook_healer.yaml`, `wook_fixer.yaml`, `wook_banker.yaml`, and `chip_doc_wooklyn.yaml`.

### Juggalo Merchant

- [ ] **Step 14.1: Create `content/npcs/juggalo_merchant.yaml`**

```yaml
id: juggalo_merchant
name: "Carnival Vendor"
description: >
  A face-painted Juggalo running a makeshift market stall from a repurposed
  carnival game booth. He sells with unhinged enthusiasm and always has a Faygo
  close at hand.
type: human
npc_type: merchant
npc_role: merchant
level: 3
max_hp: 20
ac: 11
awareness: 6
faction_id: juggalos
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 10
  grit: 10
  quickness: 11
  reasoning: 11
  savvy: 14
  flair: 16
merchant:
  merchant_type: general
  sell_margin: 1.30
  buy_margin: 0.35
  budget: 800
  inventory:
    - item_id: hatchet
      price: 30
      stock: 5
    - item_id: faygo_bottle
      price: 5
      stock: 20
    - item_id: icp_face_paint
      price: 2
      stock: 10
    - item_id: leather_jacket
      price: 80
      stock: 3
  exclusive_items:
    - item_id: hatchet_man_pendant
      price: 120
      stock: 2
      required_tier: family
    - item_id: faygo_grenade
      price: 45
      stock: 5
      required_tier: family
    - item_id: icp_mixtape
      price: 35
      stock: 3
      required_tier: family
  replenish_rate:
    min_hours: 12
    max_hours: 24
    stock_refill: 2
    budget_refill: 200
loot:
  currency:
    min: 10
    max: 40
```

### Juggalo Healer

- [ ] **Step 14.2: Create `content/npcs/juggalo_healer.yaml`**

```yaml
id: juggalo_healer
name: "Faygo Sister"
description: >
  A Juggalette in ceremonial paint who has appointed herself the faction medic.
  She patches wounds with Faygo-soaked rags and genuine enthusiasm. It works
  more often than you'd expect.
type: human
npc_type: healer
npc_role: healer
level: 3
max_hp: 22
ac: 10
awareness: 7
faction_id: juggalos
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 8
  grit: 12
  quickness: 11
  reasoning: 12
  savvy: 12
  flair: 15
healer:
  price_per_hp: 3
  daily_capacity: 50
  services:
    - service: full_hp
      price: 15
      description: "Faygo washes all wounds clean, homie."
    - service: cure_condition
      condition_id: poison
      price: 20
      description: "Ain't no poison that Faygo can't flush."
    - service: cure_condition
      condition_id: stunned
      price: 10
      description: "Walk it off, Family."
  required_tier: down
loot:
  currency:
    min: 5
    max: 20
```

### Juggalo Fixer

- [ ] **Step 14.3: Create `content/npcs/juggalo_fixer.yaml`**

```yaml
id: juggalo_fixer
name: "The Broker"
description: >
  A quiet Juggalo in a clean tracksuit who handles reputation arrangements.
  "You wanna be Family, you gotta pay the Gathering toll." He doesn't explain
  further. He doesn't need to.
type: human
npc_type: fixer
npc_role: fixer
level: 3
max_hp: 22
ac: 11
awareness: 8
faction_id: juggalos
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 9
  grit: 10
  quickness: 11
  reasoning: 12
  savvy: 16
  flair: 14
fixer:
  rep_purchases:
    - cost: 10
      rep_gain: 1
      faction_id: juggalos
      cooldown: daily
    - cost: 50
      rep_gain: 5
      faction_id: juggalos
      cooldown: weekly
  required_tier: down
loot:
  currency:
    min: 10
    max: 40
```

### Juggalo Banker

- [ ] **Step 14.4: Create `content/npcs/juggalo_banker.yaml`**

```yaml
id: juggalo_banker
name: "Hatchet Bank"
description: >
  A heavyset Juggalo with an abacus and a lockbox, sitting in a black velvet
  booth. He runs the Dark Carnival Credit Union with surprising competence.
type: human
npc_type: banker
npc_role: banker
level: 3
max_hp: 22
ac: 11
awareness: 7
faction_id: juggalos
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 11
  grit: 12
  quickness: 9
  reasoning: 13
  savvy: 13
  flair: 12
banker:
  zone_id: oregon_country_fair
  base_rate: 0.98
  rate_variance: 0.02
  required_tier: down
loot:
  currency:
    min: 20
    max: 80
```

### Juggalo Quest Giver

- [ ] **Step 14.5: Create `content/npcs/juggalo_quest_giver.yaml`**

```yaml
id: juggalo_quest_giver
name: "Prophet Scratch"
description: >
  An elder Juggalo in worn velvet, holding a crumpled notebook of field
  intelligence. He tracks Tweaker and Wook movements obsessively and has
  work for anyone willing to bleed for the Family.
type: human
npc_type: quest_giver
npc_role: quest_giver
level: 4
max_hp: 25
ac: 11
awareness: 9
faction_id: juggalos
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 10
  grit: 12
  quickness: 10
  reasoning: 13
  savvy: 14
  flair: 16
loot:
  currency:
    min: 0
    max: 0
```

### Tweaker Merchant

- [ ] **Step 14.6: Create `content/npcs/tweaker_merchant.yaml`**

```yaml
id: tweaker_merchant
name: "Supply Runner"
description: >
  A twitchy Tweaker who handles procurement with manic efficiency. He talks
  fast, stocks fast, and prices fast. Don't make eye contact too long.
type: human
npc_type: merchant
npc_role: merchant
level: 3
max_hp: 20
ac: 11
awareness: 8
faction_id: tweakers
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 9
  grit: 10
  quickness: 17
  reasoning: 12
  savvy: 13
  flair: 9
merchant:
  merchant_type: general
  sell_margin: 1.25
  buy_margin: 0.40
  budget: 750
  inventory:
    - item_id: shiv
      price: 20
      stock: 8
    - item_id: scrap_armor
      price: 60
      stock: 3
    - item_id: energy_drink
      price: 8
      stock: 15
    - item_id: duct_tape
      price: 3
      stock: 20
  exclusive_items:
    - item_id: crystal_shard_pipe
      price: 80
      stock: 3
      required_tier: inner_circle
    - item_id: speed_rig
      price: 200
      stock: 1
      required_tier: inner_circle
    - item_id: paranoia_grenade
      price: 60
      stock: 4
      required_tier: inner_circle
  replenish_rate:
    min_hours: 10
    max_hours: 20
    stock_refill: 3
    budget_refill: 200
loot:
  currency:
    min: 8
    max: 35
```

### Tweaker Healer

- [ ] **Step 14.7: Create `content/npcs/tweaker_healer.yaml`**

```yaml
id: tweaker_healer
name: "Doc Meth"
description: >
  A Tweaker cook who moonlights as a field medic. His methods are unorthodox
  and his bedside manner is nonexistent, but he gets the job done. "You look
  rough. I can fix that."
type: human
npc_type: healer
npc_role: healer
level: 3
max_hp: 22
ac: 10
awareness: 9
faction_id: tweakers
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 9
  grit: 11
  quickness: 15
  reasoning: 14
  savvy: 11
  flair: 8
healer:
  price_per_hp: 2
  daily_capacity: 60
  services:
    - service: full_hp
      price: 12
      description: "You look rough. I can fix that."
    - service: cure_condition
      condition_id: poison
      price: 18
      description: "Wrong kind of chemical. Let's flush it."
    - service: cure_condition
      condition_id: paranoid
      price: 8
      description: "You're not being followed. Probably."
    - service: cure_withdrawal
      substance_id: tweaker_crystal
      price: 25
      description: "Coming down hard? I've got something for that."
  required_tier: known
loot:
  currency:
    min: 5
    max: 20
```

### Tweaker Fixer

- [ ] **Step 14.8: Create `content/npcs/tweaker_fixer.yaml`**

```yaml
id: tweaker_fixer
name: "The Accountant"
description: >
  A methodical Tweaker in clean clothes who keeps immaculate records of who
  owes what to whom. "Trust is earned. Around here, it's also bought." He
  accepts exact payment only.
type: human
npc_type: fixer
npc_role: fixer
level: 3
max_hp: 22
ac: 11
awareness: 9
faction_id: tweakers
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 8
  grit: 10
  quickness: 14
  reasoning: 15
  savvy: 15
  flair: 9
fixer:
  rep_purchases:
    - cost: 8
      rep_gain: 1
      faction_id: tweakers
      cooldown: daily
    - cost: 40
      rep_gain: 5
      faction_id: tweakers
      cooldown: weekly
  required_tier: known
loot:
  currency:
    min: 10
    max: 40
```

### Tweaker Banker

- [ ] **Step 14.9: Create `content/npcs/tweaker_banker.yaml`**

```yaml
id: tweaker_banker
name: "Tweaker Treasury"
description: >
  A paranoid Tweaker with a custom-welded lockbox and a surveillance camera
  pointed at every approach. He doesn't trust anyone, but he'll hold your
  credits — for a fee.
type: human
npc_type: banker
npc_role: banker
level: 3
max_hp: 22
ac: 11
awareness: 10
faction_id: tweakers
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 9
  grit: 11
  quickness: 14
  reasoning: 14
  savvy: 13
  flair: 9
banker:
  zone_id: oregon_country_fair
  base_rate: 0.98
  rate_variance: 0.02
  required_tier: known
loot:
  currency:
    min: 20
    max: 80
```

### Tweaker Quest Giver

- [ ] **Step 14.10: Create `content/npcs/tweaker_quest_giver.yaml`**

```yaml
id: tweaker_quest_giver
name: "Lieutenant Scratch"
description: >
  Crystal Karen's intelligence officer, walls of notes behind him, eyes
  never still. He assigns field work against Juggalos and Wooks with
  clinical precision.
type: human
npc_type: quest_giver
npc_role: quest_giver
level: 4
max_hp: 25
ac: 11
awareness: 11
faction_id: tweakers
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 9
  grit: 11
  quickness: 16
  reasoning: 15
  savvy: 14
  flair: 9
loot:
  currency:
    min: 0
    max: 0
```

### Wook Quest Giver

- [ ] **Step 14.11: Create `content/npcs/wook_quest_giver.yaml`**

```yaml
id: wook_quest_giver
name: "Council Elder"
description: >
  An ancient Wook who speaks slowly and means everything. The council fire
  burns behind him as he assigns missions against Juggalos and Tweakers.
  "The vibe is not right. We need your help to restore it."
type: human
npc_type: quest_giver
npc_role: quest_giver
level: 4
max_hp: 25
ac: 10
awareness: 10
faction_id: wooks
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "15m"
abilities:
  brutality: 8
  grit: 12
  quickness: 9
  reasoning: 14
  savvy: 14
  flair: 17
loot:
  currency:
    min: 0
    max: 0
```

### Chip Doc (OCF)

- [ ] **Step 14.12: Create `content/npcs/chip_doc_ocf.yaml`**

```yaml
id: chip_doc_ocf
name: "Neutral Node"
description: >
  A former Portland tech worker who drove out to the OCF years ago and never
  left. She operates from a rusted food cart in Market Row, maintaining strict
  neutrality. All three factions leave her alone because all three factions
  need her.
type: human
npc_type: chip_doc
npc_role: chip_doc
level: 3
max_hp: 20
ac: 10
awareness: 7
faction_id: ""
disposition: neutral
personality: cowardly
ai_domain: ""
respawn_delay: "20m"
abilities:
  brutality: 7
  grit: 11
  quickness: 10
  reasoning: 16
  savvy: 14
  flair: 12
chip_doc:
  removal_cost: 200
  check_dc: 14
loot:
  currency:
    min: 5
    max: 20
```

### Zone YAML Fix — chip_doc spawn template

- [ ] **Step 14.13: Update `content/zones/oregon_country_fair.yaml`**

In `ocf_market_row`, change the spawn template from `chip_doc` to `chip_doc_ocf`:

```yaml
    spawns:
      - template: chip_doc_ocf
        count: 1
        respawn_after: 0s
```

### Verification

- [ ] **Step 14.14: Run go test ./... and confirm all pass**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```

---

## REQ Coverage Matrix

| REQ | Task | Description |
|-----|------|-------------|
| REQ-OCF-1 | Task 9 | Zone id/name/faction_id/danger_level/world_x/world_y |
| REQ-OCF-2 | Task 9 | 34 rooms across territories |
| REQ-OCF-3 | Task 9 | Three safe clusters (3 rooms each) |
| REQ-OCF-4 | Task 9 | Three boss rooms, one per faction |
| REQ-OCF-5 | Task 9 | Danger levels per room type |
| REQ-OCF-6 | Task 9 | Each safe cluster: merchant, healer, quest_giver, fixer; chip_doc in market_row |
| REQ-OCF-7 | **Wooklyn plan** | AmbientSubstance field on world.Room (NOT re-implemented here) |
| REQ-OCF-8 | **Wooklyn plan** | 5-second ticker ambient dosing (NOT re-implemented here) |
| REQ-OCF-9 | **Wooklyn plan** | Startup validation of AmbientSubstance (NOT re-implemented here) |
| REQ-OCF-10 | Task 9 | 10 neutral/contested rooms |
| REQ-OCF-11 | Task 9 | 8 Juggalo territory rooms |
| REQ-OCF-12 | Task 9 | 8 Tweaker territory rooms with ambient_substance: tweaker_crystal |
| REQ-OCF-13 | Task 9 | 8 Wook territory rooms with ambient_substance: wook_spore |
| REQ-OCF-14 | Task 5.1 | juggalo NPC |
| REQ-OCF-15 | Task 5.2 | juggalette NPC with faygo_splash on-hit |
| REQ-OCF-16 | Tasks 5.3, 8.1 | juggalo_prophet NPC + HTN say domain |
| REQ-OCF-17 | Task 5.4 | violent_jimmy boss |
| REQ-OCF-18 | Task 6.1 | tweaker NPC |
| REQ-OCF-19 | Tasks 6.2, 8.2 | tweaker_paranoid NPC + HTN call_for_help domain |
| REQ-OCF-20 | Task 6.3 | tweaker_cook NPC |
| REQ-OCF-21 | Task 6.4 | crystal_karen boss |
| REQ-OCF-22 | Task 7 | spiral_king boss |
| REQ-OCF-23 | Task 9 | violent_jimmys_tent hazard: faygo_rain |
| REQ-OCF-24 | Task 9 | crystal_karens_lab hazards: toxic_fumes + trip_wire_grid |
| REQ-OCF-25 | Task 9 | the_spiral_kings_grove hazard: psychedelic_fog |
| REQ-OCF-26 | Tasks 5.4, 6.4, 7 | All three bosses: respawn_delay: 72h |
| REQ-OCF-27 | Task 1 | tweaker_crystal substance YAML |
| REQ-OCF-28 | Tasks 5.2 | faygo_splash on-hit effect on juggalette |
| REQ-OCF-29 | Task 2 | juggalos faction YAML |
| REQ-OCF-30 | Task 2 | juggalos exclusive items at family tier |
| REQ-OCF-31 | Task 2 | juggalos rep sources |
| REQ-OCF-32 | Task 3 | tweakers faction YAML |
| REQ-OCF-33 | Task 3 | tweakers exclusive items at inner_circle tier |
| REQ-OCF-34 | Task 3 | tweakers rep sources |
| REQ-OCF-35 | Task 4 | wooks zone_id: "" |
| REQ-OCF-36 | Task 4 | wooks rep from killing juggalos/tweakers in OCF — kill_juggalo/kill_tweaker rep_sources added to wooks.yaml |
| REQ-OCF-37 | Tasks 9, 10 | Quest giver NPC in each safe cluster |
| REQ-OCF-38 | Task 10 | Quest completion awards rep (stub handler defers to quests feature) |
| REQ-OCF-39 | Task 10 | Quest giver stub message + no-op handler |
| REQ-OCF-40 | Task 9 | Safe cluster rooms: 0 combat spawns |
| REQ-OCF-41 | Task 9 | Neutral rooms: 2–3 multi-faction spawns |
| REQ-OCF-42 | Task 9 | Faction sketchy rooms: 1–2 faction-specific spawns |
| REQ-OCF-43 | Task 9 | Faction dangerous rooms: 2–3 spawns with at least one elite |
| REQ-OCF-44 | Tasks 5.4, 6.4, 7, 9 | Boss rooms: named boss + 2 elite guards |

---

## Notes

- REQ-OCF-7, REQ-OCF-8, and REQ-OCF-9 (AmbientSubstance field, ticker, startup validation) are intentionally **NOT implemented here** — they are implemented by the wooklyn plan. This plan only uses the field in YAML.
- The `faygo_splash` item effect (REQ-OCF-28) uses the `on_hit_effect` YAML field on `juggalette.yaml`. The effect definition (`effect_type: entry_condition`, `damage: 1d4 acid`, `condition: humiliated -2 Flair 2 rounds`) is authored as an item effect record in `content/items/` by the equipment-mechanics plan; this plan only references it by ID.
- The `wook_shaman` NPC template is defined by the wooklyn plan; this plan references it in zone spawns.
- The `wook` and `wook_enforcer` NPC templates are defined by the wooklyn plan; this plan references them in zone spawns.
- The `chip_doc` NPC template is defined by the curse-removal plan; this plan references it in `ocf_market_row`.
- Faction-specific non-combat NPC variants (`juggalo_merchant`, `juggalo_healer`, `juggalo_fixer`, `tweaker_merchant`, `tweaker_healer`, `tweaker_fixer`, `wook_merchant`, `wook_healer`, `wook_fixer`, `juggalo_banker`, `tweaker_banker`, `wook_banker`) are defined as thin YAML variants extending base non-combat NPC templates; their full definitions are out of scope for this plan but MUST be created as part of this plan's implementation to satisfy REQ-OCF-6.

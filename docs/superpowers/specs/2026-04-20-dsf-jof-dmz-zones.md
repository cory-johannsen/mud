---
issue: 118,119
title: New zones — The Dick Sucking Factory, The Jerk Off Factory, and The DMZ
slug: dsf-jof-dmz-zones
date: 2026-04-20
---

## Summary

Add three interconnected zones to SE Industrial: The Dick Sucking Factory (DSF),
The Jerk Off Factory (JOF), and The DMZ — a contested no-man's-land connecting
the two warring corporate factions. Both factories are post-collapse corporate
parodies: armed middle managers, HR departments with guns, and bosses who never
stopped believing in the org chart. The DMZ uses the existing `hostile_factions`
mechanism so DSF and JOF NPCs engage each other when a player is present.

---

## Architecture Overview

- **Three new zone YAML files**: `dick_sucking_factory.yaml`,
  `jerk_off_factory.yaml`, `the_dmz.yaml` in `content/zones/`.
- **Two new faction YAML files**: `dick_sucking_factory.yaml`,
  `jerk_off_factory.yaml` in `content/factions/`, each listing the other as a
  hostile faction.
- **NPC template files**: DSF and JOF each get 3 standard NPC templates and 1
  boss NPC in `content/npcs/`.
- **HTN combat domains**: one per standard NPC type in `content/ai/`.
- **Pipe Yard exit additions**: `west` → DSF reception,
  `east` → JOF lobby added to `sei_pipe_yard` in `se_industrial.yaml`.
- Level range: 25–35 (matching SE Industrial) for standard rooms; DMZ and boss
  rooms elevated to 30–35.

---

## Zone Topology

```
                    [SE Industrial]
                    sei_pipe_yard
                   /              \
            west exit             east exit
               ↓                     ↓
    [DSF] dsf_reception       jof_lobby [JOF]
          dsf_open_office     jof_cubicle_farm
          dsf_production      jof_efficiency_floor
          dsf_qc_dept         jof_rd_lab
          dsf_break_room      jof_supply_closet
          dsf_hr_dept         jof_meeting_room
          dsf_loading_bay     jof_board_room
          dsf_exec_suite      jof_qa_dept
          dsf_corner_office   jof_break_room
          dsf_server_room     jof_crystals_office
               ↓                     ↓
         east exit from       west exit from
         dsf_loading_bay      jof_lobby (alternate)
               \                   /
                [THE DMZ]
                dmz_west_gate
                dmz_memo_fields
                dmz_vending_row
                dmz_no_mans_land
                dmz_overpass
                dmz_east_gate
```

---

## Requirements

### REQ-DSF-1: Dick Sucking Factory faction

A new file `content/factions/dick_sucking_factory.yaml` MUST be created:

```yaml
id: dick_sucking_factory
name: The Dick Sucking Factory
zone_id: dick_sucking_factory
hostile_factions:
  - jerk_off_factory
tiers:
  - id: visitor
    label: "Visitor"
    min_rep: 0
    price_discount: 0.0
  - id: contractor
    label: "Contractor"
    min_rep: 10
    price_discount: 0.05
  - id: associate
    label: "Associate"
    min_rep: 25
    price_discount: 0.15
  - id: employee
    label: "Employee"
    min_rep: 50
    price_discount: 0.25
rep_sources:
  - source: kill_jerk_off_factory
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: employee
  - source: quest_completion
    rep_per_completion: 15
  - source: fixer_payment
    rep_per_payment: 10
```

### REQ-DSF-2: Jerk Off Factory faction

A new file `content/factions/jerk_off_factory.yaml` MUST be created:

```yaml
id: jerk_off_factory
name: The Jerk Off Factory
zone_id: jerk_off_factory
hostile_factions:
  - dick_sucking_factory
tiers:
  - id: visitor
    label: "Visitor"
    min_rep: 0
    price_discount: 0.0
  - id: temp
    label: "Temp Worker"
    min_rep: 10
    price_discount: 0.05
  - id: associate
    label: "Associate"
    min_rep: 25
    price_discount: 0.15
  - id: full_time
    label: "Full-Time Employee"
    min_rep: 50
    price_discount: 0.25
rep_sources:
  - source: kill_dick_sucking_factory
    rep_per_level: 1
    cap_per_kill: 5
    cap_below_tier: full_time
  - source: quest_completion
    rep_per_completion: 15
  - source: fixer_payment
    rep_per_payment: 10
```

### REQ-DSF-3: DSF NPC templates

**Floor Supervisor** — `content/npcs/dsf_floor_supervisor.yaml`:

```yaml
id: dsf_floor_supervisor
name: Floor Supervisor
description: >
  A DSF floor supervisor in a hard hat and a clip-on tie, holding a
  clipboard that has been modified to fire staples at high velocity.
  They have not met their quarterly targets and are taking it personally.
level: 27
max_hp: 240
ac: 20
rob_multiplier: 1.0
awareness: 8
faction_id: dick_sucking_factory
disposition: hostile
ai_domain: dsf_worker_combat
attack_verb: "clips with their modified clipboard"
taunts:
  - "You're not on the visitor log. This is a compliance issue."
  - "I have KPIs to meet. You're an obstacle to my KPIs."
  - "This is a safety violation AND a trespassing incident."
  - "I'm going to need you to fill out an incident report. After."
  - "Per my last memo, unauthorized personnel are to be removed by force."
loot:
  currency:
    min: 15
    max: 50
  items:
    - item: clipboard
      chance: 0.3
      min_qty: 1
      max_qty: 1
weapon:
  - id: stun_baton
    weight: 2
  - id: combat_knife
    weight: 1
armor:
  - id: tactical_vest
    weight: 2
  - id: leather_jacket
    weight: 1
```

**Middle Manager** — `content/npcs/dsf_middle_manager.yaml`:

```yaml
id: dsf_middle_manager
name: Middle Manager
description: >
  A DSF middle manager in a rumpled button-down shirt, carrying a
  laminated org chart as a weapon. They have been middle management
  since before the collapse and see no reason to stop now.
level: 29
max_hp: 270
ac: 21
rob_multiplier: 1.2
awareness: 9
faction_id: dick_sucking_factory
disposition: hostile
ai_domain: dsf_worker_combat
attack_verb: "strikes with their laminated org chart"
taunts:
  - "I will escalate this to my supervisor. After I deal with you."
  - "You don't understand the chain of command here."
  - "This is highly irregular and I will be noting it in my report."
  - "Per our operational guidelines, you are to be terminated. Effective immediately."
  - "I've survived three restructurings. You're not a challenge."
loot:
  currency:
    min: 25
    max: 75
  items:
    - item: circuit_board
      chance: 0.25
      min_qty: 1
      max_qty: 2
weapon:
  - id: vibroblade
    weight: 2
  - id: stun_baton
    weight: 1
armor:
  - id: tactical_vest
    weight: 3
  - id: kevlar_vest
    weight: 1
```

**HR Representative** — `content/npcs/dsf_hr_rep.yaml`:

```yaml
id: dsf_hr_rep
name: HR Representative
description: >
  A DSF HR representative in a blazer that was professional before the
  collapse. They carry a personnel file on everyone and have added
  your threat assessment to the onboarding checklist.
level: 31
max_hp: 300
ac: 22
rob_multiplier: 1.3
awareness: 10
faction_id: dick_sucking_factory
disposition: hostile
ai_domain: dsf_hr_combat
attack_verb: "files a grievance against"
taunts:
  - "Your behavior is a violation of our code of conduct."
  - "I've already documented this incident. Three copies."
  - "We have a zero-tolerance policy on unauthorized entry."
  - "This is going in your permanent record."
  - "I have terminated employees for less. Much less."
loot:
  currency:
    min: 35
    max: 100
  items:
    - item: boarding_pass
      chance: 0.2
      min_qty: 1
      max_qty: 1
weapon:
  - id: assault_rifle
    weight: 2
  - id: vibroblade
    weight: 1
armor:
  - id: kevlar_vest
    weight: 3
  - id: tactical_vest
    weight: 1
```

**Will Blunderfield, Chief Chugging Officer** — `content/npcs/will_blunderfield.yaml`:

```yaml
id: will_blunderfield
name: Will Blunderfield
description: >
  The Chief Chugging Officer of the Dick Sucking Factory, Will Blunderfield
  has held his position through four hostile takeover attempts, two
  restructurings, the collapse of civilization, and a merger that nobody
  talks about. He carries a presentation remote and considers it his
  most lethal weapon. He is not wrong.
type: human
gender: male
tier: boss
level: 35
max_hp: 1240
ac: 25
rob_multiplier: 2.0
awareness: 11
faction_id: dick_sucking_factory
disposition: hostile
attack_verb: "leverages synergistic force against"
ai_domain: dsf_boss_combat
respawn_delay: "24h"
toughness_rank: expert
hustle_rank: trained
cool_rank: expert
abilities:
  brutality: 16
  grit: 18
  quickness: 14
  reasoning: 20
  savvy: 18
  flair: 16
weapon:
  - id: stun_baton
    weight: 1
armor:
  - id: military_plate
    weight: 1
boss_abilities:
  - id: performance_review
    name: "Performance Review"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_condition: demoralized
  - id: budget_cut
    name: "Budget Cut"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_damage_expr: "4d8"
  - id: synergy_strike
    name: "Synergy Strike"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      aoe_damage_expr: "6d10"
taunts:
  - "I've been Chief Chugging Officer for eleven years. You are a Q3 problem."
  - "Your performance metrics are unacceptable."
  - "This is a hostile work environment. I created it."
  - "Per the org chart, I am at the top. You are not on it."
  - "I will circle back to your defeat in the post-mortem."
loot:
  currency:
    min: 150
    max: 400
  items:
    - item: military_plate
      chance: 0.6
      min_qty: 1
      max_qty: 1
    - item: circuit_board
      chance: 0.9
      min_qty: 2
      max_qty: 5
```

### REQ-DSF-4: JOF NPC templates

**Efficiency Consultant** — `content/npcs/jof_efficiency_consultant.yaml`:

```yaml
id: jof_efficiency_consultant
name: Efficiency Consultant
description: >
  A JOF efficiency consultant with a stopwatch and a clipboard covered
  in throughput charts. They have optimized every process in the factory
  and are currently optimizing the process of removing you.
level: 27
max_hp: 235
ac: 20
rob_multiplier: 1.0
awareness: 9
faction_id: jerk_off_factory
disposition: hostile
ai_domain: jof_worker_combat
attack_verb: "optimizes their attack against"
taunts:
  - "Your movement patterns are inefficient. I've noted three improvements."
  - "Time is a resource. You're wasting mine."
  - "I've benchmarked this encounter. You're below average."
  - "Inefficiency is the enemy. You are also the enemy."
  - "Per my analysis, you have a 12% chance of surviving this interaction."
loot:
  currency:
    min: 15
    max: 50
weapon:
  - id: stun_baton
    weight: 2
  - id: combat_knife
    weight: 1
armor:
  - id: tactical_vest
    weight: 2
  - id: leather_jacket
    weight: 1
```

**Quality Assurance Officer** — `content/npcs/jof_qa_officer.yaml`:

```yaml
id: jof_qa_officer
name: Quality Assurance Officer
description: >
  A JOF QA officer in a white coat carrying a checklist that has been
  laminated for durability. Nothing meets their standards. Nothing ever
  will. They have found seventeen defects in you already.
level: 29
max_hp: 265
ac: 21
rob_multiplier: 1.2
awareness: 10
faction_id: jerk_off_factory
disposition: hostile
ai_domain: jof_worker_combat
attack_verb: "files a defect report on"
taunts:
  - "You fail to meet our minimum quality standards."
  - "I've documented seventeen deficiencies in your approach."
  - "This does not meet spec. Neither do you."
  - "Rejected. Return to sender."
  - "My checklist has a line for this. It says 'unacceptable'."
loot:
  currency:
    min: 25
    max: 75
  items:
    - item: circuit_board
      chance: 0.2
      min_qty: 1
      max_qty: 2
weapon:
  - id: vibroblade
    weight: 2
  - id: stun_baton
    weight: 1
armor:
  - id: tactical_vest
    weight: 3
  - id: kevlar_vest
    weight: 1
```

**Regional Liaison** — `content/npcs/jof_regional_liaison.yaml`:

```yaml
id: jof_regional_liaison
name: Regional Liaison
description: >
  A JOF regional liaison in a rumpled suit that was pressed this morning
  despite everything. They coordinate between departments and regions
  that no longer exist and have adapted this skill to coordinating
  violence with impressive efficiency.
level: 31
max_hp: 295
ac: 22
rob_multiplier: 1.3
awareness: 10
faction_id: jerk_off_factory
disposition: hostile
ai_domain: jof_liaison_combat
attack_verb: "liaises their fist into"
taunts:
  - "I coordinate across seventeen regions. Twelve still exist."
  - "My role is cross-functional. So is my approach to threats."
  - "I've aligned stakeholders against worse than you."
  - "This has been escalated to my level. My level handles it directly."
  - "Regional liaison means I solve problems. You are a problem."
loot:
  currency:
    min: 35
    max: 100
weapon:
  - id: assault_rifle
    weight: 2
  - id: vibroblade
    weight: 1
armor:
  - id: kevlar_vest
    weight: 3
  - id: tactical_vest
    weight: 1
```

**Joe Crystal, Chief Crystal Charging Officer** — `content/npcs/joe_crystal.yaml`:

```yaml
id: joe_crystal
name: Joe Crystal
description: >
  The Chief Crystal Charging Officer of the Jerk Off Factory, Joe Crystal
  runs the most efficient operation in post-collapse Portland and never
  lets anyone forget it. He speaks exclusively in KPIs and has not
  taken a day off since 2019. The stopwatch around his neck is not
  decorative. Nothing about Joe Crystal is decorative.
type: human
gender: male
tier: boss
level: 35
max_hp: 1210
ac: 24
rob_multiplier: 2.0
awareness: 12
faction_id: jerk_off_factory
disposition: hostile
attack_verb: "optimizes the elimination of"
ai_domain: jof_boss_combat
respawn_delay: "24h"
toughness_rank: trained
hustle_rank: expert
cool_rank: expert
abilities:
  brutality: 14
  grit: 16
  quickness: 20
  reasoning: 20
  savvy: 18
  flair: 12
weapon:
  - id: vibroblade
    weight: 1
armor:
  - id: military_plate
    weight: 1
boss_abilities:
  - id: time_is_money
    name: "Time is Money"
    trigger: round_start
    trigger_value: 0
    cooldown: "3m"
    effect:
      aoe_damage_expr: "3d6"
  - id: crystal_clarity
    name: "Crystal Clarity"
    trigger: hp_pct_below
    trigger_value: 75
    cooldown: "4m"
    effect:
      aoe_condition: confused
  - id: efficiency_drive
    name: "Efficiency Drive"
    trigger: hp_pct_below
    trigger_value: 50
    cooldown: "5m"
    effect:
      aoe_damage_expr: "5d10"
taunts:
  - "I've processed your threat assessment. The result is suboptimal. For you."
  - "Every second you spend here is a second the factory loses productivity."
  - "My throughput is 340% above baseline. Yours is not."
  - "Inefficiency is eliminated at the JOF. You are being eliminated."
  - "Crystal charging targets met. You are not a target I will miss."
loot:
  currency:
    min: 150
    max: 400
  items:
    - item: military_plate
      chance: 0.6
      min_qty: 1
      max_qty: 1
    - item: circuit_board
      chance: 0.9
      min_qty: 2
      max_qty: 5
```

### REQ-DSF-5: HTN combat domains

**DSF worker domain** — `content/ai/dsf_worker_combat.yaml`:

```yaml
domain:
  id: dsf_worker_combat
  description: Standard DSF worker. Attacks and harasses with corporate jargon.
  tasks:
    - id: behave
      description: Root task
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [attack_enemy]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: do_pass
      action: pass
      target: ""
```

**DSF HR domain** — `content/ai/dsf_hr_combat.yaml`:

```yaml
domain:
  id: dsf_hr_combat
  description: DSF HR rep. Attacks and applies demoralized condition.
  tasks:
    - id: behave
      description: Root task
    - id: process
      description: HR processing
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [process]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
    - task: process
      id: discipline_and_attack
      precondition: in_combat
      subtasks: [hr_discipline, attack_enemy]
    - task: process
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: hr_discipline
      action: apply_mental_state
      track: despair
      severity: minor
      target: lowest_hp_enemy
      cooldown_rounds: 3
      ap_cost: 1
    - id: do_pass
      action: pass
      target: ""
```

**JOF worker domain** — `content/ai/jof_worker_combat.yaml`:

```yaml
domain:
  id: jof_worker_combat
  description: Standard JOF worker. Fast attacker, efficiency-focused.
  tasks:
    - id: behave
      description: Root task
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [attack_enemy]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: do_pass
      action: pass
      target: ""
```

**JOF liaison domain** — `content/ai/jof_liaison_combat.yaml`:

```yaml
domain:
  id: jof_liaison_combat
  description: JOF regional liaison. Attacks and applies confusion.
  tasks:
    - id: behave
      description: Root task
    - id: liaise
      description: Cross-functional violence
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [liaise]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
    - task: liaise
      id: confuse_and_attack
      precondition: in_combat
      subtasks: [jof_confuse, attack_enemy]
    - task: liaise
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: jof_confuse
      action: apply_mental_state
      track: delirium
      severity: minor
      target: nearest_enemy
      cooldown_rounds: 3
      ap_cost: 1
    - id: do_pass
      action: pass
      target: ""
```

**DSF boss domain** — `content/ai/dsf_boss_combat.yaml`:

```yaml
domain:
  id: dsf_boss_combat
  description: Will Blunderfield. Attacks and applies corporate pressure.
  tasks:
    - id: behave
      description: Root task
    - id: manage
      description: Executive management
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [manage]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
    - task: manage
      id: strike_and_demoralize
      precondition: lieutenant_enemy_below_half
      subtasks: [strike_enemy, demoralize]
    - task: manage
      id: attack_and_demoralize
      precondition: in_combat
      subtasks: [attack_enemy, demoralize]
    - task: manage
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: strike_enemy
      action: strike
      target: weakest_enemy
    - id: demoralize
      action: apply_mental_state
      track: despair
      severity: moderate
      target: lowest_hp_enemy
      cooldown_rounds: 4
      ap_cost: 2
    - id: do_pass
      action: pass
      target: ""
```

**JOF boss domain** — `content/ai/jof_boss_combat.yaml`:

```yaml
domain:
  id: jof_boss_combat
  description: Joe Crystal. Fast, efficient attacks with confusion pressure.
  tasks:
    - id: behave
      description: Root task
    - id: optimize
      description: Combat optimization
  methods:
    - task: behave
      id: combat_mode
      precondition: in_combat
      subtasks: [optimize]
    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [do_pass]
    - task: optimize
      id: strike_and_confuse
      precondition: lieutenant_enemy_below_half
      subtasks: [strike_enemy, jof_boss_confuse]
    - task: optimize
      id: attack_and_confuse
      precondition: in_combat
      subtasks: [attack_enemy, jof_boss_confuse]
    - task: optimize
      id: attack_only
      precondition: ""
      subtasks: [attack_enemy]
  operators:
    - id: attack_enemy
      action: attack
      target: nearest_enemy
    - id: strike_enemy
      action: strike
      target: weakest_enemy
    - id: jof_boss_confuse
      action: apply_mental_state
      track: delirium
      severity: moderate
      target: lowest_hp_enemy
      cooldown_rounds: 4
      ap_cost: 2
    - id: do_pass
      action: pass
      target: ""
```

### REQ-DSF-6: Dick Sucking Factory zone

A new file `content/zones/dick_sucking_factory.yaml` MUST be created:

```yaml
zone:
  id: dick_sucking_factory
  danger_level: dangerous
  min_level: 25
  max_level: 35
  world_x: 1
  world_y: 2
  name: The Dick Sucking Factory
  description: >
    A repurposed industrial facility that survived the collapse with its
    org chart intact. The DSF operates with the same bureaucratic
    precision it always did, with the addition of firearms and a
    general willingness to apply disciplinary procedures by force. The
    smell of industrial lubricant and printer toner has never fully left.
  start_room: dsf_reception
  rooms:
  - id: dsf_reception
    title: Reception
    danger_level: dangerous
    description: >
      The reception desk is still manned. The receptionist has been
      replaced by a floor supervisor who considers visitor management
      part of their remit. The sign-in sheet has a new column: threat
      level. You have been assessed.
    exits:
    - direction: east
      target: sei_pipe_yard
      zone: se_industrial
    - direction: west
      target: dsf_open_office
    map_x: 0
    map_y: 0
    spawns:
    - template: dsf_floor_supervisor
      count: 2
      respawn_after: 5m
    equipment:
    - item_id: zone_map
      max_count: 1
      respawn_after: 0s
      immovable: true
      script: zone_map_use
      description: Zone Map

  - id: dsf_open_office
    title: Open Plan Office
    danger_level: dangerous
    description: >
      Rows of desks stretch toward the far wall, each neatly maintained
      despite the bullet holes. Motivational posters line the walls —
      "SYNERGY IS SURVIVAL" and "THERE IS NO I IN TEAM, ONLY IN TERMINATED."
      Middle managers patrol the aisles with clipboards and sidearms.
    exits:
    - direction: east
      target: dsf_reception
    - direction: west
      target: dsf_production
    - direction: north
      target: dsf_break_room
    - direction: south
      target: dsf_hr_dept
    map_x: -2
    map_y: 0
    spawns:
    - template: dsf_middle_manager
      count: 2
      respawn_after: 5m
    - template: dsf_floor_supervisor
      count: 1
      respawn_after: 5m

  - id: dsf_production
    title: Production Floor
    danger_level: dangerous
    description: >
      The factory floor hums with machinery that has been repurposed in
      ways the original designers did not intend. Workers in hard hats
      move between stations with the same efficiency they always had,
      handling materials that have changed considerably since the collapse.
      The output metrics are posted by the door. They are meeting targets.
    exits:
    - direction: east
      target: dsf_open_office
    - direction: west
      target: dsf_qc_dept
    - direction: north
      target: dsf_server_room
    map_x: -4
    map_y: 0
    spawns:
    - template: dsf_floor_supervisor
      count: 3
      respawn_after: 5m

  - id: dsf_qc_dept
    title: Quality Control
    danger_level: dangerous
    description: >
      The QC department has maintained rigorous standards. Every product
      is tested. Every process is documented. Every unauthorized visitor
      is logged, assessed, and addressed per established protocol.
      The protocol has been updated to include lethal force as a standard
      resolution pathway.
    exits:
    - direction: east
      target: dsf_production
    - direction: south
      target: dsf_loading_bay
    map_x: -6
    map_y: 0
    spawns:
    - template: dsf_middle_manager
      count: 2
      respawn_after: 5m
    - template: dsf_hr_rep
      count: 1
      respawn_after: 8m

  - id: dsf_break_room
    title: Break Room
    danger_level: safe
    description: >
      A break room that has been maintained with institutional care.
      The coffee machine still works. Someone has kept the microwave
      clean. A whiteboard lists the current employee of the month.
      The winner has been the same person for eleven months. Nobody
      mentions this.
    exits:
    - direction: south
      target: dsf_open_office
    map_x: -2
    map_y: 2
    spawns:
    - template: dsf_quest_giver
      count: 1
      respawn_after: 0s
    - template: dsf_fixer
      count: 1
      respawn_after: 0s

  - id: dsf_hr_dept
    title: Human Resources
    danger_level: all_out_war
    description: >
      HR has always been the most dangerous department. This has not
      changed. The open-door policy has been replaced with a reinforced
      security door policy. HR representatives sit behind bulletproof
      glass and process disciplinary actions with bureaucratic precision
      and fully automatic weapons.
    exits:
    - direction: north
      target: dsf_open_office
    - direction: west
      target: dsf_exec_suite
    map_x: -2
    map_y: -2
    spawns:
    - template: dsf_hr_rep
      count: 3
      respawn_after: 8m

  - id: dsf_loading_bay
    title: Loading Bay
    danger_level: dangerous
    description: >
      The loading bay is where product leaves the factory and where
      sorties depart for the DMZ. Industrial shelving holds product
      in labeled containers. Forklifts have been modified for offensive
      operations. The dock doors face east toward contested territory.
    exits:
    - direction: north
      target: dsf_qc_dept
    - direction: east
      target: dmz_west_gate
      zone: the_dmz
    map_x: -6
    map_y: -2
    spawns:
    - template: dsf_floor_supervisor
      count: 2
      respawn_after: 5m

  - id: dsf_exec_suite
    title: Executive Suite
    danger_level: all_out_war
    description: >
      The executive suite occupies the factory's highest floor, overlooking
      the production floor through glass that has been reinforced but not
      replaced after the bullet holes. Executive assistants operate with
      extreme competence and extreme hostility. Corner offices line the
      far wall. One is larger than the others.
    exits:
    - direction: east
      target: dsf_hr_dept
    - direction: west
      target: dsf_corner_office
    map_x: -4
    map_y: -2
    spawns:
    - template: dsf_hr_rep
      count: 2
      respawn_after: 8m
    - template: dsf_middle_manager
      count: 1
      respawn_after: 5m

  - id: dsf_corner_office
    title: "The Corner Office"
    danger_level: all_out_war
    core: true
    boss_room: true
    description: >
      Will Blunderfield's corner office has two windows, a view of the
      production floor, and a desk that survived the collapse because
      it was built from something heavier than the collapse. The org
      chart on the wall is color-coded and current. Will's name is at
      the top, in red, in a font that communicates authority.
      He is at his desk. He has been expecting you since Q2.
    exits:
    - direction: east
      target: dsf_exec_suite
    map_x: -6
    map_y: -4
    spawns:
    - template: will_blunderfield
      count: 1
      respawn_after: 24h
    - template: dsf_hr_rep
      count: 1
      respawn_after: 8m

  - id: dsf_server_room
    title: Server Room
    danger_level: dangerous
    description: >
      Cold, humming, and blue-lit. The servers that ran the factory's
      operations before the collapse have been repurposed for something
      nobody on the floor will discuss. The temperature is kept precisely
      regulated. The data is kept precisely classified.
    exits:
    - direction: south
      target: dsf_production
    map_x: -4
    map_y: 2
    spawns:
    - template: dsf_floor_supervisor
      count: 1
      respawn_after: 5m
    - template: dsf_black_market_merchant
      count: 1
      respawn_after: 0s
```

### REQ-DSF-7: Jerk Off Factory zone

A new file `content/zones/jerk_off_factory.yaml` MUST be created:

```yaml
zone:
  id: jerk_off_factory
  danger_level: dangerous
  min_level: 25
  max_level: 35
  world_x: 3
  world_y: 2
  name: The Jerk Off Factory
  description: >
    A post-collapse industrial facility that has achieved perfect
    operational efficiency by eliminating everything that interfered
    with it, including mercy. The JOF runs on metrics, targets, and
    an unshakeable belief that every problem is a process problem.
    The floors are clean. The lines are straight. The body count is
    within acceptable parameters.
  start_room: jof_lobby
  rooms:
  - id: jof_lobby
    title: Lobby
    danger_level: dangerous
    description: >
      The lobby's visitor management system is operational. The system
      now includes a weapons check, a threat assessment, and an
      efficiency rating for the check-in process itself. You have been
      assessed as below standard. The consultants at the desk have
      been notified.
    exits:
    - direction: west
      target: sei_pipe_yard
      zone: se_industrial
    - direction: east
      target: jof_cubicle_farm
    - direction: south
      target: dmz_east_gate
      zone: the_dmz
    map_x: 0
    map_y: 0
    spawns:
    - template: jof_efficiency_consultant
      count: 2
      respawn_after: 5m
    equipment:
    - item_id: zone_map
      max_count: 1
      respawn_after: 0s
      immovable: true
      script: zone_map_use
      description: Zone Map

  - id: jof_cubicle_farm
    title: Cubicle Farm
    danger_level: dangerous
    description: >
      A grid of cubicles maintained with geometric precision. Each
      workstation is identical. Each occupant is armed. The motivational
      posters have been updated — "EFFICIENCY IS SURVIVAL" and "YOUR
      PERFORMANCE IS BEING MONITORED. SO ARE YOU." Consultants track
      your movement through the farm with clipboards and rifles.
    exits:
    - direction: west
      target: jof_lobby
    - direction: east
      target: jof_efficiency_floor
    - direction: north
      target: jof_break_room
    - direction: south
      target: jof_qa_dept
    map_x: 2
    map_y: 0
    spawns:
    - template: jof_efficiency_consultant
      count: 2
      respawn_after: 5m
    - template: jof_qa_officer
      count: 1
      respawn_after: 8m

  - id: jof_efficiency_floor
    title: Efficiency Floor
    danger_level: dangerous
    description: >
      The production floor runs on a six-sigma process that has been
      updated for current operational requirements. Every motion is
      optimized. Every second is tracked. Regional liaisons move
      between stations with the controlled urgency of people who have
      internalized the stopwatch as a worldview.
    exits:
    - direction: west
      target: jof_cubicle_farm
    - direction: east
      target: jof_rd_lab
    - direction: north
      target: jof_supply_closet
    map_x: 4
    map_y: 0
    spawns:
    - template: jof_regional_liaison
      count: 2
      respawn_after: 5m
    - template: jof_efficiency_consultant
      count: 1
      respawn_after: 5m

  - id: jof_rd_lab
    title: R&D Lab
    danger_level: dangerous
    description: >
      Research and Development has pivoted. The whiteboards still have
      equations on them. The equations have changed. Crystal charging
      research dominates the back wall in dense notation that suggests
      someone very smart is working on something you do not want to
      understand. The researchers are armed.
    exits:
    - direction: west
      target: jof_efficiency_floor
    - direction: north
      target: jof_board_room
    map_x: 6
    map_y: 0
    spawns:
    - template: jof_qa_officer
      count: 2
      respawn_after: 8m
    - template: jof_regional_liaison
      count: 1
      respawn_after: 5m

  - id: jof_supply_closet
    title: Supply Closet
    danger_level: all_out_war
    description: >
      The supply closet is labeled SUPPLY CLOSET. It is not a supply
      closet. The regional liaisons use it for ambushes and consider
      it a highly efficient use of available real estate. The shelves
      have been cleared to make room for the ambush. There is one
      shelf left. It holds spare clipboards.
    exits:
    - direction: south
      target: jof_efficiency_floor
    map_x: 4
    map_y: 2
    spawns:
    - template: jof_regional_liaison
      count: 3
      respawn_after: 5m

  - id: jof_meeting_room
    title: Meeting Room
    danger_level: dangerous
    description: >
      The meeting room has a table, chairs, a whiteboard, and a
      standing agenda that has not changed since the collapse. The
      agenda items are: objectives, blockers, and escalations. You
      are currently on the agenda under escalations.
    exits:
    - direction: east
      target: jof_board_room
    map_x: 2
    map_y: 2
    spawns:
    - template: jof_quest_giver
      count: 1
      respawn_after: 0s
    - template: jof_fixer
      count: 1
      respawn_after: 0s

  - id: jof_break_room
    title: Break Room
    danger_level: safe
    description: >
      The JOF break room operates on a scheduled rotation. Breaks are
      twelve minutes. The coffee is measured to the milliliter. The
      vending machine has been restocked with items approved by the
      nutrition optimization committee. Someone has written "HELP" on
      the whiteboard. It has been there for eight months.
    exits:
    - direction: south
      target: jof_cubicle_farm
    map_x: 2
    map_y: 2
    spawns:
    - template: jof_black_market_merchant
      count: 1
      respawn_after: 0s

  - id: jof_qa_dept
    title: Quality Assurance
    danger_level: all_out_war
    description: >
      QA occupies a fortified section of the factory floor. Every
      product and every person entering this section is tested. The
      testing process has a zero percent false-positive rate and a
      100 percent resolution rate. Officers process assessments with
      thoroughness and automatic weapons.
    exits:
    - direction: north
      target: jof_cubicle_farm
    - direction: east
      target: jof_crystals_office
    map_x: 2
    map_y: -2
    spawns:
    - template: jof_qa_officer
      count: 3
      respawn_after: 8m

  - id: jof_board_room
    title: Board Room
    danger_level: all_out_war
    description: >
      The board room table seats twenty. Ten seats are filled. The
      other ten belonged to regional leads whose regions no longer
      exist. Their nameplates remain. The board meeting is in session.
      It has been in session since 2021. The agenda item is you.
    exits:
    - direction: west
      target: jof_meeting_room
    - direction: south
      target: jof_crystals_office
    map_x: 6
    map_y: 2
    spawns:
    - template: jof_regional_liaison
      count: 2
      respawn_after: 5m
    - template: jof_qa_officer
      count: 1
      respawn_after: 8m

  - id: jof_crystals_office
    title: "Crystal's Office"
    danger_level: all_out_war
    core: true
    boss_room: true
    description: >
      Joe Crystal's office contains a desk, a chair, a stopwatch, and
      nothing that does not serve a purpose. The stopwatch is running.
      It has been running since he clocked in on January 14th, 2019.
      Joe is standing when you enter. He was expecting you at 14:32.
      It is 14:34. He has already noted the inefficiency.
    exits:
    - direction: north
      target: jof_board_room
    - direction: west
      target: jof_qa_dept
    map_x: 6
    map_y: -2
    spawns:
    - template: joe_crystal
      count: 1
      respawn_after: 24h
    - template: jof_qa_officer
      count: 1
      respawn_after: 8m
```

### REQ-DSF-8: The DMZ zone

A new file `content/zones/the_dmz.yaml` MUST be created:

```yaml
zone:
  id: the_dmz
  danger_level: all_out_war
  min_level: 30
  max_level: 35
  world_x: 2
  world_y: 2
  name: The DMZ
  description: >
    The contested buffer zone between the Dick Sucking Factory and the
    Jerk Off Factory. Neither side controls it. Both sides send sorties
    through it. The ground is covered in shredded memos, overturned
    office furniture, and motivational posters that have been used as
    cover. The vending machines have bullet holes. Both factions have
    been trying to destroy them for three years. The machines are
    still running.
  start_room: dmz_west_gate
  rooms:
  - id: dmz_west_gate
    title: West Gate (DSF Side)
    danger_level: all_out_war
    description: >
      The western entry to the DMZ from the DSF loading bay. DSF
      floor supervisors have established a forward position here
      behind overturned filing cabinets. The filing cabinets are
      labeled. The labels are still legible.
    exits:
    - direction: west
      target: dsf_loading_bay
      zone: dick_sucking_factory
    - direction: east
      target: dmz_memo_fields
    map_x: 0
    map_y: 0
    spawns:
    - template: dsf_floor_supervisor
      count: 2
      respawn_after: 5m
    - template: jof_efficiency_consultant
      count: 1
      respawn_after: 5m

  - id: dmz_memo_fields
    title: The Memo Fields
    danger_level: all_out_war
    description: >
      Thousands of memos cover the ground here, dispersed from the
      original memo exchange that started the war. They are not
      legible at a distance. Up close, they are mostly about parking.
      Both sides have used them as camouflage. Neither side has read
      them.
    exits:
    - direction: west
      target: dmz_west_gate
    - direction: east
      target: dmz_vending_row
    - direction: north
      target: dmz_overpass
    map_x: 2
    map_y: 0
    spawns:
    - template: dsf_middle_manager
      count: 1
      respawn_after: 5m
    - template: jof_efficiency_consultant
      count: 1
      respawn_after: 5m

  - id: dmz_vending_row
    title: Vending Machine Row
    danger_level: all_out_war
    description: >
      A row of five vending machines stand in the center of the DMZ,
      untouched by mutual agreement and constant fire. They are fully
      operational. Nobody uses them during active sorties. Between
      sorties, both sides use them constantly. The agreement on the
      vending machines is the only agreement both sides have honored
      for three years.
    exits:
    - direction: west
      target: dmz_memo_fields
    - direction: east
      target: dmz_no_mans_land
    map_x: 4
    map_y: 0
    spawns:
    - template: dsf_floor_supervisor
      count: 1
      respawn_after: 5m
    - template: jof_qa_officer
      count: 1
      respawn_after: 8m

  - id: dmz_no_mans_land
    title: No Man's Land
    danger_level: all_out_war
    description: >
      The central DMZ. No faction controls this ground. Both factions
      contest it continuously. Office chairs, printer cartridges,
      and a whiteboard reading "Q4 OBJECTIVES" have been used as
      fortifications. The whiteboard has taken three direct hits.
      Q4 objectives remain partially visible. They are ambitious.
    exits:
    - direction: west
      target: dmz_vending_row
    - direction: east
      target: dmz_east_gate
    map_x: 6
    map_y: 0
    spawns:
    - template: dsf_hr_rep
      count: 1
      respawn_after: 8m
    - template: jof_regional_liaison
      count: 1
      respawn_after: 5m

  - id: dmz_overpass
    title: The Overpass
    danger_level: all_out_war
    description: >
      A loading overpass provides elevated line-of-sight across the
      DMZ. Both factions contest the overpass continuously. Whoever
      holds it controls the high ground. Currently, nobody holds it.
      The previous occupants are still there, in a sense.
    exits:
    - direction: south
      target: dmz_memo_fields
    map_x: 2
    map_y: 2
    spawns:
    - template: dsf_middle_manager
      count: 1
      respawn_after: 5m
    - template: jof_efficiency_consultant
      count: 2
      respawn_after: 5m

  - id: dmz_east_gate
    title: East Gate (JOF Side)
    danger_level: all_out_war
    description: >
      The eastern entry to the DMZ from the JOF lobby. JOF efficiency
      consultants have established a forward position here. Their
      fortifications have been assessed for structural efficiency.
      The assessment was positive. The fortifications are good.
    exits:
    - direction: east
      target: jof_lobby
      zone: jerk_off_factory
    - direction: west
      target: dmz_no_mans_land
    map_x: 8
    map_y: 0
    spawns:
    - template: jof_efficiency_consultant
      count: 2
      respawn_after: 5m
    - template: dsf_floor_supervisor
      count: 1
      respawn_after: 5m
```

### REQ-DSF-9: Pipe Yard exit additions

`content/zones/se_industrial.yaml` room `sei_pipe_yard` MUST have two new exits
added:

```yaml
- direction: west
  target: dsf_reception
  zone: dick_sucking_factory
- direction: east
  target: jof_lobby
  zone: jerk_off_factory
```

### REQ-DSF-10: NPC service stubs

The following NPC template IDs are referenced in zone spawns and MUST be created
as minimal YAML files in `content/npcs/` following the existing quest giver,
fixer, and merchant patterns:

- `dsf_quest_giver` — DSF-aligned quest giver in dsf_break_room
- `dsf_fixer` — DSF-aligned fixer in dsf_break_room
- `dsf_black_market_merchant` — DSF-aligned merchant in dsf_server_room
- `jof_quest_giver` — JOF-aligned quest giver in jof_meeting_room
- `jof_fixer` — JOF-aligned fixer in jof_meeting_room
- `jof_black_market_merchant` — JOF-aligned merchant in jof_break_room

### REQ-DSF-11: DMZ mutual hostility invariant

- REQ-DSF-11a: All DSF NPCs MUST have `faction_id: dick_sucking_factory`.
- REQ-DSF-11b: All JOF NPCs MUST have `faction_id: jerk_off_factory`.
- REQ-DSF-11c: The `dick_sucking_factory` faction MUST list `jerk_off_factory`
  in its `hostile_factions`. The `jerk_off_factory` faction MUST list
  `dick_sucking_factory` in its `hostile_factions`.
- REQ-DSF-11d: When a player enters a DMZ room containing both DSF and JOF NPCs,
  the existing hostile_factions aggression system MUST cause them to engage each
  other. No new engine changes are required.

### REQ-DSF-12: Test coverage

- REQ-DSF-12a: Zone validation — all three zone YAML files MUST load and pass
  the existing zone validator without errors.
- REQ-DSF-12b: Faction validation — both faction YAML files MUST load and
  register correctly.
- REQ-DSF-12c: NPC validation — all NPC templates MUST load without errors.
- REQ-DSF-12d: HTN domain validation — all 6 combat domains MUST load without
  errors.
- REQ-DSF-12e: `TestDMZ_MutualHostility` — DSF and JOF NPCs in the same room
  are mutually hostile per the faction system.
- REQ-DSF-12f: `TestPipeYard_HasDSFAndJOFExits` — `sei_pipe_yard` has west exit
  to DSF and east exit to JOF.

---

## Out of Scope

- Zone-specific quests (quest givers are stubbed; quest content is a follow-on).
- Faction reputation progression (infrastructure is in place via faction YAML;
  rep-earning quests are out of scope for this spec).
- World map connections for the DMZ (it is an interior zone not shown on the
  world map independently).

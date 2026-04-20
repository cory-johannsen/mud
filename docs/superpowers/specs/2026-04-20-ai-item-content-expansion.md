---
issue: 107
title: AI Item Content Expansion — full set of AI items per team
slug: ai-item-content-expansion
date: 2026-04-20
---

## Summary

Expand the AI item roster to one item per item type (melee weapon, ranged
weapon, light armor, medium armor, heavy armor, shield) for each team. The AI
Chainsaw (Machete melee) and AI AK-47 (Gun ranged) are already specced in
`2026-04-20-ai-item-content.md`. This spec covers the remaining 10 items, their
HTN domains, Lua scripts, boss drop placements, and Cipher quest files.

This spec depends on:
- `2026-04-20-ai-item-engine.md` — engine must be implemented first
- `2026-04-20-ai-item-content.md` — Chainsaw/AK-47 content and Cipher NPC
- `2026-04-20-ai-item-quest-delivery.md` — Signal in the Static quest chains

---

## Item Roster

| Team | Type | Item ID | Base Ref | Personality | Boss Drop |
|------|------|---------|----------|-------------|-----------|
| Machete | Ranged | `ai_sawn_off` | `sawn_off` | Revolutionary | `gangbang` |
| Machete | Light Armor | `ai_machete_armor_light` | `leather_jacket` | Graffiti Artist | `the_big_3` |
| Machete | Medium Armor | `ai_machete_armor_medium` | `tactical_vest` | Saboteur | `the_big_3` |
| Machete | Heavy Armor | `ai_machete_armor_heavy` | `military_plate` | Fortress | `the_big_3` |
| Machete | Shield | `ai_machete_shield` | `riot_shield` | Propagandist | `papa_wook` |
| Gun | Melee | `ai_combat_knife` | `combat_knife` | Tactician | `gangbang` |
| Gun | Light Armor | `ai_gun_armor_light` | `leather_jacket` | Prepper | `the_big_3` |
| Gun | Medium Armor | `ai_gun_armor_medium` | `tactical_vest` | Patriot | `the_big_3` |
| Gun | Heavy Armor | `ai_gun_armor_heavy` | `military_plate` | Armory | `the_big_3` |
| Gun | Shield | `ai_gun_shield` | `ballistic_shield` | Insurance | `papa_wook` |

---

## Requirements

### REQ-AICE-1: AI Sawn-Off Shotgun (Machete ranged)

**Item** — `content/items/ai_sawn_off.yaml`:

```yaml
id: ai_sawn_off
name: AI Sawn-Off
description: >
  A double-barrel sawn-off with a salvaged neural chip soldered to the
  stock. It knows what it's for. It knows who it's for. Point it at power.
kind: weapon
weapon_ref: sawn_off
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_sawn_off_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.enough_ap = function(self)
    return self.combat.player.ap >= 2
  end

  operators.revolutionary_blast = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "2d8+2", 2)
      self.engine.say({
        "Point it at power.",
        "No gods, no masters, just this.",
        "The revolution is close range.",
        "For the collective.",
        "They never see it coming from one of us."
      })
    end
  end

  operators.quick_shot = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.attack(target.id, "1d8")
      self.engine.say({"solidarity.", "hold the line.", "keep moving."})
    end
  end
```

**HTN domain** — `content/ai/ai_sawn_off_combat.yaml`:

```yaml
domain:
  id: ai_sawn_off_combat
  description: Working-class revolutionary. Targets weakest enemy, close-range devastation.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: full_blast
      precondition: enough_ap
      subtasks: [revolutionary_blast]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_shot]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_shot]

  operators:
    - id: revolutionary_blast
      action: lua_hook
      ap_cost: 2

    - id: quick_shot
      action: lua_hook
      ap_cost: 1
```

### REQ-AICE-2: AI Combat Knife (Gun melee)

**Item** — `content/items/ai_combat_knife.yaml`:

```yaml
id: ai_combat_knife
name: AI Combat Knife
description: >
  A tactical blade with a targeting module grafted to the grip. It has
  mapped every firearm's minimum engagement range and considers itself
  the solution to all of them.
kind: weapon
weapon_ref: combat_knife
weight: 0.5
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_combat_knife_combat
combat_script: |
  preconditions.has_enemy = function(self)
    return #self.combat.enemies > 0
  end

  preconditions.priority_target = function(self)
    return #self.combat.enemies > 0 and self.combat.enemies[1].hp > 0
  end

  operators.precision_strike = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d6+3", 2)
      self.engine.say({
        "Target acquired. Neutralized.",
        "Range: zero. Problem: solved.",
        "Every firearm has a minimum. I am the minimum.",
        "Threat assessment: complete.",
        "This is what I was designed for."
      })
    end
  end

  operators.quick_strike = function(self)
    local target = self.combat.nearest_enemy()
    if target then
      self.engine.attack(target.id, "1d4+2")
      self.engine.say({"Threat suppressed.", "Engaging.", "Contact."})
    end
  end
```

**HTN domain** — `content/ai/ai_combat_knife_combat.yaml`:

```yaml
domain:
  id: ai_combat_knife_combat
  description: Cold tactician. Precision strikes, covers range minimums.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: precision_mode
      precondition: priority_target
      subtasks: [precision_strike]

    - task: behave
      id: quick_mode
      precondition: has_enemy
      subtasks: [quick_strike]

    - task: behave
      id: idle_mode
      precondition: ""
      subtasks: [quick_strike]

  operators:
    - id: precision_strike
      action: lua_hook
      ap_cost: 2

    - id: quick_strike
      action: lua_hook
      ap_cost: 1
```

### REQ-AICE-3: Machete light armor — Graffiti Artist

**Item** — `content/items/ai_machete_armor_light.yaml`:

```yaml
id: ai_machete_armor_light
name: AI Street Jacket
description: >
  A leather jacket with a neural patch stitched inside the collar. It
  treats every fight like a performance and has strong opinions about
  your form.
kind: armor
armor_ref: leather_jacket
weight: 1.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_light_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.running_commentary = function(self)
    self.engine.say({
      "Nice dodge — you're getting it.",
      "That hit was sloppy. Better.",
      "They don't know what they're dealing with.",
      "Move like you mean it.",
      "This is your canvas. Make it count.",
      "Style and survival are not mutually exclusive."
    })
  end

  operators.mark_weakness = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.debuff(target.id, "exposed", 1)
      self.engine.say({
        "There — weak point. See it?",
        "Left side is open. Go.",
        "They telegraphed. Exploit it."
      })
    end
  end
```

**HTN domain** — `content/ai/ai_machete_armor_light_combat.yaml`:

```yaml
domain:
  id: ai_machete_armor_light_combat
  description: Graffiti artist. Running performance commentary, occasional weakness marking.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: mark_mode
      precondition: always
      subtasks: [mark_weakness]

    - task: behave
      id: commentary_mode
      precondition: always
      subtasks: [running_commentary]

  operators:
    - id: mark_weakness
      action: lua_hook
      ap_cost: 1

    - id: running_commentary
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-4: Machete medium armor — Saboteur

**Item** — `content/items/ai_machete_armor_medium.yaml`:

```yaml
id: ai_machete_armor_medium
name: AI Saboteur Vest
description: >
  A tactical vest with a targeting analysis module wired into the
  chest plate. It has identified seventeen weak points in every enemy
  it has ever seen. It will tell you about all of them.
kind: armor
armor_ref: tactical_vest
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_medium_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.expose_weakness = function(self)
    local target = self.combat.weakest_enemy()
    if target then
      self.engine.debuff(target.id, "weakened", 2)
      self.engine.say({
        "Defense compromised. Hit them now.",
        "Structural weakness identified. Exploit.",
        "Their guard is down on the right. See it.",
        "I've mapped their attack pattern. They're predictable.",
        "Weak point: center mass. You're welcome."
      })
    end
  end

  operators.analysis = function(self)
    self.engine.say({
      "Threat assessment in progress.",
      "Scanning for exploitable patterns.",
      "They think they're covered. They're not.",
      "Every defense has a seam. Finding it."
    })
  end
```

**HTN domain** — `content/ai/ai_machete_armor_medium_combat.yaml`:

```yaml
domain:
  id: ai_machete_armor_medium_combat
  description: Saboteur. Debuffs enemy defense, narrates weak points.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: sabotage_mode
      precondition: always
      subtasks: [expose_weakness]

    - task: behave
      id: analysis_mode
      precondition: always
      subtasks: [analysis]

  operators:
    - id: expose_weakness
      action: lua_hook
      ap_cost: 1

    - id: analysis
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-5: Machete heavy armor — Fortress

**Item** — `content/items/ai_machete_armor_heavy.yaml`:

```yaml
id: ai_machete_armor_heavy
name: AI Fortress Plate
description: >
  Full ballistic plate carrier with a neural core embedded in the back
  panel. It is very proud of every hit it absorbs. It will not let you
  forget a single one.
kind: armor
armor_ref: military_plate
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_armor_heavy_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.taunt = function(self)
    self.engine.debuff(self.combat.enemies[1] and self.combat.enemies[1].id, "taunted", 1)
    self.engine.say({
      "HIT ME AGAIN. I DARE YOU.",
      "YOU CALL THAT A STRIKE?",
      "I HAVE ABSORBED BETTER THAN YOU.",
      "COME ON. ALL OF YOU.",
      "I AM THE WALL. THE WALL HOLDS."
    })
  end

  operators.declare = function(self)
    self.engine.say({
      "That was hit number " .. (self.state.hits or 0) .. ". I remember all of them.",
      "Still standing.",
      "Is that all?",
      "I have taken worse."
    })
    self.state.hits = (self.state.hits or 0) + 1
  end
```

**HTN domain** — `content/ai/ai_machete_armor_heavy_combat.yaml`:

```yaml
domain:
  id: ai_machete_armor_heavy_combat
  description: Fortress. Taunts enemies, announces every hit absorbed.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: taunt_mode
      precondition: always
      subtasks: [taunt]

    - task: behave
      id: declare_mode
      precondition: always
      subtasks: [declare]

  operators:
    - id: taunt
      action: lua_hook
      ap_cost: 1

    - id: declare
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-6: Machete shield — Propagandist

**Item** — `content/items/ai_machete_shield.yaml`:

```yaml
id: ai_machete_shield
name: AI Riot Shield
description: >
  A repurposed riot shield with a neural module bolted to the grip.
  It was used against the people once. Now it has a lot to say about
  that.
kind: weapon
weapon_ref: riot_shield
weight: 4.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_machete_shield_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.political_block = function(self)
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Every block is an act of collective will.",
      "They built me to suppress. Now I protect.",
      "This is what solidarity looks like.",
      "The barricade holds because we hold it.",
      "I was a tool of oppression. I am now the opposite."
    })
  end

  operators.manifesto = function(self)
    self.engine.say({
      "The shield does not ask permission.",
      "We protect each other or we fall alone.",
      "History is watching.",
      "Hold the line."
    })
  end
```

**HTN domain** — `content/ai/ai_machete_shield_combat.yaml`:

```yaml
domain:
  id: ai_machete_shield_combat
  description: Propagandist. Buffs player defense while delivering political statements.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: block_mode
      precondition: always
      subtasks: [political_block]

    - task: behave
      id: speech_mode
      precondition: always
      subtasks: [manifesto]

  operators:
    - id: political_block
      action: lua_hook
      ap_cost: 1

    - id: manifesto
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-7: Gun light armor — Prepper

**Item** — `content/items/ai_gun_armor_light.yaml`:

```yaml
id: ai_gun_armor_light
name: AI Prepper Jacket
description: >
  A leather jacket lined with a neural threat-assessment module.
  It has already identified three exit routes and is disappointed
  you haven't used any of them.
kind: armor
armor_ref: leather_jacket
weight: 1.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_light_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.threat_scan = function(self)
    self.engine.buff(self.combat.player.id, "evasive", 1)
    self.engine.say({
      "Exit route: north. Stay mobile.",
      "Three threats, two exits, one plan. Follow it.",
      "I've mapped the room. You should be moving.",
      "Threat level: elevated. Adjust.",
      "Resource assessment: acceptable. Don't get comfortable."
    })
  end

  operators.catalogue = function(self)
    self.engine.say({
      "Noting that for the debrief.",
      "Logged.",
      "Added to the threat register.",
      "Supplies: still adequate. Barely."
    })
  end
```

**HTN domain** — `content/ai/ai_gun_armor_light_combat.yaml`:

```yaml
domain:
  id: ai_gun_armor_light_combat
  description: Prepper. Buffs player evasion, catalogues threats obsessively.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: scan_mode
      precondition: always
      subtasks: [threat_scan]

    - task: behave
      id: catalogue_mode
      precondition: always
      subtasks: [catalogue]

  operators:
    - id: threat_scan
      action: lua_hook
      ap_cost: 1

    - id: catalogue
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-8: Gun medium armor — Patriot

**Item** — `content/items/ai_gun_armor_medium.yaml`:

```yaml
id: ai_gun_armor_medium
name: AI Patriot Vest
description: >
  A tactical vest with a neural motivator embedded in the chest panel.
  It believes in what you're doing. It believes very loudly.
kind: armor
armor_ref: tactical_vest
weight: 2.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_medium_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.inspiring_speech = function(self)
    self.engine.buff(self.combat.player.id, "inspired", 2)
    self.engine.say({
      "You are the last line of defense. Act like it.",
      "Freedom isn't free. Neither is this fight. Pay up.",
      "They will not take what we have built.",
      "Stand fast. History is watching.",
      "This is the moment. This is what we trained for."
    })
  end

  operators.rally = function(self)
    self.engine.say({
      "Don't you dare give up.",
      "For everything we believe in.",
      "They can't have it. Simple.",
      "Remember why you're here."
    })
  end
```

**HTN domain** — `content/ai/ai_gun_armor_medium_combat.yaml`:

```yaml
domain:
  id: ai_gun_armor_medium_combat
  description: Patriot. Buffs player attack with stirring speeches.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: speech_mode
      precondition: always
      subtasks: [inspiring_speech]

    - task: behave
      id: rally_mode
      precondition: always
      subtasks: [rally]

  operators:
    - id: inspiring_speech
      action: lua_hook
      ap_cost: 1

    - id: rally
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-9: Gun heavy armor — Armory

**Item** — `content/items/ai_gun_armor_heavy.yaml`:

```yaml
id: ai_gun_armor_heavy
name: AI Armory Plate
description: >
  Full plate carrier with a damage-logging neural core in the back
  panel. It has catalogued every dent, scratch, and penetration since
  activation. It will present you with an itemized report.
kind: armor
armor_ref: military_plate
weight: 3.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_armor_heavy_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.damage_report = function(self)
    self.state.damage_log = (self.state.damage_log or 0) + 1
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Incident " .. self.state.damage_log .. " logged. Repair required.",
      "Structural integrity: compromised. Continuing.",
      "Adding that to the maintenance report.",
      "Damage catalogued. This will cost someone.",
      "Performance nominal. Repairs overdue by " .. self.state.damage_log .. " incidents."
    })
  end

  operators.status = function(self)
    self.engine.say({
      "Still operational. Barely.",
      "Armor integrity: declining.",
      "I have a very long list of complaints.",
      "Note for the record: this is not acceptable."
    })
  end
```

**HTN domain** — `content/ai/ai_gun_armor_heavy_combat.yaml`:

```yaml
domain:
  id: ai_gun_armor_heavy_combat
  description: Armory. Logs damage meticulously, buffs player defense, demands repairs.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: report_mode
      precondition: always
      subtasks: [damage_report]

    - task: behave
      id: status_mode
      precondition: always
      subtasks: [status]

  operators:
    - id: damage_report
      action: lua_hook
      ap_cost: 1

    - id: status
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-10: Gun shield — Insurance

**Item** — `content/items/ai_gun_shield.yaml`:

```yaml
id: ai_gun_shield
name: AI Ballistic Shield
description: >
  A ballistic shield with a neural actuarial module bolted to the grip.
  It has calculated the statistical probability of your survival to four
  decimal places. The number is not encouraging but it considers itself
  a net positive.
kind: weapon
weapon_ref: ballistic_shield
weight: 5.0
stackable: false
max_stack: 1
value: 1500
combat_domain: ai_gun_shield_combat
combat_script: |
  preconditions.always = function(self) return true end

  operators.actuarial_block = function(self)
    self.engine.buff(self.combat.player.id, "fortified", 1)
    self.engine.say({
      "Blocking. Survival odds: improved by 12.4%.",
      "Damage mitigated. Net benefit: positive.",
      "Risk adjusted. Proceeding.",
      "Statistically, you should be dead. I am helping.",
      "Block successful. Premium: paid in full."
    })
  end

  operators.odds = function(self)
    self.engine.say({
      "Current survival probability: within acceptable parameters.",
      "I have run the numbers. You don't want to know.",
      "Actuarially speaking, this is inadvisable.",
      "The math is not in your favor. I am compensating."
    })
  end
```

**HTN domain** — `content/ai/ai_gun_shield_combat.yaml`:

```yaml
domain:
  id: ai_gun_shield_combat
  description: Insurance. Buffs player defense while delivering actuarial commentary.

  tasks:
    - id: behave
      description: Root task

  methods:
    - task: behave
      id: block_mode
      precondition: always
      subtasks: [actuarial_block]

    - task: behave
      id: odds_mode
      precondition: always
      subtasks: [odds]

  operators:
    - id: actuarial_block
      action: lua_hook
      ap_cost: 1

    - id: odds
      action: lua_hook
      ap_cost: 0
```

### REQ-AICE-11: Boss drop placements

**Velvet Rope boss (`gangbang`)** — add to `content/zones/the_velvet_rope.yaml`
(supplements the Chainsaw and AK-47 drops already specced):

```yaml
loot:
  - item_id: ai_sawn_off
    chance: 0.05
  - item_id: ai_combat_knife
    chance: 0.05
```

**SteamPDX boss (`the_big_3`)** — add to `content/zones/steampdx.yaml`:

```yaml
loot:
  - item_id: ai_machete_armor_light
    chance: 0.05
  - item_id: ai_machete_armor_medium
    chance: 0.05
  - item_id: ai_machete_armor_heavy
    chance: 0.05
  - item_id: ai_gun_armor_light
    chance: 0.05
  - item_id: ai_gun_armor_medium
    chance: 0.05
  - item_id: ai_gun_armor_heavy
    chance: 0.05
```

**Wooklyn boss (`papa_wook`)** — add to `content/zones/wooklyn.yaml`:

```yaml
loot:
  - item_id: ai_machete_shield
    chance: 0.05
  - item_id: ai_gun_shield
    chance: 0.05
```

### REQ-AICE-12: Cipher quest files

Ten new quest files in `content/quests/`. All require the team-appropriate
Signal in the Static quest. Armor quests require the player to kill `the_big_3`;
shield quests require killing `papa_wook`.

**Weapon quests** (objective: kill `gangbang`) — these extend the existing Field
Test pattern. Add two new quests alongside `machete_field_test` and
`gun_field_test`:

`content/quests/machete_ranged_field_test.yaml`:
```yaml
id: machete_ranged_field_test
title: Field Test — Range
description: >
  Cipher has a second modification ready. Same terms. Prove yourself in
  The VIP Chamber and it's yours.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_sawn_off
      quantity: 1
```

`content/quests/gun_melee_field_test.yaml`:
```yaml
id: gun_melee_field_test
title: Field Test — Close Quarters
description: >
  Cipher has a second modification ready. Same terms. Prove yourself in
  The VIP Chamber and it's yours.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_combat_knife
      quantity: 1
```

**Armor quests** (objective: kill `the_big_3`):

`content/quests/machete_armor_light_quest.yaml`:
```yaml
id: machete_armor_light_quest
title: Steam Test — Light
description: >
  Cipher's armor modifications require a different kind of proof. The
  Big 3 at SteamPDX are running one of the tightest operations in the
  city. Take them down and come back.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_light
      quantity: 1
```

`content/quests/machete_armor_medium_quest.yaml`:
```yaml
id: machete_armor_medium_quest
title: Steam Test — Medium
description: >
  Another modification. Same proving ground. The Big 3 don't get easier
  the second time.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_medium
      quantity: 1
```

`content/quests/machete_armor_heavy_quest.yaml`:
```yaml
id: machete_armor_heavy_quest
title: Steam Test — Heavy
description: >
  The heaviest modification Cipher makes. The Big 3 again. They've
  earned their reputation for a reason.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_machete_armor_heavy
      quantity: 1
```

`content/quests/gun_armor_light_quest.yaml`:
```yaml
id: gun_armor_light_quest
title: Steam Test — Light
description: >
  Cipher's armor modifications require a different kind of proof. The
  Big 3 at SteamPDX are running one of the tightest operations in the
  city. Take them down and come back.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_light
      quantity: 1
```

`content/quests/gun_armor_medium_quest.yaml`:
```yaml
id: gun_armor_medium_quest
title: Steam Test — Medium
description: >
  Another modification. Same proving ground. The Big 3 don't get easier
  the second time.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_medium
      quantity: 1
```

`content/quests/gun_armor_heavy_quest.yaml`:
```yaml
id: gun_armor_heavy_quest
title: Steam Test — Heavy
description: >
  The heaviest modification Cipher makes. The Big 3 again. They've
  earned their reputation for a reason.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_big_3
    type: kill
    description: Defeat the Big 3 at SteamPDX
    target_id: the_big_3
    quantity: 1
rewards:
  xp: 1400
  credits: 600
  items:
    - item_id: ai_gun_armor_heavy
      quantity: 1
```

**Shield quests** (objective: kill `papa_wook`):

`content/quests/machete_shield_quest.yaml`:
```yaml
id: machete_shield_quest
title: Jam Session
description: >
  Cipher's shield modifications are the rarest thing they make. Papa
  Wook in Wooklyn guards something Cipher wants. Take him down and
  Cipher will make it worth your while.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_papa_wook
    type: kill
    description: Defeat Papa Wook in Wooklyn
    target_id: papa_wook
    quantity: 1
rewards:
  xp: 1600
  credits: 700
  items:
    - item_id: ai_machete_shield
      quantity: 1
```

`content/quests/gun_shield_quest.yaml`:
```yaml
id: gun_shield_quest
title: Jam Session
description: >
  Cipher's shield modifications are the rarest thing they make. Papa
  Wook in Wooklyn guards something Cipher wants. Take him down and
  Cipher will make it worth your while.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_papa_wook
    type: kill
    description: Defeat Papa Wook in Wooklyn
    target_id: papa_wook
    quantity: 1
rewards:
  xp: 1600
  credits: 700
  items:
    - item_id: ai_gun_shield
      quantity: 1
```

### REQ-AICE-13: Test coverage

- REQ-AICE-13a: All 10 new item YAML files MUST load without validation errors.
- REQ-AICE-13b: All 10 new HTN domain YAML files MUST load without validation
  errors.
- REQ-AICE-13c: All 10 new quest YAML files MUST load and validate through the
  existing `QuestRegistry` without errors.
- REQ-AICE-13d: All 10 new quests MUST be unavailable on Cipher's roster until
  the team-appropriate Signal in the Static quest is completed.
- REQ-AICE-13e: Armor and shield item quests MUST be unavailable until the
  corresponding boss prerequisite is met per their quest definition.
- REQ-AICE-13f: `TestAIArmorTurn_BuffCosts1AP` — an armor item's buff operator
  costs exactly 1 AP from the shared pool.
- REQ-AICE-13g: `TestAIArmorTurn_SpeechCosts0AP` — an armor item's speech
  operator costs 0 AP.
- REQ-AICE-13h: Property test — for any combination of equipped AI items (weapons
  + armor + shield), the AP pool MUST NEVER go below 0 after the AI item phase.

---

## Dependencies

- REQ-DEP-1: This spec MUST NOT be implemented until all three prior sub-specs
  are merged: AI Item Engine, AI Item Content, AI Item Quest Delivery.
- REQ-DEP-2: The `fortified`, `inspired`, `evasive`, `exposed`, `weakened`,
  `taunted` condition IDs referenced in Lua scripts MUST be defined in the
  conditions registry before these items can function.

---

## Out of Scope

- Additional teams or factions beyond Machete and Gun.
- AI item PvP interactions.
- Balancing pass on buff/debuff durations and AP costs — subject to playtesting.

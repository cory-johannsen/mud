# PF2E → Gunchete Import Reference

This document records all PF2E source spells/abilities that have been mapped to Gunchete
technologies, plus the canonical conversion rules. Consult this before importing new content
to avoid duplicated research.

---

## Conversion Rules

### Attribute Mapping
┌────────────────┬────────────────────┬──────────────┐
│ PF2E Attribute │ Gunchete Attribute │ Abbreviation │
├────────────────┼────────────────────┼──────────────┤
│ Strength       │ Brutality          │ BRT          │
├────────────────┼────────────────────┼──────────────┤
│ Constitution   │ Grit               │ GRT          │
├────────────────┼────────────────────┼──────────────┤
│ Dexterity      │ Quickness          │ QCK          │
├────────────────┼────────────────────┼──────────────┤
│ Intelligence   │ Reasoning          │ RSN          │
├────────────────┼────────────────────┼──────────────┤
│ Wisdom         │ Savvy              │ SVY          │
├────────────────┼────────────────────┼──────────────┤
│ Charisma       │ Flair              │ FLR          │
└────────────────┴────────────────────┴──────────────┘

## Class Mapping

Gunchete replaces PF2E classes with **Archetypes** (broad role categories) and **Jobs** (specific
specializations within an archetype). Each Archetype corresponds to one or more PF2E base classes;
each Job corresponds to a PF2E multiclass archetype (dedication) that best captures its flavor.

### Archetype → PF2E Class

| Gunchete Archetype | PF2E Base Class(es)                            | Key Ability (PF2E) | Gunchete Key Ability | HP/Level |
|--------------------|------------------------------------------------|--------------------|----------------------|----------|
| Aggressor          | Fighter, Barbarian                             | Str                | Brutality            | 10       |
| Criminal           | Rogue, Swashbuckler                            | Dex                | Quickness            | 8        |
| Drifter            | Ranger, Gunslinger                             | Str / Dex          | Grit                 | 10       |
| Influencer         | Bard, Sorcerer                                 | Cha                | Flair                | 8        |
| Naturalist         | Druid, Ranger                                  | Wis / Con          | Reasoning            | 8        |
| Nerd               | Wizard, Alchemist, Investigator, Inventor      | Int                | Reasoning            | 6        |
| Schemer            | Witch, Oracle, Psychic, Thaumaturge            | Wis / Cha          | Savvy                | 6        |
| Zealot             | Cleric, Champion, Monk                         | Wis / Str          | Grit                 | 8        |

### Job → PF2E Archetype (Multiclass Dedication)

#### Aggressor Jobs

| Gunchete Job      | PF2E Archetype | Rationale                                      |
|-------------------|---------------|------------------------------------------------|
| boot_machete      | Fighter        | Melee weapon specialist                        |
| boot_gun          | Gunslinger     | Firearm entry-level combat                     |
| goon              | Fighter        | Generic hired muscle, weapon proficiency focus |
| grunt             | Fighter        | Disciplined frontline combatant                |
| muscle            | Fighter        | Pure physical intimidation and melee           |
| thug              | Fighter        | Street-level weapon fighter                    |
| gangster          | Fighter        | Urban combat tactician                         |
| mercenary         | Fighter        | Professional soldier-for-hire                  |
| soldier           | Fighter        | Military training and tactics                  |
| street_fighter    | Barbarian      | Unrestrained brawler, rage-fueled              |
| beat_down_artist  | Barbarian      | Brutal beatdown focus, instinct barbarian      |
| roid_rager        | Barbarian      | Chem-enhanced rage, Giant Instinct barbarian   |

#### Criminal Jobs

| Gunchete Job     | PF2E Archetype | Rationale                                       |
|------------------|---------------|-------------------------------------------------|
| thief            | Rogue          | Core sneak/steal skill set                     |
| car_jacker       | Rogue          | Opportunistic theft, mobility focus            |
| smuggler         | Rogue          | Contraband and deception, Mastermind racket    |
| contract_killer  | Rogue          | Precision strikes, Assassin racket             |
| tomb_raider      | Rogue          | Trap navigation, exploration skills            |
| beggar           | Rogue          | Social manipulation, Scoundrel racket          |
| hanger_on        | Rogue          | Opportunistic follower, low-level rackets      |
| hooker           | Bard           | Social transaction, Polymath / Maestro muse    |
| gambler          | Swashbuckler   | Risk-taking panache, calculated daring         |

#### Drifter Jobs

| Gunchete Job | PF2E Archetype | Rationale                                          |
|--------------|---------------|----------------------------------------------------|
| scout        | Ranger         | Wilderness/urban recon, Hunt Prey                 |
| tracker      | Ranger         | Quarry pursuit, Flurry edge                       |
| stalker      | Ranger         | Stealthy predation, Precision edge                |
| warden       | Ranger         | Territory protection, Outwit edge                 |
| exterminator | Ranger         | Quarry eradication, vermin as prey               |
| cop          | Fighter        | Disciplined law enforcement, weapon proficiency   |
| driver       | Gunslinger     | Vehicle/firearm synergy, Drifter Way             |
| pilot        | Gunslinger     | Ranged precision, Drifter / Vanguard Way         |
| pirate       | Swashbuckler   | Daring nautical fighter, Fencing / Gymnast style |
| free_spirit  | Monk           | Unencumbered movement, Ki focus / Stunning Fist  |
| psychopath   | Barbarian      | Uncontrolled aggression, Fury Instinct            |
| bagman       | Rogue          | Courier/fixer, Thief racket                      |

#### Influencer Jobs

| Gunchete Job  | PF2E Archetype | Rationale                                           |
|---------------|---------------|-----------------------------------------------------|
| entertainer   | Bard           | Performance-based inspiration, Maestro muse        |
| exotic_dancer | Bard           | Captivation and distraction, Maestro muse          |
| schmoozer     | Bard           | Social skill mastery, Polymath muse                |
| politician    | Bard           | Mass persuasion, Polymath muse                     |
| karen         | Bard           | Entitled authority projection, Maestro muse        |
| libertarian   | Bard           | Ideological rhetoric, Polymath muse                |
| bureaucrat    | Bard           | Procedural control, Polymath muse                  |
| anarchist     | Bard           | Disruptive social influence, Warrior muse          |
| antifa        | Champion       | Cause-driven militant, Liberator cause             |
| extortionist  | Rogue          | Leverage and threat, Scoundrel racket              |

#### Naturalist Jobs

| Gunchete Job        | PF2E Archetype | Rationale                                      |
|---------------------|---------------|------------------------------------------------|
| hippie              | Druid          | Nature harmony, Wild order                    |
| freegan             | Druid          | Scavenging ecology, Wild / Leaf order         |
| hobo                | Druid          | Wandering naturalist, Wild order              |
| fallen_trustafarian | Druid          | Reluctant wilderness survivor, Leaf order     |
| rancher             | Druid          | Livestock and land stewardship, Animal order  |
| laborer             | Druid          | Land-working physicality, Stone order         |
| tracker             | Ranger         | Quarry pursuit (shared with Drifter)          |
| exterminator        | Ranger         | Vermin eradication (shared with Drifter)      |

#### Nerd Jobs

| Gunchete Job   | PF2E Archetype | Rationale                                          |
|----------------|---------------|----------------------------------------------------|
| engineer       | Inventor       | Device construction, Construct innovation         |
| grease_monkey  | Inventor       | Vehicle/gear repair, Weapon innovation            |
| maker          | Inventor       | General fabrication, Armor/Weapon innovation      |
| cooker         | Alchemist      | Substance synthesis, Bomber / Chirurgeon research |
| dealer         | Alchemist      | Elixir/compound distribution, Mutagenist research |
| narcomancer    | Alchemist      | Toxin/stimulant mastery, Toxicologist research    |
| detective      | Investigator   | Devise a Stratagem, Empiricism methodology        |
| journalist     | Investigator   | Information gathering, Interrogation methodology  |
| narc           | Investigator   | Undercover intelligence, Interrogation / Empiricism|
| hoarder        | Wizard         | Knowledge accumulation, Universalist school       |
| specialist     | Wizard         | Deep field expertise, specialist school           |
| natural_mystic | Witch          | Intuitive knowledge, Nature patron                |

#### Schemer Jobs

| Gunchete Job  | PF2E Archetype | Rationale                                           |
|---------------|---------------|-----------------------------------------------------|
| illusionist   | Witch          | Deception and illusion, Deception patron           |
| shit_stirrer  | Oracle         | Chaos-driven revelation, Tempest mystery           |
| mall_ninja    | Swashbuckler   | Overconfident style, Gymnast / Fencing style       |
| grifter       | Rogue          | Long con and deception, Mastermind racket          |
| salesman      | Bard           | Silver-tongued persuasion, Polymath muse           |

#### Zealot Jobs

| Gunchete Job    | PF2E Archetype | Rationale                                         |
|-----------------|---------------|---------------------------------------------------|
| pastor          | Cleric         | Spiritual guidance and healing, Healing doctrine  |
| street_preacher | Cleric         | Militant evangelism, Zeal doctrine               |
| medic           | Cleric         | Field healing and triage, Healing doctrine        |
| believer        | Cleric         | Devout follower, Cloistered cleric                |
| trainee         | Cleric         | Initiate rank, Cloistered cleric                  |
| follower        | Cleric         | Rank-and-file faithful, Warpriest doctrine        |
| cult_leader     | Oracle         | Apocalyptic prophecy, Ancestors / Cosmos mystery  |
| guard           | Champion       | Protective oath, Paladin / Liberator cause        |
| hired_help      | Champion       | Cause-for-pay, Desecrator / Paladin cause         |
| vigilante       | Champion       | Self-appointed justice, Liberator cause           |

### Save Type Mapping

| PF2E Save | Gunchete Save | Ability Score | Rationale |
|-----------|--------------|---------------|-----------|
| Fortitude | `toughness`  | Grit          | Physical resilience |
| Reflex    | `hustle`     | Quickness     | Speed / reaction time |
| Will      | `cool`       | Savvy         | Mental composure |

The existing `ResolveSave(saveType, combatant, dc, src)` in `internal/game/combat/resolver.go`
accepts `"toughness"`, `"hustle"`, and `"cool"` — use those strings exactly.

### Tradition Mapping

| PF2E Tradition | Gunchete Tradition  | Notes |
|---------------|---------------------|-------|
| Occult        | `neural`            | Mind/nervous system effects |
| Primal        | `bio_synthetic`     | Biological / organic tech |
| Arcane        | `technical`         | Mechanical / electronic tech |
| Divine        | `fanatic_doctrine`  | Zealot archetype analog; doctrine/faith-based effects |

### Effect Tier Names

| Context        | Tier Names |
|---------------|------------|
| Save-based     | `on_crit_success`, `on_success`, `on_failure`, `on_crit_failure` |
| Attack-based   | `on_miss`, `on_hit`, `on_crit_hit` |
| No-roll        | `on_apply` |

### Basic Save (PF2E)

PF2E "basic save" spells (damage halved on success) are represented in Gunchete by explicitly
listing reduced dice at each tier rather than a `half: true` flag:
- `on_success`: ~half dice (e.g. `1d3` for a `1d6` base)
- `on_failure`: full dice
- `on_crit_failure`: full dice + additional effect

### Attack Roll Techs

PF2E spells that use a spell attack roll (vs AC, not a saving throw) use `resolution: attack`.
The player's tech attack modifier = `CharLevel/2 + PrimaryAbilityMod` (same as skill checks).

---

## Mapped Technologies

### Neural Tradition

#### `mind_spike`
- **PF2E source:** Daze (Cantrip 1, remaster) — no spell named "Mind Spike" exists in PF2E
- **PF2E stats:** 2 actions, 60 ft, 1 target, Will basic save, 1d6 mental, stunned 1 on crit fail
- **Gunchete save:** `cool` (Will → Savvy)
- **Gunchete DC:** 15
- **Resolution:** save
- **Effects:**
  - on_success: 1d3 mental damage (half of 1d6)
  - on_failure: 1d6 mental damage
  - on_crit_failure: 1d6 mental + stunned 1

#### `neural_static`
- **PF2E source:** Slow (Spell 3) — adapted; PF2E uses Fortitude but lore fits Reflex/hustle
- **PF2E stats:** 2 actions, 30 ft, 1 target, Fortitude save, slowed 1 (success, 1 round) or slowed 1 (fail, 1 min) or slowed 2 (crit fail, 1 min)
- **Gunchete save:** `hustle` (adapted from Fortitude to Reflex — fits "reaction disruption" lore)
- **Gunchete DC:** 15
- **Resolution:** save
- **Effects:**
  - on_crit_success: no effect
  - on_success: no effect
  - on_failure: slowed 1 (rounds:1)
  - on_crit_failure: slowed 2 (rounds:1)
- **Amped (level 3):** duration doubles — rounds:2

#### `synaptic_surge`
- **PF2E source:** Fear (Spell 1) + Phantom Pain (Spell 1) hybrid
- **Fear stats:** 2 actions, 30 ft, 1 target, Will save, frightened 1/2/3+fleeing on crit fail
- **Phantom Pain stats:** 2 actions, 30 ft, 1 target, Will save, 2d4+1d4 persist. mental + sickened 1/2
- **Gunchete save:** `cool` (Will → Savvy)
- **Gunchete DC:** 15
- **Resolution:** save
- **Effects:**
  - on_crit_success: no effect
  - on_success: frightened 1
  - on_failure: 2d4 neural + frightened 2
  - on_crit_failure: 4d4 neural + frightened 3
- **Amped (level 3):** damage increases to 4d4 (fail) and 6d4 (crit fail)

---

### Innate Technologies (Region-Granted)

#### `blackout_pulse` (Old Town)
- **PF2E source:** Darkness (Spell 2)
- **PF2E stats:** 3 actions, 120 ft, 20-ft burst, no save, 1 minute, suppresses all light
- **Gunchete resolution:** none (area — all enemies in room)
- **Effects:**
  - on_apply: blinded condition (rounds:1) to all enemies in room

#### `arc_lights` (Northeast)
- **PF2E source:** Dancing Lights (Cantrip 1)
- **PF2E stats:** 2 actions, 120 ft, 4 floating lights, no save, sustained, torch-equivalent illumination
- **Gunchete resolution:** none (utility)
- **Effects:**
  - on_apply: utility message — illumination, no mechanical combat effect

#### `pressure_burst` (Pearl District)
- **PF2E source:** Hydraulic Push (Spell 1)
- **PF2E stats:** 2 actions, 60 ft, 1 target, ranged spell attack (vs AC), 3d6 bludgeoning + push 5 ft (hit), 6d6 + push 10 ft (crit)
- **Gunchete resolution:** attack
- **Effects:**
  - on_miss: no effect
  - on_hit: 3d6 bludgeoning + movement 5 ft away
  - on_crit_hit: 6d6 bludgeoning + movement 10 ft away

#### `nanite_infusion` (Southeast Portland)
- **PF2E source:** Heal (Spell 1, 2-action version)
- **PF2E stats:** 2 actions, 30 ft, 1 living creature, no save, restore 1d8+8 HP
- **Gunchete resolution:** none (heal self)
- **Effects:**
  - on_apply: heal 1d8 + 8 flat

#### `atmospheric_surge` (Pacific Northwest)
- **PF2E source:** Gust of Wind (Spell 1)
- **PF2E stats:** 2 actions, 60-ft line area, Fortitude save; fail = prone; crit fail = prone + push 30 ft + 2d6 bludgeoning
- **Gunchete save:** `toughness` (Fortitude → Grit)
- **Gunchete DC:** 15
- **Resolution:** save (area — all enemies in room)
- **Effects:**
  - on_crit_success: no effect
  - on_success: utility message (wind buffets but holds)
  - on_failure: prone
  - on_crit_failure: 2d6 bludgeoning + prone + movement 30 ft away

#### `viscous_spray` (South)
- **PF2E source:** Web (Spell 2) + Grease (Spell 1) hybrid
- **Web stats:** 3 actions, 30 ft, 10-ft burst, Reflex; fail = immobilized (1 round); success = -10 ft speed
- **Grease stats:** 2 actions, 30 ft, area, Reflex/Acrobatics; fail = prone
- **Gunchete save:** `hustle` (Reflex → Quickness)
- **Gunchete DC:** 15
- **Resolution:** save
- **Effects:**
  - on_crit_success: no effect
  - on_success: slowed 1 (rounds:1)  [−10 ft speed → lose 1 action]
  - on_failure: immobilized (rounds:1)
  - on_crit_failure: immobilized (rounds:2)

#### `chrome_reflex` (Southern California)
- **PF2E source:** Lucky Break (Focus Spell 4, Luck domain) / Fortune trait
- **PF2E stats:** Reaction, self, trigger = fail a saving throw; reroll and use better result
- **Gunchete resolution:** none (utility — Fortune mechanic deferred; requires Reactions system)
- **Effects:**
  - on_apply: utility message only — full reroll mechanic tracked in backlog

#### `seismic_sense` (Mountain)
- **PF2E source:** Tremorsense (creature ability, not a spell)
- **PF2E stats:** Passive imprecise sense; detects movement via ground vibrations through solid surfaces
- **Gunchete resolution:** none (passive — full passive mechanics tracked in backlog)
- **Effects:**
  - on_apply: utility message — detects all moving creatures in room through floor vibrations

#### `moisture_reclaim` (Midwest)
- **PF2E source:** Create Water (Spell 1)
- **PF2E stats:** 2 actions, self, no save, produces 2 gallons potable water (evaporates after 1 day)
- **Gunchete resolution:** none (utility)
- **Effects:**
  - on_apply: utility message — produces 2 gallons potable water

#### `terror_broadcast` (North Portland)
- **PF2E source:** Fear (Spell 1, area version via heightened 3rd)
- **PF2E stats:** 2 actions, 30 ft, up to 5 targets, Will save; success = frightened 1; fail = frightened 2; crit fail = frightened 3 + fleeing 1 round
- **Gunchete save:** `cool` (Will → Savvy)
- **Gunchete DC:** 15
- **Resolution:** save (area — all enemies in room)
- **Effects:**
  - on_crit_success: no effect
  - on_success: frightened 1
  - on_failure: frightened 2
  - on_crit_failure: frightened 2 + fleeing (rounds:1)

#### `acid_spit` (Gresham Outskirts)
- **PF2E source:** Acid Splash (Cantrip 1, legacy)
- **PF2E stats:** 2 actions, 30 ft, 1 target, ranged spell attack (vs AC), 1d6 acid (hit), persistent acid 1 (crit)
- **Gunchete resolution:** attack
- **Effects:**
  - on_miss: no effect
  - on_hit: 1d6 acid damage
  - on_crit_hit: 2d6 acid damage  [persistent acid deferred — no DoT system yet]

---

## New Condition Files Required

These conditions are referenced by the techs above and do not yet exist in `content/conditions/`:

| Condition ID | PF2E Source | Mechanical Effect |
|---|---|---|
| `slowed` | Slowed N | Lose N actions per turn (slowed N = N fewer AP) |
| `immobilized` | Immobilized | Cannot move; speed = 0 |
| `blinded` | Blinded | Cannot see; -4 to attack rolls; targets are concealed |
| `fleeing` | Fleeing | Must use all actions to move away from source of fear |

---

## Pending Imports

Technologies not yet imported (no effect definitions):

| Technology | Planned Sub-project | Notes |
|---|---|---|
| Any future archetype/job techs | To be determined | Reference this doc for save/tradition mapping |

---

## How to Import a New Spell

1. Look up the spell on Archives of Nethys (2e.aonprd.com) or via Foundry VTT MCP
2. Map the PF2E save type using the Conversion Rules table above
3. Map the tradition using the Tradition Mapping table above
4. Determine resolution type: save / attack / none
5. Define explicit per-tier effects (no "basic save" shortcut — list dice at each tier)
6. Add an entry to this document under the appropriate tradition section
7. Create the YAML file in `content/technologies/<tradition>/`
8. If new condition IDs are needed, add to the "New Condition Files Required" table and create them

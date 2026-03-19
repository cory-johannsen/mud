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

### Archetype → Technology Tradition

| Gunchete Archetype | Technology Tradition | PF2E Magic Tradition |
|--------------------|---------------------|----------------------|
| Influencer         | neural              | occult               |
| Schemer            | neural              | occult               |
| Naturalist         | bio_synthetic       | primal               |
| Drifter            | bio_synthetic       | primal               |
| Zealot             | fanatic_doctrine    | divine               |
| Nerd               | technical           | arcane               |
| Aggressor          | none                | —                    |
| Criminal           | none                | —                    |

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

## Batch Import 2026-03-18

Automated import of PF2E level-1 spells via `cmd/import-content -format pf2e`. Source: pf2e-data compendium (`packs/pf2e/spells/spells/rank-1/`). All files localized into Gunchete cyberpunk aesthetic.

**Summary:** 487 tech files across 4 traditions (neural: 131, bio_synthetic: 143, fanatic_doctrine: 93, technical: 120).

### neural (131 spells)

| tech_id | Gunchete Name | PF2E Source | Resolution |
|---------|---------------|-------------|------------|
| `agitate_neural` | Synaptic Overload | Agitate | save |
| `alarm_neural` | Neural Tripwire | Alarm | none |
| `animate_rope_neural` | Kinetic Filament | Animate Rope | none |
| `anticipate_peril_neural` | Threat Precognition | Anticipate Peril | none |
| `approximate_neural` | Rapid Scan | Approximate | none |
| `aqueous_blast_neural` | Hydro-Fist Strike | Aqueous Blast | attack |
| `bane_neural` | Confidence Drain | Bane | save |
| `befuddle_neural` | Cognitive Scramble | Befuddle | save |
| `beseech_the_sphinx_neural` | Algorithmic Oracle Query | Beseech The Sphinx | none |
| `biting_words` | Psychic Barb | Biting Words | attack |
| `bless_neural` | Combat Sync | Bless | none |
| `breadcrumbs_neural` | Neural Waypoint Trail | Breadcrumbs | none |
| `bullhorn_neural` | Broadcast Amplifier | Bullhorn | none |
| `carryall_neural` | Telekinetic Platform | Carryall | none |
| `celestial_accord_neural` | Conflict Suppression Protocol | Celestial Accord | save |
| `charm_neural` | Social Manipulation Wave | Charm | save |
| `command_neural` | Compliance Impulse | Command | save |
| `concordant_choir_neural` | Resonance Cascade | Concordant Choir | save |
| `curse_of_recoil_neural` | Targeting Jinx | Curse Of Recoil | save |
| `cycle_of_retribution_neural` | Feedback Loop Protocol | Cycle Of Retribution | save |
| `daze_neural` | Cranial Shock | Daze | save |
| `defended_by_spirits_neural` | Psionic Ward | Defended By Spirits | none |
| `detect_alignment_neural` | Intent Scan | Detect Alignment | none |
| `detect_magic_neural` | EM Field Scan | Detect Magic | none |
| `detect_metal_neural` | Magnetic Resonance Scan | Detect Metal | none |
| `disguise_magic_neural` | Signal Masking | Disguise Magic | none |
| `dizzying_colors_neural` | Chromatic Seizure Burst | Dizzying Colors | save |
| `dj_vu_neural` | Loop Lock | Dj Vu | save |
| `draw_ire_neural` | Provocation Spike | Draw Ire | save |
| `eat_fire_neural` | Thermal Absorption | Eat Fire | none |
| `echoing_weapon_neural` | Phantom Strike Echo | Echoing Weapon | none |
| `endure_neural` | Mental Fortitude Boost | Endure | none |
| `enfeeble_neural` | Muscle Override | Enfeeble | save |
| `equal_footing_neural` | Size Normalization Field | Equal Footing | save |
| `exchange_image_neural` | Identity Swap Illusion | Exchange Image | save |
| `fashionista_neural` | Holographic Wardrobe | Fashionista | none |
| `fated_healing_neural` | Biofeedback Prediction | Fated Healing | none |
| `fear_neural` | Terror Protocol | Fear | save |
| `figment_neural` | Sensory Projection | Figment | none |
| `flashy_disappearance_neural` | Smoke Screen Vanish | Flashy Disappearance | none |
| `forbidding_ward_neural` | Threat Interposition Field | Forbidding Ward | none |
| `force_barrage_neural` | Kinetic Bolt Volley | Force Barrage | none |
| `forced_mercy_neural` | Restraint Frequency | Forced Mercy | save |
| `friendfetch_neural` | Telekinetic Drag | Friendfetch | none |
| `ghost_sound_neural` | Audio Ghost | Ghost Sound | none |
| `glamorize_neural` | Minor Cosmetic Overlay | Glamorize | none |
| `glowing_trail_neural` | Bioluminescent Trace | Glowing Trail | none |
| `gravitational_pull_neural` | Gravity Spike | Gravitational Pull | save |
| `grim_tendrils_neural` | Void Tendrils | Grim Tendrils | save |
| `guidance_neural` | Tactical Insight | Guidance | none |
| `haunting_hymn_neural` | Subliminal Screech | Haunting Hymn | save |
| `helpful_steps_neural` | Kinetic Scaffold | Helpful Steps | none |
| `ill_omen` | Bad Luck Injection | Ill Omen | save |
| `illuminate_neural` | Light Source Activation | Illuminate | save |
| `illusory_disguise_neural` | Holographic Disguise | Illusory Disguise | none |
| `illusory_object_neural` | Persistent Hologram | Illusory Object | none |
| `imprint_message` | Psychic Impression | Imprint Message | none |
| `infectious_enthusiasm_neural` | Morale Contagion | Infectious Enthusiasm | save |
| `inkshot_neural` | Toxic Ink Strike | Inkshot | attack |
| `inside_ropes_neural` | Combat Data Feed | Inside Ropes | none |
| `invisible_item_neural` | Cloaking Field | Invisible Item | none |
| `invoke_true_name_neural` | Designation Protocol | Invoke True Name | none |
| `item_facade_neural` | Object Disguise Field | Item Facade | none |
| `join_pasts` | Memory Bridge | Join Pasts | none |
| `kinetic_ram_neural` | Kinetic Impact Wave | Kinetic Ram | save |
| `know_location_neural` | Location Anchor | Know Location | none |
| `know_the_way_neural` | Internal Navigation System | Know The Way | none |
| `liberating_command` | Break Free Signal | Liberating Command | none |
| `light_neural` | Photon Emitter | Light | none |
| `lock_neural` | Electromagnetic Lock | Lock | none |
| `lose_the_path_neural` | Disorientation Pulse | Lose The Path | save |
| `mending_neural` | Nanobot Repair Swarm | Mending | none |
| `message_neural` | Subvocal Transmission | Message | none |
| `message_rune_neural` | Stored Message Glyph | Message Rune | none |
| `mind_spike` | Mind Spike | Mind Spike | save |
| `mindlink_neural` | Neural Direct Link | Mindlink | none |
| `musical_accompaniment_neural` | Sonic Rhythm Interface | Musical Accompaniment | none |
| `mystic_armor_neural` | Force Shield | Mystic Armor | none |
| `needle_darts_neural` | Needle Volley | Needle Darts | attack |
| `neural_static` | Neural Static | Neural Static | save |
| `nudge_the_odds_neural` | Probability Tweak | Nudge The Odds | none |
| `object_reading` | Psychometric Scan | Object Reading | none |
| `overselling_flourish_neural` | Dramatic Overclock | Overselling Flourish | save |
| `penumbral_shroud_neural` | Shadow Shroud | Penumbral Shroud | save |
| `pet_cache_neural` | Subspace Pocket | Pet Cache | none |
| `phantasmal_minion_neural` | Phantom Construct | Phantasmal Minion | none |
| `phantom_pain` | Pain Simulation | Phantom Pain | save |
| `phase_bolt_neural` | Phase Strike | Phase Bolt | attack |
| `pocket_library_neural` | Memory Archive Access | Pocket Library | none |
| `prestidigitation_neural` | Minor Tech Cantrip | Prestidigitation | none |
| `protect_companion_neural` | Shared Barrier Protocol | Protect Companion | none |
| `protection_neural` | Hazard Resistance Shield | Protection | none |
| `quick_sort_neural` | Auto-Sort Protocol | Quick Sort | none |
| `rainbows_end_neural` | Prismatic Beam | Rainbows End | save |
| `read_aura_neural` | Signature Reading | Read Aura | none |
| `read_the_air_neural` | Social Dynamics Scan | Read The Air | none |
| `reed_whistle_neural` | Resonance Whistle | Reed Whistle | none |
| `restyle_neural` | Wardrobe Reconfiguration | Restyle | none |
| `runic_body_neural` | Combat Skin Augment | Runic Body | none |
| `runic_weapon_neural` | Weapon Enhancement Protocol | Runic Weapon | none |
| `sanctuary_neural` | Non-Aggression Broadcast | Sanctuary | save |
| `schadenfreude_neural` | Resilience Cascade | Schadenfreude | save |
| `scorching_blast_neural` | Thermal Fist Blast | Scorching Blast | attack |
| `seashell_of_stolen_sound_neural` | Sound Capture Trap | Seashell Of Stolen Sound | none |
| `share_lore_neural` | Knowledge Transfer | Share Lore | none |
| `shield_neural` | Neural Barrier | Shield | none |
| `sigil_neural` | Personal Signature Tag | Sigil | none |
| `signal_skyrocket_neural` | Flare Protocol | Signal Skyrocket | save |
| `sleep_neural` | Neural Shutdown | Sleep | save |
| `soothe` | Cognitive Stabilizer | Soothe | none |
| `spirit_link_neural` | Life Link | Spirit Link | none |
| `spirit_ward_neural` | Psionic Barrier | Spirit Ward | none |
| `summon_fey_neural` | Cognitive Construct Deploy | Summon Fey | none |
| `summon_instrument_neural` | Equipment Teleport | Summon Instrument | none |
| `summon_undead_neural` | Revenant Protocol | Summon Undead | none |
| `sure_strike_neural` | Targeting Lock | Sure Strike | none |
| `synaptic_surge` | Synaptic Surge | Synaptic Surge | save |
| `synchronize_neural` | Shared Frequency Marker | Synchronize | none |
| `synchronize_steps_neural` | Neural Movement Sync | Synchronize Steps | none |
| `tame_neural` | Behavioral Override | Tame | save |
| `telekinetic_hand_neural` | Telekinetic Manipulator | Telekinetic Hand | none |
| `telekinetic_projectile_neural` | Telekinetic Launch | Telekinetic Projectile | attack |
| `thicket_of_knives_neural` | Phantom Blade Array | Thicket Of Knives | none |
| `thoughtful_gift_neural` | Object Transfer | Thoughtful Gift | none |
| `time_sense_neural` | Internal Chronometer | Time Sense | none |
| `tremor_signs_neural` | Ground Wave Signal | Tremor Signs | none |
| `unbroken_panoply_neural` | Emergency Repair Override | Unbroken Panoply | none |
| `ventriloquism_neural` | Voice Rerouter | Ventriloquism | none |
| `void_warp_neural` | Void Phase Shift | Void Warp | save |
| `warp_step_neural` | Micro-Teleport | Warp Step | none |
| `wash_your_luck_neural` | Fortune Reset Protocol | Wash Your Luck | none |

### bio_synthetic (143 spells)

| tech_id | Gunchete Name | PF2E Source | Resolution |
|---------|---------------|-------------|------------|
| `500_toads_bio_synthetic` | Spore Flood | 500 Toads | none |
| `acid_splash_bio_synthetic` | Bio-Acid Spit | Acid Splash | attack |
| `acid_spray` | Acid Spray | Acid Spray | save |
| `acidic_burst_bio_synthetic` | Corrosive Burst | Acidic Burst | save |
| `air_bubble_bio_synthetic` | Oxy-Filter | Air Bubble | none |
| `airburst_bio_synthetic` | Pressure Wave | Airburst | save |
| `alarm_bio_synthetic` | Chemical Perimeter Sensor | Alarm | none |
| `animal_allies` | Bioengineered Scout Pack | Animal Allies | save |
| `ant_haul_bio_synthetic` | Musculoskeletal Boost Injection | Ant Haul | none |
| `approximate_bio_synthetic` | Chemical Census Sweep | Approximate | none |
| `aqueous_blast_bio_synthetic` | Pressurized Fluid Fist | Aqueous Blast | attack |
| `armor_of_thorn_and_claw` | Dermal Thorn Eruption | Armor Of Thorn And Claw | none |
| `bramble_bush_bio_synthetic` | Spike Cluster Burst | Bramble Bush | save |
| `breadcrumbs_bio_synthetic` | Pheromone Trail Marker | Breadcrumbs | none |
| `breathe_fire_bio_synthetic` | Flammable Gland Spit | Breathe Fire | save |
| `briny_bolt_bio_synthetic` | Saline Injection Bolt | Briny Bolt | attack |
| `buffeting_winds` | Wind Burst Exhale | Buffeting Winds | save |
| `buoyant_bubbles_bio_synthetic` | Hydrophobic Foam Coat | Buoyant Bubbles | save |
| `camel_spit_bio_synthetic` | Modified Salivary Weapon | Camel Spit | attack |
| `caustic_blast_bio_synthetic` | Acid Cluster Spray | Caustic Blast | save |
| `charm_bio_synthetic` | Pheromone Influence Cloud | Charm | save |
| `chilling_spray_bio_synthetic` | Cryogenic Gland Spray | Chilling Spray | save |
| `cleanse_cuisine_bio_synthetic` | Contamination Neutralizer | Cleanse Cuisine | none |
| `conductive_weapon_bio_synthetic` | Electroconductive Weapon Coating | Conductive Weapon | none |
| `create_water_bio_synthetic` | Atmospheric Water Extraction | Create Water | none |
| `deep_breath_bio_synthetic` | Extended Oxygen Reserve | Deep Breath | none |
| `dehydrate_bio_synthetic` | Desiccation Enzyme Release | Dehydrate | save |
| `detect_magic_bio_synthetic` | Bio-Energy Sensor Sweep | Detect Magic | none |
| `detect_metal_bio_synthetic` | Metallic Compound Sensor | Detect Metal | none |
| `detect_poison_bio_synthetic` | Toxin Detection Organ | Detect Poison | none |
| `draw_moisture_bio_synthetic` | Moisture Siphon | Draw Moisture | none |
| `eat_fire_bio_synthetic` | Thermal Conversion Organ | Eat Fire | none |
| `electric_arc_bio_synthetic` | Bio-Electric Discharge | Electric Arc | save |
| `elemental_counter_bio_synthetic` | Elemental Resistance Compound | Elemental Counter | none |
| `elysian_whimsy_bio_synthetic` | Neurochemical Confusion Agent | Elysian Whimsy | save |
| `equal_footing_bio_synthetic` | Size-Suppression Field | Equal Footing | save |
| `fear_bio_synthetic` | Synthetic Cortisol Spike | Fear | save |
| `fleet_step_bio_synthetic` | Adrenaline Sprint Injection | Fleet Step | none |
| `flourishing_flora_bio_synthetic` | Rapid Growth Accelerant Deploy | Flourishing Flora | save |
| `foraging_friends` | Trained Scout Animals Deploy | Foraging Friends | none |
| `forge_bio_synthetic` | Bio-Fabrication Unit | Forge | save |
| `frostbite_bio_synthetic` | Cryogenic Encasement Spray | Frostbite | save |
| `funeral_flames_bio_synthetic` | Incendiary Weapon Coat | Funeral Flames | none |
| `gale_blast_bio_synthetic` | Compressed Air Exhale | Gale Blast | save |
| `gentle_landing_bio_synthetic` | Impact Cushion Foam Deploy | Gentle Landing | none |
| `glamorize_bio_synthetic` | Cosmetic Compound Application | Glamorize | none |
| `glass_shield_bio_synthetic` | Transparent Polymer Barrier | Glass Shield | save |
| `glowing_trail_bio_synthetic` | Bioluminescent Track | Glowing Trail | none |
| `goblin_pox_bio_synthetic` | Synthetic Pox Agent | Goblin Pox | save |
| `gouging_claw_bio_synthetic` | Extrude Blade Claw | Gouging Claw | attack |
| `grease_bio_synthetic` | Industrial Lubricant Spray | Grease | save |
| `gritty_wheeze_bio_synthetic` | Desiccant Particle Exhale | Gritty Wheeze | save |
| `guidance_bio_synthetic` | Bioscan Assist Signal | Guidance | none |
| `gust_of_wind_bio_synthetic` | Lung-Powered Wind Blast | Gust Of Wind | save |
| `heal_bio_synthetic` | Bio-Regeneration Pulse | Heal | save |
| `healing_plaster` | Bio-Patch Compound | Healing Plaster | none |
| `helpful_steps_bio_synthetic` | Emergent Vine Scaffold | Helpful Steps | none |
| `hippocampus_retreat_bio_synthetic` | Aquatic Withdrawal Protocol | Hippocampus Retreat | attack |
| `horizon_thunder_sphere_bio_synthetic` | Bio-Electric Thunder Ball | Horizon Thunder Sphere | save |
| `hydraulic_push_bio_synthetic` | High-Pressure Fluid Blast | Hydraulic Push | attack |
| `ignition_bio_synthetic` | Contact Igniter Compound | Ignition | attack |
| `illuminate_bio_synthetic` | Remote Bioluminescence Activation | Illuminate | save |
| `inkshot_bio_synthetic` | Ink Sac Spray | Inkshot | attack |
| `inside_ropes_bio_synthetic` | Combat Analysis Gland | Inside Ropes | none |
| `instant_pottery_bio_synthetic` | Rapid Organic Fabrication | Instant Pottery | none |
| `interposing_earth_bio_synthetic` | Terrain Shield Reaction | Interposing Earth | save |
| `invoke_true_name_bio_synthetic` | Biological Signature Lock | Invoke True Name | none |
| `jump_bio_synthetic` | Leg Enhancement Burst | Jump | none |
| `juvenile_companion` | Companion Revert Protocol | Juvenile Companion | none |
| `know_location_bio_synthetic` | Pheromone Anchor Point | Know Location | none |
| `know_the_way_bio_synthetic` | Biological Compass Calibration | Know The Way | none |
| `leaden_steps_bio_synthetic` | Adhesion Enzyme Injection | Leaden Steps | save |
| `light_bio_synthetic` | Bioluminescent Orb | Light | none |
| `live_wire_bio_synthetic` | Electro-Filament Deploy | Live Wire | attack |
| `lose_the_path_bio_synthetic` | Disorientation Pheromone Burst | Lose The Path | save |
| `magic_stone_bio_synthetic` | Bio-Charged Projectile | Magic Stone | none |
| `mending_bio_synthetic` | Rapid Bio-Repair | Mending | none |
| `mud_pit_bio_synthetic` | Mud Pit | Mud Pit | none |
| `mystic_armor_bio_synthetic` | Bio-Synthetic Skin Hardening | Mystic Armor | none |
| `needle_darts_bio_synthetic` | Spine Launcher | Needle Darts | attack |
| `negate_aroma_bio_synthetic` | Negate Aroma | Negate Aroma | none |
| `nettleskin` | Nettleskin | Nettleskin | none |
| `noxious_vapors_bio_synthetic` | Toxic Secretion Cloud | Noxious Vapors | save |
| `personal_rain_cloud_bio_synthetic` | Moisture Generation Cloud | Personal Rain Cloud | save |
| `pest_form_bio_synthetic` | Micro-Organism Form | Pest Form | none |
| `pet_cache_bio_synthetic` | Biological Pocket Organ | Pet Cache | none |
| `prestidigitation_bio_synthetic` | Minor Bio-Utility | Prestidigitation | none |
| `protect_companion_bio_synthetic` | Shared Bio-Field Extension | Protect Companion | none |
| `protector_tree` | Guardian Organism Deploy | Protector Tree | none |
| `puff_of_poison_bio_synthetic` | Puff of Poison | Puff Of Poison | save |
| `pummeling_rubble_bio_synthetic` | Pummeling Rubble | Pummeling Rubble | save |
| `purifying_icicle_bio_synthetic` | Cryo-Purification Spike | Purifying Icicle | attack |
| `putrefy_food_and_drink_bio_synthetic` | Rapid Decomposition Agent | Putrefy Food And Drink | none |
| `quick_sort_bio_synthetic` | Bio-Manipulator Sort | Quick Sort | none |
| `rainbows_end_bio_synthetic` | Prismatic Biochemical Beam | Rainbows End | save |
| `ray_of_frost_bio_synthetic` | Cryo-Beam Secretion | Ray Of Frost | attack |
| `read_aura_bio_synthetic` | Bio-Signature Scanner | Read Aura | none |
| `reed_whistle_bio_synthetic` | Bio-Resonance Signal Device | Reed Whistle | none |
| `restyle_bio_synthetic` | Textile Micro-Modification | Restyle | none |
| `root_reading_bio_synthetic` | Organic Memory Scan | Root Reading | none |
| `rousing_splash_bio_synthetic` | Stimulant Splash Compound | Rousing Splash | none |
| `runic_body_bio_synthetic` | Combat Augmentation Activation | Runic Body | none |
| `runic_weapon_bio_synthetic` | Weapon Bio-Enhancement Coat | Runic Weapon | none |
| `sacred_beasts_bio_synthetic` | Sacred Beasts | Sacred Beasts | save |
| `scatter_scree_bio_synthetic` | Debris Launch | Scatter Scree | save |
| `scorching_blast_bio_synthetic` | Thermal Compound Fist | Scorching Blast | attack |
| `scouring_sand_bio_synthetic` | Abrasive Particle Blast | Scouring Sand | save |
| `seashell_of_stolen_sound_bio_synthetic` | Sound Capture Membrane | Seashell Of Stolen Sound | none |
| `shattering_gem_bio_synthetic` | Shattering Gem | Shattering Gem | save |
| `shielded_arm_bio_synthetic` | Exoskeletal Shield Extension | Shielded Arm | none |
| `shillelagh` | Bio-Enhanced Club Strike | Shillelagh | none |
| `shocking_grasp_bio_synthetic` | Bio-Electric Shock Touch | Shocking Grasp | attack |
| `shockwave_bio_synthetic` | Shockwave | Shockwave | save |
| `sigil_bio_synthetic` | Bio-Chemical Identifier Tag | Sigil | none |
| `signal_skyrocket_bio_synthetic` | Bioluminescent Flare Launch | Signal Skyrocket | save |
| `slashing_gust_bio_synthetic` | Blade Wind Exhale | Slashing Gust | attack |
| `snowball_bio_synthetic` | Cryo-Gel Projectile | Snowball | attack |
| `spider_sting_bio_synthetic` | Paralytic Toxin Strike | Spider Sting | save |
| `spout_bio_synthetic` | High-Pressure Bio-Fluid Jet | Spout | save |
| `stabilize_bio_synthetic` | Stabilize | Stabilize | none |
| `summon_animal_bio_synthetic` | Summon Animal | Summon Animal | none |
| `summon_fey_bio_synthetic` | Summon Fey | Summon Fey | none |
| `summon_plant_or_fungus` | Summon Plant or Fungus | Summon Plant Or Fungus | none |
| `swampcall` | Bog Environment Call | Swampcall | save |
| `synchronize_bio_synthetic` | Pheromone Sync Beacon | Synchronize | none |
| `tailwind_bio_synthetic` | Metabolic Speed Boost | Tailwind | none |
| `take_root_bio_synthetic` | Root Anchor System | Take Root | none |
| `tame_bio_synthetic` | Biochemical Calming Agent | Tame | save |
| `tangle_vine_bio_synthetic` | Entanglement Vine Deploy | Tangle Vine | attack |
| `tether_bio_synthetic` | Bio-Filament Tether | Tether | save |
| `threefold_limb_bio_synthetic` | Triple Limb Strike Module | Threefold Limb | attack |
| `thunderstrike_bio_synthetic` | Bio-Electric Thunder Discharge | Thunderstrike | save |
| `timber_bio_synthetic` | Timber | Timber | save |
| `tremor_signs_bio_synthetic` | Ground Vibration Signal | Tremor Signs | none |
| `vanishing_tracks` | Track Erasure Compound | Vanishing Tracks | none |
| `ventriloquism_bio_synthetic` | Voice Projection Organ | Ventriloquism | none |
| `verdant_sprout` | Verdant Sprout | Verdant Sprout | none |
| `verminous_lure` | Verminous Lure | Verminous Lure | save |
| `vitality_lash_bio_synthetic` | Vitality Lash | Vitality Lash | save |
| `wall_of_shrubs_bio_synthetic` | Wall of Shrubs | Wall Of Shrubs | none |
| `weaken_earth_bio_synthetic` | Ground Destabilizer Compound | Weaken Earth | save |
| `weave_wood_bio_synthetic` | Plant Matter Shaper | Weave Wood | none |
| `wooden_fists_bio_synthetic` | Cellulose Reinforcement Inject | Wooden Fists | none |

### fanatic_doctrine (93 spells)

| tech_id | Gunchete Name | PF2E Source | Resolution |
|---------|---------------|-------------|------------|
| `admonishing_ray_fanatic_doctrine` | Corrective Beam | Admonishing Ray | attack |
| `air_bubble_fanatic_doctrine` | Sacred Breath | Air Bubble | none |
| `alarm_fanatic_doctrine` | Vigilance Ward | Alarm | none |
| `ancient_dust_fanatic_doctrine` | Doctrine's Reckoning | Ancient Dust | save |
| `approximate_fanatic_doctrine` | Doctrine's Census | Approximate | none |
| `bane_fanatic_doctrine` | Doubt Curse | Bane | save |
| `battle_fervor` | Battle Fervor | Battle Fervor | none |
| `benediction` | Doctrine's Benediction | Benediction | none |
| `beseech_the_sphinx_fanatic_doctrine` | Seek the Doctrine's Wisdom | Beseech The Sphinx | none |
| `bless_fanatic_doctrine` | Strike True for the Doctrine | Bless | none |
| `breadcrumbs_fanatic_doctrine` | Faith Trail Marking | Breadcrumbs | none |
| `bullhorn_fanatic_doctrine` | Doctrine Proclamation Amplifier | Bullhorn | none |
| `celestial_accord_fanatic_doctrine` | Forced Reconciliation Mandate | Celestial Accord | save |
| `cleanse_cuisine_fanatic_doctrine` | Sacramental Purification | Cleanse Cuisine | none |
| `command_fanatic_doctrine` | Doctrine Command | Command | save |
| `concordant_choir_fanatic_doctrine` | Zealous Resonance Wave | Concordant Choir | save |
| `create_water_fanatic_doctrine` | Doctrine's Provision | Create Water | none |
| `curse_of_recoil_fanatic_doctrine` | Doctrine's Retribution Curse | Curse Of Recoil | save |
| `cycle_of_retribution_fanatic_doctrine` | Retribution Protocol | Cycle Of Retribution | save |
| `daze_fanatic_doctrine` | Doctrine's Rebuke | Daze | save |
| `defended_by_spirits_fanatic_doctrine` | Martyr Guard | Defended By Spirits | none |
| `detect_alignment_fanatic_doctrine` | Heretic Detection Sweep | Detect Alignment | none |
| `detect_magic_fanatic_doctrine` | Forbidden Power Scan | Detect Magic | none |
| `detect_metal_fanatic_doctrine` | Contraband Metal Detection | Detect Metal | none |
| `detect_poison_fanatic_doctrine` | Toxin Purity Rite | Detect Poison | none |
| `divine_lance` | Doctrine's Judgment Bolt | Divine Lance | attack |
| `draw_moisture_fanatic_doctrine` | Doctrine's Desiccation | Draw Moisture | none |
| `echoing_weapon_fanatic_doctrine` | Doctrine's Echo Strike | Echoing Weapon | none |
| `elysian_whimsy_fanatic_doctrine` | Doctrine's Madness | Elysian Whimsy | save |
| `enfeeble_fanatic_doctrine` | Weakness Curse | Enfeeble | save |
| `fated_healing_fanatic_doctrine` | Foreseen Recovery | Fated Healing | none |
| `fear_fanatic_doctrine` | The Doctrine's Wrath | Fear | save |
| `flense_fanatic_doctrine` | Penitent's Stripping | Flense | attack |
| `forbidding_ward_fanatic_doctrine` | Sanctuary Barrier | Forbidding Ward | none |
| `forced_mercy_fanatic_doctrine` | Doctrine's Mercy Mandate | Forced Mercy | save |
| `funeral_flames_fanatic_doctrine` | Martyr's Torch | Funeral Flames | none |
| `glamorize_fanatic_doctrine` | Doctrine's Cosmetic Blessing | Glamorize | none |
| `glowing_trail_fanatic_doctrine` | Doctrine's Luminous Path | Glowing Trail | none |
| `guidance_fanatic_doctrine` | Doctrine's Guidance | Guidance | none |
| `harm` | Doctrine's Harm | Harm | save |
| `haunting_hymn_fanatic_doctrine` | Zealous War Hymn | Haunting Hymn | save |
| `heal_fanatic_doctrine` | Doctrine's Healing Grace | Heal | save |
| `helpful_steps_fanatic_doctrine` | Doctrine's Scaffold | Helpful Steps | none |
| `illuminate_fanatic_doctrine` | Doctrine's Illumination | Illuminate | save |
| `infuse_vitality` | Vital Doctrine Infusion | Infuse Vitality | none |
| `inside_ropes_fanatic_doctrine` | Doctrine's Combat Revelation | Inside Ropes | none |
| `invoke_true_name_fanatic_doctrine` | True Designation | Invoke True Name | none |
| `know_location_fanatic_doctrine` | Doctrine's Waypoint | Know Location | none |
| `know_the_way_fanatic_doctrine` | Doctrine's Direction | Know The Way | none |
| `light_fanatic_doctrine` | Doctrine's Light | Light | none |
| `lock_fanatic_doctrine` | Doctrine's Seal | Lock | none |
| `magic_stone_fanatic_doctrine` | Doctrine's Stone | Magic Stone | none |
| `malediction` | Doctrine's Curse | Malediction | save |
| `mending_fanatic_doctrine` | Doctrine's Repair Rite | Mending | none |
| `message_fanatic_doctrine` | Doctrine's Word | Message | none |
| `mystic_armor_fanatic_doctrine` | Doctrine's Armor of Faith | Mystic Armor | none |
| `necromancers_generosity_fanatic_doctrine` | Doctrine's Penance Transfer | Necromancers Generosity | none |
| `needle_darts_fanatic_doctrine` | Doctrine's Judgment Darts | Needle Darts | attack |
| `nudge_the_odds_fanatic_doctrine` | Doctrine's Fortune | Nudge The Odds | none |
| `pet_cache_fanatic_doctrine` | Doctrine's Pocket | Pet Cache | none |
| `prestidigitation_fanatic_doctrine` | Minor Doctrine Cantrip | Prestidigitation | none |
| `protect_companion_fanatic_doctrine` | Shared Doctrine Shield | Protect Companion | none |
| `protection_fanatic_doctrine` | Doctrine's Ward | Protection | none |
| `purifying_icicle_fanatic_doctrine` | Doctrine's Purity Spike | Purifying Icicle | attack |
| `putrefy_food_and_drink_fanatic_doctrine` | Doctrine's Contamination Curse | Putrefy Food And Drink | none |
| `quick_sort_fanatic_doctrine` | Doctrine's Order | Quick Sort | none |
| `rainbows_end_fanatic_doctrine` | Prismatic Doctrine Beam | Rainbows End | save |
| `read_aura_fanatic_doctrine` | Doctrine's Aura Reading | Read Aura | none |
| `read_the_air_fanatic_doctrine` | Social Doctrine Analysis | Read The Air | none |
| `restyle_fanatic_doctrine` | Doctrine's Wardrobe Blessing | Restyle | none |
| `rousing_splash_fanatic_doctrine` | Revival Blessing | Rousing Splash | none |
| `runic_body_fanatic_doctrine` | Doctrine's Combat Rite | Runic Body | none |
| `runic_weapon_fanatic_doctrine` | Doctrine's Weapon Blessing | Runic Weapon | none |
| `sacred_beasts_fanatic_doctrine` | Doctrine's Sacred Creature Summon | Sacred Beasts | save |
| `sanctuary_fanatic_doctrine` | Doctrine's Safe Passage | Sanctuary | save |
| `schadenfreude_fanatic_doctrine` | Doctrine's Trial Response | Schadenfreude | save |
| `shield_fanatic_doctrine` | Doctrine's Shield of Faith | Shield | none |
| `shielded_arm_fanatic_doctrine` | Doctrine's Armguard | Shielded Arm | none |
| `sigil_fanatic_doctrine` | Doctrine's Mark | Sigil | none |
| `spirit_link_fanatic_doctrine` | Martyr Link | Spirit Link | none |
| `spirit_ward_fanatic_doctrine` | Doctrine's Protective Ward | Spirit Ward | none |
| `stabilize_fanatic_doctrine` | Emergency Doctrine Stabilization | Stabilize | none |
| `summon_instrument_fanatic_doctrine` | Doctrine's Instrument Summon | Summon Instrument | none |
| `summon_lesser_servitor` | Doctrine's Servitor Summon | Summon Lesser Servitor | none |
| `summon_undead_fanatic_doctrine` | Doctrine's Resurrection Mandate | Summon Undead | none |
| `synchronize_fanatic_doctrine` | Doctrine's Sync Mark | Synchronize | none |
| `thoughtful_gift_fanatic_doctrine` | Doctrine's Delivery | Thoughtful Gift | none |
| `torturous_trauma_fanatic_doctrine` | Doctrine's Punishment | Torturous Trauma | save |
| `tremor_signs_fanatic_doctrine` | Doctrine Ground Signal | Tremor Signs | none |
| `ventriloquism_fanatic_doctrine` | Voice of the Doctrine | Ventriloquism | none |
| `vitality_lash_fanatic_doctrine` | Doctrine's Vitality Lash | Vitality Lash | save |
| `void_warp_fanatic_doctrine` | Doctrine's Void Judgment | Void Warp | save |
| `wash_your_luck_fanatic_doctrine` | Doctrine's Luck Cleansing | Wash Your Luck | none |

### technical (120 spells)

| tech_id | Gunchete Name | PF2E Source | Resolution |
|---------|---------------|-------------|------------|
| `500_toads_technical` | Swarm Unit Deploy | 500 Toads | none |
| `acidic_burst_technical` | Corrosive Payload | Acidic Burst | save |
| `admonishing_ray_technical` | Compliance Beam | Admonishing Ray | attack |
| `agitate_technical` | Feedback Overload | Agitate | save |
| `air_bubble_technical` | Emergency Atmos Pack | Air Bubble | none |
| `airburst_technical` | Concussive Drone Strike | Airburst | save |
| `alarm_technical` | Perimeter Sensor Grid | Alarm | none |
| `animate_rope_technical` | Servo-Cable Actuator | Animate Rope | none |
| `ant_haul_technical` | Load-Bearing Exo-Frame | Ant Haul | none |
| `anticipate_peril_technical` | Threat Prediction Algorithm | Anticipate Peril | none |
| `aqueous_blast_technical` | Hydraulic Impact Round | Aqueous Blast | attack |
| `befuddle_technical` | Cognitive Jam Signal | Befuddle | save |
| `breadcrumbs_technical` | Route Logging Protocol | Breadcrumbs | none |
| `breathe_fire_technical` | Incendiary Spray Unit | Breathe Fire | save |
| `briny_bolt_technical` | Saline Projectile Round | Briny Bolt | attack |
| `buoyant_bubbles_technical` | Hydrophobic Foam Coat | Buoyant Bubbles | save |
| `camel_spit_technical` | Repellent Spray Launcher | Camel Spit | attack |
| `carryall_technical` | Telekinetic Load Platform | Carryall | none |
| `charm_technical` | Social Override Protocol | Charm | save |
| `chilling_spray_technical` | Cryogenic Aerosol Burst | Chilling Spray | save |
| `command_technical` | Authority Compliance Chip | Command | save |
| `conductive_weapon_technical` | Electroconductive Coating | Conductive Weapon | none |
| `create_water_technical` | Atmospheric Condenser | Create Water | none |
| `dehydrate_technical` | Desiccant Pulse Emitter | Dehydrate | save |
| `disguise_magic_technical` | Signal Masking Layer | Disguise Magic | none |
| `dizzying_colors_technical` | Strobing Optical Disruptor | Dizzying Colors | save |
| `dj_vu_technical` | Memory Loop Injection | Dj Vu | save |
| `draw_ire_technical` | Threat Beacon Broadcast | Draw Ire | save |
| `echoing_weapon_technical` | Kinetic Echo Amplifier | Echoing Weapon | none |
| `elysian_whimsy_technical` | Behavioral Randomizer | Elysian Whimsy | save |
| `endure_technical` | Metabolic Stabilizer Injection | Endure | none |
| `enfeeble_technical` | Muscle Inhibitor Field | Enfeeble | save |
| `equal_footing_technical` | Size Normalization Harness | Equal Footing | save |
| `exchange_image_technical` | Holographic Identity Swap | Exchange Image | save |
| `fashionista_technical` | Smart Wardrobe Interface | Fashionista | none |
| `fear_technical` | Threat Assessment Override | Fear | save |
| `flashy_disappearance_technical` | Smoke-Bang Exit Package | Flashy Disappearance | none |
| `fleet_step_technical` | Mobility Boost Actuator | Fleet Step | none |
| `flense_technical` | Ablative Strip Charge | Flense | attack |
| `flourishing_flora_technical` | Rapid Growth Accelerant | Flourishing Flora | save |
| `fold_metal` | Precision Metal Former | Fold Metal | none |
| `force_barrage_technical` | Multi-Round Kinetic Volley | Force Barrage | none |
| `forge_technical` | Rapid Fabrication Unit | Forge | save |
| `friendfetch_technical` | Magnetic Tether Reel | Friendfetch | none |
| `gentle_landing_technical` | Impact Absorption Foam | Gentle Landing | none |
| `goblin_pox_technical` | Synthetic Pathogen Dispersal | Goblin Pox | save |
| `gravitational_pull_technical` | Grav-Spike Projector | Gravitational Pull | save |
| `grease_technical` | Lubricant Spray System | Grease | save |
| `grim_tendrils_technical` | Void Tendril Projector | Grim Tendrils | save |
| `gritty_wheeze_technical` | Particle Spray Grenade | Gritty Wheeze | save |
| `gust_of_wind_technical` | High-Pressure Air Cannon | Gust Of Wind | save |
| `helpful_steps_technical` | Deployable Step Scaffold | Helpful Steps | none |
| `hippocampus_retreat_technical` | Aquatic Escape Thruster | Hippocampus Retreat | attack |
| `horizon_thunder_sphere_technical` | Charged Ball Launcher | Horizon Thunder Sphere | save |
| `hydraulic_push_technical` | High-Pressure Fluid Cannon | Hydraulic Push | attack |
| `illusory_disguise_technical` | Full-Body Holographic Overlay | Illusory Disguise | none |
| `illusory_object_technical` | Persistent Hard-Light Projection | Illusory Object | none |
| `instant_pottery_technical` | Rapid Material Former | Instant Pottery | none |
| `interposing_earth_technical` | Terrain Shield Actuator | Interposing Earth | save |
| `invisible_item_technical` | Optical Cloaking Wrap | Invisible Item | none |
| `item_facade_technical` | Object Holographic Skin | Item Facade | none |
| `jump_technical` | Jump Jet Assist Pack | Jump | none |
| `kinetic_ram_technical` | Kinetic Impact Projector | Kinetic Ram | save |
| `leaden_steps_technical` | Magnetic Anchor System | Leaden Steps | save |
| `lock_technical` | Electronic Deadbolt Override | Lock | none |
| `mending_technical` | Nano-Repair Injector | Mending | none |
| `message_rune_technical` | Embedded Comm Chip | Message Rune | none |
| `mindlink_technical` | Neural Direct-Link Bridge | Mindlink | none |
| `mud_pit_technical` | Terrain Destabilizer Charge | Mud Pit | none |
| `mystic_armor_technical` | Force Field Emitter | Mystic Armor | none |
| `necromancers_generosity_technical` | Biohazard Transfer Agent | Necromancers Generosity | none |
| `negate_aroma_technical` | Scent Neutralization Spray | Negate Aroma | none |
| `noxious_vapors_technical` | Toxic Gas Dispersal Canister | Noxious Vapors | save |
| `nudge_the_odds_technical` | Luck Algorithm Adjustment | Nudge The Odds | none |
| `overselling_flourish_technical` | Dramatic Overclock Effect | Overselling Flourish | save |
| `penumbral_shroud_technical` | Darkfield Shroud | Penumbral Shroud | save |
| `personal_rain_cloud_technical` | Targeted Precipitation Unit | Personal Rain Cloud | save |
| `pest_form_technical` | Micro-Drone Disguise Shell | Pest Form | none |
| `pet_cache_technical` | Subspace Storage Unit | Pet Cache | none |
| `phantasmal_minion_technical` | Autonomous Holographic Decoy | Phantasmal Minion | none |
| `pocket_library_technical` | Portable Data Archive | Pocket Library | none |
| `pummeling_rubble_technical` | Debris Launch Barrage | Pummeling Rubble | save |
| `quick_sort_technical` | Automated Sorting Algorithm | Quick Sort | none |
| `rainbows_end_technical` | Prismatic Beam Array | Rainbows End | save |
| `reed_whistle_technical` | Signal Tone Generator | Reed Whistle | none |
| `restyle_technical` | Garment Nano-Actuator | Restyle | none |
| `runic_body_technical` | Combat Augmentation Layer | Runic Body | none |
| `runic_weapon_technical` | Weapon Enhancement Module | Runic Weapon | none |
| `schadenfreude_technical` | Adversity Feedback Loop | Schadenfreude | save |
| `scorching_blast_technical` | Thermal Plasma Fist | Scorching Blast | attack |
| `scouring_sand_technical` | Abrasive Particle Cannon | Scouring Sand | save |
| `seashell_of_stolen_sound_technical` | Sound Capture Device | Seashell Of Stolen Sound | none |
| `share_lore_technical` | Data Package Transfer | Share Lore | none |
| `shattering_gem_technical` | Resonance Shatter Charge | Shattering Gem | save |
| `shielded_arm_technical` | Forearm Shield Extrusion | Shielded Arm | none |
| `shocking_grasp_technical` | High-Voltage Contact Discharge | Shocking Grasp | attack |
| `shockwave_technical` | Seismic Pulse Generator | Shockwave | save |
| `signal_skyrocket_technical` | Flare Launcher | Signal Skyrocket | save |
| `sleep_technical` | Soporific Aerosol Grenade | Sleep | save |
| `snowball_technical` | Cryo-Gel Projectile | Snowball | attack |
| `spider_sting_technical` | Paralytic Micro-Dart | Spider Sting | save |
| `summon_animal_technical` | Trained Animal Deployment | Summon Animal | none |
| `summon_construct` | Combat Drone Deploy | Summon Construct | none |
| `summon_undead_technical` | Reanimation Protocol | Summon Undead | none |
| `sure_strike_technical` | Targeting Precision Lock | Sure Strike | none |
| `synchronize_steps_technical` | Movement Sync Protocol | Synchronize Steps | none |
| `synchronize_technical` | Beacon Frequency Sync | Synchronize | none |
| `tailwind_technical` | Slipstream Actuator | Tailwind | none |
| `temporary_tool` | Rapid Fabrication Print | Temporary Tool | none |
| `tether_technical` | Electrostatic Tether Line | Tether | save |
| `thicket_of_knives_technical` | Blade Array Projector | Thicket Of Knives | none |
| `thoughtful_gift_technical` | Object Transfer Launcher | Thoughtful Gift | none |
| `threefold_limb_technical` | Triple Strike Actuator | Threefold Limb | attack |
| `thunderstrike_technical` | EMP Thunder Discharge | Thunderstrike | save |
| `unbroken_panoply_technical` | Emergency Repair Override | Unbroken Panoply | none |
| `ventriloquism_technical` | Voice Projection Speaker | Ventriloquism | none |
| `wall_of_shrubs_technical` | Deployable Terrain Wall | Wall Of Shrubs | none |
| `weaken_earth_technical` | Ground Destabilizer Charge | Weaken Earth | save |
| `weave_wood_technical` | Polymer Shaper Tool | Weave Wood | none |
| `wooden_fists_technical` | Impact-Reinforced Gauntlets | Wooden Fists | none |

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

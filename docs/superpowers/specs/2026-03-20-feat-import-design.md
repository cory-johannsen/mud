# Feat Import — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `feat-import` (priority 260)
**Dependencies:** none

---

## Overview

Audit all PF2E general, skill, and adaptable miscellaneous feats against the existing `content/feats.yaml`. Identify covered feats (already mapped), gaps (new YAML entries needed), and skips (no viable Gunchete adaptation). Fill all gaps with Gunchete-flavored entries, then update `docs/architecture/character.md` with a Feat System section. All work is pure YAML — no code changes required.

---

## 1. Gunchete Adaptation Rules

| PF2E Concept | Gunchete Equivalent |
|---|---|
| Magic / Spells | Technology effects |
| Arcane / Divine / Occult / Primal traditions | Technology disciplines |
| Bless Tonic / Bless Toxin / Spirit Speaker / Crystal Healing / Chromotherapy | Drug-related Technology |
| Tattoo Artist | Implant Technician |
| Tattoo Artist (Legendary) | Legendary Implant Technician |
| Adopted Ancestry | Adopted Culture (gain access to another region's regional feats) |
| Ancestral Paragon | Cultural Roots (gain a 1st-level regional feat) |
| Ride / Mount | Vehicle Operator |
| Planar Survival | Zone Survival (hostile zone types: irradiated, flooded, toxic) |
| Craft magic items | Craft Technology items |
| Religion / Divine Guidance | Faction Guidance |
| Lore-named feats (Axuma's X, Kreighton's X, Pei Zing, Fane's, Ravening's, Eye of Arclords) | Renamed to Gunchete-flavor equivalents |
| Circus-specific feats | Skip |
| Spirit / supernatural feats with no drug/tech analog | Skip |

---

## 2. PF2E Skill → Gunchete Skill Mapping

| PF2E Skill | Gunchete Skill ID |
|---|---|
| Acrobatics | parkour |
| Stealth | ghosting |
| Thievery | grift |
| Athletics | muscle |
| Arcana | tech_lore |
| Crafting | rigging |
| Occultism | conspiracy |
| Society | factions |
| Lore | intel |
| Medicine | patch_job |
| Nature | wasteland |
| Religion | gang_codes |
| Survival | scavenging |
| Deception | hustle |
| Diplomacy | smooth_talk |
| Intimidation | hard_look |
| Performance | rep |

---

## 3. Audit Results

Status key: **Covered** = existing feat already maps this PF2E feat; **Gap** = new feat entry needed; **Skip** = no viable Gunchete adaptation.

---

### 3.1 General Feats

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Toughness | 1 | Covered | toughness | Toughness | Direct map |
| Fleet | 1 | Covered | fleet | Fleet | Direct map |
| Incredible Initiative | 1 | Covered | hair_trigger | Hair Trigger | Direct map |
| Armor Proficiency | 1 | Covered | armor_training | Armor Training | Direct map |
| Weapon Proficiency | 1 | Covered | weapon_training | Weapon Training | Direct map |
| Shield Block | 1 | Covered | block | Block | Direct map |
| Diehard | 1 | Covered | hard_to_kill | Hard to Kill | Direct map |
| Fast Recovery | 1 | Covered | quick_recovery | Quick Recovery | Direct map |
| Breath Control | 1 | Covered | iron_lungs | Iron Lungs | Direct map |
| Canny Acumen | 1 | Covered | sharpened_edge | Sharpened Edge | Direct map |
| Feather Step | 1 | Covered | light_step | Light Step | Direct map |
| Adopted Ancestry | 1 | Gap | adopted_culture | Adopted Culture | Gain access to another region's cultural feats |
| Pet | 1 | Gap | loyal_companion | Loyal Companion | Loyal pet animal or drone |
| Ride | 1 | Gap | vehicle_operator | Vehicle Operator | Auto-succeed at commanding your vehicle |
| Different Worlds | 1 | Gap | parallel_lives | Parallel Lives | Experience from diverse backgrounds grants versatility |
| Speedrun Strats | 1 | Gap | speedrun_strats | Speedrun Strats | Keep name; optimize movement routes |
| Ravening's Desperation | 1 | Gap | cornered_beast | Cornered Beast | Bonus when nearly dead; lore-name renamed |
| Ancestral Paragon | 3 | Gap | cultural_roots | Cultural Roots | Gain a 1st-level regional feat |
| Prescient Planner | 3 | Gap | contingency_stash | Contingency Stash | Procure a needed piece of gear |
| Untrained Improvisation | 3 | Gap | street_improvisation | Street Improvisation | Better at improvised skills |
| Improvised Repair | 3 | Gap | field_repair | Field Repair | Repair equipment without proper tools |
| Hireling Manager | 3 | Gap | crew_boss | Crew Boss | Manage hirelings more effectively |
| Robust Health | 3 | Gap | hardened_constitution | Hardened Constitution | Reduce disease/poison severity |
| Steel Your Resolve | 3 | Gap | steel_your_resolve | Steel Your Resolve | Shrug off mental conditions |
| Pick up the Pace | 3 | Gap | push_the_pace | Push the Pace | Increase group travel speed |
| Thorough Search | 3 | Gap | methodical_sweep | Methodical Sweep | Find things others miss |
| Keen Follower | 3 | Gap | sharp_follower | Sharp Follower | Bonus when following an expert |
| Skitter | 3 | Gap | skitter | Skitter | Move through small spaces quickly |
| Additional Circus Trick | 3 | Skip | — | — | No circus context in Gunchete |
| Eye of the Arclords | 2 | Gap | street_eye | Street Eye | Spot hidden things and concealed tech; lore-name renamed |
| Kreighton's Cognitive Crossover | 4 | Gap | neural_crossover | Neural Crossover | Apply one knowledge skill to another; lore-name renamed |
| Pei Zing Adept | 4 | Gap | district_adept | District Adept | Deep expertise in a specific zone district; lore-name renamed |
| Fane's Escape | 4 | Gap | ghost_step | Ghost Step | Escape impossible situations through sheer will; lore-name renamed |
| Expeditious Search | 7 | Gap | efficient_sweep | Efficient Sweep | Search areas faster |
| Prescient Consumable | 7 | Gap | contingency_consumable | Contingency Consumable | Procure needed consumables; requires contingency_stash |
| Bloodsense | 7 | Gap | vital_sense | Vital Sense | Detect wounded creatures nearby |
| Numb to Death | 7 | Gap | death_proof | Death Proof | Ignore one effect that would kill you per day |
| Supertaster | 7 | Gap | chemical_palate | Chemical Palate | Detect drug compounds by taste; Drug Tech adaptation |
| Axuma's Awakening | 11 | Gap | zone_sense | Zone Sense | Sense danger zone transitions and hazards; lore-name renamed |
| Axuma's Vigor | 11 | Gap | combat_vigor | Combat Vigor | Regain vitality during sustained combat; lore-name renamed |
| Incredible Investiture | 11 | Gap | tech_overload_capacity | Tech Overload Capacity | Invest more Technology items |
| Incredible Scout | 11 | Gap | scout_mastery | Scout Mastery | Bonus to initiative and scouting |
| A Home in Every Port | 11 | Gap | local_everywhere | Local Everywhere | Connections in every zone |
| Sanguine Tenacity | 11 | Gap | blood_will | Blood Will | Continue fighting past when you should fall |
| Caravan Leader | 11 | Gap | crew_leader | Crew Leader | Lead larger groups effectively |
| True Perception | 19 | Gap | true_perception | True Perception | Automatically detect hidden/invisible threats |

---

### 3.2 Skill Feats by Gunchete Skill

#### PARKOUR (Acrobatics)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Cat Fall | 1 | Covered | cat_fall | Fall Breaker | Direct map |
| Quick Squeeze | 1 | Covered | quick_squeeze | Squeeze Through | Direct map |
| Steady Balance | 1 | Covered | steady_balance | Street Footing | Direct map |
| Assurance (Acrobatics) | 1 | Covered | assurance_parkour | Assurance (Parkour) | Direct map |
| Acrobatic Performer | 1 | Gap | stunt_performance | Stunt Performance | Use parkour/stunts in performance |
| Nimble Crawl | 2 | Gap | fast_crawl | Fast Crawl | Crawl faster without penalty |
| Powerful Leap | 2 | Gap | power_jump | Power Jump | Jump higher and farther |
| Rapid Mantel | 2 | Gap | quick_vault | Quick Vault | Pull yourself onto ledges quickly |
| Rolling Landing | 2 | Gap | roll_landing | Roll Landing | Reduce fall damage further |
| Kip Up | 7 | Gap | kip_up | Kip Up | Stand for free without triggering reactions |
| Aerobatics Mastery | 7 | Gap | aerial_mastery | Aerial Mastery | Enhanced aerial maneuvers |
| Wall Jump | 7 | Gap | wall_jump | Wall Jump | Jump off walls to reach new heights |
| Water Sprint | 7 | Gap | water_run | Water Run | Sprint across water surfaces briefly |
| Cloud Jump | 15 | Gap | impossible_leap | Impossible Leap | Jump impossible distances |

#### MUSCLE (Athletics)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Combat Climber | 1 | Covered | combat_climber | Combat Climber | Direct map |
| Hefty Hauler | 1 | Covered | pack_mule | Pack Mule | Direct map |
| Quick Jump | 1 | Covered | quick_jump | Quick Jump | Direct map |
| Titan Wrestler | 1 | Covered | size_up | Size Up | Direct map |
| Underwater Marauder | 1 | Covered | waterproof | Waterproof | Direct map |
| Assurance (Athletics) | 1 | Covered | assurance_muscle | Assurance (Muscle) | Direct map |
| Lead Climber | 2 | Gap | anchor_climber | Anchor Climber | Help others climb safely |
| Quick Climb | 7 | Gap | speed_climb | Speed Climb | Climb at full speed |
| Quick Swim | 7 | Gap | speed_swim | Speed Swim | Swim at full speed |

#### GHOSTING (Stealth)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Experienced Smuggler | 1 | Covered | clean_pass | Clean Pass | Direct map |
| Terrain Stalker | 1 | Covered | zone_stalker | Zone Stalker | Direct map |
| Assurance (Stealth) | 1 | Covered | assurance_ghosting | Assurance (Ghosting) | Direct map |
| Armored Stealth | 2 | Gap | armored_ghost | Armored Ghost | Reduce stealth penalty in armor |
| Quiet Allies | 2 | Gap | silent_crew | Silent Crew | Allies use your stealth roll when sneaking together |
| Foil Senses | 7 | Gap | sense_block | Sense Block | Take precautions against enhanced senses |
| Swift Sneak | 7 | Gap | speed_ghost | Speed Ghost | Move at full speed while sneaking |
| Vanish Into the Land (Stealth) | 7 | Gap | terrain_vanish | Terrain Vanish | Disappear into natural terrain using Stealth |
| Legendary Sneak | 15 | Gap | ghost_legend | Ghost Legend | Hide and sneak without cover |

#### GRIFT (Thievery)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Pickpocket | 1 | Covered | pickpocket | Pickpocket | Direct map |
| Subtle Theft | 1 | Covered | clean_lift | Clean Lift | Direct map |
| Assurance (Thievery) | 1 | Covered | assurance_grift | Assurance (Grift) | Direct map |
| Concealing Legerdemain | 1 | Gap | concealed_activation | Concealed Activation | Hide Technology activation from observers |
| Wary Disarmament | 2 | Gap | careful_disarm | Careful Disarm | +2 AC/saves vs traps while disarming |
| Shadow Mark | 2 | Gap | shadow_mark | Shadow Mark | Mark a target for tracking/later action |
| Tumbling Theft | 7 | Gap | rolling_lift | Rolling Lift | Steal from target while tumbling through their space |
| Quick Unlock | 7 | Gap | speed_unlock | Speed Unlock | Pick a lock with 1 action |
| Legendary Thief | 15 | Gap | master_thief | Master Thief | Steal what would normally be impossible |

#### TECH LORE (Arcana)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Arcane Sense | 1 | Covered | tech_sense | Tech Sense | Direct map |
| Assurance (Arcana) | 1 | Covered | assurance_tech_lore | Assurance (Tech Lore) | Direct map |
| Quick Identification | 1 | Gap | quick_scan | Quick Scan | Identify Technology in 1 minute or less |
| Recognize Spell | 1 | Gap | id_tech_effect | ID Tech Effect | Identify a Technology effect as a reaction as it's activated |
| Trick Magic Item | 1 | Gap | exploit_tech | Exploit Tech | Activate Technology items you normally can't use |
| Eye for Numbers | 1 | Gap | data_eye | Data Eye | Quickly assess patterns and numerical data |
| Automatic Knowledge | 2 | Gap | instant_recall | Instant Recall | Recall knowledge about Technology as a free action once per round; requires Assurance |
| Magical Shorthand | 2 | Gap | rapid_protocol | Rapid Protocol | Learn new Technology protocols quickly and cheaply |
| Aura Sight | 2 | Gap | tech_aura_scan | Tech Aura Scan | Detect Technology auras while exploring |
| Bizarre Magic | 7 | Gap | obscured_tech | Obscured Tech | Technology activations are harder to identify |
| Quick Recognition | 7 | Gap | rapid_id | Rapid ID | Identify Technology effects as a free action |
| Unified Theory | 15 | Gap | tech_synthesis | Tech Synthesis | Use Tech Lore for checks across all Technology disciplines |

#### RIGGING (Crafting)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Alchemical Crafting | 1 | Covered | chem_crafting | Chem Crafting | Direct map |
| Quick Repair | 1 | Covered | quick_fix | Quick Fix | Direct map |
| Snare Crafting | 1 | Covered | trap_crafting | Trap Crafting | Direct map |
| Specialty Crafting | 1 | Covered | specialty_work | Specialty Work | Direct map |
| Assurance (Crafting) | 1 | Covered | assurance_rigging | Assurance (Rigging) | Direct map |
| Crafter's Appraisal | 1 | Gap | parts_eye | Parts Eye | Estimate value and quality of components at a glance |
| Communal Crafting | 2 | Gap | crew_craft | Crew Craft | Others can help you craft |
| Inventor | 2 | Gap | prototype_inventor | Prototype Inventor | Use Rigging to create new item schematics |
| Magical Crafting | 2 | Gap | tech_item_crafting | Tech Item Crafting | Craft Technology items |
| Cooperative Crafting | 2 | Gap | joint_work | Joint Work | Combine efforts for faster crafting |
| Tattoo Artist | 2 | Gap | implant_tech | Implant Tech | Perform basic cybernetic implant work; Tattoo Artist adaptation |
| Impeccable Crafting | 7 | Gap | master_rigging | Master Rigging | Craft items more efficiently; requires specialty_work |
| Signature Crafting | 7 | Gap | signature_build | Signature Build | Specialize in one type of crafted item |
| Bless Tonic | 7 | Gap | potency_compound | Potency Compound | Craft drug compounds that enhance vital force; Drug Tech |
| Bless Toxin | 7 | Gap | toxin_compound | Toxin Compound | Craft drug compounds with void properties; Drug Tech |
| Legendary Tattoo Artist | 15 | Gap | legendary_implant_tech | Legendary Implant Tech | Legendary cybernetic implant work |
| Craft Anything | 15 | Gap | rig_anything | Rig Anything | Ignore most requirements for crafting items |

#### CONSPIRACY (Occultism)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Oddity Identification | 1 | Covered | anomaly_id | Anomaly ID | Direct map |
| Assurance (Occultism) | 1 | Covered | assurance_conspiracy | Assurance (Conspiracy) | Direct map |
| Schooled in Secrets | 1 | Gap | gang_infiltrator | Gang Infiltrator | Gather info about and impersonate secret organization members |
| Spirit Speaker | 1 | Gap | drug_channel | Drug Channel | Commune with drug-altered states of consciousness; Drug Tech |
| Automatic Writing | 2 | Gap | auto_transcript | Auto Transcript | Subconsciously record information |
| Break Curse | 7 | Gap | break_conditioning | Break Conditioning | Remove mental conditioning and compulsions |
| Disturbing Knowledge | 7 | Gap | unsettling_intel | Unsettling Intel | Reveal knowledge so disturbing it frightens |
| Bizarre Magic (Occultism) | 7 | Gap | obscured_ops | Obscured Ops | Operations and methods are hard to identify; Occultism version |
| Consult the Spirits | 7 | Gap | consult_sources | Consult Sources | Tap secret information networks for guidance |
| Legendary Codebreaker (Occultism) | 15 | Gap | master_codebreaker | Master Codebreaker | Quickly decode any encryption or cipher |

#### FACTIONS (Society)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Courtly Graces | 1 | Covered | faction_protocol | Faction Protocol | Direct map |
| Multilingual | 1 | Covered | multilingual | Multilingual | Direct map |
| Read Lips | 1 | Covered | read_lips | Read Lips | Direct map |
| Sign Language | 1 | Covered | street_signs | Street Signs | Direct map |
| Streetwise | 1 | Covered | streetwise | Streetwise | Direct map |
| Assurance (Society) | 1 | Covered | assurance_factions | Assurance (Factions) | Direct map |
| Glean Contents | 1 | Gap | read_document | Read Document | Read documents without fully opening them |
| Secret Speech | 1 | Gap | code_speech | Code Speech | Communicate covertly in plain sight |
| Deceptive Worship | 1 | Gap | faction_cover | Faction Cover | Convincingly pose as member of opposing faction |
| Criminal Connections | 2 | Gap | black_market_access | Black Market Access | Connections to criminal networks |
| Eyes of the City | 2 | Gap | street_network | Street Network | Network of informants in any district |
| Underground Network | 2 | Gap | underground_network | Underground Network | Resistance/criminal network ties |
| Legendary Linguist | 15 | Gap | polyglot_master | Polyglot Master | Create shared communication with anyone |
| Legendary Codebreaker (Society) | 15 | Gap | faction_linguist | Faction Linguist | Instantly decode any faction cipher or social code |

#### INTEL (Lore)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Additional Lore | 1 | Covered | field_intel | Field Intel | Direct map |
| Experienced Professional | 1 | Covered | street_cred | Street Cred | Direct map |
| Assurance (Lore) | 1 | Covered | assurance_intel | Assurance (Intel) | Direct map |
| Dubious Knowledge | 1 | Gap | mixed_intel | Mixed Intel | On a failed check, learn true info mixed with false |
| Unmistakable Lore | 2 | Gap | solid_intel | Solid Intel | Recall knowledge more effectively |
| Legendary Professional | 15 | Gap | legendary_operator | Legendary Operator | Renown for specialized knowledge |

#### PATCH JOB (Medicine)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Battle Medicine | 1 | Covered | combat_patch | Combat Patch | Direct map |
| Assurance (Medicine) | 1 | Covered | assurance_patch_job | Assurance (Patch Job) | Direct map |
| Acupuncturist | 1 | Gap | stim_points | Stim Points | Stimulation technique that enhances recovery; Drug Tech adjacent |
| Inoculation | 1 | Gap | inoculation | Inoculation | Protect others against disease and poison |
| Medical Researcher | 1 | Gap | field_researcher | Field Researcher | Learn about diseases and toxins encountered |
| Risky Surgery | 1 | Gap | risky_op | Risky Op | Accept risk for better medical outcomes |
| Stitch Flesh | 1 | Gap | field_suture | Field Suture | Basic wound closure without medical supplies |
| Crystal Healing | 1 | Gap | crystalline_stim | Crystalline Stim | Use crystalline drug compounds for enhanced healing; Drug Tech |
| Continual Recovery | 2 | Gap | rapid_treatment | Rapid Treatment | Treat wounds more often |
| Robust Recovery | 2 | Gap | tough_patient | Tough Patient | Greater benefits from treatment |
| Ward Medic | 2 | Gap | mass_treatment | Mass Treatment | Treat several patients at once |
| Unusual Treatment | 2 | Gap | extended_treatment | Extended Treatment | Treat additional conditions beyond wounds |
| Godless Healing | 2 | Gap | secular_medicine | Secular Medicine | Heal without relying on Technology bonuses |
| Mortal Healing | 2 | Gap | natural_healing | Natural Healing | Use natural processes to heal without tech |
| Performer's Treatment | 2 | Gap | comfort_treatment | Comfort Treatment | Use calm presence to aid recovery |
| Chromotherapy | 2 | Gap | light_therapy | Light Therapy | Specialized light-based treatment; Drug Tech |
| Vasodilation | 2 | Gap | drug_vasodilation | Drug Vasodilation | Drug-assisted vasodilation for rapid healing; Drug Tech |
| Paragon Battle Medicine | 7 | Gap | master_combat_patch | Master Combat Patch | Enhanced in-combat healing |
| Advanced First Aid | 7 | Gap | advanced_patch_job | Advanced Patch Job | Use first aid to reduce frightened and sickened conditions |
| Legendary Medic | 15 | Gap | legendary_medic | Legendary Medic | Cure serious conditions including blindness and paralysis |

#### WASTELAND (Nature)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Natural Medicine | 1 | Covered | wasteland_remedy | Wasteland Remedy | Direct map |
| Train Animal | 1 | Covered | animal_bond | Animal Bond | Direct map |
| Assurance (Nature) | 1 | Covered | assurance_wasteland | Assurance (Wasteland) | Direct map |
| All of the Animal | 1 | Gap | creature_speak | Creature Speak | Understand all animal/creature communication |
| Tame Animal | 1 | Gap | tame_creature | Tame Creature | Tame wild creatures through exploration |
| Myth Hunter | 1 | Gap | legend_hunter | Legend Hunter | Track and research legendary creatures |
| Bonded Animal | 2 | Gap | permanent_bond | Permanent Bond | An animal becomes permanently easier to command |
| Influence Nature | 7 | Gap | zone_influence | Zone Influence | Influence wasteland creature behavior |

#### GANG CODES (Religion)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Student of the Canon | 1 | Covered | code_scholar | Code Scholar | Direct map |
| Assurance (Religion) | 1 | Covered | assurance_gang_codes | Assurance (Gang Codes) | Direct map |
| Pilgrim's Token | 1 | Gap | faction_token | Faction Token | Carry token that marks faction allegiance |
| Exhort the Faithful | 2 | Gap | rally_the_crew | Rally the Crew | Motivate faction members for bonuses |
| Sacred Defense | 7 | Gap | code_shield | Code Shield | Faction codes provide protection |
| Battle Prayer | 7 | Gap | battle_code | Battle Code | Invoke faction battle code for combat bonuses |
| Evangelize | 7 | Gap | spread_the_word | Spread the Word | Recruit and inspire faction loyalty |
| Divine Guidance | 15 | Gap | faction_oracle | Faction Oracle | Receive strategic guidance from faction leadership |

#### SCAVENGING (Survival)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Experienced Tracker | 1 | Covered | trail_reader | Trail Reader | Direct map |
| Forager | 1 | Covered | scavenger_eye | Scavenger's Eye | Direct map |
| Survey Wildlife | 1 | Covered | zone_survey | Zone Survey | Direct map |
| Terrain Expertise | 1 | Covered | zone_expertise | Zone Expertise | Direct map |
| Assurance (Survival) | 1 | Covered | assurance_scavenging | Assurance (Scavenging) | Direct map |
| Vanish Into the Land (Survival) | 7 | Gap | zone_vanish | Zone Vanish | Disappear into wasteland environment using Survival |
| Monster Crafting | 7 | Gap | salvage_crafting | Salvage Crafting | Craft items from defeated enemy parts |
| Planar Survival | 7 | Gap | zone_survival | Zone Survival | Survive in any hostile zone type (irradiated, flooded, toxic) |
| Environmental Guide | 7 | Gap | zone_guide | Zone Guide | Expertly guide others through dangerous wasteland |
| Legendary Survivalist | 15 | Gap | legendary_survivor | Legendary Survivor | Survive extreme environmental conditions |
| Legendary Guide | 15 | Gap | legendary_guide | Legendary Guide | Guide anyone through any terrain |

#### HUSTLE (Deception)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Charming Liar | 1 | Covered | silver_tongue | Silver Tongue | Direct map |
| Lengthy Diversion | 1 | Covered | long_con | Long Con | Direct map |
| Lie to Me | 1 | Covered | bs_detector | BS Detector | Direct map |
| Assurance (Deception) | 1 | Covered | assurance_hustle | Assurance (Hustle) | Direct map |
| Bon Mot | 1 | Gap | cutting_quip | Cutting Quip | Quick verbal jab that unsettles a target |
| Charlatan | 1 | Gap | street_charlatan | Street Charlatan | Pass yourself off as legitimate professional |
| Vicious Critique (Deception) | 1 | Gap | vicious_critique | Vicious Critique | Demoralize with scathing criticism |
| Confabulator | 2 | Gap | smooth_liar | Smooth Liar | Reduce bonuses against your repeated lies |
| Quick Disguise | 2 | Gap | quick_disguise | Quick Disguise | Set up a disguise in much less time |
| Backup Disguise | 2 | Gap | spare_identity | Spare Identity | Maintain a backup disguise |
| Half-Truths | 2 | Gap | half_truths | Half Truths | Blend lies with truth for plausible deniability |
| Sow Rumor | 2 | Gap | plant_rumor | Plant Rumor | Secretly plant damaging rumors |
| Tweak Appearances | 2 | Gap | quick_alter | Quick Alter | Quickly change someone's appearance |
| Distracting Performance (Deception) | 2 | Gap | divert_attention | Divert Attention | Use performance to distract observers; Deception version |
| Doublespeak | 7 | Gap | doublespeak | Doublespeak | Convey two contradictory messages simultaneously |
| Slippery Secrets | 7 | Gap | secrets_shield | Secrets Shield | Evade attempts to uncover your true nature |

#### SMOOTH TALK (Diplomacy)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Bargain Hunter | 1 | Covered | deal_finder | Deal Finder | Direct map |
| Group Impression | 1 | Covered | crowd_work | Crowd Work | Direct map |
| Hobnobber | 1 | Covered | street_networker | Street Networker | Direct map |
| Assurance (Diplomacy) | 1 | Covered | assurance_smooth_talk | Assurance (Smooth Talk) | Direct map |
| No Cause for Alarm | 1 | Gap | calm_down | Calm Down | Reduce frightened condition values in others |
| Contract Negotiator | 1 | Gap | deal_maker | Deal Maker | Negotiate formal agreements efficiently |
| Glad-Hand | 2 | Gap | fast_friends | Fast Friends | Make a favorable impression on someone you just met |
| Encouraging Words | 2 | Gap | pick_em_up | Pick 'Em Up | Boost an ally's spirits with words |
| Leverage Connections | 2 | Gap | call_in_favors | Call In Favors | Use connections for tangible aid |
| Quick Contacts | 2 | Gap | speed_network | Speed Network | Quickly establish contacts in a new area |
| Discreet Inquiry | 2 | Gap | quiet_ask | Quiet Ask | Gather information without revealing your interest |
| Shameless Request | 7 | Gap | brass_nerves | Brass Nerves | Make extreme requests with lesser consequences |
| Legendary Negotiation | 15 | Gap | master_negotiator | Master Negotiator | Quickly parley with hostile parties |

#### HARD LOOK (Intimidation)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Group Coercion | 1 | Covered | mass_intimidation | Mass Intimidation | Direct map |
| Intimidating Glare | 1 | Covered | death_stare | Death Stare | Direct map |
| Quick Coercion | 1 | Covered | fast_threat | Fast Threat | Direct map |
| Assurance (Intimidation) | 1 | Covered | assurance_hard_look | Assurance (Hard Look) | Direct map |
| Intimidating Prowess | 2 | Gap | physical_threat | Physical Threat | Bonus when physically Demoralizing a target |
| Lasting Coercion | 2 | Gap | long_scare | Long Scare | Coerce targets into helping for longer |
| Terrifying Resistance | 2 | Gap | iron_nerves | Iron Nerves | Reduce frightened conditions against yourself |
| Say That Again! | 6 | Gap | say_that_again | Say That Again | React when a creature calls out your actions |
| Battle Cry | 7 | Gap | battle_shout | Battle Shout | Demoralize foes when rolling for initiative |
| Terrified Retreat | 7 | Gap | break_them | Break Them | Cause demoralized targets to flee |
| Too Angry to Die | 12 | Gap | too_angry_to_die | Too Angry to Die | Continue fighting past when you should fall |
| Scare to Death | 15 | Gap | scare_to_death | Scare to Death | Frighten a target so severely they may be incapacitated |

#### REP (Performance)

| PF2E Name | Level | Status | Gunchete ID | Gunchete Name | Notes |
|---|---|---|---|---|---|
| Fascinating Performance | 1 | Covered | crowd_control | Crowd Control | Direct map |
| Impressive Performance | 1 | Covered | rep_play | Rep Play | Direct map |
| Virtuosic Performer | 1 | Covered | signature_style | Signature Style | Direct map |
| Assurance (Performance) | 1 | Covered | assurance_rep | Assurance (Rep) | Direct map |
| Acrobatic Performer (Rep) | 1 | Gap | stunt_performance | Stunt Performance | Incorporate physical stunts into performance; NOTE: shared ID with parkour entry — treat as multi-skill feat |
| Vicious Critique (Performance) | 1 | Gap | vicious_critique_rep | Vicious Critique (Rep) | Demoralize with scathing criticism via Performance |
| Distracting Performance (Rep) | 2 | Gap | stage_diversion | Stage Diversion | Use performance to distract observers; Performance version |
| Triumphant Boast | 2 | Gap | winners_speech | Winner's Speech | Inspire allies after a victory |
| Juggle | 2 | Gap | juggle | Juggle | Juggle multiple objects as performance |
| Talent Envy | 7 | Gap | copy_the_master | Copy the Master | Study and replicate a technique you've observed |
| Inflame Crowd | 7 | Gap | rile_the_crowd | Rile the Crowd | Turn audience emotion toward your desired reaction |
| Entourage | 7 | Gap | street_following | Street Following | Attract a loyal following |
| Comforting Presence | 10 | Gap | comforting_presence | Comforting Presence | Presence helps allies resist fear |
| Legendary Performer | 15 | Gap | legendary_rep | Legendary Rep | Renown for your reputation |

---

### 3.3 Miscellaneous (Adaptable)

PF2E deviant/mythic abilities adapted as general feats accessible through Gunchete's Technology, drug, and implant themes.

| PF2E Name | Gunchete ID | Gunchete Name | Status | Notes |
|---|---|---|---|---|
| Overclock Senses | overclock_senses | Overclock Senses | Gap | Temporarily boost perception via neural implant |
| Unstable Gearshift | unstable_gearshift | Unstable Gearshift | Gap | Risky but powerful gear modification |
| Irradiate | irradiate | Irradiate | Gap | Emit a burst of radiation affecting nearby targets |
| Sonic Dash | sonic_dash | Sonic Dash | Gap | Burst of sonic-tech assisted speed |
| Titan Swing | titan_swing | Titan Swing | Gap | Massive powered weapon swing with knockback |
| High-Speed Regeneration | rapid_regen | Rapid Regen | Gap | Regenerative drug/implant heals HP each round |
| Pain is Temporary | pain_is_temporary | Pain Is Temporary | Gap | Ignore pain-based penalties briefly |
| Like a Roach | like_a_roach | Like a Roach | Gap | Survive lethal hits on 1 HP once per day |
| Indomitable Spirit | unbreakable_will | Unbreakable Will | Gap | Resist mental domination effects |
| Weight of Experience | old_hand | Old Hand | Gap | Vast experience grants situational bonuses |
| I've Had Many Jobs | many_jobs | Many Jobs | Gap | Bonus from multiple job experience |
| Propulsive Leap | thruster_leap | Thruster Leap | Gap | Use tech thrusters to leap farther |
| Trailblazing Stride | trailblazer | Trailblazer | Gap | Move through difficult terrain without penalty |
| Unbreakable Resolve | iron_resolve | Iron Resolve | Gap | Resist being mentally broken |
| Calm and Centered | calm_and_centered | Calm and Centered | Gap | Mental calm provides bonuses vs effects |

**Skip (no viable Gunchete adaptation):**

| PF2E Name | Reason |
|---|---|
| Sanctify Water | Pure divine/supernatural; no drug/tech analog |
| Boneyard Acquaintance | Requires true faith in Pharasma; no analog |
| Elysium's Cadence | Supernatural planar; no analog |
| Blessing of the Five | Divine covenant mechanic; no analog |
| Additional Circus Trick | Circus context does not exist in Gunchete |
| Morphic Manipulation | Pure magic; no tech analog |
| Root Magic | Pure magic; no tech analog |
| Prepare Elemental Medicine | Elemental magic; no tech analog |
| Pilgrim's Token variants requiring true faith | Faith-gated; faction_token covers the secular version |

---

## 4. Requirements

- REQ-FI-1: All Gap feats from section 3 MUST be added to `content/feats.yaml` in the correct skill section.
- REQ-FI-2: Each new feat MUST include a `pf2e` field matching the source PF2E feat name.
- REQ-FI-3: Each new feat MUST have a Gunchete-flavored `name` and `description`; PF2E flavor text MUST NOT appear verbatim.
- REQ-FI-4: Skill feats MUST include a `skill` field matching the Gunchete skill ID from Section 2.
- REQ-FI-5: Covered feats MUST NOT be duplicated; existing feats already satisfying a PF2E mapping MUST be left unchanged.
- REQ-FI-6: All Skip feats MUST NOT be added to the system.
- REQ-FI-7: `docs/architecture/character.md` MUST be updated with a Feat System section documenting the three-category structure, the `pf2e` field convention, and the extension point for adding new feats.
- REQ-FI-8: The feat registry (`internal/game/ruleset/feat.go`) MUST NOT require code changes; all new feats are pure YAML additions.

---

## 5. Architecture Doc Update

The following section MUST be added to `docs/architecture/character.md`:

### Feat System

Content lives in `content/feats.yaml`. The registry (`internal/game/ruleset/feat.go`) provides four indexes: `byID`, `byCategory`, `bySkill`, `byArchetype`.

**Three feat categories:**

- **general** — available in the general feat pool at character creation; no skill requirement.
- **skill** — gated by skill proficiency; must carry a `skill` field matching a Gunchete skill ID; available when the player is Trained or better in that skill.
- **job** — archetype-specific; must carry an `archetype` field; granted or chosen at job selection.

The `pf2e` field records the source PF2E feat name for traceability. It is not used at runtime.

Adding a new feat requires only a YAML entry — no code changes. The registry auto-indexes on load.

**Extension Point:**

1. Add entry to `content/feats.yaml` in the appropriate section.
2. Set `category`, `skill` (if skill feat), `pf2e` (source name), `active` (true if player-triggered), `description` (Gunchete-flavored).
3. Restart server — feat is immediately available in the registry.

---

## 6. Feat Count Summary

| Category | Covered | Gap (New) | Skip | Total |
|---|---|---|---|---|
| General Feats | 11 | 35 | 1 | 47 |
| Parkour (Acrobatics) | 4 | 10 | 0 | 14 |
| Muscle (Athletics) | 6 | 3 | 0 | 9 |
| Ghosting (Stealth) | 3 | 6 | 0 | 9 |
| Grift (Thievery) | 3 | 6 | 0 | 9 |
| Tech Lore (Arcana) | 2 | 10 | 0 | 12 |
| Rigging (Crafting) | 5 | 12 | 0 | 17 |
| Conspiracy (Occultism) | 2 | 8 | 0 | 10 |
| Factions (Society) | 6 | 8 | 0 | 14 |
| Intel (Lore) | 3 | 3 | 0 | 6 |
| Patch Job (Medicine) | 2 | 19 | 0 | 21 |
| Wasteland (Nature) | 3 | 5 | 0 | 8 |
| Gang Codes (Religion) | 2 | 6 | 0 | 8 |
| Scavenging (Survival) | 5 | 6 | 0 | 11 |
| Hustle (Deception) | 4 | 12 | 0 | 16 |
| Smooth Talk (Diplomacy) | 4 | 9 | 0 | 13 |
| Hard Look (Intimidation) | 4 | 8 | 0 | 12 |
| Rep (Performance) | 4 | 10 | 0 | 14 |
| Miscellaneous (Adaptable) | 0 | 15 | 9 | 24 |
| **Total** | **73** | **195** | **10** | **278** |

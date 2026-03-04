# Feats Stage 2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement all level-1 feats (general, skill, and job categories) with interactive
selection at character creation, backfill for existing characters, a `feats` command, a
repurposed `use` command for feat activation, and a new `interact` command to replace the
old `use` for room equipment.

**Architecture:** Follows the exact same patterns as Skills Stage 1 — YAML content file,
Go ruleset types, DB table, repository, character model extension, character creation flow,
and full CMD-1 through CMD-7 pipeline for each new command. Job feats are defined per
archetype; each of the 76 job YAMLs gets a `feats` block mirroring the `skills` block.

**Tech Stack:** Go, PostgreSQL (pgx/v5), protobuf/gRPC, YAML, Telnet color rendering.

**Design doc:** `docs/plans/2026-03-03-feats-stage2-design.md`

---

## Context: Key Existing Patterns

Before starting, understand these patterns from Skills Stage 1:

- `internal/game/ruleset/skill.go` — `Skill` struct + `LoadSkills(path)` — copy this pattern
- `internal/storage/postgres/character_skills.go` — `CharacterSkillsRepository` — copy this pattern
- `internal/game/command/skills.go` — minimal stub, server does the work
- `internal/frontend/handlers/bridge_handlers.go` line 86 — `bridgeHandlerMap` registration
- `internal/frontend/handlers/game_bridge.go` line 465 — `ServerEvent` switch for rendering
- `internal/gameserver/grpc_service.go` — `handleSkills` for the server-side handler pattern
- `api/proto/game/v1/game.proto` — `ClientMessage` oneof uses fields 2–38; `ServerEvent` uses 2–19

---

## Task 1: `content/feats.yaml` — Master Feat Definitions

**Files:**
- Create: `content/feats.yaml`

**Step 1: Create the file**

```yaml
feats:
  # ── GENERAL FEATS ─────────────────────────────────────────────────────────
  - id: toughness
    name: Toughness
    category: general
    pf2e: toughness
    active: false
    activate_text: ""
    description: "Max HP increases by your level; recovery checks are easier."

  - id: fleet
    name: Fleet
    category: general
    pf2e: fleet
    active: false
    activate_text: ""
    description: "Your movement speed increases by 5 feet."

  - id: hair_trigger
    name: Hair Trigger
    category: general
    pf2e: incredible_initiative
    active: false
    activate_text: ""
    description: "+2 to initiative rolls."

  - id: armor_training
    name: Armor Training
    category: general
    pf2e: armor_proficiency
    active: false
    activate_text: ""
    description: "Gain proficiency in one additional armor category."

  - id: weapon_training
    name: Weapon Training
    category: general
    pf2e: weapon_proficiency
    active: false
    activate_text: ""
    description: "Gain proficiency with one weapon group or specific weapon."

  - id: block
    name: Block
    category: general
    pf2e: shield_block
    active: true
    activate_text: "You raise your shield and absorb the blow."
    description: "When hit by a physical attack, use your reaction to reduce damage with a raised shield."

  - id: hard_to_kill
    name: Hard to Kill
    category: general
    pf2e: diehard
    active: false
    activate_text: ""
    description: "You die at dying 5 instead of dying 4."

  - id: quick_recovery
    name: Quick Recovery
    category: general
    pf2e: fast_recovery
    active: false
    activate_text: ""
    description: "Recover twice as fast; rest restores bonus HP equal to your Grit modifier."

  - id: iron_lungs
    name: Iron Lungs
    category: general
    pf2e: breath_control
    active: false
    activate_text: ""
    description: "Hold your breath 25x longer; bonus to saves vs. gas and inhaled toxins."

  - id: sharpened_edge
    name: Sharpened Edge
    category: general
    pf2e: canny_acumen
    active: false
    activate_text: ""
    description: "Increase your proficiency rank in Fortitude, Reflex, Will, or Perception."

  - id: light_step
    name: Light Step
    category: general
    pf2e: feather_step
    active: true
    activate_text: "You pick your way through the debris with practiced ease."
    description: "Step into difficult terrain (rubble, debris) as normal movement."

  # ── SKILL FEATS: PARKOUR ──────────────────────────────────────────────────
  - id: assurance_parkour
    name: Steady Parkour
    category: skill
    skill: parkour
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Parkour rolls as 10; ignore circumstance and status penalties."

  - id: fall_breaker
    name: Fall Breaker
    category: skill
    skill: parkour
    pf2e: cat_fall
    active: false
    activate_text: ""
    description: "Treat falls as 10 feet shorter; you land rolling without injury."

  - id: squeeze_through
    name: Squeeze Through
    category: skill
    skill: parkour
    pf2e: quick_squeeze
    active: true
    activate_text: "You contort and slip through at full speed."
    description: "Move through tight spaces at full speed without slowing."

  - id: street_footing
    name: Street Footing
    category: skill
    skill: parkour
    pf2e: steady_balance
    active: false
    activate_text: ""
    description: "Ignore unstable ground penalties; stand from prone without drawing reactions."

  # ── SKILL FEATS: GHOSTING ─────────────────────────────────────────────────
  - id: assurance_ghosting
    name: Steady Ghosting
    category: skill
    skill: ghosting
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Ghosting rolls as 10."

  - id: clean_pass
    name: Clean Pass
    category: skill
    skill: ghosting
    pf2e: experienced_smuggler
    active: false
    activate_text: ""
    description: "+2 to conceal items; can hide objects in plain sight on your person."

  - id: zone_stalker
    name: Zone Stalker
    category: skill
    skill: ghosting
    pf2e: terrain_stalker
    active: false
    activate_text: ""
    description: "Choose one terrain type (rubble, sewer, crowd); move through it silently without a roll."

  # ── SKILL FEATS: GRIFT ────────────────────────────────────────────────────
  - id: assurance_grift
    name: Steady Grift
    category: skill
    skill: grift
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Grift rolls as 10."

  - id: pickpocket
    name: Pickpocket
    category: skill
    skill: grift
    pf2e: pickpocket
    active: true
    activate_text: "Your hand moves before they even notice."
    description: "Lift items up to light bulk even from alert targets without penalty."

  - id: clean_lift
    name: Clean Lift
    category: skill
    skill: grift
    pf2e: subtle_theft
    active: false
    activate_text: ""
    description: "Bystanders must pass a Perception check to notice a successful steal."

  # ── SKILL FEATS: MUSCLE ───────────────────────────────────────────────────
  - id: assurance_muscle
    name: Steady Muscle
    category: skill
    skill: muscle
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Muscle rolls as 10."

  - id: combat_climber
    name: Combat Climber
    category: skill
    skill: muscle
    pf2e: combat_climber
    active: false
    activate_text: ""
    description: "Climb without going flat-footed; use a hand holding an item for climbing."

  - id: pack_mule
    name: Pack Mule
    category: skill
    skill: muscle
    pf2e: hefty_hauler
    active: false
    activate_text: ""
    description: "Your carry weight limit increases by 2 Bulk."

  - id: quick_jump
    name: Quick Jump
    category: skill
    skill: muscle
    pf2e: quick_jump
    active: true
    activate_text: "You launch yourself in one fluid motion."
    description: "High Jump or Long Jump as a single action instead of two."

  - id: size_up
    name: Size Up
    category: skill
    skill: muscle
    pf2e: titan_wrestler
    active: false
    activate_text: ""
    description: "Disarm, Grapple, Shove, or Trip creatures up to two sizes larger than you."

  - id: waterproof
    name: Waterproof
    category: skill
    skill: muscle
    pf2e: underwater_marauder
    active: false
    activate_text: ""
    description: "No penalty for weapon use underwater; not flat-footed while submerged."

  # ── SKILL FEATS: TECH LORE ────────────────────────────────────────────────
  - id: assurance_tech_lore
    name: Steady Tech Lore
    category: skill
    skill: tech_lore
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Tech Lore rolls as 10."

  - id: tech_sense
    name: Tech Sense
    category: skill
    skill: tech_lore
    pf2e: arcane_sense
    active: true
    activate_text: "You close your eyes and let your training guide you. Nearby electronics flicker at the edge of your awareness."
    description: "Detect active electronics, transmitters, or powered devices nearby at will."

  # ── SKILL FEATS: RIGGING ──────────────────────────────────────────────────
  - id: assurance_rigging
    name: Steady Rigging
    category: skill
    skill: rigging
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Rigging rolls as 10."

  - id: chem_crafting
    name: Chem Crafting
    category: skill
    skill: rigging
    pf2e: alchemical_crafting
    active: true
    activate_text: "You mix the compounds from memory, hands moving with practiced efficiency."
    description: "Craft drugs, poisons, and chemical items; start with four common formulas."

  - id: quick_fix
    name: Quick Fix
    category: skill
    skill: rigging
    pf2e: quick_repair
    active: true
    activate_text: "You jam the components together and hope for the best."
    description: "Repair gear in 1 minute instead of 10; at higher skill in a single action."

  - id: trap_crafting
    name: Trap Crafting
    category: skill
    skill: rigging
    pf2e: snare_crafting
    active: true
    activate_text: "You set the mechanism with careful hands."
    description: "Craft mechanical traps; start with four common trap formulas."

  - id: specialty_work
    name: Specialty Work
    category: skill
    skill: rigging
    pf2e: specialty_crafting
    active: false
    activate_text: ""
    description: "Choose a craft type; +1 bonus to that type, improving with proficiency."

  # ── SKILL FEATS: CONSPIRACY ───────────────────────────────────────────────
  - id: assurance_conspiracy
    name: Steady Conspiracy
    category: skill
    skill: conspiracy
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Conspiracy rolls as 10."

  - id: anomaly_id
    name: Anomaly ID
    category: skill
    skill: conspiracy
    pf2e: oddity_identification
    active: false
    activate_text: ""
    description: "+2 to Recall Knowledge about cover-ups, weird phenomena, and shadow organizations."

  # ── SKILL FEATS: FACTIONS ─────────────────────────────────────────────────
  - id: assurance_factions
    name: Steady Factions
    category: skill
    skill: factions
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Factions rolls as 10."

  - id: faction_protocol
    name: Faction Protocol
    category: skill
    skill: factions
    pf2e: courtly_graces
    active: true
    activate_text: "You switch into the correct register for this crowd."
    description: "Use Factions instead of Smooth Talk to make an impression on faction leaders."

  - id: multilingual
    name: Multilingual
    category: skill
    skill: factions
    pf2e: multilingual
    active: false
    activate_text: ""
    description: "Learn two additional street dialects or languages."

  - id: read_lips
    name: Read Lips
    category: skill
    skill: factions
    pf2e: read_lips
    active: false
    activate_text: ""
    description: "Lip-read anyone you can see, even through a window or at a distance."

  - id: street_signs
    name: Street Signs
    category: skill
    skill: factions
    pf2e: sign_language
    active: false
    activate_text: ""
    description: "Know gang sign language for your territory plus one additional crew's signs."

  - id: streetwise
    name: Streetwise
    category: skill
    skill: factions
    pf2e: streetwise
    active: true
    activate_text: "You tap your network and start asking the right questions."
    description: "Use Factions instead of Smooth Talk to gather information in urban areas."

  # ── SKILL FEATS: INTEL ────────────────────────────────────────────────────
  - id: assurance_intel
    name: Steady Intel
    category: skill
    skill: intel
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Intel rolls as 10."

  - id: field_intel
    name: Field Intel
    category: skill
    skill: intel
    pf2e: additional_lore
    active: false
    activate_text: ""
    description: "Add one additional Intel specialty area as a trained knowledge."

  - id: street_cred
    name: Street Cred
    category: skill
    skill: intel
    pf2e: experienced_professional
    active: false
    activate_text: ""
    description: "Earn income via Intel; on a critical failure you earn the partial amount instead of nothing."

  # ── SKILL FEATS: PATCH JOB ────────────────────────────────────────────────
  - id: assurance_patch_job
    name: Steady Patch Job
    category: skill
    skill: patch_job
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Patch Job rolls as 10."

  - id: combat_patch
    name: Combat Patch
    category: skill
    skill: patch_job
    pf2e: battle_medicine
    active: true
    activate_text: "You slap a patch on the wound and keep moving."
    description: "Apply first aid as a single action without a kit; target immune for 1 day."

  # ── SKILL FEATS: WASTELAND ────────────────────────────────────────────────
  - id: assurance_wasteland
    name: Steady Wasteland
    category: skill
    skill: wasteland
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Wasteland rolls as 10."

  - id: wasteland_remedy
    name: Wasteland Remedy
    category: skill
    skill: wasteland
    pf2e: natural_medicine
    active: true
    activate_text: "You gather what you can from the ruins and get to work."
    description: "Use Wasteland instead of Patch Job to treat wounds with found materials."

  - id: animal_bond
    name: Animal Bond
    category: skill
    skill: wasteland
    pf2e: train_animal
    active: true
    activate_text: "You crouch down and make yourself small, earning trust the slow way."
    description: "Teach a non-magical animal up to 2 commands or tricks."

  # ── SKILL FEATS: GANG CODES ───────────────────────────────────────────────
  - id: assurance_gang_codes
    name: Steady Gang Codes
    category: skill
    skill: gang_codes
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Gang Codes rolls as 10."

  - id: code_scholar
    name: Code Scholar
    category: skill
    skill: gang_codes
    pf2e: student_of_the_canon
    active: false
    activate_text: ""
    description: "Never critically fail Gang Codes recall checks; +2 to religious and ritual knowledge."

  # ── SKILL FEATS: SCAVENGING ───────────────────────────────────────────────
  - id: assurance_scavenging
    name: Steady Scavenging
    category: skill
    skill: scavenging
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Scavenging rolls as 10."

  - id: trail_reader
    name: Trail Reader
    category: skill
    skill: scavenging
    pf2e: experienced_tracker
    active: false
    activate_text: ""
    description: "Track without moving at half speed; follow trails while moving at normal pace."

  - id: scavengers_eye
    name: Scavenger's Eye
    category: skill
    skill: scavenging
    pf2e: forager
    active: true
    activate_text: "You sweep the area with practiced eyes, pulling together what you need."
    description: "Subsist yourself and allies equal to your Scavenging modifier in the ruins."

  - id: zone_survey
    name: Zone Survey
    category: skill
    skill: scavenging
    pf2e: survey_wildlife
    active: true
    activate_text: "You spend ten minutes reading the zone — movement, smells, heat signatures."
    description: "10-minute survey of an area reveals threats, resources, and hazards."

  - id: zone_expertise
    name: Zone Expertise
    category: skill
    skill: scavenging
    pf2e: terrain_expertise
    active: false
    activate_text: ""
    description: "Choose a terrain (ruins, sewers, industrial); +1 to Scavenging checks there."

  # ── SKILL FEATS: HUSTLE ───────────────────────────────────────────────────
  - id: assurance_hustle
    name: Steady Hustle
    category: skill
    skill: hustle
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Hustle rolls as 10."

  - id: silver_tongue
    name: Silver Tongue
    category: skill
    skill: hustle
    pf2e: charming_liar
    active: false
    activate_text: ""
    description: "Critical success on a lie makes the target friendly or helpful toward you."

  - id: long_con
    name: Long Con
    category: skill
    skill: hustle
    pf2e: lengthy_diversion
    active: false
    activate_text: ""
    description: "A diversion you create lasts until end of your next turn rather than a moment."

  - id: bs_detector
    name: BS Detector
    category: skill
    skill: hustle
    pf2e: lie_to_me
    active: true
    activate_text: "You watch their face carefully, looking for the tells."
    description: "When someone lies to you, use Hustle instead of Perception to detect it."

  # ── SKILL FEATS: SMOOTH TALK ──────────────────────────────────────────────
  - id: assurance_smooth_talk
    name: Steady Smooth Talk
    category: skill
    skill: smooth_talk
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Smooth Talk rolls as 10."

  - id: deal_finder
    name: Deal Finder
    category: skill
    skill: smooth_talk
    pf2e: bargain_hunter
    active: false
    activate_text: ""
    description: "When earning income via Smooth Talk, find discounts and special items for sale."

  - id: crowd_work
    name: Crowd Work
    category: skill
    skill: smooth_talk
    pf2e: group_impression
    active: true
    activate_text: "You play to the whole room at once."
    description: "Make an impression on up to 10 people at once with a single check."

  - id: street_networker
    name: Street Networker
    category: skill
    skill: smooth_talk
    pf2e: hobnobber
    active: true
    activate_text: "You work your contacts and get answers fast."
    description: "Gather information in half the normal time; critical failure still yields partial info."

  # ── SKILL FEATS: HARD LOOK ────────────────────────────────────────────────
  - id: assurance_hard_look
    name: Steady Hard Look
    category: skill
    skill: hard_look
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Hard Look rolls as 10."

  - id: mass_intimidation
    name: Mass Intimidation
    category: skill
    skill: hard_look
    pf2e: group_coercion
    active: true
    activate_text: "You sweep the room with a look that says you mean business."
    description: "Coerce up to 5 people at once with a single check."

  - id: death_stare
    name: Death Stare
    category: skill
    skill: hard_look
    pf2e: intimidating_glare
    active: true
    activate_text: "You fix them with a look that says exactly how this ends."
    description: "Demoralize a target with a look alone; no words required."

  - id: fast_threat
    name: Fast Threat
    category: skill
    skill: hard_look
    pf2e: quick_coercion
    active: true
    activate_text: "You get your point across in one sentence."
    description: "Coerce someone with only one exchange of words rather than a minute."

  # ── SKILL FEATS: REP ──────────────────────────────────────────────────────
  - id: assurance_rep
    name: Steady Rep
    category: skill
    skill: rep
    pf2e: assurance
    active: false
    activate_text: ""
    description: "Treat Rep rolls as 10."

  - id: crowd_control
    name: Crowd Control
    category: skill
    skill: rep
    pf2e: fascinating_performance
    active: true
    activate_text: "You command the room. Everyone watches."
    description: "Perform to hold up to 4 people's attention; –2 Perception vs. others."

  - id: rep_play
    name: Rep Play
    category: skill
    skill: rep
    pf2e: impressive_performance
    active: true
    activate_text: "You turn on the presence and let your reputation do the work."
    description: "Use Rep instead of Smooth Talk to make an impression."

  - id: signature_style
    name: Signature Style
    category: skill
    skill: rep
    pf2e: virtuosic_performer
    active: false
    activate_text: ""
    description: "Choose a Rep specialty (music, street art, combat style); +1 to that type."

  # ── JOB FEATS: AGGRESSOR ──────────────────────────────────────────────────
  - id: brutal_charge
    name: Brutal Charge
    category: job
    archetype: aggressor
    pf2e: sudden_charge
    active: true
    activate_text: "You close the distance before they can react."
    description: "Advance twice and make a melee strike as a single sequence."

  - id: reactive_block
    name: Reactive Block
    category: job
    archetype: aggressor
    pf2e: reactive_shield
    active: true
    activate_text: "You snap your shield up just in time."
    description: "When hit by a physical attack, raise your shield as a reaction to reduce damage."

  - id: overpower
    name: Overpower
    category: job
    archetype: aggressor
    pf2e: vicious_swing
    active: true
    activate_text: "You put everything into it."
    description: "Wind up a two-handed strike dealing two extra damage dice."

  - id: snap_shot
    name: Snap Shot
    category: job
    archetype: aggressor
    pf2e: exacting_strike
    active: true
    activate_text: "You squeeze the trigger the moment you have a bead."
    description: "A missed attack counts as only one hit for multi-attack penalty purposes."

  - id: adrenaline_surge
    name: Adrenaline Surge
    category: job
    archetype: aggressor
    pf2e: moment_of_clarity
    active: true
    activate_text: "The adrenaline cuts through the haze and you think clearly for a moment."
    description: "Once per rage, use a focus action that normally can't be used while enraged."

  - id: raging_threat
    name: Raging Threat
    category: job
    archetype: aggressor
    pf2e: raging_intimidation
    active: false
    activate_text: ""
    description: "Demoralize enemies while enraged; your rage itself counts as a threat display."

  - id: cover_fire
    name: Cover Fire
    category: job
    archetype: aggressor
    pf2e: cover_fire
    active: true
    activate_text: "You lay down fire to keep their heads down."
    description: "Fire at a target to give an ally +2 AC against that target's next ranged attack."

  - id: dual_draw
    name: Dual Draw
    category: job
    archetype: aggressor
    pf2e: dual_weapon_reload
    active: true
    activate_text: "You reload and draw in one smooth motion."
    description: "Reload a one-handed firearm and simultaneously draw a second weapon."

  - id: combat_read
    name: Combat Read
    category: job
    archetype: aggressor
    pf2e: combat_assessment
    active: true
    activate_text: "You study their stance and attack at the same time."
    description: "Strike and simultaneously attempt to read the target's weaknesses on a hit."

  - id: hit_the_dirt
    name: Hit the Dirt
    category: job
    archetype: aggressor
    pf2e: hit_the_dirt
    active: true
    activate_text: "You drop flat the moment you hear the shot."
    description: "Drop prone as a reaction when targeted by ranged fire; gain +2 AC against that attack."

  # ── JOB FEATS: CRIMINAL ───────────────────────────────────────────────────
  - id: quick_dodge
    name: Quick Dodge
    category: job
    archetype: criminal
    pf2e: nimble_dodge
    active: true
    activate_text: "You twist out of the way at the last moment."
    description: "When targeted by an attack, use your reaction to gain +2 AC."

  - id: trap_eye
    name: Trap Eye
    category: job
    archetype: criminal
    pf2e: trap_finder
    active: false
    activate_text: ""
    description: "+1 to spot traps; bonus to AC and saves against traps; passively notice traps."

  - id: twin_strike
    name: Twin Strike
    category: job
    archetype: criminal
    pf2e: twin_feint
    active: true
    activate_text: "You feint high and follow through low."
    description: "Two quick strikes with different weapons; the second automatically catches them off-guard."

  - id: youre_next
    name: You're Next
    category: job
    archetype: criminal
    pf2e: youre_next
    active: true
    activate_text: "You look at the next one and let the moment speak for itself."
    description: "When you drop an enemy to 0 HP, immediately demoralize a nearby enemy as a reaction."

  - id: overextend
    name: Overextend
    category: job
    archetype: criminal
    pf2e: overextending_feint
    active: false
    activate_text: ""
    description: "A successful feint leaves the target off-guard against all your attacks this turn."

  - id: tumble_behind
    name: Tumble Behind
    category: job
    archetype: criminal
    pf2e: tumble_behind
    active: false
    activate_text: ""
    description: "Tumbling through a foe's space leaves them off-guard to your next attack."

  - id: plant_evidence
    name: Plant Evidence
    category: job
    archetype: criminal
    pf2e: plant_evidence
    active: true
    activate_text: "Smooth and clean. They'll never know."
    description: "Plant an item on a target without being detected."

  - id: dueling_guard
    name: Dueling Guard
    category: job
    archetype: criminal
    pf2e: dueling_parry
    active: true
    activate_text: "You settle into your guard and dare them to come at you."
    description: "Use an action to gain +2 AC until next turn when fighting with one melee weapon."

  - id: flying_blade
    name: Flying Blade
    category: job
    archetype: criminal
    pf2e: flying_blade
    active: false
    activate_text: ""
    description: "Thrown melee weapons apply your precision bonus as if you were in melee."

  # ── JOB FEATS: DRIFTER ────────────────────────────────────────────────────
  - id: mark_target
    name: Mark Target
    category: job
    archetype: drifter
    pf2e: hunt_prey
    active: true
    activate_text: "You fix them in your mind and start tracking every detail."
    description: "Designate a target; gain +1 to all checks against that target for 1 day."

  - id: rapid_shot
    name: Rapid Shot
    category: job
    archetype: drifter
    pf2e: hunted_shot
    active: true
    activate_text: "Two shots, one breath."
    description: "Fire two ranged attacks at your marked target as a single action."

  - id: field_study
    name: Field Study
    category: job
    archetype: drifter
    pf2e: monster_hunter
    active: true
    activate_text: "You read them as you mark them."
    description: "As part of marking a target, attempt a Recall Knowledge check about them."

  - id: known_weakness
    name: Known Weakness
    category: job
    archetype: drifter
    pf2e: known_weakness
    active: false
    activate_text: ""
    description: "Successful Recall Knowledge also reveals one weakness; critical success reveals two."

  - id: share_intel
    name: Share Intel
    category: job
    archetype: drifter
    pf2e: clue_them_all_in
    active: true
    activate_text: "You brief the team on what you know."
    description: "Share your Mark Target bonus with up to 5 allies."

  - id: animal_companion_drifter
    name: Animal Companion
    category: job
    archetype: drifter
    pf2e: animal_companion
    active: false
    activate_text: ""
    description: "Gain a young animal companion that assists you in the field."

  # ── JOB FEATS: INFLUENCER ─────────────────────────────────────────────────
  - id: street_knowledge
    name: Street Knowledge
    category: job
    archetype: influencer
    pf2e: bardic_lore
    active: true
    activate_text: "You dig through memory for what you know about this."
    description: "Use your Rep modifier for any Recall Knowledge check regardless of the skill required."

  - id: sustained_presence
    name: Sustained Presence
    category: job
    archetype: influencer
    pf2e: lingering_composition
    active: true
    activate_text: "You keep the energy up, feeding it deliberately."
    description: "Extend a performance or social effect by 1 round on a success, 3 on a critical success."

  - id: crowd_performer
    name: Crowd Performer
    category: job
    archetype: influencer
    pf2e: versatile_performance
    active: true
    activate_text: "You read the crowd and switch registers."
    description: "Use Rep in place of Smooth Talk, Hard Look, or Hustle for certain social actions."

  - id: raw_intensity
    name: Raw Intensity
    category: job
    archetype: influencer
    pf2e: dangerous_sorcery
    active: false
    activate_text: ""
    description: "Damage-dealing social or psychological actions deal bonus damage equal to their intensity rank."

  - id: reach_influence
    name: Reach Influence
    category: job
    archetype: influencer
    pf2e: reach_spell
    active: true
    activate_text: "You project further than they expect."
    description: "Spend 1 extra action to extend the reach or area of a social action."

  # ── JOB FEATS: NERD ───────────────────────────────────────────────────────
  - id: quick_bomb
    name: Quick Bomb
    category: job
    archetype: nerd
    pf2e: quick_bomber
    active: true
    activate_text: "You have it out and armed before the moment is gone."
    description: "Draw an explosive or chemical item and immediately use it as one combined action."

  - id: field_identify
    name: Field Identify
    category: job
    archetype: nerd
    pf2e: alchemical_savant
    active: true
    activate_text: "You turn it over in your hands and know exactly what it is."
    description: "Identify a tech or chemical item you're holding in a single action."

  - id: tamper
    name: Tamper
    category: job
    archetype: nerd
    pf2e: tamper
    active: true
    activate_text: "You make a small, invisible adjustment."
    description: "Sabotage a foe's weapon or armor; –2 to attack or AC until they clear it."

  - id: jury_rig
    name: Jury Rig
    category: job
    archetype: nerd
    pf2e: haphazard_repair
    active: true
    activate_text: "You slap it together and cross your fingers."
    description: "Attempt an emergency repair in combat with a DC 15 Rigging check."

  - id: research_focus
    name: Research Focus
    category: job
    archetype: nerd
    pf2e: spellbook_prodigy
    active: false
    activate_text: ""
    description: "Learn new technical knowledge in half the time; critical successes compound the benefit."

  - id: variable_load
    name: Variable Load
    category: job
    archetype: nerd
    pf2e: variable_core
    active: false
    activate_text: ""
    description: "During daily prep, switch your primary explosive or tech damage type."

  # ── JOB FEATS: NORMIE ─────────────────────────────────────────────────────
  - id: defensive_stance
    name: Defensive Stance
    category: job
    archetype: normie
    pf2e: crane_stance
    active: true
    activate_text: "You drop into a guard that makes you a smaller target."
    description: "Enter a stance granting +1 AC and access to precise, fast unarmed strikes."

  - id: iron_fist
    name: Iron Fist
    category: job
    archetype: normie
    pf2e: dragon_stance
    active: true
    activate_text: "You square up and let your body become the weapon."
    description: "Enter a stance enabling powerful, forceful unarmed strikes (d10 base damage)."

  - id: stunning_strike
    name: Stunning Strike
    category: job
    archetype: normie
    pf2e: stunning_fist
    active: true
    activate_text: "You follow up with a strike aimed at the nerve cluster."
    description: "After a flurry of strikes, if one lands, force a Fortitude save or stun the target."

  - id: dirty_strike
    name: Dirty Strike
    category: job
    archetype: normie
    pf2e: instructive_strike
    active: true
    activate_text: "You hit them and read their reaction at the same time."
    description: "Strike and simultaneously learn one weakness or vulnerability on a hit."

  - id: read_the_room
    name: Read the Room
    category: job
    archetype: normie
    pf2e: diverse_lore
    active: false
    activate_text: ""
    description: "Take a –2 penalty to attempt any Recall Knowledge check outside your area of expertise."

  - id: root_to_life
    name: Root to Life
    category: job
    archetype: normie
    pf2e: root_to_life
    active: true
    activate_text: "You get your hands on them and focus entirely on keeping them alive."
    description: "Stabilize a dying ally in 1 action; 2 actions also clears persistent damage effects."
```

**Step 2: Verify the file parses correctly**

```bash
python3 -c "import yaml; d=yaml.safe_load(open('content/feats.yaml')); print(len(d['feats']), 'feats')"
```

Expected: `112 feats` (or near that number — verify count is reasonable)

**Step 3: Commit**

```bash
git add content/feats.yaml
git commit -m "feat: add content/feats.yaml with all level-1 feat definitions"
```

---

## Task 2: `internal/game/ruleset/feat.go` — Feat Type and Loader

**Files:**
- Create: `internal/game/ruleset/feat.go`
- Create: `internal/game/ruleset/feat_test.go`

**Step 1: Write the failing test**

```go
// internal/game/ruleset/feat_test.go
package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadFeats_ParsesAllFeats(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	if len(feats) == 0 {
		t.Fatal("expected non-empty feats list")
	}
	// Verify a known general feat
	var found bool
	for _, f := range feats {
		if f.ID == "toughness" && f.Category == "general" && !f.Active {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'toughness' general feat")
	}
}

func TestLoadFeats_SkillFeatHasSkillField(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Category == "skill" && f.Skill == "" {
			t.Errorf("skill feat %q has empty Skill field", f.ID)
		}
	}
}

func TestLoadFeats_ActiveFeatHasActivateText(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Active && f.ActivateText == "" {
			t.Errorf("active feat %q has empty ActivateText", f.ID)
		}
	}
}

func TestFeatRegistry_LookupByID(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	f, ok := reg.Feat("toughness")
	if !ok {
		t.Fatal("expected to find toughness in registry")
	}
	if f.Name != "Toughness" {
		t.Errorf("expected Name=Toughness got %q", f.Name)
	}
}

func TestFeatRegistry_ByCategory(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	generals := reg.ByCategory("general")
	if len(generals) == 0 {
		t.Error("expected non-empty general feats")
	}
	for _, f := range generals {
		if f.Category != "general" {
			t.Errorf("ByCategory(general) returned feat with Category=%q", f.Category)
		}
	}
}

func TestFeatRegistry_BySkill(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	parkour := reg.BySkill("parkour")
	if len(parkour) == 0 {
		t.Error("expected parkour skill feats")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestLoadFeats -v 2>&1 | head -20
```

Expected: FAIL with "undefined: ruleset.LoadFeats"

**Step 3: Implement `feat.go`**

```go
// internal/game/ruleset/feat.go
package ruleset

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Feat defines one Gunchete feat and its P2FE equivalent.
//
// Category is one of: "general", "skill", "job".
// Skill is non-empty only for category="skill"; Archetype is non-empty only for category="job".
// Active feats require player action to use; passive feats are always-on.
type Feat struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Category     string `yaml:"category"`
	Skill        string `yaml:"skill"`
	Archetype    string `yaml:"archetype"`
	PF2E         string `yaml:"pf2e"`
	Active       bool   `yaml:"active"`
	ActivateText string `yaml:"activate_text"`
	Description  string `yaml:"description"`
}

// featsFile is the top-level YAML structure for content/feats.yaml.
type featsFile struct {
	Feats []*Feat `yaml:"feats"`
}

// LoadFeats reads the feats master YAML file and returns all feat definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all feats or a non-nil error.
func LoadFeats(path string) ([]*Feat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading feats file %s: %w", path, err)
	}
	var f featsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing feats file %s: %w", path, err)
	}
	return f.Feats, nil
}

// FeatRegistry provides fast lookup of feats by ID, category, skill, and archetype.
type FeatRegistry struct {
	byID        map[string]*Feat
	byCategory  map[string][]*Feat
	bySkill     map[string][]*Feat
	byArchetype map[string][]*Feat
}

// NewFeatRegistry builds a FeatRegistry from the given feat slice.
//
// Precondition: feats must not be nil.
// Postcondition: Returns a fully indexed registry.
func NewFeatRegistry(feats []*Feat) *FeatRegistry {
	r := &FeatRegistry{
		byID:        make(map[string]*Feat, len(feats)),
		byCategory:  make(map[string][]*Feat),
		bySkill:     make(map[string][]*Feat),
		byArchetype: make(map[string][]*Feat),
	}
	for _, f := range feats {
		r.byID[f.ID] = f
		r.byCategory[f.Category] = append(r.byCategory[f.Category], f)
		if f.Skill != "" {
			r.bySkill[f.Skill] = append(r.bySkill[f.Skill], f)
		}
		if f.Archetype != "" {
			r.byArchetype[f.Archetype] = append(r.byArchetype[f.Archetype], f)
		}
	}
	return r
}

// Feat returns the feat with the given ID and true, or nil and false if not found.
func (r *FeatRegistry) Feat(id string) (*Feat, bool) {
	f, ok := r.byID[id]
	return f, ok
}

// ByCategory returns all feats in the given category.
func (r *FeatRegistry) ByCategory(category string) []*Feat {
	return r.byCategory[category]
}

// BySkill returns all skill feats unlocked by the given skill ID.
func (r *FeatRegistry) BySkill(skillID string) []*Feat {
	return r.bySkill[skillID]
}

// ByArchetype returns all job feats for the given archetype.
func (r *FeatRegistry) ByArchetype(archetype string) []*Feat {
	return r.byArchetype[archetype]
}

// SkillFeatsForTrainedSkills returns the union of skill feat pools for all trained skills.
// trained is a map of skill_id → proficiency (only "trained" or better are included).
func (r *FeatRegistry) SkillFeatsForTrainedSkills(trained map[string]string) []*Feat {
	seen := make(map[string]bool)
	var out []*Feat
	for skillID, prof := range trained {
		if prof == "untrained" {
			continue
		}
		for _, f := range r.bySkill[skillID] {
			if !seen[f.ID] {
				seen[f.ID] = true
				out = append(out, f)
			}
		}
	}
	return out
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/game/ruleset/... -run "TestLoadFeats|TestFeatRegistry" -v 2>&1 | tail -15
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/game/ruleset/feat.go internal/game/ruleset/feat_test.go
git commit -m "feat: add Feat type, LoadFeats, and FeatRegistry"
```

---

## Task 3: Extend `Job` Struct with `FeatGrants`

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/job_test.go` (or create if absent)

**Step 1: Write the failing test**

Add to the existing test file (or create `internal/game/ruleset/job_feat_test.go`):

```go
// in package ruleset_test
func TestJob_FeatGrants_LoadsFromYAML(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	if err != nil {
		t.Fatalf("LoadJobs: %v", err)
	}
	var found bool
	for _, j := range jobs {
		if j.FeatGrants != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one job with FeatGrants set")
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/game/ruleset/... -run TestJob_FeatGrants -v 2>&1 | head -10
```

Expected: FAIL (FeatGrants field doesn't exist yet)

**Step 3: Add `FeatGrants` to `job.go`**

In `internal/game/ruleset/job.go`, add after the existing `SkillGrants` type:

```go
// FeatChoices defines a pool of feats the player picks from at creation.
type FeatChoices struct {
	Pool  []string `yaml:"pool"`
	Count int      `yaml:"count"`
}

// FeatGrants defines feat grants from a job at character creation.
// GeneralCount is how many general feats the player freely picks.
// Fixed is always granted. Choices is an optional selection pool.
type FeatGrants struct {
	GeneralCount int          `yaml:"general_count"`
	Fixed        []string     `yaml:"fixed"`
	Choices      *FeatChoices `yaml:"choices"`
}
```

Then add to the `Job` struct after `SkillGrants`:

```go
FeatGrants *FeatGrants `yaml:"feats"`
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/game/ruleset/... -run TestJob_FeatGrants -v 2>&1 | tail -5
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/game/ruleset/job.go internal/game/ruleset/
git commit -m "feat: add FeatGrants to Job struct"
```

---

## Task 4: Update All 76 Job YAMLs with `feats` Blocks

**Files:**
- Modify: all 76 `content/jobs/*.yaml` files

**Step 1: Add the `feats` block to each job YAML**

Add to the end of each job file. The `feats` block goes after the `skills` block.
Archetype assignments from `content/jobs/*.yaml` grep:

**Aggressor jobs** — feat pool: brutal_charge, reactive_block, overpower, snap_shot,
adrenaline_surge, raging_threat, cover_fire, dual_draw, combat_read, hit_the_dirt

```yaml
# beat_down_artist.yaml
feats:
  general_count: 1
  fixed:
    - brutal_charge
  choices:
    pool: [overpower, adrenaline_surge]
    count: 1

# boot_gun.yaml
feats:
  general_count: 1
  fixed:
    - cover_fire
  choices:
    pool: [dual_draw, hit_the_dirt]
    count: 1

# boot_machete.yaml
feats:
  general_count: 1
  fixed:
    - brutal_charge
  choices:
    pool: [reactive_block, overpower]
    count: 1

# contract_killer.yaml
feats:
  general_count: 1
  fixed:
    - snap_shot
  choices:
    pool: [cover_fire, combat_read]
    count: 1

# gangster.yaml
feats:
  general_count: 1
  fixed:
    - raging_threat
  choices:
    pool: [brutal_charge, combat_read]
    count: 1

# grunt.yaml
feats:
  general_count: 1
  fixed:
    - brutal_charge
  choices:
    pool: [reactive_block, cover_fire]
    count: 1

# guard.yaml
feats:
  general_count: 1
  fixed:
    - reactive_block
  choices:
    pool: [cover_fire, combat_read]
    count: 1

# laborer.yaml
feats:
  general_count: 1
  fixed:
    - overpower
  choices:
    pool: [brutal_charge, reactive_block]
    count: 1

# mall_ninja.yaml
feats:
  general_count: 1
  fixed:
    - dual_draw
  choices:
    pool: [cover_fire, snap_shot]
    count: 1

# mercenary.yaml
feats:
  general_count: 1
  fixed:
    - cover_fire
  choices:
    pool: [snap_shot, combat_read]
    count: 1

# pirate.yaml
feats:
  general_count: 1
  fixed:
    - brutal_charge
  choices:
    pool: [dual_draw, overpower]
    count: 1

# roid_rager.yaml
feats:
  general_count: 1
  fixed:
    - adrenaline_surge
  choices:
    pool: [brutal_charge, overpower]
    count: 1

# soldier.yaml
feats:
  general_count: 1
  fixed:
    - cover_fire
  choices:
    pool: [hit_the_dirt, combat_read]
    count: 1

# specialist.yaml
feats:
  general_count: 1
  fixed:
    - snap_shot
  choices:
    pool: [cover_fire, dual_draw]
    count: 1

# street_fighter.yaml
feats:
  general_count: 1
  fixed:
    - brutal_charge
  choices:
    pool: [overpower, adrenaline_surge]
    count: 1

# thug.yaml
feats:
  general_count: 1
  fixed:
    - raging_threat
  choices:
    pool: [brutal_charge, overpower]
    count: 1

# trainee.yaml
feats:
  general_count: 1
  fixed: []
  choices:
    pool: [brutal_charge, reactive_block, cover_fire]
    count: 1
```

**Criminal jobs** — feat pool: quick_dodge, trap_eye, twin_strike, youre_next, overextend,
tumble_behind, plant_evidence, dueling_guard, flying_blade

```yaml
# beggar.yaml
feats:
  general_count: 1
  fixed:
    - plant_evidence
  choices:
    pool: [quick_dodge, tumble_behind]
    count: 1

# car_jacker.yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [tumble_behind, twin_strike]
    count: 1

# free_spirit.yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [plant_evidence, tumble_behind]
    count: 1

# gambler.yaml
feats:
  general_count: 1
  fixed:
    - plant_evidence
  choices:
    pool: [overextend, quick_dodge]
    count: 1

# grifter.yaml
feats:
  general_count: 1
  fixed:
    - overextend
  choices:
    pool: [quick_dodge, plant_evidence]
    count: 1

# hobo.yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [tumble_behind, plant_evidence]
    count: 1

# hooker.yaml
feats:
  general_count: 1
  fixed:
    - overextend
  choices:
    pool: [quick_dodge, flying_blade]
    count: 1

# muscle.yaml
feats:
  general_count: 1
  fixed:
    - twin_strike
  choices:
    pool: [quick_dodge, youre_next]
    count: 1

# smuggler.yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [plant_evidence, tumble_behind]
    count: 1

# thief.yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [plant_evidence, twin_strike]
    count: 1

# tomb_raider.yaml
feats:
  general_count: 1
  fixed:
    - trap_eye
  choices:
    pool: [quick_dodge, tumble_behind]
    count: 1

# vigilante.yaml
feats:
  general_count: 1
  fixed:
    - youre_next
  choices:
    pool: [quick_dodge, twin_strike]
    count: 1
```

**Drifter jobs** — feat pool: mark_target, rapid_shot, field_study, known_weakness,
share_intel, animal_companion_drifter

```yaml
# bagman.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [known_weakness, share_intel]
    count: 1

# cop.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [field_study, known_weakness]
    count: 1

# dealer.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [share_intel, field_study]
    count: 1

# freegan.yaml
feats:
  general_count: 1
  fixed:
    - animal_companion_drifter
  choices:
    pool: [mark_target, field_study]
    count: 1

# illusionist.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [share_intel, field_study]
    count: 1

# psychopath.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [known_weakness, rapid_shot]
    count: 1

# rancher.yaml
feats:
  general_count: 1
  fixed:
    - animal_companion_drifter
  choices:
    pool: [mark_target, field_study]
    count: 1

# scout.yaml
feats:
  general_count: 1
  fixed:
    - rapid_shot
  choices:
    pool: [mark_target, field_study]
    count: 1

# stalker.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [known_weakness, rapid_shot]
    count: 1

# street_preacher.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [share_intel, field_study]
    count: 1

# tracker.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [rapid_shot, animal_companion_drifter]
    count: 1

# warden.yaml
feats:
  general_count: 1
  fixed:
    - mark_target
  choices:
    pool: [rapid_shot, field_study]
    count: 1
```

**Influencer jobs** — feat pool: street_knowledge, sustained_presence, crowd_performer,
raw_intensity, reach_influence

```yaml
# anarchist.yaml
feats:
  general_count: 1
  fixed:
    - street_knowledge
  choices:
    pool: [crowd_performer, sustained_presence]
    count: 1

# antifa.yaml
feats:
  general_count: 1
  fixed:
    - street_knowledge
  choices:
    pool: [crowd_performer, reach_influence]
    count: 1

# bureaucrat.yaml
feats:
  general_count: 1
  fixed:
    - reach_influence
  choices:
    pool: [street_knowledge, crowd_performer]
    count: 1

# entertainer.yaml
feats:
  general_count: 1
  fixed:
    - crowd_performer
  choices:
    pool: [sustained_presence, street_knowledge]
    count: 1

# exotic_dancer.yaml
feats:
  general_count: 1
  fixed:
    - crowd_performer
  choices:
    pool: [raw_intensity, sustained_presence]
    count: 1

# extortionist.yaml
feats:
  general_count: 1
  fixed:
    - raw_intensity
  choices:
    pool: [reach_influence, street_knowledge]
    count: 1

# fallen_trustafarian.yaml
feats:
  general_count: 1
  fixed:
    - street_knowledge
  choices:
    pool: [crowd_performer, sustained_presence]
    count: 1

# follower.yaml
feats:
  general_count: 1
  fixed:
    - street_knowledge
  choices:
    pool: [crowd_performer, reach_influence]
    count: 1

# goon.yaml
feats:
  general_count: 1
  fixed:
    - raw_intensity
  choices:
    pool: [reach_influence, crowd_performer]
    count: 1

# karen.yaml
feats:
  general_count: 1
  fixed:
    - reach_influence
  choices:
    pool: [raw_intensity, crowd_performer]
    count: 1

# libertarian.yaml
feats:
  general_count: 1
  fixed:
    - street_knowledge
  choices:
    pool: [reach_influence, raw_intensity]
    count: 1

# politician.yaml
feats:
  general_count: 1
  fixed:
    - crowd_performer
  choices:
    pool: [sustained_presence, reach_influence]
    count: 1

# schmoozer.yaml
feats:
  general_count: 1
  fixed:
    - crowd_performer
  choices:
    pool: [sustained_presence, street_knowledge]
    count: 1

# shit_stirrer.yaml
feats:
  general_count: 1
  fixed:
    - raw_intensity
  choices:
    pool: [street_knowledge, reach_influence]
    count: 1
```

**Nerd jobs** — feat pool: quick_bomb, field_identify, tamper, jury_rig, research_focus,
variable_load

```yaml
# believer.yaml
feats:
  general_count: 1
  fixed:
    - research_focus
  choices:
    pool: [field_identify, tamper]
    count: 1

# cooker.yaml
feats:
  general_count: 1
  fixed:
    - quick_bomb
  choices:
    pool: [field_identify, variable_load]
    count: 1

# detective.yaml
feats:
  general_count: 1
  fixed:
    - field_identify
  choices:
    pool: [research_focus, tamper]
    count: 1

# engineer.yaml
feats:
  general_count: 1
  fixed:
    - jury_rig
  choices:
    pool: [tamper, research_focus]
    count: 1

# grease_monkey.yaml
feats:
  general_count: 1
  fixed:
    - jury_rig
  choices:
    pool: [quick_bomb, field_identify]
    count: 1

# hippie.yaml
feats:
  general_count: 1
  fixed:
    - research_focus
  choices:
    pool: [field_identify, variable_load]
    count: 1

# hoarder.yaml
feats:
  general_count: 1
  fixed:
    - field_identify
  choices:
    pool: [research_focus, jury_rig]
    count: 1

# journalist.yaml
feats:
  general_count: 1
  fixed:
    - research_focus
  choices:
    pool: [field_identify, tamper]
    count: 1

# narc.yaml
feats:
  general_count: 1
  fixed:
    - field_identify
  choices:
    pool: [research_focus, tamper]
    count: 1

# narcomancer.yaml
feats:
  general_count: 1
  fixed:
    - quick_bomb
  choices:
    pool: [variable_load, field_identify]
    count: 1

# natural_mystic.yaml
feats:
  general_count: 1
  fixed:
    - research_focus
  choices:
    pool: [field_identify, variable_load]
    count: 1

# pastor.yaml
feats:
  general_count: 1
  fixed:
    - research_focus
  choices:
    pool: [field_identify, jury_rig]
    count: 1
```

**Normie jobs** — feat pool: defensive_stance, iron_fist, stunning_strike, dirty_strike,
read_the_room, root_to_life

```yaml
# cult_leader.yaml
feats:
  general_count: 1
  fixed:
    - read_the_room
  choices:
    pool: [dirty_strike, root_to_life]
    count: 1

# driver.yaml
feats:
  general_count: 1
  fixed:
    - defensive_stance
  choices:
    pool: [dirty_strike, read_the_room]
    count: 1

# exterminator.yaml
feats:
  general_count: 1
  fixed:
    - dirty_strike
  choices:
    pool: [defensive_stance, stunning_strike]
    count: 1

# hanger_on.yaml
feats:
  general_count: 1
  fixed:
    - read_the_room
  choices:
    pool: [root_to_life, defensive_stance]
    count: 1

# hired_help.yaml
feats:
  general_count: 1
  fixed:
    - defensive_stance
  choices:
    pool: [dirty_strike, stunning_strike]
    count: 1

# maker.yaml
feats:
  general_count: 1
  fixed:
    - read_the_room
  choices:
    pool: [dirty_strike, defensive_stance]
    count: 1

# medic.yaml
feats:
  general_count: 1
  fixed:
    - root_to_life
  choices:
    pool: [read_the_room, dirty_strike]
    count: 1

# pilot.yaml
feats:
  general_count: 1
  fixed:
    - defensive_stance
  choices:
    pool: [read_the_room, dirty_strike]
    count: 1

# salesman.yaml
feats:
  general_count: 1
  fixed:
    - read_the_room
  choices:
    pool: [root_to_life, dirty_strike]
    count: 1
```

**Step 2: Verify all 76 job files load correctly**

```bash
go test ./internal/game/ruleset/... -run TestJob_FeatGrants -v 2>&1 | tail -5
```

Expected: PASS

**Step 3: Commit**

```bash
git add content/jobs/
git commit -m "feat: add feats blocks to all 76 job YAMLs"
```

---

## Task 5: DB Migration — `character_feats` Table

**Files:**
- Create: `migrations/012_character_feats.up.sql`
- Create: `migrations/012_character_feats.down.sql`

**Step 1: Create the migration files**

`migrations/012_character_feats.up.sql`:
```sql
CREATE TABLE character_feats (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feat_id      TEXT NOT NULL,
    PRIMARY KEY (character_id, feat_id)
);
```

`migrations/012_character_feats.down.sql`:
```sql
DROP TABLE IF EXISTS character_feats;
```

**Step 2: Verify migration file numbers are sequential**

```bash
ls migrations/ | sort
```

Expected: `011_character_skills.up.sql` exists and `012_character_feats.up.sql` is next.

**Step 3: Commit**

```bash
git add migrations/012_character_feats.up.sql migrations/012_character_feats.down.sql
git commit -m "feat: add character_feats migration"
```

---

## Task 6: `CharacterFeatsRepository`

**Files:**
- Create: `internal/storage/postgres/character_feats.go`
- Create: `internal/storage/postgres/character_feats_test.go`

**Step 1: Write the failing tests**

Copy the pattern from `internal/storage/postgres/character_skills_test.go`. The tests require
a real PostgreSQL connection; check `character_skills_test.go` to see how test DB setup works.

```go
// internal/storage/postgres/character_feats_test.go
package postgres_test

import (
	"context"
	"testing"
)

func TestCharacterFeatsRepository_HasFeats_FalseForNew(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	ch := createTestCharacter(t, db)
	repo := NewCharacterFeatsRepository(db)

	has, err := repo.HasFeats(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasFeats: %v", err)
	}
	if has {
		t.Error("expected HasFeats=false for new character")
	}
}

func TestCharacterFeatsRepository_SetAll_And_GetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	ch := createTestCharacter(t, db)
	repo := NewCharacterFeatsRepository(db)

	feats := []string{"toughness", "quick_dodge", "combat_patch"}
	if err := repo.SetAll(ctx, ch.ID, feats); err != nil {
		t.Fatalf("SetAll: %v", err)
	}

	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != len(feats) {
		t.Errorf("expected %d feats, got %d", len(feats), len(got))
	}
	for _, id := range feats {
		var found bool
		for _, g := range got {
			if g == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected feat %q in GetAll result", id)
		}
	}
}

func TestCharacterFeatsRepository_HasFeats_TrueAfterSetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	ch := createTestCharacter(t, db)
	repo := NewCharacterFeatsRepository(db)

	if err := repo.SetAll(ctx, ch.ID, []string{"toughness"}); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	has, err := repo.HasFeats(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasFeats: %v", err)
	}
	if !has {
		t.Error("expected HasFeats=true after SetAll")
	}
}

func TestCharacterFeatsRepository_SetAll_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	ch := createTestCharacter(t, db)
	repo := NewCharacterFeatsRepository(db)

	first := []string{"toughness", "fleet"}
	second := []string{"quick_dodge"}

	if err := repo.SetAll(ctx, ch.ID, first); err != nil {
		t.Fatalf("first SetAll: %v", err)
	}
	if err := repo.SetAll(ctx, ch.ID, second); err != nil {
		t.Fatalf("second SetAll: %v", err)
	}
	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != 1 || got[0] != "quick_dodge" {
		t.Errorf("expected only [quick_dodge] after second SetAll, got %v", got)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/storage/postgres/... -run TestCharacterFeatsRepository -v 2>&1 | head -15
```

Expected: FAIL with "undefined: NewCharacterFeatsRepository"

**Step 3: Implement `character_feats.go`**

```go
// internal/storage/postgres/character_feats.go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterFeatsRepository persists per-character feat lists.
type CharacterFeatsRepository struct {
	db *pgxpool.Pool
}

// NewCharacterFeatsRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterFeatsRepository(db *pgxpool.Pool) *CharacterFeatsRepository {
	return &CharacterFeatsRepository{db: db}
}

// HasFeats reports whether the character has any rows in character_feats.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one feat row exists.
func (r *CharacterFeatsRepository) HasFeats(ctx context.Context, characterID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_feats WHERE character_id = $1`, characterID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("HasFeats: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all feat IDs for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a slice of feat IDs (may be empty).
func (r *CharacterFeatsRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT feat_id FROM character_feats WHERE character_id = $1`, characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAll feats: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning feat row: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetAll writes the complete feat list for a character, replacing any existing rows.
//
// Precondition: characterID > 0; feats must not be nil.
// Postcondition: character_feats rows match feats exactly.
func (r *CharacterFeatsRepository) SetAll(ctx context.Context, characterID int64, feats []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM character_feats WHERE character_id = $1`, characterID,
	); err != nil {
		return fmt.Errorf("deleting old feats: %w", err)
	}

	for _, featID := range feats {
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_feats (character_id, feat_id) VALUES ($1, $2)`,
			characterID, featID,
		); err != nil {
			return fmt.Errorf("inserting feat %s: %w", featID, err)
		}
	}
	return tx.Commit(ctx)
}
```

**Step 4: Run tests**

```bash
go test ./internal/storage/postgres/... -run TestCharacterFeatsRepository -v 2>&1 | tail -10
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/storage/postgres/character_feats.go internal/storage/postgres/character_feats_test.go
git commit -m "feat: add CharacterFeatsRepository (HasFeats, GetAll, SetAll)"
```

---

## Task 7: Character Model + `BuildFeatsFromJob`

**Files:**
- Modify: `internal/game/character/model.go`
- Modify: `internal/game/character/builder.go`
- Create or modify: `internal/game/character/builder_test.go`

**Step 1: Write the failing test**

```go
// in internal/game/character/builder_test.go (add to existing file)
func TestBuildFeatsFromJob_FixedAndChosen(t *testing.T) {
	job := &ruleset.Job{
		FeatGrants: &ruleset.FeatGrants{
			GeneralCount: 1,
			Fixed:        []string{"quick_dodge"},
			Choices:      &ruleset.FeatChoices{Pool: []string{"twin_strike", "trap_eye"}, Count: 1},
		},
	}
	chosen := []string{"twin_strike"}
	generalChosen := []string{"toughness"}
	skillChosen := []string{"combat_patch"}
	got := character.BuildFeatsFromJob(job, chosen, generalChosen, skillChosen)
	want := map[string]bool{
		"quick_dodge":  true,
		"twin_strike":  true,
		"toughness":    true,
		"combat_patch": true,
	}
	if len(got) != len(want) {
		t.Errorf("expected %d feats, got %d: %v", len(want), len(got), got)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected feat %q", f)
		}
	}
}

func TestBuildFeatsFromJob_NilGrants(t *testing.T) {
	job := &ruleset.Job{FeatGrants: nil}
	got := character.BuildFeatsFromJob(job, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty feats for nil FeatGrants, got %v", got)
	}
}
```

**Step 2: Run to verify fails**

```bash
go test ./internal/game/character/... -run TestBuildFeatsFromJob -v 2>&1 | head -10
```

**Step 3: Add `Feats []string` to `model.go`**

In `internal/game/character/model.go`, add after the `Skills` field:

```go
// Feats is the list of feat IDs held by this character.
// Populated after creation or loading from DB.
Feats []string
```

**Step 4: Add `BuildFeatsFromJob` to `builder.go`**

```go
// BuildFeatsFromJob constructs the feat list for a new or backfilled character.
//
// Precondition: job must not be nil. chosen, generalChosen, skillChosen may be nil.
// Postcondition: Returns a slice containing all granted feat IDs (no duplicates).
func BuildFeatsFromJob(job *ruleset.Job, chosen []string, generalChosen []string, skillChosen []string) []string {
	seen := make(map[string]bool)
	var feats []string
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			feats = append(feats, id)
		}
	}
	if job.FeatGrants == nil {
		return feats
	}
	for _, id := range job.FeatGrants.Fixed {
		add(id)
	}
	for _, id := range chosen {
		add(id)
	}
	for _, id := range generalChosen {
		add(id)
	}
	for _, id := range skillChosen {
		add(id)
	}
	return feats
}
```

**Step 5: Run tests**

```bash
go test ./internal/game/character/... -run TestBuildFeatsFromJob -v 2>&1 | tail -5
```

Expected: PASS

**Step 6: Verify all packages compile**

```bash
go build ./... 2>&1
```

**Step 7: Commit**

```bash
git add internal/game/character/model.go internal/game/character/builder.go internal/game/character/builder_test.go
git commit -m "feat: add Feats field to Character and BuildFeatsFromJob helper"
```

---

## Task 8: Character Creation Feat Selection + `ensureFeats` Backfill

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Modify: `internal/frontend/handlers/auth.go`

**Step 1: Read the existing `skillChoiceLoop` (lines ~394–450) and `ensureSkills` (lines ~394+) as reference**

The feat selection follows the exact same pattern but calls three loops in sequence:
job feat choices → general feat choices → skill feat choice.

**Step 2: Add `CharacterFeatsSetter` interface to `auth.go`**

In `internal/frontend/handlers/auth.go`, add after `CharacterSkillsSetter`:

```go
// CharacterFeatsSetter defines feat persistence operations required by AuthHandler.
type CharacterFeatsSetter interface {
	HasFeats(ctx context.Context, characterID int64) (bool, error)
	SetAll(ctx context.Context, characterID int64, feats []string) error
}
```

Add to `AuthHandler` struct:

```go
characterFeats   CharacterFeatsSetter
allFeats         []*ruleset.Feat
featRegistry     *ruleset.FeatRegistry
```

Update `NewAuthHandler` signature and body to accept and store these three new parameters.
Place them after the existing `characterSkills` and `allSkills` parameters:

```go
func NewAuthHandler(
	accounts AccountStore,
	characters CharacterStore,
	regions []*ruleset.Region,
	teams []*ruleset.Team,
	jobs []*ruleset.Job,
	archetypes []*ruleset.Archetype,
	logger *zap.Logger,
	gameServerAddr string,
	telnetCfg config.TelnetConfig,
	allSkills []*ruleset.Skill,
	characterSkills CharacterSkillsSetter,
	allFeats []*ruleset.Feat,
	characterFeats CharacterFeatsSetter,
) *AuthHandler {
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	var featReg *ruleset.FeatRegistry
	if len(allFeats) > 0 {
		featReg = ruleset.NewFeatRegistry(allFeats)
	}
	return &AuthHandler{
		// ... existing fields ...
		allFeats:        allFeats,
		characterFeats:  characterFeats,
		featRegistry:    featReg,
	}
}
```

**Step 3: Add `featChoiceLoop` to `character_flow.go`**

```go
// featChoiceLoop prompts the player to pick `count` feats from pool, one at a time.
// pool is a list of feat IDs; h.featRegistry is consulted for display names and descriptions.
// Returns chosen feat IDs or (nil, nil) if the player cancels.
//
// Precondition: count >= 1; pool must be non-empty and contain valid feat IDs in h.featRegistry.
// Postcondition: Returns exactly count chosen IDs, or (nil, nil) on cancel.
func (h *AuthHandler) featChoiceLoop(ctx context.Context, conn *telnet.Conn, header string, pool []string, count int) ([]string, error) {
	remaining := make([]string, len(pool))
	copy(remaining, pool)

	var chosen []string
	for len(chosen) < count && len(remaining) > 0 {
		left := count - len(chosen)
		_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "\r\n%s (%d remaining):", header, left))
		for i, id := range remaining {
			name := id
			desc := ""
			if h.featRegistry != nil {
				if f, ok := h.featRegistry.Feat(id); ok {
					name = f.Name
					desc = f.Description
				}
			}
			if desc != "" {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%-20s%s - %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset,
					telnet.Dim, desc, telnet.Reset))
			} else {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset))
			}
		}
		_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select feat [1-%d]: ", len(remaining)))
		line, err := conn.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("reading feat choice: %w", err)
		}
		line = strings.TrimSpace(line)
		if strings.ToLower(line) == "cancel" {
			return nil, nil
		}
		choice := 0
		if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(remaining) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection. Please enter a number from the list."))
			continue
		}
		picked := remaining[choice-1]
		chosen = append(chosen, picked)
		remaining = append(remaining[:choice-1], remaining[choice:]...)
		if h.featRegistry != nil {
			if f, ok := h.featRegistry.Feat(picked); ok {
				_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Selected: %s", f.Name))
			}
		}
	}
	return chosen, nil
}
```

**Step 4: Add `ensureFeats` to `character_flow.go`**

```go
// ensureFeats checks whether the character has feats recorded and, if not,
// runs interactive selection and persists the result. Called before gameBridge.
//
// Precondition: char must have a valid ID, Class, and Skills populated.
// Postcondition: character_feats rows exist for char; returns non-nil error only on fatal failure.
func (h *AuthHandler) ensureFeats(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
	if h.characterFeats == nil || h.featRegistry == nil {
		return nil
	}
	has, err := h.characterFeats.HasFeats(ctx, char.ID)
	if err != nil {
		h.logger.Warn("checking feats for character", zap.Int64("id", char.ID), zap.Error(err))
		return nil
	}
	if has {
		return nil
	}

	job, ok := h.jobRegistry.Job(char.Class)
	if !ok {
		h.logger.Warn("unknown job for feat backfill", zap.String("class", char.Class))
		return nil
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
		"\r\n=== Step 2: Feats ==="))

	// Announce fixed job feats.
	var fixedNames []string
	if job.FeatGrants != nil {
		for _, id := range job.FeatGrants.Fixed {
			if f, ok := h.featRegistry.Feat(id); ok {
				fixedNames = append(fixedNames, f.Name)
			} else {
				fixedNames = append(fixedNames, id)
			}
		}
	}
	if len(fixedNames) > 0 {
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Your job grants you the following feats:"))
		for _, n := range fixedNames {
			_ = conn.WriteLine(fmt.Sprintf("  %s- %s%s", telnet.BrightWhite, n, telnet.Reset))
		}
	}

	// Job feat choices.
	var jobChosen []string
	if job.FeatGrants != nil && job.FeatGrants.Choices != nil && job.FeatGrants.Choices.Count > 0 {
		jobChosen, err = h.featChoiceLoop(ctx, conn, "Choose a job feat", job.FeatGrants.Choices.Pool, job.FeatGrants.Choices.Count)
		if err != nil {
			return fmt.Errorf("feat job choice: %w", err)
		}
	}

	// General feat choices.
	var generalChosen []string
	if job.FeatGrants != nil && job.FeatGrants.GeneralCount > 0 {
		generalPool := h.featRegistry.ByCategory("general")
		poolIDs := make([]string, len(generalPool))
		for i, f := range generalPool {
			poolIDs[i] = f.ID
		}
		generalChosen, err = h.featChoiceLoop(ctx, conn, "Choose a general feat", poolIDs, job.FeatGrants.GeneralCount)
		if err != nil {
			return fmt.Errorf("feat general choice: %w", err)
		}
	}

	// Skill feat choice — 1 pick from trained skills' feat pools.
	var skillChosen []string
	if len(char.Skills) > 0 {
		skillFeatPool := h.featRegistry.SkillFeatsForTrainedSkills(char.Skills)
		if len(skillFeatPool) > 0 {
			poolIDs := make([]string, len(skillFeatPool))
			for i, f := range skillFeatPool {
				poolIDs[i] = f.ID
			}
			skillChosen, err = h.featChoiceLoop(ctx, conn, "Choose a skill feat", poolIDs, 1)
			if err != nil {
				return fmt.Errorf("feat skill choice: %w", err)
			}
		}
	}

	feats := character.BuildFeatsFromJob(job, jobChosen, generalChosen, skillChosen)
	if err := h.characterFeats.SetAll(ctx, char.ID, feats); err != nil {
		h.logger.Error("persisting backfill feats", zap.Int64("id", char.ID), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Yellow, "Warning: feats could not be saved."))
	}
	return nil
}
```

**Step 5: Wire feat selection into character creation**

In `buildAndConfirm` (the function called after job selection), after the existing skill
selection block (around line 526), add the feat selection:

```go
// Feat selection: only when feats and feat storage are configured.
if h.characterFeats != nil && h.featRegistry != nil {
	if err := h.ensureFeats(ctx, conn, created); err != nil {
		h.logger.Error("feat selection", zap.String("name", created.Name), zap.Error(err))
	}
}
```

Note: `ensureFeats` checks `HasFeats` so it will run selection for a newly created character
(which has no feat rows yet) and skip for a character that already has feats.

**Step 6: Wire `ensureFeats` at the three `gameBridge` call sites**

In `characterFlow`, after each `ensureSkills` call, add:

```go
if err := h.ensureFeats(ctx, conn, c); err != nil {
    return err
}
```

All three sites: after `c` is created fresh, after new char from list, and after existing
char `selected` is chosen.

**Step 7: Verify all packages compile**

```bash
go build ./... 2>&1
```

**Step 8: Commit**

```bash
git add internal/frontend/handlers/auth.go internal/frontend/handlers/character_flow.go
git commit -m "feat: add feat selection in character creation and ensureFeats backfill"
```

---

## Task 9: `feats` Command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/feats.go`
- Create: `internal/game/command/feats_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`

### CMD-1 and CMD-2: Register the command

In `internal/game/command/commands.go`:

Add constant:
```go
HandlerFeats = "feats"
```

Add to `BuiltinCommands()`:
```go
{Name: "feats", Aliases: []string{"ft"}, Help: "Display your feats.", Category: CategoryWorld, Handler: HandlerFeats},
```

### CMD-3: Handler function

```go
// internal/game/command/feats.go
package command

// HandleFeats returns the feats command client acknowledgement.
// The actual feat data is returned by the server in a FeatsResponse.
//
// Postcondition: Returns a non-empty string.
func HandleFeats() string {
	return "Reviewing your feats..."
}
```

Test:
```go
// internal/game/command/feats_test.go
package command_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleFeats_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleFeats()
	if got == "" {
		t.Error("HandleFeats() returned empty string")
	}
}
```

### CMD-4: Proto messages

In `api/proto/game/v1/game.proto`:

Add messages (before the closing of the file):
```protobuf
message FeatsRequest {}

message FeatEntry {
  string feat_id       = 1;
  string name          = 2;
  string category      = 3;
  bool   active        = 4;
  string description   = 5;
  string activate_text = 6;
}

message FeatsResponse {
  repeated FeatEntry feats = 1;
}
```

In `ClientMessage` oneof, add after `skills_request = 38`:
```protobuf
FeatsRequest feats_request = 39;
```

In `ServerEvent` oneof, add after `skills_response = 19`:
```protobuf
FeatsResponse feats_response = 20;
```

Run proto regeneration:
```bash
make proto
```

### CMD-5: Bridge handler

In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:
```go
command.HandlerFeats: bridgeFeats,
```

Add function:
```go
// bridgeFeats builds a FeatsRequest to retrieve feat list.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a FeatsRequest; done is false.
func bridgeFeats(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_FeatsRequest{FeatsRequest: &gamev1.FeatsRequest{}},
	}}, nil
}
```

Verify test passes:
```bash
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1 | tail -5
```

### CMD-6: gRPC server handler

In `internal/gameserver/grpc_service.go`, add fields to `GameServiceServer`:
```go
allFeats         []*ruleset.Feat
featRegistry     *ruleset.FeatRegistry
characterFeatsRepo interface {
    GetAll(ctx context.Context, characterID int64) ([]string, error)
}
```

Add handler:
```go
func (s *GameServiceServer) handleFeats(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.characterFeatsRepo == nil || s.featRegistry == nil {
		return messageEvent("Feat data is not available."), nil
	}
	featIDs, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("getting feats for %s: %w", uid, err)
	}
	heldFeats := make(map[string]bool, len(featIDs))
	for _, id := range featIDs {
		heldFeats[id] = true
	}

	var entries []*gamev1.FeatEntry
	for _, id := range featIDs {
		f, ok := s.featRegistry.Feat(id)
		if !ok {
			continue
		}
		entries = append(entries, &gamev1.FeatEntry{
			FeatId:       f.ID,
			Name:         f.Name,
			Category:     f.Category,
			Active:       f.Active,
			Description:  f.Description,
			ActivateText: f.ActivateText,
		})
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_FeatsResponse{
			FeatsResponse: &gamev1.FeatsResponse{Feats: entries},
		},
	}, nil
}
```

Wire into dispatch switch:
```go
case *gamev1.ClientMessage_FeatsRequest:
    return s.handleFeats(uid)
```

### Renderer and game bridge

In `internal/frontend/handlers/text_renderer.go`:

```go
// RenderFeatsResponse formats a FeatsResponse as colored telnet text.
// Feats are grouped by category. Active feats are marked with [active].
//
// Precondition: fr must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderFeatsResponse(fr *gamev1.FeatsResponse) string {
	categoryOrder := []string{"general", "skill", "job"}
	categoryLabel := map[string]string{
		"general": "General",
		"skill":   "Skill",
		"job":     "Job",
	}

	byCategory := make(map[string][]*gamev1.FeatEntry)
	for _, f := range fr.Feats {
		byCategory[f.Category] = append(byCategory[f.Category], f)
	}

	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "=== Feats ==="))
	sb.WriteString("\r\n")

	for _, cat := range categoryOrder {
		feats, ok := byCategory[cat]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("\r\n%s:\r\n", categoryLabel[cat]))
		for _, f := range feats {
			activeTag := ""
			if f.Active {
				activeTag = telnet.Colorize(telnet.BrightYellow, " [active]")
			}
			name := fmt.Sprintf("  %-20s", f.Name)
			sb.WriteString(telnet.Colorize(telnet.Cyan, name))
			sb.WriteString(activeTag)
			sb.WriteString(telnet.Colorize(telnet.Dim, " "+f.Description))
			sb.WriteString("\r\n")
		}
	}
	return sb.String()
}
```

In `internal/frontend/handlers/game_bridge.go`, add to the ServerEvent switch:
```go
case *gamev1.ServerEvent_FeatsResponse:
    text = RenderFeatsResponse(p.FeatsResponse)
```

### CMD-7: Verify end-to-end

```bash
go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Commit:**

```bash
git add api/proto/game/v1/game.proto internal/game/command/commands.go \
    internal/game/command/feats.go internal/game/command/feats_test.go \
    internal/frontend/handlers/bridge_handlers.go internal/gameserver/grpc_service.go \
    internal/frontend/handlers/text_renderer.go internal/frontend/handlers/game_bridge.go
git commit -m "feat: feats command end-to-end (CMD-1 through CMD-7)"
```

---

## Task 10: `interact` Command — Replaces Old `use` for Room Equipment

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/interact.go`
- Create: `internal/game/command/interact_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`

**Context:** The existing `use` command maps to `HandlerUseEquipment` and sends
`UseEquipmentRequest`. We need to create `interact` that does the same thing. The old
`use` command entry in `BuiltinCommands` will be repurposed in Task 11.

### CMD-1 and CMD-2

In `commands.go`, add constant:
```go
HandlerInteract = "interact"
```

Add to `BuiltinCommands()`:
```go
{Name: "interact", Aliases: []string{"int"}, Help: "Interact with an item in the room.", Category: CategoryWorld, Handler: HandlerInteract},
```

### CMD-3

```go
// internal/game/command/interact.go
package command

// HandleInteract returns the interact command client acknowledgement.
// The actual interaction is processed by the server.
//
// Postcondition: Returns a non-empty string.
func HandleInteract() string {
	return "Interacting..."
}
```

Test:
```go
// internal/game/command/interact_test.go
package command_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleInteract_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleInteract()
	if got == "" {
		t.Error("HandleInteract() returned empty string")
	}
}
```

### CMD-4: Proto

Add to `api/proto/game/v1/game.proto`:
```protobuf
message InteractRequest {
  string instance_id = 1;
}

message InteractResponse {
  string message = 1;
}
```

In `ClientMessage` oneof:
```protobuf
InteractRequest interact_request = 40;
```

In `ServerEvent` oneof:
```protobuf
InteractResponse interact_response = 21;
```

Run:
```bash
make proto
```

### CMD-5: Bridge handler

In `bridge_handlers.go`, add to map:
```go
command.HandlerInteract: bridgeInteract,
```

Add function (identical logic to existing `bridgeUseEquipment` but sends `InteractRequest`):
```go
// bridgeInteract builds an InteractRequest for the named item instance ID.
//
// Precondition: bctx must be non-nil with a valid reqID and Args.
// Postcondition: returns a non-nil msg containing an InteractRequest; done is false.
func bridgeInteract(bctx *bridgeContext) (bridgeResult, error) {
	instanceID := strings.TrimSpace(bctx.Args)
	if instanceID == "" {
		return bridgeResult{text: "Usage: interact <item>"}, nil
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_InteractRequest{InteractRequest: &gamev1.InteractRequest{InstanceId: instanceID}},
	}}, nil
}
```

### CMD-6 and CMD-7

In `grpc_service.go`, add handler that delegates to the same logic as `handleUseEquipment`:
```go
func (s *GameServiceServer) handleInteract(uid, instanceID string) (*gamev1.ServerEvent, error) {
	// Delegate to the existing use-equipment logic.
	return s.handleUseEquipment(uid, instanceID)
}
```

Wire into dispatch:
```go
case *gamev1.ClientMessage_InteractRequest:
    return s.handleInteract(uid, p.InteractRequest.InstanceId)
```

In `text_renderer.go`:
```go
// RenderInteractResponse formats an InteractResponse as telnet text.
func RenderInteractResponse(ir *gamev1.InteractResponse) string {
	return ir.Message
}
```

In `game_bridge.go`, add to switch:
```go
case *gamev1.ServerEvent_InteractResponse:
    text = RenderInteractResponse(p.InteractResponse)
```

**Commit:**

```bash
git add api/proto/ internal/game/command/interact.go internal/game/command/interact_test.go \
    internal/game/command/commands.go internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go \
    internal/frontend/handlers/game_bridge.go
git commit -m "feat: interact command end-to-end (replaces use for room equipment)"
```

---

## Task 11: Repurpose `use` Command for Feat Activation

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/use.go`
- Create: `internal/game/command/use_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`

### CMD-1 and CMD-2

In `commands.go`:

**Change** the existing `use` entry from `HandlerUseEquipment` to a new `HandlerUse` constant:
```go
HandlerUse = "use"
```

Update the existing `BuiltinCommands()` entry for `"use"`:
```go
{Name: "use", Aliases: nil, Help: "Activate an active feat. With no argument, shows a list.", Category: CategoryWorld, Handler: HandlerUse},
```

The `HandlerUseEquipment` constant and its mapping in `bridgeHandlerMap` stay unchanged —
it is no longer reachable via `use` but may be used internally. The bridge handler for
`HandlerUseEquipment` stays registered in case it is needed by other callers.

### CMD-3

```go
// internal/game/command/use.go
package command

// HandleUse returns the use command client acknowledgement.
// The actual feat activation is processed by the server.
//
// Postcondition: Returns a non-empty string.
func HandleUse() string {
	return "Activating..."
}
```

Test:
```go
// internal/game/command/use_test.go
package command_test

import (
	"testing"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleUse_ReturnsNonEmpty(t *testing.T) {
	got := command.HandleUse()
	if got == "" {
		t.Error("HandleUse() returned empty string")
	}
}
```

### CMD-4: Proto

Add to `api/proto/game/v1/game.proto`:
```protobuf
message UseRequest {
  string feat_id = 1;  // empty = list active feats
}

message UseResponse {
  string message             = 1;
  repeated FeatEntry choices = 2;  // non-empty when feat_id was empty
}
```

In `ClientMessage` oneof:
```protobuf
UseRequest use_request = 41;
```

In `ServerEvent` oneof:
```protobuf
UseResponse use_response = 22;
```

Run:
```bash
make proto
```

### CMD-5: Bridge handler

In `bridge_handlers.go`, update the map entry for `HandlerUse`:
```go
command.HandlerUse: bridgeUse,
```

Add function:
```go
// bridgeUse builds a UseRequest for feat activation.
// If no feat name is given, sends an empty feat_id to trigger listing.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing a UseRequest.
func bridgeUse(bctx *bridgeContext) (bridgeResult, error) {
	featID := strings.TrimSpace(bctx.Args)
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_UseRequest{UseRequest: &gamev1.UseRequest{FeatId: featID}},
	}}, nil
}
```

### CMD-6: gRPC server handler

```go
func (s *GameServiceServer) handleUse(uid, featID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	if s.characterFeatsRepo == nil || s.featRegistry == nil {
		return messageEvent("Feat data is not available."), nil
	}
	featIDs, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
	if err != nil {
		return nil, fmt.Errorf("getting feats for use: %w", err)
	}

	// Collect active feats this character holds.
	var active []*ruleset.Feat
	for _, id := range featIDs {
		f, ok := s.featRegistry.Feat(id)
		if ok && f.Active {
			active = append(active, f)
		}
	}

	if featID == "" {
		// Return list of active feats for the client to prompt selection.
		entries := make([]*gamev1.FeatEntry, len(active))
		for i, f := range active {
			entries[i] = &gamev1.FeatEntry{
				FeatId: f.ID, Name: f.Name, Category: f.Category,
				Active: f.Active, Description: f.Description, ActivateText: f.ActivateText,
			}
		}
		return &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_UseResponse{
				UseResponse: &gamev1.UseResponse{Choices: entries},
			},
		}, nil
	}

	// Activate the named feat.
	for _, f := range active {
		if strings.EqualFold(f.ID, featID) || strings.EqualFold(f.Name, featID) {
			return &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_UseResponse{
					UseResponse: &gamev1.UseResponse{Message: f.ActivateText},
				},
			}, nil
		}
	}
	return messageEvent(fmt.Sprintf("You don't have an active feat named %q.", featID)), nil
}
```

Wire into dispatch:
```go
case *gamev1.ClientMessage_UseRequest:
    return s.handleUse(uid, p.UseRequest.FeatId)
```

### Renderer

In `text_renderer.go`:
```go
// RenderUseResponse formats a UseResponse as telnet text.
// If Choices is non-empty, renders an interactive selection list.
// Otherwise renders the activation message.
//
// Precondition: ur must be non-nil.
func RenderUseResponse(ur *gamev1.UseResponse) string {
	if ur.Message != "" {
		return ur.Message
	}
	if len(ur.Choices) == 0 {
		return telnet.Colorize(telnet.Yellow, "You have no active feats.")
	}
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "Active feats:\r\n"))
	for i, f := range ur.Choices {
		sb.WriteString(fmt.Sprintf("  %s%d%s. %s%-20s%s - %s%s%s\r\n",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, f.Name, telnet.Reset,
			telnet.Dim, f.Description, telnet.Reset))
	}
	sb.WriteString(telnet.Colorf(telnet.BrightWhite, "Type: use <feat name> to activate"))
	return sb.String()
}
```

In `game_bridge.go`:
```go
case *gamev1.ServerEvent_UseResponse:
    text = RenderUseResponse(p.UseResponse)
```

**Commit:**

```bash
git add api/proto/ internal/game/command/use.go internal/game/command/use_test.go \
    internal/game/command/commands.go internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go \
    internal/frontend/handlers/game_bridge.go
git commit -m "feat: use command repurposed for feat activation (CMD-1 through CMD-7)"
```

---

## Task 12: Wire Feats into `main.go` and Dockerfiles

**Files:**
- Modify: `cmd/frontend/main.go`
- Modify: `cmd/gameserver/main.go`
- Modify: `deployments/docker/Dockerfile.frontend`
- Modify: `deployments/docker/Dockerfile.gameserver`

### `cmd/frontend/main.go`

1. Add flag (after the existing `-skills` flag):
```go
featsPath := flag.String("feats", "content/feats.yaml", "path to feats.yaml")
```

2. Load feats after loading skills:
```go
allFeats, err := ruleset.LoadFeats(*featsPath)
if err != nil {
    log.Fatalf("loading feats: %v", err)
}
```

3. Create `CharacterFeatsRepository`:
```go
characterFeatsRepo := postgres.NewCharacterFeatsRepository(db)
```

4. Pass `allFeats` and `characterFeatsRepo` to `NewAuthHandler` (the two new parameters
added in Task 8).

### `cmd/gameserver/main.go`

1. Add flag:
```go
featsPath := flag.String("feats", "content/feats.yaml", "path to feats.yaml")
```

2. Load feats:
```go
allFeats, err := ruleset.LoadFeats(*featsPath)
if err != nil {
    log.Fatalf("loading feats: %v", err)
}
featRegistry := ruleset.NewFeatRegistry(allFeats)
```

3. Create `CharacterFeatsRepository`:
```go
characterFeatsRepo := postgres.NewCharacterFeatsRepository(db)
```

4. Pass to `GameServiceServer` constructor (add the new fields to its initialization).

### Dockerfiles

In `deployments/docker/Dockerfile.frontend`, append to the CMD array:
```
"-feats", "/content/feats.yaml"
```

In `deployments/docker/Dockerfile.gameserver`, append to the CMD array:
```
"-feats", "/content/feats.yaml"
```

**Step: Verify everything compiles**

```bash
go build ./... 2>&1
go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Commit:**

```bash
git add cmd/frontend/main.go cmd/gameserver/main.go \
    deployments/docker/Dockerfile.frontend deployments/docker/Dockerfile.gameserver
git commit -m "feat: wire feats into main.go flags and Dockerfiles"
```

---

## Task 13: Deploy

**Step 1: Build and push images**

```bash
make docker-push 2>&1 | tail -10
```

**Step 2: Update k8s deployments**

```bash
TAG=$(git rev-parse --short HEAD)
kubectl set image deployment/frontend frontend=registry.johannsen.cloud:5000/mud-frontend:$TAG -n mud
kubectl set image deployment/gameserver gameserver=registry.johannsen.cloud:5000/mud-gameserver:$TAG -n mud
kubectl rollout status deployment/frontend deployment/gameserver -n mud --timeout=90s
```

**Step 3: Verify migration applied**

```bash
kubectl logs -n mud -l app=migrate --tail=20 2>/dev/null || \
kubectl get pods -n mud | grep migrate
```

If the migrate job hasn't run (because the tag changed but the job spec didn't), delete the
old completed job first:
```bash
kubectl delete job mud-migrate -n mud 2>/dev/null; true
helm upgrade mud deployments/k8s/mud \
    --values deployments/k8s/mud/values-prod.yaml \
    --set db.user=mud \
    --set db.password=mud \
    --set image.tag=$TAG 2>&1 | tail -5
```

If helm fails with a conflict error (known issue with kubectl-set conflict), use:
```bash
kubectl set image deployment/frontend frontend=registry.johannsen.cloud:5000/mud-frontend:$TAG -n mud
kubectl set image deployment/gameserver gameserver=registry.johannsen.cloud:5000/mud-gameserver:$TAG -n mud
```

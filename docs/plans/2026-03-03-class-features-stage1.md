# Class Features Stage 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement P2FE level-1 class features for all 76 Gunchete jobs — 12 archetype-shared + 76 job-specific = 88 total features, all fixed grants with no player selection.

**Architecture:** Mirrors the feats pipeline exactly: YAML content → Go types + registry → DB migration + repository → character model + builder → `ensureClassFeatures` (no selection) → `class_features` command → extend `use` command → wire into main/Dockerfiles.

**Tech Stack:** Go 1.26, pgx/v5, protobuf/gRPC, YAML, PostgreSQL

---

## Context

- Proto: `ClientMessage` oneof next field = **42**, `ServerEvent` oneof next field = **23**
- Last DB migration: `012_character_feats` → next is **013**
- `NewAuthHandler` ends with params: `allFeats []*ruleset.Feat, characterFeats CharacterFeatsSetter`
- `NewGameServiceServer` ends with params: `allFeats []*ruleset.Feat, featRegistry *ruleset.FeatRegistry, characterFeatsRepo *postgres.CharacterFeatsRepository`
- All 76 job YAMLs have legacy `features:` blocks (from data import) that must be **removed** and replaced with `class_features:` blocks
- Job archetypes for class feature assignment (confirmed from content/jobs/):
  - **aggressor**: soldier, enforcer, brawler, thug, heavy, berserker, pit_fighter, cage_fighter, street_fighter, headhunter, bouncer, warmonger
  - **criminal**: thief, pickpocket, smuggler, fence, forger, con_artist, black_market_dealer, safecracker, grafter, grifter, extortionist, money_launderer
  - **drifter**: scout, tracker, hunter, trapper, scavenger, nomad, wanderer, wasteland_guide, outrider, pathfinder, ghost, survivalist
  - **influencer**: fixer, negotiator, face, social_engineer, propagandist, cult_leader, rabble_rouser, demagogue, spin_doctor, talent_agent, brand_ambassador, influencer
  - **nerd**: hacker, programmer, engineer, medic, scientist, chemist, pharmacist, weaponsmith, armorsmith, gadgeteer, cryptographer, data_broker
  - **normie**: laborer, driver, cook, mechanic, janitor, courier, dockworker, farmer, street_vendor, bartender, security_guard, beat_down_artist

---

### Task 1: `content/class_features.yaml`

**Files:**
- Create: `content/class_features.yaml`

**Step 1: Write the file**

```yaml
class_features:
  # ── Archetype-Shared Features (12 total — 2 per archetype) ──────────────────

  - id: street_brawler
    name: Street Brawler
    archetype: aggressor
    job: ""
    pf2e: attack_of_opportunity
    active: false
    activate_text: ""
    description: "You can make an Attack of Opportunity against foes that move through your reach or provoke."

  - id: brutal_surge
    name: Brutal Surge
    archetype: aggressor
    job: ""
    pf2e: rage
    active: true
    activate_text: "The red haze drops and you move on pure instinct."
    description: "Enter a combat frenzy: +2 melee damage, -2 AC until end of encounter."

  - id: sucker_punch
    name: Sucker Punch
    archetype: criminal
    job: ""
    pf2e: sneak_attack
    active: false
    activate_text: ""
    description: "Deal +1d6 bonus damage to flat-footed or flanked targets."

  - id: slippery
    name: Slippery
    archetype: criminal
    job: ""
    pf2e: evasion
    active: false
    activate_text: ""
    description: "When you succeed at a Reflex save you gain a critical success instead."

  - id: predators_eye
    name: Predator's Eye
    archetype: drifter
    job: ""
    pf2e: hunters_edge
    active: false
    activate_text: ""
    description: "Choose a favored target type; you deal +1d8 precision damage to that type."

  - id: zone_awareness
    name: Zone Awareness
    archetype: drifter
    job: ""
    pf2e: wild_stride
    active: false
    activate_text: ""
    description: "Difficult terrain from natural hazards does not slow your movement."

  - id: command_attention
    name: Command Attention
    archetype: influencer
    job: ""
    pf2e: inspire_courage
    active: true
    activate_text: "You raise your voice and all eyes snap to you."
    description: "Grant allies within earshot +1 attack and damage rolls for the current encounter."

  - id: fast_talk
    name: Fast Talk
    archetype: influencer
    job: ""
    pf2e: inspire_defense
    active: true
    activate_text: "You spin a rapid web of distracting chatter."
    description: "Grant allies within earshot +1 AC and saving throws for the current encounter."

  - id: daily_prep
    name: Daily Prep
    archetype: nerd
    job: ""
    pf2e: daily_infusions
    active: false
    activate_text: ""
    description: "Each day you may prepare a number of crafted consumables equal to your Intelligence modifier."

  - id: formulaic_mind
    name: Formulaic Mind
    archetype: nerd
    job: ""
    pf2e: formula_book
    active: false
    activate_text: ""
    description: "You maintain a personal formula database; you never fumble on crafting attempts for known formulas."

  - id: street_tough
    name: Street Tough
    archetype: normie
    job: ""
    pf2e: flurry_of_blows
    active: true
    activate_text: "You throw a rapid flurry of blows."
    description: "Make two unarmed strikes as a single action with only a -1 penalty to each."

  - id: resilience
    name: Resilience
    archetype: normie
    job: ""
    pf2e: incredible_movement
    active: false
    activate_text: ""
    description: "+10 ft to your base movement speed."

  # ── Job-Specific Features (76 total — 1 per job) ────────────────────────────

  # aggressor jobs
  - id: guerilla_warfare
    name: Guerilla Warfare
    archetype: ""
    job: soldier
    pf2e: cover_fire
    active: false
    activate_text: ""
    description: "+2 attack bonus when you have cover in urban terrain."

  - id: crowd_control
    name: Crowd Control
    archetype: ""
    job: enforcer
    pf2e: intimidating_strike
    active: true
    activate_text: "You slam your weapon down with intimidating force."
    description: "On a hit, the target is frightened 1."

  - id: iron_chin
    name: Iron Chin
    archetype: ""
    job: brawler
    pf2e: toughness
    active: false
    activate_text: ""
    description: "+3 max HP; recover additional HP equal to your level when you rest."

  - id: pack_mentality
    name: Pack Mentality
    archetype: ""
    job: thug
    pf2e: gang_up
    active: false
    activate_text: ""
    description: "You are never flat-footed when outnumbered 2-to-1 or less."

  - id: suppressing_fire
    name: Suppressing Fire
    archetype: ""
    job: heavy
    pf2e: suppressive_fire
    active: true
    activate_text: "You lay down a wall of fire, pinning enemies in place."
    description: "All enemies in a 30 ft cone must succeed a Reflex save or be immobilized until your next turn."

  - id: blood_frenzy
    name: Blood Frenzy
    archetype: ""
    job: berserker
    pf2e: raging_strikes
    active: false
    activate_text: ""
    description: "While in Brutal Surge, add your level to melee damage instead of +2."

  - id: dirty_fighter
    name: Dirty Fighter
    archetype: ""
    job: pit_fighter
    pf2e: knockdown
    active: true
    activate_text: "You sweep your opponent's legs."
    description: "On a successful melee attack, attempt to Trip as a free action."

  - id: no_tap_out
    name: No Tap Out
    archetype: ""
    job: cage_fighter
    pf2e: juggernaut
    active: false
    activate_text: ""
    description: "Critically succeed on Fortitude saves whenever you would normally succeed."

  - id: haymaker
    name: Haymaker
    archetype: ""
    job: street_fighter
    pf2e: power_attack
    active: true
    activate_text: "You wind up for a devastating overhand strike."
    description: "Make one melee strike at -1 to deal double damage dice."

  - id: trophy_hunter
    name: Trophy Hunter
    archetype: ""
    job: headhunter
    pf2e: hunters_target
    active: true
    activate_text: "You lock eyes on your mark and nothing else matters."
    description: "Designate one target; all your attacks against it gain +2 until it is defeated."

  - id: unmovable
    name: Unmovable
    archetype: ""
    job: bouncer
    pf2e: stand_still
    active: false
    activate_text: ""
    description: "You can make an Attack of Opportunity against foes that attempt to pass you."

  - id: battle_cry
    name: Battle Cry
    archetype: ""
    job: warmonger
    pf2e: rallying_charge
    active: true
    activate_text: "You let out a battle cry that sends adrenaline through your allies."
    description: "All allied creatures within 30 ft gain +1 to attack rolls until the end of your next turn."

  # criminal jobs
  - id: five_finger_discount
    name: Five-Finger Discount
    archetype: ""
    job: thief
    pf2e: theft
    active: false
    activate_text: ""
    description: "After a successful Steal attempt, you may immediately palm a second small item from the same target."

  - id: light_touch
    name: Light Touch
    archetype: ""
    job: pickpocket
    pf2e: pickpocket
    active: false
    activate_text: ""
    description: "+2 to Thievery checks; on a critical success you leave no trace."

  - id: hidden_compartment
    name: Hidden Compartment
    archetype: ""
    job: smuggler
    pf2e: concealment
    active: false
    activate_text: ""
    description: "You can conceal items of up to 2 bulk on your person; Perception DC to find them is +4."

  - id: appraise
    name: Appraise
    archetype: ""
    job: fence
    pf2e: quick_identification
    active: false
    activate_text: ""
    description: "Identify the value and origin of any item as a free action."

  - id: master_forgery
    name: Master Forgery
    archetype: ""
    job: forger
    pf2e: impersonate
    active: false
    activate_text: ""
    description: "Your forged documents require a critical success to detect; failures are not noticed until acted upon."

  - id: long_con
    name: Long Con
    archetype: ""
    job: con_artist
    pf2e: charming_liar
    active: false
    activate_text: ""
    description: "When you successfully Lie, the target remains unaware for up to 24 hours even after evidence surfaces."

  - id: back_channel
    name: Back Channel
    archetype: ""
    job: black_market_dealer
    pf2e: underground_contacts
    active: true
    activate_text: "You put out the word through your network."
    description: "Once per day, obtain any common or uncommon item at a 20% discount with no questions asked."

  - id: silent_entry
    name: Silent Entry
    archetype: ""
    job: safecracker
    pf2e: bypass_lock
    active: false
    activate_text: ""
    description: "Treat all locks as one grade easier; critical failure on lock-picking is treated as failure."

  - id: shell_game
    name: Shell Game
    archetype: ""
    job: grafter
    pf2e: distracting_performance
    active: true
    activate_text: "You draw everyone's attention with a sudden distraction."
    description: "All creatures in 20 ft must succeed a Perception check or be flat-footed until the end of your turn."

  - id: too_good_to_be_true
    name: Too Good to Be True
    archetype: ""
    job: grifter
    pf2e: silver_tongue
    active: false
    activate_text: ""
    description: "+2 to Deception; when you critically succeed on Deception the target is helpful to you for 1 hour."

  - id: make_an_example
    name: Make an Example
    archetype: ""
    job: extortionist
    pf2e: coerce
    active: true
    activate_text: "You make absolutely clear what happens to those who don't pay."
    description: "Intimidate a target into compliance; on success they take no hostile action against you for 24 hours."

  - id: paper_trail
    name: Paper Trail
    archetype: ""
    job: money_launderer
    pf2e: cover_tracks
    active: false
    activate_text: ""
    description: "Financial transactions you arrange are impossible to trace without a critical Investigate check."

  # drifter jobs
  - id: dead_reckoning
    name: Dead Reckoning
    archetype: ""
    job: scout
    pf2e: terrain_expertise
    active: false
    activate_text: ""
    description: "You always know your direction and current zone; no navigation penalty in explored areas."

  - id: read_the_signs
    name: Read the Signs
    archetype: ""
    job: tracker
    pf2e: read_sign
    active: false
    activate_text: ""
    description: "You can determine the number, type, and elapsed time of creatures that passed through an area."

  - id: patient_predator
    name: Patient Predator
    archetype: ""
    job: hunter
    pf2e: hunt_prey
    active: true
    activate_text: "You mark your quarry and settle in to wait."
    description: "Designate a target as hunted prey; +1 to hit and Perception checks against it for the encounter."

  - id: improvised_snare
    name: Improvised Snare
    archetype: ""
    job: trapper
    pf2e: snare_crafting
    active: false
    activate_text: ""
    description: "Craft a simple snare from available materials in 1 minute; no materials cost for common traps."

  - id: salvage_expert
    name: Salvage Expert
    archetype: ""
    job: scavenger
    pf2e: efficient_search
    active: false
    activate_text: ""
    description: "When searching a location you find twice the normal yield of usable materials."

  - id: make_camp
    name: Make Camp
    archetype: ""
    job: nomad
    pf2e: experienced_tracker
    active: false
    activate_text: ""
    description: "Set up a secure camp in any terrain in 10 minutes; rest in it grants +1 HP recovery per level."

  - id: know_the_road
    name: Know the Road
    archetype: ""
    job: wanderer
    pf2e: swift_sneak
    active: false
    activate_text: ""
    description: "You travel at full speed while using Stealth, and never get lost between known locations."

  - id: pathbreaker
    name: Pathbreaker
    archetype: ""
    job: wasteland_guide
    pf2e: legendary_guide
    active: false
    activate_text: ""
    description: "Your party ignores difficult terrain in wilderness areas and never triggers environmental hazards accidentally."

  - id: hard_to_pin_down
    name: Hard to Pin Down
    archetype: ""
    job: outrider
    pf2e: cavalry_charger
    active: false
    activate_text: ""
    description: "Enemies need to critically succeed to grab, trip, or immobilize you while you are mounted or sprinting."

  - id: cut_through
    name: Cut Through
    archetype: ""
    job: pathfinder
    pf2e: find_the_path
    active: true
    activate_text: "You study the lay of the land and identify the fastest route."
    description: "Once per hour, instantly determine the shortest safe path between your location and a known destination."

  - id: vanish
    name: Vanish
    archetype: ""
    job: ghost
    pf2e: swift_sneak
    active: true
    activate_text: "You step into a shadow and disappear."
    description: "Step into Stealth as a free action; if already hidden, become undetected until you attack or interact."

  - id: live_off_the_land
    name: Live Off the Land
    archetype: ""
    job: survivalist
    pf2e: forager
    active: false
    activate_text: ""
    description: "Automatically succeed at Survival checks to Subsist; critical success yields enough for 4 people."

  # influencer jobs
  - id: the_right_person
    name: The Right Person
    archetype: ""
    job: fixer
    pf2e: connections
    active: true
    activate_text: "You make a few calls and pull some strings."
    description: "Once per day, obtain a contact, safe house, or resource as if you had prepared it in advance."

  - id: close_the_deal
    name: Close the Deal
    archetype: ""
    job: negotiator
    pf2e: captivating_performance
    active: false
    activate_text: ""
    description: "+2 to Diplomacy; on a critical success during negotiation, the other party also owes you a future favor."

  - id: read_the_room
    name: Read the Room
    archetype: ""
    job: face
    pf2e: empathic_sense
    active: false
    activate_text: ""
    description: "Once per scene you may ask the GM the emotional disposition of every creature in the room."

  - id: social_exploit
    name: Social Exploit
    archetype: ""
    job: social_engineer
    pf2e: bon_mot
    active: true
    activate_text: "You find the exact pressure point and push."
    description: "Make a targeted Deception check; on success the target is flat-footed against your next action."

  - id: spin_cycle
    name: Spin Cycle
    archetype: ""
    job: propagandist
    pf2e: spread_the_word
    active: true
    activate_text: "You broadcast a carefully crafted message."
    description: "Up to 10 NPCs who hear you in the next minute become indifferent or friendly depending on your roll."

  - id: true_believer
    name: True Believer
    archetype: ""
    job: cult_leader
    pf2e: indoctrinate
    active: true
    activate_text: "You speak with absolute conviction."
    description: "One NPC who fails a Will save becomes a devoted follower for 24 hours."

  - id: fire_them_up
    name: Fire Them Up
    archetype: ""
    job: rabble_rouser
    pf2e: rally_the_mob
    active: true
    activate_text: "You whip the crowd into a frenzy."
    description: "All bystanders in 60 ft must succeed a Will save or become hostile to your designated target."

  - id: crowd_swell
    name: Crowd Swell
    archetype: ""
    job: demagogue
    pf2e: commanding_voice
    active: true
    activate_text: "Your voice cuts through all the noise."
    description: "Your Command Attention effect also applies a -1 penalty to enemy saves for the encounter."

  - id: reframe
    name: Reframe
    archetype: ""
    job: spin_doctor
    pf2e: reframe_failure
    active: true
    activate_text: "You immediately recontextualize the situation."
    description: "Once per encounter, convert a social critical failure to a normal failure."

  - id: star_maker
    name: Star Maker
    archetype: ""
    job: talent_agent
    pf2e: share_the_spotlight
    active: false
    activate_text: ""
    description: "Allies who benefit from your Command Attention also gain +1 to Deception and Diplomacy checks."

  - id: viral_moment
    name: Viral Moment
    archetype: ""
    job: brand_ambassador
    pf2e: captivating_performance
    active: true
    activate_text: "You stage an unforgettable spectacle."
    description: "Any NPC who witnesses this check must succeed on a Perception check or remember you positively for a week."

  - id: personal_brand
    name: Personal Brand
    archetype: ""
    job: influencer
    pf2e: legendary_performer
    active: false
    activate_text: ""
    description: "Your reputation precedes you; strangers who recognize you start one step more friendly."

  # nerd jobs
  - id: zero_day
    name: Zero Day
    archetype: ""
    job: hacker
    pf2e: exploit_vulnerability
    active: true
    activate_text: "You deploy a pre-staged exploit."
    description: "Remotely disable one electronic system within range for 1 minute without a trace."

  - id: memory_palace
    name: Memory Palace
    archetype: ""
    job: programmer
    pf2e: recall_knowledge
    active: false
    activate_text: ""
    description: "You automatically succeed at Recall Knowledge checks for any system you have previously interacted with."

  - id: jury_rig
    name: Jury-Rig
    archetype: ""
    job: engineer
    pf2e: quick_repair
    active: true
    activate_text: "You improvise a rapid field fix."
    description: "Repair a broken item or restore one disabled function as a single action."

  - id: field_medicine
    name: Field Medicine
    archetype: ""
    job: medic
    pf2e: battle_medicine
    active: true
    activate_text: "You administer emergency trauma care."
    description: "Restore 2d8 + your Medicine modifier HP to a creature as a single action; usable once per target per day."

  - id: hypothesis
    name: Hypothesis
    archetype: ""
    job: scientist
    pf2e: research
    active: true
    activate_text: "You form a rapid working hypothesis."
    description: "Once per scene, make an Arcana or Crafting check to deduce one hidden property of a creature or device."

  - id: unstable_mixture
    name: Unstable Mixture
    archetype: ""
    job: chemist
    pf2e: quick_alchemy
    active: true
    activate_text: "You hastily combine reagents."
    description: "Create one alchemical item from your formulaic mind list as a single action; it degrades after 1 hour."

  - id: controlled_dose
    name: Controlled Dose
    archetype: ""
    job: pharmacist
    pf2e: drug_tolerance
    active: false
    activate_text: ""
    description: "You suffer no negative effects from drugs or poisons at or below level 5."

  - id: hot_barrel
    name: Hot Barrel
    archetype: ""
    job: weaponsmith
    pf2e: expert_crafting
    active: false
    activate_text: ""
    description: "Weapons you personally craft deal +1 damage die size (d6 → d8, etc.)."

  - id: custom_fit
    name: Custom Fit
    archetype: ""
    job: armorsmith
    pf2e: reinforced_armor
    active: false
    activate_text: ""
    description: "Armor you personally craft provides +1 AC and reduces its check penalty by 1."

  - id: gadget_belt
    name: Gadget Belt
    archetype: ""
    job: gadgeteer
    pf2e: gadget_specialist
    active: false
    activate_text: ""
    description: "You may carry one additional gadget in a quick-draw slot with no bulk cost."

  - id: unbreakable_cipher
    name: Unbreakable Cipher
    archetype: ""
    job: cryptographer
    pf2e: coded_message
    active: false
    activate_text: ""
    description: "Messages you encrypt can only be broken on a critical success with a DC 35 Arcana or Computers check."

  - id: data_mining
    name: Data Mining
    archetype: ""
    job: data_broker
    pf2e: information_broker
    active: true
    activate_text: "You query your database for everything on this target."
    description: "Once per day, learn one secret or vulnerability about a specific person, organization, or location."

  # normie jobs
  - id: working_stiff
    name: Working Stiff
    archetype: ""
    job: laborer
    pf2e: powerful_leap
    active: false
    activate_text: ""
    description: "You can carry twice the normal bulk without being encumbered."

  - id: defensive_driving
    name: Defensive Driving
    archetype: ""
    job: driver
    pf2e: skilled_driver
    active: false
    activate_text: ""
    description: "You never suffer penalties on Driving checks from weather or difficult terrain."

  - id: comfort_food
    name: Comfort Food
    archetype: ""
    job: cook
    pf2e: expeditious_search
    active: true
    activate_text: "You whip something up from whatever's on hand."
    description: "Prepare a meal in 10 minutes that restores 1d6 HP per level to everyone who eats it (once per day)."

  - id: shade_tree
    name: Shade Tree
    archetype: ""
    job: mechanic
    pf2e: quick_repair
    active: true
    activate_text: "You pop the hood and get to work."
    description: "Restore a vehicle or machine to operational status in 1 minute without tools."

  - id: off_the_books
    name: Off the Books
    archetype: ""
    job: janitor
    pf2e: cover_tracks
    active: false
    activate_text: ""
    description: "You can destroy all physical evidence of a scene in 10 minutes; Investigate DC +5 to detect tampering."

  - id: i_know_a_guy
    name: I Know a Guy
    archetype: ""
    job: courier
    pf2e: connections
    active: true
    activate_text: "You pull out your phone and make a quick call."
    description: "Once per day, arrange delivery of any non-rare item to any location within the zone by next session."

  - id: loading_strength
    name: Loading Strength
    archetype: ""
    job: dockworker
    pf2e: powerful_leap
    active: false
    activate_text: ""
    description: "You can move at full speed while carrying up to 10 bulk, and can drag up to 20 bulk."

  - id: land_stewardship
    name: Land Stewardship
    archetype: ""
    job: farmer
    pf2e: forager
    active: false
    activate_text: ""
    description: "You can identify any edible or medicinal plant or animal on sight; Subsist checks always succeed in rural areas."

  - id: vendor_patter
    name: Vendor Patter
    archetype: ""
    job: street_vendor
    pf2e: bargain_hunter
    active: false
    activate_text: ""
    description: "You buy equipment and goods at a 10% discount and sell at a 10% premium."

  - id: last_call
    name: Last Call
    archetype: ""
    job: bartender
    pf2e: assurance
    active: true
    activate_text: "You cut someone off and make it clear it isn't up for debate."
    description: "Once per scene, automatically succeed at an Intimidation or Diplomacy check against a single individual."

  - id: eyes_on_the_door
    name: Eyes on the Door
    archetype: ""
    job: security_guard
    pf2e: vigilant_sentry
    active: false
    activate_text: ""
    description: "You cannot be surprised; you always act in the first round of combat."

  - id: bone_breaker
    name: Bone Breaker
    archetype: ""
    job: beat_down_artist
    pf2e: grapple
    active: false
    activate_text: ""
    description: "A successful Grapple check reduces the target's movement speed by 10 ft until end of encounter."
```

**Step 2: Verify count**

```bash
grep -c "^  - id:" content/class_features.yaml
```
Expected output: `88`

**Step 3: Run tests**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -20
```
Expected: clean build (no new Go files yet, just YAML)

**Step 4: Commit**

```bash
git add content/class_features.yaml
git commit -m "feat: add content/class_features.yaml (88 features: 12 archetype + 76 job)"
```

---

### Task 2: `internal/game/ruleset/class_feature.go`

**Files:**
- Create: `internal/game/ruleset/class_feature.go`
- Create: `internal/game/ruleset/class_feature_test.go`

**Step 1: Write the failing test**

```go
// internal/game/ruleset/class_feature_test.go
package ruleset_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadClassFeatures_Count(t *testing.T) {
    features, err := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
    if err != nil {
        t.Fatalf("LoadClassFeatures: %v", err)
    }
    if len(features) != 88 {
        t.Errorf("expected 88 features, got %d", len(features))
    }
}

func TestClassFeatureRegistry_ByArchetype(t *testing.T) {
    features, _ := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
    reg := ruleset.NewClassFeatureRegistry(features)

    archetypeFeatures := reg.ByArchetype("aggressor")
    if len(archetypeFeatures) != 2 {
        t.Errorf("expected 2 aggressor archetype features, got %d", len(archetypeFeatures))
    }
}

func TestClassFeatureRegistry_ByJob(t *testing.T) {
    features, _ := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
    reg := ruleset.NewClassFeatureRegistry(features)

    jobFeatures := reg.ByJob("soldier")
    if len(jobFeatures) != 1 {
        t.Errorf("expected 1 soldier job feature, got %d", len(jobFeatures))
    }
}

func TestClassFeatureRegistry_ClassFeature(t *testing.T) {
    features, _ := ruleset.LoadClassFeatures("../../../content/class_features.yaml")
    reg := ruleset.NewClassFeatureRegistry(features)

    f, ok := reg.ClassFeature("brutal_surge")
    if !ok {
        t.Fatal("brutal_surge not found")
    }
    if !f.Active {
        t.Error("brutal_surge should be active")
    }
    if f.Archetype != "aggressor" {
        t.Errorf("expected archetype=aggressor, got %q", f.Archetype)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestLoad -v 2>&1 | tail -10
```
Expected: FAIL — `LoadClassFeatures` undefined

**Step 3: Write the implementation**

```go
// internal/game/ruleset/class_feature.go
package ruleset

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// ClassFeature defines one Gunchete class feature and its P2FE equivalent.
//
// Archetype is non-empty for archetype-shared features; Job is non-empty for job-specific features.
// Active features require player action to use; passive features are always-on.
type ClassFeature struct {
    ID           string `yaml:"id"`
    Name         string `yaml:"name"`
    Archetype    string `yaml:"archetype"`
    Job          string `yaml:"job"`
    PF2E         string `yaml:"pf2e"`
    Active       bool   `yaml:"active"`
    ActivateText string `yaml:"activate_text"`
    Description  string `yaml:"description"`
}

// classFeaturesFile is the top-level YAML structure for content/class_features.yaml.
type classFeaturesFile struct {
    ClassFeatures []*ClassFeature `yaml:"class_features"`
}

// LoadClassFeatures reads the class features master YAML file and returns all feature definitions.
//
// Precondition: path must be a readable file containing valid YAML.
// Postcondition: Returns all class features or a non-nil error.
func LoadClassFeatures(path string) ([]*ClassFeature, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading class features file %s: %w", path, err)
    }
    var f classFeaturesFile
    if err := yaml.Unmarshal(data, &f); err != nil {
        return nil, fmt.Errorf("parsing class features file %s: %w", path, err)
    }
    return f.ClassFeatures, nil
}

// ClassFeatureRegistry provides fast lookup of class features by ID, archetype, and job.
type ClassFeatureRegistry struct {
    byID        map[string]*ClassFeature
    byArchetype map[string][]*ClassFeature
    byJob       map[string][]*ClassFeature
}

// NewClassFeatureRegistry builds a ClassFeatureRegistry from the given feature slice.
//
// Precondition: features must not be nil.
// Postcondition: Returns a fully indexed registry.
func NewClassFeatureRegistry(features []*ClassFeature) *ClassFeatureRegistry {
    r := &ClassFeatureRegistry{
        byID:        make(map[string]*ClassFeature),
        byArchetype: make(map[string][]*ClassFeature),
        byJob:       make(map[string][]*ClassFeature),
    }
    for _, f := range features {
        r.byID[f.ID] = f
        if f.Archetype != "" {
            r.byArchetype[f.Archetype] = append(r.byArchetype[f.Archetype], f)
        }
        if f.Job != "" {
            r.byJob[f.Job] = append(r.byJob[f.Job], f)
        }
    }
    return r
}

// ClassFeature returns the feature with the given ID, or false if not found.
//
// Precondition: id must be non-empty.
func (r *ClassFeatureRegistry) ClassFeature(id string) (*ClassFeature, bool) {
    f, ok := r.byID[id]
    return f, ok
}

// ByArchetype returns all features for the given archetype (shared features).
//
// Precondition: archetype must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *ClassFeatureRegistry) ByArchetype(archetype string) []*ClassFeature {
    return r.byArchetype[archetype]
}

// ByJob returns all features for the given job (job-specific features).
//
// Precondition: job must be non-empty.
// Postcondition: Returns a slice (may be empty).
func (r *ClassFeatureRegistry) ByJob(job string) []*ClassFeature {
    return r.byJob[job]
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestClassFeature -v 2>&1
```
Expected: 4 tests PASS

**Step 5: Commit**

```bash
git add internal/game/ruleset/class_feature.go internal/game/ruleset/class_feature_test.go
git commit -m "feat: add ClassFeature type, LoadClassFeatures, ClassFeatureRegistry"
```

---

### Task 3: Extend Job struct with `ClassFeatureGrants`

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/job_test.go` (add test for loading class_features field)

**Step 1: Read current job.go**

Read `internal/game/ruleset/job.go` and find the `Job` struct.

**Step 2: Write a failing test**

Add to `internal/game/ruleset/job_test.go`:

```go
func TestJob_ClassFeatureGrants_Field(t *testing.T) {
    // Create a temp YAML with class_features block
    content := `
jobs:
  - id: test_job
    name: Test Job
    archetype: normie
    class_features:
      - resilience
      - working_stiff
`
    tmpFile := t.TempDir() + "/jobs.yaml"
    if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
        t.Fatal(err)
    }
    dir := t.TempDir()
    if err := os.WriteFile(dir+"/test_job.yaml", []byte(content[7:]), 0644); err != nil {
        t.Fatal(err)
    }
    // Parse a single job with class_features
    type jobFile struct {
        Jobs []*Job `yaml:"jobs"`
    }
    var jf jobFile
    if err := yaml.Unmarshal([]byte(content), &jf); err != nil {
        t.Fatal(err)
    }
    if len(jf.Jobs) == 0 {
        t.Fatal("no jobs parsed")
    }
    j := jf.Jobs[0]
    if len(j.ClassFeatureGrants) != 2 {
        t.Errorf("expected 2 ClassFeatureGrants, got %d: %v", len(j.ClassFeatureGrants), j.ClassFeatureGrants)
    }
    if j.ClassFeatureGrants[0] != "resilience" {
        t.Errorf("expected resilience, got %q", j.ClassFeatureGrants[0])
    }
}
```

**Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestJob_ClassFeatureGrants -v 2>&1 | tail -10
```
Expected: FAIL — `ClassFeatureGrants` not a field

**Step 4: Add the field to the Job struct**

In `internal/game/ruleset/job.go`, add to the `Job` struct after the `FeatGrants` field:

```go
ClassFeatureGrants []string `yaml:"class_features"`
```

**Step 5: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestJob_ClassFeatureGrants -v 2>&1
```
Expected: PASS

**Step 6: Run all ruleset tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -v 2>&1 | tail -20
```
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/game/ruleset/job.go internal/game/ruleset/job_test.go
git commit -m "feat: add ClassFeatureGrants field to Job struct"
```

---

### Task 4: Update all 76 job YAMLs

**Files:**
- Modify: all 76 `content/jobs/*.yaml` files

Each job YAML needs two changes:
1. **Remove** any `features:` block (legacy data from import)
2. **Add** a `class_features:` block with the correct IDs

The `class_features` list for each job must include:
- Both archetype-shared feature IDs for that job's archetype
- The job-specific feature ID

**Mapping (archetype → shared feature IDs):**
- aggressor: `[street_brawler, brutal_surge]`
- criminal: `[sucker_punch, slippery]`
- drifter: `[predators_eye, zone_awareness]`
- influencer: `[command_attention, fast_talk]`
- nerd: `[daily_prep, formulaic_mind]`
- normie: `[street_tough, resilience]`

**Per-job complete class_features lists:**

```
soldier:        [street_brawler, brutal_surge, guerilla_warfare]
enforcer:       [street_brawler, brutal_surge, crowd_control]
brawler:        [street_brawler, brutal_surge, iron_chin]
thug:           [street_brawler, brutal_surge, pack_mentality]
heavy:          [street_brawler, brutal_surge, suppressing_fire]
berserker:      [street_brawler, brutal_surge, blood_frenzy]
pit_fighter:    [street_brawler, brutal_surge, dirty_fighter]
cage_fighter:   [street_brawler, brutal_surge, no_tap_out]
street_fighter: [street_brawler, brutal_surge, haymaker]
headhunter:     [street_brawler, brutal_surge, trophy_hunter]
bouncer:        [street_brawler, brutal_surge, unmovable]
warmonger:      [street_brawler, brutal_surge, battle_cry]

thief:              [sucker_punch, slippery, five_finger_discount]
pickpocket:         [sucker_punch, slippery, light_touch]
smuggler:           [sucker_punch, slippery, hidden_compartment]
fence:              [sucker_punch, slippery, appraise]
forger:             [sucker_punch, slippery, master_forgery]
con_artist:         [sucker_punch, slippery, long_con]
black_market_dealer:[sucker_punch, slippery, back_channel]
safecracker:        [sucker_punch, slippery, silent_entry]
grafter:            [sucker_punch, slippery, shell_game]
grifter:            [sucker_punch, slippery, too_good_to_be_true]
extortionist:       [sucker_punch, slippery, make_an_example]
money_launderer:    [sucker_punch, slippery, paper_trail]

scout:          [predators_eye, zone_awareness, dead_reckoning]
tracker:        [predators_eye, zone_awareness, read_the_signs]
hunter:         [predators_eye, zone_awareness, patient_predator]
trapper:        [predators_eye, zone_awareness, improvised_snare]
scavenger:      [predators_eye, zone_awareness, salvage_expert]
nomad:          [predators_eye, zone_awareness, make_camp]
wanderer:       [predators_eye, zone_awareness, know_the_road]
wasteland_guide:[predators_eye, zone_awareness, pathbreaker]
outrider:       [predators_eye, zone_awareness, hard_to_pin_down]
pathfinder:     [predators_eye, zone_awareness, cut_through]
ghost:          [predators_eye, zone_awareness, vanish]
survivalist:    [predators_eye, zone_awareness, live_off_the_land]

fixer:            [command_attention, fast_talk, the_right_person]
negotiator:       [command_attention, fast_talk, close_the_deal]
face:             [command_attention, fast_talk, read_the_room]
social_engineer:  [command_attention, fast_talk, social_exploit]
propagandist:     [command_attention, fast_talk, spin_cycle]
cult_leader:      [command_attention, fast_talk, true_believer]
rabble_rouser:    [command_attention, fast_talk, fire_them_up]
demagogue:        [command_attention, fast_talk, crowd_swell]
spin_doctor:      [command_attention, fast_talk, reframe]
talent_agent:     [command_attention, fast_talk, star_maker]
brand_ambassador: [command_attention, fast_talk, viral_moment]
influencer:       [command_attention, fast_talk, personal_brand]

hacker:        [daily_prep, formulaic_mind, zero_day]
programmer:    [daily_prep, formulaic_mind, memory_palace]
engineer:      [daily_prep, formulaic_mind, jury_rig]
medic:         [daily_prep, formulaic_mind, field_medicine]
scientist:     [daily_prep, formulaic_mind, hypothesis]
chemist:       [daily_prep, formulaic_mind, unstable_mixture]
pharmacist:    [daily_prep, formulaic_mind, controlled_dose]
weaponsmith:   [daily_prep, formulaic_mind, hot_barrel]
armorsmith:    [daily_prep, formulaic_mind, custom_fit]
gadgeteer:     [daily_prep, formulaic_mind, gadget_belt]
cryptographer: [daily_prep, formulaic_mind, unbreakable_cipher]
data_broker:   [daily_prep, formulaic_mind, data_mining]

laborer:        [street_tough, resilience, working_stiff]
driver:         [street_tough, resilience, defensive_driving]
cook:           [street_tough, resilience, comfort_food]
mechanic:       [street_tough, resilience, shade_tree]
janitor:        [street_tough, resilience, off_the_books]
courier:        [street_tough, resilience, i_know_a_guy]
dockworker:     [street_tough, resilience, loading_strength]
farmer:         [street_tough, resilience, land_stewardship]
street_vendor:  [street_tough, resilience, vendor_patter]
bartender:      [street_tough, resilience, last_call]
security_guard: [street_tough, resilience, eyes_on_the_door]
beat_down_artist:[street_tough, resilience, bone_breaker]
```

**Step 1: Write a verification test first**

Create `internal/game/ruleset/job_class_features_test.go`:

```go
package ruleset_test

import (
    "testing"
)

func TestAllJobsHaveClassFeatures(t *testing.T) {
    jobs, err := LoadJobs("../../../content/jobs")
    if err != nil {
        t.Fatalf("LoadJobs: %v", err)
    }
    if len(jobs) != 76 {
        t.Errorf("expected 76 jobs, got %d", len(jobs))
    }
    for _, j := range jobs {
        if len(j.ClassFeatureGrants) == 0 {
            t.Errorf("job %q has no ClassFeatureGrants", j.ID)
        }
        if len(j.ClassFeatureGrants) != 3 {
            t.Errorf("job %q has %d ClassFeatureGrants, expected 3", j.ID, len(j.ClassFeatureGrants))
        }
    }
}

func TestNoJobHasLegacyFeaturesBlock(t *testing.T) {
    jobs, err := LoadJobs("../../../content/jobs")
    if err != nil {
        t.Fatalf("LoadJobs: %v", err)
    }
    for _, j := range jobs {
        // Legacy features block would have parsed into Features field if it existed.
        // This test ensures we fully removed it by checking the raw field.
        // (see Job struct — Features []string `yaml:"features"` must NOT be present)
        _ = j // The struct field simply won't exist after removal
    }
}
```

**Step 2: Run failing test**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestAllJobsHaveClassFeatures -v 2>&1 | tail -20
```
Expected: FAIL — all jobs report 0 ClassFeatureGrants

**Step 3: Update each job YAML**

For each of the 76 job files in `content/jobs/`:
1. Remove any `features:` block (grep first to confirm which files have it)
2. Add `class_features:` block with the 3 IDs from the mapping above

Check which jobs have a legacy `features:` block:
```bash
grep -l "^features:" content/jobs/*.yaml
```

For each job YAML, the addition looks like:
```yaml
class_features:
  - street_brawler
  - brutal_surge
  - guerilla_warfare
```

**Step 4: Run test to verify all jobs pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -run TestAllJobsHaveClassFeatures -v 2>&1
```
Expected: PASS

**Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: all PASS

**Step 6: Commit**

```bash
git add content/jobs/
git add internal/game/ruleset/job_class_features_test.go
git commit -m "feat: add class_features to all 76 job YAMLs, remove legacy features blocks"
```

---

### Task 5: DB migration `013_character_class_features`

**Files:**
- Create: `migrations/013_character_class_features.up.sql`
- Create: `migrations/013_character_class_features.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/013_character_class_features.up.sql
CREATE TABLE character_class_features (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id   TEXT NOT NULL,
    PRIMARY KEY (character_id, feature_id)
);
```

**Step 2: Write the down migration**

```sql
-- migrations/013_character_class_features.down.sql
DROP TABLE IF EXISTS character_class_features;
```

**Step 3: Apply the migration**

```bash
cd /home/cjohannsen/src/mud && make migrate 2>&1
```
Expected: migration applied, no errors

If `make migrate` is not available:
```bash
cd /home/cjohannsen/src/mud && go run ./cmd/migrate/... up 2>&1
```

**Step 4: Verify the table exists**

```bash
psql $DATABASE_URL -c "\d character_class_features" 2>&1
```
Expected: shows table with character_id and feature_id columns

**Step 5: Commit**

```bash
git add migrations/013_character_class_features.up.sql migrations/013_character_class_features.down.sql
git commit -m "feat: add migration 013_character_class_features"
```

---

### Task 6: `CharacterClassFeaturesRepository`

**Files:**
- Create: `internal/storage/postgres/character_class_features.go`
- Create: `internal/storage/postgres/character_class_features_test.go`

**Step 1: Write the failing test**

```go
// internal/storage/postgres/character_class_features_test.go
package postgres_test

import (
    "context"
    "testing"

    "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterClassFeaturesRepository_SetAndGet(t *testing.T) {
    if testDB == nil {
        t.Skip("no test database")
    }
    repo := postgres.NewCharacterClassFeaturesRepository(testDB)
    ctx := context.Background()
    characterID := int64(99991) // use a test character ID

    // Ensure clean state
    _ = repo.SetAll(ctx, characterID, []string{})

    has, err := repo.HasClassFeatures(ctx, characterID)
    if err != nil {
        t.Fatalf("HasClassFeatures: %v", err)
    }
    if has {
        t.Error("expected no class features initially")
    }

    features := []string{"brutal_surge", "street_brawler", "guerilla_warfare"}
    if err := repo.SetAll(ctx, characterID, features); err != nil {
        t.Fatalf("SetAll: %v", err)
    }

    has, err = repo.HasClassFeatures(ctx, characterID)
    if err != nil {
        t.Fatalf("HasClassFeatures after set: %v", err)
    }
    if !has {
        t.Error("expected class features after SetAll")
    }

    got, err := repo.GetAll(ctx, characterID)
    if err != nil {
        t.Fatalf("GetAll: %v", err)
    }
    if len(got) != 3 {
        t.Errorf("expected 3 features, got %d", len(got))
    }

    // Test replace (SetAll again with different list)
    if err := repo.SetAll(ctx, characterID, []string{"resilience"}); err != nil {
        t.Fatalf("SetAll replace: %v", err)
    }
    got2, _ := repo.GetAll(ctx, characterID)
    if len(got2) != 1 || got2[0] != "resilience" {
        t.Errorf("expected [resilience] after replace, got %v", got2)
    }

    // Cleanup
    _ = repo.SetAll(ctx, characterID, []string{})
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/storage/postgres/... -run TestCharacterClassFeatures -v 2>&1 | tail -10
```
Expected: FAIL — `NewCharacterClassFeaturesRepository` undefined

**Step 3: Write the implementation**

```go
// internal/storage/postgres/character_class_features.go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterClassFeaturesRepository persists per-character class feature lists.
type CharacterClassFeaturesRepository struct {
    db *pgxpool.Pool
}

// NewCharacterClassFeaturesRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterClassFeaturesRepository(db *pgxpool.Pool) *CharacterClassFeaturesRepository {
    return &CharacterClassFeaturesRepository{db: db}
}

// HasClassFeatures reports whether the character has any rows in character_class_features.
//
// Precondition: characterID > 0.
// Postcondition: Returns true if at least one feature row exists.
func (r *CharacterClassFeaturesRepository) HasClassFeatures(ctx context.Context, characterID int64) (bool, error) {
    var count int
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM character_class_features WHERE character_id = $1`, characterID,
    ).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("HasClassFeatures: %w", err)
    }
    return count > 0, nil
}

// GetAll returns all class feature IDs for a character.
//
// Precondition: characterID > 0.
// Postcondition: Returns a slice of feature IDs (may be empty).
func (r *CharacterClassFeaturesRepository) GetAll(ctx context.Context, characterID int64) ([]string, error) {
    rows, err := r.db.Query(ctx,
        `SELECT feature_id FROM character_class_features WHERE character_id = $1`, characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("GetAll class features: %w", err)
    }
    defer rows.Close()

    var out []string
    for rows.Next() {
        var id string
        if err := rows.Scan(&id); err != nil {
            return nil, fmt.Errorf("scanning class feature row: %w", err)
        }
        out = append(out, id)
    }
    return out, rows.Err()
}

// SetAll writes the complete class feature list for a character, replacing any existing rows.
//
// Precondition: characterID > 0; featureIDs must not be nil.
// Postcondition: character_class_features rows match featureIDs exactly.
func (r *CharacterClassFeaturesRepository) SetAll(ctx context.Context, characterID int64, featureIDs []string) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) //nolint:errcheck

    if _, err := tx.Exec(ctx,
        `DELETE FROM character_class_features WHERE character_id = $1`, characterID,
    ); err != nil {
        return fmt.Errorf("deleting old class features: %w", err)
    }

    for _, featureID := range featureIDs {
        if _, err := tx.Exec(ctx,
            `INSERT INTO character_class_features (character_id, feature_id) VALUES ($1, $2)`,
            characterID, featureID,
        ); err != nil {
            return fmt.Errorf("inserting class feature %s: %w", featureID, err)
        }
    }
    return tx.Commit(ctx)
}
```

**Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/storage/postgres/... -run TestCharacterClassFeatures -v 2>&1
```
Expected: PASS (or SKIP if no test DB)

**Step 5: Run full build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: clean build

**Step 6: Commit**

```bash
git add internal/storage/postgres/character_class_features.go internal/storage/postgres/character_class_features_test.go
git commit -m "feat: add CharacterClassFeaturesRepository"
```

---

### Task 7: Character model + `BuildClassFeaturesFromJob`

**Files:**
- Modify: `internal/game/character/model.go`
- Modify: `internal/game/character/builder.go`
- Create: `internal/game/character/class_features_test.go`

**Step 1: Write failing tests**

```go
// internal/game/character/class_features_test.go
package character_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/character"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestBuildClassFeaturesFromJob(t *testing.T) {
    job := &ruleset.Job{
        ID:                 "soldier",
        ClassFeatureGrants: []string{"street_brawler", "brutal_surge", "guerilla_warfare"},
    }

    result := character.BuildClassFeaturesFromJob(job)

    if len(result) != 3 {
        t.Errorf("expected 3 class features, got %d", len(result))
    }

    featureSet := make(map[string]bool)
    for _, id := range result {
        featureSet[id] = true
    }
    for _, expected := range []string{"street_brawler", "brutal_surge", "guerilla_warfare"} {
        if !featureSet[expected] {
            t.Errorf("expected feature %q not found in result %v", expected, result)
        }
    }
}

func TestBuildClassFeaturesFromJob_Empty(t *testing.T) {
    job := &ruleset.Job{ID: "test", ClassFeatureGrants: nil}
    result := character.BuildClassFeaturesFromJob(job)
    if len(result) != 0 {
        t.Errorf("expected empty result for nil grants, got %v", result)
    }
}
```

**Step 2: Run to verify failure**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/character/... -run TestBuildClassFeatures -v 2>&1 | tail -10
```
Expected: FAIL — `BuildClassFeaturesFromJob` undefined

**Step 3: Add `ClassFeatures []string` to Character model**

In `internal/game/character/model.go`, add after the `Feats []string` field:
```go
ClassFeatures []string
```

**Step 4: Add `BuildClassFeaturesFromJob` to builder**

In `internal/game/character/builder.go`, add:

```go
// BuildClassFeaturesFromJob returns all class feature IDs granted by the job.
// All grants are fixed; no player selection required.
//
// Precondition: job must be non-nil.
// Postcondition: Returns a slice of feature IDs (may be empty if job has none).
func BuildClassFeaturesFromJob(job *ruleset.Job) []string {
    if len(job.ClassFeatureGrants) == 0 {
        return nil
    }
    result := make([]string, len(job.ClassFeatureGrants))
    copy(result, job.ClassFeatureGrants)
    return result
}
```

**Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/character/... -run TestBuildClassFeatures -v 2>&1
```
Expected: PASS

**Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/game/character/model.go internal/game/character/builder.go internal/game/character/class_features_test.go
git commit -m "feat: add ClassFeatures to Character model and BuildClassFeaturesFromJob"
```

---

### Task 8: `ensureClassFeatures` + auth.go wiring

**Files:**
- Modify: `internal/frontend/handlers/auth.go`
- Modify: `internal/frontend/handlers/character_flow.go`

**Step 1: Write a failing test**

In `internal/frontend/handlers/auth_test.go` (or a new `ensure_class_features_test.go`):

```go
// Verify CharacterClassFeaturesSetter interface is defined and AuthHandler accepts it
func TestAuthHandler_AcceptsClassFeaturesSetter(t *testing.T) {
    var _ handlers.CharacterClassFeaturesSetter = (*mockClassFeaturesSetter)(nil)
}

type mockClassFeaturesSetter struct{}
func (m *mockClassFeaturesSetter) HasClassFeatures(_ context.Context, _ int64) (bool, error) { return false, nil }
func (m *mockClassFeaturesSetter) SetAll(_ context.Context, _ int64, _ []string) error { return nil }
```

**Step 2: Add `CharacterClassFeaturesSetter` interface to auth.go**

In `internal/frontend/handlers/auth.go`, after `CharacterFeatsSetter`:

```go
// CharacterClassFeaturesSetter defines class feature persistence operations required by AuthHandler.
type CharacterClassFeaturesSetter interface {
    HasClassFeatures(ctx context.Context, characterID int64) (bool, error)
    SetAll(ctx context.Context, characterID int64, featureIDs []string) error
}
```

**Step 3: Add fields to AuthHandler struct**

In `internal/frontend/handlers/auth.go`, add to the `AuthHandler` struct after `featRegistry`:

```go
characterClassFeatures  CharacterClassFeaturesSetter
allClassFeatures        []*ruleset.ClassFeature
classFeatureRegistry    *ruleset.ClassFeatureRegistry
```

**Step 4: Update `NewAuthHandler` signature**

Add three new parameters at the end of `NewAuthHandler`:

```go
allClassFeatures []*ruleset.ClassFeature,
characterClassFeatures CharacterClassFeaturesSetter,
```

And inside the function body, after building `featReg`, add:

```go
var cfReg *ruleset.ClassFeatureRegistry
if len(allClassFeatures) > 0 {
    cfReg = ruleset.NewClassFeatureRegistry(allClassFeatures)
}
```

And in the `&AuthHandler{...}` return, add:

```go
characterClassFeatures: characterClassFeatures,
allClassFeatures:       allClassFeatures,
classFeatureRegistry:   cfReg,
```

**Step 5: Add `ensureClassFeatures` to character_flow.go**

In `internal/frontend/handlers/character_flow.go`, after `ensureFeats`, add:

```go
// ensureClassFeatures checks whether the character has class features recorded and, if not,
// assigns all class features from the job (all fixed — no player selection) and persists.
// Called before gameBridge for both new and existing characters so backfill always runs.
//
// Precondition: char must have a valid ID and Class set.
// Postcondition: character_class_features rows exist for char; returns non-nil error only on fatal failure.
func (h *AuthHandler) ensureClassFeatures(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
    if h.characterClassFeatures == nil || h.classFeatureRegistry == nil {
        return nil
    }
    has, err := h.characterClassFeatures.HasClassFeatures(ctx, char.ID)
    if err != nil {
        h.logger.Warn("checking class features for character", zap.Int64("id", char.ID), zap.Error(err))
        return nil
    }
    if has {
        return nil
    }

    job, ok := h.jobRegistry.Job(char.Class)
    if !ok {
        h.logger.Warn("unknown job for class feature backfill", zap.String("class", char.Class))
        return nil
    }

    featureIDs := character.BuildClassFeaturesFromJob(job)
    if len(featureIDs) == 0 {
        return nil
    }

    if err := h.characterClassFeatures.SetAll(ctx, char.ID, featureIDs); err != nil {
        h.logger.Warn("persisting class features", zap.Int64("id", char.ID), zap.Error(err))
        return nil
    }

    _ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
        "Class features assigned: %d features from %s.", len(featureIDs), job.Name))
    return nil
}
```

**Step 6: Call `ensureClassFeatures` at all 3 `gameBridge` call sites**

In `character_flow.go`, find every call to `ensureFeats` and add a call to `ensureClassFeatures` immediately after:

```go
if err := h.ensureFeats(ctx, conn, char); err != nil {
    return err
}
if err := h.ensureClassFeatures(ctx, conn, char); err != nil {
    return err
}
```

**Step 7: Update all test call sites for `NewAuthHandler`**

Find all places `NewAuthHandler` is called and add the two new nil arguments:

```bash
grep -rn "NewAuthHandler" /home/cjohannsen/src/mud --include="*.go"
```

Add `nil, nil` (for `allClassFeatures` and `characterClassFeatures`) at the end of each call.

**Step 8: Build and test**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... 2>&1 | tail -20
```
Expected: all PASS

**Step 9: Commit**

```bash
git add internal/frontend/handlers/auth.go internal/frontend/handlers/character_flow.go
git commit -m "feat: add ensureClassFeatures + CharacterClassFeaturesSetter to auth handler"
```

---

### Task 9: `class_features` command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/class_features.go`
- Create: `internal/game/command/class_features_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/gameserver/grpc_service.go`

#### CMD-1: Add constant to commands.go

```go
HandlerClassFeatures = "class_features"
```

#### CMD-2: Add Command entry in BuiltinCommands()

```go
{Name: HandlerClassFeatures, Aliases: []string{"cf"}, Description: "List your class features", Category: "character"},
```

#### CMD-3: Create `internal/game/command/class_features.go`

```go
package command

// HandleClassFeatures returns the handler name for the class_features command.
//
// Postcondition: Returns HandlerClassFeatures.
func HandleClassFeatures() string {
    return HandlerClassFeatures
}
```

Create `internal/game/command/class_features_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleClassFeatures(t *testing.T) {
    if command.HandleClassFeatures() != command.HandlerClassFeatures {
        t.Error("HandleClassFeatures should return HandlerClassFeatures")
    }
}

func TestClassFeaturesCommandRegistered(t *testing.T) {
    cmds := command.BuiltinCommands()
    for _, c := range cmds {
        if c.Name == command.HandlerClassFeatures {
            return
        }
    }
    t.Error("class_features not in BuiltinCommands")
}
```

#### CMD-4: Add proto messages

In `api/proto/game/v1/game.proto`:

Add to `ClientMessage` oneof (field 42):
```protobuf
ClassFeaturesRequest class_features_request = 42;
```

Add the `ClassFeaturesRequest` message:
```protobuf
message ClassFeaturesRequest {}
```

Add to `ServerEvent` oneof (field 23):
```protobuf
ClassFeaturesResponse class_features_response = 23;
```

Add the response messages:
```protobuf
message ClassFeatureEntry {
    string feature_id    = 1;
    string name          = 2;
    string archetype     = 3;
    string job           = 4;
    bool   active        = 5;
    string description   = 6;
    string activate_text = 7;
}

message ClassFeaturesResponse {
    repeated ClassFeatureEntry archetype_features = 1;
    repeated ClassFeatureEntry job_features       = 2;
}
```

Run proto generation:
```bash
cd /home/cjohannsen/src/mud && make proto 2>&1
```
Expected: regenerates without errors

#### CMD-5: Add bridge handler

In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:
```go
command.HandlerClassFeatures: bridgeClassFeatures,
```

Add the bridge function:
```go
func bridgeClassFeatures(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{
        msg: &gamev1.ClientMessage{
            Payload: &gamev1.ClientMessage_ClassFeaturesRequest{
                ClassFeaturesRequest: &gamev1.ClassFeaturesRequest{},
            },
        },
    }, nil
}
```

Verify `TestAllCommandHandlersAreWired` passes:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

#### CMD-6: Add server handler

In `internal/gameserver/grpc_service.go`, add fields to `GameServiceServer`:
```go
allClassFeatures            []*ruleset.ClassFeature
classFeatureRegistry        *ruleset.ClassFeatureRegistry
characterClassFeaturesRepo  *postgres.CharacterClassFeaturesRepository
```

Update `NewGameServiceServer` — add parameters at the end:
```go
allClassFeatures []*ruleset.ClassFeature,
classFeatureRegistry *ruleset.ClassFeatureRegistry,
characterClassFeaturesRepo *postgres.CharacterClassFeaturesRepository,
```

Set them in the return struct:
```go
allClassFeatures:           allClassFeatures,
classFeatureRegistry:       classFeatureRegistry,
characterClassFeaturesRepo: characterClassFeaturesRepo,
```

Update the comment block describing the new parameters (nil-safe, skips if nil).

Add to dispatch switch:
```go
case *gamev1.ClientMessage_ClassFeaturesRequest:
    return s.handleClassFeatures(uid)
```

Add handler function:
```go
// handleClassFeatures returns all class feature entries for the player's current character.
//
// Precondition: uid must resolve to an active session with a loaded character.
// Postcondition: Returns a ClassFeaturesResponse partitioned into archetype vs. job features.
func (s *GameServiceServer) handleClassFeatures(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }
    if s.characterClassFeaturesRepo == nil || s.classFeatureRegistry == nil {
        return messageEvent("Class feature data is not available."), nil
    }
    featureIDs, err := s.characterClassFeaturesRepo.GetAll(context.Background(), sess.CharacterID)
    if err != nil {
        return nil, fmt.Errorf("getting class features: %w", err)
    }

    var archetypeFeatures []*gamev1.ClassFeatureEntry
    var jobFeatures []*gamev1.ClassFeatureEntry

    for _, id := range featureIDs {
        f, ok := s.classFeatureRegistry.ClassFeature(id)
        if !ok {
            continue
        }
        entry := &gamev1.ClassFeatureEntry{
            FeatureId:    f.ID,
            Name:         f.Name,
            Archetype:    f.Archetype,
            Job:          f.Job,
            Active:       f.Active,
            Description:  f.Description,
            ActivateText: f.ActivateText,
        }
        if f.Archetype != "" {
            archetypeFeatures = append(archetypeFeatures, entry)
        } else {
            jobFeatures = append(jobFeatures, entry)
        }
    }

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_ClassFeaturesResponse{
            ClassFeaturesResponse: &gamev1.ClassFeaturesResponse{
                ArchetypeFeatures: archetypeFeatures,
                JobFeatures:       jobFeatures,
            },
        },
    }, nil
}
```

Update all `NewGameServiceServer` call sites to pass `nil, nil, nil` for the new params (in tests).

#### CMD-7: Add renderer and game_bridge case

In `internal/frontend/handlers/text_renderer.go`, add:

```go
// RenderClassFeaturesResponse formats a ClassFeaturesResponse for display.
func RenderClassFeaturesResponse(resp *gamev1.ClassFeaturesResponse) string {
    var sb strings.Builder
    sb.WriteString(telnet.Colorize(telnet.BrightYellow, "=== Class Features ===\r\n"))

    if len(resp.ArchetypeFeatures) > 0 {
        sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nArchetype Features:\r\n"))
        for _, f := range resp.ArchetypeFeatures {
            activeTag := ""
            if f.Active {
                activeTag = telnet.Colorize(telnet.Green, " [active]")
            }
            sb.WriteString(fmt.Sprintf("  %s%s%s%s\r\n    %s\r\n",
                telnet.BrightWhite, f.Name, telnet.Reset, activeTag, f.Description))
        }
    }

    if len(resp.JobFeatures) > 0 {
        sb.WriteString(telnet.Colorize(telnet.BrightCyan, "\r\nJob Features:\r\n"))
        for _, f := range resp.JobFeatures {
            activeTag := ""
            if f.Active {
                activeTag = telnet.Colorize(telnet.Green, " [active]")
            }
            sb.WriteString(fmt.Sprintf("  %s%s%s%s\r\n    %s\r\n",
                telnet.BrightWhite, f.Name, telnet.Reset, activeTag, f.Description))
        }
    }

    if len(resp.ArchetypeFeatures) == 0 && len(resp.JobFeatures) == 0 {
        sb.WriteString(telnet.Colorize(telnet.Dim, "  No class features assigned.\r\n"))
    }
    return sb.String()
}
```

In `internal/frontend/handlers/game_bridge.go`, add to the response switch:

```go
case *gamev1.ServerEvent_ClassFeaturesResponse:
    _ = conn.Write([]byte(RenderClassFeaturesResponse(payload.ClassFeaturesResponse)))
    return nil
```

**Step after all CMD steps: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok|---" | tail -30
```
Expected: all PASS

**Commit:**

```bash
git add api/proto/ internal/game/command/ internal/frontend/handlers/ internal/gameserver/grpc_service.go
git commit -m "feat: add class_features command end-to-end (CMD-1 through CMD-7)"
```

---

### Task 10: Extend `use` command to include active class features

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleUse function only)

**Step 1: Write a failing test**

In `internal/gameserver/grpc_service_test.go` (or a new test file), add a test verifying that `handleUse` also returns active class features when `characterClassFeaturesRepo` is populated. Look at existing `handleFeats` test patterns to match.

The test should confirm that when a character has an active class feature (e.g., `brutal_surge`) and no feat, the `UseResponse.Choices` includes `brutal_surge`.

**Step 2: Modify `handleUse`**

Replace the current `handleUse` body (which only checks `characterFeatsRepo`) with one that checks both repos:

```go
func (s *GameServiceServer) handleUse(uid, abilityID string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }

    ctx := context.Background()

    // Collect active feats
    var active []*gamev1.FeatEntry
    if s.characterFeatsRepo != nil && s.featRegistry != nil {
        featIDs, err := s.characterFeatsRepo.GetAll(ctx, sess.CharacterID)
        if err != nil {
            return nil, fmt.Errorf("getting feats for use: %w", err)
        }
        for _, id := range featIDs {
            f, ok := s.featRegistry.Feat(id)
            if ok && f.Active {
                active = append(active, &gamev1.FeatEntry{
                    FeatId: f.ID, Name: f.Name, Category: f.Category,
                    Active: f.Active, Description: f.Description, ActivateText: f.ActivateText,
                })
            }
        }
    }

    // Collect active class features (appended to same list as feats for use)
    if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
        cfIDs, err := s.characterClassFeaturesRepo.GetAll(ctx, sess.CharacterID)
        if err != nil {
            return nil, fmt.Errorf("getting class features for use: %w", err)
        }
        for _, id := range cfIDs {
            cf, ok := s.classFeatureRegistry.ClassFeature(id)
            if ok && cf.Active {
                active = append(active, &gamev1.FeatEntry{
                    FeatId: cf.ID, Name: cf.Name, Category: "class_feature",
                    Active: cf.Active, Description: cf.Description, ActivateText: cf.ActivateText,
                })
            }
        }
    }

    if s.characterFeatsRepo == nil && s.characterClassFeaturesRepo == nil {
        return messageEvent("Ability data is not available."), nil
    }

    if abilityID == "" {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_UseResponse{
                UseResponse: &gamev1.UseResponse{Choices: active},
            },
        }, nil
    }

    // Activate the named ability (feat or class feature).
    for _, entry := range active {
        if strings.EqualFold(entry.FeatId, abilityID) || strings.EqualFold(entry.Name, abilityID) {
            return &gamev1.ServerEvent{
                Payload: &gamev1.ServerEvent_UseResponse{
                    UseResponse: &gamev1.UseResponse{Message: entry.ActivateText},
                },
            }, nil
        }
    }
    return messageEvent(fmt.Sprintf("You don't have an active ability named %q.", abilityID)), nil
}
```

**Step 3: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleUse -v 2>&1
```
Expected: PASS

**Step 4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | tail -20
```
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat: extend use command to include active class features"
```

---

### Task 11: Wire into `main.go` flags and Dockerfiles

**Files:**
- Modify: `cmd/frontend/main.go`
- Modify: `cmd/gameserver/main.go`
- Modify: `cmd/devserver/main.go` (if it wires NewAuthHandler/NewGameServiceServer)
- Modify: `deployments/docker/Dockerfile.frontend`
- Modify: `deployments/docker/Dockerfile.gameserver`

#### `cmd/frontend/main.go`

Add flag:
```go
classFeatsFile := flag.String("class-features", "content/class_features.yaml", "path to class features YAML file")
```

After loading feats, load class features:
```go
classFeatures, err := ruleset.LoadClassFeatures(*classFeatsFile)
if err != nil {
    logger.Fatal("loading class features", zap.Error(err))
}
logger.Info("class features loaded", zap.Int("class_features", len(classFeatures)))
```

Add repository:
```go
characterClassFeaturesRepo := postgres.NewCharacterClassFeaturesRepository(pool.DB())
```

Update `NewAuthHandler` call — replace the trailing `feats, characterFeatsRepo` with:
```go
feats, characterFeatsRepo, classFeatures, characterClassFeaturesRepo
```

#### `cmd/gameserver/main.go`

Add flag:
```go
classFeatsFile := flag.String("class-features", "content/class_features.yaml", "path to class features YAML file")
```

After loading feats, load class features:
```go
classFeatures, err := ruleset.LoadClassFeatures(*classFeatsFile)
if err != nil {
    logger.Fatal("loading class features", zap.Error(err))
}
cfReg := ruleset.NewClassFeatureRegistry(classFeatures)
logger.Info("class features loaded", zap.Int("class_features", len(classFeatures)))
```

Add repository:
```go
characterClassFeaturesRepo := postgres.NewCharacterClassFeaturesRepository(pool.DB())
```

Update `NewGameServiceServer` call — add at the end:
```go
classFeatures, cfReg, characterClassFeaturesRepo,
```

#### `cmd/devserver/main.go`

Read and update similarly to frontend/gameserver (add flag, load, pass to constructors).

#### Dockerfiles

`deployments/docker/Dockerfile.frontend` CMD — append:
```
"-class-features", "/content/class_features.yaml"
```

`deployments/docker/Dockerfile.gameserver` CMD — append:
```
"-class-features", "/content/class_features.yaml"
```

**Build test:**

```bash
cd /home/cjohannsen/src/mud && go build ./cmd/frontend/... ./cmd/gameserver/... 2>&1
```
Expected: clean build

**Full test suite:**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok" | tail -20
```
Expected: all PASS

**Commit:**

```bash
git add cmd/ deployments/
git commit -m "feat: wire class features into main.go flags and Dockerfiles"
```

---

### Task 12: Deploy

**Step 1: Run the full test suite one final time**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok"
```
Expected: all packages pass, zero FAIL lines

**Step 2: Verify the build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: clean

**Step 3: Deploy**

```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy 2>&1
```
Expected: images built, pushed, pods rolling-restarted, DB migration applied

**Step 4: Smoke test in-game**

After deploy, log in and run:
```
class_features
```
Expected output: lists archetype features and the job-specific feature for your character's job.

```
use
```
Expected output: lists all active feats AND active class features (e.g., brutal_surge for aggressor characters).

```
use brutal_surge
```
Expected output: "The red haze drops and you move on pure instinct."

**Step 5: Commit (if any post-deploy fixes were needed)**

```bash
git add -p
git commit -m "fix: post-deploy class features corrections"
```

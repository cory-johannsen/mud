# Feats System — Stage 2 Design

**Date:** 2026-03-03
**Scope:** All three feat categories at level 1 — general, skill, and job feats. Active feat
activation via `use <feat>` (placeholder effects). `interact` replaces current `use` for room
equipment.

---

## Feat Categories

| Category | Source | Selection |
|----------|--------|-----------|
| general  | Shared pool | Player picks N (count from job definition) |
| skill    | Tied to a trained skill | Player picks 1 from feats unlocked by trained skills |
| job      | Archetype pool | Job defines fixed grants + optional choices pool |

---

## Skill → Feat Mapping

Each Gunchete skill unlocks a pool of skill feats. Players pick 1 skill feat at creation from
the union of pools for all their trained skills.

### Parkour (Acrobatics)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_parkour | Steady Parkour | Assurance | false | Treat Parkour rolls as 10; ignore circumstance/status penalties. |
| fall_breaker | Fall Breaker | Cat Fall | false | Treat falls as 10 feet shorter when you land rolling. |
| squeeze_through | Squeeze Through | Quick Squeeze | true | Move through tight spaces at full speed without slowing. |
| street_footing | Street Footing | Steady Balance | false | Ignore unstable ground penalties; stand from prone without drawing reactions. |

### Ghosting (Stealth)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_ghosting | Steady Ghosting | Assurance | false | Treat Ghosting rolls as 10. |
| clean_pass | Clean Pass | Experienced Smuggler | false | +2 to conceal items; can hide objects in plain sight on your person. |
| zone_stalker | Zone Stalker | Terrain Stalker | false | Choose one terrain type (rubble, sewer, crowd); move through it silently without a roll. |

### Grift (Thievery)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_grift | Steady Grift | Assurance | false | Treat Grift rolls as 10. |
| pickpocket | Pickpocket | Pickpocket | true | Lift items up to light bulk even from alert targets without penalty. |
| clean_lift | Clean Lift | Subtle Theft | false | Bystanders must pass a Perception check to notice a successful steal. |

### Muscle (Athletics)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_muscle | Steady Muscle | Assurance | false | Treat Muscle rolls as 10. |
| combat_climber | Combat Climber | Combat Climber | false | Climb without going flat-footed; use a hand holding an item for climbing. |
| pack_mule | Pack Mule | Hefty Hauler | false | Your carry weight limit increases by 2 Bulk. |
| quick_jump | Quick Jump | Quick Jump | true | High Jump or Long Jump as a single action instead of two. |
| size_up | Size Up | Titan Wrestler | false | Disarm, Grapple, Shove, or Trip creatures up to two sizes larger than you. |
| waterproof | Waterproof | Underwater Marauder | false | No penalty for weapon use underwater; not flat-footed while submerged. |

### Tech Lore (Arcana)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_tech_lore | Steady Tech Lore | Assurance | false | Treat Tech Lore rolls as 10. |
| tech_sense | Tech Sense | Arcane Sense | true | Detect active electronics, transmitters, or powered devices nearby at will. |

### Rigging (Crafting)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_rigging | Steady Rigging | Assurance | false | Treat Rigging rolls as 10. |
| chem_crafting | Chem Crafting | Alchemical Crafting | true | Craft drugs, poisons, and chemical items; start with four common formulas. |
| quick_fix | Quick Fix | Quick Repair | true | Repair gear in 1 minute instead of 10; at higher skill in a single action. |
| trap_crafting | Trap Crafting | Snare Crafting | true | Craft mechanical traps; start with four common trap formulas. |
| specialty_work | Specialty Work | Specialty Crafting | false | Choose a craft type; +1 bonus to that type, improving with proficiency. |

### Conspiracy (Occultism)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_conspiracy | Steady Conspiracy | Assurance | false | Treat Conspiracy rolls as 10. |
| anomaly_id | Anomaly ID | Oddity Identification | false | +2 to Recall Knowledge about cover-ups, weird phenomena, and shadow organizations. |

### Factions (Society)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_factions | Steady Factions | Assurance | false | Treat Factions rolls as 10. |
| faction_protocol | Faction Protocol | Courtly Graces | true | Use Factions instead of Smooth Talk to make an impression on faction leaders. |
| multilingual | Multilingual | Multilingual | false | Learn two additional street dialects or languages. |
| read_lips | Read Lips | Read Lips | false | Lip-read anyone you can see, even through a window or at a distance. |
| street_signs | Street Signs | Sign Language | false | Know gang sign language for your territory plus one additional crew's signs. |
| streetwise | Streetwise | Streetwise | true | Use Factions instead of Smooth Talk to gather information in urban areas. |

### Intel (Lore)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_intel | Steady Intel | Assurance | false | Treat Intel rolls as 10. |
| field_intel | Field Intel | Additional Lore | false | Add one additional Intel specialty area as a trained knowledge. |
| street_cred | Street Cred | Experienced Professional | false | Earn income via Intel; on a critical failure you earn the partial amount instead of nothing. |

### Patch Job (Medicine)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_patch_job | Steady Patch Job | Assurance | false | Treat Patch Job rolls as 10. |
| combat_patch | Combat Patch | Battle Medicine | true | Apply first aid in a single action without a kit; target immune for 1 day. |

### Wasteland (Nature)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_wasteland | Steady Wasteland | Assurance | false | Treat Wasteland rolls as 10. |
| wasteland_remedy | Wasteland Remedy | Natural Medicine | true | Use Wasteland instead of Patch Job to treat wounds with found materials. |
| animal_bond | Animal Bond | Train Animal | true | Teach a non-magical animal up to 2 commands or tricks. |

### Gang Codes (Religion)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_gang_codes | Steady Gang Codes | Assurance | false | Treat Gang Codes rolls as 10. |
| code_scholar | Code Scholar | Student of the Canon | false | Never critically fail Gang Codes recall checks; +2 to religious/ritual knowledge. |

### Scavenging (Survival)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_scavenging | Steady Scavenging | Assurance | false | Treat Scavenging rolls as 10. |
| trail_reader | Trail Reader | Experienced Tracker | false | Track without moving at half speed; follow trails while moving at normal pace. |
| scavengers_eye | Scavenger's Eye | Forager | true | Subsist yourself and allies equal to your Scavenging modifier in the ruins. |
| zone_survey | Zone Survey | Survey Wildlife | true | 10-minute survey of an area reveals threats, resources, and hazards. |
| zone_expertise | Zone Expertise | Terrain Expertise | false | Choose a terrain (ruins, sewers, industrial); +1 to Scavenging checks there. |

### Hustle (Deception)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_hustle | Steady Hustle | Assurance | false | Treat Hustle rolls as 10. |
| silver_tongue | Silver Tongue | Charming Liar | false | Critical success on a lie makes the target friendly or helpful toward you. |
| long_con | Long Con | Lengthy Diversion | false | A diversion you create lasts until end of your next turn rather than a moment. |
| bs_detector | BS Detector | Lie to Me | true | When someone lies to you, use Hustle instead of Perception to detect it. |

### Smooth Talk (Diplomacy)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_smooth_talk | Steady Smooth Talk | Assurance | false | Treat Smooth Talk rolls as 10. |
| deal_finder | Deal Finder | Bargain Hunter | false | When earning income via Smooth Talk, find discounts and special items for sale. |
| crowd_work | Crowd Work | Group Impression | true | Make an impression on up to 10 people at once with a single check. |
| street_networker | Street Networker | Hobnobber | true | Gather information in half the normal time; critical failure still yields partial info. |

### Hard Look (Intimidation)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_hard_look | Steady Hard Look | Assurance | false | Treat Hard Look rolls as 10. |
| mass_intimidation | Mass Intimidation | Group Coercion | true | Coerce up to 5 people at once with a single check. |
| death_stare | Death Stare | Intimidating Glare | true | Demoralize a target with a look alone; no words required. |
| fast_threat | Fast Threat | Quick Coercion | true | Coerce someone with only one exchange of words rather than a minute. |

### Rep (Performance)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| assurance_rep | Steady Rep | Assurance | false | Treat Rep rolls as 10. |
| crowd_control | Crowd Control | Fascinating Performance | true | Perform to hold up to 4 people's attention, giving them –2 Perception vs. others. |
| rep_play | Rep Play | Impressive Performance | true | Use Rep instead of Smooth Talk to make an impression. |
| signature_style | Signature Style | Virtuosic Performer | false | Choose a Rep specialty (music, street art, combat style); +1 to that type. |

---

## General Feats

Shared pool available to all characters. Job defines how many the player picks.

| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| toughness | Toughness | Toughness | false | Max HP increases by your level; recovery checks are easier. |
| fleet | Fleet | Fleet | false | Your movement speed increases by 5 feet. |
| hair_trigger | Hair Trigger | Incredible Initiative | false | +2 to initiative rolls. |
| armor_training | Armor Training | Armor Proficiency | false | Gain proficiency in one additional armor category. |
| weapon_training | Weapon Training | Weapon Proficiency | false | Gain proficiency with one weapon group or specific weapon. |
| block | Block | Shield Block | true | When hit, use your reaction to reduce damage with a raised shield. |
| hard_to_kill | Hard to Kill | Diehard | false | You die at dying 5 instead of dying 4. |
| quick_recovery | Quick Recovery | Fast Recovery | false | Recover twice as fast; rest restores bonus HP equal to your Grit modifier. |
| iron_lungs | Iron Lungs | Breath Control | false | Hold your breath 25x longer; bonus to saves vs. gas and inhaled toxins. |
| sharpened_edge | Sharpened Edge | Canny Acumen | false | Increase your proficiency rank in Fortitude, Reflex, Will, or Perception. |
| light_step | Light Step | Feather Step | true | Step into difficult terrain (rubble, debris) as normal movement. |

---

## Job Feats by Archetype

Job feats are defined at the archetype level. Each job's `feats` block references fixed grants
and choices from its archetype pool. Jobs may also grant feats cross-archetype where lore demands.

### Aggressor Archetype (Fighter / Barbarian / Gunslinger)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| brutal_charge | Brutal Charge | Sudden Charge | true | Advance twice and make a melee strike as a single sequence. |
| reactive_block | Reactive Block | Reactive Shield | true | When hit by a physical attack, raise your shield as a reaction to reduce damage. |
| overpower | Overpower | Vicious Swing | true | Wind up a two-handed strike dealing two extra damage dice. |
| snap_shot | Snap Shot | Exacting Strike | true | A missed attack counts as only one hit for multi-attack penalty purposes. |
| adrenaline_surge | Adrenaline Surge | Moment of Clarity | true | Once per rage, use a focus action that normally can't be used while enraged. |
| raging_threat | Raging Threat | Raging Intimidation | false | Demoralize enemies while in a rage; your rage itself counts as a threat. |
| cover_fire | Cover Fire | Cover Fire | true | Fire at a target to give an ally +2 AC against that target's next ranged attack. |
| dual_draw | Dual Draw | Dual-Weapon Reload | true | Reload a one-handed firearm and simultaneously draw a second weapon. |
| combat_read | Combat Read | Combat Assessment | true | Strike and simultaneously attempt to read the target's weaknesses on a hit. |
| hit_the_dirt | Hit the Dirt | Hit the Dirt! | true | Drop prone as a reaction when targeted by ranged fire; gain +2 AC against that attack. |

### Criminal Archetype (Rogue / Swashbuckler)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| quick_dodge | Quick Dodge | Nimble Dodge | true | When targeted by an attack, use your reaction to gain +2 AC. |
| trap_eye | Trap Eye | Trap Finder | false | +1 to spot traps; bonus to AC and saves against traps; passively notice traps. |
| twin_strike | Twin Strike | Twin Feint | true | Two quick strikes with different weapons; second attack gains off-guard bonus automatically. |
| youre_next | You're Next | You're Next | true | When you drop an enemy to 0 HP, immediately demoralize a nearby enemy as a reaction. |
| overextend | Overextend | Overextending Feint | false | A successful feint leaves the target off-guard against all your attacks this turn. |
| tumble_behind | Tumble Behind | Tumble Behind | false | Tumbling through a foe's space leaves them off-guard to your next attack. |
| plant_evidence | Plant Evidence | Plant Evidence | true | Plant an item on a target without being detected. |
| dueling_guard | Dueling Guard | Dueling Parry | true | Use an action to gain +2 AC until next turn when fighting with one melee weapon. |
| flying_blade | Flying Blade | Flying Blade | false | Thrown melee weapons apply your precision bonus as if you were in melee. |

### Drifter Archetype (Ranger / Investigator)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| mark_target | Mark Target | Hunt Prey | true | Designate a target; gain +1 to all checks against that target for 1 day. |
| rapid_shot | Rapid Shot | Hunted Shot | true | Fire two ranged attacks at your marked target as a single action. |
| field_study | Field Study | Monster Hunter | true | As part of marking a target, attempt a Recall Knowledge check about them. |
| known_weakness | Known Weakness | Known Weakness | false | Successful Recall Knowledge also reveals one weakness; critical reveals two. |
| share_intel | Share Intel | Clue Them All In | true | Share your Mark Target bonus with up to 5 allies. |
| animal_companion_drifter | Animal Companion | Animal Companion | false | Gain a young animal companion that assists in the field. |

### Influencer Archetype (Bard / Sorcerer)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| street_knowledge | Street Knowledge | Bardic Lore | true | Use your Rep modifier for any Recall Knowledge check regardless of skill. |
| sustained_presence | Sustained Presence | Lingering Composition | true | Extend a performance or social effect by 1 round on a success, 3 on a critical success. |
| crowd_performer | Crowd Performer | Versatile Performance | true | Use Rep in place of Smooth Talk, Hard Look, or Hustle for certain social actions. |
| raw_intensity | Raw Intensity | Dangerous Sorcery | false | Damage-dealing social or psychological attacks deal bonus damage equal to their intensity rank. |
| reach_influence | Reach Influence | Reach Spell | true | Spend 1 extra action to extend the reach or range of a social or area action. |

### Nerd Archetype (Wizard / Inventor / Alchemist)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| quick_bomb | Quick Bomb | Quick Bomber | true | Draw an explosive or chemical item and immediately use it as one combined action. |
| field_identify | Field Identify | Alchemical Savant | true | Identify a tech or chemical item you're holding in a single action. |
| tamper | Tamper | Tamper | true | Sabotage a foe's weapon or armor; –2 to attack or AC until they clear it. |
| jury_rig | Jury Rig | Haphazard Repair | true | Attempt an emergency repair in combat with a DC 15 Rigging check. |
| research_focus | Research Focus | Spellbook Prodigy | false | Learn new technical knowledge in half the time; critical successes compound the benefit. |
| variable_load | Variable Load | Variable Core | false | During daily prep, switch your primary explosive or tech damage type. |

### Normie Archetype (Cleric / Monk / Thaumaturge)
| ID | Name | P2FE | Active | Description |
|----|------|------|--------|-------------|
| defensive_stance | Defensive Stance | Crane Stance | true | Enter a stance granting +1 AC and access to precise, fast unarmed strikes. |
| iron_fist | Iron Fist | Dragon Stance | true | Enter a stance enabling powerful, forceful unarmed strikes (d10). |
| stunning_strike | Stunning Strike | Stunning Fist | true | After a flurry of strikes, if one lands, force a Fortitude save or stun the target. |
| dirty_strike | Dirty Strike | Instructive Strike | true | Strike and simultaneously attempt to learn one weakness or vulnerability on a hit. |
| read_the_room | Read the Room | Diverse Lore | false | Take a –2 penalty to attempt any Recall Knowledge check outside your expertise. |
| root_to_life | Root to Life | Root to Life | true | Stabilize a dying ally in 1 action; 2 actions also clears persistent damage. |

---

## Data Model

### `content/feats.yaml`

```yaml
feats:
  - id: toughness
    name: Toughness
    category: general
    pf2e: toughness
    active: false
    activate_text: ""
    description: "Max HP increases by your level; recovery checks are easier."

  - id: combat_patch
    name: Combat Patch
    category: skill
    skill: patch_job
    pf2e: battle_medicine
    active: true
    activate_text: "You slap a patch on the wound and keep moving."
    description: "Apply first aid as a single action without a kit."

  - id: quick_dodge
    name: Quick Dodge
    category: job
    archetype: criminal
    pf2e: nimble_dodge
    active: true
    activate_text: "You twist out of the way at the last moment."
    description: "When targeted by an attack, use your reaction to gain +2 AC."
```

### `Feat` Go struct (`internal/game/ruleset/feat.go`)

```go
type Feat struct {
    ID           string `yaml:"id"`
    Name         string `yaml:"name"`
    Category     string `yaml:"category"`     // "general", "skill", "job"
    Skill        string `yaml:"skill"`         // skill feat: which skill unlocks it
    Archetype    string `yaml:"archetype"`     // job feat: which archetype owns it
    PF2E         string `yaml:"pf2e"`
    Active       bool   `yaml:"active"`
    ActivateText string `yaml:"activate_text"`
    Description  string `yaml:"description"`
}
```

### Job YAML Extension

```yaml
feats:
  general_count: 1
  fixed:
    - quick_dodge
  choices:
    pool: [twin_strike, youre_next, tumble_behind]
    count: 1
```

### DB Schema (`migrations/012_character_feats.up.sql`)

```sql
CREATE TABLE character_feats (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feat_id      TEXT NOT NULL,
    PRIMARY KEY (character_id, feat_id)
);
```

### Character Model

```go
Feats []string  // feat IDs, order of acquisition
```

---

## Character Creation Flow

After the skills step, a new "Step 2: Feats" section runs:

1. Fixed job feats announced (no choice)
2. Job feat choices — sequential single-pick from job's choices pool (if any)
3. General feat picks — sequential single-pick from shared general pool, `general_count` times
4. Skill feat pick — 1 sequential single-pick from feats unlocked by the character's trained skills

All four steps display feat name + description, same UI pattern as skill selection.

Backfill: on first login after deploy, if `character_feats` has no rows, the same flow runs
before world entry (same pattern as skill backfill in `ensureSkills`).

---

## Commands

### `feats` (CMD-1 through CMD-7)

Displays all character feats grouped by category. Active feats marked with `[active]`.

```
=== Feats ===

General:
  Hard to Kill     It takes more to put you down for good.
  Hair Trigger     +2 to initiative rolls.

Skill:
  Combat Patch     [active] Apply first aid in a single action without a kit.

Job:
  Quick Dodge      [active] React to an incoming attack with a sudden sidestep.
  Twin Strike      [active] Two fast strikes; second one automatically catches them off-guard.
```

### `use` — repurposed for feat activation

Current `use` (room equipment) is replaced by `interact`. The `use` command activates feats.

```
> use quick dodge
You twist out of the way at the last moment.

> use
Active feats:
  1. Combat Patch   - Apply first aid in a single action without a kit.
  2. Quick Dodge    - React to an incoming attack with a sudden sidestep.
Select feat [1-2]: 1
You slap a patch on the wound and keep moving.
```

No mechanical game effect in Stage 2 — output is `activate_text` from the feat definition.

### `interact` — replaces current `use` for room equipment

Replaces `use` 1:1. Same behavior as the old `use` command.

---

## Files Changed

| File | Change |
|------|--------|
| `content/feats.yaml` | New — all level-1 feat definitions |
| `content/jobs/*.yaml` | All 76 — add `feats` block |
| `internal/game/ruleset/feat.go` | New — `Feat` type, `LoadFeats()`, `FeatRegistry` |
| `internal/game/ruleset/job.go` | Extend `Job` with `FeatGrants` struct |
| `internal/game/character/model.go` | Add `Feats []string` |
| `internal/game/character/builder.go` | Add `BuildFeatsFromJob()` |
| `migrations/012_character_feats.up.sql` | New |
| `migrations/012_character_feats.down.sql` | New |
| `internal/storage/postgres/character_feats.go` | New — `CharacterFeatsRepository` |
| `internal/game/command/commands.go` | Add `HandlerFeats`, `HandlerUse`, `HandlerInteract`; rename existing `HandlerUseEquipment` |
| `internal/game/command/feats.go` | New — `HandleFeats` |
| `internal/game/command/use.go` | New — `HandleUse` |
| `internal/game/command/interact.go` | New — `HandleInteract` (replaces old `use` behavior) |
| `api/proto/game/v1/game.proto` | Add `FeatsRequest/Response`, `UseRequest/Response`, `InteractRequest/Response` |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeFeats`, `bridgeUse`, `bridgeInteract` |
| `internal/gameserver/grpc_service.go` | Add `handleFeats`, `handleUse`, `handleInteract` |
| `internal/frontend/handlers/character_flow.go` | Add feat selection step (Step 2: Feats) |
| `internal/frontend/handlers/text_renderer.go` | Add `RenderFeatsResponse`, `RenderUseResponse` |
| `cmd/frontend/main.go` | Add `-feats` flag, load feats, pass to handler |
| `cmd/gameserver/main.go` | Add `-feats` flag, load feats, wire repo |
| `deployments/docker/Dockerfile.frontend` | Add `-feats /content/feats.yaml` to CMD |
| `deployments/docker/Dockerfile.gameserver` | Add `-feats /content/feats.yaml` to CMD |

---

## Out of Scope for Stage 2

- Abilities / class features (Stage 3)
- Mechanical effects for active feats (future)
- Feat prerequisites and feat chains (future)
- Feat increases on level-up (future)
- `abilities` command (Stage 3)

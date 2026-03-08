# PF2E Actions Design

## Goal

Implement the PF2E actions the game mechanics currently support as standalone commands, and stub the rest as FEATURES.md entries organized under new categories.

## Architecture

Five new standalone commands via the full CMD-1 through CMD-7 pipeline. Four new condition YAML files. One task to populate FEATURES.md stubs for all remaining PF2E actions, organized under new feature categories.

---

## Section 1: The Five Implemented Commands

| Command | Syntax | Requires | AP Cost | Effect |
|---------|--------|----------|---------|--------|
| raise_shield | `raise shield` | Shield equipped | 1 | Apply `shield_raised` (+2 AC) until start of next turn |
| take_cover | `cover` | — | 1 | Apply `in_cover` (+2 AC, duration: encounter) |
| first_aid | `aid` | — | 2 | patch_job skill check DC 15; success → heal 2d8+4 HP (self) |
| feint | `feint` | In combat | 1 | grift skill check vs target Perception DC; success → apply `flat_footed` (−2 AC) to target for 1 round |
| demoralize | `demoralize` | In combat | 1 | smooth_talk skill check vs target Will DC; success → apply `frightened` (−1 attack, −1 AC) to target for encounter |

`raise_shield` requires a shield in the equipped loadout. Validation in the handler: check equipped items for shield type; return error message if none found.

`first_aid` and `feint` and `demoralize` use the existing skill check resolution path (1d20 + skill mod vs DC).

---

## Section 2: New Conditions

Four new YAML files in `content/conditions/`:

**shield_raised.yaml**
- `ac_bonus: 2`, `duration_type: round`, `max_stacks: 0`

**in_cover.yaml**
- `ac_bonus: 2`, `duration_type: encounter`, `max_stacks: 0`

**flat_footed.yaml**
- `ac_penalty: 2` (applied to the NPC target), `duration_type: round`, `max_stacks: 0`

**frightened.yaml**
- `attack_penalty: 1`, `ac_penalty: 1`, `duration_type: encounter`, `max_stacks: 3`

---

## Section 3: FEATURES.md Stubs

New categories and stub entries for all remaining PF2E actions:

### Combat > Athletics Actions
- Grapple — opposed Athletics check; apply Grabbed condition
- Shove — Athletics vs Fortitude DC; push target 5 ft
- Trip — Athletics vs Reflex DC; apply Prone condition
- Disarm — Athletics vs Reflex DC; knock weapon from target
- Climb — Athletics check vs surface DC; vertical movement
- Swim — Athletics check vs current DC; water movement

### Combat > Tactical Actions
- Step — move 5 ft without triggering reactions
- Seek — Perception check to detect hidden creatures/objects
- Sense Motive — Perception vs Deception to detect lies/intent
- Escape — Athletics/Acrobatics vs grappler DC to break free
- Delay — forfeit initiative position to act later in round

### Combat > Stealth & Deception Actions
- Hide — Stealth vs Perception DC; become Hidden
- Sneak — Stealth vs Perception DC; move while Hidden
- Create Diversion — Deception vs Perception DC; become Hidden briefly
- Tumble Through — Acrobatics vs Reflex DC; move through enemy space

### General Actions
- Aid — assist ally; skill check to grant +2 circumstance bonus
- Ready — prepare a reaction to trigger on a specified condition
- Hero Point — spend hero point to reroll a check or avoid death

### Exploration System
- Avoid Notice — Stealth to avoid detection while exploring
- Defend — raise shield or take cover posture during exploration
- Detect Magic — Arcana/Occultism to sense magical auras
- Search — Perception to find hidden objects/creatures/traps
- Scout — move ahead and report threats to party
- Follow the Expert — follow a skilled ally to gain their bonus
- Investigate — identify clues and piece together information
- Refocus — spend 10 minutes to restore a Focus Point

### Downtime System
- Earn Income — skill check to earn credits over downtime period
- Craft — craft an item from components over downtime period
- Treat Disease — Medicine check to reduce disease severity
- Subsist — Survival/Society check to find food and shelter
- Create Forgery — Society check to create fake documents
- Long-Term Rest — extended rest to recover HP and conditions
- Retrain (extend) — extend TrainSkill to support feat/archetype retraining

### Gear System
- Repair — Crafting check to restore a broken item to function
- Affix a Precious Material — attach material to weapon/armor
- Swap — swap equipped weapon or armor without penalty

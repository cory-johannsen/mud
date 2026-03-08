# Actions Content Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add active (and missing passive) class features for every archetype and job so every player character has at least one activatable action.

**Architecture:** Pure YAML content changes — no Go code required. Conditions go in `content/conditions/` (one file per condition). Class features go in `content/class_features.yaml`. Tests in `internal/game/ruleset/class_feature_test.go` and `internal/game/condition/definition_test.go` must be updated to reflect new counts.

**Tech Stack:** YAML, Go test suite (`go test ./...`)

---

## Background

Current state of `content/class_features.yaml`:
- 88 total features (verified by `TestLoadClassFeatures_Count`)
- 6 archetypes: aggressor, criminal, drifter, influencer, nerd, normie (no naturalist/schemer/zealot archetype-level features)
- 23 jobs already have active actions; 52 jobs need them

Existing condition files: `content/conditions/` directory with `brutal_surge_active.yaml`, `command_attention_active.yaml`, `distrusted.yaml`, `dying.yaml`, `flat_footed.yaml`, `frightened.yaml`, `prone.yaml`, `stunned.yaml`, `unconscious.yaml`, `wounded.yaml`.

**Key constraint:** `TestLoadClassFeatures_Count` in `internal/game/ruleset/class_feature_test.go` hardcodes the feature count (currently 88). This test MUST be updated each task to match the new total. Similarly `TestClassFeatureRegistry_ByArchetype` checks archetype feature counts — update when archetype features are added.

---

## Condition YAML format (reference)

```yaml
id: some_condition_active
name: Some Condition Active
description: |
  One sentence describing the mechanical effect.
duration_type: encounter   # or: permanent, rounds, minute
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

## Class feature YAML format (reference)

```yaml
  - id: brutal_surge
    name: Brutal Surge
    archetype: aggressor
    job: ""
    pf2e: rage
    active: true
    shortcut: surge
    action_cost: 1
    contexts:
      - combat
    activate_text: "The red haze drops and you move on pure instinct."
    description: "Enter a combat frenzy: +2 melee damage, -2 AC until end of encounter."
    effect:
      type: condition
      target: self
      condition_id: brutal_surge_active
```

For a passive feature (no effect block):
```yaml
  - id: hardy
    name: Hardy
    archetype: naturalist
    job: ""
    pf2e: ""
    active: false
    activate_text: ""
    description: "+1 Fortitude saves; immune to extreme temperature."
```

For a heal effect:
```yaml
    effect:
      type: heal
      amount: "1d8"
```

For a skill_check effect:
```yaml
    effect:
      type: skill_check
      skill: Reasoning
      dc: 15
```

For a damage effect:
```yaml
    effect:
      type: damage
      amount: "1d8"
      damage_type: bludgeoning
```

---

## Task 1: Add conditions for archetype-level active features

**Files:**
- Create: `content/conditions/ghost_active.yaml`
- Create: `content/conditions/primal_surge_active.yaml`
- Create: `content/conditions/setup_active.yaml`

**Step 1: Verify test currently passes**

```bash
go test ./internal/game/condition/... -v -run TestLoadDirectory
```
Expected: PASS

**Step 2: Create `content/conditions/ghost_active.yaml`**

```yaml
id: ghost_active
name: Ghost Active
description: |
  You melt into the shadows. You gain concealment and a +2 bonus to Stealth
  checks for one minute.
duration_type: minute
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 3: Create `content/conditions/primal_surge_active.yaml`**

```yaml
id: primal_surge_active
name: Primal Surge Active
description: |
  Raw primal power floods your muscles. You gain +2 to Strength-based checks
  until the end of combat.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 2
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 4: Create `content/conditions/setup_active.yaml`**

```yaml
id: setup_active
name: Setup Active
description: |
  You have positioned yourself perfectly. Your next social check is made
  with advantage.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 5: Run tests**

```bash
go test ./internal/game/condition/... -v
```
Expected: all PASS

**Step 6: Commit**

```bash
git add content/conditions/ghost_active.yaml content/conditions/primal_surge_active.yaml content/conditions/setup_active.yaml
git commit -m "content: add conditions for archetype active features (ghost, primal_surge, setup)"
```

---

## Task 2: Add archetype-level class features (6 archetypes)

**Files:**
- Modify: `content/class_features.yaml` (add 9 entries: 6 active + 3 passive)
- Modify: `internal/game/ruleset/class_feature_test.go` (update count from 88 → 97; update ByArchetype checks)

**Step 1: Update `TestLoadClassFeatures_Count` in `internal/game/ruleset/class_feature_test.go`**

Change:
```go
if len(features) != 88 {
    t.Errorf("expected 88 features, got %d", len(features))
}
```
To:
```go
if len(features) != 97 {
    t.Errorf("expected 97 features, got %d", len(features))
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL "expected 97 features, got 88"

**Step 3: Append 9 new features to `content/class_features.yaml`**

Add after the last `nerd` archetype entry and before the first job entry (i.e., before the `# --- Job features ---` comment or before `boot_gun` entry). Insert at the end of the archetype-level section. The exact insertion point: find the line `  - id: formulaic_mind` block and append after it (before the normie block) OR append at end of the archetype section. Simplest: append to end of file before EOF (or at end of archetype block).

Add these 9 entries to the `class_features:` list (at the end of the archetype-level section, before job entries):

```yaml
  # --- criminal archetype active feature ---
  - id: ghost
    name: Ghost
    archetype: criminal
    job: ""
    pf2e: ""
    active: true
    shortcut: ghost
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You become one with the shadows."
    description: "Activate concealment: +2 Stealth, treated as hidden for 1 minute."
    effect:
      type: condition
      target: self
      condition_id: ghost_active

  # --- drifter archetype active feature ---
  - id: mark
    name: Mark
    archetype: drifter
    job: ""
    pf2e: ""
    active: true
    shortcut: mark
    action_cost: 1
    contexts:
      - combat
    activate_text: "You memorize every detail of your quarry."
    description: "Designate a target as marked; you gain +1d4 on attacks against them."
    effect:
      type: condition
      target: self
      condition_id: mark_active

  # --- nerd archetype active feature ---
  - id: exploit
    name: Exploit
    archetype: nerd
    job: ""
    pf2e: ""
    active: true
    shortcut: exploit
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You spot the weak point in their system."
    description: "Reasoning skill check (DC 15) to find a weakness and gain tactical advantage."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 15

  # --- naturalist archetype passive feature ---
  - id: hardy
    name: Hardy
    archetype: naturalist
    job: ""
    pf2e: ""
    active: false
    activate_text: ""
    description: "+1 Fortitude saves; immune to extreme temperature effects."

  # --- naturalist archetype active feature ---
  - id: primal_surge
    name: Primal Surge
    archetype: naturalist
    job: ""
    pf2e: ""
    active: true
    shortcut: primal
    action_cost: 1
    contexts:
      - combat
    activate_text: "The beast within surges to the surface."
    description: "+2 Strength checks and +2 damage until end of combat."
    effect:
      type: condition
      target: self
      condition_id: primal_surge_active

  # --- schemer archetype passive feature ---
  - id: smooth_operator
    name: Smooth Operator
    archetype: schemer
    job: ""
    pf2e: ""
    active: false
    activate_text: ""
    description: "+2 bonus to all Deception checks."

  # --- schemer archetype active feature ---
  - id: setup
    name: Setup
    archetype: schemer
    job: ""
    pf2e: ""
    active: true
    shortcut: setup
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You position yourself for the play."
    description: "Your next social check is made with advantage."
    effect:
      type: condition
      target: self
      condition_id: setup_active

  # --- zealot archetype passive feature ---
  - id: true_believer
    name: True Believer
    archetype: zealot
    job: ""
    pf2e: ""
    active: false
    activate_text: ""
    description: "+2 Will saves; immune to the demoralized condition."

  # --- zealot archetype active feature ---
  - id: lay_hands
    name: Lay Hands
    archetype: zealot
    job: ""
    pf2e: ""
    active: true
    shortcut: hands
    action_cost: 2
    contexts:
      - combat
      - exploration
    activate_text: "Holy warmth flows through your hands."
    description: "Channel healing energy to restore 1d8 hit points."
    effect:
      type: heal
      amount: "1d8"
```

Note: `mark_active` condition is referenced above. Add a condition file for it too. Create `content/conditions/mark_active.yaml`:

```yaml
id: mark_active
name: Mark Active
description: |
  You have identified your quarry's weak points. You gain +1d4 bonus damage
  on attacks against your designated target.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 1
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 4: Run test to verify count passes**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 5: Run full ruleset test suite**

```bash
go test ./internal/game/ruleset/... -v
```
Expected: all PASS (note: `TestClassFeatureRegistry_ByArchetype` checks aggressor=2; naturalist/schemer/zealot don't have assertions yet so won't fail)

**Step 6: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 7: Commit**

```bash
git add content/class_features.yaml content/conditions/mark_active.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add archetype-level active/passive features for criminal, drifter, nerd, naturalist, schemer, zealot"
```

---

## Task 3: Add conditions for aggressor and criminal job active actions

**Files:**
- Create: `content/conditions/roid_rage_active.yaml`
- Create: `content/conditions/suppressive_fire_active.yaml`
- Create: `content/conditions/stash_active.yaml`

(aggressor jobs beat_down_artist, boot_gun, gangster, goon, grunt, muscle, soldier, thug use damage/skill_check — no new conditions; roid_rager uses condition; soldier uses condition; criminal jobs use skill_check except smuggler)

**Step 1: Create `content/conditions/roid_rage_active.yaml`**

```yaml
id: roid_rage_active
name: Roid Rage Active
description: |
  You explode into a chemical fury. +2 melee damage, -1 AC for the duration
  of this encounter.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 1
damage_bonus: 2
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Create `content/conditions/suppressive_fire_active.yaml`**

```yaml
id: suppressive_fire_active
name: Suppressive Fire Active
description: |
  You lay down a hail of covering fire. Enemies in your line of sight must
  spend an extra AP to move until your next turn.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 3: Create `content/conditions/stash_active.yaml`**

```yaml
id: stash_active
name: Stash Active
description: |
  You have expertly concealed your contraband. It will not be detected
  during this scene.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 4: Run condition tests**

```bash
go test ./internal/game/condition/... -v
```
Expected: all PASS

**Step 5: Commit**

```bash
git add content/conditions/roid_rage_active.yaml content/conditions/suppressive_fire_active.yaml content/conditions/stash_active.yaml
git commit -m "content: add conditions for aggressor/criminal job active actions"
```

---

## Task 4: Add aggressor job active actions (9 jobs)

Jobs: beat_down_artist, boot_gun, gangster, goon, grunt, muscle, roid_rager, soldier, thug

**Files:**
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 97 → 106)

**Step 1: Update count in test (97 → 106)**

**Step 2: Run test to verify it fails**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 3: Append 9 features to `content/class_features.yaml`**

```yaml
  # --- aggressor job active features ---
  - id: savage_beatdown
    name: Savage Beatdown
    archetype: ""
    job: beat_down_artist
    pf2e: ""
    active: true
    shortcut: beatdown
    action_cost: 1
    contexts:
      - combat
    activate_text: "You rain blows down like a machine."
    description: "Deal 1d8 bludgeoning damage to a single target."
    effect:
      type: damage
      amount: "1d8"
      damage_type: bludgeoning

  - id: quick_draw_shot
    name: Quick Draw Shot
    archetype: ""
    job: boot_gun
    pf2e: ""
    active: true
    shortcut: qdraw
    action_cost: 1
    contexts:
      - combat
    activate_text: "Your hand moves before your brain even registers the threat."
    description: "Deal 1d6 piercing damage on a snap shot."
    effect:
      type: damage
      amount: "1d6"
      damage_type: piercing

  - id: turf_war_shout
    name: Turf War Shout
    archetype: ""
    job: gangster
    pf2e: ""
    active: true
    shortcut: turf
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You announce your presence and your intentions."
    description: "Presence skill check (DC 14) to intimidate and establish dominance."
    effect:
      type: skill_check
      skill: Presence
      dc: 14

  - id: muscle_through
    name: Muscle Through
    archetype: ""
    job: goon
    pf2e: ""
    active: true
    shortcut: muscle
    action_cost: 1
    contexts:
      - combat
    activate_text: "You barrel forward, shoulder down."
    description: "Deal 1d6 bludgeoning damage and shove target 5 feet."
    effect:
      type: damage
      amount: "1d6"
      damage_type: bludgeoning

  - id: advance_order
    name: Advance
    archetype: ""
    job: grunt
    pf2e: ""
    active: true
    shortcut: advance
    action_cost: 1
    contexts:
      - combat
    activate_text: "You push forward under fire."
    description: "Deal 1d6 piercing damage — your training overrides the fear."
    effect:
      type: damage
      amount: "1d6"
      damage_type: piercing

  - id: flex_intimidate
    name: Flex
    archetype: ""
    job: muscle
    pf2e: ""
    active: true
    shortcut: flex
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You let your body do the talking."
    description: "Presence skill check (DC 13) to intimidate through sheer physical presence."
    effect:
      type: skill_check
      skill: Presence
      dc: 13

  - id: roid_rage
    name: Roid Rage
    archetype: ""
    job: roid_rager
    pf2e: ""
    active: true
    shortcut: rage
    action_cost: 1
    contexts:
      - combat
    activate_text: "The chemicals take over."
    description: "+2 melee damage, -1 AC for the rest of this encounter."
    effect:
      type: condition
      target: self
      condition_id: roid_rage_active

  - id: suppressive_fire
    name: Suppressive Fire
    archetype: ""
    job: soldier
    pf2e: ""
    active: true
    shortcut: suppress
    action_cost: 1
    contexts:
      - combat
    activate_text: "You blanket the area in lead."
    description: "Lay down suppressive fire, forcing enemies to spend extra AP to move."
    effect:
      type: condition
      target: self
      condition_id: suppressive_fire_active

  - id: shakedown
    name: Shakedown
    archetype: ""
    job: thug
    pf2e: ""
    active: true
    shortcut: shake
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You get right up in their face."
    description: "Presence skill check (DC 14) to extort or intimidate a target."
    effect:
      type: skill_check
      skill: Presence
      dc: 14
```

**Step 4: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 5: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 6: Commit**

```bash
git add content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions for 9 aggressor jobs"
```

---

## Task 5: Add criminal job active actions (8 jobs)

Jobs: beggar, car_jacker, contract_killer, hanger_on, hooker, smuggler, thief, tomb_raider

**Files:**
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 106 → 114)

**Step 1: Update count in test (106 → 114)**

**Step 2: Run test to verify it fails**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 3: Append 8 features to `content/class_features.yaml`**

```yaml
  # --- criminal job active features ---
  - id: beg
    name: Beg
    archetype: ""
    job: beggar
    pf2e: ""
    active: true
    shortcut: beg
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You look as pitiful as possible."
    description: "Presence skill check (DC 12) to elicit sympathy and resources."
    effect:
      type: skill_check
      skill: Presence
      dc: 12

  - id: ram
    name: Ram
    archetype: ""
    job: car_jacker
    pf2e: ""
    active: true
    shortcut: ram
    action_cost: 1
    contexts:
      - combat
    activate_text: "You use whatever you can as a battering ram."
    description: "Deal 1d6 bludgeoning damage by ramming or improvised assault."
    effect:
      type: damage
      amount: "1d6"
      damage_type: bludgeoning

  - id: execution_strike
    name: Execution Strike
    archetype: ""
    job: contract_killer
    pf2e: ""
    active: true
    shortcut: execute
    action_cost: 1
    contexts:
      - combat
    activate_text: "Clean. Precise. Professional."
    description: "Deal 1d8 piercing damage — a precisely aimed killing strike."
    effect:
      type: damage
      amount: "1d8"
      damage_type: piercing

  - id: mooch
    name: Mooch
    archetype: ""
    job: hanger_on
    pf2e: ""
    active: true
    shortcut: mooch
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You latch on to whoever looks most generous."
    description: "Presence skill check (DC 13) to obtain free goods or services."
    effect:
      type: skill_check
      skill: Presence
      dc: 13

  - id: honey_trap
    name: Honey Trap
    archetype: ""
    job: hooker
    pf2e: ""
    active: true
    shortcut: honey
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You turn on the charm like a weapon."
    description: "Deception skill check (DC 14) to distract or manipulate a target."
    effect:
      type: skill_check
      skill: Deception
      dc: 14

  - id: stash_it
    name: Stash It
    archetype: ""
    job: smuggler
    pf2e: ""
    active: true
    shortcut: stash
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You know how to hide things where they won't be found."
    description: "Conceal contraband so it goes undetected this scene."
    effect:
      type: condition
      target: self
      condition_id: stash_active

  - id: pickpocket_action
    name: Pickpocket
    archetype: ""
    job: thief
    pf2e: ""
    active: true
    shortcut: pick
    action_cost: 1
    contexts:
      - exploration
    activate_text: "Light fingers, lighter wallet."
    description: "Quickness skill check (DC 14) to lift an item without detection."
    effect:
      type: skill_check
      skill: Quickness
      dc: 14

  - id: loot
    name: Loot
    archetype: ""
    job: tomb_raider
    pf2e: ""
    active: true
    shortcut: loot
    action_cost: 1
    contexts:
      - exploration
    activate_text: "Your eyes sweep the space for anything worth taking."
    description: "Awareness skill check (DC 13) to find valuables in the current location."
    effect:
      type: skill_check
      skill: Awareness
      dc: 13
```

**Step 4: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 5: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 6: Commit**

```bash
git add content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions for 8 criminal jobs"
```

---

## Task 6: Add conditions for drifter job active actions, then add drifter job active actions (8 jobs)

Jobs: bagman, cop, driver, free_spirit, psychopath, scout, stalker, warden

Conditions needed: `arrest_active`, `floored_active`, `shadow_active`, `hold_the_line_active`

**Files:**
- Create: `content/conditions/arrest_active.yaml`
- Create: `content/conditions/floored_active.yaml`
- Create: `content/conditions/shadow_active.yaml`
- Create: `content/conditions/hold_the_line_active.yaml`
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 114 → 122)

**Step 1: Create condition files**

`content/conditions/arrest_active.yaml`:
```yaml
id: arrest_active
name: Arrest Active
description: |
  You have invoked your authority. The target must comply or face escalating
  consequences — treat them as flat-footed until they act against you.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

`content/conditions/floored_active.yaml`:
```yaml
id: floored_active
name: Floored Active
description: |
  You have the pedal to the metal. Your movement speed is doubled until
  your next turn.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: -2
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

`content/conditions/shadow_active.yaml`:
```yaml
id: shadow_active
name: Shadow Active
description: |
  You have blended into the background. You are treated as hidden and gain
  +2 to Stealth checks until you take an aggressive action.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

`content/conditions/hold_the_line_active.yaml`:
```yaml
id: hold_the_line_active
name: Hold the Line Active
description: |
  You have planted your feet and will not be moved. You gain +1 AC and
  cannot be forcibly moved until your next turn.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: -1
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Run condition tests**

```bash
go test ./internal/game/condition/... -v
```
Expected: all PASS

**Step 3: Update count in test (114 → 122)**

**Step 4: Run count test to verify fail**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 5: Append 8 features to `content/class_features.yaml`**

```yaml
  # --- drifter job active features ---
  - id: cash_drop
    name: Cash Drop
    archetype: ""
    job: bagman
    pf2e: ""
    active: true
    shortcut: cash
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You make the handoff like a ghost."
    description: "Presence skill check (DC 13) to complete a transaction discreetly."
    effect:
      type: skill_check
      skill: Presence
      dc: 13

  - id: arrest
    name: Arrest
    archetype: ""
    job: cop
    pf2e: ""
    active: true
    shortcut: arrest
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "Hands where I can see them."
    description: "Invoke authority to flat-foot the target until they act against you."
    effect:
      type: condition
      target: self
      condition_id: arrest_active

  - id: floored
    name: Floored
    archetype: ""
    job: driver
    pf2e: ""
    active: true
    shortcut: floor
    action_cost: 1
    contexts:
      - combat
    activate_text: "You bury the accelerator."
    description: "Double your movement speed until your next turn."
    effect:
      type: condition
      target: self
      condition_id: floored_active

  - id: go_with_the_flow
    name: Go With the Flow
    archetype: ""
    job: free_spirit
    pf2e: ""
    active: true
    shortcut: flow
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You let the universe guide you."
    description: "Awareness skill check (DC 12) to intuit the best path forward."
    effect:
      type: skill_check
      skill: Awareness
      dc: 12

  - id: unnerve
    name: Unnerve
    archetype: ""
    job: psychopath
    pf2e: ""
    active: true
    shortcut: unnerve
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "You let them see what you really are."
    description: "Presence skill check (DC 15) to frighten or destabilize a target."
    effect:
      type: skill_check
      skill: Presence
      dc: 15

  - id: reconnoiter
    name: Reconnoiter
    archetype: ""
    job: scout
    pf2e: ""
    active: true
    shortcut: recon
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You sweep the area with practiced efficiency."
    description: "Awareness skill check (DC 13) to gather tactical information."
    effect:
      type: skill_check
      skill: Awareness
      dc: 13

  - id: shadow_action
    name: Shadow
    archetype: ""
    job: stalker
    pf2e: ""
    active: true
    shortcut: shadow
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You become the darkness."
    description: "Blend into the environment: treated as hidden, +2 Stealth."
    effect:
      type: condition
      target: self
      condition_id: shadow_active

  - id: hold_the_line
    name: Hold the Line
    archetype: ""
    job: warden
    pf2e: ""
    active: true
    shortcut: hold
    action_cost: 1
    contexts:
      - combat
    activate_text: "Nobody gets past you."
    description: "+1 AC and cannot be forcibly moved until your next turn."
    effect:
      type: condition
      target: self
      condition_id: hold_the_line_active
```

**Step 6: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 7: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 8: Commit**

```bash
git add content/conditions/arrest_active.yaml content/conditions/floored_active.yaml content/conditions/shadow_active.yaml content/conditions/hold_the_line_active.yaml content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions and conditions for 8 drifter jobs"
```

---

## Task 7: Add influencer and nerd job active actions (11 jobs)

**Influencer jobs needing active actions (4):** anarchist, antifa, libertarian, schmoozer
**Nerd jobs needing active actions (7):** cooker, detective, grease_monkey, hoarder, journalist, natural_mystic, specialist

No new conditions needed (all skill_check or heal effects).

**Files:**
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 122 → 133)

**Step 1: Update count in test (122 → 133)**

**Step 2: Run test to verify fail**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 3: Append 11 features to `content/class_features.yaml`**

```yaml
  # --- influencer job active features ---
  - id: incite
    name: Incite
    archetype: ""
    job: anarchist
    pf2e: ""
    active: true
    shortcut: incite
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You fan the flames of discontent."
    description: "Presence skill check (DC 14) to rile up a crowd or individual."
    effect:
      type: skill_check
      skill: Presence
      dc: 14

  - id: bloc_up_action
    name: Bloc Up
    archetype: ""
    job: antifa
    pf2e: ""
    active: true
    shortcut: bloc
    action_cost: 1
    contexts:
      - combat
    activate_text: "You form up with whoever's nearby."
    description: "Deal 1d6 bludgeoning damage — strength in numbers."
    effect:
      type: damage
      amount: "1d6"
      damage_type: bludgeoning

  - id: property_rights
    name: Property Rights
    archetype: ""
    job: libertarian
    pf2e: ""
    active: true
    shortcut: rights
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You invoke the sanctity of private property."
    description: "Reasoning skill check (DC 14) to assert a legal or moral argument."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 14

  - id: schmooze
    name: Schmooze
    archetype: ""
    job: schmoozer
    pf2e: ""
    active: true
    shortcut: schmooze
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You slide into the conversation like you own the room."
    description: "Presence skill check (DC 13) to make a favorable impression."
    effect:
      type: skill_check
      skill: Presence
      dc: 13

  # --- nerd job active features ---
  - id: mix
    name: Mix
    archetype: ""
    job: cooker
    pf2e: ""
    active: true
    shortcut: mix
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You run through the formula in your head."
    description: "Reasoning skill check (DC 15) to synthesize a substance or recall a formula."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 15

  - id: deduce
    name: Deduce
    archetype: ""
    job: detective
    pf2e: ""
    active: true
    shortcut: deduce
    action_cost: 1
    contexts:
      - exploration
    activate_text: "The pieces click together."
    description: "Reasoning skill check (DC 15) to draw a conclusion from available evidence."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 15

  - id: quick_fix
    name: Quick Fix
    archetype: ""
    job: grease_monkey
    pf2e: ""
    active: true
    shortcut: qfix
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You slap it together with zip ties and hope."
    description: "Reasoning skill check (DC 13) to jury-rig a repair on the spot."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 13

  - id: dig_out
    name: Dig Out
    archetype: ""
    job: hoarder
    pf2e: ""
    active: true
    shortcut: dig
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You know you have one of these somewhere..."
    description: "Awareness skill check (DC 12) to locate a useful item in your collection."
    effect:
      type: skill_check
      skill: Awareness
      dc: 12

  - id: expose
    name: Expose
    archetype: ""
    job: journalist
    pf2e: ""
    active: true
    shortcut: expose
    action_cost: 1
    contexts:
      - exploration
    activate_text: "The truth, unvarnished."
    description: "Reasoning skill check (DC 15) to uncover or reveal hidden information."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 15

  - id: commune
    name: Commune
    archetype: ""
    job: natural_mystic
    pf2e: ""
    active: true
    shortcut: commune
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You open yourself to the flows of nature."
    description: "Awareness skill check (DC 14) to sense hidden or spiritual information."
    effect:
      type: skill_check
      skill: Awareness
      dc: 14

  - id: expert_analysis
    name: Expert Analysis
    archetype: ""
    job: specialist
    pf2e: ""
    active: true
    shortcut: analyze
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You apply your training to the problem."
    description: "Reasoning skill check (DC 16) to provide a precise professional assessment."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 16
```

**Step 4: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 5: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 6: Commit**

```bash
git add content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions for 4 influencer jobs and 7 nerd jobs"
```

---

## Task 8: Add conditions for naturalist jobs, then add naturalist job active actions (7 jobs)

Jobs: exterminator, freegan, hippie, hobo, laborer, rancher, tracker

Conditions needed: `dig_in_active`

**Files:**
- Create: `content/conditions/dig_in_active.yaml`
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 133 → 140)

**Step 1: Create `content/conditions/dig_in_active.yaml`**

```yaml
id: dig_in_active
name: Dig In Active
description: |
  You plant your feet and brace for impact. You gain +1 AC and resistance
  to being knocked down until your next turn.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: -1
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Run condition tests**

```bash
go test ./internal/game/condition/... -v
```
Expected: all PASS

**Step 3: Update count in test (133 → 140)**

**Step 4: Run count test to verify fail**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 5: Append 7 features to `content/class_features.yaml`**

```yaml
  # --- naturalist job active features ---
  - id: fumigate
    name: Fumigate
    archetype: ""
    job: exterminator
    pf2e: ""
    active: true
    shortcut: fumigate
    action_cost: 1
    contexts:
      - combat
    activate_text: "You deploy your chemicals with practiced efficiency."
    description: "Deal 1d6 poison damage to a target in range."
    effect:
      type: damage
      amount: "1d6"
      damage_type: poison

  - id: scavenge
    name: Scavenge
    archetype: ""
    job: freegan
    pf2e: ""
    active: true
    shortcut: scav
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You see value where others see garbage."
    description: "Awareness skill check (DC 12) to find useful discarded materials."
    effect:
      type: skill_check
      skill: Awareness
      dc: 12

  - id: chill_out
    name: Chill Out
    archetype: ""
    job: hippie
    pf2e: ""
    active: true
    shortcut: chill
    action_cost: 2
    contexts:
      - exploration
    activate_text: "You share something healing with yourself."
    description: "Use herbal knowledge to restore 1d4 hit points."
    effect:
      type: heal
      amount: "1d4"

  - id: lay_low
    name: Lay Low
    archetype: ""
    job: hobo
    pf2e: ""
    active: true
    shortcut: laylow
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You become part of the scenery."
    description: "Awareness skill check (DC 12) to go unnoticed in a public space."
    effect:
      type: skill_check
      skill: Awareness
      dc: 12

  - id: dig_in_action
    name: Dig In
    archetype: ""
    job: laborer
    pf2e: ""
    active: true
    shortcut: digin
    action_cost: 1
    contexts:
      - combat
    activate_text: "You plant your feet and lean into the work."
    description: "+1 AC, cannot be knocked down until your next turn."
    effect:
      type: condition
      target: self
      condition_id: dig_in_active

  - id: wrangle
    name: Wrangle
    archetype: ""
    job: rancher
    pf2e: ""
    active: true
    shortcut: wrangle
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You approach the situation like a spooked animal."
    description: "Awareness skill check (DC 13) to handle or calm an animal or unruly person."
    effect:
      type: skill_check
      skill: Awareness
      dc: 13

  - id: mark_trail
    name: Mark Trail
    archetype: ""
    job: tracker
    pf2e: ""
    active: true
    shortcut: trail
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You catalog every disturbance in the environment."
    description: "Awareness skill check (DC 14) to follow or identify a trail."
    effect:
      type: skill_check
      skill: Awareness
      dc: 14
```

**Step 6: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 7: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 8: Commit**

```bash
git add content/conditions/dig_in_active.yaml content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions and conditions for 7 naturalist jobs"
```

---

## Task 9: Add schemer and zealot job active actions (9 jobs)

**Schemer jobs needing active (4):** grifter, mall_ninja, narcomancer, salesman
**Zealot jobs needing active (5):** follower, guard, hired_help, trainee, vigilante

Conditions needed for schemer: none (all skill_check).
Conditions needed for zealot: `guard_challenge_active`, `extra_mile_active`.

**Files:**
- Create: `content/conditions/guard_challenge_active.yaml`
- Create: `content/conditions/extra_mile_active.yaml`
- Modify: `content/class_features.yaml`
- Modify: `internal/game/ruleset/class_feature_test.go` (count 140 → 149)

**Step 1: Create condition files**

`content/conditions/guard_challenge_active.yaml`:
```yaml
id: guard_challenge_active
name: Guard Challenge Active
description: |
  You have issued a formal challenge. The target must engage you or be
  treated as fleeing — they are flat-footed if they try to move past you.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

`content/conditions/extra_mile_active.yaml`:
```yaml
id: extra_mile_active
name: Extra Mile Active
description: |
  You push beyond your normal limits. You gain +1 to all checks and
  +1 AC until your next turn.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: -1
damage_bonus: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

**Step 2: Run condition tests**

```bash
go test ./internal/game/condition/... -v
```
Expected: all PASS

**Step 3: Update count in test (140 → 149)**

**Step 4: Run count test to verify fail**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: FAIL

**Step 5: Append 9 features to `content/class_features.yaml`**

```yaml
  # --- schemer job active features ---
  - id: con
    name: Con
    archetype: ""
    job: grifter
    pf2e: ""
    active: true
    shortcut: con
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You spin the story."
    description: "Deception skill check (DC 15) to run a convincing short con."
    effect:
      type: skill_check
      skill: Deception
      dc: 15

  - id: tactical_reload
    name: Tactical Reload
    archetype: ""
    job: mall_ninja
    pf2e: ""
    active: true
    shortcut: reload
    action_cost: 1
    contexts:
      - combat
    activate_text: "You perform a dramatic tactical reload."
    description: "Deal 1d6 piercing damage — compensating for everything with firepower."
    effect:
      type: damage
      amount: "1d6"
      damage_type: piercing

  - id: deal_action
    name: Deal
    archetype: ""
    job: narcomancer
    pf2e: ""
    active: true
    shortcut: deal
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You know what they need."
    description: "Presence skill check (DC 14) to offer a deal with favorable terms."
    effect:
      type: skill_check
      skill: Presence
      dc: 14

  - id: hard_sell
    name: Hard Sell
    archetype: ""
    job: salesman
    pf2e: ""
    active: true
    shortcut: sell
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You close the deal before they know what hit them."
    description: "Presence skill check (DC 14) to push a sale or persuade a reluctant party."
    effect:
      type: skill_check
      skill: Presence
      dc: 14

  # --- zealot job active features ---
  - id: testimony
    name: Testimony
    archetype: ""
    job: follower
    pf2e: ""
    active: true
    shortcut: testify
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You speak your truth with absolute conviction."
    description: "Presence skill check (DC 13) to move or persuade through personal testimony."
    effect:
      type: skill_check
      skill: Presence
      dc: 13

  - id: challenge
    name: Challenge
    archetype: ""
    job: guard
    pf2e: ""
    active: true
    shortcut: challenge
    action_cost: 1
    contexts:
      - combat
    activate_text: "Halt. State your business."
    description: "Issue a formal challenge: target is flat-footed if they try to move past you."
    effect:
      type: condition
      target: self
      condition_id: guard_challenge_active

  - id: extra_mile
    name: Extra Mile
    archetype: ""
    job: hired_help
    pf2e: ""
    active: true
    shortcut: extra
    action_cost: 1
    contexts:
      - combat
      - exploration
    activate_text: "The customer is always right. Even when they're wrong."
    description: "+1 to all checks and +1 AC until your next turn."
    effect:
      type: condition
      target: self
      condition_id: extra_mile_active

  - id: by_the_book
    name: By the Book
    archetype: ""
    job: trainee
    pf2e: ""
    active: true
    shortcut: book
    action_cost: 1
    contexts:
      - exploration
    activate_text: "You consult your training."
    description: "Reasoning skill check (DC 13) to recall procedure and apply it correctly."
    effect:
      type: skill_check
      skill: Reasoning
      dc: 13

  - id: justice_strike
    name: Justice Strike
    archetype: ""
    job: vigilante
    pf2e: ""
    active: true
    shortcut: justice
    action_cost: 1
    contexts:
      - combat
    activate_text: "For those who can't protect themselves."
    description: "Deal 1d6 piercing damage — a precise strike against wrongdoing."
    effect:
      type: damage
      amount: "1d6"
      damage_type: piercing
```

**Step 6: Run count test**

```bash
go test ./internal/game/ruleset/... -run TestLoadClassFeatures_Count -v
```
Expected: PASS

**Step 7: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 8: Commit**

```bash
git add content/conditions/guard_challenge_active.yaml content/conditions/extra_mile_active.yaml content/class_features.yaml internal/game/ruleset/class_feature_test.go
git commit -m "content: add active actions for 4 schemer and 5 zealot jobs"
```

---

## Task 10: Update FEATURES.md and deploy

**Files:**
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Open `docs/requirements/FEATURES.md` and find the Actions section**

Look for lines related to "Actions content" or "active actions per job". Update to mark complete.

**Step 2: Add completion note**

Find the Actions system section and append or update:

```markdown
- [x] Active action added for every archetype (criminal: ghost, drifter: mark, nerd: exploit, naturalist: primal_surge, schemer: setup, zealot: lay_hands)
- [x] Passive features added for naturalist (hardy), schemer (smooth_operator), zealot (true_believer)
- [x] Active action added for every job (52 jobs across all archetypes)
```

**Step 3: Run full test suite one final time**

```bash
go test ./... -count=1
```
Expected: all PASS

**Step 4: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark Actions content complete in FEATURES.md"
```

**Step 5: Deploy**

```bash
make k8s-redeploy
```
Expected: successful helm upgrade in namespace `mud`

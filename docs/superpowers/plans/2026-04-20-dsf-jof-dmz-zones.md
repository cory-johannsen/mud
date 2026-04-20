# DSF / JOF / DMZ Zones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three interconnected zones to SE Industrial — The Dick Sucking Factory, The Jerk Off Factory, and The DMZ — with factions, combat NPCs, HTN domains, and mutual inter-faction hostility.

**Architecture:** Two rival corporate-parody factions (`dick_sucking_factory`, `jerk_off_factory`) are wired with `hostile_factions` so their NPCs engage each other when a player is present in the DMZ. All content is data-driven YAML; no Go engine changes are required. The Pipe Yard room in SE Industrial gains west and east exits to the two factories.

**Tech Stack:** Go, YAML content files, testify, pgregory.net/rapid (property tests), existing zone/faction/NPC loaders.

---

## File Map

**Create:**
- `content/factions/dick_sucking_factory.yaml`
- `content/factions/jerk_off_factory.yaml`
- `content/npcs/dsf_floor_supervisor.yaml`
- `content/npcs/dsf_middle_manager.yaml`
- `content/npcs/dsf_hr_rep.yaml`
- `content/npcs/will_blunderfield.yaml`
- `content/npcs/jof_efficiency_consultant.yaml`
- `content/npcs/jof_qa_officer.yaml`
- `content/npcs/jof_regional_liaison.yaml`
- `content/npcs/joe_crystal.yaml`
- `content/ai/dsf_worker_combat.yaml`
- `content/ai/dsf_hr_combat.yaml`
- `content/ai/dsf_boss_combat.yaml`
- `content/ai/jof_worker_combat.yaml`
- `content/ai/jof_liaison_combat.yaml`
- `content/ai/jof_boss_combat.yaml`
- `content/npcs/non_combat/dick_sucking_factory.yaml`
- `content/npcs/non_combat/jerk_off_factory.yaml`
- `content/zones/dick_sucking_factory.yaml`
- `content/zones/jerk_off_factory.yaml`
- `content/zones/the_dmz.yaml`

**Modify:**
- `content/zones/se_industrial.yaml` — add west/east exits to `sei_pipe_yard`
- `internal/game/world/noncombat_coverage_test.go` — add `the_dmz` to `exemptZones`

---

## Task 1: DSF and JOF Faction Files

**Files:**
- Create: `content/factions/dick_sucking_factory.yaml`
- Create: `content/factions/jerk_off_factory.yaml`
- Test: `internal/game/faction/registry_test.go` (add to existing file)

- [ ] **Step 1: Write the failing test**

Add to `internal/game/faction/registry_test.go`:

```go
func TestDSFAndJOFFactionsLoad(t *testing.T) {
	root := func() string {
		_, thisFile, _, _ := runtime.Caller(0)
		dir := filepath.Dir(thisFile)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			dir = filepath.Dir(dir)
		}
	}()
	reg, err := faction.LoadFactions(filepath.Join(root, "content", "factions"))
	require.NoError(t, err)
	dsf, ok := reg["dick_sucking_factory"]
	require.True(t, ok, "dick_sucking_factory faction must be registered")
	jof, ok := reg["jerk_off_factory"]
	require.True(t, ok, "jerk_off_factory faction must be registered")
	require.Contains(t, dsf.HostileFactions, "jerk_off_factory")
	require.Contains(t, jof.HostileFactions, "dick_sucking_factory")
}
```

Add `"runtime"` to the import block if not already present.

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/faction/... -run TestDSFAndJOFFactionsLoad -v
```

Expected: FAIL — faction files do not exist yet.

- [ ] **Step 3: Create `content/factions/dick_sucking_factory.yaml`**

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

- [ ] **Step 4: Create `content/factions/jerk_off_factory.yaml`**

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

- [ ] **Step 5: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/faction/... -run TestDSFAndJOFFactionsLoad -v
```

Expected: PASS.

- [ ] **Step 6: Run the full faction test suite**

```bash
mise exec -- go test ./internal/game/faction/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add content/factions/dick_sucking_factory.yaml content/factions/jerk_off_factory.yaml internal/game/faction/registry_test.go
git commit -m "feat(content): add DSF and JOF factions with mutual hostility"
```

---

## Task 2: DSF Combat NPC Templates

**Files:**
- Create: `content/npcs/dsf_floor_supervisor.yaml`
- Create: `content/npcs/dsf_middle_manager.yaml`
- Create: `content/npcs/dsf_hr_rep.yaml`
- Create: `content/npcs/will_blunderfield.yaml`
- Test: `internal/game/npc/template_test.go` (add to existing file)

- [ ] **Step 1: Write the failing test**

Add to `internal/game/npc/template_test.go`:

```go
func TestDSFNPCTemplatesLoad(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := thisFile
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	templates := []string{
		"dsf_floor_supervisor",
		"dsf_middle_manager",
		"dsf_hr_rep",
		"will_blunderfield",
	}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "content", "npcs", name+".yaml")
			data, err := os.ReadFile(path)
			require.NoError(t, err, "NPC file missing: %s.yaml", name)
			var tmpl npc.Template
			require.NoError(t, yaml.Unmarshal(data, &tmpl), "invalid YAML: %s.yaml", name)
			require.Equal(t, name, tmpl.ID, "ID mismatch in %s.yaml", name)
			require.Equal(t, "dick_sucking_factory", tmpl.FactionID, "wrong faction_id in %s.yaml", name)
		})
	}
}
```

Add imports if needed: `"os"`, `"path/filepath"`, `"runtime"`, `"gopkg.in/yaml.v3"`.

Check what field name FactionID uses in the Template struct:

```bash
grep -n "FactionID\|faction_id" /home/cjohannsen/src/mud/internal/game/npc/template.go | head -5
```

Use the exact field name from the struct.

- [ ] **Step 2: Run the test to verify it fails**

```bash
mise exec -- go test ./internal/game/npc/... -run TestDSFNPCTemplatesLoad -v
```

Expected: FAIL — files do not exist.

- [ ] **Step 3: Create `content/npcs/dsf_floor_supervisor.yaml`**

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

- [ ] **Step 4: Create `content/npcs/dsf_middle_manager.yaml`**

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

- [ ] **Step 5: Create `content/npcs/dsf_hr_rep.yaml`**

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

- [ ] **Step 6: Create `content/npcs/will_blunderfield.yaml`**

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

- [ ] **Step 7: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/npc/... -run TestDSFNPCTemplatesLoad -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add content/npcs/dsf_floor_supervisor.yaml content/npcs/dsf_middle_manager.yaml content/npcs/dsf_hr_rep.yaml content/npcs/will_blunderfield.yaml internal/game/npc/template_test.go
git commit -m "feat(content): add DSF combat NPC templates"
```

---

## Task 3: JOF Combat NPC Templates

**Files:**
- Create: `content/npcs/jof_efficiency_consultant.yaml`
- Create: `content/npcs/jof_qa_officer.yaml`
- Create: `content/npcs/jof_regional_liaison.yaml`
- Create: `content/npcs/joe_crystal.yaml`
- Test: `internal/game/npc/template_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/npc/template_test.go`:

```go
func TestJOFNPCTemplatesLoad(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := thisFile
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	templates := []string{
		"jof_efficiency_consultant",
		"jof_qa_officer",
		"jof_regional_liaison",
		"joe_crystal",
	}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "content", "npcs", name+".yaml")
			data, err := os.ReadFile(path)
			require.NoError(t, err, "NPC file missing: %s.yaml", name)
			var tmpl npc.Template
			require.NoError(t, yaml.Unmarshal(data, &tmpl), "invalid YAML: %s.yaml", name)
			require.Equal(t, name, tmpl.ID, "ID mismatch in %s.yaml", name)
			require.Equal(t, "jerk_off_factory", tmpl.FactionID, "wrong faction_id in %s.yaml", name)
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
mise exec -- go test ./internal/game/npc/... -run TestJOFNPCTemplatesLoad -v
```

Expected: FAIL.

- [ ] **Step 3: Create `content/npcs/jof_efficiency_consultant.yaml`**

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

- [ ] **Step 4: Create `content/npcs/jof_qa_officer.yaml`**

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

- [ ] **Step 5: Create `content/npcs/jof_regional_liaison.yaml`**

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

- [ ] **Step 6: Create `content/npcs/joe_crystal.yaml`**

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

- [ ] **Step 7: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/npc/... -run TestJOFNPCTemplatesLoad -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add content/npcs/jof_efficiency_consultant.yaml content/npcs/jof_qa_officer.yaml content/npcs/jof_regional_liaison.yaml content/npcs/joe_crystal.yaml internal/game/npc/template_test.go
git commit -m "feat(content): add JOF combat NPC templates"
```

---

## Task 4: HTN Combat Domains

**Files:**
- Create: `content/ai/dsf_worker_combat.yaml`
- Create: `content/ai/dsf_hr_combat.yaml`
- Create: `content/ai/dsf_boss_combat.yaml`
- Create: `content/ai/jof_worker_combat.yaml`
- Create: `content/ai/jof_liaison_combat.yaml`
- Create: `content/ai/jof_boss_combat.yaml`
- Test: find the domain loading test file

First, identify the HTN domain loader test:

```bash
grep -rn "LoadDomain\|LoadDomains\|ai_domain\|content/ai" /home/cjohannsen/src/mud/internal/game/ai/ | grep -v "_test" | head -10
grep -rn "func Test.*Domain\|func Test.*HTN" /home/cjohannsen/src/mud/internal/ | head -10
```

- [ ] **Step 1: Write the failing test**

Find the appropriate test file for HTN domain loading (likely `internal/game/ai/` or `internal/gameserver/`). Add:

```go
func TestDSFAndJOFHTNDomainsLoad(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := thisFile
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	domains := []string{
		"dsf_worker_combat",
		"dsf_hr_combat",
		"dsf_boss_combat",
		"jof_worker_combat",
		"jof_liaison_combat",
		"jof_boss_combat",
	}
	for _, name := range domains {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "content", "ai", name+".yaml")
			data, err := os.ReadFile(path)
			require.NoError(t, err, "domain file missing: %s.yaml", name)
			// Use the existing domain loader — check what function is available:
			// grep for LoadDomain or ParseDomain in internal/game/ai/
			var wrapper struct {
				Domain ai.Domain `yaml:"domain"`
			}
			require.NoError(t, yaml.Unmarshal(data, &wrapper))
			require.Equal(t, name, wrapper.Domain.ID)
		})
	}
}
```

Run: `grep -rn "func Load\|func Parse\|type Domain" /home/cjohannsen/src/mud/internal/game/ai/` to find the exact type and loader, then adjust the test to use it.

- [ ] **Step 2: Run the test to verify it fails**

```bash
mise exec -- go test ./internal/game/ai/... -run TestDSFAndJOFHTNDomainsLoad -v
```

Expected: FAIL — domain files do not exist.

- [ ] **Step 3: Create `content/ai/dsf_worker_combat.yaml`**

```yaml
domain:
  id: dsf_worker_combat
  description: Standard DSF worker. Attacks nearest enemy.

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

- [ ] **Step 4: Create `content/ai/dsf_hr_combat.yaml`**

```yaml
domain:
  id: dsf_hr_combat
  description: DSF HR representative. Attacks and applies despair condition.

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

- [ ] **Step 5: Create `content/ai/dsf_boss_combat.yaml`**

```yaml
domain:
  id: dsf_boss_combat
  description: Will Blunderfield. Strikes weakest enemy and applies moderate despair.

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

- [ ] **Step 6: Create `content/ai/jof_worker_combat.yaml`**

```yaml
domain:
  id: jof_worker_combat
  description: Standard JOF worker. Attacks nearest enemy.

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

- [ ] **Step 7: Create `content/ai/jof_liaison_combat.yaml`**

```yaml
domain:
  id: jof_liaison_combat
  description: JOF regional liaison. Attacks and applies delirium condition.

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

- [ ] **Step 8: Create `content/ai/jof_boss_combat.yaml`**

```yaml
domain:
  id: jof_boss_combat
  description: Joe Crystal. Strikes weakest enemy and applies moderate delirium.

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

- [ ] **Step 9: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/ai/... -run TestDSFAndJOFHTNDomainsLoad -v
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add content/ai/dsf_worker_combat.yaml content/ai/dsf_hr_combat.yaml content/ai/dsf_boss_combat.yaml content/ai/jof_worker_combat.yaml content/ai/jof_liaison_combat.yaml content/ai/jof_boss_combat.yaml
git commit -m "feat(content): add DSF and JOF HTN combat domains"
```

---

## Task 5: Non-Combat NPCs

**Files:**
- Create: `content/npcs/non_combat/dick_sucking_factory.yaml`
- Create: `content/npcs/non_combat/jerk_off_factory.yaml`

Non-combat NPCs (quest_giver, fixer, merchant, healer, banker) follow the format in `content/npcs/non_combat/se_industrial.yaml`. The zone YAML files reference templates named `dsf_quest_giver`, `dsf_fixer`, `dsf_black_market_merchant` and their JOF equivalents — these IDs must live in the non_combat YAML.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/world/noncombat_coverage_test.go` inside `TestNonCombatNPCTemplateIDs` — add `"dick_sucking_factory"` and `"jerk_off_factory"` to the `zones` slice:

```go
zones := []string{
    "aloha", "beaverton", "battleground", "downtown", "felony_flats",
    "hillsboro", "lake_oswego", "ne_portland", "pdx_international",
    "ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
    "the_couve", "troutdale", "vantucky",
    "dick_sucking_factory", "jerk_off_factory",  // add these
}
```

Make the same addition to `TestAllZonesHaveRequiredNPCTypes`.

- [ ] **Step 2: Run the test to verify it fails**

```bash
mise exec -- go test ./internal/game/world/... -run TestNonCombatNPCTemplateIDs -v
```

Expected: FAIL — non_combat files do not exist.

- [ ] **Step 3: Create `content/npcs/non_combat/dick_sucking_factory.yaml`**

```yaml
- id: dick_sucking_factory_quest_giver
  name: "Disgruntled Employee"
  npc_type: quest_giver
  type: human
  description: "A DSF employee who has been passed over for promotion eleven times and has decided to channel their energy into more productive directions."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral

- id: dick_sucking_factory_merchant
  name: "Supply Requisitions"
  npc_type: merchant
  type: human
  description: "Manages the break room supply cabinet with bureaucratic precision. Technically this is not a store. Technically."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.45
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 5
        max_stock: 10
      - item_id: scrap_metal
        base_price: 20
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 10
      stock_refill: 1
      budget_refill: 180

- id: dick_sucking_factory_healer
  name: "Occupational Health"
  npc_type: healer
  type: human
  description: "The DSF occupational health officer patches wounds with the same clinical detachment they apply to everything else. Forms in triplicate required."
  level: 0
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: dick_sucking_factory_job_trainer
  name: "Professional Development"
  npc_type: job_trainer
  type: human
  description: "Runs mandatory professional development sessions. Attendance is not optional. Neither is improvement."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger

- id: dick_sucking_factory_banker
  name: "Payroll"
  npc_type: banker
  type: human
  description: "Handles payroll and financial services from the server room. Has questions about where you got that money."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    interest_rate: 0.02
    max_loan: 1000
```

- [ ] **Step 4: Create `content/npcs/non_combat/jerk_off_factory.yaml`**

```yaml
- id: jerk_off_factory_quest_giver
  name: "Process Improvement Lead"
  npc_type: quest_giver
  type: human
  description: "A JOF process improvement specialist who has identified several inefficiencies in the current threat landscape and would like your help addressing them."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral

- id: jerk_off_factory_merchant
  name: "Inventory Management"
  npc_type: merchant
  type: human
  description: "Runs the JOF break room supply system with optimized inventory levels and data-driven replenishment cycles."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.45
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 5
        max_stock: 10
      - item_id: circuit_board
        base_price: 80
        init_stock: 3
        max_stock: 6
    replenish_rate:
      min_hours: 6
      max_hours: 10
      stock_refill: 1
      budget_refill: 180

- id: jerk_off_factory_healer
  name: "Wellness Optimization"
  npc_type: healer
  type: human
  description: "The JOF wellness optimization officer measures recovery efficiency with a stopwatch. Fast healing is billable; slow healing is a performance issue."
  level: 0
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: jerk_off_factory_job_trainer
  name: "Skills Development"
  npc_type: job_trainer
  type: human
  description: "Offers skills development programs benchmarked against industry standards. Your current skill level has been assessed. The assessment was not encouraging."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger

- id: jerk_off_factory_banker
  name: "Financial Services"
  npc_type: banker
  type: human
  description: "Manages the JOF employee financial services program. Has run a cost-benefit analysis on your creditworthiness. Has concerns."
  level: 0
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    interest_rate: 0.02
    max_loan: 1000
```

Note: The zone YAML files reference `dsf_quest_giver`, `dsf_fixer`, `dsf_black_market_merchant` etc. Update those zone spawn template IDs to match the non_combat IDs above (`dick_sucking_factory_quest_giver`, `dick_sucking_factory_merchant`, etc.) when creating the zone files in Tasks 6 and 7. The non_combat system uses `{zone_id}_{npc_type}` naming.

- [ ] **Step 5: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/world/... -run TestNonCombatNPCTemplateIDs -v
mise exec -- go test ./internal/game/world/... -run TestAllZonesHaveRequiredNPCTypes -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add content/npcs/non_combat/dick_sucking_factory.yaml content/npcs/non_combat/jerk_off_factory.yaml internal/game/world/noncombat_coverage_test.go
git commit -m "feat(content): add DSF and JOF non-combat NPC rosters"
```

---

## Task 6: Dick Sucking Factory Zone

**Files:**
- Create: `content/zones/dick_sucking_factory.yaml`
- Test: `internal/game/world/noncombat_coverage_test.go` (safe room check auto-covers it)

- [ ] **Step 1: Verify the safe room test will pass**

The zone has `dsf_break_room` with `danger_level: safe`. The `TestAllZonesHaveAtLeastOneSafeRoom` test will cover it automatically once the file exists. No exemption needed.

- [ ] **Step 2: Create `content/zones/dick_sucking_factory.yaml`**

Use NPC template IDs that match the non_combat naming convention (`dick_sucking_factory_quest_giver`, `dick_sucking_factory_merchant` etc.) and combat template IDs from Task 2:

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
      "SYNERGY IS SURVIVAL" and "THERE IS NO I IN TEAM, ONLY IN
      TERMINATED." Middle managers patrol the aisles with clipboards
      and sidearms.
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
    - template: dick_sucking_factory_quest_giver
      count: 1
      respawn_after: 0s
    - template: dick_sucking_factory_merchant
      count: 1
      respawn_after: 0s
    - template: dick_sucking_factory_healer
      count: 1
      respawn_after: 0s
    - template: dick_sucking_factory_job_trainer
      count: 1
      respawn_after: 0s
    - template: dick_sucking_factory_banker
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
      the top, in red, in a font that communicates authority. He is at
      his desk. He has been expecting you since Q2.
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
```

- [ ] **Step 3: Run the zone loading tests**

```bash
mise exec -- go test ./internal/game/world/... -run TestAllZonesHaveAtLeastOneSafeRoom -v
mise exec -- go test ./internal/game/world/... -v 2>&1 | tail -20
```

Expected: all PASS including the new zone.

- [ ] **Step 4: Commit**

```bash
git add content/zones/dick_sucking_factory.yaml
git commit -m "feat(zones): add The Dick Sucking Factory zone"
```

---

## Task 7: Jerk Off Factory Zone

**Files:**
- Create: `content/zones/jerk_off_factory.yaml`

- [ ] **Step 1: Create `content/zones/jerk_off_factory.yaml`**

Note: `jof_break_room` and `jof_meeting_room` were at duplicate map coordinates (2,2) in the spec. Fix: place `jof_break_room` at map_x: 0, map_y: 2.

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
    - direction: east
      target: jof_lobby
    map_x: -2
    map_y: 0
    spawns:
    - template: jerk_off_factory_quest_giver
      count: 1
      respawn_after: 0s
    - template: jerk_off_factory_merchant
      count: 1
      respawn_after: 0s
    - template: jerk_off_factory_healer
      count: 1
      respawn_after: 0s
    - template: jerk_off_factory_job_trainer
      count: 1
      respawn_after: 0s
    - template: jerk_off_factory_banker
      count: 1
      respawn_after: 0s

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
      target: jof_meeting_room
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
    - direction: south
      target: jof_cubicle_farm
    - direction: east
      target: jof_board_room
    map_x: 2
    map_y: 2

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

- [ ] **Step 2: Run the zone loading tests**

```bash
mise exec -- go test ./internal/game/world/... -run TestAllZonesHaveAtLeastOneSafeRoom -v
```

Expected: PASS — `jof_break_room` has `danger_level: safe`.

- [ ] **Step 3: Commit**

```bash
git add content/zones/jerk_off_factory.yaml
git commit -m "feat(zones): add The Jerk Off Factory zone"
```

---

## Task 8: The DMZ Zone

**Files:**
- Create: `content/zones/the_dmz.yaml`
- Modify: `internal/game/world/noncombat_coverage_test.go` — add `the_dmz` to `exemptZones`

- [ ] **Step 1: Add DMZ to the exempt zones map**

In `internal/game/world/noncombat_coverage_test.go`, find the `exemptZones` map in `TestAllZonesHaveAtLeastOneSafeRoom` and add `"the_dmz"`:

```go
exemptZones := map[string]bool{
    "clown_camp":          true,
    "steampdx":            true,
    "the_velvet_rope":     true,
    "club_privata":        true,
    "the_dmz":             true,  // all-contested zone, no safe rooms by design
}
```

- [ ] **Step 2: Run the test before creating the file to confirm the exemption works**

```bash
mise exec -- go test ./internal/game/world/... -run TestAllZonesHaveAtLeastOneSafeRoom -v
```

Expected: PASS (the_dmz file doesn't exist yet so it's not scanned).

- [ ] **Step 3: Create `content/zones/the_dmz.yaml`**

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

- [ ] **Step 4: Run the safe room test to confirm DMZ is exempt**

```bash
mise exec -- go test ./internal/game/world/... -run TestAllZonesHaveAtLeastOneSafeRoom -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add content/zones/the_dmz.yaml internal/game/world/noncombat_coverage_test.go
git commit -m "feat(zones): add The DMZ contested zone between DSF and JOF"
```

---

## Task 9: Pipe Yard Exit Additions

**Files:**
- Modify: `content/zones/se_industrial.yaml` (add west/east exits to `sei_pipe_yard`)
- Test: `internal/game/world/noncombat_coverage_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/game/world/noncombat_coverage_test.go`:

```go
func TestPipeYard_HasDSFAndJOFExits(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "content", "zones", "se_industrial.yaml"))
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)

	pipeYard, ok := zone.Rooms["sei_pipe_yard"]
	require.True(t, ok, "sei_pipe_yard must exist in se_industrial")

	hasWest, hasEast := false, false
	for _, exit := range pipeYard.Exits {
		if string(exit.Direction) == "west" && exit.TargetRoom == "dsf_reception" {
			hasWest = true
		}
		if string(exit.Direction) == "east" && exit.TargetRoom == "jof_lobby" {
			hasEast = true
		}
	}
	require.True(t, hasWest, "sei_pipe_yard must have west exit to dsf_reception (REQ-DSF-9)")
	require.True(t, hasEast, "sei_pipe_yard must have east exit to jof_lobby (REQ-DSF-9)")
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
mise exec -- go test ./internal/game/world/... -run TestPipeYard_HasDSFAndJOFExits -v
```

Expected: FAIL — exits do not exist yet.

- [ ] **Step 3: Add exits to `sei_pipe_yard` in `content/zones/se_industrial.yaml`**

Find the `sei_pipe_yard` room (around line 394). Add two exits to the existing exits list:

```yaml
    exits:
    - direction: north
      target: sei_crane_yard
    - direction: south
      target: sei_transformer_station
    - direction: west
      target: dsf_reception
      zone: dick_sucking_factory
    - direction: east
      target: jof_lobby
      zone: jerk_off_factory
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
mise exec -- go test ./internal/game/world/... -run TestPipeYard_HasDSFAndJOFExits -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add content/zones/se_industrial.yaml internal/game/world/noncombat_coverage_test.go
git commit -m "feat(content): add DSF and JOF exits from SE Industrial Pipe Yard"
```

---

## Task 10: Mutual Hostility Integration Test and Full Suite

**Files:**
- Test: `internal/game/world/noncombat_coverage_test.go`

- [ ] **Step 1: Write the mutual hostility test**

Add to `internal/game/world/noncombat_coverage_test.go`:

```go
func TestDMZ_MutualHostility(t *testing.T) {
	root := repoRoot(t)
	reg, err := faction.LoadFactions(filepath.Join(root, "content", "factions"))
	require.NoError(t, err)

	dsf, ok := reg["dick_sucking_factory"]
	require.True(t, ok, "dick_sucking_factory faction must exist")
	jof, ok := reg["jerk_off_factory"]
	require.True(t, ok, "jerk_off_factory faction must exist")

	dsfHostile := false
	for _, h := range dsf.HostileFactions {
		if h == "jerk_off_factory" {
			dsfHostile = true
		}
	}
	require.True(t, dsfHostile, "dick_sucking_factory must list jerk_off_factory as hostile (REQ-DSF-11c)")

	jofHostile := false
	for _, h := range jof.HostileFactions {
		if h == "dick_sucking_factory" {
			jofHostile = true
		}
	}
	require.True(t, jofHostile, "jerk_off_factory must list dick_sucking_factory as hostile (REQ-DSF-11c)")
}
```

Add import: `"github.com/cory-johannsen/mud/internal/game/faction"` to the import block.

- [ ] **Step 2: Run the mutual hostility test**

```bash
mise exec -- go test ./internal/game/world/... -run TestDMZ_MutualHostility -v
```

Expected: PASS.

- [ ] **Step 3: Run the full world test suite**

```bash
mise exec -- go test ./internal/game/world/... -v -count=1 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 4: Run the full test suite**

```bash
mise exec -- go test -race -count=1 -timeout=300s ./... 2>&1 | tail -40
```

Expected: all PASS. Fix any failures before proceeding.

- [ ] **Step 5: Final commit**

```bash
git add internal/game/world/noncombat_coverage_test.go
git commit -m "test(world): add DMZ mutual hostility and Pipe Yard exit integration tests"
```

---

## Self-Review

**Spec coverage check:**
- REQ-DSF-1 ✓ Task 1 (DSF faction YAML)
- REQ-DSF-2 ✓ Task 1 (JOF faction YAML)
- REQ-DSF-3 ✓ Task 2 (DSF NPC templates)
- REQ-DSF-4 ✓ Task 3 (JOF NPC templates)
- REQ-DSF-5 ✓ Task 4 (HTN domains)
- REQ-DSF-6 ✓ Task 6 (DSF zone)
- REQ-DSF-7 ✓ Task 7 (JOF zone)
- REQ-DSF-8 ✓ Task 8 (DMZ zone)
- REQ-DSF-9 ✓ Task 9 (Pipe Yard exits)
- REQ-DSF-10 ✓ Task 5 (non-combat NPCs — quest_giver, merchant, healer, job_trainer, banker per zone)
- REQ-DSF-11 ✓ Tasks 1 + 10 (mutual hostility faction config + test)
- REQ-DSF-12 ✓ Tasks 1–10 (validation tests throughout)

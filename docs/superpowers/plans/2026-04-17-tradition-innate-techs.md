# Tradition Innate Technologies as Cantrips — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Grant all tech-capable archetypes the full innate technology list for their tradition at character creation, unlimited-use, matching PF2E cantrip semantics.

**Architecture:** Pure content + migration work. The AssignTechnologies loop already processes `archetype.InnateTechnologies` with unlimited grants gated by `isTechCapable`. Changes are: (1) 8 new innate tech YAML files for neural and fanatic_doctrine traditions, (2) `innate_technologies` blocks added to 6 archetype YAMLs, (3) SQL migration 063 to backfill existing characters. No Go code changes required.

**Tech Stack:** Go 1.23, pgregory.net/rapid (property tests), testify, golang-migrate, PostgreSQL, YAML content files.

---

## File Map

**Create:**
- `content/technologies/innate/neural_flare.yaml`
- `content/technologies/innate/static_veil.yaml`
- `content/technologies/innate/synapse_tap.yaml`
- `content/technologies/innate/doctrine_ward.yaml`
- `content/technologies/innate/martyrs_resolve.yaml`
- `content/technologies/innate/righteous_condemnation.yaml`
- `content/technologies/innate/fervor_pulse.yaml`
- `content/technologies/innate/litany_of_iron.yaml`
- `migrations/063_tradition_innate_techs.up.sql`
- `migrations/063_tradition_innate_techs.down.sql`

**Modify:**
- `content/archetypes/nerd.yaml` — add `innate_technologies` block before `technology_grants`
- `content/archetypes/naturalist.yaml` — same
- `content/archetypes/drifter.yaml` — same
- `content/archetypes/schemer.yaml` — same
- `content/archetypes/influencer.yaml` — same
- `content/archetypes/zealot.yaml` — same
- `internal/game/technology/registry_innate_test.go` — update hardcoded count from 11 → 19
- `internal/game/ruleset/loader_test.go` — add loader test for archetype innate tech population
- `internal/gameserver/technology_assignment_test.go` — add tradition innate grant integration tests

---

### Task 1: Neural innate tech YAML files

**Files:**
- Modify: `internal/game/technology/registry_innate_test.go:23`
- Create: `content/technologies/innate/neural_flare.yaml`
- Create: `content/technologies/innate/static_veil.yaml`
- Create: `content/technologies/innate/synapse_tap.yaml`

- [ ] **Step 1: Update the innate count test to expect 14**

In `internal/game/technology/registry_innate_test.go`, change line 23:

```go
// Before:
assert.Equal(t, 11, len(innate), "expected 11 innate tech files loaded")

// After:
assert.Equal(t, 14, len(innate), "expected 14 innate tech files loaded")
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -run TestLoad_InnateSubdirLoads -v
```

Expected: `FAIL` — `expected 14 innate tech files loaded, got 11`

- [ ] **Step 3: Create `content/technologies/innate/neural_flare.yaml`**

```yaml
id: neural_flare
name: Neural Flare
description: A sharp directed neural frequency that overloads a target's pain receptors, causing momentary agony.
tradition: neural
level: 1
usage_type: innate
action_cost: 2
range: single
targets: single
duration: rounds:1
resolution: save
save_type: cool
save_dc: 15
effects:
  on_success:
    - type: utility
      description: "The neural frequency grazes you — a spike of pain, then nothing."
  on_failure:
    - type: condition
      condition_id: sickened
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: sickened
      value: 1
      duration: rounds:1
    - type: condition
      condition_id: slowed
      value: 1
      duration: rounds:1
```

- [ ] **Step 4: Create `content/technologies/innate/static_veil.yaml`**

```yaml
id: static_veil
name: Static Veil
description: Emits a burst of local mesh interference that scrambles targeting optics and visual feeds, rendering you harder to perceive.
tradition: neural
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: condition
      condition_id: concealed
      value: 1
      duration: rounds:1
```

- [ ] **Step 5: Create `content/technologies/innate/synapse_tap.yaml`**

```yaml
id: synapse_tap
name: Synapse Tap
description: A passive subdermal antenna reads surface emotional resonance from nearby creatures, giving you an edge in manipulation and intimidation.
tradition: neural
level: 1
usage_type: innate
action_cost: 0
passive: true
range: zone
targets: all
duration: passive
resolution: none
effects:
  on_apply:
    - type: circumstance_bonus
      skill: deception
      value: 1
      description: "Your synapse tap reads emotional tells, giving you a +1 circumstance bonus on Deception checks against creatures in the zone."
    - type: circumstance_bonus
      skill: intimidation
      value: 1
      description: "Your synapse tap reads fear responses, giving you a +1 circumstance bonus on Intimidation checks against creatures in the zone."
```

- [ ] **Step 6: Run the test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -run TestLoad_InnateSubdirLoads -v
```

Expected: `PASS`

- [ ] **Step 7: Run full technology package tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add content/technologies/innate/neural_flare.yaml \
        content/technologies/innate/static_veil.yaml \
        content/technologies/innate/synapse_tap.yaml \
        internal/game/technology/registry_innate_test.go
git commit -m "feat(content): add neural tradition innate techs (neural_flare, static_veil, synapse_tap)"
```

---

### Task 2: Fanatic doctrine innate tech YAML files

**Files:**
- Modify: `internal/game/technology/registry_innate_test.go:23`
- Create: `content/technologies/innate/doctrine_ward.yaml`
- Create: `content/technologies/innate/martyrs_resolve.yaml`
- Create: `content/technologies/innate/righteous_condemnation.yaml`
- Create: `content/technologies/innate/fervor_pulse.yaml`
- Create: `content/technologies/innate/litany_of_iron.yaml`

- [ ] **Step 1: Update the innate count test to expect 19**

In `internal/game/technology/registry_innate_test.go`, change line 23:

```go
// Before:
assert.Equal(t, 14, len(innate), "expected 14 innate tech files loaded")

// After:
assert.Equal(t, 19, len(innate), "expected 19 innate tech files loaded")
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -run TestLoad_InnateSubdirLoads -v
```

Expected: `FAIL` — `expected 19 innate tech files loaded, got 14`

- [ ] **Step 3: Create `content/technologies/innate/doctrine_ward.yaml`**

```yaml
id: doctrine_ward
name: Doctrine Ward
description: Faith-hardened subdermal plating grown through doctrinal conviction reduces all incoming damage by 1.
tradition: fanatic_doctrine
level: 1
usage_type: innate
action_cost: 0
passive: true
range: self
targets: single
duration: passive
resolution: none
effects:
  on_apply:
    - type: damage_reduction
      value: 1
      description: "Doctrine Ward reduces all incoming damage by 1."
```

- [ ] **Step 4: Create `content/technologies/innate/martyrs_resolve.yaml`**

```yaml
id: martyrs_resolve
name: Martyr's Resolve
description: When you reach zero hit points, a surge of doctrinal conviction stabilizes you at 1 HP. Once per scene.
tradition: fanatic_doctrine
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
reaction:
  triggers:
    - on_reduced_to_zero_hp
  effect:
    type: stabilize
    hp: 1
    once_per_scene: true
```

- [ ] **Step 5: Create `content/technologies/innate/righteous_condemnation.yaml`**

```yaml
id: righteous_condemnation
name: Righteous Condemnation
description: Mark a single target as heretic. Your attacks deal an additional 1d4 damage against them for 1 round.
tradition: fanatic_doctrine
level: 1
usage_type: innate
action_cost: 1
range: single
targets: single
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: mark
      tag: condemned
      bonus_damage_dice: 1d4
      duration: rounds:1
      description: "The condemned target takes +1d4 damage from your attacks for 1 round."
```

- [ ] **Step 6: Create `content/technologies/innate/fervor_pulse.yaml`**

```yaml
id: fervor_pulse
name: Fervor Pulse
description: Radiate a wave of zealous conviction that unsettles the faithless. All enemies in the zone must save or be Frightened.
tradition: fanatic_doctrine
level: 1
usage_type: innate
action_cost: 2
range: zone
targets: all_enemies
duration: rounds:1
resolution: save
save_type: cool
save_dc: 15
effects:
  on_success:
    - type: utility
      description: "The pulse washes over you. You hold your ground, unshaken."
  on_failure:
    - type: condition
      condition_id: frightened
      value: 1
      duration: rounds:1
  on_crit_failure:
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
    - type: condition
      condition_id: fleeing
      value: 1
      duration: rounds:1
```

- [ ] **Step 7: Create `content/technologies/innate/litany_of_iron.yaml`**

```yaml
id: litany_of_iron
name: Litany of Iron
description: Recite a doctrinal litany that steels your will against all threats. Gain +2 circumstance bonus to all saving throws for 1 round.
tradition: fanatic_doctrine
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: rounds:1
resolution: none
effects:
  on_apply:
    - type: circumstance_bonus
      skill: saving_throws
      value: 2
      duration: rounds:1
      description: "Litany of Iron grants +2 circumstance bonus to all saving throws for 1 round."
```

- [ ] **Step 8: Run the count test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -run TestLoad_InnateSubdirLoads -v
```

Expected: `PASS`

- [ ] **Step 9: Run full technology package tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/technology/... -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 10: Commit**

```bash
git add content/technologies/innate/doctrine_ward.yaml \
        content/technologies/innate/martyrs_resolve.yaml \
        content/technologies/innate/righteous_condemnation.yaml \
        content/technologies/innate/fervor_pulse.yaml \
        content/technologies/innate/litany_of_iron.yaml \
        internal/game/technology/registry_innate_test.go
git commit -m "feat(content): add fanatic_doctrine tradition innate techs (5 files)"
```

---

### Task 3: Archetype YAML innate_technologies loader test

**Files:**
- Modify: `internal/game/ruleset/loader_test.go`

This task adds the loader test that will fail until Task 4 populates the archetype YAMLs.

- [ ] **Step 1: Add the failing loader test for tech-capable archetypes**

Append this test to `internal/game/ruleset/loader_test.go`. The file is `package ruleset_test` importing `github.com/cory-johannsen/mud/internal/game/ruleset`. `ruleset.LoadArchetypes(dir string) ([]*ruleset.Archetype, error)` is the correct signature.

```go
// TestLoadArchetypes_TechCapable_AllHaveInnate verifies that all 6 tech-capable archetypes
// have at least one innate technology populated after the tradition cantrip content is added.
//
// Precondition: content/archetypes/{nerd,naturalist,drifter,schemer,influencer,zealot}.yaml
//   each have an innate_technologies block with uses_per_day: 0 entries.
// Postcondition: each tech-capable archetype has len(InnateTechnologies) >= 1;
//   every grant has UsesPerDay == 0.
func TestLoadArchetypes_TechCapable_AllHaveInnate(t *testing.T) {
	archetypes, err := ruleset.LoadArchetypes("../../../content/archetypes")
	require.NoError(t, err)

	techCapable := []string{"nerd", "naturalist", "drifter", "schemer", "influencer", "zealot"}
	byID := make(map[string]*ruleset.Archetype, len(archetypes))
	for _, arch := range archetypes {
		byID[arch.ID] = arch
	}

	for _, id := range techCapable {
		arch, ok := byID[id]
		require.True(t, ok, "archetype %q not found in loaded archetypes", id)
		assert.NotEmpty(t, arch.InnateTechnologies,
			"archetype %q must have innate_technologies populated", id)
		for _, grant := range arch.InnateTechnologies {
			assert.Equal(t, 0, grant.UsesPerDay,
				"archetype %q innate tech %q must have uses_per_day: 0 (unlimited)", id, grant.ID)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadArchetypes_TechCapable_AllHaveInnate -v
```

Expected: `FAIL` — `archetype "nerd" must have innate_technologies populated`

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/game/ruleset/loader_test.go
git commit -m "test(ruleset): add failing test for tech-capable archetype innate tech population"
```

---

### Task 4: Populate archetype YAMLs with tradition innate technologies

**Files:**
- Modify: `content/archetypes/nerd.yaml`
- Modify: `content/archetypes/naturalist.yaml`
- Modify: `content/archetypes/drifter.yaml`
- Modify: `content/archetypes/schemer.yaml`
- Modify: `content/archetypes/influencer.yaml`
- Modify: `content/archetypes/zealot.yaml`

In every archetype file, add the `innate_technologies:` block immediately before the `technology_grants:` key. The `innate_technologies:` key does not currently exist — you are adding it fresh.

- [ ] **Step 1: Add innate_technologies to `content/archetypes/nerd.yaml`**

Find the line `technology_grants:` (line ~12) and insert before it:

```yaml
innate_technologies:
  - id: atmospheric_surge
    uses_per_day: 0
  - id: blackout_pulse
    uses_per_day: 0
  - id: seismic_sense
    uses_per_day: 0
  - id: arc_lights
    uses_per_day: 0
  - id: pressure_burst
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 2: Add innate_technologies to `content/archetypes/naturalist.yaml`**

Find the line `technology_grants:` (line ~13) and insert before it:

```yaml
innate_technologies:
  - id: moisture_reclaim
    uses_per_day: 0
  - id: viscous_spray
    uses_per_day: 0
  - id: nanite_infusion
    uses_per_day: 0
  - id: acid_spit
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 3: Add innate_technologies to `content/archetypes/drifter.yaml`**

Find the line `technology_grants:` (line ~12) and insert before it:

```yaml
innate_technologies:
  - id: moisture_reclaim
    uses_per_day: 0
  - id: viscous_spray
    uses_per_day: 0
  - id: nanite_infusion
    uses_per_day: 0
  - id: acid_spit
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 4: Add innate_technologies to `content/archetypes/schemer.yaml`**

Find the line `technology_grants:` (line ~13) and insert before it:

```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 0
  - id: chrome_reflex
    uses_per_day: 0
  - id: neural_flare
    uses_per_day: 0
  - id: static_veil
    uses_per_day: 0
  - id: synapse_tap
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 5: Add innate_technologies to `content/archetypes/influencer.yaml`**

Find the line `technology_grants:` and insert before it:

```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 0
  - id: chrome_reflex
    uses_per_day: 0
  - id: neural_flare
    uses_per_day: 0
  - id: static_veil
    uses_per_day: 0
  - id: synapse_tap
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 6: Add innate_technologies to `content/archetypes/zealot.yaml`**

Find the line `technology_grants:` (line ~13) and insert before it:

```yaml
innate_technologies:
  - id: doctrine_ward
    uses_per_day: 0
  - id: martyrs_resolve
    uses_per_day: 0
  - id: righteous_condemnation
    uses_per_day: 0
  - id: fervor_pulse
    uses_per_day: 0
  - id: litany_of_iron
    uses_per_day: 0
technology_grants:
```

- [ ] **Step 7: Run the loader test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadArchetypes_TechCapable_AllHaveInnate -v
```

Expected: `PASS`

- [ ] **Step 8: Run the full ruleset package tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -v 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add content/archetypes/nerd.yaml \
        content/archetypes/naturalist.yaml \
        content/archetypes/drifter.yaml \
        content/archetypes/schemer.yaml \
        content/archetypes/influencer.yaml \
        content/archetypes/zealot.yaml
git commit -m "feat(content): populate archetype innate_technologies for all 6 tech-capable archetypes"
```

---

### Task 5: AssignTechnologies integration tests for tradition innate grants

**Files:**
- Modify: `internal/gameserver/technology_assignment_test.go`

These tests use in-memory fakes (the `fakeInnateRepo`, `fakeHardwiredRepo`, etc. already in the file) and verify that the archetype innate grants flow through correctly via `AssignTechnologies`.

- [ ] **Step 1: Add the three integration tests**

Append these tests to `internal/gameserver/technology_assignment_test.go`:

```go
// REQ-TIT-1 (unit): AssignTechnologies grants all 5 technical innate techs for a nerd archetype
// with all techs at uses_per_day: 0 (unlimited).
//
// Precondition: archetype.InnateTechnologies has 5 technical innate grants with UsesPerDay=0.
// Postcondition: sess.InnateTechs has exactly 5 entries, each with MaxUses=0 and UsesRemaining=0.
func TestAssignTechnologies_TraditionInnate_NerdGetsAllTechnicalTechs(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	archetype := &ruleset.Archetype{
		ID: "nerd",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "atmospheric_surge", UsesPerDay: 0},
			{ID: "blackout_pulse", UsesPerDay: 0},
			{ID: "seismic_sense", UsesPerDay: 0},
			{ID: "arc_lights", UsesPerDay: 0},
			{ID: "pressure_burst", UsesPerDay: 0},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, archetype, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.NoError(t, err)

	require.Len(t, sess.InnateTechs, 5, "nerd must receive all 5 technical innate techs")
	for _, id := range []string{"atmospheric_surge", "blackout_pulse", "seismic_sense", "arc_lights", "pressure_burst"} {
		slot, ok := sess.InnateTechs[id]
		require.True(t, ok, "expected innate tech %q in session", id)
		assert.Equal(t, 0, slot.MaxUses, "innate tech %q must have MaxUses=0 (unlimited)", id)
		assert.Equal(t, 0, slot.UsesRemaining, "innate tech %q must have UsesRemaining=0 (unlimited)", id)
	}
}

// REQ-TIT-1 (unit): AssignTechnologies grants all 5 fanatic_doctrine innate techs for a zealot archetype.
//
// Precondition: archetype.InnateTechnologies has 5 fanatic_doctrine grants with UsesPerDay=0.
// Postcondition: sess.InnateTechs has exactly 5 entries, each with MaxUses=0 and UsesRemaining=0.
func TestAssignTechnologies_TraditionInnate_ZealotGetsAllDoctrineInnate(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	archetype := &ruleset.Archetype{
		ID: "zealot",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "doctrine_ward", UsesPerDay: 0},
			{ID: "martyrs_resolve", UsesPerDay: 0},
			{ID: "righteous_condemnation", UsesPerDay: 0},
			{ID: "fervor_pulse", UsesPerDay: 0},
			{ID: "litany_of_iron", UsesPerDay: 0},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, archetype, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.NoError(t, err)

	require.Len(t, sess.InnateTechs, 5, "zealot must receive all 5 fanatic_doctrine innate techs")
	for _, id := range []string{"doctrine_ward", "martyrs_resolve", "righteous_condemnation", "fervor_pulse", "litany_of_iron"} {
		slot, ok := sess.InnateTechs[id]
		require.True(t, ok, "expected innate tech %q in session", id)
		assert.Equal(t, 0, slot.MaxUses, "innate tech %q must have MaxUses=0 (unlimited)", id)
		assert.Equal(t, 0, slot.UsesRemaining, "innate tech %q must have UsesRemaining=0 (unlimited)", id)
	}
}

// REQ-TIT-5 (unit): Region innate tech is granted IN ADDITION to archetype tradition innate techs.
//
// Precondition: archetype has 2 innate techs; region has 1 distinct innate tech.
// Postcondition: sess.InnateTechs has 3 entries (2 archetype + 1 region).
func TestAssignTechnologies_RegionInnateAdditiveToTraditionInnate(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	archetype := &ruleset.Archetype{
		ID: "naturalist",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "moisture_reclaim", UsesPerDay: 0},
			{ID: "acid_spit", UsesPerDay: 0},
		},
	}
	region := &ruleset.Region{
		ID: "southeast_portland",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "nanite_infusion", UsesPerDay: 0},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, archetype, nil, noPrompt, hw, prep, spont, inn, nil, region)
	require.NoError(t, err)

	require.Len(t, sess.InnateTechs, 3,
		"region innate must be additive: 2 archetype techs + 1 region tech = 3 total")
	assert.NotNil(t, sess.InnateTechs["moisture_reclaim"], "archetype innate must be present")
	assert.NotNil(t, sess.InnateTechs["acid_spit"], "archetype innate must be present")
	assert.NotNil(t, sess.InnateTechs["nanite_infusion"], "region innate must be present")
}

// REQ-TIT-1 (property): For any set of N distinct innate grants with UsesPerDay=0 on a
// tech-capable archetype, AssignTechnologies grants exactly N slots all with MaxUses=0.
//
// Precondition: archetype.ID = "nerd" (tech-capable); InnateTechnologies has N grants, UsesPerDay=0.
// Postcondition: len(sess.InnateTechs) == N; every slot has MaxUses=0.
func TestProperty_AssignTechnologies_TraditionGrantsAllUnlimited(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		grants := make([]ruleset.InnateGrant, n)
		for i := 0; i < n; i++ {
			grants[i] = ruleset.InnateGrant{
				ID:         fmt.Sprintf("tradition_tech_%d", i),
				UsesPerDay: 0,
			}
		}

		archetype := &ruleset.Archetype{
			ID:                 "nerd",
			InnateTechnologies: grants,
		}

		sess := &session.PlayerSession{}
		inn := &fakeInnateRepo{}

		err := gameserver.AssignTechnologies(context.Background(), sess, 1, nil, archetype, nil,
			noPrompt, &fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, inn, nil, nil)
		if err != nil {
			rt.Fatalf("AssignTechnologies: %v", err)
		}

		if len(sess.InnateTechs) != n {
			rt.Fatalf("expected %d innate slots, got %d", n, len(sess.InnateTechs))
		}
		for id, slot := range sess.InnateTechs {
			if slot.MaxUses != 0 {
				rt.Fatalf("innate tech %q: MaxUses=%d, want 0 (unlimited)", id, slot.MaxUses)
			}
		}
	})
}
```

- [ ] **Step 2: Run the new tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... \
  -run "TestAssignTechnologies_TraditionInnate|TestAssignTechnologies_RegionInnateAdditive|TestProperty_AssignTechnologies_TraditionGrantsAllUnlimited" \
  -v
```

Expected: all 4 tests PASS (they exercise existing behaviour via the fakes — no new code needed)

- [ ] **Step 3: Run full gameserver tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -v 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/technology_assignment_test.go
git commit -m "test(gameserver): add tradition innate tech grant integration tests (REQ-TIT-1, TIT-5)"
```

---

### Task 6: DB migration 063 — backfill tradition innate techs for existing characters

**Files:**
- Create: `migrations/063_tradition_innate_techs.up.sql`
- Create: `migrations/063_tradition_innate_techs.down.sql`

- [ ] **Step 1: Create `migrations/063_tradition_innate_techs.up.sql`**

```sql
-- REQ-TIT-4: Insert missing tradition innate techs for all existing tech-capable characters.
-- ON CONFLICT DO NOTHING makes each INSERT idempotent — safe to run multiple times.
-- Job class names are the canonical IDs stored in characters.class.

-- technical tradition: nerd jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('atmospheric_surge'), ('blackout_pulse'), ('seismic_sense'),
    ('arc_lights'), ('pressure_burst')
) AS t(tech_id)
WHERE c.class IN (
    'natural_mystic', 'specialist', 'detective', 'journalist', 'hoarder',
    'grease_monkey', 'narc', 'engineer', 'cooker'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- bio_synthetic tradition: naturalist and drifter jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('moisture_reclaim'), ('viscous_spray'), ('nanite_infusion'), ('acid_spit')
) AS t(tech_id)
WHERE c.class IN (
    'rancher', 'hippie', 'laborer', 'hobo', 'tracker', 'freegan',
    'exterminator', 'fallen_trustafarian',
    'scout', 'cop', 'psychopath', 'driver', 'bagman', 'pilot',
    'warden', 'stalker', 'pirate', 'free_spirit'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- neural tradition: schemer and influencer jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('terror_broadcast'), ('chrome_reflex'), ('neural_flare'),
    ('static_veil'), ('synapse_tap')
) AS t(tech_id)
WHERE c.class IN (
    'narcomancer', 'maker', 'grifter', 'dealer', 'shit_stirrer',
    'salesman', 'mall_ninja', 'illusionist',
    'karen', 'politician', 'libertarian', 'entertainer', 'antifa',
    'bureaucrat', 'exotic_dancer', 'schmoozer', 'extortionist', 'anarchist'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- fanatic_doctrine tradition: zealot jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('doctrine_ward'), ('martyrs_resolve'), ('righteous_condemnation'),
    ('fervor_pulse'), ('litany_of_iron')
) AS t(tech_id)
WHERE c.class IN (
    'cult_leader', 'street_preacher', 'medic', 'guard', 'believer',
    'hired_help', 'vigilante', 'follower', 'trainee', 'pastor'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;
```

- [ ] **Step 2: Create `migrations/063_tradition_innate_techs.down.sql`**

```sql
-- Inserted rows represent correct game state.
-- Reversing this migration would remove valid innate techs from characters.
-- No-op.
```

- [ ] **Step 3: Verify the migration file is picked up by golang-migrate numbering**

```bash
ls -1 /home/cjohannsen/src/mud/migrations/06*.sql | sort
```

Expected output includes `063_tradition_innate_techs.up.sql` and `063_tradition_innate_techs.down.sql` after the 062 files.

- [ ] **Step 4: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected: all packages report `ok`. No `FAIL`.

- [ ] **Step 5: Commit**

```bash
git add migrations/063_tradition_innate_techs.up.sql \
        migrations/063_tradition_innate_techs.down.sql
git commit -m "feat(migration): 063 backfill tradition innate techs for existing characters (REQ-TIT-4)"
```

---

### Task 7: Final integration — verify full test suite and push

- [ ] **Step 1: Run complete test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -30
```

Expected: all packages `ok`, zero `FAIL`.

- [ ] **Step 2: Push to remote**

```bash
git push
```

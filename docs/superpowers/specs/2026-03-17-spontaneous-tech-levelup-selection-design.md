# Spontaneous Tech Level-Up Selection — Design Spec

**Date:** 2026-03-17

---

## Goal

Add two new Neural level-1 spontaneous technologies (`neural_static`, `synaptic_surge`) and wire the Influencer archetype to grant one known tech at levels 3 and 5 via interactive selection. Verify the existing deferral mechanism works end-to-end with property-based tests.

---

## Context

`PartitionTechGrants`, `fillFromSpontaneousPool`, and `ResolvePendingTechGrants` already implement the full deferred-selection flow for spontaneous `KnownByLevel` grants. The Influencer's level-up grants (levels 2–5) currently contain only `UsesByLevel` — no `KnownByLevel`. Only one Neural tech (`mind_spike`) exists, giving players no real choice. This sub-project adds content and tests; it does not change the mechanism.

**Out of scope:** Effect resolution for new techs; new commands; DB or proto changes; changes to any selection mechanism code.

---

## Feature 1: New Technology Content

### 1a. `neural_static`

File: `content/technologies/neural/neural_static.yaml`

`save_type: reflex` is a valid value per `internal/game/technology/model.go` (verified in model tests).

```yaml
id: neural_static
name: Neural Static
description: Floods a target's sensory nerves with dissonant white-noise, slowing their reactions.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
save_type: reflex
save_dc: 15
effects:
  - type: condition
    condition_id: slowed
    duration: rounds:1
amped_level: 3
amped_effects:
  - type: condition
    condition_id: slowed
    duration: rounds:2
```

### 1b. `synaptic_surge`

File: `content/technologies/neural/synaptic_surge.yaml`

```yaml
id: synaptic_surge
name: Synaptic Surge
description: Overwhelms a target's nervous system with a burst of pain signals.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
save_type: will
save_dc: 15
effects:
  - type: damage
    dice: 2d4
    damage_type: neural
  - type: condition
    condition_id: frightened
    value: 1
    duration: rounds:1
amped_level: 3
amped_effects:
  - type: damage
    dice: 4d4
    damage_type: neural
  - type: condition
    condition_id: frightened
    value: 2
    duration: rounds:1
```

---

## Feature 2: Influencer Archetype Update

File: `content/archetypes/influencer.yaml`

**This is a merge into the existing file, not a replacement.** Add `known_by_level` and `pool` keys inside the existing level 3 and level 5 entries. The complete resulting `level_up_grants` block:

```yaml
level_up_grants:
  2:
    spontaneous:
      uses_by_level:
        1: 1
  3:
    spontaneous:
      uses_by_level:
        1: 1
      known_by_level:
        1: 1
      pool:
        - id: mind_spike
          level: 1
        - id: neural_static
          level: 1
        - id: synaptic_surge
          level: 1
  4:
    spontaneous:
      uses_by_level:
        1: 1
  5:
    spontaneous:
      uses_by_level:
        1: 1
      known_by_level:
        1: 1
      pool:
        - id: mind_spike
          level: 1
        - id: neural_static
          level: 1
        - id: synaptic_surge
          level: 1
```

**Precondition:** `PartitionTechGrants` receives merged grants with `KnownByLevel[1] = 1` and pool of 3 techs; open slots = 1. Since `pool (3) > open (1)`, the grant is always deferred to `PendingTechGrants`.

**Postcondition:** After `selecttech <id>`, the chosen tech appears in `sess.SpontaneousTechs[1]` and `PendingTechGrants` is cleared.

---

## Feature 3: End-to-End Tests

All new grpc-level tests go in `internal/gameserver/grpc_service_spontaneous_selection_test.go` in **package `gameserver`** (same package as `grpc_service_selecttech_test.go`), so unexported helpers like `testMinimalService` and `fakeSessionStream` are accessible.

`handleSelectTech` does not accept a tech ID argument. It delegates to `ResolvePendingTechGrants` → `fillFromSpontaneousPool` → `promptFn` → `promptFeatureChoice`, which:
1. Sends a numbered list of options as a `MessageEvent` to the stream
2. Reads the player's numeric choice from `stream.Recv()`

Tests inject choices by pre-populating `stream.recv` with `ClientMessage` values containing a `SayMessage` whose `Message` field is the numeric option string (e.g. `"2"`).

Test setup for TEST-SSL1/2/3: directly assign `sess.PendingTechGrants` to `map[int]*ruleset.TechnologyGrants{3: {Spontaneous: &ruleset.SpontaneousGrants{KnownByLevel: map[int]int{1: 1}, Pool: []ruleset.SpontaneousEntry{{ID:"mind_spike",Level:1},{ID:"neural_static",Level:1},{ID:"synaptic_surge",Level:1}}}}}`, matching the pattern in `grpc_service_selecttech_test.go` lines 87–92.

The property test (TEST-SSL4) goes in `internal/gameserver/technology_assignment_test.go` in **package `gameserver_test`**. Since `fillFromSpontaneousPool` is unexported, TEST-SSL4 exercises the observable behavior through `LevelUpTechnologies` using a grant with `KnownByLevel > 0`, `pool size > open slots`, and **no `Fixed` entries** (so only prompt-chosen techs populate `SpontaneousTechs[level]`).

### TEST-SSL1: `selecttech` resolves spontaneous grant when valid choice submitted

- Setup: session with `PendingTechGrants` as above; `stream.recv = [SayMessage{"2"}]` (selecting `neural_static`, option 2)
- Action: call `handleSelectTech(uid, "req1", stream)`
- Assert: `sess.SpontaneousTechs[1]` contains `"neural_static"`; `sess.PendingTechGrants` is empty

### TEST-SSL2: `selecttech` sends "Invalid selection" when out-of-range choice submitted

- Setup: same pending grants; `stream.recv = [SayMessage{"99"}]` (invalid option)
- Action: call `handleSelectTech(uid, "req1", stream)`
- Assert: at least one `stream.sent` message contains `"Invalid selection"`; `sess.SpontaneousTechs[1]` is empty

**Note:** `ResolvePendingTechGrants` unconditionally clears the pending grant entry after `LevelUpTechnologies` returns, even when the promptFn returned `""` due to an invalid selection. As a result, `sess.PendingTechGrants` will also be empty after an invalid choice (grant silently lost). The test documents this actual behavior. Fixing this silent-loss edge case is out of scope for this sub-project.

### TEST-SSL3: `selecttech` prompt includes all three pool tech names

- Setup: same pending grants; `stream.recv = [SayMessage{"1"}]` (selects first option; ensures stream doesn't EOF before prompt is sent)
- Action: call `handleSelectTech(uid, "req1", stream)`
- Assert: the sent messages (before the final confirmation) contain all three tech names: `"mind_spike"`, `"neural_static"`, `"synaptic_surge"` (the prompt lists them by ID or name)

### TEST-SSL4 (property): `LevelUpTechnologies` deferred selection via promptFn

File: `internal/gameserver/technology_assignment_test.go` (package `gameserver_test`)

- Rapid generates: pool of 2–6 distinct tech IDs; N open slots where `1 ≤ N < pool size` (strictly less so promptFn is always invoked, not auto-assigned)
- Call `LevelUpTechnologies` with a grant containing `Spontaneous.KnownByLevel[level]=N` and the generated pool; provide a `promptFn` that records calls and returns pool items in order
- Invariants:
  - `promptFn` called exactly N times
  - Each selected ID is from the pool
  - No duplicate IDs selected
  - `sess.SpontaneousTechs[level]` has exactly N entries after completion

---

## Requirements

- REQ-SSL1: `selecttech` MUST resolve a deferred spontaneous grant by adding the chosen tech to `SpontaneousTechs` and clearing `PendingTechGrants`
- REQ-SSL2: `selecttech` MUST send an "Invalid selection" message when the submitted numeric choice is out of range; the pending grant is cleared after the attempt regardless of whether a tech was assigned (known limitation — grant is silently lost on invalid input)
- REQ-SSL3: `selecttech` with no argument MUST list available pool options when a deferred grant is pending
- REQ-SSL4: `LevelUpTechnologies` MUST invoke `promptFn` exactly once per open spontaneous slot when `pool size > open slots`
- REQ-CONTENT1: New tech files MUST follow the same YAML schema as `mind_spike.yaml`
- REQ-CONTENT2: Influencer pool at levels 3 and 5 MUST contain all three Neural level-1 techs
- REQ-SCOPE1: No mechanism code changes — only content files, YAML, and tests are modified

---

## Files Changed

| Action | Path | Notes |
|--------|------|-------|
| Create | `content/technologies/neural/neural_static.yaml` | New Neural level-1 tech |
| Create | `content/technologies/neural/synaptic_surge.yaml` | New Neural level-1 tech |
| Modify | `content/archetypes/influencer.yaml` | Add `known_by_level` + pool at levels 3 and 5 |
| Create | `internal/gameserver/grpc_service_spontaneous_selection_test.go` | TEST-SSL1, SSL2, SSL3 (package `gameserver`) |
| Modify | `internal/gameserver/technology_assignment_test.go` | Add TEST-SSL4 property test |
| Modify | `docs/requirements/FEATURES.md` | Mark Sub-project B items complete |

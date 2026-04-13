# Spec: Zone-Specific Quests — Rustbucket Ridge and Vantucky

**GitHub Issue:** cory-johannsen/mud#55
**Date:** 2026-04-13

---

## Context

The quest system is structurally complete: `QuestDef`, `QuestRegistry`, `quest.Service`, `QuestRepository`, and `grpc_service_quest_giver.go` all exist. However:

- `content/quests/` contains only one placeholder file (`example_kill_quest.yaml`)
- No `-quests-dir` CLI flag exists; the `QuestRegistry` is never populated at startup
- Rustbucket Ridge's `rustbucket_ridge_quest_giver` NPC has no `quest_ids`
- Vantucky has no quest giver NPC or spawn

This spec covers the minimum viable work to make quest givers functional in Rustbucket Ridge and Vantucky.

---

## REQ-1: Wire quest loading at server startup

- REQ-1a: `cmd/gameserver/main.go` MUST add a CLI flag `-quests-dir` defaulting to `content/quests`
- REQ-1b: At startup, `quest.LoadFromDir(*questsDir)` MUST be called and the resulting registry MUST be passed into `ContentDeps.QuestRegistry`
- REQ-1c: If `LoadFromDir` returns an error, the server MUST fatal at startup with a descriptive message
- REQ-1d: `ValidateQuestRegistry()` MUST be called after loading and MUST fatal on any cross-reference error (this already occurs at line 209 of `main.go`; REQ-1b makes it non-trivially exercised)

---

## REQ-2: Rustbucket Ridge quests

The Dispatch Board Operator (`rustbucket_ridge_quest_giver`) in the Scrap Office (`rust_scrap_office`) MUST offer the following quests. Each quest file MUST be created in `content/quests/`.

### REQ-2a: `rrq_scavenger_sweep` — "Scavenger Sweep"

```yaml
id: rrq_scavenger_sweep
title: Scavenger Sweep
description: >
  Scavengers keep picking through our supplies on the Ridge. Go put five of them
  down and maybe they'll get the message.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_scavengers
    type: kill
    description: Kill 5 scavengers
    target_id: scavenger
    quantity: 5
rewards:
  xp: 150
  credits: 75
```

### REQ-2b: `rrq_rail_gang_bounty` — "Rail Gang Bounty"

```yaml
id: rrq_rail_gang_bounty
title: Rail Gang Bounty
description: >
  Rail Gang's been pushing into our territory again. Take out five of their
  raiders and collect the bounty.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_raiders
    type: kill
    description: Kill 5 rail gang raiders
    target_id: rail_gang_raider
    quantity: 5
rewards:
  xp: 200
  credits: 100
```

### REQ-2c: `rrq_barrel_house_cleanup` — "Barrel House Cleanup"

```yaml
id: rrq_barrel_house_cleanup
title: Barrel House Cleanup
description: >
  Pete's muscle has been shaking down locals near the Barrel House. Go
  put two of his enforcers down and remind him this is still our Ridge.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: false
objectives:
  - id: kill_enforcers
    type: kill
    description: Kill 2 Barrel House Enforcers
    target_id: barrel_house_enforcer
    quantity: 2
rewards:
  xp: 250
  credits: 125
```

### REQ-2d: `rrq_take_down_big_grizz` — "Take Down Big Grizz"

```yaml
id: rrq_take_down_big_grizz
title: Take Down Big Grizz
description: >
  Big Grizz runs the Keg Hole and he's been bleeding this district dry.
  Nobody's been able to touch him. Show us you can.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: false
prerequisites:
  - rrq_barrel_house_cleanup
objectives:
  - id: kill_big_grizz
    type: kill
    description: Kill Big Grizz
    target_id: big_grizz
    quantity: 1
rewards:
  xp: 500
  credits: 300
```

---

## REQ-3: Update Rustbucket Ridge quest giver NPC

- REQ-3a: `content/npcs/rustbucket_ridge_quest_giver.yaml` MUST add `quest_ids` listing all four quests:

```yaml
quest_giver:
  placeholder_dialog:
    - "Got a few jobs posted. Nothing fancy — mostly recovery and recon. Pay's fair."
    - "Board's full. Pick something and stop blocking the doorway."
  quest_ids:
    - rrq_scavenger_sweep
    - rrq_rail_gang_bounty
    - rrq_barrel_house_cleanup
    - rrq_take_down_big_grizz
```

---

## REQ-4: Vantucky quest giver NPC template

- REQ-4a: A new NPC template `content/npcs/vantucky_quest_giver.yaml` MUST be created:

```yaml
id: vantucky_quest_giver
name: "Militia Quartermaster"
npc_type: quest_giver
npc_role: quest_giver
type: human
level: 3
max_hp: 22
ac: 10
awareness: 5
disposition: neutral
personality: neutral
quest_giver:
  placeholder_dialog:
    - "We've got work if you've got the stomach for it. Check the board."
    - "You looking to earn, or just taking up space?"
  quest_ids:
    - vtq_militia_patrol
    - vtq_scavenger_drive
    - vtq_bandit_bounty
    - vtq_gang_enforcer_takedown
```

---

## REQ-5: Spawn Vantucky quest giver in The Compound

- REQ-5a: `content/zones/vantucky.yaml`, room `vantucky_the_compound`, MUST add a spawn for the new quest giver:

```yaml
- template: vantucky_quest_giver
  count: 1
  respawn_after: 0s
```

---

## REQ-6: Vantucky quests

Each quest file MUST be created in `content/quests/`.

### REQ-6a: `vtq_militia_patrol` — "Neutralize the Militia Patrol"

```yaml
id: vtq_militia_patrol
title: Neutralize the Militia Patrol
description: >
  Militia patrols are making the roads impassable. Take out three of them
  and open the routes back up.
giver_npc_id: vantucky_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_militiamen
    type: kill
    description: Kill 3 Vantucky militiamen
    target_id: vantucky_militiaman
    quantity: 3
rewards:
  xp: 175
  credits: 90
```

### REQ-6b: `vtq_scavenger_drive` — "Drive Out the Scavengers"

```yaml
id: vtq_scavenger_drive
title: Drive Out the Scavengers
description: >
  Scavengers are stripping the Landing and the Creek. Drive four of them off
  for good.
giver_npc_id: vantucky_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_scavengers
    type: kill
    description: Kill 4 Vantucky scavengers
    target_id: vantucky_scavenger
    quantity: 4
rewards:
  xp: 150
  credits: 75
```

### REQ-6c: `vtq_bandit_bounty` — "Highway Bandit Bounty"

```yaml
id: vtq_bandit_bounty
title: Highway Bandit Bounty
description: >
  Highway bandits are taxing every crew that tries to move through I-84.
  Three of them, dead. That's the job.
giver_npc_id: vantucky_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_bandits
    type: kill
    description: Kill 3 highway bandits
    target_id: highway_bandit
    quantity: 3
rewards:
  xp: 200
  credits: 100
```

### REQ-6d: `vtq_gang_enforcer_takedown` — "Gang Enforcer Takedown"

```yaml
id: vtq_gang_enforcer_takedown
title: Gang Enforcer Takedown
description: >
  The gang's enforcers are running protection rackets all over the east side.
  Put three of them down and send a message.
giver_npc_id: vantucky_quest_giver
repeatable: true
cooldown: 24h
objectives:
  - id: kill_enforcers
    type: kill
    description: Kill 3 Vantucky gang enforcers
    target_id: vantucky_gang_enforcer
    quantity: 3
rewards:
  xp: 225
  credits: 110
```

---

## REQ-7: Quest cross-validation

- REQ-7a: All NPC `target_id` values in objectives MUST resolve to existing NPC template IDs in the NPC registry
- REQ-7b: All `giver_npc_id` values MUST resolve to existing NPC templates with `npc_type: quest_giver`
- REQ-7c: All `prerequisites` quest IDs MUST resolve to other quests in the registry
- REQ-7d: Server startup MUST fail if cross-validation fails (enforced by existing `ValidateQuestRegistry` + new REQ-1c)

---

## Files to Create / Modify

| File | Change |
|------|--------|
| `cmd/gameserver/main.go` | Add `-quests-dir` flag; load and inject `QuestRegistry` |
| `content/quests/rrq_scavenger_sweep.yaml` | New |
| `content/quests/rrq_rail_gang_bounty.yaml` | New |
| `content/quests/rrq_barrel_house_cleanup.yaml` | New |
| `content/quests/rrq_take_down_big_grizz.yaml` | New |
| `content/quests/vtq_militia_patrol.yaml` | New |
| `content/quests/vtq_scavenger_drive.yaml` | New |
| `content/quests/vtq_bandit_bounty.yaml` | New |
| `content/quests/vtq_gang_enforcer_takedown.yaml` | New |
| `content/npcs/rustbucket_ridge_quest_giver.yaml` | Add `quest_ids` |
| `content/npcs/vantucky_quest_giver.yaml` | New |
| `content/zones/vantucky.yaml` | Add quest giver spawn to `vantucky_the_compound` |

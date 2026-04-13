# Plan: Zone-Specific Quests — Rustbucket Ridge and Vantucky

**GitHub Issue:** cory-johannsen/mud#55
**Spec:** `docs/superpowers/specs/2026-04-13-zone-quests-rrq-vtq.md`
**Date:** 2026-04-13

---

## Step 1 — TDD: failing test for quest loading wiring

**File:** `cmd/gameserver/wire_gen_test.go` (new) or `internal/game/quest/registry_test.go` (extend)

Write a test asserting that `quest.LoadFromDir("../../../content/quests")` returns a non-empty registry with all 8 expected quest IDs once the YAML files are created. This test MUST fail before Step 3.

```go
func TestLoadFromDir_AllZoneQuests(t *testing.T) {
    reg, err := quest.LoadFromDir("../../../content/quests")
    if err != nil {
        t.Fatalf("LoadFromDir: %v", err)
    }
    want := []string{
        "rrq_scavenger_sweep", "rrq_rail_gang_bounty",
        "rrq_barrel_house_cleanup", "rrq_take_down_big_grizz",
        "vtq_militia_patrol", "vtq_scavenger_drive",
        "vtq_bandit_bounty", "vtq_gang_enforcer_takedown",
    }
    for _, id := range want {
        if reg[id] == nil {
            t.Errorf("missing quest %q", id)
        }
    }
}
```

```bash
mise exec -- go test ./internal/game/quest/... -count=1 -run TestLoadFromDir_AllZoneQuests -v
```

Expected: **FAIL** (quest YAML files don't exist yet).

---

## Step 2 — Wire `-quests-dir` flag and `AppConfig` field

**File:** `cmd/gameserver/main.go`

### Step 2a — Add `QuestsDir` to `AppConfig`

After `DowntimeQueueLimitsFile string` (line ~66), add:

```go
QuestsDir string
```

### Step 2b — Add CLI flag

After `downtimeQueueLimitsFile` flag declaration (line ~109), add:

```go
questsDir := flag.String("quests-dir", "content/quests", "path to quest YAML files directory")
```

### Step 2c — Populate in `appCfg`

After `DowntimeQueueLimitsFile: *downtimeQueueLimitsFile,` (line ~167), add:

```go
QuestsDir: *questsDir,
```

---

## Step 3 — Create 8 quest YAML files

**Directory:** `content/quests/`

### `rrq_scavenger_sweep.yaml`

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

### `rrq_rail_gang_bounty.yaml`

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

### `rrq_barrel_house_cleanup.yaml`

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

### `rrq_take_down_big_grizz.yaml`

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

### `vtq_militia_patrol.yaml`

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

### `vtq_scavenger_drive.yaml`

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

### `vtq_bandit_bounty.yaml`

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

### `vtq_gang_enforcer_takedown.yaml`

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

## Step 4 — Load quest registry in `wire_gen.go`

**File:** `cmd/gameserver/wire_gen.go`

After the `downtimeQueueLimitRegistry` block (after line ~217), add:

```go
// Load quest registry (REQ-1a/b/c: fatal if dir is missing or any quest fails validation).
questRegistry, err := quest.LoadFromDir(cfg.QuestsDir)
if err != nil {
    return nil, fmt.Errorf("loading quest registry: %w", err)
}
```

In the `ContentDeps` struct literal (line ~218), add:

```go
QuestRegistry: questRegistry,
```

Add import for `"github.com/cory-johannsen/mud/internal/game/quest"` if not already present.

---

## Step 5 — Update `rustbucket_ridge_quest_giver.yaml`

**File:** `content/npcs/rustbucket_ridge_quest_giver.yaml`

Replace the existing `quest_giver` block:

```yaml
quest_giver:
  placeholder_dialog:
    - "Got a few jobs posted. Nothing fancy — mostly recovery and recon. Pay's fair."
    - "Board's full. Pick something and stop blocking the doorway."
    - "You looking for work or just sightseeing? Because this isn't the place for sightseeing."
  quest_ids:
    - rrq_scavenger_sweep
    - rrq_rail_gang_bounty
    - rrq_barrel_house_cleanup
    - rrq_take_down_big_grizz
```

---

## Step 6 — Create `vantucky_quest_giver.yaml`

**File:** `content/npcs/vantucky_quest_giver.yaml` (new)

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
abilities:
  brutality: 8
  grit: 11
  quickness: 10
  reasoning: 13
  savvy: 12
  flair: 10
quest_giver:
  placeholder_dialog:
    - "We've got work if you've got the stomach for it. Check the board."
    - "You looking to earn, or just taking up space?"
  quest_ids:
    - vtq_militia_patrol
    - vtq_scavenger_drive
    - vtq_bandit_bounty
    - vtq_gang_enforcer_takedown
loot:
  currency:
    min: 0
    max: 0
```

---

## Step 7 — Spawn quest giver in `vantucky_the_compound`

**File:** `content/zones/vantucky.yaml`

Find the `vantucky_the_compound` room's `spawns:` list. Add:

```yaml
- template: vantucky_quest_giver
  count: 1
  respawn_after: 0s
```

---

## Step 8 — Run test suite

```bash
mise exec -- go test ./internal/game/quest/... -count=1 -v
mise exec -- go build ./cmd/gameserver/...
```

Both MUST succeed before committing.

---

## Dependency Order

```
Step 1 (TDD — write failing test)
Step 2 (flag + AppConfig)
Step 3 (8 quest YAML files) ──▶ Step 1 passes
Step 4 (wire_gen.go) ──▶ needs Step 2 + Step 3
Step 5 (RRQ NPC update) — independent
Step 6 (VTQ NPC create) — independent
Step 7 (vantucky spawn) ──▶ needs Step 6
Step 8 (test + build) ──▶ all steps complete
```

Steps 2, 3, 5, 6 are independent and can run in parallel.

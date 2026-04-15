# Tiered Technology Acquisition Design

## Overview

Technology acquisition is tiered by level. Innate technologies and level 1 technologies are always granted automatically. Level 2 and higher technologies require the player to locate a tradition-specialized trainer NPC in the appropriate zone, meet any configured prerequisites, and pay a level-scaled cost. On level-up, players with pending L2+ grants receive a quest directing them to the right zone — but must explore to find the trainer.

---

## Requirements

- REQ-TTA-1: Innate technologies and level 1 technologies MUST be granted automatically at character creation and level-up with no trainer interaction required.
- REQ-TTA-2: Technology grants at level 2 and higher MUST always be deferred into `PendingTechGrants` and MUST NOT be auto-resolved at level-up.
- REQ-TTA-3: A new `tech_trainer` NPC type MUST be defined, specializing in one tradition and a configurable set of offered tech levels.
- REQ-TTA-4: A trainer MUST only teach technologies matching its configured `tradition` and within its `offered_levels`.
- REQ-TTA-5: Training cost MUST be computed as `base_cost × tech_level`, where `base_cost` is configured on the trainer NPC.
- REQ-TTA-6: Trainers MUST support optional prerequisites, each of type `quest_complete` or `faction_rep`, combined via a configurable `operator` of `"and"` or `"or"`.
- REQ-TTA-7: When a player gains a pending L2+ tech grant, the system MUST auto-issue the find-trainer quest associated with a matching trainer NPC, if one exists and has not already been issued.
- REQ-TTA-8: Find-trainer quests MUST state the zone where the trainer is located but MUST NOT reveal the trainer's exact position.
- REQ-TTA-9: A find-trainer quest MUST auto-complete on the player's first successful training session with a matching trainer.
- REQ-TTA-10: The `train` command MUST operate in two modes: list mode (shows learnable options from pending pool) and train mode (resolves one pending slot by tech ID).
- REQ-TTA-11: Training MUST deduct the computed cost from the player's currency and persist the change.
- REQ-TTA-12: Pending L2+ tech slots MUST be tracked with tradition and usage type in a new `character_pending_tech_slots` table.

---

## Section 1: Architecture

Four components are added or modified:

### 1.1 `PartitionTechGrants` (modified)

`internal/gameserver/technology_assignment.go`

The existing function is modified so any prepared or spontaneous grant at level ≥ 2 is unconditionally placed in `deferred`, regardless of pool size vs slot count. Level 1 grants retain existing behavior (auto-resolve if pool ≤ slots, defer otherwise).

### 1.2 `tech_trainer` NPC type (new)

`internal/game/npc/noncombat.go` — new `TechTrainerConfig` struct and YAML parsing.

`content/npcs/non_combat/<zone>.yaml` — trainer NPC entries per zone.

`content/quests/tech_trainers/<quest_id>.yaml` — find-trainer quest definitions.

### 1.3 Level-up quest auto-grant (modified)

`internal/gameserver/technology_assignment.go` — `LevelUpTechnologies` calls new `issueTechTrainerQuests` after creating pending grants.

### 1.4 `handleTrainTech` handler (new)

`internal/gameserver/grpc_service_tech_trainer.go` — handles `train` command, validates and resolves one pending slot.

---

## Section 2: Tech Trainer NPC Config

### 2.1 YAML Schema

```yaml
- id: vantucky_neural_trainer
  name: "Mama Zen"
  npc_type: tech_trainer
  type: human
  description: "Runs neural conditioning sessions out of a repurposed shipping container."
  level: 5
  max_hp: 30
  ac: 12
  awareness: 8
  disposition: neutral
  personality: neutral
  tech_trainer:
    tradition: neural           # one of: neural, technical, bio_synthetic, fanatic_doctrine
    offered_levels: [2, 3]      # tech levels this trainer can teach
    base_cost: 150              # cost = base_cost × tech_level
    find_quest_id: find_neural_trainer_vantucky  # auto-issued when matching pending slot opens
    prerequisites:              # optional; omit for no prerequisites
      operator: or              # "and" | "or"; defaults to "and" if omitted
      conditions:
        - type: quest_complete
          quest_id: mama_zen_intro_quest
        - type: faction_rep
          faction_id: gray_market
          min_tier: associate
```

### 2.2 Go Structs

```go
// TechTrainerConfig defines a tech trainer NPC's teaching capabilities and access rules.
//
// Precondition: Tradition is a valid technology tradition ID; OfferedLevels is non-empty.
type TechTrainerConfig struct {
    Tradition     string            `yaml:"tradition"`
    OfferedLevels []int             `yaml:"offered_levels"`
    BaseCost      int               `yaml:"base_cost"`
    FindQuestID   string            `yaml:"find_quest_id,omitempty"`
    Prerequisites *TechTrainPrereqs `yaml:"prerequisites,omitempty"`
}

// TechTrainPrereqs defines the prerequisite gate for accessing a tech trainer.
//
// Precondition: Operator is "and" or "or"; Conditions is non-empty.
type TechTrainPrereqs struct {
    Operator   string               `yaml:"operator"`
    Conditions []TechTrainCondition `yaml:"conditions"`
}

// TechTrainCondition is a single prerequisite condition.
//
// Precondition: Type is "quest_complete" or "faction_rep".
// Postcondition: Fields irrelevant to Type are ignored.
type TechTrainCondition struct {
    Type      string `yaml:"type"`
    QuestID   string `yaml:"quest_id,omitempty"`
    FactionID string `yaml:"faction_id,omitempty"`
    MinTier   string `yaml:"min_tier,omitempty"`
}
```

---

## Section 3: Level-Up Flow & Quest Auto-Grant

### 3.1 `PartitionTechGrants` Modification

Add a level threshold check before the existing pool-vs-slots logic:

```go
// Any grant at level >= 2 is unconditionally deferred — requires trainer.
if lvl >= 2 {
    // add to deferred
    continue
}
// Existing pool <= slots logic applies for level 1
```

### 3.2 Find-Trainer Quest Content Format

`content/quests/tech_trainers/<quest_id>.yaml`:

```yaml
id: find_neural_trainer_vantucky
title: "Neural Training Available"
description: >
  Your neural potential has expanded to level 2. A trainer in Vantucky
  can unlock it — ask around and explore to find them.
type: find_trainer
tradition: neural
min_level: 2
max_level: 3
auto_complete: true
```

- `type: find_trainer` — no kill/fetch/explore objectives; completion is triggered programmatically. The quest loader must be extended to parse this new type (no objective parsing required for this type).
- `auto_complete: true` — `handleTrainTech` completes this quest after the first successful training.

### 3.3 `issueTechTrainerQuests`

Called from `LevelUpTechnologies` after pending grants are written:

```
Precondition: uid non-empty; pendingGrants non-nil; npcMgr non-nil; questMgr non-nil.
Postcondition: For each pending L2+ grant, if a matching tech_trainer NPC has a FindQuestID
               and the player does not already have that quest, the quest is auto-granted.

for each (tradition, level) in new pending grants where level >= 2:
    trainers = npcMgr.FindTechTrainers(tradition, level)
    for each trainer in trainers:
        if trainer.FindQuestID != "" AND NOT questMgr.HasQuest(uid, trainer.FindQuestID):
            questMgr.AutoGrant(uid, trainer.FindQuestID)
            persist to character_trainer_quests
```

---

## Section 4: `handleTrainTech` Handler

**File:** `internal/gameserver/grpc_service_tech_trainer.go`

**Command:** `train <npc_name>` (list mode) or `train <npc_name> <tech_id>` (train mode)

### 4.1 List Mode

```
1. Find tech_trainer NPC in player's current room by name → error if not found or wrong type
2. Filter player's PendingTechGrants to entries matching trainer.Tradition AND level ∈ trainer.OfferedLevels
3. Return formatted list of learnable tech options with costs
```

### 4.2 Train Mode

```
Precondition: uid non-empty; npcName non-empty; techID non-empty.
Postcondition: On success — one pending slot is resolved, currency deducted, find-trainer quest completed.

1. Find tech_trainer NPC in player's current room by name → error if not found or wrong type
2. Determine tech level from techID by looking up TechnologyDef in registry
3. Verify level ∈ trainer.OfferedLevels → denial if not
4. Verify player has pending grant for trainer.Tradition at that level → denial if not
5. Verify techID is in the player's pending pool for that tradition+level → denial if not
6. Evaluate prerequisites via EvalTechTrainPrereqs → denial narrative if not satisfied
7. Compute cost = trainer.BaseCost × tech_level
8. Verify sess.Currency >= cost → denial if insufficient
9. Call ResolvePendingTechGrants filtered to this tradition+level+techID
10. Deduct cost; persist currency
11. Delete row from character_pending_tech_slots for (characterID, tech_level, tradition, usage_type)
12. If trainer.FindQuestID != "" AND quest not already complete:
        questMgr.Complete(uid, trainer.FindQuestID)
        update character_trainer_quests.completed_at
13. Return training narrative event
```

### 4.3 Prerequisite Evaluation

```go
// EvalTechTrainPrereqs returns nil if all prerequisites are satisfied per operator.
//
// Precondition: prereqs non-nil; sess non-nil; questMgr non-nil.
// Postcondition: Returns nil on pass; non-nil error with denial reason on fail.
func EvalTechTrainPrereqs(
    prereqs *npc.TechTrainPrereqs,
    sess *session.PlayerSession,
    questMgr QuestManager,
) error
```

- `"and"` operator: all conditions must pass.
- `"or"` operator: at least one condition must pass.
- `"quest_complete"`: `questMgr.IsComplete(uid, questID)`
- `"faction_rep"`: look up faction tier thresholds from faction registry; compare `sess.FactionRep[factionID]` against `min_tier` rep minimum.

---

## Section 5: Database Changes

### Migration 061 — `character_pending_tech_slots`

```sql
-- Tracks pending L2+ technology slots awaiting trainer resolution.
-- Replaces pending_tech_levels for L2+ grants (pending_tech_levels is retained for L1 deferred pool selection).
CREATE TABLE character_pending_tech_slots (
    character_id  BIGINT       NOT NULL,
    tech_level    INT          NOT NULL,
    tradition     TEXT         NOT NULL,
    usage_type    TEXT         NOT NULL,  -- "prepared" | "spontaneous"
    granted_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (character_id, tech_level, tradition, usage_type)
);
```

### Migration 062 — `character_trainer_quests`

```sql
-- Tracks which find-trainer quests have been auto-granted per character.
-- Prevents re-issuance on re-login and records completion.
CREATE TABLE character_trainer_quests (
    character_id  BIGINT       NOT NULL,
    quest_id      TEXT         NOT NULL,
    granted_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    PRIMARY KEY (character_id, quest_id)
);
```

No changes to `character_prepared_technologies`, `character_spontaneous_technologies`, use-pool tables, or `pending_tech_levels`.

---

## Section 6: Testing

- REQ-TTA-TEST-1: Property test — `PartitionTechGrants` with any grant at level ≥ 2 MUST always produce a non-empty `deferred` result and an empty immediate result for that level.
- REQ-TTA-TEST-2: Unit test — `EvalTechTrainPrereqs` with `"and"` operator MUST return error if any condition fails.
- REQ-TTA-TEST-3: Unit test — `EvalTechTrainPrereqs` with `"or"` operator MUST return nil if any one condition passes.
- REQ-TTA-TEST-4: Integration test — `handleTrainTech` with no matching trainer in room MUST return denial.
- REQ-TTA-TEST-5: Integration test — `handleTrainTech` with insufficient funds MUST return denial and MUST NOT modify tech slots or currency.
- REQ-TTA-TEST-6: Integration test — `handleTrainTech` success path MUST resolve exactly one pending slot, deduct correct cost, and complete the find-trainer quest.
- REQ-TTA-TEST-7: Integration test — `issueTechTrainerQuests` MUST NOT issue a quest the player already holds.
- REQ-TTA-TEST-8: Integration test — `handleTrainTech` with unmet prerequisites MUST return denial narrative and MUST NOT modify state.

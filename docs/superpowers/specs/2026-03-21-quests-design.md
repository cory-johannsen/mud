# Quests Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `quests` (priority 380)
**Dependencies:** `npc-behaviors` (for `QuestGiverConfig`, `PlaceholderDialog`); `xp` service; `inventory` backpack

---

## Overview

A data-driven quest system that gives players objectives (kill, fetch, explore, deliver) with XP, item, and credit rewards. Quests are defined in `content/quests/*.yaml`, managed by a `quest.Service` package, and persisted to two new database tables. Players interact via `talk <npc>` for offer/acceptance and a `quest` command namespace for tracking.

---

## 1. Data Model

### 1.1 Quest Definition YAML

Quest definitions live in `content/quests/*.yaml`. Each file defines one quest.

- REQ-QU-1: `QuestDef` MUST have fields: `ID string` (YAML: `id`), `Title string` (YAML: `title`), `Description string` (YAML: `description`), `GiverNPCID string` (YAML: `giver_npc_id`), `Repeatable bool` (YAML: `repeatable`), `Cooldown string` (YAML: `cooldown`; Go duration; meaningful only when `Repeatable == true`), `Prerequisites []string` (YAML: `prerequisites`; quest IDs), `Objectives []QuestObjective` (YAML: `objectives`), `Rewards QuestRewards` (YAML: `rewards`).
- REQ-QU-2: `QuestObjective` MUST have exported fields with YAML tags: `ID string` (YAML: `id`), `Type string` (YAML: `type`; valid values: `"kill"` | `"fetch"` | `"explore"` | `"deliver"`), `Description string` (YAML: `description`), `TargetID string` (YAML: `target_id`; semantics by type: NPC template ID for `kill`; item definition ID for `fetch`; room ID for `explore`; NPC template ID for `deliver`), `ItemID string` (YAML: `item_id`; used only for `deliver` objectives — the item that must be in inventory and is consumed on delivery), `Quantity int` (YAML: `quantity`; MUST be >= 1).
- REQ-QU-3: `QuestRewards` MUST have exported fields: `XP int` (YAML: `xp`), `Credits int` (YAML: `credits`), `Items []QuestRewardItem` (YAML: `items`). `QuestRewardItem` has fields `ItemID string` (YAML: `item_id`) and `Quantity int` (YAML: `quantity`).
- REQ-QU-4: `QuestDef.Validate()` MUST return an error if: `ID` or `Title` is empty; `GiverNPCID` is empty; `Objectives` is empty; any `QuestObjective` has empty `ID`, `Description`, or `TargetID`; any `QuestObjective.Type` is not one of the four canonical values; any `QuestObjective.Quantity < 1`; a `deliver` objective has empty `ItemID`; `Cooldown` is non-empty and fails `time.ParseDuration`; or `Repeatable == false` and `Cooldown` is non-empty (a non-repeatable quest MUST NOT carry a cooldown value).

### 1.2 Quest Registry

- REQ-QU-5: `QuestRegistry` MUST be a `map[string]*QuestDef` loaded from `content/quests/` at server startup and injected into `GameServiceServer`.
- REQ-QU-6: At startup, `QuestRegistry.Validate()` MUST cross-check all quest definitions against live registries: unknown `GiverNPCID` values in `NPCRegistry`; unknown `TargetID` values for `kill` objectives in `NPCRegistry`; unknown `TargetID` values for `fetch` objectives in `ItemRegistry`; unknown `TargetID` values for `deliver` objectives in `NPCRegistry` and unknown `ItemID` values for `deliver` objectives in `ItemRegistry`; unknown `TargetID` values for `explore` objectives in the world room set; unknown `prerequisites` quest IDs in `QuestRegistry`. Any unknown reference MUST cause a fatal startup error.

### 1.3 Runtime Types

- REQ-QU-7: `ActiveQuest` MUST have fields: `QuestID string`, `ObjectiveProgress map[string]int` (objective ID → count completed).
- REQ-QU-8: `QuestRecord` MUST have fields: `CharacterID int64`, `QuestID string`, `Status string` (`"active"` | `"completed"` | `"abandoned"`), `Progress map[string]int` (objective ID → count), `CompletedAt *time.Time`.
- REQ-QU-9: `PlayerSession` MUST gain `ActiveQuests map[string]*quest.ActiveQuest` and `CompletedQuests map[string]*time.Time` (quest ID → completion time; nil pointer value used as sentinel for abandoned one-time quests).

---

## 2. Database Schema

- REQ-QU-10: A `character_quests` table MUST be created with columns `character_id BIGINT NOT NULL REFERENCES characters(id)`, `quest_id TEXT NOT NULL`, `status TEXT NOT NULL`, `completed_at TIMESTAMPTZ`, and `PRIMARY KEY (character_id, quest_id)`.
- REQ-QU-11: A `character_quest_progress` table MUST be created with columns `character_id BIGINT NOT NULL REFERENCES characters(id)`, `quest_id TEXT NOT NULL`, `objective_id TEXT NOT NULL`, `progress INT NOT NULL DEFAULT 0`, and `PRIMARY KEY (character_id, quest_id, objective_id)`.
- REQ-QU-12: `QuestRepository` MUST be an interface with methods: `SaveQuestStatus(ctx context.Context, characterID int64, questID, status string, completedAt *time.Time) error`, `SaveObjectiveProgress(ctx context.Context, characterID int64, questID, objectiveID string, progress int) error`, `LoadQuests(ctx context.Context, characterID int64) ([]QuestRecord, error)`.
- REQ-QU-13: `LoadQuests` MUST return all rows for the character across both tables (active, completed, abandoned) in a single query joining `character_quests` and `character_quest_progress`.

---

## 3. Quest Lifecycle

### 3.1 Session Loading

- REQ-QU-14: At login, `LoadQuests` MUST be called and results hydrated into the session: `active` rows populate `sess.ActiveQuests` with objective progress restored; `completed` rows populate `sess.CompletedQuests` with the non-nil `completedAt` pointer; `abandoned` rows for non-repeatable quests populate `sess.CompletedQuests` with a nil `*time.Time` pointer (blocking re-acceptance); `abandoned` rows for repeatable quests are not loaded into `CompletedQuests` (allowing re-offer).

### 3.2 Offer (`talk <npc>`)

- REQ-QU-15: The `talk <npc>` command handler MUST call `quest.Service.GetOfferable(sess, questIDs)` for NPC templates whose `NPCType == "quest_giver"`, where `questIDs` is sourced from `npc.Template.QuestGiver.QuestIDs`. The call returns quests that pass the eligibility check (REQ-QU-16).
- REQ-QU-16: `GetOfferable` MUST evaluate eligibility for each quest ID in this order: (1) quest not in `sess.ActiveQuests`; (2) all prerequisite quest IDs present in `sess.CompletedQuests` with non-nil `completedAt` (an abandoned prerequisite with nil pointer does NOT satisfy this check); (3) if `Repeatable == false`, quest not present in `sess.CompletedQuests` at all; (4) if `Repeatable == true`, either not in `sess.CompletedQuests` or `time.Since(*completedAt) >= parsedCooldown`.
- REQ-QU-17: When offerable quests exist, the `talk` handler MUST display each quest's title, description, and reward summary, followed by the acceptance instruction `"Type 'talk <npc_name> accept <quest_id>' to accept."`. When no quests are offerable, the NPC's `PlaceholderDialog` MUST be shown.

### 3.3 Accept (`talk <npc> accept <quest_id>`)

- REQ-QU-18: `quest.Service.Accept(ctx, sess, characterID, questID)` MUST: validate the quest exists and is offerable to this player (re-running eligibility check); insert a `character_quests` row with `status="active"`; insert one `character_quest_progress` row per objective with `progress=0`; add an `ActiveQuest` entry to `sess.ActiveQuests`; return the quest title and all objective descriptions for display.

### 3.4 Progress Hooks

- REQ-QU-19: The NPC death handler MUST call `quest.Service.RecordKill(ctx, sess, characterID, npcTemplateID)` after every NPC death, incrementing `ObjectiveProgress` for all active `kill` objectives whose `TargetID` matches `npcTemplateID`, up to the objective's `Quantity`.
- REQ-QU-20: The `get` command handler MUST call `quest.Service.RecordFetch(ctx, sess, characterID, itemDefID, qty)` after each successful item pickup, incrementing `ObjectiveProgress` for matching active `fetch` objectives (where `TargetID == itemDefID`), clamped to `Quantity`.
- REQ-QU-21: `quest.Service.RecordExplore(ctx, sess, characterID, roomID)` MUST be called at both room discovery sites in `grpc_service.go`: (1) the spawn-room assignment path (when a character's starting room is first recorded); (2) the movement path (when `!sess.AutomapCache[zID][newRoom.ID]` is true before inserting the automap row). Both sites count toward `explore` objectives. The call sets `ObjectiveProgress` to `Quantity` (always 1) for any active `explore` objective whose `TargetID` matches `roomID`.
- REQ-QU-22: Each `Record*` method MUST persist the updated progress via `SaveObjectiveProgress`, then call `maybeComplete`.

### 3.5 Deliver Objectives

- REQ-QU-23: When `talk <npc>` is called and the player has one or more active `deliver` objectives with `TargetID` matching that NPC, the handler MUST evaluate each such objective in the order they appear in `ActiveQuests`. For each objective: (1) iterate `backpack.Items()` to find all item instances whose `ItemDefID == objective.ItemID`, summing their quantities; (2) if total quantity >= `objective.Quantity`, call `backpack.Remove` on matched instances (removing the exact required quantity, handling partial stacks) and call `quest.Service.RecordDeliver` to mark the objective complete; (3) if total quantity < `objective.Quantity`, display `"You don't have <item name>."` and skip that objective. Multiple deliver objectives for the same NPC are all evaluated in the same `talk` interaction.

### 3.6 Completion (automatic)

- REQ-QU-24: `maybeComplete` MUST check after every progress update whether all objectives in the quest have `ObjectiveProgress >= Quantity`. If satisfied, `quest.Service.Complete(ctx, sess, characterID, questID)` MUST: update `character_quests` to `status="completed"` and set `completed_at=now`; award XP via `xp.Service.AwardXPAmount`; grant item rewards via `backpack.Add` for each reward item, calling `SaveInventory` once after all items (overflow items dropped to room floor); add `def.Rewards.Credits` to `sess.Currency` and persist; move the quest from `sess.ActiveQuests` to `sess.CompletedQuests` with the non-nil completion time; send completion message and reward summary to the player.
- REQ-QU-25: The completion message MUST follow the format: first line `"Quest complete: <Title>"`; second line `"+<XP> XP | +<credits> credits"` (omit credits segment if zero); one additional line per reward item `"+<qty>x <item name>"` (omit entire item section if no item rewards).

### 3.7 Abandon (`quest abandon <id>`)

- REQ-QU-26: `quest abandon <id>` MUST update `character_quests` to `status="abandoned"` and remove the quest from `sess.ActiveQuests`. If `Repeatable == false`, the quest MUST be added to `sess.CompletedQuests` with a nil `*time.Time` pointer to permanently block re-acceptance.
- REQ-QU-27: If the quest being abandoned has `Repeatable == false`, the handler MUST require confirmation: display `"This quest cannot be retaken. Type 'quest abandon <id> confirm' to proceed."` and take no action until the `confirm` suffix is supplied.

---

## 4. `quest` Command Namespace

- REQ-QU-28: The `quest` command MUST support subcommands: `list`, `log <id>`, `abandon <id>` (with optional `confirm` suffix per REQ-QU-27), and `completed`.
- REQ-QU-29: `quest list` MUST display all active quests with per-quest objective completion ratio, e.g.: `[2/3] recover_the_shipment — Recover the Shipment`.
- REQ-QU-30: `quest log <id>` MUST display the quest title, description, each objective with completion status (`[x]` or `[ ]`) and current/target count, and the full reward list. If `<id>` refers to an active quest, current progress is shown. If `<id>` refers to a completed quest (present in `sess.CompletedQuests` with non-nil `completedAt`), the full detail MUST be shown with a `"(Completed: <timestamp>)"` header. If the quest is not found in either active or completed state, MUST return `"No quest found with that ID."`.
- REQ-QU-31: `quest completed` MUST list all quests in `sess.CompletedQuests` with a non-nil `completedAt`, showing title and completion timestamp.

---

## 5. Rewards

- REQ-QU-32: Reward application in `quest.Service.Complete` MUST occur in this order: (1) XP via `xp.Service.AwardXPAmount`; (2) items via `backpack.Add` for each reward item with overflow dropped to room floor, then `SaveInventory` called once; (3) credits added to `sess.Currency` and persisted.
- REQ-QU-33: `SaveInventory` MUST be called once after all item rewards are granted, not once per item.

---

## 6. Requirements Summary

- REQ-QU-1: `QuestDef` MUST have all specified fields with YAML tags.
- REQ-QU-2: `QuestObjective` MUST have exported fields with YAML tags; `TargetID` semantics vary by `Type` (NPC ID for kill/deliver, item ID for fetch, room ID for explore); `ItemID` used only for deliver; `Quantity >= 1`.
- REQ-QU-3: `QuestRewards` and `QuestRewardItem` MUST have exported fields with YAML tags.
- REQ-QU-4: `QuestDef.Validate()` MUST reject empty required fields, invalid objective types, deliver objectives without `ItemID`, invalid cooldown, and non-empty `Cooldown` on non-repeatable quests.
- REQ-QU-5: `QuestRegistry` loaded from `content/quests/` at startup, injected into `GameServiceServer`.
- REQ-QU-6: Startup cross-validation of all quest definition references against live registries (NPC, item, world, quest); unknown references are fatal errors.
- REQ-QU-7: `ActiveQuest` has `QuestID` and `ObjectiveProgress map[string]int`.
- REQ-QU-8: `QuestRecord` has `CharacterID`, `QuestID`, `Status`, `Progress map[string]int`, `CompletedAt *time.Time`.
- REQ-QU-9: `PlayerSession` gains `ActiveQuests map[string]*quest.ActiveQuest` and `CompletedQuests map[string]*time.Time` (nil pointer = abandoned one-time quest sentinel).
- REQ-QU-10: `character_quests` table with PK `(character_id, quest_id)`.
- REQ-QU-11: `character_quest_progress` table with PK `(character_id, quest_id, objective_id)`.
- REQ-QU-12: `QuestRepository` interface with `SaveQuestStatus`, `SaveObjectiveProgress`, `LoadQuests`.
- REQ-QU-13: `LoadQuests` returns all rows in one query joining both tables.
- REQ-QU-14: Login hydrates `ActiveQuests` and `CompletedQuests`; abandoned repeatable quests NOT loaded into `CompletedQuests`.
- REQ-QU-15: `talk <npc>` calls `GetOfferable` for NPCs with `NPCType == "quest_giver"`, sourcing `questIDs` from `npc.Template.QuestGiver.QuestIDs`.
- REQ-QU-16: `GetOfferable` evaluates eligibility: not active, prerequisites met (non-nil completedAt), repeatability check; abandoned prerequisite (nil pointer) does NOT satisfy prerequisite check.
- REQ-QU-17: Offerable quests shown with acceptance instruction; no quests shows `PlaceholderDialog`.
- REQ-QU-18: `talk <npc> accept <id>` re-checks eligibility, inserts DB rows, adds to `sess.ActiveQuests`.
- REQ-QU-19: NPC death handler calls `RecordKill`; progress persisted and `maybeComplete` called.
- REQ-QU-20: `get` handler calls `RecordFetch` (matching on `TargetID == itemDefID`); progress persisted and `maybeComplete` called.
- REQ-QU-21: `RecordExplore` called at both discovery sites (spawn-room path and movement path); sets objective progress to Quantity for matching room ID.
- REQ-QU-22: Each `Record*` method persists progress then calls `maybeComplete`.
- REQ-QU-23: `talk <npc>` evaluates all active deliver objectives for that NPC; iterates `backpack.Items()` to resolve `ItemID` to instances; removes exact required quantity; evaluates multiple objectives in order.
- REQ-QU-24: `maybeComplete` triggers `Complete` when all objectives satisfied; awards XP, items (`SaveInventory` once), credits to `sess.Currency`; updates session and DB.
- REQ-QU-25: Completion message: `"Quest complete: <Title>"` + XP/credits line + per-item lines.
- REQ-QU-26: `quest abandon <id>` updates DB, removes from session; locks non-repeatable quest with nil pointer in `CompletedQuests`.
- REQ-QU-27: Non-repeatable quest abandonment requires `confirm` suffix.
- REQ-QU-28: `quest` command supports `list`, `log <id>`, `abandon <id> [confirm]`, `completed`.
- REQ-QU-29: `quest list` shows active quests with objective ratio.
- REQ-QU-30: `quest log <id>` shows full detail for active or completed quests; `"(Completed: <timestamp>)"` header for completed; `"No quest found with that ID."` otherwise.
- REQ-QU-31: `quest completed` lists completed quests (non-nil `completedAt`) with timestamps.
- REQ-QU-32: Reward application order: XP, then items (overflow to floor, `SaveInventory` once), then credits to `sess.Currency`.
- REQ-QU-33: `SaveInventory` called once after all item rewards granted.

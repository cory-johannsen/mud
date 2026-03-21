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

- REQ-QU-1: `QuestDef` MUST have fields: `ID string` (YAML: `id`), `Title string` (YAML: `title`), `Description string` (YAML: `description`), `GiverNPCID string` (YAML: `giver_npc_id`), `Repeatable bool` (YAML: `repeatable`), `Cooldown string` (YAML: `cooldown`; Go duration; ignored when `Repeatable == false`), `Prerequisites []string` (YAML: `prerequisites`; quest IDs), `Objectives []QuestObjective` (YAML: `objectives`), `Rewards QuestRewards` (YAML: `rewards`).
- REQ-QU-2: `QuestObjective` MUST have exported fields with YAML tags: `ID string` (YAML: `id`), `Type string` (YAML: `type`; valid values: `"kill"` | `"fetch"` | `"explore"` | `"deliver"`), `Description string` (YAML: `description`), `TargetID string` (YAML: `target_id`), `ItemID string` (YAML: `item_id`; used only for `deliver` objectives), `Quantity int` (YAML: `quantity`; MUST be >= 1).
- REQ-QU-3: `QuestRewards` MUST have exported fields: `XP int` (YAML: `xp`), `Credits int` (YAML: `credits`), `Items []QuestRewardItem` (YAML: `items`). `QuestRewardItem` has fields `ItemID string` (YAML: `item_id`) and `Quantity int` (YAML: `quantity`).
- REQ-QU-4: `QuestDef.Validate()` MUST return an error if: `ID` or `Title` is empty, `GiverNPCID` is empty, `Objectives` is empty, any `QuestObjective` has empty `ID`/`Description`/`TargetID`, any `QuestObjective.Type` is not one of the four canonical values, any `QuestObjective.Quantity < 1`, a `deliver` objective has empty `ItemID`, or `Cooldown` is non-empty and fails `time.ParseDuration`.

### 1.2 Quest Registry

- REQ-QU-5: `QuestRegistry` MUST be a `map[string]*QuestDef` loaded from `content/quests/` at server startup and injected into `GameServiceServer`.
- REQ-QU-6: At startup, `QuestRegistry.Validate()` MUST cross-check all quest definitions against live registries. Unknown `GiverNPCID` values in `NPCRegistry`, unknown `target_id` values for kill objectives in `NPCRegistry`, unknown `target_id`/`item_id` values for fetch/deliver objectives in `ItemRegistry`, unknown `target_id` values for explore objectives in the world room set, and unknown `prerequisites` quest IDs in `QuestRegistry` MUST all cause a fatal startup error.

### 1.3 Runtime Types

- REQ-QU-7: `ActiveQuest` MUST have fields: `QuestID string`, `ObjectiveProgress map[string]int` (objective ID → count completed).
- REQ-QU-8: `QuestRecord` MUST have fields: `CharacterID int64`, `QuestID string`, `Status string` (`"active"` | `"completed"` | `"abandoned"`), `Progress map[string]int`, `CompletedAt *time.Time`.
- REQ-QU-9: `PlayerSession` MUST gain `ActiveQuests map[string]*quest.ActiveQuest` and `CompletedQuests map[string]*time.Time` (quest ID → completion time; nil pointer for abandoned quests).

---

## 2. Database Schema

- REQ-QU-10: A `character_quests` table MUST be created with columns `character_id BIGINT NOT NULL REFERENCES characters(id)`, `quest_id TEXT NOT NULL`, `status TEXT NOT NULL`, `completed_at TIMESTAMPTZ`, and `PRIMARY KEY (character_id, quest_id)`.
- REQ-QU-11: A `character_quest_progress` table MUST be created with columns `character_id BIGINT NOT NULL REFERENCES characters(id)`, `quest_id TEXT NOT NULL`, `objective_id TEXT NOT NULL`, `progress INT NOT NULL DEFAULT 0`, and `PRIMARY KEY (character_id, quest_id, objective_id)`.
- REQ-QU-12: `QuestRepository` MUST be an interface with methods: `SaveQuestStatus(ctx context.Context, characterID int64, questID, status string, completedAt *time.Time) error`, `SaveObjectiveProgress(ctx context.Context, characterID int64, questID, objectiveID string, progress int) error`, `LoadQuests(ctx context.Context, characterID int64) ([]QuestRecord, error)`.
- REQ-QU-13: `LoadQuests` MUST return all rows for the character across both tables (active, completed, abandoned) in a single query joining `character_quests` and `character_quest_progress`.

---

## 3. Quest Lifecycle

### 3.1 Session Loading

- REQ-QU-14: At login, `LoadQuests` MUST be called and results hydrated into the session: `active` rows populate `sess.ActiveQuests` with objective progress restored; `completed` rows populate `sess.CompletedQuests` with `completedAt` pointer; `abandoned` rows populate `sess.CompletedQuests` with a nil `completedAt` pointer (to enforce one-time lock while allowing repeatable quests to be re-offered).

### 3.2 Offer (`talk <npc>`)

- REQ-QU-15: The `talk <npc>` command handler MUST call `quest.Service.GetOfferable(sess, questIDs)` for NPC templates with `Type == "quest_giver"`, returning quests that: are not currently active, have all prerequisites completed, and pass the repeatability check (REQ-QU-16).
- REQ-QU-16: `GetOfferable` MUST evaluate eligibility in this order: (1) quest not in `sess.ActiveQuests`; (2) all prerequisite quest IDs present in `sess.CompletedQuests` with non-nil `completedAt`; (3) if `Repeatable == false`, quest not in `sess.CompletedQuests`; (4) if `Repeatable == true`, either not in `sess.CompletedQuests` or `time.Since(*completedAt) >= parsedCooldown`.
- REQ-QU-17: When offerable quests exist, the `talk` handler MUST display each quest's title, description, and reward summary, followed by the acceptance instruction `"Type 'talk <npc_name> accept <quest_id>' to accept."`. When no quests are offerable, the NPC's `PlaceholderDialog` MUST be shown.

### 3.3 Accept (`talk <npc> accept <quest_id>`)

- REQ-QU-18: `quest.Service.Accept(ctx, sess, characterID, questID)` MUST: validate the quest exists and is offerable to this player; insert a `character_quests` row with `status="active"`; insert one `character_quest_progress` row per objective with `progress=0`; add an `ActiveQuest` entry to `sess.ActiveQuests`; return the quest title and all objective descriptions for display.

### 3.4 Progress Hooks

- REQ-QU-19: The NPC death handler MUST call `quest.Service.RecordKill(ctx, sess, characterID, npcTemplateID)` after every NPC death, incrementing `ObjectiveProgress` for all active `kill` objectives whose `TargetID` matches `npcTemplateID`, up to the objective's `Quantity`.
- REQ-QU-20: The `get` command handler MUST call `quest.Service.RecordFetch(ctx, sess, characterID, itemDefID, qty)` after each successful item pickup, incrementing `ObjectiveProgress` for matching active `fetch` objectives, clamped to `Quantity`.
- REQ-QU-21: The room discovery path MUST call `quest.Service.RecordExplore(ctx, sess, characterID, roomID)` when a new room is discovered, setting `ObjectiveProgress` to `Quantity` (always 1) for matching active `explore` objectives.
- REQ-QU-22: Each `Record*` method MUST persist the updated progress via `SaveObjectiveProgress`, then call `maybeComplete`.

### 3.5 Deliver Objectives

- REQ-QU-23: When `talk <npc>` is called and the player has an active `deliver` objective with `TargetID` matching that NPC, the handler MUST check whether the required `ItemID` is present in inventory at the required `Quantity`. If present, `quest.Service.RecordDeliver` MUST remove the item via `backpack.Remove` and mark the objective complete. If not present, the handler MUST display `"You don't have <item name>."` and take no other action.

### 3.6 Completion (automatic)

- REQ-QU-24: `maybeComplete` MUST check after every progress update whether all objectives in the quest have `ObjectiveProgress >= Quantity`. If satisfied, `quest.Service.Complete(ctx, sess, characterID, questID)` MUST: update `character_quests` to `status="completed"` and set `completed_at=now`; award XP via `xp.Service.AwardXPAmount`; grant item rewards via `backpack.Add` + `SaveInventory` (overflow items dropped to room floor); add credits to `sess.Rounds` and persist; move the quest from `sess.ActiveQuests` to `sess.CompletedQuests`; send completion message and reward summary to the player.
- REQ-QU-25: The completion message MUST follow the format: `"Quest complete: <Title>"` followed by `"+<XP> XP  |  +<credits> credits  |  +<qty>x <item name>"` for each reward line (credits and item lines omitted if zero/empty).

### 3.7 Abandon (`quest abandon <id>`)

- REQ-QU-26: `quest abandon <id>` MUST update `character_quests` to `status="abandoned"` and remove the quest from `sess.ActiveQuests`. If `Repeatable == false`, it MUST add the quest to `sess.CompletedQuests` with nil `completedAt` to prevent re-acceptance.
- REQ-QU-27: If the quest being abandoned has `Repeatable == false`, the handler MUST require confirmation: display `"This quest cannot be retaken. Type 'quest abandon <id> confirm' to proceed."` and take no action until the `confirm` suffix is supplied.

---

## 4. `quest` Command Namespace

- REQ-QU-28: The `quest` command MUST support subcommands: `list`, `log <id>`, `abandon <id>` (with optional `confirm` suffix per REQ-QU-27), and `completed`.
- REQ-QU-29: `quest list` MUST display all active quests with per-quest objective completion ratio (e.g. `[2/3] recover_the_shipment — Recover the Shipment`).
- REQ-QU-30: `quest log <id>` MUST display the quest title, description, each objective with completion status (`[x]` or `[ ]`) and current/target count, and the full reward list. MUST return `"No active quest with that ID."` if the quest is not active; completed quest details require `quest completed`.
- REQ-QU-31: `quest completed` MUST list all quests in `sess.CompletedQuests` with a non-nil `completedAt`, showing title and completion timestamp.

---

## 5. Rewards

- REQ-QU-32: Reward application in `quest.Service.Complete` MUST occur in this order: (1) XP via `xp.Service.AwardXPAmount`; (2) items via `backpack.Add` for each reward item, with overflow dropped to room floor; (3) credits added to `sess.Rounds` and persisted.
- REQ-QU-33: `SaveInventory` MUST be called once after all item rewards are granted, not once per item.

---

## 6. Requirements Summary

- REQ-QU-1: `QuestDef` MUST have all specified fields with YAML tags.
- REQ-QU-2: `QuestObjective` MUST have exported fields with YAML tags; `Type` must be one of four canonical values; `Quantity >= 1`.
- REQ-QU-3: `QuestRewards` and `QuestRewardItem` MUST have exported fields with YAML tags.
- REQ-QU-4: `QuestDef.Validate()` MUST reject empty required fields, invalid objective types, invalid cooldown, and deliver objectives without `ItemID`.
- REQ-QU-5: `QuestRegistry` loaded from `content/quests/` at startup, injected into `GameServiceServer`.
- REQ-QU-6: Startup cross-validation of all quest definition references against live registries; unknown references are fatal errors.
- REQ-QU-7: `ActiveQuest` has `QuestID` and `ObjectiveProgress map[string]int`.
- REQ-QU-8: `QuestRecord` has `CharacterID`, `QuestID`, `Status`, `Progress`, `CompletedAt`.
- REQ-QU-9: `PlayerSession` gains `ActiveQuests map[string]*quest.ActiveQuest` and `CompletedQuests map[string]*time.Time`.
- REQ-QU-10: `character_quests` table with PK `(character_id, quest_id)`.
- REQ-QU-11: `character_quest_progress` table with PK `(character_id, quest_id, objective_id)`.
- REQ-QU-12: `QuestRepository` interface with `SaveQuestStatus`, `SaveObjectiveProgress`, `LoadQuests`.
- REQ-QU-13: `LoadQuests` returns all rows in one query joining both tables.
- REQ-QU-14: Login hydrates `ActiveQuests` and `CompletedQuests` from `LoadQuests` result.
- REQ-QU-15: `talk <npc>` calls `GetOfferable` for quest giver NPCs.
- REQ-QU-16: `GetOfferable` evaluates eligibility: not active, prerequisites met, repeatability check.
- REQ-QU-17: Offerable quests shown with acceptance instruction; no quests shows `PlaceholderDialog`.
- REQ-QU-18: `talk <npc> accept <id>` inserts DB rows and adds to `sess.ActiveQuests`.
- REQ-QU-19: NPC death handler calls `RecordKill`; progress persisted and `maybeComplete` called.
- REQ-QU-20: `get` handler calls `RecordFetch`; progress persisted and `maybeComplete` called.
- REQ-QU-21: Room discovery calls `RecordExplore`; progress persisted and `maybeComplete` called.
- REQ-QU-22: Each `Record*` method persists progress then calls `maybeComplete`.
- REQ-QU-23: `talk <npc>` checks for active deliver objectives; removes item and records progress if present; shows error if not.
- REQ-QU-24: `maybeComplete` triggers `Complete` when all objectives satisfied; awards all rewards; updates session and DB.
- REQ-QU-25: Completion message format: `"Quest complete: <Title>"` + reward line.
- REQ-QU-26: `quest abandon <id>` updates DB, removes from session, locks one-time quests.
- REQ-QU-27: One-time quest abandonment requires `confirm` suffix.
- REQ-QU-28: `quest` command supports `list`, `log <id>`, `abandon <id> [confirm]`, `completed`.
- REQ-QU-29: `quest list` shows active quests with objective ratio.
- REQ-QU-30: `quest log <id>` shows full quest detail; returns error if not active.
- REQ-QU-31: `quest completed` lists completed quests with timestamps.
- REQ-QU-32: Reward application order: XP, then items (overflow to floor), then credits.
- REQ-QU-33: `SaveInventory` called once after all item rewards granted.

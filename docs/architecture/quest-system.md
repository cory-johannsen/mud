# Quest System Architecture

## Overview

The quest system supports full quest lifecycle management: offering quests from NPCs,
accepting quests, tracking objective progress (kill, fetch, explore, deliver), automatic
completion with XP/credits/item rewards, and abandonment.

## Package: `internal/game/quest`

### Core Types

- `QuestDef` — static quest definition: title, description, objectives, prerequisites, rewards, repeatability, cooldown.
- `QuestObjective` — individual objective: ID, type (`kill`/`fetch`/`explore`/`deliver`), targetID, quantity, description.
- `QuestRewards` — XP, credits, and item rewards awarded on completion.
- `ActiveQuest` — mutable runtime state: questID and per-objective progress map.
- `QuestRecord` — persistence record loaded from the database (status, completedAt, progress).
- `QuestRegistry` — `map[string]*QuestDef`; canonical registry loaded from YAML content files.

### Service

`quest.Service` orchestrates the full quest lifecycle.

| Method | Description |
|---|---|
| `GetOfferable(sess, questIDs)` | Returns defs the player may accept now (eligibility, prerequisites, cooldown). |
| `Accept(ctx, sess, characterID, questID)` | Accepts a quest, initialises progress, persists `active` status. |
| `RecordKill(ctx, sess, characterID, npcTemplateID)` | Increments kill objectives; returns completion messages if any quest completed. |
| `RecordFetch(ctx, sess, characterID, itemDefID, qty)` | Increments fetch objectives by qty; returns completion messages if any quest completed. |
| `RecordExplore(ctx, sess, characterID, roomID)` | Increments explore objectives; returns completion messages if any quest completed. |
| `RecordDeliver(ctx, sess, characterID, questID, objectiveID)` | Marks a deliver objective complete; returns completion messages if the quest completed. |
| `Complete(ctx, sess, characterID, questID)` | Awards rewards, moves quest to completed, persists; returns player-facing messages. |
| `Abandon(ctx, sess, characterID, questID, confirm)` | Abandons a quest (with confirmation guard for non-repeatables). |
| `HydrateSession(sess, records)` | Populates `ActiveQuests` and `CompletedQuests` from loaded `QuestRecord` slice. |
| `LoadQuests(ctx, characterID)` | Delegates to `QuestRepository.LoadQuests`. |

### Interfaces

- `QuestRepository` — persistence: `SaveQuestStatus`, `SaveObjectiveProgress`, `LoadQuests`.
- `XPAwarder` — awards XP on completion; implemented by `xpServiceQuestAdapter` in `gameserver`.
- `InventorySaver` — saves inventory and currency after item/credits rewards.
- `SessionState` — minimal interface over `session.PlayerSession` for testability.

### Completion Messages

`CompletionMessage(def, invRegistry)` produces player-facing lines listing the quest title and all
rewards. These are returned up through `Complete → maybeComplete → recordProgressN → RecordKill/RecordFetch/RecordExplore/RecordDeliver`.

## Integration Points

```
GameServiceServer
  ├── questSvc (*quest.Service)
  │     ├── wired in NewGameServiceServer via quest.NewService(content.QuestRegistry, storage.QuestRepo, nil, invRegistry, charSaver)
  │     └── xpSvc wired post-construction via questSvc.SetXPAwarder(&xpServiceQuestAdapter{svc})
  │
  ├── grpc_service.go
  │     ├── RecordExplore — called on spawn room and room entry (new discoveries only); messages discarded
  │     └── RecordFetch   — called after every successful item pickup; completion messages pushed via pushMessageToUID
  │
  ├── grpc_service_quests.go
  │     ├── handleQuestCommand — dispatches quest list / log / abandon / completed
  │     ├── questList(sess)    — formats active quests with objective ratios
  │     ├── questLog(sess, id) — shows full detail for active or completed quest
  │     └── questCompleted(sess) — lists completed quests with timestamps
  │
  ├── grpc_service_quest_giver.go
  │     ├── findQuestGiverInRoom — locates quest_giver NPC by name in room
  │     ├── handleTalk          — shows dialog, offerable quests, and processes deliver objectives
  │     └── RecordDeliver       — called inside handleTalk after item consumption; completion messages returned and appended to response
  │
  └── combat_handler.go
        ├── SetQuestService(svc) — wires questSvc into CombatHandler
        ├── removeDeadNPCsLocked — calls RecordKill for all living participants after each NPC death
        └── pushQuestMessages(sess, msgs) — pushes each completion message as a MessageEvent to sess.Entity
```

## Database Tables

| Table | Purpose |
|---|---|
| `character_quests` | One row per character+quest: questID, status (`active`/`completed`/`abandoned`), completedAt. |
| `character_quest_progress` | One row per character+quest+objective: progress count. |

Migrations: `038_character_quests.up.sql`, `039_character_quest_progress.up.sql`.

## Content Loading

Quest definitions are loaded from YAML files under the zone content directories.
`content.QuestRegistry` is populated by the content pipeline and passed to `quest.NewService`
at server startup.

## Session State

`session.PlayerSession` carries:
- `ActiveQuests map[string]*quest.ActiveQuest` — in-flight quests with live objective progress.
- `CompletedQuests map[string]*time.Time` — completed/abandoned quests (nil time = abandoned).

Both maps are hydrated from the database on login via `questSvc.HydrateSession`.

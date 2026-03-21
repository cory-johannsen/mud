# Quests

A data-driven quest system giving players kill, fetch, explore, and deliver objectives with XP, item, and credit rewards. Supports quest chains via prerequisites, repeatable quests with cooldowns, and NPC quest givers.

Design spec: `docs/superpowers/specs/2026-03-21-quests-design.md`

## Requirements

- [ ] REQ-QU-1: `QuestDef` MUST have all specified fields with YAML tags.
- [ ] REQ-QU-2: `QuestObjective` MUST have exported fields with YAML tags; `TargetID` semantics vary by `Type` (NPC ID for kill/deliver, item ID for fetch, room ID for explore); `ItemID` used only for deliver; `Quantity >= 1`.
- [ ] REQ-QU-3: `QuestRewards` and `QuestRewardItem` MUST have exported fields with YAML tags.
- [ ] REQ-QU-4: `QuestDef.Validate()` MUST reject empty required fields, invalid objective types, deliver objectives without `ItemID`, invalid cooldown, and non-empty `Cooldown` on non-repeatable quests.
- [ ] REQ-QU-5: `QuestRegistry` loaded from `content/quests/` at startup, injected into `GameServiceServer`.
- [ ] REQ-QU-6: Startup cross-validation of all quest definition references against live registries (NPC, item, world, quest); unknown references are fatal errors.
- [ ] REQ-QU-7: `ActiveQuest` has `QuestID` and `ObjectiveProgress map[string]int`.
- [ ] REQ-QU-8: `QuestRecord` has `CharacterID`, `QuestID`, `Status`, `Progress map[string]int`, `CompletedAt *time.Time`.
- [ ] REQ-QU-9: `PlayerSession` gains `ActiveQuests map[string]*quest.ActiveQuest` and `CompletedQuests map[string]*time.Time` (nil pointer = abandoned one-time quest sentinel).
- [ ] REQ-QU-10: `character_quests` table with PK `(character_id, quest_id)`.
- [ ] REQ-QU-11: `character_quest_progress` table with PK `(character_id, quest_id, objective_id)`.
- [ ] REQ-QU-12: `QuestRepository` interface with `SaveQuestStatus`, `SaveObjectiveProgress`, `LoadQuests`.
- [ ] REQ-QU-13: `LoadQuests` returns all rows in one query joining both tables.
- [ ] REQ-QU-14: Login hydrates `ActiveQuests` and `CompletedQuests`; abandoned repeatable quests NOT loaded into `CompletedQuests`.
- [ ] REQ-QU-15: `talk <npc>` calls `GetOfferable` for NPCs with `NPCType == "quest_giver"`, sourcing `questIDs` from `npc.Template.QuestGiver.QuestIDs`.
- [ ] REQ-QU-16: `GetOfferable` evaluates eligibility: not active, prerequisites met (non-nil completedAt), repeatability check; abandoned prerequisite (nil pointer) does NOT satisfy prerequisite check.
- [ ] REQ-QU-17: Offerable quests shown with acceptance instruction; no quests shows `PlaceholderDialog`.
- [ ] REQ-QU-18: `talk <npc> accept <id>` re-checks eligibility, inserts DB rows, adds to `sess.ActiveQuests`.
- [ ] REQ-QU-19: NPC death handler calls `RecordKill`; progress persisted and `maybeComplete` called.
- [ ] REQ-QU-20: `get` handler calls `RecordFetch` (matching on `TargetID == itemDefID`); progress persisted and `maybeComplete` called.
- [ ] REQ-QU-21: `RecordExplore` called at both discovery sites (spawn-room path and movement path); sets objective progress to Quantity for matching room ID.
- [ ] REQ-QU-22: Each `Record*` method persists progress then calls `maybeComplete`.
- [ ] REQ-QU-23: `talk <npc>` evaluates all active deliver objectives for that NPC; iterates `backpack.Items()` to resolve `ItemID` to instances; removes exact required quantity; evaluates multiple objectives in order.
- [ ] REQ-QU-24: `maybeComplete` triggers `Complete` when all objectives satisfied; awards XP, items (`SaveInventory` once), credits to `sess.Currency`; updates session and DB.
- [ ] REQ-QU-25: Completion message: `"Quest complete: <Title>"` + XP/credits line + per-item lines.
- [ ] REQ-QU-26: `quest abandon <id>` updates DB, removes from session; locks non-repeatable quest with nil pointer in `CompletedQuests`.
- [ ] REQ-QU-27: Non-repeatable quest abandonment requires `confirm` suffix.
- [ ] REQ-QU-28: `quest` command supports `list`, `log <id>`, `abandon <id> [confirm]`, `completed`.
- [ ] REQ-QU-29: `quest list` shows active quests with objective ratio.
- [ ] REQ-QU-30: `quest log <id>` shows full detail for active or completed quests; `"(Completed: <timestamp>)"` header for completed; `"No quest found with that ID."` otherwise.
- [ ] REQ-QU-31: `quest completed` lists completed quests (non-nil `completedAt`) with timestamps.
- [ ] REQ-QU-32: Reward application order: XP, then items (overflow to floor, `SaveInventory` once), then credits to `sess.Currency`.
- [ ] REQ-QU-33: `SaveInventory` called once after all item rewards granted.

---
issue: 121
title: New character onboarding quest — find and use the zone map
slug: onboarding-find-zone-map
date: 2026-04-19
---

## Summary

Newly created characters automatically receive a starter quest that guides them
to find the zone map in Felony Flats and use it, teaching the map mechanic as
part of onboarding.

---

## Requirements

### REQ-OBQ-1: Quest Definition
A new quest YAML `content/quests/onboarding_find_zone_map.yaml` MUST be
created with the following properties:

```yaml
id: onboarding_find_zone_map
title: "Find Your Bearings"
description: >
  You've been dropped into Felony Flats with nothing but instinct and the
  clothes on your back. Every survivor in this district knows one thing:
  you need a map. There's a district terminal on 82nd Avenue that has a
  zone map. Find it and figure out how to use it.
type: onboarding
auto_complete: true
repeatable: false
objectives:
  - id: explore_map_room
    type: explore
    description: Locate the zone map terminal on 82nd Avenue
    target_id: flats_82nd_ave
    quantity: 1
  - id: use_zone_map
    type: use_zone_map
    description: Use the zone map terminal
    target_id: felony_flats
    quantity: 1
rewards:
  xp: 50
  credits: 0
```

### REQ-OBQ-2: New Objective Type
The quest system MUST support a new objective type `use_zone_map`.

- REQ-OBQ-2a: `use_zone_map` MUST be added to `validObjectiveTypes` in
  `internal/game/quest/def.go`.
- REQ-OBQ-2b: `use_zone_map` objectives MUST use `target_id` as the zone ID.
- REQ-OBQ-2c: `use_zone_map` MUST NOT require `item_id` (delivery is not involved).

### REQ-OBQ-3: New Objective Type Validation
The `QuestDef.Validate()` method MUST allow `use_zone_map` objectives with
`target_id` set to a zone ID and `quantity >= 1`. No `item_id` is required.

### REQ-OBQ-4: Quest Progress Recording — Zone Map Use
A new method `RecordZoneMapUse(ctx, sess, characterID, zoneID)` MUST be added
to `internal/game/quest/service.go`.

- REQ-OBQ-4a: It MUST increment progress for all active `use_zone_map`
  objectives whose `target_id` matches `zoneID`.
- REQ-OBQ-4b: It MUST call `maybeComplete` for any quest with updated
  progress.
- REQ-OBQ-4c: It MUST return completion messages and any error, consistent
  with other `Record*` methods.

### REQ-OBQ-5: Wire Zone Map Use into reveal_zone Callback
The `wireRevealZone` callback in `internal/gameserver/grpc_service.go` MUST
call `questSvc.RecordZoneMapUse(ctx, sess, sess.CharacterID, zoneID)` after
the automap reveal completes.

- REQ-OBQ-5a: Any completion messages returned MUST be sent to the client as
  console messages.
- REQ-OBQ-5b: Errors MUST be logged at Warn level but MUST NOT interrupt the
  reveal_zone flow.

### REQ-OBQ-6: New Quest Type Validation
The `QuestDef.Validate()` method MUST allow quests with `type: onboarding` to
have no `giver_npc_id`, matching the exemption already granted to
`find_trainer`.

- REQ-OBQ-6a: `onboarding` quests MUST still validate objectives normally
  (unlike `find_trainer` which has no objectives).

### REQ-OBQ-7: Auto-Issue on Character Creation
Immediately after `grantStartingInventory` succeeds for a new character,
`questSvc.Accept` MUST be called for `onboarding_find_zone_map`.

- REQ-OBQ-7a: If the quest is already active or completed (e.g., returning
  character data migration edge case), the issue MUST be silently skipped.
- REQ-OBQ-7b: Errors from `Accept` MUST be logged at Warn level and MUST NOT
  abort session start.
- REQ-OBQ-7c: A console message MUST be sent to the client immediately after
  auto-issue: `New quest: Find Your Bearings — locate the district map
  terminal on 82nd Avenue.`

### REQ-OBQ-8: Quest Registry Cross-Validation
The quest registry cross-validation in `internal/game/quest/registry.go` MUST
skip the giver NPC check for `onboarding` type quests, matching the existing
`find_trainer` exemption.

### REQ-OBQ-9: Test Coverage
- REQ-OBQ-9a: A unit test MUST verify `RecordZoneMapUse` increments progress
  and triggers completion for `use_zone_map` objectives.
- REQ-OBQ-9b: A unit test MUST verify that `onboarding` quests pass
  `Validate()` with no `giver_npc_id`.
- REQ-OBQ-9c: A unit test MUST verify that `onboarding` quests pass registry
  `CrossValidate` without a matching NPC ID.
- REQ-OBQ-9d: A property-based test MUST verify that auto-issuing
  `onboarding_find_zone_map` on a new session never errors when `questSvc` is
  non-nil.

---

## Objective Flow

```
New character created
  └── grantStartingInventory succeeds
        └── questSvc.Accept("onboarding_find_zone_map")
              └── Console: "New quest: Find Your Bearings"

Player walks to flats_82nd_ave
  └── RecordExplore("flats_82nd_ave")
        └── explore_map_room objective → complete

Player uses zone map terminal
  └── reveal_zone("felony_flats") [Lua]
        └── RecordZoneMapUse("felony_flats")
              └── use_zone_map objective → complete
                    └── maybeComplete → Quest complete
                          └── Console: "Quest complete: Find Your Bearings"
                          └── XP: 50
```

---

## Flavor Text (Post-Collapse Portland Setting)

Quest description references the 82nd Avenue district terminal — a salvaged
pre-collapse navigation kiosk now guarded by whatever faction holds the block.
The voice is terse and functional: survival, not tourism.

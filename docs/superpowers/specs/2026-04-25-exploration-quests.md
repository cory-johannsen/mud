---
title: Exploration Quests
issue: https://github.com/cory-johannsen/mud/issues/256
date: 2026-04-25
status: spec
prefix: EXQ
depends_on: []
related:
  - "Zone quests (RRQ/VTQ) spec 2026-04-13 — quest authoring conventions"
  - "Zone difficulty scaling spec 2026-04-19 — reward-by-tier mapping"
  - "#255 Exploration challenges (sibling exploration content)"
---

# Exploration Quests

## 1. Summary

The objective primitives for exploration quests are already shipped:

- The `QuestObjective.Type` enum at `internal/game/quest/def.go:8` already includes `explore` (alongside `kill`, `fetch`, `deliver`, `use_zone_map`).
- `RecordExplore` is already called on room entry at `internal/gameserver/grpc_service.go:2918`, feeding `quest.Service.RecordExplore` which increments any matching `explore` objective.
- Per-character "discovered rooms" persistence is already in place via the `character_map_rooms` table (`internal/storage/postgres/automap.go:28-40`).
- The telnet quest log already enumerates objectives with progress (`grpc_service_quests.go:16-100`).
- Reward grants (XP, Credits, Items) work via `QuestRewards` (`def.go:17-20`).

So an exploration quest with a single-room objective works today end-to-end — it just isn't authored anywhere. What is missing for the issue to deliver real player-facing value:

1. **Area-level objectives.** Today an `explore` objective targets exactly one `room_id`. The issue asks for "rooms, areas, or zone landmarks." There is no concept of "discover any room in this set" or "visit all rooms in this zone tier."
2. **Authored exemplar quests.** No `explore`-type quest exists in `content/quests/` today. Without exemplars the feature is invisible.
3. **Reward scaling by zone tier.** Rewards are flat numbers in YAML. The issue asks for "rewards appropriate to zone level"; we need a server-side computation that scales declared baselines by the tier of the discovered area.
4. **Discovery narrative.** Today a room enter is silent for quests. The issue implies the player should *know* they completed a discovery objective on the spot.
5. **Web quest log.** Telnet has `quest log`; the web client's quest panel needs the same per-objective progress display for `explore`-type objectives so players can track progress in the GUI surface most use.
6. **Map/landmark surfacing.** Discovered landmarks should appear on the zone map (web) and be referable from telnet (`landmarks` command). Today the `character_map_rooms` table records visit but there is no `landmark` flag distinguishing significant places from corridors.

This spec lights up these gaps as a thin extension on top of the existing `explore` machinery.

## 2. Goals & Non-Goals

### 2.1 Goals

- EXQ-G1: An `explore` objective MAY target a single `room_id`, a `room_set` (list of room ids — "all of these"), an `any_of` set ("at least one of these"), or a `landmark_id`.
- EXQ-G2: Reward baselines authored in YAML scale by the discovered area's zone tier at completion time, with an explicit author opt-out.
- EXQ-G3: Discovery moments emit narrative on both telnet and web ("You have discovered the abandoned subway entrance.") without spamming corridors.
- EXQ-G4: The web client's quest panel renders `explore` objective progress identically to telnet's `quest log`.
- EXQ-G5: At least three exemplar exploration quests author and validate at startup.
- EXQ-G6: Existing `explore` objectives that target a single room continue to work unchanged.

### 2.2 Non-Goals

- EXQ-NG1: Replacing the existing `explore` objective type; this spec is additive.
- EXQ-NG2: Cross-zone exploration objectives (a single quest spanning multiple zones). v1 is single-zone.
- EXQ-NG3: Time-limited / "race to discover" mechanics.
- EXQ-NG4: Group exploration credit (party shares discovery). v1 is per-character.
- EXQ-NG5: Procedural / generated discovery quests. v1 is hand-authored.
- EXQ-NG6: Reward type expansion (perks, feats, abilities). XP/Credits/Items remain the only reward kinds.

## 3. Glossary

- **Exploration quest**: a quest whose objectives are all of type `explore`.
- **Landmark**: a room flagged as `is_landmark: true` in the room YAML; eligible to be referenced by `landmark_id` in an objective and surfaced in the landmark UI.
- **Room set**: a list of room ids the player must discover *all of* to complete the objective.
- **Any-of set**: a list of room ids; discovering any one completes the objective.
- **Zone tier**: the existing zone difficulty tier (`Desperate Streets`, `Armed & Dangerous`, etc.).
- **Reward baseline**: the YAML-declared `xp` / `credits` / item quantities; scaled by tier at grant time unless `scale: false` is set.

## 4. Requirements

### 4.1 Objective Shape Extensions

- EXQ-1: `QuestObjective` MUST gain optional fields `room_set []string`, `any_of []string`, `landmark_id string`. Exactly one of `target_id` (existing single-room target), `room_set`, `any_of`, or `landmark_id` MUST be set on an `explore`-type objective; the loader MUST reject objectives that set none or more than one.
- EXQ-2: `Quantity` (existing `QuestObjective.Quantity`) MUST default to the length of `room_set` when `room_set` is non-empty. For `any_of`, `Quantity` MUST be exactly 1. For `landmark_id`, `Quantity` MUST be exactly 1.
- EXQ-3: `RecordExplore(charID, roomID)` MUST be extended so that:
  - For `target_id == roomID`: increment progress by 1 (existing behavior, unchanged).
  - For `room_set` containing `roomID`: increment progress by 1, but never beyond `len(room_set)` and never twice for the same room id (idempotent per-room contribution).
  - For `any_of` containing `roomID`: set progress to 1 immediately (and complete).
  - For `landmark_id` matching the room's landmark id: set progress to 1 immediately.
- EXQ-4: Per-objective per-character progress MUST persist a set of visited room ids when the objective uses `room_set`, so a re-entry of an already-credited room does not double-count and a server restart does not reset partial progress. New table: `character_quest_explore_progress(character_id, quest_id, objective_id, room_id)` with composite PK.

### 4.2 Landmarks

- EXQ-5: `world.Room` MUST gain optional fields `is_landmark bool` (default `false`) and `landmark_id string` (default empty; required when `is_landmark` is true).
- EXQ-6: The loader MUST validate that no two rooms in the world share a `landmark_id` and that every quest `landmark_id` reference resolves to an existing landmark room.
- EXQ-7: `character_map_rooms` MUST gain a derived `is_landmark` column populated at room-discovery time so the per-character map can render landmark icons without a join. Migration adds the column with default `false`.
- EXQ-8: A new helper `world.Landmarks(zoneID) []*Room` MUST return all landmark rooms in a zone, ordered by `landmark_id`.

### 4.3 Reward Scaling

- EXQ-9: `QuestRewards` MUST gain optional `scale bool` (default `true`). When `scale: true`, the resolver scales XP and Credits by the zone tier of the room/landmark/set on completion. Item quantities are NOT scaled.
- EXQ-10: A new helper `quest.ScaleReward(baseline int, tier Tier) int` MUST live in the zone-difficulty package and apply the multipliers: `Desperate Streets ×1.0`, `Armed & Dangerous ×2.0`, `Warlord Territory ×4.0`, `Apex Predator ×8.0`, `End Times ×16.0`.
- EXQ-11: For multi-room objectives spanning multiple tiers (`room_set` rooms in different zones), the highest tier among them MUST be the scaling tier.
- EXQ-12: When `scale: false`, the YAML baseline is granted as-is. This preserves existing zone-quest rewards from the 2026-04-13 RRQ/VTQ spec without re-balancing.

### 4.4 Discovery Narrative

- EXQ-13: When a `RecordExplore` call materially advances any quest objective (progress changes, completion fires, landmark first-discovery), a narrative line MUST be sent to the discovering player. Format: `Discovery: <landmark_name>` for landmarks; `Quest progress: <quest title> — <objective summary> (<progress>/<total>)` for room-set/any-of advancement.
- EXQ-14: When a room entry advances no quest objective and the room is not a first-time visit, no narrative MUST fire (no spam).
- EXQ-15: Landmark first-discovery MUST always emit a narrative even when no quest references the landmark, so players learn the place name.

### 4.5 Quest Log UI

- EXQ-16: The telnet `quest log` command (`grpc_service_quests.go:80-100`) MUST render `explore` objectives with the new objective shapes:
  - Single-room: `<obj.text> — <visited?> ✓ / pending` (current behavior).
  - `room_set`: `<obj.text> — <visited_count>/<total> rooms discovered`.
  - `any_of`: `<obj.text> — discover any of: <room_a>, <room_b>, ... ✓ / pending`.
  - `landmark_id`: `<obj.text> — discover landmark "<landmark_name>" ✓ / pending`.
- EXQ-17: A new web client component `QuestLogPanel.tsx` (or extension to the existing one) MUST mirror EXQ-16's rendering with collapsible objective rows. Each `explore` objective row MUST link to the zone map filtered to its target rooms / landmark.
- EXQ-18: A new telnet command `landmarks` MUST list landmarks in the current zone with discovery state per the active character: `<landmark_id> — <name> [discovered: yes/no]`.

### 4.6 Exemplar Content

- EXQ-19: At least three exploration quests MUST be authored under `content/quests/`:
  - `exq_discover_pdx_underbelly` — `room_set` of three Portland-area rooms.
  - `exq_find_clown_camp` — `any_of` for several entry points to the clown camp faction territory.
  - `exq_landmark_radio_tower` — `landmark_id` targeting a single landmark.
- EXQ-20: The implementer MUST coordinate with the user before picking exact rooms / landmarks. The intent is exemplars that exercise each objective shape end-to-end.

### 4.7 Tests

- EXQ-21: Existing quest tests MUST pass unchanged.
- EXQ-22: New unit tests in `internal/game/quest/service_test.go` MUST cover:
  - `room_set` per-room idempotence (re-entry does not double-count).
  - `any_of` immediate completion on first qualifying room entry.
  - `landmark_id` matches a landmark room.
  - `scale: true` applies the tier multiplier; `scale: false` does not.
  - Multi-tier `room_set` uses the highest tier.
- EXQ-23: A new integration test MUST simulate a character completing each exemplar quest and verify the correct rewards (post-scaling) are credited.

### 4.8 Migration

- EXQ-24: A migration MUST be authored adding `is_landmark` and `landmark_id` to the relevant world-loader path. World data is YAML-only; no DB migration for the room model itself, only for `character_map_rooms.is_landmark` and the new `character_quest_explore_progress` table.

## 5. Architecture

### 5.1 Where the new code lives

```
content/quests/
  exq_*.yaml                                 # 3+ exemplars

content/zones/
  <zone>.yaml                                # add is_landmark / landmark_id to chosen rooms

internal/game/world/
  model.go                                   # Room.IsLandmark, Room.LandmarkID

internal/game/quest/
  def.go                                     # QuestObjective.RoomSet, AnyOf, LandmarkID,
                                             # QuestRewards.Scale
  service.go                                 # RecordExplore extended dispatch
  reward.go                                  # NEW: ScaleReward(baseline, tier) helper
  store_explore_progress.go                  # NEW: per-room set persistence

internal/storage/postgres/
  quest_explore_progress.go                  # NEW: repository

internal/gameserver/
  grpc_service.go                            # narrative emission on advancement
  grpc_service_quests.go                     # log rendering for new shapes
  grpc_service_landmarks.go                  # NEW: `landmarks` telnet command

cmd/webclient/ui/src/game/quests/
  QuestLogPanel.tsx                          # objective rendering parity with telnet

migrations/
  NNN_character_quest_explore_progress.up.sql / .down.sql
  NNN_character_map_rooms_is_landmark.up.sql  / .down.sql
```

### 5.2 Discovery flow

```
player enters room X
   │
   ▼
applyRoomSkillChecks (existing) — silent triggers
   │
   ▼
character_map_rooms upsert (existing)
   │
   ▼
quest.RecordExplore(charID, X)
   │
   ▼
for each active quest with explore objective referencing X:
    advance per shape (target_id / room_set / any_of / landmark_id)
    if changed:
       persist progress
       emit narrative (EXQ-13)
       if completed:
          quest.MaybeComplete → reward grant via ScaleReward (EXQ-9..12)
   │
   ▼
landmark first-discovery? → emit "Discovery: <name>" (EXQ-15)
```

### 5.3 Single sources of truth

- Objective shape: `internal/game/quest/def.go`.
- Discovery dispatch: `quest.Service.RecordExplore` only.
- Reward scaling: `quest.ScaleReward` only.
- Landmark data: `world.Room.LandmarkID` only.

## 6. Open Questions

- EXQ-Q1: Should `room_set` objectives expose partial progress in the narrative on every increment (current EXQ-13 design) or only on completion? Recommendation: every increment — the player can see how much remains.
- EXQ-Q2: Reward scaling multipliers (×1, ×2, ×4, ×8, ×16) are aggressive. Should we use a gentler ×1, ×1.5, ×2.25, ×3.375, ×5 progression? Recommendation: defer to user — depends on whether endgame XP grind is intended.
- EXQ-Q3: Should an `any_of` objective track *which* room satisfied it (for narrative), or just the boolean? Recommendation: track for narrative but not for serialized state — emit the discovered room name in completion narrative only.
- EXQ-Q4: Landmarks shown on the zone map — visible before discovery? Recommendation: visible as `???` greyed icon if landmark, named after first discovery. Aligns with the detection-state pattern (#254) of progressive reveal.
- EXQ-Q5: Should completing an exploration quest unlock fast-travel to its terminal landmark? Recommendation: defer; out of scope for this spec.

## 7. Acceptance

- [ ] Three exemplar exploration quests load, validate, and resolve end-to-end on a fresh character.
- [ ] A `room_set` objective increments by 1 per unique room and is idempotent on re-entry.
- [ ] An `any_of` objective completes on first qualifying room entry.
- [ ] A `landmark_id` objective completes on first entry to the landmark room.
- [ ] Rewards with `scale: true` produce tier-scaled XP and Credits at completion; `scale: false` produces baseline values.
- [ ] Telnet `quest log` renders the new objective shapes correctly; web `QuestLogPanel` mirrors.
- [ ] Telnet `landmarks` lists landmarks with per-character discovery state.
- [ ] Existing single-room `explore` quests (if any) continue to work unchanged.

## 8. Out-of-Scope Follow-Ons

- EXQ-F1: Cross-zone exploration objectives.
- EXQ-F2: Time-limited "race" exploration.
- EXQ-F3: Group / party exploration credit.
- EXQ-F4: Procedurally generated discovery quests.
- EXQ-F5: Fast-travel unlocks tied to discovery.
- EXQ-F6: Reward types beyond XP/Credits/Items (perks, feats, faction reputation).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/256
- Existing objective enum: `internal/game/quest/def.go:8`
- Existing QuestObjective shape: `internal/game/quest/def.go:23-30`
- Existing RecordExplore call site: `internal/gameserver/grpc_service.go:2918`
- Quest service progress: `internal/game/quest/service.go:220-250`
- Quest log render (telnet): `internal/gameserver/grpc_service_quests.go:16-100`
- Discovered rooms persistence: `internal/storage/postgres/automap.go:28-40`
- Reward shape: `internal/game/quest/def.go:17-20`
- Zone difficulty tiers: `docs/superpowers/specs/2026-04-19-zone-difficulty-scaling.md`
- Authored zone-quest exemplars: `docs/superpowers/specs/2026-04-13-zone-quests-rrq-vtq.md`
- Sibling exploration content: `docs/superpowers/specs/2026-04-25-exploration-challenges.md`

# Named NPCs Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `npcs-named` (priority 330)
**Dependencies:** `non-combat-npcs`

---

## Overview

Adds three named NPCs — Wayne Dawg, Jennifer Dawg, and Dwayne Dawg — to Rustbucket Ridge. All three are permanent friendly merchants. Wayne and Jennifer live together in the existing `wayne_dawgs_trailer` room (updated to `danger_level: safe`). Dwayne lives in a new adjacent room `dwayne_dawgs_trailer` (also `danger_level: safe`). Quest-giver wiring is deferred to the `quests` feature.

**Note on `npc_role`:** The `npc_role` field on `npc.Template` is added by the `non-combat-npcs` feature, which is a declared dependency of this feature. Implementation of this spec MUST NOT proceed until `non-combat-npcs` has added `NpcRole string \`yaml:"npc_role"\`` to `npc.Template`. Until that field exists, `npc_role: merchant` in the template YAML will be silently ignored by the loader.

**Note on taunts:** The `taunts`, `taunt_chance`, and `taunt_cooldown` fields are current struct fields used now. When the `npc-behaviors` feature is implemented, these will be migrated to HTN `say` operator domain entries. All three named NPC templates will require migration at that time.

---

## 1. Character Profiles

- REQ-NN-0: Implementation MUST NOT proceed until the `non-combat-npcs` feature has added `NpcRole string` with yaml tag `npc_role` to `npc.Template`.
- REQ-NN-1: All three NPC templates MUST set `disposition: friendly`.
- REQ-NN-2: All three NPC templates MUST set `npc_role: merchant`.
- REQ-NN-3: All three NPC templates MUST set `respawn_delay: 0s`.
- REQ-NN-4: All three NPC templates MUST NOT define weapon or armor fields.
- REQ-NN-5: All three NPC template files MUST include a YAML comment marking each as a future quest giver pending the `quests` feature.
- REQ-NN-6: All three NPC templates MUST set `type: human` and `loot` to credits only (no item drops).

| NPC | ID | Description |
|-----|----|-------------|
| Wayne Dawg | `wayne_dawg` | Grizzled old dog with a makeshift lab. Has seen better days but still runs the best deals on the Ridge. |
| Jennifer Dawg | `jennifer_dawg` | Wayne's wife. Sharper than Wayne by half, runs the front while he handles the back. |
| Dwayne Dawg | `dwayne_dawg` | Wayne's neighbor. Big heart, bigger opinions. Fancies himself an entrepreneur. |

---

## 2. Placement

### 2.1 Wayne Dawg's Trailer (existing room — updated)

Room `wayne_dawgs_trailer` in `content/zones/rustbucket_ridge.yaml` gains:
- `danger_level: safe`
- Updated description: `"Wayne and Jennifer Dawg have made this rusted trailer into something almost livable. There's a makeshift lab bolted to the side wall and a card table by the door where deals get done. It smells like solder and something fried."`
- A west exit to `dwayne_dawgs_trailer`
- Two spawn entries: `wayne_dawg` (count: 1) and `jennifer_dawg` (count: 1)

### 2.2 Dwayne Dawg's Trailer (new room)

New room appended to `content/zones/rustbucket_ridge.yaml`:

| Field | Value |
|-------|-------|
| `id` | `dwayne_dawgs_trailer` |
| `title` | Dwayne Dawg's Trailer |
| `description` | A battered single-wide pressed up against the fence line. Dwayne has strung lights along the eaves and put out a folding chair like he's expecting company. He usually is. |
| `danger_level` | `safe` |
| `map_x` | `-4` |
| `map_y` | `4` |
| Exits | east → `wayne_dawgs_trailer` |
| Spawns | `dwayne_dawg`, count: 1, respawn_delay: 0s |

### 2.3 Bidirectional Exit

`wayne_dawgs_trailer` gains a west exit to `dwayne_dawgs_trailer`. `dwayne_dawgs_trailer` has an east exit to `wayne_dawgs_trailer`.

- REQ-NN-7: `wayne_dawgs_trailer` MUST be updated to `danger_level: safe`.
- REQ-NN-8: `wayne_dawgs_trailer` description MUST be updated to the text specified in Section 2.1.
- REQ-NN-9: `wayne_dawgs_trailer` MUST gain spawn entries for both `wayne_dawg` and `jennifer_dawg`, each with `count: 1` and `respawn_delay: 0s`.
- REQ-NN-10: `wayne_dawgs_trailer` MUST gain a west exit to `dwayne_dawgs_trailer`.
- REQ-NN-11: A new room `dwayne_dawgs_trailer` MUST be added to `rustbucket_ridge.yaml` at `map_x: -4`, `map_y: 4` with `danger_level: safe`.
- REQ-NN-12: `dwayne_dawgs_trailer` MUST have an east exit to `wayne_dawgs_trailer` and a spawn entry for `dwayne_dawg` with `count: 1` and `respawn_delay: 0s`.
- REQ-NN-13: `map_x: -4`, `map_y: 4` MUST NOT overlap with any existing room in `rustbucket_ridge.yaml`.

---

## 3. NPC Template Files

Three files placed directly in `content/npcs/` (flat layout — the `LoadTemplates` function uses `os.ReadDir` which does not recurse into subdirectories).

- REQ-NN-14: NPC template files MUST be placed directly in `content/npcs/`, not a subdirectory.
- REQ-NN-15: Each template MUST have a unique `id` matching the filenames: `wayne_dawg`, `jennifer_dawg`, `dwayne_dawg`.
- REQ-NN-16: Each template MUST include lore-appropriate `taunts` reflecting the character's personality.

### 3.1 `content/npcs/wayne_dawg.yaml`

```yaml
# quest_giver: pending quests feature
id: wayne_dawg
name: "Wayne Dawg"
description: "A grizzled old man with oil-stained hands and a knowing squint. He's been on the Ridge longer than most and has the deals to prove it."
type: human
npc_role: merchant
disposition: friendly
level: 3
max_hp: 30
ac: 10
awareness: 8
respawn_delay: "0s"
abilities:
  brutality: 8
  quickness: 10
  grit: 12
  reasoning: 12
  savvy: 14
  flair: 10
taunt_chance: 0.2
taunt_cooldown: "45s"
taunts:
  - "You need somethin', or you just here to look?"
  - "Jennifer! We got company!"
  - "Best deals on the Ridge, and that ain't braggin'."
  - "Don't let the place fool ya. I got good stock."
loot:
  currency:
    min: 20
    max: 80
```

### 3.2 `content/npcs/jennifer_dawg.yaml`

```yaml
# quest_giver: pending quests feature
id: jennifer_dawg
name: "Jennifer Dawg"
description: "A sharp-eyed woman who keeps the trailer running while Wayne keeps it interesting. She sizes you up before Wayne's finished his sentence."
type: human
npc_role: merchant
disposition: friendly
level: 3
max_hp: 28
ac: 10
awareness: 10
respawn_delay: "0s"
abilities:
  brutality: 8
  quickness: 11
  grit: 11
  reasoning: 14
  savvy: 15
  flair: 11
taunt_chance: 0.2
taunt_cooldown: "45s"
taunts:
  - "Wayne, let me handle this one."
  - "You look like you could use something useful."
  - "I know what you need before you do. That's just how it is."
  - "Fair prices. No nonsense. That's the Dawg way."
loot:
  currency:
    min: 20
    max: 80
```

### 3.3 `content/npcs/dwayne_dawg.yaml`

```yaml
# quest_giver: pending quests feature
id: dwayne_dawg
name: "Dwayne Dawg"
description: "A big man in a small trailer with a lot of ideas. He talks like every deal is the deal of a lifetime. Sometimes he's right."
type: human
npc_role: merchant
disposition: friendly
level: 3
max_hp: 32
ac: 10
awareness: 7
respawn_delay: "0s"
abilities:
  brutality: 12
  quickness: 9
  grit: 13
  reasoning: 10
  savvy: 13
  flair: 13
taunt_chance: 0.2
taunt_cooldown: "45s"
taunts:
  - "Dwayne Dawg, entrepreneur. At your service."
  - "Wayne's good, but I'm better. Don't tell him I said that."
  - "I got things. You need things. This works out."
  - "Pull up a chair. This won't take long."
loot:
  currency:
    min: 20
    max: 80
```

---

## 4. Requirements Summary

- REQ-NN-0: Implementation MUST NOT proceed until the `non-combat-npcs` feature has added `NpcRole string` with yaml tag `npc_role` to `npc.Template`.
- REQ-NN-1: All three NPC templates MUST set `disposition: friendly`.
- REQ-NN-2: All three NPC templates MUST set `npc_role: merchant`.
- REQ-NN-3: All three NPC templates MUST set `respawn_delay: 0s`.
- REQ-NN-4: All three NPC templates MUST NOT define weapon or armor fields.
- REQ-NN-5: All three NPC template files MUST include a YAML comment marking each as a future quest giver pending the `quests` feature.
- REQ-NN-6: All three NPC templates MUST set `type: human` and `loot` to credits only (no item drops).
- REQ-NN-7: `wayne_dawgs_trailer` MUST be updated to `danger_level: safe`.
- REQ-NN-8: `wayne_dawgs_trailer` description MUST be updated to the text specified in Section 2.1.
- REQ-NN-9: `wayne_dawgs_trailer` MUST gain spawn entries for both `wayne_dawg` and `jennifer_dawg`, each with `count: 1` and `respawn_delay: 0s`.
- REQ-NN-10: `wayne_dawgs_trailer` MUST gain a west exit to `dwayne_dawgs_trailer`.
- REQ-NN-11: A new room `dwayne_dawgs_trailer` MUST be added to `rustbucket_ridge.yaml` at `map_x: -4`, `map_y: 4` with `danger_level: safe`.
- REQ-NN-12: `dwayne_dawgs_trailer` MUST have an east exit to `wayne_dawgs_trailer` and a spawn entry for `dwayne_dawg` with `count: 1` and `respawn_delay: 0s`.
- REQ-NN-13: `map_x: -4`, `map_y: 4` MUST NOT overlap with any existing room in `rustbucket_ridge.yaml`.
- REQ-NN-14: NPC template files MUST be placed directly in `content/npcs/`, not a subdirectory.
- REQ-NN-15: Each template MUST have a unique `id` matching the filenames: `wayne_dawg`, `jennifer_dawg`, `dwayne_dawg`.
- REQ-NN-16: Each template MUST include lore-appropriate `taunts` reflecting the character's personality.

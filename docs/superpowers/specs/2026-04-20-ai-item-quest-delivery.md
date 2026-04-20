---
issue: 107
title: AI Item Quest Delivery — Signal in the Static / Field Test quest chains
slug: ai-item-quest-delivery
date: 2026-04-20
---

## Summary

Deliver the AI Chainsaw and AI AK-47 to players through a two-quest chain that
begins after killing the home zone boss and ends with the rogue AI technician
NPC "Cipher" in The Velvet Rope. Both items also drop at a 5% rate from The
Velvet Rope's boss as an organic discovery path. This spec covers quest YAML
files, new NPC content, and zone placement only — no engine changes. It depends
on the AI Item Content spec (`2026-04-20-ai-item-content.md`) being implemented
first (the items must exist before quests can reward them).

---

## Architecture Overview

- Two parallel quest chains, one per team, that converge on the same NPC and
  boss objective but reward different items.
- Four new quest YAML files in `content/quests/`.
- One new NPC (`cipher`) added to the existing VIP Lounge room
  (`the_velvet_rope_brothel`) in The Velvet Rope zone.
- One new NPC template YAML in `content/npcs/`.
- Drop rate added to the existing `gangbang` NPC template in
  `content/zones/the_velvet_rope.yaml`.

---

## Requirements

### REQ-AQD-1: Cipher NPC

A new NPC template MUST be created at `content/npcs/cipher.yaml`:

```yaml
id: cipher
name: Cipher
description: >
  A slight figure hunched over a workbench covered in circuit boards and
  stripped weapon receivers. They don't look up when you enter. Whatever
  they're soldering, it's more interesting than you are. For now.
kind: quest_giver
faction: neutral
level: 50
aggressive: false
```

The NPC MUST be added as a spawn in the `the_velvet_rope_brothel` room of
`content/zones/the_velvet_rope.yaml`:

```yaml
- template: cipher
  count: 1
  respawn_after: 0s
```

### REQ-AQD-2: Machete "Signal in the Static" quest

A new file `content/quests/machete_signal_in_static.yaml` MUST be created:

```yaml
id: machete_signal_in_static
title: Signal in the Static
description: >
  Word's going around that someone in The Velvet Rope has been pulling
  neural chips out of salvage and wiring them into weapons. Nobody knows
  who. Nobody knows how to find them. That's the point. Prove you're
  serious and the signal gets clearer.
giver_npc_id: rustbucket_ridge_quest_giver
repeatable: false
prerequisites:
  - rustbucket_ridge_slasher_takedown
objectives:
  - id: find_cipher
    type: explore
    description: Find the rogue AI technician in The Velvet Rope
    target_id: the_velvet_rope_brothel
    quantity: 1
rewards:
  xp: 800
  credits: 0
```

### REQ-AQD-3: Machete "Field Test" quest

A new file `content/quests/machete_field_test.yaml` MUST be created:

```yaml
id: machete_field_test
title: Field Test
description: >
  Cipher doesn't give these things away. The modification takes weeks and
  the components aren't cheap. But they have a standing arrangement: prove
  you can operate at this tier, and the next one is yours. The VIP Chamber
  is where you prove it.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - machete_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_chainsaw
      quantity: 1
```

### REQ-AQD-4: Gun "Signal in the Static" quest

A new file `content/quests/gun_signal_in_static.yaml` MUST be created:

```yaml
id: gun_signal_in_static
title: Signal in the Static
description: >
  Word's going around that someone in The Velvet Rope has been pulling
  neural chips out of salvage and wiring them into weapons. Nobody knows
  who. Nobody knows how to find them. That's the point. Prove you're
  serious and the signal gets clearer.
giver_npc_id: vantucky_quest_giver
repeatable: false
prerequisites:
  - vantucky_militia_commander_takedown
objectives:
  - id: find_cipher
    type: explore
    description: Find the rogue AI technician in The Velvet Rope
    target_id: the_velvet_rope_brothel
    quantity: 1
rewards:
  xp: 800
  credits: 0
```

### REQ-AQD-5: Gun "Field Test" quest

A new file `content/quests/gun_field_test.yaml` MUST be created:

```yaml
id: gun_field_test
title: Field Test
description: >
  Cipher doesn't give these things away. The modification takes weeks and
  the components aren't cheap. But they have a standing arrangement: prove
  you can operate at this tier, and the next one is yours. The VIP Chamber
  is where you prove it.
giver_npc_id: cipher
repeatable: false
prerequisites:
  - gun_signal_in_static
objectives:
  - id: kill_vip_boss
    type: kill
    description: Kill the VIP in the VIP Chamber
    target_id: gangbang
    quantity: 1
rewards:
  xp: 1200
  credits: 500
  items:
    - item_id: ai_ak47
      quantity: 1
```

### REQ-AQD-6: Quest visibility gating

- REQ-AQD-6a: `machete_signal_in_static` MUST NOT appear in any NPC's quest
  roster until `rustbucket_ridge_slasher_takedown` is completed by the player.
  This is enforced by the existing prerequisite mechanism in `quest/service.go`.
- REQ-AQD-6b: `gun_signal_in_static` MUST NOT appear until
  `vantucky_militia_commander_takedown` is completed.
- REQ-AQD-6c: `machete_field_test` MUST NOT appear on Cipher's roster until
  `machete_signal_in_static` is completed.
- REQ-AQD-6d: `gun_field_test` MUST NOT appear on Cipher's roster until
  `gun_signal_in_static` is completed.
- REQ-AQD-6e: A player who has completed both `machete_signal_in_static` and
  `gun_signal_in_static` (e.g., via admin commands) MUST see both Field Test
  quests on Cipher's roster simultaneously.

### REQ-AQD-7: Boss drop

The `gangbang` NPC template in `content/zones/the_velvet_rope.yaml` MUST be
updated to include a 5% drop chance for both AI items:

```yaml
loot:
  - item_id: ai_chainsaw
    chance: 0.05
  - item_id: ai_ak47
    chance: 0.05
```

Both items may drop independently on the same kill. Drop rate applies regardless
of whether the player has completed the Field Test quest.

### REQ-AQD-8: Test coverage

- REQ-AQD-8a: `TestQuestRegistry_CipherQuestsValid` — all four new quest files
  load and validate without error through the existing `QuestRegistry`.
- REQ-AQD-8b: `TestMacheteFieldTest_UnavailableWithoutPrereq` — `machete_field_test`
  is not available on Cipher's roster before `machete_signal_in_static` is
  completed.
- REQ-AQD-8c: `TestGunFieldTest_UnavailableWithoutPrereq` — same for
  `gun_field_test` / `gun_signal_in_static`.
- REQ-AQD-8d: `TestMacheteFieldTest_RewardsAIChainsaw` — completing
  `machete_field_test` adds one `ai_chainsaw` to the player's inventory.
- REQ-AQD-8e: `TestGunFieldTest_RewardsAIAK47` — completing `gun_field_test`
  adds one `ai_ak47` to the player's inventory.

---

## Dependencies

- REQ-DEP-1: This spec MUST NOT be implemented until the AI Item Content spec
  (`2026-04-20-ai-item-content.md`) is fully implemented and merged. The
  `ai_chainsaw` and `ai_ak47` item IDs must exist before quest rewards
  referencing them will validate.
- REQ-DEP-2: The AI Item Content spec depends on the AI Item Engine spec
  (`2026-04-20-ai-item-engine.md`). Full dependency chain:
  Engine → Content → Quest Delivery.

---

## Out of Scope

- Additional AI item variants for other factions or item types — to be specced
  in the AI Item Content Expansion spec.
- Cross-team quest gating (preventing a Gun player from completing Machete quests
  via the prerequisite system) — the prerequisite chain naturally gates by home
  zone boss, which is team-specific.
- Repeatable versions of Field Test — both quests are one-time rewards.

# Beginner Zone Rebalance

## Summary

Rustbucket Ridge and Vantucky are the game's two beginner zones. This feature audits and corrects their danger levels, NPC placement, and combat NPC difficulty to match beginner-appropriate constraints.

## Requirements

- REQ-BZR-1: Both Rustbucket Ridge and Vantucky MUST be classified as beginner zones. The zone-level `danger` field for Rustbucket Ridge MUST be changed from `dangerous` to `sketchy`. Vantucky is already `sketchy` and requires no zone-level change.
- REQ-BZR-2: The maximum danger level across all rooms in both zones MUST NOT exceed `sketchy`. No room in either zone MAY have a danger level of `dangerous` or higher (explicit or inherited).
- REQ-BZR-3: Any room that contains at least one non-combat NPC — defined as any NPC whose YAML `npc_type` is not `combat` — MUST have its room-level `danger` set to `safe`.
- REQ-BZR-4: A room marked `safe` MUST NOT spawn any NPC with `personality: aggressive` or `personality: hostile`.
- REQ-BZR-5: When REQ-BZR-3 and REQ-BZR-4 conflict for a room that currently mixes non-combat and aggressive NPC spawns, the aggressive NPC spawns MUST be removed. The non-combat NPC and the safe danger level are authoritative.
- REQ-BZR-6: Rooms in Rustbucket Ridge with `danger: inherit` will inherit `sketchy` after the zone-level change (REQ-BZR-1). These rooms MAY keep `danger: inherit` provided the inherited value satisfies REQ-BZR-2.
- REQ-BZR-7: All combat NPC types present in Rustbucket Ridge and Vantucky MUST be at beginner difficulty. Grunt-tier NPCs (ganger, scavenger, vantucky_scavenger, vantucky_militiaman, highway_bandit) MUST be level 1, max_hp ≤ 20, AC ≤ 12. Enforcer-tier NPCs (compound_guard, vantucky_gang_enforcer) MUST be level 2, max_hp ≤ 28, AC ≤ 13. Elite-tier NPCs (lieutenant, commissar) MUST be level 3, max_hp ≤ 35, AC ≤ 14.
- REQ-BZR-8: After applying REQ-BZR-5, if any room is left with only non-combat NPCs and no ambient threat, its `danger` MUST be `safe`.
- REQ-BZR-9: Architecture documentation for both zones (`docs/architecture/zones/rustbucket_ridge.md` and `docs/architecture/zones/vantucky.md`) MUST be updated to reflect any room danger level changes.

## Scope

### Content Files

- `content/zones/rustbucket_ridge.yaml` — change zone `danger` from `dangerous` to `sketchy`; for each room that currently has `danger: inherit` and spawns aggressive NPCs alongside non-combat NPCs, remove the aggressive spawns and set `danger: safe`
- `content/zones/vantucky.yaml` — verify all safe rooms have no aggressive NPC spawns (current state appears clean; confirm during implementation)
- `content/npcs/ganger.yaml` — rebalance to grunt tier (level 1, max_hp ≤ 20, AC ≤ 12)
- `content/npcs/scavenger.yaml` — rebalance to grunt tier
- `content/npcs/lieutenant.yaml` — rebalance to elite tier (level 3, max_hp ≤ 35, AC ≤ 14)
- `content/npcs/commissar.yaml` — rebalance to elite tier
- `content/npcs/vantucky_militiaman.yaml` — rebalance to grunt tier
- `content/npcs/highway_bandit.yaml` — rebalance to grunt tier
- `content/npcs/compound_guard.yaml` — rebalance to enforcer tier (level 2, max_hp ≤ 28, AC ≤ 13)
- `content/npcs/vantucky_gang_enforcer.yaml` — rebalance to enforcer tier

### Rooms Requiring Audit in Rustbucket Ridge

The following Rustbucket Ridge rooms currently have `danger: inherit` (which evaluates to `dangerous`) and contain one or more named non-combat NPCs alongside aggressive combat spawns. Each MUST be audited and corrected per REQ-BZR-3 through REQ-BZR-5:

- `the_rusty_oasis` — contains `slick_sally`; verify NPC type; if non-combat, remove ganger spawns and set `danger: safe`
- `junkers_dream` — contains `tina_wires`; verify NPC type; if non-combat, remove ganger/lieutenant spawns and set `danger: safe`
- `rotgut_alley` — contains `dex`; verify NPC type; if non-combat, remove ganger/scavenger spawns and set `danger: safe`
- `the_heap` — contains `old_rusty` and `mill_plain_thug`; verify NPC types; remove aggressive spawns from any non-combat NPC room and set `danger: safe`

The following Rustbucket Ridge rooms currently have `danger: sketchy` and contain a confirmed non-combat NPC alongside a scavenger spawn. Each MUST have the scavenger spawn removed and `danger` confirmed as `safe`:

- `salvage_hut` — `rustbucket_ridge_merchant` (confirmed non-combat); remove `scavenger` spawn
- `scorchside_camp` — `rustbucket_ridge_hireling` (confirmed non-combat); remove `scavenger` spawn
- `the_razor_nest` — `rustbucket_ridge_banker` (confirmed non-combat); remove `scavenger` spawn

The following Rustbucket Ridge rooms have `danger: sketchy` and contain named NPCs whose type must be verified during implementation. If non-combat: remove scavenger spawn and set `danger: safe`:

- `the_green_hell` — `herb`
- `the_bottle_shack` — `whiskey_joe`
- `the_tinkers_den` — `clutch`, `sparks`
- `scrapshack_23` — `gail_grinder_graves`
- `wreckers_rest` — `rio_wrench`
- `blood_camp` — `patch`

### Architecture Docs

- `docs/architecture/zones/rustbucket_ridge.md` — update room table danger column
- `docs/architecture/zones/vantucky.md` — confirm accuracy; update if any rooms change

## Plan

All NPC stat baselines confirmed by reading current YAML files before planning.

### Step 1 — Rebalance grunt-tier combat NPCs (4 files, parallelizable)

**ganger.yaml**: `ac: 14` → `ac: 12`. Level 1 and HP 18 are already within spec.

**scavenger.yaml**: `ac: 13` → `ac: 12`. Level 1 and HP 14 are already within spec.

**vantucky_militiaman.yaml**: `level: 2` → `level: 1`, `max_hp: 24` → `max_hp: 20`, `ac: 14` → `ac: 12`.

**highway_bandit.yaml**: `level: 2` → `level: 1`, `max_hp: 22` → `max_hp: 20`, `ac: 14` → `ac: 12`.

### Step 2 — Rebalance enforcer-tier combat NPCs (2 files, parallelizable with Step 1)

**compound_guard.yaml**: `level: 4` → `level: 2`, `max_hp: 44` → `max_hp: 28`, `ac: 17` → `ac: 13`.

**vantucky_gang_enforcer.yaml**: `level: 3` → `level: 2`, `max_hp: 30` → `max_hp: 28`. `ac: 13` already within spec.

### Step 3 — Rebalance elite-tier combat NPCs (2 files, parallelizable with Steps 1–2)

**lieutenant.yaml**: `max_hp: 42` → `max_hp: 35`, `ac: 16` → `ac: 14`. Level 3 already within spec.

**commissar.yaml**: `level: 4` → `level: 3`, `max_hp: 42` → `max_hp: 35`, `ac: 16` → `ac: 14`.

### Step 4 — Update Rustbucket Ridge zone danger (1 change)

In `content/zones/rustbucket_ridge.yaml`, change the zone-level `danger` field:
```yaml
danger: dangerous  →  danger: sketchy
```

### Step 5 — Fix Rustbucket Ridge rooms with non-combat NPCs (13 rooms in one file)

In `content/zones/rustbucket_ridge.yaml`, apply the following per-room changes. All named NPCs below are confirmed non-combat from NPC file audit.

| Room | Current danger | Action |
|------|---------------|--------|
| `the_rusty_oasis` | inherit | Remove `ganger` spawn ×2; set `danger: safe` |
| `junkers_dream` | inherit | Remove `ganger` spawn ×2 and `lieutenant` spawn ×1; set `danger: safe` |
| `rotgut_alley` | inherit | Remove `ganger` spawn ×2 and `scavenger` spawn ×1; set `danger: safe` |
| `the_heap` | inherit | Remove `ganger` spawn ×2 and `mill_plain_thug` spawn ×1; set `danger: safe` |
| `salvage_hut` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `scorchside_camp` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `the_razor_nest` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `the_green_hell` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `the_bottle_shack` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `the_tinkers_den` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `scrapshack_23` | sketchy | Remove `scavenger` spawn ×2; set `danger: safe` |
| `wreckers_rest` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |
| `blood_camp` | sketchy | Remove `scavenger` spawn ×1; set `danger: safe` |

### Step 6 — Verify Vantucky safe rooms (read-only check)

Confirm that `vantucky_the_compound`, `vantucky_neutral_pawn`, `vantucky_neutral_back`, `vantucky_neutral_vault`, and `vantucky_fourth_plain_west` have no aggressive NPC spawns. No changes expected. If any violations are found, remove the aggressive spawns.

### Step 7 — Update architecture docs

Update the room danger columns in:
- `docs/architecture/zones/rustbucket_ridge.md` — change zone danger from `dangerous` to `sketchy`; update the 13 rooms changed in Step 5 from their prior value to `safe`
- `docs/architecture/zones/vantucky.md` — confirm accuracy; update only if Step 6 found violations

### Step 8 — Commit

Commit all changes with message summarizing: NPC rebalance (8 files), RR zone danger change, 13 room corrections, architecture doc updates.

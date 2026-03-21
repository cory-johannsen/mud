# Crafting

Recipe-driven crafting system for weapons, armor, items, and consumables from typed individual materials. Global recipe registry; hybrid quick/downtime model based on item complexity and player Rigging rank. See `docs/superpowers/specs/2026-03-20-crafting-design.md` for the full design spec.

## Requirements

- [ ] Material registry (`content/materials.yaml`)
  - REQ-CRAFT-6: MUST be the single source of truth for all valid material IDs; recipes referencing unknown material IDs MUST be a fatal load error.
  - [ ] ~100 materials across 5 categories: mechanical, chemical, organic, electrical, misc
- [ ] Recipe registry (`content/recipes/`)
  - REQ-CRAFT-10: Recipes referencing unknown `output_item_id` values per the category/registry mapping MUST be a fatal load error.
  - [ ] YAML schema: id, name, output_item_id, output_count, category, complexity, dc, quick_craft_min_rank, materials list
  - [ ] Complexity tiers 1–4 with default quick_craft_min_rank (untrained/trained/expert/master)
  - [ ] `output_item_id` validation by category: consumable→Item(), weapon→Weapon(), armor→Armor(), item→Item()
- [ ] Material inventory (`character_materials` table)
  - REQ-CRAFT-7: MUST be loaded into `PlayerSession.Materials` at login and persisted on every craft transaction.
  - REQ-CRAFT-12: All material deductions MUST execute within a single DB transaction and roll back entirely on failure.
  - [ ] `CharacterMaterialsRepository` with `Load`, `DeductMany`, `Add` methods
  - [ ] `materials` and `materials <category>` commands
- [ ] Material sources
  - [ ] Merchants — `material_stock` field on merchant NPC YAML; `buy` command extended to detect material vs item purchases
  - REQ-CRAFT-8: `scavenge` MUST use Scavenging skill vs zone `material_pool.dc`.
  - REQ-CRAFT-9: Zone with no `material_pool` MUST yield nothing on `scavenge` with no error.
  - REQ-CRAFT-11: `scavenge` MUST be limited to one attempt per room entry per player, tracked in `PlayerSession.ScavengeExhaustedRoomID`, cleared on room exit.
  - [ ] Zone YAML `material_pool` block with dc and weighted drops
  - [ ] `scavenge` command (Scavenging skill check; four-tier outcomes: crit=3 materials, success=1-2, fail/crit-fail=nothing)
  - [ ] NPC YAML `material_drops` list (material ID, quantity range, chance); drops tracked in `FloorManager.MaterialDrops`
  - [ ] `FloorManager.MaterialDrops map[string]map[string]int` extension; `take` command extended to handle material names
- [ ] Crafting commands
  - REQ-CRAFT-1: `craft list` MUST show all accessible recipes; above quick-craft threshold MUST show `[downtime only]`; insufficient materials MUST show `[missing: N]`.
  - REQ-CRAFT-2: `craft <item>` MUST fail with a message listing missing materials if insufficient.
  - REQ-CRAFT-3: Materials MUST be deducted at `craft confirm` time, not at completion time.
  - REQ-CRAFT-4: Critical failure MUST consume all required materials with no output.
  - REQ-CRAFT-5: Failure MUST consume half of each required material quantity (rounded down), with no output.
  - REQ-CRAFT-13: `craft confirm` MUST fail if `PlayerSession.PendingCraftRecipeID` is empty.
  - [ ] `craft list [category]` command
  - [ ] `craft <item>` command — validates materials, sets `PlayerSession.PendingCraftRecipeID`, displays details
  - [ ] `craft confirm` — executes quick craft or hands off to downtime via `DowntimeCraftStarter` interface
  - [ ] `PlayerSession.PendingCraftRecipeID` — cleared by any non-confirm command or room exit
  - [ ] Quick craft: critical success = output_count+1 items; success = output_count; failure = half materials; crit failure = all materials lost
- [ ] Downtime handoff
  - [ ] `DowntimeCraftStarter` interface: `BeginCraftActivity(ctx, characterID, recipeID, daysRequired)`
  - [ ] Days required by complexity: standard=1, advanced=2, expert=4
- [ ] Proto messages: `MaterialsRequest`, `CraftListRequest`, `CraftRequest`, `CraftConfirmRequest`, `CraftResultEvent`

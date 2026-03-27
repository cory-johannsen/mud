# Craft (Downtime)

Craft a non-magical item using the Crafting skill. Requires a recipe, raw materials, and access to appropriate tools. The player spends downtime days to produce items at a reduced cost compared to purchasing them.

## Requirements

- REQ-CRAFT-DT-1: The `craft` downtime command MUST gate on the player having the required raw materials in inventory.
- REQ-CRAFT-DT-2: On success, the crafted item MUST be added to the player's backpack.
- REQ-CRAFT-DT-3: On failure, raw materials MUST NOT be refunded.
- REQ-CRAFT-DT-4: On critical success, one batch of raw materials MUST be refunded.
- REQ-CRAFT-DT-5: The crafting downtime activity MUST integrate with the existing `content/recipes.yaml` recipe system.
- REQ-CRAFT-DT-6: Available recipes MUST be filtered to those the player's character meets prerequisites for.

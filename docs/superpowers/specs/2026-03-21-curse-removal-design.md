# Curse Removal — Design Spec

## Overview

Adds a `chip_doc` non-combat NPC type and an `uncurse <item>` command. Players visit a chip_doc in a Safe room to attempt curse removal on an equipped cursed item. The attempt costs credits upfront and resolves via a Rigging skill check against the chip_doc's configured DC. Outcome ranges from full curse removal with partial refund (critical success) to item staying cursed with a fatigued penalty (critical failure).

## Requirements

### chip_doc NPC Type

- REQ-CR-1: A `chip_doc` NPC type MUST be added to the non-combat NPC framework.
- REQ-CR-2: The `chip_doc` NPC YAML config MUST include two fields: `removal_cost` (integer credits, MUST be >= 1) and `check_dc` (integer difficulty class, MUST be >= 1). A config with either field less than 1 MUST be a fatal load error.
- REQ-CR-3: `chip_doc` NPCs MUST only be placed in Safe rooms. Placing a `chip_doc` in a non-Safe room MUST be a fatal load error.

### uncurse Command

- REQ-CR-4: The `uncurse <item>` command MUST be available to players.
- REQ-CR-5: The `uncurse` command MUST be blocked if the player is not in a Safe room containing a `chip_doc` NPC, with an error message stating no chip_doc is present. (Safe rooms cannot have active combat, so chip_doc availability is guaranteed by REQ-CR-3.)
- REQ-CR-6: The `uncurse` command MUST be blocked if the target item is not currently equipped, with an error message.
- REQ-CR-7: The `uncurse` command MUST be blocked if the target item is not cursed, with an error message.
- REQ-CR-8: The `uncurse` command MUST be blocked if the player has fewer credits than `removal_cost`, with an error message stating the required cost.
- REQ-CR-9: If all preconditions pass, the player's credits MUST be deducted by `removal_cost` before the Rigging skill check is performed.

### Rigging Check Outcomes

- REQ-CR-10: The Rigging skill check MUST be rolled against the `chip_doc`'s `check_dc` using the four-degree outcome system: critical success (total >= DC + 10, or natural 20); success (total >= DC); failure (total < DC); critical failure (total <= DC - 10, or natural 1).
- REQ-CR-11: On critical success, the curse MUST be removed, the item MUST be unequipped, the item's modifier MUST be set to `defective`, and `floor(removal_cost / 2)` credits MUST be refunded to the player.
- REQ-CR-12: On success, the curse MUST be removed, the item MUST be unequipped, and the item's modifier MUST be set to `defective`. No refund is issued.
- REQ-CR-13: On failure, the item MUST remain cursed and equipped. No refund is issued.
- REQ-CR-14: On critical failure, the item MUST remain cursed and equipped, and the `fatigued` condition MUST be applied to the player lasting until the player's next long rest. If the player already has the `fatigued` condition, applying it MUST reset its duration rather than stack. No refund is issued.

## Design

### Command Routing

`uncurse <item>` is handled by a new `handleUncurse()` function in the gameserver. The handler:

1. Checks for a `chip_doc` NPC in the current room
2. Resolves the item reference from the player's equipped slots
3. Validates the item is cursed
4. Checks and deducts credits
5. Performs the Rigging skill check
6. Applies the outcome per REQ-CR-11 through REQ-CR-14

### Cursed → Defective Transition

On a successful outcome, the item's curse state is cleared and its modifier is set to `defective` per the equipment-mechanics spec. The item is moved from the equipped slot to inventory.

### Fatigued Condition

The `fatigued` condition does not yet have a formal condition YAML entry; one MUST be created as part of this feature's implementation. Its duration is "until next long rest" — cleared by the existing `handleRest()` restoration logic.

### Dependencies

- `equipment-mechanics` — defines cursed item state, the cursed→defective modifier transition, and the item equip/unequip model
- `non-combat-npcs` — provides the NPC type framework that `chip_doc` extends
- `zone-content-expansion` — responsible for placing one lore-appropriate `chip_doc` NPC per zone in a Safe room

## Out of Scope

- Defining the lore flavor or dialogue of chip_doc NPCs is content work belonging to non-combat-npcs and zone-content-expansion.
- Multiple cursed items cannot be uncursed in a single command invocation.
- Partial credit refunds on failure are not included.
- The `chip_doc` NPC does not offer any service other than curse removal.

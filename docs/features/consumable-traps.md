# Consumable Traps

Trap items as craftable and purchasable consumables that players deploy in combat. A deployed trap occupies a floor slot in the current room and triggers automatically when an enemy enters the room. Extends the base `traps` feature (which handles world-placed traps) with player-deployable trap items.

## Requirements

- [ ] Trap consumable items
  - [ ] Trap items defined as consumable item type with trap-specific fields: `trap_type`, `trigger`, `effect`, `dc`
  - [ ] Trap types: subset of base traps feature types (mines, bear traps, trip wires at minimum)
  - [ ] Trap items craftable via crafting system (recipe entries) and purchasable from merchants
- [ ] `deploy <trap item>` command
  - [ ] Costs 1 AP in combat; removes item from inventory; places trap instance on room floor
  - [ ] Fails if player has no trap item in inventory
  - [ ] Fails outside of combat (traps feature handles world-placed traps)
- [ ] Floor trap tracking
  - [ ] `FloorManager` (or equivalent) tracks deployed trap instances per room: `map[roomID][]DeployedTrap`
  - [ ] `DeployedTrap`: item ID, deploying character ID, DC, effect, trap type
  - [ ] Multiple traps can be deployed in the same room; all are live simultaneously
  - [ ] Deployed traps persist for the duration of the combat encounter; cleared on encounter end
- [ ] Trigger: enemy enters room
  - [ ] When an enemy moves into a room with deployed traps, all live traps in that room trigger against that enemy
  - [ ] Trigger evaluation uses the base traps feature's detect/trigger/effect logic
  - [ ] Enemy Awareness check vs trap DC to detect; failed detection → trap fires immediately
- [ ] Ready action integration
  - [ ] The ready-action feature uses this floor-state tracking to support "enemy enters room" as a trigger condition

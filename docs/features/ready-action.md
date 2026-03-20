# Ready Action

Implement the PF2E Ready action: costs 2 AP, declares a trigger condition and a stored action (Strike, Step, etc.) that fires automatically as a Reaction when the trigger occurs during the round.

## Requirements

- [ ] Ready action
  - [ ] `ready <action> when <trigger>` command — costs 2 AP; stores a (trigger, action) pair on the player session
  - [ ] Trigger evaluation during round resolution — check trigger conditions and fire the stored action as a Reaction
  - [ ] Supported triggers: enemy moves adjacent, enemy attacks player, ally is attacked
  - [ ] Supported readied actions: Strike, Step, Raise Shield
  - [ ] Readied action consumes the player's remaining Reaction for the round
  - [ ] Readied trigger not met — readied action is lost at end of round (no refund)

# Focus Points

A Focus Point pool per character that powers focus technologies (the Gunchete equivalent of PF2E focus spells). Players spend Focus Points to activate certain technologies and restore them via the Recalibrate downtime activity.

## Requirements

- [ ] Focus Point pool
  - [ ] `FocusPoints int` and `MaxFocusPoints int` on `PlayerSession` and persisted to character DB
  - [ ] Max pool size determined by class features and feats (starts at 0; each focus-granting feature adds 1, cap at 3)
  - [ ] Focus Points displayed on character sheet and prompt
- [ ] Spending
  - [ ] Focus technologies cost 1 Focus Point on activation; activation fails if pool is empty
  - [ ] `FocusPoints` decremented on activation, persisted immediately
- [ ] Restoration
  - [ ] Recalibrate downtime activity restores Focus Points (per downtime feature outcomes)
  - [ ] Full restore on long rest (deferred to `resting` feature)
- [ ] Class feature / feat integration
  - [ ] `grants_focus_point: true` field on class feature and feat YAML
  - [ ] `MaxFocusPoints` computed at login from all active class features and feats with `grants_focus_point: true`, capped at 3

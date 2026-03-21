# Focus Points

A Focus Point pool per character powers focus technologies (`focus_cost: true` technologies). Players spend Focus Points to activate these technologies and restore them via the Recalibrate downtime activity. Max pool size derived from class features and feats at login, capped at 3. See `docs/superpowers/specs/2026-03-20-focus-points-design.md` for full design spec.

## Requirements

### Data Model

- [ ] REQ-FP-13: `CharacterSheetView` proto message gains `int32 focus_points` and `int32 max_focus_points`
- [ ] REQ-FP-14: Stat update event after FP spend MUST carry `focus_points` and `max_focus_points` to frontend
- [ ] `characters` table gains `focus_points int NOT NULL DEFAULT 0`
- [ ] `PlayerSession` gains `FocusPoints int` and `MaxFocusPoints int`
- [ ] `TechnologyDef` gains `FocusCost bool \`yaml:"focus_cost,omitempty"\``
- [ ] `ClassFeature` gains `GrantsFocusPoint bool \`yaml:"grants_focus_point,omitempty"\``
- [ ] Feat definitions gain `GrantsFocusPoint bool \`yaml:"grants_focus_point,omitempty"\``

### MaxFocusPoints Computation

- [ ] REQ-FP-1: `MaxFocusPoints` computed at login from all active class features and feats with `grants_focus_point: true`, capped at 3
- [ ] REQ-FP-2: `MaxFocusPoints` recomputed after any feat swap or level-up before next action
- [ ] REQ-FP-11: On login, `FocusPoints` clamped to `MaxFocusPoints` if it exceeds it

### Spending

- [ ] REQ-FP-3: Technologies with `focus_cost: true` require 1 Focus Point to activate
- [ ] REQ-FP-4: Activation fails with "Not enough Focus Points. (N/M)" if `FocusPoints == 0`
- [ ] REQ-FP-5: `FocusPoints` decremented and persisted immediately on activation, before result sent to client
- [ ] REQ-FP-6: Each activation costs exactly 1 Focus Point regardless of technology level

### Restoration

- [ ] REQ-FP-7: Recalibrate (downtime) critical success / success: `FocusPoints = MaxFocusPoints`, persist immediately
- [ ] REQ-FP-8: Recalibrate failure: `FocusPoints += 1` (capped at `MaxFocusPoints`), persist immediately
- [ ] Long rest restoration deferred to `resting` feature

### Display

- [ ] REQ-FP-9: Prompt displays `FP: N/M` if `MaxFocusPoints > 0`; omitted if 0
- [ ] REQ-FP-10: Character sheet displays Focus Points row if `MaxFocusPoints > 0`; omitted if 0; placement: after HP row

### Validation

- [ ] REQ-FP-12: `TechnologyDef.Validate()` MUST error if `FocusCost == true && Passive == true`

### Persistence

- [ ] `CharacterRepository` gains `SaveFocusPoints(ctx, characterID, focusPoints int) error` for targeted FP updates

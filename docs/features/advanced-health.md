# Advanced Health

Models drugs, alcohol, medicine, poisons, toxins, addiction, and recovery as health mechanics via a data-driven substance system with onset delays, overdose thresholds, and an addiction/withdrawal/recovery state machine.

Design spec: `docs/superpowers/specs/2026-03-21-advanced-health-design.md`

## Requirements

- [ ] REQ-AH-0A: `SubstanceEffect.apply_condition` MUST specify a condition ID to apply on onset; `stacks int` (default 1) specifies the stack count.
- [ ] REQ-AH-0B: `SubstanceEffect.remove_condition` MUST specify a condition ID to remove on onset.
- [ ] REQ-AH-0C: `SubstanceEffect.hp_regen int` MUST specify HP per tick while active (medicine only).
- [ ] REQ-AH-0D: `SubstanceEffect.cure_conditions []string` MUST specify condition IDs removed immediately on onset (medicine only).
- [ ] REQ-AH-1: `SubstanceDef` MUST be loaded from `content/substances/*.yaml`.
- [ ] REQ-AH-2: `SubstanceDef` MUST have fields: `id`, `name`, `category`, `onset_delay`, `duration`, `effects`, `remove_on_expire`, `addictive`, `addiction_chance`, `overdose_threshold`, `overdose_condition`, `withdrawal_conditions`, `recovery_duration`.
- [ ] REQ-AH-3: `SubstanceRegistry` MUST be loaded at startup and injected into `GameServiceServer`.
- [ ] REQ-AH-4: `SubstanceDef.Validate()` MUST reject empty `id`/`name`, invalid `category`, invalid duration strings, `addiction_chance` outside `[0,1]`, or `overdose_threshold < 1`.
- [ ] REQ-AH-4A: At startup, all condition ID references in every `SubstanceDef` MUST be cross-validated against the condition registry; any unknown ID MUST cause a fatal startup error.
- [ ] REQ-AH-5: `PlayerSession` MUST gain `ActiveSubstances []ActiveSubstance`.
- [ ] REQ-AH-6: `PlayerSession` MUST gain `AddictionState map[string]SubstanceAddiction`.
- [ ] REQ-AH-6A: `PlayerSession` MUST gain `SubstanceConditionRefs map[string]int` for reference counting conditions applied by active substances.
- [ ] REQ-AH-7: Substance state MUST be session-only; no DB persistence; all fields cleared on disconnect.
- [ ] REQ-AH-8: Substance items MUST have an `effect` field with a substance ID; `use` handler MUST look up the substance.
- [ ] REQ-AH-8A: The `use` handler MUST block `category == "poison"` or `"toxin"` with `"You can't use that directly."` before calling `applySubstanceDose`.
- [ ] REQ-AH-9: `applySubstanceDose` MUST create or increment `ActiveSubstance` entries, extending `ExpiresAt` on re-dose.
- [ ] REQ-AH-10: `DoseCount > overdose_threshold` MUST immediately apply `overdose_condition` and send overdose message.
- [ ] REQ-AH-11: Addictive dose MUST advance: clean→at_risk, at_risk→addicted (probabilistic with message), addicted→see REQ-AH-18.
- [ ] REQ-AH-12: Session 5-second ticker MUST call `tickSubstances(uid)`.
- [ ] REQ-AH-13: `tickSubstances` MUST fire onset (apply effects, increment refs, send "kicks in") and expiry (decrement refs, remove zero-ref conditions, call `onSubstanceExpired`).
- [ ] REQ-AH-14: Medicine hp_regen MUST apply per tick while active; clamped to `MaxHP`; no per-tick message.
- [ ] REQ-AH-15: `onSubstanceExpired` MUST trigger withdrawal if `status == "addicted"`: set withdrawal, apply withdrawal conditions (incrementing refs), send withdrawal message.
- [ ] REQ-AH-16: `tickSubstances` MUST check `WithdrawalUntil` expiry; on expiry, remove withdrawal conditions (decrement refs), set status to clean, send recovery message.
- [ ] REQ-AH-17: Dose while in withdrawal MUST reset `WithdrawalUntil`, remove withdrawal conditions (decrement refs), set status back to `"addicted"`.
- [ ] REQ-AH-18: Dose while `status == "addicted"` MUST re-roll addiction chance; on success send `"Your dependency deepens."`; status stays `"addicted"`.
- [ ] REQ-AH-19: Addiction state MUST be independent per substance ID.
- [ ] REQ-AH-20: `ApplySubstanceByID(uid, substanceID) error` MUST call `applySubstanceDose` directly (bypassing the `use` handler category guard).
- [ ] REQ-AH-21: Poisoned weapon items MUST have `poison_substance_id string`; attack pipeline MUST call `ApplySubstanceByID` on hit.
- [ ] REQ-AH-22: Trap definitions MUST support `substance_id string`; trap triggers MUST call `ApplySubstanceByID` if non-empty.
- [ ] REQ-AH-24: `cure_conditions` effects MUST remove listed conditions immediately on onset, decrementing refs.
- [ ] REQ-AH-25: `hp_regen` effects MUST add HP per tick while active, clamped to `MaxHP`.
- [ ] REQ-AH-26: `SubstanceDef.Validate()` MUST reject `category == "medicine"` with `addictive == true`.
- [ ] REQ-AH-27: Medicine `overdose_threshold` enforcement is a content decision; no code validation required.

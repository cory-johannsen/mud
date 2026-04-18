# Tech Catalog (KnownTechs) + Slot Assignment Modal Design

## Goal

Introduce per-archetype tech casting models (mirroring PF2E class spell mechanics), a per-character tech catalog (`KnownTechs`) for catalog-based archetypes, heightened slot assignment for all prepared archetypes, and a richer rest rearrangement modal with level tabs, heightened indicators, and back/forward navigation. Job definitions may override their archetype's default casting model to support full job-level PF2E fidelity.

## Architecture

The existing `SpontaneousTechs` infrastructure is renamed to `KnownTechs` with unified semantics across all archetypes. A `casting_model` string field is added to both `Archetype` and `Job` content definitions; the job value overrides the archetype default at runtime.

`RearrangePreparedTechs` and `LevelUpTechnologies` select their behavior based on the resolved casting model. The `FeatureChoiceModal` gains level filter tabs, heightened badges, and Back/Forward navigation buttons for all prepared models.

## Tech Stack

- Go backend: session, storage/postgres, gameserver, technology_assignment, ruleset content types
- TypeScript frontend: `FeatureChoiceModal.tsx`, `GameContext.tsx`
- Content YAML: `archetypes/*.yaml`, `jobs/*.yaml` — new `casting_model` field
- Postgres: rename table `character_spontaneous_technologies` → `character_known_technologies`
- Proto/sentinel: `ChoicePrompt` extended with `slotContext`; new `[heightened:N]`, `[back]`, `[forward]`, `[confirm]` sentinels in option strings

---

## Section 1: Casting Models

### REQ-TC-1: Casting model taxonomy

Five casting models MUST be defined as a string enum (`CastingModel`):

| Model | Value | PF2E Basis | Archetypes (default) | Catalog | +2/level | +2 L1 extras |
|-------|-------|-----------|----------------------|---------|----------|--------------|
| Wizard | `wizard` | Wizard, Witch | Nerd, Schemer | KnownTechs | ✅ | ✅ |
| Druid | `druid` | Druid, Cleric | Naturalist, Zealot | Full pool | ❌ | ❌ |
| Ranger | `ranger` | Ranger | Drifter | KnownTechs | ❌ | ❌ |
| Spontaneous | `spontaneous` | Bard, Sorcerer | Influencer | KnownTechs | — | — |
| None | `none` | Fighter, Rogue | Aggressor, Criminal | — | — | — |

### REQ-TC-2: `casting_model` field on Archetype and Job

Both `Archetype` and `Job` content YAML definitions MUST support an optional `casting_model` field (string, one of the five values above).

- If `Job.casting_model` is set, it takes precedence over `Archetype.casting_model`.
- If neither is set, behavior defaults to `none`.
- The resolved casting model is computed at runtime as: `coalesce(job.CastingModel, archetype.CastingModel, "none")`.

**Example override:** The `salesman` job (a Schemer job) sets `casting_model: spontaneous` to use Bard semantics, overriding the Schemer archetype's `wizard` default.

### REQ-TC-3: Archetype default casting models

Each archetype YAML MUST set `casting_model` to the appropriate default:

| Archetype | `casting_model` |
|-----------|----------------|
| nerd | `wizard` |
| schemer | `wizard` |
| naturalist | `druid` |
| zealot | `druid` |
| drifter | `ranger` |
| influencer | `spontaneous` |
| aggressor | `none` |
| criminal | `none` |

---

## Section 2: Data Model

### REQ-TC-4: Rename `SpontaneousTechs` → `KnownTechs`

The following renames MUST be applied consistently across the entire codebase:

| Before | After |
|--------|-------|
| `PlayerSession.SpontaneousTechs` | `PlayerSession.KnownTechs` |
| `character_spontaneous_technologies` (DB table) | `character_known_technologies` |
| `SpontaneousTechRepo` (interface + impl) | `KnownTechRepo` |
| `spontaneousTechRepo` (field names) | `knownTechRepo` |
| All local variables named `spontaneousTechs` | `knownTechs` |

### REQ-TC-5: No new DB table

The existing `character_known_technologies` table (post-rename) carries the full catalog for all archetypes that use `KnownTechs`. Schema is unchanged: `(character_id, tech_level, tech_id)`.

### REQ-TC-6: Unified field semantics by casting model

| Model | `KnownTechs` meaning | `PreparedTechs` meaning |
|-------|---------------------|------------------------|
| `wizard` | Catalog: techs eligible to prepare | Current slot assignments |
| `druid` | Not used for pool filtering | Current slot assignments |
| `ranger` | Catalog: techs eligible to prepare | Current slot assignments |
| `spontaneous` | Full known list (unchanged behavior) | Not used |
| `none` | Empty | Empty |

### REQ-TC-7: Rename backward compatibility

The DB migration renames `character_spontaneous_technologies` to `character_known_technologies`. All existing data is preserved. The migration MUST be idempotent.

---

## Section 3: Catalog Population

Rules in this section apply to `wizard` and `ranger` models only unless stated otherwise.

### REQ-TC-8: Level-up slot picks populate KnownTechs

When a prepared tech is assigned to a slot during any level-up flow (`fillFromPreparedPool`, `fillFromPreparedPoolWithSend`), the assigned tech ID MUST also be added to `KnownTechs[level]` if not already present. Applies to `wizard`, `ranger`.

### REQ-TC-9: L1 catalog extras at character creation (`wizard` only)

After the player fills their L1 prepared slots during level-up, they MUST be prompted to pick 2 additional techs from the remaining L1 grant pool. These are added to `KnownTechs[1]` only — NOT assigned to any `PreparedTechs` slot.

If fewer than 2 pool entries remain after slot picks, all remaining entries are added without prompting. If zero remain, this step is skipped silently.

Applies to `wizard` model only. `ranger` and `druid` do NOT receive L1 extras.

### REQ-TC-10: +2 catalog additions per level-up (`wizard` only)

At each level-up (level 2 and above), after all slot grants are processed, the player MUST be prompted to pick 2 techs to add to their catalog. These techs may be at any tech level for which the character has at least one prepared slot. They are added to `KnownTechs[level]` only — no slot assignment.

If all grant pool entries at all eligible levels are already in `KnownTechs`, the picker is skipped silently.

Applies to `wizard` model only.

### REQ-TC-11: Trainer populates KnownTechs

When a tech trainer teaches a tech (resolves a pending tech slot), the tech MUST be added to `KnownTechs[techLevel]` in addition to being assigned to `PreparedTechs[techLevel][slotIdx]`. The trainer flow is otherwise unchanged.

Applies to `wizard` and `ranger` models. For `druid`, the trainer still assigns to `PreparedTechs` but does NOT add to `KnownTechs` (the druid model uses the full pool at rest, not a catalog).

### REQ-TC-12: Spontaneous model unchanged

For `spontaneous` archetypes (Influencer), `KnownTechs` is populated exactly as `SpontaneousTechs` was before this change. No new catalog extras or +2 per level rules apply.

---

## Section 4: Rest Rearrangement

### REQ-TC-13: Casting-model-aware pool selection

`RearrangePreparedTechs` MUST resolve the casting model for the character and select pool behavior accordingly:

- **`wizard` / `ranger`:** Build the option list for a slot of level N from `sess.KnownTechs[M]` for all M where 1 ≤ M ≤ N. The grant pool definition is NOT consulted.
- **`druid`:** Build the option list for a slot of level N from the grant pool entries at all levels M where 1 ≤ M ≤ N (all pool entries at all levels ≤ slot level). The grant pool IS consulted; `KnownTechs` is NOT used as a filter.
- **`spontaneous` / `none`:** `RearrangePreparedTechs` is never called.

### REQ-TC-14: Heightened assignment

A tech of level M placed in a slot of level N (M < N) is considered heightened by N−M. The server MUST embed the heighten delta in the option string using the sentinel `[heightened:N]` (e.g., `[heightened:2]`) appended after the tech's option text. The frontend strips and renders this as a badge.

Applies to all prepared casting models (`wizard`, `druid`, `ranger`).

### REQ-TC-15: Default to highest known level

For a slot of level N:
- **`wizard` / `ranger`:** If `KnownTechs[N]` is empty, the active level tab defaults to the highest M < N where `KnownTechs[M]` is non-empty. If no level has entries, the slot is skipped and a warning is logged.
- **`druid`:** If the grant pool has no entries at level N, the active level tab defaults to the highest M < N with pool entries. If no level has pool entries, the slot is skipped and a warning is logged.

### REQ-TC-16: Back/forward navigation

- A `[back]` sentinel option MUST be prepended to every slot's option list except the first slot of the first level.
- A `[forward]` sentinel option MUST be appended to every slot's option list except the last slot of the last level, where it is replaced by a `[confirm]` sentinel option.
- The server tracks all in-progress slot assignments in memory during the rearrangement session.
- Assignments are written to `PreparedTechs` and the DB only after the player selects `[confirm]`.
- Selecting `[back]` decrements the current slot index and re-prompts with the previous slot's options, pre-selecting the in-progress assignment.

Applies to all prepared casting models (`wizard`, `druid`, `ranger`).

### REQ-TC-17: Single eligible tech auto-assign

If exactly one eligible tech exists for a given slot (after applying model-appropriate pool logic and accounting for all levels ≤ slot level), the slot is auto-assigned without prompting. A send message is emitted: `"Level N, {SlotNoun} M of TOTAL (auto): {techID}"`.

---

## Section 5: Frontend Modal

### REQ-TC-18: slotContext in ChoicePrompt

The `ChoicePrompt` TypeScript interface MUST be extended:

```typescript
interface ChoicePrompt {
  featureId: string;
  prompt: string;
  options: string[];
  slotContext?: {
    slotNum: number;
    totalSlots: number;
    slotLevel: number;
  };
}
```

The server MUST populate `slotContext` for all tech slot assignment prompts. The modal renders a slot progress header: `Slot {slotNum} of {totalSlots} — Level {slotLevel}`.

### REQ-TC-19: Level filter tabs

The `FeatureChoiceModal` MUST parse the `(Lv N)` substring from each option string to extract available tech levels. It MUST render a row of level filter tab buttons (e.g., `[L1] [L2] [L3]`). Clicking a tab filters the displayed option list client-side. The active tab defaults to `slotContext.slotLevel`; if that level has no options, the highest available level is active. No server round-trip occurs on tab change.

### REQ-TC-20: Heightened badge

Option strings containing a `[heightened:N]` sentinel MUST have that sentinel stripped from displayed text and replaced with a visible badge: `+N` rendered in gold (`#e0c060`) adjacent to the tech name.

### REQ-TC-21: Back/Forward navigation buttons

- Option strings equal to `[back]` are NOT rendered as numbered choices. Instead, a `← Back` button is rendered at the bottom-left of the modal.
- Option strings equal to `[forward]` are NOT rendered as numbered choices. Instead, a `Next →` button is rendered at the bottom-right.
- Option strings equal to `[confirm]` are NOT rendered as numbered choices. Instead, a `Confirm` button is rendered at the bottom-right (replaces `Next →` on the final slot).
- Clicking `← Back` sends the `[back]` sentinel value as the selection.
- Clicking `Next →` sends the `[forward]` sentinel value.
- Clicking `Confirm` sends the `[confirm]` sentinel value. The server treats `[confirm]` as the signal to write all in-progress assignments to the DB.

---

## Section 6: Edge Cases

### REQ-TC-22: Druid model at trainer

For `druid` model archetypes, tech trainers still resolve pending tech slots and assign a tech to `PreparedTechs[techLevel][slotIdx]`. The trained tech is NOT added to `KnownTechs` (druid model uses the full pool, so catalog tracking is unnecessary).

### REQ-TC-23: Job override propagation

When a job overrides the casting model (e.g., `salesman` overrides Schemer's `wizard` with `spontaneous`), ALL casting-model-dependent flows MUST use the job's model: level-up catalog population, trainer behavior, and rest rearrangement pool selection.

### REQ-TC-24: Missing casting_model field

If neither the job nor the archetype YAML specifies `casting_model`, the resolved model is `none`. No tech flows are triggered. This is the safe default for Aggressor and Criminal jobs.

### REQ-TC-25: Spontaneous archetype rest unchanged

`RearrangePreparedTechs` is never called for `spontaneous` model characters. The +2 per level-up and L1 extras rules do NOT apply to `spontaneous` models.

---

## Section 7: Testing Requirements

### REQ-TC-26: Property-based tests (SWENG-5a)

- Property: heighten delta equals `slotLevel − techLevel` for all valid assignments; delta is never negative.
- Property: back/forward navigation never produces a slot index < 0 or ≥ totalSlots.
- Property: for `wizard`/`ranger` models, all final slot assignments have tech level ≤ slot level and the tech ID is present in `KnownTechs`.
- Property: for `druid` model, all final slot assignments have tech level ≤ slot level and the tech ID is present in the grant pool.
- Property: catalog picker at level-up never adds a tech ID already present in `KnownTechs`.

### REQ-TC-27: Unit tests

- Casting model resolution: job override takes precedence over archetype default; archetype default used when job field absent; `none` when both absent.
- Level tab default is the highest M ≤ slotLevel with eligible options under the active model.
- Auto-assign fires when exactly one eligible tech exists; no prompt is shown.
- Rename: all existing spontaneous archetype tests pass unchanged with renamed field/repo/table.
- Trainer flow (`wizard`/`ranger`): tech added to both `KnownTechs` and `PreparedTechs`.
- Trainer flow (`druid`): tech added to `PreparedTechs` only; `KnownTechs` unmodified.
- `wizard` L1 extras: 2 techs added to `KnownTechs` beyond slot count after character creation.
- `ranger` L1: NO extras added beyond slot count after character creation.
- `druid` rest: option list built from grant pool, not `KnownTechs`.

# Tech Catalog (KnownTechs) + Slot Assignment Modal Design

## Goal

Introduce a per-character tech catalog (the "spellbook" equivalent from PF2E wizard rules), replace the current full-pool prepared slot selection with catalog-filtered selection, support heightened slot assignment, and enrich the rest rearrangement modal with level tabs, heightened indicators, and back/forward navigation.

## Architecture

The existing `SpontaneousTechs` infrastructure is renamed to `KnownTechs` and its semantics are unified: for prepared archetypes it is the catalog of techs available to prepare; for spontaneous archetypes it retains its existing meaning (techs the character can cast freely). No new DB table, repo, or session field shape is required — only a rename and population changes.

The rest rearrangement flow becomes catalog-driven: `RearrangePreparedTechs` reads from `sess.KnownTechs` instead of the grant pool. The `FeatureChoiceModal` gains level filter tabs, heightened badges, and Back/Forward navigation buttons.

## Tech Stack

- Go backend: session, storage/postgres, gameserver, technology_assignment
- TypeScript frontend: `FeatureChoiceModal.tsx`, `GameContext.tsx`
- Postgres: rename table `character_spontaneous_technologies` → `character_known_technologies`
- Proto: `ChoicePrompt` extended with `slotContext`; new `[heightened:N]` and `[back]`/`[forward]` sentinels in option strings

---

## Section 1: Data Model

### REQ-TC-1: Rename `SpontaneousTechs` → `KnownTechs`

The following renames MUST be applied consistently across the entire codebase:

| Before | After |
|--------|-------|
| `PlayerSession.SpontaneousTechs` | `PlayerSession.KnownTechs` |
| `character_spontaneous_technologies` (DB table) | `character_known_technologies` |
| `SpontaneousTechRepo` (interface + impl) | `KnownTechRepo` |
| `spontaneousTechRepo` (field names) | `knownTechRepo` |
| All local variables named `spontaneousTechs` | `knownTechs` |

### REQ-TC-2: No new DB table

The existing `character_known_technologies` table (post-rename) carries the full catalog for both prepared and spontaneous archetypes. Schema is unchanged: `(character_id, tech_level, tech_id)`.

### REQ-TC-3: Unified field semantics

- **Prepared archetypes** (Nerd, Naturalist, Drifter, Schemer, Zealot): `KnownTechs[level]` is the set of techs eligible to prepare in slots of that level. `PreparedTechs[level]` remains the current slot assignments.
- **Spontaneous archetypes** (Influencer): `KnownTechs[level]` retains its existing meaning — the full set of known techs at that level usable with shared use slots. No behavior change.
- **No-tech archetypes** (Aggressor, Criminal): `KnownTechs` is empty. No change.

---

## Section 2: Catalog Population

### REQ-TC-4: L1 catalog extras at character creation

After the player fills their L1 prepared slots during level-up, they MUST be prompted to pick 2 additional techs from the remaining L1 grant pool. These 2 techs are added to `KnownTechs[1]` only — they are NOT assigned to any `PreparedTechs` slot.

If fewer than 2 pool entries remain after slot picks, all remaining entries are added without prompting.

### REQ-TC-5: +2 catalog additions per level-up

At each level-up (level 2 and above), after all slot grants are processed, the player MUST be prompted to pick 2 techs to add to their catalog. These techs may be at any tech level for which the character has at least one prepared slot. They are added to `KnownTechs[level]` only — no slot assignment.

If the remaining pool at all eligible levels is exhausted (all entries already in `KnownTechs`), the picker is skipped silently.

This rule applies to prepared archetypes only. Spontaneous archetypes follow their existing known-tech progression.

### REQ-TC-6: Trainer populates KnownTechs

When a tech trainer teaches a tech (resolves a pending tech slot), the tech MUST be added to `KnownTechs[techLevel]` in addition to being assigned to `PreparedTechs[techLevel][slotIdx]`. The trainer flow is otherwise unchanged.

### REQ-TC-7: Level-up slot picks populate KnownTechs

When a prepared tech is assigned to a slot during any level-up flow (`fillFromPreparedPool`, `fillFromPreparedPoolWithSend`), the assigned tech ID MUST also be added to `KnownTechs[level]` if not already present.

---

## Section 3: Rest Rearrangement

### REQ-TC-8: Catalog-filtered pool

`RearrangePreparedTechs` MUST build the option list for a slot of level N from `sess.KnownTechs[M]` for all M where 1 ≤ M ≤ N, rather than from the grant pool definition. The grant pool is no longer consulted during rest rearrangement.

### REQ-TC-9: Heightened assignment

A tech of level M placed in a slot of level N (M < N) is considered heightened by N−M. The server MUST embed the heighten delta in the option string using the sentinel `[heightened:N]` (e.g., `[heightened:2]`) appended after the tech's option text. The frontend strips and renders this as a badge.

### REQ-TC-10: Default to highest known level

For a slot of level N, if `KnownTechs[N]` is empty, the active level tab defaults to the highest M < N where `KnownTechs[M]` is non-empty. If no level has catalog entries, the slot is skipped and a warning is logged.

### REQ-TC-11: Back/forward navigation

- A `[back]` sentinel option MUST be prepended to every slot's option list except the first slot of the first level.
- A `[forward]` sentinel option MUST be appended to every slot's option list except the last slot of the last level, where it is replaced by a `[confirm]` sentinel option.
- The server tracks all in-progress slot assignments in memory during the rearrangement session.
- Assignments are written to `PreparedTechs` and the DB only after the player confirms the final slot.
- Selecting Back decrements the current slot index and re-prompts with the previous slot's options, pre-selecting the in-progress assignment.

### REQ-TC-12: Single catalog entry auto-assign

If `KnownTechs` contains exactly one eligible tech for a given slot (after accounting for all levels ≤ slot level), the slot is auto-assigned without prompting. A send message is emitted: `"Level N, {SlotNoun} M of TOTAL (fixed): {techID}"`.

---

## Section 4: Frontend Modal

### REQ-TC-13: slotContext in ChoicePrompt

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

### REQ-TC-14: Level filter tabs

The `FeatureChoiceModal` MUST parse the `(Lv N)` substring from each option string to extract available tech levels. It MUST render a row of level filter tab buttons (e.g., `[L1] [L2] [L3]`). Clicking a tab filters the displayed option list client-side. The active tab defaults to `slotContext.slotLevel`; if that level has no options, the highest available level is active. No server round-trip occurs on tab change.

### REQ-TC-15: Heightened badge

Option strings containing a `[heightened:N]` sentinel MUST have that sentinel stripped from displayed text and replaced with a visible badge: `+N` rendered in a distinct color (gold `#e0c060`) adjacent to the tech name.

### REQ-TC-16: Back/Forward navigation buttons

- Option strings equal to the `[back]` sentinel are NOT rendered as numbered choices. Instead, a `← Back` button is rendered at the bottom-left of the modal.
- Option strings equal to the `[forward]` sentinel are NOT rendered as numbered choices. Instead, a `Next →` button is rendered at the bottom-right.
- On the final slot, `Next →` is replaced by a `Confirm` button.
- Clicking `← Back` sends the `[back]` sentinel value as the selection (the server handles navigation).
- Clicking `Next →` sends the `[forward]` sentinel value. Clicking `Confirm` (final slot only) sends the `[confirm]` sentinel value. The server treats `[confirm]` as the signal to write all in-progress assignments to the DB.

---

## Section 5: Edge Cases

### REQ-TC-17: Empty remaining pool at catalog picker

If the L1 pool has zero entries remaining after slot picks (all pool entries already in `KnownTechs`), the L1 catalog extras step (REQ-TC-4) is skipped. If fewer than 2 remain, all are added without prompting.

### REQ-TC-18: Level-up catalog exhausted

If all grant pool entries at all eligible levels are already in `KnownTechs` at the time of the +2 level-up catalog pick (REQ-TC-5), the picker is silently skipped.

### REQ-TC-19: Spontaneous archetype unchanged

`RearrangePreparedTechs` is never called for spontaneous archetypes (Influencer). The +2 per level-up catalog rule (REQ-TC-5) does NOT apply to spontaneous archetypes. Their existing level-up and known-tech flow is unchanged.

### REQ-TC-20: Rename backward compatibility

The DB migration renames `character_spontaneous_technologies` to `character_known_technologies`. All existing data is preserved. The migration MUST be idempotent.

---

## Testing Requirements

### REQ-TC-21: Property-based tests (SWENG-5a)

- Property: heighten delta equals `slotLevel − techLevel` for all valid assignments; delta is never negative.
- Property: back/forward navigation never produces a slot index < 0 or ≥ totalSlots.
- Property: all final slot assignments have tech level ≤ slot level and the tech ID is present in `KnownTechs`.
- Property: catalog picker at level-up never adds a tech ID already present in `KnownTechs`.

### REQ-TC-22: Unit tests

- Level tab default is the highest M ≤ slotLevel with non-empty `KnownTechs[M]`.
- Auto-assign fires when exactly one eligible tech exists; no prompt is shown.
- Rename: all existing spontaneous archetype tests pass unchanged with renamed field/repo/table.
- Trainer flow: tech is added to both `KnownTechs` and `PreparedTechs` after training.
- L1 extras: 2 techs are added to `KnownTechs` beyond slot count after character creation.

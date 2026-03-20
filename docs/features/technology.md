# Technology

Maps the PF2E magic system to Gunchete technology: traditions, prepared/spontaneous/innate techs, and effect resolution.

## Requirements

- [x] Technology instead of magic.
  - The P2FE system of magic needs ported into Gunchete and mapped to a combination of high technology, chemistry, and drug effects (there is no magic in Gunchete, only cyberpunk futurism).
  - For the remainder of the features specification Technology refers to the combined effects of technological devices, chemistry, and drugs.
  - [x] Technology data model — `TechnologyDef` type with `TechEffect` discriminated union; `Registry` with `Load`, `Get`, `All`, `ByTradition`, `ByTraditionAndLevel`, `ByUsageType`; seed content one per tradition; wired into `GameServiceServer`
  - [x] Traditions of magic -> Types of technology / Substances
    - Use PF2E Traits, mapping to Gunchete lore as required
    - [x] Arcane -> Technical
    - [x] Divine -> Fanatic Doctrine
    - [x] Occult -> Neural
    - [x] Primal -> Bio-Synthetic
  - [x] Archetype and Job define the technologies available for the player — `TechnologyGrants` on `Job` and `InnateTechnologies` on `Archetype`; assigned at character creation via `AssignTechnologies`; loaded at login via `LoadTechnologies`; repos wired into `GameServiceServer`
    - [x] Levelling up allows for additions and changes — hardwired grants auto-applied; prepared/spontaneous pool choices deferred to `PendingTechGrants`, resolved interactively at login or via `selecttech`
    - [x] Spellbook/memorization needs to be mapped to a lore-friendly analog that preserves the underlying mechanic
      - [x] Cantrips -> Hardwired Technologies (unlimited-use minor Technologies)
        - [x] Fixed list per Job, no player adjustment
        - [x] Level-up hardwired tech grants applied via `LevelUpTechnologies` for each level gained in ascending order (REQ-LUT7); admin `grant xp` path uses first-option auto-assign (no interactive prompt available)
      - [x] Prepared Spells -> Preparation (loading ammunition, tuning an energy weapon, mixing a drug cocktail)
        - [x] Fixed list of Technologies per job level, increases with level (higher level Technology slots and higher Job level) — Phase 1: slot progression on archetypes, pool entries on jobs; Phase 2 will expand tech library
        - [x] Prepared tech slot expending — each prepared slot is one use; `use <tech>` expends the first matching non-expended slot; `rest` restores all slots; expended state persisted in DB
        - [x] Prepared tech effect resolution — activating a tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution; REQ-TER1–22)
        - [x] Can be rearranged when resting — `rest` command; `RearrangePreparedTechs` aggregates creation + level-up grants, clears and re-fills slots interactively
        - [x] Level-up technology selection — player selects new prepared/spontaneous techs interactively at next login or via `selecttech`; auto-assigned grants notify in-console; persisted in `character_pending_tech_levels`
      - [x] Spontaneous Spells ->  Technologies with a preset daily usage limit
        - [x] Fixed list of Technologies per job level — job YAML pool entries; archetype slot progression; merged via `MergeGrants` (Phase 1)
        - [x] Fixed number usages for each Technology level (resets with long rest) — `character_spontaneous_use_pools` table; `SpontaneousUsePoolRepo`; `use` decrements, `rest` restores (Sub-project A)
        - [x] Player gets to choose which new Technologies are learned with levelling up. (Sub-project B) — `neural_static` + `synaptic_surge` added; Influencer grants known tech at levels 3 and 5 with 3-tech pool; deferral + selection verified end-to-end (REQ-SSL1-4)
      - [x] Spontaneous tech effect resolution — activating a spontaneous tech applies its game effect (Sub-project: Tech Effect Resolution; REQ-TER1–22)
      - [ ] Heightened Spells -> Amped Technology — expend a higher-level spontaneous use slot to activate a tech at amped power level; uses `AmpedEffects` defined per tech (Sub-project: Amped Technology, depends on Tech Effect Resolution)
      - [x] Innate Technologies — region-based innate tech grants; per-tech daily uses; restore on rest; character sheet display (REQ-INN1–INN9, REQ-CONTENT1–2)
        - [x] Innate tech effect resolution — activating an innate tech applies its game effect (damage, condition, etc.) (Sub-project: Tech Effect Resolution; REQ-TER1–22)
        - [ ] `chrome_reflex` reaction trigger — integrate with Reactions system so it fires as a reaction rather than via `use` command; requires Reactions system
        - [x] Passive innate tech mechanics — `seismic_sense` (always-on tremorsense) applies passively without `use` command; `moisture_reclaim` cantrip refactor is a separate sub-project
    - [x] Spell import from PF2E with translation into Gunchete
      - [x] Populate Archetype and Job yaml with options
- [ ] refactor to use `wire` for dependency injection

# Non-Combat NPCs

Nine non-combat NPC types with type-specific config, HTN personality system, and Rustbucket Ridge named instances. See `docs/superpowers/specs/2026-03-20-non-combat-npcs-design.md` for the full design spec. The `fixer` type and guard bribe fields are defined in `docs/superpowers/specs/2026-03-20-wanted-clearing-design.md`.

## Requirements

### Foundation (sub-project 1) — complete

- [x] REQ-NPC-1: NPCs with no `npc_type` MUST default to `"combat"` at load time.
- [x] REQ-NPC-2: The type-specific config sub-struct for the declared `npc_type` MUST be non-nil at load time; mismatch MUST be a fatal load error. For `npc_type: "crafter"`, an explicit `crafter: {}` YAML block MUST be present.
- [ ] REQ-NPC-2a: `Template.Validate()` MUST verify all referenced skill IDs exist in the skill registry. *(Deferred to sub-project 3: Service NPCs, where skills are first used)*
- [x] REQ-NPC-3: Non-combat NPCs MUST NOT be added to the combat initiative order (satisfied structurally — only the attacked NPC joins combat; guard engage behavior wired in sub-project 4).
- [x] REQ-NPC-4: Non-combat NPCs MUST NOT be valid attack targets (except engaging guards — enabled in sub-project 4).
- [x] REQ-NPC-13: `ReplenishConfig` MUST satisfy `0 < MinHours <= MaxHours <= 24`; fatal load error on violation.
- [x] REQ-NPC-18: `QuestGiverConfig.PlaceholderDialog` MUST contain at least one entry; fatal load error otherwise.

### Remaining requirements

- [ ] Base data model
  - [ ] HTN personality system (cowardly/brave/neutral/opportunistic presets)
  - [ ] Flee/cower behavior on combat start
- [ ] Merchant
  - REQ-NPC-5: `negotiate` MUST only be usable once per merchant room visit.
  - REQ-NPC-5a: Negotiate price modifier MUST be stored on player room session state, cleared on room exit.
  - REQ-NPC-5b: WantedLevel 1 surcharge applied before negotiate modifier; not applied to negotiate roll.
  - REQ-NPC-12: Merchant runtime state MUST be persisted and restored on restart; YAML values apply only at first initialization.
  - [ ] `browse`, `buy`, `sell`, `negotiate` commands
  - [ ] Named NPCs: Sergeant Mack (weapons, Last Stand Lodge), Slick Sally, Whiskey Joe, Old Rusty, Herb (consumables)
- [ ] Guard
  - REQ-NPC-6: On Safe room second violation, all guards present MUST enter initiative and target the aggressor.
  - REQ-NPC-7: Guards MUST check WantedLevel on room entry and on WantedLevel change events.
  - REQ-WC-2b: `GuardConfig.MaxBribeWantedLevel` MUST be in range 1–4 when `Bribeable` is true; fatal load error otherwise.
  - [ ] WantedThreshold-configurable aggression table
  - [ ] `GuardConfig.Bribeable bool` field (default: false)
  - [ ] `GuardConfig.MaxBribeWantedLevel int` field (default: 2)
  - [ ] `GuardConfig.BaseCosts map[int]int` field for bribeable guards (all keys 1–4, positive values)
  - [ ] Named NPC: one lore-appropriate guard in a Safe room in Rustbucket Ridge
- [ ] Healer
  - REQ-NPC-16: `CapacityUsed` MUST reset to 0 on daily tick; restored from DB on restart.
  - [ ] `heal` and `heal <amount>` commands
  - [ ] Named NPCs: Clutch (The Tinker's Den), Tina Wires (Junker's Dream)
- [ ] Quest Giver
  - [ ] `talk <npc>` command with placeholder dialog
  - [ ] Named NPC: Gail "Grinder" Graves (Scrapshack 23)
- [ ] Hireling
  - REQ-NPC-8: Hirelings MUST be combat allies; MUST NOT be targetable by player's own attacks.
  - REQ-NPC-15: Hireling binding MUST be atomic check-and-set.
  - [ ] `hire <npc>` and `dismiss` commands
  - [ ] Zone follow tracking with `MaxFollowZones` limit
  - [ ] Named NPC: one lore-appropriate hireling in Rustbucket Ridge
- [ ] Banker
  - REQ-NPC-14: Deposit and withdrawal MUST use `CurrentRate` at command execution time.
  - [ ] Global stash (`StashBalance` on player character)
  - [ ] `deposit`, `withdraw`, `balance` commands
  - [ ] Named NPC: one lore-appropriate banker in a Safe room in Rustbucket Ridge
- [ ] Job Trainer
  - REQ-NPC-9: Players MUST have exactly one active job after their first job is trained.
  - REQ-NPC-10: Active job MUST earn XP; inactive jobs MUST NOT.
  - REQ-NPC-11: Inactive jobs MUST still provide feats and proficiencies.
  - REQ-NPC-17: `setjob` MUST be available from any room.
  - [ ] `train <job>`, `jobs`, `setjob <job>` commands
  - [ ] JobPrerequisites (level, job level, attributes, skill ranks, required jobs)
  - [ ] Named NPC: one lore-appropriate job trainer in Rustbucket Ridge
- [ ] Crafter (stub)
  - [ ] `npc_type: "crafter"` declared; full behavior deferred to `crafting` feature
- [ ] Fixer (from `wanted-clearing` feature)
  - REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; fatal load error otherwise.
  - REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; fatal load error otherwise.
  - REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain all keys 1–4 with positive values; fatal load error otherwise.
  - REQ-WC-3: Fixers MUST default to `flee` on combat start; MUST NOT enter initiative order.
  - [ ] `Fixer *FixerConfig` field added to NPC Template struct
  - [ ] `fixer → flee` added to personality default table
  - [ ] `Template.Validate()` updated to recognize `"fixer"` type
  - [ ] Named NPC: one lore-appropriate fixer in Rustbucket Ridge

## Non-Combat NPCs — All Zones

- [ ] Every zone MUST have a lore-appropriate instance of each non-combat NPC type in a Safe room (`non-combat-npcs-all-zones` feature).

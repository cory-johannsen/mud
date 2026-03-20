# Non-Combat NPCs

Eight non-combat NPC types with type-specific config, HTN personality system, and Rustbucket Ridge named instances. See `docs/superpowers/specs/2026-03-20-non-combat-npcs-design.md` for the full design spec.

## Requirements

- [ ] Base data model
  - REQ-NPC-1: NPCs with no `npc_type` MUST default to `"combat"` at load time.
  - REQ-NPC-2: Type-specific config sub-struct MUST be non-nil at load time; mismatch MUST be fatal load error.
  - REQ-NPC-2a: `Template.Validate()` MUST verify all referenced skill IDs exist in the skill registry.
  - REQ-NPC-3: Non-combat NPCs MUST NOT be added to the combat initiative order (guards excepted when engaging per Section 3).
  - REQ-NPC-4: Non-combat NPCs MUST NOT be valid attack targets (guards excepted when engaging per Section 3).
  - [ ] HTN personality system (cowardly/brave/neutral/opportunistic presets)
  - [ ] Flee/cower behavior on combat start
- [ ] Merchant
  - REQ-NPC-5: `negotiate` MUST only be usable once per merchant room visit.
  - REQ-NPC-5a: Negotiate price modifier MUST be stored on player room session state, cleared on room exit.
  - REQ-NPC-5b: WantedLevel 1 surcharge applied before negotiate modifier; not applied to negotiate roll.
  - REQ-NPC-12: Merchant runtime state MUST be persisted and restored on restart; YAML values apply only at first initialization.
  - REQ-NPC-13: `ReplenishConfig` MUST satisfy `0 < MinHours <= MaxHours <= 24`.
  - [ ] `browse`, `buy`, `sell`, `negotiate` commands
  - [ ] Named NPCs: Sergeant Mack (weapons, Last Stand Lodge), Slick Sally, Whiskey Joe, Old Rusty, Herb (consumables)
- [ ] Guard
  - REQ-NPC-6: On Safe room second violation, all guards present MUST enter initiative and target the aggressor.
  - REQ-NPC-7: Guards MUST check WantedLevel on room entry and on WantedLevel change events.
  - [ ] WantedThreshold-configurable aggression table
  - [ ] Named NPC: one lore-appropriate guard in a Safe room in Rustbucket Ridge
- [ ] Healer
  - REQ-NPC-16: `CapacityUsed` MUST reset to 0 on daily tick; restored from DB on restart.
  - [ ] `heal` and `heal <amount>` commands
  - [ ] Named NPCs: Clutch (The Tinker's Den), Tina Wires (Junker's Dream)
- [ ] Quest Giver
  - REQ-NPC-18: `PlaceholderDialog` MUST contain at least one entry.
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

## Non-Combat NPCs — All Zones

- [ ] Every zone MUST have a lore-appropriate instance of each non-combat NPC type in a Safe room (`non-combat-npcs-all-zones` feature).

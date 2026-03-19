# Populate Archetype and Job Technology Pools Design

**Date:** 2026-03-18
**Sub-project:** PF2E Import — Technology Pool Population

---

## Goal

Wire the imported PF2E level-1 technology library into the archetype and job YAML files so that players in tech-enabled archetypes have meaningful pools to choose from when filling prepared/spontaneous slots.

---

## Context

487 level-1 technology files now exist across 4 traditions. The archetype YAML files define slot progression (`slots_by_level`, `known_by_level`, `uses_by_level`) but carry no `pool` entries. The job YAML files carry small placeholder pools (1–3 entries from the old handcrafted set). `MergeGrants` already unions archetype pool + job pool at character creation — pool entries from both are concatenated, so no code changes are required.

Pool entry IDs are matched against the `id:` field inside each technology YAML file (not the filename). Some tech IDs have no tradition suffix (e.g., `soothe`, `divine_lance`, `malediction`) — these are single-tradition spells whose files live in the appropriate tradition directory but carry an unsuffixed ID. This is correct and expected.

---

## Archetype → Tradition Mapping

| Archetype  | Tradition        | Grant Type   |
|------------|------------------|--------------|
| Influencer | neural           | spontaneous  |
| Schemer    | neural           | prepared     |
| Naturalist | bio_synthetic    | prepared     |
| Drifter    | bio_synthetic    | prepared     |
| Zealot     | fanatic_doctrine | prepared     |
| Nerd       | technical        | prepared     |
| Aggressor  | none             | —            |
| Criminal   | none             | —            |

---

## Design

### REQ-POP1
Each tech-enabled archetype YAML MUST gain a curated `pool` of ~20–25 level-1 entries under its `technology_grants` block. For prepared archetypes, the pool MUST be added as a sibling key under `technology_grants.prepared`. For the Influencer (spontaneous), the pool MUST be added as a sibling key under `technology_grants.spontaneous`.

### REQ-POP2
The archetype pool MUST contain broadly useful techs that fit any job within that archetype. It MUST cover four categories: offense (attack/save-based damage), control (debuffs/movement restriction), defense/support (healing, armor, buffs), and utility (detection, mobility, information).

### REQ-POP3
Each of the 55 tech-enabled job YAMLs MUST replace its placeholder pool entries with 3–5 flavor-specific techs that match that job's thematic identity, supplementing the archetype pool. Job pool entries MUST NOT duplicate entries already in the archetype pool, since duplicates yield redundant player choices without adding variety.

### REQ-POP4
Jobs in Aggressor and Criminal archetypes have no tech tradition and MUST NOT receive technology pool entries.

### REQ-POP5
`go test ./internal/game/ruleset/...` MUST pass after all edits. This validates that for each tech-enabled job, `MergeGrants(archetype, job).Validate()` succeeds (pool+fixed entries ≥ slots at each level). Note: slot counts come from archetype `slots_by_level`; jobs that define no `slots_by_level` of their own rely entirely on the archetype for slot count. The archetype pool alone provides sufficient entries to satisfy `Validate()` for all jobs.

---

## Archetype Pool Definitions

### neural (Influencer, Schemer)

Pool is added under `technology_grants.spontaneous` for Influencer and `technology_grants.prepared` for Schemer. Example for Influencer:

```yaml
# Before
technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 2

# After
technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 2
    pool:
      - { id: fear_neural, level: 1 }
      # ... (full list below)
```

Full neural pool (21 entries):

```yaml
pool:
  # Offense
  - { id: fear_neural, level: 1 }
  - { id: daze_neural, level: 1 }
  - { id: haunting_hymn_neural, level: 1 }
  - { id: grim_tendrils_neural, level: 1 }
  - { id: telekinetic_projectile_neural, level: 1 }
  # Control
  - { id: sleep_neural, level: 1 }
  - { id: charm_neural, level: 1 }
  - { id: command_neural, level: 1 }
  - { id: enfeeble_neural, level: 1 }
  - { id: lose_the_path_neural, level: 1 }
  - { id: void_warp_neural, level: 1 }
  # Illusion/Stealth
  - { id: illusory_disguise_neural, level: 1 }
  - { id: illusory_object_neural, level: 1 }
  - { id: ghost_sound_neural, level: 1 }
  - { id: penumbral_shroud_neural, level: 1 }
  # Support/Utility
  - { id: soothe, level: 1 }
  - { id: sanctuary_neural, level: 1 }
  - { id: shield_neural, level: 1 }
  - { id: mindlink_neural, level: 1 }
  - { id: detect_magic_neural, level: 1 }
  - { id: read_aura_neural, level: 1 }
```

### bio_synthetic (Naturalist, Drifter)

Pool added under `technology_grants.prepared.pool` (19 entries):

```yaml
pool:
  # Offense
  - { id: electric_arc_bio_synthetic, level: 1 }
  - { id: acid_splash_bio_synthetic, level: 1 }
  - { id: breathe_fire_bio_synthetic, level: 1 }
  - { id: gouging_claw_bio_synthetic, level: 1 }
  - { id: thunderstrike_bio_synthetic, level: 1 }
  # Control
  - { id: tangle_vine_bio_synthetic, level: 1 }
  - { id: gust_of_wind_bio_synthetic, level: 1 }
  - { id: leaden_steps_bio_synthetic, level: 1 }
  - { id: grease_bio_synthetic, level: 1 }
  - { id: bramble_bush_bio_synthetic, level: 1 }
  # Healing/Defense
  - { id: heal_bio_synthetic, level: 1 }
  - { id: stabilize_bio_synthetic, level: 1 }
  - { id: runic_weapon_bio_synthetic, level: 1 }
  - { id: shielded_arm_bio_synthetic, level: 1 }
  # Utility
  - { id: detect_poison_bio_synthetic, level: 1 }
  - { id: create_water_bio_synthetic, level: 1 }
  - { id: jump_bio_synthetic, level: 1 }
  - { id: ant_haul_bio_synthetic, level: 1 }
  - { id: fleet_step_bio_synthetic, level: 1 }
```

### fanatic_doctrine (Zealot)

Pool added under `technology_grants.prepared.pool` (20 entries):

```yaml
pool:
  # Offense
  - { id: divine_lance, level: 1 }
  - { id: harm, level: 1 }
  - { id: haunting_hymn_fanatic_doctrine, level: 1 }
  - { id: ancient_dust_fanatic_doctrine, level: 1 }
  - { id: vitality_lash_fanatic_doctrine, level: 1 }
  # Control
  - { id: command_fanatic_doctrine, level: 1 }
  - { id: fear_fanatic_doctrine, level: 1 }
  - { id: daze_fanatic_doctrine, level: 1 }
  - { id: malediction, level: 1 }
  - { id: enfeeble_fanatic_doctrine, level: 1 }
  # Healing/Blessing
  - { id: heal_fanatic_doctrine, level: 1 }
  - { id: bless_fanatic_doctrine, level: 1 }
  - { id: benediction, level: 1 }
  - { id: spirit_link_fanatic_doctrine, level: 1 }
  - { id: infuse_vitality, level: 1 }
  # Utility
  - { id: detect_alignment_fanatic_doctrine, level: 1 }
  - { id: detect_magic_fanatic_doctrine, level: 1 }
  - { id: light_fanatic_doctrine, level: 1 }
  - { id: guidance_fanatic_doctrine, level: 1 }
  - { id: sanctuary_fanatic_doctrine, level: 1 }
```

### technical (Nerd)

Pool added under `technology_grants.prepared.pool` (20 entries). Note: `detect_magic_technical` does not exist as a file; `anticipate_peril_technical` (Threat Prediction Algorithm) serves the detection/utility role instead.

```yaml
pool:
  # Offense
  - { id: force_barrage_technical, level: 1 }
  - { id: scorching_blast_technical, level: 1 }
  - { id: shocking_grasp_technical, level: 1 }
  - { id: thunderstrike_technical, level: 1 }
  - { id: snowball_technical, level: 1 }
  # Control
  - { id: sleep_technical, level: 1 }
  - { id: grease_technical, level: 1 }
  - { id: leaden_steps_technical, level: 1 }
  - { id: enfeeble_technical, level: 1 }
  - { id: gravitational_pull_technical, level: 1 }
  # Defense/Augment
  - { id: mystic_armor_technical, level: 1 }
  - { id: runic_weapon_technical, level: 1 }
  - { id: shielded_arm_technical, level: 1 }
  - { id: mending_technical, level: 1 }
  # Utility
  - { id: anticipate_peril_technical, level: 1 }
  - { id: illusory_disguise_technical, level: 1 }
  - { id: invisible_item_technical, level: 1 }
  - { id: lock_technical, level: 1 }
  - { id: alarm_technical, level: 1 }
  - { id: pocket_library_technical, level: 1 }
```

---

## Job Pool Strategy

Each of the 55 tech-enabled jobs replaces its placeholder pool with 3–5 techs by thematic fit. Job entries MUST NOT duplicate the archetype pool. Examples:

| Job | Tradition | Flavor Additions |
|-----|-----------|-----------------|
| Anarchist | neural | `concordant_choir_neural`, `draw_ire_neural`, `infectious_enthusiasm_neural` |
| Entertainer | neural | `musical_accompaniment_neural`, `figment_neural`, `overselling_flourish_neural` |
| Medic | fanatic_doctrine | `rousing_splash_fanatic_doctrine`, `stabilize_fanatic_doctrine`, `necromancers_generosity_fanatic_doctrine` |
| Street Preacher | fanatic_doctrine | `battle_fervor`, `torturous_trauma_fanatic_doctrine`, `cycle_of_retribution_fanatic_doctrine` |
| Engineer | technical | `forge_technical`, `conductive_weapon_technical`, `animate_rope_technical` |
| Grease Monkey | technical | `fold_metal`, `temporary_tool`, `endure_technical` |
| Tracker | bio_synthetic | `breadcrumbs_bio_synthetic`, `vanishing_tracks`, `pest_form_bio_synthetic` |
| Exterminator | bio_synthetic | `noxious_vapors_bio_synthetic`, `goblin_pox_bio_synthetic`, `puff_of_poison_bio_synthetic` |

Full per-job flavor additions are determined during implementation (one task per archetype group).

---

## Tech-Enabled Jobs (55 total)

Jobs that MUST receive updated pool entries:

**neural** (Influencer + Schemer): anarchist, antifa, bureaucrat, dealer, entertainer, exotic_dancer, extortionist, grifter, illusionist, karen, libertarian, maker, mall_ninja, narcomancer, politician, salesman, schmoozer, shit_stirrer

**bio_synthetic** (Naturalist + Drifter): bagman, cop, driver, exterminator, fallen_trustafarian, free_spirit, freegan, hippie, hobo, laborer, pilot, pirate, psychopath, rancher, scout, stalker, tracker, warden

**fanatic_doctrine** (Zealot): believer, cult_leader, follower, guard, hired_help, medic, pastor, street_preacher, trainee, vigilante

**technical** (Nerd): cooker, detective, engineer, grease_monkey, hoarder, journalist, narc, natural_mystic, specialist

---

## Files Changed

**Archetypes (6):**
- `content/archetypes/influencer.yaml`
- `content/archetypes/schemer.yaml`
- `content/archetypes/naturalist.yaml`
- `content/archetypes/drifter.yaml`
- `content/archetypes/zealot.yaml`
- `content/archetypes/nerd.yaml`

**Jobs (55):** all jobs listed in the Tech-Enabled Jobs section above.

No Go code changes. No schema changes.

---

## Testing

- Run `go test ./internal/game/ruleset/...` — `TestAllTechJobsLoadAndMergeValid` (which calls `MergeGrants(archetype, job).Validate()` for every tech-enabled job) MUST pass
- Manual spot-check: create a character with each tech-enabled archetype, verify the pool at character creation contains the expected options

---

## Non-Goals

- Populating level 2–10 pools (future sub-project)
- Adding pools to Aggressor or Criminal archetypes
- Modifying slot counts or level-up grants

# Populate Archetype and Job Technology Pools Design

**Date:** 2026-03-18
**Sub-project:** PF2E Import — Technology Pool Population

---

## Goal

Wire the imported PF2E level-1 technology library into the archetype and job YAML files so that players in tech-enabled archetypes have meaningful pools to choose from when filling prepared/spontaneous slots.

---

## Context

487 level-1 technology files now exist across 4 traditions. The archetype YAML files define slot progression (`slots_by_level`, `known_by_level`, `uses_by_level`) but carry no `pool` entries. The job YAML files carry small placeholder pools (1–3 entries from the old handcrafted set). `MergeGrants` already unions archetype pool + job pool at character creation, so no code changes are required.

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
Each tech-enabled archetype YAML MUST gain a curated `pool` of ~20–25 level-1 entries under its `technology_grants` block. The pool MUST be placed at the same grant type (prepared or spontaneous) that the archetype already uses for slots.

### REQ-POP2
The archetype pool MUST contain broadly useful techs that fit any job within that archetype. It MUST cover four categories: offense (attack/save-based damage), control (debuffs/movement restriction), defense/support (healing, armor, buffs), and utility (detection, mobility, information).

### REQ-POP3
Each job YAML that belongs to a tech-enabled archetype MUST replace its placeholder pool entries with 3–5 flavor-specific techs that match that job's thematic identity, supplementing the archetype pool.

### REQ-POP4
Jobs in Aggressor and Criminal archetypes have no tech tradition and MUST NOT receive technology pool entries.

### REQ-POP5
After editing, `TechnologyGrants.Validate()` MUST pass for all archetypes and jobs (pool+fixed entries ≥ slots at each level). The existing test suite MUST pass with no modifications.

---

## Archetype Pool Definitions

### neural (Influencer, Schemer)

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
  - { id: detect_magic_technical, level: 1 }
  - { id: illusory_disguise_technical, level: 1 }
  - { id: invisible_item_technical, level: 1 }
  - { id: lock_technical, level: 1 }
  - { id: alarm_technical, level: 1 }
  - { id: pocket_library_technical, level: 1 }
```

---

## Job Pool Strategy

Each job replaces its current placeholder with 3–5 techs chosen by thematic fit. Examples:

| Job | Tradition | Flavor Additions |
|-----|-----------|-----------------|
| Anarchist | neural | `concordant_choir_neural`, `draw_ire_neural`, `infectious_enthusiasm_neural` |
| Entertainer | neural | `ghost_sound_neural`, `charm_neural`, `figment_neural` |
| Medic | fanatic_doctrine | `heal_fanatic_doctrine`, `stabilize_fanatic_doctrine`, `infuse_vitality` |
| Street Preacher | fanatic_doctrine | `bless_fanatic_doctrine`, `command_fanatic_doctrine`, `haunting_hymn_fanatic_doctrine` |
| Engineer | technical | `forge_technical`, `mending_technical`, `conductive_weapon_technical` |
| Grease Monkey | technical | `mending_technical`, `runic_weapon_technical`, `fold_metal` |
| Tracker | bio_synthetic | `detect_poison_bio_synthetic`, `fleet_step_bio_synthetic`, `tangle_vine_bio_synthetic` |
| Exterminator | bio_synthetic | `noxious_vapors_bio_synthetic`, `goblin_pox_bio_synthetic`, `acid_splash_bio_synthetic` |

Full per-job flavor additions are defined during implementation (one task per archetype group).

---

## Files Changed

| File | Change |
|------|--------|
| `content/archetypes/influencer.yaml` | Add `pool` to spontaneous grants |
| `content/archetypes/schemer.yaml` | Add `pool` to prepared grants |
| `content/archetypes/naturalist.yaml` | Add `pool` to prepared grants |
| `content/archetypes/drifter.yaml` | Add `pool` to prepared grants |
| `content/archetypes/zealot.yaml` | Add `pool` to prepared grants |
| `content/archetypes/nerd.yaml` | Add `pool` to prepared grants |
| `content/jobs/*.yaml` (tech-enabled jobs only) | Replace placeholder pool with flavor-specific 3–5 entries |

No Go code changes. No schema changes.

---

## Testing

- REQ-POP5: run `go test ./internal/game/ruleset/...` — all `TechnologyGrants.Validate()` calls pass
- Manual spot-check: create a character with each tech-enabled archetype, verify pool contains expected options at character creation

---

## Non-Goals

- Populating level 2–10 pools (future sub-project)
- Adding pools to Aggressor or Criminal archetypes
- Modifying slot counts or level-up grants

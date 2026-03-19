# Populate Archetype and Job Technology Pools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add curated level-1 technology pools to all 6 tech-enabled archetype YAMLs and replace placeholder pools in all 55 tech-enabled job YAMLs with flavor-specific entries.

**Architecture:** Pure YAML edits — no Go code changes. Each archetype YAML gains a `pool:` block under its existing `technology_grants.prepared` (or `.spontaneous` for Influencer). Each job YAML replaces its 1–3 placeholder entries with 3–5 flavor-specific techs that do not duplicate the archetype pool. `MergeGrants` already unions both pools at runtime.

**Tech Stack:** YAML, Go test suite (`go test ./internal/game/ruleset/...`), `TestAllTechJobsLoadAndMergeValid` validates every merged grant.

---

## File Map

| File | Change |
|------|--------|
| `content/archetypes/influencer.yaml` | Add `pool:` under `technology_grants.spontaneous` |
| `content/archetypes/schemer.yaml` | Add `pool:` under `technology_grants.prepared` |
| `content/archetypes/naturalist.yaml` | Add `pool:` under `technology_grants.prepared` |
| `content/archetypes/drifter.yaml` | Add `pool:` under `technology_grants.prepared` |
| `content/archetypes/zealot.yaml` | Add `pool:` under `technology_grants.prepared` |
| `content/archetypes/nerd.yaml` | Add `pool:` under `technology_grants.prepared` |
| `content/jobs/anarchist.yaml` … (55 job files) | Replace placeholder pool with flavor-specific 3–5 entries |

---

## Task 1: Add pools to archetype YAMLs

**Files:**
- Modify: `content/archetypes/influencer.yaml`
- Modify: `content/archetypes/schemer.yaml`
- Modify: `content/archetypes/naturalist.yaml`
- Modify: `content/archetypes/drifter.yaml`
- Modify: `content/archetypes/zealot.yaml`
- Modify: `content/archetypes/nerd.yaml`

- [ ] **Step 1: Verify test baseline**

Run: `go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v`

Expected: PASS (existing job pools already satisfy existing slot counts)

- [ ] **Step 2: Add pool to `content/archetypes/influencer.yaml`**

The `influencer` archetype uses `spontaneous`. Add `pool:` as a sibling of `known_by_level` and `uses_by_level` inside `technology_grants.spontaneous`:

```yaml
technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 2
    pool:
      - { id: fear_neural, level: 1 }
      - { id: daze_neural, level: 1 }
      - { id: haunting_hymn_neural, level: 1 }
      - { id: grim_tendrils_neural, level: 1 }
      - { id: telekinetic_projectile_neural, level: 1 }
      - { id: sleep_neural, level: 1 }
      - { id: charm_neural, level: 1 }
      - { id: command_neural, level: 1 }
      - { id: enfeeble_neural, level: 1 }
      - { id: lose_the_path_neural, level: 1 }
      - { id: void_warp_neural, level: 1 }
      - { id: illusory_disguise_neural, level: 1 }
      - { id: illusory_object_neural, level: 1 }
      - { id: ghost_sound_neural, level: 1 }
      - { id: penumbral_shroud_neural, level: 1 }
      - { id: soothe, level: 1 }
      - { id: sanctuary_neural, level: 1 }
      - { id: shield_neural, level: 1 }
      - { id: mindlink_neural, level: 1 }
      - { id: detect_magic_neural, level: 1 }
      - { id: read_aura_neural, level: 1 }
```

- [ ] **Step 3: Add pool to `content/archetypes/schemer.yaml`**

Add `pool:` under `technology_grants.prepared` (after `slots_by_level`):

```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    pool:
      - { id: fear_neural, level: 1 }
      - { id: daze_neural, level: 1 }
      - { id: haunting_hymn_neural, level: 1 }
      - { id: grim_tendrils_neural, level: 1 }
      - { id: telekinetic_projectile_neural, level: 1 }
      - { id: sleep_neural, level: 1 }
      - { id: charm_neural, level: 1 }
      - { id: command_neural, level: 1 }
      - { id: enfeeble_neural, level: 1 }
      - { id: lose_the_path_neural, level: 1 }
      - { id: void_warp_neural, level: 1 }
      - { id: illusory_disguise_neural, level: 1 }
      - { id: illusory_object_neural, level: 1 }
      - { id: ghost_sound_neural, level: 1 }
      - { id: penumbral_shroud_neural, level: 1 }
      - { id: soothe, level: 1 }
      - { id: sanctuary_neural, level: 1 }
      - { id: shield_neural, level: 1 }
      - { id: mindlink_neural, level: 1 }
      - { id: detect_magic_neural, level: 1 }
      - { id: read_aura_neural, level: 1 }
```

- [ ] **Step 4: Add pool to `content/archetypes/naturalist.yaml`**

Add `pool:` under `technology_grants.prepared`:

```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    pool:
      - { id: electric_arc_bio_synthetic, level: 1 }
      - { id: acid_splash_bio_synthetic, level: 1 }
      - { id: breathe_fire_bio_synthetic, level: 1 }
      - { id: gouging_claw_bio_synthetic, level: 1 }
      - { id: thunderstrike_bio_synthetic, level: 1 }
      - { id: tangle_vine_bio_synthetic, level: 1 }
      - { id: gust_of_wind_bio_synthetic, level: 1 }
      - { id: leaden_steps_bio_synthetic, level: 1 }
      - { id: grease_bio_synthetic, level: 1 }
      - { id: bramble_bush_bio_synthetic, level: 1 }
      - { id: heal_bio_synthetic, level: 1 }
      - { id: stabilize_bio_synthetic, level: 1 }
      - { id: runic_weapon_bio_synthetic, level: 1 }
      - { id: shielded_arm_bio_synthetic, level: 1 }
      - { id: detect_poison_bio_synthetic, level: 1 }
      - { id: create_water_bio_synthetic, level: 1 }
      - { id: jump_bio_synthetic, level: 1 }
      - { id: ant_haul_bio_synthetic, level: 1 }
      - { id: fleet_step_bio_synthetic, level: 1 }
```

- [ ] **Step 5: Add pool to `content/archetypes/drifter.yaml`**

Add `pool:` under `technology_grants.prepared` (same bio_synthetic pool as naturalist):

```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 1
    pool:
      - { id: electric_arc_bio_synthetic, level: 1 }
      - { id: acid_splash_bio_synthetic, level: 1 }
      - { id: breathe_fire_bio_synthetic, level: 1 }
      - { id: gouging_claw_bio_synthetic, level: 1 }
      - { id: thunderstrike_bio_synthetic, level: 1 }
      - { id: tangle_vine_bio_synthetic, level: 1 }
      - { id: gust_of_wind_bio_synthetic, level: 1 }
      - { id: leaden_steps_bio_synthetic, level: 1 }
      - { id: grease_bio_synthetic, level: 1 }
      - { id: bramble_bush_bio_synthetic, level: 1 }
      - { id: heal_bio_synthetic, level: 1 }
      - { id: stabilize_bio_synthetic, level: 1 }
      - { id: runic_weapon_bio_synthetic, level: 1 }
      - { id: shielded_arm_bio_synthetic, level: 1 }
      - { id: detect_poison_bio_synthetic, level: 1 }
      - { id: create_water_bio_synthetic, level: 1 }
      - { id: jump_bio_synthetic, level: 1 }
      - { id: ant_haul_bio_synthetic, level: 1 }
      - { id: fleet_step_bio_synthetic, level: 1 }
```

- [ ] **Step 6: Add pool to `content/archetypes/zealot.yaml`**

Add `pool:` under `technology_grants.prepared`:

```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    pool:
      - { id: divine_lance, level: 1 }
      - { id: harm, level: 1 }
      - { id: haunting_hymn_fanatic_doctrine, level: 1 }
      - { id: ancient_dust_fanatic_doctrine, level: 1 }
      - { id: vitality_lash_fanatic_doctrine, level: 1 }
      - { id: command_fanatic_doctrine, level: 1 }
      - { id: fear_fanatic_doctrine, level: 1 }
      - { id: daze_fanatic_doctrine, level: 1 }
      - { id: malediction, level: 1 }
      - { id: enfeeble_fanatic_doctrine, level: 1 }
      - { id: heal_fanatic_doctrine, level: 1 }
      - { id: bless_fanatic_doctrine, level: 1 }
      - { id: benediction, level: 1 }
      - { id: spirit_link_fanatic_doctrine, level: 1 }
      - { id: infuse_vitality, level: 1 }
      - { id: detect_alignment_fanatic_doctrine, level: 1 }
      - { id: detect_magic_fanatic_doctrine, level: 1 }
      - { id: light_fanatic_doctrine, level: 1 }
      - { id: guidance_fanatic_doctrine, level: 1 }
      - { id: sanctuary_fanatic_doctrine, level: 1 }
```

- [ ] **Step 7: Add pool to `content/archetypes/nerd.yaml`**

Add `pool:` under `technology_grants.prepared`:

```yaml
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    pool:
      - { id: force_barrage_technical, level: 1 }
      - { id: scorching_blast_technical, level: 1 }
      - { id: shocking_grasp_technical, level: 1 }
      - { id: thunderstrike_technical, level: 1 }
      - { id: snowball_technical, level: 1 }
      - { id: sleep_technical, level: 1 }
      - { id: grease_technical, level: 1 }
      - { id: leaden_steps_technical, level: 1 }
      - { id: enfeeble_technical, level: 1 }
      - { id: gravitational_pull_technical, level: 1 }
      - { id: mystic_armor_technical, level: 1 }
      - { id: runic_weapon_technical, level: 1 }
      - { id: shielded_arm_technical, level: 1 }
      - { id: mending_technical, level: 1 }
      - { id: anticipate_peril_technical, level: 1 }
      - { id: illusory_disguise_technical, level: 1 }
      - { id: invisible_item_technical, level: 1 }
      - { id: lock_technical, level: 1 }
      - { id: alarm_technical, level: 1 }
      - { id: pocket_library_technical, level: 1 }
```

- [ ] **Step 8: Run tests**

Run: `go test ./internal/game/ruleset/... -v`

Expected: all tests PASS including `TestAllTechJobsLoadAndMergeValid`

- [ ] **Step 9: Commit**

```bash
git add content/archetypes/
git commit -m "feat: add curated level-1 pools to all tech-enabled archetypes"
```

---

## Task 2: Update neural job pools (Influencer + Schemer — 18 jobs)

**Files:** `content/jobs/anarchist.yaml`, `antifa.yaml`, `bureaucrat.yaml`, `entertainer.yaml`, `exotic_dancer.yaml`, `extortionist.yaml`, `karen.yaml`, `libertarian.yaml`, `politician.yaml`, `schmoozer.yaml`, `dealer.yaml`, `grifter.yaml`, `illusionist.yaml`, `maker.yaml`, `mall_ninja.yaml`, `narcomancer.yaml`, `salesman.yaml`, `shit_stirrer.yaml`

**Note:** All replacements use `spontaneous.pool` for Influencer jobs and `prepared.pool` for Schemer jobs. Do NOT include any tech ID that already appears in the neural archetype pool (listed in Task 1 Step 2).

- [ ] **Step 1: Replace pool in each Influencer job**

For each file, locate the `technology_grants` block and replace the entire `pool:` list with the flavor entries below. Preserve all other fields (`slots_by_level`, `known_by_level`, `uses_by_level`, `fixed`, etc.) unchanged.

**anarchist** (`content/jobs/anarchist.yaml`) — crowd agitation, provocation:
```yaml
    pool:
      - { id: concordant_choir_neural, level: 1 }
      - { id: draw_ire_neural, level: 1 }
      - { id: infectious_enthusiasm_neural, level: 1 }
```

**antifa** (`content/jobs/antifa.yaml`) — combat disruption, morale:
```yaml
    pool:
      - { id: force_barrage_neural, level: 1 }
      - { id: biting_words, level: 1 }
      - { id: concordant_choir_neural, level: 1 }
```

**bureaucrat** (`content/jobs/bureaucrat.yaml`) — social reading, communication:
```yaml
    pool:
      - { id: read_the_air_neural, level: 1 }
      - { id: message_neural, level: 1 }
      - { id: protection_neural, level: 1 }
```

**entertainer** (`content/jobs/entertainer.yaml`) — performance, illusion:
```yaml
    pool:
      - { id: musical_accompaniment_neural, level: 1 }
      - { id: figment_neural, level: 1 }
      - { id: overselling_flourish_neural, level: 1 }
```

**exotic_dancer** (`content/jobs/exotic_dancer.yaml`) — appearance, distraction:
```yaml
    pool:
      - { id: exchange_image_neural, level: 1 }
      - { id: glamorize_neural, level: 1 }
      - { id: fashionista_neural, level: 1 }
```

**extortionist** (`content/jobs/extortionist.yaml`) — threat, curses:
```yaml
    pool:
      - { id: phantom_pain, level: 1 }
      - { id: ill_omen, level: 1 }
      - { id: bane_neural, level: 1 }
```

**karen** (`content/jobs/karen.yaml`) — complaints, harassment:
```yaml
    pool:
      - { id: draw_ire_neural, level: 1 }
      - { id: bane_neural, level: 1 }
      - { id: overselling_flourish_neural, level: 1 }
```

**libertarian** (`content/jobs/libertarian.yaml`) — self-reliance, escape:
```yaml
    pool:
      - { id: protection_neural, level: 1 }
      - { id: warp_step_neural, level: 1 }
      - { id: sure_strike_neural, level: 1 }
```

**politician** (`content/jobs/politician.yaml`) — social intelligence, messaging:
```yaml
    pool:
      - { id: read_the_air_neural, level: 1 }
      - { id: message_rune_neural, level: 1 }
      - { id: musical_accompaniment_neural, level: 1 }
```

**schmoozer** (`content/jobs/schmoozer.yaml`) — charm, appearance:
```yaml
    pool:
      - { id: glamorize_neural, level: 1 }
      - { id: fashionista_neural, level: 1 }
      - { id: read_the_air_neural, level: 1 }
```

- [ ] **Step 2: Replace pool in each Schemer job**

**dealer** (`content/jobs/dealer.yaml`) — chemical control, pain:
```yaml
    pool:
      - { id: tame_neural, level: 1 }
      - { id: phantom_pain, level: 1 }
      - { id: agitate_neural, level: 1 }
```

**grifter** (`content/jobs/grifter.yaml`) — identity deception:
```yaml
    pool:
      - { id: exchange_image_neural, level: 1 }
      - { id: ventriloquism_neural, level: 1 }
      - { id: disguise_magic_neural, level: 1 }
```

**illusionist** (`content/jobs/illusionist.yaml`) — advanced illusions:
```yaml
    pool:
      - { id: figment_neural, level: 1 }
      - { id: item_facade_neural, level: 1 }
      - { id: phantom_pain, level: 1 }
```

**maker** (`content/jobs/maker.yaml`) — crafting, enhancement:
```yaml
    pool:
      - { id: runic_weapon_neural, level: 1 }
      - { id: runic_body_neural, level: 1 }
      - { id: carryall_neural, level: 1 }
```

**mall_ninja** (`content/jobs/mall_ninja.yaml`) — precise strikes, weapon buffs:
```yaml
    pool:
      - { id: sure_strike_neural, level: 1 }
      - { id: runic_weapon_neural, level: 1 }
      - { id: needle_darts_neural, level: 1 }
```

**narcomancer** (`content/jobs/narcomancer.yaml`) — chemical confusion, control:
```yaml
    pool:
      - { id: agitate_neural, level: 1 }
      - { id: tame_neural, level: 1 }
      - { id: befuddle_neural, level: 1 }
```

**salesman** (`content/jobs/salesman.yaml`) — persuasion, luck:
```yaml
    pool:
      - { id: read_the_air_neural, level: 1 }
      - { id: bless_neural, level: 1 }
      - { id: overselling_flourish_neural, level: 1 }
```

**shit_stirrer** (`content/jobs/shit_stirrer.yaml`) — provocation, chaos:
```yaml
    pool:
      - { id: agitate_neural, level: 1 }
      - { id: draw_ire_neural, level: 1 }
      - { id: dizzying_colors_neural, level: 1 }
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add content/jobs/anarchist.yaml content/jobs/antifa.yaml content/jobs/bureaucrat.yaml \
  content/jobs/entertainer.yaml content/jobs/exotic_dancer.yaml content/jobs/extortionist.yaml \
  content/jobs/karen.yaml content/jobs/libertarian.yaml content/jobs/politician.yaml \
  content/jobs/schmoozer.yaml content/jobs/dealer.yaml content/jobs/grifter.yaml \
  content/jobs/illusionist.yaml content/jobs/maker.yaml content/jobs/mall_ninja.yaml \
  content/jobs/narcomancer.yaml content/jobs/salesman.yaml content/jobs/shit_stirrer.yaml
git commit -m "feat: replace placeholder pools in neural jobs (influencer + schemer)"
```

---

## Task 3: Update bio_synthetic job pools (Naturalist + Drifter — 18 jobs)

**Files:** `content/jobs/exterminator.yaml`, `fallen_trustafarian.yaml`, `freegan.yaml`, `hippie.yaml`, `hobo.yaml`, `laborer.yaml`, `rancher.yaml`, `tracker.yaml`, `bagman.yaml`, `cop.yaml`, `driver.yaml`, `free_spirit.yaml`, `pilot.yaml`, `pirate.yaml`, `psychopath.yaml`, `scout.yaml`, `stalker.yaml`, `warden.yaml`

**Note:** Do NOT include any tech ID already in the bio_synthetic archetype pool (listed in Task 1 Step 4).

- [ ] **Step 1: Replace pool in each Naturalist job**

**exterminator** (`content/jobs/exterminator.yaml`) — poisons, toxins:
```yaml
    pool:
      - { id: noxious_vapors_bio_synthetic, level: 1 }
      - { id: goblin_pox_bio_synthetic, level: 1 }
      - { id: puff_of_poison_bio_synthetic, level: 1 }
```

**fallen_trustafarian** (`content/jobs/fallen_trustafarian.yaml`) — chaotic offense:
```yaml
    pool:
      - { id: camel_spit_bio_synthetic, level: 1 }
      - { id: chilling_spray_bio_synthetic, level: 1 }
      - { id: airburst_bio_synthetic, level: 1 }
```

**freegan** (`content/jobs/freegan.yaml`) — foraging, cultivation:
```yaml
    pool:
      - { id: foraging_friends, level: 1 }
      - { id: flourishing_flora_bio_synthetic, level: 1 }
      - { id: cleanse_cuisine_bio_synthetic, level: 1 }
```

**hippie** (`content/jobs/hippie.yaml`) — natural defense, animals:
```yaml
    pool:
      - { id: armor_of_thorn_and_claw, level: 1 }
      - { id: animal_allies, level: 1 }
      - { id: nettleskin, level: 1 }
```

**hobo** (`content/jobs/hobo.yaml`) — endurance, mobility:
```yaml
    pool:
      - { id: deep_breath_bio_synthetic, level: 1 }
      - { id: buoyant_bubbles_bio_synthetic, level: 1 }
      - { id: tailwind_bio_synthetic, level: 1 }
```

**laborer** (`content/jobs/laborer.yaml`) — brute force, terrain:
```yaml
    pool:
      - { id: weaken_earth_bio_synthetic, level: 1 }
      - { id: wooden_fists_bio_synthetic, level: 1 }
      - { id: shockwave_bio_synthetic, level: 1 }
```

**rancher** (`content/jobs/rancher.yaml`) — animal handling:
```yaml
    pool:
      - { id: tame_bio_synthetic, level: 1 }
      - { id: animal_allies, level: 1 }
      - { id: foraging_friends, level: 1 }
```

**tracker** (`content/jobs/tracker.yaml`) — stealth, navigation:
```yaml
    pool:
      - { id: breadcrumbs_bio_synthetic, level: 1 }
      - { id: vanishing_tracks, level: 1 }
      - { id: pest_form_bio_synthetic, level: 1 }
```

- [ ] **Step 2: Replace pool in each Drifter job**

**bagman** (`content/jobs/bagman.yaml`) — perimeter security, deterrence:
```yaml
    pool:
      - { id: alarm_bio_synthetic, level: 1 }
      - { id: camel_spit_bio_synthetic, level: 1 }
      - { id: noxious_vapors_bio_synthetic, level: 1 }
```

**cop** (`content/jobs/cop.yaml`) — crowd control, restraint:
```yaml
    pool:
      - { id: caustic_blast_bio_synthetic, level: 1 }
      - { id: chilling_spray_bio_synthetic, level: 1 }
      - { id: tether_bio_synthetic, level: 1 }
```

**driver** (`content/jobs/driver.yaml`) — speed, force:
```yaml
    pool:
      - { id: tailwind_bio_synthetic, level: 1 }
      - { id: hydraulic_push_bio_synthetic, level: 1 }
      - { id: aqueous_blast_bio_synthetic, level: 1 }
```

**free_spirit** (`content/jobs/free_spirit.yaml`) — movement, survival:
```yaml
    pool:
      - { id: buoyant_bubbles_bio_synthetic, level: 1 }
      - { id: gentle_landing_bio_synthetic, level: 1 }
      - { id: tailwind_bio_synthetic, level: 1 }
```

**pilot** (`content/jobs/pilot.yaml`) — air, pressure, weather:
```yaml
    pool:
      - { id: airburst_bio_synthetic, level: 1 }
      - { id: gale_blast_bio_synthetic, level: 1 }
      - { id: personal_rain_cloud_bio_synthetic, level: 1 }
```

**pirate** (`content/jobs/pirate.yaml`) — aquatic, boarding:
```yaml
    pool:
      - { id: briny_bolt_bio_synthetic, level: 1 }
      - { id: hydraulic_push_bio_synthetic, level: 1 }
      - { id: aqueous_blast_bio_synthetic, level: 1 }
```

**psychopath** (`content/jobs/psychopath.yaml`) — acid, toxins:
```yaml
    pool:
      - { id: acidic_burst_bio_synthetic, level: 1 }
      - { id: caustic_blast_bio_synthetic, level: 1 }
      - { id: puff_of_poison_bio_synthetic, level: 1 }
```

**scout** (`content/jobs/scout.yaml`) — detection, stealth:
```yaml
    pool:
      - { id: breadcrumbs_bio_synthetic, level: 1 }
      - { id: approximate_bio_synthetic, level: 1 }
      - { id: pest_form_bio_synthetic, level: 1 }
```

**stalker** (`content/jobs/stalker.yaml`) — concealment, surveillance:
```yaml
    pool:
      - { id: vanishing_tracks, level: 1 }
      - { id: breadcrumbs_bio_synthetic, level: 1 }
      - { id: alarm_bio_synthetic, level: 1 }
```

**warden** (`content/jobs/warden.yaml`) — restraint, terrain denial:
```yaml
    pool:
      - { id: dehydrate_bio_synthetic, level: 1 }
      - { id: interposing_earth_bio_synthetic, level: 1 }
      - { id: tether_bio_synthetic, level: 1 }
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add content/jobs/exterminator.yaml content/jobs/fallen_trustafarian.yaml \
  content/jobs/freegan.yaml content/jobs/hippie.yaml content/jobs/hobo.yaml \
  content/jobs/laborer.yaml content/jobs/rancher.yaml content/jobs/tracker.yaml \
  content/jobs/bagman.yaml content/jobs/cop.yaml content/jobs/driver.yaml \
  content/jobs/free_spirit.yaml content/jobs/pilot.yaml content/jobs/pirate.yaml \
  content/jobs/psychopath.yaml content/jobs/scout.yaml content/jobs/stalker.yaml \
  content/jobs/warden.yaml
git commit -m "feat: replace placeholder pools in bio_synthetic jobs (naturalist + drifter)"
```

---

## Task 4: Update fanatic_doctrine job pools (Zealot — 10 jobs)

**Files:** `content/jobs/believer.yaml`, `cult_leader.yaml`, `follower.yaml`, `guard.yaml`, `hired_help.yaml`, `medic.yaml`, `pastor.yaml`, `street_preacher.yaml`, `trainee.yaml`, `vigilante.yaml`

**Note:** Do NOT include any tech ID already in the fanatic_doctrine archetype pool (listed in Task 1 Step 6).

- [ ] **Step 1: Replace pool in each Zealot job**

**believer** (`content/jobs/believer.yaml`) — combat conviction:
```yaml
    pool:
      - { id: battle_fervor, level: 1 }
      - { id: bane_fanatic_doctrine, level: 1 }
      - { id: admonishing_ray_fanatic_doctrine, level: 1 }
```

**cult_leader** (`content/jobs/cult_leader.yaml`) — mass control, retribution:
```yaml
    pool:
      - { id: celestial_accord_fanatic_doctrine, level: 1 }
      - { id: torturous_trauma_fanatic_doctrine, level: 1 }
      - { id: cycle_of_retribution_fanatic_doctrine, level: 1 }
```

**follower** (`content/jobs/follower.yaml`) — basic doctrine, protection:
```yaml
    pool:
      - { id: bane_fanatic_doctrine, level: 1 }
      - { id: alarm_fanatic_doctrine, level: 1 }
      - { id: air_bubble_fanatic_doctrine, level: 1 }
```

**guard** (`content/jobs/guard.yaml`) — protection, deterrence:
```yaml
    pool:
      - { id: admonishing_ray_fanatic_doctrine, level: 1 }
      - { id: protect_companion_fanatic_doctrine, level: 1 }
      - { id: shielded_arm_fanatic_doctrine, level: 1 }
```

**hired_help** (`content/jobs/hired_help.yaml`) — weapon buffs, endurance:
```yaml
    pool:
      - { id: runic_weapon_fanatic_doctrine, level: 1 }
      - { id: runic_body_fanatic_doctrine, level: 1 }
      - { id: battle_fervor, level: 1 }
```

**medic** (`content/jobs/medic.yaml`) — field medicine, revival:
```yaml
    pool:
      - { id: rousing_splash_fanatic_doctrine, level: 1 }
      - { id: stabilize_fanatic_doctrine, level: 1 }
      - { id: necromancers_generosity_fanatic_doctrine, level: 1 }
```

**pastor** (`content/jobs/pastor.yaml`) — preaching, proclamation:
```yaml
    pool:
      - { id: concordant_choir_fanatic_doctrine, level: 1 }
      - { id: bullhorn_fanatic_doctrine, level: 1 }
      - { id: beseech_the_sphinx_fanatic_doctrine, level: 1 }
```

**street_preacher** (`content/jobs/street_preacher.yaml`) — street conviction, punishment:
```yaml
    pool:
      - { id: battle_fervor, level: 1 }
      - { id: torturous_trauma_fanatic_doctrine, level: 1 }
      - { id: cycle_of_retribution_fanatic_doctrine, level: 1 }
```

**trainee** (`content/jobs/trainee.yaml`) — basic combat doctrine:
```yaml
    pool:
      - { id: bane_fanatic_doctrine, level: 1 }
      - { id: admonishing_ray_fanatic_doctrine, level: 1 }
      - { id: runic_weapon_fanatic_doctrine, level: 1 }
```

**vigilante** (`content/jobs/vigilante.yaml`) — retribution, curses:
```yaml
    pool:
      - { id: admonishing_ray_fanatic_doctrine, level: 1 }
      - { id: curse_of_recoil_fanatic_doctrine, level: 1 }
      - { id: draw_moisture_fanatic_doctrine, level: 1 }
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add content/jobs/believer.yaml content/jobs/cult_leader.yaml content/jobs/follower.yaml \
  content/jobs/guard.yaml content/jobs/hired_help.yaml content/jobs/medic.yaml \
  content/jobs/pastor.yaml content/jobs/street_preacher.yaml content/jobs/trainee.yaml \
  content/jobs/vigilante.yaml
git commit -m "feat: replace placeholder pools in fanatic_doctrine jobs (zealot)"
```

---

## Task 5: Update technical job pools (Nerd — 9 jobs)

**Files:** `content/jobs/cooker.yaml`, `detective.yaml`, `engineer.yaml`, `grease_monkey.yaml`, `hoarder.yaml`, `journalist.yaml`, `narc.yaml`, `natural_mystic.yaml`, `specialist.yaml`

**Note:** Do NOT include any tech ID already in the technical archetype pool (listed in Task 1 Step 7).

- [ ] **Step 1: Replace pool in each Nerd job**

**cooker** (`content/jobs/cooker.yaml`) — chemical confusion, toxins:
```yaml
    pool:
      - { id: elysian_whimsy_technical, level: 1 }
      - { id: noxious_vapors_technical, level: 1 }
      - { id: goblin_pox_technical, level: 1 }
```

**detective** (`content/jobs/detective.yaml`) — surveillance, memory, tracking:
```yaml
    pool:
      - { id: seashell_of_stolen_sound_technical, level: 1 }
      - { id: mindlink_technical, level: 1 }
      - { id: dj_vu_technical, level: 1 }
```

**engineer** (`content/jobs/engineer.yaml`) — fabrication, weapons, tools:
```yaml
    pool:
      - { id: forge_technical, level: 1 }
      - { id: conductive_weapon_technical, level: 1 }
      - { id: animate_rope_technical, level: 1 }
```

**grease_monkey** (`content/jobs/grease_monkey.yaml`) — metalwork, improvised tools:
```yaml
    pool:
      - { id: fold_metal, level: 1 }
      - { id: temporary_tool, level: 1 }
      - { id: endure_technical, level: 1 }
```

**hoarder** (`content/jobs/hoarder.yaml`) — carrying capacity, storage:
```yaml
    pool:
      - { id: carryall_technical, level: 1 }
      - { id: pet_cache_technical, level: 1 }
      - { id: ant_haul_technical, level: 1 }
```

**journalist** (`content/jobs/journalist.yaml`) — recording, broadcasting, information:
```yaml
    pool:
      - { id: seashell_of_stolen_sound_technical, level: 1 }
      - { id: ventriloquism_technical, level: 1 }
      - { id: share_lore_technical, level: 1 }
```

**narc** (`content/jobs/narc.yaml`) — surveillance, restraint, signaling:
```yaml
    pool:
      - { id: dj_vu_technical, level: 1 }
      - { id: tether_technical, level: 1 }
      - { id: signal_skyrocket_technical, level: 1 }
```

**natural_mystic** (`content/jobs/natural_mystic.yaml`) — biology meets tech, summoning:
```yaml
    pool:
      - { id: summon_animal_technical, level: 1 }
      - { id: flourishing_flora_technical, level: 1 }
      - { id: weave_wood_technical, level: 1 }
```

**specialist** (`content/jobs/specialist.yaml`) — precision combat, weapon augment:
```yaml
    pool:
      - { id: sure_strike_technical, level: 1 }
      - { id: runic_body_technical, level: 1 }
      - { id: echoing_weapon_technical, level: 1 }
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/game/ruleset/... -run TestAllTechJobsLoadAndMergeValid -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add content/jobs/cooker.yaml content/jobs/detective.yaml content/jobs/engineer.yaml \
  content/jobs/grease_monkey.yaml content/jobs/hoarder.yaml content/jobs/journalist.yaml \
  content/jobs/narc.yaml content/jobs/natural_mystic.yaml content/jobs/specialist.yaml
git commit -m "feat: replace placeholder pools in technical jobs (nerd)"
```

---

## Task 6: Final validation

- [ ] **Step 1: Run full ruleset test suite**

Run: `go test ./internal/game/ruleset/... -v`

Expected: all tests PASS

- [ ] **Step 2: Run full project test suite**

Run: `go test ./... 2>&1 | tail -20`

Expected: all tests PASS, no failures

- [ ] **Step 3: Update feature doc**

In `content/technologies/technology.md` (or `docs/features/technology.md`), mark the checkbox for "Populate Archetype and Job yaml with options" as done:

Change:
```
      - [ ] Populate Archetype and Job yaml with options
```
To:
```
      - [x] Populate Archetype and Job yaml with options
```

File: `docs/features/technology.md`

- [ ] **Step 4: Final commit**

```bash
git add docs/features/technology.md
git commit -m "docs: mark populate archetype/job tech pools as complete"
```

# Actions Content Design

## Goal

Add active (and where missing, passive) class features to all archetypes and jobs so every player character has at least one activatable action.

## Architecture

Content lives in `content/class_features.yaml`. Each entry uses the extended `ClassFeature` schema: `id`, `name`, `active`, `shortcut`, `action_cost`, `contexts`, `effect`, and optional `activate_text`. New conditions referenced by effects are defined in `content/conditions.yaml`. No Go code changes are required.

## Tech Stack

YAML content files; Go YAML loader already handles the extended schema.

---

## Section 1: Archetype-Level Features

Six archetypes receive new features. Three archetypes (naturalist, schemer, zealot) also receive a passive feature because they had no features at all.

### criminal
- **Ghost** (`id: ghost`, `shortcut: ghost`, `active: true`, `action_cost: 1`, `contexts: [combat, exploration]`)
- Effect: `condition`, target `self`, condition_id `ghost_active`
- `ghost_active` condition: grants concealment bonus, lasts 1 minute

### drifter
- **Mark** (`id: mark`, `shortcut: mark`, `active: true`, `action_cost: 1`, `contexts: [combat]`)
- Effect: `condition`, target `other`, condition_id `marked_active`
- `marked_active` condition: target takes +1 damage from all sources

### nerd
- **Exploit** (`id: exploit`, `shortcut: exploit`, `active: true`, `action_cost: 1`, `contexts: [combat, exploration]`)
- Effect: `skill_check`, skill `Reasoning`, DC 15
- No new condition needed

### naturalist
- **Hardy** (`id: hardy`, `active: false`) — passive: +1 Fortitude, immune extreme temperature
- **Primal Surge** (`id: primal_surge`, `shortcut: primal`, `active: true`, `action_cost: 1`, `contexts: [combat]`)
- Effect: `condition`, target `self`, condition_id `primal_surge_active`
- `primal_surge_active` condition: +2 Strength checks, lasts until end of combat

### schemer
- **Smooth Operator** (`id: smooth_operator`, `active: false`) — passive: +2 Deception
- **Setup** (`id: setup`, `shortcut: setup`, `active: true`, `action_cost: 1`, `contexts: [combat, exploration]`)
- Effect: `condition`, target `self`, condition_id `setup_active`
- `setup_active` condition: next social check is made with advantage

### zealot
- **True Believer** (`id: true_believer`, `active: false`) — passive: +2 Will, immune demoralized
- **Lay Hands** (`id: lay_hands`, `shortcut: hands`, `active: true`, `action_cost: 2`, `contexts: [combat, exploration]`)
- Effect: `heal`, amount `1d8`

---

## Section 2: Job-Level Active Actions

53 jobs are missing active actions. Actions are organized in per-archetype batches. Effect type is assigned by job theme:

| Theme | Effect type |
|---|---|
| Combat / martial | `condition` (self) or `damage` |
| Stealth / criminal | `condition` (self) |
| Social / influence | `skill_check` |
| Healing / support | `heal` |
| Technical / craft | `skill_check` |

### criminal archetype jobs (10 jobs)
- **assassin** — `Marked Strike` (shortcut: `mstrike`, damage, 1d6 piercing, 1AP, combat)
- **bandit** — `Ambush` (shortcut: `ambush`, condition self `ambush_active`, 1AP, combat)
- **bookie** — `Calculate Odds` (shortcut: `odds`, skill_check Reasoning DC 14, 1AP, exploration)
- **con artist** — `Fast Talk` (shortcut: `fasttalk`, skill_check Deception DC 15, 1AP, exploration)
- **fence** — `Appraise` (shortcut: `appraise`, skill_check Reasoning DC 12, 1AP, exploration)
- **forger** — `Counterfeit` (shortcut: `forge`, skill_check Deception DC 16, 1AP, exploration)
- **pickpocket** — `Light Fingers` (shortcut: `lfing`, condition self `light_fingers_active`, 1AP, exploration)
- **racketeer** — `Intimidate` (shortcut: `racket`, skill_check Presence DC 14, 1AP, combat+exploration)
- **spy** — `Dead Drop` (shortcut: `ddrop`, skill_check Reasoning DC 15, 1AP, exploration)
- **thug** — `Brutal Strike` (shortcut: `bstrike`, damage, 1d8 bludgeoning, 1AP, combat)

### drifter archetype jobs (10 jobs)
- **bounty hunter** — `Subdue` (shortcut: `subdue`, condition self `subdue_active`, 1AP, combat)
- **caravan guard** — `Shield Wall` (shortcut: `shield`, condition self `shield_wall_active`, 1AP, combat)
- **courier** — `Sprint` (shortcut: `sprint`, condition self `sprint_active`, 1AP, combat+exploration)
- **deserter** — `Tactical Retreat` (shortcut: `retreat`, condition self `retreat_active`, 1AP, combat)
- **exile** — `Survivor's Will` (shortcut: `swill`, condition self `survivors_will_active`, 1AP, combat)
- **gambler** — `Bluff` (shortcut: `bluff`, skill_check Deception DC 14, 1AP, exploration)
- **mercenary** — `Press the Attack` (shortcut: `press`, condition self `press_active`, 1AP, combat)
- **nomad** — `Navigate` (shortcut: `navigate`, skill_check Reasoning DC 13, 1AP, exploration)
- **outlaw** — `Outlaw's Edge` (shortcut: `edge`, condition self `outlaws_edge_active`, 1AP, combat)
- **wanderer** — `Read the Land` (shortcut: `readland`, skill_check Awareness DC 12, 1AP, exploration)

### nerd archetype jobs (10 jobs)
- **alchemist** — `Brew Potion` (shortcut: `brew`, heal 1d4, 1AP, exploration)
- **artificer** — `Jury Rig` (shortcut: `jrig`, skill_check Reasoning DC 15, 1AP, exploration)
- **cartographer** — `Survey` (shortcut: `survey`, skill_check Reasoning DC 12, 1AP, exploration)
- **engineer** — `Analyze Structure` (shortcut: `analyze`, skill_check Reasoning DC 14, 1AP, exploration)
- **herald** — `Proclamation` (shortcut: `proclaim`, skill_check Presence DC 14, 1AP, exploration)
- **historian** — `Recall Lore` (shortcut: `lore`, skill_check Reasoning DC 13, 1AP, exploration)
- **lawyer** — `Legal Argument` (shortcut: `argue`, skill_check Reasoning DC 15, 1AP, exploration)
- **medic** — `Field Dressing` (shortcut: `dress`, heal 1d6, 2AP, combat+exploration)
- **sage** — `Ancient Knowledge` (shortcut: `ancient`, skill_check Reasoning DC 16, 1AP, exploration)
- **scribe** — `Decipher Script` (shortcut: `decipher`, skill_check Reasoning DC 13, 1AP, exploration)

### naturalist archetype jobs (8 jobs)
- **beastmaster** — `Beast Bond` (shortcut: `bond`, condition self `beast_bond_active`, 1AP, combat+exploration)
- **falconer** — `Falcon Strike` (shortcut: `falcon`, damage 1d6 slashing, 1AP, combat)
- **fisher** — `Net Throw` (shortcut: `net`, condition self `net_throw_active`, 1AP, combat)
- **forager** — `Forage` (shortcut: `forage`, heal 1d4, 2AP, exploration)
- **herbalist** — `Herbal Remedy` (shortcut: `herb`, heal 1d6, 2AP, exploration)
- **hunter** — `Quarry` (shortcut: `quarry`, condition self `quarry_active`, 1AP, combat)
- **ranger** — `Favored Terrain` (shortcut: `terrain`, condition self `favored_terrain_active`, 1AP, exploration)
- **trapper** — `Set Trap` (shortcut: `trap`, skill_check Reasoning DC 14, 1AP, exploration)

### schemer archetype jobs (8 jobs)
- **courtier** — `Court Intrigue` (shortcut: `intrigue`, skill_check Deception DC 15, 1AP, exploration)
- **diplomat** — `Negotiate` (shortcut: `negotiate`, skill_check Presence DC 14, 1AP, exploration)
- **fixer** — `Pull Strings` (shortcut: `pull`, skill_check Presence DC 15, 1AP, exploration)
- **infiltrator** — `Blend In` (shortcut: `blend`, condition self `blend_in_active`, 1AP, exploration)
- **manipulator** — `Puppet Strings` (shortcut: `puppet`, skill_check Deception DC 16, 1AP, exploration)
- **merchant** — `Hard Bargain` (shortcut: `bargain`, skill_check Presence DC 13, 1AP, exploration)
- **politician** — `Rabble Rouse` (shortcut: `rouse`, skill_check Presence DC 15, 1AP, exploration)
- **smuggler** — `Hidden Cargo` (shortcut: `cargo`, condition self `hidden_cargo_active`, 1AP, exploration)

### zealot archetype jobs (7 jobs)
- **cultist** — `Dark Invocation` (shortcut: `invoke`, condition self `dark_invocation_active`, 1AP, combat)
- **flagellant** — `Penance Strike` (shortcut: `penance`, damage 1d6 necrotic, 1AP, combat)
- **inquisitor** — `Denounce` (shortcut: `denounce`, skill_check Presence DC 15, 1AP, combat+exploration)
- **monk** — `Focused Strike` (shortcut: `fstrike`, damage 1d6 bludgeoning, 1AP, combat)
- **paladin** — `Divine Smite` (shortcut: `smite`, damage 1d8 radiant, 1AP, combat)
- **pilgrim** — `Fervent Prayer` (shortcut: `pray`, heal 1d6, 2AP, exploration)
- **templar** — `Holy Shield` (shortcut: `hshield`, condition self `holy_shield_active`, 1AP, combat)

---

## New Conditions Required

Each new condition referenced by a `condition` effect must be added to `content/conditions.yaml`:

- `ghost_active` — concealment, lasts 1 minute
- `marked_active` — +1 damage taken by target
- `primal_surge_active` — +2 Strength checks until end of combat
- `setup_active` — next social check with advantage
- `ambush_active` — first attack this turn gets +1d4 bonus damage
- `light_fingers_active` — bonus to Finesse checks this turn
- `subdue_active` — grapple attempt made with advantage
- `shield_wall_active` — +1 AC until next turn
- `sprint_active` — double movement speed until next turn
- `retreat_active` — move away without provoking opportunity attacks
- `survivors_will_active` — ignore first instance of being dropped below 0 HP
- `press_active` — follow-up attack costs 0 AP this turn
- `outlaws_edge_active` — +1 to all attack rolls until end of combat
- `beast_bond_active` — animal companion acts on your initiative
- `net_throw_active` — target is slowed (costs extra AP to move)
- `quarry_active` — +1d4 damage against designated target
- `favored_terrain_active` — advantage on Awareness checks outdoors
- `blend_in_active` — treated as non-threatening in social encounters
- `hidden_cargo_active` — carried contraband is not detected this scene
- `dark_invocation_active` — +1d4 necrotic damage on next attack
- `holy_shield_active` — +1 AC and immunity to frightened until next turn

---

## Implementation Order

1. Add all new conditions to `content/conditions.yaml`
2. Add archetype-level features (6 archetypes, 9 new entries)
3. Add criminal job actions (10)
4. Add drifter job actions (10)
5. Add nerd job actions (10)
6. Add naturalist job actions (8)
7. Add schemer job actions (8)
8. Add zealot job actions (7)
9. Update `docs/requirements/FEATURES.md` to mark Actions content complete

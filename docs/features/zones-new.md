# New Zones

Adds Clown Camp, SteamPDX, The Velvet Rope, and Club Privata as new explorable zones.

Design spec (Phase 1 — Mechanics): `docs/superpowers/specs/2026-03-21-zones-new-mechanics-design.md`

## Phase 1: Mechanics Requirements

- [x] REQ-ZN-1: `world.Zone` MUST gain a `ZoneEffects []RoomEffect` field (YAML: `zone_effects`). At world load time, the loader MUST append each `ZoneEffect` entry to every room's `Effects` slice in that zone. The world is loaded once at startup; hot-reload is not supported and no deduplication is required.
- [x] REQ-ZN-2: `RoomEffect.Track` MUST be treated as a condition ID. The hardcoded track enum MUST be removed. The effect application code MUST resolve the condition via `conditionRegistry.Get(effect.Track)` and apply it via `condition.ActiveSet.Apply`. If `conditionRegistry.Get` returns nil at runtime, the effect application code MUST log a warning and skip that effect.
- [x] REQ-ZN-3: At startup, `world.Validate()` MUST verify every `RoomEffect.Track` value (on both room-level and zone-level effects) exists in the condition registry. Any unknown track MUST cause a fatal startup error.
- [x] REQ-ZN-4: The four existing track values (`"rage"`, `"despair"`, `"delirium"`, `"fear"`) MUST have corresponding condition definitions in `content/conditions/` if not already present. All existing zone and room YAML files using these track values remain valid without modification.
- [x] REQ-ZN-5: New condition definitions MUST be added for: `"horror"`, `"nausea"`, `"reduced_visibility"`, `"temptation"`, `"revulsion"`, `"sonic_assault"`, `"charmed"`. These IDs MUST NOT use the `terrain_` prefix.
- [x] REQ-ZN-6: `npc.Template` MUST gain `Gender string` (YAML: `gender`); empty string indicates no gender. `npc.Instance` MUST propagate `Gender` from its template at spawn. `npc.Instance.Gender` is a runtime-only field and MUST NOT have a YAML tag. Per-instance YAML override of gender is NOT supported; the template is the sole source.
- [x] REQ-ZN-7: `seduce <npc>` MUST be an HTN operator. Preconditions: `instance.Gender != ""`, NPC does not already have the `charmed` condition, and `sess.Skills["flair"] > 0`.
- [x] REQ-ZN-8: Seduction resolution MUST use an opposed skill check: player Flair vs NPC Savvy. On player success, the NPC gains the `charmed` condition. On player failure, the NPC's disposition flips to hostile and `npc.Instance` MUST gain a `SeductionRejected map[string]bool` field (keyed by player UID) set to `true`; this field is runtime-only with no YAML tag. At respawn, `SeductionRejected` MUST be set to nil. The HTN operator MUST fail as a precondition if `SeductionRejected[playerUID]` is true.
- [x] REQ-ZN-9: The `charmed` condition MUST have `duration_type: until_save`. At the end of each round, the round-tick handler MUST check every NPC with the `charmed` condition and trigger a Savvy saving throw using the same resolution mechanism as all other NPC saving throws, against DC 15. On success the `charmed` condition is removed.
- [x] REQ-ZN-10: The `charmed` condition definition is included in REQ-ZN-5. Charmed NPCs treat the player as allied for the duration.
- [x] REQ-ZN-11: `ConditionDef` MUST gain optional fields `MoveAPCost int` (YAML: `move_ap_cost`) and `SkillPenalties map[string]int` (YAML: `skill_penalties`). Both default to zero/nil and are ignored on conditions that do not set them. Keys in `SkillPenalties` MUST be canonical skill IDs (lowercase, underscore-separated, e.g. `"flair"`, `"savvy"`). At startup, `world.Validate()` MUST verify every key in every condition's `SkillPenalties` exists in the skill registry; any unknown key MUST cause a fatal startup error.
- [x] REQ-ZN-12: Terrain condition definitions MUST be added to `content/conditions/` with IDs: `terrain_rubble`, `terrain_mud`, `terrain_flooded`, `terrain_ice`, `terrain_dense_vegetation`. All terrain condition IDs MUST use the `terrain_` prefix. Each MUST set `move_ap_cost` and/or `skill_penalties` values appropriate to the terrain type.
- [x] REQ-ZN-13: The existing `Properties["terrain"] == "difficult"` check MUST be removed from the movement handler. The movement handler MUST instead collect all conditions in the room's effective condition set (after zone propagation) whose ID has the `terrain_` prefix and whose `MoveAPCost > 0`. The total AP cost deducted MUST equal the sum of all matching conditions' `MoveAPCost` values. One message per matching terrain condition MUST be sent using that condition's label, ordered by condition ID alphabetically. The `zone_awareness` passive feat MUST suppress all terrain messages (conditions with the `terrain_` prefix) but MUST NOT suppress the AP cost deduction.
- [x] REQ-ZN-14: Terrain condition IDs MUST NOT appear in REQ-ZN-5, and non-terrain condition IDs MUST NOT use the `terrain_` prefix.

## Phase 2: Zone Content Requirements

- New zones
  - All NPCs are clowns
  - All NPCs are aggressive
  - All rooms in zone are Dangerous
  - [x] Clown Camp
    - [x] Continuous zone effects: delirium, fear
    - [x] Locations
      - [x] The Empty Theater
      - [x] Coat check
      - [x] The Changing Rooms
      - [x] Backstage
      - [x] The Stage
        - [x] Boss Fight: Just Clownin'!
  - [x] SteamPDX
    - All NPCs are male
      - All NPCs have a high probability attempt to seduce male players
        - On failure to seduce NPCs become aggressive
    - All NPCs are aggressive to non-male players
    - All rooms in zone are Dangerous
    - [x] Continuous zone effects: horror, nausea, reduced visibility
    - [x] Locations
      - [x] Parking Lot
      - [x] Lobby
      - [x] Locker Room
      - [x] Showers
      - [x] Sauna
      - [x] Hot tub
      - [x] Glory Hole
        - [x] Boss Fight: The Big 3
  - [x] The Velvet Rope
    - All NPCs have a high probability attempt to seduce the player
      - On failure to seduce NPCs become aggressive
    - All rooms in zone are Sketchy unless otherwise indicated
    - [x] continuous zone effects: temptation, revulsion, difficult terrain (lube)
    - [x] Locations
      - [x] The Buffet
      - [x] The "Play" Rooms
        - [x] The Strangers
        - [x] The Spit Roast
        - [x] The Pineapple Room
          - This room is Dangerous
      - [x] "Party" Theater
        - This room is Dangerous
        - [x] Boss Fight: Gangbang!
  - [x] Club Privata
    - [x] continuous zone effect: sonic assault (bad techno)
    - [x] Locations
      - [x] First Floor
        - [x] Bar
        - [x] Dance floor
        - [x] Dining area
        - [x] Couple's lounge
        - [x] Lockers & Showers
        - [x] Restrooms
      - [x] Second Floor
        - [x] Private Rooms
        - [x] Bottle service area
        - [x] Dance pole
        - [x] Mezzanine
        - [x] Restrooms
      - [x] Third Floor
        - [x] Public play area
        - [x] Couple's lounge with private rooms
        - [x] Bar & Restrooms
        - [x] VIP Suite
          - [x] Boss Fight
        - [x] Lockers

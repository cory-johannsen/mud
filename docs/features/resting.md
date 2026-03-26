# Resting

Adds motels and camping (with gear requirements) as rest locations, restricted to exploration zones. Also reduces passive HP regeneration from 30-second to 5-minute ticks so that resting is the meaningful post-combat recovery method.

## Requirements

- REQ-REST-1: `rest` MUST be blocked while in combat.
- REQ-REST-2: `rest` in a safe room with a motel NPC MUST perform motel rest (instant, full restoration).
- REQ-REST-3: `rest` in a non-safe zone with active exploration mode MUST perform camping rest (timed).
- REQ-REST-4: `rest` in a safe room without a motel NPC or in a non-safe zone without exploration mode MUST return an error.
- REQ-REST-5: Motel rest MUST be instant (no timer).
- REQ-REST-6: Motel rest MUST deduct the motel NPC's `RestCost` credits from the player.
- REQ-REST-7: Motel rest MUST be blocked if the player has insufficient credits, showing the cost.
- REQ-REST-8: `npc.Instance` MUST have a `RestCost int` field populated from `rest_cost:` in motel NPC YAML.
- REQ-REST-9: Motel rest MUST deliver full HP + tech pool + prepared tech restoration.
- REQ-REST-10: Camping MUST require a `sleeping_bag` tagged item in the player's backpack.
- REQ-REST-11: Camping MUST require a `fire_material` tagged item in the player's backpack.
- REQ-REST-12: Missing gear errors MUST name the specific missing item type.
- REQ-REST-13: Camping base duration MUST be 5 minutes with no `camping_gear` items.
- REQ-REST-14: Each `camping_gear` item MUST reduce camping duration by 30 seconds; minimum 2 minutes.
- REQ-REST-15: Enemy entry into the room during camping MUST cancel camping with partial HP + tech restore.
- REQ-REST-16: Player movement while camping MUST cancel camping with partial restore.
- REQ-REST-17: Gear items (sleeping_bag, fire_material, camping_gear) MUST NOT be consumed on rest or cancel.
- REQ-REST-18: Successful camping completion MUST deliver full long-rest restoration.
- REQ-REST-19: `RegenInterval` MUST be 5 minutes. Regen formula (`max(1, GritMod)` HP per tick) is unchanged.

# Combat Initiation Message

## Overview

When combat begins, a message is displayed in the player's console identifying who initiated combat and, for NPC-initiated combat, why the NPC attacked.

## Requirements

- COMBATMSG-1: The system MUST display a console message when combat is initiated.
- COMBATMSG-2: The message MUST identify the entity that initiated combat (player name or NPC name).
- COMBATMSG-3: When an NPC initiates combat, the message MUST include the reason for aggression.
- COMBATMSG-4: NPC aggression reasons MUST map to the NPC's awareness/hostility trigger:
  - COMBATMSG-4a: `"attacked on sight"` — NPC is unconditionally hostile (aggressive flag).
  - COMBATMSG-4b: `"defending its territory"` — NPC attacked due to proximity threshold.
  - COMBATMSG-4c: `"provoked by your attack"` — NPC retaliated after being struck first.
  - COMBATMSG-4d: `"responding to a call for help"` — NPC joined combat via assist/call mechanic.
  - COMBATMSG-4e: `"alerted by your wanted status"` — NPC (guard type) attacked due to wanted level.
  - COMBATMSG-4f: `"protecting [NPC name]"` — NPC joined to defend a nearby allied NPC.
- COMBATMSG-5: When a player initiates combat, the message MUST read: `"You attack [target name]."`.
- COMBATMSG-6: When an NPC initiates combat against the player, the message MUST follow the form: `"[NPC name] attacks you — [reason]."`.
- COMBATMSG-7: The message MUST be delivered only to the player(s) directly involved in the initiation event (attacker and/or initial target).
- COMBATMSG-8: The message MUST appear before the first combat round output.

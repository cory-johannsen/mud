# Social System Requirements

## Chat Channels

- SOC-1: The engine MUST support multiple concurrent chat channels per player.
- SOC-2: The engine MUST provide built-in channel types: global, zone, room (local), party, and guild.
- SOC-3: The ruleset or setting MAY define additional custom channels via YAML configuration.
- SOC-4: Players MUST be able to send direct messages (tells) to other online players.
- SOC-5: Chat messages MUST be broadcast only to eligible recipients based on channel membership.
- SOC-6: Chat channels MUST support moderation controls: mute, ban, and channel-level permissions.

## Parties and Groups

- SOC-7: Players MUST be able to form parties with a configurable maximum size defined by the ruleset.
- SOC-8: Parties MUST have a leader who can invite, kick, and disband.
- SOC-9: Party members MUST share a party chat channel.
- SOC-10: The combat system MUST recognize party membership for group combat encounters.
- SOC-11: The ruleset MAY define experience sharing, loot distribution, and other party mechanics via Lua scripts.

## Guilds and Factions

- SOC-12: Players MUST be able to create and join guilds.
- SOC-13: Guilds MUST support a configurable rank and permission system.
- SOC-14: Guilds MUST have a dedicated chat channel.
- SOC-15: Guild data (members, ranks, treasury, log) MUST be persisted to PostgreSQL.
- SOC-16: The setting MAY define NPC factions with reputation systems; faction definitions MUST be in YAML with Lua behavior hooks.

## PvP

- SOC-17: The engine MUST support player-versus-player combat using the same combat system as PvE.
- SOC-18: PvP MUST be governable by zone-level flags (PvP-enabled zones, safe zones).
- SOC-19: The ruleset MAY define additional PvP rules (level range restrictions, flagging systems, consequences) via Lua scripts.

## Economy and Trading

- SOC-20: The engine MUST support a currency system defined by the ruleset.
- SOC-21: Players MUST be able to trade items and currency directly with other players.
- SOC-22: The engine MUST support NPC vendors with buy/sell inventories defined in YAML.
- SOC-23: The ruleset MAY define crafting, auction houses, or other economic systems via Lua scripts and YAML data.
- SOC-24: All economic transactions MUST be logged for auditing and anti-cheat purposes.

## Friend Lists (Deferred)

- SOC-25: The engine SHOULD support a friend list system in a future phase.
- SOC-26: Friend list requirements MUST be defined prior to implementation.

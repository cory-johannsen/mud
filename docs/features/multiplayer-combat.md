# Multiplayer Combat

Allows multiple players to join, share XP and loot, and form groups that enter combat together.

## Requirements

- [x] Multi-player combat
  - [x] Other players can join combat already in progress
    - [x] Combat join — when a player enters a room with active combat, offer to join; on joining, add player as a new combatant with a fresh initiative roll and full AP
    - [x] All players in a combat encounter share XP for the encounter (XP is divided equally among the players)
      - [x] Track participant list on Combat struct; on combat end divide total XP equally among all participants
    - [x] All players in a combat encounter share loot for the encounter (loot is divided equally among the players)
      - [x] Currency drops split equally; item drops distributed round-robin ordered by initiative roll
  - [x] Players can form groups
    - [x] Group data model — add Group struct (leader UID, member UIDs slice) stored in session manager; one group per player
    - [x] Players in a group all automatically enter combat when any player initiates combat
      - [x] On combat start, check if initiating player is in a group; add all group members in the same room as combatants
    - [x] New group commands:
      - [x] group (create a group or list the members of the current group)
        - [x] Implement `group` — no args: display current group members and leader; with player name: create a new group and invite that player
      - [x] ungroup (leave the current group)
        - [x] Implement `ungroup` — remove self from group; if leader, disband the group and notify all members
      - [x] invite (invite a player to the group)
        - [x] Implement `invite <player>` — leader sends invitation; target receives a prompt to accept or decline; on accept, add to group
      - [x] kick (remove a player from the group)
        - [x] Implement `kick <player>` — leader only; removes the named player from the group and notifies them

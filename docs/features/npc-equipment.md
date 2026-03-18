# NPC Equipment

Each NPC gets equipment assigned and equipped at spawn time, including weapon and armor.

## NPC Equipment

- [x] NPC equipment - each NPC gets equipment assigned and equipped
  - Weapon
  - Armor
  - [x] Add `weapon` and `armor` fields to NPC YAML schema (item ID or weighted random table); parsed at load time
  - [x] On NPC spawn, populate equipped weapon and armor from YAML; apply armor AC bonus to NPC base AC
  - [x] Include equipped weapon name in combat strike messages (e.g., "Bandit attacks with a rusty knife")
  - [x] Disarm command — implement `disarm <target>` (Athletics vs Reflex DC; on success removes target's active weapon from their equipped slot for the remainder of combat; requires NPC equipment tracking in combat)

## Disarm Action

- [x] Disarm action
  - [x] See implementation items under NPC equipment — Disarm requires NPC weapon slot tracking in combat

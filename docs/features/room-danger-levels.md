# Room Danger Levels

Rooms are classified as Safe, Sketchy, Dangerous, or All Out War, with color-coded display on map and room view.

## Requirements

- [ ] Room danger levels
  - Rooms are classified as Safe, Sketchy, Dangerous, All Out War
    - Safe rooms contain no aggressive NPCs, only non-combat NPCs.
      - Combat is disabled in Safe zones.
    - Sketchy rooms contain non-Combat NPCs and combat NPCs
      - Combat is enabled in Sketchy rooms
      - Combat can only be initiated by players in Sketchy rooms, not by NPCs
      - Sketchy rooms may contain cover
        - Cover can not be destroyed in Sketchy rooms
        - Cover has a low chance of being trapped in Sketchy rooms
      - Sketchy rooms do not contain room traps
      - Sketchy rooms have a low chance of traps on room equipment
    - Dangerous rooms contain non-Combat NPCs and combat NPCs
      - Combat is enabled in Dangerous rooms
      - Combat can only be initiated by anyone in a Dangerous room
      - Non-combat NPCs flee combat if engaged
      - Dangerous rooms may contain cover
        - Cover can be destroyed in Dangerous rooms
        - Cover has a high chance to be trapped in Dangerous rooms
      - Combat rooms have a moderate change to contain room traps
      - Combat rooms have a moderate chance to contain traps on room equipment
    - All Out War rooms contain only combat NPCs
      - Combat is enabled in All Out War rooms
      - Combat NPCs attack on sight in an All Out War room
      - All Out War rooms may contain cover
        - Cover can be destroyed in All Out War rooms
        - Cover has a high chance to be trapped in All Out War rooms
      - Combat rooms have a high chance to contain room traps
      - Combat rooms have a high chance to contain traps on room equipment
  - The safely level of a room should be included in the room description, color coded to the safety level. Safe is Green, Sketchy is yellow, Dangerous is orange, All Out War is red.
  - The safely level of a room should be included in the map, color coded to the safety level. Safe is Green, Sketchy is yellow, Dangerous is orange, All Out War is red.

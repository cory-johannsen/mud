# Advanced Enemies

Adds NPC difficulty tiers, tags, feats, XP/loot scaling, and boss encounters.

## Requirements

- [ ] Advanced Enemies
  - NPC difficulty tiers
    - Tiers:
      - Normie
      - Modded
      - Vet
      - Alpha
      - Apex
    - Tags
      - Applying Tags alters the stats, attributes, and skills of the NPC.
        - Tags be hierarchical
        - Tags are chosen at random based on the NPC and Zone
          - NPC yaml includes an allow/deny list of all supported tags for the NPC
          - NPC Tier determines level and number of tags to apply
            - NPC yaml includes valid ranges to select from
    - Feats
      - Higher tiers grant additional Feats
        - selected randomly at NPC creation time
        - NPC yaml includes all/deny list of all supported feats for the NPC
    - XP and Loot scaling
      - Higher tiers grant additional XP
      - Higher tiers increase the currency dropped
      - Higher tiers increase the likelihood of item drops
      - Higher tiers increase the likelihood of items with higher rarity
      - Higher tiers increase the likelihood of items with higher stats and bonuses
  - Bosses
    - Boss rooms
      - Dangerous or All Out War depending on boss
      - Contains environment hazards
      - Contains traps
    - Boss abilities
    - Boss behavior
    - Boss rewards
      - [ ] Award 1 hero point on boss kill
    - Minions

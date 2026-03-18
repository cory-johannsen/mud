# Non-Combat NPCs

Defines the data model, behavior, and content for non-combat NPC types in the game world.

## Non-Combat NPCs

- [ ] Non-combat NPCs.
  - Define the data model and behavior for the following NPCs and implement those specifically mentioned.
  - For those not mentioned generate one that lives in a room in Rustbucket Ridge and matches the lore. Multiple NPCs can occupy the same room.
  - [ ] Non-combat NPC base data model — add `npc_type` field (merchant, guard, healer, quest_giver, hireling, banker, job_trainer) to NPC YAML; non-combat NPCs do not appear in combat initiative; they flee or cower when combat starts in their room
  - [ ] merchants
      - types:
        - [ ] weapons
          - Sergeant Mack in Last Stand Lodge, Rustbucket Ridge
        - [ ] armor
        - [ ] rings and neck equipment
        - [ ] consumables
          - Slick Sally in the Rusty Oasis, Rustbucket Ridge
          - Whiskey Joe in The Bottle Shack, Rustbucket Ridge
          - Old Rusty in The Heap, Rustbucket Ridge
          - Herb in The Green Hell, Rustbucket Ridge
        - [ ] maps - sells maps to other zones
        - [ ] technology - sells Technology
        - [ ] drugs - sells Drugs and other Technological substances
      - Each merchant has a budget with which to purchase items from players
      - Each merchant has a profit margin they apply to the items they buy and sell
      - Purchasing items from a merchant should provide players with the necessary skills to attempt to negotiate
          - Critical success: provides a substantial discount or bonus on the transaction
          - Success: provides a discount or bonus on the transactions
          - Failure: No effect
          - Critical failure: adds a penalty to on the transactions
      - [ ] Merchant YAML schema — add `inventory` (list of item IDs with stock quantities and base prices), `sell_margin` (markup multiplier), `buy_margin` (fraction of item value paid to player), `budget` (max credits available to buy from players)
      - [ ] `buy <item> [qty]` command — available in rooms with a merchant NPC; deducts credits from player, adds item to inventory
      - [ ] `sell <item> [qty]` command — available in rooms with a merchant NPC; pays player `buy_margin × item value`; checks merchant budget
      - [ ] `browse` command — list merchant's inventory with current prices
      - [ ] Negotiate skill check — player may use `negotiate` before a buy/sell; smooth_talk or grift vs merchant Perception DC; critical success: ±20% price; success: ±10%; failure: no effect; critical failure: +10% penalty applied
      - [ ] Add named merchant NPCs: Sergeant Mack (weapons, Last Stand Lodge), Slick Sally (consumables, Rusty Oasis), Whiskey Joe (consumables, Bottle Shack), Old Rusty (consumables, The Heap), Herb (consumables, The Green Hell)
    - [ ] guards
      - [ ] Guard behavior — guards are present in Safe rooms; they attack players with a Wanted flag; they do not initiate combat with non-Wanted players; if combat occurs in a Safe room they target the aggressor
      - [ ] Add a lore-appropriate guard NPC in a Safe room in Rustbucket Ridge
    - [ ] healers
        - Clutch in The Tinker's Den, Rustbucket Ridge
        - Tina Wires in Junker's Dream, Rustbucket Ridge
      - [ ] Healer behavior — players may `heal` in a healer's room for a credit cost; full heal or partial heal at a per-HP rate; healer has a daily capacity
      - [ ] Add Clutch (The Tinker's Den) and Tina Wires (Junker's Dream) NPC YAML files
    - [ ] quest givers
        - Gail "Grinder" Graves in Scrapshack 23, Rustbucket Ridge
      - [ ] Quest giver behavior — `talk <npc>` offers available quests; on quest completion player receives XP and item/credit reward; requires Quest system
      - [ ] Add Gail "Grinder" Graves NPC YAML (Scrapshack 23); wire to a starter quest once Quest system exists
    - [ ] hirelings
      - [ ] Hireling behavior — `hire <npc>` for a daily credit cost; hireling follows the player between rooms and joins combat as an AI-controlled combatant; `dismiss` releases the hireling
      - [ ] Add a lore-appropriate hireling NPC in Rustbucket Ridge
    - [ ] bankers
      - [ ] Banker behavior — `deposit <amount>` and `withdraw <amount>` commands available in banker's room; credit stash persists on character separate from carried credits; display stash balance in `inventory`
      - [ ] Add a lore-appropriate banker NPC in a Safe room in Rustbucket Ridge
    - [ ] job trainers - allow players to learn new jobs once they meet the requirements.
      - Each job has minimum requirements
      - Each player has exactly one active Job.
        - The Active Job is the one that earns XP.
        - Inactive Jobs do not earn XP, but the player may still use the feats and proficiencies they provide
        - A command must exist to allow the player to view their Jobs and select which one is Active
      - [ ] Job trainer behavior — `train <job>` command in trainer's room; checks player meets job prerequisites; deducts training credit cost; adds job to player's job list
      - [ ] `jobs` command — list player's active and inactive jobs; `setjob <job>` switches the active job
      - [ ] Add a lore-appropriate job trainer NPC in Rustbucket Ridge
    - [ ] equipment repair and crafting

## Non-Combat NPCs — All Zones

  - [ ] Every zone must have a lore appropriate instance of each non-combat NPC type that lives in a Safe room.  Multiple NPCs can live in the same room.

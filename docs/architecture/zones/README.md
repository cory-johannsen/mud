# Zone Map Index

This directory contains Mermaid graph diagrams and room tables for all 16 zones in the game world.

## Zone List

| Zone ID | Zone Name | Room Count | Safe Rooms | Link |
|---------|-----------|------------|------------|------|
| aloha | The Aloha Neutral Zone | 21 | 1 (The Bazaar) | [aloha.md](aloha.md) |
| battleground | Battleground Socialist Collective | 22 | 1 (The Infirmary) | [battleground.md](battleground.md) |
| beaverton | The Free State of Beaverton | 23 | 1 (Free Market) | [beaverton.md](beaverton.md) |
| downtown | Downtown Portland | 14 | 1 (The Underground) | [downtown.md](downtown.md) |
| felony_flats | Felony Flats | 22 | 1 (Jade District) | [felony_flats.md](felony_flats.md) |
| hillsboro | Kingdom of Hillsboro | 22 | 1 (The Keep) | [hillsboro.md](hillsboro.md) |
| lake_oswego | Lake Oswego Nation | 22 | 1 (The Commons) | [lake_oswego.md](lake_oswego.md) |
| ne_portland | Northeast Portland | 24 | 1 (Corner Store) | [ne_portland.md](ne_portland.md) |
| pdx_international | PDX International | 24 | 1 (Terminal B) | [pdx_international.md](pdx_international.md) |
| ross_island | Ross Island | 22 | 1 (Dock Shack) | [ross_island.md](ross_island.md) |
| rustbucket_ridge | Rustbucket Ridge | 36 | 5 (Scrap Office, Grinder's Row, Last Stand Lodge, Wayne Dawg's Trailer, Dwayne Dawg's Trailer) | [rustbucket_ridge.md](rustbucket_ridge.md) |
| sauvie_island | Sauvie Island | 22 | 1 (Farm Stand) | [sauvie_island.md](sauvie_island.md) |
| se_industrial | Southeast Industrial | 22 | 1 (Break Room) | [se_industrial.md](se_industrial.md) |
| the_couve | The Couve | 21 | 1 (The Crossing) | [the_couve.md](the_couve.md) |
| troutdale | Troutdale | 24 | 1 (Truck Stop) | [troutdale.md](troutdale.md) |
| vantucky | Vantucky | 30 | 5 (The Compound, Neutral Pawn, Neutral Back, Neutral Vault, Fourth Plain Border) | [vantucky.md](vantucky.md) |

## Danger Level Reference

| YAML Value | Abbreviation | Description |
|------------|-------------|-------------|
| safe | safe | No hostile encounters |
| sketchy | low | Minor threats |
| dangerous | high | Significant danger |
| all_out_war | xtr | Extreme danger |

## Notes

- Each zone has exactly one `[S]` safe room that serves as the player services hub (merchant, healer, job trainer, banker).
- Rooms with `map_x: 202` or coordinates far from origin are disconnected from the main zone grid (orphaned rooms accessible only via zone-internal connections not represented in map coordinates).
- Inter-zone exits are shown as edges to external room IDs without a node definition in the diagram.

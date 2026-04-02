# Rustbucket Ridge — Zone Map

## Zone Info

| Field | Value |
|-------|-------|
| Zone ID | `rustbucket_ridge` |
| Danger Level | dangerous |
| Map Color | Orange |
| World Position | (4, 0) |
| Start Room | `rust_scrap_office` |

## ASCII Map

```
          [ 1] [ 2]-[ 3]
            |    |
[ 4]-[ 5] [ 6]-[ 7]
       |    |
[ 8]-[ 9]-[10]-[11]
  |         |
[12]      [13]-[14]-[SS]
            |
     [16]-[17]-[18]
       |    |
[36]-[19] [20]      [21]>     [22]
            |         |
     [23]-[24]      [25]-[26] [27]
       |    |         |
     [28] [29] [30]-[31]
            |    |
          [32]-[33]-[34]
                      |
                    [35]
```

**Legend:** `[SS]` = safe start room · `>` = zone border east (NE Portland) · `[36]` added west of `[19]`

## Room Table

| # | Room ID | Name | Danger Level | Coordinates |
|---|---------|------|--------------|-------------|
| 1 | `ghosts_rest` | Ghost's Rest | inherit | (0, -6) |
| 2 | `the_forgotten_trailer` | The Forgotten Trailer | inherit | (2, -6) |
| 3 | `bonepickers_roost` | Bonepicker's Roost | inherit | (4, -6) |
| 4 | `wreckers_rest` | Wrecker's Rest | inherit | (-4, -4) |
| 5 | `the_tinkers_den` | The Tinker's Den | inherit | (-2, -4) |
| 6 | `the_graveyard` | The Graveyard | inherit | (0, -4) |
| 7 | `the_mausoleum` | The Mausoleum | inherit | (2, -4) |
| 8 | `scrapshack_23` | Scrapshack 23 | inherit | (-4, -2) |
| 9 | `the_heap` | The Heap | inherit | (-2, -2) |
| 10 | `rotgut_alley` | Rotgut Alley | inherit | (0, -2) |
| 11 | `the_bottle_shack` | The Bottle Shack | inherit | (2, -2) |
| 12 | `salvage_hut` | Salvage Hut | inherit | (-4, 0) |
| 13 | `grinders_row` | Grinder's Row | safe | (0, 0) |
| 14 | `last_stand_lodge` | Last Stand Lodge | safe | (2, 0) |
| 15 | `rust_scrap_office` | Scrap Office | safe | (4, 0) |
| 16 | `the_green_hell` | The Green Hell | inherit | (-2, 2) |
| 17 | `the_rusty_oasis` | The Rusty Oasis | inherit | (0, 2) |
| 18 | `roach_haven` | Roach Haven | inherit | (2, 2) |
| 19 | `wayne_dawgs_trailer` | Wayne Dawg's Trailer | safe | (-2, 4) |
| 20 | `junkers_dream` | Junker's Dream | inherit | (0, 4) |
| 21 | `the_slashers_den` | The Slasher's Den | inherit | (4, 4) |
| 22 | `the_razor_nest` | The Razor Nest | inherit | (6, 4) |
| 23 | `the_barrel_house` | The Barrel House | inherit | (-2, 6) |
| 24 | `the_still` | The Still | inherit | (0, 6) |
| 25 | `blade_house` | Blade House | inherit | (4, 6) |
| 26 | `blood_camp` | Blood Camp | inherit | (5, 6) |
| 27 | `the_cutthroat` | The Cutthroat | inherit | (6, 6) |
| 28 | `the_keg_hole` | The Keg Hole | inherit | (-2, 8) |
| 29 | `rotgut_shack` | Rotgut Shack | inherit | (0, 8) |
| 30 | `the_furnace` | The Furnace | inherit | (2, 8) |
| 31 | `the_embers_edge` | The Ember's Edge | inherit | (4, 8) |
| 32 | `snakepit` | Snakepit | inherit | (0, 10) |
| 33 | `ashen_hollow` | Ashen Hollow | inherit | (2, 10) |
| 34 | `smokers_den` | Smoker's Den | inherit | (4, 10) |
| 35 | `scorchside_camp` | Scorchside Camp | inherit | (4, 12) |
| 36 | `dwayne_dawgs_trailer` | Dwayne Dawg's Trailer | safe | (-4, 4) |

## Points of Interest

| Room | Name | POIs |
|------|------|------|
| `salvage_hut` [12] | Salvage Hut | Merchant `$` |
| `grinders_row` [13] | Grinder's Row | Motel Keeper `N` · Banker (Vera Coldcoin) `N` · Guard (Marshal Ironsides) `G` · Zone Map `M` |
| `last_stand_lodge` [14] | Last Stand Lodge | Merchant (Sgt. Mack) `$` · Chip Doc `N` |
| `rust_scrap_office` [SS] | Scrap Office | Healer `+` · Trainer `T` |
| `the_rusty_oasis` [17] | The Rusty Oasis | Merchant (Slick Sally) `$` |
| `junkers_dream` [20] | Junker's Dream | Healer (Tina Wires) `+` |
| `the_razor_nest` [22] | The Razor Nest | Banker `N` |
| `scorchside_camp` [35] | Scorchside Camp | Hireling `N` |

**POI symbol key:** `$` merchant · `+` healer · `T` trainer · `G` guard · `N` notable NPC · `M` zone map · `C` cover · `E` other equipment

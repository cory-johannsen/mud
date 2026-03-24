# Kingdom of Hillsboro

Zone ID: `hillsboro` | Danger Level: sketchy | World Position: (-4, 0)

```mermaid
graph LR
    hills_tv_highway_east["TV Highway East\n[DL: med]"]
    hills_campus_gate["Campus Gate\n[DL: med]"]
    hills_campus_boulevard["Campus Boulevard\n[DL: med]"]
    hills_server_keep["The Server Keep\n[DL: med]"]
    hills_throne_room["The Throne Room\n[DL: med]"]
    hills_royal_archives["The Royal Archives\n[DL: med]"]
    hills_solar_farm["The Solar Farm\n[DL: med]"]
    hills_serf_quarters["Serf Quarters\n[DL: med]"]
    hills_knight_barracks["Knight Barracks\n[DL: med]"]
    hills_training_grounds["Training Grounds\n[DL: med]"]
    hills_the_foundry["The Foundry\n[DL: med]"]
    hills_market_square["Market Square\n[DL: med]"]
    hills_orenco_station["Orenco Station\n[DL: med]"]
    hills_rock_creek_trail["Rock Creek Trail\n[DL: med]"]
    hills_tanasbourne_ruins["Tanasbourne Ruins\n[DL: med]"]
    hills_cornell_road["Cornell Road\n[DL: med]"]
    hills_power_station["Power Station\n[DL: med]"]
    hills_refugee_processing["Refugee Processing\n[DL: med]"]
    hills_amberglen_plaza["Amberglen Plaza\n[DL: med]"]
    hills_suburban_wasteland["Suburban Wasteland\n[DL: med]"]
    hills_helvetia_overlook["Helvetia Overlook\n[DL: med]"]
    hills_the_keep["The Keep\n[DL: safe] [S]"]

    hills_tv_highway_east -->|east| aloha_tv_highway_west
    hills_tv_highway_east -->|west| hills_campus_gate
    hills_tv_highway_east -->|north| hills_cornell_road
    hills_tv_highway_east -->|south| hills_the_keep
    hills_campus_gate -->|east| hills_tv_highway_east
    hills_campus_gate -->|west| hills_campus_boulevard
    hills_campus_gate -->|south| hills_orenco_station
    hills_campus_boulevard -->|east| hills_campus_gate
    hills_campus_boulevard -->|west| hills_server_keep
    hills_campus_boulevard -->|north| hills_knight_barracks
    hills_campus_boulevard -->|south| hills_serf_quarters
    hills_server_keep -->|east| hills_campus_boulevard
    hills_server_keep -->|north| hills_throne_room
    hills_server_keep -->|south| hills_the_foundry
    hills_server_keep -->|down| hills_power_station
    hills_throne_room -->|south| hills_server_keep
    hills_solar_farm -->|north| hills_the_foundry
    hills_serf_quarters -->|north| hills_campus_boulevard
    hills_knight_barracks -->|south| hills_campus_boulevard
    hills_the_foundry -->|north| hills_server_keep
    hills_the_foundry -->|south| hills_solar_farm
    hills_market_square -->|south| hills_tanasbourne_ruins
    hills_orenco_station -->|north| hills_campus_gate
    hills_orenco_station -->|south| hills_amberglen_plaza
    hills_tanasbourne_ruins -->|north| hills_market_square
    hills_tanasbourne_ruins -->|west| hills_suburban_wasteland
    hills_cornell_road -->|south| hills_tv_highway_east
    hills_cornell_road -->|west| hills_rock_creek_trail
    hills_cornell_road -->|north| hills_helvetia_overlook
    hills_power_station -->|up| hills_server_keep
    hills_amberglen_plaza -->|north| hills_orenco_station
    hills_suburban_wasteland -->|east| hills_tanasbourne_ruins
    hills_helvetia_overlook -->|south| hills_cornell_road
    hills_helvetia_overlook -->|southeast| hills_suburban_wasteland
    hills_the_keep -->|north| hills_tv_highway_east
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| hills_tv_highway_east | TV Highway East | med | 0 | 0 |
| hills_campus_gate | Campus Gate | med | -2 | 0 |
| hills_campus_boulevard | Campus Boulevard | med | -4 | 0 |
| hills_server_keep | The Server Keep | med | -6 | 0 |
| hills_throne_room | The Throne Room | med | -6 | -2 |
| hills_royal_archives | The Royal Archives | med | 202 | 0 |
| hills_solar_farm | The Solar Farm | med | -6 | 4 |
| hills_serf_quarters | Serf Quarters | med | -4 | 2 |
| hills_knight_barracks | Knight Barracks | med | -4 | -2 |
| hills_training_grounds | Training Grounds | med | 202 | 2 |
| hills_the_foundry | The Foundry | med | -6 | 2 |
| hills_market_square | Market Square | med | 4 | -4 |
| hills_orenco_station | Orenco Station | med | -2 | 2 |
| hills_rock_creek_trail | Rock Creek Trail | med | -2 | -2 |
| hills_tanasbourne_ruins | Tanasbourne Ruins | med | 4 | -2 |
| hills_cornell_road | Cornell Road | med | 0 | -2 |
| hills_power_station | Power Station | med | 202 | 8 |
| hills_refugee_processing | Refugee Processing | med | 202 | 10 |
| hills_amberglen_plaza | Amberglen Plaza | med | -2 | 4 |
| hills_suburban_wasteland | Suburban Wasteland | med | 2 | -2 |
| hills_helvetia_overlook | Helvetia Overlook | med | 0 | -4 |
| hills_the_keep | The Keep | safe | 0 | 2 |

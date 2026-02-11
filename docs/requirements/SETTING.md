# Default Setting Requirements — Gunchete

## Setting Overview

- SET-1: The default setting MUST be named "Gunchete".
- SET-2: Gunchete MUST depict a dystopian sci-fi future set in the shattered remains of Portland, Oregon.
- SET-3: The setting MUST be implemented entirely through the engine's pluggable setting and ruleset interfaces, serving as a reference implementation.
- SET-4: The setting MUST be self-contained: all content (zones, rooms, NPCs, items, lore) MUST reside in YAML data files and Lua scripts within the setting module.

## World Context

- SET-5: The setting MUST establish a coherent backstory for the collapse of Portland and the current state of the city.
- SET-6: The setting MUST define multiple distinct zone themes based on the geography and neighborhoods of Portland (e.g., ruined downtown core, flooded waterfront, fortified enclaves, overgrown parks, collapsed bridges, underground transit tunnels, industrial wastelands).
- SET-7: Zones MUST reflect recognizable Portland landmarks and districts, adapted to the post-collapse context.

## Ancestry as Home Region

- SET-8: The PF2E ancestry system MUST be repurposed to represent the player character's home region within the shattered Portland area.
- SET-9: Each home region MUST define: ability score modifiers, unique regional traits, a narrative description, and heritage options.
- SET-10: Home regions MUST follow the pattern established in the reference backgrounds (see: `github.com/cory-johannsen/gomud/tree/main/assets/backgrounds`): a name, description, modifier array across ability scores, and a list of selectable traits.
- SET-11: All player characters MUST be human; non-human ancestries from PF2E MUST NOT be available.
- SET-12: The setting MUST define at least five home regions representing distinct areas of post-collapse Portland.

## Classes

- SET-13: The setting MUST adapt PF2E classes to fit the sci-fi context (e.g., combat-oriented, tech-oriented, chemistry-focused, scavenger archetypes).
- SET-14: Class flavor and naming MUST be thematically appropriate to the post-collapse world; mechanical function MUST be preserved from PF2E.

## No Magic — Technology and Chemistry

- SET-15: The setting MUST NOT include magic, arcane, divine, primal, or occult spellcasting traditions from PF2E.
- SET-16: All PF2E spell and magical ability mechanics MUST be replaced with technology-based or chemistry-based equivalents.
- SET-17: Technology-based abilities MUST represent advanced devices, cybernetics, drones, energy systems, and electronic warfare.
- SET-18: Chemistry-based abilities MUST represent compounds, stimulants, toxins, explosives, and field-synthesized substances.
- SET-19: Replacement abilities MUST preserve the mechanical properties (action cost, range, damage dice, duration, saving throws) of the original PF2E spells they replace.
- SET-20: The setting MUST document all spell-to-technology/chemistry conversions in a deviation log within the setting module.

## Weapons and Equipment

- SET-21: The setting MUST include modern and futuristic weapons: firearms (pistols, rifles, shotguns, submachine guns), energy weapons, and explosives (grenades, mines, demolition charges).
- SET-22: The setting MUST retain all standard PF2E melee and ranged weapons, re-flavored where appropriate for the setting.
- SET-23: Firearms MUST define: damage dice, range increments, reload actions, magazine capacity, and weapon traits.
- SET-24: Explosives MUST define: area of effect, damage dice, saving throw type and DC, and fuse/trigger mechanics.
- SET-25: The setting MUST replace medieval PF2E armor with thematically appropriate equivalents (ballistic vests, salvaged plating, hazmat suits, powered exoskeletons).
- SET-26: The setting MUST define item rarity and scarcity appropriate to a post-collapse world.

## Currency and Economy

- SET-27: The setting MUST define a currency system appropriate to post-collapse Portland (e.g., barter goods, salvage credits, faction scrip).

## Factions and NPCs

- SET-28: The setting MUST define at least three NPC factions with distinct goals, territories within Portland, and reputation tracks.
- SET-29: NPC factions MUST have defined relationships with each other (allied, neutral, hostile).
- SET-30: The setting MUST provide a starter zone with tutorial-appropriate content for new characters.

## Adaptation from PF2E

- SET-31: The setting MUST document all adaptations from standard PF2E in a deviation log within the setting module.
- SET-32: The setting MUST preserve PF2E mechanical balance when adapting content; flavor changes MUST NOT alter statistical properties without explicit documentation in the deviation log.

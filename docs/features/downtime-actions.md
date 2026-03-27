# Downtime Actions

PF2E downtime actions implemented in Gunchete. Players use a downtime time-tracking system to perform activities between adventures.

## Implemented Activities

- [x] **Earn Income** — `earn_income` command. Skill check vs city DC; credits earned per day.
- [x] **Subsist** — `subsist` command. Scavenging check vs zone DC; success covers food/shelter; failure applies fatigued.
- [x] **Patch Up** — `patchup` command. Medicine check; heals HP based on outcome.
- [x] **Run Cover** — `runcover` command. Deception check; reduces wanted level.
- [x] **Forge Papers** — `forge` command. Hustle check vs DC 15. Requires `forgery_supplies` in inventory (consumed at start). CritSuccess: produces `undetectable_forgery` + refunds supplies. Success: produces `convincing_forgery`. Failure/CritFail: supplies lost.

## Excluded Activities

- **Long-Term Rest** — Intentionally excluded. Long-Term Rest restores the full HP pool after 8 hours of rest, which has no meaningful value in Gunchete's design. HP recovery is handled by the rest/camping system (`rest` command). Adding Long-Term Rest as a downtime activity would duplicate existing mechanics and create confusion.
- **Craft** — Moved to a separate feature (`craft-downtime`). Requires recipe system integration and is complex enough to warrant its own planning cycle.
- **Retrain** — Moved to a separate feature (`retrain-downtime`). Requires feat/skill mutation and trainer NPC integration.

## Requirements

- [x] REQ-DA-1: Downtime activities MUST use a real-time timer system (proxy for in-game time).
- [x] REQ-DA-2: Only one downtime activity MAY be active at a time per player.
- [x] REQ-DA-3: Activities MUST be blocked during combat.
- [x] REQ-DA-FORGE-1: The `forge` command MUST consume one `forgery_supplies` item from the player's inventory at activity start; if none are present, the activity MUST be blocked with a message.
- [x] REQ-DA-FORGE-2: On a Critical Success, `resolveForgePapers` MUST refund one `forgery_supplies` item to the player's backpack in addition to delivering `undetectable_forgery`.

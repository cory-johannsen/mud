# Traps

Room hazards that trigger on specific conditions, dealing damage or applying conditions to players. Six trap types: Mine, Pit, Bear Trap, Trip Wire, Pressure Plate, and Honkeypot. See `docs/superpowers/specs/2026-03-20-traps-design.md` for the full design spec.

## Requirements

- [x] Trap templates (`content/traps/` YAML files)
  - REQ-TR-11: Pressure Plate `payload_template` MUST NOT reference another Pressure Plate; fatal load error.
  - REQ-TR-16: Pressure Plate `reset_mode` governs instance lifecycle; linked payload template's `reset_mode` MUST be ignored.
- [x] TrapManager runtime state (`internal/game/trap/`)
  - REQ-TR-12: Detection state MUST be cleared for all players on room reset.
- [x] Trigger system
  - [x] Entry trigger — fires when player enters room
  - [x] Interaction trigger — fires on `use`/`interact` with equipment item
  - [x] Pressure plate trigger — fires on any move action during combat
  - [x] Region trigger (Honkeypot) — fires when player's home region matches `target_regions`
    - REQ-TR-1: Non-targeted players MUST NOT trigger a Honkeypot.
    - REQ-TR-2: Honkeypots MUST NOT appear in Search detection rolls for non-targeted players.
  - [x] Cover crossfire implicit hook — fires when attack misses cover item with `TrapTemplate` set
- [x] Detection (Case It mode only — `sess.ExploreMode == "case_it"`)
  - REQ-TR-3: Trap detection MUST only be available during Case It exploration mode (mode ID `"case_it"`, the `exploration` feature's implementation of Search mode).
  - REQ-TR-4: On room entry in Case It mode, secret Awareness check vs scaled `stealth_dc` for each armed trap (satisfied by REQ-EXP-19 through REQ-EXP-24 in the `exploration` feature).
  - REQ-TR-5: Successful detection MUST flag trap as detected and reveal its name and location.
  - REQ-TR-6: Failed detection check MUST produce no message.
  - REQ-TR-7: Honkeypots MUST be excluded from Search detection rolls for non-targeted players.
- [x] Disarm (`disarm <trap-name>`)
  - REQ-TR-13: Trap triggered by failed disarm MUST apply payload to the disarming player only.
- [x] Payload types
  - [x] Mine — 4d6 piercing+fire, Reflex save, AoE
  - [x] Pit — 2d6 fall, immobilized, Reflex save
  - [x] Bear Trap — 2d6 piercing, grabbed, no save
    - REQ-TR-14: Bear Trap MUST apply grabbed condition with no save.
  - [x] Trip Wire — 1d6 slashing, prone, Reflex save
  - [x] Honkeypot — Technology effect (charm/confused/etc.), Will save, no damage
    - REQ-TR-15: Honkeypot `damage_bonus` MUST be silently ignored.
  - [x] Pressure Plate — inherits payload from linked template
- [x] Danger level scaling
  - REQ-TR-8: Scaling MUST be applied at trigger/detection time from room's runtime DangerLevel.
- [x] Procedural generation
  - REQ-TR-9: Procedural placement MUST NOT overwrite statically defined room traps.
  - REQ-TR-10: Procedurally placed traps MUST use `one_shot` or `auto` reset mode only.
- [x] Global default trap pool (`content/traps/defaults.yaml`)

## Implementation Complete

All requirements implemented across Tasks 1–11. Key components:
- `internal/game/trap/` — TrapTemplate, TrapManager, danger scaling, payload resolution, procedural placement
- `internal/gameserver/grpc_service_trap.go` — entry/region/interaction/pressure-plate/crossfire triggers, detection, disarm
- `content/traps/` — YAML trap templates for all six payload types plus defaults pool

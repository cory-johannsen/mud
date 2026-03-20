# Traps

Room hazards that trigger on specific conditions, dealing damage or applying conditions to players. Six trap types: Mine, Pit, Bear Trap, Trip Wire, Pressure Plate, and Honkeypot. See `docs/superpowers/specs/2026-03-20-traps-design.md` for the full design spec.

## Requirements

- [ ] Trap templates (`content/traps/` YAML files)
  - REQ-TR-11: Pressure Plate `payload_template` MUST NOT reference another Pressure Plate; fatal load error.
  - REQ-TR-16: Pressure Plate `reset_mode` governs instance lifecycle; linked payload template's `reset_mode` MUST be ignored.
- [ ] TrapManager runtime state (`internal/game/trap/`)
  - REQ-TR-12: Detection state MUST be cleared for all players on room reset.
- [ ] Trigger system
  - [ ] Entry trigger — fires when player enters room
  - [ ] Interaction trigger — fires on `use`/`interact` with equipment item
  - [ ] Pressure plate trigger — fires on any move action during combat
  - [ ] Region trigger (Honkeypot) — fires when player's home region matches `target_regions`
    - REQ-TR-1: Non-targeted players MUST NOT trigger a Honkeypot.
    - REQ-TR-2: Honkeypots MUST NOT appear in Search detection rolls for non-targeted players.
  - [ ] Cover crossfire implicit hook — fires when attack misses cover item with `TrapTemplate` set
- [ ] Detection (Search mode only)
  - REQ-TR-3: Trap detection MUST only be available during Search exploration mode.
  - REQ-TR-4: On room entry in Search mode, secret Perception check vs scaled `stealth_dc` for each armed trap.
  - REQ-TR-5: Successful detection MUST flag trap as detected and reveal its name and location.
  - REQ-TR-6: Failed detection check MUST produce no message.
  - REQ-TR-7: Honkeypots MUST be excluded from Search detection rolls for non-targeted players.
- [ ] Disarm (`disarm <trap-name>`)
  - REQ-TR-13: Trap triggered by failed disarm MUST apply payload to the disarming player only.
- [ ] Payload types
  - [ ] Mine — 4d6 piercing+fire, Reflex save, AoE
  - [ ] Pit — 2d6 fall, immobilized, Reflex save
  - [ ] Bear Trap — 2d6 piercing, grabbed, no save
    - REQ-TR-14: Bear Trap MUST apply grabbed condition with no save.
  - [ ] Trip Wire — 1d6 slashing, prone, Reflex save
  - [ ] Honkeypot — Technology effect (charm/confused/etc.), Will save, no damage
    - REQ-TR-15: Honkeypot `damage_bonus` MUST be silently ignored.
  - [ ] Pressure Plate — inherits payload from linked template
- [ ] Danger level scaling
  - REQ-TR-8: Scaling MUST be applied at trigger/detection time from room's runtime DangerLevel.
- [ ] Procedural generation
  - REQ-TR-9: Procedural placement MUST NOT overwrite statically defined room traps.
  - REQ-TR-10: Procedurally placed traps MUST use `one_shot` or `auto` reset mode only.
- [ ] Global default trap pool (`content/traps/defaults.yaml`)

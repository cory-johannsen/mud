# Cover System Design

## Goal

Implement PF2E three-tier cover (Lesser, Standard, Greater) for both players and NPCs, with per-combatant cover tracking, condition-based bonus application, and crossfire degradation of destructible cover objects.

## Architecture

Cover is modeled entirely through the existing condition system. Three new condition YAML files replace the broken `in_cover.yaml`. Room equipment YAML gains cover metadata. Per-combatant cover state lives on `Combatant` (equipment ID + tier). Destructible cover HP is tracked in a `RoomCoverState` map in `CombatHandler`.

## Tech Stack

Go, `pgregory.net/rapid` for property-based tests, existing condition/combat/NPC infrastructure.

---

## Section 1: Data Model

### Condition YAML

Replace `content/conditions/in_cover.yaml` with three files:

- `content/conditions/lesser_cover.yaml`: `ac_penalty: 1, reflex_bonus: 1, stealth_bonus: 1`
- `content/conditions/standard_cover.yaml`: `ac_penalty: 2, reflex_bonus: 2, stealth_bonus: 2`
- `content/conditions/greater_cover.yaml`: `ac_penalty: 4, reflex_bonus: 4, stealth_bonus: 4`

### Condition Struct

Add `ReflexBonus int` and `StealthBonus int` fields to the condition definition struct. Add `ReflexBonus(s *ActiveSet) int` and `StealthBonus(s *ActiveSet) int` functions to `internal/game/condition/modifiers.go`.

### Room Equipment YAML

Add to `RoomEquipmentConfig` in `internal/game/world/model.go`:

```go
CoverTier         string `yaml:"cover_tier"`         // "lesser", "standard", "greater", or ""
CoverDestructible bool   `yaml:"cover_destructible"`
CoverHP           int    `yaml:"cover_hp"`
```

### Runtime Cover State

Add `RoomCoverState map[string]int` to `CombatHandler` (keyed by `roomID+":"+equipmentID`), storing current HP for each destructible cover object.

### Combatant Cover Tracking

Add to `Combatant` struct in `internal/game/combat/combat.go`:

```go
CoverEquipmentID string
CoverTier        string
```

---

## Section 2: Take Cover Action & NPC Cover Use

### Take Cover Command (CMD-1→7)

New command `take_cover` following full CMD-1→7 pipeline. Combat-only. Auto-selects best available cover tier from room equipment. Grants the matching cover condition to the combatant. Sets `Combatant.CoverEquipmentID` and `Combatant.CoverTier`. If already in equal or better cover, no-ops with message. Upgrades silently if better tier available.

### Leave Cover

Cover condition removed when combatant uses any move action (Stride, Step, Tumble). Cover condition also removed on cover destruction.

### NPC Cover Use

Add `combat.strategy.use_cover bool` to NPC type YAML config (`internal/game/npc/`). NPCs with this flag automatically Take Cover at the start of their turn if cover is available and they are not already in cover. Logic lives in NPC turn processing in `combat_handler.go`.

---

## Section 3: Cover Degradation (Crossfire Model)

In `ResolveRound` (`internal/game/combat/round.go`), after hit/miss determination:

- If **miss** AND target has a cover condition AND `attackRoll >= target.AC` (base, ignoring cover penalty) → decrement `RoomCoverState[roomID+":"+equipID]`
- If HP reaches 0 → remove cover condition from all combatants using that equipment ID → emit `EventCoverDestroyed`

New `RoundEvent` types: `EventTakeCover`, `EventCoverDegraded`, `EventCoverDestroyed`.

---

## Section 4: Bonus Application

- **AC bonus**: `ac_penalty` field on cover conditions feeds into existing `ACBonus()` → applied to attacker's roll in `ResolveRound`. No new code needed once YAML values are correct.
- **Reflex bonus**: `ReflexBonus(s *ActiveSet) int` in `modifiers.go`; wired into Reflex save checks (future hazard/trap system hook).
- **Stealth bonus**: `StealthBonus(s *ActiveSet) int` in `modifiers.go`; applied when computing NPC stealth DC in `maxNPCStealthInRoom` in `grpc_service.go`.

---

## Section 5: Testing

- Property-based tests for `ReflexBonus` and `StealthBonus` on `ActiveSet`
- Property-based tests for cover degradation: attack rolls in the cover window (≥ base AC, < cover-adjusted AC) always decrement HP; rolls outside never do
- Integration test: Take Cover → attack misses via cover → HP decrements → at 0 condition removed from all users
- NPC `use_cover` strategy: NPC with flag takes cover when available; NPC without flag does not

# Condition-Applying Actions Design

## Goal

Implement six PF2E actions that apply or interact with conditions, two new conditions (`grabbed`, `hidden`), and wire Sneak Attack damage into the combat path.

## Architecture

All six commands go through the full CMD-1 through CMD-7 pipeline. Resolver-adjacent logic (hidden flat check, sneak attack) lives in `grpc_service.go` where session and condition data are available, keeping the pure combat resolver free of session concerns. Two new condition YAML files. One new field on `PlayerSession` to track grabber NPC ID for Escape DC.

---

## Section 1: New Conditions

### `content/conditions/grabbed.yaml`
- `id: grabbed`
- `ac_penalty: 2` (flat-footed effect — the grabbed creature is off-guard)
- `duration_type: round` (expires at start of round; player must use Grapple again to renew)
- `max_stacks: 0`
- Immobilized effect is a stub (no room movement in combat currently)

### `content/conditions/hidden.yaml`
- `id: hidden`
- No stat penalties (miss chance handled in `grpc_service.go`, not YAML)
- `duration_type: encounter`
- `max_stacks: 0`
- Removed when: player attacks, or any NPC attack resolves against the player

---

## Section 2: The Six Commands

| Command | Syntax | AP | Skill | DC | On Success | Requirements |
|---------|--------|----|-------|----|------------|--------------|
| `grapple` | `grapple <target>` | 1 | athletics | NPC Fortitude DC | Apply `grabbed` to NPC | Combat only |
| `trip` | `trip <target>` | 1 | athletics | NPC Reflex DC | Apply `prone` to NPC | Combat only |
| `hide` | `hide` | 1 | stealth | Highest NPC Perception in room | Apply `hidden` to self | Combat only |
| `sneak` | `sneak` | 1 | stealth | Highest NPC Perception in room | Maintain `hidden`; on fail remove `hidden` | Player must have `hidden` |
| `divert` | `divert` | 1 | deception | Highest NPC Perception in room | Apply `hidden` for 1 round | Combat only |
| `escape` | `escape` | 1 | max(athletics, acrobatics) | Grabber NPC's Athletics + 10 | Remove `grabbed` from player | Player must have `grabbed` |

### NPC Perception DC

For Hide, Sneak, and Create Diversion: DC = highest Perception among all living NPC instances in the player's current room.

### Escape DC

When a player is grabbed by an NPC, store the grabber's `inst.ID` in `sess.GrabberID string` (new field on `PlayerSession`). On Escape, look up that NPC's athletics equivalent (`inst.Level + 4`) and add 10 for the DC. If `sess.GrabberID` is empty, return "You are not grabbed."

### Skill lookups

- `athletics` → `sess.Skills["athletics"]` via `skillRankBonus`
- `stealth` → `sess.Skills["stealth"]` via `skillRankBonus`
- `deception` → `sess.Skills["deception"]` (same pattern as grift in feint)
- `acrobatics` → `sess.Skills["acrobatics"]` via `skillRankBonus`

---

## Section 3: Resolver & Sneak Attack Changes

Both changes live in `grpc_service.go`, not in `internal/game/combat/resolver.go`.

### Hidden flat check (NPC attacks player)

In the NPC attack path (where NPC damage is applied to the player):

1. Check if `sess.Conditions.Has("hidden")`.
2. If yes: roll `1d20`. On 1–10 (DC 11 flat check), the attack misses — return a miss message, skip damage.
3. Remove `hidden` from `sess.Conditions` regardless of flat check result (being targeted breaks concealment).

### Sneak Attack damage (player attacks NPC)

In the player's Strike/attack resolution path, after a hit is confirmed:

1. Check if the player's session has the `sneak_attack` class feature active.
2. Check if the NPC target is eligible:
   - Target has `grabbed` condition (ACMod applied via ApplyCombatantACMod -2), OR
   - Target has `flat_footed` condition (ACMod applied), OR
   - Player has `hidden` condition (attacking from concealment).
3. If eligible: roll `1d6` and add to damage total before applying to NPC HP.

**Class feature check:** `sess.ClassFeatures` is a slice of class feature structs. Check for `pf2e == "sneak_attack"` and `active == true` (passive features are always active; check the `Active` field or the feature's nature).

**Condition eligibility:** Since grabbed and flat_footed are tracked as `ACMod` on the `Combatant` (not as named conditions on the NPC), the check is: `combatant.ACMod < 0` (any negative ACMod means the target is off-guard for sneak attack purposes). Player hidden is checked via `sess.Conditions.Has("hidden")`.

---

## Testing

- Unit tests for each of the 6 `Handle<Name>` command parsers (with property-based tests)
- Integration tests for each `handle<Name>` grpc handler: no-session, not-in-combat, target-not-found, success path, failure path
- Unit tests for hidden flat check: hidden player takes no damage on flat check fail; loses hidden after attack
- Unit tests for sneak attack: player with sneak_attack feature + flat-footed target → 1d6 bonus damage; player without feature → no bonus
- All existing tests must continue to pass

---

## FEATURES.md Updates

Mark these as `[x]` when implemented:
- Grapple (Athletics Actions)
- Trip (Athletics Actions)
- Hide, Sneak, Create Diversion (Stealth & Deception Actions)
- Escape (Tactical Actions)

Add stub under Advanced combat mechanics > Combat distance:
- `[ ] Immobilized — prevent grabbed creatures from moving`

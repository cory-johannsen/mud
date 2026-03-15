# Zone Effect Triggers — Design Spec

**Date:** 2026-03-15

---

## Goal

Rooms can declare persistent mental-state effects (Rage, Despair, Delirium, Fear) that fire on player entry and each combat round. Successful saves grant a cooldown window of immunity. Effects only apply to players, not NPCs.

---

## Feature 1: Data Model

### RoomEffect value type (`internal/game/world/model.go`)

Add a new value type alongside the existing Room model:

```go
// RoomEffect declares a persistent mental-state aura for a room.
// Effects fire on room entry and at the start of each combat round.
type RoomEffect struct {
    // Track is the mental state track to trigger.
    // One of "rage", "despair", "delirium", "fear".
    Track string `yaml:"track"`

    // Severity is the minimum severity to apply.
    // One of "mild", "moderate", "severe".
    Severity string `yaml:"severity"`

    // BaseDC is the Grit save difficulty. Effective save: d20 + GritMod vs BaseDC.
    BaseDC int `yaml:"base_dc"`

    // CooldownRounds is rounds of immunity after a successful in-combat save.
    CooldownRounds int `yaml:"cooldown_rounds"`

    // CooldownMinutes is minutes of immunity after a successful out-of-combat save.
    CooldownMinutes int `yaml:"cooldown_minutes"`
}
```

Add `Effects []RoomEffect` to the `Room` struct and the zone YAML loader.

### Room YAML example

```yaml
effects:
  - track: despair
    severity: mild
    base_dc: 12
    cooldown_rounds: 3
    cooldown_minutes: 5
```

### PlayerSession extension (`internal/game/session/manager.go`)

Add one field to `PlayerSession`:

```go
// ZoneEffectCooldowns maps "roomID:track" to an immunity value.
// In combat: value is rounds remaining (decremented each round; 0 = ready to fire).
// Out of combat: value is Unix timestamp (seconds) of expiry; 0 = ready to fire.
// Nil until first use; initialized lazily on first write.
ZoneEffectCooldowns map[string]int64
```

The key format is `roomID + ":" + track` (e.g., `"toxic_pit:despair"`).

---

## Feature 2: Combat Execution

**Location:** `internal/gameserver/combat_handler.go`

### 2a: Cooldown decrement

At the start of `autoQueueNPCsLocked`, after the existing NPC ability cooldown decrement loop, add a zone effect cooldown decrement for each living player combatant:

```
for each living player combatant cbt:
    sess := sessions.GetPlayer(cbt.ID)
    if sess.ZoneEffectCooldowns == nil: continue
    for k := range sess.ZoneEffectCooldowns:
        sess.ZoneEffectCooldowns[k]--
        if sess.ZoneEffectCooldowns[k] < 0:
            sess.ZoneEffectCooldowns[k] = 0
```

### 2b: Effect application

In `autoQueueNPCsLocked`, after decrementing cooldowns, iterate each living player combatant and apply room effects:

```
room := worldMgr.Room(roomID, zoneID)
for each effect in room.Effects:
    key := cbt.roomID + ":" + effect.Track
    if sess.ZoneEffectCooldowns[key] > 0: continue  // immune

    // Resolve Will save: d20 + GritMod vs effect.BaseDC
    saveOutcome := ResolveSave("toughness", cbt, effect.BaseDC, src)

    if saveOutcome == Failure or saveOutcome == CritFailure:
        changes := mentalStateMgr.ApplyTrigger(cbt.ID, track, severity)
        h.applyMentalStateChanges(cbt.ID, changes)
        // Broadcast narrative (reuse existing applyMentalStateChanges output)
    else:
        // Successful save — set cooldown immunity
        if sess.ZoneEffectCooldowns == nil:
            sess.ZoneEffectCooldowns = make(map[string]int64)
        sess.ZoneEffectCooldowns[key] = int64(effect.CooldownRounds)
```

Track/severity string parsing uses the same helpers as NPC ability triggers (`abilityTrack`, `abilitySeverity` in `combat_handler.go`).

---

## Feature 3: Out-of-Combat Execution

**Location:** `internal/gameserver/grpc_service.go` — `handleMove`

After the player's `RoomID` is updated and the room view is fetched, check the destination room's effects:

```
room := worldMgr.Room(newRoomID, zoneID)
now := time.Now().Unix()
for each effect in room.Effects:
    key := newRoomID + ":" + effect.Track
    if sess.ZoneEffectCooldowns != nil && sess.ZoneEffectCooldowns[key] > now:
        continue  // immune

    // Resolve Will save: d20 + GritMod (no combatant — use session Abilities directly)
    gritMod := combat.AbilityMod(sess.Abilities.Grit)
    roll := dice.Roll(1, 20, src)
    total := roll + gritMod
    if total < effect.BaseDC:
        changes := mentalStateMgr.ApplyTrigger(sess.UID, track, severity)
        // Apply and send narrative to player's stream
    else:
        if sess.ZoneEffectCooldowns == nil:
            sess.ZoneEffectCooldowns = make(map[string]int64)
        sess.ZoneEffectCooldowns[key] = now + int64(effect.CooldownMinutes)*60
```

The out-of-combat path sends the narrative as a `messageEvent` pushed to the player's stream.

---

## Feature 4: Content — Initial Room Effects

Apply effects to rooms where they make thematic sense in existing zones. Examples:

| Zone | Room(s) | Track | Severity | BaseDC | CooldownRounds | CooldownMinutes |
|------|---------|-------|----------|--------|----------------|-----------------|
| Toxic waste areas | sewer/industrial rooms | despair | mild | 12 | 3 | 5 |
| Cult shrine rooms | delirium-inducing zones | delirium | mild | 13 | 3 | 5 |
| Execution/massacre sites | fear-inducing rooms | fear | mild | 11 | 3 | 5 |

The content author surveys `content/zones/` and applies effects to appropriate rooms. This is done as part of implementation, not prescribed here.

---

## Constraints

- Zone effects apply **only to players**, not NPCs.
- A room may declare **multiple effects** (e.g., both despair and delirium). Each is checked independently.
- Cooldown keys are scoped to `roomID:track` — moving rooms resets the room-specific immunity (a player re-entering a toxic room faces the effect again once their cooldown expires).

---

## Testing

- **REQ-T1**: Living player combatant in a room with a despair effect: on `autoQueueNPCsLocked`, `ApplyTrigger` called; `ZoneEffectCooldowns["roomID:despair"]` set to `CooldownRounds`.
- **REQ-T2**: Player with `ZoneEffectCooldowns["roomID:despair"] > 0`: effect skipped; no `ApplyTrigger` call.
- **REQ-T3**: After N decrements equal to `CooldownRounds`, cooldown reaches 0; effect fires on next eligible round.
- **REQ-T4**: Successful save in combat sets `ZoneEffectCooldowns[key] = CooldownRounds`; failed save does not set cooldown.
- **REQ-T5**: `handleMove` to a room with a fear effect: save fails → `ApplyTrigger` called; narrative pushed to player stream.
- **REQ-T6**: `handleMove` to a room with a fear effect: save succeeds → `ZoneEffectCooldowns[key]` set to `now + CooldownMinutes*60`; no trigger applied.
- **REQ-T7**: NPC combatant in same room — effect loop skips NPCs entirely.
- **REQ-T8**: `ZoneEffectCooldowns` nil at first use → map initialized before write; no panic.
- **REQ-T9** (property): For any track ∈ {rage, despair, delirium, fear} and any initial mental state, executing zone effect via full combat path satisfies: `mentalStateMgr.CurrentSeverity(uid, track)` matches the condition stored in `cbt.Conditions[uid]` (no divergence between the two state stores).
- **REQ-T10**: Room with two effects (despair + delirium) — both checked independently each round; each has its own cooldown key.

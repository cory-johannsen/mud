# NPC Rob Player on Defeat — Design Spec

**Date:** 2026-03-13

---

## Goal

When a player is defeated in combat, living NPCs in the fight rifle through the player's pockets and steal a percentage of their currency. The stolen currency is held in the NPC's wallet and paid out to whoever kills that NPC (or stolen via future pickpocket command).

---

## Feature 1: Data Model

### npc.Template — new field

```go
// RobMultiplier controls whether and how aggressively this NPC robs defeated
// players. 0.0 = never robs (default). 1.0 = baseline human aggression.
// Values > 1.0 (e.g. 1.5) represent especially predatory NPCs.
// Used at spawn to compute Instance.RobPercent.
RobMultiplier float64 `yaml:"rob_multiplier"`
```

All existing YAML files that omit `rob_multiplier` default to 0.0 (no rob). Only human/mutant combat NPCs should set a non-zero value. Robot-type NPCs leave it at 0.0.

**Representative YAML values:**
- Street thugs, gangers, bandits: `rob_multiplier: 1.0`
- Aggressive criminals (lieutenants, warlords): `rob_multiplier: 1.5`
- Robots, machines, passive NPCs: omit (0.0)

### npc.Instance — two new fields

```go
// RobPercent is the fraction of a defeated player's currency this NPC steals,
// expressed as a percentage (e.g. 18.5 means 18.5%). 0 means this NPC never robs.
// Computed once at spawn from template RobMultiplier, level, and randomness.
RobPercent float64

// Currency is the NPC's current wallet — accumulated from robbing players.
// Added to loot payout when the NPC dies. Zero at spawn.
Currency int
```

### Spawn formula (NewInstanceWithResolver)

```
if tmpl.RobMultiplier == 0 {
    inst.RobPercent = 0
} else {
    base := 5 + rand.Intn(16)          // random in [5, 20]
    levelBonus := min(tmpl.Level, 10)  // up to +10 at level 10
    raw := float64(base+levelBonus) * tmpl.RobMultiplier
    inst.RobPercent = math.Min(math.Max(raw, 5.0), 30.0)
}
```

This yields:
- Level 1, multiplier 1.0 → roughly 6–21%
- Level 10, multiplier 1.0 → roughly 15–30%
- Level 5, multiplier 1.5 → roughly 15–30% (clamped)

---

## Feature 2: Rob Trigger

**Location:** `internal/gameserver/combat_handler.go`, in the end-of-round defeat check where `!cbt.HasLivingPlayers()` is detected, before the `"Everything goes dark."` narrative is sent.

**Precondition:** At least one player session is in `Dead == true` state; at least one NPC in the combat is alive and has `inst.RobPercent > 0`.

**Postcondition:** Stolen currency is deducted from player wallet, added to NPC wallet, and persisted.

**Algorithm (sequential — each NPC takes from what remains):**

```
for each living NPC inst in the combat where inst.RobPercent > 0:
    stolen := int(math.Floor(float64(sess.Currency) * inst.RobPercent / 100.0))
    if stolen <= 0:
        continue
    inst.Currency += stolen
    sess.Currency -= stolen
    send message to player stream:
        fmt.Sprintf("The %s rifles through your pockets, taking %d rounds.", inst.Name(), stolen)

if any stolen > 0:
    currencySaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency)
```

**Edge cases:**
- REQ-R1: If `sess.Currency == 0`, no rob occurs (stolen rounds to 0 for all NPCs).
- REQ-R2: Multiple NPCs rob sequentially; each computes from the player's remaining currency at that point.
- REQ-R3: Dead NPCs do not rob.
- REQ-R4: If currency saver is unavailable, log a warning but do not fail combat resolution.

---

## Feature 3: Loot Payout

**Location:** `internal/gameserver/combat_handler.go`, in `removeDeadNPCsLocked`, where existing `GenerateLoot` currency is awarded to the killer.

**Change:** After computing loot currency from the loot table, also add `inst.Currency`:

```go
lootCurrency := generatedLoot.Currency + inst.Currency
inst.Currency = 0   // zero the wallet; currency now in loot payout
// … existing: add lootCurrency to killer.Currency, persist
```

**Postcondition:** `inst.Currency` is zeroed after payout. If the NPC dies without ever robbing anyone, `inst.Currency == 0` and payout is unchanged.

---

## YAML Updates

Update the following combat NPC YAML files with `rob_multiplier`:

| File | Suggested value |
|------|----------------|
| `ganger.yaml` | `1.0` |
| `highway_bandit.yaml` | `1.0` |
| `tarmac_raider.yaml` | `1.0` |
| `mill_plain_thug.yaml` | `1.0` |
| `motel_raider.yaml` | `1.0` |
| `river_pirate.yaml` | `1.0` |
| `strip_mall_scav.yaml` | `1.0` |
| `industrial_scav.yaml` | `1.0` |
| `outlet_scavenger.yaml` | `1.0` |
| `scavenger.yaml` | `1.0` |
| `alberta_drifter.yaml` | `1.0` |
| `terminal_squatter.yaml` | `1.0` |
| `cargo_cultist.yaml` | `1.0` |
| `lieutenant.yaml` | `1.5` |
| `brew_warlord.yaml` | `1.5` |
| `gravel_pit_boss.yaml` | `1.5` |
| `commissar.yaml` | `1.5` |
| `bridge_troll.yaml` | `1.5` |

All other NPCs (guards, sentries, robots, patrols, farmers, hermits, etc.) are left at 0.0 (omit the field).

---

## Testing

- REQ-T1 (example): `RobPercent` is 0 when `RobMultiplier == 0`.
- REQ-T2 (property): For any multiplier > 0 and level in [1,20], `RobPercent` is in [5.0, 30.0].
- REQ-T3 (example): Player with 100 currency defeated by one NPC with `RobPercent=20` → NPC gains 20, player has 80.
- REQ-T4 (example): Player with 100 currency defeated by two NPCs each with `RobPercent=20` → first takes 20 (player→80), second takes 16 (player→64); NPC wallets: 20 and 16.
- REQ-T5 (example): Player with 0 currency → no rob message sent, NPC wallets unchanged.
- REQ-T6 (example): NPC with `Currency=25` dies → killer receives loot currency + 25; `inst.Currency` zeroed.
- REQ-T7 (example): Dead NPC in combat does not rob player.
- REQ-T8 (property): For any `RobPercent` in [5,30] and player currency in [0, 10000], stolen is in [0, playerCurrency] and `inst.Currency + sess.Currency == original`.
